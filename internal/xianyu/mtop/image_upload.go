package mtop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// uploadScopedImage 使用调用方明确指定的闲鱼上传作用域发送图片；响应只投影 URL、尺寸和 Cookie 变化。
func (c *ClientImpl) uploadScopedImage(ctx context.Context, cookiesStr string, img PublishImage, appKey, purpose string) (uploadedImage, string, error) {
	// hc 用于本次流程后续判断的hc
	hc := c.httpClientWithTimeout(60 * time.Second)
	// uploadAppKey 是官方上传端点使用的业务作用域；空值不得退化为不明确请求。
	uploadAppKey := strings.TrimSpace(appKey)
	if uploadAppKey == "" {
		return uploadedImage{}, cookiesStr, errors.New("图片上传 appkey 不能为空")
	}
	// uploadPurpose 是错误消息使用的非敏感业务名称。
	uploadPurpose := strings.TrimSpace(purpose)
	if uploadPurpose == "" {
		uploadPurpose = "图片"
	}
	if img.ContentType == "" {
		img.ContentType = "application/octet-stream"
	}
	if img.Filename == "" {
		img.Filename = "image"
	}
	// body 用于本次流程后续判断的请求体
	var body bytes.Buffer
	// mw 用于本次流程后续判断的mw
	mw := multipart.NewWriter(&body)
	// header 用于本次流程后续判断的header
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf("form-data; name=\"file\"; filename=\"%s\"", escapeMultipartFilename(filepath.Base(img.Filename))))
	header.Set("Content-Type", img.ContentType)
	// part、err 用于本次流程后续判断的part、err
	part, err := mw.CreatePart(header)
	if err != nil {
		return uploadedImage{}, cookiesStr, err
	}
	if // err 用于本次流程后续判断的err
	_, err := part.Write(img.Data); err != nil {
		return uploadedImage{}, cookiesStr, err
	}
	if // err 用于本次流程后续判断的err
	err := mw.Close(); err != nil {
		return uploadedImage{}, cookiesStr, err
	}
	// uploadURL 是带业务作用域的官方上传端点。
	uploadURL, _ := url.Parse(UploadMediaAPI)
	// query 保存上传端点固定拼写的查询参数。
	query := uploadURL.Query()
	query.Set("floderId", "0")
	query.Set("appkey", uploadAppKey)
	query.Set("_input_charset", "utf-8")
	uploadURL.RawQuery = query.Encode()
	// request、requestErr 是当前 multipart 上传请求及构造错误。
	request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL.String(), &body)
	if requestErr != nil {
		return uploadedImage{}, cookiesStr, requestErr
	}
	// documentURL 是 Cookie SameSite 计算使用的官方页面来源；发货凭证来自卖家工作台。
	documentURL := "https://www.goofish.com/"
	if uploadAppKey == "fleamarket" {
		documentURL = "https://seller.goofish.com/"
	}
	// requestCookies 是按当前页面来源和上传目标选择的 Cookie Header。
	_, requestCookies := mtopRequestCookies(ctx, cookiesStr, documentURL, uploadURL.String())
	request.Header.Set("content-type", mw.FormDataContentType())
	setBrowserHeaders(request, requestCookies)
	request.Header.Set("accept", "*/*")
	request.Header.Set("x-requested-with", "XMLHttpRequest")
	if uploadAppKey == "fleamarket" {
		// 发货凭证来自官方卖家工作台，保持其 seller.goofish.com Origin 与 Referer。
		request.Header.Set("origin", "https://seller.goofish.com")
		request.Header.Set("referer", "https://seller.goofish.com/")
	}
	// response、responseErr 是平台上传响应和网络错误。
	response, responseErr := hc.Do(request)
	if responseErr != nil {
		return uploadedImage{}, cookiesStr, fmt.Errorf("上传%s失败: %w", uploadPurpose, responseErr)
	}
	defer response.Body.Close()
	// updatedCookies 是上传响应合并后的 Cookie。
	updatedCookies := absorbMTopResponseCookies(ctx, cookiesStr, response)
	// raw、readErr 是有界响应正文和读取错误。
	raw, readErr := readMTopBody(response)
	if readErr != nil {
		return uploadedImage{}, updatedCookies, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return uploadedImage{}, updatedCookies, fmt.Errorf("上传%s失败: http=%d body=%s", uploadPurpose, response.StatusCode, truncate(string(raw), 240))
	}
	// decoded 是上传响应的 JSON 对象。
	var decoded map[string]any
	if // decodeErr 是上传响应 JSON 解析错误。
	decodeErr := json.Unmarshal(raw, &decoded); decodeErr != nil {
		return uploadedImage{}, updatedCookies, fmt.Errorf("解析图片上传响应失败: %w (body=%s)", decodeErr, truncate(string(raw), 240))
	}
	// object 是官方 object 字段；旧响应可兼容 data 字段。
	object := mapFromAny(decoded["object"])
	if object == nil {
		object = mapFromAny(decoded["data"])
	}
	// imageURL 是最终提交 picList 使用的官方图片地址。
	imageURL := mtopString(object["url"])
	if imageURL == "" {
		imageURL = mtopString(decoded["url"])
	}
	if imageURL == "" {
		return uploadedImage{}, updatedCookies, fmt.Errorf("图片上传响应缺少 url: %s", truncate(string(raw), 240))
	}
	// width、height 是平台 pix 或本地图片头得到的像素尺寸。
	width, height := parsePix(mtopString(object["pix"]))
	if width == 0 || height == 0 {
		if // config、decodeErr 是本地图片尺寸和无法读取图片头的错误。
		config, _, decodeErr := image.DecodeConfig(bytes.NewReader(img.Data)); decodeErr == nil {
			width, height = config.Width, config.Height
		}
	}
	if width == 0 {
		width = 800
	}
	if height == 0 {
		height = 800
	}
	return uploadedImage{URL: imageURL, Width: width, Height: height}, updatedCookies, nil
}
