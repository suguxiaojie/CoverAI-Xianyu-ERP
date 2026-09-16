import { AlertCircle,CheckCircle2,Loader2,QrCode,Search,User,X } from 'lucide-react';
import React,{ useCallback,useEffect,useRef,useState } from 'react';
import { createPortal } from 'react-dom';
import { AccountDetail } from '../api';
import {
deleteAccount,
refreshAccountProfile,
updateAccountStatus,
} from '../api';
import { AccountAISettingsModal } from '../components/AccountAISettingsModal';
import AccountAutomationModal from '../components/AccountAutomationModal';
import { AccountCard } from '../components/AccountCard';
import { AccountDeleteDialog } from '../components/AccountDeleteDialog';
import { AccountEditModal } from '../components/AccountEditModal';
import { AccountQRCodeModal } from '../components/AccountQRCodeModal';
import { useAccountsData } from '../hooks';
import { useAccountQRCodeLogin } from '../qrLogin';
import { accountRuntimePresentation } from '../runtime';
import { useAccountSubmodules,type AccountModalType } from '../submoduleHooks';
import type { AccountAutomationFocus,AccountEditFocus,AccountEditForm } from '../types';

// AccountList 渲染账号列表组件。
const AccountList: React.FC = () => {
  // accountData 保存账号列表及其加载控制器。
  const { accounts, setAccounts, loading, loadAccounts } = useAccountsData();
  // accountSearch 保存列表过滤关键词。
  const [accountSearch, setAccountSearch] = useState('');
  // selectedAccountId 保存主从布局中由用户主动选择的账号 ID。
  const [selectedAccountId, setSelectedAccountId] = useState('');
  // refreshingProfileId 保存正在刷新资料的账号 ID。
  const [refreshingProfileId, setRefreshingProfileId] = useState<string>('');
  // deletingAccountId 保存正在删除的账号 ID。
  const [deletingAccountId, setDeletingAccountId] = useState<string>('');
  // deleteDialogAccount 保存待确认删除的账号。
  const [deleteDialogAccount, setDeleteDialogAccount] = useState<AccountDetail | null>(null);
  // deleteError 保存删除失败提示。
  const [deleteError, setDeleteError] = useState('');
  // activeModal 保存当前打开的账号配置弹窗。
  const [activeModal, setActiveModal] = useState<AccountModalType>(null);
  // editingAccount 保存当前编辑账号。
  const [editingAccount, setEditingAccount] = useState<AccountDetail | null>(null);
  // taskAccount 保存当前打开自动化任务弹窗的账号。
  const [taskAccount, setTaskAccount] = useState<AccountDetail | null>(null);
	// taskFocus 保存控制中心卡片要求自动任务弹窗定位的区块；快捷入口为空。
	const [taskFocus,setTaskFocus] = useState<AccountAutomationFocus | undefined>();
	// editFocus 保存控制中心卡片要求账号编辑弹窗定位的区块；普通编辑入口为空。
	const [editFocus,setEditFocus] = useState<AccountEditFocus | undefined>();
  // taskFeedback 保存账号自动任务保存成功或失败后的页面气泡。
  const [taskFeedback, setTaskFeedback] = useState<{ /** kind 区分成功和失败反馈。 */ kind: 'success' | 'error'; /** message 是用户可见提示。 */ message: string } | null>(null);
  // taskFeedbackTimerRef 保存气泡自动消失计时器，页面卸载时统一释放。
  const taskFeedbackTimerRef = useRef<number | null>(null);

  // 编辑表单状态。
  // editForm 保存编辑弹窗中的账号草稿。
  const [editForm, setEditForm] = useState<AccountEditForm>({
    remark: '',
    cookie: '',
    auto_confirm: false,
    pause_duration: 0,
    username: '',
    login_password: '',
    show_browser: false,
    showLoginPassword: false,
    clear_password: false,
  });

  // accountSubmodules 集中管理编辑弹窗的长登录、通知绑定、AI 和密码登录状态。
  const accountSubmodules = useAccountSubmodules({
    editingAccount,
    setEditingAccount,
    setActiveModal,
    editForm,
    setEditForm,
    loadAccounts,
  });
  // submoduleHandlers 保存编辑弹窗子模块的状态和事件处理函数。
  const {
    longLogin,
    notifChannels,
    selectedChannelIds,
    bindingsLoaded,
    bindingsLoading,
    bindingsLoadError,
    aiSettings,
    saving,
    passwordLoginView,
    setAiSettings,
    setBindingsDirty,
    openEditModal,
    closeEditModal,
    openAIModal,
    closeAIModal,
    loadNotificationBindings,
    toggleNotificationChannel,
    handleLongLoginToggle,
    handleSaveAISettings,
    handleSaveEdit,
    handleRestartPause,
    handlePasswordLogin,
    handleCancelPasswordLogin,
  } = accountSubmodules;

	// handleOpenTasks 打开账号自动任务弹窗，并记录卡片对应的精确滚动目标。
	const handleOpenTasks = useCallback(/* openTasksAction 绑定账号和可选自动任务定位目标。 */ (account: AccountDetail, focus?: AccountAutomationFocus): void => {
		setTaskFocus(focus);
		setTaskAccount(account);
	}, []);
	// handleCloseTasks 关闭自动任务弹窗并清除上次入口，避免快捷入口继承旧滚动目标。
	const handleCloseTasks = useCallback(/* closeTasksAction 清理自动任务弹窗上下文。 */ (): void => {
		setTaskAccount(null);
		setTaskFocus(undefined);
	}, []);
	// handleOpenEdit 打开账号编辑弹窗，并记录自动确认卡片要求的精确滚动目标。
	const handleOpenEdit = useCallback(/* openEditAction 在调用既有编辑初始化前保存可选定位目标。 */ (account: AccountDetail, focus?: AccountEditFocus): void => {
		setEditFocus(focus);
		void openEditModal(account);
	}, [openEditModal]);
	// handleCloseEdit 关闭账号编辑弹窗并清除上次定位目标。
	const handleCloseEdit = useCallback(/* closeEditAction 收束既有编辑弹窗并重置定位状态。 */ (): void => {
		setEditFocus(undefined);
		void closeEditModal();
	}, [closeEditModal]);

  // qrLogin 集中管理二维码弹窗状态、轮询、风控验证和异步资源收束。
  const qrLogin = useAccountQRCodeLogin({ onLoginSuccess: loadAccounts });
  // qrViewState 解构二维码弹窗向页面展示和触发操作所需的最小状态。
  const {
    showQRModal,
    qrCodeUrl,
    qrStatus,
    qrErrorMessage,
    verificationScreenshot,
    faceQrUrl,
    qrReauthTarget,
    captchaMode,
    setCaptchaMode,
    startQRLogin,
    closeQRModal,
  } = qrLogin;

  // handleToggle 切换账号启用状态。
  const handleToggle = async (id: string, currentStatus: boolean) => {
    await updateAccountStatus(id, !currentStatus);
    loadAccounts();
  };

  // openDeleteDialog 打开账号删除确认框。
  const openDeleteDialog = (account: AccountDetail) => {
    if (deletingAccountId) return;
    setDeleteError('');
    setDeleteDialogAccount(account);
  };

  // closeDeleteDialog 关闭账号删除确认框。
  const closeDeleteDialog = () => {
    if (deletingAccountId) return;
    setDeleteError('');
    setDeleteDialogAccount(null);
  };

  // confirmDeleteAccount 执行账号删除并刷新列表状态。
  const confirmDeleteAccount = async () => {
    // account 保存当前确认删除的账号。
    const account = deleteDialogAccount;
    if (!account || deletingAccountId) return;
    setDeletingAccountId(account.id);
    setDeleteError('');
    try {
      await deleteAccount(account.id);
      setAccounts(/* 当前回调处理集合中的单个元素。 */ current => current.filter(/* 当前回调处理集合中的单个元素。 */ item => item.id !== account.id));
      setDeleteDialogAccount(null);
    } catch (/* error 保存账号删除请求的失败原因，仅转换为界面提示。 */ error: any) {
      console.error('删除账号失败:', error);
      setDeleteError(error?.message || '删除账号失败，请稍后重试');
    } finally {
      setDeletingAccountId('');
    }
  };

  // handleRefreshProfile 刷新账号资料并同步列表。
  const handleRefreshProfile = async (account: AccountDetail) => {
    setRefreshingProfileId(account.id);
    try {
      // res 保存资料刷新接口返回值。
      const res = await refreshAccountProfile(account.id);
      if (res?.profile_error) {
        alert('资料刷新失败：' + res.profile_error);
      }
      await loadAccounts();
    } catch (/* error 保存资料刷新请求的失败原因，仅转换为界面提示。 */ error: any) {
      console.error('刷新账号资料失败:', error);
      alert(error?.message || '刷新账号资料失败，请先重新授权该账号');
    } finally {
      setRefreshingProfileId('');
    }
  };

  // showTaskFeedback 展示账号自动任务保存结果，并替换上一条尚未结束的提示。
  const showTaskFeedback = useCallback(/* taskFeedbackAction 接收结果类别和用户可见文案并重置提示时限。 */ (kind: 'success' | 'error', message: string): void => {
    if (taskFeedbackTimerRef.current !== null) window.clearTimeout(taskFeedbackTimerRef.current);
    setTaskFeedback({ kind, message });
    taskFeedbackTimerRef.current = window.setTimeout(
      /* taskFeedbackCleanup 清除本次自动任务保存气泡。 */ () => setTaskFeedback(null),
      3_500,
    );
  }, []);

  useEffect(/* taskFeedbackLifecycleEffect 在页面卸载时释放保存气泡计时器。 */ () => {
    return /* taskFeedbackUnmountCleanup 防止页面卸载后仍更新提示状态。 */ () => {
      if (taskFeedbackTimerRef.current !== null) window.clearTimeout(taskFeedbackTimerRef.current);
    };
  }, []);
  if (loading) return <div className="p-20 flex justify-center"><Loader2 className="w-8 h-8 text-brand animate-spin"/></div>;

  // filteredAccounts 过滤后的账号列表，负责当前功能中的对应处理。
  const filteredAccounts = accounts.filter(/* 当前回调处理集合中的单个元素。 */ account => {
    // keyword 搜索关键词。
    const keyword = accountSearch.trim().toLowerCase();
    if (!keyword) return true;
    return [
      account.id,
      account.nickname,
      account.remark,
      account.username,
      account.runtime_message,
    ].some(/* 当前回调处理用户交互或异步状态变化。 */ value => (value || '').toLowerCase().includes(keyword));
  });
  // selectedAccount 优先使用用户选择且仍满足筛选条件的账号，否则回退到当前首个结果。
  const selectedAccount = filteredAccounts.find(/* 当前回调定位主从布局中已选择的账号。 */ account => account.id === selectedAccountId) || filteredAccounts[0] || null;
  // onlineAccountCount 统计全部账号中当前在线的账号数量，用于紧凑页头摘要。
  const onlineAccountCount = accounts.filter(/* 当前回调统计运行态为在线的账号。 */ account => account.runtime_state === 'online').length;

  return (
    <div className="relative space-y-4 animate-fade-in">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
            <h2 className="text-2xl font-extrabold tracking-tight text-gray-900">账号管理</h2>
            <div className="flex items-center gap-2 text-sm font-semibold text-gray-500">
              {filteredAccounts.length === accounts.length ? (
                <span>{accounts.length} 个账号</span>
              ) : (
                <span>当前显示 {filteredAccounts.length} / {accounts.length} 个账号</span>
              )}
              <span aria-hidden="true">·</span>
              <span className="text-emerald-600">{onlineAccountCount} 个在线</span>
            </div>
          </div>
          <p className="mt-2 text-sm font-medium text-gray-500">管理闲鱼授权账号及设置；选择上方账号后，在控制中心处理资料、授权和自动化。</p>
        </div>
        <div className="flex w-full flex-col gap-2 sm:flex-row xl:w-auto">
          <label className="relative block w-full sm:w-64">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
            <input
              value={accountSearch}
              onChange={/* 当前回调处理用户交互或异步状态变化。 */ event => setAccountSearch(event.target.value)}
              placeholder="搜索昵称 / 备注 / 账号ID"
              className="ios-input h-10 w-full rounded-xl py-2 pl-9 pr-3 text-sm"
            />
          </label>
          <button
            onClick={/* 当前回调处理用户交互或异步状态变化。 */ () => startQRLogin()}
            className="ios-btn-primary inline-flex h-10 items-center justify-center gap-2 rounded-xl px-4 text-sm font-bold"
          >
            <QrCode className="h-4 w-4" />
            扫码添加新账号
          </button>
        </div>
      </div>

      {accounts.length > 0 && filteredAccounts.length > 0 && selectedAccount && (
        <div className="space-y-3">
          <div className="flex flex-wrap gap-2" role="tablist" aria-label="店铺账号">
            {filteredAccounts.map(/* 当前回调渲染可切换选中账号的顶部标签。 */ account => {
              // runtime 保存账号标签所需的运行状态颜色与文案。
              const runtime = accountRuntimePresentation(account);
              // isSelected 表示该账号是否正在控制中心展示。
              const isSelected = selectedAccount.id === account.id;
              return (
                <button
                  key={account.id}
                  type="button"
                  role="tab"
                  data-testid={`account-selector-${account.id}`}
                  aria-selected={isSelected}
                  onClick={/* 当前回调把用户点击的账号切换为控制中心目标。 */ () => setSelectedAccountId(account.id)}
                  className={`flex min-w-48 items-center gap-3 rounded-xl border px-3 py-2.5 text-left transition ${isSelected ? 'border-brand bg-brand-50 shadow-sm' : 'border-gray-200 bg-white hover:border-gray-300'}`}
                >
                  {account.avatar_url ? (
                    <img src={account.avatar_url} alt={account.nickname || '账号头像'} className="h-10 w-10 flex-none rounded-full bg-gray-100 object-cover" />
                  ) : (
                    <span className="flex h-10 w-10 flex-none items-center justify-center rounded-full bg-gray-100 text-gray-400"><User className="h-5 w-5" /></span>
                  )}
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-bold text-gray-900">{account.nickname || account.remark || `账号 ${account.id.substring(0, 6)}...`}</span>
                    <span className="mt-0.5 flex items-center gap-1.5 text-xs font-semibold text-gray-500">
                      <span className={`h-2 w-2 rounded-full ${runtime.dot}`} aria-hidden="true" />
                      <span>{runtime.label}</span>
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
          <AccountCard
            account={selectedAccount}
            refreshing={refreshingProfileId === selectedAccount.id}
            deleting={deletingAccountId === selectedAccount.id}
            onRefreshProfile={handleRefreshProfile}
            onReauthorize={startQRLogin}
            onEdit={handleOpenEdit}
            onAI={openAIModal}
            onTasks={handleOpenTasks}
            onToggle={handleToggle}
            onDelete={openDeleteDialog}
          />
        </div>
      )}

      {accounts.length === 0 && (
        <div className="rounded-xl border border-gray-200 bg-white p-10 text-center">
          <div className="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-full bg-gray-100">
            <User className="h-7 w-7 text-gray-400" />
          </div>
          <h3 className="text-base font-bold text-gray-900">暂无账号</h3>
          <p className="mt-1 text-sm text-gray-500">请点击右上角扫码添加您的闲鱼账号</p>
        </div>
      )}
      {accounts.length > 0 && filteredAccounts.length === 0 && (
        <div className="rounded-xl border border-gray-200 bg-white p-10 text-center">
          <h3 className="text-base font-bold text-gray-900">没有匹配的账号</h3>
          <p className="mt-1 text-sm text-gray-500">换一个关键词搜索昵称、备注或账号ID。</p>
        </div>
      )}

      {taskAccount && createPortal(
        <AccountAutomationModal
          account={taskAccount}
		  initialFocus={taskFocus}
		  onClose={handleCloseTasks}
          onSaved={/* 当前回调处理集合中的单个元素。 */ settings => {
            setAccounts(/* 当前回调处理集合中的单个元素。 */ current => current.map(/* 当前回调处理集合中的单个元素。 */ account => account.id === taskAccount.id ? {
              ...account,
              auto_rate_enabled: settings.auto_rate_enabled,
              rate_content: settings.rate_content,
              auto_polish_enabled: settings.auto_polish_enabled,
              polish_time: settings.polish_time,
              auto_request_flower_enabled: settings.auto_request_flower_enabled,
              request_flower_after_hours: settings.request_flower_after_hours,
              request_flower_after_seconds: settings.request_flower_after_seconds,
              auto_receive_flower_enabled: settings.auto_receive_flower_enabled,
			  receive_flower_show_browser: settings.receive_flower_show_browser,
			  receive_flower_timeout_seconds: settings.receive_flower_timeout_seconds,
			  auto_receipt_reminder_enabled: settings.auto_receipt_reminder_enabled,
			  receipt_reminder_after_days: settings.receipt_reminder_after_days,
			  receipt_reminder_time: settings.receipt_reminder_time,
			  receipt_reminder_message: settings.receipt_reminder_message,
			  receipt_reminder_enabled_at: settings.receipt_reminder_enabled_at,
              last_rate_scan_at: settings.last_rate_scan_at,
              last_polish_date: settings.last_polish_date,
              last_polish_at: settings.last_polish_at,
            } : account));
          }}
          onSaveSucceeded={/* taskSaveSuccess 保存成功后关闭当前弹窗并展示账号级气泡。 */ () => {
            // accountName 是成功气泡中用于确认保存目标的账号名称。
            const accountName = taskAccount.nickname || taskAccount.remark || taskAccount.id;
			handleCloseTasks();
            showTaskFeedback('success', `${accountName}：自动任务设置已保存`);
          }}
          onSaveFailed={/* taskSaveFailure 保存失败后保留弹窗并展示失败气泡。 */ message => {
            // accountName 是失败气泡中用于确认保存目标的账号名称。
            const accountName = taskAccount.nickname || taskAccount.remark || taskAccount.id;
            showTaskFeedback('error', `${accountName}：保存失败：${message}`);
          }}
        />,
        document.body
      )}

      {taskFeedback && createPortal(
        <div role="status" className={`fixed left-1/2 top-6 z-[120] flex max-w-lg -translate-x-1/2 items-center gap-3 rounded-2xl border px-5 py-3 text-sm font-bold shadow-xl ${taskFeedback.kind === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-red-200 bg-red-50 text-red-700'}`}>
          {taskFeedback.kind === 'success' ? <CheckCircle2 className="h-5 w-5 shrink-0" /> : <AlertCircle className="h-5 w-5 shrink-0" />}
          <span>{taskFeedback.message}</span>
          {taskFeedback.kind === 'error' && <button type="button" aria-label="关闭保存失败提示" onClick={/* closeTaskFeedback 允许用户立即关闭失败气泡。 */ () => setTaskFeedback(null)} className="ml-2 rounded-lg p-1 text-red-500 hover:bg-red-100"><X className="h-4 w-4" /></button>}
        </div>,
        document.body,
      )}

      {deleteDialogAccount && (
        <AccountDeleteDialog
          account={deleteDialogAccount}
          deleting={deletingAccountId === deleteDialogAccount.id}
          error={deleteError}
          onClose={closeDeleteDialog}
          onConfirm={confirmDeleteAccount}
        />
      )}

      {showQRModal && (
        <AccountQRCodeModal
          target={qrReauthTarget}
          status={qrStatus}
          codeUrl={qrCodeUrl}
          errorMessage={qrErrorMessage}
          faceQrUrl={faceQrUrl}
          verificationScreenshot={verificationScreenshot}
          captchaMode={captchaMode}
          onCaptchaModeChange={setCaptchaMode}
          onClose={closeQRModal}
        />
      )}

      {/* 编辑账号弹窗由 accounts feature 组件负责渲染和表单交互。 */}
      {activeModal === 'edit' && editingAccount && (
        <AccountEditModal
          account={editingAccount}
          editForm={editForm}
          setEditForm={setEditForm}
          saving={saving}
		  initialFocus={editFocus}
		  onClose={handleCloseEdit}
          onSave={handleSaveEdit}
          onRestartPause={handleRestartPause}
          longLogin={longLogin}
          onToggleLongLogin={handleLongLoginToggle}
          passwordLoginView={passwordLoginView}
          onPasswordLogin={handlePasswordLogin}
          onCancelPasswordLogin={handleCancelPasswordLogin}
          notifChannels={notifChannels}
          selectedChannelIds={selectedChannelIds}
          bindingsLoaded={bindingsLoaded}
          bindingsLoading={bindingsLoading}
          bindingsLoadError={bindingsLoadError}
          onRetryBindings={/* 当前回调处理用户交互或异步状态变化。 */ () => loadNotificationBindings(editingAccount.id)}
          onToggleChannel={toggleNotificationChannel}
          onSettingsDirty={/* 当前回调处理用户交互或异步状态变化。 */ () => setBindingsDirty(true)}
        />
      )}


      {activeModal === 'ai-settings' && editingAccount && (
        <AccountAISettingsModal
          account={editingAccount}
          settings={aiSettings}
          saving={saving}
          onChange={setAiSettings}
          onClose={closeAIModal}
          onSave={handleSaveAISettings}
        />
      )}
    </div>
  );
};

export default AccountList;
