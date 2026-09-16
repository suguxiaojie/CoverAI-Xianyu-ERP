package automation

import (
	"context"
	"testing"
	"time"

	"xianyu-go/internal/db"
)

// TestRefundEventsPersistRequestAndUniquelyAssociatePlainCompletion 验证退款申请进入非稳定状态，无订单号成功消息只关联唯一候选。
func TestRefundEventsPersistRequestAndUniquelyAssociatePlainCompletion(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是订单事实和退款事件共用的测试上下文。
	ctx := context.Background()
	// eventAt 是退款成功普通系统消息的平台时间。
	eventAt := time.Now().UTC().Truncate(time.Millisecond)
	if // orderErr 是写入已发货测试订单的错误。
	orderErr := store.Orders.Upsert(ctx, "refund-order", db.OrderUpsertOpts{CookieID: "cid", ChatID: "refund-chat", OrderStatus: "shipped"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	// center 是只记录订单事实且没有匹配外部动作规则的自动化中心。
	center := New(store, nil, nil)
	if // requestErr 是精确退款申请事件处理错误。
	requestErr := center.HandleTask(ctx, Task{Source: "ws", AccountID: "cid", ChatID: "refund-chat", OrderID: "refund-order", TriggerType: TriggerRefundRequested, OrderStatus: "refunding", OccurredAt: eventAt.Add(-time.Second).UnixMilli()}); requestErr != nil {
		t.Fatal(requestErr)
	}
	// refundingOrder、refundingErr 是申请事件写入后的订单事实。
	refundingOrder, refundingErr := store.Orders.Get(ctx, "refund-order")
	if refundingErr != nil || db.NormalizeOrderStatus(refundingOrder.OrderStatus) != "refunding" {
		t.Fatalf("order=%+v err=%v", refundingOrder, refundingErr)
	}
	if // completedErr 是没有订单号的退款成功事件唯一关联错误。
	completedErr := center.HandleTask(ctx, Task{Source: "ws", AccountID: "cid", ChatID: "refund-chat", TriggerType: TriggerRefundCompleted, OrderStatus: "refunded", OccurredAt: eventAt.UnixMilli()}); completedErr != nil {
		t.Fatal(completedErr)
	}
	// refundedOrder、refundedErr 是成功事件关联后的订单及读取错误。
	refundedOrder, refundedErr := store.Orders.Get(ctx, "refund-order")
	if refundedErr != nil || db.NormalizeOrderStatus(refundedOrder.OrderStatus) != "refunded" || parseDBTime(refundedOrder.RefundedAt).UnixMilli() != eventAt.UnixMilli() {
		t.Fatalf("order=%+v err=%v", refundedOrder, refundedErr)
	}
}

// TestRefundCompletedEventRejectsAmbiguousChatCandidates 验证同会话两笔退款中订单不会被普通成功文本误关联。
func TestRefundCompletedEventRejectsAmbiguousChatCandidates(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是歧义退款候选测试上下文。
	ctx := context.Background()
	// orderID 是当前待创建的同会话退款中订单。
	for _, orderID := range []string{"refund-ambiguous-1", "refund-ambiguous-2"} {
		if // orderErr 是当前退款候选写入错误。
		orderErr := store.Orders.Upsert(ctx, orderID, db.OrderUpsertOpts{CookieID: "cid", ChatID: "refund-ambiguous-chat", OrderStatus: "refunding"}); orderErr != nil {
			t.Fatal(orderErr)
		}
	}
	// center 是不得猜测两笔退款候选的自动化中心。
	center := New(store, nil, nil)
	if // handleErr 是歧义退款成功事件的安全忽略结果。
	handleErr := center.HandleTask(ctx, Task{Source: "ws", AccountID: "cid", ChatID: "refund-ambiguous-chat", TriggerType: TriggerRefundCompleted, OrderStatus: "refunded"}); handleErr != nil {
		t.Fatal(handleErr)
	}
	// orderID 是当前待确认仍为退款中的歧义订单。
	for _, orderID := range []string{"refund-ambiguous-1", "refund-ambiguous-2"} {
		// order、readErr 是处理后的候选订单和读取错误。
		order, readErr := store.Orders.Get(ctx, orderID)
		if readErr != nil || db.NormalizeOrderStatus(order.OrderStatus) != "refunding" || order.RefundedAt != "" {
			t.Fatalf("order=%+v err=%v", order, readErr)
		}
	}
}
