package mtop

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFetchRefundDetailParsesOfficialComponents 验证官方 data.data 动态组件被收口为只读退款详情。
func TestFetchRefundDetailParsesOfficialComponents(t *testing.T) {
	// server 模拟官方退款详情 MTOP 成功响应。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Set-Cookie", "_m_h5_tk=fresh_token; Path=/")
		_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"data":{"orderId":"1234567890123","refundId":"refund-1","refundStatus":"1","seller":true,"components":[{"render":"nodeStatusInfo","data":{"title":"等待卖家处理"}},{"render":"refundInfo","data":{"refundId":"refund-1","refundType":"仅退款","refundAmount":"135.00","refundReason":"协商一致退款","refundApplyTime":"2026-08-23 21:16"}},{"render":"refundDescribe","data":{"buyerContent":"套餐升级","buyerImages":[{"url":"https://img.example/a.jpg"}],"buyerVideos":[{"playUrl":"https://video.example/a.mp4"}]}},{"render":"bottomBar","data":[{"code":"sellerAgreeRefund","name":"同意退款","clickEvent":{"type":"doubleCheck","data":{"doubleCheck":{"code":"DOUBLE_CONFIRM_WINDOWS","doubleCheckVO":{"title":"确认同意退款","buttonList":[{"code":"cancel","name":"取消","clickEvent":{"type":"action","data":{"action":"CANCEL"}}},{"code":"agreeRefundApply","name":"确认同意","clickEvent":{"type":"mtop","data":{"mtop":{"apiName":"mtop.taobao.idle.refund.precheck","apiVersion":"1.0","params":{"refundId":"refund-1"}}}}}]}}}}},{"code":"sellerRejectRefund","name":"拒绝退款","clickEvent":{"type":"doubleCheck","data":{"doubleCheck":{"code":"DOUBLE_CONFIRM_WINDOWS","doubleCheckVO":{"title":"确认拒绝退款","desc":"拒绝后买家可申请平台介入","buttonList":[{"code":"cancel","name":"取消","clickEvent":{"type":"action","data":{"action":"CANCEL"}}},{"code":"confirm","name":"确认拒绝","clickEvent":{"type":"mtop","data":{"mtop":{"apiName":"mtop.taobao.idle.refund.reject.refund","apiVersion":"1.0","params":{"refundId":"refund-1"}}}}}]}}}}}]}]}}}`)
	}))
	defer server.Close()
	// client 是仅将退款详情端点指向本地替身的 MTOP 客户端。
	client := &ClientImpl{RefundDetailURL: server.URL, TokenURL: server.URL, HTTPClient: server.Client()}
	// result、err 是退款详情解析结果和请求错误。
	result, err := client.FetchRefundDetail(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", "1234567890123")
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderID != "1234567890123" || result.RefundID != "refund-1" || result.RefundType != "仅退款" || result.RefundReason != "协商一致退款" || result.RefundAmount != "135.00" || result.BuyerDescription != "套餐升级" {
		t.Fatalf("refund detail=%+v", result)
	}
	if len(result.BuyerImages) != 1 || len(result.BuyerVideos) != 1 {
		t.Fatalf("refund media=%+v/%+v", result.BuyerImages, result.BuyerVideos)
	}
	if !result.Seller || result.RefundStatusText != "等待卖家处理" || len(result.Actions) != 2 || result.Actions[0].Kind != "agree" || result.Actions[0].Mode != "merchant_verify" || result.Actions[0].APIName != "" || result.Actions[1].Kind != "reject" || result.Actions[1].Mode != "direct" || result.Actions[1].ConfirmTitle != "确认拒绝退款" {
		t.Fatalf("refund actions=%+v", result)
	}
}

// TestSubmitRefundActionUsesLatestDynamicDescriptor 验证退款动作只使用平台详情下发且通过门禁的 MTOP 描述。
func TestSubmitRefundActionUsesLatestDynamicDescriptor(t *testing.T) {
	// receivedData 保存测试服务收到的退款动作参数。
	var receivedData string
	// server 模拟官方退款动作成功响应并记录请求体。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// body 是本次 MTOP 表单请求体。
		body, _ := io.ReadAll(request.Body)
		receivedData = string(body)
		_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"message":"退款申请已处理"}}`)
	}))
	defer server.Close()
	// client 是只把动态退款动作端点指向本地替身的客户端。
	client := &ClientImpl{RefundActionURL: server.URL, TokenURL: server.URL, HTTPClient: server.Client()}
	// action 是官方详情下发的普通同意退款动作。
	action := RefundAction{Code: "sellerAgreeRefund", Name: "同意退款", Kind: "agree", Mode: "direct", APIName: "mtop.taobao.idle.refund.agree.refund", APIVersion: "1.0", Params: map[string]any{"refundId": "stale-refund"}}
	// result、submitErr 是动作结果和请求错误。
	result, submitErr := client.SubmitRefundAction(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", "1234567890123", "refund-1", action)
	if submitErr != nil {
		t.Fatal(submitErr)
	}
	if !result.Success || result.Message != "退款申请已处理" || !strings.Contains(receivedData, "refund-1") || strings.Contains(receivedData, "stale-refund") {
		t.Fatalf("result=%+v data=%s", result, receivedData)
	}
}

// TestSubmitRefundActionTreatsIframeAsOfficialAuthentication 验证前置 MTOP 返回 iframeUrl 时绝不伪造退款成功。
func TestSubmitRefundActionTreatsIframeAsOfficialAuthentication(t *testing.T) {
	// server 模拟闲鱼前置校验成功但仍要求支付密码验证的响应。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"data":{"iframeUrl":"https://cashier.example/verify?uuid=test"}}}`)
	}))
	defer server.Close()
	// client 是只把测试动作端点指向前置校验替身的客户端。
	client := &ClientImpl{RefundActionURL: server.URL, TokenURL: server.URL, HTTPClient: server.Client()}
	// action 是假设平台未预先标记原生认证的直接动作，用于响应层最终防线。
	action := RefundAction{Code: "sellerAgreeRefund", Name: "同意退款", Kind: "agree", Mode: "direct", APIName: "mtop.taobao.idle.refund.precheck", APIVersion: "1.0", Params: map[string]any{"refundId": "refund-1"}}
	// result、submitErr 是前置响应的安全分类结果和错误。
	result, submitErr := client.SubmitRefundAction(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", "1234567890123", "refund-1", action)
	if submitErr != nil {
		t.Fatal(submitErr)
	}
	if result.Success || !result.RequiresOfficial {
		t.Fatalf("result=%+v", result)
	}
}
