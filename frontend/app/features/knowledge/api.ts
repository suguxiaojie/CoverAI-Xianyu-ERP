import type {
  KnowledgeBaseListResponse,
  KnowledgeBaseMutationRequest,
  KnowledgeBaseResponse,
  KnowledgeEntryListResponse,
  KnowledgeEntryMutationRequest,
  KnowledgeEntryResponse,
  KnowledgeFAQAliasListResponse,
  KnowledgeFAQAliasResponse,
  KnowledgeFAQAliasSuggestionResponse,
  KnowledgeRetrieveResponse,
  MutationIDResponse,
  OperationResponse,
} from '../../../shared/api-contract/knowledge';
import type { AccountDetail } from '../../../shared/api-contract/accounts';
import type { Item } from '../../../shared/api-contract/items';
import { del, get, post, put, type RequestControlOptions } from '../../../shared/http/client';
import { collectionFrom } from '../../../shared/http/contract';
import type { KnowledgeBasePreview, KnowledgeDraft, KnowledgeEntryPreview, KnowledgeFAQAliasPreview, KnowledgeScopeOption, RetrievalPreview } from './types';

/** KnowledgeBaseCreateInput 是 feature 交给 API adapter 的知识库创建输入，不暴露 HTTP snake_case DTO。 */
export interface KnowledgeBaseCreateInput {
  /** name 是知识库用户可见名称。 */
  name: string;
  /** description 是权威内容范围说明。 */
  description: string;
  /** accountIds 是用户选择的店铺范围。 */
  accountIds: string[];
  /** itemScopes 是用户选择的店铺商品范围。 */
  itemScopes: KnowledgeScopeOption['itemScopes'];
}

/** knowledgeBaseAccent 根据知识库启用状态和商品范围选择稳定辅助色。 */
const knowledgeBaseAccent = (response: KnowledgeBaseResponse): KnowledgeBasePreview['accent'] => {
  if (response.status === 'draft') return 'warning';
  if (response.item_scopes.length > 0) return 'success';
  return 'brand';
};

/** knowledgeBaseScopeLabel 将结构化店铺／商品范围转换为简洁用户文案。 */
const knowledgeBaseScopeLabel = (response: KnowledgeBaseResponse): string => {
  if (response.item_scopes.length > 0) {
    // accountCount 是商品范围中去重后的店铺数量。
    const accountCount = new Set(response.item_scopes.map(/* scope 是当前待统计的店铺商品范围。 */ scope => scope.account_id)).size;
    return `${accountCount} 个店铺 · ${response.item_scopes.length} 个商品`;
  }
  if (response.account_ids.length > 0) return `${response.account_ids.length} 个店铺 · 通用`;
  return '全部店铺 · 用户通用';
};

/** knowledgeBaseFromResponse 将知识库 HTTP DTO 转换为 feature 展示模型。 */
export const knowledgeBaseFromResponse = (response: KnowledgeBaseResponse): KnowledgeBasePreview => ({
  id: String(response.id),
  name: response.name,
  description: response.description,
  status: response.status,
  scopeLabel: knowledgeBaseScopeLabel(response),
  accountIds: response.account_ids || [],
  itemScopes: (response.item_scopes || []).map(/* scope 是当前转换为 feature 范围的商品 DTO。 */ scope => ({ accountId: scope.account_id, itemId: scope.item_id })),
  entryCount: response.entry_count,
  reviewedCount: response.reviewed_count,
  accent: knowledgeBaseAccent(response),
  createdAt: response.created_at,
  updatedAt: response.updated_at,
});

/** knowledgeEntryFromResponse 将 FAQ／文档 HTTP DTO 转换为 feature 审核展示模型。 */
export const knowledgeEntryFromResponse = (response: KnowledgeEntryResponse): KnowledgeEntryPreview => ({
  id: String(response.id),
  knowledgeBaseId: String(response.knowledge_base_id),
  type: response.type,
  title: response.title,
  content: response.content,
  scopeLabel: '',
  riskLevel: response.risk_level,
  reviewed: response.review_status === 'reviewed',
  enabled: response.enabled,
  requiresLiveData: response.requires_live_data,
  allowAutoReply: response.allow_auto_reply,
  updatedAt: response.updated_at,
  createdAt: response.created_at,
});

/** getKnowledgeBases 读取当前用户全部知识库并转换为 feature 模型。 */
export const getKnowledgeBases = async (options?: RequestControlOptions): Promise<KnowledgeBasePreview[]> => {
  // response 是知识库列表 HTTP DTO。
  const response = await get<KnowledgeBaseListResponse>('/api/v1/knowledge-bases', undefined, options);
  return (response.data || []).map(/* knowledgeBase 是当前转换为 feature 模型的知识库 DTO。 */ knowledgeBaseFromResponse);
};

/** getKnowledgeEntries 读取单个知识库的 FAQ 和文档。 */
export const getKnowledgeEntries = async (knowledgeBaseID: string, options?: RequestControlOptions): Promise<KnowledgeEntryPreview[]> => {
  // response 是单个知识库条目列表 HTTP DTO。
  const response = await get<KnowledgeEntryListResponse>(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries`, undefined, options);
  return (response.data || []).map(/* entry 是当前转换为 feature 模型的 FAQ／文档 DTO。 */ knowledgeEntryFromResponse);
};

/** getKnowledgeScopeOptions 读取当前用户的店铺与商品，组成知识库范围选项。 */
export const getKnowledgeScopeOptions = async (options?: RequestControlOptions): Promise<KnowledgeScopeOption[]> => {
  // accountResponse 是非敏感账号摘要 HTTP 响应。
  // itemResponse 是当前用户本地商品目录 HTTP 响应。
  const [accountResponse, itemResponse] = await Promise.all([get<unknown>('/api/v1/accounts/details', undefined, options), get<unknown>('/api/v1/items', undefined, options)]);
  // accounts 是从直接数组或具名包裹中取得的非敏感店铺摘要。
  const accounts = collectionFrom<AccountDetail>(accountResponse, ['data', 'accounts', 'details']);
  // items 是从直接数组或具名包裹中取得的本地商品目录。
  const items = collectionFrom<Item>(itemResponse, ['items', 'data', 'results']);
  // accountLabels 是店铺标识到脱敏显示名称的映射。
  const accountLabels = new Map(accounts.map(/* account 是当前待归一显示名称的账号摘要。 */ account => [account.id, account.nickname || account.remark || `店铺 ${account.id.slice(0, 6)}`]));
  // scopeOptions 先放入用户通用范围，再追加店铺和商品选项。
  const scopeOptions: KnowledgeScopeOption[] = [{ value: 'all', label: '全部店铺 · 用户通用', accountIds: [], itemScopes: [] }];
  // account 是当前待追加为知识库范围的店铺摘要。
  for (const account /* account 是当前待追加为知识库范围的店铺摘要。 */ of accounts) {
    scopeOptions.push({ value: `account:${account.id}`, label: `${accountLabels.get(account.id)} · 店铺通用`, accountIds: [account.id], itemScopes: [] });
  }
  // item 是当前待追加为知识库范围的本地商品。
  for (const item /* item 是当前待追加为知识库范围的本地商品。 */ of items) {
    scopeOptions.push({ value: `item:${item.cookie_id}:${item.item_id}`, label: `${accountLabels.get(item.cookie_id) || `店铺 ${item.cookie_id.slice(0, 6)}`} · ${item.item_title || item.item_id}`, accountIds: [item.cookie_id], itemScopes: [{ accountId: item.cookie_id, itemId: item.item_id }] });
  }
  return scopeOptions;
};

/** createKnowledgeBase 将 feature 输入转换为 HTTP DTO 并创建默认草稿知识库。 */
export const createKnowledgeBase = async (input: KnowledgeBaseCreateInput, options?: RequestControlOptions): Promise<MutationIDResponse> => {
  // request 是仅在 API adapter 内可见的知识库 HTTP DTO。
  const request: KnowledgeBaseMutationRequest = { name: input.name, description: input.description, account_ids: input.accountIds, item_scopes: input.itemScopes.map(/* scope 是当前转换为 HTTP DTO 的店铺商品范围。 */ scope => ({ account_id: scope.accountId, item_id: scope.itemId })) };
  return post('/api/v1/knowledge-bases', request, options);
};

/** setKnowledgeBaseStatus 切换知识库草稿／离线检索启用状态。 */
export const setKnowledgeBaseStatus = async (knowledgeBaseID: string, status: KnowledgeBasePreview['status'], options?: RequestControlOptions): Promise<OperationResponse> => put(`/api/v1/knowledge-bases/${knowledgeBaseID}/status`, { status }, options);

/** deleteKnowledgeBase 删除当前用户拥有的知识库。 */
export const deleteKnowledgeBase = async (knowledgeBaseID: string, options?: RequestControlOptions): Promise<OperationResponse> => del(`/api/v1/knowledge-bases/${knowledgeBaseID}`, undefined, options);

/** knowledgeEntryRequest 将前端表单转换为默认当前生效时间不受限的 HTTP 输入。 */
const knowledgeEntryRequest = (draft: KnowledgeDraft): KnowledgeEntryMutationRequest => ({
  type: draft.type,
  title: draft.title,
  content: draft.content,
  content_type: draft.type === 'document' ? 'markdown' : 'faq',
  risk_level: draft.riskLevel,
  requires_live_data: draft.requiresLiveData,
  effective_from: 0,
  effective_to: 0,
});

/** createKnowledgeEntry 创建默认待审核、停用的 FAQ 或文档。 */
export const createKnowledgeEntry = async (knowledgeBaseID: string, draft: KnowledgeDraft, options?: RequestControlOptions): Promise<MutationIDResponse> => post(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries`, knowledgeEntryRequest(draft), options);

/** updateKnowledgeEntry 更新 FAQ／文档，并由后端强制重置审核和检索启用状态。 */
export const updateKnowledgeEntry = async (knowledgeBaseID: string, entry: KnowledgeEntryPreview, draft: KnowledgeDraft, options?: RequestControlOptions): Promise<OperationResponse> => put(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/${entry.type}/${entry.id}`, knowledgeEntryRequest(draft), options);

/** deleteKnowledgeEntry 删除单条 FAQ／文档及其确定性检索分块。 */
export const deleteKnowledgeEntry = async (knowledgeBaseID: string, entry: KnowledgeEntryPreview, options?: RequestControlOptions): Promise<OperationResponse> => del(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/${entry.type}/${entry.id}`, undefined, options);

/** reviewKnowledgeEntry 保存单个 FAQ 或文档的人工审核结果。 */
export const reviewKnowledgeEntry = async (knowledgeBaseID: string, entry: KnowledgeEntryPreview, reviewed: boolean, options?: RequestControlOptions): Promise<OperationResponse> => put(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/${entry.type}/${entry.id}/review`, { reviewed }, options);

/** setKnowledgeEntryEnabled 切换已审核 FAQ 或文档的离线检索状态。 */
export const setKnowledgeEntryEnabled = async (knowledgeBaseID: string, entry: KnowledgeEntryPreview, enabled: boolean, options?: RequestControlOptions): Promise<OperationResponse> => put(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/${entry.type}/${entry.id}/enabled`, { enabled }, options);

/** knowledgeFAQAliasFromResponse 将相似问法 HTTP DTO 转换为 feature 模型。 */
export const knowledgeFAQAliasFromResponse = (response: KnowledgeFAQAliasResponse): KnowledgeFAQAliasPreview => ({ id: String(response.id), knowledgeBaseId: String(response.knowledge_base_id), faqId: String(response.faq_id), alias: response.alias, source: response.source, enabled: response.enabled });

/** getKnowledgeFAQAliases 读取单个 FAQ 的用户确认相似问法。 */
export const getKnowledgeFAQAliases = async (knowledgeBaseID: string, faqID: string, options?: RequestControlOptions): Promise<KnowledgeFAQAliasPreview[]> => {
  // response 是当前 FAQ 相似问法列表 DTO。
  const response = await get<KnowledgeFAQAliasListResponse>(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/faq/${faqID}/aliases`, undefined, options);
  return (response.data || []).map(/* alias 是当前待转换的相似问法 DTO。 */ alias => knowledgeFAQAliasFromResponse(alias));
};

/** getKnowledgeFAQAliasSuggestions 读取不落库的确定性相似问法建议。 */
export const getKnowledgeFAQAliasSuggestions = async (knowledgeBaseID: string, faqID: string, options?: RequestControlOptions): Promise<string[]> => {
  // response 是当前 FAQ 的确定性相似问法建议 DTO。
  const response = await get<KnowledgeFAQAliasSuggestionResponse>(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/faq/${faqID}/alias-suggestions`, undefined, options);
  return response.data || [];
};

/** createKnowledgeFAQAlias 保存用户手工、系统建议或调试器确认的相似问法。 */
export const createKnowledgeFAQAlias = async (knowledgeBaseID: string, faqID: string, alias: string, source: KnowledgeFAQAliasPreview['source'], options?: RequestControlOptions): Promise<MutationIDResponse> => post(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/faq/${faqID}/aliases`, { alias, source }, options);

/** deleteKnowledgeFAQAlias 删除单条相似问法，不改变 FAQ 标准答案。 */
export const deleteKnowledgeFAQAlias = async (knowledgeBaseID: string, faqID: string, aliasID: string, options?: RequestControlOptions): Promise<OperationResponse> => del(`/api/v1/knowledge-bases/${knowledgeBaseID}/entries/faq/${faqID}/aliases/${aliasID}`, undefined, options);

/** retrieveKnowledge 执行不调用模型的离线文本检索并转换引用证据。 */
export const retrieveKnowledge = async (query: string, knowledgeBaseIDs: string[], options?: RequestControlOptions): Promise<RetrievalPreview> => {
  // response 是后端确定性检索与可回答性 HTTP DTO。
  const response = await post<KnowledgeRetrieveResponse>('/api/v1/knowledge-retrieve', { query, knowledge_base_ids: knowledgeBaseIDs.map(/* knowledgeBaseID 是当前转换为数值主键的 feature 标识。 */ knowledgeBaseID => Number(knowledgeBaseID)) }, options);
  return {
    query: response.query,
    answerability: response.answerability,
    explanation: response.explanation,
    candidate: response.candidate,
    evidence: (response.evidence || []).map(/* evidence 是当前转换为可追溯 feature 证据的 HTTP DTO。 */ evidence => ({
      entryId: String(evidence.entry_id),
      knowledgeBaseId: String(evidence.knowledge_base_id),
      type: evidence.type,
      title: evidence.title,
      excerpt: evidence.excerpt,
      sourceLabel: `${evidence.knowledge_base_name} · ${evidence.type === 'faq' ? 'FAQ' : '文档'}`,
      score: evidence.score,
    })),
  };
};
