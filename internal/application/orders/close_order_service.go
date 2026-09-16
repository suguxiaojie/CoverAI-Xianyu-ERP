package orders

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// CloseOrderTaskType 是 account_task_runs 中卖家关闭订单的稳定任务类型。
	CloseOrderTaskType = "order_close"
	// closeOrderRequestTimeout 限制单次关闭原因或关单平台请求时间。
	closeOrderRequestTimeout = 30 * time.Second
)

var (
	// ErrCloseOrderNotEligible 表示订单不是当前账号仍可取消的待付款或已付款待发货订单。
	ErrCloseOrderNotEligible = errors.New("订单当前状态不可取消")
	// ErrCloseOrderUnavailable 表示平台运行时未装配网页版卖家关单能力。
	ErrCloseOrderUnavailable = errors.New("当前平台客户端不支持关闭订单")
	// ErrCloseOrderReasonInvalid 表示所选原因不在平台最新动态列表中。
	ErrCloseOrderReasonInvalid = errors.New("关闭原因已失效，请重新选择")
	// ErrCloseOrderAlreadyHandled 表示同一订单已经关闭、处理中或待人工核对。
	ErrCloseOrderAlreadyHandled = errors.New("订单关闭已处理或正在核对，请勿重复提交")
	// ErrCloseOrderNeedsReview 表示远端关单结果不明确，必须人工核对。
	ErrCloseOrderNeedsReview = errors.New("关闭订单结果不明确，已停止重复提交，请人工核对")
)

// closeOrderIDPattern 只接受当前平台 10～30 位数字订单号。
var closeOrderIDPattern = regexp.MustCompile(`^[0-9]{10,30}$`)

// CloseOrderContext 是从结构化待付款／付款卡片和订单表读取的非敏感取消关联。
type CloseOrderContext struct {
	// AccountID 是卖家账号标识。
	AccountID string
	// ChatID 是可取消订单卡片所属会话。
	ChatID string
	// OrderID 是待关闭的平台订单标识。
	OrderID string
	// ItemID 是卡片关联商品标识。
	ItemID string
	// Stage 是 pending_payment 或 pending_ship，决定取消确认中的资金提示。
	Stage string
}

// CloseOrderReasonsRequest 描述一次只读动态关闭原因请求。
type CloseOrderReasonsRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是卡片所属卖家账号。
	AccountID string
	// OrderID 是待取消订单标识。
	OrderID string
}

// CloseOrderReasonsResult 是 HTTP 层可安全公开的动态原因表单。
type CloseOrderReasonsResult struct {
	// OrderID 是当前原因列表对应的订单标识。
	OrderID string
	// AccountID 是当前原因列表使用的卖家账号。
	AccountID string
	// Reasons 是平台当前允许的关闭原因。
	Reasons []string
}

// CloseOrderEligibilityRequest 描述 Chat 顶部关单入口的纯本地资格查询。
type CloseOrderEligibilityRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是待取消订单所属卖家账号。
	AccountID string
	// OrderID 是待检查的平台订单标识。
	OrderID string
}

// CloseOrderEligibilityResult 返回订单是否仍可进入关单流程，不读取平台原因或凭证。
type CloseOrderEligibilityResult struct {
	// Eligible 只在订单表仍为待付款或已付款待发货，且聊天角色、终态和幂等门禁全部通过时为真。
	Eligible bool
	// AccountID 是规范化后的卖家账号。
	AccountID string
	// OrderID 是规范化后的平台订单标识。
	OrderID string
	// Reason 是不具备资格时可安全展示的本地原因。
	Reason string
	// Stage 是当前权威订单对应的 pending_payment 或 pending_ship。
	Stage string
}

// CloseOrderRequest 描述用户二次确认后的真实卖家关单请求。
type CloseOrderRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是卡片所属卖家账号。
	AccountID string
	// OrderID 是待取消订单标识。
	OrderID string
	// Reason 是用户从平台动态原因中选择的值。
	Reason string
}

// CloseOrderResult 保存一次真实卖家关单的确定性结果。
type CloseOrderResult struct {
	// Success 只在平台明确受理关单时为真。
	Success bool
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string
	// Message 是用户可见且不含凭证的结果说明。
	Message string
	// OrderID 是已提交的平台订单标识。
	OrderID string
}

// CloseOrderPlatformReasons 保存 runtime 返回的动态原因和 Cookie 会话变化。
type CloseOrderPlatformReasons struct {
	// Reasons 是平台当前允许选择的原因。
	Reasons []string
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// CloseOrderPlatformResult 保存平台关单结果和 Cookie 会话变化。
type CloseOrderPlatformResult struct {
	// Success 只在平台明确确认关单时为真。
	Success bool
	// Message 是平台非敏感提示。
	Message string
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// CloseOrderRepository 定义取消订单所需的卡片归属、幂等和凭证持久化能力。
type CloseOrderRepository interface {
	// GetPendingCloseOrder 读取仍处于待付款或已付款待发货且没有后续终态的卖家卡片。
	GetPendingCloseOrder(ctx context.Context, userID int64, accountID, orderID string) (*CloseOrderContext, error)
	// ClaimCloseOrder 原子抢占同一订单的首次关单或明确失败重试。
	ClaimCloseOrder(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error)
	// FinishCloseOrder 保存关单终态。
	FinishCloseOrder(ctx context.Context, runKey, status string, success, failed int, message string) error
	// LockCredentials 只保护短时凭证读取和写回，不得跨网络调用持有。
	LockCredentials(accountID string) func()
	// LoadCookiePlatformDetail 读取平台请求所需的最小凭证视图。
	LoadCookiePlatformDetail(ctx context.Context, accountID string) (*PlatformRuntimeData, error)
	// UpdateRenewalCookie 保存平面 Cookie 兼容响应。
	UpdateRenewalCookie(ctx context.Context, accountID, value, metadata string, at int64) error
}

// CloseOrderRuntime 定义网页版卖家关单平台调用和凭证协调能力。
type CloseOrderRuntime interface {
	// CloseOrderAvailable 判断平台客户端是否实现原因和卖家关单接口。
	CloseOrderAvailable() bool
	// CredentialAvailable 判断平台凭证视图是否可用于 MTOP。
	CredentialAvailable(detail *PlatformRuntimeData) bool
	// FetchCloseOrderReasons 读取平台动态原因，不修改订单。
	FetchCloseOrderReasons(ctx context.Context, detail *PlatformRuntimeData, orderID string) (*CloseOrderPlatformReasons, error)
	// CloseOrderBySeller 执行卖家关单真实写入。
	CloseOrderBySeller(ctx context.Context, detail *PlatformRuntimeData, orderID, reason string) (*CloseOrderPlatformResult, error)
	// PersistCookieSession 保存完整 Cookie Jar 请求产生的会话变化。
	PersistCookieSession(ctx context.Context, detail *PlatformRuntimeData, update RefreshCookieUpdate) (string, bool, bool, error)
	// UpdateRunningCookie 同步已持久化的新 Cookie 到账号运行时。
	UpdateRunningCookie(ctx context.Context, accountID, value string)
	// RecoverExpiredSession 处理平台明确返回的会话失效。
	RecoverExpiredSession(ctx context.Context, accountID string, err error) bool
	// IsSessionExpired 判断错误是否属于明确会话失效。
	IsSessionExpired(err error) bool
}

// CloseOrderService 编排可取消订单归属、动态原因、幂等、真实关单和凭证代次。
type CloseOrderService struct {
	// repository 保存可取消订单卡片、运行和凭证所需窄接口。
	repository CloseOrderRepository
	// runtime 保存网页版 MTOP 与 Cookie 会话协调能力。
	runtime CloseOrderRuntime
	// now 返回运行记录使用的当前时间；测试可以注入固定值。
	now func() time.Time
}

// NewCloseOrderService 构造关单应用服务；repository 和 runtime 是生产必需依赖。
func NewCloseOrderService(repository CloseOrderRepository, runtime CloseOrderRuntime, now func() time.Time) *CloseOrderService {
	if now == nil {
		now = time.Now
	}
	return &CloseOrderService{repository: repository, runtime: runtime, now: now}
}

// Eligibility 只检查本地订单权威状态、聊天角色证据和关单幂等，不调用闲鱼接口。
func (service *CloseOrderService) Eligibility(ctx context.Context, request CloseOrderEligibilityRequest) (CloseOrderEligibilityResult, error) {
	// accountID、orderID 是返回和底层校验共用的规范标识。
	accountID, orderID := strings.TrimSpace(request.AccountID), strings.TrimSpace(request.OrderID)
	// closeContext、validationErr 是当前本地资格上下文和校验结果。
	closeContext, validationErr := service.validateContext(ctx, request.UserID, accountID, orderID)
	if errors.Is(validationErr, ErrCloseOrderNotEligible) {
		return CloseOrderEligibilityResult{Eligible: false, AccountID: accountID, OrderID: orderID, Reason: "订单当前状态不可取消"}, nil
	}
	if validationErr != nil {
		return CloseOrderEligibilityResult{}, validationErr
	}
	return CloseOrderEligibilityResult{Eligible: true, AccountID: closeContext.AccountID, OrderID: closeContext.OrderID, Stage: closeContext.Stage}, nil
}

// Reasons 读取平台当前动态关闭原因，不创建运行或修改订单。
func (service *CloseOrderService) Reasons(ctx context.Context, request CloseOrderReasonsRequest) (CloseOrderReasonsResult, error) {
	// closeContext、validationErr 是本地卡片上下文和输入／归属错误。
	closeContext, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return CloseOrderReasonsResult{}, validationErr
	}
	// platformReasons、reasonErr 是平台动态原因和读取错误。
	platformReasons, reasonErr := service.fetchReasons(ctx, request.UserID, closeContext.AccountID, closeContext.OrderID)
	if reasonErr != nil {
		return CloseOrderReasonsResult{}, reasonErr
	}
	return CloseOrderReasonsResult{OrderID: closeContext.OrderID, AccountID: closeContext.AccountID, Reasons: append([]string(nil), platformReasons.Reasons...)}, nil
}

// Close 在重新校验动态原因和本地取消资格后提交一次真实卖家关单；不明确结果进入人工核对。
func (service *CloseOrderService) Close(ctx context.Context, request CloseOrderRequest) (CloseOrderResult, error) {
	// closeContext、validationErr 是首次本地卡片上下文和输入／归属错误。
	closeContext, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return CloseOrderResult{}, validationErr
	}
	// selectedReason 是去空白后的用户选择，必须匹配平台最新原因。
	selectedReason := strings.TrimSpace(request.Reason)
	if selectedReason == "" || len([]rune(selectedReason)) > 100 {
		return CloseOrderResult{}, ErrCloseOrderReasonInvalid
	}
	// platformReasons、reasonErr 是提交前重新读取的权威动态原因和错误。
	platformReasons, reasonErr := service.fetchReasons(ctx, request.UserID, closeContext.AccountID, closeContext.OrderID)
	if reasonErr != nil {
		return CloseOrderResult{}, reasonErr
	}
	// reasonAllowed 表示所选值仍存在于平台最新原因列表。
	reasonAllowed := false
	// reason 是当前待匹配的平台原因。
	for _, reason := range platformReasons.Reasons {
		if reason == selectedReason {
			reasonAllowed = true
			break
		}
	}
	if !reasonAllowed {
		return CloseOrderResult{}, ErrCloseOrderReasonInvalid
	}
	// latestContext、latestErr 在真实写入前再次排除刚到达的发货、退款或关闭终态。
	latestContext, latestErr := service.validateContext(ctx, request.UserID, closeContext.AccountID, closeContext.OrderID)
	if latestErr != nil {
		return CloseOrderResult{}, latestErr
	}
	// runKey 对同账号同订单永久唯一，不因关闭原因变化而允许重复关单。
	runKey := CloseOrderTaskType + ":" + latestContext.AccountID + ":" + latestContext.OrderID
	// claimed、claimErr 是真实关单的原子抢占结果。
	claimed, claimErr := service.repository.ClaimCloseOrder(ctx, runKey, latestContext.AccountID, latestContext.OrderID, service.now().UTC().Unix())
	if claimErr != nil {
		return CloseOrderResult{}, claimErr
	}
	if !claimed {
		return CloseOrderResult{}, ErrCloseOrderAlreadyHandled
	}
	// detail、credentialErr 是 submit 前短时锁内读取的平台凭证。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, latestContext.AccountID)
	if credentialErr != nil {
		_ = service.finish(ctx, runKey, "failed", 0, 1, credentialErr.Error())
		return CloseOrderResult{}, credentialErr
	}
	// submitCtx、cancel 限制真实关单平台请求时间。
	submitCtx, cancel := context.WithTimeout(ctx, closeOrderRequestTimeout)
	defer cancel()
	// platformResult、submitErr 是平台真实关单结果和错误。
	platformResult, submitErr := service.runtime.CloseOrderBySeller(submitCtx, detail, latestContext.OrderID, selectedReason)
	// credentialWarning 是响应 Cookie 未能协调的非动作失败告警。
	credentialWarning := service.persistCredential(ctx, request.UserID, latestContext.AccountID, detail, closeOrderCookieResultFromClose(platformResult))
	if submitErr != nil {
		if service.runtime.IsSessionExpired(submitErr) {
			service.runtime.RecoverExpiredSession(ctx, latestContext.AccountID, submitErr)
			_ = service.finish(ctx, runKey, "failed", 0, 1, submitErr.Error())
			return CloseOrderResult{}, submitErr
		}
		// finishErr 是不明确远端结果的隔离状态写入错误。
		finishErr := service.finish(ctx, runKey, "needs_review", 0, 1, submitErr.Error())
		if finishErr != nil {
			return CloseOrderResult{}, errors.Join(ErrCloseOrderNeedsReview, finishErr)
		}
		return CloseOrderResult{Status: "needs_review", Message: ErrCloseOrderNeedsReview.Error(), OrderID: latestContext.OrderID}, ErrCloseOrderNeedsReview
	}
	if platformResult == nil || !platformResult.Success {
		// message 是平台明确拒绝时保存和展示的说明。
		message := "闲鱼未确认订单关闭成功"
		if platformResult != nil && strings.TrimSpace(platformResult.Message) != "" {
			message = strings.TrimSpace(platformResult.Message)
		}
		_ = service.finish(ctx, runKey, "failed", 0, 1, message)
		return CloseOrderResult{Status: "failed", Message: message, OrderID: latestContext.OrderID}, fmt.Errorf("%w: %s", ErrCloseOrderNotEligible, message)
	}
	// status、message 区分纯成功和凭证并发变化警告。
	status, message := "succeeded", strings.TrimSpace(platformResult.Message)
	if message == "" {
		message = "订单已关闭，等待闲鱼系统卡片同步"
	}
	if credentialWarning != nil {
		status = "succeeded_with_warning"
		message += "；账号凭证已发生变化，未覆盖较新的登录状态"
	}
	if // finishErr 是平台明确成功后的本地幂等终态写入错误。
	finishErr := service.finish(ctx, runKey, "success", 1, 0, credentialErrorText(credentialWarning)); finishErr != nil {
		return CloseOrderResult{Success: true, Status: "needs_review", Message: "闲鱼已关闭订单，但本地状态保存失败，请勿重复提交", OrderID: latestContext.OrderID}, nil
	}
	return CloseOrderResult{Success: true, Status: status, Message: message, OrderID: latestContext.OrderID}, nil
}

// validateContext 校验服务依赖、数字订单号和结构化可取消卡片归属。
func (service *CloseOrderService) validateContext(ctx context.Context, userID int64, accountID, orderID string) (*CloseOrderContext, error) {
	if service == nil || service.repository == nil || service.runtime == nil {
		return nil, errors.New("关闭订单依赖未初始化")
	}
	// normalizedAccountID、normalizedOrderID 是去空白后的卖家账号和平台订单号。
	normalizedAccountID, normalizedOrderID := strings.TrimSpace(accountID), strings.TrimSpace(orderID)
	if normalizedAccountID == "" || !closeOrderIDPattern.MatchString(normalizedOrderID) {
		return nil, NewValidationError("关闭订单需要有效账号和数字订单号")
	}
	if !service.runtime.CloseOrderAvailable() {
		return nil, ErrCloseOrderUnavailable
	}
	// closeContext、readErr 是 repository 返回的可取消订单卡片上下文和错误。
	closeContext, readErr := service.repository.GetPendingCloseOrder(ctx, userID, normalizedAccountID, normalizedOrderID)
	if errors.Is(readErr, ErrNotFound) {
		return nil, ErrCloseOrderNotEligible
	}
	if readErr != nil {
		return nil, readErr
	}
	return closeContext, nil
}

// fetchReasons 在短时凭证读取后调用平台动态原因，并按代次协调响应 Cookie。
func (service *CloseOrderService) fetchReasons(ctx context.Context, userID int64, accountID, orderID string) (*CloseOrderPlatformReasons, error) {
	// detail、credentialErr 是平台原因请求使用的凭证快照。
	detail, credentialErr := service.loadCredential(ctx, userID, accountID)
	if credentialErr != nil {
		return nil, credentialErr
	}
	// requestCtx、cancel 限制只读平台原因请求时间。
	requestCtx, cancel := context.WithTimeout(ctx, closeOrderRequestTimeout)
	defer cancel()
	// reasons、reasonErr 是平台动态原因和错误。
	reasons, reasonErr := service.runtime.FetchCloseOrderReasons(requestCtx, detail, orderID)
	// credentialWarning 是只读响应 Cookie 协调错误。
	credentialWarning := service.persistCredential(ctx, userID, accountID, detail, closeOrderCookieResultFromReasons(reasons))
	if reasonErr != nil {
		if service.runtime.IsSessionExpired(reasonErr) {
			service.runtime.RecoverExpiredSession(ctx, accountID, reasonErr)
		}
		return nil, reasonErr
	}
	if credentialWarning != nil {
		return nil, fmt.Errorf("读取关闭原因后账号凭证已变化: %w", credentialWarning)
	}
	if reasons == nil || len(reasons.Reasons) == 0 {
		return nil, errors.New("闲鱼未返回关闭原因")
	}
	return reasons, nil
}

// loadCredential 在短时账号锁内读取并复核平台凭证，不持锁执行网络请求。
func (service *CloseOrderService) loadCredential(ctx context.Context, userID int64, accountID string) (*PlatformRuntimeData, error) {
	// unlock 是账号凭证短时读取锁释放函数。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// detail、loadErr 是权威平台凭证视图及其错误。
	detail, loadErr := service.repository.LoadCookiePlatformDetail(ctx, accountID)
	if loadErr != nil {
		return nil, loadErr
	}
	if detail == nil || detail.UserID != userID || !service.runtime.CredentialAvailable(detail) {
		return nil, errors.New("账号凭证已变化，请重新登录后再关闭订单")
	}
	return detail, nil
}

// closeOrderCookieResult 抽象原因／关单共同的 Cookie 会话结果。
type closeOrderCookieResult struct {
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// closeOrderCookieResultFromReasons 将原因结果转换为凭证协调输入。
func closeOrderCookieResultFromReasons(result *CloseOrderPlatformReasons) *closeOrderCookieResult {
	if result == nil {
		return nil
	}
	return &closeOrderCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// closeOrderCookieResultFromClose 将关单结果转换为凭证协调输入。
func closeOrderCookieResultFromClose(result *CloseOrderPlatformResult) *closeOrderCookieResult {
	if result == nil {
		return nil
	}
	return &closeOrderCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// persistCredential 仅在外部调用期间凭证代次未变化时保存响应 Cookie。
func (service *CloseOrderService) persistCredential(ctx context.Context, userID int64, accountID string, before *PlatformRuntimeData, result *closeOrderCookieResult) error {
	if before == nil || result == nil {
		return nil
	}
	// unlock 是平台调用后重新获取的凭证写回锁。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// latest、reloadErr 是外部调用完成后的权威凭证视图和错误。
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
	if // updateErr 是兼容平面 Cookie 响应写回错误。
	updateErr := service.repository.UpdateRenewalCookie(ctx, accountID, result.UpdatedCookies, before.MetadataJSON, time.Now().Unix()); updateErr != nil {
		return updateErr
	}
	service.runtime.UpdateRunningCookie(ctx, accountID, result.UpdatedCookies)
	return nil
}

// finish 保存关单运行终态。
func (service *CloseOrderService) finish(ctx context.Context, runKey, status string, success, failed int, message string) error {
	return service.repository.FinishCloseOrder(ctx, runKey, status, success, failed, message)
}
