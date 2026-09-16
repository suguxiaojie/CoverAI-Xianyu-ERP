package db

import "context"

// ClaimKeywordEventReply 抢占账号、订单、规则组和系统事件的唯一发送权。
// 首次写入或旧 pending 租约过期时返回 true；已成功或有效租约返回 false。
func (k *Keywords) ClaimKeywordEventReply(ctx context.Context, cookieID, orderID, groupID, eventType, claimToken string, now, leaseExpiresAt int64) (bool, error) {
	// insertErr 是首次插入订单事件发送租约的数据库结果。
	_, insertErr := k.DB.ExecContext(ctx, `INSERT INTO keyword_event_reply_records(cookie_id,order_id,group_id,event_type,status,claim_token,lease_expires_at,last_success_at,updated_at) VALUES(?,?,?,?,'pending',?,?,0,?)`, cookieID, orderID, groupID, eventType, claimToken, leaseExpiresAt, now)
	if insertErr == nil {
		return true, nil
	}
	if !isUniqueViolation(insertErr) {
		return false, insertErr
	}
	// result 是仅在旧 pending 租约过期时重新抢占的条件更新结果；success 永不覆盖。
	result, updateErr := k.DB.ExecContext(ctx, `UPDATE keyword_event_reply_records SET claim_token=?,lease_expires_at=?,updated_at=? WHERE cookie_id=? AND order_id=? AND group_id=? AND event_type=? AND status='pending' AND lease_expires_at<=?`, claimToken, leaseExpiresAt, now, cookieID, orderID, groupID, eventType, now)
	if updateErr != nil {
		return false, updateErr
	}
	// affected 是当前调用实际取得的订单事件幂等租约数量。
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return false, affectedErr
	}
	return affected == 1, nil
}

// MarkKeywordEventReplySuccess 在平台发送成功后永久完成订单事件幂等记录。
func (k *Keywords) MarkKeywordEventReplySuccess(ctx context.Context, cookieID, orderID, groupID, eventType, claimToken string, now int64) error {
	// result 是只有当前租约持有者能够完成的状态更新结果。
	result, updateErr := k.DB.ExecContext(ctx, `UPDATE keyword_event_reply_records SET status='success',last_success_at=?,lease_expires_at=0,updated_at=? WHERE cookie_id=? AND order_id=? AND group_id=? AND event_type=? AND status='pending' AND claim_token=?`, now, now, cookieID, orderID, groupID, eventType, claimToken)
	if updateErr != nil {
		return updateErr
	}
	return requireKnowledgeRowsAffected(result)
}

// ReleaseKeywordEventReply 仅在尚未发送成功时释放当前租约，允许同一订单后续实时事件重试。
func (k *Keywords) ReleaseKeywordEventReply(ctx context.Context, cookieID, orderID, groupID, eventType, claimToken string) error {
	// deleteErr 是删除当前调用持有的 pending 租约失败原因；零行表示租约已完成或被替换，无需报错。
	_, deleteErr := k.DB.ExecContext(ctx, `DELETE FROM keyword_event_reply_records WHERE cookie_id=? AND order_id=? AND group_id=? AND event_type=? AND status='pending' AND claim_token=?`, cookieID, orderID, groupID, eventType, claimToken)
	return deleteErr
}
