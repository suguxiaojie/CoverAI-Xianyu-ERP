package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestValidOrdersMatchesAnalyticsScope 封装Test有效订单列表MatchesAnalyticsScope业务协调。
func TestValidOrdersMatchesAnalyticsScope(t *testing.T) {
	// srv、store、cleanup 用于本次流程后续判断的srv、store、cleanup
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()

	_, _ = store.DB.ExecContext(ctx, `
		INSERT INTO orders (order_id, item_id, buyer_id, quantity, amount, order_status, cookie_id, created_at) VALUES
		('ord-valid', 'item1', 'buyer1', '2', '¥12.50', 'pending_ship', 'acc1', '2026-06-28 10:00:00'),
		('ord-received', 'item1', 'buyer4', '1', '5.00', 'received', 'acc1', '2026-06-28 10:30:00'),
		('ord-chat-title', 'item2', 'buyer5', '1', '2.00', 'shipped', 'acc1', '2026-06-28 10:45:00'),
		('ord-no-amount', 'item1', 'buyer2', '1', '', 'pending_ship', 'acc1', '2026-06-28 10:00:00'),
		('ord-bad-status', 'item1', 'buyer3', '1', '9.90', 'cancelled', 'acc1', '2026-06-28 10:00:00')
	`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO item_info (cookie_id, item_id, item_title, item_detail) VALUES ('acc1','item1','测试商品','{"pic_info":{"picUrl":"https://img.example/item.png"}}')`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO item_skus(cookie_id,item_id,sku_id,properties_json,cost_cents) VALUES('acc1','item1','__default__','[]',300)`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO order_cost_snapshots(order_id,cookie_id,item_id,sku_id,unit_cost_cents,quantity,match_source,captured_at) VALUES('ord-valid','acc1','item1','__default__',300,2,'single_local_default',1)`)
	_, _ = store.DB.ExecContext(ctx, `UPDATE item_info SET deleted_at='2026-06-29 00:00:00' WHERE cookie_id='acc1' AND item_id='item1'`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO chat_sessions (cookie_id,chat_id,item_id,item_title,updated_at) VALUES ('acc1','chat-item2','item2','聊天历史商品',1780000000)`)

	// h 用于本次流程后续判断的h
	h := srv.Router()
	// cookie 用于本次流程后续判断的登录凭证
	cookie := loginHelper(t, h)

	// req 用于本次流程后续判断的req
	req := httptest.NewRequest(http.MethodGet, "/analytics/orders?start_date=2026-06-28&end_date=2026-06-28", nil)
	req.AddCookie(cookie)
	// rec 用于本次流程后续判断的rec
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("analytics status=%d body=%s", rec.Code, rec.Body.String())
	}
	// analytics 用于本次流程后续判断的analytics
	var analytics struct {
		RevenueStats struct {
			TotalOrders int     `json:"total_orders"`
			TotalAmount float64 `json:"total_amount"`
		} `json:"revenue_stats"`
		ItemStats []struct {
			ItemID    string `json:"item_id"`
			ItemTitle string `json:"item_title"`
		} `json:"item_stats"`
		ProfitStats struct {
			CoveredOrders     int     `json:"covered_orders"`
			UnknownCostOrders int     `json:"unknown_cost_orders"`
			ProductCost       float64 `json:"product_cost"`
			PlatformFee       float64 `json:"platform_fee"`
			GrossProfit       float64 `json:"gross_profit"`
			CoverageRate      float64 `json:"coverage_rate"`
		} `json:"profit_stats"`
		DailyProfitStats []struct {
			CoveredOrders int `json:"covered_orders"`
		} `json:"daily_profit_stats"`
		ItemProfitStats []struct {
			ItemID      string  `json:"item_id"`
			GrossProfit float64 `json:"gross_profit"`
		} `json:"item_profit_stats"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec.Body.Bytes(), &analytics); err != nil {
		t.Fatal(err)
	}
	if analytics.RevenueStats.TotalOrders != 3 || analytics.RevenueStats.TotalAmount != 19.5 {
		t.Fatalf("统计口径异常: %+v", analytics.RevenueStats)
	}
	if analytics.ProfitStats.CoveredOrders != 1 || analytics.ProfitStats.UnknownCostOrders != 2 || analytics.ProfitStats.ProductCost != 6 || analytics.ProfitStats.PlatformFee != 0.2 || analytics.ProfitStats.GrossProfit != 6.3 || analytics.ProfitStats.CoverageRate != 33.33 {
		t.Fatalf("商品毛利统计异常: %+v", analytics.ProfitStats)
	}
	if len(analytics.DailyProfitStats) != 1 || analytics.DailyProfitStats[0].CoveredOrders != 1 || len(analytics.ItemProfitStats) != 2 || analytics.ItemProfitStats[0].ItemID != "item1" {
		t.Fatalf("商品毛利维度响应异常: daily=%+v item=%+v", analytics.DailyProfitStats, analytics.ItemProfitStats)
	}
	// itemTitles 保存商品聚合结果中的历史标题，用于同时验证软删除商品和聊天回退。
	itemTitles := map[string]string{}
	// item 是当前检查标题来源的商品聚合结果。
	for _, item := range analytics.ItemStats {
		itemTitles[item.ItemID] = item.ItemTitle
	}
	if itemTitles["item1"] != "测试商品" || itemTitles["item2"] != "聊天历史商品" {
		t.Fatalf("下架商品历史名称未进入统计响应: %+v", analytics.ItemStats)
	}

	// req2 用于本次流程后续判断的req2
	req2 := httptest.NewRequest(http.MethodGet, "/analytics/orders/valid?start_date=2026-06-28&end_date=2026-06-28", nil)
	req2.AddCookie(cookie)
	// rec2 用于本次流程后续判断的rec2
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("valid orders status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	// valid 用于本次流程后续判断的有效
	var valid struct {
		Orders []map[string]any `json:"orders"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec2.Body.Bytes(), &valid); err != nil {
		t.Fatal(err)
	}
	if len(valid.Orders) != 3 {
		t.Fatalf("有效订单明细数量应与统计订单数一致，got %d body=%s", len(valid.Orders), rec2.Body.String())
	}
	// statuses 保存返回明细中的归一化状态，用于验证待发货和已收货使用同一统计范围。
	statuses := map[string]bool{}
	// order 是当前检查的有效订单明细。
	for _, order := range valid.Orders {
		statuses[fmt.Sprint(order["status"])] = true
		if order["item_id"] == "item1" && (order["item_title"] != "测试商品" || !strings.Contains(order["item_image"].(string), "img.example")) {
			t.Fatalf("有效订单明细字段异常: %+v", order)
		}
	}
	if !statuses["pending_ship"] || !statuses["received"] {
		t.Fatalf("状态字段异常: %+v", valid.Orders)
	}
}

// TestValidOrdersIncludesPaidAndReportsPagination 封装Test有效订单列表IncludesPaidAndReportsPagination业务协调。
func TestValidOrdersIncludesPaidAndReportsPagination(t *testing.T) {
	// srv、store、cleanup 用于本次流程后续判断的srv、store、cleanup
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,amount,order_status,cookie_id,created_at) VALUES
		('paid-1','10','paid','acc1','2026-06-28 10:00:00'),
		('paid-2','20','pending_ship','acc1','2026-06-28 11:00:00')`)
	// h 用于本次流程后续判断的h
	h := srv.Router()
	// cookie 用于本次流程后续判断的登录凭证
	cookie := loginHelper(t, h)
	// req 用于本次流程后续判断的req
	req := httptest.NewRequest(http.MethodGet, "/analytics/orders/valid?start_date=2026-06-28&end_date=2026-06-28&page_size=1", nil)
	req.AddCookie(cookie)
	// rec 用于本次流程后续判断的rec
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// result 用于本次流程后续判断的结果
	var result struct {
		Orders    []map[string]any `json:"orders"`
		Total     int              `json:"total"`
		Truncated bool             `json:"truncated"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Orders) != 1 || result.Total != 2 || !result.Truncated {
		t.Fatalf("result=%+v", result)
	}
}

// TestDashboardStatsAreAvailableAndScopedToCurrentUser 封装TestDashboardStatsAreAvailableAndScopedToCurrent用户业务协调。
func TestDashboardStatsAreAvailableAndScopedToCurrentUser(t *testing.T) {
	// srv、store、cleanup 用于本次流程后续判断的srv、store、cleanup
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()

	if // ok、err 用于本次流程后续判断的ok、err
	ok, err := store.Users.Create(ctx, "member", "member@example.com", "pw"); err != nil || !ok {
		t.Fatalf("create member: ok=%v err=%v", ok, err)
	}
	// member、err 用于本次流程后续判断的member、err
	member, err := store.Users.GetByUsername(ctx, "member")
	if err != nil {
		t.Fatal(err)
	}
	if // err 用于本次流程后续判断的err
	err := store.Cookies.Save(ctx, "member-acc", "unb=456", member.ID); err != nil {
		t.Fatal(err)
	}
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO cards (name,type,data_content,enabled,user_id) VALUES ('member-card','data',?,1,?)`, "CARD-1\n\nCARD-2\n", member.ID)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO keywords (cookie_id,keyword,reply) VALUES ('member-acc','hi','hello')`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,cookie_id,order_status) VALUES ('member-order','member-acc','completed')`)

	// 管理员资源不能进入 member 的统计。
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO cards (name,type,user_id) VALUES ('admin-card','text',1)`)
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,cookie_id,order_status) VALUES ('admin-order','acc1','completed')`)

	// h 用于本次流程后续判断的h
	h := srv.Router()
	// loginReq 用于本次流程后续判断的登录Req
	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"member","password":"pw"}`))
	// loginRec 用于本次流程后续判断的登录Rec
	loginRec := httptest.NewRecorder()
	h.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK || len(loginRec.Result().Cookies()) == 0 {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}

	// req 用于本次流程后续判断的req
	req := httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil)
	req.AddCookie(loginRec.Result().Cookies()[0])
	// rec 用于本次流程后续判断的rec
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// stats 用于本次流程后续判断的stats
	var stats map[string]int64
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	// key 表示当前遍历过程中的key
	for _, key := range []string{"total_cookies", "active_cookies", "total_cards", "total_keywords", "total_orders"} {
		if stats[key] != 1 {
			t.Fatalf("%s=%d want 1; stats=%+v", key, stats[key], stats)
		}
	}
	if stats["available_card_stock"] != 2 {
		t.Fatalf("available_card_stock=%d want 2; stats=%+v", stats["available_card_stock"], stats)
	}

	// adminReq 用于本次流程后续判断的adminReq
	adminReq := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	adminReq.AddCookie(loginRec.Result().Cookies()[0])
	// adminRec 用于本次流程后续判断的adminRec
	adminRec := httptest.NewRecorder()
	h.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusForbidden {
		t.Fatalf("member admin stats status=%d want 403", adminRec.Code)
	}
}

// TestAnalyticsAccountScopeFiltersEveryOrderRead 验证 Dashboard 的汇总和明细共用同一账号范围，且外部用户账号不可被越权统计。
func TestAnalyticsAccountScopeFiltersEveryOrderRead(t *testing.T) {
	// srv、store、cleanup 分别是本地 HTTP 服务、测试数据库和资源释放函数。
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 限定本测试的本地数据准备生命周期。
	ctx := context.Background()
	if // saveErr 是给默认管理员增加第二个账号时的持久化错误。
	saveErr := store.Cookies.Save(ctx, "acc2", "unb=acc2", 1); saveErr != nil {
		t.Fatal(saveErr)
	}
	if // created、createErr 是外部用户的创建结果，用于验证所有权隔离。
	created, createErr := store.Users.Create(ctx, "analytics-member", "analytics-member@example.com", "pw"); createErr != nil || !created {
		t.Fatalf("create member: created=%v err=%v", created, createErr)
	}
	// member 是外部用户的非敏感本地身份记录。
	member, memberErr := store.Users.GetByUsername(ctx, "analytics-member")
	if memberErr != nil {
		t.Fatal(memberErr)
	}
	if // foreignSaveErr 是给外部用户保存账号时的持久化错误。
	foreignSaveErr := store.Cookies.Save(ctx, "foreign-acc", "unb=foreign", member.ID); foreignSaveErr != nil {
		t.Fatal(foreignSaveErr)
	}
	if // insertErr 是三个账号范围测试订单的批量写入错误。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,item_id,amount,order_status,cookie_id,created_at) VALUES
		('scope-acc1','item-1','10','completed','acc1','2026-06-28 10:00:00'),
		('scope-acc2','item-2','20','completed','acc2','2026-06-28 11:00:00'),
		('scope-foreign','item-3','40','completed','foreign-acc','2026-06-28 12:00:00')`); insertErr != nil {
		t.Fatal(insertErr)
	}
	// handler 是已挂载认证和分析路由的本地 HTTP 处理器。
	handler := srv.Router()
	// sessionCookie 是默认管理员的测试会话，不包含平台凭证。
	sessionCookie := loginHelper(t, handler)
	// analyticsReq 只请求管理员第二个账号的经营汇总。
	analyticsReq := httptest.NewRequest(http.MethodGet, "/analytics/orders?start_date=2026-06-28&end_date=2026-06-28&account_id=acc2", nil)
	analyticsReq.AddCookie(sessionCookie)
	// analyticsRecorder 捕获单账号汇总响应。
	analyticsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(analyticsRecorder, analyticsReq)
	// analyticsResponse 只提取断言所需的收益汇总字段。
	var analyticsResponse struct {
		RevenueStats struct {
			TotalOrders int     `json:"total_orders"`
			TotalAmount float64 `json:"total_amount"`
		} `json:"revenue_stats"`
	}
	if // decodeErr 是单账号汇总响应的 JSON 解析错误。
	decodeErr := json.Unmarshal(analyticsRecorder.Body.Bytes(), &analyticsResponse); analyticsRecorder.Code != http.StatusOK || decodeErr != nil {
		t.Fatalf("analytics status=%d decode=%v body=%s", analyticsRecorder.Code, decodeErr, analyticsRecorder.Body.String())
	}
	if analyticsResponse.RevenueStats.TotalOrders != 1 || analyticsResponse.RevenueStats.TotalAmount != 20 {
		t.Fatalf("analytics response=%+v", analyticsResponse)
	}
	// validReq 使用相同账号范围读取参与统计的订单明细。
	validReq := httptest.NewRequest(http.MethodGet, "/analytics/orders/valid?start_date=2026-06-28&end_date=2026-06-28&account_id=acc2", nil)
	validReq.AddCookie(sessionCookie)
	// validRecorder 捕获单账号明细响应。
	validRecorder := httptest.NewRecorder()
	handler.ServeHTTP(validRecorder, validReq)
	if validRecorder.Code != http.StatusOK || !strings.Contains(validRecorder.Body.String(), `"order_id":"scope-acc2"`) || strings.Contains(validRecorder.Body.String(), "scope-acc1") {
		t.Fatalf("valid status=%d body=%s", validRecorder.Code, validRecorder.Body.String())
	}
	// foreignReq 尝试把外部用户账号作为统计范围。
	foreignReq := httptest.NewRequest(http.MethodGet, "/analytics/orders?start_date=2026-06-28&end_date=2026-06-28&account_id=foreign-acc", nil)
	foreignReq.AddCookie(sessionCookie)
	// foreignRecorder 捕获外部账号范围的安全空结果。
	foreignRecorder := httptest.NewRecorder()
	handler.ServeHTTP(foreignRecorder, foreignReq)
	if foreignRecorder.Code != http.StatusOK || !strings.Contains(foreignRecorder.Body.String(), `"total_orders":0`) {
		t.Fatalf("foreign status=%d body=%s", foreignRecorder.Code, foreignRecorder.Body.String())
	}
}

// TestManualHistoricalCostCoversOrdersWithoutCatalog 验证已下架且无 SKU 的历史订单可经人工确认进入利润统计，不伪造商品目录。
func TestManualHistoricalCostCoversOrdersWithoutCatalog(t *testing.T) {
	// srv、store、cleanup 分别是隔离 HTTP 服务、Schema 51 SQLite 和资源释放函数。
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 限定历史成本候选和确认的本地数据操作生命周期。
	ctx := context.Background()
	if // insertErr 是两笔无商品目录历史订单的测试写入错误。
	_, insertErr := store.DB.ExecContext(ctx, `INSERT INTO orders(order_id,item_id,buyer_id,quantity,amount,order_status,cookie_id,created_at) VALUES
		('historical-manual-1','removed-item','buyer-1','2','20.00','completed','acc1','2026-06-28 10:00:00'),
		('historical-manual-2','removed-item','buyer-2','2','20.00','completed','acc1','2026-06-28 11:00:00'),
		('historical-manual-outside','removed-item','buyer-3','2','20.00','completed','acc1','2026-06-29 11:00:00')`); insertErr != nil {
		t.Fatal(insertErr)
	}
	// handler 是挂载版本化历史成本和分析端点的本地 HTTP 处理器。
	handler := srv.Router()
	// sessionCookie 是默认用户的测试会话，不包含闲鱼平台凭证。
	sessionCookie := loginHelper(t, handler)
	// candidatesReq 请求精确账号范围内的历史成本分组。
	candidatesReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/profit-backfill/candidates?account_id=acc1&start_date=2026-06-28&end_date=2026-06-28&timezone_offset_minutes=0", nil)
	candidatesReq.AddCookie(sessionCookie)
	// candidatesRecorder 捕获无 SKU 历史成本候选响应。
	candidatesRecorder := httptest.NewRecorder()
	handler.ServeHTTP(candidatesRecorder, candidatesReq)
	// candidates 是服务端按账号、商品、金额和数量归并的候选 DTO。
	var candidates []manualCostCandidateResponse
	if // decodeErr 是历史成本候选响应的 JSON 解析错误。
	decodeErr := json.Unmarshal(candidatesRecorder.Body.Bytes(), &candidates); candidatesRecorder.Code != http.StatusOK || decodeErr != nil {
		t.Fatalf("candidates status=%d decode=%v body=%s", candidatesRecorder.Code, decodeErr, candidatesRecorder.Body.String())
	}
	if len(candidates) != 1 || !candidates[0].ManualOnly || candidates[0].AccountID != "acc1" || candidates[0].ItemID != "removed-item" || candidates[0].AmountCents != 2000 || candidates[0].Quantity != 2 || candidates[0].OrderCount != 2 {
		t.Fatalf("candidates=%+v", candidates)
	}
	// confirmReq 由用户明确确认该分组历史单件成本为 3 元。
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/v1/analytics/profit-backfill/confirm", strings.NewReader(`{"account_id":"acc1","item_id":"removed-item","amount_cents":2000,"quantity":2,"start_date":"2026-06-28","end_date":"2026-06-28","timezone_offset_minutes":0,"sku_id":"","custom_unit_cost_cents":300}`))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmReq.AddCookie(sessionCookie)
	// confirmRecorder 捕获订单级人工历史成本写入结果。
	confirmRecorder := httptest.NewRecorder()
	handler.ServeHTTP(confirmRecorder, confirmReq)
	// confirmation 是人工历史成本覆盖的具名成功 DTO。
	var confirmation manualCostConfirmationResponse
	if // decodeErr 是人工确认响应的 JSON 解析错误。
	decodeErr := json.Unmarshal(confirmRecorder.Body.Bytes(), &confirmation); confirmRecorder.Code != http.StatusOK || decodeErr != nil {
		t.Fatalf("confirm status=%d decode=%v body=%s", confirmRecorder.Code, decodeErr, confirmRecorder.Body.String())
	}
	if confirmation.MatchedOrders != 2 || confirmation.MatchSource != "manual_historical_cost" {
		t.Fatalf("confirmation=%+v", confirmation)
	}
	// outsideCovered 是 Dashboard 日期范围之外的同金额订单是否被错误覆盖的存在性结果。
	var outsideCovered int
	if // outsideErr 是检查人工确认严格保留当前日期范围的查询错误。
	outsideErr := store.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM historical_order_cost_overrides WHERE order_id='historical-manual-outside')`).Scan(&outsideCovered); outsideErr != nil || outsideCovered != 0 {
		t.Fatalf("outside covered=%d err=%v", outsideCovered, outsideErr)
	}
	// analyticsReq 读取确认后同日的利润覆盖结果。
	analyticsReq := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/orders?start_date=2026-06-28&end_date=2026-06-28&account_id=acc1&timezone_offset_minutes=0", nil)
	analyticsReq.AddCookie(sessionCookie)
	// analyticsRecorder 捕获人工历史成本进入现有利润聚合后的响应。
	analyticsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(analyticsRecorder, analyticsReq)
	// analytics 是包含覆盖数、成本、手续费和毛利的具名分析 DTO。
	var analytics orderAnalyticsResponse
	if // decodeErr 是利润分析响应的 JSON 解析错误。
	decodeErr := json.Unmarshal(analyticsRecorder.Body.Bytes(), &analytics); analyticsRecorder.Code != http.StatusOK || decodeErr != nil {
		t.Fatalf("analytics status=%d decode=%v body=%s", analyticsRecorder.Code, decodeErr, analyticsRecorder.Body.String())
	}
	if analytics.ProfitStats.TotalOrders != 2 || analytics.ProfitStats.CoveredOrders != 2 || analytics.ProfitStats.UnknownCostOrders != 0 || analytics.ProfitStats.ProductCost != 12 || analytics.ProfitStats.PlatformFee != 0.64 || analytics.ProfitStats.GrossProfit != 27.36 {
		t.Fatalf("profit=%+v", analytics.ProfitStats)
	}
	// catalogRows 是人工历史成本确认后当前商品目录中的同商品行数。
	var catalogRows int
	if // catalogErr 是检查流程没有伪造 item_info 或 SKU 的查询错误。
	catalogErr := store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_info WHERE cookie_id='acc1' AND item_id='removed-item'`).Scan(&catalogRows); catalogErr != nil || catalogRows != 0 {
		t.Fatalf("catalog rows=%d err=%v", catalogRows, catalogErr)
	}
}

// TestAnalyticsIncludesLegacyNumericValidStatuses 封装TestAnalyticsIncludesLegacyNumeric有效Statuses业务协调。
func TestAnalyticsIncludesLegacyNumericValidStatuses(t *testing.T) {
	// srv、store、cleanup 用于本次流程后续判断的srv、store、cleanup
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,amount,order_status,cookie_id,created_at)
		VALUES ('legacy-shipped','8.50','3','acc1','2026-06-28 12:00:00')`)
	// h 用于本次流程后续判断的h
	h := srv.Router()
	// cookie 用于本次流程后续判断的登录凭证
	cookie := loginHelper(t, h)
	// req 用于本次流程后续判断的req
	req := httptest.NewRequest(http.MethodGet, "/analytics/orders?start_date=2026-06-28&end_date=2026-06-28", nil)
	req.AddCookie(cookie)
	// rec 用于本次流程后续判断的rec
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// response 用于本次流程后续判断的响应
	var response struct {
		RevenueStats struct {
			TotalOrders int `json:"total_orders"`
		} `json:"revenue_stats"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.RevenueStats.TotalOrders != 1 {
		t.Fatalf("legacy numeric valid status was excluded: %+v", response)
	}
	// validReq 用于本次流程后续判断的有效Req
	validReq := httptest.NewRequest(http.MethodGet, "/analytics/orders/valid?start_date=2026-06-28&end_date=2026-06-28", nil)
	validReq.AddCookie(cookie)
	// validRec 用于本次流程后续判断的有效Rec
	validRec := httptest.NewRecorder()
	h.ServeHTTP(validRec, validReq)
	if !strings.Contains(validRec.Body.String(), `"order_status":"shipped"`) {
		t.Fatalf("legacy detail status was not normalized: %s", validRec.Body.String())
	}
}

// TestAnalyticsCustomRangeDoesNotSilentlyDropDays 封装TestAnalyticsCustomRangeDoesNotSilentlyDropDays业务协调。
func TestAnalyticsCustomRangeDoesNotSilentlyDropDays(t *testing.T) {
	// srv、store、cleanup 用于本次流程后续判断的srv、store、cleanup
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	for // day 用于本次流程后续判断的day
	day := 1; day <= 31; day++ {
		// date 用于本次流程后续判断的日期
		date := time.Date(2026, 1, day, 12, 0, 0, 0, time.UTC).Format("2006-01-02 15:04:05")
		_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,amount,order_status,cookie_id,created_at) VALUES (?,?,?,?,?)`, fmt.Sprintf("day-%02d", day), "1", "completed", "acc1", date)
	}
	// h 用于本次流程后续判断的h
	h := srv.Router()
	// cookie 用于本次流程后续判断的登录凭证
	cookie := loginHelper(t, h)
	// req 用于本次流程后续判断的req
	req := httptest.NewRequest(http.MethodGet, "/analytics/orders?start_date=2026-01-01&end_date=2026-01-31", nil)
	req.AddCookie(cookie)
	// rec 用于本次流程后续判断的rec
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// response 用于本次流程后续判断的响应
	var response struct {
		Daily []map[string]any `json:"daily_stats"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || len(response.Daily) != 31 {
		t.Fatalf("daily range len=%d err=%v body=%s", len(response.Daily), err, rec.Body.String())
	}
}

// TestAnalyticsDateBoundaryConvertsLocalDayToUTC 封装TestAnalytics日期BoundaryConvertsLocalDayToUTC业务协调。
func TestAnalyticsDateBoundaryConvertsLocalDayToUTC(t *testing.T) {
	// previous 用于本次流程后续判断的previous
	previous := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	defer func() { time.Local = previous }()
	if // got 用于本次流程后续判断的got
	got := analyticsDateBoundary("2026-06-28", false, time.Local); got != "2026-06-27 16:00:00" {
		t.Fatalf("start boundary=%q", got)
	}
	if // got 用于本次流程后续判断的got
	got := analyticsDateBoundary("2026-06-28", true, time.Local); got != "2026-06-28 16:00:00" {
		t.Fatalf("end boundary=%q", got)
	}
}

// TestAnalyticsUsesBrowserTimezoneAndSkipsInvalidAmounts 封装TestAnalyticsUses浏览器TimezoneAndSkipsInvalidAmounts业务协调。
func TestAnalyticsUsesBrowserTimezoneAndSkipsInvalidAmounts(t *testing.T) {
	// srv、store、cleanup 用于本次流程后续判断的srv、store、cleanup
	srv, store, cleanup := newTestServer(t)
	defer cleanup()
	// ctx 用于本次流程后续判断的ctx
	ctx := context.Background()
	_, _ = store.DB.ExecContext(ctx, `INSERT INTO orders (order_id,amount,order_status,cookie_id,created_at) VALUES
		('tz-valid','10.50','completed','acc1','2026-06-27 16:30:00'),
		('tz-rfc3339','20.00','shipped','acc1','2026-06-28T13:00:00Z'),
		('tz-end-boundary','99.00','completed','acc1','2026-06-28T16:00:00Z'),
		('tz-invalid','abc','completed','acc1','2026-06-27 17:00:00')`)
	// h 用于本次流程后续判断的h
	h := srv.Router()
	// cookie 用于本次流程后续判断的登录凭证
	cookie := loginHelper(t, h)
	// req 用于本次流程后续判断的req
	req := httptest.NewRequest(http.MethodGet, "/analytics/orders?start_date=2026-06-28&end_date=2026-06-28&timezone_offset_minutes=480", nil)
	req.AddCookie(cookie)
	// rec 用于本次流程后续判断的rec
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// response 用于本次流程后续判断的响应
	var response struct {
		Revenue struct {
			TotalOrders int     `json:"total_orders"`
			TotalAmount float64 `json:"total_amount"`
		} `json:"revenue_stats"`
		Daily []map[string]any `json:"daily_stats"`
	}
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Revenue.TotalOrders != 2 || response.Revenue.TotalAmount != 30.5 || len(response.Daily) != 1 || response.Daily[0]["date"] != "2026-06-28" {
		t.Fatalf("response=%+v body=%s", response, rec.Body.String())
	}
}
