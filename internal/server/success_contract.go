package server

// operationResponse 是密码、会话和设置变更接口共用的操作结果 DTO。
type operationResponse struct {
	// Success 表示操作是否完成。
	Success bool `json:"success"`
	// Message 是可以直接展示给用户的操作结果说明。
	Message string `json:"message,omitempty"`
	// RequiresRelogin 表示操作完成后当前会话已被撤销，需要重新登录。
	RequiresRelogin bool `json:"requires_relogin,omitempty"`
}

// longLoginResponse 是长登录设置接口的具名响应 DTO，不包含平台 Cookie。
type longLoginResponse struct {
	// CanOpenLongLogin 表示平台是否允许开启长登录。
	CanOpenLongLogin bool `json:"can_open_long_login"`
	// Enabled 表示平台当前是否已开启长登录。
	Enabled bool `json:"enabled"`
}

// messageResponse 是只返回提示文本的简单成功响应 DTO。
type messageResponse struct {
	// Message 是接口操作结果说明。
	Message string `json:"message"`
}

// sessionVerificationResponse 是会话校验接口的具名响应 DTO。
type sessionVerificationResponse struct {
	// Authenticated 表示当前请求是否带有有效会话。
	Authenticated bool `json:"authenticated"`
	// Initialized 表示系统是否已经完成管理员初始化。
	Initialized bool `json:"initialized"`
	// UserID 是当前会话用户 ID；未认证时为空值。
	UserID int64 `json:"user_id,omitempty"`
	// Username 是当前会话用户名；未认证时为空字符串。
	Username string `json:"username,omitempty"`
	// IsAdmin 表示当前会话用户是否为管理员。
	IsAdmin bool `json:"is_admin"`
}

// accountMutationResponse 是账号新增等简单变更接口的具名成功响应 DTO。
type accountMutationResponse struct {
	// Success 表示账号变更是否完成。
	Success bool `json:"success"`
	// ID 是新增或更新账号的稳定标识。
	ID string `json:"id,omitempty"`
}

// mutationIDResponse 是使用数值主键的资源变更接口成功响应 DTO。
type mutationIDResponse struct {
	// Success 表示资源变更是否完成。
	Success bool `json:"success"`
	// ID 是新建资源的数值主键。
	ID int64 `json:"id"`
}

// orderDTO 是订单列表和详情接口共用的具名响应 DTO。
type orderDTO struct {
	// OrderID 是平台订单标识。
	OrderID string `json:"order_id"`
	// ItemID 是关联商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是关联商品标题。
	ItemTitle string `json:"item_title"`
	// ItemImage 是关联商品图片地址。
	ItemImage string `json:"item_image"`
	// BuyerID 是买家平台标识。
	BuyerID string `json:"buyer_id"`
	// ChatID 是订单精确关联的聊天会话标识；空值表示尚未建立可靠关联。
	ChatID string `json:"chat_id,omitempty"`
	// BuyerName 是本地聊天会话中最近解析到的闲鱼买家昵称。
	BuyerName string `json:"buyer_name,omitempty"`
	// BuyerAvatarURL 是本地聊天会话中最近解析到的买家头像。
	BuyerAvatarURL string `json:"buyer_avatar_url,omitempty"`
	// SpecName 是商品规格名称。
	SpecName string `json:"spec_name"`
	// SpecValue 是商品规格值。
	SpecValue string `json:"spec_value"`
	// Quantity 是购买数量。
	Quantity string `json:"quantity"`
	// Amount 是订单实付金额。
	Amount string `json:"amount"`
	// OrderStatus 是归一化后的订单状态。
	OrderStatus string `json:"order_status"`
	// Status 是兼容前端使用的订单状态别名。
	Status string `json:"status"`
	// CookieID 是订单所属账号标识。
	CookieID string `json:"cookie_id"`
	// IsBargain 表示订单是否来自议价。
	IsBargain int `json:"is_bargain"`
	// SystemShipped 表示是否由系统完成发货。
	SystemShipped bool `json:"system_shipped"`
	// ShipmentProofAvailable 表示当前订单有 ERP 保存的发货凭证可查看。
	ShipmentProofAvailable bool `json:"shipment_proof_available"`
	// ShipmentProofUnavailableReason 是凭证按钮禁用时的明确原因。
	ShipmentProofUnavailableReason string `json:"shipment_proof_unavailable_reason,omitempty"`
	// ReceiverName 是收货人姓名。
	ReceiverName string `json:"receiver_name"`
	// ReceiverPhone 是收货人电话。
	ReceiverPhone string `json:"receiver_phone"`
	// ReceiverAddress 是收货地址。
	ReceiverAddress string `json:"receiver_address"`
	// ReceiverCity 是收货城市。
	ReceiverCity string `json:"receiver_city"`
	// CreatedAt 是订单创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是订单更新时间。
	UpdatedAt string `json:"updated_at"`
	// PaidAt 是平台明确付款的时间。
	PaidAt string `json:"paid_at,omitempty"`
	// ShippedAt 是卖家明确发货的时间。
	ShippedAt string `json:"shipped_at,omitempty"`
	// ReceivedAt 是买家明确确认收货的时间。
	ReceivedAt string `json:"received_at,omitempty"`
	// CompletedAt 是平台明确交易完成的时间。
	CompletedAt string `json:"completed_at,omitempty"`
	// RefundedAt 是平台明确退款成功的时间。
	RefundedAt string `json:"refunded_at,omitempty"`
	// CancelledAt 是普通取消订单的时间。
	CancelledAt string `json:"cancelled_at,omitempty"`
}

// orderListResponse 是订单分页接口的具名响应 DTO。
type orderListResponse struct {
	// Success 表示查询是否完成。
	Success bool `json:"success"`
	// Data 是当前页订单。
	Data []orderDTO `json:"data"`
	// Total 是符合筛选条件的订单总数。
	Total int `json:"total"`
	// Page 是当前页码。
	Page int `json:"page"`
	// PageSize 是当前页大小。
	PageSize int `json:"page_size"`
	// TotalPages 是总页数。
	TotalPages int `json:"total_pages"`
	// SettlementSummary 是当前筛选范围内固定按已发货状态计算的待结算统计。
	SettlementSummary orderSettlementSummaryResponse `json:"settlement_summary"`
}

// orderSettlementSummaryResponse 是订单中心待结算统计的具名响应 DTO。
type orderSettlementSummaryResponse struct {
	// OrderCount 是参与计算的已发货订单数。
	OrderCount int `json:"order_count"`
	// GrossAmount 是扣费前已发货订单总额，使用两位小数人民币元文本。
	GrossAmount string `json:"gross_amount"`
	// ServiceFee 是按汇总总额一次性计算的 1.6% 平台服务费。
	ServiceFee string `json:"service_fee"`
	// PendingAmount 是总额扣除平台服务费后的待结算金额。
	PendingAmount string `json:"pending_amount"`
	// ServiceFeeRate 是便于客户端明确展示的固定费率文本。
	ServiceFeeRate string `json:"service_fee_rate"`
}

// cookieDetailResponse 是单个账号非敏感详情接口的具名响应 DTO。
type cookieDetailResponse struct {
	// ID 是账号稳定标识。
	ID string `json:"id"`
	// Enabled 表示账号是否允许运行。
	Enabled bool `json:"enabled"`
	// AutoConfirm 表示是否自动确认订单。
	AutoConfirm bool `json:"auto_confirm"`
	// Remark 是账号备注。
	Remark string `json:"remark"`
	// PauseDuration 是暂停时长，单位为分钟。
	PauseDuration int `json:"pause_duration"`
	// PausedUntil 是暂停结束时间的 Unix 秒。
	PausedUntil int64 `json:"paused_until"`
	// Paused 表示账号当前是否处于暂停状态。
	Paused bool `json:"paused"`
	// ShowBrowser 表示登录时显示浏览器，并在 token 风控中等待人工验证。
	ShowBrowser bool `json:"show_browser"`
	// Username 是登录用户名，不包含登录密码。
	Username string `json:"username"`
	// Nickname 是平台账号昵称缓存。
	Nickname string `json:"nickname"`
	// AvatarURL 是平台账号头像地址。
	AvatarURL string `json:"avatar_url"`
	// LoginMethod 是最近一次成功登录方式。
	LoginMethod string `json:"login_method"`
	// LastLoginAt 是最近一次成功登录时间。
	LastLoginAt int64 `json:"last_login_at"`
	// ProfileError 是资料刷新错误说明。
	ProfileError string `json:"profile_error"`
	// HasCookie 表示数据库中存在可用账号记录。
	HasCookie bool `json:"has_cookie"`
	// AutoRateEnabled 表示自动评价计划是否启用。
	AutoRateEnabled bool `json:"auto_rate_enabled"`
	// RateContent 是自动评价文案。
	RateContent string `json:"rate_content"`
	// AutoPolishEnabled 表示自动擦亮计划是否启用。
	AutoPolishEnabled bool `json:"auto_polish_enabled"`
	// PolishTime 是自动擦亮的本地时间。
	PolishTime string `json:"polish_time"`
	// AutoRequestFlowerEnabled 表示发货成功后自动求花是否启用。
	AutoRequestFlowerEnabled bool `json:"auto_request_flower_enabled"`
	// RequestFlowerAfterHours 是旧客户端兼容的小时数。
	RequestFlowerAfterHours int `json:"request_flower_after_hours"`
	// RequestFlowerAfterSeconds 是发货成功后等待的秒数。
	RequestFlowerAfterSeconds int `json:"request_flower_after_seconds"`
	// AutoReceiveFlowerEnabled 表示收到送花卡片后是否自动收花。
	AutoReceiveFlowerEnabled bool `json:"auto_receive_flower_enabled"`
	// ReceiveFlowerShowBrowser 表示自动收花是否显示独立浏览器。
	ReceiveFlowerShowBrowser bool `json:"receive_flower_show_browser"`
	// ReceiveFlowerTimeoutSeconds 是等待平台收花结果的最长秒数。
	ReceiveFlowerTimeoutSeconds int `json:"receive_flower_timeout_seconds"`
	// AutoReceiptReminderEnabled 表示自动确认收货提醒是否启用。
	AutoReceiptReminderEnabled bool `json:"auto_receipt_reminder_enabled"`
	// ReceiptReminderAfterDays 是发货后等待的完整天数。
	ReceiptReminderAfterDays int `json:"receipt_reminder_after_days"`
	// ReceiptReminderTime 是每日北京时间执行时刻。
	ReceiptReminderTime string `json:"receipt_reminder_time"`
	// ReceiptReminderMessage 是旧候选客户端兼容字段；官方系统卡片内容由闲鱼固定。
	ReceiptReminderMessage string `json:"receipt_reminder_message"`
	// ReceiptReminderEnabledAt 是最近一次开启基线 Unix 秒。
	ReceiptReminderEnabledAt int64 `json:"receipt_reminder_enabled_at"`
	// LastRateScanAt 是最近一次自动评价扫描时间。
	LastRateScanAt int64 `json:"last_rate_scan_at"`
	// LastPolishDate 是最近一次自动擦亮日期。
	LastPolishDate string `json:"last_polish_date"`
	// LastPolishAt 是最近一次自动擦亮时间。
	LastPolishAt int64 `json:"last_polish_at"`
}

// cookieSettingsResponse 是账号设置更新接口的具名成功响应 DTO。
type cookieSettingsResponse struct {
	// Success 表示账号设置是否保存成功。
	Success bool `json:"success"`
	// PausedUntil 是新的暂停结束时间 Unix 秒。
	PausedUntil int64 `json:"paused_until"`
	// Paused 表示账号当前是否处于暂停状态。
	Paused bool `json:"paused"`
}

// cookieProfileResponse 是账号资料刷新接口的具名响应 DTO。
type cookieProfileResponse struct {
	// Success 表示资料刷新是否成功。
	Success bool `json:"success"`
	// ID 是账号稳定标识。
	ID string `json:"id"`
	// Nickname 是刷新后的平台昵称。
	Nickname string `json:"nickname"`
	// AvatarURL 是刷新后的头像地址。
	AvatarURL string `json:"avatar_url"`
	// ProfileError 是资料刷新失败原因。
	ProfileError string `json:"profile_error"`
}

// autoConfirmResponse 是账号自动确认设置查询接口的具名响应 DTO。
type autoConfirmResponse struct {
	// AutoConfirm 表示是否自动确认订单。
	AutoConfirm bool `json:"auto_confirm"`
}

// pauseDurationResponse 是账号暂停时长查询接口的具名响应 DTO。
type pauseDurationResponse struct {
	// PauseDuration 是暂停时长，单位为分钟。
	PauseDuration int `json:"pause_duration"`
	// PausedUntil 是暂停结束时间的 Unix 秒。
	PausedUntil int64 `json:"paused_until"`
	// Paused 表示账号当前是否处于暂停状态。
	Paused bool `json:"paused"`
}

// itemListResponse 是本地商品列表接口的具名商品 DTO。
type itemListResponse struct {
	// ID 是本地商品记录主键。
	ID int64 `json:"id"`
	// CookieID 是商品所属账号标识。
	CookieID string `json:"cookie_id"`
	// ItemID 是平台商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是商品标题。
	ItemTitle string `json:"item_title"`
	// ItemDescription 是商品描述。
	ItemDescription string `json:"item_description"`
	// ItemCategory 是商品分类标识。
	ItemCategory string `json:"item_category"`
	// ItemPrice 是商品价格文本。
	ItemPrice string `json:"item_price"`
	// ItemDetail 是商品详情原始 JSON。
	ItemDetail string `json:"item_detail"`
	// ItemImage 是从详情中提取的商品图片地址。
	ItemImage string `json:"item_image"`
	// IsMultiSpec 表示商品是否有多规格。
	IsMultiSpec bool `json:"is_multi_spec"`
	// MultiQuantityDelivery 表示是否按数量发货。
	MultiQuantityDelivery bool `json:"multi_quantity_delivery"`
	// IsMultiQtyShip 是按数量发货的兼容字段。
	IsMultiQtyShip bool `json:"is_multi_qty_ship"`
	// SKUCount 是当前商品未逻辑删除的 SKU 数量。
	SKUCount int `json:"sku_count"`
	// SKUs 是卖家编辑详情同步的 SKU、库存以及仅在本地保存的成本。
	SKUs []itemSKUResponse `json:"skus"`
}

// itemDetailResponse 是单个本地商品详情接口的具名响应 DTO。
type itemDetailResponse struct {
	// CookieID 是商品所属账号标识。
	CookieID string `json:"cookie_id"`
	// ItemID 是平台商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是商品标题。
	ItemTitle string `json:"item_title"`
	// ItemDescription 是商品描述。
	ItemDescription string `json:"item_description"`
	// ItemCategory 是商品分类标识。
	ItemCategory string `json:"item_category"`
	// ItemPrice 是商品价格文本。
	ItemPrice string `json:"item_price"`
	// ItemDetail 是商品详情原始 JSON。
	ItemDetail string `json:"item_detail"`
	// IsMultiSpec 表示商品是否有多规格。
	IsMultiSpec bool `json:"is_multi_spec"`
	// MultiQuantityDelivery 表示是否按数量发货。
	MultiQuantityDelivery bool `json:"multi_quantity_delivery"`
	// SKUCount 是当前商品未逻辑删除的 SKU 数量。
	SKUCount int `json:"sku_count"`
	// SKUs 是当前商品的完整 SKU 列表。
	SKUs []itemSKUResponse `json:"skus"`
}

// itemPublishResponse 是单个商品发布成功接口的具名响应 DTO。
type itemPublishResponse struct {
	// Success 表示商品是否发布成功。
	Success bool `json:"success"`
	// Message 是发布结果说明。
	Message string `json:"message"`
	// ItemID 是新商品的平台标识。
	ItemID string `json:"item_id"`
	// ItemURL 是新商品的平台详情地址。
	ItemURL string `json:"item_url"`
	// ItemImage 是新商品主图地址。
	ItemImage string `json:"item_image"`
	// ItemTitle 是新商品标题。
	ItemTitle string `json:"item_title"`
	// ItemPrice 是新商品价格文本。
	ItemPrice string `json:"item_price"`
	// Quantity 是新商品库存数量。
	Quantity int `json:"quantity"`
	// CategoryID 是新商品分类标识。
	CategoryID string `json:"category_id"`
	// CategoryName 是新商品分类名称。
	CategoryName string `json:"category_name"`
}

// itemSyncResponse 是商品全集同步接口的具名响应 DTO。
type itemSyncResponse struct {
	// Success 表示同步是否完成。
	Success bool `json:"success"`
	// Message 是同步结果说明。
	Message string `json:"message"`
	// TotalCount 是平台返回的商品总数。
	TotalCount int `json:"total_count"`
	// TotalPages 是平台商品总页数。
	TotalPages int `json:"total_pages"`
	// SavedCount 是本地保存的商品数量。
	SavedCount int `json:"saved_count"`
	// DeletedCount 是本地删除标记的商品数量。
	DeletedCount int `json:"deleted_count"`
}

// itemPageSyncResponse 是商品分页同步接口的具名响应 DTO。
type itemPageSyncResponse struct {
	// Success 表示同步是否完成。
	Success bool `json:"success"`
	// Message 是同步结果说明。
	Message string `json:"message"`
	// PageNumber 是当前同步页码。
	PageNumber int `json:"page_number"`
	// PageSize 是当前同步页大小。
	PageSize int `json:"page_size"`
	// CurrentCount 是当前页商品数量。
	CurrentCount int `json:"current_count"`
	// SavedCount 是本地保存的商品数量。
	SavedCount int `json:"saved_count"`
}

// automationActionResponse 是自动化规则动作的具名响应 DTO。
type automationActionResponse struct {
	// ID 是动作稳定标识。
	ID int64 `json:"id"`
	// ActionType 是动作类型。
	ActionType string `json:"action_type"`
	// CardID 是动作关联卡券组标识。
	CardID int64 `json:"card_id"`
	// CardName 是动作关联卡券组名称。
	CardName string `json:"card_name"`
	// DeliveryCount 是动作发送数量。
	DeliveryCount int `json:"delivery_count"`
	// MessageTemplate 是动作消息模板。
	MessageTemplate string `json:"message_template"`
	// DelaySeconds 是动作延迟秒数。
	DelaySeconds int `json:"delay_seconds"`
	// ConfigJSON 是动作扩展配置 JSON。
	ConfigJSON string `json:"config_json"`
	// Enabled 表示动作是否启用。
	Enabled bool `json:"enabled"`
	// SortOrder 是动作执行顺序。
	SortOrder int `json:"sort_order"`
}

// automationRuleResponse 是自动化规则的具名响应 DTO。
type automationRuleResponse struct {
	// ID 是规则稳定标识。
	ID int64 `json:"id"`
	// CookieID 是规则所属账号标识。
	CookieID string `json:"cookie_id"`
	// ItemID 是规则关联商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是规则关联商品标题。
	ItemTitle string `json:"item_title"`
	// Name 是规则名称。
	Name string `json:"name"`
	// TriggerType 是规则触发类型。
	TriggerType string `json:"trigger_type"`
	// Enabled 表示规则是否启用。
	Enabled bool `json:"enabled"`
	// Priority 是规则优先级。
	Priority int `json:"priority"`
	// ConfigJSON 是规则扩展配置 JSON。
	ConfigJSON string `json:"config_json"`
	// Actions 是规则动作列表。
	Actions []automationActionResponse `json:"actions"`
	// CreatedAt 是规则创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是规则更新时间。
	UpdatedAt string `json:"updated_at"`
}

// automationRulePageResponse 是自动化规则分页接口的具名响应 DTO。
type automationRulePageResponse struct {
	// Success 表示查询是否完成。
	Success bool `json:"success"`
	// Data 是当前页自动化规则。
	Data []automationRuleResponse `json:"data"`
	// Total 是符合条件的规则总数。
	Total int `json:"total"`
	// Page 是当前页码。
	Page int `json:"page"`
	// PageSize 是当前页大小。
	PageSize int `json:"page_size"`
	// TotalPages 是总页数。
	TotalPages int `json:"total_pages"`
	// TriggerCounts 是各触发类型的规则数量。
	TriggerCounts map[string]int `json:"trigger_counts"`
}

// automationRunIssueDTO 是自动化异常运行接口对外暴露的具名 DTO。
type automationRunIssueDTO struct {
	// ID 是自动化运行的稳定标识。
	ID int64 `json:"id"`
	// CookieID 是关联账号标识。
	CookieID string `json:"cookie_id"`
	// OrderID 是关联订单标识。
	OrderID string `json:"order_id"`
	// TriggerType 是触发运行的事件类型。
	TriggerType string `json:"trigger_type"`
	// ErrorMessage 是运行进入人工处理状态时记录的原因。
	ErrorMessage string `json:"error_message"`
	// IssueKind 是应用层归类的异常类型。
	IssueKind string `json:"issue_kind"`
	// AllowedResolutions 是当前异常允许的人工处理动作。
	AllowedResolutions []string `json:"allowed_resolutions"`
	// ActionCursor 是下一步动作在计划中的位置。
	ActionCursor int `json:"action_cursor"`
	// SentCount 是已经确认成功的外部动作数量。
	SentCount int `json:"sent_count"`
	// UpdatedAt 是运行状态最近更新的时间文本。
	UpdatedAt string `json:"updated_at"`
}

// deferredAutomationIssueDTO 是延期任务接口对外暴露的具名 DTO。
type deferredAutomationIssueDTO struct {
	// ID 是延期任务的稳定标识。
	ID int64 `json:"id"`
	// CookieID 是关联账号标识。
	CookieID string `json:"cookie_id"`
	// TriggerType 是触发延期任务的事件类型。
	TriggerType string `json:"trigger_type"`
	// ErrorMessage 是任务进入死信状态时记录的原因。
	ErrorMessage string `json:"error_message"`
	// AttemptCount 是任务已经尝试执行的次数。
	AttemptCount int `json:"attempt_count"`
	// UpdatedAt 是任务状态最近更新的时间文本。
	UpdatedAt string `json:"updated_at"`
}

// automationIssuesResponse 是自动化异常任务接口的具名响应 DTO。
type automationIssuesResponse struct {
	// Runs 是需要处理的自动化运行记录。
	Runs []automationRunIssueDTO `json:"runs"`
	// PendingTasks 是需要处理的延迟任务记录。
	PendingTasks []deferredAutomationIssueDTO `json:"pending_tasks"`
}

// orderDetailResponse 是订单详情接口的具名响应 DTO，同时保留旧版顶层字段兼容性。
type orderDetailResponse struct {
	// orderDTO 提供历史客户端仍读取的顶层订单字段。
	orderDTO
	// Success 表示查询是否完成。
	Success bool `json:"success"`
	// Data 是新版客户端读取的订单对象。
	Data orderDTO `json:"data"`
}

// orderUpdateRequestDTO 是订单更新接口的具名请求 DTO，保留旧客户端的状态字段别名。
type orderUpdateRequestDTO struct {
	// OrderStatus 是订单状态补丁。
	OrderStatus *string `json:"order_status"`
	// Status 是旧客户端使用的订单状态别名。
	Status *string `json:"status"`
	// ItemID 是商品标识补丁。
	ItemID *string `json:"item_id"`
	// BuyerID 是买家标识补丁。
	BuyerID *string `json:"buyer_id"`
	// SpecName 是规格名称补丁。
	SpecName *string `json:"spec_name"`
	// SpecValue 是规格值补丁。
	SpecValue *string `json:"spec_value"`
	// Quantity 是兼容字符串或数字格式的购买数量。
	Quantity *any `json:"quantity"`
	// Amount 是兼容字符串或数字格式的订单金额。
	Amount *any `json:"amount"`
	// ReceiverName 是收货人补丁。
	ReceiverName *string `json:"receiver_name"`
	// ReceiverPhone 是收货电话补丁。
	ReceiverPhone *string `json:"receiver_phone"`
	// ReceiverAddress 是收货地址补丁。
	ReceiverAddress *string `json:"receiver_address"`
	// ReceiverCity 是收货城市补丁。
	ReceiverCity *string `json:"receiver_city"`
	// ChatID 是聊天会话补丁。
	ChatID *string `json:"chat_id"`
	// SystemShipped 表示是否由系统完成发货。
	SystemShipped *bool `json:"system_shipped"`
	// ItemTitle 是关联商品标题补丁。
	ItemTitle *string `json:"item_title"`
}

// manualShipRequestDTO 是批量手动发货接口的具名请求 DTO。
type manualShipRequestDTO struct {
	// OrderIDs 是待处理订单标识列表。
	OrderIDs []string `json:"order_ids"`
	// ShipMode 是发货模式。
	ShipMode string `json:"ship_mode"`
}

// qrVerificationRequestDTO 是扫码风控验证完成接口的具名请求 DTO。
type qrVerificationRequestDTO struct {
	// TargetAccountID 是可选的待重新授权本地账号标识。
	TargetAccountID string `json:"target_account_id"`
	// CaptchaMode 是扫码成功后写入账号的 Token 风控处理策略。
	CaptchaMode string `json:"captcha_mode"`
}

// orderRefreshDetailResponse 是订单刷新后返回的远端详情 DTO。
type orderRefreshDetailResponse struct {
	// Quantity 是订单购买数量。
	Quantity string `json:"quantity"`
	// SpecName 是商品规格名称。
	SpecName string `json:"spec_name"`
	// SpecValue 是商品规格值。
	SpecValue string `json:"spec_value"`
	// OrderStatus 是归一化后的订单状态。
	OrderStatus string `json:"order_status"`
	// Amount 是订单实付金额。
	Amount string `json:"amount"`
}

// orderRefreshResponse 是订单列表刷新接口的具名响应 DTO。
type orderRefreshResponse struct {
	// PartialFailure 表示批量刷新是否存在部分失败。
	PartialFailure bool `json:"partial_failure"`
	// Message 是刷新结果说明。
	Message string `json:"message"`
	// Summary 是刷新统计摘要。
	Summary orderRefreshSummary `json:"summary"`
	// Results 是逐账号或逐订单的兼容结果行。
	Results []orderRefreshResultDTO `json:"results"`
}

// orderRefreshResultDTO 是订单刷新批次中单条结果的稳定响应 DTO；可选字段只在对应阶段产生时返回。
type orderRefreshResultDTO struct {
	// Success 表示当前账号或订单刷新是否成功。
	Success bool `json:"success"`
	// CookieID 是当前结果关联的账号标识。
	CookieID string `json:"cookie_id,omitempty"`
	// Discovered 是当前账号发现的新订单数量。
	Discovered *int `json:"discovered,omitempty"`
	// Updated 是当前账号更新的订单数量。
	Updated *int `json:"updated,omitempty"`
	// SoftDeleted 表示当前账号是否标记了失效订单。
	SoftDeleted *bool `json:"soft_deleted,omitempty"`
	// ConflictSkipped 是当前账号安全跳过的跨账号归属冲突数量。
	ConflictSkipped *int `json:"conflict_skipped,omitempty"`
	// OrderID 是单订单刷新结果关联的平台订单标识。
	OrderID string `json:"order_id,omitempty"`
	// Stage 是刷新失败或完成所处的业务阶段。
	Stage string `json:"stage,omitempty"`
	// Message 是面向客户端的结果说明。
	Message string `json:"message,omitempty"`
	// Error 是当前结果的兼容错误文本。
	Error string `json:"error,omitempty"`
	// OldStatus 是刷新前的订单状态。
	OldStatus string `json:"old_status,omitempty"`
	// NewStatus 是刷新后的订单状态。
	NewStatus string `json:"new_status,omitempty"`
}

// orderRefreshSummary 是订单列表刷新统计摘要 DTO。
type orderRefreshSummary struct {
	// AccountTotal 是本次同步尝试的账号数量。
	AccountTotal int `json:"account_total"`
	// AccountSucceeded 是订单发现阶段成功的账号数量。
	AccountSucceeded int `json:"account_succeeded"`
	// AccountFailed 是订单发现阶段失败的账号数量。
	AccountFailed int `json:"account_failed"`
	// Discovered 是发现的新订单数量。
	Discovered int `json:"discovered"`
	// ListUpdated 是订单列表更新数量。
	ListUpdated int `json:"list_updated"`
	// SoftDeleted 是标记删除的订单数量。
	SoftDeleted int `json:"soft_deleted"`
	// DetailTotal 是需要补全详情的订单数量。
	DetailTotal int `json:"detail_total"`
	// Total 是本次处理订单总数。
	Total int `json:"total"`
	// Updated 是状态发生变化的订单数量。
	Updated int `json:"updated"`
	// NoChange 是状态未发生变化的订单数量。
	NoChange int `json:"no_change"`
	// Failed 是刷新失败数量。
	Failed int `json:"failed"`
	// ConflictSkipped 是安全跳过的跨账号归属冲突订单数量。
	ConflictSkipped int `json:"conflict_skipped"`
}

// orderSingleRefreshResponse 是单订单刷新接口的具名响应 DTO。
type orderSingleRefreshResponse struct {
	// Success 表示刷新是否完成。
	Success bool `json:"success"`
	// Message 是刷新结果说明。
	Message string `json:"message"`
	// Order 是刷新后的订单详情。
	Order orderRefreshDetailResponse `json:"order"`
}

// manualShipResponse 是手动发货接口的具名响应 DTO。
type manualShipResponse struct {
	// PartialFailure 表示批量发货是否存在部分失败。
	PartialFailure bool `json:"partial_failure"`
	// Message 是发货结果说明。
	Message string `json:"message"`
	// SuccessCount 是成功发货数量。
	SuccessCount int `json:"success_count"`
	// FailedCount 是失败发货数量。
	FailedCount int `json:"failed_count"`
	// Results 是逐订单的兼容结果行。
	Results []orderMutationResultDTO `json:"results"`
}

// redFlowerRequestResponse 是手动求花接口的稳定具名响应。
type redFlowerRequestResponse struct {
	// Success 表示平台明确接受了求花动作。
	Success bool `json:"success"`
	// Status 是 succeeded、succeeded_with_warning、failed 或 needs_review。
	Status string `json:"status"`
	// Message 是不包含凭证的用户提示。
	Message string `json:"message"`
	// RequestedAt 是求花动作首次开始的 Unix 秒时间戳；尚未求花时为零。
	RequestedAt int64 `json:"requested_at,omitempty"`
}

// orderMutationResultDTO 是手动发货逐订单结果的稳定响应 DTO。
type orderMutationResultDTO struct {
	// OrderID 是当前结果关联的平台订单标识。
	OrderID string `json:"order_id"`
	// Status 是发货后订单状态或失败阶段。
	Status string `json:"status"`
	// Success 表示当前订单是否发货成功。
	Success bool `json:"success"`
	// Message 是当前订单的处理说明。
	Message string `json:"message"`
	// ReconciliationID 是可选的补偿记录标识。
	ReconciliationID string `json:"reconciliation_id,omitempty"`
	// ReconciliationWarning 是可选的补偿告警文本。
	ReconciliationWarning string `json:"reconciliation_warning,omitempty"`
}

// importOrdersResponse 是订单导入接口的具名响应 DTO。
type importOrdersResponse struct {
	// PartialFailure 表示批量导入是否存在部分失败。
	PartialFailure bool `json:"partial_failure"`
	// Message 是导入结果说明。
	Message string `json:"message"`
	// Total 是本次导入订单总数。
	Total int `json:"total"`
	// SuccessCount 是成功导入数量。
	SuccessCount int `json:"success_count"`
	// FailedCount 是失败导入数量。
	FailedCount int `json:"failed_count"`
	// Results 是逐订单的兼容结果行。
	Results []orderImportResultDTO `json:"results"`
}

// orderImportResultDTO 是订单导入逐订单结果的稳定响应 DTO。
type orderImportResultDTO struct {
	// OrderID 是当前结果关联的平台订单标识。
	OrderID string `json:"order_id"`
	// Success 表示当前订单是否导入成功。
	Success bool `json:"success"`
	// Message 是当前订单的处理说明。
	Message string `json:"message"`
}
