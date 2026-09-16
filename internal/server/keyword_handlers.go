package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"xianyu-go/internal/application/keywords"
	"xianyu-go/internal/auth"
)

// keywordRequest 是普通关键词接口使用的文字回复请求 DTO。
type keywordRequest struct {
	// Keyword 是触发匹配文本。
	Keyword string `json:"keyword"`
	// Reply 是文字回复正文。
	Reply string `json:"reply"`
}

// keywordBatchItem 是带商品范围的关键词批量项 DTO。
type keywordBatchItem struct {
	// Keyword 是触发匹配文本。
	Keyword string `json:"keyword"`
	// Reply 是文字回复正文。
	Reply string `json:"reply"`
	// ItemID 是可选商品标识。
	ItemID string `json:"item_id"`
	// Type 是 text 或 image 回复类型。
	Type string `json:"type"`
	// ImageURL 是图片回复地址。
	ImageURL string `json:"image_url"`
}

// keywordBatchRequest 是关键词批量替换或单项创建请求 DTO。
type keywordBatchRequest struct {
	// Keyword 是单项创建的触发匹配文本。
	Keyword string `json:"keyword"`
	// Reply 是单项创建的文字回复正文。
	Reply string `json:"reply"`
	// ItemID 是单项创建的可选商品标识。
	ItemID string `json:"item_id"`
	// Type 是单项创建的回复类型。
	Type string `json:"type"`
	// ImageURL 是单项创建的图片回复地址。
	ImageURL string `json:"image_url"`
	// Keywords 是批量替换项；为空指针表示请求使用单项模式。
	Keywords *[]keywordBatchItem `json:"keywords"`
}

// keywordUpdateRequest 是按 ID 更新关键词的请求 DTO。
type keywordUpdateRequest struct {
	// Keyword 是触发匹配文本。
	Keyword string `json:"keyword"`
	// Reply 是文字回复正文。
	Reply string `json:"reply"`
	// ItemID 是可选商品标识。
	ItemID string `json:"item_id"`
	// Type 是 text 或 image 回复类型。
	Type string `json:"type"`
	// ImageURL 是图片回复地址。
	ImageURL string `json:"image_url"`
}

// keywordGroupRequest 是多关键词共享回复的创建或更新 DTO。
type keywordGroupRequest struct {
	// Keywords 是任意一个命中即可触发回复的关键词集合。
	Keywords []string `json:"keywords"`
	// Reply 是文字回复正文。
	Reply string `json:"reply"`
	// ItemID 是可选商品范围。
	ItemID string `json:"item_id"`
	// Type 是 text 或 image。
	Type string `json:"type"`
	// ImageURL 是图片回复地址。
	ImageURL string `json:"image_url"`
	// MatchType 是 contains、excludes 或 equals。
	MatchType string `json:"match_type"`
	// MessageScope 是 customer 或 system。
	MessageScope string `json:"message_scope"`
	// MessageScopes 是可同时选择的客户和系统消息来源。
	MessageScopes []string `json:"message_scopes"`
	// SystemTypes 是允许触发的系统消息 contentType 白名单。
	SystemTypes []string `json:"system_types"`
	// AccountIDs 是执行全局规则组的店铺账号集合。
	AccountIDs []string `json:"account_ids"`
	// Enabled 是可选启用状态；缺失时由应用层按开启处理。
	Enabled *bool `json:"enabled"`
	// ReplyIntervalSeconds 是同组重复回复冷却秒数。
	ReplyIntervalSeconds int64 `json:"reply_interval_seconds"`
	// SendDelaySeconds 是命中后等待发送秒数。
	SendDelaySeconds int64 `json:"send_delay_seconds"`
}

// keywordGroupResponse 是规则页使用的多关键词分组 DTO。
type keywordGroupResponse struct {
	// GroupID 是规则组稳定标识。
	GroupID string `json:"group_id"`
	// Keywords 是组内全部独立匹配词。
	Keywords []string `json:"keywords"`
	// Reply 是文字回复正文。
	Reply string `json:"reply"`
	// ItemID 是可选商品范围。
	ItemID string `json:"item_id"`
	// Type 是 text 或 image。
	Type string `json:"type"`
	// ImageURL 是图片回复地址。
	ImageURL string `json:"image_url"`
	// MatchType 是规则组匹配逻辑。
	MatchType string `json:"match_type"`
	// MessageScope 是规则组消息来源。
	MessageScope string `json:"message_scope"`
	// MessageScopes 是规则组消息来源集合。
	MessageScopes []string `json:"message_scopes"`
	// SystemTypes 是系统消息白名单。
	SystemTypes []string `json:"system_types"`
	// AccountIDs 是当前规则组绑定的店铺账号集合。
	AccountIDs []string `json:"account_ids"`
	// Enabled 表示规则组是否参与消息匹配。
	Enabled bool `json:"enabled"`
	// AccountStates 是各绑定店铺的独立启用状态。
	AccountStates []keywordAccountStateResponse `json:"account_states"`
	// ReplyIntervalSeconds 是同组重复回复冷却秒数。
	ReplyIntervalSeconds int64 `json:"reply_interval_seconds"`
	// SendDelaySeconds 是命中后等待发送秒数。
	SendDelaySeconds int64 `json:"send_delay_seconds"`
}

// keywordAccountStateResponse 是规则组单店铺启用状态 DTO。
type keywordAccountStateResponse struct {
	// AccountID 是店铺账号标识。
	AccountID string `json:"account_id"`
	// Enabled 表示店铺是否执行规则。
	Enabled bool `json:"enabled"`
}

// keywordAccountStatesResponse 转换应用层店铺状态为 HTTP DTO。
func keywordAccountStatesResponse(states []keywords.AccountState) []keywordAccountStateResponse {
	// response 保存逐店铺启用状态。
	response := make([]keywordAccountStateResponse, 0, len(states))
	// state 是当前转换的店铺状态。
	for _, state := range states {
		response = append(response, keywordAccountStateResponse{AccountID: state.AccountID, Enabled: state.Enabled})
	}
	return response
}

// keywordEnabledRequest 是单条和全部关键词规则启停使用的具名 DTO。
type keywordEnabledRequest struct {
	// Enabled 是用户希望保存的目标启用状态。
	Enabled bool `json:"enabled"`
}

// listGlobalKeywordGroups 返回当前用户跨店铺聚合的全局关键词规则组。
func (s *Server) listGlobalKeywordGroups(w http.ResponseWriter, r *http.Request) {
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// groups、listErr 是应用层全局规则组和读取错误。
	groups, listErr := s.keywordApplication().ListGlobalGroups(r.Context(), userID)
	if listErr != nil {
		writeKeywordError(w, listErr, "查询全局规则组失败")
		return
	}
	// response 是全局规则组具名 DTO 列表。
	response := make([]keywordGroupResponse, 0, len(groups))
	// group 是当前映射的全局规则组。
	for _, group := range groups {
		response = append(response, keywordGroupResponse{GroupID: group.GroupID, Keywords: group.Keywords, Reply: group.Reply, ItemID: group.ItemID, Type: group.Type, ImageURL: group.ImageURL, MatchType: group.MatchType, MessageScope: group.MessageScope, MessageScopes: group.MessageScopes, SystemTypes: group.SystemTypes, AccountIDs: group.AccountIDs, Enabled: group.Enabled, AccountStates: keywordAccountStatesResponse(group.AccountStates), ReplyIntervalSeconds: group.ReplyIntervalSeconds, SendDelaySeconds: group.SendDelaySeconds})
	}
	writeJSON(w, http.StatusOK, response)
}

// setGlobalKeywordGroupAccountEnabled 切换一个规则组在指定店铺中的状态。
func (s *Server) setGlobalKeywordGroupAccountEnabled(w http.ResponseWriter, r *http.Request) {
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是用户提交的单店铺目标状态。
	var request keywordEnabledRequest
	if // decodeErr 是单店铺启停请求 JSON 解析错误。
	decodeErr := decodeJSON(r, &request); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "店铺关键词规则启停格式错误")
		return
	}
	// updateErr 是应用层单店铺启停结果。
	updateErr := s.keywordApplication().SetGlobalGroupAccountEnabled(r.Context(), userID, chi.URLParam(r, "group_id"), chi.URLParam(r, "account_id"), request.Enabled)
	if updateErr != nil {
		writeKeywordError(w, updateErr, "更新店铺关键词规则状态失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// saveGlobalKeywordGroup 创建或替换全局规则组及全部店铺绑定。
func (s *Server) saveGlobalKeywordGroup(w http.ResponseWriter, r *http.Request) {
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是包含适用店铺的全局规则组请求。
	var request keywordGroupRequest
	if // decodeErr 是全局规则组 JSON 请求解析错误。
	decodeErr := decodeJSON(r, &request); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "全局关键词规则组格式错误")
		return
	}
	// groupID 是更新路径中的规则组标识，创建时为空。
	groupID := chi.URLParam(r, "group_id")
	// savedID、saveErr 是应用层全局保存结果和错误。
	savedID, saveErr := s.keywordApplication().SaveGlobalGroup(r.Context(), userID, keywords.GroupDraft{GroupID: groupID, Keywords: request.Keywords, Reply: request.Reply, ItemID: request.ItemID, Type: request.Type, ImageURL: request.ImageURL, MatchType: request.MatchType, MessageScope: request.MessageScope, MessageScopes: request.MessageScopes, SystemTypes: request.SystemTypes, AccountIDs: request.AccountIDs, Enabled: request.Enabled, ReplyIntervalSeconds: request.ReplyIntervalSeconds, SendDelaySeconds: request.SendDelaySeconds})
	if saveErr != nil {
		writeKeywordError(w, saveErr, "保存全局规则组失败")
		return
	}
	writeJSON(w, http.StatusOK, keywordGroupMutationResponse{Success: true, GroupID: savedID})
}

// setGlobalKeywordGroupEnabled 切换当前用户一个全局关键词规则组。
func (s *Server) setGlobalKeywordGroupEnabled(w http.ResponseWriter, r *http.Request) {
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是用户提交的目标启用状态。
	var request keywordEnabledRequest
	if // decodeErr 是启停请求 JSON 解析错误。
	decodeErr := decodeJSON(r, &request); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "关键词规则启停格式错误")
		return
	}
	// updateErr 是应用层单规则启停结果。
	updateErr := s.keywordApplication().SetGlobalGroupEnabled(r.Context(), userID, chi.URLParam(r, "group_id"), request.Enabled)
	if updateErr != nil {
		writeKeywordError(w, updateErr, "更新关键词规则状态失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// setAllGlobalKeywordGroupsEnabled 切换当前用户全部全局关键词规则组。
func (s *Server) setAllGlobalKeywordGroupsEnabled(w http.ResponseWriter, r *http.Request) {
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是用户提交的全部规则目标启用状态。
	var request keywordEnabledRequest
	if // decodeErr 是批量启停请求 JSON 解析错误。
	decodeErr := decodeJSON(r, &request); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "全部关键词规则启停格式错误")
		return
	}
	// updateErr 是应用层全部规则启停结果。
	updateErr := s.keywordApplication().SetAllGlobalGroupsEnabled(r.Context(), userID, request.Enabled)
	if updateErr != nil {
		writeKeywordError(w, updateErr, "更新全部关键词规则状态失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// deleteGlobalKeywordGroup 从当前用户全部店铺删除规则组。
func (s *Server) deleteGlobalKeywordGroup(w http.ResponseWriter, r *http.Request) {
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// deleteErr 是全局规则组删除应用错误。
	deleteErr := s.keywordApplication().DeleteGlobalGroup(r.Context(), userID, chi.URLParam(r, "group_id"))
	if deleteErr != nil {
		writeKeywordError(w, deleteErr, "删除全局规则组失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// keywordGroupMutationResponse 是规则组保存后的具名响应 DTO。
type keywordGroupMutationResponse struct {
	// Success 表示规则组已经成功保存。
	Success bool `json:"success"`
	// GroupID 是创建或更新后的稳定规则组标识。
	GroupID string `json:"group_id"`
}

// listKeywordGroups 返回当前账号按组聚合的多关键词回复。
func (s *Server) listKeywordGroups(w http.ResponseWriter, r *http.Request) {
	// cookieID、userID 是路由账号和当前认证用户。
	cookieID := chi.URLParam(r, "cid")
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// groups、listErr 是应用层规则组和读取错误。
	groups, listErr := s.keywordApplication().ListGroups(r.Context(), userID, cookieID)
	if listErr != nil {
		writeKeywordError(w, listErr, "查询规则组失败")
		return
	}
	// response 是逐字段映射后的规则组 DTO。
	response := make([]keywordGroupResponse, 0, len(groups))
	// group 是当前映射的多关键词规则组。
	for _, group := range groups {
		response = append(response, keywordGroupResponse{GroupID: group.GroupID, Keywords: group.Keywords, Reply: group.Reply, ItemID: group.ItemID, Type: group.Type, ImageURL: group.ImageURL, MatchType: group.MatchType, MessageScope: group.MessageScope, MessageScopes: group.MessageScopes, SystemTypes: group.SystemTypes, Enabled: group.Enabled})
	}
	writeJSON(w, http.StatusOK, response)
}

// saveKeywordGroup 创建或替换一个多关键词共享回复组。
func (s *Server) saveKeywordGroup(w http.ResponseWriter, r *http.Request) {
	// cookieID、groupID、userID 是账号、可选规则组和当前认证用户。
	cookieID, groupID := chi.URLParam(r, "cid"), chi.URLParam(r, "group_id")
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是多关键词规则组请求。
	var request keywordGroupRequest
	if // decodeErr 是规则组 JSON 请求解析错误。
	decodeErr := decodeJSON(r, &request); decodeErr != nil {
		writeErr(w, http.StatusBadRequest, "关键词规则组格式错误")
		return
	}
	// savedID、saveErr 是应用层返回的稳定组标识和保存错误。
	savedID, saveErr := s.keywordApplication().SaveGroup(r.Context(), userID, cookieID, keywords.GroupDraft{GroupID: groupID, Keywords: request.Keywords, Reply: request.Reply, ItemID: request.ItemID, Type: request.Type, ImageURL: request.ImageURL, MatchType: request.MatchType, MessageScope: request.MessageScope, MessageScopes: request.MessageScopes, SystemTypes: request.SystemTypes, Enabled: request.Enabled})
	if saveErr != nil {
		writeKeywordError(w, saveErr, "保存规则组失败")
		return
	}
	writeJSON(w, http.StatusOK, keywordGroupMutationResponse{Success: true, GroupID: savedID})
}

// deleteKeywordGroup 删除一个完整多关键词共享回复组。
func (s *Server) deleteKeywordGroup(w http.ResponseWriter, r *http.Request) {
	// cookieID、groupID、userID 是账号、规则组和当前认证用户。
	cookieID, groupID := chi.URLParam(r, "cid"), chi.URLParam(r, "group_id")
	// userID、ok 是当前认证用户标识和读取成功标记。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	if // deleteErr 是规则组删除应用服务错误。
	deleteErr := s.keywordApplication().DeleteGroup(r.Context(), userID, cookieID, groupID); deleteErr != nil {
		writeKeywordError(w, deleteErr, "删除规则组失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// itemReplyRequest 是指定商品回复写入请求 DTO。
type itemReplyRequest struct {
	// ReplyContent 是商品命中后的回复正文。
	ReplyContent string `json:"reply_content"`
}

// mountKeywordsReal 注册关键词回复兼容路由。
func (s *Server) mountKeywordsReal(r chi.Router) {
	r.Get("/keywords/{cid}", s.listKeywords)
	r.Post("/keywords/{cid}", s.addKeyword)
	r.Get("/keywords-with-item-id/{cid}", s.listKeywordsWithItemID)
	r.Post("/keywords-with-item-id/{cid}", s.addKeywordWithItemID)
	r.Get("/keywords-with-type/{cid}", s.listKeywordsWithType)
	r.Put("/keywords-with-type/{cid}/{id}", s.updateKeywordByID)
	r.Delete("/keywords-with-type/{cid}/{id}", s.deleteKeywordByID)
	r.Delete("/keywords/{cid}/{index}", s.deleteKeyword)
}

// keywordApplication 返回关键词回复应用服务；具体依赖由 Server 统一装配。
func (s *Server) keywordApplication() KeywordsPort {
	return s.applicationServiceSet().keywords
}

// keywordUserID 从认证上下文读取当前用户，不读取任何账号凭证。
func keywordUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	// session 是认证中间件注入的当前用户会话。
	session := auth.SessionFromContext(r.Context())
	if session == nil {
		writeErr(w, http.StatusUnauthorized, "未授权访问")
		return 0, false
	}
	return session.UserID, true
}

// writeKeywordError 将应用层错误映射为现有关键词接口的 HTTP 语义。
func writeKeywordError(w http.ResponseWriter, err error, fallback string) {
	if err == nil {
		return
	}
	// validationErr 是可安全展示给调用方的参数校验提示。
	var validationErr *keywords.ValidationError
	if errors.As(err, &validationErr) {
		writeErr(w, http.StatusBadRequest, validationErr.Error())
		return
	}
	switch {
	case errors.Is(err, keywords.ErrForbidden):
		writeErr(w, http.StatusForbidden, "无权限操作该账号")
	case errors.Is(err, keywords.ErrNotFound):
		writeErr(w, http.StatusNotFound, "关键字不存在")
	case errors.Is(err, keywords.ErrInvalidInput):
		writeErr(w, http.StatusBadRequest, "请求参数无效")
	default:
		writeErr(w, http.StatusInternalServerError, fallback)
	}
}

// listKeywords 返回兼容的基础关键词响应。
func (s *Server) listKeywords(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// rows 保存应用层查询结果。
	rows, err := s.keywordApplication().List(r.Context(), userID, cookieID)
	if err != nil {
		writeKeywordError(w, err, "查询失败")
		return
	}
	// result 保存 HTTP 基础响应，隐藏数据库模型和内部字段。
	result := make([]keywordBasicResponse, 0, len(rows))
	// row 是当前待映射的关键词规则。
	for _, row := range rows {
		result = append(result, keywordBasicResponse{Keyword: row.Keyword, Reply: row.Reply})
	}
	writeJSON(w, http.StatusOK, result)
}

// listKeywordsWithItemID 返回带商品范围的兼容关键词响应。
func (s *Server) listKeywordsWithItemID(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// rows 保存应用层查询结果。
	rows, err := s.keywordApplication().List(r.Context(), userID, cookieID)
	if err != nil {
		writeKeywordError(w, err, "查询失败")
		return
	}
	// result 保存带商品字段的 HTTP 响应。
	result := make([]keywordItemResponse, 0, len(rows))
	// row 是当前待映射的关键词规则。
	for _, row := range rows {
		result = append(result, keywordItemResponse{Keyword: row.Keyword, Reply: row.Reply, ItemID: row.ItemID})
	}
	writeJSON(w, http.StatusOK, result)
}

// listKeywordsWithType 返回支持 text/image 类型的兼容响应。
func (s *Server) listKeywordsWithType(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// rows 保存应用层查询结果。
	rows, err := s.keywordApplication().List(r.Context(), userID, cookieID)
	if err != nil {
		writeKeywordError(w, err, "查询失败")
		return
	}
	// result 保存带类型字段的 HTTP 响应。
	result := make([]keywordTypedResponse, 0, len(rows))
	// row 是当前待映射的关键词规则。
	for _, row := range rows {
		result = append(result, keywordTypedResponse{ID: row.ID, Keyword: row.Keyword, Reply: row.Reply, ItemID: row.ItemID, Type: row.Type, ImageURL: row.ImageURL})
	}
	writeJSON(w, http.StatusOK, result)
}

// addKeyword 创建一条普通文字关键词规则。
func (s *Server) addKeyword(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是普通关键词请求 DTO。
	var request keywordRequest
	// err 表示请求 JSON 解码失败。
	if err := decodeJSON(r, &request); err != nil {
		writeErr(w, http.StatusBadRequest, "keyword 必填")
		return
	}
	// _, err 表示创建操作的结果；ID 对兼容响应不向客户端暴露。
	if _, err := s.keywordApplication().Add(r.Context(), userID, cookieID, keywords.Draft{Keyword: request.Keyword, Reply: request.Reply, Type: "text"}); err != nil {
		writeKeywordError(w, err, "添加失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// addKeywordWithItemID 创建或批量替换带商品范围的关键词规则。
func (s *Server) addKeywordWithItemID(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是单项或批量关键词请求 DTO。
	var request keywordBatchRequest
	// err 表示请求 JSON 解码失败。
	if err := decodeJSON(r, &request); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if request.Keywords != nil {
		// drafts 保存经过应用服务校验的批量关键词输入。
		drafts := make([]keywords.Draft, 0, len(*request.Keywords))
		// item 是当前待转换的批量请求项。
		for _, item := range *request.Keywords {
			drafts = append(drafts, keywords.Draft{Keyword: item.Keyword, Reply: item.Reply, ItemID: item.ItemID, Type: item.Type, ImageURL: item.ImageURL})
		}
		// err 表示批量替换规则的应用服务错误。
		if err := s.keywordApplication().Replace(r.Context(), userID, cookieID, drafts); err != nil {
			writeKeywordError(w, err, "保存失败")
			return
		}
		writeJSON(w, http.StatusOK, operationResponse{Success: true})
		return
	}
	// id 是新建规则的持久化标识；兼容响应保留该字段。
	id, err := s.keywordApplication().Add(r.Context(), userID, cookieID, keywords.Draft{Keyword: request.Keyword, Reply: request.Reply, ItemID: request.ItemID, Type: request.Type, ImageURL: request.ImageURL})
	if err != nil {
		writeKeywordError(w, err, "添加失败")
		return
	}
	writeJSON(w, http.StatusOK, mutationIDResponse{Success: true, ID: id})
}

// updateKeywordByID 按关键词 ID 更新 text/image 规则。
func (s *Server) updateKeywordByID(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// id 是路由中的关键词持久化标识。
	id, parseErr := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if parseErr != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效关键词ID")
		return
	}
	// request 是关键词更新请求 DTO。
	var request keywordUpdateRequest
	// err 表示请求 JSON 解码失败。
	if err := decodeJSON(r, &request); err != nil {
		writeErr(w, http.StatusBadRequest, "keyword 必填")
		return
	}
	// updateErr 表示应用层更新结果。
	updateErr := s.keywordApplication().Update(r.Context(), userID, cookieID, id, keywords.Draft{Keyword: request.Keyword, Reply: request.Reply, ItemID: request.ItemID, Type: request.Type, ImageURL: request.ImageURL})
	if updateErr != nil {
		writeKeywordError(w, updateErr, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// deleteKeywordByID 按持久化 ID 删除关键词规则。
func (s *Server) deleteKeywordByID(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// id 是路由中的关键词持久化标识。
	id, parseErr := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if parseErr != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "无效关键词ID")
		return
	}
	// deleteErr 表示应用层删除结果。
	deleteErr := s.keywordApplication().DeleteByID(r.Context(), userID, cookieID, id)
	if deleteErr != nil {
		writeKeywordError(w, deleteErr, "关键字不存在")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// deleteKeyword 按兼容的零基索引删除关键词规则。
func (s *Server) deleteKeyword(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cid")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// index 是按关键词 ID 顺序解释的零基索引。
	index, parseErr := strconv.Atoi(chi.URLParam(r, "index"))
	if parseErr != nil {
		index = -1
	}
	// deleteErr 表示应用层删除结果。
	deleteErr := s.keywordApplication().DeleteByIndex(r.Context(), userID, cookieID, index)
	if deleteErr != nil {
		writeKeywordError(w, deleteErr, "关键字不存在")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// mountItemRepliesReal 注册指定商品回复兼容路由。
func (s *Server) mountItemRepliesReal(r chi.Router) {
	r.Get("/itemReplays", s.listItemReplies)
	r.Get("/item-reply/{cookie_id}/{item_id}", s.getItemReply)
	r.Put("/item-reply/{cookie_id}/{item_id}", s.setItemReply)
	r.Delete("/item-reply/{cookie_id}/{item_id}", s.deleteItemReply)
}

// listItemReplies 返回当前用户全部账号的指定商品回复。
func (s *Server) listItemReplies(w http.ResponseWriter, r *http.Request) {
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// rows 保存应用层商品回复结果。
	rows, err := s.keywordApplication().ListItemReplies(r.Context(), userID)
	if err != nil {
		// 兼容历史接口：列表查询失败仍返回空列表和 200。
		writeJSON(w, http.StatusOK, []itemReplyResponse{})
		return
	}
	// result 保存 HTTP 商品回复响应。
	result := make([]itemReplyResponse, 0, len(rows))
	// row 是当前待映射的商品回复。
	for _, row := range rows {
		result = append(result, itemReplyResponse{ItemID: row.ItemID, CookieID: row.CookieID, ReplyContent: row.ReplyContent})
	}
	writeJSON(w, http.StatusOK, result)
}

// getItemReply 返回指定商品回复；缺失时保持历史空正文响应。
func (s *Server) getItemReply(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cookie_id")
	// itemID 是路由中的商品标识。
	itemID := chi.URLParam(r, "item_id")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// row 保存应用层商品回复结果。
	row, err := s.keywordApplication().GetItemReply(r.Context(), userID, cookieID, itemID)
	if errors.Is(err, keywords.ErrNotFound) {
		writeJSON(w, http.StatusOK, itemReplyResponse{ReplyContent: ""})
		return
	}
	if err != nil {
		writeKeywordError(w, err, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, itemReplyResponse{ItemID: row.ItemID, CookieID: row.CookieID, ReplyContent: row.ReplyContent})
}

// setItemReply 覆盖指定商品回复。
func (s *Server) setItemReply(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cookie_id")
	// itemID 是路由中的商品标识。
	itemID := chi.URLParam(r, "item_id")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// request 是指定商品回复请求 DTO。
	var request itemReplyRequest
	// err 表示请求 JSON 解码失败。
	if err := decodeJSON(r, &request); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	// err 表示应用层写入结果。
	if err := s.keywordApplication().SetItemReply(r.Context(), userID, cookieID, itemID, request.ReplyContent); err != nil {
		writeKeywordError(w, err, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}

// deleteItemReply 删除指定商品回复。
func (s *Server) deleteItemReply(w http.ResponseWriter, r *http.Request) {
	// cookieID 是路由中的账号标识。
	cookieID := chi.URLParam(r, "cookie_id")
	// itemID 是路由中的商品标识。
	itemID := chi.URLParam(r, "item_id")
	// userID 是当前认证用户标识。
	userID, ok := keywordUserID(w, r)
	if !ok {
		return
	}
	// err 表示应用层删除结果。
	if err := s.keywordApplication().DeleteItemReply(r.Context(), userID, cookieID, itemID); err != nil {
		writeKeywordError(w, err, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, operationResponse{Success: true})
}
