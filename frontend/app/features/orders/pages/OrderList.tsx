import { ChevronLeft,ChevronRight,Edit,Eye,FileImage,Flower2,MessageCircleMore,PackageCheck,Plus,RefreshCw,Save,Trash2,Truck,User as UserIcon,WalletCards,X } from 'lucide-react';
import React from 'react';
import { createPortal } from 'react-dom';
import { formatLocalDateTime } from '../../../../dateTime';
import type { Order,OrderStatus } from '../api';
import { OrderFilterBar } from '../components/OrderFilterBar';
import { OrderImportModal } from '../components/OrderImportModal';
import { OrderSyncProgressCard } from '../components/OrderSyncProgressCard';
import { ShipmentProofModal } from '../components/ShipmentProofModal';
import { useOrderImport,useOrderQuery } from '../hooks';
import { useOrderActions } from '../orderActions';
import type { OrderAmountRange,OrderCreatedRange } from '../types';

// StatusBadge 渲染订单状态徽标。
const StatusBadge: React.FC<{ /** status 表示状态。 */ status: OrderStatus }> = ({ status }) => {
  // styles 样式表。
  const styles = {
    processing: 'bg-blue-100 text-blue-800',
    pending_ship: 'bg-brand text-white',
    shipped: 'bg-blue-100 text-blue-700',
		received: 'bg-cyan-100 text-cyan-700',
    completed: 'bg-green-100 text-green-700',
    cancelled: 'bg-gray-100 text-gray-500',
    refunding: 'bg-red-100 text-red-600',
		refunded: 'bg-fuchsia-100 text-fuchsia-700',
    unknown: 'bg-gray-100 text-gray-500',
  };

  // labels labels，负责当前功能中的对应处理。
  const labels = {
    processing: '处理中',
    pending_ship: '待发货',
    shipped: '已发货',
		received: '已收货',
    completed: '已完成',
    cancelled: '已取消',
    refunding: '退款中',
		refunded: '已退款',
    unknown: '未知',
  };

  return (
    <span className={`px-3 py-1.5 rounded-lg text-xs font-bold ${styles[status] || styles.cancelled}`}>
      {labels[status] || status}
    </span>
  );
};

// AutomaticEnrichmentOrder 描述判断实时卡片临时订单所需的最小非敏感字段。
interface AutomaticEnrichmentOrder {
	/** amount 是订单当前实付金额文本。 */
	amount?: string;
	/** buyer_id 是本地记录的买家平台标识。 */
	buyer_id?: string;
	/** buyer_name 是订单列表关联到的买家昵称。 */
	buyer_name?: string;
	/** cookie_id 是订单所属卖家店铺标识。 */
	cookie_id: string;
	/** status 是本地规范化后的订单生命周期状态。 */
	status: OrderStatus;
}

// orderNeedsAutomaticEnrichment 判断订单列表事实是否仍是实时卡片生成的临时资料，需要所属店铺执行一次快速增量补全。
export const orderNeedsAutomaticEnrichment = (order: AutomaticEnrichmentOrder): boolean => {
	// amountValue 是去除货币符号后得到的数值；非正数不能作为真实实付金额展示。
	const amountValue = Number.parseFloat(String(order.amount || '').replace(/[^0-9.-]/g, ''));
	// missingBuyerIdentity 表示买家仍缺少昵称／ID，或实时卡片误把卖家账号写成买家。
	const missingBuyerIdentity = !order.buyer_id || order.buyer_id === order.cookie_id || !order.buyer_name;
	// terminal 表示订单已经进入无需继续补全交易资料的本地终态。
	const terminal = order.status === 'cancelled' || order.status === 'refunded';
	return !terminal && ((!Number.isFinite(amountValue) || amountValue <= 0) || missingBuyerIdentity);
};

// OrderList 渲染订单列表组件。
const OrderList: React.FC = () => {
  // orderQuery 负责订单查询、筛选、分页和展示辅助数据。
  const orderQuery = useOrderQuery();
  // importState 负责订单导入弹窗、上传取消和失败重试。
  const importState = useOrderImport(orderQuery.loadOrders);
  // { 解构得到当前 Hook 返回的状态和操作函数。
	const { orders, accounts, filter, setFilter, accountFilter, setAccountFilter, createdRange, setCreatedRange, amountRange, setAmountRange, searchText, setSearchText, page, setPage, totalPages, settlementSummary, loading, loadOrders, accountName, accountNickname, getItemNameById } = orderQuery;
  // orderActions 集中管理订单动作、弹窗状态和异步结果。
  const orderActions = useOrderActions({ orders, page, accountFilter, filter, setPage, loadOrders });
  // actionState 解构得到页面动作协调器的状态和操作函数。
  const {
    showDetailModal,
    selectedOrder,
    showEditModal,
    editingOrder,
    showShipModal,
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
  } = orderActions;

  // selectedRedFlowerStatus 是当前详情订单对应的持久求花状态。
  const selectedRedFlowerStatus = selectedOrder && redFlowerStatus?.orderId === selectedOrder.order_id ? redFlowerStatus : null;
  // redFlowerBlocked 表示服务端状态要求永久或暂时禁止重复求花。
  const redFlowerBlocked = selectedRedFlowerStatus ? ['running', 'succeeded', 'succeeded_with_warning', 'needs_review'].includes(selectedRedFlowerStatus.status) : false;
	// copiedValueKey 保存刚完成复制的字段与订单组合键；setCopiedValueKey 驱动对应字段的短暂成功反馈。
	const [copiedValueKey, setCopiedValueKey] = React.useState('');
	// copyFeedbackTimerRef 保存复制成功提示的定时器，由组件卸载或下一次复制负责清理。
	const copyFeedbackTimerRef = React.useRef<number | null>(null);
	// autoEnrichmentAttempts 保存本次页面生命周期已经触发过补全的订单，防止平台暂未就绪时形成同步循环。
	const autoEnrichmentAttempts = React.useRef<Set<string>>(new Set());
	// shipmentProofOrderID 保存当前打开凭证弹窗的精确订单号；空值表示关闭。
	const [shipmentProofOrderID, setShipmentProofOrderID] = React.useState('');

	React.useEffect(/* automaticEnrichmentEffect 在当前页发现临时新订单时自动同步所属店铺，不要求用户组合两个刷新按钮。 */ () => {
		if (loading || syncRunning) return;
		// incompleteOrder 是当前页第一笔尚未尝试自动补全的临时订单。
		const incompleteOrder = orders.find(/* incompleteOrderPredicate 筛选需要自动补全且本页尚未尝试的订单。 */ order => orderNeedsAutomaticEnrichment(order) && !autoEnrichmentAttempts.current.has(order.order_id));
		if (!incompleteOrder) return;
		// relatedOrder 是同一店铺本页中的其他临时订单；一次店铺增量会同时补全，全部标记可避免重复发起相同任务。
		for (const /* relatedOrder 是同一店铺当前待登记尝试状态的临时订单。 */ relatedOrder of orders) {
			if (relatedOrder.cookie_id === incompleteOrder.cookie_id && orderNeedsAutomaticEnrichment(relatedOrder)) {
				autoEnrichmentAttempts.current.add(relatedOrder.order_id);
			}
		}
		void handleEnrichIncompleteOrder(incompleteOrder);
	}, [handleEnrichIncompleteOrder, loading, orders, syncRunning]);

	React.useEffect(/* copyFeedbackCleanup 注册订单页卸载时的复制反馈定时器清理逻辑。 */ () => {
		// cleanupCopyFeedback 避免晚到定时器在订单页卸载后继续更新复制反馈状态。
		const cleanupCopyFeedback = /* copyFeedbackTimerCleanup 释放当前仍存活的复制反馈定时器。 */ (): void => {
			if (copyFeedbackTimerRef.current !== null) {
				window.clearTimeout(copyFeedbackTimerRef.current);
			}
		};
		return cleanupCopyFeedback;
	}, []);

	// handleCopyText 把订单号或买家 ID 写入剪贴板，并用字段组合键隔离每个复制入口的成功反馈。
	const handleCopyText = React.useCallback(/* copyTextAction 接收反馈组合键、待复制文本和失败时的字段名称。 */ async (feedbackKey: string, value: string, fallbackLabel: string): Promise<void> => {
		try {
			await navigator.clipboard.writeText(value);
		} catch {
			window.prompt(`复制${fallbackLabel}`, value);
			return;
		}
		setCopiedValueKey(feedbackKey);
		if (copyFeedbackTimerRef.current !== null) {
			window.clearTimeout(copyFeedbackTimerRef.current);
		}
		copyFeedbackTimerRef.current = window.setTimeout(/* copyFeedbackReset 在 1.5 秒后恢复 ID 标签的默认提示。 */ () => {
			setCopiedValueKey('');
			copyFeedbackTimerRef.current = null;
		}, 1500);
	}, []);

  // handleFilterChange 切换订单状态筛选并回到第一页。
  const handleFilterChange = (value: string) => {
    setFilter(value);
    setPage(1);
    setSearchText('');
  };
  // handleAccountFilterChange 切换账号筛选并回到第一页。
  const handleAccountFilterChange = (value: string) => {
    setAccountFilter(value);
    setPage(1);
  };
  // handleCreatedRangeChange 应用下单时间范围并回到第一页，避免旧页码落入空结果页。
  const handleCreatedRangeChange = (value: OrderCreatedRange) => {
    setCreatedRange(value);
    setPage(1);
  };
  // handleAmountRangeChange 应用实付金额范围并回到第一页，避免旧页码落入空结果页。
  const handleAmountRangeChange = (value: OrderAmountRange) => {
    setAmountRange(value);
    setPage(1);
  };
  // handleSearchChange 更新订单搜索文本并回到第一页。
  const handleSearchChange = (value: string) => {
    setSearchText(value);
    setPage(1);
  };
	// handleContactBuyer 通过账号、精确会话和商品标识导航到订单对应的聊天上下文。
	const handleContactBuyer = (order: Order) => {
		// chatID 是订单持久化的精确会话标识；缺失时禁止按买家昵称猜测。
		const chatID = String(order.chat_id || '').trim();
		if (!chatID) return;
		// parameters 保存 Chat 首次加载需要消费的精确账号、会话和商品上下文。
		const parameters = new URLSearchParams({ account_id: order.cookie_id, chat_id: chatID, item_id: order.item_id });
		window.history.pushState({}, '', `/app/chat?${parameters.toString()}`);
		window.dispatchEvent(new PopStateEvent('popstate'));
	};

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex flex-col md:flex-row justify-between md:items-end gap-4">
        <div>
          <h2 className="text-4xl font-extrabold text-gray-900 tracking-tight">订单中心</h2>
          <p className="text-gray-500 mt-2 font-medium">查看所有闲鱼交易记录与状态。</p>
        </div>
        <div className="flex items-center gap-3">
            <button onClick={loadOrders} className="p-3 rounded-2xl bg-white border border-gray-100 text-gray-600 hover:bg-gray-50 hover:text-black transition-colors shadow-sm">
                <RefreshCw className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <button
			  onClick={importState.openImportModal}
              className="px-5 py-3 rounded-2xl font-bold bg-gray-900 text-white hover:bg-gray-800 transition-colors text-sm flex items-center gap-2 shadow-lg"
            >
              <Plus className="w-4 h-4" />
              插入订单
            </button>
            <button onClick={handleFullSync} disabled={syncRunning} title="读取全部订单并校准远端已删除记录" className="px-5 py-3 rounded-2xl border border-gray-200 bg-white font-bold text-gray-700 hover:bg-gray-50 text-sm flex items-center gap-2 disabled:cursor-not-allowed disabled:opacity-60">
				<RefreshCw className="w-4 h-4" />
				全量校准
			</button>
            <button onClick={handleSync} disabled={syncRunning} className="ios-btn-primary px-6 py-3 rounded-2xl font-bold shadow-lg shadow-blue-200 text-sm flex items-center gap-2 disabled:cursor-not-allowed disabled:opacity-60">
                {syncRunning ? <RefreshCw className="w-5 h-5 animate-spin" /> : <Truck className="w-5 h-5" />}
                {syncRunning ? '订单同步中' : '增量同步订单'}
            </button>
        </div>
      </div>

	  <OrderSyncProgressCard job={syncJob} starting={syncStarting} error={syncError} onCancel={handleCancelSync} />

	  <section data-testid="order-settlement-summary" aria-busy={loading} className="flex flex-col gap-4 rounded-2xl border border-blue-100 bg-gradient-to-r from-blue-50 via-white to-cyan-50 px-5 py-4 shadow-sm md:flex-row md:items-center md:justify-between">
		<div className="flex min-w-0 items-center gap-4">
		  <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-600 text-white shadow-lg shadow-blue-200"><WalletCards className="h-5 w-5" /></div>
		  <div>
			<div className="text-xs font-bold text-blue-600">待结算金额</div>
			<div className="mt-1 text-3xl font-black tabular-nums text-slate-950">¥{settlementSummary.pending_amount}</div>
			<div className="mt-1 text-xs text-slate-500">当前筛选范围 · {settlementSummary.order_count} 笔已发货订单</div>
		  </div>
		</div>
		<div className="grid grid-cols-2 gap-x-8 gap-y-2 text-sm md:text-right">
		  <div>
			<div className="text-xs text-slate-400">已发货总额</div>
			<div className="mt-1 font-bold tabular-nums text-slate-800">¥{settlementSummary.gross_amount}</div>
		  </div>
		  <div>
			<div className="text-xs text-slate-400">平台服务费（{settlementSummary.service_fee_rate}）</div>
			<div className="mt-1 font-bold tabular-nums text-amber-600">-¥{settlementSummary.service_fee}</div>
		  </div>
		</div>
	  </section>

	  <div className="ios-card rounded-xl overflow-hidden shadow-lg border-0 bg-white">
        <OrderFilterBar
          filter={filter}
          onFilterChange={handleFilterChange}
          accountFilter={accountFilter}
          onAccountFilterChange={handleAccountFilterChange}
          createdRange={createdRange}
          onCreatedRangeChange={handleCreatedRangeChange}
          amountRange={amountRange}
          onAmountRangeChange={handleAmountRangeChange}
          accounts={accounts}
          accountName={accountName}
          searchText={searchText}
          onSearchChange={handleSearchChange}
        />

        {/* Table */}
        <div className="overflow-x-auto min-h-[400px]">
          <table className="w-full text-left border-collapse table-fixed">
            <thead>
              <tr className="bg-white text-gray-400 text-xs font-bold uppercase tracking-wider border-b border-gray-50">
                <th className="px-6 py-5" style={{width: '27%'}}>订单信息</th>
                <th className="px-6 py-5" style={{width: '27%'}}>买家信息</th>
                <th className="px-6 py-5" style={{width: '11%'}}>实付金额</th>
                <th className="px-6 py-5" style={{width: '13%'}}>当前状态</th>
                <th className="px-6 py-5 text-right" style={{width: '22%'}}>操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {orders.map(/* 当前回调处理集合中的单个元素。 */ (order) => (
                <tr key={order.id} className="hover:bg-warning-50/50 transition-colors group">
                  <td className="align-top px-6 py-5">
                    <div className="flex flex-col">
                      <div className="flex min-h-14 items-start gap-3">
                        <div className="w-14 h-14 rounded-xl bg-gray-100 overflow-hidden shadow-sm border border-gray-100 flex-shrink-0">
                          {order.item_image ? (
                              <img src={order.item_image} alt="" className="w-full h-full object-cover" />
                          ) : (
                              <div className="w-full h-full flex items-center justify-center text-gray-300"><PackageCheck /></div>
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="line-clamp-2 text-sm font-bold leading-5 text-gray-900" title={getItemNameById(order.cookie_id, order.item_id, order.item_title)}>
                            {getItemNameById(order.cookie_id, order.item_id, order.item_title)}
                          </div>
                        </div>
                      </div>
                      <div className="mt-3 grid grid-cols-[64px_minmax(0,1fr)] gap-x-3 gap-y-1.5 border-t border-gray-100 pt-3 text-xs leading-5">
                        <div className="text-gray-400">订单号</div>
                        <button type="button" onClick={/* 当前回调复制该行完整订单号。 */ () => void handleCopyText(`order:${order.order_id}`, order.order_id, '订单号')} className={`inline-flex w-fit min-w-0 max-w-full cursor-copy items-center gap-1.5 rounded-md px-1.5 py-0.5 text-left transition-colors focus:outline-none focus:ring-2 focus:ring-blue-200 ${copiedValueKey === `order:${order.order_id}` ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`} title={copiedValueKey === `order:${order.order_id}` ? '订单号已复制' : '点击复制订单号'} aria-label={`复制订单号 ${order.order_id}`}><span className="min-w-0 truncate font-mono">{order.order_id}</span>{copiedValueKey === `order:${order.order_id}` && <span className="shrink-0 text-[10px] font-medium text-green-600">已复制</span>}</button>
                        <div className="text-gray-400">账号</div>
                        <div className="flex min-w-0 items-center">
                          <div className="flex w-fit min-w-0 max-w-full items-center gap-1 rounded-md bg-blue-50 px-1.5 py-0.5 text-[10px] font-bold text-blue-700" title={accountNickname(order.cookie_id)}>
                            <UserIcon className="h-3 w-3 shrink-0" />
                            <span className="min-w-0 truncate whitespace-nowrap">{accountNickname(order.cookie_id)}</span>
                          </div>
                        </div>
                        <div className="text-gray-400">下单</div>
                        <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                          <span className="shrink-0 text-gray-600">×{order.quantity}</span>
                          <span className="text-gray-300">·</span>
                          <span className="shrink-0 whitespace-nowrap text-gray-500">{formatLocalDateTime(order.created_at)}</span>
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="align-top px-6 py-5">
                      <div className="flex flex-col">
						  <div className="flex min-h-14 min-w-0 items-start gap-2.5">
							  <div className="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-blue-50 text-blue-600">
								  {order.buyer_avatar_url ? <img src={order.buyer_avatar_url} alt="" className="h-full w-full object-cover" /> : <UserIcon className="h-4 w-4" />}
							  </div>
							  <div className="min-w-0 flex-1">
								  <div className="text-xs text-gray-500">买家</div>
								  <div data-testid={`buyer-identity-${order.order_id}`} className="mt-0.5 flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
									  <span className={`min-w-0 max-w-full truncate text-sm font-bold ${order.buyer_name ? 'text-gray-900' : 'text-gray-400'}`} title={order.buyer_name || '未获取昵称'}>{order.buyer_name || '未获取昵称'}</span>
									  <button type="button" onClick={/* 当前回调复制该行完整买家 ID。 */ () => void handleCopyText(`buyer:${order.order_id}`, order.buyer_id, '买家 ID')} className={`inline-flex shrink-0 cursor-copy items-center whitespace-nowrap rounded-md px-1.5 py-0.5 text-[11px] font-normal transition-colors focus:outline-none focus:ring-2 focus:ring-blue-200 ${copiedValueKey === `buyer:${order.order_id}` ? 'bg-green-50 text-green-600' : 'bg-gray-100 text-gray-400 hover:bg-gray-200'}`} title={copiedValueKey === `buyer:${order.order_id}` ? '买家 ID 已复制' : '点击复制买家 ID'} aria-label={`复制买家 ID ${order.buyer_id}`}>{copiedValueKey === `buyer:${order.order_id}` ? '已复制' : 'ID'} <span className={`ml-1 font-mono ${copiedValueKey === `buyer:${order.order_id}` ? 'text-green-700' : 'text-gray-600'}`}>{order.buyer_id}</span></button>
								  </div>
							  </div>
						  </div>
						  <div className="mt-3 grid grid-cols-[64px_minmax(0,1fr)] gap-x-3 gap-y-1.5 border-t border-gray-100 pt-3 text-xs leading-5">
                              {order.receiver_name && (
                                  <>
                                      <div className="text-gray-400">收货人</div>
                                      <div className="min-w-0 text-gray-700">{order.receiver_name}</div>
                                  </>
                              )}
                              {order.receiver_phone && (
                                  <>
                                      <div className="text-gray-400">联系电话</div>
                                      <div className="min-w-0 font-mono text-gray-700">{order.receiver_phone}</div>
                                  </>
                              )}
                              {order.receiver_address && (
                                  <>
                                      <div className="text-gray-400">收货地址</div>
                                      <div className="min-w-0 whitespace-normal break-words text-gray-700">{order.receiver_address}</div>
                                  </>
                              )}
						  </div>
                      </div>
                  </td>
                  <td className="px-6 py-5 text-base font-extrabold text-gray-900 font-feature-settings-tnum">
                    {Number.parseFloat(order.amount || '') > 0 ? `¥${order.amount}` : <span className="text-xs text-amber-600 font-bold">资料同步中</span>}
                  </td>
                  <td className="px-6 py-5">
                    <StatusBadge status={order.status} />
                  </td>
                  <td className="px-6 py-5 text-right">
                    {order.status === 'pending_ship' && (
                        <button
                            onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleShip(order.order_id)}
                            className="mr-2 text-white bg-black hover:bg-gray-800 shadow-lg shadow-gray-200 text-xs font-bold px-3 py-2 rounded-xl transition-all active:scale-95"
                        >
                            立即发货
                        </button>
                    )}
					<button type="button" onClick={/* openShipmentProof 打开该订单本地保存的 ERP 发货凭证。 */ () => setShipmentProofOrderID(order.order_id)} disabled={!order.shipment_proof_available} className="mr-2 inline-flex p-2 text-gray-400 transition-colors hover:bg-blue-50 hover:text-blue-600 disabled:cursor-not-allowed disabled:opacity-30" title={order.shipment_proof_available ? '查看发货凭证' : order.shipment_proof_unavailable_reason || '暂无可查看发货凭证'} aria-label={`查看发货凭证 ${order.order_id}`}><FileImage className="h-4 w-4" /></button>
					<button
						type="button"
						onClick={/* contactBuyerAction 打开该订单精确关联的商品聊天。 */ () => handleContactBuyer(order)}
						disabled={!String(order.chat_id || '').trim()}
						className="mr-2 inline-flex p-2 text-gray-400 transition-colors hover:bg-blue-50 hover:text-blue-600 disabled:cursor-not-allowed disabled:opacity-30"
						title={order.chat_id ? '联系买家' : '该订单尚未关联精确聊天会话'}
						aria-label={`联系买家 ${order.buyer_name || order.buyer_id}`}
					>
						<MessageCircleMore className="h-4 w-4" />
					</button>
                    <button
                      onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleViewDetail(order)}
                      className="mr-2 text-gray-400 hover:text-blue-600 p-2 rounded-xl hover:bg-blue-50 transition-colors"
                      title="查看详情"
                    >
                      <Eye className="w-4 h-4" />
                    </button>
                    <button
                      onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleEdit(order)}
                      className="mr-2 text-gray-400 hover:text-black p-2 rounded-xl hover:bg-gray-100 transition-colors"
                      title="编辑订单"
                    >
                      <Edit className="w-4 h-4" />
                    </button>
                    <button
                      onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleSyncSingle(order.order_id)}
                      disabled={syncingOrderId === order.order_id}
                      className="mr-2 text-gray-400 hover:text-green-600 p-2 rounded-xl hover:bg-green-50 transition-colors disabled:opacity-50"
                      title="同步订单"
                    >
                      <RefreshCw className={`w-4 h-4 ${syncingOrderId === order.order_id ? 'animate-spin' : ''}`} />
                    </button>
                    <button
                      onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleDelete(order.order_id)}
                      disabled={deletingOrderId === order.order_id}
                      className="text-gray-400 hover:text-red-500 p-2 rounded-xl hover:bg-red-50 transition-colors disabled:opacity-50"
                      title="删除订单"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        <div className="p-4 border-t border-gray-50 flex items-center justify-between bg-white">
            <div className="text-sm text-gray-500 font-medium pl-2">
                第 {page} 页 / 共 {totalPages} 页
            </div>
            <div className="flex gap-2">
                <button
                    disabled={page <= 1}
                    onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setPage(/* 当前回调处理用户交互或异步状态变化。 */ p => p - 1)}
                    aria-label="上一页"
                    className="p-2.5 rounded-xl bg-gray-50 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed text-gray-600 transition-colors"
                >
                    <ChevronLeft className="w-5 h-5" />
                </button>
                <button
                    disabled={page >= totalPages}
                    onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setPage(/* 当前回调处理用户交互或异步状态变化。 */ p => p + 1)}
                    aria-label="下一页"
                    className="p-2.5 rounded-xl bg-gray-50 hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed text-gray-600 transition-colors"
                >
                    <ChevronRight className="w-5 h-5" />
                </button>
            </div>
        </div>
      </div>

      {/* 订单详情弹窗 - 使用 Portal */}
      {showDetailModal && selectedOrder && createPortal(
        <div className="modal-overlay-centered">
          <div className="modal-container">
            <div className="modal-header">
              <div className="flex items-center justify-between w-full">
                <h3 className="text-2xl font-extrabold text-gray-900">订单详情</h3>
                <button
                  onClick={closeDetailModal}
                  className="p-2 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors"
                >
                  <X className="w-5 h-5 text-gray-600" />
                </button>
              </div>
            </div>

            <div className="modal-body space-y-6">
              {/* Order Info */}
              <div className="space-y-4">
                <h4 className="text-lg font-bold text-gray-800">订单信息</h4>
                <div className="grid grid-cols-2 gap-4 p-4 bg-gray-50 rounded-xl">
                  <div>
                    <div className="text-xs text-gray-500 mb-1">订单号</div>
                    <div className="font-mono text-sm font-bold text-gray-900">{selectedOrder.order_id}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500 mb-1">所属账号</div>
                    <div className="truncate whitespace-nowrap text-sm font-bold text-blue-700" title={accountNickname(selectedOrder.cookie_id)}>{accountNickname(selectedOrder.cookie_id)}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500 mb-1">状态</div>
                    <StatusBadge status={selectedOrder.status} />
                  </div>
                  <div>
                    <div className="text-xs text-gray-500 mb-1">实付金额</div>
                    <div className="text-lg font-extrabold text-gray-900">{Number.parseFloat(selectedOrder.amount || '') > 0 ? `¥${selectedOrder.amount}` : '资料同步中'}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500 mb-1">数量</div>
                    <div className="font-bold text-gray-900">{selectedOrder.quantity}</div>
                  </div>
                  <div className="col-span-2">
                    <div className="text-xs text-gray-500 mb-1">创建时间</div>
                    <div className="text-sm font-medium text-gray-700">{formatLocalDateTime(selectedOrder.created_at)}</div>
                  </div>
                </div>
              </div>

				{/* 订单生命周期只展示服务端确认的独立里程碑，不从最终状态反推缺失时间。 */}
				{(selectedOrder.paid_at || selectedOrder.shipped_at || selectedOrder.received_at || selectedOrder.completed_at || selectedOrder.refunded_at || selectedOrder.cancelled_at) && (
					<div className="space-y-3">
						<h4 className="text-lg font-bold text-gray-800">订单进度</h4>
						<div className="grid grid-cols-1 gap-2 rounded-xl bg-gray-50 p-4 sm:grid-cols-2">
							{selectedOrder.paid_at && <div><div className="text-xs text-gray-500">买家付款</div><div className="mt-1 text-sm font-bold text-gray-800">{formatLocalDateTime(selectedOrder.paid_at)}</div></div>}
							{selectedOrder.shipped_at && <div><div className="text-xs text-gray-500">卖家发货</div><div className="mt-1 text-sm font-bold text-gray-800">{formatLocalDateTime(selectedOrder.shipped_at)}</div></div>}
							{selectedOrder.received_at && <div><div className="text-xs text-gray-500">买家确认收货</div><div className="mt-1 text-sm font-bold text-gray-800">{formatLocalDateTime(selectedOrder.received_at)}</div></div>}
							{selectedOrder.completed_at && <div><div className="text-xs text-gray-500">交易完成</div><div className="mt-1 text-sm font-bold text-gray-800">{formatLocalDateTime(selectedOrder.completed_at)}</div></div>}
							{selectedOrder.refunded_at && <div><div className="text-xs text-gray-500">退款成功</div><div className="mt-1 text-sm font-bold text-fuchsia-700">{formatLocalDateTime(selectedOrder.refunded_at)}</div></div>}
							{selectedOrder.cancelled_at && <div><div className="text-xs text-gray-500">订单取消</div><div className="mt-1 text-sm font-bold text-gray-600">{formatLocalDateTime(selectedOrder.cancelled_at)}</div></div>}
						</div>
					</div>
				)}

              {/* Item Info */}
              <div className="space-y-4">
                <h4 className="text-lg font-bold text-gray-800">商品信息</h4>
                <div className="p-4 bg-gray-50 rounded-xl flex items-center gap-4">
                  {selectedOrder.item_image && (
                    <img src={selectedOrder.item_image} alt="" className="w-20 h-20 rounded-xl object-cover border border-gray-200" />
                  )}
                  <div className="flex-1">
                    <div className="font-bold text-gray-900 mb-1">
                      {getItemNameById(selectedOrder.cookie_id, selectedOrder.item_id, selectedOrder.item_title)}
                    </div>
                    <div className="text-sm text-gray-500">商品ID: {selectedOrder.item_id}</div>
                    {selectedOrder.item_price && (
                      <div className="text-sm text-gray-500 mt-1">标价: ¥{selectedOrder.item_price}</div>
                    )}
                  </div>
                </div>
              </div>

              {/* Buyer Info */}
              <div className="space-y-4">
                <h4 className="text-lg font-bold text-gray-800">买家信息</h4>
                <div className="p-4 bg-gray-50 rounded-xl space-y-3">
                  <div>
					<div className="text-xs text-gray-500 mb-1">买家昵称</div>
					<div className={`font-bold ${selectedOrder.buyer_name ? 'text-gray-900' : 'text-gray-400'}`}>{selectedOrder.buyer_name || '未获取昵称'}</div>
					<div className="mt-2 text-xs text-gray-500">买家 ID</div>
					<div className="mt-1 font-mono text-sm text-gray-700">{selectedOrder.buyer_id}</div>
                  </div>
                  {selectedOrder.receiver_name && (
                    <div>
                      <div className="text-xs text-gray-500 mb-1">收货人</div>
                      <div className="font-medium text-gray-700">{selectedOrder.receiver_name}</div>
                    </div>
                  )}
                  {selectedOrder.receiver_phone && (
                    <div>
                      <div className="text-xs text-gray-500 mb-1">联系电话</div>
                      <div className="font-mono text-sm text-gray-700">{selectedOrder.receiver_phone}</div>
                    </div>
                  )}
                  {selectedOrder.receiver_address && (
                    <div>
                      <div className="text-xs text-gray-500 mb-1">收货地址</div>
                      <div className="text-sm text-gray-700">{selectedOrder.receiver_address}</div>
                    </div>
                  )}
                </div>
              </div>

              {redFlowerResult?.orderId === selectedOrder.order_id && (
                <div className={`rounded-xl p-3 text-sm font-medium ${redFlowerResult.success ? 'bg-rose-50 text-rose-800' : 'bg-red-50 text-red-800'}`} role="status">
                  {redFlowerResult.success ? '✓ ' : '✗ '}{redFlowerResult.message}
                </div>
              )}
              {!redFlowerResult && selectedRedFlowerStatus && selectedRedFlowerStatus.status !== 'not_requested' && (
                <div className={`rounded-xl p-3 text-sm font-medium ${selectedRedFlowerStatus.status === 'failed' || selectedRedFlowerStatus.status === 'unavailable' ? 'bg-red-50 text-red-800' : selectedRedFlowerStatus.status === 'needs_review' ? 'bg-amber-50 text-amber-800' : 'bg-rose-50 text-rose-800'}`} role="status">
                  {selectedRedFlowerStatus.message}
                </div>
              )}
            </div>

            <div className="modal-footer">
              <div className="flex w-full flex-wrap gap-3">
                <button
                  onClick={closeDetailModal}
                  className="min-w-32 flex-1 px-6 py-3 rounded-xl bg-gray-100 hover:bg-gray-200 text-gray-800 font-bold transition-colors"
                >
                  关闭
                </button>
                {['pending_ship', 'shipped', 'received', 'completed'].includes(selectedOrder.status) && (
                  <button
                    onClick={/* requestFlowerClick 请求订单求花前由 Hook 执行二次确认。 */ () => handleRequestRedFlower(selectedOrder.order_id)}
                    disabled={redFlowerLoadingOrderId !== null || redFlowerStatusLoadingOrderId !== null || redFlowerBlocked || (redFlowerResult?.orderId === selectedOrder.order_id && redFlowerResult.success)}
                    className="min-w-48 flex-1 rounded-xl bg-rose-500 px-5 py-3 font-bold text-white shadow-lg shadow-rose-100 transition-colors hover:bg-rose-600 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    <span className="inline-flex items-center justify-center gap-2">
                      {redFlowerLoadingOrderId === selectedOrder.order_id || redFlowerStatusLoadingOrderId === selectedOrder.order_id ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Flower2 className="h-4 w-4" />}
                      {redFlowerLoadingOrderId === selectedOrder.order_id ? '正在求花' : redFlowerStatusLoadingOrderId === selectedOrder.order_id ? '检查求花状态' : selectedRedFlowerStatus?.status === 'succeeded' || selectedRedFlowerStatus?.status === 'succeeded_with_warning' || redFlowerResult?.orderId === selectedOrder.order_id && redFlowerResult.success ? '已向买家求花' : selectedRedFlowerStatus?.status === 'running' ? '求花处理中' : selectedRedFlowerStatus?.status === 'needs_review' ? '求花待核对' : '求买家送小红花'}
                    </span>
                  </button>
                )}
                {selectedOrder.status === 'pending_ship' && (
                  <button
                    onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => {
                      closeDetailModal();
                      handleShip(selectedOrder.order_id);
                    }}
                    className="min-w-32 flex-1 px-6 py-3 rounded-xl ios-btn-primary font-bold shadow-lg shadow-blue-200"
                  >
                    立即发货
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>,
        document.body
      )}

      <OrderImportModal {...importState} />
		<ShipmentProofModal orderID={shipmentProofOrderID} open={shipmentProofOrderID !== ''} onClose={/* closeShipmentProof 关闭当前订单凭证弹窗。 */ () => setShipmentProofOrderID('')} />

      {/* Ship Modal - 发货方式选择 */}
      {showShipModal && createPortal(
        <div className="modal-overlay-centered">
          <div className="modal-container" style={{ maxWidth: '480px' }}>
            <div className="modal-header">
              <div className="flex items-center justify-between w-full">
                <h3 className="text-2xl font-extrabold text-gray-900">立即发货</h3>
                <button
                  onClick={closeShipModal}
                  className="p-2 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors"
                >
                  <X className="w-5 h-5 text-gray-600" />
                </button>
              </div>
            </div>

            <div className="modal-body space-y-4">
              <p className="text-sm text-gray-600">请选择发货方式：</p>

              {/* 选项A: 仅修改发货状态 */}
              <button
                onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => executeShip('status_only')}
                disabled={shipLoading}
                className="w-full text-left p-4 rounded-xl border-2 border-gray-200 hover:border-gray-400 hover:bg-gray-50 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <div className="flex items-start gap-3">
                  <div className="w-10 h-10 rounded-xl bg-blue-100 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <Truck className="w-5 h-5 text-blue-600" />
                  </div>
                  <div>
                    <div className="font-bold text-gray-900 text-sm">仅修改闲鱼发货状态</div>
                    <div className="text-xs text-gray-500 mt-1 leading-relaxed">
                      不实际扣除或发送卡券，仅在闲鱼平台将订单标记为"已发货"。
                      适用于已经给客户发过货、只是忘记在闲鱼修改状态的情况。
                    </div>
                  </div>
                </div>
              </button>

              {/* 选项B: 完整发货流程 */}
              <button
                onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => executeShip('full_delivery')}
                disabled={shipLoading}
                className="w-full text-left p-4 rounded-xl border-2 border-gray-200 hover:border-brand hover:bg-blue-50 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <div className="flex items-start gap-3">
                  <div className="w-10 h-10 rounded-xl bg-blue-100 flex items-center justify-center flex-shrink-0 mt-0.5">
                    <PackageCheck className="w-5 h-5 text-blue-700" />
                  </div>
                  <div>
                    <div className="font-bold text-gray-900 text-sm">完整发货（匹配卡券并发送）</div>
                    <div className="text-xs text-gray-500 mt-1 leading-relaxed">
                      自动匹配发货规则、获取卡券、发送卡券信息给买家，并修改发货状态。
                      适用于订单既没有发送卡券给买家、也没有修改发货状态的情况。
                    </div>
                  </div>
                </div>
              </button>

              {/* 加载状态 */}
              {shipLoading && (
                <div className="flex items-center justify-center gap-2 py-3">
                  <RefreshCw className="w-4 h-4 animate-spin text-gray-500" />
                  <span className="text-sm text-gray-500">正在处理中...</span>
                </div>
              )}

              {/* 结果显示 */}
              {shipResult && (
                <div className={`p-3 rounded-xl text-sm ${shipResult.success ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'}`}>
                  {shipResult.success ? '✓ ' : '✗ '}{shipResult.message}
                </div>
              )}
            </div>

            <div className="modal-footer">
              <button
                onClick={closeShipModal}
                className="w-full px-6 py-3 rounded-xl bg-gray-100 hover:bg-gray-200 text-gray-800 font-bold transition-colors"
              >
                {shipResult?.success ? '完成' : '取消'}
              </button>
            </div>
          </div>
        </div>,
        document.body
      )}

      {/* Edit Modal - 使用 Portal */}
      {showEditModal && editingOrder && createPortal(
        <div className="modal-overlay-centered">
          <div className="modal-container">
            <div className="modal-header">
              <div className="flex items-center justify-between w-full">
                <h3 className="text-2xl font-extrabold text-gray-900">编辑订单</h3>
                <button
                  onClick={closeEditModal}
                  className="p-2 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors"
                >
                  <X className="w-5 h-5 text-gray-600" />
                </button>
              </div>
            </div>

            <div className="modal-body space-y-5">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">订单号</label>
                  <input
                    type="text"
                    value={editingOrder.order_id}
                    disabled
                    className="w-full ios-input px-4 py-3 rounded-xl bg-gray-50 text-gray-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">订单状态</label>
                  <select
                    value={editingOrder.status}
                    onChange={/* 当前回调更新订单状态草稿。 */ (e) => updateEditingOrder({ status: e.target.value as OrderStatus })}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  >
                    <option value="processing">处理中</option>
                    <option value="pending_ship">待发货</option>
                    <option value="shipped">已发货</option>
					<option value="received">已收货</option>
                    <option value="completed">已完成</option>
                    <option value="cancelled">已取消</option>
                    <option value="refunding">退款中</option>
					<option value="refunded">已退款</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">买家ID</label>
                  <input
                    type="text"
                    value={editingOrder.buyer_id}
                    onChange={/* 当前回调更新买家标识草稿。 */ (e) => updateEditingOrder({ buyer_id: e.target.value })}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  />
                </div>
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">实付金额</label>
                  <input
                    type="number"
                    value={editingOrder.amount}
                    onChange={/* 当前回调更新订单金额草稿。 */ (e) => updateEditingOrder({ amount: e.target.value })}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">收货人</label>
                  <input
                    type="text"
                    value={editingOrder.receiver_name || ''}
                    onChange={/* 当前回调更新收货人草稿。 */ (e) => updateEditingOrder({ receiver_name: e.target.value })}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  />
                </div>
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">联系电话</label>
                  <input
                    type="text"
                    value={editingOrder.receiver_phone || ''}
                    onChange={/* 当前回调更新收货电话草稿。 */ (e) => updateEditingOrder({ receiver_phone: e.target.value })}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">收货地址</label>
                <textarea
                  value={editingOrder.receiver_address || ''}
                  onChange={/* 当前回调更新收货地址草稿。 */ (e) => updateEditingOrder({ receiver_address: e.target.value })}
                  rows={2}
                  className="w-full ios-input px-4 py-3 rounded-xl resize-none"
                />
              </div>

              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">商品标题</label>
                <input
                  type="text"
                  value={editingOrder.item_title || ''}
                  onChange={/* 当前回调更新商品标题草稿。 */ (e) => updateEditingOrder({ item_title: e.target.value })}
                  className="w-full ios-input px-4 py-3 rounded-xl"
                />
              </div>
            </div>

            <div className="modal-footer">
              <div className="flex gap-3 w-full">
                <button
                  onClick={closeEditModal}
                  className="flex-1 px-6 py-3 rounded-xl font-bold bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={handleSaveEdit}
                  className="flex-1 ios-btn-primary px-6 py-3 rounded-xl font-bold flex items-center justify-center gap-2"
                >
                  <Save className="w-4 h-4" />
                  保存更改
                </button>
              </div>
            </div>
          </div>
        </div>,
        document.body
      )}
    </div>
  );
};

export default OrderList;
