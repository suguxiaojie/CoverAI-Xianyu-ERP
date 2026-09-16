package db

import (
	"context"
	"database/sql"
	"errors"
)

// ListEntries 返回当前用户知识库下的全部 FAQ 和文档。
func (store *KnowledgeStore) ListEntries(ctx context.Context, userID, knowledgeBaseID int64) ([]KnowledgeEntryRow, error) {
	// baseErr 是列出条目前按用户和知识库 ID 执行归属复核的结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return nil, baseErr
	}
	// result 是当前知识库的 FAQ 和文档合并列表。
	result := make([]KnowledgeEntryRow, 0)
	// faqRows 是当前知识库的 FAQ 持久化游标。
	faqRows, faqErr := store.DB.QueryContext(ctx, `SELECT id,knowledge_base_id,'faq',question,answer,'faq',status,review_status,risk_level,requires_live_data,allow_auto_reply,enabled,effective_from,effective_to,created_at,updated_at FROM knowledge_faqs WHERE knowledge_base_id=? ORDER BY updated_at DESC,id DESC`, knowledgeBaseID)
	if faqErr != nil {
		return nil, faqErr
	}
	// faqRows.Next 每次推进到一条 FAQ。
	for faqRows.Next() {
		// row 是当前待扫描的 FAQ 持久化模型。
		var row KnowledgeEntryRow
		// scanErr 是当前 FAQ 行扫描失败原因。
		if scanErr := scanKnowledgeEntry(faqRows, &row); scanErr != nil {
			faqRows.Close()
			return nil, scanErr
		}
		result = append(result, row)
	}
	// faqRowsErr 是 FAQ 列表游标遍历期间的底层错误。
	if faqRowsErr := faqRows.Err(); faqRowsErr != nil {
		faqRows.Close()
		return nil, faqRowsErr
	}
	faqRows.Close()
	// documentRows 是当前知识库的文档持久化游标。
	documentRows, documentErr := store.DB.QueryContext(ctx, `SELECT id,knowledge_base_id,'document',title,content,content_type,status,review_status,risk_level,requires_live_data,allow_auto_reply,enabled,effective_from,effective_to,created_at,updated_at FROM knowledge_documents WHERE knowledge_base_id=? ORDER BY updated_at DESC,id DESC`, knowledgeBaseID)
	if documentErr != nil {
		return nil, documentErr
	}
	defer documentRows.Close()
	// documentRows.Next 每次推进到一份纯文本或 Markdown 文档。
	for documentRows.Next() {
		// row 是当前待扫描的文档持久化模型。
		var row KnowledgeEntryRow
		// scanErr 是当前文档行扫描失败原因。
		if scanErr := scanKnowledgeEntry(documentRows, &row); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, row)
	}
	return result, documentRows.Err()
}

// GetEntry 返回当前用户知识库下的单个 FAQ 或文档。
func (store *KnowledgeStore) GetEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType string) (KnowledgeEntryRow, error) {
	// baseErr 是读取单条知识前的用户与知识库归属复核结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return KnowledgeEntryRow{}, baseErr
	}
	// query 是根据经应用校验的内容类型选择的固定查询。
	query := `SELECT id,knowledge_base_id,'faq',question,answer,'faq',status,review_status,risk_level,requires_live_data,allow_auto_reply,enabled,effective_from,effective_to,created_at,updated_at FROM knowledge_faqs WHERE knowledge_base_id=? AND id=?`
	if contentType == "document" {
		query = `SELECT id,knowledge_base_id,'document',title,content,content_type,status,review_status,risk_level,requires_live_data,allow_auto_reply,enabled,effective_from,effective_to,created_at,updated_at FROM knowledge_documents WHERE knowledge_base_id=? AND id=?`
	}
	// row 是当前待扫描的 FAQ 或文档持久化模型。
	var row KnowledgeEntryRow
	// scanErr 是单个 FAQ 或文档扫描结果，无行时转换为稳定不存在错误。
	if scanErr := scanKnowledgeEntry(store.DB.QueryRowContext(ctx, query, knowledgeBaseID, entryID), &row); errors.Is(scanErr, sql.ErrNoRows) {
		return KnowledgeEntryRow{}, ErrNotFound
	} else if scanErr != nil {
		return KnowledgeEntryRow{}, scanErr
	}
	return row, nil
}

// CreateEntry 原子创建默认待审核、停用的 FAQ／文档及其分块。
func (store *KnowledgeStore) CreateEntry(ctx context.Context, userID, knowledgeBaseID int64, draft KnowledgeEntryDraftRow, chunks []KnowledgeChunkRow) (int64, error) {
	// baseErr 是创建条目前的用户与知识库归属复核结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return 0, baseErr
	}
	// transaction 原子包含条目主表与全部确定性分块。
	transaction, beginErr := store.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return 0, beginErr
	}
	defer transaction.Rollback()
	// entryID 是新 FAQ 或文档的持久化主键。
	var entryID int64
	// insertErr 是新条目写入默认待审核状态的错误。
	var insertErr error
	if draft.Type == "faq" {
		entryID, insertErr = insertReturningID(ctx, transaction, store.Dialect, `INSERT INTO knowledge_faqs(knowledge_base_id,question,answer,status,review_status,risk_level,requires_live_data,allow_auto_reply,enabled,effective_from,effective_to,created_at,updated_at) VALUES(?,?,?,'draft','pending',?, ?,0,0,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, knowledgeBaseID, draft.Title, draft.Content, draft.RiskLevel, boolToInt(draft.RequiresLiveData), draft.EffectiveFrom, draft.EffectiveTo)
	} else {
		entryID, insertErr = insertReturningID(ctx, transaction, store.Dialect, `INSERT INTO knowledge_documents(knowledge_base_id,title,content,content_type,status,review_status,risk_level,requires_live_data,allow_auto_reply,enabled,effective_from,effective_to,content_hash,created_at,updated_at) VALUES(?,?,?,?,'draft','pending',?, ?,0,0,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, knowledgeBaseID, draft.Title, draft.Content, draft.ContentType, draft.RiskLevel, boolToInt(draft.RequiresLiveData), draft.EffectiveFrom, draft.EffectiveTo, draft.ContentHash)
	}
	if insertErr != nil {
		return 0, insertErr
	}
	// chunkErr 是新条目确定性分块写入失败原因。
	if chunkErr := store.replaceEntryChunks(ctx, transaction, knowledgeBaseID, draft.Type, entryID, chunks); chunkErr != nil {
		return 0, chunkErr
	}
	// commitErr 是新条目主表与分块事务提交失败原因。
	if commitErr := transaction.Commit(); commitErr != nil {
		return 0, commitErr
	}
	return entryID, nil
}

// UpdateEntry 原子更新条目内容、重置审核／启用状态并替换分块。
func (store *KnowledgeStore) UpdateEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, draft KnowledgeEntryDraftRow, chunks []KnowledgeChunkRow) error {
	// baseErr 是更新条目前的用户与知识库归属复核结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return baseErr
	}
	// transaction 原子包含内容更新和分块替换。
	transaction, beginErr := store.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return beginErr
	}
	defer transaction.Rollback()
	// result 是当前条目主表的待审核重置结果。
	var result sql.Result
	// updateErr 是更新 FAQ 或文档主表的错误。
	var updateErr error
	if draft.Type == "faq" {
		result, updateErr = transaction.ExecContext(ctx, `UPDATE knowledge_faqs SET question=?,answer=?,status='draft',review_status='pending',risk_level=?,requires_live_data=?,allow_auto_reply=0,enabled=0,effective_from=?,effective_to=?,updated_at=CURRENT_TIMESTAMP WHERE knowledge_base_id=? AND id=?`, draft.Title, draft.Content, draft.RiskLevel, boolToInt(draft.RequiresLiveData), draft.EffectiveFrom, draft.EffectiveTo, knowledgeBaseID, entryID)
	} else {
		result, updateErr = transaction.ExecContext(ctx, `UPDATE knowledge_documents SET title=?,content=?,content_type=?,status='draft',review_status='pending',risk_level=?,requires_live_data=?,allow_auto_reply=0,enabled=0,effective_from=?,effective_to=?,content_hash=?,updated_at=CURRENT_TIMESTAMP WHERE knowledge_base_id=? AND id=?`, draft.Title, draft.Content, draft.ContentType, draft.RiskLevel, boolToInt(draft.RequiresLiveData), draft.EffectiveFrom, draft.EffectiveTo, draft.ContentHash, knowledgeBaseID, entryID)
	}
	if updateErr != nil {
		return updateErr
	}
	// affectedErr 是条目更新零行或行数读取失败的结果。
	if affectedErr := requireKnowledgeRowsAffected(result); affectedErr != nil {
		return affectedErr
	}
	// chunkErr 是更新条目确定性分块替换失败原因。
	if chunkErr := store.replaceEntryChunks(ctx, transaction, knowledgeBaseID, draft.Type, entryID, chunks); chunkErr != nil {
		return chunkErr
	}
	return transaction.Commit()
}

// ReviewEntry 将人工审核结果写入 FAQ 或文档；驳回时同时关闭检索。
func (store *KnowledgeStore) ReviewEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType string, reviewed bool) error {
	// baseErr 是审核条目前的用户与知识库归属复核结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return baseErr
	}
	// table 是经应用校验后确定的 FAQ 或文档表名。
	table := knowledgeEntryTable(contentType)
	// reviewStatus 是 pending 或 reviewed。
	reviewStatus := "pending"
	// status 是审核驳回后的 draft 或通过后的 active。
	status := "draft"
	if reviewed {
		reviewStatus = "reviewed"
		status = "active"
	}
	// result 是当前条目审核状态的更新结果。
	result, updateErr := store.DB.ExecContext(ctx, `UPDATE `+table+` SET review_status=?,status=?,enabled=0,updated_at=CURRENT_TIMESTAMP WHERE knowledge_base_id=? AND id=?`, reviewStatus, status, knowledgeBaseID, entryID)
	if updateErr != nil {
		return updateErr
	}
	return requireKnowledgeRowsAffected(result)
}

// SetEntryEnabled 切换单个已审核条目的离线检索状态。
func (store *KnowledgeStore) SetEntryEnabled(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType string, enabled bool) error {
	// baseErr 是切换条目启停前的用户与知识库归属复核结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return baseErr
	}
	// table 是经应用校验后确定的 FAQ 或文档表名。
	table := knowledgeEntryTable(contentType)
	// result 是单条知识启停更新结果。
	result, updateErr := store.DB.ExecContext(ctx, `UPDATE `+table+` SET enabled=?,updated_at=CURRENT_TIMESTAMP WHERE knowledge_base_id=? AND id=?`, boolToInt(enabled), knowledgeBaseID, entryID)
	if updateErr != nil {
		return updateErr
	}
	return requireKnowledgeRowsAffected(result)
}

// DeleteEntry 在单一事务中删除条目分块和 FAQ／文档主表。
func (store *KnowledgeStore) DeleteEntry(ctx context.Context, userID, knowledgeBaseID, entryID int64, contentType string) error {
	// baseErr 是删除条目前的用户与知识库归属复核结果。
	if _, baseErr := store.GetBase(ctx, userID, knowledgeBaseID); baseErr != nil {
		return baseErr
	}
	// transaction 原子包含分块和主表删除。
	transaction, beginErr := store.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return beginErr
	}
	defer transaction.Rollback()
	// chunkErr 是删除条目从属分块的失败原因。
	if _, chunkErr := transaction.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE knowledge_base_id=? AND source_type=? AND source_id=?`, knowledgeBaseID, contentType, entryID); chunkErr != nil {
		return chunkErr
	}
	// result 是单个 FAQ 或文档主表删除结果。
	result, deleteErr := transaction.ExecContext(ctx, `DELETE FROM `+knowledgeEntryTable(contentType)+` WHERE knowledge_base_id=? AND id=?`, knowledgeBaseID, entryID)
	if deleteErr != nil {
		return deleteErr
	}
	// affectedErr 是条目删除零行或行数读取失败的结果。
	if affectedErr := requireKnowledgeRowsAffected(result); affectedErr != nil {
		return affectedErr
	}
	return transaction.Commit()
}

// replaceEntryChunks 替换单个 FAQ 或文档的全部确定性分块。
func (store *KnowledgeStore) replaceEntryChunks(ctx context.Context, transaction *sql.Tx, knowledgeBaseID int64, sourceType string, sourceID int64, chunks []KnowledgeChunkRow) error {
	// deleteErr 是清理源条目旧分块的失败原因。
	if _, deleteErr := transaction.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE knowledge_base_id=? AND source_type=? AND source_id=?`, knowledgeBaseID, sourceType, sourceID); deleteErr != nil {
		return deleteErr
	}
	// chunk 是当前待写入的应用分块。
	for _, chunk := range chunks {
		// insertErr 是当前确定性文本分块写入失败原因。
		if _, insertErr := transaction.ExecContext(ctx, `INSERT INTO knowledge_chunks(knowledge_base_id,source_type,source_id,chunk_index,content,content_hash,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, knowledgeBaseID, sourceType, sourceID, chunk.Index, chunk.Content, chunk.ContentHash); insertErr != nil {
			return insertErr
		}
	}
	return nil
}

// knowledgeEntryTable 仅对经应用校验的 document 选择文档表，其他值回退到 FAQ 表。
func knowledgeEntryTable(contentType string) string {
	if contentType == "document" {
		return "knowledge_documents"
	}
	return "knowledge_faqs"
}

// knowledgeRowScanner 抽象 *sql.Rows 与 *sql.Row 共用的条目扫描能力。
type knowledgeRowScanner interface {
	// Scan 按查询列顺序写入持久化模型字段。
	Scan(dest ...any) error
}

// scanKnowledgeEntry 将 FAQ 和文档对齐查询扫描为统一持久化模型。
func scanKnowledgeEntry(scanner knowledgeRowScanner, row *KnowledgeEntryRow) error {
	return scanner.Scan(&row.ID, &row.KnowledgeBaseID, &row.Type, &row.Title, &row.Content, &row.ContentType, &row.Status, &row.ReviewStatus, &row.RiskLevel, &row.RequiresLiveData, &row.AllowAutoReply, &row.Enabled, &row.EffectiveFrom, &row.EffectiveTo, &row.CreatedAt, &row.UpdatedAt)
}
