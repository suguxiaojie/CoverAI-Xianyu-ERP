// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,beforeEach,describe,expect,test,vi } from 'vitest';
import { shipOrderWithEvidence } from '../api';
import { ShipOrderModal } from './ShipOrderModal';

vi.mock('../api', /* chatApiMock 隔离真实 HTTP，只验证最终表单内容和交互代次。 */ () => ({
	shipOrderWithEvidence: vi.fn(),
}));

describe('ShipOrderModal', /* 当前测试组验证拖入图片、二次确认、结果反馈和本地校验。 */ () => {
	beforeEach(/* resetShipmentMocks 为每个用例恢复平台明确成功结果。 */ () => {
		vi.mocked(shipOrderWithEvidence).mockReset();
		vi.mocked(shipOrderWithEvidence).mockResolvedValue({ success: true, status: 'succeeded', message: '闲鱼已确认发货', order_id: '5127372002248048713' });
	});

	afterEach(/* cleanupShipmentDOM 清理弹窗和测试替身。 */ () => {
		cleanup();
		vi.restoreAllMocks();
	});

	test('拖入图片后经确认页提交描述和原始文件', /* 当前回调验证图片在最终按钮前不会上传。 */ async () => {
		// onSuccess 是平台明确成功后的卡片反馈替身。
		const onSuccess = vi.fn();
		render(<ShipOrderModal accountID="seller-account" orderID="5127372002248048713" open onClose={vi.fn()} onSuccess={onSuccess} />);
		// image 是小于 3MB 的本地 PNG 文件。
		const image = new File([new Uint8Array([137, 80, 78, 71])], 'proof.png', { type: 'image/png', lastModified: 1 });
		// dropZone 是支持点击、拖入和粘贴的本地选择区域。
		const dropZone = screen.getByRole('button', { name: '添加发货凭证' });
		fireEvent.drop(dropZone, { dataTransfer: { files: [image] } });
		expect(screen.getByText('1 / 3')).toBeTruthy();
		fireEvent.change(screen.getByPlaceholderText('请提供相关交易凭证'), { target: { value: '在线交付完成' } });
		expect(shipOrderWithEvidence).not.toHaveBeenCalled();
		fireEvent.click(screen.getByRole('button', { name: '下一步' }));
		expect(screen.getByRole('dialog', { name: '确认发货' })).toBeTruthy();
		expect(screen.getByText('在线交付完成')).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: '确认无需寄件发货' }));
		await waitFor(/* shipmentCompleted 等待异步 API 完成并进入成功页。 */ () => expect(screen.getByRole('dialog', { name: '发货成功' })).toBeTruthy());
		expect(shipOrderWithEvidence).toHaveBeenCalledWith('seller-account', '5127372002248048713', '在线交付完成', [image], expect.objectContaining({ signal: expect.any(AbortSignal) }));
		expect(onSuccess).toHaveBeenCalledWith('闲鱼已确认发货');
	});

	test('拒绝不受支持或达到 3MB 的图片', /* 当前回调验证无效文件不会进入确认摘要。 */ () => {
		render(<ShipOrderModal accountID="seller-account" orderID="5127372002248048713" open onClose={vi.fn()} />);
		// invalidType 是官方不接受的 GIF。
		const invalidType = new File(['gif'], 'proof.gif', { type: 'image/gif' });
		// oversized 是达到 3MB 边界、因必须严格小于而被拒绝的 PNG。
		const oversized = new File([new Uint8Array(3 * 1024 * 1024)], 'large.png', { type: 'image/png' });
		// dropZone 是本地文件拖放区域。
		const dropZone = screen.getByRole('button', { name: '添加发货凭证' });
		fireEvent.drop(dropZone, { dataTransfer: { files: [invalidType, oversized] } });
		expect(screen.getByRole('alert').textContent).toContain('3MB');
		expect(screen.getByText('0 / 3')).toBeTruthy();
	});

	test('状态复核超时明确提示尚未执行发货', /* 当前回调验证只读预检查失败不会伪装成最终提交失败。 */ async () => {
		vi.mocked(shipOrderWithEvidence).mockRejectedValueOnce(new Error('订单状态读取超时，尚未执行发货，请稍后重试'));
		// onSuccess 是不应被超时分支调用的平台成功替身。
		const onSuccess = vi.fn();
		render(<ShipOrderModal accountID="seller-account" orderID="5127194847698062342" open onClose={vi.fn()} onSuccess={onSuccess} />);
		fireEvent.click(screen.getByRole('button', { name: '下一步' }));
		fireEvent.click(screen.getByRole('button', { name: '确认无需寄件发货' }));
		await waitFor(/* timeoutMessageAssertion 等待超时文案进入确认页。 */ () => expect(screen.getByRole('alert').textContent).toContain('尚未执行发货'));
		expect(onSuccess).not.toHaveBeenCalled();
	});
});
