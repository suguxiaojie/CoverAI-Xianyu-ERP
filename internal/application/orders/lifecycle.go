package orders

import "time"

// ResolveOrderLifecycleStatus 合并本地现态与平台新证据，返回不倒退且保留退款上下文的订单状态。
func ResolveOrderLifecycleStatus(current, incoming string) string {
	return ResolveOrderLifecycleStatusWithRefundContext(current, incoming, false)
}

// ResolveOrderLifecycleStatusWithRefundContext 在普通状态转换上叠加精确退款申请证据，解决详情接口只返回通用关闭码的情况。
func ResolveOrderLifecycleStatusWithRefundContext(current, incoming string, refundRequested bool) string {
	// currentStatus、incomingStatus 是统一数字码和文本别名后的订单状态。
	currentStatus, incomingStatus := NormalizeOrderStatus(current), NormalizeOrderStatus(incoming)
	if !ValidEditableOrderStatus(incomingStatus) {
		return currentStatus
	}
	if incomingStatus == "cancelled" && (currentStatus == "refunding" || refundRequested) {
		// 详情接口可能只返回通用关闭码；已经存在退款上下文时必须解释为退款完成。
		return "refunded"
	}
	if currentStatus == "unknown" || currentStatus == incomingStatus {
		return incomingStatus
	}
	if incomingStatus == "refunded" {
		return "refunded"
	}
	if currentStatus == "refunded" {
		return currentStatus
	}
	if currentStatus == "cancelled" {
		return currentStatus
	}
	if incomingStatus == "refunding" || incomingStatus == "cancelled" {
		return incomingStatus
	}
	// stage 保存正常交易主链的单调阶段，退款和取消分支已在上方单独处理。
	stage := map[string]int{"processing": 1, "pending_ship": 2, "shipped": 3, "received": 4, "completed": 5}
	// currentStage、currentKnown 表示当前状态是否属于正常交易主链。
	currentStage, currentKnown := stage[currentStatus]
	// incomingStage、incomingKnown 表示平台新状态是否属于正常交易主链。
	incomingStage, incomingKnown := stage[incomingStatus]
	if currentKnown && incomingKnown && incomingStage < currentStage {
		return currentStatus
	}
	return incomingStatus
}

// ApplyOrderLifecycleMilestone 把当前状态对应的明确事件时间填入写入模型；其他里程碑保持空值以免覆盖历史。
func ApplyOrderLifecycleMilestone(options *UpsertOptions, status string, occurredAt time.Time) {
	if options == nil {
		return
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	// eventTime 是跨方言持久化使用的统一 UTC RFC3339 时间。
	eventTime := occurredAt.UTC().Format(time.RFC3339Nano)
	switch NormalizeOrderStatus(status) {
	case "received":
		options.ReceivedAt = eventTime
	case "completed":
		options.CompletedAt = eventTime
	case "refunded":
		options.RefundedAt = eventTime
	case "cancelled":
		options.CancelledAt = eventTime
	}
}
