import { beforeEach, describe, expect, test, vi } from 'vitest';
import type { KnowledgeBaseResponse, KnowledgeEntryResponse } from '../../../shared/api-contract/knowledge';
import { del, get, post, put } from '../../../shared/http/client';
import { createKnowledgeFAQAlias, deleteKnowledgeEntry, getKnowledgeFAQAliases, knowledgeBaseFromResponse, knowledgeEntryFromResponse, updateKnowledgeEntry } from './api';
import type { KnowledgeDraft, KnowledgeEntryPreview } from './types';

vi.mock('../../../shared/http/client', /* knowledgeHTTPMockFactory 隔离 API adapter 与真实正式服务。 */ () => ({ del: vi.fn(), get: vi.fn(), post: vi.fn(), put: vi.fn() }));

// putMock 是知识条目更新请求的 HTTP 替身。
const putMock = vi.mocked(put);
// delMock 是知识条目删除请求的 HTTP 替身。
const delMock = vi.mocked(del);
// getMock 是相似问法列表读取的 HTTP 替身。
const getMock = vi.mocked(get);
// postMock 是相似问法创建的 HTTP 替身。
const postMock = vi.mocked(post);

describe('knowledge API adapter', /* knowledgeApiAdapterSuite 验证 HTTP DTO 不泄漏到页面并保持范围和审核语义。 */ () => {
  beforeEach(/* knowledgeAdapterReset 清理前一用例的 HTTP 调用。 */ () => vi.clearAllMocks());

  test('知识库 DTO 转换店铺商品范围和计数', /* baseAdapterTest 验证结构化范围文案。 */ () => {
    // response 是包含两个店铺商品范围的 HTTP DTO。
    const response: KnowledgeBaseResponse = { id: 7, name: '商品教程', description: '权威说明', status: 'active', account_ids: ['a', 'b'], item_scopes: [{ account_id: 'a', item_id: 'i1' }, { account_id: 'b', item_id: 'i2' }], entry_count: 9, reviewed_count: 6, created_at: '2026-08-21T10:00:00Z', updated_at: '2026-08-21T11:00:00Z' };
    // model 是不包含 snake_case DTO 字段的 feature 模型。
    const model = knowledgeBaseFromResponse(response);
    expect(model).toMatchObject({ id: '7', scopeLabel: '2 个店铺 · 2 个商品', entryCount: 9, reviewedCount: 6, accent: 'success' });
  });

  test('待审核条目 DTO 保持停用且自动回复关闭', /* entryAdapterTest 验证审核安全默认。 */ () => {
    // response 是待审核、停用的 FAQ HTTP DTO。
    const response: KnowledgeEntryResponse = { id: 3, knowledge_base_id: 7, type: 'faq', title: '问题', content: '答案', content_type: 'faq', status: 'draft', review_status: 'pending', risk_level: 'medium', requires_live_data: true, allow_auto_reply: false, enabled: false, effective_from: 0, effective_to: 0, created_at: '2026-08-21T10:00:00Z', updated_at: '2026-08-21T10:00:00Z' };
    expect(knowledgeEntryFromResponse(response)).toMatchObject({ id: '3', knowledgeBaseId: '7', reviewed: false, enabled: false, requiresLiveData: true, allowAutoReply: false });
  });

  test('编辑文档使用已有 PUT 路径并独立保留实时数据标记', /* updateAdapterTest 验证类型、风险和实时数据不会互相推导。 */ async () => {
    putMock.mockResolvedValue({ success: true });
    // entry 是待编辑的中风险文档展示模型。
    const entry: KnowledgeEntryPreview = { id: '3', knowledgeBaseId: '7', type: 'document', title: '开票说明', content: '旧内容', scopeLabel: '店铺通用', riskLevel: 'medium', reviewed: true, enabled: true, requiresLiveData: true, allowAutoReply: false, createdAt: '2026-08-21T10:00:00Z', updatedAt: '2026-08-21T10:00:00Z' };
    // draft 是保持中风险但明确关闭实时数据依赖的编辑表单。
    const draft: KnowledgeDraft = { type: 'document', title: '开票说明', content: '新内容', scopeLabel: '店铺通用', riskLevel: 'medium', requiresLiveData: false };
    await updateKnowledgeEntry('7', entry, draft);
    expect(putMock).toHaveBeenCalledWith('/api/v1/knowledge-bases/7/entries/document/3', { type: 'document', title: '开票说明', content: '新内容', content_type: 'markdown', risk_level: 'medium', requires_live_data: false, effective_from: 0, effective_to: 0 }, undefined);
  });

  test('删除文档使用已有 DELETE 路径且不携带正文', /* deleteAdapterTest 验证删除只按知识库、类型和条目标识定位。 */ async () => {
    delMock.mockResolvedValue({ success: true });
    // entry 是只用于构造删除路径的文档展示模型。
    const entry: KnowledgeEntryPreview = { id: '3', knowledgeBaseId: '7', type: 'document', title: '开票说明', content: '内容', scopeLabel: '店铺通用', riskLevel: 'high', reviewed: true, enabled: true, requiresLiveData: false, allowAutoReply: false, createdAt: '2026-08-21T10:00:00Z', updatedAt: '2026-08-21T10:00:00Z' };
    await deleteKnowledgeEntry('7', entry);
    expect(delMock).toHaveBeenCalledWith('/api/v1/knowledge-bases/7/entries/document/3', undefined, undefined);
  });

  test('相似问法列表转换类型且只有确认动作才创建', /* aliasAdapterTest 验证相似问法通过版本化 API 独立维护。 */ async () => {
    getMock.mockResolvedValue({ data: [{ id: 8, knowledge_base_id: 7, faq_id: 3, alias: '怎么充值', source: 'debug', enabled: true, created_at: '2026-08-21T10:00:00Z', updated_at: '2026-08-21T10:00:00Z' }] });
    postMock.mockResolvedValue({ success: true, id: 9 });
    // aliases 是由 HTTP DTO 转换出的相似问法 feature 模型。
    const aliases = await getKnowledgeFAQAliases('7', '3');
    expect(aliases).toEqual([{ id: '8', knowledgeBaseId: '7', faqId: '3', alias: '怎么充值', source: 'debug', enabled: true }]);
    await createKnowledgeFAQAlias('7', '3', '如何充值', 'system');
    expect(postMock).toHaveBeenCalledWith('/api/v1/knowledge-bases/7/entries/faq/3/aliases', { alias: '如何充值', source: 'system' }, undefined);
  });
});
