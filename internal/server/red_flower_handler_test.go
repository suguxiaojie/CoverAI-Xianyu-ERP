package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"xianyu-go/internal/xianyu/mtop"
)

// TestRequestOrderRedFlowerUsesVersionedRouteAndServerIdempotency 验证手动求花只调用一次平台并阻止重复发送。
func TestRequestOrderRedFlowerUsesVersionedRouteAndServerIdempotency(t *testing.T) {
	// server、store、cleanup 是隔离 HTTP 服务、数据库和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// now 是测试订单付款时间基准。
	now := time.Now().UTC()
	// _, insertErr 是已付款卖家订单写入结果和错误。
	_, insertErr := store.DB.ExecContext(context.Background(), `INSERT INTO orders
		(order_id,item_id,buyer_id,order_status,cookie_id,chat_id,paid_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, "flower-order", "item-1", "buyer-1", "completed", "acc1", "chat-1", now.Add(-time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	// platformCalls 统计官方求花端点调用次数。
	var platformCalls atomic.Int32
	// client 使用本地 RoundTripper 模拟平台明确成功响应。
	client := mtop.NewClient()
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		platformCalls.Add(1)
		if request.URL.Query().Get("api") != "mtop.taobao.idlemessage.red.flower" {
			t.Fatalf("api=%q", request.URL.Query().Get("api"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ret":["SUCCESS::调用成功"],"data":{"success":true,"message":"求花成功"}}`)), Request: request}, nil
	})}
	setTestMTop(server, client)
	// handler 是完整认证与版本路由。
	handler := server.Router()
	// sessionCookie 是测试管理员登录会话。
	sessionCookie := loginHelper(t, handler)
	// firstRequest 是首次手动求花请求。
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/v1/orders/flower-order/request-red-flower", nil)
	firstRequest.AddCookie(sessionCookie)
	// firstRecorder 保存首次求花响应。
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusOK || !strings.Contains(firstRecorder.Body.String(), `"status":"succeeded"`) {
		t.Fatalf("first status=%d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}
	// statusRequest 是页面重开后使用的只读求花状态查询。
	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/flower-order/request-red-flower", nil)
	statusRequest.AddCookie(sessionCookie)
	// statusRecorder 保存持久状态响应。
	statusRecorder := httptest.NewRecorder()
	handler.ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK || !strings.Contains(statusRecorder.Body.String(), `"status":"succeeded"`) || !strings.Contains(statusRecorder.Body.String(), "同步可能需要几分钟") {
		t.Fatalf("status code=%d body=%s", statusRecorder.Code, statusRecorder.Body.String())
	}
	// secondRequest 是同订单重复点击请求。
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/v1/orders/flower-order/request-red-flower", nil)
	secondRequest.AddCookie(sessionCookie)
	// secondRecorder 保存重复请求响应。
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusConflict || platformCalls.Load() != 1 {
		t.Fatalf("second status=%d calls=%d body=%s", secondRecorder.Code, platformCalls.Load(), secondRecorder.Body.String())
	}
	// runStatus 是数据库保存的求花幂等终态。
	var runStatus string
	// queryErr 是求花运行终态查询错误。
	if queryErr := store.DB.QueryRowContext(context.Background(), `SELECT status FROM account_task_runs WHERE run_key=?`, "red_flower_request:acc1:flower-order").Scan(&runStatus); queryErr != nil || runStatus != "success" {
		t.Fatalf("run status=%q err=%v", runStatus, queryErr)
	}
}

// TestRequestOrderRedFlowerRejectsExpiredOrderBeforePlatform 验证超过三十天的订单不会调用平台。
func TestRequestOrderRedFlowerRejectsExpiredOrderBeforePlatform(t *testing.T) {
	// server、store、cleanup 是隔离 HTTP 服务、数据库和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// now 是测试订单时间基准。
	now := time.Now().UTC()
	// _, insertErr 是过期订单写入结果和错误。
	_, insertErr := store.DB.ExecContext(context.Background(), `INSERT INTO orders
		(order_id,item_id,buyer_id,order_status,cookie_id,chat_id,paid_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, "expired-flower-order", "item-1", "buyer-1", "completed", "acc1", "chat-1", now.Add(-31*24*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if insertErr != nil {
		t.Fatal(insertErr)
	}
	// handler 是完整认证与版本路由。
	handler := server.Router()
	// sessionCookie 是测试管理员登录会话。
	sessionCookie := loginHelper(t, handler)
	// request 是过期订单的求花请求。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/expired-flower-order/request-red-flower", nil)
	request.AddCookie(sessionCookie)
	// recorder 保存资格拒绝响应。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "超过 30 天") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
