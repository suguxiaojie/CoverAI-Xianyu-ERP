package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xianyu-go/internal/db"
)

// HandleTask 处理一条自动化任务。无匹配规则时安全忽略。
func (c *Center) HandleTask(ctx context.Context, task Task) error {
	// err 是事件事实、专用小红花协调或规则动作返回的处理错误。
	_, err := c.handleTask(ctx, task)
	return err
}

// buildTriggerKey 根据触发类型和订单／平台更新键构造稳定事件幂等键。
func buildTriggerKey(task Task) string {
	if task.TriggerType == TriggerReviewMissingTimeout && task.OrderID != "" {
		if // attempt、ok 是求评价任务的稳定尝试序号和存在标记。
		attempt, ok := task.Raw["attempt"]; ok {
			return fmt.Sprintf("%s:%s:%v", task.TriggerType, task.OrderID, attempt)
		}
	}
	if task.OrderID != "" {
		return task.TriggerType + ":" + task.OrderID
	}
	if task.UpdateKey != "" {
		return task.TriggerType + ":" + task.UpdateKey
	}
	return ""
}

// eventFactRecorder 只负责把已经解析出的事件事实写入持久化层。
// 它不读取规则、不创建运行记录，也不执行任何外部动作。
// eventFactRecorder 用于本次流程后续判断的eventFactRecorder
type eventFactRecorder struct {
	// store 提供订单事实和事件时间的持久化能力。
	store *db.Store
}

// newEventFactRecorder 构造事件事实记录组件。
func newEventFactRecorder(store *db.Store) eventFactRecorder {
	return eventFactRecorder{store: store}
}

// record 将自动化任务中的订单字段和触发时间写入事实表。
func (r eventFactRecorder) record(ctx context.Context, task Task) error {
	if r.store == nil || r.store.Orders == nil || r.store.Automation == nil || task.OrderID == "" {
		return nil
	}
	if task.TriggerType == TriggerRedFlowerRequestDue {
		// 秒级任务携带的是发货时快照；执行时只能读取当前订单，禁止把旧 shipped 状态写回退款或取消订单。
		return nil
	}
	// promotedBuyerShadow 标记本轮已完成买家影子订单接管；后续通用 UPSERT 不得用卡片摘要覆盖既有商品字段。
	promotedBuyerShadow := false
	if task.TriggerType == TriggerOrderPaid && strings.TrimSpace(task.BuyerID) != "" && strings.TrimSpace(task.AccountID) != strings.TrimSpace(task.BuyerID) {
		// promoted、promotionErr 是同一用户买家影子订单被卖家付款卡片安全接管的结果；未命中不影响正常新订单插入。
		promoted, promotionErr := r.store.Orders.PromoteBuyerShadowOrder(ctx, db.PromoteBuyerShadowOrderInput{
			OrderID: task.OrderID, BuyerAccountID: task.BuyerID, SellerAccountID: task.AccountID,
			ChatID: task.ChatID, ItemID: task.ItemID,
		})
		if promotionErr != nil {
			return fmt.Errorf("接管买家影子订单: %w", promotionErr)
		}
		promotedBuyerShadow = promoted
	}
	// itemID 是普通新订单使用的卡片商品标识；接管场景已经只在原字段为空时补齐，这里留空以保留既有商品。
	itemID := task.ItemID
	if promotedBuyerShadow {
		itemID = ""
	}
	// buyerID 是当前事件允许写入的真实买家；卖家自身回显不能覆盖既有买家关联。
	buyerID := strings.TrimSpace(task.BuyerID)
	if buyerID == strings.TrimSpace(task.AccountID) {
		buyerID = ""
	}
	if // err 用于本次流程后续判断的err
	err := r.store.Orders.Upsert(ctx, task.OrderID, db.OrderUpsertOpts{
		CookieID:    task.AccountID,
		ItemID:      itemID,
		BuyerID:     buyerID,
		ChatID:      task.ChatID,
		OrderStatus: task.OrderStatus,
		SpecName:    task.SpecName,
		SpecValue:   task.SpecValue,
		Quantity:    task.Quantity,
		Amount:      task.Amount,
	}); err != nil {
		return fmt.Errorf("记录自动化事件订单事实: %w", err)
	}
	switch task.TriggerType {
	case TriggerOrderPaid:
		if // err 用于本次流程后续判断的err
		err := r.store.Automation.MarkOrderEventTime(ctx, task.OrderID, "paid_at"); err != nil {
			return fmt.Errorf("记录订单付款时间: %w", err)
		}
	case TriggerOrderShipped:
		// occurredAt 是平台发货卡片时间；缺失时 MarkOrderEventTimeAt 回退到当前 UTC。
		occurredAt := time.Time{}
		if task.OccurredAt > 0 {
			occurredAt = time.UnixMilli(task.OccurredAt).UTC()
		}
		if // err 是平台发货时间写入订单事实的错误。
		err := r.store.Automation.MarkOrderEventTimeAt(ctx, task.OrderID, "shipped_at", occurredAt); err != nil {
			return fmt.Errorf("记录订单发货时间: %w", err)
		}
	case TriggerOrderReceived, TriggerOrderCompleted, TriggerRefundCompleted, TriggerOrderCancelled:
		// occurredAt 是当前生命周期卡片的平台时间；缺失时持久化层回退到当前 UTC。
		occurredAt := time.Time{}
		if task.OccurredAt > 0 {
			occurredAt = time.UnixMilli(task.OccurredAt).UTC()
		}
		// eventField 是当前生命周期事件唯一允许写入的里程碑列。
		eventField := map[string]string{TriggerOrderReceived: "received_at", TriggerOrderCompleted: "completed_at", TriggerRefundCompleted: "refunded_at", TriggerOrderCancelled: "cancelled_at"}[task.TriggerType]
		if task.TriggerType == TriggerRefundCompleted || task.TriggerType == TriggerOrderCancelled {
			// currentOrder、readErr 确认分支终态确实成为订单当前状态，避免迟到事件写入相反里程碑。
			currentOrder, readErr := r.store.Orders.Get(ctx, task.OrderID)
			if readErr != nil {
				return fmt.Errorf("复核订单生命周期终态: %w", readErr)
			}
			if db.NormalizeOrderStatus(currentOrder.OrderStatus) != task.OrderStatus {
				return nil
			}
		}
		if // err 是生命周期事件时间写入订单事实的错误。
		err := r.store.Automation.MarkOrderEventTimeAt(ctx, task.OrderID, eventField, occurredAt); err != nil {
			return fmt.Errorf("记录订单生命周期时间: %w", err)
		}
	case TriggerBuyerReviewed:
		if // err 用于本次流程后续判断的err
		err := r.store.Automation.MarkOrderEventTime(ctx, task.OrderID, "buyer_reviewed_at"); err != nil {
			return fmt.Errorf("记录买家评价时间: %w", err)
		}
	}
	return nil
}

// ruleMatcher 只查询适用于任务的规则，不执行规则动作或修改运行状态。
type ruleMatcher struct {
	// store 提供规则查询和恢复运行读取能力。
	store *db.Store
}

// newRuleMatcher 构造无动作副作用的规则匹配组件。
func newRuleMatcher(store *db.Store) ruleMatcher {
	return ruleMatcher{store: store}
}

// match 查询普通事件或恢复运行对应的规则快照。
func (m ruleMatcher) match(ctx context.Context, task Task) ([]db.AutomationRule, error) {
	if m.store == nil || m.store.Automation == nil {
		return nil, nil
	}
	if // runID 用于本次流程后续判断的运行ID
	runID := taskAutomationRunID(task); runID > 0 {
		// run、err 用于本次流程后续判断的run、err
		run, err := m.store.Automation.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		if run.Status != "running" {
			return nil, nil
		}
		// rule、err 用于本次流程后续判断的rule、err
		rule, err := m.store.Automation.Get(ctx, run.RuleID)
		if err != nil {
			return nil, err
		}
		if rule == nil {
			return nil, nil
		}
		return []db.AutomationRule{*rule}, nil
	}
	return m.store.Automation.Match(ctx, task.AccountID, task.ItemID, task.TriggerType)
}

// actionPlanner 只根据任务事实和规则动作生成不可变的动作计划。
// 计划过程不得访问数据库、发送网络请求或修改规则。
// actionPlanner 用于本次流程后续判断的动作Planner
type actionPlanner struct{}

// plan 根据触发类型筛选可执行动作，并保留付款事件的发卡优先顺序。
func (actionPlanner) plan(task Task, actions []db.AutomationAction) []db.AutomationAction {
	// out 用于本次流程后续判断的out
	out := make([]db.AutomationAction, 0, len(actions))
	if task.TriggerType == TriggerOrderPaid {
		// action 表示当前遍历过程中的动作
		for _, action := range actions {
			if action.Enabled && action.ActionType == ActionSendCard && actionMatchesOrderSpec(task, action) {
				out = append(out, action)
			}
		}
		// action 表示当前遍历过程中的动作
		for _, action := range actions {
			if action.Enabled && action.ActionType == ActionConfirmShipment {
				out = append(out, action)
			}
		}
		return out
	}
	// action 表示当前遍历过程中的动作
	for _, action := range actions {
		if action.Enabled {
			out = append(out, action)
		}
	}
	return out
}

// hasMatchingSendCard 判断付款事件是否存在匹配当前规格的发卡动作。
func (actionPlanner) hasMatchingSendCard(task Task, actions []db.AutomationAction) bool {
	// action 表示当前遍历过程中的动作
	for _, action := range actions {
		if action.Enabled && action.ActionType == ActionSendCard && actionMatchesOrderSpec(task, action) {
			return true
		}
	}
	return false
}

// immediateManualActions 复制动作并清除延迟，供明确的人工完整发货使用。
func (actionPlanner) immediateManualActions(actions []db.AutomationAction) []db.AutomationAction {
	// out 用于本次流程后续判断的out
	out := make([]db.AutomationAction, len(actions))
	copy(out, actions)
	// i 表示当前遍历过程中的i
	for i := range out {
		out[i].DelaySeconds = 0
		if out[i].ActionType != ActionSendCard {
			continue
		}
		// config 用于本次流程后续判断的配置
		config := map[string]any{}
		_ = json.Unmarshal([]byte(out[i].ConfigJSON), &config)
		config["delay_override"] = true
		// raw 用于本次流程后续判断的原始
		raw, _ := json.Marshal(config)
		out[i].ConfigJSON = string(raw)
	}
	return out
}
