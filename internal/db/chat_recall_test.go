package db

import (
	"context"
	"testing"
)

// TestChatPlatformMessageBindingAndRecall 验证本地幂等键、平台 PNM ID 和撤回状态保持同一消息行。
func TestChatPlatformMessageBindingAndRecall(t *testing.T) {
	// store、cleanup 保存隔离 SQLite 数据库和关闭函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试数据库调用共用上下文。
	ctx := context.Background()
	// userID 保存测试账号归属用户主键。
	var userID int64
	// err 表示创建测试用户失败。
	if err := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "recall-owner", "recall-owner@example.com", "test-hash").Scan(&userID); err != nil {
		t.Fatalf("创建测试用户: %v", err)
	}
	// accountID 是不包含凭证的测试账号标识。
	const accountID = "recall-account"
	// err 表示创建测试账号失败。
	if err := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); err != nil {
		t.Fatalf("创建测试账号: %v", err)
	}
	// session 是撤回消息所属会话。
	session := ChatSession{CookieID: accountID, ChatID: "recall-chat", BuyerID: "buyer-1"}
	// local 是平台响应前使用本地幂等键的出站消息。
	local := ChatMessage{MessageKey: "local-1", Direction: "outgoing", MessageType: "text", Content: "测试", Status: "sent", SentAt: 1000}
	// stored、inserted、saveErr 保存首次写入结果。
	stored, inserted, saveErr := store.Chats.SaveMessage(ctx, session, local, false)
	if saveErr != nil || !inserted || stored.PlatformMessageID != "" {
		t.Fatalf("save message=%+v inserted=%v err=%v", stored, inserted, saveErr)
	}
	// bound、bindErr 保存 PNM ID 绑定结果。
	bound, bindErr := store.Chats.BindPlatformMessageID(ctx, accountID, "local-1", "platform-1.PNM")
	if bindErr != nil || bound.PlatformMessageID != "platform-1.PNM" {
		t.Fatalf("bind message=%+v err=%v", bound, bindErr)
	}
	// recalled、recallErr 保存平台撤回状态。
	recalled, recallErr := store.Chats.MarkMessageRecalled(ctx, accountID, "platform-1.PNM", 0, "sender-1", 2000)
	if recallErr != nil || recalled.Status != "recalled" || recalled.RecalledAt != 2000 || recalled.RecallOperatorType != 0 || recalled.RecallOperatorID != "sender-1" {
		t.Fatalf("recall message=%+v err=%v", recalled, recallErr)
	}
	// owned、ownedErr 验证用户归属查询仍返回原始内容供发送方重新编辑。
	owned, ownedErr := store.Chats.GetOwnedMessageByKey(ctx, userID, accountID, "local-1")
	if ownedErr != nil || owned.Content != "测试" || owned.PlatformMessageID != "platform-1.PNM" {
		t.Fatalf("owned message=%+v err=%v", owned, ownedErr)
	}
	// history 是随后刷新到的同 PNM 历史消息，不应插入第二行。
	history := ChatMessage{MessageKey: "platform-1.PNM", PlatformMessageID: "platform-1.PNM", Direction: "outgoing", MessageType: "text", Content: "测试", Status: "recalled", SentAt: 1000, RecalledAt: 2000, RecallOperatorType: 0}
	// merged、historyInserted、historyErr 保存平台历史对账结果。
	merged, historyInserted, historyErr := store.Chats.SaveMessage(ctx, session, history, false)
	if historyErr != nil || historyInserted || merged.MessageKey != "local-1" {
		t.Fatalf("history merge=%+v inserted=%v err=%v", merged, historyInserted, historyErr)
	}
	// count 保存该会话最终消息行数，确保没有重复气泡。
	var count int
	// err 表示统计最终消息行数失败。
	if err := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_messages WHERE cookie_id=? AND chat_id=?`, accountID, session.ChatID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("message count=%d err=%v", count, err)
	}
}

// TestSaveMessageMergesMonotonicPlatformState 验证重复历史只推进已读和撤回状态且不会覆盖原始正文。
func TestSaveMessageMergesMonotonicPlatformState(t *testing.T) {
	// store、cleanup 保存隔离 SQLite 存储及关闭函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试数据库调用共用的上下文。
	ctx := context.Background()
	// userID 保存测试账号归属用户主键。
	var userID int64
	// err 表示创建测试用户及读取主键的错误。
	if err := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "merge-owner", "merge-owner@example.com", "test-hash").Scan(&userID); err != nil {
		t.Fatalf("创建测试用户: %v", err)
	}
	// accountID 是不包含凭证的测试账号标识。
	const accountID = "merge-account"
	// err 表示创建测试账号失败。
	if err := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); err != nil {
		t.Fatalf("创建测试账号: %v", err)
	}
	// session 是被重复历史刷新命中的会话。
	session := ChatSession{CookieID: accountID, ChatID: "merge-chat", BuyerID: "buyer-1", BuyerName: "买家"}
	// original 是首次保存且仍未收到状态的跨平台出站消息。
	original := ChatMessage{MessageKey: "merge-1.PNM", PlatformMessageID: "merge-1.PNM", Direction: "outgoing", SenderID: "self", SenderName: "我", MessageType: "text", Content: "原始正文", Status: "sent", SentAt: 1000}
	// inserted、saveErr 记录首条消息保存结果。
	if _, inserted, saveErr := store.Chats.SaveMessage(ctx, session, original, false); saveErr != nil || !inserted {
		t.Fatalf("保存原始消息 inserted=%v err=%v", inserted, saveErr)
	}
	// authoritative 是平台稍后返回的已读撤回状态；隐藏正文不能覆盖已知原文。
	authoritative := ChatMessage{MessageKey: "merge-1.PNM", PlatformMessageID: "merge-1.PNM", Direction: "outgoing", SenderID: "self", SenderName: "我", MessageType: "text", Content: "[系统消息]", Status: "recalled", ReadStatus: 2, ReadAt: 3000, SentAt: 1000, RecalledAt: 3000, RecallOperatorType: 0, RecallOperatorID: "self@goofish"}
	// merged、inserted、mergeErr 保存权威历史合并结果。
	merged, inserted, mergeErr := store.Chats.SaveMessage(ctx, session, authoritative, false)
	if mergeErr != nil || inserted || merged.Status != "recalled" || merged.ReadStatus != 2 || merged.Content != "原始正文" || merged.RecalledAt != 3000 {
		t.Fatalf("权威状态合并 message=%+v inserted=%v err=%v", merged, inserted, mergeErr)
	}
	// stale 模拟更旧的历史响应；已确认状态不得倒退。
	stale := original
	// stable、inserted、staleErr 保存旧快照重放后的稳定状态。
	stable, inserted, staleErr := store.Chats.SaveMessage(ctx, session, stale, false)
	if staleErr != nil || inserted || stable.Status != "recalled" || stable.ReadStatus != 2 || stable.Content != "原始正文" {
		t.Fatalf("状态被旧历史回退 message=%+v inserted=%v err=%v", stable, inserted, staleErr)
	}
}

// TestSaveMessageBindsEarlySelfEchoToLocalOutgoing 验证平台回显早于发送响应时仍复用本地幂等消息。
func TestSaveMessageBindsEarlySelfEchoToLocalOutgoing(t *testing.T) {
	// store、cleanup 保存隔离 SQLite 存储及关闭函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试数据库调用共用的上下文。
	ctx := context.Background()
	// userID 保存测试账号归属用户主键。
	var userID int64
	// err 表示创建测试用户失败。
	if err := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "echo-owner", "echo-owner@example.com", "test-hash").Scan(&userID); err != nil {
		t.Fatalf("创建测试用户: %v", err)
	}
	// accountID 是测试消息所属账号标识。
	const accountID = "echo-account"
	// err 表示创建测试账号失败。
	if err := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); err != nil {
		t.Fatalf("创建测试账号: %v", err)
	}
	// session 是本地发送和平台回显共同所属的会话。
	session := ChatSession{CookieID: accountID, ChatID: "echo-chat", BuyerID: "buyer-1", BuyerName: "买家"}
	// local 是尚未取得平台响应的本地待发送消息。
	local := ChatMessage{MessageKey: "local-echo", Direction: "outgoing", SenderID: accountID, SenderName: "我", MessageType: "text", Content: "相同正文", Status: "sending", SentAt: 1000}
	// inserted、saveErr 保存本地消息首次写入结果。
	if _, inserted, saveErr := store.Chats.SaveMessage(ctx, session, local, false); saveErr != nil || !inserted {
		t.Fatalf("保存本地消息 inserted=%v err=%v", inserted, saveErr)
	}
	// echo 是平台先于 send 响应推送的己方消息回显。
	echo := ChatMessage{MessageKey: "echo-1.PNM", PlatformMessageID: "echo-1.PNM", Direction: "outgoing", SenderID: accountID, SenderName: "我", MessageType: "text", Content: "相同正文", Status: "sent", SentAt: 1500}
	// merged、inserted、echoErr 保存提前回显的合并结果。
	merged, inserted, echoErr := store.Chats.SaveMessage(ctx, session, echo, false)
	if echoErr != nil || inserted || merged.MessageKey != "local-echo" || merged.PlatformMessageID != "echo-1.PNM" || merged.Status != "sent" {
		t.Fatalf("提前回显未复用本地消息 message=%+v inserted=%v err=%v", merged, inserted, echoErr)
	}
	// rebound、bindErr 模拟稍后发送响应重复绑定同一 PNM；该操作必须幂等成功。
	rebound, bindErr := store.Chats.BindPlatformMessageID(ctx, accountID, "local-echo", "echo-1.PNM")
	if bindErr != nil || rebound.PlatformMessageID != "echo-1.PNM" {
		t.Fatalf("重复平台绑定未保持幂等 message=%+v err=%v", rebound, bindErr)
	}
	// count 保存最终消息行数，提前回显不得产生第二个气泡。
	var count int
	// err 表示统计最终消息行数失败。
	if err := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_messages WHERE cookie_id=? AND chat_id=?`, accountID, session.ChatID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("提前回显产生重复消息 count=%d err=%v", count, err)
	}
}
