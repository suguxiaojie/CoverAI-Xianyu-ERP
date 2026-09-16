// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import { getMerchantRefundRefuseForm,refuseMerchantRefund,submitOrderRefundAction,type ChatSystemCard } from '../api';
import { redFlowerReceiveURL,TradeSystemCard } from './TradeSystemCard';

vi.mock('../api', /* chatAPIMock 保留现有导出并替换退款详情只读请求。 */ async importOriginal => {
	// original 是 Chat API 模块的真实导出集合。
	const original = await importOriginal<typeof import('../api')>();
	return { ...original,
		getOrderRefundDetail: vi.fn().mockResolvedValue({ order_id: '3316374662163136097', account_id: 'seller-account', status: '1', status_text: '等待卖家处理', type: '仅退款', reason: '协商一致退款', amount: '135.00', apply_time: '2026-08-23 21:16', buyer_description: '升级套餐', buyer_images: [], buyer_videos: [], seller: true, actions: [{ code: 'sellerAgreeRefund', name: '同意退款', kind: 'agree', mode: 'merchant_verify', confirm_title: '确认同意退款' }, { code: 'sellerRejectRefund', name: '拒绝退款', kind: 'reject', mode: 'merchant_refuse', confirm_title: '确认拒绝退款' }], official_url: 'https://h5.m.goofish.com/refund' }),
		submitOrderRefundAction: vi.fn().mockResolvedValue({ success: true, status: 'succeeded', message: '已同意退款申请', order_id: '3316374662163136097', refund_id: 'refund-1', action: 'agree' }),
		startMerchantRefundVerification: vi.fn().mockResolvedValue({ session_id: 'verify-session', verify_url: 'https://pcauth-site.alipay.com/PASSWORD?token=test', verify_origin: 'https://pcauth-site.alipay.com', expires_at: 9999999999 }),
		completeMerchantRefundVerification: vi.fn().mockResolvedValue({ success: true, status: 'succeeded', message: '退款成功', order_id: '3316374662163136097', refund_id: 'refund-1', action: 'agree' }),
		getMerchantRefundRefuseForm: vi.fn().mockResolvedValue({ refund_id: 'refund-1', reasons: [{ id: '11', name: '已发货，无需邮寄', requires_app: false }], proof_required: false, negotiation_enabled: false, min_cents: 0, max_cents: 0 }),
		refuseMerchantRefund: vi.fn().mockResolvedValue({ success: true, status: 'succeeded', message: '已拒绝退款申请', order_id: '3316374662163136097', refund_id: 'refund-1', action: 'reject' }),
	};
});

// pendingCard 是待付款只读改价入口的结构化测试卡片。
const pendingCard: ChatSystemCard = {
  kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款',
  description: '请双方沟通及时确认价格', order_id: 'order-1', item_id: 'item-1', action: 'adjust_price',
};

describe('TradeSystemCard', /* 当前测试组验证交易卡片的只读动作和响应式容器。 */ () => {
  beforeEach(/* 当前回调隔离浏览器确认和新窗口操作。 */ () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    vi.spyOn(window, 'open').mockImplementation(/* openOfficialPageMock 禁止测试打开真实闲鱼页面。 */ () => null);
		vi.mocked(getMerchantRefundRefuseForm).mockResolvedValue({ refund_id: 'refund-1', reasons: [{ id: '11', name: '已发货，无需邮寄', requires_app: false }], proof_required: false, negotiation_enabled: false, min_cents: 0, max_cents: 0 });
		vi.mocked(refuseMerchantRefund).mockResolvedValue({ success: true, status: 'succeeded', message: '已拒绝退款申请', order_id: '3316374662163136097', refund_id: 'refund-1', action: 'reject' });
  });

  afterEach(/* 当前回调清理卡片测试 DOM 和浏览器替身。 */ () => {
    cleanup();
    vi.restoreAllMocks();
  });

  test('待付款卡片展示真实改价入口和响应式宽度', /* 当前回调确保结构化账号与订单可以打开动态表单。 */ () => {
    render(<TradeSystemCard card={pendingCard} time="08/19 16:20" accountID="account-1" />);
    // cardElement 是交易卡片的可访问文章容器。
    const cardElement = screen.getByRole('article', { name: '交易状态：我已拍下，待付款' });
    expect(cardElement.className).toContain('w-full');
    expect(cardElement.className).toContain('max-w-xl');
    expect(screen.getByText('请双方沟通及时确认价格')).toBeTruthy();
    // adjustButton 是具备账号和订单关联的真实改价入口。
    const adjustButton = screen.getByRole('button', { name: '修改价格' });
    expect((adjustButton as HTMLButtonElement).disabled).toBe(false);
    expect(adjustButton.getAttribute('title')).toBe('读取闲鱼当前价格');
    expect(screen.getByText('仅待付款订单可改价，提交前会再次确认')).toBeTruthy();
    expect(screen.queryByRole('button', { name: '取消订单' })).toBeNull();
  });

  test('非待付款交易状态不显示改价动作', /* 当前回调防止已付款和已发货卡片保留旧按钮。 */ () => {
    // paidCard 是不允许改价的已付款交易状态。
    const paidCard: ChatSystemCard = { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', description: '请及时处理订单', order_id: 'order-1' };
    render(<TradeSystemCard card={paidCard} time="08/19 16:21" />);
    expect(screen.getByRole('article', { name: '交易状态：我已付款，等待你发货' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: '修改价格' })).toBeNull();
  });

	test('需要支付密码的退款动作在 ERP 内加载支付宝验证 iframe', /* 当前回调验证密码不进入 ERP 输入框。 */ async () => {
		// refundCard 是具备订单归属的退款申请系统卡片。
		const refundCard: ChatSystemCard = { kind: 'trade', event: 'refund_requested', title: '我发起了退款申请', description: '买家申请退款', order_id: '3316374662163136097' };
		render(<TradeSystemCard card={refundCard} time="08/23 21:16" accountID="seller-account" />);
		fireEvent.click(screen.getByRole('article', { name: '交易状态：我发起了退款申请' }));
		expect(await screen.findByRole('dialog', { name: '退款申请详情' })).toBeTruthy();
		expect(await screen.findByText('协商一致退款')).toBeTruthy();
		expect(screen.getByText('¥135.00')).toBeTruthy();
		expect(screen.getByText('等待卖家处理')).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: '同意退款' }));
		// confirmButton 是应用内第二次点击后创建 PC 支付验证的按钮。
		const confirmButton = screen.getByRole('button', { name: '开始支付验证' });
		expect(confirmButton).toBeTruthy();
		fireEvent.click(confirmButton);
		expect(await screen.findByTitle('支付宝退款验证')).toBeTruthy();
		expect(screen.getByText(/支付密码由支付宝安全页面直接接收，ERP 不读取、不保存/)).toBeTruthy();
		expect(window.open).not.toHaveBeenCalled();
		expect(submitOrderRefundAction).not.toHaveBeenCalled();
	});

	test('必须上传凭证的拒绝退款在选择图片后提交原始文件', /* refundProofRequiredCase 验证图片不会以任意 URL 进入请求。 */ async () => {
		vi.mocked(getMerchantRefundRefuseForm).mockResolvedValue({ refund_id: 'refund-1', reasons: [{ id: '11', name: '已发货，无需邮寄', requires_app: false }], selected_reason_id: '11', proof_required: true, proof_placeholder: '描述发货方式', negotiation_enabled: false, min_cents: 0, max_cents: 0 });
		// refundCard 是具备订单归属的退款申请卡片。
		const refundCard: ChatSystemCard = { kind: 'trade', event: 'refund_requested', title: '我发起了退款申请', description: '买家申请退款', order_id: '3316374662163136097' };
		render(<TradeSystemCard card={refundCard} time="08/23 21:16" accountID="seller-account" />);
		fireEvent.click(screen.getByRole('article', { name: '交易状态：我发起了退款申请' }));
		expect(await screen.findByRole('dialog', { name: '退款申请详情' })).toBeTruthy();
		fireEvent.click(await screen.findByRole('button', { name: '拒绝退款' }));
		// reasonSelect 是平台动态拒绝原因选择器。
		const reasonSelect = await screen.findByRole('combobox', { name: '拒绝原因' });
		fireEvent.change(reasonSelect, { target: { value: '11' } });
		expect(await screen.findByText('该原因必须上传至少 1 张图片凭证。')).toBeTruthy();
		// proofImage 是用户通过系统选择器添加的内存 PNG。
		const proofImage = new File([new Uint8Array([137, 80, 78, 71])], 'proof.png', { type: 'image/png', lastModified: 1 });
		fireEvent.change(screen.getByLabelText('选择退款凭证图片'), { target: { files: [proofImage] } });
		// submitButton 是图片满足必填门禁后的最终拒绝按钮。
		const submitButton = screen.getByRole('button', { name: '确认拒绝退款' });
		expect((submitButton as HTMLButtonElement).disabled).toBe(false);
		fireEvent.click(submitButton);
		await vi.waitFor(/* refundRefuseSubmitted 等待最终 multipart API 替身完成。 */ () => expect(refuseMerchantRefund).toHaveBeenCalledWith('seller-account', '3316374662163136097', '11', '', 0, [proofImage], expect.objectContaining({ signal: expect.any(AbortSignal), timeoutMs: 100_000 })));
	});

	test('卖家付款卡片显示立即发货而买家视角不显示', /* 当前回调验证 ship_order 仍需卖家消息视角才可见。 */ () => {
		// paidCard 是带明确卖家订单详情动作的付款卡片。
		const paidCard: ChatSystemCard = { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', description: '请及时处理订单', order_id: '5127372002248048713', action: 'ship_order' };
		const { rerender } = render(<TradeSystemCard card={paidCard} time="08/20 19:38" accountID="seller-account" />);
		// shipmentButton 是卖家卡片上的无需寄件入口。
		const shipmentButton = screen.getByRole('button', { name: '立即发货' });
		expect((shipmentButton as HTMLButtonElement).disabled).toBe(false);
		expect(screen.getByText('支持无需寄件、描述和最多 3 张图片凭证')).toBeTruthy();
		rerender(<TradeSystemCard card={paidCard} time="08/20 19:38" accountID="buyer-account" sellerActionAllowed={false} />);
		expect(screen.queryByRole('button', { name: '立即发货' })).toBeNull();
	});

	test('同订单后续发货终态会保留并置灰原付款按钮', /* 当前回调避免历史付款卡片重复提交发货。 */ () => {
		// paidCard 是等待后续终态控制资格的卖家付款卡片。
		const paidCard: ChatSystemCard = { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: '5127372002248048713', action: 'ship_order' };
		render(<TradeSystemCard card={paidCard} time="08/20 19:38" accountID="seller-account" shipmentDisabledReason="订单已经发货，不能重复提交" />);
		// shipmentButton 是后续终态禁用的原按钮。
		const shipmentButton = screen.getByRole('button', { name: '立即发货' });
		expect((shipmentButton as HTMLButtonElement).disabled).toBe(true);
		expect(screen.getByText('订单已经发货，不能重复提交')).toBeTruthy();
	});

  test('买家账号收到同一待付款广播时不显示卖家操作', /* 当前回调防止双账号管理场景把买家 Cookie 用于改价或卖家关单。 */ () => {
    render(<TradeSystemCard card={pendingCard} time="08/19 16:20" accountID="buyer-account" sellerActionAllowed={false} />);
    expect(screen.queryByRole('button', { name: '修改价格' })).toBeNull();
    expect(screen.queryByRole('button', { name: '取消订单' })).toBeNull();
  });

  test('同订单后续已付款时保留并置灰原改价按钮', /* 当前回调确保历史卡片位置稳定且不会再次打开真实改价弹窗。 */ () => {
    render(<TradeSystemCard card={pendingCard} time="08/19 16:20" accountID="account-1" adjustPriceDisabledReason="买家已付款，不能再修改价格" />);
    // adjustButton 是已由后续付款终态禁用的原改价按钮。
    const adjustButton = screen.getByRole('button', { name: '修改价格' });
    expect((adjustButton as HTMLButtonElement).disabled).toBe(true);
    expect(adjustButton.className).toContain('bg-slate-200');
    expect(adjustButton.getAttribute('title')).toBe('买家已付款，不能再修改价格');
    expect(screen.getByText('买家已付款，不能再修改价格')).toBeTruthy();
    fireEvent.click(adjustButton);
    expect(screen.queryByRole('dialog', { name: '修改价格' })).toBeNull();
  });

  test('收花 URL 固定官方域名路径并拒绝异常订单号', /* 当前回调验证外部跳转不能由卡片载荷改写。 */ () => {
    // target 是数字订单号生成的官方收花地址。
    const target = new URL(redFlowerReceiveURL('5127372398162002704'));
    expect(target.origin).toBe('https://h5.m.goofish.com');
    expect(target.pathname).toBe('/wow/moyu/moyu-project/temp-pages/pages/red-flower-play');
    expect(target.searchParams.get('role')).toBe('seller');
    expect(target.searchParams.get('confirm')).toBe('false');
    expect(target.searchParams.get('orderId')).toBe('5127372398162002704');
    expect(redFlowerReceiveURL('https://evil.example/steal')).toBe('');
    expect(redFlowerReceiveURL('')).toBe('');
  });

  test('送花卡片二次确认后只打开闲鱼官方页面', /* 当前回调验证 ERP 不直接调用安全收花接口。 */ () => {
    // flowerCard 是带明确订单和官方收花动作的送花卡片。
    const flowerCard: ChatSystemCard = { kind: 'trade', event: 'red_flower_sent', title: '你人真不错，送你闲鱼小红花', description: '买卖换真心', order_id: '5127372398162002704', action: 'receive_red_flower' };
    render(<TradeSystemCard card={flowerCard} time="08/19 22:30" />);
    // receiveButton 是需要人工二次确认的官方页面入口。
    const receiveButton = screen.getByRole('button', { name: '立即收花' });
    vi.mocked(window.confirm).mockReturnValueOnce(false);
    fireEvent.click(receiveButton);
    expect(window.open).not.toHaveBeenCalled();
    vi.mocked(window.confirm).mockReturnValueOnce(true);
    fireEvent.click(receiveButton);
    expect(window.open).toHaveBeenCalledWith(expect.stringContaining('h5.m.goofish.com'), '_blank', 'noopener,noreferrer');
    expect(screen.getByRole('button', { name: '重新打开收花页' })).toBeTruthy();
    expect(screen.getByText('已打开闲鱼官方页面，请在页面内完成收花')).toBeTruthy();
  });

  test('后续收花结果会永久禁用原送花卡片', /* 当前回调验证历史刷新后不会重复打开已完成订单。 */ () => {
    // flowerCard 是已经由后续结果确认收取的送花卡片。
    const flowerCard: ChatSystemCard = { kind: 'trade', event: 'red_flower_sent', title: '你人真不错，送你闲鱼小红花', order_id: '5127372398162002704', action: 'receive_red_flower' };
    render(<TradeSystemCard card={flowerCard} time="08/19 22:30" received />);
    // receivedButton 是禁用的已收花状态按钮。
    const receivedButton = screen.getByRole('button', { name: '已收花' });
    expect((receivedButton as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('已检测到闲鱼收花结果')).toBeTruthy();
    expect(window.open).not.toHaveBeenCalled();
  });
});
