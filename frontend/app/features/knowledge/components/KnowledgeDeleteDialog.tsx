import React from 'react';
import { AlertTriangle, Trash2, X } from 'lucide-react';
import type { KnowledgeEntryPreview } from '../types';

/** KnowledgeDeleteDialogProps 描述单条 FAQ／文档删除确认所需状态与动作。 */
export interface KnowledgeDeleteDialogProps {
  /** open 表示删除确认弹窗是否可见。 */
  open: boolean;
  /** entry 是即将删除的 FAQ／文档；null 表示当前没有删除目标。 */
  entry: KnowledgeEntryPreview | null;
  /** pending 表示删除请求正在执行，用于禁止关闭和重复提交。 */
  pending: boolean;
  /** onClose 取消删除并关闭确认弹窗。 */
  onClose: () => void;
  /** onConfirm 执行当前条目的正式删除。 */
  onConfirm: () => void;
}

/** KnowledgeDeleteDialog 使用应用内确认说明单条知识删除范围和不可撤销性。 */
export const KnowledgeDeleteDialog: React.FC<KnowledgeDeleteDialogProps> = ({ open, entry, pending, onClose, onConfirm }) => {
  if (!open || !entry) return null;
  // typeLabel 是删除目标的用户可见内容类型名称。
  const typeLabel = entry.type === 'faq' ? 'FAQ' : '文档';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label={`删除${typeLabel}`}>
      <div className="w-full max-w-lg overflow-hidden rounded-2xl border border-white/60 bg-white shadow-modal">
        <div className="flex items-start justify-between border-b border-slate-100 px-6 py-5">
          <div className="flex items-start gap-3">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-danger-50 text-danger-600"><Trash2 className="h-5 w-5" /></span>
            <div>
              <h2 className="text-xl font-black text-slate-950">删除{typeLabel}</h2>
              <p className="mt-1 text-sm text-slate-500">只删除当前知识条目，不会删除整个知识库。</p>
            </div>
          </div>
          <button type="button" onClick={onClose} disabled={pending} className="rounded-xl p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 disabled:cursor-not-allowed disabled:opacity-50" aria-label="关闭删除知识弹窗"><X className="h-5 w-5" /></button>
        </div>

        <div className="space-y-4 px-6 py-5">
          <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
            <div className="text-xs font-extrabold text-slate-400">即将删除</div>
            <div className="mt-1 break-words text-sm font-black leading-6 text-slate-900">{entry.title}</div>
          </div>
          <div className="flex gap-3 rounded-xl border border-danger-200 bg-danger-50 px-4 py-3 text-sm text-danger-900">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <p className="leading-6">删除后，该条目会立即退出检索，对应检索分块也会一起删除。当前页面没有回收站，此操作不能撤销。</p>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-slate-100 bg-slate-50/70 px-6 py-4">
          <button type="button" onClick={onClose} disabled={pending} className="rounded-xl px-4 py-2.5 text-sm font-extrabold text-slate-600 transition-colors hover:bg-slate-200/70 disabled:cursor-not-allowed disabled:opacity-50">取消</button>
          <button type="button" onClick={onConfirm} disabled={pending} className="flex items-center gap-2 rounded-xl bg-danger-600 px-5 py-2.5 text-sm font-extrabold text-white transition-colors hover:bg-danger-700 disabled:cursor-not-allowed disabled:opacity-50">
            <Trash2 className="h-4 w-4" />{pending ? '正在删除…' : `确认删除${typeLabel}`}
          </button>
        </div>
      </div>
    </div>
  );
};
