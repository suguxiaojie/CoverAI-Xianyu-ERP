import type { ChatMessage,ChatSession } from './api';
import type { ChatReadReceipt } from './types';

/** 将未读数规范为徽标可展示的文本，超过两位数时统一显示 99+。 */
export const unreadBadgeLabel = (count: number): string => {
  // normalized 保存排除 NaN、负数与小数后的未读数量。
  const normalized = Number.isFinite(count) ? Math.max(0, Math.floor(count)) : 0;
  return normalized > 99 ? '99+' : String(normalized);
};

/** 根据未读文本长度返回稳定徽标尺寸，单数字必须为正圆，双数字及 99+ 才可横向扩展。 */
export const unreadBadgeClassName = (count: number): string => {
  // normalized 保存用于区分单数字与多数字徽标的规范未读数量。
  const normalized = Number.isFinite(count) ? Math.max(0, Math.floor(count)) : 0;
  return normalized < 10
    ? 'inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-red-500 text-[10px] font-bold leading-none text-white'
    : 'inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-red-500 px-1.5 text-[10px] font-bold leading-none text-white';
};

/** 把平台秒级或前端毫秒级聊天时间统一换算为毫秒，供消息分组比较真实时间间隔。 */
const chatTimestampMilliseconds = (value: number): number => value < 10_000_000_000 ? value * 1000 : value;

/** 把订单深链目标临时放到所有真实置顶会话之后、其他非置顶会话之前，不改变服务端置顶事实。 */
export const prioritizeNavigationSession = (sessions: ChatSession[], targetChatID: string): ChatSession[] => {
	// normalizedTarget 是去除空白后的订单深链会话标识。
	const normalizedTarget = targetChatID.trim();
	if (!normalizedTarget) return sessions;
	// target 是当前列表中的目标会话；已经真实置顶时保持原有服务端顺序。
	const target = sessions.find(/* session 是当前匹配订单深链目标的会话。 */ session => session.chat_id === normalizedTarget);
	if (!target || target.is_pinned === true) return sessions;
	// pinnedSessions 保留全部真实置顶会话及其既有顺序。
	const pinnedSessions = sessions.filter(/* session 是当前判断真实置顶状态的会话。 */ session => session.is_pinned === true);
	// remainingSessions 保留目标之外的全部非置顶会话及其既有时间顺序。
	const remainingSessions = sessions.filter(/* session 是当前保留的普通会话。 */ session => session.is_pinned !== true && session.chat_id !== normalizedTarget);
	return [...pinnedSessions, target, ...remainingSessions];
};

// adjustPriceTerminalReasons 把不再允许改价的交易终态映射为卡片旁的明确禁用原因。
const adjustPriceTerminalReasons: Record<string, string> = {
  order_paid: '买家已付款，不能再修改价格',
  order_closed: '订单已关闭，不能再修改价格',
	order_cancelled: '订单已取消，不能再修改价格',
  order_shipped: '订单已发货，不能再修改价格',
	order_received: '买家已确认收货，不能再修改价格',
  order_completed: '交易已完成，不能再修改价格',
  refund_requested: '订单已进入退款流程，不能再修改价格',
	refund_completed: '订单已退款，不能再修改价格',
};

// shipmentTerminalReasons 把不再允许发货的后续状态映射为付款卡片旁的明确禁用原因。
const shipmentTerminalReasons: Record<string, string> = {
	order_shipped: '订单已经发货，不能重复提交',
	order_received: '买家已经确认收货，不能再发货',
	order_completed: '交易已经完成，不能再发货',
	order_closed: '订单已经关闭，不能再发货',
	order_cancelled: '订单已经取消，不能再发货',
	refund_requested: '订单已进入退款流程，不能再发货',
	refund_completed: '订单已经退款，不能再发货',
};

/** isShipmentReminderCard 识别旧版本误存为 order_shipped 的“仍待发货”条件提醒。 */
const isShipmentReminderCard = (message: ChatMessage): boolean => {
	// card 是当前消息可能携带的结构化交易卡片。
	const card = message.system_card;
	if (card?.event !== 'order_shipped') return false;
	// title 是旧正式卡片的提醒标题。
	const title = String(card.title || '').trim();
	// description 是旧正式卡片中包含“如已发货”的条件说明。
	const description = String(card.description || '').trim();
	return title.includes('记得及时发货') || description.includes('如已发货') && description.includes('去发货');
};

// AdjustPriceTerminalState 保存同订单当前已知的最新不可改价状态及其平台时间。
interface AdjustPriceTerminalState {
  // sentAtMilliseconds 是终态卡片统一后的毫秒时间，用于拒绝比待付款卡片更早的无关状态。
  sentAtMilliseconds: number;
  // reason 是展示给卖家的不可改价原因，不包含平台原始载荷。
  reason: string;
}

// ShipmentTerminalState 保存同订单当前已知的最新不可发货状态及平台时间。
interface ShipmentTerminalState {
	// sentAtMilliseconds 是终态卡片统一后的毫秒时间。
	sentAtMilliseconds: number;
	// reason 是付款卡片旁展示的禁用原因。
	reason: string;
}

// CancellableSellerOrderAction 是当前会话唯一可执行卖家取消操作的精确订单上下文。
export interface CancellableSellerOrderAction {
  // accountID 是拥有卖家卡片的当前账号。
  accountID: string;
  // orderID 是待取消平台订单号。
  orderID: string;
  // messageKey 是提供卖家角色证据的最新待付款或付款卡片键。
  messageKey: string;
	// stage 区分待付款和已付款待发货，用于二次确认资金提示。
	stage: 'pending_payment' | 'pending_ship';
}

/**
 * 将同一发送方在两分钟内连续发送的普通消息合并为视觉消息组。
 * 系统事件和失败消息保持独立，避免交易状态或失败操作被相邻气泡掩盖。
 */
export const groupChatMessages = (messages: ChatMessage[]): ChatMessage[][] => {
  // groups 按原始时间顺序保存可独立展示的消息组，不改变消息对象本身。
  const groups: ChatMessage[][] = [];
  messages.forEach(/* currentMessage 是当前按时间顺序判断归组的聊天消息。 */ currentMessage => {
    // previousGroup 是已经生成的最后一组；不存在时必须新建首组。
    const previousGroup = groups.at(-1);
    // previousMessage 是上一组最后一条消息，用于判断发送方、状态和时间连续性。
    const previousMessage = previousGroup?.at(-1);
    // timeGapMilliseconds 是当前消息与上一条消息的真实时间差，允许平台秒级和前端毫秒级时间混用。
    const timeGapMilliseconds = previousMessage
      ? chatTimestampMilliseconds(currentMessage.sent_at) - chatTimestampMilliseconds(previousMessage.sent_at)
      : Number.POSITIVE_INFINITY;
    // canJoinPreviousGroup 仅允许普通、成功、同方向且时间连续的消息复用同一头像和名称。
    const canJoinPreviousGroup = Boolean(
      previousGroup
      && previousMessage
      && previousMessage.message_type !== 'system'
      && currentMessage.message_type !== 'system'
      && previousMessage.status !== 'failed'
      && currentMessage.status !== 'failed'
		&& previousMessage.status !== 'recalled'
		&& currentMessage.status !== 'recalled'
		&& previousMessage.status === currentMessage.status
      && previousMessage.direction === currentMessage.direction
      && previousMessage.sender_id === currentMessage.sender_id
      && timeGapMilliseconds >= 0
      && timeGapMilliseconds <= 120_000,
    );
    if (canJoinPreviousGroup) {
      previousGroup?.push(currentMessage);
      return;
    }
    groups.push([currentMessage]);
  });
  return groups;
};

/** 按同会话时间顺序把收花结果匹配到最近一条尚未完成的送花卡片。 */
export const receivedRedFlowerMessageKeys = (messages: ChatMessage[]): Set<string> => {
  // pendingSentKeys 保存尚未出现后续收花结果的送花卡片键，后进先出匹配最近交易。
  const pendingSentKeys: string[] = [];
  // receivedKeys 保存已经由后续 red_flower_received 确认完成的送花卡片键。
  const receivedKeys = new Set<string>();
  messages.forEach(/* currentMessage 是当前按时间顺序检查的小红花消息。 */ currentMessage => {
    // event 是当前结构化系统卡片事件；普通消息为空。
    const event = currentMessage.system_card?.event;
    if (event === 'red_flower_sent') {
      pendingSentKeys.push(currentMessage.message_key);
      return;
    }
    if (event !== 'red_flower_received') return;
    // matchedKey 是当前收花结果对应的最近一条待完成送花卡片。
    const matchedKey = pendingSentKeys.pop();
    if (matchedKey) receivedKeys.add(matchedKey);
  });
  return receivedKeys;
};

/** 按精确订单号把后续付款、关闭、发货、完成或退款终态映射到原待付款卡片，使历史改价按钮保持可见但不可点击。 */
export const adjustPriceDisabledMessageReasons = (messages: ChatMessage[]): Map<string, string> => {
  // terminalByOrderID 保存每个订单在当前已加载消息中的最新不可改价状态。
  const terminalByOrderID = new Map<string, AdjustPriceTerminalState>();
  messages.forEach(/* currentMessage 是当前检查终态和订单关联的结构化消息。 */ currentMessage => {
    // currentCard 是当前消息携带的交易卡片；普通消息不会参与改价资格派生。
    const currentCard = currentMessage.system_card;
    // orderID 是去空白后的订单号；不同文本形态不会被模糊归一，避免错误关联其他订单。
    const orderID = String(currentCard?.order_id || '').trim();
    // reason 是当前事件对应的终态禁用原因；改价结果本身不结束待付款状态。
		const reason = isShipmentReminderCard(currentMessage) ? '' : adjustPriceTerminalReasons[currentCard?.event || ''];
    if (!orderID || !reason) return;
    // sentAtMilliseconds 是当前终态统一后的平台时间。
    const sentAtMilliseconds = chatTimestampMilliseconds(currentMessage.sent_at);
    // previousTerminal 是同订单此前已知的终态；乱序消息只允许更新为更晚状态。
    const previousTerminal = terminalByOrderID.get(orderID);
    if (!previousTerminal || sentAtMilliseconds >= previousTerminal.sentAtMilliseconds) {
      terminalByOrderID.set(orderID, { sentAtMilliseconds, reason });
    }
  });
  // disabledReasons 保存待付款消息键到禁用文案的映射，供卡片保持原位置展示灰色按钮。
  const disabledReasons = new Map<string, string>();
  messages.forEach(/* currentMessage 是当前检查是否被后续终态覆盖的待付款卡片。 */ currentMessage => {
    // currentCard 是可能提供 adjust_price 动作的结构化交易卡片。
    const currentCard = currentMessage.system_card;
    if (currentCard?.event !== 'order_pending_payment' || currentCard.action !== 'adjust_price') return;
    // orderID 是待付款卡片携带的精确订单号。
    const orderID = String(currentCard.order_id || '').trim();
    // terminal 是同订单当前已知的最新终态。
    const terminal = terminalByOrderID.get(orderID);
    if (!terminal || terminal.sentAtMilliseconds < chatTimestampMilliseconds(currentMessage.sent_at)) return;
    disabledReasons.set(currentMessage.message_key, terminal.reason);
  });
  return disabledReasons;
};

/** 按精确订单号把后续发货、完成、取消或退款状态映射到原付款卡片，防止历史卡片重复发货。 */
export const shipmentDisabledMessageReasons = (messages: ChatMessage[]): Map<string, string> => {
	// terminalByOrderID 保存每个订单当前已加载的最新不可发货状态。
	const terminalByOrderID = new Map<string, ShipmentTerminalState>();
	messages.forEach(/* currentMessage 是当前检查后续订单状态的结构化消息。 */ currentMessage => {
		// currentCard 是可能携带订单终态的交易卡片。
		const currentCard = currentMessage.system_card;
		// orderID 是去空白后的精确订单号。
		const orderID = String(currentCard?.order_id || '').trim();
		// reason 是当前事件对应的不可发货原因。
		const reason = isShipmentReminderCard(currentMessage) ? '' : shipmentTerminalReasons[currentCard?.event || ''];
		if (!orderID || !reason) return;
		// sentAtMilliseconds 是当前终态统一后的平台毫秒时间。
		const sentAtMilliseconds = chatTimestampMilliseconds(currentMessage.sent_at);
		// previousTerminal 是同订单此前记录的状态，乱序消息不能覆盖更晚终态。
		const previousTerminal = terminalByOrderID.get(orderID);
		if (!previousTerminal || sentAtMilliseconds >= previousTerminal.sentAtMilliseconds) terminalByOrderID.set(orderID, { sentAtMilliseconds, reason });
	});
	// disabledReasons 保存付款消息键到禁用原因的映射。
	const disabledReasons = new Map<string, string>();
	messages.forEach(/* currentMessage 是当前检查是否被后续终态覆盖的付款卡片。 */ currentMessage => {
		// currentCard 是可能提供 ship_order 动作的付款卡片。
		const currentCard = currentMessage.system_card;
		if (currentCard?.event !== 'order_paid' || currentCard.action !== 'ship_order') return;
		// orderID 是付款卡片携带的精确订单号。
		const orderID = String(currentCard.order_id || '').trim();
		// terminal 是该订单当前已知最新终态。
		const terminal = terminalByOrderID.get(orderID);
		if (!terminal || terminal.sentAtMilliseconds < chatTimestampMilliseconds(currentMessage.sent_at)) return;
		disabledReasons.set(currentMessage.message_key, terminal.reason);
	});
	return disabledReasons;
};

/** 从当前会话返回唯一一笔待付款或已付款待发货卖家订单；零笔或多笔都返回空，绝不猜订单。 */
export const uniqueCancellableSellerOrder = (messages: ChatMessage[]): CancellableSellerOrderAction | undefined => {
  // terminalEvents 是出现后会终止普通取消资格的发货、关闭、退款和完成事件。
  const terminalEvents = new Set(['order_closed', 'order_cancelled', 'order_shipped', 'order_received', 'order_completed', 'refund_requested', 'refund_completed']);
  // candidates 按精确订单号保存当前会话的卖家取消证据，付款卡片会覆盖同订单更早待付款卡片。
	const candidates = new Map<string, {/** message 是卖家最新待付款或付款卡片。 */ message: ChatMessage; /** sentAt 是统一后的平台毫秒时间。 */ sentAt: number}>();
  messages.forEach(/* currentMessage 检查当前结构化卡片是否提供卖家取消证据。 */ currentMessage => {
    // currentCard 是当前消息的结构化交易卡片。
    const currentCard = currentMessage.system_card;
    // orderID 是去空白后的精确订单号。
    const orderID = String(currentCard?.order_id || '').trim();
    if ((currentCard?.event !== 'order_pending_payment' && currentCard?.event !== 'order_paid') || !/^\d{10,30}$/.test(orderID) || !currentMessage.account_id || currentMessage.sender_id === currentMessage.account_id) return;
		if (currentCard.event === 'order_paid' && currentCard.action && currentCard.action !== 'ship_order') return;
    // sentAt 是当前取消候选卡片统一后的平台时间。
    const sentAt = chatTimestampMilliseconds(currentMessage.sent_at);
    // previous 是同订单此前已记录的待付款证据。
    const previous = candidates.get(orderID);
    if (!previous || sentAt >= previous.sentAt) candidates.set(orderID, { message: currentMessage, sentAt });
  });
  messages.forEach(/* currentMessage 使用后续终态删除同订单取消候选。 */ currentMessage => {
    // currentCard 是可能终止订单资格的结构化卡片。
    const currentCard = currentMessage.system_card;
    // orderID 是当前终态关联的精确订单号。
    const orderID = String(currentCard?.order_id || '').trim();
    // candidate 是该订单当前保存的取消证据。
    const candidate = candidates.get(orderID);
		if (!candidate || isShipmentReminderCard(currentMessage) || !terminalEvents.has(currentCard?.event || '') || chatTimestampMilliseconds(currentMessage.sent_at) <= candidate.sentAt) return;
    candidates.delete(orderID);
  });
  messages.forEach(/* currentMessage 兼容平台未携带订单号的卖家关闭系统文本。 */ currentMessage => {
    if (currentMessage.message_type !== 'system' || currentMessage.content.trim() !== '[未付款，你关闭了订单]') return;
    // closeAt 是平台关闭系统文本的统一毫秒时间，用于排除关闭后新拍下的订单。
    const closeAt = chatTimestampMilliseconds(currentMessage.sent_at);
    // precedingOrderIDs 是关闭发生前仍在候选集合中的订单；只有唯一时才能安全关联。
    const precedingOrderIDs = [...candidates.entries()]
      .filter(/* candidateEntry 包含订单号及其最新待付款证据。 */ candidateEntry => candidateEntry[1].sentAt < closeAt)
      .map(/* candidateEntry 转换为可删除的精确订单号。 */ candidateEntry => candidateEntry[0]);
    if (precedingOrderIDs.length === 1) candidates.delete(precedingOrderIDs[0]);
  });
  if (candidates.size !== 1) return undefined;
  // candidate 是当前会话唯一剩余的卖家可取消订单。
  const candidate = [...candidates.values()][0].message;
  return { accountID: candidate.account_id, orderID: String(candidate.system_card?.order_id || '').trim(), messageKey: candidate.message_key,
		stage: candidate.system_card?.event === 'order_paid' ? 'pending_ship' : 'pending_payment' };
};

/** 判断己方文本、图片或位置卡片是否仍处于两分钟撤回窗口且已经取得平台 PNM ID。 */
export const isRecallableChatMessage = (message: ChatMessage, nowMillis = Date.now()): boolean => {
	// sentAtMillis 兼容历史秒级和当前毫秒级消息时间。
	const sentAtMillis = message.sent_at < 10_000_000_000 ? message.sent_at * 1000 : message.sent_at;
	// ageMillis 是当前浏览器与消息时间的差值，允许最多十秒时钟前偏。
	const ageMillis = nowMillis - sentAtMillis;
	return message.direction === 'outgoing'
		&& (message.message_type === 'text' || message.message_type === 'image' || message.message_type === 'location')
		&& message.status === 'sent'
		&& Boolean(message.platform_message_id)
		&& ageMillis >= -10_000
		&& ageMillis <= 120_000;
};

/** 将可确认的普通入站消息转换为平台已读回执。 */
export const collectChatReadReceipts = (messages: ChatMessage[], chatID: string): ChatReadReceipt[] => messages
  .filter(/* currentMessage 只保留平台可接受的普通入站消息。 */ currentMessage => (
    currentMessage.direction === 'incoming'
    && currentMessage.message_type !== 'system'
    && !currentMessage.message_key.startsWith('in-')
  ))
  .map(/* currentMessage 为每条消息构造其所属会话的已读确认。 */ currentMessage => ({
    messageId: currentMessage.message_key,
    sessionId: chatID,
    cid: `${chatID}@goofish`,
    conversationType: 1,
  }));

/** 按搜索条件筛选会话列表。 */
export const filterChatSessions = (sessions: ChatSession[], search: string, unreadOnly: boolean): ChatSession[] => {
  // keyword 搜索关键词。
  const keyword = search.trim().toLowerCase();
  return sessions.filter(/* 当前回调处理集合中的单个元素。 */ session => {
    if (unreadOnly && session.unread_count <= 0) return false;
    if (!keyword) return true;
    return [session.buyer_name, session.buyer_id, session.item_title, session.last_message]
      .some(/* 当前回调处理集合中的单个元素。 */ value => (value || '').toLowerCase().includes(keyword));
  });
};

/** sortChatSessionsByPinAndActivity 把已置顶会话稳定保留在顶部，普通会话按最近消息时间倒序。 */
export const sortChatSessionsByPinAndActivity = (sessions: ChatSession[]): ChatSession[] => [...sessions].sort(
	/* leftSession、rightSession 是当前比较持久化置顶分组和活跃度的两条会话。 */ (leftSession, rightSession) => {
		// leftPinned 是兼容旧服务缺失字段后的左侧会话置顶状态。
		const leftPinned = leftSession.is_pinned === true;
		// rightPinned 是兼容旧服务缺失字段后的右侧会话置顶状态。
		const rightPinned = rightSession.is_pinned === true;
		if (leftPinned !== rightPinned) return leftPinned ? -1 : 1;
		// 已置顶组保留服务端 pinned_at 顺序，避免新消息改写用户手工顺序。
		if (leftPinned) return 0;
		return rightSession.last_message_at - leftSession.last_message_at;
	},
);

/** 合并历史消息并按消息键去重。 */
export const mergeOlderMessages = (current: ChatMessage[], older: ChatMessage[]): ChatMessage[] => {
  // keys keys，负责当前功能中的对应处理。
  const keys = new Set(current.map(/* 当前回调处理集合中的单个元素。 */ message => message.message_key));
  return [...older.filter(/* 当前回调处理集合中的单个元素。 */ message => !keys.has(message.message_key)), ...current];
};

/** 合并实时消息并替换同消息键的临时记录。 */
export const mergeLiveMessage = (current: ChatMessage[], incoming: ChatMessage): ChatMessage[] => {
  // index 当前索引。
  const index = current.findIndex(/* 当前回调处理用户交互或异步状态变化。 */ message => message.message_key === incoming.message_key);
  if (index < 0) return [...current, incoming];
	// statusPriority 保证已发送、撤回中和已撤回终态不被迟到的 sending／failed 事件回退。
	const statusPriority: Record<ChatMessage['status'], number> = { sending: 0, failed: 1, received: 2, sent: 2, recall_pending: 3, recall_unknown: 3, recalled: 4 };
	return current.map(/* message、currentIndex 是当前同键合并候选和在集合中的下标。 */ (message, currentIndex) => {
		if (currentIndex !== index) return message;
		// mergedStatus 是当前已知状态与迟到实时状态中更靠后的单调结果。
		const mergedStatus = statusPriority[incoming.status] >= statusPriority[message.status] ? incoming.status : message.status;
		return {
			...message,
			...incoming,
			id: incoming.id || message.id,
			account_id: incoming.account_id || message.account_id,
			chat_id: incoming.chat_id || message.chat_id,
			platform_message_id: incoming.platform_message_id || message.platform_message_id,
			reply_to_platform_message_id: incoming.reply_to_platform_message_id || message.reply_to_platform_message_id,
			sender_id: incoming.sender_id || message.sender_id,
			sender_name: incoming.sender_name || message.sender_name,
			content: incoming.content || message.content,
			status: mergedStatus,
			read_status: Math.max(Number(message.read_status || 0), Number(incoming.read_status || 0)),
			read_at: Math.max(Number(message.read_at || 0), Number(incoming.read_at || 0)),
			sent_at: incoming.sent_at || message.sent_at,
			recalled_at: Math.max(Number(message.recalled_at || 0), Number(incoming.recalled_at || 0)),
			reply_preview: incoming.reply_preview || message.reply_preview,
			system_card: incoming.system_card || message.system_card,
			location_card: incoming.location_card || message.location_card,
		};
	});
};

/** 买家普通入站消息到达时，把此前同会话的已发送出站消息同步为已读。 */
export const markOutgoingMessagesReadByIncoming = (current: ChatMessage[], incoming: ChatMessage): ChatMessage[] => {
  if (incoming.direction !== 'incoming' || incoming.message_type === 'system') return current;
  // readAt 保存平台入站消息时间，缺失时使用本机时间作为 UI 增量更新回退值。
  const readAt = incoming.sent_at > 0 ? incoming.sent_at : Date.now();
  return current.map(/* 当前回调把已被买家后续消息确认的出站消息更新为已读。 */ message => (
    message.chat_id === incoming.chat_id
    && message.direction === 'outgoing'
    && message.status === 'sent'
    && message.sent_at <= incoming.sent_at
      ? { ...message, read_status: 2, read_at: readAt }
      : message
  ));
};

/** 判断 Chat 请求响应是否仍属于当前账号和会话。 */
export const isCurrentChatRequest = (currentSequence: number, requestSequence: number, signal: AbortSignal): boolean => (
  currentSequence === requestSequence && !signal.aborted
);

/** 判断错误是否来自请求主动取消。 */
export const isChatAbortError = (error: unknown): boolean => error instanceof Error && error.message === '请求已取消';

/** 将聊天时间戳格式化为列表时间。 */
export const formatClock = (value: number): string => {
  if (!value) return '';
  // date 日期。
  const date = new Date(value < 10_000_000_000 ? value * 1000 : value);
  // today 今天日期。
  const today = new Date();
  if (date.toDateString() === today.toDateString()) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false });
  }
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' });
};

/** 将聊天时间戳格式化为消息详情时间。 */
export const messageTime = (value: number): string => {
  // date 日期。
  const date = new Date(value < 10_000_000_000 ? value * 1000 : value);
  return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false });
};
