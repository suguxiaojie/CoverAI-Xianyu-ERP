package chat

import (
	"context"
	"errors"
	"testing"
)

// sendRepository 是实时发送用例使用的内存持久化替身，记录消息状态迁移。
type sendRepository struct {
	// createErr 表示创建待发送消息时要返回的错误。
	createErr error
	// statusErr 表示更新发送状态时要返回的错误。
	statusErr error
	// message 保存最近一次创建的本地消息。
	message Message
	// statuses 保存按调用顺序写入的消息状态。
	statuses []string
	// replyTarget 是当前用户归属查询返回的原生引用候选。
	replyTarget Message
	// replyErr 是引用目标归属查询的预设错误。
	replyErr error
}

// CreateOutgoing 创建测试文字消息。
func (r *sendRepository) CreateOutgoing(_ context.Context, session Session, _ string, replyToPlatformMessageID string) (Message, error) {
	if r.createErr != nil {
		return Message{}, r.createErr
	}
	r.message = Message{ID: 1, AccountID: session.AccountID, ChatID: session.ChatID, MessageKey: "local-1", Status: "sending", ReplyToPlatformMessageID: replyToPlatformMessageID}
	return r.message, nil
}

// ListMessages 是发送测试不使用的历史列表能力。
func (r *sendRepository) ListMessages(context.Context, int64, string, string, int64, int) ([]Message, error) {
	return nil, nil
}

// ListSessions 是发送测试不使用的会话列表能力。
func (r *sendRepository) ListSessions(context.Context, int64, string, int) ([]Session, error) {
	return nil, nil
}

// FindMessage 返回预设的引用目标，模拟应用层归属查询。
func (r *sendRepository) FindMessage(context.Context, int64, string, string) (Message, error) {
	return r.replyTarget, r.replyErr
}

// CreateOutgoingMedia 创建测试媒体消息。
func (r *sendRepository) CreateOutgoingMedia(_ context.Context, session Session, _, content string) (Message, error) {
	if r.createErr != nil {
		return Message{}, r.createErr
	}
	r.message = Message{ID: 2, AccountID: session.AccountID, ChatID: session.ChatID, MessageKey: "local-image", Content: content, Status: "sending"}
	return r.message, nil
}

// SetOutgoingStatus 记录状态并返回当前消息。
func (r *sendRepository) SetOutgoingStatus(_ context.Context, _, _ string, status string) (Message, error) {
	r.statuses = append(r.statuses, status)
	if r.statusErr != nil {
		return Message{}, r.statusErr
	}
	r.message.Status = status
	return r.message, nil
}

// BindPlatformMessageID 记录测试发送响应中的平台 PNM ID。
func (r *sendRepository) BindPlatformMessageID(_ context.Context, _, _ string, platformMessageID string) (Message, error) {
	r.message.PlatformMessageID = platformMessageID
	return r.message, nil
}

// MarkMessageRecalled 记录测试撤回元数据。
func (r *sendRepository) MarkMessageRecalled(_ context.Context, _ string, platformMessageID string, operatorType int, operatorID string, recalledAt int64) (Message, error) {
	r.message.PlatformMessageID = platformMessageID
	r.message.Status = "recalled"
	r.message.RecalledAt = recalledAt
	r.message.RecallOperatorType = operatorType
	r.message.RecallOperatorID = operatorID
	return r.message, nil
}

// sendProvider 是按账号返回固定发送器的测试替身。
type sendProvider struct {
	// sender 保存测试发送器；为 nil 时模拟离线账号。
	sender *sendSender
}

// TestServiceAvailabilityReportsRequiredPorts 验证发送和图片上传能力只由应用端口装配状态决定。
func TestServiceAvailabilityReportsRequiredPorts(t *testing.T) {
	// unavailable 是没有任何外发端口的聊天应用服务。
	unavailable := NewWithSending(nil, nil, nil, nil)
	if unavailable.SendingAvailable() || unavailable.ImageUploadAvailable() {
		t.Fatal("缺少外发端口时不应报告聊天能力可用")
	}
	// available 是同时装配文字发送和图片上传端口的聊天应用服务。
	available := NewWithSending(nil, &sendRepository{}, sendProvider{}, sendUploader{})
	if !available.SendingAvailable() || !available.ImageUploadAvailable() {
		t.Fatal("完整装配的聊天能力应报告可用")
	}
}

// Sender 返回测试发送器或模拟离线。
func (p sendProvider) Sender(string) (Sender, bool) {
	return p.sender, p.sender != nil
}

// sendSender 记录平台发送调用及其预设错误。
type sendSender struct {
	// sendErr 表示平台发送需要返回的错误。
	sendErr error
	// sentKey 保存最近一次收到的幂等键。
	sentKey string
	// updatedCookie 保存适配器同步的刷新凭证；测试只记录是否调用，不保存真实秘密。
	updatedCookie bool
	// imageWidth、imageHeight 保存图片发送收到的像素尺寸。
	imageWidth, imageHeight int
	// replyTarget 保存原生引用发送传入的目标 PNM ID。
	replyTarget string
	// location 保存最近一次位置卡片的标题、说明和坐标。
	location LocationCard
	// recallPlatformMessageID、recallContentType 保存最近一次撤回的 PNM ID 和原消息协议类型。
	recallPlatformMessageID string
	recallContentType       int
}

// SendText 记录文本发送并返回预设错误。
func (s *sendSender) SendText(_ context.Context, _, _, _, messageKey string) (PlatformSendResult, error) {
	s.sentKey = messageKey
	return PlatformSendResult{PlatformMessageID: "platform-text.PNM", CreatedAt: 1000}, s.sendErr
}

// SendReplyText 记录原生引用目标和幂等键，并返回预设平台结果。
func (s *sendSender) SendReplyText(_ context.Context, _, _, _, replyToPlatformMessageID, messageKey string) (PlatformSendResult, error) {
	s.replyTarget = replyToPlatformMessageID
	s.sentKey = messageKey
	return PlatformSendResult{PlatformMessageID: "platform-reply.PNM", CreatedAt: 1000}, s.sendErr
}

// SendImage 记录图片发送并返回预设错误。
func (s *sendSender) SendImage(_ context.Context, _, _, _ string, _ int64, width, height int, messageKey string) (PlatformSendResult, error) {
	// imageWidth、imageHeight 记录应用层透传的平台图片尺寸。
	s.imageWidth, s.imageHeight = width, height
	s.sentKey = messageKey
	return PlatformSendResult{PlatformMessageID: "platform-image.PNM", CreatedAt: 1000}, s.sendErr
}

// SendLocationCard 记录位置字段和幂等键，并返回固定平台消息标识。
func (s *sendSender) SendLocationCard(_ context.Context, _, _, title, description string, latitude, longitude float64, messageKey string) (PlatformSendResult, error) {
	s.location = LocationCard{Title: title, Description: description, Latitude: latitude, Longitude: longitude}
	s.sentKey = messageKey
	return PlatformSendResult{PlatformMessageID: "platform-location.PNM", CreatedAt: 1000}, s.sendErr
}

// RecallMessage 记录撤回目标和协议类型，并返回测试配置的撤回错误。
func (s *sendSender) RecallMessage(_ context.Context, platformMessageID string, contentType int, _ string) error {
	s.recallPlatformMessageID, s.recallContentType = platformMessageID, contentType
	return s.sendErr
}

// UpdateCookie 记录刷新凭证同步动作，但不保存明文内容。
func (s *sendSender) UpdateCookie(string) {
	s.updatedCookie = true
}

// sendUploader 是图片上传端口的测试替身。
type sendUploader struct {
	// result 保存上传成功后返回的图片地址。
	result ImageUpload
	// err 保存上传或凭证刷新阶段需要返回的错误。
	err error
}

// UploadChatImage 返回预设上传结果，不接收明文 Cookie 参数。
func (u sendUploader) UploadChatImage(context.Context, string, string, string, []byte) (ImageUpload, error) {
	return u.result, u.err
}

// TestSendTextSuccessPreservesIdempotencyKey 验证成功发送会写入 sent 状态并传递本地幂等键。
func TestSendTextSuccessPreservesIdempotencyKey(t *testing.T) {
	// repository、sender 保存实时发送用例的测试端口。
	repository, sender := &sendRepository{}, &sendSender{}
	// service 保存使用测试端口构造的聊天发送服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, nil)
	// message 和 err 保存应用层返回的消息及错误。
	message, err := service.SendText(context.Background(), OutgoingInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Text: "  你好  "})
	if err != nil || message == nil || message.Status != "sent" || sender.sentKey != "local-1" {
		t.Fatalf("message=%+v err=%v key=%q", message, err, sender.sentKey)
	}
	if len(repository.statuses) != 1 || repository.statuses[0] != "sent" {
		t.Fatalf("statuses=%v", repository.statuses)
	}
}

// TestSendReplyTextValidatesAndPersistsTarget 验证同会话 PNM 目标同时进入本地待发送行和平台原生发送端口。
func TestSendReplyTextValidatesAndPersistsTarget(t *testing.T) {
	// repository 保存当前用户确实拥有的同会话 PNM 目标。
	repository := &sendRepository{replyTarget: Message{AccountID: "acc-1", ChatID: "chat-1", MessageKey: "target-local", PlatformMessageID: "target-1.PNM", MessageType: "text"}}
	// sender 记录应用层选择原生引用端口时的精确目标。
	sender := &sendSender{}
	// service 同时使用同一替身提供目标归属查询和待发送写入。
	service := NewWithSending(repository, repository, sendProvider{sender: sender}, nil)
	// message、err 是原生引用发送的本地结果和错误。
	message, err := service.SendText(context.Background(), OutgoingInput{UserID: 7, Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Text: "引用回复", ReplyToMessageKey: "target-local"})
	if err != nil || message == nil || message.Status != "sent" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	if message.ReplyToPlatformMessageID != "target-1.PNM" || sender.replyTarget != "target-1.PNM" || sender.sentKey != "local-1" {
		t.Fatalf("message=%+v sender=%+v", message, sender)
	}
}

// TestSendReplyTextRejectsCrossConversationAndMissingPlatformID 验证引用目标不能跨会话或使用缺少 PNM 的历史行。
func TestSendReplyTextRejectsCrossConversationAndMissingPlatformID(t *testing.T) {
	// cases 覆盖跨会话和无平台标识两个必须在写入前拒绝的边界。
	cases := []Message{
		{AccountID: "acc-1", ChatID: "chat-2", PlatformMessageID: "target-2.PNM", MessageType: "text"},
		{AccountID: "acc-1", ChatID: "chat-1", MessageType: "text"},
	}
	// target 是当前分支返回的非法引用候选。
	for _, target := range cases {
		// repository 记录该非法目标，不应创建待发送行。
		repository := &sendRepository{replyTarget: target}
		// sender 用于断言非法请求没有到达平台端口。
		sender := &sendSender{}
		// service 是当前非法目标分支的应用服务。
		service := NewWithSending(repository, repository, sendProvider{sender: sender}, nil)
		// err 是应用层返回的引用禁止错误。
		_, err := service.SendText(context.Background(), OutgoingInput{UserID: 7, Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Text: "引用回复", ReplyToMessageKey: "target-local"})
		if !errors.Is(err, ErrReplyNotAllowed) || repository.message.MessageKey != "" || sender.replyTarget != "" {
			t.Fatalf("target=%+v message=%+v sender=%+v err=%v", target, repository.message, sender, err)
		}
	}
}

// TestSendTextFailureMarksMessageFailed 验证平台失败会保留可重试的本地 failed 状态。
func TestSendTextFailureMarksMessageFailed(t *testing.T) {
	// repository、sender 保存发送失败场景的测试端口。
	repository, sender := &sendRepository{}, &sendSender{sendErr: errors.New("远端拒绝")}
	// service 保存使用测试端口构造的聊天发送服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, nil)
	// message 和 err 保存失败后的本地消息及错误。
	message, err := service.SendText(context.Background(), OutgoingInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Text: "你好"})
	if !errors.Is(err, ErrSend) || message == nil || message.Status != "failed" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	if len(repository.statuses) != 1 || repository.statuses[0] != "failed" {
		t.Fatalf("statuses=%v", repository.statuses)
	}
}

// TestSendTextStatusFailureReturnsSentMessage 验证平台成功但本地状态写入失败会返回 ErrStatusSave。
func TestSendTextStatusFailureReturnsSentMessage(t *testing.T) {
	// repository 保存状态写入错误；sender 保存成功发送记录。
	repository, sender := &sendRepository{statusErr: errors.New("数据库不可用")}, &sendSender{}
	// service 保存使用测试端口构造的聊天发送服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, nil)
	// message 和 err 保存状态写入失败的返回值。
	message, err := service.SendText(context.Background(), OutgoingInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Text: "你好"})
	if !errors.Is(err, ErrStatusSave) || message == nil || message.MessageKey != "local-1" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
}

// TestSendRejectsUnavailableOfflineAndInvalidInputs 验证不可用、离线和非法输入均在访问端口前失败。
func TestSendRejectsUnavailableOfflineAndInvalidInputs(t *testing.T) {
	// session 保存可复用的有效会话参数。
	session := Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}
	// cases 描述发送服务边界分支。
	cases := []struct {
		// name 标识当前测试分支。
		name string
		// service 保存当前分支使用的聊天服务。
		service *Service
		// input 保存当前分支使用的发送输入。
		input OutgoingInput
		// wantErr 保存预期应用错误。
		wantErr error
	}{
		{name: "unavailable", service: NewWithSending(nil, nil, nil, nil), input: OutgoingInput{Session: session, Text: "你好"}, wantErr: ErrUnavailable},
		{name: "offline", service: NewWithSending(nil, &sendRepository{}, sendProvider{}, nil), input: OutgoingInput{Session: session, Text: "你好"}, wantErr: ErrOffline},
		{name: "invalid", service: NewWithSending(nil, &sendRepository{}, sendProvider{sender: &sendSender{}}, nil), input: OutgoingInput{Session: session, Text: ""}, wantErr: ErrSendInvalidInput},
	}
	// testCase 表示当前遍历的发送服务边界分支。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// err 保存当前边界分支返回的应用错误。
			_, err := testCase.service.SendText(context.Background(), testCase.input)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error=%v want=%v", err, testCase.wantErr)
			}
		})
	}
}

// TestSendImageDoesNotExposeCredentials 验证图片应用端口只接收账号标识，不传递 Cookie 字段。
func TestSendImageDoesNotExposeCredentials(t *testing.T) {
	// repository、sender、uploader 保存图片发送的测试端口。
	repository, sender := &sendRepository{}, &sendSender{}
	// uploader 保存固定图片地址的测试上传端口。
	uploader := sendUploader{result: ImageUpload{URL: "https://cdn.example/image.jpg", Width: 1280, Height: 720}}
	// service 保存使用测试端口构造的聊天发送服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, uploader)
	// message 和 err 保存图片发送结果。
	message, err := service.SendImage(context.Background(), ImageInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Filename: "a.jpg", ContentType: "image/jpeg", Data: []byte("image")})
	if err != nil || message == nil || message.Content != "https://cdn.example/image.jpg" || sender.sentKey != "local-image" || sender.imageWidth != 1280 || sender.imageHeight != 720 {
		t.Fatalf("message=%+v err=%v key=%q", message, err, sender.sentKey)
	}
}

// TestSendImagePropagatesCredentialWritebackFailure 验证平台适配器返回凭证写回错误时不会静默继续发送。
func TestSendImagePropagatesCredentialWritebackFailure(t *testing.T) {
	// repository、sender、uploader 保存凭证写回失败场景的测试端口。
	repository, sender := &sendRepository{}, &sendSender{}
	// uploader 保存模拟凭证写回失败的测试上传端口。
	uploader := sendUploader{err: errors.New("凭证写回失败")}
	// service 保存使用测试端口构造的聊天发送服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, uploader)
	// _, err 保存图片上传适配器返回的确定性错误。
	_, err := service.SendImage(context.Background(), ImageInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Filename: "a.jpg", ContentType: "image/jpeg", Data: []byte("image")})
	if !errors.Is(err, ErrSend) || sender.sentKey != "" || len(repository.statuses) != 0 {
		t.Fatalf("err=%v sentKey=%q statuses=%v", err, sender.sentKey, repository.statuses)
	}
}

// TestSendImageStatusFailureReturnsSentMessage 验证图片平台成功但本地状态写入失败仍返回幂等消息。
func TestSendImageStatusFailureReturnsSentMessage(t *testing.T) {
	// repository、sender、uploader 保存图片状态写入失败场景的测试端口。
	repository, sender := &sendRepository{statusErr: errors.New("状态写入失败")}, &sendSender{}
	// uploader 保存固定图片地址的测试上传端口。
	uploader := sendUploader{result: ImageUpload{URL: "https://cdn.example/image.jpg"}}
	// service 保存使用测试端口构造的聊天发送服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, uploader)
	// message 和 err 保存图片状态写入失败后的返回值。
	message, err := service.SendImage(context.Background(), ImageInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Filename: "a.jpg", ContentType: "image/jpeg", Data: []byte("image")})
	if !errors.Is(err, ErrStatusSave) || message == nil || message.MessageKey != "local-image" || sender.sentKey != "local-image" {
		t.Fatalf("message=%+v err=%v key=%q", message, err, sender.sentKey)
	}
}

// TestSendLocationCardPersistsStableContent 验证自定义位置卡片使用本地幂等消息并完整透传展示字段。
func TestSendLocationCardPersistsStableContent(t *testing.T) {
	// repository、sender 是位置卡片本地状态和平台发送替身。
	repository, sender := &sendRepository{}, &sendSender{}
	// service 是装配完整发送端口的聊天应用服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, nil)
	// message、sendErr 是位置卡片最终消息及平台或持久化错误。
	message, sendErr := service.SendLocationCard(context.Background(), LocationInput{
		Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"},
		Title:   " CoverAI 实体店 ", Description: " 东门电梯上楼右转 ", Latitude: 22.540503, Longitude: 113.934528,
	})
	if sendErr != nil || message == nil || message.Status != "sent" || message.PlatformMessageID != "platform-location.PNM" {
		t.Fatalf("message=%+v err=%v", message, sendErr)
	}
	if sender.sentKey != "local-image" || sender.location.Title != "CoverAI 实体店" || sender.location.Description != "东门电梯上楼右转" {
		t.Fatalf("sender=%+v", sender)
	}
	// parsed、ok 是应用响应正文重新解析的位置卡片和完整性标记。
	parsed, ok := ParseLocationCardContent(message.Content)
	if !ok || *parsed != sender.location {
		t.Fatalf("content=%q parsed=%+v sender=%+v", message.Content, parsed, sender.location)
	}
}

// TestSendLocationCardRejectsInvalidInput 验证无效坐标在创建本地消息前被拒绝。
func TestSendLocationCardRejectsInvalidInput(t *testing.T) {
	// repository、sender 用于确认非法输入没有触碰本地或平台端口。
	repository, sender := &sendRepository{}, &sendSender{}
	// service 是待验证字段门禁的聊天应用服务。
	service := NewWithSending(nil, repository, sendProvider{sender: sender}, nil)
	// _, sendErr 只保留预期的输入错误。
	_, sendErr := service.SendLocationCard(context.Background(), LocationInput{Session: Session{AccountID: "acc-1", ChatID: "chat-1", BuyerID: "buyer-1"}, Title: "店铺", Description: "地址", Latitude: 100, Longitude: 120})
	if !errors.Is(sendErr, ErrSendInvalidInput) || repository.message.MessageKey != "" || sender.sentKey != "" {
		t.Fatalf("message=%+v sender=%+v err=%v", repository.message, sender, sendErr)
	}
}
