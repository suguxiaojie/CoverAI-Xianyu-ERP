package db

import (
	"database/sql"
	"time"
)

// KnowledgeStore 持有知识库三方言持久化所需的连接与方言。
type KnowledgeStore struct {
	// DB 是由进程组合根管理的共享数据库连接池。
	DB *sql.DB
	// Dialect 决定自增主键、布尔值和占位符的数据库差异。
	Dialect Dialect
}

// KnowledgeItemScopeRow 是知识库绑定的店铺商品持久化模型。
type KnowledgeItemScopeRow struct {
	// CookieID 是商品所属闲鱼店铺账号。
	CookieID string
	// ItemID 是店铺内商品标识。
	ItemID string
}

// KnowledgeBaseRow 是知识库、范围和审核数量的持久化读模型。
type KnowledgeBaseRow struct {
	// ID 是知识库数值主键。
	ID int64
	// UserID 是知识库所属 ERP 用户。
	UserID int64
	// Name 是用户可见名称。
	Name string
	// Description 是权威内容范围说明。
	Description string
	// Status 是 draft 或 active。
	Status string
	// CookieIDs 是显式绑定的店铺账号。
	CookieIDs []string
	// ItemScopes 是显式绑定的店铺商品。
	ItemScopes []KnowledgeItemScopeRow
	// EntryCount 是 FAQ 和文档的合计数量。
	EntryCount int
	// ReviewedCount 是已通过人工审核的条目数量。
	ReviewedCount int
	// CreatedAt 是知识库创建时间。
	CreatedAt time.Time
	// UpdatedAt 是最近维护时间。
	UpdatedAt time.Time
}

// KnowledgeBaseDraftRow 是应用适配器交给 DB 层的规范化知识库输入。
type KnowledgeBaseDraftRow struct {
	// Name 是已通过应用校验的知识库名称。
	Name string
	// Description 是已通过应用校验的范围说明。
	Description string
	// CookieIDs 是需要原子替换的店铺范围。
	CookieIDs []string
	// ItemScopes 是需要原子替换的店铺商品范围。
	ItemScopes []KnowledgeItemScopeRow
}

// KnowledgeEntryRow 是 FAQ 和文档共用的持久化读模型。
type KnowledgeEntryRow struct {
	// ID 是条目在所属类型表中的主键。
	ID int64
	// KnowledgeBaseID 是条目所属知识库标识。
	KnowledgeBaseID int64
	// KnowledgeBaseName 是检索证据显示所需的知识库名称。
	KnowledgeBaseName string
	// Type 是 faq 或 document。
	Type string
	// Title 是 FAQ 问题或文档标题。
	Title string
	// Content 是 FAQ 答案或文档正文。
	Content string
	// ContentType 是 faq、text 或 markdown。
	ContentType string
	// ContentHash 是整个 FAQ 问答或文档正文的 SHA-256 摘要。
	ContentHash string
	// Status 是 draft、active 或 disabled。
	Status string
	// ReviewStatus 是 pending 或 reviewed。
	ReviewStatus string
	// RiskLevel 是 low、medium 或 high。
	RiskLevel string
	// RequiresLiveData 表示是否需要实时商品或订单状态。
	RequiresLiveData bool
	// AllowAutoReply 是预留自动模式字段；第一阶段固定关闭。
	AllowAutoReply bool
	// Enabled 表示条目是否进入离线检索。
	Enabled bool
	// Aliases 是用户确认且已启用的 FAQ 相似问法；文档固定为空。
	Aliases []string
	// EffectiveFrom 是 Unix 秒生效下界；0 表示不限。
	EffectiveFrom int64
	// EffectiveTo 是 Unix 秒失效上界；0 表示不限。
	EffectiveTo int64
	// CreatedAt 是条目创建时间。
	CreatedAt time.Time
	// UpdatedAt 是条目最近维护时间。
	UpdatedAt time.Time
}

// KnowledgeFAQAliasRow 是 FAQ 相似问法的持久化读模型。
type KnowledgeFAQAliasRow struct {
	// ID 是相似问法数值主键。
	ID int64
	// KnowledgeBaseID 是相似问法所属知识库标识。
	KnowledgeBaseID int64
	// FAQID 是相似问法对应的 FAQ 标识。
	FAQID int64
	// Alias 是用户确认的原始相似问法，不来自聊天日志。
	Alias string
	// NormalizedAlias 是用于冲突检查和确定性匹配的规范化文本。
	NormalizedAlias string
	// Source 是 manual、system 或 debug 来源标签。
	Source string
	// Enabled 表示相似问法是否参与 FAQ 检索。
	Enabled bool
	// CreatedAt 是相似问法创建时间。
	CreatedAt time.Time
	// UpdatedAt 是相似问法最近维护时间。
	UpdatedAt time.Time
}

// KnowledgeEntryDraftRow 是应用适配器交给 DB 层的规范化 FAQ／文档输入。
type KnowledgeEntryDraftRow struct {
	// Type 是 faq 或 document。
	Type string
	// Title 是 FAQ 问题或文档标题。
	Title string
	// Content 是 FAQ 答案或文档正文。
	Content string
	// ContentType 是 faq、text 或 markdown。
	ContentType string
	// ContentHash 是整个 FAQ 问答或文档正文的 SHA-256 摘要。
	ContentHash string
	// RiskLevel 是 low、medium 或 high。
	RiskLevel string
	// RequiresLiveData 表示回答是否依赖实时业务数据。
	RequiresLiveData bool
	// EffectiveFrom 是 Unix 秒生效下界。
	EffectiveFrom int64
	// EffectiveTo 是 Unix 秒失效上界。
	EffectiveTo int64
}

// KnowledgeChunkRow 是应用层确定性切分后的持久化片段。
type KnowledgeChunkRow struct {
	// Index 是片段在源条目内的零基顺序。
	Index int
	// Content 是去除多余空白后的片段文本。
	Content string
	// ContentHash 是片段文本的 SHA-256 摘要。
	ContentHash string
}
