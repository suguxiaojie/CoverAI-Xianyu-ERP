package db

import (
	"context"
	"testing"
)

// TestKeywordGroupSaveUpdateAndDuplicateGuard 验证多关键词组一次保存、整组替换和跨组重复词拒绝。
func TestKeywordGroupSaveUpdateAndDuplicateGuard(t *testing.T) {
	// store、cleanup 是隔离的 Schema 45 SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是关键词组测试共享的数据库上下文。
	ctx := context.Background()
	// _, cookieID 创建具备外键归属的测试账号。
	_, cookieID := seedAccount(t, store)
	// groupID、saveErr 是首次保存三个关键词共享回复的结果。
	groupID, saveErr := store.Keywords.SaveGroup(ctx, cookieID, "", []string{"要密码", "要账号", "提供账号"}, "无需提供账号密码", "", "text", "", "contains", "customer", "", 1, 0)
	if saveErr != nil || groupID == "" {
		t.Fatalf("group=%q err=%v", groupID, saveErr)
	}
	// rows、listErr 是首次保存后的独立关键词行。
	rows, listErr := store.Keywords.AllRows(ctx, cookieID)
	if listErr != nil || len(rows) != 3 || rows[0].GroupID != groupID || rows[0].MatchType != "contains" || rows[0].MessageScope != "customer" {
		t.Fatalf("rows=%+v err=%v", rows, listErr)
	}
	// _, updateErr 使用同一 groupID 整组替换关键词和匹配配置。
	_, updateErr := store.Keywords.SaveGroup(ctx, cookieID, groupID, []string{"付款", "已付款"}, "收到付款", "", "text", "", "equals", "system", "order_paid", 1, 0)
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	rows, _ = store.Keywords.AllRows(ctx, cookieID)
	if len(rows) != 2 || rows[0].GroupID != groupID || rows[0].MatchType != "equals" || rows[0].MessageScope != "system" || rows[0].SystemTypes != "order_paid" {
		t.Fatalf("updated rows=%+v", rows)
	}
	// _, duplicateErr 是其他组复用相同账号范围关键词时的预期拒绝。
	_, duplicateErr := store.Keywords.SaveGroup(ctx, cookieID, "", []string{"付款"}, "重复", "", "text", "", "contains", "customer", "", 1, 0)
	if duplicateErr == nil {
		t.Fatal("跨组重复关键词应被拒绝")
	}
}

// TestKeywordGlobalGroupAtomicallyRebindsStores 验证全局规则一次绑定多店铺并在编辑时移除未勾选店铺。
func TestKeywordGlobalGroupAtomicallyRebindsStores(t *testing.T) {
	// store、cleanup 是隔离的 Schema 45 SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是全局规则测试共享的数据库上下文。
	ctx := context.Background()
	// userID、firstCookieID 是测试用户和第一个店铺账号。
	userID, firstCookieID := seedAccount(t, store)
	// secondCookieID 是同一用户的第二个店铺账号。
	secondCookieID := "keyword-shop-2"
	if // saveErr 是第二个测试店铺账号保存错误。
	saveErr := store.Cookies.Save(ctx, secondCookieID, "unb=keyword-shop-2", userID); saveErr != nil {
		t.Fatal(saveErr)
	}
	// groupID、saveErr 是同时绑定两个店铺的全局规则保存结果。
	groupID, saveErr := store.Keywords.SaveGlobalGroup(ctx, []string{firstCookieID, secondCookieID}, []string{firstCookieID, secondCookieID}, "", []string{"账号", "密码"}, "无需提供账号密码", "", "text", "", "contains", "customer", "", true, 1, 0)
	if saveErr != nil || groupID == "" {
		t.Fatalf("group=%q err=%v", groupID, saveErr)
	}
	// boundStores 是规则组首次绑定的不同店铺数量。
	var boundStores int
	if // countErr 是首次全局规则绑定店铺数量查询错误。
	countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT cookie_id) FROM keywords WHERE group_id=?`, groupID).Scan(&boundStores); countErr != nil || boundStores != 2 {
		t.Fatalf("stores=%d err=%v", boundStores, countErr)
	}
	// _, rebindErr 编辑同一全局组，只保留第二个店铺。
	_, rebindErr := store.Keywords.SaveGlobalGroup(ctx, []string{firstCookieID, secondCookieID}, []string{secondCookieID}, groupID, []string{"账号", "密码"}, "无需提供账号密码", "", "text", "", "contains", "customer", "", true, 1, 0)
	if rebindErr != nil {
		t.Fatal(rebindErr)
	}
	// firstRows、secondRows 是重新绑定后两个店铺各自的关键词行数。
	var firstRows, secondRows int
	if // scanErr 是重新绑定后两个店铺关键词数量查询错误。
	scanErr := store.DB.QueryRowContext(ctx, `SELECT SUM(CASE WHEN cookie_id=? THEN 1 ELSE 0 END),SUM(CASE WHEN cookie_id=? THEN 1 ELSE 0 END) FROM keywords WHERE group_id=?`, firstCookieID, secondCookieID, groupID).Scan(&firstRows, &secondRows); scanErr != nil || firstRows != 0 || secondRows != 2 {
		t.Fatalf("first=%d second=%d err=%v", firstRows, secondRows, scanErr)
	}
}

// TestKeywordGlobalEnabledSwitches 验证单规则和全部规则开关会同步所有店铺并阻止运行时读取停用规则。
func TestKeywordGlobalEnabledSwitches(t *testing.T) {
	// store、cleanup 是隔离的 Schema 45 SQLite 存储和清理函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是规则启停测试共享的数据库上下文。
	ctx := context.Background()
	// userID、firstCookieID 创建测试用户和第一个店铺账号。
	userID, firstCookieID := seedAccount(t, store)
	// secondCookieID 是同一用户的第二个测试店铺账号。
	secondCookieID := "keyword-enabled-shop-2"
	if // saveErr 是第二个店铺账号保存错误。
	saveErr := store.Cookies.Save(ctx, secondCookieID, "unb=keyword-enabled-shop-2", userID); saveErr != nil {
		t.Fatal(saveErr)
	}
	// groupID、saveErr 是默认开启并绑定两个店铺的规则组保存结果。
	groupID, saveErr := store.Keywords.SaveGlobalGroup(ctx, []string{firstCookieID, secondCookieID}, []string{firstCookieID, secondCookieID}, "", []string{"开关测试"}, "已命中", "", "text", "", "contains", "customer", "", true, 1, 0)
	if saveErr != nil {
		t.Fatal(saveErr)
	}
	if // disableErr 是单规则关闭错误。
	disableErr := store.Keywords.SetGlobalGroupEnabled(ctx, []string{firstCookieID, secondCookieID}, groupID, false); disableErr != nil {
		t.Fatal(disableErr)
	}
	// runtimeRows 是关闭后 Engine 可读取的关键词集合。
	runtimeRows, runtimeErr := store.Keywords.AllWithType(ctx, firstCookieID)
	if runtimeErr != nil || len(runtimeRows) != 0 {
		t.Fatalf("disabled runtime rows=%+v err=%v", runtimeRows, runtimeErr)
	}
	if // enableAllErr 是全部规则重新开启错误。
	enableAllErr := store.Keywords.SetAllGlobalGroupsEnabled(ctx, []string{firstCookieID, secondCookieID}, true); enableAllErr != nil {
		t.Fatal(enableAllErr)
	}
	runtimeRows, runtimeErr = store.Keywords.AllWithType(ctx, firstCookieID)
	if runtimeErr != nil || len(runtimeRows) != 1 {
		t.Fatalf("enabled runtime rows=%+v err=%v", runtimeRows, runtimeErr)
	}
}
