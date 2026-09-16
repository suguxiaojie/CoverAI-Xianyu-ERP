/** KnowledgeItemScopeResponse 是知识库绑定的店铺商品 HTTP DTO。 */
export interface KnowledgeItemScopeResponse {
  /** account_id 是商品所属店铺账号标识。 */
  account_id: string;
  /** item_id 是店铺内商品标识。 */
  item_id: string;
}

/** KnowledgeBaseResponse 是知识库范围、计数和时间 HTTP DTO。 */
export interface KnowledgeBaseResponse {
  /** id 是知识库数值主键。 */
  id: number;
  /** name 是知识库用户可见名称。 */
  name: string;
  /** description 是权威内容范围说明。 */
  description: string;
  /** status 是 draft 或 active。 */
  status: 'draft' | 'active';
  /** account_ids 是显式绑定的店铺账号。 */
  account_ids: string[];
  /** item_scopes 是显式绑定的店铺商品。 */
  item_scopes: KnowledgeItemScopeResponse[];
  /** entry_count 是 FAQ 和文档合计数量。 */
  entry_count: number;
  /** reviewed_count 是已通过人工审核的条目数量。 */
  reviewed_count: number;
  /** created_at 是 RFC3339 创建时间。 */
  created_at: string;
  /** updated_at 是 RFC3339 最近维护时间。 */
  updated_at: string;
}

/** KnowledgeBaseListResponse 是当前用户知识库列表 HTTP DTO。 */
export interface KnowledgeBaseListResponse {
  /** data 是当前用户的全部知识库。 */
  data: KnowledgeBaseResponse[];
}

/** KnowledgeBaseMutationRequest 是创建或更新知识库的 HTTP 输入。 */
export interface KnowledgeBaseMutationRequest {
  /** name 是知识库用户可见名称。 */
  name: string;
  /** description 是权威内容范围说明。 */
  description: string;
  /** account_ids 是用户选择的店铺范围。 */
  account_ids: string[];
  /** item_scopes 是用户选择的店铺商品范围。 */
  item_scopes: KnowledgeItemScopeResponse[];
}

/** KnowledgeEntryResponse 是 FAQ／文档审核与检索状态 HTTP DTO。 */
export interface KnowledgeEntryResponse {
  /** id 是 FAQ 或文档数值主键。 */
  id: number;
  /** knowledge_base_id 是条目所属知识库标识。 */
  knowledge_base_id: number;
  /** type 是 faq 或 document。 */
  type: 'faq' | 'document';
  /** title 是 FAQ 问题或文档标题。 */
  title: string;
  /** content 是 FAQ 标准答案或文档正文。 */
  content: string;
  /** content_type 是 faq、text 或 markdown。 */
  content_type: string;
  /** status 是 draft、active 或 disabled。 */
  status: 'draft' | 'active' | 'disabled';
  /** review_status 是 pending 或 reviewed。 */
  review_status: 'pending' | 'reviewed';
  /** risk_level 是 low、medium 或 high。 */
  risk_level: 'low' | 'medium' | 'high';
  /** requires_live_data 表示候选回复前是否需要实时业务状态。 */
  requires_live_data: boolean;
  /** allow_auto_reply 是预留自动标记；第一阶段固定为 false。 */
  allow_auto_reply: boolean;
  /** enabled 表示条目是否参与离线检索。 */
  enabled: boolean;
  /** effective_from 是 Unix 秒生效下界。 */
  effective_from: number;
  /** effective_to 是 Unix 秒失效上界。 */
  effective_to: number;
  /** created_at 是 RFC3339 创建时间。 */
  created_at: string;
  /** updated_at 是 RFC3339 最近维护时间。 */
  updated_at: string;
}

/** KnowledgeEntryListResponse 是单个知识库 FAQ／文档列表 HTTP DTO。 */
export interface KnowledgeEntryListResponse {
  /** data 是知识库下的全部 FAQ 和文档。 */
  data: KnowledgeEntryResponse[];
}

/** KnowledgeEntryMutationRequest 是创建 FAQ 或纯文本／Markdown 文档的 HTTP 输入。 */
export interface KnowledgeEntryMutationRequest {
  /** type 是 faq 或 document。 */
  type: 'faq' | 'document';
  /** title 是 FAQ 问题或文档标题。 */
  title: string;
  /** content 是 FAQ 标准答案或文档正文。 */
  content: string;
  /** content_type 是 faq、text 或 markdown。 */
  content_type: string;
  /** risk_level 是 low、medium 或 high。 */
  risk_level: 'low' | 'medium' | 'high';
  /** requires_live_data 表示候选回复前是否需要实时业务数据。 */
  requires_live_data: boolean;
  /** effective_from 是 Unix 秒生效下界。 */
  effective_from: number;
  /** effective_to 是 Unix 秒失效上界。 */
  effective_to: number;
}

/** KnowledgeFAQAliasResponse 是 FAQ 相似问法 HTTP DTO。 */
export interface KnowledgeFAQAliasResponse {
  /** id 是相似问法数值主键。 */
  id: number;
  /** knowledge_base_id 是所属知识库标识。 */
  knowledge_base_id: number;
  /** faq_id 是关联 FAQ 标识。 */
  faq_id: number;
  /** alias 是用户确认的原始问法。 */
  alias: string;
  /** source 是 manual、system 或 debug 来源。 */
  source: 'manual' | 'system' | 'debug';
  /** enabled 表示相似问法是否参与检索。 */
  enabled: boolean;
  /** created_at 是 RFC3339 创建时间。 */
  created_at: string;
  /** updated_at 是 RFC3339 最近维护时间。 */
  updated_at: string;
}

/** KnowledgeFAQAliasListResponse 是单个 FAQ 相似问法列表 HTTP DTO。 */
export interface KnowledgeFAQAliasListResponse {
  /** data 是当前 FAQ 的全部相似问法。 */
  data: KnowledgeFAQAliasResponse[];
}

/** KnowledgeFAQAliasSuggestionResponse 是不落库的系统建议列表 HTTP DTO。 */
export interface KnowledgeFAQAliasSuggestionResponse {
  /** data 是用户可选择采纳的确定性建议。 */
  data: string[];
}

/** KnowledgeEvidenceResponse 是离线检索结果中可追溯证据 HTTP DTO。 */
export interface KnowledgeEvidenceResponse {
  /** entry_id 是证据源 FAQ 或文档标识。 */
  entry_id: number;
  /** knowledge_base_id 是证据所属知识库标识。 */
  knowledge_base_id: number;
  /** knowledge_base_name 是证据所属知识库名称。 */
  knowledge_base_name: string;
  /** type 表示证据来自 FAQ 还是文档。 */
  type: 'faq' | 'document';
  /** title 是证据源问题或文档标题。 */
  title: string;
  /** excerpt 是最小必要的证据片段。 */
  excerpt: string;
  /** score 是 0 到 1 之间的确定性文本相关性分数。 */
  score: number;
}

/** KnowledgeRetrieveResponse 是离线检索可回答性和引用证据 HTTP DTO。 */
export interface KnowledgeRetrieveResponse {
  /** query 是规范化后的调试问题。 */
  query: string;
  /** answerability 是确定性门禁结论。 */
  answerability: 'answerable' | 'needs_live_data' | 'needs_human' | 'insufficient';
  /** explanation 说明门禁允许或拒绝候选回复的原因。 */
  explanation: string;
  /** candidate 是直接来自已审核知识的候选文案。 */
  candidate: string;
  /** evidence 是按相关性排序的引用证据。 */
  evidence: KnowledgeEvidenceResponse[];
}

/** MutationIDResponse 是新建知识库或条目的数值主键响应。 */
export interface MutationIDResponse {
  /** success 表示资源是否创建成功。 */
  success: boolean;
  /** id 是新资源数值主键。 */
  id: number;
}

/** OperationResponse 是知识审核、启停和状态切换响应。 */
export interface OperationResponse {
  /** success 表示变更是否完成。 */
  success: boolean;
  /** message 是可直接展示的变更结果。 */
  message?: string;
}
