package mtop

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	// CloseOrderReasonsAPI 是闲鱼网页版动态读取当前订单可选关闭原因的端点。
	CloseOrderReasonsAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idle.trade.order.close.reason.get/1.0/"
	// CloseOrderSellerAPI 是闲鱼网页版卖家关闭交易的端点。
	CloseOrderSellerAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idle.trade.close.by.seller/2.0/"
	// closeOrderReasonsAPIName 是关闭原因请求的签名 API 名称。
	closeOrderReasonsAPIName = "mtop.taobao.idle.trade.order.close.reason.get"
	// closeOrderSellerAPIName 是卖家关单请求的签名 API 名称。
	closeOrderSellerAPIName = "mtop.taobao.idle.trade.close.by.seller"
	// closeOrderReferer 是网页版订单操作使用的来源页面。
	closeOrderReferer = "https://www.goofish.com/"
)

// CloseOrderReasonsResult 保存平台动态关闭原因和响应 Cookie，不包含订单外的敏感数据。
type CloseOrderReasonsResult struct {
	// Reasons 是平台当前允许卖家选择的关闭原因，保持返回顺序并去重。
	Reasons []string
	// UpdatedCookies 是本次请求协调后的平面 Cookie，只能留在凭证调用栈内。
	UpdatedCookies string
}

// CloseOrderSellerResult 保存卖家关单的明确平台结果和响应 Cookie。
type CloseOrderSellerResult struct {
	// Success 只在 MTOP ret 明确成功时为真。
	Success bool
	// Message 是不含凭证的用户提示。
	Message string
	// UpdatedCookies 是本次请求协调后的平面 Cookie，只能留在凭证调用栈内。
	UpdatedCookies string
}

// FetchCloseOrderReasons 读取指定订单当前可用的动态关闭原因，不产生关单外部写入。
func (c *ClientImpl) FetchCloseOrderReasons(ctx context.Context, cookiesStr, orderID string) (*CloseOrderReasonsResult, error) {
	// normalizedOrderID 是去空白后的数字订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil, errors.New("关闭订单 ID 不能为空")
	}
	// decoded、updatedCookies、requestErr 是平台响应、Cookie 变化和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.CloseOrderReasonsURL, CloseOrderReasonsAPI), closeOrderReasonsAPIName, "1.0",
		map[string]any{"bizOrderId": normalizedOrderID}, closeOrderReferer)
	if requestErr != nil {
		return &CloseOrderReasonsResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// rawReasons 是平台返回的动态关闭原因列表。
	rawReasons, _ := decoded.Data["closeReasons"].([]any)
	// reasons 保存去空白、去重且仍保持平台顺序的原因。
	reasons := make([]string, 0, len(rawReasons))
	// seen 防止平台重复原因在前端产生重复选项。
	seen := make(map[string]struct{}, len(rawReasons))
	// rawReason 是当前待规范的平台原因。
	for _, rawReason := range rawReasons {
		// reason 是当前平台原因的去空白文本。
		reason := strings.TrimSpace(fmt.Sprint(rawReason))
		if reason == "" || reason == "<nil>" || len([]rune(reason)) > 100 {
			continue
		}
		if // _, exists 表示当前原因已经加入结果。
		_, exists := seen[reason]; exists {
			continue
		}
		seen[reason] = struct{}{}
		reasons = append(reasons, reason)
	}
	if len(reasons) == 0 {
		return &CloseOrderReasonsResult{UpdatedCookies: updatedCookies}, errors.New("闲鱼未返回可用的关闭原因")
	}
	return &CloseOrderReasonsResult{Reasons: reasons, UpdatedCookies: updatedCookies}, nil
}

// CloseOrderBySeller 使用平台动态原因取消卖家待付款或已付款待发货订单；调用方必须先完成归属、状态、原因和幂等校验。
func (c *ClientImpl) CloseOrderBySeller(ctx context.Context, cookiesStr, orderID, reason string) (*CloseOrderSellerResult, error) {
	// normalizedOrderID、normalizedReason 是去空白后的订单号和平台原因。
	normalizedOrderID, normalizedReason := strings.TrimSpace(orderID), strings.TrimSpace(reason)
	if normalizedOrderID == "" || normalizedReason == "" || len([]rune(normalizedReason)) > 100 {
		return nil, errors.New("关闭订单需要有效订单 ID 和原因")
	}
	// decoded、updatedCookies、requestErr 是平台响应、Cookie 变化和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.CloseOrderSellerURL, CloseOrderSellerAPI), closeOrderSellerAPIName, "2.0",
		map[string]any{"tid": normalizedOrderID, "closeReason": normalizedReason, "bizOrderId": normalizedOrderID}, closeOrderReferer)
	if requestErr != nil {
		return &CloseOrderSellerResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// message 是平台成功提示；网页成功响应没有文案时使用固定说明。
	message := strings.TrimSpace(findStringField(decoded.Data, "message", "msg", "tips", "desc"))
	if message == "" {
		message = "订单已关闭"
	}
	return &CloseOrderSellerResult{Success: true, Message: message, UpdatedCookies: updatedCookies}, nil
}
