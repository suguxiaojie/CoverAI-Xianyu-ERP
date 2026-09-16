import { formatLocalDate } from '../../../dateRange';
import type { AccountDetail,AccountSummaryResponse } from '../../../shared/api-contract/accounts';
import type { DashboardStatsResponse,Item,ItemListEnvelope,Order,OrderAnalyticsResponse,OrderStatus,ValidOrderResponse,ValidOrdersResponse } from '../../../shared/api-contract/admin';
import { del,get,post,postForm,type RequestControlOptions } from '../../../shared/http/client';
import { collectionFrom,objectFrom } from '../../../shared/http/contract';
export type * from '../../../shared/api-contract/admin';

/** 仪表盘订单明细接口转换后的分页结果。 */
export interface ValidOrdersResult {
  /** 当前时间范围的订单明细。 */
  orders: Order[];
  /** 服务端匹配到的订单总数。 */
  total: number;
  /** 是否存在未返回的后续订单。 */
  truncated: boolean;
}

/** Dashboard 选择器和运行概览使用的非敏感账号摘要。 */
export type DashboardAccountSummary = Pick<AccountDetail, 'id' | 'enabled' | 'nickname' | 'remark'>;

/** Dashboard 读取的单账号运行状态，不包含 Cookie、Token 或登录秘密。 */
export interface DashboardRuntimeStatus {
  /** state 是账号运行状态机当前状态。 */
  state: NonNullable<AccountDetail['runtime_state']>;
  /** message 是账号运行状态的非敏感说明。 */
  message?: string;
  /** connected 表示消息 WebSocket 当前是否已连接。 */
  connected: boolean;
  /** failures 是运行时连续失败次数。 */
  failures: number;
  /** updated_at 是运行状态快照更新时间。 */
  updated_at: string;
}

/** 历史成本补全任务的轻量进度。 */
export interface HistoricalCostBackfillProgress {
  /** stage 是后台任务当前阶段。 */
  stage: string;
  /** message 是用户可见的阶段说明。 */
  message: string;
  /** processed 是已经读取详情的订单数。 */
  processed: number;
  /** total 是本次需要读取详情的订单数。 */
  total: number;
  /** succeeded 是详情读取并写入成功的订单数。 */
  succeeded: number;
  /** failed 是详情读取失败的订单数。 */
  failed: number;
  /** percent 是后台任务整体进度百分比。 */
  percent: number;
}

/** 历史成本补全后台任务状态。 */
export interface HistoricalCostBackfillJob {
  /** job_id 是持久化后台任务标识。 */
  job_id: string;
  /** status 是 running、succeeded、failed 或 canceled。 */
  status: string;
  /** error_message 是任务失败原因。 */
  error_message?: string;
  /** progress 是运行中的轻量进度。 */
  progress?: HistoricalCostBackfillProgress;
}

/** 人工历史成本候选中的单个可选 SKU。 */
export interface ManualCostSKU {
  /** sku_id 是平台 SKU 标识。 */ sku_id: string;
  /** label 是规格展示名称。 */ label: string;
  /** price_cents 是当前标准售价分值。 */ price_cents: number;
  /** cost_cents 是当前成本分值。 */ cost_cents: number;
}

/** 按商品和历史成交金额分组的人工成本候选。 */
export interface ManualCostCandidate {
	/** account_id 是该历史订单分组所属账号标识。 */ account_id: string;
  /** item_id 是商品标识。 */ item_id: string;
  /** item_title 是商品标题。 */ item_title: string;
  /** amount_cents 是历史成交金额分值。 */ amount_cents: number;
	/** quantity 是该组每笔订单的购买数量。 */ quantity: number;
  /** order_count 是等待确认的订单数。 */ order_count: number;
  /** suggested_sku_id 是价格距离最近的建议 SKU。 */ suggested_sku_id: string;
  /** confidence 是 exact、adjusted 或 ambiguous。 */ confidence: string;
  /** suggested_unit_cost_cents 是历史确认优先的建议成本分值。 */ suggested_unit_cost_cents: number;
  /** skus 是全部可选成本 SKU。 */ skus: ManualCostSKU[];
	/** manual_only 表示当前分组没有 SKU 可引用，必须填写历史单件成本。 */ manual_only: boolean;
}

/** 将服务端订单状态映射为仪表盘可稳定展示的状态集合。 */
const normalizeOrderStatus = (value: unknown): OrderStatus => {
  // status 是服务端返回的原始或历史订单状态。
  const status = String(value || '');
  if (status === 'paid') return 'pending_ship';
  return ['processing', 'pending_ship', 'shipped', 'received', 'completed', 'cancelled', 'refunding', 'refunded'].includes(status) ? status as OrderStatus : 'unknown';
};

/** 读取仪表盘概览统计。 */
export const getDashboardStats = async (options?: RequestControlOptions): Promise<DashboardStatsResponse> => get('/api/v1/analytics/dashboard', undefined, options);

/** 读取 Dashboard 选择器所需的账号身份摘要，不接收或保存凭证明文。 */
export const getDashboardAccounts = async (options?: RequestControlOptions): Promise<DashboardAccountSummary[]> => {
  // response 是账号详情接口的兼容响应，只提取非敏感摘要字段。
  const response = await get<unknown>('/api/v1/accounts/details', undefined, options);
  // accounts 是从兼容包装中提取的账号摘要列表。
  const accounts = collectionFrom<AccountSummaryResponse>(response, ['data', 'accounts', 'details']);
  return accounts.map(
    // account 是当前转换为 Dashboard 最小账号模型的传输记录。
    account => ({ id: account.id, enabled: account.enabled === true, nickname: account.nickname || '', remark: account.remark || '' }),
  );
};

/** 读取当前用户旗下账号的实时连接状态，用于替换 Dashboard 静态绿灯。 */
export const getDashboardRuntimeStatuses = async (options?: RequestControlOptions): Promise<Record<string, DashboardRuntimeStatus>> => {
  // response 是账号运行状态接口的未知兼容响应。
  const response = await get<unknown>('/api/v1/accounts/runtime-status', undefined, options);
  // statuses 是移除兼容包装后的账号状态索引。
  const statuses = objectFrom<Record<string, DashboardRuntimeStatus>>(response, ['statuses', 'data', 'result']);
  return statuses || {};
};

/** 读取仪表盘商品索引，兼容历史 items 包装。 */
export const getItems = async (accountID?: string, options?: RequestControlOptions): Promise<Item[]> => {
  // response 是商品列表的原始 transport 响应。
  const response = await get<Item[] | ItemListEnvelope>('/api/v1/items', accountID ? { cookie_id: accountID } : undefined, options);
  // items 是从直接数组或历史包装中取出的商品集合。
  const items = collectionFrom<Item>(response, ['items', 'data', 'results']);
  return items.map(/* item 是当前需要兼容布尔标记的商品传输记录。 */ item => ({ ...item, id: item.id || `${item.cookie_id}-${item.item_id}`, is_multi_spec: item.is_multi_spec === true || item.is_multi_spec === 1, multi_quantity_delivery: item.multi_quantity_delivery === true || item.multi_quantity_delivery === 1 }));
};

/** 读取指定自然日范围的订单统计，并显式传递浏览器时区偏移。 */
export const getOrderAnalytics = async (range: number | { /** 统计起始日期。 */ start_date: string; /** 统计结束日期。 */ end_date: string; /** 可选账号范围，空值表示全部账号。 */ account_id?: string } = 7, options?: RequestControlOptions): Promise<OrderAnalyticsResponse> => {
  // dateRange 是标准化后的统计日期范围。
  let dateRange: { /** 统计起始日期。 */ start_date: string; /** 统计结束日期。 */ end_date: string; /** 可选账号范围。 */ account_id?: string };
  if (typeof range === 'number') {
    // endDate 是计算默认区间使用的当前本地日期。
    const endDate = new Date();
    // startDate 是向前回溯指定天数后的本地日期。
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - range);
    dateRange = { start_date: formatLocalDate(startDate), end_date: formatLocalDate(endDate) };
  } else {
    dateRange = range;
  }
  return get('/api/v1/analytics/orders', { ...dateRange, timezone_offset_minutes: -new Date().getTimezoneOffset() }, options);
};

/** 创建只读取历史多规格订单详情的成本补全任务，不执行订单列表同步或平台写入。 */
export const startHistoricalCostBackfill = async (accountID: string, options?: RequestControlOptions): Promise<HistoricalCostBackfillJob> => {
	// formData 明确选择历史成本补全模式，账号为空时遍历当前用户全部账号。
	const formData = new FormData();
	formData.append('mode', 'cost_backfill');
	formData.append('status', 'all');
	if (accountID) formData.append('cookie_id', accountID);
  return postForm<HistoricalCostBackfillJob>('/api/v1/orders/refresh', formData, options);
};

/** 读取历史成本补全后台任务的当前进度。 */
export const getHistoricalCostBackfillJob = async (jobID: string, options?: RequestControlOptions): Promise<HistoricalCostBackfillJob> => get(`/api/v1/orders/refresh/${jobID}`, undefined, options);

/** 停止历史成本补全任务，已生成的精确快照继续保留。 */
export const cancelHistoricalCostBackfillJob = async (jobID: string): Promise<void> => {
  await del(`/api/v1/orders/refresh/${jobID}`);
};

/** 读取仍未覆盖成本的历史订单人工 SKU 候选分组。 */
export const getManualCostCandidates = async (accountID: string, dateRange: { /** start_date 是 Dashboard 当前本地日期起点。 */ start_date: string; /** end_date 是 Dashboard 当前本地日期终点。 */ end_date: string }, options?: RequestControlOptions): Promise<ManualCostCandidate[]> => get('/api/v1/analytics/profit-backfill/candidates', { ...dateRange, account_id: accountID || undefined, timezone_offset_minutes: -new Date().getTimezoneOffset() }, options);

/** 确认一个商品金额分组对应的 SKU，并返回新生成的成本快照数量。 */
export const confirmManualCostCandidate = async (candidate: { /** account_id 是待确认分组所属账号。 */ account_id: string; /** item_id 是历史商品标识。 */ item_id: string; /** amount_cents 是历史金额分值。 */ amount_cents: number; /** quantity 是单笔订单购买数量。 */ quantity: number; /** start_date 是 Dashboard 当前本地日期起点。 */ start_date: string; /** end_date 是 Dashboard 当前本地日期终点。 */ end_date: string; /** timezone_offset_minutes 是浏览器时区分钟偏移。 */ timezone_offset_minutes: number; /** sku_id 是可选的 SKU；纯历史成本分组为空。 */ sku_id: string; /** custom_unit_cost_cents 是用户填写的历史单件成本分值。 */ custom_unit_cost_cents: number }): Promise<{ /** success 表示写入成功。 */ success: boolean; /** matched_orders 是新增快照数。 */ matched_orders: number; /** match_source 是人工匹配来源。 */ match_source: string }> => post('/api/v1/analytics/profit-backfill/confirm', candidate);

/** 读取可参与仪表盘统计的订单并适配历史响应形状。 */
export const getValidOrders = async (dateRange: { /** 统计起始日期。 */ start_date: string; /** 统计结束日期。 */ end_date: string; /** 可选账号范围，空值表示全部账号。 */ account_id?: string }, options?: RequestControlOptions): Promise<ValidOrdersResult> => {
  // response 是有效订单接口的未知兼容响应。
  const response = await get<unknown>('/api/v1/analytics/orders/valid', { ...dateRange, timezone_offset_minutes: -new Date().getTimezoneOffset() }, options);
  // page 是移除历史 transport 包装后的分页元数据。
  const page = objectFrom<Partial<ValidOrdersResponse>>(response, ['data', 'result']) || {};
  // rows 是直接数组或历史 orders 包装中的原始订单。
  const rows = collectionFrom<ValidOrderResponse>(response, ['orders', 'data', 'items']);
  // orders 是转换状态、数量和稳定标识后的仪表盘订单模型。
  const orders = rows.map(/* row 是当前需要归一化状态与数量的有效订单 DTO。 */ row => ({ ...row, id: row.order_id, status: normalizeOrderStatus(row.status || row.order_status), order_status: normalizeOrderStatus(row.order_status), quantity: Number(row.quantity || 1) }));
  return { orders, total: Number(page.total ?? orders.length), truncated: page.truncated === true };
};
