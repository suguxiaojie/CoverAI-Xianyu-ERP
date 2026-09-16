package db

import (
	"context"
	"fmt"
	"strings"
)

// ListRetrievableEntries 返回当前用户已启用知识库中当前生效的已审核 FAQ 和文档。
func (store *KnowledgeStore) ListRetrievableEntries(ctx context.Context, userID int64, knowledgeBaseIDs []int64, nowUnix int64) ([]KnowledgeEntryRow, error) {
	// baseFilter 是可选的用户显式调试知识库过滤子句。
	baseFilter := ""
	// baseArgs 是知识库 ID 过滤所需的参数。
	baseArgs := make([]any, 0, len(knowledgeBaseIDs))
	if len(knowledgeBaseIDs) > 0 {
		// placeholders 是数量与知识库 ID 完全一致的参数化占位符。
		placeholders := make([]string, len(knowledgeBaseIDs))
		// index 是当前知识库 ID 的占位符与参数下标。
		for index := range knowledgeBaseIDs {
			placeholders[index] = "?"
			baseArgs = append(baseArgs, knowledgeBaseIDs[index])
		}
		baseFilter = " AND b.id IN (" + strings.Join(placeholders, ",") + ")"
	}
	// result 是 FAQ 和文档合并后的已审核检索候选。
	result := make([]KnowledgeEntryRow, 0)
	// faqQuery 只选择用户所属 active 知识库中当前生效的启用 FAQ。
	faqQuery := `SELECT f.id,f.knowledge_base_id,b.name,'faq',f.question,f.answer,'faq',f.status,f.review_status,f.risk_level,f.requires_live_data,f.allow_auto_reply,f.enabled,f.effective_from,f.effective_to,f.created_at,f.updated_at FROM knowledge_faqs f JOIN knowledge_bases b ON b.id=f.knowledge_base_id WHERE b.user_id=? AND b.status='active' AND f.status='active' AND f.review_status='reviewed' AND f.enabled=1 AND (f.effective_from=0 OR f.effective_from<=?) AND (f.effective_to=0 OR f.effective_to>?)` + baseFilter + ` ORDER BY f.updated_at DESC,f.id DESC`
	// faqArgs 是 FAQ 查询的用户、当前时间和可选知识库 ID 参数。
	faqArgs := append([]any{userID, nowUnix, nowUnix}, baseArgs...)
	// faqRows 是已审核 FAQ 的查询游标。
	faqRows, faqErr := store.DB.QueryContext(ctx, faqQuery, faqArgs...)
	if faqErr != nil {
		return nil, faqErr
	}
	// faqRows.Next 每次推进到一条当前可检索 FAQ。
	for faqRows.Next() {
		// row 是当前待扫描的 FAQ 候选。
		var row KnowledgeEntryRow
		// scanErr 是当前 FAQ 检索候选扫描失败原因。
		if scanErr := scanKnowledgeRetrieveEntry(faqRows, &row); scanErr != nil {
			faqRows.Close()
			return nil, scanErr
		}
		result = append(result, row)
	}
	// faqRowsErr 是 FAQ 检索候选游标遍历期间的底层错误。
	if faqRowsErr := faqRows.Err(); faqRowsErr != nil {
		faqRows.Close()
		return nil, faqRowsErr
	}
	faqRows.Close()
	// documentQuery 只选择用户所属 active 知识库中当前生效的启用文档。
	documentQuery := `SELECT d.id,d.knowledge_base_id,b.name,'document',d.title,d.content,d.content_type,d.status,d.review_status,d.risk_level,d.requires_live_data,d.allow_auto_reply,d.enabled,d.effective_from,d.effective_to,d.created_at,d.updated_at FROM knowledge_documents d JOIN knowledge_bases b ON b.id=d.knowledge_base_id WHERE b.user_id=? AND b.status='active' AND d.status='active' AND d.review_status='reviewed' AND d.enabled=1 AND (d.effective_from=0 OR d.effective_from<=?) AND (d.effective_to=0 OR d.effective_to>?)` + baseFilter + ` ORDER BY d.updated_at DESC,d.id DESC`
	// documentArgs 是文档查询的用户、当前时间和可选知识库 ID 参数。
	documentArgs := append([]any{userID, nowUnix, nowUnix}, baseArgs...)
	// documentRows 是已审核文档的查询游标。
	documentRows, documentErr := store.DB.QueryContext(ctx, documentQuery, documentArgs...)
	if documentErr != nil {
		return nil, documentErr
	}
	// documentRows.Next 每次推进到一份当前可检索文档。
	for documentRows.Next() {
		// row 是当前待扫描的文档候选。
		var row KnowledgeEntryRow
		// scanErr 是当前文档检索候选扫描失败原因。
		if scanErr := scanKnowledgeRetrieveEntry(documentRows, &row); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, row)
	}
	// documentRowsErr 是文档候选遍历期间的数据库错误。
	documentRowsErr := documentRows.Err()
	documentRows.Close()
	if documentRowsErr != nil {
		return nil, documentRowsErr
	}
	// aliasErr 是为 FAQ 候选批量补齐用户确认相似问法的失败原因。
	if aliasErr := store.loadRetrievableAliases(ctx, result); aliasErr != nil {
		return nil, aliasErr
	}
	return result, nil
}

// AddRetrieveLog 只持久化问题 SHA-256 摘要、门禁结论和证据数，不保存原始问题。
func (store *KnowledgeStore) AddRetrieveLog(ctx context.Context, userID int64, queryDigest, answerability string, evidenceCount int) error {
	if len(queryDigest) != 64 {
		return fmt.Errorf("知识检索问题摘要无效")
	}
	// insertErr 是脱敏检索日志写入错误。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO knowledge_retrieve_logs(user_id,query_digest,answerability,evidence_count,created_at) VALUES(?,?,?,?,CURRENT_TIMESTAMP)`, userID, queryDigest, answerability, evidenceCount)
	return insertErr
}

// scanKnowledgeRetrieveEntry 扫描带知识库名称的 FAQ／文档检索候选。
func scanKnowledgeRetrieveEntry(scanner knowledgeRowScanner, row *KnowledgeEntryRow) error {
	return scanner.Scan(&row.ID, &row.KnowledgeBaseID, &row.KnowledgeBaseName, &row.Type, &row.Title, &row.Content, &row.ContentType, &row.Status, &row.ReviewStatus, &row.RiskLevel, &row.RequiresLiveData, &row.AllowAutoReply, &row.Enabled, &row.EffectiveFrom, &row.EffectiveTo, &row.CreatedAt, &row.UpdatedAt)
}
