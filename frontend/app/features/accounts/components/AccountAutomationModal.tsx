import { BellRing,CalendarClock,CheckCircle2,Loader2,MessageSquareQuote,Play,Save,Sparkles,X } from 'lucide-react';
import React,{ useEffect,useRef } from 'react';
import { AccountDetail,AccountTaskSettings } from '../api';
import { useAccountAutomation } from '../accountAutomationHooks';
import type { AccountAutomationFocus } from '../types';

interface Props {
  /** account 表示账号。 */ account: AccountDetail;
  /** onClose 表示关闭弹窗的回调。 */ onClose: () => void;
  /** onSaved 表示保存完成后的回调。 */ onSaved: (settings: AccountTaskSettings) => void;
  /** onSaveSucceeded 在设置保存成功后关闭弹窗并展示页面气泡。 */ onSaveSucceeded?: () => void;
  /** onSaveFailed 在设置保存失败后展示页面气泡，弹窗保持打开。 */ onSaveFailed?: (message: string) => void;
	/** initialFocus 是弹窗打开后需要平滑定位的自动任务区块。 */ initialFocus?: AccountAutomationFocus;
}

// Toggle 渲染可复用的开关控件。
const Toggle: React.FC<{/** checked 表示开关当前是否选中。 */ checked: boolean; /** onChange 表示开关状态变化的回调。 */ onChange: () => void; /** label 表示控件的无障碍名称。 */ label: string}> = ({ checked, onChange, label }) => (
  <button type="button" aria-label={label} aria-pressed={checked} onClick={onChange}
    className={`relative h-8 w-14 shrink-0 rounded-full transition-colors ${checked ? 'bg-sky-500' : 'bg-slate-300'}`}>
    <span className={`absolute left-1 top-1 h-6 w-6 rounded-full bg-white shadow-sm transition-transform ${checked ? 'translate-x-6' : ''}`} />
  </button>
);

// AccountAutomationModal 渲染账号自动化设置弹窗。
const AccountAutomationModal: React.FC<Props> = ({ account, onClose, onSaved, onSaveSucceeded, onSaveFailed, initialFocus }) => {
  // automationState 是账号任务 feature Hook 提供的表单和动作状态。
  const { form, loading, saving, running, error, summary, retryAvailable, saved, setForm, save, run, retry } = useAccountAutomation({ account, onSaved, onSaveSucceeded, onSaveFailed });
	// rateSectionRef 指向自动评价设置区块。
	const rateSectionRef = useRef<HTMLElement | null>(null);
	// flowerRequestSectionRef 指向自动求花设置区块。
	const flowerRequestSectionRef = useRef<HTMLElement | null>(null);
	// receiptReminderSectionRef 指向自动确认收货提醒设置区块。
	const receiptReminderSectionRef = useRef<HTMLElement | null>(null);
	// flowerReceiveSectionRef 指向自动收花设置区块。
	const flowerReceiveSectionRef = useRef<HTMLDivElement | null>(null);
	// polishSectionRef 指向每日擦亮设置区块。
	const polishSectionRef = useRef<HTMLElement | null>(null);

	// 当前 effect 在弹窗完成挂载后滚动最近的 modal-body，而不是移动背景账号页面。
	useEffect(/* focusAutomationSection 根据卡片来源定位对应自动任务设置。 */ () => {
		// target 是当前入口要求定位的设置元素；普通快捷入口不传 initialFocus。
		const target = initialFocus === 'auto_rate' ? rateSectionRef.current
			: initialFocus === 'receipt_reminder' ? receiptReminderSectionRef.current
			: initialFocus === 'auto_request_flower' ? flowerRequestSectionRef.current
				: initialFocus === 'auto_receive_flower' ? flowerReceiveSectionRef.current
					: initialFocus === 'auto_polish' ? polishSectionRef.current : null;
		if (!target) return undefined;
		// frameID 确保 Portal 与 modal-body 完成布局后再执行平滑定位。
		const frameID = window.requestAnimationFrame(/* scrollAfterAutomationLayout 在弹窗布局稳定后滚动目标区块。 */ () => target.scrollIntoView({ behavior: 'smooth', block: 'start' }));
		return /* cancelAutomationScroll 在目标变化或弹窗卸载时取消尚未执行的定位。 */ () => window.cancelAnimationFrame(frameID);
	}, [initialFocus]);

	// 当前 effect 在弹窗打开期间接管 Escape，关闭后移除监听以免影响账号页其他键盘操作。
	useEffect(/* closeAutomationModalOnEscape 把 Escape 映射到现有关闭动作。 */ () => {
		// handleEscape 是当前弹窗唯一的文档级键盘监听器，其他按键保持原行为。
		const handleEscape = (event: KeyboardEvent): void => {
			if (event.key !== 'Escape') return;
			event.preventDefault();
			onClose();
		};
		document.addEventListener('keydown', handleEscape);
		return /* removeEscapeListener 弹窗卸载或 onClose 变化时释放旧监听器。 */ () => document.removeEventListener('keydown', handleEscape);
	}, [onClose]);

  return (
    <div className="modal-overlay-centered">
      <div className="modal-container" style={{ maxWidth: '640px' }} role="dialog" aria-modal="true" aria-labelledby="account-task-title">
        <div className="modal-header">
          <div>
            <h3 id="account-task-title" className="text-2xl font-extrabold text-slate-950">账号自动任务</h3>
            <p className="mt-1 text-sm text-slate-500">{account.nickname || account.remark || account.id}</p>
          </div>
          <button type="button" onClick={onClose} className="rounded-xl p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-700" aria-label="关闭">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="modal-body space-y-5">
          {loading && <div className="rounded-xl bg-slate-50 px-4 py-3 text-sm text-slate-500">正在读取任务设置...</div>}
          <section ref={rateSectionRef} data-testid="account-task-section-auto-rate" className="scroll-mt-16 rounded-2xl border border-slate-200 bg-white p-5">
            <div className="flex items-start gap-4">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600"><MessageSquareQuote className="h-5 w-5" /></div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h4 className="font-black text-slate-900">自动评价</h4>
                    <p className="mt-1 text-xs leading-5 text-slate-500">持续扫描待评价订单，按订单执行；不是每日任务。</p>
                  </div>
                  <Toggle checked={form.auto_rate_enabled} onChange={/* 当前回调处理用户交互或异步状态变化。 */ () => setForm(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, auto_rate_enabled: !current.auto_rate_enabled }))} label="自动评价" />
                </div>
                <label className="mt-4 block text-xs font-extrabold text-slate-600">统一好评文案</label>
                <textarea aria-label="统一好评文案" value={form.rate_content} maxLength={500} onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setForm(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, rate_content: event.target.value }))}
                  className="mt-2 h-24 w-full resize-none rounded-xl border border-slate-200 bg-slate-50 px-3 py-2.5 text-sm leading-6 text-slate-800 placeholder:text-slate-400 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100" />
                <div className="mt-3 flex items-center justify-between gap-3">
                  <span className="text-xs text-slate-400">最近扫描：{form.last_rate_scan_at ? new Date(form.last_rate_scan_at * 1000).toLocaleString('zh-CN') : '尚未执行'}</span>
                  <button type="button" disabled={running !== '' || !account.enabled} onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void run('auto_rate')}
                    className="flex items-center gap-2 rounded-xl bg-emerald-50 px-3 py-2 text-xs font-extrabold text-emerald-700 hover:bg-emerald-100 disabled:opacity-50">
                    {running === 'auto_rate' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}立即评价
                  </button>
                </div>
              </div>
            </div>
		  </section>

		  <section ref={receiptReminderSectionRef} data-testid="account-task-section-receipt-reminder" className="scroll-mt-16 rounded-2xl border border-cyan-100 bg-cyan-50/30 p-5">
			<div className="flex items-start gap-4">
			  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-cyan-100 text-cyan-700"><BellRing className="h-5 w-5" /></div>
			  <div className="min-w-0 flex-1">
				<div className="flex items-start justify-between gap-4">
				  <div>
					<h4 className="font-black text-slate-900">自动确认收货提醒</h4>
					<p className="mt-1 text-xs leading-5 text-slate-500">发货满完整天数后的下一次北京时间，调用闲鱼官方接口生成“记得及时确认收货”系统卡片。</p>
				  </div>
				  <Toggle checked={form.auto_receipt_reminder_enabled} onChange={/* receiptReminderToggle 切换当前店铺自动确认收货提醒。 */ () => setForm(/* receiptReminderDraft 更新提醒开关。 */ current => ({ ...current, auto_receipt_reminder_enabled: !current.auto_receipt_reminder_enabled }))} label="自动确认收货提醒" />
				</div>
				<div className="mt-4 grid gap-4 sm:grid-cols-2">
				  <label className="block">
					<span className="text-xs font-extrabold text-slate-600">发货后等待（天）</span>
					<input aria-label="确认收货提醒等待天数" type="number" min={1} max={30} value={form.receipt_reminder_after_days} onChange={/* receiptDaysChange 更新完整等待天数。 */ event => {
					  // reminderDays 是当前输入框已经转换为数字的完整等待天数。
					  const reminderDays = Number(event.target.value);
					  setForm(/* receiptDaysDraft 保存提醒等待天数。 */ current => ({ ...current, receipt_reminder_after_days: reminderDays }));
					}} className="mt-2 block w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-bold text-slate-800 outline-none focus:border-cyan-400" />
				  </label>
				  <label className="block">
					<span className="text-xs font-extrabold text-slate-600">每日执行时间（北京时间）</span>
					<input aria-label="确认收货提醒执行时间" type="time" value={form.receipt_reminder_time} onChange={/* receiptTimeChange 更新每日执行时间。 */ event => {
					  // reminderTime 是当前输入的北京时间 HH:mm 文本。
					  const reminderTime = event.target.value;
					  setForm(/* receiptTimeDraft 保存提醒时间。 */ current => ({ ...current, receipt_reminder_time: reminderTime }));
					}} className="mt-2 block w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-bold text-slate-800 outline-none focus:border-cyan-400" />
				  </label>
				</div>
				<p className="mt-4 text-xs leading-5 text-cyan-800">卡片标题和内容由闲鱼固定生成，不发送普通聊天文字。同一订单每天平台最多接受一次；ERP 永久只执行一次。首次开启或重新开启后只处理新的发货订单，不追发历史订单。</p>
			  </div>
			</div>
		  </section>

		  <section ref={flowerRequestSectionRef} data-testid="account-task-section-auto-request-flower" className="scroll-mt-16 rounded-2xl border border-rose-100 bg-rose-50/30 p-5">
            <div className="flex items-start gap-4">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-rose-100 text-rose-600"><Sparkles className="h-5 w-5" /></div>
              <div className="min-w-0 flex-1 space-y-5">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h4 className="font-black text-slate-900">自动求小红花</h4>
                    <p className="mt-1 text-xs leading-5 text-slate-500">明确发货成功后自动发送一次官方求花卡片，同一订单不会重复发送。</p>
                  </div>
                  <Toggle checked={form.auto_request_flower_enabled} onChange={/* 当前回调切换自动求花设置。 */ () => setForm(/* 当前回调更新账号任务草稿。 */ current => ({ ...current, auto_request_flower_enabled: !current.auto_request_flower_enabled }))} label="自动求小红花" />
                </div>
                <label className="block max-w-[220px]">
                  <span className="text-xs font-extrabold text-slate-600">发货成功后等待（秒）</span>
                  <input aria-label="自动求花延迟秒数" type="number" min={0} max={3600} value={form.request_flower_after_seconds}
                    onChange={/* 当前回调更新发货后求花延迟。 */ event => {
                      // delaySeconds 是用户当前输入的发货成功后等待秒数。
                      const delaySeconds = Number(event.target.value);
                      setForm(/* 当前回调更新账号任务草稿。 */ current => ({ ...current, request_flower_after_seconds: delaySeconds }));
                    }}
                    className="mt-2 block w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-bold text-slate-800 outline-none focus:border-rose-400" />
                </label>

                <div ref={flowerReceiveSectionRef} data-testid="account-task-section-auto-receive-flower" className="scroll-mt-4 border-t border-rose-100 pt-5">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h4 className="font-black text-slate-900">自动收小红花</h4>
                      <p className="mt-1 text-xs leading-5 text-slate-500">检测到买家送花后打开隔离官方页面，以后续“已收花”系统消息作为成功依据。</p>
                    </div>
                    <Toggle checked={form.auto_receive_flower_enabled} onChange={/* 当前回调切换自动收花设置。 */ () => setForm(/* 当前回调更新账号任务草稿。 */ current => ({ ...current, auto_receive_flower_enabled: !current.auto_receive_flower_enabled }))} label="自动收小红花" />
                  </div>
                  <div className="mt-4 grid gap-4 sm:grid-cols-2">
                    <div className="flex items-center justify-between gap-3 rounded-xl border border-rose-100 bg-white px-3 py-2.5">
                      <span className="text-xs font-extrabold text-slate-600">显示浏览器窗口</span>
                      <Toggle checked={form.receive_flower_show_browser} onChange={/* 当前回调切换自动收花浏览器可见性。 */ () => setForm(/* 当前回调更新账号任务草稿。 */ current => ({ ...current, receive_flower_show_browser: !current.receive_flower_show_browser }))} label="显示收花浏览器" />
                    </div>
                    <label className="block">
                      <span className="text-xs font-extrabold text-slate-600">结果超时（秒）</span>
                      <input aria-label="自动收花超时秒数" type="number" min={30} max={600} value={form.receive_flower_timeout_seconds}
                        onChange={/* 当前回调更新收花结果超时。 */ event => {
                          // timeoutSeconds 是用户当前输入的平台结果等待秒数。
                          const timeoutSeconds = Number(event.target.value);
                          setForm(/* 当前回调更新账号任务草稿。 */ current => ({ ...current, receive_flower_timeout_seconds: timeoutSeconds }));
                        }}
                        className="mt-2 block w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm font-bold text-slate-800 outline-none focus:border-rose-400" />
                    </label>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section ref={polishSectionRef} data-testid="account-task-section-auto-polish" className="scroll-mt-4 rounded-2xl border border-slate-200 bg-white p-5">
            <div className="flex items-start gap-4">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-50 text-amber-600"><Sparkles className="h-5 w-5" /></div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h4 className="font-black text-slate-900">每日自动擦亮</h4>
                    <p className="mt-1 text-xs leading-5 text-slate-500">每个账号每天最多执行一次，按北京时间判断。</p>
                  </div>
                  <Toggle checked={form.auto_polish_enabled} onChange={/* 当前回调处理用户交互或异步状态变化。 */ () => setForm(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, auto_polish_enabled: !current.auto_polish_enabled }))} label="每日自动擦亮" />
                </div>
                <div className="mt-4 flex items-end justify-between gap-4">
                  <label className="block">
                    <span className="text-xs font-extrabold text-slate-600">每日执行时间</span>
                    <input type="time" value={form.polish_time} onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setForm(/* 当前回调处理用户交互或异步状态变化。 */ current => ({ ...current, polish_time: event.target.value }))}
                      className="mt-2 block rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-sm font-bold text-slate-800 outline-none focus:border-sky-400" />
                  </label>
                  <button type="button" disabled={running !== '' || !account.enabled} onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void run('auto_polish')}
                    className="flex items-center gap-2 rounded-xl bg-amber-50 px-3 py-2 text-xs font-extrabold text-amber-700 hover:bg-amber-100 disabled:opacity-50">
                    {running === 'auto_polish' ? <Loader2 className="h-4 w-4 animate-spin" /> : <CalendarClock className="h-4 w-4" />}立即擦亮
                  </button>
                </div>
                <div className="mt-3 text-xs text-slate-400">上次完成：{form.last_polish_at ? new Date(form.last_polish_at * 1000).toLocaleString('zh-CN') : '尚未执行'}</div>
              </div>
            </div>
          </section>

          {summary && (
            <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-sm text-sky-900">
              本次发现 {summary.found} 项，成功 {summary.success}，失败 {summary.failed}，跳过 {summary.skipped}。{summary.message || ''}
            </div>
          )}
          {saved && (
            <div role="status" className="flex items-start gap-2 rounded-xl border border-emerald-100 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-800">
              <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
			  <span>设置已保存。{form.auto_receipt_reminder_enabled ? `确认收货提醒将在发货满 ${form.receipt_reminder_after_days} 天后的 ${form.receipt_reminder_time} 起执行。` : form.auto_request_flower_enabled ? `自动求花将在明确发货成功后 ${form.request_flower_after_seconds} 秒执行。` : '自动消息任务当前关闭。'}</span>
            </div>
          )}
          {error && <div className="flex items-center justify-between gap-3 rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm font-medium text-red-700"><span>{error}</span>{retryAvailable && <button type="button" className="font-bold underline" onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void retry()}>重试</button>}</div>}
        </div>

        <div className="modal-footer">
          <div className="flex w-full gap-3">
            <button type="button" onClick={onClose} disabled={saving || running !== ''}
              className="flex-1 rounded-xl bg-gray-100 px-6 py-3 font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:opacity-50">关闭</button>
			<button type="button" onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void save()} disabled={saving || saved || running !== '' || (form.auto_rate_enabled && !form.rate_content.trim()) || form.request_flower_after_seconds < 0 || form.request_flower_after_seconds > 3600 || form.receive_flower_timeout_seconds < 30 || form.receive_flower_timeout_seconds > 600 || form.receipt_reminder_after_days < 1 || form.receipt_reminder_after_days > 30}
              className="ios-btn-primary flex flex-1 items-center justify-center gap-2 rounded-xl px-6 py-3 font-bold disabled:opacity-50">
              {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : saved ? <CheckCircle2 className="h-4 w-4" /> : <Save className="h-4 w-4" />}{saved ? '已保存' : '保存'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AccountAutomationModal;
