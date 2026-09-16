// Package mtop: 虚拟发货域 — mtop.taobao.idle.logistics.merchant.consign.dummy 调用与重试。
package mtop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xianyu-go/internal/xianyu/protocol"
)

// ConsignEvidenceRequest 描述卖家工作台“无需寄件”的文本和图片 URL 请求。
type ConsignEvidenceRequest struct {
	// OrderID 是待确认发货的平台订单号。
	OrderID string
	// TradeText 是用户提供的相关描述，官方限制最多 200 字。
	TradeText string
	// ImageURLs 是已通过 fleamarket 作用域上传得到的图片 URL，最多三项。
	ImageURLs []string
}

// ConsignEvidenceResult 保存平台确认结果、非敏感提示和响应 Cookie 变化。
type ConsignEvidenceResult struct {
	// Success 只在响应明确包含成功订单号时为真。
	Success bool
	// Ret 是平台返回的稳定业务提示列表。
	Ret []string
	// UpdatedCookies 是本次请求观察到的最新平面 Cookie。
	UpdatedCookies string
}

// Consign 调用当前卖家工作台无需寄件接口确认发货。
// data_val 中 picList 是图片 URL 数组的 JSON 字符串，而不是直接数组。
// 返回成功标志、响应 ret 列表、可能更新后的 cookie。
// 移植自 secure_confirm_decrypted.auto_confirm。
// Consign 封装Consign业务协调。
func (c *ClientImpl) Consign(cookiesStr, orderID string) (ok bool, ret []string, updatedCookies string, err error) {
	return c.ConsignContext(context.Background(), cookiesStr, orderID)
}

// ConsignContext 确认发货；签名 token 过期时使用响应下发的新 Cookie 重签并重试。
func (c *ClientImpl) ConsignContext(ctx context.Context, cookiesStr, orderID string) (ok bool, ret []string, updatedCookies string, err error) {
	// result、requestErr 是无描述、无凭证的兼容发货结果。
	result, requestErr := c.ConsignEvidenceContext(ctx, cookiesStr, ConsignEvidenceRequest{OrderID: orderID})
	if result == nil {
		return false, nil, cookiesStr, requestErr
	}
	return result.Success, result.Ret, result.UpdatedCookies, requestErr
}

// ConsignEvidenceContext 使用相关描述和最多三张已上传凭证确认无需寄件；token 过期时按既有规则重签重试。
func (c *ClientImpl) ConsignEvidenceContext(ctx context.Context, cookiesStr string, evidence ConsignEvidenceRequest) (*ConsignEvidenceResult, error) {
	// currentCookies 用于本次流程后续判断的currentCookies
	currentCookies := cookiesStr
	if // session 用于本次流程后续判断的会话
	session := cookieSessionFromContext(ctx); session != nil {
		currentCookies, _, _ = session.State()
	}
	// lastRet 用于本次流程后续判断的lastRet
	var lastRet []string
	for // attempt 用于本次流程后续判断的尝试次数
	attempt := 0; attempt < 4; attempt++ {
		// previousCookies 用于本次流程后续判断的previousCookies
		previousCookies := currentCookies
		// ok、ret、updated、requestErr 用于本次流程后续判断的ok、ret、updated、requestErr
		ok, ret, updated, requestErr := c.consignOnce(ctx, currentCookies, evidence)
		if requestErr != nil {
			return &ConsignEvidenceResult{Ret: ret, UpdatedCookies: currentCookies}, requestErr
		}
		lastRet = ret
		if updated != "" {
			currentCookies = updated
		}
		if ok {
			return &ConsignEvidenceResult{Success: true, Ret: ret, UpdatedCookies: currentCookies}, nil
		}
		if isSessionExpiredRet(ret) {
			return &ConsignEvidenceResult{Ret: ret, UpdatedCookies: currentCookies}, sessionExpiredError("确认发货接口", ret)
		}
		if !isTokenExpiredRet(ret) {
			return &ConsignEvidenceResult{Ret: ret, UpdatedCookies: currentCookies}, nil
		}
		if attempt == 3 {
			break
		}

		// MTop 通常会在 token 过期响应中通过 Set-Cookie 下发新签名 token。
		// 若没有下发，则主动调用 token API 尝试刷新一次。
		if currentCookies == previousCookies {
			// refreshed、refreshErr 用于本次流程后续判断的refreshed、refreshErr
			refreshed, refreshErr := c.RefreshTokenContext(ctx, currentCookies)
			if refreshErr != nil {
				return &ConsignEvidenceResult{Ret: ret, UpdatedCookies: currentCookies}, fmt.Errorf("consign token 过期且刷新失败: %w", refreshErr)
			}
			if refreshed.UpdatedCookies != "" {
				currentCookies = refreshed.UpdatedCookies
			}
		}
		if // err 用于本次流程后续判断的err
		err := sleepCtx(ctx, MTopRetryGap); err != nil {
			return &ConsignEvidenceResult{Ret: ret, UpdatedCookies: currentCookies}, err
		}
	}
	return &ConsignEvidenceResult{Ret: lastRet, UpdatedCookies: currentCookies}, nil
}

// consignOnce 封装consignOnce业务协调。
func (c *ClientImpl) consignOnce(ctx context.Context, cookiesStr string, evidence ConsignEvidenceRequest) (ok bool, ret []string, updatedCookies string, err error) {
	// hc 用于本次流程后续判断的hc
	hc := c.httpClient()
	// consignURL 用于本次流程后续判断的consignURL
	consignURL := c.ConsignURL
	if consignURL == "" {
		consignURL = ConsignAPI
	}
	// signingCookies、requestCookies 用于本次流程后续判断的signingCookies、requestCookies
	signingCookies, requestCookies := mtopRequestCookies(ctx, cookiesStr, "https://www.goofish.com/", consignURL)
	// token 用于本次流程后续判断的令牌
	token := protocol.SignToken(signingCookies)
	// t 用于本次流程后续判断的t
	t := strconv.FormatInt(time.Now().UnixMilli(), 10)
	// pictureJSON 是官方前端 JSON.stringify 后写入 picList 字符串字段的 URL 数组。
	pictureJSON, pictureErr := json.Marshal(evidence.ImageURLs)
	if pictureErr != nil {
		return false, nil, cookiesStr, pictureErr
	}
	// dataPayload 严格对应当前官方卖家工作台参数，避免描述文本参与手工 JSON 拼接。
	dataPayload := struct {
		// OrderID 是平台订单号。
		OrderID string `json:"orderId"`
		// TradeText 是相关描述。
		TradeText string `json:"tradeText"`
		// PicList 是图片 URL 数组编码后的 JSON 字符串。
		PicList string `json:"picList"`
		// NewUnconsign 固定启用新版无需寄件流程。
		NewUnconsign bool `json:"newUnconsign"`
	}{OrderID: strings.TrimSpace(evidence.OrderID), TradeText: evidence.TradeText, PicList: string(pictureJSON), NewUnconsign: true}
	// encodedPayload、encodeErr 是用于 MTOP 签名和请求体的唯一 JSON 字节。
	encodedPayload, encodeErr := json.Marshal(dataPayload)
	if encodeErr != nil {
		return false, nil, cookiesStr, encodeErr
	}
	// dataVal 是签名和 body 必须复用的同一 JSON 文本。
	dataVal := string(encodedPayload)
	// sign 用于本次流程后续判断的sign
	sign := protocol.GenerateSign(t, token, dataVal)

	// query 用于本次流程后续判断的查询
	query := buildConsignQuery(t, sign)
	// body 用于本次流程后续判断的请求体
	body := "data=" + url.QueryEscape(dataVal)
	// req、err 用于本次流程后续判断的req、err
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		consignURL+"?"+query,
		strings.NewReader(body))
	if err != nil {
		return false, nil, cookiesStr, err
	}
	setCommonHeaders(req, requestCookies)
	// resp、err 用于本次流程后续判断的resp、err
	resp, err := hc.Do(req)
	if err != nil {
		return false, nil, cookiesStr, fmt.Errorf("consign 请求失败: %w", err)
	}
	defer resp.Body.Close()
	// updated 用于本次流程后续判断的updated
	updated := absorbMTopResponseCookies(ctx, cookiesStr, resp)
	// raw、err 用于本次流程后续判断的raw、err
	raw, err := readMTopBody(resp)
	if err != nil {
		return false, nil, updated, err
	}
	// res 用于本次流程后续判断的响应
	var res struct {
		Ret []string `json:"ret"`
		// Data 包含官方成功后回传的订单号。
		Data map[string]any `json:"data"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(raw, &res); err != nil {
		return false, nil, updated, fmt.Errorf("解析 consign 响应失败: %w (body=%s)", err, truncate(string(raw), 300))
	}
	if strings.TrimSpace(mtopString(res.Data["orderId"])) != "" {
		return true, res.Ret, updated, nil
	}
	return false, res.Ret, updated, nil
}

// buildConsignQuery 封装buildConsign查询业务协调。
func buildConsignQuery(t, sign string) string {
	// parts 用于本次流程后续判断的parts
	parts := [][2]string{
		{"jsv", "2.7.2"},
		{"appKey", protocol.SignAppKey},
		{"t", t},
		{"sign", sign},
		{"v", "1.0"},
		{"type", "originaljson"},
		{"accountSite", "xianyu"},
		{"dataType", "json"},
		{"timeout", "20000"},
		{"api", "mtop.taobao.idle.logistics.merchant.consign.dummy"},
		{"sessionOption", "AutoLoginOnly"},
	}
	// b 用于本次流程后续判断的b
	var b strings.Builder
	// i、p 表示当前遍历过程中的i、p
	for i, p := range parts {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p[0])
		b.WriteByte('=')
		b.WriteString(p[1])
	}
	return b.String()
}
