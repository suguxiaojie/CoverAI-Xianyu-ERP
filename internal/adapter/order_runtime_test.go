package adapter

import (
	"context"
	"errors"
	"fmt"
	"testing"

	orderapp "xianyu-go/internal/application/orders"
	"xianyu-go/internal/db"
	"xianyu-go/internal/xianyu/mtop"
)

// orderRuntimeAutomationFake 是订单运行时测试使用的自动化能力替身。
type orderRuntimeAutomationFake struct {
	// calls 记录自动化完整发货调用次数。
	calls int
}

// ManualFullDelivery 返回固定发送数量，验证 adapter 的订单模型转换回调。
func (f *orderRuntimeAutomationFake) ManualFullDelivery(context.Context, *db.Order) (int, error) {
	f.calls++
	return 2, nil
}

// ScheduleRedFlowerAfterShipment 记录测试自动求花调度调用。
func (f *orderRuntimeAutomationFake) ScheduleRedFlowerAfterShipment(context.Context, string) error {
	f.calls++
	return nil
}

// orderRuntimeNotifierFake 是订单运行时测试使用的通知能力替身。
type orderRuntimeNotifierFake struct {
	// calls 记录发货通知调用次数。
	calls int
}

// NotifyDelivery 记录一次发货通知调用。
func (f *orderRuntimeNotifierFake) NotifyDelivery(string, string, string, string, string, string) {
	f.calls++
}

// orderRuntimeMTopFake 是订单运行时测试使用的平台客户端替身。
type orderRuntimeMTopFake struct {
	// consign 保存确认发货调用结果函数。
	consign func(context.Context, string, string) (bool, []string, string, error)
	// consignCalls 记录确认发货调用次数。
	consignCalls int
	// redFlower 保存手动求花调用结果函数。
	redFlower func(context.Context, string, string, string) (*mtop.RedFlowerRequestResult, error)
	// redFlowerCalls 记录手动求花调用次数。
	redFlowerCalls int
	// soldPages 保存按页码返回的已售订单列表夹具。
	soldPages map[int]*mtop.SoldOrdersPage
	// soldPageCalls 保存订单分页实际请求的页码顺序。
	soldPageCalls []int
}

// RequestRedFlower 返回测试预置的官方求花结果。
func (f *orderRuntimeMTopFake) RequestRedFlower(ctx context.Context, cookies, orderID, channel string) (*mtop.RedFlowerRequestResult, error) {
	f.redFlowerCalls++
	if f.redFlower == nil {
		return nil, errors.New("订单测试未配置求花结果")
	}
	return f.redFlower(ctx, cookies, orderID, channel)
}

// FetchUserProfile 满足基础 MTOP 客户端接口但不参与订单测试。
func (f *orderRuntimeMTopFake) FetchUserProfile(context.Context, string) (*mtop.UserProfileResult, error) {
	return nil, errors.New("订单测试未预期资料请求")
}

// ConsignContext 返回测试预置的确认发货结果。
func (f *orderRuntimeMTopFake) ConsignContext(ctx context.Context, cookies, orderID string) (bool, []string, string, error) {
	f.consignCalls++
	if f.consign == nil {
		return false, nil, "", errors.New("订单测试未配置确认发货结果")
	}
	return f.consign(ctx, cookies, orderID)
}

// FetchItemsPage 满足基础 MTOP 客户端接口但不参与订单测试。
func (f *orderRuntimeMTopFake) FetchItemsPage(context.Context, string, int, int) (*mtop.ItemListResult, error) {
	return nil, errors.New("订单测试未预期商品分页请求")
}

// FetchAllItems 满足基础 MTOP 客户端接口但不参与订单测试。
func (f *orderRuntimeMTopFake) FetchAllItems(context.Context, string, int, int) (*mtop.ItemListResult, error) {
	return nil, errors.New("订单测试未预期商品列表请求")
}

// PublishItem 满足基础 MTOP 客户端接口但不参与订单测试。
func (f *orderRuntimeMTopFake) PublishItem(context.Context, string, mtop.PublishItemRequest) (*mtop.PublishItemResult, error) {
	return nil, errors.New("订单测试未预期商品发布请求")
}

// RefreshTokenWithDeviceIDContext 满足基础 MTOP 客户端接口但不参与订单测试。
func (f *orderRuntimeMTopFake) RefreshTokenWithDeviceIDContext(context.Context, string, string) (*mtop.RefreshResult, error) {
	return nil, errors.New("订单测试未预期令牌请求")
}

// FetchSoldOrdersPage 返回测试配置的已售订单页，缺少页码时视为分页结束。
func (f *orderRuntimeMTopFake) FetchSoldOrdersPage(_ context.Context, _ string, pageNumber, _ int) (*mtop.SoldOrdersPage, error) {
	f.soldPageCalls = append(f.soldPageCalls, pageNumber)
	// page 是当前页码对应的已售订单测试夹具。
	if page := f.soldPages[pageNumber]; page != nil {
		return page, nil
	}
	return &mtop.SoldOrdersPage{}, nil
}

// TestOrderRuntimeIncrementalStopsAfterSafetyBoundary 验证增量读取至少回看三页并在连续历史订单达到阈值后停止请求。
func TestOrderRuntimeIncrementalStopsAfterSafetyBoundary(t *testing.T) {
	// pages 保存五页都早于同步高水位的订单；第三页后应提前停止，不请求第四页。
	pages := make(map[int]*mtop.SoldOrdersPage)
	// pageNumber 是当前构造的测试页码。
	for pageNumber := 1; pageNumber <= 5; pageNumber++ {
		// items 保存当前页的十条历史订单。
		items := make([]mtop.SoldOrder, 0, 10)
		// itemIndex 是当前页内订单序号。
		for itemIndex := 0; itemIndex < 10; itemIndex++ {
			items = append(items, mtop.SoldOrder{OrderID: fmt.Sprintf("old-%d-%d", pageNumber, itemIndex), CreatedAt: "2025-01-01T00:00:00Z"})
		}
		pages[pageNumber] = &mtop.SoldOrdersPage{Items: items, NextPage: pageNumber < 5, TotalCount: 50}
	}
	// client 记录真实分页请求次数。
	client := &orderRuntimeMTopFake{soldPages: pages}
	// runtime 只使用本地测试平台客户端。
	runtime := NewOrderRuntime(nil, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// snapshots 保存每一页完成后的增量边界进度。
	var snapshots []orderapp.RefreshSoldPageProgress
	// result、err 保存增量分页结果和错误。
	result, err := runtime.FetchSoldOrdersWithOptions(context.Background(), &orderapp.PlatformRuntimeData{Value: "cookie"}, orderapp.RefreshSoldFetchOptions{
		Mode: orderapp.RefreshModeIncremental, Cursor: &orderapp.OrderSyncCursor{HighWaterCreatedAt: "2025-02-01T00:00:00Z", HighWaterOrderID: "known"}, MinimumPages: 3, BoundaryRequired: 20,
	}, func(progress orderapp.RefreshSoldPageProgress) { snapshots = append(snapshots, progress) })
	if err != nil || len(client.soldPageCalls) != 3 || len(result.Orders) != 30 || !result.BoundaryReached || result.CompleteSnapshot {
		t.Fatalf("calls=%v result=%+v err=%v", client.soldPageCalls, result, err)
	}
	if len(snapshots) != 3 || snapshots[2].BoundaryMatched != 30 || snapshots[2].Mode != orderapp.RefreshModeIncremental {
		t.Fatalf("snapshots=%+v", snapshots)
	}
}

// TestOrderRuntimeFullModeReadsEveryPage 验证全量校准忽略增量边界并读取到平台自然分页结束。
func TestOrderRuntimeFullModeReadsEveryPage(t *testing.T) {
	// client 提供三页完整快照。
	client := &orderRuntimeMTopFake{soldPages: map[int]*mtop.SoldOrdersPage{
		1: {Items: []mtop.SoldOrder{{OrderID: "order-3", CreatedAt: "2025-01-03T00:00:00Z"}}, NextPage: true, TotalCount: 3},
		2: {Items: []mtop.SoldOrder{{OrderID: "order-2", CreatedAt: "2025-01-02T00:00:00Z"}}, NextPage: true, TotalCount: 3},
		3: {Items: []mtop.SoldOrder{{OrderID: "order-1", CreatedAt: "2025-01-01T00:00:00Z"}}, NextPage: false, TotalCount: 3},
	}}
	// runtime 只访问本地分页夹具。
	runtime := NewOrderRuntime(nil, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// result、err 保存完整分页结果和错误。
	result, err := runtime.FetchSoldOrdersWithOptions(context.Background(), &orderapp.PlatformRuntimeData{Value: "cookie"}, orderapp.RefreshSoldFetchOptions{Mode: orderapp.RefreshModeFull}, nil)
	if err != nil || len(client.soldPageCalls) != 3 || len(result.Orders) != 3 || !result.CompleteSnapshot || result.BoundaryReached {
		t.Fatalf("calls=%v result=%+v err=%v", client.soldPageCalls, result, err)
	}
}

// TestNewOrderRuntimeHooksUsesTypedOptionalServices 验证订单运行时依赖由 adapter typed port 统一转换并安全处理空依赖。
func TestNewOrderRuntimeHooksUsesTypedOptionalServices(t *testing.T) {
	// emptyHooks 保存缺少可选服务时的回调集合。
	emptyHooks := NewOrderRuntimeHooks(nil, nil, nil, nil, nil, nil)
	if emptyHooks.AutomationReady() || emptyHooks.ClientAvailable() || emptyHooks.AccountRunning("missing") {
		t.Fatal("空依赖不应报告订单运行时能力可用")
	}
	// err 保存空自动化能力执行完整发货时返回的初始化错误。
	if _, err := emptyHooks.ManualFullDelivery(context.Background(), &orderapp.Order{}); err == nil {
		t.Fatal("空自动化依赖应返回错误")
	}
	emptyHooks.NotifyDelivery("cookie", "buyer", "item", "chat", "message")

	// automation、notifier 保存 typed port 测试替身。
	automation := &orderRuntimeAutomationFake{}
	// notifier 保存记录通知调用次数的 typed port 测试替身。
	notifier := &orderRuntimeNotifierFake{}
	// hooks 保存已装配可选服务的订单运行时回调集合。
	hooks := NewOrderRuntimeHooks(nil, nil, automation, notifier, nil, nil)
	if hooks.AutomationReady() {
		t.Fatal("缺少账号 Manager 时不应报告完整自动化已就绪")
	}
	// sent、err 保存完整发货回调的发送数量和执行错误。
	if sent, err := hooks.ManualFullDelivery(context.Background(), &orderapp.Order{OrderID: "order-1"}); err != nil || sent != 2 || automation.calls != 1 {
		t.Fatalf("自动化回调未正确转发: sent=%d err=%v calls=%d", sent, err, automation.calls)
	}
	hooks.NotifyDelivery("cookie", "buyer", "item", "chat", "message")
	if notifier.calls != 1 {
		t.Fatalf("通知回调次数=%d，期望 1", notifier.calls)
	}
}

// TestOrderRuntimeConfirmShipmentPersistsCookie 验证确认发货成功会写回平台返回的新 Cookie。
func TestOrderRuntimeConfirmShipmentPersistsCookie(t *testing.T) {
	// store、cleanup 保存测试数据库和资源清理函数。
	store, cleanup := newAdapterTestStore(t)
	defer cleanup()
	// client 保存返回成功及新 Cookie 的平台客户端替身。
	client := &orderRuntimeMTopFake{consign: func(context.Context, string, string) (bool, []string, string, error) {
		return true, []string{"SUCCESS"}, "sid=new", nil
	}}
	// runtime 保存绑定数据库和平台客户端的订单运行时适配器。
	runtime := NewOrderRuntime(store, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// result 保存确认发货的应用层结果。
	result := runtime.ConfirmShipment(context.Background(), "cid", "order-1", 1)
	if !result.Success || result.Err != nil || result.RuntimeCookie != "sid=new" || !result.RuntimeCookieChanged {
		t.Fatalf("确认发货结果异常: %+v", result)
	}
	// savedCookie 保存凭证适配器写回数据库的 Cookie。
	savedCookie, savedErr := store.Cookies.GetValue(context.Background(), "cid")
	if savedErr != nil || savedCookie != "sid=new" {
		t.Fatalf("Cookie 写回异常: value=%q err=%v", savedCookie, savedErr)
	}
}

// TestOrderRuntimeRequestRedFlowerProjectsResult 验证订单运行时只投影求花结果和受控 Cookie 会话变化。
func TestOrderRuntimeRequestRedFlowerProjectsResult(t *testing.T) {
	// client 返回平台明确成功结果并记录请求参数。
	client := &orderRuntimeMTopFake{redFlower: func(_ context.Context, cookies, orderID, channel string) (*mtop.RedFlowerRequestResult, error) {
		if cookies != "synthetic-cookie" || orderID != "order-1" || channel != "im" {
			t.Fatalf("cookies=%q order=%q channel=%q", cookies, orderID, channel)
		}
		return &mtop.RedFlowerRequestResult{Success: true, Message: "求花成功", UpdatedCookies: "synthetic-cookie"}, nil
	}}
	// runtime 使用可选求花接口，不访问真实平台或数据库。
	runtime := NewOrderRuntime(nil, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	if !runtime.RedFlowerAvailable() {
		t.Fatal("测试平台客户端应暴露求花能力")
	}
	// result、requestErr 是应用层可消费的求花结果和错误。
	result, requestErr := runtime.RequestRedFlower(context.Background(), &orderapp.PlatformRuntimeData{ID: "account-1", Value: "synthetic-cookie"}, "order-1", "im")
	if requestErr != nil || result == nil || !result.Success || result.Message != "求花成功" || client.redFlowerCalls != 1 {
		t.Fatalf("result=%+v calls=%d err=%v", result, client.redFlowerCalls, requestErr)
	}
}

// TestOrderRuntimeFetchSoldOrdersReportsPageProgress 验证平台每页返回后立即发布累计订单数和总页数。
func TestOrderRuntimeFetchSoldOrdersReportsPageProgress(t *testing.T) {
	// client 提供两页共三条脱敏订单，平台总数为 31 以验证总页数计算。
	client := &orderRuntimeMTopFake{soldPages: map[int]*mtop.SoldOrdersPage{
		1: {Items: []mtop.SoldOrder{{OrderID: "order-1"}, {OrderID: "order-2"}}, NextPage: true, TotalCount: 31},
		2: {Items: []mtop.SoldOrder{{OrderID: "order-3"}}, NextPage: false, TotalCount: 31},
	}}
	// runtime 使用测试平台客户端，不读写真实数据库或闲鱼账号。
	runtime := NewOrderRuntime(nil, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// snapshots 按平台分页完成顺序保存累计进度。
	var snapshots []orderapp.RefreshSoldPageProgress
	// reporter 复制每页进度，不执行外部 I/O。
	reporter := func(progress orderapp.RefreshSoldPageProgress) { snapshots = append(snapshots, progress) }
	// result、fetchErr 是分页累计订单结果和错误。
	result, fetchErr := runtime.FetchSoldOrdersWithProgress(context.Background(), &orderapp.PlatformRuntimeData{Value: "cookie"}, reporter)
	if fetchErr != nil || len(result.Orders) != 3 {
		t.Fatalf("result=%+v err=%v", result, fetchErr)
	}
	if len(snapshots) != 2 || snapshots[0].Processed != 2 || snapshots[0].Total != 31 || snapshots[1].Processed != 3 || snapshots[1].TotalPages != 2 {
		t.Fatalf("snapshots=%+v", snapshots)
	}
}

// TestOrderRuntimeConfirmShipmentPropagatesPlatformError 验证平台错误会透传且不会伪造成功。
func TestOrderRuntimeConfirmShipmentPropagatesPlatformError(t *testing.T) {
	// store、cleanup 保存测试数据库和资源清理函数。
	store, cleanup := newAdapterTestStore(t)
	defer cleanup()
	// expectedErr 保存平台确认发货错误。
	expectedErr := errors.New("平台确认发货失败")
	// client 保存返回平台错误的客户端替身。
	client := &orderRuntimeMTopFake{consign: func(context.Context, string, string) (bool, []string, string, error) {
		return false, []string{"FAIL"}, "", expectedErr
	}}
	// runtime 保存订单运行时适配器。
	runtime := NewOrderRuntime(store, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// result 保存平台错误对应的应用层结果。
	result := runtime.ConfirmShipment(context.Background(), "cid", "order-1", 1)
	if result.Success || !errors.Is(result.Err, expectedErr) || client.consignCalls != 1 {
		t.Fatalf("平台错误未正确透传: result=%+v calls=%d", result, client.consignCalls)
	}
}

// TestOrderRuntimeConfirmShipmentRejectsOtherOwner 验证跨用户确认发货会在平台调用前拒绝。
func TestOrderRuntimeConfirmShipmentRejectsOtherOwner(t *testing.T) {
	// store、cleanup 保存测试数据库和资源清理函数。
	store, cleanup := newAdapterTestStore(t)
	defer cleanup()
	// client 保存用于确认平台未被调用的客户端替身。
	client := &orderRuntimeMTopFake{consign: func(context.Context, string, string) (bool, []string, string, error) {
		return true, nil, "sid=unexpected", nil
	}}
	// runtime 保存订单运行时适配器。
	runtime := NewOrderRuntime(store, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// result 保存跨用户请求的应用层结果。
	result := runtime.ConfirmShipment(context.Background(), "cid", "order-1", 999)
	if !errors.Is(result.Err, orderapp.ErrForbidden) || client.consignCalls != 0 {
		t.Fatalf("跨用户请求未被拒绝: result=%+v calls=%d", result, client.consignCalls)
	}
}

// TestOrderRuntimeConfirmShipmentHonorsCancellation 验证请求取消会传入平台调用并返回取消错误。
func TestOrderRuntimeConfirmShipmentHonorsCancellation(t *testing.T) {
	// store、cleanup 保存测试数据库和资源清理函数。
	store, cleanup := newAdapterTestStore(t)
	defer cleanup()
	// client 保存检查 Context 取消状态的客户端替身。
	client := &orderRuntimeMTopFake{consign: func(ctx context.Context, _, _ string) (bool, []string, string, error) {
		return false, nil, "", ctx.Err()
	}}
	// runtime 保存订单运行时适配器。
	runtime := NewOrderRuntime(store, OrderRuntimeHooks{Client: func() mtop.Client { return client }, ClientAvailable: func() bool { return true }}, nil, nil)
	// ctx 保存已经取消的订单请求上下文。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// result 保存取消请求对应的应用层结果。
	result := runtime.ConfirmShipment(ctx, "cid", "order-1", 1)
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("取消错误未透传: %+v", result)
	}
}

// 确保测试平台替身覆盖订单运行时依赖的基础客户端接口。
var _ mtop.Client = (*orderRuntimeMTopFake)(nil)
