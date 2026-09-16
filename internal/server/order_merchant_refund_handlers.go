package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/auth"
)

// merchantRefundAccountRequest 是 Merchant 退款请求共用的账号 DTO。
type merchantRefundAccountRequest struct {
	// AccountID 是退款订单所属卖家账号。
	AccountID string `json:"account_id"`
}

// merchantRefundVerificationResponse 是支付宝 PC 验证页面响应。
type merchantRefundVerificationResponse struct {
	// SessionID 是不包含 authToken 的随机短期会话。
	SessionID string `json:"session_id"`
	// VerifyURL 是支付宝跨域 iframe 地址。
	VerifyURL string `json:"verify_url"`
	// VerifyOrigin 是前端 postMessage 必须精确匹配的来源。
	VerifyOrigin string `json:"verify_origin"`
	// ExpiresAt 是会话过期 Unix 秒。
	ExpiresAt int64 `json:"expires_at"`
}

// merchantRefundRefuseReasonDTO 是动态拒绝原因。
type merchantRefundRefuseReasonDTO struct {
	// ID 是平台 refuseReasonId。
	ID string `json:"id"`
	// Name 是平台展示原因。
	Name string `json:"name"`
	// RequiresApp 表示该原因只能到手机 App 处理。
	RequiresApp bool `json:"requires_app"`
}

// merchantRefundRefuseFormResponse 是拒绝退款动态表单。
type merchantRefundRefuseFormResponse struct {
	// RefundID 是当前退款标识。
	RefundID string `json:"refund_id"`
	// Reasons 是平台动态原因。
	Reasons []merchantRefundRefuseReasonDTO `json:"reasons"`
	// SelectedReasonID 是当前 render 原因。
	SelectedReasonID string `json:"selected_reason_id,omitempty"`
	// ProofRequired 表示必须提交图片凭证。
	ProofRequired bool `json:"proof_required"`
	// ProofPlaceholder 是说明输入提示。
	ProofPlaceholder string `json:"proof_placeholder,omitempty"`
	// NegotiationEnabled 表示允许协商金额。
	NegotiationEnabled bool `json:"negotiation_enabled"`
	// NegotiationType 是平台动态协商类型。
	NegotiationType string `json:"negotiation_type,omitempty"`
	// MinCents、MaxCents 是协商金额整数分范围。
	MinCents int64 `json:"min_cents"`
	MaxCents int64 `json:"max_cents"`
}

// merchantRefundRefuseRequest 是最终拒绝退款请求。
type merchantRefundRefuseRequest struct {
	merchantRefundAccountRequest
	// ReasonID 是平台动态原因。
	ReasonID string `json:"reason_id"`
	// Description 是补充描述。
	Description string `json:"description"`
	// NegotiationCents 是可选协商金额整数分。
	NegotiationCents int64 `json:"negotiation_cents"`
}

// StartMerchantRefundVerification 调用订单应用服务创建短期支付验证会话。
func (adapter *orderHTTPAdapter) StartMerchantRefundVerification(ctx context.Context, request orderapp.MerchantRefundVerificationStartRequest) (orderapp.MerchantRefundVerificationStartResult, error) {
	return adapter.services.StartMerchantRefundVerification(ctx, request)
}

// CompleteMerchantRefundVerification 调用订单应用服务执行验证后的最终退款。
func (adapter *orderHTTPAdapter) CompleteMerchantRefundVerification(ctx context.Context, request orderapp.MerchantRefundVerificationCompleteRequest) (orderapp.RefundActionResult, error) {
	return adapter.services.CompleteMerchantRefundVerification(ctx, request)
}

// MerchantRefundRefuseForm 调用订单应用服务读取动态拒绝表单。
func (adapter *orderHTTPAdapter) MerchantRefundRefuseForm(ctx context.Context, request orderapp.MerchantRefundRefuseFormRequest) (orderapp.MerchantRefundRefuseFormResult, error) {
	return adapter.services.MerchantRefundRefuseForm(ctx, request)
}

// RefuseMerchantRefund 调用订单应用服务执行最终拒绝退款。
func (adapter *orderHTTPAdapter) RefuseMerchantRefund(ctx context.Context, request orderapp.MerchantRefundRefuseRequest) (orderapp.RefundActionResult, error) {
	return adapter.services.RefuseMerchantRefund(ctx, request)
}

// startMerchantRefundVerification 创建支付宝 PC 验证 iframe 会话，不执行退款。
func (server *Server) startMerchantRefundVerification(writer http.ResponseWriter, request *http.Request) {
	// orderID 是当前路由中的平台订单标识。
	orderID := strings.TrimSpace(chi.URLParam(request, "order_id"))
	// input 只接收订单所属卖家账号。
	var input merchantRefundAccountRequest
	if orderID == "" || decodeJSON(request, &input) != nil || strings.TrimSpace(input.AccountID) == "" {
		writeErr(writer, http.StatusBadRequest, "退款验证请求缺少订单或账号")
		return
	}
	// session 是当前登录管理员会话。
	session := auth.SessionFromContext(request.Context())
	// result、startErr 是短期验证会话和创建错误。
	result, startErr := server.orders().StartMerchantRefundVerification(request.Context(), orderapp.MerchantRefundVerificationStartRequest{UserID: session.UserID, AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID})
	if startErr != nil {
		writeRefundDetailError(writer, startErr)
		return
	}
	writeJSON(writer, http.StatusOK, merchantRefundVerificationResponse{SessionID: result.SessionID, VerifyURL: result.VerifyURL, VerifyOrigin: result.VerifyOrigin, ExpiresAt: result.ExpiresAt})
}

// completeMerchantRefundVerification 执行用户已在支付宝 iframe 验证后的最终退款。
func (server *Server) completeMerchantRefundVerification(writer http.ResponseWriter, request *http.Request) {
	// orderID、sessionID 是路由中的订单和短期验证会话标识。
	orderID, sessionID := strings.TrimSpace(chi.URLParam(request, "order_id")), strings.TrimSpace(chi.URLParam(request, "session_id"))
	// input 只接收订单所属卖家账号。
	var input merchantRefundAccountRequest
	if orderID == "" || sessionID == "" || decodeJSON(request, &input) != nil || strings.TrimSpace(input.AccountID) == "" {
		writeErr(writer, http.StatusBadRequest, "退款验证完成请求无效")
		return
	}
	// session 是当前登录管理员会话。
	session := auth.SessionFromContext(request.Context())
	// result、completeErr 是最终 Merchant 同意结果和提交错误。
	result, completeErr := server.orders().CompleteMerchantRefundVerification(request.Context(), orderapp.MerchantRefundVerificationCompleteRequest{UserID: session.UserID, AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID, SessionID: sessionID})
	if completeErr != nil {
		writeRefundDetailError(writer, completeErr)
		return
	}
	writeJSON(writer, http.StatusOK, orderRefundActionResponse{Success: result.Success, Status: result.Status, Message: result.Message, OrderID: result.OrderID, RefundID: result.RefundID, Action: result.Action})
}

// getMerchantRefundRefuseForm 返回当前退款动态拒绝原因和要求。
func (server *Server) getMerchantRefundRefuseForm(writer http.ResponseWriter, request *http.Request) {
	// orderID、accountID 是当前表单对应的订单和卖家账号。
	orderID, accountID := strings.TrimSpace(chi.URLParam(request, "order_id")), strings.TrimSpace(request.URL.Query().Get("account_id"))
	if orderID == "" || accountID == "" {
		writeErr(writer, http.StatusBadRequest, "拒绝退款表单缺少订单或账号")
		return
	}
	// session 是当前登录管理员会话。
	session := auth.SessionFromContext(request.Context())
	// result、formErr 是平台动态拒绝表单和读取错误。
	result, formErr := server.orders().MerchantRefundRefuseForm(request.Context(), orderapp.MerchantRefundRefuseFormRequest{UserID: session.UserID, AccountID: accountID, OrderID: orderID, ReasonID: strings.TrimSpace(request.URL.Query().Get("reason_id"))})
	if formErr != nil {
		writeRefundDetailError(writer, formErr)
		return
	}
	// reasons 是去除应用内部字段后的 HTTP 拒绝原因列表。
	reasons := make([]merchantRefundRefuseReasonDTO, 0, len(result.Reasons))
	for _, reason := range result.Reasons { // reason 是当前平台动态拒绝原因。
		reasons = append(reasons, merchantRefundRefuseReasonDTO{ID: reason.ID, Name: reason.Name, RequiresApp: reason.RequiresApp})
	}
	writeJSON(writer, http.StatusOK, merchantRefundRefuseFormResponse{RefundID: result.RefundID, Reasons: reasons, SelectedReasonID: result.SelectedReasonID,
		ProofRequired: result.ProofRequired, ProofPlaceholder: result.ProofPlaceholder, NegotiationEnabled: result.NegotiationEnabled,
		NegotiationType: result.NegotiationType, MinCents: result.MinCents, MaxCents: result.MaxCents})
}

// refuseMerchantRefund 执行用户应用内二次确认后的最终拒绝退款。
func (server *Server) refuseMerchantRefund(writer http.ResponseWriter, request *http.Request) {
	// orderID 是当前路由中的平台订单标识。
	orderID := strings.TrimSpace(chi.URLParam(request, "order_id"))
	// input、images、decodeErr 是用户二次确认的字段、内存图片和解析错误。
	input, images, decodeErr := decodeMerchantRefundRefuseRequest(writer, request)
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	if orderID == "" || decodeErr != nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.ReasonID) == "" {
		writeErr(writer, http.StatusBadRequest, "拒绝退款请求无效")
		return
	}
	// session 是当前登录管理员会话。
	session := auth.SessionFromContext(request.Context())
	// result、refuseErr 是最终 Merchant 拒绝结果和提交错误。
	result, refuseErr := server.orders().RefuseMerchantRefund(request.Context(), orderapp.MerchantRefundRefuseRequest{UserID: session.UserID,
		AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID, ReasonID: strings.TrimSpace(input.ReasonID), Description: input.Description,
		NegotiationCents: input.NegotiationCents, Images: images})
	if refuseErr != nil {
		writeRefundDetailError(writer, refuseErr)
		return
	}
	writeJSON(writer, http.StatusOK, orderRefundActionResponse{Success: result.Success, Status: result.Status, Message: result.Message, OrderID: result.OrderID, RefundID: result.RefundID, Action: result.Action})
}

// decodeMerchantRefundRefuseRequest 兼容旧 JSON，并解析带图片的新 multipart 拒绝退款表单。
func decodeMerchantRefundRefuseRequest(writer http.ResponseWriter, request *http.Request) (merchantRefundRefuseRequest, []orderapp.ShipmentEvidenceImage, error) {
	// input 保存规范化前的账号、原因、说明和协商金额。
	var input merchantRefundRefuseRequest
	if request == nil {
		return input, nil, errors.New("退款请求为空")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Type"))), "multipart/form-data") {
		return input, nil, decodeJSON(request, &input)
	}
	request.Body = http.MaxBytesReader(writer, request.Body, shipmentMultipartMaxBytes)
	// parseErr 是 multipart 边界或总大小不合法时的解析错误。
	if parseErr := request.ParseMultipartForm(shipmentMultipartMaxBytes); parseErr != nil {
		return input, nil, errors.New("退款凭证表单过大或格式错误")
	}
	input.AccountID = strings.TrimSpace(request.FormValue("account_id"))
	input.ReasonID = strings.TrimSpace(request.FormValue("reason_id"))
	input.Description = request.FormValue("description")
	// negotiationText 是前端提交的可选整数分文本。
	negotiationText := strings.TrimSpace(request.FormValue("negotiation_cents"))
	if negotiationText != "" {
		// negotiationCents、parseErr 是协商金额整数分和转换错误。
		negotiationCents, parseErr := strconv.ParseInt(negotiationText, 10, 64)
		if parseErr != nil {
			return input, nil, errors.New("协商退款金额格式错误")
		}
		input.NegotiationCents = negotiationCents
	}
	// images、imageErr 是经过数量、大小和真实 MIME 检查的内存退款凭证。
	images, imageErr := shipmentEvidenceImages(request)
	return input, images, imageErr
}
