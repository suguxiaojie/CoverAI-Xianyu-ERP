// @vitest-environment jsdom
import { cleanup,fireEvent,render,screen } from '@testing-library/react';
import { afterEach,describe,expect,test,vi } from 'vitest';
import { OrderSyncProgressCard } from './OrderSyncProgressCard';

describe('OrderSyncProgressCard', /* 当前回调验证订单同步运行、取消和成功终态展示。 */ () => {
  afterEach(/* 当前回调清理订单进度卡 DOM。 */ () => cleanup());

  test('运行中展示实时数量、百分比并允许取消', /* 当前回调验证进度条和取消交互。 */ () => {
    // onCancel 是取消同步操作替身。
    const onCancel = vi.fn();
    render(<OrderSyncProgressCard
      job={{ success: true, job_id: 'job-1', status: 'running', progress: { stage: 'syncing_details', message: '正在逐单同步订单详情', processed: 4, total: 10, succeeded: 3, failed: 1, percent: 56 } }}
      starting={false}
      error=""
      onCancel={onCancel}
    />);
    expect(screen.getByText('正在逐单同步订单详情')).toBeTruthy();
    expect(screen.getByText('4 / 10')).toBeTruthy();
    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('56');
    fireEvent.click(screen.getByText('取消同步'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  test('成功终态显示百分之百且不再提供取消', /* 当前回调验证完成后进度卡保持结果可见。 */ () => {
    render(<OrderSyncProgressCard
      job={{ success: true, job_id: 'job-1', status: 'succeeded', progress: { stage: 'completed', message: '订单同步完成', processed: 10, total: 10, succeeded: 10, failed: 0, percent: 100 } }}
      starting={false}
      error=""
      onCancel={/* cancelNoop 完成状态不应触发取消。 */ () => undefined}
    />);
    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('100');
		expect(screen.getByText('详情：10 / 10')).toBeTruthy();
    expect(screen.queryByText('取消同步')).toBeNull();
  });

	test('部分完成保留账号和详情维度并展示冲突跳过', /* 当前回调验证 succeeded 不会掩盖业务 partial_failure。 */ () => {
		render(<OrderSyncProgressCard
			job={{ success: true, job_id: 'job-partial', status: 'succeeded', progress: { stage: 'completed', message: '订单同步部分完成，请查看失败详情', processed: 1, total: 1, succeeded: 1, failed: 1, percent: 100, current_account: 2, total_accounts: 2 }, result: { partial_failure: true, message: '订单同步部分完成：1 项失败', summary: { discovered: 0, list_updated: 1, soft_deleted: 0, detail_total: 1, total: 1, updated: 0, no_change: 1, failed: 1, account_total: 2, account_succeeded: 1, account_failed: 1, conflict_skipped: 3 }, results: [] } }}
			starting={false}
			error=""
			onCancel={/* cancelNoop 部分完成已是终态，不允许取消。 */ () => undefined}
		/>);
		expect(screen.getByText('订单同步部分完成：1 项失败')).toBeTruthy();
		expect(screen.getByText('详情：1 / 1')).toBeTruthy();
		expect(screen.getByText('账号：2 / 2')).toBeTruthy();
		expect(screen.getByText('跨账号冲突跳过：3')).toBeTruthy();
		expect(screen.getByText('任务状态：部分完成')).toBeTruthy();
	});

  test('订单导入阶段显示订单、页码和账号三级进度', /* 当前回调验证平台分页累计数量直接显示为 xxx/总订单数。 */ () => {
    render(<OrderSyncProgressCard
      job={{ success: true, job_id: 'job-import', status: 'running', progress: { stage: 'importing_orders', message: '全量校准正在扫描订单（账号 1/2，第 14/32 页）', processed: 420, total: 937, succeeded: 420, failed: 0, percent: 14, current_page: 14, total_pages: 32, current_account: 1, total_accounts: 2, mode: 'full' } }}
      starting={false}
      error=""
      onCancel={/* cancelNoop 本用例只验证展示。 */ () => undefined}
    />);
    expect(screen.getByText('420 / 937')).toBeTruthy();
    expect(screen.getByText('已读取：').parentElement?.textContent).toContain('420');
    expect(screen.getByText('账号：1 / 2')).toBeTruthy();
    expect(screen.getByText('页码：14 / 32')).toBeTruthy();
		expect(screen.getByText('模式：全量校准')).toBeTruthy();
  });

	test('增量阶段显示已扫描数量和可信历史边界，不把平台总量误认为必读总量', /* 当前回调验证增量范围说明。 */ () => {
		render(<OrderSyncProgressCard
			job={{ success: true, job_id: 'job-incremental', status: 'running', progress: { stage: 'importing_orders', message: '增量同步正在扫描订单', processed: 60, total: 937, succeeded: 60, failed: 0, percent: 15, current_page: 2, total_pages: 32, current_account: 1, total_accounts: 1, mode: 'incremental', boundary_matched: 17, boundary_required: 20 } }}
			starting={false}
			error=""
			onCancel={/* cancelNoop 本用例只验证增量范围展示。 */ () => undefined}
		/>);
		expect(screen.getByText('已扫描 60 条')).toBeTruthy();
		expect(screen.queryByText('60 / 937')).toBeNull();
		expect(screen.getByText('页码：2')).toBeTruthy();
		expect(screen.getByText('历史边界：17 / 20')).toBeTruthy();
		expect(screen.getByText('模式：增量同步')).toBeTruthy();
	});
});
