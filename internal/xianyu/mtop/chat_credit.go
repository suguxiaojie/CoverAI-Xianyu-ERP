package mtop

import (
	"context"
	"fmt"
	"strings"
)

// UserCreditAPI 是闲鱼 PC 个人主页当前使用的买家／卖家信用资料端点。
const UserCreditAPI = "https://h5api.m.goofish.com/h5/mtop.idle.web.user.page.head/1.0/"

// UserCreditLevel 是平台公开的单个角色信用等级，不包含账号凭证或原始响应。
type UserCreditLevel struct {
	// Role 是 buyer 或 seller，区分买家与卖家信用。
	Role string
	// Level 是平台返回的 1 至 5 数字等级。
	Level int
	// Code 是平台稳定标签代码，例如 cs_buyer_level。
	Code string
	// Text 是平台当前展示文案，例如买家信用极好。
	Text string
}

// UserCreditInfo 是公开个人主页信用查询的最小结果。
type UserCreditInfo struct {
	// Levels 保存平台返回的买家和卖家信用；缺少某个角色时不构造占位等级。
	Levels []UserCreditLevel
	// UpdatedCookies 是 MTOP 签名请求吸收的新 Cookie，只允许适配器内部持久化。
	UpdatedCookies string
}

// FetchUserCredit 查询指定闲鱼用户的公开买家／卖家信用，并只保留结构化等级字段。
func (c *ClientImpl) FetchUserCredit(ctx context.Context, cookiesStr, userID string) (*UserCreditInfo, error) {
	// normalizedUserID 是去除空白后的公开个人主页用户标识。
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil, fmt.Errorf("信用查询缺少 userId")
	}
	// decoded、updated、requestErr 分别是平台响应、吸收后的 Cookie 和请求错误。
	decoded, updated, requestErr := c.accountTaskRequest(ctx, cookiesStr,
		firstNonEmptyURL(c.UserCreditURL, UserCreditAPI), "mtop.idle.web.user.page.head", "1.0",
		map[string]any{"self": false, "userId": normalizedUserID},
		"https://www.goofish.com/")
	if requestErr != nil {
		return nil, requestErr
	}
	// module 是个人主页响应中的展示模块集合。
	module, _ := decoded.Data["module"].(map[string]any)
	// base 是个人主页公开基础资料；不存在时响应不满足当前契约。
	base, _ := module["base"].(map[string]any)
	if base == nil {
		return nil, fmt.Errorf("个人主页信用响应缺少 module.base")
	}
	// rawTags 是平台返回的信用及其他个人标签原始列表。
	rawTags, _ := base["ylzTags"].([]any)
	// levels 只保存类型为 ylzLevel 且角色明确的信用等级。
	levels := make([]UserCreditLevel, 0, len(rawTags))
	for _, rawTag := range rawTags { // rawTag 是当前待解析的个人主页标签。
		// tag 是当前标签的动态字段映射。
		tag, _ := rawTag.(map[string]any)
		if tag == nil || !strings.EqualFold(strings.TrimSpace(mtopString(tag["type"])), "ylzLevel") {
			continue
		}
		// attributes 保存平台信用角色和数字等级。
		attributes, _ := tag["attributes"].(map[string]any)
		// role 和 level 是当前信用标签的角色与等级。
		role, level := strings.ToLower(strings.TrimSpace(mtopString(attributes["role"]))), mtopInt(attributes["level"])
		if (role != "buyer" && role != "seller") || level < 1 || level > 5 {
			continue
		}
		levels = append(levels, UserCreditLevel{
			Role: role, Level: level, Code: strings.TrimSpace(mtopString(tag["code"])), Text: strings.TrimSpace(mtopString(tag["text"])),
		})
	}
	return &UserCreditInfo{Levels: levels, UpdatedCookies: updated}, nil
}
