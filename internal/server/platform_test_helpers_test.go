package server

import (
	"context"
	"errors"
	"sync"

	"xianyu-go/internal/account"
	"xianyu-go/internal/adapter"
	lifecycleapp "xianyu-go/internal/application/lifecycle"
	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/chat"
)

// testServerManagers 保存仅供测试夹具访问的账号运行时；生产 Server 不持有该对象。
var testServerManagers sync.Map

// testServerPlatforms 保存仅供测试夹具替换的平台能力；生产 Server 不保存该对象。
var testServerPlatforms sync.Map

// testServerBatchLifecycle 保存测试需要观测的批量发布恢复组件；生产 Server 不持有 worker。
var testServerBatchLifecycle sync.Map

// testServerChats 保存测试构造时的聊天领域事件中心，生产 Server 不持有该实现。
var testServerChats sync.Map

// testServerOrderServices 保存仅供生命周期测试访问的订单服务集合；生产 Server 不持有该对象。
var testServerOrderServices sync.Map

// testBatchLifecycle 是测试调用组合层批量恢复组件的窄句柄。
type testBatchLifecycle struct {
	// start 启动恢复扫描。
	start func(context.Context) error
	// close 取消并等待恢复组件。
	close func(context.Context) error
}

// testPlatformPort 是测试专用的可变平台 Port，允许单个测试替换客户端而不污染生产 Server 状态。
type testPlatformPort struct {
	// mu 保护测试过程中可变的平台客户端引用。
	mu sync.RWMutex
	// mtop 保存当前测试使用的 MTOP 客户端。
	mtop adapter.MTOPClient
	// longLogin 保存当前测试使用的长登录客户端。
	longLogin adapter.LongLoginClient
	// qr 保存当前测试使用的二维码登录客户端。
	qr adapter.QRLoginService
}

// testOrdersTransport 将测试组合根订单服务集合投影为 Server 的订单 Port。
type testOrdersTransport struct {
	// services 是测试组合根已经完成构造的订单服务集合。
	services *orderapp.ServiceSet
}

// testOrderRefreshJobsTransport 将测试组合根订单刷新服务投影为 HTTP Port，并隔离请求 Context 与 worker 生命周期。
type testOrderRefreshJobsTransport struct {
	// service 是测试组合根构造的订单刷新任务应用服务。
	service *orderapp.RefreshJobService
}

// CreateAndStart 使用请求 Context 完成创建，并使用测试进程 Context 驱动后台 worker。
func (transport testOrderRefreshJobsTransport) CreateAndStart(requestCtx context.Context, userID int64, cookieID, status string) (orderapp.RefreshJobStartResult, error) {
	return transport.service.CreateAndStart(requestCtx, context.Background(), userID, cookieID, status)
}

// CreateAndStartInMode 让测试 transport 按请求指定模式启动订单刷新任务。
func (transport testOrderRefreshJobsTransport) CreateAndStartInMode(requestCtx context.Context, userID int64, cookieID, status, mode string) (orderapp.RefreshJobStartResult, error) {
	return transport.service.CreateAndStartInMode(requestCtx, context.Background(), userID, cookieID, status, mode)
}

// GetJob 返回当前用户拥有的订单刷新任务状态。
func (transport testOrderRefreshJobsTransport) GetJob(ctx context.Context, userID int64, jobID string) (*orderapp.RefreshJob, error) {
	return transport.service.GetJob(ctx, userID, jobID)
}

// CancelForUser 取消当前用户拥有的订单刷新任务。
func (transport testOrderRefreshJobsTransport) CancelForUser(ctx context.Context, userID int64, jobID string) (orderapp.RefreshJobCancelResult, error) {
	return transport.service.CancelForUser(ctx, userID, jobID)
}

// RefreshSingle 转发测试的单订单刷新用例。
func (transport testOrdersTransport) RefreshSingle(ctx context.Context, userID int64, orderID string) (orderapp.SingleRefreshResult, error) {
	return transport.services.Refresh.RefreshSingle(ctx, userID, orderID)
}

// Refresh 转发测试的批量订单刷新用例。
func (transport testOrdersTransport) Refresh(ctx context.Context, userID int64, cookieID, status string) (orderapp.RefreshResult, error) {
	return transport.services.Refresh.Refresh(ctx, userID, cookieID, status)
}

// List 转发测试的订单列表用例。
func (transport testOrdersTransport) List(ctx context.Context, query orderapp.ListQuery) (orderapp.ListResult, error) {
	return transport.services.List.List(ctx, query)
}

// ConversationContext 转发测试 Chat 历史订单上下文查询。
func (transport testOrdersTransport) ConversationContext(ctx context.Context, query orderapp.ConversationContextQuery) (orderapp.ConversationContextResult, error) {
	return transport.services.ConversationContext.Context(ctx, query)
}

// Get 转发测试的订单详情用例。
func (transport testOrdersTransport) Get(ctx context.Context, userID int64, orderID string) (*orderapp.Order, error) {
	return transport.services.Detail.Get(ctx, userID, orderID)
}

// GetView 转发测试的订单详情视图用例。
func (transport testOrdersTransport) GetView(ctx context.Context, userID int64, orderID string) (orderapp.DetailResult, error) {
	return transport.services.Detail.GetView(ctx, userID, orderID)
}

// Delete 转发测试的订单删除用例。
func (transport testOrdersTransport) Delete(ctx context.Context, userID int64, orderID string) error {
	return transport.services.Delete.Delete(ctx, userID, orderID)
}

// Update 转发测试的订单更新用例。
func (transport testOrdersTransport) Update(ctx context.Context, userID int64, orderID string, request orderapp.UpdateRequest) error {
	return transport.services.Update.Update(ctx, userID, orderID, request)
}

// Import 转发测试的订单导入用例。
func (transport testOrdersTransport) Import(ctx context.Context, userID int64, inputs []orderapp.ImportOrder) (orderapp.ImportResult, error) {
	return transport.services.Import.Import(ctx, userID, inputs)
}

// ManualShip 转发测试的订单手动发货用例。
func (transport testOrdersTransport) ManualShip(ctx context.Context, request orderapp.ManualShipRequest) (orderapp.ManualShipResult, error) {
	return transport.services.ManualShip.ManualShip(ctx, request)
}

// ShipWithEvidence 转发测试聊天付款卡片的凭证发货用例。
func (transport testOrdersTransport) ShipWithEvidence(ctx context.Context, request orderapp.ShipmentEvidenceRequest) (orderapp.ShipmentEvidenceResult, error) {
	return transport.services.ShipmentEvidence.Ship(ctx, request)
}

// ShipmentProof 转发测试订单 ERP 发货凭证只读查询。
func (transport testOrdersTransport) ShipmentProof(ctx context.Context, userID int64, orderID string) (orderapp.ShipmentProofResult, error) {
	return transport.services.ShipmentEvidence.Proof(ctx, userID, orderID)
}

// RequestRedFlower 转发测试的订单手动求花用例。
func (transport testOrdersTransport) RequestRedFlower(ctx context.Context, request orderapp.RedFlowerRequest) (orderapp.RedFlowerRequestResult, error) {
	return transport.services.RequestRedFlower.Request(ctx, request)
}

// RedFlowerStatus 转发测试的订单求花状态查询。
func (transport testOrdersTransport) RedFlowerStatus(ctx context.Context, userID int64, orderID string) (orderapp.RedFlowerRequestStatusResult, error) {
	return transport.services.RequestRedFlower.Status(ctx, userID, orderID)
}

// PriceAdjustmentForm 转发测试订单的只读动态改价表单请求。
func (transport testOrdersTransport) PriceAdjustmentForm(ctx context.Context, request orderapp.PriceAdjustmentFormRequest) (orderapp.PriceAdjustmentFormResult, error) {
	return transport.services.AdjustPrice.Form(ctx, request)
}

// AdjustPrice 转发测试订单的真实改价应用请求；平台依赖由测试替身隔离。
func (transport testOrdersTransport) AdjustPrice(ctx context.Context, request orderapp.PriceAdjustmentRequest) (orderapp.PriceAdjustmentResult, error) {
	return transport.services.AdjustPrice.Adjust(ctx, request)
}

// RefundDetail 转发测试订单的官方只读退款详情请求。
func (transport testOrdersTransport) RefundDetail(ctx context.Context, request orderapp.RefundDetailRequest) (orderapp.RefundDetailResult, error) {
	return transport.services.RefundDetail.Get(ctx, request)
}

// RefundAction 转发测试订单的真实退款处理请求；平台依赖由测试替身隔离。
func (transport testOrdersTransport) RefundAction(ctx context.Context, request orderapp.RefundActionRequest) (orderapp.RefundActionResult, error) {
	return transport.services.RefundDetail.Act(ctx, request)
}

// StartMerchantRefundVerification 转发测试支付验证创建请求。
func (transport testOrdersTransport) StartMerchantRefundVerification(ctx context.Context, request orderapp.MerchantRefundVerificationStartRequest) (orderapp.MerchantRefundVerificationStartResult, error) {
	return transport.services.RefundDetail.StartMerchantRefundVerification(ctx, request)
}

// CompleteMerchantRefundVerification 转发测试支付验证完成请求。
func (transport testOrdersTransport) CompleteMerchantRefundVerification(ctx context.Context, request orderapp.MerchantRefundVerificationCompleteRequest) (orderapp.RefundActionResult, error) {
	return transport.services.RefundDetail.CompleteMerchantRefundVerification(ctx, request)
}

// MerchantRefundRefuseForm 转发测试 Merchant 拒绝表单请求。
func (transport testOrdersTransport) MerchantRefundRefuseForm(ctx context.Context, request orderapp.MerchantRefundRefuseFormRequest) (orderapp.MerchantRefundRefuseFormResult, error) {
	return transport.services.RefundDetail.MerchantRefundRefuseForm(ctx, request)
}

// RefuseMerchantRefund 转发测试 Merchant 拒绝提交请求。
func (transport testOrdersTransport) RefuseMerchantRefund(ctx context.Context, request orderapp.MerchantRefundRefuseRequest) (orderapp.RefundActionResult, error) {
	return transport.services.RefundDetail.RefuseMerchantRefund(ctx, request)
}

// CloseOrderReasons 转发测试订单的只读动态关闭原因请求。
func (transport testOrdersTransport) CloseOrderReasons(ctx context.Context, request orderapp.CloseOrderReasonsRequest) (orderapp.CloseOrderReasonsResult, error) {
	return transport.services.CloseOrder.Reasons(ctx, request)
}

// CloseOrderEligibility 转发测试订单的纯本地关单资格查询。
func (transport testOrdersTransport) CloseOrderEligibility(ctx context.Context, request orderapp.CloseOrderEligibilityRequest) (orderapp.CloseOrderEligibilityResult, error) {
	return transport.services.CloseOrder.Eligibility(ctx, request)
}

// CloseOrder 转发测试订单的真实卖家关单应用请求；平台依赖由测试替身隔离。
func (transport testOrdersTransport) CloseOrder(ctx context.Context, request orderapp.CloseOrderRequest) (orderapp.CloseOrderResult, error) {
	return transport.services.CloseOrder.Close(ctx, request)
}

// newTestPlatformPort 将生产平台 Port 包装为可替换的测试 Port。
func newTestPlatformPort(platform *adapter.PlatformDependencies) *testPlatformPort {
	if platform == nil {
		return &testPlatformPort{}
	}
	return &testPlatformPort{mtop: platform.MTOPClient(), longLogin: platform.LongLoginClient(), qr: platform.QRLoginService()}
}

// MTOPClient 返回当前测试注入的 MTOP 客户端。
func (port *testPlatformPort) MTOPClient() adapter.MTOPClient {
	port.mu.RLock()
	defer port.mu.RUnlock()
	return port.mtop
}

// LongLoginClient 返回当前测试注入的长登录客户端。
func (port *testPlatformPort) LongLoginClient() adapter.LongLoginClient {
	port.mu.RLock()
	defer port.mu.RUnlock()
	return port.longLogin
}

// QRLoginService 返回当前测试注入的二维码登录客户端。
func (port *testPlatformPort) QRLoginService() adapter.QRLoginService {
	port.mu.RLock()
	defer port.mu.RUnlock()
	return port.qr
}

// GenerateQRCode 通过当前测试客户端生成二维码，使替身替换在已构造的组合层 Port 中立即生效。
func (port *testPlatformPort) GenerateQRCode(ctx context.Context) (string, string, error) {
	// client 是当前测试用二维码客户端，读取后不持有 Port 锁执行外部调用。
	client := port.QRLoginService()
	if client == nil {
		return "", "", errors.New("测试二维码客户端未设置")
	}
	return client.GenerateQRCode(ctx)
}

// GetSessionStatus 通过当前测试客户端读取二维码会话状态，使状态替身保持可替换。
func (port *testPlatformPort) GetSessionStatus(sessionID string) map[string]any {
	// client 是当前测试用二维码客户端，读取后不持有 Port 锁执行平台查询。
	client := port.QRLoginService()
	if client == nil {
		return nil
	}
	return client.GetSessionStatus(sessionID)
}

// CompleteVerification 通过当前测试客户端完成二维码风控验证，使失败替身能覆盖组合层调用。
func (port *testPlatformPort) CompleteVerification(ctx context.Context, sessionID string) (string, string, error) {
	// client 是当前测试用二维码客户端，读取后不持有 Port 锁执行验证请求。
	client := port.QRLoginService()
	if client == nil {
		return "", "", errors.New("测试二维码客户端未设置")
	}
	return client.CompleteVerification(ctx, sessionID)
}

// mutableTestPlatform 返回测试 Server 装配的可变平台 Port；非测试构造对象不提供替换能力。
func mutableTestPlatform(server *Server) *testPlatformPort {
	// value、ok 分别表示测试组合根登记的平台 Port 及其存在状态。
	value, ok := testServerPlatforms.Load(server)
	if !ok {
		return nil
	}
	// port、ok 分别是登记对象转换出的可变测试平台 Port 及其类型匹配结果。
	port, ok := value.(*testPlatformPort)
	if !ok {
		return nil
	}
	return port
}

// setTestMTop 替换测试 Server 使用的 MTOP 客户端。
func setTestMTop(server *Server, client adapter.MTOPClient) {
	// port 是当前测试 Server 的可变平台 Port；非测试对象保持无副作用。
	if port := mutableTestPlatform(server); port != nil {
		port.mu.Lock()
		port.mtop = client
		port.mu.Unlock()
	}
}

// testMTop 返回测试 Server 当前注入的 MTOP 客户端，供暂存和恢复替身时保持同一 Port。
func testMTop(server *Server) adapter.MTOPClient {
	// port 是当前测试 Server 的可变平台 Port；非测试对象不提供客户端。
	if port := mutableTestPlatform(server); port != nil {
		return port.MTOPClient()
	}
	return nil
}

// setTestCookieRenew 替换测试 Server 使用的长登录客户端。
func setTestCookieRenew(server *Server, client adapter.LongLoginClient) {
	// port 是当前测试 Server 的可变平台 Port；非测试对象保持无副作用。
	if port := mutableTestPlatform(server); port != nil {
		port.mu.Lock()
		port.longLogin = client
		port.mu.Unlock()
	}
}

// setTestQRLogin 替换测试 Server 使用的二维码登录客户端。
func setTestQRLogin(server *Server, client adapter.QRLoginService) {
	// port 是当前测试 Server 的可变平台 Port；非测试对象保持无副作用。
	if port := mutableTestPlatform(server); port != nil {
		port.mu.Lock()
		port.qr = client
		port.mu.Unlock()
	}
}

// testQRLogin 返回测试 Server 当前注入的二维码登录客户端，供测试直接检查替身调用结果。
func testQRLogin(server *Server) adapter.QRLoginService {
	// port 是当前测试 Server 的可变平台 Port；非测试对象不提供客户端。
	if port := mutableTestPlatform(server); port != nil {
		return port.QRLoginService()
	}
	return nil
}

// registerTestAccountManager 登记测试组合根创建的账号管理器，避免为测试向生产 Server 回填字段。
func registerTestAccountManager(server *Server, manager *account.Manager) {
	if server != nil && manager != nil {
		testServerManagers.Store(server, manager)
	}
}

// registerTestChatDomain 登记聊天领域事件中心，供 WebSocket 行为测试发布确定性事件。
func registerTestChatDomain(server *Server, service *chat.Service) {
	if server != nil && service != nil {
		testServerChats.Store(server, service)
	}
}

// registerTestOrderServices 登记生命周期测试所需的订单服务集合。
func registerTestOrderServices(server *Server, services *orderapp.ServiceSet) {
	if server != nil && services != nil {
		testServerOrderServices.Store(server, services)
	}
}

// testOrderServices 返回测试组合根创建的订单服务集合；生产 Server 不提供该访问路径。
func testOrderServices(server *Server) *orderapp.ServiceSet {
	// value、ok 分别是测试组合根登记的订单服务对象及其存在状态。
	value, ok := testServerOrderServices.Load(server)
	if !ok {
		return nil
	}
	// services、ok 分别是类型安全转换后的订单服务集合及其匹配结果。
	services, ok := value.(*orderapp.ServiceSet)
	if !ok {
		return nil
	}
	return services
}

// testChatDomain 返回测试组合根登记的聊天事件中心。
func testChatDomain(server *Server) *chat.Service {
	// value、ok 分别是测试组合根登记的聊天领域对象及其存在状态。
	value, ok := testServerChats.Load(server)
	if !ok {
		return nil
	}
	// service 是转换后的聊天领域服务，类型不匹配时保留 nil 供测试自行失败。
	service, _ := value.(*chat.Service)
	return service
}

// registerTestPlatform 登记测试组合根创建的可变平台 Port，避免将替身回填到生产 Server。
func registerTestPlatform(server *Server, platform *testPlatformPort) {
	if server != nil && platform != nil {
		testServerPlatforms.Store(server, platform)
	}
}

// registerTestBatchLifecycle 从组合层生命周期清单登记批量恢复组件，避免测试反向读取 Server worker。
func registerTestBatchLifecycle(server *Server, components []lifecycleapp.NamedComponent) {
	// component 是按组合层启动顺序遍历的测试生命周期组件。
	for _, component := range components {
		if component.Name == "publish-batch-workers" && component.Component != nil {
			testServerBatchLifecycle.Store(server, testBatchLifecycle{start: component.Component.Start, close: component.Component.Close})
			return
		}
	}
}

// startTestBatchRecovery 启动测试 Server 对应的组合层批量恢复组件。
func startTestBatchRecovery(server *Server, ctx context.Context) error {
	// value、ok 分别是已登记批量恢复组件及其存在状态。
	value, ok := testServerBatchLifecycle.Load(server)
	if !ok {
		return errors.New("测试批量恢复组件未登记")
	}
	// lifecycle、ok 分别是类型安全转换后的批量恢复生命周期及其匹配结果。
	lifecycle, ok := value.(testBatchLifecycle)
	if !ok || lifecycle.start == nil {
		return errors.New("测试批量恢复组件无效")
	}
	return lifecycle.start(ctx)
}

// closeTestBatchRecovery 取消并等待测试 Server 对应的组合层批量恢复组件。
func closeTestBatchRecovery(server *Server, ctx context.Context) error {
	// value、ok 分别是已登记批量恢复组件及其存在状态。
	value, ok := testServerBatchLifecycle.Load(server)
	if !ok {
		return errors.New("测试批量恢复组件未登记")
	}
	// lifecycle、ok 分别是类型安全转换后的批量恢复生命周期及其匹配结果。
	lifecycle, ok := value.(testBatchLifecycle)
	if !ok || lifecycle.close == nil {
		return errors.New("测试批量恢复组件无效")
	}
	return lifecycle.close(ctx)
}

// testAccountManager 返回测试组合根登记的账号管理器；调用方必须在标准测试夹具上使用。
func testAccountManager(server *Server) *account.Manager {
	// manager、ok 分别表示已登记的测试账号管理器及其存在状态。
	manager, ok := testServerManagers.Load(server)
	if !ok {
		return nil
	}
	// typedManager、ok 分别表示映射值的类型断言结果及其有效性。
	typedManager, ok := manager.(*account.Manager)
	if !ok {
		return nil
	}
	return typedManager
}
