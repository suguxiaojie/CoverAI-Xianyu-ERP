package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrRecallInvalidInput 表示撤回请求缺少用户、账号或本地消息键。
	ErrRecallInvalidInput = errors.New("聊天撤回参数无效")
	// ErrRecallNotFound 表示消息不存在或不属于当前认证用户。
	ErrRecallNotFound = errors.New("聊天消息不存在")
	// ErrRecallUnavailable 表示撤回所需仓储或在线发送器未装配。
	ErrRecallUnavailable = errors.New("聊天撤回服务未启用")
	// ErrRecallNotAllowed 表示消息不是当前用户可撤回的已发送文本、图片或位置卡片。
	ErrRecallNotAllowed = errors.New("当前消息不可撤回")
	// ErrRecallExpired 表示消息已经超过闲鱼两分钟撤回窗口。
	ErrRecallExpired = errors.New("消息已超过两分钟撤回期限")
	// ErrRecallNotReady 表示发送成功但平台 PNM ID 尚未持久化。
	ErrRecallNotReady = errors.New("平台消息标识尚未同步")
	// ErrRecallAlready 表示消息已经被任一端撤回。
	ErrRecallAlready = errors.New("消息已经撤回")
	// ErrRecall 表示平台明确拒绝或运行时无法执行撤回。
	ErrRecall = errors.New("聊天消息撤回失败")
	// ErrRecallUncertain 表示请求超时或取消，平台最终状态需要事件或历史确认。
	ErrRecallUncertain = errors.New("聊天撤回结果待确认")
)

// RecallInput 是撤回应用用例的非敏感输入。
type RecallInput struct {
	// UserID 是当前认证用户，用于数据库归属校验。
	UserID int64
	// AccountID 是发送消息的本地账号标识。
	AccountID string
	// MessageKey 是前端持有的本地幂等消息键。
	MessageKey string
}

// Recall 撤回当前用户拥有账号在两分钟内发出的文本、图片或位置卡片。
func (s *Service) Recall(ctx context.Context, input RecallInput) (*Message, error) {
	input.AccountID = strings.TrimSpace(input.AccountID)
	input.MessageKey = strings.TrimSpace(input.MessageKey)
	if input.UserID <= 0 || input.AccountID == "" || input.MessageKey == "" {
		return nil, ErrRecallInvalidInput
	}
	if s == nil || s.repository == nil || s.outgoing == nil || s.senders == nil {
		return nil, ErrRecallUnavailable
	}
	// message、err 保存通过用户归属校验的撤回候选和查询错误。
	message, err := s.repository.FindMessage(ctx, input.UserID, input.AccountID, input.MessageKey)
	if err != nil {
		return nil, err
	}
	if message.Direction != "outgoing" || (message.MessageType != "text" && message.MessageType != "image" && message.MessageType != "location") {
		return &message, ErrRecallNotAllowed
	}
	if message.Status == "recalled" || message.RecalledAt > 0 {
		return &message, ErrRecallAlready
	}
	if message.Status != "sent" && message.Status != "recall_unknown" {
		return &message, ErrRecallNotAllowed
	}
	if strings.TrimSpace(message.PlatformMessageID) == "" {
		return &message, ErrRecallNotReady
	}
	// sentAtMillis 兼容历史秒级和当前毫秒级消息时间。
	sentAtMillis := message.SentAt
	if sentAtMillis > 0 && sentAtMillis < 10_000_000_000 {
		sentAtMillis *= 1000
	}
	// now 使用服务可注入时钟，保证两分钟边界可确定测试。
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	if sentAtMillis <= 0 || now.UnixMilli()-sentAtMillis > 120_000 || sentAtMillis-now.UnixMilli() > 10_000 {
		return &message, ErrRecallExpired
	}
	// sender、ok 保存目标账号在线撤回句柄和存在性。
	sender, ok := s.senders.Sender(input.AccountID)
	if !ok || sender == nil {
		return &message, ErrOffline
	}
	// pending、statusErr 保存撤回请求发出前的本地可观测状态和写入错误。
	pending, statusErr := s.outgoing.SetOutgoingStatus(ctx, input.AccountID, input.MessageKey, "recall_pending")
	if statusErr != nil {
		return messagePointer(pending, message), fmt.Errorf("%w: %v", ErrStatusSave, statusErr)
	}
	// contentType 是官方撤回 Feature 使用的原内容类型。
	contentType := 1
	if message.MessageType == "image" {
		contentType = 2
	} else if message.MessageType == "location" {
		contentType = 30
	}
	// recallErr 保存平台明确失败或结果不确定错误。
	recallErr := sender.RecallMessage(ctx, message.PlatformMessageID, contentType, message.Content)
	if recallErr != nil {
		if errors.Is(recallErr, context.DeadlineExceeded) || errors.Is(recallErr, context.Canceled) {
			// unknown、_ 保存结果不确定状态；写入失败不覆盖原始不确定语义。
			unknown, _ := s.outgoing.SetOutgoingStatus(context.Background(), input.AccountID, input.MessageKey, "recall_unknown")
			return messagePointer(unknown, message), fmt.Errorf("%w: %v", ErrRecallUncertain, recallErr)
		}
		// restored、_ 在平台明确拒绝时恢复 sent，允许用户修正条件后再次操作。
		restored, _ := s.outgoing.SetOutgoingStatus(context.Background(), input.AccountID, input.MessageKey, "sent")
		return messagePointer(restored, message), fmt.Errorf("%w: %v", ErrRecall, recallErr)
	}
	// recalled、err 保存平台成功后的本地撤回状态。
	recalled, err := s.outgoing.MarkMessageRecalled(ctx, input.AccountID, message.PlatformMessageID, 0, "", now.UnixMilli())
	if err != nil {
		return messagePointer(recalled, message), fmt.Errorf("%w: %v", ErrStatusSave, err)
	}
	return messagePointer(recalled, message), nil
}
