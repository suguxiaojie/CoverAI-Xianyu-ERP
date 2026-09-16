package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// ScheduleRedFlowerAfterShipment 在明确发货事实持久化后创建秒级求花任务；关闭开关时安全忽略。
func (c *Center) ScheduleRedFlowerAfterShipment(ctx context.Context, orderID string) error {
	if c == nil || c.store == nil || c.store.Orders == nil {
		return errors.New("自动求花调度器未初始化")
	}
	// order、orderErr 是发货成功后重新读取的权威订单事实和错误。
	order, orderErr := c.store.Orders.Get(ctx, strings.TrimSpace(orderID))
	if orderErr != nil {
		return orderErr
	}
	return c.scheduleRedFlowerAfterShipmentOrder(ctx, order, "shipment_success")
}

// scheduleRedFlowerAfterShipmentTask 只在付款自动化运行最终成功后安排求花，其他规则成功不触发。
func (c *Center) scheduleRedFlowerAfterShipmentTask(ctx context.Context, task Task) error {
	if task.TriggerType != TriggerOrderPaid || strings.TrimSpace(task.OrderID) == "" {
		return nil
	}
	return c.ScheduleRedFlowerAfterShipment(ctx, task.OrderID)
}

// scheduleRedFlowerAfterShipmentOrder 根据最新设置和发货时间写入可跨进程恢复的秒级任务。
func (c *Center) scheduleRedFlowerAfterShipmentOrder(ctx context.Context, order *db.Order, source string) error {
	if order == nil || strings.TrimSpace(order.OrderID) == "" || strings.TrimSpace(order.CookieID) == "" {
		return errors.New("发货成功后的自动求花缺少订单事实")
	}
	// settings、settingsErr 是当前账号任务设置和读取错误。
	settings, settingsErr := c.store.AccountTasks.Get(ctx, order.CookieID)
	if settingsErr != nil {
		return settingsErr
	}
	if !settings.AutoRequestFlowerEnabled {
		return nil
	}
	// candidate 是从权威订单模型投影出的自动求花资格事实。
	candidate := flowerRequestCandidateFromOrder(order)
	// dueAt、eligible 是明确发货时间加秒级延迟后的到期点及基础资格。
	dueAt, eligible := redFlowerRequestScheduleTime(candidate, settings.RequestFlowerAfterSeconds, time.Now().UTC())
	if !eligible {
		return errors.New("订单缺少可验证的发货成功时间或不满足求花状态")
	}
	// task 是不含 Cookie 的持久化秒级求花任务。
	task := Task{
		Source: "scheduler", AccountID: order.CookieID, TriggerType: TriggerRedFlowerRequestDue,
		ChatID: order.ChatID, OrderID: order.OrderID, ItemID: order.ItemID, BuyerID: order.BuyerID,
		OrderStatus: order.OrderStatus, Text: "发货成功后自动求花",
		Raw: map[string]any{"source": source, "shipped_at": order.ShippedAt},
	}
	return c.deferTask(ctx, task, dueAt.Unix())
}

// handleScheduledRedFlowerRequest 执行到期秒级任务；明确可重试失败会重置同一持久化任务。
func (c *Center) handleScheduledRedFlowerRequest(ctx context.Context, task Task) (bool, error) {
	if c == nil || c.store == nil || c.store.Orders == nil {
		return false, errors.New("自动求花调度器未初始化")
	}
	// settings、settingsErr 是执行时重读的账号开关和秒级延迟。
	settings, settingsErr := c.store.AccountTasks.Get(ctx, task.AccountID)
	if settingsErr != nil {
		return false, settingsErr
	}
	if !settings.AutoRequestFlowerEnabled {
		return false, nil
	}
	// order、orderErr 是平台动作前重读的订单事实。
	order, orderErr := c.store.Orders.Get(ctx, task.OrderID)
	if orderErr != nil {
		if errors.Is(orderErr, db.ErrNotFound) {
			return false, nil
		}
		return false, orderErr
	}
	// candidate 是用于最终资格和通知关联的最小订单事实。
	candidate := flowerRequestCandidateFromOrder(order)
	// now 是本轮资格、幂等运行和重试时间共用的 UTC 时间。
	now := time.Now().UTC()
	// dueAt、eligible 是按最新配置重新计算的到期时间和基础资格。
	dueAt, eligible := redFlowerRequestScheduleTime(candidate, settings.RequestFlowerAfterSeconds, now)
	if !eligible {
		return false, nil
	}
	if now.Before(dueAt) {
		if // deferErr 是按最新秒数重新写入到期任务的错误。
		deferErr := c.deferTask(ctx, task, dueAt.Unix()); deferErr != nil {
			return false, deferErr
		}
		return true, nil
	}
	// requester、available 是当前账号任务客户端的官方求花能力及可用状态。
	requester, available := c.taskRunner.client().(redFlowerAccountTaskClient)
	if !available || requester == nil {
		return false, errors.New("自动求花客户端未初始化")
	}
	// cookieValue、cookieErr 是本轮平台请求使用的最新账号 Cookie。
	cookieValue, cookieErr := c.taskRunner.repository.GetValue(ctx, task.AccountID)
	if cookieErr != nil {
		return false, cookieErr
	}
	// _, retryAt、requestErr 是动作后的 Cookie、明确安全重试时间和本地／会话错误。
	_, retryAt, requestErr := c.taskRunner.requestRedFlowerCandidate(ctx, requester, settings, candidate, cookieValue, now)
	if requestErr != nil && mtop.IsSessionExpiredErr(requestErr) {
		// recoveryErr 记录凭证恢复结果；无论恢复是否成功，本轮外部动作已经停止。
		recoveryErr := c.taskRunner.recoverAccountTaskSession(ctx, task.AccountID, requestErr)
		c.logger.Warn("发货后自动求花 Session 失效，任务等待安全重试", "account", task.AccountID, "order_id", task.OrderID, "err", recoveryErr)
		if retryAt <= now.Unix() {
			retryAt = now.Add(10 * time.Minute).Unix()
		}
		if // deferErr 是 Session 恢复等待任务重新入队的错误。
		deferErr := c.deferTask(ctx, task, retryAt); deferErr != nil {
			return false, errors.Join(requestErr, deferErr)
		}
		return true, nil
	}
	if requestErr != nil {
		if errors.Is(requestErr, errAutomationNeedsReview) {
			return false, nil
		}
		return false, requestErr
	}
	if retryAt > now.Unix() {
		if // deferErr 是平台明确失败后的安全重试任务写入错误。
		deferErr := c.deferTask(ctx, task, retryAt); deferErr != nil {
			return false, deferErr
		}
		return true, nil
	}
	return false, nil
}

// flowerRequestCandidateFromOrder 投影求花资格和通知需要的非敏感订单事实。
func flowerRequestCandidateFromOrder(order *db.Order) db.FlowerRequestCandidate {
	if order == nil {
		return db.FlowerRequestCandidate{}
	}
	return db.FlowerRequestCandidate{
		OrderID: order.OrderID, CookieID: order.CookieID, ChatID: order.ChatID,
		BuyerID: order.BuyerID, ItemID: order.ItemID, ShippedAt: order.ShippedAt,
		PaidAt: order.PaidAt, OrderStatus: order.OrderStatus,
	}
}

// redFlowerRequestScheduleTime 计算发货成功后的到期时间，并拒绝退款、取消、缺少时间和过期订单。
func redFlowerRequestScheduleTime(candidate db.FlowerRequestCandidate, delaySeconds int, now time.Time) (time.Time, bool) {
	if delaySeconds < 0 || delaySeconds > maxFlowerDelaySeconds {
		return time.Time{}, false
	}
	// normalizedStatus 是当前订单的稳定状态。
	normalizedStatus := db.NormalizeOrderStatus(candidate.OrderStatus)
	if normalizedStatus != "shipped" && normalizedStatus != "received" && normalizedStatus != "completed" {
		return time.Time{}, false
	}
	// shippedAt、paidAt 分别是发货成功和官方求花期限的时间事实。
	shippedAt, paidAt := parseDBTime(candidate.ShippedAt), parseDBTime(candidate.PaidAt)
	if shippedAt.IsZero() || paidAt.IsZero() || now.Before(paidAt) || now.Sub(paidAt) > redFlowerRequestLimit {
		return time.Time{}, false
	}
	// dueAt 是发货成功时间加账号配置秒数后的持久化到期点。
	dueAt := shippedAt.Add(time.Duration(delaySeconds) * time.Second)
	if dueAt.Before(now) {
		return now, true
	}
	return dueAt, true
}

// scheduledFlowerRequestKey 返回秒级持久化任务的稳定键，供测试和运维查询。
func scheduledFlowerRequestKey(accountID, orderID string) string {
	return accountID + ":" + TriggerRedFlowerRequestDue + ":" + orderID
}
