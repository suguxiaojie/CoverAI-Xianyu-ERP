package chat

import (
	"context"
	"errors"
	"testing"
	"time"
)

// recallMessageFixture 返回处于两分钟窗口且已经绑定 PNM ID 的己方文本消息。
func recallMessageFixture() Message {
	return Message{ID: 1, AccountID: "account-1", ChatID: "chat-1", MessageKey: "local-1",
		PlatformMessageID: "platform-1.PNM", Direction: "outgoing", MessageType: "text", Content: "测试",
		Status: "sent", SentAt: 1_800_000_000_000}
}

// newRecallService 构造可控时钟、仓储和发送器的撤回应用服务。
func newRecallService(message Message, recallErr error) (*Service, *sendRepository) {
	// repository 保存归属查询返回的撤回候选。
	repository := &fakeRepository{messages: []Message{message}}
	// outgoing 保存撤回状态迁移，初始消息与查询结果一致。
	outgoing := &sendRepository{message: message}
	// service 保存完整撤回端口。
	service := NewWithSending(repository, outgoing, sendProvider{sender: &sendSender{sendErr: recallErr}}, nil)
	service.now = func() time.Time { return time.UnixMilli(1_800_000_060_000) }
	return service, outgoing
}

// TestRecallSuccessMarksPlatformMessageRecalled 验证成功撤回经过 pending 并保存平台状态。
func TestRecallSuccessMarksPlatformMessageRecalled(t *testing.T) {
	// service、outgoing 保存成功撤回服务和状态仓储。
	service, outgoing := newRecallService(recallMessageFixture(), nil)
	// message、err 保存撤回结果。
	message, err := service.Recall(context.Background(), RecallInput{UserID: 1, AccountID: "account-1", MessageKey: "local-1"})
	if err != nil || message == nil || message.Status != "recalled" || message.PlatformMessageID != "platform-1.PNM" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	if len(outgoing.statuses) != 1 || outgoing.statuses[0] != "recall_pending" {
		t.Fatalf("statuses=%v", outgoing.statuses)
	}
}

// TestRecallLocationUsesContentType30 验证结构化位置消息通过白名单并按平台原类型撤回。
func TestRecallLocationUsesContentType30(t *testing.T) {
	// location 是两分钟窗口内已绑定 PNM 的己方位置卡片。
	location := recallMessageFixture()
	location.MessageType = "location"
	location.Content = `{"title":"实体店","description":"地址","latitude":31.2,"longitude":121.4}`
	// repository、outgoing 是撤回归属查询和状态迁移替身。
	repository, outgoing := &fakeRepository{messages: []Message{location}}, &sendRepository{message: location}
	// sender 记录应用层实际交给平台的 PNM 与 contentType。
	sender := &sendSender{}
	// service 是带固定时钟的完整位置撤回用例。
	service := NewWithSending(repository, outgoing, sendProvider{sender: sender}, nil)
	service.now = func() time.Time { return time.UnixMilli(1_800_000_060_000) }
	// message、recallErr 是位置卡片撤回后的终态和错误。
	message, recallErr := service.Recall(context.Background(), RecallInput{UserID: 1, AccountID: "account-1", MessageKey: "local-1"})
	if recallErr != nil || message == nil || message.Status != "recalled" || sender.recallPlatformMessageID != "platform-1.PNM" || sender.recallContentType != 30 {
		t.Fatalf("message=%+v sender=%+v err=%v", message, sender, recallErr)
	}
}

// TestRecallRejectsExpiredMissingPlatformAndIncoming 验证服务端权威撤回边界。
func TestRecallRejectsExpiredMissingPlatformAndIncoming(t *testing.T) {
	// cases 描述三种不应访问平台的撤回候选。
	cases := []struct {
		// name 标识当前边界。
		name string
		// mutate 修改基础消息以命中目标边界。
		mutate func(*Message)
		// wantErr 是预期应用错误。
		wantErr error
	}{
		{name: "expired", mutate: func(message *Message) { message.SentAt = 1_799_999_800_000 }, wantErr: ErrRecallExpired},
		{name: "missing-platform-id", mutate: func(message *Message) { message.PlatformMessageID = "" }, wantErr: ErrRecallNotReady},
		{name: "incoming", mutate: func(message *Message) { message.Direction = "incoming" }, wantErr: ErrRecallNotAllowed},
	}
	// testCase 是当前执行的撤回边界。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// message 保存当前边界修改后的候选消息。
			message := recallMessageFixture()
			testCase.mutate(&message)
			// service、outgoing 保存当前边界的服务和状态仓储。
			service, outgoing := newRecallService(message, nil)
			// _, err 保存边界返回错误。
			_, err := service.Recall(context.Background(), RecallInput{UserID: 1, AccountID: "account-1", MessageKey: "local-1"})
			if !errors.Is(err, testCase.wantErr) || len(outgoing.statuses) != 0 {
				t.Fatalf("err=%v want=%v statuses=%v", err, testCase.wantErr, outgoing.statuses)
			}
		})
	}
}

// TestRecallTimeoutKeepsUncertainState 验证平台超时不会盲目恢复 sent 或自动重试。
func TestRecallTimeoutKeepsUncertainState(t *testing.T) {
	// service、outgoing 保存结果不确定场景服务和状态仓储。
	service, outgoing := newRecallService(recallMessageFixture(), context.DeadlineExceeded)
	// message、err 保存不确定撤回结果。
	message, err := service.Recall(context.Background(), RecallInput{UserID: 1, AccountID: "account-1", MessageKey: "local-1"})
	if !errors.Is(err, ErrRecallUncertain) || message == nil || message.Status != "recall_unknown" {
		t.Fatalf("message=%+v err=%v", message, err)
	}
	if len(outgoing.statuses) != 2 || outgoing.statuses[0] != "recall_pending" || outgoing.statuses[1] != "recall_unknown" {
		t.Fatalf("statuses=%v", outgoing.statuses)
	}
}
