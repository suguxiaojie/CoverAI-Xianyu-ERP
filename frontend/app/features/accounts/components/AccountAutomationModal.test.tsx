// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,expect,test,vi } from 'vitest';
import type { AccountDetail,AccountTaskSettings } from '../api';
import { useAccountAutomation } from '../accountAutomationHooks';
import AccountAutomationModal from './AccountAutomationModal';

vi.mock('../accountAutomationHooks', /* automationHookMockFactory 提供账号自动化弹窗的确定性状态。 */ () => ({
  useAccountAutomation: vi.fn(),
}));

// useAutomationMock 是账号自动化 Hook 的类型化替身。
const useAutomationMock = vi.mocked(useAccountAutomation);
// setFormMock 记录小红花配置控件提交的函数式更新。
const setFormMock = vi.fn();
// saveMock 记录弹窗保存动作。
const saveMock = vi.fn();

// accountFixture 是弹窗展示使用的启用账号摘要。
const accountFixture: AccountDetail = { id: 'account-1', nickname: '账号一', enabled: true, auto_confirm: false };
// settingsFixture 是包含两个小红花开关和执行参数的完整表单。
const settingsFixture: AccountTaskSettings = {
  account_id: 'account-1', auto_rate_enabled: false, rate_content: '交易愉快', auto_polish_enabled: false, polish_time: '03:00',
  auto_request_flower_enabled: true, request_flower_after_hours: 6, request_flower_after_seconds: 10, auto_receive_flower_enabled: true,
  receive_flower_show_browser: true, receive_flower_timeout_seconds: 180,
	auto_receipt_reminder_enabled: true, receipt_reminder_after_days: 2, receipt_reminder_time: '10:00',
	receipt_reminder_message: '确认无误后麻烦确认收货，谢谢。', receipt_reminder_enabled_at: 1_787_600_000,
};

beforeEach(/* 当前回调重置账号自动化弹窗替身。 */ () => {
  vi.clearAllMocks();
  useAutomationMock.mockReturnValue({
    form: settingsFixture, loading: false, saving: false, running: '', error: '', summary: null, saved: false,
    retryAvailable: false, setForm: setFormMock, save: saveMock, run: vi.fn(), retry: vi.fn(),
  });
});

afterEach(/* 当前回调清理账号自动化弹窗测试 DOM 和浏览器方法替身。 */ () => {
	cleanup();
	vi.restoreAllMocks();
});

test('账号自动化弹窗保存成功后展示明确反馈并禁用重复保存', /* 当前回调验证保存成功不再表现为无响应。 */ () => {
  useAutomationMock.mockReturnValue({
    form: settingsFixture, loading: false, saving: false, running: '', error: '', summary: null, saved: true,
    retryAvailable: false, setForm: setFormMock, save: saveMock, run: vi.fn(), retry: vi.fn(),
  });
  render(<AccountAutomationModal account={accountFixture} onClose={vi.fn()} onSaved={vi.fn()} />);
  expect(screen.getByRole('status').textContent).toContain('设置已保存');
	expect(screen.getByRole('status').textContent).toContain('发货满 2 天后的 10:00');
  expect((screen.getByRole('button', { name: '已保存' }) as HTMLButtonElement).disabled).toBe(true);
});

test('账号自动化弹窗配置确认收货系统卡片天数和时间', /* receiptReminderFormCase 验证每店铺官方卡片设置。 */ () => {
	render(<AccountAutomationModal account={accountFixture} onClose={vi.fn()} onSaved={vi.fn()} />);
	expect(screen.getByText('自动确认收货提醒')).toBeTruthy();
	expect((screen.getByLabelText('确认收货提醒等待天数') as HTMLInputElement).value).toBe('2');
	expect((screen.getByLabelText('确认收货提醒执行时间') as HTMLInputElement).value).toBe('10:00');
	expect(screen.getByText(/闲鱼官方接口生成/)).toBeTruthy();
	expect(screen.queryByLabelText('确认收货提醒文案')).toBeNull();
	fireEvent.change(screen.getByLabelText('确认收货提醒等待天数'), { target: { value: '3' } });
	// updater 是等待天数输入交给 Hook 的函数式草稿更新器。
	const updater = setFormMock.mock.calls.at(-1)?.[0] as (current: AccountTaskSettings) => AccountTaskSettings;
	expect(updater(settingsFixture).receipt_reminder_after_days).toBe(3);
});

test('账号自动化弹窗展示独立求花和收花配置并提交字段更新', /* 当前回调验证小红花设置的响应式表单行为。 */ () => {
  render(<AccountAutomationModal account={accountFixture} onClose={vi.fn()} onSaved={vi.fn()} />);
  expect(screen.getByText('自动求小红花')).toBeTruthy();
  expect(screen.getByText('自动收小红花')).toBeTruthy();
  expect((screen.getByLabelText('自动求花延迟秒数') as HTMLInputElement).value).toBe('10');
  expect((screen.getByLabelText('自动收花超时秒数') as HTMLInputElement).value).toBe('180');
  expect(screen.getByRole('button', { name: '显示收花浏览器' }).getAttribute('aria-pressed')).toBe('true');
  fireEvent.change(screen.getByLabelText('自动求花延迟秒数'), { target: { value: '12' } });
  // updater 是输入控件交给 React 状态的函数式更新器。
  const updater = setFormMock.mock.calls[0][0] as (current: AccountTaskSettings) => AccountTaskSettings;
  expect(updater(settingsFixture).request_flower_after_seconds).toBe(12);
  fireEvent.click(screen.getByText('保存'));
  expect(saveMock).toHaveBeenCalledTimes(1);
});

test('账号自动化弹窗按控制中心入口滚动到对应设置', /* 当前回调验证五个任务卡片不会只打开弹窗顶部。 */ async () => {
	// scrolledElement 保存 scrollIntoView 最近一次接收的任务区块。
	let scrolledElement: HTMLElement | null = null;
	// scrollIntoViewMock 记录当前滚动目标，不改变 jsdom 页面位置。
	const scrollIntoViewMock = vi.fn(/* recordScrollTarget 保存当前自动任务滚动目标。 */ function recordScrollTarget(this: HTMLElement) { scrolledElement = this; });
	Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: scrollIntoViewMock });
	vi.spyOn(window, 'requestAnimationFrame').mockImplementation(/* frameCallback 立即执行弹窗布局后的定位回调。 */ callback => { callback(0); return 1; });
	// onClose 和 onSaved 是本测试所有重渲染共用的稳定弹窗回调。
	const onClose = vi.fn();
	const onSaved = vi.fn();
	// view 保存弹窗渲染结果，用不同卡片目标重渲染同一个已打开弹窗。
	const view = render(<AccountAutomationModal account={accountFixture} initialFocus="auto_rate" onClose={onClose} onSaved={onSaved} />);
	for (const /* focusCase 是当前控制中心入口及其应定位的区块测试 ID。 */ focusCase of [
		{ focus: 'auto_rate' as const, testID: 'account-task-section-auto-rate' },
		{ focus: 'receipt_reminder' as const, testID: 'account-task-section-receipt-reminder' },
		{ focus: 'auto_request_flower' as const, testID: 'account-task-section-auto-request-flower' },
		{ focus: 'auto_receive_flower' as const, testID: 'account-task-section-auto-receive-flower' },
		{ focus: 'auto_polish' as const, testID: 'account-task-section-auto-polish' },
	]) {
		view.rerender(<AccountAutomationModal account={accountFixture} initialFocus={focusCase.focus} onClose={onClose} onSaved={onSaved} />);
		await waitFor(/* scrollAssertion 等待当前目标区块完成定位。 */ () => expect(scrolledElement).toBe(screen.getByTestId(focusCase.testID)));
	}
});

test('账号自动化弹窗按 Escape 关闭并在卸载后释放监听', /* 当前回调验证键盘取消不会触发保存。 */ () => {
	// onClose 是 Escape 应调用的既有关闭动作。
	const onClose = vi.fn();
	// view 保存弹窗实例，用于确认卸载后监听器不再响应。
	const view = render(<AccountAutomationModal account={accountFixture} onClose={onClose} onSaved={vi.fn()} />);
	fireEvent.keyDown(document, { key: 'Escape' });
	expect(onClose).toHaveBeenCalledTimes(1);
	expect(saveMock).not.toHaveBeenCalled();
	view.unmount();
	fireEvent.keyDown(document, { key: 'Escape' });
	expect(onClose).toHaveBeenCalledTimes(1);
});
