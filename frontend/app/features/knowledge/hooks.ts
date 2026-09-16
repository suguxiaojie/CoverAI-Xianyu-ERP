import { useCallback, useEffect, useRef, useState } from 'react';
import {
  createKnowledgeBase,
  createKnowledgeEntry,
  createKnowledgeFAQAlias,
  deleteKnowledgeFAQAlias,
  deleteKnowledgeEntry,
  getKnowledgeBases,
  getKnowledgeEntries,
  getKnowledgeFAQAliases,
  getKnowledgeFAQAliasSuggestions,
  getKnowledgeScopeOptions,
  retrieveKnowledge,
  reviewKnowledgeEntry,
  setKnowledgeBaseStatus,
  setKnowledgeEntryEnabled,
  updateKnowledgeEntry,
  type KnowledgeBaseCreateInput,
} from './api';
import type { KnowledgeBaseDraft } from './components/KnowledgeBaseDialog';
import type { KnowledgeBasePreview, KnowledgeDraft, KnowledgeEntryPreview, KnowledgeFAQAliasPreview, KnowledgeScopeOption, RetrievalPreview } from './types';

/** UseKnowledgeResult 是知识库页面的服务端数据、短暂 UI 状态与用户动作。 */
export interface UseKnowledgeResult {
  /** knowledgeBases 是当前用户的知识库服务端快照。 */
  knowledgeBases: KnowledgeBasePreview[];
  /** entries 是当前选中知识库的 FAQ／文档服务端快照。 */
  entries: KnowledgeEntryPreview[];
  /** scopeOptions 是当前用户可绑定的用户、店铺和商品范围。 */
  scopeOptions: KnowledgeScopeOption[];
  /** selectedKnowledgeBaseId 是当前工作台选中的知识库标识。 */
  selectedKnowledgeBaseId: string;
  /** loadingBases 表示首页知识库列表是否正在读取。 */
  loadingBases: boolean;
  /** loadingEntries 表示选中知识库的条目是否正在读取。 */
  loadingEntries: boolean;
  /** actionPending 表示创建、审核或启停变更是否正在进行。 */
  actionPending: boolean;
  /** retrieving 表示离线检索请求是否正在进行。 */
  retrieving: boolean;
  /** error 是最近一次列表或变更失败的用户可见文案。 */
  error: string;
  /** notice 是最近一次真实本地 API 变更成功的用户反馈。 */
  notice: string;
  /** retrievalResult 是最近一次离线检索和可回答性结果。 */
  retrievalResult: RetrievalPreview | null;
  /** faqAliases 是当前相似问法管理弹窗读取的服务端列表。 */
  faqAliases: KnowledgeFAQAliasPreview[];
  /** faqAliasSuggestions 是当前 FAQ 尚未落库的确定性建议。 */
  faqAliasSuggestions: string[];
  /** loadingFAQAliases 表示相似问法和建议正在读取。 */
  loadingFAQAliases: boolean;
  /** selectKnowledgeBase 切换当前工作台知识库并触发条目读取。 */
  selectKnowledgeBase: (knowledgeBaseId: string) => void;
  /** reload 取消旧列表请求并重新读取知识库。 */
  reload: () => void;
  /** createBase 创建默认草稿知识库并切换到新资源。 */
  createBase: (draft: KnowledgeBaseDraft) => Promise<boolean>;
  /** toggleBaseStatus 在 draft 和 active 之间切换知识库。 */
  toggleBaseStatus: (knowledgeBase: KnowledgeBasePreview) => Promise<void>;
  /** createEntry 在选中知识库中创建默认待审核条目。 */
  createEntry: (draft: KnowledgeDraft) => Promise<boolean>;
  /** updateEntry 修改 FAQ／文档并刷新其待审核、停用状态。 */
  updateEntry: (entry: KnowledgeEntryPreview, draft: KnowledgeDraft) => Promise<boolean>;
  /** deleteEntry 删除 FAQ／文档及其检索分块。 */
  deleteEntry: (entry: KnowledgeEntryPreview) => Promise<boolean>;
  /** loadFAQAliases 读取指定 FAQ 的已保存问法和系统建议。 */
  loadFAQAliases: (entry: KnowledgeEntryPreview) => Promise<void>;
  /** addFAQAlias 保存用户确认的相似问法。 */
  addFAQAlias: (knowledgeBaseID: string, faqID: string, alias: string, source: KnowledgeFAQAliasPreview['source']) => Promise<boolean>;
  /** deleteFAQAlias 删除单条相似问法。 */
  deleteFAQAlias: (alias: KnowledgeFAQAliasPreview) => Promise<boolean>;
  /** reviewEntry 将人工审核结果保存到本地数据库。 */
  reviewEntry: (entry: KnowledgeEntryPreview) => Promise<void>;
  /** toggleEntry 切换已审核条目的离线检索状态。 */
  toggleEntry: (entry: KnowledgeEntryPreview) => Promise<void>;
  /** runRetrieval 对用户选择的已启用知识库执行离线检索。 */
  runRetrieval: (query: string) => Promise<void>;
}

/** knowledgeErrorMessage 将网络或统一 API 错误转换为页面可见文案。 */
const knowledgeErrorMessage = (error: unknown, fallback: string): string => error instanceof Error && error.message ? error.message : fallback;

/** useKnowledge 拥有知识库页面的请求代次、取消、审核变更和离线检索状态。 */
export const useKnowledge = (): UseKnowledgeResult => {
  /** knowledgeBases 是服务端知识库列表；setKnowledgeBases 只接受当前代次响应。 */
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBasePreview[]>([]);
  /** entries 是选中知识库的服务端条目；setEntries 拒绝旧知识库晚到响应。 */
  const [entries, setEntries] = useState<KnowledgeEntryPreview[]>([]);
  /** scopeOptions 和 setScopeOptions 保存从本地 API 读取的店铺／商品可选范围。 */
  const [scopeOptions, setScopeOptions] = useState<KnowledgeScopeOption[]>([{ value: 'all', label: '全部店铺 · 用户通用', accountIds: [], itemScopes: [] }]);
  /** selectedKnowledgeBaseId 是用户当前选中标识；setSelectedKnowledgeBaseId 触发条目读取。 */
  const [selectedKnowledgeBaseId, setSelectedKnowledgeBaseId] = useState('');
  /** loadingBases 和 setLoadingBases 描述知识库列表读取状态。 */
  const [loadingBases, setLoadingBases] = useState(true);
  /** loadingEntries 和 setLoadingEntries 描述选中知识库条目读取状态。 */
  const [loadingEntries, setLoadingEntries] = useState(false);
  /** actionPending 和 setActionPending 保护创建、审核和启停不被重复触发。 */
  const [actionPending, setActionPending] = useState(false);
  /** retrieving 和 setRetrieving 表示离线检索请求进行中。 */
  const [retrieving, setRetrieving] = useState(false);
  /** error 和 setError 保存当前页面最近一次失败文案。 */
  const [error, setError] = useState('');
  /** notice 和 setNotice 保存最近一次真实本地 API 成功反馈。 */
  const [notice, setNotice] = useState('');
  /** retrievalResult 和 setRetrievalResult 保存最近一次离线检索结果。 */
  const [retrievalResult, setRetrievalResult] = useState<RetrievalPreview | null>(null);
  /** faqAliases 和 setFAQAliases 保存当前 FAQ 的服务端相似问法。 */
  const [faqAliases, setFAQAliases] = useState<KnowledgeFAQAliasPreview[]>([]);
  /** faqAliasSuggestions 和 setFAQAliasSuggestions 保存尚未落库的确定性建议。 */
  const [faqAliasSuggestions, setFAQAliasSuggestions] = useState<string[]>([]);
  /** loadingFAQAliases 和 setLoadingFAQAliases 描述相似问法列表读取状态。 */
  const [loadingFAQAliases, setLoadingFAQAliases] = useState(false);
  /** baseGeneration 隔离知识库列表重载的晚到响应。 */
  const baseGeneration = useRef(0);
  /** baseAbort 保存当前知识库列表请求取消器。 */
  const baseAbort = useRef<AbortController | null>(null);
  /** entryGeneration 隔离快速切换知识库时的旧条目响应。 */
  const entryGeneration = useRef(0);
  /** entryAbort 保存当前条目列表请求取消器。 */
  const entryAbort = useRef<AbortController | null>(null);
  /** actionGeneration 隔离连续变更动作的旧完成回调。 */
  const actionGeneration = useRef(0);
  /** actionAbort 保存当前知识变更请求取消器。 */
  const actionAbort = useRef<AbortController | null>(null);
  /** retrieveGeneration 隔离连续调试问题的旧检索结果。 */
  const retrieveGeneration = useRef(0);
  /** retrieveAbort 保存当前离线检索请求取消器。 */
  const retrieveAbort = useRef<AbortController | null>(null);
  /** aliasGeneration 隔离快速切换 FAQ 时的旧相似问法响应。 */
  const aliasGeneration = useRef(0);
  /** aliasAbort 保存当前相似问法列表请求取消器。 */
  const aliasAbort = useRef<AbortController | null>(null);

  /** loadBases 取消旧列表请求并安全选中保留或首个知识库。 */
  const loadBases = useCallback(/* loadBasesCallback 执行带代次隔离的知识库列表读取。 */ async (preferredID = ''): Promise<void> => {
    // generation 是本次知识库列表请求的唯一代次。
    const generation = ++baseGeneration.current;
    baseAbort.current?.abort();
    // controller 允许刷新或页面卸载时取消列表请求。
    const controller = new AbortController();
    baseAbort.current = controller;
    setLoadingBases(true);
    setError('');
    try {
      // bases 是当前用户最新知识库快照。
      // nextScopeOptions 是当前用户可绑定的店铺和商品范围。
      const [bases, nextScopeOptions] = await Promise.all([getKnowledgeBases({ signal: controller.signal }), getKnowledgeScopeOptions({ signal: controller.signal })]);
      if (generation !== baseGeneration.current || controller.signal.aborted) return;
      setKnowledgeBases(bases);
      setScopeOptions(nextScopeOptions);
      setSelectedKnowledgeBaseId(/* selectedIDUpdater 优先保留明确目标或旧选中项。 */ previousID /* previousID 是列表刷新前的选中知识库。 */ => {
        // requestedID 是新建后的优先目标或旧选中项。
        const requestedID = preferredID || previousID;
        if (bases.some(/* base 是待与优先标识比对的知识库。 */ base => base.id === requestedID)) return requestedID;
        return bases[0]?.id || '';
      });
    } catch (requestError /* requestError 是知识库列表读取失败原因。 */) {
      if (generation === baseGeneration.current && !controller.signal.aborted) setError(knowledgeErrorMessage(requestError, '读取知识库失败'));
    } finally {
      if (generation === baseGeneration.current) setLoadingBases(false);
    }
  }, []);

  /** loadEntries 读取选中知识库条目并拒绝快速切换产生的晚到响应。 */
  const loadEntries = useCallback(/* loadEntriesCallback 执行带取消与代次隔离的条目读取。 */ async (knowledgeBaseID: string): Promise<void> => {
    // generation 是本次条目列表请求的唯一代次。
    const generation = ++entryGeneration.current;
    entryAbort.current?.abort();
    if (!knowledgeBaseID) {
      setEntries([]);
      setLoadingEntries(false);
      return;
    }
    // controller 允许知识库切换或页面卸载时取消条目请求。
    const controller = new AbortController();
    entryAbort.current = controller;
    setLoadingEntries(true);
    setError('');
    try {
      // nextEntries 是选中知识库的最新 FAQ／文档快照。
      const nextEntries = await getKnowledgeEntries(knowledgeBaseID, { signal: controller.signal });
      if (generation === entryGeneration.current && !controller.signal.aborted) setEntries(nextEntries);
    } catch (requestError /* requestError 是知识条目读取失败原因。 */) {
      if (generation === entryGeneration.current && !controller.signal.aborted) setError(knowledgeErrorMessage(requestError, '读取知识条目失败'));
    } finally {
      if (generation === entryGeneration.current) setLoadingEntries(false);
    }
  }, []);

  /** loadFAQAliases 并行读取指定 FAQ 的已保存问法和确定性建议。 */
  const loadFAQAliases = useCallback(/* loadFAQAliasesCallback 执行带取消和代次隔离的相似问法读取。 */ async (entry: KnowledgeEntryPreview): Promise<void> => {
    // generation 是本次相似问法读取的唯一代次。
    const generation = ++aliasGeneration.current;
    aliasAbort.current?.abort();
    // controller 允许切换 FAQ、关闭页面或刷新时取消请求。
    const controller = new AbortController();
    aliasAbort.current = controller;
    setLoadingFAQAliases(true);
    setError('');
    try {
      // aliases 是当前 FAQ 已保存的相似问法。
      // suggestions 是当前 FAQ 尚未保存的确定性建议。
      const [aliases, suggestions] = await Promise.all([getKnowledgeFAQAliases(entry.knowledgeBaseId, entry.id, { signal: controller.signal }), getKnowledgeFAQAliasSuggestions(entry.knowledgeBaseId, entry.id, { signal: controller.signal })]);
      if (generation !== aliasGeneration.current || controller.signal.aborted) return;
      setFAQAliases(aliases);
      setFAQAliasSuggestions(suggestions);
    } catch (requestError /* requestError 是相似问法或建议读取失败原因。 */) {
      if (generation === aliasGeneration.current && !controller.signal.aborted) setError(knowledgeErrorMessage(requestError, '读取相似问法失败'));
    } finally {
      if (generation === aliasGeneration.current) setLoadingFAQAliases(false);
    }
  }, []);

  useEffect(/* baseLoadEffect 在页面挂载时读取知识库，卸载时取消全部请求。 */ () => {
    void loadBases();
    return /* knowledgeCleanup 使所有请求代次失效并取消在途网络操作。 */ () => {
      baseGeneration.current += 1;
      entryGeneration.current += 1;
      actionGeneration.current += 1;
      retrieveGeneration.current += 1;
	  aliasGeneration.current += 1;
      baseAbort.current?.abort();
      entryAbort.current?.abort();
      actionAbort.current?.abort();
      retrieveAbort.current?.abort();
	  aliasAbort.current?.abort();
    };
  }, [loadBases]);

  useEffect(/* entryLoadEffect 在选中知识库变化时读取对应条目。 */ () => {
    void loadEntries(selectedKnowledgeBaseId);
  }, [loadEntries, selectedKnowledgeBaseId]);

  /** runAction 串行一次知识变更，丢弃被新动作取代的旧完成回调。 */
  const runAction = useCallback(/* runActionCallback 串行执行一次可取消的知识变更。 */ async (action: (signal: AbortSignal) => Promise<void>, successNotice: string): Promise<boolean> => {
    // generation 是本次变更动作的唯一代次。
    const generation = ++actionGeneration.current;
    actionAbort.current?.abort();
    // controller 允许新变更或页面卸载时取消当前动作。
    const controller = new AbortController();
    actionAbort.current = controller;
    setActionPending(true);
    setError('');
    setNotice('');
    try {
      await action(controller.signal);
      if (generation !== actionGeneration.current || controller.signal.aborted) return false;
      setNotice(successNotice);
      return true;
    } catch (requestError /* requestError 是当前知识变更失败原因。 */) {
      if (generation === actionGeneration.current && !controller.signal.aborted) setError(knowledgeErrorMessage(requestError, '知识变更失败'));
      return false;
    } finally {
      if (generation === actionGeneration.current) setActionPending(false);
    }
  }, []);

  /** createBase 创建用户级草稿知识库，店铺／商品精确绑定在后续编辑中补充。 */
  const createBase = useCallback(/* createBaseCallback 创建草稿知识库并安全刷新列表。 */ async (draft: KnowledgeBaseDraft): Promise<boolean> => {
    // selectedScope 是用户在弹窗中选择的店铺或商品范围；异常值回退到用户通用。
    const selectedScope = scopeOptions.find(/* option 是当前待与草稿范围标识比对的可选项。 */ option => option.value === draft.scopeValue) || scopeOptions[0];
    // request 是包含结构化店铺／商品范围的新建知识库 HTTP 输入。
    const request: KnowledgeBaseCreateInput = { name: draft.name, description: draft.description, accountIds: selectedScope?.accountIds || [], itemScopes: selectedScope?.itemScopes || [] };
    // createdID 保存变更成功后需要优先选中的新知识库标识。
    let createdID = '';
    // completed 表示创建和后续列表刷新是否完成。
    const completed = await runAction(/* createBaseAction 提交新知识库并读回新列表。 */ async signal /* signal 在新动作或页面卸载时取消创建。 */ => {
      // response 是创建草稿知识库返回的数值主键。
      const response = await createKnowledgeBase(request, { signal });
      createdID = String(response.id);
      await loadBases(createdID);
    }, '草稿知识库已保存到本地数据库。');
    return completed;
  }, [loadBases, runAction, scopeOptions]);

  /** toggleBaseStatus 切换知识库状态并刷新列表。 */
  const toggleBaseStatus = useCallback(/* toggleBaseStatusCallback 切换知识库草稿／启用状态。 */ async (knowledgeBase: KnowledgeBasePreview): Promise<void> => {
    // nextStatus 是用户点击后的草稿或启用状态。
    const nextStatus: KnowledgeBasePreview['status'] = knowledgeBase.status === 'active' ? 'draft' : 'active';
    await runAction(/* toggleBaseStatusAction 提交状态并重新读取当前知识库。 */ async signal /* signal 在新动作或页面卸载时取消状态切换。 */ => {
      await setKnowledgeBaseStatus(knowledgeBase.id, nextStatus, { signal });
      await loadBases(knowledgeBase.id);
    }, nextStatus === 'active' ? '知识库已启用离线检索。' : '知识库已转为草稿。');
  }, [loadBases, runAction]);

  /** createEntry 创建待审核条目并刷新选中知识库。 */
  const createEntry = useCallback(/* createEntryCallback 创建待审核条目并刷新列表。 */ async (draft: KnowledgeDraft): Promise<boolean> => {
    if (!selectedKnowledgeBaseId) return false;
    return runAction(/* createEntryAction 提交新条目并并行刷新条目与计数。 */ async signal /* signal 在新动作或页面卸载时取消创建。 */ => {
      await createKnowledgeEntry(selectedKnowledgeBaseId, draft, { signal });
      await Promise.all([loadEntries(selectedKnowledgeBaseId), loadBases(selectedKnowledgeBaseId)]);
    }, '新知识已保存，当前为待审核且不参与检索。');
  }, [loadBases, loadEntries, runAction, selectedKnowledgeBaseId]);

  /** updateEntry 修改条目后重新读取后端强制重置的审核和启用状态。 */
  const updateEntry = useCallback(/* updateEntryCallback 提交编辑并刷新知识库与条目计数。 */ async (entry: KnowledgeEntryPreview, draft: KnowledgeDraft): Promise<boolean> => {
    // knowledgeBaseID 是被编辑条目所属的知识库标识，避免快速切换时写入错误范围。
    const knowledgeBaseID = entry.knowledgeBaseId;
    return runAction(/* updateEntryAction 更新条目并读回待审核、停用结果。 */ async signal /* signal 在新动作或页面卸载时取消编辑。 */ => {
      await updateKnowledgeEntry(knowledgeBaseID, entry, draft, { signal });
      await Promise.all([loadEntries(knowledgeBaseID), loadBases(knowledgeBaseID)]);
    }, '修改已保存。该知识已转为待审核，并暂时退出离线检索。');
  }, [loadBases, loadEntries, runAction]);

  /** deleteEntry 删除单条知识并清理引用该条目的旧调试结果。 */
  const deleteEntry = useCallback(/* deleteEntryCallback 提交删除并刷新条目和知识库计数。 */ async (entry: KnowledgeEntryPreview): Promise<boolean> => {
    // knowledgeBaseID 是删除目标所属知识库标识。
    const knowledgeBaseID = entry.knowledgeBaseId;
    return runAction(/* deleteEntryAction 删除条目、清理旧证据并读回最新列表。 */ async signal /* signal 在新动作或页面卸载时取消删除。 */ => {
      await deleteKnowledgeEntry(knowledgeBaseID, entry, { signal });
      setRetrievalResult(/* retrievalResultUpdater 仅清空引用了被删条目的旧调试结果。 */ previousResult /* previousResult 是删除前的离线检索结果。 */ => previousResult?.evidence.some(/* evidence 是待检查是否来自被删条目的引用。 */ evidence => evidence.entryId === entry.id) ? null : previousResult);
      await Promise.all([loadEntries(knowledgeBaseID), loadBases(knowledgeBaseID)]);
    }, `${entry.type === 'faq' ? 'FAQ' : '文档'} 已删除，并已退出离线检索。`);
  }, [loadBases, loadEntries, runAction]);

  /** addFAQAlias 保存用户确认的问法，成功后刷新列表和系统建议。 */
  const addFAQAlias = useCallback(/* addFAQAliasCallback 提交相似问法并刷新当前 FAQ 管理数据。 */ async (knowledgeBaseID: string, faqID: string, alias: string, source: KnowledgeFAQAliasPreview['source']): Promise<boolean> => {
    // entry 是相似问法弹窗当前可见时用于刷新列表的 FAQ；调试器可引用其他知识库，届时允许为空。
    const entry = entries.find(/* candidate 是待与目标 FAQ 标识比对的当前条目。 */ candidate => candidate.type === 'faq' && candidate.knowledgeBaseId === knowledgeBaseID && candidate.id === faqID);
    return runAction(/* addFAQAliasAction 保存用户确认问法并刷新管理弹窗。 */ async signal /* signal 在新动作或页面卸载时取消新增。 */ => {
      await createKnowledgeFAQAlias(knowledgeBaseID, faqID, alias, source, { signal });
      if (entry) await loadFAQAliases(entry);
    }, '相似问法已保存，不会修改标准答案或审核状态。');
  }, [entries, loadFAQAliases, runAction]);

  /** deleteFAQAlias 删除单条问法，成功后刷新当前 FAQ 管理数据。 */
  const deleteFAQAlias = useCallback(/* deleteFAQAliasCallback 删除相似问法并刷新列表。 */ async (alias: KnowledgeFAQAliasPreview): Promise<boolean> => {
    // entry 是相似问法所属且当前仍显示的 FAQ。
    const entry = entries.find(/* candidate 是待与相似问法归属比对的 FAQ。 */ candidate => candidate.type === 'faq' && candidate.knowledgeBaseId === alias.knowledgeBaseId && candidate.id === alias.faqId);
    if (!entry) {
      setError('当前 FAQ 已切换或不存在，请重新打开相似问法。');
      return false;
    }
    return runAction(/* deleteFAQAliasAction 删除问法并刷新管理弹窗。 */ async signal /* signal 在新动作或页面卸载时取消删除。 */ => {
      await deleteKnowledgeFAQAlias(alias.knowledgeBaseId, alias.faqId, alias.id, { signal });
      await loadFAQAliases(entry);
    }, '相似问法已删除，标准答案和审核状态保持不变。');
  }, [entries, loadFAQAliases, runAction]);

  /** reviewEntry 将待审核条目确认为权威内容，通过后仍默认停用。 */
  const reviewEntry = useCallback(/* reviewEntryCallback 提交人工审核结果并刷新条目。 */ async (entry: KnowledgeEntryPreview): Promise<void> => {
    if (!selectedKnowledgeBaseId) return;
    await runAction(/* reviewEntryAction 确认权威内容并并行刷新条目与审核计数。 */ async signal /* signal 在新动作或页面卸载时取消审核。 */ => {
      await reviewKnowledgeEntry(selectedKnowledgeBaseId, entry, true, { signal });
      await Promise.all([loadEntries(selectedKnowledgeBaseId), loadBases(selectedKnowledgeBaseId)]);
    }, '知识已通过人工审核，请再明确启用离线检索。');
  }, [loadBases, loadEntries, runAction, selectedKnowledgeBaseId]);

  /** toggleEntry 切换已审核条目的离线检索状态并刷新计数。 */
  const toggleEntry = useCallback(/* toggleEntryCallback 切换已审核条目的离线检索状态。 */ async (entry: KnowledgeEntryPreview): Promise<void> => {
    if (!selectedKnowledgeBaseId) return;
    // nextEnabled 是用户点击后的离线检索状态。
    const nextEnabled = !entry.enabled;
    await runAction(/* toggleEntryAction 提交条目启停并刷新当前列表。 */ async signal /* signal 在新动作或页面卸载时取消启停。 */ => {
      await setKnowledgeEntryEnabled(selectedKnowledgeBaseId, entry, nextEnabled, { signal });
      await loadEntries(selectedKnowledgeBaseId);
    }, nextEnabled ? '知识已启用离线检索。' : '知识已停用，不再进入检索结果。');
  }, [loadEntries, runAction, selectedKnowledgeBaseId]);

  /** runRetrieval 只对当前用户已启用知识库执行可取消的离线检索。 */
  const runRetrieval = useCallback(/* runRetrievalCallback 执行带代次隔离的后端确定性检索。 */ async (query: string): Promise<void> => {
    // generation 是本次离线检索请求的唯一代次。
    const generation = ++retrieveGeneration.current;
    retrieveAbort.current?.abort();
    // controller 允许新问题或页面卸载时取消当前检索。
    const controller = new AbortController();
    retrieveAbort.current = controller;
    setRetrieving(true);
    setError('');
    try {
      // activeKnowledgeBaseIDs 是当前用户明确启用的离线调试范围。
      const activeKnowledgeBaseIDs = knowledgeBases.filter(/* knowledgeBase 是待检查是否启用的知识库。 */ knowledgeBase => knowledgeBase.status === 'active').map(/* knowledgeBase 是待提取调试 ID 的已启用知识库。 */ knowledgeBase => knowledgeBase.id);
      // result 是后端确定性检索和可回答性结果。
      const result = await retrieveKnowledge(query, activeKnowledgeBaseIDs, { signal: controller.signal });
      if (generation === retrieveGeneration.current && !controller.signal.aborted) setRetrievalResult(result);
    } catch (requestError /* requestError 是离线检索失败原因。 */) {
      if (generation === retrieveGeneration.current && !controller.signal.aborted) setError(knowledgeErrorMessage(requestError, '离线检索失败'));
    } finally {
      if (generation === retrieveGeneration.current) setRetrieving(false);
    }
  }, [knowledgeBases]);

  return {
    knowledgeBases,
    entries,
    scopeOptions,
    selectedKnowledgeBaseId,
    loadingBases,
    loadingEntries,
    actionPending,
    retrieving,
    error,
    notice,
    retrievalResult,
    faqAliases,
    faqAliasSuggestions,
    loadingFAQAliases,
    selectKnowledgeBase: setSelectedKnowledgeBaseId,
    reload: /* reloadKnowledgeBases 由错误空态和用户手动刷新触发。 */ () => void loadBases(),
    createBase,
    toggleBaseStatus,
    createEntry,
    updateEntry,
    deleteEntry,
    loadFAQAliases,
    addFAQAlias,
    deleteFAQAlias,
    reviewEntry,
    toggleEntry,
    runRetrieval,
  };
};
