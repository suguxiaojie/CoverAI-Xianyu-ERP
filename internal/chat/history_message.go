package chat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"xianyu-go/internal/db"
)

// parseHistoryMessage 将平台历史包装模型转换为可单调合并的聊天消息状态。
func parseHistoryMessage(accountID, chatID, myID string, model map[string]any) (db.ChatMessage, bool) {
	// message 是历史包装模型中的消息正文对象。
	message, _ := model["message"].(map[string]any)
	if message == nil {
		return db.ChatMessage{}, false
	}
	// extension 保存发送者和展示摘要等非敏感扩展字段。
	extension := mapValue(message["extension"])
	// senderID 是去除平台域后缀后的消息发送者标识。
	senderID := strings.Split(strings.TrimSpace(fmt.Sprint(extension["senderUserId"])), "@")[0]
	// senderName 是历史消息携带的发送者展示名称。
	senderName := strings.TrimSpace(fmt.Sprint(extension["reminderTitle"]))
	if senderName == "<nil>" {
		senderName = ""
	}
	// key 是历史消息的稳定 PNM 标识。
	key := strings.TrimSpace(fmt.Sprint(message["messageId"]))
	if key == "" || key == "<nil>" {
		return db.ChatMessage{}, false
	}
	// contentMap 保存官方自定义消息的外层载荷。
	contentMap, _ := message["content"].(map[string]any)
	// custom 保存官方自定义消息的正文编码和摘要字段。
	custom, _ := contentMap["custom"].(map[string]any)
	// rawContent 保存 base64 解码后的实际文本或媒体协议对象。
	rawContent := map[string]any{}
	if // encoded 是去空白后的 base64 正文；空值或协议 nil 文本直接走摘要降级。
	encoded := strings.TrimSpace(fmt.Sprint(custom["data"])); encoded != "" && encoded != "<nil>" {
		// decoded、err 保存正文解码结果；损坏正文保留空对象并继续使用摘要降级。
		if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
			_ = json.Unmarshal(decoded, &rawContent)
		}
	}
	// fallback 是正文协议缺失时使用的官方摘要。
	fallback := strings.TrimSpace(fmt.Sprint(custom["summary"]))
	if fallback == "<nil>" {
		fallback = ""
	}
	if // textBlock 是解码正文中的文本对象；ok 表示当前消息确实采用文本结构。
	textBlock, ok := rawContent["text"].(map[string]any); ok {
		// text 是正文协议中优先级高于摘要的实际文本。
		if text := strings.TrimSpace(fmt.Sprint(textBlock["text"])); text != "" && text != "<nil>" {
			fallback = text
		}
	}
	// messageType、content 是规范化后的消息类别和展示内容。
	messageType, content := extractMessageContent(rawContent, fallback)
	if isOfficialSystemMessage(rawContent, senderID, fallback) {
		messageType = "system"
		if senderID == "1400" {
			senderName = "闲小蜜"
		}
	}
	// direction、status 是按发送者身份确定的消息方向和初始投递状态。
	direction, status := "incoming", "received"
	if senderID != "" && senderID == strings.TrimSpace(myID) {
		direction, status = "outgoing", "sent"
	}
	// recallFeature 优先读取真实历史模型顶层字段，并兼容旧响应把它放入 message 内层。
	recallFeature := mapValue(model["recallFeature"])
	if len(recallFeature) == 0 {
		recallFeature = mapValue(message["recallFeature"])
	}
	// messageStatus 是历史包装层的消息状态；顶层缺失时兼容旧内层字段。
	messageStatus := int64Value(model["msgStatus"])
	if messageStatus == 0 {
		messageStatus = int64Value(model["messageStatus"])
	}
	if messageStatus == 0 {
		messageStatus = int64Value(message["msgStatus"])
	}
	if messageStatus == 0 {
		messageStatus = int64Value(message["messageStatus"])
	}
	// readStatus 是平台历史确认的对方阅读状态；值 2 只能推进本地状态，不能用于回退。
	readStatus := int(int64Value(model["readStatus"]))
	if readStatus == 0 {
		readStatus = int(int64Value(message["readStatus"]))
	}
	// modifiedAt 是平台历史包装层记录的最近状态时间，缺失时兼容内层字段。
	modifiedAt := int64Value(model["modifyTime"])
	if modifiedAt <= 0 {
		modifiedAt = int64Value(message["modifyTime"])
	}
	// recallOperatorType 保存撤回操作者类型；普通消息使用 -1。
	recallOperatorType := -1
	// recalledAt 保存历史消息确认的撤回时间，优先使用修改时间并回退消息创建时间。
	var recalledAt int64
	if messageStatus == 2 {
		status = "recalled"
		recallOperatorType = int(int64Value(recallFeature["operatorType"]))
		recalledAt = modifiedAt
		if recalledAt <= 0 {
			recalledAt = int64Value(message["createAt"])
		}
	}
	if content == "" {
		if status == "recalled" {
			content = "[已撤回消息]"
		} else {
			content = "[系统消息]"
		}
	}
	// readAt 在历史接口没有单独阅读时间时使用可用的修改时间；零值明确表示平台未提供精确时间。
	readAt := int64(0)
	if readStatus == 2 {
		readAt = modifiedAt
	}
	// result 是最终返回并参与重复消息单调合并的历史消息。
	result := db.ChatMessage{CookieID: accountID, ChatID: chatID, MessageKey: key, PlatformMessageID: key, Direction: direction,
		SenderID: senderID, SenderName: senderName, MessageType: messageType, Content: content,
		Status: status, ReadStatus: readStatus, ReadAt: readAt, SentAt: int64Value(message["createAt"]), RecalledAt: recalledAt,
		RecallOperatorType: recallOperatorType, RecallOperatorID: cleanNilString(recallFeature["operatorUid"]),
		ReplyToPlatformMessageID: extractReplyMessageID(extension)}
	if messageType == "location" {
		result.PlatformContentType = 30
		result.Summary = locationCardSummary(content)
	}
	if // card、ok 是历史 custom.data 中的结构化交易卡片及识别结果。
	card, ok := parseSystemCard(rawContent, fallback); ok {
		applySystemCard(&result, card)
		result.MessageType = "system"
		result.Content = card.Title
	}
	return result, true
}
