package orders

import (
	"context"
	"errors"
)

// ErrRefreshJobNotFound 表示当前用户无法读取指定订单刷新任务。
var ErrRefreshJobNotFound = errors.New("订单刷新任务不存在")

// RefreshJob 是订单刷新后台任务的应用层模型，不暴露数据库类型。
type RefreshJob struct {
	// ID 是任务唯一标识。
	ID string
	// UserID 是任务所属用户标识。
	UserID int64
	// CookieID 是可选的目标账号标识。
	CookieID string
	// FilterStatus 是订单状态筛选条件。
	FilterStatus string
	// SyncMode 是 incremental 或 full，任务恢复后必须保持创建时的同步范围。
	SyncMode string
	// Status 是 queued/running/succeeded/failed/cancelled 之一。
	Status string
	// ResultJSON 保存成功后的具名刷新结果 JSON。
	ResultJSON string
	// ErrorMessage 保存任务失败原因。
	ErrorMessage string
	// WorkerToken 是当前执行者租约令牌。
	WorkerToken string
	// LeaseExpiresAt 是租约过期 Unix 秒时间戳。
	LeaseExpiresAt int64
	// CreatedAt 是任务创建时间。
	CreatedAt string
	// UpdatedAt 是任务最后更新时间。
	UpdatedAt string
}

// RefreshJobRepository 定义订单刷新任务需要的持久化能力。
type RefreshJobRepository interface {
	// Create 创建一个 queued 状态的订单刷新任务。
	Create(ctx context.Context, job *RefreshJob) error
	// Get 按用户读取订单刷新任务。
	Get(ctx context.Context, userID int64, id string) (*RefreshJob, error)
	// Claim 原子抢占 queued 任务并写入租约令牌。
	Claim(ctx context.Context, id, token string, leaseExpiresAt int64) (bool, error)
	// Cancel 按用户归属原子取消 queued 或 running 任务。
	Cancel(ctx context.Context, userID int64, id string) (bool, error)
	// Complete 在租约令牌匹配时写入任务终态。
	Complete(ctx context.Context, id, token, status, resultJSON, errorMessage string) (bool, error)
	// UpdateProgress 在 running 状态和租约令牌匹配时更新轻量进度 JSON，不改变任务终态。
	UpdateProgress(ctx context.Context, id, token, progressJSON string) (bool, error)
	// Recoverable 返回租约已过期的 running 任务。
	Recoverable(ctx context.Context, now int64, limit int) ([]RefreshJob, error)
	// RequeueExpired 将过期运行任务恢复为 queued。
	RequeueExpired(ctx context.Context, id string, now int64) (bool, error)
}

// RefreshJobProgressReporter 接收订单刷新 worker 的轻量进度快照；实现不得阻塞业务刷新或泄露订单详情。
type RefreshJobProgressReporter func(RefreshJobProgress)

// RefreshJobProgress 是运行中任务返回给管理界面的稳定进度模型。
type RefreshJobProgress struct {
	// Stage 是 discovering、preparing、syncing_details、finalizing 或 completed 之一。
	Stage string `json:"stage"`
	// Message 是当前阶段面向用户的简短说明，不包含订单号、Cookie 或平台凭证。
	Message string `json:"message"`
	// Processed 是当前阶段已经处理的账号或订单数量。
	Processed int `json:"processed"`
	// Total 是当前阶段需要处理的账号或订单总数；尚未确定时为零。
	Total int `json:"total"`
	// Succeeded 是详情阶段成功读取的订单数，或发现阶段成功处理的账号数。
	Succeeded int `json:"succeeded"`
	// Failed 是当前阶段失败的账号或订单数量。
	Failed int `json:"failed"`
	// Percent 是跨阶段加权后的整体进度，范围为 0 到 100。
	Percent int `json:"percent"`
	// CurrentPage 是平台订单列表当前已完成页码；非分页阶段为零。
	CurrentPage int `json:"current_page,omitempty"`
	// TotalPages 是当前账号平台订单列表总页数；未知时为零。
	TotalPages int `json:"total_pages,omitempty"`
	// CurrentAccount 是当前正在同步的账号序号，从 1 开始；非账号阶段为零。
	CurrentAccount int `json:"current_account,omitempty"`
	// TotalAccounts 是本次任务需要同步的账号总数。
	TotalAccounts int `json:"total_accounts,omitempty"`
	// Mode 是 incremental 或 full，供界面区分增量同步和全量校准。
	Mode string `json:"mode,omitempty"`
	// BoundaryMatched 是增量扫描连续命中的历史边界订单数量。
	BoundaryMatched int `json:"boundary_matched,omitempty"`
	// BoundaryRequired 是增量扫描允许停止所需的连续历史订单数量。
	BoundaryRequired int `json:"boundary_required,omitempty"`
}
