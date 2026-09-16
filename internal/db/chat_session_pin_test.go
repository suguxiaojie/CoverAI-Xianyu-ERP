package db

import (
	"context"
	"errors"
	"testing"
)

// TestSetSessionPinnedPreservesManualOrderAndOwnership 验证置顶时间、普通消息顺序、幂等调用与用户归属边界。
func TestSetSessionPinnedPreservesManualOrderAndOwnership(t *testing.T) {
	// store、cleanup 是已迁移到最新 Schema 的隔离 SQLite 仓储及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部数据库操作共用的调用上下文。
	ctx := context.Background()
	// ownerID 是当前允许修改会话偏好的 ERP 用户主键。
	var ownerID int64
	if // userErr 是创建归属用户和读取主键的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users(username,email,password_hash) VALUES(?,?,?) RETURNING id`, "pin-owner", "pin-owner@example.com", "test-hash").Scan(&ownerID); userErr != nil {
		t.Fatal(userErr)
	}
	// accountID 是三条测试会话共用且不包凭证的账号标识。
	const accountID = "pin-account"
	if // accountErr 是创建归属账号的错误。
	accountErr := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", ownerID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// sessions 按原最近消息从旧到新保存三条会话夹具。
	sessions := []ChatSession{
		{CookieID: accountID, ChatID: "old", BuyerID: "buyer-old", LastMessage: "old", LastMessageAt: 100},
		{CookieID: accountID, ChatID: "middle", BuyerID: "buyer-middle", LastMessage: "middle", LastMessageAt: 200},
		{CookieID: accountID, ChatID: "newest", BuyerID: "buyer-newest", LastMessage: "newest", LastMessageAt: 300},
	}
	// session 是当前写入隔离数据库的会话夹具。
	for _, session := range sessions {
		if // sessionErr 是创建当前会话的错误。
		sessionErr := store.Chats.UpsertSession(ctx, session); sessionErr != nil {
			t.Fatal(sessionErr)
		}
	}
	if // oldPinErr 是置顶最旧会话并记录第一个手工时间的错误。
	oldPinErr := store.Chats.SetSessionPinned(ctx, ownerID, accountID, "old", true, 1000); oldPinErr != nil {
		t.Fatal(oldPinErr)
	}
	if // middlePinErr 是后置顶中间会话的错误，该会话应成为置顶组第一。
	middlePinErr := store.Chats.SetSessionPinned(ctx, ownerID, accountID, "middle", true, 2000); middlePinErr != nil {
		t.Fatal(middlePinErr)
	}
	if // repeatedPinErr 是重复置顶同一会话的幂等结果，不得刷新原置顶时间。
	repeatedPinErr := store.Chats.SetSessionPinned(ctx, ownerID, accountID, "middle", true, 3000); repeatedPinErr != nil {
		t.Fatal(repeatedPinErr)
	}
	// persistedPinnedAt 是重复置顶后中间会话仍应保留的原手工时间。
	var persistedPinnedAt int64
	if // pinnedAtErr 是读取重复置顶后持久时间的错误。
	pinnedAtErr := store.DB.QueryRowContext(ctx, `SELECT pinned_at FROM chat_sessions WHERE cookie_id=? AND chat_id=?`, accountID, "middle").Scan(&persistedPinnedAt); pinnedAtErr != nil || persistedPinnedAt != 2000 {
		t.Fatalf("pinned_at=%d err=%v", persistedPinnedAt, pinnedAtErr)
	}
	// listed 是同时包含手工置顶和普通活跃度排序的会话列表。
	listed, listErr := store.Chats.ListSessions(ctx, ownerID, accountID, 20)
	if listErr != nil || len(listed) != 3 || listed[0].ChatID != "middle" || listed[1].ChatID != "old" || listed[2].ChatID != "newest" || !listed[0].IsPinned {
		t.Fatalf("listed=%+v err=%v", listed, listErr)
	}
	if // unpinErr 是取消中间会话置顶的错误。
	unpinErr := store.Chats.SetSessionPinned(ctx, ownerID, accountID, "middle", false, 0); unpinErr != nil {
		t.Fatal(unpinErr)
	}
	// afterUnpin 是取消后应保留 old 置顶，并让普通会话按最近消息排序的列表。
	afterUnpin, afterUnpinErr := store.Chats.ListSessions(ctx, ownerID, accountID, 20)
	if afterUnpinErr != nil || afterUnpin[0].ChatID != "old" || afterUnpin[1].ChatID != "newest" || afterUnpin[2].ChatID != "middle" || afterUnpin[2].PinnedAt != 0 {
		t.Fatalf("after unpin=%+v err=%v", afterUnpin, afterUnpinErr)
	}
	if // forbiddenErr 是其他 ERP 用户尝试修改该账号会话时的拒绝结果。
	forbiddenErr := store.Chats.SetSessionPinned(ctx, ownerID+1, accountID, "old", false, 0); !errors.Is(forbiddenErr, ErrNotFound) {
		t.Fatalf("forbidden err=%v", forbiddenErr)
	}
	if // missingErr 是归属账号下不存在会话的结果。
	missingErr := store.Chats.SetSessionPinned(ctx, ownerID, accountID, "missing", true, 4000); !errors.Is(missingErr, ErrNotFound) {
		t.Fatalf("missing err=%v", missingErr)
	}
}
