package db

import (
	"context"
	"path/filepath"
	"testing"
)

// TestPromoteBuyerShadowOrderRequiresSameUserAndOpenState 验证影子订单只会安全接管到同用户真实卖家且不会覆盖终态或其他卖家。
func TestPromoteBuyerShadowOrderRequiresSameUserAndOpenState(t *testing.T) {
	// ctx 是当前测试所有数据库操作的上下文。
	ctx := context.Background()
	// database、dialect、openErr 是独立 SQLite 测试连接、方言和打开错误。
	database, dialect, openErr := Open(ctx, filepath.Join(t.TempDir(), "shadow.db"))
	if openErr != nil || dialect != DialectSQLite {
		t.Fatalf("open dialect=%s err=%v", dialect, openErr)
	}
	defer database.Close()
	// store 是当前测试使用的数据库仓储集合。
	store := NewStore(database, DialectSQLite)
	// firstUserID、secondUserID 是同用户双账号和跨用户账号的所有者。
	firstUserID, secondUserID := int64(1), int64(2)
	// insertErr 是用户归属夹具写入失败的原因。
	if _, insertErr := store.DB.ExecContext(ctx, `INSERT INTO users(id,username,email,password_hash) VALUES
		(1,'shadow-owner','shadow-owner@example.test','hash'),(2,'other-owner','other-owner@example.test','hash')`); insertErr != nil {
		t.Fatal(insertErr)
	}
	// accountFixtures 是测试涉及的买家账号、真实卖家账号、同用户其他卖家和跨用户账号。
	accountFixtures := []struct {
		// accountID 是 cookies 主键。
		accountID string
		// userID 是本地账号所有者。
		userID int64
	}{{"buyer-account", firstUserID}, {"seller-account", firstUserID}, {"other-seller", firstUserID}, {"foreign-seller", secondUserID}}
	// fixture 是当前待插入的账号归属夹具。
	for _, fixture := range accountFixtures {
		// insertErr 是当前账号夹具写入失败的原因。
		if _, insertErr := store.DB.ExecContext(ctx, `INSERT INTO cookies(id,value,user_id) VALUES(?,?,?)`, fixture.accountID, "sealed", fixture.userID); insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	// insertOrder 插入指定归属和状态的订单夹具。
	insertOrder := func(orderID, owner, status string) {
		t.Helper()
		// insertErr 是当前订单夹具写入失败的原因。
		if _, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders(order_id,buyer_id,cookie_id,order_status,amount,created_at) VALUES(?,?,?,?,?,?)`, orderID, "buyer-account", owner, status, "1.00", "2026-08-20T10:00:00Z"); insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	insertOrder("shadow-open", "buyer-account", "2")
	insertOrder("shadow-deleted", "buyer-account", "unknown")
	insertOrder("shadow-terminal", "buyer-account", "refunded")
	insertOrder("real-seller", "other-seller", "pending_ship")
	// deleteErr 是开放买家影子订单软删除夹具写入错误。
	if _, deleteErr := store.DB.ExecContext(ctx, `UPDATE orders SET deleted_at=CURRENT_TIMESTAMP WHERE order_id='shadow-deleted'`); deleteErr != nil {
		t.Fatal(deleteErr)
	}
	// promoted、promoteErr 是开放买家影子订单的安全接管结果。
	promoted, promoteErr := store.Orders.PromoteBuyerShadowOrder(ctx, PromoteBuyerShadowOrderInput{OrderID: "shadow-open", BuyerAccountID: "buyer-account", SellerAccountID: "seller-account", ChatID: "seller-chat", ItemID: "item-1"})
	if promoteErr != nil || !promoted {
		t.Fatalf("promoted=%v err=%v", promoted, promoteErr)
	}
	// owner、status、chatID、itemID、amount、createdAt 验证归属和早期状态更新，同时保留既有业务数据。
	var owner, status, chatID, itemID, amount, createdAt string
	// scanErr 是接管后订单字段读取失败的原因。
	if scanErr := store.DB.QueryRowContext(ctx, `SELECT cookie_id,order_status,chat_id,item_id,amount,created_at FROM orders WHERE order_id='shadow-open'`).Scan(&owner, &status, &chatID, &itemID, &amount, &createdAt); scanErr != nil {
		t.Fatal(scanErr)
	}
	if owner != "seller-account" || status != "pending_ship" || chatID != "seller-chat" || itemID != "item-1" || amount != "1.00" || createdAt != "2026-08-20T10:00:00Z" {
		t.Fatalf("owner=%s status=%s chat=%s item=%s amount=%s created=%s", owner, status, chatID, itemID, amount, createdAt)
	}
	// deletedPromoted、deletedErr 验证开放影子订单即使被买家同步软删除，也会由新的卖家付款证据恢复。
	deletedPromoted, deletedErr := store.Orders.PromoteBuyerShadowOrder(ctx, PromoteBuyerShadowOrderInput{OrderID: "shadow-deleted", BuyerAccountID: "buyer-account", SellerAccountID: "seller-account"})
	if deletedErr != nil || !deletedPromoted {
		t.Fatalf("deleted promoted=%v err=%v", deletedPromoted, deletedErr)
	}
	// deletedOwner 是恢复后的订单归属。
	var deletedOwner string
	// deletedAt 是恢复后的软删除标记，预期为 nil。
	var deletedAt any
	// scanErr 是恢复后影子订单字段读取错误。
	if scanErr := store.DB.QueryRowContext(ctx, `SELECT cookie_id,deleted_at FROM orders WHERE order_id='shadow-deleted'`).Scan(&deletedOwner, &deletedAt); scanErr != nil || deletedOwner != "seller-account" || deletedAt != nil {
		t.Fatalf("deleted owner=%s deleted_at=%v err=%v", deletedOwner, deletedAt, scanErr)
	}
	// rejectedInputs 覆盖终态、已有真实卖家和跨用户三类禁止接管场景。
	rejectedInputs := []PromoteBuyerShadowOrderInput{
		{OrderID: "shadow-terminal", BuyerAccountID: "buyer-account", SellerAccountID: "seller-account"},
		{OrderID: "real-seller", BuyerAccountID: "buyer-account", SellerAccountID: "seller-account"},
		{OrderID: "shadow-terminal", BuyerAccountID: "buyer-account", SellerAccountID: "foreign-seller"},
	}
	// input 是当前不得接管的归属尝试。
	for _, input := range rejectedInputs {
		// changed、rejectErr 是禁止场景的实际变更标记和数据库错误。
		if changed, rejectErr := store.Orders.PromoteBuyerShadowOrder(ctx, input); rejectErr != nil || changed {
			t.Fatalf("input=%+v changed=%v err=%v", input, changed, rejectErr)
		}
	}
}
