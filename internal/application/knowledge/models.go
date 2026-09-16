// Package knowledge 定义人工维护知识库、审核门禁和确定性检索用例。
// 本包不依赖 HTTP、数据库或模型供应商，第一阶段也不发送在线聊天。
package knowledge

import "time"

// ContentType 区分结构化 FAQ 和纯文本／Markdown 文档。
type ContentType string

const (
	// ContentTypeFAQ 表示一条人工维护的问题与标准答案。
	ContentTypeFAQ ContentType = "faq"
	// ContentTypeDocument 表示一份纯文本或 Markdown 权威文档。
	ContentTypeDocument ContentType = "document"
)

// BaseStatus 表示知识库是草稿还是可参与调试检索。
type BaseStatus string

const (
	// BaseStatusDraft 表示知识库尚未完成人工确认。
	BaseStatusDraft BaseStatus = "draft"
	// BaseStatusActive 表示知识库可用于离线检索调试。
	BaseStatusActive BaseStatus = "active"
)

// ReviewStatus 表示知识条目的人工审核结果。
type ReviewStatus string

const (
	// ReviewStatusPending 表示条目不得进入检索证据。
	ReviewStatusPending ReviewStatus = "pending"
	// ReviewStatusReviewed 表示条目已由当前用户确认为权威内容。
	ReviewStatusReviewed ReviewStatus = "reviewed"
)

// EntryStatus 表示知识条目的内容生命周期。
type EntryStatus string

const (
	// EntryStatusDraft 表示条目仍在编辑或待审核。
	EntryStatusDraft EntryStatus = "draft"
	// EntryStatusActive 表示条目内容已完成审核。
	EntryStatusActive EntryStatus = "active"
	// EntryStatusDisabled 表示条目被人工停用且不得检索。
	EntryStatusDisabled EntryStatus = "disabled"
)

// RiskLevel 表示知识内容对候选回复的业务风险。
type RiskLevel string

const (
	// RiskLevelLow 表示普通使用说明等低风险知识。
	RiskLevelLow RiskLevel = "low"
	// RiskLevelMedium 表示涉及时间、价格或实时状态的中风险知识。
	RiskLevelMedium RiskLevel = "medium"
	// RiskLevelHigh 表示退款、投诉、补发或敏感信息等必须人工处理的知识。
	RiskLevelHigh RiskLevel = "high"
)

// ItemScope 将知识范围限制到当前用户的指定店铺商品。
type ItemScope struct {
	// AccountID 是商品所属的闲鱼账号标识。
	AccountID string
	// ItemID 是店铺内的商品标识。
	ItemID string
}

// Base 是不包含凭证或数据库对象的知识库应用模型。
type Base struct {
	// ID 是知识库的持久化标识。
	ID int64
	// UserID 是知识库所属 ERP 用户。
	UserID int64
	// Name 是知识库的用户可见名称。
	Name string
	// Description 说明知识库可回答和禁止推测的范围。
	Description string
	// Status 是草稿或已启用状态。
	Status BaseStatus
	// AccountIDs 是显式绑定的店铺账号；空列表表示用户级通用。
	AccountIDs []string
	// ItemScopes 是显式绑定的店铺商品；空列表表示不限商品。
	ItemScopes []ItemScope
	// EntryCount 是 FAQ 和文档的合计数量。
	EntryCount int
	// ReviewedCount 是已通过人工审核的条目数量。
	ReviewedCount int
	// CreatedAt 是知识库创建时间。
	CreatedAt time.Time
	// UpdatedAt 是最近一次维护时间。
	UpdatedAt time.Time
}

// BaseDraft 是创建或更新知识库的应用输入。
type BaseDraft struct {
	// Name 是去空格后的知识库名称。
	Name string
	// Description 是权威内容范围说明。
	Description string
	// AccountIDs 是用户选择的店铺范围。
	AccountIDs []string
	// ItemScopes 是用户选择的店铺商品范围。
	ItemScopes []ItemScope
}

// Entry 是 FAQ 和文档共用的审核与检索应用模型。
type Entry struct {
	// ID 是条目在所属类型表中的持久化标识。
	ID int64
	// KnowledgeBaseID 是条目所属知识库标识。
	KnowledgeBaseID int64
	// KnowledgeBaseName 是检索证据展示使用的知识库名称。
	KnowledgeBaseName string
	// Type 区分 FAQ 和文档持久化表。
	Type ContentType
	// Title 是 FAQ 问题或文档标题。
	Title string
	// Content 是 FAQ 答案或文档正文。
	Content string
	// ContentType 是文档的 text／markdown 格式；FAQ 固定为 faq。
	ContentType string
	// Status 是条目内容生命周期状态。
	Status EntryStatus
	// ReviewStatus 表示条目是否通过人工审核。
	ReviewStatus ReviewStatus
	// RiskLevel 是条目对候选回复的风险分级。
	RiskLevel RiskLevel
	// RequiresLiveData 表示回答是否必须结合实时商品或订单状态。
	RequiresLiveData bool
	// AllowAutoReply 为未来自动模式预留；第一阶段始终为 false。
	AllowAutoReply bool
	// Enabled 表示已审核条目是否允许进入离线检索。
	Enabled bool
	// Aliases 是用户确认且已启用的 FAQ 相似问法；文档固定为空。
	Aliases []string
	// EffectiveFrom 是条目生效的 Unix 秒；0 表示不限开始。
	EffectiveFrom int64
	// EffectiveTo 是条目失效的 Unix 秒；0 表示不限结束。
	EffectiveTo int64
	// CreatedAt 是条目创建时间。
	CreatedAt time.Time
	// UpdatedAt 是条目最近维护时间。
	UpdatedAt time.Time
}

// AliasSource 表示相似问法由手工、系统建议或调试器确认产生。
type AliasSource string

const (
	// AliasSourceManual 表示用户在 FAQ 管理弹窗手工添加。
	AliasSourceManual AliasSource = "manual"
	// AliasSourceSystem 表示用户采纳确定性系统建议。
	AliasSourceSystem AliasSource = "system"
	// AliasSourceDebug 表示用户在离线调试器中确认当前问题。
	AliasSourceDebug AliasSource = "debug"
)

// FAQAlias 是不包含聊天或凭据的 FAQ 相似问法应用模型。
type FAQAlias struct {
	// ID 是相似问法持久化标识。
	ID int64
	// KnowledgeBaseID 是所属知识库标识。
	KnowledgeBaseID int64
	// FAQID 是关联的 FAQ 标识。
	FAQID int64
	// Alias 是用户确认的原始问法。
	Alias string
	// NormalizedAlias 是同义词归一化后的冲突检查文本。
	NormalizedAlias string
	// Source 是 manual、system 或 debug 来源。
	Source AliasSource
	// Enabled 表示相似问法是否参与检索。
	Enabled bool
	// CreatedAt 是相似问法创建时间。
	CreatedAt time.Time
	// UpdatedAt 是相似问法最近维护时间。
	UpdatedAt time.Time
}

// FAQAliasDraft 是用户明确确认写入的相似问法输入。
type FAQAliasDraft struct {
	// Alias 是去除首尾空白后的买家常见问法。
	Alias string
	// Source 记录相似问法由哪个用户动作确认。
	Source AliasSource
}

// EntryDraft 是创建或更新 FAQ／文档的人工输入。
type EntryDraft struct {
	// Type 区分当前输入应写入 FAQ 还是文档。
	Type ContentType
	// Title 是 FAQ 问题或文档标题。
	Title string
	// Content 是 FAQ 标准答案或文档正文。
	Content string
	// ContentType 是文档的 text 或 markdown 格式。
	ContentType string
	// RiskLevel 是人工选择的业务风险分级。
	RiskLevel RiskLevel
	// RequiresLiveData 标记回答时是否需要结构化实时数据。
	RequiresLiveData bool
	// EffectiveFrom 是可选的 Unix 秒生效边界。
	EffectiveFrom int64
	// EffectiveTo 是可选的 Unix 秒失效边界。
	EffectiveTo int64
}

// Chunk 是应用层确定性切分后交给持久化层的文本片段。
type Chunk struct {
	// Index 是片段在源条目内的零基顺序。
	Index int
	// Content 是去除多余空白后的文本片段。
	Content string
	// ContentHash 是片段内容的 SHA-256 摘要。
	ContentHash string
}

// RetrieveRequest 是离线调试检索的应用输入。
type RetrieveRequest struct {
	// UserID 是当前已认证 ERP 用户标识。
	UserID int64
	// Query 是用户手工输入的脱敏调试问题。
	Query string
	// KnowledgeBaseIDs 是用户显式选择的调试知识库；空列表表示全部已启用知识库。
	KnowledgeBaseIDs []int64
}

// Answerability 是确定性门禁对离线调试问题的结论。
type Answerability string

const (
	// AnswerabilityAnswerable 表示已找到可支撑候选文案的已审核证据。
	AnswerabilityAnswerable Answerability = "answerable"
	// AnswerabilityNeedsLiveData 表示回答前必须获得实时订单或商品状态。
	AnswerabilityNeedsLiveData Answerability = "needs_live_data"
	// AnswerabilityNeedsHuman 表示高风险问题只允许转人工处理。
	AnswerabilityNeedsHuman Answerability = "needs_human"
	// AnswerabilityInsufficient 表示没有找到足以支撑回答的已审核证据。
	AnswerabilityInsufficient Answerability = "insufficient"
)

// Evidence 是检索结果中可追溯到具体 FAQ 或文档的证据。
type Evidence struct {
	// EntryID 是证据源条目标识。
	EntryID int64
	// KnowledgeBaseID 是证据所属知识库标识。
	KnowledgeBaseID int64
	// KnowledgeBaseName 是证据来源的用户可见知识库名称。
	KnowledgeBaseName string
	// Type 表示证据来自 FAQ 还是文档。
	Type ContentType
	// Title 是证据源问题或文档标题。
	Title string
	// Excerpt 是最小必要的证据文本片段。
	Excerpt string
	// Score 是 0 到 1 之间的确定性文本相关性分数。
	Score float64
}

// RetrieveResult 是离线调试的可回答性、候选文案和引用证据。
type RetrieveResult struct {
	// Query 是已去除首尾空白的调试问题。
	Query string
	// Answerability 是确定性门禁结论。
	Answerability Answerability
	// Explanation 说明允许或拒绝候选回复的原因。
	Explanation string
	// Candidate 是直接来自最高分已审核知识的离线候选文案，不经模型生成。
	Candidate string
	// Evidence 是支撑门禁结论的排序证据列表。
	Evidence []Evidence
}
