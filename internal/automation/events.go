// Package automation 实现自动化处理中心。
//
// 重要边界：
//   - engine 只负责 WS 消息连接和分流，不在分流层判断业务规则。
//   - 用户消息进入关键词/AI 回复链；系统卡片和平台通知只进入自动化中心。
//   - 自动化中心把 WS 事件、计划任务、后台手动任务统一转换为 Task，再匹配规则和执行动作。
package automation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"xianyu-go/internal/db"
)

// TriggerOrderPaid 用于本次流程后续判断的Trigger订单Paid
const (
	// TriggerOrderPendingPayment 表示卖家侧收到买家已下单但尚未付款的交易事实。
	TriggerOrderPendingPayment  = "order_pending_payment"
	TriggerOrderPaid            = "order_paid"
	TriggerOrderShipped         = "order_shipped"
	TriggerOrderReceived        = "order_received"
	TriggerOrderCompleted       = "order_completed"
	TriggerOrderCancelled       = "order_cancelled"
	TriggerRefundRequested      = "refund_requested"
	TriggerRefundCompleted      = "refund_completed"
	TriggerBuyerReviewed        = "buyer_reviewed"
	TriggerReviewMissingTimeout = "review_missing_timeout"
	TriggerRedFlowerSent        = "red_flower_sent"
	TriggerRedFlowerReceived    = "red_flower_received"
	TriggerRedFlowerRequestDue  = "red_flower_request_due"

	ActionConfirmShipment = "confirm_shipment"
	ActionSendCard        = "send_card"
	ActionSendText        = "send_text"
)

// Task 是自动化中心的统一输入。它可以来自 WS 系统事件、计划任务或手动触发。
type Task struct {
	Source      string // ws/scheduler/manual
	AccountID   string
	CookieStr   string
	TriggerType string
	ChatID      string
	OrderID     string
	ItemID      string
	BuyerID     string
	SpecName    string
	SpecValue   string
	Quantity    string
	Amount      string
	OrderStatus string
	Text        string
	UpdateKey   string
	// OccurredAt 是平台消息发生时间的 Unix 毫秒；缺失时业务层使用本机接收时间。
	OccurredAt int64
	// ForceConfirmShipment 仅供明确的人工“完整发货”使用；自动事件仍遵循账号自动确认开关。
	ForceConfirmShipment bool
	// ActionPlan 是运行创建时冻结的动作计划。延迟恢复和失败重试必须使用该快照，
	// 不能把数字游标应用到管理员后来修改过的规则上。
	ActionPlan []db.AutomationAction
	Raw        map[string]any
}

// OrderDetail 是自动化中心执行交易类任务前需要补齐的订单事实。
// 规格和数量来自闲鱼订单，不由自动化规则修改商品属性。
// OrderDetail 用于本次流程后续判断的订单Detail
type OrderDetail struct {
	Quantity    string
	SpecName    string
	SpecValue   string
	Amount      string
	OrderStatus string
}

// ExtractTaskFromWS 从一条解密后的 WS 消息中提取系统事件。
// 这里只做事实解析：识别平台告诉了我们什么；是否执行自动化由 Center 根据规则决定。
// ExtractTaskFromWS 封装Extract任务FromWS业务协调。
func ExtractTaskFromWS(accountID, cookieStr string, raw map[string]any) *Task {
	if raw == nil {
		return nil
	}
	// f 用于本次流程后续判断的f
	f := fieldsFromRaw(raw)
	if f.text == "" && f.redReminder == "" && f.updateKey == "" && f.title == "" && f.detail == "" {
		return nil
	}
	// task 用于本次流程后续判断的任务
	task := &Task{
		Source:     "ws",
		AccountID:  accountID,
		CookieStr:  cookieStr,
		ChatID:     f.chatID,
		OrderID:    f.orderID,
		ItemID:     f.itemID,
		BuyerID:    f.buyerID,
		Text:       firstNonEmpty(f.text, f.redReminder, f.title, f.detail),
		UpdateKey:  f.updateKey,
		OccurredAt: rawEventUnixMilli(raw),
		Raw:        raw,
	}
	if isReceiptConfirmationReminderEvent(f) {
		return nil
	}
	switch {
	case isRedFlowerSentEvent(f):
		task.TriggerType = TriggerRedFlowerSent
	case isRedFlowerReceivedEvent(f):
		task.TriggerType = TriggerRedFlowerReceived
		// 收花结果 updateKey 的第二段可能是动作名而非订单号，只按账号和会话匹配 pending 运行。
		task.OrderID = ""
	case isOrderShippedEvent(f):
		task.TriggerType = TriggerOrderShipped
		task.OrderStatus = "shipped"
	case isOrderReceivedEvent(f):
		task.TriggerType = TriggerOrderReceived
		task.OrderStatus = "received"
	case isOrderCompletedEvent(f):
		task.TriggerType = TriggerOrderCompleted
		task.OrderStatus = "completed"
	case isRefundCompletedEvent(f):
		task.TriggerType = TriggerRefundCompleted
		task.OrderStatus = "refunded"
	case isRefundRequestedEvent(f):
		task.TriggerType = TriggerRefundRequested
		task.OrderStatus = "refunding"
	case isOrderCancelledEvent(f):
		task.TriggerType = TriggerOrderCancelled
		task.OrderStatus = "cancelled"
	case isOrderPendingPaymentEvent(f):
		task.TriggerType = TriggerOrderPendingPayment
		task.OrderStatus = "processing"
	case isOrderPaidEvent(f):
		task.TriggerType = TriggerOrderPaid
		task.OrderStatus = "pending_ship"
	case isBuyerReviewedEvent(f):
		task.TriggerType = TriggerBuyerReviewed
	default:
		return nil
	}
	return task
}

// isOrderPendingPaymentEvent 只识别卖家侧带明确订单号的待付款卡片，买家侧订单不写入卖家订单表。
func isOrderPendingPaymentEvent(fields rawFields) bool {
	if fields.orderRole == "buyer" || strings.TrimSpace(fields.orderID) == "" || strings.TrimSpace(fields.contentType) != "26" {
		return false
	}
	// combined 只汇总交易卡片的结构化展示文案，不从普通用户消息推断订单状态。
	combined := automationEventText(fields)
	return strings.Contains(combined, "我已拍下，待付款") || strings.Contains(combined, "等待买家付款")
}

// rawEventUnixMilli 从实时紧凑信封读取平台时间，并统一为 Unix 毫秒。
func rawEventUnixMilli(raw map[string]any) int64 {
	if raw == nil {
		return 0
	}
	// m1 是实时消息紧凑信封主体。
	m1 := mapAt(raw, "1")
	if m1 == nil {
		return 0
	}
	// candidates 按稳定信封字段优先级保存时间候选。
	candidates := []any{m1["5"]}
	if // m10 是实时消息展示扩展对象。
	m10 := mapAt(m1, "10"); m10 != nil {
		candidates = append(candidates, m10["sendTime"], m10["timestamp"], m10["createdAt"])
	}
	// candidate 是当前待解析的秒或毫秒时间值。
	for _, candidate := range candidates {
		// text 是时间候选的标量文本。
		text := strings.TrimSpace(strAny(candidate))
		if text == "" || text == "null" {
			continue
		}
		// value、parseErr 是候选的整数时间和解析错误。
		value, parseErr := strconv.ParseInt(strings.Trim(text, `"`), 10, 64)
		if parseErr != nil || value <= 0 {
			continue
		}
		if value < 10_000_000_000 {
			value *= 1000
		}
		return value
	}
	return 0
}

// rawFields 用于本次流程后续判断的原始字段列表
type rawFields struct {
	text        string
	redReminder string
	title       string
	detail      string
	orderRole   string
	updateKey   string
	contentType string
	chatID      string
	orderID     string
	itemID      string
	buyerID     string
	reminderURL string
	// flowerText 汇总卡片树中的展示标量，只用于识别小红花事件。
	flowerText string
}

// fieldsFromRaw 封装字段列表From原始业务协调。
func fieldsFromRaw(raw map[string]any) rawFields {
	// f 用于本次流程后续判断的f
	var f rawFields
	if // m1 用于本次流程后续判断的m1
	m1 := mapAt(raw, "1"); m1 != nil {
		if // s 用于本次流程后续判断的s
		s := strAny(m1["2"]); s != "" {
			f.chatID = trimGoofishSID(s)
		}
		if // m10 用于本次流程后续判断的m10
		m10 := mapAt(m1, "10"); m10 != nil {
			f.text = strAny(m10["reminderContent"])
			f.redReminder = strAny(m10["redReminder"])
			f.title = strAny(m10["reminderTitle"])
			f.detail = strAny(m10["detailNotice"])
			f.reminderURL = strAny(m10["reminderUrl"])
			f.buyerID = strAny(m10["senderUserId"])
			f.updateKey, f.contentType = extFields(strAny(m10["extJson"]))
			f.orderRole = orderRoleFromTaskName(bizTaskName(strAny(m10["bizTag"])))
		}
		if // contentJSON 用于本次流程后续判断的内容JSON
		contentJSON := nestedString(raw, "1", "6", "3", "5"); contentJSON != "" {
			if // role 用于本次流程后续判断的role
			role := extractOrderRoleFromContent(contentJSON); role != "" {
				f.orderRole = role
			}
			if // id 用于本次流程后续判断的标识
			id := extractOrderIDFromContent(contentJSON); id != "" {
				f.orderID = id
			}
		}
	}
	if // m3 用于本次流程后续判断的m3
	m3 := mapAt(raw, "3"); m3 != nil {
		if f.redReminder == "" {
			f.redReminder = strAny(m3["redReminder"])
		}
	}
	if // m4 用于本次流程后续判断的m4
	m4 := mapAt(raw, "4"); m4 != nil {
		if f.text == "" {
			f.text = strAny(m4["reminderContent"])
		}
		if f.redReminder == "" {
			f.redReminder = strAny(m4["redReminder"])
		}
		if f.title == "" {
			f.title = strAny(m4["reminderTitle"])
		}
		if f.detail == "" {
			f.detail = strAny(m4["detailNotice"])
		}
		if f.reminderURL == "" {
			f.reminderURL = strAny(m4["reminderUrl"])
		}
		if f.updateKey == "" {
			f.updateKey, f.contentType = extFields(strAny(m4["extJson"]))
		}
	}
	// 结构化卡片字段和官方动作 URL 比兼容 updateKey 更能证明订单归属，必须先完成收集。
	// decodedRaw 展开卡片内直接 JSON 与 Base64 JSON，使自动化事实和 Chat 卡片使用相同的订单关联能力。
	decodedRaw := decodeAutomationCardValue(raw, 0)
	collectFlowerAutomationFields(decodedRaw, &f, 0)
	if f.updateKey != "" {
		// chatID、orderID 用于本次流程后续判断的聊天ID、orderID
		chatID, orderID := parseUpdateKey(f.updateKey)
		if f.chatID == "" {
			f.chatID = chatID
		}
		if f.orderID == "" {
			f.orderID = orderID
		}
	}
	if f.reminderURL != "" {
		if f.itemID == "" {
			f.itemID = queryValue(f.reminderURL, "itemId")
		}
		if f.buyerID == "" {
			f.buyerID = queryValue(f.reminderURL, "peerUserId")
		}
		if f.chatID == "" {
			f.chatID = queryValue(f.reminderURL, "sid")
		}
		if f.orderID == "" {
			f.orderID = matchOrderID(f.reminderURL)
		}
	}
	if f.reminderURL != "" {
		if f.orderID == "" {
			f.orderID = firstNonEmpty(queryValue(f.reminderURL, "orderId"), queryValue(f.reminderURL, "bizOrderId"), queryValue(f.reminderURL, "id"), matchOrderID(f.reminderURL))
		}
		if f.chatID == "" {
			f.chatID = queryValue(f.reminderURL, "sid")
		}
	}
	return f
}

// decodeAutomationCardValue 在有限深度内展开系统卡片中的 JSON 与 Base64 JSON，同时保留长订单号原文。
func decodeAutomationCardValue(value any, depth int) any {
	if depth > 12 {
		return value
	}
	switch typed := value.(type) { // typed 是当前递归层待展开的协议值。
	case string:
		// text 是去空白后的候选编码；普通展示文本保持原值。
		text := strings.TrimSpace(typed)
		if text == "" {
			return typed
		}
		if decodedJSON, decoded := decodeAutomationCardJSON([]byte(text)); decoded { // decodedJSON、decoded 是直接 JSON 的解析结果和成功标记。
			return decodeAutomationCardValue(decodedJSON, depth+1)
		}
		if decodedBytes, decodeErr := base64.StdEncoding.DecodeString(text); decodeErr == nil { // decodedBytes、decodeErr 是 Base64 文本的字节结果和错误。
			if decodedJSON, decoded := decodeAutomationCardJSON(decodedBytes); decoded { // decodedJSON、decoded 是 Base64 内层 JSON 的解析结果和成功标记。
				return decodeAutomationCardValue(decodedJSON, depth+1)
			}
		}
		return typed
	case map[string]any:
		// decodedMap 保存展开后的新映射，不修改 WebSocket 原始载荷。
		decodedMap := make(map[string]any, len(typed))
		for key, child := range typed { // key、child 是当前协议字段及其待展开子值。
			decodedMap[key] = decodeAutomationCardValue(child, depth+1)
		}
		return decodedMap
	case []any:
		// decodedItems 保存展开后的新列表，不修改调用方切片。
		decodedItems := make([]any, len(typed))
		for index, child := range typed { // index、child 是当前列表位置及其待展开子值。
			decodedItems[index] = decodeAutomationCardValue(child, depth+1)
		}
		return decodedItems
	default:
		return value
	}
}

// decodeAutomationCardJSON 使用 json.Number 解码卡片片段，避免十九位订单号经过 float64 丢失精度。
func decodeAutomationCardJSON(raw []byte) (any, bool) {
	// decoder 是只处理当前候选片段并保留数字原文的 JSON 解码器。
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	// decoded 保存当前片段的解析结果。
	var decoded any
	if decodeErr := decoder.Decode(&decoded); decodeErr != nil { // decodeErr 表示当前候选不是完整 JSON。
		return nil, false
	}
	return decoded, true
}

// collectFlowerAutomationFields 从实时卡片的有限协议树中补齐小红花事件和订单关联字段。
func collectFlowerAutomationFields(value any, fields *rawFields, depth int) {
	if fields == nil || depth > 16 {
		return
	}
	switch // typed 是当前递归层待提取的小红花协议值。
	typed := value.(type) {
	case map[string]any:
		// key 是当前需要汇总的小红花展示字段名称。
		for _, key := range []string{"reminderTitle", "title", "detailNotice", "desc", "description", "reminderContent", "redReminder"} {
			// text 是当前协议字段的可分类标量。
			text := cleanAutomationScalar(typed[key])
			if text != "" && len(fields.flowerText) < 4096 {
				fields.flowerText += " " + text
			}
		}
		if fields.title == "" {
			fields.title = firstNonEmpty(cleanAutomationScalar(typed["reminderTitle"]), cleanAutomationScalar(typed["title"]))
		}
		if fields.detail == "" {
			fields.detail = firstNonEmpty(cleanAutomationScalar(typed["detailNotice"]), cleanAutomationScalar(typed["desc"]), cleanAutomationScalar(typed["description"]))
		}
		if fields.text == "" {
			fields.text = cleanAutomationScalar(typed["reminderContent"])
		}
		if fields.redReminder == "" {
			fields.redReminder = cleanAutomationScalar(typed["redReminder"])
		}
		if fields.updateKey == "" {
			fields.updateKey = cleanAutomationScalar(typed["updateKey"])
		}
		if fields.contentType == "" {
			fields.contentType = cleanAutomationScalar(typed["contentType"])
		}
		if fields.orderID == "" {
			fields.orderID = firstNonEmpty(cleanAutomationScalar(typed["bizOrderId"]), cleanAutomationScalar(typed["orderId"]))
		}
		if fields.itemID == "" {
			fields.itemID = cleanAutomationScalar(typed["itemId"])
		}
		if fields.buyerID == "" {
			fields.buyerID = cleanAutomationScalar(typed["senderUserId"])
		}
		if fields.reminderURL == "" {
			fields.reminderURL = firstNonEmpty(cleanAutomationScalar(typed["targetUrl"]), cleanAutomationScalar(typed["reminderUrl"]))
		}
		// targetURL 是当前协议节点可能提供的官方动作地址。
		targetURL := firstNonEmpty(cleanAutomationScalar(typed["targetUrl"]), cleanAutomationScalar(typed["reminderUrl"]))
		if targetURL != "" {
			if fields.orderID == "" {
				fields.orderID = firstNonEmpty(queryValue(targetURL, "orderId"), queryValue(targetURL, "bizOrderId"), queryValue(targetURL, "id"), matchOrderID(targetURL))
			}
			if fields.chatID == "" {
				fields.chatID = queryValue(targetURL, "sid")
			}
		}
		if // extJSON 是当前协议节点可能包含的扩展 JSON 文本。
		extJSON := cleanAutomationScalar(typed["extJson"]); extJSON != "" {
			// decoded 是当前扩展 JSON 解码后的协议对象。
			var decoded any
			if json.Unmarshal([]byte(extJSON), &decoded) == nil {
				collectFlowerAutomationFields(decoded, fields, depth+1)
			}
		}
		// child 是当前映射中继续递归检查的协议子值。
		for _, child := range typed {
			collectFlowerAutomationFields(child, fields, depth+1)
		}
	case []any:
		// child 是当前列表中继续递归检查的协议子值。
		for _, child := range typed {
			collectFlowerAutomationFields(child, fields, depth+1)
		}
	}
}

// cleanAutomationScalar 把协议标量转换为可用于事件分类的去空白字符串。
func cleanAutomationScalar(value any) string {
	switch value.(type) {
	case nil, map[string]any, []any:
		return ""
	default:
		return strings.TrimSpace(strAny(value))
	}
}

// isRedFlowerSentEvent 判断买家已经送出小红花且卖家可进入收花流程。
func isRedFlowerSentEvent(fields rawFields) bool {
	if strings.TrimSpace(fields.contentType) != "26" {
		return false
	}
	// combined 是只用于小红花事件分类的展示文本。
	combined := fields.flowerText + " " + fields.title + " " + fields.detail + " " + fields.text + " " + fields.redReminder
	return strings.Contains(combined, "你人真不错") && strings.Contains(combined, "送你闲鱼小红花")
}

// isRedFlowerReceivedEvent 判断平台已经发出收花完成结果；明确送花标题优先于兼容 updateKey，避免把待收取卡片误判为完成。
func isRedFlowerReceivedEvent(fields rawFields) bool {
	if strings.TrimSpace(fields.contentType) != "26" {
		return false
	}
	if isRedFlowerSentEvent(fields) {
		return false
	}
	// combined 是只用于收花完成分类的展示文本。
	combined := fields.flowerText + " " + fields.title + " " + fields.detail + " " + fields.text + " " + fields.redReminder
	return strings.Contains(strings.ToLower(fields.updateKey), "received_red_flower") || strings.Contains(combined, "收到小红花")
}

// isOrderPaidEvent 封装is订单PaidEvent业务协调。
func isOrderPaidEvent(f rawFields) bool {
	if f.orderRole == "buyer" {
		return false
	}
	return strings.Contains(f.text, "我已付款，等待你发货") ||
		strings.Contains(f.text, "已付款，待发货") ||
		strings.Contains(f.text, "记得及时发货") ||
		strings.Contains(f.redReminder, "等待卖家发货")
}

// isOrderShippedEvent 判断卖家侧明确发货系统卡片；订单关联由 Center 再做所有权和唯一候选校验。
func isOrderShippedEvent(fields rawFields) bool {
	if fields.contentType != "14" && fields.contentType != "26" {
		return false
	}
	// combined 是只用于发货事实分类的结构化展示文本。
	combined := fields.flowerText + " " + fields.title + " " + fields.detail + " " + fields.text + " " + fields.redReminder
	return strings.Contains(combined, "你已发货") || strings.Contains(combined, "我已发货，请查看发货凭证")
}

// automationEventText 汇总系统卡片展示字段，只用于订单生命周期事实分类。
func automationEventText(fields rawFields) string {
	return fields.flowerText + " " + fields.title + " " + fields.detail + " " + fields.text + " " + fields.redReminder
}

// isOrderReceivedEvent 判断买家明确确认收货；物流签收文本不属于该事件。
func isOrderReceivedEvent(fields rawFields) bool {
	if fields.orderRole == "buyer" {
		return false
	}
	// combined 是只用于确认收货事实分类的展示文本。
	combined := automationEventText(fields)
	return strings.Contains(combined, "确认收货") && !strings.Contains(combined, "记得及时确认收货") && !strings.Contains(combined, "等待买家收货") && !strings.Contains(combined, "物流") && !strings.Contains(combined, "快递")
}

// isReceiptConfirmationReminderEvent 判断卖家提醒买家收货的官方系统卡片，禁止冒充买家已确认收货事实。
func isReceiptConfirmationReminderEvent(fields rawFields) bool {
	if strings.TrimSpace(fields.contentType) != "26" {
		return false
	}
	// combined 是提醒卡片的标题、说明和摘要组合，只用于排除订单状态推进。
	combined := automationEventText(fields)
	return strings.Contains(combined, "记得及时确认收货") || strings.Contains(combined, "等待买家收货")
}

// isOrderCompletedEvent 判断平台明确宣告交易成功或交易完成。
func isOrderCompletedEvent(fields rawFields) bool {
	if fields.orderRole == "buyer" {
		return false
	}
	// combined 是只用于交易完成事实分类的展示文本。
	combined := automationEventText(fields)
	return strings.Contains(combined, "交易成功") || strings.Contains(combined, "交易完成")
}

// isRefundCompletedEvent 判断平台明确退款成功或钱款原路退回；该判断必须优先于普通退款申请。
func isRefundCompletedEvent(fields rawFields) bool {
	// combined 是只用于退款完成事实分类的展示文本。
	combined := automationEventText(fields)
	return strings.Contains(combined, "退款成功") || strings.Contains(combined, "已退款") || strings.Contains(combined, "钱款已原路退返")
}

// isRefundRequestedEvent 判断订单进入退款流程，但排除已经成功的退款终态。
func isRefundRequestedEvent(fields rawFields) bool {
	if fields.orderRole == "buyer" || isRefundCompletedEvent(fields) {
		return false
	}
	// combined 是只用于退款申请事实分类的展示文本。
	combined := automationEventText(fields)
	return strings.Contains(combined, "退款")
}

// isOrderCancelledEvent 判断普通取消或关闭订单，并排除退款分支。
func isOrderCancelledEvent(fields rawFields) bool {
	if fields.orderRole == "buyer" {
		return false
	}
	// combined 是只用于普通取消事实分类的展示文本。
	combined := automationEventText(fields)
	return !strings.Contains(combined, "退款") && (strings.Contains(combined, "取消") || strings.Contains(combined, "关闭了订单") || strings.Contains(combined, "交易关闭") || strings.Contains(combined, "系统关闭"))
}

// isBuyerReviewedEvent 封装is买家ReviewedEvent业务协调。
func isBuyerReviewedEvent(f rawFields) bool {
	// 闲鱼评价样本：
	//   redReminder=有新交易评价
	//   reminderContent=[我完成了评价]
	//   updateKey=chat_id:order_id:10:BUYER_RATE_SELLER:26
	// 仅“服务评价邀请”不含 BUYER_RATE_SELLER，不能误触发赠品。
	// BUYER_RATE_SELLER 是交易评价的稳定业务标识。展示文案会因客户端版本、
	// 同一买家重复购买等场景变化，不能再把两段中文文案同时存在作为必要条件。
	return strings.Contains(strings.ToUpper(f.updateKey), "BUYER_RATE_SELLER")
}

// extFields 封装ext字段列表业务协调。
func extFields(ext string) (updateKey, contentType string) {
	if strings.TrimSpace(ext) == "" {
		return "", ""
	}
	// m 用于本次流程后续判断的m
	var m map[string]any
	if json.Unmarshal([]byte(ext), &m) != nil {
		return "", ""
	}
	return strAny(m["updateKey"]), strAny(m["contentType"])
}

// parseUpdateKey 封装parseUpdateKey业务协调。
func parseUpdateKey(updateKey string) (chatID, orderID string) {
	// parts 用于本次流程后续判断的parts
	parts := strings.Split(updateKey, ":")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// queryValue 封装查询值业务协调。
func queryValue(rawURL, key string) string {
	if strings.HasPrefix(rawURL, "fleamarket://") {
		rawURL = "https://local.invalid/" + strings.TrimPrefix(rawURL, "fleamarket://")
	}
	// u、err 用于本次流程后续判断的u、err
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// mapAt 封装mapAt业务协调。
func mapAt(m map[string]any, key string) map[string]any {
	// v 用于本次流程后续判断的v
	v, _ := m[key].(map[string]any)
	return v
}

// nestedString 封装nestedString业务协调。
func nestedString(m map[string]any, path ...string) string {
	// cur 用于本次流程后续判断的cur
	var cur any = m
	// p 表示当前遍历过程中的p
	for _, p := range path {
		// cm、ok 用于本次流程后续判断的cm、ok
		cm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = cm[p]
	}
	return strAny(cur)
}

// strAny 封装strAny业务协调。
func strAny(v any) string {
	switch // x 用于本次流程后续判断的x
	x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		// b 用于本次流程后续判断的b
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// firstNonEmpty 封装firstNonEmpty业务协调。
func firstNonEmpty(values ...string) string {
	// v 表示当前遍历过程中的v
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// trimGoofishSID 封装trimGoofishSID业务协调。
func trimGoofishSID(s string) string {
	if // i 用于本次流程后续判断的i
	i := strings.Index(s, "@"); i >= 0 {
		return s[:i]
	}
	return s
}

// orderIDPatterns 只接受明确订单参数；通用 id 必须从单词边界开始，禁止把 sid 的尾部误识别为订单号。
var orderIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`orderId[=:](\d{10,})`),
	regexp.MustCompile(`order_detail\?id=(\d{10,})`),
	regexp.MustCompile(`bizOrderId[=:](\d{10,})`),
	regexp.MustCompile(`\bid=(\d{10,})`),
}

// extractOrderRoleFromContent 封装extract订单RoleFrom内容业务协调。
func extractOrderRoleFromContent(contentJSON string) string {
	// c 用于本次流程后续判断的c
	var c map[string]any
	if json.Unmarshal([]byte(contentJSON), &c) != nil {
		return ""
	}
	// path 表示当前遍历过程中的路径
	for _, path := range [][]string{
		{"dxCard", "item", "main", "exContent", "button", "targetUrl"},
		{"dxCard", "item", "main", "targetUrl"},
		{"dynamicOperation", "changeContent", "dxCard", "item", "main", "exContent", "button", "targetUrl"},
	} {
		if // role 用于本次流程后续判断的role
		role := orderRoleFromURL(nestedString(c, path...)); role != "" {
			return role
		}
	}
	return ""
}

// orderRoleFromURL 封装订单RoleFromURL业务协调。
func orderRoleFromURL(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "fleamarket://") {
		rawURL = "https://local.invalid/" + strings.TrimPrefix(rawURL, "fleamarket://")
	}
	// u、err 用于本次流程后续判断的u、err
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(u.Query().Get("role"))) {
	case "seller", "buyer":
		return strings.ToLower(strings.TrimSpace(u.Query().Get("role")))
	default:
		return ""
	}
}

// bizTaskName 封装biz任务名称业务协调。
func bizTaskName(raw string) string {
	// tag 用于本次流程后续判断的tag
	var tag map[string]any
	if json.Unmarshal([]byte(raw), &tag) != nil {
		return ""
	}
	return strAny(tag["taskName"])
}

// orderRoleFromTaskName 封装订单RoleFrom任务名称业务协调。
func orderRoleFromTaskName(taskName string) string {
	switch {
	case strings.Contains(taskName, "买家"):
		return "buyer"
	case strings.Contains(taskName, "卖家"):
		return "seller"
	default:
		return ""
	}
}

// matchOrderID 封装match订单ID业务协调。
func matchOrderID(s string) string {
	// re 表示当前遍历过程中的re
	for _, re := range orderIDPatterns {
		if // m 用于本次流程后续判断的m
		m := re.FindStringSubmatch(s); len(m) == 2 {
			return m[1]
		}
	}
	return ""
}

// extractOrderIDFromContent 封装extract订单IDFrom内容业务协调。
func extractOrderIDFromContent(contentJSON string) string {
	// c 用于本次流程后续判断的c
	var c map[string]any
	if json.Unmarshal([]byte(contentJSON), &c) != nil {
		return ""
	}
	// path 表示当前遍历过程中的路径
	for _, path := range [][]string{
		{"dxCard", "item", "main", "exContent", "button", "targetUrl"},
		{"dxCard", "item", "main", "targetUrl"},
		{"dynamicOperation", "changeContent", "dxCard", "item", "main", "exContent", "button", "targetUrl"},
	} {
		if // id 用于本次流程后续判断的标识
		id := matchOrderID(nestedString(c, path...)); id != "" {
			return id
		}
	}
	return ""
}
