package db

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

// resolvedOrderChatIDExpression 只在关联结果唯一时返回可安全导航的订单会话：持久化值优先，其次为同账号同订单系统卡片，最后为没有订单卡片时的同账号买家商品唯一会话。
const resolvedOrderChatIDExpression = `COALESCE(NULLIF(o.chat_id,''),
	(SELECT MIN(order_card.chat_id) FROM chat_messages order_card
	  WHERE order_card.cookie_id=o.cookie_id AND order_card.system_card_order_id=o.order_id
	    AND COALESCE(order_card.chat_id,'')<>''
	  HAVING COUNT(DISTINCT order_card.chat_id)=1),
	(SELECT MIN(buyer_session.chat_id) FROM chat_sessions buyer_session
	  WHERE buyer_session.cookie_id=o.cookie_id
	    AND COALESCE(o.buyer_id,'')<>'' AND buyer_session.buyer_id=o.buyer_id
	    AND COALESCE(o.item_id,'')<>'' AND buyer_session.item_id=o.item_id
	    AND COALESCE(buyer_session.chat_id,'')<>''
	    AND NOT EXISTS(SELECT 1 FROM chat_messages exact_order_card
	      WHERE exact_order_card.cookie_id=o.cookie_id AND exact_order_card.system_card_order_id=o.order_id
	        AND COALESCE(exact_order_card.chat_id,'')<>'')
	  HAVING COUNT(DISTINCT buyer_session.chat_id)=1),'')`

// OrderRow 订单列表展示行（含 item_title）。
type OrderRow struct {
	OrderID    string
	ItemID     string
	ItemTitle  string
	ItemDetail string
	BuyerID    string
	// ChatID 是持久化关联或唯一历史事实解析出的精确会话标识，供联系买家和历史订单上下文使用。
	ChatID      string
	BuyerName   string
	BuyerAvatar string
	SpecName    string
	SpecValue   string
	Quantity    string
	Amount      string
	// ShipmentProofAvailable 表示当前订单存在 ERP 保存的发货凭证。
	ShipmentProofAvailable bool
	OrderStatus            string
	CookieID               string
	IsBargain              int
	SystemShipped          bool
	ReceiverName           string
	ReceiverPhone          string
	ReceiverAddr           string
	ReceiverCity           string
	CreatedAt              string
	UpdatedAt              string
	PaidAt                 string
	ShippedAt              string
	ReceivedAt             string
	CompletedAt            string
	RefundedAt             string
	CancelledAt            string
	RefundRequested        bool
}

// OrderListFilter 是订单列表分页查询条件。
type OrderListFilter struct {
	UserID   int64
	CookieID string
	Status   string
	Search   string
	// ContextBuyerID 是会话订单查询的精确买家标识。
	ContextBuyerID string
	// ContextChatID 是会话订单查询的精确聊天标识。
	ContextChatID string
	// MatchConversationContext 表示按会话或买家任一关系匹配。
	MatchConversationContext bool
	// PrioritizeChatID 让明确属于当前会话的订单排在同买家历史订单前。
	PrioritizeChatID string
	CreatedFrom      time.Time
	CreatedTo        time.Time
	// MinAmountCents 是实付金额的可选包含下界，单位为人民币分。
	MinAmountCents *int64
	// MaxAmountCents 是实付金额的可选包含上界，单位为人民币分。
	MaxAmountCents *int64
	Limit          int
	Offset         int
}

// orderListTimeArgument 按方言生成订单时间查询参数；SQLite 使用与存量数据一致的 UTC RFC3339 文本。
func orderListTimeArgument(dialect Dialect, value time.Time) any {
	// normalized 是用于跨方言查询的 UTC 时间值。
	normalized := value.UTC()
	if dialect == DialectSQLite {
		return normalized.Format(time.RFC3339Nano)
	}
	return normalized
}

// orderListAmountCentsExpression 返回各数据库把规范十进制金额转换为整数分的表达式。
func orderListAmountCentsExpression(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return "CAST(ROUND(CAST(NULLIF(o.amount,'') AS DECIMAL(65,30))*100) AS SIGNED)"
	case DialectPostgres:
		return "CAST(ROUND(CAST(NULLIF(o.amount,'') AS NUMERIC)*100) AS BIGINT)"
	default:
		return "CAST(ROUND(CAST(NULLIF(o.amount,'') AS REAL)*100) AS INTEGER)"
	}
}

// ShipmentAssociationCandidate 是无订单号发货卡片进行唯一会话关联所需的最小订单事实。
type ShipmentAssociationCandidate struct {
	// OrderID 是待关联的订单标识。
	OrderID string
	// ItemID 是订单商品标识。
	ItemID string
	// BuyerID 是订单买家标识。
	BuyerID string
	// ChatID 是订单会话标识。
	ChatID string
	// OrderStatus 是当前原始订单状态。
	OrderStatus string
	// PaidAt 是订单付款时间。
	PaidAt string
	// ShippedAt 是订单最近确认发货的时间；待发货候选固定为空。
	ShippedAt string
}

// RefundAssociationCandidate 是无订单号退款成功消息进行唯一会话关联所需的最小订单事实。
type RefundAssociationCandidate struct {
	// OrderID 是正在退款的精确订单号。
	OrderID string
	// ChatID 是退款申请卡片所属会话。
	ChatID string
	// UpdatedAt 是最近退款事实写入时间，仅用于稳定排序。
	UpdatedAt string
}

// RefundingByChat 返回同账号同会话仍处于退款中的订单，最多读取 limit 条用于歧义判断。
func (o *Orders) RefundingByChat(ctx context.Context, cookieID, chatID string, limit int) ([]RefundAssociationCandidate, error) {
	if limit <= 0 || limit > 10 {
		limit = 3
	}
	// rows、queryErr 是退款候选查询结果和数据库错误。
	rows, queryErr := o.DB.QueryContext(ctx, `SELECT order_id,COALESCE(chat_id,''),COALESCE(updated_at,'')
		FROM orders WHERE cookie_id=? AND chat_id=? AND deleted_at IS NULL AND order_status='refunding'
		ORDER BY updated_at DESC,order_id DESC LIMIT ?`, cookieID, chatID, limit)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// candidates 保存当前会话仍等待退款终态的订单。
	candidates := make([]RefundAssociationCandidate, 0, limit)
	for rows.Next() {
		// candidate 是当前待扫描的退款候选。
		var candidate RefundAssociationCandidate
		if // scanErr 是当前退款候选字段扫描错误。
		scanErr := rows.Scan(&candidate.OrderID, &candidate.ChatID, &candidate.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// UnshippedPaidByChat 返回同账号同会话尚未记录发货时间的已付款开放订单，最多读取 limit 条用于歧义判断。
func (o *Orders) UnshippedPaidByChat(ctx context.Context, cookieID, chatID string, limit int) ([]ShipmentAssociationCandidate, error) {
	if limit <= 0 || limit > 10 {
		limit = 3
	}
	// rows、queryErr 是候选订单查询结果和数据库错误。
	rows, queryErr := o.DB.QueryContext(ctx, `SELECT order_id,COALESCE(item_id,''),COALESCE(buyer_id,''),COALESCE(chat_id,''),
		COALESCE(order_status,''),COALESCE(paid_at,'') FROM orders
		WHERE cookie_id=? AND chat_id=? AND deleted_at IS NULL AND COALESCE(paid_at,'')<>'' AND COALESCE(shipped_at,'')=''
		AND COALESCE(order_status,'') IN ('','unknown','processing','1','pending_ship','paid','2')
		ORDER BY paid_at DESC,order_id DESC LIMIT ?`, cookieID, chatID, limit)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// candidates 保存当前会话中可能关联的开放订单。
	candidates := make([]ShipmentAssociationCandidate, 0, limit)
	for rows.Next() {
		// candidate 是当前待扫描的订单关联候选。
		var candidate ShipmentAssociationCandidate
		if // scanErr 是当前发货关联候选字段扫描错误。
		scanErr := rows.Scan(&candidate.OrderID, &candidate.ItemID, &candidate.BuyerID, &candidate.ChatID, &candidate.OrderStatus, &candidate.PaidAt); scanErr != nil {
			return nil, scanErr
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// ShippedByChat 返回同账号同会话最近已记录发货时间的订单，最多读取 limit 条用于系统回复唯一关联。
func (o *Orders) ShippedByChat(ctx context.Context, cookieID, chatID string, limit int) ([]ShipmentAssociationCandidate, error) {
	if limit <= 0 || limit > 10 {
		limit = 3
	}
	// rows 和 queryErr 是最近已发货订单候选及数据库错误。
	rows, queryErr := o.DB.QueryContext(ctx, `SELECT order_id,COALESCE(item_id,''),COALESCE(buyer_id,''),COALESCE(chat_id,''),COALESCE(order_status,''),COALESCE(paid_at,''),COALESCE(shipped_at,'') FROM orders WHERE cookie_id=? AND chat_id=? AND deleted_at IS NULL AND COALESCE(shipped_at,'')<>'' ORDER BY shipped_at DESC,order_id DESC LIMIT ?`, cookieID, chatID, limit)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// candidates 保存当前会话最近已发货的订单事实。
	candidates := make([]ShipmentAssociationCandidate, 0, limit)
	// rows.Next 每次推进到一笔最近已发货订单。
	for rows.Next() {
		// candidate 是当前待扫描的已发货订单候选。
		var candidate ShipmentAssociationCandidate
		// scanErr 是当前候选字段扫描失败原因。
		if scanErr := rows.Scan(&candidate.OrderID, &candidate.ItemID, &candidate.BuyerID, &candidate.ChatID, &candidate.OrderStatus, &candidate.PaidAt, &candidate.ShippedAt); scanErr != nil {
			return nil, scanErr
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// ListForUser 按用户隔离分页查询订单，并带出商品标题/详情。
func (o *Orders) ListForUser(ctx context.Context, f OrderListFilter) ([]OrderRow, int, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}
	// where 用于本次流程后续判断的where
	where := []string{"c.user_id=?", "o.deleted_at IS NULL"}
	// args 用于本次流程后续判断的args
	args := []any{f.UserID}
	if f.CookieID != "" {
		where = append(where, "o.cookie_id=?")
		args = append(args, f.CookieID)
	}
	if f.MatchConversationContext {
		// relationshipConditions 保存当前账号内允许命中的精确会话和精确买家关系。
		relationshipConditions := make([]string, 0, 2)
		if strings.TrimSpace(f.ContextChatID) != "" {
			relationshipConditions = append(relationshipConditions, "COALESCE(o.chat_id,'')=?")
			args = append(args, strings.TrimSpace(f.ContextChatID))
		}
		if strings.TrimSpace(f.ContextBuyerID) != "" {
			relationshipConditions = append(relationshipConditions, "(COALESCE(o.buyer_id,'')<>'' AND o.buyer_id=?)")
			args = append(args, strings.TrimSpace(f.ContextBuyerID))
		}
		if len(relationshipConditions) > 0 {
			where = append(where, "("+strings.Join(relationshipConditions, " OR ")+")")
		}
	}
	if // statuses 用于本次流程后续判断的statuses
	statuses := normalizedStatusCandidates(f.Status); len(statuses) > 0 {
		// placeholders 用于本次流程后续判断的placeholders
		placeholders := make([]string, 0, len(statuses))
		// st 表示当前遍历过程中的st
		for _, st := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, st)
		}
		where = append(where, "o.order_status IN ("+strings.Join(placeholders, ",")+")")
	}
	if !f.CreatedFrom.IsZero() {
		where = append(where, "o.created_at>=?")
		args = append(args, orderListTimeArgument(o.Dialect, f.CreatedFrom))
	}
	if !f.CreatedTo.IsZero() {
		where = append(where, "o.created_at<?")
		args = append(args, orderListTimeArgument(o.Dialect, f.CreatedTo))
	}
	// amountCentsExpression 把持久化金额统一转换为整数分，避免按文本字典序或二进制浮点边界筛选。
	amountCentsExpression := orderListAmountCentsExpression(o.Dialect)
	if f.MinAmountCents != nil {
		where = append(where, amountCentsExpression+">=?")
		args = append(args, *f.MinAmountCents)
	}
	if f.MaxAmountCents != nil {
		where = append(where, amountCentsExpression+"<=?")
		args = append(args, *f.MaxAmountCents)
	}
	if // search 用于本次流程后续判断的搜索
	search := strings.ToLower(strings.TrimSpace(f.Search)); search != "" {
		// pattern 用于本次流程后续判断的pattern
		pattern := "%" + search + "%"
		where = append(where, `(LOWER(o.order_id) LIKE ? OR LOWER(COALESCE(o.item_id,'')) LIKE ?
			OR LOWER(COALESCE(o.buyer_id,'')) LIKE ? OR LOWER(COALESCE(i.item_title,'')) LIKE ?
			OR LOWER(COALESCE(o.receiver_name,'')) LIKE ? OR LOWER(COALESCE(o.receiver_phone,'')) LIKE ?
			OR EXISTS(SELECT 1 FROM chat_sessions search_session WHERE search_session.cookie_id=o.cookie_id
			  AND ((COALESCE(o.chat_id,'')<>'' AND search_session.chat_id=o.chat_id)
			    OR (COALESCE(o.buyer_id,'')<>'' AND search_session.buyer_id=o.buyer_id))
			  AND LOWER(COALESCE(search_session.buyer_name,'')) LIKE ?))`)
		for // i 用于本次流程后续判断的i
		i := 0; i < 7; i++ {
			args = append(args, pattern)
		}
	}
	// whereSQL 用于本次流程后续判断的whereSQL
	whereSQL := strings.Join(where, " AND ")

	// total 用于本次流程后续判断的总数
	var total int
	if // err 用于本次流程后续判断的err
	err := o.DB.QueryRowContext(ctx,
		`SELECT COUNT(*)
		   FROM orders o
		   JOIN cookies c ON c.id=o.cookie_id
		   LEFT JOIN item_info i ON i.cookie_id=o.cookie_id AND i.item_id=o.item_id
		  WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// queryArgs 用于本次流程后续判断的查询Args
	queryArgs := append([]any{}, args...)
	// orderBySQL 默认保持订单列表时间倒序；会话上下文查询把本会话订单置顶。
	orderBySQL := "o.created_at DESC, o.order_id DESC"
	if strings.TrimSpace(f.PrioritizeChatID) != "" {
		orderBySQL = "CASE WHEN COALESCE(o.chat_id,'')=? THEN 0 ELSE 1 END, o.created_at DESC, o.order_id DESC"
		queryArgs = append(queryArgs, strings.TrimSpace(f.PrioritizeChatID))
	}
	queryArgs = append(queryArgs, f.Limit, f.Offset)
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := o.DB.QueryContext(ctx,
		`SELECT o.order_id, o.item_id,
		        COALESCE(NULLIF(i.item_title,''),(SELECT title_session.item_title FROM chat_sessions title_session
		          WHERE title_session.cookie_id=o.cookie_id AND title_session.item_id=o.item_id AND COALESCE(title_session.item_title,'')<>''
		          ORDER BY title_session.updated_at DESC,title_session.chat_id DESC LIMIT 1),''),
		        COALESCE(i.item_detail,''), o.buyer_id, `+resolvedOrderChatIDExpression+`,
		        COALESCE((SELECT buyer_session.buyer_name FROM chat_sessions buyer_session
		          WHERE buyer_session.cookie_id=o.cookie_id
		            AND ((COALESCE(o.chat_id,'')<>'' AND buyer_session.chat_id=o.chat_id)
		              OR (COALESCE(o.buyer_id,'')<>'' AND buyer_session.buyer_id=o.buyer_id))
		            AND COALESCE(buyer_session.buyer_name,'')<>''
		          ORDER BY CASE WHEN COALESCE(o.chat_id,'')<>'' AND buyer_session.chat_id=o.chat_id THEN 0 ELSE 1 END,buyer_session.updated_at DESC,buyer_session.chat_id DESC LIMIT 1),''),
		        COALESCE((SELECT avatar_session.buyer_avatar_url FROM chat_sessions avatar_session
		          WHERE avatar_session.cookie_id=o.cookie_id
		            AND ((COALESCE(o.chat_id,'')<>'' AND avatar_session.chat_id=o.chat_id)
		              OR (COALESCE(o.buyer_id,'')<>'' AND avatar_session.buyer_id=o.buyer_id))
		            AND COALESCE(avatar_session.buyer_avatar_url,'')<>''
		          ORDER BY CASE WHEN COALESCE(o.chat_id,'')<>'' AND avatar_session.chat_id=o.chat_id THEN 0 ELSE 1 END,avatar_session.updated_at DESC,avatar_session.chat_id DESC LIMIT 1),''),
		        o.spec_name, o.spec_value, o.quantity, o.amount,
		        CASE WHEN EXISTS(SELECT 1 FROM order_shipment_proofs shipment_proof WHERE shipment_proof.order_id=o.order_id AND shipment_proof.cookie_id=o.cookie_id) THEN 1 ELSE 0 END,
		        o.order_status, o.cookie_id, o.is_bargain, o.system_shipped,
		        o.receiver_name, o.receiver_phone, o.receiver_address, o.receiver_city,
		        o.created_at, o.updated_at,COALESCE(o.paid_at,''),COALESCE(o.shipped_at,''),COALESCE(o.received_at,''),COALESCE(o.completed_at,''),COALESCE(o.refunded_at,''),COALESCE(o.cancelled_at,'')
		   FROM orders o
		   JOIN cookies c ON c.id=o.cookie_id
		   LEFT JOIN item_info i ON i.cookie_id=o.cookie_id AND i.item_id=o.item_id
		  WHERE `+whereSQL+`
		  ORDER BY `+orderBySQL+`
		  LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	// out 用于本次流程后续判断的out
	out := []OrderRow{}
	for rows.Next() {
		// r 用于本次流程后续判断的r
		var r OrderRow
		// itemID、itemTitle、itemDetail、buyerID、buyerName、buyerAvatar、specName、specValue、qty、amount、receiverName、receiverPhone、receiverAddr、receiverCity 保存订单列表的可空展示字段。
		var itemID, itemTitle, itemDetail, buyerID, buyerName, buyerAvatar, specName, specValue, qty, amount, receiverName, receiverPhone, receiverAddr, receiverCity sql.NullString
		// isBargain、sysShipped、shipmentProofAvailable 保存议价、系统发货和 ERP 凭证存在标记。
		var isBargain, sysShipped, shipmentProofAvailable int
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&r.OrderID, &itemID, &itemTitle, &itemDetail, &buyerID, &r.ChatID, &buyerName, &buyerAvatar, &specName, &specValue, &qty, &amount, &shipmentProofAvailable,
			&r.OrderStatus, &r.CookieID, &isBargain, &sysShipped, &receiverName, &receiverPhone, &receiverAddr,
			&receiverCity, &r.CreatedAt, &r.UpdatedAt, &r.PaidAt, &r.ShippedAt, &r.ReceivedAt, &r.CompletedAt, &r.RefundedAt, &r.CancelledAt); err != nil {
			return nil, 0, err
		}
		r.ItemID = itemID.String
		r.ItemTitle = itemTitle.String
		r.ItemDetail = itemDetail.String
		r.BuyerID = buyerID.String
		r.BuyerName = buyerName.String
		r.BuyerAvatar = buyerAvatar.String
		r.SpecName = specName.String
		r.SpecValue = specValue.String
		r.Quantity = qty.String
		r.Amount = amount.String
		r.IsBargain = isBargain
		r.SystemShipped = sysShipped != 0
		r.ShipmentProofAvailable = shipmentProofAvailable != 0
		r.ReceiverName = receiverName.String
		r.ReceiverPhone = receiverPhone.String
		r.ReceiverAddr = receiverAddr.String
		r.ReceiverCity = receiverCity.String
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// normalizedStatusCandidates 封装normalized状态Candidates业务协调。
func normalizedStatusCandidates(status string) []string {
	status = strings.TrimSpace(status)
	if status == "" || status == "all" {
		return nil
	}
	switch NormalizeOrderStatus(status) {
	case "processing":
		return []string{"processing", "1"}
	case "pending_ship":
		return []string{"pending_ship", "paid", "2"}
	case "shipped":
		return []string{"shipped", "3"}
	case "received":
		return []string{"received"}
	case "completed":
		return []string{"completed", "4", "11"}
	case "refunding":
		return []string{"refunding", "5", "7", "9"}
	case "refunded":
		return []string{"refunded"}
	case "cancelled":
		return []string{"cancelled", "6", "8", "10", "12"}
	case "unknown":
		return []string{status}
	default:
		return []string{status}
	}
}

// ByCookie 取某账号的订单（limit 上限）。
func (o *Orders) ByCookie(ctx context.Context, cookieID string, limit int) ([]OrderRow, error) {
	if limit <= 0 {
		limit = 1000
	}
	return o.ByCookiePage(ctx, cookieID, limit, 0)
}

// ByCookiePage 分页读取账号订单，供需要完整扫描的后台任务使用。
func (o *Orders) ByCookiePage(ctx context.Context, cookieID string, limit, offset int) ([]OrderRow, error) {
	if limit <= 0 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := o.DB.QueryContext(ctx,
		`SELECT order_id, item_id, buyer_id, spec_name, spec_value, quantity, amount,
		        order_status, is_bargain, system_shipped, receiver_name, receiver_phone,
		        receiver_address, receiver_city, created_at, updated_at,
		        EXISTS(SELECT 1 FROM chat_messages cm WHERE cm.cookie_id=orders.cookie_id AND cm.system_card_order_id=orders.order_id AND cm.system_card_event='refund_requested')
		 FROM orders WHERE cookie_id=? AND deleted_at IS NULL ORDER BY created_at DESC,order_id DESC LIMIT ? OFFSET ?`, cookieID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows, cookieID)
}

// ByCookieCursor 使用 created_at 与 order_id 复合游标扫描账号订单，避免大 OFFSET 的线性跳过成本。
func (o *Orders) ByCookieCursor(ctx context.Context, cookieID string, limit int, afterCreatedAt, afterOrderID string) ([]OrderRow, error) {
	if limit <= 0 {
		limit = 500
	}
	// query 保存按复合游标扫描订单的 SQL。
	query := `SELECT order_id, item_id, buyer_id, spec_name, spec_value, quantity, amount,
		        order_status, is_bargain, system_shipped, receiver_name, receiver_phone,
		        receiver_address, receiver_city, created_at, updated_at,
		        EXISTS(SELECT 1 FROM chat_messages cm WHERE cm.cookie_id=orders.cookie_id AND cm.system_card_order_id=orders.order_id AND cm.system_card_event='refund_requested')
		 FROM orders WHERE cookie_id=? AND deleted_at IS NULL`
	// args 保存游标查询参数。
	args := []any{cookieID}
	if afterCreatedAt != "" || afterOrderID != "" {
		// cursorCreatedAt 将驱动层返回的 RFC3339 时间还原为数据库通用的时间文本格式。
		cursorCreatedAt := normalizeOrderCursorTime(afterCreatedAt)
		query += ` AND (created_at < ? OR (created_at = ? AND order_id < ?))`
		args = append(args, cursorCreatedAt, cursorCreatedAt, afterOrderID)
	}
	query += ` ORDER BY created_at DESC,order_id DESC LIMIT ?`
	args = append(args, limit)
	// rows、err 保存游标查询结果及错误。
	rows, err := o.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows, cookieID)
}

// normalizeOrderCursorTime 将订单行时间转换为跨数据库可比较的 UTC 文本。
func normalizeOrderCursorTime(value string) string {
	// parsed 表示驱动层返回的标准时间值。
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC().Format("2006-01-02 15:04:05.999999999")
	}
	return strings.Replace(strings.TrimSuffix(value, "Z"), "T", " ", 1)
}

// scanOrderRows 将订单查询行转换为统一的订单列表模型。
func scanOrderRows(rows *sql.Rows, cookieID string) ([]OrderRow, error) {
	// out 用于本次流程后续判断的out
	var out []OrderRow
	for rows.Next() {
		// r 用于本次流程后续判断的r
		var r OrderRow
		// itemID、buyerID、specName、specValue、qty、amount、receiverName、receiverPhone、receiverAddr、receiverCity 保存商品ID、buyerID、specName、specValue、qty、amount、receiverName、receiverPhone、receiverAddr、receiverCity，供当前处理流程使用
		var itemID, buyerID, specName, specValue, qty, amount, receiverName, receiverPhone, receiverAddr, receiverCity sql.NullString
		// isBargain、sysShipped 用于本次流程后续判断的isBargain、sysShipped
		var isBargain, sysShipped int
		// refundRequested 表示当前订单是否有精确退款申请聊天证据。
		var refundRequested bool
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&r.OrderID, &itemID, &buyerID, &specName, &specValue, &qty, &amount,
			&r.OrderStatus, &isBargain, &sysShipped, &receiverName, &receiverPhone, &receiverAddr,
			&receiverCity, &r.CreatedAt, &r.UpdatedAt, &refundRequested); err != nil {
			return nil, err
		}
		r.ItemID = itemID.String
		r.BuyerID = buyerID.String
		r.SpecName = specName.String
		r.SpecValue = specValue.String
		r.Quantity = qty.String
		r.Amount = amount.String
		r.IsBargain = isBargain
		r.SystemShipped = sysShipped != 0
		r.ReceiverName = receiverName.String
		r.ReceiverPhone = receiverPhone.String
		r.ReceiverAddr = receiverAddr.String
		r.ReceiverCity = receiverCity.String
		r.CookieID = cookieID
		r.RefundRequested = refundRequested
		out = append(out, r)
	}
	return out, rows.Err()
}

// SoftDeleteMissingForCookie 将本次完整卖家订单同步中未出现的本地订单逻辑删除。
// activeIDs 为空表示线上已确认没有任何卖家订单；调用方必须确保同步完整成功后再调用。
// SoftDeleteMissingForCookie 封装SoftDeleteMissingFor登录凭证业务协调。
func (o *Orders) SoftDeleteMissingForCookie(ctx context.Context, cookieID string, activeIDs map[string]struct{}) (int, error) {
	if strings.TrimSpace(cookieID) == "" {
		return 0, errors.New("cookie_id 不能为空")
	}
	// args 保存批量 UPDATE 的参数，首个参数是账号 ID。
	args := []any{cookieID}
	// query 保存批量逻辑删除 SQL。
	query := `UPDATE orders SET deleted_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		WHERE cookie_id=? AND deleted_at IS NULL`
	if len(activeIDs) > 0 {
		// activeOrderIDs 按稳定顺序保存线上仍存在的订单 ID，便于日志和测试复现。
		activeOrderIDs := make([]string, 0, len(activeIDs))
		// orderID 表示当前线上仍存在的订单标识。
		for orderID := range activeIDs {
			activeOrderIDs = append(activeOrderIDs, orderID)
		}
		sort.Strings(activeOrderIDs)
		// placeholders 保存 NOT IN 子句所需的占位符。
		placeholders := make([]string, len(activeOrderIDs))
		// i、orderID 分别表示占位符序号和线上订单标识。
		for i, orderID := range activeOrderIDs {
			placeholders[i] = "?"
			args = append(args, orderID)
		}
		query += ` AND order_id NOT IN (` + strings.Join(placeholders, ",") + `)`
	}
	// result、err 保存批量逻辑删除结果及错误。
	result, err := o.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	// deleted 保存本次批量逻辑删除的订单数量。
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

// OrderStatusMap 将数字状态码转换为文本状态。
var OrderStatusMap = map[string]string{
	"paid": "pending_ship",
	"1":    "processing", "2": "pending_ship", "3": "shipped", "4": "completed",
	"5": "refunding", "6": "cancelled", "7": "refunding", "8": "cancelled",
	"9": "refunding", "10": "cancelled", "11": "completed", "12": "cancelled",
}

// NormalizeOrderStatus 数字码归一为文本。
func NormalizeOrderStatus(s string) string {
	if // t、ok 用于本次流程后续判断的t、ok
	t, ok := OrderStatusMap[s]; ok {
		return t
	}
	switch strings.TrimSpace(s) {
	case "待付款", "处理中":
		return "processing"
	case "待发货", "已付款":
		return "pending_ship"
	case "已发货":
		return "shipped"
	case "买家已确认收货", "已收货", "确认收货成功":
		return "received"
	case "交易成功", "交易完成", "已完成":
		return "completed"
	case "退款中", "退款申请中":
		return "refunding"
	case "退款成功", "已退款":
		return "refunded"
	case "交易关闭", "已关闭":
		return "cancelled"
	case "退款关闭":
		return "unknown"
	}
	if s == "" {
		return "unknown"
	}
	return s
}

// AllTitles 取全部 item_id → item_title 映射（订单列表用）。
func (i *Items) AllTitles(ctx context.Context) (map[string]string, error) {
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := i.DB.QueryContext(ctx, `SELECT item_id, item_title FROM item_info`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// m 用于本次流程后续判断的m
	m := make(map[string]string)
	for rows.Next() {
		// id、title 用于本次流程后续判断的id、title
		var id, title sql.NullString
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		m[id.String] = title.String
	}
	return m, rows.Err()
}

// 卡券 CRUD 辅助。

// CardFull 卡券完整信息（CRUD 用）。
type CardFull struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	APIConfig    string `json:"api_config"`
	TextContent  string `json:"text_content"`
	DataContent  string `json:"data_content"`
	ImageURL     string `json:"image_url"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	DelaySeconds int    `json:"delay_seconds"`
	IsMultiSpec  bool   `json:"is_multi_spec"`
	SpecName     string `json:"spec_name"`
	SpecValue    string `json:"spec_value"`
	UserID       int64  `json:"user_id"`
}

// ExistsOwned 判断卡密组是否属于指定用户。
func (c *Cards) ExistsOwned(ctx context.Context, cardID, userID int64) (bool, error) {
	// exists 用于本次流程后续判断的exists
	var exists bool
	// err 用于本次流程后续判断的err
	err := c.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cards WHERE id=? AND user_id=?)`, cardID, userID).Scan(&exists)
	return exists, err
}

// Get 取单个卡券。
func (c *Cards) Get(ctx context.Context, cardID int64) (*CardFull, error) {
	// cf 用于本次流程后续判断的cf
	var cf CardFull
	// enabled、isMultiSpec 用于本次流程后续判断的enabled、isMultiSpec
	var enabled, isMultiSpec int
	// apiCfg、textContent、dataContent、imageURL、specName、specValue、desc 用于本次流程后续判断的apiCfg、textContent、dataContent、imageURL、specName、specValue、desc
	var apiCfg, textContent, dataContent, imageURL, specName, specValue, desc sql.NullString
	// err 用于本次流程后续判断的err
	err := c.DB.QueryRowContext(ctx,
		`SELECT id, name, type, api_config, text_content, data_content, image_url, description,
		        enabled, delay_seconds, is_multi_spec, spec_name, spec_value, user_id
		 FROM cards WHERE id=?`, cardID).Scan(
		&cf.ID, &cf.Name, &cf.Type, &apiCfg, &textContent, &dataContent, &imageURL, &desc,
		&enabled, &cf.DelaySeconds, &isMultiSpec, &specName, &specValue, &cf.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cf.APIConfig = apiCfg.String
	cf.TextContent = textContent.String
	cf.DataContent = dataContent.String
	cf.ImageURL = imageURL.String
	cf.Description = desc.String
	cf.Enabled = enabled != 0
	cf.IsMultiSpec = isMultiSpec != 0
	cf.SpecName = specName.String
	cf.SpecValue = specValue.String
	return &cf, nil
}

// AllForUser 取某用户全部卡券。
func (c *Cards) AllForUser(ctx context.Context, userID int64) ([]CardFull, error) {
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := c.DB.QueryContext(ctx,
		`SELECT id, name, type, api_config, text_content, data_content, image_url, description,
		        enabled, delay_seconds, is_multi_spec, spec_name, spec_value, user_id
		 FROM cards WHERE user_id=? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// out 用于本次流程后续判断的out
	var out []CardFull
	for rows.Next() {
		// cf 用于本次流程后续判断的cf
		var cf CardFull
		// enabled、isMultiSpec 用于本次流程后续判断的enabled、isMultiSpec
		var enabled, isMultiSpec int
		// apiCfg、textContent、dataContent、imageURL、specName、specValue、desc 用于本次流程后续判断的apiCfg、textContent、dataContent、imageURL、specName、specValue、desc
		var apiCfg, textContent, dataContent, imageURL, specName, specValue, desc sql.NullString
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&cf.ID, &cf.Name, &cf.Type, &apiCfg, &textContent, &dataContent, &imageURL, &desc,
			&enabled, &cf.DelaySeconds, &isMultiSpec, &specName, &specValue, &cf.UserID); err != nil {
			return nil, err
		}
		cf.APIConfig = apiCfg.String
		cf.TextContent = textContent.String
		cf.DataContent = dataContent.String
		cf.ImageURL = imageURL.String
		cf.Description = desc.String
		cf.Enabled = enabled != 0
		cf.IsMultiSpec = isMultiSpec != 0
		cf.SpecName = specName.String
		cf.SpecValue = specValue.String
		out = append(out, cf)
	}
	return out, rows.Err()
}

// Create 创建卡券，返回新 ID。
func (c *Cards) Create(ctx context.Context, cf *CardFull) (int64, error) {
	return insertReturningID(ctx, c.DB, c.Dialect,
		`INSERT INTO cards (name, type, api_config, text_content, data_content, image_url, description,
		    enabled, delay_seconds, is_multi_spec, spec_name, spec_value, user_id)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		cf.Name, cf.Type, nullable(cf.APIConfig), nullable(cf.TextContent), nullable(cf.DataContent),
		nullable(cf.ImageURL), nullable(cf.Description), boolToInt(cf.Enabled), cf.DelaySeconds,
		boolToInt(cf.IsMultiSpec), nullable(cf.SpecName), nullable(cf.SpecValue), cf.UserID)
}

// Update 更新卡券。
func (c *Cards) Update(ctx context.Context, cf *CardFull) error {
	// err 用于本次流程后续判断的err
	_, err := c.DB.ExecContext(ctx,
		`UPDATE cards SET name=?, type=?, api_config=?, text_content=?, data_content=?, image_url=?,
		    description=?, enabled=?, delay_seconds=?, is_multi_spec=?, spec_name=?, spec_value=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		cf.Name, cf.Type, nullable(cf.APIConfig), nullable(cf.TextContent), nullable(cf.DataContent),
		nullable(cf.ImageURL), nullable(cf.Description), boolToInt(cf.Enabled), cf.DelaySeconds,
		boolToInt(cf.IsMultiSpec), nullable(cf.SpecName), nullable(cf.SpecValue), cf.ID)
	return err
}

// Delete 删除卡券。
func (c *Cards) Delete(ctx context.Context, cardID int64) error {
	// err 用于本次流程后续判断的err
	_, err := c.DB.ExecContext(ctx, `DELETE FROM cards WHERE id=?`, cardID)
	return err
}

// nullable 封装nullable业务协调。
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
