package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGlobalKeywordEnabledEndpoints 验证单规则与全部规则开关只更新当前用户规则且保持内容不变。
func TestGlobalKeywordEnabledEndpoints(t *testing.T) {
	// server、store、cleanup 是隔离 HTTP 服务、数据库和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是包含认证与全局关键词路由的测试处理器。
	handler := server.Router()
	// sessionCookie 是管理员登录后的会话 Cookie。
	sessionCookie := loginHelper(t, handler)
	// createRequest 创建默认开启并绑定 acc1 的全局关键词规则。
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/global-reply-rule-groups", strings.NewReader(`{"keywords":["开关"],"reply":"已回复","type":"text","message_scopes":["customer"],"account_ids":["acc1"],"enabled":true}`))
	createRequest.AddCookie(sessionCookie)
	// createResponse 保存规则创建响应。
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	// groupID 是测试数据库中新建规则的稳定组标识。
	var groupID string
	if // groupErr 是读取新建规则组标识的数据库错误。
	groupErr := store.DB.QueryRowContext(context.Background(), `SELECT group_id FROM keywords WHERE cookie_id=? AND keyword=?`, "acc1", "开关").Scan(&groupID); groupErr != nil {
		t.Fatal(groupErr)
	}
	// disableRequest 关闭刚创建的全局规则组。
	disableRequest := httptest.NewRequest(http.MethodPut, "/api/v1/global-reply-rule-groups/"+groupID+"/enabled", strings.NewReader(`{"enabled":false}`))
	disableRequest.AddCookie(sessionCookie)
	// disableResponse 保存单规则关闭响应。
	disableResponse := httptest.NewRecorder()
	handler.ServeHTTP(disableResponse, disableRequest)
	if disableResponse.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableResponse.Code, disableResponse.Body.String())
	}
	// enabled 是数据库中关闭后的持久化状态。
	var enabled bool
	if // enabledErr 是读取单规则关闭状态的数据库错误。
	enabledErr := store.DB.QueryRowContext(context.Background(), `SELECT enabled FROM keywords WHERE group_id=?`, groupID).Scan(&enabled); enabledErr != nil || enabled {
		t.Fatalf("enabled=%v err=%v", enabled, enabledErr)
	}
	// enableAllRequest 重新开启当前用户全部关键词规则。
	enableAllRequest := httptest.NewRequest(http.MethodPut, "/api/v1/global-reply-rule-groups/enabled", strings.NewReader(`{"enabled":true}`))
	enableAllRequest.AddCookie(sessionCookie)
	// enableAllResponse 保存全部规则开启响应。
	enableAllResponse := httptest.NewRecorder()
	handler.ServeHTTP(enableAllResponse, enableAllRequest)
	if enableAllResponse.Code != http.StatusOK {
		t.Fatalf("enable all status=%d body=%s", enableAllResponse.Code, enableAllResponse.Body.String())
	}
	if // enabledErr 是读取全部开启后状态的数据库错误。
	enabledErr := store.DB.QueryRowContext(context.Background(), `SELECT enabled FROM keywords WHERE group_id=?`, groupID).Scan(&enabled); enabledErr != nil || !enabled {
		t.Fatalf("enabled=%v err=%v", enabled, enabledErr)
	}
}
