package adapter

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/engine"
)

// TestResolveSystemKeywordReplyTargetUsesExactOrUniqueRecentOrder 验证卖家卡片精确订单和买家通知唯一时间窗关联。
func TestResolveSystemKeywordReplyTargetUsesExactOrUniqueRecentOrder(t *testing.T) {
	// ctx 是订单目标解析测试共用的本地上下文。
	ctx := context.Background()
	// database 是迁移到最新 Schema 的隔离 SQLite。
	database, dialect, openErr := db.Open(ctx, filepath.Join(t.TempDir(), "system-keyword-target.db"))
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer database.Close()
	// store 是包含订单仓储的隔离 Store。
	store := db.NewStore(database, dialect)
	// created 表示脱敏 ERP 用户夹具创建成功。
	created, userErr := store.Users.Create(ctx, "system-keyword-owner", "system-keyword-owner@example.com", "hash")
	if userErr != nil || !created {
		t.Fatalf("create user=%v err=%v", created, userErr)
	}
	// owner 是订单店铺所属的脱敏 ERP 用户。
	owner, ownerErr := store.Users.GetByUsername(ctx, "system-keyword-owner")
	if ownerErr != nil {
		t.Fatal(ownerErr)
	}
	// saveErr 是创建脱敏店铺夹具的数据库结果。
	if saveErr := store.Cookies.Save(ctx, "shop", "unb=shop", owner.ID); saveErr != nil {
		t.Fatal(saveErr)
	}
	// shippedAt 是买家通知一分钟前的订单发货时间。
	shippedAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	// insertErr 是创建首笔已发货订单夹具的数据库结果。
	if _, insertErr := database.ExecContext(ctx, `INSERT INTO orders(order_id,cookie_id,buyer_id,chat_id,order_status,shipped_at) VALUES(?,?,?,?,?,?)`, "order-1", "shop", "buyer", "chat", "shipped", shippedAt); insertErr != nil {
		t.Fatal(insertErr)
	}
	// adapter 是待验证订单归属解析的生产适配器。
	adapter := New(store, nil, nil)
	// exactTarget 是卖家自身卡片按订单号解析出的买家目标。
	exactTarget, exactMatched, exactErr := adapter.ResolveSystemKeywordReplyTarget(ctx, engine.SystemKeywordReplyTargetRequest{AccountID: "shop", EventType: "order_shipped", OrderID: "order-1", ChatID: "chat", BuyerID: "shop", OccurredAt: time.Now().UnixMilli()})
	if exactErr != nil || !exactMatched || exactTarget.OrderID != "order-1" || exactTarget.BuyerID != "buyer" || exactTarget.ChatID != "chat" {
		t.Fatalf("exact target=%+v matched=%v err=%v", exactTarget, exactMatched, exactErr)
	}
	// recentTarget 是无订单号买家通知按同会话唯一近期发货订单解析出的目标。
	recentTarget, recentMatched, recentErr := adapter.ResolveSystemKeywordReplyTarget(ctx, engine.SystemKeywordReplyTargetRequest{AccountID: "shop", EventType: "order_shipped", ChatID: "chat", BuyerID: "buyer", OccurredAt: time.Now().UnixMilli()})
	if recentErr != nil || !recentMatched || recentTarget.OrderID != "order-1" {
		t.Fatalf("recent target=%+v matched=%v err=%v", recentTarget, recentMatched, recentErr)
	}
	// insertErr 是创建同会话第二笔候选以验证歧义拒绝的数据库结果。
	if _, insertErr := database.ExecContext(ctx, `INSERT INTO orders(order_id,cookie_id,buyer_id,chat_id,order_status,shipped_at) VALUES(?,?,?,?,?,?)`, "order-2", "shop", "buyer", "chat", "shipped", shippedAt); insertErr != nil {
		t.Fatal(insertErr)
	}
	// ambiguousMatched 是同会话十分钟内存在两笔候选时的安全拒绝结果。
	_, ambiguousMatched, ambiguousErr := adapter.ResolveSystemKeywordReplyTarget(ctx, engine.SystemKeywordReplyTargetRequest{AccountID: "shop", EventType: "order_shipped", ChatID: "chat", BuyerID: "buyer", OccurredAt: time.Now().UnixMilli()})
	if ambiguousErr != nil || ambiguousMatched {
		t.Fatalf("ambiguous matched=%v err=%v", ambiguousMatched, ambiguousErr)
	}
}
