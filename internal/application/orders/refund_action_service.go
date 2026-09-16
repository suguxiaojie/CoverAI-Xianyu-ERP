package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Act 在重新读取平台最新详情和原子抢占后执行一次普通同意／拒绝退款动作。
func (service *RefundDetailService) Act(ctx context.Context, request RefundActionRequest) (RefundActionResult, error) {
	// _, accountID、orderID、validationErr 是本地退款上下文、规范标识和校验错误。
	_, accountID, orderID, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return RefundActionResult{}, validationErr
	}
	// actionCode 是客户端刚从详情读取的动作标识，只用于匹配最新平台数据。
	actionCode := strings.TrimSpace(request.ActionCode)
	if actionCode == "" || len([]rune(actionCode)) > 100 {
		return RefundActionResult{}, ErrRefundActionInvalid
	}
	// latest、latestErr 是提交前重新读取的权威退款详情和动态动作。
	latest, latestErr := service.fetchPlatformDetail(ctx, request.UserID, accountID, orderID)
	if latestErr != nil {
		return RefundActionResult{}, latestErr
	}
	if latest == nil || !latest.Seller || !refundActionTypeSupported(latest.Type) {
		return RefundActionResult{}, ErrRefundActionInvalid
	}
	// selected 是平台最新详情中与客户端 code 精确匹配的服务端执行描述。
	var selected *RefundPlatformAction
	// action 是当前待匹配的平台动态动作。
	for actionIndex := range latest.PlatformActions {
		// action 是当前平台动作的独立引用。
		action := &latest.PlatformActions[actionIndex]
		if action.Code == actionCode && (action.Kind == "agree" || action.Kind == "reject") {
			selected = action
			break
		}
	}
	if selected == nil || strings.TrimSpace(latest.RefundID) == "" {
		return RefundActionResult{}, ErrRefundActionInvalid
	}
	if selected.Mode != "direct" {
		return RefundActionResult{}, ErrRefundActionRequiresOfficial
	}
	// runKey 对同一账号、订单和退款申请永久唯一，阻止同意与拒绝竞态提交。
	runKey := RefundActionTaskType + ":" + accountID + ":" + orderID + ":" + strings.TrimSpace(latest.RefundID)
	// claimed、claimErr 是真实退款动作的原子抢占结果。
	claimed, claimErr := service.repository.ClaimRefundAction(ctx, runKey, accountID, orderID, time.Now().UTC().Unix())
	if claimErr != nil {
		return RefundActionResult{}, claimErr
	}
	if !claimed {
		return RefundActionResult{}, ErrRefundActionAlreadyHandled
	}
	// detail、credentialErr 是动作提交前重新读取的平台凭证。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, accountID)
	if credentialErr != nil {
		_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, credentialErr.Error(), false)
		return RefundActionResult{}, credentialErr
	}
	// submitCtx、cancel 限制真实退款处理平台请求时间。
	submitCtx, cancel := context.WithTimeout(ctx, refundDetailRequestTimeout)
	defer cancel()
	// platformResult、submitErr 是平台真实退款动作结果和错误。
	platformResult, submitErr := service.runtime.SubmitRefundAction(submitCtx, detail, orderID, latest.RefundID, *selected)
	// credentialWarning 是响应 Cookie 未能协调的非动作失败告警。
	credentialWarning := service.persistCredential(ctx, request.UserID, accountID, detail, refundCookieResultFromAction(platformResult))
	if submitErr != nil {
		if service.runtime.IsSessionExpired(submitErr) {
			service.runtime.RecoverExpiredSession(ctx, accountID, submitErr)
			_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, submitErr.Error(), false)
			return RefundActionResult{}, submitErr
		}
		// finishErr 是不明确远端结果的隔离状态写入错误。
		finishErr := service.finishRefundAction(ctx, runKey, "needs_review", 0, 1, submitErr.Error(), true)
		if finishErr != nil {
			return RefundActionResult{}, errors.Join(ErrRefundActionNeedsReview, finishErr)
		}
		return RefundActionResult{Status: "needs_review", Message: ErrRefundActionNeedsReview.Error(), OrderID: orderID, RefundID: latest.RefundID, Action: selected.Kind}, ErrRefundActionNeedsReview
	}
	if platformResult != nil && platformResult.RequiresOfficial {
		// message 是前置 MTOP 明确要求原生认证时保存的失败说明。
		message := strings.TrimSpace(platformResult.Message)
		if message == "" {
			message = ErrRefundActionRequiresOfficial.Error()
		}
		_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, message, false)
		return RefundActionResult{Status: "failed", Message: message, OrderID: orderID, RefundID: latest.RefundID, Action: selected.Kind}, ErrRefundActionRequiresOfficial
	}
	if platformResult == nil || !platformResult.Success {
		// message 是平台明确拒绝时保存和展示的说明。
		message := "闲鱼未确认退款处理成功"
		if platformResult != nil && strings.TrimSpace(platformResult.Message) != "" {
			message = strings.TrimSpace(platformResult.Message)
		}
		_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, message, false)
		return RefundActionResult{Status: "failed", Message: message, OrderID: orderID, RefundID: latest.RefundID, Action: selected.Kind}, fmt.Errorf("%w: %s", ErrRefundActionInvalid, message)
	}
	// status、message 区分纯成功和凭证并发变化警告。
	status, message := "succeeded", strings.TrimSpace(platformResult.Message)
	if message == "" {
		if selected.Kind == "agree" {
			message = "已同意退款申请"
		} else {
			message = "已拒绝退款申请"
		}
	}
	if credentialWarning != nil {
		status = "succeeded_with_warning"
		message += "；账号凭证已发生变化，未覆盖较新的登录状态"
	}
	if // finishErr 是平台明确成功后的本地幂等终态写入错误。
	finishErr := service.finishRefundAction(ctx, runKey, "success", 1, 0, credentialErrorText(credentialWarning), true); finishErr != nil {
		return RefundActionResult{Success: true, Status: "needs_review", Message: "闲鱼已处理退款，但本地状态保存失败，请勿重复提交", OrderID: orderID, RefundID: latest.RefundID, Action: selected.Kind}, nil
	}
	return RefundActionResult{Success: true, Status: status, Message: message, OrderID: orderID, RefundID: latest.RefundID, Action: selected.Kind}, nil
}

// fetchPlatformDetail 在动作前读取官方最新详情并安全协调响应 Cookie。
func (service *RefundDetailService) fetchPlatformDetail(ctx context.Context, userID int64, accountID, orderID string) (*RefundDetailPlatformResult, error) {
	// detail、credentialErr 是详情请求使用的凭证快照。
	detail, credentialErr := service.loadCredential(ctx, userID, accountID)
	if credentialErr != nil {
		return nil, credentialErr
	}
	// requestCtx、cancel 限制只读详情请求时间。
	requestCtx, cancel := context.WithTimeout(ctx, refundDetailRequestTimeout)
	defer cancel()
	// result、fetchErr 是平台最新退款详情和错误。
	result, fetchErr := service.runtime.FetchRefundDetail(requestCtx, detail, orderID)
	// credentialWarning 是只读响应 Cookie 协调错误。
	credentialWarning := service.persistCredential(ctx, userID, accountID, detail, refundCookieResultFromDetail(result))
	if fetchErr != nil {
		if service.runtime.IsSessionExpired(fetchErr) {
			service.runtime.RecoverExpiredSession(ctx, accountID, fetchErr)
		}
		return nil, fetchErr
	}
	if credentialWarning != nil {
		return nil, fmt.Errorf("读取退款动作后账号凭证已变化: %w", credentialWarning)
	}
	if result == nil {
		return nil, errors.New("闲鱼未返回退款详情")
	}
	return result, nil
}

// refundActionTypeSupported 将第二阶段限制为普通退款／仅退款，不处理退货物流状态机。
func refundActionTypeSupported(refundType string) bool {
	// normalized 是去空白后的平台退款类型。
	normalized := strings.TrimSpace(refundType)
	return normalized == "退款" || strings.Contains(normalized, "仅退款")
}

// finishRefundAction 保存退款动作终态；detached 用于外部结果已确定后的独立写入预算。
func (service *RefundDetailService) finishRefundAction(ctx context.Context, runKey, status string, success, failed int, message string, detached bool) error {
	// finishCtx 是终态写入使用的上下文。
	finishCtx := ctx
	// cancel 释放独立终态写入预算；普通失败分支保持空操作。
	cancel := func() {}
	if detached {
		finishCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	}
	defer cancel()
	return service.repository.FinishRefundAction(finishCtx, runKey, status, success, failed, message)
}
