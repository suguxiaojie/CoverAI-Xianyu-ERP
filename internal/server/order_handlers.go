package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/auth"
)

// refreshOrderChunkSize 用于本次流程后续判断的refresh订单Chunk数量
const refreshOrderChunkSize = 100

// orderListAmountPattern 接受可选人民币符号和最多两位小数的非负普通金额。
var orderListAmountPattern = regexp.MustCompile(`^(?:¥|￥)?(0|[1-9][0-9]*)(?:\.([0-9]{1,2}))?$`)

// refreshTarget 用于本次流程后续判断的refreshTarget
type refreshTarget struct {
	OrderID       string
	CurrentStatus string
}

// mountOrders 订单端点（真实实现）。
func (s *Server) mountOrdersReal(r chi.Router) {
	r.Get("/api/orders", s.listOrders)
	r.Get("/api/orders/{order_id}", s.getOrder)
	s.mountOrderRefreshJobRoutes(r, "/api")
	r.Post("/api/orders/{order_id}/refresh", s.refreshSingleOrder)
	r.Post("/api/orders/manual-ship", s.manualShipOrders)
	r.Post("/api/orders/import", s.importOrders)
	r.Delete("/api/orders/{order_id}", s.deleteOrder)
	r.Put("/api/orders/{order_id}", s.updateOrder)
}

// listOrders 分页查询当前用户订单。
func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	// request、parseErr 保存已经完成时间格式校验的订单列表查询参数和解析错误。
	request, parseErr := parseOrderListRequest(r)
	if parseErr != nil {
		writeErr(w, http.StatusBadRequest, parseErr.Error())
		return
	}
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// result、err 用于本次流程后续判断的result、err
	result, err := s.orders().List(r.Context(), orderListQuery{
		UserID: sess.UserID, CookieID: request.CookieID,
		Status: request.Status, Search: request.Search,
		CreatedFrom: request.CreatedFrom, CreatedTo: request.CreatedTo,
		MinAmountCents: request.MinAmountCents, MaxAmountCents: request.MaxAmountCents,
		Page: request.Page, PageSize: request.PageSize,
	})
	if errors.Is(err, orderapp.ErrForbidden) {
		writeErr(w, http.StatusForbidden, "无权限操作该账号")
		return
	}
	if errors.Is(err, orderapp.ErrInvalidListTimeRange) {
		writeErr(w, http.StatusBadRequest, "结束时间必须晚于开始时间")
		return
	}
	if errors.Is(err, orderapp.ErrInvalidListAmountRange) {
		writeErr(w, http.StatusBadRequest, "最高金额必须大于或等于最低金额")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, orderListResponse{
		Success:           true,
		Data:              result.Orders,
		Total:             result.Total,
		Page:              result.Page,
		PageSize:          result.PageSize,
		TotalPages:        result.TotalPages,
		SettlementSummary: result.SettlementSummary,
	})
}

// orderListRequestDTO 保存订单列表 URL 查询参数解析后的具名结果。
type orderListRequestDTO struct {
	// CookieID 是可选账号筛选条件。
	CookieID string
	// Status 是可选订单状态筛选条件。
	Status string
	// Search 是订单号、商品或买家搜索词。
	Search string
	// CreatedFrom 是 RFC3339 订单创建时间包含下界。
	CreatedFrom time.Time
	// CreatedTo 是 RFC3339 订单创建时间排除上界。
	CreatedTo time.Time
	// MinAmountCents 是实付金额的可选包含下界，单位为人民币分。
	MinAmountCents *int64
	// MaxAmountCents 是实付金额的可选包含上界，单位为人民币分。
	MaxAmountCents *int64
	// Page 是从一开始的页码。
	Page int
	// PageSize 是单页订单数。
	PageSize int
}

// parseOptionalOrderListTime 解析可选 RFC3339 时间；空字符串返回零时间。
func parseOptionalOrderListTime(raw string, fieldName string) (time.Time, error) {
	// normalized 是去除首尾空格后的时间文本。
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return time.Time{}, nil
	}
	// parsed、parseErr 是 RFC3339Nano 解析结果和格式错误。
	parsed, parseErr := time.Parse(time.RFC3339Nano, normalized)
	if parseErr != nil {
		return time.Time{}, fmt.Errorf("%s 必须是 RFC3339 时间", fieldName)
	}
	return parsed, nil
}

// parseOptionalOrderListAmount 把可选元金额严格转换为整数分；空值表示不限制该边界。
func parseOptionalOrderListAmount(raw string, fieldName string) (*int64, error) {
	// normalized 是去除首尾空格后的金额文本。
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return nil, nil
	}
	// matches 保存金额的整数元和可选小数部分。
	matches := orderListAmountPattern.FindStringSubmatch(normalized)
	if len(matches) != 3 {
		return nil, fmt.Errorf("%s 必须是最多两位小数的非负金额", fieldName)
	}
	// whole、wholeErr 是金额整数元部分及其解析错误。
	whole, wholeErr := strconv.ParseUint(matches[1], 10, 64)
	if wholeErr != nil || whole > math.MaxInt64/100 {
		return nil, fmt.Errorf("%s 超出支持范围", fieldName)
	}
	// fraction 是补齐到两位的金额小数部分。
	fraction := matches[2]
	if len(fraction) == 0 {
		fraction = "00"
	} else if len(fraction) == 1 {
		fraction += "0"
	}
	// fractionValue 是金额分部分；正则已保证它只包含数字。
	fractionValue, _ := strconv.ParseUint(fraction, 10, 64)
	// cents 是最终返回的非负人民币分值。
	cents := int64(whole*100 + fractionValue)
	return &cents, nil
}

// parseOrderListRequest 解析订单列表查询参数，并拒绝倒置或相等的双边时间范围。
func parseOrderListRequest(r *http.Request) (orderListRequestDTO, error) {
	// query 保存当前订单列表 URL 查询参数。
	query := r.URL.Query()
	// createdFrom、fromErr 保存订单创建时间下界及其格式错误。
	createdFrom, fromErr := parseOptionalOrderListTime(query.Get("created_from"), "created_from")
	if fromErr != nil {
		return orderListRequestDTO{}, fromErr
	}
	// createdTo、toErr 保存订单创建时间上界及其格式错误。
	createdTo, toErr := parseOptionalOrderListTime(query.Get("created_to"), "created_to")
	if toErr != nil {
		return orderListRequestDTO{}, toErr
	}
	if !createdFrom.IsZero() && !createdTo.IsZero() && !createdTo.After(createdFrom) {
		return orderListRequestDTO{}, errors.New("created_to 必须晚于 created_from")
	}
	// minAmountCents、minAmountErr 保存最低实付金额的分值和格式错误。
	minAmountCents, minAmountErr := parseOptionalOrderListAmount(query.Get("min_amount"), "min_amount")
	if minAmountErr != nil {
		return orderListRequestDTO{}, minAmountErr
	}
	// maxAmountCents、maxAmountErr 保存最高实付金额的分值和格式错误。
	maxAmountCents, maxAmountErr := parseOptionalOrderListAmount(query.Get("max_amount"), "max_amount")
	if maxAmountErr != nil {
		return orderListRequestDTO{}, maxAmountErr
	}
	if minAmountCents != nil && maxAmountCents != nil && *minAmountCents > *maxAmountCents {
		return orderListRequestDTO{}, errors.New("max_amount 必须大于或等于 min_amount")
	}
	return orderListRequestDTO{
		CookieID: query.Get("cookie_id"), Status: query.Get("status"), Search: query.Get("search"),
		CreatedFrom: createdFrom, CreatedTo: createdTo,
		MinAmountCents: minAmountCents, MaxAmountCents: maxAmountCents,
		Page: atoiDefault(query.Get("page"), 1), PageSize: atoiDefault(query.Get("page_size"), 20),
	}, nil
}

// getOrder 订单详情。
func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	// orderID 用于本次流程后续判断的订单ID
	orderID := chi.URLParam(r, "order_id")
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// result、err 用于本次流程后续判断的result、err
	result, err := s.orders().GetView(r.Context(), sess.UserID, orderID)
	if err != nil {
		if errors.Is(err, orderapp.ErrForbidden) {
			writeErr(w, http.StatusForbidden, "无权操作此订单")
		} else {
			writeErr(w, http.StatusNotFound, "订单不存在")
		}
		return
	}
	writeJSON(w, http.StatusOK, orderDetailResponse{
		orderDTO: result.Order, Success: true, Data: result.Order,
	})
}

// chunkRefreshTargets 封装chunkRefreshTargets业务协调。
func chunkRefreshTargets(targets []refreshTarget, size int) [][]refreshTarget {
	if size <= 0 {
		size = refreshOrderChunkSize
	}
	// chunks 用于本次流程后续判断的chunks
	chunks := make([][]refreshTarget, 0, (len(targets)+size-1)/size)
	for // start 用于本次流程后续判断的开始
	start := 0; start < len(targets); start += size {
		// end 用于本次流程后续判断的结束
		end := start + size
		if end > len(targets) {
			end = len(targets)
		}
		chunks = append(chunks, targets[start:end])
	}
	return chunks
}

// missingRefreshTargetIDs 封装missingRefreshTargetIDs业务协调。
func missingRefreshTargetIDs(targets []refreshTarget, seen map[string]struct{}) []string {
	// missing 用于本次流程后续判断的missing
	missing := make([]string, 0)
	// target 表示当前遍历过程中的target
	for _, target := range targets {
		if // ok 用于本次流程后续判断的ok
		_, ok := seen[target.OrderID]; !ok {
			missing = append(missing, target.OrderID)
		}
	}
	return missing
}

func (s *Server) refreshSingleOrder(w http.ResponseWriter, r *http.Request) { // refreshSingleOrder 保持单订单刷新与批量刷新使用相同的详情 DTO。
	orderID := chi.URLParam(r, "order_id")
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// result、err 用于本次流程后续判断的result、err
	result, err := s.orders().RefreshSingle(r.Context(), sess.UserID, orderID)
	if errors.Is(err, orderapp.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "订单不存在")
		return
	}
	if errors.Is(err, orderapp.ErrForbidden) {
		writeErr(w, http.StatusForbidden, "无权操作此订单")
		return
	}
	if errors.Is(err, errOrderDetailUnsupported) {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if errors.Is(err, errOrderCredentialChanged) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if !result.Success {
		writeErr(w, http.StatusInternalServerError, "更新订单失败")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// deleteOrder 逻辑删除订单，保留订单历史，避免破坏自动化审计数据。
func (s *Server) deleteOrder(w http.ResponseWriter, r *http.Request) {
	// orderID 用于本次流程后续判断的订单ID
	orderID := chi.URLParam(r, "order_id")
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	if // err 用于本次流程后续判断的err
	err := s.orders().Delete(r.Context(), sess.UserID, orderID); err != nil {
		if errors.Is(err, orderapp.ErrForbidden) {
			writeErr(w, http.StatusForbidden, "无权操作此订单")
		} else {
			writeErr(w, http.StatusNotFound, "订单不存在")
		}
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// updateOrder 更新订单（手动发货等）。
func (s *Server) updateOrder(w http.ResponseWriter, r *http.Request) {
	// orderID 用于本次流程后续判断的订单ID
	orderID := chi.URLParam(r, "order_id")
	// req 保存具名订单更新请求，兼容旧客户端的状态别名和数值字段。
	var req orderUpdateRequestDTO
	if // err 用于本次流程后续判断的err
	err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	// status 保存兼容字段 order_status/status 合并后的状态值。
	status := req.OrderStatus
	if status == nil {
		status = req.Status
	}
	// stringPtrFromAny 用于本次流程后续判断的stringPtrFromAny
	stringPtrFromAny := func(value *any) *string {
		if value == nil {
			return nil
		}
		// v 用于本次流程后续判断的v
		v := stringFromAny(*value)
		return &v
	}
	// amount 保存兼容 JSON 数值转换后的订单金额。
	amount := stringPtrFromAny(req.Amount)
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	if // err 用于本次流程后续判断的err
	err := s.orders().Update(r.Context(), sess.UserID, orderID, orderUpdateRequest{
		OrderStatus: status, ItemID: req.ItemID, BuyerID: req.BuyerID, SpecName: req.SpecName,
		SpecValue: req.SpecValue, Quantity: stringPtrFromAny(req.Quantity), Amount: amount,
		ReceiverName: req.ReceiverName, ReceiverPhone: req.ReceiverPhone, ReceiverAddress: req.ReceiverAddress,
		ReceiverCity: req.ReceiverCity, ChatID: req.ChatID, SystemShipped: req.SystemShipped, ItemTitle: req.ItemTitle,
	}); err != nil {
		if // kind、classified 用于本次流程后续判断的kind、classified
		kind, classified := orderErrorKindOf(err); classified && kind == orderErrorBadRequest {
			writeErr(w, http.StatusBadRequest, err.Error())
		} else if errors.Is(err, orderapp.ErrForbidden) {
			writeErr(w, http.StatusForbidden, "无权操作此订单")
		} else if errors.Is(err, orderapp.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "订单不存在")
		} else {
			writeErr(w, http.StatusInternalServerError, "更新失败")
		}
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// validOrderAmount 封装有效订单Amount业务协调。
func validOrderAmount(raw string) bool {
	// ok 用于本次流程后续判断的ok
	_, ok := normalizeOrderAmount(raw)
	return ok
}

// normalizeOrderAmount 封装normalize订单Amount业务协调。
func normalizeOrderAmount(raw string) (string, bool) {
	return orderapp.NormalizeOrderAmount(raw)
}

// manualShipOrders 封装manualShip订单列表业务协调。
func (s *Server) manualShipOrders(w http.ResponseWriter, r *http.Request) {
	// req 保存具名批量发货请求。
	var req manualShipRequestDTO
	if // err 用于本次流程后续判断的err
	err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(req.OrderIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "缺少订单ID")
		return
	}
	if req.ShipMode == "" {
		req.ShipMode = "status_only"
	}
	if req.ShipMode != "status_only" && req.ShipMode != "full_delivery" {
		writeErr(w, http.StatusBadRequest, "发货模式必须是 status_only 或 full_delivery")
		return
	}
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// result、err 用于本次流程后续判断的result、err
	result, err := s.orders().ManualShip(r.Context(), manualShipRequest{
		UserID: sess.UserID, OrderIDs: req.OrderIDs, ShipMode: req.ShipMode,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询账号失败")
		return
	}
	writeJSON(w, http.StatusOK, manualShipResponse{
		PartialFailure: result.FailedCount > 0,
		Message:        fmt.Sprintf("手动发货完成: 成功%d个, 失败%d个", result.SuccessCount, result.FailedCount),
		// Results 保留逐订单兼容字段，便于旧客户端展示失败原因。
		SuccessCount: result.SuccessCount, FailedCount: result.FailedCount, Results: result.Results,
	})
}

// requestOrderRedFlower 处理用户明确确认的单订单求花动作。
func (s *Server) requestOrderRedFlower(w http.ResponseWriter, r *http.Request) {
	// orderID 是路由中待求花的卖家订单标识。
	orderID := strings.TrimSpace(chi.URLParam(r, "order_id"))
	if orderID == "" {
		writeErr(w, http.StatusBadRequest, "缺少订单 ID")
		return
	}
	// session 是当前已认证用户会话。
	session := auth.SessionFromContext(r.Context())
	// result、requestErr 是应用层求花结果和错误。
	result, requestErr := s.orders().RequestRedFlower(r.Context(), session.UserID, orderID)
	if requestErr != nil {
		switch {
		case errors.Is(requestErr, orderapp.ErrNotFound):
			writeErr(w, http.StatusNotFound, "订单不存在")
		case errors.Is(requestErr, orderapp.ErrForbidden):
			writeErr(w, http.StatusForbidden, "无权操作此订单")
		case errors.Is(requestErr, orderapp.ErrRedFlowerAlreadyHandled), errors.Is(requestErr, orderapp.ErrRedFlowerExpired), errors.Is(requestErr, orderapp.ErrRedFlowerNotEligible), errors.Is(requestErr, orderapp.ErrRedFlowerNeedsReview):
			writeErr(w, http.StatusConflict, requestErr.Error())
		case errors.Is(requestErr, orderapp.ErrRedFlowerUnavailable):
			writeErr(w, http.StatusServiceUnavailable, requestErr.Error())
		default:
			writeErr(w, http.StatusBadGateway, "闲鱼求花请求失败: "+requestErr.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, redFlowerRequestResponse{Success: result.Success, Status: result.Status, Message: result.Message})
}

// getOrderRedFlowerStatus 返回订单持久化求花状态，不调用闲鱼接口。
func (s *Server) getOrderRedFlowerStatus(w http.ResponseWriter, r *http.Request) {
	// orderID 是路由中的卖家订单标识。
	orderID := strings.TrimSpace(chi.URLParam(r, "order_id"))
	if orderID == "" {
		writeErr(w, http.StatusBadRequest, "缺少订单 ID")
		return
	}
	// session 是当前已认证用户会话。
	session := auth.SessionFromContext(r.Context())
	// result、statusErr 是应用层持久求花状态和查询错误。
	result, statusErr := s.orders().RedFlowerStatus(r.Context(), session.UserID, orderID)
	if statusErr != nil {
		if errors.Is(statusErr, orderapp.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "订单不存在")
		} else if errors.Is(statusErr, orderapp.ErrForbidden) {
			writeErr(w, http.StatusForbidden, "无权操作此订单")
		} else {
			writeErr(w, http.StatusInternalServerError, "读取求花状态失败")
		}
		return
	}
	writeJSON(w, http.StatusOK, redFlowerRequestResponse{Success: true, Status: result.Status, Message: result.Message, RequestedAt: result.RequestedAt})
}

// importOrders 封装import订单列表业务协调。
func (s *Server) importOrders(w http.ResponseWriter, r *http.Request) {
	// orders、err 用于本次流程后续判断的orders、err
	orders, err := parseImportedOrders(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// result、err 用于本次流程后续判断的result、err
	result, err := s.orders().Import(r.Context(), sess.UserID, orders)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询账号失败")
		return
	}
	writeJSON(w, http.StatusOK, importOrdersResponse{
		PartialFailure: result.FailedCount > 0,
		Message:        fmt.Sprintf("导入完成: 成功%d个, 失败%d个", result.SuccessCount, result.FailedCount),
		// Total 和 Results 共同保留导入批次的统计及逐单结果。
		// 兼容客户端继续使用 partial_failure 判断批次是否需要复核。
		Total: result.Total, SuccessCount: result.SuccessCount, FailedCount: result.FailedCount, Results: result.Results,
	})
}

// atoiDefault 封装atoiDefault业务协调。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	// n、err 用于本次流程后续判断的n、err
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// isStableOrderStatus 封装isStable订单状态业务协调。
func isStableOrderStatus(status string) bool {
	switch status {
	case "shipped", "completed", "cancelled":
		return true
	default:
		return false
	}
}
