package orders

import (
	"context"
	"errors"
	"testing"
)

// closeOrderRepositoryFake 隔离卡片归属、运行和凭证持久化。
type closeOrderRepositoryFake struct {
	// closeContext 是测试返回的待付款或待发货取消上下文。
	closeContext *CloseOrderContext
	// detail 是测试平台凭证视图。
	detail *PlatformRuntimeData
	// claimed 控制真实关单幂等抢占结果。
	claimed bool
	// finishedStatus 记录应用服务写入的终态。
	finishedStatus string
}

// GetPendingCloseOrder 返回预置可取消订单卡片。
func (fake *closeOrderRepositoryFake) GetPendingCloseOrder(context.Context, int64, string, string) (*CloseOrderContext, error) {
	if fake.closeContext == nil {
		return nil, ErrNotFound
	}
	return fake.closeContext, nil
}

// ClaimCloseOrder 返回预置幂等抢占结果。
func (fake *closeOrderRepositoryFake) ClaimCloseOrder(context.Context, string, string, string, int64) (bool, error) {
	return fake.claimed, nil
}

// FinishCloseOrder 记录运行终态。
func (fake *closeOrderRepositoryFake) FinishCloseOrder(_ context.Context, _ string, status string, _ int, _ int, _ string) error {
	fake.finishedStatus = status
	return nil
}

// LockCredentials 返回无竞争测试锁释放函数。
func (*closeOrderRepositoryFake) LockCredentials(string) func() { return func() {} }

// LoadCookiePlatformDetail 返回预置凭证视图。
func (fake *closeOrderRepositoryFake) LoadCookiePlatformDetail(context.Context, string) (*PlatformRuntimeData, error) {
	return fake.detail, nil
}

// UpdateRenewalCookie 在测试中不写真实凭证。
func (*closeOrderRepositoryFake) UpdateRenewalCookie(context.Context, string, string, string, int64) error {
	return nil
}

// closeOrderRuntimeFake 隔离动态原因和卖家关单平台行为。
type closeOrderRuntimeFake struct {
	// reasons 是测试平台返回的动态原因。
	reasons []string
	// closeResult、closeErr 是真实关单替身结果和错误。
	closeResult *CloseOrderPlatformResult
	closeErr    error
	// closeCalls 统计具有外部写语义的关单调用次数。
	closeCalls int
}

// CloseOrderAvailable 表示测试客户端具备卖家关单能力。
func (*closeOrderRuntimeFake) CloseOrderAvailable() bool { return true }

// CredentialAvailable 判断测试凭证是否存在。
func (*closeOrderRuntimeFake) CredentialAvailable(detail *PlatformRuntimeData) bool {
	return detail != nil && detail.Value != ""
}

// FetchCloseOrderReasons 返回预置动态原因。
func (fake *closeOrderRuntimeFake) FetchCloseOrderReasons(context.Context, *PlatformRuntimeData, string) (*CloseOrderPlatformReasons, error) {
	return &CloseOrderPlatformReasons{Reasons: append([]string(nil), fake.reasons...)}, nil
}

// CloseOrderBySeller 记录调用并返回预置结果。
func (fake *closeOrderRuntimeFake) CloseOrderBySeller(context.Context, *PlatformRuntimeData, string, string) (*CloseOrderPlatformResult, error) {
	fake.closeCalls++
	return fake.closeResult, fake.closeErr
}

// PersistCookieSession 表示测试响应没有 Cookie 变化。
func (*closeOrderRuntimeFake) PersistCookieSession(_ context.Context, detail *PlatformRuntimeData, _ RefreshCookieUpdate) (string, bool, bool, error) {
	return detail.Value, false, true, nil
}

// UpdateRunningCookie 在测试中不更新真实运行账号。
func (*closeOrderRuntimeFake) UpdateRunningCookie(context.Context, string, string) {}

// RecoverExpiredSession 在测试中不启动真实恢复。
func (*closeOrderRuntimeFake) RecoverExpiredSession(context.Context, string, error) bool {
	return false
}

// IsSessionExpired 在测试中不分类普通合成错误为 Session 失效。
func (*closeOrderRuntimeFake) IsSessionExpired(error) bool { return false }

// closeOrderTestRepository 返回可执行取消订单的测试仓储。
func closeOrderTestRepository() *closeOrderRepositoryFake {
	return &closeOrderRepositoryFake{closeContext: &CloseOrderContext{AccountID: "account-1", ChatID: "chat-1", OrderID: "5127638256187075541", ItemID: "item-1", Stage: "pending_payment"}, detail: &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "test-cookie"}, claimed: true}
}

// TestCloseOrderEligibilityUsesOnlyLocalQualification 验证资格查询不调用平台，并把已失效订单安全返回为不可取消。
func TestCloseOrderEligibilityUsesOnlyLocalQualification(t *testing.T) {
	// eligibleRepository、runtime 是具备本地资格且不得发生平台调用的隔离依赖。
	eligibleRepository, runtime := closeOrderTestRepository(), &closeOrderRuntimeFake{}
	// eligibleResult、eligibleErr 是有效待付款订单的纯本地资格结果。
	eligibleResult, eligibleErr := NewCloseOrderService(eligibleRepository, runtime, nil).Eligibility(context.Background(), CloseOrderEligibilityRequest{UserID: 7, AccountID: " account-1 ", OrderID: " 5127638256187075541 "})
	if eligibleErr != nil || !eligibleResult.Eligible || eligibleResult.AccountID != "account-1" || eligibleResult.OrderID != "5127638256187075541" || eligibleResult.Stage != "pending_payment" || runtime.closeCalls != 0 {
		t.Fatalf("eligible=%+v calls=%d err=%v", eligibleResult, runtime.closeCalls, eligibleErr)
	}
	// paidRepository 把相同本地资格推进到已付款待发货阶段。
	paidRepository := closeOrderTestRepository()
	paidRepository.closeContext.Stage = "pending_ship"
	// paidResult、paidErr 是待发货订单继续可取消的纯本地资格结果。
	paidResult, paidErr := NewCloseOrderService(paidRepository, runtime, nil).Eligibility(context.Background(), CloseOrderEligibilityRequest{UserID: 7, AccountID: "account-1", OrderID: "5127638256187075541"})
	if paidErr != nil || !paidResult.Eligible || paidResult.Stage != "pending_ship" || runtime.closeCalls != 0 {
		t.Fatalf("paid=%+v calls=%d err=%v", paidResult, runtime.closeCalls, paidErr)
	}
	// ineligibleResult、ineligibleErr 是订单表或聊天终态已经失效时的稳定隐藏结果。
	ineligibleResult, ineligibleErr := NewCloseOrderService(&closeOrderRepositoryFake{}, runtime, nil).Eligibility(context.Background(), CloseOrderEligibilityRequest{UserID: 7, AccountID: "account-1", OrderID: "5127638256187075541"})
	if ineligibleErr != nil || ineligibleResult.Eligible || ineligibleResult.Reason == "" || runtime.closeCalls != 0 {
		t.Fatalf("ineligible=%+v calls=%d err=%v", ineligibleResult, runtime.closeCalls, ineligibleErr)
	}
}

// TestCloseOrderServiceUsesLatestReasonAndPersistsSuccess 验证动态原因、真实关单和成功幂等终态。
func TestCloseOrderServiceUsesLatestReasonAndPersistsSuccess(t *testing.T) {
	// repository、runtime 是隔离的应用依赖。
	repository, runtime := closeOrderTestRepository(), &closeOrderRuntimeFake{reasons: []string{"双方协商一致", "商品无货"}, closeResult: &CloseOrderPlatformResult{Success: true, Message: "订单已关闭"}}
	// result、closeErr 是使用有效平台原因的关单结果。
	result, closeErr := NewCloseOrderService(repository, runtime, nil).Close(context.Background(), CloseOrderRequest{UserID: 7, AccountID: "account-1", OrderID: "5127638256187075541", Reason: "双方协商一致"})
	if closeErr != nil || !result.Success || result.Status != "succeeded" || runtime.closeCalls != 1 || repository.finishedStatus != "success" {
		t.Fatalf("result=%+v calls=%d status=%q err=%v", result, runtime.closeCalls, repository.finishedStatus, closeErr)
	}
}

// TestCloseOrderServiceRejectsStaleReasonAndQuarantinesAmbiguousSubmit 验证失效原因不调用平台，超时结果进入人工核对。
func TestCloseOrderServiceRejectsStaleReasonAndQuarantinesAmbiguousSubmit(t *testing.T) {
	// invalidRepository、invalidRuntime 是原因已从平台列表移除的隔离依赖。
	invalidRepository, invalidRuntime := closeOrderTestRepository(), &closeOrderRuntimeFake{reasons: []string{"商品无货"}}
	// _, invalidErr 是失效原因的拒绝结果。
	_, invalidErr := NewCloseOrderService(invalidRepository, invalidRuntime, nil).Close(context.Background(), CloseOrderRequest{UserID: 7, AccountID: "account-1", OrderID: "5127638256187075541", Reason: "双方协商一致"})
	if !errors.Is(invalidErr, ErrCloseOrderReasonInvalid) || invalidRuntime.closeCalls != 0 {
		t.Fatalf("invalidErr=%v calls=%d", invalidErr, invalidRuntime.closeCalls)
	}
	// reviewRepository、reviewRuntime 是真实关单返回合成超时的隔离依赖。
	reviewRepository, reviewRuntime := closeOrderTestRepository(), &closeOrderRuntimeFake{reasons: []string{"商品无货"}, closeErr: errors.New("synthetic timeout")}
	// result、reviewErr 是不明确关单的隔离结果。
	result, reviewErr := NewCloseOrderService(reviewRepository, reviewRuntime, nil).Close(context.Background(), CloseOrderRequest{UserID: 7, AccountID: "account-1", OrderID: "5127638256187075541", Reason: "商品无货"})
	if !errors.Is(reviewErr, ErrCloseOrderNeedsReview) || result.Status != "needs_review" || reviewRepository.finishedStatus != "needs_review" || reviewRuntime.closeCalls != 1 {
		t.Fatalf("result=%+v status=%q calls=%d err=%v", result, reviewRepository.finishedStatus, reviewRuntime.closeCalls, reviewErr)
	}
}
