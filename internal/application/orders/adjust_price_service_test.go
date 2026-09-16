package orders

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// priceAdjustmentRepositoryFake 隔离卡片归属、运行和凭证持久化。
type priceAdjustmentRepositoryFake struct {
	// adjustment 是测试返回的待付款卡片上下文。
	adjustment *PriceAdjustmentContext
	// detail 是测试平台凭证视图。
	detail *PlatformRuntimeData
	// claimed 控制真实 submit 幂等抢占结果。
	claimed bool
	// finishedStatus、finishedMessage 记录应用服务写入的终态。
	finishedStatus, finishedMessage string
}

// GetPendingPriceAdjustment 返回预置待付款卡片。
func (fake *priceAdjustmentRepositoryFake) GetPendingPriceAdjustment(context.Context, int64, string, string) (*PriceAdjustmentContext, error) {
	if fake.adjustment == nil {
		return nil, ErrNotFound
	}
	return fake.adjustment, nil
}

// ClaimPriceAdjustment 返回预置幂等抢占结果。
func (fake *priceAdjustmentRepositoryFake) ClaimPriceAdjustment(context.Context, string, string, string, int64) (bool, error) {
	return fake.claimed, nil
}

// FinishPriceAdjustment 记录运行终态。
func (fake *priceAdjustmentRepositoryFake) FinishPriceAdjustment(_ context.Context, _ string, status string, _ int, _ int, message string) error {
	fake.finishedStatus, fake.finishedMessage = status, message
	return nil
}

// LockCredentials 返回无竞争测试锁释放函数。
func (*priceAdjustmentRepositoryFake) LockCredentials(string) func() { return func() {} }

// LoadCookiePlatformDetail 返回预置凭证视图。
func (fake *priceAdjustmentRepositoryFake) LoadCookiePlatformDetail(context.Context, string) (*PlatformRuntimeData, error) {
	return fake.detail, nil
}

// UpdateRenewalCookie 在测试中不写真实凭证。
func (*priceAdjustmentRepositoryFake) UpdateRenewalCookie(context.Context, string, string, string, int64) error {
	return nil
}

// priceAdjustmentRuntimeFake 隔离 render／submit 平台行为。
type priceAdjustmentRuntimeFake struct {
	// form 是测试 render 返回的动态表单。
	form *PriceAdjustmentPlatformForm
	// submitResult、submitErr 是测试 submit 结果和错误。
	submitResult *PriceAdjustmentPlatformResult
	submitErr    error
	// submitted 保存应用服务最终传给平台的整数分字段。
	submitted map[string]string
}

// PriceAdjustmentAvailable 表示测试客户端具备改价能力。
func (*priceAdjustmentRuntimeFake) PriceAdjustmentAvailable() bool { return true }

// CredentialAvailable 判断测试凭证是否存在。
func (*priceAdjustmentRuntimeFake) CredentialAvailable(detail *PlatformRuntimeData) bool {
	return detail != nil && detail.Value != ""
}

// RenderOrderAdjustPrice 返回预置动态表单。
func (fake *priceAdjustmentRuntimeFake) RenderOrderAdjustPrice(context.Context, *PlatformRuntimeData, string) (*PriceAdjustmentPlatformForm, error) {
	return fake.form, nil
}

// SubmitOrderAdjustPrice 记录整数分字段并返回预置结果。
func (fake *priceAdjustmentRuntimeFake) SubmitOrderAdjustPrice(_ context.Context, _ *PlatformRuntimeData, _ string, fields map[string]string) (*PriceAdjustmentPlatformResult, error) {
	fake.submitted = fields
	return fake.submitResult, fake.submitErr
}

// PersistCookieSession 表示测试响应没有 Cookie 变化。
func (*priceAdjustmentRuntimeFake) PersistCookieSession(_ context.Context, detail *PlatformRuntimeData, _ RefreshCookieUpdate) (string, bool, bool, error) {
	return detail.Value, false, true, nil
}

// UpdateRunningCookie 在测试中不更新真实运行账号。
func (*priceAdjustmentRuntimeFake) UpdateRunningCookie(context.Context, string, string) {}

// RecoverExpiredSession 在测试中不启动真实恢复。
func (*priceAdjustmentRuntimeFake) RecoverExpiredSession(context.Context, string, error) bool {
	return false
}

// IsSessionExpired 在测试中不分类普通合成错误为 Session 失效。
func (*priceAdjustmentRuntimeFake) IsSessionExpired(error) bool { return false }

// priceAdjustmentTestForm 返回与真实抓包一致的动态字段。
func priceAdjustmentTestForm() *PriceAdjustmentPlatformForm {
	return &PriceAdjustmentPlatformForm{Title: "修改价格", Fields: []PriceAdjustmentField{
		{Key: "modifyFee", Name: "商品价格", PrefixText: "¥", Value: "0.10"},
		{Key: "newTransportFee", Name: "运费", PrefixText: "¥", Value: "0.00"},
	}}
}

// priceAdjustmentTestRepository 返回可执行真实 submit 的测试仓储。
func priceAdjustmentTestRepository() *priceAdjustmentRepositoryFake {
	return &priceAdjustmentRepositoryFake{adjustment: &PriceAdjustmentContext{AccountID: "account-1", ChatID: "chat-1", OrderID: "5127694777172175924", ItemID: "item-1"},
		detail: &PlatformRuntimeData{ID: "account-1", UserID: 7, Value: "test-cookie"}, claimed: true}
}

// TestPriceAdjustmentServiceUsesRenderFieldsAndIntegerCents 验证真实字段集合、金额分值和成功终态。
func TestPriceAdjustmentServiceUsesRenderFieldsAndIntegerCents(t *testing.T) {
	// repository、runtime 是隔离的应用依赖。
	repository, runtime := priceAdjustmentTestRepository(), &priceAdjustmentRuntimeFake{form: priceAdjustmentTestForm(), submitResult: &PriceAdjustmentPlatformResult{Success: true, Message: "价格修改成功"}}
	// service 使用固定时间保证运行键相关状态稳定。
	service := NewPriceAdjustmentService(repository, runtime, func() time.Time { return time.Unix(100, 0) })
	// result、adjustErr 是 0.10 元改为 0.20 元的应用结果。
	result, adjustErr := service.Adjust(context.Background(), PriceAdjustmentRequest{UserID: 7, AccountID: "account-1", OrderID: "5127694777172175924", Fields: []PriceAdjustmentInput{{Key: "modifyFee", Value: "0.20"}, {Key: "newTransportFee", Value: "0"}}})
	if adjustErr != nil || !result.Success || result.Status != "succeeded" || repository.finishedStatus != "success" {
		t.Fatalf("result=%+v status=%q err=%v", result, repository.finishedStatus, adjustErr)
	}
	// expected 是真实网页 submit 使用的整数分字符串。
	expected := map[string]string{"modifyFee": "20", "newTransportFee": "0"}
	if !reflect.DeepEqual(runtime.submitted, expected) {
		t.Fatalf("submitted=%v", runtime.submitted)
	}
}

// TestPriceAdjustmentServiceRejectsUnknownNoChangeAndDuplicate 验证动态字段注入、无变化和重复提交均在 submit 前停止。
func TestPriceAdjustmentServiceRejectsUnknownNoChangeAndDuplicate(t *testing.T) {
	// cases 是不允许产生平台 submit 的输入和期望错误。
	cases := []struct {
		// name 是当前边界用例名称。
		name string
		// fields 是当前用户提交的动态金额字段。
		fields []PriceAdjustmentInput
		// claim 控制幂等抢占结果。
		claim bool
		// want 是期望错误。
		want error
	}{
		{name: "unknown", fields: []PriceAdjustmentInput{{Key: "modifyFee", Value: "0.20"}, {Key: "newTransportFee", Value: "0"}, {Key: "orderId", Value: "1"}}, claim: true, want: ErrPriceAdjustmentNoChange},
		{name: "no-change", fields: []PriceAdjustmentInput{{Key: "modifyFee", Value: "0.10"}, {Key: "newTransportFee", Value: "0.00"}}, claim: true, want: ErrPriceAdjustmentNoChange},
		{name: "duplicate", fields: []PriceAdjustmentInput{{Key: "modifyFee", Value: "0.20"}, {Key: "newTransportFee", Value: "0"}}, claim: false, want: ErrPriceAdjustmentAlreadyHandled},
	}
	// testCase 是当前待执行边界用例。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// repository、runtime 是当前用例隔离依赖。
			repository, runtime := priceAdjustmentTestRepository(), &priceAdjustmentRuntimeFake{form: priceAdjustmentTestForm(), submitResult: &PriceAdjustmentPlatformResult{Success: true}}
			repository.claimed = testCase.claim
			// _, adjustErr 是当前被拒绝的改价结果和错误。
			_, adjustErr := NewPriceAdjustmentService(repository, runtime, nil).Adjust(context.Background(), PriceAdjustmentRequest{UserID: 7, AccountID: "account-1", OrderID: "5127694777172175924", Fields: testCase.fields})
			if testCase.name == "unknown" {
				if adjustErr == nil || runtime.submitted != nil {
					t.Fatalf("err=%v submitted=%v", adjustErr, runtime.submitted)
				}
				return
			}
			if !errors.Is(adjustErr, testCase.want) || runtime.submitted != nil {
				t.Fatalf("err=%v submitted=%v", adjustErr, runtime.submitted)
			}
		})
	}
}

// TestPriceAdjustmentServiceQuarantinesAmbiguousSubmit 验证传输错误进入人工核对且不重放。
func TestPriceAdjustmentServiceQuarantinesAmbiguousSubmit(t *testing.T) {
	// repository、runtime 是返回合成超时的隔离依赖。
	repository, runtime := priceAdjustmentTestRepository(), &priceAdjustmentRuntimeFake{form: priceAdjustmentTestForm(), submitErr: errors.New("synthetic timeout")}
	// result、adjustErr 是不明确 submit 的隔离结果。
	result, adjustErr := NewPriceAdjustmentService(repository, runtime, nil).Adjust(context.Background(), PriceAdjustmentRequest{UserID: 7, AccountID: "account-1", OrderID: "5127694777172175924", Fields: []PriceAdjustmentInput{{Key: "modifyFee", Value: "0.20"}, {Key: "newTransportFee", Value: "0"}}})
	if !errors.Is(adjustErr, ErrPriceAdjustmentNeedsReview) || result.Status != "needs_review" || repository.finishedStatus != "needs_review" {
		t.Fatalf("result=%+v status=%q err=%v", result, repository.finishedStatus, adjustErr)
	}
}
