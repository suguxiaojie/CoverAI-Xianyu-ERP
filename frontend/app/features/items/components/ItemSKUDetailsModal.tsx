import { Check,Database,Save,X } from 'lucide-react';
import React,{ useEffect,useMemo,useRef,useState } from 'react';
import type { Item,ItemSKU } from '../api';
import { itemErrorMessage,updateItemSKUCosts } from '../api';
import { defaultItemSKUID,formatCents,itemPriceRange,itemRemainingQuantity,itemSpecName,localItemCostSKU,orderedItemSKUs } from './ItemSpecSummary';

// SavedSKUCost 描述统一保存成功后回写商品列表的单个成本。
export interface SavedSKUCost {
  /** sku_id 是平台 SKU 或单规格本地隐式 SKU 标识。 */
  sku_id: string;
  /** cost_cents 是可空人民币分值。 */
  cost_cents: number | null;
}

// ItemSKUDetailsModalProps 描述规格详情、统一成本保存和关闭回调。
export interface ItemSKUDetailsModalProps {
  /** item 是当前打开成本编辑器的商品。 */
  item: Item;
  /** onClose 关闭弹窗并取消仍在执行的本地成本请求。 */
  onClose: () => void;
  /** onCostsSaved 把原子保存后的全部成本同步回商品列表状态。 */
  onCostsSaved: (costs: SavedSKUCost[]) => void;
  /** onSaveFailed 把保存失败原因交给页面短暂提示，弹窗本身保持打开。 */
  onSaveFailed?: (message: string) => void;
}

// costDraftFromSKU 将可空成本分值转换为用户可编辑的人民币元文本。
const costDraftFromSKU = (sku: ItemSKU): string => sku.cost_cents === null || sku.cost_cents === undefined ? '' : (sku.cost_cents / 100).toFixed(2);

// parseCostDraft 校验人民币成本文本并转换为分；空文本表示清除本地成本。
export const parseCostDraft = (rawValue: string): number | null => {
  // normalized 是去除首尾空白后的成本输入。
  const normalized = rawValue.trim();
  if (normalized === '') return null;
  if (!/^\d+(?:\.\d{1,2})?$/.test(normalized)) throw new Error('成本最多保留两位小数');
  // cents 是四舍五入后的人民币分值。
  const cents = Math.round(Number(normalized) * 100);
  if (!Number.isSafeInteger(cents) || cents < 0) throw new Error('成本金额超出支持范围');
  return cents;
};

// itemPriceCents 将单规格商品价格文本转换为本地成本弹窗使用的售价分值。
const itemPriceCents = (item: Item): number => {
  // match 是价格文本中的首个非负人民币数值。
  const match = String(item.item_price || '').replace(/,/g, '').match(/\d+(?:\.\d{1,2})?/);
  return match ? Math.round(Number(match[0]) * 100) : 0;
};

// editableCostRows 返回平台多规格 SKU；单规格商品则返回已有或临时构造的本地隐式成本行。
const editableCostRows = (item: Item): ItemSKU[] => {
  // platformSKUs 是当前商品的真实平台 SKU。
  const platformSKUs = orderedItemSKUs(item);
  if (platformSKUs.length > 0) return platformSKUs;
  // existingLocalCost 是此前已经保存的单规格隐式成本行。
  const existingLocalCost = localItemCostSKU(item);
  if (existingLocalCost) return [existingLocalCost];
  return [{ sku_id: defaultItemSKUID, inventory_id: '', properties: [], price_cents: itemPriceCents(item), quantity: 0, initial_quantity: 0, enabled: true, sort_order: 0, cost_cents: null, synced_at: 0, local_only: true }];
};

// skuDisplayName 将销售属性组合为稳定的规格文本。
const skuDisplayName = (sku: ItemSKU): string => sku.properties.map(/* propertyLabelMapper 组合单个规格名称和值。 */ property => property.value || property.name).filter(Boolean).join(' / ') || '单规格商品';

// formatSyncedAt 将 Unix 秒格式化为本地库存同步时间。
const formatSyncedAt = (skus: ItemSKU[]): string => {
  // latest 是当前平台 SKU 集合中最新的同步 Unix 秒。
  const latest = Math.max(0, ...skus.filter(/* syncedPlatformSKUFilter 排除本地隐式成本行。 */ sku => !sku.local_only).map(/* skuSyncTimeMapper 读取单个 SKU 同步时间。 */ sku => sku.synced_at || 0));
  if (latest === 0) return '同步时间未知';
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(new Date(latest * 1000));
};

// ItemSKUDetailsModal 展示平台库存，并通过右下角按钮原子保存全部本地成本。
export const ItemSKUDetailsModal: React.FC<ItemSKUDetailsModalProps> = ({ item, onClose, onCostsSaved, onSaveFailed }) => {
  // rows 是当前弹窗持有的多规格 SKU 或单规格隐式成本行。
  const [rows, setRows] = useState<ItemSKU[]>(() => editableCostRows(item));
  // drafts 保存每行当前成本输入的人民币元文本。
  const [drafts, setDrafts] = useState<Record<string, string>>(() => Object.fromEntries(editableCostRows(item).map(/* costDraftEntryMapper 创建成本草稿。 */ sku => [sku.sku_id, costDraftFromSKU(sku)])));
  // saving 表示统一成本保存请求正在执行。
  const [saving, setSaving] = useState(false);
  // saved 表示当前草稿已完成一次原子保存。
  const [saved, setSaved] = useState(false);
  // error 保存统一保存失败提示。
  const [error, setError] = useState('');
  // fieldErrors 保存逐行成本格式错误。
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  // controllerRef 拥有当前统一保存请求的取消器。
  const controllerRef = useRef<AbortController | null>(null);

  // item 变化时重建成本行和草稿，避免上一个商品状态泄漏。
  useEffect(/* itemResetEffect 同步当前商品成本表单。 */ () => {
    // nextRows 是新商品的多规格或单规格成本行。
    const nextRows = editableCostRows(item);
    setRows(nextRows);
    setDrafts(Object.fromEntries(nextRows.map(/* costDraftEntryMapper 创建新商品成本草稿。 */ sku => [sku.sku_id, costDraftFromSKU(sku)])));
    setSaving(false);
    setSaved(false);
    setError('');
    setFieldErrors({});
  }, [item]);

  // 弹窗卸载时取消统一保存请求，晚到响应不得写入已关闭界面。
  useEffect(/* requestOwnerEffect 建立统一成本请求的组件生命周期。 */ () => {
    return /* requestCleanupEffect 卸载时终止仍在执行的统一成本请求。 */ () => {
      controllerRef.current?.abort();
      controllerRef.current = null;
    };
  }, []);

  // platformSKUs 是排除本地隐式成本行后的真实平台 SKU。
  const platformSKUs = useMemo(/* platformSKUsCalculator 筛选真实平台 SKU。 */ () => rows.filter(/* platformSKUFilter 排除本地隐式成本行。 */ sku => !sku.local_only), [rows]);
  // singleMode 表示当前商品未标记为多规格，只配置一个商品级单件成本。
  const singleMode = item.is_multi_spec !== true || platformSKUs.length === 0;
  // priceRange 是多规格价格区间或单规格售价。
  const priceRange = useMemo(/* priceRangeCalculator 计算弹窗售价。 */ () => singleMode ? formatCents(rows[0]?.price_cents || itemPriceCents(item)) : itemPriceRange({ ...item, skus: platformSKUs }), [item, platformSKUs, rows, singleMode]);
  // remainingQuantity 是多规格商品当前剩余总库存。
  const remainingQuantity = useMemo(/* remainingQuantityCalculator 汇总平台实时库存。 */ () => itemRemainingQuantity({ ...item, skus: platformSKUs }), [item, platformSKUs]);
  // hasChanges 表示至少一行草稿与已保存成本不同。
  const hasChanges = rows.some(/* changedCostDetector 比较成本草稿和当前模型。 */ sku => drafts[sku.sku_id] !== costDraftFromSKU(sku));

  // updateCostDraft 写入指定 SKU 的人民币元草稿，并清除旧反馈。
  const updateCostDraft = (skuId: string, value: string): void => {
    setDrafts(/* previousDrafts 替换指定 SKU 草稿。 */ previousDrafts => ({ ...previousDrafts, [skuId]: value }));
    setSaved(false);
    setError('');
    setFieldErrors(/* previousErrors 清除指定 SKU 格式错误。 */ previousErrors => ({ ...previousErrors, [skuId]: '' }));
  };

  // saveAllCosts 校验全部草稿后通过一个本地事务原子保存。
  const saveAllCosts = async (): Promise<void> => {
    // nextErrors 保存本次全部草稿的格式错误。
    const nextErrors: Record<string, string> = {};
    // costs 保存即将统一提交的全部可空成本分值。
    const costs: SavedSKUCost[] = [];
    for (const /* sku 是当前待校验的成本行。 */ sku of rows) {
      try {
        costs.push({ sku_id: sku.sku_id, cost_cents: parseCostDraft(drafts[sku.sku_id] || '') });
      } catch (/* validationError 是当前成本草稿的格式错误。 */ validationError: unknown) {
        nextErrors[sku.sku_id] = validationError instanceof Error ? validationError.message : '成本格式错误';
      }
    }
    setFieldErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;
    controllerRef.current?.abort();
    // controller 是本次统一保存请求的取消器。
    const controller = new AbortController();
    controllerRef.current = controller;
    setSaving(true);
    setSaved(false);
    setError('');
    try {
      await updateItemSKUCosts(item.cookie_id, item.item_id, costs, { signal: controller.signal });
      if (controller.signal.aborted) return;
      setRows(/* currentRows 写入本次原子保存后的成本。 */ currentRows => currentRows.map(/* currentRow 匹配已保存成本。 */ currentRow => ({ ...currentRow, cost_cents: costs.find(/* savedCostMatcher 匹配当前 SKU。 */ cost => cost.sku_id === currentRow.sku_id)?.cost_cents ?? null })));
      setSaved(true);
      onCostsSaved(costs);
    } catch (/* saveError 是统一成本接口返回的异常。 */ saveError: unknown) {
      if (!controller.signal.aborted) {
        // message 是同时显示在弹窗和页面短暂提示中的失败原因。
        const message = itemErrorMessage(saveError, '成本保存失败，请重试');
        setError(message);
        onSaveFailed?.(message);
      }
    } finally {
      if (controllerRef.current === controller) controllerRef.current = null;
      if (!controller.signal.aborted) setSaving(false);
    }
  };

  // handleOverlayClick 只允许点击遮罩本身关闭。
  const handleOverlayClick: React.MouseEventHandler<HTMLDivElement> = event => {
    if (event.target === event.currentTarget) onClose();
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-end justify-center bg-slate-950/60 p-0 backdrop-blur-sm md:items-center md:p-5" onMouseDown={handleOverlayClick}>
      <section role="dialog" aria-modal="true" aria-labelledby="item-sku-modal-title" className="max-h-[88vh] w-full max-w-3xl overflow-y-auto rounded-t-3xl bg-white shadow-2xl md:rounded-3xl">
        <header className="sticky top-0 z-10 flex items-start justify-between gap-4 border-b border-slate-100 bg-white px-5 py-5 md:px-6">
          <div className="min-w-0"><h3 id="item-sku-modal-title" className="text-lg font-extrabold leading-7 text-slate-900">{item.item_title}</h3><div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-500"><span>商品 ID {item.item_id}</span>{!singleMode && <span className="inline-flex items-center gap-1 font-bold text-emerald-600"><span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />平台剩余库存</span>}<span className="font-bold text-blue-600">成本仅保存在本地</span></div></div>
          <button type="button" onClick={onClose} aria-label="关闭成本设置" className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-slate-50 text-slate-500 hover:bg-slate-100"><X className="h-4 w-4" /></button>
        </header>

        {singleMode ? <div className="px-5 py-5 md:px-6">
          <div className="rounded-2xl bg-slate-50 p-4"><span className="text-[11px] font-bold text-slate-500">单规格商品售价</span><strong className="mt-1 block text-lg text-slate-900">{priceRange}</strong></div>
          <label className="mt-5 block text-xs font-extrabold text-slate-700" htmlFor="single-item-cost">本地单件成本</label>
          <div className="relative mt-2 max-w-sm"><span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">¥</span><input id="single-item-cost" inputMode="decimal" value={drafts[rows[0].sku_id] || ''} onChange={/* singleCostChange 更新单规格商品成本草稿。 */ event => updateCostDraft(rows[0].sku_id, event.target.value)} placeholder="未填写" className="w-full rounded-xl border border-slate-200 py-3 pl-8 pr-3 text-sm font-bold text-slate-900 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" /></div>
          {fieldErrors[rows[0].sku_id] && <p role="alert" className="mt-2 text-xs font-bold text-red-600">{fieldErrors[rows[0].sku_id]}</p>}
        </div> : <>
          <div className="grid grid-cols-2 gap-2.5 px-5 py-4 md:grid-cols-3 md:px-6"><div className="col-span-2 rounded-2xl bg-slate-50 px-4 py-3 md:col-span-1"><span className="text-[11px] font-bold text-slate-500">规格类型</span><strong className="mt-1 block text-sm text-slate-900">{itemSpecName({ ...item, skus: platformSKUs })} · {platformSKUs.length} 个</strong></div><div className="rounded-2xl bg-slate-50 px-4 py-3"><span className="text-[11px] font-bold text-slate-500">剩余总库存</span><strong className="mt-1 block text-sm text-slate-900">{remainingQuantity}</strong></div><div className="rounded-2xl bg-slate-50 px-4 py-3"><span className="text-[11px] font-bold text-slate-500">价格区间</span><strong className="mt-1 block text-sm text-slate-900">{priceRange || '-'}</strong></div></div>
          <div className="hidden px-6 pb-1 md:block"><table className="w-full text-left text-xs"><thead><tr className="border-b border-slate-100 text-[11px] font-bold text-slate-500"><th className="px-2 py-3">规格</th><th className="px-2 py-3 text-right">售价</th><th className="px-2 py-3 text-right">剩余库存</th><th className="px-2 py-3 text-right">初始库存</th><th className="px-2 py-3 text-right">单件成本</th></tr></thead><tbody>{platformSKUs.map(/* skuRowRenderer 渲染桌面 SKU 成本输入。 */ (sku, index) => <tr key={sku.sku_id} className="border-b border-slate-100 last:border-b-0"><td className="px-2 py-3.5"><div className="flex items-center gap-2"><span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 text-[10px] font-extrabold text-blue-700">{String(index + 1).padStart(2, '0')}</span><div><p className="font-extrabold text-slate-900">{skuDisplayName(sku)}</p><span className={`mt-0.5 inline-flex items-center gap-1 text-[10px] font-bold ${sku.enabled ? 'text-emerald-600' : 'text-slate-400'}`}><span className={`h-1.5 w-1.5 rounded-full ${sku.enabled ? 'bg-emerald-500' : 'bg-slate-300'}`} />{sku.enabled ? '在售' : '不可用'}</span></div></div></td><td className="px-2 py-3.5 text-right font-bold text-slate-800">{formatCents(sku.price_cents)}</td><td className="px-2 py-3.5 text-right font-extrabold text-emerald-600">{sku.quantity}</td><td className="px-2 py-3.5 text-right text-slate-500">{sku.initial_quantity}</td><td className="min-w-[150px] px-2 py-3.5"><div className="relative ml-auto w-28"><span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400">¥</span><input aria-label={`${skuDisplayName(sku)}单件成本`} inputMode="decimal" value={drafts[sku.sku_id] || ''} onChange={/* costDraftChange 更新当前 SKU 成本草稿。 */ event => updateCostDraft(sku.sku_id, event.target.value)} placeholder="未填写" className="w-full rounded-lg border border-slate-200 bg-white py-2 pl-7 pr-2 text-right text-xs font-bold text-slate-900 outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" /></div>{fieldErrors[sku.sku_id] && <p role="alert" className="mt-1 text-right text-[10px] font-bold text-red-600">{fieldErrors[sku.sku_id]}</p>}</td></tr>)}</tbody></table></div>
          <div className="space-y-3 px-5 pb-2 md:hidden">{platformSKUs.map(/* skuCardRenderer 渲染移动端 SKU 成本表单。 */ (sku, index) => <article key={sku.sku_id} className="border-b border-slate-100 py-3 last:border-b-0"><div className="flex items-center justify-between gap-3"><div className="flex items-center gap-2"><span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 text-[10px] font-extrabold text-blue-700">{String(index + 1).padStart(2, '0')}</span><strong className="text-xs text-slate-900">{skuDisplayName(sku)}</strong></div><span className="rounded-full bg-emerald-50 px-2 py-1 text-[10px] font-bold text-emerald-700">{sku.enabled ? '在售' : '不可用'}</span></div><div className="mt-3 grid grid-cols-3 gap-3 text-[11px]"><div><span className="block text-slate-500">售价</span><strong className="mt-1 block text-slate-900">{formatCents(sku.price_cents)}</strong></div><div><span className="block text-slate-500">剩余库存</span><strong className="mt-1 block text-emerald-600">{sku.quantity}</strong></div><div><span className="block text-slate-500">初始库存</span><strong className="mt-1 block text-slate-600">{sku.initial_quantity}</strong></div></div><label className="mt-3 block text-[11px] font-bold text-slate-600" htmlFor={`sku-cost-${sku.sku_id}`}>本地单件成本</label><div className="relative mt-1.5"><span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">¥</span><input id={`sku-cost-${sku.sku_id}`} inputMode="decimal" value={drafts[sku.sku_id] || ''} onChange={/* mobileCostDraftChange 更新当前 SKU 成本草稿。 */ event => updateCostDraft(sku.sku_id, event.target.value)} placeholder="未填写" className="w-full rounded-xl border border-slate-200 py-2.5 pl-8 pr-3 text-sm font-bold outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" /></div>{fieldErrors[sku.sku_id] && <p role="alert" className="mt-1.5 text-[10px] font-bold text-red-600">{fieldErrors[sku.sku_id]}</p>}</article>)}</div>
          <div className="mx-5 my-4 flex items-start gap-2 rounded-xl bg-slate-100 px-3 py-2.5 text-[11px] text-slate-600 md:mx-6"><Database className="mt-0.5 h-4 w-4 shrink-0" /><span>库存来自卖家商品编辑接口 · 同步于 {formatSyncedAt(platformSKUs)}；单件成本只保存在 ERP 本地，不会写回闲鱼。</span></div>
        </>}

        <footer className="sticky bottom-0 flex items-center justify-between gap-3 border-t border-slate-100 bg-white px-5 py-4 md:px-6"><div aria-live="polite" className="min-w-0 text-xs font-bold">{error ? <span role="alert" className="text-red-600">{error}</span> : saved ? <span className="inline-flex items-center gap-1 text-emerald-600"><Check className="h-3.5 w-3.5" />成本已保存</span> : null}</div><div className="flex shrink-0 gap-2"><button type="button" onClick={onClose} className="rounded-xl border border-slate-200 px-5 py-2.5 text-xs font-extrabold text-slate-700 hover:bg-slate-50">关闭</button><button type="button" disabled={saving || !hasChanges} onClick={/* saveAllCostsClick 原子保存全部成本。 */ () => void saveAllCosts()} className="inline-flex min-w-[108px] items-center justify-center gap-1.5 rounded-xl bg-blue-600 px-5 py-2.5 text-xs font-extrabold text-white hover:bg-blue-700 disabled:opacity-50"><Save className="h-3.5 w-3.5" />{saving ? '保存中…' : '保存成本'}</button></div></footer>
      </section>
    </div>
  );
};
