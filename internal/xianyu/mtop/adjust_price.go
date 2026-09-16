package mtop

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

const (
	// AdjustPriceRenderAPI 是闲鱼网页版读取待付款订单动态改价字段的官方端点。
	AdjustPriceRenderAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idle.trade.order.modify.price.render/1.0/"
	// AdjustPriceSubmitAPI 是闲鱼网页版提交待付款订单金额与运费的官方端点。
	AdjustPriceSubmitAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idle.trade.user.adjust.price/1.0/"
	// adjustPriceRenderAPIName 是 render 请求签名查询参数使用的 API 名称。
	adjustPriceRenderAPIName = "mtop.taobao.idle.trade.order.modify.price.render"
	// adjustPriceSubmitAPIName 是 submit 请求签名查询参数使用的 API 名称。
	adjustPriceSubmitAPIName = "mtop.taobao.idle.trade.user.adjust.price"
	// adjustPriceReferer 是真实网页抓包确认的 Origin／Referer 基址。
	adjustPriceReferer = "https://www.goofish.com/"
)

// adjustPriceFieldKeyPattern 只允许网页版 render 返回的普通标识符进入 submit data。
var adjustPriceFieldKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

// AdjustPriceField 描述闲鱼 render 返回的一个动态金额字段，Price 使用元字符串。
type AdjustPriceField struct {
	// Key 是 submit data 使用的平台字段名。
	Key string
	// Name 是面向用户展示的字段名称。
	Name string
	// PrefixText 是金额输入前缀，真实网页通常为人民币符号。
	PrefixText string
	// Price 是当前平台金额，单位为元的十进制字符串。
	Price string
	// ReadOnly 表示该字段是否只能按平台默认值原样提交。
	ReadOnly bool
}

// AdjustPriceFormResult 保存动态改价表单和响应 Cookie，不包含请求签名或账号凭证。
type AdjustPriceFormResult struct {
	// Title 是平台返回的表单标题。
	Title string
	// Fields 是平台要求展示和提交的金额字段。
	Fields []AdjustPriceField
	// UpdatedCookies 是 render 响应协调后的扁平 Cookie，只能留在凭证调用栈内。
	UpdatedCookies string
}

// AdjustPriceSubmitResult 保存平台明确改价结果和响应 Cookie。
type AdjustPriceSubmitResult struct {
	// Success 只在 MTOP 成功且 data.success 明确为真时成立。
	Success bool
	// Message 是平台返回的非敏感提示或安全缺省说明。
	Message string
	// UpdatedCookies 是 submit 响应协调后的扁平 Cookie，只能留在凭证调用栈内。
	UpdatedCookies string
}

// RenderOrderAdjustPrice 读取指定待付款订单当前可编辑金额字段，不产生改价外部写入。
func (c *ClientImpl) RenderOrderAdjustPrice(ctx context.Context, cookiesStr, orderID string) (*AdjustPriceFormResult, error) {
	// normalizedOrderID 是去除空白后的数字订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil, errors.New("改价订单 ID 不能为空")
	}
	// decoded、updatedCookies、requestErr 分别是 render 响应、响应 Cookie 和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.AdjustPriceRenderURL, AdjustPriceRenderAPI), adjustPriceRenderAPIName, "1.0",
		map[string]any{"bizOrderId": normalizedOrderID}, adjustPriceReferer)
	if requestErr != nil {
		// token bootstrap 可能已经更新 Cookie；即使后续 render 失败也必须交给上层安全协调。
		return &AdjustPriceFormResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// rawFields 是平台返回的动态字段列表；缺失时不能猜测固定字段。
	rawFields, _ := decoded.Data["modifyPriceRenderList"].([]any)
	// fields 保存通过字段名和金额基本校验的公开表单。
	fields := make([]AdjustPriceField, 0, len(rawFields))
	// rawField 是当前待解析的平台动态字段。
	for _, rawField := range rawFields {
		// fieldMap、fieldOK 是当前字段对象及其结构判断结果。
		fieldMap, fieldOK := rawField.(map[string]any)
		if !fieldOK {
			continue
		}
		// key、name、price 是当前平台字段的稳定标识、展示名和元金额。
		key, name, price := strings.TrimSpace(findStringField(fieldMap, "key")), strings.TrimSpace(findStringField(fieldMap, "name")), strings.TrimSpace(findStringField(fieldMap, "price"))
		if !adjustPriceFieldKeyPattern.MatchString(key) || name == "" || price == "" || key == "orderId" {
			continue
		}
		fields = append(fields, AdjustPriceField{Key: key, Name: name,
			PrefixText: strings.TrimSpace(findStringField(fieldMap, "prefixText")), Price: price,
			ReadOnly: redFlowerSuccess(fieldMap["readOnly"])})
	}
	if len(fields) == 0 {
		return nil, errors.New("改价接口未返回可用字段")
	}
	return &AdjustPriceFormResult{Title: strings.TrimSpace(findStringField(decoded.Data, "title")), Fields: fields, UpdatedCookies: updatedCookies}, nil
}

// SubmitOrderAdjustPrice 按 render 字段提交整数分字符串；调用方必须先完成归属、状态和金额校验。
func (c *ClientImpl) SubmitOrderAdjustPrice(ctx context.Context, cookiesStr, orderID string, fieldCents map[string]string) (*AdjustPriceSubmitResult, error) {
	// normalizedOrderID 是去除空白后的数字订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil, errors.New("改价订单 ID 不能为空")
	}
	// data 是只包含已校验动态字段和固定 orderId 的 MTOP 请求体。
	data := make(map[string]any, len(fieldCents)+1)
	// key、cents 是当前待提交字段名和非负整数分字符串。
	for key, cents := range fieldCents {
		// normalizedKey、normalizedCents 是去空白后的平台字段和值。
		normalizedKey, normalizedCents := strings.TrimSpace(key), strings.TrimSpace(cents)
		if !adjustPriceFieldKeyPattern.MatchString(normalizedKey) || normalizedKey == "orderId" || normalizedCents == "" {
			return nil, errors.New("改价提交字段无效")
		}
		data[normalizedKey] = normalizedCents
	}
	if len(data) == 0 {
		return nil, errors.New("改价提交字段不能为空")
	}
	data["orderId"] = normalizedOrderID
	// decoded、updatedCookies、requestErr 分别是 submit 响应、响应 Cookie 和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.AdjustPriceSubmitURL, AdjustPriceSubmitAPI), adjustPriceSubmitAPIName, "1.0", data, adjustPriceReferer)
	if requestErr != nil {
		// 本地签名前恢复或平台响应可能已经轮换 Cookie；错误结果仍返回凭证变化但绝不伪造业务成功。
		return &AdjustPriceSubmitResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// success 严格读取 data.success；仅 HTTP 200 或 MTOP ret 成功不足以证明业务改价完成。
	success := redFlowerSuccess(decoded.Data["success"])
	// message 是平台非敏感提示；当前成功响应缺失提示时使用固定说明。
	message := strings.TrimSpace(findStringField(decoded.Data, "message", "msg", "tips", "desc"))
	if message == "" {
		if success {
			message = "价格修改成功"
		} else {
			message = "闲鱼未确认价格修改成功"
		}
	}
	return &AdjustPriceSubmitResult{Success: success, Message: message, UpdatedCookies: updatedCookies}, nil
}
