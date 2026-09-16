import React from 'react';
import { AlertTriangle, BookPlus, FileText, HelpCircle, X } from 'lucide-react';
import type { KnowledgeContentType, KnowledgeDraft, KnowledgeRiskLevel } from '../types';

/** KnowledgeDraftDialogProps 描述本地新建知识的表单数据与关闭、保存交互。 */
export interface KnowledgeDraftDialogProps {
  /** open 表示预览弹窗是否对用户可见。 */
  open: boolean;
  /** knowledgeBaseName 是新条目归属的当前知识库名称。 */
  knowledgeBaseName: string;
  /** mode 区分新增知识和编辑已有知识。 */
  mode: 'create' | 'edit';
  /** resetReviewOnSave 表示保存编辑后会撤销当前审核和检索状态。 */
  resetReviewOnSave: boolean;
  /** draft 是本地表单的最新值，不包含服务端数据。 */
  draft: KnowledgeDraft;
  /** onDraftChange 在任意表单字段变化时替换短暂表单。 */
  onDraftChange: (draft: KnowledgeDraft) => void;
  /** onClose 放弃当前预览输入并关闭弹窗。 */
  onClose: () => void;
  /** onSavePreview 将待审核条目保存到第一阶段本地 API，成功后由页面关闭弹窗。 */
  onSavePreview: () => void;
}

/** ContentTypeOption 描述新增知识弹窗中一种手工维护内容。 */
interface ContentTypeOption {
  /** value 是写入本地表单的稳定内容类型。 */
  value: KnowledgeContentType;
  /** label 是内容类型的用户可见名称。 */
  label: string;
  /** description 说明该类型在知识库中承担的业务职责。 */
  description: string;
}

/** RiskOption 描述新增知识时允许选择的一个风险等级。 */
interface RiskOption {
  /** value 是写入本地表单的稳定风险值。 */
  value: KnowledgeRiskLevel;
  /** label 是风险等级的用户可见名称。 */
  label: string;
}

/** contentTypeOptions 是新建知识弹窗允许的两种手工维护内容。 */
const contentTypeOptions: ReadonlyArray<ContentTypeOption> = [
  { value: 'faq', label: 'FAQ', description: '用明确问题与标准答案约束回复' },
  { value: 'document', label: '文档', description: '粘贴纯文本或 Markdown 规则和教程' },
];

/** riskOptions 是人工标记知识业务风险的稳定选项。 */
const riskOptions: ReadonlyArray<RiskOption> = [
  { value: 'low', label: '低风险' },
  { value: 'medium', label: '中风险' },
  { value: 'high', label: '高风险' },
];

/** KnowledgeDraftDialog 渲染仅用于视觉验收的 FAQ／文档表单。 */
export const KnowledgeDraftDialog: React.FC<KnowledgeDraftDialogProps> = ({
  open,
  knowledgeBaseName,
  mode,
  resetReviewOnSave,
  draft,
  onDraftChange,
  onClose,
  onSavePreview,
}) => {
  if (!open) return null;
  // isEditing 表示当前弹窗正在修改已有 FAQ／文档。
  const isEditing = mode === 'edit';
  // canSave 只在标题和正文都有内容时允许加入页面内存预览。
  const canSave = draft.title.trim() !== '' && draft.content.trim() !== '';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label={isEditing ? `编辑${draft.type === 'faq' ? 'FAQ' : '文档'}` : '新增知识'}>
      <div className="w-full max-w-2xl overflow-hidden rounded-2xl border border-white/60 bg-white shadow-modal">
        <div className="flex items-start justify-between border-b border-slate-100 px-6 py-5">
          <div className="flex items-start gap-3">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand-50 text-brand-700">
              <BookPlus className="h-5 w-5" />
            </span>
            <div>
              <h2 className="text-xl font-black text-slate-950">{isEditing ? `编辑${draft.type === 'faq' ? ' FAQ' : '文档'}` : '新增知识'}</h2>
              <p className="mt-1 text-sm text-slate-500">归属于「{knowledgeBaseName}」，{isEditing ? '保存后需要重新审核。' : '保存后默认待审核。'}</p>
            </div>
          </div>
          <button type="button" onClick={onClose} className="rounded-xl p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700" aria-label="关闭新增知识弹窗">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="max-h-[72vh] space-y-5 overflow-y-auto px-6 py-5">
          <div>
            <div className="mb-2 text-sm font-black text-slate-800">内容类型</div>
            <div className="grid gap-3 sm:grid-cols-2">
              {contentTypeOptions.map(/* option 是当前待展示的 FAQ 或文档选项。 */ option => {
                // Icon 根据内容类型选择问答或文档图标。
                const Icon = option.value === 'faq' ? HelpCircle : FileText;
                // active 表示用户当前选中的内容类型。
                const active = draft.type === option.value;
                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={/* contentTypeChangeHandler 替换本地表单的内容类型。 */ () => onDraftChange({ ...draft, type: option.value })}
                    disabled={isEditing}
                    className={`rounded-xl border p-3 text-left transition-colors ${active ? 'border-brand-300 bg-brand-50' : 'border-slate-200 hover:border-slate-300'}`}
                    aria-pressed={active}
                  >
                    <span className="flex items-center gap-2 text-sm font-black text-slate-900"><Icon className="h-4 w-4" />{option.label}</span>
                    <span className="mt-1.5 block text-xs leading-5 text-slate-500">{option.description}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <label className="block">
            <span className="mb-2 block text-sm font-black text-slate-800">{draft.type === 'faq' ? '问题' : '文档标题'}</span>
            <input
              value={draft.title}
              onChange={/* titleChangeHandler 更新本地预览条目的标题。 */ event => onDraftChange({ ...draft, title: event.target.value })}
              placeholder={draft.type === 'faq' ? '例如：这个商品怎么使用？' : '例如：标准使用流程'}
              className="ios-input h-11 w-full rounded-xl px-4 text-sm"
            />
          </label>

          <label className="block">
            <span className="mb-2 block text-sm font-black text-slate-800">{draft.type === 'faq' ? '标准答案' : '纯文本／Markdown 内容'}</span>
            <textarea
              value={draft.content}
              onChange={/* contentChangeHandler 更新本地预览条目的脱敏正文。 */ event => onDraftChange({ ...draft, content: event.target.value })}
              placeholder="只填写已确认的权威内容，不从历史聊天自动归纳。"
              className="ios-input min-h-36 w-full resize-y rounded-xl px-4 py-3 text-sm leading-6"
            />
          </label>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block">
              <span className="mb-2 block text-sm font-black text-slate-800">适用范围</span>
              <input value={draft.scopeLabel} readOnly className="ios-input h-11 w-full cursor-not-allowed rounded-xl px-3 text-sm text-slate-500" aria-label="继承知识库范围" />
            </label>
            <label className="block">
              <span className="mb-2 block text-sm font-black text-slate-800">风险等级</span>
              <select
                value={draft.riskLevel}
                onChange={/* riskChangeHandler 将用户选择的风险等级写入本地表单。 */ event => onDraftChange({ ...draft, riskLevel: event.target.value as KnowledgeRiskLevel })}
                className="ios-input h-11 w-full rounded-xl px-3 text-sm"
              >
                {riskOptions.map(/* option 是当前待显示的风险等级选项。 */ option => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
            </label>
          </div>

          <label className="flex cursor-pointer items-start justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
            <span className="min-w-0">
              <span className="block text-sm font-black text-slate-800">需要实时数据</span>
              <span className="mt-1 block text-xs leading-5 text-slate-500">价格、库存、订单或买家身份必须读取当前业务状态时开启，与风险等级独立。</span>
            </span>
            <input
              type="checkbox"
              checked={draft.requiresLiveData}
              onChange={/* liveDataChangeHandler 独立保存是否依赖实时业务数据。 */ event => onDraftChange({ ...draft, requiresLiveData: event.target.checked })}
              className="mt-1 h-5 w-5 shrink-0 rounded border-slate-300 text-brand focus:ring-brand-300"
              aria-label="需要实时数据"
            />
          </label>

          <div className="flex gap-3 rounded-xl border border-warning-200 bg-warning-50 px-4 py-3 text-sm text-warning-900">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <p className="leading-6">{isEditing && resetReviewOnSave ? '保存修改后，该知识将自动退出检索并变为待审核；重新审核并启用后才会再次参与候选回复。' : '第一阶段不开放自动发送。新条目默认为“待审核”，不进入检索结果。'}</p>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 border-t border-slate-100 bg-slate-50/70 px-6 py-4">
          <button type="button" onClick={onClose} className="rounded-xl px-4 py-2.5 text-sm font-extrabold text-slate-600 transition-colors hover:bg-slate-200/70">取消</button>
          <button
            type="button"
            onClick={onSavePreview}
            disabled={!canSave}
            className="ios-btn-primary rounded-xl px-5 py-2.5 text-sm disabled:cursor-not-allowed disabled:opacity-40"
          >
            {isEditing ? '保存修改并重新审核' : '保存待审核知识'}
          </button>
        </div>
      </div>
    </div>
  );
};
