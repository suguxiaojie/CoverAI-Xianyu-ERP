// @vitest-environment jsdom
import { act,cleanup,fireEvent,render,screen } from '@testing-library/react';
import React from 'react';
import { afterEach,expect,test } from 'vitest';
import type { OrderCreatedRange } from '../types';
import { OrderDateFilter } from './OrderDateFilter';

// formatLocalDateKey 把测试日期转换为组件使用的本地 YYYY-MM-DD 键。
const formatLocalDateKey = (date: Date): string => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;

// DateFilterHarness 保存已应用范围并把 UTC 边界暴露给行为断言。
const DateFilterHarness: React.FC = () => {
  // range 和 setRange 模拟订单查询 Hook 持有的受控时间范围。
  const [range, setRange] = React.useState<OrderCreatedRange>({ createdFrom: '', createdTo: '', label: '全部时间' });
  return <><OrderDateFilter value={range} onChange={setRange} /><output data-testid="range-value">{JSON.stringify(range)}</output></>;
};

afterEach(/* dateFilterCleanup 清理时间筛选测试 DOM。 */ () => cleanup());

test('自定义日期时间提交 UTC 半开区间并包含结束分钟', /* 当前回调验证本地日期时间到传输契约的转换。 */ () => {
  // today 是组件允许选择的本地当前日期。
  const today = new Date();
  // startDay 是当前日期前两天的本地范围开始日。
  const startDay = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 2);
  // startKey 是月历开始按钮使用的本地日期键。
  const startKey = formatLocalDateKey(startDay);
  // endKey 是月历结束按钮使用的本地日期键。
  const endKey = formatLocalDateKey(today);
  render(<DateFilterHarness />);
  fireEvent.click(screen.getByRole('button', { name: /按下单时间筛选/ }));
  fireEvent.click(screen.getByRole('button', { name: '自定义' }));
  fireEvent.click(screen.getByRole('button', { name: `选择日期 ${startKey}` }));
  fireEvent.click(screen.getByRole('button', { name: `选择日期 ${endKey}` }));
  fireEvent.change(screen.getByRole('textbox', { name: '开始小时' }), { target: { value: '09' } });
  fireEvent.change(screen.getByRole('textbox', { name: '开始分钟' }), { target: { value: '17' } });
  fireEvent.change(screen.getByRole('textbox', { name: '结束小时' }), { target: { value: '18' } });
  fireEvent.change(screen.getByRole('textbox', { name: '结束分钟' }), { target: { value: '43' } });
  fireEvent.click(screen.getByRole('button', { name: '应用筛选' }));
  // expectedFrom 是包含用户选择开始分钟的 UTC 下界。
  const expectedFrom = new Date(startDay.getFullYear(), startDay.getMonth(), startDay.getDate(), 9, 17).toISOString();
  // expectedTo 是用户选择结束分钟下一分钟的 UTC 排除上界。
  const expectedTo = new Date(today.getFullYear(), today.getMonth(), today.getDate(), 18, 44).toISOString();
  expect(screen.getByTestId('range-value').textContent).toContain(expectedFrom);
  expect(screen.getByTestId('range-value').textContent).toContain(expectedTo);
});

test('滚轮力度按距离换算步数并阻止默认滚动', /* 当前回调验证非被动 wheel 监听和单批上限。 */ async () => {
  render(<DateFilterHarness />);
  fireEvent.click(screen.getByRole('button', { name: /按下单时间筛选/ }));
  fireEvent.click(screen.getByRole('button', { name: '自定义' }));
  // minuteInput 是待验证滚轮力度和默认滚动隔离的开始分钟输入框。
  const minuteInput = screen.getByRole('textbox', { name: '开始分钟' }) as HTMLInputElement;
  // lightWheel 是恰好一个阈值的向上滚轮事件。
  const lightWheel = new WheelEvent('wheel', { deltaY: -28, bubbles: true, cancelable: true });
  await act(/* lightWheelAction 派发轻微滚动并等待受控状态提交。 */ async () => { minuteInput.dispatchEvent(lightWheel); });
  expect(lightWheel.defaultPrevented).toBe(true);
  expect(minuteInput.value).toBe('01');
  await act(/* wheelResetWait 等待一次手势余量按产品阈值清零。 */ async () => { await new Promise(/* wheelResetExecutor 在手势重置窗口后完成等待。 */ resolve => window.setTimeout(resolve, 180)); });
  fireEvent.click(screen.getByRole('button', { name: '全天' }));
  // strongWheel 是理论十步但应被限制为六步的强滚轮事件。
  const strongWheel = new WheelEvent('wheel', { deltaY: -280, bubbles: true, cancelable: true });
  await act(/* strongWheelAction 派发强滚动并等待受控状态提交。 */ async () => { minuteInput.dispatchEvent(strongWheel); });
  expect(strongWheel.defaultPrevented).toBe(true);
  expect(minuteInput.value).toBe('06');
});
