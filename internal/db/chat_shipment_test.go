package db

import (
	"context"
	"errors"
	"testing"
)

// TestGetOwnedPendingShipmentIgnoresConditionalReminder 验证旧误分类提醒不阻断发货，而明确发货仍是终态。
func TestGetOwnedPendingShipmentIgnoresConditionalReminder(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试所有 SQLite 操作共用的上下文。
	ctx := context.Background()
	// userID 是测试卖家账号所属本地用户。
	var userID int64
	if // userErr 是创建测试用户和读取主键的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "shipment-owner", "shipment-owner@example.com", "test-hash").Scan(&userID); userErr != nil {
		t.Fatal(userErr)
	}
	// accountID、orderID 是卖家账号和待发货订单标识。
	const accountID, orderID = "shipment-account", "3316800937226128994"
	if // accountErr 是创建归属卖家账号的错误。
	accountErr := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// session 是付款和提醒卡片所属卖家会话。
	session := ChatSession{CookieID: accountID, ChatID: "shipment-chat", BuyerID: "buyer-1"}
	// paid 是具备卖家发货动作的真实付款卡片。
	paid := ChatMessage{MessageKey: "paid-shipment.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: 1000,
		SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardTitle: "我已付款，等待你发货", SystemCardOrderID: orderID, SystemCardItemID: "item-1", SystemCardAction: "ship_order"}
	if // inserted、saveErr 表示付款卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, paid, false); saveErr != nil || !inserted {
		t.Fatalf("paid inserted=%v err=%v", inserted, saveErr)
	}
	// reminder 复现正式消息 8009：事件被旧版误存为 order_shipped，但语义仍是条件提醒。
	reminder := ChatMessage{MessageKey: "reminder-shipment.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "记得及时发货", Status: "received", SentAt: 2000,
		SystemCardKind: "trade", SystemCardEvent: "order_shipped", SystemCardTitle: "记得及时发货", SystemCardDescription: "如已发货，请点击「去发货」输入快递单号", SystemCardOrderID: orderID, SystemCardItemID: "item-1", SystemCardAction: "ship_order"}
	if // inserted、saveErr 表示历史错误提醒是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, reminder, false); saveErr != nil || !inserted {
		t.Fatalf("reminder inserted=%v err=%v", inserted, saveErr)
	}
	// shipment、shipmentErr 是忽略条件提醒后仍可读取的发货上下文。
	shipment, shipmentErr := store.Chats.GetOwnedPendingShipment(ctx, userID, accountID, orderID)
	if shipmentErr != nil || shipment == nil || shipment.ChatID != "shipment-chat" || shipment.BuyerID != "buyer-1" {
		t.Fatalf("shipment=%+v err=%v", shipment, shipmentErr)
	}
	// shipped 是同订单后续明确完成发货的正向终态对照。
	shipped := ChatMessage{MessageKey: "completed-shipment.PNM", Direction: "incoming", SenderID: accountID, MessageType: "system", Content: "你已发货", Status: "received", SentAt: 3000,
		SystemCardKind: "trade", SystemCardEvent: "order_shipped", SystemCardTitle: "你已发货", SystemCardOrderID: orderID, SystemCardItemID: "item-1"}
	if // inserted、saveErr 表示明确发货终态是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, shipped, false); saveErr != nil || !inserted {
		t.Fatalf("shipped inserted=%v err=%v", inserted, saveErr)
	}
	if // _, terminalErr 是明确发货后再次读取上下文必须得到的未找到错误。
	_, terminalErr := store.Chats.GetOwnedPendingShipment(ctx, userID, accountID, orderID); !errors.Is(terminalErr, ErrNotFound) {
		t.Fatalf("terminal err=%v", terminalErr)
	}
}
