import { CircleDollarSign } from 'lucide-react';
import React from 'react';
import { Bar,CartesianGrid,ComposedChart,Legend,Line,ResponsiveContainer,Tooltip,XAxis,YAxis } from 'recharts';
import type { DashboardDailyProfitPoint } from './state';

/** 商品毛利趋势图组件的输入参数。 */
export type DashboardProfitChartProps = {
  /** 当前范围按日聚合的商品毛利数据。 */
  dailyProfitStats: DashboardDailyProfitPoint[];
  /** 当前选择范围的中文名称。 */
  selectedRangeLabel: string;
  /** 当前统计范围的起始本地日期。 */
  startDate: string;
  /** 当前统计范围的结束本地日期。 */
  endDate: string;
  /** compact 表示首屏使用方案三的紧凑图表尺寸，数据序列和交互保持完整。 */
  compact?: boolean;
};

/** 读取设计系统颜色变量。 */
const cssColor = (token: string, alpha?: number): string => (
  alpha === undefined ? `rgb(var(--color-${token}))` : `rgb(var(--color-${token}) / ${alpha})`
);

/** 将服务端按日毛利统计转换为图表数据点。 */
export const buildCompleteProfitChartData = (dailyProfitStats: DashboardDailyProfitPoint[], startDate: string, endDate: string) => {
  // statsByDate 是服务端有订单日期到利润统计的索引。
  const statsByDate = new Map(dailyProfitStats.map(/* item 是当前建立日期索引的单日统计。 */ item => [item.date, item]));
  // start 是使用 UTC 日历运算的范围起点，避免本地夏令时造成漏日。
  const start = new Date(`${startDate}T00:00:00Z`);
  // end 是使用 UTC 日历运算的范围终点。
  const end = new Date(`${endDate}T00:00:00Z`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || start > end) return [];
  // result 保存包含零订单自然日的完整趋势数据。
  const result = [];
  // cursor 是当前补齐的 UTC 自然日。
  const cursor = new Date(start);
  // days 是已补齐天数，最多输出 366 天避免异常范围造成超大 DOM。
  let days = 0;
  while (cursor <= end && days < 366) {
    // date 是当前完整日期键。
    const date = cursor.toISOString().slice(0, 10);
    // item 是当前日期服务端统计，零订单日期使用全零回退。
    const item = statsByDate.get(date);
    result.push({ name: date.slice(5), coveredRevenue: item?.covered_revenue || 0, productCost: item?.product_cost || 0, platformFee: item?.platform_fee || 0, grossProfit: item?.gross_profit || 0 });
    cursor.setUTCDate(cursor.getUTCDate() + 1);
    days += 1;
  }
  return result;
};

/** 展示仅基于已匹配成本订单的成交额、成本和预估商品毛利趋势。 */
export const DashboardProfitChart: React.FC<DashboardProfitChartProps> = ({ dailyProfitStats, selectedRangeLabel, startDate, endDate, compact = false }) => {
  // scrollRef 指向横向图表视口，日期范围变化后必须回到最左侧，避免沿用一个月视图的旧滚动位置。
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  // chartData 缓存当前日期范围的平面数据，避免父级无关更新让 Recharts 重启单点动画。
  const chartData = React.useMemo(
    /* profitChartData 只在服务端日统计或日期范围变化时重新构造。 */ () => buildCompleteProfitChartData(dailyProfitStats, startDate, endDate),
    [dailyProfitStats, endDate, startDate],
  );
  // hasCoveredData 表示当前范围至少有一天存在已匹配成本成交额。
  const hasCoveredData = chartData.some(
    // item 是当前检查是否存在成本覆盖的日期数据点。
    item => item.coveredRevenue !== 0 || item.productCost !== 0,
  );
  React.useEffect(/* chartRangeScrollReset 在已应用日期边界变化后清理旧横向滚动状态。 */ () => {
    if (scrollRef.current) scrollRef.current.scrollLeft = 0;
  }, [endDate, startDate]);
  return (
    <div className={compact ? 'rounded-lg border border-slate-200 bg-white p-4' : 'ios-card rounded-xl p-8'}>
      <div className={compact ? 'mb-3 flex flex-wrap items-end justify-between gap-2' : 'mb-8'}>
        <div>
          <h3 className={compact ? 'text-base font-black text-slate-950' : 'text-xl font-bold text-gray-900'}>经营利润趋势（预估）</h3>
          <p className={compact ? 'mt-0.5 text-xs text-slate-400' : 'mt-1 text-sm text-gray-400'}>{selectedRangeLabel}仅统计已精确匹配成本的订单，已扣除 1.6% 平台手续费，不含运费、广告及税费。</p>
        </div>
      </div>
      <div ref={scrollRef} data-range-start={startDate} data-range-end={endDate} data-point-count={chartData.length} className={compact ? 'dashboard-profit-chart h-[314px] w-full overflow-x-auto overflow-y-hidden' : 'dashboard-profit-chart h-[356px] w-full overflow-x-auto overflow-y-hidden'}>
        <div className={compact ? 'h-[298px] w-full' : 'h-[340px] w-full'} style={{ minWidth: `${Math.max(760, chartData.length * 58)}px` }}>
          {!hasCoveredData ? (
            <div className="h-full flex flex-col items-center justify-center text-gray-400">
              <CircleDollarSign className="w-16 h-16 mb-4 opacity-20" />
              <p className="text-lg font-medium">暂无可计算毛利的订单</p>
              <p className="text-sm mt-2">请先为商品规格配置成本，后续精确匹配的订单会自动计入。</p>
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%" debounce={120}>
              <ComposedChart data={chartData} margin={compact ? { top: 8, right: 12, left: 0, bottom: 12 } : { top: 12, right: 20, left: 12, bottom: 20 }}>
                <CartesianGrid vertical={false} stroke={cssColor('neutral-100')} strokeDasharray="3 3" />
                <XAxis dataKey="name" axisLine={false} tickLine={false} height={44} tickMargin={10} interval={0} minTickGap={0} tick={{ fill: cssColor('neutral-400'), fontSize: 12, fontWeight: 500 }} />
                <YAxis width={compact ? 58 : 76} tickMargin={8} axisLine={false} tickLine={false} tick={{ fill: cssColor('neutral-400'), fontSize: compact ? 11 : 13, fontWeight: 500 }} tickFormatter={/* value 是纵轴刻度对应的人民币金额。 */ value => `¥${Number(value)}`} />
                <Tooltip formatter={/* value 和 name 是提示框当前金额及数据序列标识。 */ (value, name) => [`¥${Number(value).toFixed(2)}`, name === 'coveredRevenue' ? '已覆盖成交额' : name === 'productCost' ? '商品成本' : name === 'platformFee' ? '平台手续费' : '预估经营利润']} contentStyle={{ backgroundColor: cssColor('white'), borderRadius: '8px', border: `1px solid ${cssColor('neutral-200')}`, boxShadow: 'var(--shadow-xl)' }} />
                <Legend formatter={/* value 是图例对应的数据序列标识。 */ value => value === 'coveredRevenue' ? '已覆盖成交额' : value === 'productCost' ? '商品成本' : value === 'platformFee' ? '平台手续费' : '预估经营利润'} />
                <Bar dataKey="coveredRevenue" fill={cssColor('brand', 0.45)} maxBarSize={52} radius={[8, 8, 0, 0]} isAnimationActive={false} />
                <Line type="monotone" dataKey="productCost" stroke={cssColor('warning-500')} strokeWidth={3} dot={{ r: 3 }} isAnimationActive={false} />
                <Line type="monotone" dataKey="platformFee" stroke={cssColor('accent-500')} strokeWidth={2} dot={{ r: 2 }} isAnimationActive={false} />
                <Line type="monotone" dataKey="grossProfit" stroke={cssColor('success-500')} strokeWidth={3} dot={{ r: 3 }} isAnimationActive={false} />
              </ComposedChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>
    </div>
  );
};
