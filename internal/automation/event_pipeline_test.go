package automation

import (
	"context"
	"reflect"
	"testing"

	"xianyu-go/internal/db"
)

// TestActionPlannerPaidEventKeepsCardBeforeShipment 验证付款事件只生成匹配卡密动作并保持发货顺序。
func TestActionPlannerPaidEventKeepsCardBeforeShipment(t *testing.T) {
	// task 是包含规格事实的付款事件。
	task := Task{TriggerType: TriggerOrderPaid, SpecName: "颜色", SpecValue: "蓝", Quantity: "2"}
	// actions 是待匹配的规则动作，故意包含一个规格不匹配的卡密动作。
	actions := []db.AutomationAction{
		{ID: 1, ActionType: ActionConfirmShipment, Enabled: true},
		{ID: 2, ActionType: ActionSendCard, Enabled: true, ConfigJSON: `{"spec_name":"颜色","spec_value":"蓝"}`},
		{ID: 3, ActionType: ActionSendCard, Enabled: true, ConfigJSON: `{"spec_name":"颜色","spec_value":"红"}`},
	}
	// original 用于确认规划过程不会修改规则动作输入。
	original := append([]db.AutomationAction(nil), actions...)
	// planner 是不执行外部 I/O 的纯动作计划组件。
	planner := actionPlanner{}
	// plan 是按发卡优先规则生成的不可变动作快照。
	plan := planner.plan(task, actions)
	if // got 用于本次流程后续判断的got
	got := []int64{plan[0].ID, plan[1].ID}; !reflect.DeepEqual(got, []int64{2, 1}) {
		t.Fatalf("付款事件动作顺序=%v，want [2 1]", got)
	}
	if !reflect.DeepEqual(actions, original) {
		t.Fatal("动作计划不应修改规则动作输入")
	}
}

// TestEventFactRecorderWithoutOrderIsNoOp 验证没有订单事实时记录组件不执行任何持久化动作。
func TestEventFactRecorderWithoutOrderIsNoOp(t *testing.T) {
	// recorder 未注入数据库时应对无订单任务安全忽略。
	recorder := newEventFactRecorder(nil)
	if // err 用于本次流程后续判断的err
	err := recorder.record(context.Background(), Task{AccountID: "cid", TriggerType: TriggerBuyerReviewed}); err != nil {
		t.Fatalf("无订单事实应安全忽略，err=%v", err)
	}
}

// TestEventFactRecorderPromotesBuyerShadowOnSellerPaidEvent 验证卖家付款事件会先接管同用户买家影子订单再写入付款事实。
func TestEventFactRecorderPromotesBuyerShadowOnSellerPaidEvent(t *testing.T) {
	// store、cleanup 是独立自动化测试存储及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是本测试数据库操作的上下文。
	ctx := context.Background()
	// admin 是同一 ERP 用户，买家和卖家账号都必须归属于该用户。
	admin, adminErr := store.Users.GetByUsername(ctx, "admin")
	if adminErr != nil {
		t.Fatal(adminErr)
	}
	// saveErr 是买家账号归属夹具写入失败的原因。
	if saveErr := store.Cookies.Save(ctx, "buyer-account", "buyer-cookie", admin.ID); saveErr != nil {
		t.Fatal(saveErr)
	}
	// insertErr 是买家影子订单夹具写入失败的原因。
	if _, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders(order_id,item_id,buyer_id,cookie_id,order_status,amount,created_at)
		VALUES('shadow-paid','item-existing','buyer-account','buyer-account','2','9.90','2026-08-20T10:00:00Z')`); insertErr != nil {
		t.Fatal(insertErr)
	}
	// recorder 使用真实数据库仓储但不匹配或执行任何自动化规则。
	recorder := newEventFactRecorder(store)
	// recordErr 是卖家侧付款事件写入错误。
	recordErr := recorder.record(ctx, Task{AccountID: "cid", BuyerID: "buyer-account", OrderID: "shadow-paid", ItemID: "item-new", ChatID: "seller-chat", TriggerType: TriggerOrderPaid, OrderStatus: "pending_ship"})
	if recordErr != nil {
		t.Fatal(recordErr)
	}
	// order 是接管后的权威订单实体。
	order, readErr := store.Orders.Get(ctx, "shadow-paid")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if order.CookieID != "cid" || order.BuyerID != "buyer-account" || order.ItemID != "item-existing" || db.NormalizeOrderStatus(order.OrderStatus) != "pending_ship" || order.ChatID != "seller-chat" || order.Amount != "9.90" || order.PaidAt == "" {
		t.Fatalf("order=%+v", order)
	}
}

// TestEventFactRecorderCreatesPendingPaymentConversationOrder 验证待付款卡片会立即建立可供 Chat 单笔补全的本地订单投影。
func TestEventFactRecorderCreatesPendingPaymentConversationOrder(t *testing.T) {
	// store、cleanup 是独立的订单事实存储和释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 限定本次待付款事实写入。
	ctx := context.Background()
	// recorder 只写入卡片已明确提供的订单关联，不执行任何平台动作。
	recorder := newEventFactRecorder(store)
	// recordErr 是卖家待付款卡片建立本地投影的结果。
	recordErr := recorder.record(ctx, Task{
		AccountID: "cid", BuyerID: "buyer", OrderID: "pending-chat-order", ItemID: "item", ChatID: "chat",
		TriggerType: TriggerOrderPendingPayment, OrderStatus: "processing",
	})
	if recordErr != nil {
		t.Fatal(recordErr)
	}
	// order 是 Chat 右栏随后能够按会话查到的待补全订单。
	order, readErr := store.Orders.Get(ctx, "pending-chat-order")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if order.CookieID != "cid" || order.BuyerID != "buyer" || order.ChatID != "chat" || order.ItemID != "item" || db.NormalizeOrderStatus(order.OrderStatus) != "processing" || order.Amount != "" {
		t.Fatalf("order=%+v", order)
	}
}

// TestEventFactRecorderPreservesBuyerOnSellerShipmentEcho 验证卖家自身发货回显不会把订单买家覆盖成卖家账号。
func TestEventFactRecorderPreservesBuyerOnSellerShipmentEcho(t *testing.T) {
	// store、cleanup 是卖家回显订单事实测试使用的隔离存储和释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是当前订单事实写入使用的上下文。
	ctx := context.Background()
	// seedErr 是已有真实买家待发货订单的写入错误。
	seedErr := store.Orders.Upsert(ctx, "seller-shipped-echo", db.OrderUpsertOpts{CookieID: "cid", BuyerID: "buyer-account", ItemID: "item", ChatID: "chat", OrderStatus: "pending_ship"})
	if seedErr != nil {
		t.Fatal(seedErr)
	}
	// recorder 只记录平台明确发货事实，不执行外部动作。
	recorder := newEventFactRecorder(store)
	// recordErr 是发送者等于卖家账号的发货回显写入结果。
	recordErr := recorder.record(ctx, Task{AccountID: "cid", BuyerID: "cid", OrderID: "seller-shipped-echo", ItemID: "item", ChatID: "chat", TriggerType: TriggerOrderShipped, OrderStatus: "shipped"})
	if recordErr != nil {
		t.Fatal(recordErr)
	}
	// order 是应用发货回显后的订单事实。
	order, readErr := store.Orders.Get(ctx, "seller-shipped-echo")
	if readErr != nil || order.BuyerID != "buyer-account" || db.NormalizeOrderStatus(order.OrderStatus) != "shipped" || order.ShippedAt == "" {
		t.Fatalf("order=%+v err=%v", order, readErr)
	}
}
