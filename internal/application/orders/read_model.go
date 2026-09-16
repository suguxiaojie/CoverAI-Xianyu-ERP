// Package orders 定义订单用例面向消费者的纯业务查询模型和 Port。
// 本包不得依赖数据库、HTTP、平台协议或 Server 实现。
package orders

import (
	"context"
	"time"
)

// OrderRow 是订单列表用例需要的纯业务展示行。
type OrderRow struct {
	// OrderID 是订单稳定标识。
	OrderID string
	// ItemID 是关联商品标识。
	ItemID string
	// ItemTitle 是关联商品标题。
	ItemTitle string
	// ItemDetail 是关联商品详情 JSON。
	ItemDetail string
	// BuyerID 是买家标识。
	BuyerID string
	// ChatID 是订单明确关联的聊天会话标识；为空表示只能按买家历史关系回退。
	ChatID string
	// BuyerName 是从同账号聊天会话解析出的闲鱼买家昵称。
	BuyerName string
	// BuyerAvatar 是买家最近聊天会话提供的头像地址。
	BuyerAvatar string
	// SpecName 是规格名称。
	SpecName string
	// SpecValue 是规格值。
	SpecValue string
	// Quantity 是购买数量。
	Quantity string
	// Amount 是订单金额文本。
	Amount string
	// ShipmentProofAvailable 表示订单存在 ERP 成功发货后保存的凭证。
	ShipmentProofAvailable bool
	// OrderStatus 是持久化的订单状态。
	OrderStatus string
	// CookieID 是订单所属账号标识。
	CookieID string
	// IsBargain 表示订单是否为砍价订单。
	IsBargain int
	// SystemShipped 表示是否由系统确认发货。
	SystemShipped bool
	// ReceiverName 是收货人姓名。
	ReceiverName string
	// ReceiverPhone 是收货人电话。
	ReceiverPhone string
	// ReceiverAddr 是收货地址。
	ReceiverAddr string
	// ReceiverCity 是收货城市。
	ReceiverCity string
	// CreatedAt 是订单创建时间。
	CreatedAt string
	// UpdatedAt 是订单更新时间。
	UpdatedAt string
	// PaidAt 是平台明确付款的时间；没有该事件时为空。
	PaidAt string
	// ShippedAt 是卖家明确发货的时间；没有该事件时为空。
	ShippedAt string
	// ReceivedAt 是买家明确确认收货的时间；没有该事件时为空。
	ReceivedAt string
	// CompletedAt 是平台明确交易完成的时间；没有该终态时为空。
	CompletedAt string
	// RefundedAt 是平台明确退款成功的时间；没有该终态时为空。
	RefundedAt string
	// CancelledAt 是未付款订单或普通交易明确取消的时间。
	CancelledAt string
	// RefundRequested 表示聊天中存在与该订单精确关联的退款申请证据。
	RefundRequested bool
}

// Order 是订单详情和发货用例使用的纯业务实体。
type Order struct {
	// OrderID 是订单稳定标识。
	OrderID string
	// ItemID 是关联商品标识。
	ItemID string
	// BuyerID 是买家标识。
	BuyerID string
	// SpecName 是规格名称。
	SpecName string
	// SpecValue 是规格值。
	SpecValue string
	// Quantity 是购买数量。
	Quantity string
	// Amount 是订单金额文本。
	Amount string
	// OrderStatus 是持久化的订单状态。
	OrderStatus string
	// CookieID 是订单所属账号标识。
	CookieID string
	// IsBargain 表示订单是否为砍价订单。
	IsBargain int
	// ReceiverName 是收货人姓名。
	ReceiverName string
	// ReceiverPhone 是收货人电话。
	ReceiverPhone string
	// ReceiverAddress 是收货地址。
	ReceiverAddress string
	// ReceiverCity 是收货城市。
	ReceiverCity string
	// Version 是订单乐观锁版本。
	Version int
	// ChatID 是聊天会话标识。
	ChatID string
	// SystemShipped 表示是否由系统确认发货。
	SystemShipped bool
	// PaidAt 是订单付款时间文本。
	PaidAt string
	// ShippedAt 是订单发货时间文本。
	ShippedAt string
	// CompletedAt 是订单完成时间文本。
	CompletedAt string
	// ReceivedAt 是买家明确确认收货的时间文本。
	ReceivedAt string
	// RefundedAt 是平台明确退款成功的时间文本。
	RefundedAt string
	// CancelledAt 是普通取消订单的时间文本。
	CancelledAt string
	// RefundRequested 表示聊天中存在与该订单精确关联的退款申请证据。
	RefundRequested bool
	// BuyerReviewedAt 是买家评价时间文本。
	BuyerReviewedAt string
	// LastReviewRequestAt 是最近一次索评时间文本。
	LastReviewRequestAt string
	// ReviewRequestCount 是索评次数。
	ReviewRequestCount int
	// CreatedAt 是订单创建时间。
	CreatedAt string
	// UpdatedAt 是订单更新时间。
	UpdatedAt string
}

// ItemInfo 是订单详情需要的纯商品信息。
type ItemInfo struct {
	// ID 是商品信息记录的本地标识。
	ID int64
	// CookieID 是商品所属账号标识。
	CookieID string
	// ItemID 是平台商品标识。
	ItemID string
	// ItemTitle 是商品标题。
	ItemTitle string
	// ItemDescription 是商品描述。
	ItemDescription string
	// ItemCategory 是商品分类。
	ItemCategory string
	// ItemPrice 是商品价格文本。
	ItemPrice string
	// ItemDetail 是商品详情 JSON。
	ItemDetail string
	// IsMultiSpec 表示商品是否启用多规格。
	IsMultiSpec bool
	// MultiQuantityDelivery 表示商品是否启用多数量发货。
	MultiQuantityDelivery bool
}

// PlatformRuntimeData 是订单刷新访问平台所需的最小账号运行视图。
type PlatformRuntimeData struct {
	// ID 是闲鱼账号的稳定标识。
	ID string
	// UserID 是账号所属的本地用户标识。
	UserID int64
	// Value 是 repository 解密后的 Cookie 明文，仅在平台请求边界短暂使用。
	Value string
	// MetadataJSON 是 Cookie 快照等平台请求元数据。
	MetadataJSON string
	// ShowBrowser 表示风控恢复必须使用可见浏览器并等待人工验证。
	ShowBrowser bool
}

// ListFilter 是订单列表查询的纯业务筛选条件。
type ListFilter struct {
	// UserID 是当前用户标识。
	UserID int64
	// CookieID 是可选的账号筛选条件。
	CookieID string
	// Status 是可选的订单状态筛选条件。
	Status string
	// Search 是订单号、商品或买家搜索词。
	Search string
	// ContextBuyerID 是 Chat 历史订单使用的精确买家标识；必须与当前账号共同限定。
	ContextBuyerID string
	// ContextChatID 是 Chat 历史订单使用的精确会话标识。
	ContextChatID string
	// MatchConversationContext 表示按会话或买家任一关系匹配，而不是把两个条件作为普通 AND 筛选。
	MatchConversationContext bool
	// PrioritizeChatID 让当前会话明确关联的订单排在同买家历史订单之前。
	PrioritizeChatID string
	// CreatedFrom 是订单创建时间的可选包含下界。
	CreatedFrom time.Time
	// CreatedTo 是订单创建时间的可选排除上界。
	CreatedTo time.Time
	// MinAmountCents 是实付金额的可选包含下界，单位为人民币分。
	MinAmountCents *int64
	// MaxAmountCents 是实付金额的可选包含上界，单位为人民币分。
	MaxAmountCents *int64
	// Limit 是返回条数上限。
	Limit int
	// Offset 是分页偏移量。
	Offset int
}

// Reader 定义订单列表用例需要的只读 Port。
type Reader interface {
	// ListForUser 返回当前用户可见的订单展示行和总数。
	ListForUser(ctx context.Context, filter ListFilter) ([]OrderRow, int, error)
}
