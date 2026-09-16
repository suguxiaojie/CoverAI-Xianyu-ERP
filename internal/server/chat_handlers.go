package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	chatapp "xianyu-go/internal/application/chat"
	"xianyu-go/internal/auth"
)

// markChatReadRequest 是提交聊天已读状态的 HTTP 请求 DTO。
type markChatReadRequest struct {
	// AccountID 是当前用户有权操作的账号标识。
	AccountID string `json:"account_id"`
	// ChatID 是会话标识。
	ChatID string `json:"chat_id"`
	// MessageIDs 是平台已读接口需要的消息标识集合。
	MessageIDs []map[string]any `json:"message_ids"`
}

// mountChat 封装mount聊天业务协调。
func (s *Server) mountChat(r chi.Router) {
	r.Get("/api/chat/sessions", s.listChatSessions)
	r.Get("/api/chat/messages", s.listChatMessages)
	r.Post("/api/chat/messages", s.sendChatMessage)
	r.Post("/api/chat/images", s.sendChatImage)
	r.Post("/api/chat/read", s.markChatRead)
	r.Get("/api/chat/ws", s.chatWebSocket)
}

// chatApplication 返回当前 Server 绑定的聊天历史应用服务。
func (s *Server) chatApplication() ChatPort {
	return s.applicationServiceSet().chat
}

// listChatSessions 封装list聊天Sessions业务协调。
func (s *Server) listChatSessions(w http.ResponseWriter, r *http.Request) {
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// accountID 用于本次流程后续判断的账号ID
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	if !s.ownsAccount(r, accountID) {
		writeErr(w, http.StatusForbidden, "无权访问该账号")
		return
	}
	// search 是只查询本地会话摘要和历史消息的可选关键词；非空时禁止触发平台刷新。
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len([]rune(search)) > 200 {
		writeErr(w, http.StatusBadRequest, "会话搜索关键词不能超过 200 字")
		return
	}
	if search != "" {
		// rows、searchErr 是当前账号历史消息搜索命中的会话摘要和错误。
		rows, searchErr := s.chatApplication().SearchSessions(r.Context(), sess.UserID, accountID, search, parsePositiveInt(r.URL.Query().Get("limit"), 100))
		if searchErr != nil {
			switch {
			case errors.Is(searchErr, chatapp.ErrInvalidInput):
				writeErr(w, http.StatusBadRequest, "会话搜索参数无效")
			case errors.Is(searchErr, chatapp.ErrSessionUnavailable):
				writeErr(w, http.StatusServiceUnavailable, "聊天搜索服务未启用")
			default:
				writeErr(w, http.StatusInternalServerError, "搜索聊天会话失败")
			}
			return
		}
		writeJSON(w, http.StatusOK, chatSessionPageResponse{Sessions: newChatSessionDTOsFromApplication(rows)})
		return
	}
	// cursor 用于本次流程后续判断的游标
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	// refresh 用于本次流程后续判断的refresh
	refresh := r.URL.Query().Get("refresh") == "1"
	// hasMore 用于本次流程后续判断的hasMore
	var hasMore bool
	// nextCursor 用于本次流程后续判断的next游标
	var nextCursor int64
	if // err 保存清理空会话的错误。
	err := s.chatApplication().CleanupEmptySessions(r.Context(), accountID); err != nil {
		writeErr(w, http.StatusInternalServerError, "清理无效聊天会话失败")
		return
	}
	if refresh {
		// fetchCtx 和 cancel 限制平台联系人刷新请求的最长时间。
		fetchCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		// page 和 fetchErr 保存应用层联系人分页结果及平台/持久化错误。
		page, fetchErr := s.chatApplication().RefreshConversations(fetchCtx, accountID, cursor, 100)
		cancel()
		if fetchErr == nil {
			hasMore, nextCursor = page.HasMore, page.NextCursor
		} else if errors.Is(fetchErr, chatapp.ErrRefreshPersist) {
			writeErr(w, http.StatusInternalServerError, "保存历史联系人失败")
			return
		} else if !errors.Is(fetchErr, chatapp.ErrRefreshUnavailable) && !errors.Is(fetchErr, chatapp.ErrOffline) {
			s.recoverExpiredSession(r.Context(), accountID, fetchErr)
		}
	}
	// rows、err 保存应用层会话摘要及查询错误。
	rows, err := s.chatApplication().ListSessions(r.Context(), sess.UserID, accountID, parsePositiveInt(r.URL.Query().Get("limit"), 200))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取聊天会话失败")
		return
	}
	if refresh {
		// resolveCtx 和 resolveCancel 限制联系人身份补全的总时长。
		resolveCtx, resolveCancel := context.WithTimeout(r.Context(), 25*time.Second)
		// refreshedRows 和 sessionErr 保存应用层身份补全结果及首个平台错误。
		refreshedRows, sessionErr := s.chatApplication().RefreshSessionIdentities(resolveCtx, accountID, rows)
		resolveCancel()
		rows = refreshedRows
		if sessionErr != nil {
			s.recoverExpiredSession(r.Context(), accountID, sessionErr)
		}
	}
	writeJSON(w, http.StatusOK, chatSessionPageResponse{Sessions: newChatSessionDTOsFromApplication(rows), HasMore: hasMore, NextCursor: nextCursor})
}

// setChatSessionPinned 解析版本化会话置顶请求，将归属和幂等写入交给聊天应用服务。
func (s *Server) setChatSessionPinned(w http.ResponseWriter, r *http.Request) {
	// session 是当前已通过认证中间件的 ERP 登录会话。
	session := auth.SessionFromContext(r.Context())
	// chatID 是路径中需要更新偏好的精确聊天会话标识。
	chatID := strings.TrimSpace(chi.URLParam(r, "chat_id"))
	// input 是当前请求中的账号和显式置顶布尔值。
	var input chatSessionPinRequest
	if // decodeErr 是解码具名置顶请求时的 JSON 格式或大小错误。
	decodeErr := decodeJSON(r, &input); decodeErr != nil || chatID == "" || strings.TrimSpace(input.AccountID) == "" || input.Pinned == nil {
		writeErr(w, http.StatusBadRequest, "会话置顶参数无效")
		return
	}
	// accountID 是去除空白后交给应用层校验归属的账号标识。
	accountID := strings.TrimSpace(input.AccountID)
	// pinErr 是应用服务返回的输入、归属、会话或存储错误。
	pinErr := s.chatApplication().SetSessionPinned(r.Context(), session.UserID, accountID, chatID, *input.Pinned)
	if pinErr != nil {
		switch {
		case errors.Is(pinErr, chatapp.ErrInvalidInput):
			writeErr(w, http.StatusBadRequest, "会话置顶参数无效")
		case errors.Is(pinErr, chatapp.ErrSessionForbidden):
			writeErr(w, http.StatusForbidden, "无权修改该账号会话")
		case errors.Is(pinErr, chatapp.ErrSessionNotFound):
			writeErr(w, http.StatusNotFound, "聊天会话不存在")
		case errors.Is(pinErr, chatapp.ErrSessionUnavailable):
			writeErr(w, http.StatusServiceUnavailable, "聊天会话服务未启用")
		default:
			writeErr(w, http.StatusInternalServerError, "保存会话置顶失败")
		}
		return
	}
	writeJSON(w, http.StatusOK, chatSessionPinResponse{AccountID: accountID, ChatID: chatID, Pinned: *input.Pinned})
}

// sendChatImage 封装send聊天图片业务协调。
func (s *Server) sendChatImage(w http.ResponseWriter, r *http.Request) {
	if !s.chatApplication().ImageUploadAvailable() {
		writeErr(w, http.StatusServiceUnavailable, "聊天服务未启用")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if // err 用于本次流程后续判断的err
	err := r.ParseMultipartForm(10 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "图片不能为空且不能超过 10MB")
		return
	}
	// accountID 用于本次流程后续判断的账号ID
	accountID := strings.TrimSpace(r.FormValue("account_id"))
	// chatID 用于本次流程后续判断的聊天ID
	chatID := strings.TrimSpace(r.FormValue("chat_id"))
	// buyerID 用于本次流程后续判断的买家ID
	buyerID := strings.TrimSpace(r.FormValue("buyer_id"))
	if !s.ownsAccount(r, accountID) {
		writeErr(w, http.StatusForbidden, "无权操作该账号")
		return
	}
	if chatID == "" || buyerID == "" {
		writeErr(w, http.StatusBadRequest, "会话和买家不能为空")
		return
	}
	// file、header、err 用于本次流程后续判断的file、header、err
	file, header, err := r.FormFile("image")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "请选择图片")
		return
	}
	defer file.Close()
	// contentType 用于本次流程后续判断的内容类型
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		writeErr(w, http.StatusBadRequest, "只支持图片文件")
		return
	}
	// data、err 用于本次流程后续判断的data、err
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 10<<20 {
		writeErr(w, http.StatusBadRequest, "图片不能为空且不能超过 10MB")
		return
	}
	// session 保存已完成账号归属校验的应用层会话摘要。
	session := chatapp.Session{AccountID: accountID, ChatID: chatID, BuyerID: buyerID,
		BuyerName: r.FormValue("buyer_name"), BuyerAvatar: r.FormValue("buyer_avatar_url"),
		ItemID: r.FormValue("item_id"), ItemTitle: r.FormValue("item_title")}
	// sent、err 用于本次流程后续判断的sent、err
	sent, err := s.chatApplication().SendImage(r.Context(), chatapp.ImageInput{Session: session, Filename: header.Filename, ContentType: contentType, Data: data})
	if err != nil {
		if errors.Is(err, chatapp.ErrUnavailable) {
			writeErr(w, http.StatusServiceUnavailable, "图片上传服务未启用")
		} else if errors.Is(err, chatapp.ErrOffline) {
			writeErr(w, http.StatusConflict, "账号当前离线，无法发送图片")
		} else if errors.Is(err, chatapp.ErrSend) {
			writeErrDetails(w, http.StatusBadGateway, "chat_image_send_failed", "图片发送失败，请重试", "", map[string]any{"outgoing_message": sent})
		} else if errors.Is(err, chatapp.ErrStatusSave) {
			writeErr(w, http.StatusInternalServerError, "图片已发送，但状态保存失败")
		} else {
			writeErr(w, http.StatusInternalServerError, "保存待发送图片失败")
		}
		return
	}
	writeJSON(w, http.StatusCreated, chatMessageEnvelope{Message: newChatMessageDTOFromApplication(sent)})
}

// listChatMessages 封装list聊天消息列表业务协调。
func (s *Server) listChatMessages(w http.ResponseWriter, r *http.Request) {
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// accountID 用于本次流程后续判断的账号ID
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	// chatID 用于本次流程后续判断的聊天ID
	chatID := strings.TrimSpace(r.URL.Query().Get("chat_id"))
	if !s.ownsAccount(r, accountID) {
		writeErr(w, http.StatusForbidden, "无权访问该账号")
		return
	}
	if chatID == "" {
		writeErr(w, http.StatusBadRequest, "缺少 chat_id")
		return
	}
	// beforeID 用于本次流程后续判断的beforeID
	beforeID, _ := strconv.ParseInt(r.URL.Query().Get("before_id"), 10, 64)
	// cursor 用于本次流程后续判断的游标
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	// limit 用于本次流程后续判断的上限
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 50)
	// current 保存刷新前的本地会话摘要，供平台历史写入和响应展示使用。
	current, _ := s.chatApplication().FindSession(r.Context(), sess.UserID, accountID, chatID)
	// fetchCtx 和 cancel 限制平台历史刷新请求的最长时间。
	fetchCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	// refreshed 和 fetchErr 保存应用层消息分页结果及平台/持久化错误。
	refreshed, fetchErr := s.chatApplication().RefreshHistory(fetchCtx, accountID, chatID, cursor, limit, current)
	cancel()
	if fetchErr == nil {
		// resolved 和 identityErr 保存身份补全后的会话及平台查询错误。
		resolved, identityErr := s.chatApplication().ResolveSessionIdentity(r.Context(), refreshed.Session)
		if identityErr != nil {
			s.recoverExpiredSession(r.Context(), accountID, identityErr)
		}
		writeJSON(w, http.StatusOK, chatMessagePageResponse{Messages: newChatMessageDTOsFromApplication(refreshed.Messages), HasMore: refreshed.HasMore, NextCursor: refreshed.NextCursor, Session: newChatSessionDTOFromApplication(resolved)})
		return
	}
	if errors.Is(fetchErr, chatapp.ErrRefreshPersist) {
		writeErr(w, http.StatusInternalServerError, "保存聊天历史失败")
		return
	}
	if !errors.Is(fetchErr, chatapp.ErrRefreshUnavailable) && !errors.Is(fetchErr, chatapp.ErrOffline) {
		s.recoverExpiredSession(r.Context(), accountID, fetchErr)
	}
	// page、err 用于本次流程后续判断的page、err
	page, err := s.chatApplication().ListStoredMessages(r.Context(), sess.UserID, accountID, chatID, beforeID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取聊天消息失败")
		return
	}
	// session 是应用层返回的非敏感会话摘要，供平台身份适配器补齐展示名称。
	session := page.Session
	if session.ChatID != "" {
		// resolved 和 identityErr 保存身份补全后的会话及平台查询错误。
		resolved, identityErr := s.chatApplication().ResolveSessionIdentity(r.Context(), session)
		if identityErr != nil {
			s.recoverExpiredSession(r.Context(), accountID, identityErr)
		}
		session = resolved
	}
	writeJSON(w, http.StatusOK, chatMessagePageResponse{Messages: newChatMessageDTOsFromApplication(page.Messages), HasMore: page.HasMore, Session: newChatSessionDTOFromApplication(session)})
}

// sendChatMessageRequest 用于本次流程后续判断的send聊天消息请求
type sendChatMessageRequest struct {
	// AccountID 是当前发送账号标识。
	AccountID string `json:"account_id"`
	// ChatID 是当前单聊会话标识。
	ChatID string `json:"chat_id"`
	// BuyerID 是当前会话对方的平台标识。
	BuyerID string `json:"buyer_id"`
	// BuyerName 是会话展示名称，只用于本地摘要。
	BuyerName string `json:"buyer_name"`
	// ItemID 是会话已关联的商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是会话已关联的商品标题。
	ItemTitle string `json:"item_title"`
	// Text 是待发送的文字正文。
	Text string `json:"text"`
	// ReplyToMessageKey 是可选的本地引用目标键，应用层将校验它的账号和会话归属。
	ReplyToMessageKey string `json:"reply_to_message_key"`
}

// sendChatLocationCardRequest 是发送自定义位置卡片的具名请求 DTO。
type sendChatLocationCardRequest struct {
	// AccountID 是当前发送账号标识。
	AccountID string `json:"account_id"`
	// ChatID 是当前单聊会话标识。
	ChatID string `json:"chat_id"`
	// BuyerID 是当前会话对方的平台标识。
	BuyerID string `json:"buyer_id"`
	// BuyerName 是会话展示名称，只用于本地摘要。
	BuyerName string `json:"buyer_name"`
	// ItemID 是会话关联商品标识。
	ItemID string `json:"item_id"`
	// ItemTitle 是会话关联商品标题。
	ItemTitle string `json:"item_title"`
	// Title 是卡片主标题，例如实体店名称。
	Title string `json:"title"`
	// Description 是卡片说明，例如门牌和到店指引。
	Description string `json:"description"`
	// Latitude 是 WGS84 纬度十进制度。
	Latitude float64 `json:"latitude"`
	// Longitude 是 WGS84 经度十进制度。
	Longitude float64 `json:"longitude"`
}

// recallChatMessageRequest 是撤回接口的具名请求 DTO。
type recallChatMessageRequest struct {
	// AccountID 是原消息所属的本地账号标识。
	AccountID string `json:"account_id"`
}

// sendChatMessage 封装send聊天消息业务协调。
func (s *Server) sendChatMessage(w http.ResponseWriter, r *http.Request) {
	s.sendChatTextMessage(w, r, false)
}

// sendChatLocationCard 校验账号归属与位置字段后，委托应用服务发送一张 contentType=30 卡片。
func (s *Server) sendChatLocationCard(w http.ResponseWriter, r *http.Request) {
	if !s.chatApplication().SendingAvailable() {
		writeErr(w, http.StatusServiceUnavailable, "聊天服务未启用")
		return
	}
	// input 是当前用户确认提交的位置卡片和会话字段。
	var input sendChatLocationCardRequest
	// decodeErr 是具名位置请求无法从受限 JSON 正文解码时的格式错误。
	if decodeErr := decodeJSON(r, &input); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	input.AccountID, input.ChatID, input.BuyerID = strings.TrimSpace(input.AccountID), strings.TrimSpace(input.ChatID), strings.TrimSpace(input.BuyerID)
	input.Title, input.Description = strings.TrimSpace(input.Title), strings.TrimSpace(input.Description)
	if !s.ownsAccount(r, input.AccountID) {
		writeErr(w, http.StatusForbidden, "无权操作该账号")
		return
	}
	if input.ChatID == "" || input.BuyerID == "" || input.Title == "" || input.Description == "" {
		writeErr(w, http.StatusBadRequest, "会话、买家、标题和说明不能为空")
		return
	}
	// sent、sendErr 是应用层位置卡片结果及发送或持久化错误。
	sent, sendErr := s.chatApplication().SendLocationCard(r.Context(), chatapp.LocationInput{
		Session: chatapp.Session{AccountID: input.AccountID, ChatID: input.ChatID, BuyerID: input.BuyerID, BuyerName: input.BuyerName, ItemID: input.ItemID, ItemTitle: input.ItemTitle},
		Title:   input.Title, Description: input.Description, Latitude: input.Latitude, Longitude: input.Longitude,
	})
	if sendErr != nil {
		switch {
		case errors.Is(sendErr, chatapp.ErrSendInvalidInput):
			writeErr(w, http.StatusBadRequest, "位置卡片标题、说明或坐标无效")
		case errors.Is(sendErr, chatapp.ErrUnavailable):
			writeErr(w, http.StatusServiceUnavailable, "位置卡片发送服务未启用")
		case errors.Is(sendErr, chatapp.ErrOffline):
			writeErr(w, http.StatusConflict, "账号当前离线，无法发送位置卡片")
		case errors.Is(sendErr, chatapp.ErrSend):
			writeErrDetails(w, http.StatusBadGateway, "chat_location_send_failed", "位置卡片发送失败，请重试", "", map[string]any{"outgoing_message": sent})
		case errors.Is(sendErr, chatapp.ErrStatusSave):
			writeErr(w, http.StatusInternalServerError, "位置卡片已发送，但状态保存失败")
		default:
			writeErr(w, http.StatusInternalServerError, "保存待发送位置卡片失败")
		}
		return
	}
	writeJSON(w, http.StatusCreated, chatMessageEnvelope{Message: newChatMessageDTOFromApplication(sent)})
}

// sendChatReplyMessage 处理必须携带本地目标键的原生引用文本请求。
func (s *Server) sendChatReplyMessage(w http.ResponseWriter, r *http.Request) {
	s.sendChatTextMessage(w, r, true)
}

// sendChatTextMessage 共享普通文本和原生引用文本的 HTTP 验证、应用调用和错误映射。
func (s *Server) sendChatTextMessage(w http.ResponseWriter, r *http.Request, replyRequired bool) {
	if !s.chatApplication().SendingAvailable() {
		writeErr(w, http.StatusServiceUnavailable, "聊天服务未启用")
		return
	}
	// input 用于本次流程后续判断的input
	var input sendChatMessageRequest
	if // err 用于本次流程后续判断的err
	err := decodeJSON(r, &input); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	input.AccountID, input.ChatID, input.BuyerID = strings.TrimSpace(input.AccountID), strings.TrimSpace(input.ChatID), strings.TrimSpace(input.BuyerID)
	input.Text = strings.TrimSpace(input.Text)
	input.ReplyToMessageKey = strings.TrimSpace(input.ReplyToMessageKey)
	if !s.ownsAccount(r, input.AccountID) {
		writeErr(w, http.StatusForbidden, "无权操作该账号")
		return
	}
	// session 是当前已认证 ERP 用户身份，用于引用目标归属校验。
	session := auth.SessionFromContext(r.Context())
	if input.ChatID == "" || input.BuyerID == "" || input.Text == "" {
		writeErr(w, http.StatusBadRequest, "会话、买家和消息内容不能为空")
		return
	}
	if replyRequired && input.ReplyToMessageKey == "" {
		writeErr(w, http.StatusBadRequest, "引用回复缺少目标消息")
		return
	}
	if !replyRequired && input.ReplyToMessageKey != "" {
		writeErr(w, http.StatusBadRequest, "引用回复请使用专用接口")
		return
	}
	if len([]rune(input.Text)) > 2000 {
		writeErr(w, http.StatusBadRequest, "消息不能超过 2000 个字符")
		return
	}
	// sent、err 保存应用层发送结果及错误；应用层返回的消息不含凭证。
	sent, err := s.chatApplication().SendText(r.Context(), chatapp.OutgoingInput{Session: chatapp.Session{AccountID: input.AccountID, ChatID: input.ChatID, BuyerID: input.BuyerID, BuyerName: input.BuyerName, ItemID: input.ItemID, ItemTitle: input.ItemTitle}, Text: input.Text, UserID: session.UserID, ReplyToMessageKey: input.ReplyToMessageKey})
	if err != nil {
		if errors.Is(err, chatapp.ErrUnavailable) {
			writeErr(w, http.StatusServiceUnavailable, "聊天服务未启用")
		} else if errors.Is(err, chatapp.ErrOffline) {
			writeErr(w, http.StatusConflict, "账号当前离线，无法发送消息")
		} else if errors.Is(err, chatapp.ErrSend) {
			writeErrDetails(w, http.StatusBadGateway, "chat_message_send_failed", "发送失败，请重试", "", map[string]any{"outgoing_message": sent})
		} else if errors.Is(err, chatapp.ErrStatusSave) {
			writeErr(w, http.StatusInternalServerError, "消息已发送，但状态保存失败")
		} else if errors.Is(err, chatapp.ErrReplyNotFound) {
			writeErr(w, http.StatusNotFound, "被回复消息不存在或已不可访问")
		} else if errors.Is(err, chatapp.ErrReplyNotAllowed) {
			writeErr(w, http.StatusConflict, "被回复消息不属于当前会话或缺少平台标识")
		} else {
			writeErr(w, http.StatusInternalServerError, "保存待发送消息失败")
		}
		return
	}
	writeJSON(w, http.StatusCreated, chatMessageEnvelope{Message: newChatMessageDTOFromApplication(sent)})
}

// recallChatMessage 撤回当前用户拥有账号在两分钟内发出的文本或图片消息。
func (s *Server) recallChatMessage(w http.ResponseWriter, r *http.Request) {
	// input 保存请求中的账号标识；消息键来自版本化路径参数。
	var input recallChatMessageRequest
	// err 表示撤回请求 JSON 解码失败。
	if err := decodeJSON(r, &input); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	input.AccountID = strings.TrimSpace(input.AccountID)
	// messageKey 是前端持有的本地幂等消息键。
	messageKey := strings.TrimSpace(chi.URLParam(r, "messageKey"))
	// session 保存当前认证用户，用于应用层原子归属查询。
	session := auth.SessionFromContext(r.Context())
	// message、err 保存撤回后的消息或失败时最新可观测状态。
	message, err := s.chatApplication().Recall(r.Context(), chatapp.RecallInput{UserID: session.UserID, AccountID: input.AccountID, MessageKey: messageKey})
	if err == nil {
		writeJSON(w, http.StatusOK, chatMessageEnvelope{Message: newChatMessageDTOFromApplication(message)})
		return
	}
	switch {
	case errors.Is(err, chatapp.ErrRecallInvalidInput):
		writeErr(w, http.StatusBadRequest, "账号和消息键不能为空")
	case errors.Is(err, chatapp.ErrRecallNotFound):
		writeErr(w, http.StatusNotFound, "消息不存在")
	case errors.Is(err, chatapp.ErrRecallExpired):
		writeErr(w, http.StatusConflict, "消息已超过两分钟撤回期限")
	case errors.Is(err, chatapp.ErrRecallNotReady):
		writeErr(w, http.StatusConflict, "平台消息标识尚未同步，请稍后重试")
	case errors.Is(err, chatapp.ErrRecallAlready):
		writeErr(w, http.StatusConflict, "消息已经撤回")
	case errors.Is(err, chatapp.ErrRecallNotAllowed):
		writeErr(w, http.StatusConflict, "当前消息不可撤回")
	case errors.Is(err, chatapp.ErrOffline):
		writeErr(w, http.StatusConflict, "账号当前离线，无法撤回消息")
	case errors.Is(err, chatapp.ErrRecallUnavailable), errors.Is(err, chatapp.ErrUnavailable):
		writeErr(w, http.StatusServiceUnavailable, "聊天撤回服务未启用")
	case errors.Is(err, chatapp.ErrRecallUncertain):
		writeJSON(w, http.StatusAccepted, chatMessageEnvelope{Message: newChatMessageDTOFromApplication(message)})
	case errors.Is(err, chatapp.ErrRecall):
		writeErr(w, http.StatusBadGateway, "闲鱼拒绝撤回消息")
	case errors.Is(err, chatapp.ErrStatusSave):
		writeErr(w, http.StatusInternalServerError, "消息已撤回，但本地状态保存失败")
	default:
		writeErr(w, http.StatusInternalServerError, "撤回消息失败")
	}
}

// markChatRead 封装mark聊天Read业务协调。
func (s *Server) markChatRead(w http.ResponseWriter, r *http.Request) {
	// input 是聊天已读请求的具名传输 DTO。
	var input markChatReadRequest
	if decodeJSON(r, &input) != nil || input.ChatID == "" {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if !s.ownsAccount(r, input.AccountID) {
		writeErr(w, http.StatusForbidden, "无权操作该账号")
		return
	}
	// sess 保存当前认证用户，用于本地未读状态归属隔离。
	sess := auth.SessionFromContext(r.Context())
	slog.Debug("收到聊天已读请求", "account", input.AccountID, "chat_id", input.ChatID, "message_count", len(input.MessageIDs))
	if len(input.MessageIDs) == 0 {
		// page 保存应用层返回的本地消息页，只含非敏感字段。
		page, listErr := s.chatApplication().ListStoredMessages(r.Context(), sess.UserID, input.AccountID, input.ChatID, 0, 200)
		if listErr == nil {
			// message 是当前用于补全平台已读消息标识的入站消息。
			for _, message := range page.Messages {
				if message.Direction == "incoming" && message.MessageType != "system" {
					input.MessageIDs = append(input.MessageIDs, map[string]any{"messageId": message.MessageKey})
				}
			}
		}
	}
	// 旧版本把实时 WS 通知里的 bizTag/extJson messageId 当成了平台消息
	// ID，数据库里会留下 32 位关联 ID。闲鱼的 read 接口实际要求 1.3 的
	// PNM ID；这里从已保存的解密 WS 诊断帧把旧 ID 转回 PNM，避免升级后
	// 仍有历史实时消息无法被标记已读。
	input.MessageIDs = s.resolveChatReadMessageIDs(r.Context(), input.AccountID, input.ChatID, input.MessageIDs)
	if // err 保存应用层已读状态更新错误。
	err := s.chatApplication().MarkRead(r.Context(), sess.UserID, input.AccountID, input.ChatID); err != nil {
		writeErr(w, http.StatusInternalServerError, "更新已读状态失败")
		return
	}
	// reportErr 表示平台已读上报失败；本地已读状态已成功保存，不能回滚。
	if reportErr := s.chatApplication().ReportPlatformRead(r.Context(), input.AccountID, input.ChatID, input.MessageIDs); reportErr != nil {
		slog.Warn("上报闲鱼已读状态失败", "account", input.AccountID, "chat_id", input.ChatID, "err", reportErr)
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// resolveChatReadMessageIDs 将旧版关联标识交给应用服务解析，并移除无效或重复的已读项。
func (s *Server) resolveChatReadMessageIDs(ctx context.Context, accountID, chatID string, messageIDs []map[string]any) []map[string]any {
	// resolved 保存可安全提交给平台的去重消息标识列表。
	resolved := make([]map[string]any, 0, len(messageIDs))
	// seen 保存已加入结果的平台 PNM 标识，避免重复上报。
	seen := make(map[string]struct{}, len(messageIDs))
	// item 是当前待转换的已读消息参数。
	for _, item := range messageIDs {
		// rawID 保存请求携带的原始消息标识。
		rawID, ok := item["messageId"].(string)
		if !ok || strings.TrimSpace(rawID) == "" {
			continue
		}
		// id 保存应用层解析后的平台消息标识。
		id := s.chatApplication().ResolveReadMessageID(ctx, accountID, chatID, rawID)
		if !strings.HasSuffix(id, ".PNM") {
			slog.Warn("未找到旧聊天消息对应的 PNM，跳过已读上报", "account", accountID, "chat_id", chatID, "message_id", rawID)
			continue
		}
		// exists 表示该平台消息标识是否已经加入本次上报请求。
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		// copyItem 保存保留其他平台参数、只替换消息标识的请求副本。
		copyItem := make(map[string]any, len(item)+1)
		// key、value 保存当前平台参数名及其原始值。
		for key, value := range item {
			copyItem[key] = value
		}
		copyItem["messageId"] = id
		resolved = append(resolved, copyItem)
	}
	return resolved
}

// findChatPlatformMessageID 保留既有包内测试入口，实际解析由聊天应用服务拥有。
func findChatPlatformMessageID(value any, chatID, legacyID string) string {
	return chatapp.FindPlatformMessageID(value, chatID, legacyID)
}

// chatWebSocket 将应用层聊天事件转发到当前认证用户的 WebSocket 连接。
func (s *Server) chatWebSocket(w http.ResponseWriter, r *http.Request) {
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// events、unsubscribe、err 保存应用层实时事件、清理函数和订阅错误。
	events, unsubscribe, err := s.chatApplication().Subscribe(r.Context(), sess.UserID)
	if err != nil {
		if errors.Is(err, chatapp.ErrSubscriptionUnavailable) {
			writeErr(w, http.StatusServiceUnavailable, "聊天服务未启用")
		} else {
			writeErr(w, http.StatusInternalServerError, "订阅聊天消息失败")
		}
		return
	}
	// conn、err 用于本次流程后续判断的conn、err
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		unsubscribe()
		return
	}
	// ctx、cancel 用于本次流程后续判断的ctx、cancel
	ctx, cancel := context.WithCancel(r.Context())
	conn.SetReadLimit(8 << 10)
	// readerWG 等待读取 goroutine 在连接关闭后退出，避免请求返回时遗留后台任务。
	var readerWG sync.WaitGroup
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		for {
			if // readErr 用于本次流程后续判断的readErr
			_, _, readErr := conn.Read(ctx); readErr != nil {
				cancel()
				return
			}
		}
	}()
	// cleanup 统一负责取消请求、关闭 WebSocket、等待读取任务和释放聊天订阅。
	cleanup := func() {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		readerWG.Wait()
		unsubscribe()
	}
	defer cleanup()
	if // err 用于本次流程后续判断的err
	err := wsjson.Write(ctx, conn, map[string]any{"type": "ready", "at": time.Now().UTC().UnixMilli()}); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case // event、ok 用于本次流程后续判断的event、ok
		event, ok := <-events:
			if !ok || wsjson.Write(ctx, conn, newChatEventDTOFromApplication(event)) != nil {
				return
			}
		}
	}
}

// ownsAccount 封装owns账号业务协调。
func (s *Server) ownsAccount(r *http.Request, accountID string) bool {
	if accountID == "" {
		return false
	}
	// sess 用于本次流程后续判断的sess
	sess := auth.SessionFromContext(r.Context())
	// owned 和 err 表示聊天应用端口返回的账号归属及查询错误。
	owned, err := s.chatApplication().OwnsAccount(r.Context(), sess.UserID, accountID)
	return err == nil && owned
}

/*
账号查询已采用所有权窄接口。
*/
// parsePositiveInt 将正整数文本转换为整数，无法解析时返回备用值。
// parsePositiveInt 封装parsePositiveInt业务协调。
func parsePositiveInt(raw string, fallback int) int {
	// value、err 用于本次流程后续判断的value、err
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
