package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"xianyu-go/internal/db"
)

const (
	// flowerReceiveWorkerCount 允许不同账号并行收花；同一账号仍由独立互斥锁串行。
	flowerReceiveWorkerCount = 2
	// flowerReceiveQueueSize 限制实时送花事件占用的内存队列长度。
	flowerReceiveQueueSize = 100
	// defaultFlowerReceiveTimeout 是旧设置或异常设置使用的安全等待时间。
	defaultFlowerReceiveTimeout = 120 * time.Second
)

// FlowerReceiveBrowser 定义自动收花打开官方页面所需的最小隔离浏览器能力。
type FlowerReceiveBrowser interface {
	// OpenRedFlowerReceive 打开官方收花页并阻塞到页面关闭或 Context 取消；opened 不表示已经收花。
	OpenRedFlowerReceive(ctx context.Context, accountID, cookieStr, orderID string, showBrowser bool) (opened bool, err error)
}

// flowerReceiveJob 保存送花事件进入后台 worker 后所需的非敏感关联字段。
type flowerReceiveJob struct {
	// RunKey 是每账号、每订单唯一的自动收花幂等键。
	RunKey string
	// AccountID 是卖家账号标识。
	AccountID string
	// OrderID 是送花卡片关联订单标识。
	OrderID string
	// ChatID 是等待收花结果的会话标识。
	ChatID string
	// BuyerID 是通知链路使用的买家标识。
	BuyerID string
	// ItemID 是通知链路使用的商品标识。
	ItemID string
}

// flowerReceivePending 保存一个已排队或执行中的收花结果信号。
type flowerReceivePending struct {
	// RunKey 标识当前会话等待的是哪次订单收花。
	RunKey string
	// Received 在 WS 确认收花后关闭。
	Received chan struct{}
	// receiveOnce 保证重复平台结果只关闭一次信号。
	receiveOnce sync.Once
}

// flowerBrowserOutcome 保存隔离浏览器调用是否开始导航以及最终错误。
type flowerBrowserOutcome struct {
	// Opened 表示官方页面导航已经开始，不能把后续错误视为确定未执行。
	Opened bool
	// Err 是浏览器初始化、导航、页面关闭或 Context 取消结果。
	Err error
}

// flowerAutomationCoordinator 拥有自动收花队列、worker、同账号串行锁和 WS 结果协调状态。
type flowerAutomationCoordinator struct {
	// repository 提供账号设置、状态、凭证和幂等运行持久化能力。
	repository AccountTaskRepository
	// browser 在独立 Chromium 中打开固定官方收花页面。
	browser FlowerReceiveBrowser
	// notifier 将终态写入现有持久化通知去重链路。
	notifier Notifier
	// logger 记录不含 Cookie 和完整页面地址的诊断信息。
	logger interface {
		Info(string, ...any)
		Warn(string, ...any)
	}
	// queue 是实时送花事件到后台 worker 的有界通道，发送方不关闭。
	queue chan flowerReceiveJob
	// stateMu 保护 started、closed、workerCtx 和 cancel。
	stateMu sync.Mutex
	// started 表示 worker 已经由进程生命周期启动。
	started bool
	// closed 表示协调器已经进入永久关闭状态。
	closed bool
	// workerCtx 是协调器拥有的 worker 根 Context。
	workerCtx context.Context
	// cancel 取消全部排队和执行中的收花工作。
	cancel context.CancelFunc
	// workers 等待所有 worker 和其同步浏览器调用收束。
	workers sync.WaitGroup
	// done 在全部 worker 退出后关闭。
	done chan struct{}
	// accountLocksMu 保护按账号创建的串行互斥锁映射。
	accountLocksMu sync.Mutex
	// accountLocks 确保同一账号不会同时打开多个收花页面。
	accountLocks map[string]*sync.Mutex
	// eventMu 保护 awaiting、earlyReceived 和 pending 的事件先后协调。
	eventMu sync.Mutex
	// awaiting 保存已经抢占但尚未完成的会话到运行键映射。
	awaiting map[string]string
	// earlyReceived 保存 worker 注册 pending 前已经到达的结果运行键。
	earlyReceived map[string]struct{}
	// pending 保存正在等待 WS 收花结果的会话信号。
	pending map[string]*flowerReceivePending
}

// newFlowerAutomationCoordinator 构造尚未启动的自动收花生命周期组件。
func newFlowerAutomationCoordinator(repository AccountTaskRepository, browser FlowerReceiveBrowser, notifier Notifier, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
}) *flowerAutomationCoordinator {
	return &flowerAutomationCoordinator{
		repository: repository, browser: browser, notifier: notifier, logger: logger,
		queue: make(chan flowerReceiveJob, flowerReceiveQueueSize), done: make(chan struct{}),
		accountLocks: make(map[string]*sync.Mutex), awaiting: make(map[string]string),
		earlyReceived: make(map[string]struct{}), pending: make(map[string]*flowerReceivePending),
	}
}

// StartFlowerAutomation 启动自动收花 worker；由进程生命周期协调器唯一调用。
func (c *Center) StartFlowerAutomation(ctx context.Context) error {
	if c == nil || c.flowers == nil {
		return nil
	}
	return c.flowers.Start(ctx)
}

// CloseFlowerAutomation 取消自动收花 worker 并等待隔离浏览器调用退出。
func (c *Center) CloseFlowerAutomation(ctx context.Context) error {
	if c == nil || c.flowers == nil {
		return nil
	}
	return c.flowers.Close(ctx)
}

// Start 启动固定数量 worker，并先隔离上次进程遗留的未知小红花动作。
func (coordinator *flowerAutomationCoordinator) Start(ctx context.Context) error {
	if coordinator == nil || coordinator.repository == nil {
		return errors.New("自动小红花协调器未初始化")
	}
	if ctx == nil {
		return errors.New("自动小红花协调器需要进程 Context")
	}
	coordinator.stateMu.Lock()
	if coordinator.closed {
		coordinator.stateMu.Unlock()
		return errors.New("自动小红花协调器已经关闭")
	}
	if coordinator.started {
		coordinator.stateMu.Unlock()
		return nil
	}
	// quarantined、quarantineErr 是上次进程中断时遗留的未知动作数量和隔离错误。
	quarantined, quarantineErr := coordinator.repository.QuarantineRunningRuns(ctx, []string{TaskRedFlowerRequest, TaskRedFlowerReceive, TaskReceiptReminder}, "进程在账号消息自动化外部动作期间中断，结果未知，已停止自动重放，请人工核对")
	if quarantineErr != nil {
		coordinator.stateMu.Unlock()
		return fmt.Errorf("隔离历史小红花运行: %w", quarantineErr)
	}
	// workerCtx、cancel 是由协调器拥有并在 Close 中取消的后台生命周期。
	workerCtx, cancel := context.WithCancel(ctx)
	coordinator.workerCtx = workerCtx
	coordinator.cancel = cancel
	coordinator.started = true
	coordinator.workers.Add(flowerReceiveWorkerCount)
	coordinator.stateMu.Unlock()
	// workerIndex 是当前启动的固定 worker 序号，仅用于闭包参数隔离。
	for workerIndex := 0; workerIndex < flowerReceiveWorkerCount; workerIndex++ {
		go coordinator.runWorker(workerCtx)
	}
	go func() {
		coordinator.workers.Wait()
		close(coordinator.done)
	}()
	if quarantined > 0 && coordinator.logger != nil {
		coordinator.logger.Warn("已隔离进程中断遗留的小红花运行", "count", quarantined)
	}
	return nil
}

// Close 取消 worker、等待浏览器调用退出，并把尚未开始的队列任务恢复为可重试失败。
func (coordinator *flowerAutomationCoordinator) Close(ctx context.Context) error {
	if coordinator == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("关闭自动小红花协调器需要 Context")
	}
	coordinator.stateMu.Lock()
	if !coordinator.started {
		if !coordinator.closed {
			coordinator.closed = true
			close(coordinator.done)
		}
		coordinator.stateMu.Unlock()
		return nil
	}
	// cancel 是协调器持有的根取消函数；重复关闭只会重复调用幂等取消。
	cancel := coordinator.cancel
	coordinator.closed = true
	coordinator.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-coordinator.done:
		return coordinator.failQueuedJobs(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HandleTask 接收已经分类好的小红花系统事件，不参与普通自动化规则匹配。
func (coordinator *flowerAutomationCoordinator) HandleTask(ctx context.Context, task Task) error {
	if coordinator == nil {
		return nil
	}
	switch task.TriggerType {
	case TriggerRedFlowerSent:
		return coordinator.handleSent(ctx, task)
	case TriggerRedFlowerReceived:
		coordinator.handleReceived(task)
	}
	return nil
}

// handleSent 校验账号设置并原子抢占一次自动收花任务后放入有界队列。
func (coordinator *flowerAutomationCoordinator) handleSent(ctx context.Context, task Task) error {
	if coordinator.repository == nil {
		return errors.New("自动收花存储未初始化")
	}
	if strings.TrimSpace(task.AccountID) == "" || strings.TrimSpace(task.ChatID) == "" || strings.TrimSpace(task.OrderID) == "" {
		return errors.New("自动收花事件缺少账号、会话或订单关联")
	}
	// settings、settingsErr 是当前账号小红花开关和执行参数。
	settings, settingsErr := coordinator.repository.Get(ctx, task.AccountID)
	if settingsErr != nil {
		return settingsErr
	}
	if !settings.AutoReceiveFlowerEnabled {
		return nil
	}
	if coordinator.browser == nil {
		return errors.New("自动收花浏览器未启用")
	}
	// allowed、allowErr 是账号启用且未暂停的门禁结果。
	allowed, allowErr := coordinator.accountAllowed(ctx, task.AccountID)
	if allowErr != nil || !allowed {
		return allowErr
	}
	// runKey 是每账号、每订单唯一的自动收花运行键。
	runKey := TaskRedFlowerReceive + ":" + task.AccountID + ":" + task.OrderID
	// now 是运行抢占和失败重试共用的 UTC 时间。
	now := time.Now().UTC()
	// claimed、claimErr 是首次或到期失败运行的原子抢占结果。
	claimed, claimErr := coordinator.repository.ClaimRun(ctx, db.AccountTaskRun{
		RunKey: runKey, CookieID: task.AccountID, TaskType: TaskRedFlowerReceive,
		TargetID: task.OrderID, RunDate: now.Format("2006-01-02"),
	}, now.Unix())
	if claimErr != nil || !claimed {
		return claimErr
	}
	// job 是进入后台 worker 的最小非敏感事件快照。
	job := flowerReceiveJob{RunKey: runKey, AccountID: task.AccountID, OrderID: task.OrderID, ChatID: task.ChatID, BuyerID: task.BuyerID, ItemID: task.ItemID}
	// eventKey 将账号和会话组合，避免不同账号相同 sid 互相干扰。
	eventKey := flowerReceiveEventKey(task.AccountID, task.ChatID)
	coordinator.eventMu.Lock()
	coordinator.awaiting[eventKey] = runKey
	coordinator.eventMu.Unlock()
	// workerCtx 是当前已启动协调器的根 Context。
	workerCtx, started := coordinator.currentWorkerContext()
	if !started {
		coordinator.removeAwaiting(eventKey, runKey)
		_ = coordinator.repository.FinishRun(ctx, runKey, "failed", 0, 1, "自动收花 worker 尚未启动", now.Add(time.Minute).Unix())
		return errors.New("自动收花 worker 尚未启动")
	}
	select {
	case coordinator.queue <- job:
		return nil
	case <-ctx.Done():
		coordinator.removeAwaiting(eventKey, runKey)
		_ = coordinator.repository.FinishRun(context.WithoutCancel(ctx), runKey, "failed", 0, 1, "收花事件入队前请求已取消", now.Add(time.Minute).Unix())
		return ctx.Err()
	case <-workerCtx.Done():
		coordinator.removeAwaiting(eventKey, runKey)
		_ = coordinator.repository.FinishRun(context.WithoutCancel(ctx), runKey, "failed", 0, 1, "自动收花 worker 已停止", now.Add(time.Minute).Unix())
		return errors.New("自动收花 worker 已停止")
	default:
		coordinator.removeAwaiting(eventKey, runKey)
		_ = coordinator.repository.FinishRun(ctx, runKey, "failed", 0, 1, "自动收花队列已满", now.Add(time.Minute).Unix())
		return errors.New("自动收花队列已满")
	}
}

// handleReceived 把平台已收花结果交给同账号同会话的执行中或已排队任务。
func (coordinator *flowerAutomationCoordinator) handleReceived(task Task) {
	// eventKey 是账号和会话共同组成的结果匹配键。
	eventKey := flowerReceiveEventKey(task.AccountID, task.ChatID)
	if eventKey == "" {
		return
	}
	coordinator.eventMu.Lock()
	// pending 是已经注册并正在等待结果的收花任务。
	pending := coordinator.pending[eventKey]
	if pending != nil {
		coordinator.eventMu.Unlock()
		pending.receiveOnce.Do(func() { close(pending.Received) })
		return
	}
	// runKey 是已抢占但尚未注册 pending 的排队任务；没有自动任务时忽略人工或历史结果。
	runKey := coordinator.awaiting[eventKey]
	if runKey != "" {
		coordinator.earlyReceived[runKey] = struct{}{}
	}
	coordinator.eventMu.Unlock()
}

// runWorker 领取收花任务并确保同一账号的浏览器操作严格串行。
func (coordinator *flowerAutomationCoordinator) runWorker(ctx context.Context) {
	defer coordinator.workers.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case job := <-coordinator.queue: // job 是当前 worker 领取的自动收花任务。
			// accountLock 是当前账号所有自动收花页面共用的串行锁。
			accountLock := coordinator.accountLock(job.AccountID)
			accountLock.Lock()
			coordinator.processJob(ctx, job)
			accountLock.Unlock()
		}
	}
}

// processJob 打开隔离官方页面并以 WS 收花结果而不是页面状态判断成功。
func (coordinator *flowerAutomationCoordinator) processJob(ctx context.Context, job flowerReceiveJob) {
	// eventKey 是本次运行在实时结果协调表中的键。
	eventKey := flowerReceiveEventKey(job.AccountID, job.ChatID)
	defer coordinator.removeAwaiting(eventKey, job.RunKey)
	// settings、settingsErr 是执行前重读的最新账号参数。
	settings, settingsErr := coordinator.repository.Get(ctx, job.AccountID)
	if settingsErr != nil || !settings.AutoReceiveFlowerEnabled {
		coordinator.finishJob(ctx, job, "failed", 0, 1, firstFlowerError(settingsErr, "自动收花已关闭"), time.Now().UTC().Add(10*time.Minute).Unix())
		return
	}
	// allowed、allowErr 是执行外部动作前的最终账号门禁。
	allowed, allowErr := coordinator.accountAllowed(ctx, job.AccountID)
	if allowErr != nil || !allowed {
		coordinator.finishJob(ctx, job, "failed", 0, 1, firstFlowerError(allowErr, "账号已停用或暂停"), time.Now().UTC().Add(10*time.Minute).Unix())
		return
	}
	// cookieStr、cookieErr 是本轮隔离浏览器使用的最新平面 Cookie。
	cookieStr, cookieErr := coordinator.repository.GetValue(ctx, job.AccountID)
	if cookieErr != nil || strings.TrimSpace(cookieStr) == "" {
		coordinator.finishJob(ctx, job, "failed", 0, 1, firstFlowerError(cookieErr, "账号 Cookie 为空"), time.Now().UTC().Add(10*time.Minute).Unix())
		return
	}
	// timeout 是经过安全范围归一化的 WS 结果等待时间。
	timeout := time.Duration(settings.ReceiveFlowerTimeoutSeconds) * time.Second
	if timeout < 30*time.Second || timeout > 10*time.Minute {
		timeout = defaultFlowerReceiveTimeout
	}
	// operationCtx、cancel 同时控制官方页面和结果等待；函数返回时必须释放定时器。
	operationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// pending 是当前会话正在等待的唯一收花结果信号。
	pending := &flowerReceivePending{RunKey: job.RunKey, Received: make(chan struct{})}
	coordinator.registerPending(eventKey, pending)
	defer coordinator.unregisterPending(eventKey, pending)
	// browserDone 保存隔离浏览器调用结果；容量一避免 worker 超时后阻塞写回。
	browserDone := make(chan flowerBrowserOutcome, 1)
	go func() {
		// opened、browserErr 是页面是否开始导航及浏览器最终错误。
		opened, browserErr := coordinator.browser.OpenRedFlowerReceive(operationCtx, job.AccountID, cookieStr, job.OrderID, settings.ReceiveFlowerShowBrowser)
		browserDone <- flowerBrowserOutcome{Opened: opened, Err: browserErr}
	}()
	// browserResult 保存已经结束的浏览器结果；打开后页面关闭仍继续等待 WS 到超时。
	browserResult := flowerBrowserOutcome{}
	// browserResultReady 表示浏览器调用已经退出，不再从通道读取。
	browserResultReady := false
	for {
		select {
		case <-pending.Received:
			cancel()
			if !browserResultReady {
				browserResult = <-browserDone
			}
			coordinator.finishJob(context.WithoutCancel(ctx), job, "success", 1, 0, "", 0)
			return
		case result := <-browserDone: // result 是隔离浏览器已经结束的页面状态。
			browserResult = result
			browserResultReady = true
			browserDone = nil
			if !result.Opened {
				coordinator.finishJob(context.WithoutCancel(ctx), job, "failed", 0, 1, firstFlowerError(result.Err, "自动收花浏览器未能打开"), time.Now().UTC().Add(10*time.Minute).Unix())
				return
			}
			// 页面已经发起远端导航；关闭或导航错误都不能确认动作未执行，继续等 WS 终态。
		case <-operationCtx.Done():
			cancel()
			if !browserResultReady {
				browserResult = <-browserDone
			}
			if !browserResult.Opened && errors.Is(operationCtx.Err(), context.Canceled) {
				coordinator.finishJob(context.WithoutCancel(ctx), job, "failed", 0, 1, "进程停止前尚未打开自动收花页面", time.Now().UTC().Add(10*time.Minute).Unix())
				return
			}
			coordinator.finishJob(context.WithoutCancel(ctx), job, "needs_review", 0, 1, "自动收花页面已打开，但未在超时前检测到平台收花结果，请人工核对", 0)
			return
		}
	}
}

// finishJob 保存自动收花终态，并在外部结果落库失败时降级为人工核对。
func (coordinator *flowerAutomationCoordinator) finishJob(ctx context.Context, job flowerReceiveJob, status string, success, failed int, message string, nextRetryAt int64) {
	// finishCtx、cancel 为已取消 worker Context 提供独立五秒持久化预算。
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	// finishErr 是预期终态写入错误。
	finishErr := coordinator.repository.FinishRun(finishCtx, job.RunKey, status, success, failed, message, nextRetryAt)
	if finishErr != nil {
		// quarantineMessage 说明外部动作可能已经执行但本地状态未能正常收口。
		quarantineMessage := "自动收花结果保存失败，外部动作可能已经执行，已停止自动重放，请人工核对: " + finishErr.Error()
		// quarantineCtx、quarantineCancel 为第二次隔离写入提供独立五秒预算。
		quarantineCtx, quarantineCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		// quarantineErr 是人工核对状态的补偿写入错误。
		quarantineErr := coordinator.repository.FinishRun(quarantineCtx, job.RunKey, "needs_review", success, failed, quarantineMessage, 0)
		quarantineCancel()
		if coordinator.logger != nil {
			coordinator.logger.Warn("自动收花运行状态保存失败", "account", job.AccountID, "order_id", job.OrderID, "err", errors.Join(finishErr, quarantineErr))
		}
		status = "needs_review"
		message = quarantineMessage
	}
	coordinator.notifyJob(finishCtx, job, status, message)
}

// notifyJob 将已持久化的自动收花终态交给现有通知去重链路。
func (coordinator *flowerAutomationCoordinator) notifyJob(ctx context.Context, job flowerReceiveJob, status, message string) {
	if coordinator.notifier == nil {
		return
	}
	// run、exists、runErr 是通知去重所需的运行记录和读取结果。
	run, exists, runErr := coordinator.repository.GetRunByKey(ctx, job.RunKey)
	if runErr != nil || !exists {
		if coordinator.logger != nil {
			coordinator.logger.Warn("读取自动收花通知运行失败", "account", job.AccountID, "order_id", job.OrderID, "err", runErr)
		}
		return
	}
	// notificationMessage 是成功状态缺省时使用的用户可见说明。
	notificationMessage := message
	if status == "success" && notificationMessage == "" {
		notificationMessage = "已检测到闲鱼收花结果，自动收花完成"
	}
	coordinator.notifier.NotifyAutomationRun(ctx, run.ID, job.AccountID, job.BuyerID, job.ItemID, status, notificationMessage, job.ChatID)
}

// accountAllowed 判断账号是否启用且不处于临时暂停期。
func (coordinator *flowerAutomationCoordinator) accountAllowed(ctx context.Context, accountID string) (bool, error) {
	// paused、pauseErr 是账号暂停状态和读取错误。
	paused, _, pauseErr := coordinator.repository.IsPaused(ctx, accountID)
	if pauseErr != nil {
		return false, pauseErr
	}
	// enabled、statusErr 是账号启用状态和读取错误。
	enabled, statusErr := coordinator.repository.Status(ctx, accountID)
	if statusErr != nil {
		return false, statusErr
	}
	return enabled && !paused, nil
}

// currentWorkerContext 返回已启动且未关闭的 worker Context。
func (coordinator *flowerAutomationCoordinator) currentWorkerContext() (context.Context, bool) {
	coordinator.stateMu.Lock()
	defer coordinator.stateMu.Unlock()
	return coordinator.workerCtx, coordinator.started && !coordinator.closed && coordinator.workerCtx != nil
}

// accountLock 返回当前账号长期复用的自动收花串行锁。
func (coordinator *flowerAutomationCoordinator) accountLock(accountID string) *sync.Mutex {
	coordinator.accountLocksMu.Lock()
	defer coordinator.accountLocksMu.Unlock()
	// lock 是当前账号已存在或新创建的互斥锁。
	lock := coordinator.accountLocks[accountID]
	if lock == nil {
		lock = &sync.Mutex{}
		coordinator.accountLocks[accountID] = lock
	}
	return lock
}

// registerPending 注册结果等待信号，并消费可能先到的 WS 收花结果。
func (coordinator *flowerAutomationCoordinator) registerPending(eventKey string, pending *flowerReceivePending) {
	coordinator.eventMu.Lock()
	coordinator.pending[eventKey] = pending
	// _, arrived 表示同一运行的结果是否在 worker 注册前已经到达。
	_, arrived := coordinator.earlyReceived[pending.RunKey]
	delete(coordinator.earlyReceived, pending.RunKey)
	coordinator.eventMu.Unlock()
	if arrived {
		pending.receiveOnce.Do(func() { close(pending.Received) })
	}
}

// unregisterPending 仅移除仍属于当前运行的会话等待信号。
func (coordinator *flowerAutomationCoordinator) unregisterPending(eventKey string, pending *flowerReceivePending) {
	coordinator.eventMu.Lock()
	if coordinator.pending[eventKey] == pending {
		delete(coordinator.pending, eventKey)
	}
	coordinator.eventMu.Unlock()
}

// removeAwaiting 清理仍属于当前运行的排队标记和提前结果。
func (coordinator *flowerAutomationCoordinator) removeAwaiting(eventKey, runKey string) {
	coordinator.eventMu.Lock()
	if coordinator.awaiting[eventKey] == runKey {
		delete(coordinator.awaiting, eventKey)
	}
	delete(coordinator.earlyReceived, runKey)
	coordinator.eventMu.Unlock()
}

// failQueuedJobs 在 worker 退出后把未开始浏览器动作的任务恢复为可重试失败。
func (coordinator *flowerAutomationCoordinator) failQueuedJobs(ctx context.Context) error {
	// resultErr 聚合关闭时队列任务状态写入错误。
	var resultErr error
	for {
		select {
		case job := <-coordinator.queue: // job 是关闭时尚未被 worker 领取的排队任务。
			coordinator.removeAwaiting(flowerReceiveEventKey(job.AccountID, job.ChatID), job.RunKey)
			// finishErr 是确认未打开页面的排队任务失败状态写入结果。
			finishErr := coordinator.repository.FinishRun(ctx, job.RunKey, "failed", 0, 1, "进程关闭前尚未开始自动收花", time.Now().UTC().Add(time.Minute).Unix())
			resultErr = errors.Join(resultErr, finishErr)
		default:
			return resultErr
		}
	}
}

// flowerReceiveEventKey 使用账号和会话构造内存结果匹配键。
func flowerReceiveEventKey(accountID, chatID string) string {
	// normalizedAccountID、normalizedChatID 是去空白后的账号与会话标识。
	normalizedAccountID, normalizedChatID := strings.TrimSpace(accountID), strings.TrimSpace(chatID)
	if normalizedAccountID == "" || normalizedChatID == "" {
		return ""
	}
	return normalizedAccountID + "\x00" + normalizedChatID
}

// firstFlowerError 优先返回具体错误文本，否则使用安全缺省说明。
func firstFlowerError(err error, fallback string) string {
	if err != nil {
		return err.Error()
	}
	return fallback
}
