import React, { useEffect, useState } from 'react';
import { Lightbulb, Link2, Plus, X } from 'lucide-react';
import type { KnowledgeEntryPreview, KnowledgeFAQAliasPreview } from '../types';

/** KnowledgeAliasDialogProps 描述 FAQ 相似问法列表、建议和用户确认动作。 */
export interface KnowledgeAliasDialogProps {
  /** open 表示相似问法管理弹窗是否可见。 */
  open: boolean;
  /** entry 是当前管理相似问法的 FAQ。 */
  entry: KnowledgeEntryPreview | null;
  /** aliases 是服务端已保存的相似问法。 */
  aliases: KnowledgeFAQAliasPreview[];
  /** suggestions 是后端确定性生成且尚未落库的建议。 */
  suggestions: string[];
  /** loading 表示相似问法和建议正在读取。 */
  loading: boolean;
  /** pending 表示新增或删除请求正在执行。 */
  pending: boolean;
  /** onClose 关闭弹窗且不改变 FAQ。 */
  onClose: () => void;
  /** onAdd 保存用户确认的相似问法和来源。 */
  onAdd: (alias: string, source: KnowledgeFAQAliasPreview['source']) => Promise<boolean>;
  /** onDelete 删除单条相似问法。 */
  onDelete: (alias: KnowledgeFAQAliasPreview) => Promise<boolean>;
}

/** aliasSourceLabels 将持久化来源转换为用户可见说明。 */
const aliasSourceLabels: Readonly<Record<KnowledgeFAQAliasPreview['source'], string>> = { manual: '手工', system: '系统建议', debug: '调试器' };

/** KnowledgeAliasDialog 让用户确认、补充和删除 FAQ 相似问法，不修改标准答案。 */
export const KnowledgeAliasDialog: React.FC<KnowledgeAliasDialogProps> = ({ open, entry, aliases, suggestions, loading, pending, onClose, onAdd, onDelete }) => {
  /** manualAlias 和 setManualAlias 保存尚未提交的单条手工问法。 */
  const [manualAlias, setManualAlias] = useState('');

  useEffect(/* aliasInputResetEffect 在切换 FAQ 或关闭弹窗时丢弃未提交输入。 */ () => {
    setManualAlias('');
  }, [entry?.id, open]);

  if (!open || !entry) return null;
  // canAdd 表示手工问法去空格后可提交且当前没有变更请求。
  const canAdd = manualAlias.trim() !== '' && !pending;

  /** handleAddManual 保存手工问法，成功后清空输入。 */
  const handleAddManual = async (): Promise<void> => {
    // added 表示手工问法已经通过冲突检查并写入服务端。
    const added = await onAdd(manualAlias, 'manual');
    if (added) setManualAlias('');
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm" role="dialog" aria-modal="true" aria-label="管理相似问法">
      <div className="w-full max-w-2xl overflow-hidden rounded-2xl border border-white/60 bg-white shadow-modal">
        <div className="flex items-start justify-between border-b border-slate-100 px-6 py-5">
          <div className="flex items-start gap-3">
            <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand-50 text-brand-700"><Link2 className="h-5 w-5" /></span>
            <div>
              <h2 className="text-xl font-black text-slate-950">相似问法</h2>
              <p className="mt-1 max-w-xl text-sm leading-6 text-slate-500">标准问题：{entry.title}</p>
            </div>
          </div>
          <button type="button" onClick={onClose} disabled={pending} className="rounded-xl p-2 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 disabled:cursor-not-allowed disabled:opacity-50" aria-label="关闭相似问法弹窗"><X className="h-5 w-5" /></button>
        </div>

        <div className="max-h-[70vh] space-y-5 overflow-y-auto px-6 py-5">
          <section aria-label="已保存相似问法">
            <div className="text-sm font-black text-slate-800">已保存</div>
            <p className="mt-1 text-xs leading-5 text-slate-500">这些问法匹配同一个标准答案；删除标签不会修改 FAQ 正文或审核状态。</p>
            <div className="mt-3 flex flex-wrap gap-2">
              {aliases.map(/* alias 是当前待显示来源和删除入口的相似问法。 */ alias => (
                <span key={alias.id} className="flex items-center gap-2 rounded-full border border-brand-200 bg-brand-50 px-3 py-1.5 text-xs font-bold text-brand-800">
                  <span>{alias.alias}</span><span className="text-[10px] text-brand-500">{aliasSourceLabels[alias.source]}</span>
                  <button type="button" onClick={/* deleteAliasHandler 删除当前相似问法标签。 */ () => void onDelete(alias)} disabled={pending} className="rounded-full text-brand-400 transition-colors hover:text-danger-600 disabled:cursor-not-allowed disabled:opacity-50" aria-label={`删除相似问法：${alias.alias}`}><X className="h-3.5 w-3.5" /></button>
                </span>
              ))}
              {!loading && aliases.length === 0 && <span className="text-xs text-slate-400">还没有保存相似问法。</span>}
              {loading && <span className="text-xs text-slate-400">正在读取相似问法…</span>}
            </div>
          </section>

          <section className="rounded-xl border border-info-200 bg-info-50 p-4" aria-label="系统建议">
            <div className="flex items-center gap-2 text-sm font-black text-info-900"><Lightbulb className="h-4 w-4" />系统建议</div>
            <p className="mt-1 text-xs leading-5 text-info-700">建议只来自确定性中文表达和当前 FAQ 主题，不调用模型；点击后才会保存。</p>
            <div className="mt-3 flex flex-wrap gap-2">
              {suggestions.map(/* suggestion 是当前可由用户确认采纳的问法。 */ suggestion => <button key={suggestion} type="button" onClick={/* addSuggestionHandler 确认并保存当前系统建议。 */ () => void onAdd(suggestion, 'system')} disabled={pending} className="rounded-full border border-info-200 bg-white px-3 py-1.5 text-xs font-bold text-info-800 transition-colors hover:border-info-400 disabled:cursor-not-allowed disabled:opacity-50">+ {suggestion}</button>)}
              {!loading && suggestions.length === 0 && <span className="text-xs text-info-600">暂无新的系统建议。</span>}
            </div>
          </section>

          <section aria-label="手工添加相似问法">
            <label className="block">
              <span className="mb-2 block text-sm font-black text-slate-800">手工补充</span>
              <span className="flex gap-2">
                <input value={manualAlias} onChange={/* manualAliasChangeHandler 更新尚未提交的问法。 */ event => setManualAlias(event.target.value)} onKeyDown={/* manualAliasKeyHandler 使用回车提交有效问法。 */ event => { if (event.key === 'Enter' && canAdd) void handleAddManual(); }} placeholder="例如：怎么充值" className="ios-input h-11 min-w-0 flex-1 rounded-xl px-4 text-sm" aria-label="手工相似问法" />
                <button type="button" onClick={/* addManualAliasHandler 保存当前手工问法。 */ () => void handleAddManual()} disabled={!canAdd} className="ios-btn-primary flex items-center gap-2 rounded-xl px-4 text-sm disabled:cursor-not-allowed disabled:opacity-40"><Plus className="h-4 w-4" />添加</button>
              </span>
            </label>
          </section>
        </div>

        <div className="flex justify-end border-t border-slate-100 bg-slate-50/70 px-6 py-4">
          <button type="button" onClick={onClose} disabled={pending} className="rounded-xl px-4 py-2.5 text-sm font-extrabold text-slate-600 transition-colors hover:bg-slate-200/70 disabled:cursor-not-allowed disabled:opacity-50">完成</button>
        </div>
      </div>
    </div>
  );
};
