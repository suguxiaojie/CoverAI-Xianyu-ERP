package db

import (
	"context"
	"testing"
)

// TestSaveMessagePersistsAndMergesReplyTarget 验证原生引用 PNM ID 在首次写入和平台历史单调合并时保持。
func TestSaveMessagePersistsAndMergesReplyTarget(t *testing.T) {
	// store、cleanup 是带 schema 41 的隔离 SQLite 存储和关闭函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部数据库调用的生命周期。
	ctx := context.Background()
	// userID 是测试账号所属 ERP 用户的本地主键。
	var userID int64
	if // userErr 是创建引用消息测试用户的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "reply-owner", "reply-owner@example.com", "test-hash").Scan(&userID); userErr != nil {
		t.Fatal(userErr)
	}
	if // accountErr 是创建不含真实凭证的测试账号错误。
	accountErr := store.Cookies.CreateOwned(ctx, "reply-account", "test-cookie", userID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// session 是引用消息所属的隔离聊天会话。
	session := ChatSession{CookieID: "reply-account", ChatID: "reply-chat", BuyerID: "buyer-1"}
	// local 是平台响应前已保存引用目标的本地待发送消息。
	local := ChatMessage{MessageKey: "local-reply", Direction: "outgoing", MessageType: "text", Content: "回复", Status: "sending", SentAt: 1000, ReplyToPlatformMessageID: "target-1.PNM"}
	// stored、inserted、saveErr 是首次写入的消息、插入标记和错误。
	stored, inserted, saveErr := store.Chats.SaveMessage(ctx, session, local, false)
	if saveErr != nil || !inserted || stored.ReplyToPlatformMessageID != "target-1.PNM" {
		t.Fatalf("stored=%+v inserted=%v err=%v", stored, inserted, saveErr)
	}
	// history 是随后到达且携带同一引用目标的平台历史状态。
	history := ChatMessage{MessageKey: "local-reply", PlatformMessageID: "reply-result.PNM", Direction: "outgoing", MessageType: "text", Content: "回复", Status: "sent", SentAt: 1000, ReplyToPlatformMessageID: "target-1.PNM"}
	// merged、historyInserted、historyErr 是单调合并后的消息、插入标记和错误。
	merged, historyInserted, historyErr := store.Chats.SaveMessage(ctx, session, history, false)
	if historyErr != nil || historyInserted || merged.ReplyToPlatformMessageID != "target-1.PNM" || merged.PlatformMessageID != "reply-result.PNM" {
		t.Fatalf("merged=%+v inserted=%v err=%v", merged, historyInserted, historyErr)
	}
}

// TestListMessagesResolvesSameConversationReplyPreview 验证历史列表使用同账号同会话 PNM 自连接返回最小引用快照。
func TestListMessagesResolvesSameConversationReplyPreview(t *testing.T) {
	// store、cleanup 是带 schema 41 的隔离 SQLite 存储和关闭函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试数据库调用共用的生命周期。
	ctx := context.Background()
	// userID 是引用消息列表归属校验使用的 ERP 用户主键。
	var userID int64
	if // userErr 是创建测试用户的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "reply-preview-owner", "reply-preview@example.com", "test-hash").Scan(&userID); userErr != nil {
		t.Fatal(userErr)
	}
	if // accountErr 是创建不含真实凭证的测试账号错误。
	accountErr := store.Cookies.CreateOwned(ctx, "reply-preview-account", "test-cookie", userID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// session 是原消息和引用消息共享的精确会话。
	session := ChatSession{CookieID: "reply-preview-account", ChatID: "reply-preview-chat", BuyerID: "buyer-1", BuyerName: "买家"}
	// target 是被引用的己方文本消息。
	target := ChatMessage{MessageKey: "target-local", PlatformMessageID: "target-preview.PNM", Direction: "outgoing", SenderID: "reply-preview-account", SenderName: "我", MessageType: "text", Content: "原消息", Status: "sent", SentAt: 1000}
	if // saveErr 是写入被引用目标的错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, target, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// reply 是通过 PNM 指向原消息的后续文本。
	reply := ChatMessage{MessageKey: "reply-local", PlatformMessageID: "reply-preview.PNM", ReplyToPlatformMessageID: "target-preview.PNM", Direction: "outgoing", SenderID: "reply-preview-account", SenderName: "我", MessageType: "text", Content: "新消息", Status: "sent", SentAt: 2000}
	if // saveErr 是写入引用消息的错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, reply, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// messages、listErr 是当前用户可访问的会话历史和查询错误。
	messages, listErr := store.Chats.ListMessages(ctx, userID, session.CookieID, session.ChatID, 0, 20)
	if listErr != nil || len(messages) != 2 {
		t.Fatalf("messages=%+v err=%v", messages, listErr)
	}
	// preview 是引用消息行上通过同会话自连接得到的目标快照。
	preview := messages[1].ReplyPreview
	if preview == nil || preview.PlatformMessageID != "target-preview.PNM" || preview.Content != "原消息" || preview.SenderName != "我" {
		t.Fatalf("preview=%+v", preview)
	}
}
