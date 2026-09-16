package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// merchantRefundVerificationTTL 限制支付验证授权只在当前短期交互内有效。
	merchantRefundVerificationTTL = 10 * time.Minute
	// merchantRefundRunPrefix 与旧误记 refund_action 运行键隔离。
	merchantRefundRunPrefix = "merchant_refund_action"
	// merchantRefundRefuseTimeout 覆盖最多三张凭证上传和最终拒绝请求。
	merchantRefundRefuseTimeout = 90 * time.Second
)

// MerchantRefundVerificationPlatformResult 是 runtime 返回的验证页面和受控 Cookie 变化。
type MerchantRefundVerificationPlatformResult struct {
	// VerifyURL 是支付宝 PC 密码验证页面。
	VerifyURL string
	// AuthToken 是只允许应用服务内存保存的短期授权。
	AuthToken string
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// MerchantRefundVerificationStartRequest 描述创建支付验证页面的请求。
type MerchantRefundVerificationStartRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是退款订单所属卖家账号。
	AccountID string
	// OrderID 是平台订单标识。
	OrderID string
}

// MerchantRefundVerificationStartResult 是前端可安全使用的验证会话。
type MerchantRefundVerificationStartResult struct {
	// SessionID 是不包含 authToken 的随机会话标识。
	SessionID string
	// VerifyURL 是支付宝跨域 iframe 地址。
	VerifyURL string
	// VerifyOrigin 是前端 postMessage 必须精确匹配的来源。
	VerifyOrigin string
	// ExpiresAt 是会话过期 Unix 秒。
	ExpiresAt int64
}

// MerchantRefundVerificationCompleteRequest 描述支付 iframe 明确验证成功后的最终退款请求。
type MerchantRefundVerificationCompleteRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是退款订单所属卖家账号。
	AccountID string
	// OrderID 是平台订单标识。
	OrderID string
	// SessionID 是 Start 返回的随机会话标识。
	SessionID string
}

// merchantRefundVerificationSession 是只存在进程内存的敏感短期会话。
type merchantRefundVerificationSession struct {
	// UserID 是会话所属本地用户。
	UserID int64
	// AccountID 是卖家账号标识。
	AccountID string
	// OrderID 是订单标识。
	OrderID string
	// RefundID 是验证针对的退款申请标识。
	RefundID string
	// AuthToken 是支付宝验证后提交 Merchant agree 的短期授权。
	AuthToken string
	// ExpiresAt 是会话绝对过期时间。
	ExpiresAt time.Time
	// Consumed 防止同一验证结果重复提交。
	Consumed bool
}

// MerchantRefundRefuseReason 是应用层公开的动态拒绝原因。
type MerchantRefundRefuseReason struct {
	// ID 是平台 refuseReasonId。
	ID string
	// Name 是平台展示原因。
	Name string
	// RequiresApp 表示该原因只能在手机 App 操作。
	RequiresApp bool
}

// MerchantRefundRefuseFormResult 是拒绝弹窗使用的动态表单。
type MerchantRefundRefuseFormResult struct {
	// RefundID 是当前表单对应的退款标识。
	RefundID string
	// Reasons 是平台动态原因。
	Reasons []MerchantRefundRefuseReason
	// SelectedReasonID 是本次 render 使用的原因。
	SelectedReasonID string
	// ProofRequired 表示当前原因必须上传凭证。
	ProofRequired bool
	// ProofPlaceholder 是补充说明提示。
	ProofPlaceholder string
	// NegotiationEnabled 表示当前原因允许协商金额。
	NegotiationEnabled bool
	// NegotiationType 是平台动态协商类型。
	NegotiationType string
	// MinCents、MaxCents 是允许的整数分范围。
	MinCents int64
	MaxCents int64
}

// MerchantRefundRefuseFormPlatformResult 保存平台表单和 Cookie 变化。
type MerchantRefundRefuseFormPlatformResult struct {
	MerchantRefundRefuseFormResult
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// MerchantRefundRefuseFormRequest 描述动态拒绝表单读取。
type MerchantRefundRefuseFormRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是卖家账号。
	AccountID string
	// OrderID 是订单标识。
	OrderID string
	// ReasonID 是可选的已选原因。
	ReasonID string
}

// MerchantRefundRefuseRequest 描述用户二次确认后的最终拒绝请求。
type MerchantRefundRefuseRequest struct {
	// UserID 是当前认证用户。
	UserID int64
	// AccountID 是卖家账号。
	AccountID string
	// OrderID 是订单标识。
	OrderID string
	// ReasonID 是平台动态原因。
	ReasonID string
	// Description 是补充描述。
	Description string
	// NegotiationCents 是可选协商金额整数分。
	NegotiationCents int64
	// Images 是最终确认时才上传的最多三张 PNG／JPEG 图片凭证。
	Images []ShipmentEvidenceImage
}

// MerchantRefundRefusePlatformRequest 是 runtime 使用的已校验最终请求。
type MerchantRefundRefusePlatformRequest struct {
	// RefundID、OrderID、ReasonID 是平台业务标识。
	RefundID string
	OrderID  string
	ReasonID string
	// Description 是已校验补充描述。
	Description string
	// NegotiationCents 和 NegotiationType 是动态协商值。
	NegotiationCents int64
	NegotiationType  string
	// Images 是已通过应用层校验但尚未上传的平台凭证。
	Images []ShipmentEvidenceImage
}

// StartMerchantRefundVerification 创建短期支付验证会话，但不执行退款。
func (service *RefundDetailService) StartMerchantRefundVerification(ctx context.Context, request MerchantRefundVerificationStartRequest) (MerchantRefundVerificationStartResult, error) {
	// accountID、orderID、validationErr 是通过所有权校验后的规范标识和错误。
	_, accountID, orderID, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return MerchantRefundVerificationStartResult{}, validationErr
	}
	// latest、latestErr 是当前平台退款详情和读取错误。
	latest, latestErr := service.fetchPlatformDetail(ctx, request.UserID, accountID, orderID)
	if latestErr != nil {
		return MerchantRefundVerificationStartResult{}, latestErr
	}
	if latest == nil || !latest.Seller || strings.TrimSpace(latest.RefundID) == "" || !merchantRefundActionAvailable(latest.Actions, "agree") {
		return MerchantRefundVerificationStartResult{}, ErrRefundActionInvalid
	}
	// detail、credentialErr 是验证 URL 请求使用的凭证快照。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, accountID)
	if credentialErr != nil {
		return MerchantRefundVerificationStartResult{}, credentialErr
	}
	// requestCtx、cancel 限制验证 URL 平台请求时间。
	requestCtx, cancel := context.WithTimeout(ctx, refundDetailRequestTimeout)
	defer cancel()
	// verification、verifyErr 是平台验证页面和错误。
	verification, verifyErr := service.runtime.CreateMerchantRefundVerification(requestCtx, detail, latest.RefundID)
	// credentialWarning 是验证页面响应 Cookie 协调结果。
	credentialWarning := service.persistCredential(ctx, request.UserID, accountID, detail, merchantVerificationCookieResult(verification))
	if verifyErr != nil {
		return MerchantRefundVerificationStartResult{}, verifyErr
	}
	if credentialWarning != nil || verification == nil || strings.TrimSpace(verification.AuthToken) == "" {
		return MerchantRefundVerificationStartResult{}, errors.New("创建退款支付验证失败")
	}
	// parsedURL、parseErr 是前端来源校验使用的支付宝 URL。
	parsedURL, parseErr := url.Parse(verification.VerifyURL)
	if parseErr != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return MerchantRefundVerificationStartResult{}, errors.New("退款支付验证地址无效")
	}
	// sessionID 是不可预测且不包含 authToken 的会话标识。
	sessionID, sessionErr := newMerchantVerificationID()
	if sessionErr != nil {
		return MerchantRefundVerificationStartResult{}, sessionErr
	}
	// expiresAt 是验证会话绝对过期时间。
	expiresAt := time.Now().UTC().Add(merchantRefundVerificationTTL)
	service.verificationMu.Lock()
	service.pruneMerchantVerificationsLocked(time.Now().UTC())
	service.verifications[sessionID] = merchantRefundVerificationSession{UserID: request.UserID, AccountID: accountID, OrderID: orderID,
		RefundID: latest.RefundID, AuthToken: verification.AuthToken, ExpiresAt: expiresAt}
	service.verificationMu.Unlock()
	return MerchantRefundVerificationStartResult{SessionID: sessionID, VerifyURL: verification.VerifyURL,
		VerifyOrigin: parsedURL.Scheme + "://" + parsedURL.Host, ExpiresAt: expiresAt.Unix()}, nil
}

// CompleteMerchantRefundVerification 在支付 iframe 明确成功后执行最终 Merchant 同意退款。
func (service *RefundDetailService) CompleteMerchantRefundVerification(ctx context.Context, request MerchantRefundVerificationCompleteRequest) (RefundActionResult, error) {
	// sessionID 是去空白后的短期会话标识。
	sessionID := strings.TrimSpace(request.SessionID)
	service.verificationMu.Lock()
	service.pruneMerchantVerificationsLocked(time.Now().UTC())
	// verification、exists 是短期会话和存在状态。
	verification, exists := service.verifications[sessionID]
	if !exists || verification.Consumed || verification.UserID != request.UserID || verification.AccountID != strings.TrimSpace(request.AccountID) || verification.OrderID != strings.TrimSpace(request.OrderID) {
		service.verificationMu.Unlock()
		return RefundActionResult{}, ErrRefundActionInvalid
	}
	verification.Consumed = true
	service.verifications[sessionID] = verification
	service.verificationMu.Unlock()
	defer service.deleteMerchantVerification(sessionID)
	// _, accountID、orderID、validationErr 是本地上下文和校验结果。
	_, accountID, orderID, validationErr := service.validateContext(ctx, request.UserID, verification.AccountID, verification.OrderID)
	if validationErr != nil {
		return RefundActionResult{}, validationErr
	}
	// latest、latestErr 是最终提交前重新读取的详情。
	latest, latestErr := service.fetchPlatformDetail(ctx, request.UserID, accountID, orderID)
	if latestErr != nil {
		return RefundActionResult{}, latestErr
	}
	if latest == nil || latest.RefundID != verification.RefundID || !merchantRefundActionAvailable(latest.Actions, "agree") {
		return RefundActionResult{}, ErrRefundActionInvalid
	}
	// runKey 与旧错误运行隔离，并阻止同一 Merchant 退款重复提交。
	runKey := merchantRefundRunPrefix + ":" + accountID + ":" + orderID + ":" + verification.RefundID
	// claimed、claimErr 是本次最终同意是否取得幂等执行权及持久化错误。
	claimed, claimErr := service.repository.ClaimRefundAction(ctx, runKey, accountID, orderID, time.Now().UTC().Unix())
	if claimErr != nil {
		return RefundActionResult{}, claimErr
	}
	if !claimed {
		return RefundActionResult{}, ErrRefundActionAlreadyHandled
	}
	// detail、credentialErr 是最终 Merchant 请求凭证。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, accountID)
	if credentialErr != nil {
		_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, credentialErr.Error(), false)
		return RefundActionResult{}, credentialErr
	}
	// submitCtx、cancel 限制最终退款请求时间。
	submitCtx, cancel := context.WithTimeout(ctx, refundDetailRequestTimeout)
	defer cancel()
	// platformResult、submitErr 是最终 Merchant 结果和错误。
	platformResult, submitErr := service.runtime.AgreeMerchantRefund(submitCtx, detail, verification.RefundID, verification.AuthToken)
	// credentialWarning 记录响应 Cookie 未能安全合并的非致命警告。
	credentialWarning := service.persistCredential(ctx, request.UserID, accountID, detail, refundCookieResultFromAction(platformResult))
	return service.finishMerchantRefundAction(ctx, runKey, orderID, verification.RefundID, "agree", platformResult, submitErr, credentialWarning)
}

// MerchantRefundRefuseForm 读取当前 Merchant 拒绝表单。
func (service *RefundDetailService) MerchantRefundRefuseForm(ctx context.Context, request MerchantRefundRefuseFormRequest) (MerchantRefundRefuseFormResult, error) {
	// _, accountID、orderID、validationErr 是本地上下文和校验结果。
	_, accountID, orderID, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return MerchantRefundRefuseFormResult{}, validationErr
	}
	// latest、latestErr 是当前平台退款详情和读取错误。
	latest, latestErr := service.fetchPlatformDetail(ctx, request.UserID, accountID, orderID)
	if latestErr != nil {
		return MerchantRefundRefuseFormResult{}, latestErr
	}
	if latest == nil || !merchantRefundActionAvailable(latest.Actions, "reject") {
		return MerchantRefundRefuseFormResult{}, ErrRefundActionInvalid
	}
	// detail、credentialErr 是表单读取凭证。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, accountID)
	if credentialErr != nil {
		return MerchantRefundRefuseFormResult{}, credentialErr
	}
	// requestCtx、cancel 限制动态拒绝表单读取的最长时间。
	requestCtx, cancel := context.WithTimeout(ctx, refundDetailRequestTimeout)
	defer cancel()
	// platformForm、formErr 是平台动态表单和读取错误。
	platformForm, formErr := service.runtime.FetchMerchantRefundRefuseForm(requestCtx, detail, latest.RefundID, strings.TrimSpace(request.ReasonID))
	// credentialWarning 记录表单响应 Cookie 未能安全合并的非致命警告。
	credentialWarning := service.persistCredential(ctx, request.UserID, accountID, detail, merchantRefuseFormCookieResult(platformForm))
	if formErr != nil {
		return MerchantRefundRefuseFormResult{}, formErr
	}
	if credentialWarning != nil || platformForm == nil {
		return MerchantRefundRefuseFormResult{}, errors.New("读取拒绝退款表单失败")
	}
	// result 是去除凭证变化信息后返回给前端的动态表单。
	result := platformForm.MerchantRefundRefuseFormResult
	result.RefundID, result.SelectedReasonID = latest.RefundID, strings.TrimSpace(request.ReasonID)
	result.Reasons = append([]MerchantRefundRefuseReason(nil), platformForm.Reasons...)
	return result, nil
}

// RefuseMerchantRefund 按平台最新原因上传可选图片并执行拒绝退款。
func (service *RefundDetailService) RefuseMerchantRefund(ctx context.Context, request MerchantRefundRefuseRequest) (RefundActionResult, error) {
	// imageErr 是退款凭证数量、MIME 和大小校验结果。
	if imageErr := validateShipmentImages(request.Images); imageErr != nil {
		return RefundActionResult{}, imageErr
	}
	// form、formErr 是当前原因对应的最新动态表单。
	form, formErr := service.MerchantRefundRefuseForm(ctx, MerchantRefundRefuseFormRequest{UserID: request.UserID, AccountID: request.AccountID, OrderID: request.OrderID, ReasonID: request.ReasonID})
	if formErr != nil {
		return RefundActionResult{}, formErr
	}
	// selected 是当前原因是否存在且可在 PC 处理。
	var selected *MerchantRefundRefuseReason
	for reasonIndex := range form.Reasons { // reasonIndex 定位当前待匹配的动态拒绝原因。
		if form.Reasons[reasonIndex].ID == strings.TrimSpace(request.ReasonID) {
			selected = &form.Reasons[reasonIndex]
			break
		}
	}
	if selected == nil || selected.RequiresApp {
		return RefundActionResult{}, NewValidationError("该拒绝原因只能在闲鱼官方 App 处理")
	}
	if form.ProofRequired && len(request.Images) == 0 {
		return RefundActionResult{}, NewValidationError("该拒绝原因必须上传至少 1 张图片凭证")
	}
	if len([]rune(strings.TrimSpace(request.Description))) > 200 {
		return RefundActionResult{}, NewValidationError("拒绝退款补充说明不能超过 200 字")
	}
	if request.NegotiationCents < 0 {
		return RefundActionResult{}, NewValidationError("协商退款金额不能为负数")
	}
	if form.NegotiationEnabled && request.NegotiationCents > 0 && (request.NegotiationCents < form.MinCents || request.NegotiationCents > form.MaxCents) {
		return RefundActionResult{}, NewValidationError("协商退款金额超出平台允许范围")
	}
	// _, accountID、orderID、validationErr 是最终提交前本地上下文和校验结果。
	_, accountID, orderID, validationErr := service.validateContext(ctx, request.UserID, request.AccountID, request.OrderID)
	if validationErr != nil {
		return RefundActionResult{}, validationErr
	}
	// runKey 与同意退款共用 Merchant 退款申请幂等键。
	runKey := merchantRefundRunPrefix + ":" + accountID + ":" + orderID + ":" + form.RefundID
	// claimed、claimErr 是本次最终拒绝是否取得幂等执行权及持久化错误。
	claimed, claimErr := service.repository.ClaimRefundAction(ctx, runKey, accountID, orderID, time.Now().UTC().Unix())
	if claimErr != nil {
		return RefundActionResult{}, claimErr
	}
	if !claimed {
		return RefundActionResult{}, ErrRefundActionAlreadyHandled
	}
	// detail、credentialErr 是最终拒绝请求使用的账号凭证快照和读取错误。
	detail, credentialErr := service.loadCredential(ctx, request.UserID, accountID)
	if credentialErr != nil {
		_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, credentialErr.Error(), false)
		return RefundActionResult{}, credentialErr
	}
	// submitCtx、cancel 限制图片上传和最终拒绝请求的总时间。
	submitCtx, cancel := context.WithTimeout(ctx, merchantRefundRefuseTimeout)
	defer cancel()
	// platformResult、submitErr 是最终 Merchant 拒绝结果和提交错误。
	platformResult, submitErr := service.runtime.RefuseMerchantRefund(submitCtx, detail, MerchantRefundRefusePlatformRequest{RefundID: form.RefundID,
		OrderID: orderID, ReasonID: selected.ID, Description: strings.TrimSpace(request.Description), NegotiationCents: request.NegotiationCents,
		NegotiationType: form.NegotiationType, Images: request.Images})
	// credentialWarning 记录响应 Cookie 未能安全合并的非致命警告。
	credentialWarning := service.persistCredential(ctx, request.UserID, accountID, detail, refundCookieResultFromAction(platformResult))
	return service.finishMerchantRefundAction(ctx, runKey, orderID, form.RefundID, "reject", platformResult, submitErr, credentialWarning)
}

// finishMerchantRefundAction 收口最终 Merchant 动作、凭证警告和不明确结果。
func (service *RefundDetailService) finishMerchantRefundAction(ctx context.Context, runKey, orderID, refundID, action string, result *RefundActionPlatformResult, actionErr, credentialWarning error) (RefundActionResult, error) {
	if actionErr != nil {
		if service.runtime.IsSessionExpired(actionErr) {
			_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, actionErr.Error(), false)
			return RefundActionResult{}, actionErr
		}
		if result != nil && !result.ActionAttempted {
			_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, actionErr.Error(), false)
			return RefundActionResult{}, actionErr
		}
		_ = service.finishRefundAction(ctx, runKey, "needs_review", 0, 1, actionErr.Error(), true)
		return RefundActionResult{Status: "needs_review", Message: ErrRefundActionNeedsReview.Error(), OrderID: orderID, RefundID: refundID, Action: action}, ErrRefundActionNeedsReview
	}
	if result == nil || !result.Success {
		// message 是未成功时展示并写入幂等记录的确定性结果。
		message := "闲鱼未确认退款处理成功"
		if result != nil && strings.TrimSpace(result.Message) != "" {
			message = strings.TrimSpace(result.Message)
		}
		_ = service.finishRefundAction(ctx, runKey, "failed", 0, 1, message, false)
		return RefundActionResult{Status: "failed", Message: message, OrderID: orderID, RefundID: refundID, Action: action}, ErrRefundActionInvalid
	}
	// status、message 是最终返回给前端的成功状态和平台说明。
	status, message := "succeeded", strings.TrimSpace(result.Message)
	if message == "" {
		if action == "agree" {
			message = "已同意退款申请"
		} else {
			message = "已拒绝退款申请"
		}
	}
	if credentialWarning != nil {
		status, message = "succeeded_with_warning", message+"；账号凭证已变化，未覆盖较新的登录状态"
	}
	// finishErr 是平台成功后写回本地幂等状态的错误。
	if finishErr := service.finishRefundAction(ctx, runKey, "success", 1, 0, credentialErrorText(credentialWarning), true); finishErr != nil {
		return RefundActionResult{Success: true, Status: "needs_review", Message: "闲鱼已处理退款，但本地状态保存失败，请勿重复提交", OrderID: orderID, RefundID: refundID, Action: action}, nil
	}
	return RefundActionResult{Success: true, Status: status, Message: message, OrderID: orderID, RefundID: refundID, Action: action}, nil
}

// merchantRefundActionAvailable 判断当前平台详情是否仍提供指定方向动作。
func merchantRefundActionAvailable(actions []RefundAction, kind string) bool {
	for _, action := range actions { // action 是当前平台仍允许的退款动作。
		if action.Kind == kind {
			return true
		}
	}
	return false
}

// newMerchantVerificationID 生成不可预测的短期会话标识。
func newMerchantVerificationID() (string, error) {
	// randomBytes 保存系统随机数。
	randomBytes := make([]byte, 24)
	// randomErr 是系统随机源不可用时的错误。
	if _, randomErr := rand.Read(randomBytes); randomErr != nil {
		return "", fmt.Errorf("生成退款验证会话: %w", randomErr)
	}
	return hex.EncodeToString(randomBytes), nil
}

// pruneMerchantVerificationsLocked 删除过期会话；调用方必须持有 verificationMu。
func (service *RefundDetailService) pruneMerchantVerificationsLocked(now time.Time) {
	for sessionID, verification := range service.verifications { // sessionID、verification 是当前短期支付验证会话。
		if !now.Before(verification.ExpiresAt) {
			delete(service.verifications, sessionID)
		}
	}
}

// deleteMerchantVerification 删除并释放短期 authToken。
func (service *RefundDetailService) deleteMerchantVerification(sessionID string) {
	service.verificationMu.Lock()
	delete(service.verifications, sessionID)
	service.verificationMu.Unlock()
}

// merchantVerificationCookieResult 转换验证页面 Cookie 结果。
func merchantVerificationCookieResult(result *MerchantRefundVerificationPlatformResult) *refundCookieResult {
	if result == nil {
		return nil
	}
	return &refundCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// merchantRefuseFormCookieResult 转换拒绝表单 Cookie 结果。
func merchantRefuseFormCookieResult(result *MerchantRefundRefuseFormPlatformResult) *refundCookieResult {
	if result == nil {
		return nil
	}
	return &refundCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}
