/** KnowledgeContentType 区分人工维护的 FAQ 和纯文本文档。 */
export type KnowledgeContentType = 'faq' | 'document';

/** KnowledgeBaseStatus 表示知识库在设计预览中的启停状态。 */
export type KnowledgeBaseStatus = 'active' | 'draft';

/** KnowledgeRiskLevel 是可回答性门禁使用的三级业务风险。 */
export type KnowledgeRiskLevel = 'low' | 'medium' | 'high';

/** KnowledgeItemScope 是知识库显式绑定的店铺商品范围。 */
export interface KnowledgeItemScope {
  /** accountId 是商品所属闲鱼店铺账号标识。 */
  accountId: string;
  /** itemId 是店铺内商品标识。 */
  itemId: string;
}

/** KnowledgeScopeOption 是新建知识库时可选的用户、店铺或商品范围。 */
export interface KnowledgeScopeOption {
  /** value 是表单使用的稳定范围标识。 */
  value: string;
  /** label 是用户可见的脱敏店铺或商品名称。 */
  label: string;
  /** accountIds 是选中该项后写入的店铺范围。 */
  accountIds: string[];
  /** itemScopes 是选中该项后写入的店铺商品范围。 */
  itemScopes: KnowledgeItemScope[];
}

/** KnowledgeBasePreview 是前端设计验收使用的非持久化知识库摘要。 */
export interface KnowledgeBasePreview {
  /** id 是页面内部稳定选中知识库的演示标识。 */
  id: string;
  /** name 是用户可见的知识库名称。 */
  name: string;
  /** description 说明该知识库的权威内容范围。 */
  description: string;
  /** status 决定知识库在检索预览中是否可用。 */
  status: KnowledgeBaseStatus;
  /** scopeLabel 用脱敏文案摘要展示店铺或商品范围。 */
  scopeLabel: string;
  /** accountIds 是知识库显式绑定的店铺账号标识。 */
  accountIds: string[];
  /** itemScopes 是知识库显式绑定的店铺商品标识。 */
  itemScopes: KnowledgeItemScope[];
  /** entryCount 是当前预览知识条目数量。 */
  entryCount: number;
  /** reviewedCount 是已通过人工审核的预览条目数量。 */
  reviewedCount: number;
  /** accent 为列表卡片提供不承载业务语义的视觉辅助色。 */
  accent: 'brand' | 'success' | 'warning';
  /** createdAt 是知识库的 RFC3339 创建时间。 */
  createdAt: string;
  /** updatedAt 是知识库的 RFC3339 最近维护时间。 */
  updatedAt: string;
}

/** KnowledgeEntryPreview 是设计预览中一条可审核 FAQ 或文档。 */
export interface KnowledgeEntryPreview {
  /** id 是条目在当前页面生命周期内的稳定标识。 */
  id: string;
  /** knowledgeBaseId 指向条目所属知识库。 */
  knowledgeBaseId: string;
  /** type 区分结构化 FAQ 和段落文档。 */
  type: KnowledgeContentType;
  /** title 是条目列表中的主标题或 FAQ 问题。 */
  title: string;
  /** content 是脱敏的演示答案或文档正文。 */
  content: string;
  /** scopeLabel 说明条目适用的店铺、商品或通用范围。 */
  scopeLabel: string;
  /** riskLevel 供可回答性门禁判断是否允许未来自动化。 */
  riskLevel: KnowledgeRiskLevel;
  /** reviewed 表示条目是否经人工审核。 */
  reviewed: boolean;
  /** enabled 表示条目是否可进入当前检索预览。 */
  enabled: boolean;
  /** requiresLiveData 标记回答时是否必须读取实时商品或订单状态。 */
  requiresLiveData: boolean;
  /** allowAutoReply 是预留的自动回复标记；第一阶段始终为 false。 */
  allowAutoReply: boolean;
  /** updatedAt 是纯展示的脱敏最近维护时间。 */
  updatedAt: string;
  /** createdAt 是条目的 RFC3339 创建时间。 */
  createdAt: string;
}

/** KnowledgeFAQAliasPreview 是用户确认保存的 FAQ 相似问法。 */
export interface KnowledgeFAQAliasPreview {
  /** id 是相似问法稳定主键。 */
  id: string;
  /** knowledgeBaseId 是所属知识库标识。 */
  knowledgeBaseId: string;
  /** faqId 是关联 FAQ 标识。 */
  faqId: string;
  /** alias 是用于匹配买家口语的原始问法。 */
  alias: string;
  /** source 是手工、系统建议或调试器确认来源。 */
  source: 'manual' | 'system' | 'debug';
  /** enabled 表示相似问法是否参与检索。 */
  enabled: boolean;
}

/** RetrievalAnswerability 表示检索结果的可回答性结论。 */
export type RetrievalAnswerability = 'answerable' | 'needs_live_data' | 'needs_human' | 'insufficient';

/** RetrievalEvidencePreview 是调试结果中一条可追溯的知识证据。 */
export interface RetrievalEvidencePreview {
  /** entryId 指向证据对应的预览知识条目。 */
  entryId: string;
  /** knowledgeBaseId 是证据所属知识库标识。 */
  knowledgeBaseId: string;
  /** type 区分 FAQ 和文档，只有 FAQ 可以添加相似问法。 */
  type: KnowledgeContentType;
  /** title 是用户识别证据来源的条目标题。 */
  title: string;
  /** excerpt 是显示在调试区的最小必要脱敏片段。 */
  excerpt: string;
  /** sourceLabel 说明证据所属知识库与类型。 */
  sourceLabel: string;
  /** score 是 0 到 1 之间的演示相关性分数。 */
  score: number;
}

/** RetrievalPreview 是本地夹具产生的检索调试结果，不代表真实模型输出。 */
export interface RetrievalPreview {
  /** query 是用户在调试面板提交的问题。 */
  query: string;
  /** answerability 是确定性门禁对该问题的结论。 */
  answerability: RetrievalAnswerability;
  /** explanation 说明门禁允许或拒绝回答的理由。 */
  explanation: string;
  /** candidate 是仅供前端验收的候选文案，永不发送给真实买家。 */
  candidate: string;
  /** evidence 保存当前门禁判断使用的证据列表。 */
  evidence: RetrievalEvidencePreview[];
}

/** KnowledgeDraft 是“新增知识”预览弹窗的短暂表单数据。 */
export interface KnowledgeDraft {
  /** type 是用户选择的知识内容类型。 */
  type: KnowledgeContentType;
  /** title 是 FAQ 问题或文档标题。 */
  title: string;
  /** content 是 FAQ 答案或文档内容。 */
  content: string;
  /** scopeLabel 是设计预览中的适用范围文案。 */
  scopeLabel: string;
  /** riskLevel 是人工为该条目选择的风险等级。 */
  riskLevel: KnowledgeRiskLevel;
  /** requiresLiveData 表示候选回复前是否必须读取实时商品或订单状态。 */
  requiresLiveData: boolean;
}
