// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import type { Item } from '../api';
import { updateItemSKUCosts } from '../api';
import { ItemSKUDetailsModal,parseCostDraft } from './ItemSKUDetailsModal';
import { ItemSpecSummary,itemPriceRange,itemRemainingQuantity,itemSpecValues } from './ItemSpecSummary';

vi.mock('../api', /* itemSKUApiMockFactory 提供本地成本批量接口替身。 */ async importOriginal => {
  // original 保存商品 API 的真实纯转换函数和类型导出。
  const original = await importOriginal<typeof import('../api')>();
  return { ...original, updateItemSKUCosts: vi.fn() };
});

// updateCostsMock 是本地 SKU 成本原子保存接口的可控替身。
const updateCostsMock = vi.mocked(updateItemSKUCosts);

// itemFixture 是三规格真实库存结构的脱敏回归样本。
const itemFixture: Item = {
  id: 1, cookie_id: 'account-1', item_id: 'item-1', item_title: '多规格月卡商品', is_multi_spec: true, sku_count: 3,
  skus: [
    { sku_id: 'sku-1', inventory_id: 'inventory-1', properties: [{ name: '套餐', value: '一倍额度', enabled: true, status: 0, sort_order: 0 }], price_cents: 13500, quantity: 955, initial_quantity: 1000, enabled: true, sort_order: 0, cost_cents: null, synced_at: 1787287605, local_only: false },
    { sku_id: 'sku-2', inventory_id: 'inventory-2', properties: [{ name: '套餐', value: '五倍额度', enabled: true, status: 0, sort_order: 1 }], price_cents: 69000, quantity: 997, initial_quantity: 1000, enabled: true, sort_order: 1, cost_cents: 50000, synced_at: 1787287605, local_only: false },
    { sku_id: 'sku-3', inventory_id: 'inventory-3', properties: [{ name: '套餐', value: '二十倍额度', enabled: true, status: 0, sort_order: 2 }], price_cents: 125000, quantity: 997, initial_quantity: 1000, enabled: true, sort_order: 2, cost_cents: null, synced_at: 1787287605, local_only: false },
  ],
};

// singleItemFixture 是没有平台 SKU 的单规格商品。
const singleItemFixture: Item = { id: 2, cookie_id: 'account-1', item_id: 'single-item', item_title: '5x会员月卡老客户专拍', item_price: '¥690', is_multi_spec: false, sku_count: 0, skus: [] };

describe('商品规格展示与统一成本保存', /* itemCostFeatureSuite 验证多规格批量保存和单规格入口。 */ () => {
  beforeEach(/* resetItemCostApiMock 重置成本接口替身。 */ () => {
    vi.clearAllMocks();
    updateCostsMock.mockResolvedValue({ success: true });
  });
  afterEach(/* cleanupItemCostRender 清理当前测试渲染。 */ () => cleanup());

  test('多规格卡片摘要保持价格区间、规格值和实时库存', /* itemSpecSummaryCase 验证卡片摘要。 */ () => {
    // onOpen 是成本详情打开回调。
    const onOpen = vi.fn();
    render(<ItemSpecSummary item={itemFixture} onOpen={onOpen} />);
    expect(screen.getByText('套餐 · 3 个规格')).toBeTruthy();
    expect(screen.getByText('剩余 2949')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '查看 3 个规格' }));
    expect(onOpen).toHaveBeenCalledTimes(1);
    expect(itemPriceRange(itemFixture)).toBe('¥135–¥1250');
    expect(itemRemainingQuantity(itemFixture)).toBe(2949);
    expect(itemSpecValues(itemFixture)).toEqual(['一倍额度', '五倍额度', '二十倍额度']);
  });

  test('多规格成本只通过右下角按钮一次性原子提交', /* batchCostSaveCase 验证统一保存成功路径。 */ async () => {
    // onCostsSaved 记录统一保存后的列表回写。
    const onCostsSaved = vi.fn();
    render(<ItemSKUDetailsModal item={itemFixture} onClose={vi.fn()} onCostsSaved={onCostsSaved} />);
    expect(screen.queryByRole('button', { name: '保存一倍额度成本' })).toBeNull();
    fireEvent.change(screen.getAllByRole('textbox', { name: '一倍额度单件成本' })[0], { target: { value: '88.50' } });
    fireEvent.change(screen.getAllByRole('textbox', { name: '五倍额度单件成本' })[0], { target: { value: '630.00' } });
    fireEvent.click(screen.getByRole('button', { name: '保存成本' }));
    // expectedCosts 是一次请求提交的全部 SKU 成本。
    const expectedCosts = [{ sku_id: 'sku-1', cost_cents: 8850 }, { sku_id: 'sku-2', cost_cents: 63000 }, { sku_id: 'sku-3', cost_cents: null }];
    await waitFor(/* waitBatchSaved 等待统一成本请求完成。 */ () => expect(updateCostsMock).toHaveBeenCalledWith('account-1', 'item-1', expectedCosts, expect.objectContaining({ signal: expect.any(AbortSignal) })));
    await waitFor(/* waitBatchCallback 等待全部成本回写。 */ () => expect(onCostsSaved).toHaveBeenCalledWith(expectedCosts));
    expect(await screen.findByText('成本已保存')).toBeTruthy();
  });

  test('单规格商品显示设置成本入口并使用本地隐式 SKU', /* singleItemCostCase 验证无平台 SKU 的成本保存。 */ async () => {
    // onOpen 是单规格卡片成本入口回调。
    const onOpen = vi.fn();
    // summary 是单规格商品卡片成本入口渲染结果。
    const summary = render(<ItemSpecSummary item={singleItemFixture} onOpen={onOpen} />);
    fireEvent.click(screen.getByRole('button', { name: '设置成本' }));
    expect(onOpen).toHaveBeenCalledTimes(1);
    summary.unmount();
    render(<ItemSKUDetailsModal item={singleItemFixture} onClose={vi.fn()} onCostsSaved={vi.fn()} />);
    expect(screen.getByText('单规格商品售价')).toBeTruthy();
    expect(screen.getByText('¥690')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('本地单件成本'), { target: { value: '630' } });
    fireEvent.click(screen.getByRole('button', { name: '保存成本' }));
    await waitFor(/* waitSingleCostSaved 等待隐式成本保存。 */ () => expect(updateCostsMock).toHaveBeenCalledWith('account-1', 'single-item', [{ sku_id: '__default__', cost_cents: 63000 }], expect.any(Object)));
  });

  test('任一成本格式错误都会阻止整批请求，卸载会取消在途保存', /* costValidationAndAbortCase 验证原子校验和取消。 */ async () => {
    // pendingRequest 模拟直到取消都不主动完成的批量成本请求。
    updateCostsMock.mockImplementation(/* pendingCostRequest 捕获取消信号并保持请求挂起。 */ (_accountID, _itemID, _costs, options) => new Promise((_resolve, reject) => {
      options?.signal?.addEventListener('abort', /* abortListener 将取消转换为请求拒绝。 */ () => reject(new Error('aborted')), { once: true });
    }));
    // rendered 是当前多规格成本弹窗实例。
    const rendered = render(<ItemSKUDetailsModal item={itemFixture} onClose={vi.fn()} onCostsSaved={vi.fn()} />);
    fireEvent.change(screen.getAllByRole('textbox', { name: '二十倍额度单件成本' })[0], { target: { value: '1.234' } });
    fireEvent.click(screen.getByRole('button', { name: '保存成本' }));
    expect((await screen.findAllByText('成本最多保留两位小数')).length).toBeGreaterThan(0);
    expect(updateCostsMock).not.toHaveBeenCalled();
    fireEvent.change(screen.getAllByRole('textbox', { name: '二十倍额度单件成本' })[0], { target: { value: '100' } });
    fireEvent.click(screen.getByRole('button', { name: '保存成本' }));
    await waitFor(/* waitPendingRequest 等待请求取得取消信号。 */ () => expect(updateCostsMock).toHaveBeenCalled());
    // signal 是统一成本请求收到的取消信号。
    const signal = updateCostsMock.mock.calls[0][3]?.signal;
    rendered.unmount();
    await waitFor(/* waitRequestAborted 等待卸载终止请求。 */ () => expect(signal?.aborted).toBe(true));
    expect(parseCostDraft('0')).toBe(0);
    expect(parseCostDraft('')).toBeNull();
  });

  test('保存失败保留弹窗和草稿并同时回传失败提示', /* costFailureFeedbackCase 验证失败反馈不关闭编辑器。 */ async () => {
    updateCostsMock.mockRejectedValueOnce(new Error('数据库暂时不可用'));
    // onSaveFailed 记录页面红色短暂提示需要展示的失败原因。
    const onSaveFailed = vi.fn();
    render(<ItemSKUDetailsModal item={itemFixture} onClose={vi.fn()} onCostsSaved={vi.fn()} onSaveFailed={onSaveFailed} />);
    // input 是失败后必须保留原值的一倍额度成本输入。
    const input = screen.getAllByRole('textbox', { name: '一倍额度单件成本' })[0] as HTMLInputElement;
    fireEvent.change(input, { target: { value: '118' } });
    fireEvent.click(screen.getByRole('button', { name: '保存成本' }));
    expect((await screen.findByRole('alert')).textContent).toContain('数据库暂时不可用');
    expect(onSaveFailed).toHaveBeenCalledWith('数据库暂时不可用');
    expect(screen.getByRole('dialog')).toBeTruthy();
    expect(input.value).toBe('118');
  });
});
