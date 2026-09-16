package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	itemapp "xianyu-go/internal/application/items"
)

// itemSKUPropertyResponse 是 SKU 销售属性的具名 HTTP DTO。
type itemSKUPropertyResponse struct {
	// Name 是规格名称，例如“套餐”。
	Name string `json:"name"`
	// Value 是规格值，例如“一倍额度”。
	Value string `json:"value"`
	// Enabled 表示规格值当前是否可选。
	Enabled bool `json:"enabled"`
	// Status 是平台规格状态码。
	Status int `json:"status"`
	// SortOrder 是规格值展示顺序。
	SortOrder int `json:"sort_order"`
}

// itemSKUResponse 是商品 SKU、平台库存与本地成本的具名 HTTP DTO。
type itemSKUResponse struct {
	// SKUID 是平台 SKU 标识，始终按字符串输出防止精度损失。
	SKUID string `json:"sku_id"`
	// InventoryID 是平台库存标识，始终按字符串输出防止精度损失。
	InventoryID string `json:"inventory_id"`
	// Properties 是组成当前 SKU 的销售属性。
	Properties []itemSKUPropertyResponse `json:"properties"`
	// PriceCents 是平台售价，单位为人民币分。
	PriceCents int64 `json:"price_cents"`
	// Quantity 是卖家编辑详情返回的当前剩余库存。
	Quantity int `json:"quantity"`
	// InitialQuantity 是发布时设置的初始库存。
	InitialQuantity int `json:"initial_quantity"`
	// Enabled 表示 SKU 当前是否在售可选。
	Enabled bool `json:"enabled"`
	// SortOrder 是平台 SKU 展示顺序。
	SortOrder int `json:"sort_order"`
	// CostCents 是仅在本地保存的单件成本；nil 在 JSON 中输出 null。
	CostCents *int64 `json:"cost_cents"`
	// SyncedAt 是库存快照同步时间，Unix 秒。
	SyncedAt int64 `json:"synced_at"`
	// LocalOnly 表示该行是单规格商品的本地成本载体，不是平台 SKU。
	LocalOnly bool `json:"local_only"`
}

// itemSKUCostRequest 是更新本地 SKU 单件成本的请求 DTO；null 表示清除成本。
type itemSKUCostRequest struct {
	// CostCents 是可空的人民币分值，平台同步永远不会修改该字段。
	CostCents *int64 `json:"cost_cents"`
}

// itemSKUCostEntryRequest 是统一成本保存请求中的单个 SKU 成本。
type itemSKUCostEntryRequest struct {
	// SKUID 是平台 SKU 或单规格商品的本地隐式 SKU 标识。
	SKUID string `json:"sku_id"`
	// CostCents 是可空人民币分值；null 表示清除。
	CostCents *int64 `json:"cost_cents"`
}

// itemSKUCostsRequest 是右下角统一保存按钮提交的全部 SKU 成本。
type itemSKUCostsRequest struct {
	// Costs 是同一商品需要原子保存的成本集合。
	Costs []itemSKUCostEntryRequest `json:"costs"`
}

// itemSKUsToResponse 将应用 SKU 模型转换为稳定 HTTP DTO。
func itemSKUsToResponse(skus []itemapp.CatalogSKU) []itemSKUResponse {
	// result 保存按平台顺序输出的 SKU。
	result := make([]itemSKUResponse, 0, len(skus))
	// sku 表示当前待转换的应用 SKU。
	for _, sku := range skus {
		// properties 保存当前 SKU 的规格名称和值。
		properties := make([]itemSKUPropertyResponse, 0, len(sku.Properties))
		// property 表示当前待转换的规格属性。
		for _, property := range sku.Properties {
			properties = append(properties, itemSKUPropertyResponse{Name: property.Name, Value: property.Value, Enabled: property.Enabled, Status: property.Status, SortOrder: property.SortOrder})
		}
		result = append(result, itemSKUResponse{
			SKUID: sku.SKUID, InventoryID: sku.InventoryID, Properties: properties,
			PriceCents: sku.PriceCents, Quantity: sku.Quantity, InitialQuantity: sku.InitialQuantity,
			Enabled: sku.Enabled, SortOrder: sku.SortOrder, CostCents: sku.CostCents, SyncedAt: sku.SyncedAt, LocalOnly: sku.LocalOnly,
		})
	}
	return result
}

// itemSKUCount 返回真实平台 SKU 数量，单规格商品的本地隐式成本行不计入规格数。
func itemSKUCount(skus []itemapp.CatalogSKU) int {
	// count 是当前真实平台 SKU 数量。
	count := 0
	// sku 表示当前待检查的商品 SKU。
	for _, sku := range skus {
		if !sku.LocalOnly {
			count++
		}
	}
	return count
}

// updateItemSKUCost 校验账号归属并仅更新本地 SKU 单件成本，不调用闲鱼平台。
func (s *Server) updateItemSKUCost(writer http.ResponseWriter, request *http.Request) {
	// cookieID 是待更新 SKU 所属账号。
	cookieID := chi.URLParam(request, "cookie_id")
	if !s.requireCookieOwnership(writer, request, cookieID) {
		return
	}
	// itemID、skuID 是 URL 中的商品和 SKU 稳定标识。
	itemID, skuID := chi.URLParam(request, "item_id"), chi.URLParam(request, "sku_id")
	// body 保存解码后的可空成本分值。
	var body itemSKUCostRequest
	// decodeErr 是成本请求 JSON 解码错误。
	if decodeErr := decodeJSON(request, &body); decodeErr != nil {
		writeErr(writer, http.StatusBadRequest, "请求格式错误")
		return
	}
	if body.CostCents != nil && *body.CostCents < 0 {
		writeErr(writer, http.StatusBadRequest, "SKU 成本不能小于零")
		return
	}
	// updateErr 是商品应用服务的本地成本更新结果。
	updateErr := s.itemCatalogMutationApplication().UpdateSKUCost(request.Context(), cookieID, itemID, skuID, body.CostCents)
	if errors.Is(updateErr, itemapp.ErrCatalogNotFound) {
		writeErr(writer, http.StatusNotFound, "商品 SKU 不存在")
		return
	}
	if updateErr != nil {
		writeErr(writer, http.StatusInternalServerError, "保存 SKU 成本失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true})
}

// updateItemSKUCosts 校验账号归属并在一个本地事务中保存全部 SKU 成本。
func (s *Server) updateItemSKUCosts(writer http.ResponseWriter, request *http.Request) {
	// cookieID 是待保存成本商品所属账号。
	cookieID := chi.URLParam(request, "cookie_id")
	if !s.requireCookieOwnership(writer, request, cookieID) {
		return
	}
	// itemID 是待保存成本的平台商品标识。
	itemID := chi.URLParam(request, "item_id")
	// body 保存统一成本请求。
	var body itemSKUCostsRequest
	// decodeErr 是统一成本请求 JSON 解码错误。
	if decodeErr := decodeJSON(request, &body); decodeErr != nil || len(body.Costs) == 0 || len(body.Costs) > 100 {
		writeErr(writer, http.StatusBadRequest, "成本请求格式错误")
		return
	}
	// costs 是 HTTP DTO 转换后的应用成本集合。
	costs := make([]itemapp.SKUCostInput, 0, len(body.Costs))
	// cost 表示当前待转换的请求成本。
	for _, cost := range body.Costs {
		if cost.CostCents != nil && *cost.CostCents < 0 {
			writeErr(writer, http.StatusBadRequest, "SKU 成本不能小于零")
			return
		}
		costs = append(costs, itemapp.SKUCostInput{SKUID: cost.SKUID, CostCents: cost.CostCents})
	}
	// updateErr 是应用服务原子保存全部成本的结果。
	updateErr := s.itemCatalogMutationApplication().UpdateSKUCosts(request.Context(), cookieID, itemID, costs)
	if errors.Is(updateErr, itemapp.ErrCatalogNotFound) {
		writeErr(writer, http.StatusNotFound, "商品或 SKU 不存在")
		return
	}
	if updateErr != nil {
		writeErr(writer, http.StatusBadRequest, updateErr.Error())
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true})
}
