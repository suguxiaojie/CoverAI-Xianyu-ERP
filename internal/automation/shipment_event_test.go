package automation

import (
	"context"
	"testing"
	"time"

	"xianyu-go/internal/db"
)

// TestOrderShippedEventAssociatesUniqueChatOrderAndSchedulesFlower 验证无订单号卡片只关联同会话唯一已付款订单并创建秒级任务。
func TestOrderShippedEventAssociatesUniqueChatOrderAndSchedulesFlower(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是订单事实、事件和调度断言共用的上下文。
	ctx := context.Background()
	// eventAt 是发货卡片平台时间。
	eventAt := time.Now().UTC().Truncate(time.Millisecond)
	// paidAt 是发货前二十秒的订单付款时间。
	paidAt := eventAt.Add(-20 * time.Second)
	if // orderErr 是唯一候选订单写入错误。
	orderErr := store.Orders.Upsert(ctx, "unique-shipment-order", db.OrderUpsertOpts{
		CookieID: "cid", ItemID: "item-1", BuyerID: "buyer-1", ChatID: "chat-unique", OrderStatus: "unknown",
	}); orderErr != nil {
		t.Fatal(orderErr)
	}
	if // paidErr 是唯一候选付款时间写入错误。
	paidErr := store.Automation.MarkOrderEventTimeAt(ctx, "unique-shipment-order", "paid_at", paidAt); paidErr != nil {
		t.Fatal(paidErr)
	}
	if // settingsErr 是唯一候选测试的自动求花配置写入错误。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoRequestFlowerEnabled: true,
		RequestFlowerAfterHours: 24, RequestFlowerAfterSeconds: 10, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	}); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// center 是待验证唯一候选关联和后续调度的自动化中心。
	center := New(store, nil, nil)
	// handleErr 是没有订单号的发货事件处理结果。
	handleErr := center.HandleTask(ctx, Task{
		Source: "ws", AccountID: "cid", TriggerType: TriggerOrderShipped, ChatID: "chat-unique",
		OrderStatus: "shipped", OccurredAt: eventAt.UnixMilli(), Text: "你已发货",
	})
	if handleErr != nil {
		t.Fatal(handleErr)
	}
	// order、readErr 是关联后写入发货状态和平台事件时间的订单。
	order, readErr := store.Orders.Get(ctx, "unique-shipment-order")
	if readErr != nil || db.NormalizeOrderStatus(order.OrderStatus) != "shipped" || parseDBTime(order.ShippedAt).UnixMilli() != eventAt.UnixMilli() {
		t.Fatalf("order=%+v err=%v", order, readErr)
	}
	// pending 是发货事件关联后创建的唯一秒级求花任务数量。
	var pending int
	if // countErr 是唯一候选关联后秒级任务数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_pending_tasks WHERE task_key=?`, scheduledFlowerRequestKey("cid", "unique-shipment-order")).Scan(&pending); countErr != nil || pending != 1 {
		t.Fatalf("pending=%d err=%v", pending, countErr)
	}
}

// TestOrderShippedEventRejectsAmbiguousChatOrders 验证同会话存在两笔开放订单时只保留聊天卡片，不猜测发货订单。
func TestOrderShippedEventRejectsAmbiguousChatOrders(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是歧义候选测试上下文。
	ctx := context.Background()
	// eventAt 是两笔订单之后的发货卡片时间。
	eventAt := time.Now().UTC().Truncate(time.Millisecond)
	// orderID 是当前待创建的同会话开放订单标识。
	for _, orderID := range []string{"ambiguous-order-1", "ambiguous-order-2"} {
		if // orderErr 是当前歧义候选订单写入错误。
		orderErr := store.Orders.Upsert(ctx, orderID, db.OrderUpsertOpts{CookieID: "cid", ChatID: "chat-ambiguous", OrderStatus: "pending_ship"}); orderErr != nil {
			t.Fatal(orderErr)
		}
		if // paidErr 是当前歧义候选付款时间写入错误。
		paidErr := store.Automation.MarkOrderEventTimeAt(ctx, orderID, "paid_at", eventAt.Add(-time.Minute)); paidErr != nil {
			t.Fatal(paidErr)
		}
	}
	// center 是不得猜测两笔候选的自动化中心。
	center := New(store, nil, nil)
	if // handleErr 是歧义发货事件的安全忽略结果。
	handleErr := center.HandleTask(ctx, Task{Source: "ws", AccountID: "cid", TriggerType: TriggerOrderShipped, ChatID: "chat-ambiguous", OccurredAt: eventAt.UnixMilli(), Text: "你已发货"}); handleErr != nil {
		t.Fatal(handleErr)
	}
	// orderID 是当前待确认仍未写入 shipped_at 的歧义订单。
	for _, orderID := range []string{"ambiguous-order-1", "ambiguous-order-2"} {
		// order、readErr 是处理后的候选订单和读取错误。
		order, readErr := store.Orders.Get(ctx, orderID)
		if readErr != nil || order.ShippedAt != "" || db.NormalizeOrderStatus(order.OrderStatus) != "pending_ship" {
			t.Fatalf("order=%+v err=%v", order, readErr)
		}
	}
	// pending 是歧义发货事件后不应存在的秒级任务数量。
	var pending int
	if // countErr 是歧义事件后秒级任务数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_pending_tasks WHERE trigger_type=?`, TriggerRedFlowerRequestDue).Scan(&pending); countErr != nil || pending != 0 {
		t.Fatalf("pending=%d err=%v", pending, countErr)
	}
}
