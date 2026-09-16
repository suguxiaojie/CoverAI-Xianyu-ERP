// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import type { KnowledgeBasePreview, KnowledgeEntryPreview, RetrievalPreview } from '../types';
import KnowledgePreview from './KnowledgePreview';
import {
  createKnowledgeBase,
  createKnowledgeEntry,
  createKnowledgeFAQAlias,
  deleteKnowledgeEntry,
  getKnowledgeBases,
  getKnowledgeEntries,
  getKnowledgeFAQAliases,
  getKnowledgeFAQAliasSuggestions,
  getKnowledgeScopeOptions,
  retrieveKnowledge,
  reviewKnowledgeEntry,
  setKnowledgeBaseStatus,
  setKnowledgeEntryEnabled,
  updateKnowledgeEntry,
} from '../api';

vi.mock('../api', /* knowledgeApiMockFactory 隔离页面行为测试与真实本地 HTTP 服务。 */ () => ({
  createKnowledgeBase: vi.fn(),
  createKnowledgeEntry: vi.fn(),
  createKnowledgeFAQAlias: vi.fn(),
  deleteKnowledgeFAQAlias: vi.fn(),
  deleteKnowledgeEntry: vi.fn(),
  getKnowledgeBases: vi.fn(),
  getKnowledgeEntries: vi.fn(),
  getKnowledgeFAQAliases: vi.fn(),
  getKnowledgeFAQAliasSuggestions: vi.fn(),
  getKnowledgeScopeOptions: vi.fn(),
  retrieveKnowledge: vi.fn(),
  reviewKnowledgeEntry: vi.fn(),
  setKnowledgeBaseStatus: vi.fn(),
  setKnowledgeEntryEnabled: vi.fn(),
  updateKnowledgeEntry: vi.fn(),
}));

// getKnowledgeBasesMock 是读取知识库列表的可控替身。
const getKnowledgeBasesMock = vi.mocked(getKnowledgeBases);
// getKnowledgeEntriesMock 是读取 FAQ／文档列表的可控替身。
const getKnowledgeEntriesMock = vi.mocked(getKnowledgeEntries);
// getKnowledgeScopeOptionsMock 是读取店铺／商品范围选项的可控替身。
const getKnowledgeScopeOptionsMock = vi.mocked(getKnowledgeScopeOptions);
// createKnowledgeBaseMock 是创建草稿知识库的可控替身。
const createKnowledgeBaseMock = vi.mocked(createKnowledgeBase);
// createKnowledgeEntryMock 是创建待审核条目的可控替身。
const createKnowledgeEntryMock = vi.mocked(createKnowledgeEntry);
// updateKnowledgeEntryMock 是编辑已有 FAQ／文档的可控替身。
const updateKnowledgeEntryMock = vi.mocked(updateKnowledgeEntry);
// deleteKnowledgeEntryMock 是删除单条 FAQ／文档的可控替身。
const deleteKnowledgeEntryMock = vi.mocked(deleteKnowledgeEntry);
// getKnowledgeFAQAliasesMock 是读取已保存相似问法的可控替身。
const getKnowledgeFAQAliasesMock = vi.mocked(getKnowledgeFAQAliases);
// getKnowledgeFAQAliasSuggestionsMock 是读取确定性建议的可控替身。
const getKnowledgeFAQAliasSuggestionsMock = vi.mocked(getKnowledgeFAQAliasSuggestions);
// createKnowledgeFAQAliasMock 是用户确认新增相似问法的可控替身。
const createKnowledgeFAQAliasMock = vi.mocked(createKnowledgeFAQAlias);
// reviewKnowledgeEntryMock 是人工审核条目的可控替身。
const reviewKnowledgeEntryMock = vi.mocked(reviewKnowledgeEntry);
// setKnowledgeEntryEnabledMock 是条目离线检索启停的可控替身。
const setKnowledgeEntryEnabledMock = vi.mocked(setKnowledgeEntryEnabled);
// retrieveKnowledgeMock 是后端确定性检索的可控替身。
const retrieveKnowledgeMock = vi.mocked(retrieveKnowledge);
// setKnowledgeBaseStatusMock 是知识库草稿／启用变更的可控替身。
const setKnowledgeBaseStatusMock = vi.mocked(setKnowledgeBaseStatus);

/** baseFixture 是页面读取成功时的已启用知识库展示模型。 */
const baseFixture: KnowledgeBasePreview = {
  id: '7',
  name: '使用教程',
  description: '已确认的商品使用说明。',
  status: 'active',
  scopeLabel: '全部店铺 · 用户通用',
  accountIds: [],
  itemScopes: [],
  entryCount: 2,
  reviewedCount: 1,
  accent: 'brand',
  createdAt: '2026-08-21T10:00:00Z',
  updatedAt: '2026-08-21T10:00:00Z',
};

/** reviewedEntryFixture 是已审核且启用的低风险 FAQ。 */
const reviewedEntryFixture: KnowledgeEntryPreview = {
  id: '11',
  knowledgeBaseId: '7',
  type: 'faq',
  title: '这个商品怎么使用？',
  content: '请按商品说明完成设置。',
  scopeLabel: '',
  riskLevel: 'low',
  reviewed: true,
  enabled: true,
  requiresLiveData: false,
  allowAutoReply: false,
  updatedAt: '2026-08-21T10:00:00Z',
  createdAt: '2026-08-21T10:00:00Z',
};

/** pendingEntryFixture 是尚未通过人工审核的 FAQ。 */
const pendingEntryFixture: KnowledgeEntryPreview = {
  ...reviewedEntryFixture,
  id: '12',
  title: '可以查看处理进度吗？',
  reviewed: false,
  enabled: false,
};

/** safeRetrievalFixture 是有一条权威引用的低风险离线检索结果。 */
const safeRetrievalFixture: RetrievalPreview = {
  query: '这个商品怎么使用？',
  answerability: 'answerable',
  explanation: '已找到当前生效的已审核知识。',
  candidate: '请按商品说明完成设置。',
  evidence: [{ entryId: '11', knowledgeBaseId: '7', type: 'faq', title: reviewedEntryFixture.title, excerpt: reviewedEntryFixture.content, sourceLabel: '使用教程 · FAQ', score: 0.94 }],
};

describe('KnowledgePreview 本地 API 交互', /* knowledgePreviewSuite 验证已验收布局在服务端数据、审核和检索下保持安全。 */ () => {
  beforeEach(/* knowledgeMockReset 为每个页面行为测试恢复稳定 API 响应。 */ () => {
    vi.clearAllMocks();
    getKnowledgeBasesMock.mockResolvedValue([baseFixture]);
    getKnowledgeEntriesMock.mockResolvedValue([reviewedEntryFixture, pendingEntryFixture]);
    getKnowledgeScopeOptionsMock.mockResolvedValue([{ value: 'all', label: '全部店铺 · 用户通用', accountIds: [], itemScopes: [] }]);
    getKnowledgeFAQAliasesMock.mockResolvedValue([{ id: '21', knowledgeBaseId: '7', faqId: '11', alias: '怎么设置', source: 'manual', enabled: true }]);
    getKnowledgeFAQAliasSuggestionsMock.mockResolvedValue(['如何设置']);
    createKnowledgeBaseMock.mockResolvedValue({ success: true, id: 8 });
    createKnowledgeEntryMock.mockResolvedValue({ success: true, id: 13 });
    updateKnowledgeEntryMock.mockResolvedValue({ success: true });
    deleteKnowledgeEntryMock.mockResolvedValue({ success: true });
    createKnowledgeFAQAliasMock.mockResolvedValue({ success: true, id: 22 });
    reviewKnowledgeEntryMock.mockResolvedValue({ success: true });
    setKnowledgeEntryEnabledMock.mockResolvedValue({ success: true });
    setKnowledgeBaseStatusMock.mockResolvedValue({ success: true });
    retrieveKnowledgeMock.mockResolvedValue(safeRetrievalFixture);
  });

  afterEach(/* knowledgePreviewCleanup 清理每个页面行为测试留下的 DOM 和弹窗。 */ () => cleanup());

  test('读取本地 API 后显示已验收布局并保持在线发送关闭', /* loadedLayoutTest 验证真实数据替换夹具后的安全文案。 */ async () => {
    render(<KnowledgePreview />);
    await waitFor(/* loadedBaseAssertion 等待知识库和条目请求完成。 */ () => expect(screen.getAllByText('使用教程').length).toBeGreaterThan(0));
    expect(screen.getByText('已接本地 API · 未调模型 · 不会发送消息')).toBeTruthy();
    await waitFor(/* loadedEntriesAssertion 等待选中知识库条目请求完成。 */ () => expect(screen.getByText('可以查看处理进度吗？')).toBeTruthy());
    expect(screen.getAllByText('参与检索')).toHaveLength(2);
    expect(screen.getAllByText('已启用').length).toBeGreaterThan(0);
    expect((screen.getByRole('switch', { name: '待审核不能启用可以查看处理进度吗？' }) as HTMLButtonElement).disabled).toBe(true);
  });

  test('快捷问题通过后端检索并显示候选与引用', /* retrievalTest 验证页面不在前端重复实现排名和门禁。 */ async () => {
    render(<KnowledgePreview />);
    await waitFor(/* loadedBaseAssertion 等待知识库列表加载。 */ () => expect(screen.getAllByText('使用教程').length).toBeGreaterThan(0));
    fireEvent.click(screen.getByRole('button', { name: '这个商品怎么使用？' }));
    await waitFor(/* retrievalAssertion 等待后端离线检索响应显示。 */ () => expect(screen.getByText('证据充分')).toBeTruthy());
    expect(retrieveKnowledgeMock).toHaveBeenCalledWith('这个商品怎么使用？', ['7'], expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(screen.getByText('候选回复预览')).toBeTruthy();
    expect(screen.getByText('1 条命中')).toBeTruthy();
  });

  test('待审核条目需要明确人工确认后才可启用', /* reviewTest 验证页面将审核和启停分为两步。 */ async () => {
    render(<KnowledgePreview />);
    await waitFor(/* pendingEntryAssertion 等待待审核条目显示。 */ () => expect(screen.getByText('可以查看处理进度吗？')).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: '通过审核' }));
    await waitFor(/* reviewCallAssertion 等待审核 API 被调用。 */ () => expect(reviewKnowledgeEntryMock).toHaveBeenCalledWith('7', expect.objectContaining({ id: pendingEntryFixture.id, title: pendingEntryFixture.title }), true, expect.objectContaining({ signal: expect.any(AbortSignal) })));
  });

  test('新增 FAQ 保存到本地 API 而不再声称仅页面预览', /* createEntryTest 验证弹窗保存语义与第一阶段后端一致。 */ async () => {
    render(<KnowledgePreview />);
    await waitFor(/* loadedBaseAssertion 等待知识库加载后开放新增入口。 */ () => expect(screen.getByRole('button', { name: '新增知识' })).toBeTruthy());
    fireEvent.click(screen.getByRole('button', { name: '新增知识' }));
    fireEvent.change(screen.getByPlaceholderText('例如：这个商品怎么使用？'), { target: { value: '新 FAQ' } });
    fireEvent.change(screen.getByPlaceholderText('只填写已确认的权威内容，不从历史聊天自动归纳。'), { target: { value: '已确认的标准答案。' } });
    fireEvent.click(screen.getByRole('button', { name: '保存待审核知识' }));
    await waitFor(/* createCallAssertion 等待新增 FAQ API 被调用。 */ () => expect(createKnowledgeEntryMock).toHaveBeenCalledWith('7', expect.objectContaining({ title: '新 FAQ', content: '已确认的标准答案。', requiresLiveData: false }), expect.objectContaining({ signal: expect.any(AbortSignal) })));
  });

  test('编辑已启用 FAQ 回填内容并独立保存实时数据开关', /* editEntryTest 验证编辑复用表单且不从风险等级推导实时数据。 */ async () => {
    render(<KnowledgePreview />);
    // editButton 是条目列表完成异步加载后出现的 FAQ 编辑入口。
    const editButton = await waitFor(/* loadedEntryAssertion 等待已审核 FAQ 的编辑入口显示。 */ () => screen.getByRole('button', { name: `编辑FAQ：${reviewedEntryFixture.title}` }));
    fireEvent.click(editButton);
    expect(screen.getByRole('dialog', { name: '编辑FAQ' })).toBeTruthy();
    expect(screen.getByText('保存修改后，该知识将自动退出检索并变为待审核；重新审核并启用后才会再次参与候选回复。')).toBeTruthy();
    // contentInput 是回填已有标准答案的编辑文本框。
    const contentInput = screen.getByPlaceholderText('只填写已确认的权威内容，不从历史聊天自动归纳。');
    fireEvent.change(contentInput, { target: { value: '修改后的标准答案。' } });
    fireEvent.click(screen.getByRole('checkbox', { name: '需要实时数据' }));
    fireEvent.click(screen.getByRole('button', { name: '保存修改并重新审核' }));
    await waitFor(/* updateCallAssertion 等待 FAQ 更新接口收到独立实时数据标记。 */ () => expect(updateKnowledgeEntryMock).toHaveBeenCalledWith('7', expect.objectContaining({ id: reviewedEntryFixture.id, type: 'faq' }), expect.objectContaining({ title: reviewedEntryFixture.title, content: '修改后的标准答案。', riskLevel: 'low', requiresLiveData: true }), expect.objectContaining({ signal: expect.any(AbortSignal) })));
  });

  test('删除 FAQ 需要应用内确认且取消不会写入', /* deleteEntryTest 验证确认弹窗、取消和正式删除调用。 */ async () => {
    render(<KnowledgePreview />);
    // deleteButton 是条目异步加载后出现的显式删除入口。
    const deleteButton = await waitFor(/* deleteButtonAssertion 等待 FAQ 删除按钮显示。 */ () => screen.getByRole('button', { name: `删除FAQ：${reviewedEntryFixture.title}` }));
    fireEvent.click(deleteButton);
    expect(screen.getByRole('dialog', { name: '删除FAQ' })).toBeTruthy();
    expect(screen.getByText('当前页面没有回收站，此操作不能撤销。', { exact: false })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '取消' }));
    expect(deleteKnowledgeEntryMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: `删除FAQ：${reviewedEntryFixture.title}` }));
    fireEvent.click(screen.getByRole('button', { name: '确认删除FAQ' }));
    await waitFor(/* deleteCallAssertion 等待确认后调用正式删除 API。 */ () => expect(deleteKnowledgeEntryMock).toHaveBeenCalledWith('7', expect.objectContaining({ id: reviewedEntryFixture.id, type: 'faq' }), expect.objectContaining({ signal: expect.any(AbortSignal) })));
  });

  test('相似问法弹窗展示已保存标签并由用户确认系统建议', /* aliasDialogTest 验证建议不会自动落库。 */ async () => {
    render(<KnowledgePreview />);
    // manageButton 是 FAQ 异步加载后的相似问法入口。
    const manageButton = await waitFor(/* aliasButtonAssertion 等待相似问法按钮显示。 */ () => screen.getByRole('button', { name: `管理相似问法：${reviewedEntryFixture.title}` }));
    fireEvent.click(manageButton);
    await waitFor(/* aliasLoadedAssertion 等待已保存问法和系统建议显示。 */ () => expect(screen.getByText('怎么设置')).toBeTruthy());
    expect(screen.getByText('+ 如何设置')).toBeTruthy();
    expect(createKnowledgeFAQAliasMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '+ 如何设置' }));
    await waitFor(/* aliasCreateAssertion 等待用户点击后才保存系统建议。 */ () => expect(createKnowledgeFAQAliasMock).toHaveBeenCalledWith('7', '11', '如何设置', 'system', expect.objectContaining({ signal: expect.any(AbortSignal) })));
  });
});
