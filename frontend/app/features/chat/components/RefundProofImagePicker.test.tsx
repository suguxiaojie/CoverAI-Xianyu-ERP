// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,expect,test,vi } from 'vitest';
import { RefundProofImagePicker } from './RefundProofImagePicker';

beforeEach(/* refundProofPreviewMocks 提供 jsdom 缺少的 object URL 能力。 */ () => {
	vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(/* createPreviewURLMock 返回稳定本地预览。 */ file => `blob:${file.name}`), revokeObjectURL: vi.fn() });
});

afterEach(/* refundProofPickerCleanup 清理组件、全局粘贴监听和 URL 替身。 */ () => {
	cleanup();
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
});

test('退款凭证支持点击选择、拖拽和粘贴三种入口', /* refundProofInputModesCase 验证图片只同步内存 File。 */ async () => {
	// onFilesChange 记录图片顺序。
	const onFilesChange = vi.fn();
	// onError 记录本地图片校验错误。
	const onError = vi.fn();
	render(<RefundProofImagePicker required onFilesChange={onFilesChange} onError={onError} />);
	// chosenImage 是点击选择加入的 PNG。
	const chosenImage = new File([new Uint8Array([137, 80, 78, 71])], 'chosen.png', { type: 'image/png', lastModified: 1 });
	// droppedImage 是拖拽加入的 JPEG。
	const droppedImage = new File([new Uint8Array([255, 216, 255])], 'dropped.jpg', { type: 'image/jpeg', lastModified: 2 });
	// pastedImage 是剪贴板粘贴加入的 PNG。
	const pastedImage = new File([new Uint8Array([137, 80, 78, 71])], 'pasted.png', { type: 'image/png', lastModified: 3 });
	fireEvent.change(screen.getByLabelText('选择退款凭证图片'), { target: { files: [chosenImage] } });
	fireEvent.drop(screen.getByRole('button', { name: '添加退款凭证' }), { dataTransfer: { files: [droppedImage] } });
	fireEvent.paste(window, { clipboardData: { files: [pastedImage], items: [] } });
	await waitFor(/* allRefundProofsSelected 等待三种事件的 React 状态依次合并。 */ () => expect(screen.getByText('3 / 3')).toBeTruthy());
	await waitFor(/* refundProofFilesForwarded 等待最终原始 File 顺序同步。 */ () => expect(onFilesChange).toHaveBeenLastCalledWith([chosenImage, droppedImage, pastedImage]));
	expect(onError).toHaveBeenCalledWith('');
	expect(screen.getByAltText('退款凭证 1')).toBeTruthy();
});
