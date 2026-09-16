package chat

import (
	"context"
	"encoding/base64"
	"testing"
)

// TestExtractReplyMessageIDSupportsOuterAndExtJSON 验证实时扩展外层和 extJson 内层都能提取同一 PNM 引用目标。
func TestExtractReplyMessageIDSupportsOuterAndExtJSON(t *testing.T) {
	// outer 是真实入站结构中直接携带 replyMessageId 的消息扩展对象。
	outer := map[string]any{"10": map[string]any{"replyMessageId": "outer-target.PNM", "extJson": `{"replyMessageId":"outer-target.PNM"}`}}
	if // target 是从外层扩展对象提取的 PNM 引用目标。
	target := extractReplyMessageID(outer); target != "outer-target.PNM" {
		t.Fatalf("outer target=%q", target)
	}
	// nested 是只在 extJson 内携带引用目标的历史兼容对象。
	nested := map[string]any{"extJson": `{"replyMessageId":"nested-target.PNM"}`}
	if // target 是从 extJson 内层提取的 PNM 引用目标。
	target := extractReplyMessageID(nested); target != "nested-target.PNM" {
		t.Fatalf("nested target=%q", target)
	}
	if // target 是无 PNM 后缀时必须返回的空引用目标。
	target := extractReplyMessageID(map[string]any{"replyMessageId": "unsafe-target"}); target != "" {
		t.Fatalf("invalid target=%q", target)
	}
}

// TestParseHistoryMessagePreservesReplyTarget 验证历史文本解码时把扩展中的 PNM 引用关系带入持久化模型。
func TestParseHistoryMessagePreservesReplyTarget(t *testing.T) {
	// encoded 是历史 API custom.data 使用的 contentType=1 base64 正文。
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"contentType":1,"text":{"text":"已回复"}}`))
	// model 是仅包含引用关系和文本所需字段的历史消息夹具。
	model := map[string]any{"message": map[string]any{
		"messageId": "reply-message.PNM", "createAt": int64(1000),
		"extension": map[string]any{"senderUserId": "buyer@goofish", "reminderTitle": "买家", "extJson": `{"replyMessageId":"history-target.PNM"}`},
		"content":   map[string]any{"custom": map[string]any{"data": encoded}},
	}}
	// message、ok 是规范化后的历史消息和解析成功标记。
	message, ok := parseHistoryMessage("account-1", "chat-1", "self", model)
	if !ok || message.Content != "已回复" || message.ReplyToPlatformMessageID != "history-target.PNM" {
		t.Fatalf("message=%+v ok=%v", message, ok)
	}
}

// TestRecordIncomingPersistsReplyTarget 验证实时入站扩展中的 PNM 引用目标通过聊天领域服务持久化。
func TestRecordIncomingPersistsReplyTarget(t *testing.T) {
	// store、cleanup 是已创建脱敏测试账号的隔离聊天存储和关闭函数。
	store, cleanup := chatTestStore(t)
	defer cleanup()
	// service 是使用隔离存储的聊天领域服务。
	service := New(store)
	// incoming 是在实时扩展外层和 extJson 中双写引用目标的文本消息。
	incoming := Incoming{AccountID: "account-1", ChatID: "reply-chat", BuyerID: "buyer-1", BuyerName: "买家", Text: "实时回复", MessageID: "incoming-reply.PNM", Raw: map[string]any{
		"messageId": "incoming-reply.PNM", "sendTime": int64(1000),
		"10": map[string]any{"replyMessageId": "live-target.PNM", "extJson": `{"replyMessageId":"live-target.PNM"}`},
	}}
	// stored、inserted、err 是实时消息持久化结果、首次插入标记和错误。
	stored, inserted, err := service.RecordIncoming(context.Background(), incoming)
	if err != nil || !inserted || stored.ReplyToPlatformMessageID != "live-target.PNM" {
		t.Fatalf("stored=%+v inserted=%v err=%v", stored, inserted, err)
	}
}
