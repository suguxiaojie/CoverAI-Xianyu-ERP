import { CheckCircle2,Loader2,TriangleAlert,X } from 'lucide-react';
import React from 'react';
import { closeOrderBySeller,getCloseOrderReasons } from '../api';

// CloseOrderModalProps 描述卖家取消订单弹窗的账号、阶段和关闭／成功回调。
export interface CloseOrderModalProps {
  // accountID 是卡片所属卖家账号。
  accountID: string;
  // orderID 是待取消平台订单号。
  orderID: string;
	// stage 区分待付款与已付款待发货，决定二次确认中的资金说明。
	stage: 'pending_payment' | 'pending_ship';
  // open 表示弹窗是否挂载并读取最新原因。
  open: boolean;
  // onClose 关闭弹窗并取消未完成请求。
  onClose: () => void;
  // onSuccess 在平台明确关单后通知卡片更新临时状态。
  onSuccess?: (message: string) => void;
}

// CloseOrderStep 表示单一模态内的原因选择、最终确认和成功结果阶段。
type CloseOrderStep = 'select' | 'confirm' | 'success';

/** CloseOrderModal 动态读取平台原因，并在应用内二次确认后执行一次真实卖家关单。 */
export const CloseOrderModal: React.FC<CloseOrderModalProps> = ({ accountID, orderID, stage, open, onClose, onSuccess }) => {
  // reasons 保存服务端返回的平台动态原因，属于服务端数据。
  const [reasons, setReasons] = React.useState<string[]>([]);
  // selectedReason 保存用户当前选择，属于表单状态。
  const [selectedReason, setSelectedReason] = React.useState('');
  // step 保存当前单一模态流程阶段。
  const [step, setStep] = React.useState<CloseOrderStep>('select');
  // loading 表示只读原因请求进行中。
  const [loading, setLoading] = React.useState(false);
  // submitting 表示真实关单请求进行中，期间禁止重复提交和关闭误触。
  const [submitting, setSubmitting] = React.useState(false);
  // error 保存原因或关单错误。
  const [error, setError] = React.useState('');
  // success 保存平台明确成功后的提示。
  const [success, setSuccess] = React.useState('');
  // generationRef 隔离关闭、重开或订单切换后的晚到响应。
  const generationRef = React.useRef(0);
  // submitControllerRef 保存当前真实关单的取消句柄。
  const submitControllerRef = React.useRef<AbortController | null>(null);

  // 当前 effect 在弹窗打开时读取最新平台原因，cleanup 取消旧请求并推进代次。
  React.useEffect(/* loadCloseReasons 在打开或订单切换时同步最新平台原因。 */ () => {
    generationRef.current += 1;
    // generation 是本次原因响应允许写入状态的代次。
    const generation = generationRef.current;
    // controller 只控制本次动态原因读取。
    const controller = new AbortController();
    setReasons([]); setSelectedReason(''); setStep('select'); setError(''); setSuccess('');
    if (!open || !accountID || !orderID) return /* emptyReasonCleanup 释放未启动请求的控制器。 */ () => controller.abort();
    setLoading(true);
    void getCloseOrderReasons(accountID, orderID, { signal: controller.signal }).then(/* result 是当前平台动态原因。 */ result => {
      if (generation !== generationRef.current) return;
      setReasons(result.reasons || []);
    }).catch(/* reasonError 是当前原因读取错误。 */ reasonError => {
      if (generation !== generationRef.current || controller.signal.aborted) return;
      setError(reasonError instanceof Error ? reasonError.message : '读取关闭原因失败');
    }).finally(/* reasonFinished 仅由当前代次清理加载态。 */ () => {
      if (generation === generationRef.current) setLoading(false);
    });
    return /* reasonCleanup 取消旧请求并让晚到响应失效。 */ () => { controller.abort(); generationRef.current += 1; };
  }, [accountID, open, orderID]);

  // 当前 effect 在组件卸载时取消真实关单请求。
  React.useEffect(/* registerCloseSubmitCleanup 只登记组件卸载清理。 */ () => /* closeSubmitCleanup 取消仍未结束的真实关单。 */ () => submitControllerRef.current?.abort(), []);

  if (!open) return null;
  // modalTitle 根据当前阶段提供稳定标题。
  const modalTitle = step === 'confirm' ? '确认取消订单' : step === 'success' ? '订单已取消' : '取消订单';
  // handleClose 取消请求并关闭弹窗；已发出的平台请求仍以服务端幂等状态为准。
  const handleClose = (): void => { submitControllerRef.current?.abort(); onClose(); };
  // handleBack 返回原因选择页并保留用户选择。
  const handleBack = (): void => { if (!submitting) { setError(''); setStep('select'); } };
  // handleSubmit 第一次只进入确认页，第二次才执行真实卖家关单。
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (!selectedReason || submitting) { setError('请选择取消订单原因'); return; }
    if (step === 'select') { setError(''); setStep('confirm'); return; }
    if (step !== 'confirm') return;
    // controller 只控制本次真实关单。
    const controller = new AbortController();
    submitControllerRef.current = controller;
    // generation 是关单返回时必须仍匹配的弹窗代次。
    const generation = generationRef.current;
    setSubmitting(true); setError('');
    try {
      // result 是平台明确关单结果。
      const result = await closeOrderBySeller(accountID, orderID, selectedReason, { signal: controller.signal });
      if (generation !== generationRef.current) return;
      setSuccess(result.message || '订单已取消'); setStep('success'); onSuccess?.(result.message || '订单已取消');
    } catch (submitError /* submitError 是真实关单错误或人工核对状态。 */) {
      if (generation !== generationRef.current || controller.signal.aborted) return;
      setError(submitError instanceof Error ? submitError.message : '取消订单失败');
    } finally {
      if (generation === generationRef.current) setSubmitting(false);
      if (submitControllerRef.current === controller) submitControllerRef.current = null;
    }
  };

  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4 backdrop-blur-[2px]" role="presentation">
    <section role="dialog" aria-modal="true" aria-label={modalTitle} className="max-h-[calc(100vh-2rem)] w-full max-w-md overflow-hidden rounded-[20px] bg-white shadow-modal">
      <header className="flex items-center justify-between border-b border-slate-200 px-6 py-5"><div className="min-w-0"><h3 className="text-xl font-black text-slate-950">{modalTitle}</h3><p className="mt-1 truncate text-xs font-medium tabular-nums text-slate-500">订单 {orderID}</p></div><button type="button" aria-label="关闭取消订单弹窗" onClick={handleClose} disabled={submitting} className="flex h-9 w-9 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 disabled:opacity-40"><X className="h-4 w-4" /></button></header>
      <form onSubmit={handleSubmit} className="max-h-[calc(100vh-7rem)] overflow-y-auto p-6">
        {step === 'select' && <div className="space-y-4">
          <div><p className="text-sm font-black text-slate-900">请选择取消原因</p><p className="mt-1 text-xs leading-5 text-slate-500">原因来自闲鱼当前订单，提交前服务端会再次校验。</p></div>
          {loading && <div className="flex items-center justify-center gap-2 py-8 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />正在读取闲鱼取消原因</div>}
          {!loading && reasons.map(/* reason 是当前平台动态关闭原因。 */ reason => <label key={reason} className={`flex cursor-pointer items-center gap-3 rounded-xl border px-4 py-3 text-sm font-semibold transition ${selectedReason === reason ? 'border-red-300 bg-red-50 text-red-800' : 'border-slate-200 text-slate-700 hover:border-slate-300'}`}><input type="radio" name="close-reason" value={reason} checked={selectedReason === reason} onChange={/* selectCloseReason 保存用户选择并清除错误。 */ () => { setSelectedReason(reason); setError(''); }} className="h-4 w-4 accent-red-600" /><span>{reason}</span></label>)}
        </div>}
        {step === 'confirm' && <div className="space-y-5"><div className="flex gap-3 rounded-2xl border border-red-200 bg-red-50 px-4 py-4"><TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-red-600" /><div><p className="text-sm font-black text-red-900">{stage === 'pending_ship' ? '取消后买家已支付款项将原路退回' : '取消后买家将无法继续付款'}</p><p className="mt-1 text-xs leading-5 text-red-700">{stage === 'pending_ship' ? '当前订单已付款但尚未发货；提交前服务端会重新检查订单状态和平台动态原因。' : '当前订单尚未付款；提交前服务端会重新检查订单状态和平台动态原因。'}</p></div></div><div className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4"><p className="text-xs font-semibold text-slate-500">取消原因</p><p className="mt-1.5 text-base font-black text-slate-950">{selectedReason}</p></div></div>}
        {step === 'success' && <div className="flex flex-col items-center py-4 text-center" role="status"><div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-100 text-emerald-600"><CheckCircle2 className="h-7 w-7" /></div><p className="mt-4 text-lg font-black text-slate-950">{success || '订单已取消'}</p><p className="mt-1 text-sm text-slate-500">闲鱼系统卡片可能稍后到达，请勿重复操作。</p></div>}
        {error && step !== 'success' && <p role="alert" className="mt-5 rounded-2xl border border-red-100 bg-red-50 px-4 py-3 text-sm font-medium leading-5 text-red-700">{error}</p>}
        {step === 'select' && <div className="mt-6 flex justify-end gap-2"><button type="button" onClick={handleClose} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100">取消</button><button type="submit" disabled={loading || !selectedReason} className="h-11 min-w-32 rounded-xl bg-slate-950 px-5 text-sm font-black text-white hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40">下一步</button></div>}
        {step === 'confirm' && <div className="mt-6 flex justify-end gap-2"><button type="button" onClick={handleBack} disabled={submitting} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100 disabled:opacity-40">返回修改</button><button type="submit" disabled={submitting} className="flex h-11 min-w-40 items-center justify-center gap-2 rounded-xl bg-red-600 px-5 text-sm font-black text-white hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50">{submitting && <Loader2 className="h-4 w-4 animate-spin" />}{submitting ? '正在取消…' : '确认取消订单'}</button></div>}
        {step === 'success' && <div className="mt-6 flex justify-end"><button type="button" onClick={handleClose} className="h-11 min-w-28 rounded-xl bg-slate-950 px-5 text-sm font-black text-white hover:bg-slate-800">完成</button></div>}
      </form>
    </section>
  </div>;
};
