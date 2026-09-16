package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// DefaultReceiptReminderMessage 是新账号默认使用的确认收货提醒文案；发送前仍由账号设置覆盖。
const DefaultReceiptReminderMessage = "您好，订单已经发货一段时间。确认商品信息无误后，麻烦在闲鱼确认收货，谢谢。"

// AccountTaskSettings 用于本次流程后续判断的账号任务设置
type AccountTaskSettings struct {
	CookieID                    string `json:"account_id"`
	AutoRateEnabled             bool   `json:"auto_rate_enabled"`
	RateContent                 string `json:"rate_content"`
	AutoPolishEnabled           bool   `json:"auto_polish_enabled"`
	PolishTime                  string `json:"polish_time"`
	AutoRequestFlowerEnabled    bool   `json:"auto_request_flower_enabled"`
	RequestFlowerAfterHours     int    `json:"request_flower_after_hours"`
	RequestFlowerAfterSeconds   int    `json:"request_flower_after_seconds"`
	AutoReceiveFlowerEnabled    bool   `json:"auto_receive_flower_enabled"`
	ReceiveFlowerShowBrowser    bool   `json:"receive_flower_show_browser"`
	ReceiveFlowerTimeoutSeconds int    `json:"receive_flower_timeout_seconds"`
	// AutoReceiptReminderEnabled 表示是否启用发货后自动确认收货提醒。
	AutoReceiptReminderEnabled bool `json:"auto_receipt_reminder_enabled"`
	// ReceiptReminderAfterDays 是发货后至少等待的完整天数。
	ReceiptReminderAfterDays int `json:"receipt_reminder_after_days"`
	// ReceiptReminderTime 是达到完整天数后下一次北京时间执行时刻。
	ReceiptReminderTime string `json:"receipt_reminder_time"`
	// ReceiptReminderMessage 是 Schema 53 普通文字候选遗留的兼容字段；官方系统卡片调度不读取它。
	ReceiptReminderMessage string `json:"receipt_reminder_message"`
	// ReceiptReminderEnabledAt 是最近一次从关闭切换为开启的 Unix 秒基线。
	ReceiptReminderEnabledAt int64  `json:"receipt_reminder_enabled_at"`
	LastRateScanAt           int64  `json:"last_rate_scan_at"`
	LastPolishDate           string `json:"last_polish_date"`
	LastPolishAt             int64  `json:"last_polish_at"`
}

// ReceiptReminderCandidate 是调度器发送确认收货提醒前需要的最小订单事实。
type ReceiptReminderCandidate struct {
	// OrderID 是订单稳定标识。
	OrderID string
	// CookieID 是订单所属卖家账号。
	CookieID string
	// ChatID 是持久化或唯一历史事实解析出的精确会话标识。
	ChatID string
	// BuyerID 是提醒消息接收方。
	BuyerID string
	// ItemID 是订单关联商品，用于运行通知上下文。
	ItemID string
	// ShippedAt 是可靠发货时间文本。
	ShippedAt string
	// OrderStatus 是发送前必须再次规范化验证的订单状态。
	OrderStatus string
}

// FlowerRequestCandidate 是自动求花扫描从订单表读取的最小非敏感事实集合。
type FlowerRequestCandidate struct {
	// OrderID 是待判断的卖家订单标识。
	OrderID string
	// CookieID 是订单所属账号标识。
	CookieID string
	// ChatID 是订单关联的会话标识。
	ChatID string
	// BuyerID 是订单买家标识。
	BuyerID string
	// ItemID 是订单关联商品标识。
	ItemID string
	// ShippedAt 是数据库保存的明确发货时间文本。
	ShippedAt string
	// PaidAt 是官方三十天求花期限使用的付款时间文本。
	PaidAt string
	// OrderStatus 是尚未归一化的平台订单状态。
	OrderStatus string
}

// AccountTaskRun 用于本次流程后续判断的账号任务运行
type AccountTaskRun struct {
	ID           int64  `json:"id"`
	RunKey       string `json:"run_key"`
	CookieID     string `json:"account_id"`
	TaskType     string `json:"task_type"`
	TargetID     string `json:"target_id"`
	RunDate      string `json:"run_date"`
	Status       string `json:"status"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
	ErrorMessage string `json:"error_message"`
	NextRetryAt  int64  `json:"next_retry_at"`
	StartedAt    int64  `json:"started_at"`
	FinishedAt   int64  `json:"finished_at"`
}

// AccountTaskStore 用于本次流程后续判断的账号任务Store
type AccountTaskStore struct {
	DB      *sql.DB
	Dialect Dialect
}

// defaultAccountTaskSettings 封装default账号任务设置业务协调。
func defaultAccountTaskSettings(cookieID string) AccountTaskSettings {
	return AccountTaskSettings{
		CookieID: cookieID, RateContent: "不错的买家，交易愉快", PolishTime: "03:00",
		RequestFlowerAfterHours: 24, RequestFlowerAfterSeconds: 10,
		ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
		ReceiptReminderAfterDays: 2, ReceiptReminderTime: "10:00", ReceiptReminderMessage: DefaultReceiptReminderMessage,
	}
}

// Get 读取当前值。
func (s *AccountTaskStore) Get(ctx context.Context, cookieID string) (AccountTaskSettings, error) {
	// result 用于本次流程后续判断的结果
	result := defaultAccountTaskSettings(cookieID)
	// rateEnabled、polishEnabled、requestFlowerEnabled、receiveFlowerEnabled、showReceiveBrowser、receiptReminderEnabled 保存数据库布尔字段。
	var rateEnabled, polishEnabled, requestFlowerEnabled, receiveFlowerEnabled, showReceiveBrowser, receiptReminderEnabled int
	// err 用于本次流程后续判断的err
	err := s.DB.QueryRowContext(ctx, `SELECT auto_rate_enabled,rate_content,auto_polish_enabled,polish_time,
		auto_request_flower_enabled,request_flower_after_hours,request_flower_after_seconds,auto_receive_flower_enabled,
		receive_flower_show_browser,receive_flower_timeout_seconds,auto_receipt_reminder_enabled,receipt_reminder_after_days,
		receipt_reminder_time,receipt_reminder_message,receipt_reminder_enabled_at,last_rate_scan_at,last_polish_date,last_polish_at
		FROM account_task_settings WHERE cookie_id=?`, cookieID).Scan(
		&rateEnabled, &result.RateContent, &polishEnabled, &result.PolishTime,
		&requestFlowerEnabled, &result.RequestFlowerAfterHours, &result.RequestFlowerAfterSeconds, &receiveFlowerEnabled,
		&showReceiveBrowser, &result.ReceiveFlowerTimeoutSeconds, &receiptReminderEnabled, &result.ReceiptReminderAfterDays,
		&result.ReceiptReminderTime, &result.ReceiptReminderMessage, &result.ReceiptReminderEnabledAt, &result.LastRateScanAt,
		&result.LastPolishDate, &result.LastPolishAt)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	result.AutoRateEnabled = rateEnabled != 0
	result.AutoPolishEnabled = polishEnabled != 0
	result.AutoRequestFlowerEnabled = requestFlowerEnabled != 0
	result.AutoReceiveFlowerEnabled = receiveFlowerEnabled != 0
	result.ReceiveFlowerShowBrowser = showReceiveBrowser != 0
	result.AutoReceiptReminderEnabled = receiptReminderEnabled != 0
	return result, err
}

// Upsert 封装Upsert业务协调。
func (s *AccountTaskStore) Upsert(ctx context.Context, settings AccountTaskSettings) error {
	settings.RateContent = strings.TrimSpace(settings.RateContent)
	if settings.RateContent == "" {
		settings.RateContent = "不错的买家，交易愉快"
	}
	if settings.PolishTime == "" {
		settings.PolishTime = "03:00"
	}
	settings.ReceiptReminderMessage = strings.TrimSpace(settings.ReceiptReminderMessage)
	if settings.ReceiptReminderMessage == "" {
		settings.ReceiptReminderMessage = DefaultReceiptReminderMessage
	}
	if settings.ReceiptReminderTime == "" {
		settings.ReceiptReminderTime = "10:00"
	}
	if settings.ReceiptReminderAfterDays == 0 {
		settings.ReceiptReminderAfterDays = 2
	}
	// now 用于本次流程后续判断的now
	now := time.Now().UTC().Unix()
	// query 用于本次流程后续判断的查询
	query := `INSERT INTO account_task_settings
		(cookie_id,auto_rate_enabled,rate_content,auto_polish_enabled,polish_time,
		auto_request_flower_enabled,request_flower_after_hours,request_flower_after_seconds,auto_receive_flower_enabled,
		receive_flower_show_browser,receive_flower_timeout_seconds,auto_receipt_reminder_enabled,receipt_reminder_after_days,
		receipt_reminder_time,receipt_reminder_message,receipt_reminder_enabled_at,last_rate_scan_at,last_polish_date,last_polish_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)` + dialectUpsert(s.Dialect, []string{"cookie_id"}, map[string]string{
		"auto_rate_enabled": "EXCLUDED.auto_rate_enabled", "rate_content": "EXCLUDED.rate_content",
		"auto_polish_enabled": "EXCLUDED.auto_polish_enabled", "polish_time": "EXCLUDED.polish_time",
		"auto_request_flower_enabled": "EXCLUDED.auto_request_flower_enabled", "request_flower_after_hours": "EXCLUDED.request_flower_after_hours",
		"request_flower_after_seconds": "EXCLUDED.request_flower_after_seconds",
		"auto_receive_flower_enabled":  "EXCLUDED.auto_receive_flower_enabled", "receive_flower_show_browser": "EXCLUDED.receive_flower_show_browser",
		"receive_flower_timeout_seconds": "EXCLUDED.receive_flower_timeout_seconds",
		"auto_receipt_reminder_enabled":  "EXCLUDED.auto_receipt_reminder_enabled", "receipt_reminder_after_days": "EXCLUDED.receipt_reminder_after_days",
		"receipt_reminder_time": "EXCLUDED.receipt_reminder_time", "receipt_reminder_message": "EXCLUDED.receipt_reminder_message",
		"receipt_reminder_enabled_at": "EXCLUDED.receipt_reminder_enabled_at",
		"updated_at":                  "EXCLUDED.updated_at",
	})
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, query, settings.CookieID, boolInt(settings.AutoRateEnabled), settings.RateContent,
		boolInt(settings.AutoPolishEnabled), settings.PolishTime, boolInt(settings.AutoRequestFlowerEnabled),
		settings.RequestFlowerAfterHours, settings.RequestFlowerAfterSeconds, boolInt(settings.AutoReceiveFlowerEnabled), boolInt(settings.ReceiveFlowerShowBrowser),
		settings.ReceiveFlowerTimeoutSeconds, boolInt(settings.AutoReceiptReminderEnabled), settings.ReceiptReminderAfterDays,
		settings.ReceiptReminderTime, settings.ReceiptReminderMessage, settings.ReceiptReminderEnabledAt,
		settings.LastRateScanAt, settings.LastPolishDate, settings.LastPolishAt, now, now)
	return err
}

// Enabled 封装启用状态业务协调。
func (s *AccountTaskStore) Enabled(ctx context.Context) ([]AccountTaskSettings, error) {
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := s.DB.QueryContext(ctx, `SELECT s.cookie_id,s.auto_rate_enabled,s.rate_content,s.auto_polish_enabled,s.polish_time,
		s.auto_request_flower_enabled,s.request_flower_after_hours,s.request_flower_after_seconds,s.auto_receive_flower_enabled,
		s.receive_flower_show_browser,s.receive_flower_timeout_seconds,s.auto_receipt_reminder_enabled,s.receipt_reminder_after_days,
		s.receipt_reminder_time,s.receipt_reminder_message,s.receipt_reminder_enabled_at,s.last_rate_scan_at,s.last_polish_date,s.last_polish_at
		FROM account_task_settings s JOIN cookies c ON c.id=s.cookie_id
		WHERE s.auto_rate_enabled=1 OR s.auto_polish_enabled=1 OR s.auto_request_flower_enabled=1 OR s.auto_receive_flower_enabled=1 OR s.auto_receipt_reminder_enabled=1
		ORDER BY s.cookie_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// result 用于本次流程后续判断的结果
	var result []AccountTaskSettings
	for rows.Next() {
		// row 用于本次流程后续判断的row
		var row AccountTaskSettings
		// rate、polish、requestFlower、receiveFlower、showReceiveBrowser、receiptReminder 保存当前设置的数据库布尔字段。
		var rate, polish, requestFlower, receiveFlower, showReceiveBrowser, receiptReminder int
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&row.CookieID, &rate, &row.RateContent, &polish, &row.PolishTime,
			&requestFlower, &row.RequestFlowerAfterHours, &row.RequestFlowerAfterSeconds, &receiveFlower, &showReceiveBrowser,
			&row.ReceiveFlowerTimeoutSeconds, &receiptReminder, &row.ReceiptReminderAfterDays, &row.ReceiptReminderTime,
			&row.ReceiptReminderMessage, &row.ReceiptReminderEnabledAt, &row.LastRateScanAt, &row.LastPolishDate, &row.LastPolishAt); err != nil {
			return nil, err
		}
		row.AutoRateEnabled, row.AutoPolishEnabled = rate != 0, polish != 0
		row.AutoRequestFlowerEnabled, row.AutoReceiveFlowerEnabled = requestFlower != 0, receiveFlower != 0
		row.ReceiveFlowerShowBrowser = showReceiveBrowser != 0
		row.AutoReceiptReminderEnabled = receiptReminder != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

// ListReceiptReminderCandidates 按稳定订单游标读取仍处于已发货状态且有可靠发货时间的提醒候选。
func (s *AccountTaskStore) ListReceiptReminderCandidates(ctx context.Context, cookieID, afterOrderID string, limit int) ([]ReceiptReminderCandidate, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	// rows、queryErr 是当前页候选订单结果和查询错误。
	rows, queryErr := s.DB.QueryContext(ctx, `SELECT o.order_id,o.cookie_id,`+resolvedOrderChatIDExpression+`,COALESCE(o.buyer_id,''),COALESCE(o.item_id,''),
		COALESCE(o.shipped_at,''),COALESCE(o.order_status,'') FROM orders o
		WHERE o.cookie_id=? AND o.deleted_at IS NULL AND o.order_status IN ('shipped','3')
		  AND COALESCE(o.shipped_at,'')<>'' AND o.order_id>?
		ORDER BY o.order_id LIMIT ?`, cookieID, afterOrderID, limit)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// candidates 保存当前页可继续做应用层资格判断的订单事实。
	candidates := make([]ReceiptReminderCandidate, 0, limit)
	for rows.Next() {
		// candidate 是当前扫描出的确认收货提醒候选。
		var candidate ReceiptReminderCandidate
		// scanErr 是当前候选字段扫描错误。
		scanErr := rows.Scan(&candidate.OrderID, &candidate.CookieID, &candidate.ChatID, &candidate.BuyerID, &candidate.ItemID, &candidate.ShippedAt, &candidate.OrderStatus)
		if scanErr != nil {
			return nil, scanErr
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// GetReceiptReminderCandidate 在发送前按账号和订单重新读取候选；订单离开已发货状态时返回 ErrNotFound。
func (s *AccountTaskStore) GetReceiptReminderCandidate(ctx context.Context, cookieID, orderID string) (ReceiptReminderCandidate, error) {
	// candidate 保存发送前重新读取的最新订单事实。
	var candidate ReceiptReminderCandidate
	// queryErr 是订单不存在、状态变化或字段扫描失败的原因。
	queryErr := s.DB.QueryRowContext(ctx, `SELECT o.order_id,o.cookie_id,`+resolvedOrderChatIDExpression+`,COALESCE(o.buyer_id,''),COALESCE(o.item_id,''),
		COALESCE(o.shipped_at,''),COALESCE(o.order_status,'') FROM orders o
		WHERE o.cookie_id=? AND o.order_id=? AND o.deleted_at IS NULL AND o.order_status IN ('shipped','3')
		  AND COALESCE(o.shipped_at,'')<>''`, cookieID, orderID).Scan(
		&candidate.OrderID, &candidate.CookieID, &candidate.ChatID, &candidate.BuyerID,
		&candidate.ItemID, &candidate.ShippedAt, &candidate.OrderStatus,
	)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return ReceiptReminderCandidate{}, ErrNotFound
	}
	return candidate, queryErr
}

// ListFlowerRequestCandidates 按稳定订单游标读取已明确发货、未删除的自动求花候选。
func (s *AccountTaskStore) ListFlowerRequestCandidates(ctx context.Context, cookieID, afterOrderID string, limit int) ([]FlowerRequestCandidate, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	// rows、queryErr 是候选订单查询结果和数据库错误。
	rows, queryErr := s.DB.QueryContext(ctx, `SELECT order_id,cookie_id,COALESCE(chat_id,''),COALESCE(buyer_id,''),COALESCE(item_id,''),
		COALESCE(shipped_at,''),COALESCE(paid_at,''),COALESCE(order_status,'') FROM orders
		WHERE cookie_id=? AND deleted_at IS NULL AND COALESCE(shipped_at,'')<>'' AND order_id>?
		ORDER BY order_id LIMIT ?`, cookieID, afterOrderID, limit)
	if queryErr != nil {
		return nil, queryErr
	}
	defer rows.Close()
	// candidates 保存本页最小订单事实。
	candidates := make([]FlowerRequestCandidate, 0, limit)
	for rows.Next() {
		// candidate 是当前待扫描的订单候选。
		var candidate FlowerRequestCandidate
		// scanErr 是当前候选字段扫描错误。
		scanErr := rows.Scan(&candidate.OrderID, &candidate.CookieID, &candidate.ChatID, &candidate.BuyerID, &candidate.ItemID, &candidate.ShippedAt, &candidate.PaidAt, &candidate.OrderStatus)
		if scanErr != nil {
			return nil, scanErr
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// QuarantineRunningRuns 把进程重启时遗留的指定任务运行转为人工核对，禁止自动重放未知外部结果。
func (s *AccountTaskStore) QuarantineRunningRuns(ctx context.Context, taskTypes []string, message string) (int64, error) {
	if len(taskTypes) == 0 {
		return 0, nil
	}
	// placeholders 保存跨方言兼容的任务类型占位符。
	placeholders := make([]string, len(taskTypes))
	// arguments 依次保存人工核对说明、完成时间和任务类型。
	arguments := make([]any, 0, len(taskTypes)+2)
	arguments = append(arguments, message, time.Now().UTC().Unix())
	// index、taskType 是当前任务类型的占位符位置和值。
	for index, taskType := range taskTypes {
		placeholders[index] = "?"
		arguments = append(arguments, taskType)
	}
	// result 是批量隔离运行记录的数据库执行结果。
	result, execErr := s.DB.ExecContext(ctx, `UPDATE account_task_runs SET status='needs_review',error_message=?,
		next_retry_at=0,finished_at=? WHERE status='running' AND task_type IN (`+strings.Join(placeholders, ",")+`)`, arguments...)
	if execErr != nil {
		return 0, execErr
	}
	// affected 是本次被隔离的运行数量。
	affected, rowsErr := result.RowsAffected()
	return affected, rowsErr
}

// ClaimRun creates a run or atomically reclaims a due failed run.
// ClaimRun 封装Claim运行业务协调。
func (s *AccountTaskStore) ClaimRun(ctx context.Context, run AccountTaskRun, now int64) (bool, error) {
	return s.claimRun(ctx, run, now, false)
}

// ClaimRunImmediately creates a run or immediately reclaims a failed run. It is
// intended for an explicit user retry; scheduled workers should keep using
// ClaimRun so repeated platform failures still honor their retry delay.
// ClaimRunImmediately 封装Claim运行Immediately业务协调。
func (s *AccountTaskStore) ClaimRunImmediately(ctx context.Context, run AccountTaskRun, now int64) (bool, error) {
	return s.claimRun(ctx, run, now, true)
}

// claimRun 封装claim运行业务协调。
func (s *AccountTaskStore) claimRun(ctx context.Context, run AccountTaskRun, now int64, immediate bool) (bool, error) {
	// retryCondition 用于本次流程后续判断的重试Condition
	retryCondition := "next_retry_at<=?"
	// args 用于本次流程后续判断的args
	args := []any{now, run.RunKey, now}
	if immediate {
		retryCondition = "1=1"
		args = args[:2]
	}
	// res、err 用于本次流程后续判断的res、err
	res, err := s.DB.ExecContext(ctx, `UPDATE account_task_runs SET status='running',started_at=?,finished_at=0,error_message=''
		WHERE run_key=? AND status='failed' AND `+retryCondition, args...)
	if err != nil {
		return false, err
	}
	if // n 用于本次流程后续判断的n
	n, _ := res.RowsAffected(); n > 0 {
		return true, nil
	}
	// query 用于本次流程后续判断的查询
	query := dialectInsertIgnorePrefix(s.Dialect) + ` INTO account_task_runs
		(run_key,cookie_id,task_type,target_id,run_date,status,success_count,failed_count,error_message,next_retry_at,started_at,finished_at)
		VALUES(?,?,?,?,?,'running',0,0,'',0,?,0)` + dialectInsertIgnore(s.Dialect, []string{"run_key"})
	res, err = s.DB.ExecContext(ctx, query, run.RunKey, run.CookieID, run.TaskType, run.TargetID, run.RunDate, now)
	if err != nil {
		return false, err
	}
	// n 用于本次流程后续判断的n
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FinishRun 封装Finish运行业务协调。
func (s *AccountTaskStore) FinishRun(ctx context.Context, runKey, status string, success, failed int, message string, nextRetryAt int64) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE account_task_runs SET status=?,success_count=?,failed_count=?,error_message=?,next_retry_at=?,finished_at=? WHERE run_key=?`,
		status, success, failed, message, nextRetryAt, time.Now().UTC().Unix(), runKey)
	return err
}

// GetRunByKey 按稳定幂等键读取单个账号任务运行；exists=false 表示尚未执行。
func (s *AccountTaskStore) GetRunByKey(ctx context.Context, runKey string) (AccountTaskRun, bool, error) {
	// row 保存匹配幂等键的账号任务运行。
	var row AccountTaskRun
	// queryErr 是运行记录查询或字段扫描错误。
	queryErr := s.DB.QueryRowContext(ctx, `SELECT id,run_key,cookie_id,task_type,target_id,run_date,status,success_count,failed_count,
		error_message,next_retry_at,started_at,finished_at FROM account_task_runs WHERE run_key=?`, runKey).Scan(
		&row.ID, &row.RunKey, &row.CookieID, &row.TaskType, &row.TargetID, &row.RunDate,
		&row.Status, &row.SuccessCount, &row.FailedCount, &row.ErrorMessage, &row.NextRetryAt,
		&row.StartedAt, &row.FinishedAt,
	)
	if errors.Is(queryErr, sql.ErrNoRows) {
		return AccountTaskRun{}, false, nil
	}
	if queryErr != nil {
		return AccountTaskRun{}, false, queryErr
	}
	return row, true, nil
}

// MarkRateScan 封装MarkRateScan业务协调。
func (s *AccountTaskStore) MarkRateScan(ctx context.Context, cookieID string, at int64) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE account_task_settings SET last_rate_scan_at=?,updated_at=? WHERE cookie_id=?`, at, at, cookieID)
	return err
}

// MarkPolished 封装MarkPolished业务协调。
func (s *AccountTaskStore) MarkPolished(ctx context.Context, cookieID, date string, at int64) error {
	// err 用于本次流程后续判断的err
	_, err := s.DB.ExecContext(ctx, `UPDATE account_task_settings SET last_polish_date=?,last_polish_at=?,updated_at=? WHERE cookie_id=?`, date, at, at, cookieID)
	return err
}

// RecentRuns 封装Recent运行记录业务协调。
func (s *AccountTaskStore) RecentRuns(ctx context.Context, cookieID string, limit int) ([]AccountTaskRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// rows、err 用于本次流程后续判断的rows、err
	rows, err := s.DB.QueryContext(ctx, `SELECT id,run_key,cookie_id,task_type,target_id,run_date,status,success_count,failed_count,
		error_message,next_retry_at,started_at,finished_at FROM account_task_runs WHERE cookie_id=? ORDER BY id DESC LIMIT ?`, cookieID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// result 用于本次流程后续判断的结果
	var result []AccountTaskRun
	for rows.Next() {
		// row 用于本次流程后续判断的row
		var row AccountTaskRun
		if // err 用于本次流程后续判断的err
		err := rows.Scan(&row.ID, &row.RunKey, &row.CookieID, &row.TaskType, &row.TargetID, &row.RunDate,
			&row.Status, &row.SuccessCount, &row.FailedCount, &row.ErrorMessage, &row.NextRetryAt,
			&row.StartedAt, &row.FinishedAt); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// boolInt 封装boolInt业务协调。
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
