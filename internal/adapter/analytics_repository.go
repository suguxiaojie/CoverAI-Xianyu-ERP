package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	analyticsapp "xianyu-go/internal/application/analytics"
	"xianyu-go/internal/db"
)

// AnalyticsRepository 将 Store 的只读查询能力适配为订单分析应用 Port。
type AnalyticsRepository struct {
	// store 保存数据库聚合入口，仅由该基础设施适配器访问。
	store *db.Store
}

// QueryManualCostCandidates 返回仍缺成本的多规格历史订单金额分组和全部可选成本 SKU。
func (r *AnalyticsRepository) QueryManualCostCandidates(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.ManualCostCandidateRecord, error) {
	// where、args、clean 和 amountFilter 是安全金额分组查询条件。
	where, args, clean, amountFilter := r.orderScope(filter, "orders.amount")
	// rows、queryErr 是未覆盖历史订单的账号、商品、金额和数量分组。
	rows, queryErr := r.store.Analytics.QueryContext(ctx, `SELECT orders.cookie_id,orders.item_id,COALESCE(i.item_title,''),ROUND((`+clean+`)*100),COALESCE(orders.quantity,'1'),COUNT(*) FROM orders JOIN item_info i ON i.cookie_id=orders.cookie_id AND i.item_id=orders.item_id AND i.deleted_at IS NULL LEFT JOIN order_cost_snapshots s ON s.order_id=orders.order_id LEFT JOIN historical_order_cost_overrides h ON h.order_id=orders.order_id `+where+amountFilter+` AND s.order_id IS NULL AND h.order_id IS NULL AND (SELECT COUNT(*) FROM item_skus k WHERE k.cookie_id=orders.cookie_id AND k.item_id=orders.item_id AND k.sku_id<>? AND k.deleted_at IS NULL AND k.cost_cents IS NOT NULL)>1 GROUP BY orders.cookie_id,orders.item_id,i.item_title,ROUND((`+clean+`)*100),orders.quantity ORDER BY orders.cookie_id,orders.item_id,ROUND((`+clean+`)*100),orders.quantity`, append(args, db.DefaultItemSKUID)...)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// records 是按商品和成交金额分组的人工候选原始记录。
	records := make([]analyticsapp.ManualCostCandidateRecord, 0)
	for rows.Next() {
		// record 是当前历史订单金额分组。
		var record analyticsapp.ManualCostCandidateRecord
		// quantityText 是当前历史分组持久化的购买数量。
		var quantityText string
		if // scanErr 是当前人工候选金额分组读取错误。
		scanErr := rows.Scan(&record.AccountID, &record.ItemID, &record.ItemTitle, &record.AmountCents, &quantityText, &record.OrderCount); scanErr != nil {
			return nil, scanErr
		}
		// quantity、quantityErr 是历史分组的正整数购买数量和解析错误。
		quantity, quantityErr := strconv.Atoi(strings.TrimSpace(quantityText))
		if quantityErr != nil || quantity <= 0 {
			continue
		}
		record.Quantity = quantity
		records = append(records, record)
	}
	if // rowsErr 是人工候选分组遍历期间产生的延迟错误。
	rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	// skuRows、skuErr 是当前用户全部有效 SKU 和本地成本。
	skuRows, skuErr := r.store.Items.ListSKUsForUser(ctx, filter.UserID, filter.AccountID)
	if skuErr != nil {
		return nil, skuErr
	}
	// skusByItem 按商品收集人工可选的已配置成本平台 SKU。
	skusByItem := make(map[string][]analyticsapp.ManualCostSKU)
	// skuRow 是当前转换为人工候选选项的本地 SKU。
	for _, skuRow := range skuRows {
		if skuRow.SKUID == db.DefaultItemSKUID || skuRow.CostCents == nil {
			continue
		}
		// properties 是当前 SKU 的规格属性展示数组。
		var properties []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		_ = json.Unmarshal([]byte(skuRow.PropertiesJSON), &properties)
		// labels 是规格名称和值组合后的展示片段。
		labels := make([]string, 0, len(properties))
		// property 是当前 SKU 的一个规格属性。
		for _, property := range properties {
			if strings.TrimSpace(property.Value) != "" {
				labels = append(labels, strings.TrimSpace(property.Value))
			}
		}
		// label 是人工确认界面显示的规格名称。
		label := strings.Join(labels, " / ")
		if label == "" {
			label = fmt.Sprintf("SKU %s", skuRow.SKUID)
		}
		// itemKey 保证同商品标识在不同账号下不会共享 SKU 候选。
		itemKey := skuRow.CookieID + "\x00" + skuRow.ItemID
		skusByItem[itemKey] = append(skusByItem[itemKey], analyticsapp.ManualCostSKU{SKUID: skuRow.SKUID, Label: label, PriceCents: skuRow.PriceCents, CostCents: *skuRow.CostCents})
	}
	// historicalByGroup 保存同商品同成交金额最近一次人工确认的 SKU 和历史成本。
	historicalByGroup := make(map[string]struct {
		// skuID 是最近人工确认的 SKU。
		skuID string
		// unitCostCents 是最近人工确认的历史单件成本分值。
		unitCostCents int64
	})
	// historicalAmount 是跨方言清洗人工快照关联订单金额的表达式。
	historicalAmount := amountExpression(r.store.Orders.Dialect, "o.amount")
	// historicalRows、historicalErr 是用户全部人工快照按最新捕获时间排序的查询结果。
	// historicalConditions 保存历史 SKU 确认复用时的用户、账号和日期范围。
	historicalConditions := []string{"s.match_source IN ('manual_exact_price','manual_adjusted_price','manual_custom_cost')", "EXISTS(SELECT 1 FROM cookies c WHERE c.id=s.cookie_id AND c.user_id=?)", "(?='' OR s.cookie_id=?)", historicalAmount + " IS NOT NULL"}
	// historicalArgs 是历史 SKU 确认复用查询的参数值。
	historicalArgs := []any{filter.UserID, filter.AccountID, filter.AccountID}
	// createdAtExpression、boundaryExpression 是按方言比较订单时间的表达式和边界占位。
	createdAtExpression, boundaryExpression := "o.created_at", "?"
	if r.store.Orders.Dialect == db.DialectSQLite {
		createdAtExpression, boundaryExpression = "datetime(o.created_at)", "datetime(?)"
	}
	if filter.StartAt != "" {
		historicalConditions = append(historicalConditions, createdAtExpression+" >= "+boundaryExpression)
		historicalArgs = append(historicalArgs, filter.StartAt)
	}
	if filter.EndBefore != "" {
		historicalConditions = append(historicalConditions, createdAtExpression+" < "+boundaryExpression)
		historicalArgs = append(historicalArgs, filter.EndBefore)
	}
	// historicalRows、historicalErr 是当前 Dashboard 范围内的历史 SKU 确认记录。
	historicalRows, historicalErr := r.store.Analytics.QueryContext(ctx, `SELECT s.cookie_id,s.item_id,ROUND((`+historicalAmount+`)*100),s.quantity,s.sku_id,s.unit_cost_cents FROM order_cost_snapshots s JOIN orders o ON o.order_id=s.order_id WHERE `+strings.Join(historicalConditions, " AND ")+` ORDER BY s.captured_at DESC,s.order_id DESC`, historicalArgs...)
	if historicalErr != nil {
		return nil, historicalErr
	}
	for historicalRows.Next() {
		// historicalAccountID、itemID、skuID、amountValue、quantity、unitCostCents 是当前历史人工快照的精确匹配字段。
		var historicalAccountID, itemID, skuID string
		// amountValue 是历史订单成交金额转换后的分值数值。
		var amountValue float64
		// unitCostCents 是最近人工确认的历史单件成本分值。
		var unitCostCents int64
		// quantity 是历史人工快照锁定的购买数量。
		var quantity int
		if // scanErr 是当前人工快照复用记录读取错误。
		scanErr := historicalRows.Scan(&historicalAccountID, &itemID, &amountValue, &quantity, &skuID, &unitCostCents); scanErr != nil {
			_ = historicalRows.Close()
			return nil, scanErr
		}
		// key 是商品和历史成交金额组成的复用键。
		key := fmt.Sprintf("%s:%s:%d:%d", historicalAccountID, itemID, int64(amountValue), quantity)
		// _, exists 只读取当前商品金额组是否已经保留了更新的人工确认。
		if _, exists := historicalByGroup[key]; !exists {
			historicalByGroup[key] = struct {
				skuID         string
				unitCostCents int64
			}{skuID: skuID, unitCostCents: unitCostCents}
		}
	}
	if // historicalRowsErr 是人工快照复用记录遍历期间产生的延迟错误。
	historicalRowsErr := historicalRows.Err(); historicalRowsErr != nil {
		_ = historicalRows.Close()
		return nil, historicalRowsErr
	}
	if // closeErr 是释放人工快照复用结果集时产生的错误。
	closeErr := historicalRows.Close(); closeErr != nil {
		return nil, closeErr
	}
	// index 是当前补充可选 SKU 的金额分组下标。
	for index := range records {
		records[index].SKUs = skusByItem[records[index].AccountID+"\x00"+records[index].ItemID]
		// historical 是同商品同金额已存在的人工确认成本；新订单可沿用但仍需点击确认。
		historical, exists := historicalByGroup[fmt.Sprintf("%s:%s:%d:%d", records[index].AccountID, records[index].ItemID, records[index].AmountCents, records[index].Quantity)]
		if exists {
			// historicalCost 是避免返回 map 内部字段地址的独立历史成本值。
			historicalCost := historical.unitCostCents
			records[index].HistoricalSKUID = historical.skuID
			records[index].HistoricalUnitCostCents = &historicalCost
		}
	}
	return records, nil
}

// QueryManualHistoricalCostCandidates 返回无需当前商品或 SKU 也能由用户确认的历史成本分组。
func (r *AnalyticsRepository) QueryManualHistoricalCostCandidates(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.ManualHistoricalCostCandidateRecord, error) {
	// rows、queryErr 是 DB 层按账号、商品、金额和数量生成的全部未覆盖分组。
	rows, queryErr := r.store.OrderCosts.ListHistoricalCostCandidates(ctx, filter.UserID, filter.AccountID, filter.StartAt, filter.EndBefore, filter.Statuses)
	if queryErr != nil {
		return nil, queryErr
	}
	// records 是从 DB 持久化模型逐字段映射的应用层历史成本分组。
	records := make([]analyticsapp.ManualHistoricalCostCandidateRecord, 0, len(rows))
	// row 是当前映射的 DB 历史成本候选。
	for _, row := range rows {
		records = append(records, analyticsapp.ManualHistoricalCostCandidateRecord{AccountID: row.AccountID, ItemID: row.ItemID, ItemTitle: row.ItemTitle, AmountCents: row.AmountCents, Quantity: row.Quantity, OrderCount: row.OrderCount})
	}
	return records, nil
}

// ConfirmManualCostCandidate 为用户确认的商品金额分组生成不可变成本快照。
func (r *AnalyticsRepository) ConfirmManualCostCandidate(ctx context.Context, confirmation analyticsapp.ManualCostConfirmation) (analyticsapp.ManualCostConfirmationResult, error) {
	// matched、source、confirmErr 是 DB 层重新校验后生成的快照数量、来源和错误。
	matched, source, confirmErr := r.store.OrderCosts.ConfirmManualGroup(ctx, confirmation.UserID, confirmation.AccountID, confirmation.ItemID, confirmation.AmountCents, confirmation.Quantity, confirmation.StartAt, confirmation.EndBefore, confirmation.SKUID, confirmation.CustomUnitCostCents)
	return analyticsapp.ManualCostConfirmationResult{MatchedOrders: matched, MatchSource: source}, confirmErr
}

// ConfirmManualHistoricalCostCandidate 原子生成不依赖当前 SKU 的订单级人工历史成本覆盖。
func (r *AnalyticsRepository) ConfirmManualHistoricalCostCandidate(ctx context.Context, confirmation analyticsapp.ManualCostConfirmation) (analyticsapp.ManualCostConfirmationResult, error) {
	// unitCostCents 是应用层已校验存在的历史单件成本分值。
	unitCostCents := *confirmation.CustomUnitCostCents
	// matched、confirmErr 是精确分组新生成的不可变覆盖数量和写入错误。
	matched, confirmErr := r.store.OrderCosts.ConfirmHistoricalCostGroup(ctx, confirmation.UserID, confirmation.AccountID, confirmation.ItemID, confirmation.AmountCents, confirmation.Quantity, confirmation.StartAt, confirmation.EndBefore, unitCostCents, analyticsapp.ValidOrderStatuses)
	return analyticsapp.ManualCostConfirmationResult{MatchedOrders: matched, MatchSource: "manual_historical_cost"}, confirmErr
}

// NewAnalyticsRepository 构造订单分析查询适配器。
func NewAnalyticsRepository(store *db.Store) *AnalyticsRepository {
	return &AnalyticsRepository{store: store}
}

// DashboardStats 返回用户范围内的仪表盘计数，不读取账号凭证字段。
func (r *AnalyticsRepository) DashboardStats(ctx context.Context, userID int64) (analyticsapp.DashboardStats, error) {
	// stats 保存基础统计结果。
	stats := analyticsapp.DashboardStats{}
	// queries 保存用户范围内的固定计数查询及其目标字段。
	queries := []struct {
		// query 是当前统计项的 SQL。
		query string
		// target 是统计项写入结果的字段名称。
		target string
	}{
		{query: `SELECT COUNT(*) FROM cookies WHERE user_id=?`, target: "cookies"},
		{query: `SELECT COUNT(*) FROM cards WHERE user_id=?`, target: "cards"},
		{query: `SELECT COUNT(*) FROM keywords k WHERE EXISTS (SELECT 1 FROM cookies c WHERE c.id=k.cookie_id AND c.user_id=?)`, target: "keywords"},
		{query: `SELECT COUNT(*) FROM orders o WHERE o.deleted_at IS NULL AND EXISTS (SELECT 1 FROM cookies c WHERE c.id=o.cookie_id AND c.user_id=?)`, target: "orders"},
	}
	// item 是当前遍历的仪表盘统计项。
	for _, item := range queries {
		// count 是当前统计项的数据库结果。
		var count int64
		if // err 是当前统计项查询错误。
		err := r.store.Analytics.QueryRowContext(ctx, item.query, userID).Scan(&count); err != nil {
			return analyticsapp.DashboardStats{}, err
		}
		switch item.target {
		case "cookies":
			stats.TotalCookies = count
		case "cards":
			stats.TotalCards = count
		case "keywords":
			stats.TotalKeywords = count
		case "orders":
			stats.TotalOrders = count
		}
	}
	// activeCookies 是没有明确禁用记录的账号数量。
	var activeCookies int64
	if // err 是活跃账号统计查询错误。
	err := r.store.Analytics.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM cookies c WHERE c.user_id=?
		  AND NOT EXISTS (SELECT 1 FROM cookie_status cs WHERE cs.cookie_id=c.id AND cs.enabled=0)
	`, userID).Scan(&activeCookies); err != nil {
		return analyticsapp.DashboardStats{}, err
	}
	stats.ActiveCookies = activeCookies
	return stats, nil
}

// AvailableCardStock 计算启用数据卡密组中的非空卡密行数，不向应用层传递卡密内容。
func (r *AnalyticsRepository) AvailableCardStock(ctx context.Context, userID int64) (int64, error) {
	if r == nil || r.store == nil || r.store.Cards == nil {
		return 0, db.ErrNotFound
	}
	// stock、stockErr 保存 db 层在不泄露卡密正文时计算出的可用库存数量及错误。
	stock, stockErr := r.store.Cards.AvailableDataStock(ctx, userID)
	if stockErr != nil {
		return 0, stockErr
	}
	return stock, nil
}

// QueryRevenue 返回订单收益汇总。
func (r *AnalyticsRepository) QueryRevenue(ctx context.Context, filter analyticsapp.Filter) (analyticsapp.RevenueStats, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, clean, amountFilter := r.orderScope(filter, "amount")
	// stats 是数据库返回的收益聚合值。
	var stats analyticsapp.RevenueStats
	// err 是收益汇总查询错误。
	err := r.store.Analytics.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT order_id), COALESCE(SUM(`+clean+`),0),
		       COALESCE(AVG(`+clean+`),0), COUNT(DISTINCT buyer_id), COUNT(DISTINCT item_id)
		FROM orders `+where+amountFilter, args...).Scan(
		&stats.TotalOrders, &stats.TotalAmount, &stats.AvgAmount, &stats.UniqueBuyers, &stats.UniqueItems)
	return stats, err
}

// QueryDaily 返回按订单读取的日期聚合原始记录。
func (r *AnalyticsRepository) QueryDaily(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.DailyRecord, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, _, amountFilter := r.orderScope(filter, "amount")
	// rows、err 是日期聚合查询结果及错误。
	rows, err := r.store.Analytics.QueryContext(ctx, `SELECT order_id,amount,created_at FROM orders `+where+amountFilter, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// records 是订单日期聚合原始记录。
	records := make([]analyticsapp.DailyRecord, 0)
	for rows.Next() {
		// record 是当前订单的原始统计字段。
		var record analyticsapp.DailyRecord
		if // err 是当前日期记录读取错误。
		err := rows.Scan(&record.OrderID, &record.Amount, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// historicalItemTitle 在当前用户范围内优先读取商品目录最近标题，再回退聊天会话最近标题。
// ctx 控制两个本地只读查询；userID 和 itemID 限定所有权与商品，返回空标题表示没有可信历史记录。
func (r *AnalyticsRepository) historicalItemTitle(ctx context.Context, userID int64, itemID string) (string, error) {
	// title 保存当前候选来源返回的非空历史商品标题。
	var title string
	// itemErr 是包含软删除记录的商品目录标题查询结果。
	itemErr := r.store.Analytics.QueryRowContext(ctx, `
		SELECT i.item_title FROM item_info i
		JOIN cookies c ON c.id=i.cookie_id
		WHERE c.user_id=? AND i.item_id=? AND TRIM(i.item_title)<>''
		ORDER BY CASE WHEN i.deleted_at IS NULL THEN 0 ELSE 1 END, i.updated_at DESC
		LIMIT 1`, userID, itemID).Scan(&title)
	if itemErr == nil {
		return title, nil
	}
	if itemErr != sql.ErrNoRows {
		return "", itemErr
	}
	// chatErr 是商品目录没有标题时，同用户聊天会话最近标题的查询结果。
	chatErr := r.store.Analytics.QueryRowContext(ctx, `
		SELECT s.item_title FROM chat_sessions s
		JOIN cookies c ON c.id=s.cookie_id
		WHERE c.user_id=? AND s.item_id=? AND TRIM(s.item_title)<>''
		ORDER BY s.updated_at DESC
		LIMIT 1`, userID, itemID).Scan(&title)
	if chatErr == nil {
		return title, nil
	}
	if chatErr == sql.ErrNoRows {
		return "", nil
	}
	return "", chatErr
}

// QueryStatus 返回按原始订单状态聚合的结果。
func (r *AnalyticsRepository) QueryStatus(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.StatusRecord, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, clean, amountFilter := r.orderScope(filter, "amount")
	// rows、err 是状态聚合查询结果及错误。
	rows, err := r.store.Analytics.QueryContext(ctx, `
		SELECT COALESCE(order_status,'unknown'), COUNT(DISTINCT order_id), COALESCE(SUM(`+clean+`),0)
		FROM orders `+where+amountFilter+`
		GROUP BY order_status ORDER BY COUNT(DISTINCT order_id) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// records 是数据库按原始状态返回的聚合记录。
	records := make([]analyticsapp.StatusRecord, 0)
	for rows.Next() {
		// record 是当前状态的聚合字段。
		var record analyticsapp.StatusRecord
		if // err 是当前状态记录读取错误。
		err := rows.Scan(&record.Status, &record.Count, &record.Amount); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// QueryCity 返回按收货城市聚合的结果。
func (r *AnalyticsRepository) QueryCity(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.CityRecord, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, clean, amountFilter := r.orderScope(filter, "amount")
	// rows、err 是城市聚合查询结果及错误。
	rows, err := r.store.Analytics.QueryContext(ctx, `
		SELECT receiver_city, COUNT(DISTINCT order_id), COALESCE(SUM(`+clean+`),0)
		FROM orders `+where+amountFilter+`
		  AND receiver_city IS NOT NULL AND receiver_city != ''
		GROUP BY receiver_city ORDER BY COUNT(DISTINCT order_id) DESC LIMIT 50`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// records 是数据库按城市返回的聚合记录。
	records := make([]analyticsapp.CityRecord, 0)
	for rows.Next() {
		// record 是当前城市的聚合字段。
		var record analyticsapp.CityRecord
		if // err 是当前城市记录读取错误。
		err := rows.Scan(&record.City, &record.Count, &record.Amount); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// QueryItem 返回按商品标识聚合的结果。
func (r *AnalyticsRepository) QueryItem(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.ItemRecord, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, clean, amountFilter := r.orderScope(filter, "amount")
	// rows、err 是商品聚合查询结果及错误。
	rows, err := r.store.Analytics.QueryContext(ctx, `
		SELECT item_id, COUNT(DISTINCT order_id), COALESCE(SUM(`+clean+`),0), COALESCE(AVG(`+clean+`),0)
		FROM orders `+where+amountFilter+`
		  AND item_id IS NOT NULL AND item_id != ''
		GROUP BY item_id ORDER BY COUNT(DISTINCT order_id) DESC LIMIT 20`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// records 是数据库按商品返回的聚合记录。
	records := make([]analyticsapp.ItemRecord, 0)
	for rows.Next() {
		// record 是当前商品的聚合字段。
		var record analyticsapp.ItemRecord
		if // err 是当前商品记录读取错误。
		err := rows.Scan(&record.ItemID, &record.Count, &record.TotalAmount, &record.AvgAmount); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if // iterationErr 是商品聚合结果遍历期间产生的延迟错误。
	iterationErr := rows.Err(); iterationErr != nil {
		return nil, iterationErr
	}
	if // closeErr 是释放商品聚合结果集时产生的数据库错误；释放后才能安全执行标题回退查询。
	closeErr := rows.Close(); closeErr != nil {
		return nil, closeErr
	}
	// index 是当前补充历史商品标题的聚合结果下标。
	for index := range records {
		// title、titleErr 是当前商品从本地历史商品或聊天会话解析出的非空标题和查询错误。
		title, titleErr := r.historicalItemTitle(ctx, filter.UserID, records[index].ItemID)
		if titleErr != nil {
			return nil, titleErr
		}
		records[index].ItemTitle = title
	}
	return records, nil
}

// QueryProfitRecords 返回全部有效订单及其可选不可变成本快照，供应用层按用户时区聚合。
func (r *AnalyticsRepository) QueryProfitRecords(ctx context.Context, filter analyticsapp.Filter) ([]analyticsapp.ProfitRecord, error) {
	// where、args 和 amountFilter 是当前方言的订单范围条件。
	where, args, _, amountFilter := r.orderScope(filter, "orders.amount")
	// rows、queryErr 是订单毛利原始记录查询结果。
	rows, queryErr := r.store.Analytics.QueryContext(ctx, `SELECT orders.order_id,COALESCE(orders.item_id,''),COALESCE(item_info.item_title,''),
		orders.amount,COALESCE(orders.order_status,'unknown'),orders.created_at,
		COALESCE(order_cost_snapshots.unit_cost_cents,historical_order_cost_overrides.unit_cost_cents,0),COALESCE(order_cost_snapshots.quantity,historical_order_cost_overrides.quantity,0),
		CASE WHEN order_cost_snapshots.order_id IS NULL AND historical_order_cost_overrides.order_id IS NULL THEN 0 ELSE 1 END
		FROM orders
		LEFT JOIN item_info ON item_info.cookie_id=orders.cookie_id AND item_info.item_id=orders.item_id AND item_info.deleted_at IS NULL
		LEFT JOIN order_cost_snapshots ON order_cost_snapshots.order_id=orders.order_id
		LEFT JOIN historical_order_cost_overrides ON historical_order_cost_overrides.order_id=orders.order_id
		`+where+amountFilter+` ORDER BY orders.created_at`, args...)
	if queryErr != nil {
		return nil, queryErr
	}
	// records 保存订单毛利原始记录。
	records := make([]analyticsapp.ProfitRecord, 0)
	for rows.Next() {
		// record 是当前订单的收入和成本快照字段。
		var record analyticsapp.ProfitRecord
		// covered 是跨方言整数布尔值。
		var covered int
		if // scanErr 是当前订单毛利原始记录读取错误。
		scanErr := rows.Scan(&record.OrderID, &record.ItemID, &record.ItemTitle, &record.Amount, &record.Status, &record.CreatedAt, &record.UnitCostCents, &record.CostQuantity, &covered); scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		record.Covered = covered != 0
		records = append(records, record)
	}
	if // iterationErr 是订单毛利记录遍历期间产生的延迟错误。
	iterationErr := rows.Err(); iterationErr != nil {
		_ = rows.Close()
		return nil, iterationErr
	}
	if // closeErr 是释放订单毛利查询结果集时产生的错误。
	closeErr := rows.Close(); closeErr != nil {
		return nil, closeErr
	}
	// titleCache 避免同一历史商品重复执行标题回退查询。
	titleCache := make(map[string]string)
	// index 是当前补充历史商品标题的订单毛利记录下标。
	for index := range records {
		if records[index].ItemTitle != "" || records[index].ItemID == "" {
			continue
		}
		// cachedTitle、cached 表示是否已经解析过当前历史商品标题。
		cachedTitle, cached := titleCache[records[index].ItemID]
		if !cached {
			// title、titleErr 是本地商品或聊天历史中的标题和查询错误。
			title, titleErr := r.historicalItemTitle(ctx, filter.UserID, records[index].ItemID)
			if titleErr != nil {
				return nil, titleErr
			}
			cachedTitle = title
			titleCache[records[index].ItemID] = title
		}
		records[index].ItemTitle = cachedTitle
	}
	return records, nil
}

// CountValidOrders 返回有效订单总数。
func (r *AnalyticsRepository) CountValidOrders(ctx context.Context, filter analyticsapp.Filter) (int, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, _, amountFilter := r.orderScope(filter, "orders.amount")
	// total 是数据库返回的有效订单数量。
	var total int
	// err 是有效订单总数查询错误。
	err := r.store.Analytics.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders `+where+amountFilter, args...).Scan(&total)
	return total, err
}

// ListValidOrders 返回有效订单分页明细。
func (r *AnalyticsRepository) ListValidOrders(ctx context.Context, filter analyticsapp.Filter, limit, offset int) ([]analyticsapp.ValidOrderRecord, error) {
	// where、args、clean 和 amountFilter 是当前方言的订单范围条件。
	where, args, _, amountFilter := r.orderScope(filter, "orders.amount")
	args = append(args, limit, offset)
	// rows、err 是有效订单分页查询结果及错误。
	rows, err := r.store.Analytics.QueryContext(ctx, `
		SELECT orders.order_id, COALESCE(orders.item_id,''), COALESCE(item_info.item_title,''),
		       COALESCE(item_info.item_detail,''), COALESCE(orders.buyer_id,''), COALESCE(orders.quantity,'1'),
		       orders.amount, COALESCE(orders.order_status,'unknown'), COALESCE(orders.cookie_id,''), orders.created_at
		FROM orders LEFT JOIN item_info ON item_info.cookie_id=orders.cookie_id AND item_info.item_id=orders.item_id
		`+where+amountFilter+` ORDER BY orders.created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// records 是当前分页的有效订单字段。
	records := make([]analyticsapp.ValidOrderRecord, 0)
	for rows.Next() {
		// record 是当前订单的明细字段。
		var record analyticsapp.ValidOrderRecord
		if // err 是当前有效订单记录读取错误。
		err := rows.Scan(&record.OrderID, &record.ItemID, &record.ItemTitle, &record.ItemDetail, &record.BuyerID, &record.Quantity, &record.Amount, &record.Status, &record.CookieID, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// orderScope 构造订单查询的用户、日期、状态和金额过滤条件。
func (r *AnalyticsRepository) orderScope(filter analyticsapp.Filter, amountColumn string) (string, []any, string, string) {
	// conditions 保存 SQL WHERE 条件片段。
	conditions := []string{"orders.deleted_at IS NULL"}
	// args 保存条件对应的参数值。
	args := make([]any, 0, len(filter.Statuses)+4)
	// createdAtExpression 是当前方言用于比较订单创建时间的表达式；SQLite 需要统一混合文本时间格式。
	createdAtExpression := "orders.created_at"
	// boundaryExpression 是当前方言用于解释 UTC 边界参数的表达式。
	boundaryExpression := "?"
	if r.store.Orders.Dialect == db.DialectSQLite {
		createdAtExpression = "datetime(orders.created_at)"
		boundaryExpression = "datetime(?)"
	}
	if filter.StartAt != "" {
		conditions = append(conditions, createdAtExpression+" >= "+boundaryExpression)
		args = append(args, filter.StartAt)
	}
	if filter.EndBefore != "" {
		conditions = append(conditions, createdAtExpression+" < "+boundaryExpression)
		args = append(args, filter.EndBefore)
	}
	if filter.UserID != 0 {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM cookies WHERE cookies.id = orders.cookie_id AND cookies.user_id = ?)")
		args = append(args, filter.UserID)
	}
	if filter.AccountID != "" {
		conditions = append(conditions, "orders.cookie_id = ?")
		args = append(args, filter.AccountID)
	}
	if len(filter.Statuses) > 0 {
		// placeholders 是状态条件需要的占位符列表。
		placeholders := make([]string, len(filter.Statuses))
		// index 是当前状态占位符的序号。
		for index := range filter.Statuses {
			placeholders[index] = "?"
			args = append(args, filter.Statuses[index])
		}
		conditions = append(conditions, "orders.order_status IN ("+strings.Join(placeholders, ",")+")")
	}
	// where 是带 WHERE 前缀和尾随空格的条件文本。
	where := "WHERE " + strings.Join(conditions, " AND ") + " "
	// clean 是当前数据库方言清洗金额文本的表达式。
	clean := amountExpression(r.store.Orders.Dialect, amountColumn)
	return where, args, clean, " AND " + clean + " IS NOT NULL"
}

// amountExpression 返回当前数据库方言的安全金额表达式。
func amountExpression(dialect db.Dialect, column string) string {
	// clean 是移除货币符号和千分位后的金额文本表达式。
	clean := `TRIM(REPLACE(REPLACE(` + column + `, '¥', ''), ',', ''))`
	switch dialect {
	case db.DialectPostgres:
		return `CASE WHEN ` + clean + ` ~ '^[0-9]+([.][0-9]+)?$' THEN CAST(` + clean + ` AS DOUBLE PRECISION) END`
	case db.DialectMySQL:
		return `CASE WHEN ` + clean + ` REGEXP '^[0-9]+([.][0-9]+)?$' THEN CAST(` + clean + ` AS DOUBLE) END`
	default:
		return `CASE WHEN ` + clean + ` GLOB '[0-9]*' AND ` + clean + ` NOT GLOB '*[^0-9.]*' AND ` + clean + ` NOT GLOB '*.*.*' AND ` + clean + ` NOT LIKE '%.' THEN CAST(` + clean + ` AS REAL) END`
	}
}

// 确保基础设施适配器实现完整的订单分析应用 Port。
var _ analyticsapp.Repository = (*AnalyticsRepository)(nil)
