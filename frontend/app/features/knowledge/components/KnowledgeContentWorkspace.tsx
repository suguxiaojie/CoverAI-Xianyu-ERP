import React from 'react';
import { AlertCircle, CheckCircle2, Clock3, FileText, HelpCircle, MessageSquareMore, PencilLine, Plus, ShieldCheck, SlidersHorizontal, Trash2 } from 'lucide-react';
import type { KnowledgeBasePreview, KnowledgeContentType, KnowledgeEntryPreview, KnowledgeRiskLevel } from '../types';

/** KnowledgeContentWorkspaceProps 描述选中知识库的顶部摘要、分类筛选和条目操作。 */
export interface KnowledgeContentWorkspaceProps {
  /** knowledgeBase 是当前用户选中的知识库摘要。 */
  knowledgeBase: KnowledgeBasePreview;
  /** activeType 是中间工作台当前显示的 FAQ 或文档分类。 */
  activeType: KnowledgeContentType;
  /** entries 是当前知识库和分类下的预览条目。 */
  entries: KnowledgeEntryPreview[];
  /** onTypeChange 在用户切换 FAQ 和文档时更新页面状态。 */
  onTypeChange: (type: KnowledgeContentType) => void;
  /** onCreateEntry 打开当前知识库的新增知识预览弹窗。 */
  onCreateEntry: () => void;
  /** onEditEntry 打开当前 FAQ／文档的编辑弹窗。 */
  onEditEntry: (entry: KnowledgeEntryPreview) => void;
  /** onDeleteEntry 打开当前 FAQ／文档的删除确认弹窗。 */
  onDeleteEntry: (entry: KnowledgeEntryPreview) => void;
  /** onManageAliases 打开当前 FAQ 的相似问法管理弹窗。 */
  onManageAliases: (entry: KnowledgeEntryPreview) => void;
  /** onToggleEntry 仅在当前页面内存中切换条目启停状态。 */
  onToggleEntry: (entry: KnowledgeEntryPreview) => void;
  /** onReviewEntry 将待审核条目确认为权威内容，通过后仍默认停用。 */
  onReviewEntry: (entry: KnowledgeEntryPreview) => void;
  /** onToggleBaseStatus 切换知识库是否可参与离线检索。 */
  onToggleBaseStatus: () => void;
  /** actionPending 表示真实本地 API 变更正在进行，用于防止重复提交。 */
  actionPending: boolean;
  /** loading 表示当前知识库条目正在读取。 */
  loading: boolean;
}

/** RiskPresentation 描述风险等级在知识条目上的文案和视觉语义。 */
interface RiskPresentation {
  /** label 是显示给用户的风险等级名称。 */
  label: string;
  /** className 是区分风险语义的徽签样式。 */
  className: string;
}

/** riskPresentationByLevel 为条目风险等级提供文案和语义化样式。 */
const riskPresentationByLevel: Readonly<Record<KnowledgeRiskLevel, RiskPresentation>> = {
  low: { label: '低风险', className: 'bg-success-50 text-success-700' },
  medium: { label: '中风险', className: 'bg-warning-50 text-warning-700' },
  high: { label: '高风险', className: 'bg-danger-50 text-danger-700' },
};

/** KnowledgeContentWorkspace 渲染知识条目的审核、范围和启停预览。 */
export const KnowledgeContentWorkspace: React.FC<KnowledgeContentWorkspaceProps> = ({
  knowledgeBase,
  activeType,
  entries,
  onTypeChange,
  onCreateEntry,
  onEditEntry,
  onDeleteEntry,
  onManageAliases,
  onToggleEntry,
  onReviewEntry,
  onToggleBaseStatus,
  actionPending,
  loading,
}) => (
  <section className="flex min-h-[620px] flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white shadow-card" aria-label="知识内容">
    <div className="border-b border-slate-100 px-5 py-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="truncate text-lg font-black text-slate-950">{knowledgeBase.name}</h2>
            <span className={`rounded-full px-2.5 py-1 text-[11px] font-extrabold ${knowledgeBase.status === 'active' ? 'bg-success-50 text-success-700' : 'bg-warning-50 text-warning-700'}`}>
              {knowledgeBase.status === 'active' ? '已启用' : '草稿'}
            </span>
          </div>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-slate-500">{knowledgeBase.description}</p>
          <div className="mt-2 flex flex-wrap items-center gap-3 text-xs font-bold text-slate-400">
            <span>{knowledgeBase.scopeLabel}</span>
            <span>{knowledgeBase.reviewedCount}/{knowledgeBase.entryCount} 已审核</span>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" onClick={onToggleBaseStatus} disabled={actionPending} className="rounded-xl border border-slate-200 bg-white px-3.5 py-2.5 text-xs font-extrabold text-slate-600 transition-colors hover:border-brand-200 hover:text-brand-700 disabled:cursor-not-allowed disabled:opacity-50">
            {knowledgeBase.status === 'active' ? '转为草稿' : '启用离线检索'}
          </button>
          <button type="button" onClick={onCreateEntry} disabled={actionPending} className="ios-btn-primary flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm disabled:cursor-not-allowed disabled:opacity-50">
            <Plus className="h-4 w-4" />
            新增知识
          </button>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between gap-4 border-t border-slate-100 pt-4">
        <div className="flex rounded-xl bg-slate-100 p-1" role="tablist" aria-label="知识内容类型">
          <button
            type="button"
            onClick={/* faqTabHandler 将中间工作台切换到结构化问答。 */ () => onTypeChange('faq')}
            className={`flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-extrabold transition-colors ${activeType === 'faq' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-500 hover:text-slate-800'}`}
            role="tab"
            aria-selected={activeType === 'faq'}
          >
            <HelpCircle className="h-4 w-4" />FAQ
          </button>
          <button
            type="button"
            onClick={/* documentTabHandler 将中间工作台切换到纯文本文档。 */ () => onTypeChange('document')}
            className={`flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-extrabold transition-colors ${activeType === 'document' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-500 hover:text-slate-800'}`}
            role="tab"
            aria-selected={activeType === 'document'}
          >
            <FileText className="h-4 w-4" />文档
          </button>
        </div>
        <div className="hidden items-center gap-2 text-xs font-bold text-slate-400 sm:flex">
          <SlidersHorizontal className="h-4 w-4" />
          审核后才可检索
        </div>
      </div>
    </div>

    <div className="flex-1 space-y-3 overflow-y-auto bg-slate-50/50 p-4">
      {loading && <div className="flex min-h-64 items-center justify-center rounded-xl border border-slate-200 bg-white text-sm font-bold text-slate-400" role="status">正在读取知识条目…</div>}
      {!loading && entries.map(/* entry 是当前待展示审核、范围和风险的知识条目。 */ entry => {
        // riskPresentation 是当前条目风险等级对应的文案与样式。
        const riskPresentation = riskPresentationByLevel[entry.riskLevel];
        return (
          <article key={entry.id} className={`rounded-xl border bg-white p-4 transition-colors ${entry.enabled ? 'border-slate-200' : 'border-dashed border-slate-200 opacity-70'}`}>
            <div className="flex items-start gap-3">
              <span className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${entry.type === 'faq' ? 'bg-brand-50 text-brand-700' : 'bg-slate-100 text-slate-600'}`}>
                {entry.type === 'faq' ? <HelpCircle className="h-[18px] w-[18px]" /> : <FileText className="h-[18px] w-[18px]" />}
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="text-sm font-black leading-6 text-slate-950">{entry.title}</h3>
                    <div className="mt-1.5 flex flex-wrap items-center gap-2">
                      <span className={`rounded-full px-2 py-0.5 text-[10px] font-extrabold ${riskPresentation.className}`}>{riskPresentation.label}</span>
                      <span className={`flex items-center gap-1 text-[11px] font-bold ${entry.reviewed ? 'text-success-700' : 'text-warning-700'}`}>
                        {entry.reviewed ? <CheckCircle2 className="h-3.5 w-3.5" /> : <Clock3 className="h-3.5 w-3.5" />}
                        {entry.reviewed ? '已审核' : '待审核'}
                      </span>
                      {entry.requiresLiveData && <span className="flex items-center gap-1 text-[11px] font-bold text-info-700"><AlertCircle className="h-3.5 w-3.5" />需实时数据</span>}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <button
                      type="button"
                      onClick={/* editEntryHandler 打开当前条目的编辑弹窗。 */ () => onEditEntry(entry)}
                      disabled={actionPending}
                      className="flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1.5 text-[11px] font-extrabold text-slate-600 transition-colors hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                      aria-label={`编辑${entry.type === 'faq' ? 'FAQ' : '文档'}：${entry.title}`}
                    >
                      <PencilLine className="h-3.5 w-3.5" />编辑
                    </button>
                    {entry.type === 'faq' && (
                      <button type="button" onClick={/* manageAliasesHandler 打开当前 FAQ 相似问法管理弹窗。 */ () => onManageAliases(entry)} disabled={actionPending} className="flex items-center gap-1 rounded-lg border border-info-200 bg-white px-2.5 py-1.5 text-[11px] font-extrabold text-info-700 transition-colors hover:bg-info-50 disabled:cursor-not-allowed disabled:opacity-50" aria-label={`管理相似问法：${entry.title}`}>
                        <MessageSquareMore className="h-3.5 w-3.5" />相似问法
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={/* deleteEntryHandler 打开当前条目的删除确认弹窗。 */ () => onDeleteEntry(entry)}
                      disabled={actionPending}
                      className="flex items-center gap-1 rounded-lg border border-danger-200 bg-white px-2.5 py-1.5 text-[11px] font-extrabold text-danger-600 transition-colors hover:bg-danger-50 disabled:cursor-not-allowed disabled:opacity-50"
                      aria-label={`删除${entry.type === 'faq' ? 'FAQ' : '文档'}：${entry.title}`}
                    >
                      <Trash2 className="h-3.5 w-3.5" />删除
                    </button>
                    {!entry.reviewed && (
                      <button type="button" onClick={/* reviewEntryHandler 将待审核条目确认为权威内容。 */ () => onReviewEntry(entry)} disabled={actionPending} className="rounded-lg border border-warning-200 bg-warning-50 px-2.5 py-1.5 text-[11px] font-extrabold text-warning-800 transition-colors hover:bg-warning-100 disabled:cursor-not-allowed disabled:opacity-50">通过审核</button>
                    )}
                    <div className={`flex items-center gap-2 rounded-xl border px-2.5 py-1.5 ${entry.reviewed ? 'border-slate-200 bg-slate-50' : 'border-warning-200 bg-warning-50'}`}>
                      <span className="text-[11px] font-extrabold text-slate-500">参与检索</span>
                      <span className={`text-[11px] font-black ${!entry.reviewed ? 'text-warning-700' : entry.enabled ? 'text-brand-700' : 'text-slate-500'}`}>{!entry.reviewed ? '待审核' : entry.enabled ? '已启用' : '已停用'}</span>
                      <button
                        type="button"
                        onClick={/* entryToggleHandler 切换已审核条目的本地检索状态。 */ () => onToggleEntry(entry)}
                        disabled={!entry.reviewed || actionPending}
                        className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${entry.enabled ? 'bg-brand' : 'bg-slate-300'} disabled:cursor-not-allowed disabled:opacity-50`}
                        role="switch"
                        aria-checked={entry.enabled}
                        aria-label={entry.reviewed ? `${entry.enabled ? '停用' : '启用'}${entry.title}` : `待审核不能启用${entry.title}`}
                      >
                        <span className={`absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-sm transition-transform ${entry.enabled ? 'translate-x-5' : 'translate-x-0'}`} />
                      </button>
                    </div>
                  </div>
                </div>
                <p className="mt-3 line-clamp-3 text-sm leading-6 text-slate-600">{entry.content}</p>
                <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 pt-3 text-[11px] font-semibold text-slate-400">
                  <span>{entry.scopeLabel}</span>
                  <span>更新于 {entry.updatedAt}</span>
                </div>
              </div>
            </div>
          </article>
        );
      })}
      {!loading && entries.length === 0 && (
        <div className="flex min-h-64 flex-col items-center justify-center rounded-xl border border-dashed border-slate-200 bg-white px-5 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-100 text-slate-400"><ShieldCheck className="h-6 w-6" /></span>
          <h3 className="mt-4 text-sm font-black text-slate-800">还没有{activeType === 'faq' ? ' FAQ' : '文档'}</h3>
          <p className="mt-1 max-w-xs text-xs leading-5 text-slate-500">手工新增的条目将默认进入待审核，不会立即用于检索。</p>
        </div>
      )}
    </div>
  </section>
);
