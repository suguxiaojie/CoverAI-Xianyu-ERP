package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// PendingPriceAdjustment 描述从卖家待付款卡片确定的非敏感改价上下文。
type PendingPriceAdjustment struct {
	// CookieID 是拥有订单卡片的卖家账号标识。
	CookieID string
	// ChatID 是订单卡片所属的单聊会话标识。
	ChatID string
	// OrderID 是平台待付款订单标识。
	OrderID string
	// ItemID 是卡片明确关联的商品标识。
	ItemID string
	// SentAt 是待付款卡片的平台 Unix 毫秒时间，用于排除后续终态事件。
	SentAt int64
}

// GetOwnedPendingPriceAdjustment 按用户、卖家账号和订单读取仍未出现付款／关闭终态的改价卡片。
func (s *ChatStore) GetOwnedPendingPriceAdjustment(ctx context.Context, userID int64, cookieID, orderID string) (*PendingPriceAdjustment, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("聊天存储未初始化")
	}
	// result 保存通过账号归属和交易状态门禁的改价上下文。
	var result PendingPriceAdjustment
	// queryErr 是卡片查询和字段映射错误；没有资格时统一转换为 ErrNotFound。
	queryErr := s.DB.QueryRowContext(ctx, `SELECT pending.cookie_id,pending.chat_id,pending.system_card_order_id,
		pending.system_card_item_id,pending.sent_at
		FROM chat_messages pending JOIN cookies account ON account.id=pending.cookie_id
		WHERE account.user_id=? AND pending.cookie_id=? AND pending.system_card_order_id=?
		  AND pending.system_card_event='order_pending_payment' AND pending.system_card_action='adjust_price'
		  AND NOT EXISTS (SELECT 1 FROM chat_messages later
		    WHERE later.cookie_id=pending.cookie_id AND later.system_card_order_id=pending.system_card_order_id
		      AND later.sent_at>pending.sent_at AND later.system_card_event IN
		      ('order_paid','order_closed','order_cancelled','order_shipped','order_received','order_completed','refund_requested','refund_completed'))
		ORDER BY pending.sent_at DESC,pending.id DESC LIMIT 1`, userID, strings.TrimSpace(cookieID), strings.TrimSpace(orderID)).Scan(
		&result.CookieID, &result.ChatID, &result.OrderID, &result.ItemID, &result.SentAt)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if queryErr != nil {
		return nil, queryErr
	}
	return &result, nil
}
