import { expect,test } from 'vitest';
import type { ChatMessage,ChatSession } from '../../../shared/api-contract/chat';
import { adjustPriceDisabledMessageReasons,collectChatReadReceipts,filterChatSessions,formatClock,groupChatMessages,isChatAbortError,isCurrentChatRequest,isRecallableChatMessage,markOutgoingMessagesReadByIncoming,mergeLiveMessage,mergeOlderMessages,messageTime,prioritizeNavigationSession,receivedRedFlowerMessageKeys,shipmentDisabledMessageReasons,sortChatSessionsByPinAndActivity,uniqueCancellableSellerOrder,unreadBadgeClassName,unreadBadgeLabel } from './state';

// sessionFixture 是覆盖搜索、未读筛选和联系人隔离的最小会话数据。
const sessionFixture: ChatSession[] = [
  { account_id: 'a1', chat_id: 'c1', buyer_id: 'b1', buyer_name: '张三', item_title: '测试商品', last_message: '你好', last_message_at: 1, unread_count: 2 },
  { account_id: 'a1', chat_id: 'c2', buyer_id: 'b2', buyer_name: '李四', item_title: '另一个商品', last_message: '已发货', last_message_at: 2, unread_count: 0 },
];

// messageFixture 是覆盖消息去重和实时替换的最小消息数据。
const messageFixture: ChatMessage = { id: 1, account_id: 'a1', chat_id: 'c1', message_key: 'm1', direction: 'incoming', sender_id: 'b1', sender_name: '张三', message_type: 'text', content: '旧消息', status: 'received', sent_at: 1 };

test('订单深链目标只提升为第一个非置顶会话', /* navigationPriorityCase 验证临时定位不污染真实置顶顺序。 */ () => {
	// pinnedOld、pinnedNew 是服务端已经确认且顺序必须保持的两个置顶会话。
	const pinnedOld: ChatSession = { ...sessionFixture[0], chat_id: 'pinned-old', is_pinned: true };
	const pinnedNew: ChatSession = { ...sessionFixture[1], chat_id: 'pinned-new', is_pinned: true };
	// normalRecent 是原本排在最前的普通会话。
	const normalRecent: ChatSession = { ...sessionFixture[0], chat_id: 'normal-recent', last_message_at: 300, is_pinned: false };
	// target 是订单深链需要临时提升的较早普通会话。
	const target: ChatSession = { ...sessionFixture[0], chat_id: 'order-target', last_message_at: 100, is_pinned: false };
	// normalOld 是原本排在目标之后的普通会话。
	const normalOld: ChatSession = { ...sessionFixture[1], chat_id: 'normal-old', last_message_at: 50, is_pinned: false };
	// prioritized 是应用订单深链临时提升后的可见顺序。
	const prioritized = prioritizeNavigationSession([pinnedOld, pinnedNew, normalRecent, target, normalOld], 'order-target');
	expect(prioritized.map(/* session 是当前读取排序结果标识的会话。 */ session => session.chat_id)).toEqual(['pinned-old', 'pinned-new', 'order-target', 'normal-recent', 'normal-old']);
	expect(prioritizeNavigationSession(prioritized, '')).toBe(prioritized);
});

test('Chat 已读回执排除系统消息和平台内部消息',
  // 已读回执测试确保接口仅接收可确认的普通入站消息。
  () => {
    // messages 覆盖普通消息、系统通知、内部标记及出站消息。
    const messages: ChatMessage[] = [
      messageFixture,
      { ...messageFixture, id: 2, message_key: 'in-local', message_type: 'text' },
      { ...messageFixture, id: 3, message_key: 'system-1', message_type: 'system' },
      { ...messageFixture, id: 4, message_key: 'outgoing-1', direction: 'outgoing' },
    ];
    expect(collectChatReadReceipts(messages, 'c1')).toEqual([
      { messageId: 'm1', sessionId: 'c1', cid: 'c1@goofish', conversationType: 1 },
    ]);
  });

test('Chat 会话筛选和历史消息合并保持账号内顺序',
  // 会话状态测试验证搜索、未读筛选和历史消息去重语义。
  () => {
    expect(filterChatSessions(sessionFixture, '张三', false)).toHaveLength(1);
    expect(filterChatSessions(sessionFixture, '', true)).toEqual([sessionFixture[0]]);
    expect(mergeOlderMessages([messageFixture], [{ ...messageFixture, id: 0, message_key: 'm0', content: '更早' }])).toHaveLength(2);
  });

test('Chat 会话排序保持置顶组并只重排普通会话',
  // 排序测试验证实时消息不改写服务端手工置顶顺序。
  () => {
    // pinnedFirst、pinnedSecond 按服务端 pinned_at 顺序预置，即使消息时间相反也不得互换。
    const pinnedFirst = { ...sessionFixture[0], chat_id: 'pinned-first', last_message_at: 1, is_pinned: true };
    const pinnedSecond = { ...sessionFixture[1], chat_id: 'pinned-second', last_message_at: 999, is_pinned: true };
    // normalOlder 是消息时间较早的普通会话。
    const normalOlder = { ...sessionFixture[0], chat_id: 'normal-older', last_message_at: 10, is_pinned: false };
    // normalNewer 是应在普通组中按最近消息优先的会话。
    const normalNewer = { ...sessionFixture[1], chat_id: 'normal-newer', last_message_at: 20, is_pinned: false };
    expect(sortChatSessionsByPinAndActivity([normalOlder, pinnedFirst, normalNewer, pinnedSecond]).map(/* session 是当前读取排序结果标识的会话。 */ session => session.chat_id)).toEqual([
      'pinned-first', 'pinned-second', 'normal-newer', 'normal-older',
    ]);
  });
test('Chat 实时消息替换同键记录并拒绝过期请求',
  // 请求边界测试验证实时回执不会产生重复消息，切换会话后的旧请求不能写入。
  () => {
    // controller 请求取消控制器。
    const controller = new AbortController();
    expect(mergeLiveMessage([messageFixture], { ...messageFixture, content: '新消息' })[0].content).toBe('新消息');
    expect(isCurrentChatRequest(3, 3, controller.signal)).toBe(true);
    expect(isCurrentChatRequest(2, 3, controller.signal)).toBe(false);
    controller.abort();
    expect(isCurrentChatRequest(3, 3, controller.signal)).toBe(false);
  });

test('Chat 迟到的不完整发送事件不能擦除 HTTP 已返回的平台标识和会话归属',
  // 同键单调合并保证刚发送的消息立即可回复和撤回。
  () => {
		// complete 是 HTTP 发送成功响应中已绑定 PNM 和完整会话归属的己方消息。
		const complete: ChatMessage = { ...messageFixture, id: 20, account_id: 'account-1', chat_id: 'chat-1', message_key: 'local-race', platform_message_id: 'platform-race.PNM', direction: 'outgoing', sender_id: 'account-1', sender_name: '我', status: 'sent', read_status: 2, read_at: 200, sent_at: 1_800_000_000_000 };
		// delayed 是 Engine 旁路晚到且缺少 PNM、已读和会话字段的同键状态事件。
		const delayed: ChatMessage = { ...complete, id: 0, account_id: '', chat_id: '', platform_message_id: '', status: 'sending', read_status: 0, read_at: 0 };
		// merged 是应保留 HTTP 权威字段和更高状态的实时合并结果。
		const merged = mergeLiveMessage([complete], delayed)[0];
		expect(merged).toMatchObject({ id: 20, account_id: 'account-1', chat_id: 'chat-1', platform_message_id: 'platform-race.PNM', status: 'sent', read_status: 2, read_at: 200 });
		expect(isRecallableChatMessage(merged, 1_800_000_001_000)).toBe(true);
  });

test('Chat 后续入站消息确认此前已发送出站消息为已读',
  // 已读推导测试验证实时状态与数据库事务中的会话级确认语义一致。
  () => {
    // outgoing 是尚未收到已读回执的本地出站消息。
    const outgoing: ChatMessage = { ...messageFixture, direction: 'outgoing', status: 'sent', read_status: 0, read_at: 0, sent_at: 10, message_key: 'outgoing-1' };
    // incoming 是买家随后发来的普通消息。
    const incoming: ChatMessage = { ...messageFixture, direction: 'incoming', sent_at: 20, message_key: 'incoming-1' };
    // updated 保存后续入站消息确认后的出站消息状态。
    const updated = markOutgoingMessagesReadByIncoming([outgoing], incoming);
    expect(updated[0]).toMatchObject({ read_status: 2, read_at: 20 });
  });

test('Chat 状态工具覆盖追加消息、搜索字段和时间格式化',
  // 边界场景测试验证新消息追加、不同搜索字段和取消错误识别。
  () => {
    expect(filterChatSessions(sessionFixture, 'B2', false)).toEqual([sessionFixture[1]]);
    expect(filterChatSessions(sessionFixture, '已发货', false)).toEqual([sessionFixture[1]]);
    expect(filterChatSessions(sessionFixture, '不存在', false)).toEqual([]);
    // incoming 是没有出现在当前列表中的实时消息。
    const incoming = { ...messageFixture, message_key: 'm2', content: '新消息' };
    expect(mergeLiveMessage([messageFixture], incoming)).toEqual([messageFixture, incoming]);
    expect(isChatAbortError(new Error('请求已取消'))).toBe(true);
    expect(isChatAbortError(new Error('网络失败'))).toBe(false);
    expect(isChatAbortError('请求已取消')).toBe(false);
    expect(formatClock(0)).toBe('');
    expect(formatClock(Date.now())).toMatch(/^\d{2}:\d{2}$/);
    expect(formatClock(Date.now() - 86_400_000)).toMatch(/\d{2}\/\d{2}/);
    expect(messageTime(Date.now())).toMatch(/\d{2}\/\d{2}/);
  });

test('Chat 未读徽标为单数字保留正圆尺寸，多数字才横向扩展',
  // 未读徽标测试验证账号页签和会话列表共享的形状边界。
  () => {
    expect(unreadBadgeLabel(1)).toBe('1');
    expect(unreadBadgeLabel(99)).toBe('99');
    expect(unreadBadgeLabel(100)).toBe('99+');
    expect(unreadBadgeClassName(1)).toContain('w-5');
    expect(unreadBadgeClassName(1)).not.toContain('min-w-5');
    expect(unreadBadgeClassName(10)).toContain('min-w-5');
    expect(unreadBadgeClassName(99)).toContain('min-w-5');
  });

test('Chat 消息分组只合并两分钟内同一发送方的连续普通消息',
  // 消息分组测试验证长会话压缩不会吞并系统事件、失败状态或不同发送方边界。
  () => {
    // messages 覆盖秒级时间、毫秒级时间、系统事件、失败状态和不同发送方边界。
    const messages: ChatMessage[] = [
      { ...messageFixture, message_key: 'incoming-1', sent_at: 100 },
      { ...messageFixture, message_key: 'incoming-2', sent_at: 160 },
      { ...messageFixture, message_key: 'system-1', message_type: 'system', sent_at: 170 },
      { ...messageFixture, message_key: 'outgoing-failed', direction: 'outgoing', sender_id: 'seller', status: 'failed', sent_at: 180 },
      { ...messageFixture, message_key: 'outgoing-1', direction: 'outgoing', sender_id: 'seller', status: 'sent', sent_at: 190 },
      { ...messageFixture, message_key: 'outgoing-2', direction: 'outgoing', sender_id: 'seller', status: 'sent', sent_at: 310 },
      { ...messageFixture, message_key: 'millisecond-1', sender_id: 'other', sent_at: 1_800_000_000_000 },
      { ...messageFixture, message_key: 'millisecond-2', sender_id: 'other', sent_at: 1_800_000_060_000 },
    ];
    expect(groupChatMessages(messages).map(/* currentGroup 将每个分组转换为稳定的消息键断言。 */ currentGroup => currentGroup.map(/* currentMessage 提取当前分组内消息键。 */ currentMessage => currentMessage.message_key))).toEqual([
      ['incoming-1', 'incoming-2'],
      ['system-1'],
      ['outgoing-failed'],
      ['outgoing-1', 'outgoing-2'],
      ['millisecond-1', 'millisecond-2'],
    ]);
  });

test('Chat 撤回入口只允许两分钟内已绑定平台 ID 的己方文本或图片',
  // 撤回可见性测试覆盖方向、平台 ID、时间和撤回状态边界。
  () => {
    // now 是本组断言使用的固定浏览器时间。
    const now = 1_800_000_120_000;
    // candidate 是恰好位于两分钟边界的己方文本消息。
    const candidate = { ...messageFixture, direction: 'outgoing', status: 'sent', platform_message_id: 'platform-1.PNM', sent_at: 1_800_000_000_000 } as ChatMessage;
    expect(isRecallableChatMessage(candidate, now)).toBe(true);
    expect(isRecallableChatMessage({ ...candidate, platform_message_id: '' }, now)).toBe(false);
    expect(isRecallableChatMessage({ ...candidate, direction: 'incoming' }, now)).toBe(false);
	expect(isRecallableChatMessage({ ...candidate, message_type: 'location' }, now)).toBe(true);
    expect(isRecallableChatMessage({ ...candidate, status: 'recalled' }, now)).toBe(false);
    expect(isRecallableChatMessage({ ...candidate, sent_at: 1_799_999_999_999 }, now)).toBe(false);
  });

test('Chat 收花结果只完成最近一条尚未收取的送花卡片',
  // 小红花状态测试验证历史和实时消息都能持久禁用已经完成的收花入口。
  () => {
    // messages 包含两次送花和一次收花，后一次送花应先被完成。
    const messages: ChatMessage[] = [
      { ...messageFixture, message_key: 'flower-sent-1', message_type: 'system', system_card: { kind: 'trade', event: 'red_flower_sent', title: '送你小红花', order_id: '1000000000000000001', action: 'receive_red_flower' } },
      { ...messageFixture, message_key: 'flower-sent-2', message_type: 'system', system_card: { kind: 'trade', event: 'red_flower_sent', title: '送你小红花', order_id: '1000000000000000002', action: 'receive_red_flower' } },
      { ...messageFixture, message_key: 'flower-received', message_type: 'system', system_card: { kind: 'trade', event: 'red_flower_received', title: '已收到小红花' } },
    ];
    expect([...receivedRedFlowerMessageKeys(messages)]).toEqual(['flower-sent-2']);
  });

test('Chat 同订单付款终态会禁用原待付款改价卡片',
  // 改价资格测试验证改价结果不结束待付款，但后续付款会给原按钮提供明确禁用原因。
  () => {
    // messages 覆盖待付款、仍可继续议价的改价结果、另一订单付款和目标订单付款。
    const messages: ChatMessage[] = [
      { ...messageFixture, message_key: 'pending-target', message_type: 'system', sent_at: 100, system_card: { kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款', order_id: 'order-1', action: 'adjust_price' } },
      { ...messageFixture, message_key: 'adjusted-target', message_type: 'system', sent_at: 110, system_card: { kind: 'trade', event: 'order_price_adjusted', title: '我已修改价格，等待你付款', order_id: 'order-1' } },
      { ...messageFixture, message_key: 'paid-other', message_type: 'system', sent_at: 120, system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: 'order-2' } },
      { ...messageFixture, message_key: 'paid-target', message_type: 'system', sent_at: 130, system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: 'order-1' } },
    ];
    // disabledReasons 是消息键到用户可见禁用原因的派生映射。
    const disabledReasons = adjustPriceDisabledMessageReasons(messages);
    expect([...disabledReasons.entries()]).toEqual([['pending-target', '买家已付款，不能再修改价格']]);
  });

test('Chat 同订单后续发货会禁用原付款卡片',
	// 发货资格测试确保较早或其他订单终态不会误伤当前付款入口。
	() => {
		// paid 是卖家账号收到的可发货付款卡片。
		const paid: ChatMessage = { ...messageFixture, account_id: 'seller', sender_id: 'buyer', message_key: 'paid-target', message_type: 'system', sent_at: 100,
			system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: '5127372002248048713', action: 'ship_order' } };
		// earlierShipped 是早于付款卡片的历史发货状态，不应禁用新订单入口。
		const earlierShipped: ChatMessage = { ...paid, message_key: 'shipped-earlier', sent_at: 90, system_card: { kind: 'trade', event: 'order_shipped', title: '你已发货', order_id: '5127372002248048713' } };
		// otherShipped 是另一订单的后续发货状态，不应影响目标订单。
		const otherShipped: ChatMessage = { ...paid, message_key: 'shipped-other', sent_at: 120, system_card: { kind: 'trade', event: 'order_shipped', title: '你已发货', order_id: '5127000000000000001' } };
		// targetShipped 是目标订单在付款后的明确发货状态。
		const targetShipped: ChatMessage = { ...paid, message_key: 'shipped-target', sent_at: 130, system_card: { kind: 'trade', event: 'order_shipped', title: '你已发货', order_id: '5127372002248048713' } };
		// falseReminder 复现历史错误行：事件被旧版写成 order_shipped，但标题和说明都明确仍在提醒发货。
		const falseReminder: ChatMessage = { ...paid, message_key: 'shipment-reminder', sent_at: 140, system_card: { kind: 'trade', event: 'order_shipped', title: '记得及时发货', description: '如已发货，请点击「去发货」输入快递单号', order_id: '5127372002248048713' } };
		expect(shipmentDisabledMessageReasons([earlierShipped, paid, otherShipped]).size).toBe(0);
		expect(shipmentDisabledMessageReasons([paid, targetShipped]).get('paid-target')).toBe('订单已经发货，不能重复提交');
		expect(shipmentDisabledMessageReasons([paid, falseReminder]).size).toBe(0);
	});

test('Chat 历史发货提醒不会覆盖真实付款状态或终止待付款派生',
	// 历史防御测试确保已落库的错误 order_shipped 提醒不再伪造终态。
	() => {
		// pending 是付款前的改价卡片。
		const pending: ChatMessage = { ...messageFixture, account_id: 'seller', sender_id: 'buyer', message_key: 'pending-reminder', message_type: 'system', sent_at: 100,
			system_card: { kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款', order_id: '5127372002248048713', action: 'adjust_price' } };
		// paid 是同订单真实付款卡片，应成为改价终止原因但不能终止发货入口。
		const paid: ChatMessage = { ...pending, message_key: 'paid-reminder', sent_at: 110,
			system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: '5127372002248048713', action: 'ship_order' } };
		// falseReminder 是旧分类器错误保存的正式提醒形态。
		const falseReminder: ChatMessage = { ...pending, message_key: 'false-shipped-reminder', sent_at: 120,
			system_card: { kind: 'trade', event: 'order_shipped', title: '记得及时发货', description: '如已发货，请点击「去发货」输入快递单号', order_id: '5127372002248048713' } };
		expect(adjustPriceDisabledMessageReasons([pending, paid, falseReminder]).get('pending-reminder')).toBe('买家已付款，不能再修改价格');
		expect(shipmentDisabledMessageReasons([paid, falseReminder]).size).toBe(0);
		expect(uniqueCancellableSellerOrder([pending, falseReminder])).toEqual({ accountID: 'seller', orderID: '5127372002248048713', messageKey: 'pending-reminder', stage: 'pending_payment' });
		expect(uniqueCancellableSellerOrder([pending, paid, falseReminder])).toEqual({ accountID: 'seller', orderID: '5127372002248048713', messageKey: 'paid-reminder', stage: 'pending_ship' });
	});

test('Chat 顶部取消订单接受唯一卖家待付款或已付款待发货订单',
  // 会话订单测试验证买家广播、付款阶段延续和同会话多订单都不会被错误选择。
  () => {
    // sellerPending 是卖家账号收到的第一笔未付款订单卡片。
    const sellerPending: ChatMessage = { ...messageFixture, account_id: 'seller', sender_id: 'buyer', message_key: 'pending-1', message_type: 'system', sent_at: 100,
      system_card: { kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款', order_id: '5127638256187075541', action: 'adjust_price' } };
    expect(uniqueCancellableSellerOrder([sellerPending])).toEqual({ accountID: 'seller', orderID: '5127638256187075541', messageKey: 'pending-1', stage: 'pending_payment' });
    // buyerBroadcast 是同订单广播到买家自身账号的卡片，不能生成卖家操作。
    const buyerBroadcast: ChatMessage = { ...sellerPending, account_id: 'buyer', sender_id: 'buyer', message_key: 'pending-buyer' };
    expect(uniqueCancellableSellerOrder([buyerBroadcast])).toBeUndefined();
    // paid 是第一笔订单之后到达的付款卡片，应覆盖旧待付款证据并保持取消入口。
    const paid: ChatMessage = { ...sellerPending, message_key: 'paid-1', sent_at: 200, system_card: { kind: 'trade', event: 'order_paid', title: '我已付款，等待你发货', order_id: '5127638256187075541' } };
		expect(uniqueCancellableSellerOrder([sellerPending, paid])).toEqual({ accountID: 'seller', orderID: '5127638256187075541', messageKey: 'paid-1', stage: 'pending_ship' });
    // secondPending 证明同一会话出现两笔订单时必须拒绝猜测，不能依赖“新订单必开新会话”。
    const secondPending: ChatMessage = { ...sellerPending, message_key: 'pending-2', sent_at: 300, system_card: { ...sellerPending.system_card!, order_id: '5127694777172175924' } };
    expect(uniqueCancellableSellerOrder([sellerPending, secondPending])).toBeUndefined();
    // plainClosed 是平台不携带订单号的卖家关闭系统文本；关闭时唯一的旧候选可被安全淘汰。
    const plainClosed: ChatMessage = { ...messageFixture, account_id: 'seller', sender_id: 'buyer', message_key: 'closed-plain', message_type: 'system', content: '[未付款，你关闭了订单]', sent_at: 200 };
    expect(uniqueCancellableSellerOrder([sellerPending, plainClosed, secondPending])).toEqual({ accountID: 'seller', orderID: '5127694777172175924', messageKey: 'pending-2', stage: 'pending_payment' });
    // ambiguousPending 是关闭文本之前的第二笔候选；多笔时不能把无订单号终态归给任意一笔。
    const ambiguousPending: ChatMessage = { ...sellerPending, message_key: 'pending-ambiguous', sent_at: 150, system_card: { ...sellerPending.system_card!, order_id: '5127195711163048713' } };
    expect(uniqueCancellableSellerOrder([sellerPending, ambiguousPending, plainClosed, secondPending])).toBeUndefined();
  });

test('发货、收货、完成、退款和取消事件都会终止统一取消操作',
  // 新生命周期事件测试验证拆分状态后仍保持改价和取消的安全门禁。
  () => {
    // pending 是当前卖家待付款卡片。
    const pending: ChatMessage = { ...messageFixture, account_id: 'seller', sender_id: 'buyer', message_key: 'pending-life', message_type: 'system', sent_at: 100,
      system_card: { kind: 'trade', event: 'order_pending_payment', title: '我已拍下，待付款', order_id: '5127000000000000001', action: 'adjust_price' } };
    for (const terminalEvent /* terminalEvent 是当前待验证的订单生命周期终态。 */ of ['order_shipped', 'order_received', 'order_completed', 'refund_requested', 'refund_completed', 'order_cancelled'] as const) {
      // terminal 是同订单后续到达的结构化状态卡片。
      const terminal: ChatMessage = { ...pending, message_key: `terminal-${terminalEvent}`, sent_at: 200, system_card: { kind: 'trade', event: terminalEvent, title: terminalEvent, order_id: '5127000000000000001' } };
		expect(uniqueCancellableSellerOrder([pending, terminal])).toBeUndefined();
      expect(adjustPriceDisabledMessageReasons([pending, terminal]).get('pending-life')).toBeTruthy();
    }
  });
