package db

import (
	"context"
	"testing"
	"time"
)

// TestSettlementSummaryForUserAggregatesOnlyFilteredShippedOrders 验证待结算汇总只统计当前筛选范围内的已发货订单。
func TestSettlementSummaryForUserAggregatesOnlyFilteredShippedOrders(t *testing.T) {
	// store、cleanup 是隔离 SQLite 订单仓储和释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是账号和订单夹具写入及汇总查询共用的上下文。
	ctx := context.Background()
	// userID、accountID 是当前用户及其第一家店铺标识。
	userID, accountID := seedAccount(t, store)
	// otherAccountID 是同一用户的另一家店铺，用于验证账号筛选。
	const otherAccountID = "settlement-other-account"
	if // accountErr 是创建另一家测试店铺的错误。
	accountErr := store.Cookies.CreateOwned(ctx, otherAccountID, "cookie", userID); accountErr != nil {
		t.Fatalf("创建另一家店铺: %v", accountErr)
	}
	// fixtures 保存两笔目标已发货订单及应被状态、金额、时间或账号条件排除的订单。
	fixtures := []struct {
		// orderID 是订单标识，同时承载搜索关键词。
		orderID string
		// cookieID 是订单所属测试店铺。
		cookieID string
		// status 是订单原始状态，覆盖规范已发货和历史数字已发货。
		status string
		// amount 是订单实付元金额。
		amount string
		// createdAt 是平台订单创建时间。
		createdAt string
	}{
		{orderID: "settlement-target-a", cookieID: accountID, status: "shipped", amount: "0.31", createdAt: "2026-08-25T01:00:00Z"},
		{orderID: "settlement-target-b", cookieID: accountID, status: "3", amount: "0.31", createdAt: "2026-08-25T02:00:00Z"},
		{orderID: "settlement-completed", cookieID: accountID, status: "completed", amount: "0.31", createdAt: "2026-08-25T03:00:00Z"},
		{orderID: "settlement-too-large", cookieID: accountID, status: "shipped", amount: "10.00", createdAt: "2026-08-25T04:00:00Z"},
		{orderID: "settlement-too-early", cookieID: accountID, status: "shipped", amount: "0.31", createdAt: "2026-08-24T23:59:59Z"},
		{orderID: "settlement-other-shop", cookieID: otherAccountID, status: "shipped", amount: "0.31", createdAt: "2026-08-25T02:00:00Z"},
	}
	// fixture 是当前待写入的汇总订单夹具。
	for _, fixture := range fixtures {
		// writeErr 是当前订单写入错误。
		writeErr := store.Orders.Upsert(ctx, fixture.orderID, OrderUpsertOpts{CookieID: fixture.cookieID, BuyerID: "buyer", ItemID: "item", OrderStatus: fixture.status, Amount: fixture.amount, CreatedAt: fixture.createdAt})
		if writeErr != nil {
			t.Fatalf("写入订单 %s: %v", fixture.orderID, writeErr)
		}
	}
	// minAmountCents、maxAmountCents 只允许三角一分的测试金额进入汇总。
	minAmountCents, maxAmountCents := int64(30), int64(32)
	// summary、summaryErr 是组合账号、时间、金额、搜索及固定已发货状态后的汇总结果。
	summary, summaryErr := store.Orders.SettlementSummaryForUser(ctx, OrderListFilter{
		UserID: userID, CookieID: accountID, Status: "completed", Search: "settlement-target",
		CreatedFrom: time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC), CreatedTo: time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC),
		MinAmountCents: &minAmountCents, MaxAmountCents: &maxAmountCents,
	})
	if summaryErr != nil {
		t.Fatalf("SettlementSummaryForUser: %v", summaryErr)
	}
	if summary.OrderCount != 2 || summary.GrossAmountCents != 62 {
		t.Fatalf("summary=%+v, want count=2 gross=62", summary)
	}
}
