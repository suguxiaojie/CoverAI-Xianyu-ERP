package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestGetOwnedPendingOrderCloseAllowsPaidPendingShipmentAndRejectsLaterTerminal 验证取消上下文覆盖待付款和已付款待发货，并拒绝后续终态。
func TestGetOwnedPendingOrderCloseAllowsPaidPendingShipmentAndRejectsLaterTerminal(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部数据库调用共用的上下文。
	ctx := context.Background()
	// userID 是测试卖家账号归属的本地用户主键。
	var userID int64
	if // userErr 是创建测试归属用户及读取主键的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "close-owner", "close-owner@example.com", "test-hash").Scan(&userID); userErr != nil {
		t.Fatal(userErr)
	}
	// accountID 是不包含凭证的测试卖家账号标识。
	const accountID = "close-account"
	if // accountErr 是创建测试卖家账号的错误。
	accountErr := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// orderID 是订单表必须确认仍为待付款的测试订单号。
	const orderID = "5127638256187075541"
	if // orderErr 是创建权威待付款订单状态的错误。
	orderErr := store.Orders.Upsert(ctx, orderID, OrderUpsertOpts{CookieID: accountID, OrderStatus: "processing"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	// session 是待付款卡片所属会话。
	session := ChatSession{CookieID: accountID, ChatID: "close-chat", BuyerID: "buyer-1"}
	// pending 是带明确订单号的卖家待付款卡片。
	pending := ChatMessage{MessageKey: "pending-close.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已拍下，待付款", Status: "received", SentAt: 1000,
		SystemCardKind: "trade", SystemCardEvent: "order_pending_payment", SystemCardOrderID: orderID, SystemCardItemID: "item-1", SystemCardAction: "adjust_price"}
	if // inserted、saveErr 表示待付款卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, pending, false); saveErr != nil || !inserted {
		t.Fatalf("inserted=%v err=%v", inserted, saveErr)
	}
	// closeContext、readErr 是终态到达前读取的归属关单上下文。
	closeContext, readErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, orderID)
	if readErr != nil || closeContext.ChatID != "close-chat" || closeContext.ItemID != "item-1" || closeContext.Stage != "pending_payment" {
		t.Fatalf("context=%+v err=%v", closeContext, readErr)
	}
	if // _, forbiddenErr 是其他用户读取当前账号卡片的拒绝结果。
	_, forbiddenErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID+1, accountID, orderID); !errors.Is(forbiddenErr, ErrNotFound) {
		t.Fatalf("foreign owner err=%v", forbiddenErr)
	}
	// buyerAccountID 是同一用户管理但在交易中扮演买家的另一个账号。
	const buyerAccountID = "buyer-1"
	if // buyerAccountErr 是创建买家账号的错误。
	buyerAccountErr := store.Cookies.CreateOwned(ctx, buyerAccountID, "buyer-cookie", userID); buyerAccountErr != nil {
		t.Fatal(buyerAccountErr)
	}
	// buyerSession 模拟同一平台卡片广播到买家自己的账号会话。
	buyerSession := ChatSession{CookieID: buyerAccountID, ChatID: "close-chat", BuyerID: accountID}
	// buyerCard 使用 sender_id 与 cookie_id 相同的买家视角卡片。
	buyerCard := pending
	buyerCard.MessageKey, buyerCard.SenderID = "pending-close-buyer.PNM", buyerAccountID
	if // inserted、saveErr 表示买家侧广播是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, buyerSession, buyerCard, false); saveErr != nil || !inserted {
		t.Fatalf("buyer inserted=%v err=%v", inserted, saveErr)
	}
	if // _, buyerRoleErr 是买家账号尝试读取卖家关单入口的拒绝结果。
	_, buyerRoleErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, buyerAccountID, "5127638256187075541"); !errors.Is(buyerRoleErr, ErrNotFound) {
		t.Fatalf("buyer role err=%v", buyerRoleErr)
	}
	if // _, cancelErr 是订单表已经取消后即使聊天缺少终态卡片也必须拒绝的结果。
	_, cancelErr := store.DB.ExecContext(ctx, `UPDATE orders SET order_status='cancelled' WHERE order_id=?`, orderID); cancelErr != nil {
		t.Fatal(cancelErr)
	}
	if // _, cancelledErr 是权威取消状态对旧待付款卡片的拒绝结果。
	_, cancelledErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, orderID); !errors.Is(cancelledErr, ErrNotFound) {
		t.Fatalf("cancelled order err=%v", cancelledErr)
	}
	if // _, restoreErr 恢复为已付款待发货状态以继续验证同一取消流程。
	_, restoreErr := store.DB.ExecContext(ctx, `UPDATE orders SET order_status='pending_ship' WHERE order_id=?`, orderID); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	// paid 是同一订单后续明确付款卡片，写入后应把取消入口推进到待发货阶段。
	paid := ChatMessage{MessageKey: "paid-close.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: 2000,
		SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardOrderID: "5127638256187075541", SystemCardItemID: "item-1", SystemCardAction: "ship_order"}
	if // inserted、saveErr 表示付款卡片是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, paid, false); saveErr != nil || !inserted {
		t.Fatalf("inserted=%v err=%v", inserted, saveErr)
	}
	// paidContext、paidErr 是已付款待发货阶段继续可取消的上下文。
	paidContext, paidErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, orderID)
	if paidErr != nil || paidContext.Stage != "pending_ship" || paidContext.SentAt != 2000 {
		t.Fatalf("paid context=%+v err=%v", paidContext, paidErr)
	}
	// reminder 是平台把待发货提示误标为 order_shipped 的非终态提醒卡片。
	reminder := ChatMessage{MessageKey: "shipment-reminder-close.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "记得及时发货", Status: "received", SentAt: 2500,
		SystemCardKind: "trade", SystemCardEvent: "order_shipped", SystemCardTitle: "记得及时发货", SystemCardDescription: "如已发货，请点击「去发货」输入快递单号", SystemCardOrderID: orderID}
	if // inserted、saveErr 表示非终态发货提醒是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, reminder, false); saveErr != nil || !inserted {
		t.Fatalf("reminder inserted=%v err=%v", inserted, saveErr)
	}
	if // reminderContext、reminderErr 是提醒到达后仍保留的待发货取消上下文。
	reminderContext, reminderErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, orderID); reminderErr != nil || reminderContext.Stage != "pending_ship" {
		t.Fatalf("reminder context=%+v err=%v", reminderContext, reminderErr)
	}
	// shipped 是平台明确确认已经发货的真实终态卡片。
	shipped := ChatMessage{MessageKey: "shipped-close.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "你已发货", Status: "received", SentAt: 3000,
		SystemCardKind: "trade", SystemCardEvent: "order_shipped", SystemCardTitle: "你已发货", SystemCardDescription: "等待买家收货", SystemCardOrderID: orderID}
	if // inserted、saveErr 表示真实发货终态是否首次保存及其错误。
	_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, shipped, false); saveErr != nil || !inserted {
		t.Fatalf("shipped inserted=%v err=%v", inserted, saveErr)
	}
	if // _, terminalErr 是真实发货后取消资格被拒绝的结果。
	_, terminalErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, orderID); !errors.Is(terminalErr, ErrNotFound) {
		t.Fatalf("terminal err=%v", terminalErr)
	}
}

// TestSuccessfulCloseRunProjectsPersistentTerminalState 验证平台终态消息缺失时，成功运行仍会隐藏历史动作且阻止重复关单。
func TestSuccessfulCloseRunProjectsPersistentTerminalState(t *testing.T) {
	// store、cleanup 是隔离 SQLite 数据库及释放函数。
	store, cleanup := newTestDB(t)
	defer cleanup()
	// ctx 是本测试全部数据库调用共用的上下文。
	ctx := context.Background()
	// userID 是测试卖家账号归属的本地用户主键。
	var userID int64
	if // userErr 是创建测试归属用户及读取主键的错误。
	userErr := store.DB.QueryRowContext(ctx, `INSERT INTO users (username,email,password_hash) VALUES (?,?,?) RETURNING id`, "close-projection-owner", "close-projection@example.com", "test-hash").Scan(&userID); userErr != nil {
		t.Fatal(userErr)
	}
	// accountID、successfulOrderID、failedOrderID 是测试账号和两个互不影响的订单号。
	const accountID, successfulOrderID, failedOrderID = "close-projection-account", "5127371642201267125", "5127371642201267999"
	if // accountErr 是创建测试卖家账号的错误。
	accountErr := store.Cookies.CreateOwned(ctx, accountID, "test-cookie", userID); accountErr != nil {
		t.Fatal(accountErr)
	}
	// eligibleOrderID 是仍允许关单失败重试的待付款订单号。
	const eligibleOrderID = failedOrderID
	if // orderErr 是创建待发货成功案例订单行的错误。
	orderErr := store.Orders.Upsert(ctx, successfulOrderID, OrderUpsertOpts{CookieID: accountID, OrderStatus: "pending_ship"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	if // orderErr 是创建待付款失败重试案例订单行的错误。
	orderErr := store.Orders.Upsert(ctx, eligibleOrderID, OrderUpsertOpts{CookieID: accountID, OrderStatus: "processing"}); orderErr != nil {
		t.Fatal(orderErr)
	}
	// session 是两张可取消订单卡片共用的测试会话。
	session := ChatSession{CookieID: accountID, ChatID: "close-projection-chat", BuyerID: "buyer-1"}
	// successfulPending 是已有明确成功运行、但没有平台关闭卡片的已付款待发货消息。
	successfulPending := ChatMessage{MessageKey: "paid-successful-close.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已付款，等待你发货", Status: "received", SentAt: 1000,
		SystemCardKind: "trade", SystemCardEvent: "order_paid", SystemCardTitle: "我已付款，等待你发货", SystemCardDescription: "等待卖家发货", SystemCardOrderID: successfulOrderID, SystemCardAction: "ship_order"}
	// failedPending 是明确失败后仍允许用户重新进入流程的另一笔待付款消息。
	failedPending := ChatMessage{MessageKey: "pending-failed-close.PNM", Direction: "incoming", SenderID: "buyer-1", MessageType: "system", Content: "我已拍下，待付款", Status: "received", SentAt: 2000,
		SystemCardKind: "trade", SystemCardEvent: "order_pending_payment", SystemCardTitle: "我已拍下，待付款", SystemCardDescription: "等待买家付款", SystemCardOrderID: failedOrderID, SystemCardAction: "adjust_price"}
	// pendingMessage 是当前待保存的测试卡片。
	for _, pendingMessage := range []ChatMessage{successfulPending, failedPending} {
		if // inserted、saveErr 表示当前待付款卡片是否首次保存及其错误。
		_, inserted, saveErr := store.Chats.SaveMessage(ctx, session, pendingMessage, false); saveErr != nil || !inserted {
			t.Fatalf("key=%s inserted=%v err=%v", pendingMessage.MessageKey, inserted, saveErr)
		}
	}
	// successfulRunKey、failedRunKey 是两笔关单运行的稳定幂等键。
	successfulRunKey, failedRunKey := "order_close:"+accountID+":"+successfulOrderID, "order_close:"+accountID+":"+failedOrderID
	// runCase 保存当前待创建运行的幂等键、订单号和终态。
	for _, runCase := range []struct {
		// key 是账号任务运行的稳定幂等键。
		key string
		// orderID 是运行关联的精确订单号。
		orderID string
		// status 是平台动作完成后的本地终态。
		status string
	}{{successfulRunKey, successfulOrderID, "success"}, {failedRunKey, failedOrderID, "failed"}} {
		// claimed、claimErr 表示测试运行是否首次抢占及其错误。
		claimed, claimErr := store.AccountTasks.ClaimRunImmediately(ctx, AccountTaskRun{RunKey: runCase.key, CookieID: accountID, TaskType: "order_close", TargetID: runCase.orderID, RunDate: "2026-08-20"}, 100)
		if claimErr != nil || !claimed {
			t.Fatalf("order=%s claimed=%v err=%v", runCase.orderID, claimed, claimErr)
		}
		// successCount、failedCount 反映当前运行终态的确定性计数。
		successCount, failedCount := 0, 1
		if runCase.status == "success" {
			successCount, failedCount = 1, 0
		}
		if // finishErr 是保存当前运行终态的错误。
		finishErr := store.AccountTasks.FinishRun(ctx, runCase.key, runCase.status, successCount, failedCount, "", 0); finishErr != nil {
			t.Fatal(finishErr)
		}
	}
	// messages、listErr 是刷新聊天历史时读取的只读投影。
	messages, listErr := store.Chats.ListMessages(ctx, userID, accountID, session.ChatID, 0, 20)
	if listErr != nil || len(messages) != 2 {
		t.Fatalf("messages=%+v err=%v", messages, listErr)
	}
	if messages[0].SystemCardEvent != "order_closed" || messages[0].SystemCardTitle != "订单已取消" || messages[0].SystemCardAction != "" || !strings.Contains(messages[0].SystemCardDescription, "原路退回") {
		t.Fatalf("successful projection=%+v", messages[0])
	}
	if messages[1].SystemCardEvent != "order_pending_payment" || messages[1].SystemCardAction != "adjust_price" {
		t.Fatalf("failed projection=%+v", messages[1])
	}
	if // _, handledErr 是成功运行后再次读取关单资格的拒绝结果。
	_, handledErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, successfulOrderID); !errors.Is(handledErr, ErrNotFound) {
		t.Fatalf("successful run eligibility err=%v", handledErr)
	}
	if // retryContext、retryErr 是明确失败后仍可重新进入的关单上下文。
	retryContext, retryErr := store.Chats.GetOwnedPendingOrderClose(ctx, userID, accountID, failedOrderID); retryErr != nil || retryContext.OrderID != failedOrderID {
		t.Fatalf("failed retry context=%+v err=%v", retryContext, retryErr)
	}
}
