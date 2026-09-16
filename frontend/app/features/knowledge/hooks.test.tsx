// @vitest-environment jsdom
import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, test, vi } from 'vitest';
import type { KnowledgeBasePreview } from './types';
import { useKnowledge } from './hooks';
import { getKnowledgeBases, getKnowledgeEntries, getKnowledgeScopeOptions } from './api';

vi.mock('./api', /* knowledgeHookApiMockFactory 隔离 Hook 请求代次测试与真实 HTTP 服务。 */ () => ({
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

// getKnowledgeBasesMock 是可手动控制完成顺序的列表请求替身。
const getKnowledgeBasesMock = vi.mocked(getKnowledgeBases);
// getKnowledgeEntriesMock 是选中知识库条目读取替身。
const getKnowledgeEntriesMock = vi.mocked(getKnowledgeEntries);
// getKnowledgeScopeOptionsMock 是 Hook 列表读取共用的店铺／商品范围替身。
const getKnowledgeScopeOptionsMock = vi.mocked(getKnowledgeScopeOptions);

/** knowledgeBaseFixture 生成指定标识和名称的最小知识库 feature 模型。 */
const knowledgeBaseFixture = (id: string, name: string): KnowledgeBasePreview => ({ id, name, description: '权威说明', status: 'active', scopeLabel: '全部店铺 · 用户通用', accountIds: [], itemScopes: [], entryCount: 0, reviewedCount: 0, accent: 'brand', createdAt: '2026-08-21T10:00:00Z', updatedAt: '2026-08-21T10:00:00Z' });

describe('useKnowledge request lifecycle', /* knowledgeHookSuite 验证刷新、晚到响应和卸载取消边界。 */ () => {
  afterEach(/* knowledgeHookReset 清理每个 Hook 测试的可控请求替身。 */ () => vi.clearAllMocks());

  test('新列表请求完成后拒绝旧响应覆盖', /* staleResponseTest 验证代次隔离。 */ async () => {
    getKnowledgeScopeOptionsMock.mockResolvedValue([{ value: 'all', label: '全部店铺 · 用户通用', accountIds: [], itemScopes: [] }]);
    // resolveFirst 完成首个已被刷新取代的列表请求。
    let resolveFirst: (value: KnowledgeBasePreview[]) => void = () => undefined;
    // resolveSecond 完成用户刷新后的最新列表请求。
    let resolveSecond: (value: KnowledgeBasePreview[]) => void = () => undefined;
    getKnowledgeBasesMock
      .mockImplementationOnce(/* firstRequest 保持首个请求在途直到测试手动完成。 */ () => new Promise<KnowledgeBasePreview[]>(/* firstResolverCapture 捕获首个请求的完成函数。 */ resolve /* resolve 保存首个请求的完成函数。 */ => { resolveFirst = resolve; }))
      .mockImplementationOnce(/* secondRequest 保持第二个请求在途直到测试手动完成。 */ () => new Promise<KnowledgeBasePreview[]>(/* secondResolverCapture 捕获第二个请求的完成函数。 */ resolve /* resolve 保存第二个请求的完成函数。 */ => { resolveSecond = resolve; }));
    getKnowledgeEntriesMock.mockResolvedValue([]);
    // hook 是待验证的知识库 Hook 渲染结果。
    const hook = renderHook(/* knowledgeHookRenderer 挂载知识库 Hook。 */ () => useKnowledge());
    act(/* reloadAction 在首个请求完成前发起刷新。 */ () => hook.result.current.reload());
    await act(/* secondResolution 先完成最新列表请求。 */ async () => resolveSecond([knowledgeBaseFixture('2', '新知识')]));
    await waitFor(/* newestAssertion 等待最新列表写入 Hook 状态。 */ () => expect(hook.result.current.knowledgeBases[0]?.name).toBe('新知识'));
    await act(/* firstResolution 后完成已被取代的旧列表请求。 */ async () => resolveFirst([knowledgeBaseFixture('1', '旧知识')]));
    expect(hook.result.current.knowledgeBases[0]?.name).toBe('新知识');
    hook.unmount();
  });

  test('页面卸载时取消尚未完成的列表请求', /* abortOnUnmountTest 验证请求清理。 */ () => {
    getKnowledgeScopeOptionsMock.mockResolvedValue([{ value: 'all', label: '全部店铺 · 用户通用', accountIds: [], itemScopes: [] }]);
    // capturedSignal 记录知识库列表 API 收到的取消信号。
    let capturedSignal: AbortSignal | undefined;
    getKnowledgeBasesMock.mockImplementation(/* pendingRequest 保持列表请求在途以验证卸载取消。 */ options => {
      capturedSignal = options?.signal;
      return new Promise<KnowledgeBasePreview[]>(() => undefined /* pendingPromise 故意不完成，只由取消信号收束。 */);
    });
    // hook 是将立即卸载的知识库 Hook 渲染结果。
    const hook = renderHook(/* knowledgeHookRenderer 挂载知识库 Hook。 */ () => useKnowledge());
    hook.unmount();
    expect(capturedSignal?.aborted).toBe(true);
  });
});
