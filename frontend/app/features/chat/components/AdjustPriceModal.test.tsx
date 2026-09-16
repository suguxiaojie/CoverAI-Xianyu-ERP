// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import { adjustOrderPrice,getOrderAdjustPriceForm } from '../api';
import { AdjustPriceModal } from './AdjustPriceModal';

vi.mock('../api', /* adjustPriceApiMockFactory 隔离测试不访问真实闲鱼或本地后端。 */ async importOriginal => {
  // actual 保留当前模块的类型和其他 API 导出。
  const actual = await importOriginal<typeof import('../api')>();
  return { ...actual, getOrderAdjustPriceForm: vi.fn(), adjustOrderPrice: vi.fn() };
});

// getFormMock、adjustPriceMock 是动态表单和真实 submit 的可控替身。
const getFormMock = vi.mocked(getOrderAdjustPriceForm);
const adjustPriceMock = vi.mocked(adjustOrderPrice);

// formFixture 是与真实网页抓包一致的动态改价表单。
const formFixture = {
  order_id: '5127694777172175924', account_id: 'account-1', title: '修改价格', fields: [
    { key: 'modifyFee', name: '商品价格', prefix_text: '¥', value: '0.10', read_only: false },
    { key: 'newTransportFee', name: '运费', prefix_text: '¥', value: '0.00', read_only: false },
  ],
};

describe('AdjustPriceModal', /* 当前测试组验证动态表单、应用内确认、真实提交和取消边界。 */ () => {
  beforeEach(/* 当前回调重置动态表单与提交 API 替身。 */ () => {
    vi.clearAllMocks();
    getFormMock.mockResolvedValue(formFixture);
    adjustPriceMock.mockResolvedValue({ success: true, status: 'succeeded', message: '价格修改成功', order_id: formFixture.order_id, fields: [
      { ...formFixture.fields[0], value: '0.20' }, formFixture.fields[1],
    ] });
  });

  afterEach(/* 当前回调卸载弹窗并恢复浏览器替身。 */ () => {
    cleanup();
    vi.restoreAllMocks();
  });

  test('读取动态字段、应用内核对价格变化并提交规范元金额', /* 当前回调覆盖编辑、确认和成功结果页。 */ async () => {
    // onSuccess 记录平台明确成功提示。
    const onSuccess = vi.fn();
    render(<AdjustPriceModal accountID="account-1" orderID={formFixture.order_id} open onClose={vi.fn()} onSuccess={onSuccess} />);
    // productInput 是 render 返回的商品价格输入框。
    const productInput = await screen.findByLabelText('商品价格');
    // transportInput 是 render 返回的运费输入框。
    const transportInput = screen.getByLabelText('运费');
    expect((productInput as HTMLInputElement).value).toBe('0.10');
    expect((transportInput as HTMLInputElement).value).toBe('0.00');
    expect((screen.getByRole('button', { name: '下一步' }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('当前金额尚未修改，调整后即可进入下一步')).toBeTruthy();
    fireEvent.change(productInput, { target: { value: '0.20' } });
    expect(screen.getByText('¥0.20')).toBeTruthy();
    expect((screen.getByRole('button', { name: '下一步' }) as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    expect(screen.getByRole('dialog', { name: '确认改价' })).toBeTruthy();
    expect(screen.getByText('请核对本次价格变化')).toBeTruthy();
    expect(screen.getByText('原 ¥0.10')).toBeTruthy();
    expect(adjustPriceMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '确认改为 ¥0.20' }));
    await waitFor(/* submitFinished 等待平台替身完成。 */ () => expect(adjustPriceMock).toHaveBeenCalledWith('account-1', formFixture.order_id, expect.arrayContaining([
      expect.objectContaining({ key: 'modifyFee', value: '0.20' }), expect.objectContaining({ key: 'newTransportFee', value: '0.00' }),
    ]), expect.objectContaining({ signal: expect.any(AbortSignal) })));
    expect(await screen.findByRole('dialog', { name: '价格修改成功' })).toBeTruthy();
    expect(screen.getByRole('status').textContent).toContain('价格修改成功');
    expect(screen.getByRole('button', { name: '完成' })).toBeTruthy();
    expect(onSuccess).toHaveBeenCalledWith('价格修改成功');
  });

  test('submit 失败保留弹窗并展示服务端错误', /* 当前回调覆盖明确失败反馈。 */ async () => {
    adjustPriceMock.mockRejectedValueOnce(new Error('订单不是当前账号可修改的待付款订单'));
    render(<AdjustPriceModal accountID="account-1" orderID={formFixture.order_id} open onClose={vi.fn()} />);
    fireEvent.change(await screen.findByLabelText('商品价格'), { target: { value: '0.20' } });
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    fireEvent.click(screen.getByRole('button', { name: '确认改为 ¥0.20' }));
    expect((await screen.findByRole('alert')).textContent).toContain('订单不是当前账号可修改的待付款订单');
    expect(screen.getByRole('button', { name: '确认改为 ¥0.20' })).toBeTruthy();
    expect(screen.getByRole('button', { name: '返回修改' })).toBeTruthy();
  });

  test('确认页返回修改会保留输入且不调用 submit', /* 当前回调验证两步流程不丢失用户编辑状态。 */ async () => {
    render(<AdjustPriceModal accountID="account-1" orderID={formFixture.order_id} open onClose={vi.fn()} />);
    // productInput 是本轮需要跨确认页往返保留的商品价格输入。
    const productInput = await screen.findByLabelText('商品价格');
    fireEvent.change(productInput, { target: { value: '0.20' } });
    fireEvent.click(screen.getByRole('button', { name: '下一步' }));
    fireEvent.click(screen.getByRole('button', { name: '返回修改' }));
    expect(screen.getByRole('dialog', { name: '修改价格' })).toBeTruthy();
    expect((screen.getByLabelText('商品价格') as HTMLInputElement).value).toBe('0.20');
    expect(adjustPriceMock).not.toHaveBeenCalled();
  });

  test('关闭弹窗会取消尚未完成的 render', /* 当前回调覆盖卸载取消，不允许晚到响应写入。 */ async () => {
    // capturedSignal 保存 API 收到的取消信号。
    let capturedSignal: AbortSignal | undefined;
    getFormMock.mockImplementationOnce(/* pendingRender 永不主动完成，等待卸载取消。 */ (_accountID, _orderID, options) => {
      capturedSignal = options?.signal;
      return new Promise(/* keepRenderPending 保持请求未完成以验证卸载取消。 */ () => {});
    });
    // unmount 卸载当前弹窗，触发 effect cleanup。
    const { unmount } = render(<AdjustPriceModal accountID="account-1" orderID={formFixture.order_id} open onClose={vi.fn()} />);
    await waitFor(/* signalCreated 等待 render 请求建立。 */ () => expect(capturedSignal).toBeDefined());
    unmount();
    expect(capturedSignal?.aborted).toBe(true);
  });
});
