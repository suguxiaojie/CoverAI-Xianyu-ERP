package orders

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// ShipmentEvidenceTaskType 是 account_task_runs 中带凭证无需寄件动作的稳定任务类型。
	ShipmentEvidenceTaskType = "order_ship_with_evidence"
	// shipmentEvidenceTimeout 限制订单复核、图片上传和最终发货的单次平台流程。
	shipmentEvidenceTimeout = 90 * time.Second
	// shipmentStatusTimeout 限制最终写操作前的只读订单状态复核，超时不得继续发货。
	shipmentStatusTimeout = 12 * time.Second
	// shipmentEvidenceMaxImages 与当前官方工作台的三张凭证上限一致。
	shipmentEvidenceMaxImages = 3
	// shipmentEvidenceMaxImageBytes 与官方前端“小于 3MB”门禁一致。
	shipmentEvidenceMaxImageBytes = 3 << 20
)

var (
	// ErrShipmentNotEligible 表示本地卡片、订单归属或最新平台状态不再允许发货。
	ErrShipmentNotEligible = errors.New("订单不是当前账号可发货的待发货订单")
	// ErrShipmentUnavailable 表示当前平台运行时未实现官方凭证上传和无需寄件接口。
	ErrShipmentUnavailable = errors.New("当前平台客户端不支持凭证发货")
	// ErrShipmentAlreadyHandled 表示同一账号订单已经发货、处理中或等待人工核对。
	ErrShipmentAlreadyHandled = errors.New("订单发货已处理或正在核对，请勿重复提交")
	// ErrShipmentNeedsReview 表示最终发货请求结果不明确，必须停止自动重放。
	ErrShipmentNeedsReview = errors.New("发货结果不明确，已停止重复提交，请人工核对闲鱼订单")
	// ErrShipmentStatusTimeout 表示提交前只读状态复核超时，最终发货尚未执行。
	ErrShipmentStatusTimeout = errors.New("订单状态读取超时，尚未执行发货，请稍后重试")
)

// shipmentOrderPattern 只接受闲鱼当前 10～30 位数字订单号。
var shipmentOrderPattern = regexp.MustCompile(`^[0-9]{10,30}$`)

// ShipmentContext 是从卖家付款卡片读取的非敏感订单关联。
type ShipmentContext struct {
	// AccountID 是收到 role=seller 付款卡片的卖家账号。
	AccountID string
	// ChatID 是付款卡片所属卖家侧会话。
	ChatID string
	// OrderID 是平台付款订单号。
	OrderID string
	// ItemID 是卡片关联商品标识。
	ItemID string
	// BuyerID 是付款卡片发送者，也是订单买家标识。
	BuyerID string
}

// ShipmentEvidenceImage 是用户最终确认后才会上传的一张内存凭证。
type ShipmentEvidenceImage struct {
	// Filename 是清理目录后的展示文件名，不进入日志。
	Filename string
	// ContentType 是 PNG 或 JPEG MIME。
	ContentType string
	// Data 是当前请求内存中的图片字节，不持久化。
	Data []byte
}

// ShipmentEvidenceRequest 描述聊天卡片一次真实无需寄件提交。
type ShipmentEvidenceRequest struct {
	// UserID 是当前认证用户标识。
	UserID int64
	// AccountID 是付款卡片所属卖家账号。
	AccountID string
	// OrderID 是付款卡片明确订单号。
	OrderID string
	// TradeText 是最多 200 字的相关描述。
	TradeText string
	// Images 是最多三张、单张小于 3MB 的 PNG/JPEG 凭证。
	Images []ShipmentEvidenceImage
}

// ShipmentEvidenceResult 是 HTTP 层可安全公开的确定性发货结果。
type ShipmentEvidenceResult struct {
	// Success 只在平台明确返回成功订单号时为真。
	Success bool
	// Status 是 succeeded、succeeded_with_warning、failed 或 needs_review。
	Status string
	// Message 是不含凭证、收货信息和 Cookie 的用户提示。
	Message string
	// OrderID 是本次操作对应的订单号。
	OrderID string
}

// ShipmentStatusPlatformResult 保存提交前平台订单状态和 Cookie 会话变化。
type ShipmentStatusPlatformResult struct {
	// OrderStatus 是平台详情返回的原始订单状态。
	OrderStatus string
	// UpdatedCookies 是仅有平面 Cookie 时的兼容响应。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
}

// ShipmentEvidencePlatformResult 保存上传及最终无需寄件请求的结果。
type ShipmentEvidencePlatformResult struct {
	// Success 表示平台明确返回发货订单号。
	Success bool
	// ShipmentAttempted 表示图片上传完成后已经发出最终发货请求，用于区分可重试上传失败和不明确提交结果。
	ShipmentAttempted bool
	// Message 是平台非敏感提示。
	Message string
	// UpdatedCookies 是仅有平面 Cookie 时的兼容响应。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 观察到的会话变化。
	CookieUpdate RefreshCookieUpdate
	// TradeText 是最终提交给闲鱼的发货描述。
	TradeText string
	// ImageURLs 是最终提交给闲鱼的官方 HTTPS 图片地址。
	ImageURLs []string
}

// ShipmentProof 是应用层保存和读取的 ERP 发货凭证。
type ShipmentProof struct {
	// OrderID 是平台订单标识。
	OrderID string
	// AccountID 是执行 ERP 发货的卖家账号。
	AccountID string
	// TradeText 是最终提交给闲鱼的发货描述。
	TradeText string
	// ImageURLs 是最终提交给闲鱼的官方 HTTPS 图片地址。
	ImageURLs []string
	// Source 当前固定为 erp。
	Source string
	// SubmittedAt 是平台明确成功时的 Unix 秒。
	SubmittedAt int64
}

// ShipmentProofResult 是 HTTP 层可公开的只读 ERP 发货凭证。
type ShipmentProofResult struct {
	// Proof 是当前订单保存的 ERP 发货凭证。
	Proof ShipmentProof
}

// ShipmentEvidenceRepository 定义卖家付款卡片、影子接管、幂等和本地发货收口所需能力。
type ShipmentEvidenceRepository interface {
	// GetPendingShipment 读取仍未出现后续终态的卖家付款卡片。
	GetPendingShipment(ctx context.Context, userID int64, accountID, orderID string) (*ShipmentContext, error)
	// PromoteBuyerShadowOrder 在严格条件下把同用户买家影子订单交给真实卖家账号。
	PromoteBuyerShadowOrder(ctx context.Context, context ShipmentContext) (bool, error)
	// GetOrder 读取接管后的订单实体。
	GetOrder(ctx context.Context, orderID string) (*Order, error)
	// UpsertOrder 创建缺失卖家订单或写入明确成功状态。
	UpsertOrder(ctx context.Context, orderID string, options UpsertOptions) error
	// MarkOrderShippedAt 写入平台明确发货成功时间。
	MarkOrderShippedAt(ctx context.Context, orderID string) error
	// ClaimShipmentEvidence 原子抢占当前账号订单的首次或明确失败重试。
	ClaimShipmentEvidence(ctx context.Context, runKey, accountID, orderID string, now int64) (bool, error)
	// FinishShipmentEvidence 保存发货运行终态。
	FinishShipmentEvidence(ctx context.Context, runKey, status string, success, failed int, message string) error
	// SaveShipmentProof 保存平台明确成功后的 ERP 发货凭证。
	SaveShipmentProof(ctx context.Context, proof ShipmentProof) error
	// GetShipmentProofForUser 按用户和订单归属读取 ERP 发货凭证。
	GetShipmentProofForUser(ctx context.Context, userID int64, orderID string) (*ShipmentProof, error)
	// LockCredentials 只保护短时凭证读取和写回，不得跨网络请求持有。
	LockCredentials(accountID string) func()
	// LoadCookiePlatformDetail 读取平台请求所需最小凭证视图。
	LoadCookiePlatformDetail(ctx context.Context, accountID string) (*PlatformRuntimeData, error)
	// UpdateRenewalCookie 保存平面 Cookie 兼容响应。
	UpdateRenewalCookie(ctx context.Context, accountID, value, metadata string, at int64) error
}

// ShipmentEvidenceRuntime 定义订单状态复核、凭证上传、无需寄件和成功后补偿能力。
type ShipmentEvidenceRuntime interface {
	// ShipmentEvidenceAvailable 判断平台客户端是否同时支持上传和无需寄件。
	ShipmentEvidenceAvailable() bool
	// CredentialAvailable 判断平台凭证是否足以发起 MTOP 请求。
	CredentialAvailable(detail *PlatformRuntimeData) bool
	// FetchShipmentStatus 在最终提交前读取最新平台订单状态。
	FetchShipmentStatus(ctx context.Context, detail *PlatformRuntimeData, orderID string) (*ShipmentStatusPlatformResult, error)
	// SubmitShipmentEvidence 上传图片并调用官方无需寄件接口。
	SubmitShipmentEvidence(ctx context.Context, detail *PlatformRuntimeData, orderID, tradeText string, images []ShipmentEvidenceImage) (*ShipmentEvidencePlatformResult, error)
	// PersistCookieSession 保存完整 Cookie Jar 请求产生的会话变化。
	PersistCookieSession(ctx context.Context, detail *PlatformRuntimeData, update RefreshCookieUpdate) (string, bool, bool, error)
	// UpdateRunningCookie 同步已经持久化的新 Cookie 到在线账号。
	UpdateRunningCookie(ctx context.Context, accountID, value string)
	// RecoverExpiredSession 处理平台明确会话失效。
	RecoverExpiredSession(ctx context.Context, accountID string, err error) bool
	// IsSessionExpired 判断错误是否是明确会话失效。
	IsSessionExpired(err error) bool
	// ScheduleRedFlowerAfterShipment 创建发货成功后的秒级求花任务。
	ScheduleRedFlowerAfterShipment(ctx context.Context, orderID string) error
	// NotifyDelivery 发送不含敏感数据的发货结果通知。
	NotifyDelivery(cookieID, buyerID, itemID, chatID, message string)
	// RecordReconciliation 保存外部成功、本地状态失败的补偿记录。
	RecordReconciliation(ctx context.Context, orderID, cookieID, kind, message string) (string, error)
	// ReportPersistenceFailure 记录本地收口错误。
	ReportPersistenceFailure(orderID string, err error)
}

// ShipmentEvidenceService 编排卖家卡片归属、状态复核、幂等、凭证上传和发货收口。
type ShipmentEvidenceService struct {
	// repository 保存卡片、订单、凭证和运行记录 Port。
	repository ShipmentEvidenceRepository
	// runtime 保存平台调用、Cookie 会话和补偿 Port。
	runtime ShipmentEvidenceRuntime
	// now 返回运行记录时间，测试可注入固定值。
	now func() time.Time
}

// NewShipmentEvidenceService 构造无需寄件服务；repository 和 runtime 是生产必需依赖。
func NewShipmentEvidenceService(repository ShipmentEvidenceRepository, runtime ShipmentEvidenceRuntime, now func() time.Time) *ShipmentEvidenceService {
	if now == nil {
		now = time.Now
	}
	return &ShipmentEvidenceService{repository: repository, runtime: runtime, now: now}
}

// Ship 在最终确认后执行一次带可选描述和凭证的无需寄件；不明确结果进入人工核对且禁止自动重放。
func (service *ShipmentEvidenceService) Ship(ctx context.Context, request ShipmentEvidenceRequest) (ShipmentEvidenceResult, error) {
	// shipmentContext、validationErr 是卡片上下文和输入／归属验证结果。
	shipmentContext, validationErr := service.validateRequest(ctx, request)
	if validationErr != nil {
		return ShipmentEvidenceResult{}, validationErr
	}
	// order、orderErr 是安全接管或创建后的卖家订单。
	order, orderErr := service.ensureSellerOrder(ctx, *shipmentContext)
	if orderErr != nil {
		return ShipmentEvidenceResult{}, orderErr
	}
	// initialDetail、credentialErr 是提交前状态复核使用的凭证快照。
	initialDetail, credentialErr := service.loadCredential(ctx, request.UserID, shipmentContext.AccountID)
	if credentialErr != nil {
		return ShipmentEvidenceResult{}, credentialErr
	}
	// statusCtx、statusCancel 限制提交前平台状态读取时间。
	statusCtx, statusCancel := context.WithTimeout(ctx, shipmentStatusTimeout)
	// platformStatus、statusErr 是最新平台订单状态及请求错误。
	platformStatus, statusErr := service.runtime.FetchShipmentStatus(statusCtx, initialDetail, shipmentContext.OrderID)
	statusCancel()
	// statusCredentialErr 是状态读取响应 Cookie 的协调错误。
	statusCredentialErr := service.persistCredential(ctx, request.UserID, shipmentContext.AccountID, initialDetail, shipmentStatusCookie(platformStatus))
	if statusErr != nil {
		if errors.Is(statusErr, context.DeadlineExceeded) {
			return ShipmentEvidenceResult{}, fmt.Errorf("%w: %v", ErrShipmentStatusTimeout, statusErr)
		}
		if service.runtime.IsSessionExpired(statusErr) {
			service.runtime.RecoverExpiredSession(ctx, shipmentContext.AccountID, statusErr)
		}
		return ShipmentEvidenceResult{}, statusErr
	}
	if statusCredentialErr != nil {
		return ShipmentEvidenceResult{}, fmt.Errorf("提交发货前账号凭证已变化: %w", statusCredentialErr)
	}
	if platformStatus == nil {
		return ShipmentEvidenceResult{}, ErrShipmentNotEligible
	}
	// latestPlatformStatus 是提交前从闲鱼详情得到的规范状态。
	latestPlatformStatus := NormalizeOrderStatus(platformStatus.OrderStatus)
	if latestPlatformStatus != "pending_ship" {
		if latestPlatformStatus != "unknown" {
			// staleWriteErr 让已经完成的影子接管同步到平台最新终态，避免页面继续把它显示为待发货。
			staleWriteErr := service.repository.UpsertOrder(ctx, order.OrderID, UpsertOptions{CookieID: order.CookieID, OrderStatus: latestPlatformStatus})
			if staleWriteErr != nil {
				service.runtime.ReportPersistenceFailure(order.OrderID, staleWriteErr)
			}
		}
		return ShipmentEvidenceResult{}, ErrShipmentNotEligible
	}
	// runKey 是账号和订单组成的稳定幂等键，同一订单不因描述或图片变化重复发货。
	runKey := "ship_evidence:" + shipmentContext.AccountID + ":" + shipmentContext.OrderID
	// claimed、claimErr 是最终平台写操作的原子抢占结果。
	claimed, claimErr := service.repository.ClaimShipmentEvidence(ctx, runKey, shipmentContext.AccountID, shipmentContext.OrderID, service.now().UTC().Unix())
	if claimErr != nil {
		return ShipmentEvidenceResult{}, claimErr
	}
	if !claimed {
		return ShipmentEvidenceResult{}, ErrShipmentAlreadyHandled
	}
	// submitDetail、submitCredentialErr 是状态复核 Cookie 协调后重新读取的权威凭证。
	submitDetail, submitCredentialErr := service.loadCredential(ctx, request.UserID, shipmentContext.AccountID)
	if submitCredentialErr != nil {
		_ = service.finish(ctx, runKey, "failed", 0, 1, submitCredentialErr.Error())
		return ShipmentEvidenceResult{}, submitCredentialErr
	}
	// submitCtx、submitCancel 限制图片上传和最终无需寄件的总时间。
	submitCtx, submitCancel := context.WithTimeout(ctx, shipmentEvidenceTimeout)
	defer submitCancel()
	// platformResult、submitErr 是上传和最终平台写请求的结果。
	platformResult, submitErr := service.runtime.SubmitShipmentEvidence(submitCtx, submitDetail, shipmentContext.OrderID, request.TradeText, request.Images)
	// credentialWarning 是平台响应 Cookie 未能安全协调的告警。
	credentialWarning := service.persistCredential(ctx, request.UserID, shipmentContext.AccountID, submitDetail, shipmentResultCookie(platformResult))
	if submitErr != nil {
		if service.runtime.IsSessionExpired(submitErr) {
			service.runtime.RecoverExpiredSession(ctx, shipmentContext.AccountID, submitErr)
			_ = service.finish(ctx, runKey, "failed", 0, 1, submitErr.Error())
			return ShipmentEvidenceResult{}, submitErr
		}
		if platformResult == nil || !platformResult.ShipmentAttempted {
			_ = service.finish(ctx, runKey, "failed", 0, 1, submitErr.Error())
			return ShipmentEvidenceResult{}, submitErr
		}
		// finishErr 保存不明确最终请求的隔离状态；写入失败时 running 仍会阻止重复发货。
		finishErr := service.finish(ctx, runKey, "needs_review", 0, 1, submitErr.Error())
		if finishErr != nil {
			return ShipmentEvidenceResult{}, errors.Join(ErrShipmentNeedsReview, finishErr)
		}
		return ShipmentEvidenceResult{Status: "needs_review", Message: ErrShipmentNeedsReview.Error(), OrderID: shipmentContext.OrderID}, ErrShipmentNeedsReview
	}
	if platformResult == nil || !platformResult.Success {
		// message 是平台明确拒绝时保存和展示的非敏感说明。
		message := "闲鱼未确认发货成功"
		if platformResult != nil && strings.TrimSpace(platformResult.Message) != "" {
			message = strings.TrimSpace(platformResult.Message)
		}
		_ = service.finish(ctx, runKey, "failed", 0, 1, message)
		return ShipmentEvidenceResult{Status: "failed", Message: message, OrderID: shipmentContext.OrderID}, fmt.Errorf("%w: %s", ErrShipmentNotEligible, message)
	}
	return service.finishSuccess(ctx, runKey, order, platformResult, credentialWarning)
}

// Proof 按认证用户和订单读取已经保存的 ERP 发货凭证，不访问闲鱼平台。
func (service *ShipmentEvidenceService) Proof(ctx context.Context, userID int64, orderID string) (ShipmentProofResult, error) {
	if service == nil || service.repository == nil {
		return ShipmentProofResult{}, errors.New("发货凭证依赖未初始化")
	}
	// proof、readErr 是通过用户和订单归属校验的 ERP 发货凭证。
	proof, readErr := service.repository.GetShipmentProofForUser(ctx, userID, strings.TrimSpace(orderID))
	if readErr != nil {
		return ShipmentProofResult{}, readErr
	}
	if proof == nil {
		return ShipmentProofResult{}, ErrNotFound
	}
	return ShipmentProofResult{Proof: *proof}, nil
}

// validateRequest 校验依赖、卖家付款卡片、描述长度和图片格式，不产生平台或订单写入。
func (service *ShipmentEvidenceService) validateRequest(ctx context.Context, request ShipmentEvidenceRequest) (*ShipmentContext, error) {
	if service == nil || service.repository == nil || service.runtime == nil {
		return nil, errors.New("凭证发货依赖未初始化")
	}
	// accountID、orderID 是去空白后的卖家账号和订单号。
	accountID, orderID := strings.TrimSpace(request.AccountID), strings.TrimSpace(request.OrderID)
	if accountID == "" || !shipmentOrderPattern.MatchString(orderID) {
		return nil, NewValidationError("发货需要有效账号和数字订单号")
	}
	if utf8.RuneCountInString(request.TradeText) > 200 {
		return nil, NewValidationError("相关描述不能超过 200 字")
	}
	// imageErr 是图片数量、MIME 或大小不符合官方限制的输入错误。
	if imageErr := validateShipmentImages(request.Images); imageErr != nil {
		return nil, imageErr
	}
	if !service.runtime.ShipmentEvidenceAvailable() {
		return nil, ErrShipmentUnavailable
	}
	// shipmentContext、readErr 是归属当前用户且没有后续终态的卖家付款卡片。
	shipmentContext, readErr := service.repository.GetPendingShipment(ctx, request.UserID, accountID, orderID)
	if readErr != nil {
		if errors.Is(readErr, ErrNotFound) {
			return nil, ErrShipmentNotEligible
		}
		return nil, readErr
	}
	if shipmentContext == nil || shipmentContext.AccountID != accountID || shipmentContext.OrderID != orderID || strings.TrimSpace(shipmentContext.BuyerID) == "" || shipmentContext.BuyerID == accountID {
		return nil, ErrShipmentNotEligible
	}
	return shipmentContext, nil
}

// validateShipmentImages 检查官方卖家工作台允许的数量、MIME 和严格小于 3MB 的大小。
func validateShipmentImages(images []ShipmentEvidenceImage) error {
	if len(images) > shipmentEvidenceMaxImages {
		return NewValidationError("相关凭证最多上传 3 张")
	}
	// imageIndex 是当前待校验并清理文件名的凭证位置。
	for imageIndex := range images {
		// image 是当前内存凭证；其字节不会被复制或持久化。
		image := &images[imageIndex]
		image.Filename = normalizeShipmentFilename(image.Filename)
		// contentType 是统一大小写后的图片 MIME。
		contentType := strings.ToLower(strings.TrimSpace(image.ContentType))
		if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/jpg" {
			return NewValidationError("相关凭证只支持 PNG 或 JPEG 图片")
		}
		if len(image.Data) == 0 || len(image.Data) >= shipmentEvidenceMaxImageBytes {
			return NewValidationError("每张相关凭证必须小于 3MB")
		}
	}
	return nil
}

// ensureSellerOrder 安全接管买家影子订单或创建缺失卖家订单，并复核本地状态仍为待发货。
func (service *ShipmentEvidenceService) ensureSellerOrder(ctx context.Context, shipmentContext ShipmentContext) (*Order, error) {
	// _, promotionErr 是严格条件接管结果；未命中时继续检查是否已是正确卖家订单。
	_, promotionErr := service.repository.PromoteBuyerShadowOrder(ctx, shipmentContext)
	if promotionErr != nil {
		return nil, promotionErr
	}
	// order、readErr 是接管后的订单实体和读取错误。
	order, readErr := service.repository.GetOrder(ctx, shipmentContext.OrderID)
	if errors.Is(readErr, ErrNotFound) {
		// createErr 是卖家付款卡片首次建立订单事实的写入错误。
		createErr := service.repository.UpsertOrder(ctx, shipmentContext.OrderID, UpsertOptions{
			CookieID: shipmentContext.AccountID, BuyerID: shipmentContext.BuyerID, ItemID: shipmentContext.ItemID,
			ChatID: shipmentContext.ChatID, OrderStatus: "pending_ship",
		})
		if createErr != nil {
			return nil, createErr
		}
		order, readErr = service.repository.GetOrder(ctx, shipmentContext.OrderID)
	}
	if readErr != nil {
		return nil, readErr
	}
	if order != nil && order.CookieID == shipmentContext.AccountID && NormalizeOrderStatus(order.OrderStatus) == "processing" {
		// advanceErr 用精确卖家付款卡片把历史早期投影推进到待发货；最终写操作前仍会读取闲鱼最新状态。
		advanceErr := service.repository.UpsertOrder(ctx, shipmentContext.OrderID, UpsertOptions{
			CookieID: shipmentContext.AccountID, BuyerID: shipmentContext.BuyerID, ItemID: shipmentContext.ItemID,
			ChatID: shipmentContext.ChatID, OrderStatus: "pending_ship",
		})
		if advanceErr != nil {
			return nil, advanceErr
		}
		order, readErr = service.repository.GetOrder(ctx, shipmentContext.OrderID)
		if readErr != nil {
			return nil, readErr
		}
	}
	if order == nil || order.CookieID != shipmentContext.AccountID || NormalizeOrderStatus(order.OrderStatus) != "pending_ship" {
		return nil, ErrShipmentNotEligible
	}
	return order, nil
}

// loadCredential 在短时账号锁内读取并复核平台凭证，不持锁执行网络请求。
func (service *ShipmentEvidenceService) loadCredential(ctx context.Context, userID int64, accountID string) (*PlatformRuntimeData, error) {
	// unlock 是账号凭证短时读取锁释放函数。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// detail、loadErr 是权威平台凭证视图及读取错误。
	detail, loadErr := service.repository.LoadCookiePlatformDetail(ctx, accountID)
	if loadErr != nil {
		return nil, loadErr
	}
	if detail == nil || detail.UserID != userID || !service.runtime.CredentialAvailable(detail) {
		return nil, errors.New("账号凭证已变化，请重新登录后再发货")
	}
	return detail, nil
}

// shipmentCookieResult 抽象状态复核和发货提交共同的 Cookie 会话结果。
type shipmentCookieResult struct {
	// UpdatedCookies 是平面 Cookie 兼容结果。
	UpdatedCookies string
	// CookieUpdate 是完整 Cookie Jar 会话变化。
	CookieUpdate RefreshCookieUpdate
}

// shipmentStatusCookie 将状态复核结果转换为凭证协调输入。
func shipmentStatusCookie(result *ShipmentStatusPlatformResult) *shipmentCookieResult {
	if result == nil {
		return nil
	}
	return &shipmentCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// shipmentResultCookie 将最终发货结果转换为凭证协调输入。
func shipmentResultCookie(result *ShipmentEvidencePlatformResult) *shipmentCookieResult {
	if result == nil {
		return nil
	}
	return &shipmentCookieResult{UpdatedCookies: result.UpdatedCookies, CookieUpdate: result.CookieUpdate}
}

// persistCredential 协调平台响应 Cookie；并发出现较新凭证时返回告警且绝不覆盖。
func (service *ShipmentEvidenceService) persistCredential(ctx context.Context, userID int64, accountID string, detail *PlatformRuntimeData, result *shipmentCookieResult) error {
	if detail == nil || result == nil {
		return nil
	}
	// unlock 是 Cookie 写回期间的账号短时互斥锁。
	unlock := service.repository.LockCredentials(accountID)
	defer unlock()
	// latest、loadErr 是写回前的最新凭证视图和读取错误。
	latest, loadErr := service.repository.LoadCookiePlatformDetail(ctx, accountID)
	if loadErr != nil {
		return loadErr
	}
	if latest == nil || latest.UserID != userID || latest.Value != detail.Value || latest.MetadataJSON != detail.MetadataJSON {
		return errors.New("账号凭证已被其他登录流程更新")
	}
	// value、valueChanged、handled、persistErr 是完整 Cookie Jar 的持久化结果。
	value, valueChanged, handled, persistErr := service.runtime.PersistCookieSession(ctx, detail, result.CookieUpdate)
	if persistErr != nil {
		return persistErr
	}
	if handled {
		if valueChanged && value != "" {
			service.runtime.UpdateRunningCookie(ctx, accountID, value)
		}
		return nil
	}
	if result.UpdatedCookies == "" || result.UpdatedCookies == detail.Value {
		return nil
	}
	// updateErr 是旧式平面 Cookie 响应的受控持久化错误。
	if updateErr := service.repository.UpdateRenewalCookie(ctx, accountID, result.UpdatedCookies, detail.MetadataJSON, service.now().UTC().Unix()); updateErr != nil {
		return updateErr
	}
	service.runtime.UpdateRunningCookie(ctx, accountID, result.UpdatedCookies)
	return nil
}

// finishSuccess 写入明确平台成功后的本地状态、补偿、求花任务和通知。
func (service *ShipmentEvidenceService) finishSuccess(ctx context.Context, runKey string, order *Order, platformResult *ShipmentEvidencePlatformResult, credentialWarning error) (ShipmentEvidenceResult, error) {
	// systemShipped 标记订单由 ERP 调用平台确认发货。
	systemShipped := true
	// upsertErr 是平台成功后的本地订单状态写入错误。
	upsertErr := service.repository.UpsertOrder(ctx, order.OrderID, UpsertOptions{
		CookieID: order.CookieID, OrderStatus: "shipped", SystemShipped: &systemShipped, ItemID: order.ItemID,
		BuyerID: order.BuyerID, ReceiverName: order.ReceiverName, ReceiverPhone: order.ReceiverPhone,
		ReceiverAddress: order.ReceiverAddress, ReceiverCity: order.ReceiverCity, ChatID: order.ChatID,
		SpecName: order.SpecName, SpecValue: order.SpecValue, Quantity: order.Quantity, Amount: order.Amount,
	})
	if upsertErr == nil {
		upsertErr = service.repository.MarkOrderShippedAt(ctx, order.OrderID)
	}
	if upsertErr == nil {
		// proof、proofErr 是完成 HTTPS 校验后准备持久化的 ERP 发货凭证和错误。
		proof, proofErr := normalizedShipmentProof(order, platformResult, service.now().UTC().Unix())
		if proofErr != nil {
			upsertErr = proofErr
		} else {
			upsertErr = service.repository.SaveShipmentProof(ctx, proof)
		}
	}
	// status、message 是远端成功后的本地收口状态和展示说明。
	status, message := "succeeded", "闲鱼已确认发货，等待系统卡片同步"
	// postActionWarning 保存本地收口或求花调度告警，写入运行记录但不误报为凭证并发变化。
	var postActionWarning error
	if upsertErr != nil {
		service.runtime.ReportPersistenceFailure(order.OrderID, upsertErr)
		// reconciliationCtx、reconciliationCancel 限制补偿记录写入时间，不受请求取消影响。
		reconciliationCtx, reconciliationCancel := context.WithTimeout(context.Background(), 5*time.Second)
		// reconciliationErr 是外部成功后补偿记录创建失败的原因。
		_, reconciliationErr := service.runtime.RecordReconciliation(reconciliationCtx, order.OrderID, order.CookieID, "shipment_evidence", upsertErr.Error())
		reconciliationCancel()
		if reconciliationErr != nil {
			upsertErr = errors.Join(upsertErr, reconciliationErr)
		}
		postActionWarning = upsertErr
		status, message = "needs_review", "闲鱼已确认发货，但本地状态待补偿，请勿重复提交"
	} else if // scheduleErr 是秒级求花任务创建失败的原因，不能回退已经成功的发货事实。
	scheduleErr := service.runtime.ScheduleRedFlowerAfterShipment(ctx, order.OrderID); scheduleErr != nil {
		postActionWarning = scheduleErr
		status = "succeeded_with_warning"
		message = "闲鱼已确认发货；自动求花将由分钟补偿扫描恢复"
	}
	if credentialWarning != nil && status == "succeeded" {
		status = "succeeded_with_warning"
		message += "；账号凭证已发生变化，未覆盖较新的登录状态"
	}
	// finishErr 是平台成功后的幂等运行终态写入错误。
	finishErr := service.finish(ctx, runKey, "success", 1, 0, credentialErrorText(errors.Join(credentialWarning, postActionWarning)))
	if finishErr != nil {
		status, message = "needs_review", "闲鱼已确认发货，但本地运行状态保存失败，请勿重复提交"
	}
	service.runtime.NotifyDelivery(order.CookieID, order.BuyerID, order.ItemID, order.ChatID, fmt.Sprintf("凭证发货成功（订单 %s）", order.OrderID))
	return ShipmentEvidenceResult{Success: true, Status: status, Message: message, OrderID: order.OrderID}, nil
}

// normalizedShipmentProof 校验并复制最终平台提交内容，拒绝非 HTTPS 或超过三张的动态地址。
func normalizedShipmentProof(order *Order, result *ShipmentEvidencePlatformResult, submittedAt int64) (ShipmentProof, error) {
	if order == nil || result == nil || strings.TrimSpace(order.OrderID) == "" || strings.TrimSpace(order.CookieID) == "" || submittedAt <= 0 {
		return ShipmentProof{}, errors.New("发货成功响应缺少凭证上下文")
	}
	if len(result.ImageURLs) > shipmentEvidenceMaxImages {
		return ShipmentProof{}, errors.New("发货成功响应图片数量超出限制")
	}
	// imageURLs 保存通过 HTTPS 主机校验后的独立地址副本。
	imageURLs := make([]string, 0, len(result.ImageURLs))
	for _, rawURL := range result.ImageURLs { // rawURL 是当前待验证的官方上传地址。
		// parsedURL、parseErr 是当前图片地址的结构化结果和解析错误。
		parsedURL, parseErr := url.Parse(strings.TrimSpace(rawURL))
		if parseErr != nil || !strings.EqualFold(parsedURL.Scheme, "https") || strings.TrimSpace(parsedURL.Hostname()) == "" {
			return ShipmentProof{}, errors.New("发货成功响应包含不安全的图片地址")
		}
		imageURLs = append(imageURLs, parsedURL.String())
	}
	return ShipmentProof{OrderID: order.OrderID, AccountID: order.CookieID, TradeText: result.TradeText, ImageURLs: imageURLs, Source: "erp", SubmittedAt: submittedAt}, nil
}

// finish 保存当前发货幂等运行终态。
func (service *ShipmentEvidenceService) finish(ctx context.Context, runKey, status string, success, failed int, message string) error {
	return service.repository.FinishShipmentEvidence(ctx, runKey, status, success, failed, message)
}

// normalizeShipmentFilename 移除客户端路径，只保留官方 multipart 可接受的短文件名。
func normalizeShipmentFilename(filename string) string {
	// normalized 是清理路径和首尾空白后的文件名。
	normalized := strings.TrimSpace(filepath.Base(filename))
	if normalized == "." || normalized == "" {
		return "evidence"
	}
	return normalized
}
