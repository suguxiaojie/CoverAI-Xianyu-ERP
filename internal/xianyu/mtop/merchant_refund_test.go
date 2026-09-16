package mtop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMerchantRefundVerificationAndAgree 验证 PC 支付验证和最终 Merchant 退款使用不同接口及授权字段。
func TestMerchantRefundVerificationAndAgree(t *testing.T) {
	// requests 保存测试服务观察到的 API 和请求体。
	requests := make([]string, 0, 2)
	// server 模拟支付验证 URL 和最终退款响应。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Query().Get("api")+"|"+request.FormValue("data"))
		switch request.URL.Query().Get("api") {
		case merchantRefundVerifyAPI:
			_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"module":{"verifyUrl":"https://pcauth-site.alipay.com/PASSWORD?token=test","token":"auth-short"}}}`)
		case merchantRefundAgreeAPI:
			_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"message":"退款成功"}}`)
		default:
			http.Error(writer, "unexpected", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	// client 将两个 Merchant 端点指向本地测试服务。
	client := &ClientImpl{MerchantRefundVerifyURL: server.URL, MerchantRefundAgreeURL: server.URL, TokenURL: server.URL, HTTPClient: server.Client()}
	// verification、verifyErr 是短期支付验证结果和错误。
	verification, verifyErr := client.CreateMerchantRefundVerification(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", "refund-1")
	if verifyErr != nil || verification.AuthToken != "auth-short" || !strings.HasPrefix(verification.VerifyURL, "https://pcauth-site.alipay.com/") {
		t.Fatalf("verification=%+v err=%v", verification, verifyErr)
	}
	// result、agreeErr 是最终 Merchant 同意退款结果和错误。
	result, agreeErr := client.AgreeMerchantRefund(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", "refund-1", verification.AuthToken)
	if agreeErr != nil || !result.Success || len(requests) != 2 || !strings.Contains(requests[1], "auth-short") {
		t.Fatalf("result=%+v requests=%v err=%v", result, requests, agreeErr)
	}
}

// TestMerchantRefundRefuseRenderAndSubmit 验证动态原因、手机专属门禁和最终拒绝字段。
func TestMerchantRefundRefuseRenderAndSubmit(t *testing.T) {
	// submitData 保存测试服务收到的最终拒绝请求体。
	var submitData string
	// server 模拟动态拒绝表单和最终成功响应。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("api") {
		case merchantRefundRefuseRenderAPI:
			_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"data":{"refuseReasonList":[{"reasonId":"1","reasonName":"已协商其它方案","refuseReasonId":"11"},{"reasonId":"20653007","reasonName":"需要手机处理","refuseReasonId":"12"}],"refuseNegotiation":{"negotiationType":"NEGOTIATION_REFUND_FEE","negotiationRefundFee":{"minRefundFee":"1","maxRefundFee":"100"}},"refuseProof":{"mustProof":false,"desc":"补充说明"}}}}`)
		case merchantRefundRefuseAPI:
			submitData = request.FormValue("data")
			_, _ = fmt.Fprint(writer, `{"ret":["SUCCESS::调用成功"],"data":{"message":"提交成功"}}`)
		}
	}))
	defer server.Close()
	// client 将拒绝表单和提交端点指向本地服务。
	client := &ClientImpl{MerchantRefundRefuseRenderURL: server.URL, MerchantRefundRefuseURL: server.URL, TokenURL: server.URL, HTTPClient: server.Client()}
	// form、formErr 是动态拒绝表单和错误。
	form, formErr := client.FetchMerchantRefundRefuseForm(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", "refund-1", "11")
	if formErr != nil || len(form.Reasons) != 2 || form.Reasons[1].RequiresApp != true || !form.Negotiation.Enabled || form.Negotiation.MinCents != 1 || form.Negotiation.MaxCents != 100 {
		t.Fatalf("form=%+v err=%v", form, formErr)
	}
	// result、submitErr 是最终拒绝结果和错误。
	result, submitErr := client.RefuseMerchantRefund(context.Background(), "_m_h5_tk=token_1; _m_h5_tk_enc=enc", MerchantRefundRefuseRequest{RefundID: "refund-1", OrderID: "order-1", ReasonID: "11", Description: "已协商", ProofURLs: []string{"https://img.example/proof.png"}})
	if submitErr != nil || !result.Success || !strings.Contains(submitData, "refuseReasonId") || !strings.Contains(submitData, "refuseProof") || !strings.Contains(submitData, "https://img.example/proof.png") {
		t.Fatalf("result=%+v data=%s err=%v", result, submitData, submitErr)
	}
}
