package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// TestKnowledgeStorePersistsScopesReviewChunksAndRedactedLogs 验证 SQLite 知识库归属、审核门禁、分块和脱敏日志。
func TestKnowledgeStorePersistsScopesReviewChunksAndRedactedLogs(t *testing.T) {
	// databasePath 是只服务于本测试的临时 SQLite 文件。
	databasePath := filepath.Join(t.TempDir(), "knowledge.db")
	// ctx 是所有本地仓储操作共用的测试上下文。
	ctx := context.Background()
	// database 是已自动迁移到当前最新 Schema 的隔离数据库。
	database, dialect, openErr := Open(ctx, databasePath)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer database.Close()
	// store 是包含知识库仓储的 SQLite Store。
	store := NewStore(database, dialect)
	// created 表示第一个知识库所属用户创建成功。
	created, userErr := store.Users.Create(ctx, "knowledge-owner", "knowledge-owner@example.com", "hash")
	if userErr != nil || !created {
		t.Fatalf("create owner=%v err=%v", created, userErr)
	}
	// owner 是待绑定店铺和知识库的 ERP 用户。
	owner, ownerErr := store.Users.GetByUsername(ctx, "knowledge-owner")
	if ownerErr != nil {
		t.Fatal(ownerErr)
	}
	// saveErr 是为知识范围创建脱敏店铺夹具的失败原因。
	if saveErr := store.Cookies.Save(ctx, "knowledge-shop", "unb=1", owner.ID); saveErr != nil {
		t.Fatal(saveErr)
	}
	// itemErr 是为知识范围创建脱敏商品夹具的失败原因。
	if _, itemErr := database.ExecContext(ctx, `INSERT INTO item_info(cookie_id,item_id,item_title) VALUES(?,?,?)`, "knowledge-shop", "knowledge-item", "脱敏商品"); itemErr != nil {
		t.Fatal(itemErr)
	}
	// knowledgeBaseID 是同时绑定所属店铺和商品的新草稿知识库。
	knowledgeBaseID, baseErr := store.Knowledge.CreateBase(ctx, owner.ID, KnowledgeBaseDraftRow{Name: "使用教程", Description: "已确认的使用说明", CookieIDs: []string{"knowledge-shop"}, ItemScopes: []KnowledgeItemScopeRow{{CookieID: "knowledge-shop", ItemID: "knowledge-item"}}})
	if baseErr != nil {
		t.Fatal(baseErr)
	}
	// base 是重新读取并已补齐范围与计数的知识库。
	base, getErr := store.Knowledge.GetBase(ctx, owner.ID, knowledgeBaseID)
	if getErr != nil || len(base.CookieIDs) != 1 || len(base.ItemScopes) != 1 || base.Status != "draft" {
		t.Fatalf("base=%+v err=%v", base, getErr)
	}
	// entryID 是默认待审核、停用的 FAQ 主键。
	entryID, entryErr := store.Knowledge.CreateEntry(ctx, owner.ID, knowledgeBaseID, KnowledgeEntryDraftRow{Type: "faq", Title: "怎么用？", Content: "请按权威教程操作。", ContentType: "faq", ContentHash: "faq-hash", RiskLevel: "low"}, []KnowledgeChunkRow{{Index: 0, Content: "怎么用？\n请按权威教程操作。", ContentHash: "chunk-hash"}})
	if entryErr != nil {
		t.Fatal(entryErr)
	}
	// entry 是创建后的安全默认 FAQ 状态。
	entry, readErr := store.Knowledge.GetEntry(ctx, owner.ID, knowledgeBaseID, entryID, "faq")
	if readErr != nil || entry.ReviewStatus != "pending" || entry.Status != "draft" || entry.Enabled || entry.AllowAutoReply {
		t.Fatalf("entry=%+v err=%v", entry, readErr)
	}
	// reviewErr 是人工审核 FAQ 夹具的失败原因。
	if reviewErr := store.Knowledge.ReviewEntry(ctx, owner.ID, knowledgeBaseID, entryID, "faq", true); reviewErr != nil {
		t.Fatal(reviewErr)
	}
	// enableErr 是启用已审核 FAQ 离线检索的失败原因。
	if enableErr := store.Knowledge.SetEntryEnabled(ctx, owner.ID, knowledgeBaseID, entryID, "faq", true); enableErr != nil {
		t.Fatal(enableErr)
	}
	// statusErr 是启用知识库离线检索的失败原因。
	if statusErr := store.Knowledge.SetBaseStatus(ctx, owner.ID, knowledgeBaseID, "active"); statusErr != nil {
		t.Fatal(statusErr)
	}
	// aliasID 是用户确认写入的 FAQ 相似问法主键。
	aliasID, aliasErr := store.Knowledge.CreateFAQAlias(ctx, owner.ID, knowledgeBaseID, entryID, "怎么操作", "如何操作", "debug")
	if aliasErr != nil || aliasID <= 0 {
		t.Fatalf("create alias id=%d err=%v", aliasID, aliasErr)
	}
	// aliases 是重新读取的 FAQ 相似问法列表。
	aliases, aliasesErr := store.Knowledge.ListFAQAliases(ctx, owner.ID, knowledgeBaseID, entryID)
	if aliasesErr != nil || len(aliases) != 1 || aliases[0].Alias != "怎么操作" || aliases[0].Source != "debug" {
		t.Fatalf("aliases=%+v err=%v", aliases, aliasesErr)
	}
	// conflict 是同一知识库复用规范化相似问法的预期冲突。
	conflict, conflictErr := store.Knowledge.FAQAliasConflict(ctx, owner.ID, knowledgeBaseID, entryID, "如何操作")
	if conflictErr != nil || !conflict {
		t.Fatalf("alias conflict=%v err=%v", conflict, conflictErr)
	}
	// retrievable 是审核、条目启用和知识库启用后的唯一候选。
	retrievable, retrieveErr := store.Knowledge.ListRetrievableEntries(ctx, owner.ID, []int64{knowledgeBaseID}, 100)
	if retrieveErr != nil || len(retrievable) != 1 || retrievable[0].KnowledgeBaseName != "使用教程" || len(retrievable[0].Aliases) != 1 {
		t.Fatalf("retrievable=%+v err=%v", retrievable, retrieveErr)
	}
	// logErr 是写入不含原始问题的脱敏检索日志失败原因。
	if logErr := store.Knowledge.AddRetrieveLog(ctx, owner.ID, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "answerable", 1); logErr != nil {
		t.Fatal(logErr)
	}
	// logCount 是只保存摘要的检索日志数量。
	var logCount int
	// queryErr 是核对脱敏日志只按问题摘要写入的查询结果。
	if queryErr := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_retrieve_logs WHERE user_id=? AND query_digest=?`, owner.ID, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef").Scan(&logCount); queryErr != nil || logCount != 1 {
		t.Fatalf("log count=%d err=%v", logCount, queryErr)
	}
	// chunkCount 是 FAQ 主表与同事务写入的分块数量。
	var chunkCount int
	// queryErr 是核对 FAQ 与分块事务一致性的查询结果。
	if queryErr := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_chunks WHERE source_type='faq' AND source_id=?`, entryID).Scan(&chunkCount); queryErr != nil || chunkCount != 1 {
		t.Fatalf("chunk count=%d err=%v", chunkCount, queryErr)
	}
}

// TestKnowledgeStoreRejectsCrossUserResources 验证用户不能读取、更新或绑定其他用户的知识资源。
func TestKnowledgeStoreRejectsCrossUserResources(t *testing.T) {
	// databasePath 是用户隔离测试的临时 SQLite 文件。
	databasePath := filepath.Join(t.TempDir(), "knowledge-ownership.db")
	// ctx 是全部隔离操作共用的测试上下文。
	ctx := context.Background()
	// database 是已迁移到最新 Schema 的隔离数据库。
	database, dialect, openErr := Open(ctx, databasePath)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer database.Close()
	// store 是隔离测试使用的 SQLite Store。
	store := NewStore(database, dialect)
	// username 是当前待创建的脱敏 ERP 用户名。
	for _, username := range []string{"owner-a", "owner-b"} {
		// created 和 createErr 是当前脱敏 ERP 用户的创建结果。
		if created, createErr := store.Users.Create(ctx, username, username+"@example.com", "hash"); createErr != nil || !created {
			t.Fatalf("create %s=%v err=%v", username, created, createErr)
		}
	}
	// ownerA 是知识库的真实所有者。
	ownerA, ownerAErr := store.Users.GetByUsername(ctx, "owner-a")
	// ownerB 是尝试跨用户操作的其他用户。
	ownerB, ownerBErr := store.Users.GetByUsername(ctx, "owner-b")
	if ownerAErr != nil || ownerBErr != nil {
		t.Fatal(ownerAErr, ownerBErr)
	}
	// saveErr 是为 ownerA 创建店铺范围夹具的失败原因。
	if saveErr := store.Cookies.Save(ctx, "owner-a-shop", "unb=1", ownerA.ID); saveErr != nil {
		t.Fatal(saveErr)
	}
	// knowledgeBaseID 是 ownerA 创建的草稿知识库。
	knowledgeBaseID, createErr := store.Knowledge.CreateBase(ctx, ownerA.ID, KnowledgeBaseDraftRow{Name: "A 知识", Description: "A 用户权威内容", CookieIDs: []string{"owner-a-shop"}})
	if createErr != nil {
		t.Fatal(createErr)
	}
	// getErr 是 ownerB 跨用户读取 ownerA 知识库的预期不存在错误。
	if _, getErr := store.Knowledge.GetBase(ctx, ownerB.ID, knowledgeBaseID); !errors.Is(getErr, ErrNotFound) {
		t.Fatalf("cross-user get err=%v", getErr)
	}
	// updateErr 是 ownerB 跨用户更新 ownerA 知识库的预期不存在错误。
	if updateErr := store.Knowledge.UpdateBase(ctx, ownerB.ID, knowledgeBaseID, KnowledgeBaseDraftRow{Name: "B 修改", Description: "不允许"}); !errors.Is(updateErr, ErrNotFound) {
		t.Fatalf("cross-user update err=%v", updateErr)
	}
	// bindErr 是 ownerB 尝试绑定 ownerA 店铺的预期禁止错误。
	if _, bindErr := store.Knowledge.CreateBase(ctx, ownerB.ID, KnowledgeBaseDraftRow{Name: "B 知识", Description: "B 用户不可绑定 A 店铺", CookieIDs: []string{"owner-a-shop"}}); !errors.Is(bindErr, ErrForbidden) {
		t.Fatalf("cross-user binding err=%v", bindErr)
	}
}
