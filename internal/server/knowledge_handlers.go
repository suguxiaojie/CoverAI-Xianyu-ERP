package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	knowledgeapp "xianyu-go/internal/application/knowledge"
	"xianyu-go/internal/auth"
)

// knowledgeApplication 返回组合期注入的知识库应用 Port。
func (server *Server) knowledgeApplication() KnowledgePort {
	return server.applicationServiceSet().knowledge
}

// listKnowledgeBases 返回当前用户的全部知识库与审核计数。
func (server *Server) listKnowledgeBases(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已通过 RequireAuth 的 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// bases 是应用层返回的当前用户知识库。
	bases, listErr := server.knowledgeApplication().ListBases(request.Context(), session.UserID)
	if listErr != nil {
		server.writeKnowledgeError(writer, request, listErr, "读取知识库失败")
		return
	}
	// response 是不包含数据库对象的知识库列表 DTO。
	response := make([]knowledgeBaseResponse, 0, len(bases))
	// base 是当前待映射为 HTTP DTO 的知识库应用模型。
	for _, base := range bases {
		response = append(response, knowledgeBaseResponseFromApplication(base))
	}
	writeJSON(writer, http.StatusOK, knowledgeBaseListResponse{Data: response})
}

// getKnowledgeBase 返回当前用户拥有的单个知识库。
func (server *Server) getKnowledgeBase(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, parseErr := knowledgeResourceID(request, "knowledge_base_id")
	if parseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, parseErr.Error())
		return
	}
	// base 是应用层返回的单个知识库。
	base, getErr := server.knowledgeApplication().GetBase(request.Context(), session.UserID, knowledgeBaseID)
	if getErr != nil {
		server.writeKnowledgeError(writer, request, getErr, "读取知识库失败")
		return
	}
	writeJSON(writer, http.StatusOK, knowledgeBaseResponseFromApplication(base))
}

// createKnowledgeBase 解析具名请求并创建默认草稿知识库。
func (server *Server) createKnowledgeBase(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// payload 是当前创建知识库的具名 HTTP DTO。
	var payload knowledgeBaseMutationRequest
	// decodeErr 是创建知识库 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库请求格式无效")
		return
	}
	// knowledgeBaseID 是应用层创建的草稿知识库主键。
	knowledgeBaseID, createErr := server.knowledgeApplication().CreateBase(request.Context(), session.UserID, knowledgeBaseDraftFromRequest(payload))
	if createErr != nil {
		server.writeKnowledgeError(writer, request, createErr, "创建知识库失败")
		return
	}
	writeJSON(writer, http.StatusCreated, mutationIDResponse{Success: true, ID: knowledgeBaseID})
}

// updateKnowledgeBase 更新知识库文案并原子替换店铺／商品范围。
func (server *Server) updateKnowledgeBase(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, parseErr := knowledgeResourceID(request, "knowledge_base_id")
	if parseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, parseErr.Error())
		return
	}
	// payload 是更新知识库的具名 HTTP DTO。
	var payload knowledgeBaseMutationRequest
	// decodeErr 是更新知识库 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库请求格式无效")
		return
	}
	// updateErr 是更新知识库文案和范围的应用结果。
	if updateErr := server.knowledgeApplication().UpdateBase(request.Context(), session.UserID, knowledgeBaseID, knowledgeBaseDraftFromRequest(payload)); updateErr != nil {
		server.writeKnowledgeError(writer, request, updateErr, "更新知识库失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识库已更新"})
}

// setKnowledgeBaseStatus 切换知识库草稿／启用状态。
func (server *Server) setKnowledgeBaseStatus(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, parseErr := knowledgeResourceID(request, "knowledge_base_id")
	if parseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, parseErr.Error())
		return
	}
	// payload 是草稿／启用状态请求 DTO。
	var payload knowledgeBaseStatusRequest
	// decodeErr 是知识库状态 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库状态请求格式无效")
		return
	}
	// updateErr 是切换知识库草稿／启用状态的应用结果。
	if updateErr := server.knowledgeApplication().SetBaseStatus(request.Context(), session.UserID, knowledgeBaseID, knowledgeapp.BaseStatus(payload.Status)); updateErr != nil {
		server.writeKnowledgeError(writer, request, updateErr, "切换知识库状态失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识库状态已更新"})
}

// deleteKnowledgeBase 删除当前用户拥有的知识库。
func (server *Server) deleteKnowledgeBase(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, parseErr := knowledgeResourceID(request, "knowledge_base_id")
	if parseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, parseErr.Error())
		return
	}
	// deleteErr 是删除当前用户知识库的应用结果。
	if deleteErr := server.knowledgeApplication().DeleteBase(request.Context(), session.UserID, knowledgeBaseID); deleteErr != nil {
		server.writeKnowledgeError(writer, request, deleteErr, "删除知识库失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识库已删除"})
}

// listKnowledgeEntries 返回单个知识库下的 FAQ 和文档。
func (server *Server) listKnowledgeEntries(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, parseErr := knowledgeResourceID(request, "knowledge_base_id")
	if parseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, parseErr.Error())
		return
	}
	// entries 是应用层返回的 FAQ 和文档。
	entries, listErr := server.knowledgeApplication().ListEntries(request.Context(), session.UserID, knowledgeBaseID)
	if listErr != nil {
		server.writeKnowledgeError(writer, request, listErr, "读取知识条目失败")
		return
	}
	// response 是逐字段映射的 FAQ／文档 HTTP DTO。
	response := make([]knowledgeEntryResponse, 0, len(entries))
	// entry 是当前待转换的 FAQ 或文档应用模型。
	for _, entry := range entries {
		response = append(response, knowledgeEntryResponseFromApplication(entry))
	}
	writeJSON(writer, http.StatusOK, knowledgeEntryListResponse{Data: response})
}

// createKnowledgeEntry 在单个知识库中创建默认待审核 FAQ 或文档。
func (server *Server) createKnowledgeEntry(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, parseErr := knowledgeResourceID(request, "knowledge_base_id")
	if parseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, parseErr.Error())
		return
	}
	// payload 是新建 FAQ／文档的具名 HTTP DTO。
	var payload knowledgeEntryMutationRequest
	// decodeErr 是新建 FAQ／文档 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识条目请求格式无效")
		return
	}
	// entryID 是应用层创建的待审核 FAQ 或文档主键。
	entryID, createErr := server.knowledgeApplication().CreateEntry(request.Context(), session.UserID, knowledgeBaseID, knowledgeEntryDraftFromRequest(payload))
	if createErr != nil {
		server.writeKnowledgeError(writer, request, createErr, "创建知识条目失败")
		return
	}
	writeJSON(writer, http.StatusCreated, mutationIDResponse{Success: true, ID: entryID})
}

// updateKnowledgeEntry 更新 FAQ／文档内容并强制重置待审核状态。
func (server *Server) updateKnowledgeEntry(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// entryID 是经数值校验的路径条目标识。
	entryID, entryParseErr := knowledgeResourceID(request, "entry_id")
	if baseParseErr != nil || entryParseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库或条目 ID 无效")
		return
	}
	// payload 是更新 FAQ／文档的具名 HTTP DTO。
	var payload knowledgeEntryMutationRequest
	// decodeErr 是更新 FAQ／文档 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识条目请求格式无效")
		return
	}
	// updateErr 是更新条目并重置待审核状态的应用结果。
	if updateErr := server.knowledgeApplication().UpdateEntry(request.Context(), session.UserID, knowledgeBaseID, entryID, knowledgeEntryDraftFromRequest(payload)); updateErr != nil {
		server.writeKnowledgeError(writer, request, updateErr, "更新知识条目失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识条目已更新并重置为待审核"})
}

// reviewKnowledgeEntry 保存单个 FAQ 或文档的人工审核结果。
func (server *Server) reviewKnowledgeEntry(writer http.ResponseWriter, request *http.Request) {
	server.mutateKnowledgeEntryFlag(writer, request, "review")
}

// setKnowledgeEntryEnabled 切换已审核 FAQ 或文档的离线检索状态。
func (server *Server) setKnowledgeEntryEnabled(writer http.ResponseWriter, request *http.Request) {
	server.mutateKnowledgeEntryFlag(writer, request, "enabled")
}

// mutateKnowledgeEntryFlag 共用审核和启停路径的 ID、类型和具名布尔请求解析。
func (server *Server) mutateKnowledgeEntryFlag(writer http.ResponseWriter, request *http.Request, action string) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// entryID 是经数值校验的路径条目标识。
	entryID, entryParseErr := knowledgeResourceID(request, "entry_id")
	// contentType 是路径中的 faq 或 document 类型。
	contentType := knowledgeapp.ContentType(chi.URLParam(request, "entry_type"))
	if baseParseErr != nil || entryParseErr != nil || contentType != knowledgeapp.ContentTypeFAQ && contentType != knowledgeapp.ContentTypeDocument {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库、条目 ID 或类型无效")
		return
	}
	if action == "review" {
		// payload 是人工审核布尔请求 DTO。
		var payload knowledgeReviewRequest
		// decodeErr 是人工审核 JSON 正文解析失败原因。
		if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
			writeErrRequest(writer, request, http.StatusBadRequest, "审核请求格式无效")
			return
		}
		// reviewErr 是保存人工审核结果的应用结果。
		if reviewErr := server.knowledgeApplication().ReviewEntry(request.Context(), session.UserID, knowledgeBaseID, entryID, contentType, payload.Reviewed); reviewErr != nil {
			server.writeKnowledgeError(writer, request, reviewErr, "更新知识审核状态失败")
			return
		}
		writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识审核状态已更新"})
		return
	}
	// payload 是离线检索启停布尔请求 DTO。
	var payload knowledgeEnabledRequest
	// decodeErr 是条目离线检索启停 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识启停请求格式无效")
		return
	}
	// enabledErr 是切换已审核条目离线检索状态的应用结果。
	if enabledErr := server.knowledgeApplication().SetEntryEnabled(request.Context(), session.UserID, knowledgeBaseID, entryID, contentType, payload.Enabled); enabledErr != nil {
		server.writeKnowledgeError(writer, request, enabledErr, "更新知识启停状态失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识启停状态已更新"})
}

// deleteKnowledgeEntry 删除当前用户知识库下的单个 FAQ 或文档。
func (server *Server) deleteKnowledgeEntry(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// entryID 是经数值校验的路径条目标识。
	entryID, entryParseErr := knowledgeResourceID(request, "entry_id")
	// contentType 是路径中的 faq 或 document 类型。
	contentType := knowledgeapp.ContentType(chi.URLParam(request, "entry_type"))
	if baseParseErr != nil || entryParseErr != nil || contentType != knowledgeapp.ContentTypeFAQ && contentType != knowledgeapp.ContentTypeDocument {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库、条目 ID 或类型无效")
		return
	}
	// deleteErr 是删除 FAQ／文档及其分块的应用结果。
	if deleteErr := server.knowledgeApplication().DeleteEntry(request.Context(), session.UserID, knowledgeBaseID, entryID, contentType); deleteErr != nil {
		server.writeKnowledgeError(writer, request, deleteErr, "删除知识条目失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "知识条目已删除"})
}

// listKnowledgeFAQAliases 返回当前用户指定 FAQ 的相似问法。
func (server *Server) listKnowledgeFAQAliases(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// faqID 是经数值校验的 FAQ 标识。
	faqID, faqParseErr := knowledgeResourceID(request, "faq_id")
	if baseParseErr != nil || faqParseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库或 FAQ ID 无效")
		return
	}
	// aliases 是应用层返回的当前 FAQ 相似问法。
	aliases, listErr := server.knowledgeApplication().ListFAQAliases(request.Context(), session.UserID, knowledgeBaseID, faqID)
	if listErr != nil {
		server.writeKnowledgeError(writer, request, listErr, "读取 FAQ 相似问法失败")
		return
	}
	// data 是不暴露应用类型的相似问法响应列表。
	data := make([]knowledgeFAQAliasResponse, 0, len(aliases))
	// alias 是当前待转换的相似问法应用模型。
	for _, alias := range aliases {
		data = append(data, knowledgeFAQAliasResponse{ID: alias.ID, KnowledgeBaseID: alias.KnowledgeBaseID, FAQID: alias.FAQID, Alias: alias.Alias, Source: string(alias.Source), Enabled: alias.Enabled, CreatedAt: alias.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: alias.UpdatedAt.UTC().Format(time.RFC3339)})
	}
	writeJSON(writer, http.StatusOK, knowledgeFAQAliasListResponse{Data: data})
}

// createKnowledgeFAQAlias 创建用户在管理弹窗或调试器中明确确认的相似问法。
func (server *Server) createKnowledgeFAQAlias(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// faqID 是经数值校验的 FAQ 标识。
	faqID, faqParseErr := knowledgeResourceID(request, "faq_id")
	if baseParseErr != nil || faqParseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库或 FAQ ID 无效")
		return
	}
	// payload 是用户确认的相似问法和来源 DTO。
	var payload knowledgeFAQAliasMutationRequest
	// decodeErr 是相似问法请求 JSON 解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "FAQ 相似问法请求格式无效")
		return
	}
	// aliasID 是应用层完成冲突检查后创建的相似问法主键。
	aliasID, createErr := server.knowledgeApplication().CreateFAQAlias(request.Context(), session.UserID, knowledgeBaseID, faqID, knowledgeapp.FAQAliasDraft{Alias: payload.Alias, Source: knowledgeapp.AliasSource(payload.Source)})
	if createErr != nil {
		server.writeKnowledgeError(writer, request, createErr, "创建 FAQ 相似问法失败")
		return
	}
	writeJSON(writer, http.StatusCreated, mutationIDResponse{Success: true, ID: aliasID})
}

// deleteKnowledgeFAQAlias 删除当前用户指定 FAQ 的单条相似问法。
func (server *Server) deleteKnowledgeFAQAlias(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 和 baseParseErr 是经数值校验的知识库标识及解析错误。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// faqID 和 faqParseErr 是经数值校验的 FAQ 标识及解析错误。
	faqID, faqParseErr := knowledgeResourceID(request, "faq_id")
	// aliasID 和 aliasParseErr 是经数值校验的相似问法标识及解析错误。
	aliasID, aliasParseErr := knowledgeResourceID(request, "alias_id")
	if baseParseErr != nil || faqParseErr != nil || aliasParseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库、FAQ 或相似问法 ID 无效")
		return
	}
	// deleteErr 是删除当前用户相似问法的应用结果。
	if deleteErr := server.knowledgeApplication().DeleteFAQAlias(request.Context(), session.UserID, knowledgeBaseID, faqID, aliasID); deleteErr != nil {
		server.writeKnowledgeError(writer, request, deleteErr, "删除 FAQ 相似问法失败")
		return
	}
	writeJSON(writer, http.StatusOK, operationResponse{Success: true, Message: "FAQ 相似问法已删除"})
}

// suggestKnowledgeFAQAliases 返回不落库的确定性相似问法建议。
func (server *Server) suggestKnowledgeFAQAliases(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// knowledgeBaseID 是经数值校验的路径知识库标识。
	knowledgeBaseID, baseParseErr := knowledgeResourceID(request, "knowledge_base_id")
	// faqID 是经数值校验的 FAQ 标识。
	faqID, faqParseErr := knowledgeResourceID(request, "faq_id")
	if baseParseErr != nil || faqParseErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识库或 FAQ ID 无效")
		return
	}
	// suggestions 是应用层基于标准问题生成且尚未落库的建议。
	suggestions, suggestErr := server.knowledgeApplication().SuggestFAQAliases(request.Context(), session.UserID, knowledgeBaseID, faqID)
	if suggestErr != nil {
		server.writeKnowledgeError(writer, request, suggestErr, "生成 FAQ 相似问法建议失败")
		return
	}
	writeJSON(writer, http.StatusOK, knowledgeFAQAliasSuggestionResponse{Data: suggestions})
}

// retrieveKnowledge 执行不调用模型也不发送在线消息的离线检索调试。
func (server *Server) retrieveKnowledge(writer http.ResponseWriter, request *http.Request) {
	// session 是当前已认证 ERP 用户会话。
	session := auth.SessionFromContext(request.Context())
	// payload 是离线检索问题与知识库范围 DTO。
	var payload knowledgeRetrieveRequest
	// decodeErr 是离线检索 JSON 正文解析失败原因。
	if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
		writeErrRequest(writer, request, http.StatusBadRequest, "知识检索请求格式无效")
		return
	}
	// result 是应用层确定性文本检索与可回答性结果。
	result, retrieveErr := server.knowledgeApplication().Retrieve(request.Context(), knowledgeapp.RetrieveRequest{UserID: session.UserID, Query: payload.Query, KnowledgeBaseIDs: payload.KnowledgeBaseIDs})
	if retrieveErr != nil {
		server.writeKnowledgeError(writer, request, retrieveErr, "知识检索调试失败")
		return
	}
	writeJSON(writer, http.StatusOK, knowledgeRetrieveResponseFromApplication(result))
}

// knowledgeResourceID 将 chi 路径参数解析为正数知识资源 ID。
func knowledgeResourceID(request *http.Request, name string) (int64, error) {
	// resourceID 是从路径参数解析的数值主键。
	resourceID, parseErr := strconv.ParseInt(chi.URLParam(request, name), 10, 64)
	if parseErr != nil || resourceID <= 0 {
		return 0, &knowledgeapp.ValidationError{Message: "知识资源 ID 无效"}
	}
	return resourceID, nil
}

// knowledgeBaseDraftFromRequest 将 HTTP 范围 DTO 转换为不依赖 transport 的应用输入。
func knowledgeBaseDraftFromRequest(payload knowledgeBaseMutationRequest) knowledgeapp.BaseDraft {
	// itemScopes 是不依赖 HTTP JSON 标签的店铺商品应用范围。
	itemScopes := make([]knowledgeapp.ItemScope, 0, len(payload.ItemScopes))
	// scope 是当前待转换的店铺商品请求 DTO。
	for _, scope := range payload.ItemScopes {
		itemScopes = append(itemScopes, knowledgeapp.ItemScope{AccountID: scope.AccountID, ItemID: scope.ItemID})
	}
	return knowledgeapp.BaseDraft{Name: payload.Name, Description: payload.Description, AccountIDs: payload.AccountIDs, ItemScopes: itemScopes}
}

// knowledgeEntryDraftFromRequest 将 FAQ／文档 HTTP DTO 转换为应用输入。
func knowledgeEntryDraftFromRequest(payload knowledgeEntryMutationRequest) knowledgeapp.EntryDraft {
	return knowledgeapp.EntryDraft{Type: knowledgeapp.ContentType(payload.Type), Title: payload.Title, Content: payload.Content, ContentType: payload.ContentType, RiskLevel: knowledgeapp.RiskLevel(payload.RiskLevel), RequiresLiveData: payload.RequiresLiveData, EffectiveFrom: payload.EffectiveFrom, EffectiveTo: payload.EffectiveTo}
}

// knowledgeBaseResponseFromApplication 将知识库应用模型逐字段映射为 HTTP DTO。
func knowledgeBaseResponseFromApplication(base knowledgeapp.Base) knowledgeBaseResponse {
	// itemScopes 是知识库店铺商品范围 HTTP DTO。
	itemScopes := make([]knowledgeItemScopeResponse, 0, len(base.ItemScopes))
	// scope 是当前待转换的店铺商品应用范围。
	for _, scope := range base.ItemScopes {
		itemScopes = append(itemScopes, knowledgeItemScopeResponse{AccountID: scope.AccountID, ItemID: scope.ItemID})
	}
	return knowledgeBaseResponse{ID: base.ID, Name: base.Name, Description: base.Description, Status: string(base.Status), AccountIDs: base.AccountIDs, ItemScopes: itemScopes, EntryCount: base.EntryCount, ReviewedCount: base.ReviewedCount, CreatedAt: base.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: base.UpdatedAt.UTC().Format(time.RFC3339)}
}

// knowledgeEntryResponseFromApplication 将 FAQ／文档应用模型逐字段映射为 HTTP DTO。
func knowledgeEntryResponseFromApplication(entry knowledgeapp.Entry) knowledgeEntryResponse {
	return knowledgeEntryResponse{ID: entry.ID, KnowledgeBaseID: entry.KnowledgeBaseID, Type: string(entry.Type), Title: entry.Title, Content: entry.Content, ContentType: entry.ContentType, Status: string(entry.Status), ReviewStatus: string(entry.ReviewStatus), RiskLevel: string(entry.RiskLevel), RequiresLiveData: entry.RequiresLiveData, AllowAutoReply: entry.AllowAutoReply, Enabled: entry.Enabled, EffectiveFrom: entry.EffectiveFrom, EffectiveTo: entry.EffectiveTo, CreatedAt: entry.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: entry.UpdatedAt.UTC().Format(time.RFC3339)}
}

// knowledgeRetrieveResponseFromApplication 将离线检索结果和引用逐字段映射为 HTTP DTO。
func knowledgeRetrieveResponseFromApplication(result knowledgeapp.RetrieveResult) knowledgeRetrieveResponse {
	// evidence 是不包含应用层类型的引用证据 DTO。
	evidence := make([]knowledgeEvidenceResponse, 0, len(result.Evidence))
	// item 是当前待转换的应用层检索证据。
	for _, item := range result.Evidence {
		evidence = append(evidence, knowledgeEvidenceResponse{EntryID: item.EntryID, KnowledgeBaseID: item.KnowledgeBaseID, KnowledgeBaseName: item.KnowledgeBaseName, Type: string(item.Type), Title: item.Title, Excerpt: item.Excerpt, Score: item.Score})
	}
	return knowledgeRetrieveResponse{Query: result.Query, Answerability: string(result.Answerability), Explanation: result.Explanation, Candidate: result.Candidate, Evidence: evidence}
}

// writeKnowledgeError 将知识库应用错误映射为统一 HTTP 错误包。
func (server *Server) writeKnowledgeError(writer http.ResponseWriter, request *http.Request, err error, fallback string) {
	// validationErr 用于识别可安全展示的应用输入错误。
	var validationErr *knowledgeapp.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeErrRequest(writer, request, http.StatusBadRequest, validationErr.Error())
	case errors.Is(err, knowledgeapp.ErrNotFound):
		writeErrRequest(writer, request, http.StatusNotFound, "知识资源不存在")
	case errors.Is(err, knowledgeapp.ErrForbidden):
		writeErrRequest(writer, request, http.StatusForbidden, "无权操作该知识资源")
	default:
		writeErrRequest(writer, request, http.StatusInternalServerError, fallback)
	}
}
