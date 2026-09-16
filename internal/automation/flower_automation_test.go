package automation

import (
	"context"
	"testing"
	"time"

	"xianyu-go/internal/db"
)

// flowerReceiveBrowserFake 记录自动收花页面调用，并在协调器取消后退出。
type flowerReceiveBrowserFake struct {
	// calls 接收实际打开的账号、订单和窗口可见性。
	calls chan flowerBrowserCall
}

// flowerBrowserCall 保存浏览器替身观察到的非敏感调用参数。
type flowerBrowserCall struct {
	// AccountID 是调用所属账号。
	AccountID string
	// OrderID 是调用关联订单。
	OrderID string
	// ShowBrowser 是配置的窗口可见性。
	ShowBrowser bool
}

// OpenRedFlowerReceive 记录调用并阻塞到 WS 成功路径取消浏览器 Context。
func (browser *flowerReceiveBrowserFake) OpenRedFlowerReceive(ctx context.Context, accountID, _ string, orderID string, showBrowser bool) (bool, error) {
	browser.calls <- flowerBrowserCall{AccountID: accountID, OrderID: orderID, ShowBrowser: showBrowser}
	<-ctx.Done()
	return true, ctx.Err()
}

// TestFlowerAutomationCompletesOnlyAfterReceivedEvent 验证打开官方页面不会直接成功，只有同会话 WS 收花结果才完成运行。
func TestFlowerAutomationCompletesOnlyAfterReceivedEvent(t *testing.T) {
	// store、cleanup 是隔离自动化数据库及释放函数。
	store, cleanup := newAutomationTestStore(t)
	defer cleanup()
	// ctx、cancel 是自动收花协调器的测试生命周期。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// settingsErr 是自动收花账号配置写入错误。
	settingsErr := store.AccountTasks.Upsert(ctx, db.AccountTaskSettings{
		CookieID: "cid", RateContent: "交易愉快", PolishTime: "03:00", AutoReceiveFlowerEnabled: true,
		RequestFlowerAfterHours: 24, ReceiveFlowerShowBrowser: true, ReceiveFlowerTimeoutSeconds: 120,
	})
	if settingsErr != nil {
		t.Fatal(settingsErr)
	}
	// browser 是只记录页面打开并等待取消的隔离浏览器替身。
	browser := &flowerReceiveBrowserFake{calls: make(chan flowerBrowserCall, 1)}
	// center 是注入浏览器替身的自动化中心。
	center := NewWithDependencies(store, nil, nil, CenterDependencies{FlowerReceiveBrowser: browser})
	// startErr 是自动收花 worker 启动结果。
	startErr := center.StartFlowerAutomation(ctx)
	if startErr != nil {
		t.Fatal(startErr)
	}
	// sent 是买家已经送花的结构化系统事件。
	sent := Task{Source: "ws", AccountID: "cid", TriggerType: TriggerRedFlowerSent, ChatID: "chat-1", OrderID: "5127372398162002704", BuyerID: "buyer-1", ItemID: "item-1"}
	// handleErr 是送花事件入队结果。
	handleErr := center.HandleTask(ctx, sent)
	if handleErr != nil {
		t.Fatal(handleErr)
	}
	select {
	case call := <-browser.calls: // call 是浏览器替身观察到的页面调用。
		if call.AccountID != "cid" || call.OrderID != sent.OrderID || !call.ShowBrowser {
			t.Fatalf("browser call=%+v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("自动收花浏览器未启动")
	}
	// before、exists、beforeErr 是页面已打开但 WS 结果尚未到达时的运行状态。
	before, exists, beforeErr := store.AccountTasks.GetRunByKey(ctx, "red_flower_receive:cid:"+sent.OrderID)
	if beforeErr != nil || !exists || before.Status != "running" {
		t.Fatalf("before=%+v exists=%v err=%v", before, exists, beforeErr)
	}
	// received 是同账号同会话的收花完成系统事件。
	received := Task{Source: "ws", AccountID: "cid", TriggerType: TriggerRedFlowerReceived, ChatID: "chat-1"}
	// handleErr 是平台收花结果交给 pending 运行的处理结果。
	handleErr = center.HandleTask(ctx, received)
	if handleErr != nil {
		t.Fatal(handleErr)
	}
	// deadline 限制持久化成功终态的轮询时间。
	deadline := time.Now().Add(2 * time.Second)
	for {
		// run、runExists、runErr 是当前自动收花运行状态。
		run, runExists, runErr := store.AccountTasks.GetRunByKey(ctx, "red_flower_receive:cid:"+sent.OrderID)
		if runErr != nil {
			t.Fatal(runErr)
		}
		if runExists && run.Status == "success" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not succeed: %+v", run)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// closeCtx、closeCancel 限制 worker 关闭等待时间。
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer closeCancel()
	// closeErr 是 worker 和浏览器调用的收束结果。
	closeErr := center.CloseFlowerAutomation(closeCtx)
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}
