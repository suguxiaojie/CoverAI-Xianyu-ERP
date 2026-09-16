package db

import (
	"context"
	"testing"
)

// TestOrderSyncCursorsUpsert 验证账号高水位可以创建、覆盖并保持账号级唯一性。
func TestOrderSyncCursorsUpsert(t *testing.T) {
	// store、cleanup 保存迁移完成的 SQLite 测试数据库及清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本次仓储读写使用的测试上下文。
	ctx := context.Background()
	// _, cookieID 保存具备外键父记录的测试账号。
	_, cookieID := seedAccount(t, store)
	// first 保存首次完整基线游标。
	first := OrderSyncCursor{CookieID: cookieID, HighWaterCreatedAt: "2025-01-01T00:00:00Z", HighWaterOrderID: "order-1", LastIncrementalSyncAt: 10, LastFullSyncAt: 10}
	// err 保存首次游标写入错误，后续复用检查覆盖写入。
	err := store.OrderSyncCursors.Upsert(ctx, first)
	if err != nil {
		t.Fatalf("insert cursor: %v", err)
	}
	// second 保存后续增量同步推进后的高水位。
	second := OrderSyncCursor{CookieID: cookieID, HighWaterCreatedAt: "2025-01-02T00:00:00Z", HighWaterOrderID: "order-2", LastIncrementalSyncAt: 20, LastFullSyncAt: 10}
	err = store.OrderSyncCursors.Upsert(ctx, second)
	if err != nil {
		t.Fatalf("update cursor: %v", err)
	}
	// loaded、exists、err 保存覆盖后的游标查询结果。
	loaded, exists, err := store.OrderSyncCursors.Get(ctx, cookieID)
	if err != nil || !exists || loaded.HighWaterOrderID != "order-2" || loaded.LastIncrementalSyncAt != 20 || loaded.LastFullSyncAt != 10 {
		t.Fatalf("cursor=%+v exists=%v err=%v", loaded, exists, err)
	}
	// missing、exists、err 保存不存在账号游标的兼容结果。
	missing, exists, err := store.OrderSyncCursors.Get(ctx, "missing")
	if err != nil || exists || missing != nil {
		t.Fatalf("missing cursor=%+v exists=%v err=%v", missing, exists, err)
	}
}
