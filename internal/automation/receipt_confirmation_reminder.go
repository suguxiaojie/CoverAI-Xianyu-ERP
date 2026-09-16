package automation

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// receiptReminderPageSize 限制单次数据库分页读取数量；每个账号每轮最多真正发送一条，避免消息突发。
const receiptReminderPageSize = 200

// scanReceiptConfirmationReminders 分页寻找当前账号第一笔到期且未发送的订单，并在发送前再次复核状态。
func (c *accountTaskCoordinator) scanReceiptConfirmationReminders(ctx context.Context, settings db.AccountTaskSettings, now time.Time) error {
	if c == nil || c.repository == nil || c.client == nil || c.client() == nil {
		return errors.New("自动确认收货提醒依赖未初始化")
	}
	// requester、available 是账号任务客户端的官方提醒确认收货系统卡片能力和可用状态。
	requester, available := c.client().(receiptConfirmationReminderClient)
	if !available || requester == nil {
		return errors.New("官方确认收货提醒客户端未初始化")
	}
	// currentCookies、cookieErr 是本轮第一个官方请求使用的账号 Cookie 和读取错误。
	currentCookies, cookieErr := c.repository.GetValue(ctx, settings.CookieID)
	if cookieErr != nil {
		return cookieErr
	}
	// afterOrderID 是稳定分页游标，防止旧订单长期阻塞后续候选。
	afterOrderID := ""
	for {
		// candidates、queryErr 是当前页确认收货提醒候选和查询错误。
		candidates, queryErr := c.repository.ListReceiptReminderCandidates(ctx, settings.CookieID, afterOrderID, receiptReminderPageSize)
		if queryErr != nil {
			return queryErr
		}
		// candidate 是当前待判断的已发货订单。
		for _, candidate := range candidates {
			if !receiptConfirmationReminderDue(candidate, settings, now) {
				continue
			}
			// runKey 是同账号同订单永久唯一的提醒运行键，确保成功、失败不明和重启后都不重复发送。
			runKey := receiptConfirmationReminderRunKey(settings.CookieID, candidate.OrderID)
			// existing、exists、runErr 先跳过任何已有运行，避免 failed 或 needs_review 被 ClaimRun 自动重试。
			existing, exists, runErr := c.repository.GetRunByKey(ctx, runKey)
			if runErr != nil {
				return runErr
			}
			if exists && (existing.Status != "failed" || existing.NextRetryAt <= 0 || existing.NextRetryAt > now.UTC().Unix()) {
				continue
			}
			// latest、latestErr 在抢占运行前重新确认订单仍为已发货并读取最新唯一聊天目标。
			latest, latestErr := c.repository.GetReceiptReminderCandidate(ctx, settings.CookieID, candidate.OrderID)
			if errors.Is(latestErr, db.ErrNotFound) {
				continue
			}
			if latestErr != nil {
				return latestErr
			}
			if !receiptConfirmationReminderDue(latest, settings, now) || strings.TrimSpace(latest.ChatID) == "" || strings.TrimSpace(latest.BuyerID) == "" {
				continue
			}
			// claimed、claimErr 原子创建 running 记录；进程中断时启动流程会将其隔离，禁止自动重放。
			claimed, claimErr := c.repository.ClaimRun(ctx, db.AccountTaskRun{
				RunKey: runKey, CookieID: settings.CookieID, TaskType: TaskReceiptReminder,
				TargetID: latest.OrderID, RunDate: now.Format("2006-01-02"),
			}, now.UTC().Unix())
			if claimErr != nil {
				return claimErr
			}
			if !claimed {
				continue
			}
			// finalCandidate、finalErr 在已持久化运行后做最后一次订单状态门禁，消除领取和发送之间的状态竞态。
			finalCandidate, finalErr := c.repository.GetReceiptReminderCandidate(ctx, settings.CookieID, latest.OrderID)
			if errors.Is(finalErr, db.ErrNotFound) || finalErr == nil && !receiptConfirmationReminderDue(finalCandidate, settings, now) {
				return c.finishAccountTaskRun(ctx, runKey, "cancelled", 0, 0, "订单已离开可提醒的已发货状态", 0)
			}
			if finalErr != nil {
				return c.quarantineAccountTaskRun(ctx, runKey, 0, 0, finalErr)
			}
			if strings.TrimSpace(finalCandidate.ChatID) == "" || strings.TrimSpace(finalCandidate.BuyerID) == "" {
				return c.finishAccountTaskRun(ctx, runKey, "cancelled", 0, 0, "订单缺少唯一聊天目标", 0)
			}
			// sendCtx、cancel 把单条官方系统卡片请求限制在三十秒，超时或传输错误统一视为结果不明确。
			sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			// result、sendErr 是闲鱼是否明确生成系统卡片的结果和请求错误。
			result, sendErr := requester.RemindBuyerConfirmReceipt(sendCtx, currentCookies, finalCandidate.OrderID)
			cancel()
			if sendErr != nil {
				if mtop.IsSessionExpiredErr(sendErr) {
					// retryAt 是凭证恢复后允许安全重试官方请求的时间。
					retryAt := now.UTC().Add(10 * time.Minute).Unix()
					// finishErr 把平台明确拒绝的失效会话记录为可安全重试失败。
					finishErr := c.finishAccountTaskRun(ctx, runKey, "failed", 0, 1, db.SafeRetryErrorPrefix+sendErr.Error(), retryAt)
					return errors.Join(sendErr, finishErr)
				}
				return c.quarantineAccountTaskRun(ctx, runKey, 0, 1, sendErr)
			}
			if result == nil || !result.Success {
				// message 是平台明确拒绝当前订单提醒时的非敏感原因。
				message := "闲鱼未接受本次确认收货提醒"
				if result != nil && strings.TrimSpace(result.Message) != "" {
					message = strings.TrimSpace(result.Message)
				}
				return c.finishAccountTaskRun(ctx, runKey, "cancelled", 0, 1, message, 0)
			}
			// persistErr 是官方接口成功后响应 Cookie 的安全持久化错误。
			_, persistErr := c.persistTaskCookies(ctx, settings.CookieID, currentCookies, result.UpdatedCookies)
			if persistErr != nil {
				return c.quarantineAccountTaskRun(ctx, runKey, 1, 0, persistErr)
			}
			// finishErr 保存明确发送完成的成功状态；保存失败时 helper 会立即隔离，避免下一轮重复发送。
			finishErr := c.finishAccountTaskRun(ctx, runKey, "success", 1, 0, "", 0)
			if finishErr != nil {
				c.notifyReceiptReminderTask(ctx, runKey, finalCandidate, "needs_review", "闲鱼已生成确认收货提醒系统卡片，但本地状态保存失败，请人工核对")
				return finishErr
			}
			c.notifyReceiptReminderTask(ctx, runKey, finalCandidate, "success", "闲鱼已生成确认收货提醒系统卡片")
			return nil
		}
		if len(candidates) < receiptReminderPageSize {
			return nil
		}
		afterOrderID = candidates[len(candidates)-1].OrderID
	}
}

// receiptConfirmationReminderDue 判断订单是否在本次开启基线之后发货，并已到达完整天数后的下一次北京时间。
func receiptConfirmationReminderDue(candidate db.ReceiptReminderCandidate, settings db.AccountTaskSettings, now time.Time) bool {
	if !settings.AutoReceiptReminderEnabled || settings.ReceiptReminderEnabledAt <= 0 || settings.ReceiptReminderAfterDays < 1 || settings.ReceiptReminderAfterDays > 30 {
		return false
	}
	if db.NormalizeOrderStatus(candidate.OrderStatus) != "shipped" || strings.TrimSpace(candidate.ChatID) == "" || strings.TrimSpace(candidate.BuyerID) == "" {
		return false
	}
	// shippedAt 是按现有三方言兼容格式解析出的可靠发货时间。
	shippedAt := parseDBTime(candidate.ShippedAt)
	if shippedAt.IsZero() || shippedAt.Unix() < settings.ReceiptReminderEnabledAt || now.Before(shippedAt) {
		return false
	}
	// timeParts 是配置的北京时间小时和分钟；应用层已验证格式，这里仍以失败关闭保护调度器。
	timeParts := strings.Split(settings.ReceiptReminderTime, ":")
	if len(timeParts) != 2 {
		return false
	}
	// hour、hourErr 是每日执行小时和解析错误。
	hour, hourErr := strconv.Atoi(timeParts[0])
	// minute、minuteErr 是每日执行分钟和解析错误。
	minute, minuteErr := strconv.Atoi(timeParts[1])
	if hourErr != nil || minuteErr != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return false
	}
	// location 固定为北京时间，与账号页面和每日擦亮现有口径一致。
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	// threshold 是发货后完整 N×24 小时门槛。
	threshold := shippedAt.Add(time.Duration(settings.ReceiptReminderAfterDays) * 24 * time.Hour)
	// localThreshold 是门槛在北京时间的自然日表示。
	localThreshold := threshold.In(location)
	// dueAt 是门槛所在日的配置时刻；若早于完整时长，则顺延到下一天同一时刻。
	dueAt := time.Date(localThreshold.Year(), localThreshold.Month(), localThreshold.Day(), hour, minute, 0, 0, location)
	if dueAt.Before(threshold) {
		dueAt = dueAt.AddDate(0, 0, 1)
	}
	return !now.In(location).Before(dueAt)
}

// receiptConfirmationReminderRunKey 返回同账号同订单永久唯一的确认收货提醒幂等键。
func receiptConfirmationReminderRunKey(accountID, orderID string) string {
	return TaskReceiptReminder + ":" + strings.TrimSpace(accountID) + ":" + strings.TrimSpace(orderID)
}

// notifyReceiptReminderTask 将提醒终态交给既有通知去重链路，不发送额外买家消息。
func (c *accountTaskCoordinator) notifyReceiptReminderTask(ctx context.Context, runKey string, candidate db.ReceiptReminderCandidate, status, message string) {
	if c == nil || c.notifier == nil || c.repository == nil {
		return
	}
	// run、exists、runErr 是通知所需的持久化运行、存在状态和读取错误。
	run, exists, runErr := c.repository.GetRunByKey(ctx, runKey)
	if runErr != nil || !exists {
		c.logger.Warn("读取确认收货提醒运行失败", "account", candidate.CookieID, "order_id", candidate.OrderID, "err", runErr)
		return
	}
	c.notifier.NotifyAutomationRun(ctx, run.ID, candidate.CookieID, candidate.BuyerID, candidate.ItemID, status, message, candidate.ChatID)
}
