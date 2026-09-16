package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xianyu-go/internal/db"
)

// OutgoingEcho 描述当前账号从其他官方客户端发出后经 WebSocket 回传的非敏感消息。
type OutgoingEcho struct {
	// AccountID 是拥有该平台连接的本地账号标识。
	AccountID string
	// ChatID 是消息所属闲鱼单聊会话标识。
	ChatID string
	// SenderID 是当前闲鱼账号的平台用户标识。
	SenderID string
	// SenderName 是官方回显携带的发送方展示名。
	SenderName string
	// Text 是回显提供的可展示文本或媒体摘要。
	Text string
	// MessageID 是已由平台分配的 PNM 幂等标识。
	MessageID string
	// ItemID 是回显链接中可解析出的商品标识。
	ItemID string
	// Raw 保存本次消息的解密协议对象，仅用于内容和时间规范化。
	Raw map[string]any
}

// RecordOutgoingEcho 将其他官方客户端发送的己方回显实时保存为出站消息，不进入自动回复链。
func (s *Service) RecordOutgoingEcho(ctx context.Context, echo OutgoingEcho) (*db.ChatMessage, bool, error) {
	if s == nil || s.repository == nil {
		return nil, false, fmt.Errorf("聊天服务未初始化")
	}
	// sentAt 是平台消息创建时间；协议缺失时以本机接收时间保持消息可排序。
	sentAt := extractUnixMilli(echo.Raw)
	if sentAt == 0 {
		sentAt = time.Now().UTC().UnixMilli()
	}
	// key 优先使用 Engine 已解析的 PNM ID，并兼容原始对象中的稳定消息标识。
	key := strings.TrimSpace(echo.MessageID)
	if key == "" {
		key = extractString(echo.Raw, "messageId", "message_id", "msgId", "mid", "uuid")
	}
	if key == "" {
		// raw 保存生成稳定降级键所需的协议序列化结果。
		raw, _ := json.Marshal(echo.Raw)
		// digest 是账号、会话、正文和原始载荷组成的稳定摘要，避免无 PNM 回显重复入库。
		digest := sha256.Sum256([]byte(echo.AccountID + "\x00" + echo.ChatID + "\x00" + echo.Text + "\x00" + string(raw)))
		key = "out-" + hex.EncodeToString(digest[:16])
	}
	// messageType、content 是从官方自定义消息载荷规范化出的展示类型和正文。
	messageType, content := extractMessageContent(echo.Raw, echo.Text)
	// platformMessageID 仅接受平台 PNM；降级本地键不能用于撤回或已读匹配。
	platformMessageID := ""
	if strings.HasSuffix(key, ".PNM") {
		platformMessageID = key
	}
	// session 只提供回显中确定的会话和商品字段；存储层必须保留既有买家身份。
	session := db.ChatSession{CookieID: echo.AccountID, ChatID: echo.ChatID, ItemID: echo.ItemID, ItemTitle: extractString(echo.Raw, "itemTitle", "title")}
	// message 是不会增加未读数的跨平台己方出站消息。
	message := db.ChatMessage{MessageKey: key, PlatformMessageID: platformMessageID, Direction: "outgoing",
		SenderID: echo.SenderID, SenderName: echo.SenderName, MessageType: messageType, Content: content, Status: "sent", SentAt: sentAt,
		ReplyToPlatformMessageID: extractReplyMessageID(echo.Raw)}
	if messageType == "location" {
		message.PlatformContentType = 30
		message.Summary = locationCardSummary(content)
	}
	// stored、inserted、err 保存幂等写入和历史状态合并后的结果。
	stored, inserted, err := s.repository.SaveMessage(ctx, session, message, false)
	if err == nil {
		// eventType 区分首次跨平台消息和对既有 ERP 本地消息的平台 ID/内容补全。
		eventType := "message.updated"
		if inserted {
			eventType = "message.created"
		}
		s.Publish(echo.AccountID, Event{Type: eventType, Message: stored, Session: &session})
	}
	return stored, inserted, err
}
