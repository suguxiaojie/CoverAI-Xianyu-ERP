package db

import (
	"context"
	"testing"
)

// TestSaveMessageUpgradesLocationPlaceholderAndUsesSummary 验证旧 `[位置]` 行可被结构化历史补全，联系人栏不显示 JSON。
func TestSaveMessageUpgradesLocationPlaceholderAndUsesSummary(t *testing.T) {
	// store、cleanup 是隔离 SQLite 仓储及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部消息写入共用的生命周期。
	ctx := context.Background()
	// ownerID 是测试闲鱼账号所属的本地用户主键。
	var ownerID int64
	// ownerErr 是创建本地测试用户并取得主键时的错误。
	if ownerErr := store.DB.QueryRowContext(ctx, `INSERT INTO users(username,email,password_hash) VALUES(?,?,?) RETURNING id`, "location-owner", "location@example.com", "hash").Scan(&ownerID); ownerErr != nil {
		t.Fatal(ownerErr)
	}
	// accountErr 是创建归属明确的测试闲鱼账号时的错误。
	if accountErr := store.Cookies.CreateOwned(ctx, "location-account", "cookie", ownerID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// session 是位置消息所属的账号和单聊会话。
	session := ChatSession{CookieID: "location-account", ChatID: "location-chat", BuyerID: "buyer-1"}
	// placeholder 是旧解析器已经保存的降级文本消息。
	placeholder := ChatMessage{MessageKey: "location.PNM", PlatformMessageID: "location.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "text", Content: "[位置]", Status: "received", SentAt: 1000}
	// inserted、saveErr 是旧占位消息首次写入标记和持久化错误。
	if _, inserted, saveErr := store.Chats.SaveMessage(ctx, session, placeholder, false); saveErr != nil || !inserted {
		t.Fatalf("placeholder inserted=%v err=%v", inserted, saveErr)
	}
	// locationJSON 是后续平台历史补入的稳定位置卡片正文。
	const locationJSON = `{"title":"CoverAI 实体店","description":"东门","latitude":22.5,"longitude":113.9}`
	// structured 是同一 PNM 的权威 contentType=30 结果，并携带联系人短摘要。
	structured := ChatMessage{MessageKey: "location.PNM", PlatformMessageID: "location.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "location", Content: locationJSON, Summary: "[位置] CoverAI 实体店", PlatformContentType: 30, Status: "received", SentAt: 1000}
	// merged、inserted、mergeErr 是旧行升级后的消息、插入标记和合并错误。
	merged, inserted, mergeErr := store.Chats.SaveMessage(ctx, session, structured, false)
	if mergeErr != nil || inserted || merged.MessageType != "location" || merged.Content != locationJSON || merged.PlatformContentType != 30 {
		t.Fatalf("merged=%+v inserted=%v err=%v", merged, inserted, mergeErr)
	}
	// newer 是验证瞬时 Summary 不会把结构化 JSON 写进联系人栏的新位置消息。
	newer := ChatMessage{MessageKey: "location-2.PNM", PlatformMessageID: "location-2.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "location", Content: locationJSON, Summary: "[位置] CoverAI 实体店", PlatformContentType: 30, Status: "received", SentAt: 2000}
	// saveErr 是新位置卡片摘要写入测试仓储时的错误。
	if _, _, saveErr := store.Chats.SaveMessage(ctx, session, newer, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// summary 是数据库实际保存的联系人短摘要。
	var summary string
	// queryErr 是读取联系人栏最终摘要时的数据库错误。
	if queryErr := store.DB.QueryRowContext(ctx, `SELECT last_message FROM chat_sessions WHERE cookie_id=? AND chat_id=?`, session.CookieID, session.ChatID).Scan(&summary); queryErr != nil || summary != "[位置] CoverAI 实体店" {
		t.Fatalf("summary=%q err=%v", summary, queryErr)
	}
}
