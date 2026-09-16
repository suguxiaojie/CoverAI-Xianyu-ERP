package automation

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// TestExtractTaskFromWS_BuyerReviewed 封装TestExtract任务FromWS买家Reviewed业务协调。
func TestExtractTaskFromWS_BuyerReviewed(t *testing.T) {
	// raw 用于本次流程后续判断的原始
	raw := mustMap(t, `{
	  "1": {
	    "2": "62904549781@goofish",
	    "10": {
	      "redReminder": "有新交易评价",
	      "reminderContent": "[我完成了评价]",
	      "reminderTitle": "我完成了评价",
	      "senderUserId": "2222315258815",
	      "reminderUrl": "fleamarket://message_chat?itemId=1063217820795&peerUserId=2222315258815&sid=62904549781&messageId=abc&adv=no",
	      "extJson": "{\"updateKey\":\"62904549781:3310145690545023994:10:BUYER_RATE_SELLER:26\",\"contentType\":\"26\"}"
	    }
	  }
	}`)
	// task 用于本次流程后续判断的任务
	task := ExtractTaskFromWS("acc1", "cookie", raw)
	if task == nil {
		t.Fatal("评价卡片应解析为自动化事件")
	}
	if task.TriggerType != TriggerBuyerReviewed || task.OrderID != "3310145690545023994" ||
		task.ChatID != "62904549781" || task.ItemID != "1063217820795" || task.BuyerID != "2222315258815" {
		t.Fatalf("task=%+v", task)
	}
}

// TestExtractTaskFromWS_BuyerReviewedUsesBusinessKeyAcrossCopyVariants 封装TestExtract任务FromWS买家ReviewedUsesBusinessKeyAcrossCopyVariants业务协调。
func TestExtractTaskFromWS_BuyerReviewedUsesBusinessKeyAcrossCopyVariants(t *testing.T) {
	// raw 用于本次流程后续判断的原始
	raw := mustMap(t, `{
	  "1":{"2":"62904549781@goofish","10":{
	    "reminderContent":"感谢您的再次购买，评价已经完成",
	    "senderUserId":"buyer-2",
	    "reminderUrl":"fleamarket://message_chat?itemId=item-2&peerUserId=buyer-2&sid=62904549781",
	    "extJson":"{\"updateKey\":\"62904549781:order-second:10:buyer_rate_seller:26\",\"contentType\":\"26\"}"
	  }}
	}`)
	// task 用于本次流程后续判断的任务
	task := ExtractTaskFromWS("acc1", "cookie", raw)
	if task == nil || task.TriggerType != TriggerBuyerReviewed || task.OrderID != "order-second" {
		t.Fatalf("second-purchase review task=%+v", task)
	}
}

// TestExtractTaskFromWS_ServiceReviewInvitationIgnored 封装TestExtract任务FromWSServiceReviewInvitationIgnored业务协调。
func TestExtractTaskFromWS_ServiceReviewInvitationIgnored(t *testing.T) {
	// raw 用于本次流程后续判断的原始
	raw := mustMap(t, `{
	  "1": {
	    "2": "62854995941@goofish",
	    "10": {
	      "reminderContent": "为了给您提供更好的服务，诚邀您参与服务评价>>",
	      "reminderTitle": "闲小蜜发来一条新消息",
	      "senderUserId": "1400",
	      "extJson": "{\"messageId\":\"e5e96\"}"
	    }
	  }
	}`)
	if // task 用于本次流程后续判断的任务
	task := ExtractTaskFromWS("acc1", "cookie", raw); task != nil {
		t.Fatalf("服务评价邀请不能触发买家评价赠品: %+v", task)
	}
}

// TestExtractTaskFromWS_OrderPaid 封装TestExtract任务FromWS订单Paid业务协调。
func TestExtractTaskFromWS_OrderPaid(t *testing.T) {
	// raw 用于本次流程后续判断的原始
	raw := mustMap(t, `{
	  "1": {
	    "2": "63107041124@goofish",
	    "10": {
	      "redReminder": "等待卖家发货",
	      "reminderContent": "[我已付款，等待你发货]",
	      "senderUserId": "2222315258815",
	      "reminderUrl": "fleamarket://message_chat?itemId=1063177864132&peerUserId=2222315258815&sid=63107041124"
	    },
	    "6": {"3": {"5": "{\"dxCard\":{\"item\":{\"main\":{\"targetUrl\":\"fleamarket://order_detail?id=3310145690545023994&role=seller\"}}}}"}}
	  }
	}`)
	// task 用于本次流程后续判断的任务
	task := ExtractTaskFromWS("acc1", "cookie", raw)
	if task == nil || task.TriggerType != TriggerOrderPaid || task.OrderID != "3310145690545023994" || task.OrderStatus != "pending_ship" {
		t.Fatalf("task=%+v", task)
	}
}

// TestExtractTaskFromWSOrderPaidDecodesBase64Card 验证付款卡片订单号只存在于 Base64 JSON 时仍能进入订单事实链。
func TestExtractTaskFromWSOrderPaidDecodesBase64Card(t *testing.T) {
	// encodedCard 是模拟实测嵌套卡片的 Base64 JSON，订单号只存在于卖家详情目标地址。
	encodedCard := base64.StdEncoding.EncodeToString([]byte(`{"dxCard":{"item":{"main":{"title":"我已付款，等待你发货","description":"等待卖家发货","button":{"targetUrl":"fleamarket://order_detail?bizOrderId=3316373186236021980&role=seller"}}}}}`))
	// raw 是展示摘要可识别付款事件、但普通顶层字段没有订单号的实时信封。
	raw := map[string]any{"1": map[string]any{"2": "65724359531@goofish", "10": map[string]any{
		"reminderContent": "[我已付款，等待你发货]", "senderUserId": "1033798067",
		"extJson": `{"contentType":"26"}`, "customData": encodedCard,
	}}}
	// task 是解开 Base64 卡片后带精确订单关联的付款事实。
	task := ExtractTaskFromWS("seller-account", "cookie", raw)
	if task == nil || task.TriggerType != TriggerOrderPaid || task.OrderID != "3316373186236021980" || task.BuyerID != "1033798067" || task.OrderStatus != "pending_ship" {
		t.Fatalf("task=%+v", task)
	}
}

// TestExtractTaskFromWSPendingPayment 验证卖家侧待付款卡片会产生可落库的订单事实。
func TestExtractTaskFromWSPendingPayment(t *testing.T) {
	// raw 是带卖家订单详情地址的待付款卡片夹具。
	raw := mustMap(t, `{
	  "1": {
	    "2": "63107041124@goofish",
	    "10": {
	      "redReminder": "等待买家付款",
	      "reminderContent": "[我已拍下，待付款]",
	      "senderUserId": "2222315258815",
	      "extJson": "{\"contentType\":26}"
	    },
	    "6": {"3": {"5": "{\"dxCard\":{\"item\":{\"main\":{\"targetUrl\":\"fleamarket://order_detail?id=3310145690545023995&role=seller\"}}}}"}}
	  }
	}`)
	// task 是从卡片提取的待付款订单事实。
	task := ExtractTaskFromWS("acc1", "cookie", raw)
	if task == nil || task.TriggerType != TriggerOrderPendingPayment || task.OrderID != "3310145690545023995" || task.OrderStatus != "processing" {
		t.Fatalf("task=%+v", task)
	}
}

// TestExtractTaskFromWSBuyerPendingPaymentIgnored 验证买家侧待付款卡片不会污染卖家订单表。
func TestExtractTaskFromWSBuyerPendingPaymentIgnored(t *testing.T) {
	// raw 是明确标记 role=buyer 的待付款卡片夹具。
	raw := mustMap(t, `{
	  "1": {
	    "2": "63107041124@goofish",
	    "10": {"redReminder":"等待买家付款","reminderContent":"[我已拍下，待付款]","senderUserId":"2222315258815","extJson":"{\"contentType\":26}"},
	    "6": {"3": {"5": "{\"dxCard\":{\"item\":{\"main\":{\"targetUrl\":\"fleamarket://order_detail?id=3310145690545023995&role=buyer\"}}}}"}}
	  }
	}`)
	// task 是买家侧卡片的解析结果，正确行为必须为空。
	task := ExtractTaskFromWS("acc1", "cookie", raw)
	if task != nil {
		t.Fatalf("买家侧待付款卡片不应进入卖家订单事实: %+v", task)
	}
}

// TestExtractTaskFromWS_BuyerOrderPaidIgnored 封装TestExtract任务FromWS买家订单PaidIgnored业务协调。
func TestExtractTaskFromWS_BuyerOrderPaidIgnored(t *testing.T) {
	// raw 用于本次流程后续判断的原始
	raw := mustMap(t, `{
	  "1": {
	    "2": "63107041124@goofish",
	    "10": {
	      "redReminder": "等待卖家发货",
	      "reminderContent": "[我已付款，等待你发货]",
	      "senderUserId": "2222315258815",
	      "reminderUrl": "fleamarket://message_chat?itemId=1063177864132&peerUserId=2222315258815&sid=63107041124"
	    },
	    "6": {"3": {"5": "{\"dxCard\":{\"item\":{\"main\":{\"targetUrl\":\"fleamarket://order_detail?id=3310145690545023994&role=buyer\"}}}}"}}
	  }
	}`)
	if // task 用于本次流程后续判断的任务
	task := ExtractTaskFromWS("acc1", "cookie", raw); task != nil {
		t.Fatalf("买家订单不应进入卖家自动化和订单管理: %+v", task)
	}
}

// TestExtractTaskFromWS_BuyerOrderPaidTaskNameIgnored 封装TestExtract任务FromWS买家订单Paid任务名称Ignored业务协调。
func TestExtractTaskFromWS_BuyerOrderPaidTaskNameIgnored(t *testing.T) {
	// raw 用于本次流程后续判断的原始
	raw := mustMap(t, `{
	  "1": {"2": "63107041124@goofish", "10": {
	    "bizTag": "{\"taskName\":\"付款完成待发货_买家\"}",
	    "redReminder": "等待卖家发货",
	    "reminderContent": "[我已付款，等待你发货]"
	  }}
	}`)
	if // task 用于本次流程后续判断的任务
	task := ExtractTaskFromWS("acc1", "cookie", raw); task != nil {
		t.Fatalf("买家侧 taskName 不应进入卖家自动化和订单管理: %+v", task)
	}
}

// TestExtractTaskFromWSSeparatesReceiptCompletionCancellationAndRefund 验证订单生命周期事件不会再合并为 completed 或 cancelled。
func TestExtractTaskFromWSSeparatesReceiptCompletionCancellationAndRefund(t *testing.T) {
	// cases 保存平台展示文本、稳定更新键和预期触发状态。
	cases := []struct {
		// text 是当前系统消息展示文本。
		text string
		// updateKey 是包含会话和订单号的兼容业务键；退款成功普通通知故意留空订单号。
		updateKey string
		// trigger 是应进入事实记录器的稳定事件。
		trigger string
		// status 是事件明确表达的订单状态。
		status string
		// orderID 是解析后允许关联的精确订单号。
		orderID string
	}{
		{"买家已确认收货", "chat-life:5127000000000000001:order_received:26", TriggerOrderReceived, "received", "5127000000000000001"},
		{"交易成功", "chat-life:5127000000000000002:order_completed:26", TriggerOrderCompleted, "completed", "5127000000000000002"},
		{"我发起了退款申请", "chat-life:5127000000000000003:refund_requested:26", TriggerRefundRequested, "refunding", "5127000000000000003"},
		{"退款成功，钱款已原路退返", "", TriggerRefundCompleted, "refunded", ""},
		{"未付款，你关闭了订单", "chat-life:5127000000000000004:order_cancelled:26", TriggerOrderCancelled, "cancelled", "5127000000000000004"},
	}
	// testCase 是当前待解析的生命周期事件夹具。
	for _, testCase := range cases {
		// extension 是当前夹具的协议扩展，保留明确 contentType。
		extension := `{"contentType":"26","updateKey":"` + testCase.updateKey + `"}`
		// raw 是最小实时系统消息信封。
		raw := map[string]any{"1": map[string]any{"2": "chat-life@goofish", "10": map[string]any{"reminderContent": testCase.text, "extJson": extension}}}
		// task 是从当前平台消息解析出的订单生命周期事实。
		task := ExtractTaskFromWS("cid", "cookie", raw)
		if task == nil || task.TriggerType != testCase.trigger || task.OrderStatus != testCase.status || task.OrderID != testCase.orderID {
			t.Fatalf("text=%s task=%+v", testCase.text, task)
		}
	}
}

// TestExtractTaskFromWSIgnoresReceiptReminderSystemCard 验证官方提醒收货卡片不会写成买家已确认收货。
func TestExtractTaskFromWSIgnoresReceiptReminderSystemCard(t *testing.T) {
	// raw 是真实提醒卡片的最小结构化形态，标题和说明均不能推进订单状态。
	raw := map[string]any{"1": map[string]any{"2": "receipt-chat@goofish", "10": map[string]any{
		"reminderContent": "记得及时确认收货", "detailNotice": "等待买家收货",
		"extJson": `{"contentType":"26","updateKey":"receipt-chat:5127000000000000010:REMIND_BUYER_TO_CONFIRM:26"}`,
	}}}
	if // task 是提醒卡片尝试解析出的自动化任务，预期为空。
	task := ExtractTaskFromWS("cid", "cookie", raw); task != nil {
		t.Fatalf("提醒收货系统卡片不应进入订单事实自动化: %+v", task)
	}
}

// TestExtractTaskFromWS_RedFlowerEvents 验证送花卡片和后续收花结果按账号会话关联进入专用协调器。
func TestExtractTaskFromWS_RedFlowerEvents(t *testing.T) {
	// sentRaw 模拟实测载荷：卡片明确表示买家送花并提供卖家收取地址，但兼容 updateKey 同时含 received_red_flower。
	sentRaw := map[string]any{"1": map[string]any{"2": "chat-flower@goofish", "10": map[string]any{
		"reminderContent": "[你人真不错，送你闲鱼小红花]", "extJson": `{"contentType":"26","updateKey":"chat-flower:received_red_flower:26"}`,
		"dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
			"title": "你人真不错，送你闲鱼小红花", "button": map[string]any{"targetUrl": "https://h5.m.goofish.com/wow/moyu/moyu-project/temp-pages/pages/red-flower-play?role=seller&orderId=5127372398162002704"},
		}}}},
	}}}
	// sentTask 是解析出的自动收花触发任务。
	sentTask := ExtractTaskFromWS("cid", "cookie", sentRaw)
	if sentTask == nil || sentTask.TriggerType != TriggerRedFlowerSent || sentTask.ChatID != "chat-flower" || sentTask.OrderID != "5127372398162002704" {
		t.Fatalf("sent task=%+v", sentTask)
	}
	// receivedRaw 是不含订单号但带稳定 updateKey 的收花完成结果。
	receivedRaw := map[string]any{"1": map[string]any{"2": "chat-flower@goofish", "10": map[string]any{
		"reminderContent": "[收到小红花，心里乐开花！]", "extJson": `{"contentType":"26","updateKey":"chat-flower:received_red_flower:26"}`,
	}}}
	// receivedTask 是按同会话返回的收花结果任务。
	receivedTask := ExtractTaskFromWS("cid", "cookie", receivedRaw)
	if receivedTask == nil || receivedTask.TriggerType != TriggerRedFlowerReceived || receivedTask.ChatID != "chat-flower" {
		t.Fatalf("received task=%+v", receivedTask)
	}
}

// TestExtractTaskFromWS_UserFlowerTextIgnored 验证普通用户文本即使复述送花标题也不会误触发自动收花。
func TestExtractTaskFromWS_UserFlowerTextIgnored(t *testing.T) {
	// raw 是没有 contentType=26 的普通聊天文本载荷。
	raw := map[string]any{"1": map[string]any{"2": "chat-flower@goofish", "10": map[string]any{
		"reminderContent": "你人真不错，送你闲鱼小红花",
	}}}
	// task 是普通文本尝试解析得到的自动化任务，预期为空。
	task := ExtractTaskFromWS("cid", "cookie", raw)
	if task != nil {
		t.Fatalf("普通用户文本不能触发自动收花: %+v", task)
	}
}

// TestExtractTaskFromWS_OrderShippedWithoutOrderID 验证真实“你已发货”卡片保留会话和平台时间供唯一候选关联。
func TestExtractTaskFromWS_OrderShippedWithoutOrderID(t *testing.T) {
	// occurredAt 是测试发货卡片的平台 Unix 毫秒时间。
	occurredAt := int64(1787157648709)
	// raw 是没有 order_id、但有 chat_id 和 contentType=14 的真实发货卡片形态。
	raw := map[string]any{"1": map[string]any{
		"2": "65691803069@goofish", "5": occurredAt,
		"10": map[string]any{"reminderContent": "[你已发货]", "detailNotice": "等待买家收货", "extJson": `{"contentType":"14"}`},
	}}
	// task 是供 Center 执行唯一候选关联的发货事件。
	task := ExtractTaskFromWS("account-1", "cookie", raw)
	if task == nil || task.TriggerType != TriggerOrderShipped || task.OrderID != "" || task.ChatID != "65691803069" || task.OrderStatus != "shipped" || task.OccurredAt != occurredAt {
		t.Fatalf("task=%+v", task)
	}
}

// mustMap 封装mustMap业务协调。
func mustMap(t *testing.T, s string) map[string]any {
	t.Helper()
	// m 用于本次流程后续判断的m
	var m map[string]any
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
