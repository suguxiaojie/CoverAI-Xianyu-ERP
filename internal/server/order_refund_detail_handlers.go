package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/auth"
)

const (
	// refundDetailOfficialURL 是只在 ERP 读取失败时供用户手动打开的闲鱼官方页面。
	refundDetailOfficialURL = "https://h5.m.goofish.com/wow/moyu/moyu-project/idle-reverse/pages/refundDetail"
)

// orderRefundDetailResponse 是退款申请卡片使用的具名只读响应。
type orderRefundDetailResponse struct {
	// OrderID 是平台订单标识。
	OrderID string `json:"order_id"`
	// AccountID 是订单所属卖家账号。
	AccountID string `json:"account_id"`
	// RefundID 是平台退款申请标识。
	RefundID string `json:"refund_id,omitempty"`
	// Status 是平台退款状态编码或展示文本。
	Status string `json:"status,omitempty"`
	// StatusText 是平台状态组件提供的可读文本。
	StatusText string `json:"status_text,omitempty"`
	// Type 是仅退款、退货退款等平台展示类型。
	Type string `json:"type,omitempty"`
	// Reason 是买家选择的退款原因。
	Reason string `json:"reason,omitempty"`
	// Amount 是平台展示的退款金额文本。
	Amount string `json:"amount,omitempty"`
	// ApplyTime 是平台展示的申请时间。
	ApplyTime string `json:"apply_time,omitempty"`
	// BuyerDescription 是买家补充说明。
	BuyerDescription string `json:"buyer_description,omitempty"`
	// BuyerImages 是买家提交的图片凭证。
	BuyerImages []string `json:"buyer_images"`
	// BuyerVideos 是买家提交的视频凭证。
	BuyerVideos []string `json:"buyer_videos"`
	// Seller 表示当前详情明确属于卖家处理视角。
	Seller bool `json:"seller"`
	// Actions 是平台当前允许在 ERP 中执行的普通同意／拒绝动作。
	Actions []orderRefundActionDTO `json:"actions"`
	// OfficialURL 是闲鱼官方详情页的安全兜底地址。
	OfficialURL string `json:"official_url"`
}

// orderRefundActionDTO 是不包含 MTOP 名称和参数的公开动态动作。
type orderRefundActionDTO struct {
	// Code 是平台当前动作标识。
	Code string `json:"code"`
	// Name 是平台按钮名称。
	Name string `json:"name"`
	// Kind 是 agree 或 reject。
	Kind string `json:"kind"`
	// Mode 是 direct 或 official。
	Mode string `json:"mode"`
	// ConfirmTitle 是平台双重确认标题。
	ConfirmTitle string `json:"confirm_title,omitempty"`
	// ConfirmDescription 是平台双重确认说明。
	ConfirmDescription string `json:"confirm_description,omitempty"`
}

// orderRefundActionRequest 是应用内二次确认后的具名退款动作请求。
type orderRefundActionRequest struct {
	// AccountID 是退款订单所属卖家账号。
	AccountID string `json:"account_id"`
	// ActionCode 是当前详情返回的平台动作标识。
	ActionCode string `json:"action_code"`
}

// orderRefundActionResponse 是平台真实退款动作的具名确定性结果。
type orderRefundActionResponse struct {
	// Success 只在平台明确受理动作时为真。
	Success bool `json:"success"`
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string `json:"status"`
	// Message 是不含凭证的用户提示。
	Message string `json:"message"`
	// OrderID 是本次处理的平台订单标识。
	OrderID string `json:"order_id"`
	// RefundID 是本次处理的平台退款申请标识。
	RefundID string `json:"refund_id"`
	// Action 是实际提交的 agree 或 reject。
	Action string `json:"action"`
}

// RefundDetail 调用订单应用服务读取官方退款详情。
func (adapter *orderHTTPAdapter) RefundDetail(ctx context.Context, request orderapp.RefundDetailRequest) (orderapp.RefundDetailResult, error) {
	return adapter.services.RefundDetail(ctx, request)
}

// RefundAction 调用订单应用服务执行用户确认后的真实退款动作。
func (adapter *orderHTTPAdapter) RefundAction(ctx context.Context, request orderapp.RefundActionRequest) (orderapp.RefundActionResult, error) {
	return adapter.services.RefundAction(ctx, request)
}

// getOrderRefundDetail 返回退款申请卡片对应的官方只读详情。
func (server *Server) getOrderRefundDetail(writer http.ResponseWriter, request *http.Request) {
	// orderID、accountID 是路由订单和查询中的卖家账号标识。
	orderID, accountID := strings.TrimSpace(chi.URLParam(request, "order_id")), strings.TrimSpace(request.URL.Query().Get("account_id"))
	if orderID == "" || accountID == "" {
		writeErr(writer, http.StatusBadRequest, "缺少订单 ID 或账号 ID")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// result、detailErr 是应用层公开退款详情和读取错误。
	result, detailErr := server.orders().RefundDetail(request.Context(), orderapp.RefundDetailRequest{UserID: session.UserID, AccountID: accountID, OrderID: orderID})
	if detailErr != nil {
		writeRefundDetailError(writer, detailErr)
		return
	}
	// images、videos 是在传输边界完成 HTTPS 归一的买家媒体地址。
	images, videos := normalizeRefundMedia(result.BuyerImages), normalizeRefundMedia(result.BuyerVideos)
	// actions 是剔除服务端 MTOP 描述后的公开动态动作列表。
	actions := refundActionDTOs(result.Actions)
	writeJSON(writer, http.StatusOK, orderRefundDetailResponse{OrderID: result.OrderID, AccountID: result.AccountID,
		RefundID: result.RefundID, Status: result.Status, StatusText: result.StatusText, Type: result.Type, Reason: result.Reason, Amount: result.Amount,
		ApplyTime: result.ApplyTime, BuyerDescription: result.BuyerDescription, BuyerImages: images, BuyerVideos: videos,
		Seller: result.Seller, Actions: actions, OfficialURL: officialRefundDetailURL(result.OrderID)})
}

// submitOrderRefundAction 执行用户在应用内二次确认的普通同意／拒绝退款动作。
func (server *Server) submitOrderRefundAction(writer http.ResponseWriter, request *http.Request) {
	// orderID 是路由中的平台订单标识。
	orderID := strings.TrimSpace(chi.URLParam(request, "order_id"))
	// input 是具名 JSON 请求。
	var input orderRefundActionRequest
	if orderID == "" || decodeJSON(request, &input) != nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ActionCode) == "" {
		writeErr(writer, http.StatusBadRequest, "退款处理请求缺少订单、账号或动作")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// result、actionErr 是应用层真实退款动作结果和错误。
	result, actionErr := server.orders().RefundAction(request.Context(), orderapp.RefundActionRequest{UserID: session.UserID,
		AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID, ActionCode: strings.TrimSpace(input.ActionCode)})
	if actionErr != nil {
		writeRefundDetailError(writer, actionErr)
		return
	}
	writeJSON(writer, http.StatusOK, orderRefundActionResponse{Success: result.Success, Status: result.Status, Message: result.Message,
		OrderID: result.OrderID, RefundID: result.RefundID, Action: result.Action})
}

// refundActionDTOs 将应用层动态动作转换为不含执行参数的具名 DTO。
func refundActionDTOs(actions []orderapp.RefundAction) []orderRefundActionDTO {
	// result 保存保持平台顺序的公开动作。
	result := make([]orderRefundActionDTO, 0, len(actions))
	// action 是当前待转换的动态退款动作。
	for _, action := range actions {
		result = append(result, orderRefundActionDTO{Code: action.Code, Name: action.Name, Kind: action.Kind, Mode: action.Mode,
			ConfirmTitle: action.ConfirmTitle, ConfirmDescription: action.ConfirmDescription})
	}
	return result
}

// normalizeRefundMedia 在 HTTP 边界规范买家退款凭证协议并剔除空地址。
func normalizeRefundMedia(values []string) []string {
	// result 保存保持平台顺序的非空媒体地址。
	result := make([]string, 0, len(values))
	// value 是当前待规范的媒体地址。
	for _, value := range values {
		// normalized 是仅对受支持 CDN 升级 HTTPS 后的地址。
		normalized := strings.TrimSpace(normalizeRemoteMediaURL(value))
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

// officialRefundDetailURL 使用固定闲鱼域名和数字订单号构造人工兜底页面。
func officialRefundDetailURL(orderID string) string {
	// target 是固定白名单基址解析出的官方页面地址。
	target, parseErr := url.Parse(refundDetailOfficialURL)
	if parseErr != nil {
		return ""
	}
	// query 是只加入订单号和卖家页面标记的安全查询参数。
	query := target.Query()
	query.Set("orderId", strings.TrimSpace(orderID))
	query.Set("kun", "true")
	target.RawQuery = query.Encode()
	return target.String()
}

// writeRefundDetailError 把只读详情错误映射为统一 HTTP 状态。
func writeRefundDetailError(writer http.ResponseWriter, err error) {
	// validationErr 保存账号或订单号输入错误。
	var validationErr *orderapp.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeErr(writer, http.StatusBadRequest, err.Error())
	case errors.Is(err, orderapp.ErrForbidden):
		writeErr(writer, http.StatusForbidden, "无权读取此订单")
	case errors.Is(err, orderapp.ErrNotFound):
		writeErr(writer, http.StatusNotFound, "订单不存在")
	case errors.Is(err, orderapp.ErrRefundDetailNotEligible):
		writeErr(writer, http.StatusConflict, err.Error())
	case errors.Is(err, orderapp.ErrRefundActionInvalid), errors.Is(err, orderapp.ErrRefundActionAlreadyHandled), errors.Is(err, orderapp.ErrRefundActionNeedsReview), errors.Is(err, orderapp.ErrRefundActionRequiresOfficial):
		writeErr(writer, http.StatusConflict, err.Error())
	case errors.Is(err, orderapp.ErrRefundDetailUnavailable):
		writeErr(writer, http.StatusServiceUnavailable, err.Error())
	default:
		writeErr(writer, http.StatusBadGateway, "读取闲鱼退款详情失败: "+err.Error())
	}
}
