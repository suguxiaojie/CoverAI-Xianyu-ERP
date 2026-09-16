// Package automation 提供自动化业务用例的纯应用层编排。
package automation

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

// ErrUnavailable 表示账号任务运行所需的自动化中心尚未装配。
var ErrUnavailable = errors.New("自动化中心未启用")

// ErrInvalidTaskType 表示请求未使用受支持的账号任务类型。
var ErrInvalidTaskType = errors.New("不支持的任务类型")

// TaskAutoRate 表示自动评价任务。
const TaskAutoRate = "auto_rate"

// TaskAutoPolish 表示商品擦亮任务。
const TaskAutoPolish = "auto_polish"

// accountTaskTimePattern 校验擦亮任务每天执行的本地时间格式。
var accountTaskTimePattern = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)

// AccountTaskSettings 是不含数据库模型的账号任务设置。
type AccountTaskSettings struct {
	// CookieID 是设置所属的账号标识。
	CookieID string `json:"account_id"`
	// AutoRateEnabled 表示是否启用自动评价。
	AutoRateEnabled bool `json:"auto_rate_enabled"`
	// RateContent 是自动评价使用的文字内容。
	RateContent string `json:"rate_content"`
	// AutoPolishEnabled 表示是否启用商品擦亮。
	AutoPolishEnabled bool `json:"auto_polish_enabled"`
	// PolishTime 是每天执行擦亮的本地时间，格式为 HH:mm。
	PolishTime string `json:"polish_time"`
	// AutoRequestFlowerEnabled 表示是否在明确发货成功后自动向买家求花。
	AutoRequestFlowerEnabled bool `json:"auto_request_flower_enabled"`
	// RequestFlowerAfterHours 是 schema 37 旧客户端保留的小时配置，新调度只使用秒字段。
	RequestFlowerAfterHours int `json:"request_flower_after_hours"`
	// RequestFlowerAfterSeconds 是订单明确发货成功后等待多少秒再自动求花。
	RequestFlowerAfterSeconds int `json:"request_flower_after_seconds"`
	// AutoReceiveFlowerEnabled 表示收到买家送花卡片后是否自动打开官方收花页。
	AutoReceiveFlowerEnabled bool `json:"auto_receive_flower_enabled"`
	// ReceiveFlowerShowBrowser 表示自动收花时是否显示独立 Chromium 窗口。
	ReceiveFlowerShowBrowser bool `json:"receive_flower_show_browser"`
	// ReceiveFlowerTimeoutSeconds 是等待平台已收花 WS 结果的最长秒数。
	ReceiveFlowerTimeoutSeconds int `json:"receive_flower_timeout_seconds"`
	// AutoReceiptReminderEnabled 表示是否启用发货后确认收货提醒。
	AutoReceiptReminderEnabled bool `json:"auto_receipt_reminder_enabled"`
	// ReceiptReminderAfterDays 是至少等待的完整发货天数。
	ReceiptReminderAfterDays int `json:"receipt_reminder_after_days"`
	// ReceiptReminderTime 是达到门槛后下一次北京时间执行时刻。
	ReceiptReminderTime string `json:"receipt_reminder_time"`
	// ReceiptReminderMessage 是 Schema 53 普通文字候选遗留的兼容字段；官方系统卡片由闲鱼固定生成。
	ReceiptReminderMessage string `json:"receipt_reminder_message"`
	// ReceiptReminderEnabledAt 是最近一次开启时间；只由服务端维护历史任务基线。
	ReceiptReminderEnabledAt int64 `json:"receipt_reminder_enabled_at"`
	// LastRateScanAt 是上次扫描待评价订单的 Unix 秒时间。
	LastRateScanAt int64 `json:"last_rate_scan_at"`
	// LastPolishDate 是上次擦亮日期，使用 YYYY-MM-DD 格式。
	LastPolishDate string `json:"last_polish_date"`
	// LastPolishAt 是上次擦亮完成的 Unix 秒时间。
	LastPolishAt int64 `json:"last_polish_at"`
}

// AccountTaskRun 是面向应用层的账号任务运行记录。
type AccountTaskRun struct {
	// ID 是运行记录的持久化标识。
	ID int64 `json:"id"`
	// RunKey 是用于幂等去重的运行键。
	RunKey string `json:"run_key"`
	// CookieID 是运行所属的账号标识。
	CookieID string `json:"account_id"`
	// TaskType 是账号任务类型。
	TaskType string `json:"task_type"`
	// TargetID 是本次运行对应的平台对象标识。
	TargetID string `json:"target_id"`
	// RunDate 是任务业务日期。
	RunDate string `json:"run_date"`
	// Status 是运行当前状态。
	Status string `json:"status"`
	// SuccessCount 是外部动作成功数量。
	SuccessCount int `json:"success_count"`
	// FailedCount 是外部动作失败数量。
	FailedCount int `json:"failed_count"`
	// ErrorMessage 是供人工核对的非敏感错误摘要。
	ErrorMessage string `json:"error_message"`
	// NextRetryAt 是下一次重试的 Unix 秒时间。
	NextRetryAt int64 `json:"next_retry_at"`
	// StartedAt 是运行开始的 Unix 秒时间。
	StartedAt int64 `json:"started_at"`
	// FinishedAt 是运行结束的 Unix 秒时间。
	FinishedAt int64 `json:"finished_at"`
}

// TaskSummary 是手动执行账号任务后的统计结果。
type TaskSummary struct {
	// TaskType 是本次执行的任务类型。
	TaskType string `json:"task_type"`
	// Found 是发现的待处理对象数量。
	Found int `json:"found"`
	// Success 是外部动作成功数量。
	Success int `json:"success"`
	// Failed 是外部动作失败数量。
	Failed int `json:"failed"`
	// Skipped 是因状态或幂等检查而跳过的数量。
	Skipped int `json:"skipped"`
	// Message 是面向操作人员的非敏感执行说明。
	Message string `json:"message,omitempty"`
}

// Repository 定义账号任务设置和历史记录所需的最小持久化能力。
type Repository interface {
	// GetSettings 读取账号任务设置，未创建设置时返回默认值。
	GetSettings(ctx context.Context, accountID string) (AccountTaskSettings, error)
	// SaveSettings 保存账号任务设置。
	SaveSettings(ctx context.Context, settings AccountTaskSettings) error
	// ListRuns 返回账号最近的任务运行记录。
	ListRuns(ctx context.Context, accountID string, limit int) ([]AccountTaskRun, error)
}

// Runner 定义手动触发账号任务所需的运行能力。
type Runner interface {
	// RunAccountTask 执行指定账号的一个自动化任务。
	RunAccountTask(ctx context.Context, accountID, taskType string) (TaskSummary, error)
}

// Service 编排账号任务的设置、历史和手动执行用例。
type Service struct {
	// repository 提供账号任务持久化能力。
	repository Repository
	// runner 提供自动化中心的手动执行能力。
	runner Runner
	// now 返回设置变更时的当前 UTC 时间，测试可替换为固定时钟。
	now func() time.Time
}

// NewService 构造账号任务应用服务。
func NewService(repository Repository, runner Runner) *Service {
	return &Service{repository: repository, runner: runner, now: time.Now}
}

// GetSettings 读取指定账号的账号任务设置。
func (s *Service) GetSettings(ctx context.Context, accountID string) (AccountTaskSettings, error) {
	if s == nil || s.repository == nil {
		return AccountTaskSettings{}, errors.New("账号任务存储未初始化")
	}
	return s.repository.GetSettings(ctx, strings.TrimSpace(accountID))
}

// UpdateSettings 校验并保存账号任务设置，再读取数据库中的最终值。
func (s *Service) UpdateSettings(ctx context.Context, settings AccountTaskSettings) (AccountTaskSettings, error) {
	if s == nil || s.repository == nil {
		return AccountTaskSettings{}, errors.New("账号任务存储未初始化")
	}
	settings.CookieID = strings.TrimSpace(settings.CookieID)
	settings.RateContent = strings.TrimSpace(settings.RateContent)
	settings.ReceiptReminderMessage = strings.TrimSpace(settings.ReceiptReminderMessage)
	settings.ReceiptReminderTime = strings.TrimSpace(settings.ReceiptReminderTime)
	if settings.CookieID == "" {
		return AccountTaskSettings{}, errors.New("账号标识不能为空")
	}
	if settings.AutoRateEnabled && settings.RateContent == "" {
		return AccountTaskSettings{}, errors.New("启用自动评价时评价内容不能为空")
	}
	if len([]rune(settings.RateContent)) > 500 {
		return AccountTaskSettings{}, errors.New("评价内容不能超过 500 个字符")
	}
	if !accountTaskTimePattern.MatchString(settings.PolishTime) {
		return AccountTaskSettings{}, errors.New("擦亮时间格式必须为 HH:mm")
	}
	if settings.RequestFlowerAfterHours < 0 || settings.RequestFlowerAfterHours > 720 {
		return AccountTaskSettings{}, errors.New("自动求花延迟必须在 0 到 720 小时之间")
	}
	// legacyFlowerDelay 表示 schema 37 已启用小时配置迁移后的等值秒数，允许旧配置保持原语义。
	legacyFlowerDelay := settings.RequestFlowerAfterHours * 3600
	if settings.RequestFlowerAfterSeconds < 0 || settings.RequestFlowerAfterSeconds > 3600 && settings.RequestFlowerAfterSeconds != legacyFlowerDelay {
		return AccountTaskSettings{}, errors.New("发货后自动求花延迟必须在 0 到 3600 秒之间")
	}
	if settings.ReceiveFlowerTimeoutSeconds == 0 {
		settings.ReceiveFlowerTimeoutSeconds = 120
	}
	if settings.ReceiveFlowerTimeoutSeconds < 30 || settings.ReceiveFlowerTimeoutSeconds > 600 {
		return AccountTaskSettings{}, errors.New("自动收花超时必须在 30 到 600 秒之间")
	}
	if settings.ReceiptReminderAfterDays == 0 {
		settings.ReceiptReminderAfterDays = 2
	}
	if settings.ReceiptReminderTime == "" {
		settings.ReceiptReminderTime = "10:00"
	}
	if settings.ReceiptReminderMessage == "" {
		settings.ReceiptReminderMessage = "您好，订单已经发货一段时间。确认商品信息无误后，麻烦在闲鱼确认收货，谢谢。"
	}
	if settings.ReceiptReminderAfterDays < 1 || settings.ReceiptReminderAfterDays > 30 {
		return AccountTaskSettings{}, errors.New("确认收货提醒等待天数必须在 1 到 30 天之间")
	}
	if !accountTaskTimePattern.MatchString(settings.ReceiptReminderTime) {
		return AccountTaskSettings{}, errors.New("确认收货提醒时间格式必须为 HH:mm")
	}
	// previous、previousErr 是更新前设置，用于识别关闭到开启的边沿并建立不追发历史订单的基线。
	previous, previousErr := s.repository.GetSettings(ctx, settings.CookieID)
	if previousErr != nil {
		return AccountTaskSettings{}, previousErr
	}
	if settings.AutoReceiptReminderEnabled {
		if !previous.AutoReceiptReminderEnabled || previous.ReceiptReminderEnabledAt <= 0 {
			// now 是本次开启动作的服务端 UTC 时间，客户端不能伪造历史基线。
			now := time.Now()
			if s.now != nil {
				now = s.now()
			}
			settings.ReceiptReminderEnabledAt = now.UTC().Unix()
		} else {
			settings.ReceiptReminderEnabledAt = previous.ReceiptReminderEnabledAt
		}
	} else {
		settings.ReceiptReminderEnabledAt = 0
	}
	// err 表示账号任务设置写入失败。
	if err := s.repository.SaveSettings(ctx, settings); err != nil {
		return AccountTaskSettings{}, err
	}
	return s.repository.GetSettings(ctx, settings.CookieID)
}

// ListRuns 查询指定账号最近的任务运行记录。
func (s *Service) ListRuns(ctx context.Context, accountID string, limit int) ([]AccountTaskRun, error) {
	if s == nil || s.repository == nil {
		return nil, errors.New("账号任务存储未初始化")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repository.ListRuns(ctx, strings.TrimSpace(accountID), limit)
}

// Run 手动执行指定账号的账号任务。
func (s *Service) Run(ctx context.Context, accountID, taskType string) (TaskSummary, error) {
	if s == nil || s.runner == nil {
		return TaskSummary{}, ErrUnavailable
	}
	if taskType != TaskAutoRate && taskType != TaskAutoPolish {
		return TaskSummary{}, ErrInvalidTaskType
	}
	return s.runner.RunAccountTask(ctx, strings.TrimSpace(accountID), taskType)
}
