package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"xianyu-go/internal/db"
)

// TestChatSessionSearchEndpointFindsHistoricalMessageWithoutReturningContent 验证认证搜索只返回匹配会话摘要。
func TestChatSessionSearchEndpointFindsHistoricalMessageWithoutReturningContent(t *testing.T) {
	// server、store、cleanup 是启用聊天应用服务的 HTTP 测试组合。
	server, store, cleanup := newTestServerWithChat(t)
	defer cleanup()
	// ctx 是预置会话和消息使用的本地上下文。
	ctx := context.Background()
	// session 是当前账号中需要通过旧消息定位的测试会话。
	session := db.ChatSession{CookieID: "acc1", ChatID: "search-chat", BuyerID: "buyer-search", BuyerName: "历史买家", ItemTitle: "历史商品"}
	// historical 是不再属于会话摘要的旧卡密消息。
	historical := db.ChatMessage{MessageKey: "search-history.PNM", Direction: "outgoing", SenderID: "acc1", MessageType: "text", Content: "TEST-CARD-CODE-001", Status: "sent", SentAt: 1000}
	if // _, _, saveErr 是保存历史消息的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, historical, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// latest 是覆盖会话最新摘要的后续消息。
	latest := db.ChatMessage{MessageKey: "search-latest.PNM", Direction: "incoming", SenderID: "buyer-search", MessageType: "text", Content: "地址在这里", Status: "received", SentAt: 2000}
	if // _, _, saveErr 是保存最新消息的结果和错误。
	_, _, saveErr := store.Chats.SaveMessage(ctx, session, latest, false); saveErr != nil {
		t.Fatal(saveErr)
	}
	// handler 是包含版本化聊天搜索的完整认证路由。
	handler := server.Router()
	// sessionCookie 是管理员登录后得到的认证会话。
	sessionCookie := loginHelper(t, handler)
	// searchPath 是携带 URL 编码历史卡密和 refresh=1 的版本化查询；搜索分支不得触发平台刷新。
	searchPath := "/api/v1/chat/sessions?account_id=acc1&refresh=1&search=" + url.QueryEscape("TEST-CARD-CODE-001")
	// searchRequest 是携带认证 Cookie 的历史搜索请求。
	searchRequest := httptest.NewRequest(http.MethodGet, searchPath, nil)
	searchRequest.AddCookie(sessionCookie)
	// searchRecorder 捕获只包含匹配会话摘要的响应。
	searchRecorder := httptest.NewRecorder()
	handler.ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK || !strings.Contains(searchRecorder.Body.String(), `"chat_id":"search-chat"`) || !strings.Contains(searchRecorder.Body.String(), `"last_message":"地址在这里"`) || strings.Contains(searchRecorder.Body.String(), "TEST-CARD-CODE-001") {
		t.Fatalf("status=%d body=%s", searchRecorder.Code, searchRecorder.Body.String())
	}
	// foreignRequest 验证其他账号不能借搜索读取当前会话。
	foreignRequest := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions?account_id=missing&search=PLUS", nil)
	foreignRequest.AddCookie(sessionCookie)
	// foreignRecorder 捕获账号归属拒绝响应。
	foreignRecorder := httptest.NewRecorder()
	handler.ServeHTTP(foreignRecorder, foreignRequest)
	if foreignRecorder.Code != http.StatusForbidden {
		t.Fatalf("foreign status=%d body=%s", foreignRecorder.Code, foreignRecorder.Body.String())
	}
	// longRequest 验证超长搜索词在进入仓储前返回 400。
	longRequest := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions?account_id=acc1&search="+url.QueryEscape(strings.Repeat("长", 201)), nil)
	longRequest.AddCookie(sessionCookie)
	// longRecorder 捕获超长关键词的参数错误响应。
	longRecorder := httptest.NewRecorder()
	handler.ServeHTTP(longRecorder, longRequest)
	if longRecorder.Code != http.StatusBadRequest {
		t.Fatalf("long status=%d body=%s", longRecorder.Code, longRecorder.Body.String())
	}
}
