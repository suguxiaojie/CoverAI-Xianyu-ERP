package mtop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCloseOrderWebProtocolUsesDynamicReasonsAndSellerEndpoint 验证原因和卖家关单请求与当前闲鱼网页协议一致。
func TestCloseOrderWebProtocolUsesDynamicReasonsAndSellerEndpoint(t *testing.T) {
	// requests 保存本地服务收到的 API、版本和 data，确保不会调用买家取消接口。
	requests := make([]map[string]any, 0, 2)
	// server 是只处理虚构订单和 Cookie 的本地 MTOP 服务。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if // parseErr 是解析本地表单的错误。
		parseErr := request.ParseForm(); parseErr != nil {
			t.Fatal(parseErr)
		}
		// data 是当前请求签名对应的 JSON 对象。
		var data map[string]any
		if // decodeErr 是解析 data JSON 的错误。
		decodeErr := json.Unmarshal([]byte(request.Form.Get("data")), &data); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		requests = append(requests, map[string]any{"api": request.URL.Query().Get("api"), "version": request.URL.Query().Get("v"), "data": data, "origin": request.Header.Get("Origin")})
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Query().Get("api") {
		case closeOrderReasonsAPIName:
			fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"closeReasons":["双方协商一致","商品无货","双方协商一致",""]}}`)
		case closeOrderSellerAPIName:
			fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{}}`)
		default:
			t.Fatalf("unexpected api: %s", request.URL.Query().Get("api"))
		}
	}))
	defer server.Close()
	// client 把关闭原因和卖家关单端点都指向本地服务。
	client := NewClient()
	client.HTTPClient, client.CloseOrderReasonsURL, client.CloseOrderSellerURL = server.Client(), server.URL, server.URL
	// cookies 是只供本地签名使用的虚构 Cookie。
	cookies := "unb=123; _m_h5_tk=local-token_1"
	// reasons、reasonErr 是动态原因结果和错误。
	reasons, reasonErr := client.FetchCloseOrderReasons(context.Background(), cookies, "5127638256187075541")
	if reasonErr != nil || len(reasons.Reasons) != 2 || reasons.Reasons[0] != "双方协商一致" || reasons.Reasons[1] != "商品无货" {
		t.Fatalf("reasons=%+v err=%v", reasons, reasonErr)
	}
	// result、closeErr 是卖家关单明确结果和错误。
	result, closeErr := client.CloseOrderBySeller(context.Background(), cookies, "5127638256187075541", "双方协商一致")
	if closeErr != nil || result == nil || !result.Success || result.Message != "订单已关闭" {
		t.Fatalf("result=%+v err=%v", result, closeErr)
	}
	if len(requests) != 2 {
		t.Fatalf("requests=%+v", requests)
	}
	// reasonData、closeData 是两个端点的请求 data。
	reasonData, closeData := requests[0]["data"].(map[string]any), requests[1]["data"].(map[string]any)
	if requests[0]["api"] != closeOrderReasonsAPIName || requests[0]["version"] != "1.0" || reasonData["bizOrderId"] != "5127638256187075541" {
		t.Fatalf("reason request=%+v", requests[0])
	}
	if requests[1]["api"] != closeOrderSellerAPIName || requests[1]["version"] != "2.0" || closeData["tid"] != "5127638256187075541" || closeData["bizOrderId"] != "5127638256187075541" || closeData["closeReason"] != "双方协商一致" {
		t.Fatalf("close request=%+v", requests[1])
	}
	if requests[0]["origin"] != "https://www.goofish.com" || requests[1]["origin"] != "https://www.goofish.com" {
		t.Fatalf("origins=%v/%v", requests[0]["origin"], requests[1]["origin"])
	}
}
