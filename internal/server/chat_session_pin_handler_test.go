package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xianyu-go/internal/db"
)

// TestChatSessionPinEndpointPersistsAndReturnsPinnedSessionFirst 验证版本化接口持久化置顶、保持会话归属并让列表立即按置顶排序。
func TestChatSessionPinEndpointPersistsAndReturnsPinnedSessionFirst(t *testing.T) {
	// server、store、cleanup 是使用真实 SQLite 应用端口的隔离 HTTP 服务、仓储和释放函数。
	server, store, cleanup := newTestServerWithChat(t)
	defer cleanup()
	// ctx 是预置会话数据使用的调用上下文。
	ctx := context.Background()
	// olderSession 是需要从最近消息之后提到置顶组的较早会话。
	olderSession := db.ChatSession{CookieID: "acc1", ChatID: "pin-chat", BuyerID: "buyer-pin", BuyerName: "置顶买家", LastMessage: "较早消息", LastMessageAt: 100}
	// newerSession 是未置顶时应排在第一的最新会话。
	newerSession := db.ChatSession{CookieID: "acc1", ChatID: "new-chat", BuyerID: "buyer-new", BuyerName: "最新买家", LastMessage: "最新消息", LastMessageAt: 200}
	if // olderErr 是创建较早会话的错误。
	olderErr := store.Chats.UpsertSession(ctx, olderSession); olderErr != nil {
		t.Fatal(olderErr)
	}
	if // newerErr 是创建最新会话的错误。
	newerErr := store.Chats.UpsertSession(ctx, newerSession); newerErr != nil {
		t.Fatal(newerErr)
	}
	// handler 是包含版本化聊天路由和认证中间件的测试入口。
	handler := server.Router()
	// sessionCookie 是归属 acc1 的测试 ERP 登录会话。
	sessionCookie := loginHelper(t, handler)
	// pinRequest 是将较早会话置顶的认证 PUT 请求。
	pinRequest := httptest.NewRequest(http.MethodPut, "/api/v1/chat/sessions/pin-chat/pin", strings.NewReader(`{"account_id":"acc1","pinned":true}`))
	pinRequest.AddCookie(sessionCookie)
	// pinRecorder 捕获置顶接口返回的具名结果。
	pinRecorder := httptest.NewRecorder()
	handler.ServeHTTP(pinRecorder, pinRequest)
	if pinRecorder.Code != http.StatusOK || !strings.Contains(pinRecorder.Body.String(), `"pinned":true`) {
		t.Fatalf("pin status=%d body=%s", pinRecorder.Code, pinRecorder.Body.String())
	}
	// listRequest 是置顶后读取当前账号会话列表的认证请求。
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions?account_id=acc1", nil)
	listRequest.AddCookie(sessionCookie)
	// listRecorder 捕获应按置顶优先返回的会话列表。
	listRecorder := httptest.NewRecorder()
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	// pinnedIndex 是置顶会话在 JSON 响应中的位置。
	pinnedIndex := strings.Index(listRecorder.Body.String(), `"chat_id":"pin-chat"`)
	// newerIndex 是原最新普通会话在 JSON 响应中的位置。
	newerIndex := strings.Index(listRecorder.Body.String(), `"chat_id":"new-chat"`)
	if pinnedIndex < 0 || newerIndex < 0 || pinnedIndex >= newerIndex || !strings.Contains(listRecorder.Body.String(), `"is_pinned":true`) {
		t.Fatalf("pinned session not first: %s", listRecorder.Body.String())
	}
	// unpinRequest 是重复使用同一幂等端点取消置顶的请求。
	unpinRequest := httptest.NewRequest(http.MethodPut, "/api/v1/chat/sessions/pin-chat/pin", strings.NewReader(`{"account_id":"acc1","pinned":false}`))
	unpinRequest.AddCookie(sessionCookie)
	// unpinRecorder 捕获取消置顶后的成功结果。
	unpinRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unpinRecorder, unpinRequest)
	if unpinRecorder.Code != http.StatusOK || !strings.Contains(unpinRecorder.Body.String(), `"pinned":false`) {
		t.Fatalf("unpin status=%d body=%s", unpinRecorder.Code, unpinRecorder.Body.String())
	}
}

// TestChatSessionPinEndpointRejectsInvalidForeignAndMissingSessions 验证缺失布尔值、越权账号和不存在会话不会被伪装为成功。
func TestChatSessionPinEndpointRejectsInvalidForeignAndMissingSessions(t *testing.T) {
	// server、_、cleanup 是测试 HTTP 服务、未使用仓储和释放函数。
	server, _, cleanup := newTestServerWithChat(t)
	defer cleanup()
	// handler 是应用归属和参数门禁的 HTTP 入口。
	handler := server.Router()
	// sessionCookie 是当前测试用户的 ERP 登录会话。
	sessionCookie := loginHelper(t, handler)
	// cases 是待验证的非法请求体、路径和期望 HTTP 状态。
	cases := []struct {
		// path 是当前请求的置顶路由。
		path string
		// body 是当前请求的 JSON 文本。
		body string
		// status 是应用和传输边界期望的 HTTP 状态。
		status int
	}{
		{path: "/api/v1/chat/sessions/chat-1/pin", body: `{"account_id":"acc1"}`, status: http.StatusBadRequest},
		{path: "/api/v1/chat/sessions/chat-1/pin", body: `{"account_id":"foreign","pinned":true}`, status: http.StatusForbidden},
		{path: "/api/v1/chat/sessions/missing/pin", body: `{"account_id":"acc1","pinned":true}`, status: http.StatusNotFound},
	}
	// item 是当前执行的拒绝场景。
	for _, item := range cases {
		// request 是当前携带认证 Cookie 的非法置顶请求。
		request := httptest.NewRequest(http.MethodPut, item.path, strings.NewReader(item.body))
		request.AddCookie(sessionCookie)
		// recorder 捕获当前拒绝响应。
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != item.status {
			t.Fatalf("path=%s status=%d want=%d body=%s", item.path, recorder.Code, item.status, recorder.Body.String())
		}
	}
}
