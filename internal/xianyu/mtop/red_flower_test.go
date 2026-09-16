package mtop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequestRedFlowerUsesOfficialContract 验证求花请求使用官方 API、版本和最小订单参数。
func TestRequestRedFlowerUsesOfficialContract(t *testing.T) {
	// received 保存测试服务收到的求花请求数据。
	var received map[string]any
	// server 模拟闲鱼 MTOP 求花成功响应，不访问真实平台。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("api") != redFlowerRequestAPIName || request.URL.Query().Get("v") != "1.0" {
			t.Fatalf("api=%q version=%q", request.URL.Query().Get("api"), request.URL.Query().Get("v"))
		}
		if request.Header.Get("Referer") != "https://h5.m.goofish.com/" {
			t.Fatalf("referer=%q", request.Header.Get("Referer"))
		}
		// parseErr 是表单请求体解析错误。
		if parseErr := request.ParseForm(); parseErr != nil {
			t.Fatal(parseErr)
		}
		// decodeErr 是官方 data JSON 解码错误。
		if decodeErr := json.Unmarshal([]byte(request.Form.Get("data")), &received); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ret":["SUCCESS::调用成功"],"data":{"success":true,"message":"求花成功"}}`))
	}))
	defer server.Close()
	// client 使用本地服务覆盖求花端点。
	client := &ClientImpl{HTTPClient: server.Client(), RedFlowerURL: server.URL}
	// result、requestErr 是本地求花结果和请求错误。
	result, requestErr := client.RequestRedFlower(context.Background(), "unb=123; _m_h5_tk=token_1", " order-1 ", "im")
	if requestErr != nil || result == nil || !result.Success || result.Message != "求花成功" {
		t.Fatalf("result=%+v err=%v", result, requestErr)
	}
	if received["orderId"] != "order-1" || received["channel"] != "im" || len(received) != 2 {
		t.Fatalf("data=%+v", received)
	}
}

// TestRequestRedFlowerPreservesExplicitFailure 验证 HTTP 成功但 data.success=false 不会伪造成已发送。
func TestRequestRedFlowerPreservesExplicitFailure(t *testing.T) {
	// server 返回平台明确拒绝结果。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ret":["SUCCESS::调用成功"],"data":{"success":"false","tips":"订单超过可求花期限"}}`))
	}))
	defer server.Close()
	// client 使用本地失败夹具。
	client := &ClientImpl{HTTPClient: server.Client(), RedFlowerURL: server.URL}
	// result、requestErr 是平台明确失败结果和传输错误。
	result, requestErr := client.RequestRedFlower(context.Background(), "unb=123; _m_h5_tk=token_1", "order-1", "")
	if requestErr != nil || result == nil || result.Success || result.Message != "订单超过可求花期限" {
		t.Fatalf("result=%+v err=%v", result, requestErr)
	}
}
