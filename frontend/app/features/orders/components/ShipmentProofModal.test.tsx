// @vitest-environment jsdom
import { cleanup,render,screen,waitFor } from '@testing-library/react';
import { afterEach,expect,test,vi } from 'vitest';
import { getShipmentProof } from '../api';
import { ShipmentProofModal } from './ShipmentProofModal';

vi.mock('../api', /* shipmentProofApiMock 隔离凭证弹窗的真实 HTTP 请求。 */ () => ({ getShipmentProof: vi.fn() }));

// getShipmentProofMock 是凭证只读请求的可控替身。
const getShipmentProofMock = vi.mocked(getShipmentProof);

afterEach(/* shipmentProofModalCleanup 清理弹窗 Portal 和请求替身。 */ () => {
	cleanup();
	vi.clearAllMocks();
});

test('发货凭证弹窗展示 ERP 描述、时间和官方图片', /* shipmentProofSuccessCase 验证本地凭证完整展示。 */ async () => {
	getShipmentProofMock.mockResolvedValue({ success: true, order_id: 'order-1', account_id: 'account-1', trade_text: '在线交付完成', image_urls: ['https://img.example/proof.png'], source: 'erp', submitted_at: 100 });
	render(<ShipmentProofModal orderID="order-1" open onClose={vi.fn()} />);
	await waitFor(/* proofLoadedAssertion 等待描述和图片进入只读弹窗。 */ () => expect(screen.getByText('在线交付完成')).toBeTruthy());
	expect(screen.getByRole('img', { name: '发货凭证' }).getAttribute('src')).toBe('https://img.example/proof.png');
	expect(screen.getByText('图片凭证 · 1 张')).toBeTruthy();
});

test('发货凭证读取失败显示服务端原因', /* shipmentProofFailureCase 验证缺失记录不会显示空成功页。 */ async () => {
	getShipmentProofMock.mockRejectedValue(new Error('该订单不是通过 ERP 发货或未保存凭证'));
	render(<ShipmentProofModal orderID="order-missing" open onClose={vi.fn()} />);
	await waitFor(/* proofErrorAssertion 等待只读错误进入提示区。 */ () => expect(screen.getByRole('alert').textContent).toContain('不是通过 ERP 发货'));
});
