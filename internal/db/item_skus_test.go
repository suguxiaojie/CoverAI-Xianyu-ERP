package db

import (
	"context"
	"errors"
	"testing"
)

// TestItemSKUSyncPreservesLocalCost 验证平台库存反复同步、逻辑删除和恢复都不会覆盖本地单件成本。
func TestItemSKUSyncPreservesLocalCost(t *testing.T) {
	// store、cleanup 是隔离的 Schema 42 SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是当前测试全部数据库操作使用的上下文。
	ctx := context.Background()
	// created、userErr 是测试商品所有者创建结果和错误。
	if created, userErr := store.Users.Create(ctx, "sku-owner", "sku-owner@example.com", "hash"); userErr != nil || !created {
		t.Fatalf("create owner: created=%v err=%v", created, userErr)
	}
	// owner 是当前商品账号的本地所有者。
	owner, ownerErr := store.Users.GetByUsername(ctx, "sku-owner")
	if ownerErr != nil {
		t.Fatalf("read owner: %v", ownerErr)
	}
	// cookieErr 是测试平台账号保存错误。
	if cookieErr := store.Cookies.Save(ctx, "acc1", "unb=1", owner.ID); cookieErr != nil {
		t.Fatalf("save account: %v", cookieErr)
	}
	// insertErr 是多规格商品基础行保存错误。
	if insertErr := store.Items.Upsert(ctx, &ItemInfoRow{CookieID: "acc1", ItemID: "item-1", ItemTitle: "多规格商品"}); insertErr != nil {
		t.Fatalf("insert item: %v", insertErr)
	}
	// initialRows 是平台首次返回的两个 SKU 库存快照。
	initialRows := []ItemSKURow{
		{SKUID: "sku-1", InventoryID: "inventory-long-1", PropertiesJSON: `[{"name":"套餐","value":"一倍额度","enabled":true,"status":0,"sort_order":0}]`, PriceCents: 13500, Quantity: 955, InitialQuantity: 1000, Enabled: true, SortOrder: 0, SyncedAt: 100},
		{SKUID: "sku-2", InventoryID: "inventory-long-2", PropertiesJSON: `[{"name":"套餐","value":"五倍额度","enabled":true,"status":0,"sort_order":1}]`, PriceCents: 69000, Quantity: 997, InitialQuantity: 1000, Enabled: true, SortOrder: 1, SyncedAt: 100},
	}
	// syncErr 是首次 SKU 快照保存错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "item-1", initialRows); syncErr != nil {
		t.Fatalf("initial sync: %v", syncErr)
	}
	// cost 是用户为第一个 SKU 设置的本地单件成本分值。
	cost := int64(8800)
	// costErr 是本地单件成本保存错误。
	if costErr := store.Items.UpdateSKUCost(ctx, "acc1", "item-1", "sku-1", &cost); costErr != nil {
		t.Fatalf("set cost: %v", costErr)
	}
	// refreshedRows 模拟平台更新售价和剩余库存，同时移除第二个 SKU。
	refreshedRows := []ItemSKURow{{SKUID: "sku-1", InventoryID: "inventory-long-1", PropertiesJSON: initialRows[0].PropertiesJSON, PriceCents: 13600, Quantity: 954, InitialQuantity: 1000, Enabled: true, SortOrder: 0, SyncedAt: 200}}
	// syncErr 是平台库存刷新后的 SKU 对账错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "item-1", refreshedRows); syncErr != nil {
		t.Fatalf("refresh sync: %v", syncErr)
	}
	// activeRows 是平台刷新后的当前 SKU 列表。
	activeRows, listErr := store.Items.ListSKUs(ctx, "acc1", "item-1")
	if listErr != nil || len(activeRows) != 1 {
		t.Fatalf("active rows=%+v err=%v", activeRows, listErr)
	}
	if activeRows[0].Quantity != 954 || activeRows[0].PriceCents != 13600 || activeRows[0].CostCents == nil || *activeRows[0].CostCents != 8800 || activeRows[0].SyncedAt != 200 {
		t.Fatalf("refreshed row=%+v", activeRows[0])
	}
	// restoredRows 模拟同一 SKU 标识再次出现，必须恢复而不是丢失历史本地成本。
	restoredRows := append(refreshedRows, initialRows[1])
	// syncErr 是已消失 SKU 重新出现后的恢复错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "item-1", restoredRows); syncErr != nil {
		t.Fatalf("restore sync: %v", syncErr)
	}
	activeRows, listErr = store.Items.ListSKUs(ctx, "acc1", "item-1")
	if listErr != nil || len(activeRows) != 2 || activeRows[0].CostCents == nil || *activeRows[0].CostCents != 8800 {
		t.Fatalf("restored rows=%+v err=%v", activeRows, listErr)
	}
}

// TestItemSKUCostDistinguishesMissingZeroAndClear 验证未填写、零成本与清除成本具有不同持久化语义。
func TestItemSKUCostDistinguishesMissingZeroAndClear(t *testing.T) {
	// store、cleanup 是隔离的 Schema 42 SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是当前测试数据库操作上下文。
	ctx := context.Background()
	// created、userErr 是成本空值测试所有者创建结果和错误。
	if created, userErr := store.Users.Create(ctx, "sku-clear-owner", "sku-clear-owner@example.com", "hash"); userErr != nil || !created {
		t.Fatalf("create owner: created=%v err=%v", created, userErr)
	}
	// owner 是成本清除场景使用的本地账号所有者。
	owner, ownerErr := store.Users.GetByUsername(ctx, "sku-clear-owner")
	if ownerErr != nil {
		t.Fatalf("read owner: %v", ownerErr)
	}
	// cookieErr 是成本空值测试账号保存错误。
	if cookieErr := store.Cookies.Save(ctx, "acc1", "unb=1", owner.ID); cookieErr != nil {
		t.Fatalf("save account: %v", cookieErr)
	}
	// itemErr 是成本空值测试商品保存错误。
	if itemErr := store.Items.Upsert(ctx, &ItemInfoRow{CookieID: "acc1", ItemID: "item-1"}); itemErr != nil {
		t.Fatalf("insert item: %v", itemErr)
	}
	// syncErr 是成本空值测试 SKU 保存错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "item-1", []ItemSKURow{{SKUID: "sku-1", Enabled: true}}); syncErr != nil {
		t.Fatalf("sync sku: %v", syncErr)
	}
	// rows 是尚未填写成本的初始 SKU。
	rows, _ := store.Items.ListSKUs(ctx, "acc1", "item-1")
	if len(rows) != 1 || rows[0].CostCents != nil {
		t.Fatalf("initial cost=%+v", rows)
	}
	// zero 是用户明确设置的零成本。
	zero := int64(0)
	// updateErr 是显式零成本保存错误。
	if updateErr := store.Items.UpdateSKUCost(ctx, "acc1", "item-1", "sku-1", &zero); updateErr != nil {
		t.Fatalf("set zero cost: %v", updateErr)
	}
	rows, _ = store.Items.ListSKUs(ctx, "acc1", "item-1")
	if rows[0].CostCents == nil || *rows[0].CostCents != 0 {
		t.Fatalf("zero cost=%+v", rows[0].CostCents)
	}
	// clearErr 是清除本地成本的执行错误。
	if clearErr := store.Items.UpdateSKUCost(ctx, "acc1", "item-1", "sku-1", nil); clearErr != nil {
		t.Fatalf("clear cost: %v", clearErr)
	}
	rows, _ = store.Items.ListSKUs(ctx, "acc1", "item-1")
	if rows[0].CostCents != nil {
		t.Fatalf("cleared cost=%+v", rows[0].CostCents)
	}
}

// TestItemSKUCostsAreAtomicAndDefaultCostSurvivesPlatformSync 验证统一保存失败会回滚全部成本，隐式单规格成本不被平台同步删除。
func TestItemSKUCostsAreAtomicAndDefaultCostSurvivesPlatformSync(t *testing.T) {
	// store、cleanup 是隔离的 Schema 42 SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是当前测试数据库操作上下文。
	ctx := context.Background()
	// created、userErr 是测试所有者创建结果和错误。
	if created, userErr := store.Users.Create(ctx, "sku-batch-owner", "sku-batch-owner@example.com", "hash"); userErr != nil || !created {
		t.Fatalf("create owner: created=%v err=%v", created, userErr)
	}
	// owner 是成本批量保存测试的账号所有者。
	owner, ownerErr := store.Users.GetByUsername(ctx, "sku-batch-owner")
	if ownerErr != nil {
		t.Fatalf("read owner: %v", ownerErr)
	}
	// cookieErr 是测试账号保存错误。
	if cookieErr := store.Cookies.Save(ctx, "acc1", "unb=1", owner.ID); cookieErr != nil {
		t.Fatalf("save account: %v", cookieErr)
	}
	// itemErr 是单规格测试商品保存错误。
	if itemErr := store.Items.Upsert(ctx, &ItemInfoRow{CookieID: "acc1", ItemID: "single-item"}); itemErr != nil {
		t.Fatalf("insert single item: %v", itemErr)
	}
	// defaultCost 是单规格商品明确保存的本地成本。
	defaultCost := int64(63000)
	// updateErr 是隐式单规格成本保存错误。
	if updateErr := store.Items.UpdateSKUCosts(ctx, "acc1", "single-item", []ItemSKUCostUpdate{{SKUID: DefaultItemSKUID, CostCents: &defaultCost}}); updateErr != nil {
		t.Fatalf("save default cost: %v", updateErr)
	}
	// syncErr 是平台空 SKU 快照对账错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "single-item", nil); syncErr != nil {
		t.Fatalf("sync empty platform skus: %v", syncErr)
	}
	// rows 是平台空 SKU 同步后的本地隐式成本行。
	rows, listErr := store.Items.ListSKUs(ctx, "acc1", "single-item")
	if listErr != nil || len(rows) != 1 || rows[0].SKUID != DefaultItemSKUID || rows[0].CostCents == nil || *rows[0].CostCents != defaultCost {
		t.Fatalf("default rows=%+v err=%v", rows, listErr)
	}
	// validCost 是原子回滚场景尝试写入的新成本。
	validCost := int64(100)
	// multiErr 是批量保存中混入不存在平台 SKU 时的预期错误。
	multiErr := store.Items.UpdateSKUCosts(ctx, "acc1", "single-item", []ItemSKUCostUpdate{{SKUID: DefaultItemSKUID, CostCents: &validCost}, {SKUID: "missing", CostCents: &validCost}})
	if !errors.Is(multiErr, ErrNotFound) {
		t.Fatalf("batch err=%v", multiErr)
	}
	rows, _ = store.Items.ListSKUs(ctx, "acc1", "single-item")
	if rows[0].CostCents == nil || *rows[0].CostCents != defaultCost {
		t.Fatalf("atomic rollback failed: %+v", rows[0])
	}
}
