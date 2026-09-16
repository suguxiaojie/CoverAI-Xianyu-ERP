package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// chatTaskRouteCase 描述一个聊天或账号任务兼容路由测试样例。
type chatTaskRouteCase struct {
	// name 是测试样例名称。
	name string
	// method 是请求使用的 HTTP 方法。
	method string
	// versionedPath 是版本化入口路径。
	versionedPath string
	// legacyPath 是旧兼容入口路径。
	legacyPath string
	// body 是两条入口共用的请求体。
	body string
	// wantStatus 是两条入口应返回的状态码。
	wantStatus int
}

// requestChatTaskRoute 发送带认证会话的聊天或账号任务请求并返回状态码。
func requestChatTaskRoute(t *testing.T, handler http.Handler, sessionCookie *http.Cookie, method, path, body string) int {
	t.Helper()
	// request 是当前聊天或账号任务兼容入口请求。
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(sessionCookie)
	// recorder 是捕获兼容入口响应的记录器。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}

// TestVersionedChatTaskRoutesPreserveLegacyContracts 验证聊天和账号任务入口复用旧 handler 与权限边界。
func TestVersionedChatTaskRoutesPreserveLegacyContracts(t *testing.T) {
	// srv 是用于验证聊天与账号任务路由的 HTTP 测试服务。
	srv, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是当前测试使用的完整路由树。
	handler := srv.Router()
	// sessionCookie 是管理员登录后得到的认证会话。
	sessionCookie := loginHelper(t, handler)
	// cases 是覆盖版本化与旧路径状态码兼容性的测试样例集合。
	cases := []chatTaskRouteCase{
		{name: "chat-sessions-missing-account", method: http.MethodGet, versionedPath: "/api/v1/chat/sessions?account_id=missing", legacyPath: "/api/chat/sessions?account_id=missing", wantStatus: http.StatusForbidden},
		{name: "chat-messages-missing-chat", method: http.MethodGet, versionedPath: "/api/v1/chat/messages?account_id=acc1", legacyPath: "/api/chat/messages?account_id=acc1", wantStatus: http.StatusBadRequest},
		{name: "chat-message-service-disabled", method: http.MethodPost, versionedPath: "/api/v1/chat/messages", legacyPath: "/api/chat/messages", body: `{}`, wantStatus: http.StatusServiceUnavailable},
		{name: "chat-image-service-disabled", method: http.MethodPost, versionedPath: "/api/v1/chat/images", legacyPath: "/api/chat/images", wantStatus: http.StatusServiceUnavailable},
		{name: "chat-read-invalid", method: http.MethodPost, versionedPath: "/api/v1/chat/read", legacyPath: "/api/chat/read", body: `{}`, wantStatus: http.StatusBadRequest},
		{name: "chat-websocket-service-disabled", method: http.MethodGet, versionedPath: "/api/v1/chat/ws", legacyPath: "/api/chat/ws", wantStatus: http.StatusServiceUnavailable},
		{name: "account-task-settings-missing", method: http.MethodGet, versionedPath: "/api/v1/account-tasks/missing", legacyPath: "/api/account-tasks/missing", wantStatus: http.StatusForbidden},
		{name: "account-task-settings-invalid", method: http.MethodPut, versionedPath: "/api/v1/account-tasks/acc1", legacyPath: "/api/account-tasks/acc1", body: `{"auto_rate_enabled":true,"rate_content":"","auto_polish_enabled":false,"polish_time":"03:00"}`, wantStatus: http.StatusBadRequest},
		{name: "account-task-runs-missing", method: http.MethodGet, versionedPath: "/api/v1/account-tasks/missing/runs", legacyPath: "/api/account-tasks/missing/runs", wantStatus: http.StatusForbidden},
		{name: "account-task-run-service-disabled", method: http.MethodPost, versionedPath: "/api/v1/account-tasks/acc1/run", legacyPath: "/api/account-tasks/acc1/run", body: `{"task_type":"auto_rate"}`, wantStatus: http.StatusServiceUnavailable},
	}
	for _, routeCase := range cases { // routeCase 是当前正在执行的聊天或账号任务路由样例。
		// versionedStatus 是版本化入口实际返回的状态码。
		versionedStatus := requestChatTaskRoute(t, handler, sessionCookie, routeCase.method, routeCase.versionedPath, routeCase.body)
		// legacyStatus 是旧兼容入口实际返回的状态码。
		legacyStatus := requestChatTaskRoute(t, handler, sessionCookie, routeCase.method, routeCase.legacyPath, routeCase.body)
		if versionedStatus != routeCase.wantStatus || legacyStatus != routeCase.wantStatus {
			t.Errorf("%s status versioned=%d legacy=%d want=%d", routeCase.name, versionedStatus, legacyStatus, routeCase.wantStatus)
		}
	}
}

// TestVersionedChatRecallRouteRejectsInvalidRequest 验证新撤回入口存在且使用具名请求校验。
func TestVersionedChatRecallRouteRejectsInvalidRequest(t *testing.T) {
	// srv、handler、sessionCookie 保存带认证会话的测试服务环境。
	srv, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是包含版本化撤回路由的完整测试路由树。
	handler := srv.Router()
	// sessionCookie 是管理员认证会话。
	sessionCookie := loginHelper(t, handler)
	// status 是缺少账号标识的撤回请求状态码。
	status := requestChatTaskRoute(t, handler, sessionCookie, http.MethodPost, "/api/v1/chat/messages/local-1/recall", `{}`)
	if status != http.StatusBadRequest {
		t.Fatalf("recall invalid status=%d want=%d", status, http.StatusBadRequest)
	}
}

// TestVersionedChatReplyRouteExistsAndRequiresSendingService 验证原生引用使用独立版本化路由，旧普通消息路由不会静默降级它。
func TestVersionedChatReplyRouteExistsAndRequiresSendingService(t *testing.T) {
	// srv、cleanup 是未装配聊天发送端口的测试服务和关闭函数。
	srv, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是包含版本化引用路由的完整 HTTP 路由树。
	handler := srv.Router()
	// sessionCookie 是已登录管理员的测试会话。
	sessionCookie := loginHelper(t, handler)
	// status 是引用专用路由在发送服务未装配时的明确状态码。
	status := requestChatTaskRoute(t, handler, sessionCookie, http.MethodPost, "/api/v1/chat/replies", `{"account_id":"acc1","chat_id":"chat-1","buyer_id":"buyer-1","text":"回复","reply_to_message_key":"target"}`)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("reply unavailable status=%d want=%d", status, http.StatusServiceUnavailable)
	}
}

// TestVersionedChatLocationRouteExistsAndRequiresSendingService 验证位置卡片仅提供版本化入口且复用聊天发送能力门禁。
func TestVersionedChatLocationRouteExistsAndRequiresSendingService(t *testing.T) {
	// srv、cleanup 是未装配聊天发送端口的测试服务和关闭函数。
	srv, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是包含版本化位置卡片路由的完整 HTTP 路由树。
	handler := srv.Router()
	// sessionCookie 是已登录管理员的测试会话。
	sessionCookie := loginHelper(t, handler)
	// status 是位置卡片入口在发送服务未装配时的明确状态码。
	status := requestChatTaskRoute(t, handler, sessionCookie, http.MethodPost, "/api/v1/chat/location-cards", `{"account_id":"acc1","chat_id":"chat-1","buyer_id":"buyer-1","title":"实体店","description":"地址","latitude":22.5,"longitude":113.9}`)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("location unavailable status=%d want=%d", status, http.StatusServiceUnavailable)
	}
}
