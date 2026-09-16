package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	automationapp "xianyu-go/internal/application/automation"
)

// mountAccountTasks 封装mount账号任务列表业务协调。
func (s *Server) mountAccountTasks(r chi.Router) {
	r.Get("/api/account-tasks/{cid}", s.getAccountTaskSettings)
	r.Put("/api/account-tasks/{cid}", s.updateAccountTaskSettings)
	r.Get("/api/account-tasks/{cid}/runs", s.listAccountTaskRuns)
	r.Post("/api/account-tasks/{cid}/run", s.runAccountTask)
}

// getAccountTaskSettings 封装get账号任务设置业务协调。
func (s *Server) getAccountTaskSettings(w http.ResponseWriter, r *http.Request) {
	// cid 用于本次流程后续判断的cid
	cid := chi.URLParam(r, "cid")
	if !s.ownsAccount(r, cid) {
		writeErr(w, http.StatusForbidden, "无权访问该账号")
		return
	}
	// settings、err 用于本次流程后续判断的settings、err
	settings, err := s.accountTaskApplication().GetSettings(r.Context(), cid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取账号任务配置失败")
		return
	}
	writeJSON(w, http.StatusOK, newApplicationAccountTaskSettingsResponse(settings))
}

// updateAccountTaskSettings 封装update账号任务设置业务协调。
func (s *Server) updateAccountTaskSettings(w http.ResponseWriter, r *http.Request) {
	// cid 用于本次流程后续判断的cid
	cid := chi.URLParam(r, "cid")
	if !s.ownsAccount(r, cid) {
		writeErr(w, http.StatusForbidden, "无权操作该账号")
		return
	}
	// input 是账号任务设置的具名 HTTP 请求 DTO；新增小红花字段用指针区分旧客户端省略与显式关闭。
	var input accountTaskSettingsRequest
	if decodeJSON(r, &input) != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	// current、currentErr 保存更新前设置，确保旧客户端不会把新增小红花配置重置为零值。
	current, currentErr := s.accountTaskApplication().GetSettings(r.Context(), cid)
	if currentErr != nil {
		writeErr(w, http.StatusInternalServerError, "读取账号任务配置失败")
		return
	}
	current.AutoRateEnabled = input.AutoRateEnabled
	current.RateContent = input.RateContent
	current.AutoPolishEnabled = input.AutoPolishEnabled
	current.PolishTime = input.PolishTime
	if input.AutoRequestFlowerEnabled != nil {
		current.AutoRequestFlowerEnabled = *input.AutoRequestFlowerEnabled
	}
	if input.RequestFlowerAfterHours != nil {
		current.RequestFlowerAfterHours = *input.RequestFlowerAfterHours
	}
	if input.RequestFlowerAfterSeconds != nil {
		current.RequestFlowerAfterSeconds = *input.RequestFlowerAfterSeconds
	} else if input.RequestFlowerAfterHours != nil {
		current.RequestFlowerAfterSeconds = *input.RequestFlowerAfterHours * 3600
	}
	if input.AutoReceiveFlowerEnabled != nil {
		current.AutoReceiveFlowerEnabled = *input.AutoReceiveFlowerEnabled
	}
	if input.ReceiveFlowerShowBrowser != nil {
		current.ReceiveFlowerShowBrowser = *input.ReceiveFlowerShowBrowser
	}
	if input.ReceiveFlowerTimeoutSeconds != nil {
		current.ReceiveFlowerTimeoutSeconds = *input.ReceiveFlowerTimeoutSeconds
	}
	if input.AutoReceiptReminderEnabled != nil {
		current.AutoReceiptReminderEnabled = *input.AutoReceiptReminderEnabled
	}
	if input.ReceiptReminderAfterDays != nil {
		current.ReceiptReminderAfterDays = *input.ReceiptReminderAfterDays
	}
	if input.ReceiptReminderTime != nil {
		current.ReceiptReminderTime = *input.ReceiptReminderTime
	}
	if input.ReceiptReminderMessage != nil {
		current.ReceiptReminderMessage = *input.ReceiptReminderMessage
	}
	// stored、err 保存应用服务规范化后的设置及写入错误。
	stored, err := s.accountTaskApplication().UpdateSettings(r.Context(), current)
	if err != nil {
		if strings.Contains(err.Error(), "不能为空") || strings.Contains(err.Error(), "不能超过") || strings.Contains(err.Error(), "格式必须") || strings.Contains(err.Error(), "必须在") {
			writeErr(w, http.StatusBadRequest, err.Error())
		} else {
			writeErr(w, http.StatusInternalServerError, "保存账号任务配置失败")
		}
		return
	}
	writeJSON(w, http.StatusOK, newApplicationAccountTaskSettingsResponse(stored))
}

// listAccountTaskRuns 封装list账号任务运行记录业务协调。
func (s *Server) listAccountTaskRuns(w http.ResponseWriter, r *http.Request) {
	// cid 用于本次流程后续判断的cid
	cid := chi.URLParam(r, "cid")
	if !s.ownsAccount(r, cid) {
		writeErr(w, http.StatusForbidden, "无权访问该账号")
		return
	}
	// runs、err 用于本次流程后续判断的runs、err
	runs, err := s.accountTaskApplication().ListRuns(r.Context(), cid, parsePositiveInt(r.URL.Query().Get("limit"), 20))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取任务记录失败")
		return
	}
	writeJSON(w, http.StatusOK, accountTaskRunsResponse{Runs: newApplicationAccountTaskRunResponses(runs)})
}

// runAccountTask 封装运行账号任务业务协调。
func (s *Server) runAccountTask(w http.ResponseWriter, r *http.Request) {
	// cid 用于本次流程后续判断的cid
	cid := chi.URLParam(r, "cid")
	if !s.ownsAccount(r, cid) {
		writeErr(w, http.StatusForbidden, "无权操作该账号")
		return
	}
	// input 用于本次流程后续判断的input
	var input accountTaskRunRequest
	if decodeJSON(r, &input) != nil || (input.TaskType != automationapp.TaskAutoRate && input.TaskType != automationapp.TaskAutoPolish) {
		writeErr(w, http.StatusBadRequest, "不支持的任务类型")
		return
	}
	// summary、err 用于本次流程后续判断的summary、err
	summary, err := s.accountTaskApplication().Run(r.Context(), cid, input.TaskType)
	if err != nil {
		if errors.Is(err, automationapp.ErrUnavailable) {
			writeErr(w, http.StatusServiceUnavailable, "自动化中心未启用")
			return
		}
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, accountTaskRunResponseEnvelope{Success: true, Summary: newApplicationAccountTaskSummaryResponse(summary)})
}

// accountTaskRunRequest 是手动执行账号任务的具名 HTTP 请求 DTO。
type accountTaskRunRequest struct {
	// TaskType 是要求执行的账号任务类型。
	TaskType string `json:"task_type"`
}

// accountTaskSettingsRequest 是账号任务设置更新的具名 HTTP 请求 DTO。
type accountTaskSettingsRequest struct {
	// AutoRateEnabled 表示是否启用自动评价。
	AutoRateEnabled bool `json:"auto_rate_enabled"`
	// RateContent 是自动评价文案。
	RateContent string `json:"rate_content"`
	// AutoPolishEnabled 表示是否启用每日擦亮。
	AutoPolishEnabled bool `json:"auto_polish_enabled"`
	// PolishTime 是每日擦亮时间。
	PolishTime string `json:"polish_time"`
	// AutoRequestFlowerEnabled 为非空时更新自动求花开关。
	AutoRequestFlowerEnabled *bool `json:"auto_request_flower_enabled"`
	// RequestFlowerAfterHours 为旧客户端兼容的等待小时数。
	RequestFlowerAfterHours *int `json:"request_flower_after_hours"`
	// RequestFlowerAfterSeconds 为非空时更新发货成功后等待秒数。
	RequestFlowerAfterSeconds *int `json:"request_flower_after_seconds"`
	// AutoReceiveFlowerEnabled 为非空时更新自动收花开关。
	AutoReceiveFlowerEnabled *bool `json:"auto_receive_flower_enabled"`
	// ReceiveFlowerShowBrowser 为非空时更新独立浏览器显示方式。
	ReceiveFlowerShowBrowser *bool `json:"receive_flower_show_browser"`
	// ReceiveFlowerTimeoutSeconds 为非空时更新等待平台结果的秒数。
	ReceiveFlowerTimeoutSeconds *int `json:"receive_flower_timeout_seconds"`
	// AutoReceiptReminderEnabled 为非空时更新自动确认收货提醒开关。
	AutoReceiptReminderEnabled *bool `json:"auto_receipt_reminder_enabled"`
	// ReceiptReminderAfterDays 为非空时更新发货后等待天数。
	ReceiptReminderAfterDays *int `json:"receipt_reminder_after_days"`
	// ReceiptReminderTime 为非空时更新每日北京时间执行时刻。
	ReceiptReminderTime *string `json:"receipt_reminder_time"`
	// ReceiptReminderMessage 为非空时保留旧候选客户端字段；官方系统卡片内容不可编辑。
	ReceiptReminderMessage *string `json:"receipt_reminder_message"`
}
