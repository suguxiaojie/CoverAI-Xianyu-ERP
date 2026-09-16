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

// orderPriceFieldDTO 是动态 render／submit 共用的具名金额字段。
type orderPriceFieldDTO struct {
	// Key 是平台 render 返回的字段名。
	Key string `json:"key"`
	// Name 是字段展示名称。
	Name string `json:"name"`
	// PrefixText 是金额输入前缀。
	PrefixText string `json:"prefix_text"`
	// Value 是单位为元的规范金额。
	Value string `json:"value"`
	// ReadOnly 表示该字段是否不可编辑。
	ReadOnly bool `json:"read_only"`
}

// orderAdjustPriceFormResponse 是只读动态改价表单响应。
type orderAdjustPriceFormResponse struct {
	// OrderID 是当前待付款订单标识。
	OrderID string `json:"order_id"`
	// AccountID 是卡片所属卖家账号。
	AccountID string `json:"account_id"`
	// Title 是平台表单标题。
	Title string `json:"title"`
	// Fields 是平台当前要求展示和提交的金额字段。
	Fields []orderPriceFieldDTO `json:"fields"`
}

// orderAdjustPriceRequest 是用户二次确认后的具名真实改价请求。
type orderAdjustPriceRequest struct {
	// AccountID 是卡片所属卖家账号。
	AccountID string `json:"account_id"`
	// Fields 是按 render key 提交的目标元金额列表。
	Fields []orderPriceFieldDTO `json:"fields"`
}

// orderAdjustPriceResponse 是平台真实改价的具名确定性结果。
type orderAdjustPriceResponse struct {
	// Success 表示平台明确返回 data.success=true。
	Success bool `json:"success"`
	// Status 是 succeeded、failed、needs_review 或 succeeded_with_warning。
	Status string `json:"status"`
	// Message 是可以直接展示的非敏感结果说明。
	Message string `json:"message"`
	// OrderID 是已提交的平台订单标识。
	OrderID string `json:"order_id"`
	// Fields 是本次实际提交的规范元金额。
	Fields []orderPriceFieldDTO `json:"fields"`
}

// PriceAdjustmentForm 调用订单应用服务读取动态改价表单。
func (a *orderHTTPAdapter) PriceAdjustmentForm(ctx context.Context, request orderapp.PriceAdjustmentFormRequest) (orderapp.PriceAdjustmentFormResult, error) {
	return a.services.PriceAdjustmentForm(ctx, request)
}

// AdjustPrice 调用订单应用服务执行用户确认后的真实改价。
func (a *orderHTTPAdapter) AdjustPrice(ctx context.Context, request orderapp.PriceAdjustmentRequest) (orderapp.PriceAdjustmentResult, error) {
	return a.services.AdjustPrice(ctx, request)
}

// getOrderAdjustPriceForm 返回当前待付款卡片的只读平台动态字段。
func (s *Server) getOrderAdjustPriceForm(writer http.ResponseWriter, request *http.Request) {
	// orderID、accountID 是路由订单和查询中的卖家账号标识。
	orderID, accountID := strings.TrimSpace(chi.URLParam(request, "order_id")), strings.TrimSpace(request.URL.Query().Get("account_id"))
	if orderID == "" || accountID == "" {
		writeErr(writer, http.StatusBadRequest, "缺少订单 ID 或账号 ID")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// result、formErr 是应用层动态表单和错误。
	result, formErr := s.orders().PriceAdjustmentForm(request.Context(), orderapp.PriceAdjustmentFormRequest{UserID: session.UserID, AccountID: accountID, OrderID: orderID})
	if formErr != nil {
		writePriceAdjustmentError(writer, formErr, "读取闲鱼改价表单失败")
		return
	}
	writeJSON(writer, http.StatusOK, orderAdjustPriceFormResponse{OrderID: result.OrderID, AccountID: result.AccountID, Title: result.Title, Fields: priceFieldDTOs(result.Fields)})
}

// adjustOrderPrice 执行一次用户二次确认后的真实待付款订单改价。
func (s *Server) adjustOrderPrice(writer http.ResponseWriter, request *http.Request) {
	// orderID 是路由中的待付款平台订单标识。
	orderID := strings.TrimSpace(chi.URLParam(request, "order_id"))
	// input 是具名 JSON 请求。
	var input orderAdjustPriceRequest
	if orderID == "" || decodeJSON(request, &input) != nil || strings.TrimSpace(input.AccountID) == "" {
		writeErr(writer, http.StatusBadRequest, "改价请求缺少订单、账号或有效字段")
		return
	}
	// session 是当前认证用户会话。
	session := auth.SessionFromContext(request.Context())
	// fields 是应用层只接收 key／value 的动态金额列表。
	fields := make([]orderapp.PriceAdjustmentInput, 0, len(input.Fields))
	// field 是当前待转换的传输字段。
	for _, field := range input.Fields {
		fields = append(fields, orderapp.PriceAdjustmentInput{Key: field.Key, Value: field.Value})
	}
	// result、adjustErr 是应用层真实改价结果和错误。
	result, adjustErr := s.orders().AdjustPrice(request.Context(), orderapp.PriceAdjustmentRequest{UserID: session.UserID,
		AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID, Fields: fields})
	if adjustErr != nil {
		writePriceAdjustmentError(writer, adjustErr, "闲鱼订单改价失败")
		return
	}
	// responseFields 把应用结果转换为具名传输字段。
	responseFields := make([]orderPriceFieldDTO, 0, len(result.Fields))
	// field 是当前待转换的应用结果字段。
	for _, field := range result.Fields {
		responseFields = append(responseFields, orderPriceFieldDTO{Key: field.Key, Value: field.Value})
	}
	writeJSON(writer, http.StatusOK, orderAdjustPriceResponse{Success: result.Success, Status: result.Status,
		Message: result.Message, OrderID: result.OrderID, Fields: responseFields})
}

// priceFieldDTOs 将应用层动态表单转换为 HTTP 具名字段。
func priceFieldDTOs(fields []orderapp.PriceAdjustmentField) []orderPriceFieldDTO {
	// result 保存与平台顺序一致的传输字段。
	result := make([]orderPriceFieldDTO, 0, len(fields))
	// field 是当前待转换的应用字段。
	for _, field := range fields {
		result = append(result, orderPriceFieldDTO{Key: field.Key, Name: field.Name, PrefixText: field.PrefixText, Value: field.Value, ReadOnly: field.ReadOnly})
	}
	return result
}

// writePriceAdjustmentError 把应用错误稳定映射为统一 HTTP 状态和错误 envelope。
func writePriceAdjustmentError(writer http.ResponseWriter, err error, fallback string) {
	// validationErr 保存金额或字段校验错误。
	var validationErr *orderapp.ValidationError
	switch {
	case errors.As(err, &validationErr), errors.Is(err, orderapp.ErrPriceAdjustmentNoChange):
		writeErr(writer, http.StatusBadRequest, err.Error())
	case errors.Is(err, orderapp.ErrForbidden):
		writeErr(writer, http.StatusForbidden, "无权操作此订单")
	case errors.Is(err, orderapp.ErrNotFound):
		writeErr(writer, http.StatusNotFound, "订单不存在")
	case errors.Is(err, orderapp.ErrPriceAdjustmentNotEligible), errors.Is(err, orderapp.ErrPriceAdjustmentAlreadyHandled), errors.Is(err, orderapp.ErrPriceAdjustmentNeedsReview):
		writeErr(writer, http.StatusConflict, err.Error())
	case errors.Is(err, orderapp.ErrPriceAdjustmentUnavailable):
		writeErr(writer, http.StatusServiceUnavailable, err.Error())
	default:
		writeErr(writer, http.StatusBadGateway, fallback+": "+err.Error())
	}
}
