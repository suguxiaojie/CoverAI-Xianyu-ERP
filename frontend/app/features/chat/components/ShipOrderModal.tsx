import { AlertTriangle,CheckCircle2,ChevronLeft,ChevronRight,Loader2,UploadCloud,X } from 'lucide-react';
import React from 'react';
import { shipOrderWithEvidence } from '../api';

/** shipmentImageMaxBytes 与闲鱼官方卖家工作台单图严格小于 3MB 的限制一致。 */
const shipmentImageMaxBytes = 3 * 1024 * 1024;
/** shipmentImageTypes 是官方当前实际接受的 PNG/JPEG MIME 白名单。 */
const shipmentImageTypes = new Set(['image/png', 'image/jpeg', 'image/jpg']);

/** SelectedShipmentImage 保存浏览器内存文件和仅供本地预览的 object URL。 */
interface SelectedShipmentImage {
	/** id 在同名文件之间提供稳定 React key。 */
	id: string;
	/** file 只在用户最终确认时加入 multipart 请求。 */
	file: File;
	/** previewURL 是页面本地预览地址，删除或卸载时必须释放。 */
	previewURL: string;
}

/** ShipOrderModalProps 描述付款卡片发货弹窗的账号、订单和完成回调。 */
export interface ShipOrderModalProps {
	/** accountID 是卖家付款卡片所属账号。 */
	accountID: string;
	/** orderID 是平台明确提供的数字订单号。 */
	orderID: string;
	/** open 控制弹窗是否挂载。 */
	open: boolean;
	/** onClose 在未提交或结果查看完毕后关闭弹窗。 */
	onClose: () => void;
	/** onSuccess 只在平台明确确认发货后更新原卡片的本地反馈。 */
	onSuccess?: (message: string) => void;
}

/** createPreviewURL 在浏览器支持 object URL 时生成本地预览；测试环境缺失时返回空值。 */
const createPreviewURL = (file: File): string => typeof URL.createObjectURL === 'function' ? URL.createObjectURL(file) : '';

/** releasePreviewURL 释放已生成的本地图片地址，避免连续选择图片导致内存泄漏。 */
const releasePreviewURL = (previewURL: string): void => {
	if (previewURL && typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(previewURL);
};

/** ShipOrderModal 提供无需寄件描述、三图拖拽／粘贴、二次确认和确定性结果页。 */
export const ShipOrderModal: React.FC<ShipOrderModalProps> = ({ accountID, orderID, open, onClose, onSuccess }) => {
	/** phase 是弹窗短暂 UI 状态，不保存服务端订单数据。 */
	const [phase, setPhase] = React.useState<'edit' | 'confirm' | 'success'>('edit');
	/** tradeText 和 setTradeText 保存最多 200 字的表单描述。 */
	const [tradeText, setTradeText] = React.useState('');
	/** selectedImages 和 setSelectedImages 保存尚未上传的浏览器内存图片。 */
	const [selectedImages, setSelectedImages] = React.useState<SelectedShipmentImage[]>([]);
	/** dragging 和 setDragging 表示文件正悬停在拖放区域。 */
	const [dragging, setDragging] = React.useState(false);
	/** submitting 和 setSubmitting 防止最终发货重复点击。 */
	const [submitting, setSubmitting] = React.useState(false);
	/** errorMessage 和 setErrorMessage 保存当前表单或请求的可见错误。 */
	const [errorMessage, setErrorMessage] = React.useState('');
	/** successMessage 和 setSuccessMessage 保存平台明确成功后的提示。 */
	const [successMessage, setSuccessMessage] = React.useState('');
	/** fileInputRef 指向隐藏文件输入，用于点击上传。 */
	const fileInputRef = React.useRef<HTMLInputElement>(null);
	/** requestRef 保存当前最终请求；关闭或切换订单时负责取消晚到响应。 */
	const requestRef = React.useRef<AbortController | null>(null);
	/** imagesRef 保存最新预览列表，卸载 cleanup 不依赖过期闭包。 */
	const imagesRef = React.useRef<SelectedShipmentImage[]>([]);

	React.useEffect(/* selectedImagesRefSync 让卸载清理读取最新预览地址。 */ () => {
		imagesRef.current = selectedImages;
	}, [selectedImages]);

	React.useEffect(/* resetShipmentModal 在订单或打开状态变化时重置短暂表单并取消旧请求。 */ () => {
		requestRef.current?.abort();
		requestRef.current = null;
		imagesRef.current.forEach(/* selectedImage 释放上一轮弹窗的本地预览 URL。 */ selectedImage => releasePreviewURL(selectedImage.previewURL));
		imagesRef.current = [];
		setSelectedImages([]);
		setTradeText('');
		setPhase('edit');
		setDragging(false);
		setSubmitting(false);
		setErrorMessage('');
		setSuccessMessage('');
	}, [accountID, open, orderID]);

	React.useEffect(/* cleanupShipmentModal 组件卸载时取消请求并释放所有本地预览。 */ () => () => {
		requestRef.current?.abort();
		imagesRef.current.forEach(/* selectedImage 是卸载时仍保留的本地预览。 */ selectedImage => releasePreviewURL(selectedImage.previewURL));
	}, []);

	/** addFiles 校验并追加用户点击、拖入或粘贴的文件，不触发网络上传。 */
	const addFiles = React.useCallback((incomingFiles: File[]) => {
		setErrorMessage('');
		setSelectedImages(/* currentImages 是追加前的本地图片列表。 */ currentImages => {
			/** nextImages 是本次校验通过后的新列表。 */
			const nextImages = [...currentImages];
			for (const incomingFile /* incomingFile 是当前待校验的浏览器文件。 */ of incomingFiles) {
				if (nextImages.length >= 3) {
					setErrorMessage('相关凭证最多上传 3 张');
					break;
				}
				if (!shipmentImageTypes.has(incomingFile.type.toLowerCase())) {
					setErrorMessage('相关凭证只支持 PNG 或 JPEG 图片');
					continue;
				}
				if (incomingFile.size <= 0 || incomingFile.size >= shipmentImageMaxBytes) {
					setErrorMessage('每张相关凭证必须小于 3MB');
					continue;
				}
				/** duplicate 表示同名、同大小和同修改时间文件已经加入，避免粘贴事件重复触发。 */
				const duplicate = nextImages.some(/* selectedImage 与当前新文件比较浏览器元数据。 */ selectedImage => selectedImage.file.name === incomingFile.name && selectedImage.file.size === incomingFile.size && selectedImage.file.lastModified === incomingFile.lastModified);
				if (duplicate) continue;
				/** selectedImage 是新增文件及其本地预览模型。 */
				const selectedImage: SelectedShipmentImage = {
					id: `${incomingFile.name}-${incomingFile.size}-${incomingFile.lastModified}-${nextImages.length}`,
					file: incomingFile,
					previewURL: createPreviewURL(incomingFile),
				};
				nextImages.push(selectedImage);
			}
			return nextImages;
		});
	}, []);

	/** removeImage 删除指定图片并立即释放其本地预览地址。 */
	const removeImage = React.useCallback((imageID: string) => {
		setSelectedImages(/* currentImages 是删除前的图片列表。 */ currentImages => currentImages.filter(/* selectedImage 保留非目标图片并释放目标地址。 */ selectedImage => {
			if (selectedImage.id !== imageID) return true;
			releasePreviewURL(selectedImage.previewURL);
			return false;
		}));
	}, []);

	/** moveImage 在三张凭证范围内调整最终提交顺序。 */
	const moveImage = React.useCallback((imageIndex: number, direction: -1 | 1) => {
		setSelectedImages(/* currentImages 是排序前的图片列表。 */ currentImages => {
			/** targetIndex 是图片移动后的目标位置。 */
			const targetIndex = imageIndex + direction;
			if (targetIndex < 0 || targetIndex >= currentImages.length) return currentImages;
			/** reorderedImages 是保持对象引用的浅复制排序结果。 */
			const reorderedImages = [...currentImages];
			[reorderedImages[imageIndex], reorderedImages[targetIndex]] = [reorderedImages[targetIndex], reorderedImages[imageIndex]];
			return reorderedImages;
		});
	}, []);

	/** closeModal 取消当前请求并关闭弹窗；提交中禁止关闭以避免误解最终结果。 */
	const closeModal = React.useCallback(/* closeShipmentModalCallback 负责取消请求和关闭当前模态。 */ () => {
		if (submitting) return;
		requestRef.current?.abort();
		onClose();
	}, [onClose, submitting]);

	/** openConfirm 校验描述后进入本地确认页，不调用任何平台接口。 */
	const openConfirm = React.useCallback(/* openShipmentConfirmCallback 只切换本地确认视图。 */ () => {
		if (Array.from(tradeText).length > 200) {
			setErrorMessage('相关描述不能超过 200 字');
			return;
		}
		setErrorMessage('');
		setPhase('confirm');
	}, [tradeText]);

	/** submitShipment 上传当前图片并执行一次最终发货；AbortController 阻止晚到结果覆盖新订单弹窗。 */
	const submitShipment = React.useCallback(async () => {
		if (submitting) return;
		/** controller 是本次最终请求的取消和代次边界。 */
		const controller = new AbortController();
		requestRef.current = controller;
		setSubmitting(true);
		setErrorMessage('');
		try {
			/** result 是平台明确成功后的服务端安全投影。 */
			const result = await shipOrderWithEvidence(accountID, orderID, tradeText, selectedImages.map(/* selectedImage 只提取原始 File，不提交本地预览 URL。 */ selectedImage => selectedImage.file), { signal: controller.signal, timeoutMs: 100_000 });
			if (requestRef.current !== controller) return;
			setSuccessMessage(result.message || '闲鱼已确认发货');
			setPhase('success');
			onSuccess?.(result.message || '闲鱼已确认发货');
		} catch (requestError /* requestError 是最终 HTTP 请求返回的可展示错误。 */) {
			if (requestRef.current !== controller || controller.signal.aborted) return;
			setErrorMessage(requestError instanceof Error ? requestError.message : '发货失败');
		} finally {
			if (requestRef.current === controller) {
				requestRef.current = null;
				setSubmitting(false);
			}
		}
	}, [accountID, onSuccess, orderID, selectedImages, submitting, tradeText]);

	if (!open) return null;

	return <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4 backdrop-blur-[2px]" role="presentation">
		<section role="dialog" aria-modal="true" aria-label={phase === 'confirm' ? '确认发货' : phase === 'success' ? '发货成功' : '立即发货'} className="flex max-h-[calc(100vh-2rem)] w-full max-w-2xl flex-col overflow-hidden rounded-[20px] bg-white shadow-modal">
			<header className="flex items-center justify-between border-b border-slate-200 px-6 py-5">
				<div className="min-w-0"><h3 className="text-xl font-black text-slate-950">{phase === 'confirm' ? '确认发货' : phase === 'success' ? '发货成功' : '立即发货'}</h3><p className="mt-1 truncate text-xs font-medium tabular-nums text-slate-500">订单 {orderID}</p></div>
				<button type="button" aria-label="关闭发货弹窗" onClick={closeModal} disabled={submitting} className="flex h-9 w-9 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 disabled:opacity-40"><X className="h-4 w-4" /></button>
			</header>
			{phase !== 'success' && <nav className="flex border-b border-slate-200 px-6" aria-label="发货方式">
				<button type="button" className="border-b-2 border-slate-950 px-4 py-3 text-sm font-black text-slate-950">无需寄件</button>
				<button type="button" disabled title="物流发货协议将在下一阶段接入" className="flex cursor-not-allowed items-center gap-2 px-4 py-3 text-sm font-bold text-slate-400">我已寄出<span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px]">尚未接入</span></button>
			</nav>}
			<div className="overflow-y-auto p-6">
				{phase === 'edit' && <div className="space-y-5">
					<div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">无需寄件会直接把闲鱼订单标记为已发货。描述和图片均为选填，提交前还会再次确认。</div>
					<label className="block"><span className="text-sm font-black text-slate-900">相关描述 <span className="font-medium text-slate-400">（选填）</span></span><textarea value={tradeText} onChange={/* updateTradeText 保存当前本地描述并清除旧错误。 */ event => { setTradeText(event.target.value); setErrorMessage(''); }} maxLength={200} rows={4} placeholder="请提供相关交易凭证" className="mt-2 w-full resize-none rounded-xl border border-slate-200 px-4 py-3 text-sm leading-6 outline-none transition focus:border-amber-400 focus:ring-4 focus:ring-amber-100" /><span className="mt-1 block text-right text-xs tabular-nums text-slate-400">{Array.from(tradeText).length} / 200</span></label>
					<div><div className="flex items-end justify-between"><div><p className="text-sm font-black text-slate-900">相关凭证 <span className="font-medium text-slate-400">（选填）</span></p><p className="mt-1 text-xs text-slate-500">PNG／JPEG，单张小于 3MB，最多 3 张</p></div><span className="text-xs font-bold tabular-nums text-slate-500">{selectedImages.length} / 3</span></div>
						<div role="button" tabIndex={0} aria-label="添加发货凭证" onClick={/* openFilePicker 打开系统图片选择器。 */ () => fileInputRef.current?.click()} onKeyDown={/* openFilePickerByKeyboard 支持键盘 Enter 或空格选择图片。 */ event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); fileInputRef.current?.click(); } }} onDragEnter={/* markDragging 阻止浏览器打开文件并进入拖放高亮。 */ event => { event.preventDefault(); setDragging(true); }} onDragOver={/* keepDragging 允许文件在当前区域触发 drop。 */ event => { event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; }} onDragLeave={/* clearDragging 在离开当前区域时移除高亮。 */ event => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragging(false); }} onDrop={/* addDroppedFiles 仅把拖入图片加入本地表单。 */ event => { event.preventDefault(); setDragging(false); addFiles(Array.from(event.dataTransfer.files)); }} onPaste={/* addPastedFiles 从剪贴板读取图片并保留在本地。 */ event => {
							/** pastedFiles 是剪贴板当前携带的图片文件，不读取普通文本。 */
							const pastedFiles = Array.from(event.clipboardData.files);
							if (pastedFiles.length) { event.preventDefault(); addFiles(pastedFiles); }
						}} className={`mt-3 flex min-h-32 cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed px-5 py-6 text-center transition ${dragging ? 'border-amber-400 bg-amber-50' : 'border-slate-200 bg-slate-50 hover:border-slate-300 hover:bg-slate-100'}`}>
							<UploadCloud className="h-7 w-7 text-slate-400" /><p className="mt-2 text-sm font-bold text-slate-700">点击、拖入或粘贴图片</p><p className="mt-1 text-xs text-slate-400">图片将在最终确认后才上传到闲鱼</p>
						</div>
						<input ref={fileInputRef} type="file" accept="image/png,image/jpeg" multiple className="hidden" onChange={/* addChosenFiles 加入系统选择的文件并允许再次选择同一文件。 */ event => { addFiles(Array.from(event.target.files || [])); event.target.value = ''; }} />
						{selectedImages.length > 0 && <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-3">{selectedImages.map(/* selectedImage 是当前预览图片；imageIndex 决定排序按钮可用性。 */ (selectedImage, imageIndex) => <div key={selectedImage.id} className="overflow-hidden rounded-xl border border-slate-200 bg-white"><div className="aspect-video bg-slate-100">{selectedImage.previewURL ? <img src={selectedImage.previewURL} alt={`凭证 ${imageIndex + 1}`} className="h-full w-full object-contain" /> : <div className="flex h-full items-center justify-center text-xs text-slate-400">{selectedImage.file.name}</div>}</div><div className="flex items-center justify-between gap-1 border-t border-slate-100 p-2"><span className="min-w-0 flex-1 truncate text-xs text-slate-500" title={selectedImage.file.name}>{selectedImage.file.name}</span><button type="button" aria-label={`凭证 ${imageIndex + 1} 向前`} disabled={imageIndex === 0} onClick={/* movePrevious 把当前图片向前移动一位。 */ event => { event.stopPropagation(); moveImage(imageIndex, -1); }} className="rounded p-1 text-slate-500 hover:bg-slate-100 disabled:opacity-25"><ChevronLeft className="h-3.5 w-3.5" /></button><button type="button" aria-label={`凭证 ${imageIndex + 1} 向后`} disabled={imageIndex === selectedImages.length - 1} onClick={/* moveNext 把当前图片向后移动一位。 */ event => { event.stopPropagation(); moveImage(imageIndex, 1); }} className="rounded p-1 text-slate-500 hover:bg-slate-100 disabled:opacity-25"><ChevronRight className="h-3.5 w-3.5" /></button><button type="button" aria-label={`删除凭证 ${imageIndex + 1}`} onClick={/* deleteSelectedImage 删除当前本地图片。 */ event => { event.stopPropagation(); removeImage(selectedImage.id); }} className="rounded p-1 text-red-500 hover:bg-red-50"><X className="h-3.5 w-3.5" /></button></div></div>)}</div>}
					</div>
				</div>}
				{phase === 'confirm' && <div className="space-y-5"><div className="flex gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4"><AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" /><div><p className="text-sm font-black text-slate-900">确认后将立即改变闲鱼订单状态</p><p className="mt-1 text-xs leading-5 text-slate-600">服务端会再次核对卖家账号、付款卡片和平台待发货状态；结果不明确时不会自动重试。</p></div></div><dl className="overflow-hidden rounded-2xl border border-slate-200 bg-white text-sm"><div className="flex gap-4 border-b border-slate-100 px-4 py-3"><dt className="w-20 shrink-0 font-semibold text-slate-500">订单</dt><dd className="break-all font-black tabular-nums text-slate-950">{orderID}</dd></div><div className="flex gap-4 border-b border-slate-100 px-4 py-3"><dt className="w-20 shrink-0 font-semibold text-slate-500">描述</dt><dd className="whitespace-pre-wrap break-words text-slate-700">{tradeText || '未填写'}</dd></div><div className="flex gap-4 px-4 py-3"><dt className="w-20 shrink-0 font-semibold text-slate-500">凭证</dt><dd className="font-bold text-slate-700">{selectedImages.length} 张</dd></div></dl></div>}
				{phase === 'success' && <div className="flex flex-col items-center py-6 text-center" role="status"><div className="flex h-16 w-16 items-center justify-center rounded-full bg-emerald-100 text-emerald-600"><CheckCircle2 className="h-8 w-8" /></div><p className="mt-4 text-lg font-black text-slate-950">闲鱼已确认发货</p><p className="mt-2 max-w-md text-sm leading-6 text-slate-500">{successMessage}</p></div>}
				{errorMessage && phase !== 'success' && <p role="alert" className="mt-5 rounded-2xl border border-red-100 bg-red-50 px-4 py-3 text-sm font-medium leading-5 text-red-700">{errorMessage}</p>}
			</div>
			<footer className="flex justify-end gap-2 border-t border-slate-200 bg-white px-6 py-4">
				{phase === 'edit' && <><button type="button" onClick={closeModal} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100">取消</button><button type="button" onClick={openConfirm} className="h-11 min-w-32 rounded-xl bg-amber-400 px-5 text-sm font-black text-slate-950 hover:bg-amber-500">下一步</button></>}
				{phase === 'confirm' && <><button type="button" onClick={/* backToEdit 返回表单但保留本地描述和图片。 */ () => { if (!submitting) { setPhase('edit'); setErrorMessage(''); } }} disabled={submitting} className="h-11 rounded-xl px-5 text-sm font-bold text-slate-600 hover:bg-slate-100 disabled:opacity-40">返回修改</button><button type="button" onClick={/* confirmShipment 执行唯一一次最终请求。 */ () => void submitShipment()} disabled={submitting} className="flex h-11 min-w-40 items-center justify-center gap-2 rounded-xl bg-amber-400 px-5 text-sm font-black text-slate-950 hover:bg-amber-500 disabled:cursor-not-allowed disabled:opacity-50">{submitting && <Loader2 className="h-4 w-4 animate-spin" />}{submitting ? '正在发货…' : '确认无需寄件发货'}</button></>}
				{phase === 'success' && <button type="button" onClick={closeModal} className="h-11 min-w-28 rounded-xl bg-slate-950 px-5 text-sm font-black text-white hover:bg-slate-800">完成</button>}
			</footer>
		</section>
	</div>;
};
