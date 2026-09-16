package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// PendingShipmentContext 描述从卖家付款卡片确定的非敏感发货上下文。
type PendingShipmentContext struct {
	// CookieID 是收到卖家付款卡片的真实卖家账号。
	CookieID string
	// ChatID 是付款卡片所属卖家侧会话。
	ChatID string
	// OrderID 是平台付款订单标识。
	OrderID string
	// ItemID 是卡片明确关联的商品标识。
	ItemID string
	// BuyerID 是付款卡片发送者，也是订单买家标识。
	BuyerID string
	// SentAt 是付款卡片的平台 Unix 毫秒时间，用于排除后续终态。
	SentAt int64
}

// GetOwnedPendingShipment 按用户、卖家账号和订单读取仍未出现发货、退款或取消终态的付款卡片。
func (s *ChatStore) GetOwnedPendingShipment(ctx context.Context, userID int64, cookieID, orderID string) (*PendingShipmentContext, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("聊天发货存储未初始化")
	}
	// result 保存通过账号归属、卖家视角和后续终态门禁的付款卡片。
	var result PendingShipmentContext
	// queryErr 是卡片查询错误；没有资格时统一转换为 ErrNotFound。
	queryErr := s.DB.QueryRowContext(ctx, `SELECT paid.cookie_id,paid.chat_id,paid.system_card_order_id,
		paid.system_card_item_id,paid.sender_id,paid.sent_at
		FROM chat_messages paid JOIN cookies account ON account.id=paid.cookie_id
		WHERE account.user_id=? AND paid.cookie_id=? AND paid.system_card_order_id=?
		  AND paid.system_card_event='order_paid' AND TRIM(paid.sender_id)<>TRIM(paid.cookie_id)
		  AND (paid.system_card_action='ship_order' OR TRIM(paid.system_card_action)='')
		  AND NOT EXISTS (SELECT 1 FROM chat_messages later
		    WHERE later.cookie_id=paid.cookie_id AND later.system_card_order_id=paid.system_card_order_id
		      AND later.sent_at>paid.sent_at AND later.system_card_event IN
		      ('order_shipped','order_received','order_completed','order_closed','order_cancelled','refund_requested','refund_completed')
		      AND NOT (later.system_card_event='order_shipped' AND
		        (later.system_card_title LIKE '%记得及时发货%' OR
		          (later.system_card_description LIKE '%如已发货%' AND later.system_card_description LIKE '%去发货%'))))
		  AND NOT EXISTS (SELECT 1 FROM account_task_runs shipment_run
		    WHERE shipment_run.cookie_id=paid.cookie_id AND shipment_run.task_type='order_ship_with_evidence'
		      AND shipment_run.target_id=paid.system_card_order_id
		      AND shipment_run.status IN ('running','success','needs_review'))
		ORDER BY paid.sent_at DESC,paid.id DESC LIMIT 1`, userID, strings.TrimSpace(cookieID), strings.TrimSpace(orderID)).Scan(
		&result.CookieID, &result.ChatID, &result.OrderID, &result.ItemID, &result.BuyerID, &result.SentAt)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if queryErr != nil {
		return nil, queryErr
	}
	return &result, nil
}
