import { AlertTriangle,Check,ChevronLeft,ChevronRight,Copy,Loader2,PackageSearch,RefreshCw,ShoppingBag,X } from 'lucide-react';
import React from 'react';
import { formatLocalDateTime } from '../../../../dateTime';
import type { ConversationOrder,ConversationOrderStatus } from '../api';
import { useConversationOrders } from '../useConversationOrders';

/** ConversationOrderPreviewPanelProps 描述 Chat 订单上下文侧栏所需的精确会话和抽屉状态。 */
type ConversationOrderPreviewPanelProps = {
  /** open 控制中窄屏抽屉，超宽屏始终展示第三栏。 */
  open: boolean;
  /** onClose 关闭中窄屏抽屉，不改变当前会话。 */
  onClose: () => void;
  /** accountID 是订单归属必须匹配的当前聊天账号。 */
  accountID: string;
  /** chatID 是当前会话标识，用于区分当前订单和同买家历史。 */
  chatID: string;
  /** buyerName 是当前会话买家的显示名称。 */
  buyerName: string;
  /** buyerID 是当前会话买家的稳定平台标识。 */
  buyerID: string;
  /** buyerAvatarURL 是当前会话已有头像地址，缺失时使用标准图标。 */
  buyerAvatarURL?: string;
  /** revision 是最新订单系统卡片消息键，用于实时刷新本地投影。 */
  revision: string;
  /** enrichmentOrderID 是最新待付款或已付款卡片的订单号。 */
  enrichmentOrderID?: string;
};

/** ConversationOrderCardProps 描述一张只读订单卡及其复制反馈。 */
type ConversationOrderCardProps = {
  /** order 是服务端返回的最小非敏感订单上下文。 */
  order: ConversationOrder;
  /** current 表示订单已通过账号和会话标识精确关联到当前聊天。 */
  current: boolean;
  /** attention 区分仍在退款处理和已经结束的历史退款，空值表示普通订单。 */
  attention?: 'active' | 'resolved';
  /** copied 表示当前订单号刚完成剪贴板复制。 */
  copied: boolean;
  /** onCopy 由卡片触发订单号复制，不执行订单或平台写操作。 */
  onCopy: (orderID: string) => void;
};

/** HistoryOrderFilter 是同买家历史订单栏公开的四种人工查看范围。 */
type HistoryOrderFilter = 'all' | 'refund' | 'completed' | 'cancelled';

/** historyOrderFilterOptions 保留客服最常用的售后、完成和取消入口，默认展示全部。 */
const historyOrderFilterOptions: ReadonlyArray<{ /** key 是本地筛选值。 */ key: HistoryOrderFilter; /** label 是按钮文案。 */ label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'refund', label: '退款／售后' },
  { key: 'completed', label: '已完成' },
  { key: 'cancelled', label: '已取消' },
];

/** historyOrderNeedsAttention 只把服务端明确返回的退款状态纳入售后警示，不从文案推断风险。 */
const historyOrderNeedsAttention = (order: ConversationOrder): boolean => order.status === 'refunding' || order.status === 'refunded';

/** historyOrderMatchesFilter 按明确订单状态筛选当前已加载的同买家历史，不影响当前会话订单。 */
const historyOrderMatchesFilter = (order: ConversationOrder, filter: HistoryOrderFilter): boolean => {
  if (filter === 'all') return true;
  if (filter === 'refund') return historyOrderNeedsAttention(order);
  return order.status === filter;
};

/** attentionHistoryOrderRank 让仍在处理的退款排在已结束退款之前。 */
const attentionHistoryOrderRank = (order: ConversationOrder): number => order.status === 'refunding' ? 0 : 1;

/** historyOrderCreatedAtTimestamp 把订单时间转换为同优先级内的倒序比较值，非法或缺失时间排在最后。 */
const historyOrderCreatedAtTimestamp = (order: ConversationOrder): number => {
  // timestamp 是浏览器解析后的毫秒时间戳，非有限值不会参与靠前排序。
  const timestamp = Date.parse(order.created_at || '');
  return Number.isFinite(timestamp) ? timestamp : 0;
};

/** statusPresentation 把归一化订单状态转换为卡片文案和语义颜色。 */
const statusPresentation = (status: ConversationOrderStatus): { /** label 是状态中文名。 */ label: string; /** className 是状态徽标颜色。 */ className: string } => {
  switch (status) {
  case 'processing': return { label: '待付款', className: 'bg-amber-50 text-amber-700' };
  case 'pending_ship': return { label: '待发货', className: 'bg-orange-50 text-orange-700' };
  case 'shipped': return { label: '已发货', className: 'bg-sky-50 text-sky-700' };
  case 'received': return { label: '已收货', className: 'bg-cyan-50 text-cyan-700' };
  case 'completed': return { label: '已完成', className: 'bg-emerald-50 text-emerald-700' };
  case 'cancelled': return { label: '已取消', className: 'bg-slate-100 text-slate-600' };
  case 'refunding': return { label: '退款中', className: 'bg-rose-50 text-rose-700' };
  case 'refunded': return { label: '已退款', className: 'bg-fuchsia-50 text-fuchsia-700' };
  default: return { label: '未知', className: 'bg-slate-100 text-slate-500' };
  }
};

/** ConversationOrderCard 用稳定摘要展示一笔当前或历史订单，不再暴露内部匹配依据。 */
const ConversationOrderCard: React.FC<ConversationOrderCardProps> = ({ order, current, attention, copied, onCopy }) => {
  // status 是当前订单状态的文案和语义颜色。
  const status = statusPresentation(order.status);
  // createdAt 是订单创建时间的本地化展示文本。
  const createdAt = order.created_at ? formatLocalDateTime(order.created_at) : '时间未知';
  // amount 是移除重复货币前缀后的金额文本。
  const amount = String(order.amount || '0.00').replace(/^[¥￥]\s*/, '');
  // amountValue 是元单位实付金额；非正数时右栏明确表示正在同步，不误导为零元订单。
  const amountValue = Number.parseFloat(amount.replace(/[^0-9.-]/g, ''));
  // relationshipLabel 描述订单与当前会话的客观关系，不给买家添加主观风险标签。
  const relationshipLabel = current ? '当前关联' : attention === 'active' ? '售后处理中' : attention === 'resolved' ? '历史售后' : '历史订单';
  // relationshipClassName 为当前订单、进行中售后和历史退款提供不同强度的语义颜色。
  const relationshipClassName = current ? 'bg-sky-100 text-sky-800' : attention === 'active' ? 'bg-rose-100 text-rose-800' : attention === 'resolved' ? 'bg-fuchsia-50 text-fuchsia-700' : 'bg-slate-100 text-slate-600';
  // cardClassName 使用边框区分关系，已结束退款不会采用正在处理时的强烈红色表面。
  const cardClassName = current ? 'border-sky-200 ring-1 ring-sky-100' : attention === 'active' ? 'border-rose-300 ring-1 ring-rose-100' : attention === 'resolved' ? 'border-fuchsia-200 ring-1 ring-fuchsia-50' : 'border-slate-200 hover:border-sky-200';
  // footerClassName 延续卡片语义色，并保持金额与商品主体为中性白底。
  const footerClassName = current ? 'border-sky-100 bg-sky-50/60' : attention === 'active' ? 'border-rose-100 bg-rose-50/60' : attention === 'resolved' ? 'border-fuchsia-100 bg-fuchsia-50/40' : 'border-slate-100 bg-slate-50/60';
  // contextLabel 说明警示对应订单事件而非买家属性。
  const contextLabel = current ? '当前聊天订单' : attention === 'active' ? '退款／售后处理中' : attention === 'resolved' ? '历史退款记录' : '同买家历史';

  return <article data-association={current ? 'current_chat' : 'same_buyer'} data-attention={attention} className={`overflow-hidden rounded-2xl border bg-white shadow-sm transition hover:shadow-md ${cardClassName}`}>
    <div className="px-4 pb-3 pt-4">
      <div className="flex items-center justify-between gap-2"><span className={`rounded-full px-2.5 py-1 text-[11px] font-black ${relationshipClassName}`}>{relationshipLabel}</span><span className={`rounded-full px-2.5 py-1 text-[11px] font-black ${status.className}`}>{status.label}</span></div>
      <div className="mt-3 flex gap-3">
        {order.item_image ? <img src={order.item_image} alt="" className="h-16 w-16 shrink-0 rounded-xl border border-slate-100 bg-slate-50 object-cover" /> : <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-xl border border-slate-100 bg-slate-50"><PackageSearch className="h-6 w-6 text-slate-300" aria-hidden="true" /></div>}
        <div className="min-w-0 flex-1"><h3 className="line-clamp-2 text-sm font-black leading-5 text-slate-900">{order.item_title || order.item_id || '未知商品'}</h3><div className="mt-2 flex items-end justify-between gap-3"><span className="text-xs font-medium text-slate-500">数量 ×{order.quantity || 1}</span>{Number.isFinite(amountValue) && amountValue > 0 ? <strong className="text-lg font-black tabular-nums text-slate-950">¥{amount}</strong> : <span className="text-xs font-black text-amber-600">金额同步中</span>}</div></div>
      </div>
    </div>
    <footer className={`border-t px-4 py-2.5 ${footerClassName}`}>
      <div className="flex items-center justify-between gap-3"><span className="text-xs font-medium text-slate-500">{createdAt}</span><span className={`text-[11px] font-semibold ${attention ? 'text-fuchsia-600' : 'text-slate-400'}`}>{contextLabel}</span></div>
      <div className="mt-2 flex items-center gap-2 border-t border-slate-200/70 pt-2 text-[11px] text-slate-400"><span className="min-w-0 flex-1 truncate font-mono tabular-nums">订单号 {order.order_id}</span><button type="button" aria-label={`复制订单号 ${order.order_id}`} onClick={/* copyCurrentOrder 复制当前卡片订单号。 */ () => onCopy(order.order_id)} className="flex h-7 shrink-0 items-center gap-1 rounded-lg px-2 font-bold text-slate-500 hover:bg-white hover:text-sky-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300">{copied ? <><Check className="h-3.5 w-3.5" aria-hidden="true" />已复制</> : <><Copy className="h-3.5 w-3.5" aria-hidden="true" />复制</>}</button></div>
    </footer>
  </article>;
};

/** ConversationOrderPreviewPanel 展示当前会话订单和同买家本地历史订单，并适配宽屏第三栏和中窄屏抽屉。 */
export const ConversationOrderPreviewPanel: React.FC<ConversationOrderPreviewPanelProps> = ({ open, onClose, accountID, chatID, buyerName, buyerID, buyerAvatarURL, revision, enrichmentOrderID }) => {
  // orderState 是订单上下文 Hook 返回的摘要、分页和请求状态。
  const orderState = useConversationOrders({ accountID, chatID, buyerID, revision, orderID: enrichmentOrderID });
  // copiedIdentifier 保存最近复制成功的订单号或买家 ID，用于短暂反馈。
  const [copiedIdentifier, setCopiedIdentifier] = React.useState('');
  // historyFilter 保存历史订单栏当前人工查看范围；首次进入始终展示全部。
  const [historyFilter, setHistoryFilter] = React.useState<HistoryOrderFilter>('all');
  // currentOrders 保存当前页中由服务端精确关联到当前 chat_id 的订单。
  const currentOrders = orderState.orders.filter(/* currentOrderFilter 只保留当前会话订单。 */ order => order.association === 'current_chat');
  // historyOrders 保存当前页中同账号、同买家但不属于当前 chat_id 的历史订单。
  const historyOrders = orderState.orders.filter(/* historyOrderFilter 只保留同买家历史订单。 */ order => order.association === 'same_buyer');
  // filteredHistoryOrders 只筛选历史订单，切换按钮不会隐藏或改写当前会话订单。
  const filteredHistoryOrders = historyOrders.filter(/* selectedHistoryOrderFilter 应用当前人工选择的状态范围。 */ order => historyOrderMatchesFilter(order, historyFilter));
  // attentionHistoryOrders 保存明确处于退款中或已退款的历史订单，并按处理状态和时间倒序排列。
  const attentionHistoryOrders = filteredHistoryOrders.filter(historyOrderNeedsAttention).sort(
    // attentionOrderComparator 先比较处理紧急度，再比较同组下单时间。
    (leftOrder, rightOrder) => attentionHistoryOrderRank(leftOrder) - attentionHistoryOrderRank(rightOrder)
      || historyOrderCreatedAtTimestamp(rightOrder) - historyOrderCreatedAtTimestamp(leftOrder),
  );
  // otherHistoryOrders 保留全部非退款历史订单，继续沿用服务端业务排序。
  const otherHistoryOrders = filteredHistoryOrders.filter(/* otherHistoryOrderFilter 排除已经进入警示组的退款订单。 */ order => !historyOrderNeedsAttention(order));
  // historyOrderTotal 从完整摘要扣除当前会话订单，避免把两种关系混成一个指标。
  const historyOrderTotal = Math.max(orderState.summary.total - orderState.summary.current_chat, 0);
  // copyIdentifier 把订单号或买家 ID 复制到本机剪贴板，并显示短暂反馈。
  const copyIdentifier = React.useCallback(/* copyIdentifierAction 响应标识复制按钮。 */ async (identifier: string): Promise<void> => {
    await navigator.clipboard.writeText(identifier);
    setCopiedIdentifier(identifier);
    window.setTimeout(/* clearCopyFeedback 清除当前复制反馈。 */ () => setCopiedIdentifier(/* currentIdentifier 只清除仍属于本次复制的反馈。 */ currentIdentifier => currentIdentifier === identifier ? '' : currentIdentifier), 1500);
  }, []);

  return <>
    {open && <button type="button" aria-label="关闭订单上下文遮罩" onClick={onClose} className="fixed inset-0 z-40 bg-slate-950/30 backdrop-blur-[1px] 2xl:hidden" />}
    <aside aria-label="订单上下文" className={`${open ? 'flex' : 'hidden'} fixed bottom-4 right-4 top-4 z-50 w-[min(92vw,390px)] flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-2xl 2xl:static 2xl:z-auto 2xl:flex 2xl:w-auto 2xl:rounded-none 2xl:border-y-0 2xl:border-r-0 2xl:shadow-none`}>
      <header className="shrink-0 border-b border-slate-200 bg-white px-5 pb-4 pt-5">
        <div className="text-[11px] font-black uppercase tracking-[0.16em] text-sky-600">订单上下文</div>
        <div className="mt-3 flex items-center gap-3">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-sky-50 text-sky-600 ring-1 ring-sky-100">{buyerAvatarURL ? <img src={buyerAvatarURL} alt="" className="h-full w-full object-cover" /> : <ShoppingBag className="h-5 w-5" aria-hidden="true" />}</div>
          <div className="min-w-0 flex-1"><div className="truncate text-base font-black text-slate-950">{buyerName || buyerID}</div><button type="button" onClick={/* copyBuyerID 复制当前买家平台标识。 */ () => void copyIdentifier(buyerID)} className="mt-0.5 max-w-full truncate text-left text-xs font-medium text-slate-500 hover:text-sky-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300">{copiedIdentifier === buyerID ? '买家 ID 已复制' : `买家 ID · ${buyerID}`}</button></div>
          <button type="button" aria-label="刷新订单上下文" onClick={orderState.reload} disabled={orderState.loading} className="flex h-9 w-9 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-100 hover:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 disabled:opacity-40"><RefreshCw className={`h-4 w-4 ${orderState.loading ? 'animate-spin' : ''}`} aria-hidden="true" /></button>
          <button type="button" aria-label="关闭订单上下文" onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-100 hover:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 2xl:hidden"><X className="h-4 w-4" aria-hidden="true" /></button>
        </div>
        <div className="mt-4 grid grid-cols-3 divide-x divide-slate-200 rounded-xl bg-slate-50 px-2 py-3 text-center"><div><strong className="block text-lg font-black text-slate-950">{historyOrderTotal}</strong><span className="text-[11px] font-medium text-slate-500">同买家历史</span></div><div><strong className="block text-lg font-black text-slate-950">{orderState.summary.current_chat}</strong><span className="text-[11px] font-medium text-slate-500">当前会话</span></div><div><strong className="block text-lg font-black text-slate-950">{orderState.summary.completed}</strong><span className="text-[11px] font-medium text-slate-500">已完成</span></div></div>
        {orderState.truncated && <div className="mt-3 rounded-xl bg-amber-50 px-3 py-2 text-[11px] leading-4 text-amber-700">同买家订单超过 500 笔，当前仅展示最近同步记录。</div>}
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto bg-slate-50/60 p-3">
        {orderState.loading && orderState.orders.length === 0 ? <div className="flex h-full items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-sky-500" aria-label="正在加载订单上下文" /></div> : <div className="space-y-4">
          {orderState.error && <div role="alert" className="rounded-2xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700"><div>{orderState.error}</div><button type="button" onClick={orderState.reload} className="mt-2 font-black underline">重新加载</button></div>}

          {!orderState.error && <section aria-labelledby="current-conversation-orders-heading">
            <div className="mb-2 flex items-center justify-between gap-3 px-1"><h2 id="current-conversation-orders-heading" className="text-sm font-black text-slate-950">当前会话订单</h2><span className="text-xs font-bold text-slate-400">{orderState.summary.current_chat} 笔</span></div>
            {currentOrders.length > 0 ? <div className="space-y-3">{currentOrders.map(/* currentOrderCard 渲染当前会话精确关联订单。 */ order => <ConversationOrderCard key={order.order_id} order={order} current copied={copiedIdentifier === order.order_id} onCopy={/* copyCurrentConversationOrder 复制当前会话订单号。 */ orderID => void copyIdentifier(orderID)} />)}</div> : <div className="rounded-2xl border border-dashed border-sky-200 bg-sky-50/70 px-4 py-4"><div className="text-sm font-black text-slate-700">暂未识别本次聊天对应的订单</div><div className="mt-1 text-xs leading-5 text-slate-500">下方仍会展示同账号、同买家的本地历史订单，供人工判断。</div></div>}
          </section>}

          {!orderState.error && <section aria-labelledby="same-buyer-orders-heading">
            <div className="mb-2 flex items-end justify-between gap-3 px-1"><div><h2 id="same-buyer-orders-heading" className="text-sm font-black text-slate-950">同买家历史订单</h2><p className="mt-0.5 text-[11px] text-slate-400">仅展示本地已同步记录</p></div><span className="text-xs font-bold text-slate-400">共 {historyOrderTotal} 笔</span></div>
            <nav aria-label="同买家历史订单筛选" className="mb-3 grid grid-cols-4 gap-1 rounded-xl bg-slate-100 p-1">{historyOrderFilterOptions.map(/* historyFilterButton 渲染紧凑且不横向滚动的状态切换按钮。 */ option => <button key={option.key} type="button" aria-pressed={historyFilter === option.key} onClick={/* selectHistoryOrderFilter 切换历史订单范围，当前会话订单保持不变。 */ () => setHistoryFilter(option.key)} className={`min-w-0 rounded-lg px-1.5 py-2 text-[11px] font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 ${historyFilter === option.key ? 'bg-white text-sky-700 shadow-sm' : 'text-slate-500 hover:bg-white/70 hover:text-slate-700'}`}>{option.label}</button>)}</nav>
            {filteredHistoryOrders.length > 0 ? <div>
              {attentionHistoryOrders.length > 0 && <section aria-label="需注意的退款售后记录">
                <div className="mb-2 rounded-xl bg-rose-50 px-3 py-2.5 text-rose-800"><div className="flex items-center justify-between gap-3"><div className="flex items-center gap-2 text-xs font-black"><AlertTriangle className="h-4 w-4" aria-hidden="true" />需注意</div><span className="text-[11px] font-bold">{attentionHistoryOrders.length} 笔</span></div><p className="mt-1 text-[11px] leading-4 text-rose-700">该买家有退款／售后记录，请结合当前咨询核对相关订单。</p></div>
                <div className="space-y-3">{attentionHistoryOrders.map(/* attentionHistoryOrderCard 优先渲染明确退款历史。 */ order => <ConversationOrderCard key={order.order_id} order={order} current={false} attention={order.status === 'refunding' ? 'active' : 'resolved'} copied={copiedIdentifier === order.order_id} onCopy={/* copyAttentionHistoryOrder 复制退款／售后订单号。 */ orderID => void copyIdentifier(orderID)} />)}</div>
              </section>}
              {otherHistoryOrders.length > 0 && <section aria-label={attentionHistoryOrders.length > 0 ? '其他历史订单' : '历史订单列表'} className={attentionHistoryOrders.length > 0 ? 'mt-4' : ''}>
                {attentionHistoryOrders.length > 0 && <div className="mb-2 flex items-center justify-between gap-3 px-1"><h3 className="text-xs font-black text-slate-700">其他历史订单</h3><span className="text-[11px] font-bold text-slate-400">{otherHistoryOrders.length} 笔</span></div>}
                <div className="space-y-3">{otherHistoryOrders.map(/* otherHistoryOrderCard 渲染非退款的同买家历史订单。 */ order => <ConversationOrderCard key={order.order_id} order={order} current={false} copied={copiedIdentifier === order.order_id} onCopy={/* copyBuyerHistoryOrder 复制同买家历史订单号。 */ orderID => void copyIdentifier(orderID)} />)}</div>
              </section>}
            </div> : <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-slate-200 bg-white px-5 py-10 text-center"><PackageSearch className="h-8 w-8 text-slate-300" aria-hidden="true" /><div className="mt-3 text-sm font-black text-slate-700">{historyFilter === 'all' ? '暂无同买家历史订单' : '暂无该状态历史订单'}</div><div className="mt-1 text-xs text-slate-400">{historyFilter === 'all' ? '当前账号下没有匹配的本地同步记录' : '可以切换“全部”查看其他历史订单'}</div></div>}
          </section>}

          {orderState.totalPages > 1 && <nav aria-label="订单上下文分页" className="flex items-center justify-center gap-3 py-2"><button type="button" aria-label="上一页订单上下文" disabled={orderState.page <= 1 || orderState.loading} onClick={/* previousOrderPage 读取上一页。 */ () => orderState.setPage(orderState.page - 1)} className="flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 disabled:opacity-30"><ChevronLeft className="h-4 w-4" aria-hidden="true" /></button><span className="text-xs font-bold text-slate-500">{orderState.page} / {orderState.totalPages}</span><button type="button" aria-label="下一页订单上下文" disabled={orderState.page >= orderState.totalPages || orderState.loading} onClick={/* nextOrderPage 读取下一页。 */ () => orderState.setPage(orderState.page + 1)} className="flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 disabled:opacity-30"><ChevronRight className="h-4 w-4" aria-hidden="true" /></button></nav>}
        </div>}
      </div>
    </aside>
  </>;
};
