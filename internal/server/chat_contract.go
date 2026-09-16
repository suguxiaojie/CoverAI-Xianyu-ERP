package server

import chatapp "xianyu-go/internal/application/chat"

// chatSessionDTO 是聊天会话对外暴露的非敏感 DTO，不直接复用数据库模型。
type chatSessionDTO struct {
	// AccountID 是账号稳定标识。
	AccountID string `json:"account_id"`
	// ChatID 是平台聊天会话标识。
	ChatID string `json:"chat_id"`
	// BuyerID 是买家平台标识。
	BuyerID string `json:"buyer_id"`
	// BuyerName 是买家昵称。
	BuyerName string `json:"buyer_name"`
	// BuyerAvatar 是买家头像地址。
	BuyerAvatar string `json:"buyer_avatar_url"`
	// ItemID 是会话关联商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是会话关联商品标题。
	ItemTitle string `json:"item_title"`
	// LastMessage 是最近一条消息摘要。
	LastMessage string `json:"last_message"`
	// LastMessageAt 是最近消息时间的 Unix 秒。
	LastMessageAt int64 `json:"last_message_at"`
	// UnreadCount 是当前会话未读消息数量。
	UnreadCount int `json:"unread_count"`
	// IsPinned 表示当前 ERP 用户是否已在所属账号内置顶该会话。
	IsPinned bool `json:"is_pinned"`
}

// newChatSessionDTOFromApplication 将应用层聊天会话转换为 HTTP DTO。
func newChatSessionDTOFromApplication(session chatapp.Session) chatSessionDTO {
	return chatSessionDTO{
		AccountID: session.AccountID, ChatID: session.ChatID, BuyerID: session.BuyerID,
		BuyerName: session.BuyerName, BuyerAvatar: normalizeRemoteMediaURL(session.BuyerAvatar), ItemID: session.ItemID,
		ItemTitle: session.ItemTitle, LastMessage: session.LastMessage,
		LastMessageAt: session.LastMessageAt, UnreadCount: session.UnreadCount, IsPinned: session.IsPinned,
	}
}

// chatSessionPinRequest 是幂等设置精确会话置顶偏好的 HTTP 请求 DTO。
type chatSessionPinRequest struct {
	// AccountID 是当前用户有权操作的闲鱼账号标识。
	AccountID string `json:"account_id"`
	// Pinned 使用指针区分显式 false 与缺少字段，避免非预期取消置顶。
	Pinned *bool `json:"pinned"`
}

// chatSessionPinResponse 是会话置顶持久化成功后返回的非敏感结果 DTO。
type chatSessionPinResponse struct {
	// AccountID 是已更新偏好的账号标识。
	AccountID string `json:"account_id"`
	// ChatID 是已更新的精确会话标识。
	ChatID string `json:"chat_id"`
	// Pinned 是服务端已确认保存的目标置顶状态。
	Pinned bool `json:"pinned"`
}

// newChatSessionDTOsFromApplication 批量转换应用层聊天会话，保持响应不暴露数据库模型。
func newChatSessionDTOsFromApplication(sessions []chatapp.Session) []chatSessionDTO {
	// result 是转换后的聊天会话 DTO 列表。
	result := make([]chatSessionDTO, 0, len(sessions))
	// session 表示当前待转换的应用层会话。
	for _, session := range sessions {
		result = append(result, newChatSessionDTOFromApplication(session))
	}
	return result
}

// chatMessageDTO 是聊天消息对外暴露的具名 DTO，不直接复用数据库模型。
type chatMessageDTO struct {
	// ID 是本地消息主键。
	ID int64 `json:"id"`
	// AccountID 是账号稳定标识。
	AccountID string `json:"account_id"`
	// ChatID 是平台聊天会话标识。
	ChatID string `json:"chat_id"`
	// MessageKey 是消息幂等键。
	MessageKey string `json:"message_key"`
	// PlatformMessageID 是闲鱼撤回和回执使用的 PNM ID。
	PlatformMessageID string `json:"platform_message_id,omitempty"`
	// ReplyToPlatformMessageID 是当前消息原生引用的目标 PNM ID。
	ReplyToPlatformMessageID string `json:"reply_to_platform_message_id,omitempty"`
	// ReplyPreview 是同会话目标消息的最小引用展示快照。
	ReplyPreview *chatReplyPreviewDTO `json:"reply_preview,omitempty"`
	// Direction 是消息方向。
	Direction string `json:"direction"`
	// SenderID 是消息发送者平台标识。
	SenderID string `json:"sender_id"`
	// SenderName 是消息发送者名称。
	SenderName string `json:"sender_name"`
	// MessageType 是消息类型。
	MessageType string `json:"message_type"`
	// Content 是消息文本或媒体地址。
	Content string `json:"content"`
	// Status 是消息投递状态。
	Status string `json:"status"`
	// ReadStatus 是平台已读回执状态；值为 2 时表示对方已读。
	ReadStatus int `json:"read_status"`
	// ReadAt 是平台确认已读的 Unix 毫秒时间戳；零值表示尚未确认。
	ReadAt int64 `json:"read_at"`
	// SentAt 是消息发送时间的 Unix 秒。
	SentAt int64 `json:"sent_at"`
	// RecalledAt 是平台确认撤回的 Unix 毫秒时间。
	RecalledAt int64 `json:"recalled_at,omitempty"`
	// RecallOperatorType 区分发送者、群主、系统和安全撤回。
	RecallOperatorType int `json:"recall_operator_type"`
	// RecallOperatorID 是平台撤回操作者标识。
	RecallOperatorID string `json:"recall_operator_id,omitempty"`
	// SystemCard 是结构化 contentType=26 交易卡片；普通系统通知省略。
	SystemCard *chatSystemCardDTO `json:"system_card,omitempty"`
	// LocationCard 是结构化 contentType=30 位置卡片；其他消息省略。
	LocationCard *chatLocationCardDTO `json:"location_card,omitempty"`
}

// chatLocationCardDTO 是聊天页面展示位置标题、说明和坐标的非敏感传输模型。
type chatLocationCardDTO struct {
	// Title 是发送者确认的位置名称。
	Title string `json:"title"`
	// Description 是发送者确认的地址或到店说明。
	Description string `json:"description"`
	// Latitude 是 WGS84 纬度十进制度。
	Latitude float64 `json:"latitude"`
	// Longitude 是 WGS84 经度十进制度。
	Longitude float64 `json:"longitude"`
}

// chatReplyPreviewDTO 是消息气泡区分原生引用时使用的具名非敏感 DTO。
type chatReplyPreviewDTO struct {
	// PlatformMessageID 是被引用消息的 PNM ID。
	PlatformMessageID string `json:"platform_message_id"`
	// Direction 是被引用消息相对当前账号的方向。
	Direction string `json:"direction"`
	// SenderID 是被引用消息发送者的平台标识。
	SenderID string `json:"sender_id"`
	// SenderName 是被引用消息发送者的展示名称。
	SenderName string `json:"sender_name"`
	// MessageType 是被引用消息的文本、图片或视频类型。
	MessageType string `json:"message_type"`
	// Content 是被引用消息的原文或媒体地址。
	Content string `json:"content"`
	// Status 是被引用消息的当前状态。
	Status string `json:"status"`
}

// chatSystemCardDTO 是 HTTP 和实时事件共用的只读交易卡片契约。
type chatSystemCardDTO struct {
	// Kind 当前固定为 trade。
	Kind string `json:"kind"`
	// Event 是归一化后的交易状态。
	Event string `json:"event"`
	// Title 是平台交易卡片标题。
	Title string `json:"title"`
	// Description 是平台交易卡片说明。
	Description string `json:"description,omitempty"`
	// OrderID 是明确解析出的平台订单标识。
	OrderID string `json:"order_id,omitempty"`
	// ItemID 是明确解析出的平台商品标识。
	ItemID string `json:"item_id,omitempty"`
	// Action 是白名单动作提示；HTTP 层不执行平台写入。
	Action string `json:"action,omitempty"`
}

// newChatMessageDTOFromApplication 将聊天应用层发送结果转换为 HTTP DTO，避免响应引用数据库模型。
func newChatMessageDTOFromApplication(message *chatapp.Message) chatMessageDTO {
	if message == nil {
		return chatMessageDTO{}
	}
	// result 保存应用消息转换后的 HTTP DTO。
	result := chatMessageDTO{
		ID: message.ID, AccountID: message.AccountID, ChatID: message.ChatID,
		MessageKey: message.MessageKey, PlatformMessageID: message.PlatformMessageID, ReplyToPlatformMessageID: message.ReplyToPlatformMessageID, Direction: message.Direction,
		SenderID: message.SenderID, SenderName: message.SenderName,
		MessageType: message.MessageType, Content: normalizeChatMediaContent(message.MessageType, message.Content),
		Status: message.Status, ReadStatus: message.ReadStatus, ReadAt: message.ReadAt, SentAt: message.SentAt,
		RecalledAt: message.RecalledAt, RecallOperatorType: message.RecallOperatorType, RecallOperatorID: message.RecallOperatorID,
	}
	if message.ReplyPreview != nil {
		result.ReplyPreview = &chatReplyPreviewDTO{PlatformMessageID: message.ReplyPreview.PlatformMessageID,
			Direction: message.ReplyPreview.Direction, SenderID: message.ReplyPreview.SenderID, SenderName: message.ReplyPreview.SenderName,
			MessageType: message.ReplyPreview.MessageType, Content: normalizeChatMediaContent(message.ReplyPreview.MessageType, message.ReplyPreview.Content), Status: message.ReplyPreview.Status}
	}
	if message.SystemCard != nil {
		result.SystemCard = &chatSystemCardDTO{
			Kind: message.SystemCard.Kind, Event: message.SystemCard.Event, Title: message.SystemCard.Title,
			Description: message.SystemCard.Description, OrderID: message.SystemCard.OrderID,
			ItemID: message.SystemCard.ItemID, Action: message.SystemCard.Action,
		}
	}
	if message.LocationCard != nil {
		result.LocationCard = &chatLocationCardDTO{
			Title: message.LocationCard.Title, Description: message.LocationCard.Description,
			Latitude: message.LocationCard.Latitude, Longitude: message.LocationCard.Longitude,
		}
	}
	return result
}

// newChatMessageDTOsFromApplication 将聊天应用层消息转换为 HTTP DTO，避免响应暴露数据库模型。
func newChatMessageDTOsFromApplication(messages []chatapp.Message) []chatMessageDTO {
	// result 是转换后的聊天消息 DTO 列表。
	result := make([]chatMessageDTO, 0, len(messages))
	// message 保存当前待转换的应用层消息。
	for _, message := range messages {
		result = append(result, newChatMessageDTOFromApplication(&message))
	}
	return result
}

// chatEventDTO 是聊天实时推送的具名传输契约，确保 WebSocket 与 HTTP 使用相同的 snake_case 消息字段。
type chatEventDTO struct {
	// Type 是实时事件类别，例如 message.created。
	Type string `json:"type"`
	// Message 是本次事件关联的非敏感聊天消息；非消息事件可以为空。
	Message *chatMessageDTO `json:"message,omitempty"`
	// Session 是本次事件关联的非敏感会话摘要；无需更新会话时可以为空。
	Session *chatSessionDTO `json:"session,omitempty"`
}

// newChatEventDTOFromApplication 将应用层实时事件转换为浏览器稳定使用的 WebSocket DTO。
func newChatEventDTOFromApplication(event chatapp.Event) chatEventDTO {
	// result 保存转换后的实时事件；指针字段只在应用事件提供对应实体时设置。
	result := chatEventDTO{Type: event.Type}
	if event.Message != nil {
		// message 保存避免 WebSocket 直接序列化应用层 PascalCase 字段的消息 DTO。
		message := newChatMessageDTOFromApplication(event.Message)
		result.Message = &message
	}
	if event.Session != nil {
		// session 保存与 HTTP 会话接口一致的 snake_case 会话 DTO。
		session := newChatSessionDTOFromApplication(*event.Session)
		result.Session = &session
	}
	return result
}

// chatSessionPageResponse 是聊天会话分页接口的具名响应 DTO。
type chatSessionPageResponse struct {
	// Sessions 是当前页聊天会话。
	Sessions []chatSessionDTO `json:"sessions"`
	// HasMore 表示是否还有下一页。
	HasMore bool `json:"has_more"`
	// NextCursor 是下一页游标。
	NextCursor int64 `json:"next_cursor,omitempty"`
}

// chatMessageEnvelope 是发送聊天消息接口的具名响应 DTO。
type chatMessageEnvelope struct {
	// Message 是已经写入本地队列的消息。
	Message chatMessageDTO `json:"message"`
}

// chatMessagePageResponse 是聊天消息分页接口的具名响应 DTO。
type chatMessagePageResponse struct {
	// Messages 是当前页聊天消息。
	Messages []chatMessageDTO `json:"messages"`
	// HasMore 表示是否还有更多历史消息。
	HasMore bool `json:"has_more"`
	// NextCursor 是下一页游标。
	NextCursor int64 `json:"next_cursor,omitempty"`
	// Session 是当前聊天会话摘要。
	Session chatSessionDTO `json:"session"`
}
