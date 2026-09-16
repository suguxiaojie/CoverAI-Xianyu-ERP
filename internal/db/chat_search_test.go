package db

import (
	"context"
	"testing"
)

// TestSearchSessionsMatchesHistoricalMessagesAndKeepsAccountBoundary 验证历史消息可定位会话且不会跨账号或命中撤回内容。
func TestSearchSessionsMatchesHistoricalMessagesAndKeepsAccountBoundary(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是当前搜索测试全部数据库操作共用的上下文。
	ctx := context.Background()
	// ownerID、foreignOwnerID 是两个互不授权的本地用户主键。
	var ownerID, foreignOwnerID int64
	if // ownerErr 是创建当前用户和读取主键的错误。
	ownerErr := store.DB.QueryRowContext(ctx, `INSERT INTO users(username,email,password_hash) VALUES(?,?,?) RETURNING id`, "search-owner", "search-owner@example.com", "hash").Scan(&ownerID); ownerErr != nil {
		t.Fatal(ownerErr)
	}
	if // foreignOwnerErr 是创建其他用户和读取主键的错误。
	foreignOwnerErr := store.DB.QueryRowContext(ctx, `INSERT INTO users(username,email,password_hash) VALUES(?,?,?) RETURNING id`, "search-foreign", "search-foreign@example.com", "hash").Scan(&foreignOwnerID); foreignOwnerErr != nil {
		t.Fatal(foreignOwnerErr)
	}
	// accountID、foreignAccountID 是当前用户和其他用户的闲鱼账号标识。
	const accountID, foreignAccountID = "search-account", "search-foreign-account"
	if // accountErr 是创建当前用户闲鱼账号的错误。
	accountErr := store.Cookies.CreateOwned(ctx, accountID, "cookie", ownerID); accountErr != nil {
		t.Fatal(accountErr)
	}
	if // foreignAccountErr 是创建其他用户闲鱼账号的错误。
	foreignAccountErr := store.Cookies.CreateOwned(ctx, foreignAccountID, "cookie", foreignOwnerID); foreignAccountErr != nil {
		t.Fatal(foreignAccountErr)
	}
	// session 是历史卡密所在会话；最新摘要之后会被另一条消息覆盖。
	session := ChatSession{CookieID: accountID, ChatID: "chat-history", BuyerID: "buyer-1", BuyerName: "测试买家", ItemTitle: "历史商品"}
	// historical 是需要通过完整聊天历史命中的旧出站卡密。
	historical := ChatMessage{MessageKey: "history-token.PNM", Direction: "outgoing", SenderID: accountID, MessageType: "text", Content: "TEST-CARD-CODE-001", Status: "sent", SentAt: 1000}
	if // _, _, saveErr 是保存历史卡密的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, historical, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// latest 是覆盖会话摘要但不应让历史卡密失去可搜索性的最新消息。
	latest := ChatMessage{MessageKey: "latest-text.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "text", Content: "地址在这里", Status: "received", SentAt: 2000}
	if // _, _, saveErr 是保存最新摘要消息的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, latest, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// recalled 是已经撤回且不应继续用于会话搜索的内容。
	recalled := ChatMessage{MessageKey: "recalled-secret.PNM", Direction: "outgoing", SenderID: accountID, MessageType: "text", Content: "RECALLED-SECRET", Status: "recalled", SentAt: 1500}
	if // _, _, saveErr 是保存撤回内容的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, recalled, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// wildcardSession 保存包含 LIKE 特殊字符的普通文本，用于验证按字面量搜索。
	wildcardSession := ChatSession{CookieID: accountID, ChatID: "chat-wildcard", BuyerID: "buyer-2", BuyerName: "特殊字符"}
	// wildcardMessage 是包含百分号和下划线的普通历史消息。
	wildcardMessage := ChatMessage{MessageKey: "wildcard.PNM", Direction: "incoming", SenderID: "buyer-2", MessageType: "text", Content: "折扣 100%_DONE", Status: "received", SentAt: 3000}
	if // _, _, saveErr 是保存特殊字符内容的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, wildcardSession, wildcardMessage, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// foreignSession 在其他账号保存相同卡密，必须被账号归属隔离。
	foreignSession := ChatSession{CookieID: foreignAccountID, ChatID: "foreign-chat", BuyerID: "foreign-buyer"}
	// foreignMessage 是其他账号中与目标完全相同的历史卡密。
	foreignMessage := ChatMessage{MessageKey: "foreign-token.PNM", Direction: "incoming", SenderID: "foreign-buyer", MessageType: "text", Content: "TEST-CARD-CODE-001", Status: "received", SentAt: 4000}
	if // _, _, saveErr 是保存其他账号同内容的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, foreignSession, foreignMessage, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// matches、searchErr 是当前账号按历史卡密搜索得到的会话。
	matches, searchErr := store.Chats.SearchSessions(ctx, ownerID, accountID, "test-card-code-001", 20)
	if searchErr != nil || len(matches) != 1 || matches[0].ChatID != "chat-history" || matches[0].LastMessage != "地址在这里" {
		t.Fatalf("matches=%+v err=%v", matches, searchErr)
	}
	// foreignMatches、foreignErr 是其他用户尝试搜索当前账号的隔离结果。
	foreignMatches, foreignErr := store.Chats.SearchSessions(ctx, foreignOwnerID, accountID, "TEST-CARD-CODE-001", 20)
	if foreignErr != nil || len(foreignMatches) != 0 {
		t.Fatalf("foreign matches=%+v err=%v", foreignMatches, foreignErr)
	}
	// recalledMatches、recalledErr 是撤回内容搜索结果，必须为空。
	recalledMatches, recalledErr := store.Chats.SearchSessions(ctx, ownerID, accountID, "RECALLED-SECRET", 20)
	if recalledErr != nil || len(recalledMatches) != 0 {
		t.Fatalf("recalled matches=%+v err=%v", recalledMatches, recalledErr)
	}
	// wildcardMatches、wildcardErr 验证百分号和下划线按普通字符匹配。
	wildcardMatches, wildcardErr := store.Chats.SearchSessions(ctx, ownerID, accountID, "100%_DONE", 20)
	if wildcardErr != nil || len(wildcardMatches) != 1 || wildcardMatches[0].ChatID != "chat-wildcard" {
		t.Fatalf("wildcard matches=%+v err=%v", wildcardMatches, wildcardErr)
	}
}
