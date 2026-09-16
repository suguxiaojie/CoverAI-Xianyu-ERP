package orders

import (
	"context"
	"fmt"
)

// RefreshSoldPageProgress 描述平台订单列表每页读取后的累计进度，不包含账号或订单标识。
type RefreshSoldPageProgress struct {
	// Processed 是当前账号已从平台读取的累计订单数量。
	Processed int
	// Total 是平台为当前账号返回的订单总数；未知时至少等于 Processed。
	Total int
	// CurrentPage 是当前已完成的页码，从 1 开始。
	CurrentPage int
	// TotalPages 是按平台总数和页大小计算的总页数。
	TotalPages int
	// Mode 是 incremental 或 full，用于界面解释当前扫描范围。
	Mode string
	// BoundaryMatched 是增量模式当前连续命中的历史边界订单数。
	BoundaryMatched int
	// BoundaryRequired 是允许提前停止前需要连续命中的历史边界订单数。
	BoundaryRequired int
}

// RefreshSoldFetchOptions 控制平台分页读取是完整校准还是带安全回看的增量扫描。
type RefreshSoldFetchOptions struct {
	// Mode 是 incremental 或 full；其他值按 incremental 处理。
	Mode string
	// Cursor 是增量模式使用的账号高水位；为空时调用方必须退化为完整基线。
	Cursor *OrderSyncCursor
	// MinimumPages 是增量模式停止前至少读取的页数。
	MinimumPages int
	// BoundaryRequired 是提前停止前必须连续命中的旧边界订单数。
	BoundaryRequired int
}

// RefreshSoldPageReporter 接收当前账号订单列表逐页读取进度。
type RefreshSoldPageReporter func(RefreshSoldPageProgress)

// RefreshSoldProgressRuntime 是可选的逐页订单列表能力；不支持时继续使用一次性兼容接口。
type RefreshSoldProgressRuntime interface {
	// FetchSoldOrdersWithProgress 获取全部已售订单，并在每页响应后发布累计数量。
	FetchSoldOrdersWithProgress(ctx context.Context, detail *PlatformRuntimeData, reporter RefreshSoldPageReporter) (RefreshSoldFetchResult, error)
}

// RefreshSoldModeRuntime 是支持完整校准和增量边界停止的可选平台分页能力。
type RefreshSoldModeRuntime interface {
	// FetchSoldOrdersWithOptions 按同步模式读取订单并逐页上报扫描范围。
	FetchSoldOrdersWithOptions(ctx context.Context, detail *PlatformRuntimeData, options RefreshSoldFetchOptions, reporter RefreshSoldPageReporter) (RefreshSoldFetchResult, error)
}

// refreshProgressTracker 把订单刷新内部阶段转换为稳定、无敏感字段的任务进度快照。
type refreshProgressTracker struct {
	// reporter 是 runner 注入的轻量进度接收器；nil 表示兼容同步调用不需要进度。
	reporter RefreshJobProgressReporter
}

// newRefreshProgressTracker 创建当前刷新调用独占的进度协调器。
func newRefreshProgressTracker(reporter RefreshJobProgressReporter) refreshProgressTracker {
	return refreshProgressTracker{reporter: reporter}
}

// discovering 上报逐账号订单发现进度，整体占 5% 到 25%。
func (tracker refreshProgressTracker) discovering(processed, total, succeeded, failed int) {
	// percent 是发现阶段的整体加权进度；账号总数未知或为零时保持起始值。
	percent := 5
	if total > 0 {
		percent += processed * 20 / total
	}
	tracker.publish(RefreshJobProgress{
		Stage: "discovering", Message: "正在从闲鱼发现订单",
		Processed: processed, Total: total, Succeeded: succeeded, Failed: failed, Percent: percent,
	})
}

// importingOrders 上报当前账号逐页读取订单的累计数量，整体进度落在该账号分配的发现阶段区间。
func (tracker refreshProgressTracker) importingOrders(currentAccount, totalAccounts int, page RefreshSoldPageProgress) {
	// percent 从已完成账号和当前账号页内比例计算 5% 到 25% 的整体进度。
	percent := 5
	if totalAccounts > 0 {
		// completedAccountShare 是当前账号之前已经完整处理的发现阶段进度。
		completedAccountShare := (currentAccount - 1) * 20 / totalAccounts
		// currentAccountShare 是当前账号按订单累计数量计算的页内进度。
		currentAccountShare := 0
		if page.Total > 0 {
			currentAccountShare = page.Processed * 20 / page.Total / totalAccounts
		}
		percent += completedAccountShare + currentAccountShare
	}
	// message 明确当前显示的是平台订单读取／导入准备进度，而不是账号数量。
	// modeLabel 是当前同步范围的中文名称。
	modeLabel := "增量同步"
	if page.Mode == RefreshModeFull {
		modeLabel = "全量校准"
	}
	// pageLabel 在增量模式不承诺读取平台全部页，只展示当前实际请求页码。
	pageLabel := fmt.Sprintf("第 %d 页", page.CurrentPage)
	if page.Mode == RefreshModeFull && page.TotalPages > 0 {
		pageLabel = fmt.Sprintf("第 %d/%d 页", page.CurrentPage, page.TotalPages)
	}
	// boundaryLabel 展示增量扫描距离可信停止边界的实时进度。
	boundaryLabel := ""
	if page.Mode != RefreshModeFull && page.BoundaryRequired > 0 {
		boundaryLabel = fmt.Sprintf("，历史边界 %d/%d", page.BoundaryMatched, page.BoundaryRequired)
	}
	// message 汇总当前账号、实际请求页码和增量边界进度，不包含订单标识。
	message := fmt.Sprintf("%s正在扫描订单（账号 %d/%d，%s%s）", modeLabel, currentAccount, totalAccounts, pageLabel, boundaryLabel)
	tracker.publish(RefreshJobProgress{
		Stage: "importing_orders", Message: message,
		Processed: page.Processed, Total: page.Total, Succeeded: page.Processed, Percent: percent,
		CurrentPage: page.CurrentPage, TotalPages: page.TotalPages,
		CurrentAccount: currentAccount, TotalAccounts: totalAccounts,
		Mode: page.Mode, BoundaryMatched: page.BoundaryMatched, BoundaryRequired: page.BoundaryRequired,
	})
}

// preparing 上报本地订单扫描完成和详情目标数量，整体进度固定为 30%。
func (tracker refreshProgressTracker) preparing(detailTotal int) {
	// message 根据详情目标数量说明下一阶段工作量，不包含订单号或账号标识。
	message := fmt.Sprintf("已找到 %d 个需要补全详情的订单", detailTotal)
	tracker.publish(RefreshJobProgress{Stage: "preparing", Message: message, Total: detailTotal, Percent: 30})
}

// syncingDetails 上报逐单详情读取进度，整体占 30% 到 95%。
func (tracker refreshProgressTracker) syncingDetails(processed, total, succeeded, failed int) {
	// percent 是详情阶段的整体加权进度；没有目标时保持准备阶段值。
	percent := 30
	if total > 0 {
		percent += processed * 65 / total
	}
	tracker.publish(RefreshJobProgress{
		Stage: "syncing_details", Message: "正在逐单同步订单详情",
		Processed: processed, Total: total, Succeeded: succeeded, Failed: failed, Percent: percent,
	})
}

// finalizing 上报数据库和 Cookie 会话收尾阶段，等待 runner 写入最终完整结果。
func (tracker refreshProgressTracker) finalizing(processed, total, succeeded, failed int) {
	tracker.publish(RefreshJobProgress{
		Stage: "finalizing", Message: "正在保存同步结果",
		Processed: processed, Total: total, Succeeded: succeeded, Failed: failed, Percent: 98,
	})
}

// publish 在兼容同步调用中安全跳过，在异步任务中只发布不含逐单详情的轻量快照。
func (tracker refreshProgressTracker) publish(progress RefreshJobProgress) {
	if tracker.reporter != nil {
		tracker.reporter(progress)
	}
}
