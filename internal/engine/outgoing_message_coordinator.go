package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"xianyu-go/internal/automation"
	"xianyu-go/internal/xianyu/protocol"
	wsprotocol "xianyu-go/internal/xianyu/ws"
)

// PlatformSendResult 是账号运行时向应用层返回的非敏感平台发送结果。
type PlatformSendResult struct {
	// PlatformMessageID 是闲鱼 PNM 消息 ID。
	PlatformMessageID string
	// CreatedAt 是平台消息创建时间，Unix 毫秒。
	CreatedAt int64
}

// outgoingMessageCoordinator 拥有当前连接上的出站消息、聊天历史和会话查询边界。
// 它只在锁内读取 WebSocket 与账号身份快照，任何发送或查询 I/O 都在锁外执行。
type outgoingMessageCoordinator struct {
	// account 是构造完成后固定的账号 facade，提供连接状态和出站旁路观察器。
	account *Account
}

// locationCardObservation 是出站旁路使用的稳定 JSON 结构，字段顺序与应用层本地消息保持一致。
type locationCardObservation struct {
	// Title 是用户确认的店铺或地点名称。
	Title string `json:"title"`
	// Description 是用户确认的地址或到店说明。
	Description string `json:"description"`
	// Latitude 是 WGS84 纬度十进制度。
	Latitude float64 `json:"latitude"`
	// Longitude 是 WGS84 经度十进制度。
	Longitude float64 `json:"longitude"`
}

// sendText 使用当前已注册 WebSocket 发送文本，并在平台接受后通知可选的聊天旁路。
// ctx 是调用方取消边界；chatID、toUserID 与 text 共同确定一次出站消息；错误保持原调用方语义。
func (c *outgoingMessageCoordinator) sendText(ctx context.Context, chatID, toUserID, text string) error {
	// err 是带结果发送返回的错误；自动化发送不消费平台 ID。
	_, err := c.sendTextWithResult(ctx, chatID, toUserID, text, "")
	return err
}

// sendTextWithResult 使用当前连接发送普通或原生引用文本，并在支持时返回平台 PNM ID。
func (c *outgoingMessageCoordinator) sendTextWithResult(ctx context.Context, chatID, toUserID, text, replyToPlatformMessageID string) (PlatformSendResult, error) {
	// a 是当前协调器绑定的账号 facade；它在 New 中写入且之后不可替换。
	a := c.account
	if a == nil {
		return PlatformSendResult{}, errors.New("账号出站消息协调器未初始化")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return PlatformSendResult{}, nil
	}
	// conn、myID、err 保存锁外发送所需的连接与账号身份快照，以及读取失败原因。
	conn, myID, err := c.currentSenderState()
	if err != nil {
		return PlatformSendResult{}, err
	}
	// sendCtx、cancel 限制单次文本发送的最长等待，并在函数返回时释放计时器。
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// result 保存支持请求关联的生产连接返回的 PNM ID；旧测试连接保持兼容空结果。
	var result PlatformSendResult
	if replyToPlatformMessageID != "" {
		// replySender 是生产 WebSocket 提供的带结果原生引用发送能力。
		replySender, supported := conn.(interface {
			SendReplyTextWithResult(context.Context, string, string, string, string, string) (wsprotocol.SendResult, error)
		})
		if !supported {
			return PlatformSendResult{}, errors.New("当前 WebSocket 不支持原生引用回复")
		}
		// platformResult、sendErr 保存引用发送结果及明确错误。
		platformResult, sendErr := replySender.SendReplyTextWithResult(sendCtx, myID, chatID, toUserID, text, replyToPlatformMessageID)
		if sendErr != nil {
			return PlatformSendResult{}, sendErr
		}
		result = PlatformSendResult{PlatformMessageID: platformResult.PlatformMessageID, CreatedAt: platformResult.CreatedAt}
	} else if // resultSender、supported 是普通文本带结果发送能力和接口匹配标记。
	resultSender, supported := conn.(interface {
		SendTextWithResult(context.Context, string, string, string, string) (wsprotocol.SendResult, error)
	}); supported {
		// platformResult、sendErr 保存平台发送结果及明确失败。
		// sendErr 表示平台带结果文本发送失败。
		platformResult, sendErr := resultSender.SendTextWithResult(sendCtx, myID, chatID, toUserID, text)
		if sendErr != nil {
			return PlatformSendResult{}, sendErr
		}
		result = PlatformSendResult{PlatformMessageID: platformResult.PlatformMessageID, CreatedAt: platformResult.CreatedAt}
	} else {
		// sendErr 是旧测试连接不支持结果返回时的兼容发送错误。
		if sendErr := conn.SendText(sendCtx, myID, chatID, toUserID, text); sendErr != nil {
			return PlatformSendResult{}, sendErr
		}
	}
	// observer、ok 是可选出站旁路观察器及其接口匹配结果；旁路失败不能改变平台发送成功结果。
	if observer, ok := a.handler.(outgoingChatHandler); ok {
		// key 是 UI 创建的待发送消息关联键，避免旁路重复插入同一文本。
		key, _ := ctx.Value(outgoingMessageKeyContextKey{}).(string)
		// err 是旁路持久化或广播失败原因，仅记录脱敏告警。
		if err := observer.HandleOutgoingChatMessage(ctx, OutgoingChatMessage{
			AccountID: a.CookieID, ChatID: chatID, BuyerID: toUserID, Text: text, MessageKey: key,
			MessageType: "text", PlatformMessageID: result.PlatformMessageID, SentAt: result.CreatedAt,
		}); err != nil {
			a.logger.Warn("保存出站聊天旁路失败", "account", a.CookieID, "chat_id", chatID, "err", err)
		}
	}
	return result, nil
}

// sendImage 使用当前已注册 WebSocket 发送可直接访问的远程图片。
// ctx 是调用方取消边界；cardID 仅用于兼容 MessageSender 契约，当前协议发送不直接使用它；width/height 为图片像素尺寸。
func (c *outgoingMessageCoordinator) sendImage(ctx context.Context, chatID, toUserID, imageURL string, cardID int64, width, height int) error {
	// result、sendErr 分别是平台图片标识和明确发送错误；只有平台成功后才产生聊天旁路事件。
	result, sendErr := c.sendImageWithResult(ctx, chatID, toUserID, imageURL, cardID, width, height)
	if sendErr != nil {
		return sendErr
	}
	// a 是当前账号 facade；sendImageWithResult 已完成非空校验，此处只读取旁路观察器。
	a := c.account
	if strings.TrimSpace(result.PlatformMessageID) == "" || a == nil {
		return nil
	}
	// observer、ok 分别是可选聊天出站观察器及当前 handler 是否支持该能力。
	if observer, ok := a.handler.(outgoingChatHandler); ok {
		// key 是人工发送预建消息使用的本地键；自动化图片通常为空并改用平台 PNM 幂等。
		key, _ := ctx.Value(outgoingMessageKeyContextKey{}).(string)
		// observeErr 是本地聊天持久化或广播错误；平台图片已经成功，旁路失败只能记录告警。
		if observeErr := observer.HandleOutgoingChatMessage(ctx, OutgoingChatMessage{
			AccountID: a.CookieID, ChatID: chatID, BuyerID: toUserID, Text: imageURL, MessageKey: key,
			MessageType: "image", PlatformMessageID: result.PlatformMessageID, SentAt: result.CreatedAt,
		}); observeErr != nil {
			a.logger.Warn("保存出站图片聊天旁路失败", "account", a.CookieID, "chat_id", chatID, "err", observeErr)
		}
	}
	return nil
}

// sendImageWithResult 使用当前连接发送图片，并在支持时返回平台 PNM ID。
func (c *outgoingMessageCoordinator) sendImageWithResult(ctx context.Context, chatID, toUserID, imageURL string, cardID int64, width, height int) (PlatformSendResult, error) {
	// a 是当前协调器绑定的账号 facade；它用于维持与文本发送一致的初始化检查。
	a := c.account
	if a == nil {
		return PlatformSendResult{}, errors.New("账号出站消息协调器未初始化")
	}
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return PlatformSendResult{}, nil
	}
	if strings.HasPrefix(imageURL, "/static/") || strings.HasPrefix(imageURL, "static/") {
		return PlatformSendResult{}, fmt.Errorf("当前运行时暂不支持本地图片自动上传到闲鱼 CDN: %s", imageURL)
	}
	// conn、myID、err 保存锁外图片发送所需的连接与账号身份快照，以及读取失败原因。
	conn, myID, err := c.currentSenderState()
	if err != nil {
		return PlatformSendResult{}, err
	}
	// sendCtx、cancel 限制单次图片发送的最长等待，并在函数返回时释放计时器。
	sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_ = cardID // cardID 由上层动作检查点持久化，协议图片发送本身不携带该字段。
	// resultSender 是生产 WebSocket 提供的带结果图片发送能力。
	if resultSender, supported := conn.(interface {
		SendImageWithResult(context.Context, string, string, string, string, int, int) (wsprotocol.SendResult, error)
	}); supported {
		// platformResult、sendErr 保存平台图片发送结果及错误。
		platformResult, sendErr := resultSender.SendImageWithResult(sendCtx, myID, chatID, toUserID, imageURL, width, height)
		return PlatformSendResult{PlatformMessageID: platformResult.PlatformMessageID, CreatedAt: platformResult.CreatedAt}, sendErr
	}
	return PlatformSendResult{}, conn.SendImage(sendCtx, myID, chatID, toUserID, imageURL, width, height)
}

// sendLocationCardWithResult 复用当前账号连接发送一张位置卡片，并在平台成功后发布本地结构化回显。
func (c *outgoingMessageCoordinator) sendLocationCardWithResult(ctx context.Context, chatID, toUserID, title, description string, latitude, longitude float64) (PlatformSendResult, error) {
	// account 是当前协调器绑定的账号 facade，提供连接与旁路观察器。
	account := c.account
	if account == nil {
		return PlatformSendResult{}, errors.New("账号出站消息协调器未初始化")
	}
	// conn、myID、stateErr 是锁外发送需要的连接、账号身份及不可用原因。
	conn, myID, stateErr := c.currentSenderState()
	if stateErr != nil {
		return PlatformSendResult{}, stateErr
	}
	// locationSender、supported 是生产 WebSocket 的位置卡片能力及接口匹配结果。
	locationSender, supported := conn.(interface {
		SendLocationCardWithResult(context.Context, string, string, string, string, string, float64, float64) (wsprotocol.SendResult, error)
	})
	if !supported {
		return PlatformSendResult{}, errors.New("当前 WebSocket 不支持位置卡片")
	}
	// sendCtx、cancel 把单次位置卡片平台写入限制在八秒内，并由本方法释放计时器。
	sendCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	// platformResult、sendErr 是平台 PNM 结果及明确发送失败。
	platformResult, sendErr := locationSender.SendLocationCardWithResult(sendCtx, myID, chatID, toUserID, title, description, latitude, longitude)
	if sendErr != nil {
		return PlatformSendResult{}, sendErr
	}
	// result 是向应用层公开的非敏感平台标识和创建时间。
	result := PlatformSendResult{PlatformMessageID: platformResult.PlatformMessageID, CreatedAt: platformResult.CreatedAt}
	// observationJSON、marshalErr 是与应用层本地待发送消息一致的最小结构化正文。
	observationJSON, marshalErr := json.Marshal(locationCardObservation{Title: strings.TrimSpace(title), Description: strings.TrimSpace(description), Latitude: latitude, Longitude: longitude})
	if marshalErr != nil {
		account.logger.Warn("编码位置卡片出站旁路失败", "account", account.CookieID, "chat_id", chatID, "err", marshalErr)
		return result, nil
	}
	// observer、ok 是可选出站聊天观察器及其接口匹配结果。
	if observer, ok := account.handler.(outgoingChatHandler); ok {
		// key 是 HTTP 应用层预建的本地幂等消息键。
		key, _ := ctx.Value(outgoingMessageKeyContextKey{}).(string)
		// observeErr 是平台成功后本地持久化或广播失败，不反转发送结果。
		if observeErr := observer.HandleOutgoingChatMessage(ctx, OutgoingChatMessage{
			AccountID: account.CookieID, ChatID: chatID, BuyerID: toUserID, Text: string(observationJSON), MessageKey: key,
			MessageType: "location", PlatformMessageID: result.PlatformMessageID, SentAt: result.CreatedAt,
		}); observeErr != nil {
			account.logger.Warn("保存出站位置卡片旁路失败", "account", account.CookieID, "chat_id", chatID, "err", observeErr)
		}
	}
	return result, nil
}

// recallMessage 使用当前在线连接按官方 Feature 模型撤回发送者消息。
func (c *outgoingMessageCoordinator) recallMessage(ctx context.Context, platformMessageID string, contentType int, text string) error {
	// conn、myID、err 保存撤回所需连接、账号身份和读取失败原因。
	conn, myID, err := c.currentSenderState()
	if err != nil {
		return err
	}
	// recallConn 是生产 WebSocket 提供的官方撤回能力。
	recallConn, supported := conn.(interface {
		RecallMessageByFeature(context.Context, string, wsprotocol.RecallFeature) error
	})
	if !supported {
		return errors.New("当前 WebSocket 不支持消息撤回")
	}
	// recallCtx、cancel 限制单次撤回等待时间，超时结果由上层标记待确认。
	recallCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return recallConn.RecallMessageByFeature(recallCtx, platformMessageID, wsprotocol.RecallFeature{
		OperatorID: myID, OriginContentType: contentType, TextContent: text,
	})
}

// currentSenderState 返回可用 WebSocket 与账号 unb 身份快照；持锁范围只覆盖快照读取。
func (c *outgoingMessageCoordinator) currentSenderState() (WSConn, string, error) {
	// a 是当前协调器绑定的账号 facade；未初始化时不能安全读取连接状态。
	a := c.account
	if a == nil {
		return nil, "", errors.New("账号出站消息协调器未初始化")
	}
	a.runtimeMu.Lock()
	// conn 是当前连接快照；后续读取账号身份字段使用 Account 自身的凭证锁。
	conn := a.conn
	a.runtimeMu.Unlock()
	if conn == nil {
		return nil, "", fmt.Errorf("%w: 账号 %s 当前没有可用 WebSocket 连接", automation.ErrMessageNotSent, a.CookieID)
	}
	a.mu.Lock()
	// myID 是发送协议所需的当前账号 unb 身份快照。
	myID := strings.TrimSpace(a.UserID)
	if myID == "" {
		myID = protocol.TransCookies(a.CookieStr)["unb"]
	}
	a.mu.Unlock()
	if myID == "" {
		return nil, "", fmt.Errorf("%w: 账号 %s 缺少 unb，无法发送消息", automation.ErrMessageNotSent, a.CookieID)
	}
	return conn, myID, nil
}

// fetchChatHistory 使用当前已注册连接查询指定聊天的历史消息。
// ctx、chatID、cursor 与 limit 直接传给平台连接；返回值保留账号身份快照与原始平台正文。
func (c *outgoingMessageCoordinator) fetchChatHistory(ctx context.Context, chatID string, cursor int64, limit int) (map[string]any, string, error) {
	// conn、myID、err 保存历史查询所需连接、账号身份快照与读取失败原因。
	conn, myID, err := c.currentSenderState()
	if err != nil {
		return nil, "", err
	}
	// history、ok 保存连接是否支持历史查询的可选能力及其类型判断结果。
	history, ok := conn.(interface {
		ListUserMessages(context.Context, string, int64, int) (map[string]any, error)
	})
	if !ok {
		return nil, "", errors.New("当前 WebSocket 连接不支持聊天历史")
	}
	// body、err 保存平台返回的历史正文与查询错误。
	body, err := history.ListUserMessages(ctx, chatID, cursor, limit)
	return body, myID, err
}

// fetchChatConversations 使用当前已注册连接查询历史会话。
// ctx、cursor 与 limit 直接传给平台连接；返回值保留账号身份快照与原始平台正文。
func (c *outgoingMessageCoordinator) fetchChatConversations(ctx context.Context, cursor int64, limit int) (map[string]any, string, error) {
	// conn、myID、err 保存会话查询所需连接、账号身份快照与读取失败原因。
	conn, myID, err := c.currentSenderState()
	if err != nil {
		return nil, "", err
	}
	// fetcher、ok 保存连接是否支持会话查询的可选能力及其类型判断结果。
	fetcher, ok := conn.(interface {
		ListConversations(context.Context, int64, int) (map[string]any, error)
	})
	if !ok {
		return nil, "", errors.New("当前 WebSocket 连接不支持历史会话")
	}
	// body、err 保存平台返回的会话正文与查询错误。
	body, err := fetcher.ListConversations(ctx, cursor, limit)
	return body, myID, err
}

// automationReady 返回当前 WebSocket 是否已进入 online 状态，供 Automation 在发送前做无 I/O 门禁。
func (c *outgoingMessageCoordinator) automationReady() bool {
	// a 是当前协调器绑定的账号 facade；未初始化协调器不可能提供在线发送能力。
	a := c.account
	if a == nil {
		return false
	}
	a.runtimeMu.Lock()
	// ready 表示连接存在且状态已进入 online 的瞬时快照。
	ready := a.conn != nil && a.runtimeState == RuntimeOnline
	a.runtimeMu.Unlock()
	return ready
}
