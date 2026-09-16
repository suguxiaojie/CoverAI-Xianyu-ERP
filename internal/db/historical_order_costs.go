package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HistoricalCostCandidateRow 是不依赖当前商品或 SKU 的历史订单成本确认分组。
type HistoricalCostCandidateRow struct {
	// AccountID 是该组订单所属账号标识。
	AccountID string
	// ItemID 是订单保留的历史商品标识，可能已不在当前目录。
	ItemID string
	// ItemTitle 是商品目录或同账号聊天历史中最近保留的标题。
	ItemTitle string
	// AmountCents 是单笔历史成交金额分值。
	AmountCents int64
	// Quantity 是单笔订单购买数量。
	Quantity int
	// OrderCount 是当前账号、商品、金额和数量组合下待确认订单数。
	OrderCount int
}

// ListHistoricalCostCandidates 返回缺少精确快照和人工覆盖的有效订单分组；只读本地非敏感订单与标题。
func (costs *OrderCosts) ListHistoricalCostCandidates(ctx context.Context, userID int64, accountID, startAt, endBefore string, statuses []string) ([]HistoricalCostCandidateRow, error) {
	if costs == nil || costs.DB == nil {
		return nil, errors.New("订单成本存储未初始化")
	}
	// conditions 保存用户所有权、可选账号、有效状态和未覆盖条件。
	conditions := []string{"o.deleted_at IS NULL", "s.order_id IS NULL", "h.order_id IS NULL", "EXISTS(SELECT 1 FROM cookies c WHERE c.id=o.cookie_id AND c.user_id=?)"}
	// args 保存参数化查询的用户、账号和状态值。
	args := []any{userID}
	if strings.TrimSpace(accountID) != "" {
		conditions = append(conditions, "o.cookie_id=?")
		args = append(args, strings.TrimSpace(accountID))
	}
	// createdAtExpression、boundaryExpression 是当前方言比较历史订单时间的表达式和边界占位。
	createdAtExpression, boundaryExpression := "o.created_at", "?"
	if costs.Dialect == DialectSQLite {
		createdAtExpression, boundaryExpression = "datetime(o.created_at)", "datetime(?)"
	}
	if startAt != "" {
		conditions = append(conditions, createdAtExpression+" >= "+boundaryExpression)
		args = append(args, startAt)
	}
	if endBefore != "" {
		conditions = append(conditions, createdAtExpression+" < "+boundaryExpression)
		args = append(args, endBefore)
	}
	if len(statuses) > 0 {
		// placeholders 是有效订单状态使用的占位符。
		placeholders := make([]string, len(statuses))
		// index、status 是当前写入占位符和参数的状态序号与值。
		for index, status := range statuses {
			placeholders[index] = "?"
			args = append(args, status)
		}
		conditions = append(conditions, "o.order_status IN ("+strings.Join(placeholders, ",")+")")
	}
	// rows、queryErr 是全部未覆盖订单及历史标题的参数化查询结果。
	rows, queryErr := costs.DB.QueryContext(ctx, `SELECT o.cookie_id,COALESCE(o.item_id,''),
		COALESCE(NULLIF(i.item_title,''),(SELECT NULLIF(cs.item_title,'') FROM chat_sessions cs WHERE cs.cookie_id=o.cookie_id AND cs.item_id=o.item_id ORDER BY cs.updated_at DESC LIMIT 1),''),
		COALESCE(o.amount,''),COALESCE(o.quantity,'')
		FROM orders o
		LEFT JOIN item_info i ON i.cookie_id=o.cookie_id AND i.item_id=o.item_id
		LEFT JOIN order_cost_snapshots s ON s.order_id=o.order_id
		LEFT JOIN historical_order_cost_overrides h ON h.order_id=o.order_id
		WHERE `+strings.Join(conditions, " AND ")+`
		ORDER BY o.cookie_id,o.item_id,o.created_at,o.order_id`, args...)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// grouped 按账号、商品、历史成交金额和购买数量累计待确认订单。
	grouped := make(map[string]HistoricalCostCandidateRow)
	for rows.Next() {
		// row 是当前订单的账号、商品和解析后分组字段。
		var row HistoricalCostCandidateRow
		// amountText、quantityText 是持久化的历史金额和数量文本。
		var amountText, quantityText string
		if // scanErr 是当前历史订单字段读取错误。
		scanErr := rows.Scan(&row.AccountID, &row.ItemID, &row.ItemTitle, &amountText, &quantityText); scanErr != nil {
			return nil, scanErr
		}
		// amountCents、amountOK 是历史成交金额的安全分值和有效标记。
		amountCents, amountOK := historicalAmountCents(amountText)
		// quantity、quantityErr 是历史订单的正整数购买数量和解析错误。
		quantity, quantityErr := strconv.Atoi(strings.TrimSpace(quantityText))
		if !amountOK || quantityErr != nil || quantity <= 0 {
			continue
		}
		row.AmountCents, row.Quantity, row.OrderCount = amountCents, quantity, 1
		// key 是不会把不同账号、商品、金额或数量混合的稳定分组键。
		key := fmt.Sprintf("%s:%s:%d:%d", row.AccountID, row.ItemID, row.AmountCents, row.Quantity)
		// current 是当前分组已累计的订单数和标题。
		current := grouped[key]
		if current.OrderCount == 0 {
			grouped[key] = row
			continue
		}
		current.OrderCount++
		if current.ItemTitle == "" && row.ItemTitle != "" {
			current.ItemTitle = row.ItemTitle
		}
		grouped[key] = current
	}
	if // rowsErr 是历史订单结果遍历期间的延迟错误。
	rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	// candidates 是按账号、商品、金额和数量稳定排序的人工历史成本分组。
	candidates := make([]HistoricalCostCandidateRow, 0, len(grouped))
	// candidate 是当前从分组索引收集的历史成本候选。
	for _, candidate := range grouped {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].AccountID != candidates[right].AccountID {
			return candidates[left].AccountID < candidates[right].AccountID
		}
		if candidates[left].ItemID != candidates[right].ItemID {
			return candidates[left].ItemID < candidates[right].ItemID
		}
		if candidates[left].AmountCents != candidates[right].AmountCents {
			return candidates[left].AmountCents < candidates[right].AmountCents
		}
		return candidates[left].Quantity < candidates[right].Quantity
	})
	return candidates, nil
}

// ConfirmHistoricalCostGroup 为精确账号、商品、成交金额和数量分组原子生成订单级不可变人工成本覆盖。
func (costs *OrderCosts) ConfirmHistoricalCostGroup(ctx context.Context, userID int64, accountID, itemID string, amountCents int64, quantity int, startAt, endBefore string, unitCostCents int64, statuses []string) (int, error) {
	if costs == nil || costs.DB == nil {
		return 0, errors.New("订单成本存储未初始化")
	}
	if userID <= 0 || strings.TrimSpace(accountID) == "" || amountCents < 0 || quantity <= 0 || unitCostCents < 0 {
		return 0, errors.New("历史成本确认参数无效")
	}
	// transaction 保证同一历史分组的全部订单覆盖一次提交。
	transaction, beginErr := costs.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return 0, beginErr
	}
	defer transaction.Rollback()
	// owned 是当前账号是否属于请求用户的存在性结果。
	var owned int
	if // ownershipErr 是账号所有权查询错误。
	ownershipErr := transaction.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cookies WHERE id=? AND user_id=?)`, strings.TrimSpace(accountID), userID).Scan(&owned); ownershipErr != nil || owned == 0 {
		if ownershipErr != nil {
			return 0, ownershipErr
		}
		return 0, ErrForbidden
	}
	// conditions 保存当前分组、有效状态和未覆盖条件。
	conditions := []string{"o.cookie_id=?", "COALESCE(o.item_id,'')=?", "o.deleted_at IS NULL", "s.order_id IS NULL", "h.order_id IS NULL"}
	// args 保存账号、商品和有效状态查询参数。
	args := []any{strings.TrimSpace(accountID), itemID}
	// createdAtExpression、boundaryExpression 是确认时重新限定 Dashboard 日期范围的方言表达式。
	createdAtExpression, boundaryExpression := "o.created_at", "?"
	if costs.Dialect == DialectSQLite {
		createdAtExpression, boundaryExpression = "datetime(o.created_at)", "datetime(?)"
	}
	if startAt != "" {
		conditions = append(conditions, createdAtExpression+" >= "+boundaryExpression)
		args = append(args, startAt)
	}
	if endBefore != "" {
		conditions = append(conditions, createdAtExpression+" < "+boundaryExpression)
		args = append(args, endBefore)
	}
	if len(statuses) > 0 {
		// placeholders 是人工确认仅允许有效成交状态的占位符。
		placeholders := make([]string, len(statuses))
		// index、status 是当前写入查询的状态序号与值。
		for index, status := range statuses {
			placeholders[index] = "?"
			args = append(args, status)
		}
		conditions = append(conditions, "o.order_status IN ("+strings.Join(placeholders, ",")+")")
	}
	// rows、queryErr 是服务端重新校验后尚未覆盖的分组订单。
	rows, queryErr := transaction.QueryContext(ctx, `SELECT o.order_id,COALESCE(o.amount,''),COALESCE(o.quantity,'') FROM orders o
		LEFT JOIN order_cost_snapshots s ON s.order_id=o.order_id
		LEFT JOIN historical_order_cost_overrides h ON h.order_id=o.order_id
		WHERE `+strings.Join(conditions, " AND ")+` ORDER BY o.order_id`, args...)
	if queryErr != nil {
		return 0, queryErr
	}
	// orderIDs 是金额和数量均精确匹配当前确认分组的订单标识。
	orderIDs := make([]string, 0)
	for rows.Next() {
		// orderID、amountText、quantityText 是当前订单的标识、历史金额和数量文本。
		var orderID, amountText, quantityText string
		if // scanErr 是当前订单确认字段读取错误。
		scanErr := rows.Scan(&orderID, &amountText, &quantityText); scanErr != nil {
			_ = rows.Close()
			return 0, scanErr
		}
		// parsedAmount、amountOK 是订单金额的安全分值和有效标记。
		parsedAmount, amountOK := historicalAmountCents(amountText)
		// parsedQuantity、quantityErr 是订单购买数量和解析错误。
		parsedQuantity, quantityErr := strconv.Atoi(strings.TrimSpace(quantityText))
		if amountOK && parsedAmount == amountCents && quantityErr == nil && parsedQuantity == quantity {
			orderIDs = append(orderIDs, orderID)
		}
	}
	if // rowsErr 是分组订单遍历期间的延迟错误。
	rowsErr := rows.Err(); rowsErr != nil {
		_ = rows.Close()
		return 0, rowsErr
	}
	if // closeErr 是人工成本确认结果集释放错误。
	closeErr := rows.Close(); closeErr != nil {
		return 0, closeErr
	}
	// insertQuery 使用方言兼容的主键冲突忽略保持订单成本不可变。
	insertQuery := dialectInsertIgnorePrefix(costs.Dialect) + ` INTO historical_order_cost_overrides(order_id,cookie_id,item_id,unit_cost_cents,quantity,match_source,captured_at) VALUES(?,?,?,?,?,?,?)` + dialectInsertIgnore(costs.Dialect, []string{"order_id"})
	// capturedAt 是当前整组人工确认共用的 Unix 秒时间。
	capturedAt := time.Now().Unix()
	// matched 是排除冲突后实际新增的订单成本覆盖数。
	matched := 0
	// orderID 是当前准备写入不可变历史成本的订单标识。
	for _, orderID := range orderIDs {
		// result、insertErr 是当前订单覆盖的写入结果和错误。
		result, insertErr := transaction.ExecContext(ctx, insertQuery, orderID, strings.TrimSpace(accountID), itemID, unitCostCents, quantity, "manual_historical_cost", capturedAt)
		if insertErr != nil {
			return 0, insertErr
		}
		// affected、affectedErr 是冲突忽略后实际新增行数和读取错误。
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return 0, affectedErr
		}
		matched += int(affected)
	}
	if // commitErr 是整组不可变覆盖的事务提交错误。
	commitErr := transaction.Commit(); commitErr != nil {
		return 0, commitErr
	}
	return matched, nil
}
