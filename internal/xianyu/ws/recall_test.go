package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestSendWithResultAndRecallFeature 验证平台 PNM ID 回传和官方 Feature 撤回请求格式。
func TestSendWithResultAndRecallFeature(t *testing.T) {
	// requests 保存服务端收到的发送与撤回请求，供严格字段断言。
	requests := make(chan map[string]any, 2)
	// server 模拟闲鱼 WS 请求响应关联，不访问真实平台。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// socket、acceptErr 保存本地 WebSocket 和升级错误。
		socket, acceptErr := websocket.Accept(writer, request, nil)
		if acceptErr != nil {
			return
		}
		defer socket.Close(websocket.StatusNormalClosure, "")
		for {
			// _, payload、readErr 保存客户端发送的单帧请求。
			_, payload, readErr := socket.Read(request.Context())
			if readErr != nil {
				return
			}
			// frame 保存解码后的请求帧。
			var frame map[string]any
			if json.Unmarshal(payload, &frame) != nil {
				continue
			}
			requests <- frame
			// headers 保存必须原样回传的 mid 关联字段。
			headers, _ := frame["headers"].(map[string]any)
			// body 默认为空；发送请求返回平台消息标识。
			var body any = map[string]any{}
			if frame["lwp"] == "/r/MessageSend/sendByReceiverScope" {
				body = map[string]any{"messageId": "platform-1.PNM", "createAt": float64(123456)}
			}
			// response 是与请求 mid 关联的成功响应。
			response, _ := json.Marshal(map[string]any{"code": 200, "headers": headers, "body": body})
			_ = socket.Write(request.Context(), websocket.MessageText, response)
		}
	}))
	t.Cleanup(server.Close)
	// connection 是连接本地服务的协议客户端。
	connection := dialLocal(t, server, Config{})
	// ctx、cancel 为两次请求提供统一测试上限。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// result、sendErr 保存带平台结果的文本发送响应。
	result, sendErr := connection.SendTextWithResult(ctx, "100", "chat-1", "200", "测试")
	if sendErr != nil || result.PlatformMessageID != "platform-1.PNM" || result.CreatedAt != 123456 {
		t.Fatalf("send result=%+v err=%v", result, sendErr)
	}
	// sendFrame 是服务端观察到的发送帧。
	sendFrame := <-requests
	if sendFrame["lwp"] != "/r/MessageSend/sendByReceiverScope" {
		t.Fatalf("send path=%v", sendFrame["lwp"])
	}
	// recallErr 保存官方 Feature 撤回响应。
	recallErr := connection.RecallMessageByFeature(ctx, result.PlatformMessageID, RecallFeature{OperatorID: "100", OriginContentType: 1, TextContent: "测试"})
	if recallErr != nil {
		t.Fatalf("RecallMessageByFeature() error=%v", recallErr)
	}
	// recallFrame 是服务端观察到的撤回帧。
	recallFrame := <-requests
	if recallFrame["lwp"] != "/r/MessageManager/recallMessageByFeature" {
		t.Fatalf("recall path=%v", recallFrame["lwp"])
	}
	// recallBody 保存官方三参数撤回正文。
	recallBody, _ := recallFrame["body"].([]any)
	if len(recallBody) != 3 || recallBody[1] != "platform-1.PNM" {
		t.Fatalf("recall body=%#v", recallBody)
	}
	// feature 保存发送者撤回展示元数据。
	feature, _ := recallBody[2].(map[string]any)
	if feature["operatorType"] != float64(0) || feature["operatorUid"] != "100" || feature["showRecallStatusSetting"] != float64(1) {
		t.Fatalf("recall feature=%#v", feature)
	}
}
