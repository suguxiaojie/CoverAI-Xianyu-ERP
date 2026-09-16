package db

// DefaultItemSKUID 是单规格商品在本地成本表中的隐式 SKU 标识；平台同步不得删除或发送该值。
const DefaultItemSKUID = "__default__"

// ItemInfoRow 是 item_info 的非敏感商品持久化行。
type ItemInfoRow struct {
	// ID 是本地商品记录主键。
	ID int64
	// CookieID 是商品所属平台账号标识。
	CookieID string
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是商品标题。
	ItemTitle string
	// ItemDescription 是本地商品描述。
	ItemDescription string
	// ItemCategory 是平台类目标识。
	ItemCategory string
	// ItemPrice 是商品价格文本。
	ItemPrice string
	// ItemDetail 是列表摘要 JSON。
	ItemDetail string
	// IsMultiSpec 表示商品当前是否包含销售规格。
	IsMultiSpec bool
	// MultiQuantityDelivery 表示本地发货是否按购买数量执行。
	MultiQuantityDelivery bool
}

// ItemSyncResult 是一次远端商品全集同步的结果。
type ItemSyncResult struct {
	// Saved 是本次写入或更新的有效商品数。
	Saved int
	// Deleted 是本次逻辑删除的本地商品数。
	Deleted int
}

// ItemSKURow 是商品 SKU 的本地持久化模型；CostCents 仅由本地用户维护，平台同步不得覆盖。
type ItemSKURow struct {
	// ID 是本地 SKU 记录主键。
	ID int64
	// CookieID 是 SKU 所属平台账号标识。
	CookieID string
	// ItemID 是 SKU 所属平台商品标识。
	ItemID string
	// SKUID 是平台 SKU 标识。
	SKUID string
	// InventoryID 是可能超过 JavaScript 安全整数范围的平台库存标识。
	InventoryID string
	// PropertiesJSON 是规格名称和值组成的 JSON 数组。
	PropertiesJSON string
	// PriceCents 是平台售价，单位为人民币分。
	PriceCents int64
	// Quantity 是卖家编辑页返回的当前剩余库存。
	Quantity int
	// InitialQuantity 是发布时设置的初始库存。
	InitialQuantity int
	// Enabled 表示平台当前是否允许选择该 SKU。
	Enabled bool
	// SortOrder 是平台返回的 SKU 展示顺序。
	SortOrder int
	// CostCents 是本地维护的单件成本分值；nil 表示尚未填写。
	CostCents *int64
	// SyncedAt 是平台 SKU 快照写入时间，Unix 秒。
	SyncedAt int64
}

// ItemSKUCostUpdate 是一次本地成本批量保存中的单个 SKU 更新。
type ItemSKUCostUpdate struct {
	// SKUID 是平台 SKU 或单规格商品的本地隐式 SKU 标识。
	SKUID string
	// CostCents 是可空单件成本；nil 表示清除本地成本。
	CostCents *int64
}

// RemoteItemRecord 是一次平台商品同步中的商品和可选 SKU 快照。
type RemoteItemRecord struct {
	// Item 是平台商品基础字段。
	Item ItemInfoRow
	// SKUDetailLoaded 表示卖家编辑详情读取成功；失败时必须保留旧 SKU。
	SKUDetailLoaded bool
	// SKUs 是详情读取成功后返回的 SKU 快照，空切片表示商品当前无 SKU。
	SKUs []ItemSKURow
}
