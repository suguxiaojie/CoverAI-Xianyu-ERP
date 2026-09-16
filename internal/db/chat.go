package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ChatSession 用于本次流程后续判断的聊天会话
type ChatSession struct {
	CookieID      string `json:"account_id"`
	ChatID        string `json:"chat_id"`
	BuyerID       string `json:"buyer_id"`
	BuyerName     string `json:"buyer_name"`
	BuyerAvatar   string `json:"buyer_avatar_url"`
	ItemID        string `json:"item_id"`
	ItemTitle     string `json:"item_title"`
	LastMessage   string `json:"last_message"`
	LastMessageAt int64  `json:"last_message_at"`
	UnreadCount   int    `json:"unread_count"`
	// IsPinned 表示当前会话是否由 ERP 用户手工置顶，不会同步到闲鱼平台。
	IsPinned bool `json:"is_pinned"`
	// PinnedAt 是最近一次从未置顶切换为置顶的 Unix 秒时间，取消置顶时归零。
	PinnedAt int64 `json:"pinned_at"`
}

// ChatMessage 用于本次流程后续判断的聊天消息
type ChatMessage struct {
	ID         int64  `json:"id"`
	CookieID   string `json:"account_id"`
	ChatID     string `json:"chat_id"`
	MessageKey string `json:"message_key"`
	// PlatformMessageID 是闲鱼撤回和回执使用的 PNM ID；本地待发送消息取得响应前为空。
	PlatformMessageID string `json:"platform_message_id,omitempty"`
	// ReplyToPlatformMessageID 是当前消息原生引用的目标 PNM ID；普通消息为空。
	ReplyToPlatformMessageID string `json:"reply_to_platform_message_id,omitempty"`
	// ReplyPreview 是历史查询在同账号同会话内解析的最小引用目标快照；持久化时不单独写入。
	ReplyPreview *ChatReplyPreview `json:"reply_preview,omitempty"`
	Direction    string            `json:"direction"`
	SenderID     string            `json:"sender_id"`
	SenderName   string            `json:"sender_name"`
	MessageType  string            `json:"message_type"`
	Content      string            `json:"content"`
	// Summary 是仅用于更新会话列表的瞬时展示摘要；为空时使用 Content，不单独持久化到消息行。
	Summary    string `json:"-"`
	Status     string `json:"status"`
	ReadStatus int    `json:"read_status"`
	ReadAt     int64  `json:"read_at,omitempty"`
	SentAt     int64  `json:"sent_at"`
	// RecalledAt 是平台确认撤回的 Unix 毫秒时间；零值表示仍为普通消息。
	RecalledAt int64 `json:"recalled_at,omitempty"`
	// RecallOperatorType 区分发送者、群主、系统和安全撤回；未撤回时为 -1。
	RecallOperatorType int `json:"recall_operator_type"`
	// RecallOperatorID 保存平台撤回操作者标识，不包含账号凭证。
	RecallOperatorID string `json:"recall_operator_id,omitempty"`
	// PlatformContentType 是闲鱼消息协议类型；26 表示官方交易卡片。
	PlatformContentType int `json:"platform_content_type,omitempty"`
	// SystemCardKind 区分交易卡片和未来其他结构化系统卡片；普通消息为空。
	SystemCardKind string `json:"system_card_kind,omitempty"`
	// SystemCardEvent 是归一化后的交易状态，不依赖平台中文展示文案。
	SystemCardEvent string `json:"system_card_event,omitempty"`
	// SystemCardTitle 是平台卡片标题，只保存非敏感展示文本。
	SystemCardTitle string `json:"system_card_title,omitempty"`
	// SystemCardDescription 是平台卡片说明，只保存非敏感展示文本。
	SystemCardDescription string `json:"system_card_description,omitempty"`
	// SystemCardOrderID 是从明确平台字段或动作地址解析出的交易订单标识。
	SystemCardOrderID string `json:"system_card_order_id,omitempty"`
	// SystemCardItemID 是从明确平台字段或动作地址解析出的商品标识。
	SystemCardItemID string `json:"system_card_item_id,omitempty"`
	// SystemCardAction 是归一化后的只读动作提示；第一阶段不会执行外部写操作。
	SystemCardAction string `json:"system_card_action,omitempty"`
}

// ChatLatestReadState 保存会话最新消息用于未读摘要归一化的最小状态。
type ChatLatestReadState struct {
	// SentAt 是消息平台毫秒时间戳。
	SentAt int64
	// ReadStatus 为 2 时表示该消息已经确认已读。
	ReadStatus int
}

// ChatReplyPreview 是历史消息气泡区分原生引用所需的同会话非敏感目标快照。
type ChatReplyPreview struct {
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
	// Content 是被引用消息的原文或媒体地址，仅用于构造展示摘要。
	Content string `json:"content"`
	// Status 是被引用消息的当前状态，用于显示已撤回占位。
	Status string `json:"status"`
}

// ChatStore 用于本次流程后续判断的聊天Store
type ChatStore struct {
	DB      *sql.DB
	Dialect Dialect
}

// UpsertSession 封装Upsert会话业务协调。
func (s *ChatStore) UpsertSession(ctx context.Context, session ChatSession) error {
	// now 用于本次流程后续判断的now
	now := time.Now().UTC().Unix()
	// prefix 用于本次流程后续判断的prefix
	prefix := dialectInsertIgnorePrefix(s.Dialect)
	// query 用于本次流程后续判断的查询
	query := prefix + ` INTO chat_sessions
		(cookie_id,chat_id,buyer_id,buyer_name,buyer_avatar_url,item_id,item_title,last_message,last_message_at,unread_count,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)` + dialectInsertIgnore(s.Dialect, []string{"cookie_id", "chat_id"})
	if // err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, query, session.CookieID, session.ChatID, session.BuyerID, session.BuyerName,
		session.BuyerAvatar, session.ItemID, session.ItemTitle, session.LastMessage, session.LastMessageAt,
		session.UnreadCount, now, now); err != nil {
		return err
	}
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE chat_sessions SET
		buyer_id=CASE WHEN ?<>'' THEN ? ELSE buyer_id END,
		buyer_name=CASE WHEN ?<>'' THEN ? ELSE buyer_name END,
		buyer_avatar_url=CASE WHEN ?<>'' THEN ? ELSE buyer_avatar_url END,
		item_id=CASE WHEN ?<>'' THEN ? ELSE item_id END,
		item_title=CASE WHEN ?<>'' THEN ? ELSE item_title END,
		last_message=CASE WHEN last_message_at<=? THEN ? ELSE last_message END,
		last_message_at=CASE WHEN last_message_at<=? THEN ? ELSE last_message_at END,
		unread_count=CASE WHEN ?>unread_count THEN ? ELSE unread_count END,updated_at=?
		WHERE cookie_id=? AND chat_id=?`, session.BuyerID, session.BuyerID, session.BuyerName, session.BuyerName,
		session.BuyerAvatar, session.BuyerAvatar, session.ItemID, session.ItemID, session.ItemTitle, session.ItemTitle,
		session.LastMessageAt, session.LastMessage, session.LastMessageAt, session.LastMessageAt,
		session.UnreadCount, session.UnreadCount, now, session.CookieID, session.ChatID)
	return err
}

// DeleteSession 删除会话。
func (s *ChatStore) DeleteSession(ctx context.Context, cookieID, chatID string) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `DELETE FROM chat_sessions WHERE cookie_id=? AND chat_id=?`, cookieID, chatID)
	return err
}

// DeleteEmptySessions removes conversation shells returned by IM pagination
// with visible=0 and no lastMessage. Older versions persisted these shells as
// "暂无消息", although the official UI never renders them.
// DeleteEmptySessions 删除EmptySessions。
func (s *ChatStore) DeleteEmptySessions(ctx context.Context, cookieID string) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `DELETE FROM chat_sessions
		WHERE cookie_id=? AND (last_message='' OR last_message='暂无消息')
		AND NOT EXISTS (SELECT 1 FROM chat_messages m WHERE m.cookie_id=chat_sessions.cookie_id AND m.chat_id=chat_sessions.chat_id)`, cookieID)
	return err
}

// SyncSessionSummary applies the authoritative last-message timestamp from the
// official conversation response. observedModifyAt guards against overwriting
// a genuinely newer live message that arrived after that response was built.
// SyncSessionSummary 同步会话Summary。
func (s *ChatStore) SyncSessionSummary(ctx context.Context, cookieID, chatID, summary string, sentAt, observedModifyAt int64, unread int) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE chat_sessions SET last_message=?,last_message_at=?,unread_count=?,updated_at=?
		WHERE cookie_id=? AND chat_id=? AND last_message_at<=?`, summary, sentAt, unread, time.Now().UTC().Unix(),
		cookieID, chatID, observedModifyAt)
	return err
}

// UpdateSessionIdentity 更新会话Identity。
func (s *ChatStore) UpdateSessionIdentity(ctx context.Context, cookieID, chatID, buyerID, buyerName, avatarURL string) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE chat_sessions SET
		buyer_id=CASE WHEN ?<>'' THEN ? ELSE buyer_id END,
		buyer_name=CASE WHEN ?<>'' THEN ? ELSE buyer_name END,
		buyer_avatar_url=CASE WHEN ?<>'' THEN ? ELSE buyer_avatar_url END,
		updated_at=? WHERE cookie_id=? AND chat_id=?`, buyerID, buyerID, buyerName, buyerName,
		avatarURL, avatarURL, time.Now().UTC().Unix(), cookieID, chatID)
	return err
}

// LatestUnmaskedPeerName recovers the most recent real nickname observed in
// message history. Conversation summaries and profile APIs may return masked
// names such as x***3, while older message extensions still contain the nick.
// LatestUnmaskedPeerName 封装LatestUnmaskedPeer名称业务协调。
func (s *ChatStore) LatestUnmaskedPeerName(ctx context.Context, cookieID, chatID string) (string, error) {
	// name 用于本次流程后续判断的名称
	var name string
	// err 用于本次流程后续判断的err
	err := s.DB.QueryRowContext(ctx, `SELECT sender_name FROM chat_messages
		WHERE cookie_id=? AND chat_id=? AND direction='incoming' AND sender_name<>'' AND sender_name NOT LIKE '%***%'
			AND message_type<>'system'
			AND sender_name<>content AND sender_name NOT IN ('交易消息','系统消息','卡片消息','我完成了评价','对方完成了评价',
			'快给ta一个评价吧～','卖家已发货','买家已付款','买家已确认收货','等待您发货','超时未付款，系统关闭了订单','邀您填写售后问卷')
		ORDER BY sent_at DESC,id DESC LIMIT 1`, cookieID, chatID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(name), err
}

// SaveMessage inserts a message idempotently and updates its conversation only
// when the message was new. This keeps retries from inflating unread counters.
// SaveMessage 保存消息。
func (s *ChatStore) SaveMessage(ctx context.Context, session ChatSession, message ChatMessage, unread bool) (*ChatMessage, bool, error) {
	if s == nil || s.DB == nil {
		return nil, false, errors.New("聊天存储未初始化")
	}
	session.CookieID = strings.TrimSpace(session.CookieID)
	session.ChatID = strings.TrimSpace(session.ChatID)
	message.MessageKey = strings.TrimSpace(message.MessageKey)
	message.PlatformMessageID = strings.TrimSpace(message.PlatformMessageID)
	message.ReplyToPlatformMessageID = strings.TrimSpace(message.ReplyToPlatformMessageID)
	if session.CookieID == "" || session.ChatID == "" || message.MessageKey == "" {
		return nil, false, errors.New("聊天消息缺少账号、会话或消息键")
	}
	if message.SentAt <= 0 {
		message.SentAt = time.Now().UTC().UnixMilli()
	}
	if message.PlatformMessageID != "" {
		// existing、lookupErr 保存此前已由本地发送响应绑定的同一平台消息。
		existing, lookupErr := s.GetMessageByPlatformID(ctx, session.CookieID, message.PlatformMessageID)
		if lookupErr == nil {
			// merged、mergeErr 保存平台历史对既有本地消息的单调状态补全结果。
			merged, mergeErr := s.mergeExistingMessage(ctx, existing, message)
			return merged, false, mergeErr
		}
		if !errors.Is(lookupErr, ErrNotFound) {
			return nil, false, lookupErr
		}
		if message.Direction == "outgoing" && strings.TrimSpace(message.Content) != "" {
			// pending、pendingErr 查找可能尚未收到 send 响应的同正文本地消息，避免平台回显抢先生成重复行。
			pending, pendingErr := s.findMatchingPendingOutgoing(ctx, session.CookieID, session.ChatID, message.Content, message.ReplyToPlatformMessageID, message.SentAt)
			if pendingErr == nil {
				// bound、bindErr 先把 PNM 绑定到本地幂等行，再合并平台确认状态。
				bound, bindErr := s.BindPlatformMessageID(ctx, session.CookieID, pending.MessageKey, message.PlatformMessageID)
				if bindErr != nil {
					return nil, false, bindErr
				}
				// merged、mergeErr 保存提前回显对本地 sending 状态的单调推进结果。
				merged, mergeErr := s.mergeExistingMessage(ctx, bound, message)
				return merged, false, mergeErr
			}
			if !errors.Is(pendingErr, ErrNotFound) {
				return nil, false, pendingErr
			}
		}
	}
	if message.Status != "recalled" {
		message.RecallOperatorType = -1
	}
	// read_status is also used for incoming messages: only a newly received
	// real-user message starts unread. Imported history and official system
	// notices must never contribute to the chat badge.
	if message.Direction == "incoming" && (!unread || message.MessageType == "system") {
		message.ReadStatus = 2
		message.ReadAt = time.Now().UTC().UnixMilli()
	}
	message.CookieID, message.ChatID = session.CookieID, session.ChatID
	// messageSummary 是会话列表使用的短摘要；结构化消息可避免把内部 JSON 展示在联系人栏。
	messageSummary := strings.TrimSpace(message.Summary)
	if messageSummary == "" {
		messageSummary = message.Content
	}
	// now 用于本次流程后续判断的now
	now := time.Now().UTC().Unix()
	// tx、err 用于本次流程后续判断的tx、err
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	// The composite foreign key on chat_messages requires the session to exist
	// first. Insert an empty shell without touching an existing conversation.
	// sessionPrefix 用于本次流程后续判断的会话Prefix
	sessionPrefix := dialectInsertIgnorePrefix(s.Dialect)
	// sessionInsert 用于本次流程后续判断的会话Insert
	sessionInsert := sessionPrefix + ` INTO chat_sessions
		(cookie_id,chat_id,buyer_id,buyer_name,buyer_avatar_url,item_id,item_title,last_message,last_message_at,unread_count,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)` + dialectInsertIgnore(s.Dialect, []string{"cookie_id", "chat_id"})
	if // err 用于本次流程后续判断的err
	_, err := tx.ExecContext(ctx, sessionInsert, session.CookieID, session.ChatID, session.BuyerID,
		session.BuyerName, session.BuyerAvatar, session.ItemID, session.ItemTitle, "", int64(0), 0, now, now); err != nil {
		return nil, false, fmt.Errorf("建立聊天会话: %w", err)
	}

	// prefix 用于本次流程后续判断的prefix
	prefix := dialectInsertIgnorePrefix(s.Dialect)
	// query 保存带已读字段的幂等插入 SQL，三方言冲突时保持同一列顺序。
	query := prefix + ` INTO chat_messages
		(cookie_id,chat_id,message_key,platform_message_id,reply_to_platform_message_id,direction,sender_id,sender_name,message_type,content,status,read_status,read_at,sent_at,recalled_at,recall_operator_type,recall_operator_id,
		 platform_content_type,system_card_kind,system_card_event,system_card_title,system_card_description,system_card_order_id,system_card_item_id,system_card_action,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)` + dialectInsertIgnore(s.Dialect, []string{"cookie_id", "message_key"})
	// res、err 保存插入结果及执行错误，用于判断是否更新会话摘要。
	res, err := tx.ExecContext(ctx, query, message.CookieID, message.ChatID, message.MessageKey, nullable(message.PlatformMessageID),
		message.ReplyToPlatformMessageID, message.Direction, message.SenderID, message.SenderName, message.MessageType, message.Content,
		message.Status, message.ReadStatus, message.ReadAt, message.SentAt, message.RecalledAt,
		message.RecallOperatorType, message.RecallOperatorID, message.PlatformContentType,
		message.SystemCardKind, message.SystemCardEvent, message.SystemCardTitle, message.SystemCardDescription,
		message.SystemCardOrderID, message.SystemCardItemID, message.SystemCardAction, now)
	if err != nil {
		if message.PlatformMessageID != "" {
			// existing、lookupErr 在并发唯一冲突后读取已经完成绑定的消息。
			if existing, lookupErr := s.GetMessageByPlatformID(ctx, message.CookieID, message.PlatformMessageID); lookupErr == nil {
				// merged、mergeErr 将并发到达的权威状态合并到唯一消息行。
				merged, mergeErr := s.mergeExistingMessage(ctx, existing, message)
				return merged, false, mergeErr
			}
		}
		return nil, false, fmt.Errorf("保存聊天消息: %w", err)
	}
	// inserted 用于本次流程后续判断的inserted
	inserted, _ := res.RowsAffected()
	if inserted > 0 {
		// inheritErr 保存买家后续消息确认此前出站消息已读时的失败；同一会话中的后续回复是已读历史的确定证据。
		if message.Direction == "incoming" && message.MessageType != "system" {
			// inheritErr 表示把后续买家消息作为已读证据回写到此前出站消息时的数据库错误。
			_, inheritErr := tx.ExecContext(ctx, `UPDATE chat_messages SET read_status=2,
				read_at=CASE WHEN read_status=2 AND read_at>0 THEN read_at ELSE ? END
				WHERE cookie_id=? AND chat_id=? AND direction='outgoing' AND sent_at<=?`, message.SentAt, message.CookieID, message.ChatID, message.SentAt)
			if inheritErr != nil {
				return nil, false, fmt.Errorf("按后续消息确认聊天已读: %w", inheritErr)
			}
		}
		// inheritErr 保存平台消息补入历史时继承本地临时消息已读回执的失败，避免同一消息因键不同长期显示未读。
		if message.Direction == "outgoing" && strings.HasSuffix(message.MessageKey, ".PNM") {
			// inheritErr 表示把临时出站消息的已读回执继承到平台补入历史消息时的数据库错误。
			_, inheritErr := tx.ExecContext(ctx, `UPDATE chat_messages AS platform SET read_status=2,read_at=(SELECT local.read_at FROM chat_messages AS local
				WHERE local.cookie_id=platform.cookie_id AND local.chat_id=platform.chat_id AND local.direction='outgoing'
				AND local.message_key NOT LIKE '%.PNM' AND local.content=platform.content AND local.read_status=2
				AND ABS(local.sent_at-platform.sent_at)<=10000 ORDER BY local.read_at DESC LIMIT 1)
				WHERE platform.cookie_id=? AND platform.message_key=? AND platform.read_status<>2 AND EXISTS (SELECT 1 FROM chat_messages AS local
				WHERE local.cookie_id=platform.cookie_id AND local.chat_id=platform.chat_id AND local.direction='outgoing'
				AND local.message_key NOT LIKE '%.PNM' AND local.content=platform.content AND local.read_status=2
				AND ABS(local.sent_at-platform.sent_at)<=10000)`, message.CookieID, message.MessageKey)
			if inheritErr != nil {
				return nil, false, fmt.Errorf("继承聊天已读回执: %w", inheritErr)
			}
		}
		// unreadDelta 用于本次流程后续判断的unreadDelta
		unreadDelta := 0
		if unread {
			unreadDelta = 1
		}
		if // err 用于本次流程后续判断的err
		_, err := tx.ExecContext(ctx, `UPDATE chat_sessions SET buyer_id=COALESCE(NULLIF(?,''),buyer_id),buyer_name=COALESCE(NULLIF(?,''),buyer_name),buyer_avatar_url=COALESCE(NULLIF(?,''),buyer_avatar_url),
			item_id=COALESCE(NULLIF(?,''),item_id),item_title=COALESCE(NULLIF(?,''),item_title),last_message=CASE WHEN last_message_at<=? THEN ? ELSE last_message END,
			last_message_at=CASE WHEN last_message_at<=? THEN ? ELSE last_message_at END,
			unread_count=unread_count+?,updated_at=?
			WHERE cookie_id=? AND chat_id=?`, session.BuyerID, session.BuyerName, session.BuyerAvatar,
			session.ItemID, session.ItemTitle, message.SentAt, messageSummary, message.SentAt, message.SentAt, unreadDelta, now,
			session.CookieID, session.ChatID); err != nil {
			return nil, false, fmt.Errorf("更新聊天会话: %w", err)
		}
	}
	if // err 用于本次流程后续判断的err
	err := tx.Commit(); err != nil {
		return nil, false, err
	}
	// stored、err 用于本次流程后续判断的stored、err
	stored, err := s.GetMessageByKey(ctx, message.CookieID, message.MessageKey)
	if errors.Is(err, ErrNotFound) && message.PlatformMessageID != "" {
		stored, err = s.GetMessageByPlatformID(ctx, message.CookieID, message.PlatformMessageID)
	}
	if err == nil && inserted == 0 {
		// merged、mergeErr 保存相同 message_key 重放时的单调状态更新结果。
		merged, mergeErr := s.mergeExistingMessage(ctx, stored, message)
		return merged, false, mergeErr
	}
	return stored, inserted > 0, err
}

// mergeExistingMessage 将平台历史或跨客户端回显的权威状态单调合并到既有消息行。
func (s *ChatStore) mergeExistingMessage(ctx context.Context, existing *ChatMessage, incoming ChatMessage) (*ChatMessage, error) {
	if existing == nil {
		return nil, ErrNotFound
	}
	// replacementContent 仅在平台仍提供真实正文时补全空占位，撤回后的隐藏正文不得覆盖已知原文。
	replacementContent := strings.TrimSpace(incoming.Content)
	if replacementContent == "[系统消息]" || replacementContent == "[已撤回消息]" {
		replacementContent = ""
	}
	// result、err 保存并发安全的单条状态合并结果；CASE 保证已读和撤回状态不能被旧历史回退。
	result, err := s.DB.ExecContext(ctx, `UPDATE chat_messages SET
		platform_message_id=COALESCE(platform_message_id,NULLIF(?,'')),
		reply_to_platform_message_id=CASE WHEN reply_to_platform_message_id='' AND ?<>'' THEN ? ELSE reply_to_platform_message_id END,
		sender_id=CASE WHEN sender_id='' AND ?<>'' THEN ? ELSE sender_id END,
		sender_name=CASE WHEN sender_name='' AND ?<>'' THEN ? ELSE sender_name END,
		message_type=CASE WHEN ?='location' AND message_type='text' AND content='[位置]' THEN 'location' WHEN message_type='' AND ?<>'' THEN ? ELSE message_type END,
		content=CASE WHEN ?<>'' AND (content='' OR content='[系统消息]' OR content='[已撤回消息]' OR content='[位置]') THEN ? ELSE content END,
		status=CASE WHEN status='recalled' OR ?='recalled' THEN 'recalled' WHEN ?='sent' AND status IN ('sending','failed') THEN 'sent' ELSE status END,
		read_status=CASE WHEN read_status=2 OR ?=2 THEN 2 ELSE read_status END,
		read_at=CASE WHEN read_at>0 THEN read_at WHEN ?=2 AND ?>0 THEN ? ELSE read_at END,
		recalled_at=CASE WHEN recalled_at>0 THEN recalled_at WHEN ?='recalled' AND ?>0 THEN ? ELSE recalled_at END,
		recall_operator_type=CASE WHEN ?='recalled' AND recall_operator_type<0 THEN ? ELSE recall_operator_type END,
		recall_operator_id=CASE WHEN recall_operator_id='' AND ?='recalled' THEN ? ELSE recall_operator_id END,
		platform_content_type=CASE WHEN platform_content_type=0 AND ?>0 THEN ? ELSE platform_content_type END,
		system_card_kind=CASE WHEN system_card_kind='' AND ?<>'' THEN ? ELSE system_card_kind END,
		system_card_event=CASE WHEN system_card_event='' AND ?<>'' THEN ? WHEN system_card_event='unknown_trade_event' AND ?<>'' AND ?<>'unknown_trade_event' THEN ? ELSE system_card_event END,
		system_card_title=CASE WHEN system_card_title='' AND ?<>'' THEN ? ELSE system_card_title END,
		system_card_description=CASE WHEN system_card_description='' AND ?<>'' THEN ? ELSE system_card_description END,
		system_card_order_id=CASE WHEN system_card_order_id='' AND ?<>'' THEN ? ELSE system_card_order_id END,
		system_card_item_id=CASE WHEN system_card_item_id='' AND ?<>'' THEN ? ELSE system_card_item_id END,
		system_card_action=CASE WHEN system_card_action='' AND ?<>'' THEN ? ELSE system_card_action END
		WHERE id=?`, incoming.PlatformMessageID, incoming.ReplyToPlatformMessageID, incoming.ReplyToPlatformMessageID,
		incoming.SenderID, incoming.SenderID, incoming.SenderName, incoming.SenderName,
		incoming.MessageType, incoming.MessageType, incoming.MessageType, replacementContent, replacementContent,
		incoming.Status, incoming.Status, incoming.ReadStatus,
		incoming.ReadStatus, incoming.ReadAt, incoming.ReadAt,
		incoming.Status, incoming.RecalledAt, incoming.RecalledAt,
		incoming.Status, incoming.RecallOperatorType,
		incoming.Status, incoming.RecallOperatorID,
		incoming.PlatformContentType, incoming.PlatformContentType,
		incoming.SystemCardKind, incoming.SystemCardKind,
		incoming.SystemCardEvent, incoming.SystemCardEvent,
		incoming.SystemCardEvent, incoming.SystemCardEvent, incoming.SystemCardEvent,
		incoming.SystemCardTitle, incoming.SystemCardTitle,
		incoming.SystemCardDescription, incoming.SystemCardDescription,
		incoming.SystemCardOrderID, incoming.SystemCardOrderID,
		incoming.SystemCardItemID, incoming.SystemCardItemID,
		incoming.SystemCardAction, incoming.SystemCardAction, existing.ID)
	if err != nil {
		return nil, err
	}
	// affected 保存目标消息是否仍然存在；并发删除时返回统一未找到错误。
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, ErrNotFound
	}
	return s.GetMessageByKey(ctx, existing.CookieID, existing.MessageKey)
}

// findMatchingPendingOutgoing 查找十秒窗口内同正文且同引用目标的本地 sending 消息，避免并发普通文本与引用文本误合并。
func (s *ChatStore) findMatchingPendingOutgoing(ctx context.Context, cookieID, chatID, content, replyToPlatformMessageID string, sentAt int64) (*ChatMessage, error) {
	// message 保存与平台回显最接近的未绑定本地出站消息。
	var message ChatMessage
	// err 保存查询和字段映射错误；没有候选时转换为统一 ErrNotFound。
	err := s.DB.QueryRowContext(ctx, `SELECT id,cookie_id,chat_id,message_key,COALESCE(platform_message_id,''),reply_to_platform_message_id,direction,sender_id,sender_name,message_type,content,status,read_status,read_at,sent_at,recalled_at,recall_operator_type,recall_operator_id,
		platform_content_type,system_card_kind,system_card_event,system_card_title,system_card_description,system_card_order_id,system_card_item_id,system_card_action
		FROM chat_messages WHERE cookie_id=? AND chat_id=? AND direction='outgoing' AND status='sending'
		AND (platform_message_id IS NULL OR platform_message_id='') AND message_key NOT LIKE '%.PNM' AND content=? AND reply_to_platform_message_id=? AND ABS(sent_at-?)<=10000
		ORDER BY ABS(sent_at-?) ASC,id DESC LIMIT 1`, cookieID, chatID, content, replyToPlatformMessageID, sentAt, sentAt).Scan(
		&message.ID, &message.CookieID, &message.ChatID, &message.MessageKey, &message.PlatformMessageID, &message.ReplyToPlatformMessageID,
		&message.Direction, &message.SenderID, &message.SenderName, &message.MessageType, &message.Content,
		&message.Status, &message.ReadStatus, &message.ReadAt, &message.SentAt, &message.RecalledAt,
		&message.RecallOperatorType, &message.RecallOperatorID, &message.PlatformContentType,
		&message.SystemCardKind, &message.SystemCardEvent, &message.SystemCardTitle, &message.SystemCardDescription,
		&message.SystemCardOrderID, &message.SystemCardItemID, &message.SystemCardAction)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &message, err
}

// GetMessageByKey 读取消息ByKey。
func (s *ChatStore) GetMessageByKey(ctx context.Context, cookieID, key string) (*ChatMessage, error) {
	// m 保存按账号和幂等键读取的完整聊天消息。
	var m ChatMessage
	// err 保存查询错误；不存在时转换为仓储统一的 ErrNotFound。
	err := s.DB.QueryRowContext(ctx, `SELECT id,cookie_id,chat_id,message_key,COALESCE(platform_message_id,''),reply_to_platform_message_id,direction,sender_id,sender_name,message_type,content,status,read_status,read_at,sent_at,recalled_at,recall_operator_type,recall_operator_id,
		platform_content_type,system_card_kind,system_card_event,system_card_title,system_card_description,system_card_order_id,system_card_item_id,system_card_action
		FROM chat_messages WHERE cookie_id=? AND message_key=?`, cookieID, key).Scan(
		&m.ID, &m.CookieID, &m.ChatID, &m.MessageKey, &m.PlatformMessageID, &m.ReplyToPlatformMessageID, &m.Direction, &m.SenderID, &m.SenderName,
		&m.MessageType, &m.Content, &m.Status, &m.ReadStatus, &m.ReadAt, &m.SentAt,
		&m.RecalledAt, &m.RecallOperatorType, &m.RecallOperatorID, &m.PlatformContentType,
		&m.SystemCardKind, &m.SystemCardEvent, &m.SystemCardTitle, &m.SystemCardDescription,
		&m.SystemCardOrderID, &m.SystemCardItemID, &m.SystemCardAction)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

// GetOwnedMessageByKey 按用户、账号和本地消息键读取撤回候选，不读取任何账号凭证。
func (s *ChatStore) GetOwnedMessageByKey(ctx context.Context, userID int64, cookieID, key string) (*ChatMessage, error) {
	// message 保存通过账号归属校验的聊天消息。
	var message ChatMessage
	// err 保存读取或字段映射错误。
	err := s.DB.QueryRowContext(ctx, `SELECT m.id,m.cookie_id,m.chat_id,m.message_key,COALESCE(m.platform_message_id,''),m.reply_to_platform_message_id,m.direction,m.sender_id,m.sender_name,m.message_type,m.content,m.status,m.read_status,m.read_at,m.sent_at,m.recalled_at,m.recall_operator_type,m.recall_operator_id,
		m.platform_content_type,m.system_card_kind,m.system_card_event,m.system_card_title,m.system_card_description,m.system_card_order_id,m.system_card_item_id,m.system_card_action
		FROM chat_messages m JOIN cookies c ON c.id=m.cookie_id
		WHERE c.user_id=? AND m.cookie_id=? AND m.message_key=?`, userID, cookieID, key).Scan(
		&message.ID, &message.CookieID, &message.ChatID, &message.MessageKey, &message.PlatformMessageID, &message.ReplyToPlatformMessageID,
		&message.Direction, &message.SenderID, &message.SenderName, &message.MessageType, &message.Content,
		&message.Status, &message.ReadStatus, &message.ReadAt, &message.SentAt, &message.RecalledAt,
		&message.RecallOperatorType, &message.RecallOperatorID, &message.PlatformContentType,
		&message.SystemCardKind, &message.SystemCardEvent, &message.SystemCardTitle, &message.SystemCardDescription,
		&message.SystemCardOrderID, &message.SystemCardItemID, &message.SystemCardAction)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &message, err
}

// GetMessageByPlatformID 按账号和闲鱼 PNM ID 读取消息；不存在时返回 ErrNotFound。
func (s *ChatStore) GetMessageByPlatformID(ctx context.Context, cookieID, platformMessageID string) (*ChatMessage, error) {
	// message 保存平台 ID 对应的完整非凭证聊天记录。
	var message ChatMessage
	// err 保存读取或字段映射错误。
	err := s.DB.QueryRowContext(ctx, `SELECT id,cookie_id,chat_id,message_key,COALESCE(platform_message_id,''),reply_to_platform_message_id,direction,sender_id,sender_name,message_type,content,status,read_status,read_at,sent_at,recalled_at,recall_operator_type,recall_operator_id,
		platform_content_type,system_card_kind,system_card_event,system_card_title,system_card_description,system_card_order_id,system_card_item_id,system_card_action
		FROM chat_messages WHERE cookie_id=? AND platform_message_id=?`, cookieID, platformMessageID).Scan(
		&message.ID, &message.CookieID, &message.ChatID, &message.MessageKey, &message.PlatformMessageID, &message.ReplyToPlatformMessageID,
		&message.Direction, &message.SenderID, &message.SenderName, &message.MessageType, &message.Content,
		&message.Status, &message.ReadStatus, &message.ReadAt, &message.SentAt, &message.RecalledAt,
		&message.RecallOperatorType, &message.RecallOperatorID, &message.PlatformContentType,
		&message.SystemCardKind, &message.SystemCardEvent, &message.SystemCardTitle, &message.SystemCardDescription,
		&message.SystemCardOrderID, &message.SystemCardItemID, &message.SystemCardAction)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &message, err
}

// BindPlatformMessageID 把平台发送响应中的 PNM ID 绑定到既有本地幂等消息。
func (s *ChatStore) BindPlatformMessageID(ctx context.Context, cookieID, key, platformMessageID string) (*ChatMessage, error) {
	// result 保存更新影响行数，用于区分消息不存在与绑定成功。
	result, err := s.DB.ExecContext(ctx, `UPDATE chat_messages SET platform_message_id=?
		WHERE cookie_id=? AND message_key=? AND (platform_message_id IS NULL OR platform_message_id='')`, platformMessageID, cookieID, key)
	if err != nil {
		return nil, err
	}
	// affected 保存实际完成绑定的消息数量。
	affected, _ := result.RowsAffected()
	if affected == 0 {
		// existing、readErr 区分同一 PNM 的幂等重复绑定与消息不存在或冲突。
		existing, readErr := s.GetMessageByKey(ctx, cookieID, key)
		if readErr == nil && existing.PlatformMessageID == platformMessageID {
			return existing, nil
		}
		if readErr != nil {
			return nil, readErr
		}
		return nil, ErrNotFound
	}
	return s.GetMessageByKey(ctx, cookieID, key)
}

// MarkMessageRecalled 按平台消息 ID 保存撤回操作者和时间，并保留原文供发送方重新编辑。
func (s *ChatStore) MarkMessageRecalled(ctx context.Context, cookieID, platformMessageID string, operatorType int, operatorID string, recalledAt int64) (*ChatMessage, error) {
	// result 保存按平台 ID 更新撤回状态的执行结果。
	result, err := s.DB.ExecContext(ctx, `UPDATE chat_messages SET status='recalled',recalled_at=?,recall_operator_type=?,recall_operator_id=?
		WHERE cookie_id=? AND (platform_message_id=? OR message_key=?)`, recalledAt, operatorType, operatorID, cookieID, platformMessageID, platformMessageID)
	if err != nil {
		return nil, err
	}
	// affected 保存本次实际收敛为撤回状态的消息数量。
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, ErrNotFound
	}
	// message 优先按平台 ID 读取；历史消息可能只把 PNM 放在 message_key。
	message, readErr := s.GetMessageByPlatformID(ctx, cookieID, platformMessageID)
	if errors.Is(readErr, ErrNotFound) {
		return s.GetMessageByKey(ctx, cookieID, platformMessageID)
	}
	return message, readErr
}

// UpdateMessageType refreshes the classification of an already persisted
// message when a later history response exposes richer protocol metadata.
// UpdateMessageType 更新消息类型。
func (s *ChatStore) UpdateMessageType(ctx context.Context, cookieID, key, messageType string) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE chat_messages SET message_type=?
		WHERE cookie_id=? AND message_key=?`, messageType, cookieID, key)
	return err
}

// ListSessions 读取Sessions。
func (s *ChatStore) ListSessions(ctx context.Context, userID int64, cookieID string, limit int) ([]ChatSession, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := s.DB.QueryContext(ctx, `SELECT cs.cookie_id,cs.chat_id,cs.buyer_id,cs.buyer_name,cs.buyer_avatar_url,
		cs.item_id,cs.item_title,cs.last_message,cs.last_message_at,cs.unread_count,cs.is_pinned,cs.pinned_at
		FROM chat_sessions cs JOIN cookies c ON c.id=cs.cookie_id
		WHERE c.user_id=? AND cs.cookie_id=?
		ORDER BY cs.is_pinned DESC,cs.pinned_at DESC,cs.last_message_at DESC,cs.chat_id DESC LIMIT ?`, userID, cookieID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// result 用于本次流程后续判断的结果
	var result []ChatSession
	for rows.Next() {
		// row 用于本次流程后续判断的row
		var row ChatSession
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&row.CookieID, &row.ChatID, &row.BuyerID, &row.BuyerName, &row.BuyerAvatar,
			&row.ItemID, &row.ItemTitle, &row.LastMessage, &row.LastMessageAt, &row.UnreadCount, &row.IsPinned, &row.PinnedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// SetSessionPinned 按用户归属幂等设置精确会话置顶状态；置顶时仅在原状态为未置顶时更新置顶时间，取消时时间归零。
func (s *ChatStore) SetSessionPinned(ctx context.Context, userID int64, cookieID, chatID string, pinned bool, pinnedAt int64) error {
	if s == nil || s.DB == nil || userID <= 0 || strings.TrimSpace(cookieID) == "" || strings.TrimSpace(chatID) == "" {
		return ErrNotFound
	}
	if pinnedAt < 0 {
		pinnedAt = 0
	}
	// updateErr 是按账号归属和会话复合主键执行幂等更新的数据库错误。
	_, updateErr := s.DB.ExecContext(ctx, `UPDATE chat_sessions SET
		pinned_at=CASE WHEN ? AND is_pinned=FALSE THEN ? WHEN NOT ? THEN 0 ELSE pinned_at END,
		is_pinned=?,updated_at=?
		WHERE cookie_id=? AND chat_id=?
		AND EXISTS(SELECT 1 FROM cookies c WHERE c.id=chat_sessions.cookie_id AND c.user_id=?)`,
		pinned, pinnedAt, pinned, pinned, time.Now().UTC().Unix(), strings.TrimSpace(cookieID), strings.TrimSpace(chatID), userID)
	if updateErr != nil {
		return updateErr
	}
	// exists 只确认精确会话归属且存在，不读取买家或账号敏感字段。
	var exists int
	// readErr 是更新后复核归属会话是否存在的查询结果。
	readErr := s.DB.QueryRowContext(ctx, `SELECT 1 FROM chat_sessions cs JOIN cookies c ON c.id=cs.cookie_id
		WHERE c.user_id=? AND cs.cookie_id=? AND cs.chat_id=?`, userID, strings.TrimSpace(cookieID), strings.TrimSpace(chatID)).Scan(&exists)
	if errors.Is(readErr, sql.ErrNoRows) {
		return ErrNotFound
	}
	return readErr
}

// ListMessages 读取消息列表。
func (s *ChatStore) ListMessages(ctx context.Context, userID int64, cookieID, chatID string, beforeID int64, limit int) ([]ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// query 保存按时间倒序读取再反转为时间正序的分页 SQL。
	query := `SELECT m.id,m.cookie_id,m.chat_id,m.message_key,COALESCE(m.platform_message_id,''),m.reply_to_platform_message_id,m.direction,m.sender_id,m.sender_name,m.message_type,m.content,m.status,m.read_status,m.read_at,m.sent_at,m.recalled_at,m.recall_operator_type,m.recall_operator_id,
		m.platform_content_type,m.system_card_kind,
		CASE WHEN close_run.id IS NOT NULL AND m.system_card_event IN ('order_pending_payment','order_paid') THEN 'order_closed' ELSE m.system_card_event END,
		CASE WHEN close_run.id IS NOT NULL AND m.system_card_event IN ('order_pending_payment','order_paid') THEN '订单已取消' ELSE m.system_card_title END,
		CASE WHEN close_run.id IS NOT NULL AND m.system_card_event='order_paid' THEN '本地已确认取消成功，买家款项将由闲鱼原路退回'
		     WHEN close_run.id IS NOT NULL AND m.system_card_event='order_pending_payment' THEN '本地已确认取消成功，买家无法继续付款'
		     ELSE m.system_card_description END,
		m.system_card_order_id,m.system_card_item_id,
		CASE WHEN close_run.id IS NOT NULL AND m.system_card_event IN ('order_pending_payment','order_paid') THEN '' ELSE m.system_card_action END,
		COALESCE(reply_target.platform_message_id,''),COALESCE(reply_target.direction,''),COALESCE(reply_target.sender_id,''),COALESCE(reply_target.sender_name,''),
		COALESCE(reply_target.message_type,''),COALESCE(reply_target.content,''),COALESCE(reply_target.status,'')
		FROM chat_messages m JOIN cookies c ON c.id=m.cookie_id
		LEFT JOIN account_task_runs close_run ON close_run.cookie_id=m.cookie_id
		  AND close_run.task_type='order_close' AND close_run.target_id=m.system_card_order_id AND close_run.status='success'
		LEFT JOIN chat_messages reply_target ON reply_target.cookie_id=m.cookie_id AND reply_target.chat_id=m.chat_id
		  AND reply_target.platform_message_id=m.reply_to_platform_message_id
		WHERE c.user_id=? AND m.cookie_id=? AND m.chat_id=?`
	// args 用于本次流程后续判断的args
	args := []any{userID, cookieID, chatID}
	if beforeID > 0 {
		query += ` AND (m.sent_at < COALESCE((SELECT older.sent_at FROM chat_messages older WHERE older.id=? AND older.cookie_id=?), m.sent_at)
			OR (m.sent_at = COALESCE((SELECT same.sent_at FROM chat_messages same WHERE same.id=? AND same.cookie_id=?), m.sent_at) AND m.id<?))`
		args = append(args, beforeID, cookieID, beforeID, cookieID, beforeID)
	}
	query += ` ORDER BY m.sent_at DESC,m.id DESC LIMIT ?`
	args = append(args, limit)
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// result 用于本次流程后续判断的结果
	var result []ChatMessage
	for rows.Next() {
		// m 保存当前扫描出的消息及其本地已读状态。
		var m ChatMessage
		// replyPlatformID、replyDirection、replySenderID、replySenderName、replyType、replyContent、replyStatus 是同会话目标快照的可空扫描值。
		var replyPlatformID, replyDirection, replySenderID, replySenderName, replyType, replyContent, replyStatus string
		// err 保存当前行字段映射错误，避免返回缺少已读字段的不完整消息。
		if err := rows.Scan(&m.ID, &m.CookieID, &m.ChatID, &m.MessageKey, &m.PlatformMessageID, &m.ReplyToPlatformMessageID, &m.Direction, &m.SenderID,
			&m.SenderName, &m.MessageType, &m.Content, &m.Status, &m.ReadStatus, &m.ReadAt, &m.SentAt,
			&m.RecalledAt, &m.RecallOperatorType, &m.RecallOperatorID, &m.PlatformContentType,
			&m.SystemCardKind, &m.SystemCardEvent, &m.SystemCardTitle, &m.SystemCardDescription,
			&m.SystemCardOrderID, &m.SystemCardItemID, &m.SystemCardAction,
			&replyPlatformID, &replyDirection, &replySenderID, &replySenderName, &replyType, &replyContent, &replyStatus); err != nil {
			return nil, err
		}
		if replyPlatformID != "" {
			m.ReplyPreview = &ChatReplyPreview{PlatformMessageID: replyPlatformID, Direction: replyDirection, SenderID: replySenderID,
				SenderName: replySenderName, MessageType: replyType, Content: replyContent, Status: replyStatus}
		}
		result = append(result, m)
	}
	// API returns chronological order while the query remains index-friendly.
	for // i、j 用于本次流程后续判断的i、j
	i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, rows.Err()
}

// MarkRead 仅对当前用户拥有账号的非系统入站消息标记已读，并同步归零会话红点。
func (s *ChatStore) MarkRead(ctx context.Context, userID int64, cookieID, chatID string) error {
	// now 是同一批消息和会话状态使用的统一 UTC 时间，避免页面显示先后矛盾。
	now := time.Now().UTC()
	// err 保存批量更新非系统入站消息的错误；失败时不得清空会话红点以避免状态不一致。
	if _, err := s.DB.ExecContext(ctx, `UPDATE chat_messages SET read_status=2,read_at=?
		WHERE cookie_id=? AND chat_id=? AND direction='incoming' AND message_type<>'system' AND read_status<>2`,
		now.UnixMilli(), cookieID, chatID); err != nil {
		return err
	}
	// err 保存归零会话红点的错误，该更新通过用户归属子查询阻止越权修改。
	_, err := s.DB.ExecContext(ctx, `UPDATE chat_sessions SET unread_count=0,updated_at=?
		WHERE cookie_id=? AND chat_id=? AND EXISTS(SELECT 1 FROM cookies c WHERE c.id=chat_sessions.cookie_id AND c.user_id=?)`,
		now.Unix(), cookieID, chatID, userID)
	return err
}

// CountUnreadUserMessages 返回界面红点使用的入站真实用户未读数，系统消息永不计入。
func (s *ChatStore) CountUnreadUserMessages(ctx context.Context, cookieID, chatID string) (int, error) {
	// count 保存符合当前账号、会话及未读条件的消息总数。
	var count int
	// err 保存聚合查询错误，调用方可在平台响应缺失时退回官方红点值。
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_messages
		WHERE cookie_id=? AND chat_id=? AND direction='incoming' AND message_type<>'system' AND read_status<>2`, cookieID, chatID).Scan(&count)
	return count, err
}

// LatestReadState 返回会话最新消息的时间和已读状态，不加载消息正文。
func (s *ChatStore) LatestReadState(ctx context.Context, cookieID, chatID string) (ChatLatestReadState, error) {
	// state 保存最新消息的最小已读状态。
	var state ChatLatestReadState
	// queryErr 是按平台时间和本地主键读取最新消息的错误。
	queryErr := s.DB.QueryRowContext(ctx, `SELECT sent_at,read_status FROM chat_messages WHERE cookie_id=? AND chat_id=? ORDER BY sent_at DESC,id DESC LIMIT 1`, cookieID, chatID).Scan(&state.SentAt, &state.ReadStatus)
	return state, queryErr
}

// UpdateMessageStatus 更新消息状态。
func (s *ChatStore) UpdateMessageStatus(ctx context.Context, cookieID, key, status string) (*ChatMessage, error) {
	if // err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE chat_messages SET status=? WHERE cookie_id=? AND message_key=?`, status, cookieID, key); err != nil {
		return nil, err
	}
	return s.GetMessageByKey(ctx, cookieID, key)
}

// MarkMessageRead 按平台回执把目标出站消息及同会话中更早的出站消息标记为已读，并返回目标消息。
func (s *ChatStore) MarkMessageRead(ctx context.Context, cookieID, key string, readAt int64) (*ChatMessage, error) {
	// message 保存平台回执对应的出站消息；其会话和发送时间界定本次批量确认范围。
	message, err := s.GetMessageByKey(ctx, cookieID, key)
	if errors.Is(err, ErrNotFound) && strings.HasSuffix(strings.TrimSpace(key), ".PNM") {
		message, err = s.GetMessageByPlatformID(ctx, cookieID, key)
	}
	if err != nil {
		return nil, err
	}
	if message.Direction != "outgoing" {
		return nil, ErrNotFound
	}
	// readAt 保存平台已读时间；缺失时使用本机 UTC 时间作为展示回退。
	if readAt <= 0 {
		readAt = time.Now().UTC().UnixMilli()
	}
	// err 保存按回执水位更新同会话出站历史的错误；对方读到目标消息时更早消息也已被阅读。
	if _, err = s.DB.ExecContext(ctx, `UPDATE chat_messages SET read_status=2,
		read_at=CASE WHEN read_status=2 AND read_at>0 THEN read_at ELSE ? END
		WHERE cookie_id=? AND chat_id=? AND direction='outgoing' AND sent_at<=?`, readAt, cookieID, message.ChatID, message.SentAt); err != nil {
		return nil, err
	}
	return s.GetMessageByKey(ctx, cookieID, key)
}

// MarkLatestOutgoingRead 在回执未带消息键时回退标记会话中最近待确认的出站消息。
func (s *ChatStore) MarkLatestOutgoingRead(ctx context.Context, cookieID, chatID string, readAt int64) (*ChatMessage, error) {
	// readAt 保存平台已读时间；缺失时使用本机 UTC 时间作为展示回退。
	if readAt <= 0 {
		readAt = time.Now().UTC().UnixMilli()
	}
	// key 保存最近一条已发送且未标记已读的消息幂等键。
	var key string
	// err 保存查询错误；没有可更新消息时返回统一 ErrNotFound。
	err := s.DB.QueryRowContext(ctx, `SELECT message_key FROM chat_messages WHERE cookie_id=? AND chat_id=? AND direction='outgoing' AND status='sent' AND read_status<>2 ORDER BY sent_at DESC,id DESC LIMIT 1`, cookieID, chatID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.MarkMessageRead(ctx, cookieID, key, readAt)
}
