// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen } from '@testing-library/react';
import React from 'react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import { DashboardDateRangePicker } from './DashboardDateRangePicker';

describe('DashboardDateRangePicker', /* dateRangePickerSuite 验证应用内日期范围选择不依赖浏览器原生面板。 */ () => {
  beforeEach(/* fixedCalendarClock 固定系统月份，防止日历用例随真实日期漂移。 */ () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 7, 15, 12));
  });

  afterEach(/* dateRangePickerCleanup 清理弹层 DOM 并恢复真实时钟。 */ () => {
    cleanup();
    vi.useRealTimers();
  });

  test('selects a start and end day before applying', /* rangeSelectionCase 验证先起点后终点的受控交互。 */ () => {
    // applyAction 记录用户完成范围后提交的动作。
    const applyAction = vi.fn();
    // RangeHarness 提供最小受控状态，模拟 Dashboard 保存日期草稿。
    const RangeHarness: React.FC = () => {
      // [startDate, setStartDate] 保存测试中的开始日期。
      const [startDate, setStartDate] = React.useState('');
      // [endDate, setEndDate] 保存测试中的结束日期。
      const [endDate, setEndDate] = React.useState('');
      return <DashboardDateRangePicker startDate={startDate} endDate={endDate} onStartDateChange={setStartDate} onEndDateChange={setEndDate} onApply={applyAction} onCancel={/* cancelAction 本用例不需要关闭宿主。 */ () => undefined} />;
    };
    render(<RangeHarness />);
    // applyButton 是范围完整前保持禁用的提交按钮。
    const applyButton = screen.getByRole('button', { name: '应用范围' });
    expect((applyButton as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: '2026-08-10' }));
    expect(screen.getByText('8月10日')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '2026-08-20' }));
    expect(screen.getByText('8月20日')).toBeTruthy();
    expect((applyButton as HTMLButtonElement).disabled).toBe(false);
    fireEvent.click(applyButton);
    expect(applyAction).toHaveBeenCalledTimes(1);
  });

  test('provides month navigation and clear without native date inputs', /* calendarControlsCase 验证紧凑弹层的月份与清除入口。 */ () => {
    // startChange 记录清除开始日期时的受控更新。
    const startChange = vi.fn();
    // endChange 记录清除结束日期时的受控更新。
    const endChange = vi.fn();
    render(<DashboardDateRangePicker startDate="2026-08-10" endDate="2026-08-20" onStartDateChange={startChange} onEndDateChange={endChange} onApply={/* applyAction 本用例不提交。 */ () => undefined} onCancel={/* cancelAction 本用例不关闭。 */ () => undefined} />);
    expect(document.querySelector('input[type="date"]')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '下一个月' }));
    expect(screen.getByText('2026年9月')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '清除' }));
    expect(startChange).toHaveBeenCalledWith('');
    expect(endChange).toHaveBeenCalledWith('');
  });
});
