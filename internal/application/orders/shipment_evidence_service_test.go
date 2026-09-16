package orders

import (
	"context"
	"errors"
	"testing"
	"time"
)

// shipmentEvidenceRepositoryFake 保存凭证发货测试的卡片、订单和幂等状态。
type shipmentEvidenceRepositoryFake struct {
	// shipment 是卖家付款卡片上下文。
	shipment *ShipmentContext
	// order 是当前订单实体；测试允许从买家影子归属开始。
	order *Order
	// claimed 控制并记录幂等抢占结果。
	claimed bool
	// finishStatus 是应用服务最终写入的运行状态。
	finishStatus string
	// markedShipped 表示本地发货时间已写入。
	markedShipped bool
	// proof 保存平台明确成功后写入的 ERP 发货凭证。
	proof *ShipmentProof
}

// GetPendingShipment 返回预设卖家付款卡片并校验请求关联。
func (fake *shipmentEvidenceRepositoryFake) GetPendingShipment(_ context.Context, _ int64, accountID, orderID string) (*ShipmentContext, error) {
	if fake.shipment == nil || fake.shipment.AccountID != accountID || fake.shipment.OrderID != orderID {
		return nil, ErrNotFound
	}
	// copyContext 避免应用服务修改测试夹具。
	copyContext := *fake.shipment
	return &copyContext, nil
}

// PromoteBuyerShadowOrder 模拟同用户买家影子订单接管并保留既有业务字段。
func (fake *shipmentEvidenceRepositoryFake) PromoteBuyerShadowOrder(_ context.Context, shipmentContext ShipmentContext) (bool, error) {
	if fake.order == nil || fake.order.CookieID != shipmentContext.BuyerID || fake.order.BuyerID != shipmentContext.BuyerID || NormalizeOrderStatus(fake.order.OrderStatus) != "pending_ship" {
		return false, nil
	}
	fake.order.CookieID = shipmentContext.AccountID
	fake.order.ChatID = shipmentContext.ChatID
	fake.order.OrderStatus = "pending_ship"
	return true, nil
}

// GetOrder 返回当前测试订单副本。
func (fake *shipmentEvidenceRepositoryFake) GetOrder(_ context.Context, orderID string) (*Order, error) {
	if fake.order == nil || fake.order.OrderID != orderID {
		return nil, ErrNotFound
	}
	// copyOrder 防止调用方绕过 UpsertOrder 直接修改仓储状态。
	copyOrder := *fake.order
	return &copyOrder, nil
}

// UpsertOrder 创建缺失订单或应用明确发货状态补丁。
func (fake *shipmentEvidenceRepositoryFake) UpsertOrder(_ context.Context, orderID string, options UpsertOptions) error {
	if fake.order == nil {
		fake.order = &Order{OrderID: orderID}
	}
	if options.CookieID != "" {
		fake.order.CookieID = options.CookieID
	}
	if options.BuyerID != "" {
		fake.order.BuyerID = options.BuyerID
	}
	if options.ItemID != "" {
		fake.order.ItemID = options.ItemID
	}
	if options.ChatID != "" {
		fake.order.ChatID = options.ChatID
	}
	if options.OrderStatus != "" {
		fake.order.OrderStatus = options.OrderStatus
	}
	return nil
}

// MarkOrderShippedAt 记录明确平台成功后的本地发货时间写入。
func (fake *shipmentEvidenceRepositoryFake) MarkOrderShippedAt(context.Context, string) error {
	fake.markedShipped = true
	return nil
}

// ClaimShipmentEvidence 返回预设抢占结果并在首次调用后保持已抢占。
func (fake *shipmentEvidenceRepositoryFake) ClaimShipmentEvidence(context.Context, string, string, string, int64) (bool, error) {
	if !fake.claimed {
		fake.claimed = true
		return true, nil
	}
	return false, nil
}

// FinishShipmentEvidence 保存应用服务写入的幂等终态。
func (fake *shipmentEvidenceRepositoryFake) FinishShipmentEvidence(_ context.Context, _ string, status string, _, _ int, _ string) error {
	fake.finishStatus = status
	return nil
}

// SaveShipmentProof 保存测试中的 ERP 发货凭证副本。
func (fake *shipmentEvidenceRepositoryFake) SaveShipmentProof(_ context.Context, proof ShipmentProof) error {
	// copiedProof 防止调用方继续修改测试仓储持有的图片切片。
	copiedProof := proof
	copiedProof.ImageURLs = append([]string(nil), proof.ImageURLs...)
	fake.proof = &copiedProof
	return nil
}

// GetShipmentProofForUser 返回测试仓储保存的订单凭证。
func (fake *shipmentEvidenceRepositoryFake) GetShipmentProofForUser(_ context.Context, _ int64, orderID string) (*ShipmentProof, error) {
	if fake.proof == nil || fake.proof.OrderID != orderID {
		return nil, ErrNotFound
	}
	// copiedProof 防止只读服务修改仓储夹具。
	copiedProof := *fake.proof
	copiedProof.ImageURLs = append([]string(nil), fake.proof.ImageURLs...)
	return &copiedProof, nil
}

// LockCredentials 返回无状态测试锁释放函数。
func (fake *shipmentEvidenceRepositoryFake) LockCredentials(string) func() { return func() {} }

// LoadCookiePlatformDetail 返回归属测试用户的非空凭证视图。
func (fake *shipmentEvidenceRepositoryFake) LoadCookiePlatformDetail(_ context.Context, accountID string) (*PlatformRuntimeData, error) {
	return &PlatformRuntimeData{ID: accountID, UserID: 7, Value: "synthetic-cookie"}, nil
}

// UpdateRenewalCookie 在测试中不持久化平面 Cookie。
func (fake *shipmentEvidenceRepositoryFake) UpdateRenewalCookie(context.Context, string, string, string, int64) error {
	return nil
}

// shipmentEvidenceRuntimeFake 保存平台状态、最终响应和副作用计数。
type shipmentEvidenceRuntimeFake struct {
	// status 是提交前平台详情返回的状态。
	status string
	// submitResult 是最终无需寄件响应。
	submitResult *ShipmentEvidencePlatformResult
	// submitErr 是最终上传或发货错误。
	submitErr error
	// submitCalls 记录最终平台流程调用次数。
	submitCalls int
	// scheduled 记录自动求花任务创建次数。
	scheduled int
	// waitForStatusCancellation 表示状态复核必须等待调用上下文结束。
	waitForStatusCancellation bool
	// statusCalls 记录提交前状态复核调用次数。
	statusCalls int
}

// ShipmentEvidenceAvailable 表示测试平台能力完整。
func (fake *shipmentEvidenceRuntimeFake) ShipmentEvidenceAvailable() bool { return true }

// CredentialAvailable 只接受非空测试凭证。
func (fake *shipmentEvidenceRuntimeFake) CredentialAvailable(detail *PlatformRuntimeData) bool {
	return detail != nil && detail.Value != ""
}

// FetchShipmentStatus 返回预设最新平台订单状态。
func (fake *shipmentEvidenceRuntimeFake) FetchShipmentStatus(ctx context.Context, _ *PlatformRuntimeData, _ string) (*ShipmentStatusPlatformResult, error) {
	fake.statusCalls++
	if fake.waitForStatusCancellation {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &ShipmentStatusPlatformResult{OrderStatus: fake.status}, nil
}

// SubmitShipmentEvidence 记录调用并返回预设确定性或不明确结果。
func (fake *shipmentEvidenceRuntimeFake) SubmitShipmentEvidence(_ context.Context, _ *PlatformRuntimeData, _ string, _ string, _ []ShipmentEvidenceImage) (*ShipmentEvidencePlatformResult, error) {
	fake.submitCalls++
	return fake.submitResult, fake.submitErr
}

// PersistCookieSession 表示测试没有完整 Cookie Jar 变化。
func (fake *shipmentEvidenceRuntimeFake) PersistCookieSession(context.Context, *PlatformRuntimeData, RefreshCookieUpdate) (string, bool, bool, error) {
	return "", false, true, nil
}

// UpdateRunningCookie 在测试中不操作账号运行时。
func (fake *shipmentEvidenceRuntimeFake) UpdateRunningCookie(context.Context, string, string) {}

// RecoverExpiredSession 在测试中不启动真实登录恢复。
func (fake *shipmentEvidenceRuntimeFake) RecoverExpiredSession(context.Context, string, error) bool {
	return false
}

// IsSessionExpired 将测试错误统一视为普通网络错误。
func (fake *shipmentEvidenceRuntimeFake) IsSessionExpired(error) bool { return false }

// ScheduleRedFlowerAfterShipment 记录明确发货后的秒级任务创建。
func (fake *shipmentEvidenceRuntimeFake) ScheduleRedFlowerAfterShipment(context.Context, string) error {
	fake.scheduled++
	return nil
}

// NotifyDelivery 在测试中不发送真实通知。
func (fake *shipmentEvidenceRuntimeFake) NotifyDelivery(string, string, string, string, string) {}

// RecordReconciliation 返回固定补偿标识，测试成功路径不会调用。
func (fake *shipmentEvidenceRuntimeFake) RecordReconciliation(context.Context, string, string, string, string) (string, error) {
	return "reconciliation-test", nil
}

// ReportPersistenceFailure 在测试中不写日志。
func (fake *shipmentEvidenceRuntimeFake) ReportPersistenceFailure(string, error) {}

// TestShipmentEvidenceServicePromotesShadowAndShips 验证买家影子订单接管、平台状态复核和成功收口组成同一用例。
func TestShipmentEvidenceServicePromotesShadowAndShips(t *testing.T) {
	// repository 从买家账号持有的原始状态 2 影子订单开始，并保留既有金额和商品。
	repository := &shipmentEvidenceRepositoryFake{shipment: &ShipmentContext{AccountID: "seller-account", BuyerID: "buyer-account", OrderID: "5127372002248048713", ItemID: "card-item", ChatID: "seller-chat"}, order: &Order{OrderID: "5127372002248048713", CookieID: "buyer-account", BuyerID: "buyer-account", ItemID: "existing-item", OrderStatus: "2", Amount: "0.01"}}
	// runtime 返回平台待发货和明确提交成功。
	runtime := &shipmentEvidenceRuntimeFake{status: "2", submitResult: &ShipmentEvidencePlatformResult{Success: true, ShipmentAttempted: true, TradeText: "在线交付完成", ImageURLs: []string{"https://img.example/proof.png"}}}
	// service 使用固定时间，避免幂等日期受测试时钟影响。
	service := NewShipmentEvidenceService(repository, runtime, func() time.Time { return time.Unix(1_787_228_800, 0) })
	// result、shipErr 是带一张小图片的无需寄件结果。
	result, shipErr := service.Ship(context.Background(), ShipmentEvidenceRequest{UserID: 7, AccountID: "seller-account", OrderID: "5127372002248048713", TradeText: "在线交付完成", Images: []ShipmentEvidenceImage{{Filename: "proof.png", ContentType: "image/png", Data: []byte("png")}}})
	if shipErr != nil || !result.Success || repository.order.CookieID != "seller-account" || repository.order.ItemID != "existing-item" || repository.order.Amount != "0.01" || repository.order.OrderStatus != "shipped" || !repository.markedShipped || repository.finishStatus != "success" || runtime.submitCalls != 1 || runtime.scheduled != 1 || repository.proof == nil || repository.proof.TradeText != "在线交付完成" || len(repository.proof.ImageURLs) != 1 {
		t.Fatalf("result=%+v order=%+v repo=%+v runtime=%+v err=%v", result, repository.order, repository, runtime, shipErr)
	}
}

// TestShipmentEvidenceServiceAdvancesProcessingOrderFromExactPaidCard 验证历史 processing 投影可由精确卖家付款卡片安全推进并继续平台复核。
func TestShipmentEvidenceServiceAdvancesProcessingOrderFromExactPaidCard(t *testing.T) {
	// repository 保存已经归属卖家但仍停在待付款状态的历史订单和精确付款卡片。
	repository := &shipmentEvidenceRepositoryFake{shipment: &ShipmentContext{AccountID: "seller-account", BuyerID: "buyer-account", OrderID: "3316373186236021980", ItemID: "item-1", ChatID: "seller-chat"}, order: &Order{OrderID: "3316373186236021980", CookieID: "seller-account", BuyerID: "buyer-account", ItemID: "item-1", ChatID: "seller-chat", OrderStatus: "processing"}}
	// runtime 返回平台仍为待发货并明确接受最终提交。
	runtime := &shipmentEvidenceRuntimeFake{status: "pending_ship", submitResult: &ShipmentEvidencePlatformResult{Success: true, ShipmentAttempted: true}}
	// service 是待验证历史状态收敛的发货应用服务。
	service := NewShipmentEvidenceService(repository, runtime, nil)
	// result、shipErr 是历史订单通过付款卡片收敛后的发货结果。
	result, shipErr := service.Ship(context.Background(), ShipmentEvidenceRequest{UserID: 7, AccountID: "seller-account", OrderID: "3316373186236021980"})
	if shipErr != nil || !result.Success || runtime.statusCalls != 1 || runtime.submitCalls != 1 || repository.order.OrderStatus != "shipped" {
		t.Fatalf("result=%+v order=%+v runtime=%+v err=%v", result, repository.order, runtime, shipErr)
	}
}

// TestShipmentEvidenceServiceTimesOutBeforeClaim 验证状态复核超时明确表示尚未执行发货且不会创建幂等运行。
func TestShipmentEvidenceServiceTimesOutBeforeClaim(t *testing.T) {
	// repository 保存本地资格完整的卖家待发货订单。
	repository := &shipmentEvidenceRepositoryFake{shipment: &ShipmentContext{AccountID: "seller-account", BuyerID: "buyer-account", OrderID: "5127194847698062342"}, order: &Order{OrderID: "5127194847698062342", CookieID: "seller-account", BuyerID: "buyer-account", OrderStatus: "pending_ship"}}
	// runtime 模拟平台订单详情一直等待到上下文截止。
	runtime := &shipmentEvidenceRuntimeFake{waitForStatusCancellation: true}
	// service 是待验证状态复核超时隔离的发货应用服务。
	service := NewShipmentEvidenceService(repository, runtime, nil)
	// ctx、cancel 把测试等待压缩到毫秒级，同时覆盖生产十二秒上限的错误分类。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// _, shipErr 是状态复核截止后的稳定应用错误。
	_, shipErr := service.Ship(ctx, ShipmentEvidenceRequest{UserID: 7, AccountID: "seller-account", OrderID: "5127194847698062342"})
	if !errors.Is(shipErr, ErrShipmentStatusTimeout) || repository.claimed || runtime.submitCalls != 0 || runtime.statusCalls != 1 {
		t.Fatalf("claimed=%v statusCalls=%d submitCalls=%d err=%v", repository.claimed, runtime.statusCalls, runtime.submitCalls, shipErr)
	}
}

// TestShipmentEvidenceServiceReadsSavedProofAndRejectsUnsafeURL 验证只读凭证返回和 HTTPS 图片门禁。
func TestShipmentEvidenceServiceReadsSavedProofAndRejectsUnsafeURL(t *testing.T) {
	// repository 保存当前用户可读取的 ERP 发货凭证。
	repository := &shipmentEvidenceRepositoryFake{proof: &ShipmentProof{OrderID: "proof-order", AccountID: "seller-account", TradeText: "已交付", ImageURLs: []string{"https://img.example/proof.png"}, Source: "erp", SubmittedAt: 100}}
	// service 是只读凭证查询使用的应用服务。
	service := NewShipmentEvidenceService(repository, &shipmentEvidenceRuntimeFake{}, nil)
	// result、readErr 是已保存凭证的读取结果和错误。
	result, readErr := service.Proof(context.Background(), 7, "proof-order")
	if readErr != nil || result.Proof.TradeText != "已交付" || len(result.Proof.ImageURLs) != 1 {
		t.Fatalf("result=%+v err=%v", result, readErr)
	}
	// _, unsafeErr 是非 HTTPS 图片必须触发的安全错误。
	_, unsafeErr := normalizedShipmentProof(&Order{OrderID: "proof-order", CookieID: "seller-account"}, &ShipmentEvidencePlatformResult{ImageURLs: []string{"http://img.example/proof.png"}}, 100)
	if unsafeErr == nil {
		t.Fatal("非 HTTPS 发货凭证不应进入持久化")
	}
}

// TestShipmentEvidenceServiceRejectsStalePlatformStatus 验证平台不再待发货时不会抢占或上传凭证。
func TestShipmentEvidenceServiceRejectsStalePlatformStatus(t *testing.T) {
	// repository 已经是正确卖家订单，本地仍显示待发货。
	repository := &shipmentEvidenceRepositoryFake{shipment: &ShipmentContext{AccountID: "seller-account", BuyerID: "buyer-account", OrderID: "5127372002248048713"}, order: &Order{OrderID: "5127372002248048713", CookieID: "seller-account", BuyerID: "buyer-account", OrderStatus: "pending_ship"}}
	// runtime 返回平台已经发货，应用必须在最终写操作前停止。
	runtime := &shipmentEvidenceRuntimeFake{status: "shipped"}
	// service 是待验证陈旧状态门禁的应用服务。
	service := NewShipmentEvidenceService(repository, runtime, nil)
	// _, shipErr 是应被拒绝的陈旧页面提交结果。
	_, shipErr := service.Ship(context.Background(), ShipmentEvidenceRequest{UserID: 7, AccountID: "seller-account", OrderID: "5127372002248048713"})
	if !errors.Is(shipErr, ErrShipmentNotEligible) || repository.claimed || runtime.submitCalls != 0 {
		t.Fatalf("claimed=%v calls=%d err=%v", repository.claimed, runtime.submitCalls, shipErr)
	}
}

// TestShipmentEvidenceServiceQuarantinesUncertainSubmit 验证最终请求网络错误进入 needs_review 并禁止安全重放。
func TestShipmentEvidenceServiceQuarantinesUncertainSubmit(t *testing.T) {
	// repository 和 runtime 提供可提交订单，但最终响应在请求发出后丢失。
	repository := &shipmentEvidenceRepositoryFake{shipment: &ShipmentContext{AccountID: "seller-account", BuyerID: "buyer-account", OrderID: "5127372002248048713"}, order: &Order{OrderID: "5127372002248048713", CookieID: "seller-account", BuyerID: "buyer-account", OrderStatus: "pending_ship"}}
	// runtime 模拟最终请求已发出但响应丢失。
	runtime := &shipmentEvidenceRuntimeFake{status: "pending_ship", submitResult: &ShipmentEvidencePlatformResult{ShipmentAttempted: true}, submitErr: errors.New("response lost")}
	// service 是待验证不明确结果隔离的应用服务。
	service := NewShipmentEvidenceService(repository, runtime, nil)
	// result、shipErr 是不明确平台结果和稳定错误分类。
	result, shipErr := service.Ship(context.Background(), ShipmentEvidenceRequest{UserID: 7, AccountID: "seller-account", OrderID: "5127372002248048713"})
	if !errors.Is(shipErr, ErrShipmentNeedsReview) || result.Status != "needs_review" || repository.finishStatus != "needs_review" {
		t.Fatalf("result=%+v finish=%s err=%v", result, repository.finishStatus, shipErr)
	}
}

// TestValidateShipmentImagesMatchesOfficialLimits 验证图片数量、MIME 和严格小于 3MB 的边界。
func TestValidateShipmentImagesMatchesOfficialLimits(t *testing.T) {
	// valid 是官方接受的最小 PNG 凭证。
	valid := ShipmentEvidenceImage{Filename: "folder/proof.png", ContentType: "image/png", Data: []byte("png")}
	// validationErr 是有效图片不应产生的校验错误。
	if validationErr := validateShipmentImages([]ShipmentEvidenceImage{valid}); validationErr != nil {
		t.Fatal(validationErr)
	}
	// invalidCases 是数量、类型和大小三个拒绝分支。
	invalidCases := [][]ShipmentEvidenceImage{
		{valid, valid, valid, valid},
		{{Filename: "proof.gif", ContentType: "image/gif", Data: []byte("gif")}},
		{{Filename: "proof.png", ContentType: "image/png", Data: make([]byte, shipmentEvidenceMaxImageBytes)}},
	}
	// images 是当前预期返回 ValidationError 的图片集合。
	for _, images := range invalidCases {
		// validationErr 是当前无效图片集合必须返回的校验错误。
		if validationErr := validateShipmentImages(images); validationErr == nil {
			t.Fatalf("images=%d should fail", len(images))
		}
	}
}
