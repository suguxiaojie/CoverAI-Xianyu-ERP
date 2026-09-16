package db

import (
	"context"
	"testing"
)

// TestOrdersPersistIndependentLifecycleMilestonesAndTerminalPrecedence 验证收货、完成、退款时间独立保存且退款终态不被迟到消息覆盖。
func TestOrdersPersistIndependentLifecycleMilestonesAndTerminalPrecedence(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部订单写入和读取共用的上下文。
	ctx := context.Background()
	// userID、accountID 是满足订单账号外键和列表归属约束的测试身份。
	userID, accountID := seedAccount(t, store)
	// receivedAt、completedAt、refundedAt 是三个互不覆盖的生命周期时间。
	const receivedAt, completedAt, refundedAt = "2026-08-20T08:00:00Z", "2026-08-20T08:01:00Z", "2026-08-20T09:00:00Z"
	if // receivedErr 是写入买家确认收货状态和时间的错误。
	receivedErr := store.Orders.Upsert(ctx, "lifecycle-order", OrderUpsertOpts{CookieID: accountID, OrderStatus: "received", ReceivedAt: receivedAt}); receivedErr != nil {
		t.Fatal(receivedErr)
	}
	if // completedErr 是交易完成状态和时间的错误。
	completedErr := store.Orders.Upsert(ctx, "lifecycle-order", OrderUpsertOpts{CookieID: accountID, OrderStatus: "completed", CompletedAt: completedAt}); completedErr != nil {
		t.Fatal(completedErr)
	}
	if // refundedErr 是明确退款成功状态和时间的错误。
	refundedErr := store.Orders.Upsert(ctx, "lifecycle-order", OrderUpsertOpts{CookieID: accountID, OrderStatus: "refunded", RefundedAt: refundedAt}); refundedErr != nil {
		t.Fatal(refundedErr)
	}
	if // delayedErr 是迟到交易完成事件写入尝试的错误；状态必须保持退款成功。
	delayedErr := store.Orders.Upsert(ctx, "lifecycle-order", OrderUpsertOpts{CookieID: accountID, OrderStatus: "completed"}); delayedErr != nil {
		t.Fatal(delayedErr)
	}
	// order、readErr 是全部状态事件处理后的订单事实。
	order, readErr := store.Orders.Get(ctx, "lifecycle-order")
	if readErr != nil || NormalizeOrderStatus(order.OrderStatus) != "refunded" || order.ReceivedAt != receivedAt || order.CompletedAt != completedAt || order.RefundedAt != refundedAt || order.CancelledAt != "" {
		t.Fatalf("order=%+v err=%v", order, readErr)
	}
	// rows、total、listErr 是已退款筛选返回的订单列表和里程碑字段。
	rows, total, listErr := store.Orders.ListForUser(ctx, OrderListFilter{UserID: userID, CookieID: accountID, Status: "refunded", Limit: 20})
	if listErr != nil || total != 1 || len(rows) != 1 || rows[0].ReceivedAt != receivedAt || rows[0].CompletedAt != completedAt || rows[0].RefundedAt != refundedAt {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, listErr)
	}
}

// TestFindByIDsReturnsSoftDeletedOrdersForOwnershipIsolation 验证逻辑删除行仍参与跨账号主键冲突预筛。
func TestFindByIDsReturnsSoftDeletedOrdersForOwnershipIsolation(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是订单写入、逻辑删除和批量读取共用的上下文。
	ctx := context.Background()
	// _, accountID 是满足订单账号外键约束的测试归属。
	_, accountID := seedAccount(t, store)
	if // upsertErr 是创建待逻辑删除订单的错误。
	upsertErr := store.Orders.Upsert(ctx, "soft-deleted-owner", OrderUpsertOpts{CookieID: accountID, OrderStatus: "cancelled"}); upsertErr != nil {
		t.Fatal(upsertErr)
	}
	if // deleted、deleteErr 是订单逻辑删除结果和错误。
	deleted, deleteErr := store.Orders.SoftDelete(ctx, "soft-deleted-owner"); deleteErr != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, deleteErr)
	}
	// orders、findErr 是包含逻辑删除行的归属预筛结果。
	orders, findErr := store.Orders.FindByIDs(ctx, []string{"soft-deleted-owner"})
	if findErr != nil || orders["soft-deleted-owner"] == nil || orders["soft-deleted-owner"].CookieID != accountID {
		t.Fatalf("orders=%+v err=%v", orders, findErr)
	}
}
