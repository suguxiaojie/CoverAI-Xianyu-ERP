// @vitest-environment jsdom
import { act,renderHook,waitFor } from '@testing-library/react';
import { beforeEach,describe,expect,test,vi } from 'vitest';
import type { AccountDetail,ChatMessage,ChatSession } from '../../../shared/api-contract/chat';
import { getAccountDetails,getAccountRuntimeStatuses,getChatLocationCardDefaults,getChatMessagePage,getChatSessionPage,getChatUserCredit,getCloseOrderEligibility,markChatRead,recallChatMessage,sendChatImage,sendChatLocationCard,sendChatMessage,setChatSessionPinned } from './api';
import { useChat } from './hooks';
import { publishChatConnectionState,publishChatLiveMessage } from './liveEvents';

vi.mock('./api', /* chatApiMockFactory 提供聊天 Hook 的确定性 API 替身。 */ () => ({
  getAccountDetails: vi.fn(),
  getAccountRuntimeStatuses: vi.fn(),
	getChatLocationCardDefaults: vi.fn(),
  getChatMessagePage: vi.fn(),
  getChatSessionPage: vi.fn(),
	getChatUserCredit: vi.fn(),
	getCloseOrderEligibility: vi.fn(),
  markChatRead: vi.fn(),
  sendChatImage: vi.fn(),
	sendChatLocationCard: vi.fn(),
  sendChatMessage: vi.fn(),
  recallChatMessage: vi.fn(),
  setChatSessionPinned: vi.fn(),
}));

// getDetailsMock 是聊天账号详情请求的可控替身。
const getDetailsMock = vi.mocked(getAccountDetails);
// getRuntimeMock 是聊天账号运行状态请求的可控替身。
const getRuntimeMock = vi.mocked(getAccountRuntimeStatuses);
// getLocationDefaultsMock 是系统设置页位置默认值读取替身。
const getLocationDefaultsMock = vi.mocked(getChatLocationCardDefaults);
// getMessagePageMock 是聊天消息分页请求的可控替身。
const getMessagePageMock = vi.mocked(getChatMessagePage);
// getSessionPageMock 是聊天会话分页请求的可控替身。
const getSessionPageMock = vi.mocked(getChatSessionPage);
// getCreditMock 是当前会话停留后单买家信用请求的可控替身。
const getCreditMock = vi.mocked(getChatUserCredit);
// getCloseEligibilityMock 是顶部关单入口纯本地资格请求的可控替身。
const getCloseEligibilityMock = vi.mocked(getCloseOrderEligibility);
// markReadMock 是聊天已读请求的可控替身。
const markReadMock = vi.mocked(markChatRead);
// sendImageMock 是聊天图片发送请求的可控替身。
const sendImageMock = vi.mocked(sendChatImage);
// sendLocationMock 是聊天位置卡片请求的可控替身。
const sendLocationMock = vi.mocked(sendChatLocationCard);
// sendMessageMock 是聊天文字发送请求的可控替身。
const sendMessageMock = vi.mocked(sendChatMessage);
// recallMessageMock 是聊天撤回请求的可控替身。
const recallMessageMock = vi.mocked(recallChatMessage);
// setSessionPinnedMock 是聊天会话置顶持久化 API 的可控替身。
const setSessionPinnedMock = vi.mocked(setChatSessionPinned);

// accountFixture 是聊天 Hook 使用的启用账号对象。
const accountFixture: AccountDetail = { id: 'account-1', enabled: true, auto_confirm: false, nickname: '测试账号' };
// sessionFixture 是聊天会话列表中的当前会话。
const sessionFixture: ChatSession = { account_id: 'account-1', chat_id: 'chat-1', buyer_id: 'buyer-1', buyer_name: '买家', item_title: '商品', last_message: '你好', last_message_at: 1, unread_count: 1 };
// messageFixture 是当前会话中的历史消息。
const messageFixture = { id: 1, account_id: 'account-1', chat_id: 'chat-1', message_key: 'message-1', direction: 'incoming', sender_id: 'buyer-1', sender_name: '买家', message_type: 'text', content: '你好', status: 'received', sent_at: 1 } as never as ChatMessage;
// sentMessageFixture 是文字发送成功后返回的消息。
const sentMessageFixture = { ...messageFixture, id: 2, message_key: 'message-2', direction: 'outgoing', content: '回复内容' } as ChatMessage;

describe('useChat', /* 当前回调处理聊天加载、分页、发送和实时连接状态。 */ () => {
  beforeEach(/* 当前回调重置聊天 API 替身和全局实时连接状态。 */ () => {
    vi.clearAllMocks();
    getDetailsMock.mockResolvedValue([accountFixture]);
    getRuntimeMock.mockResolvedValue({ 'account-1': { state: 'online', connected: true, failures: 0, updated_at: '2026-08-15T00:00:00Z' } });
	getLocationDefaultsMock.mockResolvedValue({ title: '默认门店', description: '默认入口', latitude: '31.230400', longitude: '121.473700' });
    getSessionPageMock.mockResolvedValue({ sessions: [sessionFixture], has_more: true, next_cursor: 2 });
    getMessagePageMock.mockResolvedValue({ messages: [messageFixture], has_more: true, next_cursor: 2, session: sessionFixture });
	getCreditMock.mockResolvedValue({ user_id: 'buyer-1', buyer: { role: 'buyer', level: 5, code: 'cs_buyer_level', text: '买家信用极好' }, seller: { role: 'seller', level: 4, code: 'cs_seller_level', text: '卖家信用优秀' }, fetched_at: '2026-08-27T02:30:00Z', expires_at: '2026-08-28T02:30:00Z', stale: false });
	getCloseEligibilityMock.mockResolvedValue({ eligible: true, account_id: 'account-1', order_id: '5127638256187075541', stage: 'pending_payment' });
    markReadMock.mockResolvedValue({ success: true });
    sendMessageMock.mockResolvedValue({ message: sentMessageFixture });
    sendImageMock.mockResolvedValue({ message: sentMessageFixture });
	sendLocationMock.mockResolvedValue({ message: { ...sentMessageFixture, message_type: 'location', location_card: { title: '实体店', description: '东门', latitude: 22.5, longitude: 113.9 } } });
	recallMessageMock.mockResolvedValue({ message: { ...sentMessageFixture, status: 'recalled', recalled_at: Date.now() } });
    setSessionPinnedMock.mockResolvedValue({ account_id: 'account-1', chat_id: 'chat-1', pinned: true });
    publishChatConnectionState('connecting');
    // localStorageStub 是聊天 Hook 记忆账号选择所需的浏览器存储替身。
    Object.defineProperty(window, 'localStorage', { configurable: true, value: { getItem: vi.fn().mockReturnValue(''), setItem: vi.fn(), removeItem: vi.fn() } });
    // createObjectURLMock 和 revokeObjectURLMock 模拟浏览器图片预览地址的创建与释放。
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: vi.fn(/* 当前回调为测试图片生成稳定的临时地址。 */ (file: File) => `blob:${file.name}`) });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() });
		window.history.replaceState({}, '', '/app/chat');
  });

	test('订单深链加载较早会话并切换到精确账号和商品', /* orderChatDeepLinkCase 验证联系人分页之外的目标也能直达。 */ async () => {
		// secondAccount 是订单所属但不是默认本地偏好的启用账号。
		const secondAccount: AccountDetail = { id: 'account-2', enabled: true, auto_confirm: false, nickname: '订单账号' };
		// secondDefaultSession 是第二账号初始联系人页中的其他会话。
		const secondDefaultSession: ChatSession = { ...sessionFixture, account_id: 'account-2', chat_id: 'chat-other', buyer_id: 'buyer-other', item_id: 'item-other' };
		// pinnedSession 是第二账号真实置顶会话，订单目标必须排在它之后而不是覆盖其顺序。
		const pinnedSession: ChatSession = { ...sessionFixture, account_id: 'account-2', chat_id: 'chat-pinned', buyer_id: 'buyer-pinned', item_id: 'item-pinned', is_pinned: true };
		// targetSession 是订单精确 chat_id 读取返回的较早商品会话。
		const targetSession: ChatSession = { ...sessionFixture, account_id: 'account-2', chat_id: 'chat-target', buyer_id: 'buyer-target', buyer_name: '目标买家', item_id: 'item-target', item_title: '目标商品' };
		getDetailsMock.mockResolvedValue([accountFixture, secondAccount]);
		getRuntimeMock.mockResolvedValue({
			'account-1': { state: 'online', connected: true, failures: 0, updated_at: '' },
			'account-2': { state: 'online', connected: true, failures: 0, updated_at: '' },
		});
		getSessionPageMock.mockImplementation(/* sessionPageByAccount 返回两个账号不同的初始联系人页。 */ async accountID => ({ sessions: accountID === 'account-2' ? [pinnedSession, secondDefaultSession] : [sessionFixture], has_more: false }));
		getMessagePageMock.mockImplementation(/* messagePageByChat 为目标 chat_id 返回精确会话摘要。 */ async (accountID, chatID) => ({ messages: [messageFixture], has_more: false, session: accountID === 'account-2' && chatID === 'chat-target' ? targetSession : sessionFixture }));
		window.history.replaceState({}, '', '/app/chat?account_id=account-2&chat_id=chat-target&item_id=item-target');
		// hook 是从订单深链启动的聊天状态。
		const hook = renderHook(/* deepLinkedChatHookFactory 创建带精确目标的 Chat Hook。 */ () => useChat());
		await waitFor(/* targetAccountAssertion 等待订单账号和会话同时成为活跃上下文。 */ () => {
			expect(hook.result.current.activeAccountID).toBe('account-2');
			expect(hook.result.current.activeChatID).toBe('chat-target');
		});
		expect(getMessagePageMock).toHaveBeenCalledWith('account-2', 'chat-target', undefined, undefined, expect.objectContaining({ signal: expect.any(AbortSignal) }));
		expect(hook.result.current.selectedSession).toMatchObject({ chat_id: 'chat-target', item_id: 'item-target' });
		expect(hook.result.current.filteredSessions.map(/* session 是当前读取订单深链可见顺序的会话。 */ session => session.chat_id)).toEqual(['chat-pinned', 'chat-target', 'chat-other']);
		expect(hook.result.current.navigationPriorityChatID).toBe('chat-target');
		expect(window.location.pathname).toBe('/app/chat');
		expect(window.location.search).toBe('');
		hook.unmount();
	});

  test('加载账号、会话和消息后可以发送文字与图片', /* 当前回调验证聊天 Hook 成功加载和发送路径。 */ async () => {
    // hook 是聊天 Hook 的渲染结果。
    const hook = renderHook(
      // chatHookFactory 创建聊天 Hook。
      () => useChat(),
    );
    await waitFor(
      // loadingAssertion 等待账号和会话加载完成。
      () => expect(hook.result.current.loading).toBe(false),
    );
    await waitFor(
      // activeChatAssertion 等待默认会话被选中。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await waitFor(
      // messagesAssertion 等待当前会话消息加载完成。
      () => expect(hook.result.current.messagesLoading).toBe(false),
    );
    expect(hook.result.current.accounts[0]).toMatchObject(accountFixture);
    expect(hook.result.current.activeChatID).toBe('chat-1');
    expect(hook.result.current.messages).toEqual([messageFixture]);
	await waitFor(/* creditProfileAssertion 等待五百毫秒停留后只查询当前买家信用。 */ () => expect(hook.result.current.creditProfile?.buyer?.level).toBe(5));
	expect(getCreditMock).toHaveBeenCalledWith('account-1', 'buyer-1', expect.objectContaining({ signal: expect.any(AbortSignal), timeoutMs: 35_000 }));
    expect(markReadMock).toHaveBeenCalledWith('account-1', 'chat-1', [
      { messageId: 'message-1', sessionId: 'chat-1', cid: 'chat-1@goofish', conversationType: 1 },
    ], expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(hook.result.current.unreadForAccount('account-1')).toBe(0);
	await act(
		// defaultsAction 在打开位置弹窗前读取系统设置默认值。
		async () => hook.result.current.loadLocationCardDefaults(),
	);
	expect(hook.result.current.locationCardDefaults).toEqual({ title: '默认门店', description: '默认入口', latitude: '31.230400', longitude: '121.473700' });
    await act(
      // emptySendAction 在没有草稿时阻止文字发送。
      async () => hook.result.current.handleSend(),
    );

	// historyViewport 模拟用户停留在较早消息位置的聊天滚动容器。
	const historyViewport = { scrollHeight: 1000, scrollTop: 100, clientHeight: 200 } as unknown as HTMLDivElement;
	hook.result.current.scrollRef.current = historyViewport;
	hook.result.current.handleMessageScroll();
    await act(
      // draftAction 写入文字消息草稿。
      () => hook.result.current.setDraft('回复内容'),
    );
    await act(
      // sendAction 提交文字消息。
      async () => hook.result.current.handleSend(),
    );
    expect(sendMessageMock).toHaveBeenCalledWith(expect.objectContaining({ text: '回复内容', chat_id: 'chat-1' }), expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(hook.result.current.messages).toContainEqual(sentMessageFixture);
	expect(historyViewport.scrollTop).toBe(historyViewport.scrollHeight);
	await act(
		// recallAction 撤回刚发送的本地消息并用服务端状态替换。
		async () => hook.result.current.recallMessage(sentMessageFixture.message_key),
	);
	expect(recallMessageMock).toHaveBeenCalledWith('account-1', sentMessageFixture.message_key);
	expect(hook.result.current.messages.find(/* currentMessage 查找撤回后的目标消息。 */ currentMessage => currentMessage.message_key === sentMessageFixture.message_key)?.status).toBe('recalled');

    await act(
      // imageAction 提交图片消息。
      async () => hook.result.current.handleImage(new File(['image'], 'image.png', { type: 'image/png' })),
    );
    expect(hook.result.current.pendingImage?.url).toBe('blob:image.png');
    await act(
      // confirmImageAction 确认预览后提交图片消息。
      async () => hook.result.current.confirmSendImage(),
    );
    expect(sendImageMock).toHaveBeenCalledWith(expect.objectContaining({ chat_id: 'chat-1', image: expect.any(File) }), expect.objectContaining({ signal: expect.any(AbortSignal) }));
	await act(
		// locationAction 提交用户在弹窗中确认的标题、说明和坐标。
		async () => hook.result.current.handleSendLocationCard({ title: 'CoverAI 实体店', description: '东门电梯上楼右转', latitude: 22.540503, longitude: 113.934528 }),
	);
	expect(sendLocationMock).toHaveBeenCalledWith(expect.objectContaining({ account_id: 'account-1', chat_id: 'chat-1', buyer_id: 'buyer-1', title: 'CoverAI 实体店', latitude: 22.540503, longitude: 113.934528 }), expect.objectContaining({ signal: expect.any(AbortSignal) }));
    // imageInput 是图片发送成功后需要清空的输入框替身。
    const imageInput = { value: 'selected-file' } as HTMLInputElement;
    hook.result.current.imageInputRef.current = imageInput;
    await act(
      // secondImageAction 再次发送图片以验证输入框清理。
      async () => hook.result.current.handleImage(new File(['image'], 'second.png', { type: 'image/png' })),
    );
    await act(
      // confirmSecondImageAction 确认第二张图片并验证发送后的输入清理。
      async () => hook.result.current.confirmSendImage(),
    );
    expect(imageInput.value).toBe('');
    await act(
      // emptyImageAction 在没有图片文件时阻止发送。
      async () => hook.result.current.handleImage(),
    );
    // emptyScrollAction 在没有滚动容器时保持滚动策略稳定。
    hook.result.current.scrollRef.current = null;
    await act(
      // nullScrollAction 验证滚动容器缺失守卫。
      () => hook.result.current.handleMessageScroll(),
    );
    hook.unmount();
  });

  test('联系人分页和发送失败都提供可重试状态', /* 当前回调验证聊天分页和错误重试路径。 */ async () => {
    // hook 是聊天失败场景的 Hook 渲染结果。
    const hook = renderHook(
      // failedChatHookFactory 创建聊天错误场景的 Hook。
      () => useChat(),
    );
    await waitFor(
      // loadingAssertion 等待错误场景的账号加载完成。
      () => expect(hook.result.current.loading).toBe(false),
    );
    await waitFor(
      // activeChatAssertion 等待错误场景的默认会话被选中。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await waitFor(
      // contactsAssertion 等待联系人分页标记生效。
      () => expect(hook.result.current.hasMoreContacts).toBe(true),
    );
    await waitFor(
      // messagesAssertion 等待错误场景的消息加载完成。
      () => expect(hook.result.current.messagesLoading).toBe(false),
    );
    await act(
      // contactsAction 请求更早的联系人。
      async () => hook.result.current.loadMoreContacts(),
    );
    expect(getSessionPageMock).toHaveBeenCalledWith('account-1', undefined, expect.objectContaining({ signal: expect.any(AbortSignal) }), true);

    sendMessageMock.mockRejectedValueOnce(new Error('发送失败'));
    await act(
      // draftAction 写入会失败的文字草稿。
      () => hook.result.current.setDraft('失败消息'),
    );
    await act(
		// failedSendAction 提交会失败的原生引用文本。
		async () => hook.result.current.handleSend('message-1'),
    );
    expect(hook.result.current.error).toBe('发送失败');
    expect(hook.result.current.retryAvailable).toBe(true);
    sendMessageMock.mockResolvedValueOnce({ message: sentMessageFixture });
    await act(
		// retryAction 重试最近一次失败的引用文本。
      async () => hook.result.current.retrySend(),
    );
    expect(sendMessageMock).toHaveBeenCalledTimes(2);
	expect(sendMessageMock).toHaveBeenNthCalledWith(1, expect.objectContaining({ text: '失败消息', reply_to_message_key: 'message-1' }), expect.anything());
	expect(sendMessageMock).toHaveBeenNthCalledWith(2, expect.objectContaining({ text: '失败消息', reply_to_message_key: 'message-1' }), expect.anything());
    hook.unmount();
  });

  test('引用发送失败后切换会话会清理旧错误和重试状态', /* 当前回调验证失败引用不能在其他买家会话继续显示或重试。 */ async () => {
		// secondSession 是失败后切换进入的另一个买家会话。
		const secondSession: ChatSession = { ...sessionFixture, chat_id: 'chat-2', buyer_id: 'buyer-2', buyer_name: '其他买家', last_message: '其他消息', last_message_at: 2, unread_count: 0 };
		getSessionPageMock.mockResolvedValue({ sessions: [sessionFixture, secondSession], has_more: false });
		// hook 是当前验证引用失败状态生命周期的聊天 Hook。
		const hook = renderHook(
			// failedReplyHookFactory 创建包含两个会话的引用失败场景。
			() => useChat(),
		);
		await waitFor(
			// activeChatAssertion 等待默认会话准备完成。
			() => expect(hook.result.current.activeChatID).toBe('chat-1'),
		);
		sendMessageMock.mockRejectedValueOnce(new Error('引用发送失败'));
		await act(
			// draftAction 写入仅属于 chat-1 的失败引用草稿。
			() => hook.result.current.setDraft('失败引用草稿'),
		);
		await act(
			// failedReplyAction 提交带目标键且会失败的引用消息。
			async () => hook.result.current.handleSend('message-1'),
		);
		expect(hook.result.current.error).toBe('引用发送失败');
		expect(hook.result.current.retryAvailable).toBe(true);
		await act(
			// switchConversationAction 切换到 chat-2 并触发旧发送状态清理。
			() => hook.result.current.setActiveChatID('chat-2'),
		);
		await waitFor(
			// clearedFailureAssertion 等待旧会话错误和重试入口同时消失。
			() => {
				expect(hook.result.current.error).toBe('');
				expect(hook.result.current.retryAvailable).toBe(false);
				expect(hook.result.current.sending).toBe(false);
			},
		);
		hook.unmount();
	});

	test('聊天待付款卡片必须经过后端本地资格确认才公开取消订单号', /* 当前回调验证订单状态查询可取消且不会被旧会话响应覆盖。 */ async () => {
		// pendingOrderMessage 是只有聊天证据、仍需后端订单表确认的卖家待付款卡片。
		const pendingOrderMessage = { ...messageFixture, message_key: 'pending-close.PNM', message_type: 'system', sender_id: 'buyer-1', content: '我已拍下，待付款', system_card: { kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款', order_id: '5127638256187075541', action: 'adjust_price' } } as ChatMessage;
		getMessagePageMock.mockResolvedValue({ messages: [pendingOrderMessage], has_more: false, session: sessionFixture });
		// hook 是本地资格异步确认场景的聊天 Hook。
		const hook = renderHook(
			// closeEligibilityHookFactory 创建不会访问真实订单或平台的资格场景。
			() => useChat(),
		);
		await waitFor(
			// eligibilityAssertion 等待后端确认当前唯一聊天候选仍为待付款。
			() => expect(hook.result.current.closeEligibleOrderID).toBe('5127638256187075541'),
		);
		expect(hook.result.current.closeEligibleOrderStage).toBe('pending_payment');
		expect(getCloseEligibilityMock).toHaveBeenCalledWith('account-1', '5127638256187075541', expect.objectContaining({ signal: expect.any(AbortSignal) }));
		hook.unmount();
	});

	test('聊天已付款卡片通过后端确认后公开待发货取消阶段', /* paidCloseEligibilityHookCase 验证付款卡片不会把统一取消入口错误终止。 */ async () => {
		// paidOrderMessage 是已付款但尚未发货的卖家操作卡片。
		const paidOrderMessage = { ...messageFixture, message_key: 'paid-close.PNM', message_type: 'system', sender_id: 'buyer-1', content: '我已付款，等待你发货', system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: '5127638256187075541', action: 'ship_order' } } as ChatMessage;
		getMessagePageMock.mockResolvedValue({ messages: [paidOrderMessage], has_more: false, session: sessionFixture });
		getCloseEligibilityMock.mockResolvedValue({ eligible: true, account_id: 'account-1', order_id: '5127638256187075541', stage: 'pending_ship' });
		// hook 是已付款待发货资格异步确认场景的聊天 Hook。
		const hook = renderHook(
			// paidCloseEligibilityHookFactory 创建不会访问真实订单或平台的资格场景。
			() => useChat(),
		);
		await waitFor(
			// paidEligibilityAssertion 等待后端确认已付款订单仍可取消。
			() => expect(hook.result.current.closeEligibleOrderStage).toBe('pending_ship'),
		);
		expect(hook.result.current.closeEligibleOrderID).toBe('5127638256187075541');
		hook.unmount();
	});

  test('会话刷新不会让当前打开会话重新显示未读', /* 当前回调验证已读状态与慢刷新响应的竞态边界。 */ async () => {
    // hook 是当前会话已读后执行刷新的 Hook 渲染结果。
    const hook = renderHook(
      // refreshHookFactory 创建会话刷新竞态场景。
      () => useChat(),
    );
    await waitFor(
      // activeChatAssertion 等待默认会话完成消息读取。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await waitFor(
      // readAssertion 等待初始消息读取将本地未读数归零。
      () => expect(hook.result.current.unreadForAccount('account-1')).toBe(0),
    );
    // staleSessionPage 模拟服务端延迟返回的旧未读计数。
    const staleSessionPage = { ...sessionFixture, unread_count: 4 };
    getSessionPageMock.mockResolvedValueOnce({ sessions: [staleSessionPage], has_more: false });
    await act(
      // refreshAction 执行返回旧未读数据的会话刷新。
      async () => hook.result.current.reloadSessions('account-1'),
    );
    expect(hook.result.current.unreadForAccount('account-1')).toBe(0);
    hook.unmount();
  });

	test('历史消息搜索支持账号切换取消并拒绝旧响应覆盖', /* historicalSessionSearchIsolationCase 验证 debounce、取消和代次隔离。 */ async () => {
		// secondAccount 是搜索期间切换进入的另一个启用账号。
		const secondAccount: AccountDetail = { id: 'account-2', enabled: true, auto_confirm: false, nickname: '第二账号' };
		getDetailsMock.mockResolvedValue([accountFixture, secondAccount]);
		getRuntimeMock.mockResolvedValue({
			'account-1': { state: 'online', connected: true, failures: 0, updated_at: '2026-08-15T00:00:00Z' },
			'account-2': { state: 'online', connected: true, failures: 0, updated_at: '2026-08-15T00:00:00Z' },
		});
		// secondSession 是第二账号正常联系人页中的默认会话。
		const secondSession: ChatSession = { ...sessionFixture, account_id: 'account-2', chat_id: 'chat-2', buyer_id: 'buyer-2', buyer_name: '第二买家' };
		// firstSearchResult、secondSearchResult 是两个账号按同关键词命中的不同历史会话。
		const firstSearchResult: ChatSession = { ...sessionFixture, chat_id: 'history-account-1', buyer_name: '旧账号结果', last_message: '地址在这里' };
		const secondSearchResult: ChatSession = { ...secondSession, chat_id: 'history-account-2', buyer_name: '新账号结果', last_message: '新的摘要' };
		// resolveFirstSearch 在账号切换后才释放第一账号的晚到响应。
		let resolveFirstSearch: ((page: {
			/** sessions 是第一账号晚到的历史搜索结果。 */ sessions: ChatSession[];
			/** has_more 对搜索响应固定为 false。 */ has_more: boolean;
		}) => void) | undefined;
		// firstSearchSignal 保存第一账号搜索收到的 AbortSignal。
		let firstSearchSignal: AbortSignal | undefined;
		getSessionPageMock.mockImplementation(/* sessionSearchMock 按账号和可选关键词返回正常页或受控晚到结果。 */ (accountID, _cursor, options, _refresh, searchText) => {
			if (searchText && accountID === 'account-1') {
				firstSearchSignal = options?.signal;
				return new Promise(/* firstSearchPending 保持第一账号响应未完成直到切换后手工释放。 */ resolve => { resolveFirstSearch = resolve; });
			}
			if (searchText && accountID === 'account-2') return Promise.resolve({ sessions: [secondSearchResult], has_more: false });
			return Promise.resolve({ sessions: [accountID === 'account-2' ? secondSession : sessionFixture], has_more: false });
		});
		// hook 是当前验证历史搜索请求生命周期的聊天 Hook。
		const hook = renderHook(/* historicalSearchHookFactory 创建双账号搜索场景。 */ () => useChat());
		await waitFor(/* initialAccountLoaded 等待第一账号默认加载完成。 */ () => expect(hook.result.current.activeAccountID).toBe('account-1'));
		await act(/* startHistoricalSearch 输入只存在于历史消息中的卡密。 */ () => hook.result.current.setSearch('TEST-CARD-CODE-001'));
		await waitFor(/* firstSearchStarted 等待 debounce 后第一账号请求建立。 */ () => expect(firstSearchSignal).toBeDefined(), { timeout: 1500 });
		await act(/* switchSearchAccount 在旧响应完成前切换账号。 */ () => hook.result.current.setActiveAccountID('account-2'));
		await waitFor(/* secondSearchRendered 等待同关键词的新账号搜索结果可见。 */ () => expect(hook.result.current.filteredSessions.map(/* session 是当前可见搜索会话。 */ session => session.chat_id)).toEqual(['history-account-2']), { timeout: 1500 });
		expect(firstSearchSignal?.aborted).toBe(true);
		await act(/* resolveStaleSearch 手工释放已经取消的第一账号晚到响应。 */ async () => { resolveFirstSearch?.({ sessions: [firstSearchResult], has_more: false }); });
		expect(hook.result.current.filteredSessions.map(/* session 是旧响应释放后的当前结果。 */ session => session.chat_id)).toEqual(['history-account-2']);
		await act(/* clearHistoricalSearch 清空关键词并恢复当前账号正常会话缓存。 */ () => hook.result.current.setSearch(''));
		await waitFor(/* normalSessionsRestored 等待普通联系人列表重新可见。 */ () => expect(hook.result.current.filteredSessions.some(/* session 是清空搜索后的普通账号会话。 */ session => session.chat_id === 'chat-2')).toBe(true));
		hook.unmount();
	});

	test('历史消息搜索失败显示错误且不清空正常会话缓存', /* historicalSessionSearchFailureCase 验证失败不破坏已加载联系人。 */ async () => {
		getSessionPageMock.mockImplementation(/* failedSearchMock 只让非空关键词请求失败。 */ (_accountID, _cursor, _options, _refresh, searchText) => searchText
			? Promise.reject(new Error('本地搜索失败'))
			: Promise.resolve({ sessions: [sessionFixture], has_more: false }));
		// hook 是观察搜索失败和正常会话缓存的聊天 Hook。
		const hook = renderHook(/* failedHistoricalSearchHookFactory 创建失败搜索场景。 */ () => useChat());
		await waitFor(/* initialSessionsLoaded 等待普通会话加载完成。 */ () => expect(hook.result.current.activeSessions.some(/* session 是当前检查默认会话标识的摘要。 */ session => session.chat_id === sessionFixture.chat_id)).toBe(true));
		await act(/* startFailedSearch 输入会触发失败的历史搜索关键词。 */ () => hook.result.current.setSearch('失败关键词'));
		await waitFor(/* searchFailureVisible 等待最新搜索错误写入页面状态。 */ () => expect(hook.result.current.error).toBe('本地搜索失败'), { timeout: 1500 });
		expect(hook.result.current.activeSessions.some(/* session 是搜索失败后仍保留的普通会话。 */ session => session.chat_id === sessionFixture.chat_id)).toBe(true);
		await act(/* clearFailedSearch 清空失败关键词。 */ () => hook.result.current.setSearch(''));
		await waitFor(/* normalSessionVisibleAfterFailure 等待普通会话重新显示。 */ () => expect(hook.result.current.filteredSessions.some(/* session 是清空搜索后恢复的普通会话。 */ session => session.chat_id === sessionFixture.chat_id)).toBe(true));
		hook.unmount();
	});

  test('会话置顶成功后仅更新原账号并移到置顶组', /* 当前回调验证服务端权威结果和账号隔离。 */ async () => {
    // newerSession 是原列表中比目标更新的普通会话。
    const newerSession = { ...sessionFixture, chat_id: 'chat-2', buyer_id: 'buyer-2', last_message_at: 2, unread_count: 0 };
    getSessionPageMock.mockResolvedValue({ sessions: [newerSession, sessionFixture], has_more: false });
    // hook 是提交置顶并观察会话排序的聊天 Hook。
    const hook = renderHook(/* pinHookFactory 创建置顶成功场景。 */ () => useChat());
    await waitFor(/* loadingAssertion 等待会话首页加载完成。 */ () => expect(hook.result.current.loading).toBe(false));
    await act(/* pinAction 提交当前账号中较早会话的置顶请求。 */ async () => hook.result.current.setSessionPinned('chat-1', true));
    expect(setSessionPinnedMock).toHaveBeenCalledWith('account-1', 'chat-1', true, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(hook.result.current.activeSessions.map(/* session 是当前读取置顶后顺序的会话。 */ session => session.chat_id)).toEqual(['chat-1', 'chat-2']);
    expect(hook.result.current.activeSessions[0].is_pinned).toBe(true);
    expect(hook.result.current.pinningSessionIDs.size).toBe(0);
    hook.unmount();
  });

  test('会话置顶失败保留原状态并显示错误', /* 当前回调验证失败请求不会伪装成本地成功。 */ async () => {
    setSessionPinnedMock.mockRejectedValueOnce(new Error('持久化失败'));
    // hook 是观察失败状态与原会话数据的聊天 Hook。
    const hook = renderHook(/* failedPinHookFactory 创建置顶失败场景。 */ () => useChat());
    await waitFor(/* loadingAssertion 等待聊天初始数据加载完成。 */ () => expect(hook.result.current.loading).toBe(false));
    await act(/* failedPinAction 提交会被 API 拒绝的置顶请求。 */ async () => hook.result.current.setSessionPinned('chat-1', true));
    expect(hook.result.current.activeSessions[0].is_pinned).not.toBe(true);
    expect(hook.result.current.error).toBe('持久化失败');
    hook.unmount();
  });

  test('应用壳唯一实时连接发布的状态和消息会更新聊天状态', /* 当前回调验证 Chat 页面不再自行创建 WebSocket。 */ async () => {
    // secondSession 是用于覆盖实时会话排序比较器的第二条会话。
    const secondSession = { ...sessionFixture, chat_id: 'chat-2', last_message_at: 2, unread_count: 0 };
    getSessionPageMock.mockResolvedValue({ sessions: [sessionFixture, secondSession], has_more: true, next_cursor: 2 });
    // outgoingFixture 是当前会话中等待后续买家消息确认的出站消息。
    const outgoingFixture = { ...messageFixture, id: 2, message_key: 'outgoing-1', direction: 'outgoing' as const, status: 'sent' as const, read_status: 0, read_at: 0, sent_at: 2, content: '我发出的消息' };
    getMessagePageMock.mockResolvedValue({ messages: [outgoingFixture, messageFixture], has_more: true, next_cursor: 2, session: sessionFixture });
    // hook 是实时连接场景的聊天 Hook 渲染结果。
    const hook = renderHook(
      // socketHookFactory 创建实时连接场景的聊天 Hook。
      () => useChat(),
    );
    await waitFor(
      // loadingAssertion 等待聊天初始数据加载完成。
      () => expect(hook.result.current.loading).toBe(false),
    );
    await waitFor(
      // activeChatAssertion 等待实时消息目标会话选中。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await act(
      // onlineAction 触发应用壳全局连接建立成功事件。
      () => publishChatConnectionState('online'),
    );
    expect(hook.result.current.liveState).toBe('online');
    // incomingMessage 是实时 WebSocket 推送的入站消息。
    const incomingMessage = { ...messageFixture, id: 3, message_key: 'message-3', sent_at: 3, content: '实时消息' };
    await act(
      // messageAction 触发应用壳发布的合法实时消息事件。
      () => publishChatLiveMessage(incomingMessage),
    );
    expect(hook.result.current.messages).toContainEqual(incomingMessage);
    expect(hook.result.current.messages.find(/* 当前回调定位需要验证已读状态的出站消息。 */ message => message.message_key === 'outgoing-1')).toMatchObject({ read_status: 2, read_at: 3 });
    expect(markReadMock).toHaveBeenCalledWith('account-1', 'chat-1', [
      { messageId: 'message-3', sessionId: 'chat-1', cid: 'chat-1@goofish', conversationType: 1 },
    ]);
	// outgoingImage 是自动化图片成功后由服务端实时广播的当前会话消息。
	const outgoingImage = { ...messageFixture, id: 4, message_key: 'image-live.PNM', platform_message_id: 'image-live.PNM', direction: 'outgoing' as const, sender_id: 'account-1', message_type: 'image' as const, content: 'https://cdn.example/card.png', status: 'sent' as const, sent_at: 4 };
	await act(
	  // imageMessageAction 触发自动化图片实时广播，当前会话无需切换即可接纳。
	  () => publishChatLiveMessage(outgoingImage),
	);
	expect(hook.result.current.messages).toContainEqual(outgoingImage);
    await act(
      // closeAction 触发应用壳全局连接断开事件。
      () => publishChatConnectionState('offline'),
    );
    expect(hook.result.current.liveState).toBe('offline');
    hook.unmount();
  });

  test('联系人、消息、历史分页和图片发送失败时保留可恢复错误', /* 当前回调验证聊天业务请求错误分支。 */ async () => {
    // hook 是聊天请求错误场景的 Hook 渲染结果。
    const hook = renderHook(
      // errorHookFactory 创建聊天请求错误场景的 Hook。
      () => useChat(),
    );
    await waitFor(
      // loadingAssertion 等待聊天初始数据加载完成。
      () => expect(hook.result.current.loading).toBe(false),
    );
    await waitFor(
      // activeChatAssertion 等待默认会话选中。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );

    getSessionPageMock.mockRejectedValueOnce(new Error('联系人读取失败'));
    await act(
      // contactsErrorAction 触发联系人刷新错误。
      async () => hook.result.current.reloadSessions('account-1'),
    );
    expect(hook.result.current.error).toBe('联系人读取失败');

    getSessionPageMock.mockRejectedValueOnce(new Error('历史联系人失败'));
    await act(
      // moreContactsErrorAction 触发联系人分页错误。
      async () => hook.result.current.loadMoreContacts(),
    );
    expect(hook.result.current.error).toBe('历史联系人失败');

    // scrollContainer 是滚动策略测试使用的最小 DOM 容器。
    const scrollContainer = { scrollHeight: 100, scrollTop: 20, clientHeight: 50 } as HTMLDivElement;
    hook.result.current.scrollRef.current = scrollContainer;
    await act(
      // scrollAction 验证距离底部较远时不自动滚动。
      () => hook.result.current.handleMessageScroll(),
    );
    scrollContainer.scrollTop = 60;
    await act(
      // nearBottomScrollAction 验证接近底部时启用自动滚动。
      () => hook.result.current.handleMessageScroll(),
    );

    getMessagePageMock.mockRejectedValueOnce(new Error('消息读取失败'));
    await act(
      // chatSwitchAction 切换到不存在会话以触发消息读取错误。
      () => hook.result.current.setActiveChatID('chat-2'),
    );
    await waitFor(
      // messageErrorAssertion 等待消息读取错误收口。
      () => expect(hook.result.current.error).toBe('消息读取失败'),
    );

    await act(
      // chatRestoreAction 恢复默认会话以继续验证历史分页。
      () => hook.result.current.setActiveChatID('chat-1'),
    );
    await waitFor(
      // restoredMessageAssertion 等待默认会话消息恢复。
      () => expect(hook.result.current.messagesLoading).toBe(false),
    );
    getMessagePageMock.mockRejectedValueOnce(new Error('历史消息失败'));
    await act(
      // olderErrorAction 触发历史消息分页错误。
      async () => hook.result.current.loadOlderMessages(),
    );
    expect(hook.result.current.error).toBe('历史消息失败');

    sendImageMock.mockRejectedValueOnce(new Error('图片发送失败'));
    await act(
      // imageErrorAction 触发图片发送错误。
      async () => hook.result.current.handleImage(new File(['image'], 'error.png', { type: 'image/png' })),
    );
    await act(
      // confirmImageErrorAction 确认预览并进入图片发送错误分支。
      async () => hook.result.current.confirmSendImage(),
    );
    expect(hook.result.current.error).toBe('图片发送失败');
    expect(hook.result.current.retryAvailable).toBe(true);
    sendImageMock.mockResolvedValueOnce({ message: sentMessageFixture });
    await act(
      // imageRetryAction 重试最近一次图片发送。
      async () => hook.result.current.retrySend(),
    );
    expect(sendImageMock).toHaveBeenCalledTimes(2);
    hook.unmount();
  });

  test('初始聊天数据加载失败时结束加载状态并保留错误', /* 当前回调验证聊天初始化失败的状态收口。 */ async () => {
    getDetailsMock.mockRejectedValueOnce(new Error('聊天初始化失败'));
    // hook 是初始化失败场景的聊天 Hook 渲染结果。
    const hook = renderHook(
      // failedLoadHookFactory 创建初始化失败场景的 Hook。
      () => useChat(),
    );
    await waitFor(
      // loadingAssertion 等待初始化失败后的加载状态收口。
      () => expect(hook.result.current.loading).toBe(false),
    );
    expect(hook.result.current.error).toBe('聊天初始化失败');
    hook.unmount();
  });

  test('图片预览取消和卸载都会释放临时对象地址', /* 当前回调验证图片预览资源不会跨会话泄漏。 */ async () => {
    // hook 是图片预览资源生命周期测试使用的聊天 Hook。
    const hook = renderHook(
      // chatHookFactory 创建图片预览测试 Hook。
      () => useChat(),
    );
    await waitFor(
      // loadingAssertion 等待图片预览测试完成初始加载。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await act(
      // previewAction 创建第一张图片预览。
      async () => hook.result.current.handleImage(new File(['image'], 'cancel.png', { type: 'image/png' })),
    );
    await act(
      // closeAction 取消预览并释放第一张图片地址。
      () => hook.result.current.closeImagePreview(),
    );
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:cancel.png');
    await act(
      // secondPreviewAction 通过剪贴板入口创建第二张图片预览。
      async () => hook.result.current.handlePastedImages([new File(['image'], 'unmount.png', { type: 'image/png' })]),
    );
    hook.unmount();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:unmount.png');
  });

  test('切换会话会使旧图片预览失效', /* 当前回调验证旧会话图片不能误发到新会话。 */ async () => {
    // hook 是会话切换预览隔离测试使用的聊天 Hook。
    const hook = renderHook(
      // chatHookFactory 创建会话切换测试 Hook。
      () => useChat(),
    );
    await waitFor(
      // chatAssertion 等待默认会话准备完成。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await act(
      // previewAction 在原会话创建图片预览。
      async () => hook.result.current.handleImage(new File(['image'], 'switch.png', { type: 'image/png' })),
    );
    await act(
      // switchAction 切换到另一个会话，使原预览不可再发送。
      () => hook.result.current.setActiveChatID('chat-2'),
    );
    await waitFor(
      // previewAssertion 等待旧会话预览被清理。
      () => expect(hook.result.current.pendingImage).toBeNull(),
    );
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:switch.png');
    hook.unmount();
  });

  test('运行状态轮询失败后仍继续调度下一次轮询', /* 当前回调验证运行状态轮询失败的容错与重试。 */ async () => {
    vi.useFakeTimers();
    try {
      // initialStatus 是初始化请求返回的运行状态。
      const initialStatus = { 'account-1': { state: 'online' as const, connected: true, failures: 0, updated_at: '2026-08-15T00:00:00Z' } };
      // refreshedStatus 是首次轮询返回的更新状态。
      const refreshedStatus = { 'account-1': { state: 'reconnecting' as const, connected: false, failures: 1, updated_at: '2026-08-15T00:00:01Z' } };
      getRuntimeMock.mockReset();
      getRuntimeMock.mockResolvedValueOnce(initialStatus);
      getRuntimeMock.mockResolvedValueOnce(refreshedStatus);
      getRuntimeMock.mockRejectedValueOnce(new Error('轮询失败'));
      // hook 是轮询失败场景的聊天 Hook 渲染结果。
      const hook = renderHook(
        // pollingHookFactory 创建轮询失败场景的 Hook。
        () => useChat(),
      );
      await act(
        // initialFlushAction 刷新初始化 Promise 和 React 状态更新。
        async () => { await Promise.resolve(); },
      );
      await act(
        // pollingTimerAction 推进首次轮询定时器。
        async () => { await vi.advanceTimersByTimeAsync(3_000); },
      );
      expect(hook.result.current.accounts[0]?.runtime_state).toBe('reconnecting');
      await act(
        // retryPollingTimerAction 推进失败后的下一次轮询定时器。
        async () => { await vi.advanceTimersByTimeAsync(3_000); },
      );
      expect(getRuntimeMock).toHaveBeenCalledTimes(3);
      hook.unmount();
    } finally {
      vi.useRealTimers();
    }
  });

  test('历史消息成功加载后调整滚动位置并处理未知实时会话', /* 当前回调验证历史分页滚动和实时未知会话刷新。 */ async () => {
    // hook 是历史分页成功场景的聊天 Hook 渲染结果。
    const hook = renderHook(
      // olderHookFactory 创建历史分页场景的 Hook。
      () => useChat(),
    );
    await waitFor(
      // activeChatAssertion 等待默认会话被选中。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await waitFor(
      // messagesAssertion 等待当前会话消息加载完成。
      () => expect(hook.result.current.messagesLoading).toBe(false),
    );
    // height 保存滚动容器当前高度，模拟历史消息插入后的高度变化。
    let height = 100;
    // container 是历史分页滚动位置测试使用的容器替身。
    const container = { clientHeight: 50, scrollTop: 0, get scrollHeight() { return height; } } as HTMLDivElement;
    hook.result.current.scrollRef.current = container;
    // olderMessage 是历史分页返回的更早消息。
    const olderMessage = { ...messageFixture, id: 0, message_key: 'message-0', sent_at: 0, content: '更早消息' };
    getMessagePageMock.mockResolvedValueOnce({ messages: [olderMessage], has_more: false, next_cursor: undefined });
    vi.stubGlobal('requestAnimationFrame', vi.fn(/* frameFactory 创建可控的滚动帧回调。 */ (callback: FrameRequestCallback) => {
      height = 180;
      callback(0);
      return 1;
    }));
    await act(
      // olderAction 请求更早的消息并恢复滚动位置。
      async () => hook.result.current.loadOlderMessages(),
    );
    expect(hook.result.current.messages).toEqual([olderMessage, messageFixture]);
    expect(container.scrollTop).toBe(80);
    await act(
      // noOlderAction 在没有更多历史消息时阻止重复分页请求。
      async () => hook.result.current.loadOlderMessages(),
    );
    expect(getMessagePageMock).toHaveBeenCalledTimes(2);

    // unknownMessage 是不在当前联系人列表中的实时消息。
    const unknownMessage = { ...messageFixture, chat_id: 'chat-unknown', message_key: 'message-unknown', content: '未知会话消息' };
    // unknownSession 是刷新接口返回的新会话，必须自动出现在联系人列表中。
    const unknownSession = { ...sessionFixture, chat_id: 'chat-unknown', buyer_id: 'buyer-unknown', last_message: '未知会话消息', last_message_at: 9 };
    getSessionPageMock.mockResolvedValueOnce({ sessions: [unknownSession, sessionFixture], has_more: false, next_cursor: undefined });
    await act(
      // unknownMessageAction 触发未知会话的联系人刷新。
      () => publishChatLiveMessage(unknownMessage),
    );
    await waitFor(
      // reloadAssertion 等待未知会话触发联系人刷新。
      () => expect(getSessionPageMock).toHaveBeenCalledWith('account-1', undefined, expect.objectContaining({ signal: expect.any(AbortSignal) }), true),
    );
    await waitFor(
      // unknownSessionAssertion 等待实时事件关联的新会话写入联系人列表。
      () => expect(hook.result.current.activeSessions).toContainEqual(unknownSession),
    );
    await act(
      // offlineAction 触发全局连接断开状态并保持消息列表稳定。
      () => publishChatConnectionState('offline'),
    );
    hook.unmount();
  });

  test('切换或清空会话时取消未完成的历史消息分页', /* 当前回调验证历史分页不会在会话上下文失效后写入新会话状态。 */ async () => {
    // historySignal 保存历史分页接口收到的取消信号。
    let historySignal: AbortSignal | undefined;
    // hook 是已加载默认账号和会话的聊天 Hook。
    const hook = renderHook(
      // chatHookFactory 创建聊天 Hook。
      () => useChat(),
    );
    await waitFor(
      // activeChatAssertion 等待默认会话成为当前选择。
      () => expect(hook.result.current.activeChatID).toBe('chat-1'),
    );
    await waitFor(
      // messagesLoadedAssertion 等待首次消息分页完成并允许继续加载历史消息。
      () => expect(hook.result.current.messagesLoading).toBe(false),
    );
    getMessagePageMock.mockImplementationOnce(
      // pendingHistory 保持历史分页未完成，以便验证会话切换时主动取消。
      (_accountID, _chatID, _cursor, _oldestID, requestOptions) => {
        historySignal = requestOptions?.signal;
        return new Promise(/* pendingHistoryExecutor 故意不完成历史分页 Promise，直到会话切换取消请求。 */ () => undefined);
      },
    );
    await act(
      // olderMessagesAction 触发需要保持未完成的历史消息分页。
      () => { void hook.result.current.loadOlderMessages(); },
    );
    await waitFor(
      // historyStartedAssertion 等待历史分页请求建立取消信号。
      () => expect(historySignal).toBeDefined(),
    );
    await act(
      // clearChatAction 清空当前会话，使历史分页上下文立即失效。
      () => hook.result.current.setActiveChatID(''),
    );
    expect(historySignal?.aborted).toBe(true);
    hook.unmount();
  });
});
