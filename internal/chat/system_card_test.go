package chat

import "testing"

// TestParseSystemCardClassifiesTradeStates 验证交易标题只映射为允许公开的稳定事件。
func TestParseSystemCardClassifiesTradeStates(t *testing.T) {
	// testCases 覆盖待付款后的主要只读交易状态和未知降级。
	testCases := []struct {
		// title 是平台卡片标题。
		title string
		// expectedEvent 是结构化后应得到的稳定事件。
		expectedEvent string
	}{
		{title: "我已修改价格，等待你付款", expectedEvent: "order_price_adjusted"},
		{title: "我已付款，等待你发货", expectedEvent: "order_paid"},
		{title: "你已发货", expectedEvent: "order_shipped"},
		{title: "记得及时确认收货", expectedEvent: "receipt_confirmation_reminded"},
		{title: "买家已确认收货", expectedEvent: "order_received"},
		{title: "交易成功", expectedEvent: "order_completed"},
		{title: "超时未付款，系统关闭了订单", expectedEvent: "order_cancelled"},
		{title: "买家申请退款", expectedEvent: "refund_requested"},
		{title: "退款成功，钱款已原路退返", expectedEvent: "refund_completed"},
		{title: "退款关闭", expectedEvent: "unknown_trade_event"},
		{title: "可以送我闲鱼小红花吗～", expectedEvent: "red_flower_requested"},
		{title: "卖家人不错？送Ta闲鱼小红花", expectedEvent: "red_flower_prompted"},
		{title: "你人真不错，送你闲鱼小红花", expectedEvent: "red_flower_sent"},
		{title: "收到小红花，心里乐开花！", expectedEvent: "red_flower_received"},
		{title: "新的交易状态", expectedEvent: "unknown_trade_event"},
	}
	// testCase 是当前待验证的交易状态夹具。
	for _, testCase := range testCases {
		t.Run(testCase.title, func(t *testing.T) {
			// raw 是只含非操作交易状态的 contentType=26 载荷。
			raw := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{"title": testCase.title, "desc": "交易说明"}}}}}
			// card、ok 保存结构化解析结果。
			card, ok := parseSystemCard(raw, "")
			if !ok || card.Event != testCase.expectedEvent || card.Action != "" {
				t.Fatalf("card=%+v ok=%v", card, ok)
			}
		})
	}
}

// TestParseSystemCardExposesShipmentOnlyForSellerOrderDetail 验证同一付款文案只有卖家角色订单详情地址能够生成发货动作。
func TestParseSystemCardExposesShipmentOnlyForSellerOrderDetail(t *testing.T) {
	// sellerRaw 是闲鱼卖家客服当前实际付款卡片形态，按钮本身只显示“已付款”但目标明确携带 role=seller。
	sellerRaw := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "我已付款，等待你发货", "desc": "请包装好商品，并按我在闲鱼上提供的地址发货",
		"button": map[string]any{"text": "已付款", "targetUrl": "fleamarket://order_detail?id=5127372002248048713&role=seller"},
	}}}}}
	// sellerCard、sellerOK 保存卖家侧结构化结果。
	sellerCard, sellerOK := parseSystemCard(sellerRaw, "")
	if !sellerOK || sellerCard.Event != "order_paid" || sellerCard.OrderID != "5127372002248048713" || sellerCard.Action != "ship_order" {
		t.Fatalf("seller card=%+v ok=%v", sellerCard, sellerOK)
	}
	// buyerRaw 复用相同订单和文案，但平台明确标记 role=buyer，绝不能暴露卖家发货动作。
	buyerRaw := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "我已付款，等待你发货", "desc": "等待卖家发货",
		"button": map[string]any{"text": "已付款", "targetUrl": "fleamarket://order_detail?id=5127372002248048713&role=buyer"},
	}}}}}
	// buyerCard、buyerOK 保存买家侧只读结果。
	buyerCard, buyerOK := parseSystemCard(buyerRaw, "")
	if !buyerOK || buyerCard.Event != "order_paid" || buyerCard.Action != "" {
		t.Fatalf("buyer card=%+v ok=%v", buyerCard, buyerOK)
	}
}

// TestParseSystemCardKeepsShipmentReminderPending 验证“如已发货”条件说明不能冒充卖家已经发货。
func TestParseSystemCardKeepsShipmentReminderPending(t *testing.T) {
	// reminderRaw 复现正式消息 8009 的标题、条件说明、订单号和“去发货”按钮形态。
	reminderRaw := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "记得及时发货", "desc": "如已发货，请点击「去发货」输入快递单号", "orderId": "3316800937226128994",
		"button": map[string]any{"text": "去发货", "targetUrl": "https://h5.m.goofish.com/app/idleFish-F2e/fm-delivery/home.html?orderId=3316800937226128994"},
	}}}}}
	// reminderCard、reminderOK 是发货提醒解析结果；它仍属于付款后待发货状态。
	reminderCard, reminderOK := parseSystemCard(reminderRaw, "记得及时发货")
	if !reminderOK || reminderCard.Event != "order_paid" || reminderCard.OrderID != "3316800937226128994" {
		t.Fatalf("reminder card=%+v ok=%v", reminderCard, reminderOK)
	}
	// shippedRaw 是同订单明确完成发货的正向对照。
	shippedRaw := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "你已发货", "desc": "等待买家收货", "orderId": "3316800937226128994",
	}}}}}
	// shippedCard、shippedOK 是明确发货完成卡片解析结果。
	shippedCard, shippedOK := parseSystemCard(shippedRaw, "你已发货")
	if !shippedOK || shippedCard.Event != "order_shipped" {
		t.Fatalf("shipped card=%+v ok=%v", shippedCard, shippedOK)
	}
}

// TestParseSystemCardAcceptsOfficialRedFlowerPromptType25 验证平台自动提示使用 contentType=25 时仍按 updateKey 安全分类。
func TestParseSystemCardAcceptsOfficialRedFlowerPromptType25(t *testing.T) {
	// prompt 是平台自动生成的买家送花提示，只有明确 sys_want_red_flower 才允许升级。
	prompt := map[string]any{
		"extJson": `{"contentType":"25","updateKey":"chat-1:order-1:sys_want_red_flower"}`,
		"dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
			"title": "卖家人不错？送Ta闲鱼小红花", "desc": "买卖换真心，鼓励Ta一下",
		}}}},
	}
	// card、ok 是小红花平台提示的结构化结果。
	card, ok := parseSystemCard(prompt, "卖家人不错？送Ta闲鱼小红花")
	if !ok || card.ContentType != 25 || card.Event != "red_flower_prompted" {
		t.Fatalf("card=%+v ok=%v", card, ok)
	}
}

// TestParseSystemCardExposesOnlyOfficialReceiveFlowerAction 验证送花卡片携带订单时才提供官方收花入口提示。
func TestParseSystemCardExposesOnlyOfficialReceiveFlowerAction(t *testing.T) {
	// sentCard 模拟实测送花卡片；兼容 updateKey 不能覆盖明确的送花标题、订单号和卖家收取动作。
	sentCard := map[string]any{"contentType": 26, "extJson": `{"contentType":"26","updateKey":"chat-flower:received_red_flower:26"}`, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "你人真不错，送你闲鱼小红花", "desc": "买卖换真心，鼓励你一下~",
		"button": map[string]any{"text": "立即收下", "targetUrl": "https://h5.m.goofish.com/wow/moyu/moyu-project/temp-pages/pages/red-flower-play?role=seller&orderId=5127372398162002704"},
	}}}}}
	// sent、sentOK 是送花卡片解析结果。
	sent, sentOK := parseSystemCard(sentCard, "")
	if !sentOK || sent.Event != "red_flower_sent" || sent.OrderID != "5127372398162002704" || sent.Action != "receive_red_flower" {
		t.Fatalf("sent=%+v ok=%v", sent, sentOK)
	}
	// receivedCard 是已经收花后的结果卡片，不应继续提供动作。
	receivedCard := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "收到小红花，心里乐开花！", "desc": "谢谢支持！",
	}}}}}
	// received、receivedOK 是收花结果解析结果。
	received, receivedOK := parseSystemCard(receivedCard, "")
	if !receivedOK || received.Event != "red_flower_received" || received.Action != "" {
		t.Fatalf("received=%+v ok=%v", received, receivedOK)
	}
}

// TestParseSystemCardRejectsNoticeAndProductTitle 验证普通通知不会升级卡片，商品标题也不会替代事件标题。
func TestParseSystemCardRejectsNoticeAndProductTitle(t *testing.T) {
	// notice 是普通 contentType=14 平台提示。
	notice := map[string]any{"contentType": 14, "title": "普通提示"}
	if // card、ok 是普通通知的解析结果，必须保持未识别。
	card, ok := parseSystemCard(notice, "普通提示"); ok {
		t.Fatalf("contentType=14 不应生成交易卡片: %+v", card)
	}
	// trade 同时含商品标题和交易展示层，解析器必须选择 exContent.title。
	trade := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{
		"title": "商品标题不能当事件", "main": map[string]any{"exContent": map[string]any{"title": "我已付款，等待你发货", "desc": "请及时处理订单"}},
	}}}
	// card、ok 保存包含商品标题干扰项的解析结果。
	card, ok := parseSystemCard(trade, "摘要")
	if !ok || card.Title != "我已付款，等待你发货" || card.Event != "order_paid" {
		t.Fatalf("交易展示层标题优先级错误: %+v ok=%v", card, ok)
	}
}

// TestParseSystemCardPromotesExplicitShippedNotice 验证真实 contentType=14“你已发货”升级为无操作交易卡片。
func TestParseSystemCardPromotesExplicitShippedNotice(t *testing.T) {
	// shippedNotice 模拟真实平台扩展：明确发货摘要、等待收货说明和商品关联地址。
	shippedNotice := map[string]any{
		"extJson":      `{"contentType":"14"}`,
		"detailNotice": "[你已发货]", "redReminder": "等待买家收货",
		"reminderUrl": "fleamarket://message_chat?itemId=item-shipped&sid=chat-1",
	}
	// card、ok 保存明确发货通知的结构化结果。
	card, ok := parseSystemCard(shippedNotice, "[你已发货]")
	if !ok || card.ContentType != 14 || card.Event != "order_shipped" || card.Title != "你已发货" ||
		card.Description != "等待买家收货" || card.ItemID != "item-shipped" || card.OrderID != "" || card.Action != "" {
		t.Fatalf("发货通知未安全升级为交易卡片: card=%+v ok=%v", card, ok)
	}
}

// TestParseSystemCardPromotesExplicitPlainLifecycleNotices 验证无订单号普通系统文本只升级明确退款成功与普通取消终态。
func TestParseSystemCardPromotesExplicitPlainLifecycleNotices(t *testing.T) {
	// refundCard、refundOK 是真实退款成功普通通知的只读卡片投影。
	refundCard, refundOK := parseSystemCard(map[string]any{}, "[退款成功，钱款已原路退返]")
	if !refundOK || refundCard.Event != "refund_completed" || refundCard.Title != "退款成功" || refundCard.OrderID != "" {
		t.Fatalf("refund card=%+v ok=%v", refundCard, refundOK)
	}
	// cancelledCard、cancelledOK 是未付款卖家取消普通通知的只读卡片投影。
	cancelledCard, cancelledOK := parseSystemCard(map[string]any{}, "[未付款，你关闭了订单]")
	if !cancelledOK || cancelledCard.Event != "order_cancelled" || cancelledCard.Title != "订单已取消" || cancelledCard.OrderID != "" {
		t.Fatalf("cancelled card=%+v ok=%v", cancelledCard, cancelledOK)
	}
	if // _, ambiguousOK 表示普通“退款”文本不能被误判为退款成功终态。
	_, ambiguousOK := parseSystemCard(map[string]any{}, "退款处理中"); ambiguousOK {
		t.Fatal("普通退款处理中通知不应升级为无订单号终态")
	}
}

// TestParseSystemCardPreservesLargeNumericIdentifiers 验证嵌套 JSON 中超过 JavaScript 安全整数范围的订单号仍按原始十进制文本解析。
func TestParseSystemCardPreservesLargeNumericIdentifiers(t *testing.T) {
	// raw 是平台可能以 JSON 数字而非字符串返回订单号和商品号的真实形态。
	raw := `{"contentType":26,"dxCard":{"item":{"main":{"exContent":{"title":"我已修改价格，等待你付款","desc":"等待买家付款","orderId":5127638256187075541,"itemId":1074702895033}}}}}`
	// card、ok 保存保留超长整数精度后的结构化结果。
	card, ok := parseSystemCard(raw, "")
	if !ok || card.OrderID != "5127638256187075541" || card.ItemID != "1074702895033" || card.Event != "order_price_adjusted" {
		t.Fatalf("超长数字标识发生精度损失: card=%+v ok=%v", card, ok)
	}
}

// TestParseSystemCardFallsBackFromUnsafeFloatIdentifier 验证上游已转成不安全 float64 时不持久化伪订单号，而是使用动作 URL 中的精确文本。
func TestParseSystemCardFallsBackFromUnsafeFloatIdentifier(t *testing.T) {
	// raw 模拟外层 WS 解码已经把直接订单字段变为不安全浮点，但按钮 URL 仍保留精确标识。
	raw := map[string]any{"contentType": 26, "dxCard": map[string]any{"item": map[string]any{"main": map[string]any{"exContent": map[string]any{
		"title": "我已拍下，待付款", "desc": "请双方沟通及时确认价格", "orderId": float64(5127638256187075541),
		"button": map[string]any{"text": "修改价格", "targetUrl": "fleamarket://adjust_price?bizOrderId=5127638256187075541"},
	}}}}}
	// card、ok 保存从精确 URL 回退得到的待付款卡片。
	card, ok := parseSystemCard(raw, "")
	if !ok || card.OrderID != "5127638256187075541" || card.Event != "order_pending_payment" || card.Action != "adjust_price" {
		t.Fatalf("不安全浮点没有回退到精确 URL: card=%+v ok=%v", card, ok)
	}
	if cleanSystemCardIdentifier("5.127638256187076e+18") != "" {
		t.Fatal("科学计数法订单号不应作为稳定标识持久化")
	}
}
