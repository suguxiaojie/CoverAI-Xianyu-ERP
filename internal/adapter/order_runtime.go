package adapter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	accountmanager "xianyu-go/internal/account"
	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/cookierefresh"
	"xianyu-go/internal/xianyu/mtop"
)

// OrderAutomation 定义订单手动发货所需的最小自动化能力，避免订单适配器依赖具体 Center 类型。
type OrderAutomation interface {
	// ManualFullDelivery 执行一次订单完整发货并返回实际发送数量。
	ManualFullDelivery(context.Context, *db.Order) (int, error)
	// ScheduleRedFlowerAfterShipment 在明确发货事实后创建秒级求花任务。
	ScheduleRedFlowerAfterShipment(context.Context, string) error
}

// OrderNotifier 定义订单发货结果所需的最小通知能力，通知实现不进入订单应用层。
type OrderNotifier interface {
	// NotifyDelivery 发送订单发货结果通知。
	NotifyDelivery(string, string, string, string, string, string)
}

// orderDetailMTop 是订单运行时需要的可选详情接口能力。
type orderDetailMTop interface {
	// FetchOrderDetail 请求订单详情并返回平台响应中的订单字段。
	FetchOrderDetail(context.Context, string, string) (*mtop.OrderDetailResult, error)
}

// redFlowerMTop 是订单运行时可选的小红花官方动作能力。
type redFlowerMTop interface {
	// RequestRedFlower 请求平台向订单买家发送官方求花卡片。
	RequestRedFlower(context.Context, string, string, string) (*mtop.RedFlowerRequestResult, error)
}

// adjustPriceMTop 是订单运行时可选的网页版待付款改价能力。
type adjustPriceMTop interface {
	// RenderOrderAdjustPrice 读取平台动态改价表单。
	RenderOrderAdjustPrice(context.Context, string, string) (*mtop.AdjustPriceFormResult, error)
	// SubmitOrderAdjustPrice 按 render 字段提交整数分字符串。
	SubmitOrderAdjustPrice(context.Context, string, string, map[string]string) (*mtop.AdjustPriceSubmitResult, error)
}

// closeOrderMTop 是订单运行时可选的网页版卖家关单能力。
type closeOrderMTop interface {
	// FetchCloseOrderReasons 读取平台动态关闭原因。
	FetchCloseOrderReasons(context.Context, string, string) (*mtop.CloseOrderReasonsResult, error)
	// CloseOrderBySeller 使用平台原因执行卖家关单。
	CloseOrderBySeller(context.Context, string, string, string) (*mtop.CloseOrderSellerResult, error)
}

// shipmentEvidenceMTop 是当前卖家工作台凭证上传和无需寄件提交的可选平台能力。
type shipmentEvidenceMTop interface {
	// UploadShipmentEvidenceImage 使用 fleamarket 作用域上传一张 PNG/JPEG 凭证。
	UploadShipmentEvidenceImage(context.Context, string, mtop.ShipmentEvidenceImage) (*mtop.ShipmentEvidenceUpload, error)
	// ConsignEvidenceContext 使用描述和图片 URL 数组提交无需寄件。
	ConsignEvidenceContext(context.Context, string, mtop.ConsignEvidenceRequest) (*mtop.ConsignEvidenceResult, error)
}

// OrderRuntimeHooks 定义 Server 运行时注入订单适配器的非持久化能力。
type OrderRuntimeHooks struct {
	// Client 返回当前使用的平台客户端。
	Client func() mtop.Client
	// ClientAvailable 判断平台客户端是否已由 Server 显式装配。
	ClientAvailable func() bool
	// AccountRunning 判断账号运行实例是否在线。
	AccountRunning func(string) bool
	// AutomationReady 判断完整发货自动化依赖是否可用。
	AutomationReady func() bool
	// ManualFullDelivery 执行完整自动化发货；订单模型转换由装配回调负责。
	ManualFullDelivery func(context.Context, *orderapp.Order) (int, error)
	// ScheduleRedFlowerAfterShipment 在手动确认发货成功后委托自动化中心创建任务。
	ScheduleRedFlowerAfterShipment func(context.Context, string) error
	// UpdateRunningCookie 同步平台响应中的最新 Cookie 到账号运行实例。
	UpdateRunningCookie func(context.Context, string, string)
	// NotifyDelivery 发送手动发货结果通知。
	NotifyDelivery func(string, string, string, string, string)
	// RecoverExpiredSession 处理平台会话过期。
	RecoverExpiredSession func(context.Context, string, error) bool
	// ReportPersistenceFailure 记录本地订单状态写入失败。
	ReportPersistenceFailure func(string, error)
}

// NewOrderRuntimeHooks 将账号、自动化和通知依赖转换为订单运行时回调；闭包只存在于 adapter 装配边界。
func NewOrderRuntimeHooks(client func() mtop.Client, manager *accountmanager.Manager, automation OrderAutomation, notifier OrderNotifier, updateCookie func(context.Context, string, string), recoverSession func(context.Context, string, error) bool) OrderRuntimeHooks {
	return OrderRuntimeHooks{
		Client:          client,
		ClientAvailable: func() bool { return client != nil && client() != nil },
		AccountRunning:  AccountRunningLookup(manager),
		AutomationReady: func() bool { return manager != nil && automation != nil },
		ManualFullDelivery: func(ctx context.Context, order *orderapp.Order) (int, error) {
			if automation == nil {
				return 0, errors.New("自动化中心未初始化")
			}
			return automation.ManualFullDelivery(ctx, OrderForAutomation(order))
		},
		ScheduleRedFlowerAfterShipment: func(ctx context.Context, orderID string) error {
			if automation == nil {
				return errors.New("自动化中心未初始化")
			}
			return automation.ScheduleRedFlowerAfterShipment(ctx, orderID)
		},
		UpdateRunningCookie: updateCookie,
		NotifyDelivery: func(cookieID, buyerID, itemID, chatID, message string) {
			if notifier != nil {
				notifier.NotifyDelivery(cookieID, "", buyerID, itemID, message, chatID)
			}
		},
		RecoverExpiredSession: recoverSession,
	}
}

// OrderRuntime 将订单应用 Port 适配到数据库凭证、MTOP 和账号运行时。
type OrderRuntime struct {
	// store 提供订单确认发货所需的平台凭证读取和 Cookie 写回能力。
	store *db.Store
	// hooks 保存 Server 注入的运行时回调，不保存 HTTP 或 Server 对象。
	hooks OrderRuntimeHooks
	// reconciliation 保存外部动作成功后的补偿记录端口。
	reconciliation orderapp.ReconciliationRecorder
	// logger 记录不含凭证的订单持久化错误。
	logger *slog.Logger
}

// NewOrderRuntime 构造订单平台与运行时适配器。
func NewOrderRuntime(store *db.Store, hooks OrderRuntimeHooks, reconciliation orderapp.ReconciliationRecorder, logger *slog.Logger) *OrderRuntime {
	// resolvedLogger 保存订单适配器使用的日志器。
	resolvedLogger := logger
	if resolvedLogger == nil {
		resolvedLogger = slog.Default()
	}
	return &OrderRuntime{store: store, hooks: hooks, reconciliation: reconciliation, logger: resolvedLogger}
}

// AccountRunning 判断指定账号是否在线运行。
func (r *OrderRuntime) AccountRunning(cookieID string) bool {
	return r != nil && r.hooks.AccountRunning != nil && r.hooks.AccountRunning(cookieID)
}

// AutomationReady 判断完整发货自动化依赖是否已装配。
func (r *OrderRuntime) AutomationReady() bool {
	return r != nil && r.hooks.AutomationReady != nil && r.hooks.AutomationReady()
}

// ManualFullDelivery 执行完整自动化发货。
func (r *OrderRuntime) ManualFullDelivery(ctx context.Context, order *orderapp.Order) (int, error) {
	if r == nil || r.hooks.ManualFullDelivery == nil {
		return 0, errors.New("自动化中心未初始化")
	}
	return r.hooks.ManualFullDelivery(ctx, order)
}

// ScheduleRedFlowerAfterShipment 将手动确认发货成功事实交给自动化中心的秒级调度器。
func (r *OrderRuntime) ScheduleRedFlowerAfterShipment(ctx context.Context, orderID string) error {
	if r == nil || r.hooks.ScheduleRedFlowerAfterShipment == nil {
		return errors.New("自动求花调度器未初始化")
	}
	return r.hooks.ScheduleRedFlowerAfterShipment(ctx, orderID)
}

// MTopAvailable 判断平台客户端是否已由 Server 显式装配。
func (r *OrderRuntime) MTopAvailable() bool {
	return r != nil && r.hooks.ClientAvailable != nil && r.hooks.ClientAvailable()
}

// RedFlowerAvailable 判断当前平台客户端是否实现官方求花接口。
func (r *OrderRuntime) RedFlowerAvailable() bool {
	// client 是当前订单运行时使用的平台客户端。
	client := r.mtopClient()
	// _, available 只记录可选接口能力，不向应用层暴露具体客户端。
	_, available := client.(redFlowerMTop)
	return available
}

// PriceAdjustmentAvailable 判断当前平台客户端是否同时实现网页版改价 render 和 submit。
func (r *OrderRuntime) PriceAdjustmentAvailable() bool {
	// client 是当前订单运行时使用的平台客户端。
	client := r.mtopClient()
	// _, available 只记录可选接口能力，不向应用层暴露具体客户端。
	_, available := client.(adjustPriceMTop)
	return available
}

// CloseOrderAvailable 判断当前平台客户端是否同时实现关闭原因和卖家关单接口。
func (r *OrderRuntime) CloseOrderAvailable() bool {
	// client 是当前订单运行时使用的平台客户端。
	client := r.mtopClient()
	// _, available 只记录可选接口能力，不暴露具体客户端。
	_, available := client.(closeOrderMTop)
	return available
}

// ShipmentEvidenceAvailable 判断当前平台客户端是否同时实现凭证上传和当前无需寄件接口。
func (r *OrderRuntime) ShipmentEvidenceAvailable() bool {
	// client 是当前订单运行时使用的平台客户端。
	client := r.mtopClient()
	// _, available 只记录可选发货能力，不向应用层暴露具体客户端。
	_, available := client.(shipmentEvidenceMTop)
	// _, detailAvailable 确保最终提交前可以读取最新平台订单状态。
	_, detailAvailable := client.(orderDetailMTop)
	return available && detailAvailable
}

// mtopClient 返回订单流程使用的平台客户端。
func (r *OrderRuntime) mtopClient() mtop.Client {
	if r == nil || r.hooks.Client == nil {
		return nil
	}
	return r.hooks.Client()
}

// ConfirmShipment 使用当前账号凭证调用平台确认发货接口。
func (r *OrderRuntime) ConfirmShipment(ctx context.Context, cookieID, orderID string, userID int64) orderapp.ConsignResult {
	// success、messages、runtimeCookie、runtimeCookieChanged、callErr 保存确认发货结果及凭证变化。
	success, messages, runtimeCookie, runtimeCookieChanged, callErr := r.consignWithCurrentCookie(ctx, cookieID, orderID, userID)
	return orderapp.ConsignResult{Success: success, Messages: messages, RuntimeCookie: runtimeCookie, RuntimeCookieChanged: runtimeCookieChanged, Err: callErr}
}

// FetchShipmentStatus 使用给定卖家凭证读取最新订单状态，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) FetchShipmentStatus(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID string) (*orderapp.ShipmentStatusPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("发货状态复核缺少账号凭证")
	}
	// fetcher、available 是当前平台客户端的订单详情能力及可用状态。
	fetcher, available := r.mtopClient().(orderDetailMTop)
	if !available {
		return nil, orderapp.ErrShipmentUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台订单详情响应和请求错误。
	response, requestErr := fetcher.FetchOrderDetail(requestCtx, detail.Value, orderID)
	// result 始终携带本次请求观察到的 Cookie 会话变化。
	result := &orderapp.ShipmentStatusPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.OrderStatus, result.UpdatedCookies = response.OrderStatus, response.UpdatedCookies
	}
	return result, requestErr
}

// SubmitShipmentEvidence 依次上传凭证并提交一次官方无需寄件；图片字节只存在当前请求内存。
func (r *OrderRuntime) SubmitShipmentEvidence(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID, tradeText string, images []orderapp.ShipmentEvidenceImage) (*orderapp.ShipmentEvidencePlatformResult, error) {
	if detail == nil {
		return nil, errors.New("凭证发货缺少账号凭证")
	}
	// submitter、available 是当前平台客户端的凭证上传和无需寄件能力。
	submitter, available := r.mtopClient().(shipmentEvidenceMTop)
	if !available {
		return nil, orderapp.ErrShipmentUnavailable
	}
	// requestCtx、session 保存整个上传和发货流程共用的 Cookie 会话。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// imageURLs 按用户确认顺序保存官方 object.url，最终作为 JSON 字符串数组提交。
	imageURLs := make([]string, 0, len(images))
	// updatedCookies 保存没有完整 Cookie Jar 时最后一次平台响应的平面 Cookie。
	updatedCookies := ""
	// image 是当前待上传的内存凭证。
	for _, image := range images {
		// upload、uploadErr 是当前图片的官方上传结果和错误。
		upload, uploadErr := submitter.UploadShipmentEvidenceImage(requestCtx, detail.Value, mtop.ShipmentEvidenceImage{
			Filename: image.Filename, ContentType: image.ContentType, Data: image.Data,
		})
		if uploadErr != nil {
			return &orderapp.ShipmentEvidencePlatformResult{UpdatedCookies: updatedCookies, CookieUpdate: orderCookieUpdate(detail, session)}, uploadErr
		}
		if upload == nil || strings.TrimSpace(upload.URL) == "" {
			return &orderapp.ShipmentEvidencePlatformResult{UpdatedCookies: updatedCookies, CookieUpdate: orderCookieUpdate(detail, session)}, errors.New("发货凭证上传响应缺少图片地址")
		}
		imageURLs = append(imageURLs, strings.TrimSpace(upload.URL))
		if upload.UpdatedCookies != "" {
			updatedCookies = upload.UpdatedCookies
		}
	}
	// response、submitErr 是已经发出最终无需寄件请求后的平台结果。
	response, submitErr := submitter.ConsignEvidenceContext(requestCtx, detail.Value, mtop.ConsignEvidenceRequest{
		OrderID: orderID, TradeText: tradeText, ImageURLs: imageURLs,
	})
	// result 标记最终发货请求已经执行；网络错误必须进入人工核对而不能自动重放。
	result := &orderapp.ShipmentEvidencePlatformResult{ShipmentAttempted: true, UpdatedCookies: updatedCookies, CookieUpdate: orderCookieUpdate(detail, session), TradeText: tradeText, ImageURLs: append([]string(nil), imageURLs...)}
	if response != nil {
		result.Success = response.Success
		if response.UpdatedCookies != "" {
			result.UpdatedCookies = response.UpdatedCookies
		}
		if len(response.Ret) > 0 {
			result.Message = strings.Join(response.Ret, "; ")
		}
	}
	return result, submitErr
}

// RequestRedFlower 使用给定凭证快照请求官方求花，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) RequestRedFlower(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID, channel string) (*orderapp.RedFlowerPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("求花请求缺少账号凭证")
	}
	// requester、available 是当前平台客户端的求花能力及可用状态。
	requester, available := r.mtopClient().(redFlowerMTop)
	if !available {
		return nil, orderapp.ErrRedFlowerUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台求花响应和请求错误。
	response, requestErr := requester.RequestRedFlower(requestCtx, detail.Value, orderID, channel)
	// result 始终携带本次请求观察到的 Cookie 会话变化，即使平台返回错误也允许安全协调。
	result := &orderapp.RedFlowerPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Success = response.Success
		result.Message = response.Message
		result.UpdatedCookies = response.UpdatedCookies
	}
	return result, requestErr
}

// RenderOrderAdjustPrice 使用给定凭证读取官方动态表单，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) RenderOrderAdjustPrice(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID string) (*orderapp.PriceAdjustmentPlatformForm, error) {
	if detail == nil {
		return nil, errors.New("改价表单请求缺少账号凭证")
	}
	// renderer、available 是当前平台客户端的改价能力及可用状态。
	renderer, available := r.mtopClient().(adjustPriceMTop)
	if !available {
		return nil, orderapp.ErrPriceAdjustmentUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台 render 响应和请求错误。
	response, requestErr := renderer.RenderOrderAdjustPrice(requestCtx, detail.Value, orderID)
	// result 始终携带请求观察到的 Cookie 会话变化。
	result := &orderapp.PriceAdjustmentPlatformForm{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Title, result.UpdatedCookies = response.Title, response.UpdatedCookies
		result.Fields = make([]orderapp.PriceAdjustmentField, 0, len(response.Fields))
		// field 是当前待投影的平台动态字段。
		for _, field := range response.Fields {
			result.Fields = append(result.Fields, orderapp.PriceAdjustmentField{Key: field.Key, Name: field.Name,
				PrefixText: field.PrefixText, Value: field.Price, ReadOnly: field.ReadOnly})
		}
	}
	return result, requestErr
}

// SubmitOrderAdjustPrice 使用给定凭证提交真实改价，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) SubmitOrderAdjustPrice(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID string, fieldCents map[string]string) (*orderapp.PriceAdjustmentPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("改价提交缺少账号凭证")
	}
	// submitter、available 是当前平台客户端的改价能力及可用状态。
	submitter, available := r.mtopClient().(adjustPriceMTop)
	if !available {
		return nil, orderapp.ErrPriceAdjustmentUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台 submit 响应和请求错误。
	response, requestErr := submitter.SubmitOrderAdjustPrice(requestCtx, detail.Value, orderID, fieldCents)
	// result 始终携带请求观察到的 Cookie 会话变化。
	result := &orderapp.PriceAdjustmentPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Success, result.Message, result.UpdatedCookies = response.Success, response.Message, response.UpdatedCookies
	}
	return result, requestErr
}

// FetchCloseOrderReasons 使用给定凭证读取官方动态关闭原因，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) FetchCloseOrderReasons(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID string) (*orderapp.CloseOrderPlatformReasons, error) {
	if detail == nil {
		return nil, errors.New("关闭原因请求缺少账号凭证")
	}
	// requester、available 是当前平台客户端的关单能力及可用状态。
	requester, available := r.mtopClient().(closeOrderMTop)
	if !available {
		return nil, orderapp.ErrCloseOrderUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台动态原因响应和错误。
	response, requestErr := requester.FetchCloseOrderReasons(requestCtx, detail.Value, orderID)
	// result 始终携带请求观察到的 Cookie 会话变化。
	result := &orderapp.CloseOrderPlatformReasons{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Reasons, result.UpdatedCookies = append([]string(nil), response.Reasons...), response.UpdatedCookies
	}
	return result, requestErr
}

// CloseOrderBySeller 使用给定凭证执行官方卖家关单，并收集响应 Cookie 会话变化。
func (r *OrderRuntime) CloseOrderBySeller(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID, reason string) (*orderapp.CloseOrderPlatformResult, error) {
	if detail == nil {
		return nil, errors.New("关闭订单请求缺少账号凭证")
	}
	// requester、available 是当前平台客户端的关单能力及可用状态。
	requester, available := r.mtopClient().(closeOrderMTop)
	if !available {
		return nil, orderapp.ErrCloseOrderUnavailable
	}
	// requestCtx、session 保存带完整 Cookie Jar 或平面 Cookie 的请求上下文。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// response、requestErr 是平台真实卖家关单响应和错误。
	response, requestErr := requester.CloseOrderBySeller(requestCtx, detail.Value, orderID, reason)
	// result 始终携带请求观察到的 Cookie 会话变化。
	result := &orderapp.CloseOrderPlatformResult{CookieUpdate: orderCookieUpdate(detail, session)}
	if response != nil {
		result.Success, result.Message, result.UpdatedCookies = response.Success, response.Message, response.UpdatedCookies
	}
	return result, requestErr
}

// consignWithCurrentCookie 使用账号平台凭证确认发货并保存响应 Cookie。
func (r *OrderRuntime) consignWithCurrentCookie(ctx context.Context, cookieID, orderID string, userID int64) (bool, []string, string, bool, error) {
	if r == nil || r.store == nil || r.store.Cookies == nil {
		return false, nil, "", false, errors.New("订单凭证存储未初始化")
	}
	// unlock 保护当前账号凭证读取和写回，保持现有订单流程的串行语义。
	unlock := r.store.LockAccountCredentials(cookieID)
	defer unlock()
	// detail、loadErr 保存按账号读取的平台运行凭证及错误。
	detail, loadErr := r.store.Cookies.GetCookiePlatformRuntimeData(ctx, cookieID)
	if loadErr != nil {
		return false, nil, "", false, loadErr
	}
	if detail.UserID != userID {
		return false, nil, "", false, orderapp.ErrForbidden
	}
	if !hasStoredOrderCredential(detail) {
		return false, nil, "", false, errors.New("账号 Cookie 为空")
	}
	// requestCtx、session 保存带 Cookie 快照的平台上下文及响应会话。
	requestCtx, session := withOrderCookieSnapshot(ctx, detail)
	// client 保存当前平台调用客户端。
	client := r.mtopClient()
	if client == nil {
		return false, nil, "", false, errors.New("MTop 客户端未初始化")
	}
	// success、messages、updatedCookies、callErr 保存平台确认发货响应。
	success, messages, updatedCookies, callErr := client.ConsignContext(requestCtx, detail.Value, orderID)
	// value、valueChanged、handled、persistErr 保存响应 Cookie 会话写回结果。
	value, valueChanged, handled, persistErr := r.persistOrderCookieSession(ctx, detail, session, updatedCookies)
	if persistErr != nil {
		// wrappedPersistErr 保存包含订单语义的 Cookie 写回错误。
		wrappedPersistErr := fmt.Errorf("保存发货响应 Cookie Jar: %w", persistErr)
		if callErr != nil {
			return success, messages, "", false, errors.Join(callErr, wrappedPersistErr)
		}
		return success, messages, "", false, wrappedPersistErr
	}
	if handled {
		// runtimeCookie 保存完整 Cookie 会话变化后的平面 Cookie 值。
		runtimeCookie := ""
		if valueChanged {
			runtimeCookie = value
		}
		return success, messages, runtimeCookie, valueChanged, callErr
	}
	if callErr != nil {
		return false, messages, "", false, callErr
	}
	if updatedCookies == "" || updatedCookies == detail.Value {
		return success, messages, "", false, nil
	}
	// err 保存旧式平面 Cookie 写回错误。
	if err := r.store.Cookies.UpdateValueOwned(ctx, cookieID, updatedCookies, userID); err != nil {
		return success, messages, "", false, fmt.Errorf("保存发货响应 Cookie: %w", err)
	}
	return success, messages, updatedCookies, true, nil
}

// UpdateRunningCookie 同步运行时账号 Cookie。
func (r *OrderRuntime) UpdateRunningCookie(ctx context.Context, cookieID, value string) {
	if r != nil && r.hooks.UpdateRunningCookie != nil {
		r.hooks.UpdateRunningCookie(ctx, cookieID, value)
	}
}

// NotifyDelivery 发送发货结果通知。
func (r *OrderRuntime) NotifyDelivery(cookieID, buyerID, itemID, chatID, message string) {
	if r != nil && r.hooks.NotifyDelivery != nil {
		r.hooks.NotifyDelivery(cookieID, buyerID, itemID, chatID, message)
	}
}

// RecordOrderReconciliation 创建外部动作成功后的补偿记录。
func (r *OrderRuntime) RecordOrderReconciliation(ctx context.Context, orderID, cookieID, kind, message string) (string, error) {
	if r == nil || r.reconciliation == nil {
		return "", errors.New("订单补偿存储未初始化")
	}
	return r.reconciliation.RecordReconciliation(ctx, orderID, cookieID, kind, message)
}

// RecordReconciliation 将补偿记录能力暴露给手动发货应用 Port。
func (r *OrderRuntime) RecordReconciliation(ctx context.Context, orderID, cookieID, kind, message string) (string, error) {
	return r.RecordOrderReconciliation(ctx, orderID, cookieID, kind, message)
}

// ReportPersistenceFailure 记录手动发货本地订单状态持久化失败。
func (r *OrderRuntime) ReportPersistenceFailure(orderID string, err error) {
	if r == nil || err == nil {
		return
	}
	r.logger.Error("更新订单为系统已发货失败", "order_id", orderID, "err", err)
	if r.hooks.ReportPersistenceFailure != nil {
		r.hooks.ReportPersistenceFailure(orderID, err)
	}
}

// RecoverExpiredSession 处理订单刷新应用服务报告的平台会话过期。
func (r *OrderRuntime) RecoverExpiredSession(ctx context.Context, cookieID string, err error) bool {
	return r != nil && r.hooks.RecoverExpiredSession != nil && r.hooks.RecoverExpiredSession(ctx, cookieID, err)
}

// DetailAvailable 判断平台详情接口是否可用。
func (r *OrderRuntime) DetailAvailable() bool {
	// client 保存当前平台客户端。
	client := r.mtopClient()
	// _, available 丢弃客户端具体类型并记录详情接口能力。
	_, available := client.(orderDetailMTop)
	return available
}

// SoldAvailable 判断平台已售订单列表接口是否可用。
func (r *OrderRuntime) SoldAvailable() bool {
	// client 保存当前平台客户端。
	client := r.mtopClient()
	// _, available 丢弃客户端具体类型并记录订单列表接口能力。
	_, available := client.(mtop.SoldOrderFetcher)
	return available
}

// CredentialAvailable 判断平台请求视图是否包含可用 Cookie。
func (r *OrderRuntime) CredentialAvailable(detail *orderapp.PlatformRuntimeData) bool {
	return detail != nil && strings.TrimSpace(detail.Value) != ""
}

// FetchOrderDetail 调用平台详情接口并收集 Cookie 会话变化。
func (r *OrderRuntime) FetchOrderDetail(ctx context.Context, detail *orderapp.PlatformRuntimeData, orderID string) (orderapp.RefreshDetailFetchResult, error) {
	// fetcher、available 保存详情接口实现及其可用状态。
	fetcher, available := r.mtopClient().(orderDetailMTop)
	if !available {
		return orderapp.RefreshDetailFetchResult{}, orderapp.ErrRefreshDetailUnsupported
	}
	if detail == nil {
		return orderapp.RefreshDetailFetchResult{}, errors.New("订单详情请求缺少账号凭证")
	}
	// requestCtx、session 保存带 Cookie 快照的平台上下文及响应会话。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// result、callErr 保存平台详情响应和错误。
	result, callErr := fetcher.FetchOrderDetail(requestCtx, detail.Value, orderID)
	// cookieUpdate 保存平台详情请求观察到的 Cookie 会话变化。
	cookieUpdate := orderCookieUpdate(detail, session)
	if callErr != nil {
		return orderapp.RefreshDetailFetchResult{CookieUpdate: cookieUpdate}, callErr
	}
	if result == nil {
		return orderapp.RefreshDetailFetchResult{CookieUpdate: cookieUpdate}, errors.New("订单详情接口未返回结果")
	}
	return orderapp.RefreshDetailFetchResult{Detail: &orderapp.RefreshDetail{Quantity: result.Quantity, SpecName: result.SpecName, SpecValue: result.SpecValue, OrderStatus: result.OrderStatus, Amount: result.Amount, UpdatedCookies: result.UpdatedCookies}, CookieUpdate: cookieUpdate}, nil
}

// FetchSoldOrders 调用平台已售订单接口并收集 Cookie 会话变化。
func (r *OrderRuntime) FetchSoldOrders(ctx context.Context, detail *orderapp.PlatformRuntimeData) (orderapp.RefreshSoldFetchResult, error) {
	return r.fetchSoldOrders(ctx, detail, orderapp.RefreshSoldFetchOptions{Mode: orderapp.RefreshModeFull}, nil)
}

// FetchSoldOrdersWithProgress 获取平台已售订单，并在每页完成后发布累计订单数量。
func (r *OrderRuntime) FetchSoldOrdersWithProgress(ctx context.Context, detail *orderapp.PlatformRuntimeData, reporter orderapp.RefreshSoldPageReporter) (orderapp.RefreshSoldFetchResult, error) {
	return r.fetchSoldOrders(ctx, detail, orderapp.RefreshSoldFetchOptions{Mode: orderapp.RefreshModeFull}, reporter)
}

// FetchSoldOrdersWithOptions 按完整快照或账号高水位执行平台分页读取。
func (r *OrderRuntime) FetchSoldOrdersWithOptions(ctx context.Context, detail *orderapp.PlatformRuntimeData, options orderapp.RefreshSoldFetchOptions, reporter orderapp.RefreshSoldPageReporter) (orderapp.RefreshSoldFetchResult, error) {
	return r.fetchSoldOrders(ctx, detail, options, reporter)
}

// fetchSoldOrders 是一次性兼容调用和逐页进度调用共享的平台分页实现。
func (r *OrderRuntime) fetchSoldOrders(ctx context.Context, detail *orderapp.PlatformRuntimeData, options orderapp.RefreshSoldFetchOptions, reporter orderapp.RefreshSoldPageReporter) (orderapp.RefreshSoldFetchResult, error) {
	// fetcher、available 保存订单列表接口实现及其可用状态。
	fetcher, available := r.mtopClient().(mtop.SoldOrderFetcher)
	if !available {
		return orderapp.RefreshSoldFetchResult{}, errors.New("当前 MTop 客户端不支持订单列表发现")
	}
	if detail == nil {
		return orderapp.RefreshSoldFetchResult{}, errors.New("订单列表请求缺少账号凭证")
	}
	// requestCtx、session 保存带 Cookie 快照的平台上下文及响应会话。
	requestCtx, session := withOrderCookieSnapshot(ctx, platformRuntimeDataForOrder(detail))
	// orders 保存跨分页累积的平台订单。
	orders := make([]orderapp.RefreshSoldOrder, 0)
	// mode 是本次实际执行的同步范围；缺少可信游标时强制读取完整基线。
	mode := options.Mode
	if mode != orderapp.RefreshModeIncremental || options.Cursor == nil || strings.TrimSpace(options.Cursor.HighWaterCreatedAt) == "" {
		mode = orderapp.RefreshModeFull
	}
	// minimumPages 是增量停止前必须完成的安全回看页数。
	minimumPages := options.MinimumPages
	if minimumPages < 1 {
		minimumPages = 3
	}
	// boundaryRequired 是增量停止前必须连续命中的历史订单数量。
	boundaryRequired := options.BoundaryRequired
	if boundaryRequired < 1 {
		boundaryRequired = 20
	}
	// boundaryMatched 保存当前连续命中的历史边界订单数量；遇到新订单或无时间订单时清零。
	boundaryMatched := 0
	// completeSnapshot 表示分页自然结束而非高水位提前停止或触及保护上限。
	completeSnapshot := false
	// boundaryReached 表示增量扫描在安全回看后命中可信停止边界。
	boundaryReached := false
	// pageNumber 是当前请求的订单列表页码。
	for pageNumber := 1; pageNumber <= orderRuntimeMaxSoldOrderPages; pageNumber++ {
		// page、callErr 保存当前订单列表页及错误。
		page, callErr := fetcher.FetchSoldOrdersPage(requestCtx, detail.Value, pageNumber, 30)
		if callErr != nil {
			return orderapp.RefreshSoldFetchResult{Orders: orders, CookieUpdate: orderCookieUpdate(detail, session)}, callErr
		}
		if page == nil {
			return orderapp.RefreshSoldFetchResult{Orders: orders, CookieUpdate: orderCookieUpdate(detail, session)}, errors.New("订单列表接口未返回结果")
		}
		// remote 是当前平台订单列表项。
		for _, remote := range page.Items {
			// converted 保存当前平台订单的应用层只读字段。
			converted := orderapp.RefreshSoldOrder{OrderID: remote.OrderID, ItemID: remote.ItemID, BuyerID: remote.BuyerID, OrderStatus: orderapp.NormalizeOrderStatus(remote.OrderStatus), Quantity: remote.Quantity, Amount: remote.Amount, ReceiverName: remote.ReceiverName, ReceiverPhone: remote.ReceiverPhone, ReceiverAddr: remote.ReceiverAddr, ReceiverCity: remote.ReceiverCity, IsBargain: remote.IsBargain, CreatedAt: remote.CreatedAt}
			orders = append(orders, converted)
			if mode == orderapp.RefreshModeIncremental {
				if orderAtOrBeforeSyncCursor(converted, options.Cursor) {
					boundaryMatched++
				} else {
					boundaryMatched = 0
				}
			}
		}
		if reporter != nil {
			// total 是平台当前账号订单总数；异常为零时至少使用已读取数量，保证分母不小于分子。
			total := page.TotalCount
			if total < len(orders) {
				total = len(orders)
			}
			// totalPages 是按固定每页 30 条和平台总数计算的页数。
			totalPages := 0
			if total > 0 {
				totalPages = (total + 29) / 30
			}
			reporter(orderapp.RefreshSoldPageProgress{Processed: len(orders), Total: total, CurrentPage: pageNumber, TotalPages: totalPages, Mode: mode, BoundaryMatched: boundaryMatched, BoundaryRequired: boundaryRequired})
		}
		if !page.NextPage || len(page.Items) == 0 {
			completeSnapshot = true
			break
		}
		if mode == orderapp.RefreshModeIncremental && pageNumber >= minimumPages && boundaryMatched >= boundaryRequired {
			boundaryReached = true
			break
		}
	}
	return orderapp.RefreshSoldFetchResult{Orders: orders, CookieUpdate: orderCookieUpdate(detail, session), CompleteSnapshot: completeSnapshot, BoundaryReached: boundaryReached}, nil
}

// orderAtOrBeforeSyncCursor 判断平台订单是否已经进入账号成功同步过的历史区间。
func orderAtOrBeforeSyncCursor(order orderapp.RefreshSoldOrder, cursor *orderapp.OrderSyncCursor) bool {
	if cursor == nil || strings.TrimSpace(order.CreatedAt) == "" || strings.TrimSpace(cursor.HighWaterCreatedAt) == "" {
		return false
	}
	// orderTime、orderErr 保存当前订单平台时间的解析结果。
	orderTime, orderErr := time.Parse(time.RFC3339, order.CreatedAt)
	// cursorTime、cursorErr 保存高水位平台时间的解析结果。
	cursorTime, cursorErr := time.Parse(time.RFC3339, cursor.HighWaterCreatedAt)
	if orderErr != nil || cursorErr != nil {
		return false
	}
	if orderTime.Before(cursorTime) {
		return true
	}
	if orderTime.After(cursorTime) {
		return false
	}
	return strings.Compare(order.OrderID, cursor.HighWaterOrderID) <= 0
}

// PersistCookieSession 在凭证锁内保存应用层 Cookie 更新。
func (r *OrderRuntime) PersistCookieSession(ctx context.Context, detail *orderapp.PlatformRuntimeData, update orderapp.RefreshCookieUpdate) (string, bool, bool, error) {
	if detail == nil || !update.Handled {
		return "", false, false, nil
	}
	if !update.Changed {
		return detail.Value, false, true, nil
	}
	if r == nil || r.store == nil || r.store.Cookies == nil {
		return update.Value, update.Value != detail.Value, true, errors.New("账号 Cookie 持久化服务未初始化")
	}
	// persistErr 保存账号续期 Cookie 写入错误。
	persistErr := r.store.Cookies.UpdateRenewalCookie(ctx, detail.ID, update.Value, update.MetadataJSON, time.Now().Unix())
	if persistErr != nil {
		return update.Value, update.Value != detail.Value, true, persistErr
	}
	return update.Value, update.Value != detail.Value, true, nil
}

// IsSessionExpired 判断平台错误是否为会话过期。
func (r *OrderRuntime) IsSessionExpired(err error) bool {
	return mtop.IsSessionExpiredErr(err)
}

// withOrderCookieSnapshot 为订单平台请求挂载平面 Cookie 或完整 Cookie Jar。
func withOrderCookieSnapshot(ctx context.Context, detail db.CookiePlatformRuntimeData) (context.Context, *mtop.CookieSession) {
	// snapshot、complete 保存账号 metadata 中的 Cookie Jar 及其完整性。
	snapshot, complete := cookierefresh.SnapshotFromMetadataOK(detail.MetadataJSON)
	if !complete {
		return mtop.WithFlatCookieSession(ctx, detail.Value)
	}
	return mtop.WithCookieSnapshot(ctx, snapshot)
}

// hasStoredOrderCredential 判断订单平台视图是否包含可用凭证。
func hasStoredOrderCredential(detail db.CookiePlatformRuntimeData) bool {
	if strings.TrimSpace(detail.Value) != "" {
		return true
	}
	// _, complete 丢弃 Cookie 快照内容，仅判断是否存在完整凭证。
	_, complete := cookierefresh.SnapshotFromMetadataOK(detail.MetadataJSON)
	return complete
}

// platformRuntimeDataForOrder 将应用平台运行视图转换为数据库适配内部模型。
func platformRuntimeDataForOrder(data *orderapp.PlatformRuntimeData) db.CookiePlatformRuntimeData {
	if data == nil {
		return db.CookiePlatformRuntimeData{}
	}
	return db.CookiePlatformRuntimeData{ID: data.ID, UserID: data.UserID, Value: data.Value, MetadataJSON: data.MetadataJSON, ShowBrowser: data.ShowBrowser}
}

// orderCookieUpdate 将 MTOP 会话状态转换为订单应用层 Cookie 更新模型。
func orderCookieUpdate(detail *orderapp.PlatformRuntimeData, session *mtop.CookieSession) orderapp.RefreshCookieUpdate {
	if detail == nil || session == nil {
		return orderapp.RefreshCookieUpdate{}
	}
	// value、snapshot、changed 保存会话当前 Cookie、快照及变化状态。
	value, snapshot, changed := session.State()
	if snapshot == nil {
		return orderapp.RefreshCookieUpdate{Value: value, Changed: changed, Handled: false}
	}
	// metadata 保存包含最新 Cookie 快照的账号元数据。
	metadata := cookierefresh.MetadataWithSnapshot(detail.MetadataJSON, snapshot)
	return orderapp.RefreshCookieUpdate{Value: value, MetadataJSON: metadata, Changed: changed, Handled: true}
}

// persistOrderCookieSession 保存确认发货响应中的完整或平面 Cookie 会话。
func (r *OrderRuntime) persistOrderCookieSession(ctx context.Context, detail db.CookiePlatformRuntimeData, session *mtop.CookieSession, updatedCookies string) (string, bool, bool, error) {
	// value、snapshot、changed 保存平台 CookieSession 状态。
	value, snapshot, changed := session.State()
	if !changed {
		if snapshot != nil {
			return detail.Value, false, true, nil
		}
		return "", false, false, nil
	}
	// metadata 保存移除或更新 Cookie 快照后的账号元数据。
	metadata := cookierefresh.MetadataWithoutSnapshot(detail.MetadataJSON)
	if snapshot != nil {
		metadata = cookierefresh.MetadataWithSnapshot(detail.MetadataJSON, snapshot)
	}
	if r.store == nil || r.store.Cookies == nil {
		return value, value != detail.Value, true, errors.New("账号 Cookie 持久化 repository 未初始化")
	}
	// persistErr 保存完整 Cookie 会话写回错误。
	persistErr := r.store.Cookies.UpdateRenewalCookie(ctx, detail.ID, value, metadata, time.Now().Unix())
	if persistErr != nil {
		return value, value != detail.Value, true, persistErr
	}
	return value, value != detail.Value, true, nil
}

// orderRuntimeMaxSoldOrderPages 限制一次订单发现最多请求的平台页数。
const orderRuntimeMaxSoldOrderPages = 100

// 确保订单运行时覆盖手动发货和订单刷新应用层端口。
var _ orderapp.ManualShipRuntime = (*OrderRuntime)(nil)
var _ orderapp.RedFlowerRequestRuntime = (*OrderRuntime)(nil)
var _ orderapp.RefreshRuntime = (*OrderRuntime)(nil)
