package adapter

import (
	"context"
	"errors"
	"strings"

	"xianyu-go/internal/account"
	chatapp "xianyu-go/internal/application/chat"
	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// chatCreditResolver 在适配器内短暂读取账号凭证并查询公开个人主页信用。
type chatCreditResolver struct {
	// credentials 提供现有账号 Cookie 读取和平台刷新后的受控写回能力。
	credentials chatCredentialRepository
	// clientProvider 返回当前可注入 MTOP 客户端，便于测试隔离真实平台。
	clientProvider func() mtop.Client
	// manager 用于把平台刷新后的 Cookie 同步给当前在线账号运行时。
	manager *account.Manager
}

// NewChatCreditResolver 创建只向应用层返回结构化买家／卖家信用的适配器。
func NewChatCreditResolver(store *db.Store, clientProvider func() mtop.Client, manager *account.Manager) chatapp.CreditResolver {
	if store == nil || store.Cookies == nil || clientProvider == nil {
		return nil
	}
	return chatCreditResolver{credentials: chatCredentialRepository{store: store}, clientProvider: clientProvider, manager: manager}
}

// Resolve 查询公开信用并在平台刷新签名 Cookie 时保持数据库和在线运行时一致。
func (r chatCreditResolver) Resolve(ctx context.Context, accountID, userID string) (chatapp.CreditProfile, error) {
	// cookieValue、credentialErr 是仅在当前适配器调用期间存在的明文 Cookie 和读取错误，禁止记录或返回。
	cookieValue, credentialErr := r.credentials.getCookieValue(ctx, accountID)
	if credentialErr != nil {
		return chatapp.CreditProfile{}, credentialErr
	}
	// client 是当前平台依赖提供的 MTOP 客户端。
	client := r.clientProvider()
	// fetcher、supported 是客户端公开信用查询能力和接口匹配状态。
	fetcher, supported := client.(interface {
		FetchUserCredit(context.Context, string, string) (*mtop.UserCreditInfo, error)
	})
	if !supported {
		return chatapp.CreditProfile{}, chatapp.ErrCreditUnavailable
	}
	// info、fetchErr 是平台最小信用结果和查询错误。
	info, fetchErr := fetcher.FetchUserCredit(ctx, cookieValue, userID)
	if fetchErr != nil {
		if mtop.IsRiskVerificationErr(fetchErr) {
			return chatapp.CreditProfile{}, errors.Join(chatapp.ErrCreditRisk, fetchErr)
		}
		return chatapp.CreditProfile{}, fetchErr
	}
	if info == nil {
		return chatapp.CreditProfile{}, chatapp.ErrCreditUnavailable
	}
	if strings.TrimSpace(info.UpdatedCookies) != "" && info.UpdatedCookies != cookieValue {
		// persistErr 是平台刷新 Cookie 的受控持久化错误；失败时不得返回看似成功的信用结果。
		if persistErr := r.credentials.updateCookieValue(ctx, accountID, info.UpdatedCookies); persistErr != nil {
			return chatapp.CreditProfile{}, persistErr
		}
		if r.manager != nil {
			// runtime、runtimeOK 是当前账号在线实例及存在状态。
			runtime, runtimeOK := r.manager.GetInstance(accountID)
			if runtimeOK && runtime != nil {
				runtime.UpdateCookie(info.UpdatedCookies)
			}
		}
	}
	// profile 只保存前端需要的结构化角色等级，不传播图标或原始平台响应。
	profile := chatapp.CreditProfile{UserID: strings.TrimSpace(userID)}
	for _, level := range info.Levels { // level 是当前待转换的公开信用标签。
		// converted 是脱离平台传输模型的应用层信用等级。
		converted := &chatapp.CreditLevel{Role: level.Role, Level: level.Level, Code: level.Code, Text: level.Text}
		switch level.Role {
		case "buyer":
			profile.Buyer = converted
		case "seller":
			profile.Seller = converted
		}
	}
	return profile, nil
}

// 编译期确认公开信用适配器满足聊天应用定义的最小端口。
var _ chatapp.CreditResolver = chatCreditResolver{}
