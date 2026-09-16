package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SaveGlobalGroup 在一个事务中删除用户全部账号的旧绑定，并把同一规则组复制到选定账号。
func (k *Keywords) SaveGlobalGroup(ctx context.Context, ownedCookieIDs, selectedCookieIDs []string, groupID string, keywords []string, reply, itemID, kwType, imageURL, matchType, messageScope, systemTypes string, enabled bool, intervalSeconds, delaySeconds int64) (string, error) {
	// existingEnabled 保存编辑现有规则时各店铺原有状态，避免内容保存抹平部分开启。
	existingEnabled := make(map[string]bool)
	if strings.TrimSpace(groupID) != "" {
		// stateRows、stateErr 是现有规则逐行店铺状态查询结果。
		stateRows, stateErr := k.DB.QueryContext(ctx, `SELECT cookie_id,enabled FROM keywords WHERE group_id=?`, groupID)
		if stateErr != nil {
			return "", stateErr
		}
		// cookieID、rowEnabled 是当前现有关键词行的店铺和状态。
		for stateRows.Next() {
			// cookieID、rowEnabled 是当前现有关键词行的店铺和状态。
			var cookieID string
			// rowEnabled 表示该关键词行是否参与店铺自动回复。
			var rowEnabled bool
			if // scanErr 是现有店铺状态行解析错误。
			scanErr := stateRows.Scan(&cookieID, &rowEnabled); scanErr != nil {
				stateRows.Close()
				return "", scanErr
			}
			existingEnabled[cookieID] = rowEnabled
		}
		if // stateErr 是关闭现有状态结果集时的错误。
		stateErr := stateRows.Close(); stateErr != nil {
			return "", stateErr
		}
	}
	if strings.TrimSpace(groupID) == "" {
		groupID = fmt.Sprintf("kwg-%d", time.Now().UnixNano())
	}
	if len(ownedCookieIDs) == 0 || len(selectedCookieIDs) == 0 {
		return "", fmt.Errorf("至少选择一个适用店铺")
	}
	if len(keywords) == 0 {
		keywords = []string{""}
	}
	// transaction 保证全部店铺绑定一次替换，任一店铺失败会恢复旧配置。
	transaction, beginErr := k.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return "", beginErr
	}
	defer transaction.Rollback()
	// placeholders 是用户全部账号删除旧组所需的占位符。
	placeholders := make([]string, len(ownedCookieIDs))
	// deleteArgs 是规则组标识和全部账号参数。
	deleteArgs := []any{groupID}
	// index、cookieID 是当前构造删除范围的账号下标和标识。
	for index, cookieID := range ownedCookieIDs {
		placeholders[index] = "?"
		deleteArgs = append(deleteArgs, cookieID)
	}
	if // deleteErr 是用户全部店铺旧规则绑定删除错误。
	_, deleteErr := transaction.ExecContext(ctx, `DELETE FROM keywords WHERE group_id=? AND cookie_id IN (`+strings.Join(placeholders, ",")+`)`, deleteArgs...); deleteErr != nil {
		return "", deleteErr
	}
	// cookieID 是当前写入规则组绑定的目标店铺账号。
	for _, cookieID := range selectedCookieIDs {
		// cookieEnabled 优先保留编辑前该店铺状态；新增绑定使用请求默认状态。
		cookieEnabled := enabled
		// previousEnabled、exists 是编辑前该店铺状态和存在标记。
		if previousEnabled, exists := existingEnabled[cookieID]; exists {
			cookieEnabled = previousEnabled
		}
		// keyword 是当前店铺写入的独立匹配词。
		for _, keyword := range keywords {
			if keyword != "" {
				// duplicate 保存当前店铺同商品范围是否已有其他组使用关键词。
				var duplicate int
				if // duplicateErr 是当前店铺跨组重复关键词检查错误。
				duplicateErr := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM keywords WHERE cookie_id=? AND COALESCE(item_id,'')=? AND LOWER(keyword)=LOWER(?) AND group_id<>?)`, cookieID, itemID, keyword, groupID).Scan(&duplicate); duplicateErr != nil {
					return "", duplicateErr
				}
				if duplicate != 0 {
					return "", fmt.Errorf("店铺 %s 的关键词已被其他规则使用: %s", cookieID, keyword)
				}
			}
			if // insertErr 是当前店铺规则关键词写入错误。
			_, insertErr := transaction.ExecContext(ctx, `INSERT INTO keywords(cookie_id,keyword,reply,item_id,type,image_url,group_id,match_type,message_scope,system_types,enabled,reply_interval_seconds,send_delay_seconds) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, cookieID, keyword, reply, nullable(itemID), kwType, nullable(imageURL), groupID, matchType, messageScope, systemTypes, cookieEnabled, intervalSeconds, delaySeconds); insertErr != nil {
				return "", insertErr
			}
		}
	}
	if // commitErr 是全局规则店铺绑定事务提交错误。
	commitErr := transaction.Commit(); commitErr != nil {
		return "", commitErr
	}
	return groupID, nil
}

// SetGlobalGroupEnabled 在当前用户全部店铺范围内原子更新一个规则组的启用状态。
func (k *Keywords) SetGlobalGroupEnabled(ctx context.Context, ownedCookieIDs []string, groupID string, enabled bool) error {
	if len(ownedCookieIDs) == 0 || strings.TrimSpace(groupID) == "" {
		return ErrNotFound
	}
	// placeholders、arguments 是限定当前用户店铺范围的 SQL 占位符和参数。
	placeholders, arguments := make([]string, len(ownedCookieIDs)), []any{enabled, groupID}
	// index、cookieID 是当前加入更新范围的店铺下标和账号标识。
	for index, cookieID := range ownedCookieIDs {
		placeholders[index] = "?"
		arguments = append(arguments, cookieID)
	}
	// result、updateErr 是规则组启停更新结果和数据库错误。
	result, updateErr := k.DB.ExecContext(ctx, `UPDATE keywords SET enabled=? WHERE group_id=? AND cookie_id IN (`+strings.Join(placeholders, ",")+`)`, arguments...)
	if updateErr != nil {
		return updateErr
	}
	// affected、rowsErr 是实际更新行数和数据库驱动读取错误。
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAllGlobalGroupsEnabled 原子更新当前用户全部店铺中所有关键词规则的启用状态。
func (k *Keywords) SetAllGlobalGroupsEnabled(ctx context.Context, ownedCookieIDs []string, enabled bool) error {
	if len(ownedCookieIDs) == 0 {
		return nil
	}
	// placeholders、arguments 是限定当前用户全部店铺的 SQL 占位符和参数。
	placeholders, arguments := make([]string, len(ownedCookieIDs)), []any{enabled}
	// index、cookieID 是当前加入批量更新范围的店铺下标和账号标识。
	for index, cookieID := range ownedCookieIDs {
		placeholders[index] = "?"
		arguments = append(arguments, cookieID)
	}
	// updateErr 是单条 SQL 原子批量启停产生的数据库错误。
	_, updateErr := k.DB.ExecContext(ctx, `UPDATE keywords SET enabled=? WHERE cookie_id IN (`+strings.Join(placeholders, ",")+`)`, arguments...)
	return updateErr
}

// SetGlobalGroupAccountEnabled 更新一个规则组在单个店铺中的启用状态。
func (k *Keywords) SetGlobalGroupAccountEnabled(ctx context.Context, cookieID, groupID string, enabled bool) error {
	// result、updateErr 是单店铺规则启停结果和数据库错误。
	result, updateErr := k.DB.ExecContext(ctx, `UPDATE keywords SET enabled=? WHERE cookie_id=? AND group_id=?`, enabled, cookieID, groupID)
	if updateErr != nil {
		return updateErr
	}
	// affected、rowsErr 是实际更新行数和驱动错误。
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteGlobalGroup 从用户全部账号删除一个全局关键词规则组。
func (k *Keywords) DeleteGlobalGroup(ctx context.Context, ownedCookieIDs []string, groupID string) error {
	if len(ownedCookieIDs) == 0 {
		return ErrNotFound
	}
	// placeholders、arguments 是全部账号删除范围的占位符和参数。
	placeholders, arguments := make([]string, len(ownedCookieIDs)), []any{groupID}
	// index、cookieID 是当前加入删除范围的账号下标和标识。
	for index, cookieID := range ownedCookieIDs {
		placeholders[index] = "?"
		arguments = append(arguments, cookieID)
	}
	// result、deleteErr 是全局规则组删除结果和错误。
	result, deleteErr := k.DB.ExecContext(ctx, `DELETE FROM keywords WHERE group_id=? AND cookie_id IN (`+strings.Join(placeholders, ",")+`)`, arguments...)
	if deleteErr != nil {
		return deleteErr
	}
	// affected、rowsErr 是删除行数和读取错误。
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SaveGroup 创建或替换一个多关键词回复组；同组关键词共享回复、商品范围、匹配逻辑和消息来源。
func (k *Keywords) SaveGroup(ctx context.Context, cookieID, groupID string, keywords []string, reply, itemID, kwType, imageURL, matchType, messageScope, systemTypes string, intervalSeconds, delaySeconds int64) (string, error) {
	if strings.TrimSpace(groupID) == "" {
		groupID = fmt.Sprintf("kwg-%d", time.Now().UnixNano())
	}
	// transaction 保证旧组删除和新关键词集合一次提交。
	transaction, beginErr := k.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return "", beginErr
	}
	defer transaction.Rollback()
	if // deleteErr 是同组旧关键词删除错误；事务失败会恢复旧组。
	_, deleteErr := transaction.ExecContext(ctx, `DELETE FROM keywords WHERE cookie_id=? AND group_id=?`, cookieID, groupID); deleteErr != nil {
		return "", deleteErr
	}
	if len(keywords) == 0 {
		keywords = []string{""}
	}
	// keyword 是当前写入同一回复组的独立匹配词；空值表示仅按系统事件触发。
	for _, keyword := range keywords {
		// duplicate 保存同账号和商品范围是否已有其他组使用当前关键词。
		var duplicate int
		if keyword != "" {
			if // duplicateErr 是跨组重复关键词检查错误。
			duplicateErr := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM keywords WHERE cookie_id=? AND COALESCE(item_id,'')=? AND LOWER(keyword)=LOWER(?) AND group_id<>?)`, cookieID, itemID, keyword, groupID).Scan(&duplicate); duplicateErr != nil {
				return "", duplicateErr
			}
		}
		if duplicate != 0 {
			return "", fmt.Errorf("关键词已被其他规则使用: %s", keyword)
		}
		if // insertErr 是当前组内关键词写入错误。
		_, insertErr := transaction.ExecContext(ctx, `INSERT INTO keywords(cookie_id,keyword,reply,item_id,type,image_url,group_id,match_type,message_scope,system_types,reply_interval_seconds,send_delay_seconds) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, cookieID, keyword, reply, nullable(itemID), kwType, nullable(imageURL), groupID, matchType, messageScope, systemTypes, intervalSeconds, delaySeconds); insertErr != nil {
			return "", insertErr
		}
	}
	if // commitErr 是规则组事务提交错误。
	commitErr := transaction.Commit(); commitErr != nil {
		return "", commitErr
	}
	return groupID, nil
}

// DeleteGroup 删除账号下一个完整关键词回复组。
func (k *Keywords) DeleteGroup(ctx context.Context, cookieID, groupID string) error {
	// result、deleteErr 是规则组删除结果和错误。
	result, deleteErr := k.DB.ExecContext(ctx, `DELETE FROM keywords WHERE cookie_id=? AND group_id=?`, cookieID, groupID)
	if deleteErr != nil {
		return deleteErr
	}
	// affected、rowsErr 是实际删除关键词行数和读取错误。
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return rowsErr
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
