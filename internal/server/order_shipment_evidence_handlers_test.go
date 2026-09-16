package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// TestShipOrderWithEvidenceUsesVersionedRouteAndMultipart 验证认证路由把内存图片交给应用服务并完成本地确定性收口。
func TestShipOrderWithEvidenceUsesVersionedRouteAndMultipart(t *testing.T) {
	// server、store、cleanup 是完整测试组合根、SQLite 仓储和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是建立卖家付款卡片和订单夹具的上下文。
	ctx := context.Background()
	// orderID 是当前测试使用的数字订单号。
	orderID := "5127372002248048713"
	// _, _, messageErr 是卖家付款卡片的幂等保存结果。
	_, _, messageErr := store.Chats.SaveMessage(ctx, db.ChatSession{CookieID: "acc1", ChatID: "seller-chat", BuyerID: "buyer-account"}, db.ChatMessage{
		MessageKey: "paid-seller.PNM", Direction: "incoming", SenderID: "buyer-account", SenderName: "买家",
		MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: time.Now().UTC().UnixMilli(),
		PlatformContentType: 26, SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardTitle: "我已付款，等待你发货",
		SystemCardOrderID: orderID, SystemCardItemID: "item-1", SystemCardAction: "ship_order",
	}, false)
	if messageErr != nil {
		t.Fatal(messageErr)
	}
	// orderErr 是卖家订单夹具写入失败的原因。
	if orderErr := store.Orders.Upsert(ctx, orderID, db.OrderUpsertOpts{CookieID: "acc1", BuyerID: "buyer-account", ItemID: "item-1", ChatID: "seller-chat", OrderStatus: "pending_ship", Amount: "0.01"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	// platformCalls 记录状态复核、上传和最终发货三个本地拦截请求。
	platformCalls := make(map[string]int)
	// client 使用本地 RoundTripper 模拟官方三类响应，不访问真实闲鱼。
	client := mtop.NewClient()
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		// responseBody 是当前请求对应的官方最小成功响应。
		responseBody := `{"ret":["SUCCESS::调用成功"]}`
		switch {
		case strings.Contains(request.URL.Host, "stream-upload"):
			platformCalls["upload"]++
			if request.URL.Query().Get("appkey") != "fleamarket" {
				t.Fatalf("upload url=%s", request.URL.String())
			}
			responseBody = `{"success":true,"object":{"url":"https://img.example/proof.png","pix":"10x20"}}`
		case request.URL.Query().Get("api") == "mtop.idle.web.trade.order.detail":
			platformCalls["detail"]++
			responseBody = `{"ret":["SUCCESS::调用成功"],"data":{"utArgs":{"orderStatus":"2"},"components":[]}}`
		case request.URL.Query().Get("api") == "mtop.taobao.idle.logistics.merchant.consign.dummy":
			platformCalls["consign"]++
			responseBody = `{"ret":["SUCCESS::调用成功"],"data":{"orderId":"5127372002248048713"}}`
		default:
			t.Fatalf("unexpected platform request: %s", request.URL.String())
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody)), Request: request}, nil
	})}
	setTestMTop(server, client)
	// body 和 writer 构造与浏览器 FormData 一致的 multipart 请求。
	var body bytes.Buffer
	// writer 是当前 multipart 请求体编码器。
	writer := multipart.NewWriter(&body)
	// formErr 是卖家账号字段写入错误。
	if formErr := writer.WriteField("account_id", "acc1"); formErr != nil {
		t.Fatal(formErr)
	}
	// formErr 是相关描述字段写入错误。
	if formErr := writer.WriteField("trade_text", "在线交付完成"); formErr != nil {
		t.Fatal(formErr)
	}
	// filePart、partErr 是相关凭证的 multipart file 字段。
	filePart, partErr := writer.CreateFormFile("images", "proof.png")
	if partErr != nil {
		t.Fatal(partErr)
	}
	// pngHeader 是足以让服务端魔数检测识别 PNG 的固定签名。
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	// writeErr 是 PNG 测试字节写入 multipart 的错误。
	if _, writeErr := filePart.Write(pngHeader); writeErr != nil {
		t.Fatal(writeErr)
	}
	// closeErr 是 multipart 尾部写入错误。
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	// handler 和 sessionCookie 是完整认证路由及管理员会话。
	handler := server.Router()
	// sessionCookie 是管理员登录后得到的认证会话。
	sessionCookie := loginHelper(t, handler)
	// request 是用户二次确认后的版本化发货请求。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID+"/ship-with-evidence", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.AddCookie(sessionCookie)
	// recorder 捕获具名成功响应。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s calls=%v", recorder.Code, recorder.Body.String(), platformCalls)
	}
	// response 是无需寄件 HTTP 结果。
	var response shipmentEvidenceResponseDTO
	// decodeErr 是具名响应反序列化错误。
	if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &response); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !response.Success || response.OrderID != orderID || platformCalls["detail"] != 1 || platformCalls["upload"] != 1 || platformCalls["consign"] != 1 {
		t.Fatalf("response=%+v calls=%v", response, platformCalls)
	}
	// order 是平台成功后由应用服务收口的本地订单。
	order, readErr := store.Orders.Get(ctx, orderID)
	if readErr != nil || db.NormalizeOrderStatus(order.OrderStatus) != "shipped" || !order.SystemShipped || order.ShippedAt == "" {
		t.Fatalf("order=%+v err=%v", order, readErr)
	}
}

// TestShipOrderWithEvidenceReportsPreflightTimeout 验证订单状态复核截止返回 504 并明确尚未执行最终发货。
func TestShipOrderWithEvidenceReportsPreflightTimeout(t *testing.T) {
	// server、store、cleanup 是超时路由测试的完整组合根、SQLite 仓储和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是建立卖家付款卡片和待发货订单夹具的上下文。
	ctx := context.Background()
	// orderID 是当前超时场景使用的数字订单号。
	orderID := "5127194847698062342"
	// _, _, messageErr 是卖家付款卡片的幂等保存结果。
	_, _, messageErr := store.Chats.SaveMessage(ctx, db.ChatSession{CookieID: "acc1", ChatID: "seller-timeout", BuyerID: "buyer-account"}, db.ChatMessage{
		MessageKey: "paid-timeout.PNM", Direction: "incoming", SenderID: "buyer-account", MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: time.Now().UTC().UnixMilli(),
		PlatformContentType: 26, SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardTitle: "我已付款，等待你发货", SystemCardOrderID: orderID, SystemCardAction: "ship_order",
	}, false)
	if messageErr != nil {
		t.Fatal(messageErr)
	}
	// orderErr 是待发货订单夹具写入失败的原因。
	if orderErr := store.Orders.Upsert(ctx, orderID, db.OrderUpsertOpts{CookieID: "acc1", BuyerID: "buyer-account", ChatID: "seller-timeout", OrderStatus: "pending_ship"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	// client 在订单详情复核阶段立即返回上下文截止，不会触发真实平台请求。
	client := mtop.NewClient()
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}
	setTestMTop(server, client)
	// handler 是包含版本化发货端点的完整认证路由。
	handler := server.Router()
	// sessionCookie 是管理员登录后得到的认证会话。
	sessionCookie := loginHelper(t, handler)
	// request 是无图片客户端使用 JSON 提交的最终确认请求。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID+"/ship-with-evidence", strings.NewReader(`{"account_id":"acc1","trade_text":"无需寄件"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(sessionCookie)
	// recorder 捕获状态复核截止后的稳定错误响应。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusGatewayTimeout || !strings.Contains(recorder.Body.String(), "尚未执行发货") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	// runCount 验证状态复核失败发生在最终发货幂等 Claim 之前。
	var runCount int
	if countErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_task_runs WHERE task_type='order_ship_with_evidence' AND target_id=?`, orderID).Scan(&runCount); countErr != nil || runCount != 0 { // countErr 是运行记录计数错误。
		t.Fatalf("runCount=%d err=%v", runCount, countErr)
	}
}

// TestShipmentEvidenceImagesRejectsNonImageMagic 验证 multipart Content-Type 不能绕过真实图片魔数检查。
func TestShipmentEvidenceImagesRejectsNonImageMagic(t *testing.T) {
	// body 和 writer 构造伪装为 PNG 的文本文件。
	var body bytes.Buffer
	// writer 是伪图片 multipart 请求体编码器。
	writer := multipart.NewWriter(&body)
	// header 是显式伪造 image/png 的 multipart 头。
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="images"; filename="fake.png"`)
	header.Set("Content-Type", "image/png")
	// part、partErr 是伪图片内容写入目标。
	part, partErr := writer.CreatePart(header)
	if partErr != nil {
		t.Fatal(partErr)
	}
	_, _ = fmt.Fprint(part, "not-an-image")
	_ = writer.Close()
	// request 只用于调用 multipart 解析辅助函数。
	request := httptest.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	// parseErr 是伪图片 multipart 解析错误。
	if parseErr := request.ParseMultipartForm(shipmentMultipartMaxBytes); parseErr != nil {
		t.Fatal(parseErr)
	}
	// imageErr 是伪图片必须触发的魔数验证错误。
	if _, imageErr := shipmentEvidenceImages(request); imageErr == nil {
		t.Fatal("伪造 MIME 的文本文件不应通过图片凭证检查")
	}
}

// TestGetOrderShipmentProofReturnsOwnedERPEvidence 验证只读路由返回本地凭证并拒绝没有记录的订单。
func TestGetOrderShipmentProofReturnsOwnedERPEvidence(t *testing.T) {
	// server、store、cleanup 是凭证读取路由使用的完整测试组合根和释放函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是订单和凭证夹具写入上下文。
	ctx := context.Background()
	// orderID 是当前有 ERP 凭证的订单标识。
	orderID := "5127372002248048713"
	if orderErr := store.Orders.Upsert(ctx, orderID, db.OrderUpsertOpts{CookieID: "acc1", BuyerID: "buyer", OrderStatus: "shipped"}); orderErr != nil { // orderErr 是订单夹具写入错误。
		t.Fatal(orderErr)
	}
	if proofErr := store.OrderShipmentProofs.Save(ctx, db.OrderShipmentProof{OrderID: orderID, CookieID: "acc1", TradeText: "在线交付完成", ImageURLsJSON: `["https://img.example/proof.png"]`, Source: "erp", SubmittedAt: 100}); proofErr != nil { // proofErr 是凭证夹具写入错误。
		t.Fatal(proofErr)
	}
	// handler 是认证后的订单路由。
	handler := server.Router()
	// sessionCookie 是管理员登录会话。
	sessionCookie := loginHelper(t, handler)
	// request 是当前凭证 GET。
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID+"/shipment-proof", nil)
	request.AddCookie(sessionCookie)
	// recorder 是当前凭证响应记录器。
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	// response 是凭证路由返回的具名 JSON。
	var response shipmentProofResponseDTO
	if decodeErr := json.Unmarshal(recorder.Body.Bytes(), &response); decodeErr != nil || response.OrderID != orderID || response.TradeText != "在线交付完成" || len(response.ImageURLs) != 1 { // decodeErr 是响应 JSON 解析错误。
		t.Fatalf("response=%+v err=%v", response, decodeErr)
	}
	// missingRequest 验证没有 ERP 记录时明确返回 404。
	missingRequest := httptest.NewRequest(http.MethodGet, "/api/v1/orders/3316370000000000000/shipment-proof", nil)
	missingRequest.AddCookie(sessionCookie)
	// missingRecorder 保存缺失凭证响应。
	missingRecorder := httptest.NewRecorder()
	handler.ServeHTTP(missingRecorder, missingRequest)
	if missingRecorder.Code != http.StatusNotFound || !strings.Contains(missingRecorder.Body.String(), "不是通过 ERP 发货") {
		t.Fatalf("missing status=%d body=%s", missingRecorder.Code, missingRecorder.Body.String())
	}
}
