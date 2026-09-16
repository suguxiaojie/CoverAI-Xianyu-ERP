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

// TestOrderAdjustPriceVersionedRoutesUseDynamicFormAndPersistentIdempotency 验证 HTTP 表单、submit 分值和重复拦截。
func TestOrderAdjustPriceVersionedRoutesUseDynamicFormAndPersistentIdempotency(t *testing.T) {
	// server、store、cleanup 是隔离 HTTP 服务、数据库和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是测试卡片与运行记录共用的上下文。
	ctx := context.Background()
	// session 是待付款卡片所属会话。
	session := db.ChatSession{CookieID: "acc1", ChatID: "price-chat", BuyerID: "buyer-1"}
	// pending 是带明确订单和 adjust_price 动作的卖家待付款卡片。
	pending := db.ChatMessage{MessageKey: "price-pending.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已拍下，待付款", Status: "received", SentAt: 1000,
		SystemCardKind: "trade", SystemCardEvent: "order_pending_payment", SystemCardOrderID: "5127694777172175924", SystemCardItemID: "item-1", SystemCardAction: "adjust_price"}
	if // inserted、saveErr 表示待付款卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, pending, false); saveErr != nil || !inserted {
		t.Fatalf("inserted=%v err=%v", inserted, saveErr)
	}
	// renderCalls、submitCalls 统计只读表单和真实动作替身的调用次数。
	var renderCalls, submitCalls atomic.Int32
	// client 使用本地 RoundTripper 模拟已抓包的两个官方端点。
	client := mtop.NewClient()
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		// api 是当前 MTOP 请求名称。
		api := request.URL.Query().Get("api")
		// responseBody 是当前 API 的本地响应正文。
		responseBody := ""
		switch api {
		case "mtop.taobao.idle.trade.order.modify.price.render":
			renderCalls.Add(1)
			responseBody = `{"ret":["SUCCESS::调用成功"],"data":{"title":"修改价格","modifyPriceRenderList":[{"key":"modifyFee","name":"商品价格","prefixText":"¥","price":"0.10","readOnly":false},{"key":"newTransportFee","name":"运费","prefixText":"¥","price":"0.00","readOnly":false}]}}`
		case "mtop.taobao.idle.trade.user.adjust.price":
			submitCalls.Add(1)
			if // parseErr 是解析本地 submit 表单的错误。
			parseErr := request.ParseForm(); parseErr != nil {
				t.Fatal(parseErr)
			}
			// submitted 是解码后的 submit data。
			var submitted map[string]any
			if // decodeErr 是解析 submit data JSON 的错误。
			decodeErr := json.Unmarshal([]byte(request.Form.Get("data")), &submitted); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if submitted["modifyFee"] != "20" || submitted["newTransportFee"] != "0" || submitted["orderId"] != "5127694777172175924" {
				t.Fatalf("submitted=%v", submitted)
			}
			responseBody = `{"ret":["SUCCESS::调用成功"],"data":{"success":true}}`
		default:
			t.Fatalf("api=%q", api)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody)), Request: request}, nil
	})}
	setTestMTop(server, client)
	// handler、sessionCookie 是完整认证路由和管理员会话。
	handler, sessionCookie := server.Router(), loginHelper(t, server.Router())
	// formRequest 是动态改价表单读取请求。
	formRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/5127694777172175924/adjust-price?account_id=acc1", nil)
	formRequest.AddCookie(sessionCookie)
	// formRecorder 保存 render 响应。
	formRecorder := httptest.NewRecorder()
	handler.ServeHTTP(formRecorder, formRequest)
	if formRecorder.Code != http.StatusOK || !strings.Contains(formRecorder.Body.String(), `"key":"modifyFee"`) || !strings.Contains(formRecorder.Body.String(), `"value":"0.10"`) {
		t.Fatalf("form status=%d body=%s", formRecorder.Code, formRecorder.Body.String())
	}
	// submitBody 是使用元金额的具名真实改价请求。
	submitBody := `{"account_id":"acc1","fields":[{"key":"modifyFee","value":"0.20"},{"key":"newTransportFee","value":"0.00"}]}`
	// submitRequest 是首次改价请求。
	submitRequest := httptest.NewRequest(http.MethodPost, "/api/v1/orders/5127694777172175924/adjust-price", strings.NewReader(submitBody))
	submitRequest.Header.Set("Content-Type", "application/json")
	submitRequest.AddCookie(sessionCookie)
	// submitRecorder 保存首次改价响应。
	submitRecorder := httptest.NewRecorder()
	handler.ServeHTTP(submitRecorder, submitRequest)
	if submitRecorder.Code != http.StatusOK || !strings.Contains(submitRecorder.Body.String(), `"status":"succeeded"`) || submitCalls.Load() != 1 {
		t.Fatalf("submit status=%d calls=%d body=%s", submitRecorder.Code, submitCalls.Load(), submitRecorder.Body.String())
	}
	// duplicateRequest 是相同订单和目标金额的重复提交。
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/api/v1/orders/5127694777172175924/adjust-price", strings.NewReader(submitBody))
	duplicateRequest.Header.Set("Content-Type", "application/json")
	duplicateRequest.AddCookie(sessionCookie)
	// duplicateRecorder 保存幂等拒绝响应。
	duplicateRecorder := httptest.NewRecorder()
	handler.ServeHTTP(duplicateRecorder, duplicateRequest)
	if duplicateRecorder.Code != http.StatusConflict || submitCalls.Load() != 1 {
		t.Fatalf("duplicate status=%d submit=%d render=%d body=%s", duplicateRecorder.Code, submitCalls.Load(), renderCalls.Load(), duplicateRecorder.Body.String())
	}
	// runStatus 是 account_task_runs 保存的改价成功终态。
	var runStatus string
	if // runErr 是读取改价幂等终态的错误。
	runErr := store.DB.QueryRowContext(ctx, `SELECT status FROM account_task_runs WHERE task_type=? AND target_id=?`, "order_price_adjust", "5127694777172175924").Scan(&runStatus); runErr != nil || runStatus != "success" {
		t.Fatalf("run status=%q err=%v", runStatus, runErr)
	}
}
