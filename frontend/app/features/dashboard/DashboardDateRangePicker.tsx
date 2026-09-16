import { CalendarDays,ChevronLeft,ChevronRight,X } from 'lucide-react';
import React from 'react';

// DashboardDateRangePickerProps 描述紧凑日期范围弹层的受控值和用户动作。
export interface DashboardDateRangePickerProps {
  /** startDate 是已选择的开始自然日，使用 YYYY-MM-DD。 */
  startDate: string;
  /** endDate 是已选择的结束自然日，使用 YYYY-MM-DD。 */
  endDate: string;
  /** onStartDateChange 更新尚未应用的开始日期。 */
  onStartDateChange: (value: string) => void;
  /** onEndDateChange 更新尚未应用的结束日期。 */
  onEndDateChange: (value: string) => void;
  /** onApply 在范围完整时提交统计范围。 */
  onApply: () => void;
  /** onCancel 关闭弹层并保留当前草稿。 */
  onCancel: () => void;
}

// CalendarDay 是月份网格中的一个本地自然日。
interface CalendarDay {
  /** key 是稳定的 YYYY-MM-DD 日期键。 */
  key: string;
  /** label 是网格显示的日号。 */
  label: number;
  /** inMonth 表示日期是否属于当前标题月份。 */
  inMonth: boolean;
}

// padDatePart 把月份和日号补齐为两位文本。
const padDatePart = (value: number): string => String(value).padStart(2, '0');

// localDateKey 把本地日期转为不会受 UTC 偏移影响的 YYYY-MM-DD。
const localDateKey = (date: Date): string => `${date.getFullYear()}-${padDatePart(date.getMonth() + 1)}-${padDatePart(date.getDate())}`;

// parseLocalDate 把受控日期键转换为本地午夜，空值或非法值返回当前日期。
const parseLocalDate = (value: string): Date => {
  // parts 是日期键拆分后的年、月和日数值。
  const parts = value.split('-').map(/* datePartParser 把日期片段转换为十进制数值。 */ part => Number(part));
  if (parts.length !== 3 || parts.some(/* invalidDatePart 检查日期片段是否无法解析。 */ part => !Number.isFinite(part))) return new Date();
  return new Date(parts[0], parts[1] - 1, parts[2]);
};

// calendarDays 构造包含前后月份补位的六周日期网格。
const calendarDays = (month: Date): CalendarDay[] => {
  // firstDay 是当前月份第一天。
  const firstDay = new Date(month.getFullYear(), month.getMonth(), 1);
  // mondayOffset 把周日为零的索引转换为周一开头。
  const mondayOffset = (firstDay.getDay() + 6) % 7;
  // gridStart 是六周网格第一格对应的自然日。
  const gridStart = new Date(firstDay);
  gridStart.setDate(firstDay.getDate() - mondayOffset);
  return Array.from({ length: 42 }, /* calendarDayFactory 创建当前网格位置对应的日期。 */ (_, index) => {
    // date 是从网格起点顺延 index 天得到的本地日期。
    const date = new Date(gridStart);
    date.setDate(gridStart.getDate() + index);
    return { key: localDateKey(date), label: date.getDate(), inMonth: date.getMonth() === month.getMonth() };
  });
};

// dateLabel 把日期键转换为紧凑中文月日，空值显示选择提示。
const dateLabel = (value: string, fallback: string): string => value ? `${Number(value.slice(5, 7))}月${Number(value.slice(8, 10))}日` : fallback;

// DashboardDateRangePicker 用应用内月份网格替代不可控的浏览器原生日期面板。
export const DashboardDateRangePicker: React.FC<DashboardDateRangePickerProps> = ({ startDate, endDate, onStartDateChange, onEndDateChange, onApply, onCancel }) => {
  // [visibleMonth, setVisibleMonth] 保存当前展示月份，优先从开始日期恢复。
  const [visibleMonth, setVisibleMonth] = React.useState(/* initialVisibleMonth 选择开始日期或今天所在月份。 */ () => parseLocalDate(startDate));
  // days 是当前月份对应的固定六周网格。
  const days = React.useMemo(/* visibleCalendarDays 只在切换月份时重新生成。 */ () => calendarDays(visibleMonth), [visibleMonth]);
  // todayKey 是当前本地自然日，用于轻量描边提示。
  const todayKey = localDateKey(new Date());
  // canApply 表示范围起止都存在且顺序有效。
  const canApply = Boolean(startDate && endDate && startDate <= endDate);

  // changeMonth 按月切换网格，保留尚未应用的范围草稿。
  const changeMonth = (offset: number): void => setVisibleMonth(
    // currentMonth 是切换前的展示月份。
    currentMonth => new Date(currentMonth.getFullYear(), currentMonth.getMonth() + offset, 1),
  );

  // selectDay 按“先开始、再结束”的顺序更新范围；第三次点击重新开始选择。
  const selectDay = (key: string): void => {
    if (!startDate || endDate) {
      onStartDateChange(key);
      onEndDateChange('');
      return;
    }
    if (key < startDate) {
      onStartDateChange(key);
      onEndDateChange('');
      return;
    }
    onEndDateChange(key);
  };

  // clearRange 清空当前范围并保持弹层打开。
  const clearRange = (): void => {
    onStartDateChange('');
    onEndDateChange('');
  };

  return <div className="absolute right-0 top-full z-50 mt-2 w-[340px] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-xl shadow-slate-200/70" role="dialog" aria-label="自定义统计范围">
    <div className="flex items-center justify-between border-b border-slate-100 px-4 py-3">
      <div>
        <h3 className="text-sm font-black text-slate-900">自定义统计范围</h3>
        <p className="mt-0.5 text-[11px] text-slate-400">依次选择开始和结束日期</p>
      </div>
      <button type="button" onClick={onCancel} className="flex h-7 w-7 items-center justify-center rounded-md text-slate-400 hover:bg-slate-100 hover:text-slate-700" aria-label="关闭日期选择"><X className="h-4 w-4" /></button>
    </div>

    <div className="grid grid-cols-2 gap-2 bg-slate-50/70 px-4 py-3">
      <div className={startDate ? 'rounded-lg border border-blue-200 bg-white px-3 py-2' : 'rounded-lg border border-dashed border-slate-200 bg-white px-3 py-2'}><span className="block text-[10px] font-semibold text-slate-400">开始日期</span><strong className={startDate ? 'mt-0.5 block text-sm text-blue-700' : 'mt-0.5 block text-sm text-slate-400'}>{dateLabel(startDate, '请选择')}</strong></div>
      <div className={endDate ? 'rounded-lg border border-blue-200 bg-white px-3 py-2' : 'rounded-lg border border-dashed border-slate-200 bg-white px-3 py-2'}><span className="block text-[10px] font-semibold text-slate-400">结束日期</span><strong className={endDate ? 'mt-0.5 block text-sm text-blue-700' : 'mt-0.5 block text-sm text-slate-400'}>{dateLabel(endDate, '请选择')}</strong></div>
    </div>

    <div className="px-4 pb-3 pt-2">
      <div className="mb-2 flex items-center justify-between">
        <button type="button" onClick={/* previousMonthAction 展示上一个月。 */ () => changeMonth(-1)} className="flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100" aria-label="上一个月"><ChevronLeft className="h-4 w-4" /></button>
        <div className="flex items-center gap-2 text-sm font-black text-slate-800"><CalendarDays className="h-4 w-4 text-blue-600" />{visibleMonth.getFullYear()}年{visibleMonth.getMonth() + 1}月</div>
        <button type="button" onClick={/* nextMonthAction 展示下一个月。 */ () => changeMonth(1)} className="flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100" aria-label="下一个月"><ChevronRight className="h-4 w-4" /></button>
      </div>
      <div className="grid grid-cols-7 text-center text-[10px] font-semibold text-slate-400">{['一','二','三','四','五','六','日'].map(/* weekdayRenderer 渲染周一开头的星期标题。 */ weekday => <span key={weekday} className="py-1">{weekday}</span>)}</div>
      <div className="grid grid-cols-7 gap-y-1">
        {days.map(/* calendarDayRenderer 渲染日期范围和端点状态。 */ day => {
          // selected 表示日期是范围起点或终点。
          const selected = day.key === startDate || day.key === endDate;
          // inRange 表示日期位于已完成范围内部。
          const inRange = Boolean(startDate && endDate && day.key > startDate && day.key < endDate);
          // today 表示当前日期，用描边而非大色块提示。
          const today = day.key === todayKey;
          // dayClass 是当前日期按钮的紧凑视觉状态。
          const dayClass = selected ? 'bg-blue-600 text-white' : inRange ? 'bg-blue-50 text-blue-700' : day.inMonth ? 'text-slate-700 hover:bg-slate-100' : 'text-slate-300 hover:bg-slate-50';
          return <button key={day.key} type="button" onClick={/* calendarDayAction 选择当前自然日。 */ () => selectDay(day.key)} className={`mx-auto flex h-8 w-8 items-center justify-center rounded-md text-xs font-semibold transition-colors ${dayClass} ${today && !selected ? 'ring-1 ring-inset ring-blue-300' : ''}`} aria-label={day.key}>{day.label}</button>;
        })}
      </div>
    </div>

    <div className="flex items-center justify-between border-t border-slate-100 px-4 py-3">
      <button type="button" onClick={clearRange} className="text-xs font-semibold text-slate-500 hover:text-slate-800">清除</button>
      <div className="flex gap-2"><button type="button" onClick={onCancel} className="h-8 rounded-md px-3 text-xs font-semibold text-slate-600 hover:bg-slate-100">取消</button><button type="button" onClick={onApply} disabled={!canApply} className="h-8 rounded-md bg-blue-600 px-4 text-xs font-bold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-400">应用范围</button></div>
    </div>
  </div>;
};
