package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ListSKUs 返回指定商品当前未逻辑删除的 SKU，并包含本地单件成本。
func (items *Items) ListSKUs(ctx context.Context, cookieID, itemID string) ([]ItemSKURow, error) {
	if items == nil || items.DB == nil {
		return nil, errors.New("商品 SKU 存储未初始化")
	}
	// rows 是按平台顺序读取的 SKU 查询游标。
	rows, queryErr := items.DB.QueryContext(ctx, `SELECT id,cookie_id,item_id,sku_id,inventory_id,properties_json,
		price_cents,quantity,initial_quantity,enabled,sort_order,cost_cents,synced_at
		FROM item_skus WHERE cookie_id=? AND item_id=? AND deleted_at IS NULL ORDER BY sort_order,id`, cookieID, itemID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	return scanItemSKURows(rows)
}

// ListSKUsForCookie 返回指定账号全部当前 SKU，供账号范围商品列表一次装配。
func (items *Items) ListSKUsForCookie(ctx context.Context, cookieID string) ([]ItemSKURow, error) {
	if items == nil || items.DB == nil {
		return nil, errors.New("商品 SKU 存储未初始化")
	}
	// rows 是按商品和平台顺序读取的账号 SKU 查询游标。
	rows, queryErr := items.DB.QueryContext(ctx, `SELECT s.id,s.cookie_id,s.item_id,s.sku_id,s.inventory_id,s.properties_json,
		s.price_cents,s.quantity,s.initial_quantity,s.enabled,s.sort_order,s.cost_cents,s.synced_at
		FROM item_skus s JOIN item_info i ON i.cookie_id=s.cookie_id AND i.item_id=s.item_id
		WHERE s.cookie_id=? AND s.deleted_at IS NULL AND i.deleted_at IS NULL ORDER BY s.item_id,s.sort_order,s.id`, cookieID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	return scanItemSKURows(rows)
}

// ListSKUsForUser 一次读取用户范围内的当前 SKU，供商品列表避免逐商品查询。
func (items *Items) ListSKUsForUser(ctx context.Context, userID int64, cookieID string) ([]ItemSKURow, error) {
	if items == nil || items.DB == nil {
		return nil, errors.New("商品 SKU 存储未初始化")
	}
	// rows 是经账号归属过滤的 SKU 查询游标。
	rows, queryErr := items.DB.QueryContext(ctx, `SELECT s.id,s.cookie_id,s.item_id,s.sku_id,s.inventory_id,s.properties_json,
		s.price_cents,s.quantity,s.initial_quantity,s.enabled,s.sort_order,s.cost_cents,s.synced_at
		FROM item_skus s JOIN cookies c ON c.id=s.cookie_id JOIN item_info i ON i.cookie_id=s.cookie_id AND i.item_id=s.item_id
		WHERE c.user_id=? AND (?='' OR s.cookie_id=?) AND s.deleted_at IS NULL AND i.deleted_at IS NULL
		ORDER BY s.cookie_id,s.item_id,s.sort_order,s.id`, userID, cookieID, cookieID)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	return scanItemSKURows(rows)
}

// CountCostedPlatformSKUs 返回商品当前已配置成本且有效的平台 SKU 数量，不把本地默认行算作多规格。
func (items *Items) CountCostedPlatformSKUs(ctx context.Context, cookieID, itemID string) (int, error) {
	if items == nil || items.DB == nil {
		return 0, errors.New("商品 SKU 存储未初始化")
	}
	// count 是当前商品可用于规格精确匹配的平台 SKU 数量。
	var count int
	// queryErr 是 SKU 成本数量查询错误。
	queryErr := items.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_skus WHERE cookie_id=? AND item_id=? AND sku_id<>? AND deleted_at IS NULL AND cost_cents IS NOT NULL`, cookieID, itemID, DefaultItemSKUID).Scan(&count)
	return count, queryErr
}

// scanItemSKURows 将数据库查询游标转换为 SKU 行模型，并保留成本空值语义。
func scanItemSKURows(rows *sql.Rows) ([]ItemSKURow, error) {
	// result 保存按查询顺序返回的 SKU。
	result := make([]ItemSKURow, 0)
	for rows.Next() {
		// row 是当前待扫描的 SKU 行。
		var row ItemSKURow
		// enabled 是跨方言使用的整数布尔值。
		var enabled int
		// costCents 保留“未填写”和零成本的区别。
		var costCents sql.NullInt64
		// scanErr 是当前 SKU 行字段读取错误。
		if scanErr := rows.Scan(&row.ID, &row.CookieID, &row.ItemID, &row.SKUID, &row.InventoryID, &row.PropertiesJSON,
			&row.PriceCents, &row.Quantity, &row.InitialQuantity, &enabled, &row.SortOrder, &costCents, &row.SyncedAt); scanErr != nil {
			return nil, scanErr
		}
		row.Enabled = enabled != 0
		if costCents.Valid {
			// value 是当前 SKU 已显式填写的本地单件成本。
			value := costCents.Int64
			row.CostCents = &value
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// SyncSKUs 保存一个商品的权威 SKU 快照；平台字段更新时明确保留 cost_cents。
func (items *Items) SyncSKUs(ctx context.Context, cookieID, itemID string, rows []ItemSKURow) error {
	if items == nil || items.DB == nil {
		return errors.New("商品 SKU 存储未初始化")
	}
	// transaction 保证同一商品 SKU 的新增、更新和逻辑删除一次提交。
	transaction, beginErr := items.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return beginErr
	}
	// syncErr 是当前商品 SKU 事务内对账错误。
	if syncErr := items.syncSKUsTx(ctx, transaction, cookieID, itemID, rows); syncErr != nil {
		_ = transaction.Rollback()
		return syncErr
	}
	return transaction.Commit()
}

// syncSKUsTx 在调用方事务中对账一个商品的 SKU，绝不改写本地成本。
func (items *Items) syncSKUsTx(ctx context.Context, transaction *sql.Tx, cookieID, itemID string, rows []ItemSKURow) error {
	// remoteIDs 保存有效且去重后的远端 SKU 标识。
	remoteIDs := make(map[string]struct{}, len(rows))
	// rowIndex 表示当前待写入 SKU 在远端快照中的下标。
	for rowIndex := range rows {
		// row 是当前待写入的远端 SKU 快照。
		row := rows[rowIndex]
		row.CookieID = strings.TrimSpace(cookieID)
		row.ItemID = strings.TrimSpace(itemID)
		row.SKUID = strings.TrimSpace(row.SKUID)
		if row.CookieID == "" || row.ItemID == "" || row.SKUID == "" {
			continue
		}
		// duplicated 表示同一远端快照是否已经包含当前 SKU。
		if _, duplicated := remoteIDs[row.SKUID]; duplicated {
			continue
		}
		remoteIDs[row.SKUID] = struct{}{}
		// propertiesJSON 保证空规格仍保存有效 JSON 数组。
		propertiesJSON := strings.TrimSpace(row.PropertiesJSON)
		if propertiesJSON == "" {
			propertiesJSON = "[]"
		}
		// upsertQuery 只更新平台字段，故意不包含 cost_cents。
		upsertQuery := `INSERT INTO item_skus(cookie_id,item_id,sku_id,inventory_id,properties_json,price_cents,quantity,
			initial_quantity,enabled,sort_order,synced_at,updated_at,deleted_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,NULL)` + dialectUpsert(items.Dialect, []string{"cookie_id", "item_id", "sku_id"}, map[string]string{
			"inventory_id": "EXCLUDED.inventory_id", "properties_json": "EXCLUDED.properties_json",
			"price_cents": "EXCLUDED.price_cents", "quantity": "EXCLUDED.quantity", "initial_quantity": "EXCLUDED.initial_quantity",
			"enabled": "EXCLUDED.enabled", "sort_order": "EXCLUDED.sort_order", "synced_at": "EXCLUDED.synced_at",
			"updated_at": "CURRENT_TIMESTAMP", "deleted_at": "NULL",
		})
		// upsertErr 是当前平台 SKU 字段写入错误。
		if _, upsertErr := transaction.ExecContext(ctx, upsertQuery, row.CookieID, row.ItemID, row.SKUID, row.InventoryID,
			propertiesJSON, row.PriceCents, row.Quantity, row.InitialQuantity, boolToInt(row.Enabled), row.SortOrder, row.SyncedAt); upsertErr != nil {
			return upsertErr
		}
	}
	// deleteQuery 逻辑删除本次成功快照中已消失的 SKU，同时保留其成本以便同一 SKU 恢复。
	deleteQuery := `UPDATE item_skus SET deleted_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE cookie_id=? AND item_id=? AND sku_id<>? AND deleted_at IS NULL`
	// deleteArguments 是逻辑删除语句的账号、商品和远端 SKU 参数。
	deleteArguments := []any{cookieID, itemID, DefaultItemSKUID}
	if len(remoteIDs) > 0 {
		// placeholders 是本次远端 SKU 集合的 SQL 占位符。
		placeholders := make([]string, 0, len(remoteIDs))
		// remoteID 表示当前加入逻辑删除保留集合的 SKU 标识。
		for remoteID := range remoteIDs {
			placeholders = append(placeholders, "?")
			deleteArguments = append(deleteArguments, remoteID)
		}
		deleteQuery += ` AND sku_id NOT IN (` + strings.Join(placeholders, ",") + `)`
	}
	// deleteErr 是已消失 SKU 逻辑删除的执行错误。
	_, deleteErr := transaction.ExecContext(ctx, deleteQuery, deleteArguments...)
	return deleteErr
}

// UpdateSKUCost 更新一个当前有效 SKU 的本地单件成本；nil 清除成本但不影响平台库存字段。
func (items *Items) UpdateSKUCost(ctx context.Context, cookieID, itemID, skuID string, costCents *int64) error {
	return items.UpdateSKUCosts(ctx, cookieID, itemID, []ItemSKUCostUpdate{{SKUID: skuID, CostCents: costCents}})
}

// UpdateSKUCosts 在一个事务中保存商品全部本地成本；单规格隐式 SKU 可按需创建，平台 SKU 必须已经存在。
func (items *Items) UpdateSKUCosts(ctx context.Context, cookieID, itemID string, updates []ItemSKUCostUpdate) error {
	if items == nil || items.DB == nil {
		return errors.New("商品 SKU 存储未初始化")
	}
	if len(updates) == 0 {
		return errors.New("SKU 成本更新不能为空")
	}
	// transaction 保证多规格成本全部成功或全部回滚。
	transaction, beginErr := items.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return beginErr
	}
	// exists 验证目标商品当前有效，避免隐式 SKU 脱离商品创建。
	var exists int
	// itemErr 是目标商品有效性查询错误。
	if itemErr := transaction.QueryRowContext(ctx, `SELECT 1 FROM item_info WHERE cookie_id=? AND item_id=? AND deleted_at IS NULL`, cookieID, itemID).Scan(&exists); itemErr != nil {
		_ = transaction.Rollback()
		if errors.Is(itemErr, sql.ErrNoRows) {
			return ErrNotFound
		}
		return itemErr
	}
	// update 表示当前待保存的 SKU 本地成本。
	for _, update := range updates {
		// skuID 是去除空白后的平台或隐式 SKU 标识。
		skuID := strings.TrimSpace(update.SKUID)
		if skuID == "" || update.CostCents != nil && *update.CostCents < 0 {
			_ = transaction.Rollback()
			return errors.New("SKU 成本参数无效")
		}
		if skuID == DefaultItemSKUID {
			// insertQuery 创建或恢复单规格商品的本地隐式 SKU，并且只更新成本字段。
			insertQuery := `INSERT INTO item_skus(cookie_id,item_id,sku_id,inventory_id,properties_json,price_cents,quantity,
				initial_quantity,enabled,sort_order,cost_cents,synced_at,updated_at,deleted_at)
				VALUES(?,?,?,'','[]',0,0,0,1,0,?,0,CURRENT_TIMESTAMP,NULL)` + dialectUpsert(items.Dialect, []string{"cookie_id", "item_id", "sku_id"}, map[string]string{
				"cost_cents": "EXCLUDED.cost_cents", "updated_at": "CURRENT_TIMESTAMP", "deleted_at": "NULL",
			})
			// insertErr 是隐式单规格成本行创建或恢复错误。
			if _, insertErr := transaction.ExecContext(ctx, insertQuery, cookieID, itemID, skuID, update.CostCents); insertErr != nil {
				_ = transaction.Rollback()
				return insertErr
			}
			continue
		}
		// result 是精确平台 SKU 本地成本更新结果。
		result, updateErr := transaction.ExecContext(ctx, `UPDATE item_skus SET cost_cents=?,updated_at=CURRENT_TIMESTAMP
			WHERE cookie_id=? AND item_id=? AND sku_id=? AND deleted_at IS NULL`, update.CostCents, cookieID, itemID, skuID)
		if updateErr != nil {
			_ = transaction.Rollback()
			return updateErr
		}
		// affected 是当前平台 SKU 成本更新命中的行数。
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil || affected == 0 {
			_ = transaction.Rollback()
			if rowsErr != nil {
				return rowsErr
			}
			return ErrNotFound
		}
	}
	return transaction.Commit()
}
