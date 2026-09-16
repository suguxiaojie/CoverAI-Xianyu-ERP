package server

// knowledgeItemScopeRequest 是知识库绑定的店铺商品请求 DTO。
type knowledgeItemScopeRequest struct {
	// AccountID 是商品所属店铺账号标识。
	AccountID string `json:"account_id"`
	// ItemID 是店铺内商品标识。
	ItemID string `json:"item_id"`
}

// knowledgeBaseMutationRequest 是创建或更新知识库的具名请求 DTO。
type knowledgeBaseMutationRequest struct {
	// Name 是知识库用户可见名称。
	Name string `json:"name"`
	// Description 是权威内容和禁止推测范围说明。
	Description string `json:"description"`
	// AccountIDs 是显式选择的店铺范围。
	AccountIDs []string `json:"account_ids"`
	// ItemScopes 是显式选择的店铺商品范围。
	ItemScopes []knowledgeItemScopeRequest `json:"item_scopes"`
}

// knowledgeBaseStatusRequest 是切换知识库草稿／启用状态的请求 DTO。
type knowledgeBaseStatusRequest struct {
	// Status 是 draft 或 active。
	Status string `json:"status"`
}

// knowledgeEntryMutationRequest 是创建或更新 FAQ／文档的具名请求 DTO。
type knowledgeEntryMutationRequest struct {
	// Type 是 faq 或 document。
	Type string `json:"type"`
	// Title 是 FAQ 问题或文档标题。
	Title string `json:"title"`
	// Content 是 FAQ 标准答案或文档正文。
	Content string `json:"content"`
	// ContentType 是文档的 text 或 markdown 格式。
	ContentType string `json:"content_type"`
	// RiskLevel 是 low、medium 或 high。
	RiskLevel string `json:"risk_level"`
	// RequiresLiveData 表示候选回复前是否必须读取实时业务数据。
	RequiresLiveData bool `json:"requires_live_data"`
	// EffectiveFrom 是 Unix 秒生效下界；0 表示不限。
	EffectiveFrom int64 `json:"effective_from"`
	// EffectiveTo 是 Unix 秒失效上界；0 表示不限。
	EffectiveTo int64 `json:"effective_to"`
}

// knowledgeReviewRequest 是人工通过或驳回知识条目的请求 DTO。
type knowledgeReviewRequest struct {
	// Reviewed 表示是否将条目确认为已审核权威内容。
	Reviewed bool `json:"reviewed"`
}

// knowledgeEnabledRequest 是切换已审核条目离线检索状态的请求 DTO。
type knowledgeEnabledRequest struct {
	// Enabled 表示条目是否参与离线检索。
	Enabled bool `json:"enabled"`
}

// knowledgeFAQAliasMutationRequest 是用户确认新增 FAQ 相似问法的请求 DTO。
type knowledgeFAQAliasMutationRequest struct {
	// Alias 是不来自聊天日志的用户确认问法。
	Alias string `json:"alias"`
	// Source 是 manual、system 或 debug 用户动作来源。
	Source string `json:"source"`
}

// knowledgeFAQAliasResponse 是 FAQ 相似问法响应 DTO。
type knowledgeFAQAliasResponse struct {
	// ID 是相似问法数值主键。
	ID int64 `json:"id"`
	// KnowledgeBaseID 是所属知识库标识。
	KnowledgeBaseID int64 `json:"knowledge_base_id"`
	// FAQID 是关联 FAQ 标识。
	FAQID int64 `json:"faq_id"`
	// Alias 是用户确认的原始问法。
	Alias string `json:"alias"`
	// Source 是 manual、system 或 debug 来源。
	Source string `json:"source"`
	// Enabled 表示相似问法是否参与检索。
	Enabled bool `json:"enabled"`
	// CreatedAt 是 RFC3339 创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是 RFC3339 最近维护时间。
	UpdatedAt string `json:"updated_at"`
}

// knowledgeFAQAliasListResponse 是单个 FAQ 相似问法列表响应 DTO。
type knowledgeFAQAliasListResponse struct {
	// Data 是当前 FAQ 的全部用户确认相似问法。
	Data []knowledgeFAQAliasResponse `json:"data"`
}

// knowledgeFAQAliasSuggestionResponse 是不落库的确定性建议响应 DTO。
type knowledgeFAQAliasSuggestionResponse struct {
	// Data 是用户可选择采纳的相似问法建议。
	Data []string `json:"data"`
}

// knowledgeRetrieveRequest 是不调用外部模型的离线检索调试 DTO。
type knowledgeRetrieveRequest struct {
	// Query 是用户手工输入的脱敏调试问题。
	Query string `json:"query"`
	// KnowledgeBaseIDs 是显式参与调试的知识库列表。
	KnowledgeBaseIDs []int64 `json:"knowledge_base_ids"`
}

// knowledgeItemScopeResponse 是知识库店铺商品范围的响应 DTO。
type knowledgeItemScopeResponse struct {
	// AccountID 是商品所属店铺账号标识。
	AccountID string `json:"account_id"`
	// ItemID 是店铺内商品标识。
	ItemID string `json:"item_id"`
}

// knowledgeBaseResponse 是知识库范围、计数和时间的具名响应 DTO。
type knowledgeBaseResponse struct {
	// ID 是知识库数值主键。
	ID int64 `json:"id"`
	// Name 是知识库用户可见名称。
	Name string `json:"name"`
	// Description 是权威内容范围说明。
	Description string `json:"description"`
	// Status 是 draft 或 active。
	Status string `json:"status"`
	// AccountIDs 是显式绑定的店铺范围。
	AccountIDs []string `json:"account_ids"`
	// ItemScopes 是显式绑定的店铺商品范围。
	ItemScopes []knowledgeItemScopeResponse `json:"item_scopes"`
	// EntryCount 是 FAQ 和文档合计数量。
	EntryCount int `json:"entry_count"`
	// ReviewedCount 是已通过人工审核的条目数量。
	ReviewedCount int `json:"reviewed_count"`
	// CreatedAt 是 RFC3339 创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是 RFC3339 最近维护时间。
	UpdatedAt string `json:"updated_at"`
}

// knowledgeBaseListResponse 是当前用户知识库列表的具名响应 DTO。
type knowledgeBaseListResponse struct {
	// Data 是当前用户的全部知识库。
	Data []knowledgeBaseResponse `json:"data"`
}

// knowledgeEntryResponse 是 FAQ／文档审核与检索状态的具名响应 DTO。
type knowledgeEntryResponse struct {
	// ID 是 FAQ 或文档数值主键。
	ID int64 `json:"id"`
	// KnowledgeBaseID 是条目所属知识库标识。
	KnowledgeBaseID int64 `json:"knowledge_base_id"`
	// Type 是 faq 或 document。
	Type string `json:"type"`
	// Title 是 FAQ 问题或文档标题。
	Title string `json:"title"`
	// Content 是 FAQ 标准答案或文档正文。
	Content string `json:"content"`
	// ContentType 是 faq、text 或 markdown。
	ContentType string `json:"content_type"`
	// Status 是 draft、active 或 disabled。
	Status string `json:"status"`
	// ReviewStatus 是 pending 或 reviewed。
	ReviewStatus string `json:"review_status"`
	// RiskLevel 是 low、medium 或 high。
	RiskLevel string `json:"risk_level"`
	// RequiresLiveData 表示候选回复前是否必须读取实时业务数据。
	RequiresLiveData bool `json:"requires_live_data"`
	// AllowAutoReply 是预留自动标记；第一阶段固定为 false。
	AllowAutoReply bool `json:"allow_auto_reply"`
	// Enabled 表示条目是否参与离线检索。
	Enabled bool `json:"enabled"`
	// EffectiveFrom 是 Unix 秒生效下界。
	EffectiveFrom int64 `json:"effective_from"`
	// EffectiveTo 是 Unix 秒失效上界。
	EffectiveTo int64 `json:"effective_to"`
	// CreatedAt 是 RFC3339 创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是 RFC3339 最近维护时间。
	UpdatedAt string `json:"updated_at"`
}

// knowledgeEntryListResponse 是单个知识库 FAQ 和文档的具名响应 DTO。
type knowledgeEntryListResponse struct {
	// Data 是当前知识库的全部条目。
	Data []knowledgeEntryResponse `json:"data"`
}

// knowledgeEvidenceResponse 是离线检索结果中可追溯证据的具名 DTO。
type knowledgeEvidenceResponse struct {
	// EntryID 是证据源 FAQ 或文档标识。
	EntryID int64 `json:"entry_id"`
	// KnowledgeBaseID 是证据所属知识库标识。
	KnowledgeBaseID int64 `json:"knowledge_base_id"`
	// KnowledgeBaseName 是证据所属知识库名称。
	KnowledgeBaseName string `json:"knowledge_base_name"`
	// Type 表示证据来自 FAQ 还是文档。
	Type string `json:"type"`
	// Title 是证据源问题或文档标题。
	Title string `json:"title"`
	// Excerpt 是最小必要的脱敏证据片段。
	Excerpt string `json:"excerpt"`
	// Score 是 0 到 1 之间的确定性文本相关性分数。
	Score float64 `json:"score"`
}

// knowledgeRetrieveResponse 是离线检索的可回答性、候选文案和引用证据 DTO。
type knowledgeRetrieveResponse struct {
	// Query 是去除首尾空白后的调试问题。
	Query string `json:"query"`
	// Answerability 是 answerable、needs_live_data、needs_human 或 insufficient。
	Answerability string `json:"answerability"`
	// Explanation 说明允许或拒绝候选回复的原因。
	Explanation string `json:"explanation"`
	// Candidate 是直接来自已审核知识的离线候选文案。
	Candidate string `json:"candidate"`
	// Evidence 是按确定性相关性降序排列的引用证据。
	Evidence []knowledgeEvidenceResponse `json:"evidence"`
}
