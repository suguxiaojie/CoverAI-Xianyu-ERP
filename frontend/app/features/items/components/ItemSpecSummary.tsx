import { CircleDollarSign,Layers3,ListTree } from 'lucide-react';
import React from 'react';
import type { Item,ItemSKU } from '../api';

// defaultItemSKUID 是单规格商品仅在 ERP 本地使用的隐式成本标识。
export const defaultItemSKUID = '__default__';

// orderedItemSKUs 返回按平台顺序稳定排列的真实 SKU，排除单规格商品的本地隐式成本行。
export const orderedItemSKUs = (item: Item): ItemSKU[] => [...(item.skus || [])].filter(/* platformSKUFilter 排除本地隐式成本行。 */ sku => !sku.local_only).sort(/* skuOrderComparator 按平台顺序和 SKU 标识稳定排序。 */ (left, right) => left.sort_order - right.sort_order || left.sku_id.localeCompare(right.sku_id));

// localItemCostSKU 返回单规格商品已有的本地隐式成本行。
export const localItemCostSKU = (item: Item): ItemSKU | undefined => (item.skus || []).find(/* localCostFilter 匹配本地隐式成本行。 */ sku => sku.local_only || sku.sku_id === defaultItemSKUID);

// formatCents 将人民币分值格式化为不带无意义小数零的金额文本。
export const formatCents = (cents: number): string => {
  // yuan 是分值转换后的人民币元数值。
  const yuan = cents / 100;
  return `¥${Number.isInteger(yuan) ? yuan.toFixed(0) : yuan.toFixed(2)}`;
};

// itemPriceRange 返回多规格商品的最低与最高售价区间；无 SKU 时返回空文本。
export const itemPriceRange = (item: Item): string => {
  // prices 保存有效 SKU 的非负价格分值。
  const prices = orderedItemSKUs(item).map(/* skuPriceMapper 读取单个 SKU 售价。 */ sku => sku.price_cents).filter(/* validPriceFilter 排除异常负值。 */ price => price >= 0);
  if (prices.length === 0) return '';
  // minimum 是当前 SKU 集合的最低售价。
  const minimum = Math.min(...prices);
  // maximum 是当前 SKU 集合的最高售价。
  const maximum = Math.max(...prices);
  return minimum === maximum ? formatCents(minimum) : `${formatCents(minimum)}–${formatCents(maximum)}`;
};

// itemRemainingQuantity 汇总卖家编辑详情返回的当前 SKU 剩余库存。
export const itemRemainingQuantity = (item: Item): number => orderedItemSKUs(item).reduce(/* quantityReducer 累加每个 SKU 的非负剩余库存。 */ (total, sku) => total + Math.max(0, sku.quantity), 0);

// itemSpecValues 返回卡片摘要使用的规格值，按平台顺序去重。
export const itemSpecValues = (item: Item): string[] => {
  // values 保存规格值并通过 Set 维持首次出现顺序。
  const values = new Set<string>();
  orderedItemSKUs(item).forEach(/* skuPropertyCollector 收集每个 SKU 的规格值。 */ sku => sku.properties.forEach(/* propertyValueCollector 收集非空规格值。 */ property => {
    if (property.value.trim()) values.add(property.value.trim());
  }));
  return [...values];
};

// itemSpecName 返回商品首个销售规格名称；多维规格使用斜线连接。
export const itemSpecName = (item: Item): string => {
  // names 保存规格名称并保持平台首次出现顺序。
  const names = new Set<string>();
  orderedItemSKUs(item).forEach(/* skuPropertyCollector 收集每个 SKU 的规格名称。 */ sku => sku.properties.forEach(/* propertyNameCollector 收集非空规格名称。 */ property => {
    if (property.name.trim()) names.add(property.name.trim());
  }));
  return [...names].join(' / ') || '规格';
};

// ItemSpecBadge 在商品图片上显示紧凑的 SKU 数量角标。
export const ItemSpecBadge: React.FC<{ /** item 是待展示的商品。 */ item: Item }> = ({ item }) => {
  // skuCount 是当前商品有效 SKU 数量。
  const skuCount = orderedItemSKUs(item).length;
  if (item.is_multi_spec !== true || skuCount === 0) return null;
  return <span className="absolute bottom-1.5 right-1.5 inline-flex items-center gap-1 rounded-md bg-blue-600/90 px-2 py-1 text-[10px] font-extrabold text-white shadow-md backdrop-blur"><Layers3 className="h-3 w-3" />{skuCount} 个规格</span>;
};

// ItemSpecSummary 在商品卡片中展示规格名称、值摘要和实时剩余总库存。
export const ItemSpecSummary: React.FC<{
  /** item 是待展示的多规格商品。 */
  item: Item;
  /** onOpen 打开完整规格和成本弹窗。 */
  onOpen: () => void;
}> = ({ item, onOpen }) => {
  // skus 是当前商品按平台顺序排列的 SKU。
  const skus = orderedItemSKUs(item);
  if (item.is_multi_spec !== true || skus.length === 0) {
    // localCost 是单规格商品真实单 SKU 或本地隐式行已经保存的成本。
    const localCost = (skus[0] || localItemCostSKU(item))?.cost_cents;
    return <div className="mb-2"><button type="button" onClick={onOpen} className="flex w-full items-center justify-center gap-1.5 rounded-lg border border-blue-200 bg-blue-50 px-2.5 py-2 text-[11px] font-extrabold text-blue-700 hover:bg-blue-100"><CircleDollarSign className="h-3.5 w-3.5" />{localCost === null || localCost === undefined ? '设置成本' : `成本 ${formatCents(localCost)}`}</button></div>;
  }
  // values 是卡片单行展示的规格值摘要。
  const values = itemSpecValues(item);
  return (
    <div className="mb-2 space-y-2">
      <div className="rounded-xl bg-slate-50 px-2.5 py-2">
        <div className="flex items-center justify-between gap-2 text-[10px] font-extrabold text-slate-700">
          <span className="truncate">{itemSpecName(item)} · {skus.length} 个规格</span>
          <span className="shrink-0 text-emerald-600">剩余 {itemRemainingQuantity(item)}</span>
        </div>
        <p className="mt-1 truncate text-[10px] text-slate-500">{values.join(' · ')}</p>
      </div>
      <button type="button" onClick={onOpen} className="ios-btn-primary flex w-full items-center justify-center gap-1.5 rounded-lg px-2.5 py-2 text-[11px] font-extrabold">
        <ListTree className="h-3.5 w-3.5" />查看 {skus.length} 个规格
      </button>
    </div>
  );
};
