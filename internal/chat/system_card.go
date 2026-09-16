package chat

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"xianyu-go/internal/db"
)

// systemCard 保存从平台交易与小红花卡片提取的非敏感字段，不保留平台原始载荷。
type systemCard struct {
	// ContentType 是平台协议消息类型；交易主要为 26，小红花平台提示可为 25。
	ContentType int
	// Kind 当前固定为 trade，为未来只读系统卡片保留明确分类。
	Kind string
	// Event 是不依赖中文 UI 的归一化交易状态。
	Event string
	// Title 是平台提供的交易卡片标题。
	Title string
	// Description 是平台提供的交易状态说明。
	Description string
	// OrderID 是从明确字段或平台动作地址取得的订单标识。
	OrderID string
	// ItemID 是从明确字段或平台动作地址取得的商品标识。
	ItemID string
	// Action 是归一化动作提示；只允许前端已知的改价、卖家发货或官方收花入口。
	Action string
}

// ClassifySystemEvent 从实时或历史平台载荷返回稳定系统事件；未知或非系统卡片返回空字符串。
func ClassifySystemEvent(raw any, fallback string) string {
	// card、ok 是系统卡片解析结果和有效标记。
	card, ok := parseSystemCard(raw, fallback)
	if !ok || card.Event == "unknown_trade_event" {
		return ""
	}
	return card.Event
}

// systemCardCandidate 收集递归遍历中的平台字段，最终只投影允许持久化的非敏感值。
type systemCardCandidate struct {
	// updateKey 是平台交易动作的稳定更新标识，仅用于事件分类。
	updateKey string
	// title 是优先级最高的 exContent.title。
	title string
	// description 是优先级最高的 exContent.desc。
	description string
	// buttonText 是平台卡片按钮文本，仅用于动作归一化。
	buttonText string
	// targetURLs 保存卡片和按钮明确提供的动作地址。
	targetURLs []string
	// orderID 是显式 orderId 或 bizOrderId 字段。
	orderID string
	// itemID 是显式 itemId 字段。
	itemID string
}

// applySystemCard 把解析结果复制到持久化模型；调用方继续负责消息方向、时间和状态。
func applySystemCard(message *db.ChatMessage, card systemCard) {
	if message == nil {
		return
	}
	message.PlatformContentType = card.ContentType
	message.SystemCardKind = card.Kind
	message.SystemCardEvent = card.Event
	message.SystemCardTitle = card.Title
	message.SystemCardDescription = card.Description
	message.SystemCardOrderID = card.OrderID
	message.SystemCardItemID = card.ItemID
	message.SystemCardAction = card.Action
}

// parseSystemCard 递归解析历史与实时协议载荷；接受交易、小红花卡片及明确的 contentType=14 发货状态。
func parseSystemCard(raw any, fallback string) (systemCard, bool) {
	// decodedRoot 是展开 JSON/base64 字符串后的可遍历协议根对象。
	decodedRoot := decodeSystemCardValue(raw, 0)
	// contentType 是平台明确提供的消息协议类型。
	contentType := findOfficialContentType(decodedRoot)
	// candidate 收集结构化卡片所需的最小平台字段。
	candidate := systemCardCandidate{}
	collectSystemCardCandidate(decodedRoot, &candidate, 0)
	// plainNotice 是移除平台摘要括号后的普通系统通知文本；只升级语义完全明确的退款成功和普通取消终态。
	plainNotice := strings.Trim(strings.TrimSpace(fallback), "[]【】")
	if contentType == "" || contentType == "0" || contentType == "14" {
		switch {
		case strings.Contains(plainNotice, "退款成功") && strings.Contains(plainNotice, "原路退返"):
			return systemCard{ContentType: 0, Kind: "trade", Event: "refund_completed", Title: "退款成功", Description: plainNotice}, true
		case strings.Contains(plainNotice, "关闭了订单") && !strings.Contains(plainNotice, "退款"):
			return systemCard{ContentType: 0, Kind: "trade", Event: "order_cancelled", Title: "订单已取消", Description: plainNotice}, true
		}
	}
	if contentType == "14" {
		// noticeTitle 是移除平台摘要括号后的明确交易状态标题。
		noticeTitle := strings.Trim(strings.TrimSpace(fallback), "[]【】")
		if noticeTitle != "你已发货" {
			return systemCard{}, false
		}
		// itemID 只从明确字段或提醒地址提取；该通知没有订单 ID 时保持空值。
		itemID := strings.TrimSpace(candidate.itemID)
		// actionURL 是当前待解析商品关联的平台提醒地址。
		for _, actionURL := range candidate.targetURLs {
			// _, parsedItemID, _ 是提醒地址携带的商品关联；不把聊天或消息 ID 当订单 ID。
			_, parsedItemID, _ := parseSystemCardTarget(actionURL)
			if itemID == "" {
				itemID = parsedItemID
			}
		}
		return systemCard{ContentType: 14, Kind: "trade", Event: "order_shipped", Title: noticeTitle,
			Description: limitedSystemCardText(candidate.description, 1000), ItemID: limitedSystemCardText(itemID, 255)}, true
	}
	if contentType != "26" && contentType != "25" {
		return systemCard{}, false
	}
	// title 优先采用平台卡片标题，缺失时使用已经归一化的系统消息摘要。
	title := limitedSystemCardText(firstSystemCardText(candidate.title, fallback), 255)
	if title == "" {
		return systemCard{}, false
	}
	// orderID 和 itemID 先使用明确字段，再从平台动作地址解析。
	orderID := strings.TrimSpace(candidate.orderID)
	// itemID 保存平台卡片关联商品标识；空值允许只关联订单。
	itemID := strings.TrimSpace(candidate.itemID)
	// actionURL 是当前遍历的卡片或按钮动作地址。
	// sellerShipmentTarget 表示平台明确把订单详情动作标记为卖家角色，买家侧同文案不能据此发货。
	sellerShipmentTarget := false
	// actionURL 是当前待解析的平台动作地址。
	for _, actionURL := range candidate.targetURLs {
		// parsedOrderID、parsedItemID、parsedAction 是当前地址投影出的安全业务字段。
		parsedOrderID, parsedItemID, parsedAction := parseSystemCardTarget(actionURL)
		if orderID == "" {
			orderID = parsedOrderID
		}
		if itemID == "" {
			itemID = parsedItemID
		}
		if parsedAction == "adjust_price" {
			candidate.buttonText = firstSystemCardText(candidate.buttonText, "修改价格")
		}
		if parsedAction == "ship_order" {
			sellerShipmentTarget = true
		}
	}
	// event 使用标题和说明归一化，无法确定时保留 unknown_trade_event 而不是猜测状态。
	event := classifySystemCardEvent(title, candidate.description, candidate.updateKey)
	if contentType == "25" && event != "red_flower_prompted" {
		return systemCard{}, false
	}
	// action 只有待付款卡片携带明确改价按钮或动作地址时才提供。
	action := ""
	if event == "order_pending_payment" && strings.Contains(candidate.buttonText, "修改价格") && orderID != "" {
		action = "adjust_price"
	}
	if event == "red_flower_sent" && orderID != "" {
		action = "receive_red_flower"
	}
	if event == "order_paid" && sellerShipmentTarget && orderID != "" {
		action = "ship_order"
	}
	return systemCard{
		ContentType: map[string]int{"25": 25, "26": 26}[contentType],
		Kind:        "trade",
		Event:       event,
		Title:       title,
		Description: limitedSystemCardText(candidate.description, 1000),
		OrderID:     limitedSystemCardText(orderID, 255),
		ItemID:      limitedSystemCardText(itemID, 255),
		Action:      action,
	}, true
}

// decodeSystemCardValue 在有限深度内展开 JSON 和 base64 JSON 字符串，兼容实时 WS 与历史 custom.data。
func decodeSystemCardValue(value any, depth int) any {
	if depth > 12 {
		return value
	}
	switch // typed 是当前递归层展开后的协议值类型。
	typed := value.(type) {
	case string:
		// text 是去空白后的候选编码；普通展示文本保持原样。
		text := strings.TrimSpace(typed)
		if text == "" {
			return typed
		}
		if // decodedJSON、decoded 表示直接 JSON 字符串在保留超长整数精度后的展开结果。
		decodedJSON, decoded := decodeSystemCardJSON([]byte(text)); decoded {
			return decodeSystemCardValue(decodedJSON, depth+1)
		}
		// decodedBytes 保存历史 custom.data 的 base64 解码内容；只有内部仍是 JSON 才采用。
		if decodedBytes, decodeErr := base64.StdEncoding.DecodeString(text); decodeErr == nil {
			if // decodedJSON、decoded 表示 base64 内层 JSON 在保留超长整数精度后的展开结果。
			decodedJSON, decoded := decodeSystemCardJSON(decodedBytes); decoded {
				return decodeSystemCardValue(decodedJSON, depth+1)
			}
		}
		return typed
	case map[string]any:
		// result 是展开所有子字段后的新映射，不修改调用方持有的原始协议对象。
		result := make(map[string]any, len(typed))
		// key、child 表示当前待展开的平台字段和值。
		for key, child := range typed {
			result[key] = decodeSystemCardValue(child, depth+1)
		}
		return result
	case []any:
		// result 是展开所有列表成员后的新切片。
		result := make([]any, len(typed))
		// index、child 表示当前待展开的列表位置和值。
		for index, child := range typed {
			result[index] = decodeSystemCardValue(child, depth+1)
		}
		return result
	default:
		return value
	}
}

// decodeSystemCardJSON 使用 json.Number 解码平台卡片 JSON；raw 是待展开的原始字节，返回值保留订单号的十进制文本和解析成功状态。
func decodeSystemCardJSON(raw []byte) (any, bool) {
	// decoder 只用于当前卡片片段，并通过 UseNumber 避免 19 位订单号先经过 float64 丢失精度。
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	// decoded 保存保留数字原文的协议树，不持久化未筛选的原始载荷。
	var decoded any
	if // decodeErr 表示当前片段不是合法 JSON；调用方将继续按普通展示文本处理。
	decodeErr := decoder.Decode(&decoded); decodeErr != nil {
		return nil, false
	}
	return decoded, true
}

// collectSystemCardCandidate 从解码协议树中提取卡片展示字段和明确关联标识。
func collectSystemCardCandidate(value any, candidate *systemCardCandidate, depth int) {
	if candidate == nil || depth > 16 {
		return
	}
	switch // typed 是当前递归层待提取字段的协议值类型。
	typed := value.(type) {
	case map[string]any:
		if candidate.updateKey == "" {
			candidate.updateKey = cleanSystemCardValue(typed["updateKey"])
		}
		// title 是当前对象可能直接承载的卡片展示标题。
		title := cleanSystemCardValue(typed["title"])
		// description 是当前对象可能直接承载的卡片展示说明。
		description := firstSystemCardText(cleanSystemCardValue(typed["desc"]), cleanSystemCardValue(typed["description"]), cleanSystemCardValue(typed["redReminder"]))
		// _, hasButton 限定带按钮的交易卡片展示层，避免把商品标题误当系统事件标题。
		_, hasButton := typed["button"]
		// _, hasDescription 限定带说明的交易卡片展示层。
		_, hasDescription := typed["desc"]
		if !hasDescription {
			_, hasDescription = typed["description"]
		}
		if candidate.title == "" && title != "" && (hasButton || hasDescription) {
			candidate.title = title
		}
		if candidate.description == "" && description != "" {
			candidate.description = description
		}
		if candidate.orderID == "" {
			candidate.orderID = firstSystemCardText(cleanSystemCardIdentifier(typed["bizOrderId"]), cleanSystemCardIdentifier(typed["orderId"]))
		}
		if candidate.itemID == "" {
			candidate.itemID = cleanSystemCardIdentifier(typed["itemId"])
		}
		if // targetURL 是当前卡片层明确提供的平台动作地址。
		targetURL := cleanSystemCardValue(typed["targetUrl"]); targetURL != "" {
			candidate.targetURLs = append(candidate.targetURLs, targetURL)
		}
		if // reminderURL 是 contentType=14 平台提醒携带的只读聊天／商品地址。
		reminderURL := cleanSystemCardValue(typed["reminderUrl"]); reminderURL != "" {
			candidate.targetURLs = append(candidate.targetURLs, reminderURL)
		}
		if // button、ok 是当前对象的卡片按钮及其结构判断结果。
		button, ok := typed["button"].(map[string]any); ok {
			if candidate.buttonText == "" {
				candidate.buttonText = cleanSystemCardValue(button["text"])
			}
			if // targetURL 是当前按钮明确提供的平台动作地址。
			targetURL := cleanSystemCardValue(button["targetUrl"]); targetURL != "" {
				candidate.targetURLs = append(candidate.targetURLs, targetURL)
			}
		}
		// child 表示当前递归检查的协议子对象。
		for _, child := range typed {
			collectSystemCardCandidate(child, candidate, depth+1)
		}
	case []any:
		// child 表示当前递归检查的协议列表成员。
		for _, child := range typed {
			collectSystemCardCandidate(child, candidate, depth+1)
		}
	}
}

// parseSystemCardTarget 只从平台 URL 查询参数读取订单、商品和卖家动作，不执行或打开该地址。
func parseSystemCardTarget(rawTarget string) (orderID, itemID, action string) {
	// parsedTarget 是经过标准库解析的平台动作地址。
	parsedTarget, parseErr := url.Parse(strings.TrimSpace(rawTarget))
	if parseErr != nil {
		return "", "", ""
	}
	// query 保存平台动作地址中的显式关联参数。
	query := parsedTarget.Query()
	orderID = firstSystemCardText(query.Get("bizOrderId"), query.Get("orderId"), query.Get("id"))
	itemID = query.Get("itemId")
	if strings.EqualFold(parsedTarget.Host, "adjust_price") || strings.Contains(strings.ToLower(parsedTarget.Path), "adjust_price") {
		action = "adjust_price"
	}
	// sellerOrderDetail 表示闲鱼明确提供的卖家订单详情入口；只有该角色证据才允许付款卡片生成发货动作。
	sellerOrderDetail := strings.EqualFold(parsedTarget.Scheme, "fleamarket") && strings.EqualFold(parsedTarget.Host, "order_detail") && strings.EqualFold(query.Get("role"), "seller")
	if sellerOrderDetail {
		action = "ship_order"
	}
	return strings.TrimSpace(orderID), strings.TrimSpace(itemID), action
}

// classifySystemCardEvent 把平台动作标识和标题映射为稳定事件；小红花优先于普通交易文本分类。
func classifySystemCardEvent(title, description, updateKey string) string {
	// combined 是仅用于分类的展示文本，不进入日志或外部请求。
	combined := strings.TrimSpace(title + " " + description)
	// normalizedTitle、normalizedDescription 分开保留标题和说明，避免条件说明中的“如已发货”冒充完成事实。
	normalizedTitle, normalizedDescription := strings.TrimSpace(title), strings.TrimSpace(description)
	// normalizedUpdateKey 是统一大小写的小红花动作标识。
	normalizedUpdateKey := strings.ToLower(strings.TrimSpace(updateKey))
	switch {
	case strings.Contains(combined, "你人真不错") && strings.Contains(combined, "送你闲鱼小红花"):
		// 实测卖家待收取卡片也可能携带 received_red_flower updateKey；明确送花标题必须优先保留收花入口。
		return "red_flower_sent"
	case strings.Contains(combined, "收到小红花") || strings.Contains(normalizedUpdateKey, "received_red_flower"):
		return "red_flower_received"
	case strings.Contains(normalizedUpdateKey, "sys_want_red_flower") || (strings.Contains(combined, "卖家人不错") && strings.Contains(combined, "送Ta闲鱼小红花")):
		return "red_flower_prompted"
	case strings.Contains(normalizedUpdateKey, "want_red_flower") || strings.Contains(combined, "可以送我闲鱼小红花") || strings.Contains(combined, "已求买家送我闲鱼小红花"):
		return "red_flower_requested"
	case strings.Contains(combined, "修改价格") && strings.Contains(combined, "付款"):
		return "order_price_adjusted"
	case strings.Contains(combined, "已付款") || strings.Contains(combined, "等待你发货") || strings.Contains(combined, "等待您发货"):
		return "order_paid"
	case strings.Contains(normalizedTitle, "记得及时发货") || (strings.Contains(normalizedDescription, "如已发货") && strings.Contains(normalizedDescription, "去发货")):
		return "order_paid"
	case normalizedTitle == "你已发货" || strings.Contains(normalizedTitle, "我已发货，请查看发货凭证"):
		return "order_shipped"
	case strings.Contains(normalizedTitle, "记得及时确认收货") || strings.Contains(normalizedDescription, "等待买家收货"):
		return "receipt_confirmation_reminded"
	case strings.Contains(combined, "确认收货"):
		return "order_received"
	case strings.Contains(combined, "交易成功") || strings.Contains(combined, "交易完成"):
		return "order_completed"
	case strings.Contains(combined, "退款关闭"):
		// 退款关闭可能表示申请撤销或拒绝，缺少订单当前状态时不能猜成退款成功或普通取消。
		return "unknown_trade_event"
	case strings.Contains(combined, "退款成功") || strings.Contains(combined, "已退款") || strings.Contains(combined, "钱款已原路退返"):
		return "refund_completed"
	case strings.Contains(combined, "退款"):
		return "refund_requested"
	case strings.Contains(combined, "关闭") || strings.Contains(combined, "取消"):
		return "order_cancelled"
	case strings.Contains(combined, "待付款") && (strings.Contains(combined, "拍下") || strings.Contains(combined, "等待付款")):
		return "order_pending_payment"
	default:
		return "unknown_trade_event"
	}
}

// cleanSystemCardValue 将平台标量转换为非空字符串，跳过 map、slice 和协议 nil 占位。
func cleanSystemCardValue(value any) string {
	switch value.(type) {
	case map[string]any, []any:
		return ""
	}
	// text 是当前平台标量的去空白表示。
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" || text == "null" {
		return ""
	}
	return text
}

// cleanSystemCardIdentifier 把订单／商品标识转换为精确文本；已经超出安全整数范围的浮点值返回空值，以便后续从平台 URL 或保留原文的 json.Number 补齐。
func cleanSystemCardIdentifier(value any) string {
	switch // typed 是当前待规范的标识字段实际类型。
	typed := value.(type) {
	case nil, map[string]any, []any:
		return ""
	case string:
		return cleanSystemCardIdentifierText(typed)
	case json.Number:
		return cleanSystemCardIdentifierText(typed.String())
	case float64:
		// maxExactFloat64Integer 是 IEEE-754 双精度可以无损表达的最大整数。
		const maxExactFloat64Integer = float64(1<<53 - 1)
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) || math.Abs(typed) > maxExactFloat64Integer {
			return ""
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		// maxExactFloat32Integer 是单精度可以无损表达的最大整数，超过后不能恢复平台原始标识。
		const maxExactFloat32Integer = float32(1<<24 - 1)
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) || typed != float32(math.Trunc(float64(typed))) || float32(math.Abs(float64(typed))) > maxExactFloat32Integer {
			return ""
		}
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.FormatInt(int64(typed), 10)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return ""
	}
}

// cleanSystemCardIdentifierText 保留普通字符串标识，但拒绝可以被解释为小数或科学计数法的数字文本，避免把已经丢失精度的值当成真实订单号。
func cleanSystemCardIdentifierText(value string) string {
	// text 是去空白后的平台标识原文。
	text := strings.TrimSpace(value)
	if text == "" || text == "<nil>" || text == "null" {
		return ""
	}
	if strings.ContainsAny(text, ".eE") {
		if // _, parseErr 只用于判断带小数点或指数的文本是否确实是数值标识。
		_, parseErr := strconv.ParseFloat(text, 64); parseErr == nil {
			return ""
		}
	}
	return text
}

// firstSystemCardText 返回首个非空候选，保持平台字段优先级。
func firstSystemCardText(values ...string) string {
	// value 是当前待检查的展示或标识候选。
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// limitedSystemCardText 按 Unicode 字符限制持久化文本，避免异常平台载荷无限膨胀数据库行。
func limitedSystemCardText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	// runes 保存待截断文本的 Unicode 字符，避免从多字节字符中间切断。
	runes := []rune(value)
	return string(runes[:limit])
}
