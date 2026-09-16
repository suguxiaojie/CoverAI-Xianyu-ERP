import { Activity,AlertCircle,ChevronRight,ExternalLink,PackageCheck } from 'lucide-react';
import React,{ useEffect,useRef,useState } from 'react';
import { Cell,Legend,Pie,PieChart,ResponsiveContainer,Tooltip } from 'recharts';
import { TimeRange } from '../../../../dateRange';
import { formatLocalDateTime } from '../../../../dateTime';
import { OrderStatus } from '../api';
import type { DashboardAccountSummary } from '../api';
import { DashboardProfitChart } from '../DashboardProfitChart';
import { DashboardDateRangePicker } from '../DashboardDateRangePicker';
import { HistoricalCostBackfillModal } from '../HistoricalCostBackfillModal';
import { DashboardTrendChart } from '../DashboardTrendChart';
import { useDashboard } from '../hooks';
import type { DashboardRuntimeSummary } from '../state';

// cssColor 状态颜色样式。
const cssColor = (token: string, alpha?: number) => (
  alpha === undefined
    ? `rgb(var(--color-${token}))`
    : `rgb(var(--color-${token}) / ${alpha})`
);

// 状态徽章组件
export const StatusBadge: React.FC<{ /** status 表示状态。 */ status: OrderStatus }> = ({ status }) => {
  // styles 样式表。
  const styles = {
    processing: 'bg-blue-100 text-blue-800',
    pending_ship: 'bg-brand text-white',
    shipped: 'bg-blue-100 text-blue-700',
		received: 'bg-cyan-100 text-cyan-700',
    completed: 'bg-green-100 text-green-700',
    cancelled: 'bg-gray-100 text-gray-500',
    refunding: 'bg-red-100 text-red-600',
		refunded: 'bg-fuchsia-100 text-fuchsia-700',
    unknown: 'bg-gray-100 text-gray-500',
  };

  // labels labels，负责当前功能中的对应处理。
  const labels = {
    processing: '处理中',
    pending_ship: '待发货',
    shipped: '已发货',
		received: '已收货',
    completed: '已完成',
    cancelled: '已取消',
    refunding: '退款中',
		refunded: '已退款',
    unknown: '未知',
  };

  return (
    <span className={`inline-flex items-center justify-center whitespace-nowrap px-3 py-1.5 rounded-lg text-xs leading-none font-bold ${styles[status] || styles.cancelled}`}>
      {labels[status] || status}
    </span>
  );
};

// DashboardMetricProps 描述紧凑指标带中的单个真实经营指标。
interface DashboardMetricProps {
  /** title 是指标口径名称。 */
  title: string;
  /** value 是当前范围的格式化值。 */
  value: string | number;
  /** detail 是可选的趋势或口径说明。 */
  detail?: string;
  /** tone 控制强调色，不改变指标业务语义。 */
  tone?: 'default' | 'success' | 'warning';
}

// DashboardMetric 使用分隔线和排版而非大卡片呈现指标，降低首屏模块占比。
const DashboardMetric: React.FC<DashboardMetricProps> = ({ title, value, detail, tone = 'default' }) => (
  <div className="min-w-0 border-b border-slate-200 px-4 py-3 last:border-b-0 sm:border-b-0 sm:border-r sm:last:border-r-0">
    <p className="truncate text-xs font-medium text-slate-500">{title}</p>
    <div className={`mt-1 truncate text-[clamp(1.45rem,2vw,2rem)] font-black tracking-tight tabular-nums ${tone === 'success' ? 'text-emerald-600' : tone === 'warning' ? 'text-amber-600' : 'text-slate-950'}`}>{value}</div>
    {detail && <p className="mt-1 truncate text-[11px] font-medium text-slate-400">{detail}</p>}
  </div>
);

/** Dashboard 运行状态对应的用户文案与颜色。 */
type RuntimeStatusPresentation = {
  /** label 是顶部状态胶囊显示的主要文案。 */
  label: string;
  /** detail 是需要时显示的在线账号比例。 */
  detail: string;
  /** containerClass 是状态胶囊使用的语义颜色。 */
  containerClass: string;
  /** dotClass 是状态圆点使用的语义颜色和动画。 */
  dotClass: string;
};

/** 将账号运行摘要转换为 Dashboard 顶部可见状态，不把页面能加载等同于账号在线。 */
export const runtimeStatusPresentation = (summary: DashboardRuntimeSummary): RuntimeStatusPresentation => {
  // detail 是存在启用账号时显示的真实在线比例。
  const detail = summary.enabledAccounts > 0 ? `${summary.onlineAccounts} / ${summary.enabledAccounts} 在线` : '';
  switch (summary.state) {
  case 'healthy': return { label: '系统正常运行', detail, containerClass: 'bg-green-50 text-green-700 border-green-100', dotClass: 'bg-green-500 animate-pulse' };
  case 'partial': return { label: '部分账号异常', detail, containerClass: 'bg-amber-50 text-amber-700 border-amber-100', dotClass: 'bg-amber-500' };
  case 'auth_expired': return { label: '登录已过期', detail, containerClass: 'bg-red-50 text-red-700 border-red-100', dotClass: 'bg-red-500' };
  case 'verification_required': return { label: '需要人工验证', detail, containerClass: 'bg-amber-50 text-amber-700 border-amber-100', dotClass: 'bg-amber-500' };
  case 'runtime_conflict': return { label: '账号运行冲突', detail, containerClass: 'bg-red-50 text-red-700 border-red-100', dotClass: 'bg-red-500' };
  case 'offline': return { label: '账号服务异常', detail, containerClass: 'bg-red-50 text-red-700 border-red-100', dotClass: 'bg-red-500' };
  case 'no_enabled_accounts': return { label: '暂无启用账号', detail: '', containerClass: 'bg-gray-50 text-gray-600 border-gray-100', dotClass: 'bg-gray-400' };
  case 'loading': return { label: '正在读取运行状态', detail: '', containerClass: 'bg-blue-50 text-blue-700 border-blue-100', dotClass: 'bg-blue-500 animate-pulse' };
  default: return { label: '运行状态未知', detail: '', containerClass: 'bg-gray-50 text-gray-600 border-gray-100', dotClass: 'bg-gray-400' };
  }
};

/** 将非敏感账号摘要转换为 Dashboard 选择器文案，优先昵称和备注并保留稳定 ID 回退。 */
export const dashboardAccountLabel = (account: DashboardAccountSummary): string => {
	// nickname 是平台账号昵称，可能因资料尚未刷新而为空。
	const nickname = account.nickname?.trim() || '';
	// remark 是用户为店铺设置的本地备注。
	const remark = account.remark?.trim() || '';
	if (nickname && remark && nickname !== remark) return `${nickname}（${remark}）`;
	return nickname || remark || `账号 ${account.id}`;
};

// Dashboard 渲染仪表盘页面组件。
const Dashboard: React.FC = () => {
  // [timeRange, 解构得到当前 Hook 返回的状态和操作函数。
	const [timeRange, setTimeRange] = useState<TimeRange>('7days');
	// selectedAccountID 是当前经营统计的账号范围，空值作为默认的全部账号。
	const [selectedAccountID, setSelectedAccountID] = useState('');
  // [customStartDate, 解构得到当前 Hook 返回的状态和操作函数。
  const [customStartDate, setCustomStartDate] = useState('');
  // [customEndDate, 解构得到当前 Hook 返回的状态和操作函数。
  const [customEndDate, setCustomEndDate] = useState('');
  // [searchTerm, 解构得到当前 Hook 返回的状态和操作函数。
  const [searchTerm, setSearchTerm] = useState('');
  // [customRangeVersion, 解构得到当前 Hook 返回的状态和操作函数。
  const [customRangeVersion, setCustomRangeVersion] = useState(0);
  // [customPickerOpen, setCustomPickerOpen] 控制应用内日期范围弹层，不触发浏览器原生日期面板。
  const [customPickerOpen, setCustomPickerOpen] = useState(false);
  // [backfillOpen, setBackfillOpen] 控制历史成本补全确认与进度弹窗。
  const [backfillOpen, setBackfillOpen] = useState(false);
  // [extendedAnalysisOpen, setExtendedAnalysisOpen] 控制次级分析区展开状态，所有原有模块仍可访问。
  const [extendedAnalysisOpen, setExtendedAnalysisOpen] = useState(false);
  // ordersSectionRef 指向原有参与统计订单模块，待办入口可快速定位而不复制订单能力。
  const ordersSectionRef = useRef<HTMLDivElement | null>(null);
  // dashboard 仪表盘数据。
	const dashboard = useDashboard({ range: timeRange, customStartDate, customEndDate, customRangeVersion, accountID: selectedAccountID });
	// { 解构得到当前 Hook 返回的状态和操作函数。
	const { accounts, data, status, chartData, productSalesData, sourceData: sourceDataData, categoryData: categoryDataData, maxProductSales, trendPercent, selectedRangeLabel, runtimeSummary, refresh } = dashboard;
  // stats 统计概览数据。
  const stats = data?.stats || null;
  // analytics 统计分析数据。
  const analytics = data?.analytics || null;
  // validOrders 有效订单列表。
  const validOrders = data?.validOrders.orders || [];
  // validOrdersTotal 有效数据订单列表总数，负责当前功能中的对应处理。
  const validOrdersTotal = data?.validOrders.total || 0;
  // validOrdersTruncated 有效数据订单列表Truncated，负责当前功能中的对应处理。
  const validOrdersTruncated = data?.validOrders.truncated || false;
  // ordersLoading 订单加载状态。
  const ordersLoading = status.range === 'loading';
  // loadError 加载当前数据（错误）。
  const loadError = status.error;
  // runtimePresentation 是真实账号连接状态对应的顶部文案和颜色。
  const runtimePresentation = runtimeStatusPresentation(runtimeSummary);
  // onlineAccountValue 是在线账号卡片显示的在线数与启用数；加载或未知时避免伪造零在线。
  const onlineAccountValue = runtimeSummary.state === 'loading' || runtimeSummary.state === 'unknown' ? '—' : `${runtimeSummary.onlineAccounts} / ${runtimeSummary.enabledAccounts}`;
  // accountIssueCount 是当前启用但未在线的真实账号数量。
	const accountIssueCount = Math.max(0, runtimeSummary.enabledAccounts - runtimeSummary.onlineAccounts);
	// selectedAccount 是当前选中账号的非敏感摘要，全部账号模式为 undefined。
	const selectedAccount = accounts.find(/* account 是当前匹配选择值的账号摘要。 */ account => account.id === selectedAccountID);
	// selectedAccountLabel 是当前统计范围的用户可见名称。
	const selectedAccountLabel = selectedAccount ? dashboardAccountLabel(selectedAccount) : '全部账号';

	useEffect(/* 当前副作用在账号被删除或归属变化后安全回到全部账号范围。 */ () => {
		if (selectedAccountID && runtimeSummary.state !== 'loading' && !selectedAccount) setSelectedAccountID('');
	}, [runtimeSummary.state, selectedAccount, selectedAccountID]);

  // 颜色配置
  const COLORS = [
    cssColor('brand'),
    cssColor('brand-highlight'),
    cssColor('success-500'),
    cssColor('warning-500'),
    cssColor('accent-500'),
  ];
  // formatCurrency 格式化金额函数。
  const formatCurrency = (value: number) => `¥${Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 2 })}`;

  if (loadError && (!stats || !analytics)) {
    return (
      <div className="p-8 flex flex-col items-center gap-3 text-red-600">
        <AlertCircle className="w-8 h-8" />
        <span>{loadError}</span>
        <button type="button" className="ios-btn-primary px-4 py-2 rounded-xl" onClick={refresh}>重新加载</button>
      </div>
    );
  }
  if (!stats || !analytics) return <div className="p-8 flex justify-center text-gray-400"><Activity className="w-8 h-8 animate-spin text-brand" /></div>;
  // totalOrders 总数订单列表，负责当前功能中的对应处理。
  const totalOrders = analytics.revenue_stats.total_orders || 0;
  // totalAmount 订单总金额。
  const totalAmount = analytics.revenue_stats.total_amount || 0;
  // profitStats 是仅基于精确成本快照计算的商品毛利摘要。
  const profitStats = analytics.profit_stats || { total_orders: totalOrders, covered_orders: 0, unknown_cost_orders: totalOrders, covered_revenue: 0, product_cost: 0, platform_fee: 0, gross_profit: 0, realized_gross_profit: 0, gross_margin: 0, coverage_rate: 0 };
  // itemProfitStats 是至少有一个订单匹配成本的商品毛利排行。
  const itemProfitStats = (analytics.item_profit_stats || []).filter(
    // item 是当前检查成本覆盖数量的商品毛利统计。
    item => item.covered_orders > 0,
  ).slice(0, 8);
  // itemImagesByID 把已加载商品主图按平台商品 ID 建立索引，排行只复用真实图片。
  const itemImagesByID = new Map(data!.items.map(
    // item 是当前建立真实主图索引的商品。
    item => [item.item_id, item.item_image || ''],
  ));

  // timeRangeOptions 保存全部既有时间范围，紧凑控件仍完整保留每个选择。
  const timeRangeOptions = [
    { key: 'today' as TimeRange, label: '今天' },
    { key: 'yesterday' as TimeRange, label: '昨天' },
    { key: '3days' as TimeRange, label: '三天内' },
    { key: '7days' as TimeRange, label: '7天内' },
    { key: '30days' as TimeRange, label: '一个月内' },
    { key: 'custom' as TimeRange, label: '自定义' },
  ];
  // currentRangeDates 使用已完成请求携带的权威日期范围，避免按钮先切换时把旧分析数据与新日期边界混用。
  const currentRangeDates = data!.dateRange;
  // normalizedSearchTerm 归一化当前数据（d搜索条件Term）。
  const normalizedSearchTerm = searchTerm.trim().toLowerCase();
  // filteredValidOrders 过滤后的有效订单列表。
  const filteredValidOrders = validOrders.filter(/* 当前回调处理集合中的单个元素。 */ (order) =>
    order.order_id?.toLowerCase().includes(normalizedSearchTerm) ||
    order.item_id?.toLowerCase().includes(normalizedSearchTerm) ||
    order.item_title?.toLowerCase().includes(normalizedSearchTerm) ||
    order.buyer_id?.toLowerCase().includes(normalizedSearchTerm)
  );

  // selectTimeRange 直接应用快捷范围；自定义只打开弹层，避免空日期提前触发请求。
  const selectTimeRange = (range: TimeRange): void => {
    if (range === 'custom') {
      setCustomPickerOpen(true);
      return;
    }
    setCustomPickerOpen(false);
    setTimeRange(range);
  };

  // applyCustomRange 提交完整自定义日期并触发新一代统计请求。
  const applyCustomRange = (): void => {
    if (!customStartDate || !customEndDate || customStartDate > customEndDate) return;
    setTimeRange('custom');
    setCustomRangeVersion(/* customRangeRequestVersion 使相同日期也可由用户明确重新应用。 */ value => value + 1);
    setCustomPickerOpen(false);
  };

  return (
    <div className="space-y-5 animate-fade-in">
      {loadError && (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-red-100 bg-red-50 px-4 py-2.5 text-sm text-red-700">
          <span>{loadError}</span>
          <button type="button" className="font-bold underline" onClick={refresh}>重试</button>
        </div>
      )}

      <header className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <h2 className="text-2xl font-black tracking-tight text-slate-950">经营概览</h2>
            <div className={'inline-flex items-center gap-1.5 text-xs font-semibold ' + runtimePresentation.containerClass.replace('bg-', 'text-').split(' ')[1]}>
              <span className={'h-2 w-2 rounded-full ' + runtimePresentation.dotClass}></span>
              <span>{runtimePresentation.label}</span>
              {runtimePresentation.detail && <span className="font-medium text-slate-400">· {runtimePresentation.detail}</span>}
            </div>
          </div>
          <p className="mt-1 text-sm text-slate-500">实时掌握店铺经营关键指标，次级分析与订单明细完整保留在下方。</p>
        </div>

		<div className="flex flex-wrap items-center gap-2">
			<label className="relative">
				<span className="sr-only">选择统计账号</span>
				<select aria-label="统计账号" value={selectedAccountID} onChange={/* event 是用户切换经营统计账号的表单事件。 */ event => setSelectedAccountID(event.target.value)} className="h-[34px] min-w-[150px] max-w-[240px] rounded-md border border-slate-200 bg-white px-3 text-xs font-semibold text-slate-700 outline-none transition hover:bg-slate-50 focus:border-blue-400 focus:ring-2 focus:ring-blue-100">
					<option value="">全部账号</option>
					{accounts.map(/* account 是当前渲染为统计选项的非敏感账号摘要。 */ account => <option key={account.id} value={account.id}>{dashboardAccountLabel(account)}</option>)}
				</select>
			</label>
			<div className="relative">
            <div className="inline-flex max-w-full overflow-x-auto rounded-md border border-slate-200 bg-white">
              {timeRangeOptions.map(/* timeRangeRenderer 平铺渲染全部既有日期范围。 */ option => (
                <button key={option.key} type="button" onClick={/* timeRangeAction 切换快捷范围或打开应用内日期弹层。 */ () => selectTimeRange(option.key)} className={timeRange === option.key ? 'shrink-0 bg-blue-600 px-3 py-2 text-xs font-bold text-white' : 'shrink-0 border-l border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 first:border-l-0 hover:bg-slate-50'}>
                  {option.label}
                </button>
              ))}
            </div>
            {customPickerOpen && <DashboardDateRangePicker startDate={customStartDate} endDate={customEndDate} onStartDateChange={setCustomStartDate} onEndDateChange={setCustomEndDate} onApply={applyCustomRange} onCancel={/* customPickerCancelAction 关闭弹层并保留当前草稿。 */ () => setCustomPickerOpen(false)} />}
          </div>
        </div>
      </header>

      <section className="grid overflow-hidden rounded-lg border border-slate-200 bg-white sm:grid-cols-3 xl:grid-cols-6" aria-label="经营利润统计与经营关键指标">
        <DashboardMetric title="有效成交额" value={formatCurrency(analytics.revenue_stats.total_amount)} detail={trendPercent ? '较上周期 ' + trendPercent : '按下单时间'} />
        <DashboardMetric title="预估经营利润" value={formatCurrency(profitStats.gross_profit)} detail="已扣除 1.6% 手续费" />
        <DashboardMetric title="成本覆盖" value={profitStats.coverage_rate.toFixed(1) + '%'} detail={profitStats.covered_orders + ' / ' + profitStats.total_orders + ' 单'} tone={profitStats.coverage_rate >= 80 ? 'success' : 'warning'} />
        <DashboardMetric title="在线账号" value={onlineAccountValue} detail={runtimePresentation.label} />
        <DashboardMetric title="订单数" value={analytics.revenue_stats.total_orders.toLocaleString()} detail={selectedRangeLabel} />
		<DashboardMetric title={selectedAccountID ? '共享库存卡密' : '库存卡密'} value={stats.available_card_stock} detail={selectedAccountID ? '全部账号共用' : '当前可用余量'} tone={stats.available_card_stock > 0 ? 'default' : 'warning'} />
      </section>

      <div className="grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,0.9fr)]">
        <DashboardProfitChart compact dailyProfitStats={analytics.daily_profit_stats || []} selectedRangeLabel={selectedRangeLabel} startDate={currentRangeDates.startDate} endDate={currentRangeDates.endDate} />

        <section className="rounded-lg border border-slate-200 bg-white p-4" aria-labelledby="dashboard-priority-title">
          <div className="flex items-center justify-between border-b border-slate-200 pb-3">
            <div>
              <h3 id="dashboard-priority-title" className="text-base font-black text-slate-950">待办优先处理</h3>
              <p className="mt-0.5 text-xs text-slate-400">只使用当前真实业务状态</p>
            </div>
            <span className="text-xs font-semibold text-slate-400">{profitStats.unknown_cost_orders + accountIssueCount} 项关注</span>
          </div>
          <div className="divide-y divide-slate-100">
            <button type="button" onClick={/* costBackfillAction 打开既有历史成本补全流程。 */ () => setBackfillOpen(true)} className="flex w-full items-center gap-3 py-4 text-left hover:bg-amber-50/60">
              <span className="flex h-8 w-8 items-center justify-center rounded-full bg-amber-50 text-amber-600"><AlertCircle className="h-4 w-4" /></span>
              <span className="min-w-0 flex-1"><span className="block text-sm font-bold text-slate-800">成本资料待补全</span><span className="mt-0.5 block truncate text-xs text-slate-400">未知成本订单不会进入利润金额</span></span>
              <strong className="tabular-nums text-amber-600">{profitStats.unknown_cost_orders}</strong><ChevronRight className="h-4 w-4 text-slate-300" />
            </button>
            <button type="button" onClick={refresh} className="flex w-full items-center gap-3 py-4 text-left hover:bg-slate-50">
              <span className="flex h-8 w-8 items-center justify-center rounded-full bg-blue-50 text-blue-600"><Activity className="h-4 w-4" /></span>
              <span className="min-w-0 flex-1"><span className="block text-sm font-bold text-slate-800">账号运行状态</span><span className="mt-0.5 block truncate text-xs text-slate-400">{runtimePresentation.label}，点击重新读取</span></span>
              <strong className={accountIssueCount > 0 ? 'tabular-nums text-red-600' : 'tabular-nums text-emerald-600'}>{accountIssueCount}</strong><ChevronRight className="h-4 w-4 text-slate-300" />
            </button>
            <button type="button" onClick={/* orderSectionAction 定位原有订单明细与搜索。 */ () => ordersSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })} className="flex w-full items-center gap-3 py-4 text-left hover:bg-slate-50">
              <span className="flex h-8 w-8 items-center justify-center rounded-full bg-slate-100 text-slate-600"><PackageCheck className="h-4 w-4" /></span>
              <span className="min-w-0 flex-1"><span className="block text-sm font-bold text-slate-800">参与统计订单</span><span className="mt-0.5 block truncate text-xs text-slate-400">查看完整订单明细与状态</span></span>
              <strong className="tabular-nums text-slate-700">{validOrdersTotal}</strong><ChevronRight className="h-4 w-4 text-slate-300" />
            </button>
          </div>
        </section>
      </div>

      <section className="overflow-hidden border-b border-slate-200 bg-white" aria-labelledby="profit-ranking-title">
        <div className="flex flex-wrap items-end justify-between gap-3 border-b border-slate-200 px-4 py-3">
          <div>
            <h3 id="profit-ranking-title" className="text-base font-black text-slate-950">商品毛利排行（预估）</h3>
            <p className="mt-0.5 text-xs text-slate-400">保留全部已匹配成本商品，表格区域可滚动</p>
          </div>
          <div className="grid grid-cols-2 gap-x-5 gap-y-1 text-right text-[11px] text-slate-500 sm:grid-cols-4">
            <span>已覆盖成交额 <strong className="ml-1 text-slate-800">{formatCurrency(profitStats.covered_revenue)}</strong></span>
            <span>商品成本 <strong className="ml-1 text-slate-800">{formatCurrency(profitStats.product_cost)}</strong></span>
            <span>平台手续费 <strong className="ml-1 text-slate-800">{formatCurrency(profitStats.platform_fee)}</strong></span>
            <span>利润率 <strong className="ml-1 text-emerald-600">{profitStats.gross_margin.toFixed(1)}%</strong></span>
          </div>
        </div>
        {itemProfitStats.length === 0 ? (
          <div className="flex h-28 items-center justify-center text-sm text-slate-400">暂无已匹配成本的商品订单</div>
        ) : (
          <div className="max-h-[310px] overflow-auto">
            <table className="w-full min-w-[820px] text-sm">
              <thead className="sticky top-0 z-10 bg-white">
                <tr className="border-b border-slate-200 text-left text-[11px] font-bold text-slate-400">
                  <th className="px-4 py-2">商品</th><th className="px-3 py-2 text-right">覆盖订单</th><th className="px-3 py-2 text-right">成交额</th><th className="px-3 py-2 text-right">商品成本</th><th className="px-3 py-2 text-right">手续费</th><th className="px-3 py-2 text-right">预估利润</th><th className="px-4 py-2 text-right">利润率</th>
                </tr>
              </thead>
              <tbody>
                {itemProfitStats.map(
                  // item 是当前渲染的真实商品利润排行项。
                  (item, index) => <tr key={item.item_id} className="border-b border-slate-100 last:border-0 hover:bg-slate-50">
                    <td className="px-4 py-2.5"><div className="flex min-w-0 items-center gap-3"><span className="w-5 text-center text-xs font-bold text-slate-400">{index + 1}</span>{itemImagesByID.get(item.item_id) && <img src={itemImagesByID.get(item.item_id)} alt="" className="h-8 w-8 shrink-0 rounded-md border border-slate-100 object-cover" />}<div className="min-w-0"><div className="max-w-[360px] truncate font-semibold text-slate-800" title={item.item_title || item.item_id}>{item.item_title || item.item_id}</div><div className="text-[11px] text-slate-400">成本覆盖 {item.coverage_rate.toFixed(1)}%</div></div></div></td>
                    <td className="px-3 py-2.5 text-right font-mono text-xs">{item.covered_orders} / {item.total_orders}</td>
                    <td className="px-3 py-2.5 text-right font-mono text-xs">{formatCurrency(item.covered_revenue)}</td>
                    <td className="px-3 py-2.5 text-right font-mono text-xs">{formatCurrency(item.product_cost)}</td>
                    <td className="px-3 py-2.5 text-right font-mono text-xs">{formatCurrency(item.platform_fee)}</td>
                    <td className={item.gross_profit >= 0 ? 'px-3 py-2.5 text-right font-mono text-xs font-bold text-emerald-600' : 'px-3 py-2.5 text-right font-mono text-xs font-bold text-red-600'}>{formatCurrency(item.gross_profit)}</td>
                    <td className="px-4 py-2.5 text-right font-mono text-xs">{item.gross_margin.toFixed(1)}%</td>
                  </tr>,
                )}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {profitStats.unknown_cost_orders > 0 && <div className="flex items-center justify-between gap-3 rounded-lg border border-amber-100 bg-amber-50 px-4 py-2.5 text-xs text-amber-800"><span>还有 {profitStats.unknown_cost_orders} 单未精确匹配成本；系统不会用售价倒推或猜测成本。</span><button type="button" className="shrink-0 font-bold underline" onClick={/* compactBackfillAction 打开既有补全流程。 */ () => setBackfillOpen(true)}>补全历史成本</button></div>}

      <button type="button" onClick={/* extendedAnalysisToggle 显示或收起所有原有次级分析模块。 */ () => setExtendedAnalysisOpen(/* extendedAnalysisUpdater 切换次级分析可见状态。 */ value => !value)} className="flex w-full items-center justify-between border-y border-slate-200 px-1 py-3 text-sm font-bold text-slate-700 hover:text-blue-600" aria-expanded={extendedAnalysisOpen}>
        <span>{extendedAnalysisOpen ? '收起完整经营分析' : '查看完整经营分析'}</span><ChevronRight className={'h-4 w-4 transition-transform ' + (extendedAnalysisOpen ? 'rotate-90' : '')} />
      </button>

	  {backfillOpen && <HistoricalCostBackfillModal accountID={selectedAccountID} accountLabel={selectedAccountLabel} startDate={currentRangeDates.startDate} endDate={currentRangeDates.endDate} onClose={/* 当前回调关闭历史成本补全弹窗。 */ () => setBackfillOpen(false)} onCompleted={refresh} />}

      {extendedAnalysisOpen && <section className="space-y-5" aria-label="完整经营分析">
      <DashboardTrendChart chartData={chartData} selectedRangeLabel={selectedRangeLabel} totalAmount={totalAmount} />

      {/* 商品销量排行和订单来源分布 */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
        {/* 商品销量排行 */}
        <div className="rounded-lg border border-slate-200 bg-white p-4">
          <h3 className="mb-4 text-base font-black text-slate-950">商品销量排行</h3>
          <div className="h-[220px]">
            {productSalesData.length === 0 ? (
              <div className="flex items-center justify-center h-full text-gray-400">暂无数据</div>
            ) : (
              <div className="h-full space-y-3 overflow-y-auto pr-2">
                {productSalesData.map(/* 当前回调处理集合中的单个元素。 */ (item, index) => (
                  <div key={`${item.name}-${index}`} className="space-y-2">
                    <div className="flex items-center justify-between gap-4">
                      <div className="flex items-center gap-3 min-w-0">
                        <span className={`w-7 h-7 rounded-xl flex items-center justify-center text-xs font-extrabold ${index < 3 ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-500'}`}>
                          {index + 1}
                        </span>
                        <span className="min-w-0 truncate text-sm font-bold text-gray-800" title={item.name}>{item.name}</span>
                      </div>
                      <span className="font-mono text-sm font-extrabold text-gray-900">{item.sales} 单</span>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-slate-100">
                      <div
                        className="h-full rounded-full bg-brand"
                        style={{ width: `${Math.max(8, (item.sales / maxProductSales) * 100)}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* 商品下单占比 */}
        <div className="rounded-lg border border-slate-200 bg-white p-4">
          <h3 className="mb-4 text-base font-black text-slate-950">商品下单占比</h3>
          <div
			className="dashboard-pie-chart relative h-[300px] overflow-hidden"
            role="img"
            aria-label={`商品下单占比，共 ${totalOrders} 单`}
          >
            {sourceDataData.length === 0 ? (
              <div className="flex items-center justify-center h-full text-gray-400">暂无数据</div>
            ) : (
              <>
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart accessibilityLayer={false}>
                    <Pie
                      data={sourceDataData}
                      cx="50%"
					  cy="36%"
                      innerRadius={48}
                      outerRadius={70}
                      paddingAngle={2}
                      dataKey="value"
                      activeShape={{
                        outerRadius: 76,
                        stroke: 'none',
                        strokeWidth: 0,
                      }}
                      rootTabIndex={-1}
                      label={false}
                      labelLine={false}
                    >
                      {sourceDataData.map(/* 当前回调处理集合中的单个元素。 */ (entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color} />
                      ))}
                    </Pie>
                    <Tooltip
                      formatter={/* 当前回调处理用户交互或异步状态变化。 */ (value) => `${Number(value || 0)} 单`}
                      wrapperStyle={{ zIndex: 30, outline: 'none' }}
                      contentStyle={{
                        backgroundColor: cssColor('white'),
                        border: `1px solid ${cssColor('neutral-200')}`,
                        borderRadius: '10px',
                        boxShadow: 'var(--shadow-md)'
                      }}
                    />
                    <Legend
                      verticalAlign="bottom"
					  height={112}
					  iconType="circle"
					  wrapperStyle={{ maxHeight: 104, overflowY: 'auto', padding: '0 8px 4px', lineHeight: '24px' }}
					  formatter={/* legendNameRenderer 保留完整商品名，长文案在图例区内换行而不越出卡片。 */ (value) => <span title={String(value)} style={{ color: cssColor('neutral-500'), display: 'inline-block', fontWeight: 500, lineHeight: '20px', maxWidth: '420px', overflowWrap: 'anywhere', verticalAlign: 'middle', whiteSpace: 'normal' }}>{value}</span>}
                    />
                  </PieChart>
                </ResponsiveContainer>
				<div className="pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center pb-20">
                  <span className="text-2xl font-extrabold text-gray-900 tabular-nums">{totalOrders}</span>
                  <span className="text-xs font-medium text-gray-400 mt-0.5">总订单</span>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* 收支明细和品类营收 */}
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
        {/* 参与统计的订单列表 */}
        <div ref={ordersSectionRef} className="flex scroll-mt-20 flex-col overflow-hidden rounded-lg border border-slate-200 bg-white lg:col-span-2">
          <div className="flex items-center justify-between border-b border-slate-200 bg-white p-4">
			<div>
			  <h3 className="font-bold text-lg text-gray-900">参与统计的订单</h3>
			  {validOrdersTruncated && (
				<p className="text-xs text-amber-700 mt-1">当前显示最近 {validOrders.length} / {validOrdersTotal} 条，搜索仅覆盖已加载明细。</p>
			  )}
			</div>
            <div className="relative">
              <input
                placeholder="搜索订单号/商品/买家..."
                value={searchTerm}
                onChange={/* 当前回调处理用户交互或异步状态变化。 */ (e) => setSearchTerm(e.target.value)}
                className="h-8 w-52 rounded-md border border-slate-200 bg-white px-3 text-xs outline-none focus:border-blue-400"
                type="text"
              />
            </div>
          </div>
          <div className="max-h-[340px] flex-1 overflow-x-auto">
            {ordersLoading ? (
              <div className="flex items-center justify-center py-20 text-gray-400">
                <Activity className="w-6 h-6 animate-spin mr-2" />
                加载中...
              </div>
            ) : filteredValidOrders.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-16 px-8 text-center">
                <div className="w-14 h-14 rounded-2xl bg-gray-100 flex items-center justify-center mb-4">
                  <PackageCheck className="w-7 h-7 text-gray-300" />
                </div>
                {normalizedSearchTerm ? (
                  <>
                    <div className="text-sm font-extrabold text-gray-900">没有匹配的订单</div>
                    <div className="text-xs text-gray-400 mt-2 max-w-md">
                      当前共有 {validOrders.length} 单参与统计，但没有订单号、商品、买家匹配“{searchTerm}”。
                    </div>
                  </>
                ) : (
                  <>
                    <div className="text-sm font-extrabold text-gray-900">当前范围内没有参与统计的订单</div>
                    <div className="text-xs text-gray-400 mt-2 max-w-lg leading-6">
                      日期范围：{currentRangeDates.startDate} 至 {currentRangeDates.endDate}；
                      统计口径：待发货、已发货、已收货、已完成，且订单金额合法。
                      当前统计卡片订单数：{analytics.revenue_stats.total_orders} 单。
                    </div>
                  </>
                )}
              </div>
            ) : (
              <table className="w-full min-w-[760px] text-left border-collapse">
                <thead>
                  <tr className="bg-white text-gray-400 text-xs font-bold uppercase tracking-wider border-b border-gray-50">
                    <th className="px-4 py-2.5">订单信息</th>
                    <th className="px-4 py-2.5">买家信息</th>
                    <th className="px-4 py-2.5">金额</th>
                    <th className="whitespace-nowrap px-4 py-2.5">状态</th>
                    <th className="px-4 py-2.5 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {filteredValidOrders.map(/* 当前回调处理集合中的单个元素。 */ (order) => (
                      <tr key={order.order_id} className="hover:bg-warning-50/50 transition-colors group">
                        <td className="px-4 py-2.5">
                          <div className="flex items-center gap-3">
                            <div className="h-9 w-9 flex-shrink-0 overflow-hidden rounded-md border border-slate-100 bg-slate-50">
                              <PackageCheck className="w-full h-full text-gray-300 p-2" />
                            </div>
                            <div className="min-w-0">
                              <div className="font-bold text-gray-900 text-sm line-clamp-1">
                                {order.item_title || order.item_id || '未知商品'}
                              </div>
                              <div className="text-xs text-gray-500 mt-1 font-mono">{order.order_id}</div>
                              <div className="text-xs text-gray-400 mt-0.5">数量: {order.quantity || 1}</div>
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-2.5">
                          <div className="text-sm font-bold text-gray-800">{order.buyer_id}</div>
                          {order.created_at && (
                            <div className="text-xs text-gray-400 mt-1">{formatLocalDateTime(order.created_at)}</div>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-sm font-extrabold text-gray-900 font-feature-settings-tnum">
                          ¥{order.amount || '0.00'}
                        </td>
                        <td className="whitespace-nowrap px-4 py-2.5">
                          <StatusBadge status={order.status || order.order_status || 'unknown'} />
                        </td>
                        <td className="px-4 py-2.5 text-right">
                          <a
                            href={`https://www.goofish.com/order-detail?orderId=${order.order_id}&role=seller`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex text-gray-400 hover:text-blue-600 p-2 rounded-xl hover:bg-blue-50 transition-colors"
                            title="查看闲鱼详情"
                          >
                            <ExternalLink className="w-4 h-4" />
                          </a>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* 商品金额分析 */}
        <div className="rounded-lg border border-slate-200 bg-white p-4">
          <h3 className="mb-4 text-base font-black text-slate-950">商品金额分析 (TOP5)</h3>
          {categoryDataData.length === 0 ? (
            <div className="flex items-center justify-center h-[300px] text-gray-400">暂无数据</div>
          ) : (
            <>
              <div
                className="dashboard-pie-chart relative h-[220px]"
                role="img"
                aria-label={`商品金额分析，总金额 ${formatCurrency(totalAmount)}`}
              >
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart accessibilityLayer={false}>
                    <Pie
                      data={categoryDataData}
                      cx="50%"
                      cy="50%"
                      innerRadius={48}
                      outerRadius={70}
                      paddingAngle={2}
                      dataKey="value"
                      activeShape={{
                        outerRadius: 76,
                        stroke: 'none',
                        strokeWidth: 0,
                      }}
                      rootTabIndex={-1}
                      label={false}
                      labelLine={false}
                    >
                      {categoryDataData.map(/* 当前回调处理集合中的单个元素。 */ (entry, index) => (
                        <Cell key={`cell-${index}`} fill={entry.color || COLORS[index % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip
                      wrapperStyle={{ zIndex: 30, outline: 'none' }}
                      contentStyle={{
                        backgroundColor: cssColor('white'),
                        border: `1px solid ${cssColor('neutral-200')}`,
                        borderRadius: '6px',
                        boxShadow: 'var(--shadow-md)'
                      }}
                      formatter={/* 当前回调处理用户交互或异步状态变化。 */ (value) => `¥${Number(value || 0).toLocaleString()}`}
                    />
                  </PieChart>
                </ResponsiveContainer>
                <div className="pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center">
                  <span className="text-lg font-extrabold text-gray-900 tabular-nums">{formatCurrency(totalAmount)}</span>
                  <span className="text-xs font-medium text-gray-400 mt-0.5">总金额</span>
                </div>
              </div>
              <div className="space-y-3 mt-4">
                {categoryDataData.map(/* 当前回调处理集合中的单个元素。 */ (cat) => (
                  <div key={cat.name} className="flex justify-between items-center gap-3 text-sm">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <div
                        className="w-3 h-3 rounded-full shrink-0"
                        style={{ backgroundColor: cat.color || COLORS[categoryDataData.indexOf(cat) % COLORS.length] }}
                      ></div>
                      <span className="text-gray-600 font-medium truncate" title={cat.name}>{cat.name}</span>
                    </div>
                    <div className="flex items-center gap-3 shrink-0 whitespace-nowrap">
                      <span className="font-bold text-gray-900">¥{cat.value.toLocaleString()}</span>
                      <span className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded">{cat.percentage}%</span>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
      </section>}
    </div>
  );
};

export default Dashboard;
