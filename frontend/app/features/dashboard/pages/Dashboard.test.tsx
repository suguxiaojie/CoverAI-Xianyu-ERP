import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe,expect,test } from 'vitest';
import { dashboardAccountLabel,runtimeStatusPresentation,StatusBadge } from './Dashboard';
import { buildCompleteProfitChartData } from '../DashboardProfitChart';

// dashboardSource 仪表盘测试数据源。
const dashboardSource = readFileSync(resolve(__dirname, 'Dashboard.tsx'), 'utf8');
// trendChartSource 趋势图测试数据源。
const trendChartSource = readFileSync(resolve(__dirname, '../DashboardTrendChart.tsx'), 'utf8');
// profitChartSource 商品毛利趋势图测试数据源。
const profitChartSource = readFileSync(resolve(__dirname, '../DashboardProfitChart.tsx'), 'utf8');
// backfillModalSource 历史成本补全弹窗测试数据源。
const backfillModalSource = readFileSync(resolve(__dirname, '../HistoricalCostBackfillModal.tsx'), 'utf8');
// globalStyles 测试用全局样式。
const globalStyles = readFileSync(resolve(__dirname, '../../../../index.css'), 'utf8');

describe('Dashboard presentation safeguards', /* 当前回调处理用户交互或异步状态变化。 */ () => {
  test('keeps order status badges on one line', /* 当前回调处理用户交互或异步状态变化。 */ () => {
    // html 渲染后的 HTML。
    const html = renderToStaticMarkup(<StatusBadge status="shipped" />);

    expect(html).toContain('已发货');
    expect(html).toContain('inline-flex');
    expect(html).toContain('whitespace-nowrap');
    expect(dashboardSource).toContain('min-w-[760px]');
  });

  test('pie charts use enlarged active sectors without focus rings or external label lines', /* 当前回调处理用户交互或异步状态变化。 */ () => {
    expect(dashboardSource.match(/accessibilityLayer=\{false\}/g)).toHaveLength(2);
    expect(dashboardSource.match(/activeShape=\{\{/g)).toHaveLength(2);
    expect(dashboardSource.match(/outerRadius: 76/g)).toHaveLength(2);
    expect(dashboardSource.match(/stroke: 'none'/g)).toHaveLength(2);
    expect(dashboardSource.match(/strokeWidth: 0/g)).toHaveLength(2);
    expect(dashboardSource.match(/rootTabIndex=\{-1\}/g)).toHaveLength(2);
    expect(dashboardSource.match(/label=\{false\}/g)).toHaveLength(2);
    expect(dashboardSource.match(/labelLine=\{false\}/g)).toHaveLength(2);
    expect(dashboardSource.match(/wrapperStyle=\{\{ zIndex: 30, outline: 'none' \}\}/g)).toHaveLength(2);
    expect(dashboardSource.match(/absolute inset-0 z-10/g)).toHaveLength(2);
    expect(globalStyles).toContain('.dashboard-pie-chart .recharts-sector:focus');
    expect(trendChartSource).toContain('dashboard-revenue-chart h-[296px] w-full overflow-x-auto overflow-y-hidden');
    expect(trendChartSource).toContain('h-[280px] w-full');
    expect(trendChartSource).toContain('left: 12, bottom: 24');
    expect(trendChartSource.match(/width=\{72\}/g)).toHaveLength(2);
    expect(trendChartSource).toContain('height={42}');
    expect(trendChartSource.match(/interval=\{0\}/g)).toHaveLength(2);
    expect(trendChartSource).toContain('Math.max(760, chartData.length * 58)');
    expect(trendChartSource).toContain('overflow-x-auto');
    expect(trendChartSource).toContain('activeBar={false}');
    expect(trendChartSource).not.toMatch(/#[0-9A-Fa-f]{3,8}/);
    expect(trendChartSource).toContain("fill={cssColor('brand')}");
    expect(globalStyles).toContain('.dashboard-revenue-chart .recharts-rectangle:focus');
    expect(globalStyles).toContain('.dashboard-revenue-chart .recharts-curve:focus');
    expect(globalStyles).toContain('.dashboard-revenue-chart [class*="recharts-zIndex-layer_"]:focus');
		expect(globalStyles).toContain('stroke: none !important');
		expect(dashboardSource).toContain('dashboard-pie-chart relative h-[300px] overflow-hidden');
		expect(dashboardSource).toContain('height={112}');
		expect(dashboardSource).toContain("maxHeight: 104, overflowY: 'auto'");
		expect(dashboardSource).toContain("overflowWrap: 'anywhere'");
		expect(dashboardSource).toContain("whiteSpace: 'normal'");
  });

	test('uses effective transaction wording and real runtime summary', /* 当前回调验证 Dashboard 不再把范围成交额或静态绿灯表述为累计健康状态。 */ () => {
    expect(dashboardSource).toContain('title="有效成交额"');
    expect(dashboardSource).toContain("detail={trendPercent ? '较上周期 ' + trendPercent : '按下单时间'}");
    expect(dashboardSource).toContain('title="在线账号"');
    expect(dashboardSource).toContain('runtimeSummary');
    expect(dashboardSource).toContain('已收货');
    expect(dashboardSource).not.toContain('累计营收 (CNY)');
	});

	test('默认全部账号并明确保留共享库存口径', /* 当前回调验证账号选择与无法按账号归属的库存不会被混淆。 */ () => {
		expect(dashboardSource).toContain('<option value="">全部账号</option>');
		expect(dashboardSource).toContain("selectedAccountID ? '共享库存卡密' : '库存卡密'");
		expect(dashboardSource).toContain("selectedAccountID ? '全部账号共用' : '当前可用余量'");
		expect(dashboardAccountLabel({ id: 'account-1', enabled: true, nickname: '店铺一', remark: '主店' })).toBe('店铺一（主店）');
		expect(dashboardAccountLabel({ id: 'account-2', enabled: false })).toBe('账号 account-2');
	});

  test('labels profit as estimated and exposes cost coverage without guessing unknown orders', /* 当前回调验证毛利展示不会把覆盖订单口径伪装成全部订单净利润。 */ () => {
    expect(dashboardSource).toContain('经营利润统计与经营关键指标');
    expect(dashboardSource).toContain('预估经营利润');
    expect(dashboardSource).toContain('成本覆盖');
    expect(dashboardSource).toContain('系统不会用售价倒推或猜测成本');
    expect(dashboardSource).toContain('商品毛利排行（预估）');
    expect(dashboardSource).toContain('itemImagesByID');
    expect(dashboardSource).toContain('itemImagesByID.get(item.item_id)');
    expect(profitChartSource).toContain('仅统计已精确匹配成本的订单');
    expect(profitChartSource).toContain('已扣除 1.6% 平台手续费');
    expect(profitChartSource).toContain('coveredRevenue');
    expect(profitChartSource).toContain('productCost');
    expect(profitChartSource).toContain('grossProfit');
    expect(profitChartSource).toContain('platformFee');
    expect(dashboardSource).toContain('补全历史成本');
    expect(backfillModalSource).toContain('只处理规格为空');
    expect(backfillModalSource).toContain('系统不会按成交金额猜测');
    expect(backfillModalSource).toContain('当前 SKU 成本生成不可变快照');
    expect(backfillModalSource).toContain('历史单件成本');
		expect(backfillModalSource).toContain('人工历史成本');
		expect(backfillModalSource).toContain('无可用 SKU，仅保存订单级历史成本');
		expect(backfillModalSource).toContain('candidate.manual_only');
		expect(backfillModalSource).toContain('candidate.quantity * candidate.order_count');
		expect(backfillModalSource).toContain('candidate.account_id');
    expect(backfillModalSource).toContain('沿用历史确认');
    expect(backfillModalSource).toContain('平台手续费 ¥');
    expect(backfillModalSource).toContain('bg-transparent');
    expect(backfillModalSource).not.toContain('bg-black/40');
    expect(backfillModalSource).toContain('createPortal(modal, document.body)');
    expect(profitChartSource).toContain('interval={0}');
  });

  test('profit chart fills zero-order calendar dates without dropping x-axis labels', /* 当前回调验证完整自然日范围不会因无订单而缺失横轴日期。 */ () => {
    expect(buildCompleteProfitChartData([{ date: '2026-08-03', covered_revenue: 10, product_cost: 5, platform_fee: 0.16, gross_profit: 4.84 }], '2026-08-01', '2026-08-04')).toEqual([
      { name: '08-01', coveredRevenue: 0, productCost: 0, platformFee: 0, grossProfit: 0 },
      { name: '08-02', coveredRevenue: 0, productCost: 0, platformFee: 0, grossProfit: 0 },
      { name: '08-03', coveredRevenue: 10, productCost: 5, platformFee: 0.16, grossProfit: 4.84 },
      { name: '08-04', coveredRevenue: 0, productCost: 0, platformFee: 0, grossProfit: 0 },
    ]);
    expect(profitChartSource).toContain('React.useMemo');
    expect(profitChartSource).toContain('debounce={120}');
    expect(profitChartSource.match(/isAnimationActive=\{false\}/g)).toHaveLength(4);
    expect(profitChartSource).toContain('h-[356px] w-full overflow-x-auto overflow-y-hidden');
    expect(profitChartSource).toContain('h-[340px] w-full');
    expect(profitChartSource).toContain('scrollRef.current.scrollLeft = 0');
    expect(profitChartSource).toContain('data-point-count={chartData.length}');
    expect(dashboardSource).toContain('const currentRangeDates = data!.dateRange');
  });

  test('selected compact redesign keeps every prior dashboard capability reachable', /* compactFeatureParity 验证视觉压缩不删除日期、成本、分析或订单入口。 */ () => {
    expect(dashboardSource).toContain("{ key: 'yesterday' as TimeRange, label: '昨天' }");
    expect(dashboardSource).toContain("{ key: '3days' as TimeRange, label: '三天内' }");
    expect(dashboardSource).toContain("{ key: 'custom' as TimeRange, label: '自定义' }");
    expect(dashboardSource).toContain('timeRangeOptions.map');
    expect(dashboardSource).not.toContain('更多时间范围');
    expect(dashboardSource).toContain('DashboardDateRangePicker');
    expect(dashboardSource).not.toContain('type="date"');
    expect(dashboardSource).toContain('补全历史成本');
    expect(dashboardSource).toContain('查看完整经营分析');
    expect(dashboardSource).toContain('商品销量排行');
    expect(dashboardSource).toContain('商品下单占比');
		expect(dashboardSource).toContain('height={112}');
    expect(dashboardSource).not.toContain("maxWidth: '180px'");
    expect(dashboardSource).toContain('参与统计的订单');
    expect(dashboardSource).toContain('商品金额分析 (TOP5)');
    expect(dashboardSource).toContain('搜索订单号/商品/买家...');
    expect(dashboardSource).not.toContain('处理待办');
    expect(dashboardSource).toContain('xl:grid-cols-6');
    expect(dashboardSource).not.toContain('grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6');
  });

  test('runtime status presentation exposes actionable account states', /* 当前回调验证真实运行状态可以区分在线、认证、人工验证、冲突和离线。 */ () => {
    expect(runtimeStatusPresentation({ state: 'healthy', onlineAccounts: 2, enabledAccounts: 2 }).label).toBe('系统正常运行');
    expect(runtimeStatusPresentation({ state: 'partial', onlineAccounts: 1, enabledAccounts: 2 })).toMatchObject({ label: '部分账号异常', detail: '1 / 2 在线' });
    expect(runtimeStatusPresentation({ state: 'auth_expired', onlineAccounts: 1, enabledAccounts: 2 }).label).toBe('登录已过期');
    expect(runtimeStatusPresentation({ state: 'verification_required', onlineAccounts: 1, enabledAccounts: 2 }).label).toBe('需要人工验证');
    expect(runtimeStatusPresentation({ state: 'runtime_conflict', onlineAccounts: 0, enabledAccounts: 2 }).label).toBe('账号运行冲突');
    expect(runtimeStatusPresentation({ state: 'offline', onlineAccounts: 0, enabledAccounts: 2 }).label).toBe('账号服务异常');
    expect(runtimeStatusPresentation({ state: 'no_enabled_accounts', onlineAccounts: 0, enabledAccounts: 0 }).label).toBe('暂无启用账号');
    expect(runtimeStatusPresentation({ state: 'loading', onlineAccounts: 0, enabledAccounts: 0 }).label).toBe('正在读取运行状态');
    expect(runtimeStatusPresentation({ state: 'unknown', onlineAccounts: 0, enabledAccounts: 0 }).label).toBe('运行状态未知');
  });
});
