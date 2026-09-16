package server

// analyticsRevenueStatsResponse 是订单收益统计的具名 DTO。
type analyticsRevenueStatsResponse struct {
	TotalOrders  int     `json:"total_orders"`  // TotalOrders 是统计范围内的订单数。
	TotalAmount  float64 `json:"total_amount"`  // TotalAmount 是统计范围内的订单总金额。
	AvgAmount    float64 `json:"avg_amount"`    // AvgAmount 是订单平均金额。
	UniqueBuyers int     `json:"unique_buyers"` // UniqueBuyers 是买家数量。
	UniqueItems  int     `json:"unique_items"`  // UniqueItems 是商品数量。
}

// analyticsDailyStatsResponse 是按日期聚合的订单统计 DTO。
type analyticsDailyStatsResponse struct {
	Date       string  `json:"date"`        // Date 是用户本地日期。
	OrderCount int     `json:"order_count"` // OrderCount 是当天订单数。
	Amount     float64 `json:"amount"`      // Amount 是当天订单金额。
}

// analyticsStatusStatsResponse 是按订单状态聚合的统计 DTO。
type analyticsStatusStatsResponse struct {
	Status string  `json:"status"` // Status 是归一化后的订单状态。
	Count  int     `json:"count"`  // Count 是该状态订单数。
	Amount float64 `json:"amount"` // Amount 是该状态订单金额。
}

// analyticsCityStatsResponse 是按收货城市聚合的统计 DTO。
type analyticsCityStatsResponse struct {
	City        string  `json:"city"`         // City 是收货城市。
	OrderCount  int     `json:"order_count"`  // OrderCount 是该城市订单数。
	TotalAmount float64 `json:"total_amount"` // TotalAmount 是该城市订单金额。
}

// analyticsItemStatsResponse 是按商品聚合的统计 DTO。
type analyticsItemStatsResponse struct {
	ItemID      string  `json:"item_id"`      // ItemID 是商品平台标识。
	ItemTitle   string  `json:"item_title"`   // ItemTitle 是当前或历史本地记录解析出的商品标题。
	OrderCount  int     `json:"order_count"`  // OrderCount 是该商品订单数。
	TotalAmount float64 `json:"total_amount"` // TotalAmount 是该商品订单金额。
	AvgAmount   float64 `json:"avg_amount"`   // AvgAmount 是该商品订单平均金额。
}

// analyticsProfitStatsResponse 是仅基于已锁定成本快照计算的商品毛利汇总 DTO。
type analyticsProfitStatsResponse struct {
	TotalOrders         int     `json:"total_orders"`          // TotalOrders 是统计范围内可计入覆盖率分母的有效订单数。
	CoveredOrders       int     `json:"covered_orders"`        // CoveredOrders 是存在精确成本快照的订单数。
	UnknownCostOrders   int     `json:"unknown_cost_orders"`   // UnknownCostOrders 是尚未精确匹配成本的订单数。
	CoveredRevenue      float64 `json:"covered_revenue"`       // CoveredRevenue 是已匹配成本订单的成交金额。
	ProductCost         float64 `json:"product_cost"`          // ProductCost 是已匹配成本订单的商品成本。
	PlatformFee         float64 `json:"platform_fee"`          // PlatformFee 是已覆盖订单按 1.6% 计算的平台手续费。
	GrossProfit         float64 `json:"gross_profit"`          // GrossProfit 是已匹配成本订单的预估商品毛利。
	RealizedGrossProfit float64 `json:"realized_gross_profit"` // RealizedGrossProfit 是已完成或已收货订单的预估商品毛利。
	GrossMargin         float64 `json:"gross_margin"`          // GrossMargin 是已覆盖成交额对应的毛利率百分比。
	CoverageRate        float64 `json:"coverage_rate"`         // CoverageRate 是成本覆盖率百分比。
}

// analyticsDailyProfitStatsResponse 是按日期聚合的预估商品毛利 DTO。
type analyticsDailyProfitStatsResponse struct {
	Date           string  `json:"date"`            // Date 是用户本地日期。
	TotalOrders    int     `json:"total_orders"`    // TotalOrders 是当天有效订单数。
	CoveredOrders  int     `json:"covered_orders"`  // CoveredOrders 是当天已匹配成本订单数。
	TotalRevenue   float64 `json:"total_revenue"`   // TotalRevenue 是当天全部有效订单成交额。
	CoveredRevenue float64 `json:"covered_revenue"` // CoveredRevenue 是当天已匹配成本订单成交额。
	ProductCost    float64 `json:"product_cost"`    // ProductCost 是当天已匹配成本订单商品成本。
	PlatformFee    float64 `json:"platform_fee"`    // PlatformFee 是当天已覆盖订单的平台手续费。
	GrossProfit    float64 `json:"gross_profit"`    // GrossProfit 是当天已匹配成本订单预估商品毛利。
}

// analyticsItemProfitStatsResponse 是按商品聚合的预估商品毛利 DTO。
type analyticsItemProfitStatsResponse struct {
	ItemID         string  `json:"item_id"`         // ItemID 是商品平台标识。
	ItemTitle      string  `json:"item_title"`      // ItemTitle 是商品标题。
	TotalOrders    int     `json:"total_orders"`    // TotalOrders 是该商品有效订单数。
	CoveredOrders  int     `json:"covered_orders"`  // CoveredOrders 是该商品已匹配成本订单数。
	CoveredRevenue float64 `json:"covered_revenue"` // CoveredRevenue 是该商品已匹配成本订单成交额。
	ProductCost    float64 `json:"product_cost"`    // ProductCost 是该商品已匹配成本订单商品成本。
	PlatformFee    float64 `json:"platform_fee"`    // PlatformFee 是该商品已覆盖订单的平台手续费。
	GrossProfit    float64 `json:"gross_profit"`    // GrossProfit 是该商品已匹配成本订单预估商品毛利。
	GrossMargin    float64 `json:"gross_margin"`    // GrossMargin 是该商品已覆盖成交额对应的毛利率百分比。
	CoverageRate   float64 `json:"coverage_rate"`   // CoverageRate 是该商品成本覆盖率百分比。
}

// orderAnalyticsResponse 是订单分析接口的具名响应 DTO。
type orderAnalyticsResponse struct {
	RevenueStats     analyticsRevenueStatsResponse       `json:"revenue_stats"`      // RevenueStats 是收益统计。
	DailyStats       []analyticsDailyStatsResponse       `json:"daily_stats"`        // DailyStats 是按日统计。
	StatusStats      []analyticsStatusStatsResponse      `json:"status_stats"`       // StatusStats 是按状态统计。
	CityStats        []analyticsCityStatsResponse        `json:"city_stats"`         // CityStats 是按城市统计。
	ItemStats        []analyticsItemStatsResponse        `json:"item_stats"`         // ItemStats 是按商品统计。
	ProfitStats      analyticsProfitStatsResponse        `json:"profit_stats"`       // ProfitStats 是仅按精确成本快照计算的商品毛利汇总。
	DailyProfitStats []analyticsDailyProfitStatsResponse `json:"daily_profit_stats"` // DailyProfitStats 是按日统计的商品毛利。
	ItemProfitStats  []analyticsItemProfitStatsResponse  `json:"item_profit_stats"`  // ItemProfitStats 是按商品统计的商品毛利。
}

// manualCostSKUResponse 是人工确认界面可选择的 SKU DTO。
type manualCostSKUResponse struct {
	SKUID      string `json:"sku_id"`      // SKUID 是平台 SKU 标识。
	Label      string `json:"label"`       // Label 是规格展示名称。
	PriceCents int64  `json:"price_cents"` // PriceCents 是当前标准售价分值。
	CostCents  int64  `json:"cost_cents"`  // CostCents 是当前本地成本分值。
}

// manualCostCandidateResponse 是历史订单金额分组人工候选 DTO。
type manualCostCandidateResponse struct {
	AccountID              string                  `json:"account_id"`                // AccountID 是历史订单分组所属账号标识。
	ItemID                 string                  `json:"item_id"`                   // ItemID 是商品标识。
	ItemTitle              string                  `json:"item_title"`                // ItemTitle 是商品标题。
	AmountCents            int64                   `json:"amount_cents"`              // AmountCents 是历史成交金额分值。
	Quantity               int                     `json:"quantity"`                  // Quantity 是该组每笔订单的购买数量。
	OrderCount             int                     `json:"order_count"`               // OrderCount 是待确认订单数。
	SuggestedSKUID         string                  `json:"suggested_sku_id"`          // SuggestedSKUID 是唯一最近价格 SKU。
	Confidence             string                  `json:"confidence"`                // Confidence 是 exact、adjusted 或 ambiguous。
	SKUs                   []manualCostSKUResponse `json:"skus"`                      // SKUs 是全部可选成本 SKU。
	SuggestedUnitCostCents int64                   `json:"suggested_unit_cost_cents"` // SuggestedUnitCostCents 是历史确认优先的建议成本分值。
	ManualOnly             bool                    `json:"manual_only"`               // ManualOnly 表示当前分组无 SKU 可引用，需用户填写历史成本。
}

// manualCostConfirmationRequest 是人工确认金额分组对应 SKU 的请求 DTO。
type manualCostConfirmationRequest struct {
	AccountID           string `json:"account_id"`              // AccountID 是可选的账号范围，空值表示全部账号。
	ItemID              string `json:"item_id"`                 // ItemID 是待确认商品标识。
	AmountCents         int64  `json:"amount_cents"`            // AmountCents 是待确认成交金额分值。
	Quantity            int    `json:"quantity"`                // Quantity 是待确认分组的购买数量。
	StartDate           string `json:"start_date"`              // StartDate 是 Dashboard 当前本地日期起点。
	EndDate             string `json:"end_date"`                // EndDate 是 Dashboard 当前本地日期终点。
	TimezoneOffset      int    `json:"timezone_offset_minutes"` // TimezoneOffset 是浏览器相对 UTC 的分钟偏移。
	SKUID               string `json:"sku_id"`                  // SKUID 是用户最终选择的平台 SKU。
	CustomUnitCostCents *int64 `json:"custom_unit_cost_cents"`  // CustomUnitCostCents 是用户填写的历史单件成本分值。
}

// manualCostConfirmationResponse 是人工成本快照批量生成结果 DTO。
type manualCostConfirmationResponse struct {
	Success       bool   `json:"success"`        // Success 表示确认已成功提交。
	MatchedOrders int    `json:"matched_orders"` // MatchedOrders 是新生成快照的订单数。
	MatchSource   string `json:"match_source"`   // MatchSource 是人工匹配来源。
}
