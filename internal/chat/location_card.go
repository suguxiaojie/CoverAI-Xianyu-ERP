package chat

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// locationCardContent 是聊天存储使用的稳定位置卡片 JSON，不保存可变平台跳转地址或凭证。
type locationCardContent struct {
	// Title 是发送者设置的位置名称。
	Title string `json:"title"`
	// Description 是地址或到店说明。
	Description string `json:"description"`
	// Latitude 是 WGS84 纬度十进制度。
	Latitude float64 `json:"latitude"`
	// Longitude 是 WGS84 经度十进制度。
	Longitude float64 `json:"longitude"`
}

// normalizeLocationCardContent 从 contentType=30 的 locationCard 对象提取稳定非敏感展示字段。
func normalizeLocationCardContent(value any) (string, bool) {
	// card 是协议 locationCard 对象；字符串包装会由 mapValue 兼容解码。
	card := mapValue(value)
	// title、description 是用户在卡片中可见的标题和说明。
	title, description := strings.TrimSpace(cleanNilString(card["title"])), strings.TrimSpace(cleanNilString(card["content"]))
	// latitude、latitudeErr 是纬度文本解析值及格式错误。
	latitude, latitudeErr := strconv.ParseFloat(strings.TrimSpace(cleanNilString(card["latitude"])), 64)
	// longitude、longitudeErr 是经度文本解析值及格式错误。
	longitude, longitudeErr := strconv.ParseFloat(strings.TrimSpace(cleanNilString(card["longitude"])), 64)
	if title == "" || description == "" || latitudeErr != nil || longitudeErr != nil ||
		math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return "", false
	}
	// encoded、encodeErr 是字段顺序稳定的本地 JSON 及编码错误。
	encoded, encodeErr := json.Marshal(locationCardContent{Title: title, Description: description, Latitude: latitude, Longitude: longitude})
	if encodeErr != nil {
		return "", false
	}
	return string(encoded), true
}

// locationCardSummary 返回联系人列表使用的位置摘要；损坏正文保留通用占位。
func locationCardSummary(content string) string {
	// card、decodeErr 是本地位置正文及 JSON 解码错误。
	var card locationCardContent
	// decodeErr 是联系人摘要无法读取结构化位置正文时的 JSON 错误。
	decodeErr := json.Unmarshal([]byte(strings.TrimSpace(content)), &card)
	if decodeErr != nil || strings.TrimSpace(card.Title) == "" {
		return "[位置]"
	}
	return "[位置] " + strings.TrimSpace(card.Title)
}
