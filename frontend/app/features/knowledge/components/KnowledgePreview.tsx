import React, { useState } from 'react';
import { AlertTriangle, BookOpenCheck, BookPlus, CheckCircle2, CircleSlash2, FlaskConical, RefreshCw, ShieldCheck, Sparkles } from 'lucide-react';
import { KnowledgeBaseDialog, type KnowledgeBaseDraft } from './KnowledgeBaseDialog';
import { KnowledgeBaseRail } from './KnowledgeBaseRail';
import { KnowledgeAliasDialog } from './KnowledgeAliasDialog';
import { KnowledgeContentWorkspace } from './KnowledgeContentWorkspace';
import { KnowledgeDeleteDialog } from './KnowledgeDeleteDialog';
import { KnowledgeDraftDialog } from './KnowledgeDraftDialog';
import { RetrievalDebugPanel } from './RetrievalDebugPanel';
import { useKnowledge } from '../hooks';
import { retrievalExampleQueries } from '../previewData';
import { entriesForKnowledgeBase, filterKnowledgeBases } from '../state';
import type { KnowledgeContentType, KnowledgeDraft, KnowledgeEntryPreview, KnowledgeFAQAliasPreview, RetrievalEvidencePreview } from '../types';

/** emptyKnowledgeDraft 是每次打开新增条目弹窗时的默认待审核表单。 */
const emptyKnowledgeDraft: KnowledgeDraft = {
  type: 'faq',
  title: '',
  content: '',
  scopeLabel: '继承当前知识库范围',
  riskLevel: 'low',
  requiresLiveData: false,
};

/** emptyKnowledgeBaseDraft 是新建用户通用草稿知识库的默认表单。 */
const emptyKnowledgeBaseDraft: KnowledgeBaseDraft = {
  name: '',
  description: '',
  scopeValue: 'all',
};

/** KnowledgeEmptyWorkspaceProps 描述未创建知识库时的引导动作。 */
interface KnowledgeEmptyWorkspaceProps {
  /** onCreate 打开创建第一个草稿知识库的弹窗。 */
  onCreate: () => void;
}

/** KnowledgeEmptyWorkspace 引导用户创建第一个人工知识库。 */
const KnowledgeEmptyWorkspace: React.FC<KnowledgeEmptyWorkspaceProps> = ({ onCreate }) => (
  <section className="flex min-h-[620px] flex-col items-center justify-center rounded-2xl border border-dashed border-slate-300 bg-white px-6 text-center shadow-card" aria-label="知识内容空状态">
    <span className="flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-50 text-brand-700"><BookPlus className="h-7 w-7" /></span>
    <h2 className="mt-5 text-xl font-black text-slate-950">还没有知识库</h2>
    <p className="mt-2 max-w-md text-sm leading-6 text-slate-500">先创建一个草稿知识库，再手工维护 FAQ 或 Markdown 文档。未审核内容不会进入检索。</p>
    <button type="button" onClick={onCreate} className="ios-btn-primary mt-5 rounded-xl px-5 py-2.5 text-sm">创建第一个知识库</button>
  </section>
);

/** KnowledgePreview 管理人工 FAQ／文档、审核门禁和不调用模型的离线检索调试。 */
const KnowledgePreview: React.FC = () => {
  // knowledgeState 拥有所有服务端列表、请求取消、变更动作和离线检索结果。
  const knowledgeState = useKnowledge();
  /** knowledgeBaseSearch 是左侧知识库的短暂搜索词；setKnowledgeBaseSearch 不发起网络请求。 */
  const [knowledgeBaseSearch, setKnowledgeBaseSearch] = useState('');
  /** activeType 是中间工作台的 FAQ／文档分类；setActiveType 由页签操作更新。 */
  const [activeType, setActiveType] = useState<KnowledgeContentType>('faq');
  /** retrievalQuery 是右侧离线检索的短暂问题；setRetrievalQuery 只更新表单。 */
  const [retrievalQuery, setRetrievalQuery] = useState('');
  /** entryDialogOpen 和 setEntryDialogOpen 控制新增 FAQ／文档弹窗。 */
  const [entryDialogOpen, setEntryDialogOpen] = useState(false);
  /** entryDraft 和 setEntryDraft 保存尚未提交的 FAQ／文档表单。 */
  const [entryDraft, setEntryDraft] = useState<KnowledgeDraft>(emptyKnowledgeDraft);
  /** editingEntry 和 setEditingEntry 保存当前正在编辑的服务端条目；null 表示新增模式。 */
  const [editingEntry, setEditingEntry] = useState<KnowledgeEntryPreview | null>(null);
  /** deletingEntry 和 setDeletingEntry 保存等待用户确认删除的 FAQ／文档。 */
  const [deletingEntry, setDeletingEntry] = useState<KnowledgeEntryPreview | null>(null);
  /** aliasEntry 和 setAliasEntry 保存当前正在管理相似问法的 FAQ。 */
  const [aliasEntry, setAliasEntry] = useState<KnowledgeEntryPreview | null>(null);
  /** knowledgeBaseDialogOpen 和 setKnowledgeBaseDialogOpen 控制新建草稿知识库弹窗。 */
  const [knowledgeBaseDialogOpen, setKnowledgeBaseDialogOpen] = useState(false);
  /** knowledgeBaseDraft 和 setKnowledgeBaseDraft 保存尚未提交的知识库表单。 */
  const [knowledgeBaseDraft, setKnowledgeBaseDraft] = useState<KnowledgeBaseDraft>(emptyKnowledgeBaseDraft);

  // selectedKnowledgeBase 是与服务端选中标识对应的知识库。
  const selectedKnowledgeBase = knowledgeState.knowledgeBases.find(/* knowledgeBase 是待与选中 ID 比对的知识库。 */ knowledgeBase => knowledgeBase.id === knowledgeState.selectedKnowledgeBaseId);
  // visibleKnowledgeBases 是按用户本地搜索词过滤的知识库列表。
  const visibleKnowledgeBases = filterKnowledgeBases(knowledgeState.knowledgeBases, knowledgeBaseSearch);
  // visibleEntries 是当前知识库和内容类型下的条目，范围继承自知识库。
  const visibleEntries = selectedKnowledgeBase ? entriesForKnowledgeBase(knowledgeState.entries, selectedKnowledgeBase.id, activeType).map(/* entry 是当前补齐知识库范围文案的条目。 */ entry => ({ ...entry, scopeLabel: selectedKnowledgeBase.scopeLabel })) : [];
  // totalEntryCount 是当前用户所有知识库的 FAQ／文档合计数。
  const totalEntryCount = knowledgeState.knowledgeBases.reduce(/* total 是已累加条目数；knowledgeBase 是当前待累加的知识库。 */ (total, knowledgeBase) => total + knowledgeBase.entryCount, 0);
  // totalReviewedCount 是当前用户所有知识库的已审核条目数。
  const totalReviewedCount = knowledgeState.knowledgeBases.reduce(/* total 是已累加审核数；knowledgeBase 是当前待累加的知识库。 */ (total, knowledgeBase) => total + knowledgeBase.reviewedCount, 0);

  /** handleSelectKnowledgeBase 切换服务端知识库并恢复 FAQ 视图。 */
  const handleSelectKnowledgeBase = (knowledgeBaseID: string): void => {
    setActiveType('faq');
    knowledgeState.selectKnowledgeBase(knowledgeBaseID);
  };

  /** handleOpenEntryDialog 根据当前 FAQ／文档页签初始化新增表单。 */
  const handleOpenEntryDialog = (): void => {
    if (!selectedKnowledgeBase) return;
    setEditingEntry(null);
    setEntryDraft({ ...emptyKnowledgeDraft, type: activeType, scopeLabel: `继承：${selectedKnowledgeBase.scopeLabel}` });
    setEntryDialogOpen(true);
  };

  /** handleEditEntry 使用现有条目完整回填编辑弹窗，并保持所属知识库范围只读。 */
  const handleEditEntry = (entry: KnowledgeEntryPreview): void => {
    setEditingEntry(entry);
    setEntryDraft({ type: entry.type, title: entry.title, content: entry.content, scopeLabel: entry.scopeLabel, riskLevel: entry.riskLevel, requiresLiveData: entry.requiresLiveData });
    setEntryDialogOpen(true);
  };

  /** handleCloseEntryDialog 放弃当前新增或编辑草稿并恢复空表单。 */
  const handleCloseEntryDialog = (): void => {
    setEntryDialogOpen(false);
    setEditingEntry(null);
    setEntryDraft(emptyKnowledgeDraft);
  };

  /** handleConfirmDeleteEntry 删除当前目标，只在服务端成功后关闭确认弹窗。 */
  const handleConfirmDeleteEntry = async (): Promise<void> => {
    if (!deletingEntry) return;
    // deleted 表示条目、分块和列表刷新是否全部完成。
    const deleted = await knowledgeState.deleteEntry(deletingEntry);
    if (!deleted) return;
    setDeletingEntry(null);
  };

  /** handleOpenAliases 打开 FAQ 相似问法弹窗并读取已保存问法和建议。 */
  const handleOpenAliases = (entry: KnowledgeEntryPreview): void => {
    setAliasEntry(entry);
    void knowledgeState.loadFAQAliases(entry);
  };

  /** handleAddAlias 保存弹窗内用户确认的相似问法。 */
  const handleAddAlias = (alias: string, source: KnowledgeFAQAliasPreview['source']): Promise<boolean> => {
    if (!aliasEntry) return Promise.resolve(false);
    return knowledgeState.addFAQAlias(aliasEntry.knowledgeBaseId, aliasEntry.id, alias, source);
  };

  /** handleAddAliasFromDebug 将当前调试问题明确绑定到最高分 FAQ 并重新检索。 */
  const handleAddAliasFromDebug = async (evidence: RetrievalEvidencePreview, query: string): Promise<void> => {
    // added 表示当前问题已通过冲突校验并作为调试来源保存。
    const added = await knowledgeState.addFAQAlias(evidence.knowledgeBaseId, evidence.entryId, query, 'debug');
    if (added) await knowledgeState.runRetrieval(query);
  };

  /** handleSaveEntry 创建或更新条目，只在服务端成功并刷新状态后关闭弹窗。 */
  const handleSaveEntry = async (): Promise<void> => {
    // saved 表示创建或更新 FAQ／文档及列表刷新是否完成。
    const saved = editingEntry ? await knowledgeState.updateEntry(editingEntry, entryDraft) : await knowledgeState.createEntry(entryDraft);
    if (!saved) return;
    handleCloseEntryDialog();
  };

  /** handleSaveKnowledgeBase 保存草稿知识库，只在服务端成功后关闭弹窗。 */
  const handleSaveKnowledgeBase = async (): Promise<void> => {
    // saved 表示创建草稿知识库及列表刷新是否完成。
    const saved = await knowledgeState.createBase(knowledgeBaseDraft);
    if (!saved) return;
    setKnowledgeBaseDialogOpen(false);
    setKnowledgeBaseDraft(emptyKnowledgeBaseDraft);
  };

  /** handleRunRetrieval 对当前问题执行后端确定性离线检索。 */
  const handleRunRetrieval = (): void => {
    void knowledgeState.runRetrieval(retrievalQuery);
  };

  /** handleUseExample 填入脱敏示例并立即执行后端离线检索。 */
  const handleUseExample = (query: string): void => {
    setRetrievalQuery(query);
    void knowledgeState.runRetrieval(query);
  };

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div className="flex flex-wrap items-center gap-3">
          <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-brand text-white shadow-brand-active"><BookOpenCheck className="h-5 w-5" /></span>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-3xl font-black tracking-tight text-slate-950">知识库</h1>
              <span className="rounded-full border border-info-200 bg-info-50 px-2.5 py-1 text-[11px] font-extrabold text-info-700">第一阶段 · 离线检索</span>
            </div>
            <p className="mt-1.5 text-sm leading-6 text-slate-500">手工维护权威 FAQ 与文档，先完成审核和可回答性验证，再考虑在线聊天。</p>
          </div>
        </div>
        <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-3 text-xs font-bold text-slate-500 shadow-sm">
          <CircleSlash2 className="h-4 w-4 text-danger-500" />
          已接本地 API · 未调模型 · 不会发送消息
        </div>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="知识库本地统计">
        <div className="rounded-2xl border border-slate-200/80 bg-white p-4 shadow-card"><div className="flex items-center justify-between"><span className="text-xs font-extrabold text-slate-400">知识库</span><BookOpenCheck className="h-4 w-4 text-brand-500" /></div><div className="mt-3 text-2xl font-black text-slate-950">{knowledgeState.loadingBases ? '—' : knowledgeState.knowledgeBases.length}</div><div className="mt-1 text-xs text-slate-500">包含草稿范围</div></div>
        <div className="rounded-2xl border border-slate-200/80 bg-white p-4 shadow-card"><div className="flex items-center justify-between"><span className="text-xs font-extrabold text-slate-400">知识条目</span><Sparkles className="h-4 w-4 text-brand-500" /></div><div className="mt-3 text-2xl font-black text-slate-950">{knowledgeState.loadingBases ? '—' : totalEntryCount}</div><div className="mt-1 text-xs text-slate-500">FAQ 与纯文本文档</div></div>
        <div className="rounded-2xl border border-slate-200/80 bg-white p-4 shadow-card"><div className="flex items-center justify-between"><span className="text-xs font-extrabold text-slate-400">已审核</span><CheckCircle2 className="h-4 w-4 text-success-500" /></div><div className="mt-3 text-2xl font-black text-slate-950">{knowledgeState.loadingBases ? '—' : totalReviewedCount}</div><div className="mt-1 text-xs text-slate-500">只有审核内容可检索</div></div>
        <div className="rounded-2xl border border-slate-200/80 bg-white p-4 shadow-card"><div className="flex items-center justify-between"><span className="text-xs font-extrabold text-slate-400">在线自动回复</span><ShieldCheck className="h-4 w-4 text-slate-400" /></div><div className="mt-3 text-2xl font-black text-slate-950">0</div><div className="mt-1 text-xs text-slate-500">第一阶段后端固定关闭</div></div>
      </section>

      <div className="flex items-start gap-3 rounded-2xl border border-info-200 bg-info-50 px-4 py-3 text-sm text-info-900">
        <FlaskConical className="mt-0.5 h-5 w-5 shrink-0" />
        <div className="min-w-0 flex-1"><div className="font-black">本地安全边界</div><p className="mt-1 leading-6">新知识默认待审核且停用；检索日志只保存问题 SHA-256 摘要，不保存问题明文。</p></div>
      </div>

      {knowledgeState.error && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-danger-200 bg-danger-50 px-4 py-3 text-sm font-bold text-danger-800" role="alert">
          <span className="flex items-center gap-2"><AlertTriangle className="h-4 w-4" />{knowledgeState.error}</span>
          <button type="button" onClick={knowledgeState.reload} className="flex items-center gap-1.5 rounded-lg bg-white px-3 py-1.5 text-xs font-extrabold text-danger-700"><RefreshCw className="h-3.5 w-3.5" />重试</button>
        </div>
      )}
      {knowledgeState.notice && <div className="flex items-center gap-2 rounded-xl border border-success-200 bg-success-50 px-4 py-3 text-sm font-bold text-success-800" role="status"><CheckCircle2 className="h-4 w-4" />{knowledgeState.notice}</div>}

      <div className="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)] 2xl:grid-cols-[280px_minmax(0,1fr)_390px]">
        <KnowledgeBaseRail
          knowledgeBases={visibleKnowledgeBases}
          selectedId={selectedKnowledgeBase?.id || ''}
          searchQuery={knowledgeBaseSearch}
          onSearchQueryChange={setKnowledgeBaseSearch}
          onSelect={handleSelectKnowledgeBase}
          onCreate={/* createKnowledgeBaseHandler 打开创建用户通用草稿知识库弹窗。 */ () => setKnowledgeBaseDialogOpen(true)}
        />
        {selectedKnowledgeBase ? (
          <KnowledgeContentWorkspace
            knowledgeBase={selectedKnowledgeBase}
            activeType={activeType}
            entries={visibleEntries}
            onTypeChange={setActiveType}
            onCreateEntry={handleOpenEntryDialog}
            onEditEntry={handleEditEntry}
            onDeleteEntry={setDeletingEntry}
            onManageAliases={handleOpenAliases}
            onToggleEntry={/* toggleEntryHandler 将已审核条目的离线检索状态变更交给 Hook。 */ entry => void knowledgeState.toggleEntry(entry)}
            onReviewEntry={/* reviewEntryHandler 将待审核条目的人工确认交给 Hook。 */ entry => void knowledgeState.reviewEntry(entry)}
            onToggleBaseStatus={/* toggleBaseStatusHandler 在草稿与启用状态之间切换当前知识库。 */ () => void knowledgeState.toggleBaseStatus(selectedKnowledgeBase)}
            actionPending={knowledgeState.actionPending}
            loading={knowledgeState.loadingEntries}
          />
        ) : <KnowledgeEmptyWorkspace onCreate={/* emptyCreateHandler 从空态打开新建知识库弹窗。 */ () => setKnowledgeBaseDialogOpen(true)} />}
        <div className="min-w-0 xl:col-span-2 2xl:col-span-1">
          <RetrievalDebugPanel
            query={retrievalQuery}
            exampleQueries={retrievalExampleQueries}
            result={knowledgeState.retrievalResult}
            onQueryChange={setRetrievalQuery}
            onRun={handleRunRetrieval}
            onUseExample={handleUseExample}
            onAddAlias={/* debugAliasHandler 将用户当前调试问题绑定到明确 FAQ。 */ (evidence, query) => void handleAddAliasFromDebug(evidence, query)}
            aliasPending={knowledgeState.actionPending}
            loading={knowledgeState.retrieving}
          />
        </div>
      </div>

      <KnowledgeDraftDialog
        open={entryDialogOpen}
        knowledgeBaseName={selectedKnowledgeBase?.name || ''}
        mode={editingEntry ? 'edit' : 'create'}
        resetReviewOnSave={Boolean(editingEntry?.reviewed || editingEntry?.enabled)}
        draft={entryDraft}
        onDraftChange={setEntryDraft}
        onClose={handleCloseEntryDialog}
        onSavePreview={handleSaveEntry}
      />
      <KnowledgeBaseDialog
        open={knowledgeBaseDialogOpen}
        draft={knowledgeBaseDraft}
        onDraftChange={setKnowledgeBaseDraft}
        scopeOptions={knowledgeState.scopeOptions}
        onClose={/* knowledgeBaseDialogCloseHandler 只在未提交或保存成功后关闭知识库弹窗。 */ () => setKnowledgeBaseDialogOpen(false)}
        onSavePreview={handleSaveKnowledgeBase}
      />
      <KnowledgeDeleteDialog
        open={Boolean(deletingEntry)}
        entry={deletingEntry}
        pending={knowledgeState.actionPending}
        onClose={/* deleteDialogCloseHandler 取消删除并保留正式知识内容。 */ () => setDeletingEntry(null)}
        onConfirm={/* deleteDialogConfirmHandler 执行当前确认目标的正式删除。 */ () => void handleConfirmDeleteEntry()}
      />
      <KnowledgeAliasDialog
        open={Boolean(aliasEntry)}
        entry={aliasEntry}
        aliases={knowledgeState.faqAliases}
        suggestions={knowledgeState.faqAliasSuggestions}
        loading={knowledgeState.loadingFAQAliases}
        pending={knowledgeState.actionPending}
        onClose={/* aliasDialogCloseHandler 关闭相似问法弹窗且不改变 FAQ。 */ () => setAliasEntry(null)}
        onAdd={handleAddAlias}
        onDelete={knowledgeState.deleteFAQAlias}
      />
    </div>
  );
};

export default KnowledgePreview;
