package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// OrderShipmentProof 是 ERP 明确发货成功后保存的非敏感凭证元数据。
type OrderShipmentProof struct {
	// OrderID 是平台订单标识，也是单订单凭证主键。
	OrderID string
	// CookieID 是执行 ERP 发货的卖家账号。
	CookieID string
	// TradeText 是用户提交给闲鱼的发货描述。
	TradeText string
	// ImageURLsJSON 是最多三张官方 HTTPS 图片地址的 JSON 数组。
	ImageURLsJSON string
	// Source 当前固定为 erp，手机或历史发货不写入本表。
	Source string
	// SubmittedAt 是平台明确发货成功时的 Unix 秒。
	SubmittedAt int64
}

// OrderShipmentProofs 管理 ERP 发货凭证元数据，不保存图片二进制或账号凭证。
type OrderShipmentProofs struct {
	// DB 是发货凭证使用的数据库连接。
	DB *sql.DB
	// Dialect 决定跨方言 UPSERT 语法。
	Dialect Dialect
}

// Save 在同一订单重复成功回显时更新为最新的 ERP 发货凭证。
func (proofs *OrderShipmentProofs) Save(ctx context.Context, proof OrderShipmentProof) error {
	if proofs == nil || proofs.DB == nil {
		return errors.New("发货凭证仓储未初始化")
	}
	if strings.TrimSpace(proof.OrderID) == "" || strings.TrimSpace(proof.CookieID) == "" || proof.SubmittedAt <= 0 {
		return errors.New("发货凭证缺少订单、账号或提交时间")
	}
	// query 是三方言共享 INSERT 和方言化冲突更新子句。
	query := `INSERT INTO order_shipment_proofs(order_id,cookie_id,trade_text,image_urls_json,source,submitted_at)
		VALUES(?,?,?,?,?,?)` + dialectUpsert(proofs.Dialect, []string{"order_id"}, map[string]string{
		"cookie_id": "EXCLUDED.cookie_id", "trade_text": "EXCLUDED.trade_text", "image_urls_json": "EXCLUDED.image_urls_json",
		"source": "EXCLUDED.source", "submitted_at": "EXCLUDED.submitted_at", "updated_at": "CURRENT_TIMESTAMP",
	})
	// saveErr 是凭证 UPSERT 执行错误。
	_, saveErr := proofs.DB.ExecContext(ctx, query, strings.TrimSpace(proof.OrderID), strings.TrimSpace(proof.CookieID), proof.TradeText, proof.ImageURLsJSON, "erp", proof.SubmittedAt)
	return saveErr
}

// GetForUser 按订单账号归属读取 ERP 发货凭证，跨用户或不存在统一返回 ErrNotFound。
func (proofs *OrderShipmentProofs) GetForUser(ctx context.Context, userID int64, orderID string) (*OrderShipmentProof, error) {
	if proofs == nil || proofs.DB == nil {
		return nil, errors.New("发货凭证仓储未初始化")
	}
	// proof 保存通过订单和账号用户归属校验的凭证元数据。
	var proof OrderShipmentProof
	// readErr 是用户归属凭证查询错误。
	readErr := proofs.DB.QueryRowContext(ctx, `SELECT proof.order_id,proof.cookie_id,proof.trade_text,proof.image_urls_json,proof.source,proof.submitted_at
		FROM order_shipment_proofs proof JOIN orders current_order ON current_order.order_id=proof.order_id AND current_order.cookie_id=proof.cookie_id
		JOIN cookies account ON account.id=proof.cookie_id
		WHERE account.user_id=? AND proof.order_id=? AND current_order.deleted_at IS NULL`, userID, strings.TrimSpace(orderID)).Scan(
		&proof.OrderID, &proof.CookieID, &proof.TradeText, &proof.ImageURLsJSON, &proof.Source, &proof.SubmittedAt)
	if errors.Is(readErr, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if readErr != nil {
		return nil, readErr
	}
	return &proof, nil
}
