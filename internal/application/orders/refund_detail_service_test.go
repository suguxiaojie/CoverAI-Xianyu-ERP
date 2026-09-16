package orders

import (
	"context"
	"errors"
	"testing"
)

// refundDetailRepositoryFake 保存退款详情服务测试所需的本地订单和凭证。
type refundDetailRepositoryFake struct {
	// order 是归属且带退款申请证据的测试订单。
	order *Order
	// detail 是平台请求使用的测试凭证视图。
	detail *PlatformRuntimeData
	// claimed 表示退款动作幂等抢占是否成功。
	claimed bool
	// claimCalls 记录退款动作幂等抢占调用次数。
	claimCalls int
	// finishedStatus 保存退款动作最终运行状态。
	finishedStatus string
}

// ExistsOwned 返回账号归属测试结果。
func (fake *refundDetailRepositoryFake) ExistsOwned(context.Context, int64, string) (bool, error) {
	return true, nil
}

// GetOrder 返回预置测试订单。
func (fake *refundDetailRepositoryFake) GetOrder(context.Context, string) (*Order, error) {
	return fake.order, nil
}

// LockCredentials 返回无需并发协调的测试释放函数。
func (fake *refundDetailRepositoryFake) LockCredentials(string) func() {
	return func() {}
}

// LoadCookiePlatformDetail 返回预置测试凭证视图。
func (fake *refundDetailRepositoryFake) LoadCookiePlatformDetail(context.Context, string) (*PlatformRuntimeData, error) {
	return fake.detail, nil
}

// UpdateRenewalCookie 在本测试中不产生写入。
func (fake *refundDetailRepositoryFake) UpdateRenewalCookie(context.Context, string, string, string, int64) error {
	return nil
}

// ClaimRefundAction 返回预置幂等抢占结果。
func (fake *refundDetailRepositoryFake) ClaimRefundAction(context.Context, string, string, string, int64) (bool, error) {
	fake.claimCalls++
	return fake.claimed, nil
}

// FinishRefundAction 保存测试退款动作终态。
func (fake *refundDetailRepositoryFake) FinishRefundAction(_ context.Context, _ string, status string, _ int, _ int, _ string) error {
	fake.finishedStatus = status
	return nil
}

// refundDetailRuntimeFake 保存官方只读详情测试结果。
type refundDetailRuntimeFake struct {
	// result 是平台只读调用返回的公开退款详情。
	result *RefundDetailPlatformResult
	// actionResult 是真实退款动作测试返回的确定性结果。
	actionResult *RefundActionPlatformResult
	// actionErr 是测试真实退款动作返回的网络或平台错误。
	actionErr error
	// submittedKind 保存测试实际提交的动作方向。
	submittedKind string
	// verificationResult 是 Merchant PC 支付验证测试结果。
	verificationResult *MerchantRefundVerificationPlatformResult
	// merchantForm 是 Merchant 动态拒绝表单测试结果。
	merchantForm *MerchantRefundRefuseFormPlatformResult
	// merchantRefuseRequest 保存最终拒绝测试收到的图片和动态原因。
	merchantRefuseRequest MerchantRefundRefusePlatformRequest
}

// RefundDetailAvailable 声明测试替身支持官方只读详情。
func (fake refundDetailRuntimeFake) RefundDetailAvailable() bool { return true }

// CredentialAvailable 判断测试凭证存在且非空。
func (fake refundDetailRuntimeFake) CredentialAvailable(detail *PlatformRuntimeData) bool {
	return detail != nil && detail.Value != ""
}

// FetchRefundDetail 返回预置官方只读详情。
func (fake refundDetailRuntimeFake) FetchRefundDetail(context.Context, *PlatformRuntimeData, string) (*RefundDetailPlatformResult, error) {
	return fake.result, nil
}

// SubmitRefundAction 返回预置结果并记录测试动作方向。
func (fake *refundDetailRuntimeFake) SubmitRefundAction(_ context.Context, _ *PlatformRuntimeData, _, _ string, action RefundPlatformAction) (*RefundActionPlatformResult, error) {
	fake.submittedKind = action.Kind
	return fake.actionResult, fake.actionErr
}

// CreateMerchantRefundVerification 在既有退款服务测试中返回未配置结果。
func (fake *refundDetailRuntimeFake) CreateMerchantRefundVerification(context.Context, *PlatformRuntimeData, string) (*MerchantRefundVerificationPlatformResult, error) {
	if fake.verificationResult == nil {
		return nil, errors.New("merchant verification not configured")
	}
	return fake.verificationResult, nil
}

// AgreeMerchantRefund 在既有退款服务测试中返回预置动作结果。
func (fake *refundDetailRuntimeFake) AgreeMerchantRefund(context.Context, *PlatformRuntimeData, string, string) (*RefundActionPlatformResult, error) {
	return fake.actionResult, fake.actionErr
}

// FetchMerchantRefundRefuseForm 在既有退款服务测试中返回未配置结果。
func (fake *refundDetailRuntimeFake) FetchMerchantRefundRefuseForm(context.Context, *PlatformRuntimeData, string, string) (*MerchantRefundRefuseFormPlatformResult, error) {
	if fake.merchantForm == nil {
		return nil, errors.New("merchant refuse form not configured")
	}
	return fake.merchantForm, nil
}

// TestMerchantRefundVerificationCompletesFinalAgree 验证 authToken 只在服务端会话内完成最终 Merchant 退款。
func TestMerchantRefundVerificationCompletesFinalAgree(t *testing.T) {
	// repository 是允许新 Merchant 运行并记录 success 的测试存储。
	repository := &refundDetailRepositoryFake{claimed: true, order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是需要 PC 支付验证的同意动作。
	publicAction := RefundAction{Code: "sellerAgreeRefund", Name: "同意退款", Kind: "agree", Mode: "merchant_verify"}
	// runtime 返回短期验证页面和最终成功结果。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "仅退款", Seller: true, Actions: []RefundAction{publicAction}}}, verificationResult: &MerchantRefundVerificationPlatformResult{VerifyURL: "https://pcauth-site.alipay.com/PASSWORD?token=test", AuthToken: "secret-auth"}, actionResult: &RefundActionPlatformResult{Success: true, Message: "退款成功"}}
	// service 是持有短期 authToken 内存会话的测试应用服务。
	service := NewRefundDetailService(repository, runtime)
	// started、startErr 是不包含 authToken 的公开验证会话和错误。
	started, startErr := service.StartMerchantRefundVerification(context.Background(), MerchantRefundVerificationStartRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097"})
	if startErr != nil || started.SessionID == "" || started.VerifyOrigin != "https://pcauth-site.alipay.com" {
		t.Fatalf("started=%+v err=%v", started, startErr)
	}
	// completed、completeErr 是模拟 iframe code=1000 后的最终退款结果和错误。
	completed, completeErr := service.CompleteMerchantRefundVerification(context.Background(), MerchantRefundVerificationCompleteRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", SessionID: started.SessionID})
	if completeErr != nil || !completed.Success || repository.finishedStatus != "success" || repository.claimCalls != 1 {
		t.Fatalf("completed=%+v finished=%s claims=%d err=%v", completed, repository.finishedStatus, repository.claimCalls, completeErr)
	}
}

// TestMerchantRefundRefuseUsesDynamicReason 验证拒绝退款只提交平台最新且无需手机凭证的原因。
func TestMerchantRefundRefuseUsesDynamicReason(t *testing.T) {
	// repository 是允许新 Merchant 拒绝运行的测试存储。
	repository := &refundDetailRepositoryFake{claimed: true, order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是 Merchant 拒绝动作。
	publicAction := RefundAction{Code: "sellerRejectRefund", Name: "拒绝退款", Kind: "reject", Mode: "merchant_refuse"}
	// runtime 返回一个可直接 PC 处理的动态原因和最终成功结果。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "仅退款", Seller: true, Actions: []RefundAction{publicAction}}}, merchantForm: &MerchantRefundRefuseFormPlatformResult{MerchantRefundRefuseFormResult: MerchantRefundRefuseFormResult{Reasons: []MerchantRefundRefuseReason{{ID: "11", Name: "已协商", RequiresApp: false}}}}, actionResult: &RefundActionPlatformResult{Success: true, Message: "提交成功"}}
	// result、refuseErr 是最终 Merchant 拒绝结果和错误。
	result, refuseErr := NewRefundDetailService(repository, runtime).RefuseMerchantRefund(context.Background(), MerchantRefundRefuseRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ReasonID: "11", Description: "已协商"})
	if refuseErr != nil || !result.Success || result.Action != "reject" || repository.finishedStatus != "success" {
		t.Fatalf("result=%+v finished=%s err=%v", result, repository.finishedStatus, refuseErr)
	}
}

// TestMerchantRefundRefuseRequiresAndForwardsProofImages 验证必填凭证在 Claim 前拦截，并把合法图片交给运行时上传。
func TestMerchantRefundRefuseRequiresAndForwardsProofImages(t *testing.T) {
	// repository 是允许带凭证 Merchant 拒绝运行的测试存储。
	repository := &refundDetailRepositoryFake{claimed: true, order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是 Merchant 拒绝动作。
	publicAction := RefundAction{Code: "sellerRejectRefund", Name: "拒绝退款", Kind: "reject", Mode: "merchant_refuse"}
	// runtime 返回必须上传图片的动态原因和最终成功结果。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "仅退款", Seller: true, Actions: []RefundAction{publicAction}}}, merchantForm: &MerchantRefundRefuseFormPlatformResult{MerchantRefundRefuseFormResult: MerchantRefundRefuseFormResult{Reasons: []MerchantRefundRefuseReason{{ID: "11", Name: "已发货，无需邮寄"}}, ProofRequired: true}}, actionResult: &RefundActionPlatformResult{Success: true, ActionAttempted: true, Message: "提交成功"}}
	// service 是当前带图片退款服务。
	service := NewRefundDetailService(repository, runtime)
	// _, missingErr 只关心未上传必填图片时的本地拒绝。
	_, missingErr := service.RefuseMerchantRefund(context.Background(), MerchantRefundRefuseRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ReasonID: "11"})
	if missingErr == nil || repository.claimCalls != 0 {
		t.Fatalf("missingErr=%v claims=%d", missingErr, repository.claimCalls)
	}
	// image 是小于 3MB 的内存 PNG 凭证。
	image := ShipmentEvidenceImage{Filename: "proof.png", ContentType: "image/png", Data: []byte("png")}
	// result、refuseErr 是带必填图片的最终拒绝结果和错误。
	result, refuseErr := service.RefuseMerchantRefund(context.Background(), MerchantRefundRefuseRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ReasonID: "11", Images: []ShipmentEvidenceImage{image}})
	if refuseErr != nil || !result.Success || len(runtime.merchantRefuseRequest.Images) != 1 || runtime.merchantRefuseRequest.Images[0].Filename != "proof.png" {
		t.Fatalf("result=%+v request=%+v err=%v", result, runtime.merchantRefuseRequest, refuseErr)
	}
}

// RefuseMerchantRefund 在既有退款服务测试中返回预置动作结果。
func (fake *refundDetailRuntimeFake) RefuseMerchantRefund(_ context.Context, _ *PlatformRuntimeData, request MerchantRefundRefusePlatformRequest) (*RefundActionPlatformResult, error) {
	fake.merchantRefuseRequest = request
	return fake.actionResult, fake.actionErr
}

// TestRefundDetailServiceQuarantinesAmbiguousActionResult 验证网络结果不明确时进入人工核对且不自动重放。
func TestRefundDetailServiceQuarantinesAmbiguousActionResult(t *testing.T) {
	// repository 是允许首次动作并记录 needs_review 的测试存储。
	repository := &refundDetailRepositoryFake{claimed: true, order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是平台最新详情中的拒绝退款动作。
	publicAction := RefundAction{Code: "sellerRejectRefund", Name: "拒绝退款", Kind: "reject", Mode: "direct"}
	// runtime 模拟动作发出后网络断开，结果不能安全判断。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "退款", Seller: true, Actions: []RefundAction{publicAction}}, PlatformActions: []RefundPlatformAction{{RefundAction: publicAction, APIName: "mtop.taobao.idle.refund.reject.refund", APIVersion: "1.0"}}}, actionErr: errors.New("network reset")}
	// _, actionErr 只关心人工核对错误和运行终态。
	_, actionErr := NewRefundDetailService(repository, runtime).Act(context.Background(), RefundActionRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ActionCode: "sellerRejectRefund"})
	if !errors.Is(actionErr, ErrRefundActionNeedsReview) || repository.finishedStatus != "needs_review" {
		t.Fatalf("error=%v finished=%s", actionErr, repository.finishedStatus)
	}
}

// TestRefundDetailServiceRejectsDuplicateActionClaim 验证同一退款申请不会并发提交相反动作。
func TestRefundDetailServiceRejectsDuplicateActionClaim(t *testing.T) {
	// repository 的 claimed=false 模拟同一退款申请已被另一请求抢占。
	repository := &refundDetailRepositoryFake{order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是本轮尝试提交的同意动作。
	publicAction := RefundAction{Code: "sellerAgreeRefund", Name: "同意退款", Kind: "agree", Mode: "direct"}
	// runtime 提供最新动作，但幂等抢占应在真实 submit 前停止。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "仅退款", Seller: true}, PlatformActions: []RefundPlatformAction{{RefundAction: publicAction, APIName: "mtop.taobao.idle.refund.agree.refund", APIVersion: "1.0"}}}}
	// _, actionErr 只关心重复处理错误。
	_, actionErr := NewRefundDetailService(repository, runtime).Act(context.Background(), RefundActionRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ActionCode: "sellerAgreeRefund"})
	if !errors.Is(actionErr, ErrRefundActionAlreadyHandled) || runtime.submittedKind != "" {
		t.Fatalf("error=%v submitted=%s", actionErr, runtime.submittedKind)
	}
}

// TestRefundDetailServiceRequiresOfficialAuthenticationBeforeClaim 验证原生认证动作不会创建本地成功或幂等运行。
func TestRefundDetailServiceRequiresOfficialAuthenticationBeforeClaim(t *testing.T) {
	// repository 保存仍处于退款中的订单；claimed 即使为 true 也不应进入抢占步骤。
	repository := &refundDetailRepositoryFake{claimed: true, order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是需要闲鱼原生支付密码认证的同意动作。
	publicAction := RefundAction{Code: "sellerAgreeRefund", Name: "同意退款", Kind: "agree", Mode: "official"}
	// runtime 只返回最新官方动作，不应收到真实 submit。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "仅退款", Seller: true, Actions: []RefundAction{publicAction}}, PlatformActions: []RefundPlatformAction{{RefundAction: publicAction}}}}
	// _, actionErr 只关心官方认证分类和未提交事实。
	_, actionErr := NewRefundDetailService(repository, runtime).Act(context.Background(), RefundActionRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ActionCode: "sellerAgreeRefund"})
	if !errors.Is(actionErr, ErrRefundActionRequiresOfficial) || runtime.submittedKind != "" || repository.finishedStatus != "" || repository.claimCalls != 0 {
		t.Fatalf("error=%v submitted=%s finished=%s claims=%d", actionErr, runtime.submittedKind, repository.finishedStatus, repository.claimCalls)
	}
}

// PersistCookieSession 声明测试请求未产生完整 Cookie Jar 变化。
func (fake refundDetailRuntimeFake) PersistCookieSession(context.Context, *PlatformRuntimeData, RefreshCookieUpdate) (string, bool, bool, error) {
	return "", false, true, nil
}

// UpdateRunningCookie 在本测试中不更新真实账号运行时。
func (fake refundDetailRuntimeFake) UpdateRunningCookie(context.Context, string, string) {}

// RecoverExpiredSession 在本测试中不启动会话恢复。
func (fake refundDetailRuntimeFake) RecoverExpiredSession(context.Context, string, error) bool {
	return false
}

// IsSessionExpired 在本测试中不把普通错误归类为会话过期。
func (fake refundDetailRuntimeFake) IsSessionExpired(error) bool { return false }

// TestRefundDetailServiceReturnsReadOnlyOfficialFields 验证归属退款订单只返回平台公开详情。
func TestRefundDetailServiceReturnsReadOnlyOfficialFields(t *testing.T) {
	// repository 是带退款申请证据和归属凭证的测试存储。
	repository := &refundDetailRepositoryFake{order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunded", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// runtime 是返回退款原因、金额和买家说明的只读平台替身。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Reason: "协商一致退款", Amount: "135.00", Type: "仅退款", BuyerDescription: "升级套餐"}}}
	// result、detailErr 是应用服务返回的公开详情和错误。
	result, detailErr := NewRefundDetailService(repository, runtime).Get(context.Background(), RefundDetailRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097"})
	if detailErr != nil {
		t.Fatal(detailErr)
	}
	if result.AccountID != "seller-1" || result.OrderID != "3316374662163136097" || result.Reason != "协商一致退款" || result.Amount != "135.00" || result.BuyerDescription != "升级套餐" {
		t.Fatalf("refund detail=%+v", result)
	}
}

// TestRefundDetailServiceRejectsOrderWithoutRefundEvidence 验证普通订单不会触发平台退款详情请求。
func TestRefundDetailServiceRejectsOrderWithoutRefundEvidence(t *testing.T) {
	// repository 是没有退款申请证据的普通完成订单存储。
	repository := &refundDetailRepositoryFake{order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "completed"}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// _, detailErr 只关心本地资格拒绝结果。
	_, detailErr := NewRefundDetailService(repository, &refundDetailRuntimeFake{}).Get(context.Background(), RefundDetailRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097"})
	if !errors.Is(detailErr, ErrRefundDetailNotEligible) {
		t.Fatalf("error=%v", detailErr)
	}
}

// TestRefundDetailServiceSubmitsLatestSellerAction 验证二次确认后只提交平台最新详情中的同意动作。
func TestRefundDetailServiceSubmitsLatestSellerAction(t *testing.T) {
	// repository 是允许首次退款动作并记录终态的测试存储。
	repository := &refundDetailRepositoryFake{claimed: true, order: &Order{OrderID: "3316374662163136097", CookieID: "seller-1", OrderStatus: "refunding", RefundRequested: true}, detail: &PlatformRuntimeData{ID: "seller-1", UserID: 7, Value: "cookie"}}
	// publicAction 是平台当前公开的同意退款动作。
	publicAction := RefundAction{Code: "sellerAgreeRefund", Name: "同意退款", Kind: "agree", Mode: "direct", ConfirmTitle: "确认同意退款"}
	// runtime 是同时返回最新动作并明确确认提交成功的平台替身。
	runtime := &refundDetailRuntimeFake{result: &RefundDetailPlatformResult{RefundDetailResult: RefundDetailResult{RefundID: "refund-1", Type: "仅退款", Seller: true, Actions: []RefundAction{publicAction}}, PlatformActions: []RefundPlatformAction{{RefundAction: publicAction, APIName: "mtop.taobao.idle.refund.agree.refund", APIVersion: "1.0", Params: map[string]any{"refundId": "refund-1"}}}}, actionResult: &RefundActionPlatformResult{Success: true, Message: "已同意退款申请"}}
	// result、actionErr 是应用服务真实退款动作结果和错误。
	result, actionErr := NewRefundDetailService(repository, runtime).Act(context.Background(), RefundActionRequest{UserID: 7, AccountID: "seller-1", OrderID: "3316374662163136097", ActionCode: "sellerAgreeRefund"})
	if actionErr != nil {
		t.Fatal(actionErr)
	}
	if !result.Success || result.Action != "agree" || runtime.submittedKind != "agree" || repository.finishedStatus != "success" {
		t.Fatalf("result=%+v submitted=%s finished=%s", result, runtime.submittedKind, repository.finishedStatus)
	}
}
