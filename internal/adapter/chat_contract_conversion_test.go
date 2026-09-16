package adapter

import (
	"reflect"
	"testing"

	chatapp "xianyu-go/internal/application/chat"
	"xianyu-go/internal/db"
)

// TestChatSessionFromApplicationKeepsNonSensitiveFields 验证会话转换只复制非敏感展示字段。
func TestChatSessionFromApplicationKeepsNonSensitiveFields(t *testing.T) {
	// session 保存应用层聊天会话摘要。
	session := chatapp.Session{AccountID: "account-1", ChatID: "chat-1", BuyerID: "buyer-1", BuyerName: "买家", BuyerAvatar: "avatar", ItemID: "item-1", ItemTitle: "商品", LastMessage: "你好", LastMessageAt: 42, UnreadCount: 3}
	// converted 保存转换后的 legacy 聊天会话模型。
	converted := ChatSessionFromApplication(session)
	// expected 保存应由领域仓储接收的非敏感会话字段。
	expected := db.ChatSession{CookieID: "account-1", ChatID: "chat-1", BuyerID: "buyer-1", BuyerName: "买家", BuyerAvatar: "avatar", ItemID: "item-1", ItemTitle: "商品", LastMessage: "你好", LastMessageAt: 42, UnreadCount: 3}
	if !reflect.DeepEqual(converted, expected) {
		t.Fatalf("聊天会话转换异常: got=%+v want=%+v", converted, expected)
	}
}

// TestChatMessagesFromDBKeepsMessageContract 验证数据库消息转换为应用消息时完整保留 API 所需字段。
func TestChatMessagesFromDBKeepsMessageContract(t *testing.T) {
	// messages 保存 legacy 聊天仓储返回的消息模型。
	messages := []db.ChatMessage{{ID: 7, CookieID: "account-1", ChatID: "chat-1", MessageKey: "key-1", PlatformMessageID: "key-1.PNM", ReplyToPlatformMessageID: "target-1.PNM", Direction: "incoming", SenderID: "buyer-1", SenderName: "买家", MessageType: "system", Content: "我已拍下，待付款", Status: "received", ReadStatus: 2, ReadAt: 88, SentAt: 99,
		ReplyPreview:        &db.ChatReplyPreview{PlatformMessageID: "target-1.PNM", Direction: "outgoing", SenderID: "account-1", SenderName: "我", MessageType: "text", Content: "原消息", Status: "sent"},
		PlatformContentType: 26, SystemCardKind: "trade", SystemCardEvent: "order_pending_payment", SystemCardTitle: "我已拍下，待付款", SystemCardDescription: "请双方沟通及时确认价格", SystemCardOrderID: "order-1", SystemCardItemID: "item-1", SystemCardAction: "adjust_price"}}
	// converted 保存不再暴露数据库模型的应用层消息。
	converted := ChatMessagesFromDB(messages)
	if len(converted) != 1 {
		t.Fatalf("消息数量异常: got=%d", len(converted))
	}
	// expected 保存应用层消息字段。
	expected := chatapp.Message{ID: 7, AccountID: "account-1", ChatID: "chat-1", MessageKey: "key-1", PlatformMessageID: "key-1.PNM", ReplyToPlatformMessageID: "target-1.PNM", Direction: "incoming", SenderID: "buyer-1", SenderName: "买家", MessageType: "system", Content: "我已拍下，待付款", Status: "received", ReadStatus: 2, ReadAt: 88, SentAt: 99,
		ReplyPreview: &chatapp.ReplyPreview{PlatformMessageID: "target-1.PNM", Direction: "outgoing", SenderID: "account-1", SenderName: "我", MessageType: "text", Content: "原消息", Status: "sent"},
		SystemCard:   &chatapp.SystemCard{Kind: "trade", Event: "order_pending_payment", Title: "我已拍下，待付款", Description: "请双方沟通及时确认价格", OrderID: "order-1", ItemID: "item-1", Action: "adjust_price"}}
	if !reflect.DeepEqual(converted[0], expected) {
		t.Fatalf("聊天消息转换异常: got=%+v want=%+v", converted[0], expected)
	}
}

// TestChatMessagesFromDBHandlesEmptyInput 验证空消息页转换后保持可安全遍历的空切片。
func TestChatMessagesFromDBHandlesEmptyInput(t *testing.T) {
	// converted 保存空 legacy 消息列表转换结果。
	converted := ChatMessagesFromDB(nil)
	if converted == nil || len(converted) != 0 {
		t.Fatalf("空消息转换应返回非 nil 空切片: %#v", converted)
	}
}

// TestChatMessagesFromDBBackfillsHistoricalReceiveFlowerAction 验证历史送花卡片无需重新抓取即可出现官方收花入口。
func TestChatMessagesFromDBBackfillsHistoricalReceiveFlowerAction(t *testing.T) {
	// messages 是动作字段尚为空但已有明确事件和订单号的历史数据库记录。
	messages := []db.ChatMessage{{
		MessageKey: "flower-sent", MessageType: "system", SystemCardKind: "trade",
		SystemCardEvent: "red_flower_sent", SystemCardTitle: "你人真不错，送你闲鱼小红花",
		SystemCardOrderID: "5127372398162002704",
	}}
	// converted 是应用层安全补齐动作后的消息。
	converted := ChatMessagesFromDB(messages)
	if len(converted) != 1 || converted[0].SystemCard == nil || converted[0].SystemCard.Action != "receive_red_flower" {
		t.Fatalf("converted=%+v", converted)
	}
}

// TestChatMessagesFromDBBackfillsHistoricalSellerShipmentAction 验证历史卖家付款卡片按发送者和账号差异安全补齐发货入口。
func TestChatMessagesFromDBBackfillsHistoricalSellerShipmentAction(t *testing.T) {
	// sellerMessage 是已有精确订单和卖家视角、但动作字段尚为空的历史记录。
	sellerMessage := db.ChatMessage{CookieID: "seller-account", SenderID: "buyer-account", MessageKey: "paid-seller", MessageType: "system", SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardTitle: "我已付款，等待你发货", SystemCardOrderID: "5127372002248048713"}
	// buyerMessage 复用相同事件但发送者就是当前账号，不得补齐卖家动作。
	buyerMessage := sellerMessage
	buyerMessage.CookieID, buyerMessage.SenderID, buyerMessage.MessageKey = "buyer-account", "buyer-account", "paid-buyer"
	// converted 是两个视角的应用层转换结果。
	converted := ChatMessagesFromDB([]db.ChatMessage{sellerMessage, buyerMessage})
	if len(converted) != 2 || converted[0].SystemCard == nil || converted[0].SystemCard.Action != "ship_order" || converted[1].SystemCard == nil || converted[1].SystemCard.Action != "" {
		t.Fatalf("converted=%+v", converted)
	}
}
