package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	// ErrCreditUnavailable 表示公开信用查询端口没有完成装配。
	ErrCreditUnavailable = errors.New("信用查询服务未启用")
	// ErrCreditCoolingDown 表示当前用户或全局信用查询正在冷却，不得立即重试平台。
	ErrCreditCoolingDown = errors.New("信用查询暂缓更新")
	// ErrCreditRisk 表示平台返回频控、验证码或其他风控结果，应用层必须开启全局熔断。
	ErrCreditRisk = errors.New("平台限制信用查询")
)

const (
	// defaultCreditCacheTTL 是成功信用资料不再访问平台的时长。
	defaultCreditCacheTTL = 24 * time.Hour
	// defaultCreditStaleTTL 是平台暂时失败时仍允许展示旧信用资料的最长时长。
	defaultCreditStaleTTL = 7 * 24 * time.Hour
	// defaultCreditFailureCooldown 是普通网络或平台错误针对单个用户的冷却时间。
	defaultCreditFailureCooldown = 10 * time.Minute
	// defaultCreditRiskCooldown 是命中平台频控或验证后的全局熔断时间。
	defaultCreditRiskCooldown = 2 * time.Hour
	// defaultCreditRequestGap 是不同用户信用请求之间的最小启动间隔。
	defaultCreditRequestGap = 3 * time.Second
)

// CreditLevel 是聊天页可以公开展示的单个角色信用等级。
type CreditLevel struct {
	// Role 是 buyer 或 seller。
	Role string
	// Level 是平台一至五级信用数字。
	Level int
	// Code 是平台稳定信用代码。
	Code string
	// Text 是平台当前中文等级文案。
	Text string
}

// CreditProfile 是当前买家的公开信用摘要和缓存状态。
type CreditProfile struct {
	// UserID 是被查询的闲鱼用户标识。
	UserID string
	// Buyer 是买家信用；平台未返回时为空。
	Buyer *CreditLevel
	// Seller 是卖家信用；平台未返回时为空。
	Seller *CreditLevel
	// FetchedAt 是最近一次成功平台查询的 UTC 时间。
	FetchedAt time.Time
	// ExpiresAt 是本结果需要重新查询平台的 UTC 时间。
	ExpiresAt time.Time
	// Stale 表示本次因平台失败或熔断返回了仍可展示的旧资料。
	Stale bool
}

// CreditResolver 定义适配器读取账号凭证并查询目标用户公开信用的最小能力。
type CreditResolver interface {
	// Resolve 使用指定账号的现有平台会话查询目标用户信用，不向应用层返回凭证。
	Resolve(ctx context.Context, accountID, userID string) (CreditProfile, error)
}

// creditCacheEntry 保存一位用户最近成功查询结果及其新鲜和兜底截止时间。
type creditCacheEntry struct {
	// profile 是最近成功的平台信用摘要。
	profile CreditProfile
	// staleUntil 是平台失败时允许继续展示此摘要的最后时间。
	staleUntil time.Time
}

// creditCoordinator 拥有信用缓存、单请求合并、串行请求门和冷却状态。
// mu 只保护内存时间与映射，绝不在持锁期间调用平台；gate 串行化外部 I/O。
type creditCoordinator struct {
	// resolver 是实际读取公开信用的平台适配端口。
	resolver CreditResolver
	// mu 保护 cache、failureUntil、nextRequestAt 和 circuitUntil。
	mu sync.Mutex
	// cache 按公开 userId 跨店铺复用成功信用摘要。
	cache map[string]creditCacheEntry
	// failureUntil 按 userId 记录普通失败后的重试时间。
	failureUntil map[string]time.Time
	// nextRequestAt 是下一次允许启动任意信用平台请求的时间。
	nextRequestAt time.Time
	// circuitUntil 是命中平台风控后全局禁止信用请求的截止时间。
	circuitUntil time.Time
	// gate 保证任何时刻只有一个信用平台请求执行；发送者和关闭者均为 coordinator。
	gate chan struct{}
	// group 合并同一 userId 的并发查询，避免重复消耗平台调用。
	group singleflight.Group
	// now 为测试提供可替换时钟；生产使用 UTC 系统时间。
	now func() time.Time
	// cacheTTL、staleTTL、failureCooldown、riskCooldown 和 requestGap 保存可测试的频率参数。
	cacheTTL, staleTTL, failureCooldown, riskCooldown, requestGap time.Duration
}

// newCreditCoordinator 创建包含一个可用串行令牌的信用查询协调器。
func newCreditCoordinator(resolver CreditResolver) *creditCoordinator {
	// coordinator 是完成默认缓存和冷却参数初始化的运行期所有者。
	coordinator := &creditCoordinator{
		resolver: resolver, cache: make(map[string]creditCacheEntry), failureUntil: make(map[string]time.Time), gate: make(chan struct{}, 1),
		now: func() time.Time { return time.Now().UTC() }, cacheTTL: defaultCreditCacheTTL, staleTTL: defaultCreditStaleTTL,
		failureCooldown: defaultCreditFailureCooldown, riskCooldown: defaultCreditRiskCooldown, requestGap: defaultCreditRequestGap,
	}
	coordinator.gate <- struct{}{}
	return coordinator
}

// WithCreditResolver 为聊天应用装配按需信用查询；空端口保持明确不可用语义。
func WithCreditResolver(service *Service, resolver CreditResolver) *Service {
	if service != nil && resolver != nil {
		service.credit = newCreditCoordinator(resolver)
	}
	return service
}

// GetUserCredit 校验账号归属后读取缓存或低频查询当前会话买家的公开信用。
func (s *Service) GetUserCredit(ctx context.Context, userID int64, accountID, buyerID string) (CreditProfile, error) {
	// normalizedAccountID 和 normalizedBuyerID 是完成空白清理的账号与目标用户标识。
	normalizedAccountID, normalizedBuyerID := strings.TrimSpace(accountID), strings.TrimSpace(buyerID)
	if s == nil || s.repository == nil || userID <= 0 || normalizedAccountID == "" || normalizedBuyerID == "" || normalizedBuyerID == "1400" {
		return CreditProfile{}, ErrInvalidInput
	}
	// repository 是只返回账号存在性的窄会话仓储。
	repository, supported := s.repository.(SessionRepository)
	if !supported {
		return CreditProfile{}, ErrSessionUnavailable
	}
	// owned、ownershipErr 是不读取凭证的账号归属结果和仓储错误。
	owned, ownershipErr := repository.ExistsOwned(ctx, userID, normalizedAccountID)
	if ownershipErr != nil {
		return CreditProfile{}, ownershipErr
	}
	if !owned {
		return CreditProfile{}, ErrSessionForbidden
	}
	if s.credit == nil {
		return CreditProfile{}, ErrCreditUnavailable
	}
	return s.credit.get(ctx, normalizedAccountID, normalizedBuyerID)
}

// get 合并同一用户请求，并在缓存、失败冷却和风控熔断边界内执行一次查询。
func (c *creditCoordinator) get(ctx context.Context, accountID, buyerID string) (CreditProfile, error) {
	if c == nil || c.resolver == nil {
		return CreditProfile{}, ErrCreditUnavailable
	}
	// cached、cacheOK 和 immediateErr 是无需进入 singleflight 时的缓存结果、命中状态和冷却错误。
	cached, cacheOK, immediateErr := c.cachedResult(buyerID)
	if cacheOK || immediateErr != nil {
		return cached, immediateErr
	}
	// value、resolveErr 和 _ 是同一 buyerId 合并后的动态结果、查询错误和共享标记。
	value, resolveErr, _ := c.group.Do(buyerID, func() (any, error) {
		// rechecked、recheckedOK 和 recheckedErr 防止等待 singleflight 时另一请求已写入缓存或冷却。
		rechecked, recheckedOK, recheckedErr := c.cachedResult(buyerID)
		if recheckedOK || recheckedErr != nil {
			return rechecked, recheckedErr
		}
		return c.resolve(ctx, accountID, buyerID)
	})
	if resolveErr != nil {
		return CreditProfile{}, resolveErr
	}
	// profile、ok 是 singleflight 返回值的类型安全信用摘要。
	profile, ok := value.(CreditProfile)
	if !ok {
		return CreditProfile{}, ErrCreditUnavailable
	}
	return profile, nil
}

// cachedResult 返回新鲜缓存，或在冷却期间返回可展示旧缓存／明确冷却错误。
func (c *creditCoordinator) cachedResult(buyerID string) (CreditProfile, bool, error) {
	// now 是当前缓存和冷却判断使用的 UTC 时间。
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	// entry、hasEntry 是当前目标用户最近成功的信用缓存和存在状态。
	entry, hasEntry := c.cache[buyerID]
	if hasEntry && now.Before(entry.profile.ExpiresAt) {
		return entry.profile, true, nil
	}
	// cooling 表示全局风控或当前用户普通失败仍在冷却。
	cooling := now.Before(c.circuitUntil) || now.Before(c.failureUntil[buyerID])
	if cooling {
		if hasEntry && now.Before(entry.staleUntil) {
			// staleProfile 是标记为旧数据但仍可展示的缓存副本。
			staleProfile := entry.profile
			staleProfile.Stale = true
			return staleProfile, true, nil
		}
		return CreditProfile{}, false, ErrCreditCoolingDown
	}
	return CreditProfile{}, false, nil
}

// resolve 等待全局串行令牌和最小请求间隔，然后调用平台并更新缓存或冷却。
func (c *creditCoordinator) resolve(ctx context.Context, accountID, buyerID string) (CreditProfile, error) {
	select {
	case <-ctx.Done():
		return CreditProfile{}, ctx.Err()
	case <-c.gate:
	}
	defer func() { c.gate <- struct{}{} }()
	// now 是取得串行令牌后的时间基准。
	now := c.now()
	c.mu.Lock()
	// wait 是距离下一次允许启动平台请求的剩余时长。
	wait := c.nextRequestAt.Sub(now)
	c.mu.Unlock()
	if wait > 0 {
		// timer 由当前请求拥有，Context 取消时必须停止。
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return CreditProfile{}, ctx.Err()
		case <-timer.C:
		}
	}
	// startedAt 是本次平台请求实际启动时间，并确定下一次全局请求窗口。
	startedAt := c.now()
	c.mu.Lock()
	c.nextRequestAt = startedAt.Add(c.requestGap)
	c.mu.Unlock()
	// profile、resolveErr 是适配器返回的公开信用摘要和平台错误。
	profile, resolveErr := c.resolver.Resolve(ctx, accountID, buyerID)
	if resolveErr != nil {
		return c.recordFailure(buyerID, resolveErr)
	}
	profile.UserID, profile.FetchedAt, profile.ExpiresAt, profile.Stale = buyerID, startedAt, startedAt.Add(c.cacheTTL), false
	c.mu.Lock()
	c.cache[buyerID] = creditCacheEntry{profile: profile, staleUntil: startedAt.Add(c.staleTTL)}
	delete(c.failureUntil, buyerID)
	c.mu.Unlock()
	return profile, nil
}

// recordFailure 写入普通失败或全局风控冷却，并优先返回仍在七天兜底期内的旧缓存。
func (c *creditCoordinator) recordFailure(buyerID string, resolveErr error) (CreditProfile, error) {
	// now 是失败冷却和旧缓存判断使用的 UTC 时间。
	now := c.now()
	c.mu.Lock()
	if errors.Is(resolveErr, ErrCreditRisk) {
		c.circuitUntil = now.Add(c.riskCooldown)
	} else {
		c.failureUntil[buyerID] = now.Add(c.failureCooldown)
	}
	// entry、hasEntry 是失败前最后一次成功缓存和存在状态。
	entry, hasEntry := c.cache[buyerID]
	c.mu.Unlock()
	if hasEntry && now.Before(entry.staleUntil) {
		// staleProfile 保留平台最近成功数据并标明当前是失败兜底。
		staleProfile := entry.profile
		staleProfile.Stale = true
		return staleProfile, nil
	}
	if errors.Is(resolveErr, ErrCreditRisk) {
		return CreditProfile{}, ErrCreditCoolingDown
	}
	return CreditProfile{}, resolveErr
}
