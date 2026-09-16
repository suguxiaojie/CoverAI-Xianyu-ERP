package db

import (
	"context"
	"errors"
	"testing"
)

// TestOrderShipmentProofsSaveReadAndListFlag 验证凭证 UPSERT、用户隔离和订单列表可用标记。
func TestOrderShipmentProofsSaveReadAndListFlag(t *testing.T) {
	// store、cleanup 是 Schema 52 凭证测试使用的隔离数据库和释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是当前凭证持久化测试上下文。
	ctx := context.Background()
	// userID、cookieID 是凭证订单所属用户和卖家账号。
	userID, cookieID := seedAccount(t, store)
	// orderErr 是待关联订单夹具写入错误。
	orderErr := store.Orders.Upsert(ctx, "proof-order", OrderUpsertOpts{CookieID: cookieID, BuyerID: "buyer", OrderStatus: "shipped"})
	if orderErr != nil {
		t.Fatal(orderErr)
	}
	// saveErr 是首次 ERP 凭证保存错误。
	saveErr := store.OrderShipmentProofs.Save(ctx, OrderShipmentProof{OrderID: "proof-order", CookieID: cookieID, TradeText: "首次描述", ImageURLsJSON: `["https://img.example/one.png"]`, Source: "erp", SubmittedAt: 100})
	if saveErr != nil {
		t.Fatal(saveErr)
	}
	// updateErr 是相同订单后续幂等更新错误。
	updateErr := store.OrderShipmentProofs.Save(ctx, OrderShipmentProof{OrderID: "proof-order", CookieID: cookieID, TradeText: "最终描述", ImageURLsJSON: `[]`, Source: "erp", SubmittedAt: 200})
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	// proof、readErr 是当前用户读取到的最终凭证和错误。
	proof, readErr := store.OrderShipmentProofs.GetForUser(ctx, userID, "proof-order")
	if readErr != nil || proof.TradeText != "最终描述" || proof.ImageURLsJSON != "[]" || proof.SubmittedAt != 200 {
		t.Fatalf("proof=%+v err=%v", proof, readErr)
	}
	// otherUser 是不拥有目标订单的另一个 ERP 用户。
	created, createErr := store.Users.Create(ctx, "proof-other", "proof-other@example.com", "password")
	if createErr != nil || !created {
		t.Fatalf("created=%v err=%v", created, createErr)
	}
	// otherUser 是刚创建且不拥有目标订单的用户实体。
	otherUser, lookupErr := store.Users.GetByUsername(ctx, "proof-other")
	if lookupErr != nil {
		t.Fatal(lookupErr)
	}
	if _, forbiddenErr := store.OrderShipmentProofs.GetForUser(ctx, otherUser.ID, "proof-order"); !errors.Is(forbiddenErr, ErrNotFound) { // forbiddenErr 是跨用户读取必须得到的不存在语义。
		t.Fatalf("forbiddenErr=%v", forbiddenErr)
	}
	// rows、total、listErr 是订单列表对凭证存在标记的读取结果。
	rows, total, listErr := store.Orders.ListForUser(ctx, OrderListFilter{UserID: userID, Limit: 20})
	if listErr != nil || total != 1 || len(rows) != 1 || !rows[0].ShipmentProofAvailable {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, listErr)
	}
}
