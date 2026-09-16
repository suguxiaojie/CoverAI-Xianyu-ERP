import { CheckCircle2,ExternalLink,Image as ImageIcon,Loader2,ReceiptText,TriangleAlert,X } from 'lucide-react';
import React from 'react';
import { completeMerchantRefundVerification,getMerchantRefundRefuseForm,getOrderRefundDetail,refuseMerchantRefund,startMerchantRefundVerification,submitOrderRefundAction,type MerchantRefundRefuseForm,type MerchantRefundVerification,type RefundAction,type RefundDetail } from '../api';
import { RefundProofImagePicker } from './RefundProofImagePicker';

// RefundDetailModalProps 描述退款详情弹窗的订单归属和关闭回调。
export interface RefundDetailModalProps {
	// accountID 是退款订单所属卖家账号。
	accountID: string;
	// orderID 是平台订单标识。
	orderID: string;
	// open 表示弹窗是否挂载并读取官方详情。
	open: boolean;
	// onClose 关闭弹窗并取消未完成的只读请求。
	onClose: () => void;
}

/** refundStatusLabel 把常见平台状态编码转换为简洁中文，未知文本原样展示。 */
const refundStatusLabel = (status: string | undefined, statusText: string | undefined): string => {
	if (String(statusText || '').trim()) return String(statusText).trim();
	// normalized 是去空白并统一大写后的平台状态。
	const normalized = String(status || '').trim().toUpperCase();
	// labels 保存第一阶段只读页面遇到的常见状态显示名。
	const labels: Record<string, string> = {
		'1': '等待卖家处理',
		WAIT_SELLER_AGREE: '等待卖家处理', WAIT_BUYER_RETURN_GOODS: '等待买家退货',
		WAIT_SELLER_CONFIRM_GOODS: '等待卖家确认收货', REFUND_SUCCESS: '退款成功', REFUNDED: '退款成功',
		REFUND_CLOSED: '退款关闭', CLOSED: '退款关闭',
	};
	return labels[normalized] || String(status || '').trim() || '处理中';
};

/** RefundDetailModal 展示官方退款信息，并只执行平台最新详情明确下发的普通同意／拒绝动作。 */
export const RefundDetailModal: React.FC<RefundDetailModalProps> = ({ accountID, orderID, open, onClose }) => {
	// detail 保存当前官方退款详情服务端数据。
	const [detail, setDetail] = React.useState<RefundDetail | null>(null);
	// loading 表示官方只读请求进行中。
	const [loading, setLoading] = React.useState(false);
	// error 保存当前可展示的详情读取错误。
	const [error, setError] = React.useState('');
	// selectedAction 保存用户准备二次确认的平台动态动作。
	const [selectedAction, setSelectedAction] = React.useState<RefundAction | null>(null);
	// submitting 表示真实退款动作正在提交，期间禁止重复点击和关闭误触。
	const [submitting, setSubmitting] = React.useState(false);
	// success 保存平台明确受理后的提示。
	const [success, setSuccess] = React.useState('');
	// officialNotice 保存需要在闲鱼原生认证流程继续处理的提示。
	const [officialNotice, setOfficialNotice] = React.useState('');
	// verification 保存服务端短期支付验证会话；authToken 不进入前端。
	const [verification, setVerification] = React.useState<MerchantRefundVerification | null>(null);
	// refuseForm 保存 Merchant 动态拒绝原因和要求。
	const [refuseForm, setRefuseForm] = React.useState<MerchantRefundRefuseForm | null>(null);
	// refuseReasonID 保存当前选择的平台 refuseReasonId。
	const [refuseReasonID, setRefuseReasonID] = React.useState('');
	// refuseDescription 保存最多二百字的拒绝补充说明。
	const [refuseDescription, setRefuseDescription] = React.useState('');
	// negotiationYuan 保存用户输入的可选协商元金额。
	const [negotiationYuan, setNegotiationYuan] = React.useState('');
	// refuseProofImages 保存最终确认前只存在浏览器内存的退款图片凭证。
	const [refuseProofImages, setRefuseProofImages] = React.useState<File[]>([]);
	// generationRef 隔离关闭、重开或订单切换后的晚到响应。
	const generationRef = React.useRef(0);
	// submitControllerRef 保存真实退款动作及成功后刷新使用的取消句柄。
	const submitControllerRef = React.useRef<AbortController | null>(null);
	// verificationFrameRef 指向支付宝跨域 iframe，只用于校验 postMessage source。
	const verificationFrameRef = React.useRef<HTMLIFrameElement | null>(null);

	// 当前 effect 在弹窗打开时读取官方退款详情，cleanup 取消旧请求并推进代次。
	React.useEffect(/* loadRefundDetail 在打开或订单切换时读取最新退款信息。 */ () => {
		generationRef.current += 1;
		// generation 是本次响应允许写入状态的代次。
		const generation = generationRef.current;
		// controller 只控制本次只读详情请求。
		const controller = new AbortController();
		if (!open || !accountID || !orderID) {
			setDetail(null);
			setError('');
			setLoading(false);
			setSelectedAction(null);
			setSuccess('');
			setOfficialNotice('');
			setVerification(null);
			setRefuseForm(null);
			setRefuseReasonID('');
			setRefuseDescription('');
			setNegotiationYuan('');
			setRefuseProofImages([]);
			return /* emptyDetailCleanup 释放未启动请求的控制器。 */ () => controller.abort();
		}
		setLoading(true);
		setDetail(null);
		setError('');
		setSelectedAction(null);
		setSuccess('');
		setOfficialNotice('');
		setVerification(null);
		setRefuseForm(null);
		setRefuseReasonID('');
		setRefuseDescription('');
		setNegotiationYuan('');
		setRefuseProofImages([]);
		void getOrderRefundDetail(accountID, orderID, { signal: controller.signal }).then(/* currentDetail 是当前官方退款详情。 */ currentDetail => {
			if (generation === generationRef.current) setDetail(currentDetail);
		}).catch(/* detailError 是当前详情读取错误。 */ detailError => {
			if (generation !== generationRef.current || controller.signal.aborted) return;
			setError(detailError instanceof Error ? detailError.message : '读取退款详情失败');
		}).finally(/* detailFinished 仅由当前代次清理加载态。 */ () => {
			if (generation === generationRef.current) setLoading(false);
		});
		return /* detailCleanup 取消旧请求并使晚到响应失效。 */ () => {
			controller.abort();
			generationRef.current += 1;
		};
	}, [accountID, open, orderID]);

	// 当前 effect 支持用 Escape 关闭只读详情弹窗。
	React.useEffect(/* registerEscapeClose 登记弹窗键盘关闭行为。 */ () => {
		if (!open) return undefined;
		// handleKeyDown 仅响应 Escape，其他按键留给弹窗内容。
		const handleKeyDown = (event: KeyboardEvent): void => {
			if (event.key === 'Escape' && !submitting) onClose();
		};
		window.addEventListener('keydown', handleKeyDown);
		return /* escapeCloseCleanup 移除本轮键盘监听。 */ () => window.removeEventListener('keydown', handleKeyDown);
	}, [onClose, open, submitting]);

	// 当前 effect 在组件卸载时取消仍未结束的真实退款动作。
	React.useEffect(/* registerRefundSubmitCleanup 只登记组件卸载清理。 */ () => /* refundSubmitCleanup 取消仍未结束的真实动作。 */ () => submitControllerRef.current?.abort(), []);

	// 当前 effect 只接受支付宝验证 iframe 本身、精确 origin 和 code=1000 的成功消息。
	React.useEffect(/* listenMerchantRefundVerification 监听跨域支付验证结果。 */ () => {
		if (!verification) return undefined;
		// handleVerificationMessage 校验来源后执行最终 Merchant 同意退款。
		const handleVerificationMessage = (event: MessageEvent): void => {
			if (event.origin !== verification.verify_origin || event.source !== verificationFrameRef.current?.contentWindow) return;
			// payload 是支付宝 iframe 的字符串或对象消息。
			let payload: unknown = event.data;
			if (typeof payload === 'string') {
				try { payload = JSON.parse(payload); } catch { return; }
			}
			if (!payload || typeof payload !== 'object') return;
			// message 是经过对象类型检查的验证消息。
			const message = payload as {
				/** type 区分高度更新和支付验证结果。 */ type?: string;
				/** msg 保留支付宝可能返回的附加消息。 */ msg?: unknown;
				/** result 保存支付验证结果。 */ result?: {
					/** code 为 1000 时才表示支付密码验证成功。 */ code?: string;
				};
			};
			if (message.type === 'UPDATE_HEIGHT') return;
			if (message.type !== 'verify' || message.result?.code !== '1000' || submitting) return;
			// controller 只控制验证成功后的最终 Merchant 退款请求。
			const controller = new AbortController();
			submitControllerRef.current = controller;
			setSubmitting(true);
			setError('');
			void completeMerchantRefundVerification(accountID, orderID, verification.session_id, { signal: controller.signal }).then(/* result 是最终 Merchant 退款结果。 */ result => {
				setSuccess(result.message || '退款成功');
				setVerification(null);
				return getOrderRefundDetail(accountID, orderID, { signal: controller.signal });
			}).then(/* refreshedDetail 是最终退款后的平台详情。 */ refreshedDetail => setDetail(refreshedDetail)).catch(/* verificationError 是支付验证完成后的最终请求错误。 */ verificationError => {
				if (!controller.signal.aborted) setError(verificationError instanceof Error ? verificationError.message : '退款失败');
			}).finally(/* verificationFinished 清理最终请求状态。 */ () => {
				if (submitControllerRef.current === controller) submitControllerRef.current = null;
				setSubmitting(false);
			});
		};
		window.addEventListener('message', handleVerificationMessage);
		return /* verificationMessageCleanup 移除支付验证监听。 */ () => window.removeEventListener('message', handleVerificationMessage);
	}, [accountID, orderID, submitting, verification]);

	if (!open) return null;
	// statusText 是平台状态编码的用户可见文本。
	const statusText = refundStatusLabel(detail?.status, detail?.status_text);
	// mediaCount 是买家图片和视频凭证总数。
	const mediaCount = (detail?.buyer_images.length || 0) + (detail?.buyer_videos.length || 0);
	// handleClose 取消当前请求并关闭弹窗；已发出的平台动作仍以服务端幂等状态为准。
	const handleClose = (): void => {
		if (submitting) return;
		submitControllerRef.current?.abort();
		onClose();
	};
	// handleSelectAction 进入应用内二次确认，不立即调用平台接口。
	const handleSelectAction = (action: RefundAction): void => {
		setError('');
		setSuccess('');
		setOfficialNotice('');
		setSelectedAction(action);
		setRefuseForm(null);
		setRefuseReasonID('');
		setRefuseDescription('');
		setNegotiationYuan('');
		setRefuseProofImages([]);
		if (action.mode === 'merchant_refuse') {
			setLoading(true);
			void getMerchantRefundRefuseForm(accountID, orderID).then(/* form 是平台初始动态拒绝表单。 */ form => setRefuseForm(form)).catch(/* formError 是拒绝表单读取错误。 */ formError => setError(formError instanceof Error ? formError.message : '读取拒绝原因失败')).finally(/* refuseFormFinished 清理加载状态。 */ () => setLoading(false));
		}
	};
	// handleSubmitAction 只在二次确认页提交最新动作，成功后立即重新读取平台详情。
	const handleSubmitAction = async (): Promise<void> => {
		if (!selectedAction || submitting) return;
		if (selectedAction.mode === 'official') {
			if (!detail?.official_url) {
				setError('闲鱼官方退款详情地址不可用，请稍后重新打开退款卡片');
				return;
			}
			window.open(detail.official_url, '_blank', 'noopener,noreferrer');
			setOfficialNotice('该操作需要闲鱼／支付宝支付密码验证。已打开闲鱼官方详情，请在官方流程完成后关闭并重新打开本卡片读取结果。');
			setSelectedAction(null);
			return;
		}
		if (selectedAction.mode === 'merchant_verify') {
			// controller 只控制本次验证页面创建请求。
			const controller = new AbortController();
			submitControllerRef.current = controller;
			setSubmitting(true);
			setError('');
			try {
				// currentVerification 是服务端创建且不暴露 authToken 的短期会话。
				const currentVerification = await startMerchantRefundVerification(accountID, orderID, { signal: controller.signal });
				setVerification(currentVerification);
				setSelectedAction(null);
			} catch (verificationError /* verificationError 是验证页面创建错误。 */) {
				if (!controller.signal.aborted) setError(verificationError instanceof Error ? verificationError.message : '创建支付验证失败');
			} finally {
				if (submitControllerRef.current === controller) submitControllerRef.current = null;
				setSubmitting(false);
			}
			return;
		}
		if (selectedAction.mode === 'merchant_refuse') {
			if (!refuseForm || !refuseReasonID) { setError('请选择拒绝原因'); return; }
			// negotiationCents 是用户可选协商金额换算后的整数分。
			const negotiationCents = negotiationYuan.trim() ? Math.round(Number(negotiationYuan) * 100) : 0;
			// controller 共同控制拒绝提交和成功后的详情刷新。
			const controller = new AbortController();
			submitControllerRef.current = controller;
			setSubmitting(true);
			setError('');
			try {
				// result 是平台明确返回的最终拒绝结果。
				const result = await refuseMerchantRefund(accountID, orderID, refuseReasonID, refuseDescription, negotiationCents, refuseProofImages, { signal: controller.signal, timeoutMs: 100_000 });
				setSuccess(result.message || '已拒绝退款申请');
				setSelectedAction(null);
				setRefuseForm(null);
				setRefuseProofImages([]);
				setDetail(await getOrderRefundDetail(accountID, orderID, { signal: controller.signal }));
			} catch (refuseError /* refuseError 是最终拒绝提交或详情刷新错误。 */) {
				if (!controller.signal.aborted) setError(refuseError instanceof Error ? refuseError.message : '拒绝退款失败');
			} finally {
				if (submitControllerRef.current === controller) submitControllerRef.current = null;
				setSubmitting(false);
			}
			return;
		}
		// controller 共同控制本次真实动作和成功后的只读刷新。
		const controller = new AbortController();
		submitControllerRef.current = controller;
		// generation 是动作返回时必须仍匹配的弹窗代次。
		const generation = generationRef.current;
		setSubmitting(true);
		setError('');
		setSuccess('');
		try {
			// result 是平台明确受理的退款处理结果。
			const result = await submitOrderRefundAction(accountID, orderID, selectedAction.code, { signal: controller.signal });
			if (generation !== generationRef.current) return;
			setSuccess(result.message || (selectedAction.kind === 'agree' ? '已同意退款申请' : '已拒绝退款申请'));
			setSelectedAction(null);
			try {
				// refreshedDetail 是动作成功后重新读取的平台最新状态和可用动作。
				const refreshedDetail = await getOrderRefundDetail(accountID, orderID, { signal: controller.signal });
				if (generation !== generationRef.current) return;
				setDetail(refreshedDetail);
			} catch (refreshError /* refreshError 不改变平台动作已经成功的事实。 */) {
				if (generation !== generationRef.current || controller.signal.aborted) return;
				setError(refreshError instanceof Error ? `退款已处理，但最新状态刷新失败：${refreshError.message}` : '退款已处理，但最新状态刷新失败');
			}
		} catch (actionError /* actionError 是真实退款动作或成功后刷新错误。 */) {
			if (generation !== generationRef.current || controller.signal.aborted) return;
			setError(actionError instanceof Error ? actionError.message : '退款处理失败');
		} finally {
			if (generation === generationRef.current) setSubmitting(false);
			if (submitControllerRef.current === controller) submitControllerRef.current = null;
		}
	};
	return <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4 backdrop-blur-[2px]" role="presentation">
		<section role="dialog" aria-modal="true" aria-label="退款申请详情" className="max-h-[calc(100vh-2rem)] w-full max-w-[34rem] overflow-hidden rounded-[20px] bg-white shadow-modal">
			<header className="flex items-center justify-between border-b border-slate-200 px-6 py-5">
				<div className="min-w-0"><h3 className="text-xl font-black leading-7 text-slate-950">退款申请详情</h3><p className="mt-1 truncate text-xs font-medium tabular-nums text-slate-500">订单 {orderID}</p></div>
				<button type="button" aria-label="关闭退款详情" onClick={handleClose} disabled={submitting} className="flex h-9 w-9 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 disabled:opacity-40"><X className="h-4 w-4" /></button>
			</header>
			<div className="max-h-[calc(100vh-7rem)] overflow-y-auto p-6">
				{verification && <div className="space-y-4"><div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">支付密码由支付宝安全页面直接接收，ERP 不读取、不保存。验证成功后会自动提交最终 Merchant 退款。</div><iframe ref={verificationFrameRef} src={verification.verify_url} title="支付宝退款验证" className="h-[260px] w-full rounded-2xl border border-slate-200 bg-white" /></div>}
				{loading && <div className="flex items-center justify-center gap-2 py-12 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />正在读取闲鱼退款详情</div>}
				{error && <div role="alert" className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4"><TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" /><div><p className="text-sm font-black text-slate-900">{success ? '退款已处理，状态刷新未完成' : selectedAction ? '退款操作未完成' : '退款详情暂时读取失败'}</p><p className="mt-1 text-xs leading-5 text-slate-600">{error}</p></div></div>}
				{success && <div role="status" className="mb-5 flex gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-4"><CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" /><div><p className="text-sm font-black text-emerald-900">退款申请已处理</p><p className="mt-1 text-xs leading-5 text-emerald-700">{success}</p></div></div>}
				{officialNotice && <div role="status" className="mb-5 flex gap-3 rounded-2xl border border-sky-200 bg-sky-50 px-4 py-4"><ExternalLink className="mt-0.5 h-5 w-5 shrink-0 text-sky-600" /><div><p className="text-sm font-black text-sky-900">需要闲鱼官方验证</p><p className="mt-1 text-xs leading-5 text-sky-700">{officialNotice}</p></div></div>}
				{detail && <div className="space-y-5">
					<div className="grid grid-cols-2 gap-3 rounded-2xl border border-slate-200 bg-slate-50/70 p-4">
						<div><p className="text-xs font-semibold text-slate-500">退款金额</p><p className="mt-1 text-xl font-black tabular-nums text-slate-950">{detail.amount ? `¥${detail.amount.replace(/^[¥￥]/, '')}` : '--'}</p></div>
						<div><p className="text-xs font-semibold text-slate-500">当前状态</p><p className="mt-1 text-sm font-black text-amber-700">{statusText}</p></div>
					</div>
					<dl className="overflow-hidden rounded-2xl border border-slate-200">
						<div className="grid grid-cols-[6rem_minmax(0,1fr)] gap-3 border-b border-slate-100 px-4 py-3"><dt className="text-sm font-semibold text-slate-500">退款类型</dt><dd className="text-sm font-bold text-slate-900">{detail.type || '--'}</dd></div>
						<div className="grid grid-cols-[6rem_minmax(0,1fr)] gap-3 border-b border-slate-100 px-4 py-3"><dt className="text-sm font-semibold text-slate-500">退款原因</dt><dd className="text-sm font-bold leading-5 text-slate-900">{detail.reason || '--'}</dd></div>
						<div className="grid grid-cols-[6rem_minmax(0,1fr)] gap-3 border-b border-slate-100 px-4 py-3"><dt className="text-sm font-semibold text-slate-500">买家说明</dt><dd className="whitespace-pre-wrap break-words text-sm font-medium leading-5 text-slate-700">{detail.buyer_description || '买家未补充说明'}</dd></div>
						<div className="grid grid-cols-[6rem_minmax(0,1fr)] gap-3 px-4 py-3"><dt className="text-sm font-semibold text-slate-500">申请时间</dt><dd className="text-sm font-medium tabular-nums text-slate-700">{detail.apply_time || '--'}</dd></div>
					</dl>
					{mediaCount > 0 && <div><div className="mb-2 flex items-center gap-2 text-sm font-black text-slate-900"><ImageIcon className="h-4 w-4" />买家凭证（{mediaCount}）</div><div className="grid grid-cols-3 gap-2">
						{detail.buyer_images.map(/* imageURL 是当前买家图片凭证地址。 */ (imageURL, imageIndex) => <a key={`${imageURL}-${imageIndex}`} href={imageURL} target="_blank" rel="noreferrer" className="block aspect-square overflow-hidden rounded-xl border border-slate-200 bg-slate-100"><img src={imageURL} alt={`退款凭证 ${imageIndex + 1}`} className="h-full w-full object-cover" /></a>)}
						{detail.buyer_videos.map(/* videoURL 是当前买家视频凭证地址。 */ (videoURL, videoIndex) => <a key={`${videoURL}-${videoIndex}`} href={videoURL} target="_blank" rel="noreferrer" className="flex aspect-square items-center justify-center rounded-xl border border-slate-200 bg-slate-100 text-xs font-bold text-slate-600">视频 {videoIndex + 1}</a>)}
					</div></div>}
					{selectedAction && <div className={`rounded-2xl border px-4 py-4 ${selectedAction.kind === 'agree' ? 'border-emerald-200 bg-emerald-50' : 'border-red-200 bg-red-50'}`}>
						<div className="flex gap-3"><TriangleAlert className={`mt-0.5 h-5 w-5 shrink-0 ${selectedAction.kind === 'agree' ? 'text-emerald-600' : 'text-red-600'}`} /><div><p className="text-sm font-black text-slate-950">{selectedAction.confirm_title || (selectedAction.kind === 'agree' ? '确认同意退款' : '确认拒绝退款')}</p><p className="mt-1 text-xs leading-5 text-slate-600">{selectedAction.confirm_description || (selectedAction.mode === 'official' ? '该操作需要闲鱼／支付宝支付密码验证，下一步只会打开官方流程，ERP 不保存或代填支付密码。' : selectedAction.kind === 'agree' ? `确认后将按闲鱼流程退还 ¥${detail.amount || '--'}，操作通常不可撤销。` : '确认后将向闲鱼提交拒绝退款，买家仍可能申请平台介入。')}</p></div></div>
					</div>}
					{selectedAction?.mode === 'merchant_refuse' && refuseForm && <div className="space-y-4 rounded-2xl border border-red-100 bg-white p-4"><label className="block text-sm font-bold text-slate-700">拒绝原因<select value={refuseReasonID} onChange={/* changeRefundReason 重新读取所选原因要求。 */ event => {
						// reasonID 是用户刚选择的平台动态原因。
						const reasonID = event.target.value;
						setRefuseReasonID(reasonID);
						if (reasonID) void getMerchantRefundRefuseForm(accountID, orderID, reasonID).then(setRefuseForm).catch(/* reasonRenderError 显示原因二次 render 错误。 */ reasonRenderError => setError(reasonRenderError instanceof Error ? reasonRenderError.message : '读取原因要求失败'));
					}} className="mt-2 h-11 w-full rounded-xl border border-slate-200 px-3"><option value="">请选择</option>{refuseForm.reasons.map(/* reason 是平台动态拒绝原因。 */ reason => <option key={reason.id} value={reason.id} disabled={reason.requires_app}>{reason.name}{reason.requires_app ? '（需手机 App）' : ''}</option>)}</select></label>{refuseReasonID && <label className="block text-sm font-bold text-slate-700">补充说明<textarea value={refuseDescription} onChange={/* changeRefuseDescription 保存本地说明。 */ event => setRefuseDescription(event.target.value)} maxLength={200} rows={3} placeholder={refuseForm.proof_placeholder || '选填'} className="mt-2 w-full rounded-xl border border-slate-200 px-3 py-2" /></label>}{refuseReasonID && <RefundProofImagePicker required={refuseForm.proof_required} disabled={submitting} onFilesChange={setRefuseProofImages} onError={setError} />}{refuseForm.proof_required && refuseProofImages.length === 0 && <p className="text-xs font-bold text-red-600">该原因必须上传至少 1 张图片凭证。</p>}{refuseForm.negotiation_enabled && <label className="block text-sm font-bold text-slate-700">协商金额（元）<input value={negotiationYuan} onChange={/* changeNegotiationAmount 保存本地协商元金额。 */ event => setNegotiationYuan(event.target.value)} inputMode="decimal" className="mt-2 h-11 w-full rounded-xl border border-slate-200 px-3" /></label>}</div>}
					{!verification && !selectedAction && !success && !officialNotice && detail.seller && detail.actions.length > 0 && <div className="grid grid-cols-2 gap-3">
						{detail.actions.map(/* action 是平台当前允许卖家执行的普通退款动作。 */ action => <button key={action.code} type="button" onClick={/* selectRefundAction 只进入应用内二次确认。 */ () => handleSelectAction(action)} className={`h-11 rounded-xl px-4 text-sm font-black ${action.kind === 'agree' ? 'bg-emerald-600 text-white hover:bg-emerald-700' : 'border border-red-200 bg-white text-red-600 hover:bg-red-50'}`}>{action.name}</button>)}
					</div>}
					{!selectedAction && !success && !officialNotice && detail.seller && detail.actions.length === 0 && <p className="text-xs leading-5 text-slate-400">当前退款状态没有可在 ERP 中安全执行的普通动作；复杂退货、凭证或支付密码流程请使用闲鱼官方详情。</p>}
				</div>}
				<div className="mt-6 flex justify-end gap-2">
					{selectedAction ? <React.Fragment><button type="button" onClick={/* backToRefundDetail 返回详情且不执行平台动作。 */ () => { if (!submitting) { setSelectedAction(null); setRefuseProofImages([]); } }} disabled={submitting} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100 disabled:opacity-40">返回</button><button type="button" onClick={/* confirmRefundAction 执行应用内二次确认后的真实动作或打开官方认证。 */ () => void handleSubmitAction()} disabled={submitting || selectedAction.mode === 'merchant_refuse' && (!refuseReasonID || Boolean(refuseForm?.proof_required && refuseProofImages.length === 0))} className={`flex h-11 min-w-40 items-center justify-center gap-2 rounded-xl px-5 text-sm font-black text-white disabled:cursor-not-allowed disabled:opacity-50 ${selectedAction.kind === 'agree' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-red-600 hover:bg-red-700'}`}>{submitting && <Loader2 className="h-4 w-4 animate-spin" />}{submitting ? '正在提交…' : selectedAction.mode === 'official' ? '前往闲鱼验证' : selectedAction.mode === 'merchant_verify' ? '开始支付验证' : selectedAction.kind === 'agree' ? '确认同意退款' : '确认拒绝退款'}</button></React.Fragment> : <React.Fragment>{!verification && detail?.official_url && <button type="button" onClick={/* openOfficialRefundDetail 只打开服务端固定白名单生成的闲鱼详情页。 */ () => window.open(detail.official_url, '_blank', 'noopener,noreferrer')} className="flex h-11 items-center gap-2 rounded-xl px-4 text-sm font-bold text-slate-600 hover:bg-slate-100"><ExternalLink className="h-4 w-4" />打开闲鱼官方详情</button>}<button type="button" onClick={handleClose} className="flex h-11 min-w-24 items-center justify-center gap-2 rounded-xl bg-slate-950 px-5 text-sm font-black text-white hover:bg-slate-800"><ReceiptText className="h-4 w-4" />关闭</button></React.Fragment>}
				</div>
			</div>
		</section>
	</div>;
};
