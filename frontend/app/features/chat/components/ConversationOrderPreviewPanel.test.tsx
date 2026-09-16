// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,within } from '@testing-library/react';
import { afterEach,describe,expect,test,vi } from 'vitest';
import type { ConversationOrder } from '../api';
import { useConversationOrders } from '../useConversationOrders';
import { ConversationOrderPreviewPanel } from './ConversationOrderPreviewPanel';

vi.mock('../useConversationOrders', /* conversationOrderHookMockFactory 隔离组件测试不访问真实订单上下文接口。 */ () => ({ useConversationOrders: vi.fn() }));

// useConversationOrdersMock 是真实订单上下文 Hook 的可控测试替身。
const useConversationOrdersMock = vi.mocked(useConversationOrders);
// setPageMock 记录组件提交的历史订单翻页动作。
const setPageMock = vi.fn();
// reloadMock 记录错误态和顶部刷新触发的重新读取动作。
const reloadMock = vi.fn();
// currentOrder 是明确属于当前会话的真实响应形状夹具。
const currentOrder = { id: 'order-current', order_id: 'order-current', cookie_id: 'account-1', chat_id: 'chat-1', item_id: 'item-1', item_title: '当前会话商品', item_image: '', buyer_id: 'buyer-1', quantity: 1, amount: '1375.00', status: 'completed', created_at: '2026-08-20T10:00:00Z', association: 'current_chat' } as ConversationOrder;
// historyOrder 是同账号同买家的历史订单夹具。
const historyOrder = { id: 'order-history', order_id: 'order-history', cookie_id: 'account-1', chat_id: 'chat-old', item_id: 'item-2', item_title: '历史商品', item_image: '', buyer_id: 'buyer-1', quantity: 1, amount: '748.00', status: 'shipped', created_at: '2026-07-10T10:00:00Z', association: 'same_buyer' } as ConversationOrder;
// completedHistoryOrder 是正常完成且应进入其他历史订单组的夹具。
const completedHistoryOrder = { ...historyOrder, id: 'order-completed', order_id: 'order-completed', item_title: '正常完成商品', status: 'completed', created_at: '2026-08-20T10:00:00Z' } as ConversationOrder;
// refundedHistoryOrder 是已结束退款且应排在普通完成订单之前的夹具。
const refundedHistoryOrder = { ...historyOrder, id: 'order-refunded', order_id: 'order-refunded', item_title: '历史退款商品', status: 'refunded', created_at: '2026-08-21T10:00:00Z' } as ConversationOrder;
// refundingHistoryOrder 是仍在处理且必须排在已退款之前的夹具。
const refundingHistoryOrder = { ...historyOrder, id: 'order-refunding', order_id: 'order-refunding', item_title: '退款处理中商品', status: 'refunding', created_at: '2026-08-19T10:00:00Z' } as ConversationOrder;
// cancelledHistoryOrder 是已取消且只在全部或已取消筛选中展示的夹具。
const cancelledHistoryOrder = { ...historyOrder, id: 'order-cancelled', order_id: 'order-cancelled', item_title: '已取消商品', status: 'cancelled', created_at: '2026-08-22T10:00:00Z' } as ConversationOrder;

describe('ConversationOrderPreviewPanel', /* conversationPanelSuite 验证当前会话订单和同买家历史订单的两段式布局。 */ () => {
  afterEach(/* cleanupConversationPanel 清理组件 DOM 和调用记录，避免用例互相污染。 */ () => {
    cleanup();
    vi.clearAllMocks();
  });

  test('分区展示当前订单和同买家历史且移除无价值跳转', /* groupConversationOrders 验证新信息结构和已删除入口。 */ () => {
    useConversationOrdersMock.mockReturnValue({ orders: [currentOrder, historyOrder], summary: { total: 2, current_chat: 1, completed: 1 }, total: 2, page: 1, totalPages: 1, status: 'all', loading: false, error: '', truncated: false, setStatus: vi.fn(), setPage: setPageMock, reload: reloadMock });
    render(<ConversationOrderPreviewPanel open onClose={vi.fn()} accountID="account-1" chatID="chat-1" buyerName="测试买家" buyerID="buyer-1" revision="message-1" />);

    expect(screen.getByRole('complementary', { name: '订单上下文' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: '当前会话订单' })).toBeTruthy();
    expect(screen.getByRole('heading', { name: '同买家历史订单' })).toBeTruthy();
    expect(screen.getByText('当前关联')).toBeTruthy();
    expect(screen.getByText('当前会话商品')).toBeTruthy();
    expect(screen.getByText('历史商品')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /订单详情/ })).toBeNull();
    expect(screen.queryByRole('button', { name: '查看订单' })).toBeNull();
    expect(screen.queryByText(/关联依据/)).toBeNull();
  });

  test('没有当前订单时保留人工判断提示和同买家历史', /* preserveHistoryWithoutCurrentOrder 验证未关联状态不会隐藏历史订单。 */ () => {
    useConversationOrdersMock.mockReturnValue({ orders: [historyOrder], summary: { total: 1, current_chat: 0, completed: 0 }, total: 1, page: 1, totalPages: 1, status: 'all', loading: false, error: '', truncated: false, setStatus: vi.fn(), setPage: setPageMock, reload: reloadMock });
    render(<ConversationOrderPreviewPanel open onClose={vi.fn()} accountID="account-1" chatID="chat-1" buyerName="测试买家" buyerID="buyer-1" revision="message-1" />);

    expect(screen.getByText('暂未识别本次聊天对应的订单')).toBeTruthy();
    expect(screen.getByText('历史商品')).toBeTruthy();
    expect(screen.getByText('仅展示本地已同步记录')).toBeTruthy();
  });

  test('零金额临时订单显示同步状态而不是零元', /* pendingAmountPresentation 验证补全窗口不用虚假金额误导客服。 */ () => {
    // incompleteOrder 是交易卡片刚落库但详情尚未返回的当前会话订单。
    const incompleteOrder: ConversationOrder = { ...currentOrder, amount: '' };
    useConversationOrdersMock.mockReturnValue({ orders: [incompleteOrder], summary: { total: 1, current_chat: 1, completed: 0 }, total: 1, page: 1, totalPages: 1, status: 'all', loading: false, error: '', truncated: false, setStatus: vi.fn(), setPage: setPageMock, reload: reloadMock });
    render(<ConversationOrderPreviewPanel open onClose={vi.fn()} accountID="account-1" chatID="chat-1" buyerName="测试买家" buyerID="buyer-1" revision="pending-card" enrichmentOrderID="order-current" />);
    expect(screen.getByText('金额同步中')).toBeTruthy();
    expect(screen.queryByText('¥0.00')).toBeNull();
  });

  test('默认展示全部并把退款售后分组前置', /* prioritizeRefundHistory 验证警示排序和四种人工筛选。 */ () => {
    useConversationOrdersMock.mockReturnValue({ orders: [completedHistoryOrder, refundedHistoryOrder, refundingHistoryOrder, cancelledHistoryOrder], summary: { total: 4, current_chat: 0, completed: 1 }, total: 4, page: 1, totalPages: 1, status: 'all', loading: false, error: '', truncated: false, setStatus: vi.fn(), setPage: setPageMock, reload: reloadMock });
    render(<ConversationOrderPreviewPanel open onClose={vi.fn()} accountID="account-1" chatID="chat-1" buyerName="测试买家" buyerID="buyer-1" revision="message-1" />);

    // allFilter 是首次进入时默认选中的全部历史订单按钮。
    const allFilter = screen.getByRole('button', { name: '全部' });
    expect(allFilter.getAttribute('aria-pressed')).toBe('true');
    // attentionSection 是退款中和已退款订单共同使用的客观警示区域。
    const attentionSection = screen.getByRole('region', { name: '需注意的退款售后记录' });
    // attentionCards 按视觉顺序保存警示区内的退款订单卡片。
    const attentionCards = Array.from(attentionSection.querySelectorAll('article'));
    expect(attentionCards[0]?.textContent).toContain('退款处理中商品');
    expect(attentionCards[1]?.textContent).toContain('历史退款商品');
    expect(screen.getByRole('region', { name: '其他历史订单' })).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: '退款／售后' }));
    expect(screen.getByText('退款处理中商品')).toBeTruthy();
    expect(screen.getByText('历史退款商品')).toBeTruthy();
    expect(screen.queryByText('正常完成商品')).toBeNull();
    expect(screen.queryByText('已取消商品')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '已完成' }));
    expect(screen.getByText('正常完成商品')).toBeTruthy();
    expect(screen.queryByText('历史退款商品')).toBeNull();
    expect(screen.queryByRole('region', { name: '需注意的退款售后记录' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: '已取消' }));
    expect(screen.getByText('已取消商品')).toBeTruthy();
    expect(screen.queryByText('正常完成商品')).toBeNull();

    fireEvent.click(allFilter);
    expect(within(screen.getByRole('region', { name: '需注意的退款售后记录' })).getByText('历史退款记录')).toBeTruthy();
  });

  test('错误态允许原地重载且分页不离开聊天页', /* recoverAndPaginateConversationOrders 验证侧栏内的恢复与分页交互。 */ () => {
    useConversationOrdersMock.mockReturnValue({ orders: [], summary: { total: 0, current_chat: 0, completed: 0 }, total: 0, page: 1, totalPages: 2, status: 'all', loading: false, error: '查询历史订单失败', truncated: false, setStatus: vi.fn(), setPage: setPageMock, reload: reloadMock });
    render(<ConversationOrderPreviewPanel open onClose={vi.fn()} accountID="account-1" chatID="chat-1" buyerName="测试买家" buyerID="buyer-1" revision="message-1" />);

    fireEvent.click(screen.getByRole('button', { name: '重新加载' }));
    fireEvent.click(screen.getByRole('button', { name: '下一页订单上下文' }));
    expect(reloadMock).toHaveBeenCalledTimes(1);
    expect(setPageMock).toHaveBeenCalledWith(2);
  });
});
