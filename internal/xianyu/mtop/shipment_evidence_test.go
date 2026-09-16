package mtop

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestUploadShipmentEvidenceUsesFleamarketScope 验证发货凭证上传复用官方 fleamarket 作用域和卖家工作台请求来源。
func TestUploadShipmentEvidenceUsesFleamarketScope(t *testing.T) {
	// requestSeen 标记上传请求已经经过本地 RoundTripper 验证。
	requestSeen := false
	// client 使用内存响应拦截器，禁止真实上传测试图片。
	client := &ClientImpl{HTTPClient: &http.Client{Transport: cookieSessionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestSeen = true
		if request.URL.Query().Get("appkey") != "fleamarket" || request.URL.Query().Get("floderId") != "0" {
			t.Fatalf("url=%s", request.URL.String())
		}
		if request.Header.Get("Origin") != "https://seller.goofish.com" || request.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			t.Fatalf("headers=%v", request.Header)
		}
		// parseErr 是官方 file 字段 multipart 解析错误。
		if parseErr := request.ParseMultipartForm(1 << 20); parseErr != nil {
			t.Fatal(parseErr)
		}
		// file、header、fileErr 是 multipart 中唯一的官方 file 字段及其元数据。
		file, header, fileErr := request.FormFile("file")
		if fileErr != nil {
			t.Fatal(fileErr)
		}
		defer file.Close()
		if header.Filename != "proof.png" {
			t.Fatalf("filename=%s", header.Filename)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"success":true,"object":{"url":"https://img.example/proof.png","pix":"10x20"}}`)), Request: request}, nil
	})}}
	// upload、uploadErr 是官方响应投影出的凭证 URL。
	upload, uploadErr := client.UploadShipmentEvidenceImage(context.Background(), consignCookies, ShipmentEvidenceImage{Filename: "proof.png", ContentType: "image/png", Data: []byte("png")})
	if uploadErr != nil || upload == nil || upload.URL != "https://img.example/proof.png" || !requestSeen {
		t.Fatalf("upload=%+v seen=%v err=%v", upload, requestSeen, uploadErr)
	}
}
