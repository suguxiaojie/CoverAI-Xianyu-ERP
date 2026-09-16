// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor,within } from '@testing-library/react';
import React from 'react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import type { ChatMessage,ChatSession } from '../api';
import { useChat,type UseChatResult } from '../hooks';
import Chat from './Chat';

vi.mock('../hooks', /* chatHookMockFactory 让页面交互测试不访问真实账号、会话或发送接口。 */ () => ({
  useChat: vi.fn(),
}));

vi.mock('yet-another-react-lightbox', /* lightboxMockFactory 隔离本次聊天布局测试不关心的第三方灯箱渲染。 */ () => ({
  default: /* LightboxMock 在测试环境中不生成额外对话框节点。 */ () => null,
}));

vi.mock('../components/ConversationOrderPreviewPanel', /* conversationOrderPanelMockFactory 隔离聊天页面测试不访问真实历史订单接口。 */ () => ({
  ConversationOrderPreviewPanel: /* ConversationOrderPreviewPanelMock 保留第三栏语义标识，具体筛选与分页由组件和 Hook 测试覆盖。 */ () => <aside aria-label="历史订单">历史订单</aside>,
}));

// useChatMock 是聊天页面状态 Hook 的可控替身。
const useChatMock = vi.mocked(useChat);
// setActiveAccountIDMock 记录账号页签鼠标和键盘选择结果。
const setActiveAccountIDMock = vi.fn();
// setActiveChatIDMock 记录窄屏会话列表打开的聊天标识。
const setActiveChatIDMock = vi.fn();
// clearNavigationPriorityMock 记录用户主动切换会话后解除订单深链临时排序。
const clearNavigationPriorityMock = vi.fn();
// setUnreadOnlyMock 记录全部与未读筛选按钮的明确目标状态。
const setUnreadOnlyMock = vi.fn();
// recallMessageMock 记录页面菜单提交的本地消息键，不执行真实平台撤回。
const recallMessageMock = vi.fn(/* recallMessageActionMock 模拟撤回应用调用。 */ async () => undefined);
// setDraftMock 记录撤回文本重新编辑时恢复的草稿。
const setDraftMock = vi.fn();
// setSessionPinnedMock 记录图钉提交的精确会话和目标持久化状态。
const setSessionPinnedMock = vi.fn(/* setSessionPinnedActionMock 不访问测试外的真实置顶接口。 */ async () => undefined);
// handleSendMock 记录页面提交的可选引用消息键，不访问真实发送接口。
const handleSendMock = vi.fn(/* handleSendActionMock 默认模拟最新请求明确成功。 */ async (_replyToMessageKey?: string) => true);

// sessionFixture 是聊天页会话列表、商品上下文和窄屏详情导航共享的会话。
const sessionFixture: ChatSession = {
  account_id: 'account-1', chat_id: 'chat-1', buyer_id: 'buyer-1', buyer_name: '买家', item_title: '测试商品',
  last_message: '第二条', last_message_at: 160, unread_count: 2,
};

// messageFixtures 覆盖连续消息分组以及必须独立展示的平台系统事件。
const messageFixtures: ChatMessage[] = [
  { id: 1, account_id: 'account-1', chat_id: 'chat-1', message_key: 'message-1', direction: 'incoming', sender_id: 'buyer-1', sender_name: '买家', message_type: 'text', content: '第一条', status: 'received', sent_at: 100 },
  { id: 2, account_id: 'account-1', chat_id: 'chat-1', message_key: 'message-2', direction: 'incoming', sender_id: 'buyer-1', sender_name: '买家', message_type: 'text', content: '第二条', status: 'received', sent_at: 160 },
  { id: 3, account_id: 'account-1', chat_id: 'chat-1', message_key: 'system-1', direction: 'incoming', sender_id: 'system', sender_name: '系统', message_type: 'system', content: '买家已付款', status: 'received', sent_at: 170 },
  { id: 4, account_id: 'account-1', chat_id: 'chat-1', message_key: 'trade-card-1', direction: 'incoming', sender_id: 'system', sender_name: '系统', message_type: 'system', content: '我已拍下，待付款', status: 'received', sent_at: 180,
    system_card: { kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款', description: '请双方沟通及时确认价格', order_id: '5127638256187075541', item_id: 'item-1', action: 'adjust_price' } },
  { id: 5, account_id: 'account-1', chat_id: 'chat-1', message_key: 'flower-sent-1', direction: 'incoming', sender_id: 'buyer-1', sender_name: '买家', message_type: 'system', content: '你人真不错，送你闲鱼小红花', status: 'received', sent_at: 190,
    system_card: { kind: 'trade', event: 'red_flower_sent', title: '你人真不错，送你闲鱼小红花', order_id: '5127372398162002704', action: 'receive_red_flower' } },
  { id: 6, account_id: 'account-1', chat_id: 'chat-1', message_key: 'flower-received-1', direction: 'outgoing', sender_id: 'account-1', sender_name: '账号一', message_type: 'system', content: '收到小红花，心里乐开花！', status: 'sent', sent_at: 200,
    system_card: { kind: 'trade', event: 'red_flower_received', title: '收到小红花，心里乐开花！' } },
];

/** createChatResult 构造不触发网络和真实发送的完整聊天页面状态。 */
const createChatResult = (): UseChatResult => ({
  accounts: [
    { id: 'account-1', nickname: '账号一', enabled: true, runtime_state: 'online' },
    { id: 'account-2', nickname: '账号二', enabled: true, runtime_state: 'online' },
  ] as never,
  activeAccountID: 'account-1',
  activeSessions: [sessionFixture],
  activeChatID: 'chat-1',
	navigationPriorityChatID: '',
	closeEligibleOrderID: '5127638256187075541',
	closeEligibleOrderStage: 'pending_payment',
  activeAccount: { id: 'account-1', nickname: '账号一', enabled: true, runtime_state: 'online' } as never,
  selectedSession: sessionFixture,
  filteredSessions: [sessionFixture],
	creditProfile: { user_id: 'buyer-1', buyer: { role: 'buyer', level: 5, code: 'cs_buyer_level', text: '买家信用极好' }, seller: { role: 'seller', level: 4, code: 'cs_seller_level', text: '卖家信用优秀' }, fetched_at: '2026-08-27T02:30:00Z', expires_at: '2026-08-28T02:30:00Z', stale: false },
	creditLoading: false, creditError: '',
  messages: messageFixtures,
  search: '', unreadOnly: false, draft: '', loading: false, messagesLoading: false, olderLoading: false,
  hasOlder: false, contactsLoading: false, hasMoreContacts: false, emojiOpen: false, sending: false,
  error: '', liveState: 'online', pendingImage: null,
	locationCardDefaults: { title: '默认门店', description: '默认入口', latitude: '31.230400', longitude: '121.473700' }, locationDefaultsLoading: false,
  scrollRef: React.createRef<HTMLDivElement>(), imageInputRef: React.createRef<HTMLInputElement>(),
	setActiveAccountID: setActiveAccountIDMock, setActiveChatID: setActiveChatIDMock, clearNavigationPriority: clearNavigationPriorityMock, setSearch: vi.fn(),
  setUnreadOnly: setUnreadOnlyMock, setDraft: setDraftMock, setEmojiOpen: vi.fn(),
  reloadSessions: vi.fn(/* reloadSessionsMock 返回当前已加载会话，不读取后端。 */ async () => [sessionFixture]),
  loadMoreContacts: vi.fn(/* loadMoreContactsMock 不执行真实联系人分页。 */ async () => undefined),
  pinningSessionIDs: new Set(), setSessionPinned: setSessionPinnedMock,
  loadOlderMessages: vi.fn(/* loadOlderMessagesMock 不执行真实消息分页。 */ async () => undefined),
  handleMessageScroll: vi.fn(), handleSend: handleSendMock,
  handleImage: vi.fn(/* handleImageMock 禁止测试上传真实图片。 */ async () => undefined),
  handlePastedImages: vi.fn(/* handlePastedImagesMock 禁止测试读取真实剪贴板图片。 */ async () => undefined),
  confirmSendImage: vi.fn(/* confirmSendImageMock 禁止测试发送真实图片。 */ async () => undefined),
	handleSendLocationCard: vi.fn(/* handleSendLocationCardMock 禁止页面测试发送真实位置卡片。 */ async () => true),
	loadLocationCardDefaults: vi.fn(/* loadLocationCardDefaultsMock 返回已加载系统默认值。 */ async () => true),
	closeImagePreview: vi.fn(), retrySend: vi.fn(/* retrySendMock 禁止测试重试真实发送。 */ async () => false),
  recallMessage: recallMessageMock,
  retryAvailable: false, unreadForAccount: /* unreadForAccountMock 返回固定未读数供页签渲染。 */ accountID => accountID === 'account-1' ? 2 : 0,
  emojiURL: /* emojiURLMock 返回不访问网络的空媒体地址。 */ () => '', xianyuEmojis: [] as unknown as UseChatResult['xianyuEmojis'],
  renderXianyuText: /* renderXianyuTextMock 用 React 文本节点回显已脱敏测试内容。 */ text => [<React.Fragment key={text}>{text}</React.Fragment>],
  formatClock: /* formatClockMock 提供稳定列表时间。 */ () => '10:00',
  messageTime: /* messageTimeMock 暴露分组最终使用的时间值。 */ value => `时间-${value}`,
});

describe('Chat 页面交互', /* 当前测试组验证窄屏导航、账号键盘切换和消息分组展示。 */ () => {
  beforeEach(/* 当前回调重置页面 Hook 和交互替身，避免用例互相污染。 */ () => {
    vi.clearAllMocks();
    useChatMock.mockReturnValue(createChatResult());
  });

  afterEach(/* 当前回调卸载聊天页面 DOM，避免前一用例的重复页签和消息日志污染后续查询。 */ () => {
    cleanup();
  });

  test('窄屏从会话列表进入消息详情并可返回', /* 当前回调验证响应式单栏不会把会话栏和消息栏同时挤窄。 */ () => {
    render(<Chat />);
    // sessionPane 是窄屏默认显示的账号会话列表。
    const sessionPane = screen.getByLabelText('会话列表');
    // conversationPane 是选择会话后显示的消息详情面板。
    const conversationPane = screen.getByLabelText('聊天内容');
    expect(sessionPane.className).toContain('flex');
    expect(conversationPane.className).toContain('hidden');
    fireEvent.click(within(sessionPane).getByRole('button', { name: /买家.*第二条/ }));
    expect(setActiveChatIDMock).toHaveBeenCalledWith('chat-1');
    expect(sessionPane.className).toContain('hidden');
    expect(conversationPane.className).toContain('flex');
    fireEvent.click(within(conversationPane).getByRole('button', { name: '返回会话列表' }));
    expect(sessionPane.className).toContain('flex');
  });

	test('订单深链目标渲染后滚动左栏顶部且手动切换会解除临时提升', /* orderNavigationListCase 验证定位只作用于本次订单跳转。 */ () => {
		// scrollToMock 记录联系人滚动区收到的顶部定位请求。
		const scrollToMock = vi.fn();
		Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: scrollToMock });
		// otherSession 是用户随后主动选择的普通会话。
		const otherSession: ChatSession = { ...sessionFixture, chat_id: 'chat-2', buyer_id: 'buyer-2', buyer_name: '其他买家', last_message: '其他消息' };
		useChatMock.mockReturnValue({ ...createChatResult(), navigationPriorityChatID: 'chat-1', filteredSessions: [sessionFixture, otherSession] });
		render(<Chat />);
		expect(scrollToMock).toHaveBeenCalledWith({ top: 0, behavior: 'auto' });
		fireEvent.click(screen.getByRole('button', { name: /其他买家.*其他消息/ }));
		expect(clearNavigationPriorityMock).toHaveBeenCalledTimes(1);
		expect(setActiveChatIDMock).toHaveBeenCalledWith('chat-2');
		delete (HTMLElement.prototype as { /** scrollTo 是当前测试临时注入的方法。 */ scrollTo?: unknown }).scrollTo;
	});

	test('不同会话的文本和引用状态互相隔离且返回时恢复', /* 当前回调验证页面内撰写状态按账号和 chatID 分区保存。 */ () => {
		// otherSession 是与 chat-1 分别保存草稿的第二个买家会话。
		const otherSession: ChatSession = { ...sessionFixture, chat_id: 'chat-2', buyer_id: 'buyer-2', buyer_name: '其他买家', last_message: '新会话', last_message_at: 300, unread_count: 0 };
		// replyMessage 是 chat-1 中已关联 PNM 且需要随会话恢复的引用目标。
		const replyMessage = { ...messageFixtures[0], message_key: 'draft-reply-target.PNM', platform_message_id: 'draft-reply-target.PNM', content: '请帮我查一下' } as ChatMessage;
		// firstResult 表示 chat-1 正在编辑尚未发送的第一份草稿。
		const firstResult = { ...createChatResult(), activeSessions: [sessionFixture, otherSession], filteredSessions: [sessionFixture, otherSession], messages: [replyMessage], draft: 'A 会话草稿' };
		useChatMock.mockReturnValue(firstResult);
		// rerender 用于模拟 Hook 完成会话切换后向页面提供新上下文。
		const { rerender } = render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		fireEvent.click(screen.getByRole('menuitem', { name: '回复' }));
		expect(within(screen.getByLabelText('引用回复预览')).getByText('请帮我查一下')).toBeTruthy();
		setDraftMock.mockClear();
		// secondResult 保留切换瞬间的旧撰写状态，layout effect 应在绘制前把它替换为 chat-2 的空状态。
		const secondResult = { ...firstResult, activeChatID: 'chat-2', selectedSession: otherSession };
		useChatMock.mockReturnValue(secondResult);
		rerender(<Chat />);
		expect(setDraftMock).toHaveBeenLastCalledWith('');
		expect(screen.queryByLabelText('引用回复预览')).toBeNull();
		// secondDraftResult 表示用户已在 chat-2 输入自己的独立草稿。
		const secondDraftResult = { ...secondResult, draft: 'B 会话草稿' };
		useChatMock.mockReturnValue(secondDraftResult);
		rerender(<Chat />);
		setDraftMock.mockClear();
		// returnedResult 模拟返回 chat-1 前 Hook 仍持有 chat-2 文本的瞬间状态。
		const returnedResult = { ...secondDraftResult, activeChatID: 'chat-1', selectedSession: sessionFixture };
		useChatMock.mockReturnValue(returnedResult);
		rerender(<Chat />);
		expect(setDraftMock).toHaveBeenLastCalledWith('A 会话草稿');
		// restoredReplyPreview 是返回 chat-1 后恢复的原引用条，不应变成 chat-2 状态。
		const restoredReplyPreview = screen.getByLabelText('引用回复预览');
		expect(within(restoredReplyPreview).getByText('回复 买家')).toBeTruthy();
		expect(within(restoredReplyPreview).getByText('请帮我查一下')).toBeTruthy();
	});

  test('账号页签支持方向键并让窄屏回到会话列表', /* 当前回调验证 WAI-ARIA 页签键盘导航与响应式状态同步。 */ () => {
    render(<Chat />);
    // firstAccountTab 是当前选中的第一个账号页签。
    const firstAccountTab = screen.getByRole('tab', { name: /账号一/ });
    // secondAccountTab 是方向键后应获得焦点的下一个账号页签。
    const secondAccountTab = screen.getByRole('tab', { name: /账号二/ });
    fireEvent.keyDown(firstAccountTab, { key: 'ArrowRight' });
    expect(setActiveAccountIDMock).toHaveBeenCalledWith('account-2');
    expect(document.activeElement).toBe(secondAccountTab);
  });

  test('会话图钉分别提交置顶和取消且不打开会话', /* 当前回调验证持久化图钉与会话主按钮的交互边界。 */ () => {
    // newestSession 是当前未置顶且可提交 true 的普通会话。
    const newestSession: ChatSession = { ...sessionFixture, chat_id: 'chat-newest', buyer_id: 'buyer-newest', buyer_name: '最新会话', last_message: '最新消息', last_message_at: 300, unread_count: 0 };
    // olderSession 是服务端已持久化且可提交 false 取消的置顶会话。
    const olderSession: ChatSession = { ...sessionFixture, chat_id: 'chat-older', buyer_id: 'buyer-older', buyer_name: '较早会话', last_message: '较早消息', last_message_at: 100, unread_count: 0, is_pinned: true };
    useChatMock.mockReturnValue({
      ...createChatResult(), activeSessions: [olderSession, newestSession], filteredSessions: [olderSession, newestSession],
      selectedSession: newestSession, activeChatID: newestSession.chat_id,
    });
    render(<Chat />);
    // sessionPane 是当前验证持久化图钉和会话主按钮分离的列表容器。
    const sessionPane = screen.getByLabelText('会话列表');
    expect(within(sessionPane).getAllByTestId('chat-session-row').map(/* row 是当前读取服务端排序结果的会话行。 */ row => row.getAttribute('data-chat-id'))).toEqual(['chat-older', 'chat-newest']);
    fireEvent.click(within(sessionPane).getByRole('button', { name: '置顶会话 最新会话' }));
    expect(setSessionPinnedMock).toHaveBeenCalledWith('chat-newest', true);
    fireEvent.click(within(sessionPane).getByRole('button', { name: '取消置顶 较早会话' }));
    expect(setSessionPinnedMock).toHaveBeenCalledWith('chat-older', false);
    expect(setActiveChatIDMock).not.toHaveBeenCalled();
    expect(within(sessionPane).getByText('置顶')).toBeTruthy();
  });

  test('连续消息只显示一次发送者并保留商品与系统事件上下文', /* 当前回调验证长会话压缩不会丢失业务和平台状态。 */ () => {
    render(<Chat />);
    // messageLog 是具有实时消息语义的可访问聊天记录区域。
    const messageLog = screen.getByRole('log', { name: '聊天消息' });
    expect(within(messageLog).getAllByText('买家')).toHaveLength(1);
    expect(within(messageLog).getByText('第一条')).toBeTruthy();
    expect(within(messageLog).getByText('第二条')).toBeTruthy();
    expect(within(messageLog).queryByText('时间-100')).toBeNull();
    expect(within(messageLog).getByText('时间-160')).toBeTruthy();
    expect(within(messageLog).getByText(/系统事件/)).toBeTruthy();
    expect(within(messageLog).getByRole('article', { name: '交易状态：我已拍下，待付款' })).toBeTruthy();
    // adjustButton 是具备消息账号与订单关联的真实改价入口。
    const adjustButton = within(messageLog).getByRole('button', { name: '修改价格' });
    expect((adjustButton as HTMLButtonElement).disabled).toBe(false);
    expect(within(messageLog).getByText('仅待付款订单可改价，提交前会再次确认')).toBeTruthy();
    expect(screen.getByRole('button', { name: '取消订单' })).toBeTruthy();
	// historyButton 只服务中窄屏抽屉，超宽屏已有常驻第三栏时必须隐藏。
	const historyButton = screen.getByRole('button', { name: '历史订单' });
	expect(historyButton.className).toContain('2xl:hidden');
    // receivedFlowerButton 是由后续收花结果禁用的原送花卡片入口。
    const receivedFlowerButton = within(messageLog).getByRole('button', { name: '已收花' });
    expect((receivedFlowerButton as HTMLButtonElement).disabled).toBe(true);
    expect(within(messageLog).getByText('已检测到闲鱼收花结果')).toBeTruthy();
    expect(screen.getByText(/关联商品 · 测试商品/)).toBeTruthy();
  });

	test('订单表未确认待付款时隐藏聊天卡片推导出的取消入口', /* 当前回调验证前端不再只凭历史系统卡片展示危险操作。 */ () => {
		useChatMock.mockReturnValue({ ...createChatResult(), closeEligibleOrderID: '' });
		render(<Chat />);
		expect(screen.queryByRole('button', { name: '取消订单' })).toBeNull();
	});

	test('已发送引用消息在气泡内展示原发送者和摘要', /* 当前回调验证 API 快照和未加载历史占位都不会被渲染成普通气泡。 */ () => {
		// replyWithPreview 是目标不在当前分页但 API 已提供最小快照的己方引用消息。
		const replyWithPreview = { ...messageFixtures[0], id: 21, message_key: 'reply-with-preview', platform_message_id: 'reply-result-1.PNM', reply_to_platform_message_id: 'target-old.PNM', direction: 'outgoing', sender_id: 'account-1', sender_name: '我', content: '新消息', status: 'sent', sent_at: 300,
			reply_preview: { platform_message_id: 'target-old.PNM', direction: 'incoming', sender_id: 'buyer-1', sender_name: '买家', message_type: 'text', content: '原消息', status: 'received' } } as ChatMessage;
		// replyWithoutPreview 是目标既不在当前分页也没有服务端快照的兼容引用消息。
		const replyWithoutPreview = { ...replyWithPreview, id: 22, message_key: 'reply-without-preview', platform_message_id: 'reply-result-2.PNM', reply_to_platform_message_id: 'missing-target.PNM', content: '另一条消息', sent_at: 301, reply_preview: undefined } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [replyWithPreview, replyWithoutPreview] });
		render(<Chat />);
		// messageLog 是当前检查引用区和正文层级的聊天滚动容器。
		const messageLog = screen.getByRole('log', { name: '聊天消息' });
		expect(within(messageLog).getByText('买家')).toBeTruthy();
		expect(within(messageLog).getByText('原消息')).toBeTruthy();
		expect(within(messageLog).getByText('新消息')).toBeTruthy();
		expect(within(messageLog).getByText('历史消息')).toBeTruthy();
		expect(within(messageLog).getByText('引用了一条历史消息')).toBeTruthy();
	});

  test('实时付款卡片到达后原改价按钮立即置灰', /* 当前回调验证 Chat 页面把同订单终态传给待付款卡片。 */ () => {
    // paidMessage 是目标订单在待付款卡片之后到达的结构化付款终态。
    const paidMessage: ChatMessage = { ...messageFixtures[3], id: 7, message_key: 'trade-paid-1', content: '我已付款，等待你发货', sent_at: 210,
      system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: '5127638256187075541' } };
    useChatMock.mockReturnValue({ ...createChatResult(), closeEligibleOrderStage: 'pending_ship', messages: [...messageFixtures, paidMessage] });
    render(<Chat />);
    // adjustButton 是仍保留在原待付款卡片中的灰色禁用入口。
    const adjustButton = screen.getByRole('button', { name: '修改价格' });
    expect((adjustButton as HTMLButtonElement).disabled).toBe(true);
    expect(adjustButton.className).toContain('bg-slate-200');
    expect(screen.getByText('买家已付款，不能再修改价格')).toBeTruthy();
		expect(screen.getByRole('button', { name: '取消订单' })).toBeTruthy();
  });

	 test('普通消息头像与昵称气泡顶部对齐', /* 当前回调防止头像再次下沉到时间和状态行造成消息关系错位。 */ () => {
		// outgoingMessage 是用于检查右侧己方头像对齐的独立出站消息。
		const outgoingMessage = { ...messageFixtures[0], id: 4, message_key: 'outgoing-layout', direction: 'outgoing', sender_id: 'account-1', sender_name: '账号一', content: '己方消息', status: 'sent', read_status: 2, sent_at: 300 } as ChatMessage;
		useChatMock.mockReturnValue({
			...createChatResult(),
			selectedSession: { ...sessionFixture, buyer_avatar_url: '/buyer-avatar.png' },
			activeAccount: { id: 'account-1', nickname: '账号一', avatar_url: '/self-avatar.png', enabled: true, runtime_state: 'online' } as never,
			messages: [messageFixtures[0], outgoingMessage],
	 });

		render(<Chat />);
		// buyerAvatar 是左侧买家头像图片，其消息行必须使用顶部对齐。
		const buyerAvatar = screen.getByAltText('买家');
		// selfAvatar 是右侧己方头像图片，其消息行必须使用同一顶部对齐规则。
		const selfAvatar = screen.getByAltText('我');
		expect(buyerAvatar.parentElement?.parentElement?.className).toContain('items-start');
		expect(selfAvatar.parentElement?.parentElement?.className).toContain('items-start');
	});

	test('只有非零已读时间才能显示对方已读双勾', /* 当前回调拒绝把历史接口模糊状态误画为已读。 */ () => {
		// uncertainMessage 是只有 read_status、没有明确 read_at 的出站消息。
		const uncertainMessage = { ...messageFixtures[0], id: 7, message_key: 'uncertain-read', direction: 'outgoing', status: 'sent', read_status: 2, read_at: 0, sent_at: 300 } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [uncertainMessage] });
		// view 保留同一页面实例，用于把模糊未读状态更新为具有权威时间的已读状态。
		const view = render(<Chat />);
		expect(screen.getByLabelText('已发送未读')).toBeTruthy();
		// confirmedMessage 是同时具备已读状态和明确时间的出站消息。
		const confirmedMessage = { ...uncertainMessage, read_at: 301 };
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [confirmedMessage] });
		view.rerender(<Chat />);
		expect(screen.getByLabelText('对方已读')).toBeTruthy();
	});

	test('买家和卖家气泡使用可区分的文字选区配色', /* 当前回调防止卖家蓝色气泡与浏览器默认蓝色选区混在一起。 */ () => {
		// outgoingMessage 是用于检查黄色选区的卖家文本气泡。
		const outgoingMessage = { ...messageFixtures[0], id: 31, message_key: 'outgoing-selection', direction: 'outgoing', sender_id: 'account-1', sender_name: '账号一', content: '卖家消息', status: 'sent', sent_at: 300 } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [messageFixtures[0], outgoingMessage] });
		render(<Chat />);
		// buyerBubble 是使用浅蓝选区的买家白色气泡。
		const buyerBubble = screen.getByText('第一条').closest('.rounded-2xl');
		// sellerBubble 是使用琥珀黄选区的卖家蓝色气泡。
		const sellerBubble = screen.getByText('卖家消息').closest('.rounded-2xl');
		expect(buyerBubble?.className).toContain('selection:bg-sky-200');
		expect(buyerBubble?.className).toContain('selection:text-slate-950');
		expect(sellerBubble?.className).toContain('selection:bg-amber-200');
		expect(sellerBubble?.className).toContain('selection:text-slate-950');
	});

		test('己方消息菜单提交撤回，撤回文本可以重新编辑', /* 当前回调验证操作菜单和撤回占位状态。 */ async () => {
		// recallableMessage 是处于两分钟窗口且已绑定 PNM ID 的己方文本。
		const recallableMessage = { ...messageFixtures[0], direction: 'outgoing', sender_id: 'account-1', message_key: 'local-recall', platform_message_id: 'platform-recall.PNM', status: 'sent', sent_at: Date.now() } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [recallableMessage] });
		render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		fireEvent.click(screen.getByRole('menuitem', { name: '撤回' }));
		expect(recallMessageMock).toHaveBeenCalledWith('local-recall');
		cleanup();
		// recalledMessage 是服务端确认撤回后仍保留原文的消息。
		const recalledMessage = { ...recallableMessage, status: 'recalled', recalled_at: Date.now() } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [recalledMessage] });
		render(<Chat />);
		expect(screen.getByText('你撤回了一条消息')).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: '重新编辑' }));
		expect(setDraftMock).toHaveBeenCalledWith(recalledMessage.content);
	});

	test('消息菜单点击内部保持打开且点击空白或 Escape 收回', /* 当前回调验证文档级点击外部与键盘关闭边界。 */ () => {
		// menuMessage 是具备复制菜单的单条己方文本消息。
		const menuMessage = { ...messageFixtures[0], direction: 'outgoing', sender_id: 'account-1', message_key: 'menu-message', status: 'sent', sent_at: Date.now() } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [menuMessage] });
		render(<Chat />);
		// menuButton 是当前消息旁用于打开复制／撤回菜单的独立按钮。
		const menuButton = screen.getByRole('button', { name: '消息操作' });
		fireEvent.click(menuButton);
		// menu 是当前打开的消息操作菜单。
		const menu = screen.getByRole('menu', { name: '消息操作菜单' });
		fireEvent.pointerDown(menu);
		expect(screen.getByRole('menu', { name: '消息操作菜单' })).toBeTruthy();
		fireEvent.pointerDown(screen.getByRole('log', { name: '聊天消息' }));
		expect(screen.queryByRole('menu', { name: '消息操作菜单' })).toBeNull();
		fireEvent.click(menuButton);
		expect(screen.getByRole('menu', { name: '消息操作菜单' })).toBeTruthy();
		fireEvent.keyDown(document, { key: 'Escape' });
		expect(screen.queryByRole('menu', { name: '消息操作菜单' })).toBeNull();
	});

	test('有 PNM 标识的消息使用专用引用键发送且成功后清除引用条', /* 当前回调验证页面只传本地键并以 Hook 明确成功为收口。 */ async () => {
		// replyMessage 是已关联平台 PNM 标识且可用于引用预览的买家文本。
		const replyMessage = { ...messageFixtures[0], message_key: 'reply-target.PNM', platform_message_id: 'reply-target.PNM', content: '请问什么时候发货？' } as ChatMessage;
		// chatResult 给输入区一段已有草稿，用于确认引用预览单独禁用发送。
		const chatResult = { ...createChatResult(), messages: [replyMessage], draft: '准备回复' };
		useChatMock.mockReturnValue(chatResult);
		render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		fireEvent.click(screen.getByRole('menuitem', { name: '回复' }));
		// replyPreview 是输入区上方展示发送者和原消息摘要的引用条。
		const replyPreview = screen.getByLabelText('引用回复预览');
		expect(within(replyPreview).getByText('回复 买家')).toBeTruthy();
		expect(within(replyPreview).getByText('请问什么时候发货？')).toBeTruthy();
		expect(within(replyPreview).getByText('引用回复')).toBeTruthy();
		// composer 是选择回复后应该获得语义提示的撰写框。
		const composer = screen.getByPlaceholderText('回复 买家');
		expect(composer).toBeTruthy();
		// sendButton 在已有草稿时允许调用专用引用发送流程。
		const sendButton = screen.getByRole('button', { name: '发送消息' }) as HTMLButtonElement;
		expect(sendButton.disabled).toBe(false);
		fireEvent.click(sendButton);
		await waitFor(/* replySendCompleted 等待异步 Hook 明确成功后收起引用条。 */ () => expect(screen.queryByLabelText('引用回复预览')).toBeNull());
		expect(handleSendMock).toHaveBeenCalledWith('reply-target.PNM');
	});

	test('引用发送失败时保留原消息摘要和输入草稿', /* 当前回调验证平台或旧后端拒绝不会丢失用户撰写状态。 */ async () => {
		// replyMessage 是已关联 PNM 且可进入原生引用流程的买家消息。
		const replyMessage = { ...messageFixtures[0], message_key: 'failed-reply-target.PNM', platform_message_id: 'failed-reply-target.PNM', content: '请继续处理' } as ChatMessage;
		handleSendMock.mockResolvedValueOnce(false);
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [replyMessage], draft: '失败后要保留' });
		render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		fireEvent.click(screen.getByRole('menuitem', { name: '回复' }));
		fireEvent.click(screen.getByRole('button', { name: '发送消息' }));
		await waitFor(/* failedReplyCalled 等待页面把精确本地键提交给 Hook。 */ () => expect(handleSendMock).toHaveBeenCalledWith('failed-reply-target.PNM'));
		// replyPreview 在 Hook 返回 false 后必须仍然显示原引用摘要。
		const replyPreview = screen.getByLabelText('引用回复预览');
		expect(within(replyPreview).getByText('请继续处理')).toBeTruthy();
		expect((screen.getByPlaceholderText('回复 买家') as HTMLTextAreaElement).value).toBe('失败后要保留');
	});

	test('引用失败后切换会话不会把旧引用条带入新买家', /* 当前回调验证引用目标在绘制和再次发送前均受账号会话绑定保护。 */ async () => {
		// replyMessage 是 chat-1 中会失败并必须保持在原会话范围内的引用目标。
		const replyMessage = { ...messageFixtures[0], message_key: 'failed-switch-target.PNM', platform_message_id: 'failed-switch-target.PNM', content: '原会话消息' } as ChatMessage;
		// otherSession 是切换后绝不能显示 chat-1 引用条的第二个买家会话。
		const otherSession: ChatSession = { ...sessionFixture, chat_id: 'chat-2', buyer_id: 'buyer-2', buyer_name: '其他买家', last_message: '其他消息', last_message_at: 300, unread_count: 0 };
		handleSendMock.mockResolvedValueOnce(false);
		// firstResult 表示 chat-1 已输入待发送草稿并允许选择引用。
		const firstResult = { ...createChatResult(), activeSessions: [sessionFixture, otherSession], filteredSessions: [sessionFixture, otherSession], messages: [replyMessage], draft: '失败草稿' };
		useChatMock.mockReturnValue(firstResult);
		// rerender 用于模拟失败后立刻切换到 chat-2 的页面上下文。
		const { rerender } = render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		fireEvent.click(screen.getByRole('menuitem', { name: '回复' }));
		fireEvent.click(screen.getByRole('button', { name: '发送消息' }));
		await waitFor(/* failedReplyCalled 等待 chat-1 引用请求明确失败并保留原引用条。 */ () => expect(handleSendMock).toHaveBeenCalledWith('failed-switch-target.PNM'));
		expect(screen.getByLabelText('引用回复预览')).toBeTruthy();
		// secondResult 保留切换瞬间的旧组件状态，用于验证渲染守卫无需等待 effect 就能隐藏引用条。
		const secondResult = { ...firstResult, activeChatID: 'chat-2', selectedSession: otherSession, draft: '' };
		useChatMock.mockReturnValue(secondResult);
		rerender(<Chat />);
		expect(screen.queryByLabelText('引用回复预览')).toBeNull();
		expect(screen.getByPlaceholderText('输入消息，Enter 发送，Shift + Enter 换行，Ctrl + V 粘贴图片')).toBeTruthy();
		expect(handleSendMock).toHaveBeenCalledTimes(1);
	});

	test('缺少平台标识的历史消息不允许进入引用预览', /* 当前回调验证界面不会对无法精确定位的消息伪装可回复。 */ () => {
		// unsupportedMessage 没有 PNM 字段或后缀，只能保留复制能力。
		const unsupportedMessage = { ...messageFixtures[0], message_key: 'legacy-message' } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [unsupportedMessage] });
		render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		// replyButton 保留可见性以解释能力边界，但不可触发预览。
		const replyButton = screen.getByRole('menuitem', { name: '回复' }) as HTMLButtonElement;
		expect(replyButton.disabled).toBe(true);
		expect(replyButton.title).toContain('缺少平台标识');
	});

	test('位置消息按卡片展示且输入区可以打开自定义发送弹窗', /* locationCardComposerCase 验证入口和结构化显示。 */ async () => {
		// locationMessage 是不包含任意跳转地址的结构化位置消息。
		const locationMessage = { ...messageFixtures[0], message_key: 'location-1.PNM', platform_message_id: 'location-1.PNM', message_type: 'location', content: '{location}', location_card: { title: 'CoverAI 实体店', description: '东门电梯上楼右转', latitude: 22.540503, longitude: 113.934528 } } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [locationMessage] });
		render(<Chat />);
		expect(screen.getByText('CoverAI 实体店')).toBeTruthy();
		expect(screen.getByText('东门电梯上楼右转')).toBeTruthy();
		expect(screen.getByText('22.540503, 113.934528')).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: '发送位置卡片' }));
		expect(await screen.findByRole('dialog', { name: '发送位置卡片' })).toBeTruthy();
	});

	test('会话切换期间迟到的旧消息不能被标记为当前会话引用目标', /* 当前回调验证同买家多商品会话切换时不会向服务端提交伪造归属。 */ () => {
		// staleImage 是 chat-2 迟到但暂时仍出现在 chat-1 消息列表中的带 PNM 图片。
		const staleImage = { ...messageFixtures[0], id: 88, account_id: 'account-1', chat_id: 'chat-2', message_key: 'stale-image.PNM', platform_message_id: 'stale-image.PNM', message_type: 'image', content: 'https://img.example/stale.png' } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [staleImage] });
		render(<Chat />);
		fireEvent.click(screen.getByRole('button', { name: '消息操作' }));
		// replyButton 是跨会话迟到消息的禁用回复入口。
		const replyButton = screen.getByRole('menuitem', { name: '回复' }) as HTMLButtonElement;
		expect(replyButton.disabled).toBe(true);
		expect(replyButton.title).toContain('不属于当前会话');
		fireEvent.click(replyButton);
		expect(screen.queryByLabelText('引用回复预览')).toBeNull();
		expect(handleSendMock).not.toHaveBeenCalled();
	});

	test('底部消息菜单空间不足时向上展开', /* 当前回调验证滚动容器不会裁剪底部消息的完整菜单。 */ () => {
		// bottomMessage 是用于模拟靠近聊天滚动区底部的己方文本消息。
		const bottomMessage = { ...messageFixtures[0], direction: 'outgoing', sender_id: 'account-1', message_key: 'bottom-menu-message', status: 'sent', sent_at: Date.now() } as ChatMessage;
		useChatMock.mockReturnValue({ ...createChatResult(), messages: [bottomMessage] });
		render(<Chat />);
		// messageLog 是提供底部裁剪边界的聊天滚动视口。
		const messageLog = screen.getByRole('log', { name: '聊天消息' });
		// menuButton 是底部消息旁用于打开自适应菜单的按钮。
		const menuButton = screen.getByRole('button', { name: '消息操作' });
		// messageOwner 是按钮所属且需要与滚动视口比较空间的消息行。
		const messageOwner = menuButton.closest<HTMLElement>('[data-message-menu-owner]');
		if (!messageOwner) throw new Error('测试消息行不存在');
		Object.defineProperty(messageLog, 'getBoundingClientRect', { configurable: true, value: /* viewportRectMock 返回高度 400 的聊天滚动视口。 */ () => ({ top: 100, bottom: 500 } as DOMRect) });
		Object.defineProperty(messageOwner, 'getBoundingClientRect', { configurable: true, value: /* ownerRectMock 把消息行放在距视口底部 20 像素的位置。 */ () => ({ top: 440, bottom: 480 } as DOMRect) });
		fireEvent.click(menuButton);
		// menu 是应使用 bottom-full 向上展开的底部消息菜单。
		const menu = screen.getByRole('menu', { name: '消息操作菜单' });
		expect(menu.className).toContain('bottom-full');
		expect(menu.className).not.toContain('top-full');
	});
});
