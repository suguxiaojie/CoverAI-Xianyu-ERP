// Package analytics 定义订单分析用例、纯查询模型和消费者侧持久化 Port。
// 本包不依赖数据库、HTTP、平台协议或 Server 实现。
package analytics

import (
	"context"
	"time"
)

// Query 是订单分析用例使用的用户范围、日期和时区条件。
type Query struct {
	// UserID 是当前登录用户的本地身份标识。
	UserID int64
	// AccountID 是可选的闲鱼账号范围；空值表示统计当前用户的全部账号。
	AccountID string
	// StartDate 是用户本地日期范围的起始日期，格式为 YYYY-MM-DD。
	StartDate string
	// EndDate 是用户本地日期范围的结束日期，格式为 YYYY-MM-DD。
	EndDate string
	// Location 是用于日期边界和按日聚合的用户本地时区。
	Location *time.Location
}

// Filter 是持久化查询已经转换后的用户、UTC 日期和有效状态条件。
type Filter struct {
	// UserID 是订单所属账号对应的用户身份标识。
	UserID int64
	// AccountID 是经传输层选择的可选账号标识；必须与 UserID 所有权条件同时生效。
	AccountID string
	// StartAt 是包含边界的 UTC 起始时间文本；为空表示不限制起始时间。
	StartAt string
	// EndBefore 是排除边界的 UTC 结束时间文本；为空表示不限制结束时间。
	EndBefore string
	// Statuses 是允许参与统计的订单状态候选集合。
	Statuses []string
}

// DashboardStats 是用户仪表盘所需的非敏感统计摘要。
type DashboardStats struct {
	// TotalCookies 是用户账号总数。
	TotalCookies int64
	// ActiveCookies 是没有明确禁用记录的账号数。
	ActiveCookies int64
	// TotalCards 是用户卡密组总数。
	TotalCards int64
	// AvailableCardStock 是启用数据卡密组中的非空卡密行数。
	AvailableCardStock int64
	// TotalKeywords 是用户关键词规则总数。
	TotalKeywords int64
	// TotalOrders 是用户未删除订单总数。
	TotalOrders int64
}

// RevenueStats 是订单收益聚合结果。
type RevenueStats struct {
	// TotalOrders 是统计范围内的去重订单数。
	TotalOrders int
	// TotalAmount 是统计范围内的订单总金额。
	TotalAmount float64
	// AvgAmount 是统计范围内的订单平均金额。
	AvgAmount float64
	// UniqueBuyers 是统计范围内的去重买家数。
	UniqueBuyers int
	// UniqueItems 是统计范围内的去重商品数。
	UniqueItems int
}

// DailyRecord 是按订单返回的日期聚合原始记录。
type DailyRecord struct {
	// OrderID 是用于保留原有订单计数口径的订单标识。
	OrderID string
	// Amount 是数据库中的订单金额文本。
	Amount string
	// CreatedAt 是数据库中的订单创建时间文本。
	CreatedAt string
}

// StatusRecord 是数据库按原始状态聚合的结果。
type StatusRecord struct {
	// Status 是持久化的订单状态文本或数字码。
	Status string
	// Count 是该原始状态对应的去重订单数。
	Count int
	// Amount 是该原始状态对应的金额合计。
	Amount float64
}

// CityRecord 是数据库按收货城市聚合的结果。
type CityRecord struct {
	// City 是收货城市名称。
	City string
	// Count 是该城市的去重订单数。
	Count int
	// Amount 是该城市的金额合计。
	Amount float64
}

// ItemRecord 是数据库按商品标识聚合的结果。
type ItemRecord struct {
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是当前或历史商品记录中最近保存的非空标题。
	ItemTitle string
	// Count 是该商品的去重订单数。
	Count int
	// TotalAmount 是该商品的金额合计。
	TotalAmount float64
	// AvgAmount 是该商品的平均金额。
	AvgAmount float64
}

// ProfitRecord 是数据库按订单返回的商品毛利聚合原始记录。
type ProfitRecord struct {
	// OrderID 是平台订单标识。
	OrderID string
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是当前或历史商品标题。
	ItemTitle string
	// Amount 是订单成交金额文本。
	Amount string
	// Status 是订单当前状态。
	Status string
	// CreatedAt 是订单创建时间文本。
	CreatedAt string
	// UnitCostCents 是快照锁定的单件成本分值。
	UnitCostCents int64
	// CostQuantity 是成本快照锁定的订单数量。
	CostQuantity int
	// Covered 表示当前订单已经具备可靠成本快照。
	Covered bool
}

// ValidOrderRecord 是有效订单明细所需的非敏感字段。
type ValidOrderRecord struct {
	// OrderID 是平台订单标识。
	OrderID string
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是本地商品标题。
	ItemTitle string
	// ItemDetail 是本地商品详情 JSON，用于提取主图地址。
	ItemDetail string
	// BuyerID 是买家平台标识。
	BuyerID string
	// Quantity 是订单数量文本。
	Quantity string
	// Amount 是订单金额文本。
	Amount string
	// Status 是持久化的订单状态文本或数字码。
	Status string
	// CookieID 是订单所属账号标识。
	CookieID string
	// CreatedAt 是订单创建时间文本。
	CreatedAt string
}

// OrderAnalytics 是订单分析用例的完整结果模型。
type OrderAnalytics struct {
	// RevenueStats 是收益汇总。
	RevenueStats RevenueStats
	// DailyStats 是按用户本地日期聚合的结果。
	DailyStats []DailyStats
	// StatusStats 是按归一化订单状态聚合的结果。
	StatusStats []StatusStats
	// CityStats 是按收货城市聚合的结果。
	CityStats []CityStats
	// ItemStats 是按商品标识聚合的结果。
	ItemStats []ItemStats
	// ProfitStats 是仅基于已覆盖订单的商品毛利和覆盖率摘要。
	ProfitStats ProfitStats
	// DailyProfitStats 是按用户本地日期聚合的收入、成本和毛利。
	DailyProfitStats []DailyProfitStats
	// ItemProfitStats 是按商品聚合的毛利与覆盖率排行。
	ItemProfitStats []ItemProfitStats
}

// ProfitStats 是商品成本快照覆盖范围内的毛利摘要；不包含平台费、运费、广告和税费。
type ProfitStats struct {
	// TotalOrders 是当前有效订单总数。
	TotalOrders int
	// CoveredOrders 是具有可靠成本快照的订单数。
	CoveredOrders int
	// UnknownOrders 是缺少可靠成本快照的订单数。
	UnknownOrders int
	// CoveredRevenue 是已覆盖订单成交额。
	CoveredRevenue float64
	// CostAmount 是已覆盖订单商品成本。
	CostAmount float64
	// PlatformFee 是已覆盖订单按成交额 1.6% 逐单四舍五入的平台手续费。
	PlatformFee float64
	// GrossProfit 是已覆盖订单商品毛利。
	GrossProfit float64
	// RealizedGrossProfit 是已收货或已完成覆盖订单的商品毛利。
	RealizedGrossProfit float64
	// GrossMarginPercent 是已覆盖订单商品毛利率。
	GrossMarginPercent float64
	// CoveragePercent 是当前订单成本覆盖率。
	CoveragePercent float64
}

// DailyProfitStats 是单日收入、成本、商品毛利和成本覆盖情况。
type DailyProfitStats struct {
	// Date 是用户本地日期。
	Date string
	// TotalOrders 是当日有效订单数。
	TotalOrders int
	// CoveredOrders 是当日已匹配成本订单数。
	CoveredOrders int
	// RevenueAmount 是当日全部有效订单成交额。
	RevenueAmount float64
	// CoveredRevenue 是当日已覆盖订单成交额。
	CoveredRevenue float64
	// CostAmount 是当日已覆盖订单商品成本。
	CostAmount float64
	// PlatformFee 是当日已覆盖订单的平台手续费。
	PlatformFee float64
	// GrossProfit 是当日已覆盖订单商品毛利。
	GrossProfit float64
}

// ItemProfitStats 是单个商品的毛利与覆盖率统计。
type ItemProfitStats struct {
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是商品展示标题。
	ItemTitle string
	// TotalOrders 是该商品有效订单数。
	TotalOrders int
	// CoveredOrders 是该商品已匹配成本订单数。
	CoveredOrders int
	// CoveredRevenue 是已覆盖订单成交额。
	CoveredRevenue float64
	// CostAmount 是已覆盖订单商品成本。
	CostAmount float64
	// PlatformFee 是该商品已覆盖订单的平台手续费。
	PlatformFee float64
	// GrossProfit 是已覆盖订单商品毛利。
	GrossProfit float64
	// GrossMarginPercent 是已覆盖订单商品毛利率。
	GrossMarginPercent float64
	// CoveragePercent 是该商品订单成本覆盖率。
	CoveragePercent float64
}

// ManualCostCandidateRecord 是持久化层返回的历史订单金额分组和可选 SKU 原始记录。
type ManualCostCandidateRecord struct {
	// AccountID 是该历史成本分组所属的账号标识。
	AccountID string
	// ItemID 是历史订单关联商品标识。
	ItemID string
	// ItemTitle 是商品展示标题。
	ItemTitle string
	// AmountCents 是该组历史订单成交金额分值。
	AmountCents int64
	// Quantity 是该组每笔订单的购买数量。
	Quantity int
	// OrderCount 是该金额分组尚未覆盖成本的有效订单数。
	OrderCount int
	// SKUs 是当前商品全部已配置成本的平台 SKU。
	SKUs []ManualCostSKU
	// HistoricalSKUID 是同商品同成交金额最近一次人工确认的 SKU。
	HistoricalSKUID string
	// HistoricalUnitCostCents 是同商品同成交金额最近一次人工确认的历史单件成本。
	HistoricalUnitCostCents *int64
}

// ManualHistoricalCostCandidateRecord 是不依赖当前商品或 SKU 的历史订单成本分组。
type ManualHistoricalCostCandidateRecord struct {
	// AccountID 是待确认订单所属账号标识。
	AccountID string
	// ItemID 是订单保留的历史商品标识。
	ItemID string
	// ItemTitle 是可从商品或聊天历史恢复的商品标题。
	ItemTitle string
	// AmountCents 是该组单笔历史成交金额分值。
	AmountCents int64
	// Quantity 是该组每笔订单的购买数量。
	Quantity int
	// OrderCount 是该精确分组中待覆盖订单数。
	OrderCount int
}

// ManualCostSKU 是人工候选界面可选择的平台 SKU。
type ManualCostSKU struct {
	// SKUID 是平台 SKU 标识。
	SKUID string
	// Label 是规格属性组合后的展示名称。
	Label string
	// PriceCents 是当前平台标准售价分值。
	PriceCents int64
	// CostCents 是当前本地单件成本分值。
	CostCents int64
}

// ManualCostCandidate 是应用层计算建议和置信度后的人工确认分组。
type ManualCostCandidate struct {
	// AccountID 是历史成本候选所属账号标识。
	AccountID string
	// ItemID 是历史订单关联商品标识。
	ItemID string
	// ItemTitle 是商品展示标题。
	ItemTitle string
	// AmountCents 是该组历史成交金额分值。
	AmountCents int64
	// Quantity 是该组每笔订单的购买数量。
	Quantity int
	// OrderCount 是等待人工确认的订单数。
	OrderCount int
	// SuggestedSKUID 是价格距离最小且唯一的候选 SKU。
	SuggestedSKUID string
	// Confidence 是 exact、adjusted 或 ambiguous。
	Confidence string
	// SKUs 是供用户最终选择的全部成本 SKU。
	SKUs []ManualCostSKU
	// SuggestedUnitCostCents 是历史确认优先、当前 SKU 成本回退的建议单件成本。
	SuggestedUnitCostCents int64
	// ManualOnly 表示当前分组没有可引用 SKU，必须由用户填写历史单件成本。
	ManualOnly bool
}

// ManualCostConfirmation 是人工确认一个商品金额分组对应 SKU 的输入。
type ManualCostConfirmation struct {
	// UserID 是当前 ERP 用户标识。
	UserID int64
	// AccountID 是可选的账号范围；空值保留全部账号的历史行为。
	AccountID string
	// ItemID 是待确认商品标识。
	ItemID string
	// AmountCents 是待确认金额分组的成交金额分值。
	AmountCents int64
	// Quantity 是待确认分组的单笔购买数量。
	Quantity int
	// StartAt 是当前 Dashboard 日期范围的包含 UTC 起点。
	StartAt string
	// EndBefore 是当前 Dashboard 日期范围的排除 UTC 终点。
	EndBefore string
	// SKUID 是用户最终选择的平台 SKU。
	SKUID string
	// CustomUnitCostCents 是用户为该历史金额分组填写的单件成本分值。
	CustomUnitCostCents *int64
}

// ManualCostConfirmationResult 是批量锁定人工成本快照的结果。
type ManualCostConfirmationResult struct {
	// MatchedOrders 是本次成功生成快照的订单数。
	MatchedOrders int
	// MatchSource 是 manual_exact_price 或 manual_adjusted_price。
	MatchSource string
}

// DailyStats 是单个本地日期的订单聚合结果。
type DailyStats struct {
	// Date 是用户本地日期。
	Date string
	// OrderCount 是该日期的订单数。
	OrderCount int
	// Amount 是该日期的金额合计。
	Amount float64
}

// StatusStats 是单个归一化状态的订单聚合结果。
type StatusStats struct {
	// Status 是归一化后的状态名称。
	Status string
	// Count 是该状态的订单数。
	Count int
	// Amount 是该状态的金额合计。
	Amount float64
}

// CityStats 是单个收货城市的订单聚合结果。
type CityStats struct {
	// City 是收货城市名称。
	City string
	// OrderCount 是该城市的订单数。
	OrderCount int
	// TotalAmount 是该城市的金额合计。
	TotalAmount float64
}

// ItemStats 是单个商品的订单聚合结果。
type ItemStats struct {
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是供历史经营统计展示的商品标题；为空时客户端才回退商品标识。
	ItemTitle string
	// OrderCount 是该商品的订单数。
	OrderCount int
	// TotalAmount 是该商品的金额合计。
	TotalAmount float64
	// AvgAmount 是该商品的平均金额。
	AvgAmount float64
}

// ValidOrders 是有效订单分页结果。
type ValidOrders struct {
	// Orders 是当前页的有效订单明细。
	Orders []ValidOrder
	// Total 是筛选条件下的有效订单总数。
	Total int
	// Page 是当前页码。
	Page int
	// PageSize 是当前页大小。
	PageSize int
	// Truncated 表示结果是否还有未返回的订单。
	Truncated bool
}

// ValidOrder 是面向 HTTP 映射的有效订单明细模型。
type ValidOrder struct {
	// OrderID 是平台订单标识。
	OrderID string
	// ItemID 是平台商品标识。
	ItemID string
	// BuyerID 是买家平台标识。
	BuyerID string
	// ItemTitle 是本地商品标题。
	ItemTitle string
	// ItemImage 是从商品详情提取的主图地址。
	ItemImage string
	// Quantity 是订单数量文本。
	Quantity string
	// Amount 是订单金额文本。
	Amount string
	// OrderStatus 是兼容字段，值为归一化状态名称。
	OrderStatus string
	// Status 是归一化状态名称。
	Status string
	// CookieID 是订单所属账号标识。
	CookieID string
	// CreatedAt 是订单创建时间文本。
	CreatedAt string
}

// Repository 定义订单分析用例需要的最小查询能力。
type Repository interface {
	// DashboardStats 返回用户范围内的仪表盘计数。
	DashboardStats(ctx context.Context, userID int64) (DashboardStats, error)
	// AvailableCardStock 返回用户启用数据卡密组中的可用卡密行数。
	AvailableCardStock(ctx context.Context, userID int64) (int64, error)
	// QueryRevenue 返回收益汇总。
	QueryRevenue(ctx context.Context, filter Filter) (RevenueStats, error)
	// QueryDaily 返回订单日期聚合所需的原始记录。
	QueryDaily(ctx context.Context, filter Filter) ([]DailyRecord, error)
	// QueryStatus 返回订单状态聚合结果。
	QueryStatus(ctx context.Context, filter Filter) ([]StatusRecord, error)
	// QueryCity 返回收货城市聚合结果。
	QueryCity(ctx context.Context, filter Filter) ([]CityRecord, error)
	// QueryItem 返回商品聚合结果。
	QueryItem(ctx context.Context, filter Filter) ([]ItemRecord, error)
	// QueryProfitRecords 返回订单毛利聚合所需的订单和可选成本快照。
	QueryProfitRecords(ctx context.Context, filter Filter) ([]ProfitRecord, error)
	// QueryManualCostCandidates 返回当前用户在可选账号范围内仍缺成本的多规格历史订单金额分组。
	QueryManualCostCandidates(ctx context.Context, filter Filter) ([]ManualCostCandidateRecord, error)
	// QueryManualHistoricalCostCandidates 返回不依赖当前商品或 SKU 的全部未覆盖历史订单分组。
	QueryManualHistoricalCostCandidates(ctx context.Context, filter Filter) ([]ManualHistoricalCostCandidateRecord, error)
	// ConfirmManualCostCandidate 为用户确认的金额分组生成不可变订单成本快照。
	ConfirmManualCostCandidate(ctx context.Context, confirmation ManualCostConfirmation) (ManualCostConfirmationResult, error)
	// ConfirmManualHistoricalCostCandidate 为无可引用 SKU 的精确历史分组生成订单级不可变成本覆盖。
	ConfirmManualHistoricalCostCandidate(ctx context.Context, confirmation ManualCostConfirmation) (ManualCostConfirmationResult, error)
	// CountValidOrders 返回有效订单总数。
	CountValidOrders(ctx context.Context, filter Filter) (int, error)
	// ListValidOrders 返回有效订单分页明细。
	ListValidOrders(ctx context.Context, filter Filter, limit, offset int) ([]ValidOrderRecord, error)
}
