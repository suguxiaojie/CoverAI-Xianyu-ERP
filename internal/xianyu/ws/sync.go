package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"

	"xianyu-go/internal/xianyu/protocol"
)

// locationCardPreviewURL 是闲鱼位置消息使用的官方通用预览图，不携带用户或店铺信息。
const locationCardPreviewURL = "https://img.alicdn.com/imgextra/i4/O1CN01t5Yxwi1vA3LNVBAWU_!!6000000006131-2-tps-300-300.png"

// SendResult 是闲鱼接受聊天消息后返回的非敏感平台标识。
type SendResult struct {
	// PlatformMessageID 是撤回、已读和历史对账使用的 PNM ID。
	PlatformMessageID string
	// CreatedAt 是平台记录的消息创建时间，Unix 毫秒；缺失时为零。
	CreatedAt int64
}

// RecallFeature 描述当前官方网页发送者撤回需要的展示元数据。
type RecallFeature struct {
	// OperatorID 是当前发送账号的平台用户标识。
	OperatorID string
	// OriginContentType 是被撤回内容的闲鱼类型，例如文本 1、图片 2。
	OriginContentType int
	// TextContent 仅保存发送方文本，供撤回成功后的重新编辑入口使用。
	TextContent string
}

// handleSyncExtra 处理服务端增量同步帧并在需要时回传确认，返回解码或发送错误。
func (c *Conn) handleSyncExtra(ctx context.Context, msg map[string]any) error {
	// body 用于本次流程后续判断的请求体
	body, _ := msg["body"].(map[string]any)
	// extra 用于本次流程后续判断的extra
	extra, _ := body["syncExtraType"].(map[string]any)
	// typeCode、ok 用于本次流程后续判断的类型Code、ok
	typeCode, ok := responseCode(extra["type"])
	if !ok || (typeCode != 1 && typeCode != 2) {
		return nil
	}
	// state、err 用于本次流程后续判断的state、err
	state, err := c.request(ctx, "/r/SyncStatus/getState", map[string]any{}, []any{map[string]any{"topic": "sync"}}, regResponseTimeout)
	if err != nil {
		return fmt.Errorf("getState: %w", err)
	}
	if // code、ok 用于本次流程后续判断的code、ok
	code, ok := responseCode(state["code"]); !ok || code != http.StatusOK || state["body"] == nil {
		return fmt.Errorf("getState 返回异常: code=%v", state["code"])
	}
	// response、err 用于本次流程后续判断的response、err
	response, err := c.request(ctx, "/r/SyncStatus/ackDiff", map[string]any{}, []any{state["body"]}, regResponseTimeout)
	if err != nil {
		return fmt.Errorf("ackDiff: %w", err)
	}
	if // code、ok 用于本次流程后续判断的code、ok
	code, ok := responseCode(response["code"]); ok && code != http.StatusOK {
		return fmt.Errorf("ackDiff 返回异常: code=%d", code)
	}
	return nil
}

// sendACK 回复 {"code":200, headers:<服务端完整 headers>}。
func (c *Conn) sendACK(ctx context.Context, msg map[string]any) {
	// headers 用于本次流程后续判断的headers
	headers, _ := msg["headers"].(map[string]any)
	// ackHeaders 用于本次流程后续判断的ackHeaders
	ackHeaders := make(map[string]any, len(headers))
	// key、value 表示当前遍历过程中的key、value
	for key, value := range headers {
		ackHeaders[key] = value
	}
	// ack 用于本次流程后续判断的ack
	ack := map[string]any{
		"code":    200,
		"headers": ackHeaders,
	}
	// ACK 失败不阻塞主循环。
	ackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	_ = c.sendJSON(ackCtx, ack)
	cancel()
}

// extractSyncPayload 取出 body.syncPushPackage.data[0].data（字符串）。
func extractSyncPayload(msg map[string]any) (string, bool) {
	// body 用于本次流程后续判断的请求体
	body, _ := msg["body"].(map[string]any)
	if body == nil {
		return "", false
	}
	// pkg 用于本次流程后续判断的pkg
	pkg, _ := body["syncPushPackage"].(map[string]any)
	if pkg == nil {
		return "", false
	}
	// arr 用于本次流程后续判断的arr
	arr, _ := pkg["data"].([]any)
	if len(arr) == 0 {
		return "", false
	}
	// first 用于本次流程后续判断的first
	first, _ := arr[0].(map[string]any)
	if first == nil {
		return "", false
	}
	// d、ok 用于本次流程后续判断的d、ok
	d, ok := first["data"].(string)
	return d, ok && d != ""
}

// decodeSyncData 先尝试 base64+JSON（未加密系统消息），失败则 base64+msgpack 解密。
func decodeSyncData(data string) (map[string]any, error) {
	// 1) base64 解码后尝试解析 JSON。
	if dec, err := base64.StdEncoding.DecodeString(data); err == nil {
		// parsed 用于本次流程后续判断的解析结果
		var parsed map[string]any
		if // jsonErr 用于本次流程后续判断的jsonErr
		jsonErr := json.Unmarshal(dec, &parsed); jsonErr == nil {
			return parsed, nil
		}
	}
	// 2) JSON 解析失败 → msgpack 解密
	out, err := protocol.Decrypt(data)
	if err != nil {
		return nil, err
	}
	// parsed 用于本次流程后续判断的解析结果
	var parsed map[string]any
	if // err 用于本次流程后续判断的err
	err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, fmt.Errorf("解密后非 JSON: %w", err)
	}
	return parsed, nil
}

// sendJSON 发送一条 JSON 文本帧。
func (c *Conn) sendJSON(ctx context.Context, v any) error {
	// b、err 用于本次流程后续判断的b、err
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if // recorder 用于本次流程后续判断的recorder
	recorder := c.recorderSnapshot(); recorder != nil {
		recorder("out", string(b), string(b), "json", "")
	}
	select {
	case c.sendGate <- struct{}{}:
		defer func() { <-c.sendGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return c.ws.Write(ctx, websocket.MessageText, b)
}

// SendText 发送一条闲鱼聊天文本消息。
func (c *Conn) SendText(ctx context.Context, myID, cid, toID, text string) error {
	// content 用于本次流程后续判断的内容
	content := map[string]any{
		"contentType": 1,
		"text": map[string]any{
			"text": text,
		},
	}
	return c.sendChatContent(ctx, myID, cid, toID, content)
}

// SendTextWithResult 发送文本并等待平台响应，返回撤回所需的 PNM ID。
func (c *Conn) SendTextWithResult(ctx context.Context, myID, cid, toID, text string) (SendResult, error) {
	// content 是官方自定义消息内层的文本载荷。
	content := map[string]any{"contentType": 1, "text": map[string]any{"text": text}}
	return c.sendChatContentWithResult(ctx, myID, cid, toID, content)
}

// SendReplyTextWithResult 发送带双字段 replyMessageId 扩展的原生引用文本，并等待平台 PNM 结果。
func (c *Conn) SendReplyTextWithResult(ctx context.Context, myID, cid, toID, text, replyToPlatformMessageID string) (SendResult, error) {
	// content 是与普通文本一致的 contentType=1 内层载荷，引用关系只存放在外层扩展。
	content := map[string]any{"contentType": 1, "text": map[string]any{"text": text}}
	return c.sendChatContentWithReplyResult(ctx, myID, cid, toID, content, replyToPlatformMessageID)
}

// MarkChatRead 将当前会话的 PNM 消息 ID 上报为已读。
// ctx 控制远端请求生命周期；cid 仅用于本地可观测日志；messageIDs 为待上报消息对象。
// 返回值仅报告远端调用失败，平台拒绝会被记录为告警以保留既有调用兼容性。
func (c *Conn) MarkChatRead(ctx context.Context, cid string, messageIDs []map[string]any) error {
	// ids 是剔除空值后的 PNM ID 列表，按平台 MessageStatusService 的参数格式发送。
	ids := make([]string, 0, len(messageIDs))
	// item 为调用方传入的一条待读消息对象，可能缺少平台消息 ID。
	for _, item := range messageIDs {
		// id 是当前对象中可上报的非空 PNM 消息 ID。
		if id := strings.TrimSpace(fmt.Sprint(item["messageId"])); id != "" && id != "<nil>" {
			ids = append(ids, id)
		}
	}
	c.logger.Debug("准备上报闲鱼已读", "cid", cid, "message_count", len(ids), "message_ids", ids)
	// response 保存平台响应；err 表示请求或传输失败。服务只接受一个 string 列表参数。
	response, err := c.request(ctx, "/r/MessageStatus/read", map[string]any{}, []any{ids}, regResponseTimeout)
	if err == nil {
		// code 是平台业务状态码；ok 表示响应中的状态码可被规范解析。
		if code, ok := responseCode(response["code"]); ok && code >= 400 {
			c.logger.Warn("闲鱼已读上报被拒绝", "cid", cid, "message_count", len(ids), "code", code, "body", response["body"])
		} else {
			c.logger.Debug("闲鱼已读上报成功", "cid", cid, "message_count", len(ids), "message_ids", ids, "code", response["code"])
		}
	}
	return err
}

// SendImage 发送一条闲鱼聊天图片消息。imageURL 应为闲鱼可访问的 CDN/公网 URL。
func (c *Conn) SendImage(ctx context.Context, myID, cid, toID, imageURL string, width, height int) error {
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}
	// content 用于本次流程后续判断的内容
	content := map[string]any{
		"contentType": 2,
		"image": map[string]any{
			"pics": []map[string]any{{
				"height": height,
				"type":   0,
				"url":    imageURL,
				"width":  width,
			}},
		},
	}
	return c.sendChatContent(ctx, myID, cid, toID, content)
}

// SendImageWithResult 发送图片并等待平台响应，返回撤回所需的 PNM ID。
func (c *Conn) SendImageWithResult(ctx context.Context, myID, cid, toID, imageURL string, width, height int) (SendResult, error) {
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}
	// content 是官方自定义消息内层的图片载荷。
	content := map[string]any{"contentType": 2, "image": map[string]any{"pics": []map[string]any{{
		"height": height, "type": 0, "url": imageURL, "width": width,
	}}}}
	return c.sendChatContentWithResult(ctx, myID, cid, toID, content)
}

// SendLocationCardWithResult 发送标题、说明和真实坐标组成的位置卡片，并等待平台返回 PNM ID。
// ctx 控制一次平台写入；myID、cid、toID 是当前在线单聊身份；经纬度使用 WGS84 十进制度。
func (c *Conn) SendLocationCardWithResult(ctx context.Context, myID, cid, toID, title, description string, latitude, longitude float64) (SendResult, error) {
	// content、contentErr 是通过位置字段校验后构造的 contentType=30 内层协议载荷。
	content, contentErr := buildLocationCardContent(title, description, latitude, longitude)
	if contentErr != nil {
		return SendResult{}, contentErr
	}
	return c.sendChatContentWithResult(ctx, myID, cid, toID, content)
}

// buildLocationCardContent 构造闲鱼官方地图页与 contentType=30 载荷；文本只进入查询值，不控制目标域名或路径。
func buildLocationCardContent(title, description string, latitude, longitude float64) (map[string]any, error) {
	// normalizedTitle、normalizedDescription 是去除首尾空白后的卡片展示文本。
	normalizedTitle, normalizedDescription := strings.TrimSpace(title), strings.TrimSpace(description)
	if normalizedTitle == "" || normalizedDescription == "" {
		return nil, errors.New("位置卡片标题和说明不能为空")
	}
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return nil, errors.New("位置卡片经纬度超出有效范围")
	}
	// latitudeText、longitudeText 以六位小数匹配闲鱼现有位置卡片精度。
	latitudeText, longitudeText := strconv.FormatFloat(latitude, 'f', 6, 64), strconv.FormatFloat(longitude, 'f', 6, 64)
	// pageURL 固定为闲鱼官方位置卡片页面，调用方不能替换域名或路径。
	pageURL := url.URL{Scheme: "https", Host: "market.m.taobao.com", Path: "/app/idleFish-F2e/fish-base/pages/map-for-im/index.html"}
	// query 保存官方页面展示角色、坐标及与聊天卡片一致的标题和说明。
	query := pageURL.Query()
	query.Set("role", "consumer")
	query.Set("latitude", latitudeText)
	query.Set("longitude", longitudeText)
	query.Set("mainTitle", normalizedTitle)
	query.Set("subTitle", normalizedDescription)
	pageURL.RawQuery = query.Encode()
	return map[string]any{
		"contentType": 30,
		"locationCard": map[string]any{
			"action": map[string]any{
				"actionType": 4,
				"page": map[string]any{
					"actionStyle": 0, "actionType": 0, "iosActionStyle": 0,
					"showGuideAlways": false, "url": pageURL.String(),
				},
			},
			"content": normalizedDescription, "height": 0, "latitude": latitudeText,
			"longitude": longitudeText, "showSender": false, "title": normalizedTitle,
			"url": locationCardPreviewURL, "width": 0,
		},
	}, nil
}

// sendChatContent 封装send聊天内容业务协调。
func (c *Conn) sendChatContent(ctx context.Context, myID, cid, toID string, content any) error {
	// body 是 sendByReceiverScope 的消息模型与接收者范围。
	body, err := buildChatSendBody(myID, cid, toID, content)
	if err != nil {
		return err
	}
	// msg 保持既有无需等待响应的自动化发送帧格式。
	msg := map[string]any{"lwp": "/r/MessageSend/sendByReceiverScope", "headers": map[string]any{"mid": protocol.GenerateMid()}, "body": body}
	return c.sendJSON(ctx, msg)
}

// sendChatContentWithResult 使用请求关联等待平台发送结果，结果不包含 Cookie 或正文。
func (c *Conn) sendChatContentWithResult(ctx context.Context, myID, cid, toID string, content any) (SendResult, error) {
	// body 是与既有发送完全一致的平台请求体。
	body, err := buildChatSendBody(myID, cid, toID, content)
	return c.sendChatBodyWithResult(ctx, body, err)
}

// sendChatContentWithReplyResult 构造携带原生引用扩展的请求体，并复用同一平台响应校验。
func (c *Conn) sendChatContentWithReplyResult(ctx context.Context, myID, cid, toID string, content any, replyToPlatformMessageID string) (SendResult, error) {
	// body、err 是带引用目标的 sendByReceiverScope 请求体和本地校验错误。
	body, err := buildChatSendBodyWithReply(myID, cid, toID, content, replyToPlatformMessageID)
	return c.sendChatBodyWithResult(ctx, body, err)
}

// sendChatBodyWithResult 发送已构造的聊天请求体，并仅返回非敏感 PNM ID 和创建时间。
func (c *Conn) sendChatBodyWithResult(ctx context.Context, body []any, buildErr error) (SendResult, error) {
	if buildErr != nil {
		return SendResult{}, buildErr
	}
	// response 是平台带业务状态和 SendResultModel 的响应帧。
	response, err := c.request(ctx, "/r/MessageSend/sendByReceiverScope", map[string]any{}, body, regResponseTimeout)
	if err != nil {
		return SendResult{}, err
	}
	// code、ok 保存平台发送业务状态及可解析性。
	if code, ok := responseCode(response["code"]); !ok || code != http.StatusOK {
		return SendResult{}, fmt.Errorf("闲鱼发送被拒绝: code=%v", response["code"])
	}
	// result 是从平台响应正文提取的 PNM ID 和时间。
	result := parseSendResult(response)
	if result.PlatformMessageID == "" {
		return SendResult{}, errors.New("闲鱼发送成功但未返回平台消息 ID")
	}
	return result, nil
}

// buildChatSendBody 构造文本和图片共用的官方 sendByReceiverScope 请求体。
func buildChatSendBody(myID, cid, toID string, content any) ([]any, error) {
	return buildChatSendBodyWithReply(myID, cid, toID, content, "")
}

// buildChatSendBodyWithReply 构造文本和图片共用请求体；引用文本在扩展字段和 extJson 内双写同一 PNM ID。
func buildChatSendBodyWithReply(myID, cid, toID string, content any, replyToPlatformMessageID string) ([]any, error) {
	myID = stripGoofish(myID)
	cid = stripGoofish(cid)
	toID = stripGoofish(toID)
	if myID == "" || cid == "" || toID == "" {
		return nil, fmt.Errorf("发送消息缺少必要参数: myID=%q cid=%q toID=%q", myID, cid, toID)
	}
	replyToPlatformMessageID = strings.TrimSpace(replyToPlatformMessageID)
	if replyToPlatformMessageID != "" && !strings.HasSuffix(replyToPlatformMessageID, ".PNM") {
		return nil, errors.New("引用回复缺少有效 PNM 目标")
	}
	// raw、err 用于本次流程后续判断的raw、err
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	// encoded 用于本次流程后续判断的encoded
	encoded := base64.StdEncoding.EncodeToString(raw)
	// extension 是官方消息扩展对象；普通消息保持原空 extJson。
	extension := map[string]any{"extJson": "{}"}
	if replyToPlatformMessageID != "" {
		// extJSON 是与外层字段一致的紧凑 JSON，匹配真实入站帧的双写结构。
		extJSON, marshalErr := json.Marshal(map[string]string{"replyMessageId": replyToPlatformMessageID})
		if marshalErr != nil {
			return nil, marshalErr
		}
		extension["replyMessageId"] = replyToPlatformMessageID
		extension["extJson"] = string(extJSON)
	}
	return []any{
		map[string]any{
			"uuid":             protocol.GenerateUUID(),
			"cid":              cid + "@goofish",
			"conversationType": 1,
			"content": map[string]any{
				"contentType": 101,
				"custom": map[string]any{
					"type": 1,
					"data": encoded,
				},
			},
			"redPointPolicy": 0,
			"extension":      extension,
			"ctx": map[string]any{
				"appVersion": "1.0",
				"platform":   "web",
			},
			"mtags":                map[string]any{},
			"msgReadStatusSetting": 1,
		},
		map[string]any{
			"actualReceivers": []string{
				toID + "@goofish",
				myID + "@goofish",
			},
		},
	}, nil
}

// parseSendResult 递归读取平台响应中的 messageId 和 createAt，兼容 body 包装层差异。
func parseSendResult(response map[string]any) SendResult {
	// result 保存首次命中的平台消息 ID 和创建时间。
	var result SendResult
	// walk 递归访问平台响应对象和数组，不复制正文到日志或返回值。
	var walk func(any)
	walk = func(value any) {
		if result.PlatformMessageID != "" && result.CreatedAt > 0 {
			return
		}
		// typed 是当前递归节点的具体容器类型。
		switch typed := value.(type) {
		case map[string]any:
			// key、child 是当前响应对象的字段名和值。
			for key, child := range typed {
				if strings.EqualFold(key, "messageId") && result.PlatformMessageID == "" {
					result.PlatformMessageID = strings.TrimSpace(fmt.Sprint(child))
				}
				if strings.EqualFold(key, "createAt") && result.CreatedAt == 0 {
					_, _ = fmt.Sscan(strings.TrimSpace(fmt.Sprint(child)), &result.CreatedAt)
				}
			}
			// child 是当前响应对象待递归检查的字段值。
			for _, child := range typed {
				walk(child)
			}
		case []any:
			// child 是当前响应数组待递归检查的元素。
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(response["body"])
	return result
}

// RecallMessageByFeature 按当前官方网页协议撤回发送者自己的消息。
func (c *Conn) RecallMessageByFeature(ctx context.Context, platformMessageID string, feature RecallFeature) error {
	platformMessageID = strings.TrimSpace(platformMessageID)
	feature.OperatorID = stripGoofish(feature.OperatorID)
	if platformMessageID == "" || feature.OperatorID == "" || feature.OriginContentType <= 0 {
		return errors.New("撤回消息缺少平台消息 ID、操作者或内容类型")
	}
	// recallFeature 保持官方网页的发送者撤回展示模型。
	recallFeature := map[string]any{
		"showRecallStatusSetting": 1,
		"operatorType":            0,
		"operatorUid":             feature.OperatorID,
		"code":                    "200",
		"extension": map[string]any{
			"originContentType": strconv.Itoa(feature.OriginContentType),
			"textContent":       feature.TextContent,
		},
	}
	// response 是平台撤回业务响应；请求正文不携带账号凭证。
	response, err := c.request(ctx, "/r/MessageManager/recallMessageByFeature", map[string]any{}, []any{map[string]any{}, platformMessageID, recallFeature}, regResponseTimeout)
	if err != nil {
		return err
	}
	// code、ok 保存平台撤回业务状态及可解析性。
	if code, ok := responseCode(response["code"]); !ok || code != http.StatusOK {
		return fmt.Errorf("闲鱼撤回被拒绝: code=%v", response["code"])
	}
	return nil
}

// stripGoofish 封装stripGoofish业务协调。
func stripGoofish(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimSuffix(s, "@goofish")
}

// Close 关闭连接。
func (c *Conn) Close() error {
	c.ensureReadPump()
	c.readCancel()
	return c.ws.Close(websocket.StatusNormalClosure, "bye")
}
