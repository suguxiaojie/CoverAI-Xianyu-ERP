package ws

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// TestBuildChatSendBodyWithReplyWritesObservedDualFields 验证原生引用目标同时位于扩展对象和 extJson，文本仍保持 contentType=1。
func TestBuildChatSendBodyWithReplyWritesObservedDualFields(t *testing.T) {
	// content 是与普通文本完全一致的内层协议载荷。
	content := map[string]any{"contentType": 1, "text": map[string]any{"text": "引用回复"}}
	// body、err 是构造后的 sendByReceiverScope 参数和本地校验错误。
	body, err := buildChatSendBodyWithReply("self", "chat-1", "buyer", content, "target-1.PNM")
	if err != nil || len(body) != 2 {
		t.Fatalf("body=%+v err=%v", body, err)
	}
	// message 是请求体中的官方消息模型。
	message, ok := body[0].(map[string]any)
	if !ok {
		t.Fatalf("message=%T", body[0])
	}
	// extension 是必须双写原生引用目标的扩展对象。
	extension, ok := message["extension"].(map[string]any)
	if !ok || extension["replyMessageId"] != "target-1.PNM" {
		t.Fatalf("extension=%+v", message["extension"])
	}
	// extJSON 是扩展对象内与外层值一致的紧凑 JSON。
	var extJSON map[string]string
	if // unmarshalErr 是解码 extJson 内层引用字段的错误。
	unmarshalErr := json.Unmarshal([]byte(extension["extJson"].(string)), &extJSON); unmarshalErr != nil || extJSON["replyMessageId"] != "target-1.PNM" {
		t.Fatalf("ext_json=%+v err=%v", extJSON, unmarshalErr)
	}
	// outerContent 是包裹 base64 自定义数据的 contentType=101 外层对象。
	outerContent := message["content"].(map[string]any)
	// custom 是包含文本协议 base64 数据的自定义对象。
	custom := outerContent["custom"].(map[string]any)
	// decoded 是解码后不应因引用关系而改变的文本载荷。
	decoded, decodeErr := base64.StdEncoding.DecodeString(custom["data"].(string))
	if decodeErr != nil || string(decoded) != `{"contentType":1,"text":{"text":"引用回复"}}` || outerContent["contentType"] != 101 {
		t.Fatalf("decoded=%s outer=%+v err=%v", decoded, outerContent, decodeErr)
	}
}

// TestBuildChatSendBodyWithReplyRejectsNonPNMTarget 验证平台目标缺少 PNM 后缀时在发送前本地拒绝。
func TestBuildChatSendBodyWithReplyRejectsNonPNMTarget(t *testing.T) {
	// body、err 是非法引用目标的空请求体和校验错误。
	body, err := buildChatSendBodyWithReply("self", "chat-1", "buyer", map[string]any{"contentType": 1}, "not-platform-id")
	if err == nil || body != nil {
		t.Fatalf("body=%+v err=%v", body, err)
	}
}
