package db

import (
	"context"
	"database/sql"
	"errors"
)

// ListBases 返回当前用户的知识库、范围和条目审核计数。
func (store *KnowledgeStore) ListBases(ctx context.Context, userID int64) ([]KnowledgeBaseRow, error) {
	// rows 是当前用户的知识库摘要查询游标。
	rows, queryErr := store.DB.QueryContext(ctx, `
		SELECT b.id,b.user_id,b.name,b.description,b.status,b.created_at,b.updated_at,
		       (SELECT COUNT(*) FROM knowledge_faqs f WHERE f.knowledge_base_id=b.id)+
		       (SELECT COUNT(*) FROM knowledge_documents d WHERE d.knowledge_base_id=b.id),
		       (SELECT COUNT(*) FROM knowledge_faqs f WHERE f.knowledge_base_id=b.id AND f.review_status='reviewed')+
		       (SELECT COUNT(*) FROM knowledge_documents d WHERE d.knowledge_base_id=b.id AND d.review_status='reviewed')
		FROM knowledge_bases b WHERE b.user_id=? ORDER BY b.updated_at DESC,b.id DESC`, userID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// result 是将被交给 Adapter 的知识库持久化模型列表。
	result := make([]KnowledgeBaseRow, 0)
	// rows.Next 每次推进到当前用户的一个知识库。
	for rows.Next() {
		// row 是当前待扫描的知识库持久化模型。
		var row KnowledgeBaseRow
		// scanErr 是当前知识库摘要扫描失败原因。
		if scanErr := rows.Scan(&row.ID, &row.UserID, &row.Name, &row.Description, &row.Status, &row.CreatedAt, &row.UpdatedAt, &row.EntryCount, &row.ReviewedCount); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, row)
	}
	// rowsErr 是知识库列表游标遍历期间的底层错误。
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	// index 是当前需要补齐店铺／商品范围的知识库下标。
	for index := range result {
		// scopeErr 是当前知识库店铺／商品范围补齐失败原因。
		if scopeErr := store.loadBaseScopes(ctx, &result[index]); scopeErr != nil {
			return nil, scopeErr
		}
	}
	return result, nil
}

// GetBase 返回当前用户拥有的单个知识库。
func (store *KnowledgeStore) GetBase(ctx context.Context, userID, knowledgeBaseID int64) (KnowledgeBaseRow, error) {
	// row 是当前用户范围内读取的知识库持久化模型。
	var row KnowledgeBaseRow
	// queryErr 是按用户与知识库 ID 读取摘要的错误。
	queryErr := store.DB.QueryRowContext(ctx, `
		SELECT b.id,b.user_id,b.name,b.description,b.status,b.created_at,b.updated_at,
		       (SELECT COUNT(*) FROM knowledge_faqs f WHERE f.knowledge_base_id=b.id)+
		       (SELECT COUNT(*) FROM knowledge_documents d WHERE d.knowledge_base_id=b.id),
		       (SELECT COUNT(*) FROM knowledge_faqs f WHERE f.knowledge_base_id=b.id AND f.review_status='reviewed')+
		       (SELECT COUNT(*) FROM knowledge_documents d WHERE d.knowledge_base_id=b.id AND d.review_status='reviewed')
		FROM knowledge_bases b WHERE b.user_id=? AND b.id=?`, userID, knowledgeBaseID).Scan(
		&row.ID, &row.UserID, &row.Name, &row.Description, &row.Status, &row.CreatedAt, &row.UpdatedAt, &row.EntryCount, &row.ReviewedCount)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return KnowledgeBaseRow{}, ErrNotFound
	}
	if queryErr != nil {
		return KnowledgeBaseRow{}, queryErr
	}
	// scopeErr 是单知识库店铺／商品范围补齐失败原因。
	if scopeErr := store.loadBaseScopes(ctx, &row); scopeErr != nil {
		return KnowledgeBaseRow{}, scopeErr
	}
	return row, nil
}

// CreateBase 使用单一事务创建草稿知识库并保存显式店铺／商品范围。
func (store *KnowledgeStore) CreateBase(ctx context.Context, userID int64, draft KnowledgeBaseDraftRow) (int64, error) {
	// transaction 原子包含知识库主表和全部范围绑定。
	transaction, beginErr := store.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return 0, beginErr
	}
	defer transaction.Rollback()
	// scopeErr 是创建前的店铺／商品用户归属校验结果。
	if scopeErr := store.validateKnowledgeScopes(ctx, transaction, userID, draft); scopeErr != nil {
		return 0, scopeErr
	}
	// knowledgeBaseID 是刚创建的草稿知识库主键。
	knowledgeBaseID, insertErr := insertReturningID(ctx, transaction, store.Dialect, `INSERT INTO knowledge_bases(user_id,name,description,status,created_at,updated_at) VALUES(?,?,?,'draft',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, userID, draft.Name, draft.Description)
	if insertErr != nil {
		return 0, insertErr
	}
	// scopeErr 是新知识库范围原子写入失败原因。
	if scopeErr := store.replaceBaseScopes(ctx, transaction, knowledgeBaseID, draft); scopeErr != nil {
		return 0, scopeErr
	}
	// commitErr 是新知识库主表与范围事务提交失败原因。
	if commitErr := transaction.Commit(); commitErr != nil {
		return 0, commitErr
	}
	return knowledgeBaseID, nil
}

// UpdateBase 使用单一事务更新知识库文案并替换范围。
func (store *KnowledgeStore) UpdateBase(ctx context.Context, userID, knowledgeBaseID int64, draft KnowledgeBaseDraftRow) error {
	// transaction 原子包含主表更新与范围替换。
	transaction, beginErr := store.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return beginErr
	}
	defer transaction.Rollback()
	// scopeErr 是更新前的店铺／商品用户归属校验结果。
	if scopeErr := store.validateKnowledgeScopes(ctx, transaction, userID, draft); scopeErr != nil {
		return scopeErr
	}
	// result 记录用户范围内知识库主表的更新结果。
	result, updateErr := transaction.ExecContext(ctx, `UPDATE knowledge_bases SET name=?,description=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=?`, draft.Name, draft.Description, knowledgeBaseID, userID)
	if updateErr != nil {
		return updateErr
	}
	// affected 是当前用户实际更新的知识库行数。
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return affectedErr
	}
	if affected == 0 {
		return ErrNotFound
	}
	// scopeErr 是更新知识库时的范围原子替换失败原因。
	if scopeErr := store.replaceBaseScopes(ctx, transaction, knowledgeBaseID, draft); scopeErr != nil {
		return scopeErr
	}
	return transaction.Commit()
}

// SetBaseStatus 在当前用户范围内切换知识库草稿／启用状态。
func (store *KnowledgeStore) SetBaseStatus(ctx context.Context, userID, knowledgeBaseID int64, status string) error {
	// result 是按用户与主键限定的状态更新结果。
	result, updateErr := store.DB.ExecContext(ctx, `UPDATE knowledge_bases SET status=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=?`, status, knowledgeBaseID, userID)
	if updateErr != nil {
		return updateErr
	}
	return requireKnowledgeRowsAffected(result)
}

// DeleteBase 删除当前用户拥有的知识库，从属表由外键级联删除。
func (store *KnowledgeStore) DeleteBase(ctx context.Context, userID, knowledgeBaseID int64) error {
	// result 是按用户与主键限定的删除结果。
	result, deleteErr := store.DB.ExecContext(ctx, `DELETE FROM knowledge_bases WHERE id=? AND user_id=?`, knowledgeBaseID, userID)
	if deleteErr != nil {
		return deleteErr
	}
	return requireKnowledgeRowsAffected(result)
}

// loadBaseScopes 为已完成用户隔离的知识库补齐店铺和商品范围。
func (store *KnowledgeStore) loadBaseScopes(ctx context.Context, row *KnowledgeBaseRow) error {
	// accountRows 是当前知识库的店铺绑定游标。
	accountRows, accountErr := store.DB.QueryContext(ctx, `SELECT cookie_id FROM knowledge_base_accounts WHERE knowledge_base_id=? ORDER BY cookie_id`, row.ID)
	if accountErr != nil {
		return accountErr
	}
	defer accountRows.Close()
	// accountRows.Next 每次推进到一个绑定店铺。
	for accountRows.Next() {
		// cookieID 是当前待加入范围的闲鱼账号标识。
		var cookieID string
		// scanErr 是当前店铺范围标识扫描失败原因。
		if scanErr := accountRows.Scan(&cookieID); scanErr != nil {
			return scanErr
		}
		row.CookieIDs = append(row.CookieIDs, cookieID)
	}
	// accountRowsErr 是店铺范围游标遍历期间的底层错误。
	if accountRowsErr := accountRows.Err(); accountRowsErr != nil {
		return accountRowsErr
	}
	// itemRows 是当前知识库的店铺商品绑定游标。
	itemRows, itemErr := store.DB.QueryContext(ctx, `SELECT cookie_id,item_id FROM knowledge_base_items WHERE knowledge_base_id=? ORDER BY cookie_id,item_id`, row.ID)
	if itemErr != nil {
		return itemErr
	}
	defer itemRows.Close()
	// itemRows.Next 每次推进到一个绑定商品。
	for itemRows.Next() {
		// scope 是当前待加入范围的店铺商品。
		var scope KnowledgeItemScopeRow
		// scanErr 是当前店铺商品范围扫描失败原因。
		if scanErr := itemRows.Scan(&scope.CookieID, &scope.ItemID); scanErr != nil {
			return scanErr
		}
		row.ItemScopes = append(row.ItemScopes, scope)
	}
	return itemRows.Err()
}

// validateKnowledgeScopes 确认所有店铺和商品都属于当前用户。
func (store *KnowledgeStore) validateKnowledgeScopes(ctx context.Context, transaction *sql.Tx, userID int64, draft KnowledgeBaseDraftRow) error {
	// cookieID 是当前待校验归属的店铺账号。
	for _, cookieID := range draft.CookieIDs {
		// exists 是当前店铺是否属于用户的数据库布尔结果。
		var exists bool
		// queryErr 是按用户与店铺标识执行归属查询的失败原因。
		if queryErr := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cookies WHERE id=? AND user_id=?)`, cookieID, userID).Scan(&exists); queryErr != nil {
			return queryErr
		}
		if !exists {
			return ErrForbidden
		}
	}
	// scope 是当前待校验归属的店铺商品。
	for _, scope := range draft.ItemScopes {
		// exists 是当前商品是否属于用户账号的数据库布尔结果。
		var exists bool
		// queryErr 是按用户、店铺和商品执行归属查询的失败原因。
		if queryErr := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM item_info i JOIN cookies c ON c.id=i.cookie_id WHERE i.cookie_id=? AND i.item_id=? AND c.user_id=?)`, scope.CookieID, scope.ItemID, userID).Scan(&exists); queryErr != nil {
			return queryErr
		}
		if !exists {
			return ErrForbidden
		}
	}
	return nil
}

// replaceBaseScopes 在知识库主表事务内替换全部店铺和商品范围。
func (store *KnowledgeStore) replaceBaseScopes(ctx context.Context, transaction *sql.Tx, knowledgeBaseID int64, draft KnowledgeBaseDraftRow) error {
	// deleteErr 是清理旧店铺范围失败原因，失败时整个事务回滚。
	if _, deleteErr := transaction.ExecContext(ctx, `DELETE FROM knowledge_base_accounts WHERE knowledge_base_id=?`, knowledgeBaseID); deleteErr != nil {
		return deleteErr
	}
	// deleteErr 是清理旧商品范围失败原因，失败时整个事务回滚。
	if _, deleteErr := transaction.ExecContext(ctx, `DELETE FROM knowledge_base_items WHERE knowledge_base_id=?`, knowledgeBaseID); deleteErr != nil {
		return deleteErr
	}
	// cookieID 是当前待写入的店铺账号范围。
	for _, cookieID := range draft.CookieIDs {
		// insertErr 是当前店铺范围写入失败原因。
		if _, insertErr := transaction.ExecContext(ctx, `INSERT INTO knowledge_base_accounts(knowledge_base_id,cookie_id) VALUES(?,?)`, knowledgeBaseID, cookieID); insertErr != nil {
			return insertErr
		}
	}
	// scope 是当前待写入的店铺商品范围。
	for _, scope := range draft.ItemScopes {
		// insertErr 是当前店铺商品范围写入失败原因。
		if _, insertErr := transaction.ExecContext(ctx, `INSERT INTO knowledge_base_items(knowledge_base_id,cookie_id,item_id) VALUES(?,?,?)`, knowledgeBaseID, scope.CookieID, scope.ItemID); insertErr != nil {
			return insertErr
		}
	}
	return nil
}

// requireKnowledgeRowsAffected 将零行更新／删除转换为稳定的不存在错误。
func requireKnowledgeRowsAffected(result sql.Result) error {
	// affected 是当前操作实际变更的知识资源行数。
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return affectedErr
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
