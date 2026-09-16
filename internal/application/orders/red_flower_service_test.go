package orders

import (
	"context"
	"errors"
	"testing"
	"time"
)

// redFlowerRepositoryFake 保存手动求花服务测试使用的订单、凭证和运行终态。
type redFlowerRepositoryFake struct {
	// order 是测试读取的订单。
	order *Order
	// owned 表示订单账号是否属于当前用户。
	owned bool
	// claim 表示幂等运行是否抢占成功。
	claim bool
	// details 是连续凭证读取返回的账号视图。
	details []*PlatformRuntimeData
	// detailIndex 是下一次凭证读取位置。
	detailIndex int
	// platformCallsBeforeFinish 不参与生产逻辑，仅由测试运行时写入以验证顺序。
	platformCallsBeforeFinish int
	// finishStatus 保存最后写入的运行终态。
	finishStatus string
	// finishMessage 保存最后写入的非敏感说明。
	finishMessage string
	// finishErr 是运行终态持久化错误。
	finishErr error
	// updatedCookie 保存兼容平面 Cookie 写回值。
	updatedCookie string
	// storedRun 是状态查询返回的求花运行。
	storedRun RedFlowerRequestRun
	// runExists 表示状态查询是否存在运行记录。
	runExists bool
	// runErr 是状态查询错误。
	runErr error
}

// GetOrder 返回测试订单。
func (repository *redFlowerRepositoryFake) GetOrder(context.Context, string) (*Order, error) {
	return repository.order, nil
}

// ExistsOwned 返回测试账号归属结果。
func (repository *redFlowerRepositoryFake) ExistsOwned(context.Context, int64, string) (bool, error) {
	return repository.owned, nil
}

// ClaimRedFlowerRequest 返回预置幂等抢占结果。
func (repository *redFlowerRepositoryFake) ClaimRedFlowerRequest(context.Context, string, string, string, int64) (bool, error) {
	return repository.claim, nil
}

// FinishRedFlowerRequest 记录运行终态。
func (repository *redFlowerRepositoryFake) FinishRedFlowerRequest(_ context.Context, _ string, status string, _ int, _ int, message string) error {
	repository.finishStatus = status
	repository.finishMessage = message
	return repository.finishErr
}

// GetRedFlowerRequestRun 返回预置求花运行状态。
func (repository *redFlowerRepositoryFake) GetRedFlowerRequestRun(context.Context, string) (RedFlowerRequestRun, bool, error) {
	return repository.storedRun, repository.runExists, repository.runErr
}

// LockCredentials 返回无状态释放函数，测试凭证代次由 details 序列控制。
func (repository *redFlowerRepositoryFake) LockCredentials(string) func() {
	return func() {}
}

// LoadCookiePlatformDetail 按顺序返回测试凭证视图。
func (repository *redFlowerRepositoryFake) LoadCookiePlatformDetail(context.Context, string) (*PlatformRuntimeData, error) {
	if len(repository.details) == 0 {
		return nil, errors.New("测试未配置凭证")
	}
	// index 是不超过最后一项的当前凭证位置。
	index := repository.detailIndex
	if index >= len(repository.details) {
		index = len(repository.details) - 1
	}
	repository.detailIndex++
	return repository.details[index], nil
}

// UpdateRenewalCookie 记录平面 Cookie 写回值。
func (repository *redFlowerRepositoryFake) UpdateRenewalCookie(_ context.Context, _ string, value, _ string, _ int64) error {
	repository.updatedCookie = value
	return nil
}

// redFlowerRuntimeFake 保存平台求花和 Cookie 会话协调测试结果。
type redFlowerRuntimeFake struct {
	// available 表示平台求花能力是否可用。
	available bool
	// result 是平台求花结果。
	result *RedFlowerPlatformResult
	// requestErr 是平台请求错误。
	requestErr error
	// calls 统计平台求花调用次数。
	calls int
	// updatedRunningCookie 保存同步到账号运行时的 Cookie。
	updatedRunningCookie string
	// recovered 表示会话恢复是否被调用。
	recovered bool
}

// RedFlowerAvailable 返回平台求花能力状态。
func (runtime *redFlowerRuntimeFake) RedFlowerAvailable() bool {
	return runtime.available
}

// CredentialAvailable 判断测试凭证是否含非空值。
func (runtime *redFlowerRuntimeFake) CredentialAvailable(detail *PlatformRuntimeData) bool {
	return detail != nil && detail.Value != ""
}

// RequestRedFlower 返回预置平台结果。
func (runtime *redFlowerRuntimeFake) RequestRedFlower(context.Context, *PlatformRuntimeData, string, string) (*RedFlowerPlatformResult, error) {
	runtime.calls++
	return runtime.result, runtime.requestErr
}

// PersistCookieSession 表示测试未使用完整 Cookie Jar 写回。
func (runtime *redFlowerRuntimeFake) PersistCookieSession(_ context.Context, detail *PlatformRuntimeData, _ RefreshCookieUpdate) (string, bool, bool, error) {
	return detail.Value, false, false, nil
}

// UpdateRunningCookie 记录运行时 Cookie 同步。
func (runtime *redFlowerRuntimeFake) UpdateRunningCookie(_ context.Context, _ string, value string) {
	runtime.updatedRunningCookie = value
}

// RecoverExpiredSession 记录会话恢复调用。
func (runtime *redFlowerRuntimeFake) RecoverExpiredSession(context.Context, string, error) bool {
	runtime.recovered = true
	return true
}

// IsSessionExpired 只把测试哨兵错误判为会话失效。
func (runtime *redFlowerRuntimeFake) IsSessionExpired(err error) bool {
	return err != nil && err.Error() == "session expired"
}

// redFlowerEligibleOrder 创建付款一天且状态已完成的测试订单。
func redFlowerEligibleOrder(now time.Time) *Order {
	return &Order{OrderID: "order-1", CookieID: "account-1", OrderStatus: "completed", PaidAt: now.Add(-24 * time.Hour).Format(time.RFC3339Nano)}
}

// TestRedFlowerRequestServiceSuccess 验证明确成功会写入幂等终态且不重复修改凭证。
func TestRedFlowerRequestServiceSuccess(t *testing.T) {
	// now 是本用例固定时间。
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	// detail 是平台调用前后的同一凭证代次。
	detail := &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "synthetic-cookie", MetadataJSON: "{}"}
	// repository 保存订单、凭证和幂等状态。
	repository := &redFlowerRepositoryFake{order: redFlowerEligibleOrder(now), owned: true, claim: true, details: []*PlatformRuntimeData{detail, detail}}
	// runtime 返回平台明确成功结果。
	runtime := &redFlowerRuntimeFake{available: true, result: &RedFlowerPlatformResult{Success: true, Message: "求花成功"}}
	// service 是固定时间的待测求花服务。
	service := NewRedFlowerRequestService(repository, runtime, RedFlowerRequestOptions{Now: func() time.Time { return now }})
	// result、requestErr 是求花结果和错误。
	result, requestErr := service.Request(context.Background(), RedFlowerRequest{UserID: 7, OrderID: "order-1"})
	if requestErr != nil || !result.Success || result.Status != "succeeded" || runtime.calls != 1 || repository.finishStatus != "success" {
		t.Fatalf("result=%+v runtime=%+v repository=%+v err=%v", result, runtime, repository, requestErr)
	}
}

// TestRedFlowerRequestServiceRejectsIneligibleAndDuplicate 验证资格与幂等校验发生在平台调用前。
func TestRedFlowerRequestServiceRejectsIneligibleAndDuplicate(t *testing.T) {
	// now 是资格边界固定时间。
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	// runtime 记录所有不应发生的平台调用。
	runtime := &redFlowerRuntimeFake{available: true, result: &RedFlowerPlatformResult{Success: true}}
	// expiredOrder 是付款超过三十天的已完成订单。
	expiredOrder := redFlowerEligibleOrder(now)
	expiredOrder.PaidAt = now.Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	// expiredService 验证过期订单不会抢占或调用平台。
	expiredService := NewRedFlowerRequestService(&redFlowerRepositoryFake{order: expiredOrder, owned: true, claim: true}, runtime, RedFlowerRequestOptions{Now: func() time.Time { return now }})
	// requestErr 是过期订单资格校验错误。
	if _, requestErr := expiredService.Request(context.Background(), RedFlowerRequest{UserID: 7, OrderID: "order-1"}); !errors.Is(requestErr, ErrRedFlowerExpired) {
		t.Fatalf("expired err=%v", requestErr)
	}
	// detail 是重复动作场景不会实际使用的平台凭证。
	detail := &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "synthetic-cookie"}
	// duplicateService 使用 claim=false 模拟既有成功或人工核对运行。
	duplicateService := NewRedFlowerRequestService(&redFlowerRepositoryFake{order: redFlowerEligibleOrder(now), owned: true, claim: false, details: []*PlatformRuntimeData{detail}}, runtime, RedFlowerRequestOptions{Now: func() time.Time { return now }})
	// requestErr 是重复订单的幂等拒绝错误。
	if _, requestErr := duplicateService.Request(context.Background(), RedFlowerRequest{UserID: 7, OrderID: "order-1"}); !errors.Is(requestErr, ErrRedFlowerAlreadyHandled) {
		t.Fatalf("duplicate err=%v", requestErr)
	}
	if runtime.calls != 0 {
		t.Fatalf("不合格或重复订单不应调用平台，calls=%d", runtime.calls)
	}
}

// TestRedFlowerEligibilityAllowsReceivedButRejectsRefundStates 验证收货后仍可求花，退款中和已退款不会创建外部动作。
func TestRedFlowerEligibilityAllowsReceivedButRejectsRefundStates(t *testing.T) {
	// now 是资格判断使用的固定时间。
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	// received 是付款一天且买家已确认收货的订单。
	received := &Order{OrderID: "received-order", CookieID: "account-1", OrderStatus: "received", PaidAt: now.Add(-24 * time.Hour).Format(time.RFC3339Nano)}
	if // eligibleErr 是已收货订单的求花资格结果。
	eligibleErr := validateRedFlowerEligibility(received, now); eligibleErr != nil {
		t.Fatalf("received eligibility err=%v", eligibleErr)
	}
	// status 是当前必须被退款门禁拒绝的订单状态。
	for _, status := range []string{"refunding", "refunded", "cancelled"} {
		// blocked 是当前退款或取消状态的订单。
		blocked := *received
		blocked.OrderStatus = status
		if // blockedErr 是当前终态的资格拒绝结果。
		blockedErr := validateRedFlowerEligibility(&blocked, now); !errors.Is(blockedErr, ErrRedFlowerNotEligible) {
			t.Fatalf("status=%s err=%v", status, blockedErr)
		}
	}
}

// TestRedFlowerRequestServiceQuarantinesAmbiguousResult 验证平台错误进入人工核对并阻止自动重放。
func TestRedFlowerRequestServiceQuarantinesAmbiguousResult(t *testing.T) {
	// now 是本用例固定时间。
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	// detail 是平台调用前后的同一凭证代次。
	detail := &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "synthetic-cookie"}
	// repository 记录不明确结果的隔离终态。
	repository := &redFlowerRepositoryFake{order: redFlowerEligibleOrder(now), owned: true, claim: true, details: []*PlatformRuntimeData{detail, detail}}
	// runtime 模拟可能发生在请求发送后的网络断开。
	runtime := &redFlowerRuntimeFake{available: true, result: &RedFlowerPlatformResult{}, requestErr: errors.New("connection reset")}
	// service 是待测求花服务。
	service := NewRedFlowerRequestService(repository, runtime, RedFlowerRequestOptions{Now: func() time.Time { return now }})
	// _, requestErr 是必须映射为人工核对的调用结果。
	_, requestErr := service.Request(context.Background(), RedFlowerRequest{UserID: 7, OrderID: "order-1"})
	if !errors.Is(requestErr, ErrRedFlowerNeedsReview) || repository.finishStatus != "needs_review" {
		t.Fatalf("status=%q message=%q err=%v", repository.finishStatus, repository.finishMessage, requestErr)
	}
}

// TestRedFlowerRequestServicePreservesNewerCredentialAfterSuccess 验证并发扫码凭证胜过求花响应 Cookie。
func TestRedFlowerRequestServicePreservesNewerCredentialAfterSuccess(t *testing.T) {
	// now 是本用例固定时间。
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	// before 是求花请求开始时的旧账号凭证。
	before := &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "generation-before", MetadataJSON: "{}"}
	// after 是求花请求期间扫码写入的新账号凭证。
	after := &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "generation-after", MetadataJSON: "{}"}
	// repository 按调用顺序返回旧、新凭证。
	repository := &redFlowerRepositoryFake{order: redFlowerEligibleOrder(now), owned: true, claim: true, details: []*PlatformRuntimeData{before, after}}
	// runtime 返回平台明确成功以及基于旧凭证的响应值。
	runtime := &redFlowerRuntimeFake{available: true, result: &RedFlowerPlatformResult{Success: true, UpdatedCookies: "response-from-before"}}
	// service 是待测求花服务。
	service := NewRedFlowerRequestService(repository, runtime, RedFlowerRequestOptions{Now: func() time.Time { return now }})
	// result、requestErr 是明确成功但凭证不覆盖的新代次结果。
	result, requestErr := service.Request(context.Background(), RedFlowerRequest{UserID: 7, OrderID: "order-1"})
	if requestErr != nil || !result.Success || result.Status != "succeeded_with_warning" || repository.updatedCookie != "" || runtime.updatedRunningCookie != "" {
		t.Fatalf("result=%+v repository=%+v runtime=%+v err=%v", result, repository, runtime, requestErr)
	}
}

// TestRedFlowerRequestServiceStatusPersistsAcrossPageReload 验证详情状态可在前端状态丢失后从幂等运行恢复。
func TestRedFlowerRequestServiceStatusPersistsAcrossPageReload(t *testing.T) {
	// now 是状态夹具的固定时间。
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	// repository 保存已成功求花的持久运行。
	repository := &redFlowerRepositoryFake{
		order: redFlowerEligibleOrder(now), owned: true, runExists: true,
		storedRun: RedFlowerRequestRun{Status: "success", StartedAt: now.Unix(), FinishedAt: now.Unix()},
	}
	// service 只读取状态，不需要平台运行时。
	service := NewRedFlowerRequestService(repository, nil, RedFlowerRequestOptions{Now: func() time.Time { return now }})
	// result、statusErr 是订单详情持久求花状态和错误。
	result, statusErr := service.Status(context.Background(), 7, "order-1")
	if statusErr != nil || result.Status != "succeeded" || result.RequestedAt != now.Unix() || result.Message == "" {
		t.Fatalf("result=%+v err=%v", result, statusErr)
	}
	// missingRepository 模拟尚未求花订单。
	missingRepository := &redFlowerRepositoryFake{order: redFlowerEligibleOrder(now), owned: true}
	// missingStatus、missingErr 是未求花订单的安全默认状态。
	missingStatus, missingErr := NewRedFlowerRequestService(missingRepository, nil, RedFlowerRequestOptions{}).Status(context.Background(), 7, "order-1")
	if missingErr != nil || missingStatus.Status != "not_requested" {
		t.Fatalf("missing=%+v err=%v", missingStatus, missingErr)
	}
}
