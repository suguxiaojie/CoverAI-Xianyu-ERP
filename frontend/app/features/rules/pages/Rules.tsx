import {
AlertCircle,
Bot,
CheckCircle2,
ChevronLeft,
ChevronRight,
Clock3,
Edit,
Layers3,
MessageCircle,
Plus,
RefreshCw,
Save,
Search,
Send,
SlidersHorizontal,
Trash2,
X,
Zap,
} from 'lucide-react';
import React,{ useEffect,useMemo,useState } from 'react';
import { createPortal } from 'react-dom';
import { AutomationIssuePanel } from '../components/AutomationIssuePanel';
import { setAllReplyRulesEnabled,setReplyRuleAccountEnabled,setReplyRuleEnabled } from '../api';
import { useRulesData } from '../hooks';
import { filterAutomationIssues } from '../issueState';
import { useRuleActions } from '../ruleActions';
import type { AutomationTriggerType,ReplyRule,RulesProps,RulesTab } from '../types';
import { accentClasses,accountLabel,actionSummary,buildReviewConfig,cardActionsForTrigger,statusPill,triggerMeta,triggerOrder } from '../utils';

/** 按逗号、中文逗号、顿号、分号和换行解析一批关键词，并保持首次出现顺序。 */
export const parseKeywordBatch = (raw: string): string[] => {
  // seen 保存已经解析的小写关键词，避免重复气泡。
  const seen = new Set<string>();
  // tokens 保存去空白、去重后的关键词。
  const tokens: string[] = [];
  raw.split(/[,，、;；\r\n]+/).forEach(/* value 是当前分隔得到的关键词候选。 */ value => {
    // token 是去除首尾空白后的关键词。
    const token = value.trim();
    // key 是用于不区分英文大小写去重的索引。
    const key = token.toLowerCase();
    if (!token || seen.has(key)) return;
    seen.add(key);
    tokens.push(token);
  });
  return tokens;
};

/** 系统关键词回复允许配置的稳定事件，按业务类别完整展示。 */
const SYSTEM_EVENT_GROUPS = [
  { title: '订单交易', events: [
    { value: 'order_pending_payment', label: '买家已拍下，待付款' }, { value: 'order_paid', label: '买家已付款，等待发货' },
    { value: 'order_price_adjusted', label: '订单价格已调整' }, { value: 'order_shipped', label: '订单已发货' },
    { value: 'order_received', label: '买家已确认收货' }, { value: 'order_completed', label: '交易完成' },
  ] },
  { title: '售后退款', events: [{ value: 'refund_requested', label: '买家申请退款' }] },
  { title: '小红花', events: [
    { value: 'red_flower_requested', label: '已向买家请求小红花' }, { value: 'red_flower_prompted', label: '平台提示可以送花' },
    { value: 'red_flower_sent', label: '买家已送出小红花' }, { value: 'red_flower_received', label: '已收到小红花' },
  ] },
];

/** replyAccountStates 返回规则绑定店铺的独立状态，并兼容旧接口统一状态。 */
const replyAccountStates = (rule: ReplyRule): Array<{ /** account_id 是绑定店铺账号。 */ account_id: string; /** enabled 表示店铺是否执行规则。 */ enabled: boolean }> =>
  rule.account_states?.length ? rule.account_states : (rule.account_ids || []).map(/* accountID 是兼容旧响应的绑定店铺。 */ accountID => ({ account_id: accountID, enabled: rule.enabled }));

/** replyRuleStateLabel 根据店铺状态返回规则整体状态文案。 */
const replyRuleStateLabel = (rule: ReplyRule): '已开启' | '已关闭' | '部分开启' => {
  // states 是规则全部绑定店铺的独立启用状态。
  const states = replyAccountStates(rule);
  if (states.length > 0 && states.every(/* state 是当前检查的店铺状态。 */ state => state.enabled)) return '已开启';
  if (states.some(/* state 是当前检查的店铺状态。 */ state => state.enabled)) return '部分开启';
  return '已关闭';
};

// Rules 是规则 feature 在旧页面目录下保留的兼容入口组件。
const Rules: React.FC<RulesProps> = ({ initialDeliveryTarget, onDeliveryTargetHandled }) => {
  // [activeTab, 解构得到当前 Hook 返回的状态和操作函数。
  const [activeTab, setActiveTab] = useState<RulesTab>('automation');
  // [selectedAccountId, 解构得到当前 Hook 返回的状态和操作函数。
  const [selectedAccountId, setSelectedAccountId] = useState('');
  // keywordDraft 保存尚未提交为气泡的关键词输入文本。
  const [keywordDraft, setKeywordDraft] = useState('');
  // keywordParseNotice 保存最近一次批量解析成功提示。
  const [keywordParseNotice, setKeywordParseNotice] = useState('');
  // deleteReplyTarget 保存等待应用内二次确认删除的关键词规则组。
  const [deleteReplyTarget, setDeleteReplyTarget] = useState<ReplyRule | null>(null);
  // replyTogglePending 保存正在提交单条开关的规则组标识，避免重复点击覆盖状态。
  const [replyTogglePending, setReplyTogglePending] = useState<Set<string>>(new Set());
  // replyBulkPending 表示全部关键词规则开关正在提交。
  const [replyBulkPending, setReplyBulkPending] = useState(false);
  // replyAccountTogglePending 保存正在提交的规则与店铺组合键。
  const [replyAccountTogglePending, setReplyAccountTogglePending] = useState<Set<string>>(new Set());
  // [automationSearch, 解构得到当前 Hook 返回的状态和操作函数。
  const [automationSearch, setAutomationSearch] = useState('');
  // [debouncedAutomationSearch, 解构得到当前 Hook 返回的状态和操作函数。
  const [debouncedAutomationSearch, setDebouncedAutomationSearch] = useState('');
  // [automationTriggerFilter, 解构得到当前 Hook 返回的状态和操作函数。
  const [automationTriggerFilter, setAutomationTriggerFilter] = useState<AutomationTriggerType | ''>('');
  // [automationStatusFilter, 解构得到当前 Hook 返回的状态和操作函数。
  const [automationStatusFilter, setAutomationStatusFilter] = useState<'all' | 'enabled' | 'disabled'>('all');
  // [automationPage, 解构得到当前 Hook 返回的状态和操作函数。
  const [automationPage, setAutomationPage] = useState(1);
  // [automationPageSize, 解构得到当前 Hook 返回的状态和操作函数。
  const [automationPageSize, setAutomationPageSize] = useState(10);

  // rulesData 规则列表数据，负责当前功能中的对应处理。
  const rulesData = useRulesData({
    activeTab,
    selectedAccountId,
    automationTriggerFilter,
    automationStatusFilter,
    debouncedAutomationSearch,
    automationPage,
    automationPageSize,
    setSelectedAccountId,
    onAutomationPageChange: setAutomationPage,
  });
  // 解构数据 解构得到当前 Hook 返回的状态和操作函数。
  const {
    automationRules,
    automationIssues,
    replyRules,
    defaultReplies,
    accounts,
    cards,
    items,
    loading,
    setLoading,
    automationTotal,
    automationTotalPages,
    automationTriggerCounts,
    setAutomationRules,
    setCards,
    setItems,
    loadReferenceData,
    loadAutomationRules,
    loadReplyRules,
    loadDefaultReplies,
    refresh,
  } = rulesData;

  // ruleActions 规则 feature 提供弹窗状态、编辑草稿和所有保存删除动作。
  const ruleActions = useRuleActions({
    selectedAccountId,
    setSelectedAccountId,
    setActiveTab,
    items,
    accounts,
    setAutomationRules,
    setCards,
    setItems,
    setLoading,
    loadAutomationRules,
    loadReferenceData,
    loadReplyRules,
    loadDefaultReplies,
    initialDeliveryTarget,
    onDeliveryTargetHandled,
  });
  // 解构规则动作，保持旧页面 JSX 的字段名称和行为不变。
  const {
    showAutomationModal, setShowAutomationModal, showReplyModal, setShowReplyModal, showDefaultModal, setShowDefaultModal,
    replyFeedback, setReplyFeedback,
    editingAutomationRule, setEditingAutomationRule,
    editingReplyRule, setEditingReplyRule, defaultForm, setDefaultForm, selectedRuleItem, isMultiSpecRule, currentTrigger,
    currentMeta, reviewConfig, displayVariants, openAutomationRule, openNewAutomationRule, handleTriggerChange,
    handleAutomationItemChange, updateVariant, appendDeliveryContent, handleSaveAutomationRule, handleDeleteAutomation,
    handleToggleAutomation, handleResolveRunIssue, handleResolveDeferredIssue, handleAddReplyRule, handleSaveReplyRule,
    handleDeleteReply, openDefaultReplyModal, handleSaveDefaultReply, handleDeleteDefaultReply, handleClearDefaultReplyRecords,
  } = ruleActions;

  // commitKeywordText 把输入或粘贴文本解析成关键词气泡并合并到当前规则组。
  const commitKeywordText = (raw: string) => {
    if (!editingReplyRule) return;
    // parsed 是本次从分隔文本识别的关键词。
    const parsed = parseKeywordBatch(raw);
    if (parsed.length === 0) return;
    // existing 是当前规则组已经形成气泡的关键词。
    const existing = editingReplyRule.keywords || [];
    // merged 是不区分英文大小写去重后的完整关键词集合。
    const merged = parseKeywordBatch([...existing, ...parsed].join('、'));
    setEditingReplyRule({ ...editingReplyRule, keywords: merged, keyword: merged.join('、') });
    setKeywordDraft('');
    setKeywordParseNotice(`已识别 ${parsed.length} 个关键词，共 ${merged.length} 个`);
  };

  // removeKeywordChip 从当前规则组删除一个关键词气泡。
  const removeKeywordChip = (keyword: string) => {
    if (!editingReplyRule) return;
    // remaining 是删除目标后的关键词集合。
    const remaining = (editingReplyRule.keywords || []).filter(/* item 是当前保留的关键词。 */ item => item !== keyword);
    setEditingReplyRule({ ...editingReplyRule, keywords: remaining, keyword: remaining.join('、') });
    setKeywordParseNotice(remaining.length > 0 ? `当前共 ${remaining.length} 个关键词` : '请继续添加关键词');
  };

  // totalReplyBindings 是全部规则绑定店铺的执行关系总数。
  const totalReplyBindings = replyRules.reduce(/* bindingTotal 累加每条规则的绑定店铺数量。 */ (total, rule) => total + replyAccountStates(rule).length, 0);
  // enabledReplyBindings 是当前实际开启的规则店铺关系数量。
  const enabledReplyBindings = replyRules.reduce(/* enabledTotal 累加开启店铺数量。 */ (total, rule) => total + replyAccountStates(rule).filter(/* state 是当前检查的店铺状态。 */ state => state.enabled).length, 0);
  // allReplyRulesEnabled 表示全部规则的全部绑定店铺是否都已开启。
  const allReplyRulesEnabled = totalReplyBindings > 0 && enabledReplyBindings === totalReplyBindings;
  // someReplyRulesEnabled 表示店铺规则绑定同时存在开启和关闭状态。
  const someReplyRulesEnabled = enabledReplyBindings > 0 && !allReplyRulesEnabled;

  // handleToggleReplyRule 响应单条规则开关，成功后刷新列表并展示应用内气泡反馈。
  const handleToggleReplyRule = async (rule: ReplyRule): Promise<void> => {
    if (replyBulkPending || replyTogglePending.has(rule.id)) return;
    // targetEnabled 是本次单规则切换希望保存的目标状态。
    const targetEnabled = !rule.enabled;
    setReplyTogglePending(/* current 是正在提交的规则组集合。 */ current => new Set(current).add(rule.id));
    try {
      await setReplyRuleEnabled(rule.id, targetEnabled);
      await loadReplyRules();
      setReplyFeedback({ kind: 'success', message: targetEnabled ? '该自动回复已开启' : '该自动回复已关闭' });
    } catch (/* error 是单规则启停请求或刷新失败。 */ error) {
      setReplyFeedback({ kind: 'error', message: `${targetEnabled ? '开启' : '关闭'}失败：${(error as Error).message}` });
    } finally {
      setReplyTogglePending(/* current 是提交结束前的规则组集合。 */ current => {
        // next 删除已完成规则后作为下一次渲染的等待集合。
        const next = new Set(current);
        next.delete(rule.id);
        return next;
      });
    }
  };

  // handleToggleAllReplyRules 响应顶部总开关；部分开启时统一开启全部规则。
  const handleToggleAllReplyRules = async (): Promise<void> => {
    if (replyRules.length === 0 || replyBulkPending || replyTogglePending.size > 0) return;
    // targetEnabled 是总开关根据当前全开状态计算出的目标状态。
    const targetEnabled = !allReplyRulesEnabled;
    setReplyBulkPending(true);
    try {
      await setAllReplyRulesEnabled(targetEnabled);
      await loadReplyRules();
      setReplyFeedback({ kind: 'success', message: targetEnabled ? '全部自动回复已开启' : '全部自动回复已关闭' });
    } catch (/* error 是全部规则启停请求或刷新失败。 */ error) {
      setReplyFeedback({ kind: 'error', message: `${targetEnabled ? '全部开启' : '全部关闭'}失败：${(error as Error).message}` });
    } finally {
      setReplyBulkPending(false);
    }
  };

  // handleToggleReplyAccount 切换一条规则在单个店铺中的状态并显示气泡反馈。
  const handleToggleReplyAccount = async (rule: ReplyRule, accountID: string, enabled: boolean): Promise<void> => {
    // pendingKey 是规则和店铺组合形成的提交锁键。
    const pendingKey = `${rule.id}:${accountID}`;
    // storeLabel 是气泡反馈中明确指出的店铺展示名称。
    const storeLabel = accountLabel(accounts.find(/* account 是当前切换状态的店铺。 */ account => account.id === accountID) || { id: accountID } as never);
    if (replyBulkPending || replyTogglePending.has(rule.id) || replyAccountTogglePending.has(pendingKey)) return;
    setReplyAccountTogglePending(/* current 是当前店铺提交锁集合。 */ current => new Set(current).add(pendingKey));
    try {
      await setReplyRuleAccountEnabled(rule.id, accountID, enabled);
      await loadReplyRules();
      setReplyFeedback({ kind: 'success', message: `${storeLabel}：该规则已${enabled ? '开启' : '关闭'}` });
    } catch (/* error 是店铺级启停请求或刷新失败。 */ error) {
      setReplyFeedback({ kind: 'error', message: `${storeLabel} ${enabled ? '开启' : '关闭'}失败：${(error as Error).message}` });
    } finally {
      setReplyAccountTogglePending(/* current 是提交结束前的店铺锁集合。 */ current => {
        // next 是移除已完成组合键后的新集合。
        const next = new Set(current);
        next.delete(pendingKey);
        return next;
      });
    }
  };

  useEffect(/* 当前回调同步 React 副作用和资源生命周期。 */ () => {
	// timer 定时器。
	const timer = window.setTimeout(/* 当前回调处理用户交互或异步状态变化。 */ () => {
	  setAutomationPage(1);
	  setDebouncedAutomationSearch(automationSearch.trim());
	}, 300);
	return /* 当前回调处理用户交互或异步状态变化。 */ () => window.clearTimeout(timer);
  }, [automationSearch]);

  useEffect(/* 当前回调处理异步操作结果。 */ () => {
	void loadReferenceData().catch(/* 当前回调处理异步操作结果。 */ error => console.error('加载规则参考数据失败', error));
  }, [loadReferenceData]);

  useEffect(/* 当前回调处理异步操作结果。 */ () => {
	void refresh().catch(/* 当前回调处理异步操作结果。 */ error => console.error('刷新规则页面失败', error));
  }, [refresh]);

  useEffect(/* 当前副作用让关键词规则应用内反馈三秒后自动消失。 */ () => {
    if (!replyFeedback) return;
    // timer 是关键词规则反馈自动关闭计时器。
    // duration 是成功和失败提示各自的可见毫秒数。
    const duration = replyFeedback.kind === 'error' ? 5000 : 2500;
    const timer = window.setTimeout(/* 当前回调清理已经展示完成的反馈。 */ () => setReplyFeedback(null), duration);
    return /* 当前清理取消旧反馈计时器。 */ () => window.clearTimeout(timer);
  }, [replyFeedback, setReplyFeedback]);

  // visibleAutomationRules 可见数据自动化规则列表，负责当前功能中的对应处理。
  const visibleAutomationRules = useMemo(
    /* 当前回调处理集合中的单个元素。 */ () => automationRules.filter(/* 当前回调处理集合中的单个元素。 */ rule => !selectedAccountId || rule.cookie_id === selectedAccountId),
    [automationRules, selectedAccountId],
  );

  // visibleAutomationIssues 可见数据自动化Issues，负责当前功能中的对应处理。
  const visibleAutomationIssues = useMemo(
	/* 当前回调计算并缓存派生数据。 */ () => filterAutomationIssues(automationIssues, selectedAccountId),
	[automationIssues, selectedAccountId],
  );

  // visibleDefaultAccounts 可见数据默认账号列表，负责当前功能中的对应处理。
  const visibleDefaultAccounts = useMemo(
    /* 当前回调处理集合中的单个元素。 */ () => accounts.filter(/* 当前回调处理集合中的单个元素。 */ account => !selectedAccountId || account.id === selectedAccountId),
    [accounts, selectedAccountId],
  );

  // automationPageNumbers 自动化页码Numbers，负责当前功能中的对应处理。
  const automationPageNumbers = useMemo(/* 当前回调计算并缓存派生数据。 */ () => {
	if (automationTotalPages <= 1) return [];
	// first 首项。
	const first = Math.max(1, Math.min(automationPage - 2, automationTotalPages - 4));
	// last last，负责当前功能中的对应处理。
	const last = Math.min(automationTotalPages, first + 4);
	return Array.from({ length: last - first + 1 }, /* 当前回调处理用户交互或异步状态变化。 */ (_, index) => first + index);
  }, [automationPage, automationTotalPages]);

  // hasAutomationListFilters has自动化列表Filters，负责当前功能中的对应处理。
  const hasAutomationListFilters = Boolean(
	automationSearch.trim() || automationTriggerFilter || automationStatusFilter !== 'all',
  );

  // clearAutomationListFilters 清理自动化列表Filters，负责当前功能中的对应处理。
  const clearAutomationListFilters = () => {
	setAutomationSearch('');
	setDebouncedAutomationSearch('');
	setAutomationTriggerFilter('');
	setAutomationStatusFilter('all');
	setAutomationPage(1);
  };

  // modalAccountItems modal账号商品列表，负责当前功能中的对应处理。
  const modalAccountItems = useMemo(/* 当前回调计算并缓存派生数据。 */ () => {
    // cookieID 账号凭证标识。
    const cookieID = editingAutomationRule?.cookie_id || selectedAccountId;
    return items.filter(/* 当前回调处理集合中的单个元素。 */ item => item.cookie_id === cookieID);
  }, [editingAutomationRule?.cookie_id, items, selectedAccountId]);

  // primaryActionLabel 主操作按钮文案。
  const primaryActionLabel = activeTab === 'automation'
    ? '新建自动化'
    : activeTab === 'reply'
      ? '新增全局回复规则'
      : '编辑默认回复';

  return (
    <div className="min-w-0 space-y-8 animate-fade-in">
      {replyFeedback && createPortal(<div role="status" className={`fixed left-1/2 top-6 z-[100] flex max-w-lg -translate-x-1/2 items-center gap-3 rounded-2xl border px-5 py-3 text-sm font-bold shadow-xl ${replyFeedback.kind === 'success' ? 'border-green-200 bg-green-50 text-green-700' : 'border-red-200 bg-red-50 text-red-700'}`}>{replyFeedback.kind === 'success' ? <CheckCircle2 className="h-5 w-5 shrink-0" /> : <AlertCircle className="h-5 w-5 shrink-0" />}<span>{replyFeedback.message}</span>{replyFeedback.kind === 'error' && <button type="button" aria-label="关闭错误提示" onClick={/* closeFeedback 允许用户立即关闭失败气泡。 */ () => setReplyFeedback(null)} className="ml-2 rounded-lg p-1 text-red-500 hover:bg-red-100"><X className="h-4 w-4" /></button>}</div>, document.body)}
      <div className="flex flex-col xl:flex-row justify-between xl:items-end gap-4">
        <div>
          <h2 className="text-4xl font-extrabold text-gray-900 tracking-tight">自动化规则</h2>
          <p className="text-gray-500 mt-2 font-medium">客户消息可进入关键词、默认或 AI；勾选的具体系统事件只进入关键词专用回复。</p>
        </div>
        <div className="flex flex-col sm:flex-row sm:flex-wrap gap-3">
          {activeTab === 'reply' ? <div className="flex w-full items-center justify-center rounded-2xl bg-blue-50 px-4 py-3 text-sm font-extrabold text-blue-700 sm:w-64"><Layers3 className="mr-2 h-4 w-4" />全局关键词规则</div> : <select
            value={selectedAccountId}
            onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => {
			  setSelectedAccountId(event.target.value);
			  setAutomationPage(1);
			}}
            className="ios-input w-full px-4 py-3 rounded-2xl text-sm sm:w-64"
          >
            <option value="">全部账号</option>
            {accounts.map(/* 当前回调处理集合中的单个元素。 */ account => (
              <option key={account.id} value={account.id}>{accountLabel(account)}</option>
            ))}
          </select>}
          <button
            onClick={refresh}
            className="px-4 py-3 rounded-2xl font-bold bg-gray-100 hover:bg-gray-200 text-gray-700 flex items-center justify-center gap-2 whitespace-nowrap transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </button>
          <button
            onClick={activeTab === 'automation' ? /* 当前回调处理用户交互或异步状态变化。 */ () => openNewAutomationRule('order_paid') : activeTab === 'reply' ? /* 当前回调打开空规则组并清理上次解析反馈。 */ () => { setKeywordDraft(''); setKeywordParseNotice(''); handleAddReplyRule(); } : /* 当前回调处理用户交互或异步状态变化。 */ () => void openDefaultReplyModal()}
            disabled={activeTab !== 'reply' && !selectedAccountId}
            className="ios-btn-primary px-5 py-3 rounded-2xl text-sm font-extrabold flex items-center justify-center gap-2 whitespace-nowrap disabled:opacity-50"
          >
            <Plus className="w-4 h-4" />
            {primaryActionLabel}
          </button>
        </div>
      </div>

      <div className="flex flex-wrap gap-2 p-2 bg-gray-100/50 rounded-2xl">
        {[
          { id: 'automation' as const, label: '交易自动化', icon: Zap },
          { id: 'reply' as const, label: '关键词回复', icon: MessageCircle },
          { id: 'default' as const, label: '账号默认回复', icon: Bot },
        ].map(/* 当前回调处理用户交互或异步状态变化。 */ tab => {
          // Icon 渲染Icon React 组件。
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setActiveTab(tab.id)}
              className={`inline-flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-bold transition-all ${
                activeTab === tab.id
                  ? 'bg-brand text-white shadow-md'
                  : 'bg-white text-gray-600 hover:text-black hover:bg-gray-50'
              }`}
            >
              <Icon className="w-4 h-4" />
              {tab.label}
            </button>
          );
        })}
      </div>

	  {activeTab === 'automation' && (visibleAutomationIssues.runs.length > 0 || visibleAutomationIssues.pending_tasks.length > 0) && (
	    <AutomationIssuePanel
	      runs={visibleAutomationIssues.runs}
	      pendingTasks={visibleAutomationIssues.pending_tasks}
	      onResolveRun={/* 当前回调处理用户交互或异步状态变化。 */ (id, resolution) => void handleResolveRunIssue(id, resolution)}
	      onResolveDeferredTask={/* 当前回调处理用户交互或异步状态变化。 */ (id, resolution) => void handleResolveDeferredIssue(id, resolution)}
	    />
	  )}

      {activeTab === 'automation' && (
        <div className="grid min-w-0 grid-cols-1 gap-6 xl:grid-cols-[minmax(270px,0.72fr)_minmax(0,1.28fr)]">
          <aside className="min-w-0 space-y-4">
            <div className="bg-white rounded-xl p-5 border border-gray-100 shadow-sm">
              <h3 className="font-black text-gray-900 mb-1">新建规则</h3>
              <p className="text-sm text-gray-500 mb-4">先选自动化类型，再配置对应动作。</p>
              <div className="space-y-3">
                {triggerOrder.map(/* 当前回调处理集合中的单个元素。 */ trigger => {
                  // meta 元数据。
                  const meta = triggerMeta[trigger];
                  // Icon 渲染Icon React 组件。
                  const Icon = meta.icon;
                  return (
                    <button
                      key={trigger}
                      type="button"
                      onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => openNewAutomationRule(trigger)}
                      className={`w-full text-left rounded-2xl border p-4 transition-colors ${accentClasses(meta.accent)}`}
                    >
                      <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-xl bg-white/80 flex items-center justify-center shrink-0">
                          <Icon className="w-5 h-5" />
                        </div>
                        <div>
                          <div className="font-extrabold">{meta.label}</div>
                          <div className="text-xs opacity-75 mt-1 leading-5">{meta.description}</div>
                        </div>
                      </div>
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="bg-white rounded-xl p-5 border border-gray-100 shadow-sm">
              <div className="mb-4 flex items-center justify-between gap-3">
				<h3 className="font-black text-gray-900">筛选结果构成</h3>
				<span className="text-xs font-bold text-gray-400">共 {automationTotal} 条</span>
			  </div>
              <div className="space-y-3">
                {triggerOrder.map(/* 当前回调处理集合中的单个元素。 */ trigger => {
                  // meta 元数据。
                  const meta = triggerMeta[trigger];
                  // Icon 渲染Icon React 组件。
                  const Icon = meta.icon;
                  return (
                    <div key={trigger} className="flex items-center justify-between rounded-2xl bg-gray-50 p-3">
                      <div className="flex items-center gap-3">
                        <Icon className="w-4 h-4 text-gray-500" />
                        <span className="text-sm font-bold text-gray-700">{meta.shortLabel}</span>
                      </div>
                      <span className="text-sm font-black text-gray-900">{automationTriggerCounts[trigger] || 0}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          </aside>

		  <section className="min-w-0 space-y-4">
			<div className="rounded-xl border border-gray-100 bg-surface-muted p-4 shadow-sm">
			  <div className="flex flex-col gap-3 xl:flex-row xl:items-center">
				<div className="relative min-w-0 flex-1">
				  <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
				  <input
					type="search"
					aria-label="搜索自动化规则"
					placeholder="搜索规则名、商品名或商品 ID..."
					value={automationSearch}
					onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => {
					  setAutomationSearch(event.target.value);
					  setAutomationPage(1);
					}}
					className="ios-input w-full rounded-xl border-none bg-white py-2.5 pl-10 pr-4 text-sm shadow-sm"
				  />
				</div>
				<div className="relative xl:w-52">
				  <SlidersHorizontal className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
				  <select
					aria-label="按自动化类型筛选"
					value={automationTriggerFilter}
					onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => {
					  setAutomationTriggerFilter(event.target.value as AutomationTriggerType | '');
					  setAutomationPage(1);
					}}
					className="ios-input w-full rounded-xl border-none bg-white py-2.5 pl-10 pr-9 text-sm shadow-sm"
				  >
					<option value="">全部自动化类型</option>
					{triggerOrder.map(/* 当前回调处理集合中的单个元素。 */ trigger => <option key={trigger} value={trigger}>{triggerMeta[trigger].shortLabel}</option>)}
				  </select>
				</div>
				<select
				  aria-label="按启用状态筛选"
				  value={automationStatusFilter}
				  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => {
					setAutomationStatusFilter(event.target.value as 'all' | 'enabled' | 'disabled');
					setAutomationPage(1);
				  }}
				  className="ios-input rounded-xl border-none bg-white px-4 py-2.5 text-sm shadow-sm xl:w-36"
				>
				  <option value="all">全部状态</option>
				  <option value="enabled">已启用</option>
				  <option value="disabled">已禁用</option>
				</select>
				{hasAutomationListFilters && (
				  <button
					type="button"
					onClick={clearAutomationListFilters}
					className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm transition-colors hover:bg-gray-100 hover:text-gray-900"
					title="清除筛选"
					aria-label="清除筛选"
				  >
					<X className="h-4 w-4" />
				  </button>
				)}
			  </div>
			  <div className="mt-3 flex items-center justify-between text-xs font-bold text-gray-400">
				<span>找到 {automationTotal} 条规则</span>
				{loading && <span className="inline-flex items-center gap-1.5"><RefreshCw className="h-3.5 w-3.5 animate-spin" />正在更新</span>}
			  </div>
			</div>

			{loading && visibleAutomationRules.length === 0 ? (
			  <div className="flex min-h-56 items-center justify-center rounded-xl border border-gray-100 bg-white text-sm font-bold text-gray-400">
				<RefreshCw className="mr-2 h-4 w-4 animate-spin" />
				正在加载规则
			  </div>
			) : visibleAutomationRules.length === 0 ? (
			  <div className="bg-white rounded-xl border border-dashed border-gray-200 p-16 text-center">
				<Zap className="w-12 h-12 text-gray-300 mx-auto mb-4" />
				<h3 className="text-xl font-black text-gray-900">{hasAutomationListFilters ? '没有匹配的自动化规则' : '还没有自动化规则'}</h3>
				<p className="text-gray-500 mt-2">{hasAutomationListFilters ? '调整或清除筛选条件后再试。' : '从左侧选择一个模板开始配置。'}</p>
			  </div>
			) : (
			  visibleAutomationRules.map(/* 当前回调处理集合中的单个元素。 */ rule => {
                // meta 元数据。
                const meta = triggerMeta[rule.trigger_type];
                // Icon 渲染Icon React 组件。
                const Icon = meta.icon;
                return (
                  <article key={rule.id} className="bg-white rounded-xl border border-gray-100 p-5 shadow-sm hover:shadow-lg transition-all">
                    <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                      <div className="flex items-start gap-4 min-w-0">
                        <div className={`w-12 h-12 rounded-2xl flex items-center justify-center shrink-0 ${accentClasses(meta.accent, true)}`}>
                          <Icon className="w-5 h-5" />
                        </div>
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2 mb-2">
                            <h3 className="text-lg font-black text-gray-900 truncate">{rule.name}</h3>
                            <span className={`px-2.5 py-1 rounded-full text-xs font-bold ${statusPill(rule.enabled)}`}>
                              {rule.enabled ? '已启用' : '已禁用'}
                            </span>
                          </div>
                          <div className="flex flex-wrap gap-2 text-xs font-bold">
                            <span className="px-2.5 py-1 rounded-lg bg-gray-100 text-gray-600">{meta.label}</span>
                            <span className="px-2.5 py-1 rounded-lg bg-gray-100 text-gray-600">{rule.item_title || rule.item_id || '账号级规则'}</span>
                            <span className="px-2.5 py-1 rounded-lg bg-blue-50 text-blue-700">{actionSummary(rule)}</span>
                          </div>
                        </div>
                      </div>

                      <div className="flex items-center gap-2 shrink-0">
                        <button
                          onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => openAutomationRule(rule)}
                          className="px-4 py-2 rounded-xl bg-gray-100 hover:bg-gray-200 text-gray-700 text-sm font-bold flex items-center gap-2"
                        >
                          <Edit className="w-4 h-4" />
                          编辑
                        </button>
                        <button
                          onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleToggleAutomation(rule)}
                          className={`px-4 py-2 rounded-xl text-sm font-bold ${rule.enabled ? 'bg-amber-50 text-amber-700 hover:bg-amber-100' : 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100'}`}
                        >
                          {rule.enabled ? '禁用' : '启用'}
                        </button>
                        <button
                          onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleDeleteAutomation(rule.id)}
                          className="p-2.5 rounded-xl text-red-500 hover:bg-red-50"
                          title="删除"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  </article>
                );
              })
			)}

			{automationTotal > 0 && (
			  <div className="flex flex-col gap-3 rounded-xl border border-gray-100 bg-white px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
				<div className="flex items-center gap-3 text-sm font-medium text-gray-500">
				  <span>第 {automationPage} / {Math.max(automationTotalPages, 1)} 页</span>
				  <span className="h-4 w-px bg-gray-200" />
				  <label className="flex items-center gap-2">
					<span className="sr-only">每页显示数量</span>
					<select
					  value={automationPageSize}
					  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => {
						setAutomationPageSize(Number(event.target.value));
						setAutomationPage(1);
					  }}
					  className="ios-input rounded-lg border-none bg-gray-50 px-2.5 py-2 text-sm"
					>
					  {[10, 20, 50].map(/* 当前回调处理集合中的单个元素。 */ size => <option key={size} value={size}>{size} 条/页</option>)}
					</select>
				  </label>
				</div>
				<div className="flex items-center gap-1.5">
				  <button
					type="button"
					disabled={automationPage <= 1 || loading}
					onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setAutomationPage(/* 当前回调处理用户交互或异步状态变化。 */ page => Math.max(1, page - 1))}
					className="flex h-9 w-9 items-center justify-center rounded-lg bg-gray-50 text-gray-600 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40"
					aria-label="上一页"
					title="上一页"
				  >
					<ChevronLeft className="h-4 w-4" />
				  </button>
				  {automationPageNumbers.map(/* 当前回调处理集合中的单个元素。 */ pageNumber => (
					<button
					  key={pageNumber}
					  type="button"
					  disabled={loading}
					  onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setAutomationPage(pageNumber)}
					  className={`h-9 min-w-9 rounded-lg px-2 text-sm font-bold transition-colors ${pageNumber === automationPage ? 'bg-brand text-white' : 'bg-gray-50 text-gray-600 hover:bg-gray-100'} disabled:cursor-not-allowed disabled:opacity-60`}
					  aria-label={`第 ${pageNumber} 页`}
					  aria-current={pageNumber === automationPage ? 'page' : undefined}
					>
					  {pageNumber}
					</button>
				  ))}
				  <button
					type="button"
					disabled={automationPage >= automationTotalPages || loading}
					onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setAutomationPage(/* 当前回调处理用户交互或异步状态变化。 */ page => Math.min(automationTotalPages, page + 1))}
					className="flex h-9 w-9 items-center justify-center rounded-lg bg-gray-50 text-gray-600 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40"
					aria-label="下一页"
					title="下一页"
				  >
					<ChevronRight className="h-4 w-4" />
				  </button>
				</div>
			  </div>
			)}
		  </section>
        </div>
      )}

      {activeTab === 'reply' && (
        <section className="bg-white rounded-xl border border-gray-100 p-6 shadow-sm">
          <div className="mb-5 flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
            <div className="flex w-fit items-center gap-2 rounded-xl bg-blue-50 px-4 py-2 text-sm text-blue-700">
              <AlertCircle className="w-4 h-4" />
              关键词组可分别处理客户消息和勾选的特定系统消息；系统消息只走关键词，不会进入 API、AI 或默认回复。
            </div>
            <div className="flex items-center gap-3 rounded-2xl border border-gray-200 bg-white px-4 py-2.5 shadow-sm">
              <div className="text-right">
                <div className="text-sm font-black text-gray-800">全部关键词自动回复</div>
                <div className="text-xs font-semibold text-gray-500">{totalReplyBindings === 0 ? '暂无规则' : someReplyRulesEnabled ? `部分开启 · ${enabledReplyBindings}/${totalReplyBindings} 个店铺规则` : allReplyRulesEnabled ? `已全部开启 · ${enabledReplyBindings}/${totalReplyBindings}` : `已全部关闭 · 0/${totalReplyBindings}`}</div>
              </div>
              <button type="button" role="switch" aria-checked={allReplyRulesEnabled} aria-label="切换全部关键词自动回复" disabled={replyRules.length === 0 || replyBulkPending || replyTogglePending.size > 0} onClick={/* toggleAllReplyRules 提交全部规则目标状态。 */ () => void handleToggleAllReplyRules()} className={`relative h-8 w-14 rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${allReplyRulesEnabled ? 'bg-blue-500' : someReplyRulesEnabled ? 'bg-amber-400' : 'bg-gray-300'}`}>
                <span className={`absolute left-0 top-1 block h-6 w-6 rounded-full bg-white shadow transition-transform ${allReplyRulesEnabled || someReplyRulesEnabled ? 'translate-x-7' : 'translate-x-1'}`} />
              </button>
            </div>
          </div>
          <div className="space-y-3">
            {replyRules.map(/* 当前回调处理集合中的单个元素。 */ rule => (
              <div key={rule.id} className={`flex flex-col gap-4 rounded-2xl border p-5 transition-all hover:bg-white hover:shadow-lg md:flex-row md:items-center md:justify-between ${rule.enabled ? 'border-gray-100 bg-surface-subtle' : 'border-gray-200 bg-gray-50/70'}`}>
                <div className="flex-1 min-w-0">
                  <div className="mb-2 flex flex-wrap items-center gap-2.5">
                    <span className="px-3 py-1 bg-black text-white rounded-lg text-xs font-bold">{rule.match_type === 'excludes' ? '不包含' : rule.match_type === 'equals' ? '等于匹配' : '包含匹配'}</span>
                    <span className="px-2.5 py-1 rounded-lg text-xs font-bold bg-blue-100 text-blue-700">{(rule.message_scopes || [rule.message_scope || 'customer']).includes('customer') ? '客户消息' : ''}{(rule.message_scopes || [rule.message_scope || 'customer']).includes('customer') && (rule.message_scopes || []).includes('system') ? '＋' : ''}{(rule.message_scopes || []).includes('system') ? '特定系统消息' : ''}</span>
                    <span className="rounded-lg bg-emerald-50 px-2.5 py-1 text-xs font-bold text-emerald-700">重复间隔 {rule.reply_interval_seconds ?? 1} 秒</span>
                    <span className="rounded-lg bg-sky-50 px-2.5 py-1 text-xs font-bold text-sky-700">{(rule.send_delay_seconds || 0) === 0 ? '立即发送' : `延迟 ${rule.send_delay_seconds} 秒`}</span>
                    <div className="flex flex-wrap gap-2">{(rule.keywords?.length ? rule.keywords : rule.keyword ? [rule.keyword] : []).map(/* keyword 是当前规则组展示的关键词气泡。 */ keyword => <span key={keyword} className="rounded-lg border border-gray-200 bg-white px-2.5 py-1 text-sm font-bold text-gray-700">{keyword}</span>)}{!(rule.keywords?.length || rule.keyword) && <span className="rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-1 text-sm font-bold text-amber-700">仅按系统事件触发</span>}</div>
                  </div>
                  <div className="bg-white p-3 rounded-xl border border-gray-100 text-sm text-gray-600 leading-relaxed">
                    {rule.type === 'image' && rule.image_url ? rule.image_url : rule.reply_content}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">{replyAccountStates(rule).map(/* state 是当前规则组绑定店铺的独立状态。 */ state => {
                    // pendingKey 是当前店铺开关的提交锁键。
                    const pendingKey = `${rule.id}:${state.account_id}`;
                    return <button key={state.account_id} type="button" role="switch" aria-checked={state.enabled} aria-label={`${state.enabled ? '关闭' : '开启'}店铺 ${state.account_id} 的自动回复`} disabled={replyBulkPending || replyTogglePending.has(rule.id) || replyAccountTogglePending.has(pendingKey)} onClick={/* toggleStoreRule 切换当前店铺规则状态。 */ () => void handleToggleReplyAccount(rule, state.account_id, !state.enabled)} className={`inline-flex items-center gap-2 rounded-lg border px-2.5 py-1.5 text-xs font-bold transition disabled:cursor-wait disabled:opacity-50 ${state.enabled ? 'border-purple-200 bg-purple-50 text-purple-700' : 'border-gray-200 bg-white text-gray-400'}`}><span className={`relative h-4 w-7 rounded-full ${state.enabled ? 'bg-purple-500' : 'bg-gray-300'}`}><span className={`absolute left-0 top-0.5 h-3 w-3 rounded-full bg-white shadow transition-transform ${state.enabled ? 'translate-x-3.5' : 'translate-x-0.5'}`} /></span><span>{accountLabel(accounts.find(/* account 是当前匹配展示名称的店铺。 */ account => account.id === state.account_id) || { id: state.account_id } as never)}</span></button>;
                  })}</div>
                </div>
                <div className="flex items-center gap-3 border-t border-gray-200 pt-4 md:min-w-[138px] md:flex-col md:justify-center md:border-l md:border-t-0 md:pl-6 md:pt-0">
                  <button type="button" role="switch" aria-checked={replyRuleStateLabel(rule) === '已开启'} aria-label="切换该规则全部店铺" disabled={replyBulkPending || replyTogglePending.has(rule.id) || replyAccountTogglePending.size > 0} onClick={/* toggleReplyRule 提交当前规则全部店铺的目标状态。 */ () => void handleToggleReplyRule({ ...rule, enabled: replyRuleStateLabel(rule) === '已开启' })} className={`inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border px-3 text-xs font-bold transition disabled:cursor-wait disabled:opacity-50 ${replyRuleStateLabel(rule) === '已开启' ? 'border-blue-200 bg-blue-50 text-blue-700' : replyRuleStateLabel(rule) === '部分开启' ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-gray-200 bg-white text-gray-500'}`}><span className={`relative h-5 w-9 rounded-full ${replyRuleStateLabel(rule) === '已开启' ? 'bg-blue-500' : replyRuleStateLabel(rule) === '部分开启' ? 'bg-amber-500' : 'bg-gray-300'}`}><span className={`absolute left-0 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${replyRuleStateLabel(rule) === '已关闭' ? 'translate-x-0.5' : 'translate-x-[18px]'}`} /></span><span>{replyTogglePending.has(rule.id) ? '保存中' : replyRuleStateLabel(rule)}</span></button>
                  <div className="flex items-center justify-center gap-3"><button
                    onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => {
                      setEditingReplyRule({ ...rule });
                      setKeywordDraft('');
                      setKeywordParseNotice(`已识别 ${(rule.keywords || [rule.keyword]).length} 个关键词`);
                      setShowReplyModal(true);
                    }}
                    className="p-2 text-gray-400 hover:text-black hover:bg-gray-100 rounded-xl transition-colors"
                    title="编辑"
                  >
                    <Edit className="w-4 h-4" />
                  </button>
                  <button onClick={/* 当前回调打开应用内规则组删除确认。 */ () => setDeleteReplyTarget(rule)} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-xl transition-colors" title="删除">
                    <Trash2 className="w-5 h-5" />
                  </button></div>
                </div>
              </div>
            ))}
            {replyRules.length === 0 && <div className="text-center py-20 text-gray-400">暂无关键词回复规则</div>}
          </div>
        </section>
      )}

      {activeTab === 'default' && (
        <section className="bg-white rounded-xl border border-gray-100 p-6 shadow-sm">
          <div className="flex items-center gap-2 text-sm text-blue-700 bg-blue-50 px-4 py-2 rounded-xl mb-5 w-fit">
            <AlertCircle className="w-4 h-4" />
            默认回复只处理买家用户消息；关键词未命中且 AI 未接管时才会使用。
          </div>
          <div className="space-y-3">
            {visibleDefaultAccounts.map(/* 当前回调处理集合中的单个元素。 */ account => {
              // defaultReply 默认Reply，负责当前功能中的对应处理。
              const defaultReply = defaultReplies[account.id];
              // enabled 启用状态。
              const enabled = Boolean(defaultReply?.enabled);
              return (
                <div key={account.id} className={`flex flex-col md:flex-row md:items-center justify-between p-5 rounded-2xl border transition-all gap-4 ${enabled ? 'border-purple-100 bg-purple-50/50 hover:bg-white hover:shadow-lg' : 'border-gray-100 bg-surface-subtle hover:bg-white hover:shadow-lg'}`}>
                  <div className="flex items-center gap-4 min-w-0">
                    <div className={`w-12 h-12 rounded-2xl flex items-center justify-center ${enabled ? 'bg-purple-600 text-white' : 'bg-gray-200 text-gray-400'}`}>
                      <Bot className="w-5 h-5" />
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 mb-2">
                        <h3 className="font-bold text-gray-900 text-lg truncate">{accountLabel(account)}</h3>
                        <span className={`px-2 py-0.5 rounded-lg text-xs font-bold ${enabled ? 'bg-green-100 text-green-700' : 'bg-gray-200 text-gray-500'}`}>
                          {enabled ? '已启用' : '未启用'}
                        </span>
                        {defaultReply?.reply_once && (
                          <span className="px-2 py-0.5 rounded-lg text-xs font-bold bg-purple-100 text-purple-700">只回复一次</span>
                        )}
                      </div>
                      <div className="text-sm text-gray-600 line-clamp-2">
                        {enabled ? (defaultReply.reply_content || defaultReply.reply_image_url || '已配置默认回复') : '未配置默认回复'}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 border-t md:border-t-0 md:border-l border-gray-200 pt-4 md:pt-0 md:pl-6">
                    <button
                      onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void openDefaultReplyModal(account.id)}
                      className="p-2 text-gray-400 hover:text-black hover:bg-gray-100 rounded-xl transition-colors"
                      title="编辑"
                    >
                      <Edit className="w-4 h-4" />
                    </button>
                    {enabled && (
                      <>
                        <button
                          onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void handleClearDefaultReplyRecords(account.id)}
                          className="px-3 py-2 text-xs font-bold text-blue-600 hover:bg-blue-50 rounded-xl transition-colors"
                        >
                          清空记录
                        </button>
                        <button onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => void handleDeleteDefaultReply(account.id)} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-xl transition-colors" title="删除">
                          <Trash2 className="w-5 h-5" />
                        </button>
                      </>
                    )}
                  </div>
                </div>
              );
            })}
            {visibleDefaultAccounts.length === 0 && <div className="text-center py-20 text-gray-400">暂无账号</div>}
          </div>
        </section>
      )}

      {showAutomationModal && editingAutomationRule && createPortal(
        <div className="modal-overlay">
          <div className="modal-container" style={{ maxWidth: '72rem', maxHeight: '92vh' }}>
            <div className="px-6 py-5 border-b border-gray-100 flex items-center justify-between">
              <div>
                <h3 className="text-2xl font-black text-gray-900">{editingAutomationRule.id ? '编辑自动化规则' : '新建自动化规则'}</h3>
                <p className="text-sm text-gray-500 mt-1">{currentMeta.description}</p>
              </div>
              <button
                onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setShowAutomationModal(false)}
                className="w-10 h-10 rounded-2xl bg-gray-100 hover:bg-gray-200 flex items-center justify-center"
                title="关闭"
              >
                <X className="w-5 h-5 text-gray-600" />
              </button>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-[320px_1fr] min-h-0">
              <aside className="bg-slate-900 text-white p-5 overflow-y-auto">
                <div className="text-xs font-bold text-slate-400 mb-3">选择自动化类型</div>
                <div className="space-y-3">
                  {triggerOrder.map(/* 当前回调处理集合中的单个元素。 */ trigger => {
                    // meta 元数据。
                    const meta = triggerMeta[trigger];
                    // Icon 渲染Icon React 组件。
                    const Icon = meta.icon;
                    // selected 处理当前选择（ed）。
                    const selected = currentTrigger === trigger;
                    return (
                      <button
                        key={trigger}
                        type="button"
                        onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => handleTriggerChange(trigger)}
                        className={`w-full rounded-2xl p-4 text-left border transition-all ${
                          selected ? 'bg-white text-slate-950 border-white' : 'bg-white/5 text-white border-white/10 hover:bg-white/10'
                        }`}
                      >
                        <div className="flex items-start gap-3">
                          <Icon className={`w-5 h-5 mt-0.5 ${selected ? 'text-brand' : 'text-white'}`} />
                          <div>
                            <div className="font-black">{meta.label}</div>
                            <div className={`text-xs mt-1 leading-5 ${selected ? 'text-gray-500' : 'text-gray-400'}`}>{meta.description}</div>
                          </div>
                        </div>
                      </button>
                    );
                  })}
                </div>

                <div className="mt-6 rounded-2xl bg-white/5 border border-white/10 p-4">
                  <div className="text-xs font-bold text-slate-400 mb-3">执行流程</div>
                  <div className="space-y-3">
                    {currentMeta.flow.map(/* 当前回调处理集合中的单个元素。 */ (step, index) => (
                      <div key={step} className="flex items-center gap-3">
                        <div className="w-6 h-6 rounded-full bg-white text-slate-950 text-xs font-black flex items-center justify-center">{index + 1}</div>
                        <span className="text-sm font-bold text-gray-100">{step}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </aside>

              <div className="p-6 overflow-y-auto bg-surface-subtle">
                <div className="space-y-5">
                  <section className="bg-white rounded-3xl border border-gray-100 p-5">
                    <div className="flex items-center gap-2 mb-4">
                      <CheckCircle2 className="w-5 h-5 text-brand" />
                      <h4 className="font-black text-gray-900">生效范围</h4>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div className="md:col-span-2">
                        <label className="block text-sm font-bold text-gray-700 mb-2">规则名称</label>
                        <input
                          type="text"
                          value={editingAutomationRule.name || ''}
                          onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({ ...editingAutomationRule, name: event.target.value })}
                          placeholder="不填时按类型和商品自动生成"
                          className="w-full ios-input px-4 py-3 rounded-xl"
                        />
                      </div>
                      <div>
                        <label className="block text-sm font-bold text-gray-700 mb-2">闲鱼账号</label>
                        <select
                          value={editingAutomationRule.cookie_id || ''}
                          onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({
                            ...editingAutomationRule,
                            cookie_id: event.target.value,
                            item_id: '',
                            item_title: '',
                            item_keyword: '',
                          })}
                          className="w-full ios-input px-4 py-3 rounded-xl"
                        >
                          <option value="">选择账号</option>
                          {accounts.map(/* 当前回调处理集合中的单个元素。 */ account => (
                            <option key={account.id} value={account.id}>{accountLabel(account)}</option>
                          ))}
                        </select>
                      </div>
                      <div>
                        <label className="block text-sm font-bold text-gray-700 mb-2">关联商品</label>
                        <select
                          value={editingAutomationRule.item_id || ''}
                          onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => handleAutomationItemChange(event.target.value)}
                          className="w-full ios-input px-4 py-3 rounded-xl"
                        >
                          <option value="">账号级规则（不限定商品）</option>
                          {modalAccountItems.map(/* 当前回调处理集合中的单个元素。 */ item => (
                            <option key={`${item.cookie_id}-${item.item_id}`} value={item.item_id}>{item.item_title || item.item_id}</option>
                          ))}
                        </select>
                      </div>
                    </div>

                    {selectedRuleItem && currentTrigger !== 'review_missing_timeout' && (
                      <div className="mt-4 rounded-2xl bg-gray-50 border border-gray-100 p-4">
                        <div className="flex flex-wrap items-center gap-2 mb-2">
                          <span className="px-3 py-1.5 rounded-lg bg-gray-100 text-gray-700 text-xs font-bold">{selectedRuleItem.item_title || selectedRuleItem.item_id}</span>
                          <span className={`px-3 py-1.5 rounded-lg text-xs font-bold ${isMultiSpecRule ? 'bg-blue-50 text-blue-700' : 'bg-gray-100 text-gray-500'}`}>
                            {isMultiSpecRule ? '多规格商品' : '普通商品'}
                          </span>
                          <span className="px-3 py-1.5 rounded-lg text-xs font-bold bg-emerald-50 text-emerald-700">按订单购买数量自动发货</span>
                        </div>
                        <p className="text-xs leading-5 text-gray-500">
                          多规格状态来自闲鱼商品本身，发布后不能在这里修改；系统会在买家付款后读取订单详情，按实际购买规格和数量匹配下面的发货规则。
                        </p>
                      </div>
                    )}
                  </section>

                  {currentTrigger !== 'review_missing_timeout' ? (
                    <section className="bg-white rounded-3xl border border-gray-100 p-5">
                      <div className="flex items-start justify-between gap-4 mb-4">
                        <div>
                          <div className="flex items-center gap-2">
                            <Layers3 className="w-5 h-5 text-brand" />
                            <h4 className="font-black text-gray-900">{currentTrigger === 'buyer_reviewed' ? '赠品库存' : '发货库存'}</h4>
                          </div>
                          <p className="text-sm text-gray-500 mt-1">
                            {isMultiSpecRule
                              ? '每条发货内容绑定一个订单规格；同一规格可添加多条内容并全部发送。'
                              : '可添加多条发货内容，买家付款后会按顺序全部发送。'}
                          </p>
                        </div>
                        <button
                          type="button"
                          onClick={appendDeliveryContent}
                          className="px-3 py-2 rounded-xl bg-gray-900 text-white text-xs font-bold hover:bg-black flex items-center gap-1.5"
                        >
                          <Plus className="w-3.5 h-3.5" />
                          添加发货内容
                        </button>
                      </div>

                      <div className="space-y-3">
                        {displayVariants.map((variant, index) => (/* 当前回调处理集合中的单个元素。 */
                          <div
                            key={variant.id || index}
                            className={`grid grid-cols-1 gap-3 items-end rounded-2xl border border-gray-200 p-4 ${isMultiSpecRule ? 'md:grid-cols-[1fr_1fr_1.4fr_110px_40px]' : 'md:grid-cols-[1.4fr_110px_40px]'}`}
                          >
                            {isMultiSpecRule && (
                              <>
                                <div>
                                  <label className="block text-xs font-bold text-gray-600 mb-2">规格名称</label>
                                  <input
                                    value={variant.spec_name}
                                    onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => updateVariant(index, { spec_name: event.target.value })}
                                    className="w-full ios-input px-3 py-2.5 rounded-lg"
                                    placeholder="例如：套餐"
                                  />
                                </div>
                                <div>
                                  <label className="block text-xs font-bold text-gray-600 mb-2">规格值</label>
                                  <input
                                    value={variant.spec_value}
                                    onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => updateVariant(index, { spec_value: event.target.value })}
                                    className="w-full ios-input px-3 py-2.5 rounded-lg"
                                    placeholder="例如：30天"
                                  />
                                </div>
                              </>
                            )}
                            <div>
                              <label className="block text-xs font-bold text-gray-600 mb-2">卡密库存</label>
                              <select
                                value={variant.card_id || ''}
                                onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => updateVariant(index, { card_id: Number(event.target.value) })}
                                className="w-full ios-input px-3 py-2.5 rounded-lg"
                              >
                                <option value="">请选择卡密库存</option>
                                {cards.filter(/* 当前回调处理集合中的单个元素。 */ card => card.enabled && card.type !== 'api').map(/* 当前回调处理集合中的单个元素。 */ card => (
                                  <option key={card.id} value={card.id}>{card.name}</option>
                                ))}
                              </select>
                            </div>
                            <div>
                              <label className="block text-xs font-bold text-gray-600 mb-2">每件份数</label>
                              <input
                                type="number"
                                min="1"
                                max="100"
                                value={variant.delivery_count}
                                onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => updateVariant(index, { delivery_count: Math.max(1, Number(event.target.value) || 1) })}
                                className="w-full ios-input px-3 py-2.5 rounded-lg"
                              />
                            </div>
                            <div className="md:col-span-full flex flex-wrap items-center gap-3 rounded-xl bg-gray-50 px-3 py-2">
                              <label className="flex items-center gap-2 text-xs font-bold text-gray-600 cursor-pointer">
                                <input
                                  type="checkbox"
                                  checked={variant.delay_override === true}
                                  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => updateVariant(index, { delay_override: event.target.checked })}
                                  className="accent-brand"
                                />
                                覆盖卡密默认延时
                              </label>
                              {variant.delay_override && (
                                <input
                                  type="number"
                                  min="0"
                                  max="3600"
                                  value={variant.delay_seconds || 0}
                                  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => updateVariant(index, { delay_seconds: Math.max(0, Number(event.target.value) || 0) })}
                                  className="w-28 ios-input px-2 py-1.5 rounded-lg text-xs"
                                  aria-label="动作延时秒数"
                                />
                              )}
                              <span className="text-xs text-gray-500">{variant.delay_override ? `本动作延时 ${variant.delay_seconds || 0} 秒` : '使用卡密默认延时'}</span>
                            </div>
                            <button
                              type="button"
                              disabled={displayVariants.length === 1}
                              onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setEditingAutomationRule({
                                ...editingAutomationRule,
                                variants: displayVariants.filter(/* 当前回调处理集合中的单个元素。 */ (_, variantIndex) => variantIndex !== index),
                              })}
                              className="w-10 h-10 flex items-center justify-center rounded-lg text-red-500 hover:bg-red-50 disabled:opacity-25"
                              title="删除发货内容"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          </div>
                        ))}
                      </div>
                    </section>
                  ) : (
                    <section className="bg-white rounded-3xl border border-gray-100 p-5">
                      <div className="flex items-center gap-2 mb-4">
                        <Clock3 className="w-5 h-5 text-amber-600" />
                        <h4 className="font-black text-gray-900">求评价计划</h4>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div>
                          <label className="block text-sm font-bold text-gray-700 mb-2">发货后等待小时</label>
                          <input
                            type="number"
                            min="1"
                            value={Number(reviewConfig.after_shipped_hours || 72)}
                            onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({
                              ...editingAutomationRule,
                              config_json: buildReviewConfig(editingAutomationRule.config_json, {
                                after_shipped_hours: Math.max(1, Number(event.target.value) || 72),
                              }),
                            })}
                            className="w-full ios-input px-4 py-3 rounded-xl"
                          />
                        </div>
                        <div>
                          <label className="block text-sm font-bold text-gray-700 mb-2">再次求评间隔小时</label>
                          <input
                            type="number"
                            min="1"
                            value={Number(reviewConfig.repeat_interval_hours || 24)}
                            onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({
                              ...editingAutomationRule,
                              config_json: buildReviewConfig(editingAutomationRule.config_json, {
                                repeat_interval_hours: Math.max(1, Number(event.target.value) || 24),
                              }),
                            })}
                            className="w-full ios-input px-4 py-3 rounded-xl"
                          />
                        </div>
                        <div>
                          <label className="block text-sm font-bold text-gray-700 mb-2">最多求评次数</label>
                          <input
                            type="number"
                            min="1"
                            value={Number(reviewConfig.max_attempts || 1)}
                            onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({
                              ...editingAutomationRule,
                              config_json: buildReviewConfig(editingAutomationRule.config_json, {
                                max_attempts: Math.max(1, Number(event.target.value) || 1),
                              }),
                            })}
                            className="w-full ios-input px-4 py-3 rounded-xl"
                          />
                        </div>
                        <div className="md:col-span-3">
                          <label className="block text-sm font-bold text-gray-700 mb-2">求评价文案</label>
                          <textarea
                            value={editingAutomationRule.actions?.find(/* 当前回调处理集合中的单个元素。 */ action => action.action_type === 'send_text')?.message_template || ''}
                            onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({
                              ...editingAutomationRule,
                              actions: (editingAutomationRule.actions?.length ? editingAutomationRule.actions : cardActionsForTrigger('review_missing_timeout')).map(/* 当前回调处理用户交互或异步状态变化。 */ action =>
                                action.action_type === 'send_text' ? { ...action, message_template: event.target.value } : action
                              ),
                            })}
                            className="w-full ios-input px-4 py-3 rounded-xl h-28 resize-none"
                          />
                        </div>
                      </div>
                    </section>
                  )}

                  <section className="bg-white rounded-3xl border border-gray-100 p-5">
                    <div className="grid grid-cols-1 md:grid-cols-[180px_1fr] gap-4 items-end">
                      <div>
						<label className="block text-sm font-bold text-gray-700 mb-2">优先级</label>
						<p className="text-xs text-gray-500 mb-2">数字越小优先级越高；同一账号、商品和触发条件只执行优先级最高的一条规则。</p>
                        <input
                          type="number"
                          value={editingAutomationRule.priority || 100}
                          onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({ ...editingAutomationRule, priority: Number(event.target.value) || 100 })}
                          min="1"
                          className="w-full ios-input px-4 py-3 rounded-xl"
                        />
                      </div>
                      <label className="h-[48px] flex items-center gap-3 px-4 bg-gray-50 rounded-xl text-sm font-bold text-gray-800">
                        <input
                          type="checkbox"
                          checked={editingAutomationRule.enabled !== false}
                          onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingAutomationRule({ ...editingAutomationRule, enabled: event.target.checked })}
                          className="w-4 h-4 rounded"
                        />
                        启用规则
                      </label>
                    </div>
                  </section>
                </div>
              </div>
            </div>

            <div className="px-6 py-4 border-t border-gray-100 bg-white flex gap-3">
              <button onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setShowAutomationModal(false)} className="flex-1 px-6 py-3 rounded-2xl font-bold bg-gray-100 text-gray-700 hover:bg-gray-200">
                取消
              </button>
              <button onClick={handleSaveAutomationRule} className="flex-1 ios-btn-primary px-6 py-3 rounded-2xl font-bold flex items-center justify-center gap-2">
                <Save className="w-4 h-4" />
                保存自动化规则
              </button>
            </div>
          </div>
        </div>,
        document.body
      )}

      {deleteReplyTarget && createPortal(
        <div className="modal-overlay">
          <div className="w-full max-w-md rounded-3xl bg-white p-6 shadow-2xl">
            <h3 className="text-xl font-extrabold text-gray-900">删除关键词回复规则？</h3>
            <p className="mt-3 text-sm leading-6 text-gray-600">将从所有已绑定店铺删除这条全局规则，删除后不会再自动回复。</p>
            <div className="mt-6 flex gap-3"><button type="button" className="flex-1 rounded-xl bg-gray-100 px-5 py-3 font-bold text-gray-700" onClick={/* 当前回调关闭应用内删除确认。 */ () => setDeleteReplyTarget(null)}>取消</button><button type="button" className="flex-1 rounded-xl bg-red-500 px-5 py-3 font-bold text-white" onClick={/* 当前回调确认删除全局规则并关闭确认框。 */ () => {
              // id 是等待删除的全局关键词规则组标识。
              const id = deleteReplyTarget.id;
              setDeleteReplyTarget(null);
              void handleDeleteReply(id);
            }}>确认删除</button></div>
          </div>
        </div>,
        document.body,
      )}

      {showReplyModal && editingReplyRule && createPortal(
        <div className="modal-overlay">
          <div className="modal-container">
            <div className="modal-header">
              <div className="flex items-center justify-between w-full">
                <h3 className="text-2xl font-extrabold text-gray-900">
                  {editingReplyRule.id ? '编辑回复规则' : '新增回复规则'}
                </h3>
                <button
                  onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setShowReplyModal(false)}
                  className="p-2 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors"
                >
                  <X className="w-5 h-5 text-gray-600" />
                </button>
              </div>
            </div>

            <div className="modal-body space-y-5">
              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">适用店铺</label>
                <div className="grid gap-2 md:grid-cols-2">{accounts.map(/* account 是当前可绑定全局规则的店铺账号。 */ account => <label key={account.id} className="flex items-center gap-3 rounded-xl border border-gray-200 px-4 py-3 text-sm font-bold"><input type="checkbox" checked={(editingReplyRule.account_ids || []).includes(account.id)} onChange={/* event 是店铺绑定勾选事件。 */ event => setEditingReplyRule({ ...editingReplyRule, account_ids: event.target.checked ? [...(editingReplyRule.account_ids || []), account.id] : (editingReplyRule.account_ids || []).filter(/* accountID 是当前保留的店铺账号。 */ accountID => accountID !== account.id) })} /><span>{accountLabel(account)}</span></label>)}</div>
                <p className="mt-2 text-xs text-gray-500">规则只在勾选店铺执行；编辑一次会同步全部绑定店铺。</p>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">关联商品</label>
                  <select
                    value={editingReplyRule.item_id || ''}
                    onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingReplyRule({ ...editingReplyRule, item_id: event.target.value })}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  >
                    <option value="">账号级回复</option>
                    {items.filter(/* item 是当前属于已选店铺的商品。 */ item => (editingReplyRule.account_ids || []).includes(item.cookie_id)).map(/* 当前回调处理集合中的单个元素。 */ item => (
                      <option key={`${item.cookie_id}-${item.item_id}`} value={item.item_id}>{item.item_title || item.item_id}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">回复类型</label>
                  <select
                    value={editingReplyRule.type || 'text'}
                    onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => {
                      // type 类型。
                      const type = event.target.value as 'text' | 'image';
                      setEditingReplyRule({
                        ...editingReplyRule,
                        type,
                        reply_content: type === 'text' ? editingReplyRule.reply_content : '',
                        image_url: type === 'image' ? editingReplyRule.image_url : '',
                      });
                    }}
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  >
                    <option value="text">文字</option>
                    <option value="image">图片</option>
                  </select>
                </div>
              </div>
              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">关键词组{(editingReplyRule.message_scopes || []).includes('system') && !(editingReplyRule.message_scopes || []).includes('customer') ? '（可选）' : ''}</label>
                <div className="rounded-xl border border-gray-200 bg-white p-3 focus-within:ring-2 focus-within:ring-brand/30">
                  <div className="mb-2 flex flex-wrap gap-2">
                    {(editingReplyRule.keywords || []).map(/* keyword 是当前可单独删除的关键词气泡。 */ keyword => <span key={keyword} className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-3 py-1.5 text-sm font-bold text-blue-700"><span>{keyword}</span><button type="button" className="rounded-full p-0.5 hover:bg-blue-100" onClick={/* 当前回调删除当前关键词气泡。 */ () => removeKeywordChip(keyword)} aria-label={`删除关键词 ${keyword}`}><X className="h-3.5 w-3.5" /></button></span>)}
                  </div>
                  <input type="text" value={keywordDraft} onChange={/* event 是关键词输入或粘贴事件。 */ event => {
                    // value 是输入框最新文本；包含分隔符时立即解析为气泡。
                    const value = event.target.value;
                    if (/[,，、;；\r\n]/.test(value)) commitKeywordText(value); else setKeywordDraft(value);
                  }} onKeyDown={/* event 是关键词输入键盘事件。 */ event => {
                    if (event.key === 'Enter') { event.preventDefault(); commitKeywordText(keywordDraft); }
                  }} onBlur={/* 当前回调在输入框失焦时提交剩余单个关键词。 */ () => commitKeywordText(keywordDraft)} placeholder="输入关键词后回车；支持粘贴逗号、顿号或换行分隔的一批关键词" className="w-full border-0 px-1 py-2 text-sm outline-none" />
                </div>
                {keywordParseNotice && <div className="mt-2 flex items-center gap-1.5 text-sm font-bold text-green-600"><CheckCircle2 className="h-4 w-4" />{keywordParseNotice}</div>}
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div><label className="block text-sm font-bold text-gray-700 mb-2">匹配逻辑</label><select value={editingReplyRule.match_type === 'excludes' || editingReplyRule.match_type === 'equals' ? editingReplyRule.match_type : 'contains'} onChange={/* event 是匹配逻辑选择事件。 */ event => setEditingReplyRule({ ...editingReplyRule, match_type: event.target.value as 'contains' | 'excludes' | 'equals' })} className="w-full ios-input px-4 py-3 rounded-xl"><option value="contains">包含任意关键词</option><option value="excludes">不包含全部关键词</option><option value="equals">全文等于任意关键词</option></select>{editingReplyRule.match_type === 'excludes' && <p className="mt-2 text-xs font-bold text-amber-600">高风险兜底：消息不包含组内任何关键词时触发。</p>}</div>
                <div><label className="block text-sm font-bold text-gray-700 mb-2">消息来源</label><div className="grid grid-cols-2 gap-2">{[{ value: 'customer', label: '客户消息' }, { value: 'system', label: '系统消息' }].map(/* option 是当前可同时勾选的消息来源。 */ option => <label key={option.value} className="flex items-center gap-2 rounded-xl border border-gray-200 px-3 py-3 text-sm font-bold"><input type="checkbox" checked={(editingReplyRule.message_scopes || ['customer']).includes(option.value as 'customer' | 'system')} onChange={/* event 是消息来源勾选事件。 */ event => {
                  // current 是变更前的消息来源集合。
                  const current = editingReplyRule.message_scopes || ['customer'];
                  // next 是应用当前勾选后的消息来源集合。
                  const next = event.target.checked ? [...current, option.value as 'customer' | 'system'] : current.filter(/* value 是当前保留的消息来源。 */ value => value !== option.value);
                  if (next.length === 0) return;
                  setEditingReplyRule({ ...editingReplyRule, message_scopes: next, message_scope: next[0], system_types: next.includes('system') ? (editingReplyRule.system_types || []) : [] });
                }} />{option.label}</label>)}</div></div>
              </div>

              {(editingReplyRule.message_scopes || []).includes('system') && <div className="space-y-3"><div><label className="block text-sm font-bold text-gray-700">允许的具体系统事件</label><p className="mt-1 text-xs text-gray-500">先通过稳定事件白名单，再执行关键词匹配；未知交易事件不会触发。</p></div>{SYSTEM_EVENT_GROUPS.map(/* group 是当前系统事件业务分类。 */ group => <div key={group.title} className="rounded-2xl border border-gray-200 p-3"><div className="mb-2 text-xs font-extrabold text-gray-500">{group.title}</div><div className="grid gap-2 sm:grid-cols-2">{group.events.map(/* option 是当前可选稳定系统事件。 */ option => <label key={option.value} className="flex items-center gap-2 rounded-xl bg-gray-50 px-3 py-2 text-sm font-bold"><input type="checkbox" checked={(editingReplyRule.system_types || []).includes(option.value)} onChange={/* event 是稳定系统事件勾选事件。 */ event => setEditingReplyRule({ ...editingReplyRule, system_types: event.target.checked ? [...(editingReplyRule.system_types || []), option.value] : (editingReplyRule.system_types || []).filter(/* value 是当前保留的系统事件。 */ value => value !== option.value) })} /><span>{option.label}</span></label>)}</div></div>)}</div>}

              <section className="rounded-2xl border border-gray-200 bg-gray-50/80 p-4"><div className="mb-3"><div className="text-sm font-black text-gray-800">触发控制</div><p className="mt-1 text-xs text-gray-500">分别控制同组重复回复频率，以及命中后多久发送消息。</p></div><div className="grid gap-3 md:grid-cols-2"><div className="rounded-xl border border-gray-200 bg-white p-4"><label className="block text-sm font-bold text-gray-700">重复回复间隔</label><div className="mt-2 flex items-center gap-2"><input type="number" min="0" max="86400" value={editingReplyRule.reply_interval_seconds ?? 1} onChange={/* event 是重复回复间隔秒数输入事件。 */ event => setEditingReplyRule({ ...editingReplyRule, reply_interval_seconds: Math.max(0, Math.min(86400, Number(event.target.value) || 0)) })} className="ios-input min-w-0 flex-1 rounded-xl px-3 py-2.5" /><span className="text-sm font-bold text-gray-500">秒</span></div><p className="mt-2 text-xs leading-5 text-gray-500">同一店铺的同一客户命中后，间隔内不重复回复。</p></div><div className="rounded-xl border border-blue-200 bg-blue-50/60 p-4"><label className="block text-sm font-bold text-gray-700">发送延迟</label><div className="mt-2 flex items-center gap-2"><input type="number" min="0" max="3600" value={editingReplyRule.send_delay_seconds ?? 0} onChange={/* event 是关键词发送延迟秒数输入事件。 */ event => setEditingReplyRule({ ...editingReplyRule, send_delay_seconds: Math.max(0, Math.min(3600, Number(event.target.value) || 0)) })} className="ios-input min-w-0 flex-1 rounded-xl border-blue-200 bg-white px-3 py-2.5" /><span className="text-sm font-bold text-gray-500">秒</span></div><p className="mt-2 text-xs leading-5 text-blue-700">{(editingReplyRule.send_delay_seconds || 0) === 0 ? '关键词命中后立即发送回复。' : `关键词命中后等待 ${editingReplyRule.send_delay_seconds} 秒，再发送回复消息。`}</p></div></div><div className="mt-3 rounded-xl bg-amber-50 px-3 py-2.5 text-xs leading-5 text-amber-700">ⓘ 发送延迟只推迟本次回复；重复回复间隔从消息成功发送后开始计算。</div></section>

              {editingReplyRule.type === 'image' ? (
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">图片 URL</label>
                  <input
                    value={editingReplyRule.image_url || ''}
                    onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingReplyRule({ ...editingReplyRule, image_url: event.target.value })}
                    placeholder="https://..."
                    className="w-full ios-input px-4 py-3 rounded-xl"
                  />
                </div>
              ) : (
                <div>
                  <label className="block text-sm font-bold text-gray-700 mb-2">回复内容</label>
                  <textarea
                    value={editingReplyRule.reply_content || ''}
                    onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setEditingReplyRule({ ...editingReplyRule, reply_content: event.target.value })}
                    placeholder="自动回复的内容"
                    className="w-full ios-input px-4 py-3 rounded-xl h-32 resize-none"
                  />
                </div>
              )}

              <div className="flex gap-3 pt-4">
                <button
                  onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setShowReplyModal(false)}
                  className="flex-1 px-6 py-3 rounded-xl font-bold bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={/* 当前回调保存前用应用内反馈拒绝尚未解析为气泡的残留文本。 */ () => { if (keywordDraft.trim()) { setReplyFeedback({ kind: 'error', message: '请先按回车确认当前关键词' }); return; } void handleSaveReplyRule(); }}
                  className="flex-1 ios-btn-primary px-6 py-3 rounded-xl font-bold flex items-center justify-center gap-2"
                >
                  <Send className="w-4 h-4" />
                  保存规则
                </button>
              </div>
            </div>
          </div>
        </div>,
        document.body
      )}

      {showDefaultModal && createPortal(
        <div className="modal-overlay">
          <div className="modal-container">
            <div className="modal-header">
              <div className="flex items-center justify-between w-full">
                <div>
                  <h3 className="text-2xl font-extrabold text-gray-900">账号默认回复</h3>
                  <p className="text-sm text-gray-500 mt-1">关键词和 AI 都未处理时，才会使用默认回复。</p>
                </div>
                <button
                  onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setShowDefaultModal(false)}
                  className="p-2 bg-gray-100 rounded-full hover:bg-gray-200 transition-colors"
                  title="关闭"
                >
                  <X className="w-5 h-5 text-gray-600" />
                </button>
              </div>
            </div>

            <div className="modal-body space-y-5">
              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">闲鱼账号</label>
                <select
                  value={defaultForm.cookie_id}
                  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setDefaultForm({ ...defaultForm, cookie_id: event.target.value })}
                  className="w-full ios-input px-4 py-3 rounded-xl"
                >
                  <option value="">选择账号</option>
                  {accounts.map(/* 当前回调处理集合中的单个元素。 */ account => (
                    <option key={account.id} value={account.id}>{accountLabel(account)}</option>
                  ))}
                </select>
              </div>

              <div className="flex items-center justify-between p-4 bg-gray-50 rounded-xl">
                <div>
                  <div className="font-bold text-gray-900">启用默认回复</div>
                  <div className="text-xs text-gray-500 mt-1">启用后，未命中关键词时自动发送</div>
                </div>
                <button
                  type="button"
                  onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setDefaultForm({ ...defaultForm, enabled: !defaultForm.enabled })}
                  className={`w-14 h-8 rounded-full transition-colors duration-300 relative ${defaultForm.enabled ? 'bg-brand' : 'bg-gray-300'}`}
                >
                  <span className={`absolute top-1 w-6 h-6 bg-white rounded-full shadow-md transition-transform duration-300 block ${defaultForm.enabled ? 'translate-x-7' : 'translate-x-1'}`} />
                </button>
              </div>

              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">回复内容</label>
                <textarea
                  value={defaultForm.reply_content}
                  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setDefaultForm({ ...defaultForm, reply_content: event.target.value })}
                  placeholder="输入默认回复内容"
                  className="w-full ios-input px-4 py-3 rounded-xl h-32 resize-none"
                />
              </div>

              <div>
                <label className="block text-sm font-bold text-gray-700 mb-2">回复图片 URL（可选）</label>
                <input
                  type="text"
                  value={defaultForm.reply_image_url}
                  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setDefaultForm({ ...defaultForm, reply_image_url: event.target.value })}
                  placeholder="https://example.com/image.jpg"
                  className="w-full ios-input px-4 py-3 rounded-xl"
                />
              </div>

              <label className="flex items-center justify-between p-4 bg-gray-50 rounded-xl text-sm font-bold text-gray-800">
                <span>
                  只回复一次
                  <span className="block text-xs text-gray-500 font-medium mt-1">同一会话只发送一次默认回复</span>
                </span>
                <input
                  type="checkbox"
                  checked={defaultForm.reply_once}
                  onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setDefaultForm({ ...defaultForm, reply_once: event.target.checked })}
                  className="w-4 h-4 rounded"
                />
              </label>

              <div className="flex gap-3 pt-4">
                <button
                  onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => setShowDefaultModal(false)}
                  className="flex-1 px-6 py-3 rounded-xl font-bold bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={handleSaveDefaultReply}
                  className="flex-1 ios-btn-primary px-6 py-3 rounded-xl font-bold flex items-center justify-center gap-2"
                >
                  <Save className="w-4 h-4" />
                  保存默认回复
                </button>
              </div>
            </div>
          </div>
        </div>,
        document.body
      )}
    </div>
  );
};

export default Rules;
