package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// incrementalLifecycleRecheckLimit 限制交互式增量同步每账号轮转复核的历史订单数量，避免历史巡检阻塞新订单补全。
	incrementalLifecycleRecheckLimit = 3
	// incrementalMinimumPages 保留两页安全回看，兼顾新订单分页抖动与交互同步速度。
	incrementalMinimumPages = 2
	// incrementalBoundaryRequired 要求连续命中十笔历史订单后才停止，避免单条乱序订单导致过早截断。
	incrementalBoundaryRequired = 10
)

// ErrRefreshDetailUnsupported 表示当前平台运行时不支持订单详情接口。
var ErrRefreshDetailUnsupported = errors.New("当前 Go MTOP 客户端不支持订单详情接口")

// ErrRefreshCredentialChanged 表示刷新期间账号凭证无法通过一致性复核。
var ErrRefreshCredentialChanged = errors.New("账号凭证已变化，请重试")

// RefreshService 承载订单单笔和批量刷新的应用编排。
type RefreshService struct {
	// repository 保存订单刷新所需的持久化 Port。
	repository RefreshRepository
	// runtime 保存订单刷新所需的平台运行时 Port。
	runtime RefreshRuntime
	// detailChunkSize 是批量详情请求的单账号分片大小。
	detailChunkSize int
}

// NewRefreshService 创建订单刷新应用服务。
func NewRefreshService(repository RefreshRepository, runtime RefreshRuntime, detailChunkSize int) *RefreshService {
	if detailChunkSize <= 0 {
		detailChunkSize = 100
	}
	return &RefreshService{repository: repository, runtime: runtime, detailChunkSize: detailChunkSize}
}

// RefreshSingle 刷新单个订单详情并写回本地订单。
func (s *RefreshService) RefreshSingle(ctx context.Context, userID int64, orderID string) (SingleRefreshResult, error) {
	if s == nil || s.repository == nil || s.runtime == nil {
		return SingleRefreshResult{}, errors.New("订单刷新依赖未初始化")
	}
	// order、err 保存订单读取结果及错误。
	order, err := s.repository.GetOrder(ctx, orderID)
	if err != nil {
		return SingleRefreshResult{}, err
	}
	if order == nil {
		return SingleRefreshResult{}, ErrNotFound
	}
	if strings.TrimSpace(order.CookieID) == "" {
		return SingleRefreshResult{}, ErrForbidden
	}
	// owned 保存订单账号归属结果。
	owned, err := s.repository.ExistsOwned(ctx, userID, order.CookieID)
	if err != nil {
		return SingleRefreshResult{}, err
	}
	if !owned {
		return SingleRefreshResult{}, ErrForbidden
	}
	if !s.runtime.DetailAvailable() {
		return SingleRefreshResult{}, ErrRefreshDetailUnsupported
	}
	// cookieID 保存订单所属账号标识。
	cookieID := order.CookieID
	// unlock 保存凭证锁释放函数。
	unlock := s.repository.LockCredentials(cookieID)
	// locked 表示当前函数是否仍持有凭证锁。
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()
	// latest、err 保存加锁后读取的平台凭证视图及错误。
	latest, err := s.repository.LoadCookiePlatformDetail(ctx, cookieID)
	if err != nil || latest == nil || latest.UserID != userID || !s.runtime.CredentialAvailable(latest) {
		return SingleRefreshResult{}, ErrRefreshCredentialChanged
	}
	// refreshCtx、cancel 限制单订单外部请求最长执行时间。
	refreshCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	// unlockBeforeFetch 保存外部请求前释放凭证锁的动作。
	unlock()
	locked = false
	// fetchResult、callErr 保存远端详情结果及错误。
	fetchResult, callErr := s.runtime.FetchOrderDetail(refreshCtx, latest, orderID)
	// unlockAfterFetch 保存外部请求完成后重新获取凭证锁的释放函数。
	unlockAfterFetch := s.repository.LockCredentials(cookieID)
	// latestAfterFetch、reloadErr 保存外部请求后的凭证视图及重读错误。
	latestAfterFetch, reloadErr := s.repository.LoadCookiePlatformDetail(ctx, cookieID)
	// credentialChanged 表示外部请求期间凭证快照是否发生变化。
	credentialChanged := reloadErr != nil || latestAfterFetch == nil || latestAfterFetch.UserID != userID || latestAfterFetch.Value != latest.Value || latestAfterFetch.MetadataJSON != latest.MetadataJSON
	if !credentialChanged {
		// value、valueChanged、handled、persistErr 保存 Cookie 会话提交结果。
		value, valueChanged, handled, persistErr := s.runtime.PersistCookieSession(ctx, latest, fetchResult.CookieUpdate)
		if persistErr != nil {
			callErr = errors.Join(callErr, fmt.Errorf("保存订单详情响应 Cookie Jar: %w", persistErr))
		} else if handled && valueChanged {
			if value != "" {
				s.runtime.UpdateRunningCookie(ctx, cookieID, value)
			}
		} else if !handled && callErr == nil && fetchResult.Detail != nil && fetchResult.Detail.UpdatedCookies != "" && fetchResult.Detail.UpdatedCookies != latest.Value {
			// metadata 保存不含快照的 Cookie 元数据。
			metadata := latest.MetadataJSON
			// saveErr 保存扁平 Cookie 写入错误。
			if saveErr := s.repository.UpdateRenewalCookie(ctx, cookieID, fetchResult.Detail.UpdatedCookies, metadata, time.Now().Unix()); saveErr == nil {
				s.runtime.UpdateRunningCookie(ctx, cookieID, fetchResult.Detail.UpdatedCookies)
			}
		}
	}
	unlockAfterFetch()
	if credentialChanged {
		return SingleRefreshResult{}, ErrRefreshCredentialChanged
	}
	if callErr != nil {
		s.runtime.RecoverExpiredSession(ctx, cookieID, callErr)
		return SingleRefreshResult{}, callErr
	}
	if fetchResult.Detail == nil {
		return SingleRefreshResult{}, errors.New("订单详情接口未返回结果")
	}
	// status 保存规范化后的订单状态。
	status := ResolveOrderLifecycleStatusWithRefundContext(order.OrderStatus, fetchResult.Detail.OrderStatus, order.RefundRequested)
	if !ValidEditableOrderStatus(status) {
		status = NormalizeOrderStatus(order.OrderStatus)
	}
	// options 保存单笔详情字段和本次明确终态对应的里程碑时间。
	options := UpsertOptions{CookieID: cookieID, OrderStatus: status, SpecName: fetchResult.Detail.SpecName, SpecValue: fetchResult.Detail.SpecValue, Quantity: fetchResult.Detail.Quantity, Amount: fetchResult.Detail.Amount}
	ApplyOrderLifecycleMilestone(&options, status, time.Now().UTC())
	// err 保存订单详情写入错误。
	if err := s.repository.UpsertOrder(ctx, orderID, options); err != nil {
		return SingleRefreshResult{}, err
	}
	return SingleRefreshResult{Success: true, Message: "订单刷新完成", Detail: RefreshDetail{Quantity: fetchResult.Detail.Quantity, SpecName: fetchResult.Detail.SpecName, SpecValue: fetchResult.Detail.SpecValue, OrderStatus: status, Amount: fetchResult.Detail.Amount}}, nil
}

// Refresh 执行当前用户订单的发现、缺失清理和详情补全；同步兼容调用不发布任务进度。
func (s *RefreshService) Refresh(ctx context.Context, userID int64, cookieID, status string) (RefreshResult, error) {
	return s.refresh(ctx, userID, cookieID, status, RefreshModeIncremental, nil)
}

// RefreshWithProgress 执行订单刷新并在发现、详情和收尾阶段发布轻量实时进度。
func (s *RefreshService) RefreshWithProgress(ctx context.Context, userID int64, cookieID, status string, reporter RefreshJobProgressReporter) (RefreshResult, error) {
	return s.refresh(ctx, userID, cookieID, status, RefreshModeIncremental, reporter)
}

// RefreshInMode 按指定增量或全量模式执行订单刷新；未知模式按增量处理。
func (s *RefreshService) RefreshInMode(ctx context.Context, userID int64, cookieID, status, mode string) (RefreshResult, error) {
	return s.refresh(ctx, userID, cookieID, status, normalizeRefreshMode(mode), nil)
}

// RefreshWithProgressInMode 按指定同步模式执行订单刷新并发布实时进度。
func (s *RefreshService) RefreshWithProgressInMode(ctx context.Context, userID int64, cookieID, status, mode string, reporter RefreshJobProgressReporter) (RefreshResult, error) {
	return s.refresh(ctx, userID, cookieID, status, normalizeRefreshMode(mode), reporter)
}

// normalizeRefreshMode 接受完整校准和历史成本补全，其余空值或未知值统一使用低请求量的增量模式。
func normalizeRefreshMode(mode string) string {
	// normalized 是去除空白并统一大小写后的任务模式。
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == RefreshModeFull {
		return RefreshModeFull
	}
	if normalized == RefreshModeCostBackfill {
		return RefreshModeCostBackfill
	}
	return RefreshModeIncremental
}

// refresh 是同步调用与后台任务共享的订单刷新实现；reporter 为空时完全保持既有结果语义。
func (s *RefreshService) refresh(ctx context.Context, userID int64, cookieID, status, mode string, reporter RefreshJobProgressReporter) (RefreshResult, error) {
	if s == nil || s.repository == nil || s.runtime == nil {
		return RefreshResult{}, errors.New("订单刷新依赖未初始化")
	}
	// cookieIDs、err 保存用户账号列表及错误。
	cookieIDs, err := s.repository.ListOwnedIDs(ctx, userID)
	if err != nil {
		return RefreshResult{}, err
	}
	if cookieID != "" {
		// owned、ownedErr 保存筛选账号归属结果及错误。
		owned, ownedErr := s.repository.ExistsOwned(ctx, userID, cookieID)
		if ownedErr != nil {
			return RefreshResult{}, ownedErr
		}
		if !owned {
			return RefreshResult{}, ErrForbidden
		}
		cookieIDs = []string{cookieID}
	}
	// progress 把内部阶段转换为不含账号、订单号或凭证的任务进度。
	progress := newRefreshProgressTracker(reporter)
	// discoveryProcessed、discoverySucceeded、discoveryFailed 是逐账号发现阶段的实时统计。
	discoveryProcessed, discoverySucceeded, discoveryFailed := 0, 0, 0
	progress.discovering(0, len(cookieIDs), 0, 0)
	// summary 保存批量刷新统计，并保留账号维度供完成页解释部分失败。
	summary := RefreshSummary{AccountTotal: len(cookieIDs)}
	// results 保存逐账号或逐订单结果。
	results := make([]RefreshOrderResult, 0)
	// newOrderIDs 保存发现的新订单标识。
	newOrderIDs := make(map[string]struct{})
	// sessionExpiredAccounts 保存会话过期账号标识。
	sessionExpiredAccounts := make(map[string]struct{})
	if mode == RefreshModeCostBackfill {
		discoveryProcessed, discoverySucceeded = len(cookieIDs), len(cookieIDs)
		summary.AccountSucceeded = len(cookieIDs)
		progress.discovering(discoveryProcessed, len(cookieIDs), discoverySucceeded, 0)
	} else if s.runtime.SoldAvailable() {
		// currentCookieID 是当前执行订单发现的账号标识。
		for accountIndex, currentCookieID := range cookieIDs {
			// pageReporter 把当前账号分页读取映射为跨账号整体发现进度。
			pageReporter := func(pageProgress RefreshSoldPageProgress) {
				progress.importingOrders(accountIndex+1, len(cookieIDs), pageProgress)
			}
			// discovered、updated、discoveryResult、discoveryErr 保存账号发现结果。
			discovered, updated, discoveryResult, discoveryErr := s.discoverAccount(ctx, userID, currentCookieID, mode, pageReporter)
			summary.Discovered += discovered
			summary.ListUpdated += updated
			// orderID 是本次发现的新订单标识。
			for orderID := range discoveryResult.NewOrderIDs {
				newOrderIDs[orderID] = struct{}{}
			}
			if discoveryResult.SessionExpired {
				sessionExpiredAccounts[currentCookieID] = struct{}{}
			}
			// result 保存当前账号的订单发现结果。
			result := RefreshOrderResult{CookieID: currentCookieID, Stage: "discover", Success: discoveryErr == nil, Discovered: discovered, Updated: updated, ConflictSkipped: discoveryResult.ConflictSkipped}
			summary.ConflictSkipped += discoveryResult.ConflictSkipped
			if discoveryErr != nil {
				summary.Failed++
				summary.AccountFailed++
				discoveryFailed++
				result.Error = discoveryErr.Error()
			} else {
				discoverySucceeded++
				summary.AccountSucceeded++
			}
			if discoveryResult.SoftDeleted >= 0 && discoveryErr == nil {
				result.SoftDeleted = discoveryResult.SoftDeleted
				summary.SoftDeleted += discoveryResult.SoftDeleted
			}
			results = append(results, result)
			discoveryProcessed++
			progress.discovering(discoveryProcessed, len(cookieIDs), discoverySucceeded, discoveryFailed)
		}
	} else {
		summary.Failed++
		summary.AccountFailed = len(cookieIDs)
		results = append(results, RefreshOrderResult{Stage: "discover", Message: "当前 MTop 客户端不支持订单列表发现"})
		progress.discovering(len(cookieIDs), len(cookieIDs), 0, 1)
	}
	// ordersByCookie 保存按状态筛选、会话阻断和增量轮转规则收集的详情目标。
	ordersByCookie := s.collectRefreshTargets(ctx, cookieIDs, status, mode, sessionExpiredAccounts, newOrderIDs)
	// total 保存需要补全详情的订单总数。
	total := 0
	// targets 是当前账号的待补全详情目标。
	for _, targets := range ordersByCookie {
		total += len(targets)
	}
	summary.DetailTotal, summary.Total = total, total
	progress.preparing(total)
	if !s.runtime.DetailAvailable() {
		// message 保存详情接口不可用时返回的说明。
		message := refreshCompletionMessage(summary)
		if total > 0 {
			message += fmt.Sprintf("；当前 Go MTOP 客户端不支持详情接口，已跳过 %d 个订单", total)
		}
		progress.finalizing(0, total, 0, summary.Failed)
		return RefreshResult{PartialFailure: summary.Failed > 0, Message: message, Summary: summary, Results: results}, nil
	}
	if total == 0 {
		progress.finalizing(0, 0, 0, summary.Failed)
		return RefreshResult{PartialFailure: summary.Failed > 0, Message: refreshCompletionMessage(summary) + "；没有需要补全详情的订单", Summary: summary, Results: results}, nil
	}
	// detailProcessed、detailSucceeded、detailFailed 是详情请求完成后的累计实时统计。
	detailProcessed, detailSucceeded, detailFailed := 0, 0, 0
	progress.syncingDetails(0, total, 0, 0)
	// onDetailProcessed 在每个远端详情请求返回后更新任务进度，不携带订单标识或响应内容。
	onDetailProcessed := func(success bool) {
		detailProcessed++
		if success {
			detailSucceeded++
		} else {
			detailFailed++
		}
		progress.syncingDetails(detailProcessed, total, detailSucceeded, detailFailed)
	}
	// currentCookieID、targets 保存当前账号及其详情目标。
	for currentCookieID, targets := range ordersByCookie {
		// blocked 表示账号是否因会话过期而跳过详情。
		if _, blocked := sessionExpiredAccounts[currentCookieID]; blocked {
			continue
		}
		// accountExpired 表示当前账号是否因会话过期而停止处理。
		accountExpired := false
		// chunk 是当前账号的详情请求分片。
		for _, chunk := range splitRefreshTargets(targets, s.detailChunkSize) {
			// updated、noChange、failed、chunkResults、expired 保存分片处理统计和结果。
			updated, noChange, failed, chunkResults, expired := s.refreshDetailChunk(ctx, userID, currentCookieID, chunk, onDetailProcessed)
			summary.Updated += updated
			summary.NoChange += noChange
			summary.Failed += failed
			results = append(results, chunkResults...)
			if expired {
				accountExpired = true
				break
			}
		}
		if accountExpired {
			continue
		}
	}
	progress.finalizing(detailProcessed, total, detailSucceeded, summary.Failed)
	return RefreshResult{PartialFailure: summary.Failed > 0, Message: refreshCompletionMessage(summary), Summary: summary, Results: results}, nil
}

// refreshDiscoveryResult 保存单账号发现阶段的内部结果。
type refreshDiscoveryResult struct {
	// NewOrderIDs 保存本次发现的新订单标识。
	NewOrderIDs map[string]struct{}
	// SoftDeleted 保存本次发现阶段标记删除的订单数量。
	SoftDeleted int
	// SessionExpired 表示本账号平台会话已过期。
	SessionExpired bool
	// ConflictSkipped 是本账号远端批次中因已有其他账号归属而安全跳过的订单数。
	ConflictSkipped int
}

// discoverAccount 执行一个账号的锁内快照、锁外发现和锁内提交。
func (s *RefreshService) discoverAccount(ctx context.Context, userID int64, cookieID, mode string, reporter RefreshSoldPageReporter) (int, int, refreshDiscoveryResult, error) {
	// emptyResult 保存发现失败时的默认结果。
	emptyResult := refreshDiscoveryResult{NewOrderIDs: make(map[string]struct{})}
	// unlock 保存账号凭证锁释放函数。
	unlock := s.repository.LockCredentials(cookieID)
	// latest、err 保存最新账号平台视图及错误。
	latest, err := s.repository.LoadCookiePlatformDetail(ctx, cookieID)
	if err != nil || latest == nil || latest.UserID != userID || !s.runtime.CredentialAvailable(latest) {
		unlock()
		if err == nil {
			err = errors.New("账号凭证已变化")
		}
		return 0, 0, emptyResult, err
	}
	unlock()
	// cursor、cursorExists、cursorErr 保存账号已确认的增量同步边界及查询结果。
	cursor, cursorExists, cursorErr := s.repository.GetOrderSyncCursor(ctx, cookieID)
	if cursorErr != nil {
		return 0, 0, emptyResult, fmt.Errorf("读取订单同步游标失败: %w", cursorErr)
	}
	// actualMode 是当前账号最终使用的读取范围；无游标时必须全量建立可靠基线。
	actualMode := normalizeRefreshMode(mode)
	if !cursorExists {
		actualMode = RefreshModeFull
	}
	// fetchResult 保存锁外订单发现结果。
	var fetchResult RefreshSoldFetchResult
	// discoveryErr 保存订单分页读取或平台调用错误。
	var discoveryErr error
	// modeRuntime、supportsMode 表示平台运行时是否支持增量边界停止。
	modeRuntime, supportsMode := s.runtime.(RefreshSoldModeRuntime)
	// progressRuntime、supportsProgress 表示旧平台运行时是否至少支持逐页全量进度。
	progressRuntime, supportsProgress := s.runtime.(RefreshSoldProgressRuntime)
	if supportsMode {
		fetchResult, discoveryErr = modeRuntime.FetchSoldOrdersWithOptions(ctx, latest, RefreshSoldFetchOptions{
			Mode: actualMode, Cursor: cursor, MinimumPages: incrementalMinimumPages, BoundaryRequired: incrementalBoundaryRequired,
		}, reporter)
	} else if supportsProgress {
		fetchResult, discoveryErr = progressRuntime.FetchSoldOrdersWithProgress(ctx, latest, reporter)
		fetchResult.CompleteSnapshot = discoveryErr == nil
	} else {
		fetchResult, discoveryErr = s.runtime.FetchSoldOrders(ctx, latest)
		fetchResult.CompleteSnapshot = discoveryErr == nil
	}
	unlock = s.repository.LockCredentials(cookieID)
	// latestAfterFetch、reloadErr 保存发现完成后的最新凭证及错误。
	// latestAfterFetch、reloadErr 保存发现完成后的凭证视图及重读错误。
	latestAfterFetch, reloadErr := s.repository.LoadCookiePlatformDetail(ctx, cookieID)
	// credentialChanged 表示发现期间凭证快照是否发生变化。
	credentialChanged := reloadErr != nil || latestAfterFetch == nil || latestAfterFetch.UserID != userID || latestAfterFetch.Value != latest.Value || latestAfterFetch.MetadataJSON != latest.MetadataJSON
	if reloadErr == nil && latestAfterFetch != nil && latestAfterFetch.UserID == userID && !credentialChanged {
		// persistErr 保存发现响应 Cookie Jar 写入错误。
		_, _, _, persistErr := s.runtime.PersistCookieSession(ctx, latest, fetchResult.CookieUpdate)
		if persistErr != nil {
			discoveryErr = errors.Join(discoveryErr, fmt.Errorf("保存订单列表响应 Cookie Jar: %w", persistErr))
		}
	}
	unlock()
	if credentialChanged {
		discoveryErr = errors.Join(discoveryErr, errors.New("订单发现完成后账号凭证无法复核"))
	}
	if fetchResult.CookieUpdate.Changed && fetchResult.CookieUpdate.Value != "" && discoveryErr == nil {
		s.runtime.UpdateRunningCookie(ctx, cookieID, fetchResult.CookieUpdate.Value)
	}
	// result 保存当前账号发现阶段的汇总结果。
	result := emptyResult
	if s.runtime.IsSessionExpired(discoveryErr) {
		result.SessionExpired = true
		s.runtime.RecoverExpiredSession(ctx, cookieID, discoveryErr)
	}
	if discoveryErr != nil {
		return 0, 0, result, discoveryErr
	}
	// discovered、updated、conflictSkipped、newOrderIDs、remoteOrderIDs、writeErr 保存订单发现统计和错误。
	discovered, updated, conflictSkipped, newOrderIDs, remoteOrderIDs, writeErr := s.persistSoldOrders(ctx, cookieID, fetchResult.Orders)
	result.NewOrderIDs, result.SoftDeleted, result.ConflictSkipped = newOrderIDs, 0, conflictSkipped
	if writeErr != nil {
		return discovered, updated, result, writeErr
	}
	if actualMode == RefreshModeFull && fetchResult.CompleteSnapshot {
		// deleted、deleteErr 保存完整远端快照缺失订单清理数量及错误。
		deleted, deleteErr := s.repository.SoftDeleteMissingOrders(ctx, cookieID, remoteOrderIDs)
		if deleteErr != nil {
			return discovered, updated, result, fmt.Errorf("标记缺失订单失败: %w", deleteErr)
		}
		result.SoftDeleted = deleted
	}
	// nextCursor、hasHighWater 保存本次订单集合计算出的最新可靠平台创建时间。
	nextCursor, hasHighWater := newestOrderSyncCursor(cookieID, fetchResult.Orders, cursor)
	if hasHighWater && (fetchResult.CompleteSnapshot || fetchResult.BoundaryReached) {
		// now 是本次成功发现阶段统一写入的 Unix 秒。
		now := time.Now().Unix()
		nextCursor.LastIncrementalSyncAt = now
		if actualMode == RefreshModeFull && fetchResult.CompleteSnapshot {
			nextCursor.LastFullSyncAt = now
		}
		if cursor != nil && nextCursor.LastFullSyncAt == 0 {
			nextCursor.LastFullSyncAt = cursor.LastFullSyncAt
		}
		// cursorErr 保存账号高水位写入错误；失败时不把本次任务报告为完整成功。
		cursorErr := s.repository.UpsertOrderSyncCursor(ctx, nextCursor)
		if cursorErr != nil {
			return discovered, updated, result, fmt.Errorf("保存订单同步游标失败: %w", cursorErr)
		}
	}
	return discovered, updated, result, nil
}

// newestOrderSyncCursor 从已成功读取的订单中选择最新平台时间，并保留既有同步时间信息。
func newestOrderSyncCursor(cookieID string, orders []RefreshSoldOrder, previous *OrderSyncCursor) (OrderSyncCursor, bool) {
	// cursor 保存当前已选中的最新订单边界。
	cursor := OrderSyncCursor{CookieID: cookieID}
	if previous != nil {
		cursor = *previous
		cursor.CookieID = cookieID
	}
	// found 表示至少存在一个可解析的平台创建时间。
	found := strings.TrimSpace(cursor.HighWaterCreatedAt) != ""
	// order 是当前待比较的平台订单。
	for _, order := range orders {
		// createdAt 保存平台订单时间的规范文本。
		createdAt := strings.TrimSpace(order.CreatedAt)
		if createdAt == "" {
			continue
		}
		// orderTime、orderErr 保存当前平台时间的解析结果。
		orderTime, orderErr := time.Parse(time.RFC3339, createdAt)
		if orderErr != nil {
			continue
		}
		// cursorTime、cursorErr 保存当前已选高水位的解析结果。
		cursorTime, cursorErr := time.Parse(time.RFC3339, cursor.HighWaterCreatedAt)
		if !found || cursorErr != nil || orderTime.After(cursorTime) || (orderTime.Equal(cursorTime) && strings.Compare(order.OrderID, cursor.HighWaterOrderID) > 0) {
			cursor.HighWaterCreatedAt = createdAt
			cursor.HighWaterOrderID = order.OrderID
			found = true
		}
	}
	return cursor, found
}

// persistSoldOrders 将平台订单列表写入数据库并统计变化。
func (s *RefreshService) persistSoldOrders(ctx context.Context, cookieID string, remoteOrders []RefreshSoldOrder) (int, int, int, map[string]struct{}, map[string]struct{}, error) {
	// discovered、updated、conflictSkipped 保存新增、变化和跨账号安全跳过数量。
	discovered, updated, conflictSkipped := 0, 0, 0
	// newOrderIDs、remoteOrderIDs 保存新增和远端订单标识集合。
	newOrderIDs := make(map[string]struct{})
	// remoteOrderIDs 保存远端订单标识集合。
	remoteOrderIDs := make(map[string]struct{})
	// normalizedRemoteOrders 保存去重并完成金额归一化的平台订单。
	normalizedRemoteOrders := make([]RefreshSoldOrder, 0, len(remoteOrders))
	// seenRemoteIDs 保存已经处理的平台订单标识。
	seenRemoteIDs := make(map[string]struct{}, len(remoteOrders))
	// remote 是当前平台订单列表项。
	for _, remote := range remoteOrders {
		remote.OrderID = strings.TrimSpace(remote.OrderID)
		if remote.OrderID == "" {
			continue
		}
		// exists 表示当前远端订单是否已经在本批次出现。
		if _, exists := seenRemoteIDs[remote.OrderID]; exists {
			continue
		}
		seenRemoteIDs[remote.OrderID] = struct{}{}
		remoteOrderIDs[remote.OrderID] = struct{}{}
		// normalizedAmount、ok 保存金额归一化结果。
		normalizedAmount, ok := NormalizeOrderAmount(remote.Amount)
		if ok {
			remote.Amount = normalizedAmount
		}
		normalizedRemoteOrders = append(normalizedRemoteOrders, remote)
	}
	if len(normalizedRemoteOrders) == 0 {
		return discovered, updated, conflictSkipped, newOrderIDs, remoteOrderIDs, nil
	}
	// remoteIDs 保存批量读取本地订单的标识集合。
	remoteIDs := make([]string, 0, len(normalizedRemoteOrders))
	// remote 是当前已归一化的平台订单。
	for _, remote := range normalizedRemoteOrders {
		remoteIDs = append(remoteIDs, remote.OrderID)
	}
	// existingOrders、findErr 保存批量读取的本地订单及错误。
	existingOrders, findErr := s.repository.FindOrdersByIDs(ctx, remoteIDs)
	if findErr != nil {
		return discovered, updated, conflictSkipped, newOrderIDs, remoteOrderIDs, fmt.Errorf("批量读取订单失败: %w", findErr)
	}
	// batchRows 保存订单发现阶段待一次性写入的订单。
	batchRows := make([]RefreshOrderWrite, 0, len(normalizedRemoteOrders))
	// remote 是当前待比较并写入的平台订单。
	for _, remote := range normalizedRemoteOrders {
		// existing、exists 保存当前订单的本地实体及存在标记。
		existing, exists := existingOrders[remote.OrderID]
		if exists && strings.TrimSpace(existing.CookieID) != "" && existing.CookieID != cookieID {
			// 同一订单已经归属其他账号时保持既有所有者，只隔离当前远端行，不能拖垮同批正常订单。
			conflictSkipped++
			continue
		}
		if exists && strings.TrimSpace(remote.CreatedAt) == "" {
			remote.CreatedAt = existing.CreatedAt
		}
		// status 保存待写入的订单状态。
		status := remote.OrderStatus
		if exists {
			status = ResolveOrderLifecycleStatusWithRefundContext(existing.OrderStatus, status, existing.RefundRequested)
		}
		remote.OrderStatus = status
		// changed 表示统一状态转换后的远端订单字段是否发生变化。
		changed := !exists || refreshSoldOrderChanged(existing, remote)
		// bargain 保存砍价订单标记指针。
		var bargain *bool
		if remote.IsBargain {
			// value 保存砍价订单标记值。
			value := true
			bargain = &value
		}
		// options 保存订单列表字段及明确生命周期状态的首次时间。
		options := UpsertOptions{ItemID: remote.ItemID, BuyerID: remote.BuyerID, CookieID: cookieID, OrderStatus: status, Quantity: remote.Quantity, Amount: remote.Amount, ReceiverName: remote.ReceiverName, ReceiverPhone: remote.ReceiverPhone, ReceiverAddress: remote.ReceiverAddr, ReceiverCity: remote.ReceiverCity, CreatedAt: remote.CreatedAt, IsBargain: bargain}
		ApplyOrderLifecycleMilestone(&options, status, time.Now().UTC())
		batchRows = append(batchRows, RefreshOrderWrite{OrderID: remote.OrderID, Options: options})
		if !exists {
			discovered++
			newOrderIDs[remote.OrderID] = struct{}{}
		} else if changed {
			updated++
		}
	}
	// err 保存订单发现批量写入错误。
	if err := s.repository.BatchUpsertOrders(ctx, batchRows); err != nil {
		return 0, 0, conflictSkipped, make(map[string]struct{}), remoteOrderIDs, fmt.Errorf("批量保存订单失败: %w", err)
	}
	return discovered, updated, conflictSkipped, newOrderIDs, remoteOrderIDs, nil
}

// refreshDetailChunk 刷新一个账号详情分片并提交单事务结果。
func (s *RefreshService) refreshDetailChunk(ctx context.Context, userID int64, cookieID string, targets []refreshTarget, onProcessed func(bool)) (int, int, int, []RefreshOrderResult, bool) {
	// results 保存当前详情分片结果。
	results := make([]RefreshOrderResult, 0, len(targets))
	// unlock 保存账号凭证锁释放函数。
	unlock := s.repository.LockCredentials(cookieID)
	// latest、err 保存详情请求前的平台视图及错误。
	latest, err := s.repository.LoadCookiePlatformDetail(ctx, cookieID)
	if err != nil || latest == nil || latest.UserID != userID || !s.runtime.CredentialAvailable(latest) {
		unlock()
		// target 是凭证失效时需要返回失败的详情目标。
		for _, target := range targets {
			results = append(results, RefreshOrderResult{CookieID: cookieID, OrderID: target.OrderID, Success: false, Message: "账号凭证已变化"})
			if onProcessed != nil {
				onProcessed(false)
			}
		}
		return 0, 0, len(targets), results, false
	}
	unlock()
	// detailCtx、cancel 限制本次详情分片外部请求时间。
	detailCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	// pendingWrites 保存成功获取、等待事务写入的订单详情。
	pendingWrites := make([]refreshWrite, 0, len(targets))
	// lastUpdate 保存详情分片最后一次平台响应的 Cookie 更新，即使该次详情请求失败也要尝试提交会话。
	lastUpdate := RefreshCookieUpdate{}
	// sessionErr 保存平台会话过期错误。
	var sessionErr error
	// target 是当前详情请求目标。
	for _, target := range targets {
		// fetchResult、fetchErr 保存当前订单详情结果及错误。
		fetchResult, fetchErr := s.runtime.FetchOrderDetail(detailCtx, latest, target.OrderID)
		if fetchResult.CookieUpdate.Changed || fetchResult.CookieUpdate.Handled {
			lastUpdate = fetchResult.CookieUpdate
		}
		if fetchErr != nil || fetchResult.Detail == nil {
			// message 保存当前详情失败的可读原因。
			message := "订单详情接口未返回结果"
			if fetchErr != nil {
				message = fetchErr.Error()
			}
			results = append(results, RefreshOrderResult{CookieID: cookieID, OrderID: target.OrderID, Success: false, Message: message})
			if onProcessed != nil {
				onProcessed(false)
			}
			if s.runtime.IsSessionExpired(fetchErr) {
				sessionErr = fetchErr
				break
			}
			continue
		}
		if target.RequireSpec && strings.TrimSpace(fetchResult.Detail.SpecValue) == "" {
			results = append(results, RefreshOrderResult{CookieID: cookieID, OrderID: target.OrderID, Success: false, Message: "订单详情未返回明确规格，保持未知成本"})
			if onProcessed != nil {
				onProcessed(false)
			}
			continue
		}
		if onProcessed != nil {
			onProcessed(true)
		}
		// newStatus 保存远端详情归一化后的状态。
		newStatus := ResolveOrderLifecycleStatusWithRefundContext(target.CurrentStatus, fetchResult.Detail.OrderStatus, target.RefundRequested)
		if !ValidEditableOrderStatus(newStatus) {
			newStatus = target.CurrentStatus
		}
		// options 保存详情字段和本轮明确生命周期状态对应的时间。
		options := UpsertOptions{CookieID: cookieID, OrderStatus: newStatus, SpecName: fetchResult.Detail.SpecName, SpecValue: fetchResult.Detail.SpecValue, Quantity: fetchResult.Detail.Quantity, Amount: fetchResult.Detail.Amount, CreatedAt: target.CreatedAt}
		ApplyOrderLifecycleMilestone(&options, newStatus, time.Now().UTC())
		pendingWrites = append(pendingWrites, refreshWrite{OrderID: target.OrderID, CurrentStatus: target.CurrentStatus, NewStatus: newStatus, Options: options, CookieUpdate: fetchResult.CookieUpdate})
	}
	// batchRows 保存详情分片等待一次性写入的订单记录。
	batchRows := make([]RefreshOrderWrite, 0, len(pendingWrites))
	// write 是当前详情分片待批量写入的订单详情。
	for _, write := range pendingWrites {
		batchRows = append(batchRows, RefreshOrderWrite{OrderID: write.OrderID, Options: write.Options})
	}
	// batchWriteErr 保存详情分片单条多值 UPSERT 错误。
	batchWriteErr := s.repository.BatchUpsertOrders(ctx, batchRows)
	// updated、noChange、failed 保存详情分片统计。
	updated, noChange, failed := 0, 0, 0
	// write 是当前统计对应的订单详情。
	for _, write := range pendingWrites {
		if batchWriteErr != nil {
			failed++
			results = append(results, RefreshOrderResult{CookieID: cookieID, OrderID: write.OrderID, Success: false, Message: "批量更新数据库失败"})
			continue
		}
		// changed 表示订单状态是否发生变化。
		changed := write.NewStatus != "" && write.NewStatus != write.CurrentStatus
		if changed {
			updated++
		} else {
			noChange++
		}
		results = append(results, RefreshOrderResult{CookieID: cookieID, OrderID: write.OrderID, Success: true, OldStatus: write.CurrentStatus, NewStatus: write.NewStatus})
	}
	cancel()
	unlock = s.repository.LockCredentials(cookieID)
	// latestAfterDetails、reloadErr 保存详情完成后的最新凭证视图及错误。
	latestAfterDetails, reloadErr := s.repository.LoadCookiePlatformDetail(ctx, cookieID)
	// credentialChanged 表示详情请求期间凭证快照是否发生变化。
	credentialChanged := reloadErr != nil || latestAfterDetails == nil || latestAfterDetails.UserID != userID || latestAfterDetails.Value != latest.Value || latestAfterDetails.MetadataJSON != latest.MetadataJSON
	if !credentialChanged {
		// valueChanged、persistErr 保存 Cookie 会话提交状态及错误。
		_, valueChanged, _, persistErr := s.runtime.PersistCookieSession(ctx, latest, lastUpdate)
		if persistErr != nil {
			failed++
			results = append(results, RefreshOrderResult{CookieID: cookieID, Stage: "persist_cookie", Success: false, Message: persistErr.Error()})
		} else if valueChanged && lastUpdate.Value != "" {
			s.runtime.UpdateRunningCookie(ctx, cookieID, lastUpdate.Value)
		}
	}
	unlock()
	if reloadErr != nil {
		failed += len(targets)
		results = append(results, RefreshOrderResult{CookieID: cookieID, Stage: "persist_cookie", Success: false, Message: "订单详情完成后账号凭证无法复核"})
	}
	if sessionErr != nil {
		s.runtime.RecoverExpiredSession(ctx, cookieID, sessionErr)
	}
	return updated, noChange, failed, results, sessionErr != nil
}
