// reply.go 四级回复引擎：API → 关键词 → AI → 默认回复。
// 实现关键词回复、默认回复和 AI 回复的调度。
//
// Phase 3 实现：关键词（含商品ID优先+变量替换+空回复标记）、默认回复（指定商品优先+reply_once+变量替换）。
// API 回复（调外部 /xianyu/reply 接口）和 AI 回复（OpenAI 兼容）留接口注入。

package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	chatdomain "xianyu-go/internal/chat"
	"xianyu-go/internal/db"
)

// keywordEventReplyLease 在平台发送结果不确定时阻止同一订单事件当天重复发送。
const keywordEventReplyLease = 24 * time.Hour

// ReplyResult 回复结果。
type ReplyResult struct {
	Text                 string // 文本回复（可空）
	ImageURL             string // 图片回复（可空）
	Source               string // 回复来源：API/关键词/AI/默认
	Skip                 bool   // true 表示匹配到空回复，不发送任何内容
	ReplyOnce            bool   // 仅默认回复使用，发送状态由 Handle 持久化
	GroupID              string
	ReplyIntervalSeconds int64
	SendDelaySeconds     int64
}

// APIReplier 外部 API 回复（优先级1）。返回 nil 表示无回复。
type APIReplier interface {
	Reply(ctx context.Context, m ChatMessage) (*ReplyResult, error)
}

// AIReplier AI 回复（优先级3）。返回 nil 表示无回复。
type AIReplier interface {
	Reply(ctx context.Context, m ChatMessage) (*ReplyResult, error)
}

// MessageSender 是回复服务发送文本/图片所需的最小接口。
type MessageSender interface {
	SendText(ctx context.Context, chatID, toUserID, text string) error
	SendImage(ctx context.Context, chatID, toUserID, imageURL string, cardID int64, width, height int) error
}

// ReplyService 单账号回复服务。
type ReplyService struct {
	cookieID string
	store    *db.Store
	api      APIReplier // 可为 nil
	ai       AIReplier  // 可为 nil
	sender   MessageSender
	logger   *slog.Logger
}

// NewReplyService 构造。
func NewReplyService(cookieID string, store *db.Store, sender MessageSender,
	api APIReplier, ai AIReplier, logger *slog.Logger) *ReplyService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReplyService{
		cookieID: cookieID,
		store:    store,
		api:      api,
		ai:       ai,
		sender:   sender,
		logger:   logger.With("account", cookieID, "subsys", "reply"),
	}
}

// Handle 收到一条聊天消息，按四级优先级回复。
// 由 Account 在防抖后调用。返回是否产生了回复。
// Handle 处理当前值。
func (r *ReplyService) Handle(ctx context.Context, m ChatMessage) error {
	// res 用于本次流程后续判断的响应
	res := r.resolve(ctx, m)
	if res == nil || res.Skip {
		return nil
	}
	// 发送：图片优先，文本随后。reply_once 使用持久化分段状态，失败时只重试
	// 尚未成功的部分。
	if r.sender == nil {
		return nil
	}
	if res.Source == "关键词" && res.GroupID != "" && m.ChatID != "" {
		// allowed、cooldownErr 是规则组重复回复间隔判断结果和数据库错误。
		allowed, cooldownErr := r.store.Keywords.KeywordReplyAllowed(ctx, r.cookieID, m.ChatID, res.GroupID, res.ReplyIntervalSeconds, time.Now().Unix())
		if cooldownErr != nil {
			return fmt.Errorf("检查关键词重复回复间隔: %w", cooldownErr)
		}
		if !allowed {
			return nil
		}
	}
	if res.Source == "关键词" && res.SendDelaySeconds > 0 {
		// timer 是关键词命中后的可取消发送等待器。
		timer := time.NewTimer(time.Duration(res.SendDelaySeconds) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	// record 用于本次流程后续判断的record
	record := db.DefaultReplyRecord{}
	if res.ReplyOnce && m.ChatID != "" {
		// claimed 用于本次流程后续判断的claimed
		var claimed bool
		// err 用于本次流程后续判断的err
		var err error
		record, claimed, err = r.store.DefaultReps.ClaimRecord(ctx, r.cookieID, m.ChatID, res.Text != "", res.ImageURL != "")
		if err != nil {
			return fmt.Errorf("领取默认回复发送任务: %w", err)
		}
		if !claimed {
			return nil
		}
	}
	if res.ImageURL != "" && !record.ImageSent {
		if // err 用于本次流程后续判断的err
		err := r.sender.SendImage(ctx, m.ChatID, m.SenderUserID, res.ImageURL, 0, 0, 0); err != nil {
			r.logger.Error("发送回复图片失败", "err", err)
			r.markReplyFailure(ctx, res, m, err)
			return err
		}
		if res.ReplyOnce && m.ChatID != "" {
			if // err 用于本次流程后续判断的err
			err := r.store.DefaultReps.MarkPartSent(ctx, r.cookieID, m.ChatID, "image"); err != nil {
				r.markReplyFailure(ctx, res, m, err)
				return err
			}
		}
	}
	if res.Text != "" && !record.TextSent {
		if // err 用于本次流程后续判断的err
		err := r.sender.SendText(ctx, m.ChatID, m.SenderUserID, res.Text); err != nil {
			r.logger.Error("发送回复文本失败", "err", err)
			r.markReplyFailure(ctx, res, m, err)
			return err
		}
		if res.ReplyOnce && m.ChatID != "" {
			if // err 用于本次流程后续判断的err
			err := r.store.DefaultReps.MarkPartSent(ctx, r.cookieID, m.ChatID, "text"); err != nil {
				r.markReplyFailure(ctx, res, m, err)
				return err
			}
		}
	}
	if res.ReplyOnce && m.ChatID != "" {
		if // err 用于本次流程后续判断的err
		err := r.store.DefaultReps.MarkRecordSent(ctx, r.cookieID, m.ChatID); err != nil {
			r.markReplyFailure(ctx, res, m, err)
			return err
		}
	}
	if res.Source == "关键词" && res.GroupID != "" && m.ChatID != "" {
		// markErr 是持久化本次成功回复冷却起点时的数据库错误。
		if markErr := r.store.Keywords.MarkKeywordReplySuccess(ctx, r.cookieID, m.ChatID, res.GroupID, time.Now().Unix()); markErr != nil {
			return fmt.Errorf("记录关键词回复成功时间: %w", markErr)
		}
	}
	return nil
}

// HandleKeywordOnly 只允许关键词规则处理白名单系统消息，禁止 API、AI 和默认回复接管系统卡片。
func (r *ReplyService) HandleKeywordOnly(ctx context.Context, m ChatMessage) error {
	return r.handleKeywordSystem(ctx, m, "", "")
}

// HandleKeywordEvent 使用精确订单和稳定事件执行系统关键词回复，并持久化订单级幂等。
func (r *ReplyService) HandleKeywordEvent(ctx context.Context, m ChatMessage, eventType, orderID string) error {
	if strings.TrimSpace(eventType) == "" || strings.TrimSpace(orderID) == "" {
		return fmt.Errorf("系统关键词订单事件缺少事件类型或订单号")
	}
	return r.handleKeywordSystem(ctx, m, strings.TrimSpace(eventType), strings.TrimSpace(orderID))
}

// handleKeywordSystem 共用系统消息目标校验、规则匹配、冷却、订单幂等和发送流程。
func (r *ReplyService) handleKeywordSystem(ctx context.Context, m ChatMessage, eventType, orderID string) error {
	// targetID 是去除平台协议后缀后的系统消息对端身份。
	targetID := strings.TrimSuffix(strings.TrimSpace(m.SenderUserID), "@goofish")
	if targetID == "" || targetID == "1400" || m.IsSelf {
		return nil
	}
	// result 是系统消息关键词匹配结果。
	result := r.keywordReply(ctx, m)
	if result == nil || result.Skip || r.sender == nil {
		return nil
	}
	// allowed、cooldownErr 是系统消息规则组重复回复间隔判断结果和错误。
	allowed, cooldownErr := r.store.Keywords.KeywordReplyAllowed(ctx, r.cookieID, m.ChatID, result.GroupID, result.ReplyIntervalSeconds, time.Now().Unix())
	if cooldownErr != nil || !allowed {
		return cooldownErr
	}
	// claimToken 是订单事件模式下当前发送尝试持有的幂等租约；普通系统事件保持空值。
	claimToken := ""
	// releaseClaim 表示尚未完成平台发送时需要释放订单事件租约以允许后续实时事件重试。
	releaseClaim := false
	if eventType != "" && orderID != "" {
		claimToken = newKeywordEventClaimToken(m.MessageID)
		// now 是订单事件租约和成功时间使用的 Unix 秒。
		now := time.Now().Unix()
		// claimed 表示当前实时卡片是否取得同订单、规则和事件的唯一发送权。
		claimed, claimErr := r.store.Keywords.ClaimKeywordEventReply(ctx, r.cookieID, orderID, result.GroupID, eventType, claimToken, now, now+int64(keywordEventReplyLease/time.Second))
		if claimErr != nil {
			return fmt.Errorf("抢占系统关键词订单事件发送权: %w", claimErr)
		}
		if !claimed {
			return nil
		}
		releaseClaim = true
		defer func() {
			if releaseClaim {
				// releaseCtx 和 releaseCancel 为本地租约释放提供独立短超时，避免请求取消后永久阻塞重试。
				releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer releaseCancel()
				_ = r.store.Keywords.ReleaseKeywordEventReply(releaseCtx, r.cookieID, orderID, result.GroupID, eventType, claimToken)
			}
		}()
	}
	if result.SendDelaySeconds > 0 {
		// timer 是系统规则回复的可取消发送等待器。
		timer := time.NewTimer(time.Duration(result.SendDelaySeconds) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	// sendErr 是图片或文字系统规则回复的发送结果。
	var sendErr error
	if result.ImageURL != "" {
		sendErr = r.sender.SendImage(ctx, m.ChatID, m.SenderUserID, result.ImageURL, 0, 0, 0)
	}
	if result.Text != "" && sendErr == nil {
		sendErr = r.sender.SendText(ctx, m.ChatID, m.SenderUserID, result.Text)
	}
	if sendErr != nil {
		return sendErr
	}
	if claimToken != "" {
		// successAt 是平台发送成功后写入订单事件永久幂等记录的 Unix 秒。
		successAt := time.Now().Unix()
		// successErr 是平台发送成功后完成订单事件幂等记录的数据库结果。
		if successErr := r.store.Keywords.MarkKeywordEventReplySuccess(ctx, r.cookieID, orderID, result.GroupID, eventType, claimToken, successAt); successErr != nil {
			// 平台已发送但本地完成失败时保留二十四小时租约，禁止立即释放后重复发送。
			releaseClaim = false
			return fmt.Errorf("完成系统关键词订单事件幂等记录: %w", successErr)
		}
		releaseClaim = false
	}
	return r.store.Keywords.MarkKeywordReplySuccess(ctx, r.cookieID, m.ChatID, result.GroupID, time.Now().Unix())
}

// newKeywordEventClaimToken 生成不含凭据的随机租约标识；熵源失败时使用消息 ID 和纳秒时间兜底。
func newKeywordEventClaimToken(messageID string) string {
	// raw 是订单事件租约使用的十六字节随机数。
	raw := make([]byte, 16)
	// randomErr 是读取本机安全随机源生成租约标识的结果。
	if _, randomErr := rand.Read(raw); randomErr == nil {
		return hex.EncodeToString(raw)
	}
	return fmt.Sprintf("%s-%d", strings.TrimSpace(messageID), time.Now().UnixNano())
}

// markReplyFailure 封装mark回复Failure业务协调。
func (r *ReplyService) markReplyFailure(ctx context.Context, res *ReplyResult, m ChatMessage, sendErr error) {
	if res.ReplyOnce && m.ChatID != "" {
		_ = r.store.DefaultReps.MarkRecordFailed(ctx, r.cookieID, m.ChatID, sendErr.Error())
	}
}

// resolve 按优先级确定回复内容（不发送）。
func (r *ReplyService) resolve(ctx context.Context, m ChatMessage) *ReplyResult {
	// 优先级1：API 回复。
	if r.api != nil {
		if // res、err 用于本次流程后续判断的res、err
		res, err := r.api.Reply(ctx, m); err != nil {
			r.logger.Error("API 回复失败", "err", err)
		} else if res != nil {
			res.Source = "API"
			return res
		}
	}

	// 优先级2：关键词匹配。
	if res := r.keywordReply(ctx, m); res != nil {
		return res
	}

	// 优先级3：AI 回复。
	if r.ai != nil {
		if // res、err 用于本次流程后续判断的res、err
		res, err := r.ai.Reply(ctx, m); err != nil {
			r.logger.Error("AI 回复失败", "err", err)
		} else if res != nil {
			res.Source = "AI"
			return res
		}
	}

	// 优先级4：默认回复。
	return r.defaultReply(ctx, m)
}

// keywordReply 关键词匹配。返回 nil 表示无匹配；
// 返回 Skip=true 表示匹配到空回复（不发送）。
// 移植自 get_keyword_reply：商品ID关键词优先 → 通用关键词。
// keywordReply 封装关键词回复业务协调。
func (r *ReplyService) keywordReply(ctx context.Context, m ChatMessage) *ReplyResult {
	// kws、err 用于本次流程后续判断的kws、err
	kws, err := r.store.Keywords.AllWithType(ctx, r.cookieID)
	if err != nil || len(kws) == 0 {
		return nil
	}
	// groups 是按数据库优先顺序聚合的关键词规则组。
	groups := groupKeywords(kws)
	if m.ItemID != "" {
		// group 是当前商品范围内待匹配的关键词组。
		for _, group := range groups {
			if group[0].ItemID == m.ItemID && keywordGroupMatches(group, m) {
				return r.keywordResult(group[0], m)
			}
		}
	}
	// group 是当前待匹配的账号级关键词组。
	for _, group := range groups {
		if group[0].ItemID == "" && keywordGroupMatches(group, m) {
			return r.keywordResult(group[0], m)
		}
	}
	return nil
}

// groupKeywords 按 group_id 聚合关键词行并保持最长关键词优先的首次出现顺序。
func groupKeywords(rows []db.Keyword) [][]db.Keyword {
	// groups 保存按首次出现顺序输出的关键词组。
	groups := make([][]db.Keyword, 0)
	// indexes 保存组标识到结果下标的映射。
	indexes := make(map[string]int)
	// row 是当前归组的关键词行。
	for _, row := range rows {
		// groupID 是稳定组标识；旧异常空值使用关键词自身隔离。
		groupID := row.GroupID
		if groupID == "" {
			groupID = "legacy:" + row.ItemID + ":" + row.Keyword
		}
		// index、exists 是当前组下标和存在标记。
		index, exists := indexes[groupID]
		if !exists {
			index = len(groups)
			indexes[groupID] = index
			groups = append(groups, nil)
		}
		groups[index] = append(groups[index], row)
	}
	return groups
}

// keywordGroupMatches 按消息来源和 contains、excludes、equals 逻辑判断规则组。
func keywordGroupMatches(group []db.Keyword, message ChatMessage) bool {
	if len(group) == 0 {
		return false
	}
	// rule 是组内共享配置的代表行。
	rule := group[0]
	if message.IsSystem {
		if !messageScopeAllowed(rule.MessageScope, "system") || !systemEventAllowed(rule.SystemTypes, message) {
			return false
		}
	} else if !messageScopeAllowed(rule.MessageScope, "customer") {
		return false
	}
	// text 是去首尾空白并转小写后的消息文本。
	text := strings.ToLower(strings.TrimSpace(message.Text))
	// anyMatch 表示任一关键词满足包含或全文相等条件。
	anyMatch := false
	// keywordCount 是组内实际非空关键词数量。
	keywordCount := 0
	// keyword 是当前参与组逻辑判断的关键词行。
	for _, keyword := range group {
		// normalized 是去首尾空白并转小写后的关键词。
		normalized := strings.ToLower(strings.TrimSpace(keyword.Keyword))
		if normalized == "" {
			continue
		}
		keywordCount++
		if rule.MatchType == "equals" && text == normalized || rule.MatchType != "equals" && strings.Contains(text, normalized) {
			anyMatch = true
			break
		}
	}
	if keywordCount == 0 {
		return message.IsSystem
	}
	if rule.MatchType == "excludes" {
		return !anyMatch
	}
	return anyMatch
}

// messageScopeAllowed 判断逗号分隔的规则来源是否包含目标客户或系统消息来源。
func messageScopeAllowed(rawScopes, target string) bool {
	// scope 是当前规则配置的一个消息来源。
	for _, scope := range strings.Split(rawScopes, ",") {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

// systemEventAllowed 判断结构化系统消息稳定事件是否在规则白名单内。
func systemEventAllowed(rawEvents string, message ChatMessage) bool {
	// allowed 保存规则配置的稳定系统事件集合。
	allowed := make(map[string]struct{})
	// value 是当前配置的稳定系统事件。
	for _, value := range strings.Split(rawEvents, ",") {
		allowed[strings.TrimSpace(value)] = struct{}{}
	}
	// event 是聊天领域从平台载荷和展示摘要解析出的稳定事件。
	event := chatdomain.ClassifySystemEvent(message.Raw, message.Text)
	// _, exists 只读取当前稳定事件是否位于白名单。
	_, exists := allowed[event]
	return exists
}

// keywordResult 封装关键词结果业务协调。
func (r *ReplyService) keywordResult(kw db.Keyword, m ChatMessage) *ReplyResult {
	if kw.Type == "image" && kw.ImageURL != "" {
		return &ReplyResult{ImageURL: kw.ImageURL, Source: "关键词", GroupID: kw.GroupID, ReplyIntervalSeconds: kw.ReplyIntervalSeconds, SendDelaySeconds: kw.SendDelaySeconds}
	}
	if strings.TrimSpace(kw.Reply) == "" {
		return &ReplyResult{Skip: true, Source: "关键词", GroupID: kw.GroupID, ReplyIntervalSeconds: kw.ReplyIntervalSeconds, SendDelaySeconds: kw.SendDelaySeconds} // EMPTY_REPLY
	}
	return &ReplyResult{Text: formatReply(kw.Reply, m), Source: "关键词", GroupID: kw.GroupID, ReplyIntervalSeconds: kw.ReplyIntervalSeconds, SendDelaySeconds: kw.SendDelaySeconds}
}

// defaultReply 默认回复。移植自 get_default_reply：
// 指定商品回复优先 → 账号默认回复（reply_once 防重复 + 变量替换）。
// defaultReply 封装default回复业务协调。
func (r *ReplyService) defaultReply(ctx context.Context, m ChatMessage) *ReplyResult {
	// 1. 指定商品回复。
	if m.ItemID != "" {
		if // ir、err 用于本次流程后续判断的ir、err
		ir, err := r.store.ItemReps.Get(ctx, r.cookieID, m.ItemID); err == nil && ir != nil && strings.TrimSpace(ir.ReplyContent) != "" {
			return &ReplyResult{Text: formatReplyWithItem(ir.ReplyContent, m), Source: "默认"}
		}
	}
	// 2. 账号默认回复。
	dr, err := r.store.DefaultReps.Get(ctx, r.cookieID)
	if err != nil || dr == nil || !dr.Enabled {
		return nil
	}
	// 文字和图片都为空 → 空回复标记。
	if strings.TrimSpace(dr.ReplyContent) == "" && strings.TrimSpace(dr.ReplyImageURL) == "" {
		return &ReplyResult{Skip: true, Source: "默认"}
	}
	// res 用于本次流程后续判断的响应
	res := &ReplyResult{Source: "默认", ReplyOnce: dr.ReplyOnce}
	if strings.TrimSpace(dr.ReplyContent) != "" {
		res.Text = formatReply(dr.ReplyContent, m)
	}
	if strings.TrimSpace(dr.ReplyImageURL) != "" {
		res.ImageURL = dr.ReplyImageURL
	}
	return res
}

// formatReply 变量替换：{send_user_name} {send_user_id} {send_message}。
// 替换回复模板变量；若替换出错则返回原文。
// formatReply 封装format回复业务协调。
func formatReply(template string, m ChatMessage) string {
	return safeFormat(template, map[string]string{
		"send_user_name": m.SenderName,
		"send_user_id":   m.SenderUserID,
		"send_message":   m.Text,
	})
}

// formatReplyWithItem 含 {item_id} 变量。
func formatReplyWithItem(template string, m ChatMessage) string {
	return safeFormat(template, map[string]string{
		"send_user_name": m.SenderName,
		"send_user_id":   m.SenderUserID,
		"send_message":   m.Text,
		"item_id":        m.ItemID,
	})
}

// safeFormat 实现命名占位符替换。
func safeFormat(template string, vars map[string]string) string {
	// out 用于本次流程后续判断的out
	out := template
	// k、v 表示当前遍历过程中的k、v
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}
