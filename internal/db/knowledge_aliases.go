package db

import (
	"context"
	"errors"
)

// ListFAQAliases 返回当前用户指定 FAQ 的全部相似问法。
func (store *KnowledgeStore) ListFAQAliases(ctx context.Context, userID, knowledgeBaseID, faqID int64) ([]KnowledgeFAQAliasRow, error) {
	// rows 是通过知识库用户归属和 FAQ 外键限定的相似问法查询游标。
	rows, queryErr := store.DB.QueryContext(ctx, `SELECT a.id,a.knowledge_base_id,a.faq_id,a.alias,a.normalized_alias,a.source,a.enabled,a.created_at,a.updated_at FROM knowledge_faq_aliases a JOIN knowledge_faqs f ON f.id=a.faq_id AND f.knowledge_base_id=a.knowledge_base_id JOIN knowledge_bases b ON b.id=a.knowledge_base_id WHERE b.user_id=? AND a.knowledge_base_id=? AND a.faq_id=? ORDER BY a.id`, userID, knowledgeBaseID, faqID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// result 是当前 FAQ 的相似问法持久化模型。
	result := make([]KnowledgeFAQAliasRow, 0)
	// rows.Next 每次推进到一条相似问法。
	for rows.Next() {
		// row 是当前待扫描的相似问法。
		var row KnowledgeFAQAliasRow
		// scanErr 是相似问法字段扫描失败原因。
		if scanErr := rows.Scan(&row.ID, &row.KnowledgeBaseID, &row.FAQID, &row.Alias, &row.NormalizedAlias, &row.Source, &row.Enabled, &row.CreatedAt, &row.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// FAQAliasConflict 判断同一知识库是否已有相同规范化问法，并保持 FAQ 用户归属校验。
func (store *KnowledgeStore) FAQAliasConflict(ctx context.Context, userID, knowledgeBaseID, faqID int64, normalizedAlias string) (bool, error) {
	// owned 表示 FAQ 是否属于当前用户指定知识库。
	var owned bool
	// ownershipErr 是 FAQ 用户归属读取失败原因。
	if ownershipErr := store.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_faqs f JOIN knowledge_bases b ON b.id=f.knowledge_base_id WHERE b.user_id=? AND f.knowledge_base_id=? AND f.id=?)`, userID, knowledgeBaseID, faqID).Scan(&owned); ownershipErr != nil {
		return false, ownershipErr
	}
	if !owned {
		return false, ErrNotFound
	}
	// conflict 表示相同知识库是否已经保存该规范化问法。
	var conflict bool
	// conflictErr 是规范化问法冲突查询失败原因。
	conflictErr := store.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM knowledge_faq_aliases WHERE knowledge_base_id=? AND normalized_alias=?)`, knowledgeBaseID, normalizedAlias).Scan(&conflict)
	return conflict, conflictErr
}

// CreateFAQAlias 创建用户确认的 FAQ 相似问法并返回主键。
func (store *KnowledgeStore) CreateFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID int64, alias, normalizedAlias, source string) (int64, error) {
	// conflict 是相似问法写入前的用户归属和唯一性检查结果。
	conflict, conflictErr := store.FAQAliasConflict(ctx, userID, knowledgeBaseID, faqID, normalizedAlias)
	if conflictErr != nil {
		return 0, conflictErr
	}
	if conflict {
		return 0, errors.New("相似问法已存在")
	}
	return insertReturningID(ctx, store.DB, store.Dialect, `INSERT INTO knowledge_faq_aliases(knowledge_base_id,faq_id,alias,normalized_alias,source,enabled,created_at,updated_at) VALUES(?,?,?,?,?,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, knowledgeBaseID, faqID, alias, normalizedAlias, source)
}

// DeleteFAQAlias 删除当前用户指定 FAQ 下的单条相似问法。
func (store *KnowledgeStore) DeleteFAQAlias(ctx context.Context, userID, knowledgeBaseID, faqID, aliasID int64) error {
	// result 是按用户、知识库、FAQ 和相似问法主键限定的删除结果。
	result, deleteErr := store.DB.ExecContext(ctx, `DELETE FROM knowledge_faq_aliases WHERE id=? AND knowledge_base_id=? AND faq_id=? AND EXISTS(SELECT 1 FROM knowledge_bases b WHERE b.id=? AND b.user_id=?)`, aliasID, knowledgeBaseID, faqID, knowledgeBaseID, userID)
	if deleteErr != nil {
		return deleteErr
	}
	return requireKnowledgeRowsAffected(result)
}

// loadRetrievableAliases 为已完成用户隔离的 FAQ 检索候选补齐启用相似问法。
func (store *KnowledgeStore) loadRetrievableAliases(ctx context.Context, entries []KnowledgeEntryRow) error {
	// index 是当前待补齐相似问法的检索候选下标。
	for index := range entries {
		if entries[index].Type != "faq" {
			continue
		}
		// rows 是当前 FAQ 启用相似问法查询游标。
		rows, queryErr := store.DB.QueryContext(ctx, `SELECT alias FROM knowledge_faq_aliases WHERE knowledge_base_id=? AND faq_id=? AND enabled=1 ORDER BY id`, entries[index].KnowledgeBaseID, entries[index].ID)
		if queryErr != nil {
			return queryErr
		}
		// rows.Next 每次推进到一条可参与检索的相似问法。
		for rows.Next() {
			// alias 是当前待加入 FAQ 候选的用户确认问法。
			var alias string
			// scanErr 是当前相似问法文本扫描失败原因。
			if scanErr := rows.Scan(&alias); scanErr != nil {
				rows.Close()
				return scanErr
			}
			entries[index].Aliases = append(entries[index].Aliases, alias)
		}
		// rowsErr 是当前相似问法游标遍历错误。
		rowsErr := rows.Err()
		rows.Close()
		if rowsErr != nil {
			return rowsErr
		}
	}
	return nil
}
