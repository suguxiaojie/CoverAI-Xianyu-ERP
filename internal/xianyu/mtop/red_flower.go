package mtop

import (
	"context"
	"errors"
	"strings"
)

const (
	// RedFlowerRequestAPI 是闲鱼卖家求花与买家送花共用的官方 MTOP 端点。
	RedFlowerRequestAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idlemessage.red.flower/1.0/"
	// redFlowerRequestAPIName 是签名查询参数使用的官方 API 名称。
	redFlowerRequestAPIName = "mtop.taobao.idlemessage.red.flower"
)

// RedFlowerRequestResult 保存平台明确返回的求花结果和响应 Cookie，不包含账号凭证明文。
type RedFlowerRequestResult struct {
	// Success 表示平台已经接受本次订单求花动作。
	Success bool
	// Message 是平台返回的非敏感业务提示。
	Message string
	// UpdatedCookies 是响应合并后的 Cookie，仅在调用栈内交给凭证持久化边界。
	UpdatedCookies string
}

// RequestRedFlower 请求闲鱼向指定订单买家发送官方求花卡片。
// ctx 控制单次平台请求，cookiesStr 只在请求栈内使用，orderID 是卖家订单号，channel 是官方页面可选渠道。
func (c *ClientImpl) RequestRedFlower(ctx context.Context, cookiesStr, orderID, channel string) (*RedFlowerRequestResult, error) {
	// normalizedOrderID 是去除空白后的平台订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil, errors.New("求花订单 ID 不能为空")
	}
	// data 是官方 H5 当前提交的最小求花参数；channel 允许为空且不得猜测固定值。
	data := map[string]any{"orderId": normalizedOrderID, "channel": strings.TrimSpace(channel)}
	// decoded、updatedCookies、requestErr 分别是平台响应、响应 Cookie 和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(
		ctx,
		cookiesStr,
		firstNonEmptyURL(c.RedFlowerURL, RedFlowerRequestAPI),
		redFlowerRequestAPIName,
		"1.0",
		data,
		"https://h5.m.goofish.com/",
	)
	if requestErr != nil {
		return nil, requestErr
	}
	if decoded == nil {
		return nil, errors.New("求花接口未返回结果")
	}
	// success 是平台 data.success 的兼容布尔值。
	success := redFlowerSuccess(decoded.Data["success"])
	// message 优先使用平台业务提示，缺失时使用 ret 摘要。
	message := findStringField(decoded.Data, "message", "msg", "tips", "desc")
	if message == "" {
		if success {
			message = "求花成功"
		} else {
			message = firstRet(decoded.Ret)
		}
	}
	return &RedFlowerRequestResult{Success: success, Message: message, UpdatedCookies: updatedCookies}, nil
}

// redFlowerSuccess 兼容平台 success 的布尔、数字和字符串表达。
// value 是平台响应字段；返回值只在明确真值时为 true。
func redFlowerSuccess(value any) bool {
	switch // typed 是平台 success 字段的实际类型。
	typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed == 1
	case string:
		// normalizedValue 是去空白并统一大小写后的响应值。
		normalizedValue := strings.ToLower(strings.TrimSpace(typed))
		return normalizedValue == "true" || normalizedValue == "1" || normalizedValue == "success"
	default:
		return false
	}
}
