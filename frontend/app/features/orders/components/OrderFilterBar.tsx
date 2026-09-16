import { ChevronDown,Search,User as UserIcon } from 'lucide-react';
import React from 'react';
import type { AccountDetail } from '../api';
import { orderStatusOptions } from '../state';
import type { OrderAmountRange,OrderCreatedRange } from '../types';
import { OrderAmountFilter } from './OrderAmountFilter';
import { OrderDateFilter } from './OrderDateFilter';

// OrderFilterBarProps 描述订单状态、账号和文本筛选所需的页面状态。
export interface OrderFilterBarProps {
  // filter 是当前订单状态筛选值。
  filter: string;
  // onFilterChange 响应订单状态筛选切换。
  onFilterChange: (value: string) => void;
  // accountFilter 是当前账号筛选值。
  accountFilter: string;
  // onAccountFilterChange 响应账号筛选切换。
  onAccountFilterChange: (value: string) => void;
  // createdRange 是当前已经应用的订单创建时间范围。
  createdRange: OrderCreatedRange;
  // onCreatedRangeChange 响应快捷或自定义时间范围应用。
  onCreatedRangeChange: (value: OrderCreatedRange) => void;
  // amountRange 是当前已经应用的实付金额范围。
  amountRange: OrderAmountRange;
  // onAmountRangeChange 响应实付金额范围应用或清除。
  onAmountRangeChange: (value: OrderAmountRange) => void;
  // accounts 是账号下拉框的数据源。
  accounts: AccountDetail[];
  // accountName 将账号 ID 转换为展示名称。
  accountName: (cookieId: string) => string;
  // searchText 是搜索框当前输入值。
  searchText: string;
  // onSearchChange 响应订单搜索输入。
  onSearchChange: (value: string) => void;
}

// OrderFilterBar 渲染订单状态、账号和关键词筛选工具栏。
export const OrderFilterBar: React.FC<OrderFilterBarProps> = ({
  filter,
  onFilterChange,
  accountFilter,
  onAccountFilterChange,
  createdRange,
  onCreatedRangeChange,
  amountRange,
  onAmountRangeChange,
  accounts,
  accountName,
  searchText,
  onSearchChange,
}) => {
  // handleStatusClick 将用户选择的状态传回订单页面。
  const handleStatusClick = (event: React.MouseEvent<HTMLButtonElement>) => onFilterChange(event.currentTarget.dataset.status || 'all');
  // handleAccountChange 将用户选择的账号传回订单页面。
  const handleAccountChange = (event: React.ChangeEvent<HTMLSelectElement>) => onAccountFilterChange(event.target.value);
  // handleSearchChange 将用户输入传回订单页面。
  const handleSearchChange = (event: React.ChangeEvent<HTMLInputElement>) => onSearchChange(event.target.value);

  return (
    <div className="flex flex-col items-center justify-between gap-4 border-b border-gray-50 bg-surface-muted p-4 xl:flex-row xl:flex-wrap">
      <div className="flex max-w-full shrink-0 gap-1 overflow-x-auto rounded-xl bg-gray-200/50 p-1">
        {orderStatusOptions.map(
          // option 是当前订单状态筛选标签配置。
          option => (
            <button
              key={option.key}
              data-status={option.key}
              onClick={handleStatusClick}
              className={`px-5 py-2 rounded-lg text-sm font-bold transition-all whitespace-nowrap ${filter === option.key ? 'bg-white text-black shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
            >
              {option.label}
            </button>
          ),
        )}
      </div>
		<div className="ml-auto flex w-full flex-col gap-3 sm:flex-row sm:flex-wrap xl:w-auto xl:flex-nowrap">
				<OrderDateFilter value={createdRange} onChange={onCreatedRangeChange} />
        <OrderAmountFilter value={amountRange} onChange={onAmountRangeChange} />
        <div className="flex h-[42px] w-full items-center gap-2 rounded-xl bg-white px-4 text-gray-700 shadow-sm transition-shadow focus-within:ring-4 focus-within:ring-blue-100 sm:w-52">
          <UserIcon className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
          <select
            aria-label="按账号筛选订单"
            value={accountFilter}
            onChange={handleAccountChange}
            className="min-w-0 flex-1 appearance-none bg-transparent text-sm font-medium outline-none"
          >
            <option value="">全部账号</option>
            {accounts.map(
              // account 是当前账号筛选下拉项。
              account => <option key={account.id} value={account.id}>{accountName(account.id)}</option>,
            )}
          </select>
          <ChevronDown className="h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
        </div>
        <div className="relative group">
          <Search className="w-4 h-4 absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 group-focus-within:text-brand transition-colors" />
          <input
            type="text"
            placeholder="搜索订单号/商品/买家..."
            value={searchText}
            onChange={handleSearchChange}
            className="ios-input pl-10 pr-4 py-2.5 rounded-xl w-64 bg-white border-none shadow-sm focus:ring-0"
          />
        </div>
      </div>
    </div>
  );
};
