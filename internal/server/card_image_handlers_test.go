package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// cardImagePNG 返回 multipart 卡密上传测试使用的固定 1×1 PNG 字节。
func cardImagePNG(t *testing.T) []byte {
	t.Helper()
	// data、decodeErr 分别保存固定图片夹具和 Base64 解码错误。
	data, decodeErr := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if decodeErr != nil {
		t.Fatalf("解码 PNG 夹具失败: %v", decodeErr)
	}
	return data
}

// buildCardImageMultipart 构造与浏览器 FormData 一致的卡券 payload 和可选图片字段。
func buildCardImageMultipart(t *testing.T, payload string, image []byte) (*bytes.Buffer, string) {
	t.Helper()
	// body 是编码完成后交给 HTTP 测试请求的 multipart 请求体。
	body := &bytes.Buffer{}
	// writer 负责写入 multipart 边界、payload 和图片文件字段。
	writer := multipart.NewWriter(body)
	// payloadErr 表示卡券 JSON 字段无法写入 multipart 测试请求。
	if payloadErr := writer.WriteField("payload", payload); payloadErr != nil {
		t.Fatalf("写入卡密 payload 失败: %v", payloadErr)
	}
	if image != nil {
		// imagePart、partErr 分别保存图片表单字段写入器和创建错误。
		imagePart, partErr := writer.CreateFormFile("image", "拖拽图片.png")
		if partErr != nil {
			t.Fatalf("创建卡密图片字段失败: %v", partErr)
		}
		// writeErr 表示图片夹具无法写入 multipart 文件字段。
		if _, writeErr := imagePart.Write(image); writeErr != nil {
			t.Fatalf("写入卡密图片失败: %v", writeErr)
		}
	}
	// closeErr 表示 multipart 结束边界无法完成写入。
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("关闭 multipart 写入器失败: %v", closeErr)
	}
	return body, writer.FormDataContentType()
}

// TestCardImageMultipartCreateAndPreview 验证拖拽图片与卡密表单共同提交后自动保存内部引用并可鉴权预览。
func TestCardImageMultipartCreateAndPreview(t *testing.T) {
	// server、cleanup 保存带临时受管图片目录的测试服务及资源释放函数。
	server, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是包含新旧卡券路由和认证中间件的 HTTP 处理器。
	handler := server.Router()
	// sessionCookie 是管理员登录后的认证 Cookie。
	sessionCookie := loginHelper(t, handler)
	// payload 是图片卡券的具名 JSON 表单字段。
	payload := `{"name":"拖拽教程图","type":"image","description":"自动引用","enabled":true,"delay_seconds":0}`
	// body、contentType 保存 multipart 请求体和带边界的 Content-Type。
	body, contentType := buildCardImageMultipart(t, payload, cardImagePNG(t))
	// createRequest 是通过版本化入口提交的拖拽图片卡券请求。
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/cards", body)
	createRequest.Header.Set("Content-Type", contentType)
	createRequest.AddCookie(sessionCookie)
	// createResponse 捕获创建状态和卡券标识。
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("创建图片卡密失败 status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	// result 是创建接口返回的具名标识响应。
	var result mutationIDResponse
	// decodeErr 表示创建响应无法解码为具名卡券标识 DTO。
	if decodeErr := json.Unmarshal(createResponse.Body.Bytes(), &result); decodeErr != nil || result.ID <= 0 {
		t.Fatalf("解析创建响应失败 result=%+v err=%v", result, decodeErr)
	}
	// detailRequest 读取新卡券，确认数据库保存的是内部引用而不是浏览器临时 URL。
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+strconv.FormatInt(result.ID, 10), nil)
	detailRequest.AddCookie(sessionCookie)
	// detailResponse 捕获卡券详情 DTO。
	detailResponse := httptest.NewRecorder()
	handler.ServeHTTP(detailResponse, detailRequest)
	// detail 是图片卡券详情响应。
	var detail cardResponse
	// decodeErr 表示卡券详情无法解码或未返回受管图片引用。
	if decodeErr := json.Unmarshal(detailResponse.Body.Bytes(), &detail); decodeErr != nil || !strings.HasPrefix(detail.ImageURL, "card-image://") {
		t.Fatalf("卡券未自动保存受管引用 detail=%+v err=%v", detail, decodeErr)
	}
	// previewRequest 通过鉴权图片端点读取受管文件。
	previewRequest := httptest.NewRequest(http.MethodGet, "/api/v1/cards/"+strconv.FormatInt(result.ID, 10)+"/image", nil)
	previewRequest.AddCookie(sessionCookie)
	// previewResponse 捕获真实图片字节和安全响应头。
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || previewResponse.Header().Get("Content-Type") != "image/png" || !bytes.Equal(previewResponse.Body.Bytes(), cardImagePNG(t)) {
		t.Fatalf("受管图片预览异常 status=%d type=%q bytes=%d", previewResponse.Code, previewResponse.Header().Get("Content-Type"), previewResponse.Body.Len())
	}
}

// TestCardImageMultipartRejectsFakeImage 验证 multipart 文件名和声明不能让普通文本绕过真实图片魔数检查。
func TestCardImageMultipartRejectsFakeImage(t *testing.T) {
	// server、cleanup 保存测试 HTTP 服务和资源释放函数。
	server, _, cleanup := newTestServer(t)
	defer cleanup()
	// handler 是包含卡券路由和认证中间件的测试处理器。
	handler := server.Router()
	// sessionCookie 是管理员登录后提交伪图片请求使用的认证 Cookie。
	sessionCookie := loginHelper(t, handler)
	// body、contentType 保存伪图片 multipart 请求。
	body, contentType := buildCardImageMultipart(t, `{"name":"伪图片","type":"image","enabled":true}`, []byte("not an image"))
	// request 是携带伪图片内容的创建请求。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cards", body)
	request.Header.Set("Content-Type", contentType)
	request.AddCookie(sessionCookie)
	// response 捕获统一校验错误。
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "只支持 PNG、JPEG 或 WebP 图片") {
		t.Fatalf("伪图片应返回稳定 400 status=%d body=%s", response.Code, response.Body.String())
	}
}
