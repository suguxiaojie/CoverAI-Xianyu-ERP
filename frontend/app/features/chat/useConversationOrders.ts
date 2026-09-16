import React from 'react';
import { getConversationOrderContext,syncConversationOrder,type ConversationOrder,type ConversationOrderContextResponse,type ConversationOrderSummary } from './api';

/** ConversationOrderFilter 是历史订单侧栏公开的状态筛选集合。 */
export type ConversationOrderFilter = 'all' | 'pending_payment' | 'pending_ship' | 'shipped' | 'received' | 'completed' | 'refund' | 'cancelled';

/** UseConversationOrdersOptions 是真实历史订单请求所需的会话上下文。 */
export interface UseConversationOrdersOptions {
	/** accountID 是当前聊天账号。 */
	accountID: string;
	/** chatID 是当前聊天会话。 */
	chatID: string;
	/** buyerID 是当前稳定买家标识。 */
	buyerID: string;
	/** revision 是当前会话最新订单系统卡片键，用于实时刷新本地投影。 */
	revision: string;
	/** orderID 是最新待付款或已付款卡片的订单号，仅用于单笔详情补全。 */
	orderID?: string;
}

/** UseConversationOrdersResult 是历史订单侧栏的服务端数据、筛选和分页状态。 */
export interface UseConversationOrdersResult {
	/** orders 是当前页真实历史订单。 */
	orders: ConversationOrder[];
	/** summary 是全部状态订单摘要。 */
	summary: ConversationOrderSummary;
	/** total 是当前状态筛选后的订单数。 */
	total: number;
	/** page 是当前页码。 */
	page: number;
	/** totalPages 是总页数。 */
	totalPages: number;
	/** status 是当前状态筛选。 */
	status: ConversationOrderFilter;
	/** loading 表示首屏或筛选请求正在进行。 */
	loading: boolean;
	/** error 是不含凭证的查询错误文案。 */
	error: string;
	/** truncated 表示服务端保护上限已命中。 */
	truncated: boolean;
	/** setStatus 切换筛选并回到第一页。 */
	setStatus: (status: ConversationOrderFilter) => void;
	/** setPage 切换历史订单页码。 */
	setPage: (page: number) => void;
	/** reload 重新读取当前上下文。 */
	reload: () => void;
}

/** emptyConversationOrderResponse 创建会话切换时不携带旧买家数据的空响应。 */
const emptyConversationOrderResponse = (): ConversationOrderContextResponse => ({ summary: { total: 0, current_chat: 0, completed: 0 }, orders: [], total: 0, page: 1, page_size: 20, total_pages: 0, truncated: false });

/** useConversationOrders 加载真实历史订单，并用取消与请求代次阻止旧会话响应覆盖新会话。 */
export const useConversationOrders = (options: UseConversationOrdersOptions): UseConversationOrdersResult => {
	// targetOrderID 是最新交易卡片明确提供的订单号。
	const targetOrderID = String(options.orderID || '').trim();
	// currentEnrichmentKey 将账号、会话、卡片和订单绑定，用于防止自动补全循环并允许手动重试。
	const currentEnrichmentKey = `${options.accountID}\u0000${options.chatID}\u0000${options.revision}\u0000${targetOrderID}`;
	// status、setStatusState 保存当前服务端状态筛选。
	const [status, setStatusState] = React.useState<ConversationOrderFilter>('all');
	// page、setPage 保存当前历史订单页码。
	const [page, setPage] = React.useState(1);
	// data、setData 保存当前会话的服务端订单响应。
	const [data, setData] = React.useState<ConversationOrderContextResponse>(emptyConversationOrderResponse);
	// loading、setLoading 表示历史订单请求正在进行。
	const [loading, setLoading] = React.useState(true);
	// error、setError 保存当前查询失败文案。
	const [error, setError] = React.useState('');
	// reloadVersion、setReloadVersion 驱动用户手动刷新当前上下文。
	const [reloadVersion, setReloadVersion] = React.useState(0);
	// requestSequence 隔离取消信号未被底层遵守时的晚到响应。
	const requestSequence = React.useRef(0);
	// enrichmentAttempts 记录已尝试补全的精确卡片，避免本地投影刷新形成请求循环。
	const enrichmentAttempts = React.useRef(new Set<string>());

	React.useEffect(/* resetConversationOrders 在组件 key 未变化的防御场景下清除旧上下文状态。 */ () => {
		setStatusState('all');
		setPage(1);
		setData(emptyConversationOrderResponse());
		setError('');
	}, [options.accountID, options.buyerID, options.chatID]);

	React.useEffect(/* loadConversationOrders 同步当前精确会话上下文并在清理时取消请求。 */ () => {
		if (!options.accountID || (!options.chatID && !options.buyerID)) {
			setData(emptyConversationOrderResponse());
			setLoading(false);
			return /* noContextCleanup 当前没有可取消请求。 */ () => undefined;
		}
		// sequence 是本轮历史订单请求代次。
		const sequence = ++requestSequence.current;
		// controller 在会话、筛选、页码切换或卸载时取消旧请求。
		const controller = new AbortController();
		setLoading(true);
		setError('');
		void getConversationOrderContext(options.accountID, options.chatID, options.buyerID, status, page, { signal: controller.signal }).then(
			// response 是当前精确上下文的最新历史订单响应。
			response => {
				if (controller.signal.aborted || requestSequence.current !== sequence) return;
				setData(response);
				setLoading(false);
				// projectedOrder 是本地上下文中与卡片订单号一致的当前投影。
				const projectedOrder = response.orders.find(/* matchingProjectedOrder 只按平台订单号匹配补全对象。 */ order => order.order_id === targetOrderID);
				// amountValue 是本地投影的元单位实付金额；非正数代表卡片临时订单尚未补全。
				const amountValue = Number.parseFloat(String(projectedOrder?.amount || '').replace(/[^0-9.-]/g, ''));
				if (!targetOrderID || (Number.isFinite(amountValue) && amountValue > 0)) return;
				if (enrichmentAttempts.current.has(currentEnrichmentKey)) return;
				enrichmentAttempts.current.add(currentEnrichmentKey);
				// enrichmentTimer 给同一 WS 卡片的订单事实落库留出短暂时间，不启动整店同步。
				const enrichmentTimer = window.setTimeout(/* enrichConversationOrder 定向补全一笔订单并重读右栏。 */ () => {
					void syncConversationOrder(targetOrderID, { signal: controller.signal, timeoutMs: 60_000 }).then(
						/* enrichmentSucceeded 平台详情写回后触发新一代上下文请求。 */ () => {
							if (!controller.signal.aborted && requestSequence.current === sequence) setReloadVersion(/* currentVersion 是补全成功前的请求版本。 */ currentVersion => currentVersion + 1);
						},
						/* enrichmentFailed 保留临时订单供手动刷新，不用平台读失败覆盖已加载的历史订单。 */ () => undefined,
					);
				}, 500);
				controller.signal.addEventListener('abort', /* cancelEnrichmentTimer 会话切换时取消尚未开始的单笔补全。 */ () => window.clearTimeout(enrichmentTimer), { once: true });
			},
			// requestError 是历史订单请求失败原因；取消和旧代次不写入页面。
			requestError => {
				if (controller.signal.aborted || requestSequence.current !== sequence) return;
				setError(requestError instanceof Error ? requestError.message : '查询历史订单失败');
				setLoading(false);
			},
		);
		return /* cancelConversationOrderRequest 取消当前会话的历史订单请求。 */ () => controller.abort();
	}, [currentEnrichmentKey, options.accountID, options.buyerID, options.chatID, options.orderID, options.revision, page, reloadVersion, status, targetOrderID]);

	// setStatus 切换状态筛选并确保从第一页重新读取。
	const setStatus = React.useCallback(/* changeConversationOrderStatus 响应历史订单状态选择。 */ (nextStatus: ConversationOrderFilter): void => {
		setStatusState(nextStatus);
		setPage(1);
	}, []);
	// reload 手动递增请求版本并释放当前卡片的补全尝试标记，允许平台暂时未就绪后原地重试。
	const reload = React.useCallback(/* reloadConversationOrders 重新读取当前会话上下文。 */ () => {
		enrichmentAttempts.current.delete(currentEnrichmentKey);
		setReloadVersion(/* currentVersion 是刷新前版本。 */ currentVersion => currentVersion + 1);
	}, [currentEnrichmentKey]);
	return { orders: data.orders, summary: data.summary, total: data.total, page: data.page, totalPages: data.total_pages, status, loading, error, truncated: data.truncated, setStatus, setPage, reload };
};
