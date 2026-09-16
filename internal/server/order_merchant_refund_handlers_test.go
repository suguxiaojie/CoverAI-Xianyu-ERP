package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDecodeMerchantRefundRefuseMultipartReadsProofImage 验证退款 multipart 字段和真实图片进入应用请求。
func TestDecodeMerchantRefundRefuseMultipartReadsProofImage(t *testing.T) {
	// body 构造与浏览器 FormData 一致的退款凭证表单字节。
	var body bytes.Buffer
	// formWriter 负责写入 multipart 字段和文件边界。
	formWriter := multipart.NewWriter(&body)
	// fields 是当前表单的非文件字段。
	fields := map[string]string{"account_id": "seller-1", "reason_id": "reason-1", "description": "已经发货", "negotiation_cents": "1"}
	for fieldName, fieldValue := range fields { // fieldName、fieldValue 是当前待写入的表单键值。
		if fieldErr := formWriter.WriteField(fieldName, fieldValue); fieldErr != nil {
			t.Fatal(fieldErr)
		}
	}
	// filePart、partErr 是退款图片 multipart 文件段和创建错误。
	filePart, partErr := formWriter.CreateFormFile("images", "proof.png")
	if partErr != nil {
		t.Fatal(partErr)
	}
	// pngHeader 是服务端真实 MIME 检测识别的 PNG 固定签名。
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	// writeErr 是测试 PNG 签名写入文件段的错误。
	if _, writeErr := filePart.Write(pngHeader); writeErr != nil {
		t.Fatal(writeErr)
	}
	// closeErr 是 multipart 结束边界写入错误。
	if closeErr := formWriter.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	// request 是待解析的退款 multipart HTTP 请求。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1234567890/refund-refuse", &body)
	// recorder 是 MaxBytesReader 使用的响应记录器。
	recorder := httptest.NewRecorder()
	request.Header.Set("Content-Type", formWriter.FormDataContentType())
	// input、images、decodeErr 是解析后的字段、图片和错误。
	input, images, decodeErr := decodeMerchantRefundRefuseRequest(recorder, request)
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	if decodeErr != nil || input.AccountID != "seller-1" || input.ReasonID != "reason-1" || input.Description != "已经发货" || input.NegotiationCents != 1 || len(images) != 1 || images[0].ContentType != "image/png" {
		t.Fatalf("input=%+v images=%+v err=%v", input, images, decodeErr)
	}
}
