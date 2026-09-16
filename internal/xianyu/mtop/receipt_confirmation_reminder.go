package mtop

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

const (
	// ReceiptConfirmationReminderAPI 是闲鱼卖家工作台“提醒买家确认收货”的官方 MTOP 端点。
	ReceiptConfirmationReminderAPI = "https://h5api.m.goofish.com/h5/mtop.taobao.idle.trade.merchant.batch.remind.confirm/1.0/"
	// receiptConfirmationReminderAPIName 是签名查询参数使用的官方 API 名称。
	receiptConfirmationReminderAPIName = "mtop.taobao.idle.trade.merchant.batch.remind.confirm"
)

// ReceiptConfirmationReminderResult 保存平台明确返回的系统卡片发送结果和响应 Cookie。
type ReceiptConfirmationReminderResult struct {
	// Success 表示平台已接受本次提醒并生成官方系统卡片。
	Success bool
	// Message 是平台返回的非敏感成功或失败说明。
	Message string
	// UpdatedCookies 是响应合并后的 Cookie，只在调用栈内交给凭证持久化边界。
	UpdatedCookies string
}

// RemindBuyerConfirmReceipt 请求闲鱼向指定订单买家发送官方“提醒确认收货”系统卡片。
func (c *ClientImpl) RemindBuyerConfirmReceipt(ctx context.Context, cookiesStr, orderID string) (*ReceiptConfirmationReminderResult, error) {
	// normalizedOrderID 是去除首尾空白后的卖家订单标识。
	normalizedOrderID := strings.TrimSpace(orderID)
	if normalizedOrderID == "" {
		return nil, errors.New("提醒确认收货订单 ID 不能为空")
	}
	// decoded、updatedCookies、requestErr 是官方 MTOP 响应、合并 Cookie 和传输错误。
	decoded, updatedCookies, requestErr := c.accountTaskRequest(
		ctx, cookiesStr, firstNonEmptyURL(c.ReceiptReminderURL, ReceiptConfirmationReminderAPI),
		receiptConfirmationReminderAPIName, "1.0",
		map[string]any{"orderIdList": []string{normalizedOrderID}, "remindAllOrder": false},
		"https://seller.goofish.com/",
	)
	if requestErr != nil {
		return nil, requestErr
	}
	if decoded == nil {
		return nil, errors.New("提醒确认收货接口未返回结果")
	}
	// module 是官方批量结果模块，包含 totalNum、successNum 和 failOrderInfos。
	module, _ := decoded.Data["module"].(map[string]any)
	// totalNum、_ 是平台报告的处理订单总数。
	totalNum, _ := strconv.Atoi(findStringField(module, "totalNum"))
	// successNum、_ 是平台报告的成功订单数。
	successNum, _ := strconv.Atoi(findStringField(module, "successNum"))
	// message 默认使用官方页面的成功提示或响应 ret 摘要。
	message := "已提醒买家确认收货，同一订单每天最多提醒一次"
	if successNum != totalNum || totalNum != 1 {
		message = receiptReminderFailureMessage(module, normalizedOrderID)
		if message == "" {
			message = firstRet(decoded.Ret)
		}
		if message == "" {
			message = "闲鱼未接受本次确认收货提醒"
		}
	}
	return &ReceiptConfirmationReminderResult{Success: totalNum == 1 && successNum == 1, Message: message, UpdatedCookies: updatedCookies}, nil
}

// receiptReminderFailureMessage 从官方 failOrderInfos 中读取当前订单的明确失败原因。
func receiptReminderFailureMessage(module map[string]any, orderID string) string {
	// rawFailures 是平台返回的逐订单失败列表。
	rawFailures, _ := module["failOrderInfos"].([]any)
	// rawFailure 是当前待匹配的失败订单对象。
	for _, rawFailure := range rawFailures {
		if strings.TrimSpace(findStringField(rawFailure, "orderId")) != strings.TrimSpace(orderID) {
			continue
		}
		return strings.TrimSpace(findStringField(rawFailure, "errorMsg", "message", "msg"))
	}
	return ""
}
