package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestConversationOrderContextScopesAssociatesAndFilters 验证版本化接口只返回当前账号的本会话和同买家历史订单。
func TestConversationOrderContextScopesAssociatesAndFilters(t *testing.T) {
	// server、store、cleanup 是真实 SQLite 测试服务、仓储和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是测试数据写入使用的请求上下文。
	ctx := context.Background()
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO item_info (cookie_id,item_id,item_title,item_detail) VALUES
		('acc1','item-current','当前会话商品','{"pic_info":{"picUrl":"https://img.example/current.png"}}')`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO chat_sessions (cookie_id,chat_id,buyer_id,buyer_name,item_id,item_title,updated_at) VALUES
		('acc1','chat-1','buyer-1','历史买家','item-current','当前会话商品',200),
		('acc1','chat-old','buyer-1','历史买家','item-history','聊天历史商品',100)`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,item_id,buyer_id,quantity,amount,order_status,cookie_id,chat_id,created_at) VALUES
		('order-current','item-current','buyer-1','1','100.00','completed','acc1','chat-1','2026-08-20T10:00:00Z'),
		('order-history','item-history','buyer-1','1','50.00','shipped','acc1','chat-old','2026-08-19T10:00:00Z'),
		('order-unrelated','item-other','buyer-2','1','30.00','completed','acc1','chat-other','2026-08-18T10:00:00Z')`)
	// handler 是包含版本化路由和认证中间件的测试 HTTP 入口。
	handler := server.Router()
	// sessionCookie 是当前测试用户的登录会话 Cookie。
	sessionCookie := loginHelper(t, handler)
	// request 是读取全部会话订单的认证请求。
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/conversation-context?account_id=acc1&chat_id=chat-1&buyer_id=buyer-1", nil)
	request.AddCookie(sessionCookie)
	// recorder 捕获历史订单接口响应。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("历史订单接口 status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	// response 是历史订单具名响应 DTO。
	var response conversationOrderContextResponse
	// decodeErr 是全部历史订单响应的 JSON 解码错误。
	if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &response); decodeErr != nil {
		t.Fatalf("解析历史订单响应失败: %v", decodeErr)
	}
	if response.Summary.Total != 2 || response.Summary.CurrentChat != 1 || response.Summary.Completed != 1 || response.Total != 2 || len(response.Orders) != 2 {
		t.Fatalf("历史订单摘要异常: %+v", response)
	}
	if response.Orders[0].OrderID != "order-current" || response.Orders[0].Association != "current_chat" || response.Orders[0].ItemTitle != "当前会话商品" {
		t.Fatalf("本会话订单未置顶或字段异常: %+v", response.Orders[0])
	}
	if response.Orders[1].OrderID != "order-history" || response.Orders[1].Association != "same_buyer" || response.Orders[1].ItemTitle != "聊天历史商品" {
		t.Fatalf("同买家历史订单异常: %+v", response.Orders[1])
	}
	if strings.Contains(recorder.Body.String(), "receiver_phone") || strings.Contains(recorder.Body.String(), "receiver_address") || strings.Contains(recorder.Body.String(), "buyer_id") {
		t.Fatalf("历史订单响应泄露非必要隐私字段: %s", recorder.Body.String())
	}
	// filteredRequest 是只读取已发货历史订单的请求。
	filteredRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/conversation-context?account_id=acc1&chat_id=chat-1&buyer_id=buyer-1&status=shipped", nil)
	filteredRequest.AddCookie(sessionCookie)
	// filteredRecorder 捕获状态筛选后的响应。
	filteredRecorder := httptest.NewRecorder()
	handler.ServeHTTP(filteredRecorder, filteredRequest)
	// filteredResponse 是状态筛选后的历史订单响应。
	var filteredResponse conversationOrderContextResponse
	if filteredRecorder.Code != http.StatusOK {
		t.Fatalf("筛选接口 status=%d body=%s", filteredRecorder.Code, filteredRecorder.Body.String())
	}
	// decodeErr 是状态筛选响应的 JSON 解码错误。
	if decodeErr := json.Unmarshal(filteredRecorder.Body.Bytes(), &filteredResponse); decodeErr != nil {
		t.Fatalf("解析筛选响应失败: %v", decodeErr)
	}
	if filteredResponse.Total != 1 || len(filteredResponse.Orders) != 1 || filteredResponse.Orders[0].OrderID != "order-history" || filteredResponse.Summary.Total != 2 {
		t.Fatalf("状态筛选未保持全量摘要: %+v", filteredResponse)
	}
}

// TestConversationOrderContextRejectsMissingOrForeignScope 验证缺少稳定关系和非归属账号不会泄露订单。
func TestConversationOrderContextRejectsMissingOrForeignScope(t *testing.T) {
	// server、_、cleanup 是测试服务、未使用仓储和释放函数。
	server, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是认证后的版本化路由入口。
	handler := server.Router()
	// sessionCookie 是测试用户登录会话。
	sessionCookie := loginHelper(t, handler)
	// cases 是非法参数和越权账号对应的期望状态。
	cases := []struct {
		// path 是待请求的历史订单地址。
		path string
		// status 是期望 HTTP 状态。
		status int
	}{
		{path: "/api/v1/orders/conversation-context?account_id=acc1", status: http.StatusBadRequest},
		{path: "/api/v1/orders/conversation-context?account_id=foreign&buyer_id=buyer-1", status: http.StatusForbidden},
	}
	// item 是当前执行的拒绝场景。
	for _, item := range cases {
		// request 是当前非法或越权请求。
		request := httptest.NewRequest(http.MethodGet, item.path, nil)
		request.AddCookie(sessionCookie)
		// recorder 捕获拒绝响应。
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != item.status {
			t.Fatalf("path=%s status=%d body=%s", item.path, recorder.Code, recorder.Body.String())
		}
	}
}
