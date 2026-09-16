package adapter

import (
	"context"
	"errors"
	"strings"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/xianyu/mtop"
)

// refundDetailMTop 是订单运行时可选的官方只读退款详情能力。
type refundDetailMTop interface {
	// FetchRefundDetail 读取平台退款原因、金额和买家说明，不执行退款动作。
	FetchRefundDetail(context.Context, string, string) (*mtop.RefundDetailResult, error)
	// SubmitRefundAction 使用详情刚下发的动态描述执行普通退款动作。
	SubmitRefundAction(context.Context, string, string, string, mtop.RefundAction) (*mtop.RefundActionResult, error)
	// CreateMerchantRefundVerification 创建 PC 支付验证页面。
	CreateMerchantRefundVerification(context.Context, string, string) (*mtop.MerchantRefundVerification, error)
	// AgreeMerchantRefund 执行最终 Merchant 同意退款。
	AgreeMerchantRefund(context.Context, string, string, string) (*mtop.MerchantRefundResult, error)
	// FetchMerchantRefundRefuseForm 读取 Merchant 动态拒绝表单。
	FetchMerchantRefundRefuseForm(context.Context, string, string, string) (*mtop.MerchantRefundRefuseForm, error)
	// RefuseMerchantRefund 执行最终 Merchant 拒绝退款。
	RefuseMerchantRefund(context.Context, string, mtop.MerchantRefundRefuseRequest) (*mtop.MerchantRefundResult, error)
}

// refundProofUploadMTop 是 Merchant 拒绝退款复用的闲鱼图片上传能力。
type refundProofUploadMTop interface {
	// UploadShipmentEvidenceImage 使用 fleamarket 作用域上传一张 PNG／JPEG 凭证。
	UploadShipmentEvidenceImage(context.Context, string, mtop.ShipmentEvidenceImage) (*mtop.ShipmentEvidenceUpload, error)
}

// CreateMerchantRefundVerification 创建 PC 支付验证页面并收集 Cookie 变化。
func (r *OrderRuntime) CreateMerchantRefundVerification(ctx context.Context, detail *orderapp.PlatformRuntimeData, refundID string) (*orderapp.MerchantRefundVerificationPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("退款验证缺少账号凭证")
	}
	// requester、available 是 Merchant 验证能力和可用状态。
	requester, available := r.mtopClient().(refundDetailMTop)
	if !available {
		return nil, orderapp.ErrRefundDetailUnavailable
	}
	// requestCtx、session 保存请求 Cookie Jar 上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是验证页面响应和错误。
	response, requestErr := requester.CreateMerchantRefundVerification(requestCtx, detail.Value, refundID)
	// result 保存应用层验证页面投影。
	result := &orderapp.MerchantRefundVerificationPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.VerifyURL, result.AuthToken, result.UpdatedCookies = response.VerifyURL, response.AuthToken, response.UpdatedCookies
	}
	return result, requestErr
}

// AgreeMerchantRefund 使用支付验证授权执行最终 Merchant 同意退款。
func (r *OrderRuntime) AgreeMerchantRefund(ctx context.Context, detail *orderapp.PlatformRuntimeData, refundID, authToken string) (*orderapp.RefundActionPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("同意退款缺少账号凭证")
	}
	// requester、available 是 Merchant 同意能力和可用状态。
	requester, available := r.mtopClient().(refundDetailMTop)
	if !available {
		return nil, orderapp.ErrRefundDetailUnavailable
	}
	// requestCtx、session 保存请求 Cookie Jar 上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是最终同意响应和错误。
	response, requestErr := requester.AgreeMerchantRefund(requestCtx, detail.Value, refundID, authToken)
	// result 保存应用层最终动作投影。
	result := &orderapp.RefundActionPlatformResult{ActionAttempted: true, CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Success, result.Message, result.UpdatedCookies = response.Success, response.Message, response.UpdatedCookies
	}
	return result, requestErr
}

// FetchMerchantRefundRefuseForm 读取 Merchant 动态拒绝表单并收集 Cookie 变化。
func (r *OrderRuntime) FetchMerchantRefundRefuseForm(ctx context.Context, detail *orderapp.PlatformRuntimeData, refundID, reasonID string) (*orderapp.MerchantRefundRefuseFormPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("拒绝退款表单缺少账号凭证")
	}
	// requester、available 是 Merchant 拒绝表单能力和可用状态。
	requester, available := r.mtopClient().(refundDetailMTop)
	if !available {
		return nil, orderapp.ErrRefundDetailUnavailable
	}
	// requestCtx、session 保存请求 Cookie Jar 上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是拒绝表单响应和错误。
	response, requestErr := requester.FetchMerchantRefundRefuseForm(requestCtx, detail.Value, refundID, reasonID)
	// result 保存应用层动态表单投影。
	result := &orderapp.MerchantRefundRefuseFormPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Reasons = make([]orderapp.MerchantRefundRefuseReason, 0, len(response.Reasons))
		for _, reason := range response.Reasons { // reason 是当前平台动态拒绝原因。
			result.Reasons = append(result.Reasons, orderapp.MerchantRefundRefuseReason{ID: reason.ID, Name: reason.Name, RequiresApp: reason.RequiresApp})
		}
		result.ProofRequired, result.ProofPlaceholder = response.Proof.Required, response.Proof.Placeholder
		result.NegotiationEnabled, result.NegotiationType = response.Negotiation.Enabled, response.Negotiation.Type
		result.MinCents, result.MaxCents, result.UpdatedCookies = response.Negotiation.MinCents, response.Negotiation.MaxCents, response.UpdatedCookies
	}
	return result, requestErr
}

// RefuseMerchantRefund 依次上传图片后执行最终 Merchant 拒绝退款并收集 Cookie 变化。
func (r *OrderRuntime) RefuseMerchantRefund(ctx context.Context, detail *orderapp.PlatformRuntimeData, request orderapp.MerchantRefundRefusePlatformRequest) (*orderapp.RefundActionPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("拒绝退款缺少账号凭证")
	}
	// client 是本次上传和拒绝共用的平台客户端。
	client := r.mtopClient()
	// requester、available 是 Merchant 拒绝动作能力和可用状态。
	requester, available := client.(refundDetailMTop)
	if !available {
		return nil, orderapp.ErrRefundDetailUnavailable
	}
	// requestCtx、session 保存请求 Cookie Jar 上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// result 保存上传和最终拒绝共同产生的 Cookie 会话变化。
	result := &orderapp.RefundActionPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	// proofURLs 按用户选择顺序保存闲鱼上传后的平台图片地址。
	proofURLs := make([]string, 0, len(request.Images))
	// updatedCookies 保存没有完整 Cookie Jar 时最后一次平台响应的平面 Cookie。
	updatedCookies := ""
	if len(request.Images) > 0 {
		// uploader、uploadAvailable 是当前客户端图片上传能力和可用状态。
		uploader, uploadAvailable := client.(refundProofUploadMTop)
		if !uploadAvailable {
			return result, errors.New("当前平台客户端不支持上传退款凭证")
		}
		for _, image := range request.Images { // image 是当前待上传的内存退款凭证。
			// upload、uploadErr 是当前图片的官方上传结果和错误。
			upload, uploadErr := uploader.UploadShipmentEvidenceImage(requestCtx, detail.Value, mtop.ShipmentEvidenceImage{
				Filename: image.Filename, ContentType: image.ContentType, Data: image.Data,
			})
			if uploadErr != nil {
				result.UpdatedCookies = updatedCookies
				return result, uploadErr
			}
			if upload == nil || strings.TrimSpace(upload.URL) == "" {
				result.UpdatedCookies = updatedCookies
				return result, errors.New("退款凭证上传响应缺少图片地址")
			}
			proofURLs = append(proofURLs, strings.TrimSpace(upload.URL))
			if upload.UpdatedCookies != "" {
				updatedCookies = upload.UpdatedCookies
			}
		}
	}
	result.ActionAttempted = true
	// response、requestErr 是最终拒绝响应和错误。
	response, requestErr := requester.RefuseMerchantRefund(requestCtx, detail.Value, mtop.MerchantRefundRefuseRequest{RefundID: request.RefundID,
		OrderID: request.OrderID, ReasonID: request.ReasonID, Description: request.Description,
		ProofURLs: proofURLs, NegotiationCents: request.NegotiationCents, NegotiationType: request.NegotiationType})
	result.UpdatedCookies = updatedCookies
	if response != nil {
		result.Success, result.Message = response.Success, response.Message
		if response.UpdatedCookies != "" {
			result.UpdatedCookies = response.UpdatedCookies
		}
	}
	return result, requestErr
}

// RefundDetailAvailable 判断当前平台客户端是否实现官方只读退款详情接口。
func (r *OrderRuntime) RefundDetailAvailable() bool {
	// client 是当前订单运行时使用的平台客户端。
	client := r.mtopClient()
	// _, available 只记录可选读取能力，不暴露具体客户端。
	_, available := client.(refundDetailMTop)
	return available
}

// FetchRefundDetail 使用给定凭证读取官方退款详情，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) FetchRefundDetail(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID string) (*orderapp.RefundDetailPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("退款详情请求缺少账号凭证")
	}
	// requester、available 是当前平台客户端的只读退款详情能力和可用状态。
	requester, available := r.mtopClient().(refundDetailMTop)
	if !available {
		return nil, orderapp.ErrRefundDetailUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是官方退款详情响应和请求错误。
	response, requestErr := requester.FetchRefundDetail(requestCtx, detail.Value, orderID)
	// result 始终携带请求观察到的 Cookie 会话变化。
	result := &orderapp.RefundDetailPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.RefundDetailResult = orderapp.RefundDetailResult{OrderID: response.OrderID, RefundID: response.RefundID,
			Status: response.RefundStatus, StatusText: response.RefundStatusText, Type: response.RefundType, Reason: response.RefundReason, Amount: response.RefundAmount,
			ApplyTime: response.RefundApplyTime, BuyerDescription: response.BuyerDescription,
			BuyerImages: append([]string(nil), response.BuyerImages...), BuyerVideos: append([]string(nil), response.BuyerVideos...), Seller: response.Seller}
		result.Actions = make([]orderapp.RefundAction, 0, len(response.Actions))
		result.PlatformActions = make([]orderapp.RefundPlatformAction, 0, len(response.Actions))
		// action 是当前平台动态退款动作。
		for _, action := range response.Actions {
			// publicAction 是前端可见且不含 MTOP 参数的动作描述。
			publicAction := orderapp.RefundAction{Code: action.Code, Name: action.Name, Kind: action.Kind, Mode: action.Mode, ConfirmTitle: action.ConfirmTitle, ConfirmDescription: action.ConfirmDescription}
			result.Actions = append(result.Actions, publicAction)
			result.PlatformActions = append(result.PlatformActions, orderapp.RefundPlatformAction{RefundAction: publicAction,
				APIName: action.APIName, APIVersion: action.APIVersion, Params: cloneRefundActionParams(action.Params)})
		}
		result.UpdatedCookies = response.UpdatedCookies
	}
	return result, requestErr
}

// SubmitRefundAction 使用给定凭证和最新平台动作描述执行普通同意／拒绝退款。
func (r *OrderRuntime) SubmitRefundAction(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID, refundID string, action orderapp.RefundPlatformAction) (*orderapp.RefundActionPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("退款动作请求缺少账号凭证")
	}
	// requester、available 是当前平台客户端的退款详情／动作能力和可用状态。
	requester, available := r.mtopClient().(refundDetailMTop)
	if !available {
		return nil, orderapp.ErrRefundDetailUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台退款动作响应和请求错误。
	response, requestErr := requester.SubmitRefundAction(requestCtx, detail.Value, orderID, refundID, mtop.RefundAction{Code: action.Code,
		Name: action.Name, Kind: action.Kind, Mode: action.Mode, ConfirmTitle: action.ConfirmTitle, ConfirmDescription: action.ConfirmDescription,
		APIName: action.APIName, APIVersion: action.APIVersion, Params: cloneRefundActionParams(action.Params)})
	// result 始终携带请求观察到的 Cookie 会话变化。
	result := &orderapp.RefundActionPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Success, result.Message, result.RequiresOfficial, result.UpdatedCookies = response.Success, response.Message, response.RequiresOfficial, response.UpdatedCookies
	}
	return result, requestErr
}

// cloneRefundActionParams 复制动态退款参数，避免适配层和平台客户端共享可变 map。
func cloneRefundActionParams(params map[string]any) map[string]any {
	// cloned 保存动态参数的独立浅复制。
	cloned := make(map[string]any, len(params))
	// key、value 是当前待复制的动态参数。
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}
