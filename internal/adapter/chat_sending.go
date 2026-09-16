package adapter

import (
	"context"
	"strings"

	"xianyu-go/internal/account"
	chatapp "xianyu-go/internal/application/chat"
	"xianyu-go/internal/automation"
	domainchat "xianyu-go/internal/chat"
	"xianyu-go/internal/db"
	"xianyu-go/internal/engine"
	"xianyu-go/internal/xianyu/mtop"
)

// chatOutgoingRepository 将聊天领域服务适配为应用层外发消息端口。
type chatOutgoingRepository struct {
	// service 保存聊天消息幂等写入和实时事件发布能力。
	service *domainchat.Service
}

// NewChatSendingApplication 装配聊天历史、实时发送、图片上传和身份补全端口。
func NewChatSendingApplication(domainService *domainchat.Service, store *db.Store, manager *account.Manager, clientProvider func() mtop.Client) *chatapp.Service {
	// readReporter 保存从账号运行时适配出的平台已读上报能力，未运行账号保持可选语义。
	readReporter := NewChatReadReporter(manager)
	if domainService == nil {
		// service 保留历史查询应用对象，但不伪造未装配的发送、订阅和刷新端口。
		service := chatapp.WithPlatformReadReporter(chatapp.NewWithSendingSubscriptionAndRefresh(NewChatRepository(store), nil, nil, nil, nil, nil, NewChatIdentityResolver(store, clientProvider)), readReporter)
		return chatapp.WithCreditResolver(service, NewChatCreditResolver(store, clientProvider, manager))
	}
	// service 是装配发送、订阅、刷新、身份和已读能力后的聊天应用服务。
	service := chatapp.WithPlatformReadReporter(chatapp.NewWithSendingSubscriptionAndRefresh(
		NewChatRepository(store),
		NewChatOutgoingRepository(domainService),
		NewChatSenderProvider(manager),
		NewChatImageUploader(store, clientProvider, manager),
		NewChatSubscriptionProvider(domainService),
		NewChatRefreshProvider(domainService, manager),
		NewChatIdentityResolver(store, clientProvider),
	), readReporter)
	return chatapp.WithCreditResolver(service, NewChatCreditResolver(store, clientProvider, manager))
}

// chatReadReporter 将账号运行时的可选已读上报能力适配为聊天应用端口。
type chatReadReporter struct {
	// manager 保存当前进程的账号运行时查询入口。
	manager *account.Manager
}

// NewChatReadReporter 创建聊天已读上报端口；nil Manager 表示当前进程不提供运行时上报能力。
func NewChatReadReporter(manager *account.Manager) chatapp.PlatformReadReporter {
	return chatReadReporter{manager: manager}
}

// ReportRead 向在线账号实例尽力上报平台已读状态；不支持该能力时保持无副作用。
func (reporter chatReadReporter) ReportRead(ctx context.Context, accountID, chatID string, messageIDs []map[string]any) error {
	if reporter.manager == nil {
		return nil
	}
	// sender、ok 保存账号运行实例与其在线状态。
	sender, ok := reporter.manager.GetInstance(accountID)
	if !ok || sender == nil {
		return nil
	}
	// reader、ok 保存运行时是否支持平台已读上报。
	reader, ok := sender.(interface {
		MarkChatRead(context.Context, string, []map[string]any) error
	})
	if !ok {
		return nil
	}
	return reader.MarkChatRead(ctx, chatID, messageIDs)
}

// NewChatOutgoingRepository 创建聊天外发消息的领域适配器。
func NewChatOutgoingRepository(service *domainchat.Service) chatapp.OutgoingRepository {
	return chatOutgoingRepository{service: service}
}

// CreateOutgoing 创建可选携带原生引用目标的文字消息，并转换为应用层非敏感模型。
func (r chatOutgoingRepository) CreateOutgoing(ctx context.Context, session chatapp.Session, text, replyToPlatformMessageID string) (chatapp.Message, error) {
	if r.service == nil {
		return chatapp.Message{}, chatapp.ErrUnavailable
	}
	// message、err 保存领域服务返回的消息及写入错误。
	message, err := r.service.CreateOutgoing(ctx, dbChatSession(session), text, replyToPlatformMessageID)
	return chatApplicationMessage(message), err
}

// CreateOutgoingMedia 创建媒体消息并转换为应用层非敏感模型。
func (r chatOutgoingRepository) CreateOutgoingMedia(ctx context.Context, session chatapp.Session, messageType, content string) (chatapp.Message, error) {
	if r.service == nil {
		return chatapp.Message{}, chatapp.ErrUnavailable
	}
	// message、err 保存领域服务返回的媒体消息及写入错误。
	message, err := r.service.CreateOutgoingMedia(ctx, dbChatSession(session), messageType, content)
	return chatApplicationMessage(message), err
}

// SetOutgoingStatus 更新外发消息状态并转换为应用层非敏感模型。
func (r chatOutgoingRepository) SetOutgoingStatus(ctx context.Context, accountID, key, status string) (chatapp.Message, error) {
	if r.service == nil {
		return chatapp.Message{}, chatapp.ErrUnavailable
	}
	// message、err 保存领域服务返回的状态消息及更新错误。
	message, err := r.service.SetOutgoingStatus(ctx, accountID, key, status)
	return chatApplicationMessage(message), err
}

// BindPlatformMessageID 保存平台发送响应中的 PNM ID 并转换最新消息。
func (r chatOutgoingRepository) BindPlatformMessageID(ctx context.Context, accountID, key, platformMessageID string) (chatapp.Message, error) {
	if r.service == nil {
		return chatapp.Message{}, chatapp.ErrUnavailable
	}
	// message、err 保存绑定平台 ID 后的领域消息和持久化错误。
	message, err := r.service.BindPlatformMessageID(ctx, accountID, key, platformMessageID)
	return chatApplicationMessage(message), err
}

// MarkMessageRecalled 保存平台撤回元数据并转换最新消息。
func (r chatOutgoingRepository) MarkMessageRecalled(ctx context.Context, accountID, platformMessageID string, operatorType int, operatorID string, recalledAt int64) (chatapp.Message, error) {
	if r.service == nil {
		return chatapp.Message{}, chatapp.ErrUnavailable
	}
	// message、err 保存撤回后的领域消息和持久化错误。
	message, err := r.service.MarkMessageRecalled(ctx, accountID, platformMessageID, operatorType, operatorID, recalledAt)
	return chatApplicationMessage(message), err
}

// dbChatSession 将应用层会话摘要转换为领域层写入模型，不携带账号凭证。
func dbChatSession(session chatapp.Session) db.ChatSession {
	return db.ChatSession{
		CookieID: session.AccountID, ChatID: session.ChatID, BuyerID: session.BuyerID,
		BuyerName: session.BuyerName, BuyerAvatar: session.BuyerAvatar, ItemID: session.ItemID,
		ItemTitle: session.ItemTitle, LastMessage: session.LastMessage, LastMessageAt: session.LastMessageAt,
		UnreadCount: session.UnreadCount,
	}
}

// chatApplicationMessage 将领域消息转换为应用层非敏感模型；空指针保持零值。
func chatApplicationMessage(message *db.ChatMessage) chatapp.Message {
	if message == nil {
		return chatapp.Message{}
	}
	// converted 保存不包含数据库内部字段的应用层聊天消息。
	converted := chatapp.Message{
		ID: message.ID, AccountID: message.CookieID, ChatID: message.ChatID, MessageKey: message.MessageKey,
		PlatformMessageID: message.PlatformMessageID, ReplyToPlatformMessageID: message.ReplyToPlatformMessageID,
		Direction: message.Direction, SenderID: message.SenderID, SenderName: message.SenderName,
		MessageType: message.MessageType, Content: message.Content, Status: message.Status,
		ReadStatus: message.ReadStatus, ReadAt: message.ReadAt, SentAt: message.SentAt,
		RecalledAt: message.RecalledAt, RecallOperatorType: message.RecallOperatorType, RecallOperatorID: message.RecallOperatorID,
	}
	if message.SystemCardKind != "" {
		// action 是持久化动作；历史送花卡片缺少动作时按已知事件和明确订单安全补齐官方入口。
		action := message.SystemCardAction
		if action == "" && message.SystemCardEvent == "red_flower_sent" && message.SystemCardOrderID != "" {
			action = "receive_red_flower"
		}
		if action == "" && message.SystemCardEvent == "order_paid" && message.SystemCardOrderID != "" && strings.TrimSpace(message.SenderID) != "" && strings.TrimSpace(message.SenderID) != strings.TrimSpace(message.CookieID) {
			// 历史付款卡片已经保存卖家账号、买家发送者和精确订单号，可安全补齐新 ship_order 展示动作。
			action = "ship_order"
		}
		converted.SystemCard = &chatapp.SystemCard{
			Kind: message.SystemCardKind, Event: message.SystemCardEvent, Title: message.SystemCardTitle,
			Description: message.SystemCardDescription, OrderID: message.SystemCardOrderID,
			ItemID: message.SystemCardItemID, Action: action,
		}
	}
	if strings.EqualFold(strings.TrimSpace(message.MessageType), "location") {
		// locationCard、ok 是从本地稳定 JSON 解析出的结构化位置卡片及完整性标记。
		if locationCard, ok := chatapp.ParseLocationCardContent(message.Content); ok {
			converted.LocationCard = locationCard
		}
	}
	if message.ReplyPreview != nil {
		converted.ReplyPreview = &chatapp.ReplyPreview{PlatformMessageID: message.ReplyPreview.PlatformMessageID,
			Direction: message.ReplyPreview.Direction, SenderID: message.ReplyPreview.SenderID, SenderName: message.ReplyPreview.SenderName,
			MessageType: message.ReplyPreview.MessageType, Content: message.ReplyPreview.Content, Status: message.ReplyPreview.Status}
	}
	return converted
}

// chatSenderProvider 将账号管理器适配为聊天应用的在线发送端口。
type chatSenderProvider struct {
	// manager 保存当前进程内的账号运行时管理器。
	manager *account.Manager
}

// NewChatSenderProvider 创建按账号解析在线发送器的适配器。
func NewChatSenderProvider(manager *account.Manager) chatapp.SenderProvider {
	return chatSenderProvider{manager: manager}
}

// Sender 返回指定账号的在线发送能力；账号不存在或运行时未装配时返回 false。
func (p chatSenderProvider) Sender(accountID string) (chatapp.Sender, bool) {
	if p.manager == nil {
		return nil, false
	}
	// sender、ok 保存账号管理器返回的运行时发送器及存在性。
	sender, ok := p.manager.GetInstance(accountID)
	if !ok || sender == nil {
		return nil, false
	}
	return chatSender{sender: sender}, true
}

// chatSender 将自动化消息发送接口收敛为应用聊天端口，并注入幂等键上下文。
type chatSender struct {
	// sender 保存账号运行时提供的最小消息发送能力。
	sender automation.MessageSender
}

// SendText 发送文字并将应用层幂等键传递给运行时旁路观察器。
func (s chatSender) SendText(ctx context.Context, chatID, toUserID, text, messageKey string) (chatapp.PlatformSendResult, error) {
	if s.sender == nil {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// resultSender 是账号运行时提供的带 PNM ID 文本发送能力。
	resultSender, ok := s.sender.(interface {
		SendTextWithResult(context.Context, string, string, string) (engine.PlatformSendResult, error)
	})
	if !ok {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// result、err 保存平台发送结果和错误。
	result, err := resultSender.SendTextWithResult(engine.WithOutgoingMessageKey(ctx, messageKey), chatID, toUserID, text)
	return chatapp.PlatformSendResult{PlatformMessageID: result.PlatformMessageID, CreatedAt: result.CreatedAt}, err
}

// SendReplyText 把已校验的目标 PNM ID 传递给账号运行时，不允许降级为普通文本。
func (s chatSender) SendReplyText(ctx context.Context, chatID, toUserID, text, replyToPlatformMessageID, messageKey string) (chatapp.PlatformSendResult, error) {
	if s.sender == nil {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// resultSender 是账号运行时提供的带 PNM ID 原生引用发送能力。
	resultSender, ok := s.sender.(interface {
		SendReplyTextWithResult(context.Context, string, string, string, string) (engine.PlatformSendResult, error)
	})
	if !ok {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// result、err 保存原生引用发送结果和错误。
	result, err := resultSender.SendReplyTextWithResult(engine.WithOutgoingMessageKey(ctx, messageKey), chatID, toUserID, text, replyToPlatformMessageID)
	return chatapp.PlatformSendResult{PlatformMessageID: result.PlatformMessageID, CreatedAt: result.CreatedAt}, err
}

// SendImage 发送图片并将应用层幂等键传递给运行时接口。
func (s chatSender) SendImage(ctx context.Context, chatID, toUserID, imageURL string, cardID int64, width, height int, messageKey string) (chatapp.PlatformSendResult, error) {
	if s.sender == nil {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// resultSender 是账号运行时提供的带 PNM ID 图片发送能力。
	resultSender, ok := s.sender.(interface {
		SendImageWithResult(context.Context, string, string, string, int64, int, int) (engine.PlatformSendResult, error)
	})
	if !ok {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// result、err 保存平台图片发送结果和错误。
	result, err := resultSender.SendImageWithResult(engine.WithOutgoingMessageKey(ctx, messageKey), chatID, toUserID, imageURL, cardID, width, height)
	return chatapp.PlatformSendResult{PlatformMessageID: result.PlatformMessageID, CreatedAt: result.CreatedAt}, err
}

// SendLocationCard 通过当前在线账号发送位置卡片，并把本地幂等键交给出站旁路。
func (s chatSender) SendLocationCard(ctx context.Context, chatID, toUserID, title, description string, latitude, longitude float64, messageKey string) (chatapp.PlatformSendResult, error) {
	if s.sender == nil {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// resultSender、ok 是账号运行时的位置卡片能力及接口匹配结果。
	resultSender, ok := s.sender.(interface {
		SendLocationCardWithResult(context.Context, string, string, string, string, float64, float64) (engine.PlatformSendResult, error)
	})
	if !ok {
		return chatapp.PlatformSendResult{}, chatapp.ErrUnavailable
	}
	// result、sendErr 是平台位置卡片发送结果及明确失败。
	result, sendErr := resultSender.SendLocationCardWithResult(engine.WithOutgoingMessageKey(ctx, messageKey), chatID, toUserID, title, description, latitude, longitude)
	return chatapp.PlatformSendResult{PlatformMessageID: result.PlatformMessageID, CreatedAt: result.CreatedAt}, sendErr
}

// RecallMessage 委托账号运行时执行官方发送者撤回。
func (s chatSender) RecallMessage(ctx context.Context, platformMessageID string, contentType int, text string) error {
	if s.sender == nil {
		return chatapp.ErrUnavailable
	}
	// recallSender 是账号运行时提供的官方撤回能力。
	recallSender, ok := s.sender.(interface {
		RecallMessage(context.Context, string, int, string) error
	})
	if !ok {
		return chatapp.ErrUnavailable
	}
	return recallSender.RecallMessage(ctx, platformMessageID, contentType, text)
}

// chatCredentialRepository 将 Cookie 读取与写回限制在平台适配器内。
type chatCredentialRepository struct {
	// store 保存数据库聚合入口，仅在适配器内读取和更新明文 Cookie。
	store *db.Store
}

// getCookieValue 读取图片上传所需的账号凭证；调用方不得记录返回值。
func (r chatCredentialRepository) getCookieValue(ctx context.Context, accountID string) (string, error) {
	if r.store == nil || r.store.Cookies == nil {
		return "", chatapp.ErrUnavailable
	}
	return r.store.Cookies.GetValue(ctx, accountID)
}

// updateCookieValue 保存图片上传后平台刷新的账号凭证。
func (r chatCredentialRepository) updateCookieValue(ctx context.Context, accountID, cookieValue string) error {
	if r.store == nil || r.store.Cookies == nil {
		return chatapp.ErrUnavailable
	}
	return r.store.Cookies.UpdateValueExisting(ctx, accountID, cookieValue)
}

// chatImageUploader 将 MTOP 图片上传和凭证刷新适配为聊天应用端口。
type chatImageUploader struct {
	// clientProvider 返回当前可注入的 MTOP 客户端，便于运行时替换和测试。
	clientProvider func() mtop.Client
	// credentials 负责在平台刷新后持久化明文凭证，但不向应用层返回。
	credentials chatCredentialRepository
	// manager 负责将刷新后的凭证同步到在线运行时。
	manager *account.Manager
}

// NewChatImageUploader 创建聊天图片上传适配器；平台客户端和数据库依赖由调用方注入。
func NewChatImageUploader(store *db.Store, clientProvider func() mtop.Client, manager *account.Manager) chatapp.ImageUploader {
	return chatImageUploader{clientProvider: clientProvider, credentials: chatCredentialRepository{store: store}, manager: manager}
}

// UploadChatImage 在适配器内部读取和刷新凭证，只向应用层返回图片地址。
func (u chatImageUploader) UploadChatImage(ctx context.Context, accountID, filename, contentType string, data []byte) (chatapp.ImageUpload, error) {
	if u.clientProvider == nil {
		return chatapp.ImageUpload{}, chatapp.ErrUnavailable
	}
	// cookieValue 和 err 保存平台调用所需的短暂明文凭证及读取错误，不得离开适配器。
	cookieValue, err := u.credentials.getCookieValue(ctx, accountID)
	if err != nil {
		return chatapp.ImageUpload{}, err
	}
	// client 保存当前可用的 MTOP 客户端。
	client := u.clientProvider()
	// uploader、ok 保存 MTOP 图片上传能力及接口支持情况。
	uploader, ok := client.(interface {
		UploadChatImage(context.Context, string, string, string, []byte) (*mtop.ChatImageUpload, error)
	})
	if !ok {
		return chatapp.ImageUpload{}, chatapp.ErrUnavailable
	}
	// upload、err 保存图片平台返回结果及调用错误。
	upload, err := uploader.UploadChatImage(ctx, cookieValue, filename, contentType, data)
	if err != nil {
		return chatapp.ImageUpload{}, err
	}
	if upload == nil {
		return chatapp.ImageUpload{}, chatapp.ErrSend
	}
	if upload.UpdatedCookies != "" && upload.UpdatedCookies != cookieValue {
		// persistErr 保存刷新凭证的持久化错误；该错误必须反馈给调用方，避免静默丢失会话状态。
		if persistErr := u.credentials.updateCookieValue(ctx, accountID, upload.UpdatedCookies); persistErr != nil {
			return chatapp.ImageUpload{}, persistErr
		}
		if u.manager != nil {
			// sender、senderOK 保存刷新凭证同步到运行时的结果。
			sender, senderOK := u.manager.GetInstance(accountID)
			if senderOK && sender != nil {
				sender.UpdateCookie(upload.UpdatedCookies)
			}
		}
	}
	return chatapp.ImageUpload{URL: upload.URL, Width: upload.Width, Height: upload.Height}, nil
}

// 编译期确认聊天实时适配器覆盖应用层定义的全部能力。
var (
	_ chatapp.OutgoingRepository = chatOutgoingRepository{}
	_ chatapp.SenderProvider     = chatSenderProvider{}
	_ chatapp.Sender             = chatSender{}
	_ chatapp.ImageUploader      = chatImageUploader{}
)
