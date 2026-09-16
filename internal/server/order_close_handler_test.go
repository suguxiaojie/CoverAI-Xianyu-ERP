package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// TestOrderCloseVersionedRoutesAllowPaidPendingShipmentAndPersistentIdempotency 验证待发货取消、动态原因和重复拦截。
func TestOrderCloseVersionedRoutesAllowPaidPendingShipmentAndPersistentIdempotency(t *testing.T) {
	// server、store、cleanup 是隔离 HTTP 服务、数据库和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是测试卡片与运行记录共用的上下文。
	ctx := context.Background()
	// session 是已付款待发货卡片所属会话。
	session := db.ChatSession{CookieID: "acc1", ChatID: "close-chat", BuyerID: "buyer-1"}
	// pending 是带明确订单号的卖家已付款待发货卡片。
	pending := db.ChatMessage{MessageKey: "close-paid.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: 1000,
		SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardOrderID: "5127638256187075541", SystemCardItemID: "item-1", SystemCardAction: "ship_order"}
	if // inserted、saveErr 表示待发货卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, pending, false); saveErr != nil || !inserted {
		t.Fatalf("inserted=%v err=%v", inserted, saveErr)
	}
	if // orderErr 是创建仍可取消的权威待发货订单状态的错误。
	orderErr := store.Orders.Upsert(ctx, "5127638256187075541", db.OrderUpsertOpts{CookieID: "acc1", OrderStatus: "pending_ship"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	// reasonCalls、closeCalls 统计只读原因和真实卖家关单替身的调用次数。
	var reasonCalls, closeCalls atomic.Int32
	// client 使用本地 RoundTripper 模拟当前官方网页端点。
	client := mtop.NewClient()
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		// api 是当前 MTOP 请求名称。
		api := request.URL.Query().Get("api")
		// responseBody 是当前 API 的本地响应正文。
		responseBody := ""
		switch api {
		case "mtop.taobao.idle.trade.order.close.reason.get":
			reasonCalls.Add(1)
			responseBody = `{"ret":["SUCCESS::调用成功"],"data":{"closeReasons":["双方协商一致","商品无货"]}}`
		case "mtop.taobao.idle.trade.close.by.seller":
			closeCalls.Add(1)
			if // parseErr 是解析本地关单表单的错误。
			parseErr := request.ParseForm(); parseErr != nil {
				t.Fatal(parseErr)
			}
			// submitted 是解码后的关单 data。
			var submitted map[string]any
			if // decodeErr 是解析关单 data JSON 的错误。
			decodeErr := json.Unmarshal([]byte(request.Form.Get("data")), &submitted); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if submitted["tid"] != "5127638256187075541" || submitted["bizOrderId"] != "5127638256187075541" || submitted["closeReason"] != "双方协商一致" {
				t.Fatalf("submitted=%v", submitted)
			}
			responseBody = `{"ret":["SUCCESS::调用成功"],"data":{}}`
		default:
			t.Fatalf("api=%q", api)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody)), Request: request}, nil
	})}
	setTestMTop(server, client)
	// handler、sessionCookie 是完整认证路由和管理员会话。
	handler, sessionCookie := server.Router(), loginHelper(t, server.Router())
	// eligibilityRequest 是不会访问平台的本地关单资格请求。
	eligibilityRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/5127638256187075541/close-eligibility?account_id=acc1", nil)
	eligibilityRequest.AddCookie(sessionCookie)
	// eligibilityRecorder 保存待发货订单的可取消结果。
	eligibilityRecorder := httptest.NewRecorder()
	handler.ServeHTTP(eligibilityRecorder, eligibilityRequest)
	if eligibilityRecorder.Code != http.StatusOK || !strings.Contains(eligibilityRecorder.Body.String(), `"eligible":true`) || !strings.Contains(eligibilityRecorder.Body.String(), `"stage":"pending_ship"`) || reasonCalls.Load() != 0 || closeCalls.Load() != 0 {
		t.Fatalf("eligibility status=%d reasons=%d close=%d body=%s", eligibilityRecorder.Code, reasonCalls.Load(), closeCalls.Load(), eligibilityRecorder.Body.String())
	}
	// reasonsRequest 是动态关闭原因读取请求。
	reasonsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/5127638256187075541/close?account_id=acc1", nil)
	reasonsRequest.AddCookie(sessionCookie)
	// reasonsRecorder 保存原因响应。
	reasonsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(reasonsRecorder, reasonsRequest)
	if reasonsRecorder.Code != http.StatusOK || !strings.Contains(reasonsRecorder.Body.String(), `"双方协商一致"`) {
		t.Fatalf("reasons status=%d body=%s", reasonsRecorder.Code, reasonsRecorder.Body.String())
	}
	// closeBody 是用户从平台原因中选择后的真实关单请求。
	closeBody := `{"account_id":"acc1","reason":"双方协商一致"}`
	// closeRequest 是首次卖家关单请求。
	closeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/orders/5127638256187075541/close", strings.NewReader(closeBody))
	closeRequest.Header.Set("Content-Type", "application/json")
	closeRequest.AddCookie(sessionCookie)
	// closeRecorder 保存首次关单响应。
	closeRecorder := httptest.NewRecorder()
	handler.ServeHTTP(closeRecorder, closeRequest)
	if closeRecorder.Code != http.StatusOK || !strings.Contains(closeRecorder.Body.String(), `"status":"succeeded"`) || closeCalls.Load() != 1 {
		t.Fatalf("close status=%d calls=%d body=%s", closeRecorder.Code, closeCalls.Load(), closeRecorder.Body.String())
	}
	// duplicateRequest 是相同订单的重复关单请求。
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/api/v1/orders/5127638256187075541/close", strings.NewReader(closeBody))
	duplicateRequest.Header.Set("Content-Type", "application/json")
	duplicateRequest.AddCookie(sessionCookie)
	// duplicateRecorder 保存幂等拒绝响应。
	duplicateRecorder := httptest.NewRecorder()
	handler.ServeHTTP(duplicateRecorder, duplicateRequest)
	if duplicateRecorder.Code != http.StatusConflict || closeCalls.Load() != 1 {
		t.Fatalf("duplicate status=%d close=%d reasons=%d body=%s", duplicateRecorder.Code, closeCalls.Load(), reasonCalls.Load(), duplicateRecorder.Body.String())
	}
	// handledEligibilityRequest 是成功关单后再次读取本地资格的请求。
	handledEligibilityRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/5127638256187075541/close-eligibility?account_id=acc1", nil)
	handledEligibilityRequest.AddCookie(sessionCookie)
	// handledEligibilityRecorder 保存成功幂等终态投影后的不可关单结果。
	handledEligibilityRecorder := httptest.NewRecorder()
	handler.ServeHTTP(handledEligibilityRecorder, handledEligibilityRequest)
	if handledEligibilityRecorder.Code != http.StatusOK || !strings.Contains(handledEligibilityRecorder.Body.String(), `"eligible":false`) || closeCalls.Load() != 1 {
		t.Fatalf("handled eligibility status=%d close=%d body=%s", handledEligibilityRecorder.Code, closeCalls.Load(), handledEligibilityRecorder.Body.String())
	}
	// runStatus 是 account_task_runs 保存的关单成功终态。
	var runStatus string
	if // runErr 是读取关单幂等终态的错误。
	runErr := store.DB.QueryRowContext(ctx, `SELECT status FROM account_task_runs WHERE task_type=? AND target_id=?`, "order_close", "5127638256187075541").Scan(&runStatus); runErr != nil || runStatus != "success" {
		t.Fatalf("run status=%q err=%v", runStatus, runErr)
	}
}
