import {
AccountDetail,
AdminStatsResponse,
Item,
OperationResponse,
Order,
OrderBatchResponse,
OrderDTOResponse,OrderDetailResponse,
OrderRefreshJobCancelResponse,
OrderRefreshJobStartResponse,OrderRefreshJobStatusResponse,
OrderRefreshResponse,
OrderSingleRefreshResponse,
RedFlowerRequestResponse,
PaginatedResponse
} from '../../../shared/api-contract/orders';
import { del,get,post,postForm,put,type RequestControlOptions } from '../../../shared/http/client';
import { collectionFrom,objectFrom } from '../../../shared/http/contract';
export type * from '../../../shared/api-contract/orders';

/** ShipmentProof 是 ERP 明确发货成功后保存的只读凭证。 */
export interface ShipmentProof {
	/** success 表示读取完成。 */ success: boolean;
	/** order_id 是平台订单号。 */ order_id: string;
	/** account_id 是执行 ERP 发货的账号。 */ account_id: string;
	/** trade_text 是提交给闲鱼的发货描述。 */ trade_text: string;
	/** image_urls 是官方 HTTPS 图片地址。 */ image_urls: string[];
	/** source 当前固定为 erp。 */ source: string;
	/** submitted_at 是平台明确成功时的 Unix 秒。 */ submitted_at: number;
}

/** OrderSettlementSummary 是当前订单筛选范围内已发货订单扣除平台服务费后的待结算统计。 */
export interface OrderSettlementSummary {
	/** order_count 是参与计算的已发货订单数量。 */ order_count: number;
	/** gross_amount 是扣费前已发货订单总额，单位为人民币元。 */ gross_amount: string;
	/** service_fee 是总额累加后一次性计算的 1.6% 平台服务费。 */ service_fee: string;
	/** pending_amount 是扣除服务费后的待结算金额。 */ pending_amount: string;
	/** service_fee_rate 是服务端返回的固定费率文案。 */ service_fee_rate: string;
}

/** OrderListResponse 在通用分页契约之外携带待结算统计；旧服务缺失时由 Hook 回退为零。 */
export interface OrderListResponse extends PaginatedResponse<Order> {
	/** settlement_summary 是当前筛选范围的待结算统计。 */ settlement_summary?: OrderSettlementSummary;
}

/** 订单刷新前端最多轮询约 31 分钟，覆盖后端 30 分钟 worker 上限和终态写入余量。 */
const orderRefreshPollLimit = 3_720;
/** 订单刷新取消和终态复查使用独立五秒网络预算，不能复用已超时或已 Abort 的主信号。 */
const orderRefreshCancelTimeoutMs = 5_000;

/** 订单刷新轮询选项仅控制前端等待行为；不改变后端任务、HTTP 路径或请求体契约。 */
export interface OrderRefreshPollOptions extends RequestControlOptions {
  /** pollLimit 是本次前端最多读取任务状态的次数；省略时保持约三十一分钟默认预算。 */
  pollLimit?: number;
  /** pollIntervalMs 是两次状态读取之间的等待毫秒数；默认五百毫秒。 */
  pollIntervalMs?: number;
  /** onProgress 在每次状态轮询后接收轻量任务快照，页面据此实时更新进度而不等待完整结果。 */
  onProgress?: (job: OrderRefreshJobStatusResponse) => void;
}

/** OrderSyncMode 区分低请求量的默认增量同步和完整远端校准。 */
export type OrderSyncMode = 'incremental' | 'full';

/** OrderListRequestOptions 在通用请求控制之外携带订单创建时间和实付金额范围。 */
export interface OrderListRequestOptions extends RequestControlOptions {
  /** createdFrom 是包含下界的 RFC3339 UTC 时间。 */
  createdFrom?: string;
  /** createdTo 是排除上界的 RFC3339 UTC 时间。 */
  createdTo?: string;
  /** minAmount 是实付金额包含下界，单位为人民币元且最多两位小数。 */
  minAmount?: string;
  /** maxAmount 是实付金额包含上界，单位为人民币元且最多两位小数。 */
  maxAmount?: string;
}

/** 订单筛选器读取非敏感账号摘要。 */
export const getAccountDetails = async (options?: RequestControlOptions): Promise<AccountDetail[]> => get('/api/v1/accounts/details', undefined, options);

/** 订单关联商品展示读取当前商品索引。 */
export const getItems = async (accountID?: string, options?: RequestControlOptions): Promise<Item[]> => get('/api/v1/items', accountID ? { cookie_id: accountID } : undefined, options);

/** 管理员统计仍由订单域兼容 API 提供给历史管理页面。 */
export const getAdminStats = async (): Promise<AdminStatsResponse> => get('/api/v1/admin/stats');
// Orders
// normalizeOrderStatus 归一化订单状态。
const normalizeOrderStatus = (value: unknown): Order['status'] => {
  // status 状态值，用于当前 API 处理流程。
  const status = String(value || '');
  if (status === 'paid') return 'pending_ship';
  return ['processing', 'pending_ship', 'shipped', 'received', 'completed', 'cancelled', 'refunding', 'refunded'].includes(status)
    ? status as Order['status']
    : 'unknown';
};

// getOrders 读取订单列表。
export const getOrders = async (
  cookieId?: string,
  status?: string,
  page: number = 1,
  pageSize: number = 20,
  search?: string,
  options?: OrderListRequestOptions,
): Promise<OrderListResponse> => {
  // params 请求参数，用于当前 API 处理流程。
  const params: any = { page, page_size: pageSize };
  if (cookieId) params.cookie_id = cookieId;
  if (status && status !== 'all') params.status = status;
  if (search?.trim()) params.search = search.trim();
  if (options?.createdFrom) params.created_from = options.createdFrom;
  if (options?.createdTo) params.created_to = options.createdTo;
  if (options?.minAmount) params.min_amount = options.minAmount;
  if (options?.maxAmount) params.max_amount = options.maxAmount;

  // res 接口响应结果，用于当前 API 处理流程。
  const response = await get<unknown>('/api/v1/orders', params, options);
  // res 是兼容直接分页对象、orders 别名和 data 包裹后的订单响应。
	const res = objectFrom<Partial<OrderListResponse> & { /** orders 是历史订单列表字段别名。 */ orders?: Order[] }>(response, ['data', 'result']) || {};

  // Handle backend response variations
  // rawOrders 原始订单列表，用于当前 API 处理流程。
  const rawOrders = Array.isArray(res.orders) ? res.orders : collectionFrom<Order>(res.data, ['data', 'orders', 'items']);
  // orders 订单列表，用于当前 API 处理流程。
	const orders = rawOrders.map(/* 当前回调用于处理集合元素或接口响应。 */ (item: any) => ({
    ...item,
    id: item.id || item.order_id,
    status: normalizeOrderStatus(item.status || item.order_status),
    quantity: Number(item.quantity || 1),
	}));
	// rawSettlement 是服务端返回的待结算对象；兼容尚未提供该字段的旧服务。
	const rawSettlement = objectFrom<Partial<OrderSettlementSummary>>(res.settlement_summary) || {};
	// settlementSummary 把统计字段规范为页面可直接展示的稳定类型。
	const settlementSummary: OrderSettlementSummary = {
		order_count: Number(rawSettlement.order_count || 0),
		gross_amount: String(rawSettlement.gross_amount || '0.00'),
		service_fee: String(rawSettlement.service_fee || '0.00'),
		pending_amount: String(rawSettlement.pending_amount || '0.00'),
		service_fee_rate: String(rawSettlement.service_fee_rate || '1.6%'),
	};
	return {
    success: true,
    data: orders,
    total: res.total || orders.length,
    page: res.page || page,
    page_size: res.page_size || pageSize,
		total_pages: res.total_pages || 1,
		settlement_summary: settlementSummary,
	};
};

// getOrderDetail 读取订单详情。
export const getOrderDetail = async (orderId: string): Promise<{ /** success 表示是否成功。 */ success: boolean; /** data 表示数据。 */ data?: OrderDTOResponse }> => {
  // result 接口响应结果，用于当前 API 处理流程。
  const result = await get<OrderDetailResponse>(`/api/v1/orders/${orderId}`);
  return {
    success: true,
    data: result.data
  };
};

/** getShipmentProof 只读取本地保存的 ERP 发货凭证，不调用闲鱼平台。 */
export const getShipmentProof = async (orderId: string, options?: RequestControlOptions): Promise<ShipmentProof> => get(`/api/v1/orders/${encodeURIComponent(orderId)}/shipment-proof`, undefined, options);

// updateOrder 更新订单。
export const updateOrder = async (orderId: string, data: Partial<Order>): Promise<OperationResponse> => {
  return put(`/api/v1/orders/${orderId}`, data);
};

// deleteOrder 删除订单。
export const deleteOrder = async (orderId: string): Promise<OperationResponse> => {
  return del(`/api/v1/orders/${orderId}`);
};

// syncOrders 同步订单。
export const syncOrders = async (cookieId?: string, status?: string, options?: OrderRefreshPollOptions, mode: OrderSyncMode = 'incremental'): Promise<OrderRefreshResponse> => {
  // formData 表单数据，用于当前 API 处理流程。
  const formData = new FormData();
  if (cookieId) formData.append('cookie_id', cookieId);
  if (status) formData.append('status', status);
	formData.append('mode', mode);

	// start 表示后台订单刷新任务创建响应。
	const start = await postForm<OrderRefreshJobStartResponse>('/api/v1/orders/refresh', formData, options);
	options?.onProgress?.({ success: true, job_id: start.job_id, status: start.status });
	// cancelOnAbort 在调用方取消轮询时通知服务端停止同一后台任务；取消命令使用独立信号，主请求已取消也能发出。
	const cancelOnAbort = () => {
		void cancelOrderRefreshJob(start.job_id, { timeoutMs: orderRefreshCancelTimeoutMs }).catch(/* 取消请求失败时忽略网络错误，主请求仍按取消语义结束。 */ () => undefined);
	};
	options?.signal?.addEventListener('abort', cancelOnAbort, { once: true });
	// pollLimit 限制前端等待后台任务的轮询次数。
	const pollLimit = options?.pollLimit ?? orderRefreshPollLimit;
	// pollIntervalMs 是本次状态查询之间的等待时长，测试可缩短它验证超时取消而不改变默认产品体验。
	const pollIntervalMs = options?.pollIntervalMs ?? 500;
	// pollIndex 表示当前订单刷新任务状态轮询次数。
	let pollIndex = 0;
	try {
		while (pollIndex < pollLimit) {
			// job 表示当前轮询得到的后台任务状态。
			const job = await get<OrderRefreshJobStatusResponse>(`/api/v1/orders/refresh/${start.job_id}`, undefined, options);
			options?.onProgress?.(job);
		if (job.status === 'succeeded' && job.result) {
			return job.result;
		}
		if (job.status === 'failed' || job.status === 'cancelled') {
			throw new Error(job.error_message || '订单刷新任务失败');
		}
		// waitMs 是下一次任务状态轮询前的等待时间。
			const waitMs = pollIntervalMs;
		await new Promise<void>(/* 轮询等待器负责等待下一次任务状态查询。 */ (resolve, reject) => {
			// abort 负责响应调用方取消轮询。
			const abort = () => {
				globalThis.clearTimeout(timer);
				reject(new Error('请求已取消'));
			};
			// timer 表示当前轮询等待定时器。
			const timer = globalThis.setTimeout(/* 轮询完成回调清理取消监听并结束等待。 */ () => {
				options?.signal?.removeEventListener('abort', abort);
				resolve();
			}, waitMs);
			if (!options?.signal) return;
			if (options.signal.aborted) abort();
			else options.signal.addEventListener('abort', abort, { once: true });
		});
		pollIndex += 1;
		}
	} finally {
		options?.signal?.removeEventListener('abort', cancelOnAbort);
	}
	// finalJob 保存取消命令与终态竞争后的最终任务状态；成功或失败终态优先于“等待超时”展示。
	const finalJob = await cancelAndReadOrderRefreshJob(start.job_id);
	options?.onProgress?.(finalJob);
	if (finalJob.status === 'succeeded' && finalJob.result) {
		return finalJob.result;
	}
	if (finalJob.status === 'failed') {
		throw new Error(finalJob.error_message || '订单刷新任务失败');
	}
	throw new Error('订单刷新任务等待超时，已请求取消');
};

/** 取消订单刷新任务后读取一次独立终态，解决取消响应和 worker 完成响应同时到达的竞态。 */
const cancelAndReadOrderRefreshJob = async (jobId: string): Promise<OrderRefreshJobStatusResponse> => {
	try {
		await cancelOrderRefreshJob(jobId, { timeoutMs: orderRefreshCancelTimeoutMs });
	} catch {
		// 取消返回冲突或网络错误时仍读取终态：任务可能已经在取消命令到达前结束。
	}
	return get<OrderRefreshJobStatusResponse>(`/api/v1/orders/refresh/${jobId}`, undefined, { timeoutMs: orderRefreshCancelTimeoutMs });
};

// cancelOrderRefreshJob 请求取消当前用户的订单刷新后台任务。
export const cancelOrderRefreshJob = async (jobId: string, options?: RequestControlOptions): Promise<OrderRefreshJobCancelResponse> => {
	return del(`/api/v1/orders/refresh/${jobId}`, undefined, options);
};

// syncSingleOrder 同步单个订单。
export const syncSingleOrder = async (orderId: string): Promise<OrderSingleRefreshResponse> => {
  return post(`/api/v1/orders/${orderId}/refresh`);
};

// manualShipOrder 手动发货订单。
export const manualShipOrder = async (orderIds: string[], shipMode: 'status_only' | 'full_delivery'): Promise<OrderBatchResponse> => {
    return post('/api/v1/orders/manual-ship', {
        order_ids: orderIds,
        ship_mode: shipMode,
    });
}

/** requestRedFlower 请求平台向指定订单买家发送官方求花卡片。 */
export const requestRedFlower = async (orderId: string): Promise<RedFlowerRequestResponse> => {
	return post(`/api/v1/orders/${orderId}/request-red-flower`);
};

/** getRedFlowerStatus 读取订单持久化求花状态，不触发平台动作。 */
export const getRedFlowerStatus = async (orderId: string): Promise<RedFlowerRequestResponse> => {
	return get(`/api/v1/orders/${orderId}/request-red-flower`);
};

// importOrders 导入订单。
export const importOrders = async (data: Partial<Order>[] | FormData, options?: RequestControlOptions): Promise<OrderBatchResponse> => {
	// isFormData 是否为表单请求，用于当前 API 处理流程。
	const isFormData = data instanceof FormData;
	return isFormData ? postForm('/api/v1/orders/import', data, options) : post('/api/v1/orders/import', data, options);
}
