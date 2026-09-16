// @vitest-environment jsdom
import { act,renderHook,waitFor } from '@testing-library/react';
import { beforeEach,describe,expect,test,vi } from 'vitest';
import { getConversationOrderContext,syncConversationOrder,type ConversationOrderContextResponse } from './api';
import { useConversationOrders } from './useConversationOrders';

vi.mock('./api', /* conversationOrderApiMockFactory 提供可取消的上下文请求和单笔详情补全替身。 */ () => ({ getConversationOrderContext: vi.fn(), syncConversationOrder: vi.fn() }));

// getConversationOrderContextMock 是真实历史订单 API 的可控替身。
const getConversationOrderContextMock = vi.mocked(getConversationOrderContext);
// syncConversationOrderMock 是 Chat 专用单笔订单详情补全替身。
const syncConversationOrderMock = vi.mocked(syncConversationOrder);
// firstResponse 是第一个会话的历史订单响应。
const firstResponse: ConversationOrderContextResponse = { summary: { total: 1, current_chat: 1, completed: 1 }, orders: [{ id: 'old-order', order_id: 'old-order', item_id: 'item-1', quantity: 1, amount: '10', status: 'completed', association: 'current_chat' }], total: 1, page: 1, page_size: 20, total_pages: 1, truncated: false };
// secondResponse 是切换后新会话的历史订单响应。
const secondResponse: ConversationOrderContextResponse = { summary: { total: 1, current_chat: 0, completed: 0 }, orders: [{ id: 'new-order', order_id: 'new-order', item_id: 'item-2', quantity: 1, amount: '20', status: 'shipped', association: 'same_buyer' }], total: 1, page: 1, page_size: 20, total_pages: 1, truncated: false };

describe('useConversationOrders', /* conversationOrderHookSuite 验证真实历史订单请求、筛选和旧响应隔离。 */ () => {
  beforeEach(/* resetConversationOrderMocks 清理每个用例的 API 调用。 */ () => {
    vi.clearAllMocks();
    syncConversationOrderMock.mockResolvedValue({ success: true, message: '订单刷新完成', order: { quantity: '1', spec_name: '套餐', spec_value: '五倍', order_status: 'processing', amount: '135.00' } });
  });

  test('加载当前上下文并在状态切换时回到第一页', /* loadAndFilterConversationOrders 验证正常请求和筛选行为。 */ async () => {
    getConversationOrderContextMock.mockResolvedValue(firstResponse);
    // hook 是当前会话历史订单 Hook。
    const hook = renderHook(
      // renderConversationOrders 使用稳定会话参数渲染 Hook。
      () => useConversationOrders({ accountID: 'account-1', chatID: 'chat-1', buyerID: 'buyer-1', revision: '' }),
    );
    await waitFor(/* loadedAssertion 等待真实订单写入状态。 */ () => expect(hook.result.current.orders[0]?.order_id).toBe('old-order'));
    await act(/* changeStatusAction 切换到已发货筛选。 */ async () => hook.result.current.setStatus('shipped'));
    await waitFor(/* filterRequestAssertion 等待新筛选请求。 */ () => expect(getConversationOrderContextMock).toHaveBeenLastCalledWith('account-1', 'chat-1', 'buyer-1', 'shipped', 1, expect.objectContaining({ signal: expect.any(AbortSignal) })));
  });

  test('会话切换后丢弃忽略取消信号的旧响应', /* rejectStaleConversationOrders 验证旧买家订单不会覆盖新会话。 */ async () => {
    // resolveOld 保存旧会话请求的完成函数，模拟底层忽略 AbortSignal。
    let resolveOld: (value: ConversationOrderContextResponse) => void = () => undefined;
    // oldRequest 是保持未完成的旧会话请求。
    const oldRequest = new Promise<ConversationOrderContextResponse>(/* oldRequestExecutor 暴露旧请求完成函数。 */ resolve => { resolveOld = resolve; });
    getConversationOrderContextMock.mockReturnValueOnce(oldRequest).mockResolvedValue(secondResponse);
    // hook 是可切换账号、会话和买家的历史订单 Hook。
    const hook = renderHook(
      // renderConversationOrders 根据当前测试 props 渲染 Hook。
      ({ accountID, chatID, buyerID }) => useConversationOrders({ accountID, chatID, buyerID, revision: '' }),
      { initialProps: { accountID: 'account-1', chatID: 'chat-1', buyerID: 'buyer-1' } },
    );
    hook.rerender({ accountID: 'account-2', chatID: 'chat-2', buyerID: 'buyer-2' });
    await waitFor(/* newConversationAssertion 等待新会话结果。 */ () => expect(hook.result.current.orders[0]?.order_id).toBe('new-order'));
    resolveOld(firstResponse);
    await act(/* resolveStaleAction 等待旧请求完成。 */ async () => { await oldRequest; });
    expect(hook.result.current.orders[0]?.order_id).toBe('new-order');
  });

  test('最新交易卡片的零金额订单自动单笔补全并重读右栏', /* enrichCurrentConversationOrder 验证 Chat 不需要跳转订单页执行整店同步。 */ async () => {
    // incompleteResponse 是待付款卡片刚落库时的零金额临时投影。
    const incompleteResponse: ConversationOrderContextResponse = { summary: { total: 1, current_chat: 1, completed: 0 }, orders: [{ id: 'pending-order', order_id: 'pending-order', item_id: 'item-1', quantity: 1, amount: '', status: 'processing', association: 'current_chat' }], total: 1, page: 1, page_size: 20, total_pages: 1, truncated: false };
    // enrichedResponse 是单笔详情写回后带真实实付金额的投影。
    const enrichedResponse: ConversationOrderContextResponse = { ...incompleteResponse, orders: [{ ...incompleteResponse.orders[0], amount: '135.00' }] };
    getConversationOrderContextMock.mockResolvedValueOnce(incompleteResponse).mockResolvedValue(enrichedResponse);
    // hook 是携带最新待付款卡片订单号的会话订单状态。
    const hook = renderHook(
      // renderEnrichedConversationOrder 模拟 Chat 收到一条新交易卡片。
      () => useConversationOrders({ accountID: 'account-1', chatID: 'chat-1', buyerID: 'buyer-1', revision: 'pending-card.PNM', orderID: 'pending-order' }),
    );
    await waitFor(/* singleOrderRefreshAssertion 等待短暂落库窗口后的单笔详情请求。 */ () => expect(syncConversationOrderMock).toHaveBeenCalledWith('pending-order', expect.objectContaining({ signal: expect.any(AbortSignal), timeoutMs: 60_000 })), { timeout: 2_000 });
    await waitFor(/* enrichedAmountAssertion 等待补全成功后重读到真实金额。 */ () => expect(hook.result.current.orders[0]?.amount).toBe('135.00'));
    expect(syncConversationOrderMock).toHaveBeenCalledTimes(1);
  });

  test('平台详情暂时失败后手动刷新可重试同一订单', /* retryFailedConversationEnrichment 验证一次失败不会永久锁死当前卡片。 */ async () => {
    // incompleteResponse 是平台详情未就绪时持续保留的本地临时投影。
    const incompleteResponse: ConversationOrderContextResponse = { summary: { total: 1, current_chat: 1, completed: 0 }, orders: [{ id: 'pending-order', order_id: 'pending-order', item_id: 'item-1', quantity: 1, amount: '', status: 'processing', association: 'current_chat' }], total: 1, page: 1, page_size: 20, total_pages: 1, truncated: false };
    getConversationOrderContextMock.mockResolvedValue(incompleteResponse);
    syncConversationOrderMock.mockRejectedValue(new Error('平台详情暂未就绪'));
    // hook 是需要手动刷新释放失败标记的会话订单状态。
    const hook = renderHook(
      // renderRetryableConversationOrder 保持同一待付款卡片和订单号。
      () => useConversationOrders({ accountID: 'account-1', chatID: 'chat-1', buyerID: 'buyer-1', revision: 'pending-card.PNM', orderID: 'pending-order' }),
    );
    await waitFor(/* firstFailureAssertion 等待第一轮自动补全失败。 */ () => expect(syncConversationOrderMock).toHaveBeenCalledTimes(1), { timeout: 2_000 });
    act(/* manualReloadAction 模拟用户点击右栏刷新按钮。 */ () => hook.result.current.reload());
    await waitFor(/* retryAssertion 等待同一卡片发起第二轮单笔详情请求。 */ () => expect(syncConversationOrderMock).toHaveBeenCalledTimes(2), { timeout: 2_000 });
  });
});
