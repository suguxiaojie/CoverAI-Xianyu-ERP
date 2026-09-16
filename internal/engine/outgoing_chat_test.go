package engine

import (
	"context"
	"testing"

	"xianyu-go/internal/automation"
	wsprotocol "xianyu-go/internal/xianyu/ws"
)

// outgoingObserverHandler 用于本次流程后续判断的outgoingObserverHandler
type outgoingObserverHandler struct {
	messages []OutgoingChatMessage
}

// replyWSConn 在基础测试连接上增加带结果的原生引用发送能力。
type replyWSConn struct {
	fakeWSConn
	// replyTarget 保存运行时传递到 WebSocket 层的引用 PNM ID。
	replyTarget string
}

// imageResultWSConn 在基础连接上增加带平台 PNM 结果的图片发送能力。
type imageResultWSConn struct {
	fakeWSConn
}

// locationResultWSConn 记录位置卡片字段并返回固定平台结果。
type locationResultWSConn struct {
	fakeWSConn
	// title、description 是运行时透传到协议层的卡片文案。
	title, description string
	// latitude、longitude 是运行时透传到协议层的 WGS84 坐标。
	latitude, longitude float64
}

// SendLocationCardWithResult 记录位置字段并模拟平台明确成功。
func (c *locationResultWSConn) SendLocationCardWithResult(_ context.Context, _, _, _, title, description string, latitude, longitude float64) (wsprotocol.SendResult, error) {
	c.title, c.description, c.latitude, c.longitude = title, description, latitude, longitude
	return wsprotocol.SendResult{PlatformMessageID: "location-result.PNM", CreatedAt: 3000}, nil
}

// SendImageWithResult 返回固定平台图片消息标识和时间，模拟生产 WebSocket 成功结果。
func (c *imageResultWSConn) SendImageWithResult(_ context.Context, _, _, _, _ string, _, _ int) (wsprotocol.SendResult, error) {
	return wsprotocol.SendResult{PlatformMessageID: "image-result.PNM", CreatedAt: 2000}, nil
}

// SendReplyTextWithResult 记录原生引用目标，并返回模拟平台分配的新 PNM ID。
func (c *replyWSConn) SendReplyTextWithResult(_ context.Context, _, _, _, _, replyToPlatformMessageID string) (wsprotocol.SendResult, error) {
	c.replyTarget = replyToPlatformMessageID
	return wsprotocol.SendResult{PlatformMessageID: "reply-result.PNM", CreatedAt: 1000}, nil
}

// HandleChatMessage 处理聊天消息。
func (h *outgoingObserverHandler) HandleChatMessage(context.Context, ChatMessage) error { return nil }

// HandleSystemEvent 处理系统Event。
func (h *outgoingObserverHandler) HandleSystemEvent(context.Context, automation.Task) error {
	return nil
}

// OnPasswordLoginRefresh 封装On密码登录Refresh业务协调。
func (h *outgoingObserverHandler) OnPasswordLoginRefresh(context.Context, string) bool { return false }

// OnAccountAlert 封装On账号Alert业务协调。
func (h *outgoingObserverHandler) OnAccountAlert(context.Context, string, string, string, string) {}

// HandleOutgoingChatMessage 处理Outgoing聊天消息。
func (h *outgoingObserverHandler) HandleOutgoingChatMessage(_ context.Context, message OutgoingChatMessage) error {
	h.messages = append(h.messages, message)
	return nil
}

// TestSendTextEmitsCorrelatedOutgoingObservation 封装TestSend文本EmitsCorrelatedOutgoingObservation业务协调。
func TestSendTextEmitsCorrelatedOutgoingObservation(t *testing.T) {
	// handler 用于本次流程后续判断的handler
	handler := &outgoingObserverHandler{}
	// account 用于本次流程后续判断的账号
	account := New(Config{CookieID: "account-1", CookieStr: "unb=me", Handler: handler})
	// conn 用于本次流程后续判断的conn
	conn := &fakeWSConn{}
	account.mu.Lock()
	account.conn = conn
	account.mu.Unlock()
	// ctx 用于本次流程后续判断的ctx
	ctx := WithOutgoingMessageKey(context.Background(), "local-1")
	if // err 用于本次流程后续判断的err
	err := account.SendText(ctx, "chat-1", "buyer-1", "您好"); err != nil {
		t.Fatal(err)
	}
	if len(handler.messages) != 1 {
		t.Fatalf("messages=%+v", handler.messages)
	}
	// got 用于本次流程后续判断的got
	got := handler.messages[0]
	if got.AccountID != "account-1" || got.ChatID != "chat-1" || got.BuyerID != "buyer-1" || got.Text != "您好" || got.MessageKey != "local-1" {
		t.Fatalf("observation=%+v", got)
	}
}

// TestSendImageEmitsPlatformIdentifiedOutgoingObservation 验证自动化图片成功后立即产生带 PNM 的图片旁路事件。
func TestSendImageEmitsPlatformIdentifiedOutgoingObservation(t *testing.T) {
	// handler 记录图片平台成功后的本地聊天观察事件。
	handler := &outgoingObserverHandler{}
	// account 是使用固定账号身份和聊天旁路处理器的运行时。
	account := New(Config{CookieID: "account-1", CookieStr: "unb=me", Handler: handler})
	// conn 是返回固定图片 PNM ID 的 WebSocket 测试连接。
	conn := &imageResultWSConn{}
	account.mu.Lock()
	account.conn = conn
	account.mu.Unlock()
	// sendErr 是自动化图片通过无结果兼容方法发送时的结果。
	sendErr := account.SendImage(context.Background(), "chat-1", "buyer-1", "https://cdn.example/card.png", 8, 640, 480)
	if sendErr != nil {
		t.Fatalf("发送图片失败: %v", sendErr)
	}
	if len(handler.messages) != 1 {
		t.Fatalf("图片成功后应产生一次出站观察事件: %+v", handler.messages)
	}
	// observed 是 Engine 交给 Adapter 的图片成功事件。
	observed := handler.messages[0]
	if observed.AccountID != "account-1" || observed.ChatID != "chat-1" || observed.BuyerID != "buyer-1" || observed.MessageType != "image" || observed.Text != "https://cdn.example/card.png" || observed.PlatformMessageID != "image-result.PNM" || observed.SentAt != 2000 {
		t.Fatalf("图片出站观察字段错误: %+v", observed)
	}
}

// TestSendLocationCardEmitsStructuredObservation 验证位置卡片使用当前连接发送并产生可幂等合并的结构化旁路。
func TestSendLocationCardEmitsStructuredObservation(t *testing.T) {
	// handler 记录平台成功后的本地位置卡片旁路。
	handler := &outgoingObserverHandler{}
	// account 是使用固定账号身份和旁路处理器的运行时。
	account := New(Config{CookieID: "account-1", CookieStr: "unb=me", Handler: handler})
	// conn 是支持位置卡片平台结果的测试连接。
	conn := &locationResultWSConn{}
	account.runtimeMu.Lock()
	account.conn = conn
	account.runtimeMu.Unlock()
	// ctx 携带应用层预建的本地位置消息键。
	ctx := WithOutgoingMessageKey(context.Background(), "local-location")
	// result、sendErr 是位置卡片平台结果及运行时错误。
	result, sendErr := account.SendLocationCardWithResult(ctx, "chat-1", "buyer-1", "CoverAI 实体店", "东门电梯上楼右转", 22.540503, 113.934528)
	if sendErr != nil || result.PlatformMessageID != "location-result.PNM" || conn.title != "CoverAI 实体店" || conn.description != "东门电梯上楼右转" {
		t.Fatalf("result=%+v conn=%+v err=%v", result, conn, sendErr)
	}
	if len(handler.messages) != 1 {
		t.Fatalf("messages=%+v", handler.messages)
	}
	// observed 是 Engine 交给 Adapter 的位置卡片成功事件。
	observed := handler.messages[0]
	if observed.MessageKey != "local-location" || observed.MessageType != "location" || observed.PlatformMessageID != "location-result.PNM" || observed.SentAt != 3000 || observed.Text != `{"title":"CoverAI 实体店","description":"东门电梯上楼右转","latitude":22.540503,"longitude":113.934528}` {
		t.Fatalf("observation=%+v", observed)
	}
}

// TestSendReplyTextUsesDedicatedConnectionCapability 验证账号运行时把引用 PNM ID 交给专用 WebSocket 能力，不降级为普通文本。
func TestSendReplyTextUsesDedicatedConnectionCapability(t *testing.T) {
	// handler 记录平台成功后不包含凭证的出站旁路事件。
	handler := &outgoingObserverHandler{}
	// account 是具备本地账号身份和旁路处理器的运行时。
	account := New(Config{CookieID: "account-1", CookieStr: "unb=me", Handler: handler})
	// conn 是支持原生引用结果的 WebSocket 测试替身。
	conn := &replyWSConn{}
	account.mu.Lock()
	account.conn = conn
	account.mu.Unlock()
	// ctx 携带 UI 创建的本地幂等键，供出站旁路收口同一消息。
	ctx := WithOutgoingMessageKey(context.Background(), "local-reply")
	// result、err 是账号运行时返回的新平台消息标识和错误。
	result, err := account.SendReplyTextWithResult(ctx, "chat-1", "buyer-1", "引用回复", "target-1.PNM")
	if err != nil || result.PlatformMessageID != "reply-result.PNM" || conn.replyTarget != "target-1.PNM" {
		t.Fatalf("result=%+v target=%q err=%v", result, conn.replyTarget, err)
	}
	if len(handler.messages) != 1 || handler.messages[0].MessageKey != "local-reply" {
		t.Fatalf("messages=%+v", handler.messages)
	}
}
