// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor,within } from '@testing-library/react';
import React from 'react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import type { Order } from '../../../../shared/api-contract/orders';

// orderListMocks 保存订单页面测试使用的 Hook、API 和子组件替身。
const orderListMocks = vi.hoisted(/* orderListMockFactory 创建订单页面共享替身。 */ () => ({
  orders: [] as Order[],
  setFilter: vi.fn(),
  setAccountFilter: vi.fn(),
  setCreatedRange: vi.fn(),
  setAmountRange: vi.fn(),
  setSearchText: vi.fn(),
  setPage: vi.fn(),
  loadOrders: vi.fn(),
  openImportModal: vi.fn(),
  closeImportModal: vi.fn(),
  setImportFile: vi.fn(),
  handleImportOrders: vi.fn(),
  handleRetryImport: vi.fn(),
  syncOrders: vi.fn(),
  syncSingleOrder: vi.fn(),
  manualShipOrder: vi.fn(),
  requestRedFlower: vi.fn(),
  updateOrder: vi.fn(),
  deleteOrder: vi.fn(),
  getRedFlowerStatus: vi.fn(),
}));

vi.mock('../hooks', /* ordersHooksMockFactory 提供订单查询与导入 Hook 替身。 */ () => ({
  useOrderQuery: /* useOrderQueryMock 返回订单页面固定查询状态。 */ () => ({
    orders: orderListMocks.orders,
    accounts: [{ id: 'account-1', enabled: true, auto_confirm: false, nickname: 'Alpha', remark: '主账号' }],
    filter: 'all',
    setFilter: orderListMocks.setFilter,
    accountFilter: '',
    setAccountFilter: orderListMocks.setAccountFilter,
    createdRange: { createdFrom: '', createdTo: '', label: '全部时间' },
    setCreatedRange: orderListMocks.setCreatedRange,
    amountRange: { minAmount: '', maxAmount: '', label: '金额范围' },
    setAmountRange: orderListMocks.setAmountRange,
    searchText: '',
    setSearchText: orderListMocks.setSearchText,
		page: 2,
		setPage: orderListMocks.setPage,
		totalPages: 3,
		settlementSummary: { order_count: 3, gross_amount: '405.00', service_fee: '6.48', pending_amount: '398.52', service_fee_rate: '1.6%' },
    loading: false,
    loadOrders: orderListMocks.loadOrders,
    accountName: /* accountNameMock 返回订单筛选账号名称。 */ () => '主账号 · account',
    accountNickname: /* accountNicknameMock 返回订单行账号名称。 */ () => '主账号',
    getItemNameById: /* itemNameMock 返回订单商品名称。 */ (_cookieId: string, _itemId: string, orderItemTitle?: string) => orderItemTitle || '测试商品',
  }),
  useOrderImport: /* useOrderImportMock 返回订单导入弹窗状态。 */ () => ({
    showImportModal: false,
    importFile: null,
    setImportFile: orderListMocks.setImportFile,
    importResult: null,
    importing: false,
    importError: '',
    openImportModal: orderListMocks.openImportModal,
    closeImportModal: orderListMocks.closeImportModal,
    handleImportOrders: orderListMocks.handleImportOrders,
    handleRetryImport: orderListMocks.handleRetryImport,
  }),
}));

vi.mock('../api', /* ordersApiMockFactory 提供订单页面动作 API 替身。 */ () => ({
  syncOrders: orderListMocks.syncOrders,
  syncSingleOrder: orderListMocks.syncSingleOrder,
  manualShipOrder: orderListMocks.manualShipOrder,
  requestRedFlower: orderListMocks.requestRedFlower,
  updateOrder: orderListMocks.updateOrder,
  deleteOrder: orderListMocks.deleteOrder,
  getRedFlowerStatus: orderListMocks.getRedFlowerStatus,
}));

vi.mock('../components/OrderFilterBar', /* filterBarMockFactory 提供订单筛选栏替身。 */ () => {
  // OrderFilterBarMock 暴露筛选栏的状态切换和输入事件。
  const OrderFilterBarMock: React.FC<any> = (props /* props 表示筛选栏状态和事件回调。 */) => (
    <div data-testid="order-filter-bar">
      <button onClick={/* filterAction 切换待发货筛选。 */ () => props.onFilterChange('pending_ship')}>待发货筛选</button>
			<button onClick={/* timeFilterAction 应用订单创建时间范围。 */ () => props.onCreatedRangeChange({ createdFrom: '2026-08-20T00:00:00.000Z', createdTo: '2026-08-21T00:00:00.000Z', label: '今天' })}>今天下单</button>
			<button onClick={/* amountFilterAction 应用订单实付金额范围。 */ () => props.onAmountRangeChange({ minAmount: '135', maxAmount: '690', label: '¥135–¥690' })}>筛选金额</button>
      <select aria-label="按账号筛选订单" value={props.accountFilter} onChange={/* accountAction 切换账号筛选。 */ event => props.onAccountFilterChange(event.target.value)}>
        <option value="">全部账号</option>
        <option value="account-1">主账号</option>
      </select>
      <input placeholder="搜索订单号/商品/买家..." value={props.searchText} onChange={/* searchAction 修改订单搜索词。 */ event => props.onSearchChange(event.target.value)} />
    </div>
  );
  return { OrderFilterBar: OrderFilterBarMock };
});

vi.mock('../components/OrderImportModal', /* importModalMockFactory 提供订单导入弹窗替身。 */ () => ({
  OrderImportModal: /* OrderImportModalMock 表示订单导入弹窗替身。 */ (props: any) => props.showImportModal ? <div data-testid="import-modal">订单导入弹窗</div> : null,
}));

vi.mock('../components/ShipmentProofModal', /* shipmentProofModalMockFactory 暴露订单页凭证弹窗的打开状态。 */ () => ({
	ShipmentProofModal: /* ShipmentProofModalMock 只显示当前打开的凭证订单号。 */ (props: any) => props.open ? <div data-testid="shipment-proof-modal">凭证 {props.orderID}</div> : null,
}));

import OrderList from './OrderList';

// orderFixture 表示订单页面测试中的待发货订单。
const orderFixture: Order = {
  id: 'row-1',
  order_id: 'order-1',
  cookie_id: 'account-1',
  item_id: 'item-1',
	chat_id: 'chat-1',
  item_title: '测试商品',
  item_image: '',
  item_price: '10.00',
  buyer_id: 'buyer-1',
	buyer_name: '测试买家',
	buyer_avatar_url: 'https://img.example/buyer.png',
  quantity: 2,
  amount: '20.00',
  status: 'pending_ship',
	shipment_proof_available: true,
  receiver_name: '收货人',
  receiver_phone: '13800000000',
  receiver_address: '测试地址',
  created_at: '2026-08-15T10:00:00Z',
};

describe('OrderList 页面组合行为', /* 当前回调验证订单筛选、编辑、发货、同步、删除和分页流程。 */ () => {
  beforeEach(/* 当前回调重置订单页面 API、Hook 状态和浏览器提示替身。 */ () => {
    vi.clearAllMocks();
    orderListMocks.orders = [orderFixture];
    orderListMocks.loadOrders.mockResolvedValue(undefined);
    orderListMocks.syncOrders.mockResolvedValue({ success: true, message: '同步完成' });
    orderListMocks.syncSingleOrder.mockResolvedValue({ success: true, message: '订单已同步' });
    orderListMocks.manualShipOrder.mockResolvedValue({ results: [{ success: true, message: '发货成功' }] });
    orderListMocks.requestRedFlower.mockResolvedValue({ success: true, status: 'succeeded', message: '求花成功' });
    orderListMocks.updateOrder.mockResolvedValue({ success: true });
    orderListMocks.deleteOrder.mockResolvedValue({ success: true });
    orderListMocks.getRedFlowerStatus.mockResolvedValue({ success: true, status: 'not_requested', message: '' });
		Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
    vi.spyOn(window, 'alert').mockImplementation(/* alertImplementation 屏蔽订单页面提示。 */ () => undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
		vi.spyOn(window, 'prompt').mockReturnValue(null);
		window.history.replaceState({}, '', '/app/orders');
  });

  afterEach(/* 当前回调清理订单页面 DOM 和浏览器提示替身。 */ () => {
    cleanup();
    vi.restoreAllMocks();
  });

	test('筛选栏、同步和导入按钮转发页面操作', /* 当前回调验证订单页面顶部操作组合边界。 */ async () => {
    render(<OrderList />);
    fireEvent.click(screen.getByText('待发货筛选'));
    expect(orderListMocks.setFilter).toHaveBeenCalledWith('pending_ship');
    expect(orderListMocks.setPage).toHaveBeenCalledWith(1);
    expect(orderListMocks.setSearchText).toHaveBeenCalledWith('');

    fireEvent.change(screen.getByLabelText('按账号筛选订单'), { target: { value: 'account-1' } });
    expect(orderListMocks.setAccountFilter).toHaveBeenCalledWith('account-1');
		fireEvent.click(screen.getByText('今天下单'));
		expect(orderListMocks.setCreatedRange).toHaveBeenCalledWith({ createdFrom: '2026-08-20T00:00:00.000Z', createdTo: '2026-08-21T00:00:00.000Z', label: '今天' });
		expect(orderListMocks.setPage).toHaveBeenCalledWith(1);
		fireEvent.click(screen.getByText('筛选金额'));
		expect(orderListMocks.setAmountRange).toHaveBeenCalledWith({ minAmount: '135', maxAmount: '690', label: '¥135–¥690' });
		expect(orderListMocks.setPage).toHaveBeenCalledWith(1);
    fireEvent.change(screen.getByPlaceholderText('搜索订单号/商品/买家...'), { target: { value: 'order-1' } });
    expect(orderListMocks.setSearchText).toHaveBeenCalledWith('order-1');

    fireEvent.click(screen.getByText('增量同步订单'));
    await waitFor(/* syncAssertion 等待增量订单同步 API 完成。 */ () => expect(orderListMocks.syncOrders).toHaveBeenCalledWith(undefined, 'all', expect.objectContaining({ signal: expect.any(AbortSignal), onProgress: expect.any(Function) }), 'incremental'));
    expect(orderListMocks.loadOrders).toHaveBeenCalled();
    expect(window.alert).not.toHaveBeenCalledWith('同步完成');

		fireEvent.click(screen.getByText('全量校准'));
		await waitFor(/* fullSyncAssertion 等待完整订单校准 API 完成。 */ () => expect(orderListMocks.syncOrders).toHaveBeenLastCalledWith(undefined, 'all', expect.objectContaining({ signal: expect.any(AbortSignal), onProgress: expect.any(Function) }), 'full'));

    fireEvent.click(screen.getByText('插入订单'));
    expect(orderListMocks.openImportModal).toHaveBeenCalledTimes(1);
  });

	test('联系买家按订单账号、精确会话和商品跳转到聊天页', /* contactBuyerCase 验证订单入口不会按昵称猜会话。 */ () => {
		render(<OrderList />);
		fireEvent.click(screen.getByTitle('联系买家'));
		expect(window.location.pathname).toBe('/app/chat');
		// parameters 是订单入口写入聊天地址的精确上下文。
		const parameters = new URLSearchParams(window.location.search);
		expect(Object.fromEntries(parameters.entries())).toEqual({ account_id: 'account-1', chat_id: 'chat-1', item_id: 'item-1' });
	});

	test('待结算卡片展示汇总后扣除百分之一点六服务费的结果', /* settlementSummaryCase 验证页面不使用当前分页金额自行累加。 */ () => {
		render(<OrderList />);
		// summary 是服务端权威待结算统计卡片。
		const summary = screen.getByTestId('order-settlement-summary');
		expect(within(summary).getByText('¥398.52')).toBeTruthy();
		expect(within(summary).getByText('当前筛选范围 · 3 笔已发货订单')).toBeTruthy();
		expect(within(summary).getByText('¥405.00')).toBeTruthy();
		expect(within(summary).getByText('-¥6.48')).toBeTruthy();
		expect(within(summary).getByText('平台服务费（1.6%）')).toBeTruthy();
	});

	test('缺少精确 chat_id 的订单保留禁用入口', /* missingChatCase 验证旧订单不会按买家 ID 猜测会话。 */ () => {
		orderListMocks.orders = [{ ...orderFixture, chat_id: '' }];
		render(<OrderList />);
		// contactButton 是缺少精确关联时仍可解释原因的禁用入口。
		const contactButton = screen.getByTitle('该订单尚未关联精确聊天会话') as HTMLButtonElement;
		expect(contactButton.disabled).toBe(true);
	});

	test('ERP 发货凭证按钮打开本地弹窗，手机发货订单禁用并解释原因', /* shipmentProofButtonCase 验证按钮替代旧闲鱼外链。 */ () => {
		render(<OrderList />);
		fireEvent.click(screen.getByTitle('查看发货凭证'));
		expect(screen.getByTestId('shipment-proof-modal').textContent).toContain('order-1');
		cleanup();
		orderListMocks.orders = [{ ...orderFixture, shipment_proof_available: false, shipment_proof_unavailable_reason: '该订单不是通过 ERP 发货，暂无可查看凭证' }];
		render(<OrderList />);
		// disabledProofButton 是手机发货订单的禁用凭证入口。
		const disabledProofButton = screen.getByTitle('该订单不是通过 ERP 发货，暂无可查看凭证') as HTMLButtonElement;
		expect(disabledProofButton.disabled).toBe(true);
	});

	test('实时卡片生成的临时订单自动同步所属店铺并隐藏零金额', /* incompleteOrderCase 验证新订单无需手工组合单刷和增量同步。 */ async () => {
		// incompleteOrder 保存金额和买家仍未由平台列表补齐的新订单。
		const incompleteOrder = { ...orderFixture, order_id: 'order-pending-data', amount: '0.00', buyer_id: 'account-1', buyer_name: '' };
		orderListMocks.orders = [incompleteOrder];
		render(<OrderList />);
		expect(screen.getByText('资料同步中')).toBeTruthy();
		await waitFor(/* autoEnrichmentAssertion 等待页面自动启动当前店铺的快速增量任务。 */ () => expect(orderListMocks.syncOrders).toHaveBeenCalledWith('account-1', 'all', expect.objectContaining({ signal: expect.any(AbortSignal), onProgress: expect.any(Function) }), 'incremental'));
		await waitFor(/* refreshedListAssertion 等待自动补全完成后刷新当前订单列表。 */ () => expect(orderListMocks.loadOrders).toHaveBeenCalled());
	});

	test('订单列表分别展示已收货和已退款状态', /* 当前回调验证新增生命周期状态不会回退到默认灰色文案。 */ () => {
		orderListMocks.orders = [{ ...orderFixture, order_id: 'received-order', id: 'received-order', status: 'received' }, { ...orderFixture, order_id: 'refunded-order', id: 'refunded-order', status: 'refunded' }];
		render(<OrderList />);
		expect(screen.getByText('已收货')).toBeTruthy();
		expect(screen.getByText('已退款')).toBeTruthy();
	});

	test('订单列表优先展示买家昵称并在缺失时保留 ID 回退', /* 当前回调验证买家昵称、ID 和收货人不会混用。 */ () => {
		orderListMocks.orders = [orderFixture, { ...orderFixture, id: 'row-2', order_id: 'order-2', buyer_id: 'buyer-2', buyer_name: '', buyer_avatar_url: '' }];
		render(<OrderList />);
		// namedBuyerIdentity 表示昵称与辅助 ID 共用的响应式买家身份行。
		const namedBuyerIdentity = screen.getByTestId('buyer-identity-order-1');
		expect(within(namedBuyerIdentity).getByText('测试买家')).toBeTruthy();
		expect(within(namedBuyerIdentity).getByText('buyer-1')).toBeTruthy();
		expect(namedBuyerIdentity.className).toContain('flex-wrap');
		// fallbackBuyerIdentity 表示没有昵称时仍把回退文案和稳定 ID 放在同一身份行。
		const fallbackBuyerIdentity = screen.getByTestId('buyer-identity-order-2');
		expect(within(fallbackBuyerIdentity).getByText('未获取昵称')).toBeTruthy();
		expect(within(fallbackBuyerIdentity).getByText('buyer-2')).toBeTruthy();
		expect(screen.getAllByText('收货人').length).toBeGreaterThan(0);
	});

	test('订单号和买家 ID 点击后复制完整值并在剪贴板失败时回退', /* 当前回调验证两个复制入口共享反馈且不会改写显示值。 */ async () => {
		render(<OrderList />);
		// writeText 记录订单页向浏览器剪贴板提交的完整标识。
		const writeText = vi.mocked(navigator.clipboard.writeText);
		fireEvent.click(screen.getByRole('button', { name: '复制订单号 order-1' }));
		await waitFor(/* orderCopyAssertion 等待订单号写入剪贴板并显示成功反馈。 */ () => expect(writeText).toHaveBeenCalledWith('order-1'));
		expect(screen.getByText('已复制')).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: '复制买家 ID buyer-1' }));
		await waitFor(/* buyerCopyAssertion 等待买家 ID 写入同一剪贴板入口。 */ () => expect(writeText).toHaveBeenLastCalledWith('buyer-1'));
		writeText.mockRejectedValueOnce(new Error('clipboard unavailable'));
		fireEvent.click(screen.getByRole('button', { name: '复制订单号 order-1' }));
		await waitFor(/* copyFallbackAssertion 等待剪贴板失败时提供手动复制内容。 */ () => expect(window.prompt).toHaveBeenCalledWith('复制订单号', 'order-1'));
	});

  test('订单详情和编辑保存保持字段映射', /* 当前回调验证订单详情展示和编辑提交边界。 */ async () => {
		orderListMocks.orders = [{ ...orderFixture, received_at: '2026-08-20T08:00:00Z', completed_at: '2026-08-20T08:01:00Z' }];
    render(<OrderList />);
    fireEvent.click(screen.getByTitle('查看详情'));
    expect(screen.getByText('订单详情')).toBeTruthy();
		expect(screen.getAllByText('测试买家').length).toBeGreaterThan(0);
		expect(screen.getByText('买家确认收货')).toBeTruthy();
		expect(screen.getByText('交易完成')).toBeTruthy();
    await waitFor(/* flowerStatusAssertion 等待详情恢复持久求花状态。 */ () => expect(orderListMocks.getRedFlowerStatus).toHaveBeenCalledWith('order-1'));
    fireEvent.click(screen.getByText('求买家送小红花'));
    await waitFor(/* flowerAssertion 等待手动求花 API 完成。 */ () => expect(orderListMocks.requestRedFlower).toHaveBeenCalledWith('order-1'));
    expect(screen.getByText('✓ 求花成功')).toBeTruthy();
    fireEvent.click(screen.getByText('关闭'));
    expect(screen.queryByText('订单详情')).toBeNull();

    fireEvent.click(screen.getByTitle('编辑订单'));
    fireEvent.change(screen.getByDisplayValue('buyer-1'), { target: { value: 'buyer-2' } });
    fireEvent.change(screen.getByDisplayValue('20.00'), { target: { value: '30.00' } });
    fireEvent.click(screen.getByText('保存更改'));
    await waitFor(/* updateAssertion 等待订单编辑 API 完成。 */ () => expect(orderListMocks.updateOrder).toHaveBeenCalled());
    expect(orderListMocks.updateOrder).toHaveBeenCalledWith('order-1', expect.objectContaining({ order_status: 'pending_ship', buyer_id: 'buyer-2', amount: '30.00' }));
    expect(orderListMocks.loadOrders).toHaveBeenCalled();
  });

  test('已成功求花的订单重开详情后保持禁用并提示同步延迟', /* 当前回调验证持久状态避免用户因延迟重复点击。 */ async () => {
    orderListMocks.getRedFlowerStatus.mockResolvedValueOnce({ success: true, status: 'succeeded', message: '已向买家求花，闲鱼系统卡片同步可能需要几分钟', requested_at: 123 });
    render(<OrderList />);
    fireEvent.click(screen.getByTitle('查看详情'));
    // flowerButton 是从服务端状态恢复后的禁用求花按钮。
    const flowerButton = await screen.findByRole('button', { name: '已向买家求花' });
    expect((flowerButton as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('已向买家求花，闲鱼系统卡片同步可能需要几分钟')).toBeTruthy();
    expect(orderListMocks.requestRedFlower).not.toHaveBeenCalled();
  });

  test('发货、单笔同步和删除操作分别调用对应 API', /* 当前回调验证订单行动作和结果收束。 */ async () => {
    render(<OrderList />);
    fireEvent.click(screen.getByText('立即发货'));
    expect(screen.getByText('请选择发货方式：')).toBeTruthy();
    fireEvent.click(screen.getByText('仅修改闲鱼发货状态'));
    await waitFor(/* shipAssertion 等待订单发货 API 完成。 */ () => expect(orderListMocks.manualShipOrder).toHaveBeenCalledWith(['order-1'], 'status_only'));
    expect(screen.getByText('✓ 发货成功')).toBeTruthy();
    expect(orderListMocks.loadOrders).toHaveBeenCalled();

    fireEvent.click(screen.getByTitle('同步订单'));
    await waitFor(/* singleSyncAssertion 等待单笔订单同步 API 完成。 */ () => expect(orderListMocks.syncSingleOrder).toHaveBeenCalledWith('order-1'));
    expect(orderListMocks.loadOrders).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByTitle('删除订单'));
    await waitFor(/* deleteAssertion 等待订单删除 API 完成。 */ () => expect(orderListMocks.deleteOrder).toHaveBeenCalledWith('order-1'));
    expect(orderListMocks.loadOrders).toHaveBeenCalledTimes(2);
    expect(orderListMocks.setPage).toHaveBeenCalledWith(expect.any(Function));
  });

  test('分页按钮使用函数式更新并遵守边界状态', /* 当前回调验证订单分页控件的可操作边界。 */ () => {
    render(<OrderList />);
    fireEvent.click(screen.getByRole('button', { name: '上一页' }));
    fireEvent.click(screen.getByRole('button', { name: '下一页' }));
    expect(orderListMocks.setPage).toHaveBeenCalledWith(expect.any(Function));
    // pageUpdaters 保存页面翻页使用的函数式更新回调。
    const pageUpdaters = orderListMocks.setPage.mock.calls.filter(/* pageUpdaterCall 筛选分页函数式更新调用。 */ call => typeof call[0] === 'function');
    expect(pageUpdaters).toHaveLength(2);
    expect(pageUpdaters[0][0](2)).toBe(1);
    expect(pageUpdaters[1][0](2)).toBe(3);
  });
});
