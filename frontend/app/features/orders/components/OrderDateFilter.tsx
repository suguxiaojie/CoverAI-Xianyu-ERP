import { ArrowRight,CalendarDays,ChevronDown,ChevronLeft,ChevronRight,X } from 'lucide-react';
import React from 'react';
import type { OrderCreatedRange } from '../types';

// DatePreset 表示订单下单时间筛选器中的快捷区间或自定义区间。
type DatePreset = 'all' | 'today' | 'yesterday' | 'last7Days' | 'last30Days' | 'custom';

// DatePresetOption 描述一个可见的时间快捷筛选标签。
interface DatePresetOption {
  // value 是组件内部使用的快捷区间键。
  value: DatePreset;
  // label 是用户看到的快捷区间名称。
  label: string;
}

// CalendarDay 描述内嵌月历中的一个日期单元格。
interface CalendarDay {
  // dateKey 是本地日期的 YYYY-MM-DD 键。
  dateKey: string;
  // dayNumber 是月历单元格显示的日期数字。
  dayNumber: number;
  // inCurrentMonth 表示日期是否属于当前浏览月份。
  inCurrentMonth: boolean;
  // disabled 表示未来日期不可用于订单筛选。
  disabled: boolean;
}

// OrderDateFilterProps 描述已应用时间范围和向订单查询层提交新范围的回调。
interface OrderDateFilterProps {
  // value 是当前已应用的 UTC 半开区间和本地摘要。
  value: OrderCreatedRange;
  // onChange 在快捷选择或应用自定义范围时更新订单查询条件。
  onChange: (value: OrderCreatedRange) => void;
}

// datePresetOptions 定义时间弹层内的快捷区间和自定义入口。
const datePresetOptions: ReadonlyArray<DatePresetOption> = [
  { value: 'all', label: '全部' },
  { value: 'today', label: '今天' },
  { value: 'yesterday', label: '昨天' },
  { value: 'last7Days', label: '近 7 天' },
  { value: 'last30Days', label: '近 30 天' },
  { value: 'custom', label: '自定义' },
];

// weekdayLabels 按周一到周日定义内嵌月历表头。
const weekdayLabels = ['一', '二', '三', '四', '五', '六', '日'] as const;

// padTwoDigits 把小时、分钟和日期数字补齐为两位文本。
const padTwoDigits = (value: number): string => String(value).padStart(2, '0');

// formatDateKey 把本地日期转换为 YYYY-MM-DD，避免预览阶段引入 UTC 日期偏移。
const formatDateKey = (date: Date): string => `${date.getFullYear()}-${padTwoDigits(date.getMonth() + 1)}-${padTwoDigits(date.getDate())}`;

// startOfMonth 返回输入日期所在月份的本地月初。
const startOfMonth = (date: Date): Date => new Date(date.getFullYear(), date.getMonth(), 1);

// shiftMonth 返回相对当前月初偏移指定月数后的新月初。
const shiftMonth = (month: Date, offset: number): Date => new Date(month.getFullYear(), month.getMonth() + offset, 1);

// addLocalDays 在浏览器本地日历中偏移指定天数，自动处理月份和夏令时边界。
const addLocalDays = (date: Date, days: number): Date => new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);

// startOfLocalDay 返回浏览器本地日期的零点。
const startOfLocalDay = (date: Date): Date => new Date(date.getFullYear(), date.getMonth(), date.getDate());

// parseLocalDateTime 把 YYYY-MM-DD 与小时分钟组合为浏览器本地时间。
const parseLocalDateTime = (dateKey: string, hour: string, minute: string): Date => {
  // dateParts 保存年、月、日三个本地日期数字。
  const dateParts = dateKey.split('-').map(/* datePartParser 把日期片段转换为整数。 */ part => Number(part));
  return new Date(dateParts[0], dateParts[1] - 1, dateParts[2], Number(hour), Number(minute));
};

// quickRangeFromPreset 把快捷区间转换为前端提交的 UTC 半开时间范围。
const quickRangeFromPreset = (preset: Exclude<DatePreset, 'custom'>): OrderCreatedRange => {
  if (preset === 'all') {
    return { createdFrom: '', createdTo: '', label: '全部时间' };
  }
  // todayStart 是浏览器本地今天零点。
  const todayStart = startOfLocalDay(new Date());
  // tomorrowStart 是浏览器本地明天零点，也是包含今天的排除上界。
  const tomorrowStart = addLocalDays(todayStart, 1);
  // startDayOffset 是快捷范围相对今天零点的开始天数。
  const startDayOffset = preset === 'yesterday' ? -1 : preset === 'last7Days' ? -6 : preset === 'last30Days' ? -29 : 0;
  // rangeStart 是快捷区间的浏览器本地包含下界。
  const rangeStart = addLocalDays(todayStart, startDayOffset);
  // rangeEnd 是快捷区间的浏览器本地排除上界；昨天只到今天零点，其余范围包含今天。
  const rangeEnd = preset === 'yesterday' ? todayStart : tomorrowStart;
  // label 是快捷区间在筛选入口显示的名称。
  const label = datePresetOptions.find(/* quickPresetLabel 定位当前快捷范围的中文文案。 */ option => option.value === preset)?.label || '全部时间';
  return { createdFrom: rangeStart.toISOString(), createdTo: rangeEnd.toISOString(), label };
};

// buildCalendarDays 生成覆盖六周的 42 个星期一开头月历单元格。
const buildCalendarDays = (month: Date, todayKey: string): CalendarDay[] => {
  // firstDay 是当前浏览月份的本地月初。
  const firstDay = startOfMonth(month);
  // mondayOffset 把 JavaScript 的周日开头序号换算为周一开头的前置天数。
  const mondayOffset = (firstDay.getDay() + 6) % 7;
  // gridStart 是六周月历中第一个可见日期。
  const gridStart = new Date(firstDay.getFullYear(), firstDay.getMonth(), 1 - mondayOffset);
  return Array.from({ length: 42 }, /* calendarDayFactory 创建当前月历格对应的日期信息。 */ (_, dayOffset) => {
    // currentDate 是从月历起点偏移后的当前日期。
    const currentDate = new Date(gridStart.getFullYear(), gridStart.getMonth(), gridStart.getDate() + dayOffset);
    // dateKey 是当前日期可比较且可提交的本地日期键。
    const dateKey = formatDateKey(currentDate);
    return {
      dateKey,
      dayNumber: currentDate.getDate(),
      inCurrentMonth: currentDate.getMonth() === firstDay.getMonth(),
      disabled: dateKey > todayKey,
    };
  });
};

// TimePartInputProps 描述一个不打开原生菜单的两位小时或分钟输入框。
interface TimePartInputProps {
  // ariaLabel 是输入框提供给辅助技术的字段名称。
  ariaLabel: string;
  // value 是当前两位小时或分钟文本。
  value: string;
  // max 是当前字段允许的最大数值。
  max: number;
  // onChange 把清洗后的数字文本写回父组件状态。
  onChange: (value: string) => void;
}

// normalizeTimePart 把小时或分钟限制到合法范围并补齐两位。
const normalizeTimePart = (value: string, max: number): string => {
  // numericValue 是输入文本转换后的有限整数，空值按零处理。
  const numericValue = Number.parseInt(value || '0', 10);
  return padTwoDigits(Math.min(max, Math.max(0, Number.isFinite(numericValue) ? numericValue : 0)));
};

// isTimePartInvalid 判断小时或分钟文本是否为空或超出允许范围。
const isTimePartInvalid = (value: string, max: number): boolean => value === '' || Number(value) > max;

// TimePartInput 渲染可直接键入并支持上下方向键微调的两位时间字段。
const TimePartInput: React.FC<TimePartInputProps> = ({ ariaLabel, value, max, onChange }) => {
  // invalid 表示当前输入尚未形成合法小时或分钟。
  const invalid = isTimePartInvalid(value, max);
  // inputRef 指向真实时间输入框，用于安装可阻止页面滚动的非被动 wheel 监听器。
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  // valueRef 保存滚轮连续事件使用的最新数字，避免 React 重渲染前重复读取旧值。
  const valueRef = React.useRef(value);
  valueRef.current = value;
  // wheelDeltaRef 累积当前触摸板手势的垂直滚动距离，避免微小抖动误触发。
  const wheelDeltaRef = React.useRef(0);
  // wheelResetTimerRef 保存手势结束判定定时器，由下一事件或组件卸载负责清理。
  const wheelResetTimerRef = React.useRef<number | null>(null);
  // resetWheelGesture 清空滚动累计，下一次独立滑动会重新计算力度。
  const resetWheelGesture = React.useCallback(/* wheelGestureReset 重置当前时间输入的触摸板手势状态。 */ () => {
    wheelDeltaRef.current = 0;
    wheelResetTimerRef.current = null;
  }, []);

  React.useEffect(/* nativeWheelLifecycle 在真实输入框上安装非被动滚轮监听，并在重渲染或卸载时完整清理。 */ () => {
    // input 是当前需要独占触摸板手势的真实时间输入框。
    const input = inputRef.current;
    if (!input) {
      return undefined;
    }
    // handleNativeWheel 把一次独立触摸板或鼠标滚轮手势转换为一个单位，并阻止事件继续滚动页面。
    const handleNativeWheel = (event: WheelEvent): void => {
      event.preventDefault();
      event.stopPropagation();
      input.focus({ preventScroll: true });
      if (wheelResetTimerRef.current !== null) {
        window.clearTimeout(wheelResetTimerRef.current);
      }
      wheelResetTimerRef.current = window.setTimeout(resetWheelGesture, 160);
      wheelDeltaRef.current += event.deltaY;
      // wheelStepThreshold 是每调整一个小时或一分钟所需的累计滚动像素。
      const wheelStepThreshold = 28;
      // accumulatedDistance 是当前连续手势的绝对滚动距离。
      const accumulatedDistance = Math.abs(wheelDeltaRef.current);
      if (accumulatedDistance < wheelStepThreshold) {
        return;
      }
      // direction 把向上滑转换为加一、向下滑转换为减一。
      const direction = wheelDeltaRef.current < 0 ? 1 : -1;
      // rawSteps 是当前力度按阈值换算出的原始变化量。
      const rawSteps = Math.floor(accumulatedDistance / wheelStepThreshold);
      // appliedSteps 把单批变化限制为最多六个单位，避免惯性滚动失控。
      const appliedSteps = Math.min(rawSteps, 6);
      // currentValue 是连续事件链中已经更新到的最新时间数字。
      const currentValue = Number(normalizeTimePart(valueRef.current, max));
      // nextValue 是应用方向、力度和合法边界后的新数字。
      const nextValue = padTwoDigits(Math.min(max, Math.max(0, currentValue + direction * appliedSteps)));
      valueRef.current = nextValue;
      onChange(nextValue);
      // remainingDistance 保留不足一个步长的余量，同时丢弃超过单批上限的惯性峰值。
      const remainingDistance = accumulatedDistance % wheelStepThreshold;
      wheelDeltaRef.current = direction > 0 ? -remainingDistance : remainingDistance;
    };
    input.addEventListener('wheel', handleNativeWheel, { passive: false });
    // cleanupNativeWheel 释放原生监听器和仍存活的手势定时器。
    const cleanupNativeWheel = /* nativeWheelCleanup 保证重渲染后不会残留重复 wheel 监听器。 */ (): void => {
      input.removeEventListener('wheel', handleNativeWheel);
      if (wheelResetTimerRef.current !== null) {
        window.clearTimeout(wheelResetTimerRef.current);
      }
    };
    return cleanupNativeWheel;
  }, [max, onChange, resetWheelGesture, value]);
  // handleChange 只保留前两位数字，避免浏览器原生数字步进器和下拉菜单。
  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => onChange(event.target.value.replace(/\D/g, '').slice(0, 2));
  // handleBlur 在输入结束时限制范围并补齐为两位文本。
  const handleBlur = (event: React.FocusEvent<HTMLInputElement>) => onChange(normalizeTimePart(event.target.value, max));
  // handleFocus 自动全选当前两位数字，方便一次键入直接替换。
  const handleFocus = (event: React.FocusEvent<HTMLInputElement>) => event.currentTarget.select();
  // handleKeyDown 使用上下方向键按一分钟或一小时步进并阻止页面滚动。
  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') {
      return;
    }
    event.preventDefault();
    // direction 是方向键对应的增减步长。
    const direction = event.key === 'ArrowUp' ? 1 : -1;
    // currentValue 是当前合法化后的数字值。
    const currentValue = Number(normalizeTimePart(value, max));
    onChange(padTwoDigits(Math.min(max, Math.max(0, currentValue + direction))));
  };
  return <input ref={inputRef} type="text" inputMode="numeric" pattern="[0-9]*" maxLength={2} aria-label={ariaLabel} aria-invalid={invalid} value={value} onChange={handleChange} onBlur={handleBlur} onFocus={handleFocus} onKeyDown={handleKeyDown} className={`h-10 w-12 overscroll-contain rounded-lg border text-center font-mono text-sm outline-none transition-colors focus:ring-2 ${invalid ? 'border-red-200 bg-red-50 text-red-600 focus:ring-red-100' : 'border-transparent bg-white text-gray-700 focus:border-blue-200 focus:ring-blue-100'}`} />;
};

// OrderDateFilter 渲染下单时间入口、快捷筛选和不依赖浏览器原生面板的日期时间范围选择器。
export const OrderDateFilter: React.FC<OrderDateFilterProps> = ({ value, onChange }) => {
  // dateMenuOpen 表示时间筛选弹层是否展开；setDateMenuOpen 只控制当前前端预览交互。
  const [dateMenuOpen, setDateMenuOpen] = React.useState(false);
  // datePreset 保存当前选中的快捷或自定义区间；setDatePreset 更新入口摘要和选中态。
  const [datePreset, setDatePreset] = React.useState<DatePreset>('all');
  // rangeStartDate 保存自定义范围开始日期；setRangeStartDate 响应月历第一次点击。
  const [rangeStartDate, setRangeStartDate] = React.useState('');
  // rangeEndDate 保存自定义范围结束日期；setRangeEndDate 响应月历第二次点击。
  const [rangeEndDate, setRangeEndDate] = React.useState('');
  // calendarMonth 保存当前浏览月份；setCalendarMonth 响应月份导航。
  const [calendarMonth, setCalendarMonth] = React.useState(/* initialCalendarMonth 使用本地当前月份初始化月历。 */ () => startOfMonth(new Date()));
  // startHour 和 setStartHour 保存开始时间小时，默认从整点 00:00 开始。
  const [startHour, setStartHour] = React.useState('00');
  // startMinute 和 setStartMinute 保存开始时间分钟，默认值为整小时的 00 分。
  const [startMinute, setStartMinute] = React.useState('00');
  // endHour 和 setEndHour 保存结束时间小时，默认定位到当天最后一个整点 23:00。
  const [endHour, setEndHour] = React.useState('23');
  // endMinute 和 setEndMinute 保存结束时间分钟，默认 59 分以覆盖所选日期的完整一天。
  const [endMinute, setEndMinute] = React.useState('59');
  // todayKey 是本地今天的日期上限，未来日期不会进入订单筛选。
  const todayKey = formatDateKey(new Date());
  // calendarDays 是当前浏览月份需要渲染的六周日期网格。
  const calendarDays = buildCalendarDays(calendarMonth, todayKey);
  // currentMonthKey 是当前真实月份，用于禁止向未来月份导航。
  const currentMonthKey = formatDateKey(startOfMonth(new Date()));
  // nextMonthDisabled 表示当前已经浏览到真实月份，不能继续进入未来月份。
  const nextMonthDisabled = formatDateKey(calendarMonth) >= currentMonthKey;
  // startMinutes 和 endMinutes 将小时分钟转换为同日比较使用的总分钟数。
  const startMinutes = Number(startHour) * 60 + Number(startMinute);
  const endMinutes = Number(endHour) * 60 + Number(endMinute);
  // sameDayTimeInvalid 表示同一天内结束时间必须晚于开始时间。
  const sameDayTimeInvalid = Boolean(rangeStartDate && rangeEndDate && rangeStartDate === rangeEndDate && endMinutes <= startMinutes);
  // timePartsInvalid 表示任一小时或分钟仍为空或超出范围。
  const timePartsInvalid = isTimePartInvalid(startHour, 23) || isTimePartInvalid(startMinute, 59) || isTimePartInvalid(endHour, 23) || isTimePartInvalid(endMinute, 59);
  // allDaySelected 表示当前时间范围覆盖所选日期的完整一天。
  const allDaySelected = startHour === '00' && startMinute === '00' && endHour === '23' && endMinute === '59';
  // customRangeComplete 表示自定义日期时间范围可以提交到未来的查询层。
  const customRangeComplete = Boolean(rangeStartDate && rangeEndDate && !timePartsInvalid && !sameDayTimeInvalid);
  // dateFilterLabel 是筛选入口当前展示的区间摘要。
  const dateFilterLabel = value.label || '全部时间';

  // handleDateMenuToggle 切换时间筛选弹层，不触发真实订单查询。
  const handleDateMenuToggle = () => setDateMenuOpen(/* dateMenuToggle 反转时间弹层展开状态。 */ currentOpen => !currentOpen);
  // handleDatePresetClick 选择快捷区间；自定义入口保留弹层并显示内嵌月历。
  const handleDatePresetClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    // selectedPreset 是当前快捷标签携带的时间区间键。
    const selectedPreset = event.currentTarget.dataset.preset as DatePreset;
    setDatePreset(selectedPreset);
    if (selectedPreset !== 'custom') {
      onChange(quickRangeFromPreset(selectedPreset));
      setDateMenuOpen(false);
    }
  };
  // handleCalendarDayClick 按“开始后结束”的顺序更新自定义日期范围。
  const handleCalendarDayClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    // selectedDate 是当前月历按钮携带的本地日期键。
    const selectedDate = event.currentTarget.dataset.date || '';
    if (!rangeStartDate || rangeEndDate || selectedDate < rangeStartDate) {
      setRangeStartDate(selectedDate);
      setRangeEndDate('');
      return;
    }
    setRangeEndDate(selectedDate);
  };
  // handlePreviousMonth 把内嵌月历切换到上一个月。
  const handlePreviousMonth = () => setCalendarMonth(/* previousCalendarMonth 根据当前月份生成上一个月。 */ currentMonth => shiftMonth(currentMonth, -1));
  // handleNextMonth 在未达到真实月份上限时切换到下一个月。
  const handleNextMonth = () => setCalendarMonth(/* nextCalendarMonth 根据当前月份生成下一个月。 */ currentMonth => shiftMonth(currentMonth, 1));
  // handleAllDayTime 把开始和结束时间恢复为完整一天 00:00 至 23:59。
  const handleAllDayTime = () => {
    setStartHour('00');
    setStartMinute('00');
    setEndHour('23');
    setEndMinute('59');
  };
  // handleApplyCustomDate 使用当前完整日期时间范围更新入口摘要并关闭弹层。
  const handleApplyCustomDate = () => {
		// rangeStart 是用户选择的浏览器本地包含下界。
		const rangeStart = parseLocalDateTime(rangeStartDate, startHour, startMinute);
		// selectedEnd 是用户选择的浏览器本地结束分钟。
		const selectedEnd = parseLocalDateTime(rangeEndDate, endHour, endMinute);
		// rangeEnd 是结束分钟之后一分钟的排除上界，确保用户选择的分钟被完整包含。
		const rangeEnd = new Date(selectedEnd.getTime() + 60_000);
		onChange({
			createdFrom: rangeStart.toISOString(),
			createdTo: rangeEnd.toISOString(),
			label: `${rangeStartDate} ${startHour}:${startMinute} 至 ${rangeEndDate} ${endHour}:${endMinute}`,
		});
    setDatePreset('custom');
    setDateMenuOpen(false);
  };
  // handleClearDateFilter 清除日期范围并恢复默认全天 00:00 至 23:59。
  const handleClearDateFilter = () => {
    setRangeStartDate('');
    setRangeEndDate('');
    setStartHour('00');
    setStartMinute('00');
    setEndHour('23');
    setEndMinute('59');
    setDatePreset('all');
		onChange({ createdFrom: '', createdTo: '', label: '全部时间' });
    setDateMenuOpen(false);
  };

  return (
    <div className="relative">
      <button type="button" onClick={handleDateMenuToggle} className={`ios-input flex w-full items-center gap-2 rounded-xl border-none bg-white px-4 py-2.5 text-left text-sm font-medium shadow-sm transition-colors sm:w-44 ${datePreset !== 'all' ? 'text-brand' : 'text-gray-700'}`} aria-haspopup="dialog" aria-expanded={dateMenuOpen} aria-label={`按下单时间筛选，当前${dateFilterLabel}`} title={dateFilterLabel}>
        <CalendarDays className="h-4 w-4 shrink-0 text-gray-400" />
        <span className="min-w-0 flex-1 truncate">{dateFilterLabel}</span>
        <ChevronDown className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${dateMenuOpen ? 'rotate-180' : ''}`} />
      </button>
      {dateMenuOpen && (
        <div role="dialog" aria-label="下单时间筛选" className="absolute left-0 top-full z-40 mt-2 w-[420px] max-w-[calc(100vw-2rem)] rounded-2xl border border-gray-100 bg-white p-4 shadow-2xl shadow-gray-200/70 sm:left-auto sm:right-0">
          <div className="flex items-start justify-between">
            <div>
              <div className="text-sm font-bold text-gray-900">下单时间</div>
              <div className="mt-0.5 text-xs text-gray-400">快捷区间，或精确到小时和分钟</div>
            </div>
            <button type="button" onClick={handleDateMenuToggle} className="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600" aria-label="关闭下单时间筛选"><X className="h-4 w-4" /></button>
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            {datePresetOptions.map(
              // option 是下单时间弹层中的当前快捷区间标签。
              option => (
                <button type="button" key={option.value} data-preset={option.value} onClick={handleDatePresetClick} className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${datePreset === option.value ? 'bg-blue-50 text-brand' : 'bg-gray-50 text-gray-600 hover:bg-gray-100'}`}>{option.label}</button>
              ),
            )}
          </div>
          {datePreset === 'custom' && (
            <>
              <div className="mt-4 rounded-2xl bg-gray-50/80 p-3">
                <div className="flex items-center justify-between">
                  <button type="button" onClick={handlePreviousMonth} className="rounded-lg p-2 text-gray-500 transition-colors hover:bg-white hover:text-gray-800" aria-label="查看上一个月"><ChevronLeft className="h-4 w-4" /></button>
                  <div className="text-sm font-bold text-gray-800">{calendarMonth.getFullYear()}年{calendarMonth.getMonth() + 1}月</div>
                  <button type="button" onClick={handleNextMonth} disabled={nextMonthDisabled} className="rounded-lg p-2 text-gray-500 transition-colors hover:bg-white hover:text-gray-800 disabled:cursor-not-allowed disabled:opacity-25" aria-label="查看下一个月"><ChevronRight className="h-4 w-4" /></button>
                </div>
                <div className="mt-2 grid grid-cols-7 text-center text-[11px] font-medium text-gray-400">
                  {weekdayLabels.map(/* weekdayLabel 渲染当前星期表头。 */ weekdayLabel => <div key={weekdayLabel} className="py-1">{weekdayLabel}</div>)}
                </div>
                <div className="grid grid-cols-7 gap-y-1">
                  {calendarDays.map(
                    // day 是内嵌月历中的当前日期单元格。
                    day => {
                      // rangeEndpoint 表示日期是范围开始或结束端点。
                      const rangeEndpoint = day.dateKey === rangeStartDate || day.dateKey === rangeEndDate;
                      // insideRange 表示日期位于已选范围内部但不是端点。
                      const insideRange = Boolean(rangeStartDate && rangeEndDate && day.dateKey > rangeStartDate && day.dateKey < rangeEndDate);
                      // today 表示当前日期是本地今天。
                      const today = day.dateKey === todayKey;
                      return (
                        <button type="button" key={day.dateKey} data-date={day.dateKey} onClick={handleCalendarDayClick} disabled={day.disabled} className={`mx-auto flex h-9 w-9 items-center justify-center rounded-lg text-xs transition-colors ${rangeEndpoint ? 'bg-brand font-bold text-white shadow-sm' : insideRange ? 'bg-blue-50 text-blue-700' : today ? 'ring-1 ring-blue-300 text-blue-700 hover:bg-blue-50' : day.inCurrentMonth ? 'text-gray-700 hover:bg-white' : 'text-gray-300 hover:bg-white'} disabled:cursor-not-allowed disabled:opacity-25`} aria-label={`选择日期 ${day.dateKey}`}>{day.dayNumber}</button>
                      );
                    },
                  )}
                </div>
              </div>
              <div className="mt-3 rounded-2xl border border-gray-100 bg-gray-50/70 p-3">
                <div className="flex items-center justify-between">
                  <div className="text-xs font-bold text-gray-500">时间范围</div>
                  <button type="button" onClick={handleAllDayTime} className={`rounded-lg px-2.5 py-1 text-[11px] font-medium transition-colors ${allDaySelected ? 'bg-blue-50 text-brand' : 'bg-white text-gray-500 hover:bg-gray-100'}`}>全天</button>
                </div>
                <div className="mt-3 grid grid-cols-[1fr_auto_1fr] items-end gap-3">
                  <div>
                    <div className="mb-1.5 text-[11px] text-gray-400">开始</div>
                    <div className="flex items-center gap-1.5"><TimePartInput ariaLabel="开始小时" value={startHour} max={23} onChange={setStartHour} /><span className="font-bold text-gray-300">:</span><TimePartInput ariaLabel="开始分钟" value={startMinute} max={59} onChange={setStartMinute} /></div>
                  </div>
                  <ArrowRight className="mb-2.5 h-4 w-4 text-gray-300" />
                  <div>
                    <div className="mb-1.5 text-[11px] text-gray-400">结束</div>
                    <div className="flex items-center justify-end gap-1.5"><TimePartInput ariaLabel="结束小时" value={endHour} max={23} onChange={setEndHour} /><span className="font-bold text-gray-300">:</span><TimePartInput ariaLabel="结束分钟" value={endMinute} max={59} onChange={setEndMinute} /></div>
                  </div>
                </div>
              </div>
              <div className={`mt-3 rounded-xl px-3 py-2.5 text-xs ${timePartsInvalid || sameDayTimeInvalid ? 'bg-red-50 text-red-600' : 'bg-blue-50/70 text-blue-700'}`}>
                {timePartsInvalid ? '请输入合法的小时和分钟' : sameDayTimeInvalid ? '结束时间必须晚于开始时间' : rangeStartDate && rangeEndDate ? `${rangeStartDate} ${startHour}:${startMinute} 至 ${rangeEndDate} ${endHour}:${endMinute}` : rangeStartDate ? '请选择结束日期' : '请选择开始日期'}
              </div>
              <div className="mt-2 text-[11px] text-gray-400">直接输入或上下滑动；滑动力度决定步数，单批最多 6。</div>
            </>
          )}
          <div className="mt-4 flex items-center justify-end gap-2 border-t border-gray-100 pt-3">
            <button type="button" onClick={handleClearDateFilter} className="rounded-xl px-3 py-2 text-sm text-gray-500 transition-colors hover:bg-gray-100">清除</button>
            {datePreset === 'custom' && <button type="button" onClick={handleApplyCustomDate} disabled={!customRangeComplete} className="rounded-xl bg-brand px-4 py-2 text-sm font-bold text-white transition-opacity disabled:cursor-not-allowed disabled:opacity-30">应用筛选</button>}
          </div>
        </div>
      )}
    </div>
  );
};
