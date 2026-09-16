package db

import (
	"context"
	"strings"
)

// PromoteBuyerShadowOrderInput 描述从卖家付款卡片取得的安全订单归属证据。
type PromoteBuyerShadowOrderInput struct {
	// OrderID 是平台明确提供的数字订单号。
	OrderID string
	// BuyerAccountID 是此前误建影子订单的买家账号，也是订单买家标识。
	BuyerAccountID string
	// SellerAccountID 是收到 role=seller 付款卡片的真实卖家账号。
	SellerAccountID string
	// ChatID 是卖家侧付款卡片所属会话。
	ChatID string
	// ItemID 是付款卡片明确关联的商品标识；为空时保留原值。
	ItemID string
}

// PromoteBuyerShadowOrder 只把同一用户下、仍处于付款早期状态的买家影子订单转交真实卖家账号。
// 返回 true 表示恰好一行完成接管；终态订单、其他卖家订单或跨用户账号保持不变并返回 false。
func (o *Orders) PromoteBuyerShadowOrder(ctx context.Context, input PromoteBuyerShadowOrderInput) (bool, error) {
	if o == nil || o.DB == nil {
		return false, ErrNotFound
	}
	// orderID、buyerAccountID、sellerAccountID 是归属条件使用的规范标识。
	orderID := strings.TrimSpace(input.OrderID)
	// buyerAccountID 是当前订单误归属的买家账号。
	buyerAccountID := strings.TrimSpace(input.BuyerAccountID)
	// sellerAccountID 是付款卡片明确指向的真实卖家账号。
	sellerAccountID := strings.TrimSpace(input.SellerAccountID)
	if orderID == "" || buyerAccountID == "" || sellerAccountID == "" || buyerAccountID == sellerAccountID {
		return false, nil
	}
	// result 是严格条件更新结果；SQL 同时验证两个账号属于同一 ERP 用户，避免跨租户转移。
	result, updateErr := o.DB.ExecContext(ctx, `UPDATE orders
		SET cookie_id=?,
		    buyer_id=CASE WHEN COALESCE(buyer_id,'')='' THEN ? ELSE buyer_id END,
		    chat_id=CASE WHEN ?<>'' THEN ? ELSE chat_id END,
		    item_id=CASE WHEN COALESCE(item_id,'')='' AND ?<>'' THEN ? ELSE item_id END,
		    order_status='pending_ship',deleted_at=NULL,version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE order_id=? AND cookie_id=? AND COALESCE(NULLIF(buyer_id,''),?)=?
		  AND COALESCE(order_status,'') IN ('','unknown','processing','1','paid','2','pending_ship')
		  AND EXISTS (
		    SELECT 1 FROM cookies seller_account
		    JOIN cookies buyer_account ON buyer_account.user_id=seller_account.user_id
		    WHERE seller_account.id=? AND buyer_account.id=?
		  )`, sellerAccountID, buyerAccountID,
		strings.TrimSpace(input.ChatID), strings.TrimSpace(input.ChatID),
		strings.TrimSpace(input.ItemID), strings.TrimSpace(input.ItemID),
		orderID, buyerAccountID, buyerAccountID, buyerAccountID, sellerAccountID, buyerAccountID)
	if updateErr != nil {
		return false, updateErr
	}
	// affected 是满足全部安全门禁并实际转移的订单行数。
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return false, rowsErr
	}
	return affected == 1, nil
}
