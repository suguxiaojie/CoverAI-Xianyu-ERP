package orders

import (
	"testing"
	"time"
)

// TestResolveOrderLifecycleStatusSeparatesReceiptCompletionAndRefund 验证主链不倒退，退款上下文不会被普通关闭码抹去。
func TestResolveOrderLifecycleStatusSeparatesReceiptCompletionAndRefund(t *testing.T) {
	// cases 保存当前状态、平台证据和预期状态。
	cases := []struct {
		// current 是写入前的本地状态。
		current string
		// incoming 是本轮平台订单列表、详情或聊天证据。
		incoming string
		// expected 是统一转换后的持久状态。
		expected string
	}{
		{"shipped", "received", "received"},
		{"received", "completed", "completed"},
		{"completed", "shipped", "completed"},
		{"completed", "refunding", "refunding"},
		{"refunding", "cancelled", "refunded"},
		{"shipped", "refunded", "refunded"},
		{"refunded", "completed", "refunded"},
		{"cancelled", "processing", "cancelled"},
		{"shipped", "买家已确认收货", "received"},
		{"refunding", "退款成功", "refunded"},
	}
	// testCase 是当前待验证的状态转换。
	for _, testCase := range cases {
		if // actual 是当前输入得到的统一订单状态。
		actual := ResolveOrderLifecycleStatus(testCase.current, testCase.incoming); actual != testCase.expected {
			t.Fatalf("current=%s incoming=%s actual=%s expected=%s", testCase.current, testCase.incoming, actual, testCase.expected)
		}
	}
}

// TestApplyOrderLifecycleMilestoneWritesOnlyExplicitStateTime 验证收货、完成、退款和取消时间互不冒充。
func TestApplyOrderLifecycleMilestoneWritesOnlyExplicitStateTime(t *testing.T) {
	// occurredAt 是四种里程碑共用的固定平台时间。
	occurredAt := time.Date(2026, 8, 20, 8, 30, 0, 0, time.UTC)
	// testCase 保存当前状态和读取对应时间字段的函数。
	for _, testCase := range []struct {
		// status 是待写入的明确生命周期状态。
		status string
		// read 返回该状态唯一允许写入的时间字段。
		read func(UpsertOptions) string
	}{
		{"received", func(options UpsertOptions) string { return options.ReceivedAt }},
		{"completed", func(options UpsertOptions) string { return options.CompletedAt }},
		{"refunded", func(options UpsertOptions) string { return options.RefundedAt }},
		{"cancelled", func(options UpsertOptions) string { return options.CancelledAt }},
	} {
		// options 保存当前状态产生的时间写入模型。
		options := UpsertOptions{}
		ApplyOrderLifecycleMilestone(&options, testCase.status, occurredAt)
		if testCase.read(options) != occurredAt.Format(time.RFC3339Nano) {
			t.Fatalf("status=%s options=%+v", testCase.status, options)
		}
	}
}

// TestResolveOrderLifecycleStatusUsesPersistedRefundEvidence 验证旧状态已被压成 cancelled 时，精确退款申请证据仍可纠正为 refunded。
func TestResolveOrderLifecycleStatusUsesPersistedRefundEvidence(t *testing.T) {
	if // actual 是存在退款申请证据时通用关闭码得到的正确终态。
	actual := ResolveOrderLifecycleStatusWithRefundContext("cancelled", "12", true); actual != "refunded" {
		t.Fatalf("refund context actual=%s", actual)
	}
	if // actual 是没有退款证据时必须保留的普通取消状态。
	actual := ResolveOrderLifecycleStatusWithRefundContext("cancelled", "12", false); actual != "cancelled" {
		t.Fatalf("cancel context actual=%s", actual)
	}
}
