package orders

import "fmt"

// refreshWrite 保存详情分片中等待事务写入的单条订单。
type refreshWrite struct {
	// OrderID 是待更新订单标识。
	OrderID string
	// CurrentStatus 是刷新前订单状态。
	CurrentStatus string
	// NewStatus 是刷新后订单状态。
	NewStatus string
	// Options 是订单写入字段。
	Options UpsertOptions
	// CookieUpdate 是本次详情请求观察到的 Cookie 更新。
	CookieUpdate RefreshCookieUpdate
}

// refreshTarget 保存待补全详情的订单目标。
type refreshTarget struct {
	// OrderID 是待刷新订单标识。
	OrderID string
	// CurrentStatus 是本地当前订单状态。
	CurrentStatus string
	// CreatedAt 是本地已保存的平台订单创建时间，详情阶段必须原样保留。
	CreatedAt string
	// UpdatedAt 是本地最近写入时间，用于增量同步轮转补查售后可变订单。
	UpdatedAt string
	// RefundRequested 表示存在精确退款申请卡片，通用关闭码应解释为退款完成。
	RefundRequested bool
	// RequireSpec 表示历史成本补全必须取得非空规格值，否则本次详情应计为失败。
	RequireSpec bool
}

// refreshSoldOrderChanged 判断平台订单字段是否发生业务变化。
func refreshSoldOrderChanged(existing *Order, remote RefreshSoldOrder) bool {
	if existing == nil {
		return true
	}
	return (remote.OrderStatus != "" && remote.OrderStatus != "unknown" && NormalizeOrderStatus(existing.OrderStatus) != remote.OrderStatus) ||
		(remote.ItemID != "" && existing.ItemID != remote.ItemID) ||
		(remote.BuyerID != "" && existing.BuyerID != remote.BuyerID) ||
		(remote.Quantity != "" && existing.Quantity != remote.Quantity) ||
		(remote.Amount != "" && existing.Amount != remote.Amount) ||
		(remote.ReceiverName != "" && existing.ReceiverName != remote.ReceiverName) ||
		(remote.ReceiverPhone != "" && existing.ReceiverPhone != remote.ReceiverPhone) ||
		(remote.ReceiverAddr != "" && existing.ReceiverAddress != remote.ReceiverAddr) ||
		(remote.ReceiverCity != "" && existing.ReceiverCity != remote.ReceiverCity) ||
		(remote.CreatedAt != "" && existing.CreatedAt != remote.CreatedAt) ||
		(remote.IsBargain && existing.IsBargain == 0)
}

// isStableRefreshStatus 判断订单是否处于无需重复详情抓取的稳定状态。
func isStableRefreshStatus(status string) bool {
	switch status {
	case "shipped", "received", "completed", "refunded", "cancelled":
		return true
	default:
		return false
	}
}

// needsLifecycleRecheck 判断看似稳定但仍可能进入售后退款的订单是否需要增量轮转补查。
func needsLifecycleRecheck(status string) bool {
	switch NormalizeOrderStatus(status) {
	case "shipped", "received", "completed":
		return true
	default:
		return false
	}
}

// refreshCompletionMessage 根据失败和跨账号隔离数量生成不会把部分完成误报为全绿的结果文案。
func refreshCompletionMessage(summary RefreshSummary) string {
	// message 是包含新订单数量的默认完成说明。
	message := fmt.Sprintf("订单同步完成，发现 %d 个新订单", summary.Discovered)
	if summary.ConflictSkipped > 0 {
		message += fmt.Sprintf("；安全跳过 %d 个跨账号归属冲突订单", summary.ConflictSkipped)
	}
	if summary.Failed > 0 {
		message = fmt.Sprintf("订单同步部分完成：%d 项失败；发现 %d 个新订单", summary.Failed, summary.Discovered)
		if summary.ConflictSkipped > 0 {
			message += fmt.Sprintf("；安全跳过 %d 个跨账号归属冲突订单", summary.ConflictSkipped)
		}
	}
	return message
}

// splitRefreshTargets 按固定大小切分订单刷新目标。
func splitRefreshTargets(targets []refreshTarget, size int) [][]refreshTarget {
	if size <= 0 {
		size = 100
	}
	// chunks 保存切分后的详情目标分片。
	chunks := make([][]refreshTarget, 0, (len(targets)+size-1)/size)
	// start 是当前分片起始下标。
	for start := 0; start < len(targets); start += size {
		// end 是当前分片结束下标。
		end := start + size
		if end > len(targets) {
			end = len(targets)
		}
		chunks = append(chunks, targets[start:end])
	}
	return chunks
}
