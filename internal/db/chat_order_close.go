package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// PendingOrderClose 描述从卖家待付款／付款卡片和订单表确定的非敏感取消上下文。
type PendingOrderClose struct {
	// CookieID 是拥有订单卡片的卖家账号标识。
	CookieID string
	// ChatID 是订单卡片所属的单聊会话标识。
	ChatID string
	// OrderID 是平台待取消订单标识。
	OrderID string
	// ItemID 是卡片明确关联的商品标识。
	ItemID string
	// SentAt 是当前阶段卡片的平台 Unix 毫秒时间，用于排除后续终态事件。
	SentAt int64
	// Stage 是 pending_payment 或 pending_ship，来自权威订单表状态。
	Stage string
}

// GetOwnedPendingOrderClose 按用户、卖家账号和订单读取仍可取消的待付款或已付款待发货卡片。
func (s *ChatStore) GetOwnedPendingOrderClose(ctx context.Context, userID int64, cookieID, orderID string) (*PendingOrderClose, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("聊天存储未初始化")
	}
	// result 保存通过账号归属和交易状态门禁的关单上下文。
	var result PendingOrderClose
	// queryErr 是卡片查询和字段映射错误；没有资格时统一转换为 ErrNotFound。
	queryErr := s.DB.QueryRowContext(ctx, `SELECT pending.cookie_id,pending.chat_id,pending.system_card_order_id,
		pending.system_card_item_id,pending.sent_at,
		CASE WHEN TRIM(COALESCE(current_order.order_status,'')) IN ('pending_ship','paid','2') THEN 'pending_ship' ELSE 'pending_payment' END
		FROM chat_messages pending
		JOIN cookies account ON account.id=pending.cookie_id
		JOIN orders current_order ON current_order.order_id=pending.system_card_order_id AND current_order.cookie_id=pending.cookie_id
		WHERE account.user_id=? AND pending.cookie_id=? AND pending.system_card_order_id=?
		  AND pending.system_card_event IN ('order_pending_payment','order_paid')
		  AND TRIM(pending.sender_id)<>TRIM(pending.cookie_id)
		  AND (pending.system_card_event<>'order_paid' OR TRIM(COALESCE(pending.system_card_action,'')) IN ('','ship_order'))
		  AND current_order.deleted_at IS NULL
		  AND TRIM(COALESCE(current_order.order_status,'')) IN ('processing','1','pending_ship','paid','2')
		  AND NOT EXISTS (SELECT 1 FROM chat_messages later
		    WHERE later.cookie_id=pending.cookie_id AND later.system_card_order_id=pending.system_card_order_id
		      AND later.sent_at>pending.sent_at AND later.system_card_event IN
		      ('order_closed','order_cancelled','order_shipped','order_received','order_completed','refund_requested','refund_completed')
		      AND NOT (later.system_card_event='order_shipped' AND
		        (later.system_card_title LIKE '%记得及时发货%' OR
		          (later.system_card_description LIKE '%如已发货%' AND later.system_card_description LIKE '%去发货%'))))
		  AND NOT EXISTS (SELECT 1 FROM account_task_runs close_run
		    WHERE close_run.cookie_id=pending.cookie_id AND close_run.task_type='order_close'
		      AND close_run.target_id=pending.system_card_order_id
		      AND close_run.status IN ('running','success','needs_review'))
		ORDER BY pending.sent_at DESC,pending.id DESC LIMIT 1`, userID, strings.TrimSpace(cookieID), strings.TrimSpace(orderID)).Scan(
		&result.CookieID, &result.ChatID, &result.OrderID, &result.ItemID, &result.SentAt, &result.Stage)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if queryErr != nil {
		return nil, queryErr
	}
	return &result, nil
}
