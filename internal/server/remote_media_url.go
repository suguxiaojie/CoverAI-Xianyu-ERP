package server

import (
	"net/url"
	"strings"
)

// normalizeRemoteMediaURL 只将已验证支持 HTTPS 的阿里图片 CDN 地址升级为安全协议。
//
// 数据库继续保留平台原始值；该函数只用于 HTTP/WebSocket DTO 输出边界，
// 避免 macOS WKWebView 的 ATS 拦截历史 HTTP 头像和商品图。其他域名不猜测 HTTPS 能力。
func normalizeRemoteMediaURL(raw string) string {
	// trimmed 是去除平台字段首尾空白后的媒体地址。
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	// candidate 将协议相对地址按 HTTPS 解析，便于使用同一域名门禁。
	candidate := trimmed
	if strings.HasPrefix(candidate, "//") {
		candidate = "https:" + candidate
	}
	// parsed 是仅用于协议和主机判定的标准 URL；非绝对地址原样返回。
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Hostname() == "" {
		return trimmed
	}
	// hostname 是不含端口且已转小写的 CDN 主机名。
	hostname := strings.ToLower(parsed.Hostname())
	// alibabaImageCDN 只允许当前数据和平台图片链路使用的阿里 CDN 根域。
	alibabaImageCDN := hostname == "alicdn.com" || strings.HasSuffix(hostname, ".alicdn.com") || hostname == "tbcdn.cn" || strings.HasSuffix(hostname, ".tbcdn.cn")
	if parsed.Scheme == "http" && alibabaImageCDN {
		parsed.Scheme = "https"
		return parsed.String()
	}
	return candidate
}

// normalizeChatMediaContent 仅对图片和视频消息的 CDN 内容地址执行安全协议归一。
func normalizeChatMediaContent(messageType, content string) string {
	if messageType != "image" && messageType != "video" {
		return content
	}
	return normalizeRemoteMediaURL(content)
}
