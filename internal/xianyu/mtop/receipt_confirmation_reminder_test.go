package mtop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRemindBuyerConfirmReceiptUsesOfficialSystemCardContract 验证提醒使用官方 API、版本和单笔批量参数。
func TestRemindBuyerConfirmReceiptUsesOfficialSystemCardContract(t *testing.T) {
	// received 保存测试服务收到的官方请求数据。
	var received map[string]any
	// server 模拟闲鱼明确生成系统卡片的成功响应。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("api") != receiptConfirmationReminderAPIName || request.URL.Query().Get("v") != "1.0" {
			t.Fatalf("api=%q version=%q", request.URL.Query().Get("api"), request.URL.Query().Get("v"))
		}
		if request.Header.Get("Referer") != "https://seller.goofish.com/" {
			t.Fatalf("referer=%q", request.Header.Get("Referer"))
		}
		if // parseErr 是官方表单请求体解析错误。
		parseErr := request.ParseForm(); parseErr != nil {
			t.Fatal(parseErr)
		}
		if // decodeErr 是官方 data JSON 解码错误。
		decodeErr := json.Unmarshal([]byte(request.Form.Get("data")), &received); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ret":["SUCCESS::调用成功"],"data":{"module":{"totalNum":1,"successNum":1,"failOrderInfos":[]}}}`))
	}))
	defer server.Close()
	// client 使用本地服务覆盖官方提醒端点。
	client := &ClientImpl{HTTPClient: server.Client(), ReceiptReminderURL: server.URL}
	// result、requestErr 是本地协议验证结果和错误。
	result, requestErr := client.RemindBuyerConfirmReceipt(context.Background(), "unb=123; _m_h5_tk=token_1", " order-1 ")
	if requestErr != nil || result == nil || !result.Success || result.Message == "" {
		t.Fatalf("result=%+v err=%v", result, requestErr)
	}
	// orderIDs 是官方 orderIdList 参数。
	orderIDs, _ := received["orderIdList"].([]any)
	if len(orderIDs) != 1 || orderIDs[0] != "order-1" || received["remindAllOrder"] != false || len(received) != 2 {
		t.Fatalf("data=%+v", received)
	}
}

// TestRemindBuyerConfirmReceiptPreservesDailyLimitFailure 验证平台每日一次限制等明确失败不会伪造成成功。
func TestRemindBuyerConfirmReceiptPreservesDailyLimitFailure(t *testing.T) {
	// server 返回当前订单的明确失败原因。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ret":["SUCCESS::调用成功"],"data":{"module":{"totalNum":1,"successNum":0,"failOrderInfos":[{"orderId":"order-1","errorMsg":"同一订单每天最多提醒一次"}]}}}`))
	}))
	defer server.Close()
	// client 使用本地明确失败夹具。
	client := &ClientImpl{HTTPClient: server.Client(), ReceiptReminderURL: server.URL}
	// result、requestErr 是平台明确失败结果和传输错误。
	result, requestErr := client.RemindBuyerConfirmReceipt(context.Background(), "unb=123; _m_h5_tk=token_1", "order-1")
	if requestErr != nil || result == nil || result.Success || result.Message != "同一订单每天最多提醒一次" {
		t.Fatalf("result=%+v err=%v", result, requestErr)
	}
}
