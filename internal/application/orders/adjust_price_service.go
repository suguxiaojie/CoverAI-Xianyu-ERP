package orders

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// PriceAdjustmentTaskType 是 account_task_runs 中订单改价动作的稳定任务类型。
	PriceAdjustmentTaskType = "order_price_adjust"
	// priceAdjustmentRequestTimeout 限制单次 render 或 submit 平台请求时间。
	priceAdjustmentRequestTimeout = 30 * time.Second
)

var (
	// ErrPriceAdjustmentNotEligible 表示本地没有仍可改价的归属待付款卡片。
	ErrPriceAdjustmentNotEligible = errors.New("订单不是当前账号可修改的待付款订单")
	// ErrPriceAdjustmentUnavailable 表示平台运行时尚未提供真实网页版改价能力。
	ErrPriceAdjustmentUnavailable = errors.New("当前平台客户端不支持订单改价")
	// ErrPriceAdjustmentAlreadyHandled 表示相同订单和目标金额已经执行、处理中或待人工核对。
	ErrPriceAdjustmentAlreadyHandled = errors.New("相同价格修改已处理或正在核对，请勿重复提交")
	// ErrPriceAdjustmentNeedsReview 表示 submit 远端结果不明确，必须停止自动重放。
	ErrPriceAdjustmentNeedsReview = errors.New("改价结果不明确，已停止重复提交，请人工核对")
	// ErrPriceAdjustmentNoChange 表示所有金额字段与平台当前值一致。
	ErrPriceAdjustmentNoChange = errors.New("修改后的价格与当前价格相同")
)

// priceAdjustmentOrderPattern 只接受当前平台的 10～30 位数字订单号。
var priceAdjustmentOrderPattern = regexp.MustCompile(`^[0-9]{10,30}$`)

// priceAdjustmentAmountPattern 接受非负普通十进制元金额且最多两位小数。
var priceAdjustmentAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]{1,2}))?$`)

// PriceAdjustmentContext 是从结构化待付款卡片读取的非敏感平台关联。
type PriceAdjustmentContext struct {
	// AccountID 是卖家账号标识。
	AccountID string
	// ChatID 是待付款卡片所属会话。
	ChatID string
	// OrderID 是待修改的平台订单标识。
	OrderID string
	// ItemID 是卡片关联商品标识。
	ItemID string
}

// PriceAdjustmentField 是应用层公开的动态金额字段，Value 使用元字符串。
type PriceAdjustmentField struct {
	// Key 是平台 render 返回并由 submit 复用的字段名。
	Key string
	// Name 是字段展示名称。
	Name string
	// PrefixText 是输入框前缀。
	PrefixText string
	// Value 是当前平台金额，单位为元。
	Value string
	// ReadOnly 表示客户端是否只能展示当前金额。
	ReadOnly bool
}

// PriceAdjustmentInput 是用户提交的单个动态字段和值。
type PriceAdjustmentInput struct {
	// Key 必须来自当前 render 的可编辑字段。
	Key string
	// Value 是单位为元、最多两位小数的目标金额。
	Value string
}

// PriceAdjustmentFormRequest 描述一次只读表单请求。
type PriceAdjustmentFormRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是卡片所属卖家账号。
	AccountID string
	// OrderID 是待付款订单标识。
	OrderID string
}

// PriceAdjustmentRequest 描述一次用户二次确认后的真实改价请求。
type PriceAdjustmentRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是卡片所属卖家账号。
	AccountID string
	// OrderID 是待付款订单标识。
	OrderID string
	// Fields 是用户按 render 表单填写的目标金额。
	Fields []PriceAdjustmentInput
}

// PriceAdjustmentFormResult 是 HTTP 层可安全公开的动态表单。
type PriceAdjustmentFormResult struct {
	// OrderID 是当前表单对应的订单标识。
	OrderID string
	// AccountID 是当前表单使用的卖家账号。
	AccountID string
	// Title 是平台返回的表单标题。
	Title string
	// Fields 是平台当前要求展示的金额字段。
	Fields []PriceAdjustmentField
}

// PriceAdjustmentResult 是一次真实 submit 的确定性结果。
type PriceAdjustmentResult struct {
	// Success 表示平台明确返回 data.success=true。
	Success bool
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string
	// Message 是面向用户的非敏感结果说明。
	Message string
	// OrderID 是已提交的平台订单标识。
	OrderID string
	// Fields 是本次按整数分提交后对应的规范化元金额。
	Fields []PriceAdjustmentInput
}

// PriceAdjustmentPlatformForm 保存 runtime 返回的动态字段和 Cookie 会话变化。
type PriceAdjustmentPlatformForm struct {
	// Title 是平台表单标题。
	Title string
	// Fields 是平台动态金额字段。
	Fields []PriceAdjustmentField
	// UpdatedCookies 是仅有平面 Cookie 时的兼容响应。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// PriceAdjustmentPlatformResult 保存平台 submit 结果和 Cookie 会话变化。
type PriceAdjustmentPlatformResult struct {
	// Success 只在平台明确确认改价时为真。
	Success bool
	// Message 是平台非敏感提示。
	Message string
	// UpdatedCookies 是仅有平面 Cookie 时的兼容响应。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// PriceAdjustmentRepository 定义改价所需的卡片归属、幂等和凭证持久化能力。
type PriceAdjustmentRepository interface {
	// GetPendingPriceAdjustment 读取仍未出现付款／关闭终态的归属待付款卡片。
	GetPendingPriceAdjustment(ctx context.Context, userID int64, accountID, orderID string) (*PriceAdjustmentContext, error)
	// ClaimPriceAdjustment 原子抢占相同订单和目标金额的首次或明确失败重试。
	ClaimPriceAdjustment(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error)
	// FinishPriceAdjustment 保存外部动作终态。
	FinishPriceAdjustment(ctx context.Context, runKey, status string, success, failed int, message string) error
	// LockCredentials 只保护短时凭证读取和写回，不得跨网络调用持有。
	LockCredentials(accountID string) func()
	// LoadCookiePlatformDetail 读取平台请求所需的最小凭证视图。
	LoadCookiePlatformDetail(ctx context.Context, accountID string) (*PlatformRuntimeData, error)
	// UpdateRenewalCookie 保存平面 Cookie 兼容响应。
	UpdateRenewalCookie(ctx context.Context, accountID, value, metadata string, at int64) error
}

// PriceAdjustmentRuntime 定义网页版改价平台调用和凭证会话协调能力。
type PriceAdjustmentRuntime interface {
	// PriceAdjustmentAvailable 判断平台客户端是否实现 render 和 submit。
	PriceAdjustmentAvailable() bool
	// CredentialAvailable 判断平台凭证视图是否可用于 MTOP。
	CredentialAvailable(detail *PlatformRuntimeData) bool
	// RenderOrderAdjustPrice 调用只读 render，不修改订单价格。
	RenderOrderAdjustPrice(ctx context.Context, detail *PlatformRuntimeData, orderID string) (*PriceAdjustmentPlatformForm, error)
	// SubmitOrderAdjustPrice 调用真实 submit，fieldCents 的值均为整数分字符串。
	SubmitOrderAdjustPrice(ctx context.Context, detail *PlatformRuntimeData, orderID string, fieldCents map[string]string) (*PriceAdjustmentPlatformResult, error)
	// PersistCookieSession 保存完整 Cookie Jar 请求产生的会话变化。
	PersistCookieSession(ctx context.Context, detail *PlatformRuntimeData, update RefreshCookieUpdate) (string, bool, bool, error)
	// UpdateRunningCookie 同步已持久化的新 Cookie 到账号运行时。
	UpdateRunningCookie(ctx context.Context, accountID, value string)
	// RecoverExpiredSession 处理平台明确返回的会话失效。
	RecoverExpiredSession(ctx context.Context, accountID string, err error) bool
	// IsSessionExpired 判断错误是否属于明确会话失效。
	IsSessionExpired(err error) bool
}

// PriceAdjustmentService 编排待付款卡片归属、动态表单、幂等、平台动作和凭证代次。
type PriceAdjustmentService struct {
	// repository 保存改价卡片、运行和凭证所需窄接口。
	repository PriceAdjustmentRepository
	// runtime 保存网页版 MTOP 与 Cookie 会话协调能力。
	runtime PriceAdjustmentRuntime
	// now 返回运行记录使用的当前时间；测试注入固定值。
	now func() time.Time
}

// NewPriceAdjustmentService 构造改价应用服务；repository 和 runtime 均为生产必需依赖。
func NewPriceAdjustmentService(repository PriceAdjustmentRepository, runtime PriceAdjustmentRuntime, now func() time.Time) *PriceAdjustmentService {
	if now == nil {
		now = time.Now
	}
	return &PriceAdjustmentService{repository: repository, runtime: runtime, now: now}
}

// Form 读取平台当前动态改价字段，不创建运行或产生订单金额写入。
func (service *PriceAdjustmentService) Form(ctx context.Context, request PriceAdjustmentFormRequest) (PriceAdjustmentFormResult, error) {
	// adjustment、validationErr 是本地卡片上下文和输入／归属错误。
	adjustment, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return PriceAdjustmentFormResult{}, validationErr
	}
	// detail、credentialErr 是短时锁内读取的平台凭证。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, adjustment.AccountID)
	if credentialErr != nil {
		return PriceAdjustmentFormResult{}, credentialErr
	}
	// requestCtx、cancel 限制 render 平台读取时间。
	requestCtx, cancel := context.WithTimeout(ctx, priceAdjustmentRequestTimeout)
	defer cancel()
	// platformForm、renderErr 是平台动态表单及读取错误。
	platformForm, renderErr := service.runtime.RenderOrderAdjustPrice(requestCtx, detail, adjustment.OrderID)
	// credentialWarning 是 render 响应 Cookie 未能协调的非表单错误。
	credentialWarning := service.persistCredential(ctx, request.UserID, adjustment.AccountID, detail, platformFormCookie(platformForm))
	if renderErr != nil {
		if service.runtime.IsSessionExpired(renderErr) {
			service.runtime.RecoverExpiredSession(ctx, adjustment.AccountID, renderErr)
		}
		return PriceAdjustmentFormResult{}, renderErr
	}
	if credentialWarning != nil {
		return PriceAdjustmentFormResult{}, fmt.Errorf("读取改价表单后账号凭证已变化: %w", credentialWarning)
	}
	if platformForm == nil || len(platformForm.Fields) == 0 {
		return PriceAdjustmentFormResult{}, errors.New("闲鱼未返回改价表单")
	}
	return PriceAdjustmentFormResult{OrderID: adjustment.OrderID, AccountID: adjustment.AccountID,
		Title: strings.TrimSpace(platformForm.Title), Fields: append([]PriceAdjustmentField(nil), platformForm.Fields...)}, nil
}

// Adjust 根据最新 render 校验用户金额并提交一次真实改价；不明确结果进入人工核对。
func (service *PriceAdjustmentService) Adjust(ctx context.Context, request PriceAdjustmentRequest) (PriceAdjustmentResult, error) {
	// adjustment、validationErr 是本地卡片上下文和输入／归属错误。
	adjustment, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return PriceAdjustmentResult{}, validationErr
	}
	// initialDetail、credentialErr 是 render 前读取的平台凭证。
	initialDetail, credentialErr := service.loadCredential(ctx, request.UserID, adjustment.AccountID)
	if credentialErr != nil {
		return PriceAdjustmentResult{}, credentialErr
	}
	// renderCtx、renderCancel 限制 submit 前重新读取当前表单的时间。
	renderCtx, renderCancel := context.WithTimeout(ctx, priceAdjustmentRequestTimeout)
	// platformForm、renderErr 是 submit 前权威表单及错误。
	platformForm, renderErr := service.runtime.RenderOrderAdjustPrice(renderCtx, initialDetail, adjustment.OrderID)
	renderCancel()
	// renderCredentialErr 是 render 响应 Cookie 协调错误。
	renderCredentialErr := service.persistCredential(ctx, request.UserID, adjustment.AccountID, initialDetail, platformFormCookie(platformForm))
	if renderErr != nil {
		if service.runtime.IsSessionExpired(renderErr) {
			service.runtime.RecoverExpiredSession(ctx, adjustment.AccountID, renderErr)
		}
		return PriceAdjustmentResult{}, renderErr
	}
	if renderCredentialErr != nil {
		return PriceAdjustmentResult{}, fmt.Errorf("提交改价前账号凭证已变化: %w", renderCredentialErr)
	}
	if platformForm == nil {
		return PriceAdjustmentResult{}, errors.New("闲鱼未返回改价表单")
	}
	// fieldCents、normalizedInputs、fieldsErr 是按最新 render 校验后的分值、元金额和错误。
	fieldCents, normalizedInputs, fieldsErr := validatePriceAdjustmentFields(platformForm.Fields, request.Fields)
	if fieldsErr != nil {
		return PriceAdjustmentResult{}, fieldsErr
	}
	// runKey 是账号、订单和规范化目标金额共同组成的持久化幂等键。
	runKey := priceAdjustmentRunKey(adjustment.AccountID, adjustment.OrderID, fieldCents)
	// claimed、claimErr 是真实 submit 的原子抢占结果。
	claimed, claimErr := service.repository.ClaimPriceAdjustment(ctx, runKey, adjustment.AccountID, adjustment.OrderID, service.now().UTC().Unix())
	if claimErr != nil {
		return PriceAdjustmentResult{}, claimErr
	}
	if !claimed {
		return PriceAdjustmentResult{}, ErrPriceAdjustmentAlreadyHandled
	}
	// submitDetail、submitCredentialErr 是 render Cookie 协调后重新读取的权威凭证。
	submitDetail, submitCredentialErr := service.loadCredential(ctx, request.UserID, adjustment.AccountID)
	if submitCredentialErr != nil {
		_ = service.finish(ctx, runKey, "failed", 0, 1, submitCredentialErr.Error(), false)
		return PriceAdjustmentResult{}, submitCredentialErr
	}
	// submitCtx、submitCancel 限制真实改价请求时间。
	submitCtx, submitCancel := context.WithTimeout(ctx, priceAdjustmentRequestTimeout)
	defer submitCancel()
	// platformResult、submitErr 是平台真实改价结果和错误。
	platformResult, submitErr := service.runtime.SubmitOrderAdjustPrice(submitCtx, submitDetail, adjustment.OrderID, fieldCents)
	// credentialWarning 是 submit 响应 Cookie 未能协调的非动作失败告警。
	credentialWarning := service.persistCredential(ctx, request.UserID, adjustment.AccountID, submitDetail, platformResultCookie(platformResult))
	if submitErr != nil {
		if service.runtime.IsSessionExpired(submitErr) {
			service.runtime.RecoverExpiredSession(ctx, adjustment.AccountID, submitErr)
			_ = service.finish(ctx, runKey, "failed", 0, 1, submitErr.Error(), false)
			return PriceAdjustmentResult{}, submitErr
		}
		// finishErr 是不明确结果的隔离状态写入错误；写入失败时 running 仍阻止同键重放。
		finishErr := service.finish(ctx, runKey, "needs_review", 0, 1, submitErr.Error(), true)
		if finishErr != nil {
			return PriceAdjustmentResult{}, errors.Join(ErrPriceAdjustmentNeedsReview, finishErr)
		}
		return PriceAdjustmentResult{Status: "needs_review", Message: ErrPriceAdjustmentNeedsReview.Error(), OrderID: adjustment.OrderID, Fields: normalizedInputs}, ErrPriceAdjustmentNeedsReview
	}
	if platformResult == nil || !platformResult.Success {
		// message 是平台明确拒绝时保存和展示的说明。
		message := "闲鱼未确认价格修改成功"
		if platformResult != nil && strings.TrimSpace(platformResult.Message) != "" {
			message = strings.TrimSpace(platformResult.Message)
		}
		_ = service.finish(ctx, runKey, "failed", 0, 1, message, false)
		return PriceAdjustmentResult{Status: "failed", Message: message, OrderID: adjustment.OrderID, Fields: normalizedInputs}, fmt.Errorf("%w: %s", ErrPriceAdjustmentNotEligible, message)
	}
	// status、message 区分纯成功和凭证并发变化警告。
	status, message := "succeeded", "价格修改成功，等待闲鱼系统卡片同步"
	if credentialWarning != nil {
		status = "succeeded_with_warning"
		message += "；账号凭证已发生变化，未覆盖较新的登录状态"
	}
	if // finishErr 是平台明确成功后的本地幂等终态写入错误。
	finishErr := service.finish(ctx, runKey, "success", 1, 0, credentialErrorText(credentialWarning), true); finishErr != nil {
		return PriceAdjustmentResult{Success: true, Status: "needs_review", Message: "闲鱼已修改价格，但本地状态保存失败，请勿重复提交", OrderID: adjustment.OrderID, Fields: normalizedInputs}, nil
	}
	return PriceAdjustmentResult{Success: true, Status: status, Message: message, OrderID: adjustment.OrderID, Fields: normalizedInputs}, nil
}

// validateContext 校验服务依赖、订单号和结构化待付款卡片归属。
func (service *PriceAdjustmentService) validateContext(ctx context.Context, userID int64, accountID, orderID string) (*PriceAdjustmentContext, error) {
	if service == nil || service.repository == nil || service.runtime == nil {
		return nil, errors.New("订单改价依赖未初始化")
	}
	// normalizedAccountID、normalizedOrderID 是去空白后的卖家账号和平台订单标识。
	normalizedAccountID, normalizedOrderID := strings.TrimSpace(accountID), strings.TrimSpace(orderID)
	if normalizedAccountID == "" || !priceAdjustmentOrderPattern.MatchString(normalizedOrderID) {
		return nil, NewValidationError("改价需要有效账号和数字订单号")
	}
	if !service.runtime.PriceAdjustmentAvailable() {
		return nil, ErrPriceAdjustmentUnavailable
	}
	// adjustment、readErr 是归属且未出现后续终态的待付款卡片上下文。
	adjustment, readErr := service.repository.GetPendingPriceAdjustment(ctx, userID, normalizedAccountID, normalizedOrderID)
	if readErr != nil {
		if errors.Is(readErr, ErrNotFound) {
			return nil, ErrPriceAdjustmentNotEligible
		}
		return nil, readErr
	}
	if adjustment == nil || adjustment.AccountID != normalizedAccountID || adjustment.OrderID != normalizedOrderID {
		return nil, ErrPriceAdjustmentNotEligible
	}
	return adjustment, nil
}

// validatePriceAdjustmentFields 按最新 render 校验字段集合、元金额和是否实际发生变化。
func validatePriceAdjustmentFields(platformFields []PriceAdjustmentField, inputs []PriceAdjustmentInput) (map[string]string, []PriceAdjustmentInput, error) {
	// inputByKey 保存用户提交的去空白字段；重复 key 直接拒绝。
	inputByKey := make(map[string]string, len(inputs))
	// input 是当前待规范化的用户金额字段。
	for _, input := range inputs {
		// key 是去空白后的平台字段名。
		key := strings.TrimSpace(input.Key)
		if key == "" {
			return nil, nil, NewValidationError("改价字段名不能为空")
		}
		if // _, exists 表示客户端是否重复提交当前动态字段。
		_, exists := inputByKey[key]; exists {
			return nil, nil, NewValidationError("改价字段不能重复")
		}
		inputByKey[key] = strings.TrimSpace(input.Value)
	}
	// fieldCents 保存平台 submit 使用的整数分字符串。
	fieldCents := make(map[string]string, len(platformFields))
	// normalizedInputs 保存按平台顺序返回给前端的两位小数元金额。
	normalizedInputs := make([]PriceAdjustmentInput, 0, len(platformFields))
	// knownKeys 保存 render 允许的字段名，用于拒绝客户端注入额外 data。
	knownKeys := make(map[string]struct{}, len(platformFields))
	// changed 表示至少一个字段与 render 当前金额不同。
	changed := false
	// platformField 是当前待校验的 render 字段。
	for _, platformField := range platformFields {
		// key 是平台字段名。
		key := strings.TrimSpace(platformField.Key)
		if key == "" || key == "orderId" {
			return nil, nil, NewValidationError("闲鱼返回了无效改价字段")
		}
		knownKeys[key] = struct{}{}
		// targetYuan 是只读默认值或用户填写值。
		targetYuan := strings.TrimSpace(platformField.Value)
		if !platformField.ReadOnly {
			// supplied、exists 是客户端对当前可编辑字段提交的金额和值存在标记。
			supplied, exists := inputByKey[key]
			if !exists {
				return nil, nil, NewValidationError("改价字段不完整")
			}
			targetYuan = supplied
		}
		// allowZero 只有商品价格字段禁止零元；运费等其他 render 字段允许零元。
		allowZero := key != "modifyFee"
		// targetNormalized、targetCents、targetErr 是目标金额的规范元文本、分值和错误。
		targetNormalized, targetCents, targetErr := normalizePriceAdjustmentAmount(targetYuan, allowZero)
		if targetErr != nil {
			return nil, nil, targetErr
		}
		// currentNormalized、currentCents、currentErr 是 render 当前金额的规范结果。
		currentNormalized, currentCents, currentErr := normalizePriceAdjustmentAmount(platformField.Value, allowZero)
		if currentErr != nil {
			return nil, nil, NewValidationError("闲鱼返回了无效当前金额")
		}
		fieldCents[key] = targetCents
		normalizedInputs = append(normalizedInputs, PriceAdjustmentInput{Key: key, Value: targetNormalized})
		if targetCents != currentCents || targetNormalized != currentNormalized {
			changed = true
		}
	}
	// inputKey 是客户端提交的当前字段名，必须存在于最新 render。
	for inputKey := range inputByKey {
		if // _, known 表示当前客户端字段是否来自最新平台 render。
		_, known := knownKeys[inputKey]; !known {
			return nil, nil, NewValidationError("改价请求包含未知字段")
		}
	}
	if len(fieldCents) == 0 {
		return nil, nil, NewValidationError("闲鱼未返回可修改金额")
	}
	if !changed {
		return nil, nil, ErrPriceAdjustmentNoChange
	}
	return fieldCents, normalizedInputs, nil
}

// normalizePriceAdjustmentAmount 把元金额严格转换为两位小数文本和非负整数分字符串。
func normalizePriceAdjustmentAmount(raw string, allowZero bool) (string, string, error) {
	// normalized 是去空白后的普通十进制金额。
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(raw), "¥"), "￥"))
	// matches 保存整数和可选小数部分。
	matches := priceAdjustmentAmountPattern.FindStringSubmatch(normalized)
	if len(matches) != 3 {
		return "", "", NewValidationError("价格必须是最多两位小数的非负金额")
	}
	// whole、wholeErr 是元整数部分及其解析错误。
	whole, wholeErr := strconv.ParseUint(matches[1], 10, 64)
	if wholeErr != nil || whole > math.MaxInt64/100 {
		return "", "", NewValidationError("价格超出支持范围")
	}
	// fraction 是补足到两位的小数部分。
	fraction := matches[2]
	if len(fraction) == 0 {
		fraction = "00"
	} else if len(fraction) == 1 {
		fraction += "0"
	}
	// fractionValue 是分的小数部分数值。
	fractionValue, _ := strconv.ParseUint(fraction, 10, 64)
	// cents 是最终整数分值。
	cents := whole*100 + fractionValue
	if cents == 0 && !allowZero {
		return "", "", NewValidationError("商品价格必须大于 0")
	}
	return fmt.Sprintf("%d.%02d", whole, fractionValue), strconv.FormatUint(cents, 10), nil
}

// priceAdjustmentRunKey 使用排序后的字段分值摘要构造不泄露动态参数的稳定幂等键。
func priceAdjustmentRunKey(accountID, orderID string, fieldCents map[string]string) string {
	// keys 保存按字典序排列的平台字段名，避免 map 遍历顺序改变摘要。
	keys := make([]string, 0, len(fieldCents))
	// key 是当前待排序的动态字段名。
	for key := range fieldCents {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// canonical 保存字段名和分值组成的稳定文本，不包含 Cookie 或签名。
	var canonical strings.Builder
	// key 是当前写入摘要的排序字段名。
	for _, key := range keys {
		canonical.WriteString(key)
		canonical.WriteByte('=')
		canonical.WriteString(fieldCents[key])
		canonical.WriteByte('&')
	}
	// digest 是目标金额组合的短摘要。
	digest := sha256.Sum256([]byte(canonical.String()))
	return PriceAdjustmentTaskType + ":" + strings.TrimSpace(accountID) + ":" + strings.TrimSpace(orderID) + ":" + hex.EncodeToString(digest[:8])
}

// loadCredential 在短时账号锁内读取并复核平台凭证，不持锁执行网络请求。
func (service *PriceAdjustmentService) loadCredential(ctx context.Context, userID int64, accountID string) (*PlatformRuntimeData, error) {
	// unlock 是账号凭证短时读取锁释放函数。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// detail、loadErr 是权威平台凭证视图及其读取错误。
	detail, loadErr := service.repository.LoadCookiePlatformDetail(ctx, accountID)
	if loadErr != nil {
		return nil, loadErr
	}
	if detail == nil || detail.UserID != userID || !service.runtime.CredentialAvailable(detail) {
		return nil, errors.New("账号凭证已变化，请重新登录后再改价")
	}
	return detail, nil
}

// priceAdjustmentCookieResult 抽象 render／submit 共同的 Cookie 会话结果。
type priceAdjustmentCookieResult struct {
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// platformFormCookie 将 render 结果转换为凭证协调输入。
func platformFormCookie(form *PriceAdjustmentPlatformForm) *priceAdjustmentCookieResult {
	if form == nil {
		return nil
	}
	return &priceAdjustmentCookieResult{UpdatedCookies: form.UpdatedCookies, CookieUpdate: form.CookieUpdate}
}

// platformResultCookie 将 submit 结果转换为凭证协调输入。
func platformResultCookie(result *PriceAdjustmentPlatformResult) *priceAdjustmentCookieResult {
	if result == nil {
		return nil
	}
	return &priceAdjustmentCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// persistCredential 仅在外部调用期间凭证代次未变化时保存响应 Cookie。
func (service *PriceAdjustmentService) persistCredential(ctx context.Context, userID int64, accountID string, before *PlatformRuntimeData, result *priceAdjustmentCookieResult) error {
	if before == nil || result == nil {
		return nil
	}
	// unlock 是平台调用后重新获取的凭证写回锁。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// latest、reloadErr 是外部调用完成后的权威凭证视图和读取错误。
	latest, reloadErr := service.repository.LoadCookiePlatformDetail(ctx, accountID)
	if reloadErr != nil {
		return reloadErr
	}
	if latest == nil || latest.UserID != userID || latest.Value != before.Value || latest.MetadataJSON != before.MetadataJSON {
		return errors.New("外部调用期间账号凭证已更新")
	}
	// value、changed、handled、persistErr 是完整 Cookie 会话的持久化结果。
	value, changed, handled, persistErr := service.runtime.PersistCookieSession(ctx, before, result.CookieUpdate)
	if persistErr != nil {
		return persistErr
	}
	if handled {
		if changed && strings.TrimSpace(value) != "" {
			service.runtime.UpdateRunningCookie(ctx, accountID, value)
		}
		return nil
	}
	if strings.TrimSpace(result.UpdatedCookies) == "" || result.UpdatedCookies == before.Value {
		return nil
	}
	if // updateErr 是兼容平面 Cookie 响应写回数据库的错误。
	updateErr := service.repository.UpdateRenewalCookie(ctx, accountID, result.UpdatedCookies, before.MetadataJSON, time.Now().Unix()); updateErr != nil {
		return updateErr
	}
	service.runtime.UpdateRunningCookie(ctx, accountID, result.UpdatedCookies)
	return nil
}

// finish 保存改价运行终态；detached=true 时不受 HTTP 请求取消影响。
func (service *PriceAdjustmentService) finish(ctx context.Context, runKey, status string, success, failed int, message string, detached bool) error {
	// finishCtx 是终态写入使用的上下文。
	finishCtx := ctx
	// cancel 释放独立终态写入的五秒预算；普通失败分支保持空操作。
	cancel := func() {}
	if detached {
		finishCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	}
	defer cancel()
	return service.repository.FinishPriceAdjustment(finishCtx, runKey, status, success, failed, message)
}
