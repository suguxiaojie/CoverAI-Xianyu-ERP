package automation

import "context"

// resolveAssociatedOrderTask 为需要本地唯一候选的系统事件补齐订单；无法唯一关联时记录原因并安全忽略业务事实。
func (c *Center) resolveAssociatedOrderTask(ctx context.Context, task Task) (Task, bool, error) {
	switch task.TriggerType {
	case TriggerOrderShipped:
		// resolvedTask、matched、resolveErr 是发货卡片订单关联结果、唯一匹配标记和查询错误。
		resolvedTask, matched, resolveErr := c.resolveOrderShippedTask(ctx, task)
		if resolveErr != nil {
			return task, false, resolveErr
		}
		if !matched {
			c.logger.Info("无订单号发货卡片未满足唯一候选条件，仅保留聊天展示", "account", task.AccountID, "chat_id", task.ChatID)
		}
		return resolvedTask, matched, nil
	case TriggerRefundCompleted:
		// resolvedTask、matched、resolveErr 是退款成功消息订单关联结果、唯一匹配标记和查询错误。
		resolvedTask, matched, resolveErr := c.resolveRefundCompletedTask(ctx, task)
		if resolveErr != nil {
			return task, false, resolveErr
		}
		if !matched {
			c.logger.Info("无订单号退款成功消息未满足唯一候选条件，仅保留聊天展示", "account", task.AccountID, "chat_id", task.ChatID)
		}
		return resolvedTask, matched, nil
	default:
		return task, true, nil
	}
}
