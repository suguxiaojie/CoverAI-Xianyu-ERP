package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/auth"
)

// conversationOrderDTO 是 Chat 历史订单卡的具名响应和关联依据。
type conversationOrderDTO struct {
	// OrderID 是平台订单标识。
	OrderID string `json:"order_id"`
	// ItemID 是关联商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是当前或历史本地商品名称。
	ItemTitle string `json:"item_title"`
	// ItemImage 是本地商品详情解析出的主图地址。
	ItemImage string `json:"item_image"`
	// Quantity 是订单数量文本。
	Quantity string `json:"quantity"`
	// Amount 是订单实付金额文本。
	Amount string `json:"amount"`
	// Status 是归一化后的订单状态。
	Status string `json:"status"`
	// CreatedAt 是订单创建时间。
	CreatedAt string `json:"created_at"`
	// Association 是 current_chat 或 same_buyer。
	Association string `json:"association"`
}

// conversationOrderSummaryDTO 是不受状态筛选影响的买家历史订单摘要。
type conversationOrderSummaryDTO struct {
	// Total 是全部精确上下文订单数。
	Total int `json:"total"`
	// CurrentChat 是明确属于当前 chat_id 的订单数。
	CurrentChat int `json:"current_chat"`
	// Completed 是已完成订单数。
	Completed int `json:"completed"`
}

// conversationOrderContextResponse 是 Chat 历史订单分页响应。
type conversationOrderContextResponse struct {
	// Summary 是全部状态的历史订单摘要。
	Summary conversationOrderSummaryDTO `json:"summary"`
	// Orders 是当前状态和页码下的订单卡。
	Orders []conversationOrderDTO `json:"orders"`
	// Total 是当前状态筛选后的订单数。
	Total int `json:"total"`
	// Page 是当前页码。
	Page int `json:"page"`
	// PageSize 是当前页大小。
	PageSize int `json:"page_size"`
	// TotalPages 是筛选结果总页数。
	TotalPages int `json:"total_pages"`
	// Truncated 表示上下文订单超过五百条读取保护上限。
	Truncated bool `json:"truncated"`
}

// conversationOrderContext 读取当前账号、会话或买家精确关联的历史订单，不触发平台同步。
func (s *Server) conversationOrderContext(w http.ResponseWriter, r *http.Request) {
	// session 是已经通过中间件认证的当前用户会话。
	session := auth.SessionFromContext(r.Context())
	// queryValues 是历史订单上下文的 URL 参数集合。
	queryValues := r.URL.Query()
	// accountID 是去除空白后的当前聊天账号标识。
	accountID := strings.TrimSpace(queryValues.Get("account_id"))
	// chatID 是去除空白后的当前聊天会话标识。
	chatID := strings.TrimSpace(queryValues.Get("chat_id"))
	// buyerID 是去除空白后的稳定买家标识。
	buyerID := strings.TrimSpace(queryValues.Get("buyer_id"))
	// status 是去除空白后的订单状态筛选。
	status := strings.TrimSpace(queryValues.Get("status"))
	if accountID == "" || (chatID == "" && buyerID == "") {
		writeErr(w, http.StatusBadRequest, "缺少账号或会话买家上下文")
		return
	}
	// result、contextErr 是应用服务返回的精确关联结果和错误。
	result, contextErr := s.orders().ConversationContext(r.Context(), orderapp.ConversationContextQuery{
		UserID: session.UserID, AccountID: accountID, ChatID: chatID, BuyerID: buyerID, Status: status,
		Page: atoiDefault(queryValues.Get("page"), 1), PageSize: atoiDefault(queryValues.Get("page_size"), 20),
	})
	if errors.Is(contextErr, orderapp.ErrForbidden) {
		writeErr(w, http.StatusForbidden, "无权限读取该账号历史订单")
		return
	}
	if errors.Is(contextErr, orderapp.ErrInvalidConversationContext) {
		writeErr(w, http.StatusBadRequest, "历史订单筛选条件无效")
		return
	}
	if contextErr != nil {
		writeErr(w, http.StatusInternalServerError, "查询历史订单失败")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ConversationContext 把 HTTP 内部订单适配器调用转发到应用 Port，并转换为具名响应模型。
func (a *orderHTTPAdapter) ConversationContext(ctx context.Context, query orderapp.ConversationContextQuery) (conversationOrderContextResponse, error) {
	// result、contextErr 是应用层历史订单结果和错误。
	result, contextErr := a.services.ConversationContext(ctx, query)
	if contextErr != nil {
		return conversationOrderContextResponse{}, contextErr
	}
	return conversationOrderContextResponseFromApplication(result), nil
}

// conversationOrderContextResponseFromApplication 将应用历史订单分页映射为 HTTP DTO。
func conversationOrderContextResponseFromApplication(result orderapp.ConversationContextResult) conversationOrderContextResponse {
	// orders 是当前页应用订单到传输 DTO 的转换结果。
	orders := make([]conversationOrderDTO, 0, len(result.Orders))
	// item 是当前转换的历史订单和关联依据。
	for _, item := range result.Orders {
		// normalizedStatus 是当前订单的稳定业务状态。
		normalizedStatus := orderapp.NormalizeOrderStatus(item.Row.OrderStatus)
		orders = append(orders, conversationOrderDTO{
			OrderID: item.Row.OrderID, ItemID: item.Row.ItemID, ItemTitle: item.Row.ItemTitle,
			ItemImage: itemImageFromDetail(item.Row.ItemDetail), Quantity: item.Row.Quantity,
			Amount: item.Row.Amount, Status: normalizedStatus, CreatedAt: item.Row.CreatedAt,
			Association: string(item.Association),
		})
	}
	return conversationOrderContextResponse{
		Summary: conversationOrderSummaryDTO{Total: result.Summary.Total, CurrentChat: result.Summary.CurrentChat, Completed: result.Summary.Completed},
		Orders:  orders, Total: result.Total, Page: result.Page, PageSize: result.PageSize, TotalPages: result.TotalPages, Truncated: result.Truncated,
	}
}
