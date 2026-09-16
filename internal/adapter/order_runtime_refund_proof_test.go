package adapter

import (
	"context"
	"errors"
	"fmt"
	"testing"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/xianyu/mtop"
)

// merchantRefundProofClient 组合基础 MTOP 客户端，并记录退款凭证上传与最终提交。
type merchantRefundProofClient struct {
	mtop.Client
	// uploadErr 是测试指定的上传阶段错误。
	uploadErr error
	// uploaded 保存按顺序收到的图片文件名。
	uploaded []string
	// refuseRequest 保存最终 Merchant refuse 请求。
	refuseRequest mtop.MerchantRefundRefuseRequest
	// refuseCalls 记录最终拒绝请求次数。
	refuseCalls int
}

// FetchRefundDetail 满足退款运行时能力，本测试不调用只读详情。
func (client *merchantRefundProofClient) FetchRefundDetail(context.Context, string, string) (*mtop.RefundDetailResult, error) {
	return nil, errors.New("测试未配置退款详情")
}

// SubmitRefundAction 满足普通退款动作能力，本测试不调用。
func (client *merchantRefundProofClient) SubmitRefundAction(context.Context, string, string, string, mtop.RefundAction) (*mtop.RefundActionResult, error) {
	return nil, errors.New("测试未配置普通退款动作")
}

// CreateMerchantRefundVerification 满足 Merchant 验证能力，本测试不调用。
func (client *merchantRefundProofClient) CreateMerchantRefundVerification(context.Context, string, string) (*mtop.MerchantRefundVerification, error) {
	return nil, errors.New("测试未配置支付验证")
}

// AgreeMerchantRefund 满足 Merchant 同意能力，本测试不调用。
func (client *merchantRefundProofClient) AgreeMerchantRefund(context.Context, string, string, string) (*mtop.MerchantRefundResult, error) {
	return nil, errors.New("测试未配置同意退款")
}

// FetchMerchantRefundRefuseForm 满足 Merchant 拒绝表单能力，本测试不调用。
func (client *merchantRefundProofClient) FetchMerchantRefundRefuseForm(context.Context, string, string, string) (*mtop.MerchantRefundRefuseForm, error) {
	return nil, errors.New("测试未配置拒绝表单")
}

// UploadShipmentEvidenceImage 记录上传顺序并返回对应平台 URL。
func (client *merchantRefundProofClient) UploadShipmentEvidenceImage(_ context.Context, _ string, image mtop.ShipmentEvidenceImage) (*mtop.ShipmentEvidenceUpload, error) {
	client.uploaded = append(client.uploaded, image.Filename)
	if client.uploadErr != nil {
		return nil, client.uploadErr
	}
	return &mtop.ShipmentEvidenceUpload{URL: fmt.Sprintf("https://img.example/%d.png", len(client.uploaded))}, nil
}

// RefuseMerchantRefund 记录最终图片 URL 并返回明确成功。
func (client *merchantRefundProofClient) RefuseMerchantRefund(_ context.Context, _ string, request mtop.MerchantRefundRefuseRequest) (*mtop.MerchantRefundResult, error) {
	client.refuseCalls++
	client.refuseRequest = request
	return &mtop.MerchantRefundResult{Success: true, Message: "提交成功"}, nil
}

// TestOrderRuntimeUploadsRefundProofsBeforeFinalRefuse 验证图片全成功后才提交平台 URL。
func TestOrderRuntimeUploadsRefundProofsBeforeFinalRefuse(t *testing.T) {
	// client 是按顺序返回两张图片 URL 的平台替身。
	client := &merchantRefundProofClient{Client: mtop.NewClient()}
	// runtime 是只注入测试 MTOP 客户端的订单运行时。
	runtime := NewOrderRuntime(nil, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// result、refuseErr 是上传两图并最终拒绝的运行时结果。
	result, refuseErr := runtime.RefuseMerchantRefund(context.Background(), &orderapp.PlatformRuntimeData{Value: "cookie"}, orderapp.MerchantRefundRefusePlatformRequest{
		RefundID: "refund-1", OrderID: "order-1", ReasonID: "reason-1", Images: []orderapp.ShipmentEvidenceImage{
			{Filename: "first.png", ContentType: "image/png", Data: []byte("one")},
			{Filename: "second.jpg", ContentType: "image/jpeg", Data: []byte("two")},
		},
	})
	if refuseErr != nil || result == nil || !result.Success || !result.ActionAttempted || client.refuseCalls != 1 || len(client.refuseRequest.ProofURLs) != 2 || client.refuseRequest.ProofURLs[1] != "https://img.example/2.png" {
		t.Fatalf("result=%+v uploads=%v request=%+v calls=%d err=%v", result, client.uploaded, client.refuseRequest, client.refuseCalls, refuseErr)
	}
}

// TestOrderRuntimeUploadFailureDoesNotAttemptRefund 验证图片上传失败仍可安全重试且不调用最终拒绝。
func TestOrderRuntimeUploadFailureDoesNotAttemptRefund(t *testing.T) {
	// client 在第一张图片上传阶段返回明确失败。
	client := &merchantRefundProofClient{Client: mtop.NewClient(), uploadErr: errors.New("upload failed")}
	// runtime 是只注入测试 MTOP 客户端的订单运行时。
	runtime := NewOrderRuntime(nil, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// result、refuseErr 是上传前失败的可安全重试结果。
	result, refuseErr := runtime.RefuseMerchantRefund(context.Background(), &orderapp.PlatformRuntimeData{Value: "cookie"}, orderapp.MerchantRefundRefusePlatformRequest{
		RefundID: "refund-1", OrderID: "order-1", ReasonID: "reason-1", Images: []orderapp.ShipmentEvidenceImage{{Filename: "proof.png", ContentType: "image/png", Data: []byte("one")}},
	})
	if refuseErr == nil || result == nil || result.ActionAttempted || client.refuseCalls != 0 {
		t.Fatalf("result=%+v calls=%d err=%v", result, client.refuseCalls, refuseErr)
	}
}
