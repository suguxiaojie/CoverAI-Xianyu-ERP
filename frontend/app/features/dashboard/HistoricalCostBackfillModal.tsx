import { AlertCircle,CheckCircle2,Database,Loader2,X } from 'lucide-react';
import React,{ useEffect,useRef,useState } from 'react';
import { createPortal } from 'react-dom';
import { cancelHistoricalCostBackfillJob,confirmManualCostCandidate,getHistoricalCostBackfillJob,getManualCostCandidates,startHistoricalCostBackfill } from './api';
import type { HistoricalCostBackfillJob,ManualCostCandidate } from './api';

/** 历史成本补全弹窗输入参数。 */
export type HistoricalCostBackfillModalProps = {
	/** accountID 是成本候选和扫描任务的账号范围，空值表示全部账号。 */
	accountID: string;
	/** accountLabel 是弹窗标题下展示的当前统计范围名称。 */
	accountLabel: string;
	/** startDate 是当前 Dashboard 本地日期起点。 */
	startDate: string;
	/** endDate 是当前 Dashboard 本地日期终点。 */
	endDate: string;
  /** onClose 关闭当前弹窗。 */
  onClose: () => void;
  /** onCompleted 在任务成功后刷新 Dashboard 数据。 */
  onCompleted: () => void;
};

/** 等待下一次任务轮询，并允许组件卸载时立即取消等待。 */
const waitForPoll = (signal: AbortSignal): Promise<void> => new Promise(
  // resolve 和 reject 分别结束正常等待或传播取消信号。
  (resolve, reject) => {
    // abort 取消计时器并拒绝等待；正常完成时会移除监听，避免长任务累积回调。
    const abort = () => {
      window.clearTimeout(timer);
      reject(new DOMException('请求已取消', 'AbortError'));
    };
    // timer 是五百毫秒轮询间隔计时器，结束时清理取消监听。
    const timer = window.setTimeout(/* 当前回调结束本轮等待并清理取消监听。 */ () => {
      signal.removeEventListener('abort', abort);
      resolve();
    }, 500);
    signal.addEventListener('abort', abort, { once: true });
  },
);

/** 展示历史成本补全确认、实时进度和停止入口。 */
export const HistoricalCostBackfillModal: React.FC<HistoricalCostBackfillModalProps> = ({ accountID, accountLabel, startDate, endDate, onClose, onCompleted }) => {
  // job 保存后台任务当前快照；null 表示尚未开始。
  const [job, setJob] = useState<HistoricalCostBackfillJob | null>(null);
  // error 保存创建、轮询或停止任务的用户可见错误。
  const [error, setError] = useState('');
  // starting 表示创建任务请求正在进行。
  const [starting, setStarting] = useState(false);
  // candidates 保存尚未人工确认的历史金额分组。
  const [candidates, setCandidates] = useState<ManualCostCandidate[]>([]);
  // candidatesLoading 表示人工候选首次读取尚未完成，期间不得显示扫描按钮。
  const [candidatesLoading, setCandidatesLoading] = useState(true);
  // selectedSKUs 保存每个金额分组当前人工选择的 SKU。
  const [selectedSKUs, setSelectedSKUs] = useState<Record<string,string>>({});
  // customCosts 保存每个金额分组由用户填写的历史单件成本元值文本。
  const [customCosts, setCustomCosts] = useState<Record<string,string>>({});
  // confirmingKey 是当前正在提交确认的金额分组键。
  const [confirmingKey, setConfirmingKey] = useState('');
  // controllerRef 保存当前轮询生命周期控制器，卸载后拒绝晚到状态写入。
  const controllerRef = useRef<AbortController | null>(null);

	useEffect(/* 当前副作用加载本地人工候选并在卸载时取消请求；不会写入成本快照。 */ () => {
		// controller 控制人工候选读取生命周期。
		const controller = new AbortController();
		setCandidatesLoading(true);
		setCandidates([]);
		setSelectedSKUs({});
		setCustomCosts({});
	getManualCostCandidates(accountID, { start_date: startDate, end_date: endDate }, { signal: controller.signal }).then(/* loaded 是服务端返回的人工候选分组。 */ loaded => {
      setCandidates(loaded);
      // defaults 保存每组基于价格距离的默认建议，仍需用户点击确认。
      const defaults: Record<string,string> = {};
      // costDefaults 保存建议 SKU 当前成本对应的可编辑历史成本初始值。
      const costDefaults: Record<string,string> = {};
      loaded.forEach(/* candidate 是当前建立默认 SKU 和历史成本的金额候选。 */ candidate => {
        // key 是当前候选的商品金额组合键。
        const key = `${candidate.account_id}:${candidate.item_id}:${candidate.amount_cents}:${candidate.quantity}`;
        // suggestedID 是价格距离建议或第一个可选 SKU。
        const suggestedID = candidate.suggested_sku_id || candidate.skus[0]?.sku_id || '';
        defaults[key] = suggestedID;
        // suggestedSKU 是默认选择对应的 SKU 成本参考。
        const suggestedSKU = candidate.skus.find(/* sku 是当前寻找默认成本的候选 SKU。 */ sku => sku.sku_id === suggestedID);
        costDefaults[key] = candidate.manual_only ? '' : candidate.suggested_unit_cost_cents >= 0 ? (candidate.suggested_unit_cost_cents / 100).toFixed(2) : suggestedSKU ? (suggestedSKU.cost_cents / 100).toFixed(2) : '';
      });
      setSelectedSKUs(defaults);
      setCustomCosts(costDefaults);
    }).catch(/* loadError 是候选读取失败原因；取消请求不展示错误。 */ loadError => {
      if (!controller.signal.aborted) setError(loadError instanceof Error ? loadError.message : '读取人工候选失败');
    }).finally(/* 当前回调结束候选加载状态，卸载后的状态写入由 React 安全忽略。 */ () => setCandidatesLoading(false));
    return /* 当前清理同时取消候选请求和任务轮询。 */ () => { controller.abort(); controllerRef.current?.abort(); };
	}, [accountID, endDate, startDate]);

  /** 用户确认金额分组对应 SKU 后生成不可变成本快照并移除已处理分组。 */
  const handleConfirmCandidate = async (candidate: ManualCostCandidate) => {
    // key 是商品和历史成交金额组成的稳定分组键。
    const key = `${candidate.account_id}:${candidate.item_id}:${candidate.amount_cents}:${candidate.quantity}`;
    // skuID 是用户当前明确选择的 SKU。
    const skuID = selectedSKUs[key];
    // customCostText 是用户填写的历史单件成本元值。
    const customCostText = customCosts[key]?.trim() || '';
    if ((!candidate.manual_only && !skuID) || !/^\d+(?:\.\d{1,2})?$/.test(customCostText)) {
      setError('历史单件成本必须是最多两位小数的非负金额');
      return;
    }
    // customUnitCostCents 是提交给服务端的历史单件成本分值。
    const customUnitCostCents = Math.round(Number(customCostText) * 100);
    setConfirmingKey(key);
    setError('');
    try {
		await confirmManualCostCandidate({ account_id: candidate.account_id, item_id: candidate.item_id, amount_cents: candidate.amount_cents, quantity: candidate.quantity, start_date: startDate, end_date: endDate, timezone_offset_minutes: -new Date().getTimezoneOffset(), sku_id: skuID || '', custom_unit_cost_cents: customUnitCostCents });
      setCandidates(/* current 是确认前的候选集合。 */ current => current.filter(/* item 是当前保留的未确认分组。 */ item => `${item.account_id}:${item.item_id}:${item.amount_cents}:${item.quantity}` !== key));
      onCompleted();
    } catch (/* confirmError 是人工确认写入失败原因。 */ confirmError) {
      setError(confirmError instanceof Error ? confirmError.message : '历史成本确认失败');
    } finally {
      setConfirmingKey('');
    }
  };

  /** 用户确认当前成本适用于精确匹配的历史订单后创建并轮询任务。 */
  const handleStart = async () => {
    setStarting(true);
    setError('');
    // controller 隔离本轮任务轮询，避免弹窗关闭后的响应写入状态。
    const controller = new AbortController();
    controllerRef.current = controller;
    try {
      // started 是后端创建的持久化历史补全任务。
		const started = await startHistoricalCostBackfill(accountID, { signal: controller.signal });
      setJob(started);
      // current 是每次轮询得到的最新任务状态。
      let current = started;
      // terminalStatuses 是不会再产生后续进度的持久任务终态。
      const terminalStatuses = new Set(['succeeded', 'failed', 'cancelled']);
      while (!controller.signal.aborted && !terminalStatuses.has(current.status)) {
        await waitForPoll(controller.signal);
        current = await getHistoricalCostBackfillJob(started.job_id, { signal: controller.signal });
        setJob(current);
      }
      if (current.status === 'succeeded') onCompleted();
      if (current.status === 'failed') setError(current.error_message || '历史成本补全失败');
    } catch (/* requestError 是创建或轮询历史补全任务的失败原因。 */ requestError) {
      if (!controller.signal.aborted) setError(requestError instanceof Error ? requestError.message : '历史成本补全失败');
    } finally {
      if (!controller.signal.aborted) setStarting(false);
    }
  };

  /** 用户主动停止任务；已精确匹配并锁定的快照不会回滚。 */
  const handleCancel = async () => {
    if (!job?.job_id) return;
    setError('');
    try {
      await cancelHistoricalCostBackfillJob(job.job_id);
      controllerRef.current?.abort();
      setJob({ ...job, status: 'canceled' });
      setStarting(false);
      onCompleted();
    } catch (/* cancelError 是停止后台任务失败的原因。 */ cancelError) {
      setError(cancelError instanceof Error ? cancelError.message : '停止任务失败');
    }
  };

  // progress 是任务当前进度；准备阶段尚无进度时使用安全零值。
  const progress = job?.progress || { stage: 'preparing', message: '准备读取历史订单详情', processed: 0, total: 0, succeeded: 0, failed: 0, percent: 0 };
  // running 表示任务已创建且尚未进入终态。
  const running = job !== null && !['succeeded', 'failed', 'cancelled'].includes(job.status);
  // succeeded 表示精确历史详情扫描已经完成。
  const succeeded = job?.status === 'succeeded';

  // modal 是脱离 Dashboard 动画定位上下文、始终相对浏览器视口居中的弹窗内容。
  const modal = (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-transparent p-4" role="dialog" aria-modal="true" aria-labelledby="historical-cost-title">
      <div className="w-full max-w-xl rounded-3xl border border-gray-200 bg-white shadow-xl">
        <div className="flex items-start justify-between border-b border-gray-100 px-6 py-5">
          <div>
            <h3 id="historical-cost-title" className="text-xl font-extrabold text-gray-900">补全历史成本</h3>
			<p className="mt-1 text-sm text-gray-500">当前范围：{accountLabel}，{startDate} 至 {endDate}。优先恢复规格，无 SKU 时由用户确认历史成本。</p>
          </div>
          <button type="button" className="rounded-xl p-2 text-gray-400 hover:bg-gray-100" onClick={onClose} aria-label="关闭"><X className="h-5 w-5" /></button>
        </div>
        <div className="space-y-5 px-6 py-6">
          {candidatesLoading && !job ? <div className="flex items-center justify-center gap-2 py-10 text-sm text-gray-500"><Loader2 className="h-5 w-5 animate-spin text-blue-500" />正在读取未覆盖订单</div> : !job && candidates.length === 0 ? (
            <>
              <div className="rounded-2xl bg-blue-50 p-4 text-sm leading-6 text-blue-800">
                <div className="mb-2 flex items-center gap-2 font-bold"><Database className="h-5 w-5" />本次只读取必要的多规格历史订单详情</div>
                <p>只处理规格为空、尚未覆盖成本且当前存在多个已配置成本 SKU 的有效订单。不会运行订单列表同步，也不会修改闲鱼订单或商品。</p>
              </div>
              <div className="rounded-2xl border border-amber-100 bg-amber-50 p-4 text-sm leading-6 text-amber-800">
                <div className="mb-1 flex items-center gap-2 font-bold"><AlertCircle className="h-5 w-5" />成本口径确认</div>
                <p>开始后，精确匹配成功的历史订单将使用当前 SKU 成本生成不可变快照。读取不到明确规格的订单继续保持未知成本，系统不会按成交金额猜测。</p>
              </div>
            </>
          ) : job ? (
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                {succeeded ? <CheckCircle2 className="h-7 w-7 text-green-500" /> : running ? <Loader2 className="h-7 w-7 animate-spin text-blue-500" /> : <AlertCircle className="h-7 w-7 text-amber-500" />}
                <div>
                  <div className="font-bold text-gray-900">{succeeded ? '历史成本补全完成' : running ? '正在读取历史订单详情' : '任务已停止'}</div>
                  <div className="text-sm text-gray-500">{progress.message}</div>
                </div>
              </div>
              <div className="h-3 overflow-hidden rounded-full bg-gray-100"><div className="h-full rounded-full bg-blue-500 transition-all" style={{ width: `${Math.max(0, Math.min(100, progress.percent))}%` }} /></div>
              <div className="grid grid-cols-3 gap-3 text-center text-sm">
                <div className="rounded-xl bg-gray-50 p-3"><div className="text-lg font-extrabold">{progress.processed} / {progress.total}</div><div className="text-gray-400">已处理</div></div>
                <div className="rounded-xl bg-green-50 p-3"><div className="text-lg font-extrabold text-green-600">{progress.succeeded}</div><div className="text-gray-400">读取成功</div></div>
                <div className="rounded-xl bg-red-50 p-3"><div className="text-lg font-extrabold text-red-600">{progress.failed}</div><div className="text-gray-400">读取失败</div></div>
              </div>
            </div>
          ) : null}
          {error && <div className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}
          {candidates.length > 0 && (
            <div className="space-y-3 border-t border-gray-100 pt-5">
              <div><h4 className="font-extrabold text-gray-900">人工历史成本</h4><p className="mt-1 text-sm text-gray-500">已下架或缺少 SKU 的订单可直接填写历史单件成本；不按售价猜测，1.6% 手续费仍由统计单独扣除。</p></div>
              <div className="max-h-72 space-y-3 overflow-y-auto pr-1">
                {candidates.map(/* candidate 是当前渲染的商品金额候选分组。 */ candidate => {
                  // key 是当前候选的稳定分组键。
                  const key = `${candidate.account_id}:${candidate.item_id}:${candidate.amount_cents}:${candidate.quantity}`;
                  // selectedSKU 是当前人工选择的 SKU 标识。
                  const selectedSKU = selectedSKUs[key] || '';
                  // customCostText 是当前分组可编辑的历史单件成本。
                  const customCostText = customCosts[key] || '';
                  // customCost 是可用于实时预览的历史单件成本元值。
                  const customCost = Number(customCostText || 0);
                  // groupRevenue 是当前分组全部订单成交额。
                  const groupRevenue = candidate.amount_cents / 100 * candidate.order_count;
                  // groupFee 是按单笔四舍五入后累计的 1.6% 平台手续费。
                  const groupFee = Math.round(candidate.amount_cents * 0.016) / 100 * candidate.order_count;
                  // groupCost 是按当前自定义单件成本估算的分组商品成本。
                  const groupCost = customCost * candidate.quantity * candidate.order_count;
                  // groupProfit 是扣除商品成本和平台手续费后的分组经营利润。
                  const groupProfit = groupRevenue - groupCost - groupFee;
                  return <div key={key} className="rounded-2xl border border-gray-200 p-4">
                    <div className="flex flex-wrap items-start justify-between gap-2"><div><div className="font-bold text-gray-900">成交 ¥{(candidate.amount_cents / 100).toFixed(2)} · 数量 {candidate.quantity} · {candidate.order_count} 笔</div><div className="text-xs text-gray-400">{candidate.item_title || candidate.item_id || '未知商品'} · 账号 {candidate.account_id}</div></div><span className={`rounded-full px-2.5 py-1 text-xs font-bold ${candidate.confidence === 'exact' || candidate.confidence === 'historical' ? 'bg-green-50 text-green-700' : 'bg-amber-50 text-amber-700'}`}>{candidate.manual_only ? '需填写历史成本' : candidate.confidence === 'historical' ? '沿用历史确认' : candidate.confidence === 'exact' ? '标准售价一致' : '改价候选，需确认'}</span></div>
                    <div className="mt-3 grid gap-2 sm:grid-cols-[minmax(0,1fr)_150px_auto]">
                      {candidate.manual_only ? <div className="rounded-xl bg-gray-50 px-3 py-2 text-sm text-gray-500">无可用 SKU，仅保存订单级历史成本</div> : <select className="min-w-0 rounded-xl border border-gray-200 px-3 py-2 text-sm" value={selectedSKU} onChange={/* event 是用户选择 SKU 的表单事件。 */ event => {
                        // nextSKU 是用户刚选择的 SKU 完整候选。
                        const nextSKU = candidate.skus.find(/* sku 是当前匹配下拉选择的候选 SKU。 */ sku => sku.sku_id === event.target.value);
                        setSelectedSKUs(/* current 是更新前的分组选择索引。 */ current => ({ ...current, [key]: event.target.value }));
                        setCustomCosts(/* current 是更新前的历史成本文本索引。 */ current => ({ ...current, [key]: nextSKU ? (nextSKU.cost_cents / 100).toFixed(2) : '' }));
                      }}>
                        {candidate.skus.map(/* sku 是当前候选分组的一个可选 SKU。 */ sku => <option key={sku.sku_id} value={sku.sku_id}>{sku.label} · 售价 ¥{(sku.price_cents / 100).toFixed(2)} · 成本 ¥{(sku.cost_cents / 100).toFixed(2)}</option>)}
                      </select>}
                      <label className="relative"><span className="sr-only">历史单件成本</span><span className="pointer-events-none absolute left-3 top-2.5 text-sm text-gray-400">¥</span><input className="w-full rounded-xl border border-gray-200 py-2 pl-7 pr-3 text-sm font-bold" inputMode="decimal" value={customCostText} onChange={/* event 是用户输入历史单件成本的表单事件。 */ event => setCustomCosts(/* current 是更新前的历史成本文本索引。 */ current => ({ ...current, [key]: event.target.value }))} placeholder="历史成本" /></label>
                      <button type="button" className="rounded-xl bg-blue-600 px-4 py-2 text-sm font-bold text-white disabled:opacity-50" disabled={(!candidate.manual_only && !selectedSKU) || !customCostText || confirmingKey === key} onClick={/* 当前回调提交当前精确历史成本分组。 */ () => handleConfirmCandidate(candidate)}>{confirmingKey === key ? '确认中…' : `确认 ${candidate.order_count} 笔`}</button>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500"><span>商品成本 ¥{groupCost.toFixed(2)}</span><span>平台手续费 ¥{groupFee.toFixed(2)}</span><span className={groupProfit >= 0 ? 'font-bold text-green-600' : 'font-bold text-red-600'}>预估经营利润 ¥{groupProfit.toFixed(2)}</span></div>
                  </div>;
                })}
              </div>
            </div>
          )}
        </div>
        <div className="flex justify-end gap-3 border-t border-gray-100 px-6 py-5">
          <button type="button" className="rounded-xl border border-gray-200 px-5 py-2.5 font-bold text-gray-700" onClick={onClose}>{running ? '后台继续' : '关闭'}</button>
          {!job && !candidatesLoading && candidates.length === 0 && <button type="button" className="ios-btn-primary rounded-xl px-5 py-2.5 font-bold" disabled={starting} onClick={handleStart}>{starting ? '正在创建…' : '仅扫描新增订单'}</button>}
          {running && <button type="button" className="rounded-xl bg-red-500 px-5 py-2.5 font-bold text-white" onClick={handleCancel}>停止任务</button>}
        </div>
      </div>
    </div>
  );
  return typeof document === 'undefined' ? null : createPortal(modal, document.body);
};
