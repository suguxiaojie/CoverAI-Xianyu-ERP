// Package mtop: 卖家商品编辑详情域 — 读取 SKU 当前剩余库存，不执行商品编辑或发布。
package mtop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xianyu-go/internal/xianyu/protocol"
)

// ItemEditDetailAPI 是闲鱼网页卖家编辑页用于回填 SKU 当前库存的只读端点。
const ItemEditDetailAPI = "https://h5api.m.goofish.com/h5/mtop.idle.pc.idleitem.editdetail/1.0/"

// ItemEditDetailFetcher 是商品同步使用的卖家编辑详情能力；实现只允许读取，不提交编辑表单。
type ItemEditDetailFetcher interface {
	// FetchItemEditDetail 使用商品所有者 Cookie 读取当前 SKU、价格和剩余库存。
	FetchItemEditDetail(context.Context, string, string) (*ItemEditDetailResult, error)
}

// ItemEditDetailResult 是卖家商品编辑详情中的库存快照。
type ItemEditDetailResult struct {
	// TotalQuantity 是平台当前返回的全部 SKU 剩余库存合计。
	TotalQuantity int
	// SyncedAt 是本次只读平台响应成功解析的 Unix 秒时间。
	SyncedAt int64
	// SKUs 是平台按商品规格顺序返回的 SKU 列表。
	SKUs []ItemSKU
}

// ItemSKU 是平台 SKU 的非敏感库存快照；所有长整型标识都使用字符串避免 JSON 精度损失。
type ItemSKU struct {
	// SKUID 是平台 SKU 标识。
	SKUID string
	// InventoryID 是平台库存标识，可能超过 JavaScript 安全整数范围。
	InventoryID string
	// Properties 是组成当前 SKU 的一至两个销售属性。
	Properties []ItemSKUProperty
	// PriceCents 是 SKU 售价，单位为人民币分。
	PriceCents int64
	// Quantity 是卖家编辑页返回的当前剩余库存。
	Quantity int
	// InitialQuantity 是发布时设置的原始库存。
	InitialQuantity int
	// Enabled 表示 SKU 的所有销售属性当前均可用。
	Enabled bool
	// SortOrder 是平台规格顺序，用于稳定展示。
	SortOrder int
}

// ItemSKUProperty 是 SKU 的单个销售属性和值。
type ItemSKUProperty struct {
	// Name 是规格名称，例如“套餐”。
	Name string `json:"name"`
	// Value 是规格值，例如“一倍额度”。
	Value string `json:"value"`
	// Enabled 表示该规格值当前是否可选。
	Enabled bool `json:"enabled"`
	// Status 是平台返回的规格状态码。
	Status int `json:"status"`
	// SortOrder 是规格值在同一属性下的展示顺序。
	SortOrder int `json:"sort_order"`
}

var _ ItemEditDetailFetcher = (*ClientImpl)(nil)

// FetchItemEditDetail 调用卖家编辑详情接口并解析当前 SKU 库存；它不会发送保存或发布请求。
func (c *ClientImpl) FetchItemEditDetail(ctx context.Context, cookies, itemID string) (*ItemEditDetailResult, error) {
	// normalizedItemID 是去除输入空白后的平台商品标识。
	normalizedItemID := strings.TrimSpace(itemID)
	if normalizedItemID == "" {
		return nil, fmt.Errorf("item_id 不能为空")
	}
	// endpoint 是生产端点或测试覆盖地址。
	endpoint := c.ItemEditDetailURL
	if endpoint == "" {
		endpoint = ItemEditDetailAPI
	}
	// documentURL 是签名 Cookie 作用域和卖家编辑页 Referer。
	documentURL := "https://www.goofish.com/publish?itemId=" + url.QueryEscape(normalizedItemID)
	// signingCookies、requestCookies 分别用于 MTOP 签名和真实请求头。
	signingCookies, requestCookies := mtopRequestCookies(ctx, cookies, documentURL, endpoint)
	// token 是 H5 MTOP 签名 token；缺失时不能伪造无签名请求。
	token := protocol.SignToken(signingCookies)
	if token == "" {
		return nil, fmt.Errorf("cookie 缺少 _m_h5_tk，无法获取卖家商品编辑详情")
	}
	// dataValue 是平台接口要求的最小商品查询参数。
	dataValue := `{"itemId":` + strconv.Quote(normalizedItemID) + `}`
	// timestamp 是本次签名使用的 Unix 毫秒时间文本。
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	// sign 是基于时间、token 和业务参数生成的 MTOP 签名。
	sign := protocol.GenerateSign(timestamp, token, dataValue)
	// request 是只读的卖家商品编辑详情请求。
	request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+buildItemEditDetailQuery(timestamp, sign), strings.NewReader("data="+url.QueryEscape(dataValue)))
	if requestErr != nil {
		return nil, requestErr
	}
	setCommonHeaders(request, requestCookies)
	request.Header.Set("Origin", "https://www.goofish.com")
	request.Header.Set("Referer", documentURL)
	// response 是平台返回的编辑详情响应。
	response, callErr := c.httpClient().Do(request)
	if callErr != nil {
		return nil, fmt.Errorf("卖家商品编辑详情请求失败: %w", callErr)
	}
	defer response.Body.Close()
	absorbMTopResponseCookies(ctx, cookies, response)
	// rawBody 是经过大小限制读取的响应正文。
	rawBody, readErr := readMTopBody(response)
	if readErr != nil {
		return nil, readErr
	}
	// decoded 保存启用 json.Number 后的 MTOP 响应，避免长整型标识先转 float64。
	var decoded struct {
		// Ret 是平台返回码列表。
		Ret []string `json:"ret"`
		// Data 是商品编辑详情原始对象。
		Data map[string]any `json:"data"`
	}
	// decoder 使用 UseNumber 保留 SKU 和库存标识的十进制文本。
	decoder := json.NewDecoder(bytes.NewReader(rawBody))
	decoder.UseNumber()
	// decodeErr 是卖家编辑详情 JSON 解码错误。
	if decodeErr := decoder.Decode(&decoded); decodeErr != nil {
		return nil, fmt.Errorf("解析卖家商品编辑详情失败: %w (body=%s)", decodeErr, truncate(string(rawBody), 300))
	}
	if isSessionExpiredRet(decoded.Ret) {
		return nil, sessionExpiredError("卖家商品编辑详情接口", decoded.Ret)
	}
	if !hasMTopSuccess(decoded.Ret) {
		return nil, fmt.Errorf("卖家商品编辑详情接口返回非成功: ret=%v", decoded.Ret)
	}
	// result 是已经移除用户、凭证和无关商品字段的 SKU 快照。
	result := parseItemEditDetail(decoded.Data)
	result.SyncedAt = time.Now().Unix()
	return result, nil
}

// buildItemEditDetailQuery 构造与闲鱼卖家网页一致的 MTOP 查询参数。
func buildItemEditDetailQuery(timestamp, sign string) string {
	// values 保存签名外的 MTOP 固定协议参数。
	values := url.Values{
		"jsv": {"2.7.2"}, "appKey": {protocol.SignAppKey}, "t": {timestamp}, "sign": {sign},
		"v": {"1.0"}, "type": {"originaljson"}, "accountSite": {"xianyu"}, "dataType": {"json"},
		"timeout": {"20000"}, "api": {"mtop.idle.pc.idleitem.editDetail"},
		"sessionOption": {"AutoLoginOnly"}, "spm_cnt": {"a21ybx.publish.0.0"},
	}
	return values.Encode()
}

// parseItemEditDetail 将卖家编辑详情转换为不含用户和凭证信息的 SKU 库存快照。
func parseItemEditDetail(data map[string]any) *ItemEditDetailResult {
	// rawSKUs 是平台当前返回的 SKU 对象列表。
	rawSKUs, _ := data["itemSkuList"].([]any)
	// result 保存总库存和按平台顺序解析的 SKU。
	result := &ItemEditDetailResult{TotalQuantity: mtopInt(data["quantity"]), SKUs: make([]ItemSKU, 0, len(rawSKUs))}
	// rawSKU 表示当前待解析的 SKU 对象。
	for _, rawSKU := range rawSKUs {
		// sku、valid 表示当前值是否为可解析的 JSON 对象。
		sku, valid := rawSKU.(map[string]any)
		if !valid {
			continue
		}
		// skuID 是当前 SKU 的稳定平台标识；缺失行不能参与本地对账。
		skuID := strings.TrimSpace(mtopString(sku["skuId"]))
		if skuID == "" {
			continue
		}
		// features 保存平台原始库存和排序扩展字段。
		features, _ := sku["features"].(map[string]any)
		// rawProperties 是当前 SKU 的销售属性列表。
		rawProperties, _ := sku["propertyList"].([]any)
		// properties 保存规范化后的规格名称和值。
		properties := make([]ItemSKUProperty, 0, len(rawProperties))
		// enabled 表示当前 SKU 的全部规格值均可用。
		enabled := true
		// rawProperty 表示当前待解析的销售属性。
		for _, rawProperty := range rawProperties {
			// property、propertyValid 表示当前属性是否为 JSON 对象。
			property, propertyValid := rawProperty.(map[string]any)
			if !propertyValid {
				continue
			}
			// propertyEnabled 是平台返回的单个规格值启用状态；字段缺失时按可用处理。
			propertyEnabled := true
			// exists 表示平台是否显式返回规格值启用状态。
			if _, exists := property["enabled"]; exists {
				propertyEnabled = mtopBool(property["enabled"])
			}
			enabled = enabled && propertyEnabled
			properties = append(properties, ItemSKUProperty{
				Name:    strings.TrimSpace(mtopString(property["propertyText"])),
				Value:   strings.TrimSpace(firstMTopString(property, "actualValueText", "valueText")),
				Enabled: propertyEnabled, Status: mtopInt(property["status"]), SortOrder: mtopInt(property["valueSortOrder"]),
			})
		}
		// priceCents 是平台返回的 SKU 售价分值。
		priceCents, _ := strconv.ParseInt(mtopString(sku["priceInCent"]), 10, 64)
		result.SKUs = append(result.SKUs, ItemSKU{
			SKUID: skuID, InventoryID: strings.TrimSpace(mtopString(sku["inventoryId"])), Properties: properties,
			PriceCents: priceCents, Quantity: mtopInt(sku["quantity"]), InitialQuantity: mtopInt(features["idleOriginalQuantity"]),
			Enabled: enabled, SortOrder: mtopInt(features["sku_order_num"]),
		})
	}
	return result
}
