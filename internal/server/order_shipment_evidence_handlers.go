package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/auth"
)

const (
	// shipmentMultipartMaxBytes 限制三张图片和 multipart 开销，防止聊天页面上传无限请求体。
	shipmentMultipartMaxBytes = 10 << 20
	// shipmentImageMaxBytes 与官方工作台单图严格小于 3MB 的限制一致。
	shipmentImageMaxBytes = 3 << 20
)

// shipmentEvidenceResponseDTO 是无需寄件接口的具名成功响应。
type shipmentEvidenceResponseDTO struct {
	// Success 只在闲鱼明确确认发货时为真。
	Success bool `json:"success"`
	// Status 是 succeeded、succeeded_with_warning、failed 或 needs_review。
	Status string `json:"status"`
	// Message 是不含凭证和收货信息的结果说明。
	Message string `json:"message"`
	// OrderID 是本次操作对应的平台订单号。
	OrderID string `json:"order_id"`
}

// shipmentProofResponseDTO 是订单页展示的 ERP 发货凭证只读响应。
type shipmentProofResponseDTO struct {
	// Success 表示凭证读取完成。
	Success bool `json:"success"`
	// OrderID 是平台订单标识。
	OrderID string `json:"order_id"`
	// AccountID 是执行 ERP 发货的卖家账号。
	AccountID string `json:"account_id"`
	// TradeText 是最终提交给闲鱼的发货描述。
	TradeText string `json:"trade_text"`
	// ImageURLs 是经过服务端媒体地址归一化的官方图片。
	ImageURLs []string `json:"image_urls"`
	// Source 当前固定为 erp。
	Source string `json:"source"`
	// SubmittedAt 是平台明确发货成功时的 Unix 秒。
	SubmittedAt int64 `json:"submitted_at"`
}

// shipmentEvidenceRequestDTO 是无图片发货使用的 JSON 请求；带图片客户端继续使用 multipart。
type shipmentEvidenceRequestDTO struct {
	// AccountID 是付款卡片所属卖家账号。
	AccountID string `json:"account_id"`
	// TradeText 是用户确认的最多 200 字交易说明。
	TradeText string `json:"trade_text"`
}

// shipOrderWithEvidence 解析用户最终确认的 multipart 表单并调用凭证发货应用服务。
func (s *Server) shipOrderWithEvidence(w http.ResponseWriter, r *http.Request) {
	// orderID 是路由中待发货的数字订单号。
	orderID := strings.TrimSpace(chi.URLParam(r, "order_id"))
	if orderID == "" {
		writeErr(w, http.StatusBadRequest, "缺少订单 ID")
		return
	}
	// input、images、parseErr 是按媒体类型解析的具名字段、内存图片和格式错误。
	input, images, parseErr := parseShipmentEvidenceRequest(w, r)
	if parseErr != nil {
		writeErr(w, http.StatusBadRequest, parseErr.Error())
		return
	}
	// session 是当前已认证用户会话。
	session := auth.SessionFromContext(r.Context())
	// result、shipErr 是应用层确定性发货结果和错误。
	result, shipErr := s.orders().ShipWithEvidence(r.Context(), orderapp.ShipmentEvidenceRequest{
		UserID: session.UserID, AccountID: strings.TrimSpace(input.AccountID), OrderID: orderID,
		TradeText: input.TradeText, Images: images,
	})
	if shipErr != nil {
		switch {
		case errors.Is(shipErr, orderapp.ErrForbidden):
			writeErr(w, http.StatusForbidden, "无权操作此订单")
		case errors.Is(shipErr, orderapp.ErrShipmentNotEligible), errors.Is(shipErr, orderapp.ErrShipmentAlreadyHandled), errors.Is(shipErr, orderapp.ErrShipmentNeedsReview):
			writeErr(w, http.StatusConflict, shipErr.Error())
		case errors.Is(shipErr, orderapp.ErrShipmentStatusTimeout):
			writeErr(w, http.StatusGatewayTimeout, orderapp.ErrShipmentStatusTimeout.Error())
		case errors.Is(shipErr, orderapp.ErrShipmentUnavailable):
			writeErr(w, http.StatusServiceUnavailable, shipErr.Error())
		default:
			// validationErr、isValidation 表示应用层输入错误，不应映射为平台故障。
			var validationErr *orderapp.ValidationError
			if errors.As(shipErr, &validationErr) {
				writeErr(w, http.StatusBadRequest, validationErr.Error())
			} else {
				writeErr(w, http.StatusBadGateway, "闲鱼发货请求失败: "+shipErr.Error())
			}
		}
		return
	}
	writeJSON(w, http.StatusOK, shipmentEvidenceResponseDTO{Success: result.Success, Status: result.Status, Message: result.Message, OrderID: result.OrderID})
}

// getOrderShipmentProof 只读取当前用户订单已经保存的 ERP 发货凭证。
func (s *Server) getOrderShipmentProof(w http.ResponseWriter, r *http.Request) {
	// orderID 是路由中的平台订单标识。
	orderID := strings.TrimSpace(chi.URLParam(r, "order_id"))
	// session 是当前已认证用户会话。
	session := auth.SessionFromContext(r.Context())
	// result、readErr 是应用层归属校验后的凭证结果和错误。
	result, readErr := s.orders().ShipmentProof(r.Context(), session.UserID, orderID)
	if readErr != nil {
		if errors.Is(readErr, orderapp.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "该订单不是通过 ERP 发货或未保存凭证")
		} else {
			writeErr(w, http.StatusInternalServerError, "读取发货凭证失败")
		}
		return
	}
	// imageURLs 保存经过受控远程媒体地址归一化的凭证图片。
	imageURLs := make([]string, 0, len(result.Proof.ImageURLs))
	for _, imageURL := range result.Proof.ImageURLs { // imageURL 是当前待归一化的官方图片地址。
		if normalizedURL := normalizeRemoteMediaURL(imageURL); normalizedURL != "" { // normalizedURL 是可公开返回的 HTTP(S) 媒体地址。
			imageURLs = append(imageURLs, normalizedURL)
		}
	}
	writeJSON(w, http.StatusOK, shipmentProofResponseDTO{Success: true, OrderID: result.Proof.OrderID, AccountID: result.Proof.AccountID, TradeText: result.Proof.TradeText, ImageURLs: imageURLs, Source: result.Proof.Source, SubmittedAt: result.Proof.SubmittedAt})
}

// parseShipmentEvidenceRequest 使用 JSON 处理无图片请求，只在确有图片时读取 multipart 上传体。
func parseShipmentEvidenceRequest(w http.ResponseWriter, r *http.Request) (shipmentEvidenceRequestDTO, []orderapp.ShipmentEvidenceImage, error) {
	if r == nil {
		return shipmentEvidenceRequestDTO{}, nil, errors.New("发货请求不能为空")
	}
	r.Body = http.MaxBytesReader(w, r.Body, shipmentMultipartMaxBytes)
	// mediaType、_, mediaErr 是去除 boundary 后的请求媒体类型和解析错误。
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil {
		return shipmentEvidenceRequestDTO{}, nil, errors.New("发货表单格式错误")
	}
	switch mediaType {
	case "application/json":
		// input 是无图片发货的具名 JSON 请求。
		var input shipmentEvidenceRequestDTO
		// decoder 只接受当前 DTO 字段，避免客户端偷偷提交图片 URL 或平台参数。
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if decodeErr := decoder.Decode(&input); decodeErr != nil { // decodeErr 是 JSON 请求体格式错误。
			return shipmentEvidenceRequestDTO{}, nil, errors.New("发货表单格式错误")
		}
		return input, nil, nil
	case "multipart/form-data":
		if parseErr := r.ParseMultipartForm(shipmentMultipartMaxBytes); parseErr != nil { // parseErr 是 multipart 解析或总请求体超限错误。
			return shipmentEvidenceRequestDTO{}, nil, errors.New("发货表单过大或格式错误")
		}
		// images、imageErr 是经过数量、大小和真实 MIME 检查的内存凭证。
		images, imageErr := shipmentEvidenceImages(r)
		if imageErr != nil {
			return shipmentEvidenceRequestDTO{}, nil, imageErr
		}
		return shipmentEvidenceRequestDTO{AccountID: r.FormValue("account_id"), TradeText: r.FormValue("trade_text")}, images, nil
	default:
		return shipmentEvidenceRequestDTO{}, nil, errors.New("发货请求只支持 JSON 或 multipart 表单")
	}
}

// shipmentEvidenceImages 从 multipart 的 images 字段读取最多三张 PNG/JPEG；字节只在当前请求内存中存在。
func shipmentEvidenceImages(r *http.Request) ([]orderapp.ShipmentEvidenceImage, error) {
	if r == nil || r.MultipartForm == nil {
		return nil, nil
	}
	// headers 是浏览器按用户确认顺序上传的图片文件头。
	headers := r.MultipartForm.File["images"]
	if len(headers) > 3 {
		return nil, errors.New("相关凭证最多上传 3 张")
	}
	// images 保存通过真实文件头检测的内存凭证。
	images := make([]orderapp.ShipmentEvidenceImage, 0, len(headers))
	// header 是当前待读取的 multipart 图片头。
	for _, header := range headers {
		// file、openErr 是当前临时 multipart 文件和打开错误。
		file, openErr := header.Open()
		if openErr != nil {
			return nil, errors.New("读取相关凭证失败")
		}
		// data、readErr 是严格限制长度后的图片字节和读取错误。
		data, readErr := io.ReadAll(io.LimitReader(file, shipmentImageMaxBytes))
		// closeErr 是当前 multipart 临时文件关闭错误。
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.New("读取相关凭证失败")
		}
		if len(data) == 0 || len(data) >= shipmentImageMaxBytes {
			return nil, errors.New("每张相关凭证必须小于 3MB")
		}
		// detectedType 是根据文件魔数识别的真实 MIME，不能信任客户端 Content-Type。
		detectedType := http.DetectContentType(data)
		if detectedType != "image/png" && detectedType != "image/jpeg" {
			return nil, errors.New("相关凭证只支持 PNG 或 JPEG 图片")
		}
		images = append(images, orderapp.ShipmentEvidenceImage{Filename: header.Filename, ContentType: detectedType, Data: data})
	}
	return images, nil
}
