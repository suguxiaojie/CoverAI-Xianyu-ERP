package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestAccountTaskReceiptReminderDefaultsAndCandidateResolution 验证 Schema 53 默认值及历史订单卡片唯一会话解析。
func TestAccountTaskReceiptReminderDefaultsAndCandidateResolution(t *testing.T) {
	// store、cleanup 是隔离 SQLite Store 和释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是账号设置、订单和聊天夹具共用的上下文。
	ctx := context.Background()
	// accountID 是当前测试用户拥有的店铺标识。
	_, accountID := seedAccount(t, store)
	// defaults、defaultErr 是尚未创建设置行时的安全默认值。
	defaults, defaultErr := store.AccountTasks.Get(ctx, accountID)
	if defaultErr != nil || defaults.AutoReceiptReminderEnabled || defaults.ReceiptReminderAfterDays != 2 || defaults.ReceiptReminderTime != "10:00" || defaults.ReceiptReminderMessage != DefaultReceiptReminderMessage || defaults.ReceiptReminderEnabledAt != 0 {
		t.Fatalf("defaults=%+v err=%v", defaults, defaultErr)
	}
	// settings 是待持久化的启用设置和历史追发基线。
	settings := defaults
	settings.AutoReceiptReminderEnabled = true
	settings.ReceiptReminderEnabledAt = 123
	if // settingsErr 是保存提醒设置的错误。
	settingsErr := store.AccountTasks.Upsert(ctx, settings); settingsErr != nil {
		t.Fatalf("保存提醒设置: %v", settingsErr)
	}
	// stored、storedErr 是设置写入后的完整读取结果。
	stored, storedErr := store.AccountTasks.Get(ctx, accountID)
	if storedErr != nil || !stored.AutoReceiptReminderEnabled || stored.ReceiptReminderEnabledAt != 123 {
		t.Fatalf("stored=%+v err=%v", stored, storedErr)
	}
	// orderErr 是写入缺少持久化 chat_id 的已发货订单错误。
	orderErr := store.Orders.Upsert(ctx, "receipt-db-order", OrderUpsertOpts{CookieID: accountID, BuyerID: "buyer-1", ItemID: "item-1", OrderStatus: "shipped"})
	if orderErr != nil {
		t.Fatal(orderErr)
	}
	// shippedAt 是候选调度使用的可靠发货时间。
	shippedAt := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC).Format(time.RFC3339)
	if // timeErr 是保存可靠发货时间的错误。
	_, timeErr := store.DB.ExecContext(ctx, `UPDATE orders SET shipped_at=? WHERE order_id=?`, shippedAt, "receipt-db-order"); timeErr != nil {
		t.Fatal(timeErr)
	}
	// session 是订单历史卡片所在的唯一会话。
	session := ChatSession{CookieID: accountID, ChatID: "receipt-chat", BuyerID: "buyer-1", ItemID: "item-1"}
	// message 是携带精确订单号的历史已发货系统卡片。
	message := ChatMessage{MessageKey: "receipt-db-card.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Status: "received", SentAt: 1000, SystemCardKind: "trade", SystemCardEvent: "order_shipped", SystemCardOrderID: "receipt-db-order"}
	if // saveErr 是保存历史订单系统卡片的错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, message, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// candidates、candidateErr 是按订单游标读取的提醒候选。
	candidates, candidateErr := store.AccountTasks.ListReceiptReminderCandidates(ctx, accountID, "", 20)
	if candidateErr != nil || len(candidates) != 1 || candidates[0].ChatID != "receipt-chat" || candidates[0].BuyerID != "buyer-1" || candidates[0].ShippedAt != shippedAt {
		t.Fatalf("candidates=%+v err=%v", candidates, candidateErr)
	}
	// completedStatus 是发送前订单状态变化补丁。
	completedStatus := "completed"
	if // patchErr 是把订单变更为完成状态的错误。
	patchErr := store.Orders.Patch(ctx, "receipt-db-order", OrderPatch{OrderStatus: &completedStatus}); patchErr != nil {
		t.Fatal(patchErr)
	}
	if // readErr 是完成订单再次读取候选时的预期未找到错误。
	_, readErr := store.AccountTasks.GetReceiptReminderCandidate(ctx, accountID, "receipt-db-order"); !errors.Is(readErr, ErrNotFound) {
		t.Fatalf("完成订单应退出候选 readErr=%v", readErr)
	}
}
