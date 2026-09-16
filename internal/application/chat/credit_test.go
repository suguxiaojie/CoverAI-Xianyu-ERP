package chat

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeCreditResolver 提供可计数、可失败的公开信用查询替身。
type fakeCreditResolver struct {
	// mu 保护 calls 和 err，允许并发合并测试安全读取。
	mu sync.Mutex
	// calls 是实际进入平台替身的次数。
	calls int
	// profile 是成功时返回的结构化信用摘要。
	profile CreditProfile
	// err 是当前模拟的平台失败。
	err error
}

// Resolve 记录一次平台调用，并返回测试配置的信用资料或错误。
func (r *fakeCreditResolver) Resolve(_ context.Context, _, _ string) (CreditProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.profile, r.err
}

// callCount 返回已经执行的平台替身次数。
func (r *fakeCreditResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// TestGetUserCreditCachesAcrossAccountsAndReturnsStructuredRoles 验证同一买家跨店铺共享二十四小时缓存。
func TestGetUserCreditCachesAcrossAccountsAndReturnsStructuredRoles(t *testing.T) {
	// repository 允许两个账号通过归属检查且不接触任何真实凭证。
	repository := &fakeRepository{ownedAccounts: map[string]bool{"account-1": true, "account-2": true}}
	// resolver 返回一个买家和卖家信用等级。
	resolver := &fakeCreditResolver{profile: CreditProfile{
		Buyer:  &CreditLevel{Role: "buyer", Level: 5, Code: "cs_buyer_level", Text: "买家信用极好"},
		Seller: &CreditLevel{Role: "seller", Level: 4, Code: "cs_seller_level", Text: "卖家信用优秀"},
	}}
	// service 是装配公开信用查询后的聊天应用服务。
	service := WithCreditResolver(New(repository), resolver)
	service.credit.requestGap = 0
	// first、firstErr 是第一家店铺触发的平台查询结果。
	first, firstErr := service.GetUserCredit(context.Background(), 1, "account-1", "buyer-1")
	// second、secondErr 是第二家店铺读取同一买家共享缓存的结果。
	second, secondErr := service.GetUserCredit(context.Background(), 1, "account-2", "buyer-1")
	if firstErr != nil || secondErr != nil || first.Buyer == nil || second.Seller == nil || resolver.callCount() != 1 {
		t.Fatalf("first=%+v second=%+v calls=%d errors=%v/%v", first, second, resolver.callCount(), firstErr, secondErr)
	}
}

// TestGetUserCreditUsesStaleCacheDuringRiskCooldown 验证风控后不重试平台并继续展示七天内旧信用。
func TestGetUserCreditUsesStaleCacheDuringRiskCooldown(t *testing.T) {
	// currentTime 是测试控制缓存新鲜度和熔断时间的 UTC 时钟。
	currentTime := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	// repository 允许目标账号归属校验。
	repository := &fakeRepository{ownedAccounts: map[string]bool{"account-1": true}}
	// resolver 首次成功，随后用于模拟平台风控。
	resolver := &fakeCreditResolver{profile: CreditProfile{Buyer: &CreditLevel{Role: "buyer", Level: 5, Text: "买家信用极好"}}}
	// service 是使用可控时钟和零请求间隔的聊天信用服务。
	service := WithCreditResolver(New(repository), resolver)
	service.credit.now = func() time.Time { return currentTime }
	service.credit.requestGap = 0
	service.credit.cacheTTL = time.Hour
	// first、firstErr 是用于建立成功缓存的首次结果。
	first, firstErr := service.GetUserCredit(context.Background(), 1, "account-1", "buyer-1")
	if firstErr != nil || first.Buyer == nil {
		t.Fatalf("first=%+v err=%v", first, firstErr)
	}
	currentTime = currentTime.Add(2 * time.Hour)
	resolver.mu.Lock()
	resolver.err = ErrCreditRisk
	resolver.mu.Unlock()
	// stale、staleErr 是缓存过期后平台风控返回的旧资料。
	stale, staleErr := service.GetUserCredit(context.Background(), 1, "account-1", "buyer-1")
	// cooled、cooledErr 是熔断期间不再访问平台的同一旧资料。
	cooled, cooledErr := service.GetUserCredit(context.Background(), 1, "account-1", "buyer-1")
	if staleErr != nil || cooledErr != nil || !stale.Stale || !cooled.Stale || resolver.callCount() != 2 {
		t.Fatalf("stale=%+v cooled=%+v calls=%d errors=%v/%v", stale, cooled, resolver.callCount(), staleErr, cooledErr)
	}
}

// TestGetUserCreditRejectsForeignAccountAndCoolsUncachedRisk 验证越权和无旧缓存风控都不会返回伪造等级。
func TestGetUserCreditRejectsForeignAccountAndCoolsUncachedRisk(t *testing.T) {
	// repository 只允许 account-1，拒绝 foreign。
	repository := &fakeRepository{ownedAccounts: map[string]bool{"account-1": true}}
	// resolver 始终返回平台风控。
	resolver := &fakeCreditResolver{err: ErrCreditRisk}
	// service 是装配风险替身后的信用服务。
	service := WithCreditResolver(New(repository), resolver)
	service.credit.requestGap = 0
	// foreign、foreignErr 是越权账号查询结果。
	foreign, foreignErr := service.GetUserCredit(context.Background(), 1, "foreign", "buyer-1")
	// blocked、blockedErr 是无旧缓存时的风控结果。
	blocked, blockedErr := service.GetUserCredit(context.Background(), 1, "account-1", "buyer-1")
	if !errors.Is(foreignErr, ErrSessionForbidden) || !errors.Is(blockedErr, ErrCreditCoolingDown) || foreign.Buyer != nil || blocked.Buyer != nil || resolver.callCount() != 1 {
		t.Fatalf("foreign=%+v blocked=%+v calls=%d errors=%v/%v", foreign, blocked, resolver.callCount(), foreignErr, blockedErr)
	}
}
