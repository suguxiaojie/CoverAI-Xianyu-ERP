package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/auth"
)

// orderCloseReasonsResponse 是当前可取消订单的动态平台原因响应。
type orderCloseReasonsResponse struct {
	// OrderID 是当前原因列表对应的平台订单号。
	OrderID string `json:"order_id"`
	// AccountID 是卡片所属卖家账号。
	AccountID string `json:"account_id"`
	// Reasons 是平台当前允许选择的关闭原因。
	Reasons []string `json:"reasons"`
}

// orderCloseEligibilityResponse 是 Chat 顶部关单入口使用的纯本地资格结果。
type orderCloseEligibilityResponse struct {
	// Eligible 只在订单权威状态和聊天角色证据均允许关单时为真。
	Eligible bool `json:"eligible"`
	// OrderID 是本次检查的平台订单号。
	OrderID string `json:"order_id"`
	// AccountID 是本次检查的卖家账号。
	AccountID string `json:"account_id"`
	// Reason 是不可关单时的安全说明。
	Reason string `json:"reason,omitempty"`
	// Stage 是 pending_payment 或 pending_ship，供前端展示正确资金提示。
	Stage string `json:"stage,omitempty"`
}

// orderCloseRequest 是用户二次确认后的具名卖家关单请求。
type orderCloseRequest struct {
	// AccountID 是卡片所属卖家账号。
	AccountID string `json:"account_id"`
	// Reason 是从平台动态列表选择的关闭原因。
	Reason string `json:"reason"`
}

// orderCloseResponse 是平台真实关单的具名确定性结果。
type orderCloseResponse struct {
	// Success 只在平台明确受理关单时为真。
	Success bool `json:"success"`
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string `json:"status"`
	// Message 是用户可见且不含凭证的结果说明。
	Message string `json:"message"`
	// OrderID 是已提交的平台订单号。
	OrderID string `json:"order_id"`
}

// CloseOrderReasons 调用订单应用服务读取动态关闭原因。
func (a *orderHTTPAdapter) CloseOrderReasons(ctx context.Context, request orderapp.CloseOrderReasonsRequest) (orderapp.CloseOrderReasonsResult, error) {
	return a.services.CloseOrderReasons(ctx, request)
}

// CloseOrderEligibility 调用订单应用服务执行纯本地关单资格查询。
func (a *orderHTTPAdapter) CloseOrderEligibility(ctx context.Context, request orderapp.CloseOrderEligibilityRequest) (orderapp.CloseOrderEligibilityResult, error) {
	return a.services.CloseOrderEligibility(ctx, request)
}

// CloseOrder 调用订单应用服务执行用户确认后的真实卖家关单。
func (a *orderHTTPAdapter) CloseOrder(ctx context.Context, request orderapp.CloseOrderRequest) (orderapp.CloseOrderResult, error) {
	return a.services.CloseOrder(ctx, request)
}

// getOrderCloseReasons 返回当前卖家可取消订单的动态平台原因。
func (s *Server) getOrderCloseReasons(writer http.ResponseWriter, request *http.Request) {
	// orderID、accountID 是路由订单号和查询中的卖家账号。
	orderID, accountID := strings.TrimSpace(chi.URLParam(request, "order_id")), strings.TrimSpace(request.URL.Query().Get("account_id"))
	if orderID == "" || accountID == "" {
		writeErr(writer, http.StatusBadRequest, "缺少订单 ID 或账号 ID")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// result、reasonErr 是应用层动态原因和错误。
	result, reasonErr := s.orders().CloseOrderReasons(request.Context(), orderapp.CloseOrderReasonsRequest{UserID: session.UserID, AccountID: accountID, OrderID: orderID})
	if reasonErr != nil {
		writeCloseOrderError(writer, reasonErr, "读取闲鱼关闭原因失败")
		return
	}
	writeJSON(writer, http.StatusOK, orderCloseReasonsResponse{OrderID: result.OrderID, AccountID: result.AccountID, Reasons: result.Reasons})
}

// getOrderCloseEligibility 返回订单表、聊天角色和幂等共同确认的本地关单资格。
func (s *Server) getOrderCloseEligibility(writer http.ResponseWriter, request *http.Request) {
	// orderID、accountID 是路由订单号和查询中的卖家账号。
	orderID, accountID := strings.TrimSpace(chi.URLParam(request, "order_id")), strings.TrimSpace(request.URL.Query().Get("account_id"))
	if orderID == "" || accountID == "" {
		writeErr(writer, http.StatusBadRequest, "缺少订单 ID 或账号 ID")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// result、eligibilityErr 是应用层纯本地资格和错误。
	result, eligibilityErr := s.orders().CloseOrderEligibility(request.Context(), orderapp.CloseOrderEligibilityRequest{UserID: session.UserID, AccountID: accountID, OrderID: orderID})
	if eligibilityErr != nil {
		writeCloseOrderError(writer, eligibilityErr, "读取订单取消资格失败")
		return
	}
	writeJSON(writer, http.StatusOK, orderCloseEligibilityResponse{Eligible: result.Eligible, OrderID: result.OrderID, AccountID: result.AccountID, Reason: result.Reason, Stage: result.Stage})
}

// closeOrderBySeller 执行一次用户二次确认后的真实卖家取消订单。
func (s *Server) closeOrderBySeller(writer http.ResponseWriter, request *http.Request) {
	// orderID 是路由中的待取消平台订单号。
	orderID := strings.TrimSpace(chi.URLParam(request, "order_id"))
	// input 是具名 JSON 请求。
	var input orderCloseRequest
	if orderID == "" || decodeJSON(request, &input) != nil || strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.Reason) == "" {
		writeErr(writer, http.StatusBadRequest, "关闭订单请求缺少订单、账号或原因")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// result、closeErr 是应用层真实关单结果和错误。
	result, closeErr := s.orders().CloseOrder(request.Context(), orderapp.CloseOrderRequest{UserID: session.UserID,
		AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID, Reason: strings.TrimSpace(input.Reason)})
	if closeErr != nil {
		writeCloseOrderError(writer, closeErr, "闲鱼关闭订单失败")
		return
	}
	writeJSON(writer, http.StatusOK, orderCloseResponse{Success: result.Success, Status: result.Status, Message: result.Message, OrderID: result.OrderID})
}

// writeCloseOrderError 把应用错误稳定映射为统一 HTTP 状态和错误 envelope。
func writeCloseOrderError(writer http.ResponseWriter, err error, fallback string) {
	// validationErr 保存订单号或请求字段校验错误。
	var validationErr *orderapp.ValidationError
	switch {
	case errors.As(err, &validationErr), errors.Is(err, orderapp.ErrCloseOrderReasonInvalid):
		writeErr(writer, http.StatusBadRequest, err.Error())
	case errors.Is(err, orderapp.ErrForbidden):
		writeErr(writer, http.StatusForbidden, "无权操作此订单")
	case errors.Is(err, orderapp.ErrNotFound):
		writeErr(writer, http.StatusNotFound, "订单不存在")
	case errors.Is(err, orderapp.ErrCloseOrderNotEligible), errors.Is(err, orderapp.ErrCloseOrderAlreadyHandled), errors.Is(err, orderapp.ErrCloseOrderNeedsReview):
		writeErr(writer, http.StatusConflict, err.Error())
	case errors.Is(err, orderapp.ErrCloseOrderUnavailable):
		writeErr(writer, http.StatusServiceUnavailable, err.Error())
	default:
		writeErr(writer, http.StatusBadGateway, fallback+": "+err.Error())
	}
}
