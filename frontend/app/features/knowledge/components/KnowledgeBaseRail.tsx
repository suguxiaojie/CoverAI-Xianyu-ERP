import React from 'react';
import { BookOpenCheck, CircleDot, Database, Search } from 'lucide-react';
import type { KnowledgeBasePreview } from '../types';

/** KnowledgeBaseRailProps 描述知识库列表的搜索、选中和新建交互。 */
export interface KnowledgeBaseRailProps {
  /** knowledgeBases 是经搜索过滤后需要展示的预览知识库。 */
  knowledgeBases: KnowledgeBasePreview[];
  /** selectedId 是当前内容工作台展示的知识库标识。 */
  selectedId: string;
  /** searchQuery 是当前仅作用于本地夹具的搜索词。 */
  searchQuery: string;
  /** onSearchQueryChange 在用户编辑搜索词时更新页面短暂状态。 */
  onSearchQueryChange: (query: string) => void;
  /** onSelect 在用户选择知识库时切换中间内容。 */
  onSelect: (knowledgeBaseId: string) => void;
  /** onCreate 打开新建知识库的设计预览弹窗。 */
  onCreate: () => void;
}

/** accentClassByName 把夹具辅助色转换为不承载业务状态的图标样式。 */
const accentClassByName: Readonly<Record<KnowledgeBasePreview['accent'], string>> = {
  brand: 'bg-brand-50 text-brand-700',
  success: 'bg-success-50 text-success-700',
  warning: 'bg-warning-50 text-warning-700',
};

/** KnowledgeBaseRail 渲染知识库搜索与范围选择区。 */
export const KnowledgeBaseRail: React.FC<KnowledgeBaseRailProps> = ({
  knowledgeBases,
  selectedId,
  searchQuery,
  onSearchQueryChange,
  onSelect,
  onCreate,
}) => (
  <section className="flex min-h-[620px] flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white shadow-card" aria-label="知识库列表">
    <div className="border-b border-slate-100 p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <div className="text-xs font-extrabold uppercase tracking-[0.16em] text-slate-400">知识范围</div>
          <h2 className="mt-1 text-lg font-black text-slate-950">知识库</h2>
        </div>
        <button
          type="button"
          onClick={onCreate}
          className="rounded-xl bg-slate-950 px-3 py-2 text-xs font-extrabold text-white transition-colors hover:bg-slate-800"
        >
          新建
        </button>
      </div>
      <label className="relative mt-4 block">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          value={searchQuery}
          onChange={/* searchChangeHandler 把搜索输入同步到页面内存状态。 */ event => onSearchQueryChange(event.target.value)}
          placeholder="搜索名称或范围"
          className="ios-input h-10 w-full rounded-xl pl-9 pr-3 text-sm"
          aria-label="搜索知识库"
        />
      </label>
    </div>

    <div className="flex-1 space-y-2 overflow-y-auto p-3">
      {knowledgeBases.map(/* knowledgeBase 是当前待展示和选中的预览知识库。 */ knowledgeBase => {
        // active 表示当前知识库是否为中间工作台的选中项。
        const active = knowledgeBase.id === selectedId;
        return (
          <button
            key={knowledgeBase.id}
            type="button"
            onClick={/* knowledgeBaseSelectHandler 将用户点击的知识库切换为当前项。 */ () => onSelect(knowledgeBase.id)}
            className={`w-full rounded-xl border p-3 text-left transition-all ${active ? 'border-brand-200 bg-brand-50/70 shadow-brand-soft' : 'border-transparent bg-slate-50/70 hover:border-slate-200 hover:bg-white'}`}
            aria-pressed={active}
          >
            <div className="flex items-start gap-3">
              <span className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${accentClassByName[knowledgeBase.accent]}`}>
                <BookOpenCheck className="h-[18px] w-[18px]" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-black text-slate-900">{knowledgeBase.name}</span>
                  <CircleDot className={`h-3.5 w-3.5 shrink-0 ${knowledgeBase.status === 'active' ? 'text-success-500' : 'text-warning-500'}`} aria-label={knowledgeBase.status === 'active' ? '已启用' : '草稿'} />
                </span>
                <span className="mt-1 block line-clamp-2 text-xs leading-5 text-slate-500">{knowledgeBase.description}</span>
                <span className="mt-2 flex items-center gap-1.5 text-[11px] font-bold text-slate-400">
                  <Database className="h-3.5 w-3.5" />
                  {knowledgeBase.entryCount} 条 · {knowledgeBase.reviewedCount} 条已审核
                </span>
                <span className="mt-1.5 block truncate text-[11px] font-semibold text-slate-500">{knowledgeBase.scopeLabel}</span>
              </span>
            </div>
          </button>
        );
      })}
      {knowledgeBases.length === 0 && (
        <div className="rounded-xl border border-dashed border-slate-200 px-4 py-10 text-center text-sm font-semibold text-slate-400">
          没有匹配的知识库
        </div>
      )}
    </div>
  </section>
);
