import {
	AlertCircle,Check,CheckCheck,ChevronLeft,Copy,History,ImagePlus,Loader2,MapPinned,MessageCircleMore,MoreHorizontal,Package,RefreshCw,
	MessageSquareReply,Pin,Search,Send,Smile,UserRound,Volume2,VolumeX,Wifi,WifiOff,X,XCircle,
} from 'lucide-react';
import React from 'react';
import Lightbox from 'yet-another-react-lightbox';
import 'yet-another-react-lightbox/styles.css';
import { CloseOrderModal } from '../components/CloseOrderModal';
import { ChatCreditBadges } from '../components/ChatCreditBadges';
import { ConversationOrderPreviewPanel } from '../components/ConversationOrderPreviewPanel';
import { LocationCardModal } from '../components/LocationCardModal';
import { TradeSystemCard } from '../components/TradeSystemCard';
import type { ChatMessage } from '../api';
import { useChat } from '../hooks';
import { adjustPriceDisabledMessageReasons,groupChatMessages,isRecallableChatMessage,receivedRedFlowerMessageKeys,shipmentDisabledMessageReasons,uniqueCancellableSellerOrder,unreadBadgeClassName,unreadBadgeLabel } from '../state';
import { useChatSoundPreference } from '../soundNotification';

// CompactChatPane 描述窄屏聊天页当前展示会话列表还是消息详情。
type CompactChatPane = 'sessions' | 'conversation';

// MessageMenuPlacement 描述消息操作菜单相对气泡向上或向下展开。
type MessageMenuPlacement = 'above' | 'below';

// MessageReplyPreview 保存当前输入区只读引用条所需的非敏感消息摘要。
interface MessageReplyPreview {
	/** accountID 是选择引用时的账号，防止切换店铺后沿用旧目标。 */
	accountID: string;
	/** chatID 是选择引用时的会话，防止快速切换买家后把旧目标带入新会话。 */
	chatID: string;
	/** messageKey 是引用目标在当前会话内的本地稳定键。 */
	messageKey: string;
	/** senderLabel 是引用条展示的发送者名称。 */
	senderLabel: string;
	/** summary 是单行文本摘要或媒体类型占位。 */
	summary: string;
}

// MessageReplyQuoteProps 提供历史气泡解析原生引用摘要所需的当前消息、已加载索引和发送者回退文案。
interface MessageReplyQuoteProps {
	/** message 是可能携带原生引用目标的当前气泡消息。 */
	message: ChatMessage;
	/** messagesByPlatformID 是当前已加载分页中按 PNM 建立的同会话消息索引。 */
	messagesByPlatformID: Map<string, ChatMessage>;
	/** outgoing 表示当前引用消息是否为己方蓝色气泡。 */
	outgoing: boolean;
	/** incomingFallbackName 是目标没有昵称时使用的当前买家名称。 */
	incomingFallbackName: string;
	/** outgoingFallbackName 是己方目标只保存“我”时显示的真实账号昵称。 */
	outgoingFallbackName: string;
}

// messageMenuRequiredSpace 是回复、复制与可选撤回三项菜单需要预留的最小垂直像素。
const messageMenuRequiredSpace = 152;

/** messageReplySummary 为引用条生成可识别且不泄露媒体地址的单行摘要。 */
const messageReplySummary = (message: ChatMessage): string => {
	if (message.message_type === 'image') return '[图片]';
	if (message.message_type === 'video') return '[视频]';
	if (message.message_type === 'location') return '[位置]';
	return message.content.trim() || '消息内容不可用';
};

/** messageSupportsNativeReply 只允许真实属于当前账号和会话、且已关联 PNM 的普通消息进入引用预览。 */
const messageSupportsNativeReply = (message: ChatMessage, accountID: string, chatID: string): boolean =>
	message.account_id === accountID && message.chat_id === chatID && message.message_type !== 'system' && Boolean(message.platform_message_id?.trim() || message.message_key.endsWith('.PNM'));

/** messageReplyDisabledReason 区分迟到的跨会话消息、系统消息和缺失 PNM 的历史记录。 */
const messageReplyDisabledReason = (message: ChatMessage, accountID: string, chatID: string): string => {
	if (message.account_id !== accountID || message.chat_id !== chatID) return '该消息不属于当前会话，暂时无法回复';
	if (message.message_type === 'system') return '系统消息暂时无法回复';
	return '该历史消息缺少平台标识，暂时无法回复';
};

/** conversationDraftKey 用账号和会话双重身份生成页面内草稿隔离键，缺少任一身份时不持有草稿。 */
const conversationDraftKey = (accountID: string, chatID: string): string => accountID && chatID ? `${accountID}:${chatID}` : '';

/** currentConversationReplyTarget 只返回明确属于当前账号和会话的引用目标，其他目标在绘制和发送阶段均不可见。 */
const currentConversationReplyTarget = (target: MessageReplyPreview | null, accountID: string, chatID: string): MessageReplyPreview | null =>
	target?.accountID === accountID && target.chatID === chatID ? target : null;

/** replyQuoteSummary 把被引用消息转换为单行文本、媒体占位或撤回状态。 */
const replyQuoteSummary = (messageType: ChatMessage['message_type'], content: string, status: ChatMessage['status']): string => {
	if (status === 'recalled') return '该消息已撤回';
	if (messageType === 'image') return '[图片]';
	if (messageType === 'video') return '[视频]';
	if (messageType === 'location') return '[位置]';
	return content.trim() || '引用了一条历史消息';
};

/** MessageReplyQuote 在已发送或历史气泡内展示原生引用的发送者和原消息摘要。 */
const MessageReplyQuote: React.FC<MessageReplyQuoteProps> = ({ message, messagesByPlatformID, outgoing, incomingFallbackName, outgoingFallbackName }) => {
	if (!message.reply_to_platform_message_id) return null;
	// loadedTarget 是当前分页已加载的被引用消息，实时回显时可先于服务端快照使用。
	const loadedTarget = messagesByPlatformID.get(message.reply_to_platform_message_id);
	// resolvedTarget 表示服务端快照或当前分页至少有一个可用的原消息来源。
	const resolvedTarget = Boolean(message.reply_preview || loadedTarget);
	// replyDirection 优先使用服务端自连接快照，并回退当前已加载目标的消息方向。
	const replyDirection = message.reply_preview?.direction || loadedTarget?.direction;
	// senderLabel 是被引用消息的发送者名称；目标不在分页时保留“历史消息”语义。
	// storedSenderName 是持久化快照或已加载目标保存的原发送者名称。
	const storedSenderName = message.reply_preview?.sender_name || loadedTarget?.sender_name || '';
	// senderLabel 己方目标优先显示真实账号昵称，对方目标优先显示平台昵称。
	const senderLabel = !resolvedTarget ? '历史消息' : replyDirection === 'outgoing'
		? (outgoingFallbackName || (storedSenderName !== '我' ? storedSenderName : '') || '我')
		: (storedSenderName || incomingFallbackName || '对方');
	// messageType 是服务端快照或当前分页目标提供的内容类型。
	const messageType = message.reply_preview?.message_type || loadedTarget?.message_type;
	// status 是被引用消息的当前状态，缺少目标时使用 received 构造历史占位。
	const status = message.reply_preview?.status || loadedTarget?.status || 'received';
	// content 是被引用消息的原文，目标未加载且无快照时为空。
	const content = message.reply_preview?.content || loadedTarget?.content || '';
	// summary 是引用区最终显示的不泄露媒体地址的单行摘要。
	const summary = messageType ? replyQuoteSummary(messageType, content, status) : '引用了一条历史消息';
	return <div className={`mb-3 max-w-full border-l-2 pl-3 pr-1 text-left text-xs ${outgoing ? 'border-white/60' : 'border-slate-300'}`}>
		<div className={`truncate font-semibold ${outgoing ? 'text-sky-100' : 'text-slate-500'}`}>{senderLabel}</div>
		<div className={`mt-1 truncate ${outgoing ? 'text-white/90' : 'text-slate-600'}`} title={summary}>{summary}</div>
	</div>;
};

/** resolveMessageMenuPlacement 比较消息行在聊天滚动区上下的可用空间，底部不足且上方更宽时改为向上展开。 */
const resolveMessageMenuPlacement = (owner: HTMLElement): MessageMenuPlacement => {
	// viewport 是裁剪消息和菜单的聊天独立滚动容器。
	const viewport = owner.closest<HTMLElement>('[role="log"]');
	if (!viewport) return 'below';
	// ownerRect、viewportRect 是消息行和滚动视口的当前屏幕几何位置。
	const ownerRect = owner.getBoundingClientRect();
	const viewportRect = viewport.getBoundingClientRect();
	// availableBelow 是消息行底部到滚动视口底部的可用像素。
	const availableBelow = viewportRect.bottom - ownerRect.bottom;
	// availableAbove 是消息行顶部到滚动视口顶部的可用像素。
	const availableAbove = ownerRect.top - viewportRect.top;
	return availableBelow < messageMenuRequiredSpace && availableAbove > availableBelow ? 'above' : 'below';
};

// Chat 展示实时会话、消息分页和消息发送界面。
const Chat: React.FC = () => {
  // chatState 是 Chat feature Hook 提供的状态、引用和交互动作。
  const {
    accounts, activeAccountID, activeChatID, navigationPriorityChatID, closeEligibleOrderID, closeEligibleOrderStage, activeAccount, activeSessions, selectedSession, filteredSessions, creditProfile, creditLoading, creditError,
    messages, search, unreadOnly, draft, loading, messagesLoading, olderLoading, hasOlder, contactsLoading,
    hasMoreContacts, emojiOpen, sending, error, liveState, pendingImage, locationCardDefaults, locationDefaultsLoading, scrollRef, imageInputRef, setActiveAccountID,
		setActiveChatID, clearNavigationPriority, setSearch, setUnreadOnly, setDraft, setEmojiOpen, reloadSessions, loadMoreContacts, pinningSessionIDs, setSessionPinned,
    loadOlderMessages, handleMessageScroll, handleSend, handleImage, handlePastedImages, confirmSendImage, handleSendLocationCard, loadLocationCardDefaults, closeImagePreview, retrySend, recallMessage, retryAvailable,
    unreadForAccount, emojiURL, xianyuEmojis, renderXianyuText, formatClock, messageTime,
  } = useChat();

  // imageMessages 保存当前会话中的图片消息，供灯箱按消息顺序浏览。
  const imageMessages = React.useMemo(/* 当前回调筛选当前会话中的图片消息。 */ () => messages.filter(/* 当前回调判断消息是否为图片类型。 */ message => message.message_type === 'image'), [messages]);
  // imageSlides 将聊天图片转换为灯箱组件所需的展示模型。
  const imageSlides = React.useMemo(/* 当前回调构造灯箱图片展示数据。 */ () => imageMessages.map(/* 当前回调转换单条图片消息。 */ message => ({ src: message.content, alt: '聊天图片' })), [imageMessages]);
  // messageGroups 合并短时间内同一发送方的连续消息，减少重复头像和名称造成的纵向占用。
  const messageGroups = React.useMemo(/* 当前回调按稳定业务规则派生视觉消息组。 */ () => groupChatMessages(messages), [messages]);
	// messagesByPlatformID 为当前已加载消息建立 PNM 索引，供实时引用气泡在历史 API 快照到达前解析原文。
	const messagesByPlatformID = React.useMemo(/* buildLoadedMessageIndex 忽略尚未绑定 PNM 的本地待发送行。 */ () => {
		// result 是只包含已绑定平台标识消息的当前分页索引。
		const result = new Map<string, ChatMessage>();
		for (const /* indexedMessage 是当前待登记 PNM 索引的已加载消息。 */ indexedMessage of messages) {
			if (indexedMessage.platform_message_id) result.set(indexedMessage.platform_message_id, indexedMessage);
		}
		return result;
	}, [messages]);
  // receivedRedFlowerKeys 把后续收花结果匹配到同会话最近一条送花卡片。
  const receivedRedFlowerKeys = React.useMemo(/* currentFlowerState 派生已经完成收花的消息键。 */ () => receivedRedFlowerMessageKeys(messages), [messages]);
  // disabledAdjustPriceReasons 把后续交易终态映射到同订单待付款卡片，实时付款时立即关闭改价入口。
  const disabledAdjustPriceReasons = React.useMemo(/* currentTradeState 派生当前已加载消息中的不可改价原因。 */ () => adjustPriceDisabledMessageReasons(messages), [messages]);
	// disabledShipmentReasons 把后续发货、完成、取消或退款状态映射到原付款卡片。
	const disabledShipmentReasons = React.useMemo(/* currentShipmentState 派生当前已加载消息中的不可发货原因。 */ () => shipmentDisabledMessageReasons(messages), [messages]);
	// cancellableSellerOrder 是当前会话唯一一笔待付款或已付款待发货卖家订单；多笔时为空以拒绝猜测。
	const cancellableSellerOrder = React.useMemo(/* currentCancellableOrder 派生顶部取消入口的精确订单上下文。 */ () => uniqueCancellableSellerOrder(messages), [messages]);
	// eligibleCancellableSellerOrder 只保留后端订单表、聊天角色和幂等共同确认仍可取消的候选。
	const eligibleCancellableSellerOrder = cancellableSellerOrder?.orderID === closeEligibleOrderID ? cancellableSellerOrder : undefined;
	// conversationOrderTarget 记录最新订单卡片与需要补全金额的待付款／已付款订单。
	const conversationOrderTarget = React.useMemo(/* currentOrderTarget 从当前会话已加载系统卡片派生精确订单目标。 */ () => {
		// target 保存遍历后的最新卡片键和可补全订单号。
		const target = { revision: '', enrichmentOrderID: '' };
    for (const /* message 是当前检查订单关联的聊天消息。 */ message of messages) {
			if (message.message_type !== 'system' || !message.system_card?.order_id) continue;
			target.revision = message.message_key;
			target.enrichmentOrderID = message.system_card.event === 'order_pending_payment' || message.system_card.event === 'order_paid' ? message.system_card.order_id : '';
    }
		return target;
  }, [messages]);
  // unreadSessionCount 统计当前已加载会话中仍需处理的未读会话数，不把消息条数误当联系人数量。
  const unreadSessionCount = React.useMemo(/* 当前回调统计当前账号已加载的未读会话。 */ () => activeSessions.filter(/* currentSession 是当前检查未读状态的账号内会话。 */ currentSession => currentSession.unread_count > 0).length, [activeSessions]);
  // lightboxIndex 保存当前灯箱图片下标；负值表示灯箱关闭。
  const [lightboxIndex, setLightboxIndex] = React.useState(-1);
  // compactPane 保存窄屏短暂导航状态；宽屏始终同时展示两个面板。
  const [compactPane, setCompactPane] = React.useState<CompactChatPane>('sessions');
	// sessionListRef 指向左侧联系人滚动区，订单深链选中后把临时优先会话带回可见顶部。
	const sessionListRef = React.useRef<HTMLDivElement | null>(null);
		// messageMenuKey 保存当前打开操作菜单的消息键；空值表示没有菜单。
		const [messageMenuKey, setMessageMenuKey] = React.useState('');
	// messageMenuPlacement 保存当前菜单根据滚动视口空间选择的上下展开方向。
	const [messageMenuPlacement, setMessageMenuPlacement] = React.useState<MessageMenuPlacement>('below');
	// replyPreviewTarget 保存当前会话的引用目标；发送时只提交本地消息键供服务端再校验。
	const [replyPreviewTarget, setReplyPreviewTarget] = React.useState<MessageReplyPreview | null>(null);
	// replyComposerRef 指向消息输入框，选择回复后把键盘焦点交还给撰写区。
	const replyComposerRef = React.useRef<HTMLTextAreaElement | null>(null);
	// conversationDraftsRef 在当前 Chat 页生命周期内按账号和会话保存未发送文本。
	const conversationDraftsRef = React.useRef<Map<string, string>>(new Map());
	// conversationReplyTargetsRef 按同一会话键保存引用目标，使返回原会话时恢复回复状态。
	const conversationReplyTargetsRef = React.useRef<Map<string, MessageReplyPreview | null>>(new Map());
	// activeDraftKeyRef 记录上一次渲染所属的撰写状态隔离键，用于切换时先保存再恢复。
	const activeDraftKeyRef = React.useRef(conversationDraftKey(activeAccountID, activeChatID));
	// draftValueRef 始终指向当前输入框最新文本，避免会话切换 effect 读到旧闭包。
	const draftValueRef = React.useRef(draft);
	draftValueRef.current = draft;
	// replyPreviewValueRef 始终指向当前引用条目标，与文本草稿使用同一会话切换时机。
	const replyPreviewValueRef = React.useRef<MessageReplyPreview | null>(replyPreviewTarget);
	replyPreviewValueRef.current = replyPreviewTarget;
	// visibleReplyPreviewTarget 是通过账号和会话双重校验后允许当前页面展示和提交的引用目标。
	const visibleReplyPreviewTarget = currentConversationReplyTarget(replyPreviewTarget, activeAccountID, activeChatID);
  // cancelOrderOpen 表示当前会话顶部的取消订单弹窗是否打开。
  const [cancelOrderOpen, setCancelOrderOpen] = React.useState(false);
  // cancelledOrderID 保存本轮页面生命周期内平台明确取消成功的订单号。
  const [cancelledOrderID, setCancelledOrderID] = React.useState('');
  // cancelledOrderMessage 保存本轮明确取消结果，供顶部按钮与结果页同步。
  const [cancelledOrderMessage, setCancelledOrderMessage] = React.useState('');
  // chatSound 保存当前浏览器提示音偏好、解锁状态和用户手势切换动作。
  const chatSound = useChatSoundPreference();
  // orderPreviewOpen 控制中窄屏历史订单抽屉；超宽屏第三栏始终可见。
  const [orderPreviewOpen, setOrderPreviewOpen] = React.useState(false);
	// locationCardOpen 控制当前会话的位置卡片编辑弹窗。
	const [locationCardOpen, setLocationCardOpen] = React.useState(false);
	React.useEffect(/* closeStaleLocationCardEditor 在账号或会话切换时关闭旧客户表单，避免串发。 */ () => setLocationCardOpen(false), [activeAccountID, activeChatID]);
  // openLightbox 根据消息键打开对应的图片灯箱。
  const openLightbox = React.useCallback(/* 当前回调定位并打开指定聊天图片。 */ (messageKey: string): void => {
    setLightboxIndex(imageMessages.findIndex(/* 当前回调匹配图片消息键。 */ item => item.message_key === messageKey));
  }, [imageMessages]);
  // handleAccountSelect 切换账号并让窄屏回到该账号的会话列表，避免继续显示旧账号详情。
  const handleAccountSelect = React.useCallback(/* 当前回调响应账号页签选择。 */ (accountID: string): void => {
		clearNavigationPriority();
    setActiveAccountID(accountID);
    setCompactPane('sessions');
	}, [clearNavigationPriority, setActiveAccountID]);
  // handleAccountTabKeyDown 实现页签左右键、Home 和 End 导航，并同步窄屏会话面板。
  const handleAccountTabKeyDown = React.useCallback(/* 当前回调响应账号页签键盘导航。 */ (event: React.KeyboardEvent<HTMLButtonElement>, accountIndex: number): void => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    // lastAccountIndex 是账号页签数组的最后一个有效下标，用于循环导航。
    const lastAccountIndex = accounts.length - 1;
    // nextAccountIndex 是本次键盘操作需要激活并聚焦的账号页签下标。
    const nextAccountIndex = event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? lastAccountIndex
        : event.key === 'ArrowRight'
          ? (accountIndex + 1) % accounts.length
          : (accountIndex - 1 + accounts.length) % accounts.length;
    // nextAccount 是键盘导航最终选中的账号；账号列表为空时不产生状态变更。
    const nextAccount = accounts[nextAccountIndex];
    if (!nextAccount) return;
    handleAccountSelect(nextAccount.id);
    // accountTabs 是当前页签容器内的可聚焦账号按钮集合，用于把视觉选择和键盘焦点保持一致。
    const accountTabs = event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    accountTabs?.[nextAccountIndex]?.focus();
  }, [accounts, handleAccountSelect]);
  // handleSessionSelect 打开指定会话，并在窄屏切换到消息详情面板。
  const handleSessionSelect = React.useCallback(/* 当前回调响应会话列表选择。 */ (chatID: string): void => {
		if (chatID !== navigationPriorityChatID) clearNavigationPriority();
    setActiveChatID(chatID);
    setCompactPane('conversation');
	}, [clearNavigationPriority, navigationPriorityChatID, setActiveChatID]);
	// orderNavigationScrollEffect 在临时优先会话完成渲染后把联系人列表滚回顶部，置顶项仍自然位于其前。
	React.useLayoutEffect(/* scrollOrderNavigationTarget 把完成临时排序的联系人栏带回顶部。 */ () => {
		if (!navigationPriorityChatID || activeChatID !== navigationPriorityChatID) return;
		sessionListRef.current?.scrollTo({ top: 0, behavior: 'auto' });
	}, [activeChatID, filteredSessions, navigationPriorityChatID]);
	// handleMessageContextMenu 阻止浏览器默认菜单，并按当前滚动视口空间打开指定消息的自定义菜单。
	const handleMessageContextMenu = React.useCallback(/* contextEvent、messageKey 是右键事件与目标消息键。 */ (contextEvent: React.MouseEvent<HTMLDivElement>, messageKey: string): void => {
		contextEvent.preventDefault();
		setMessageMenuPlacement(resolveMessageMenuPlacement(contextEvent.currentTarget));
		setMessageMenuKey(messageKey);
	}, []);
	// handleMessageMenuToggle 从消息旁按钮切换菜单，打开时同样执行上下空间判定。
	const handleMessageMenuToggle = React.useCallback(/* clickEvent、messageKey 是操作按钮点击事件与目标消息键。 */ (clickEvent: React.MouseEvent<HTMLButtonElement>, messageKey: string): void => {
		if (messageMenuKey === messageKey) {
			setMessageMenuKey('');
			return;
		}
		// owner 是按钮所属的完整消息行，用于计算菜单可用空间。
		const owner = clickEvent.currentTarget.closest<HTMLElement>('[data-message-menu-owner]');
		if (owner) setMessageMenuPlacement(resolveMessageMenuPlacement(owner));
		setMessageMenuKey(messageKey);
	}, [messageMenuKey]);
	// handleCopyMessage 把当前消息文本或媒体地址复制到本机剪贴板。
	const handleCopyMessage = React.useCallback(/* 当前回调响应消息菜单复制操作。 */ async (content: string): Promise<void> => {
		await navigator.clipboard.writeText(content);
		setMessageMenuKey('');
	}, []);
	// handleRecallMessage 执行服务端权威撤回并在完成后关闭操作菜单。
	const handleRecallMessage = React.useCallback(/* 当前回调响应消息菜单撤回操作。 */ async (messageKey: string): Promise<void> => {
		try {
			await recallMessage(messageKey);
		} finally {
			setMessageMenuKey('');
		}
		}, [recallMessage]);
	// handleReplyPreview 打开引用撰写状态并关闭消息菜单，实际发送只由后续提交动作触发。
	const handleReplyPreview = React.useCallback(/* message、senderLabel 是引用目标和界面显示的发送者。 */ (message: ChatMessage, senderLabel: string): void => {
		if (!messageSupportsNativeReply(message, activeAccountID, activeChatID)) return;
		setReplyPreviewTarget({ accountID: message.account_id, chatID: message.chat_id, messageKey: message.message_key, senderLabel, summary: messageReplySummary(message) });
		setMessageMenuKey('');
		// focusTimer 在 React 完成引用条渲染后再聚焦输入框。
		const focusTimer = window.setTimeout(/* focusReplyComposer 让用户可直接输入回复内容。 */ () => replyComposerRef.current?.focus(), 0);
		void focusTimer;
	}, [activeAccountID, activeChatID]);
	// handleComposerSend 把当前会话的可选引用目标交给发送 Hook，只在明确成功后清除引用条。
	const handleComposerSend = React.useCallback(/* submitCurrentComposer 发送当前文本和原生引用键。 */ async (): Promise<void> => {
		if (replyPreviewTarget && !visibleReplyPreviewTarget) {
			setReplyPreviewTarget(null);
			return;
		}
		// submittedReplyTarget 是本次发送开始时的引用目标，用于防止异步结果清除后续新选择。
		const submittedReplyTarget = visibleReplyPreviewTarget;
		// succeeded 只在 Hook 确认最新请求已被平台接受且本地状态收口时为 true。
		const succeeded = await handleSend(submittedReplyTarget?.messageKey);
		if (succeeded && submittedReplyTarget && replyPreviewValueRef.current?.messageKey === submittedReplyTarget.messageKey) {
			setReplyPreviewTarget(null);
		}
	}, [handleSend, replyPreviewTarget, visibleReplyPreviewTarget]);
	// handleComposerRetry 重试时由 Hook 保留原引用键，明确成功后才清除当前引用条。
	const handleComposerRetry = React.useCallback(/* retryCurrentComposer 执行带原目标的最近失败发送。 */ async (): Promise<void> => {
		// succeeded 表示最新重试已经完成平台发送和本地状态收口。
		const succeeded = await retrySend();
		if (succeeded) setReplyPreviewTarget(null);
	}, [retrySend]);
	// 当前 effect 只在消息操作菜单打开时监听页面外部点击和 Escape，关闭后立即释放全局监听。
	React.useEffect(/* closeMessageMenuOutside 让菜单内部操作保持可用，其他页面区域收回当前右键菜单。 */ () => {
		if (!messageMenuKey) return undefined;
		/** handleOutsidePointerDown 判断指针事件是否位于当前消息菜单所属行外部。 */
		const handleOutsidePointerDown = (pointerEvent: PointerEvent): void => {
			// target 是可用于向上查找消息菜单容器的 DOM 元素；非元素目标按外部处理。
			const target = pointerEvent.target instanceof Element ? pointerEvent.target : null;
			// owner 是指针位置最近的消息菜单容器，可能属于另一条消息。
			const owner = target?.closest<HTMLElement>('[data-message-menu-owner]');
			if (owner?.dataset.messageMenuOwner === messageMenuKey) return;
			setMessageMenuKey('');
		};
		/** handleMessageMenuEscape 允许键盘用户用 Escape 收回当前消息操作菜单。 */
		const handleMessageMenuEscape = (keyboardEvent: KeyboardEvent): void => {
			if (keyboardEvent.key === 'Escape') setMessageMenuKey('');
		};
		document.addEventListener('pointerdown', handleOutsidePointerDown);
		document.addEventListener('keydown', handleMessageMenuEscape);
		/** cleanupMessageMenuListeners 在菜单关闭、切换或页面卸载时释放文档级监听。 */
		const cleanupMessageMenuListeners = (): void => {
			document.removeEventListener('pointerdown', handleOutsidePointerDown);
			document.removeEventListener('keydown', handleMessageMenuEscape);
		};
		return cleanupMessageMenuListeners;
	}, [messageMenuKey]);
	// 当前 layout effect 在界面绘制新会话前保存旧撰写状态并恢复目标会话，避免文字或引用条串到其他买家。
	React.useLayoutEffect(/* restoreConversationComposer 使每个账号下的每个 chatID 拥有独文本和引用目标。 */ () => {
		// nextDraftKey 是当前新账号与会话组合的草稿隔离键。
		const nextDraftKey = conversationDraftKey(activeAccountID, activeChatID);
		// previousDraftKey 是切换前正在编辑的账号与会话键。
		const previousDraftKey = activeDraftKeyRef.current;
		if (nextDraftKey === previousDraftKey) return;
		if (previousDraftKey) {
			conversationDraftsRef.current.set(previousDraftKey, draftValueRef.current);
			conversationReplyTargetsRef.current.set(previousDraftKey, replyPreviewValueRef.current);
		}
		// nextDraft 是目标会话已保存的本地草稿，首次打开时为空。
		const nextDraft = nextDraftKey ? (conversationDraftsRef.current.get(nextDraftKey) || '') : '';
		// nextReplyTarget 是目标会话已保存的引用目标，首次打开时不展示引用条。
		const nextReplyTarget = nextDraftKey ? (conversationReplyTargetsRef.current.get(nextDraftKey) || null) : null;
		activeDraftKeyRef.current = nextDraftKey;
		draftValueRef.current = nextDraft;
		replyPreviewValueRef.current = nextReplyTarget;
		setDraft(nextDraft);
		setReplyPreviewTarget(nextReplyTarget);
	}, [activeAccountID, activeChatID, setDraft]);
	// 当前 effect 在账号或会话切换时关闭旧订单弹窗并清除仅属于上一会话的成功状态。
  React.useEffect(/* resetCancelOrderState 隔离不同账号和会话的取消订单状态。 */ () => {
		setCancelOrderOpen(false);
		setCancelledOrderID('');
		setCancelledOrderMessage('');
		setOrderPreviewOpen(false);
		setMessageMenuKey('');
	}, [activeAccountID, activeChatID]);

  if (loading) return <div className="flex h-[calc(100vh-4rem)] items-center justify-center"><Loader2 className="h-8 w-8 animate-spin text-sky-500" /></div>;

  return (
    <section className="flex h-[calc(100vh-4rem)] min-h-[560px] flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-chat">
      <header className="flex min-h-12 items-end justify-between gap-4 border-b border-slate-200 bg-slate-50/70 px-5">
        <div className="flex min-w-0 flex-1 gap-1 overflow-x-auto pb-0" role="tablist" aria-label="聊天账号">
          {accounts.map(/* 当前回调渲染可通过鼠标和键盘选择的账号页签。 */ (account, accountIndex) => {
            // active 表示当前页签是否控制下方账号会话面板。
            const active = account.id === activeAccountID;
            // unread 保存当前账号跨会话累计的未读消息数。
            const unread = unreadForAccount(account.id);
            // unreadLabel 保存账号未读徽标的展示文本。
            const unreadLabel = unreadBadgeLabel(unread);
            // online 表示当前账号运行时是否已连接消息服务。
            const online = account.runtime_state === 'online';
            return (
              <button key={account.id} id={`chat-account-tab-${account.id}`} type="button" role="tab" aria-selected={active} aria-controls="chat-account-panel" tabIndex={active ? 0 : -1}
                onClick={/* 当前回调通过鼠标选择账号页签。 */ () => handleAccountSelect(account.id)}
                onKeyDown={/* 当前回调把账号下标传给共享键盘导航处理器。 */ event => handleAccountTabKeyDown(event, accountIndex)}
                className={`relative flex h-11 shrink-0 items-center gap-2 border-b-2 px-3 text-sm font-extrabold transition-colors ${active ? 'border-sky-500 text-sky-700' : 'border-transparent text-slate-500 hover:text-slate-900'}`}>
                <span className={`h-2 w-2 rounded-full ${online ? 'bg-emerald-500' : 'bg-slate-300'}`} />
                <span className="max-w-36 truncate">{account.nickname || account.remark || account.id}</span>
                {unread > 0 && <span aria-label={`未读消息 ${unreadLabel} 条`} className={unreadBadgeClassName(unread)}>{unreadLabel}</span>}
              </button>
            );
          })}
        </div>
        <div className="mb-2.5 flex shrink-0 items-center gap-2"><button type="button" onClick={/* toggleChatSound 通过用户手势解锁或切换提示音。 */ () => void chatSound.toggle()} className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-bold transition ${chatSound.enabled && chatSound.unlocked ? 'bg-sky-100 text-sky-700' : 'bg-slate-100 text-slate-500'}`} title="新消息提示音">{chatSound.enabled && chatSound.unlocked ? <Volume2 className="h-3.5 w-3.5" /> : <VolumeX className="h-3.5 w-3.5" />}{!chatSound.unlocked ? '点击启用提示音' : chatSound.enabled ? '提示音已开启' : '提示音已关闭'}</button><div className={`flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-bold ${liveState === 'online' ? 'bg-emerald-50 text-emerald-700' : liveState === 'connecting' ? 'bg-amber-50 text-amber-700' : 'bg-red-50 text-red-700'}`}>
          {liveState === 'online' ? <Wifi className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
          {liveState === 'online' ? '实时同步中' : liveState === 'connecting' ? '正在连接' : '连接已断开'}
        </div></div>
      </header>

      {accounts.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center text-center">
          <MessageCircleMore className="h-12 w-12 text-slate-300" />
          <h3 className="mt-4 font-black text-slate-800">暂无启用账号</h3>
          <p className="mt-1 text-sm text-slate-500">先在账号管理中启用账号，聊天会话会自动出现。</p>
        </div>
      ) : (
        <div id="chat-account-panel" role="tabpanel" aria-labelledby={`chat-account-tab-${activeAccountID}`} className="grid min-h-0 flex-1 grid-cols-1 overflow-hidden xl:grid-cols-[320px_minmax(0,1fr)] 2xl:grid-cols-[320px_minmax(0,1fr)_clamp(360px,20vw,390px)]">
          <aside aria-label="会话列表" className={`${compactPane === 'sessions' ? 'flex' : 'hidden'} min-h-0 flex-col border-r border-slate-200 bg-slate-50/40 xl:flex`}>
            <div className="space-y-3 border-b border-slate-200 p-3">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                <label htmlFor="chat-session-search" className="sr-only">搜索会话</label>
                <input id="chat-session-search" value={search} maxLength={200} onChange={/* 当前回调更新会话搜索表单值。 */ event => setSearch(event.target.value)} placeholder="搜索用户、商品或全部历史消息"
                  className="h-10 w-full rounded-xl border border-slate-200 bg-white pl-9 pr-3 text-sm outline-none transition focus:border-sky-400 focus:ring-2 focus:ring-sky-100" />
              </div>
              <div className="flex items-center justify-between">
                <div role="group" aria-label="会话筛选" className="flex rounded-lg bg-slate-100 p-0.5">
                  <button type="button" aria-pressed={!unreadOnly} onClick={/* 当前回调展示当前账号全部已加载会话。 */ () => setUnreadOnly(false)}
                    className={`rounded-md px-2.5 py-1.5 text-xs font-bold transition ${!unreadOnly ? 'bg-white text-sky-700 shadow-sm' : 'text-slate-500 hover:text-slate-800'}`}>全部</button>
                  <button type="button" aria-pressed={unreadOnly} onClick={/* 当前回调仅展示当前账号未读会话。 */ () => setUnreadOnly(true)}
                    className={`rounded-md px-2.5 py-1.5 text-xs font-bold transition ${unreadOnly ? 'bg-white text-sky-700 shadow-sm' : 'text-slate-500 hover:text-slate-800'}`}>未读 {unreadSessionCount}</button>
                </div>
                <button type="button" title="刷新会话" aria-label="刷新会话" onClick={/* 当前回调重新读取当前账号会话。 */ () => void reloadSessions(activeAccountID)} className="flex h-10 w-10 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100 hover:text-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400">
                  <RefreshCw className="h-4 w-4" />
                </button>
              </div>
            </div>
			<div ref={sessionListRef} data-testid="chat-session-list-scroll" className="min-h-0 flex-1 overflow-y-auto">
              {filteredSessions.map(/* session 是当前渲染且可独立置顶的单个精确会话。 */ session => {
                // sessionPinned 表示服务端是否已确认持久化当前会话的置顶偏好。
                const sessionPinned = session.is_pinned === true;
                // sessionPinning 表示当前会话的置顶请求尚未完成，图钉不得重复提交。
                const sessionPinning = pinningSessionIDs.has(session.chat_id);
                // sessionName 是会话主按钮和置顶按钮共用的可读买家名称。
                const sessionName = session.buyer_name || `用户 ${session.buyer_id}`;
                // sessionActive 表示会话主按钮是否正在控制中央消息面板。
                const sessionActive = session.chat_id === activeChatID;
                return (
                  <div key={session.chat_id} data-testid="chat-session-row" data-chat-id={session.chat_id} data-pinned={sessionPinned ? 'true' : 'false'}
                    className={`group relative border-b border-slate-100 transition-colors ${sessionActive ? 'bg-sky-50/80 shadow-chat-active' : sessionPinned ? 'bg-sky-50/35' : 'hover:bg-white/80'}`}>
                    <button type="button" aria-pressed={sessionActive} onClick={/* 当前回调选择会话并在窄屏打开详情。 */ () => handleSessionSelect(session.chat_id)}
                      className="flex w-full gap-3 p-4 pr-14 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-sky-400">
                      <div className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-slate-200 text-slate-500">
                        {session.buyer_avatar_url ? <img src={session.buyer_avatar_url} alt="" className="h-full w-full object-cover" /> : <UserRound className="h-5 w-5" />}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="truncate text-sm font-extrabold text-slate-900">{sessionName}</span>
                          {sessionPinned && <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-sky-100 px-1.5 py-0.5 text-[10px] font-bold text-sky-700"><Pin className="h-2.5 w-2.5 fill-sky-200" />置顶</span>}
                          <span className="ml-auto shrink-0 text-xs font-medium text-slate-500">{formatClock(session.last_message_at)}</span>
                        </div>
                        <div className="mt-1 flex items-center gap-2">
                          <span className="truncate text-xs text-slate-500">{session.last_message || '暂无消息'}</span>
                          {session.unread_count > 0 && <span aria-label={`未读消息 ${unreadBadgeLabel(session.unread_count)} 条`} className={`ml-auto ${unreadBadgeClassName(session.unread_count)}`}>{unreadBadgeLabel(session.unread_count)}</span>}
                        </div>
                        {session.item_title && <div className="mt-1.5 truncate text-xs font-medium text-sky-700">商品 · {session.item_title}</div>}
                      </div>
                    </button>
                    <button type="button" title={sessionPinned ? '取消置顶' : '置顶会话'} aria-label={`${sessionPinned ? '取消置顶' : '置顶会话'} ${sessionName}`}
                      disabled={sessionPinning} onClick={/* 当前回调持久化该会话的相反置顶状态，不打开会话。 */ () => void setSessionPinned(session.chat_id, !sessionPinned)}
                      className={`absolute right-2.5 top-2.5 flex h-8 w-8 items-center justify-center rounded-lg border transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 disabled:cursor-wait disabled:opacity-70 ${sessionPinned ? 'border-sky-200 bg-sky-100 text-sky-700 opacity-100' : 'border-transparent bg-white/90 text-slate-400 opacity-0 shadow-sm hover:border-sky-200 hover:text-sky-600 group-hover:opacity-100 focus:opacity-100'}`}>
                      {sessionPinning ? <Loader2 className="h-4 w-4 animate-spin" /> : <Pin className={`h-4 w-4 ${sessionPinned ? 'fill-sky-200' : ''}`} />}
                    </button>
                  </div>
                );
              })}
              {filteredSessions.length === 0 && <div className="px-6 py-16 text-center text-sm text-slate-500">当前账号暂无匹配会话</div>}
              {hasMoreContacts && !search && !unreadOnly && <div className="flex justify-center p-4">
                <button type="button" onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void loadMoreContacts()} disabled={contactsLoading}
                  className="flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-xs font-bold text-slate-500 shadow-sm hover:border-sky-200 hover:text-sky-600 disabled:opacity-50">
                  {contactsLoading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}{contactsLoading ? '正在加载' : '加载更多历史联系人'}
                </button>
              </div>}
            </div>
          </aside>

          <main aria-label="聊天内容" className={`${compactPane === 'conversation' ? 'flex' : 'hidden'} min-h-0 min-w-0 flex-col overflow-hidden bg-surface-subtle xl:flex`}>
            {selectedSession ? (
              <>
                <div className="flex min-h-16 shrink-0 items-center gap-3 border-b border-slate-200 bg-white px-4 py-3 sm:px-5">
                  <button type="button" onClick={/* 当前回调让窄屏返回当前账号会话列表。 */ () => setCompactPane('sessions')}
                    className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-slate-600 transition hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 xl:hidden" aria-label="返回会话列表">
                    <ChevronLeft className="h-5 w-5" />
                  </button>
                  <div className="min-w-0 flex-1">
					<div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
						<span className="max-w-full truncate text-sm font-black text-slate-950">{selectedSession.buyer_name || selectedSession.buyer_id}</span>
						<ChatCreditBadges profile={creditProfile} loading={creditLoading} error={creditError} />
					</div>
                    <div className="mt-0.5 truncate text-xs text-slate-500">用户 ID：{selectedSession.buyer_id}</div>
                    {selectedSession.item_title && <div className="mt-1.5 flex min-w-0 items-center gap-1.5 text-xs font-medium text-sky-700" title={selectedSession.item_title}>
                      <Package className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                      <span className="truncate">关联商品 · {selectedSession.item_title}</span>
                    </div>}
                  </div>
                  <div className="ml-auto flex shrink-0 items-center gap-3">
                    <button type="button" onClick={/* openOrderPreview 只在未常驻第三栏的中窄屏打开历史订单抽屉。 */ () => setOrderPreviewOpen(true)} className="flex h-9 items-center gap-1.5 rounded-xl border border-sky-200 bg-sky-50 px-3.5 text-sm font-bold text-sky-700 transition hover:border-sky-300 hover:bg-sky-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 2xl:hidden"><History className="h-4 w-4" aria-hidden="true" /><span className="hidden sm:inline">历史订单</span></button>
                    {eligibleCancellableSellerOrder && <button type="button" onClick={/* openCancelOrder 从后端权威确认的可取消卖家订单打开动态原因弹窗。 */ () => setCancelOrderOpen(true)} disabled={cancelledOrderID === eligibleCancellableSellerOrder.orderID} aria-label={cancelledOrderID === eligibleCancellableSellerOrder.orderID ? '订单已取消' : '取消订单'} title={`取消订单 ${eligibleCancellableSellerOrder.orderID}`} className="flex h-9 items-center gap-1.5 rounded-xl border border-red-200 bg-white px-3.5 text-sm font-bold text-red-600 transition hover:border-red-300 hover:bg-red-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-200 disabled:cursor-not-allowed disabled:border-slate-200 disabled:bg-slate-100 disabled:text-slate-400"><XCircle className="h-4 w-4" aria-hidden="true" /><span className="hidden sm:inline">{cancelledOrderID === eligibleCancellableSellerOrder.orderID ? '已取消' : '取消订单'}</span></button>}
                    <span className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-bold ${activeAccount?.runtime_state === 'online' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'}`}>
                      {activeAccount?.runtime_state === 'online' ? '账号在线' : '账号离线'}
                    </span>
                  </div>
                </div>
                <div ref={scrollRef} role="log" aria-label="聊天消息" aria-live="polite" aria-relevant="additions text" aria-busy={messagesLoading}
                  onScroll={handleMessageScroll} className="min-h-0 flex-1 space-y-5 overflow-y-auto px-4 py-5 sm:px-6">
                  {messagesLoading ? <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin text-sky-500" /></div> : <>
                    {hasOlder && <div className="flex justify-center pb-1">
                      <button type="button" onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void loadOlderMessages()} disabled={olderLoading}
                        className="flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-1.5 text-xs font-bold text-slate-500 shadow-sm transition hover:border-sky-200 hover:text-sky-600 disabled:opacity-50">
                        {olderLoading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}{olderLoading ? '正在加载' : '加载更早消息'}
                      </button>
                    </div>}
                    {messageGroups.map(/* 当前回调渲染已按发送方和时间合并的消息组。 */ messageGroup => {
                    // firstMessage 是当前消息组第一条消息，决定发送方向、头像和系统事件类型。
                    const firstMessage = messageGroup[0];
                    // lastMessage 是当前消息组最后一条消息，承担整组时间和发送状态展示。
                    const lastMessage = messageGroup[messageGroup.length - 1];
                    // outgoing 表示当前消息组是否由所选账号发出。
                    const outgoing = firstMessage.direction === 'outgoing';
                    // system 表示当前组是必须独立显示的平台交易或通知事件。
                    const system = firstMessage.message_type === 'system';
                    if (system) {
                      if (firstMessage.system_card?.kind === 'trade') {
	                        return <TradeSystemCard key={firstMessage.message_key} card={firstMessage.system_card} time={messageTime(firstMessage.sent_at)} received={receivedRedFlowerKeys.has(firstMessage.message_key)} accountID={firstMessage.account_id} adjustPriceDisabledReason={cancelledOrderID && firstMessage.system_card.order_id === cancelledOrderID ? '订单已取消，不能再修改价格' : disabledAdjustPriceReasons.get(firstMessage.message_key)} shipmentDisabledReason={disabledShipmentReasons.get(firstMessage.message_key)} sellerActionAllowed={firstMessage.sender_id !== firstMessage.account_id} />;
                      }
                      return (
                        <div key={firstMessage.message_key} className="flex justify-center py-1">
                          <div className="max-w-[82%] rounded-full border border-slate-200 bg-slate-100 px-4 py-1.5 text-center text-xs leading-5 text-slate-600">
                            <span className="font-semibold text-slate-500">系统事件 · </span>{renderXianyuText(firstMessage.content)}
                            <span className="ml-2 whitespace-nowrap text-xs text-slate-500">{messageTime(firstMessage.sent_at)}</span>
                          </div>
                        </div>
                      );
                    }
					if (firstMessage.status === 'recalled') {
						return <div key={firstMessage.message_key} className="flex justify-center py-1">
							<div className="rounded-full bg-slate-100 px-4 py-1.5 text-xs text-slate-500">
								{outgoing ? '你撤回了一条消息' : '对方撤回了一条消息'}
								{outgoing && firstMessage.message_type === 'text' && <button type="button" onClick={/* 当前回调把撤回文本放回草稿供人工修改。 */ () => setDraft(firstMessage.content)} className="ml-2 font-bold text-sky-700 hover:underline">重新编辑</button>}
							</div>
						</div>;
					}
                    // senderLabel 是当前消息组唯一展示的发送者名称，避免连续消息重复占用空间。
                    const senderLabel = outgoing
                      ? (activeAccount?.nickname || activeAccount?.remark || '我')
                      : (selectedSession.buyer_name || firstMessage.sender_name || selectedSession.buyer_id);
                    return (
	                      <div key={firstMessage.message_key} className={`flex items-start gap-2.5 ${outgoing ? 'justify-end' : 'justify-start'}`}>
	                        {!outgoing && <div className="h-10 w-10 shrink-0 overflow-hidden rounded-full bg-slate-200 ring-2 ring-white">
                          {selectedSession.buyer_avatar_url ? <img src={selectedSession.buyer_avatar_url} alt={selectedSession.buyer_name || '用户'} className="h-full w-full object-cover" /> : <UserRound className="m-2 h-5 w-5 text-slate-500" />}
                        </div>}
                        <div className={`flex max-w-[min(72%,42.5rem)] flex-col ${outgoing ? 'items-end' : 'items-start'}`}>
                          <div className="mb-1 px-1 text-xs font-semibold text-slate-500">{senderLabel}</div>
                          <div className={`flex flex-col gap-1.5 ${outgoing ? 'items-end' : 'items-start'}`}>
                            {messageGroup.map(/* 当前回调渲染组内每一条保留独立媒体或文本内容的消息。 */ message => <div key={message.message_key} data-message-menu-owner={message.message_key} className={`group relative flex items-center gap-1.5 ${outgoing ? 'flex-row-reverse' : ''}`}
								onContextMenu={/* 当前回调传递右键事件和精确消息键。 */ contextEvent => handleMessageContextMenu(contextEvent, message.message_key)}>
								{message.message_type === 'image' ? (
									<button type="button" title="点击预览大图" onClick={openLightbox.bind(null, message.message_key)} className={`block cursor-zoom-in overflow-hidden rounded-2xl border bg-white p-1 text-left shadow-sm ${outgoing ? 'rounded-br-md border-sky-200' : 'rounded-bl-md border-slate-200'}`}>
										<img src={message.content} alt="聊天图片" className="max-h-80 max-w-full rounded-xl object-contain" />
									</button>
								) : message.message_type === 'video' ? (
									<video src={message.content} controls preload="metadata" className="max-h-80 max-w-full rounded-2xl bg-black" />
								) : message.message_type === 'location' && message.location_card ? (
									<div className={`w-[min(25rem,70vw)] overflow-hidden rounded-2xl border bg-white shadow-sm ${outgoing ? 'rounded-br-md border-sky-200' : 'rounded-bl-md border-slate-200'}`}>
										<div className="flex gap-3 bg-sky-50/80 px-4 py-3"><div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-sky-500 text-white"><MapPinned className="h-5 w-5" aria-hidden="true" /></div><div className="min-w-0"><div className="truncate text-sm font-black text-slate-900">{message.location_card.title}</div><div className="mt-1 whitespace-pre-wrap text-sm leading-5 text-slate-600">{message.location_card.description}</div></div></div>
										<div className="border-t border-sky-100 px-4 py-2 font-mono text-[11px] tabular-nums text-slate-400">{message.location_card.latitude.toFixed(6)}, {message.location_card.longitude.toFixed(6)}</div>
									</div>
								) : (
									<div className={`rounded-2xl px-4 py-2.5 text-sm leading-6 shadow-sm ${outgoing ? 'rounded-br-md bg-sky-500 text-white selection:bg-amber-200 selection:text-slate-950' : 'rounded-bl-md border border-slate-200 bg-white text-slate-800 selection:bg-sky-200 selection:text-slate-950'}`}>
										<MessageReplyQuote message={message} messagesByPlatformID={messagesByPlatformID} outgoing={outgoing} incomingFallbackName={selectedSession.buyer_name || selectedSession.buyer_id} outgoingFallbackName={activeAccount?.nickname || activeAccount?.remark || '我'} />
										<div>{renderXianyuText(message.content)}</div>
									</div>
								)}
								<button type="button" aria-label="消息操作" onClick={/* 当前回调传递按钮事件和精确消息键。 */ clickEvent => handleMessageMenuToggle(clickEvent, message.message_key)}
									className="flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 opacity-0 transition hover:bg-white hover:text-slate-700 focus:opacity-100 focus-visible:ring-2 focus-visible:ring-sky-400 group-hover:opacity-100">
									<MoreHorizontal className="h-4 w-4" />
								</button>
								{messageMenuKey === message.message_key && <div className={`absolute z-30 flex min-w-32 flex-col rounded-xl border border-slate-200 bg-white p-1 shadow-xl ${messageMenuPlacement === 'above' ? 'bottom-full mb-1' : 'top-full mt-1'} ${outgoing ? 'right-0' : 'left-0'}`} role="menu" aria-label="消息操作菜单">
									<button type="button" role="menuitem" disabled={!messageSupportsNativeReply(message, activeAccountID, activeChatID)} title={messageSupportsNativeReply(message, activeAccountID, activeChatID) ? '回复该条消息' : messageReplyDisabledReason(message, activeAccountID, activeChatID)} onClick={/* 当前回调只打开引用撰写预览。 */ () => handleReplyPreview(message, senderLabel)} className="flex items-center gap-2 rounded-lg px-3 py-2 text-left text-xs font-semibold text-slate-600 hover:bg-sky-50 hover:text-sky-700 disabled:cursor-not-allowed disabled:text-slate-300 disabled:hover:bg-transparent"><MessageSquareReply className="h-3.5 w-3.5" />回复</button>
									<button type="button" role="menuitem" onClick={/* 当前回调复制消息正文或媒体地址。 */ () => void handleCopyMessage(message.content)} className="flex items-center gap-2 rounded-lg px-3 py-2 text-left text-xs font-semibold text-slate-600 hover:bg-slate-100"><Copy className="h-3.5 w-3.5" />复制</button>
									{isRecallableChatMessage(message) && <button type="button" role="menuitem" onClick={/* 当前回调撤回符合两分钟条件的己方消息。 */ () => void handleRecallMessage(message.message_key)} className="rounded-lg px-3 py-2 text-left text-xs font-semibold text-red-600 hover:bg-red-50">撤回</button>}
								</div>}
							</div>)}
                          </div>
                          <div className="mt-1 flex items-center gap-1 text-xs text-slate-500">
                            {messageTime(lastMessage.sent_at)}
                            {outgoing && (lastMessage.status === 'failed' ? <AlertCircle className="h-3.5 w-3.5 text-red-500" aria-label="发送失败" /> : lastMessage.read_status === 2 && Number(lastMessage.read_at || 0) > 0 ? <CheckCheck className="h-3.5 w-3.5 text-sky-600" aria-label="对方已读" /> : lastMessage.status === 'sent' ? <Check className="h-3.5 w-3.5 text-sky-600" aria-label="已发送未读" /> : <Check className="h-3.5 w-3.5" aria-label="发送中" />)}
                          </div>
                        </div>
	                        {outgoing && <div className="h-10 w-10 shrink-0 overflow-hidden rounded-full bg-sky-100 ring-2 ring-white">
                          {activeAccount?.avatar_url ? <img src={activeAccount.avatar_url} alt="我" className="h-full w-full object-cover" /> : <UserRound className="m-2 h-5 w-5 text-sky-600" />}
                        </div>}
                      </div>
                    );
                    })}
                  </>}
                </div>
                {error && <div className="flex items-center justify-between gap-3 border-t border-red-100 bg-red-50 px-5 py-2 text-xs font-medium text-red-700"><span>{error}</span>{retryAvailable && <button type="button" className="font-bold underline" onClick={/* 当前回调保留原引用目标重试发送。 */ () => void handleComposerRetry()}>重试发送</button>}</div>}
                <div className="relative z-10 shrink-0 border-t border-slate-200 bg-white p-4 shadow-chat-input">
				  {visibleReplyPreviewTarget && <div className="mb-2 flex items-center gap-3 rounded-xl border border-sky-200 bg-sky-50/80 px-3 py-2 shadow-sm" aria-label="引用回复预览">
					<div className="h-9 w-1 shrink-0 rounded-full bg-sky-500" aria-hidden="true" />
					<div className="min-w-0 flex-1">
					  <div className="flex min-w-0 items-center gap-2 text-xs">
						<MessageSquareReply className="h-3.5 w-3.5 shrink-0 text-sky-600" aria-hidden="true" />
						<span className="truncate font-bold text-sky-800">回复 {visibleReplyPreviewTarget.senderLabel}</span>
						<span className="shrink-0 rounded-full bg-white px-2 py-0.5 font-semibold text-sky-600">引用回复</span>
					  </div>
					  <div className="mt-1 truncate text-xs text-slate-600" title={visibleReplyPreviewTarget.summary}>{visibleReplyPreviewTarget.summary}</div>
					</div>
					<button type="button" aria-label="取消回复" title="取消回复" onClick={/* cancelReplyPreview 只清除引用目标，保留已输入的草稿。 */ () => setReplyPreviewTarget(null)} className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-slate-400 transition hover:bg-white hover:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400"><X className="h-4 w-4" /></button>
				  </div>}
                  <div className="mb-2 flex items-center gap-1">
                    <div className="relative">
                      <button type="button" onClick={/* 当前回调切换闲鱼表情面板。 */ () => setEmojiOpen(/* currentOpen 表示切换前的表情面板展开状态。 */ currentOpen => !currentOpen)} disabled={sending || activeAccount?.runtime_state !== 'online'} className="flex h-10 w-10 items-center justify-center rounded-lg text-slate-500 hover:bg-sky-50 hover:text-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 disabled:opacity-40" title="闲鱼表情"><Smile className="h-5 w-5" /></button>
                      {emojiOpen && <div className="absolute bottom-11 left-0 z-30 w-[360px] rounded-2xl border border-slate-200 bg-white p-3 shadow-2xl">
                        <div className="mb-2 text-xs font-bold text-slate-500">全部表情</div>
                        <div className="grid max-h-72 grid-cols-8 gap-1 overflow-y-auto">
                          {xianyuEmojis.map(/* 当前回调处理集合中的单个元素。 */ ([name, file]) => <button key={name} type="button" title={`[${name}]`} onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => { setDraft(/* 当前回调处理用户交互或异步状态变化。 */ value => value + `[${name}]`); setEmojiOpen(false); }} className="flex h-10 w-10 items-center justify-center rounded-lg hover:bg-slate-100"><img src={emojiURL(file)} alt={`[${name}]`} className="h-8 w-8 object-contain" /></button>)}
                        </div>
                      </div>}
                    </div>
                    <input ref={imageInputRef} type="file" accept="image/*" className="hidden" onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => void handleImage(event.target.files?.[0])} />
                    <button type="button" onClick={/* 当前回调打开浏览器图片选择器。 */ () => imageInputRef.current?.click()} disabled={sending || activeAccount?.runtime_state !== 'online' || Boolean(visibleReplyPreviewTarget)} className="flex h-10 w-10 items-center justify-center rounded-lg text-slate-500 transition hover:bg-sky-50 hover:text-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 disabled:opacity-40" title={visibleReplyPreviewTarget ? '引用图片发送尚未接入' : '发送图片（最大 10MB）'}><ImagePlus className="h-5 w-5" /></button>
					<button type="button" onClick={/* openLocationCardEditor 先读取系统默认值，成功后再打开当前会话表单。 */ () => void loadLocationCardDefaults().then(/* defaultsLoaded 是默认设置读取结果。 */ defaultsLoaded => { if (defaultsLoaded) setLocationCardOpen(true); })} disabled={sending || locationDefaultsLoading || activeAccount?.runtime_state !== 'online' || Boolean(visibleReplyPreviewTarget)} className="flex h-10 w-10 items-center justify-center rounded-lg text-slate-500 transition hover:bg-sky-50 hover:text-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 disabled:opacity-40" title={visibleReplyPreviewTarget ? '引用位置卡片发送尚未接入' : '发送位置卡片'} aria-label="发送位置卡片">{locationDefaultsLoading ? <Loader2 className="h-5 w-5 animate-spin" /> : <MapPinned className="h-5 w-5" />}</button>
                  </div>
                  <div className="flex items-end gap-3 rounded-2xl border border-slate-200 bg-slate-50 p-2 transition focus-within:border-sky-400 focus-within:ring-2 focus-within:ring-sky-100">
					<textarea ref={replyComposerRef} value={draft} onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setDraft(event.target.value)} rows={2} maxLength={2000}
					  onKeyDown={/* 当前回调使 Enter 与按钮共用同一原生引用发送流程。 */ event => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void handleComposerSend(); } }}
                      onPaste={/* 当前回调识别剪贴板中的图片并进入预览流程。 */ event => {
                        // files 保存剪贴板提供的文件候选列表。
                        const files = Array.from(event.clipboardData?.files || []);
                        // image 保存候选列表中的首张图片。
                        const image = files.find(/* 当前回调判断剪贴板文件是否为图片。 */ file => file.type.startsWith('image/'));
                        if (image) {
                          event.preventDefault();
						  if (visibleReplyPreviewTarget) return;
                          void handlePastedImages(files);
                        }
                      }}
					  disabled={activeAccount?.runtime_state !== 'online'} placeholder={activeAccount?.runtime_state !== 'online' ? '账号离线，暂时无法发送' : visibleReplyPreviewTarget ? `回复 ${visibleReplyPreviewTarget.senderLabel}` : '输入消息，Enter 发送，Shift + Enter 换行，Ctrl + V 粘贴图片'}
                      className="max-h-32 min-h-12 flex-1 resize-none bg-transparent px-2 py-2 text-sm leading-6 outline-none disabled:cursor-not-allowed" />
					<button type="button" title={visibleReplyPreviewTarget ? '发送原生引用回复' : '发送消息'} onClick={/* 当前回调提交普通或原生引用文本。 */ () => void handleComposerSend()} disabled={!draft.trim() || sending || activeAccount?.runtime_state !== 'online'} className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-sky-500 text-white shadow-md shadow-sky-100 transition hover:bg-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-400 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:bg-slate-300 disabled:shadow-none" aria-label="发送消息">
                      {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                    </button>
                  </div>
                </div>
              </>
            ) : (
              <div className="flex flex-1 flex-col items-center justify-center text-center">
                <MessageCircleMore className="h-12 w-12 text-slate-300" />
                <h3 className="mt-4 font-black text-slate-700">选择一个用户开始聊天</h3>
                <p className="mt-1 text-sm text-slate-500">该账号的新消息会实时出现在左侧列表。</p>
              </div>
            )}
          </main>
          {selectedSession && <ConversationOrderPreviewPanel key={`${activeAccountID}:${selectedSession.chat_id}:${selectedSession.buyer_id}`} open={orderPreviewOpen} onClose={/* closeOrderPreview 关闭中窄屏订单抽屉。 */ () => setOrderPreviewOpen(false)} accountID={activeAccountID} chatID={selectedSession.chat_id} buyerName={selectedSession.buyer_name || selectedSession.buyer_id} buyerID={selectedSession.buyer_id} buyerAvatarURL={selectedSession.buyer_avatar_url} revision={conversationOrderTarget.revision} enrichmentOrderID={conversationOrderTarget.enrichmentOrderID} />}
        </div>
      )}

      {pendingImage && <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 p-4" onClick={/* 当前回调点击遮罩取消图片预览。 */ closeImagePreview}>
        <div className="flex max-h-full w-full max-w-2xl flex-col overflow-hidden rounded-2xl bg-white shadow-2xl" onClick={/* 当前回调阻止预览内容点击冒泡到遮罩。 */ event => event.stopPropagation()}>
          <div className="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3">
            <div className="text-sm font-black text-slate-900">发送图片预览</div>
            <button type="button" title="取消" onClick={/* 当前回调关闭图片预览。 */ closeImagePreview} className="rounded-lg p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"><X className="h-5 w-5" /></button>
          </div>
          <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-slate-100 p-4"><img src={pendingImage.url} alt="待发送图片预览" className="max-h-[55vh] max-w-full rounded-xl object-contain" /></div>
          <div className="flex shrink-0 items-center justify-between gap-3 border-t border-slate-200 px-5 py-3">
            <div className="min-w-0 truncate text-xs text-slate-500">{pendingImage.file.name || '粘贴的图片'} · {(pendingImage.file.size / 1024).toFixed(0)} KB</div>
            <div className="flex shrink-0 items-center gap-2">
              <button type="button" onClick={/* 当前回调取消图片发送。 */ closeImagePreview} className="rounded-xl px-4 py-2 text-sm font-bold text-slate-600 transition hover:bg-slate-100">取消</button>
              <button type="button" onClick={/* 当前回调确认发送预览图片。 */ () => void confirmSendImage()} disabled={sending || activeAccount?.runtime_state !== 'online'} className="flex items-center gap-2 rounded-xl bg-sky-500 px-4 py-2 text-sm font-bold text-white shadow-md shadow-sky-100 transition hover:bg-sky-600 disabled:cursor-not-allowed disabled:opacity-50">{sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}发送</button>
            </div>
          </div>
        </div>
      </div>}

	      <Lightbox open={lightboxIndex >= 0} index={Math.max(lightboxIndex, 0)} close={/* 当前回调关闭图片灯箱。 */ () => setLightboxIndex(-1)} slides={imageSlides} />
	      <LocationCardModal open={locationCardOpen} sending={sending} defaults={locationCardDefaults} onClose={/* closeLocationCardEditor 关闭未提交或已成功的位置卡片表单。 */ () => setLocationCardOpen(false)} onSend={handleSendLocationCard} />
	      {eligibleCancellableSellerOrder && <CloseOrderModal accountID={eligibleCancellableSellerOrder.accountID} orderID={eligibleCancellableSellerOrder.orderID} stage={closeEligibleOrderStage || eligibleCancellableSellerOrder.stage} open={cancelOrderOpen} onClose={/* closeCancelOrderModal 关闭顶部取消订单弹窗。 */ () => setCancelOrderOpen(false)} onSuccess={/* cancelOrderSucceeded 保存明确结果并保持成功页可见。 */ message => { setCancelledOrderID(eligibleCancellableSellerOrder.orderID); setCancelledOrderMessage(message); }} />}
	      <span className="sr-only" aria-live="polite">{cancelledOrderMessage}</span>
    </section>
  );
};

export default Chat;
