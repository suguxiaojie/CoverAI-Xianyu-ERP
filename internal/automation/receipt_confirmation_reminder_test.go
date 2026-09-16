package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// TestReceiptConfirmationReminderDueUsesFullDaysAndNextBeijingTime 验证满完整天数后只在下一次北京时间到期。
func TestReceiptConfirmationReminderDueUsesFullDaysAndNextBeijingTime(t *testing.T) {
	// location 是确认收货提醒固定使用的北京时间。
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	// enabledAt 是开关开启基线，早于测试订单发货时间。
	enabledAt := time.Date(2026, 8, 1, 8, 0, 0, 0, location)
	// shippedAt 是下午发货时间；满两天时当天 10:00 已过去，所以应顺延到第三天 10:00。
	shippedAt := time.Date(2026, 8, 2, 15, 0, 0, 0, location)
	// candidate 是具备可靠状态、买家和会话的订单候选。
	candidate := db.ReceiptReminderCandidate{OrderID: "order-1", CookieID: "cid", ChatID: "chat-1", BuyerID: "buyer-1", OrderStatus: "shipped", ShippedAt: shippedAt.UTC().Format(time.RFC3339)}
	// settings 是发货满两天后每天十点执行的启用设置。
	settings := db.AccountTaskSettings{AutoReceiptReminderEnabled: true, ReceiptReminderAfterDays: 2, ReceiptReminderTime: "10:00", ReceiptReminderEnabledAt: enabledAt.UTC().Unix()}
	if receiptConfirmationReminderDue(candidate, settings, time.Date(2026, 8, 4, 17, 1, 0, 0, location)) {
		t.Fatal("满两天当天 10:00 已过去时不得提前发送")
	}
	if !receiptConfirmationReminderDue(candidate, settings, time.Date(2026, 8, 5, 10, 0, 0, 0, location)) {
		t.Fatal("满两天后的下一次北京时间 10:00 应到期")
	}
	// oldCandidate 是开启前已发货的历史订单，必须被启用基线排除。
	oldCandidate := candidate
	oldCandidate.ShippedAt = enabledAt.Add(-time.Hour).UTC().Format(time.RFC3339)
	if receiptConfirmationReminderDue(oldCandidate, settings, time.Date(2026, 8, 10, 10, 0, 0, 0, location)) {
		t.Fatal("开启前历史订单不得追发提醒")
	}
	// completedCandidate 模拟已经完成的订单，任何时间都不得提醒。
	completedCandidate := candidate
	completedCandidate.OrderStatus = "completed"
	if receiptConfirmationReminderDue(completedCandidate, settings, time.Date(2026, 8, 10, 10, 0, 0, 0, location)) {
		t.Fatal("已完成订单不得发送确认收货提醒")
	}
}

// TestReceiptReminderSendsOnceAndPersistsSuccess 验证到期订单只发送一次并保存永久幂等成功运行。
func TestReceiptReminderSendsOnceAndPersistsSuccess(t *testing.T) {
	// store、cleanup 是隔离数据库和释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是设置、订单和调度器调用共用的上下文。
	ctx := context.Background()
	// location 是测试调度时刻使用的北京时间。
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	// enabledAt、shippedAt、now 分别是开关基线、发货时间和到期扫描时间。
	enabledAt := time.Date(2026, 8, 1, 0, 0, 0, 0, location)
	// shippedAt 是启用基线后的可靠发货时间。
	shippedAt := time.Date(2026, 8, 2, 9, 0, 0, 0, location)
	// now 是发货满两天后配置时刻已经到达的扫描时间。
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, location)
	// settings 是当前店铺已启用的单次提醒配置。
	settings := db.AccountTaskSettings{CookieID: "cid", AutoReceiptReminderEnabled: true, ReceiptReminderAfterDays: 2, ReceiptReminderTime: "10:00", ReceiptReminderMessage: "请确认收货", ReceiptReminderEnabledAt: enabledAt.UTC().Unix()}
	if // settingsErr 是保存提醒设置的错误。
	settingsErr := store.AccountTasks.Upsert(ctx, settings); settingsErr != nil {
		t.Fatalf("保存提醒设置: %v", settingsErr)
	}
	if // orderErr 是保存提醒订单的错误。
	orderErr := store.Orders.Upsert(ctx, "receipt-order", db.OrderUpsertOpts{CookieID: "cid", BuyerID: "buyer-1", ItemID: "item-1", ChatID: "chat-1", OrderStatus: "shipped"}); orderErr != nil {
		t.Fatalf("保存提醒订单: %v", orderErr)
	}
	if // timeErr 是保存可靠发货时间的错误。
	_, timeErr := store.DB.ExecContext(ctx, `UPDATE orders SET shipped_at=? WHERE order_id=?`, shippedAt.UTC().Format(time.RFC3339), "receipt-order"); timeErr != nil {
		t.Fatalf("保存发货时间: %v", timeErr)
	}
	// taskClient 模拟闲鱼明确生成官方系统卡片，不访问真实平台。
	taskClient := &fakeAccountTaskClient{receiptReminderResult: &mtop.ReceiptConfirmationReminderResult{Success: true, Message: "已提醒买家确认收货"}}
	// textSender 记录普通聊天发送调用，系统卡片路径必须始终保持为空。
	textSender := &testSender{}
	// center 是使用隔离 Store 和测试官方接口的自动化中心。
	center := NewWithDependencies(store, testSenderProvider{sender: textSender}, nil, CenterDependencies{AccountTaskClient: taskClient})
	if // scanErr 是首次到期扫描和发送错误。
	scanErr := center.taskRunner.scanReceiptConfirmationReminders(ctx, settings, now); scanErr != nil {
		t.Fatalf("首次扫描: %v", scanErr)
	}
	if taskClient.receiptReminderCalls != 1 {
		t.Fatalf("官方系统卡片调用次数=%d", taskClient.receiptReminderCalls)
	}
	if len(textSender.texts) != 0 {
		t.Fatalf("系统卡片路径不应发送普通文字: %v", textSender.texts)
	}
	if // duplicateErr 是成功后重复扫描返回的错误。
	duplicateErr := center.taskRunner.scanReceiptConfirmationReminders(ctx, settings, now.Add(time.Minute)); duplicateErr != nil {
		t.Fatalf("重复扫描: %v", duplicateErr)
	}
	if taskClient.receiptReminderCalls != 1 {
		t.Fatalf("同订单系统卡片重复调用: %d", taskClient.receiptReminderCalls)
	}
	// runKey 是当前订单确认收货提醒的稳定幂等键。
	runKey := receiptConfirmationReminderRunKey("cid", "receipt-order")
	// run、exists、runErr 是成功运行的持久化结果。
	run, exists, runErr := store.AccountTasks.GetRunByKey(ctx, runKey)
	if runErr != nil || !exists || run.Status != "success" || run.SuccessCount != 1 {
		t.Fatalf("run=%+v exists=%v err=%v", run, exists, runErr)
	}
}

// TestReceiptReminderUnknownSendResultNeedsReviewAndNeverRetries 验证发送错误隔离后不会自动重放。
func TestReceiptReminderUnknownSendResultNeedsReviewAndNeverRetries(t *testing.T) {
	// store、cleanup 是隔离数据库和释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx 是当前错误分支测试上下文。
	ctx := context.Background()
	// now 是已达到提醒时刻的北京时间。
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	// settings 是启用且已经跨过历史基线的提醒设置。
	settings := db.AccountTaskSettings{CookieID: "cid", AutoReceiptReminderEnabled: true, ReceiptReminderAfterDays: 1, ReceiptReminderTime: "10:00", ReceiptReminderMessage: "请确认收货", ReceiptReminderEnabledAt: now.Add(-72 * time.Hour).Unix()}
	if // settingsErr 是保存错误分支提醒设置的错误。
	settingsErr := store.AccountTasks.Upsert(ctx, settings); settingsErr != nil {
		t.Fatal(settingsErr)
	}
	if // orderErr 是保存错误分支订单的错误。
	orderErr := store.Orders.Upsert(ctx, "uncertain-order", db.OrderUpsertOpts{CookieID: "cid", BuyerID: "buyer", ItemID: "item", ChatID: "chat", OrderStatus: "shipped"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	if // timeErr 是保存错误分支发货时间的错误。
	_, timeErr := store.DB.ExecContext(ctx, `UPDATE orders SET shipped_at=? WHERE order_id=?`, now.Add(-48*time.Hour).UTC().Format(time.RFC3339), "uncertain-order"); timeErr != nil {
		t.Fatal(timeErr)
	}
	// taskClient 返回无法确认闲鱼是否生成系统卡片的传输错误。
	taskClient := &fakeAccountTaskClient{receiptReminderErr: errors.New("mtop result unknown")}
	// textSender 记录普通聊天调用，错误分支同样不得降级发送文字。
	textSender := &testSender{}
	// center 使用错误官方接口验证人工核对隔离。
	center := NewWithDependencies(store, testSenderProvider{sender: textSender}, nil, CenterDependencies{AccountTaskClient: taskClient})
	if // scanErr 是结果不明确的首次发送错误。
	scanErr := center.taskRunner.scanReceiptConfirmationReminders(ctx, settings, now); scanErr == nil {
		t.Fatal("发送结果不明确应返回可观测错误")
	}
	// runKey 是不明确运行的稳定键。
	runKey := receiptConfirmationReminderRunKey("cid", "uncertain-order")
	// run、exists、runErr 是人工核对运行状态。
	run, exists, runErr := store.AccountTasks.GetRunByKey(ctx, runKey)
	if runErr != nil || !exists || run.Status != "needs_review" {
		t.Fatalf("run=%+v exists=%v err=%v", run, exists, runErr)
	}
	if // secondErr 是 needs_review 运行在后续扫描中的安全跳过结果。
	secondErr := center.taskRunner.scanReceiptConfirmationReminders(ctx, settings, now.Add(24*time.Hour)); secondErr != nil {
		t.Fatalf("人工核对运行应被安全跳过: %v", secondErr)
	}
	if len(textSender.texts) != 0 {
		t.Fatalf("结果不明确时不得降级发送普通文字: %v", textSender.texts)
	}
}
