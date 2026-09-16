import { Flower2,ReceiptText } from 'lucide-react';
import React from 'react';
import type { ChatSystemCard } from '../api';
import { AdjustPriceModal } from './AdjustPriceModal';
import { ShipOrderModal } from './ShipOrderModal';
import { RefundDetailModal } from './RefundDetailModal';

// redFlowerReceiveBaseURL 是唯一允许打开的闲鱼官方收花页面，不接受平台载荷覆盖域名或路径。
const redFlowerReceiveBaseURL = 'https://h5.m.goofish.com/wow/moyu/moyu-project/temp-pages/pages/red-flower-play';
// redFlowerOrderPattern 只接受闲鱼当前数字订单号，异常标识不会生成外部链接。
const redFlowerOrderPattern = /^\d{10,30}$/;

// TradeSystemCardProps 描述交易卡片的结构化数据、时间和同会话收花结果。
export interface TradeSystemCardProps {
  // card 是后端从平台系统消息安全投影出的交易卡片。
  card: ChatSystemCard;
  // time 是聊天页按本地时区格式化后的消息时间。
  time: string;
  // received 表示同会话后续已经出现 red_flower_received 结果。
  received?: boolean;
  // accountID 是卡片所属卖家账号，用于真实改价归属请求。
  accountID?: string;
  // adjustPriceDisabledReason 是同订单后续终态给出的不可改价原因；非空时按钮保留但置灰。
  adjustPriceDisabledReason?: string;
  // sellerActionAllowed 表示当前卡片属于卖家视角；买家账号不显示改价或卖家关单入口。
  sellerActionAllowed?: boolean;
	// shipmentDisabledReason 是同订单后续发货、退款或取消终态给出的发货禁用原因。
	shipmentDisabledReason?: string;
}

/** redFlowerReceiveURL 使用固定官方地址和编码订单号构造收花页面 URL。 */
export const redFlowerReceiveURL = (orderID: string | undefined): string => {
  // normalizedOrderID 是去空白后的订单标识。
  const normalizedOrderID = String(orderID || '').trim();
  if (!redFlowerOrderPattern.test(normalizedOrderID)) return '';
  // target 是从固定白名单基址创建的官方页面地址。
  const target = new URL(redFlowerReceiveBaseURL);
  target.searchParams.set('kun', 'true');
  target.searchParams.set('opaque', 'false');
  target.searchParams.set('role', 'seller');
  target.searchParams.set('confirm', 'false');
  target.searchParams.set('orderId', normalizedOrderID);
  return target.toString();
};

// TradeSystemCard 按闲鱼交易卡片层级展示状态；收花动作只打开官方页面，由用户在页面内确认。
export const TradeSystemCard: React.FC<TradeSystemCardProps> = ({ card, time, received = false, accountID = '', adjustPriceDisabledReason = '', sellerActionAllowed = true, shipmentDisabledReason = '' }) => {
  // showAdjustPrice 表示平台卡片明确提供改价动作且当前事件是待付款。
  const showAdjustPrice = sellerActionAllowed && card.event === 'order_pending_payment' && card.action === 'adjust_price';
  // receiveURL 是受固定域名、路径和数字订单号约束的官方收花页面。
  const receiveURL = card.event === 'red_flower_sent' && card.action === 'receive_red_flower' ? redFlowerReceiveURL(card.order_id) : '';
  // showReceiveFlower 表示当前卡片具备可安全打开的官方收花入口。
  const showReceiveFlower = receiveURL !== '';
	// showShipment 表示平台明确给出卖家订单详情动作且当前事件是已付款待发货。
	const showShipment = sellerActionAllowed && card.event === 'order_paid' && card.action === 'ship_order';
	// showRefundDetail 表示退款申请卡片具备账号和订单归属，可执行官方只读详情查询。
	const showRefundDetail = sellerActionAllowed && card.event === 'refund_requested' && Boolean(accountID && card.order_id);
  // flowerCard 表示当前卡片属于小红花视觉语义。
  const flowerCard = card.event.startsWith('red_flower_');
  // opened 表示本轮页面生命周期内已经打开过官方收花页，但不代表平台操作成功。
  const [opened, setOpened] = React.useState(false);
  // adjustOpen 表示当前卡片的真实改价弹窗是否打开。
  const [adjustOpen, setAdjustOpen] = React.useState(false);
  // adjustedMessage 保存本轮页面生命周期内平台明确成功后的反馈。
  const [adjustedMessage, setAdjustedMessage] = React.useState('');
	// shipmentOpen 和 setShipmentOpen 控制当前付款卡片的无需寄件弹窗。
	const [shipmentOpen, setShipmentOpen] = React.useState(false);
	// shipmentMessage 和 setShipmentMessage 保存本轮平台明确发货后的反馈。
	const [shipmentMessage, setShipmentMessage] = React.useState('');
	// refundOpen 和 setRefundOpen 控制当前退款申请卡片的只读详情弹窗。
	const [refundOpen, setRefundOpen] = React.useState(false);
  // adjustDisabled 表示缺少归属、已经成功提交或同订单出现终态，任一条件都禁止再次打开弹窗。
  const adjustDisabled = !accountID || !card.order_id || adjustedMessage !== '' || adjustPriceDisabledReason !== '';
  // adjustStatusMessage 优先展示平台终态原因，再展示本轮成功结果和默认安全提示。
  const adjustStatusMessage = adjustPriceDisabledReason || adjustedMessage || '仅待付款订单可改价，提交前会再次确认';
  // adjustButtonTitle 解释当前按钮可用性，避免仅靠颜色传达付款后禁用状态。
  const adjustButtonTitle = adjustPriceDisabledReason || (!accountID || !card.order_id ? '缺少账号或订单关联' : '读取闲鱼当前价格');
  // adjustModalAllowed 只受账号／订单完整性和后续终态限制；本轮成功后仍保持弹窗显示结果页。
  const adjustModalAllowed = Boolean(accountID && card.order_id && !adjustPriceDisabledReason);
  // adjustDescriptionID 关联改价按钮和当前资格说明。
  const adjustDescriptionID = `adjust-price-action-${card.order_id || 'unknown'}`;
  // receiveDescriptionID 关联收花按钮和官方页面提示。
  const receiveDescriptionID = `receive-flower-action-${card.order_id || 'unknown'}`;
	// shipmentDisabled 表示缺少归属、已经成功提交或同订单出现后续终态。
	const shipmentDisabled = !accountID || !card.order_id || shipmentMessage !== '' || shipmentDisabledReason !== '';
	// shipmentStatusMessage 解释发货入口当前状态，避免仅靠按钮颜色表达。
	const shipmentStatusMessage = shipmentDisabledReason || shipmentMessage || '支持无需寄件、描述和最多 3 张图片凭证';
	// shipmentDescriptionID 关联发货按钮和资格说明。
	const shipmentDescriptionID = `shipment-action-${card.order_id || 'unknown'}`;
	// handleRefundCardOpen 只在退款申请卡片归属完整时打开只读详情。
	const handleRefundCardOpen = (): void => {
		if (showRefundDetail) setRefundOpen(true);
	};

  // handleReceiveFlower 在用户二次确认后打开固定闲鱼官方页面，不调用 ERP 平台接口。
  const handleReceiveFlower = React.useCallback(/* receiveFlowerAction 打开官方收花页。 */ () => {
    if (!receiveURL || received) return;
    if (!window.confirm('将打开闲鱼官方收花页面。请在官方页面核对奖励并确认收花，是否继续？')) return;
    window.open(receiveURL, '_blank', 'noopener,noreferrer');
    setOpened(true);
  }, [receiveURL, received]);

  return (
    <div className="flex w-full items-start justify-center gap-2.5 py-0.5" data-system-card-event={card.event}>
      <div className={`mt-1 flex h-8 w-8 shrink-0 items-center justify-center rounded-full ring-2 ring-white ${flowerCard ? 'bg-rose-100 text-rose-600' : 'bg-sky-100 text-sky-700'}`} aria-hidden="true">
        {flowerCard ? <Flower2 className="h-4 w-4" /> : <ReceiptText className="h-4 w-4" />}
      </div>
		<article className={`w-full max-w-xl overflow-hidden rounded-2xl border border-slate-200 bg-white px-4 py-3 shadow-sm sm:px-5 ${showRefundDetail ? 'cursor-pointer transition hover:border-sky-300 hover:shadow-md focus:outline-none focus:ring-4 focus:ring-sky-100' : ''}`} aria-label={`${flowerCard ? '小红花消息' : '交易状态'}：${card.title}`} onClick={handleRefundCardOpen} onKeyDown={/* refundCardKeyDown 支持键盘打开退款详情。 */ event => { if (showRefundDetail && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); handleRefundCardOpen(); } }} tabIndex={showRefundDetail ? 0 : undefined}>
        <header className="flex items-start gap-2.5">
          <h4 className="min-w-0 flex-1 text-sm font-black leading-5 text-slate-950 sm:text-base">{card.title}</h4>
          <time className="shrink-0 text-xs font-medium leading-5 text-slate-500">{time}</time>
        </header>
		{(card.description || showAdjustPrice || showReceiveFlower || showShipment || showRefundDetail) && <div className="mt-3 border-t border-slate-200 pt-3">
          {card.description && <p className="text-xs font-medium leading-5 text-slate-500 sm:text-sm">{card.description}</p>}
          {showAdjustPrice && <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
            <span id={adjustDescriptionID} className={`text-xs font-medium ${adjustPriceDisabledReason ? 'text-slate-500' : adjustedMessage ? 'text-emerald-600' : 'text-slate-400'}`}>{adjustStatusMessage}</span>
            <button type="button" onClick={/* openAdjustPrice 仅在当前仍满足本地资格时打开动态改价表单。 */ () => { if (!adjustDisabled) setAdjustOpen(true); }} disabled={adjustDisabled} aria-disabled={adjustDisabled} aria-describedby={adjustDescriptionID} title={adjustButtonTitle} className={`h-9 rounded-full px-5 text-sm font-bold ${adjustDisabled ? 'cursor-not-allowed bg-slate-200 text-slate-500' : 'bg-yellow-300 text-slate-900 hover:bg-yellow-400'}`}>
              {adjustedMessage ? '已修改价格' : '修改价格'}
            </button>
          </div>}
          {showReceiveFlower && <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
            <span id={receiveDescriptionID} className={`text-xs font-medium ${received ? 'text-emerald-600' : opened ? 'text-rose-500' : 'text-slate-400'}`}>
              {received ? '已检测到闲鱼收花结果' : opened ? '已打开闲鱼官方页面，请在页面内完成收花' : '将跳转闲鱼官方页面，ERP 不代替确认奖励'}
            </span>
            <button type="button" onClick={handleReceiveFlower} disabled={received} aria-disabled={received} aria-describedby={receiveDescriptionID}
              className={`h-9 rounded-full px-5 text-sm font-bold text-white ${received ? 'cursor-not-allowed bg-emerald-500 opacity-70' : 'bg-rose-500 hover:bg-rose-600'}`}>
              {received ? '已收花' : opened ? '重新打开收花页' : '立即收花'}
            </button>
          </div>}
			{showShipment && <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
				<span id={shipmentDescriptionID} className={`text-xs font-medium ${shipmentDisabledReason ? 'text-slate-500' : shipmentMessage ? 'text-emerald-600' : 'text-slate-400'}`}>{shipmentStatusMessage}</span>
				<button type="button" onClick={/* openShipment 只在当前本地资格完整时打开无需寄件弹窗。 */ () => { if (!shipmentDisabled) setShipmentOpen(true); }} disabled={shipmentDisabled} aria-disabled={shipmentDisabled} aria-describedby={shipmentDescriptionID} title={shipmentDisabledReason || (!accountID || !card.order_id ? '缺少卖家账号或订单关联' : '填写发货描述和图片凭证')} className={`h-9 rounded-full px-5 text-sm font-bold ${shipmentDisabled ? 'cursor-not-allowed bg-slate-200 text-slate-500' : 'bg-yellow-300 text-slate-900 hover:bg-yellow-400'}`}>
					{shipmentMessage ? '已发货' : '立即发货'}
				</button>
			</div>}
			{showRefundDetail && <div className="mt-3 flex items-center justify-end gap-2 text-xs font-bold text-sky-600"><ReceiptText className="h-4 w-4" />点击查看退款原因与金额</div>}
        </div>}
      </article>
      {showAdjustPrice && <AdjustPriceModal accountID={accountID} orderID={card.order_id || ''} open={adjustOpen && adjustModalAllowed} onClose={/* closeAdjustPrice 关闭当前卡片弹窗。 */ () => setAdjustOpen(false)} onSuccess={/* adjustPriceSucceeded 保存平台明确成功提示但保持结果页可见。 */ message => setAdjustedMessage(message)} />}
		{showShipment && <ShipOrderModal accountID={accountID} orderID={card.order_id || ''} open={shipmentOpen && !shipmentDisabledReason} onClose={/* closeShipment 关闭当前付款卡片发货弹窗。 */ () => setShipmentOpen(false)} onSuccess={/* shipmentSucceeded 保存平台明确发货反馈并保持成功页可见。 */ message => setShipmentMessage(message)} />}
		{showRefundDetail && <RefundDetailModal accountID={accountID} orderID={card.order_id || ''} open={refundOpen} onClose={/* closeRefundDetail 关闭当前退款详情弹窗。 */ () => setRefundOpen(false)} />}
    </div>
  );
};
