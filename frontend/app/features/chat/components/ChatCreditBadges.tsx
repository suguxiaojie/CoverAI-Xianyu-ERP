import { Clock3,ShieldCheck } from 'lucide-react';
import React from 'react';
import type { ChatCreditLevel,ChatCreditProfile } from '../api';

/** creditToneClasses 按一至五级提供可读文字、浅底和同色边框，避免只靠颜色表达等级。 */
const creditToneClasses: Record<number,string> = {
	1: 'border-red-200 bg-red-50 text-red-700',
	2: 'border-orange-200 bg-orange-50 text-orange-700',
	3: 'border-amber-200 bg-amber-50 text-amber-700',
	4: 'border-blue-200 bg-blue-50 text-blue-700',
	5: 'border-emerald-200 bg-emerald-50 text-emerald-700',
};

/** ChatCreditBadgesProps 描述当前会话顶部信用标签的服务端状态。 */
export interface ChatCreditBadgesProps {
	/** profile 是当前会话买家的结构化信用和缓存时间。 */
	profile?: ChatCreditProfile;
	/** loading 表示停留防抖结束后的查询仍在执行。 */
	loading: boolean;
	/** error 是查询失败或熔断的非敏感提示。 */
	error: string;
}

/** creditRoleLabel 返回当前信用角色的中文名称。 */
const creditRoleLabel = (role: ChatCreditLevel['role']): string => role === 'buyer' ? '买家信用' : '卖家信用';

/** creditGradeLabel 优先使用平台文案，并去除已经由标签前缀表达的角色文字。 */
const creditGradeLabel = (level: ChatCreditLevel): string => {
	// roleLabel 是当前角色的中文标签前缀。
	const roleLabel = creditRoleLabel(level.role);
	// platformText 是去除空白后的平台完整信用文案。
	const platformText = String(level.text || '').trim();
	return platformText.startsWith(roleLabel) ? platformText.slice(roleLabel.length) || `等级 ${level.level}` : platformText || `等级 ${level.level}`;
};

/** creditFetchedLabel 把 UTC 时间转换为北京时间的紧凑展示文本。 */
const creditFetchedLabel = (value: string): string => {
	// parsed 是服务端 RFC3339 时间；无效值不显示误导时间。
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return '更新时间未知';
	return `更新于 ${parsed.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false, month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}`;
};

/** CreditBadge 渲染一个同时包含角色、文案和语义颜色的可点击信用标签。 */
const CreditBadge: React.FC<{/** level 是当前角色信用。 */ level: ChatCreditLevel; /** onOpen 打开统一信用详情。 */ onOpen: () => void}> = ({ level,onOpen }) => {
	// roleLabel 和 gradeLabel 分别是标签的角色和等级文案。
	const [roleLabel, gradeLabel] = [creditRoleLabel(level.role), creditGradeLabel(level)];
	// toneClasses 是数字等级对应的语义色；未知等级使用中性灰。
	const toneClasses = creditToneClasses[level.level] || 'border-slate-200 bg-slate-100 text-slate-600';
	return <button type="button" onClick={onOpen} data-credit-level={level.level} aria-label={`查看${roleLabel}：${gradeLabel}`}
		className={`inline-flex h-6 shrink-0 items-center gap-1 rounded-full border px-2 text-[11px] font-bold transition hover:brightness-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300 ${toneClasses}`}>
		<span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />
		<span>{roleLabel}·{gradeLabel}</span>
	</button>;
};

/** ChatCreditBadges 只在当前打开会话顶部展示信用，并用弹层解释双角色与缓存状态。 */
export const ChatCreditBadges: React.FC<ChatCreditBadgesProps> = ({ profile,loading,error }) => {
	// open 控制当前会话信用详情弹层。
	const [open, setOpen] = React.useState(false);
	// ownerRef 指向标签和弹层共同容器，用于点击外部关闭。
	const ownerRef = React.useRef<HTMLDivElement | null>(null);
	// levels 按业务优先级先买家后卖家，不为平台未返回的角色构造占位。
	const levels = React.useMemo(/* orderedCreditLevels 只派生最多两个当前信用角色。 */ () => [profile?.buyer, profile?.seller].filter(/* level 是当前判断是否存在的信用角色。 */ (level): level is ChatCreditLevel => Boolean(level)), [profile]);

	React.useEffect(/* closeCreditPopover 监听弹层外点击和 Escape，卸载时移除全局监听。 */ () => {
		if (!open) return undefined;
		/** handlePointerDown 在点击信用容器之外时关闭详情。 */
		const handlePointerDown = (event: PointerEvent): void => {
			if (!ownerRef.current?.contains(event.target as Node)) setOpen(false);
		};
		/** handleKeyDown 允许 Escape 关闭信用详情而不影响聊天输入。 */
		const handleKeyDown = (event: KeyboardEvent): void => {
			if (event.key === 'Escape') setOpen(false);
		};
		document.addEventListener('pointerdown', handlePointerDown);
		document.addEventListener('keydown', handleKeyDown);
		return /* creditPopoverCleanup 移除当前详情弹层拥有的全局监听。 */ () => {
			document.removeEventListener('pointerdown', handlePointerDown);
			document.removeEventListener('keydown', handleKeyDown);
		};
	}, [open]);

	if (loading && !profile) {
		return <span className="inline-flex h-6 items-center gap-1 rounded-full border border-slate-200 bg-slate-100 px-2 text-[11px] font-bold text-slate-500" aria-label="信用获取中">
			<Clock3 className="h-3 w-3" aria-hidden="true" />信用获取中
		</span>;
	}
	if (!profile && error) {
		// errorLabel 区分平台熔断和普通未获取，二者都不得使用信用较差的红色。
		const errorLabel = error.includes('暂缓') ? '信用暂缓更新' : '信用暂未获取';
		return <span title={error} className="inline-flex h-6 items-center gap-1 rounded-full border border-slate-200 bg-slate-100 px-2 text-[11px] font-bold text-slate-500">
			<ShieldCheck className="h-3 w-3" aria-hidden="true" />{errorLabel}
		</span>;
	}
	if (!profile) return null;

	return <div ref={ownerRef} className="relative flex min-w-0 flex-wrap items-center gap-1.5">
		{levels.length > 0 ? levels.map(/* level 是当前渲染并可打开详情的角色信用。 */ level => <CreditBadge key={level.role} level={level} onOpen={/* openCreditDetails 打开统一信用详情。 */ () => setOpen(true)} />)
			: <button type="button" onClick={/* openEmptyCreditDetails 打开没有角色标签的说明。 */ () => setOpen(true)} className="inline-flex h-6 items-center rounded-full border border-slate-200 bg-slate-100 px-2 text-[11px] font-bold text-slate-500">暂无信用标签</button>}
		{open && <div role="dialog" aria-label="信用信息" className="absolute left-0 top-full z-50 mt-2 w-72 rounded-2xl border border-slate-200 bg-white p-4 shadow-xl">
			<div className="flex items-center gap-2 text-sm font-black text-slate-950"><ShieldCheck className="h-4 w-4 text-sky-600" aria-hidden="true" />信用信息</div>
			<div className="mt-3 space-y-2">
				{levels.length > 0 ? levels.map(/* level 是详情中的一行角色信用。 */ level => {
					// toneClasses 是详情等级文字的语义色。
					const toneClasses = creditToneClasses[level.level] || 'border-slate-200 bg-slate-100 text-slate-600';
					return <div key={level.role} className="flex items-center justify-between gap-3 rounded-xl bg-slate-50 px-3 py-2">
						<span className="text-xs font-semibold text-slate-600">{creditRoleLabel(level.role)}</span>
						<span className={`rounded-full border px-2 py-0.5 text-xs font-bold ${toneClasses}`}>{creditGradeLabel(level)}</span>
					</div>;
				}) : <div className="rounded-xl bg-slate-50 px-3 py-3 text-xs text-slate-500">平台暂未返回该用户的买家或卖家信用标签。</div>}
			</div>
			<div className="mt-3 flex items-center gap-1.5 text-[11px] text-slate-500"><Clock3 className="h-3.5 w-3.5" aria-hidden="true" />{creditFetchedLabel(profile.fetched_at)}{profile.stale ? ' · 当前展示缓存' : ''}</div>
			<p className="mt-2 text-[11px] leading-5 text-slate-400">信用信息来自闲鱼公开个人主页，仅供人工交易判断参考。</p>
		</div>}
	</div>;
};
