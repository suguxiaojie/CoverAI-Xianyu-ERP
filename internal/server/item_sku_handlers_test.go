package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xianyu-go/internal/db"
)

// TestVersionedItemSKUCostIsLocalAndSurvivesPlatformSync 验证统一成本接口只更新本地字段，平台库存同步后仍保留成本。
func TestVersionedItemSKUCostIsLocalAndSurvivesPlatformSync(t *testing.T) {
	// server、store、cleanup 是带完整应用装配的测试服务、数据库和清理函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是测试数据库准备和验证使用的上下文。
	ctx := context.Background()
	// itemErr 是多规格测试商品保存错误。
	if itemErr := store.Items.Upsert(ctx, &db.ItemInfoRow{CookieID: "acc1", ItemID: "item-sku-cost", ItemTitle: "多规格商品", IsMultiSpec: true}); itemErr != nil {
		t.Fatalf("insert item: %v", itemErr)
	}
	// skuRows 是首次平台 SKU 库存快照。
	skuRows := []db.ItemSKURow{{SKUID: "sku-1", PropertiesJSON: `[{"name":"套餐","value":"一倍额度","enabled":true,"status":0,"sort_order":0}]`, PriceCents: 13500, Quantity: 955, InitialQuantity: 1000, Enabled: true, SyncedAt: 100}}
	// syncErr 是首次 SKU 库存快照保存错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "item-sku-cost", skuRows); syncErr != nil {
		t.Fatalf("sync sku: %v", syncErr)
	}
	// router 是待验证的 HTTP 路由。
	router := server.Router()
	// sessionCookie 是当前测试用户的登录会话。
	sessionCookie := loginHelper(t, router)
	// updateRequest 通过统一按钮请求保存 88 元单件成本。
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/v1/items/acc1/item-sku-cost/sku-costs", strings.NewReader(`{"costs":[{"sku_id":"sku-1","cost_cents":8800}]}`))
	updateRequest.AddCookie(sessionCookie)
	// updateRecorder 记录成本保存响应。
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	// refreshedRows 模拟后续平台同步只改变售价、库存和同步时间。
	refreshedRows := []db.ItemSKURow{{SKUID: "sku-1", PropertiesJSON: skuRows[0].PropertiesJSON, PriceCents: 13600, Quantity: 954, InitialQuantity: 1000, Enabled: true, SyncedAt: 200}}
	// syncErr 是平台库存刷新后的 SKU 对账错误。
	if syncErr := store.Items.SyncSKUs(ctx, "acc1", "item-sku-cost", refreshedRows); syncErr != nil {
		t.Fatalf("refresh sku: %v", syncErr)
	}
	// listRequest 读取商品列表，验证具名 SKU DTO 同时返回新库存和旧成本。
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/items?cookie_id=acc1", nil)
	listRequest.AddCookie(sessionCookie)
	// listRecorder 记录商品列表响应。
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	// items 是商品列表接口返回的具名 DTO。
	var items []itemListResponse
	// decodeErr 是商品列表响应解码错误。
	if decodeErr := json.Unmarshal(listRecorder.Body.Bytes(), &items); decodeErr != nil {
		t.Fatalf("decode list: %v", decodeErr)
	}
	if len(items) != 1 || items[0].SKUCount != 1 || len(items[0].SKUs) != 1 {
		t.Fatalf("items=%+v", items)
	}
	// sku 是 API 返回的当前 SKU。
	sku := items[0].SKUs[0]
	if sku.Quantity != 954 || sku.PriceCents != 13600 || sku.CostCents == nil || *sku.CostCents != 8800 || sku.SyncedAt != 200 {
		t.Fatalf("sku=%+v", sku)
	}
}

// TestVersionedSingleItemCostCreatesLocalDefaultSKU 验证无平台 SKU 的单规格商品可保存本地成本且不增加规格数量。
func TestVersionedSingleItemCostCreatesLocalDefaultSKU(t *testing.T) {
	// server、store、cleanup 是带完整应用装配的测试服务、数据库和清理函数。
	server, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 是测试数据准备上下文。
	ctx := context.Background()
	// itemErr 是单规格商品保存错误。
	if itemErr := store.Items.Upsert(ctx, &db.ItemInfoRow{CookieID: "acc1", ItemID: "single-item", ItemTitle: "5x会员月卡老客户专拍", ItemPrice: "¥690"}); itemErr != nil {
		t.Fatalf("insert item: %v", itemErr)
	}
	// router 是待验证的 HTTP 路由。
	router := server.Router()
	// sessionCookie 是当前测试用户登录会话。
	sessionCookie := loginHelper(t, router)
	// request 使用本地隐式 SKU 保存 630 元单件成本。
	request := httptest.NewRequest(http.MethodPut, "/api/v1/items/acc1/single-item/sku-costs", strings.NewReader(`{"costs":[{"sku_id":"__default__","cost_cents":63000}]}`))
	request.AddCookie(sessionCookie)
	// recorder 记录统一成本响应。
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	// listRequest 读取商品 DTO，验证隐式成本行不计入规格数。
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/items?cookie_id=acc1", nil)
	listRequest.AddCookie(sessionCookie)
	// listRecorder 记录商品列表响应。
	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, listRequest)
	// items 是当前账号商品列表 DTO。
	var items []itemListResponse
	// decodeErr 是商品列表响应解码错误。
	if decodeErr := json.Unmarshal(listRecorder.Body.Bytes(), &items); decodeErr != nil {
		t.Fatalf("decode items: %v", decodeErr)
	}
	// single 是按商品标识定位的单规格商品响应。
	var single itemListResponse
	// item 表示当前待匹配的商品 DTO。
	for _, item := range items {
		if item.ItemID == "single-item" {
			single = item
		}
	}
	if single.SKUCount != 0 || len(single.SKUs) != 1 || !single.SKUs[0].LocalOnly || single.SKUs[0].CostCents == nil || *single.SKUs[0].CostCents != 63000 {
		t.Fatalf("single=%+v", single)
	}
}

// TestVersionedItemSKUCostRejectsNegativeAndMissingSKU 验证非法成本和不存在 SKU 使用稳定 HTTP 状态。
func TestVersionedItemSKUCostRejectsNegativeAndMissingSKU(t *testing.T) {
	// server、cleanup 是待验证服务和清理函数。
	server, _, cleanup := newTestServer(t)
	defer cleanup()
	// router、sessionCookie 是当前测试路由和登录会话。
	router := server.Router()
	// sessionCookie 是当前测试用户的登录会话。
	sessionCookie := loginHelper(t, router)
	// cases 保存请求体、目标 SKU 和期望状态。
	cases := []struct {
		// name 是子测试名称。
		name string
		// skuID 是 URL 中的 SKU 标识。
		skuID string
		// body 是成本请求 JSON。
		body string
		// wantStatus 是期望 HTTP 状态。
		wantStatus int
	}{
		{name: "negative", skuID: "sku-1", body: `{"cost_cents":-1}`, wantStatus: http.StatusBadRequest},
		{name: "missing", skuID: "missing", body: `{"cost_cents":100}`, wantStatus: http.StatusNotFound},
	}
	// testCase 表示当前非法请求场景。
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// request 是当前成本更新请求。
			request := httptest.NewRequest(http.MethodPut, "/api/v1/items/acc1/item-1/skus/"+testCase.skuID+"/cost", strings.NewReader(testCase.body))
			request.AddCookie(sessionCookie)
			// recorder 记录当前非法请求响应。
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
		})
	}
}
