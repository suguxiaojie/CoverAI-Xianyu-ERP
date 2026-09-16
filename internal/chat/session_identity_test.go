package chat

import (
	"context"
	"testing"

	"xianyu-go/internal/db"
)

// TestIncomingSessionIdentityRejectsSellerAndSystemSenders 验证卖家回显和系统卡片不得覆盖买家身份。
func TestIncomingSessionIdentityRejectsSellerAndSystemSenders(t *testing.T) {
	// realBuyer 是允许写入会话的普通买家入站消息。
	realBuyer := Incoming{AccountID: "seller-1", BuyerID: "buyer-1@goofish", BuyerName: "买家甲", Raw: map[string]any{"senderAvatar": "https://img.example/buyer.png"}}
	// buyerID、buyerName、buyerAvatar 是可信买家消息解析出的会话身份。
	buyerID, buyerName, buyerAvatar := incomingSessionIdentity(realBuyer, "text")
	if buyerID != "buyer-1" || buyerName != "买家甲" || buyerAvatar != "https://img.example/buyer.png" {
		t.Fatalf("买家身份未保留 id=%q name=%q avatar=%q", buyerID, buyerName, buyerAvatar)
	}
	// sellerEcho 模拟平台未标记 IsSelf 但 senderUserId 仍是卖家账号的回显。
	sellerEcho := Incoming{AccountID: "seller-1", BuyerID: "seller-1@goofish", BuyerName: "卖家名"}
	buyerID, buyerName, buyerAvatar = incomingSessionIdentity(sellerEcho, "text")
	if buyerID != "" || buyerName != "" || buyerAvatar != "" {
		t.Fatalf("卖家回显泄漏为买家身份 id=%q name=%q avatar=%q", buyerID, buyerName, buyerAvatar)
	}
	// systemCard 模拟携带买家发送者的交易卡片，消息行保留发送者但会话不采信。
	systemCard := Incoming{AccountID: "seller-1", BuyerID: "buyer-1", BuyerName: "交易消息"}
	buyerID, buyerName, buyerAvatar = incomingSessionIdentity(systemCard, "system")
	if buyerID != "" || buyerName != "" || buyerAvatar != "" {
		t.Fatalf("系统卡片泄漏为买家身份 id=%q name=%q avatar=%q", buyerID, buyerName, buyerAvatar)
	}
}

// TestRecordIncomingSellerEchoPreservesExistingBuyerIdentity 验证未正确标记的卖家回显也不会污染已有会话。
func TestRecordIncomingSellerEchoPreservesExistingBuyerIdentity(t *testing.T) {
	// store、cleanup 是隔离 SQLite 聊天存储及其释放函数。
	store, cleanup := chatTestStore(t)
	defer cleanup()
	// ctx 是会话预置、消息写入和结果查询的共享上下文。
	ctx := context.Background()
	// existing 是已由真实买家消息确认的权威会话身份。
	existing := db.ChatSession{CookieID: "account-1", ChatID: "chat-identity", BuyerID: "buyer-1", BuyerName: "买家甲", BuyerAvatar: "https://img.example/buyer.png"}
	// saveErr 是预置权威买家会话的存储错误。
	saveErr := store.Chats.UpsertSession(ctx, existing)
	if saveErr != nil {
		t.Fatalf("预置会话身份: %v", saveErr)
	}
	// service 是执行实时入站消息规范化的聊天领域服务。
	service := New(store)
	// sellerEcho 的 BuyerID 错误等于当前卖家账号，但消息仍允许作为历史行保存。
	sellerEcho := Incoming{AccountID: "account-1", ChatID: "chat-identity", BuyerID: "account-1", BuyerName: "卖家名", Text: "回显文本", MessageID: "seller-echo.PNM", Raw: map[string]any{"sendTime": int64(2000)}}
	// recorded、inserted、recordErr 是卖家回显的持久消息、首次插入标记和写入错误。
	recorded, inserted, recordErr := service.RecordIncoming(ctx, sellerEcho)
	if recordErr != nil || !inserted || recorded == nil {
		t.Fatalf("保存卖家回显 inserted=%v err=%v", inserted, recordErr)
	}
	// buyerID、buyerName、buyerAvatar 是回显写入后必须保持不变的会话身份。
	var buyerID, buyerName, buyerAvatar string
	// queryErr 是读取回显后会话身份的数据库错误。
	queryErr := store.DB.QueryRowContext(ctx, `SELECT buyer_id,buyer_name,buyer_avatar_url FROM chat_sessions WHERE cookie_id=? AND chat_id=?`, "account-1", "chat-identity").Scan(&buyerID, &buyerName, &buyerAvatar)
	if queryErr != nil {
		t.Fatalf("读取会话身份: %v", queryErr)
	}
	if buyerID != "buyer-1" || buyerName != "买家甲" || buyerAvatar != "https://img.example/buyer.png" {
		t.Fatalf("卖家回显覆盖买家身份 id=%q name=%q avatar=%q", buyerID, buyerName, buyerAvatar)
	}
}
