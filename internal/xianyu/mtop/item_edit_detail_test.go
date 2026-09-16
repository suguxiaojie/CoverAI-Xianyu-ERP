package mtop

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestFetchItemEditDetailParsesCurrentInventory 验证卖家编辑详情区分当前剩余库存和发布时初始库存，并保留长整型库存标识。
func TestFetchItemEditDetailParsesCurrentInventory(t *testing.T) {
	// server 是模拟闲鱼编辑详情端点的本地 HTTP 服务。
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("api") != "mtop.idle.pc.idleitem.editDetail" {
			t.Fatalf("api=%q", request.URL.Query().Get("api"))
		}
		// rawBody 是本次表单请求正文。
		rawBody, _ := io.ReadAll(request.Body)
		// form 是解码后的表单字段。
		form, _ := url.ParseQuery(string(rawBody))
		if form.Get("data") != `{"itemId":"item-1"}` {
			t.Fatalf("data=%q", form.Get("data"))
		}
		_, _ = io.WriteString(writer, `{"ret":["SUCCESS::调用成功"],"data":{"quantity":"2949","itemSkuList":[`+
			`{"skuId":"6286771311126","inventoryId":"1114713017577732954","priceInCent":"13500","quantity":"955","features":{"idleOriginalQuantity":"1000","sku_order_num":"0"},"propertyList":[{"propertyText":"套餐","actualValueText":"一倍额度","enabled":true,"status":"0","valueSortOrder":"0"}]},`+
			`{"skuId":"6286771311127","inventoryId":"1114713017577732955","priceInCent":"69000","quantity":"997","features":{"idleOriginalQuantity":"1000","sku_order_num":"1"},"propertyList":[{"propertyText":"套餐","valueText":"五倍额度","enabled":true,"status":"0","valueSortOrder":"1"}]}`+
			`]}}`)
	}))
	defer server.Close()
	// client 使用本地端点验证请求和解析，不连接真实闲鱼。
	client := &ClientImpl{HTTPClient: server.Client(), ItemEditDetailURL: server.URL}
	// result、fetchErr 是编辑详情解析结果和调用错误。
	result, fetchErr := client.FetchItemEditDetail(context.Background(), "_m_h5_tk=token_1", "item-1")
	if fetchErr != nil {
		t.Fatalf("FetchItemEditDetail: %v", fetchErr)
	}
	if result.TotalQuantity != 2949 || len(result.SKUs) != 2 {
		t.Fatalf("result=%+v", result)
	}
	// first 是需要验证长整型标识、成本无关库存与规格文本的首个 SKU。
	first := result.SKUs[0]
	if first.SKUID != "6286771311126" || first.InventoryID != "1114713017577732954" || first.PriceCents != 13500 || first.Quantity != 955 || first.InitialQuantity != 1000 || !first.Enabled || first.SortOrder != 0 {
		t.Fatalf("first=%+v", first)
	}
	if len(first.Properties) != 1 || first.Properties[0].Name != "套餐" || first.Properties[0].Value != "一倍额度" {
		t.Fatalf("properties=%+v", first.Properties)
	}
}

// TestParseItemEditDetailSkipsMissingSKUID 验证无平台 SKU 标识的异常行不会进入本地库存对账。
func TestParseItemEditDetailSkipsMissingSKUID(t *testing.T) {
	// result 是包含一个异常行的解析结果。
	result := parseItemEditDetail(map[string]any{"itemSkuList": []any{map[string]any{"quantity": "7"}}})
	if len(result.SKUs) != 0 {
		t.Fatalf("skus=%+v", result.SKUs)
	}
}
