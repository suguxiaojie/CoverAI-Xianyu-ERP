import { useCallback,useEffect,useRef,useState,type Dispatch,type SetStateAction } from 'react';
import type { Order,OrderRefreshJobStatusResponse,OrderSyncMode } from './api';
import { deleteOrder,getRedFlowerStatus,manualShipOrder,requestRedFlower,syncOrders,syncSingleOrder,updateOrder } from './api';

// OrderShipMode 表示订单发货操作的两种业务模式。
export type OrderShipMode = 'status_only' | 'full_delivery';

// OrderShipResult 描述订单发货弹窗展示的结果信息。
export interface OrderShipResult {
  // success 表示发货操作是否成功。
  success: boolean;
  // message 保存发货操作的用户可见说明。
  message: string;
}

// RedFlowerActionResult 描述订单详情中最近一次手动求花结果。
export interface RedFlowerActionResult {
  // orderId 是结果所属订单，避免切换详情后显示旧请求结果。
  orderId: string;
  // success 表示平台明确接受了求花动作。
  success: boolean;
  // message 是不包含凭证的用户提示。
  message: string;
}

// RedFlowerStatusState 描述从服务端恢复的订单持久求花状态。
export interface RedFlowerStatusState {
  // orderId 是状态所属订单。
  orderId: string;
  // status 是服务端保存的稳定求花状态；unavailable 只表示本次查询失败。
  status: 'not_requested' | 'running' | 'succeeded' | 'succeeded_with_warning' | 'failed' | 'needs_review' | 'unavailable';
  // message 是持久状态对应的用户提示。
  message: string;
  // requestedAt 是求花动作首次开始的 Unix 秒时间戳。
  requestedAt?: number;
}

// OrderActionsOptions 描述订单动作协调器依赖的查询状态和页面操作。
export interface OrderActionsOptions {
  // orders 保存当前分页中的订单，用于删除后的分页判断。
  orders: Order[];
  // page 保存当前订单列表页码。
  page: number;
  // accountFilter 保存当前账号筛选条件。
  accountFilter: string;
  // filter 保存当前订单状态筛选条件。
  filter: string;
  // setPage 更新订单列表页码。
  setPage: Dispatch<SetStateAction<number>>;
  // loadOrders 刷新当前筛选条件下的订单列表。
  loadOrders: () => Promise<void>;
}

// OrderActionsState 暴露订单页面动作、弹窗状态和异步结果。
export interface OrderActionsState {
  // showDetailModal 表示订单详情弹窗是否打开。
  showDetailModal: boolean;
  // selectedOrder 保存当前查看详情的订单。
  selectedOrder: Order | null;
  // showEditModal 表示订单编辑弹窗是否打开。
  showEditModal: boolean;
  // editingOrder 保存当前编辑中的订单草稿。
  editingOrder: Partial<Order> | null;
  // showShipModal 表示订单发货弹窗是否打开。
  showShipModal: boolean;
  // shipOrderId 保存当前待发货订单号。
  shipOrderId: string;
  // shipLoading 表示发货请求是否正在执行。
  shipLoading: boolean;
  // shipResult 保存最近一次发货操作结果。
  shipResult: OrderShipResult | null;
  // redFlowerLoadingOrderId 是当前正在提交求花动作的订单号。
  redFlowerLoadingOrderId: string | null;
  // redFlowerResult 保存当前订单详情中的求花结果。
  redFlowerResult: RedFlowerActionResult | null;
  // redFlowerStatusLoadingOrderId 是当前正在读取持久求花状态的订单号。
  redFlowerStatusLoadingOrderId: string | null;
  // redFlowerStatus 保存当前详情订单的服务端持久求花状态。
  redFlowerStatus: RedFlowerStatusState | null;
  // syncingOrderId 保存当前正在单笔同步的订单号。
  syncingOrderId: string | null;
  // deletingOrderId 保存当前正在删除的订单号。
  deletingOrderId: string | null;
  // syncJob 保存批量订单同步后台任务的最新轮询快照。
  syncJob: OrderRefreshJobStatusResponse | null;
  // syncStarting 表示创建后台同步任务的请求仍在进行。
  syncStarting: boolean;
  // syncError 保存任务创建、轮询、失败或取消的用户可见说明。
  syncError: string;
  // syncRunning 表示批量同步任务仍处于创建、排队或运行状态。
  syncRunning: boolean;
  // handleSync 以默认增量模式同步当前筛选条件下的订单。
  handleSync: () => Promise<void>;
  // handleEnrichIncompleteOrder 为当前页资料不完整的新订单自动执行所属店铺的快速增量补全。
  handleEnrichIncompleteOrder: (order: Order) => Promise<void>;
  // handleFullSync 执行完整远端校准并允许清理缺失订单。
  handleFullSync: () => Promise<void>;
  // handleCancelSync 请求取消当前批量订单同步任务。
  handleCancelSync: () => void;
  // handleShip 打开发货弹窗并选择待发货订单。
  handleShip: (orderId: string) => void;
  // executeShip 执行指定模式的订单发货。
  executeShip: (mode: OrderShipMode) => Promise<void>;
  // handleRequestRedFlower 二次确认后向平台提交一次订单求花。
  handleRequestRedFlower: (orderId: string) => Promise<void>;
  // handleViewDetail 打开指定订单的详情弹窗。
  handleViewDetail: (order: Order) => void;
  // handleEdit 打开指定订单的编辑弹窗。
  handleEdit: (order: Order) => void;
  // handleSaveEdit 保存当前订单编辑草稿。
  handleSaveEdit: () => Promise<void>;
  // updateEditingOrder 更新当前订单编辑草稿的局部字段。
  updateEditingOrder: (patch: Partial<Order>) => void;
  // handleSyncSingle 同步指定的单笔订单。
  handleSyncSingle: (orderId: string) => Promise<void>;
  // handleDelete 删除指定订单并处理分页回退。
  handleDelete: (orderId: string) => Promise<void>;
  // closeDetailModal 关闭订单详情弹窗。
  closeDetailModal: () => void;
  // closeEditModal 关闭订单编辑弹窗。
  closeEditModal: () => void;
  // closeShipModal 关闭订单发货弹窗并清理结果。
  closeShipModal: () => void;
}

// orderErrorMessage 将未知异常转换为稳定的订单动作提示。
const orderErrorMessage = (error: unknown, fallback: string): string => error instanceof Error ? error.message : fallback;

// useOrderActions 集中管理订单同步、发货、编辑、删除和弹窗生命周期。
export const useOrderActions = ({ orders, page, accountFilter, filter, setPage, loadOrders }: OrderActionsOptions): OrderActionsState => {
  // showDetailModal 表示订单详情弹窗是否打开。
  const [showDetailModal, setShowDetailModal] = useState(false);
  // selectedOrder 保存当前查看详情的订单。
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null);
  // showEditModal 表示订单编辑弹窗是否打开。
  const [showEditModal, setShowEditModal] = useState(false);
  // editingOrder 保存当前编辑中的订单草稿。
  const [editingOrder, setEditingOrder] = useState<Partial<Order> | null>(null);
  // showShipModal 表示订单发货弹窗是否打开。
  const [showShipModal, setShowShipModal] = useState(false);
  // shipOrderId 保存当前待发货订单号。
  const [shipOrderId, setShipOrderId] = useState('');
  // shipLoading 表示发货请求是否正在执行。
  const [shipLoading, setShipLoading] = useState(false);
  // shipResult 保存最近一次发货操作结果。
  const [shipResult, setShipResult] = useState<OrderShipResult | null>(null);
  // redFlowerLoadingOrderId 是当前正在请求官方求花卡片的订单号。
  const [redFlowerLoadingOrderId, setRedFlowerLoadingOrderId] = useState<string | null>(null);
  // redFlowerResult 保存当前详情弹窗展示的求花结果。
  const [redFlowerResult, setRedFlowerResult] = useState<RedFlowerActionResult | null>(null);
  // redFlowerInFlight 在 React 状态提交前同步阻止连续点击创建第二个请求。
  const redFlowerInFlight = useRef(false);
  // redFlowerStatusGeneration 保证切换订单后旧状态响应不能覆盖新详情。
  const redFlowerStatusGeneration = useRef(0);
  // redFlowerStatusLoadingOrderId 是当前正在读取求花状态的订单号。
  const [redFlowerStatusLoadingOrderId, setRedFlowerStatusLoadingOrderId] = useState<string | null>(null);
  // redFlowerStatus 保存从服务端恢复的当前订单求花状态。
  const [redFlowerStatus, setRedFlowerStatus] = useState<RedFlowerStatusState | null>(null);
  // syncingOrderId 保存当前正在单笔同步的订单号。
  const [syncingOrderId, setSyncingOrderId] = useState<string | null>(null);
  // deletingOrderId 保存当前正在删除的订单号。
  const [deletingOrderId, setDeletingOrderId] = useState<string | null>(null);
  // syncJob 保存服务端订单刷新任务的最新轻量状态，不保存逐单完整结果。
  const [syncJob, setSyncJob] = useState<OrderRefreshJobStatusResponse | null>(null);
  // syncStarting 表示 POST 创建后台任务尚未返回首个 job_id。
  const [syncStarting, setSyncStarting] = useState(false);
  // syncError 保存批量同步的失败、取消或超时说明。
  const [syncError, setSyncError] = useState('');
  // syncGeneration 区分连续发起的批量刷新，旧任务完成后不得覆盖最新一次用户操作的列表和提示。
  const syncGeneration = useRef(0);
  // syncController 保存当前批量同步轮询的取消控制；取消会同时请求后端停止 worker。
  const syncController = useRef<AbortController | null>(null);
  // syncRunning 从任务创建状态和服务端任务状态派生，避免维护重复布尔状态。
  const syncRunning = syncStarting || syncJob?.status === 'queued' || syncJob?.status === 'running';

  // runSync 按用户选择的增量或完整校准模式同步订单并刷新列表。
  const runSync = useCallback(/* syncAction 执行当前筛选条件或指定店铺的订单同步。 */ async (mode: OrderSyncMode, requestedCookieId?: string) => {
		if (syncController.current) return;
		// generation 是本次批量同步的代次；仅仍为最新代次的结果允许更新界面。
		const generation = ++syncGeneration.current;
    // controller 负责取消本次轮询和对应服务端后台 worker。
    const controller = new AbortController();
    syncController.current = controller;
    setSyncStarting(true);
    setSyncJob(null);
    setSyncError('');
    try {
      // result 保存订单同步接口返回的结果说明。
			const result = await syncOrders(requestedCookieId || accountFilter || undefined, filter, {
				signal: controller.signal,
				onProgress: /* progressUpdate 只接受当前代次的服务端任务快照。 */ job => {
					if (generation !== syncGeneration.current) return;
					setSyncStarting(false);
					setSyncJob(job);
				},
			}, mode);
			if (generation !== syncGeneration.current) return;
      await loadOrders();
			if (generation !== syncGeneration.current) return;
			setSyncError(result.partial_failure ? result.message || '订单同步存在部分失败' : '');
    } catch (/* error 表示订单同步请求异常。 */ error: unknown) {
			if (generation !== syncGeneration.current) return;
      console.error('同步订单失败:', error);
			setSyncError(controller.signal.aborted ? '订单同步已取消' : orderErrorMessage(error, '同步失败，请重试'));
		} finally {
			if (generation === syncGeneration.current) {
				setSyncStarting(false);
				syncController.current = null;
			}
    }
  }, [accountFilter, filter, loadOrders]);

	// handleSync 执行默认增量同步，通常只读取最新两页到可信历史边界。
	const handleSync = useCallback(/* incrementalSyncAction 启动低请求量增量同步。 */ () => runSync('incremental'), [runSync]);
	// handleEnrichIncompleteOrder 只同步资料不完整订单所属店铺，避免新订单要求用户先单刷再全店铺同步。
	const handleEnrichIncompleteOrder = useCallback(/* incompleteOrderSyncAction 启动新订单资料补全。 */ (order: Order) => runSync('incremental', order.cookie_id), [runSync]);
	// handleFullSync 执行完整校准，读取全部分页并处理远端缺失订单。
	const handleFullSync = useCallback(/* fullSyncAction 启动完整订单校准。 */ () => runSync('full'), [runSync]);

  // handleCancelSync 取消当前前端轮询，并由 API 层向服务端发送独立取消请求。
  const handleCancelSync = useCallback(/* cancelSyncAction 取消当前批量同步任务。 */ () => {
		syncController.current?.abort();
	}, []);

  // 组件卸载时取消仍在轮询的任务，避免离开订单页后残留请求和无主后台 worker。
  useEffect(/* syncCleanupEffect 绑定订单页生命周期与同步任务取消。 */ () => {
		return /* syncCleanup 取消卸载页面留下的轮询和后台 worker。 */ () => {
			syncController.current?.abort();
		};
	}, []);

  // handleShip 打开发货弹窗并选择待发货订单。
  const handleShip = useCallback(/* shipAction 打开发货弹窗并保存订单号。 */ (orderId: string) => {
    setShipOrderId(orderId);
    setShipResult(null);
    setShowShipModal(true);
  }, []);

  // executeShip 执行指定模式的订单发货并更新结果。
  const executeShip = useCallback(/* executeShipAction 执行指定模式的订单发货。 */ async (mode: OrderShipMode) => {
    setShipLoading(true);
    setShipResult(null);
    try {
      // response 保存订单批量发货接口响应。
      const response = await manualShipOrder([shipOrderId], mode);
      // result 保存当前订单的发货结果行。
      const result = response.results?.[0];
      if (result?.success) {
        setShipResult({ success: true, message: result.message });
        void loadOrders();
      } else {
        setShipResult({ success: false, message: result?.message || '发货失败' });
      }
    } catch (/* error 表示订单发货请求异常。 */ error: unknown) {
      setShipResult({ success: false, message: orderErrorMessage(error, '请求失败') });
    } finally {
      setShipLoading(false);
    }
  }, [loadOrders, shipOrderId]);

  // loadRedFlowerStatus 读取服务端持久状态，并丢弃切换订单后的迟到响应。
  const loadRedFlowerStatus = useCallback(/* loadFlowerStatusAction 读取单订单求花状态。 */ async (orderId: string) => {
    // generation 是本次状态查询代次。
    const generation = ++redFlowerStatusGeneration.current;
    setRedFlowerStatusLoadingOrderId(orderId);
    try {
      // response 是不触发平台动作的持久状态响应。
      const response = await getRedFlowerStatus(orderId);
      if (generation !== redFlowerStatusGeneration.current) return;
      setRedFlowerStatus({ orderId, status: response.status, message: response.message || '', requestedAt: response.requested_at });
    } catch (/* error 是本次只读状态查询错误。 */ error: unknown) {
      if (generation !== redFlowerStatusGeneration.current) return;
      setRedFlowerStatus({ orderId, status: 'unavailable', message: orderErrorMessage(error, '暂时无法读取求花状态') });
    } finally {
      if (generation === redFlowerStatusGeneration.current) setRedFlowerStatusLoadingOrderId(null);
    }
  }, []);

  // handleRequestRedFlower 二次确认后提交官方求花动作，并在结果不明确时保留服务端人工核对提示。
  const handleRequestRedFlower = useCallback(/* requestFlowerAction 提交单订单求花。 */ async (orderId: string) => {
    if (redFlowerInFlight.current) return;
    if (!window.confirm('将向买家发送闲鱼官方小红花系统卡片。每笔订单最多求花一次，确认发送吗？')) return;
    redFlowerInFlight.current = true;
    setRedFlowerLoadingOrderId(orderId);
    setRedFlowerResult(null);
    try {
      // response 是服务端幂等收口后的官方求花结果。
      const response = await requestRedFlower(orderId);
      setRedFlowerResult({ orderId, success: response.success, message: response.message || '已向买家发送求花卡片' });
      setRedFlowerStatus({ orderId, status: response.status, message: response.message || '', requestedAt: response.requested_at });
    } catch (/* error 是平台拒绝、会话失效或人工核对提示。 */ error: unknown) {
      setRedFlowerResult({ orderId, success: false, message: orderErrorMessage(error, '求花请求失败，请核对订单后重试') });
      void loadRedFlowerStatus(orderId);
    } finally {
      redFlowerInFlight.current = false;
      setRedFlowerLoadingOrderId(null);
    }
  }, [loadRedFlowerStatus]);

  // handleViewDetail 打开指定订单的详情弹窗。
  const handleViewDetail = useCallback(/* detailAction 打开订单详情弹窗。 */ (order: Order) => {
    setSelectedOrder(order);
    setRedFlowerResult(null);
    setRedFlowerStatus(null);
    setShowDetailModal(true);
    void loadRedFlowerStatus(order.order_id);
  }, [loadRedFlowerStatus]);

  // handleEdit 打开指定订单的编辑弹窗并复制编辑草稿。
  const handleEdit = useCallback(/* editAction 打开订单编辑弹窗。 */ (order: Order) => {
    setEditingOrder({ ...order });
    setShowEditModal(true);
  }, []);

  // updateEditingOrder 使用函数式更新合并订单编辑草稿字段。
  const updateEditingOrder = useCallback(/* draftAction 合并订单编辑草稿字段。 */ (patch: Partial<Order>) => {
    setEditingOrder(/* currentDraft 当前订单编辑草稿。 */ current => current ? { ...current, ...patch } : current);
  }, []);

  // handleSaveEdit 保存当前订单编辑草稿并刷新列表。
  const handleSaveEdit = useCallback(/* saveEditAction 保存订单编辑草稿。 */ async () => {
    if (!editingOrder?.order_id) return;
    try {
      // updateData 保存映射到订单更新接口的字段。
      const updateData: Partial<Order> = {};
      if (editingOrder.status !== undefined) updateData.order_status = editingOrder.status;
      if (editingOrder.buyer_id !== undefined) updateData.buyer_id = editingOrder.buyer_id;
      if (editingOrder.amount !== undefined) updateData.amount = editingOrder.amount;
      if (editingOrder.receiver_name !== undefined) updateData.receiver_name = editingOrder.receiver_name;
      if (editingOrder.receiver_phone !== undefined) updateData.receiver_phone = editingOrder.receiver_phone;
      if (editingOrder.receiver_address !== undefined) updateData.receiver_address = editingOrder.receiver_address;
      if (editingOrder.item_id !== undefined) updateData.item_id = editingOrder.item_id;
      if (editingOrder.quantity !== undefined) updateData.quantity = editingOrder.quantity;
      if (editingOrder.item_title !== undefined) updateData.item_title = editingOrder.item_title;

      await updateOrder(editingOrder.order_id, updateData);
      setShowEditModal(false);
      setEditingOrder(null);
      await loadOrders();
    } catch (/* error 表示订单编辑请求异常。 */ error: unknown) {
      console.error('更新订单失败:', error);
      alert('更新失败，请重试');
    }
  }, [editingOrder, loadOrders]);

  // handleSyncSingle 同步指定的单笔订单并刷新列表。
  const handleSyncSingle = useCallback(/* singleSyncAction 执行单笔订单同步。 */ async (orderId: string) => {
    setSyncingOrderId(orderId);
    try {
      // result 保存单笔订单同步接口响应。
      const result = await syncSingleOrder(orderId);
      if (result.success) {
        await loadOrders();
      } else {
        alert(result.message || '同步失败');
      }
    } catch (/* error 表示单笔订单同步请求异常。 */ error: unknown) {
      console.error('同步订单失败:', error);
      alert(orderErrorMessage(error, '同步失败，请重试'));
    } finally {
      setSyncingOrderId(null);
    }
  }, [loadOrders]);

  // handleDelete 删除指定订单并在当前页为空时回退页码。
  const handleDelete = useCallback(/* deleteAction 删除订单并处理分页回退。 */ async (orderId: string) => {
    if (!confirm('确认删除该订单吗？删除后无法恢复。')) return;
    setDeletingOrderId(orderId);
    try {
      await deleteOrder(orderId);
      if (orders.length === 1 && page > 1) {
        setPage(/* currentPage 当前订单页码。 */ current => current - 1);
      } else {
        await loadOrders();
      }
    } catch (/* error 表示订单删除请求异常。 */ error: unknown) {
      console.error('删除订单失败:', error);
      alert(orderErrorMessage(error, '删除失败，请重试'));
      await loadOrders();
    } finally {
      setDeletingOrderId(null);
    }
  }, [loadOrders, orders.length, page, setPage]);

  // closeDetailModal 关闭订单详情弹窗。
  const closeDetailModal = useCallback(/* closeDetailAction 关闭订单详情弹窗。 */ () => {
    redFlowerStatusGeneration.current += 1;
    setRedFlowerStatusLoadingOrderId(null);
    setShowDetailModal(false);
  }, []);
  // closeEditModal 关闭订单编辑弹窗。
  const closeEditModal = useCallback(/* closeEditAction 关闭订单编辑弹窗。 */ () => setShowEditModal(false), []);
  // closeShipModal 关闭订单发货弹窗并清理结果。
  const closeShipModal = useCallback(/* closeShipAction 关闭订单发货弹窗。 */ () => {
    setShowShipModal(false);
    setShipResult(null);
  }, []);

  return {
    showDetailModal,
    selectedOrder,
    showEditModal,
    editingOrder,
    showShipModal,
    shipOrderId,
    shipLoading,
    shipResult,
    redFlowerLoadingOrderId,
    redFlowerResult,
    redFlowerStatusLoadingOrderId,
    redFlowerStatus,
    syncingOrderId,
    deletingOrderId,
    syncJob,
    syncStarting,
    syncError,
    syncRunning,
    handleSync,
    handleEnrichIncompleteOrder,
		handleFullSync,
    handleCancelSync,
    handleShip,
    executeShip,
    handleRequestRedFlower,
    handleViewDetail,
    handleEdit,
    handleSaveEdit,
    updateEditingOrder,
    handleSyncSingle,
    handleDelete,
    closeDetailModal,
    closeEditModal,
    closeShipModal,
  };
};
