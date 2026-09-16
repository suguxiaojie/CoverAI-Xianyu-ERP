package chat

import (
	"context"
	"encoding/base64"
	"testing"

	"xianyu-go/internal/db"
)

// TestRecordHistoryPageStructuresLocationCard 验证历史 contentType=30 保存稳定 JSON，并让联系人栏显示短摘要。
func TestRecordHistoryPageStructuresLocationCard(t *testing.T) {
	// store、cleanup 是隔离数据库及测试结束清理函数。
	store, cleanup := chatTestStore(t)
	defer cleanup()
	// service 是复用真实聊天仓储的位置卡片解析服务。
	service := New(store)
	// raw 是平台历史 custom.data 解码后的位置卡片正文。
	raw := `{"contentType":30,"locationCard":{"title":"CoverAI 实体店","content":"东门电梯上楼右转","latitude":"22.540503","longitude":"113.934528"}}`
	// body 是只包含一条买家位置卡片的历史分页响应。
	body := map[string]any{"userMessageModels": []any{map[string]any{"message": map[string]any{
		"messageId": "location-history.PNM", "createAt": float64(1000),
		"extension": map[string]any{"senderUserId": "peer", "reminderTitle": "买家"},
		"content":   map[string]any{"custom": map[string]any{"data": base64.StdEncoding.EncodeToString([]byte(raw)), "summary": "[位置]"}},
	}}}}
	// session 是目标账号、会话和买家的非敏感摘要。
	session := db.ChatSession{CookieID: "account-1", ChatID: "chat-location", BuyerID: "peer", BuyerName: "买家"}
	// page、recordErr 是历史解析结果及持久化错误。
	page, recordErr := service.RecordHistoryPage(context.Background(), "account-1", "chat-location", "self", session, body)
	if recordErr != nil || len(page.Messages) != 1 {
		t.Fatalf("page=%+v err=%v", page, recordErr)
	}
	// message 是解析后的结构化位置消息。
	message := page.Messages[0]
	if message.MessageType != "location" || message.PlatformContentType != 30 || message.Content != `{"title":"CoverAI 实体店","description":"东门电梯上楼右转","latitude":22.540503,"longitude":113.934528}` {
		t.Fatalf("message=%+v", message)
	}
	// owner、ownerErr 是测试账号所有者及读取错误。
	owner, ownerErr := store.Users.GetByUsername(context.Background(), "owner")
	if ownerErr != nil {
		t.Fatal(ownerErr)
	}
	// sessions、listErr 是联系人栏会话摘要及读取错误。
	sessions, listErr := store.Chats.ListSessions(context.Background(), owner.ID, "account-1", 10)
	if listErr != nil || len(sessions) != 1 || sessions[0].LastMessage != "[位置] CoverAI 实体店" {
		t.Fatalf("sessions=%+v err=%v", sessions, listErr)
	}
}
