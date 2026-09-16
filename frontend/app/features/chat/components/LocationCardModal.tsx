import React from 'react';
import { AlertTriangle, Loader2, MapPinned, Send, X } from 'lucide-react';

/** LocationCardDraft 是用户每次发送前确认的位置卡片表单值。 */
export interface LocationCardDraft {
	/** title 是卡片主标题。 */ title: string;
	/** description 是地址或到店说明。 */ description: string;
	/** latitude 是 WGS84 纬度。 */ latitude: number;
	/** longitude 是 WGS84 经度。 */ longitude: number;
}

/** LocationCardModalProps 描述位置卡片弹窗的受控发送和关闭边界。 */
interface LocationCardModalProps {
	/** open 控制弹窗是否挂载。 */ open: boolean;
	/** sending 表示平台写入仍在进行，期间禁止重复提交和关闭。 */ sending: boolean;
	/** defaults 是系统设置页保存、打开本轮弹窗时带入的可编辑初值。 */
	defaults: { /** title 是默认标题。 */ title: string; /** description 是默认说明。 */ description: string; /** latitude 是默认纬度文本。 */ latitude: string; /** longitude 是默认经度文本。 */ longitude: string };
	/** onClose 关闭未提交的表单。 */ onClose: () => void;
	/** onSend 提交当前卡片；返回 true 时弹窗清空并关闭。 */ onSend: (draft: LocationCardDraft) => Promise<boolean>;
}

/** LocationCardStage 区分表单编辑和最终二次确认页面。 */
type LocationCardStage = 'edit' | 'confirm';

/** LocationCardModal 让用户逐次填写标题、说明和真实坐标，并在一次显式确认后发送。 */
export const LocationCardModal: React.FC<LocationCardModalProps> = ({ open, sending, defaults, onClose, onSend }) => {
	/** title、setTitle 是本次位置卡片标题表单状态。 */
	const [title, setTitle] = React.useState('');
	/** description、setDescription 是本次地址或到店说明表单状态。 */
	const [description, setDescription] = React.useState('');
	/** latitudeText、setLatitudeText 是保留用户输入精度的纬度文本。 */
	const [latitudeText, setLatitudeText] = React.useState('');
	/** longitudeText、setLongitudeText 是保留用户输入精度的经度文本。 */
	const [longitudeText, setLongitudeText] = React.useState('');
	/** formError、setFormError 保存仅属于当前弹窗的字段校验提示。 */
	const [formError, setFormError] = React.useState('');
	/** stage、setStage 控制编辑和不可误触的平台写入确认两步流程。 */
	const [stage, setStage] = React.useState<LocationCardStage>('edit');

	React.useEffect(/* resetLocationCardDraft 在每次重新打开时清除上一次客户的数据。 */ () => {
		if (!open) return;
		setTitle(defaults.title);
		setDescription(defaults.description);
		setLatitudeText(defaults.latitude);
		setLongitudeText(defaults.longitude);
		setFormError('');
		setStage('edit');
	}, [defaults.description, defaults.latitude, defaults.longitude, defaults.title, open]);

	if (!open) return null;
	/** latitude、longitude 是当前坐标文本转换出的十进制度。 */
	const latitude = Number(latitudeText);
	const longitude = Number(longitudeText);
	/** coordinateValid 表示两个坐标均为有限数且位于地理范围内。 */
	const coordinateValid = latitudeText.trim() !== '' && longitudeText.trim() !== '' && Number.isFinite(latitude) && Number.isFinite(longitude) && latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180;
	/** formValid 表示四个必填字段均可提交。 */
	const formValid = title.trim() !== '' && description.trim() !== '' && coordinateValid;

	/** advanceOrSubmitLocationCard 首次点击只进入确认页，第二次点击才等待平台明确结果。 */
	const advanceOrSubmitLocationCard = async (): Promise<void> => {
		if (sending) return;
		if (!formValid) {
			setFormError('请填写标题、说明和有效经纬度。');
			return;
		}
		setFormError('');
		if (stage === 'edit') {
			setStage('confirm');
			return;
		}
		/** succeeded 表示平台发送和本地状态均已明确收口。 */
		const succeeded = await onSend({ title: title.trim(), description: description.trim(), latitude, longitude });
		if (succeeded) onClose();
	};

	return <div role="presentation" className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 p-4 backdrop-blur-[2px]" onClick={/* closeLocationBackdrop 仅在未发送时关闭弹窗。 */ () => { if (!sending) onClose(); }}>
		<section role="dialog" aria-modal="true" aria-label="发送位置卡片" className="w-full max-w-xl overflow-hidden rounded-2xl bg-white shadow-2xl" onClick={/* keepLocationDialogOpen 阻止表单点击冒泡到遮罩。 */ event => event.stopPropagation()}>
			<header className="flex items-center justify-between border-b border-slate-200 px-6 py-4">
				<div><h2 className="text-lg font-black text-slate-950">{stage === 'edit' ? '发送位置卡片' : '确认发送位置卡片'}</h2><p className="mt-1 text-xs text-slate-500">{stage === 'edit' ? '每次发送前可自定义标题、说明和真实坐标。' : '请再次核对接收会话和卡片内容，确认后才会发送。'}</p></div>
				<button type="button" aria-label="关闭位置卡片弹窗" disabled={sending} onClick={onClose} className="flex h-9 w-9 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 disabled:opacity-40"><X className="h-4 w-4" /></button>
			</header>
			<div className="space-y-4 p-6">
				{stage === 'edit' ? <>
					<label className="block"><span className="text-sm font-bold text-slate-700">卡片标题</span><input aria-label="位置卡片标题" value={title} maxLength={80} onChange={/* updateLocationTitle 更新当前卡片标题。 */ event => { setTitle(event.target.value); setFormError(''); }} placeholder="例如：CoverAI 实体店" className="mt-2 h-11 w-full rounded-xl border border-slate-200 px-3 text-sm outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100" /></label>
					<label className="block"><span className="text-sm font-bold text-slate-700">地址／到店说明</span><textarea aria-label="位置卡片说明" value={description} maxLength={300} rows={3} onChange={/* updateLocationDescription 更新当前地址或到店说明。 */ event => { setDescription(event.target.value); setFormError(''); }} placeholder="例如：东门电梯上楼右转，营业时间 10:00—21:00" className="mt-2 w-full resize-none rounded-xl border border-slate-200 px-3 py-2.5 text-sm leading-6 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100" /></label>
					<div className="grid grid-cols-2 gap-3">
						<label className="block"><span className="text-sm font-bold text-slate-700">纬度</span><input aria-label="位置纬度" type="number" step="0.000001" value={latitudeText} onChange={/* updateLocationLatitude 更新 WGS84 纬度文本。 */ event => { setLatitudeText(event.target.value); setFormError(''); }} placeholder="22.540503" className="mt-2 h-11 w-full rounded-xl border border-slate-200 px-3 text-sm tabular-nums outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100" /></label>
						<label className="block"><span className="text-sm font-bold text-slate-700">经度</span><input aria-label="位置经度" type="number" step="0.000001" value={longitudeText} onChange={/* updateLocationLongitude 更新 WGS84 经度文本。 */ event => { setLongitudeText(event.target.value); setFormError(''); }} placeholder="113.934528" className="mt-2 h-11 w-full rounded-xl border border-slate-200 px-3 text-sm tabular-nums outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100" /></label>
					</div>
				</> : <div className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3"><AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" /><div><div className="text-sm font-black text-amber-900">确认发送给当前会话</div><div className="mt-1 text-xs leading-5 text-amber-700">平台发送后会留下真实聊天记录，请确认标题、说明和坐标无误。</div></div></div>}
				<div className="rounded-2xl border border-sky-200 bg-sky-50/70 p-4">
					<div className="flex gap-3"><div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-sky-500 text-white"><MapPinned className="h-5 w-5" /></div><div className="min-w-0"><div className="truncate font-black text-slate-900">{title.trim() || '位置卡片标题'}</div><div className="mt-1 whitespace-pre-wrap text-sm leading-5 text-slate-600">{description.trim() || '地址或到店说明'}</div>{coordinateValid && <div className="mt-2 font-mono text-xs tabular-nums text-slate-400">{latitude.toFixed(6)}, {longitude.toFixed(6)}</div>}</div></div>
				</div>
				{formError && <p role="alert" className="rounded-xl bg-red-50 px-3 py-2 text-sm font-medium text-red-700">{formError}</p>}
			</div>
			<footer className="flex justify-end gap-2 border-t border-slate-200 px-6 py-4"><button type="button" disabled={sending} onClick={stage === 'edit' ? onClose : /* returnLocationCardEdit 返回表单且不发送。 */ () => setStage('edit')} className="h-10 rounded-xl px-4 text-sm font-bold text-slate-600 hover:bg-slate-100 disabled:opacity-40">{stage === 'edit' ? '取消' : '返回修改'}</button><button type="button" disabled={sending || !formValid} onClick={/* confirmLocationCard 先进入确认页，再执行最终平台发送。 */ () => void advanceOrSubmitLocationCard()} className="flex h-10 min-w-32 items-center justify-center gap-2 rounded-xl bg-sky-500 px-4 text-sm font-black text-white hover:bg-sky-600 disabled:cursor-not-allowed disabled:opacity-40">{sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}{sending ? '正在发送…' : stage === 'edit' ? '下一步' : '确认发送位置卡片'}</button></footer>
		</section>
	</div>;
};
