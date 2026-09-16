package automation

import (
	"context"
	"strings"
)

// resolveRefundCompletedTask 为无订单号退款成功消息执行严格唯一候选关联；matched=false 表示只保留聊天展示。
func (c *Center) resolveRefundCompletedTask(ctx context.Context, task Task) (Task, bool, error) {
	if task.TriggerType != TriggerRefundCompleted {
		return task, true, nil
	}
	task.OrderStatus = "refunded"
	if strings.TrimSpace(task.OrderID) != "" {
		return task, true, nil
	}
	if c == nil || c.store == nil || c.store.Orders == nil || strings.TrimSpace(task.AccountID) == "" || strings.TrimSpace(task.ChatID) == "" {
		return task, false, nil
	}
	// candidates、queryErr 是同账号同会话退款中订单及查询错误。
	candidates, queryErr := c.store.Orders.RefundingByChat(ctx, task.AccountID, task.ChatID, 3)
	if queryErr != nil {
		return task, false, queryErr
	}
	if len(candidates) != 1 {
		return task, false, nil
	}
	// matched 是唯一仍处于退款中的订单，普通系统文本只能关联到该订单。
	matched := candidates[0]
	task.OrderID = matched.OrderID
	task.ChatID = matched.ChatID
	return task, true, nil
}
