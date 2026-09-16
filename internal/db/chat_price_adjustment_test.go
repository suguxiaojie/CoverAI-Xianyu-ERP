package db

import (
	"context"
	"errors"
	"testing"
)

// TestGetOwnedPendingPriceAdjustmentRequiresOwnershipAndNoLaterTerminalEvent 验证改价上下文来自归属卡片且付款后立即失效。
func TestGetOwnedPendingPriceAdjustmentRequiresOwnershipAndNoLaterTerminalEvent(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部数据库调用共用的上下文。
	ctx := context.Background()
	// userID 是测试卖家账号归属的本地用户主键。
	var userID int64
	if // userErr 是创建测试归属用户及读取主键的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "price-owner", "price-owner@example.com", "test-hash").Scan(&userID); userErr != nil {
		t.Fatal(userErr)
	}
	// accountID 是不包含凭证的测试卖家账号标识。
	const accountID = "price-account"
	if // accountErr 是创建测试卖家账号的错误。
	accountErr := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// session 是待付款卡片所属会话。
	session := ChatSession{CookieID: accountID, ChatID: "price-chat", BuyerID: "buyer-1"}
	// pending 是带明确改价动作的待付款卡片。
	pending := ChatMessage{MessageKey: "pending-price.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已拍下，待付款", Status: "received", SentAt: 1000,
		SystemCardKind: "trade", SystemCardEvent: "order_pending_payment", SystemCardOrderID: "5127694777172175924", SystemCardItemID: "item-1", SystemCardAction: "adjust_price"}
	if // inserted、saveErr 表示待付款卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, pending, false); saveErr != nil || !inserted {
		t.Fatalf("inserted=%v err=%v", inserted, saveErr)
	}
	// adjustment、readErr 是归属用户在终态到达前读取的改价上下文。
	adjustment, readErr := store.Chats.GetOwnedPendingPriceAdjustment(ctx, userID, accountID, "5127694777172175924")
	if readErr != nil || adjustment.ChatID != "price-chat" || adjustment.ItemID != "item-1" {
		t.Fatalf("adjustment=%+v err=%v", adjustment, readErr)
	}
	if // _, forbiddenErr 是其他用户读取当前账号卡片的拒绝结果。
	_, forbiddenErr := store.Chats.GetOwnedPendingPriceAdjustment(ctx, userID+1, accountID, "5127694777172175924"); !errors.Is(forbiddenErr, ErrNotFound) {
		t.Fatalf("foreign owner err=%v", forbiddenErr)
	}
	// paid 是同一订单后续明确付款卡片，写入后旧待付款入口必须失效。
	paid := ChatMessage{MessageKey: "paid-price.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: 2000,
		SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardOrderID: "5127694777172175924", SystemCardItemID: "item-1"}
	if // inserted、saveErr 表示付款终态卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, paid, false); saveErr != nil || !inserted {
		t.Fatalf("inserted=%v err=%v", inserted, saveErr)
	}
	if // _, terminalErr 是付款后再次读取旧改价入口的结果。
	_, terminalErr := store.Chats.GetOwnedPendingPriceAdjustment(ctx, userID, accountID, "5127694777172175924"); !errors.Is(terminalErr, ErrNotFound) {
		t.Fatalf("terminal err=%v", terminalErr)
	}
}
