package mtop

import "context"

// ShipmentEvidenceImage 描述一张待上传的无需寄件凭证图片。
type ShipmentEvidenceImage struct {
	// Filename 是 multipart 使用的本地展示文件名，不进入日志。
	Filename string
	// ContentType 是已经校验的 PNG 或 JPEG MIME。
	ContentType string
	// Data 是仅在当前请求内存中存在的图片字节。
	Data []byte
}

// ShipmentEvidenceUpload 是发货凭证上传后可提交给 picList 的安全结果。
type ShipmentEvidenceUpload struct {
	// URL 是官方上传响应 object.url。
	URL string
	// UpdatedCookies 是上传响应可能更新后的兼容平面 Cookie。
	UpdatedCookies string
}

// UploadShipmentEvidenceImage 使用官方 fleamarket 作用域上传一张发货凭证。
func (c *ClientImpl) UploadShipmentEvidenceImage(ctx context.Context, cookiesStr string, image ShipmentEvidenceImage) (*ShipmentEvidenceUpload, error) {
	// uploaded、updatedCookies、uploadErr 是通用上传器返回的图片和凭证变化。
	uploaded, updatedCookies, uploadErr := c.uploadScopedImage(ctx, cookiesStr, PublishImage{
		Filename: image.Filename, ContentType: image.ContentType, Data: image.Data,
	}, "fleamarket", "发货凭证")
	if uploadErr != nil {
		return nil, uploadErr
	}
	return &ShipmentEvidenceUpload{URL: uploaded.URL, UpdatedCookies: updatedCookies}, nil
}
