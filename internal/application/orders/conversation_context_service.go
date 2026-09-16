package orders

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// conversationContextLoadLimit 限制单个账号买家的最近历史订单读取量，避免异常买家造成无界查询。
const conversationContextLoadLimit = 500

var (
	// ErrInvalidConversationContext 表示账号、会话、买家或状态参数不足以形成安全的精确关联。
	ErrInvalidConversationContext = errors.New("会话订单上下文无效")
)

// ConversationOrderAssociation 标识历史订单与当前聊天的关联依据。
type ConversationOrderAssociation string

const (
	// ConversationOrderCurrentChat 表示订单的账号和 chat_id 都与当前会话一致。
	ConversationOrderCurrentChat ConversationOrderAssociation = "current_chat"
	// ConversationOrderSameBuyer 表示订单仅通过同账号下稳定 buyer_id 关联。
	ConversationOrderSameBuyer ConversationOrderAssociation = "same_buyer"
)

// ConversationContextQuery 是 Chat 历史订单只读用例的精确上下文和分页条件。
type ConversationContextQuery struct {
	// UserID 是当前登录用户标识。
	UserID int64
	// AccountID 是当前 Chat 页签选择的闲鱼账号。
	AccountID string
	// ChatID 是当前会话标识；非空时用于置顶并标记本会话订单。
	ChatID string
	// BuyerID 是当前会话的稳定买家标识；非空时补充同账号历史订单。
	BuyerID string
	// Status 是可选的历史订单状态筛选。
	Status string
	// Page 是从一开始的页码。
	Page int
	// PageSize 是单页订单卡数量。
	PageSize int
}

// ConversationOrder 是单张历史订单卡的业务模型和关联依据。
type ConversationOrder struct {
	// Row 是复用订单只读模型的非敏感展示字段。
	Row OrderRow
	// Association 是本会话或同买家历史关系。
	Association ConversationOrderAssociation
}

// ConversationOrderSummary 是右侧栏顶部的全部历史订单摘要。
type ConversationOrderSummary struct {
	// Total 是匹配当前账号会话上下文的全部订单数量。
	Total int
	// CurrentChat 是 chat_id 精确匹配当前会话的订单数量。
	CurrentChat int
	// Completed 是当前归一化状态为 completed 的订单数量。
	Completed int
}

// ConversationContextResult 是历史订单卡、摘要和分页元数据。
type ConversationContextResult struct {
	// Summary 是不受当前状态筛选影响的全部历史摘要。
	Summary ConversationOrderSummary
	// Orders 是当前状态和页码下的订单卡。
	Orders []ConversationOrder
	// Total 是当前状态筛选后的订单数量。
	Total int
	// Page 是规范化后的当前页码。
	Page int
	// PageSize 是规范化后的每页数量。
	PageSize int
	// TotalPages 是当前筛选结果总页数。
	TotalPages int
	// Truncated 表示该买家超过本用例五百单保护上限，摘要或筛选可能只覆盖最近记录。
	Truncated bool
}

// ConversationContextRepository 是 Chat 历史订单只读用例需要的最小持久化能力。
type ConversationContextRepository interface {
	// ExistsOwned 判断目标账号是否属于当前用户。
	ExistsOwned(ctx context.Context, userID int64, cookieID string) (bool, error)
	// ListOrdersForUser 按精确会话上下文读取订单展示行。
	ListOrdersForUser(ctx context.Context, filter ListFilter) ([]OrderRow, int, error)
}

// ConversationContextService 负责 Chat 历史订单的所有权、精确关联、状态筛选和分页。
type ConversationContextService struct {
	// repository 提供账号归属和订单只读列表能力。
	repository ConversationContextRepository
}

// NewConversationContextService 创建会话历史订单只读服务。
func NewConversationContextService(repository ConversationContextRepository) *ConversationContextService {
	return &ConversationContextService{repository: repository}
}

// Context 查询当前账号同会话及同买家历史订单，不使用昵称、手机号或商品进行模糊关联。
func (service *ConversationContextService) Context(ctx context.Context, query ConversationContextQuery) (ConversationContextResult, error) {
	if service == nil || service.repository == nil {
		return ConversationContextResult{}, errors.New("会话订单 repository 未初始化")
	}
	query.AccountID = strings.TrimSpace(query.AccountID)
	query.ChatID = strings.TrimSpace(query.ChatID)
	query.BuyerID = strings.TrimSpace(query.BuyerID)
	query.Status = strings.TrimSpace(query.Status)
	if query.UserID == 0 || query.AccountID == "" || (query.ChatID == "" && query.BuyerID == "") || !validConversationOrderStatus(query.Status) {
		return ConversationContextResult{}, ErrInvalidConversationContext
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 50 {
		query.PageSize = 50
	}
	// owned、ownershipErr 是当前账号归属校验结果和数据库错误。
	owned, ownershipErr := service.repository.ExistsOwned(ctx, query.UserID, query.AccountID)
	if ownershipErr != nil {
		return ConversationContextResult{}, ownershipErr
	}
	if !owned {
		return ConversationContextResult{}, ErrForbidden
	}
	// rows、matchedTotal、listErr 是按精确会话或买家关系读取的最近订单及完整匹配数。
	rows, matchedTotal, listErr := service.repository.ListOrdersForUser(ctx, ListFilter{
		UserID: query.UserID, CookieID: query.AccountID,
		ContextBuyerID: query.BuyerID, ContextChatID: query.ChatID,
		MatchConversationContext: true, PrioritizeChatID: query.ChatID,
		Limit: conversationContextLoadLimit, Offset: 0,
	})
	if listErr != nil {
		return ConversationContextResult{}, listErr
	}
	// index 是当前规范化订单状态的下标；排序和筛选必须使用统一业务状态。
	for index := range rows {
		rows[index].OrderStatus = NormalizeOrderStatus(rows[index].OrderStatus)
	}
	// 稳定排序先按业务状态优先级，再在同状态内保留本会话优先和数据库时间倒序。
	sort.SliceStable(rows, func(leftIndex, rightIndex int) bool {
		// left、right 是当前比较的两笔历史订单。
		left, right := rows[leftIndex], rows[rightIndex]
		// leftRank、rightRank 是已完成优先、退款取消末尾的业务排序级别。
		leftRank, rightRank := conversationOrderStatusRank(left.OrderStatus), conversationOrderStatusRank(right.OrderStatus)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		// leftCurrent、rightCurrent 表示订单是否精确属于当前 chat_id。
		leftCurrent, rightCurrent := query.ChatID != "" && left.ChatID == query.ChatID, query.ChatID != "" && right.ChatID == query.ChatID
		return leftCurrent && !rightCurrent
	})
	// summary 保存不受状态筛选影响的全部上下文统计；超过保护上限时 Total 仍使用数据库总数。
	summary := ConversationOrderSummary{Total: matchedTotal}
	// filtered 保存满足当前状态筛选的订单和关联类型。
	filtered := make([]ConversationOrder, 0, len(rows))
	// row 是当前归一化、统计并筛选的订单展示行。
	for _, row := range rows {
		// association 是当前订单相对所选聊天的精确关联依据。
		association := ConversationOrderSameBuyer
		if query.ChatID != "" && row.ChatID == query.ChatID {
			association = ConversationOrderCurrentChat
			summary.CurrentChat++
		}
		if row.OrderStatus == "completed" {
			summary.Completed++
		}
		if conversationOrderStatusMatches(row.OrderStatus, query.Status) {
			filtered = append(filtered, ConversationOrder{Row: row, Association: association})
		}
	}
	// offset 是当前页在过滤结果中的起始下标。
	offset := (query.Page - 1) * query.PageSize
	if offset > len(filtered) {
		offset = len(filtered)
	}
	// end 是当前页在过滤结果中的排除结束下标。
	end := offset + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	// pageOrders 是独立切片，避免调用方持有完整买家历史底层数组。
	pageOrders := append([]ConversationOrder(nil), filtered[offset:end]...)
	return ConversationContextResult{
		Summary: summary, Orders: pageOrders, Total: len(filtered), Page: query.Page, PageSize: query.PageSize,
		TotalPages: (len(filtered) + query.PageSize - 1) / query.PageSize,
		Truncated:  matchedTotal > len(rows),
	}, nil
}

// conversationOrderStatusRank 返回历史订单业务排序级别：完成优先，退款与取消末尾。
func conversationOrderStatusRank(status string) int {
	switch NormalizeOrderStatus(status) {
	case "completed":
		return 0
	case "processing", "pending_ship", "shipped", "received":
		return 1
	case "refunding", "unknown":
		return 2
	case "refunded", "cancelled":
		return 3
	default:
		return 2
	}
}

// validConversationOrderStatus 判断 HTTP 可选筛选是否属于第一版布局公开集合。
func validConversationOrderStatus(status string) bool {
	switch status {
	case "", "all", "pending_payment", "pending_ship", "shipped", "received", "completed", "refund", "cancelled":
		return true
	default:
		return false
	}
}

// conversationOrderStatusMatches 判断归一化订单状态是否属于当前 Chat 筛选。
func conversationOrderStatusMatches(normalized, filter string) bool {
	switch filter {
	case "", "all":
		return true
	case "pending_payment":
		return normalized == "processing"
	case "refund":
		return normalized == "refunding" || normalized == "refunded"
	default:
		return normalized == filter
	}
}
