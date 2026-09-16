package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xianyu-go/internal/xianyu/mtop"
)

// TestChatUserCreditEndpointReturnsCurrentBuyerRolesAndEnforcesOwnership 验证信用接口只查询单个已归属账号买家。
func TestChatUserCreditEndpointReturnsCurrentBuyerRolesAndEnforcesOwnership(t *testing.T) {
	// server、cleanup 是启用 Chat 应用和固定账号数据的测试服务及清理函数。
	server, _, cleanup := newTestServerWithChat(t)
	defer cleanup()
	// client 使用本地传输返回结构化买家／卖家信用，不访问真实闲鱼。
	client := &mtop.ClientImpl{UserCreditURL: "https://example.test/user-credit", HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("api") != "mtop.idle.web.user.page.head" {
			t.Fatalf("api=%q", request.URL.Query().Get("api"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: request, Body: io.NopCloser(strings.NewReader(
			`{"ret":["SUCCESS::调用成功"],"data":{"module":{"base":{"ylzTags":[{"attributes":{"role":"buyer","level":5},"code":"cs_buyer_level","text":"买家信用极好","type":"ylzLevel"},{"attributes":{"role":"seller","level":4},"code":"cs_seller_level","text":"卖家信用优秀","type":"ylzLevel"}]}}}}`,
		))}, nil
	})}}
	setTestMTop(server, client)
	// handler 是包含版本化信用路由的认证 HTTP 处理器。
	handler := server.Router()
	// sessionCookie 是管理员登录后访问自有账号所需的测试会话。
	sessionCookie := loginHelper(t, handler)
	// request 是当前会话买家信用的只读请求。
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chat/user-credit?account_id=acc1&buyer_id=2272060441", nil)
	request.AddCookie(sessionCookie)
	// recorder 保存成功响应。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"text":"买家信用极好"`) || !strings.Contains(recorder.Body.String(), `"level":4`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	// foreignRequest 使用不属于当前 ERP 用户的账号，必须在平台调用前拒绝。
	foreignRequest := httptest.NewRequest(http.MethodGet, "/api/v1/chat/user-credit?account_id=missing&buyer_id=2272060441", nil)
	foreignRequest.AddCookie(sessionCookie)
	// foreignRecorder 保存越权响应。
	foreignRecorder := httptest.NewRecorder()
	handler.ServeHTTP(foreignRecorder, foreignRequest)
	if foreignRecorder.Code != http.StatusForbidden {
		t.Fatalf("foreign status=%d body=%s", foreignRecorder.Code, foreignRecorder.Body.String())
	}
}
