import { CheckCircle2,Loader2,XCircle } from 'lucide-react';
import React from 'react';
import type { OrderRefreshJobStatusResponse } from '../api';

interface OrderSyncProgressCardProps {
  /** job 是服务端订单刷新任务的最新轻量状态。 */
  job: OrderRefreshJobStatusResponse | null;
  /** starting 表示后台任务创建请求尚未返回 job_id。 */
  starting: boolean;
  /** error 保存创建、轮询、取消或终态失败说明。 */
  error: string;
  /** onCancel 请求取消仍在运行的后台任务。 */
  onCancel: () => void;
}

// OrderSyncProgressCard 展示订单同步阶段、实时数量、整体进度和取消操作，不渲染逐单完整结果。
export const OrderSyncProgressCard: React.FC<OrderSyncProgressCardProps> = ({ job, starting, error, onCancel }) => {
  // progress 是当前服务端进度；任务刚创建时使用零进度占位。
  const progress = job?.progress;
  // percent 把服务端进度限制到有效范围，防止异常数据破坏进度条布局。
  const percent = Math.min(100, Math.max(0, progress?.percent ?? (job?.status === 'succeeded' ? 100 : 0)));
  // running 表示任务仍可取消。
  const running = starting || job?.status === 'queued' || job?.status === 'running';
  // succeeded 表示服务端已经写入完整成功终态。
  const succeeded = job?.status === 'succeeded';
	// partial 表示 worker 已正常结束，但业务结果仍有账号或订单失败。
	const partial = succeeded && job?.result?.partial_failure === true;
  // failed 表示任务失败、取消或页面收到错误说明。
  const failed = job?.status === 'failed' || job?.status === 'cancelled' || Boolean(error);
	// problem 表示进度卡必须使用警示样式，包含任务失败和业务部分完成。
	const problem = failed || partial;
  // importing 表示当前正在逐页读取平台订单，此时 succeeded 的语义是已读取数量而不是最终成功订单数。
  const importing = progress?.stage === 'importing_orders';
	// incremental 表示当前任务按可信历史边界提前停止，不承诺读取平台全部订单数。
	const incremental = progress?.mode === 'incremental';
	// completed 表示当前快照来自最终业务结果，不再把 detail 数量误认为账号数量。
	const completed = progress?.stage === 'completed';
  if (!starting && !job && !error) return null;

  return (
    <section className="rounded-2xl border border-blue-100 bg-white p-5 shadow-sm" aria-live="polite">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          {partial ? <XCircle className="mt-0.5 h-5 w-5 shrink-0 text-amber-500" /> : succeeded ? <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-green-600" /> : failed ? <XCircle className="mt-0.5 h-5 w-5 shrink-0 text-red-500" /> : <Loader2 className="mt-0.5 h-5 w-5 shrink-0 animate-spin text-blue-600" />}
          <div className="min-w-0">
            <h3 className="font-extrabold text-gray-900">订单同步进度</h3>
            <p className={`mt-1 text-sm ${problem ? partial ? 'text-amber-700' : 'text-red-600' : 'text-gray-600'}`}>
              {error || (partial ? job?.result?.message || '订单同步部分完成，请查看失败详情' : progress?.message) || (starting ? '正在创建后台同步任务' : succeeded ? '订单同步完成' : '等待后台任务开始')}
            </p>
          </div>
        </div>
        {running && <button type="button" onClick={onCancel} className="shrink-0 rounded-xl border border-red-100 bg-red-50 px-4 py-2 text-sm font-bold text-red-600 hover:bg-red-100">取消同步</button>}
      </div>

      <div className="mt-4">
        <div className="mb-2 flex items-center justify-between text-xs font-bold text-gray-500">
		  <span>{incremental && importing ? `已扫描 ${progress?.processed || 0} 条` : completed && progress?.total ? `详情：${progress.processed} / ${progress.total}` : progress?.total ? `${progress.processed} / ${progress.total}` : '正在准备'}</span>
          <span>{percent}%</span>
        </div>
        <div className="h-3 overflow-hidden rounded-full bg-gray-100" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent}>
          <div className={`h-full rounded-full transition-[width] duration-500 ${partial ? 'bg-amber-500' : failed ? 'bg-red-500' : succeeded ? 'bg-green-500' : 'bg-blue-600'}`} style={{ width: `${percent}%` }} />
        </div>
        {progress && (
          <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-500">
            <span>{importing ? '已读取' : completed ? '详情成功' : '成功'}：<strong className="text-green-700">{progress.succeeded}</strong></span>
            <span>失败：<strong className={progress.failed > 0 ? 'text-red-600' : 'text-gray-700'}>{progress.failed}</strong></span>
            {progress.total_accounts ? <span>账号：{progress.current_account || 0} / {progress.total_accounts}</span> : null}
			{progress.current_page ? <span>页码：{incremental ? progress.current_page : `${progress.current_page} / ${progress.total_pages || '?'}`}</span> : null}
			{incremental && progress.boundary_required ? <span>历史边界：{progress.boundary_matched || 0} / {progress.boundary_required}</span> : null}
			{progress.mode ? <span>模式：{progress.mode === 'full' ? '全量校准' : '增量同步'}</span> : null}
			{job?.result?.summary?.conflict_skipped ? <span>跨账号冲突跳过：{job.result.summary.conflict_skipped}</span> : null}
            <span>任务状态：{partial ? '部分完成' : job?.status || 'running'}</span>
          </div>
        )}
      </div>
    </section>
  );
};
