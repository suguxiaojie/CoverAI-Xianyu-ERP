package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

// OrderCosts 负责把可靠 SKU 成本锁定为订单级历史快照。
type OrderCosts struct {
	// DB 是成本快照持久化连接池。
	DB *sql.DB
	// Dialect 决定插入忽略语法。
	Dialect Dialect
}

// Exists 判断订单是否已经生成不可变成本快照，供历史补全严格增量筛选。
func (costs *OrderCosts) Exists(ctx context.Context, orderID string) (bool, error) {
	if costs == nil || costs.DB == nil {
		return false, errors.New("订单成本快照存储未初始化")
	}
	// exists 是订单成本快照存在性查询结果。
	var exists int
	// queryErr 是快照存在性查询错误。
	queryErr := costs.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM order_cost_snapshots WHERE order_id=?)`, orderID).Scan(&exists)
	return exists != 0, queryErr
}

// ConfirmManualGroup 为用户确认的商品金额分组原子生成不可变成本快照；accountID 为空时保留全账号行为，否则与用户所有权共同限定 SKU。
func (costs *OrderCosts) ConfirmManualGroup(ctx context.Context, userID int64, accountID, itemID string, amountCents int64, expectedQuantity int, startAt, endBefore, skuID string, customUnitCostCents *int64) (int, string, error) {
	if costs == nil || costs.DB == nil {
		return 0, "", errors.New("订单成本快照存储未初始化")
	}
	// transaction 保证同一金额分组的全部成本快照一次提交。
	transaction, beginErr := costs.DB.BeginTx(ctx, nil)
	if beginErr != nil {
		return 0, "", beginErr
	}
	defer transaction.Rollback()
	// cookieID、skuItemID、unitCostCents、priceCents 是人工选择 SKU 的归属和当前金额字段。
	var cookieID, skuItemID string
	// unitCostCents、priceCents 分别是人工选择 SKU 的当前成本和标准售价分值。
	var unitCostCents, priceCents int64
	// skuErr 是经当前用户账号归属校验后的 SKU 查询错误。
	skuErr := transaction.QueryRowContext(ctx, `SELECT s.cookie_id,s.item_id,s.cost_cents,s.price_cents FROM item_skus s JOIN cookies c ON c.id=s.cookie_id WHERE c.user_id=? AND s.item_id=? AND s.sku_id=? AND s.sku_id<>? AND (?='' OR s.cookie_id=?) AND s.deleted_at IS NULL AND s.cost_cents IS NOT NULL`, userID, itemID, skuID, DefaultItemSKUID, accountID, accountID).Scan(&cookieID, &skuItemID, &unitCostCents, &priceCents)
	if skuErr != nil {
		return 0, "", skuErr
	}
	if customUnitCostCents != nil {
		if *customUnitCostCents < 0 {
			return 0, "", errors.New("历史单件成本不能为负数")
		}
		unitCostCents = *customUnitCostCents
	}
	// source 区分标准售价完全一致和人工确认的改价候选。
	source := "manual_adjusted_price"
	if priceCents == amountCents {
		source = "manual_exact_price"
	}
	if customUnitCostCents != nil {
		source = "manual_custom_cost"
	}
	// conditions 是同账号、商品、有效状态、未覆盖和当前 Dashboard 日期范围条件。
	conditions := []string{"o.cookie_id=?", "o.item_id=?", "o.deleted_at IS NULL", "o.order_status IN ('pending_ship','paid','2','shipped','3','received','completed','4','11')", "s.order_id IS NULL", "h.order_id IS NULL"}
	// queryArgs 是当前 SKU 人工确认范围的账号、商品和日期参数。
	queryArgs := []any{cookieID, skuItemID}
	// createdAtExpression、boundaryExpression 是当前方言比较历史订单日期的表达式和边界占位。
	createdAtExpression, boundaryExpression := "o.created_at", "?"
	if costs.Dialect == DialectSQLite {
		createdAtExpression, boundaryExpression = "datetime(o.created_at)", "datetime(?)"
	}
	if startAt != "" {
		conditions = append(conditions, createdAtExpression+" >= "+boundaryExpression)
		queryArgs = append(queryArgs, startAt)
	}
	if endBefore != "" {
		conditions = append(conditions, createdAtExpression+" < "+boundaryExpression)
		queryArgs = append(queryArgs, endBefore)
	}
	// rows 是经完整范围重新校验后仍缺成本的历史订单候选。
	rows, queryErr := transaction.QueryContext(ctx, `SELECT o.order_id,COALESCE(o.quantity,''),COALESCE(o.amount,'') FROM orders o LEFT JOIN order_cost_snapshots s ON s.order_id=o.order_id LEFT JOIN historical_order_cost_overrides h ON h.order_id=o.order_id WHERE `+strings.Join(conditions, " AND "), queryArgs...)
	if queryErr != nil {
		return 0, "", queryErr
	}
	defer rows.Close()
	// matched 保存金额精确属于当前人工确认分组并成功插入的订单数。
	matched := 0
	// insertQuery 使用订单主键冲突忽略保持快照不可变。
	insertQuery := dialectInsertIgnorePrefix(costs.Dialect) + ` INTO order_cost_snapshots(order_id,cookie_id,item_id,sku_id,unit_cost_cents,quantity,match_source,captured_at) VALUES(?,?,?,?,?,?,?,?)` + dialectInsertIgnore(costs.Dialect, []string{"order_id"})
	for rows.Next() {
		// orderID、quantityText、amountText 是当前待重新验证的历史订单字段。
		var orderID, quantityText, amountText string
		if // scanErr 是当前人工候选订单读取错误。
		scanErr := rows.Scan(&orderID, &quantityText, &amountText); scanErr != nil {
			return 0, "", scanErr
		}
		// parsedAmountCents、amountOK 是订单金额安全转换后的分值和有效标记。
		parsedAmountCents, amountOK := historicalAmountCents(amountText)
		// quantity、quantityErr 是订单购买数量及解析错误。
		quantity, quantityErr := strconv.Atoi(strings.TrimSpace(quantityText))
		if !amountOK || parsedAmountCents != amountCents || quantityErr != nil || quantity <= 0 || quantity != expectedQuantity {
			continue
		}
		// result、insertErr 是当前人工成本快照插入结果。
		result, insertErr := transaction.ExecContext(ctx, insertQuery, orderID, cookieID, skuItemID, skuID, unitCostCents, quantity, source, time.Now().Unix())
		if insertErr != nil {
			return 0, "", insertErr
		}
		// affected、affectedErr 是冲突忽略后实际新增的快照数量。
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return 0, "", affectedErr
		}
		matched += int(affected)
	}
	if // rowsErr 是人工候选订单遍历期间产生的延迟错误。
	rowsErr := rows.Err(); rowsErr != nil {
		return 0, "", rowsErr
	}
	if // commitErr 是人工成本快照事务提交错误。
	commitErr := transaction.Commit(); commitErr != nil {
		return 0, "", commitErr
	}
	return matched, source, nil
}

// historicalAmountCents 把普通人民币金额文本安全转换为分值，非法格式保持未匹配。
func historicalAmountCents(raw string) (int64, bool) {
	// normalized 是移除人民币符号和千分位后的金额文本。
	normalized := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "¥", ""), ",", ""))
	// amount、parseErr 是金额浮点值和解析错误。
	amount, parseErr := strconv.ParseFloat(normalized, 64)
	if parseErr != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return 0, false
	}
	return int64(math.Round(amount * 100)), true
}

// orderCostCandidate 是当前商品可用于精确匹配的 SKU 成本。
type orderCostCandidate struct {
	// skuID 是平台 SKU 或本地隐式 SKU 标识。
	skuID string
	// unitCostCents 是当前单件成本分值。
	unitCostCents int64
	// propertiesJSON 是平台 SKU 的规格数组。
	propertiesJSON string
	// localOnly 表示候选是单规格本地隐式成本行。
	localOnly bool
}

// orderCostProperty 是用于精确匹配订单规格的最小 SKU 属性。
type orderCostProperty struct {
	// Name 是规格名称。
	Name string `json:"name"`
	// Value 是规格值。
	Value string `json:"value"`
}

// CaptureExactTx 在订单写入事务中为可靠匹配创建不可变成本快照；信息不足时安全跳过。
func (costs *OrderCosts) CaptureExactTx(ctx context.Context, transaction *sql.Tx, orderID string) error {
	if costs == nil || transaction == nil {
		return errors.New("订单成本快照存储未初始化")
	}
	// snapshotExists 表示订单成本已经锁定，后续 SKU 成本变化不得重写历史。
	var snapshotExists int
	// snapshotErr 是成本快照存在性查询错误。
	snapshotErr := transaction.QueryRowContext(ctx, `SELECT 1 FROM order_cost_snapshots WHERE order_id=?`, orderID).Scan(&snapshotExists)
	if snapshotErr == nil {
		return nil
	}
	if !errors.Is(snapshotErr, sql.ErrNoRows) {
		return snapshotErr
	}
	// cookieID、itemID、specName、specValue、quantityText、status 保存最终订单匹配字段。
	var cookieID, itemID, specName, specValue, quantityText, status string
	// multiSpec 保存商品是否要求规格精确匹配。
	var multiSpec int
	// orderErr 是当前订单和商品匹配字段查询错误。
	orderErr := transaction.QueryRowContext(ctx, `SELECT COALESCE(o.cookie_id,''),COALESCE(o.item_id,''),COALESCE(o.spec_name,''),
		COALESCE(o.spec_value,''),COALESCE(o.quantity,''),COALESCE(o.order_status,''),COALESCE(i.is_multi_spec,0)
		FROM orders o LEFT JOIN item_info i ON i.cookie_id=o.cookie_id AND i.item_id=o.item_id AND i.deleted_at IS NULL
		WHERE o.order_id=? AND o.deleted_at IS NULL`, orderID).Scan(&cookieID, &itemID, &specName, &specValue, &quantityText, &status, &multiSpec)
	if errors.Is(orderErr, sql.ErrNoRows) {
		return nil
	}
	if orderErr != nil {
		return orderErr
	}
	if !profitEligibleOrderStatus(status) || strings.TrimSpace(cookieID) == "" || strings.TrimSpace(itemID) == "" {
		return nil
	}
	// rows 是当前商品所有已配置成本的有效 SKU。
	rows, queryErr := transaction.QueryContext(ctx, `SELECT sku_id,properties_json,cost_cents FROM item_skus
		WHERE cookie_id=? AND item_id=? AND deleted_at IS NULL AND cost_cents IS NOT NULL ORDER BY sort_order,id`, cookieID, itemID)
	if queryErr != nil {
		return queryErr
	}
	defer rows.Close()
	// candidates 保存当前商品可匹配成本。
	candidates := make([]orderCostCandidate, 0)
	for rows.Next() {
		// candidate 是当前扫描到的 SKU 成本。
		var candidate orderCostCandidate
		if // scanErr 是当前 SKU 成本候选读取错误。
		scanErr := rows.Scan(&candidate.skuID, &candidate.propertiesJSON, &candidate.unitCostCents); scanErr != nil {
			return scanErr
		}
		candidate.localOnly = candidate.skuID == DefaultItemSKUID
		candidates = append(candidates, candidate)
	}
	if // rowsErr 是成本候选遍历期间产生的延迟错误。
	rowsErr := rows.Err(); rowsErr != nil {
		return rowsErr
	}
	// matched、source 是唯一可靠匹配的 SKU 成本和匹配来源。
	matched, source, matchedOK := matchOrderCostCandidate(candidates, multiSpec != 0, specName, specValue)
	if !matchedOK {
		return nil
	}
	// quantity 是订单购买数量；缺失或非法时拒绝快照，避免低估成本。
	quantity, quantityErr := strconv.Atoi(strings.TrimSpace(quantityText))
	if quantityErr != nil || quantity <= 0 {
		return nil
	}
	// insertQuery 使用冲突忽略保证同一订单成本快照不可变。
	insertQuery := dialectInsertIgnorePrefix(costs.Dialect) + ` INTO order_cost_snapshots(order_id,cookie_id,item_id,sku_id,unit_cost_cents,quantity,match_source,captured_at)
		VALUES(?,?,?,?,?,?,?,?)` + dialectInsertIgnore(costs.Dialect, []string{"order_id"})
	// insertErr 是不可变订单成本快照插入错误。
	_, insertErr := transaction.ExecContext(ctx, insertQuery, orderID, cookieID, itemID, matched.skuID, matched.unitCostCents, quantity, source, time.Now().Unix())
	return insertErr
}

// matchOrderCostCandidate 按单规格默认、唯一单 SKU 或规格精确值匹配成本；歧义时返回 false。
func matchOrderCostCandidate(candidates []orderCostCandidate, multiSpec bool, specName, specValue string) (orderCostCandidate, string, bool) {
	if !multiSpec {
		// defaults 保存单规格商品的本地隐式成本。
		defaults := make([]orderCostCandidate, 0, 1)
		// platform 保存单规格商品的真实平台 SKU 成本。
		platform := make([]orderCostCandidate, 0, 1)
		// candidate 是当前按单规格规则分类的成本候选。
		for _, candidate := range candidates {
			if candidate.localOnly {
				defaults = append(defaults, candidate)
			} else {
				platform = append(platform, candidate)
			}
		}
		if len(defaults) == 1 {
			return defaults[0], "single_local_default", true
		}
		if len(defaults) == 0 && len(platform) == 1 {
			return platform[0], "single_platform_sku", true
		}
		return orderCostCandidate{}, "", false
	}
	// normalizedName、normalizedValue 是订单规格的精确匹配文本。
	normalizedName, normalizedValue := strings.TrimSpace(specName), strings.TrimSpace(specValue)
	if normalizedValue == "" {
		return orderCostCandidate{}, "", false
	}
	// matched 保存规格名称和值完全匹配的候选。
	matched := make([]orderCostCandidate, 0, 1)
	// candidate 是当前尝试匹配订单规格的 SKU 成本候选。
	for _, candidate := range candidates {
		if candidate.localOnly {
			continue
		}
		// properties 是当前 SKU 的销售规格。
		var properties []orderCostProperty
		if // jsonErr 是当前 SKU 规格属性解析错误；损坏候选不得参与匹配。
		jsonErr := json.Unmarshal([]byte(candidate.propertiesJSON), &properties); jsonErr != nil {
			continue
		}
		// property 是当前 SKU 中待与订单规格比较的属性。
		for _, property := range properties {
			if strings.TrimSpace(property.Value) == normalizedValue && (normalizedName == "" || strings.TrimSpace(property.Name) == normalizedName) {
				matched = append(matched, candidate)
				break
			}
		}
	}
	if len(matched) != 1 {
		return orderCostCandidate{}, "", false
	}
	return matched[0], "exact_spec", true
}

// profitEligibleOrderStatus 判断订单当前是否仍属于成交额和预计商品毛利范围。
func profitEligibleOrderStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "pending_ship", "paid", "2", "shipped", "3", "received", "completed", "4", "11":
		return true
	default:
		return false
	}
}
