package server

import (
	"testing"

	chatapp "xianyu-go/internal/application/chat"
)

// TestNormalizeRemoteMediaURL 验证只升级已知阿里 CDN，不改写本地和其他远程地址。
func TestNormalizeRemoteMediaURL(t *testing.T) {
	// cases 覆盖 HTTP、协议相对、已安全、未知域名、本地路径和非法输入。
	cases := []struct {
		// name 是当前协议归一场景的简短名称。
		name string
		// input 是平台或本地返回的原始地址。
		input string
		// want 是传输 DTO 应返回的地址。
		want string
	}{
		{name: "alicdn http", input: " http://gtms03.alicdn.com/avatar.png?x=1 ", want: "https://gtms03.alicdn.com/avatar.png?x=1"},
		{name: "tbcdn http", input: "http://img.tbcdn.cn/item.jpg", want: "https://img.tbcdn.cn/item.jpg"},
		{name: "scheme relative", input: "//img.alicdn.com/avatar.jpg", want: "https://img.alicdn.com/avatar.jpg"},
		{name: "already https", input: "https://img.alicdn.com/avatar.jpg", want: "https://img.alicdn.com/avatar.jpg"},
		{name: "unknown http", input: "http://example.com/image.jpg", want: "http://example.com/image.jpg"},
		{name: "local path", input: "/api/v1/cards/1/image", want: "/api/v1/cards/1/image"},
		{name: "blob URL", input: "blob:http://127.0.0.1/local", want: "blob:http://127.0.0.1/local"},
		{name: "empty", input: "  ", want: ""},
	}
	// testCase 是当前执行的协议归一样例。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// got 是当前样例经过传输边界归一后的地址。
			got := normalizeRemoteMediaURL(testCase.input)
			if got != testCase.want {
				t.Fatalf("normalizeRemoteMediaURL(%q)=%q want %q", testCase.input, got, testCase.want)
			}
		})
	}
}

// TestRemoteMediaDTOConversion 验证聊天 HTTP/WebSocket DTO 对头像和媒体内容使用同一 HTTPS 边界。
func TestRemoteMediaDTOConversion(t *testing.T) {
	// session 是携带历史 HTTP 买家头像的应用层会话。
	session := newChatSessionDTOFromApplication(chatapp.Session{BuyerAvatar: "http://img.alicdn.com/buyer.png"})
	if session.BuyerAvatar != "https://img.alicdn.com/buyer.png" {
		t.Fatalf("buyer avatar=%q", session.BuyerAvatar)
	}
	// imageMessage 是需要升级 CDN 地址的聊天图片。
	imageMessage := newChatMessageDTOFromApplication(&chatapp.Message{MessageType: "image", Content: "http://img.alicdn.com/chat.png"})
	if imageMessage.Content != "https://img.alicdn.com/chat.png" {
		t.Fatalf("image content=%q", imageMessage.Content)
	}
	// textMessage 证明普通文字中的 URL 不会被媒体适配器改写。
	textMessage := newChatMessageDTOFromApplication(&chatapp.Message{MessageType: "text", Content: "http://img.alicdn.com/not-an-image"})
	if textMessage.Content != "http://img.alicdn.com/not-an-image" {
		t.Fatalf("text content=%q", textMessage.Content)
	}
	// itemImage 是商品详情 pic_info 中需要升级的历史 HTTP 主图。
	itemImage := itemImageFromDetail(`{"pic_info":{"picUrl":"http://img.alicdn.com/item.jpg"}}`)
	if itemImage != "https://img.alicdn.com/item.jpg" {
		t.Fatalf("item image=%q", itemImage)
	}
	// fallbackItemImage 是没有 pic_info 时从兼容 item_image 字段取得的主图。
	fallbackItemImage := itemImageFromDetail(`{"item_image":"http://gw.alicdn.com/item.jpg"}`)
	if fallbackItemImage != "https://gw.alicdn.com/item.jpg" {
		t.Fatalf("fallback item image=%q", fallbackItemImage)
	}
}
