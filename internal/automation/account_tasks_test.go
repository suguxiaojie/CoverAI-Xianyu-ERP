package automation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// fakeAccountTaskClient 用于本次流程后续判断的fake账号任务Client
type fakeAccountTaskClient struct {
	pendingCalls  int
	rateCalls     int
	polishCalls   int
	pending       []mtop.PendingRateOrder
	pendingErr    error
	rateErr       error
	items         []mtop.ItemListItem
	fetchItemsErr error
	fetchPageSize int
	fetchMaxPages int
	polishErr     error
	// flowerCalls 统计官方求花请求次数。
	flowerCalls int
	// flowerResult 是自动求花测试使用的平台明确结果。
	flowerResult *mtop.RedFlowerRequestResult
	// flowerErr 是自动求花测试使用的平台传输错误。
	flowerErr error
	// receiptReminderCalls 统计官方确认收货系统卡片请求次数。
	receiptReminderCalls int
	// receiptReminderResult 是确认收货系统卡片测试使用的平台明确结果。
	receiptReminderResult *mtop.ReceiptConfirmationReminderResult
	// receiptReminderErr 是确认收货系统卡片测试使用的请求错误。
	receiptReminderErr error
}

// cancelingAccountTaskClient 在评价动作已经返回成功前取消调用方上下文，用于验证补偿收口不会依赖已取消请求。
type cancelingAccountTaskClient struct {
	// fakeAccountTaskClient 提供除取消场景外的账号任务平台响应。
	*fakeAccountTaskClient
	// cancel 负责模拟外部动作完成后请求生命周期结束。
	cancel context.CancelFunc
}

// RateBuyer 在记录远端动作成功结果前取消调用方上下文。
func (c *cancelingAccountTaskClient) RateBuyer(_ context.Context, cookiesStr, tradeID, feedback string) (*mtop.AccountTaskResult, error) {
	c.rateCalls++
	c.cancel()
	return &mtop.AccountTaskResult{Success: true, Message: "ok"}, nil
}

// FetchPendingRateOrders 封装FetchPendingRate订单列表业务协调。
func (f *fakeAccountTaskClient) FetchPendingRateOrders(context.Context, string, int, int) (*mtop.PendingRateResult, error) {
	f.pendingCalls++
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return &mtop.PendingRateResult{Orders: f.pending}, nil
}

// RateBuyer 封装Rate买家业务协调。
func (f *fakeAccountTaskClient) RateBuyer(context.Context, string, string, string) (*mtop.AccountTaskResult, error) {
	f.rateCalls++
	if f.rateErr != nil {
		return nil, f.rateErr
	}
	return &mtop.AccountTaskResult{Success: true, Message: "ok"}, nil
}

// FetchAllItems 封装FetchAll商品列表业务协调。
func (f *fakeAccountTaskClient) FetchAllItems(_ context.Context, _ string, pageSize, maxPages int) (*mtop.ItemListResult, error) {
	f.fetchPageSize, f.fetchMaxPages = pageSize, maxPages
	if f.fetchItemsErr != nil {
		return nil, f.fetchItemsErr
	}
	return &mtop.ItemListResult{Items: f.items}, nil
}

// PolishItem 封装Polish商品业务协调。
func (f *fakeAccountTaskClient) PolishItem(context.Context, string, string) (*mtop.AccountTaskResult, error) {
	f.polishCalls++
	if f.polishErr != nil {
		return nil, f.polishErr
	}
	return &mtop.AccountTaskResult{Success: true, Message: "ok"}, nil
}

// RequestRedFlower 返回自动求花测试预置的平台结果。
func (f *fakeAccountTaskClient) RequestRedFlower(context.Context, string, string, string) (*mtop.RedFlowerRequestResult, error) {
	f.flowerCalls++
	if f.flowerErr != nil {
		return nil, f.flowerErr
	}
	if f.flowerResult != nil {
		return f.flowerResult, nil
	}
	return &mtop.RedFlowerRequestResult{Success: true, Message: "求花成功"}, nil
}

// RemindBuyerConfirmReceipt 返回确认收货系统卡片测试预置的平台结果。
func (f *fakeAccountTaskClient) RemindBuyerConfirmReceipt(context.Context, string, string) (*mtop.ReceiptConfirmationReminderResult, error) {
	f.receiptReminderCalls++
	if f.receiptReminderErr != nil {
		return nil, f.receiptReminderErr
	}
	if f.receiptReminderResult != nil {
		return f.receiptReminderResult, nil
	}
	return &mtop.ReceiptConfirmationReminderResult{Success: true, Message: "已提醒买家确认收货"}, nil
}

// TestAutoRequestFlowerSharesOrderIdempotency 验证发货后补偿扫描与人工入口共用订单幂等键且不会重复调用平台。
func TestAutoRequestFlowerSharesOrderIdempotency(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是本次自动求花扫描共用的测试上下文。
	ctx := context.Background()
	// now 是订单付款、发货时间和候选资格的当前基准。
	now := time.Now().UTC()
	// _, insertErr 是已发货两秒订单写入结果和错误。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders
		(order_id,item_id,buyer_id,order_status,cookie_id,chat_id,paid_at,shipped_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, "auto-flower-order", "item-1", "buyer-1", "shipped", "cid", "chat-1",
		now.Add(-2*time.Hour).Format(time.RFC3339Nano), now.Add(-2*time.Second).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	// settingsErr 是自动求花账号设置写入错误。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoRequestFlowerEnabled: true,
		RequestFlowerAfterHours: 1, RequestFlowerAfterSeconds: 1, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	})
	if settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// client 记录自动求花平台调用次数并返回明确成功。
	client := &fakeAccountTaskClient{flowerResult: &mtop.RedFlowerRequestResult{Success: true, Message: "求花成功"}}
	// center 是使用测试客户端的自动化中心。
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	center.scanAccountTasks(ctx)
	center.scanAccountTasks(ctx)
	if client.flowerCalls != 1 {
		t.Fatalf("flower calls=%d want 1", client.flowerCalls)
	}
	// run、exists、runErr 是人工与自动入口共用幂等键对应的持久化终态。
	run, exists, runErr := store.AccountTasks.GetRunByKey(ctx, "red_flower_request:cid:auto-flower-order")
	if runErr != nil || !exists || run.Status != "success" || run.SuccessCount != 1 {
		t.Fatalf("run=%+v exists=%v err=%v", run, exists, runErr)
	}
}

// TestAutoRequestFlowerQuarantinesUncertainResult 验证传输错误不会自动重放可能已经发送的求花卡片。
func TestAutoRequestFlowerQuarantinesUncertainResult(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是不明确求花结果测试使用的上下文。
	ctx := context.Background()
	// now 是订单付款和发货时间基准。
	now := time.Now().UTC()
	// _, insertErr 是发货后到期订单写入结果和错误。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders
		(order_id,order_status,cookie_id,chat_id,paid_at,shipped_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?)`, "uncertain-flower-order", "completed", "cid", "chat-1",
		now.Add(-2*time.Hour).Format(time.RFC3339Nano), now.Add(-2*time.Second).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	// settingsErr 是自动求花配置写入错误。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoRequestFlowerEnabled: true,
		RequestFlowerAfterHours: 1, RequestFlowerAfterSeconds: 1, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	})
	if settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// client 返回无法确定远端是否执行的传输错误。
	client := &fakeAccountTaskClient{flowerErr: errors.New("synthetic transport timeout")}
	// center 是待验证结果隔离策略的自动化中心。
	center := NewWithDependencies(store, nil, nil, CenterDependencies{AccountTaskClient: client})
	center.scanAccountTasks(ctx)
	center.scanAccountTasks(ctx)
	if client.flowerCalls != 1 {
		t.Fatalf("uncertain flower calls=%d want 1", client.flowerCalls)
	}
	// run、exists、runErr 是不明确动作隔离后的运行状态。
	run, exists, runErr := store.AccountTasks.GetRunByKey(ctx, "red_flower_request:cid:uncertain-flower-order")
	if runErr != nil || !exists || run.Status != "needs_review" || !strings.Contains(run.ErrorMessage, "禁止自动重放") {
		t.Fatalf("run=%+v exists=%v err=%v", run, exists, runErr)
	}
}

// TestRedFlowerRequestDueEnforcesDelayStatusAndWindow 验证自动求花资格同时受发货状态、秒级延迟和三十天期限约束。
func TestRedFlowerRequestDueEnforcesDelayStatusAndWindow(t *testing.T) {
	// now 是所有资格分支共用的固定 UTC 时间。
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	// eligible 是付款两小时、发货两秒且已完成的订单候选。
	eligible := db.FlowerRequestCandidate{OrderID: "1", OrderStatus: "completed", PaidAt: now.Add(-2 * time.Hour).Format(time.RFC3339), ShippedAt: now.Add(-2 * time.Second).Format(time.RFC3339Nano)}
	if !redFlowerRequestDue(eligible, 1, now) {
		t.Fatal("发货后到期订单应具备自动求花资格")
	}
	// received 是买家已确认收货但交易尚未完成的候选，仍属于允许求花的交易后阶段。
	received := eligible
	received.OrderStatus = "received"
	if !redFlowerRequestDue(received, 1, now) {
		t.Fatal("已收货订单应具备自动求花资格")
	}
	// pending 是发货时间尚未达到配置秒级延迟的候选。
	pending := eligible
	pending.ShippedAt = now.Add(-500 * time.Millisecond).Format(time.RFC3339Nano)
	if redFlowerRequestDue(pending, 1, now) {
		t.Fatal("发货后未达到延迟的订单不能自动求花")
	}
	// expired 是超过闲鱼官方三十天期限的候选。
	expired := eligible
	expired.PaidAt = now.Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	if redFlowerRequestDue(expired, 1, now) {
		t.Fatal("超过三十天的订单不能自动求花")
	}
	// cancelled 是已取消订单候选。
	cancelled := eligible
	cancelled.OrderStatus = "cancelled"
	if redFlowerRequestDue(cancelled, 1, now) {
		t.Fatal("已取消订单不能自动求花")
	}
	// refunded 是钱款已经退回的订单候选，必须阻止自动求花。
	refunded := eligible
	refunded.OrderStatus = "refunded"
	if redFlowerRequestDue(refunded, 1, now) {
		t.Fatal("已退款订单不能自动求花")
	}
}

// TestScheduleRedFlowerAfterShipmentRunsThroughSecondQueue 验证发货成功事实会创建可跨进程恢复的秒级任务并执行一次求花。
func TestScheduleRedFlowerAfterShipmentRunsThroughSecondQueue(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是秒级任务创建和领取使用的上下文。
	ctx := context.Background()
	// now 是付款、发货和到期时间的共同基准。
	now := time.Now().UTC()
	// _, insertErr 是已明确发货且秒级延迟已到期的订单写入结果。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders
		(order_id,item_id,buyer_id,order_status,cookie_id,chat_id,paid_at,shipped_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, "scheduled-flower-order", "item-1", "buyer-1", "shipped", "cid", "chat-1",
		now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(-2*time.Second).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	// settingsErr 是发货后十秒配置写入错误；测试使用一秒使夹具无需真实等待。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoRequestFlowerEnabled: true,
		RequestFlowerAfterHours: 24, RequestFlowerAfterSeconds: 1, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	})
	if settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// client 返回平台明确接受并记录调用次数。
	client := &fakeAccountTaskClient{flowerResult: &mtop.RedFlowerRequestResult{Success: true, Message: "求花成功"}}
	// center 是使用测试客户端的自动化中心。
	center := NewWithDependencies(store, nil, nil, CenterDependencies{AccountTaskClient: client})
	if // scheduleErr 是明确发货后创建秒级任务的结果。
	scheduleErr := center.ScheduleRedFlowerAfterShipment(ctx, "scheduled-flower-order"); scheduleErr != nil {
		t.Fatal(scheduleErr)
	}
	// pendingBefore 是秒级调度器领取前的持久化任务数量。
	var pendingBefore int
	if // countErr 是领取前秒级任务数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_pending_tasks WHERE task_key=?`, scheduledFlowerRequestKey("cid", "scheduled-flower-order")).Scan(&pendingBefore); countErr != nil || pendingBefore != 1 {
		t.Fatalf("pending before=%d err=%v", pendingBefore, countErr)
	}
	// scheduler 只执行秒级待办领取，不运行分钟补偿扫描。
	scheduler := NewScheduler(center)
	if // runErr 是秒级调度器执行到期任务的错误。
	runErr := scheduler.runDeferredTasks(ctx); runErr != nil {
		t.Fatal(runErr)
	}
	if client.flowerCalls != 1 {
		t.Fatalf("flower calls=%d want 1", client.flowerCalls)
	}
	// pendingAfter 是平台成功后应被删除的秒级任务数量。
	var pendingAfter int
	if // countErr 是平台成功后秒级任务数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_pending_tasks WHERE task_key=?`, scheduledFlowerRequestKey("cid", "scheduled-flower-order")).Scan(&pendingAfter); countErr != nil || pendingAfter != 0 {
		t.Fatalf("pending after=%d err=%v", pendingAfter, countErr)
	}
}

// TestScheduleRedFlowerAfterShipmentRequiresEnabledCompleteFact 验证关闭开关或缺少发货时间时不会创建平台动作任务。
func TestScheduleRedFlowerAfterShipmentRequiresEnabledCompleteFact(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是发货事实门禁测试上下文。
	ctx := context.Background()
	// now 是订单付款时间基准。
	now := time.Now().UTC()
	// _, insertErr 是缺少 shipped_at 的待发货订单写入结果。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,order_status,cookie_id,paid_at) VALUES(?,?,?,?)`,
		"not-shipped-flower-order", "pending_ship", "cid", now.Add(-time.Hour).Format(time.RFC3339Nano))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	// disabledErr 是开关关闭时的安全忽略结果。
	disabledErr := New(store, nil, nil).ScheduleRedFlowerAfterShipment(ctx, "not-shipped-flower-order")
	if disabledErr != nil {
		t.Fatal(disabledErr)
	}
	// settingsErr 是打开自动求花后的配置写入错误。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoRequestFlowerEnabled: true,
		RequestFlowerAfterHours: 24, RequestFlowerAfterSeconds: 10, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	})
	if settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// incompleteErr 是开关已开但发货事实不完整时的拒绝错误。
	incompleteErr := New(store, nil, nil).ScheduleRedFlowerAfterShipment(ctx, "not-shipped-flower-order")
	if incompleteErr == nil {
		t.Fatal("缺少明确发货时间时不能创建自动求花任务")
	}
	// pending 是所有门禁分支完成后的持久化任务数量。
	var pending int
	if // countErr 是所有门禁分支后的任务数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_pending_tasks`).Scan(&pending); countErr != nil || pending != 0 {
		t.Fatalf("pending=%d err=%v", pending, countErr)
	}
}

// TestScheduledFlowerRechecksCancelledOrderWithoutRegression 验证到期任务不会把取消订单的旧发货快照写回或调用平台。
func TestScheduledFlowerRechecksCancelledOrderWithoutRegression(t *testing.T) {
	// store、cleanup 是隔离数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是任务创建、状态变化和领取使用的上下文。
	ctx := context.Background()
	// now 是付款和发货时间基准。
	now := time.Now().UTC()
	// _, insertErr 是最初满足发货成功条件的订单写入结果。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders
		(order_id,order_status,cookie_id,paid_at,shipped_at) VALUES(?,?,?,?,?)`,
		"cancelled-after-shipment", "shipped", "cid", now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	if // settingsErr 是取消状态复核测试的自动求花配置写入错误。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoRequestFlowerEnabled: true,
		RequestFlowerAfterHours: 24, RequestFlowerAfterSeconds: 10, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	}); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// client 会记录是否错误调用了求花平台。
	client := &fakeAccountTaskClient{}
	// center 是待验证当前订单状态复核的自动化中心。
	center := NewWithDependencies(store, nil, nil, CenterDependencies{AccountTaskClient: client})
	if // scheduleErr 是订单取消前秒级求花任务创建结果。
	scheduleErr := center.ScheduleRedFlowerAfterShipment(ctx, "cancelled-after-shipment"); scheduleErr != nil {
		t.Fatal(scheduleErr)
	}
	// cancelled 是调度后、执行前发生的订单取消状态。
	cancelled := "cancelled"
	if // patchErr 是执行前把订单推进到取消状态的写入错误。
	patchErr := store.Orders.Patch(ctx, "cancelled-after-shipment", db.OrderPatch{OrderStatus: &cancelled}); patchErr != nil {
		t.Fatal(patchErr)
	}
	if // runErr 是取消订单到期任务的领取结果。
	runErr := NewScheduler(center).runDeferredTasks(ctx); runErr != nil {
		t.Fatal(runErr)
	}
	if client.flowerCalls != 0 {
		t.Fatalf("cancelled order flower calls=%d", client.flowerCalls)
	}
	// order、orderErr 是任务执行后的当前订单，状态必须保持取消。
	order, orderErr := store.Orders.Get(ctx, "cancelled-after-shipment")
	if orderErr != nil || db.NormalizeOrderStatus(order.OrderStatus) != "cancelled" {
		t.Fatalf("order=%+v err=%v", order, orderErr)
	}
}

// TestAccountTaskRateIsOrderIdempotent 封装Test账号任务RateIs订单Idempotent业务协调。
func TestAccountTaskRateIsOrderIdempotent(t *testing.T) {
	// store、cleanup 用于本次流程后续判断的store、cleanup
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// client 用于本次流程后续判断的client
	client := &fakeAccountTaskClient{pending: []mtop.PendingRateOrder{{TradeID: "order-1"}, {TradeID: "order-2"}}}
	// center 用于本次流程后续判断的center
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	if // err 用于本次流程后续判断的err
	err := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoRateEnabled: true,
		RateContent: "交易愉快", PolishTime: "03:00"}); err != nil {
		t.Fatal(err)
	}
	// first、err 用于本次流程后续判断的first、err
	first, err := center.RunAccountTask(ctx, "cid", TaskAutoRate)
	if err != nil || first.Success != 2 || client.rateCalls != 2 {
		t.Fatalf("first=%+v calls=%d err=%v", first, client.rateCalls, err)
	}
	// second、err 用于本次流程后续判断的second、err
	second, err := center.RunAccountTask(ctx, "cid", TaskAutoRate)
	if err != nil || second.Skipped != 2 || client.rateCalls != 2 {
		t.Fatalf("second=%+v calls=%d err=%v", second, client.rateCalls, err)
	}
}

// TestAccountTaskRateFinishFailureQuarantinesExternalSuccess 验证评价动作已成功但运行结果写入失败时会隔离记录，避免下一轮重复评价。
func TestAccountTaskRateFinishFailureQuarantinesExternalSuccess(t *testing.T) {
	// store 是当前测试使用的 SQLite 自动化存储。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是测试数据库操作共用的上下文。
	ctx := context.Background()
	// triggerErr 表示故意阻止 success 状态写入的 SQLite 触发器创建错误。
	if _, triggerErr := store.DB.ExecContext(ctx, `CREATE TRIGGER reject_account_task_success
		BEFORE UPDATE OF status ON account_task_runs
		WHEN NEW.status='success'
		BEGIN SELECT RAISE(ABORT, 'forced account task finish failure'); END`); triggerErr != nil {
		t.Fatal(triggerErr)
	}
	// client 记录远端评价调用次数，验证本地状态异常不会触发同一轮的重复外部动作。
	client := &fakeAccountTaskClient{pending: []mtop.PendingRateOrder{{TradeID: "order-finish-failure"}}}
	// center 是待验证账号任务结果隔离逻辑的自动化中心。
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// settingsErr 表示写入自动评价设置时的数据库错误。
	if settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoRateEnabled: true,
		RateContent: "交易愉快", PolishTime: "03:00"}); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// runErr 保存外部动作成功但本地运行结果收口失败后的人工核对错误。
	_, runErr := center.RunAccountTask(ctx, "cid", TaskAutoRate)
	if !errors.Is(runErr, errAutomationNeedsReview) || !strings.Contains(runErr.Error(), "保存账号任务运行结果失败") {
		t.Fatalf("运行结果写入失败应返回人工核对错误，err=%v", runErr)
	}
	if client.rateCalls != 1 {
		t.Fatalf("外部评价动作应只执行一次，calls=%d", client.rateCalls)
	}
	// status、successCount、message 保存隔离后的任务状态、已完成动作数和人工核对原因。
	var status, message string
	// successCount 保存已确认完成的评价动作数量。
	var successCount int
	// queryErr 表示读取隔离后任务状态时的数据库错误。
	queryErr := store.DB.QueryRowContext(ctx, `SELECT status,success_count,error_message FROM account_task_runs WHERE run_key=?`,
		"rate:cid:order-finish-failure").Scan(&status, &successCount, &message)
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	if status != "needs_review" || successCount != 1 || !strings.Contains(message, "禁止自动重放") {
		t.Fatalf("任务未正确隔离: status=%q success=%d message=%q", status, successCount, message)
	}
}

// TestAccountTaskRateFinishAndQuarantineFailureJoinsErrors 验证运行结果和人工核对状态均无法落库时不会吞掉任一错误。
func TestAccountTaskRateFinishAndQuarantineFailureJoinsErrors(t *testing.T) {
	// store 是当前测试使用的 SQLite 自动化存储。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是测试数据库操作共用的上下文。
	ctx := context.Background()
	// triggerErr 表示故意阻止 success 与 needs_review 状态写入的 SQLite 触发器创建错误。
	if _, triggerErr := store.DB.ExecContext(ctx, `CREATE TRIGGER reject_account_task_result_states
		BEFORE UPDATE OF status ON account_task_runs
		WHEN NEW.status IN ('success','needs_review')
		BEGIN SELECT RAISE(ABORT, 'forced account task result failure'); END`); triggerErr != nil {
		t.Fatal(triggerErr)
	}
	// client 记录已经成功执行的远端评价动作。
	client := &fakeAccountTaskClient{pending: []mtop.PendingRateOrder{{TradeID: "order-double-failure"}}}
	// center 是待验证双重落库错误传播逻辑的自动化中心。
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// settingsErr 表示写入自动评价设置时的数据库错误。
	if settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoRateEnabled: true,
		RateContent: "交易愉快", PolishTime: "03:00"}); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// runErr 保存结果写入和隔离写入均失败后的组合错误。
	_, runErr := center.RunAccountTask(ctx, "cid", TaskAutoRate)
	if !errors.Is(runErr, errAutomationNeedsReview) || !strings.Contains(runErr.Error(), "保存账号任务运行结果失败") ||
		!strings.Contains(runErr.Error(), "保存账号任务人工核对状态失败") {
		t.Fatalf("双重落库失败应返回完整错误链，err=%v", runErr)
	}
	if client.rateCalls != 1 {
		t.Fatalf("外部评价动作应只执行一次，calls=%d", client.rateCalls)
	}
}

// TestAccountTaskRateCancellationQuarantinesAndBlocksRetry 验证外部评价成功后请求取消仍能隔离运行并阻止重复重试。
func TestAccountTaskRateCancellationQuarantinesAndBlocksRetry(t *testing.T) {
	// store 是当前测试使用的 SQLite 自动化存储。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// setupCtx 是测试设置写入共用的数据库上下文。
	setupCtx := context.Background()
	// settingsErr 表示写入自动评价设置时的数据库错误。
	if settingsErr := store.AccountTasks.Upsert(setupCtx, db.AccountTaskSettings{CookieID: "cid", AutoRateEnabled: true,
		RateContent: "交易愉快", PolishTime: "03:00"}); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// requestCtx、cancel 模拟外部动作完成后被上层请求取消的生命周期。
	requestCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// client 在评价动作返回成功前取消 requestCtx，并保留调用次数用于重试断言。
	client := &cancelingAccountTaskClient{
		fakeAccountTaskClient: &fakeAccountTaskClient{pending: []mtop.PendingRateOrder{{TradeID: "order-cancelled"}}},
		cancel:                cancel,
	}
	// center 是待验证取消后补偿收口逻辑的自动化中心。
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// runErr 保存外部评价已完成但调用方取消后的人工核对错误。
	_, runErr := center.RunAccountTask(requestCtx, "cid", TaskAutoRate)
	if !errors.Is(runErr, errAutomationNeedsReview) || !strings.Contains(runErr.Error(), "保存账号任务运行结果失败") {
		t.Fatalf("取消后的外部成功应隔离运行，err=%v", runErr)
	}
	if client.rateCalls != 1 {
		t.Fatalf("首次外部评价调用次数=%d want 1", client.rateCalls)
	}
	// status、successCount 保存补偿收口后的任务状态和已确认成功数量。
	var status string
	// successCount 保存数据库记录的外部评价成功数，用于确认取消请求未抹掉已经完成的动作结果。
	var successCount int
	// queryErr 表示读取补偿状态时的数据库错误。
	queryErr := store.DB.QueryRowContext(context.Background(), `SELECT status,success_count FROM account_task_runs WHERE run_key=?`,
		"rate:cid:order-cancelled").Scan(&status, &successCount)
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	if status != "needs_review" || successCount != 1 {
		t.Fatalf("取消后任务状态错误: status=%q success=%d", status, successCount)
	}
	// retrySummary、retryErr 保存新请求尝试重跑时的结果，确认人工核对状态不会被自动重放。
	retrySummary, retryErr := center.RunAccountTask(context.Background(), "cid", TaskAutoRate)
	if retryErr != nil || retrySummary.Skipped != 1 || client.rateCalls != 1 {
		t.Fatalf("人工核对任务不应自动重试: summary=%+v calls=%d err=%v", retrySummary, client.rateCalls, retryErr)
	}
}

// TestAccountTaskPolishMarkFailureQuarantinesExternalSuccess 验证商品擦亮成功但日期索引写入失败时会隔离运行记录。
func TestAccountTaskPolishMarkFailureQuarantinesExternalSuccess(t *testing.T) {
	// store 是当前测试使用的 SQLite 自动化存储。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是测试数据库操作共用的上下文。
	ctx := context.Background()
	// settingsErr 表示写入擦亮设置时的数据库错误。
	if settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoPolishEnabled: true,
		RateContent: "交易愉快", PolishTime: "00:00"}); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// triggerErr 表示故意阻止擦亮日期索引写入的 SQLite 触发器创建错误。
	if _, triggerErr := store.DB.ExecContext(ctx, `CREATE TRIGGER reject_account_task_polish_mark
		BEFORE UPDATE OF last_polish_date ON account_task_settings
		WHEN NEW.last_polish_date <> ''
		BEGIN SELECT RAISE(ABORT, 'forced account task polish mark failure'); END`); triggerErr != nil {
		t.Fatal(triggerErr)
	}
	// client 记录远端擦亮调用次数，验证本地状态异常不会导致同一轮重复执行。
	client := &fakeAccountTaskClient{items: []mtop.ItemListItem{{ID: "item-mark-failure"}}}
	// center 是待验证擦亮结果隔离逻辑的自动化中心。
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// runErr 保存外部擦亮成功但日期索引收口失败后的人工核对错误。
	_, runErr := center.RunAccountTask(ctx, "cid", TaskAutoPolish)
	if !errors.Is(runErr, errAutomationNeedsReview) || !strings.Contains(runErr.Error(), "保存商品擦亮日期") {
		t.Fatalf("擦亮日期写入失败应返回人工核对错误，err=%v", runErr)
	}
	if client.polishCalls != 1 {
		t.Fatalf("外部擦亮动作应只执行一次，calls=%d", client.polishCalls)
	}
	// status 保存被隔离的擦亮运行状态。
	var status string
	// queryErr 表示读取隔离后擦亮运行状态时的数据库错误。
	queryErr := store.DB.QueryRowContext(ctx, `SELECT status FROM account_task_runs WHERE run_key LIKE 'polish:cid:%'`).Scan(&status)
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	if status != "needs_review" {
		t.Fatalf("擦亮日期写入失败后应隔离任务，status=%q", status)
	}
}

// TestAccountTaskSessionExpiredRecoversOnceAndBlocksFurtherAPIRequests 封装Test账号任务会话ExpiredRecoversOnceAndBlocksFurtherAPI请求列表业务协调。
func TestAccountTaskSessionExpiredRecoversOnceAndBlocksFurtherAPIRequests(t *testing.T) {
	// store、cleanup 用于本次流程后续判断的store、cleanup
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	// sessionErr 用于本次流程后续判断的会话Err
	sessionErr := &mtop.SessionExpiredError{API: "自动评价接口", Ret: []string{"FAIL_SYS_SESSION_EXPIRED::Session过期"}}
	// client 用于本次流程后续判断的client
	client := &fakeAccountTaskClient{pendingErr: sessionErr}
	// recoverer 用于本次流程后续判断的recoverer
	recoverer := &fakeCredentialRecoverer{store: store, fail: true}
	// center 用于本次流程后续判断的center
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{
		AccountTaskClient:  client,
		OrderDetailFetcher: recoverer,
	})
	if // err 用于本次流程后续判断的err
	err := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoRateEnabled: true,
		RateContent: "交易愉快", PolishTime: "03:00"}); err != nil {
		t.Fatal(err)
	}

	if // err 用于本次流程后续判断的err
	_, err := center.RunAccountTask(ctx, "cid", TaskAutoRate); err == nil || !mtop.IsSessionExpiredErr(err) {
		t.Fatalf("首次 session 失效应触发续期并返回原始分类错误: %v", err)
	}
	if client.pendingCalls != 1 || recoverer.calls != 1 {
		t.Fatalf("first calls: api=%d recover=%d want 1/1", client.pendingCalls, recoverer.calls)
	}
	if // err 用于本次流程后续判断的err
	_, err := center.RunAccountTask(ctx, "cid", TaskAutoRate); err == nil || !strings.Contains(err.Error(), "已停止自动化 API 请求") {
		t.Fatalf("未更新凭证时应保持阻断: %v", err)
	}
	if client.pendingCalls != 1 || recoverer.calls != 1 {
		t.Fatalf("blocked run must not call API/recovery again: api=%d recover=%d", client.pendingCalls, recoverer.calls)
	}

	if // err 用于本次流程后续判断的err
	err := store.Cookies.UpdateValueExisting(ctx, "cid", "unb=1; _m_h5_tk=fresh_1; renewed=1"); err != nil {
		t.Fatal(err)
	}
	client.pendingErr = nil
	if // err 用于本次流程后续判断的err
	_, err := center.RunAccountTask(ctx, "cid", TaskAutoRate); err != nil {
		t.Fatalf("凭证变化后应自动解除阻断: %v", err)
	}
	if client.pendingCalls != 2 {
		t.Fatalf("api calls after credential update=%d want 2", client.pendingCalls)
	}
}

// TestAccountTaskStopsRemainingOrdersOnSessionExpired 封装Test账号任务StopsRemaining订单列表On会话Expired业务协调。
func TestAccountTaskStopsRemainingOrdersOnSessionExpired(t *testing.T) {
	// store、cleanup 用于本次流程后续判断的store、cleanup
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	// sessionErr 用于本次流程后续判断的会话Err
	sessionErr := &mtop.SessionExpiredError{API: "评价接口", Ret: []string{"FAIL_SYS_SESSION_EXPIRED::Session过期"}}
	// client 用于本次流程后续判断的client
	client := &fakeAccountTaskClient{
		pending: []mtop.PendingRateOrder{{TradeID: "order-1"}, {TradeID: "order-2"}},
		rateErr: sessionErr,
	}
	// recoverer 用于本次流程后续判断的recoverer
	recoverer := &fakeCredentialRecoverer{store: store, fail: true}
	// center 用于本次流程后续判断的center
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{
		AccountTaskClient:  client,
		OrderDetailFetcher: recoverer,
	})
	if // err 用于本次流程后续判断的err
	err := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoRateEnabled: true,
		RateContent: "交易愉快", PolishTime: "03:00"}); err != nil {
		t.Fatal(err)
	}

	if // err 用于本次流程后续判断的err
	_, err := center.RunAccountTask(ctx, "cid", TaskAutoRate); err == nil {
		t.Fatal("session expiry must be returned")
	}
	if client.rateCalls != 1 || recoverer.calls != 1 {
		t.Fatalf("remaining orders must stop immediately: rate=%d recover=%d", client.rateCalls, recoverer.calls)
	}
}

// TestAccountTaskPolishRunsOncePerBeijingDay 封装Test账号任务Polish运行记录OncePerBeijingDay业务协调。
func TestAccountTaskPolishRunsOncePerBeijingDay(t *testing.T) {
	// store、cleanup 用于本次流程后续判断的store、cleanup
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// client 用于本次流程后续判断的client
	client := &fakeAccountTaskClient{items: []mtop.ItemListItem{{ID: "item-1"}, {ID: "item-2"}}}
	// center 用于本次流程后续判断的center
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	if // err 用于本次流程后续判断的err
	err := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoPolishEnabled: true,
		RateContent: "交易愉快", PolishTime: "00:00"}); err != nil {
		t.Fatal(err)
	}
	// first、err 用于本次流程后续判断的first、err
	first, err := center.RunAccountTask(ctx, "cid", TaskAutoPolish)
	if err != nil || first.Success != 2 || client.polishCalls != 2 {
		t.Fatalf("first=%+v calls=%d err=%v", first, client.polishCalls, err)
	}
	if client.fetchPageSize != 20 || client.fetchMaxPages != 20 {
		t.Fatalf("unexpected item pagination: pageSize=%d maxPages=%d", client.fetchPageSize, client.fetchMaxPages)
	}
	// second、err 用于本次流程后续判断的second、err
	second, err := center.RunAccountTask(ctx, "cid", TaskAutoPolish)
	if err != nil || second.Skipped != 1 || client.polishCalls != 2 {
		t.Fatalf("second=%+v calls=%d err=%v", second, client.polishCalls, err)
	}
	// settings、err 用于本次流程后续判断的settings、err
	settings, err := store.AccountTasks.Get(ctx, "cid")
	if err != nil || settings.LastPolishDate != beijingNow().Format("2006-01-02") {
		t.Fatalf("settings=%+v err=%v", settings, err)
	}
}

// TestManualPolishReportsItemFailures 封装TestManualPolishReports商品Failures业务协调。
func TestManualPolishReportsItemFailures(t *testing.T) {
	// store、cleanup 用于本次流程后续判断的store、cleanup
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// client 用于本次流程后续判断的client
	client := &fakeAccountTaskClient{items: []mtop.ItemListItem{{ID: "item-1"}}, polishErr: errors.New("both polish APIs failed")}
	// center 用于本次流程后续判断的center
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	if // err 用于本次流程后续判断的err
	err := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoPolishEnabled: true,
		RateContent: "交易愉快", PolishTime: "00:00"}); err != nil {
		t.Fatal(err)
	}
	// summary、err 用于本次流程后续判断的summary、err
	summary, err := center.RunAccountTask(ctx, "cid", TaskAutoPolish)
	if err == nil || summary.Failed != 1 || summary.Success != 0 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
}

// TestManualPolishCanRetryImmediatelyAfterFailure 封装TestManualPolishCan重试ImmediatelyAfterFailure业务协调。
func TestManualPolishCanRetryImmediatelyAfterFailure(t *testing.T) {
	// store、cleanup 用于本次流程后续判断的store、cleanup
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// client 用于本次流程后续判断的client
	client := &fakeAccountTaskClient{items: []mtop.ItemListItem{{ID: "item-1"}}, fetchItemsErr: errors.New("upstream 502")}
	// center 用于本次流程后续判断的center
	center := NewWithDependencies(store, testSenderProvider{sender: &testSender{}}, nil, CenterDependencies{AccountTaskClient: client})
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	if // err 用于本次流程后续判断的err
	err := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{CookieID: "cid", AutoPolishEnabled: true,
		RateContent: "交易愉快", PolishTime: "00:00"}); err != nil {
		t.Fatal(err)
	}
	if // err 用于本次流程后续判断的err
	_, err := center.RunAccountTask(ctx, "cid", TaskAutoPolish); err == nil {
		t.Fatal("first polish should expose the upstream failure")
	}
	client.fetchItemsErr = nil
	// second、err 用于本次流程后续判断的second、err
	second, err := center.RunAccountTask(ctx, "cid", TaskAutoPolish)
	if err != nil || second.Success != 1 || second.Skipped != 0 || client.polishCalls != 1 {
		t.Fatalf("manual retry=%+v calls=%d err=%v", second, client.polishCalls, err)
	}
	// third、err 用于本次流程后续判断的third、err
	third, err := center.RunAccountTask(ctx, "cid", TaskAutoPolish)
	if err != nil || third.Skipped != 1 || client.polishCalls != 1 {
		t.Fatalf("successful day must remain idempotent: summary=%+v calls=%d err=%v", third, client.polishCalls, err)
	}
}

// TestPolishDueHonorsConfiguredTimeAndDate 封装TestPolishDueHonorsConfigured时间And日期业务协调。
func TestPolishDueHonorsConfiguredTimeAndDate(t *testing.T) {
	// now 用于本次流程后续判断的now
	now := beijingNow()
	// settings 用于本次流程后续判断的设置
	settings := db.AccountTaskSettings{PolishTime: now.Add(2 * time.Hour).Format("15:04")}
	if polishDue(settings, now) && now.Hour() < 22 {
		t.Fatal("task must not run before configured time")
	}
	settings.PolishTime = "00:00"
	if !polishDue(settings, now) {
		t.Fatal("task should be due after midnight")
	}
	settings.LastPolishDate = now.Format("2006-01-02")
	if polishDue(settings, now) {
		t.Fatal("task must not run twice on the same Beijing date")
	}
}
