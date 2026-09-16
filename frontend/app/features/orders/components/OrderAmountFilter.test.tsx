// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen } from '@testing-library/react';
import { afterEach,expect,test,vi } from 'vitest';
import { OrderAmountFilter } from './OrderAmountFilter';

afterEach(/* amountFilterCleanup 在每个金额筛选用例后清理挂载节点。 */ () => cleanup());

// renderAmountFilter 使用默认未筛选状态渲染金额组件并返回变更替身。
const renderAmountFilter = () => {
  // onChange 是金额筛选应用结果的可控替身。
  const onChange = vi.fn();
  render(<OrderAmountFilter value={{ minAmount: '', maxAmount: '', label: '金额范围' }} onChange={onChange} />);
  return onChange;
};

test('金额筛选支持双边、单边和清除', /* 当前回调验证金额筛选的主要交互。 */ () => {
  // onChange 是当前金额筛选应用结果替身。
  const onChange = renderAmountFilter();
  fireEvent.click(screen.getByRole('button', { name: '按实付金额筛选，当前金额范围' }));
  fireEvent.change(screen.getByLabelText('最低金额'), { target: { value: '135.00' } });
  fireEvent.change(screen.getByLabelText('最高金额'), { target: { value: '690' } });
  fireEvent.click(screen.getByRole('button', { name: '应用筛选' }));
  expect(onChange).toHaveBeenLastCalledWith({ minAmount: '135.00', maxAmount: '690', label: '¥135.00–¥690' });

  fireEvent.click(screen.getByRole('button', { name: '按实付金额筛选，当前金额范围' }));
  fireEvent.change(screen.getByLabelText('最高金额'), { target: { value: '100' } });
  fireEvent.click(screen.getByRole('button', { name: '清除' }));
  expect(onChange).toHaveBeenLastCalledWith({ minAmount: '', maxAmount: '', label: '金额范围' });
});

test('金额筛选拒绝超过两位小数和倒置范围', /* 当前回调验证金额草稿错误不会提交查询。 */ () => {
  // onChange 是当前非法金额场景的应用结果替身。
  const onChange = renderAmountFilter();
  fireEvent.click(screen.getByRole('button', { name: '按实付金额筛选，当前金额范围' }));
  fireEvent.change(screen.getByLabelText('最低金额'), { target: { value: '1.001' } });
  fireEvent.click(screen.getByRole('button', { name: '应用筛选' }));
  expect(screen.getByRole('alert').textContent).toContain('最多两位小数');
  expect(onChange).not.toHaveBeenCalled();

  fireEvent.change(screen.getByLabelText('最低金额'), { target: { value: '20' } });
  fireEvent.change(screen.getByLabelText('最高金额'), { target: { value: '10' } });
  fireEvent.click(screen.getByRole('button', { name: '应用筛选' }));
  expect(screen.getByRole('alert').textContent).toContain('最高金额不能低于最低金额');
  expect(onChange).not.toHaveBeenCalled();
});
