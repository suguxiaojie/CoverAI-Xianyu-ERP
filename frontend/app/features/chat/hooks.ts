import type React from 'react';
import { useCallback,useEffect,useLayoutEffect,useMemo,useRef,useState } from 'react';
import { emojiURL,renderXianyuText,xianyuEmojis } from '../../../chatEmojis';
import type { AccountDetail,ChatCreditProfile,ChatMessage,ChatSession } from './api';
import { getAccountDetails,getAccountRuntimeStatuses,getChatLocationCardDefaults,getChatMessagePage,getChatSessionPage,getChatUserCredit,getCloseOrderEligibility,markChatRead,recallChatMessage,sendChatImage,sendChatLocationCard,sendChatMessage,setChatSessionPinned } from './api';
import type { ChatLocationCardDefaults,SendLocationCardInput } from './api';
import { publishChatUnreadStatus,subscribeToChatLiveEvents } from './liveEvents';
import { collectChatReadReceipts,filterChatSessions,formatClock,isChatAbortError,isCurrentChatRequest,markOutgoingMessagesReadByIncoming,mergeLiveMessage,mergeOlderMessages,messageTime,prioritizeNavigationSession,sortChatSessionsByPinAndActivity,uniqueCancellableSellerOrder } from './state';
import type { ChatFeatureState,ChatLiveState,SessionsByAccount } from './types';

/** PendingImagePreview 描述等待用户确认的本地图片预览及其资源所有权。 */
type PendingImagePreview = {
  /** file 保存当前预览会话待发送的图片文件。 */
  file: File;
  /** url 保存由浏览器创建、仅在当前预览会话有效的临时地址。 */
  url: string;
};

/** SessionPinOperation 跟踪单个账号会话最新置顶请求的代次和取消责任。 */
type SessionPinOperation = {
  /** generation 是同一账号会话连续操作的递增代次，防止旧响应覆盖新状态。 */
  generation: number;
  /** controller 由该会话下一次置顶操作或 Hook 卸载时取消。 */
  controller: AbortController;
};

/** sessionPinOperationKey 生成账号与精确会话的内部操作隔离键。 */
const sessionPinOperationKey = (accountID: string, chatID: string): string => `${accountID}\u0000${chatID}`;

/** chatSessionSearchDebounceMs 避免每次按键都查询本地完整聊天历史。 */
const chatSessionSearchDebounceMs = 250;

/** chatCreditDwellMs 只在用户确实停留当前会话后启动信用查询，快速切换不访问服务端。 */
const chatCreditDwellMs = 500;

// ChatNavigationTarget 描述订单页传给 Chat 的一次性精确账号、会话和商品上下文。
interface ChatNavigationTarget {
	// accountID 是订单所属卖家账号。
	accountID: string;
	// chatID 是订单持久化的精确聊天会话标识。
	chatID: string;
	// itemID 是订单关联商品，用于拒绝同会话错误商品上下文。
	itemID: string;
}

// chatNavigationTargetFromLocation 只从 /app/chat 的完整查询参数读取可用深链目标。
const chatNavigationTargetFromLocation = (): ChatNavigationTarget | null => {
	if (window.location.pathname !== '/app/chat') return null;
	// parameters 是当前聊天地址携带的查询参数。
	const parameters = new URLSearchParams(window.location.search);
	// target 是去除首尾空白后的账号、会话和商品目标。
	const target: ChatNavigationTarget = {
		accountID: String(parameters.get('account_id') || '').trim(),
		chatID: String(parameters.get('chat_id') || '').trim(),
		itemID: String(parameters.get('item_id') || '').trim(),
	};
	return target.accountID && target.chatID && target.itemID ? target : null;
};

// sessionMatchesNavigationTarget 校验会话同时属于目标账号、精确 chat_id 和订单商品。
const sessionMatchesNavigationTarget = (session: ChatSession | undefined, target: ChatNavigationTarget): session is ChatSession =>
	Boolean(session && session.account_id === target.accountID && session.chat_id === target.chatID && (!session.item_id || session.item_id === target.itemID));

/** Chat Hook 对外暴露的状态、引用和交互动作。 */
export type UseChatResult = ChatFeatureState & {
  /** 待确认发送的图片预览；URL 仅在当前预览会话内有效。 */
  pendingImage: PendingImagePreview | null;
	/** 系统设置页保存、当前弹窗可临时修改的位置卡片默认值。 */
	locationCardDefaults: ChatLocationCardDefaults;
	/** locationDefaultsLoading 表示正在读取最新系统默认值。 */
	locationDefaultsLoading: boolean;
  /** 当前选中的会话 ID。 */
  activeChatID: string;
	/** navigationPriorityChatID 是订单深链临时提升到第一个非置顶位置的会话。 */
	navigationPriorityChatID: string;
	/** closeEligibleOrderID 是后端权威确认仍可取消的当前会话订单号。 */
	closeEligibleOrderID: string;
	/** closeEligibleOrderStage 是后端确认的待付款或已付款待发货阶段。 */
	closeEligibleOrderStage: 'pending_payment' | 'pending_ship' | '';
  /** 当前账号过滤后的会话。 */
  filteredSessions: ChatSession[];
  /** 消息滚动容器引用。 */
  scrollRef: React.MutableRefObject<HTMLDivElement | null>;
  /** 图片文件输入引用。 */
  imageInputRef: React.MutableRefObject<HTMLInputElement | null>;
  /** 更新当前账号。 */
  setActiveAccountID: React.Dispatch<React.SetStateAction<string>>;
  /** 更新当前会话。 */
  setActiveChatID: React.Dispatch<React.SetStateAction<string>>;
	/** 清除订单深链的临时联系人排序提升。 */
	clearNavigationPriority: () => void;
  /** 更新搜索文本。 */
  setSearch: React.Dispatch<React.SetStateAction<string>>;
  /** 更新未读筛选。 */
  setUnreadOnly: React.Dispatch<React.SetStateAction<boolean>>;
  /** 更新消息草稿。 */
  setDraft: React.Dispatch<React.SetStateAction<string>>;
  /** 更新表情选择器状态。 */
  setEmojiOpen: React.Dispatch<React.SetStateAction<boolean>>;
  /** 刷新当前账号会话。 */
  reloadSessions: (accountID: string) => Promise<ChatSession[]>;
  /** 加载更早的联系人。 */
  loadMoreContacts: () => Promise<void>;
  /** 当前账号正在持久化置顶偏好的会话标识集合。 */
  pinningSessionIDs: Set<string>;
  /** 幂等设置当前账号精确会话的持久化置顶状态。 */
  setSessionPinned: (chatID: string, pinned: boolean) => Promise<void>;
  /** 加载更早的消息。 */
  loadOlderMessages: () => Promise<void>;
  /** 根据滚动位置更新自动滚动策略。 */
  handleMessageScroll: () => void;
  /** 发送普通或原生引用文本；返回值表示最新请求是否明确成功。 */
  handleSend: (replyToMessageKey?: string) => Promise<boolean>;
  /** 发送图片消息。 */
  handleImage: (file?: File) => Promise<void>;
  /** 从剪贴板候选文件中选择首张图片进入预览。 */
  handlePastedImages: (files: File[]) => Promise<void>;
  /** 确认发送当前预览图片。 */
  confirmSendImage: () => Promise<void>;
	/** 发送用户在弹窗中确认的位置卡片；返回值表示是否明确成功。 */
	handleSendLocationCard: (draft: Pick<SendLocationCardInput, 'title' | 'description' | 'latitude' | 'longitude'>) => Promise<boolean>;
	/** 读取最新系统位置默认值；成功时返回 true。 */
	loadLocationCardDefaults: () => Promise<boolean>;
  /** 取消当前图片预览并释放临时地址。 */
  closeImagePreview: () => void;
	/** 重试最近一次失败发送；返回值表示重试是否明确成功。 */
	retrySend: () => Promise<boolean>;
	/** 撤回两分钟内已取得平台 ID 的己方消息。 */
	recallMessage: (messageKey: string) => Promise<void>;
  /** 是否存在可重试的发送动作。 */
  retryAvailable: boolean;
  /** 列出指定账号的未读总数。 */
  unreadForAccount: (accountID: string) => number;
  /** 表情资源导出，保持页面兼容入口。 */
  emojiURL: typeof emojiURL;
  /** 闲鱼表情列表导出，保持页面兼容入口。 */
  xianyuEmojis: typeof xianyuEmojis;
  /** 闲鱼文本渲染器导出，保持页面兼容入口。 */
  renderXianyuText: typeof renderXianyuText;
  /** 时间格式化函数导出，保持页面兼容入口。 */
  formatClock: typeof formatClock;
  /** 消息时间格式化函数导出，保持页面兼容入口。 */
  messageTime: typeof messageTime;
};

// RetryTextMessage 保存失败文本和可选引用目标，避免重试时降级成普通消息。
interface RetryTextMessage {
	/** accountID 是首次失败发送所属账号，防止切换账号后误重试。 */
	accountID: string;
	/** chatID 是首次失败发送所属会话，防止引用键跨会话重试。 */
	chatID: string;
	/** text 是首次失败时已规范化的文本。 */
	text: string;
	/** replyToMessageKey 是首次发送时选中的本地引用目标键。 */
	replyToMessageKey?: string;
}

/** 统一管理聊天账号、会话、消息分页、实时连接和发送重试状态。 */
export const useChat = (): UseChatResult => {
  // accounts 保存启用账号及其运行状态。
  const [accounts, setAccounts] = useState<AccountDetail[]>([]);
  // activeAccountID 保存当前选中的账号。
  const [activeAccountID, setActiveAccountID] = useState('');
  // sessionsByAccount 按账号隔离会话列表。
  const [sessionsByAccount, setSessionsByAccount] = useState<SessionsByAccount>({});
  // activeChatID 保存当前选中的会话。
  const [activeChatID, setActiveChatID] = useState('');
	// navigationPriorityChatID 保存订单深链本次页面生命周期内临时提升的会话标识。
	const [navigationPriorityChatID, setNavigationPriorityChatID] = useState('');
	// closeEligibleOrderID 保存后端纯本地资格确认通过的当前会话订单号。
	const [closeEligibleOrderID, setCloseEligibleOrderID] = useState('');
	// closeEligibleOrderStage 保存后端确认的取消阶段，避免只按历史卡片决定退款提示。
	const [closeEligibleOrderStage, setCloseEligibleOrderStage] = useState<'pending_payment' | 'pending_ship' | ''>('');
  // messages 保存当前会话消息。
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  // search 保存会话搜索文本。
  const [search, setSearch] = useState('');
	// searchResults 保存当前账号和最新关键词对应的服务端历史消息搜索结果。
	const [searchResults, setSearchResults] = useState<ChatSession[]>([]);
  // unreadOnly 控制是否仅展示未读会话。
  const [unreadOnly, setUnreadOnly] = useState(false);
  // draft 保存待发送文本。
  const [draft, setDraft] = useState('');
  // pendingImage 保存等待用户确认的本地图片和临时预览地址。
  const [pendingImage, setPendingImage] = useState<PendingImagePreview | null>(null);
	// locationCardDefaults 保存系统设置页最近一次持久化的位置默认表单值。
	const [locationCardDefaults, setLocationCardDefaults] = useState<ChatLocationCardDefaults>({ title: '', description: '', latitude: '', longitude: '' });
	// locationDefaultsLoading 表示位置按钮正在读取最新默认设置。
	const [locationDefaultsLoading, setLocationDefaultsLoading] = useState(false);
  // loading 表示聊天初始数据加载状态。
  const [loading, setLoading] = useState(true);
  // messagesLoading 表示当前会话消息加载状态。
  const [messagesLoading, setMessagesLoading] = useState(false);
  // olderLoading 表示历史消息分页状态。
  const [olderLoading, setOlderLoading] = useState(false);
  // hasOlder 表示当前会话是否还有历史消息。
  const [hasOlder, setHasOlder] = useState(false);
  // historyCursor 保存历史消息分页游标。
  const [historyCursor, setHistoryCursor] = useState<number | undefined>();
  // contactCursors 保存各账号联系人分页游标。
  const [contactCursors, setContactCursors] = useState<Record<string, number | undefined>>({});
  // hasMoreContacts 保存各账号是否还有联系人。
  const [hasMoreContacts, setHasMoreContacts] = useState<Record<string, boolean>>({});
  // contactsLoading 表示联系人分页状态。
  const [contactsLoading, setContactsLoading] = useState(false);
  // pinningSessionKeys 保存当前仍在等待持久化结果的账号会话内部键。
  const [pinningSessionKeys, setPinningSessionKeys] = useState<Set<string>>(new Set());
  // emojiOpen 控制表情选择器显示。
  const [emojiOpen, setEmojiOpen] = useState(false);
  // sending 表示当前是否正在发送消息。
  const [sending, setSending] = useState(false);
  // error 保存聊天页面最近错误。
  const [error, setError] = useState('');
  // liveState 保存 WebSocket 连接状态。
  const [liveState, setLiveState] = useState<ChatLiveState>('connecting');
	// creditProfile 保存当前会话按需读取的买家和卖家信用，不参与左侧会话列表。
	const [creditProfile, setCreditProfile] = useState<ChatCreditProfile | undefined>();
	// creditLoading 表示停留防抖结束后当前信用请求仍在进行。
	const [creditLoading, setCreditLoading] = useState(false);
	// creditError 保存当前信用查询失败或熔断的非敏感提示。
	const [creditError, setCreditError] = useState('');
  // retryText 保存最近失败的文本和原生引用目标。
  const [retryText, setRetryText] = useState<RetryTextMessage | null>(null);
  // retryImage 保存最近失败的图片消息。
  const [retryImage, setRetryImage] = useState<File | null>(null);
  // activeAccountRef 供实时回调读取最新账号。
  const activeAccountRef = useRef('');
	// navigationTargetRef 保存订单页带入且只消费一次的精确聊天目标。
	const navigationTargetRef = useRef<ChatNavigationTarget | null>(chatNavigationTargetFromLocation());
  // activeChatRef 供实时回调读取最新会话。
  const activeChatRef = useRef('');
  // scrollRef 指向消息滚动容器。
  const scrollRef = useRef<HTMLDivElement | null>(null);
  // scrollContextRef 保存滚动上下文。
  const scrollContextRef = useRef({ accountID: '', chatID: '' });
  // shouldScrollToBottomRef 控制新消息是否自动滚到底部。
  const shouldScrollToBottomRef = useRef(true);
  // skipNextMessageScrollRef 防止加载历史消息后跳到底部。
  const skipNextMessageScrollRef = useRef(false);
  // imageInputRef 指向图片文件输入框。
  const imageInputRef = useRef<HTMLInputElement | null>(null);
  // refreshedAccountsRef 防止同一账号重复刷新联系人。
  const refreshedAccountsRef = useRef(new Set<string>());
  // sessionSequence 隔离联系人刷新请求。
  const sessionSequence = useRef(0);
  // sessionController 保存当前联系人请求控制器。
  const sessionController = useRef<AbortController | null>(null);
	// searchSequence 隔离连续输入和账号切换产生的旧搜索响应。
	const searchSequence = useRef(0);
	// searchController 保存当前历史消息搜索请求，下一次输入或卸载时取消。
	const searchController = useRef<AbortController | null>(null);
  // messageSequence 隔离会话切换产生的旧消息响应。
  const messageSequence = useRef(0);
  // messageController 保存当前消息请求控制器。
  const messageController = useRef<AbortController | null>(null);
  // olderSequence 隔离同一会话中被关闭、切换或替换的历史消息分页请求。
  const olderSequence = useRef(0);
  // olderController 保存当前历史消息分页请求，切换会话或卸载时必须由本 Hook 取消。
  const olderController = useRef<AbortController | null>(null);
  // contactSequence 隔离联系人分页产生的旧响应。
  const contactSequence = useRef(0);
  // contactController 保存当前联系人分页控制器。
  const contactController = useRef<AbortController | null>(null);
  // sessionPinOperationsRef 按账号会话键持有最新置顶请求，Hook 卸载时统一取消。
  const sessionPinOperationsRef = useRef(new Map<string, SessionPinOperation>());
  // sendSequence 隔离账号或会话切换产生的旧发送响应。
  const sendSequence = useRef(0);
  // sendController 保存当前消息发送控制器。
  const sendController = useRef<AbortController | null>(null);
	// locationDefaultsController 保存位置默认值请求；重复点击和卸载会取消旧请求。
	const locationDefaultsController = useRef<AbortController | null>(null);
	// closeEligibilitySequence 隔离会话切换或新系统卡片产生的旧资格响应。
	const closeEligibilitySequence = useRef(0);
	// creditSequence 隔离账号或会话切换后迟到的信用响应。
	const creditSequence = useRef(0);
	// creditController 保存当前信用请求；切换会话或卸载时由本 Hook 取消。
	const creditController = useRef<AbortController | null>(null);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => { activeAccountRef.current = activeAccountID; }, [activeAccountID]);
  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => { activeChatRef.current = activeChatID; }, [activeChatID]);
	useEffect(/* releaseLocationDefaultsRequest 登记位置默认值请求的卸载清理。 */ () => {
		return /* cancelLocationDefaultsRequest 在 Chat Hook 卸载时取消尚未完成的设置读取。 */ () => locationDefaultsController.current?.abort();
	}, []);

	useEffect(/* searchHistoricalSessions 对最新账号和关键词执行延迟、本地历史消息搜索。 */ () => {
		// normalizedSearch 是去除首尾空白后的当前关键词。
		const normalizedSearch = search.trim();
		// accountID 固定本轮搜索所属账号，切换账号后旧响应不得写入。
		const accountID = activeAccountID;
		// sequence 是本轮搜索允许更新状态的递增代次。
		const sequence = ++searchSequence.current;
		searchController.current?.abort();
		searchController.current = null;
		setSearchResults([]);
		if (!accountID || !normalizedSearch) return undefined;
		setError('');
		// timer 是二百五十毫秒输入防抖句柄。
		const timer = window.setTimeout(/* runHistoricalSessionSearch 在防抖结束后启动当前本地搜索。 */ () => {
			// controller 只拥有本轮本地搜索请求的取消责任。
			const controller = new AbortController();
			searchController.current = controller;
			void getChatSessionPage(accountID, undefined, { signal: controller.signal, timeoutMs: 15_000 }, false, normalizedSearch).then(
				/* searchPage 是服务端按完整历史消息返回的匹配会话页。 */ searchPage => {
					if (!isCurrentChatRequest(searchSequence.current, sequence, controller.signal) || activeAccountRef.current !== accountID) return;
					setSearchResults(searchPage.sessions);
					setSessionsByAccount(/* currentByAccount 保存搜索结果合并前的会话缓存。 */ currentByAccount => {
						// currentSessions 是当前账号已加载的正常联系人和历史分页结果。
						const currentSessions = currentByAccount[accountID] || [];
						// mergedByChatID 按精确会话标识合并搜索命中项，保证清空搜索后仍能保持已选会话上下文。
						const mergedByChatID = new Map(currentSessions.map(/* session 是当前缓存中的会话摘要。 */ session => [session.chat_id, session]));
						for (const matchedSession /* matchedSession 是当前历史搜索命中的会话摘要。 */ of searchPage.sessions) {
							// existingSession 是同会话已存在的实时摘要；当前打开会话必须继续保持本地已读状态。
							const existingSession = mergedByChatID.get(matchedSession.chat_id);
							mergedByChatID.set(matchedSession.chat_id, existingSession
								? { ...existingSession, ...matchedSession, unread_count: matchedSession.chat_id === activeChatRef.current ? 0 : matchedSession.unread_count }
								: matchedSession);
						}
						return { ...currentByAccount, [accountID]: sortChatSessionsByPinAndActivity([...mergedByChatID.values()]) };
					});
				},
				/* searchError 是本地历史会话搜索失败或取消结果。 */ searchError => {
					if (isCurrentChatRequest(searchSequence.current, sequence, controller.signal) && !isChatAbortError(searchError)) setError(searchError instanceof Error ? searchError.message : '搜索聊天记录失败');
				},
			).finally(/* searchFinished 只清理仍属于当前代次的搜索控制器。 */ () => {
				if (searchController.current === controller) searchController.current = null;
			});
		}, chatSessionSearchDebounceMs);
		return /* historicalSessionSearchCleanup 取消防抖计时和已经发出的旧搜索。 */ () => {
			window.clearTimeout(timer);
			searchController.current?.abort();
		};
	}, [activeAccountID, search]);

  useEffect(/* registerSessionPinCleanup 登记 Hook 卸载时的会话置顶请求取消责任。 */ () => {
    /** cleanupSessionPinOperations 取消全部未完成置顶请求，阻止后续写入已卸载状态。 */
    return () => {
      for (const /* operation 是当前需要由 Hook 统一取消的会话置顶请求。 */ operation of sessionPinOperationsRef.current.values()) operation.controller.abort();
      sessionPinOperationsRef.current.clear();
    };
  }, []);

  /** 刷新指定账号的联系人列表，并丢弃过期响应。 */
  const reloadSessions = useCallback(/* 当前回调封装可复用的交互处理逻辑。 */ async (accountID: string): Promise<ChatSession[]> => {
    // sequence 请求序号。
    const sequence = ++sessionSequence.current;
    sessionController.current?.abort();
    // controller 请求取消控制器。
    const controller = new AbortController();
    sessionController.current = controller;
    try {
      // page 页码。
      const page = await getChatSessionPage(accountID, undefined, { signal: controller.signal }, true);
      if (!isCurrentChatRequest(sessionSequence.current, sequence, controller.signal)) return [];
      // sessions 保留当前打开会话的本地已读状态，避免刷新响应迟到后重新展示未读徽标。
      const sessions = page.sessions.map(/* session 按当前活跃会话覆写已读计数。 */ session => (
        accountID === activeAccountRef.current && session.chat_id === activeChatRef.current
          ? { ...session, unread_count: 0 }
          : session
      ));
      setSessionsByAccount(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, [accountID]: sessions }));
      setContactCursors(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, [accountID]: page.next_cursor }));
      setHasMoreContacts(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, [accountID]: page.has_more }));
      return sessions;
    } catch (/* error 保存会话列表请求的失败原因；仅最新请求可以更新错误状态。 */ error) {
      if (isCurrentChatRequest(sessionSequence.current, sequence, controller.signal) && !isChatAbortError(error)) setError(error instanceof Error ? error.message : '同步会话失败');
      return [];
    }
  }, []);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    // controller 请求取消控制器。
    const controller = new AbortController();
    // load 加载当前数据。
    const load = async (): Promise<void> => {
      setLoading(true);
      try {
        // [details, 解构得到当前 Hook 返回的状态和操作函数。
        const [details, statuses] = await Promise.all([
          getAccountDetails({ signal: controller.signal }),
          getAccountRuntimeStatuses({ signal: controller.signal }),
        ]);
        // withRuntime with运行状态，负责当前功能中的对应处理。
        const withRuntime = details.map(/* 当前回调处理集合中的单个元素。 */ account => ({
          ...account,
          runtime_state: statuses[account.id]?.state || (account.enabled ? 'connecting' : 'disabled'),
          runtime_connected: statuses[account.id]?.connected === true,
        }));
        // enabled 启用状态。
        const enabled = withRuntime.filter(/* 当前回调处理集合中的单个元素。 */ account => account.enabled);
		// sessionPages 会话Pages，负责当前功能中的对应处理。
		const sessionPages = await Promise.all(enabled.map(/* 当前回调处理集合中的单个元素。 */ async account => [account.id, await getChatSessionPage(account.id, undefined, { signal: controller.signal })] as const));
        if (controller.signal.aborted) return;
		// sessionsForAccounts 是允许按深链补入精确会话的账号会话映射。
		const sessionsForAccounts = Object.fromEntries(sessionPages.map(/* id、page 是当前账号及其初始会话页。 */ ([id, page]) => [id, page.sessions])) as SessionsByAccount;
		// navigationTarget 是订单页当前尚未消费的精确聊天目标。
		const navigationTarget = navigationTargetRef.current;
		// preferredAccountID 优先使用有效深链账号，其次恢复本地账号偏好。
		let preferredAccountID = '';
		if (navigationTarget && enabled.some(/* account 是当前判断是否允许打开的启用账号。 */ account => account.id === navigationTarget.accountID)) {
			// targetSessions 是目标账号当前已经加载的会话摘要。
			const targetSessions = sessionsForAccounts[navigationTarget.accountID] || [];
			// targetSession 是初始联系人页中精确命中的订单商品会话。
			let targetSession = targetSessions.find(/* session 是当前与订单深链比较的会话摘要。 */ session => sessionMatchesNavigationTarget(session, navigationTarget));
			if (!targetSession) {
				// targetPage 直接读取精确 chat_id，避免目标是较早联系人时依赖当前分页或昵称搜索。
				const targetPage = await getChatMessagePage(navigationTarget.accountID, navigationTarget.chatID, undefined, undefined, { signal: controller.signal });
				if (controller.signal.aborted) return;
				if (sessionMatchesNavigationTarget(targetPage.session, navigationTarget)) targetSession = targetPage.session;
			}
			if (targetSession) {
				sessionsForAccounts[navigationTarget.accountID] = sortChatSessionsByPinAndActivity([targetSession, ...targetSessions.filter(/* session 是目标账号除精确会话之外的摘要。 */ session => session.chat_id !== targetSession?.chat_id)]);
				preferredAccountID = navigationTarget.accountID;
				setActiveChatID(navigationTarget.chatID);
				setNavigationPriorityChatID(navigationTarget.chatID);
				refreshedAccountsRef.current.add(navigationTarget.accountID);
				navigationTargetRef.current = null;
				window.history.replaceState({}, '', '/app/chat');
			} else {
				setError('未找到该订单对应的商品会话');
			}
		}
        setAccounts(enabled);
		setSessionsByAccount(sessionsForAccounts);
        setContactCursors(Object.fromEntries(sessionPages.map(/* 当前回调处理集合中的单个元素。 */ ([id, page]) => [id, page.next_cursor])));
        setHasMoreContacts(Object.fromEntries(sessionPages.map(/* 当前回调处理集合中的单个元素。 */ ([id, page]) => [id, page.has_more])));
		// stored 已保存数据。
		const stored = window.localStorage.getItem('ydisks.chat.account.v1') || '';
		// first 首项。
		const first = preferredAccountID || (enabled.some(/* 当前回调处理集合中的单个元素。 */ account => account.id === stored) ? stored : enabled[0]?.id || '');
        setActiveAccountID(first);
      } catch (/* loadError 表示加载错误。 */ loadError) {
        if (!controller.signal.aborted) setError(loadError instanceof Error ? loadError.message : '加载聊天数据失败');
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    };
    void load();
    return /* 当前回调处理用户交互或异步状态变化。 */ () => controller.abort();
  }, []);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    // disposed disposed，负责当前功能中的对应处理。
    let disposed = false;
    // timer 定时器。
    let timer = 0;
    // controller 请求取消控制器。
    let controller: AbortController | null = null;
    // poll 轮询函数。
    const poll = async (): Promise<void> => {
      controller = new AbortController();
      try {
        // statuses statuses，负责当前功能中的对应处理。
        const statuses = await getAccountRuntimeStatuses({ signal: controller.signal, timeoutMs: 10_000 });
        if (!disposed) setAccounts(/* 当前回调处理集合中的单个元素。 */ current => current.map(/* 当前回调处理集合中的单个元素。 */ account => ({
          ...account,
          runtime_state: statuses[account.id]?.state || account.runtime_state,
          runtime_connected: statuses[account.id]?.connected ?? account.runtime_connected,
        })));
      } catch {
        // WebSocket 拥有独立的可见状态，短暂轮询失败不清除已加载会话。
      } finally {
        if (!disposed) timer = window.setTimeout(poll, 3_000);
      }
    };
    timer = window.setTimeout(poll, 3_000);
    return /* 当前回调处理用户交互或异步状态变化。 */ () => {
      disposed = true;
      window.clearTimeout(timer);
      controller?.abort();
    };
  }, []);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    if (!activeAccountID) return;
    window.localStorage.setItem('ydisks.chat.account.v1', activeAccountID);
    // sessions 会话列表。
    const sessions = sessionsByAccount[activeAccountID] || [];
    setActiveChatID(/* 当前回调处理集合中的单个元素。 */ current => sessions.some(/* 当前回调处理集合中的单个元素。 */ session => session.chat_id === current) ? current : sessions[0]?.chat_id || '');
  }, [activeAccountID, sessionsByAccount]);

  useEffect(/* 当前副作用将所有已加载账号会话的未读聚合状态回传给应用壳，红点只能在没有任何未读时消失。 */ () => {
    // loadedAccountCount 保存已成功写入会话列表状态的账号数量；初始请求尚未完成或失败时不得错误发布无未读。
    const loadedAccountCount = Object.keys(sessionsByAccount).length;
    if (loading || (accounts.length > 0 && loadedAccountCount === 0)) return;
    // hasUnreadChatMessage 保存当前所有已加载会话是否至少存在一条未读消息。
    const hasUnreadChatMessage = Object.values(sessionsByAccount).some(/* sessions 保存当前账号的会话列表。 */ sessions => sessions.some(/* session 保存当前参与未读聚合判断的会话。 */ session => session.unread_count > 0));
    publishChatUnreadStatus(hasUnreadChatMessage);
  }, [accounts.length, loading, sessionsByAccount]);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    if (!activeAccountID || refreshedAccountsRef.current.has(activeAccountID)) return;
    refreshedAccountsRef.current.add(activeAccountID);
    void reloadSessions(activeAccountID);
  }, [activeAccountID, reloadSessions]);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    if (!activeAccountID || !activeChatID) {
      // 无有效会话时也推进消息与历史分页代次，避免晚到响应恢复已清空的消息列表。
      messageSequence.current += 1;
      olderSequence.current += 1;
      olderController.current?.abort();
      skipNextMessageScrollRef.current = false;
      setMessages([]);
      return;
    }
    // sequence 请求序号。
    const sequence = ++messageSequence.current;
    messageController.current?.abort();
    // controller 请求取消控制器。
    const controller = new AbortController();
    messageController.current = controller;
    setMessagesLoading(true);
    void getChatMessagePage(activeAccountID, activeChatID, undefined, undefined, { signal: controller.signal }).then(/* 当前回调处理用户交互或异步状态变化。 */ page => {
      if (!isCurrentChatRequest(messageSequence.current, sequence, controller.signal)) return;
      // readReceipts 只确认可被平台接受的普通入站消息。
      const readReceipts = collectChatReadReceipts(page.messages, activeChatID);
      void markChatRead(activeAccountID, activeChatID, readReceipts, { signal: controller.signal });
      setMessages(page.messages);
      setHasOlder(page.has_more);
      setHistoryCursor(page.next_cursor);
      if (page.session) setSessionsByAccount(/* 当前回调处理集合中的单个元素。 */ current => ({ ...current, [activeAccountID]: (current[activeAccountID] || []).map(/* 当前回调处理集合中的单个元素。 */ session => session.chat_id === page.session?.chat_id ? page.session! : session) }));
      setSessionsByAccount(/* 当前回调处理集合中的单个元素。 */ current => ({ ...current, [activeAccountID]: (current[activeAccountID] || []).map(/* 当前回调处理集合中的单个元素。 */ session => session.chat_id === activeChatID ? { ...session, unread_count: 0 } : session) }));
    }).catch(/* 当前回调处理用户交互或异步状态变化。 */ loadError => {
      if (isCurrentChatRequest(messageSequence.current, sequence, controller.signal) && !isChatAbortError(loadError)) setError(loadError instanceof Error ? loadError.message : '加载消息失败');
    }).finally(/* 当前回调处理用户交互或异步状态变化。 */ () => {
      if (isCurrentChatRequest(messageSequence.current, sequence, controller.signal)) setMessagesLoading(false);
    });
    return /* 当前回调处理用户交互或异步状态变化。 */ () => controller.abort();
  }, [activeAccountID, activeChatID]);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    contactController.current?.abort();
    contactSequence.current += 1;
  }, [activeAccountID]);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    sendController.current?.abort();
    sendSequence.current += 1;
	setSending(false);
	setError('');
	setRetryText(null);
	setRetryImage(null);
  }, [activeAccountID, activeChatID]);

  useEffect(/* 当前回调使会话切换前创建的图片预览失效。 */ () => {
    // 会话或账号切换后不得把旧预览发送到新会话。
    setPendingImage(null);
  }, [activeAccountID, activeChatID]);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    // 会话切换时取消历史分页；新的消息加载拥有独立控制器，不能复用这个低优先级请求。
    olderSequence.current += 1;
    olderController.current?.abort();
    skipNextMessageScrollRef.current = false;
  }, [activeAccountID, activeChatID]);

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => /* 当前回调同步 React 副作用和资源生命周期。 */ () => {
    sessionController.current?.abort();
		searchSequence.current += 1;
		searchController.current?.abort();
    messageController.current?.abort();
    olderSequence.current += 1;
    olderController.current?.abort();
    contactController.current?.abort();
    sendController.current?.abort();
		creditSequence.current += 1;
		creditController.current?.abort();
  }, []);

  useEffect(/* 当前回调负责图片预览临时地址的生命周期清理。 */ () => {
    // preview 保存本次渲染仍然拥有的图片预览；状态替换或组件卸载时释放地址。
    const preview = pendingImage;
    return /* 当前回调释放已不再使用的图片对象地址。 */ () => {
      if (preview) URL.revokeObjectURL(preview.url);
    };
  }, [pendingImage]);

  /** 加载当前会话更早消息并保持滚动位置。 */
  const loadOlderMessages = useCallback(/* 当前回调封装可复用的交互处理逻辑。 */ async (): Promise<void> => {
    if (!activeAccountID || !activeChatID || olderLoading || !hasOlder) return;
    // container 容器。
    const container = scrollRef.current;
    // previousHeight 上一项高度，负责当前功能中的对应处理。
    const previousHeight = container?.scrollHeight || 0;
    // sequence 请求序号。
    const sequence = messageSequence.current;
    // olderRequestSequence 标识本次历史分页，连续翻页、切换会话或卸载后旧分页不能写入状态。
    const olderRequestSequence = ++olderSequence.current;
    olderController.current?.abort();
    // controller 取消当前历史消息请求。
    const controller = new AbortController();
    olderController.current = controller;
    skipNextMessageScrollRef.current = true;
    setOlderLoading(true);
    setError('');
    try {
      // oldestID 最早标识，负责当前功能中的对应处理。
      const oldestID = messages[0]?.id;
      // page 页码。
      const page = await getChatMessagePage(activeAccountID, activeChatID, historyCursor, oldestID, { signal: controller.signal });
      if (!isCurrentChatRequest(messageSequence.current, sequence, controller.signal) || olderRequestSequence !== olderSequence.current) return;
      setMessages(/* 当前回调处理用户交互或异步状态变化。 */ current => mergeOlderMessages(current, page.messages));
      setHasOlder(page.has_more);
      setHistoryCursor(page.next_cursor);
      requestAnimationFrame(/* 当前回调处理用户交互或异步状态变化。 */ () => {
        if (olderRequestSequence === olderSequence.current && !controller.signal.aborted && container) {
          container.scrollTop += container.scrollHeight - previousHeight;
        }
      });
    } catch (/* loadError 表示加载错误。 */ loadError) {
      skipNextMessageScrollRef.current = false;
      if (!isChatAbortError(loadError)) setError(loadError instanceof Error ? loadError.message : '加载历史消息失败');
    } finally {
      if (olderRequestSequence === olderSequence.current) setOlderLoading(false);
    }
  }, [activeAccountID, activeChatID, hasOlder, historyCursor, messages, olderLoading]);

  /** handleLiveMessage 将应用壳唯一连接发布的消息同步到当前聊天页的会话、消息和已读状态。 */
  const handleLiveMessage = useCallback(/* 当前回调只处理类型化聊天消息，不解析原始 WebSocket 帧。 */ (message: ChatMessage): void => {
    // accountID 保存当前实时消息所属账号，用于隔离不同闲鱼账号的会话状态。
    const accountID = message.account_id;
    setSessionsByAccount(/* 当前回调在对应账号会话列表中合并最新消息与未读计数。 */ current => {
      // rows 保存当前账号已有的会话行。
      const rows = current[accountID] || [];
      // found 表示推送消息是否能匹配当前已加载会话；缺失时异步刷新该账号会话。
      const found = rows.some(/* row 保存当前参与会话匹配的联系人行。 */ row => row.chat_id === message.chat_id);
      if (!found) {
        void reloadSessions(accountID);
        return current;
      }
      return { ...current, [accountID]: sortChatSessionsByPinAndActivity(rows.map(/* row 保存当前待合并实时消息的联系人行。 */ row => row.chat_id === message.chat_id ? {
        ...row,
        last_message: message.content,
        last_message_at: message.sent_at,
        unread_count: message.direction === 'incoming' && message.message_type !== 'system' && (activeAccountRef.current !== accountID || activeChatRef.current !== message.chat_id) ? row.unread_count + 1 : row.unread_count,
      } : row)) };
    });
    if (activeAccountRef.current === accountID && activeChatRef.current === message.chat_id) {
      setMessages(/* 当前回调合并实时消息并同步后续入站消息确认的出站已读状态。 */ current => markOutgoingMessagesReadByIncoming(mergeLiveMessage(current, message), message));
      if (message.direction === 'incoming' && message.message_type !== 'system') {
        // readReceipts 为当前实时消息生成平台要求的会话读取回执。
        const readReceipts = collectChatReadReceipts([message], message.chat_id);
        void markChatRead(accountID, message.chat_id, readReceipts);
      }
    }
  }, [reloadSessions]);

  useEffect(/* 当前副作用订阅认证应用壳唯一连接发布的事件，聊天页卸载时取消订阅而不关闭全局连接。 */ () => {
    /** handleLiveEvent 根据事件类型分别同步连接状态或消息内容。 */
    const handleLiveEvent = (event: Parameters<Parameters<typeof subscribeToChatLiveEvents>[0]>[0]): void => {
      if (event.type === 'connection') {
        setLiveState(event.state);
        return;
      }
      handleLiveMessage(event.message);
    };
    return subscribeToChatLiveEvents(handleLiveEvent);
  }, [handleLiveMessage]);

  /** 根据滚动位置决定新消息是否自动滚到底部。 */
  const handleMessageScroll = useCallback(/* 当前回调封装可复用的交互处理逻辑。 */ () => {
    // container 容器。
    const container = scrollRef.current;
    if (!container) return;
    // distanceFromBottom 距离FromBottom，负责当前功能中的对应处理。
    const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
    shouldScrollToBottomRef.current = distanceFromBottom <= 48;
  }, []);

	/** acceptSentMessage 合并用户已成功发送的消息，并让下一次布局更新无条件滚到当前会话底部。 */
	const acceptSentMessage = useCallback(/* sentMessage 是服务端确认成功且可立即回显的当前会话消息。 */ (sentMessage: ChatMessage): void => {
		shouldScrollToBottomRef.current = true;
		setMessages(/* currentMessages 是发送成功前当前会话已经加载的消息。 */ currentMessages => mergeLiveMessage(currentMessages, sentMessage));
	}, []);

  useLayoutEffect(/* 当前回调处理用户交互或异步状态变化。 */ () => {
    // contextChanged 上下文Changed，负责当前功能中的对应处理。
    const contextChanged = scrollContextRef.current.accountID !== activeAccountID || scrollContextRef.current.chatID !== activeChatID;
    scrollContextRef.current = { accountID: activeAccountID, chatID: activeChatID };
    if (contextChanged) shouldScrollToBottomRef.current = true;
    // container 容器。
    const container = scrollRef.current;
    if (!container) return;
    if (skipNextMessageScrollRef.current) {
      skipNextMessageScrollRef.current = false;
      return;
    }
    if (messagesLoading || shouldScrollToBottomRef.current) container.scrollTop = container.scrollHeight;
  }, [activeAccountID, activeChatID, messages, messagesLoading]);

  // activeAccount 当前状态账号，负责当前功能中的对应处理。
  const activeAccount = accounts.find(/* 当前回调处理集合中的单个元素。 */ account => account.id === activeAccountID);
  // activeSessions 当前状态会话列表，负责当前功能中的对应处理。
  const activeSessions = sessionsByAccount[activeAccountID] || [];
  // selectedSession 处理当前选择（ed会话）。
  const selectedSession = activeSessions.find(/* 当前回调处理集合中的单个元素。 */ session => session.chat_id === activeChatID);

	useEffect(/* loadCurrentConversationCredit 只为停留超过五百毫秒的当前会话按需读取信用。 */ () => {
		// sequence 是本轮信用响应允许写入当前会话的递增代次。
		const sequence = ++creditSequence.current;
		creditController.current?.abort();
		creditController.current = null;
		setCreditProfile(undefined);
		setCreditLoading(false);
		setCreditError('');
		// accountID、chatID 和 buyerID 固定本轮请求上下文，切换后旧响应不得覆盖。
		const [accountID, chatID, buyerID] = [activeAccountID, activeChatID, selectedSession?.buyer_id || ''];
		if (!accountID || !chatID || !buyerID || buyerID === '1400') return undefined;
		// timer 是当前会话五百毫秒停留防抖句柄。
		const timer = window.setTimeout(/* fetchCurrentCredit 在用户仍停留当前会话时启动单个买家查询。 */ () => {
			// controller 只拥有本轮信用 HTTP 请求的取消责任。
			const controller = new AbortController();
			creditController.current = controller;
			setCreditLoading(true);
			void getChatUserCredit(accountID, buyerID, { signal: controller.signal, timeoutMs: 35_000 }).then(
				/* profile 是服务端已完成缓存、限速和归属校验的信用摘要。 */ profile => {
					if (!isCurrentChatRequest(creditSequence.current, sequence, controller.signal) || activeAccountRef.current !== accountID || activeChatRef.current !== chatID || profile.user_id !== buyerID) return;
					setCreditProfile(profile);
					setCreditError('');
				},
				/* creditRequestError 是取消、平台失败或冷却结果；只有最新会话显示提示。 */ creditRequestError => {
					if (!isCurrentChatRequest(creditSequence.current, sequence, controller.signal) || isChatAbortError(creditRequestError)) return;
					setCreditError(creditRequestError instanceof Error ? creditRequestError.message : '信用信息暂未获取');
				},
			).finally(/* creditRequestFinished 只收口仍属于当前代次的加载状态。 */ () => {
				if (isCurrentChatRequest(creditSequence.current, sequence, controller.signal)) setCreditLoading(false);
				if (creditController.current === controller) creditController.current = null;
			});
		}, chatCreditDwellMs);
		return /* currentCreditCleanup 取消尚未触发的防抖或已经发出的旧会话请求。 */ () => {
			window.clearTimeout(timer);
			creditController.current?.abort();
		};
	}, [activeAccountID, activeChatID, selectedSession?.buyer_id]);
	// cancellableCloseOrder 是当前聊天卡片推导出的唯一待付款或已付款待发货候选，只作为后端资格查询输入。
	const cancellableCloseOrder = useMemo(/* currentCancellableOrder 拒绝零笔或多笔聊天候选。 */ () => uniqueCancellableSellerOrder(messages), [messages]);

	useEffect(/* verifyCloseOrderEligibility 使用订单表、聊天角色和幂等的统一后端门禁控制顶部入口。 */ () => {
		// sequence 是本轮资格响应允许更新当前会话的请求代次。
		const sequence = ++closeEligibilitySequence.current;
		// controller 在候选、账号或会话变化时取消旧资格请求。
		const controller = new AbortController();
		setCloseEligibleOrderID('');
		setCloseEligibleOrderStage('');
		if (!cancellableCloseOrder || cancellableCloseOrder.accountID !== activeAccountID || cancellableCloseOrder.messageKey === '' || !activeChatID) {
			return /* emptyEligibilityCleanup 当前没有可取消资格请求。 */ () => controller.abort();
		}
		void getCloseOrderEligibility(cancellableCloseOrder.accountID, cancellableCloseOrder.orderID, { signal: controller.signal }).then(
			// result 是后端纯本地资格结果；只有当前代次和精确候选仍匹配时才显示按钮。
			result => {
				if (controller.signal.aborted || closeEligibilitySequence.current !== sequence) return;
				if (result.eligible && result.account_id === cancellableCloseOrder.accountID && result.order_id === cancellableCloseOrder.orderID) {
					setCloseEligibleOrderID(result.order_id);
					setCloseEligibleOrderStage(result.stage === 'pending_ship' ? 'pending_ship' : result.stage === 'pending_payment' ? 'pending_payment' : cancellableCloseOrder.stage);
				}
			},
			// eligibilityError 只让按钮保持隐藏；资格查询失败不能放宽为可关单。
			() => undefined,
		);
		return /* closeEligibilityCleanup 取消旧会话资格请求并拒绝晚到响应。 */ () => controller.abort();
	}, [activeAccountID, activeChatID, cancellableCloseOrder]);
  // filteredSessions 过滤后的会话列表，负责当前功能中的对应处理。
	const filteredSessions = useMemo(/* visibleChatSessions 对订单深链目标执行一次临时非置顶优先，其余筛选顺序保持不变。 */ () => prioritizeNavigationSession(filterChatSessions(search.trim() ? searchResults : activeSessions, '', unreadOnly), navigationPriorityChatID), [activeSessions, navigationPriorityChatID, search, searchResults, unreadOnly]);
	// clearNavigationPriority 在用户主动切换账号或会话后恢复正常时间排序。
	const clearNavigationPriority = useCallback(/* clearOrderNavigationPriority 清除订单深链临时排序标识。 */ (): void => setNavigationPriorityChatID(''), []);
  // pinningSessionIDs 从全部账号请求键中派生当前账号正在保存的会话标识。
  const pinningSessionIDs = useMemo(/* currentPinningSessions 隔离非当前账号的同 chat_id 请求。 */ () => {
    // prefix 是当前账号在内部操作键中的完整前缀。
    const prefix = `${activeAccountID}\u0000`;
    return new Set([...pinningSessionKeys]
      .filter(/* operationKey 是当前检查是否属于活跃账号的置顶请求键。 */ operationKey => operationKey.startsWith(prefix))
      .map(/* operationKey 去除账号前缀后返回精确会话标识。 */ operationKey => operationKey.slice(prefix.length)));
  }, [activeAccountID, pinningSessionKeys]);
  // unreadForAccount unreadFor账号，负责当前功能中的对应处理。
  const unreadForAccount = useCallback(/* 当前回调处理集合中的单个元素。 */ (accountID: string) => (sessionsByAccount[accountID] || []).reduce(/* 当前回调处理集合中的单个元素。 */ (sum, session) => sum + session.unread_count, 0), [sessionsByAccount]);

  /** setSessionPinned 持久化当前账号会话置顶偏好，并仅在同一操作代次仍为最新时更新本地列表。 */
  const setSessionPinned = useCallback(/* chatID、pinned 是用户点击图钉时的精确会话和目标状态。 */ async (chatID: string, pinned: boolean): Promise<void> => {
    // accountID 捕获用户点击时的账号，切换账号后响应仍只能更新原账号列表。
    const accountID = activeAccountRef.current.trim();
    // normalizedChatID 是去除空白后交给版本化 API 的会话标识。
    const normalizedChatID = chatID.trim();
    if (!accountID || !normalizedChatID) return;
    // operationKey 隔离不同账号及会话的并行置顶请求。
    const operationKey = sessionPinOperationKey(accountID, normalizedChatID);
    // previousOperation 是同一账号会话尚未完成的旧请求，新操作会取消它。
    const previousOperation = sessionPinOperationsRef.current.get(operationKey);
    previousOperation?.controller.abort();
    // generation 是同一账号会话的最新操作代次。
    const generation = (previousOperation?.generation || 0) + 1;
    // controller 只属于本次置顶请求，下一次同会话操作或卸载会取消它。
    const controller = new AbortController();
    sessionPinOperationsRef.current.set(operationKey, { generation, controller });
    setPinningSessionKeys(/* currentKeys 是当前其他会话仍在运行的置顶请求键。 */ currentKeys => new Set(currentKeys).add(operationKey));
    setError('');
    try {
      // result 是服务端确认持久化后的账号、会话和置顶状态。
      const result = await setChatSessionPinned(accountID, normalizedChatID, pinned, { signal: controller.signal });
      // currentOperation 是响应返回时同一账号会话已登记的最新操作。
      const currentOperation = sessionPinOperationsRef.current.get(operationKey);
      if (controller.signal.aborted || currentOperation?.generation !== generation) return;
      if (result.account_id !== accountID || result.chat_id !== normalizedChatID) throw new Error('会话置顶响应上下文不一致');
      setSessionsByAccount(/* currentByAccount 是服务端确认前的全部账号会话列表。 */ currentByAccount => {
        // rows 是用户点击时所属账号的当前会话列表。
        const rows = currentByAccount[accountID] || [];
        // target 是需要更新持久化状态的本地会话；分页已移出时可能为空。
        const target = rows.find(/* row 是当前匹配精确会话标识的本地会话。 */ row => row.chat_id === normalizedChatID);
        if (!target) return currentByAccount;
        // updatedTarget 是应用服务端权威结果后的会话摘要。
        const updatedTarget = { ...target, is_pinned: result.pinned };
        // remainingRows 是排除目标后仍保留服务端置顶顺序的其他会话。
        const remainingRows = rows.filter(/* row 是当前判断是否保留的其他会话。 */ row => row.chat_id !== normalizedChatID);
        // nextRows 在新置顶时把目标放到置顶组第一，取消时交给活跃度排序。
        const nextRows = result.pinned ? [updatedTarget, ...remainingRows] : [...remainingRows, updatedTarget];
        return { ...currentByAccount, [accountID]: sortChatSessionsByPinAndActivity(nextRows) };
      });
    } catch (/* pinError 是持久化置顶失败、取消或响应上下文不一致错误。 */ pinError) {
      // currentOperation 是失败时用于拒绝旧代次更新用户错误的最新操作。
      const currentOperation = sessionPinOperationsRef.current.get(operationKey);
      if (currentOperation?.generation === generation && !isChatAbortError(pinError)) setError(pinError instanceof Error ? pinError.message : '保存会话置顶失败');
    } finally {
      // currentOperation 是收口时用于判断本次请求是否仍为最新代次的操作。
      const currentOperation = sessionPinOperationsRef.current.get(operationKey);
      if (currentOperation?.generation === generation) {
        sessionPinOperationsRef.current.delete(operationKey);
        setPinningSessionKeys(/* currentKeys 是完成前所有运行中置顶请求键。 */ currentKeys => {
          // nextKeys 是删除当前已完成操作后的请求键集合。
          const nextKeys = new Set(currentKeys);
          nextKeys.delete(operationKey);
          return nextKeys;
        });
      }
    }
  }, []);

  /** 加载当前账号下一页联系人。 */
  const loadMoreContacts = useCallback(/* 当前回调封装可复用的交互处理逻辑。 */ async (): Promise<void> => {
    if (!activeAccountID || contactsLoading || !hasMoreContacts[activeAccountID]) return;
    // sequence 请求序号。
    const sequence = ++contactSequence.current;
    contactController.current?.abort();
    // controller 请求取消控制器。
    const controller = new AbortController();
    contactController.current = controller;
    // accountID 账号标识。
    const accountID = activeAccountID;
    setContactsLoading(true);
    setError('');
    try {
      // page 页码。
      const page = await getChatSessionPage(accountID, contactCursors[accountID], { signal: controller.signal }, true);
      if (!isCurrentChatRequest(contactSequence.current, sequence, controller.signal)) return;
      setSessionsByAccount(/* current 是分页刷新前所有账号会话，当前账号用服务端最新排序页替换。 */ current => ({ ...current, [accountID]: page.sessions }));
      setContactCursors(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, [accountID]: page.next_cursor }));
      setHasMoreContacts(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, [accountID]: page.has_more }));
    } catch (/* loadError 表示加载错误。 */ loadError) {
      if (isCurrentChatRequest(contactSequence.current, sequence, controller.signal) && !isChatAbortError(loadError)) setError(loadError instanceof Error ? loadError.message : '加载历史联系人失败');
    } finally {
      if (isCurrentChatRequest(contactSequence.current, sequence, controller.signal)) setContactsLoading(false);
    }
  }, [activeAccountID, contactCursors, contactsLoading, hasMoreContacts]);

  /** 发送普通或原生引用文本，并记录保留引用键的失败重试数据。 */
  const sendText = useCallback(/* 当前回调封装普通和原生引用文本的共享发送流程。 */ async (text: string, replyToMessageKey: string | undefined, rememberRetry: boolean): Promise<boolean> => {
    if (!selectedSession || !activeAccountID || sending) return false;
    // sequence 请求序号。
    const sequence = ++sendSequence.current;
    sendController.current?.abort();
    // controller 请求取消控制器。
    const controller = new AbortController();
    sendController.current = controller;
    setSending(true);
    setError('');
    try {
      // result 处理结果。
      const result = await sendChatMessage({ account_id: activeAccountID, chat_id: selectedSession.chat_id, buyer_id: selectedSession.buyer_id, buyer_name: selectedSession.buyer_name, item_id: selectedSession.item_id, item_title: selectedSession.item_title, text, reply_to_message_key: replyToMessageKey || undefined }, { signal: controller.signal });
      if (!isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) return false;
      setDraft('');
      setRetryText(null);
      acceptSentMessage(result.message);
      return true;
    } catch (/* sendError 表示发送错误。 */ sendError) {
      if (isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) {
        if (rememberRetry) setRetryText({ accountID: activeAccountID, chatID: selectedSession.chat_id, text, replyToMessageKey });
        if (!isChatAbortError(sendError)) setError(sendError instanceof Error ? sendError.message : '消息发送失败');
      }
      return false;
    } finally {
      if (isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) setSending(false);
    }
  }, [acceptSentMessage, activeAccountID, selectedSession, sending]);

  /** 发送图片消息并记录失败重试数据。 */
  const sendImage = useCallback(/* 当前回调封装可复用的交互处理逻辑。 */ async (file: File, rememberRetry: boolean): Promise<void> => {
    if (!selectedSession || !activeAccountID || sending) return;
    // sequence 请求序号。
    const sequence = ++sendSequence.current;
    sendController.current?.abort();
    // controller 请求取消控制器。
    const controller = new AbortController();
    sendController.current = controller;
    setSending(true);
    setError('');
    try {
      // result 处理结果。
      const result = await sendChatImage({ account_id: activeAccountID, chat_id: selectedSession.chat_id, buyer_id: selectedSession.buyer_id, buyer_name: selectedSession.buyer_name, buyer_avatar_url: selectedSession.buyer_avatar_url, item_id: selectedSession.item_id, item_title: selectedSession.item_title, image: file }, { signal: controller.signal });
      if (!isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) return;
      setRetryImage(null);
      acceptSentMessage(result.message);
    } catch (/* sendError 表示发送错误。 */ sendError) {
      if (isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) {
        if (rememberRetry) setRetryImage(file);
        if (!isChatAbortError(sendError)) setError(sendError instanceof Error ? sendError.message : '图片发送失败');
      }
    } finally {
      if (isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) {
        setSending(false);
        if (imageInputRef.current) imageInputRef.current.value = '';
      }
    }
  }, [acceptSentMessage, activeAccountID, selectedSession, sending]);

  /** 处理普通或原生引用文本的发送按钮和 Enter 快捷键。 */
  const handleSend = useCallback(/* 当前回调从撰写区提交文本和可选引用目标。 */ async (replyToMessageKey?: string): Promise<boolean> => {
    // text 文本。
    const text = draft.trim();
    if (!text || !selectedSession || !activeAccountID || sending) return false;
    return sendText(text, replyToMessageKey, true);
  }, [activeAccountID, draft, selectedSession, sendText, sending]);

  /** 处理图片选择并进入确认预览，不直接触发平台发送。 */
  const handleImage = useCallback(/* 当前回调接收文件选择结果并创建图片预览。 */ async (file?: File): Promise<void> => {
    if (!file || !selectedSession || !activeAccountID || sending) return;
    if (!file.type.startsWith('image/')) {
      setError('仅支持粘贴/发送图片文件');
      return;
    }
    setError('');
    setPendingImage({ file, url: URL.createObjectURL(file) });
  }, [activeAccountID, selectedSession, sending]);

  /** 从剪贴板文件中选择首张图片，其余文件仍由原生文本粘贴流程处理。 */
  const handlePastedImages = useCallback(/* 当前回调从剪贴板候选文件中筛选图片。 */ async (files: File[]): Promise<void> => {
    // image 保存剪贴板中的首张图片，只有图片才会阻止原生文本粘贴。
    const image = files.find(/* 当前回调判断文件是否为图片。 */ file => file.type.startsWith('image/'));
    if (image) await handleImage(image);
  }, [handleImage]);

  /** 关闭图片预览并清空文件输入，使同一文件可以再次触发选择事件。 */
  const closeImagePreview = useCallback(/* 当前回调关闭预览并清理文件输入。 */ (): void => {
    setPendingImage(null);
    if (imageInputRef.current) imageInputRef.current.value = '';
  }, []);

  /** 确认发送当前预览图片，发送流程沿用已有请求代次和失败重试保护。 */
  const confirmSendImage = useCallback(/* 当前回调提交用户确认的图片预览。 */ async (): Promise<void> => {
    if (!pendingImage || !selectedSession || !activeAccountID || sending) return;
    // file 保存用户确认后要提交的平台图片文件。
    const file = pendingImage.file;
    setPendingImage(null);
    await sendImage(file, true);
  }, [activeAccountID, pendingImage, selectedSession, sendImage, sending]);

	/** handleSendLocationCard 复用当前发送代次提交结构化位置卡片，切换会话后丢弃晚到响应。 */
	const handleSendLocationCard = useCallback(/* locationDraft 是用户在弹窗中最终确认的标题、说明和坐标。 */ async (locationDraft: Pick<SendLocationCardInput, 'title' | 'description' | 'latitude' | 'longitude'>): Promise<boolean> => {
		if (!selectedSession || !activeAccountID || sending) return false;
		// sequence 是本轮位置卡片发送的最新请求代次。
		const sequence = ++sendSequence.current;
		sendController.current?.abort();
		// controller 只拥有本轮位置卡片请求，账号或会话切换会使旧响应失效。
		const controller = new AbortController();
		sendController.current = controller;
		setSending(true);
		setError('');
		try {
			// result 是平台接受并完成本地状态绑定的位置消息。
			const result = await sendChatLocationCard({
				account_id: activeAccountID, chat_id: selectedSession.chat_id, buyer_id: selectedSession.buyer_id,
				buyer_name: selectedSession.buyer_name, item_id: selectedSession.item_id, item_title: selectedSession.item_title,
				...locationDraft,
			}, { signal: controller.signal });
			if (!isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) return false;
			acceptSentMessage(result.message);
			return true;
		} catch (/* locationSendError 是平台拒绝、请求取消或本地状态错误。 */ locationSendError) {
			if (isCurrentChatRequest(sendSequence.current, sequence, controller.signal) && !isChatAbortError(locationSendError)) setError(locationSendError instanceof Error ? locationSendError.message : '位置卡片发送失败');
			return false;
		} finally {
			if (isCurrentChatRequest(sendSequence.current, sequence, controller.signal)) setSending(false);
		}
	}, [acceptSentMessage, activeAccountID, selectedSession, sending]);

	/** loadLocationCardDefaults 在打开弹窗前读取系统设置页的最新四个非敏感默认值。 */
	const loadLocationCardDefaults = useCallback(/* loadLatestLocationDefaults 取消旧读取并只接受当前请求结果。 */ async (): Promise<boolean> => {
		locationDefaultsController.current?.abort();
		// controller 只拥有本次默认值读取请求，完成或卸载后释放。
		const controller = new AbortController();
		locationDefaultsController.current = controller;
		setLocationDefaultsLoading(true);
		setError('');
		try {
			// defaults 是系统设置页当前保存的位置卡片表单初值。
			const defaults = await getChatLocationCardDefaults({ signal: controller.signal });
			if (controller.signal.aborted || locationDefaultsController.current !== controller) return false;
			setLocationCardDefaults(defaults);
			return true;
		} catch (/* defaultsError 是设置读取失败或主动取消。 */ defaultsError) {
			if (!controller.signal.aborted && locationDefaultsController.current === controller) setError(defaultsError instanceof Error ? defaultsError.message : '读取位置卡片默认设置失败');
			return false;
		} finally {
			if (locationDefaultsController.current === controller) {
				locationDefaultsController.current = null;
				setLocationDefaultsLoading(false);
			}
		}
	}, []);

  /** 重试最近一次失败的文本或图片发送，引用文本必须保留原目标。 */
  const retrySend = useCallback(/* 当前回调保留原引用目标重试失败发送。 */ async (): Promise<boolean> => {
		if (retryText) {
			if (retryText.accountID !== activeAccountID || retryText.chatID !== selectedSession?.chat_id) return false;
			return sendText(retryText.text, retryText.replyToMessageKey, false);
		}
    if (retryImage) await sendImage(retryImage, false);
    return false;
  }, [activeAccountID, retryImage, retryText, selectedSession?.chat_id, sendImage, sendText]);

	/** 撤回指定本地消息键，并以服务端最新状态替换当前会话消息。 */
	const recallMessage = useCallback(/* 当前回调执行用户确认的消息撤回。 */ async (messageKey: string): Promise<void> => {
		if (!activeAccountID || !messageKey) return;
		setError('');
		try {
			// response 是服务端明确成功或结果待确认时返回的最新消息。
			const response = await recallChatMessage(activeAccountID, messageKey);
			setMessages(/* currentMessages 保存撤回前当前会话消息。 */ currentMessages => currentMessages.map(/* currentMessage 是待匹配本地消息键的消息。 */ currentMessage => currentMessage.message_key === response.message.message_key ? response.message : currentMessage));
		} catch (recallError /* recallError 保存撤回请求的用户可见失败。 */) {
			setError(recallError instanceof Error ? recallError.message : '撤回消息失败');
			throw recallError;
		}
	}, [activeAccountID]);

  return {
	accounts, activeAccountID, activeSessions, selectedSession, activeAccount, creditProfile, creditLoading, creditError, messages, search, unreadOnly, draft, loading, messagesLoading, olderLoading, hasOlder, contactsLoading, hasMoreContacts: hasMoreContacts[activeAccountID] === true, emojiOpen, sending, error, liveState, pendingImage, locationCardDefaults, locationDefaultsLoading,
		activeChatID, navigationPriorityChatID, closeEligibleOrderID, closeEligibleOrderStage, filteredSessions, scrollRef, imageInputRef, setActiveAccountID, setActiveChatID, clearNavigationPriority, setSearch, setUnreadOnly, setDraft, setEmojiOpen,
		reloadSessions, loadMoreContacts, pinningSessionIDs, setSessionPinned, loadOlderMessages, handleMessageScroll, handleSend, handleImage, handlePastedImages, confirmSendImage, handleSendLocationCard, loadLocationCardDefaults, closeImagePreview, retrySend, recallMessage, retryAvailable: Boolean(retryText || retryImage), unreadForAccount,
    emojiURL, xianyuEmojis, renderXianyuText, formatClock, messageTime,
  };
};
