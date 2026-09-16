package server

import (
	"github.com/go-chi/chi/v5"

	"xianyu-go/internal/auth"
)

// mountVersionedOrders 挂载订单列表、详情和更新的 `/api/v1` 兼容入口。
func (s *Server) mountVersionedOrders(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.Auth.Middleware)
		r.Use(auth.RequireAuth)
		r.Get("/api/v1/orders", s.listOrders)
		r.Get("/api/v1/orders/conversation-context", s.conversationOrderContext)
		r.Get("/api/v1/orders/{order_id}", s.getOrder)
		r.Put("/api/v1/orders/{order_id}", s.updateOrder)
		r.Delete("/api/v1/orders/{order_id}", s.deleteOrder)
		s.mountOrderRefreshJobRoutes(r, "/api/v1")
		r.Post("/api/v1/orders/manual-ship", s.manualShipOrders)
		r.Post("/api/v1/orders/{order_id}/ship-with-evidence", s.shipOrderWithEvidence)
		r.Get("/api/v1/orders/{order_id}/shipment-proof", s.getOrderShipmentProof)
		r.Post("/api/v1/orders/import", s.importOrders)
		r.Post("/api/v1/orders/{order_id}/refresh", s.refreshSingleOrder)
		r.Post("/api/v1/orders/{order_id}/request-red-flower", s.requestOrderRedFlower)
		r.Get("/api/v1/orders/{order_id}/request-red-flower", s.getOrderRedFlowerStatus)
		r.Get("/api/v1/orders/{order_id}/adjust-price", s.getOrderAdjustPriceForm)
		r.Post("/api/v1/orders/{order_id}/adjust-price", s.adjustOrderPrice)
		r.Get("/api/v1/orders/{order_id}/refund-detail", s.getOrderRefundDetail)
		r.Post("/api/v1/orders/{order_id}/refund-detail", s.submitOrderRefundAction)
		r.Post("/api/v1/orders/{order_id}/refund-verification", s.startMerchantRefundVerification)
		r.Post("/api/v1/orders/{order_id}/refund-verification/{session_id}/complete", s.completeMerchantRefundVerification)
		r.Get("/api/v1/orders/{order_id}/refund-refuse", s.getMerchantRefundRefuseForm)
		r.Post("/api/v1/orders/{order_id}/refund-refuse", s.refuseMerchantRefund)
		r.Get("/api/v1/orders/{order_id}/close-eligibility", s.getOrderCloseEligibility)
		r.Get("/api/v1/orders/{order_id}/close", s.getOrderCloseReasons)
		r.Post("/api/v1/orders/{order_id}/close", s.closeOrderBySeller)
	})
}
