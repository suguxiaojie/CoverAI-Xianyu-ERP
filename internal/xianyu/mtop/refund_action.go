package mtop

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// RefundActionResult 保存平台明确受理的退款动作和响应 Cookie。
type RefundActionResult struct {
	// Success 只在 MTOP 明确成功时为真。
	Success bool
	// Message 是不含凭证的用户提示。
	Message string
	// RequiresOfficial 表示前置 MTOP 只返回原生认证地址，不能视为退款成功。
	RequiresOfficial bool
	// UpdatedCookies 是响应协调后的平面 Cookie，只能留在凭证调用栈内。
	UpdatedCookies string
}

// SubmitRefundAction 使用刚从官方详情读取的动态描述执行普通同意／拒绝退款动作。
func (c *ClientImpl) SubmitRefundAction(ctx context.Context, cookiesStr, orderID, refundID string, action RefundAction) (*RefundActionResult, error) {
	// normalizedOrderID、normalizedRefundID 是去空白后的订单和退款申请标识。
	normalizedOrderID, normalizedRefundID := strings.TrimSpace(orderID), strings.TrimSpace(refundID)
	if normalizedOrderID == "" || normalizedRefundID == "" {
		return nil, errors.New("退款动作缺少订单或退款申请 ID")
	}
	if // descriptorErr 是动态退款动作名称、版本和参数体积校验错误。
	descriptorErr := validateRefundActionDescriptor(action); descriptorErr != nil {
		return nil, descriptorErr
	}
	// params 是平台动态参数的独立副本，并使用最新详情的退款标识覆盖旧值。
	params := cloneRefundParams(action.Params)
	params["refundId"] = normalizedRefundID
	if // _, exists 表示平台动作是否原本要求订单号。
	_, exists := params["orderId"]; exists {
		params["orderId"] = normalizedOrderID
	}
	// endpoint 是测试覆盖地址或固定闲鱼 H5API 域名下的动态路径。
	endpoint := strings.TrimSpace(c.RefundActionURL)
	if endpoint == "" {
		// target 只使用已校验的 API 名称和版本构造官方地址。
		target := &url.URL{Scheme: "https", Host: "h5api.m.goofish.com", Path: "/h5/" + strings.ToLower(action.APIName) + "/" + action.APIVersion + "/"}
		endpoint = target.String()
	}
	// decoded、updatedCookies、requestErr 是平台动作响应、Cookie 变化和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr, endpoint, action.APIName, action.APIVersion, params, refundDetailReferer)
	if requestErr != nil {
		return &RefundActionResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// iframeURL 是闲鱼要求通过 WindVane／支付宝支付密码继续认证的前置地址。
	iframeURL := strings.TrimSpace(findStringField(decoded.Data, "iframeUrl"))
	if iframeURL != "" {
		return &RefundActionResult{RequiresOfficial: true, Message: "该退款需要在闲鱼官方完成支付密码验证", UpdatedCookies: updatedCookies}, nil
	}
	// message 是平台成功提示；缺失时按动作方向使用安全默认说明。
	message := strings.TrimSpace(findStringField(decoded.Data, "message", "msg", "tips", "toast", "desc"))
	if message == "" {
		if action.Kind == "agree" {
			message = "已同意退款申请"
		} else {
			message = "已拒绝退款申请"
		}
	}
	return &RefundActionResult{Success: true, Message: message, UpdatedCookies: updatedCookies}, nil
}
