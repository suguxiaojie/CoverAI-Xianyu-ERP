package db

import (
	"context"
	"database/sql"
	"strings"
)

// OrderSettlementSummary 保存当前筛选范围内已发货订单的数量与实付总额，金额单位为人民币分。
type OrderSettlementSummary struct {
	// OrderCount 是命中已发货状态和其他筛选条件的订单数。
	OrderCount int
	// GrossAmountCents 是所有命中订单先累加后的实付总额，单位为人民币分。
	GrossAmountCents int64
}

// SettlementSummaryForUser 按用户、账号、时间、金额和搜索条件汇总已发货订单，不受页面当前状态标签影响。
func (o *Orders) SettlementSummaryForUser(ctx context.Context, filter OrderListFilter) (OrderSettlementSummary, error) {
	// where 保存待结算汇总必须同时满足的用户隔离和业务筛选条件。
	where := []string{"c.user_id=?", "o.deleted_at IS NULL"}
	// args 保存与 where 顺序一致的参数化查询值。
	args := []any{filter.UserID}
	if filter.CookieID != "" {
		where = append(where, "o.cookie_id=?")
		args = append(args, filter.CookieID)
	}
	// statuses 保存“已发货”的规范状态和历史数字状态，禁止把已收货、已完成或退款订单计入待结算。
	statuses := normalizedStatusCandidates("shipped")
	// statusPlaceholders 保存已发货状态集合的参数占位符。
	statusPlaceholders := make([]string, 0, len(statuses))
	// status 是当前待加入查询的规范或历史已发货状态。
	for _, status := range statuses {
		statusPlaceholders = append(statusPlaceholders, "?")
		args = append(args, status)
	}
	where = append(where, "o.order_status IN ("+strings.Join(statusPlaceholders, ",")+")")
	if !filter.CreatedFrom.IsZero() {
		where = append(where, "o.created_at>=?")
		args = append(args, orderListTimeArgument(o.Dialect, filter.CreatedFrom))
	}
	if !filter.CreatedTo.IsZero() {
		where = append(where, "o.created_at<?")
		args = append(args, orderListTimeArgument(o.Dialect, filter.CreatedTo))
	}
	// amountCentsExpression 把规范金额转换为整数分，供汇总和金额范围使用同一精度口径。
	amountCentsExpression := orderListAmountCentsExpression(o.Dialect)
	if filter.MinAmountCents != nil {
		where = append(where, amountCentsExpression+">=?")
		args = append(args, *filter.MinAmountCents)
	}
	if filter.MaxAmountCents != nil {
		where = append(where, amountCentsExpression+"<=?")
		args = append(args, *filter.MaxAmountCents)
	}
	if // search 是规范化后的订单号、商品或买家搜索词。
	search := strings.ToLower(strings.TrimSpace(filter.Search)); search != "" {
		// pattern 是参数化模糊查询模式；沿用订单列表现有搜索字段。
		pattern := "%" + search + "%"
		where = append(where, `(LOWER(o.order_id) LIKE ? OR LOWER(COALESCE(o.item_id,'')) LIKE ?
			OR LOWER(COALESCE(o.buyer_id,'')) LIKE ? OR LOWER(COALESCE(i.item_title,'')) LIKE ?
			OR LOWER(COALESCE(o.receiver_name,'')) LIKE ? OR LOWER(COALESCE(o.receiver_phone,'')) LIKE ?
			OR EXISTS(SELECT 1 FROM chat_sessions search_session WHERE search_session.cookie_id=o.cookie_id
			  AND ((COALESCE(o.chat_id,'')<>'' AND search_session.chat_id=o.chat_id)
			    OR (COALESCE(o.buyer_id,'')<>'' AND search_session.buyer_id=o.buyer_id))
			  AND LOWER(COALESCE(search_session.buyer_name,'')) LIKE ?))`)
		// index 是七个订单搜索字段当前追加参数的位置。
		for index := 0; index < 7; index++ {
			args = append(args, pattern)
		}
	}
	// summary 是当前用户筛选范围内的已发货订单聚合结果。
	var summary OrderSettlementSummary
	// grossAmount 保存数据库聚合出的可空整数分；无订单时转换为零。
	var grossAmount sql.NullInt64
	// queryErr 是待结算聚合查询失败原因。
	queryErr := o.DB.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(COALESCE(`+amountCentsExpression+`,0)),0)
		FROM orders o
		JOIN cookies c ON c.id=o.cookie_id
		LEFT JOIN item_info i ON i.cookie_id=o.cookie_id AND i.item_id=o.item_id
		WHERE `+strings.Join(where, " AND "), args...).Scan(&summary.OrderCount, &grossAmount)
	if queryErr != nil {
		return OrderSettlementSummary{}, queryErr
	}
	summary.GrossAmountCents = grossAmount.Int64
	return summary, nil
}
