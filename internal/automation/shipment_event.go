package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	"xianyu-go/internal/db"
)

// shipmentAssociationWindow 限制无订单号发货卡片只能关联二十四小时内的已付款订单。
const shipmentAssociationWindow = 24 * time.Hour

// resolveOrderShippedTask 为无订单号发货卡片执行严格唯一候选关联；matched=false 表示安全忽略自动化事实。
func (c *Center) resolveOrderShippedTask(ctx context.Context, task Task) (Task, bool, error) {
	if task.TriggerType != TriggerOrderShipped {
		return task, true, nil
	}
	task.OrderStatus = "shipped"
	if strings.TrimSpace(task.OrderID) != "" {
		return task, true, nil
	}
	if c == nil || c.store == nil || c.store.Orders == nil || strings.TrimSpace(task.AccountID) == "" || strings.TrimSpace(task.ChatID) == "" {
		return task, false, nil
	}
	// candidates、queryErr 是同账号同会话尚未发货的已付款开放订单及查询错误。
	candidates, queryErr := c.store.Orders.UnshippedPaidByChat(ctx, task.AccountID, task.ChatID, 3)
	if queryErr != nil {
		return task, false, queryErr
	}
	// occurredAt 是平台发货卡片时间；协议缺失时使用本机接收时间，但仍执行付款窗口校验。
	occurredAt := time.Now().UTC()
	if task.OccurredAt > 0 {
		occurredAt = time.UnixMilli(task.OccurredAt).UTC()
	}
	// eligible 保存付款早于发货且间隔不超过二十四小时的候选。
	eligible := make([]db.ShipmentAssociationCandidate, 0, len(candidates))
	// candidate 是当前待检查的会话订单。
	for _, candidate := range candidates {
		// paidAt 是候选订单付款时间。
		paidAt := parseDBTime(candidate.PaidAt)
		if paidAt.IsZero() || occurredAt.Before(paidAt) || occurredAt.Sub(paidAt) > shipmentAssociationWindow {
			continue
		}
		eligible = append(eligible, candidate)
	}
	if len(eligible) != 1 {
		return task, false, nil
	}
	// matched 是唯一通过账号、会话、状态和时间窗口校验的订单。
	matched := eligible[0]
	task.OrderID = matched.OrderID
	task.ItemID = firstNonEmpty(task.ItemID, matched.ItemID)
	task.BuyerID = firstNonEmpty(task.BuyerID, matched.BuyerID)
	task.ChatID = firstNonEmpty(task.ChatID, matched.ChatID)
	return task, true, nil
}

// handleOrderShippedTask 在发货事实落库和账号门禁通过后创建秒级求花任务；调度失败由分钟补偿兜底。
func (c *Center) handleOrderShippedTask(ctx context.Context, task Task) error {
	if strings.TrimSpace(task.OrderID) == "" {
		return nil
	}
	// scheduleErr 是发货卡片事实触发秒级求花任务的错误。
	scheduleErr := c.ScheduleRedFlowerAfterShipment(ctx, task.OrderID)
	if scheduleErr != nil && !errors.Is(scheduleErr, db.ErrNotFound) {
		c.logger.Warn("实时发货卡片创建自动求花任务失败，等待分钟补偿扫描", "account", task.AccountID, "order_id", task.OrderID, "err", scheduleErr)
	}
	return nil
}
