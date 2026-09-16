package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestKnowledgeHTTPWorkflowKeepsReviewAndOfflineRetrievalGates 验证版本化知识库 API 的审核、启停、引用与高风险门禁。
func TestKnowledgeHTTPWorkflowKeepsReviewAndOfflineRetrievalGates(t *testing.T) {
	// server 是使用已迁移临时 SQLite 的完整 HTTP 测试服务。
	server, _, cleanup := newTestServer(t)
	defer cleanup()
	// publicHandler 是正式默认路由树，用于登录并验证知识库接口已停止注册。
	publicHandler := server.Router()
	// sessionCookie 是测试管理员登录后的 ERP 会话 Cookie。
	sessionCookie := loginHelper(t, publicHandler)
	// dormantRequest 是下线期间对旧知识库接口的直接访问。
	dormantRequest := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases", nil)
	dormantRequest.AddCookie(sessionCookie)
	// dormantResponse 记录正式路由树对旧接口的不可用响应。
	dormantResponse := httptest.NewRecorder()
	publicHandler.ServeHTTP(dormantResponse, dormantRequest)
	if dormantResponse.Code != http.StatusNotFound {
		t.Fatalf("dormant knowledge status=%d body=%s", dormantResponse.Code, dormantResponse.Body.String())
	}
	// handler 是仅在隔离测试中显式挂载的知识库路由，证明保留实现和数据库能力仍可恢复。
	handler := chi.NewRouter()
	server.mountVersionedKnowledgeRoutes(handler)
	// baseResponse 是创建草稿知识库的数值主键响应。
	var baseResponse mutationIDResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPost, "/api/v1/knowledge-bases", `{"name":"使用教程","description":"已确认的商品使用说明"}`, http.StatusCreated, &baseResponse)
	if baseResponse.ID <= 0 {
		t.Fatalf("base response=%+v", baseResponse)
	}
	// basePath 是当前测试知识库的版本化 API 路径。
	basePath := fmt.Sprintf("/api/v1/knowledge-bases/%d", baseResponse.ID)
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPut, basePath+"/status", `{"status":"active"}`, http.StatusOK, &operationResponse{})
	// entryResponse 是创建待审核 FAQ 的数值主键响应。
	var entryResponse mutationIDResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPost, basePath+"/entries", `{"type":"faq","title":"这个商品怎么使用？","content":"请按商品说明完成设置。","risk_level":"low"}`, http.StatusCreated, &entryResponse)
	// pendingEnableResponse 是待审核 FAQ 尝试启用时的统一错误响应。
	pendingEnableResponse := httptest.NewRecorder()
	// pendingEnableRequest 是待审核 FAQ 违反启用门禁的请求。
	pendingEnableRequest := httptest.NewRequest(http.MethodPut, fmt.Sprintf("%s/entries/faq/%d/enabled", basePath, entryResponse.ID), strings.NewReader(`{"enabled":true}`))
	pendingEnableRequest.Header.Set("Content-Type", "application/json")
	pendingEnableRequest.AddCookie(sessionCookie)
	handler.ServeHTTP(pendingEnableResponse, pendingEnableRequest)
	if pendingEnableResponse.Code != http.StatusBadRequest || !strings.Contains(pendingEnableResponse.Body.String(), "只有已审核") {
		t.Fatalf("pending enable status=%d body=%s", pendingEnableResponse.Code, pendingEnableResponse.Body.String())
	}
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPut, fmt.Sprintf("%s/entries/faq/%d/review", basePath, entryResponse.ID), `{"reviewed":true}`, http.StatusOK, &operationResponse{})
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPut, fmt.Sprintf("%s/entries/faq/%d/enabled", basePath, entryResponse.ID), `{"enabled":true}`, http.StatusOK, &operationResponse{})
	// entriesResponse 是审核并启用后的 FAQ／文档列表响应。
	var entriesResponse knowledgeEntryListResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodGet, basePath+"/entries", "", http.StatusOK, &entriesResponse)
	if len(entriesResponse.Data) != 1 || entriesResponse.Data[0].ReviewStatus != "reviewed" || !entriesResponse.Data[0].Enabled || entriesResponse.Data[0].AllowAutoReply {
		t.Fatalf("entries=%+v", entriesResponse.Data)
	}
	// aliasResponse 是用户确认“怎么配置”相似问法后的主键响应。
	var aliasResponse mutationIDResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPost, fmt.Sprintf("%s/entries/faq/%d/aliases", basePath, entryResponse.ID), `{"alias":"怎么配置","source":"debug"}`, http.StatusCreated, &aliasResponse)
	// aliasesResponse 是创建后重新读取的 FAQ 相似问法列表。
	var aliasesResponse knowledgeFAQAliasListResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodGet, fmt.Sprintf("%s/entries/faq/%d/aliases", basePath, entryResponse.ID), "", http.StatusOK, &aliasesResponse)
	if aliasResponse.ID <= 0 || len(aliasesResponse.Data) != 1 || aliasesResponse.Data[0].Alias != "怎么配置" || aliasesResponse.Data[0].Source != "debug" {
		t.Fatalf("alias id=%d list=%+v", aliasResponse.ID, aliasesResponse.Data)
	}
	// suggestionsResponse 是不自动落库的 FAQ 确定性建议。
	var suggestionsResponse knowledgeFAQAliasSuggestionResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodGet, fmt.Sprintf("%s/entries/faq/%d/alias-suggestions", basePath, entryResponse.ID), "", http.StatusOK, &suggestionsResponse)
	// retrieveResponse 是低风险使用教程的离线检索响应。
	var retrieveResponse knowledgeRetrieveResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPost, "/api/v1/knowledge-retrieve", fmt.Sprintf(`{"query":"怎样配置","knowledge_base_ids":[%d]}`, baseResponse.ID), http.StatusOK, &retrieveResponse)
	if retrieveResponse.Answerability != "answerable" || retrieveResponse.Candidate == "" || len(retrieveResponse.Evidence) != 1 {
		t.Fatalf("retrieve=%+v", retrieveResponse)
	}
	// highRiskResponse 是退款问题被确定性转人工门禁拦截的响应。
	var highRiskResponse knowledgeRetrieveResponse
	knowledgeJSONRequest(t, handler, sessionCookie, http.MethodPost, "/api/v1/knowledge-retrieve", fmt.Sprintf(`{"query":"我要退款","knowledge_base_ids":[%d]}`, baseResponse.ID), http.StatusOK, &highRiskResponse)
	if highRiskResponse.Answerability != "needs_human" {
		t.Fatalf("high risk=%+v", highRiskResponse)
	}
}

// knowledgeJSONRequest 发送带认证 Cookie 的知识库 JSON 请求并解析成指定响应 DTO。
func knowledgeJSONRequest(t *testing.T, handler http.Handler, sessionCookie *http.Cookie, method, path, body string, expectedStatus int, target any) {
	t.Helper()
	// request 是当前待发送的版本化知识库 HTTP 请求。
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(sessionCookie)
	// response 记录知识库路由返回的 HTTP 状态和 JSON 正文。
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expectedStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.Code, expectedStatus, response.Body.String())
	}
	if target == nil || response.Body.Len() == 0 {
		return
	}
	// decodeErr 是将成功 JSON 响应解析到指定具名 DTO 的失败原因。
	if decodeErr := json.NewDecoder(response.Body).Decode(target); decodeErr != nil {
		t.Fatalf("decode %s %s: %v body=%s", method, path, decodeErr, response.Body.String())
	}
}
