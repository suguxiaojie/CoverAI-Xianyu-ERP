package mtop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestFetchSoldOrdersPageRequestAndParse 封装TestFetchSold订单列表页码请求AndParse业务协调。
func TestFetchSoldOrdersPageRequestAndParse(t *testing.T) {
	// server 用于本次流程后续判断的server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api") != "mtop.taobao.idle.trade.merchant.sold.get" || r.URL.Query().Get("sign") == "" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		if r.Header.Get("Origin") != "https://seller.goofish.com" || r.Header.Get("idle_site_biz_code") != "COMMONPRO" {
			t.Errorf("headers=%v", r.Header)
		}
		// rawBody 用于本次流程后续判断的原始请求体
		rawBody, _ := io.ReadAll(r.Body)
		// form 用于本次流程后续判断的表单
		form, _ := url.ParseQuery(string(rawBody))
		// payload 用于本次流程后续判断的请求载荷
		var payload map[string]any
		if // err 用于本次流程后续判断的err
		err := json.Unmarshal([]byte(form.Get("data")), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["pageNumber"] != float64(2) || payload["rowsPerPage"] != float64(30) || payload["queryCode"] != "ALL" {
			t.Errorf("payload=%+v", payload)
		}
		_, _ = io.WriteString(w, `{"ret":["SUCCESS::调用成功"],"data":{"module":{"nextPage":"true","totalCount":"31","items":[{`+
			`"commonData":{"orderId":"order-1","itemId":"item-1","orderStatus":"待发货","inRefund":"false","createTime":"2026-08-01 12:34:56"},`+
			`"buyerInfoVO":{"buyerId":"buyer-1","name":"李四","phone":"13900000000","address":"杭州市"},`+
			`"priceVO":{"totalPrice":"29.90","buyNum":"3"},"rightVO":{"btnList":[{"tradeAction":"SKIP_PIN"}]}}]}}}`)
	}))
	defer server.Close()

	// client 用于本次流程后续判断的client
	client := &ClientImpl{HTTPClient: server.Client(), SoldOrdersURL: server.URL}
	// page、err 用于本次流程后续判断的page、err
	page, err := client.FetchSoldOrdersPage(context.Background(), "unb=1; _m_h5_tk=token_1;", 2, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !page.NextPage || page.TotalCount != 31 || len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
	// item 用于本次流程后续判断的商品
	item := page.Items[0]
	if item.OrderID != "order-1" || item.ItemID != "item-1" || item.OrderStatus != "pending_ship" ||
		item.Quantity != "3" || item.Amount != "29.90" || !item.IsBargain || item.ReceiverName != "李四" || item.CreatedAt != "2026-08-01T04:34:56Z" {
		t.Fatalf("item=%+v", item)
	}
}

// TestNormalizeSoldOrderCreatedAt 验证平台北京时间、秒／毫秒时间戳和非法文本的稳定归一化。
func TestNormalizeSoldOrderCreatedAt(t *testing.T) {
	// cases 保存输入值到预期 UTC RFC3339 的映射。
	cases := map[any]string{
		"2026-08-01 12:34:56":  "2026-08-01T04:34:56Z",
		"2026/08/01 12:34":     "2026-08-01T04:34:00Z",
		"1754022896":           "2025-08-01T04:34:56Z",
		float64(1754022896000): "2025-08-01T04:34:56Z",
		"not-a-time":           "",
	}
	// input、expected 是当前待验证的平台时间值和规范结果。
	for input, expected := range cases {
		// actual 是当前输入规范化后的 UTC RFC3339 结果。
		if actual := normalizeSoldOrderCreatedAt(input); actual != expected {
			t.Fatalf("input=%v actual=%q expected=%q", input, actual, expected)
		}
	}
}

// TestFetchSoldOrdersPageRejectsMissingTokenAndFailure 封装TestFetchSold订单列表页码RejectsMissing令牌AndFailure业务协调。
func TestFetchSoldOrdersPageRejectsMissingTokenAndFailure(t *testing.T) {
	// client 用于本次流程后续判断的client
	client := &ClientImpl{}
	if // err 用于本次流程后续判断的err
	_, err := client.FetchSoldOrdersPage(context.Background(), "unb=1", 1, 30); err == nil || !strings.Contains(err.Error(), "_m_h5_tk") {
		t.Fatalf("err=%v", err)
	}
	// server 用于本次流程后续判断的server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ret":["FAIL_BIZ_ERROR::失败"]}`)
	}))
	defer server.Close()
	client = &ClientImpl{HTTPClient: server.Client(), SoldOrdersURL: server.URL}
	if // err 用于本次流程后续判断的err
	_, err := client.FetchSoldOrdersPage(context.Background(), "_m_h5_tk=token_1", 1, 30); err == nil || !strings.Contains(err.Error(), "非成功") {
		t.Fatalf("err=%v", err)
	}
}

// TestFetchSoldOrdersPageUsesIndependentTimeout 验证平台不响应时单页发现不会占满后台任务总预算。
func TestFetchSoldOrdersPageUsesIndependentTimeout(t *testing.T) {
	// previousTimeout 保存生产默认值，测试结束后恢复，避免影响同包其他用例。
	previousTimeout := soldOrdersPageTimeout
	soldOrdersPageTimeout = 10 * time.Millisecond
	defer func() { soldOrdersPageTimeout = previousTimeout }() // timeoutRestore 恢复订单发现默认超时。
	// client 使用等待请求上下文结束的传输层，不访问真实平台。
	client := &ClientImpl{HTTPClient: &http.Client{Transport: cookieSessionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}}
	// started 是单页请求开始时间，用于拒绝长时间悬挂。
	started := time.Now()
	// _, fetchErr 是独立单页上限触发后的订单发现错误。
	_, fetchErr := client.FetchSoldOrdersPage(context.Background(), "unb=1; _m_h5_tk=token_1;", 1, 30)
	if !errors.Is(fetchErr, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("err=%v duration=%v", fetchErr, time.Since(started))
	}
}

// TestNormalizeSoldOrderStatus 封装TestNormalizeSold订单状态业务协调。
func TestNormalizeSoldOrderStatus(t *testing.T) {
	// cases 用于本次流程后续判断的cases
	cases := map[string]string{
		"待付款": "processing", "待发货": "pending_ship", "已发货": "shipped",
		"买家已确认收货": "received", "交易成功": "completed", "退款成功": "refunded", "退款中": "refunding", "交易关闭": "cancelled", "退款关闭": "unknown",
	}
	// input、want 表示当前遍历过程中的input、want
	for input, want := range cases {
		if // got 用于本次流程后续判断的got
		got := normalizeSoldOrderStatus(input, false); got != want {
			t.Fatalf("input=%s got=%s want=%s", input, got, want)
		}
	}
	if // got 用于本次流程后续判断的got
	got := normalizeSoldOrderStatus("待发货", true); got != "refunding" {
		t.Fatalf("inRefund got=%s", got)
	}
}
