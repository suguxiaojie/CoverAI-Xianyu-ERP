package orders

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// refundDetailRequestTimeout 限制单次只读退款详情平台请求时间。
	refundDetailRequestTimeout = 30 * time.Second
	// RefundActionTaskType 是 account_task_runs 中普通同意／拒绝退款动作的稳定任务类型。
	RefundActionTaskType = "refund_action"
)

var (
	// ErrRefundDetailUnavailable 表示当前平台客户端没有装配只读退款详情能力。
	ErrRefundDetailUnavailable = errors.New("当前平台客户端不支持读取退款详情")
	// ErrRefundDetailNotEligible 表示订单没有退款申请证据或不属于指定账号。
	ErrRefundDetailNotEligible = errors.New("订单没有可读取的退款申请")
	// ErrRefundActionInvalid 表示客户端动作已不在平台最新退款详情中。
	ErrRefundActionInvalid = errors.New("退款处理动作已经失效，请刷新详情后重试")
	// ErrRefundActionAlreadyHandled 表示同一退款申请已经处理、处理中或待人工核对。
	ErrRefundActionAlreadyHandled = errors.New("退款申请已处理或正在核对，请勿重复提交")
	// ErrRefundActionNeedsReview 表示远端结果不明确，必须停止自动重放。
	ErrRefundActionNeedsReview = errors.New("退款处理结果不明确，已停止重复提交，请到闲鱼官方详情核对")
	// ErrRefundActionRequiresOfficial 表示动作必须在闲鱼原生支付密码流程中完成。
	ErrRefundActionRequiresOfficial = errors.New("该退款需要在闲鱼官方完成支付密码验证")
	// refundDetailOrderPattern 只接受当前平台的十至三十位数字订单号。
	refundDetailOrderPattern = regexp.MustCompile(`^[0-9]{10,30}$`)
)

// RefundDetailRequest 描述一次已认证的只读退款详情请求。
type RefundDetailRequest struct {
	// UserID 是当前认证用户标识。
	UserID int64
	// AccountID 是退款订单所属卖家账号。
	AccountID string
	// OrderID 是平台订单标识。
	OrderID string
}

// RefundDetailResult 是可以安全展示给卖家的退款申请信息。
type RefundDetailResult struct {
	// OrderID 是平台订单标识。
	OrderID string
	// AccountID 是订单所属卖家账号。
	AccountID string
	// RefundID 是平台退款申请标识。
	RefundID string
	// Status 是平台退款状态编码或展示文本。
	Status string
	// StatusText 是平台状态组件提供的可读文本。
	StatusText string
	// Type 是仅退款、退货退款等平台展示类型。
	Type string
	// Reason 是买家选择的退款原因。
	Reason string
	// Amount 是平台展示的退款金额文本。
	Amount string
	// ApplyTime 是平台展示的退款申请时间。
	ApplyTime string
	// BuyerDescription 是买家补充说明。
	BuyerDescription string
	// BuyerImages 是买家提交的图片凭证地址。
	BuyerImages []string
	// BuyerVideos 是买家提交的视频凭证地址。
	BuyerVideos []string
	// Seller 表示当前详情明确属于卖家处理视角。
	Seller bool
	// Actions 是平台当前允许在 ERP 内执行的普通同意／拒绝动作。
	Actions []RefundAction
}

// RefundAction 是 HTTP 层可以安全公开的动态退款动作。
type RefundAction struct {
	// Code 是平台当前动作标识，提交时必须重新匹配最新详情。
	Code string
	// Name 是平台面向卖家的按钮名称。
	Name string
	// Kind 是 agree 或 reject。
	Kind string
	// Mode 是 direct 或 official。
	Mode string
	// ConfirmTitle 是平台双重确认标题。
	ConfirmTitle string
	// ConfirmDescription 是平台双重确认说明。
	ConfirmDescription string
}

// RefundPlatformAction 保存服务端执行所需的动态 MTOP 描述，不得序列化到前端。
type RefundPlatformAction struct {
	RefundAction
	// APIName 是平台最新详情下发并通过退款域门禁的接口名。
	APIName string
	// APIVersion 是平台动态接口版本。
	APIVersion string
	// Params 是平台动态请求参数，只在服务端调用栈内存在。
	Params map[string]any
}

// RefundDetailPlatformResult 保存平台只读结果和受控 Cookie 会话变化。
type RefundDetailPlatformResult struct {
	RefundDetailResult
	// PlatformActions 是与公开 Actions 对应的服务端执行描述。
	PlatformActions []RefundPlatformAction
	// UpdatedCookies 是仅有平面 Cookie 时的兼容响应。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// RefundActionRequest 描述用户在应用内二次确认后的真实退款处理请求。
type RefundActionRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是退款订单所属卖家账号。
	AccountID string
	// OrderID 是平台订单标识。
	OrderID string
	// ActionCode 是刚从退款详情读取的平台动作标识。
	ActionCode string
}

// RefundActionResult 保存一次真实退款动作的确定性结果。
type RefundActionResult struct {
	// Success 只在平台明确受理动作时为真。
	Success bool
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string
	// Message 是不含凭证的用户提示。
	Message string
	// OrderID 是本次处理的平台订单标识。
	OrderID string
	// RefundID 是本次处理的平台退款申请标识。
	RefundID string
	// Action 是实际提交的 agree 或 reject。
	Action string
}

// RefundActionPlatformResult 保存平台动作结果和受控 Cookie 会话变化。
type RefundActionPlatformResult struct {
	// Success 只在平台 MTOP 明确成功时为真。
	Success bool
	// ActionAttempted 区分上传前可安全重试失败和最终退款请求后的不明确结果。
	ActionAttempted bool
	// Message 是平台非敏感提示。
	Message string
	// RequiresOfficial 表示平台只返回了原生认证前置地址，不能视为退款成功。
	RequiresOfficial bool
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// RefundDetailRepository 定义退款详情读取所需的订单归属和凭证能力。
type RefundDetailRepository interface {
	// ExistsOwned 判断账号是否归属于当前用户。
	ExistsOwned(context.Context, int64, string) (bool, error)
	// GetOrder 读取本地订单及退款申请证据。
	GetOrder(context.Context, string) (*Order, error)
	// LockCredentials 串行化短时凭证读取和写回。
	LockCredentials(string) func()
	// LoadCookiePlatformDetail 读取平台请求所需的最小凭证视图。
	LoadCookiePlatformDetail(context.Context, string) (*PlatformRuntimeData, error)
	// UpdateRenewalCookie 保存兼容平面 Cookie 响应。
	UpdateRenewalCookie(context.Context, string, string, string, int64) error
	// ClaimRefundAction 原子抢占同一退款申请的首次处理或明确失败重试。
	ClaimRefundAction(context.Context, string, string, string, int64) (bool, error)
	// FinishRefundAction 保存退款处理运行终态。
	FinishRefundAction(context.Context, string, string, int, int, string) error
}

// RefundDetailRuntime 定义只读退款详情和凭证会话协调能力。
type RefundDetailRuntime interface {
	// RefundDetailAvailable 判断平台客户端是否实现官方只读详情接口。
	RefundDetailAvailable() bool
	// CredentialAvailable 判断平台凭证视图是否可用于 MTOP。
	CredentialAvailable(*PlatformRuntimeData) bool
	// FetchRefundDetail 调用官方只读详情接口，不执行同意或拒绝退款。
	FetchRefundDetail(context.Context, *PlatformRuntimeData, string) (*RefundDetailPlatformResult, error)
	// SubmitRefundAction 使用平台最新详情下发的执行描述提交普通同意／拒绝动作。
	SubmitRefundAction(context.Context, *PlatformRuntimeData, string, string, RefundPlatformAction) (*RefundActionPlatformResult, error)
	// CreateMerchantRefundVerification 创建 PC 支付宝验证页面和短期服务端授权。
	CreateMerchantRefundVerification(context.Context, *PlatformRuntimeData, string) (*MerchantRefundVerificationPlatformResult, error)
	// AgreeMerchantRefund 使用验证后的短期授权执行最终同意退款。
	AgreeMerchantRefund(context.Context, *PlatformRuntimeData, string, string) (*RefundActionPlatformResult, error)
	// FetchMerchantRefundRefuseForm 读取 Merchant 动态拒绝表单。
	FetchMerchantRefundRefuseForm(context.Context, *PlatformRuntimeData, string, string) (*MerchantRefundRefuseFormPlatformResult, error)
	// RefuseMerchantRefund 先上传内存图片凭证，再执行 Merchant 拒绝退款。
	RefuseMerchantRefund(context.Context, *PlatformRuntimeData, MerchantRefundRefusePlatformRequest) (*RefundActionPlatformResult, error)
	// PersistCookieSession 保存完整 Cookie Jar 请求产生的会话变化。
	PersistCookieSession(context.Context, *PlatformRuntimeData, RefreshCookieUpdate) (string, bool, bool, error)
	// UpdateRunningCookie 同步已持久化的新 Cookie 到账号运行时。
	UpdateRunningCookie(context.Context, string, string)
	// RecoverExpiredSession 处理平台明确返回的会话失效。
	RecoverExpiredSession(context.Context, string, error) bool
	// IsSessionExpired 判断错误是否属于明确会话失效。
	IsSessionExpired(error) bool
}

// RefundDetailService 编排退款订单归属、只读平台请求和凭证代次保护。
type RefundDetailService struct {
	// repository 保存订单归属和平台凭证读取能力。
	repository RefundDetailRepository
	// runtime 保存官方只读退款详情能力。
	runtime RefundDetailRuntime
	// verificationMu 保护短期支付验证会话；不得跨平台网络调用持有。
	verificationMu sync.Mutex
	// verifications 只在内存保存短期 authToken，不持久化、不输出日志。
	verifications map[string]merchantRefundVerificationSession
}

// NewRefundDetailService 构造只读退款详情应用服务。
func NewRefundDetailService(repository RefundDetailRepository, runtime RefundDetailRuntime) *RefundDetailService {
	return &RefundDetailService{repository: repository, runtime: runtime, verifications: make(map[string]merchantRefundVerificationSession)}
}

// Get 校验本地退款证据后读取官方退款详情；本方法不写订单和退款业务状态。
func (service *RefundDetailService) Get(ctx context.Context, request RefundDetailRequest) (RefundDetailResult, error) {
	// _, accountID、orderID、validationErr 是本地退款上下文占位、规范标识和校验错误。
	_, accountID, orderID, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return RefundDetailResult{}, validationErr
	}
	// before、credentialErr 是短时锁内读取的平台凭证和错误。
	before, credentialErr := service.loadCredential(ctx, request.UserID, accountID)
	if credentialErr != nil {
		return RefundDetailResult{}, credentialErr
	}
	// requestCtx、cancel 限制官方只读详情请求时间。
	requestCtx, cancel := context.WithTimeout(ctx, refundDetailRequestTimeout)
	defer cancel()
	// platformResult、fetchErr 是平台公开退款详情和读取错误。
	platformResult, fetchErr := service.runtime.FetchRefundDetail(requestCtx, before, orderID)
	// credentialErr 保存响应 Cookie 的代次协调结果。
	credentialErr = service.persistCredential(ctx, request.UserID, accountID, before, refundCookieResultFromDetail(platformResult))
	if fetchErr != nil {
		if service.runtime.IsSessionExpired(fetchErr) {
			service.runtime.RecoverExpiredSession(ctx, accountID, fetchErr)
		}
		return RefundDetailResult{}, fetchErr
	}
	if credentialErr != nil {
		return RefundDetailResult{}, fmt.Errorf("读取退款详情后账号凭证已变化: %w", credentialErr)
	}
	if platformResult == nil {
		return RefundDetailResult{}, errors.New("闲鱼未返回退款详情")
	}
	// result 复制平台公开字段并固定本地账号和订单归属。
	result := platformResult.RefundDetailResult
	result.OrderID, result.AccountID = orderID, accountID
	result.BuyerImages = append([]string(nil), platformResult.BuyerImages...)
	result.BuyerVideos = append([]string(nil), platformResult.BuyerVideos...)
	if refundActionTypeSupported(result.Type) {
		result.Actions = append([]RefundAction(nil), platformResult.Actions...)
	} else {
		result.Actions = nil
	}
	return result, nil
}

// validateContext 校验服务依赖、账号归属、订单归属和本地退款申请证据。
func (service *RefundDetailService) validateContext(ctx context.Context, userID int64, rawAccountID, rawOrderID string) (*Order, string, string, error) {
	if service == nil || service.repository == nil || service.runtime == nil {
		return nil, "", "", errors.New("退款详情依赖未初始化")
	}
	// accountID、orderID 是去空白后的卖家账号和平台订单标识。
	accountID, orderID := strings.TrimSpace(rawAccountID), strings.TrimSpace(rawOrderID)
	if accountID == "" || !refundDetailOrderPattern.MatchString(orderID) {
		return nil, "", "", NewValidationError("退款详情需要有效账号和数字订单号")
	}
	if !service.runtime.RefundDetailAvailable() {
		return nil, "", "", ErrRefundDetailUnavailable
	}
	// owned、ownershipErr 是账号归属结果和数据库错误。
	owned, ownershipErr := service.repository.ExistsOwned(ctx, userID, accountID)
	if ownershipErr != nil {
		return nil, "", "", ownershipErr
	}
	if !owned {
		return nil, "", "", ErrForbidden
	}
	// order、orderErr 是本地订单和读取错误。
	order, orderErr := service.repository.GetOrder(ctx, orderID)
	if orderErr != nil {
		return nil, "", "", orderErr
	}
	if order == nil {
		return nil, "", "", ErrNotFound
	}
	// normalizedStatus 是本地退款资格判断使用的状态文本。
	normalizedStatus := strings.ToLower(strings.TrimSpace(order.OrderStatus))
	if order.CookieID != accountID || (!order.RefundRequested && normalizedStatus != "refunding" && normalizedStatus != "refunded") {
		return nil, "", "", ErrRefundDetailNotEligible
	}
	return order, accountID, orderID, nil
}

// loadCredential 在短时账号锁内读取平台凭证，不持锁执行网络请求。
func (service *RefundDetailService) loadCredential(ctx context.Context, userID int64, accountID string) (*PlatformRuntimeData, error) {
	// unlock 是账号凭证短时读取锁释放函数。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// detail、loadErr 是权威平台凭证视图及其读取错误。
	detail, loadErr := service.repository.LoadCookiePlatformDetail(ctx, accountID)
	if loadErr != nil {
		return nil, loadErr
	}
	if detail == nil || detail.UserID != userID || !service.runtime.CredentialAvailable(detail) {
		return nil, errors.New("账号凭证已变化，请重新登录后读取退款详情")
	}
	return detail, nil
}

// persistCredential 仅在平台读取期间凭证代次未变化时保存响应 Cookie。
func (service *RefundDetailService) persistCredential(ctx context.Context, userID int64, accountID string, before *PlatformRuntimeData, result *refundCookieResult) error {
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
	// value、changed、handled、persistErr 是完整 Cookie 会话持久化结果。
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

// refundCookieResult 抽象详情和动作共同的 Cookie 会话结果。
type refundCookieResult struct {
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// refundCookieResultFromDetail 将详情结果转换为凭证协调输入。
func refundCookieResultFromDetail(result *RefundDetailPlatformResult) *refundCookieResult {
	if result == nil {
		return nil
	}
	return &refundCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// refundCookieResultFromAction 将动作结果转换为凭证协调输入。
func refundCookieResultFromAction(result *RefundActionPlatformResult) *refundCookieResult {
	if result == nil {
		return nil
	}
	return &refundCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}
