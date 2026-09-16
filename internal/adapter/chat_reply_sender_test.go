package adapter

import (
	"context"
	"testing"

	"xianyu-go/internal/engine"
)

// replySenderStub 模拟账号运行时的普通发送和原生引用发送能力。
type replySenderStub struct {
	// replyTarget 保存适配器透传到运行时的引用 PNM ID。
	replyTarget string
}

// SendText 实现自动化发送基础接口，本测试不调用。
func (s *replySenderStub) SendText(context.Context, string, string, string) error { return nil }

// SendImage 实现自动化图片发送基础接口，本测试不调用。
func (s *replySenderStub) SendImage(context.Context, string, string, string, int64, int, int) error {
	return nil
}

// UpdateCookie 实现运行时凭证协调接口，本测试不保存任何凭证。
func (s *replySenderStub) UpdateCookie(string) {}

// SendReplyTextWithResult 记录原生引用目标并返回非敏感平台结果。
func (s *replySenderStub) SendReplyTextWithResult(_ context.Context, _, _, _, replyToPlatformMessageID string) (engine.PlatformSendResult, error) {
	s.replyTarget = replyToPlatformMessageID
	return engine.PlatformSendResult{PlatformMessageID: "reply-result.PNM", CreatedAt: 1000}, nil
}

// TestChatSenderRoutesNativeReplyWithoutPlainTextFallback 验证适配器只调用引用专用运行时方法，并透传精确目标 PNM ID。
func TestChatSenderRoutesNativeReplyWithoutPlainTextFallback(t *testing.T) {
	// runtime 是记录原生引用目标的账号运行时替身。
	runtime := &replySenderStub{}
	// sender 是应用层 Sender 与账号运行时之间的精确适配器。
	sender := chatSender{sender: runtime}
	// result、err 是适配后返回的平台消息标识和错误。
	result, err := sender.SendReplyText(context.Background(), "chat-1", "buyer-1", "回复", "target-1.PNM", "local-1")
	if err != nil || result.PlatformMessageID != "reply-result.PNM" || runtime.replyTarget != "target-1.PNM" {
		t.Fatalf("result=%+v target=%q err=%v", result, runtime.replyTarget, err)
	}
}
