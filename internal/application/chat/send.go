package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
)

var (
	// ErrUnavailable 表示聊天发送所需的持久化、运行时或图片上传端口未装配。
	ErrUnavailable = errors.New("聊天发送服务未启用")
	// ErrOffline 表示目标账号没有可用的在线发送实例。
	ErrOffline = errors.New("账号当前离线")
	// ErrSend 表示平台发送动作失败，消息状态已尽力标记为失败。
	ErrSend = errors.New("聊天消息发送失败")
	// ErrStatusSave 表示平台动作已成功，但本地发送状态没有保存成功。
	ErrStatusSave = errors.New("聊天发送状态保存失败")
	// ErrSendInvalidInput 表示发送用例缺少会话标识或消息内容不符合限制。
	ErrSendInvalidInput = errors.New("聊天发送参数无效")
	// ErrReplyNotFound 表示被回复消息不存在或不属于当前认证用户。
	ErrReplyNotFound = errors.New("被回复消息不存在")
	// ErrReplyNotAllowed 表示被回复消息不在当前会话、是系统消息或缺少 PNM ID。
	ErrReplyNotAllowed = errors.New("当前消息不可引用回复")
)

// OutgoingInput 是发送文字消息的应用层输入，不携带 HTTP 或数据库模型。
type OutgoingInput struct {
	// Session 保存已完成账号归属校验的会话摘要，不包含登录凭证。
	Session Session
	// Text 保存待发送的文字内容，应用层会去除首尾空白并限制长度。
	Text string
	// UserID 是当前 ERP 认证用户，仅在引用目标归属查询时使用。
	UserID int64
	// ReplyToMessageKey 是前端持有的本地目标消息键；空值表示普通文本。
	ReplyToMessageKey string
}

// ImageInput 是发送图片消息的应用层输入，Data 只在当前请求生命周期内使用。
type ImageInput struct {
	// Session 保存已完成账号归属校验的会话摘要，不包含登录凭证。
	Session Session
	// Filename 保存上传文件名，供平台接口识别图片来源。
	Filename string
	// ContentType 保存 HTTP 已校验的图片 MIME 类型。
	ContentType string
	// Data 保存图片二进制内容，调用完成后不由服务长期持有。
	Data []byte
}

// LocationInput 是一次人工位置卡片发送输入；标题、说明和坐标均由当前用户在发送前确认。
type LocationInput struct {
	// Session 保存已完成账号归属校验的会话摘要，不包含登录凭证。
	Session Session
	// Title 是位置卡片主标题，例如实体店名称。
	Title string
	// Description 是位置卡片副标题，例如门牌和到店指引。
	Description string
	// Latitude 是 WGS84 纬度十进制度，范围为 -90 到 90。
	Latitude float64
	// Longitude 是 WGS84 经度十进制度，范围为 -180 到 180。
	Longitude float64
}

// LocationCard 是聊天历史和 HTTP 响应允许公开的结构化位置卡片，不包含平台凭证或任意跳转地址。
type LocationCard struct {
	// Title 是发送者确认的位置名称。
	Title string `json:"title"`
	// Description 是发送者确认的地址或到店说明。
	Description string `json:"description"`
	// Latitude 是位置纬度十进制度。
	Latitude float64 `json:"latitude"`
	// Longitude 是位置经度十进制度。
	Longitude float64 `json:"longitude"`
}

// ImageUpload 是图片平台适配器返回的非敏感结果。
type ImageUpload struct {
	// URL 保存可供聊天发送的图片地址。
	URL string
	// Width 保存平台识别出的图片宽度，单位为像素；非正值表示使用协议默认尺寸。
	Width int
	// Height 保存平台识别出的图片高度，单位为像素；非正值表示使用协议默认尺寸。
	Height int
}

// PlatformSendResult 是运行时返回的闲鱼平台消息标识。
type PlatformSendResult struct {
	// PlatformMessageID 是后续撤回和历史对账使用的 PNM ID。
	PlatformMessageID string
	// CreatedAt 是平台创建时间，Unix 毫秒；缺失时为零。
	CreatedAt int64
}

// OutgoingRepository 定义发送用例需要的本地消息写入能力。
type OutgoingRepository interface {
	// CreateOutgoing 创建状态为 sending 的文字消息，可选保存经校验的原生引用 PNM ID。
	CreateOutgoing(ctx context.Context, session Session, text, replyToPlatformMessageID string) (Message, error)
	// CreateOutgoingMedia 创建状态为 sending 的媒体消息并返回幂等键。
	CreateOutgoingMedia(ctx context.Context, session Session, messageType, content string) (Message, error)
	// SetOutgoingStatus 更新外发消息状态并返回最新消息。
	SetOutgoingStatus(ctx context.Context, accountID, key, status string) (Message, error)
	// BindPlatformMessageID 把平台发送结果绑定到既有本地消息。
	BindPlatformMessageID(ctx context.Context, accountID, key, platformMessageID string) (Message, error)
	// MarkMessageRecalled 按平台消息 ID 收敛主动或被动撤回状态。
	MarkMessageRecalled(ctx context.Context, accountID, platformMessageID string, operatorType int, operatorID string, recalledAt int64) (Message, error)
}

// Sender 定义单个在线账号的最小聊天发送能力。
type Sender interface {
	// SendText 发送文本；messageKey 用于将平台旁路事件与待发送消息关联。
	SendText(ctx context.Context, chatID, toUserID, text, messageKey string) (PlatformSendResult, error)
	// SendReplyText 发送原生引用文本；replyToPlatformMessageID 是已经应用层校验的同会话 PNM ID。
	SendReplyText(ctx context.Context, chatID, toUserID, text, replyToPlatformMessageID, messageKey string) (PlatformSendResult, error)
	// SendImage 发送图片；messageKey 用于将平台旁路事件与待发送消息关联。
	SendImage(ctx context.Context, chatID, toUserID, imageURL string, cardID int64, width, height int, messageKey string) (PlatformSendResult, error)
	// SendLocationCard 发送自定义标题、说明和真实坐标组成的位置卡片。
	SendLocationCard(ctx context.Context, chatID, toUserID, title, description string, latitude, longitude float64, messageKey string) (PlatformSendResult, error)
	// RecallMessage 使用当前账号在线连接撤回已发送平台消息。
	RecallMessage(ctx context.Context, platformMessageID string, contentType int, text string) error
}

// SenderProvider 按账号标识解析当前在线发送实例。
type SenderProvider interface {
	// Sender 返回指定账号的发送能力；不存在时返回 false。
	Sender(accountID string) (Sender, bool)
}

// ImageUploader 定义图片上传所需的平台能力。
type ImageUploader interface {
	// UploadChatImage 按账号标识上传图片；凭证读取与刷新由平台适配器内部完成。
	UploadChatImage(ctx context.Context, accountID, filename, contentType string, data []byte) (ImageUpload, error)
}

// NewWithSending 创建同时支持历史查询和实时发送的聊天应用服务。
func NewWithSending(repository Repository, outgoing OutgoingRepository, senders SenderProvider, uploader ImageUploader, identity ...IdentityResolver) *Service {
	// service 保存聊天历史、发送和平台身份能力的统一应用服务。
	service := &Service{
		repository: repository,
		outgoing:   outgoing,
		senders:    senders,
		uploader:   uploader,
	}
	if len(identity) > 0 {
		service.identityResolver = identity[0]
	}
	return service
}

// NewWithSendingAndSubscription 创建同时支持发送和实时订阅的聊天应用服务。
func NewWithSendingAndSubscription(repository Repository, outgoing OutgoingRepository, senders SenderProvider, uploader ImageUploader, subscription SubscriptionProvider, identity ...IdentityResolver) *Service {
	// service 保存聊天历史、发送和实时订阅能力的统一应用服务。
	service := NewWithSending(repository, outgoing, senders, uploader, identity...)
	service.subscription = subscription
	return service
}

// NewWithSendingSubscriptionAndRefresh 创建同时支持发送、订阅和平台刷新的聊天应用服务。
func NewWithSendingSubscriptionAndRefresh(repository Repository, outgoing OutgoingRepository, senders SenderProvider, uploader ImageUploader, subscription SubscriptionProvider, refresh RefreshProvider, identity ...IdentityResolver) *Service {
	// service 保存聊天用例所需的持久化、平台刷新、发送和订阅端口。
	service := NewWithSendingAndSubscription(repository, outgoing, senders, uploader, subscription, identity...)
	service.refresh = refresh
	return service
}

// SendingAvailable 报告文字/媒体消息所需的应用端口是否已完成装配。
// 该查询只反映依赖生命周期，不触碰账号凭证或外部平台。
func (s *Service) SendingAvailable() bool {
	return s != nil && s.outgoing != nil && s.senders != nil
}

// ImageUploadAvailable 报告图片上传所需的应用端口是否已完成装配。
func (s *Service) ImageUploadAvailable() bool {
	return s != nil && s.uploader != nil
}

// Subscribe 订阅当前用户有权接收的实时聊天事件；取消函数可安全重复调用。
func (s *Service) Subscribe(ctx context.Context, userID int64) (<-chan Event, func(), error) {
	if s == nil || s.subscription == nil || userID <= 0 {
		return nil, nil, ErrSubscriptionUnavailable
	}
	// events、cancel、err 保存订阅事件流、幂等清理函数和底层错误。
	events, cancel, err := s.subscription.Subscribe(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	// once 保证应用层向 HTTP 暴露的清理函数可安全重复调用。
	var once sync.Once
	return events, func() { once.Do(cancel) }, nil
}

// RefreshConversations 刷新并保存指定账号的联系人页；原始平台数据不会离开应用端口。
func (s *Service) RefreshConversations(ctx context.Context, accountID string, cursor int64, limit int) (ConversationPage, error) {
	accountID = strings.TrimSpace(accountID)
	if s == nil || s.refresh == nil || accountID == "" || limit <= 0 {
		return ConversationPage{}, ErrRefreshUnavailable
	}
	return s.refresh.RefreshConversations(ctx, accountID, cursor, limit)
}

// RefreshHistory 刷新并保存指定会话的消息页；session 只包含非敏感展示字段。
func (s *Service) RefreshHistory(ctx context.Context, accountID, chatID string, cursor int64, limit int, session Session) (HistoryPage, error) {
	accountID = strings.TrimSpace(accountID)
	chatID = strings.TrimSpace(chatID)
	if s == nil || s.refresh == nil || accountID == "" || chatID == "" || limit <= 0 {
		return HistoryPage{}, ErrRefreshUnavailable
	}
	return s.refresh.RefreshHistory(ctx, accountID, chatID, cursor, limit, session)
}

// SendText 创建并发送一条文字消息，失败时尽力保留本地 failed 状态。
func (s *Service) SendText(ctx context.Context, input OutgoingInput) (*Message, error) {
	// session 和 text 保存规范化后的会话及消息内容。
	session, text, err := normalizeOutgoingInput(input.Session, input.Text)
	if err != nil {
		return nil, err
	}
	if s == nil || s.outgoing == nil || s.senders == nil {
		return nil, ErrUnavailable
	}
	// replyToPlatformMessageID 是通过当前用户、账号和会话三重校验后的原生引用目标。
	replyToPlatformMessageID := ""
	if input.ReplyToMessageKey = strings.TrimSpace(input.ReplyToMessageKey); input.ReplyToMessageKey != "" {
		if input.UserID <= 0 || s.repository == nil {
			return nil, ErrReplyNotFound
		}
		// replyTarget 是当前 ERP 用户确实拥有的本地引用候选。
		replyTarget, replyErr := s.repository.FindMessage(ctx, input.UserID, session.AccountID, input.ReplyToMessageKey)
		if replyErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrReplyNotFound, replyErr)
		}
		replyToPlatformMessageID = strings.TrimSpace(replyTarget.PlatformMessageID)
		if replyTarget.ChatID != session.ChatID || replyTarget.MessageType == "system" || !strings.HasSuffix(replyToPlatformMessageID, ".PNM") {
			return &replyTarget, ErrReplyNotAllowed
		}
	}
	// sender 和 ok 保存目标账号的在线发送句柄及存在性。
	sender, ok := s.senders.Sender(session.AccountID)
	if !ok || sender == nil {
		return nil, ErrOffline
	}
	// message 和 err 保存本地待发送消息及持久化错误。
	message, err := s.outgoing.CreateOutgoing(ctx, session, text, replyToPlatformMessageID)
	if err != nil {
		return nil, fmt.Errorf("保存待发送消息失败: %w", err)
	}
	// sendErr 表示平台文字发送失败；失败分支会补写本地 failed 状态。
	// platformResult、sendErr 保存平台 PNM ID 和明确发送错误。
	// platformResult、sendErr 是普通文本或原生引用文本的平台结果与明确错误。
	var platformResult PlatformSendResult
	// sendErr 是平台普通或原生引用文本发送的明确错误。
	var sendErr error
	if replyToPlatformMessageID != "" {
		platformResult, sendErr = sender.SendReplyText(ctx, session.ChatID, session.BuyerID, text, replyToPlatformMessageID, message.MessageKey)
	} else {
		platformResult, sendErr = sender.SendText(ctx, session.ChatID, session.BuyerID, text, message.MessageKey)
	}
	if sendErr != nil {
		// failed 保存平台发送失败后的本地状态；状态保存失败不覆盖原始发送错误。
		failed, _ := s.outgoing.SetOutgoingStatus(context.Background(), session.AccountID, message.MessageKey, "failed")
		return messagePointer(failed, message), fmt.Errorf("%w: %v", ErrSend, sendErr)
	}
	if strings.TrimSpace(platformResult.PlatformMessageID) == "" {
		return messagePointer(Message{}, message), fmt.Errorf("%w: 平台未返回消息 ID", ErrStatusSave)
	}
	// bound、err 保存 PNM ID 绑定后的本地消息和持久化错误。
	bound, err := s.outgoing.BindPlatformMessageID(ctx, session.AccountID, message.MessageKey, platformResult.PlatformMessageID)
	if err != nil {
		return messagePointer(bound, message), fmt.Errorf("%w: %v", ErrStatusSave, err)
	}
	// sent 和 err 保存平台发送成功后的本地状态及状态持久化错误。
	sent, err := s.outgoing.SetOutgoingStatus(ctx, session.AccountID, message.MessageKey, "sent")
	if err != nil {
		return messagePointer(sent, message), fmt.Errorf("%w: %v", ErrStatusSave, err)
	}
	return messagePointer(sent, message), nil
}

// SendImage 上传并发送一条图片消息，失败时尽力保留本地 failed 状态。
func (s *Service) SendImage(ctx context.Context, input ImageInput) (*Message, error) {
	// session 保存规范化后的会话摘要。
	session, _, err := normalizeOutgoingInput(input.Session, "图片")
	if err != nil {
		return nil, err
	}
	if s == nil || s.outgoing == nil || s.senders == nil || s.uploader == nil {
		return nil, ErrUnavailable
	}
	if len(input.Data) == 0 {
		return nil, ErrSendInvalidInput
	}
	// sender 和 ok 保存目标账号的在线发送句柄及存在性。
	sender, ok := s.senders.Sender(session.AccountID)
	if !ok || sender == nil {
		return nil, ErrOffline
	}
	// upload 和 err 保存图片上传结果及平台错误。
	upload, err := s.uploader.UploadChatImage(ctx, session.AccountID, input.Filename, input.ContentType, input.Data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSend, err)
	}
	if strings.TrimSpace(upload.URL) == "" {
		return nil, fmt.Errorf("%w: 图片上传未返回地址", ErrSend)
	}
	// message 和 err 保存图片待发送消息及本地写入错误。
	message, err := s.outgoing.CreateOutgoingMedia(ctx, session, "image", upload.URL)
	if err != nil {
		return nil, fmt.Errorf("保存待发送图片失败: %w", err)
	}
	// sendErr 表示平台图片发送失败；失败分支会补写本地 failed 状态。
	// platformResult、sendErr 保存平台图片 PNM ID 和明确发送错误。
	platformResult, sendErr := sender.SendImage(ctx, session.ChatID, session.BuyerID, upload.URL, 0, upload.Width, upload.Height, message.MessageKey)
	if sendErr != nil {
		// failed 保存图片发送失败后的本地状态。
		failed, _ := s.outgoing.SetOutgoingStatus(context.Background(), session.AccountID, message.MessageKey, "failed")
		return messagePointer(failed, message), fmt.Errorf("%w: %v", ErrSend, sendErr)
	}
	if strings.TrimSpace(platformResult.PlatformMessageID) == "" {
		return messagePointer(Message{}, message), fmt.Errorf("%w: 平台未返回消息 ID", ErrStatusSave)
	}
	// bound、err 保存图片 PNM ID 绑定后的本地消息和持久化错误。
	bound, err := s.outgoing.BindPlatformMessageID(ctx, session.AccountID, message.MessageKey, platformResult.PlatformMessageID)
	if err != nil {
		return messagePointer(bound, message), fmt.Errorf("%w: %v", ErrStatusSave, err)
	}
	// sent 和 err 保存图片发送成功后的本地状态及状态持久化错误。
	sent, err := s.outgoing.SetOutgoingStatus(ctx, session.AccountID, message.MessageKey, "sent")
	if err != nil {
		return messagePointer(sent, message), fmt.Errorf("%w: %v", ErrStatusSave, err)
	}
	return messagePointer(sent, message), nil
}

// SendLocationCard 创建本地待发送卡片并复用当前账号在线连接完成一次平台投递。
func (s *Service) SendLocationCard(ctx context.Context, input LocationInput) (*Message, error) {
	// session 保存完成账号、会话和买家校验后的非敏感会话摘要。
	session, _, sessionErr := normalizeOutgoingInput(input.Session, "位置卡片")
	if sessionErr != nil {
		return nil, sessionErr
	}
	// card、cardErr 是规范化后的结构化位置卡片及字段校验错误。
	card, cardErr := normalizeLocationCard(input)
	if cardErr != nil {
		return nil, cardErr
	}
	if s == nil || s.outgoing == nil || s.senders == nil {
		return nil, ErrUnavailable
	}
	// sender、online 保存当前账号发送能力及在线状态。
	sender, online := s.senders.Sender(session.AccountID)
	if !online || sender == nil {
		return nil, ErrOffline
	}
	// content、encodeErr 是用于本地幂等匹配和前端结构化展示的最小 JSON，不包含跳转地址。
	content, encodeErr := encodeLocationCardContent(card)
	if encodeErr != nil {
		return nil, fmt.Errorf("编码位置卡片失败: %w", encodeErr)
	}
	// message、createErr 是平台写入前创建的 sending 消息及持久化错误。
	message, createErr := s.outgoing.CreateOutgoingMedia(ctx, session, "location", content)
	if createErr != nil {
		return nil, fmt.Errorf("保存待发送位置卡片失败: %w", createErr)
	}
	// platformResult、sendErr 是平台返回的 PNM ID、创建时间及明确发送错误。
	platformResult, sendErr := sender.SendLocationCard(ctx, session.ChatID, session.BuyerID, card.Title, card.Description, card.Latitude, card.Longitude, message.MessageKey)
	if sendErr != nil {
		// failed 保存明确发送失败后的本地状态；状态补写失败不覆盖平台错误。
		failed, _ := s.outgoing.SetOutgoingStatus(context.Background(), session.AccountID, message.MessageKey, "failed")
		return messagePointer(failed, message), fmt.Errorf("%w: %v", ErrSend, sendErr)
	}
	if strings.TrimSpace(platformResult.PlatformMessageID) == "" {
		return messagePointer(Message{}, message), fmt.Errorf("%w: 平台未返回消息 ID", ErrStatusSave)
	}
	// bound、bindErr 是绑定平台 PNM ID 后的最新本地消息及持久化错误。
	bound, bindErr := s.outgoing.BindPlatformMessageID(ctx, session.AccountID, message.MessageKey, platformResult.PlatformMessageID)
	if bindErr != nil {
		return messagePointer(bound, message), fmt.Errorf("%w: %v", ErrStatusSave, bindErr)
	}
	// sent、statusErr 是投递成功后的最终本地消息及状态写入错误。
	sent, statusErr := s.outgoing.SetOutgoingStatus(ctx, session.AccountID, message.MessageKey, "sent")
	if statusErr != nil {
		return messagePointer(sent, message), fmt.Errorf("%w: %v", ErrStatusSave, statusErr)
	}
	return messagePointer(sent, message), nil
}

// normalizeLocationCard 校验用户输入并返回可安全发送和展示的位置卡片。
func normalizeLocationCard(input LocationInput) (LocationCard, error) {
	// title、description 是去除首尾空白后的用户可见字段。
	title, description := strings.TrimSpace(input.Title), strings.TrimSpace(input.Description)
	if title == "" || description == "" || len([]rune(title)) > 80 || len([]rune(description)) > 300 {
		return LocationCard{}, ErrSendInvalidInput
	}
	if math.IsNaN(input.Latitude) || math.IsInf(input.Latitude, 0) || input.Latitude < -90 || input.Latitude > 90 ||
		math.IsNaN(input.Longitude) || math.IsInf(input.Longitude, 0) || input.Longitude < -180 || input.Longitude > 180 {
		return LocationCard{}, ErrSendInvalidInput
	}
	return LocationCard{Title: title, Description: description, Latitude: input.Latitude, Longitude: input.Longitude}, nil
}

// encodeLocationCardContent 把位置卡片编码为本地稳定 JSON，用于幂等回显和跨分页展示。
func encodeLocationCardContent(card LocationCard) (string, error) {
	// raw、marshalErr 是字段顺序稳定的 JSON 正文及编码错误。
	raw, marshalErr := json.Marshal(card)
	return string(raw), marshalErr
}

// ParseLocationCardContent 解析本地稳定 JSON；损坏或非位置内容返回 false，避免前端收到半张卡片。
func ParseLocationCardContent(content string) (*LocationCard, bool) {
	// card、decodeErr 是解码后的结构化位置卡片及 JSON 错误。
	var card LocationCard
	// decodeErr 是本地位置正文无法还原为结构化字段时的 JSON 错误。
	decodeErr := json.Unmarshal([]byte(strings.TrimSpace(content)), &card)
	if decodeErr != nil || card.Title == "" || card.Description == "" ||
		math.IsNaN(card.Latitude) || math.IsInf(card.Latitude, 0) || card.Latitude < -90 || card.Latitude > 90 ||
		math.IsNaN(card.Longitude) || math.IsInf(card.Longitude, 0) || card.Longitude < -180 || card.Longitude > 180 {
		return nil, false
	}
	return &card, true
}

// normalizeOutgoingInput 校验发送会话并返回去除首尾空白的文字内容。
func normalizeOutgoingInput(session Session, text string) (Session, string, error) {
	session.AccountID = strings.TrimSpace(session.AccountID)
	session.ChatID = strings.TrimSpace(session.ChatID)
	session.BuyerID = strings.TrimSpace(session.BuyerID)
	text = strings.TrimSpace(text)
	if session.AccountID == "" || session.ChatID == "" || session.BuyerID == "" || text == "" || len([]rune(text)) > 2000 {
		return Session{}, "", ErrSendInvalidInput
	}
	return session, text, nil
}

// messagePointer 在状态更新返回空值时回退到已创建消息，确保错误响应仍能携带幂等键。
func messagePointer(message Message, fallback Message) *Message {
	if message.MessageKey == "" {
		message = fallback
	}
	return &message
}
