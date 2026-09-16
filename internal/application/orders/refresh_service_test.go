package orders

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// refreshRepositoryFake 是订单刷新应用服务使用的内存持久化 Port。
type refreshRepositoryFake struct {
	// owned 保存账号归属关系。
	owned map[string]bool
	// order 保存订单实体。
	order *Order
	// orders 保存按订单号索引的订单实体。
	orders map[string]*Order
	// rows 保存按账号读取的订单列表。
	rows []OrderRow
	// detail 保存账号平台请求视图。
	detail *PlatformRuntimeData
	// soldDeleteCount 保存软删除数量。
	soldDeleteCount int
	// softDeleteCalls 记录完整快照缺失订单清理次数。
	softDeleteCalls int
	// upsertCount 保存订单写入次数。
	upsertCount int
	// batchUpsertCount 保存详情分片批量写入调用次数。
	batchUpsertCount int
	// batchFindCount 保存订单发现批量读取调用次数。
	batchFindCount int
	// batchFindErr 保存测试批量读取错误。
	batchFindErr error
	// batchUpsertErr 保存测试批量写入错误。
	batchUpsertErr error
	// transactionErr 保存事务错误。
	transactionErr error
	// loadErr 保存账号视图读取错误。
	loadErr error
	// syncCursor 保存测试账号的增量同步边界。
	syncCursor *OrderSyncCursor
	// savedSyncCursor 保存应用服务最后写入的同步边界。
	savedSyncCursor *OrderSyncCursor
	// coveredOrders 保存已经生成成本快照、历史补全必须跳过的订单标识。
	coveredOrders map[string]bool
}

// ExistsOwned 判断测试账号是否属于指定用户。
func (f *refreshRepositoryFake) ExistsOwned(context.Context, int64, string) (bool, error) {
	return f.owned["cookie-1"], nil
}

// ListOwnedIDs 返回测试用户账号列表。
func (f *refreshRepositoryFake) ListOwnedIDs(context.Context, int64) ([]string, error) {
	return []string{"cookie-1"}, nil
}

// GetOrder 返回测试订单实体。
func (f *refreshRepositoryFake) GetOrder(_ context.Context, orderID string) (*Order, error) {
	if f.orders != nil {
		return f.orders[orderID], nil
	}
	if f.order == nil || f.order.OrderID != orderID {
		return nil, nil
	}
	return f.order, nil
}

// FindOrder 返回测试订单及存在标记。
func (f *refreshRepositoryFake) FindOrder(_ context.Context, orderID string) (*Order, bool, error) {
	if f.orders != nil {
		// order、ok 保存测试订单及存在标记。
		order, ok := f.orders[orderID]
		return order, ok, nil
	}
	// order、err 保存测试订单及查询错误。
	order, err := f.GetOrder(context.Background(), orderID)
	return order, order != nil, err
}

// FindOrdersByIDs 返回测试订单发现批量读取结果。
func (f *refreshRepositoryFake) FindOrdersByIDs(_ context.Context, orderIDs []string) (map[string]*Order, error) {
	f.batchFindCount++
	if f.batchFindErr != nil {
		return nil, f.batchFindErr
	}
	// result 保存批量读取到的测试订单。
	result := make(map[string]*Order, len(orderIDs))
	if f.orders == nil {
		return result, nil
	}
	// orderID 是当前批量读取的订单标识。
	for _, orderID := range orderIDs {
		// order 保存当前标识对应的测试订单。
		if order := f.orders[orderID]; order != nil {
			result[orderID] = order
		}
	}
	return result, nil
}

// LockCredentials 返回无需等待的测试凭证锁。
func (f *refreshRepositoryFake) LockCredentials(string) func() {
	return func() {}
}

// LoadCookiePlatformDetail 返回测试平台请求视图。
func (f *refreshRepositoryFake) LoadCookiePlatformDetail(context.Context, string) (*PlatformRuntimeData, error) {
	return f.detail, f.loadErr
}

// UpdateRenewalCookie 接受测试 Cookie 更新。
func (f *refreshRepositoryFake) UpdateRenewalCookie(context.Context, string, string, string, int64) error {
	return nil
}

// UpsertOrder 记录测试订单写入。
func (f *refreshRepositoryFake) UpsertOrder(_ context.Context, orderID string, options UpsertOptions) error {
	f.upsertCount++
	if f.orders == nil {
		f.orders = map[string]*Order{}
	}
	// order 保存待更新的测试订单。
	order := f.orders[orderID]
	if order == nil {
		order = &Order{OrderID: orderID}
		f.orders[orderID] = order
	}
	order.CookieID, order.OrderStatus, order.Amount, order.CreatedAt = options.CookieID, options.OrderStatus, options.Amount, options.CreatedAt
	order.SpecName, order.SpecValue, order.Quantity = options.SpecName, options.SpecValue, options.Quantity
	order.ReceivedAt, order.CompletedAt, order.RefundedAt, order.CancelledAt = options.ReceivedAt, options.CompletedAt, options.RefundedAt, options.CancelledAt
	return nil
}

// BatchUpsertOrders 记录测试详情分片批量写入。
func (f *refreshRepositoryFake) BatchUpsertOrders(ctx context.Context, rows []RefreshOrderWrite) error {
	f.batchUpsertCount++
	if f.batchUpsertErr != nil {
		return f.batchUpsertErr
	}
	// row 是当前测试批量写入的订单详情。
	for _, row := range rows {
		// err 保存测试订单写入错误。
		if err := f.UpsertOrder(ctx, row.OrderID, row.Options); err != nil {
			return err
		}
	}
	return nil
}

// SoftDeleteMissingOrders 返回测试软删除数量。
func (f *refreshRepositoryFake) SoftDeleteMissingOrders(context.Context, string, map[string]struct{}) (int, error) {
	f.softDeleteCalls++
	return f.soldDeleteCount, nil
}

// ListOrdersByCookieCursor 返回测试详情目标。
func (f *refreshRepositoryFake) ListOrdersByCookieCursor(context.Context, string, int, string, string) ([]OrderRow, error) {
	return f.rows, nil
}

// CountCostedPlatformSKUs 返回测试商品可用于历史规格匹配的平台 SKU 数量。
func (f *refreshRepositoryFake) CountCostedPlatformSKUs(context.Context, string, string) (int, error) {
	return 3, nil
}

// HasOrderCostSnapshot 返回测试订单是否已经完成成本匹配。
func (f *refreshRepositoryFake) HasOrderCostSnapshot(_ context.Context, orderID string) (bool, error) {
	return f.coveredOrders[orderID], nil
}

// GetOrderSyncCursor 返回测试预置的账号订单同步边界。
func (f *refreshRepositoryFake) GetOrderSyncCursor(context.Context, string) (*OrderSyncCursor, bool, error) {
	return f.syncCursor, f.syncCursor != nil, nil
}

// UpsertOrderSyncCursor 记录应用服务成功同步后保存的账号高水位。
func (f *refreshRepositoryFake) UpsertOrderSyncCursor(_ context.Context, cursor OrderSyncCursor) error {
	f.savedSyncCursor = &cursor
	return nil
}

// WithTransaction 执行测试事务回调并返回预置错误。
func (f *refreshRepositoryFake) WithTransaction(ctx context.Context, work func(Writer) error) error {
	if f.transactionErr != nil {
		return f.transactionErr
	}
	return work(refreshWriterFake{repository: f})
}

// refreshWriterFake 是订单刷新事务写入器。
type refreshWriterFake struct {
	// repository 指向内存持久化 Port。
	repository *refreshRepositoryFake
}

// PatchOrder 不执行测试补丁写入。
func (w refreshWriterFake) PatchOrder(context.Context, string, OrderPatch) error { return nil }

// UpsertItemBasic 不执行测试商品写入。
func (w refreshWriterFake) UpsertItemBasic(context.Context, ItemWrite) error { return nil }

// UpsertOrder 委托内存持久化 Port 写入订单。
func (w refreshWriterFake) UpsertOrder(ctx context.Context, orderID string, options UpsertOptions) error {
	return w.repository.UpsertOrder(ctx, orderID, options)
}

// refreshRuntimeFake 是订单刷新应用服务使用的平台运行时 Port。
type refreshRuntimeFake struct {
	// detailAvailable 表示详情接口是否可用。
	detailAvailable bool
	// soldAvailable 表示订单列表接口是否可用。
	soldAvailable bool
	// detailResult 保存详情请求结果。
	detailResult RefreshDetailFetchResult
	// soldResult 保存订单列表请求结果。
	soldResult RefreshSoldFetchResult
	// fetchErr 保存平台请求错误。
	fetchErr error
	// expired 表示请求错误是否为会话过期。
	expired bool
	// recovered 保存是否执行了会话恢复。
	recovered bool
	// updatedCookie 保存同步到运行时的 Cookie。
	updatedCookie string
	// detailOrderIDs 按调用顺序保存被增量同步补查的订单号。
	detailOrderIDs []string
}

// incrementalRefreshRuntimeFake 为应用服务测试提供可控的增量分页终态。
type incrementalRefreshRuntimeFake struct {
	// refreshRuntimeFake 提供基础订单刷新 Port 行为。
	*refreshRuntimeFake
	// modeResult 保存增量或全量分页返回结果。
	modeResult RefreshSoldFetchResult
	// receivedOptions 保存应用服务传入的安全回看参数。
	receivedOptions RefreshSoldFetchOptions
}

// FetchSoldOrdersWithOptions 返回预置分页结果并记录应用服务选择的同步模式。
func (f *incrementalRefreshRuntimeFake) FetchSoldOrdersWithOptions(_ context.Context, _ *PlatformRuntimeData, options RefreshSoldFetchOptions, reporter RefreshSoldPageReporter) (RefreshSoldFetchResult, error) {
	f.receivedOptions = options
	if reporter != nil {
		reporter(RefreshSoldPageProgress{Processed: len(f.modeResult.Orders), Total: 937, CurrentPage: 3, TotalPages: 32,
			Mode: options.Mode, BoundaryMatched: options.BoundaryRequired, BoundaryRequired: options.BoundaryRequired})
	}
	return f.modeResult, f.fetchErr
}

// DetailAvailable 返回详情接口可用状态。
func (f *refreshRuntimeFake) DetailAvailable() bool { return f.detailAvailable }

// SoldAvailable 返回订单列表接口可用状态。
func (f *refreshRuntimeFake) SoldAvailable() bool { return f.soldAvailable }

// CredentialAvailable 判断测试平台请求视图是否有效。
func (f *refreshRuntimeFake) CredentialAvailable(detail *PlatformRuntimeData) bool {
	return detail != nil && detail.Value != ""
}

// FetchOrderDetail 返回预置订单详情结果。
func (f *refreshRuntimeFake) FetchOrderDetail(_ context.Context, _ *PlatformRuntimeData, orderID string) (RefreshDetailFetchResult, error) {
	f.detailOrderIDs = append(f.detailOrderIDs, orderID)
	return f.detailResult, f.fetchErr
}

// FetchSoldOrders 返回预置订单列表结果。
func (f *refreshRuntimeFake) FetchSoldOrders(context.Context, *PlatformRuntimeData) (RefreshSoldFetchResult, error) {
	return f.soldResult, f.fetchErr
}

// FetchSoldOrdersWithProgress 返回预置订单，并模拟两页平台累计进度供任务进度测试使用。
func (f *refreshRuntimeFake) FetchSoldOrdersWithProgress(_ context.Context, _ *PlatformRuntimeData, reporter RefreshSoldPageReporter) (RefreshSoldFetchResult, error) {
	// total 是测试平台结果的订单总数。
	total := len(f.soldResult.Orders)
	if reporter != nil && total > 0 {
		// firstPageSize 把两条以上夹具拆成两页，验证页面进度会逐次更新。
		firstPageSize := total
		if firstPageSize > 1 {
			firstPageSize--
		}
		reporter(RefreshSoldPageProgress{Processed: firstPageSize, Total: total, CurrentPage: 1, TotalPages: 2})
		if firstPageSize < total {
			reporter(RefreshSoldPageProgress{Processed: total, Total: total, CurrentPage: 2, TotalPages: 2})
		}
	}
	return f.soldResult, f.fetchErr
}

// PersistCookieSession 返回预置 Cookie 会话变化。
func (f *refreshRuntimeFake) PersistCookieSession(context.Context, *PlatformRuntimeData, RefreshCookieUpdate) (string, bool, bool, error) {
	return f.soldResult.CookieUpdate.Value, f.soldResult.CookieUpdate.Changed, true, nil
}

// UpdateRunningCookie 记录运行时 Cookie 更新。
func (f *refreshRuntimeFake) UpdateRunningCookie(_ context.Context, _, value string) {
	f.updatedCookie = value
}

// RecoverExpiredSession 记录会话恢复调用。
func (f *refreshRuntimeFake) RecoverExpiredSession(context.Context, string, error) bool {
	f.recovered = true
	return true
}

// IsSessionExpired 返回预置会话过期标记。
func (f *refreshRuntimeFake) IsSessionExpired(error) bool { return f.expired }

// TestRefreshIncrementalUsesCursorWithoutSoftDelete 验证增量模式即使自然读完分页也不执行缺失订单清理。
func TestRefreshIncrementalUsesCursorWithoutSoftDelete(t *testing.T) {
	// repository 保存已建立全量基线的账号和一个历史同步游标。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"},
		syncCursor: &OrderSyncCursor{CookieID: "cookie-1", HighWaterCreatedAt: "2025-01-02T00:00:00Z", HighWaterOrderID: "order-2", LastFullSyncAt: 10}}
	// runtime 返回自然读完的订单片段；同步模式仍是增量，因此不得升级为缺失订单校准。
	runtime := &incrementalRefreshRuntimeFake{refreshRuntimeFake: &refreshRuntimeFake{soldAvailable: true}, modeResult: RefreshSoldFetchResult{
		Orders: []RefreshSoldOrder{{OrderID: "order-3", CreatedAt: "2025-01-03T00:00:00Z"}, {OrderID: "order-2", CreatedAt: "2025-01-02T00:00:00Z"}}, CompleteSnapshot: true,
	}}
	// service 使用真实应用编排验证游标读写和软删除边界。
	service := NewRefreshService(repository, runtime, 100)
	// result、err 保存增量同步结果和错误。
	result, err := service.RefreshInMode(context.Background(), 7, "cookie-1", "all", RefreshModeIncremental)
	if err != nil || result.Summary.Discovered != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if runtime.receivedOptions.Mode != RefreshModeIncremental || runtime.receivedOptions.MinimumPages != incrementalMinimumPages || runtime.receivedOptions.BoundaryRequired != incrementalBoundaryRequired {
		t.Fatalf("options=%+v", runtime.receivedOptions)
	}
	if repository.softDeleteCalls != 0 {
		t.Fatalf("增量同步不应清理未扫描订单，calls=%d", repository.softDeleteCalls)
	}
	if repository.savedSyncCursor == nil || repository.savedSyncCursor.HighWaterOrderID != "order-3" || repository.savedSyncCursor.LastFullSyncAt != 10 {
		t.Fatalf("saved cursor=%+v", repository.savedSyncCursor)
	}
}

// TestRefreshIncrementalRotatesBoundedAfterSaleRechecks 验证明确退款纠正优先执行，普通售后轮转仍限制二十笔。
func TestRefreshIncrementalRotatesBoundedAfterSaleRechecks(t *testing.T) {
	// rows 保存二十五笔已发货订单、两笔真正终态和一笔更新时间较新的明确退款纠正。
	rows := make([]OrderRow, 0, 28)
	// orders 保存详情写入前的本地订单状态。
	orders := make(map[string]*Order)
	// index 是当前已发货订单序号。
	for index := 0; index < 25; index++ {
		// orderID 是当前轮转候选的稳定测试订单号。
		orderID := fmt.Sprintf("shipped-%02d", index)
		rows = append(rows, OrderRow{OrderID: orderID, OrderStatus: "shipped", Amount: "10.00", CreatedAt: orderID, UpdatedAt: fmt.Sprintf("2026-08-20T00:%02d:00Z", index)})
		orders[orderID] = &Order{OrderID: orderID, CookieID: "cookie-1", OrderStatus: "shipped"}
	}
	rows = append(rows,
		OrderRow{OrderID: "already-refunded", OrderStatus: "refunded", Amount: "10.00", UpdatedAt: "2026-08-19T00:00:00Z"},
		OrderRow{OrderID: "already-cancelled", OrderStatus: "cancelled", Amount: "10.00", UpdatedAt: "2026-08-19T00:00:00Z"},
		OrderRow{OrderID: "priority-refund", OrderStatus: "cancelled", Amount: "10.00", UpdatedAt: "2026-08-20T02:00:00Z", RefundRequested: true},
	)
	orders["priority-refund"] = &Order{OrderID: "priority-refund", CookieID: "cookie-1", OrderStatus: "cancelled", RefundRequested: true}
	// repository 保存可信增量游标和本地轮转候选。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"}, rows: rows, orders: orders,
		syncCursor: &OrderSyncCursor{CookieID: "cookie-1", HighWaterCreatedAt: "2026-08-20T01:00:00Z", HighWaterOrderID: "latest"}}
	// runtime 返回无新订单的可信边界，并把详情明确标记为退款成功。
	runtime := &incrementalRefreshRuntimeFake{refreshRuntimeFake: &refreshRuntimeFake{soldAvailable: true, detailAvailable: true,
		detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "退款成功", Amount: "10.00"}}}, modeResult: RefreshSoldFetchResult{BoundaryReached: true}}
	// result、refreshErr 是本轮增量补查结果和错误。
	result, refreshErr := NewRefreshService(repository, runtime, 100).RefreshInMode(context.Background(), 7, "cookie-1", "all", RefreshModeIncremental)
	if refreshErr != nil || result.Summary.DetailTotal != incrementalLifecycleRecheckLimit+1 || len(runtime.detailOrderIDs) != incrementalLifecycleRecheckLimit+1 {
		t.Fatalf("result=%+v calls=%v err=%v", result, runtime.detailOrderIDs, refreshErr)
	}
	if runtime.detailOrderIDs[0] != "priority-refund" || runtime.detailOrderIDs[1] != "shipped-00" || runtime.detailOrderIDs[len(runtime.detailOrderIDs)-1] != "shipped-02" {
		t.Fatalf("detail order rotation=%v", runtime.detailOrderIDs)
	}
	if repository.orders["priority-refund"].OrderStatus != "refunded" || repository.orders["priority-refund"].RefundedAt == "" {
		t.Fatalf("refunded order=%+v", repository.orders["priority-refund"])
	}
}

// TestRefreshIncrementalCorrectsCancelledOrderWithRefundEvidence 验证旧版本误写的 cancelled 可由精确退款申请证据安全纠正。
func TestRefreshIncrementalCorrectsCancelledOrderWithRefundEvidence(t *testing.T) {
	// repository 保存一笔旧版本已压成取消、但聊天卡片明确存在退款申请的订单。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"},
		rows:       []OrderRow{{OrderID: "legacy-refund", OrderStatus: "cancelled", Amount: "10.00", UpdatedAt: "2026-08-19T00:00:00Z", RefundRequested: true}},
		orders:     map[string]*Order{"legacy-refund": {OrderID: "legacy-refund", CookieID: "cookie-1", OrderStatus: "cancelled", RefundRequested: true}},
		syncCursor: &OrderSyncCursor{CookieID: "cookie-1", HighWaterCreatedAt: "2026-08-20T01:00:00Z", HighWaterOrderID: "latest"}}
	// runtime 的订单详情只返回旧版通用关闭码 12。
	runtime := &incrementalRefreshRuntimeFake{refreshRuntimeFake: &refreshRuntimeFake{soldAvailable: true, detailAvailable: true,
		detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "12", Amount: "10.00"}}}, modeResult: RefreshSoldFetchResult{BoundaryReached: true}}
	// result、refreshErr 是精确退款上下文协调后的同步结果。
	result, refreshErr := NewRefreshService(repository, runtime, 100).RefreshInMode(context.Background(), 7, "cookie-1", "all", RefreshModeIncremental)
	if refreshErr != nil || result.Summary.DetailTotal != 1 || repository.orders["legacy-refund"].OrderStatus != "refunded" || repository.orders["legacy-refund"].RefundedAt == "" {
		t.Fatalf("result=%+v order=%+v err=%v", result, repository.orders["legacy-refund"], refreshErr)
	}
}

// TestRefreshFullCorrectsCancelledOrderWithRefundEvidence 验证全量列表部分异常时仍会补查旧版误写的退款订单。
func TestRefreshFullCorrectsCancelledOrderWithRefundEvidence(t *testing.T) {
	// repository 保存全量模式下需要独立补查的历史退款订单。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"},
		rows:       []OrderRow{{OrderID: "legacy-full-refund", OrderStatus: "cancelled", Amount: "10.00", UpdatedAt: "2026-08-19T00:00:00Z", RefundRequested: true}},
		orders:     map[string]*Order{"legacy-full-refund": {OrderID: "legacy-full-refund", CookieID: "cookie-1", OrderStatus: "cancelled", RefundRequested: true}},
		syncCursor: &OrderSyncCursor{CookieID: "cookie-1", HighWaterCreatedAt: "2026-08-20T01:00:00Z", HighWaterOrderID: "latest"}}
	// runtime 返回完整列表快照和通用关闭详情码。
	runtime := &incrementalRefreshRuntimeFake{refreshRuntimeFake: &refreshRuntimeFake{soldAvailable: true, detailAvailable: true,
		detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "12", Amount: "10.00"}}}, modeResult: RefreshSoldFetchResult{CompleteSnapshot: true}}
	// result、refreshErr 是全量模式退款证据补查结果。
	result, refreshErr := NewRefreshService(repository, runtime, 100).RefreshInMode(context.Background(), 7, "cookie-1", "all", RefreshModeFull)
	if refreshErr != nil || result.Summary.DetailTotal != 1 || repository.orders["legacy-full-refund"].OrderStatus != "refunded" || repository.orders["legacy-full-refund"].RefundedAt == "" {
		t.Fatalf("result=%+v order=%+v err=%v", result, repository.orders["legacy-full-refund"], refreshErr)
	}
}

// TestRefreshFullSnapshotSoftDeletesMissingOrders 验证显式全量校准完成全部分页后才清理远端缺失订单。
func TestRefreshFullSnapshotSoftDeletesMissingOrders(t *testing.T) {
	// repository 保存已有增量游标和预置软删除统计。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"}, soldDeleteCount: 4,
		syncCursor: &OrderSyncCursor{CookieID: "cookie-1", HighWaterCreatedAt: "2025-01-01T00:00:00Z", HighWaterOrderID: "order-1"}}
	// runtime 返回完整平台快照。
	runtime := &incrementalRefreshRuntimeFake{refreshRuntimeFake: &refreshRuntimeFake{soldAvailable: true}, modeResult: RefreshSoldFetchResult{
		Orders: []RefreshSoldOrder{{OrderID: "order-2", CreatedAt: "2025-01-02T00:00:00Z"}}, CompleteSnapshot: true,
	}}
	// service 执行全量校准。
	service := NewRefreshService(repository, runtime, 100)
	// result、err 保存完整校准结果和错误。
	result, err := service.RefreshInMode(context.Background(), 7, "cookie-1", "all", RefreshModeFull)
	if err != nil || result.Summary.SoftDeleted != 4 || repository.softDeleteCalls != 1 {
		t.Fatalf("result=%+v calls=%d err=%v", result, repository.softDeleteCalls, err)
	}
	if runtime.receivedOptions.Mode != RefreshModeFull || repository.savedSyncCursor == nil || repository.savedSyncCursor.LastFullSyncAt == 0 {
		t.Fatalf("options=%+v cursor=%+v", runtime.receivedOptions, repository.savedSyncCursor)
	}
}

// TestRefreshSingleSuccess 验证单订单刷新会写入详情并返回兼容结果。
func TestRefreshSingleSuccess(t *testing.T) {
	// repository 保存本用例的内存持久化依赖。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, order: &Order{OrderID: "order-1", CookieID: "cookie-1", OrderStatus: "processing"}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"}}
	// runtime 保存本用例的平台运行时依赖。
	runtime := &refreshRuntimeFake{detailAvailable: true, detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "2", Quantity: "2", Amount: "12.00"}}}
	// result、err 保存单订单刷新结果和错误。
	result, err := NewRefreshService(repository, runtime, 1).RefreshSingle(context.Background(), 7, "order-1")
	if err != nil || !result.Success || repository.upsertCount != 1 || result.Detail.OrderStatus != "pending_ship" {
		t.Fatalf("单订单刷新结果异常: result=%+v err=%v", result, err)
	}
}

// TestRefreshSingleUsesRefundContextForGenericClosedDetail 验证详情只返回通用关闭码时，退款中订单仍收敛为已退款。
func TestRefreshSingleUsesRefundContextForGenericClosedDetail(t *testing.T) {
	// repository 保存已经由聊天退款申请进入 refunding 的订单。
	repository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, order: &Order{OrderID: "refund-order", CookieID: "cookie-1", OrderStatus: "refunding"}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"}}
	// runtime 使用平台通用关闭码 12，不能独立说明普通取消还是退款完成。
	runtime := &refreshRuntimeFake{detailAvailable: true, detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "12", Amount: "12.00"}}}
	// result、refreshErr 是退款上下文协调后的单笔详情结果。
	result, refreshErr := NewRefreshService(repository, runtime, 1).RefreshSingle(context.Background(), 7, "refund-order")
	// stored 是内存仓储记录的退款完成订单。
	stored := repository.orders["refund-order"]
	if refreshErr != nil || result.Detail.OrderStatus != "refunded" || stored == nil || stored.OrderStatus != "refunded" || stored.RefundedAt == "" {
		t.Fatalf("result=%+v stored=%+v err=%v", result, stored, refreshErr)
	}
}

// TestRefreshSingleRejectsUnsupportedAndCredentialChanges 验证单订单刷新拒绝不支持接口和失效凭证。
func TestRefreshSingleRejectsUnsupportedAndCredentialChanges(t *testing.T) {
	// baseRepository 保存本用例的基础持久化依赖。
	baseRepository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, order: &Order{OrderID: "order-1", CookieID: "cookie-1"}, detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"}}
	// unsupportedRuntime 保存不支持详情接口的运行时依赖。
	unsupportedRuntime := &refreshRuntimeFake{}
	// err 保存详情接口不支持错误。
	if _, err := NewRefreshService(baseRepository, unsupportedRuntime, 1).RefreshSingle(context.Background(), 7, "order-1"); !errors.Is(err, ErrRefreshDetailUnsupported) {
		t.Fatalf("未返回详情接口不支持错误: %v", err)
	}
	// credentialRepository 保存用户不匹配的持久化依赖。
	credentialRepository := &refreshRepositoryFake{owned: map[string]bool{"cookie-1": true}, order: baseRepository.order, detail: &PlatformRuntimeData{UserID: 8, Value: "cookie"}}
	// credentialRuntime 保存可用详情接口运行时依赖。
	credentialRuntime := &refreshRuntimeFake{detailAvailable: true}
	// err 保存凭证变化错误。
	if _, err := NewRefreshService(credentialRepository, credentialRuntime, 1).RefreshSingle(context.Background(), 7, "order-1"); !errors.Is(err, ErrRefreshCredentialChanged) {
		t.Fatalf("未返回凭证变化错误: %v", err)
	}
}

// TestRefreshBatchDiscoveryAndDetails 验证批量刷新会发现订单、清理缺失记录并补全详情。
func TestRefreshBatchDiscoveryAndDetails(t *testing.T) {
	// repository 保存批量刷新使用的内存持久化依赖。
	repository := &refreshRepositoryFake{
		owned:           map[string]bool{"cookie-1": true},
		detail:          &PlatformRuntimeData{UserID: 7, Value: "cookie"},
		orders:          map[string]*Order{"order-1": {OrderID: "order-1", CookieID: "cookie-1", OrderStatus: "processing"}},
		rows:            []OrderRow{{OrderID: "order-1", OrderStatus: "processing", CreatedAt: "2025-01-01T00:00:00Z"}},
		soldDeleteCount: 1,
	}
	// runtime 保存批量刷新使用的平台运行时依赖。
	runtime := &refreshRuntimeFake{
		soldAvailable: true, detailAvailable: true,
		soldResult:   RefreshSoldFetchResult{Orders: []RefreshSoldOrder{{OrderID: "order-1", OrderStatus: "2", Amount: "¥12.00", CreatedAt: "2025-01-01T00:00:00Z"}, {OrderID: "order-2", OrderStatus: "2", CreatedAt: "2025-01-02T00:00:00Z"}}},
		detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "3", Amount: "12.00"}},
	}
	// result、err 保存批量刷新结果和错误。
	result, err := NewRefreshService(repository, runtime, 1).Refresh(context.Background(), 7, "", "all")
	if err != nil || result.Summary.Discovered != 1 || result.Summary.SoftDeleted != 1 || result.Summary.DetailTotal == 0 || repository.upsertCount == 0 || repository.batchFindCount != 1 || repository.batchUpsertCount != 2 {
		t.Fatalf("批量刷新结果异常: result=%+v err=%v repository=%+v", result, err, repository)
	}
	if repository.orders["order-1"].CreatedAt != "2025-01-01T00:00:00Z" || repository.orders["order-2"].CreatedAt != "2025-01-02T00:00:00Z" {
		t.Fatalf("平台订单时间未写入: orders=%+v", repository.orders)
	}
}

// TestRefreshBatchReportsIncrementalProgress 验证批量刷新在发现、准备、逐单详情和收尾阶段持续发布进度。
func TestRefreshBatchReportsIncrementalProgress(t *testing.T) {
	// repository 保存两条待补全订单和单账号平台视图。
	repository := &refreshRepositoryFake{
		owned:  map[string]bool{"cookie-1": true},
		detail: &PlatformRuntimeData{UserID: 7, Value: "cookie"},
		orders: map[string]*Order{
			"order-1": {OrderID: "order-1", CookieID: "cookie-1", OrderStatus: "processing"},
			"order-2": {OrderID: "order-2", CookieID: "cookie-1", OrderStatus: "processing"},
		},
		rows: []OrderRow{{OrderID: "order-1", OrderStatus: "processing", CreatedAt: "2"}, {OrderID: "order-2", OrderStatus: "processing", CreatedAt: "1"}},
	}
	// runtime 返回成功的订单列表和详情，确保逐单回调执行两次。
	runtime := &refreshRuntimeFake{
		soldAvailable: true, detailAvailable: true,
		soldResult:   RefreshSoldFetchResult{Orders: []RefreshSoldOrder{{OrderID: "order-1", OrderStatus: "2"}, {OrderID: "order-2", OrderStatus: "2"}}},
		detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{OrderStatus: "3", Amount: "12.00"}},
	}
	// snapshots 按发布顺序保存进度值副本。
	var snapshots []RefreshJobProgress
	// reporter 复制每次轻量进度，不执行数据库或平台 I/O。
	reporter := func(progress RefreshJobProgress) { snapshots = append(snapshots, progress) }
	// result、refreshErr 是带进度批量刷新最终结果和错误。
	result, refreshErr := NewRefreshService(repository, runtime, 1).RefreshWithProgress(context.Background(), 7, "", "all", reporter)
	if refreshErr != nil || result.Summary.Total != 2 {
		t.Fatalf("result=%+v err=%v", result, refreshErr)
	}
	if len(snapshots) < 8 || snapshots[0].Stage != "discovering" || snapshots[len(snapshots)-1].Stage != "finalizing" {
		t.Fatalf("snapshots=%+v", snapshots)
	}
	// importSnapshots 保存平台分页读取快照，最后一页必须显示完整订单分母和账号页码。
	var importSnapshots []RefreshJobProgress
	// importSnapshot 是当前待筛选的平台导入进度。
	for _, importSnapshot := range snapshots {
		if importSnapshot.Stage == "importing_orders" {
			importSnapshots = append(importSnapshots, importSnapshot)
		}
	}
	if len(importSnapshots) != 2 || importSnapshots[1].Processed != 2 || importSnapshots[1].Total != 2 || importSnapshots[1].CurrentPage != 2 || importSnapshots[1].CurrentAccount != 1 {
		t.Fatalf("import progress=%+v", importSnapshots)
	}
	// detailSnapshots 保存逐单详情阶段快照，最后一条必须处理完两条订单且进度达到 95%。
	var detailSnapshots []RefreshJobProgress
	// snapshot 是当前待筛选的阶段进度快照。
	for _, snapshot := range snapshots {
		if snapshot.Stage == "syncing_details" {
			detailSnapshots = append(detailSnapshots, snapshot)
		}
	}
	if len(detailSnapshots) != 3 || detailSnapshots[2].Processed != 2 || detailSnapshots[2].Total != 2 || detailSnapshots[2].Percent != 95 {
		t.Fatalf("detail progress=%+v", detailSnapshots)
	}
}

// TestPersistSoldOrdersBatchesLookupAndWrite 验证订单发现会去重并批量读取、写入本地订单。
func TestPersistSoldOrdersBatchesLookupAndWrite(t *testing.T) {
	// repository 保存订单发现使用的内存持久化依赖。
	repository := &refreshRepositoryFake{orders: map[string]*Order{"existing": {OrderID: "existing", CookieID: "cookie-1", OrderStatus: "processing", Amount: "1.00"}}}
	// service 保存仅用于调用订单发现持久化的应用服务。
	service := &RefreshService{repository: repository}
	// discovered、updated、newIDs、remoteIDs、err 保存批量发现结果。
	discovered, updated, conflictSkipped, newIDs, remoteIDs, err := service.persistSoldOrders(context.Background(), "cookie-1", []RefreshSoldOrder{
		{OrderID: " existing ", OrderStatus: "unknown", Amount: "2.00"},
		{OrderID: "new-order", OrderStatus: "processing", Amount: "3.00"},
		{OrderID: "new-order", OrderStatus: "processing", Amount: "3.00"},
	})
	if err != nil || discovered != 1 || updated != 1 || conflictSkipped != 0 || len(newIDs) != 1 || len(remoteIDs) != 2 || repository.batchFindCount != 1 || repository.batchUpsertCount != 1 || repository.upsertCount != 2 || repository.orders["existing"].OrderStatus != "processing" {
		t.Fatalf("批量订单发现结果异常: discovered=%d updated=%d conflicts=%d new=%v remote=%v repository=%+v err=%v", discovered, updated, conflictSkipped, newIDs, remoteIDs, repository, err)
	}
}

// TestPersistSoldOrdersSkipsForeignOwnerWithoutFailingBatch 验证一个跨账号冲突订单不会阻断同批正常订单写入。
func TestPersistSoldOrdersSkipsForeignOwnerWithoutFailingBatch(t *testing.T) {
	// repository 保存一笔已归属其他账号的冲突订单和一笔当前账号正常订单。
	repository := &refreshRepositoryFake{orders: map[string]*Order{
		"foreign-order": {OrderID: "foreign-order", CookieID: "cookie-2", OrderStatus: "shipped"},
		"safe-order":    {OrderID: "safe-order", CookieID: "cookie-1", OrderStatus: "pending_ship"},
	}}
	// service 保存仅用于调用订单发现持久化的应用服务。
	service := &RefreshService{repository: repository}
	// discovered、updated、conflictSkipped、newIDs、remoteIDs、persistErr 是隔离写入统计。
	discovered, updated, conflictSkipped, newIDs, remoteIDs, persistErr := service.persistSoldOrders(context.Background(), "cookie-1", []RefreshSoldOrder{
		{OrderID: "foreign-order", OrderStatus: "completed"},
		{OrderID: "safe-order", OrderStatus: "shipped"},
	})
	if persistErr != nil || discovered != 0 || updated != 1 || conflictSkipped != 1 || len(newIDs) != 0 || len(remoteIDs) != 2 || repository.upsertCount != 1 {
		t.Fatalf("discovered=%d updated=%d conflicts=%d new=%v remote=%v repository=%+v err=%v", discovered, updated, conflictSkipped, newIDs, remoteIDs, repository, persistErr)
	}
	if repository.orders["foreign-order"].CookieID != "cookie-2" || repository.orders["safe-order"].OrderStatus != "shipped" {
		t.Fatalf("orders=%+v", repository.orders)
	}
}

// TestPersistSoldOrdersReturnsBatchWriteError 验证批量写入失败不会返回虚假的发现统计。
func TestPersistSoldOrdersReturnsBatchWriteError(t *testing.T) {
	// writeErr 保存预置的批量写入错误。
	writeErr := errors.New("批量写入失败")
	// repository 保存返回批量写入错误的内存依赖。
	repository := &refreshRepositoryFake{batchUpsertErr: writeErr}
	// service 保存订单刷新应用服务。
	service := &RefreshService{repository: repository}
	// discovered、updated、newIDs、remoteIDs、err 保存批量写入失败结果。
	discovered, updated, conflictSkipped, newIDs, remoteIDs, err := service.persistSoldOrders(context.Background(), "cookie-1", []RefreshSoldOrder{{OrderID: "failed-order", OrderStatus: "processing"}})
	if err == nil || discovered != 0 || updated != 0 || conflictSkipped != 0 || len(newIDs) != 0 || len(remoteIDs) != 1 || !errors.Is(err, writeErr) {
		t.Fatalf("批量写入失败结果异常: discovered=%d updated=%d conflicts=%d new=%v remote=%v err=%v", discovered, updated, conflictSkipped, newIDs, remoteIDs, err)
	}
}

// TestHistoricalCostBackfillReadsOnlyEligibleMultiSpecDetails 验证历史成本模式不扫描订单列表，只读取缺规格且可精确匹配成本的有效订单详情。
func TestHistoricalCostBackfillReadsOnlyEligibleMultiSpecDetails(t *testing.T) {
	// repository 保存一个应补全订单以及已有规格、退款和空金额三类跳过订单。
	repository := &refreshRepositoryFake{
		owned:         map[string]bool{"cookie-1": true},
		detail:        &PlatformRuntimeData{UserID: 7, Value: "test-cookie"},
		orders:        map[string]*Order{"eligible": {OrderID: "eligible", ItemID: "multi-item", CookieID: "cookie-1", OrderStatus: "shipped", Quantity: "1", Amount: "670.00"}},
		coveredOrders: map[string]bool{"already-covered": true},
		rows: []OrderRow{
			{OrderID: "eligible", ItemID: "multi-item", CookieID: "cookie-1", OrderStatus: "shipped", Quantity: "1", Amount: "670.00", CreatedAt: "2026-08-20T00:00:00Z"},
			{OrderID: "already-covered", ItemID: "multi-item", CookieID: "cookie-1", OrderStatus: "completed", Quantity: "1", Amount: "135.00"},
			{OrderID: "already-spec", ItemID: "multi-item", CookieID: "cookie-1", OrderStatus: "completed", SpecValue: "五倍额度", Quantity: "1", Amount: "690.00"},
			{OrderID: "refunded", ItemID: "multi-item", CookieID: "cookie-1", OrderStatus: "refunded", Quantity: "1", Amount: "690.00"},
			{OrderID: "empty-amount", ItemID: "multi-item", CookieID: "cookie-1", OrderStatus: "completed", Quantity: "1"},
		},
	}
	// runtime 返回明确规格，并故意声明列表接口可用以验证历史模式仍不会进入发现阶段。
	runtime := &refreshRuntimeFake{detailAvailable: true, soldAvailable: true, detailResult: RefreshDetailFetchResult{Detail: &RefreshDetail{SpecName: "套餐", SpecValue: "五倍额度", Quantity: "1", Amount: "670.00", OrderStatus: "shipped"}}}
	// result、refreshErr 是历史成本补全执行结果。
	result, refreshErr := NewRefreshService(repository, runtime, 10).RefreshInMode(context.Background(), 7, "cookie-1", "all", RefreshModeCostBackfill)
	if refreshErr != nil {
		t.Fatal(refreshErr)
	}
	if len(runtime.detailOrderIDs) != 1 || runtime.detailOrderIDs[0] != "eligible" || repository.softDeleteCalls != 0 || result.Summary.DetailTotal != 1 {
		t.Fatalf("detail IDs=%v soft deletes=%d summary=%+v", runtime.detailOrderIDs, repository.softDeleteCalls, result.Summary)
	}
	if repository.orders["eligible"].SpecValue != "五倍额度" {
		t.Fatalf("recovered order=%+v", repository.orders["eligible"])
	}
}
