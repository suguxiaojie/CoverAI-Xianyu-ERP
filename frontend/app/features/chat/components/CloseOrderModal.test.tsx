// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import { closeOrderBySeller,getCloseOrderReasons } from '../api';
import { CloseOrderModal } from './CloseOrderModal';

vi.mock('../api', /* closeOrderApiMockFactory 隔离测试不访问真实闲鱼或本地后端。 */ async importOriginal => {
  // actual 保留当前模块的类型和其他 API 导出。
  const actual = await importOriginal<typeof import('../api')>();
  return { ...actual, getCloseOrderReasons: vi.fn(), closeOrderBySeller: vi.fn() };
});

// getReasonsMock、closeOrderMock 是动态原因和真实关单的可控替身。
const getReasonsMock = vi.mocked(getCloseOrderReasons);
const closeOrderMock = vi.mocked(closeOrderBySeller);
// orderID 是全部用例使用的虚构数字订单号。
const orderID = '5127638256187075541';

describe('CloseOrderModal', /* 当前测试组验证动态原因、应用内确认和真实关单边界。 */ () => {
  beforeEach(/* 当前回调重置本地 API 替身。 */ () => {
    vi.clearAllMocks();
    getReasonsMock.mockResolvedValue({ order_id: orderID, account_id: 'account-1', reasons: ['双方协商一致', '商品无货'] });
    closeOrderMock.mockResolvedValue({ success: true, status: 'succeeded', message: '订单已关闭', order_id: orderID });
  });
  afterEach(/* 当前回调卸载弹窗并恢复替身。 */ () => { cleanup(); vi.restoreAllMocks(); });

  test('选择动态原因、应用内确认后才执行真实关单', /* 当前回调覆盖原因、确认和成功结果页。 */ async () => {
    // onSuccess 记录平台明确关单提示。
    const onSuccess = vi.fn();
    render(<CloseOrderModal accountID="account-1" orderID={orderID} stage="pending_payment" open onClose={vi.fn()} onSuccess={onSuccess} />);
    // reasonOption 是平台动态返回的首个原因。
    const reasonOption = await screen.findByRole('radio', { name: '双方协商一致' });
    expect((screen.getByRole('button', { name: '下一步' }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(reasonOption);
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    expect(screen.getByRole('dialog', { name: '确认取消订单' })).toBeTruthy();
    expect(closeOrderMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '确认取消订单' }));
    await waitFor(/* closeFinished 等待真实关单替身完成。 */ () => expect(closeOrderMock).toHaveBeenCalledWith('account-1', orderID, '双方协商一致', expect.objectContaining({ signal: expect.any(AbortSignal) })));
    expect(await screen.findByRole('dialog', { name: '订单已取消' })).toBeTruthy();
    expect(screen.getByRole('status').textContent).toContain('订单已关闭');
    expect(onSuccess).toHaveBeenCalledWith('订单已关闭');
  });

  test('关单失败停留确认页且允许返回原因选择', /* 当前回调覆盖明确失败反馈。 */ async () => {
    closeOrderMock.mockRejectedValueOnce(new Error('订单当前状态不可取消'));
    render(<CloseOrderModal accountID="account-1" orderID={orderID} stage="pending_payment" open onClose={vi.fn()} />);
    fireEvent.click(await screen.findByRole('radio', { name: '商品无货' }));
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    fireEvent.click(screen.getByRole('button', { name: '确认取消订单' }));
    expect((await screen.findByRole('alert')).textContent).toContain('订单当前状态不可取消');
    fireEvent.click(screen.getByRole('button', { name: '返回修改' }));
    expect(screen.getByRole('dialog', { name: '取消订单' })).toBeTruthy();
    expect((screen.getByRole('radio', { name: '商品无货' }) as HTMLInputElement).checked).toBe(true);
  });

	test('已付款待发货阶段明确提示取消后原路退款', /* paidCancelCopyCase 验证统一取消入口不会误用未付款说明。 */ async () => {
		render(<CloseOrderModal accountID="account-1" orderID={orderID} stage="pending_ship" open onClose={vi.fn()} />);
		fireEvent.click(await screen.findByRole('radio', { name: '双方协商一致' }));
		fireEvent.click(screen.getByRole('button', { name: '下一步' }));
		expect(screen.getByText('取消后买家已支付款项将原路退回')).toBeTruthy();
		expect(screen.getByText(/已付款但尚未发货/)).toBeTruthy();
	});
});
