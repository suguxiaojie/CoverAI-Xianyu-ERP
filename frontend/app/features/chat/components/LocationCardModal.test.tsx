// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen,waitFor } from '@testing-library/react';
import { afterEach,describe,expect,test,vi } from 'vitest';
import { LocationCardModal } from './LocationCardModal';

afterEach(/* cleanupLocationCardModal 卸载弹窗并恢复测试替身。 */ () => { cleanup(); vi.restoreAllMocks(); });

describe('LocationCardModal', /* locationCardModalCases 验证逐次编辑、预览和显式发送边界。 */ () => {
	test('填写自定义标题说明和坐标后提交一次卡片', /* submitCustomLocationCardCase 验证完整表单值。 */ async () => {
		// onSend 是不访问真实平台的位置发送替身。
		const onSend = vi.fn(/* locationSendMock 返回明确成功以关闭弹窗。 */ async () => true);
		// onClose 记录成功后的受控关闭。
		const onClose = vi.fn();
		render(<LocationCardModal open sending={false} defaults={{ title: '', description: '', latitude: '', longitude: '' }} onClose={onClose} onSend={onSend} />);
		fireEvent.change(screen.getByLabelText('位置卡片标题'), { target: { value: 'CoverAI 实体店' } });
		fireEvent.change(screen.getByLabelText('位置卡片说明'), { target: { value: '东门电梯上楼右转' } });
		fireEvent.change(screen.getByLabelText('位置纬度'), { target: { value: '22.540503' } });
		fireEvent.change(screen.getByLabelText('位置经度'), { target: { value: '113.934528' } });
		fireEvent.click(screen.getByRole('button', { name: '下一步' }));
		expect(onSend).not.toHaveBeenCalled();
		expect(screen.getByRole('dialog', { name: '发送位置卡片' }).textContent).toContain('确认发送给当前会话');
		fireEvent.click(screen.getByRole('button', { name: '确认发送位置卡片' }));
		await waitFor(/* locationSubmittedAssertion 等待异步发送替身完成。 */ () => expect(onSend).toHaveBeenCalledWith({ title: 'CoverAI 实体店', description: '东门电梯上楼右转', latitude: 22.540503, longitude: 113.934528 }));
		expect(onClose).toHaveBeenCalledTimes(1);
	});

	test('无效坐标不能提交', /* rejectInvalidLocationCoordinateCase 验证本地范围门禁。 */ () => {
		// onSend 用于断言非法表单没有触发平台动作。
		const onSend = vi.fn(/* blockedLocationSendMock 不应被调用。 */ async () => true);
		render(<LocationCardModal open sending={false} defaults={{ title: '', description: '', latitude: '', longitude: '' }} onClose={vi.fn()} onSend={onSend} />);
		fireEvent.change(screen.getByLabelText('位置卡片标题'), { target: { value: '实体店' } });
		fireEvent.change(screen.getByLabelText('位置卡片说明'), { target: { value: '地址' } });
		fireEvent.change(screen.getByLabelText('位置纬度'), { target: { value: '91' } });
		fireEvent.change(screen.getByLabelText('位置经度'), { target: { value: '120' } });
		expect((screen.getByRole('button', { name: '下一步' }) as HTMLButtonElement).disabled).toBe(true);
		expect(onSend).not.toHaveBeenCalled();
	});

	test('打开时带入系统设置默认值且允许本次修改', /* loadLocationDefaultsCase 验证设置初值和逐次覆盖。 */ () => {
		render(<LocationCardModal open sending={false} defaults={{ title: '默认门店', description: '默认入口', latitude: '31.230400', longitude: '121.473700' }} onClose={vi.fn()} onSend={vi.fn(/* unusedLocationSendMock 不执行真实发送。 */ async () => true)} />);
		expect((screen.getByLabelText('位置卡片标题') as HTMLInputElement).value).toBe('默认门店');
		expect((screen.getByLabelText('位置卡片说明') as HTMLTextAreaElement).value).toBe('默认入口');
		fireEvent.change(screen.getByLabelText('位置卡片标题'), { target: { value: '本次临时门店' } });
		expect((screen.getByLabelText('位置卡片标题') as HTMLInputElement).value).toBe('本次临时门店');
	});
});
