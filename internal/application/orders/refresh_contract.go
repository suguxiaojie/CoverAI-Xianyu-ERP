package orders

import "context"

const (
	// RefreshModeIncremental 表示默认只扫描最新订单到可信历史边界。
	RefreshModeIncremental = "incremental"
	// RefreshModeFull 表示读取完整远端快照并允许清理缺失订单。
	RefreshModeFull = "full"
	// RefreshModeCostBackfill 表示只读取本地缺少规格且存在多规格成本的历史订单详情。
	RefreshModeCostBackfill = "cost_backfill"
)

// OrderSyncCursor 保存应用层可见的账号订单增量同步边界，不包含任何平台凭证。
type OrderSyncCursor struct {
	// CookieID 是游标所属账号标识。
	CookieID string
	// HighWaterCreatedAt 是已落库订单的最新平台创建时间。
	HighWaterCreatedAt string
	// HighWaterOrderID 用于稳定区分创建时间相同的订单。
	HighWaterOrderID string
	// LastIncrementalSyncAt 是最近增量同步完成的 Unix 秒。
	LastIncrementalSyncAt int64
	// LastFullSyncAt 是最近完整校准完成的 Unix 秒。
	LastFullSyncAt int64
}

// RefreshDetail 是平台返回的订单详情字段。
type RefreshDetail struct {
	// Quantity 是订单购买数量。
	Quantity string
	// SpecName 是商品规格名称。
	SpecName string
	// SpecValue 是商品规格值。
	SpecValue string
	// OrderStatus 是平台返回的订单状态。
	OrderStatus string
	// Amount 是订单实付金额。
	Amount string
	// UpdatedCookies 是非完整 Cookie Jar 流程返回的更新 Cookie。
	UpdatedCookies string
}

// RefreshSoldOrder 是平台订单列表可直接落库的字段。
type RefreshSoldOrder struct {
	// OrderID 是订单业务标识。
	OrderID string
	// ItemID 是关联商品标识。
	ItemID string
	// BuyerID 是买家标识。
	BuyerID string
	// OrderStatus 是归一化后的订单状态。
	OrderStatus string
	// Quantity 是购买数量。
	Quantity string
	// Amount 是订单金额。
	Amount string
	// ReceiverName 是收货人姓名。
	ReceiverName string
	// ReceiverPhone 是收货人电话。
	ReceiverPhone string
	// ReceiverAddr 是收货地址。
	ReceiverAddr string
	// ReceiverCity 是收货城市。
	ReceiverCity string
	// IsBargain 表示是否为砍价订单。
	IsBargain bool
	// CreatedAt 是平台订单创建时间的 UTC RFC3339 文本；为空时保留本地现值。
	CreatedAt string
}

// RefreshDetailFetchResult 是一次订单详情请求及其 Cookie 会话结果。
type RefreshDetailFetchResult struct {
	// Detail 是平台返回的订单详情，可为空。
	Detail *RefreshDetail
	// CookieUpdate 是请求期间观察到的 Cookie 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// RefreshSoldFetchResult 是一次订单列表请求及其 Cookie 会话结果。
type RefreshSoldFetchResult struct {
	// Orders 是平台返回的全部已售订单。
	Orders []RefreshSoldOrder
	// CookieUpdate 是请求期间观察到的 Cookie 会话变化。
	CookieUpdate RefreshCookieUpdate
	// CompleteSnapshot 表示本次读取覆盖了平台全部分页，只有此时才允许清理缺失订单。
	CompleteSnapshot bool
	// BoundaryReached 表示增量读取已命中可信历史边界并提前结束。
	BoundaryReached bool
}

// RefreshCookieUpdate 描述平台请求观察到的 Cookie Jar 更新。
type RefreshCookieUpdate struct {
	// Value 是更新后的平台 Cookie 值。
	Value string
	// MetadataJSON 是更新后的 Cookie 元数据。
	MetadataJSON string
	// Changed 表示请求期间 Cookie 会话是否发生变化。
	Changed bool
	// Handled 表示本次请求是否由完整 Cookie Jar 接管。
	Handled bool
}

// RefreshOrderWrite 描述详情分片批量写入的一条订单记录。
type RefreshOrderWrite struct {
	// OrderID 是待写入订单标识。
	OrderID string
	// Options 是本次详情刷新需要更新的订单字段。
	Options UpsertOptions
}

// RefreshOrderResult 描述订单刷新结果中的单条兼容结果。
type RefreshOrderResult struct {
	// CookieID 是账号刷新结果对应的账号标识。
	CookieID string
	// OrderID 是订单刷新结果对应的订单标识。
	OrderID string
	// Stage 是 discover、detail 或 persist_cookie 阶段。
	Stage string
	// Success 表示当前结果是否成功。
	Success bool
	// Message 是结果提示文本。
	Message string
	// Error 是结果失败详情文本。
	Error string
	// Discovered 是本账号发现的新订单数。
	Discovered int
	// Updated 是本账号订单列表发生变化的数量。
	Updated int
	// SoftDeleted 是本账号被标记删除的订单数。
	SoftDeleted int
	// ConflictSkipped 是因订单已归属其他账号而安全跳过的数量。
	ConflictSkipped int
	// OldStatus 是详情刷新前的订单状态。
	OldStatus string
	// NewStatus 是详情刷新后的订单状态。
	NewStatus string
}

// RefreshSummary 是批量刷新统计摘要。
type RefreshSummary struct {
	// AccountTotal 是本次同步尝试的账号数量。
	AccountTotal int
	// AccountSucceeded 是订单发现阶段成功完成的账号数量。
	AccountSucceeded int
	// AccountFailed 是订单发现阶段失败的账号数量。
	AccountFailed int
	// Discovered 是发现的新订单数量。
	Discovered int
	// ListUpdated 是订单列表更新数量。
	ListUpdated int
	// SoftDeleted 是标记删除数量。
	SoftDeleted int
	// DetailTotal 是需要补全详情的订单数量。
	DetailTotal int
	// Total 是本次处理订单总数。
	Total int
	// Updated 是状态发生变化的订单数量。
	Updated int
	// NoChange 是状态未发生变化的订单数量。
	NoChange int
	// Failed 是刷新失败数量。
	Failed int
	// ConflictSkipped 是未覆盖其他账号归属而安全跳过的远端订单数量。
	ConflictSkipped int
}

// RefreshResult 是批量订单刷新应用结果。
type RefreshResult struct {
	// PartialFailure 表示批量刷新是否存在失败。
	PartialFailure bool
	// Message 是刷新结果说明。
	Message string
	// Summary 是刷新统计摘要。
	Summary RefreshSummary
	// Results 是逐账号或逐订单的结果。
	Results []RefreshOrderResult
}

// SingleRefreshResult 是单订单刷新应用结果。
type SingleRefreshResult struct {
	// Success 表示刷新是否完成。
	Success bool
	// Message 是刷新结果说明。
	Message string
	// Detail 是刷新后的订单详情。
	Detail RefreshDetail
}

// RefreshRepository 定义订单刷新用例所需的最小持久化能力。
type RefreshRepository interface {
	// ExistsOwned 判断账号是否归属于用户。
	ExistsOwned(ctx context.Context, userID int64, cookieID string) (bool, error)
	// ListOwnedIDs 返回用户拥有的账号标识。
	ListOwnedIDs(ctx context.Context, userID int64) ([]string, error)
	// GetOrder 读取订单实体。
	GetOrder(ctx context.Context, orderID string) (*Order, error)
	// FindOrder 读取订单实体并以 exists 区分不存在。
	FindOrder(ctx context.Context, orderID string) (*Order, bool, error)
	// FindOrdersByIDs 批量读取订单实体，避免订单发现逐单查询。
	FindOrdersByIDs(ctx context.Context, orderIDs []string) (map[string]*Order, error)
	// LockCredentials 获取账号凭证互斥锁。
	LockCredentials(cookieID string) func()
	// LoadCookiePlatformDetail 读取平台请求所需的账号视图。
	LoadCookiePlatformDetail(ctx context.Context, cookieID string) (*PlatformRuntimeData, error)
	// UpdateRenewalCookie 保存扁平 Cookie 和元数据。
	UpdateRenewalCookie(ctx context.Context, cookieID, value, metadata string, at int64) error
	// UpsertOrder 写入订单刷新结果。
	UpsertOrder(ctx context.Context, orderID string, options UpsertOptions) error
	// BatchUpsertOrders 使用单条多值 UPSERT 写入详情分片。
	BatchUpsertOrders(ctx context.Context, rows []RefreshOrderWrite) error
	// SoftDeleteMissingOrders 标记远端订单列表中缺失的本地订单。
	SoftDeleteMissingOrders(ctx context.Context, cookieID string, activeIDs map[string]struct{}) (int, error)
	// ListOrdersByCookieCursor 使用复合游标读取账号订单。
	ListOrdersByCookieCursor(ctx context.Context, cookieID string, limit int, afterCreatedAt, afterOrderID string) ([]OrderRow, error)
	// CountCostedPlatformSKUs 返回商品当前已配置成本的平台 SKU 数量，不包含本地默认 SKU。
	CountCostedPlatformSKUs(ctx context.Context, cookieID, itemID string) (int, error)
	// HasOrderCostSnapshot 判断订单是否已经完成任一可靠或人工成本匹配。
	HasOrderCostSnapshot(ctx context.Context, orderID string) (bool, error)
	// GetOrderSyncCursor 读取账号增量同步边界；不存在时 exists=false。
	GetOrderSyncCursor(ctx context.Context, cookieID string) (*OrderSyncCursor, bool, error)
	// UpsertOrderSyncCursor 保存成功同步后的账号高水位和校准时间。
	UpsertOrderSyncCursor(ctx context.Context, cursor OrderSyncCursor) error
}

// RefreshRuntime 定义订单刷新访问平台和运行时能力的最小 Port。
type RefreshRuntime interface {
	// DetailAvailable 判断平台详情接口是否可用。
	DetailAvailable() bool
	// SoldAvailable 判断平台已售订单接口是否可用。
	SoldAvailable() bool
	// CredentialAvailable 判断账号是否有可用平台凭证。
	CredentialAvailable(detail *PlatformRuntimeData) bool
	// FetchOrderDetail 请求单个订单详情。
	FetchOrderDetail(ctx context.Context, detail *PlatformRuntimeData, orderID string) (RefreshDetailFetchResult, error)
	// FetchSoldOrders 请求账号全部已售订单列表。
	FetchSoldOrders(ctx context.Context, detail *PlatformRuntimeData) (RefreshSoldFetchResult, error)
	// PersistCookieSession 在凭证锁内保存完整 Cookie 会话。
	PersistCookieSession(ctx context.Context, detail *PlatformRuntimeData, update RefreshCookieUpdate) (string, bool, bool, error)
	// UpdateRunningCookie 同步运行时账号 Cookie。
	UpdateRunningCookie(ctx context.Context, cookieID, value string)
	// RecoverExpiredSession 处理平台会话过期。
	RecoverExpiredSession(ctx context.Context, cookieID string, err error) bool
	// IsSessionExpired 判断错误是否表示平台会话过期。
	IsSessionExpired(err error) bool
}
