package orders

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// RedFlowerRequestTaskType 是 account_task_runs 中手动求花动作的稳定任务类型。
	RedFlowerRequestTaskType = "red_flower_request"
	// redFlowerRequestWindow 是闲鱼官方允许付款订单求花的最长时间。
	redFlowerRequestWindow = 30 * 24 * time.Hour
)

var (
	// ErrRedFlowerNotEligible 表示订单状态或付款时间不满足求花条件。
	ErrRedFlowerNotEligible = errors.New("订单不满足求花条件")
	// ErrRedFlowerExpired 表示订单付款已经超过闲鱼官方三十天期限。
	ErrRedFlowerExpired = errors.New("订单付款已超过 30 天，不能求花")
	// ErrRedFlowerAlreadyHandled 表示同一订单已经求花、正在处理或处于人工核对状态。
	ErrRedFlowerAlreadyHandled = errors.New("该订单已求花或正在处理，请勿重复发送")
	// ErrRedFlowerUnavailable 表示当前平台客户端不具备官方求花能力。
	ErrRedFlowerUnavailable = errors.New("当前平台客户端不支持求花")
	// ErrRedFlowerNeedsReview 表示远端结果无法确定，必须人工核对且禁止重放。
	ErrRedFlowerNeedsReview = errors.New("求花结果不明确，已停止重复发送，请人工核对")
)

// RedFlowerRequest 描述一次由当前用户明确确认的订单求花动作。
type RedFlowerRequest struct {
	// UserID 是当前登录用户标识。
	UserID int64
	// OrderID 是待求花的卖家订单标识。
	OrderID string
	// Channel 是官方 H5 可选渠道；没有真实入口证据时保持空值。
	Channel string
}

// RedFlowerRequestResult 描述手动求花动作的确定性结果，不包含平台凭证。
type RedFlowerRequestResult struct {
	// Success 表示平台明确接受了求花动作。
	Success bool
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string
	// Message 是面向用户的结果说明。
	Message string
}

// RedFlowerRequestRun 是应用层读取的求花幂等运行状态。
type RedFlowerRequestRun struct {
	// Status 是数据库保存的运行终态。
	Status string
	// Message 是失败或人工核对说明。
	Message string
	// StartedAt 是动作开始的 Unix 秒时间戳。
	StartedAt int64
	// FinishedAt 是动作结束的 Unix 秒时间戳。
	FinishedAt int64
}

// RedFlowerRequestStatusResult 是订单详情使用的非敏感求花状态。
type RedFlowerRequestStatusResult struct {
	// Status 是 not_requested、running、succeeded、failed 或 needs_review。
	Status string
	// Message 是订单详情的持久化状态说明。
	Message string
	// RequestedAt 是首次平台动作开始的 Unix 秒时间戳。
	RequestedAt int64
}

// RedFlowerPlatformResult 保存平台调用结果和待协调 Cookie 会话。
type RedFlowerPlatformResult struct {
	// Success 表示平台 data.success 明确为真。
	Success bool
	// Message 是平台返回的非敏感业务提示。
	Message string
	// UpdatedCookies 是仅有扁平 Cookie 会话时的兼容返回值。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 请求的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// RedFlowerRequestRepository 定义手动求花所需的最小订单、凭证和幂等持久化能力。
type RedFlowerRequestRepository interface {
	// GetOrder 读取待求花订单。
	GetOrder(ctx context.Context, orderID string) (*Order, error)
	// ExistsOwned 判断订单账号是否属于当前用户。
	ExistsOwned(ctx context.Context, userID int64, cookieID string) (bool, error)
	// ClaimRedFlowerRequest 原子抢占首次或明确失败后的人工重试。
	ClaimRedFlowerRequest(ctx context.Context, runKey, cookieID, orderID string, now int64) (bool, error)
	// FinishRedFlowerRequest 保存求花动作终态。
	FinishRedFlowerRequest(ctx context.Context, runKey, status string, success, failed int, message string) error
	// GetRedFlowerRequestRun 读取指定订单的持久化求花状态。
	GetRedFlowerRequestRun(ctx context.Context, runKey string) (RedFlowerRequestRun, bool, error)
	// LockCredentials 串行化短时凭证读取和写回，不得覆盖外部网络调用。
	LockCredentials(cookieID string) func()
	// LoadCookiePlatformDetail 读取平台请求所需的账号凭证视图。
	LoadCookiePlatformDetail(ctx context.Context, cookieID string) (*PlatformRuntimeData, error)
	// UpdateRenewalCookie 保存只有扁平 Cookie 返回时的兼容凭证变化。
	UpdateRenewalCookie(ctx context.Context, cookieID, value, metadata string, at int64) error
}

// RedFlowerRequestRuntime 定义手动求花访问平台和协调 Cookie 会话的最小能力。
type RedFlowerRequestRuntime interface {
	// RedFlowerAvailable 判断平台客户端是否实现官方求花接口。
	RedFlowerAvailable() bool
	// CredentialAvailable 判断账号平台视图是否包含可用凭证。
	CredentialAvailable(detail *PlatformRuntimeData) bool
	// RequestRedFlower 调用官方求花接口，不负责业务幂等。
	RequestRedFlower(ctx context.Context, detail *PlatformRuntimeData, orderID, channel string) (*RedFlowerPlatformResult, error)
	// PersistCookieSession 保存完整 Cookie Jar 请求产生的会话变化。
	PersistCookieSession(ctx context.Context, detail *PlatformRuntimeData, update RefreshCookieUpdate) (string, bool, bool, error)
	// UpdateRunningCookie 同步已持久化的新 Cookie 到账号运行时。
	UpdateRunningCookie(ctx context.Context, cookieID, value string)
	// RecoverExpiredSession 处理平台明确返回的会话失效。
	RecoverExpiredSession(ctx context.Context, cookieID string, err error) bool
	// IsSessionExpired 判断错误是否属于明确会话失效。
	IsSessionExpired(err error) bool
}

// RedFlowerRequestOptions 保存求花服务的确定性时间和官方渠道配置。
type RedFlowerRequestOptions struct {
	// Now 返回当前时间；测试注入固定值，生产使用 time.Now。
	Now func() time.Time
	// Channel 是官方页面传入 MTOP 的可选渠道，未确认时保持空字符串。
	Channel string
}

// RedFlowerRequestService 承载订单归属、资格、幂等、平台动作和凭证代次协调。
type RedFlowerRequestService struct {
	// repository 保存手动求花需要的窄持久化端口。
	repository RedFlowerRequestRepository
	// runtime 保存平台请求和 Cookie 会话协调端口。
	runtime RedFlowerRequestRuntime
	// now 返回当前时间，用于三十天期限和运行记录。
	now func() time.Time
	// channel 是官方求花请求的可选渠道。
	channel string
}

// NewRedFlowerRequestService 创建手动求花应用服务。
// repository 和 runtime 是必需端口；options 只控制时间与已确认渠道，不改变业务边界。
func NewRedFlowerRequestService(repository RedFlowerRequestRepository, runtime RedFlowerRequestRuntime, options RedFlowerRequestOptions) *RedFlowerRequestService {
	// now 是归一化后的当前时间来源。
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &RedFlowerRequestService{repository: repository, runtime: runtime, now: now, channel: strings.TrimSpace(options.Channel)}
}

// Request 执行一次人工确认的卖家求花；任何不明确结果都会阻止同订单重放。
func (service *RedFlowerRequestService) Request(ctx context.Context, request RedFlowerRequest) (RedFlowerRequestResult, error) {
	if service == nil || service.repository == nil || service.runtime == nil {
		return RedFlowerRequestResult{}, errors.New("手动求花依赖未初始化")
	}
	// orderID 是去除空白后的平台订单标识。
	orderID := strings.TrimSpace(request.OrderID)
	if orderID == "" {
		return RedFlowerRequestResult{}, ErrNotFound
	}
	// order、orderErr 是待校验订单及其读取错误。
	order, orderErr := service.repository.GetOrder(ctx, orderID)
	if orderErr != nil {
		return RedFlowerRequestResult{}, orderErr
	}
	if order == nil {
		return RedFlowerRequestResult{}, ErrNotFound
	}
	// owned、ownershipErr 是订单账号归属结果和查询错误。
	owned, ownershipErr := service.repository.ExistsOwned(ctx, request.UserID, order.CookieID)
	if ownershipErr != nil {
		return RedFlowerRequestResult{}, ownershipErr
	}
	if !owned {
		return RedFlowerRequestResult{}, ErrForbidden
	}
	// now 是本次资格判断和运行记录共用的 UTC 时间。
	now := service.now().UTC()
	// eligibilityErr 是订单状态与付款期限校验错误。
	if eligibilityErr := validateRedFlowerEligibility(order, now); eligibilityErr != nil {
		return RedFlowerRequestResult{}, eligibilityErr
	}
	if !service.runtime.RedFlowerAvailable() {
		return RedFlowerRequestResult{}, ErrRedFlowerUnavailable
	}
	// runKey 是每账号、每订单唯一的手动求花幂等键。
	runKey := RedFlowerRequestTaskType + ":" + order.CookieID + ":" + orderID
	// claimed、claimErr 是本次动作的原子抢占结果和数据库错误。
	claimed, claimErr := service.repository.ClaimRedFlowerRequest(ctx, runKey, order.CookieID, orderID, now.Unix())
	if claimErr != nil {
		return RedFlowerRequestResult{}, claimErr
	}
	if !claimed {
		return RedFlowerRequestResult{}, ErrRedFlowerAlreadyHandled
	}
	// latest、credentialErr 是网络调用前短时锁内读取的权威凭证。
	latest, credentialErr := service.loadRedFlowerCredential(ctx, request.UserID, order.CookieID)
	if credentialErr != nil {
		_ = service.finish(ctx, runKey, "failed", 0, 1, credentialErr.Error(), false)
		return RedFlowerRequestResult{}, credentialErr
	}
	// requestCtx、cancel 限制单次官方求花请求最长执行三十秒。
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// platformResult、platformErr 是外部求花结果和错误；请求期间不持有凭证锁。
	platformResult, platformErr := service.runtime.RequestRedFlower(requestCtx, latest, orderID, firstNonEmptyRedFlowerChannel(request.Channel, service.channel))
	// credentialWarning 是平台调用后 Cookie 会话未写回的非动作失败告警。
	credentialWarning := service.persistRedFlowerCredential(ctx, request.UserID, order.CookieID, latest, platformResult)
	if platformErr != nil {
		if service.runtime.IsSessionExpired(platformErr) {
			service.runtime.RecoverExpiredSession(ctx, order.CookieID, platformErr)
			_ = service.finish(ctx, runKey, "failed", 0, 1, platformErr.Error(), false)
			return RedFlowerRequestResult{}, platformErr
		}
		// finishErr 是不明确外部结果的隔离状态写入错误；即使失败，running 状态也继续阻止重放。
		finishErr := service.finish(ctx, runKey, "needs_review", 0, 1, platformErr.Error(), true)
		if finishErr != nil {
			return RedFlowerRequestResult{}, errors.Join(ErrRedFlowerNeedsReview, finishErr)
		}
		return RedFlowerRequestResult{Status: "needs_review", Message: ErrRedFlowerNeedsReview.Error()}, ErrRedFlowerNeedsReview
	}
	if platformResult == nil || !platformResult.Success {
		// message 是平台明确拒绝时展示和记录的非敏感说明。
		message := "闲鱼未接受本次求花请求"
		if platformResult != nil && strings.TrimSpace(platformResult.Message) != "" {
			message = strings.TrimSpace(platformResult.Message)
		}
		_ = service.finish(ctx, runKey, "failed", 0, 1, message, false)
		return RedFlowerRequestResult{Status: "failed", Message: message}, fmt.Errorf("%w: %s", ErrRedFlowerNotEligible, message)
	}
	// successMessage 是平台明确成功后的用户提示。
	successMessage := "闲鱼已受理求花请求，系统卡片同步可能需要几分钟"
	// status 区分纯成功和凭证并发变化／写回失败警告。
	status := "succeeded"
	if credentialWarning != nil {
		status = "succeeded_with_warning"
		successMessage += "；账号凭证已发生变化，未覆盖较新的登录状态"
	}
	// finishErr 是平台明确成功后的幂等终态写入错误。
	if finishErr := service.finish(ctx, runKey, "success", 1, 0, credentialErrorText(credentialWarning), true); finishErr != nil {
		return RedFlowerRequestResult{Success: true, Status: "needs_review", Message: "闲鱼已发送求花卡片，但本地状态保存失败，请勿重复发送"}, nil
	}
	return RedFlowerRequestResult{Success: true, Status: status, Message: successMessage}, nil
}

// Status 读取订单的持久化求花状态，不调用闲鱼接口或修改运行记录。
func (service *RedFlowerRequestService) Status(ctx context.Context, userID int64, orderID string) (RedFlowerRequestStatusResult, error) {
	if service == nil || service.repository == nil {
		return RedFlowerRequestStatusResult{}, errors.New("手动求花 repository 未初始化")
	}
	// normalizedOrderID 是去除空白后的平台订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return RedFlowerRequestStatusResult{}, ErrNotFound
	}
	// order、orderErr 是待校验订单及其读取错误。
	order, orderErr := service.repository.GetOrder(ctx, normalizedOrderID)
	if orderErr != nil {
		return RedFlowerRequestStatusResult{}, orderErr
	}
	if order == nil {
		return RedFlowerRequestStatusResult{}, ErrNotFound
	}
	// owned、ownershipErr 是订单所属账号的当前用户归属结果。
	owned, ownershipErr := service.repository.ExistsOwned(ctx, userID, order.CookieID)
	if ownershipErr != nil {
		return RedFlowerRequestStatusResult{}, ownershipErr
	}
	if !owned {
		return RedFlowerRequestStatusResult{}, ErrForbidden
	}
	// runKey 是当前账号和订单对应的唯一求花幂等键。
	runKey := RedFlowerRequestTaskType + ":" + order.CookieID + ":" + normalizedOrderID
	// run、exists、runErr 是持久化运行、存在标记和查询错误。
	run, exists, runErr := service.repository.GetRedFlowerRequestRun(ctx, runKey)
	if runErr != nil {
		return RedFlowerRequestStatusResult{}, runErr
	}
	if !exists {
		return RedFlowerRequestStatusResult{Status: "not_requested"}, nil
	}
	return redFlowerStatusFromRun(run), nil
}

// redFlowerStatusFromRun 把数据库运行终态转换为订单详情稳定状态和提示。
func redFlowerStatusFromRun(run RedFlowerRequestRun) RedFlowerRequestStatusResult {
	// result 是默认进入人工核对的安全状态，未知值不会重新开放按钮。
	result := RedFlowerRequestStatusResult{Status: "needs_review", Message: "求花状态需要人工核对，请勿重复发送", RequestedAt: run.StartedAt}
	switch strings.TrimSpace(run.Status) {
	case "success":
		result.Status = "succeeded"
		result.Message = "已向买家求花，闲鱼系统卡片同步可能需要几分钟"
	case "running":
		result.Status = "running"
		result.Message = "求花请求正在处理，请勿重复点击"
	case "failed":
		result.Status = "failed"
		result.Message = strings.TrimSpace(run.Message)
		if result.Message == "" {
			result.Message = "上次求花失败，核对订单后可以重试"
		}
	case "needs_review":
		result.Status = "needs_review"
		result.Message = "求花结果待核对，请勿重复发送"
	}
	return result
}

// validateRedFlowerEligibility 验证订单已付款、状态允许且未超过三十天。
// order 是本地订单快照，now 是本次动作固定时间；返回值用于阻止不安全外部调用。
func validateRedFlowerEligibility(order *Order, now time.Time) error {
	if order == nil {
		return ErrNotFound
	}
	// status 是订单归一化后的卖家状态。
	status := NormalizeOrderStatus(order.OrderStatus)
	if status != "pending_ship" && status != "shipped" && status != "received" && status != "completed" {
		return fmt.Errorf("%w: 只有已付款、已发货或已完成订单可以求花", ErrRedFlowerNotEligible)
	}
	// paidAt、parsed 是订单付款时间和解析状态。
	paidAt, parsed := parseRedFlowerPaidAt(order.PaidAt)
	if !parsed {
		return fmt.Errorf("%w: 订单缺少可靠付款时间，请先同步订单", ErrRedFlowerNotEligible)
	}
	if paidAt.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("%w: 订单付款时间异常，请先同步订单", ErrRedFlowerNotEligible)
	}
	if now.Sub(paidAt) > redFlowerRequestWindow {
		return ErrRedFlowerExpired
	}
	return nil
}

// parseRedFlowerPaidAt 解析数据库现有的 RFC3339、SQL 时间或 Unix 时间文本。
// raw 是订单付款时间；返回值包含 UTC 时间和是否可以安全用于三十天判断。
func parseRedFlowerPaidAt(raw string) (time.Time, bool) {
	// normalized 是去除空白后的付款时间文本。
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return time.Time{}, false
	}
	// layout 是当前尝试的已知数据库时间格式。
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		// parsed、parseErr 是当前格式解析结果。
		if parsed, parseErr := time.Parse(layout, normalized); parseErr == nil {
			return parsed.UTC(), true
		}
	}
	// unixValue、unixErr 是兼容历史 Unix 秒或毫秒时间的数值结果。
	unixValue, unixErr := strconv.ParseInt(normalized, 10, 64)
	if unixErr != nil || unixValue <= 0 {
		return time.Time{}, false
	}
	if unixValue > 10_000_000_000 {
		unixValue /= 1000
	}
	return time.Unix(unixValue, 0).UTC(), true
}

// loadRedFlowerCredential 在短时账号锁内读取并复核平台凭证，不持锁执行网络请求。
func (service *RedFlowerRequestService) loadRedFlowerCredential(ctx context.Context, userID int64, cookieID string) (*PlatformRuntimeData, error) {
	// unlock 是账号凭证短时读取锁释放函数。
	unlock := service.repository.LockCredentials(cookieID)
	defer unlock()
	// detail、loadErr 是权威平台凭证视图及其读取错误。
	detail, loadErr := service.repository.LoadCookiePlatformDetail(ctx, cookieID)
	if loadErr != nil {
		return nil, loadErr
	}
	if detail == nil || detail.UserID != userID || !service.runtime.CredentialAvailable(detail) {
		return nil, errors.New("账号凭证已变化，请重新登录后再求花")
	}
	return detail, nil
}

// persistRedFlowerCredential 仅在外部调用期间凭证代次未变化时保存响应 Cookie。
// 返回错误只表示 Cookie 未协调，不改变平台已经明确返回的求花结果。
func (service *RedFlowerRequestService) persistRedFlowerCredential(ctx context.Context, userID int64, cookieID string, before *PlatformRuntimeData, result *RedFlowerPlatformResult) error {
	if before == nil || result == nil {
		return nil
	}
	// unlock 是平台调用后重新获取的凭证写回锁。
	unlock := service.repository.LockCredentials(cookieID)
	defer unlock()
	// latest、reloadErr 是外部调用完成后的权威凭证视图和读取错误。
	latest, reloadErr := service.repository.LoadCookiePlatformDetail(ctx, cookieID)
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
			service.runtime.UpdateRunningCookie(ctx, cookieID, value)
		}
		return nil
	}
	if strings.TrimSpace(result.UpdatedCookies) == "" || result.UpdatedCookies == before.Value {
		return nil
	}
	// updateErr 是兼容平面 Cookie 会话写回错误。
	if updateErr := service.repository.UpdateRenewalCookie(ctx, cookieID, result.UpdatedCookies, before.MetadataJSON, time.Now().Unix()); updateErr != nil {
		return updateErr
	}
	service.runtime.UpdateRunningCookie(ctx, cookieID, result.UpdatedCookies)
	return nil
}

// finish 保存手动求花运行终态；detached=true 时不受 HTTP 请求取消影响。
func (service *RedFlowerRequestService) finish(ctx context.Context, runKey, status string, success, failed int, message string, detached bool) error {
	// finishCtx 是终态写入使用的上下文。
	finishCtx := ctx
	// cancel 释放独立终态写入的五秒预算；普通失败分支保持空操作。
	cancel := func() {}
	if detached {
		finishCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	}
	defer cancel()
	return service.repository.FinishRedFlowerRequest(finishCtx, runKey, status, success, failed, message)
}

// firstNonEmptyRedFlowerChannel 返回请求显式渠道或组合期默认渠道。
func firstNonEmptyRedFlowerChannel(requestChannel, configuredChannel string) string {
	if strings.TrimSpace(requestChannel) != "" {
		return strings.TrimSpace(requestChannel)
	}
	return strings.TrimSpace(configuredChannel)
}

// credentialErrorText 返回不包含凭证内容的协调错误文本。
func credentialErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
