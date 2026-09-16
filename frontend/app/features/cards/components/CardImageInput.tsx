import { CloudUpload,FileImage,Link2,RefreshCw,X } from 'lucide-react';
import React,{ useEffect,useId,useState,type DragEvent } from 'react';
import type { CardImageSourceMode } from '../types';

// maxCardImageBytes 与服务端单图上限一致，用于在文件离开浏览器前提供即时反馈。
const maxCardImageBytes = 10 * 1024 * 1024;
// acceptedCardImageTypes 是拖拽输入允许的浏览器 MIME 集合，服务端仍会根据真实魔数复核。
const acceptedCardImageTypes = new Set(['image/png','image/jpeg','image/webp']);

// CardImageInputProps 描述图片卡密拖拽上传和远程 URL 兼容入口的受控状态。
export interface CardImageInputProps {
  // mode 表示当前使用本地上传还是兼容远程 URL。
  mode: CardImageSourceMode;
  // file 是等待随卡券表单共同提交的单张本地图片。
  file: File | null;
  // remoteURL 是 URL 模式下保存的公网图片地址。
  remoteURL: string;
  // currentPreviewURL 是编辑既有受管图片时使用的鉴权预览地址。
  currentPreviewURL?: string;
  // onModeChange 切换上传和 URL 模式，父表单负责保留各自草稿。
  onModeChange: (mode: CardImageSourceMode) => void;
  // onFileChange 接收已经通过浏览器侧类型和体积校验的文件或清空操作。
  onFileChange: (file: File | null) => void;
  // onRemoteURLChange 更新兼容远程 URL 草稿。
  onRemoteURLChange: (value: string) => void;
}

// formatCardImageSize 把图片字节数格式化为便于用户核对的 KB 或 MB。
const formatCardImageSize = (bytes: number): string => (
  bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`
);

// CardImageInput 渲染单图拖拽／选择、即时预览、替换和折叠 URL 兼容入口。
export const CardImageInput: React.FC<CardImageInputProps> = ({
  mode,file,remoteURL,currentPreviewURL,onModeChange,onFileChange,onRemoteURLChange,
}) => {
  // inputID 是隐藏文件输入与可见拖拽标签之间的稳定可访问关联。
  const inputID = useId();
  // isDragging 和 setIsDragging 保存文件悬停在投放区时的短暂视觉状态。
  const [isDragging,setIsDragging] = useState(false);
  // validationError 和 setValidationError 保存最近一次本地文件校验提示，不持久化到服务端。
  const [validationError,setValidationError] = useState('');
  // localPreviewURL 和 setLocalPreviewURL 保存当前 File 的临时对象地址，文件变化或卸载时必须释放。
  const [localPreviewURL,setLocalPreviewURL] = useState('');
  // previewBroken 和 setPreviewBroken 表示当前预览地址加载失败，避免用第三方占位图产生额外请求。
  const [previewBroken,setPreviewBroken] = useState(false);

  // 当前 effect 只同步浏览器 File 对象到临时预览 URL；cleanup 负责撤销旧对象地址。
	useEffect(/* localPreviewEffect 为当前 File 创建并管理浏览器临时预览地址。 */ () => {
    if (!file) {
      setLocalPreviewURL('');
      return undefined;
    }
    // objectURL 是浏览器为当前文件创建的临时预览地址，不会写入卡券或上传到外部服务。
    const objectURL = URL.createObjectURL(file);
    setLocalPreviewURL(objectURL);
	return /* localPreviewCleanup 文件变化或卸载时释放临时对象地址。 */ () => URL.revokeObjectURL(objectURL);
  }, [file]);

  // previewSource 是当前模式下优先展示的新文件、本地受管图片或远程 URL。
  const previewSource = mode === 'upload' ? (localPreviewURL || currentPreviewURL || '') : remoteURL.trim();
  // 当前 effect 在预览来源变化时清除旧加载错误，允许替换图片后重新显示。
	useEffect(/* previewResetEffect 在来源变化时允许新图片重新尝试加载。 */ () => setPreviewBroken(false), [previewSource]);

  // selectFile 校验用户拖入或选择的第一张图片，并把结果交给父表单。
  const selectFile = (selectedFile: File | undefined) => {
    if (!selectedFile) return;
    if (!acceptedCardImageTypes.has(selectedFile.type)) {
      setValidationError('只支持 PNG、JPEG 或 WebP 图片');
      onFileChange(null);
      return;
    }
    if (selectedFile.size <= 0 || selectedFile.size > maxCardImageBytes) {
      setValidationError('图片不能为空且不能超过 10 MiB');
      onFileChange(null);
      return;
    }
    setValidationError('');
    onFileChange(selectedFile);
  };

  // handleDrop 接收拖入文件并阻止浏览器把图片作为新页面打开。
  const handleDrop = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setIsDragging(false);
    selectFile(event.dataTransfer.files[0]);
  };

  return (
    <div className="space-y-3">
      {mode === 'upload' ? (
        <>
          <label
            htmlFor={inputID}
            onDragEnter={/* 当前回调显示图片已经进入可投放区域。 */ event => { event.preventDefault(); setIsDragging(true); }}
            onDragOver={/* 当前回调允许浏览器把文件投放到标签区域。 */ event => event.preventDefault()}
            onDragLeave={/* 当前回调在文件离开投放区时恢复默认边框。 */ event => { event.preventDefault(); setIsDragging(false); }}
            onDrop={handleDrop}
            className={`flex min-h-40 cursor-pointer flex-col items-center justify-center rounded-2xl border-2 border-dashed px-5 py-6 text-center transition-colors ${isDragging ? 'border-brand bg-blue-50' : 'border-gray-300 bg-white hover:border-brand hover:bg-blue-50/50'}`}
          >
            <input
              id={inputID}
              type="file"
              accept="image/png,image/jpeg,image/webp"
              className="sr-only"
              onChange={/* 当前回调读取文件选择器中的第一张图片。 */ event => { selectFile(event.target.files?.[0]); event.currentTarget.value = ''; }}
            />
            <CloudUpload className="mb-3 h-9 w-9 text-brand" />
            <p className="font-bold text-gray-800">拖动图片到这里，或点击选择</p>
            <p className="mt-1 text-xs text-gray-500">支持 PNG、JPEG、WebP，单张不超过 10 MiB</p>
          </label>
          {validationError && <p role="alert" className="text-sm font-medium text-red-600">{validationError}</p>}
          {file && (
            <div className="flex items-center gap-3 rounded-xl border border-blue-100 bg-blue-50 px-4 py-3">
              <FileImage className="h-5 w-5 shrink-0 text-brand" />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-bold text-gray-800">{file.name}</p>
                <p className="text-xs text-gray-500">{formatCardImageSize(file.size)} · 将在保存卡密时自动上传并引用</p>
              </div>
              <button type="button" onClick={/* 当前回调清除本次选择但不删除既有受管图片。 */ () => onFileChange(null)} className="rounded-lg p-2 text-gray-400 hover:bg-white hover:text-red-500" aria-label="清除已选图片">
                <X className="h-4 w-4" />
              </button>
            </div>
          )}
        </>
      ) : (
        <div className="space-y-2">
          <label className="block text-sm font-bold text-gray-700">图片 URL</label>
          <div className="relative">
            <Link2 className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
            <input
              type="url"
              value={remoteURL}
              onChange={/* 当前回调更新兼容的公网图片地址草稿。 */ event => onRemoteURLChange(event.target.value)}
              className="w-full rounded-xl ios-input py-3 pl-11 pr-4 font-mono text-sm"
              placeholder="https://example.com/card.png"
            />
          </div>
          <p className="text-xs text-gray-500">兼容旧流程；发货时仍会安全下载并上传到实际闲鱼账号</p>
        </div>
      )}

      {previewSource && !previewBroken && (
        <div className="overflow-hidden rounded-xl border border-gray-200 bg-white p-3">
          <p className="mb-2 text-xs font-bold text-gray-500">图片预览</p>
          <img src={previewSource} alt="卡密图片预览" className="max-h-52 w-full rounded-lg object-contain" onError={/* 当前回调只显示本地错误状态，不加载第三方占位图。 */ () => setPreviewBroken(true)} />
        </div>
      )}
      {previewSource && previewBroken && <p role="alert" className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600">图片预览加载失败，请检查文件或 URL</p>}

      <button
        type="button"
        onClick={/* 当前回调在本地上传和兼容 URL 之间切换，不主动提交任何文件。 */ () => onModeChange(mode === 'upload' ? 'url' : 'upload')}
        className="inline-flex items-center gap-2 text-sm font-bold text-gray-500 transition-colors hover:text-brand"
      >
        {mode === 'upload' ? <Link2 className="h-4 w-4" /> : <RefreshCw className="h-4 w-4" />}
        {mode === 'upload' ? '高级：使用图片 URL' : '改用拖拽上传'}
      </button>
    </div>
  );
};
