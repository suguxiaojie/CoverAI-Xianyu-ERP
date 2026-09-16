import React from 'react';
import { BookOpenCheck, X } from 'lucide-react';
import type { KnowledgeScopeOption } from '../types';

/** KnowledgeBaseDraft 是新建知识库预览弹窗中的短暂表单数据。 */
export interface KnowledgeBaseDraft {
  /** name 是用户输入的知识库名称。 */
  name: string;
  /** description 是对权威内容范围的简短说明。 */
  description: string;
  /** scopeValue 是用户、店铺或商品范围的稳定选项标识。 */
  scopeValue: string;
}

/** KnowledgeBaseDialogProps 描述新建知识库预览弹窗的表单与交互。 */
export interface KnowledgeBaseDialogProps {
  /** open 表示弹窗是否在页面上展示。 */
  open: boolean;
  /** draft 是当前尚未持久化的知识库输入。 */
  draft: KnowledgeBaseDraft;
  /** onDraftChange 在任意输入变化时替换当前表单数据。 */
  onDraftChange: (draft: KnowledgeBaseDraft) => void;
  /** scopeOptions 是从本地 API 读取的用户、店铺和商品可选范围。 */
  scopeOptions: KnowledgeScopeOption[];
  /** onClose 关闭弹窗且不创建页面预览项。 */
  onClose: () => void;
  /** onSavePreview 将草稿知识库保存到第一阶段本地 API，成功后由页面关闭弹窗。 */
  onSavePreview: () => void;
}

/** KnowledgeBaseDialog 渲染不发起 API 请求的新建知识库交互。 */
export const KnowledgeBaseDialog: React.FC<KnowledgeBaseDialogProps> = ({ open, draft, onDraftChange, scopeOptions, onClose, onSavePreview }) => {
  if (!open) return null;
  // canSave 要求知识库名称和权威范围说明同时存在。
  const canSave = draft.name.trim() !== '' && draft.description.trim() !== '';
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label="新建知识库预览">
      <div className="w-full max-w-lg overflow-hidden rounded-2xl border border-white/60 bg-white shadow-modal">
        <div className="flex items-start justify-between border-b border-slate-100 px-6 py-5">
          <div className="flex items-start gap-3">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand-50 text-brand-700"><BookOpenCheck className="h-5 w-5" /></span>
            <div>
              <h2 className="text-xl font-black text-slate-950">新建知识库</h2>
              <p className="mt-1 text-sm text-slate-500">保存到本地数据库，默认为草稿。</p>
            </div>
          </div>
          <button type="button" onClick={onClose} className="rounded-xl p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700" aria-label="关闭新建知识库弹窗"><X className="h-5 w-5" /></button>
        </div>
        <div className="space-y-5 px-6 py-5">
          <label className="block">
            <span className="mb-2 block text-sm font-black text-slate-800">知识库名称</span>
            <input value={draft.name} onChange={/* knowledgeBaseNameHandler 更新页面内存中的知识库名称。 */ event => onDraftChange({ ...draft, name: event.target.value })} placeholder="例如：商品使用教程" className="ios-input h-11 w-full rounded-xl px-4 text-sm" />
          </label>
          <label className="block">
            <span className="mb-2 block text-sm font-black text-slate-800">权威内容范围</span>
            <textarea value={draft.description} onChange={/* knowledgeBaseDescriptionHandler 更新知识库的责任范围说明。 */ event => onDraftChange({ ...draft, description: event.target.value })} placeholder="说明这个知识库可以回答什么，不包含什么。" className="ios-input min-h-28 w-full resize-none rounded-xl px-4 py-3 text-sm leading-6" />
          </label>
          <label className="block">
            <span className="mb-2 block text-sm font-black text-slate-800">适用范围</span>
            <select value={draft.scopeValue} onChange={/* knowledgeBaseScopeHandler 将用户选择的店铺／商品范围写入草稿表单。 */ event => onDraftChange({ ...draft, scopeValue: event.target.value })} className="ios-input h-11 w-full rounded-xl px-3 text-sm" aria-label="当前知识库适用范围">
              {scopeOptions.map(/* option 是当前待展示的用户、店铺或商品范围。 */ option => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
            <span className="mt-2 block text-xs leading-5 text-slate-500">商品范围会同时保存所属店铺；后端会再次校验店铺和商品归属。</span>
          </label>
          <div className="rounded-xl border border-info-200 bg-info-50 px-4 py-3 text-xs leading-5 text-info-800">新知识库默认为草稿，没有经过人工审核的条目不会进入检索结果。</div>
        </div>
        <div className="flex justify-end gap-3 border-t border-slate-100 bg-slate-50/70 px-6 py-4">
          <button type="button" onClick={onClose} className="rounded-xl px-4 py-2.5 text-sm font-extrabold text-slate-600 transition-colors hover:bg-slate-200/70">取消</button>
          <button type="button" onClick={onSavePreview} disabled={!canSave} className="ios-btn-primary rounded-xl px-5 py-2.5 text-sm disabled:cursor-not-allowed disabled:opacity-40">创建草稿知识库</button>
        </div>
      </div>
    </div>
  );
};
