package server

import (
	"time"

	chatapp "xianyu-go/internal/application/chat"
)

// chatCreditLevelDTO 是聊天顶部单个买家／卖家信用标签的具名响应模型。
type chatCreditLevelDTO struct {
	// Role 是 buyer 或 seller。
	Role string `json:"role"`
	// Level 是平台一至五级数字信用。
	Level int `json:"level"`
	// Code 是平台稳定信用标签代码。
	Code string `json:"code"`
	// Text 是平台当前中文等级文案。
	Text string `json:"text"`
}

// chatCreditProfileDTO 是当前会话按需查询的非敏感信用摘要。
type chatCreditProfileDTO struct {
	// UserID 是被查询的闲鱼用户标识。
	UserID string `json:"user_id"`
	// Buyer 是买家信用；平台未返回时省略。
	Buyer *chatCreditLevelDTO `json:"buyer,omitempty"`
	// Seller 是卖家信用；平台未返回时省略。
	Seller *chatCreditLevelDTO `json:"seller,omitempty"`
	// FetchedAt 是最近一次成功平台查询时间。
	FetchedAt string `json:"fetched_at"`
	// ExpiresAt 是服务端缓存需要重新查询的时间。
	ExpiresAt string `json:"expires_at"`
	// Stale 表示当前因平台失败或熔断展示旧缓存。
	Stale bool `json:"stale"`
}

// newChatCreditLevelDTO 将应用层信用等级转换为稳定 HTTP 字段。
func newChatCreditLevelDTO(level *chatapp.CreditLevel) *chatCreditLevelDTO {
	if level == nil {
		return nil
	}
	return &chatCreditLevelDTO{Role: level.Role, Level: level.Level, Code: level.Code, Text: level.Text}
}

// newChatCreditProfileDTO 将应用信用缓存状态转换为当前会话响应。
func newChatCreditProfileDTO(profile chatapp.CreditProfile) chatCreditProfileDTO {
	return chatCreditProfileDTO{
		UserID: profile.UserID, Buyer: newChatCreditLevelDTO(profile.Buyer), Seller: newChatCreditLevelDTO(profile.Seller),
		FetchedAt: profile.FetchedAt.UTC().Format(time.RFC3339), ExpiresAt: profile.ExpiresAt.UTC().Format(time.RFC3339), Stale: profile.Stale,
	}
}
