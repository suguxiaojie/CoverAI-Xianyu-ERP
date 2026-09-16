package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidOrderStatuses 是参与订单分析的有效状态候选集合。
var ValidOrderStatuses = []string{"pending_ship", "paid", "2", "shipped", "3", "received", "completed", "4", "11"}

// Stage 标识订单分析失败的查询阶段。
type Stage string

const (
	// StageRevenue 表示收益汇总查询失败。
	StageRevenue Stage = "revenue"
	// StageDaily 表示每日统计查询失败。
	StageDaily Stage = "daily"
	// StageStatus 表示状态统计查询失败。
	StageStatus Stage = "status"
	// StageCity 表示城市统计查询失败。
	StageCity Stage = "city"
	// StageItem 表示商品统计查询失败。
	StageItem Stage = "item"
	// StageProfit 表示商品毛利记录查询失败。
	StageProfit Stage = "profit"
	// StageValidCount 表示有效订单总数查询失败。
	StageValidCount Stage = "valid_count"
	// StageValidRows 表示有效订单明细查询失败。
	StageValidRows Stage = "valid_rows"
)

// StageError 保留查询阶段并包装底层错误，供传输层映射兼容消息。
type StageError struct {
	// Stage 是失败的查询阶段。
	Stage Stage
	// Err 是底层查询错误。
	Err error
}

// Error 返回底层错误文本。
func (e *StageError) Error() string {
	if e == nil || e.Err == nil {
		return "订单分析查询失败"
	}
	return e.Err.Error()
}

// Unwrap 暴露底层查询错误以支持 errors.Is 和 errors.As。
func (e *StageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewService 构造订单分析应用服务。
func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// Service 编排订单分析用例，不感知 HTTP、数据库连接或 Server。
type Service struct {
	// repository 提供订单分析所需的窄查询能力。
	repository Repository
}

// LocationFromOffset 将浏览器提交的分钟偏移转换为用户时区。
func LocationFromOffset(rawOffset string) *time.Location {
	// offset 是浏览器相对 UTC 的分钟偏移。
	offset, err := strconv.Atoi(strings.TrimSpace(rawOffset))
	if err != nil || offset < -14*60 || offset > 14*60 {
		return time.Local
	}
	return time.FixedZone("browser", offset*60)
}

// DateBoundary 将用户本地日期转换为数据库使用的 UTC 边界文本。
func DateBoundary(raw string, endExclusive bool, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	// parsed 是按用户时区解释后的日期起点。
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), location)
	if err != nil {
		return raw
	}
	if endExclusive {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return parsed.UTC().Format("2006-01-02 15:04:05")
}

// DashboardStats 查询用户数据概览和可用卡密库存。
func (svc *Service) DashboardStats(ctx context.Context, userID int64) (DashboardStats, error) {
	// stats 是数据库返回的用户范围计数。
	stats, err := svc.repository.DashboardStats(ctx, userID)
	if err != nil {
		return DashboardStats{}, err
	}
	// stock 是基础设施按数据卡密组内容计算出的可用库存数量。
	stock, err := svc.repository.AvailableCardStock(ctx, userID)
	if err != nil {
		return DashboardStats{}, err
	}
	stats.AvailableCardStock = stock
	return stats, nil
}

// ManualCostCandidates 返回 Dashboard 用户、账号和日期范围内的人工历史成本候选，不执行任何写入。
func (svc *Service) ManualCostCandidates(ctx context.Context, query Query) ([]ManualCostCandidate, error) {
	// filter 是将用户本地日期转为 UTC 边界后的权威历史成本范围。
	filter := query.filter()
	// records、queryErr 是持久化层返回的金额分组和 SKU 选项。
	records, queryErr := svc.repository.QueryManualCostCandidates(ctx, filter)
	if queryErr != nil {
		return nil, queryErr
	}
	// historicalRecords、historicalErr 是不依赖当前商品和 SKU 的全部未覆盖分组。
	historicalRecords, historicalErr := svc.repository.QueryManualHistoricalCostCandidates(ctx, filter)
	if historicalErr != nil {
		return nil, historicalErr
	}
	// candidates 是包含价格距离建议的人工确认分组。
	candidates := make([]ManualCostCandidate, 0, len(historicalRecords))
	// skuCandidateKeys 保存已能引用当前 SKU 的精确账号、商品、金额和数量分组。
	skuCandidateKeys := make(map[string]struct{}, len(records))
	// record 是当前计算 SKU 建议的历史金额分组。
	for _, record := range records {
		// suggested、confidence 是当前分组唯一最近 SKU 及建议置信度。
		suggested, confidence := suggestedManualCostSKU(record.AmountCents, record.SKUs)
		// suggestedCost 是当前建议 SKU 对应的成本，若存在同金额历史确认则优先沿用历史成本。
		suggestedCost := int64(0)
		if record.HistoricalSKUID != "" && record.HistoricalUnitCostCents != nil {
			suggested, confidence, suggestedCost = record.HistoricalSKUID, "historical", *record.HistoricalUnitCostCents
		} else {
			// sku 是当前寻找建议 SKU 成本的可选项。
			for _, sku := range record.SKUs {
				if sku.SKUID == suggested {
					suggestedCost = sku.CostCents
					break
				}
			}
		}
		// key 是当前 SKU 候选的稳定精确分组键。
		key := manualCostCandidateKey(record.AccountID, record.ItemID, record.AmountCents, record.Quantity)
		skuCandidateKeys[key] = struct{}{}
		candidates = append(candidates, ManualCostCandidate{AccountID: record.AccountID, ItemID: record.ItemID, ItemTitle: record.ItemTitle, AmountCents: record.AmountCents, Quantity: record.Quantity, OrderCount: record.OrderCount, SuggestedSKUID: suggested, Confidence: confidence, SKUs: record.SKUs, SuggestedUnitCostCents: suggestedCost})
	}
	// record 是当前检查是否需要纯人工历史成本的全量未覆盖分组。
	for _, record := range historicalRecords {
		// key 是当前历史分组与 SKU 候选去重使用的稳定键。
		key := manualCostCandidateKey(record.AccountID, record.ItemID, record.AmountCents, record.Quantity)
		if // exists 表示当前精确分组已可引用 SKU，不再重复生成纯人工候选。
		_, exists := skuCandidateKeys[key]; exists {
			continue
		}
		candidates = append(candidates, ManualCostCandidate{AccountID: record.AccountID, ItemID: record.ItemID, ItemTitle: record.ItemTitle, AmountCents: record.AmountCents, Quantity: record.Quantity, OrderCount: record.OrderCount, Confidence: "manual", SKUs: []ManualCostSKU{}, ManualOnly: true})
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

// ConfirmManualCostCandidate 校验人工选择并委托持久化层原子生成该金额分组的成本快照。
func (svc *Service) ConfirmManualCostCandidate(ctx context.Context, confirmation ManualCostConfirmation) (ManualCostConfirmationResult, error) {
	if confirmation.UserID <= 0 || strings.TrimSpace(confirmation.AccountID) == "" || confirmation.AmountCents < 0 || confirmation.Quantity <= 0 {
		return ManualCostConfirmationResult{}, errors.New("历史成本确认参数无效")
	}
	if confirmation.CustomUnitCostCents != nil && *confirmation.CustomUnitCostCents < 0 {
		return ManualCostConfirmationResult{}, errors.New("历史单件成本不能为负数")
	}
	if strings.TrimSpace(confirmation.SKUID) == "" {
		if confirmation.CustomUnitCostCents == nil {
			return ManualCostConfirmationResult{}, errors.New("历史单件成本不能为空")
		}
		return svc.repository.ConfirmManualHistoricalCostCandidate(ctx, confirmation)
	}
	if strings.TrimSpace(confirmation.ItemID) == "" {
		return ManualCostConfirmationResult{}, errors.New("历史成本确认参数无效")
	}
	return svc.repository.ConfirmManualCostCandidate(ctx, confirmation)
}

// manualCostCandidateKey 构造账号、商品、单笔金额和购买数量组成的稳定分组键。
func manualCostCandidateKey(accountID, itemID string, amountCents int64, quantity int) string {
	return accountID + "\x00" + itemID + "\x00" + strconv.FormatInt(amountCents, 10) + "\x00" + strconv.Itoa(quantity)
}

// suggestedManualCostSKU 选择与成交金额距离最小且唯一的 SKU；相同距离保持歧义。
func suggestedManualCostSKU(amountCents int64, skus []ManualCostSKU) (string, string) {
	// suggested 保存当前唯一最近 SKU 标识。
	suggested := ""
	// minimumDistance 保存当前最小价格差分值。
	minimumDistance := int64(-1)
	// ambiguous 表示至少两个 SKU 与成交金额距离相同。
	ambiguous := false
	// sku 是当前参与价格距离比较的成本 SKU。
	for _, sku := range skus {
		// distance 是当前标准售价与历史成交金额的绝对差分值。
		distance := sku.PriceCents - amountCents
		if distance < 0 {
			distance = -distance
		}
		if minimumDistance < 0 || distance < minimumDistance {
			minimumDistance, suggested, ambiguous = distance, sku.SKUID, false
		} else if distance == minimumDistance {
			ambiguous = true
		}
	}
	if suggested == "" || ambiguous {
		return "", "ambiguous"
	}
	if minimumDistance == 0 {
		return suggested, "exact"
	}
	return suggested, "adjusted"
}

// OrderAnalytics 查询收益及按日、状态、城市和商品维度聚合的订单分析结果。
func (svc *Service) OrderAnalytics(ctx context.Context, query Query) (OrderAnalytics, error) {
	// filter 是按用户本地日期转换后的持久化查询条件。
	filter := query.filter()
	// revenue 是收益汇总查询结果。
	revenue, err := svc.repository.QueryRevenue(ctx, filter)
	if err != nil {
		return OrderAnalytics{}, &StageError{Stage: StageRevenue, Err: err}
	}
	// dailyRecords 是按订单返回的日期聚合原始记录。
	dailyRecords, err := svc.repository.QueryDaily(ctx, filter)
	if err != nil {
		return OrderAnalytics{}, &StageError{Stage: StageDaily, Err: err}
	}
	// dailyMap 按用户本地日期累计订单数量和金额。
	dailyMap := make(map[string]dailyValue)
	// record 是当前订单日期聚合原始记录。
	for _, record := range dailyRecords {
		// created 是解析后的订单创建时间。
		created := parseDBTime(record.CreatedAt)
		if created.IsZero() {
			continue
		}
		if query.Location != nil {
			created = created.In(query.Location)
		}
		// date 是订单在用户时区中的日期。
		date := created.Format("2006-01-02")
		// value 是当前日期的累计统计值。
		value := dailyMap[date]
		value.count++
		value.amount += parseAmount(record.Amount)
		dailyMap[date] = value
	}
	// daily 是排序后的按日统计结果。
	daily := make([]DailyStats, 0, len(dailyMap))
	// dates 是待输出的日期列表。
	dates := make([]string, 0, len(dailyMap))
	// date 是当前待输出的日期键。
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	// date 是当前输出的日期键。
	for _, date := range dates {
		// value 是当前日期的累计统计值。
		value := dailyMap[date]
		daily = append(daily, DailyStats{Date: date, OrderCount: value.count, Amount: round2(value.amount)})
	}

	// statusRecords 是数据库按原始状态返回的聚合结果。
	statusRecords, err := svc.repository.QueryStatus(ctx, filter)
	if err != nil {
		return OrderAnalytics{}, &StageError{Stage: StageStatus, Err: err}
	}
	// statusMap 按归一化状态累计数据库聚合结果。
	statusMap := make(map[string]statusValue)
	// record 是当前原始状态聚合记录。
	for _, record := range statusRecords {
		// status 是兼容数字码转换后的状态名称。
		status := normalizeStatus(record.Status)
		// value 是当前状态的累计统计值。
		value := statusMap[status]
		value.count += record.Count
		value.amount += record.Amount
		statusMap[status] = value
	}
	// statusStats 是按订单数量降序排列的状态统计结果。
	statusStats := make([]StatusStats, 0, len(statusMap))
	// statusNames 是待排序的状态名称。
	statusNames := make([]string, 0, len(statusMap))
	// status 是当前待排序的归一化状态名称。
	for status := range statusMap {
		statusNames = append(statusNames, status)
	}
	sort.Slice(statusNames, func(i, j int) bool { return statusMap[statusNames[i]].count > statusMap[statusNames[j]].count })
	// status 是当前输出的归一化状态名称。
	for _, status := range statusNames {
		// value 是当前状态的累计统计值。
		value := statusMap[status]
		statusStats = append(statusStats, StatusStats{Status: status, Count: value.count, Amount: round2(value.amount)})
	}

	// cityRecords 是数据库按收货城市返回的聚合结果。
	cityRecords, err := svc.repository.QueryCity(ctx, filter)
	if err != nil {
		return OrderAnalytics{}, &StageError{Stage: StageCity, Err: err}
	}
	// cityStats 是收货城市统计结果。
	cityStats := make([]CityStats, 0, len(cityRecords))
	// record 是当前收货城市聚合记录。
	for _, record := range cityRecords {
		cityStats = append(cityStats, CityStats{City: record.City, OrderCount: record.Count, TotalAmount: round2(record.Amount)})
	}

	// itemRecords 是数据库按商品返回的聚合结果。
	itemRecords, err := svc.repository.QueryItem(ctx, filter)
	if err != nil {
		return OrderAnalytics{}, &StageError{Stage: StageItem, Err: err}
	}
	// itemStats 是商品排行统计结果。
	itemStats := make([]ItemStats, 0, len(itemRecords))
	// record 是当前商品聚合记录。
	for _, record := range itemRecords {
		itemStats = append(itemStats, ItemStats{ItemID: record.ItemID, ItemTitle: record.ItemTitle, OrderCount: record.Count, TotalAmount: round2(record.TotalAmount), AvgAmount: round2(record.AvgAmount)})
	}
	// profitRecords 是订单毛利聚合使用的全部有效订单和可选成本快照。
	profitRecords, err := svc.repository.QueryProfitRecords(ctx, filter)
	if err != nil {
		return OrderAnalytics{}, &StageError{Stage: StageProfit, Err: err}
	}
	// profit、dailyProfit 和 itemProfit 是从订单成本快照聚合的毛利结果。
	profit, dailyProfit, itemProfit := aggregateProfit(profitRecords, query.Location)
	return OrderAnalytics{RevenueStats: RevenueStats{
		TotalOrders: revenue.TotalOrders, TotalAmount: round2(revenue.TotalAmount), AvgAmount: round2(revenue.AvgAmount),
		UniqueBuyers: revenue.UniqueBuyers, UniqueItems: revenue.UniqueItems,
	}, DailyStats: daily, StatusStats: statusStats, CityStats: cityStats, ItemStats: itemStats,
		ProfitStats: profit, DailyProfitStats: dailyProfit, ItemProfitStats: itemProfit}, nil
}

// aggregateProfit 仅使用可靠成本快照计算商品毛利，同时保留全部订单作为覆盖率分母。
func aggregateProfit(records []ProfitRecord, location *time.Location) (ProfitStats, []DailyProfitStats, []ItemProfitStats) {
	// profit 保存全范围商品毛利摘要。
	profit := ProfitStats{TotalOrders: len(records)}
	// dailyMap 保存按本地日期聚合的毛利状态。
	dailyMap := make(map[string]*DailyProfitStats)
	// itemMap 保存按商品聚合的毛利状态。
	itemMap := make(map[string]*ItemProfitStats)
	// record 是当前参与成本覆盖和商品毛利聚合的订单记录。
	for _, record := range records {
		// revenue 是当前订单成交额。
		revenue := parseAmount(record.Amount)
		// created 是当前订单按用户时区解释的创建时间。
		created := parseDBTime(record.CreatedAt)
		if location != nil && !created.IsZero() {
			created = created.In(location)
		}
		// date 是当前订单的本地日期；无法解析时不进入趋势但仍进入总计。
		date := ""
		if !created.IsZero() {
			date = created.Format("2006-01-02")
		}
		// daily 是当前日期的累计值。
		var daily *DailyProfitStats
		if date != "" {
			daily = dailyMap[date]
			if daily == nil {
				daily = &DailyProfitStats{Date: date}
				dailyMap[date] = daily
			}
			daily.TotalOrders++
			daily.RevenueAmount += revenue
		}
		// item 是当前商品的累计值。
		item := itemMap[record.ItemID]
		if item == nil {
			item = &ItemProfitStats{ItemID: record.ItemID, ItemTitle: record.ItemTitle}
			itemMap[record.ItemID] = item
		}
		item.TotalOrders++
		if !record.Covered {
			continue
		}
		// costAmount 是当前订单快照成本，单位为人民币元。
		costAmount := float64(record.UnitCostCents*int64(record.CostQuantity)) / 100
		// grossProfit 是当前订单商品毛利。
		// platformFee 是当前订单按成交额 1.6% 逐单四舍五入的平台手续费。
		platformFee := round2(revenue * 0.016)
		// grossProfit 是扣除商品成本和平台手续费后的预估经营利润。
		grossProfit := revenue - costAmount - platformFee
		profit.CoveredOrders++
		profit.CoveredRevenue += revenue
		profit.CostAmount += costAmount
		profit.PlatformFee += platformFee
		profit.GrossProfit += grossProfit
		if normalizeStatus(record.Status) == "received" || normalizeStatus(record.Status) == "completed" {
			profit.RealizedGrossProfit += grossProfit
		}
		if daily != nil {
			daily.CoveredOrders++
			daily.CoveredRevenue += revenue
			daily.CostAmount += costAmount
			daily.PlatformFee += platformFee
			daily.GrossProfit += grossProfit
		}
		item.CoveredOrders++
		item.CoveredRevenue += revenue
		item.CostAmount += costAmount
		item.PlatformFee += platformFee
		item.GrossProfit += grossProfit
	}
	profit.UnknownOrders = profit.TotalOrders - profit.CoveredOrders
	if profit.TotalOrders > 0 {
		profit.CoveragePercent = round2(float64(profit.CoveredOrders) / float64(profit.TotalOrders) * 100)
	}
	if profit.CoveredRevenue > 0 {
		profit.GrossMarginPercent = round2(profit.GrossProfit / profit.CoveredRevenue * 100)
	}
	profit.CoveredRevenue, profit.CostAmount, profit.PlatformFee, profit.GrossProfit, profit.RealizedGrossProfit = round2(profit.CoveredRevenue), round2(profit.CostAmount), round2(profit.PlatformFee), round2(profit.GrossProfit), round2(profit.RealizedGrossProfit)
	// dates 是待输出的升序日期。
	dates := make([]string, 0, len(dailyMap))
	// date 是当前待输出的毛利统计日期。
	for date := range dailyMap {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	// dailyResult 是稳定排序且金额四舍五入的每日毛利。
	dailyResult := make([]DailyProfitStats, 0, len(dates))
	// date 是当前输出的升序毛利统计日期。
	for _, date := range dates {
		// value 是当前日期完成金额舍入后的毛利统计副本。
		value := *dailyMap[date]
		value.RevenueAmount, value.CoveredRevenue, value.CostAmount, value.PlatformFee, value.GrossProfit = round2(value.RevenueAmount), round2(value.CoveredRevenue), round2(value.CostAmount), round2(value.PlatformFee), round2(value.GrossProfit)
		dailyResult = append(dailyResult, value)
	}
	// itemResult 是按毛利降序输出的商品统计。
	itemResult := make([]ItemProfitStats, 0, len(itemMap))
	// value 是当前商品的毛利统计累计值。
	for _, value := range itemMap {
		if value.TotalOrders > 0 {
			value.CoveragePercent = round2(float64(value.CoveredOrders) / float64(value.TotalOrders) * 100)
		}
		if value.CoveredRevenue > 0 {
			value.GrossMarginPercent = round2(value.GrossProfit / value.CoveredRevenue * 100)
		}
		value.CoveredRevenue, value.CostAmount, value.PlatformFee, value.GrossProfit = round2(value.CoveredRevenue), round2(value.CostAmount), round2(value.PlatformFee), round2(value.GrossProfit)
		itemResult = append(itemResult, *value)
	}
	sort.Slice(itemResult, func(left, right int) bool { return itemResult[left].GrossProfit > itemResult[right].GrossProfit })
	return profit, dailyResult, itemResult
}

// ValidOrders 查询有效订单分页明细。
func (svc *Service) ValidOrders(ctx context.Context, query Query, page, pageSize int) (ValidOrders, error) {
	// filter 是按用户本地日期转换后的持久化查询条件。
	filter := query.filter()
	// total 是符合筛选条件的有效订单总数。
	total, err := svc.repository.CountValidOrders(ctx, filter)
	if err != nil {
		return ValidOrders{}, &StageError{Stage: StageValidCount, Err: err}
	}
	// offset 是当前分页需要跳过的记录数。
	offset := (page - 1) * pageSize
	// records 是当前分页的订单明细原始记录。
	records, err := svc.repository.ListValidOrders(ctx, filter, pageSize, offset)
	if err != nil {
		return ValidOrders{}, &StageError{Stage: StageValidRows, Err: err}
	}
	// orders 是面向传输层的有效订单明细。
	orders := make([]ValidOrder, 0, len(records))
	// record 是当前有效订单原始记录。
	for _, record := range records {
		// status 是兼容数字码转换后的状态名称。
		status := normalizeStatus(record.Status)
		orders = append(orders, ValidOrder{
			OrderID: record.OrderID, ItemID: record.ItemID, BuyerID: record.BuyerID, ItemTitle: record.ItemTitle,
			ItemImage: itemImageFromDetail(record.ItemDetail), Quantity: record.Quantity, Amount: record.Amount,
			OrderStatus: status, Status: status, CookieID: record.CookieID, CreatedAt: record.CreatedAt,
		})
	}
	return ValidOrders{Orders: orders, Total: total, Page: page, PageSize: pageSize, Truncated: offset+len(orders) < total}, nil
}

// ErrorMessage 将分析失败阶段映射为原有 HTTP 错误消息。
func ErrorMessage(err error) string {
	// stageErr 是带查询阶段的应用服务错误。
	var stageErr *StageError
	if !errors.As(err, &stageErr) {
		return "查询失败"
	}
	switch stageErr.Stage {
	case StageRevenue:
		return "查询收益统计失败"
	case StageDaily:
		return "查询每日统计失败"
	case StageStatus:
		return "查询状态统计失败"
	case StageCity:
		return "查询城市统计失败"
	case StageItem:
		return "查询商品统计失败"
	case StageProfit:
		return "查询商品毛利失败"
	default:
		return "查询失败"
	}
}

// filter 将用户本地日期查询转换为数据库 UTC 范围并复制状态集合。
func (query Query) filter() Filter {
	// statuses 是固定有效状态的独立副本，避免调用方修改全局集合。
	statuses := append([]string(nil), ValidOrderStatuses...)
	return Filter{UserID: query.UserID, AccountID: strings.TrimSpace(query.AccountID), StartAt: dateBoundary(query.StartDate, false, query.Location), EndBefore: dateBoundary(query.EndDate, true, query.Location), Statuses: statuses}
}

// dailyValue 保存单个日期的订单数和金额累计值。
type dailyValue struct {
	// count 是当前日期的订单数量。
	count int
	// amount 是当前日期的金额累计值。
	amount float64
}

// statusValue 保存单个归一化状态的订单数和金额累计值。
type statusValue struct {
	// count 是当前状态的订单数量。
	count int
	// amount 是当前状态的金额累计值。
	amount float64
}

// dateBoundary 是内部日期边界转换实现。
func dateBoundary(raw string, endExclusive bool, location *time.Location) string {
	return DateBoundary(raw, endExclusive, location)
}

// parseDBTime 将数据库常见时间文本解析为 UTC 时间。
func parseDBTime(raw string) time.Time {
	// layouts 是支持的数据库时间格式集合。
	layouts := []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04:05Z07:00"}
	// layout 是当前尝试的数据库时间格式。
	for _, layout := range layouts {
		// parsed 是当前格式尝试得到的时间值。
		parsed, err := time.ParseInLocation(layout, strings.TrimSpace(raw), time.UTC)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// parseAmount 将订单金额文本转换为数值。
func parseAmount(raw string) float64 {
	// cleaned 是移除货币符号和千分位后的金额文本。
	cleaned := strings.TrimSpace(strings.NewReplacer("¥", "", ",", "").Replace(raw))
	// value 是解析后的金额；非法金额按零处理以保持历史统计口径。
	value, _ := strconv.ParseFloat(cleaned, 64)
	return value
}

// round2 将金额按两位小数进行四舍五入。
func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

// normalizeStatus 将历史数字状态码归一为业务状态名称。
func normalizeStatus(status string) string {
	// normalized 是历史平台数字状态到业务名称的兼容映射。
	normalized, ok := map[string]string{
		"paid": "pending_ship", "1": "processing", "2": "pending_ship", "3": "shipped", "4": "completed",
		"5": "refunding", "6": "cancelled", "7": "refunding", "8": "cancelled", "9": "refunding", "10": "cancelled", "11": "completed", "12": "cancelled",
	}[status]
	if ok {
		return normalized
	}
	if status == "" {
		return "unknown"
	}
	return status
}

// itemImageFromDetail 从商品详情 JSON 中提取主图地址。
func itemImageFromDetail(detail string) string {
	if detail == "" {
		return ""
	}
	// payload 是商品详情的动态 JSON 对象。
	var payload map[string]any
	if // err 是商品详情 JSON 解析错误。
	err := json.Unmarshal([]byte(detail), &payload); err != nil {
		return ""
	}
	// picture 是商品详情中的图片信息对象。
	if picture, ok := payload["pic_info"].(map[string]any); ok {
		if // url、ok 是图片地址及其类型判断结果。
		url, ok := picture["picUrl"].(string); ok {
			return url
		}
	}
	if // url、ok 是兼容图片地址及其类型判断结果。
	url, ok := payload["item_image"].(string); ok {
		return url
	}
	return ""
}
