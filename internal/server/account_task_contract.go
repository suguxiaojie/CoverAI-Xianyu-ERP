package server

// accountTaskSettingsResponse 是账号任务设置接口的具名 DTO。
type accountTaskSettingsResponse struct {
	// AccountID 是账号稳定标识。
	AccountID string `json:"account_id"`
	// AutoRateEnabled 表示自动评价是否启用。
	AutoRateEnabled bool `json:"auto_rate_enabled"`
	// RateContent 是自动评价文案。
	RateContent string `json:"rate_content"`
	// AutoPolishEnabled 表示自动擦亮是否启用。
	AutoPolishEnabled bool `json:"auto_polish_enabled"`
	// PolishTime 是自动擦亮本地时间。
	PolishTime string `json:"polish_time"`
	// AutoRequestFlowerEnabled 表示自动求花是否启用。
	AutoRequestFlowerEnabled bool `json:"auto_request_flower_enabled"`
	// RequestFlowerAfterHours 是旧客户端兼容的小时数。
	RequestFlowerAfterHours int `json:"request_flower_after_hours"`
	// RequestFlowerAfterSeconds 是明确发货成功后等待的秒数。
	RequestFlowerAfterSeconds int `json:"request_flower_after_seconds"`
	// AutoReceiveFlowerEnabled 表示自动收花是否启用。
	AutoReceiveFlowerEnabled bool `json:"auto_receive_flower_enabled"`
	// ReceiveFlowerShowBrowser 表示自动收花是否显示独立浏览器。
	ReceiveFlowerShowBrowser bool `json:"receive_flower_show_browser"`
	// ReceiveFlowerTimeoutSeconds 是等待平台结果的最长秒数。
	ReceiveFlowerTimeoutSeconds int `json:"receive_flower_timeout_seconds"`
	// AutoReceiptReminderEnabled 表示自动确认收货提醒是否启用。
	AutoReceiptReminderEnabled bool `json:"auto_receipt_reminder_enabled"`
	// ReceiptReminderAfterDays 是发货后等待的完整天数。
	ReceiptReminderAfterDays int `json:"receipt_reminder_after_days"`
	// ReceiptReminderTime 是每日北京时间执行时刻。
	ReceiptReminderTime string `json:"receipt_reminder_time"`
	// ReceiptReminderMessage 是旧候选客户端保留的兼容字段；系统卡片调度不读取它。
	ReceiptReminderMessage string `json:"receipt_reminder_message"`
	// ReceiptReminderEnabledAt 是最近一次开启基线 Unix 秒。
	ReceiptReminderEnabledAt int64 `json:"receipt_reminder_enabled_at"`
	// LastRateScanAt 是最近一次评价扫描时间。
	LastRateScanAt int64 `json:"last_rate_scan_at"`
	// LastPolishDate 是最近一次擦亮日期。
	LastPolishDate string `json:"last_polish_date"`
	// LastPolishAt 是最近一次擦亮时间。
	LastPolishAt int64 `json:"last_polish_at"`
}
