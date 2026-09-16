import { BadgeDollarSign,ChevronDown,X } from 'lucide-react';
import React from 'react';
import type { OrderAmountRange } from '../types';

// OrderAmountFilterProps 描述已应用金额范围和向订单查询层提交新范围的回调。
interface OrderAmountFilterProps {
  // value 是当前已应用的实付金额元范围和可见摘要。
  value: OrderAmountRange;
  // onChange 在应用或清除金额范围时更新订单查询条件。
  onChange: (value: OrderAmountRange) => void;
}

// amountInputPattern 接受非负普通十进制元金额且最多两位小数。
const amountInputPattern = /^(0|[1-9][0-9]*)(?:\.([0-9]{1,2}))?$/;

// formatAmountRangeLabel 把已经校验的上下界格式化为紧凑入口摘要。
const formatAmountRangeLabel = (minAmount: string, maxAmount: string): string => {
  if (minAmount && maxAmount) return `¥${minAmount}–¥${maxAmount}`;
  if (minAmount) return `≥ ¥${minAmount}`;
  if (maxAmount) return `≤ ¥${maxAmount}`;
  return '金额范围';
};

// OrderAmountFilter 渲染支持单边或双边输入的实付金额筛选弹层。
export const OrderAmountFilter: React.FC<OrderAmountFilterProps> = ({ value, onChange }) => {
  // menuOpen 表示金额筛选弹层是否展开。
  const [menuOpen, setMenuOpen] = React.useState(false);
  // draftMin 保存尚未应用的最低金额输入。
  const [draftMin, setDraftMin] = React.useState(value.minAmount);
  // draftMax 保存尚未应用的最高金额输入。
  const [draftMax, setDraftMax] = React.useState(value.maxAmount);
  // validationError 保存当前草稿的格式或范围错误。
  const [validationError, setValidationError] = React.useState('');

  // handleMenuToggle 打开弹层时同步已应用值，关闭时不改变当前查询。
  const handleMenuToggle = () => {
    setMenuOpen(/* amountMenuToggle 反转金额弹层展开状态。 */ currentOpen => {
      if (!currentOpen) {
        setDraftMin(value.minAmount);
        setDraftMax(value.maxAmount);
        setValidationError('');
      }
      return !currentOpen;
    });
  };
  // handleMinChange 更新最低金额草稿并清除旧校验提示。
  const handleMinChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setDraftMin(event.target.value);
    setValidationError('');
  };
  // handleMaxChange 更新最高金额草稿并清除旧校验提示。
  const handleMaxChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setDraftMax(event.target.value);
    setValidationError('');
  };
  // handleApply 校验金额格式和顺序后提交当前范围。
  const handleApply = () => {
    // minAmount 是去除首尾空格后的最低金额草稿。
    const minAmount = draftMin.trim();
    // maxAmount 是去除首尾空格后的最高金额草稿。
    const maxAmount = draftMax.trim();
    if (minAmount && !amountInputPattern.test(minAmount) || maxAmount && !amountInputPattern.test(maxAmount)) {
      setValidationError('请输入最多两位小数的非负金额');
      return;
    }
    // minValue 是用于前端即时比较的最低元金额数值；服务端仍以整数分执行权威校验。
    const minValue = minAmount ? Number(minAmount) : null;
    // maxValue 是用于前端即时比较的最高元金额数值；服务端仍以整数分执行权威校验。
    const maxValue = maxAmount ? Number(maxAmount) : null;
    if (minValue !== null && !Number.isFinite(minValue) || maxValue !== null && !Number.isFinite(maxValue)) {
      setValidationError('金额超出支持范围');
      return;
    }
    if (minValue !== null && maxValue !== null && minValue > maxValue) {
      setValidationError('最高金额不能低于最低金额');
      return;
    }
    onChange({ minAmount, maxAmount, label: formatAmountRangeLabel(minAmount, maxAmount) });
    setMenuOpen(false);
  };
  // handleClear 清除金额上下界并立即恢复全部金额订单。
  const handleClear = () => {
    setDraftMin('');
    setDraftMax('');
    setValidationError('');
    onChange({ minAmount: '', maxAmount: '', label: '金额范围' });
    setMenuOpen(false);
  };

  return (
    <div className="relative">
      <button type="button" onClick={handleMenuToggle} className={`ios-input flex w-full items-center gap-2 rounded-xl border-none bg-white px-4 py-2.5 text-left text-sm font-medium shadow-sm transition-colors sm:w-40 ${value.minAmount || value.maxAmount ? 'text-brand' : 'text-gray-700'}`} aria-haspopup="dialog" aria-expanded={menuOpen} aria-label={`按实付金额筛选，当前${value.label}`} title={value.label}>
        <BadgeDollarSign className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
        <span className="min-w-0 flex-1 truncate">{value.label}</span>
        <ChevronDown className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${menuOpen ? 'rotate-180' : ''}`} aria-hidden="true" />
      </button>
      {menuOpen && (
        <div role="dialog" aria-label="实付金额筛选" className="absolute right-0 top-full z-40 mt-2 w-80 rounded-2xl border border-gray-100 bg-white p-4 shadow-2xl shadow-gray-200/70">
          <div className="mb-4 flex items-center justify-between"><div><p className="text-sm font-black text-gray-900">实付金额范围</p><p className="mt-1 text-xs text-gray-400">支持只填最低或最高金额</p></div><button type="button" onClick={handleMenuToggle} className="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700" aria-label="关闭金额筛选"><X className="h-4 w-4" /></button></div>
          <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
            <label className="relative"><span className="sr-only">最低金额</span><span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">¥</span><input type="text" inputMode="decimal" value={draftMin} onChange={handleMinChange} placeholder="最低" aria-label="最低金额" className="ios-input w-full rounded-xl py-2.5 pl-7 pr-3 text-sm outline-none" /></label>
            <span className="text-sm font-bold text-gray-300">—</span>
            <label className="relative"><span className="sr-only">最高金额</span><span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">¥</span><input type="text" inputMode="decimal" value={draftMax} onChange={handleMaxChange} placeholder="最高" aria-label="最高金额" className="ios-input w-full rounded-xl py-2.5 pl-7 pr-3 text-sm outline-none" /></label>
          </div>
          {validationError && <p role="alert" className="mt-2 text-xs font-bold text-red-500">{validationError}</p>}
          <div className="mt-4 flex justify-end gap-2"><button type="button" onClick={handleClear} className="rounded-xl px-4 py-2 text-sm font-bold text-gray-500 hover:bg-gray-100">清除</button><button type="button" onClick={handleApply} className="rounded-xl bg-gray-950 px-4 py-2 text-sm font-bold text-white hover:bg-gray-800">应用筛选</button></div>
        </div>
      )}
    </div>
  );
};
