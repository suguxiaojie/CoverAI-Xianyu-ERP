package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/db"
)

// OrderRepository 将数据库 Store 适配为订单应用服务窄 repository。
type OrderRepository struct {
	// store 保存数据库聚合入口，仅在适配器内部使用。
	store *db.Store
}

// ExistsOwned 委托账号归属查询。
func (r OrderRepository) ExistsOwned(ctx context.Context, userID int64, cookieID string) (bool, error) {
	return r.store.Cookies.ExistsOwned(ctx, userID, cookieID)
}

// ListOwnedIDs 委托用户账号列表查询。
func (r OrderRepository) ListOwnedIDs(ctx context.Context, userID int64) ([]string, error) {
	return r.store.Cookies.ListOwnedIDs(ctx, userID)
}

// ListOrdersForUser 委托用户订单列表查询。
func (r OrderRepository) ListOrdersForUser(ctx context.Context, filter orderapp.ListFilter) ([]orderapp.OrderRow, int, error) {
	// rows、total、err 是数据库订单列表查询结果及其错误。
	rows, total, err := r.store.Orders.ListForUser(ctx, db.OrderListFilter{
		UserID: filter.UserID, CookieID: filter.CookieID, Status: filter.Status,
		Search: filter.Search, ContextBuyerID: filter.ContextBuyerID, ContextChatID: filter.ContextChatID,
		MatchConversationContext: filter.MatchConversationContext, PrioritizeChatID: filter.PrioritizeChatID,
		CreatedFrom: filter.CreatedFrom, CreatedTo: filter.CreatedTo,
		MinAmountCents: filter.MinAmountCents, MaxAmountCents: filter.MaxAmountCents,
		Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return orderRowsFromDB(rows), total, nil
}

// SettlementSummaryForUser 委托数据库按用户和筛选条件汇总已发货订单金额。
func (r OrderRepository) SettlementSummaryForUser(ctx context.Context, filter orderapp.ListFilter) (orderapp.SettlementAggregate, error) {
	// summary、summaryErr 是数据库返回的已发货订单数量和整数分总额。
	summary, summaryErr := r.store.Orders.SettlementSummaryForUser(ctx, db.OrderListFilter{
		UserID: filter.UserID, CookieID: filter.CookieID, Search: filter.Search,
		CreatedFrom: filter.CreatedFrom, CreatedTo: filter.CreatedTo,
		MinAmountCents: filter.MinAmountCents, MaxAmountCents: filter.MaxAmountCents,
	})
	if summaryErr != nil {
		return orderapp.SettlementAggregate{}, summaryErr
	}
	return orderapp.SettlementAggregate{OrderCount: summary.OrderCount, GrossAmountCents: summary.GrossAmountCents}, nil
}

// orderRowsFromDB 将数据库列表模型转换为订单应用层模型。
func orderRowsFromDB(rows []db.OrderRow) []orderapp.OrderRow {
	// converted 保存转换后的应用层订单行。
	converted := make([]orderapp.OrderRow, 0, len(rows))
	for _, row := range rows { // row 是待转换的数据库订单列表行。
		converted = append(converted, orderapp.OrderRow{
			OrderID: row.OrderID, ItemID: row.ItemID, ItemTitle: row.ItemTitle,
			ItemDetail: row.ItemDetail, BuyerID: row.BuyerID, ChatID: row.ChatID, BuyerName: row.BuyerName, BuyerAvatar: row.BuyerAvatar, SpecName: row.SpecName,
			SpecValue: row.SpecValue, Quantity: row.Quantity, Amount: row.Amount,
			ShipmentProofAvailable: row.ShipmentProofAvailable,
			OrderStatus:            row.OrderStatus, CookieID: row.CookieID, IsBargain: row.IsBargain,
			SystemShipped: row.SystemShipped, ReceiverName: row.ReceiverName,
			ReceiverPhone: row.ReceiverPhone, ReceiverAddr: row.ReceiverAddr,
			ReceiverCity: row.ReceiverCity, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, PaidAt: row.PaidAt, ShippedAt: row.ShippedAt,
			ReceivedAt: row.ReceivedAt, CompletedAt: row.CompletedAt, RefundedAt: row.RefundedAt, CancelledAt: row.CancelledAt,
			RefundRequested: row.RefundRequested,
		})
	}
	return converted
}

// orderFromDB 将数据库订单实体转换为不暴露存储层字段命名的应用实体。
func orderFromDB(order *db.Order) *orderapp.Order {
	if order == nil {
		return nil
	}
	return &orderapp.Order{
		OrderID: order.OrderID, ItemID: order.ItemID, BuyerID: order.BuyerID,
		SpecName: order.SpecName, SpecValue: order.SpecValue, Quantity: order.Quantity,
		Amount: order.Amount, OrderStatus: order.OrderStatus, CookieID: order.CookieID,
		IsBargain: order.IsBargain, ReceiverName: order.ReceiverName,
		ReceiverPhone: order.ReceiverPhone, ReceiverAddress: order.ReceiverAddr,
		ReceiverCity: order.ReceiverCity, Version: order.Version, ChatID: order.ChatID,
		SystemShipped: order.SystemShipped, PaidAt: order.PaidAt, ShippedAt: order.ShippedAt,
		CompletedAt: order.CompletedAt, ReceivedAt: order.ReceivedAt, RefundedAt: order.RefundedAt, CancelledAt: order.CancelledAt, BuyerReviewedAt: order.BuyerReviewedAt,
		LastReviewRequestAt: order.LastReviewRequestAt, ReviewRequestCount: order.ReviewRequestCount,
		CreatedAt: order.CreatedAt, UpdatedAt: order.UpdatedAt, RefundRequested: order.RefundRequested,
	}
}

// itemInfoFromDB 将数据库商品实体转换为订单应用层商品模型。
func itemInfoFromDB(item *db.ItemInfo) *orderapp.ItemInfo {
	if item == nil {
		return nil
	}
	return &orderapp.ItemInfo{
		ID: item.ID, CookieID: item.CookieID, ItemID: item.ItemID,
		ItemTitle: item.ItemTitle, ItemDescription: item.ItemDescription,
		ItemCategory: item.ItemCategory, ItemPrice: item.ItemPrice,
		ItemDetail: item.ItemDetail, IsMultiSpec: item.IsMultiSpec,
		MultiQuantityDelivery: item.MultiQuantityDelivery,
	}
}

// platformRuntimeDataFromDB 将数据库平台运行视图转换为订单应用层模型。
func platformRuntimeDataFromDB(data db.CookiePlatformRuntimeData) *orderapp.PlatformRuntimeData {
	return &orderapp.PlatformRuntimeData{
		ID: data.ID, UserID: data.UserID, Value: data.Value,
		MetadataJSON: data.MetadataJSON, ShowBrowser: data.ShowBrowser,
	}
}

// GetOrder 委托订单详情查询。
func (r OrderRepository) GetOrder(ctx context.Context, orderID string) (*orderapp.Order, error) {
	// order 和 err 保存数据库订单查询结果及其错误。
	order, err := r.store.Orders.Get(ctx, orderID)
	if err != nil {
		return nil, NormalizeOrderError(err)
	}
	return orderFromDB(order), nil
}

// FindOrder 委托订单查询并把数据库的不存在错误转换为 exists=false。
func (r OrderRepository) FindOrder(ctx context.Context, orderID string) (*orderapp.Order, bool, error) {
	// order、err 保存数据库订单查询结果及其错误。
	order, err := r.store.Orders.Get(ctx, orderID)
	if errors.Is(err, db.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return orderFromDB(order), true, nil
}

// FindOrdersByIDs 委托数据库批量读取订单并转换为应用实体。
func (r OrderRepository) FindOrdersByIDs(ctx context.Context, orderIDs []string) (map[string]*orderapp.Order, error) {
	// orders、err 保存数据库批量订单及查询错误。
	orders, err := r.store.Orders.FindByIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	// converted 保存转换后的应用订单索引。
	converted := make(map[string]*orderapp.Order, len(orders))
	// orderID、order 保存当前数据库订单的标识和实体。
	for orderID, order := range orders {
		converted[orderID] = orderFromDB(order)
	}
	return converted, nil
}

// GetItem 委托商品信息查询。
func (r OrderRepository) GetItem(ctx context.Context, cookieID, itemID string) (*orderapp.ItemInfo, error) {
	// item 和 err 保存数据库商品查询结果及其错误。
	item, err := r.store.Items.Get(ctx, cookieID, itemID)
	if err != nil {
		return nil, err
	}
	return itemInfoFromDB(item), nil
}

// SoftDeleteOrder 委托订单逻辑删除。
func (r OrderRepository) SoftDeleteOrder(ctx context.Context, orderID string) (bool, error) {
	return r.store.Orders.SoftDelete(ctx, orderID)
}

// WithTransaction 创建、提交或回滚订单事务。
func (r OrderRepository) WithTransaction(ctx context.Context, work func(orderapp.Writer) error) error {
	if r.store == nil || r.store.OrderWrites == nil {
		return errors.New("订单写入 Unit of Work 未初始化")
	}
	if work == nil {
		return errors.New("订单写入事务工作函数不能为空")
	}
	// transaction 是 db 层创建的窄事务写入能力，适配器不会接触或暴露原始 SQL 事务。
	return r.store.OrderWrites.WithTransaction(ctx, func(transaction *db.OrderWriteTransaction) error {
		// writer 将应用订单模型转换为当前事务的 db 写入模型。
		writer := orderWriter{transaction: transaction}
		return work(writer)
	})
}

// orderWriter 将订单应用写入模型适配为数据库事务操作。
type orderWriter struct {
	// transaction 保存订单/商品窄事务写入能力，不包含可供上层执行任意 SQL 的接口。
	transaction *db.OrderWriteTransaction
}

// PatchOrder 委托事务内订单更新。
func (w orderWriter) PatchOrder(ctx context.Context, orderID string, patch orderapp.OrderPatch) error {
	return w.transaction.PatchOrder(ctx, orderID, db.OrderPatch{
		OrderStatus: patch.OrderStatus, ItemID: patch.ItemID, BuyerID: patch.BuyerID,
		SpecName: patch.SpecName, SpecValue: patch.SpecValue, Quantity: patch.Quantity,
		Amount: patch.Amount, ReceiverName: patch.ReceiverName, ReceiverPhone: patch.ReceiverPhone,
		ReceiverAddr: patch.ReceiverAddress, ReceiverCity: patch.ReceiverCity, ChatID: patch.ChatID,
		SystemShipped: patch.SystemShipped,
	})
}

// UpsertItemBasic 委托事务内商品基础信息写入。
func (w orderWriter) UpsertItemBasic(ctx context.Context, item orderapp.ItemWrite) error {
	return w.transaction.UpsertItemBasic(ctx, &db.ItemInfoRow{
		CookieID: item.CookieID, ItemID: item.ItemID, ItemTitle: item.ItemTitle,
		ItemPrice: item.ItemPrice, ItemDetail: item.ItemDetail,
	})
}

// UpsertOrder 委托事务内订单写入。
func (w orderWriter) UpsertOrder(ctx context.Context, orderID string, options orderapp.UpsertOptions) error {
	return w.transaction.UpsertOrder(ctx, orderID, db.OrderUpsertOpts{
		ItemID: options.ItemID, BuyerID: options.BuyerID, CookieID: options.CookieID,
		OrderStatus: options.OrderStatus, SpecName: options.SpecName, SpecValue: options.SpecValue,
		Quantity: options.Quantity, Amount: options.Amount, ReceiverName: options.ReceiverName,
		ReceiverPhone: options.ReceiverPhone, ReceiverAddr: options.ReceiverAddress,
		ReceiverCity: options.ReceiverCity, ChatID: options.ChatID, CreatedAt: options.CreatedAt,
		IsBargain: options.IsBargain, SystemShipped: options.SystemShipped, ReceivedAt: options.ReceivedAt,
		CompletedAt: options.CompletedAt, RefundedAt: options.RefundedAt, CancelledAt: options.CancelledAt,
	})
}

// UpsertOrder 委托订单写入。
func (r OrderRepository) UpsertOrder(ctx context.Context, orderID string, opts orderapp.UpsertOptions) error {
	if r.store == nil || r.store.OrderWrites == nil {
		return errors.New("订单写入 Unit of Work 未初始化")
	}
	// options 是应用订单字段转换后的数据库写入模型。
	options := db.OrderUpsertOpts{
		ItemID: opts.ItemID, BuyerID: opts.BuyerID, CookieID: opts.CookieID,
		OrderStatus: opts.OrderStatus, SpecName: opts.SpecName, SpecValue: opts.SpecValue,
		Quantity: opts.Quantity, Amount: opts.Amount, ReceiverName: opts.ReceiverName,
		ReceiverPhone: opts.ReceiverPhone, ReceiverAddr: opts.ReceiverAddress,
		ReceiverCity: opts.ReceiverCity, ChatID: opts.ChatID, CreatedAt: opts.CreatedAt,
		IsBargain: opts.IsBargain, SystemShipped: opts.SystemShipped, ReceivedAt: opts.ReceivedAt,
		CompletedAt: opts.CompletedAt, RefundedAt: opts.RefundedAt, CancelledAt: opts.CancelledAt,
	}
	return r.store.OrderWrites.WithTransaction(ctx, func(transaction *db.OrderWriteTransaction) error {
		return transaction.UpsertOrder(ctx, orderID, options)
	})
}

// MarkOrderShippedAt 委托明确发货成功时间写入。
func (r OrderRepository) MarkOrderShippedAt(ctx context.Context, orderID string) error {
	return r.store.Automation.MarkOrderEventTime(ctx, orderID, "shipped_at")
}

// BatchUpsertOrders 委托订单详情分片的单条多值 UPSERT。
func (r OrderRepository) BatchUpsertOrders(ctx context.Context, rows []orderapp.RefreshOrderWrite) error {
	if len(rows) == 0 {
		return nil
	}
	// converted 保存数据库批量写入模型。
	converted := make([]db.BatchOrderUpsert, 0, len(rows))
	// row 是当前待转换的应用层批量订单记录。
	for _, row := range rows {
		converted = append(converted, db.BatchOrderUpsert{OrderID: row.OrderID, Options: db.OrderUpsertOpts{ItemID: row.Options.ItemID, BuyerID: row.Options.BuyerID, CookieID: row.Options.CookieID, OrderStatus: row.Options.OrderStatus, SpecName: row.Options.SpecName, SpecValue: row.Options.SpecValue, Quantity: row.Options.Quantity, Amount: row.Options.Amount, ReceiverName: row.Options.ReceiverName, ReceiverPhone: row.Options.ReceiverPhone, ReceiverAddr: row.Options.ReceiverAddress, ReceiverCity: row.Options.ReceiverCity, ChatID: row.Options.ChatID, CreatedAt: row.Options.CreatedAt, IsBargain: row.Options.IsBargain, SystemShipped: row.Options.SystemShipped, ReceivedAt: row.Options.ReceivedAt, CompletedAt: row.Options.CompletedAt, RefundedAt: row.Options.RefundedAt, CancelledAt: row.Options.CancelledAt}})
	}
	if r.store == nil || r.store.OrderWrites == nil {
		return errors.New("订单写入 Unit of Work 未初始化")
	}
	// transaction 是 db 层管理的详情分片事务，批量写入失败时不会留下部分订单。
	return r.store.OrderWrites.WithTransaction(ctx, func(transaction *db.OrderWriteTransaction) error {
		return transaction.UpsertOrders(ctx, converted)
	})
}

// LockCredentials 委托账号凭证锁。
func (r OrderRepository) LockCredentials(cookieID string) func() {
	return r.store.LockAccountCredentials(cookieID)
}

// LoadCookiePlatformDetail 委托平台凭证详情查询。
func (r OrderRepository) LoadCookiePlatformDetail(ctx context.Context, cookieID string) (*orderapp.PlatformRuntimeData, error) {
	// data 和 err 保存平台运行视图查询结果。
	data, err := r.store.Cookies.GetCookiePlatformRuntimeData(ctx, cookieID)
	if err != nil {
		return nil, err
	}
	return platformRuntimeDataFromDB(data), nil
}

// UpdateRenewalCookie 委托续期 Cookie 更新。
func (r OrderRepository) UpdateRenewalCookie(ctx context.Context, cookieID, value, metadata string, at int64) error {
	return r.store.Cookies.UpdateRenewalCookie(ctx, cookieID, value, metadata, at)
}

// GetPendingPriceAdjustment 读取归属当前用户且尚未出现付款／关闭终态的待付款改价卡片。
func (r OrderRepository) GetPendingPriceAdjustment(ctx context.Context, userID int64, accountID, orderID string) (*orderapp.PriceAdjustmentContext, error) {
	if r.store == nil || r.store.Chats == nil {
		return nil, errors.New("聊天改价仓储未初始化")
	}
	// adjustment、readErr 是数据库卡片上下文和读取错误。
	adjustment, readErr := r.store.Chats.GetOwnedPendingPriceAdjustment(ctx, userID, accountID, orderID)
	if readErr != nil {
		return nil, NormalizeOrderError(readErr)
	}
	return &orderapp.PriceAdjustmentContext{AccountID: adjustment.CookieID, ChatID: adjustment.ChatID,
		OrderID: adjustment.OrderID, ItemID: adjustment.ItemID}, nil
}

// ClaimPriceAdjustment 原子抢占相同订单和目标金额的真实改价运行。
func (r OrderRepository) ClaimPriceAdjustment(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error) {
	if r.store == nil || r.store.AccountTasks == nil {
		return false, errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.ClaimRunImmediately(ctx, db.AccountTaskRun{RunKey: runKey, CookieID: accountID,
		TaskType: orderapp.PriceAdjustmentTaskType, TargetID: orderID,
		RunDate: time.Unix(now, 0).UTC().Format("2006-01-02")}, now)
}

// FinishPriceAdjustment 保存改价终态；success 和 needs_review 都不会被相同运行键再次抢占。
func (r OrderRepository) FinishPriceAdjustment(ctx context.Context, runKey, status string, success, failed int, message string) error {
	if r.store == nil || r.store.AccountTasks == nil {
		return errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.FinishRun(ctx, runKey, status, success, failed, message, 0)
}

// GetPendingCloseOrder 读取归属当前用户且仍处于待付款或已付款待发货阶段的卖家卡片。
func (r OrderRepository) GetPendingCloseOrder(ctx context.Context, userID int64, accountID, orderID string) (*orderapp.CloseOrderContext, error) {
	if r.store == nil || r.store.Chats == nil {
		return nil, errors.New("聊天关单仓储未初始化")
	}
	// closeContext、readErr 是数据库卡片上下文和读取错误。
	closeContext, readErr := r.store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, orderID)
	if readErr != nil {
		return nil, NormalizeOrderError(readErr)
	}
	return &orderapp.CloseOrderContext{AccountID: closeContext.CookieID, ChatID: closeContext.ChatID,
		OrderID: closeContext.OrderID, ItemID: closeContext.ItemID, Stage: closeContext.Stage}, nil
}

// GetPendingShipment 读取归属当前用户、具备卖家角色证据且尚未出现终态的付款卡片。
func (r OrderRepository) GetPendingShipment(ctx context.Context, userID int64, accountID, orderID string) (*orderapp.ShipmentContext, error) {
	if r.store == nil || r.store.Chats == nil {
		return nil, errors.New("聊天发货仓储未初始化")
	}
	// shipment、readErr 是数据库付款卡片上下文和读取错误。
	shipment, readErr := r.store.Chats.GetOwnedPendingShipment(ctx, userID, accountID, orderID)
	if readErr != nil {
		return nil, NormalizeOrderError(readErr)
	}
	return &orderapp.ShipmentContext{AccountID: shipment.CookieID, ChatID: shipment.ChatID,
		OrderID: shipment.OrderID, ItemID: shipment.ItemID, BuyerID: shipment.BuyerID}, nil
}

// PromoteBuyerShadowOrder 委托数据库执行同用户、开放状态和精确买家归属限制下的安全接管。
func (r OrderRepository) PromoteBuyerShadowOrder(ctx context.Context, shipmentContext orderapp.ShipmentContext) (bool, error) {
	if r.store == nil || r.store.Orders == nil {
		return false, errors.New("订单发货仓储未初始化")
	}
	return r.store.Orders.PromoteBuyerShadowOrder(ctx, db.PromoteBuyerShadowOrderInput{
		OrderID: shipmentContext.OrderID, BuyerAccountID: shipmentContext.BuyerID,
		SellerAccountID: shipmentContext.AccountID, ChatID: shipmentContext.ChatID, ItemID: shipmentContext.ItemID,
	})
}

// ClaimShipmentEvidence 原子抢占相同账号和订单的真实凭证发货运行。
func (r OrderRepository) ClaimShipmentEvidence(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error) {
	if r.store == nil || r.store.AccountTasks == nil {
		return false, errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.ClaimRunImmediately(ctx, db.AccountTaskRun{RunKey: runKey, CookieID: accountID,
		TaskType: orderapp.ShipmentEvidenceTaskType, TargetID: orderID,
		RunDate: time.Unix(now, 0).UTC().Format("2006-01-02")}, now)
}

// FinishShipmentEvidence 保存发货终态；success 和 needs_review 都会阻止相同订单重复提交。
func (r OrderRepository) FinishShipmentEvidence(ctx context.Context, runKey, status string, success, failed int, message string) error {
	if r.store == nil || r.store.AccountTasks == nil {
		return errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.FinishRun(ctx, runKey, status, success, failed, message, 0)
}

// SaveShipmentProof 保存平台明确成功后的 ERP 发货描述和官方图片地址。
func (r OrderRepository) SaveShipmentProof(ctx context.Context, proof orderapp.ShipmentProof) error {
	// imageURLsJSON、encodeErr 是经过应用校验的图片地址数组及编码错误。
	imageURLsJSON, encodeErr := json.Marshal(proof.ImageURLs)
	if encodeErr != nil {
		return encodeErr
	}
	return r.store.OrderShipmentProofs.Save(ctx, db.OrderShipmentProof{OrderID: proof.OrderID, CookieID: proof.AccountID, TradeText: proof.TradeText, ImageURLsJSON: string(imageURLsJSON), Source: proof.Source, SubmittedAt: proof.SubmittedAt})
}

// GetShipmentProofForUser 按用户和订单归属读取 ERP 发货凭证并转换为应用模型。
func (r OrderRepository) GetShipmentProofForUser(ctx context.Context, userID int64, orderID string) (*orderapp.ShipmentProof, error) {
	// proof、readErr 是数据库归属校验后的凭证元数据和错误。
	proof, readErr := r.store.OrderShipmentProofs.GetForUser(ctx, userID, orderID)
	if readErr != nil {
		return nil, NormalizeOrderError(readErr)
	}
	// imageURLs 是凭证保存的官方图片地址数组。
	var imageURLs []string
	if decodeErr := json.Unmarshal([]byte(proof.ImageURLsJSON), &imageURLs); decodeErr != nil { // decodeErr 表示本地凭证 JSON 已损坏。
		return nil, decodeErr
	}
	return &orderapp.ShipmentProof{OrderID: proof.OrderID, AccountID: proof.CookieID, TradeText: proof.TradeText, ImageURLs: imageURLs, Source: proof.Source, SubmittedAt: proof.SubmittedAt}, nil
}

// ClaimCloseOrder 原子抢占同一账号和订单的卖家关单运行。
func (r OrderRepository) ClaimCloseOrder(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error) {
	if r.store == nil || r.store.AccountTasks == nil {
		return false, errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.ClaimRunImmediately(ctx, db.AccountTaskRun{RunKey: runKey, CookieID: accountID,
		TaskType: orderapp.CloseOrderTaskType, TargetID: orderID, RunDate: time.Unix(now, 0).UTC().Format("2006-01-02")}, now)
}

// FinishCloseOrder 保存关单终态；成功和 needs_review 都不会被相同运行键再次抢占。
func (r OrderRepository) FinishCloseOrder(ctx context.Context, runKey, status string, success, failed int, message string) error {
	if r.store == nil || r.store.AccountTasks == nil {
		return errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.FinishRun(ctx, runKey, status, success, failed, message, 0)
}

// ClaimRefundAction 原子抢占同一退款申请的普通同意／拒绝动作。
func (r OrderRepository) ClaimRefundAction(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error) {
	if r.store == nil || r.store.AccountTasks == nil {
		return false, errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.ClaimRunImmediately(ctx, db.AccountTaskRun{RunKey: runKey, CookieID: accountID,
		TaskType: orderapp.RefundActionTaskType, TargetID: orderID, RunDate: time.Unix(now, 0).UTC().Format("2006-01-02")}, now)
}

// FinishRefundAction 保存退款处理终态；成功和 needs_review 都阻止同一申请重复提交。
func (r OrderRepository) FinishRefundAction(ctx context.Context, runKey, status string, success, failed int, message string) error {
	if r.store == nil || r.store.AccountTasks == nil {
		return errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.FinishRun(ctx, runKey, status, success, failed, message, 0)
}

// ClaimRedFlowerRequest 原子抢占手动求花运行；只有首次动作或明确失败后的人工重试可以继续。
func (r OrderRepository) ClaimRedFlowerRequest(ctx context.Context, runKey, cookieID, orderID string, now int64) (bool, error) {
	if r.store == nil || r.store.AccountTasks == nil {
		return false, errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.ClaimRunImmediately(ctx, db.AccountTaskRun{
		RunKey: runKey, CookieID: cookieID, TaskType: orderapp.RedFlowerRequestTaskType,
		TargetID: orderID, RunDate: time.Unix(now, 0).UTC().Format("2006-01-02"),
	}, now)
}

// FinishRedFlowerRequest 保存手动求花终态，成功和人工核对状态都不会被再次抢占。
func (r OrderRepository) FinishRedFlowerRequest(ctx context.Context, runKey, status string, success, failed int, message string) error {
	if r.store == nil || r.store.AccountTasks == nil {
		return errors.New("账号任务运行仓储未初始化")
	}
	return r.store.AccountTasks.FinishRun(ctx, runKey, status, success, failed, message, 0)
}

// GetRedFlowerRequestRun 读取手动求花幂等状态，不返回账号凭证或数据库模型。
func (r OrderRepository) GetRedFlowerRequestRun(ctx context.Context, runKey string) (orderapp.RedFlowerRequestRun, bool, error) {
	if r.store == nil || r.store.AccountTasks == nil {
		return orderapp.RedFlowerRequestRun{}, false, errors.New("账号任务运行仓储未初始化")
	}
	// run、exists、runErr 是数据库运行记录、存在标记和查询错误。
	run, exists, runErr := r.store.AccountTasks.GetRunByKey(ctx, runKey)
	if runErr != nil || !exists {
		return orderapp.RedFlowerRequestRun{}, exists, runErr
	}
	return orderapp.RedFlowerRequestRun{Status: run.Status, Message: run.ErrorMessage, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt}, true, nil
}

// SoftDeleteMissingOrders 委托账号远端缺失订单清理。
func (r OrderRepository) SoftDeleteMissingOrders(ctx context.Context, cookieID string, activeIDs map[string]struct{}) (int, error) {
	return r.store.Orders.SoftDeleteMissingForCookie(ctx, cookieID, activeIDs)
}

// ListOrdersByCookieCursor 委托订单复合游标查询并转换应用层模型。
func (r OrderRepository) ListOrdersByCookieCursor(ctx context.Context, cookieID string, limit int, afterCreatedAt, afterOrderID string) ([]orderapp.OrderRow, error) {
	// rows、err 保存数据库游标查询结果及错误。
	rows, err := r.store.Orders.ByCookieCursor(ctx, cookieID, limit, afterCreatedAt, afterOrderID)
	if err != nil {
		return nil, err
	}
	return orderRowsFromDB(rows), nil
}

// CountCostedPlatformSKUs 返回历史成本补全可以精确匹配的平台 SKU 数量。
func (r OrderRepository) CountCostedPlatformSKUs(ctx context.Context, cookieID, itemID string) (int, error) {
	return r.store.Items.CountCostedPlatformSKUs(ctx, cookieID, itemID)
}

// HasOrderCostSnapshot 判断历史订单是否已经完成成本修正，已完成订单不得重复读取详情。
func (r OrderRepository) HasOrderCostSnapshot(ctx context.Context, orderID string) (bool, error) {
	return r.store.OrderCosts.Exists(ctx, orderID)
}

// GetOrderSyncCursor 读取账号订单同步游标并转换为应用层模型。
func (r OrderRepository) GetOrderSyncCursor(ctx context.Context, cookieID string) (*orderapp.OrderSyncCursor, bool, error) {
	// cursor、exists、err 保存数据库游标、存在性和查询错误。
	cursor, exists, err := r.store.OrderSyncCursors.Get(ctx, cookieID)
	if err != nil || !exists {
		return nil, exists, err
	}
	return &orderapp.OrderSyncCursor{CookieID: cursor.CookieID, HighWaterCreatedAt: cursor.HighWaterCreatedAt,
		HighWaterOrderID: cursor.HighWaterOrderID, LastIncrementalSyncAt: cursor.LastIncrementalSyncAt,
		LastFullSyncAt: cursor.LastFullSyncAt}, true, nil
}

// UpsertOrderSyncCursor 保存应用层计算出的账号高水位和最近同步时间。
func (r OrderRepository) UpsertOrderSyncCursor(ctx context.Context, cursor orderapp.OrderSyncCursor) error {
	return r.store.OrderSyncCursors.Upsert(ctx, db.OrderSyncCursor{CookieID: cursor.CookieID,
		HighWaterCreatedAt: cursor.HighWaterCreatedAt, HighWaterOrderID: cursor.HighWaterOrderID,
		LastIncrementalSyncAt: cursor.LastIncrementalSyncAt, LastFullSyncAt: cursor.LastFullSyncAt})
}

// Create 创建订单刷新后台任务并同步应用层模型默认值。
func (r OrderRepository) Create(ctx context.Context, job *orderapp.RefreshJob) error {
	// dbJob 保存待写入的数据库任务模型。
	dbJob := &db.OrderRefreshJob{
		ID: job.ID, UserID: job.UserID, CookieID: job.CookieID, FilterStatus: job.FilterStatus, SyncMode: job.SyncMode,
		Status: job.Status, ResultJSON: job.ResultJSON, ErrorMessage: job.ErrorMessage,
		WorkerToken: job.WorkerToken, LeaseExpiresAt: job.LeaseExpiresAt,
	}
	// err 表示数据库任务创建错误。
	if err := r.store.OrderRefreshJobs.Create(ctx, dbJob); err != nil {
		return err
	}
	job.Status, job.ResultJSON = dbJob.Status, dbJob.ResultJSON
	return nil
}

// Get 按用户读取订单刷新后台任务并转换为应用层模型。
func (r OrderRepository) Get(ctx context.Context, userID int64, id string) (*orderapp.RefreshJob, error) {
	// job、err 保存数据库任务读取结果及错误。
	job, err := r.store.OrderRefreshJobs.Get(ctx, userID, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, orderapp.ErrRefreshJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return refreshJobFromDB(job), nil
}

// Claim 原子抢占订单刷新后台任务。
func (r OrderRepository) Claim(ctx context.Context, id, token string, leaseExpiresAt int64) (bool, error) {
	return r.store.OrderRefreshJobs.Claim(ctx, id, token, leaseExpiresAt)
}

// Cancel 按用户归属委托订单刷新任务取消。
func (r OrderRepository) Cancel(ctx context.Context, userID int64, id string) (bool, error) {
	return r.store.OrderRefreshJobs.Cancel(ctx, userID, id)
}

// Complete 以租约令牌安全写入订单刷新后台任务终态。
func (r OrderRepository) Complete(ctx context.Context, id, token, status, resultJSON, errorMessage string) (bool, error) {
	return r.store.OrderRefreshJobs.Complete(ctx, id, token, status, resultJSON, errorMessage)
}

// UpdateProgress 以租约令牌安全写入订单刷新任务的轻量进度 JSON。
func (r OrderRepository) UpdateProgress(ctx context.Context, id, token, progressJSON string) (bool, error) {
	return r.store.OrderRefreshJobs.UpdateProgress(ctx, id, token, progressJSON)
}

// Recoverable 查询租约过期的订单刷新后台任务。
func (r OrderRepository) Recoverable(ctx context.Context, now int64, limit int) ([]orderapp.RefreshJob, error) {
	// jobs、err 保存数据库层可恢复任务及查询错误。
	jobs, err := r.store.OrderRefreshJobs.Recoverable(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	// converted 保存转换后的应用层任务列表。
	converted := make([]orderapp.RefreshJob, 0, len(jobs))
	for _, job := range jobs { // job 表示当前待转换的数据库任务。
		converted = append(converted, *refreshJobFromDB(&job))
	}
	return converted, nil
}

// RequeueExpired 将过期订单刷新后台任务恢复为 queued。
func (r OrderRepository) RequeueExpired(ctx context.Context, id string, now int64) (bool, error) {
	return r.store.OrderRefreshJobs.RequeueExpired(ctx, id, now)
}

// refreshJobFromDB 将数据库任务模型转换为应用层任务模型。
func refreshJobFromDB(job *db.OrderRefreshJob) *orderapp.RefreshJob {
	if job == nil {
		return nil
	}
	return &orderapp.RefreshJob{
		ID: job.ID, UserID: job.UserID, CookieID: job.CookieID, FilterStatus: job.FilterStatus, SyncMode: job.SyncMode,
		Status: job.Status, ResultJSON: job.ResultJSON, ErrorMessage: job.ErrorMessage,
		WorkerToken: job.WorkerToken, LeaseExpiresAt: job.LeaseExpiresAt,
		CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}
}

// NewOrderRepository 从数据库 Store 构造订单应用服务适配器。
func NewOrderRepository(store *db.Store) *OrderRepository {
	if store == nil || store.Cookies == nil || store.Orders == nil || store.Items == nil || store.OrderSyncCursors == nil {
		return nil
	}
	return &OrderRepository{store: store}
}

// NewOrderRefreshJobRepository 构造订单刷新任务持久化适配器。
func NewOrderRefreshJobRepository(store *db.Store) orderapp.RefreshJobRepository {
	if store == nil || store.OrderRefreshJobs == nil {
		return nil
	}
	return OrderRepository{store: store}
}

// 确保 Store 适配器始终覆盖订单应用服务所需的全部能力。
var _ orderapp.Repository = OrderRepository{}
var _ orderapp.UnitOfWork = OrderRepository{}
var _ orderapp.Writer = orderWriter{}
var _ orderapp.RedFlowerRequestRepository = OrderRepository{}
var _ orderapp.RefreshJobRepository = OrderRepository{}
