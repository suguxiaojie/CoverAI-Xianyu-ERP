package adapter

import (
	"context"
	"testing"

	domainchat "xianyu-go/internal/chat"
	"xianyu-go/internal/db"
	"xianyu-go/internal/engine"
)

// TestHandleChatMessagePersistsSelfEchoAsOutgoing 验证其他客户端发送的己方回显实时落库且保持幂等。
func TestHandleChatMessagePersistsSelfEchoAsOutgoing(t *testing.T) {
	// store、cleanup 保存隔离适配器存储及关闭函数。
	store, cleanup := newAdapterTestStore(t)
	defer cleanup()
	// runtimeAdapter 是仅注入聊天旁路的运行时适配器，不会执行自动回复或外部调用。
	runtimeAdapter := New(store, nil, nil)
	runtimeAdapter.SetChatService(domainchat.New(store))
	// session 保存回显到达前已经解析出的真实买家身份，空回显字段不得覆盖它。
	session := db.ChatSession{CookieID: "cid", ChatID: "chat-cross", BuyerID: "buyer-1", BuyerName: "追风kk"}
	// err 表示预置会话失败。
	if err := store.Chats.UpsertSession(context.Background(), session); err != nil {
		t.Fatalf("预置聊天会话: %v", err)
	}
	// raw 模拟闲鱼对其他官方客户端发送消息的己方紧凑回显。
	raw := map[string]any{"1": map[string]any{"2": "chat-cross@goofish", "3": "cross-1.PNM", "5": float64(1000), "10": map[string]any{"reminderContent": "跨平台消息", "senderUserId": "1"}}}
	// message 是 Engine 已标记为己方、只允许持久化的消息。
	message := engine.ChatMessage{AccountID: "cid", CookieStr: "unb=1;", ChatID: "chat-cross", SenderUserID: "1", SenderName: "我", Text: "跨平台消息", MessageID: "cross-1.PNM", Raw: raw, IsSelf: true}
	// err 表示首次持久化跨平台消息的错误。
	if err := runtimeAdapter.HandleChatMessage(context.Background(), message); err != nil {
		t.Fatalf("首次保存己方回显: %v", err)
	}
	// err 表示重复回显的幂等保存错误。
	if err := runtimeAdapter.HandleChatMessage(context.Background(), message); err != nil {
		t.Fatalf("重复保存己方回显: %v", err)
	}
	// saved、readErr 保存最终唯一消息及读取错误。
	saved, readErr := store.Chats.GetMessageByPlatformID(context.Background(), "cid", "cross-1.PNM")
	if readErr != nil || saved.Direction != "outgoing" || saved.Status != "sent" || saved.Content != "跨平台消息" {
		t.Fatalf("己方回显状态错误 message=%+v err=%v", saved, readErr)
	}
	// buyerID、buyerName 保存回显写入后的会话身份，必须保留既有真实买家信息。
	var buyerID, buyerName string
	// err 表示读取会话身份失败。
	if err := store.DB.QueryRowContext(context.Background(), `SELECT buyer_id,buyer_name FROM chat_sessions WHERE cookie_id=? AND chat_id=?`, "cid", "chat-cross").Scan(&buyerID, &buyerName); err != nil || buyerID != "buyer-1" || buyerName != "追风kk" {
		t.Fatalf("己方回显覆盖买家身份 buyer_id=%q buyer_name=%q err=%v", buyerID, buyerName, err)
	}
	// err 表示一个先于目标消息到达的未知已读回执；它不得误标当前会话的另一条出站消息。
	if err := runtimeAdapter.HandleMessageRead(context.Background(), engine.MessageReadEvent{AccountID: "cid", ChatID: "chat-cross", MessageID: "missing.PNM", ReadAt: 2000}); err != nil {
		t.Fatalf("缺失目标已读回执应等待历史收敛: %v", err)
	}
	// unchanged、readErr 保存未知回执处理后的原消息，必须继续保持未读。
	unchanged, readErr := store.Chats.GetMessageByPlatformID(context.Background(), "cid", "cross-1.PNM")
	if readErr != nil || unchanged.ReadStatus != 0 {
		t.Fatalf("未知回执误标其他消息 message=%+v err=%v", unchanged, readErr)
	}
	// count 保存该 PNM 对应的实际行数，重复回显不得产生双气泡。
	var count int
	// err 表示统计幂等结果失败。
	if err := store.DB.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM chat_messages WHERE cookie_id=? AND platform_message_id=?`, "cid", "cross-1.PNM").Scan(&count); err != nil || count != 1 {
		t.Fatalf("己方回显未保持幂等 count=%d err=%v", count, err)
	}
}

// TestHandleOutgoingChatImagePersistsPlatformMessage 验证 Engine 图片成功事件通过 Adapter 写入可实时广播的 PNM 幂等消息。
func TestHandleOutgoingChatImagePersistsPlatformMessage(t *testing.T) {
	// store、cleanup 分别是隔离适配器数据库和资源释放函数。
	store, cleanup := newAdapterTestStore(t)
	defer cleanup()
	// runtimeAdapter 是只启用聊天旁路、不执行平台发送的运行时适配器。
	runtimeAdapter := New(store, nil, nil)
	runtimeAdapter.SetChatService(domainchat.New(store))
	// event 是 Engine 在平台图片成功后产生的非敏感出站观察事件。
	event := engine.OutgoingChatMessage{AccountID: "cid", ChatID: "chat-image", BuyerID: "buyer-1",
		Text: "https://cdn.example/card.png", MessageType: "image", PlatformMessageID: "image-adapter.PNM", SentAt: 2000}
	// handleErr 是 Adapter 将图片观察事件持久化到 Chat 服务的错误。
	handleErr := runtimeAdapter.HandleOutgoingChatMessage(context.Background(), event)
	if handleErr != nil {
		t.Fatalf("保存图片出站旁路失败: %v", handleErr)
	}
	// saved、readErr 分别是按平台 PNM 读取的唯一图片消息和数据库错误。
	saved, readErr := store.Chats.GetMessageByPlatformID(context.Background(), "cid", "image-adapter.PNM")
	if readErr != nil || saved.ChatID != "chat-image" || saved.Direction != "outgoing" || saved.MessageType != "image" || saved.Content != event.Text || saved.Status != "sent" {
		t.Fatalf("图片出站旁路字段错误 message=%+v err=%v", saved, readErr)
	}
}
