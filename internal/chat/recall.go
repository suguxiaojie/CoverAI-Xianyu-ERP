package chat

import (
	"context"
	"time"

	"xianyu-go/internal/db"
)

// BindPlatformMessageID 保存平台发送响应中的 PNM ID，并广播同一本地消息的增量更新。
func (s *Service) BindPlatformMessageID(ctx context.Context, accountID, key, platformMessageID string) (*db.ChatMessage, error) {
	// message、err 保存完成平台 ID 绑定后的消息及持久化错误。
	message, err := s.repository.BindPlatformMessageID(ctx, accountID, key, platformMessageID)
	if err == nil {
		s.Publish(accountID, Event{Type: "message.updated", Message: message})
	}
	return message, err
}

// MarkMessageRecalled 按平台事件或主动撤回结果收敛消息状态，并广播前端增量。
func (s *Service) MarkMessageRecalled(ctx context.Context, accountID, platformMessageID string, operatorType int, operatorID string, recalledAt int64) (*db.ChatMessage, error) {
	if recalledAt <= 0 {
		recalledAt = time.Now().UTC().UnixMilli()
	}
	// message、err 保存撤回后的消息及持久化结果。
	message, err := s.repository.MarkMessageRecalled(ctx, accountID, platformMessageID, operatorType, operatorID, recalledAt)
	if err == nil {
		s.Publish(accountID, Event{Type: "message.updated", Message: message})
	}
	return message, err
}
