import { AlertCircle,BellRing,Bot,CalendarClock,Check,Clock,Edit2,Flower,Flower2,Loader2,MessageCircle,PackageCheck,Power,QrCode,RefreshCw,Sparkles,Trash2,User } from 'lucide-react';
import React from 'react';
import type { AccountDetail } from '../api';
import { accountRuntimePresentation } from '../runtime';
import type { AccountAutomationFocus,AccountEditFocus } from '../types';

// AccountCardProps 描述账号卡片展示所需的数据和动作。
export interface AccountCardProps {
  // account 是当前账号的非敏感展示数据。
  account: AccountDetail;
  // refreshing 表示当前账号是否正在刷新资料。
  refreshing: boolean;
  // deleting 表示当前账号是否正在删除。
  deleting: boolean;
  // onRefreshProfile 刷新账号昵称和头像。
  onRefreshProfile: (account: AccountDetail) => void | Promise<void>;
  // onReauthorize 启动当前账号二维码重新授权。
  onReauthorize: (account: AccountDetail) => void | Promise<void>;
  // onEdit 打开账号编辑弹窗。
  onEdit: (account: AccountDetail, focus?: AccountEditFocus) => void | Promise<void>;
  // onAI 打开账号 AI 设置弹窗。
  onAI: (account: AccountDetail) => void | Promise<void>;
  // onTasks 打开账号自动化任务弹窗。
  onTasks: (account: AccountDetail, focus?: AccountAutomationFocus) => void;
  // onToggle 切换账号启用状态。
  onToggle: (id: string, currentStatus: boolean) => void | Promise<void>;
  // onDelete 打开账号删除确认框。
  onDelete: (account: AccountDetail) => void;
}

// AccountCard 渲染单个账号的状态摘要与操作入口。
export const AccountCard = React.memo(/* AccountCard 负责渲染单个账号卡片及其操作。 */ function AccountCard({
  account,
  refreshing,
  deleting,
  onRefreshProfile,
  onReauthorize,
  onEdit,
  onAI,
  onTasks,
  onToggle,
  onDelete,
}: AccountCardProps) {
  // runtime 保存账号运行状态的展示样式。
  const runtime = accountRuntimePresentation(account);
  // requiresLogin 表示当前账号是否需要重新授权。
  const requiresLogin = account.runtime_state === 'auth_expired' || account.runtime_state === 'verification_required';
  // runtimeUpdatedLabel 展示选中账号最近一次运行状态更新时间。
  const runtimeUpdatedLabel = account.runtime_updated_at ? new Date(account.runtime_updated_at).toLocaleString('zh-CN', { hour12: false }) : '等待状态更新';
  // lastRateLabel 展示最近一次自动评价扫描时间，缺少记录时明确保持空态。
  const lastRateLabel = account.last_rate_scan_at ? new Date(account.last_rate_scan_at * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) : '尚未执行';
  // lastPolishLabel 展示最近一次商品擦亮时间，缺少记录时明确保持空态。
  const lastPolishLabel = account.last_polish_at ? new Date(account.last_polish_at * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) : '尚未执行';
  // runtimeClockLabel 把运行快照缩短为活动时间线使用的时分文案。
  const runtimeClockLabel = account.runtime_updated_at ? new Date(account.runtime_updated_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) : '等待更新';
	// automationItems 汇总当前账号的七项配置，并把点击动作映射到既有设置弹窗。
  const automationItems = [
    { label: 'AI 回复', enabled: account.ai_enabled === true, icon: <Bot className="h-5 w-5" />, tone: 'text-purple-600 bg-purple-50', open: /* 当前回调进入选中账号的 AI 设置。 */ () => onAI(account) },
    { label: '自动评价', enabled: account.auto_rate_enabled === true, icon: <MessageCircle className="h-5 w-5" />, tone: 'text-emerald-600 bg-emerald-50', open: /* 当前回调打开任务弹窗并定位自动评价。 */ () => onTasks(account, 'auto_rate') },
    { label: '每日擦亮', enabled: account.auto_polish_enabled === true, icon: <Sparkles className="h-5 w-5" />, tone: 'text-amber-600 bg-amber-50', open: /* 当前回调打开任务弹窗并定位每日擦亮。 */ () => onTasks(account, 'auto_polish') },
		{ label: '自动确认发货', enabled: account.auto_confirm === true, icon: <PackageCheck className="h-5 w-5" />, tone: 'text-blue-600 bg-blue-50', open: /* 当前回调打开编辑弹窗并定位自动确认发货。 */ () => onEdit(account, 'auto_confirm') },
		{ label: '收货提醒', enabled: account.auto_receipt_reminder_enabled === true, icon: <BellRing className="h-5 w-5" />, tone: 'text-cyan-600 bg-cyan-50', open: /* 当前回调打开任务弹窗并定位自动确认收货提醒。 */ () => onTasks(account, 'receipt_reminder') },
    { label: '自动求花', enabled: account.auto_request_flower_enabled === true, icon: <Flower2 className="h-5 w-5" />, tone: 'text-red-600 bg-red-50', open: /* 当前回调打开任务弹窗并定位自动求花。 */ () => onTasks(account, 'auto_request_flower') },
    { label: '自动收花', enabled: account.auto_receive_flower_enabled === true, icon: <Flower className="h-5 w-5" />, tone: 'text-red-600 bg-red-50', open: /* 当前回调打开任务弹窗并定位自动收花。 */ () => onTasks(account, 'auto_receive_flower') },
  ];

  return (
    <section className="overflow-hidden rounded-xl border border-gray-200 bg-white" data-testid="account-command-center" aria-label="账号控制中心">
      <div className="grid xl:grid-cols-[18rem_minmax(0,1fr)_15rem]">
        <div className="border-b border-gray-100 p-5 xl:border-b-0 xl:border-r">
          <div className="flex flex-col items-start gap-4 sm:flex-row sm:items-center xl:flex-col xl:items-start">
            <div className="relative flex-none">
              {account.avatar_url ? (
                <img src={account.avatar_url} alt={account.nickname || '账号头像'} className="h-16 w-16 rounded-full bg-gray-100 object-cover" />
              ) : (
                <div className="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 text-gray-400"><User className="h-8 w-8" /></div>
              )}
              <span className={`absolute -bottom-0.5 -right-0.5 flex h-5 w-5 items-center justify-center rounded-full border-2 border-white ${runtime.dot}`}>
                {account.runtime_state === 'online' && <Check className="h-3 w-3 text-white" />}
              </span>
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="text-xl font-extrabold text-gray-900">{account.nickname || account.remark || `账号 ${account.id.substring(0, 6)}...`}</h3>
                <span className={`rounded-md px-2 py-0.5 text-[11px] font-bold ${runtime.badge}`}>{runtime.label}</span>
              </div>
              <p className="mt-2 text-sm font-medium text-gray-500">{account.remark || '暂无备注'}</p>
              <p className="mt-1 font-mono text-xs text-gray-400">ID: {account.id}</p>
            </div>
          </div>
          <div className="mt-4 space-y-2 border-t border-gray-100 pt-4">
            <div className="flex items-center justify-between gap-3 text-xs"><span className="text-gray-500">滑块处理</span><span className={`font-bold ${account.show_browser ? 'text-amber-600' : 'text-brand-700'}`}>{account.show_browser ? '弹窗人工' : '自动处理'}</span></div>
            {account.paused && <div className="flex items-center gap-1.5 text-xs font-bold text-blue-700"><Clock className="h-3.5 w-3.5" />暂停处理中</div>}
            {account.profile_error && <div className="flex items-center gap-1.5 text-xs font-bold text-amber-700"><AlertCircle className="h-3.5 w-3.5" />资料未同步</div>}
            <div className={`rounded-lg px-3 py-2.5 text-xs ${requiresLogin ? 'bg-red-50 text-red-700' : 'bg-emerald-50 text-emerald-700'}`}>
              <div className="font-bold">{account.runtime_message || (account.runtime_connected ? '消息服务连接正常' : '等待运行状态')}</div>
              <div className="mt-1 opacity-70">状态更新：{runtimeUpdatedLabel}</div>
            </div>
          </div>
        </div>

        <div className="border-b border-gray-100 p-5 xl:border-b-0 xl:border-r">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h4 className="text-sm font-bold text-gray-900">自动化配置</h4>
            <span className="text-[11px] text-gray-400">点击项目进入对应设置</span>
          </div>
          <div className="grid overflow-hidden rounded-lg border border-gray-100 sm:grid-cols-2 lg:grid-cols-3">
            {automationItems.map(/* 当前回调渲染一个可进入既有设置弹窗的自动化状态。 */ automationItem => (
              <button key={automationItem.label} type="button" onClick={automationItem.open} aria-label={`配置${automationItem.label}`} className="group min-h-28 border-b border-r border-gray-100 p-4 text-left transition hover:bg-gray-50">
                <div className="flex items-center justify-between gap-3">
                  <span className={`flex h-9 w-9 items-center justify-center rounded-lg ${automationItem.tone}`}>{automationItem.icon}</span>
                  <span className={`relative h-6 w-11 rounded-full ring-1 ring-inset transition ${automationItem.enabled ? 'bg-brand ring-brand-700/20' : 'bg-slate-200 ring-slate-300'}`} aria-hidden="true"><span className={`absolute top-0.5 h-5 w-5 rounded-full bg-white shadow-sm ring-1 ring-slate-300/70 transition-transform ${automationItem.enabled ? 'translate-x-5' : 'translate-x-0.5'}`} /></span>
                </div>
                <div className="mt-3 text-sm font-bold text-gray-800">{automationItem.label}</div>
                <div className={`mt-1 text-xs font-semibold ${automationItem.enabled ? 'text-emerald-600' : 'text-slate-500'}`}>{automationItem.enabled ? '已开启' : '已关闭'}</div>
              </button>
            ))}
          </div>
        </div>

        <div className="p-5">
          <h4 className="mb-2 text-sm font-bold text-gray-900">快捷操作</h4>
          <div className="grid grid-cols-2 gap-1 xl:grid-cols-1">
            <button onClick={/* 当前回调刷新账号昵称和头像。 */ () => onRefreshProfile(account)} disabled={refreshing} className="flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-semibold text-gray-600 transition hover:bg-gray-50 disabled:opacity-50" title={`刷新当前账号 ${account.nickname || account.remark || account.id} 的昵称和头像`}><RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} /><span>刷新资料</span></button>
            <button onClick={/* 当前回调重新发起二维码授权。 */ () => onReauthorize(account)} className={`flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-bold transition ${requiresLogin ? 'bg-red-50 text-red-700 hover:bg-red-100' : 'bg-brand-50 text-brand-700 hover:bg-brand-100'}`} title="重新扫码授权当前账号"><QrCode className="h-4 w-4" /><span>重新授权</span></button>
            <button onClick={/* 当前回调打开账号编辑弹窗。 */ () => onEdit(account)} className="flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-semibold text-gray-600 transition hover:bg-gray-50" title="编辑账号"><Edit2 className="h-4 w-4" /><span>编辑</span></button>
            <button onClick={/* 当前回调打开 AI 设置弹窗。 */ () => onAI(account)} className="flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-semibold text-gray-600 transition hover:bg-gray-50" title="AI设置"><Bot className="h-4 w-4" /><span>AI 设置</span></button>
            <button onClick={/* 当前回调打开账号自动化任务弹窗。 */ () => onTasks(account)} className="flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-semibold text-gray-600 transition hover:bg-gray-50" title="自动评价与每日擦亮"><CalendarClock className="h-4 w-4" /><span>自动化任务</span></button>
            <button onClick={/* 当前回调切换账号启用状态。 */ () => onToggle(account.id, account.enabled)} className={`flex min-h-10 items-center gap-2 rounded-lg px-3 text-xs font-semibold transition ${account.enabled ? 'text-emerald-600 hover:bg-emerald-50' : 'text-gray-500 hover:bg-gray-50'}`} title={account.enabled ? '停用账号' : '启用账号'}><Power className="h-4 w-4" /><span>{account.enabled ? '停用' : '启用'}</span></button>
            <button onClick={/* 当前回调打开账号删除确认框。 */ () => onDelete(account)} disabled={deleting} className="mt-1 flex min-h-10 items-center gap-2 rounded-lg border-t border-gray-100 px-3 text-xs font-semibold text-red-500 transition hover:bg-red-50 disabled:opacity-40" title={deleting ? '删除中…' : `删除账号 ${account.nickname || account.remark || account.id}`}>{deleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}<span>{deleting ? '删除中…' : '删除'}</span></button>
          </div>
        </div>
      </div>

      <div className="grid border-t border-gray-100 bg-gray-50/40 md:grid-cols-[9rem_repeat(3,minmax(0,1fr))]">
        <div className="flex items-center px-5 py-4 text-sm font-bold text-gray-900">最近活动</div>
        <div className="border-t border-gray-100 px-5 py-4 md:border-l md:border-t-0"><div className="flex items-center gap-2 text-xs font-bold text-gray-700"><span className={`h-2 w-2 rounded-full ${runtime.dot}`} />状态更新</div><div className="mt-1 text-xs text-gray-500">{runtimeClockLabel} · {runtime.label}</div></div>
        <div className="border-t border-gray-100 px-5 py-4 md:border-l md:border-t-0"><div className="flex items-center gap-2 text-xs font-bold text-gray-700"><MessageCircle className="h-4 w-4 text-emerald-600" />自动评价扫描</div><div className="mt-1 text-xs text-gray-500">{lastRateLabel}{account.last_rate_scan_at ? ' · 已完成' : ''}</div></div>
        <div className="border-t border-gray-100 px-5 py-4 md:border-l md:border-t-0"><div className="flex items-center gap-2 text-xs font-bold text-gray-700"><Sparkles className="h-4 w-4 text-amber-600" />商品擦亮</div><div className="mt-1 text-xs text-gray-500">{lastPolishLabel}{account.last_polish_at ? ' · 已完成' : ''}</div></div>
      </div>
    </section>
  );
});
