package mtop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

const (
	// RefundDetailAPI 是闲鱼官方退款详情页使用的只读 MTOP 端点。
	RefundDetailAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idle.refund.detail/1.0/"
	// refundDetailAPIName 是退款详情请求的签名 API 名称。
	refundDetailAPIName = "mtop.taobao.idle.refund.detail"
	// refundDetailReferer 是官方退款详情 H5 页面的固定来源地址。
	refundDetailReferer = "https://h5.m.goofish.com/wow/moyu/moyu-project/idle-reverse/pages/refundDetail"
)

// RefundDetailResult 是退款详情页公开展示的只读数据和响应 Cookie 变化。
type RefundDetailResult struct {
	// OrderID 是平台返回的订单标识。
	OrderID string
	// RefundID 是退款申请标识。
	RefundID string
	// RefundStatus 是平台退款状态编码或展示文本。
	RefundStatus string
	// RefundStatusText 是平台状态组件提供的可读文本。
	RefundStatusText string
	// RefundType 是仅退款、退货退款等平台展示类型。
	RefundType string
	// RefundReason 是买家选择的退款原因。
	RefundReason string
	// RefundAmount 是平台展示的退款金额文本。
	RefundAmount string
	// RefundApplyTime 是平台展示的申请时间。
	RefundApplyTime string
	// BuyerDescription 是买家补充说明；缺失时为空。
	BuyerDescription string
	// BuyerImages 是买家提交的图片凭证 URL。
	BuyerImages []string
	// BuyerVideos 是买家提交的视频凭证 URL。
	BuyerVideos []string
	// Seller 表示详情明确属于卖家处理视角。
	Seller bool
	// Actions 是平台当前允许卖家执行的普通同意／拒绝动作及服务端执行描述。
	Actions []RefundAction
	// UpdatedCookies 是本次请求协调后的平面 Cookie，只能留在凭证调用栈内。
	UpdatedCookies string
}

// FetchRefundDetail 按官方 H5 bundle 的 orderId/refundId 请求只读退款详情，不产生退款操作。
func (c *ClientImpl) FetchRefundDetail(ctx context.Context, cookiesStr, orderID string) (*RefundDetailResult, error) {
	// normalizedOrderID 是去除空白后的数字订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil, errors.New("退款详情订单 ID 不能为空")
	}
	// decoded、updatedCookies、requestErr 是官方响应、Cookie 变化和请求错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.RefundDetailURL, RefundDetailAPI), refundDetailAPIName, "1.0",
		map[string]any{"orderId": normalizedOrderID, "refundId": ""}, refundDetailReferer)
	if requestErr != nil {
		return &RefundDetailResult{UpdatedCookies: updatedCookies}, requestErr
	}
	// businessData 兼容官方 mtop promise 的 data.data 与直接 data 两种包装。
	businessData := decoded.Data
	// nested、ok 是官方 data.data 包装中的业务对象和类型判断结果。
	if nested, ok := decoded.Data["data"].(map[string]any); ok {
		businessData = nested
	}
	// components 是退款详情页按 render 名称返回的动态组件。
	components, _ := businessData["components"].([]any)
	// result 保存从公开退款组件提取的最小展示字段。
	result := &RefundDetailResult{OrderID: strings.TrimSpace(findStringField(businessData, "orderId")),
		RefundID: strings.TrimSpace(findStringField(businessData, "refundId")), RefundStatus: strings.TrimSpace(findStringField(businessData, "refundStatus")),
		Seller: redFlowerSuccess(businessData["seller"]), UpdatedCookies: updatedCookies}
	// rawComponent 是当前待解析的官方动态组件。
	for _, rawComponent := range components {
		// component、componentOK 是当前组件对象及其类型判断结果。
		component, componentOK := rawComponent.(map[string]any)
		if !componentOK {
			continue
		}
		// render 是官方页面用于选择展示组件的稳定名称。
		render, _ := component["render"].(string)
		render = strings.TrimSpace(render)
		// data 是当前组件公开展示数据。
		data, _ := component["data"].(map[string]any)
		switch render {
		case "refundInfo":
			result.RefundID = firstNonEmptyText(result.RefundID, findStringField(data, "refundId"))
			result.RefundType = strings.TrimSpace(findStringField(data, "refundType"))
			result.RefundReason = strings.TrimSpace(findStringField(data, "refundReason"))
			result.RefundAmount = strings.TrimSpace(findStringField(data, "refundAmount"))
			result.RefundApplyTime = strings.TrimSpace(findStringField(data, "refundApplyTime"))
		case "refundDescribe":
			result.BuyerDescription = strings.TrimSpace(findStringField(data, "buyerContent", "buyerOpinion", "refundDescribe", "description", "desc"))
			result.BuyerImages = refundMediaURLs(data["buyerImages"])
			result.BuyerVideos = refundMediaURLs(data["buyerVideos"])
		case "nodeStatusInfo", "refundStatusInfo":
			result.RefundStatusText = strings.TrimSpace(findStringField(data, "title", "statusText", "desc"))
		case "bottomBar":
			if result.Seller {
				result.Actions = refundActions(component["data"])
			}
		}
	}
	if result.OrderID == "" {
		result.OrderID = normalizedOrderID
	}
	if result.RefundID == "" && result.RefundReason == "" && result.RefundType == "" && result.RefundStatus == "" {
		return &RefundDetailResult{UpdatedCookies: updatedCookies}, errors.New("闲鱼未返回可识别的退款详情")
	}
	return result, nil
}

// RefundAction 描述官方详情当前下发的普通退款处理动作；执行参数只在服务端调用栈内使用。
type RefundAction struct {
	// Code 是平台当前动作的稳定标识。
	Code string
	// Name 是平台面向卖家的按钮名称。
	Name string
	// Kind 是 agree 或 reject。
	Kind string
	// Mode 是 direct 或 official；official 表示必须进入闲鱼原生认证流程。
	Mode string
	// ConfirmTitle 是平台双重确认标题；缺失时由 ERP 使用安全默认文案。
	ConfirmTitle string
	// ConfirmDescription 是平台双重确认说明。
	ConfirmDescription string
	// APIName 是详情动态下发且通过退款域名门禁的 MTOP 名称。
	APIName string
	// APIVersion 是动态 MTOP 版本。
	APIVersion string
	// Params 是平台下发的请求参数；不会返回给浏览器。
	Params map[string]any
}

// refundActions 从官方 bottomBar 提取当前卖家可直接执行的普通同意／拒绝动作。
func refundActions(value any) []RefundAction {
	// rows 是 bottomBar 的动态按钮列表。
	rows, _ := value.([]any)
	// actions 保存平台顺序下通过安全门禁的动作。
	actions := make([]RefundAction, 0, 2)
	// seen 防止平台重复按钮造成重复提交入口。
	seen := make(map[string]struct{}, len(rows))
	// rawRow 是当前待解析的动态按钮。
	for _, rawRow := range rows {
		// row、rowOK 是当前按钮对象及其类型判断结果。
		row, rowOK := rawRow.(map[string]any)
		if !rowOK {
			continue
		}
		// code、name 是平台动作标识和展示名称。
		code, name := strings.TrimSpace(mtopString(row["code"])), strings.TrimSpace(mtopString(row["name"]))
		// kind 是按平台 code 和中文按钮文案归一的动作方向。
		kind := refundActionKind(code, name)
		if code == "" || name == "" || kind == "" {
			continue
		}
		// clickEvent 是当前按钮的服务端驱动行为。
		clickEvent, _ := row["clickEvent"].(map[string]any)
		// apiName、apiVersion、params、mode 是当前动作的最终 MTOP 描述和执行方式。
		apiName, apiVersion, params, mode := refundActionMTop(clickEvent)
		if refundActionRequiresOfficial(code) {
			apiName, apiVersion, params, mode = "", "", nil, "official"
		}
		if mode == "direct" && !refundActionAPIAllowed(apiName, apiVersion) {
			mode = "official"
			apiName, apiVersion, params = "", "", nil
		}
		if mode == "" {
			mode = "official"
		}
		if mode == "official" && kind == "agree" {
			mode = "merchant_verify"
		} else if mode == "official" && kind == "reject" {
			mode = "merchant_refuse"
		}
		if // _, exists 表示同一平台动作已被收录。
		_, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		// doubleCheckVO 是平台可选的双重确认内容。
		doubleCheckVO := refundDoubleCheckVO(clickEvent)
		actions = append(actions, RefundAction{Code: code, Name: name, Kind: kind, Mode: mode,
			ConfirmTitle:       refundDisplayText(doubleCheckVO, "title", "headerTitle"),
			ConfirmDescription: refundDisplayText(doubleCheckVO, "desc", "description", "descRichText", "text", "content"),
			APIName:            apiName, APIVersion: apiVersion, Params: cloneRefundParams(params)})
	}
	return actions
}

// refundActionKind 只识别平台明确同意或拒绝语义，其他客服／物流动作不进入 ERP。
func refundActionKind(code, name string) string {
	// normalizedCode 是统一小写的平台动作标识。
	normalizedCode := strings.ToLower(strings.TrimSpace(code))
	// normalizedName 是去空白后的按钮文案。
	normalizedName := strings.TrimSpace(name)
	if strings.Contains(normalizedCode, "reject") || strings.Contains(normalizedCode, "refuse") || strings.Contains(normalizedCode, "disagree") || strings.Contains(normalizedName, "拒绝") || strings.Contains(normalizedName, "不同意") {
		return "reject"
	}
	if strings.Contains(normalizedCode, "agree") || strings.Contains(normalizedName, "同意") {
		return "agree"
	}
	return ""
}

// refundDoubleCheckVO 返回平台双重确认数据；直接 MTOP 动作为空。
func refundDoubleCheckVO(clickEvent map[string]any) map[string]any {
	// data 是 clickEvent 的动态数据对象。
	data, _ := clickEvent["data"].(map[string]any)
	// doubleCheck 是双重确认包装。
	doubleCheck, _ := data["doubleCheck"].(map[string]any)
	// view 是确认弹窗的公开展示数据。
	view, _ := doubleCheck["doubleCheckVO"].(map[string]any)
	return view
}

// refundActionMTop 读取直接 MTOP 或普通双重确认按钮中的最终动作描述。
func refundActionMTop(clickEvent map[string]any) (string, string, map[string]any, string) {
	// actionType 是官方行为分发类型。
	actionType := strings.TrimSpace(mtopString(clickEvent["type"]))
	// data 是当前行为数据。
	data, _ := clickEvent["data"].(map[string]any)
	if actionType == "mtop" {
		// apiName、apiVersion、params 是直接 MTOP 描述。
		apiName, apiVersion, params := refundMTopDescriptor(data["mtop"])
		return apiName, apiVersion, params, "direct"
	}
	if actionType != "doubleCheck" {
		return "", "", nil, "official"
	}
	// doubleCheck 是平台普通确认或富文本确认包装。
	doubleCheck, _ := data["doubleCheck"].(map[string]any)
	// code 是官方 bundle 可在网页内直接处理、且不要求地址／凭证／支付密码的确认场景。
	code := strings.TrimSpace(mtopString(doubleCheck["code"]))
	if code != "DOUBLE_CONFIRM_WINDOWS" && code != "OPEN_GENERAL_WINDOW" && code != "OPEN_RICH_TEXT_WINDOW" {
		return "", "", nil, "official"
	}
	// view 是平台确认弹窗数据。
	view, _ := doubleCheck["doubleCheckVO"].(map[string]any)
	// buttons 是确认弹窗按钮列表，取消按钮没有 MTOP 描述。
	buttons, _ := view["buttonList"].([]any)
	// rawButton 是当前待检查的确认按钮。
	for _, rawButton := range buttons {
		// button、buttonOK 是当前确认按钮及其类型判断结果。
		button, buttonOK := rawButton.(map[string]any)
		if !buttonOK {
			continue
		}
		// buttonCode 是官方确认按钮的业务动作标识。
		buttonCode := strings.TrimSpace(mtopString(button["code"]))
		if refundActionRequiresOfficial(buttonCode) {
			return "", "", nil, "official"
		}
		// nestedClick 是确认按钮的最终行为。
		nestedClick, _ := button["clickEvent"].(map[string]any)
		// nestedType 是确认按钮的官方行为类型；取消／关闭按钮不决定执行方式。
		nestedType := strings.TrimSpace(mtopString(nestedClick["type"]))
		if nestedType != "mtop" && nestedType != "doubleCheck" {
			continue
		}
		// apiName、apiVersion、params、mode 是当前确认按钮的 MTOP 描述和执行方式。
		apiName, apiVersion, params, mode := refundActionMTop(nestedClick)
		if apiName != "" || mode == "official" {
			return apiName, apiVersion, params, mode
		}
	}
	return "", "", nil, "official"
}

// refundActionRequiresOfficial 识别官方 bundle 中必须经过 WindVane／支付宝原生认证的动作。
func refundActionRequiresOfficial(code string) bool {
	// normalized 是统一小写后的平台动作标识。
	normalized := strings.ToLower(strings.TrimSpace(code))
	switch normalized {
	case "selleragreepay", "agreerefundgoodsrecharge", "agreerefundapply", "sellerconfirm", "confirmrefuseapply",
		"agreerefundapplyforhmos", "agreerefundapplyforwxapplet", "sellerconfirmforwxapplet", "confirmrefuseapplyforawxapplet":
		return true
	default:
		return false
	}
}

// refundMTopDescriptor 解析平台 data.mtop 的名称、版本和参数。
func refundMTopDescriptor(value any) (string, string, map[string]any) {
	// descriptor、descriptorOK 是 MTOP 描述对象及其类型判断结果。
	descriptor, descriptorOK := value.(map[string]any)
	if !descriptorOK {
		return "", "", nil
	}
	// params 是平台下发的业务参数。
	params, _ := descriptor["params"].(map[string]any)
	if params == nil {
		// rawParams 是部分容器可能使用的 JSON 字符串参数。
		rawParams := strings.TrimSpace(mtopString(descriptor["params"]))
		if rawParams != "" {
			_ = json.Unmarshal([]byte(rawParams), &params)
		}
	}
	return strings.TrimSpace(mtopString(descriptor["apiName"])), strings.TrimSpace(mtopString(descriptor["apiVersion"])), params
}

// refundActionAPIAllowed 只允许官方详情下发的退款域 MTOP，排除详情读取和复杂支付动作。
func refundActionAPIAllowed(apiName, version string) bool {
	// normalizedAPI 是统一小写后的动态接口名。
	normalizedAPI := strings.ToLower(strings.TrimSpace(apiName))
	// normalizedVersion 是去空白后的接口版本。
	normalizedVersion := strings.TrimSpace(version)
	return strings.HasPrefix(normalizedAPI, "mtop.taobao.idle.refund.") && normalizedAPI != refundDetailAPIName &&
		len(normalizedAPI) <= 160 && len(normalizedVersion) >= 3 && len(normalizedVersion) <= 12
}

// refundDisplayText 从平台富文本结构中提取有限长度的可读确认文案。
func refundDisplayText(value any, keys ...string) string {
	// text 是平台结构中第一个匹配的可读文本。
	text := strings.TrimSpace(findStringField(value, keys...))
	// runes 是限制长度前的确认文案字符。
	runes := []rune(text)
	if len(runes) > 300 {
		text = string(runes[:300])
	}
	return text
}

// cloneRefundParams 复制平台动作参数，避免调用方修改详情缓存对象。
func cloneRefundParams(params map[string]any) map[string]any {
	if params == nil {
		return map[string]any{}
	}
	// cloned 保存当前动作参数的浅复制；嵌套对象只作为 JSON 请求数据读取。
	cloned := make(map[string]any, len(params))
	// key、value 是当前待复制的平台参数。
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

// validateRefundActionDescriptor 返回动态动作是否仍符合退款接口和参数体积边界。
func validateRefundActionDescriptor(action RefundAction) error {
	if action.Code == "" || action.Mode != "direct" || (action.Kind != "agree" && action.Kind != "reject") || !refundActionAPIAllowed(action.APIName, action.APIVersion) {
		return errors.New("退款动作已经失效")
	}
	// encoded、encodeErr 是平台动态参数的 JSON 编码及错误。
	encoded, encodeErr := jsonMarshalRefundParams(action.Params)
	if encodeErr != nil || len(encoded) > 32*1024 {
		return errors.New("退款动作参数无效")
	}
	return nil
}

// jsonMarshalRefundParams 隔离动态参数 JSON 编码，便于定向测试边界。
func jsonMarshalRefundParams(params map[string]any) ([]byte, error) {
	return json.Marshal(params)
}

// refundMediaURLs 从买家凭证列表提取有限数量的 HTTP(S) 地址，忽略动态业务对象。
func refundMediaURLs(value any) []string {
	// rows 是平台返回的可选媒体对象列表。
	rows, _ := value.([]any)
	// urls 保存去重后的最多九个媒体地址。
	urls := make([]string, 0, len(rows))
	// seen 防止同一凭证在多个兼容字段重复展示。
	seen := make(map[string]struct{}, len(rows))
	// row 是当前待解析的媒体对象。
	for _, row := range rows {
		// mediaURL 是当前对象中递归找到的公开媒体地址。
		mediaURL := strings.TrimSpace(findStringField(row, "url", "imageUrl", "videoUrl", "playUrl"))
		if !strings.HasPrefix(mediaURL, "https://") && !strings.HasPrefix(mediaURL, "http://") {
			continue
		}
		if // _, exists 表示当前媒体地址是否已经收录。
		_, exists := seen[mediaURL]; exists {
			continue
		}
		seen[mediaURL] = struct{}{}
		urls = append(urls, mediaURL)
		if len(urls) >= 9 {
			break
		}
	}
	return urls
}

// firstNonEmptyText 返回首个去空白后的非空退款展示文本。
func firstNonEmptyText(values ...string) string {
	// value 是当前待去空白并判断的候选文本。
	for _, value := range values {
		// normalized 是当前候选文本的去空白结果。
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}
