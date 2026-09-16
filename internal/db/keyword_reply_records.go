package db

import (
	"context"
	"database/sql"
	"errors"
)

// KeywordReplyAllowed 判断同店铺、会话和规则组是否已经度过重复回复间隔。
func (k *Keywords) KeywordReplyAllowed(ctx context.Context, cookieID, chatID, groupID string, intervalSeconds, now int64) (bool, error) {
	if intervalSeconds <= 0 {
		return true, nil
	}
	// lastSuccess 是该会话规则组最近一次成功回复的秒级时间。
	var lastSuccess int64
	// queryErr 是读取最近成功时间的数据库错误。
	queryErr := k.DB.QueryRowContext(ctx, `SELECT last_success_at FROM keyword_reply_records WHERE cookie_id=? AND chat_id=? AND group_id=?`, cookieID, chatID, groupID).Scan(&lastSuccess)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return true, nil
	}
	if queryErr != nil {
		return false, queryErr
	}
	return now-lastSuccess >= intervalSeconds, nil
}

// MarkKeywordReplySuccess 在回复全部发送成功后更新规则组冷却起点。
func (k *Keywords) MarkKeywordReplySuccess(ctx context.Context, cookieID, chatID, groupID string, now int64) error {
	// upsert 保存跨方言一致的成功时间覆盖语义。
	upsert := dialectUpsert(k.Dialect, []string{"cookie_id", "chat_id", "group_id"}, map[string]string{"last_success_at": "EXCLUDED.last_success_at", "pending_token": "''", "lease_expires_at": "0", "updated_at": "EXCLUDED.updated_at"})
	// writeErr 是写入最新成功回复时间的数据库执行结果。
	_, writeErr := k.DB.ExecContext(ctx, `INSERT INTO keyword_reply_records(cookie_id,chat_id,group_id,last_success_at,pending_token,lease_expires_at,updated_at) VALUES(?,?,?,?,'',0,?)`+upsert, cookieID, chatID, groupID, now, now)
	return writeErr
}
