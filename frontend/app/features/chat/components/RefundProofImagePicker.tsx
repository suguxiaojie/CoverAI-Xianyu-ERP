import { UploadCloud,X } from 'lucide-react';
import React from 'react';

/** refundProofMaxBytes 与服务端及闲鱼当前单图严格小于 3MB 的门禁一致。 */
const refundProofMaxBytes = 3 * 1024 * 1024;
/** refundProofTypes 是退款凭证当前允许的 PNG／JPEG MIME。 */
const refundProofTypes = new Set(['image/png', 'image/jpeg', 'image/jpg']);

/** SelectedRefundProof 保存尚未上传的浏览器 File 和本地预览地址。 */
interface SelectedRefundProof {
	/** id 为同名文件提供稳定 React key。 */
	id: string;
	/** file 只在用户最终确认后进入 multipart 请求。 */
	file: File;
	/** previewURL 是仅在本地 WKWebView 使用的图片预览地址。 */
	previewURL: string;
}

/** RefundProofImagePickerProps 描述退款凭证选择器的必填态和文件回调。 */
export interface RefundProofImagePickerProps {
	/** required 表示当前闲鱼拒绝原因要求至少一张图片。 */
	required: boolean;
	/** disabled 在最终请求进行中禁止继续修改文件。 */
	disabled?: boolean;
	/** onFilesChange 把当前原始 File 顺序同步给退款弹窗。 */
	onFilesChange: (files: File[]) => void;
	/** onError 展示本地数量、类型或大小错误。 */
	onError: (message: string) => void;
}

/** createRefundProofPreview 创建本地 object URL；测试环境不支持时返回空值。 */
const createRefundProofPreview = (file: File): string => typeof URL.createObjectURL === 'function' ? URL.createObjectURL(file) : '';

/** releaseRefundProofPreview 释放不再使用的本地 object URL。 */
const releaseRefundProofPreview = (previewURL: string): void => {
	if (previewURL && typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(previewURL);
};

/** RefundProofImagePicker 支持点击选择、全局图片粘贴、拖拽、预览和删除。 */
export const RefundProofImagePicker: React.FC<RefundProofImagePickerProps> = ({ required, disabled = false, onFilesChange, onError }) => {
	/** selectedImages 和 setSelectedImages 保存尚未上传的本地退款凭证。 */
	const [selectedImages, setSelectedImages] = React.useState<SelectedRefundProof[]>([]);
	/** dragging 和 setDragging 表示文件正在拖放区域上方。 */
	const [dragging, setDragging] = React.useState(false);
	/** fileInputRef 指向点击选择使用的隐藏文件控件。 */
	const fileInputRef = React.useRef<HTMLInputElement>(null);
	/** imagesRef 让卸载清理始终读取最新 object URL。 */
	const imagesRef = React.useRef<SelectedRefundProof[]>([]);

	/** addFiles 校验并追加点击、粘贴或拖入的图片，不触发网络上传。 */
	const addFiles = React.useCallback(/* addRefundProofFilesCallback 合并当前图片输入。 */ (incomingFiles: File[]) => {
		if (disabled || incomingFiles.length === 0) return;
		onError('');
		setSelectedImages(/* currentImages 是追加前的本地图片列表。 */ currentImages => {
			/** nextImages 是本次校验后返回的新图片列表。 */
			const nextImages = [...currentImages];
			for (const incomingFile /* incomingFile 是当前待校验的浏览器 File。 */ of incomingFiles) {
				if (nextImages.length >= 3) {
					onError('退款凭证最多上传 3 张');
					break;
				}
				if (!refundProofTypes.has(incomingFile.type.toLowerCase())) {
					onError('退款凭证只支持 PNG 或 JPEG 图片');
					continue;
				}
				if (incomingFile.size <= 0 || incomingFile.size >= refundProofMaxBytes) {
					onError('每张退款凭证必须小于 3MB');
					continue;
				}
				/** duplicate 表示相同文件元数据已存在，避免一次粘贴事件重复加入。 */
				const duplicate = nextImages.some(/* selectedImage 是当前已加入的退款凭证。 */ selectedImage => selectedImage.file.name === incomingFile.name && selectedImage.file.size === incomingFile.size && selectedImage.file.lastModified === incomingFile.lastModified);
				if (duplicate) continue;
				/** selectedImage 是新增文件对应的本地预览模型。 */
				const selectedImage: SelectedRefundProof = {
					id: `${incomingFile.name}-${incomingFile.size}-${incomingFile.lastModified}-${nextImages.length}`,
					file: incomingFile,
					previewURL: createRefundProofPreview(incomingFile),
				};
				nextImages.push(selectedImage);
			}
			return nextImages;
		});
	}, [disabled, onError]);

	/** removeImage 删除指定凭证并立即释放本地预览。 */
	const removeImage = React.useCallback((imageID: string) => {
		if (disabled) return;
		setSelectedImages(/* currentImages 是删除前的退款凭证列表。 */ currentImages => currentImages.filter(/* selectedImage 保留非目标图片。 */ selectedImage => {
			if (selectedImage.id !== imageID) return true;
			releaseRefundProofPreview(selectedImage.previewURL);
			return false;
		}));
	}, [disabled]);

	React.useEffect(/* syncRefundProofFiles 把最新原始文件顺序同步给最终请求。 */ () => {
		imagesRef.current = selectedImages;
		onFilesChange(selectedImages.map(/* selectedImage 只提取原始 File。 */ selectedImage => selectedImage.file));
	}, [onFilesChange, selectedImages]);

	React.useEffect(/* listenRefundProofPaste 在拒绝表单打开期间接收 macOS 剪贴板图片。 */ () => {
		/** handlePaste 只消费剪贴板中的图片文件，不拦截普通文本粘贴。 */
		const handlePaste = (event: ClipboardEvent): void => {
			if (disabled) return;
			/** pastedFiles 合并 clipboard files 和 file items，并排除空项。 */
			const pastedFiles = Array.from(event.clipboardData?.files || []);
			if (pastedFiles.length === 0) {
				for (const item /* item 是剪贴板当前条目。 */ of Array.from(event.clipboardData?.items || [])) {
					/** pastedFile 是 file 类型条目转换出的浏览器 File。 */
					const pastedFile = item.kind === 'file' ? item.getAsFile() : null;
					if (pastedFile) pastedFiles.push(pastedFile);
				}
			}
			if (pastedFiles.some(/* pastedFile 判断剪贴板是否包含图片。 */ pastedFile => pastedFile.type.startsWith('image/'))) {
				event.preventDefault();
				addFiles(pastedFiles);
			}
		};
		window.addEventListener('paste', handlePaste);
		return /* refundProofPasteCleanup 移除当前表单的全局粘贴监听。 */ () => window.removeEventListener('paste', handlePaste);
	}, [addFiles, disabled]);

	React.useEffect(/* cleanupRefundProofPreviews 在组件卸载时释放所有本地预览。 */ () => () => {
		imagesRef.current.forEach(/* selectedImage 是卸载时仍存在的本地预览。 */ selectedImage => releaseRefundProofPreview(selectedImage.previewURL));
	}, []);

	return <div className="rounded-xl border border-slate-200 bg-slate-50/60 p-3">
		<div className="flex items-end justify-between gap-3"><div><p className="text-sm font-bold text-slate-700">图片凭证 {required ? <span className="text-red-600">（必填）</span> : <span className="font-medium text-slate-400">（选填）</span>}</p><p className="mt-1 text-xs text-slate-500">PNG／JPEG，单张小于 3MB，最多 3 张</p></div><span className="text-xs font-bold tabular-nums text-slate-500">{selectedImages.length} / 3</span></div>
		<div role="button" tabIndex={disabled ? -1 : 0} aria-label="添加退款凭证" onClick={/* openRefundProofPicker 打开 macOS 图片选择器。 */ () => { if (!disabled) fileInputRef.current?.click(); }} onKeyDown={/* openRefundProofPickerByKeyboard 支持键盘选择。 */ event => { if (!disabled && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); fileInputRef.current?.click(); } }} onDragEnter={/* startRefundProofDrag 进入拖放高亮。 */ event => { event.preventDefault(); if (!disabled) setDragging(true); }} onDragOver={/* allowRefundProofDrop 允许当前区域接收文件。 */ event => { event.preventDefault(); event.dataTransfer.dropEffect = disabled ? 'none' : 'copy'; }} onDragLeave={/* stopRefundProofDrag 离开区域时清除高亮。 */ event => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragging(false); }} onDrop={/* addDroppedRefundProofs 把拖入图片加入本地列表。 */ event => { event.preventDefault(); setDragging(false); addFiles(Array.from(event.dataTransfer.files)); }} className={`mt-3 flex min-h-24 flex-col items-center justify-center rounded-xl border-2 border-dashed px-4 py-4 text-center transition ${disabled ? 'cursor-not-allowed border-slate-200 opacity-50' : dragging ? 'cursor-copy border-red-400 bg-red-50' : 'cursor-pointer border-slate-300 bg-white hover:border-red-300 hover:bg-red-50/50'}`}>
			<UploadCloud className="h-6 w-6 text-slate-400" /><p className="mt-2 text-sm font-bold text-slate-700">点击选择、粘贴或拖拽图片</p><p className="mt-1 text-xs text-slate-400">最终确认前不会上传到闲鱼</p>
		</div>
		<input ref={fileInputRef} aria-label="选择退款凭证图片" type="file" accept="image/png,image/jpeg" multiple disabled={disabled} className="hidden" onChange={/* addChosenRefundProofs 加入系统选择文件并允许重复选择同名文件。 */ event => { addFiles(Array.from(event.target.files || [])); event.target.value = ''; }} />
		{selectedImages.length > 0 && <div className="mt-3 grid grid-cols-3 gap-2">{selectedImages.map(/* selectedImage 是当前预览凭证；imageIndex 用于可访问标签。 */ (selectedImage, imageIndex) => <div key={selectedImage.id} className="group relative aspect-square overflow-hidden rounded-lg border border-slate-200 bg-white">{selectedImage.previewURL ? <img src={selectedImage.previewURL} alt={`退款凭证 ${imageIndex + 1}`} className="h-full w-full object-contain" /> : <div className="flex h-full items-center justify-center p-2 text-center text-[10px] text-slate-400">{selectedImage.file.name}</div>}<button type="button" aria-label={`删除退款凭证 ${imageIndex + 1}`} disabled={disabled} onClick={/* deleteRefundProof 删除当前本地图片。 */ event => { event.stopPropagation(); removeImage(selectedImage.id); }} className="absolute right-1 top-1 rounded-full bg-slate-950/75 p-1 text-white shadow hover:bg-red-600 disabled:opacity-40"><X className="h-3.5 w-3.5" /></button></div>)}</div>}
	</div>;
};
