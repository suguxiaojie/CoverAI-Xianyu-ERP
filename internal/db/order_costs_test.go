package db

import (
	"context"
	"fmt"
	"testing"
)

// TestOrderCostSnapshotLocksSingleItemCost 验证单规格订单会锁定成本，后续改成本不会改写历史快照。
func TestOrderCostSnapshotLocksSingleItemCost(t *testing.T) {
	// store、cleanup 是隔离的当前 Schema SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试数据库操作共享的上下文。
	ctx := context.Background()
	// cookieID 是测试订单与商品共同使用的账号标识。
	_, cookieID := seedAccount(t, store)
	if // itemErr 是单规格商品基础信息保存错误。
	itemErr := store.Items.Upsert(ctx, &ItemInfoRow{CookieID: cookieID, ItemID: "single-profit", ItemTitle: "单规格利润商品"}); itemErr != nil {
		t.Fatal(itemErr)
	}
	// initialCost 是订单创建时应锁定的单件成本分值。
	initialCost := int64(11800)
	if // costErr 是本地隐式 SKU 成本保存错误。
	costErr := store.Items.UpdateSKUCosts(ctx, cookieID, "single-profit", []ItemSKUCostUpdate{{SKUID: DefaultItemSKUID, CostCents: &initialCost}}); costErr != nil {
		t.Fatal(costErr)
	}
	if // writeErr 是订单及成本快照原子写入错误。
	writeErr := store.OrderWrites.WithTransaction(ctx, func(transaction *OrderWriteTransaction) error {
		return transaction.UpsertOrder(ctx, "profit-order-single", OrderUpsertOpts{CookieID: cookieID, ItemID: "single-profit", OrderStatus: "paid", Quantity: "2", Amount: "270.00"})
	}); writeErr != nil {
		t.Fatal(writeErr)
	}
	// updatedCost 是订单创建后重新配置的 SKU 成本分值。
	updatedCost := int64(12500)
	if // updateErr 是订单创建后成本更新错误。
	updateErr := store.Items.UpdateSKUCost(ctx, cookieID, "single-profit", DefaultItemSKUID, &updatedCost); updateErr != nil {
		t.Fatal(updateErr)
	}
	if // writeErr 是同一订单后续状态同步及快照幂等检查错误。
	writeErr := store.OrderWrites.WithTransaction(ctx, func(transaction *OrderWriteTransaction) error {
		return transaction.UpsertOrder(ctx, "profit-order-single", OrderUpsertOpts{CookieID: cookieID, ItemID: "single-profit", OrderStatus: "shipped", Quantity: "2", Amount: "270.00"})
	}); writeErr != nil {
		t.Fatal(writeErr)
	}
	// unitCost、quantity、source 是数据库锁定的历史成本快照字段。
	var unitCost int64
	// quantity 是快照锁定的购买数量。
	var quantity int
	// source 是快照记录的精确匹配来源。
	var source string
	if // scanErr 是历史成本快照读取错误。
	scanErr := store.DB.QueryRowContext(ctx, `SELECT unit_cost_cents,quantity,match_source FROM order_cost_snapshots WHERE order_id=?`, "profit-order-single").Scan(&unitCost, &quantity, &source); scanErr != nil {
		t.Fatal(scanErr)
	}
	if unitCost != initialCost || quantity != 2 || source != "single_local_default" {
		t.Fatalf("snapshot cost=%d quantity=%d source=%q", unitCost, quantity, source)
	}
}

// TestOrderCostSnapshotRequiresExactMultiSpec 验证多规格订单只有规格值唯一精确匹配时才会创建成本快照。
func TestOrderCostSnapshotRequiresExactMultiSpec(t *testing.T) {
	// store、cleanup 是隔离的当前 Schema SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试数据库操作共享的上下文。
	ctx := context.Background()
	// cookieID 是测试订单与商品共同使用的账号标识。
	_, cookieID := seedAccount(t, store)
	if // itemErr 是多规格商品基础信息保存错误。
	itemErr := store.Items.Upsert(ctx, &ItemInfoRow{CookieID: cookieID, ItemID: "multi-profit", ItemTitle: "多规格利润商品", IsMultiSpec: true}); itemErr != nil {
		t.Fatal(itemErr)
	}
	// skuRows 是两个具备不同规格值和本地成本的平台 SKU。
	skuRows := []ItemSKURow{
		{SKUID: "sku-one", PropertiesJSON: `[{"name":"套餐","value":"一倍额度"}]`, Enabled: true},
		{SKUID: "sku-five", PropertiesJSON: `[{"name":"套餐","value":"五倍额度"}]`, Enabled: true},
	}
	if // syncErr 是平台 SKU 测试夹具保存错误。
	syncErr := store.Items.SyncSKUs(ctx, cookieID, "multi-profit", skuRows); syncErr != nil {
		t.Fatal(syncErr)
	}
	// oneCost、fiveCost 是两个 SKU 各自的本地单件成本分值。
	oneCost, fiveCost := int64(11800), int64(63000)
	if // costErr 是两个规格成本的统一保存错误。
	costErr := store.Items.UpdateSKUCosts(ctx, cookieID, "multi-profit", []ItemSKUCostUpdate{{SKUID: "sku-one", CostCents: &oneCost}, {SKUID: "sku-five", CostCents: &fiveCost}}); costErr != nil {
		t.Fatal(costErr)
	}
	// orders 是精确规格、空规格和非法数量三类订单输入。
	orders := []struct {
		// orderID 是测试订单标识。
		orderID string
		// specValue 是订单规格值。
		specValue string
		// quantity 是订单购买数量文本。
		quantity string
	}{
		{orderID: "profit-order-exact", specValue: "五倍额度", quantity: "1"},
		{orderID: "profit-order-blank", specValue: "", quantity: "1"},
		{orderID: "profit-order-invalid-quantity", specValue: "一倍额度", quantity: "unknown"},
	}
	// order 是当前写入并检查快照创建条件的订单输入。
	for _, order := range orders {
		if // writeErr 是当前规格订单及可选成本快照写入错误。
		writeErr := store.OrderWrites.WithTransaction(ctx, func(transaction *OrderWriteTransaction) error {
			return transaction.UpsertOrder(ctx, order.orderID, OrderUpsertOpts{CookieID: cookieID, ItemID: "multi-profit", OrderStatus: "completed", SpecName: "套餐", SpecValue: order.specValue, Quantity: order.quantity, Amount: "690.00"})
		}); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	// snapshotCount 是三类订单中实际创建的可靠成本快照数量。
	var snapshotCount int
	if // countErr 是可靠成本快照数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_cost_snapshots`).Scan(&snapshotCount); countErr != nil {
		t.Fatal(countErr)
	}
	if snapshotCount != 1 {
		t.Fatalf("snapshot count=%d, want 1", snapshotCount)
	}
	// skuID、unitCost 是精确规格订单锁定的平台 SKU 和单件成本。
	var skuID string
	// unitCost 是精确规格快照锁定的成本分值。
	var unitCost int64
	if // scanErr 是精确规格成本快照读取错误。
	scanErr := store.DB.QueryRowContext(ctx, `SELECT sku_id,unit_cost_cents FROM order_cost_snapshots WHERE order_id=?`, "profit-order-exact").Scan(&skuID, &unitCost); scanErr != nil {
		t.Fatal(scanErr)
	}
	if skuID != "sku-five" || unitCost != fiveCost {
		t.Fatalf("snapshot sku=%q cost=%d", skuID, unitCost)
	}
}

// TestConfirmManualCostGroupLocksOnlyConfirmedAmount 验证人工确认只覆盖同用户、同商品和同金额分组，并区分标准售价与改价来源。
func TestConfirmManualCostGroupLocksOnlyConfirmedAmount(t *testing.T) {
	// store、cleanup 是隔离的当前 Schema SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是人工确认测试共享的数据库上下文。
	ctx := context.Background()
	// userID、cookieID 是人工候选订单和 SKU 的所有者与账号标识。
	userID, cookieID := seedAccount(t, store)
	if // itemErr 是人工候选测试商品保存错误。
	itemErr := store.Items.Upsert(ctx, &ItemInfoRow{CookieID: cookieID, ItemID: "manual-item", ItemTitle: "人工候选商品", IsMultiSpec: true}); itemErr != nil {
		t.Fatal(itemErr)
	}
	// skuRows 是一倍和五倍额度的平台 SKU 测试夹具。
	skuRows := []ItemSKURow{{SKUID: "sku-one", PropertiesJSON: `[{"name":"套餐","value":"一倍额度"}]`, PriceCents: 13500, Enabled: true}, {SKUID: "sku-five", PropertiesJSON: `[{"name":"套餐","value":"五倍额度"}]`, PriceCents: 69000, Enabled: true}}
	if // syncErr 是人工候选测试 SKU 保存错误。
	syncErr := store.Items.SyncSKUs(ctx, cookieID, "manual-item", skuRows); syncErr != nil {
		t.Fatal(syncErr)
	}
	// oneCost、fiveCost 是两个 SKU 当前人工确认使用的成本分值。
	oneCost, fiveCost := int64(11800), int64(63000)
	if // costErr 是人工候选 SKU 成本保存错误。
	costErr := store.Items.UpdateSKUCosts(ctx, cookieID, "manual-item", []ItemSKUCostUpdate{{SKUID: "sku-one", CostCents: &oneCost}, {SKUID: "sku-five", CostCents: &fiveCost}}); costErr != nil {
		t.Fatal(costErr)
	}
	// orders 是两个标准售价订单、一个改价订单和一个退款排除订单。
	orders := []OrderUpsertOpts{{CookieID: cookieID, ItemID: "manual-item", OrderStatus: "completed", Quantity: "1", Amount: "135.00"}, {CookieID: cookieID, ItemID: "manual-item", OrderStatus: "shipped", Quantity: "2", Amount: "135.00"}, {CookieID: cookieID, ItemID: "manual-item", OrderStatus: "completed", Quantity: "1", Amount: "148.50"}, {CookieID: cookieID, ItemID: "manual-item", OrderStatus: "refunded", Quantity: "1", Amount: "135.00"}}
	// index、order 是当前写入的人工候选订单下标和字段。
	for index, order := range orders {
		if // orderErr 是当前人工候选订单保存错误。
		orderErr := store.Orders.Upsert(ctx, fmt.Sprintf("manual-order-%d", index), order); orderErr != nil {
			t.Fatal(orderErr)
		}
	}
	// forbiddenMatched、forbiddenSource、forbiddenErr 验证人工确认不能把所选 SKU 跨账号应用。
	forbiddenMatched, forbiddenSource, forbiddenErr := store.OrderCosts.ConfirmManualGroup(ctx, userID, "another-account", "manual-item", 13500, 1, "", "", "sku-one", nil)
	if forbiddenErr == nil || forbiddenMatched != 0 || forbiddenSource != "" {
		t.Fatalf("cross-account confirmation matched=%d source=%q err=%v", forbiddenMatched, forbiddenSource, forbiddenErr)
	}
	// matched、source、confirmErr 是全账号口径下首个金额分组的确认结果。
	matched, source, confirmErr := store.OrderCosts.ConfirmManualGroup(ctx, userID, cookieID, "manual-item", 13500, 1, "", "", "sku-one", nil)
	if confirmErr != nil || matched != 1 || source != "manual_exact_price" {
		t.Fatalf("matched=%d source=%q err=%v", matched, source, confirmErr)
	}
	// quantityTwoMatched、quantityTwoSource、quantityTwoErr 是同金额但购买数量为二的独立分组确认结果。
	quantityTwoMatched, quantityTwoSource, quantityTwoErr := store.OrderCosts.ConfirmManualGroup(ctx, userID, cookieID, "manual-item", 13500, 2, "", "", "sku-one", nil)
	if quantityTwoErr != nil || quantityTwoMatched != 1 || quantityTwoSource != "manual_exact_price" {
		t.Fatalf("quantity two matched=%d source=%q err=%v", quantityTwoMatched, quantityTwoSource, quantityTwoErr)
	}
	// adjustedMatched、adjustedSource、adjustedErr 是改价金额分组人工确认结果。
	// customCost 是改价订单实际采用的人工历史单件成本分值。
	customCost := int64(9900)
	// adjustedMatched、adjustedSource、adjustedErr 是自定义历史成本确认结果和错误。
	adjustedMatched, adjustedSource, adjustedErr := store.OrderCosts.ConfirmManualGroup(ctx, userID, cookieID, "manual-item", 14850, 1, "", "", "sku-one", &customCost)
	if adjustedErr != nil || adjustedMatched != 1 || adjustedSource != "manual_custom_cost" {
		t.Fatalf("matched=%d source=%q err=%v", adjustedMatched, adjustedSource, adjustedErr)
	}
	// snapshotCount 是人工确认后实际生成的不可变成本快照数量。
	var snapshotCount int
	if // countErr 是人工确认后成本快照数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_cost_snapshots`).Scan(&snapshotCount); countErr != nil || snapshotCount != 3 {
		t.Fatalf("snapshots=%d err=%v", snapshotCount, countErr)
	}
}
