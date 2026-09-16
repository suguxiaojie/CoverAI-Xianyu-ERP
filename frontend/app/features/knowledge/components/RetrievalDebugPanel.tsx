import React from 'react';
import { AlertTriangle, ArrowRight, Bot, CheckCircle2, CircleSlash2, DatabaseZap, ExternalLink, SearchCheck, ShieldAlert } from 'lucide-react';
import { answerabilityLabels } from '../state';
import type { RetrievalEvidencePreview, RetrievalPreview } from '../types';

/** RetrievalDebugPanelProps 描述本地检索问题、演示结果和快捷示例交互。 */
export interface RetrievalDebugPanelProps {
  /** query 是当前待执行确定性夹具检索的用户输入。 */
  query: string;
  /** exampleQueries 是用来快速演示三类门禁分支的脱敏问题。 */
  exampleQueries: string[];
  /** result 是最近一次本地调试的可回答性结果。 */
  result: RetrievalPreview | null;
  /** onQueryChange 在用户编辑问题时更新短暂输入。 */
  onQueryChange: (query: string) => void;
  /** onRun 用本地夹具执行一次不调用模型的检索调试。 */
  onRun: () => void;
  /** onUseExample 把选中的脱敏示例填入问题并立即调试。 */
  onUseExample: (query: string) => void;
  /** onAddAlias 将当前调试问题明确绑定到最高分 FAQ。 */
  onAddAlias: (evidence: RetrievalEvidencePreview, query: string) => void;
  /** aliasPending 表示调试器相似问法正在保存。 */
  aliasPending: boolean;
  /** loading 表示后端确定性检索请求正在进行。 */
  loading: boolean;
}

/** answerabilityClassByStatus 为可回答性结果提供统一的颜色语义。 */
const answerabilityClassByStatus: Readonly<Record<RetrievalPreview['answerability'], string>> = {
  answerable: 'border-success-200 bg-success-50 text-success-800',
  needs_live_data: 'border-info-200 bg-info-50 text-info-800',
  needs_human: 'border-warning-200 bg-warning-50 text-warning-900',
  insufficient: 'border-slate-200 bg-slate-50 text-slate-700',
};

/** AnswerabilityIconProps 描述可回答性图标需要的确定性门禁状态。 */
interface AnswerabilityIconProps {
  /** status 是当前门禁结论，用于选择对应语义图标。 */
  status: RetrievalPreview['answerability'];
}

/** AnswerabilityIcon 根据确定性门禁结论渲染用户可识别图标。 */
const AnswerabilityIcon: React.FC<AnswerabilityIconProps> = ({ status }) => {
  if (status === 'answerable') return <CheckCircle2 className="h-5 w-5" />;
  if (status === 'needs_live_data') return <DatabaseZap className="h-5 w-5" />;
  if (status === 'needs_human') return <ShieldAlert className="h-5 w-5" />;
  return <CircleSlash2 className="h-5 w-5" />;
};

/** RetrievalDebugPanel 渲染不接真实 API 的检索调试和证据展示区。 */
export const RetrievalDebugPanel: React.FC<RetrievalDebugPanelProps> = ({
  query,
  exampleQueries,
  result,
  onQueryChange,
  onRun,
  onUseExample,
  onAddAlias,
  aliasPending,
  loading,
}) => {
  // aliasEvidence 是知识不足时仍达到证据展示阈值的最高分 FAQ。
  const aliasEvidence = result?.answerability === 'insufficient' ? result.evidence.find(/* evidence 是待检查是否可绑定相似问法的证据。 */ evidence => evidence.type === 'faq') : undefined;
  return (
  <section className="flex min-h-[620px] flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-slate-950 text-white shadow-panel" aria-label="检索调试">
    <div className="border-b border-white/10 px-5 py-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 text-xs font-extrabold uppercase tracking-[0.16em] text-brand-300">
            <Bot className="h-4 w-4" />RAG Debugger
          </div>
          <h2 className="mt-2 text-lg font-black">检索与可回答性</h2>
          <p className="mt-1 text-xs leading-5 text-slate-400">本地 API 确定性检索，不调用模型、不发送消息。</p>
        </div>
        <span className="shrink-0 rounded-full border border-warning-400/30 bg-warning-400/10 px-2.5 py-1 text-[10px] font-extrabold text-warning-300">预览模式</span>
      </div>
    </div>

    <div className="space-y-4 border-b border-white/10 p-4">
      <label className="block">
        <span className="mb-2 block text-xs font-extrabold text-slate-300">模拟买家问题</span>
        <textarea
          value={query}
          onChange={/* retrievalQueryChangeHandler 只更新本地检索输入，不触发外部请求。 */ event => onQueryChange(event.target.value)}
          placeholder="输入一个问题，查看知识证据和门禁结果…"
          className="min-h-24 w-full resize-none rounded-xl border border-white/10 bg-white/5 px-3.5 py-3 text-sm leading-6 text-white outline-none transition-colors placeholder:text-slate-500 focus:border-brand-400 focus:bg-white/10"
          aria-label="模拟买家问题"
        />
      </label>
      <button type="button" onClick={onRun} disabled={!query.trim() || loading} className="flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-brand text-sm font-black text-white transition-colors hover:bg-brand-highlight disabled:cursor-not-allowed disabled:opacity-40">
        <SearchCheck className="h-4 w-4" />
        {loading ? '正在检索…' : '开始检索调试'}
      </button>
      <div>
        <div className="mb-2 text-[11px] font-extrabold text-slate-500">快捷示例</div>
        <div className="flex flex-wrap gap-2">
          {exampleQueries.map(/* exampleQuery 是当前可一键执行的脱敏门禁示例。 */ exampleQuery => (
            <button key={exampleQuery} type="button" onClick={/* exampleQueryHandler 填入并立即执行当前示例。 */ () => onUseExample(exampleQuery)} className="rounded-lg border border-white/10 bg-white/5 px-2.5 py-1.5 text-left text-[11px] font-semibold text-slate-300 transition-colors hover:border-brand-400/60 hover:bg-brand-400/10 hover:text-white">
              {exampleQuery}
            </button>
          ))}
        </div>
      </div>
    </div>

    <div className="flex-1 overflow-y-auto p-4">
      {!result && (
        <div className="flex min-h-72 flex-col items-center justify-center rounded-xl border border-dashed border-white/10 px-5 text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/5 text-slate-500"><SearchCheck className="h-6 w-6" /></span>
          <h3 className="mt-4 text-sm font-black text-slate-200">等待调试问题</h3>
          <p className="mt-1 max-w-xs text-xs leading-5 text-slate-500">调试会展示命中证据、相关性和拒绝回答原因。</p>
        </div>
      )}

      {result && (
        <div className="space-y-4" aria-live="polite">
          <div className={`rounded-xl border p-4 ${answerabilityClassByStatus[result.answerability]}`}>
            <div className="flex items-center gap-2 text-sm font-black">
              <AnswerabilityIcon status={result.answerability} />
              {answerabilityLabels[result.answerability]}
            </div>
            <p className="mt-2 text-xs leading-5 opacity-90">{result.explanation}</p>
            {aliasEvidence && (
              <button type="button" onClick={/* addDebugAliasHandler 将当前问题绑定到最高分 FAQ。 */ () => onAddAlias(aliasEvidence, result.query)} disabled={aliasPending} className="mt-3 w-full rounded-lg border border-current/20 bg-white/70 px-3 py-2 text-left text-xs font-extrabold transition-colors hover:bg-white disabled:cursor-not-allowed disabled:opacity-50">
                {aliasPending ? '正在保存相似问法…' : `将“${result.query}”添加为《${aliasEvidence.title}》的相似问法`}
              </button>
            )}
          </div>

          {result.candidate && (
            <div className="rounded-xl border border-white/10 bg-white/5 p-4">
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-xs font-extrabold text-slate-300">候选回复预览</h3>
                <span className="rounded-full bg-white/5 px-2 py-1 text-[10px] font-bold text-slate-500">不会发送</span>
              </div>
              <p className="mt-3 text-sm leading-6 text-slate-100">{result.candidate}</p>
            </div>
          )}

          <div>
            <div className="mb-2 flex items-center justify-between gap-3">
              <h3 className="text-xs font-extrabold text-slate-300">引用证据</h3>
              <span className="text-[10px] font-bold text-slate-500">{result.evidence.length} 条命中</span>
            </div>
            <div className="space-y-2">
              {result.evidence.map(/* evidence 是当前门禁判断引用的一条脱敏知识证据。 */ evidence => (
                <article key={evidence.entryId} className="rounded-xl border border-white/10 bg-white/[0.03] p-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h4 className="truncate text-xs font-black text-white">{evidence.title}</h4>
                      <p className="mt-1 text-[10px] font-bold text-brand-300">{evidence.sourceLabel}</p>
                    </div>
                    <span className="shrink-0 rounded-md bg-success-400/10 px-2 py-1 font-mono text-[10px] font-bold text-success-300">{Math.round(evidence.score * 100)}%</span>
                  </div>
                  <p className="mt-2 line-clamp-3 text-xs leading-5 text-slate-400">{evidence.excerpt}</p>
                  <button type="button" className="mt-2 flex items-center gap-1 text-[10px] font-extrabold text-slate-500" aria-label={`预览来源 ${evidence.title}`}>
                    查看来源 <ExternalLink className="h-3 w-3" />
                  </button>
                </article>
              ))}
              {result.evidence.length === 0 && (
                <div className="flex gap-2 rounded-xl border border-white/10 bg-white/[0.03] px-3 py-3 text-xs leading-5 text-slate-400">
                  <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-warning-400" />
                  无可引用的已审核知识，不应让模型自由补全答案。
                </div>
              )}
            </div>
          </div>

          <div className="flex items-center gap-2 border-t border-white/10 pt-4 text-[10px] font-bold text-slate-500">
            入站消息 <ArrowRight className="h-3 w-3" /> 范围过滤 <ArrowRight className="h-3 w-3" /> 检索 <ArrowRight className="h-3 w-3" /> 门禁
          </div>
        </div>
      )}
    </div>
  </section>
  );
};
