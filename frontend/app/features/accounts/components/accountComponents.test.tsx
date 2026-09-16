// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,describe,expect,test,vi } from 'vitest';
import type { AccountDetail,AIReplySettings } from '../../../../shared/api-contract/accounts';
import { AccountAISettingsModal } from './AccountAISettingsModal';
import { AccountCard } from './AccountCard';
import { AccountDeleteDialog } from './AccountDeleteDialog';
import { AccountEditModal } from './AccountEditModal';
import { AccountQRCodeModal } from './AccountQRCodeModal';
import type { AccountEditForm } from '../types';

// accountFixture 是账号组件测试使用的最小非敏感账号摘要。
const accountFixture = {
  id: 'account-1',
  nickname: '测试账号',
  remark: '测试备注',
  enabled: true,
  runtime_state: 'online',
  runtime_message: '',
  ai_enabled: true,
  auto_rate_enabled: false,
  auto_polish_enabled: false,
  auto_confirm: false,
  paused: false,
} as AccountDetail;

// aiSettingsFixture 是 AI 设置弹窗测试使用的编辑草稿。
const aiSettingsFixture: AIReplySettings = {
  ai_enabled: false,
  max_discount_percent: 10,
  max_discount_amount: 100,
  max_bargain_rounds: 3,
  custom_prompts: '',
};

// noopAccountAction 是账号卡片测试使用的动作占位函数。
const noopAccountAction = (): void => undefined;

// manualEditFormFixture 是账号编辑弹窗人工风控模式测试使用的非敏感表单草稿。
const manualEditFormFixture: AccountEditForm = {
  remark: '',
  cookie: '',
  auto_confirm: false,
  pause_duration: 0,
  username: '',
  login_password: '',
  show_browser: true,
  showLoginPassword: false,
  clear_password: false,
};

describe('账号 feature 展示组件', /* 当前回调覆盖账号页面子模块展示边界。 */ () => {
	 afterEach(/* 当前回调清理 Portal、测试 DOM 和浏览器方法替身。 */ () => {
		cleanup();
		vi.restoreAllMocks();
	 });

  test('账号卡片展示状态并转发所有操作', /* 当前回调验证账号卡片的操作边界。 */ () => {
    // onDelete 是删除操作测试替身。
    const onDelete = vi.fn();
    // onAI 是 AI 设置操作测试替身。
    const onAI = vi.fn();
	// onToggle 是启停操作测试替身。
	const onToggle = vi.fn();
	// onEdit 记录控制中心卡片打开的账号编辑弹窗及精确定位目标。
	const onEdit = vi.fn();
	// onTasks 记录控制中心卡片打开的自动任务弹窗及精确定位目标。
	const onTasks = vi.fn();
	render(<AccountCard account={accountFixture} refreshing={false} deleting={false} onRefreshProfile={noopAccountAction} onReauthorize={noopAccountAction} onEdit={onEdit} onAI={onAI} onTasks={onTasks} onToggle={onToggle} onDelete={onDelete} />);
    expect(screen.getByText('测试账号')).toBeTruthy();
    fireEvent.click(screen.getByTitle('AI设置'));
    fireEvent.click(screen.getByTitle('停用账号'));
    fireEvent.click(screen.getByTitle('删除账号 测试账号'));
    expect(onAI).toHaveBeenCalledWith(accountFixture);
    expect(onToggle).toHaveBeenCalledWith(accountFixture.id, accountFixture.enabled);
	expect(onDelete).toHaveBeenCalledWith(accountFixture);
	fireEvent.click(screen.getByRole('button', { name: '配置AI 回复' }));
	fireEvent.click(screen.getByRole('button', { name: '配置自动评价' }));
	fireEvent.click(screen.getByRole('button', { name: '配置每日擦亮' }));
	fireEvent.click(screen.getByRole('button', { name: '配置自动确认发货' }));
	fireEvent.click(screen.getByRole('button', { name: '配置收货提醒' }));
	fireEvent.click(screen.getByRole('button', { name: '配置自动求花' }));
	fireEvent.click(screen.getByRole('button', { name: '配置自动收花' }));
	expect(onAI).toHaveBeenNthCalledWith(2, accountFixture);
	expect(onTasks).toHaveBeenNthCalledWith(1, accountFixture, 'auto_rate');
	expect(onTasks).toHaveBeenNthCalledWith(2, accountFixture, 'auto_polish');
	expect(onEdit).toHaveBeenCalledWith(accountFixture, 'auto_confirm');
	expect(onTasks).toHaveBeenNthCalledWith(3, accountFixture, 'receipt_reminder');
	expect(onTasks).toHaveBeenNthCalledWith(4, accountFixture, 'auto_request_flower');
	expect(onTasks).toHaveBeenNthCalledWith(5, accountFixture, 'auto_receive_flower');
  });

  test('AI 设置弹窗使用补丁更新并转发保存', /* 当前回调验证 AI 设置字段更新和保存动作。 */ () => {
    // onChange 是 AI 设置草稿更新测试替身。
    const onChange = vi.fn();
    // onSave 是 AI 设置保存测试替身。
    const onSave = vi.fn();
    render(<AccountAISettingsModal account={accountFixture} settings={aiSettingsFixture} saving={false} onChange={onChange} onClose={noopAccountAction} onSave={onSave} />);
    fireEvent.click(screen.getByLabelText('切换 AI 自动回复'));
    fireEvent.change(screen.getByDisplayValue('10'), { target: { value: '20' } });
    fireEvent.click(screen.getByText('保存'));
    expect(onChange).toHaveBeenNthCalledWith(1, { ...aiSettingsFixture, ai_enabled: true });
    expect(onChange).toHaveBeenNthCalledWith(2, { ...aiSettingsFixture, max_discount_percent: 20 });
    expect(onSave).toHaveBeenCalledTimes(1);
  });

	test('AI 设置和账号编辑弹窗均支持 Escape 关闭', /* 当前回调验证两个独立弹窗不会遗漏键盘取消。 */ () => {
		// aiClose 和 aiSave 分别记录 AI 弹窗关闭与保存动作。
		const aiClose = vi.fn();
		const aiSave = vi.fn();
		// aiView 保存 AI 弹窗实例，卸载后再检查账号编辑弹窗。
		const aiView = render(<AccountAISettingsModal account={accountFixture} settings={aiSettingsFixture} saving={false} onChange={vi.fn()} onClose={aiClose} onSave={aiSave} />);
		fireEvent.keyDown(document, { key: 'Escape' });
		expect(aiClose).toHaveBeenCalledTimes(1);
		expect(aiSave).not.toHaveBeenCalled();
		aiView.unmount();

		// editClose 和 editSave 分别记录账号编辑弹窗关闭与保存动作。
		const editClose = vi.fn();
		// editSave 记录账号编辑弹窗的保存动作。
		const editSave = vi.fn();
		render(<AccountEditModal
			account={accountFixture}
			editForm={manualEditFormFixture}
			setEditForm={vi.fn()}
			saving={false}
			onClose={editClose}
			onSave={editSave}
			onRestartPause={noopAccountAction}
			longLogin={{ loading: false, saving: false, canOpen: true, enabled: false, error: '' }}
			onToggleLongLogin={noopAccountAction}
			passwordLoginView={{ sessionId: '', status: 'idle', message: '', qrCodeUrl: '' }}
			onPasswordLogin={noopAccountAction}
			onCancelPasswordLogin={noopAccountAction}
			notifChannels={[]}
			selectedChannelIds={[]}
			bindingsLoaded={true}
			bindingsLoading={false}
			bindingsLoadError=""
			onRetryBindings={noopAccountAction}
			onToggleChannel={/* channelToggle 不修改测试状态。 */ () => undefined}
			onSettingsDirty={noopAccountAction}
		/>);
		fireEvent.keyDown(document, { key: 'Escape' });
		expect(editClose).toHaveBeenCalledTimes(1);
		expect(editSave).not.toHaveBeenCalled();
	});

  test('删除确认框展示错误并转发确认动作', /* 当前回调验证删除确认框的错误和提交分支。 */ () => {
    // onConfirm 是删除确认测试替身。
    const onConfirm = vi.fn();
    render(<AccountDeleteDialog account={accountFixture} deleting={false} error="删除失败" onClose={noopAccountAction} onConfirm={onConfirm} />);
    expect(screen.getByRole('alert').textContent).toContain('删除失败');
    fireEvent.click(screen.getByText('确认删除'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  test('二维码弹窗展示风控验证面板且不生成外部链接', /* 当前回调验证二维码风控状态的安全展示边界。 */ () => {
    // onCaptchaModeChange 记录二维码弹窗中的显式模式选择。
    const onCaptchaModeChange = vi.fn();
	// onClose 记录 Escape 是否复用二维码弹窗既有关闭和轮询收束入口。
	const onClose = vi.fn();
	render(<AccountQRCodeModal target={accountFixture} status="verification" codeUrl="" errorMessage="" faceQrUrl="face-qr" verificationScreenshot="screen" captchaMode="automatic" onCaptchaModeChange={onCaptchaModeChange} onClose={onClose} />);
    expect(screen.getByText('需要完成安全风控验证')).toBeTruthy();
    fireEvent.click(screen.getByRole('radio', { name: '弹窗人工处理' }));
    expect(onCaptchaModeChange).toHaveBeenCalledWith('manual');
    expect(document.querySelectorAll('a')).toHaveLength(0);
	fireEvent.keyDown(document, { key: 'Escape' });
	expect(onClose).toHaveBeenCalledTimes(1);
  });

  test('账号编辑弹窗明确说明显示浏览器会禁用自动滑块', /* 当前回调验证人工接管开关不会继续使用旧的调试文案。 */ () => {
    // setEditForm 是表单状态更新替身，本断言只检查人工模式说明而不提交设置。
    const setEditForm = vi.fn();
    render(<AccountEditModal
      account={accountFixture}
      editForm={manualEditFormFixture}
      setEditForm={setEditForm}
      saving={false}
      onClose={noopAccountAction}
      onSave={noopAccountAction}
      onRestartPause={noopAccountAction}
      longLogin={{ loading: false, saving: false, canOpen: true, enabled: false, error: '' }}
      onToggleLongLogin={noopAccountAction}
      passwordLoginView={{ sessionId: '', status: 'idle', message: '', qrCodeUrl: '' }}
      onPasswordLogin={noopAccountAction}
      onCancelPasswordLogin={noopAccountAction}
      notifChannels={[]}
      selectedChannelIds={[]}
      bindingsLoaded={true}
      bindingsLoading={false}
      bindingsLoadError=""
      onRetryBindings={noopAccountAction}
      onToggleChannel={/* channelToggle 不修改测试状态。 */ () => undefined}
      onSettingsDirty={noopAccountAction}
    />);
    expect(screen.getByText('Token 滑块处理方式')).toBeTruthy();
    expect(screen.getByRole('radio', { name: /弹窗人工处理/ }).getAttribute('aria-checked')).toBe('true');
  });

	test('自动确认卡片打开编辑弹窗后滚动到对应设置', /* 当前回调验证定位只作用于弹窗内目标区块。 */ async () => {
		// scrolledElement 保存 scrollIntoView 实际接收的自动确认区块。
		let scrolledElement: HTMLElement | null = null;
		// scrollIntoViewMock 记录当前滚动目标，不改变 jsdom 页面位置。
		const scrollIntoViewMock = vi.fn(/* recordScrollTarget 保存当前账号编辑滚动目标。 */ function recordScrollTarget(this: HTMLElement) { scrolledElement = this; });
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: scrollIntoViewMock });
		vi.spyOn(window, 'requestAnimationFrame').mockImplementation(/* frameCallback 立即执行弹窗布局后的定位回调。 */ callback => { callback(0); return 1; });
		render(<AccountEditModal
			account={accountFixture}
			editForm={manualEditFormFixture}
			setEditForm={vi.fn()}
			saving={false}
			initialFocus="auto_confirm"
			onClose={noopAccountAction}
			onSave={noopAccountAction}
			onRestartPause={noopAccountAction}
			longLogin={{ loading: false, saving: false, canOpen: true, enabled: false, error: '' }}
			onToggleLongLogin={noopAccountAction}
			passwordLoginView={{ sessionId: '', status: 'idle', message: '', qrCodeUrl: '' }}
			onPasswordLogin={noopAccountAction}
			onCancelPasswordLogin={noopAccountAction}
			notifChannels={[]}
			selectedChannelIds={[]}
			bindingsLoaded={true}
			bindingsLoading={false}
			bindingsLoadError=""
			onRetryBindings={noopAccountAction}
			onToggleChannel={/* channelToggle 不修改测试状态。 */ () => undefined}
			onSettingsDirty={noopAccountAction}
		/>);
		await waitFor(/* scrollAssertion 等待 Portal 布局后的目标定位。 */ () => expect(scrollIntoViewMock).toHaveBeenCalledTimes(1));
		expect(scrolledElement).toBe(screen.getByTestId('account-edit-section-auto-confirm'));
	});
});
