import { ArrowRight,CheckCircle2,Loader2,ReceiptText,TriangleAlert,X } from 'lucide-react';
import React from 'react';
import { adjustOrderPrice,getOrderAdjustPriceForm,type AdjustPriceField,type AdjustPriceForm } from '../api';

// AdjustPriceModalProps 描述真实改价弹窗的账号、订单和关闭／成功回调。
export interface AdjustPriceModalProps {
  // accountID 是卡片所属卖家账号。
  accountID: string;
  // orderID 是待付款平台订单标识。
  orderID: string;
  // open 表示弹窗是否挂载并读取最新 render。
  open: boolean;
  // onClose 关闭弹窗并取消未完成请求。
  onClose: () => void;
  // onSuccess 在平台明确成功后通知卡片更新临时状态。
  onSuccess?: (message: string) => void;
}

// AdjustPriceStep 表示单一模态内的编辑、最终确认和成功结果阶段，提交中由 submitting 叠加表达。
type AdjustPriceStep = 'edit' | 'confirm' | 'success';

/** payableTotal 把当前动态字段的普通元金额相加并输出两位小数；非法中间输入返回空值。 */
const payableTotal = (fields: AdjustPriceField[]): string => {
  // cents 累加所有金额字段的整数分值，避免浏览器浮点误差。
  let cents = 0;
  // field 是当前待累加的动态金额字段。
  for (const field /* field 是当前待累加的动态金额字段。 */ of fields) {
    // match 保存整数元和可选小数部分。
    const match = String(field.value || '').trim().match(/^(0|[1-9]\d*)(?:\.(\d{1,2}))?$/);
    if (!match) return '';
    // fraction 是补足两位的分文本。
    const fraction = (match[2] || '').padEnd(2, '0');
    cents += Number(match[1]) * 100 + Number(fraction || '0');
    if (!Number.isSafeInteger(cents)) return '';
  }
  return (cents / 100).toFixed(2);
};

/** AdjustPriceModal 读取闲鱼动态字段并在二次确认后执行一次真实改价。 */
export const AdjustPriceModal: React.FC<AdjustPriceModalProps> = ({ accountID, orderID, open, onClose, onSuccess }) => {
  // form 保存服务端 render 返回的平台表单，属于服务端数据。
  const [form, setForm] = React.useState<AdjustPriceForm | null>(null);
  // fields 保存用户当前编辑的动态金额字段，属于表单状态。
  const [fields, setFields] = React.useState<AdjustPriceField[]>([]);
  // loading 表示只读 render 请求进行中。
  const [loading, setLoading] = React.useState(false);
  // submitting 表示真实 submit 请求进行中，期间禁止重复提交和关闭误触。
  const [submitting, setSubmitting] = React.useState(false);
  // error 保存当前弹窗可展示的 render／submit 错误。
  const [error, setError] = React.useState('');
  // success 保存平台明确成功后的提示。
  const [success, setSuccess] = React.useState('');
  // step 保存当前单一模态流程阶段，避免调用浏览器原生 confirm 形成双层弹窗。
  const [step, setStep] = React.useState<AdjustPriceStep>('edit');
  // generationRef 隔离关闭、重开或订单切换后的晚到 render 响应。
  const generationRef = React.useRef(0);
  // submitControllerRef 保存当前真实 submit 的取消句柄；组件卸载或关闭时负责取消。
  const submitControllerRef = React.useRef<AbortController | null>(null);

  // 当前 effect 在弹窗打开时读取最新平台 render，cleanup 取消旧请求并推进代次。
  React.useEffect(/* loadAdjustPriceForm 在打开或订单切换时同步最新平台表单。 */ () => {
    generationRef.current += 1;
    // generation 是本次 render 响应允许写入状态的代次。
    const generation = generationRef.current;
    // controller 只控制本次动态表单读取。
    const controller = new AbortController();
    if (!open || !accountID || !orderID) {
      setForm(null);
      setFields([]);
      setError('');
      setSuccess('');
      setStep('edit');
      return /* emptyRenderCleanup 释放未启动或无效表单的控制器。 */ () => controller.abort();
    }
    setLoading(true);
    setError('');
    setSuccess('');
    setStep('edit');
    void getOrderAdjustPriceForm(accountID, orderID, { signal: controller.signal }).then(/* currentForm 是当前平台 render 表单。 */ currentForm => {
      if (generation !== generationRef.current) return;
      setForm(currentForm);
      setFields(currentForm.fields.map(/* currentField 复制为用户可编辑表单状态。 */ currentField => ({ ...currentField })));
    }).catch(/* renderError 是当前表单读取错误。 */ renderError => {
      if (generation !== generationRef.current || controller.signal.aborted) return;
      setError(renderError instanceof Error ? renderError.message : '读取改价表单失败');
    }).finally(/* renderFinished 仅由当前代次清理加载态。 */ () => {
      if (generation === generationRef.current) setLoading(false);
    });
    return /* renderCleanup 取消旧 render 并让晚到响应失效。 */ () => {
      controller.abort();
      generationRef.current += 1;
    };
  }, [accountID, open, orderID]);

  // 当前 effect 在组件卸载时取消真实 submit，避免晚到响应覆盖其他订单弹窗。
  React.useEffect(/* registerSubmitCleanup 只登记组件卸载清理。 */ () => /* submitCleanup 取消仍未结束的真实提交。 */ () => submitControllerRef.current?.abort(), []);

  if (!open) return null;
  // total 是动态字段当前买家应付合计，仅为展示派生值。
  const total = payableTotal(fields);
  // currentTotal 是平台 render 原始字段的买家应付合计，用于确认页展示改价前后差异。
  const currentTotal = payableTotal(form?.fields || []);
  // hasChanges 表示至少一个动态金额字段与平台 render 原值不同，无变化时不进入确认页。
  const hasChanges = Boolean(form && fields.some(/* currentField 检查当前输入是否改变同 key 原始金额。 */ currentField => form.fields.find(/* originalField 匹配当前字段的 render 原值。 */ originalField => originalField.key === currentField.key)?.value !== currentField.value));
  // modalTitle 根据当前阶段提供稳定标题，不再让浏览器原生弹窗显示本机地址。
  const modalTitle = step === 'confirm' ? '确认改价' : step === 'success' ? '价格修改成功' : (form?.title || '修改价格');
  // handleFieldChange 更新一个可编辑字段，保留平台字段顺序和元数据。
  const handleFieldChange = (key: string, value: string): void => {
    setFields(/* previousFields 是修改前动态字段列表。 */ previousFields => previousFields.map(/* previousField 是当前待匹配字段。 */ previousField => previousField.key === key ? { ...previousField, value } : previousField));
    setError('');
    setSuccess('');
  };
  // handleClose 取消 submit 并关闭弹窗；已发出的平台请求仍以服务端幂等状态为准。
  const handleClose = (): void => {
    submitControllerRef.current?.abort();
    onClose();
  };
  // handleBackToEdit 从确认页返回编辑页并保留用户输入，不触发任何平台请求。
  const handleBackToEdit = (): void => {
    if (submitting) return;
    setError('');
    setStep('edit');
  };
  // handleSubmit 第一次提交只进入应用内确认页，第二次才执行真实改价，并隔离重复点击和晚到响应。
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (!form || submitting || !total) {
      setError('请填写有效的价格和运费');
      return;
    }
    if (step === 'edit') {
      if (!hasChanges) {
        setError('请至少修改一项金额后再继续');
        return;
      }
      setError('');
      setStep('confirm');
      return;
    }
    if (step !== 'confirm') return;
    // controller 只控制本次真实 submit。
    const controller = new AbortController();
    submitControllerRef.current = controller;
    // generation 是 submit 返回时必须仍匹配的弹窗代次。
    const generation = generationRef.current;
    setSubmitting(true);
    setError('');
    setSuccess('');
    try {
      // result 是平台明确改价结果。
      const result = await adjustOrderPrice(accountID, orderID, fields, { signal: controller.signal });
      if (generation !== generationRef.current) return;
      setSuccess(result.message || '价格修改成功');
      setFields(/* previousFields 保留 render 名称和只读元数据，仅合并服务端规范金额。 */ previousFields => previousFields.map(/* previousField 是当前待合并的平台字段。 */ previousField => {
        // resultField 是服务端返回的同 key 规范金额；缺失时保留当前值。
        const resultField = result.fields.find(/* currentResultField 匹配当前 render key。 */ currentResultField => currentResultField.key === previousField.key);
        return resultField ? { ...previousField, value: resultField.value } : previousField;
      }));
      setStep('success');
      onSuccess?.(result.message || '价格修改成功');
    } catch (submitError /* submitError 是真实改价请求错误或人工核对状态。 */) {
      if (generation !== generationRef.current || controller.signal.aborted) return;
      setError(submitError instanceof Error ? submitError.message : '订单改价失败');
    } finally {
      if (generation === generationRef.current) setSubmitting(false);
      if (submitControllerRef.current === controller) submitControllerRef.current = null;
    }
  };

  return <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4 backdrop-blur-[2px]" role="presentation">
    <section role="dialog" aria-modal="true" aria-label={modalTitle} className="max-h-[calc(100vh-2rem)] w-full max-w-[30rem] overflow-hidden rounded-[20px] bg-white shadow-modal">
      <header className="flex items-center justify-between border-b border-slate-200 px-6 py-5">
        <div className="min-w-0"><h3 className="text-xl font-black leading-7 text-slate-950">{modalTitle}</h3><p className="mt-1 truncate text-xs font-medium tabular-nums text-slate-500">订单 {orderID}</p></div>
        <button type="button" aria-label="关闭改价弹窗" onClick={handleClose} disabled={submitting} className="flex h-9 w-9 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 disabled:opacity-40"><X className="h-4 w-4" /></button>
      </header>
      <form onSubmit={handleSubmit} className="max-h-[calc(100vh-7rem)] overflow-y-auto p-6">
        {step === 'edit' && <div className="space-y-5">
          {loading && <div className="flex items-center justify-center gap-2 py-10 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />正在读取闲鱼当前价格</div>}
          {!loading && fields.map(/* field 是当前平台动态金额字段。 */ field => <label key={field.key} className="block text-sm font-bold text-slate-700">
            <span>{field.name}</span><div className="mt-2 flex h-12 items-center rounded-xl border border-slate-200 bg-white px-4 transition focus-within:border-amber-400 focus-within:ring-4 focus-within:ring-amber-100/80">
              <span className="mr-2 text-slate-500">{field.prefix_text || '¥'}</span><input aria-label={field.name} inputMode="decimal" value={field.value} readOnly={field.read_only} onChange={/* priceInputChange 更新当前动态金额。 */ inputEvent => handleFieldChange(field.key, inputEvent.target.value)} className="min-w-0 flex-1 bg-transparent text-lg font-bold tabular-nums text-slate-950 outline-none read-only:text-slate-400" />
            </div>
          </label>)}
          {!loading && fields.length > 0 && <div className="rounded-2xl border border-amber-200 bg-amber-50/80 px-4 py-4">
            <div className="flex items-end justify-between gap-4"><div><p className="text-sm font-semibold text-slate-600">买家应付</p><p className="mt-1 text-xs text-slate-500">商品与运费合计</p></div><strong className="text-2xl font-black tabular-nums text-slate-950">¥{total || '--'}</strong></div>
            {!hasChanges && <p className="mt-3 border-t border-amber-200/70 pt-2.5 text-xs font-medium text-amber-700">当前金额尚未修改，调整后即可进入下一步</p>}
          </div>}
        </div>}

        {step === 'confirm' && <div className="space-y-5">
          <div className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4"><TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" aria-hidden="true" /><div><p className="text-sm font-black text-slate-900">请核对本次价格变化</p><p className="mt-1 text-xs leading-5 text-slate-600">确认后将立即更新买家的待付款订单；如果买家已经付款，服务端会停止提交。</p></div></div>
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
            <div className="grid grid-cols-[minmax(0,1fr)_auto_auto_auto] items-center gap-3 border-b border-slate-100 bg-slate-50 px-4 py-2.5 text-xs font-semibold text-slate-500"><span>项目</span><span>当前</span><span aria-hidden="true" /><span>修改后</span></div>
            {fields.map(/* currentField 是确认页当前待核对的目标字段。 */ currentField => {
              // originalField 是平台 render 中同 key 的原始字段，用于展示改价前金额。
              const originalField = form?.fields.find(/* candidateField 匹配当前目标字段的原始值。 */ candidateField => candidateField.key === currentField.key);
              // originalValue 是当前平台金额；平台缺失时回退目标值，避免显示虚构差异。
              const originalValue = originalField?.value || currentField.value;
              // changed 表示该字段是否确实发生金额变化。
              const changed = originalValue !== currentField.value;
              return <div key={currentField.key} className="grid grid-cols-[minmax(0,1fr)_auto_auto_auto] items-center gap-3 border-b border-slate-100 px-4 py-3.5 last:border-b-0"><span className="truncate text-sm font-bold text-slate-700">{currentField.name}</span><span className="text-sm font-semibold tabular-nums text-slate-500">{currentField.prefix_text || '¥'}{originalValue}</span><ArrowRight className="h-4 w-4 text-slate-300" aria-hidden="true" /><span className={`text-sm font-black tabular-nums ${changed ? 'text-amber-700' : 'text-slate-700'}`}>{currentField.prefix_text || '¥'}{currentField.value}</span></div>;
            })}
            <div className="flex items-center justify-between border-t border-amber-200 bg-amber-50/80 px-4 py-4"><div><p className="text-sm font-black text-slate-900">买家应付</p><p className="mt-0.5 text-xs text-slate-500">原 ¥{currentTotal || '--'}</p></div><strong className="text-2xl font-black tabular-nums text-slate-950">¥{total}</strong></div>
          </div>
        </div>}

        {step === 'success' && <div className="space-y-5" role="status">
          <div className="flex flex-col items-center py-2 text-center"><div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-100 text-emerald-600"><CheckCircle2 className="h-7 w-7" /></div><p className="mt-4 text-lg font-black text-slate-950">{success || '价格修改成功'}</p><p className="mt-1 text-sm text-slate-500">闲鱼系统卡片可能稍后到达，请勿重复提交。</p></div>
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-slate-50/70">
            {fields.map(/* resultField 是平台确认后展示的规范金额字段。 */ resultField => <div key={resultField.key} className="flex items-center justify-between border-b border-slate-200 px-4 py-3 last:border-b-0"><span className="text-sm font-semibold text-slate-600">{resultField.name}</span><strong className="text-sm font-black tabular-nums text-slate-950">{resultField.prefix_text || '¥'}{resultField.value}</strong></div>)}
            <div className="flex items-center justify-between border-t border-emerald-200 bg-emerald-50 px-4 py-4"><span className="text-sm font-black text-slate-900">买家应付</span><strong className="text-xl font-black tabular-nums text-slate-950">¥{payableTotal(fields) || '--'}</strong></div>
          </div>
        </div>}

        {error && step !== 'success' && <p role="alert" className="mt-5 rounded-2xl border border-red-100 bg-red-50 px-4 py-3 text-sm font-medium leading-5 text-red-700">{error}</p>}

        {step === 'edit' && <div className="mt-6 flex justify-end gap-2"><button type="button" onClick={handleClose} disabled={submitting} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100 disabled:opacity-40">取消</button><button type="submit" disabled={loading || fields.length === 0 || !total || !hasChanges} className="flex h-11 min-w-32 items-center justify-center rounded-xl bg-yellow-300 px-5 text-sm font-black text-slate-950 hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-50">下一步</button></div>}
        {step === 'confirm' && <div className="mt-6 flex justify-end gap-2"><button type="button" onClick={handleBackToEdit} disabled={submitting} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100 disabled:opacity-40">返回修改</button><button type="submit" disabled={submitting || !total} className="flex h-11 min-w-44 items-center justify-center gap-2 rounded-xl bg-yellow-300 px-5 text-sm font-black text-slate-950 hover:bg-yellow-400 disabled:cursor-not-allowed disabled:opacity-50">{submitting && <Loader2 className="h-4 w-4 animate-spin" />}{submitting ? '正在修改…' : `确认改为 ¥${total}`}</button></div>}
        {step === 'success' && <div className="mt-6 flex justify-end"><button type="button" onClick={handleClose} className="flex h-11 min-w-28 items-center justify-center gap-2 rounded-xl bg-slate-950 px-5 text-sm font-black text-white hover:bg-slate-800"><ReceiptText className="h-4 w-4" />完成</button></div>}
      </form>
    </section>
  </div>;
};
