package mtop

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

const (
	// merchantRefundVerifyAPI 是卖家工作台创建 PC 支付宝验证页面的接口。
	merchantRefundVerifyAPI = "mtop.idle.alipay.verify.url.query"
	// merchantRefundAgreeAPI 是卖家工作台支付验证后的最终同意退款接口。
	merchantRefundAgreeAPI = "mtop.taobao.idle.merchant.refund.agree.refund"
	// merchantRefundRefuseRenderAPI 是卖家工作台动态拒绝原因和凭证要求接口。
	merchantRefundRefuseRenderAPI = "mtop.taobao.idle.merchant.refund.refuse.render"
	// merchantRefundRefuseAPI 是卖家工作台最终拒绝退款接口。
	merchantRefundRefuseAPI = "mtop.taobao.idle.merchant.refund.refuse"
	// merchantRefundReferer 是官方桌面卖家工作台退款管理页面。
	merchantRefundReferer = "https://seller.goofish.com/?site=COMMONPRO#/seller-trade/refund-manage"
)

// MerchantRefundVerification 保存短期支付宝验证页面和只允许服务端持有的授权 token。
type MerchantRefundVerification struct {
	// VerifyURL 是支付宝 PC 密码验证 iframe 地址。
	VerifyURL string
	// AuthToken 是支付验证成功后最终 Merchant 退款接口使用的短期 token。
	AuthToken string
	// UpdatedCookies 是本次请求协调后的平面 Cookie。
	UpdatedCookies string
}

// MerchantRefundResult 保存 Merchant 最终动作的确定性响应。
type MerchantRefundResult struct {
	// Success 只在 Merchant MTOP 明确成功时为真。
	Success bool
	// Message 是平台非敏感提示。
	Message string
	// UpdatedCookies 是本次请求协调后的平面 Cookie。
	UpdatedCookies string
}

// MerchantRefundRefuseReason 是平台当前允许选择的拒绝原因。
type MerchantRefundRefuseReason struct {
	// ID 是提交 Merchant refuse 使用的 refuseReasonId。
	ID string
	// Name 是平台展示原因。
	Name string
	// RequiresApp 表示平台明确要求扫码到手机 App 处理。
	RequiresApp bool
}

// MerchantRefundNegotiation 描述拒绝时可选的协商金额范围。
type MerchantRefundNegotiation struct {
	// Enabled 表示当前原因允许协商金额或运费。
	Enabled bool
	// Type 是平台 idleNegotiationType。
	Type string
	// MinCents 是平台允许的最小整数分。
	MinCents int64
	// MaxCents 是平台允许的最大整数分。
	MaxCents int64
}

// MerchantRefundProofRequirement 描述拒绝原因的凭证要求。
type MerchantRefundProofRequirement struct {
	// Required 表示该原因必须上传图片凭证。
	Required bool
	// Placeholder 是补充说明输入提示。
	Placeholder string
}

// MerchantRefundRefuseForm 保存动态原因、协商范围和凭证要求。
type MerchantRefundRefuseForm struct {
	// Reasons 是平台当前原因列表。
	Reasons []MerchantRefundRefuseReason
	// Negotiation 是当前选中原因对应的协商配置。
	Negotiation MerchantRefundNegotiation
	// Proof 是当前选中原因对应的凭证要求。
	Proof MerchantRefundProofRequirement
	// UpdatedCookies 是本次请求协调后的平面 Cookie。
	UpdatedCookies string
}

// MerchantRefundRefuseRequest 是最终拒绝退款请求。
type MerchantRefundRefuseRequest struct {
	// RefundID 是售后申请标识。
	RefundID string
	// OrderID 是平台订单标识。
	OrderID string
	// ReasonID 是平台动态 refuseReasonId。
	ReasonID string
	// Description 是最多二百字的补充描述。
	Description string
	// ProofURLs 是已经上传到平台的图片地址。
	ProofURLs []string
	// NegotiationCents 是可选的协商退款整数分。
	NegotiationCents int64
	// NegotiationType 是平台动态协商类型。
	NegotiationType string
}

// CreateMerchantRefundVerification 创建 PC 支付验证 iframe；不执行退款。
func (c *ClientImpl) CreateMerchantRefundVerification(ctx context.Context, cookiesStr, refundID string) (*MerchantRefundVerification, error) {
	// normalizedRefundID 是去空白后的退款标识。
	normalizedRefundID := strings.TrimSpace(refundID)
	if normalizedRefundID == "" {
		return nil, errors.New("退款验证缺少 refundId")
	}
	// endpoint 是测试覆盖地址或固定官方 Merchant MTOP 地址。
	endpoint := firstNonEmptyURL(c.MerchantRefundVerifyURL, merchantMTopURL(merchantRefundVerifyAPI, "1.0"))
	// decoded、updatedCookies、requestErr 是验证页面响应、Cookie 变化和错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr, endpoint, merchantRefundVerifyAPI, "1.0",
		map[string]any{"bizId": normalizedRefundID, "scene": "REFUND_PC", "callBackUrl": merchantRefundReferer}, merchantRefundReferer)
	if requestErr != nil {
		return &MerchantRefundVerification{UpdatedCookies: updatedCookies}, requestErr
	}
	// module 是平台验证页面和 token 包装。
	module, _ := decoded.Data["module"].(map[string]any)
	// verifyURL、authToken 是短期支付宝页面和最终退款授权 token。
	verifyURL, authToken := strings.TrimSpace(findStringField(module, "verifyUrl")), strings.TrimSpace(findStringField(module, "token"))
	if !merchantVerifyURLAllowed(verifyURL) || authToken == "" {
		return &MerchantRefundVerification{UpdatedCookies: updatedCookies}, errors.New("闲鱼未返回可用的 PC 退款验证页面")
	}
	return &MerchantRefundVerification{VerifyURL: verifyURL, AuthToken: authToken, UpdatedCookies: updatedCookies}, nil
}

// AgreeMerchantRefund 使用支付验证后的短期 token 执行最终同意退款。
func (c *ClientImpl) AgreeMerchantRefund(ctx context.Context, cookiesStr, refundID, authToken string) (*MerchantRefundResult, error) {
	// normalizedRefundID、normalizedToken 是去空白后的业务标识和短期授权。
	normalizedRefundID, normalizedToken := strings.TrimSpace(refundID), strings.TrimSpace(authToken)
	if normalizedRefundID == "" || normalizedToken == "" {
		return nil, errors.New("同意退款缺少 refundId 或支付验证授权")
	}
	// decoded、updatedCookies、requestErr 是最终 Merchant 响应、Cookie 变化和错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.MerchantRefundAgreeURL, merchantMTopURL(merchantRefundAgreeAPI, "1.0")), merchantRefundAgreeAPI, "1.0",
		map[string]any{"refundId": normalizedRefundID, "authToken": normalizedToken}, merchantRefundReferer)
	if requestErr != nil {
		return &MerchantRefundResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// message 是最终 Merchant 接口返回的非敏感说明。
	message := strings.TrimSpace(findStringField(decoded.Data, "message", "msg", "toast", "tips"))
	if message == "" {
		message = "退款成功"
	}
	return &MerchantRefundResult{Success: true, Message: message, UpdatedCookies: updatedCookies}, nil
}

// FetchMerchantRefundRefuseForm 读取动态拒绝原因；不执行拒绝退款。
func (c *ClientImpl) FetchMerchantRefundRefuseForm(ctx context.Context, cookiesStr, refundID, reasonID string) (*MerchantRefundRefuseForm, error) {
	// normalizedRefundID、normalizedReasonID 是去空白后的退款和可选原因标识。
	normalizedRefundID, normalizedReasonID := strings.TrimSpace(refundID), strings.TrimSpace(reasonID)
	if normalizedRefundID == "" {
		return nil, errors.New("拒绝退款表单缺少 refundId")
	}
	// data 是平台 render 请求体。
	data := map[string]any{"refundId": normalizedRefundID}
	if normalizedReasonID != "" {
		data["refuseReasonId"] = normalizedReasonID
	}
	// decoded、updatedCookies、requestErr 是动态表单响应、Cookie 变化和错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.MerchantRefundRefuseRenderURL, merchantMTopURL(merchantRefundRefuseRenderAPI, "1.0")), merchantRefundRefuseRenderAPI, "1.0", data, merchantRefundReferer)
	if requestErr != nil {
		return &MerchantRefundRefuseForm{UpdatedCookies: updatedCookies}, requestErr
	}
	// businessData 是官方 data.data 业务包装。
	businessData := decoded.Data
	if // nested、nestedOK 是官方 data.data 业务对象和类型判断。
	nested, nestedOK := decoded.Data["data"].(map[string]any); nestedOK {
		businessData = nested
	}
	// form 保存规范化后的动态表单。
	form := &MerchantRefundRefuseForm{UpdatedCookies: updatedCookies}
	// rawReasons 是平台拒绝原因列表。
	rawReasons, _ := businessData["refuseReasonList"].([]any)
	// rawReason 是当前待解析原因。
	for _, rawReason := range rawReasons {
		// reasonMap、reasonOK 是当前原因对象及类型结果。
		reasonMap, reasonOK := rawReason.(map[string]any)
		if !reasonOK {
			continue
		}
		// id、name、platformReasonID 是提交 ID、展示名和手机专属判断 ID。
		id := strings.TrimSpace(findStringField(reasonMap, "refuseReasonId"))
		// name 是平台展示原因。
		name := strings.TrimSpace(findStringField(reasonMap, "reasonName"))
		// platformReasonID 是手机专属判断 ID。
		platformReasonID := strings.TrimSpace(findStringField(reasonMap, "reasonId"))
		if id == "" || name == "" {
			continue
		}
		form.Reasons = append(form.Reasons, MerchantRefundRefuseReason{ID: id, Name: name, RequiresApp: platformReasonID == "20653007"})
	}
	// negotiation 是动态协商金额对象。
	negotiation, _ := businessData["refuseNegotiation"].(map[string]any)
	if len(negotiation) > 0 {
		form.Negotiation = MerchantRefundNegotiation{Enabled: true, Type: strings.TrimSpace(findStringField(negotiation, "negotiationType")),
			MinCents: merchantInt64(findStringField(negotiation, "minRefundFee")), MaxCents: merchantInt64(findStringField(negotiation, "maxRefundFee"))}
	}
	// proof 是动态凭证要求对象。
	proof, _ := businessData["refuseProof"].(map[string]any)
	form.Proof = MerchantRefundProofRequirement{Required: redFlowerSuccess(proof["mustProof"]), Placeholder: strings.TrimSpace(findStringField(proof, "desc", "placeholder"))}
	return form, nil
}

// RefuseMerchantRefund 使用平台最新原因和可选凭证拒绝退款。
func (c *ClientImpl) RefuseMerchantRefund(ctx context.Context, cookiesStr string, request MerchantRefundRefuseRequest) (*MerchantRefundResult, error) {
	if strings.TrimSpace(request.RefundID) == "" || strings.TrimSpace(request.OrderID) == "" || strings.TrimSpace(request.ReasonID) == "" {
		return nil, errors.New("拒绝退款缺少订单、退款或原因标识")
	}
	// data 是 Merchant refuse 请求体。
	data := map[string]any{"refundId": strings.TrimSpace(request.RefundID), "orderId": strings.TrimSpace(request.OrderID), "refuseReasonId": strings.TrimSpace(request.ReasonID)}
	if strings.TrimSpace(request.Description) != "" || len(request.ProofURLs) > 0 {
		// mediaList 是官方 proofMultiMediaList 图片数组。
		mediaList := make([]map[string]any, 0, len(request.ProofURLs))
		// proofURL 是当前平台图片地址。
		for _, proofURL := range request.ProofURLs {
			mediaList = append(mediaList, map[string]any{"mediaTypeEnum": "IMAGE", "url": proofURL})
		}
		data["refuseProof"] = mustMerchantJSON(map[string]any{"desc": strings.TrimSpace(request.Description), "proofMultiMediaList": mediaList})
	}
	if request.NegotiationCents > 0 && strings.TrimSpace(request.NegotiationType) != "" {
		data["negotiationApply"] = mustMerchantJSON(map[string]any{"negotiationRefundFee": request.NegotiationCents, "idleNegotiationType": strings.TrimSpace(request.NegotiationType)})
	}
	// decoded、updatedCookies、requestErr 是最终拒绝响应、Cookie 变化和错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.MerchantRefundRefuseURL, merchantMTopURL(merchantRefundRefuseAPI, "1.0")), merchantRefundRefuseAPI, "1.0", data, merchantRefundReferer)
	if requestErr != nil {
		return &MerchantRefundResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// message 是平台成功提示。
	message := strings.TrimSpace(findStringField(decoded.Data, "message", "msg", "toast", "tips"))
	if message == "" {
		message = "已拒绝退款申请"
	}
	return &MerchantRefundResult{Success: true, Message: message, UpdatedCookies: updatedCookies}, nil
}

// merchantMTopURL 使用固定官方域名构造 Merchant MTOP 地址。
func merchantMTopURL(apiName, version string) string {
	return (&url.URL{Scheme: "https", Host: "h5api.m.goofish.com", Path: "/h5/" + strings.ToLower(apiName) + "/" + version + "/"}).String()
}

// merchantVerifyURLAllowed 只允许支付宝 PC 认证 HTTPS 页面。
func merchantVerifyURLAllowed(raw string) bool {
	// parsed、parseErr 是验证页面标准 URL 和解析错误。
	parsed, parseErr := url.Parse(strings.TrimSpace(raw))
	if parseErr != nil || parsed.Scheme != "https" {
		return false
	}
	// hostname 是统一小写的支付宝认证主机。
	hostname := strings.ToLower(parsed.Hostname())
	return hostname == "pcauth-site.alipay.com" || strings.HasSuffix(hostname, ".pcauth-site.alipay.com")
}

// merchantInt64 把平台整数文本转换为 int64，非法值安全回退零。
func merchantInt64(raw string) int64 {
	// value、parseErr 是平台整数和转换错误。
	value, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if parseErr != nil {
		return 0
	}
	return value
}

// mustMerchantJSON 编码内部已校验结构；编码失败时返回空对象而不泄露参数。
func mustMerchantJSON(value any) string {
	// encoded、encodeErr 是动态结构 JSON 和编码错误。
	encoded, encodeErr := json.Marshal(value)
	if encodeErr != nil {
		return "{}"
	}
	return string(encoded)
}
