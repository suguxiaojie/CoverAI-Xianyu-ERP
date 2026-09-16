package orders

import (
	"context"
	"errors"
	"testing"
)

// conversationContextRepositoryFake 是历史订单用例的最小账号归属和列表替身。
type conversationContextRepositoryFake struct {
	// owned 表示测试账号是否归属当前用户。
	owned bool
	// rows 是预置的会话和买家历史订单。
	rows []OrderRow
	// total 是数据库匹配的完整订单数。
	total int
	// receivedFilter 保存应用服务下发的精确关联条件。
	receivedFilter ListFilter
	// err 是归属或列表查询返回的预置错误。
	err error
}

// ExistsOwned 返回预置归属结果。
func (fake *conversationContextRepositoryFake) ExistsOwned(context.Context, int64, string) (bool, error) {
	return fake.owned, fake.err
}

// ListOrdersForUser 记录筛选并返回预置订单。
func (fake *conversationContextRepositoryFake) ListOrdersForUser(_ context.Context, filter ListFilter) ([]OrderRow, int, error) {
	fake.receivedFilter = filter
	return fake.rows, fake.total, fake.err
}

// TestConversationContextAssociatesFiltersAndPaginates 验证本会话优先、同买家回退、摘要和状态分页。
func TestConversationContextAssociatesFiltersAndPaginates(t *testing.T) {
	// repository 是包含一笔本会话完成单和两笔买家历史单的测试仓储。
	repository := &conversationContextRepositoryFake{owned: true, total: 3, rows: []OrderRow{
		{OrderID: "current", ChatID: "chat-1", BuyerID: "buyer-1", OrderStatus: "completed"},
		{OrderID: "history-shipped", ChatID: "chat-old", BuyerID: "buyer-1", OrderStatus: "3"},
		{OrderID: "history-refund", BuyerID: "buyer-1", OrderStatus: "refunded"},
	}}
	// service 是待验证的历史订单应用服务。
	service := NewConversationContextService(repository)
	// result、queryErr 是已发货筛选后的分页结果和错误。
	result, queryErr := service.Context(context.Background(), ConversationContextQuery{
		UserID: 1, AccountID: "account-1", ChatID: "chat-1", BuyerID: "buyer-1", Status: "shipped", Page: 1, PageSize: 20,
	})
	if queryErr != nil {
		t.Fatalf("查询历史订单失败: %v", queryErr)
	}
	if result.Summary.Total != 3 || result.Summary.CurrentChat != 1 || result.Summary.Completed != 1 {
		t.Fatalf("历史摘要异常: %+v", result.Summary)
	}
	if result.Total != 1 || len(result.Orders) != 1 || result.Orders[0].Row.OrderID != "history-shipped" || result.Orders[0].Association != ConversationOrderSameBuyer {
		t.Fatalf("状态筛选或关联异常: %+v", result)
	}
	if !repository.receivedFilter.MatchConversationContext || repository.receivedFilter.ContextChatID != "chat-1" || repository.receivedFilter.ContextBuyerID != "buyer-1" || repository.receivedFilter.PrioritizeChatID != "chat-1" {
		t.Fatalf("精确关联筛选未下发: %+v", repository.receivedFilter)
	}
	// allResult、allErr 是不带状态筛选时用于验证完成优先和退款末尾的结果。
	allResult, allErr := service.Context(context.Background(), ConversationContextQuery{UserID: 1, AccountID: "account-1", ChatID: "chat-1", BuyerID: "buyer-1", Page: 1, PageSize: 20})
	if allErr != nil {
		t.Fatalf("查询全部历史订单失败: %v", allErr)
	}
	if len(allResult.Orders) != 3 || allResult.Orders[0].Row.OrderID != "current" || allResult.Orders[1].Row.OrderID != "history-shipped" || allResult.Orders[2].Row.OrderID != "history-refund" {
		t.Fatalf("历史订单状态排序异常: %+v", allResult.Orders)
	}
}

// TestConversationContextRejectsUnsafeOrForbiddenScope 验证缺少稳定关系、未知状态和越权账号均被拒绝。
func TestConversationContextRejectsUnsafeOrForbiddenScope(t *testing.T) {
	// service 是默认归属成功的历史订单服务。
	service := NewConversationContextService(&conversationContextRepositoryFake{owned: true})
	// invalidQueries 是不能形成安全关联或包含未知状态的请求集合。
	invalidQueries := []ConversationContextQuery{
		{UserID: 1, AccountID: "account-1"},
		{UserID: 1, AccountID: "account-1", BuyerID: "buyer-1", Status: "unknown-status"},
	}
	// query 是当前验证的非法上下文。
	for _, query := range invalidQueries {
		// queryErr 是当前非法上下文返回的应用错误。
		if _, queryErr := service.Context(context.Background(), query); !errors.Is(queryErr, ErrInvalidConversationContext) {
			t.Fatalf("非法上下文未拒绝 query=%+v err=%v", query, queryErr)
		}
	}
	// forbiddenService 是账号归属失败的历史订单服务。
	forbiddenService := NewConversationContextService(&conversationContextRepositoryFake{owned: false})
	// queryErr 是越权账号上下文返回的应用错误。
	if _, queryErr := forbiddenService.Context(context.Background(), ConversationContextQuery{UserID: 1, AccountID: "other", BuyerID: "buyer-1"}); !errors.Is(queryErr, ErrForbidden) {
		t.Fatalf("越权账号未拒绝: %v", queryErr)
	}
}
