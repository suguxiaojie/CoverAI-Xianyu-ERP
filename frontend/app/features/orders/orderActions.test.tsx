// @vitest-environment jsdom
import { act,renderHook } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import type { Order,OrderRefreshResponse } from '../../../shared/api-contract/orders';
import { useOrderActions } from './orderActions';

// RedFlowerResponseFixture 描述可控求花 Promise 的测试结果。
interface RedFlowerResponseFixture {
  /** success 表示平台明确接受求花。 */
  success: boolean;
  /** status 是平台动作的稳定终态。 */
  status: string;
  /** message 是测试用户提示。 */
  message: string;
}

// orderActionMocks 保存订单动作 Hook 测试使用的 API 替身。
const orderActionMocks = vi.hoisted(/* orderActionMockFactory 创建订单动作 API 替身。 */ () => ({
  deleteOrder: vi.fn(),
  getRedFlowerStatus: vi.fn(),
  manualShipOrder: vi.fn(),
  requestRedFlower: vi.fn(),
  syncOrders: vi.fn(),
  syncSingleOrder: vi.fn(),
  updateOrder: vi.fn(),
}));

vi.mock('./api', /* orderApiMockFactory 提供订单动作 API 替身。 */ () => ({
  deleteOrder: orderActionMocks.deleteOrder,
  getRedFlowerStatus: orderActionMocks.getRedFlowerStatus,
  manualShipOrder: orderActionMocks.manualShipOrder,
  requestRedFlower: orderActionMocks.requestRedFlower,
  syncOrders: orderActionMocks.syncOrders,
  syncSingleOrder: orderActionMocks.syncSingleOrder,
  updateOrder: orderActionMocks.updateOrder,
}));

// orderFixture 表示订单动作 Hook 使用的最小订单。
const orderFixture: Order = {
  id: 'row-1',
  order_id: 'order-1',
  cookie_id: 'account-1',
  item_id: 'item-1',
  item_title: '测试商品',
  buyer_id: 'buyer-1',
  quantity: 1,
  amount: '10.00',
  status: 'pending_ship',
};

// createActionHook 创建订单动作 Hook 并注入页面依赖替身。
const createActionHook = () => {
  // loadOrders 是订单动作完成后的列表刷新替身。
  const loadOrders = vi.fn().mockResolvedValue(undefined);
  // setPage 是订单删除后分页调整替身。
  const setPage = vi.fn();
  // hook 是订单动作 Hook 的渲染结果。
  const hook = renderHook(/* hookFactory 创建订单动作 Hook。 */ () => useOrderActions({
    orders: [orderFixture],
    page: 2,
    accountFilter: 'account-1',
    filter: 'pending_ship',
    setPage,
    loadOrders,
  }));
  return { hook, loadOrders, setPage };
};

describe('useOrderActions 订单动作协调器', /* 当前回调验证订单动作成功、失败、取消和弹窗清理边界。 */ () => {
  beforeEach(/* 当前回调重置订单 API 和浏览器提示替身。 */ () => {
    vi.clearAllMocks();
    orderActionMocks.syncOrders.mockResolvedValue({ message: '同步完成' });
    orderActionMocks.manualShipOrder.mockResolvedValue({ results: [{ success: true, message: '发货成功' }] });
    orderActionMocks.requestRedFlower.mockResolvedValue({ success: true, status: 'succeeded', message: '求花成功' });
    orderActionMocks.syncSingleOrder.mockResolvedValue({ success: true, message: '同步完成' });
    orderActionMocks.updateOrder.mockResolvedValue({ success: true });
    orderActionMocks.deleteOrder.mockResolvedValue({ success: true });
    orderActionMocks.getRedFlowerStatus.mockResolvedValue({ success: true, status: 'not_requested', message: '' });
    vi.spyOn(window, 'alert').mockImplementation(/* alertImplementation 屏蔽订单动作提示。 */ () => undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
  });

  afterEach(/* 当前回调恢复浏览器提示替身。 */ () => {
    vi.restoreAllMocks();
  });

  test('批量同步错误进入进度卡，单笔同步失败继续使用即时提示', /* 当前回调验证两类订单同步异常分支。 */ async () => {
    orderActionMocks.syncOrders.mockRejectedValueOnce(new Error('批量同步失败'));
    // actionContext 保存订单动作 Hook 和刷新依赖。
    const { hook } = createActionHook();
    await act(/* syncAction 执行失败的订单批量同步。 */ async () => hook.result.current.handleSync());
    expect(hook.result.current.syncError).toBe('批量同步失败');
    expect(window.alert).not.toHaveBeenCalledWith('批量同步失败');

    orderActionMocks.syncSingleOrder.mockResolvedValueOnce({ success: false, message: '单笔同步失败' });
    await act(/* singleSyncAction 执行失败的单笔同步。 */ async () => hook.result.current.handleSyncSingle('order-1'));
    expect(window.alert).toHaveBeenCalledWith('单笔同步失败');
    expect(hook.result.current.syncingOrderId).toBeNull();
  });

  test('运行中重复点击不会创建第二个批量同步任务', /* 当前回调验证按钮和 Hook 双重防重边界。 */ async () => {
    // resolveFirst 保存首次同步 API 适配器的延迟结果完成函数。
    let resolveFirst: ((value: OrderRefreshResponse) => void) | undefined;
    // firstCompletion 保存首次同步尚未完成的 API 适配器 Promise。
    const firstCompletion = new Promise<OrderRefreshResponse>(/* resolve 保存首次同步的结果完成器。 */ resolve => { resolveFirst = resolve; });
    orderActionMocks.syncOrders.mockImplementationOnce(/* firstSync 模拟订单任务仍在运行。 */ () => firstCompletion);
    // actionContext 保存待验证防重逻辑的订单动作 Hook。
    const { hook, loadOrders } = createActionHook();

    // firstTask 启动首个同步但不等待它完成。
    const firstTask = hook.result.current.handleSync();
    // secondTask 模拟运行中再次点击；它必须立即返回且不创建第二个后端任务。
    const secondTask = hook.result.current.handleSync();
    await act(/* completeSecond 等待被忽略的重复调用结束。 */ async () => { await secondTask; });
    expect(orderActionMocks.syncOrders).toHaveBeenCalledTimes(1);
    expect(loadOrders).not.toHaveBeenCalled();

    resolveFirst?.({ partial_failure: false, message: '同步完成', summary: { discovered: 0, list_updated: 0, soft_deleted: 0, detail_total: 0, total: 0, updated: 0, no_change: 0, failed: 0 }, results: [] });
    await act(/* completeFirst 放行唯一任务并刷新列表。 */ async () => { await firstTask; });
    expect(loadOrders).toHaveBeenCalledTimes(1);
    expect(window.alert).not.toHaveBeenCalled();
  });

  test('服务端轮询快照实时更新订单同步进度', /* 当前回调验证运行和成功终态会进入进度卡状态。 */ async () => {
    orderActionMocks.syncOrders.mockImplementationOnce(/* progressSync 依次发布运行和成功状态。 */ async (_cookieID, _status, options) => {
      options?.onProgress?.({ success: true, job_id: 'job-1', status: 'running', progress: { stage: 'syncing_details', message: '正在逐单同步订单详情', processed: 3, total: 10, succeeded: 2, failed: 1, percent: 49 } });
      options?.onProgress?.({ success: true, job_id: 'job-1', status: 'succeeded', progress: { stage: 'completed', message: '订单同步完成', processed: 10, total: 10, succeeded: 9, failed: 1, percent: 100 } });
      return { partial_failure: true, message: '1 个订单失败', summary: { discovered: 0, list_updated: 0, soft_deleted: 0, detail_total: 10, total: 10, updated: 4, no_change: 5, failed: 1 }, results: [] };
    });
    // actionContext 保存进度更新后的 Hook 和列表刷新替身。
    const { hook, loadOrders } = createActionHook();
    await act(/* progressAction 执行带实时进度的订单同步。 */ async () => hook.result.current.handleSync());
    expect(hook.result.current.syncJob?.status).toBe('succeeded');
    expect(hook.result.current.syncJob?.progress?.percent).toBe(100);
    expect(hook.result.current.syncError).toBe('1 个订单失败');
    expect(loadOrders).toHaveBeenCalledTimes(1);
  });

  test('发货结果失败和异常均保留错误结果', /* 当前回调验证订单发货异常分支。 */ async () => {
    // actionContext 保存发货动作 Hook 和刷新依赖。
    const { hook } = createActionHook();
    act(/* openShipAction 打开发货弹窗。 */ () => hook.result.current.handleShip('order-1'));
    orderActionMocks.manualShipOrder.mockResolvedValueOnce({ results: [{ success: false, message: '卡券不足' }] });
    await act(/* failedShipAction 执行返回失败结果的发货。 */ async () => hook.result.current.executeShip('full_delivery'));
    expect(hook.result.current.shipResult).toEqual({ success: false, message: '卡券不足' });

    orderActionMocks.manualShipOrder.mockRejectedValueOnce(new Error('发货网络失败'));
    await act(/* errorShipAction 执行抛出异常的发货。 */ async () => hook.result.current.executeShip('status_only'));
    expect(hook.result.current.shipResult).toEqual({ success: false, message: '发货网络失败' });
  });

  test('手动求花要求二次确认并同步阻止连续点击', /* 当前回调验证真实外部动作的确认和防重边界。 */ async () => {
    // resolveRequest 保存唯一求花请求的延迟完成函数。
    let resolveRequest: ((value: RedFlowerResponseFixture) => void) | undefined;
    // requestCompletion 保存尚未完成的平台动作响应。
    const requestCompletion = new Promise<RedFlowerResponseFixture>(/* requestPromise 创建可控求花请求。 */ resolve => { resolveRequest = resolve; });
    orderActionMocks.requestRedFlower.mockImplementationOnce(/* pendingFlowerRequest 保持首次请求运行中。 */ () => requestCompletion);
    // actionContext 保存求花动作 Hook。
    const { hook } = createActionHook();
    vi.mocked(window.confirm).mockReturnValueOnce(false);
    await act(/* cancelFlowerAction 取消首次确认。 */ async () => hook.result.current.handleRequestRedFlower('order-1'));
    expect(orderActionMocks.requestRedFlower).not.toHaveBeenCalled();

    vi.mocked(window.confirm).mockReturnValue(true);
    // firstRequest 启动唯一平台求花请求。
    const firstRequest = hook.result.current.handleRequestRedFlower('order-1');
    // duplicateRequest 在 React 状态提交前立即模拟第二次点击。
    const duplicateRequest = hook.result.current.handleRequestRedFlower('order-1');
    await act(/* duplicateCompletion 等待被同步防重忽略的请求结束。 */ async () => { await duplicateRequest; });
    expect(orderActionMocks.requestRedFlower).toHaveBeenCalledTimes(1);
    resolveRequest?.({ success: true, status: 'succeeded', message: '求花成功' });
    await act(/* firstCompletion 完成唯一求花请求。 */ async () => { await firstRequest; });
    expect(hook.result.current.redFlowerResult).toEqual({ orderId: 'order-1', success: true, message: '求花成功' });
    expect(hook.result.current.redFlowerLoadingOrderId).toBeNull();
  });

  test('重新打开订单详情会恢复成功状态并丢弃切换订单后的旧响应', /* 当前回调验证持久状态和迟到响应隔离。 */ async () => {
    // resolveOldStatus 保存首个订单状态查询的延迟完成函数。
    let resolveOldStatus: ((value: RedFlowerResponseFixture) => void) | undefined;
    // oldStatusCompletion 保存首个订单尚未完成的状态查询。
    const oldStatusCompletion = new Promise<RedFlowerResponseFixture>(/* oldStatusPromise 创建可控旧订单状态响应。 */ resolve => { resolveOldStatus = resolve; });
    orderActionMocks.getRedFlowerStatus
      .mockImplementationOnce(/* delayedOldStatus 模拟首个订单查询迟到。 */ () => oldStatusCompletion)
      .mockResolvedValueOnce({ success: true, status: 'succeeded', message: '已向买家求花，系统卡片同步可能延迟', requested_at: 123 });
    // actionContext 保存待验证订单详情 Hook。
    const { hook } = createActionHook();
    // secondOrder 是切换后的另一笔订单。
    const secondOrder = { ...orderFixture, id: 'row-2', order_id: 'order-2' };
    act(/* openOldOrder 打开首个订单并开始迟到查询。 */ () => hook.result.current.handleViewDetail(orderFixture));
    act(/* switchOrder 在首个查询完成前切换订单。 */ () => hook.result.current.handleViewDetail(secondOrder));
    await act(/* waitNewStatus 等待第二笔订单持久状态恢复。 */ async () => { await Promise.resolve(); });
    expect(hook.result.current.redFlowerStatus).toEqual({ orderId: 'order-2', status: 'succeeded', message: '已向买家求花，系统卡片同步可能延迟', requestedAt: 123 });
    resolveOldStatus?.({ success: true, status: 'not_requested', message: '' });
    await act(/* finishOldStatus 放行迟到的首个订单响应。 */ async () => { await oldStatusCompletion; });
    expect(hook.result.current.redFlowerStatus?.orderId).toBe('order-2');
  });

  test('编辑草稿使用函数式补丁并在保存失败时保留弹窗', /* 当前回调验证订单编辑草稿和失败收束。 */ async () => {
    // actionContext 保存订单编辑动作 Hook 和刷新依赖。
    const { hook } = createActionHook();
    act(/* openEditAction 打开订单编辑弹窗。 */ () => hook.result.current.handleEdit(orderFixture));
    act(/* patchAction 更新订单编辑草稿。 */ () => hook.result.current.updateEditingOrder({ buyer_id: 'buyer-2' }));
    expect(hook.result.current.editingOrder?.buyer_id).toBe('buyer-2');

    orderActionMocks.updateOrder.mockRejectedValueOnce(new Error('编辑失败'));
    await act(/* saveEditAction 执行失败的订单编辑保存。 */ async () => hook.result.current.handleSaveEdit());
    expect(window.alert).toHaveBeenCalledWith('更新失败，请重试');
    expect(hook.result.current.showEditModal).toBe(true);
  });

  test('删除确认取消不调用 API，删除异常会刷新列表', /* 当前回调验证订单删除取消和异常分支。 */ async () => {
    // actionContext 保存订单删除动作 Hook、刷新函数和分页 Setter。
    const { hook, loadOrders, setPage } = createActionHook();
    vi.mocked(window.confirm).mockReturnValueOnce(false);
    await act(/* cancelDeleteAction 取消订单删除确认。 */ async () => hook.result.current.handleDelete('order-1'));
    expect(orderActionMocks.deleteOrder).not.toHaveBeenCalled();

    orderActionMocks.deleteOrder.mockRejectedValueOnce(new Error('删除失败'));
    await act(/* errorDeleteAction 执行抛出异常的订单删除。 */ async () => hook.result.current.handleDelete('order-1'));
    expect(window.alert).toHaveBeenCalledWith('删除失败');
    expect(loadOrders).toHaveBeenCalledTimes(1);
    expect(setPage).not.toHaveBeenCalled();
  });

  test('关闭三个弹窗会清理对应状态', /* 当前回调验证订单弹窗状态收束。 */ () => {
    // actionContext 保存订单弹窗动作 Hook 和刷新依赖。
    const { hook } = createActionHook();
    act(/* detailAction 打开订单详情弹窗。 */ () => hook.result.current.handleViewDetail(orderFixture));
    act(/* editAction 打开订单编辑弹窗。 */ () => hook.result.current.handleEdit(orderFixture));
    act(/* shipAction 打开发货弹窗。 */ () => hook.result.current.handleShip('order-1'));
    act(/* closeAction 关闭订单详情、编辑和发货弹窗。 */ () => {
      hook.result.current.closeDetailModal();
      hook.result.current.closeEditModal();
      hook.result.current.closeShipModal();
    });
    expect(hook.result.current.showDetailModal).toBe(false);
    expect(hook.result.current.showEditModal).toBe(false);
    expect(hook.result.current.showShipModal).toBe(false);
    expect(hook.result.current.shipResult).toBeNull();
  });
});
