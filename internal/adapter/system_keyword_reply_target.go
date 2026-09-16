package adapter

import (
	"context"
	"errors"
	"strings"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/engine"
)

// systemKeywordReplyAssociationWindow 限制无订单号买家通知只能关联前后十分钟内的已发货订单。
const systemKeywordReplyAssociationWindow = 10 * time.Minute

// ResolveSystemKeywordReplyTarget 按精确订单号或同会话唯一近期发货订单解析买家目标。
func (a *Adapter) ResolveSystemKeywordReplyTarget(ctx context.Context, request engine.SystemKeywordReplyTargetRequest) (engine.SystemKeywordReplyTarget, bool, error) {
	if a == nil || a.store == nil || a.store.Orders == nil || strings.TrimSpace(request.AccountID) == "" || strings.TrimSpace(request.EventType) != "order_shipped" {
		return engine.SystemKeywordReplyTarget{}, false, nil
	}
	// orderID 是平台卖家卡片可能直接提供的精确订单标识。
	orderID := strings.TrimSpace(request.OrderID)
	if orderID != "" {
		// order 是当前账号精确订单读取结果。
		order, orderErr := a.store.Orders.Get(ctx, orderID)
		if orderErr != nil {
			if errors.Is(orderErr, db.ErrNotFound) {
				return engine.SystemKeywordReplyTarget{}, false, nil
			}
			return engine.SystemKeywordReplyTarget{}, false, orderErr
		}
		// target 是同时通过账号、会话和买家非空门禁的发送目标。
		target, matched := systemKeywordTargetFromOrder(request, order)
		return target, matched, nil
	}
	if strings.TrimSpace(request.ChatID) == "" {
		return engine.SystemKeywordReplyTarget{}, false, nil
	}
	// candidates 是当前账号同会话最近的已发货订单，最多三笔用于歧义判断。
	candidates, candidatesErr := a.store.Orders.ShippedByChat(ctx, request.AccountID, request.ChatID, 3)
	if candidatesErr != nil {
		return engine.SystemKeywordReplyTarget{}, false, candidatesErr
	}
	// occurredAt 是平台系统通知时间；缺失时使用当前接收时间。
	occurredAt := time.Now().UTC()
	if request.OccurredAt > 0 {
		occurredAt = time.UnixMilli(request.OccurredAt).UTC()
	}
	// matches 保存通过买家、账号、会话和十分钟发货窗口的候选。
	matches := make([]engine.SystemKeywordReplyTarget, 0, len(candidates))
	// candidate 是当前待校验的近期已发货订单。
	for _, candidate := range candidates {
		if strings.TrimSpace(request.BuyerID) != "" && strings.TrimSpace(request.BuyerID) != strings.TrimSpace(candidate.BuyerID) {
			continue
		}
		// shippedAt 是候选订单的可解析 UTC 发货时间。
		shippedAt := parseSystemKeywordReplyTime(candidate.ShippedAt)
		if shippedAt.IsZero() || occurredAt.Sub(shippedAt) < -systemKeywordReplyAssociationWindow || occurredAt.Sub(shippedAt) > systemKeywordReplyAssociationWindow {
			continue
		}
		// order 是把最小候选投影为统一目标校验所需的订单字段。
		order := &db.Order{OrderID: candidate.OrderID, CookieID: request.AccountID, ChatID: candidate.ChatID, BuyerID: candidate.BuyerID}
		// target 和 matched 是当前候选通过统一订单门禁后的回复目标及匹配标记。
		if target, matched := systemKeywordTargetFromOrder(request, order); matched {
			matches = append(matches, target)
		}
	}
	if len(matches) != 1 {
		return engine.SystemKeywordReplyTarget{}, false, nil
	}
	return matches[0], true, nil
}

// systemKeywordTargetFromOrder 校验订单归属、会话和买家后投影回复目标。
func systemKeywordTargetFromOrder(request engine.SystemKeywordReplyTargetRequest, order *db.Order) (engine.SystemKeywordReplyTarget, bool) {
	if order == nil || strings.TrimSpace(order.OrderID) == "" || strings.TrimSpace(order.CookieID) != strings.TrimSpace(request.AccountID) || strings.TrimSpace(order.BuyerID) == "" || strings.TrimSpace(order.BuyerID) == strings.TrimSpace(request.AccountID) {
		return engine.SystemKeywordReplyTarget{}, false
	}
	if strings.TrimSpace(request.ChatID) != "" && strings.TrimSpace(order.ChatID) != "" && strings.TrimSpace(request.ChatID) != strings.TrimSpace(order.ChatID) {
		return engine.SystemKeywordReplyTarget{}, false
	}
	// chatID 优先使用订单持久化会话，空值时回退实时卡片会话。
	chatID := strings.TrimSpace(order.ChatID)
	if chatID == "" {
		chatID = strings.TrimSpace(request.ChatID)
	}
	if chatID == "" {
		return engine.SystemKeywordReplyTarget{}, false
	}
	return engine.SystemKeywordReplyTarget{OrderID: strings.TrimSpace(order.OrderID), ChatID: chatID, BuyerID: strings.TrimSpace(order.BuyerID)}, true
}

// parseSystemKeywordReplyTime 兼容订单表 RFC3339 和 SQLite 空格时间格式。
func parseSystemKeywordReplyTime(value string) time.Time {
	// layout 是当前待尝试的订单时间格式。
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		// parsed 和 parseErr 是当前格式解析结果和错误。
		if parsed, parseErr := time.Parse(layout, strings.TrimSpace(value)); parseErr == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

// 编译期确认 Adapter 实现 Engine 的系统关键词订单目标解析端口。
var _ engine.SystemKeywordReplyTargetResolver = (*Adapter)(nil)
