import { FileImage,Loader2,X } from 'lucide-react';
import React from 'react';
import { createPortal } from 'react-dom';
import { getShipmentProof,type ShipmentProof } from '../api';

// ShipmentProofModalProps 描述订单页只读发货凭证弹窗的目标和关闭动作。
interface ShipmentProofModalProps {
	// orderID 是当前待读取凭证的平台订单号。
	orderID: string;
	// open 表示弹窗是否可见。
	open: boolean;
	// onClose 关闭弹窗并取消尚未完成的读取请求。
	onClose: () => void;
}

// ShipmentProofModal 展示 ERP 保存的发货描述和官方图片，不提供任何平台写操作。
export const ShipmentProofModal: React.FC<ShipmentProofModalProps> = ({ orderID, open, onClose }) => {
	// proof 保存当前订单已经读取的 ERP 发货凭证。
	const [proof, setProof] = React.useState<ShipmentProof | null>(null);
	// loading 表示只读凭证请求尚未完成。
	const [loading, setLoading] = React.useState(false);
	// error 保存只读请求的用户可见错误。
	const [error, setError] = React.useState('');

	React.useEffect(/* loadShipmentProof 在弹窗目标变化时取消旧请求并读取精确订单凭证。 */ () => {
		if (!open || !orderID) return undefined;
		// controller 拥有当前凭证请求的取消责任。
		const controller = new AbortController();
		setProof(null);
		setError('');
		setLoading(true);
		void getShipmentProof(orderID, { signal: controller.signal }).then(
			/* loadedProof 是当前精确订单返回的 ERP 发货凭证。 */ loadedProof => {
				if (!controller.signal.aborted && loadedProof.order_id === orderID) setProof(loadedProof);
			},
			/* loadError 是凭证缺失或本地读取失败结果。 */ loadError => {
				if (!controller.signal.aborted) setError(loadError instanceof Error ? loadError.message : '读取发货凭证失败');
			},
		).finally(/* proofLoadFinished 只在当前请求未取消时结束加载状态。 */ () => {
			if (!controller.signal.aborted) setLoading(false);
		});
		return /* shipmentProofCleanup 关闭或切换订单时取消旧读取。 */ () => controller.abort();
	}, [open, orderID]);

	if (!open) return null;
	return createPortal(<div className="modal-overlay-centered"><section role="dialog" aria-modal="true" aria-label="发货凭证" className="modal-container max-w-2xl">
		<header className="modal-header"><div className="flex w-full items-center justify-between"><div><h3 className="text-2xl font-extrabold text-gray-900">发货凭证</h3><p className="mt-1 font-mono text-xs text-gray-400">订单 {orderID}</p></div><button type="button" onClick={onClose} className="rounded-full bg-gray-100 p-2 text-gray-500 hover:bg-gray-200" aria-label="关闭发货凭证"><X className="h-5 w-5" /></button></div></header>
		<div className="modal-body space-y-5">
			{loading && <div className="flex items-center justify-center gap-2 py-16 text-sm font-bold text-gray-500"><Loader2 className="h-5 w-5 animate-spin" />正在读取 ERP 发货凭证</div>}
			{error && <div role="alert" className="rounded-xl bg-red-50 px-4 py-3 text-sm font-bold text-red-700">{error}</div>}
			{proof && <><dl className="overflow-hidden rounded-xl border border-gray-100 text-sm"><div className="grid grid-cols-[96px_1fr] border-b border-gray-100 px-4 py-3"><dt className="font-bold text-gray-400">发货来源</dt><dd className="font-bold text-gray-800">ERP 无需寄件</dd></div><div className="grid grid-cols-[96px_1fr] border-b border-gray-100 px-4 py-3"><dt className="font-bold text-gray-400">提交时间</dt><dd className="text-gray-700">{new Date(proof.submitted_at * 1000).toLocaleString('zh-CN')}</dd></div><div className="grid grid-cols-[96px_1fr] px-4 py-3"><dt className="font-bold text-gray-400">发货描述</dt><dd className="whitespace-pre-wrap break-words text-gray-700">{proof.trade_text || '未填写描述'}</dd></div></dl><div><div className="mb-3 flex items-center gap-2 text-sm font-black text-gray-900"><FileImage className="h-4 w-4 text-blue-600" />图片凭证 · {proof.image_urls.length} 张</div>{proof.image_urls.length > 0 ? <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">{proof.image_urls.map(/* imageURL 是当前 ERP 发货提交的官方图片地址。 */ imageURL => <a key={imageURL} href={imageURL} target="_blank" rel="noopener noreferrer" className="overflow-hidden rounded-xl border border-gray-100 bg-gray-50"><img src={imageURL} alt="发货凭证" className="aspect-square w-full object-cover" /></a>)}</div> : <div className="rounded-xl bg-gray-50 px-4 py-8 text-center text-sm text-gray-400">本次 ERP 发货未上传图片凭证</div>}</div></>}
		</div>
		<footer className="modal-footer"><button type="button" onClick={onClose} className="w-full rounded-xl bg-gray-950 px-5 py-3 font-bold text-white">关闭</button></footer>
	</section></div>, document.body);
};
