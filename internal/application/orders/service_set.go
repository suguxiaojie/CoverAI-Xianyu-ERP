package orders

// ServiceSet 聚合订单领域的应用服务实例，由应用层统一负责构造。
// HTTP/Server 适配层只持有该集合，不再分别创建业务服务实现。
type ServiceSet struct {
	// List 负责订单列表分页和筛选用例。
	List *ListService
	// ConversationContext 负责 Chat 历史订单精确关联和只读分页。
	ConversationContext *ConversationContextService
	// Detail 负责订单详情读取和商品补全用例。
	Detail *DetailService
	// Delete 负责订单逻辑删除用例。
	Delete *DeleteService
	// Update 负责订单字段与商品标题更新用例。
	Update *UpdateService
	// Import 负责订单文件导入用例。
	Import *ImportService
	// ManualShip 负责手动发货和补偿用例。
	ManualShip *ManualShipService
	// ShipmentEvidence 负责聊天付款卡片的凭证上传和无需寄件用例。
	ShipmentEvidence *ShipmentEvidenceService
	// RequestRedFlower 负责人工确认后的订单求花和幂等收口。
	RequestRedFlower *RedFlowerRequestService
	// AdjustPrice 负责待付款卡片动态表单和真实改价幂等收口。
	AdjustPrice *PriceAdjustmentService
	// CloseOrder 负责待付款及已付款待发货订单的动态原因和卖家取消幂等收口。
	CloseOrder *CloseOrderService
	// RefundDetail 负责退款申请卡片的官方只读详情查询。
	RefundDetail *RefundDetailService
	// Refresh 负责订单发现、详情刷新和缺失清理用例。
	Refresh *RefreshService
	// RefreshJobs 负责订单刷新后台任务的持久化操作。
	RefreshJobs RefreshJobRepository
}

// NewServiceSet 使用应用层 Port 构造完整订单服务集合。
func NewServiceSet(repository Repository, refreshRepository RefreshRepository, redFlowerRepository RedFlowerRequestRepository, priceRepository PriceAdjustmentRepository, closeRepository CloseOrderRepository, shipmentRepository ShipmentEvidenceRepository, refundRepository RefundDetailRepository, manualRuntime ManualShipRuntime, refreshRuntime RefreshRuntime, redFlowerRuntime RedFlowerRequestRuntime, priceRuntime PriceAdjustmentRuntime, closeRuntime CloseOrderRuntime, shipmentRuntime ShipmentEvidenceRuntime, refundRuntime RefundDetailRuntime, refreshJobs RefreshJobRepository, detailChunkSize int) *ServiceSet {
	return &ServiceSet{
		List:                NewListService(repository),
		ConversationContext: NewConversationContextService(repository),
		Detail:              NewDetailService(repository),
		Delete:              NewDeleteService(repository),
		Update:              NewUpdateService(repository),
		Import:              NewImportService(repository),
		ManualShip:          NewManualShipService(repository, manualRuntime),
		ShipmentEvidence:    NewShipmentEvidenceService(shipmentRepository, shipmentRuntime, nil),
		RequestRedFlower:    NewRedFlowerRequestService(redFlowerRepository, redFlowerRuntime, RedFlowerRequestOptions{}),
		AdjustPrice:         NewPriceAdjustmentService(priceRepository, priceRuntime, nil),
		CloseOrder:          NewCloseOrderService(closeRepository, closeRuntime, nil),
		RefundDetail:        NewRefundDetailService(refundRepository, refundRuntime),
		Refresh:             NewRefreshService(refreshRepository, refreshRuntime, detailChunkSize),
		RefreshJobs:         refreshJobs,
	}
}
