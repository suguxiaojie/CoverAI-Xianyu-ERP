import {
AccountDetail,
ChatCreditProfile,
ChatMessage,
ChatSession,
OperationResponse
} from '../../../shared/api-contract/chat';
import { get,post,postForm,put,type RequestControlOptions } from '../../../shared/http/client';
import { objectFrom } from '../../../shared/http/contract';
export type * from '../../../shared/api-contract/chat';
import type { ChatReadReceipt } from './types';
import type { OrderStatus } from '../../../shared/api-contract/orders';
import type { OrderSingleRefreshResponse } from '../../../shared/api-contract/orders';

/** 聊天账号选择器读取非敏感账号摘要。 */
export const getAccountDetails = async (options?: RequestControlOptions): Promise<AccountDetail[]> => get('/api/v1/accounts/details', undefined, options);

/** 聊天运行提示读取账号连接状态索引。 */
export const getAccountRuntimeStatuses = async (options?: RequestControlOptions): Promise<Record<string, { /** 当前连接状态。 */ state: NonNullable<AccountDetail['runtime_state']>; /** 状态说明。 */ message?: string; /** 是否已连接。 */ connected: boolean; /** 连续失败次数。 */ failures: number; /** 最近更新时间。 */ updated_at: string }>> => get('/api/v1/accounts/runtime-status', undefined, options);

/** ChatLocationCardDefaults 是系统设置页保存、聊天弹窗逐次带入的默认表单文本。 */
export interface ChatLocationCardDefaults {
	/** title 是默认位置名称。 */ title: string;
	/** description 是默认地址或到店说明。 */ description: string;
	/** latitude 是保留输入精度的默认纬度文本。 */ latitude: string;
	/** longitude 是保留输入精度的默认经度文本。 */ longitude: string;
}

/** getChatLocationCardDefaults 只从已脱敏系统设置响应提取四个非敏感位置默认值。 */
export const getChatLocationCardDefaults = async (options?: RequestControlOptions): Promise<ChatLocationCardDefaults> => {
	// response 是管理员系统设置的已脱敏传输对象。
	const response = await get<Record<string, unknown>>('/api/v1/settings/system', undefined, options);
	// settings 是兼容 data／settings 包裹层后的普通配置映射。
	const settings = objectFrom<Record<string, unknown>>(response, ['data', 'settings']) || response;
	return {
		title: String(settings['chat.location_card_title'] || ''),
		description: String(settings['chat.location_card_content'] || ''),
		latitude: String(settings['chat.location_card_latitude'] || ''),
		longitude: String(settings['chat.location_card_longitude'] || ''),
	};
};
export interface ChatSessionPage { /** sessions 表示聊天会话列表。 */ sessions: ChatSession[]; /** has_more 表示是否存在更多数据。 */ has_more: boolean; /** next_cursor 表示下一页游标。 */ next_cursor?: number }

/** ChatSessionPinResult 是服务端确认持久化后的精确会话置顶结果。 */
export interface ChatSessionPinResult {
	/** account_id 是已更新会话偏好的账号标识。 */
	account_id: string;
	/** chat_id 是已更新的精确会话标识。 */
	chat_id: string;
	/** pinned 是服务端已确认的目标置顶状态。 */
	pinned: boolean;
}

// getChatSessionPage 分页读取聊天会话。
export const getChatSessionPage = async (accountId: string, cursor?: number, options?: RequestControlOptions, refresh = false, search = ''): Promise<ChatSessionPage> => {
	// result 接口响应结果，用于当前 API 处理流程。
	const response = await get<unknown>('/api/v1/chat/sessions', { account_id: accountId, cursor, refresh: refresh ? 1 : undefined, search: search.trim() || undefined },
		refresh ? { timeoutMs: 60_000, ...options } : options);
	// result 是兼容直接分页对象和 data 包裹后的聊天会话分页。
	const result = objectFrom<Partial<ChatSessionPage>>(response, ['data', 'result']) || {};
	return { sessions: result.sessions || [], has_more: result.has_more === true, next_cursor: result.next_cursor };
};

// getChatSessions 读取聊天会话列表。
export const getChatSessions = async (accountId: string, options?: RequestControlOptions): Promise<ChatSession[]> =>
	(await getChatSessionPage(accountId, undefined, options)).sessions;

/** setChatSessionPinned 幂等保存当前 ERP 账号内精确会话的置顶偏好。 */
export const setChatSessionPinned = async (accountId: string, chatId: string, pinned: boolean, options?: RequestControlOptions): Promise<ChatSessionPinResult> =>
	put(`/api/v1/chat/sessions/${encodeURIComponent(chatId)}/pin`, { account_id: accountId, pinned }, options);

/** getChatUserCredit 只查询当前已打开会话买家的公开信用，列表加载不会调用此接口。 */
export const getChatUserCredit = async (accountId: string, buyerId: string, options?: RequestControlOptions): Promise<ChatCreditProfile> =>
	get('/api/v1/chat/user-credit', { account_id: accountId, buyer_id: buyerId }, options);

export interface ChatMessagePage {
	/** messages 表示聊天消息列表。 */ messages: ChatMessage[];
	/** has_more 表示是否存在更多数据。 */ has_more: boolean;
	/** next_cursor 表示下一页游标。 */ next_cursor?: number;
	/** session 表示会话。 */ session?: ChatSession;
}

/** ConversationOrderAssociation 区分精确本会话订单和同账号买家历史订单。 */
export type ConversationOrderAssociation = 'current_chat' | 'same_buyer';

/** ConversationOrderStatus 是 Chat feature 从 transport 状态适配出的订单状态。 */
export type ConversationOrderStatus = OrderStatus;

/** ConversationOrder 是历史订单侧栏使用的最小非敏感订单模型和关联依据。 */
export interface ConversationOrder {
	/** id 是前端列表使用的稳定订单标识。 */
	id: string;
	/** order_id 是平台订单标识。 */
	order_id: string;
	/** item_id 是关联商品标识。 */
	item_id: string;
	/** item_title 是当前或历史本地商品标题。 */
	item_title?: string;
	/** item_image 是本地商品详情解析出的主图。 */
	item_image?: string;
	/** quantity 是订单数量。 */
	quantity: number;
	/** amount 是实付金额文本。 */
	amount: string;
	/** status 是归一化后的订单状态。 */
	status: OrderStatus;
	/** order_status 是兼容服务端状态字段。 */
	order_status?: OrderStatus;
	/** created_at 是订单创建时间。 */
	created_at?: string;
	/** association 是服务端确认的精确关联类型。 */
	association: ConversationOrderAssociation;
}

/** ConversationOrderSummary 是不受状态筛选影响的历史订单摘要。 */
export interface ConversationOrderSummary {
	/** total 是全部上下文订单数。 */
	total: number;
	/** current_chat 是明确属于当前 chat_id 的订单数。 */
	current_chat: number;
	/** completed 是已完成订单数。 */
	completed: number;
}

/** ConversationOrderContextResponse 是历史订单侧栏的只读分页响应。 */
export interface ConversationOrderContextResponse {
	/** summary 是全部状态的历史订单摘要。 */
	summary: ConversationOrderSummary;
	/** orders 是当前状态和页码下的订单。 */
	orders: ConversationOrder[];
	/** total 是当前状态筛选后的订单数。 */
	total: number;
	/** page 是当前页码。 */
	page: number;
	/** page_size 是每页数量。 */
	page_size: number;
	/** total_pages 是筛选结果总页数。 */
	total_pages: number;
	/** truncated 表示服务端五百单保护上限已命中。 */
	truncated: boolean;
}

/** normalizeConversationOrderStatus 把后端兼容状态转换为共享订单状态。 */
const normalizeConversationOrderStatus = (value: unknown): OrderStatus => {
	// status 是服务端返回的文本或历史数字状态。
	const status = String(value || '');
	if (status === 'paid' || status === '2') return 'pending_ship';
	if (status === '1') return 'processing';
	if (status === '3') return 'shipped';
	if (status === '4' || status === '11') return 'completed';
	return ['processing', 'pending_ship', 'shipped', 'received', 'completed', 'cancelled', 'refunding', 'refunded'].includes(status) ? status as OrderStatus : 'unknown';
};

/** conversationOrderStatusRank 返回完成优先、退款和取消末尾的同页防御排序级别。 */
const conversationOrderStatusRank = (status: ConversationOrderStatus): number => {
	if (status === 'completed') return 0;
	if (['processing', 'pending_ship', 'shipped', 'received'].includes(status)) return 1;
	if (status === 'refunding' || status === 'unknown') return 2;
	return 3;
};

/** getConversationOrderContext 读取当前账号会话和买家精确关联的本地历史订单。 */
export const getConversationOrderContext = async (accountID: string, chatID: string, buyerID: string, status: string, page: number, options?: RequestControlOptions): Promise<ConversationOrderContextResponse> => {
	// response 是服务端具名历史订单分页响应。
	const response = await get<ConversationOrderContextResponse>('/api/v1/orders/conversation-context', {
		account_id: accountID, chat_id: chatID || undefined, buyer_id: buyerID || undefined,
		status: status === 'all' ? undefined : status, page, page_size: 20,
	}, options);
	// orders 是规范状态后按服务端同一业务优先级稳定排序的当前页订单。
	const orders = (response.orders || []).map(
		// order 是当前补充稳定 id、数量和状态的历史订单传输记录。
		order => ({ ...order, id: order.order_id, quantity: Number(order.quantity || 1), status: normalizeConversationOrderStatus(order.status || order.order_status) }),
	).sort(
		// left、right 是当前比较状态和会话关系的两张订单卡。
		(left, right) => conversationOrderStatusRank(left.status) - conversationOrderStatusRank(right.status)
			|| (left.association === right.association ? 0 : left.association === 'current_chat' ? -1 : 1),
	);
	return {
		...response,
		orders,
	};
};

/** syncConversationOrder 只读取指定订单的平台详情并写回本地投影，不扫描整个店铺。 */
export const syncConversationOrder = async (orderID: string, options?: RequestControlOptions): Promise<OrderSingleRefreshResponse> =>
	post(`/api/v1/orders/${encodeURIComponent(orderID)}/refresh`, undefined, options);

/** AdjustPriceField 是闲鱼 render 返回的动态金额字段。 */
export interface AdjustPriceField {
	/** key 是 submit data 使用的平台字段名。 */ key: string;
	/** name 是字段展示名称。 */ name: string;
	/** prefix_text 是金额前缀。 */ prefix_text: string;
	/** value 是单位为元的当前或目标金额。 */ value: string;
	/** read_only 表示字段是否只能展示。 */ read_only: boolean;
}

/** AdjustPriceForm 是待付款订单的只读动态表单。 */
export interface AdjustPriceForm {
	/** order_id 是平台待付款订单标识。 */ order_id: string;
	/** account_id 是卡片所属卖家账号。 */ account_id: string;
	/** title 是平台表单标题。 */ title: string;
	/** fields 是平台当前要求展示和提交的金额字段。 */ fields: AdjustPriceField[];
}

/** AdjustPriceResult 是平台真实 submit 的确定性结果。 */
export interface AdjustPriceResult {
	/** success 只在平台明确确认改价时为真。 */ success: boolean;
	/** status 是 succeeded、failed、needs_review 或 succeeded_with_warning。 */ status: string;
	/** message 是面向用户的非敏感结果说明。 */ message: string;
	/** order_id 是已提交的平台订单标识。 */ order_id: string;
	/** fields 是本次实际提交的规范元金额。 */ fields: AdjustPriceField[];
}

/** RefundDetail 是退款申请卡片读取的官方只读详情。 */
export interface RefundDetail {
	/** order_id 是平台订单标识。 */ order_id: string;
	/** account_id 是订单所属卖家账号。 */ account_id: string;
	/** refund_id 是平台退款申请标识。 */ refund_id?: string;
	/** status 是平台退款状态编码或展示文本。 */ status?: string;
	/** status_text 是平台状态组件提供的可读文本。 */ status_text?: string;
	/** type 是仅退款、退货退款等展示类型。 */ type?: string;
	/** reason 是买家选择的退款原因。 */ reason?: string;
	/** amount 是平台展示的退款金额文本。 */ amount?: string;
	/** apply_time 是平台展示的申请时间。 */ apply_time?: string;
	/** buyer_description 是买家补充说明。 */ buyer_description?: string;
	/** buyer_images 是买家提交的图片凭证。 */ buyer_images: string[];
	/** buyer_videos 是买家提交的视频凭证。 */ buyer_videos: string[];
	/** seller 表示当前详情明确属于卖家处理视角。 */ seller: boolean;
	/** actions 是平台当前允许在 ERP 中执行的普通退款动作。 */ actions: RefundAction[];
	/** official_url 是闲鱼官方详情页的只读兜底地址。 */ official_url: string;
}

/** RefundAction 是不包含 MTOP 参数的动态退款动作。 */
export interface RefundAction {
	/** code 是提交时重新匹配平台最新详情的动作标识。 */ code: string;
	/** name 是平台按钮名称。 */ name: string;
	/** kind 是同意或拒绝方向。 */ kind: 'agree' | 'reject';
	/** mode 是直连、PC 支付验证、Merchant 拒绝或官方兜底。 */ mode: 'direct' | 'merchant_verify' | 'merchant_refuse' | 'official';
	/** confirm_title 是平台双重确认标题。 */ confirm_title?: string;
	/** confirm_description 是平台双重确认说明。 */ confirm_description?: string;
}

/** RefundActionResult 是平台真实退款动作的确定性结果。 */
export interface RefundActionResult {
	/** success 只在平台明确受理动作时为真。 */ success: boolean;
	/** status 是 succeeded、failed、needs_review 或 succeeded_with_warning。 */ status: string;
	/** message 是不含凭证的用户提示。 */ message: string;
	/** order_id 是本次处理的平台订单标识。 */ order_id: string;
	/** refund_id 是本次处理的平台退款申请标识。 */ refund_id: string;
	/** action 是实际提交的 agree 或 reject。 */ action: 'agree' | 'reject';
}

/** MerchantRefundVerification 是服务端短期保存 authToken 的支付验证会话。 */
export interface MerchantRefundVerification {
	/** session_id 是不包含授权 token 的随机会话标识。 */ session_id: string;
	/** verify_url 是支付宝跨域 iframe 地址。 */ verify_url: string;
	/** verify_origin 是 postMessage 必须精确匹配的来源。 */ verify_origin: string;
	/** expires_at 是会话过期 Unix 秒。 */ expires_at: number;
}

/** MerchantRefundRefuseReason 是平台动态拒绝原因。 */
export interface MerchantRefundRefuseReason {
	/** id 是平台 refuseReasonId。 */ id: string;
	/** name 是平台展示原因。 */ name: string;
	/** requires_app 表示该原因只能在手机 App 处理。 */ requires_app: boolean;
}

/** MerchantRefundRefuseForm 是拒绝退款动态表单。 */
export interface MerchantRefundRefuseForm {
	/** refund_id 是当前退款申请标识。 */ refund_id: string;
	/** reasons 是平台动态原因。 */ reasons: MerchantRefundRefuseReason[];
	/** selected_reason_id 是当前 render 原因。 */ selected_reason_id?: string;
	/** proof_required 表示必须上传凭证。 */ proof_required: boolean;
	/** proof_placeholder 是补充说明提示。 */ proof_placeholder?: string;
	/** negotiation_enabled 表示允许协商金额。 */ negotiation_enabled: boolean;
	/** negotiation_type 是平台动态协商类型。 */ negotiation_type?: string;
	/** min_cents 是协商金额下限，单位为分。 */ min_cents: number;
	/** max_cents 是协商金额上限，单位为分。 */ max_cents: number;
}

/** CloseOrderReasons 是平台当前允许卖家选择的动态关闭原因。 */
export interface CloseOrderReasons {
	/** order_id 是当前仍可取消的平台订单号。 */ order_id: string;
	/** account_id 是卡片所属卖家账号。 */ account_id: string;
	/** reasons 是平台动态原因列表。 */ reasons: string[];
}

/** CloseOrderEligibility 是不访问平台的本地卖家关单资格结果。 */
export interface CloseOrderEligibility {
	/** eligible 只在订单表仍为待付款或已付款待发货且聊天角色和幂等门禁均通过时为真。 */
	eligible: boolean;
	/** order_id 是本次检查的平台订单号。 */
	order_id: string;
	/** account_id 是本次检查的卖家账号。 */
	account_id: string;
	/** reason 是不可取消时的本地安全说明。 */
	reason?: string;
	/** stage 是后端权威确认的待付款或已付款待发货阶段。 */
	stage?: 'pending_payment' | 'pending_ship';
}

/** CloseOrderResult 是卖家关单的确定性结果。 */
export interface CloseOrderResult {
	/** success 只在平台明确确认关单时为真。 */ success: boolean;
	/** status 是 succeeded、failed、needs_review 或 succeeded_with_warning。 */ status: string;
	/** message 是用户可见且不含凭证的结果说明。 */ message: string;
	/** order_id 是已提交的平台订单号。 */ order_id: string;
}

/** ShipmentEvidenceResult 是闲鱼明确确认无需寄件后的稳定结果。 */
export interface ShipmentEvidenceResult {
	/** success 只在平台响应明确包含成功订单号时为真。 */
	success: boolean;
	/** status 是 succeeded、succeeded_with_warning、failed 或 needs_review。 */
	status: string;
	/** message 是不含凭证和收货信息的结果说明。 */
	message: string;
	/** order_id 是本次发货对应的平台订单号。 */
	order_id: string;
}

/** getOrderAdjustPriceForm 读取待付款订单当前动态改价表单，不修改价格。 */
export const getOrderAdjustPriceForm = async (accountID: string, orderID: string, options?: RequestControlOptions): Promise<AdjustPriceForm> =>
	get(`/api/v1/orders/${encodeURIComponent(orderID)}/adjust-price`, { account_id: accountID }, options);

/** getOrderRefundDetail 读取退款原因、金额和买家说明，不执行同意或拒绝退款。 */
export const getOrderRefundDetail = async (accountID: string, orderID: string, options?: RequestControlOptions): Promise<RefundDetail> =>
	get(`/api/v1/orders/${encodeURIComponent(orderID)}/refund-detail`, { account_id: accountID }, options);

/** submitOrderRefundAction 提交用户应用内二次确认后的普通同意／拒绝退款动作。 */
export const submitOrderRefundAction = async (accountID: string, orderID: string, actionCode: string, options?: RequestControlOptions): Promise<RefundActionResult> =>
	post(`/api/v1/orders/${encodeURIComponent(orderID)}/refund-detail`, { account_id: accountID, action_code: actionCode }, options);

/** startMerchantRefundVerification 创建 PC 支付验证 iframe，不执行退款。 */
export const startMerchantRefundVerification = async (accountID: string, orderID: string, options?: RequestControlOptions): Promise<MerchantRefundVerification> =>
	post(`/api/v1/orders/${encodeURIComponent(orderID)}/refund-verification`, { account_id: accountID }, options);

/** completeMerchantRefundVerification 在支付宝 iframe 明确成功后执行最终 Merchant 退款。 */
export const completeMerchantRefundVerification = async (accountID: string, orderID: string, sessionID: string, options?: RequestControlOptions): Promise<RefundActionResult> =>
	post(`/api/v1/orders/${encodeURIComponent(orderID)}/refund-verification/${encodeURIComponent(sessionID)}/complete`, { account_id: accountID }, options);

/** getMerchantRefundRefuseForm 读取当前动态拒绝原因和要求。 */
export const getMerchantRefundRefuseForm = async (accountID: string, orderID: string, reasonID = '', options?: RequestControlOptions): Promise<MerchantRefundRefuseForm> =>
	get(`/api/v1/orders/${encodeURIComponent(orderID)}/refund-refuse`, { account_id: accountID, reason_id: reasonID || undefined }, options);

/** refuseMerchantRefund 在最终确认后上传内存图片并提交 Merchant 拒绝退款。 */
export const refuseMerchantRefund = async (accountID: string, orderID: string, reasonID: string, description: string, negotiationCents: number, images: File[], options?: RequestControlOptions): Promise<RefundActionResult> => {
	/** form 是拒绝原因、说明、协商金额和图片组成的 multipart 表单。 */
	const form = new FormData();
	form.append('account_id', accountID);
	form.append('reason_id', reasonID);
	form.append('description', description);
	form.append('negotiation_cents', String(negotiationCents));
	images.forEach(/* image 是按用户选择顺序提交的一张退款凭证。 */ image => form.append('images', image, image.name));
	return postForm(`/api/v1/orders/${encodeURIComponent(orderID)}/refund-refuse`, form, options);
};

/** adjustOrderPrice 提交用户二次确认后的真实平台改价。 */
export const adjustOrderPrice = async (accountID: string, orderID: string, fields: AdjustPriceField[], options?: RequestControlOptions): Promise<AdjustPriceResult> =>
	post(`/api/v1/orders/${encodeURIComponent(orderID)}/adjust-price`, { account_id: accountID, fields: fields.map(/* priceFieldInput 只提交平台 key 和元金额。 */ field => ({ key: field.key, value: field.value })) }, options);

/** getCloseOrderReasons 只读取卖家当前可取消订单的动态原因，不关闭订单。 */
export const getCloseOrderReasons = async (accountID: string, orderID: string, options?: RequestControlOptions): Promise<CloseOrderReasons> =>
	get(`/api/v1/orders/${encodeURIComponent(orderID)}/close`, { account_id: accountID }, options);

/** getCloseOrderEligibility 只读取本地订单、聊天角色和幂等共同确认的关单资格。 */
export const getCloseOrderEligibility = async (accountID: string, orderID: string, options?: RequestControlOptions): Promise<CloseOrderEligibility> =>
	get(`/api/v1/orders/${encodeURIComponent(orderID)}/close-eligibility`, { account_id: accountID }, options);

/** closeOrderBySeller 提交用户二次确认后的真实卖家关单。 */
export const closeOrderBySeller = async (accountID: string, orderID: string, reason: string, options?: RequestControlOptions): Promise<CloseOrderResult> =>
	post(`/api/v1/orders/${encodeURIComponent(orderID)}/close`, { account_id: accountID, reason }, options);

/** shipOrderWithEvidence 在用户二次确认后上传内存图片并执行一次官方无需寄件。 */
export const shipOrderWithEvidence = async (accountID: string, orderID: string, tradeText: string, images: File[], options?: RequestControlOptions): Promise<ShipmentEvidenceResult> => {
	if (images.length === 0) {
		return post(`/api/v1/orders/${encodeURIComponent(orderID)}/ship-with-evidence`, { account_id: accountID, trade_text: tradeText }, options);
	}
	/** form 是当前最终提交使用的 multipart 表单；图片在此之前不会离开浏览器。 */
	const form = new FormData();
	form.append('account_id', accountID);
	form.append('trade_text', tradeText);
	images.forEach(/* image 是按用户排序追加的一张凭证。 */ image => form.append('images', image, image.name));
	return postForm(`/api/v1/orders/${encodeURIComponent(orderID)}/ship-with-evidence`, form, options);
};

// getChatMessagePage 分页读取聊天消息。
export const getChatMessagePage = async (accountId: string, chatId: string, cursor?: number, beforeId?: number, options?: RequestControlOptions): Promise<ChatMessagePage> => {
	// result 接口响应结果，用于当前 API 处理流程。
	const response = await get<unknown>('/api/v1/chat/messages', {
		account_id: accountId, chat_id: chatId, cursor, before_id: beforeId,
	}, options);
	// result 是兼容直接分页对象和 data 包裹后的聊天消息分页。
	const result = objectFrom<Partial<ChatMessagePage>>(response, ['data', 'result']) || {};
	return { messages: result.messages || [], has_more: result.has_more === true, next_cursor: result.next_cursor, session: result.session };
};

// getChatMessages 读取聊天消息列表。
export const getChatMessages = async (accountId: string, chatId: string, beforeId?: number, options?: RequestControlOptions): Promise<ChatMessage[]> =>
	(await getChatMessagePage(accountId, chatId, undefined, beforeId, options)).messages;

// sendChatMessage 发送聊天文本消息。
export const sendChatMessage = async (input: {
  /** account_id 表示账号标识。 */ account_id: string; /** chat_id 表示聊天标识。 */ chat_id: string; /** buyer_id 表示买家标识。 */ buyer_id: string; /** buyer_name 表示买家名称。 */ buyer_name?: string;
  /** item_id 表示商品标识。 */ item_id?: string; /** item_title 表示商品标题。 */ item_title?: string; /** text 表示文本。 */ text: string;
	/** reply_to_message_key 是当前会话中经服务端再次校验的本地引用目标键。 */ reply_to_message_key?: string;
}, options?: RequestControlOptions): Promise<{/** message 表示消息数据。 */ message: ChatMessage}> =>
	post(input.reply_to_message_key ? '/api/v1/chat/replies' : '/api/v1/chat/messages', input, options);

// sendChatImage 发送聊天图片消息。
export const sendChatImage = async (input: {
  /** account_id 表示账号标识。 */ account_id: string; /** chat_id 表示聊天标识。 */ chat_id: string; /** buyer_id 表示买家标识。 */ buyer_id: string; /** buyer_name 表示买家名称。 */ buyer_name?: string;
  /** buyer_avatar_url 表示买家头像地址。 */ buyer_avatar_url?: string; /** item_id 表示商品标识。 */ item_id?: string; /** item_title 表示商品标题。 */ item_title?: string; /** image 表示图片数据。 */ image: File;
}, options?: RequestControlOptions): Promise<{/** message 表示消息数据。 */ message: ChatMessage}> => {
	// form 消息表单，用于当前 API 处理流程。
	const form = new FormData();
	Object.entries(input).forEach(/* 当前回调用于处理集合元素或接口响应。 */ ([key, value]) => form.append(key, value));
	return postForm('/api/v1/chat/images', form, { timeoutMs: 120_000, ...options });
};

/** SendLocationCardInput 是用户发送前确认的自定义位置卡片字段。 */
export interface SendLocationCardInput {
	/** account_id 是实际发送的店铺账号。 */ account_id: string;
	/** chat_id 是当前精确单聊会话。 */ chat_id: string;
	/** buyer_id 是当前会话接收者。 */ buyer_id: string;
	/** buyer_name 只用于本地会话摘要。 */ buyer_name?: string;
	/** item_id 是当前会话关联商品。 */ item_id?: string;
	/** item_title 是当前会话关联商品名称。 */ item_title?: string;
	/** title 是位置卡片主标题。 */ title: string;
	/** description 是位置卡片说明。 */ description: string;
	/** latitude 是 WGS84 纬度。 */ latitude: number;
	/** longitude 是 WGS84 经度。 */ longitude: number;
}

/** sendChatLocationCard 通过版本化聊天接口发送一张位置卡片。 */
export const sendChatLocationCard = async (input: SendLocationCardInput, options?: RequestControlOptions): Promise<{/** message 是平台接受后的本地位置消息。 */ message: ChatMessage}> =>
	post('/api/v1/chat/location-cards', input, options);

/** 撤回当前账号两分钟内发出的文本或图片消息。 */
export const recallChatMessage = async (accountId: string, messageKey: string, options?: RequestControlOptions): Promise<{/** message 是撤回后或待确认的最新消息。 */ message: ChatMessage}> =>
	post(`/api/v1/chat/messages/${encodeURIComponent(messageKey)}/recall`, { account_id: accountId }, options);

/** 向平台确认指定会话中的入站消息已读。 */
export const markChatRead = async (accountId: string, chatId: string, messageIDs: ChatReadReceipt[], options?: RequestControlOptions): Promise<OperationResponse> =>
	post('/api/v1/chat/read', { account_id: accountId, chat_id: chatId, message_ids: messageIDs }, options);
