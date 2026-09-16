import type { AccountDetail,AccountTaskSettings,AccountTaskSummary } from './api';

/** 账号任务类型。 */
export type AccountTaskType = 'auto_rate' | 'auto_polish';

/** AccountAutomation Hook 的输入参数。 */
export type AccountAutomationOptions = {
  /** 当前编辑的账号。 */
  account: AccountDetail;
  /** 保存成功后的页面同步回调。 */
  onSaved: (settings: AccountTaskSettings) => void;
  /** 设置保存成功后的交互收口回调，不用于立即执行任务。 */
  onSaveSucceeded?: () => void;
  /** 设置保存失败后的页面气泡回调，不用于加载或立即执行失败。 */
  onSaveFailed?: (message: string) => void;
};

/** AccountAutomation Hook 的状态和动作。 */
export type AccountAutomationState = {
  /** 账号任务设置草稿。 */
  form: AccountTaskSettings;
  /** 初始设置加载状态。 */
  loading: boolean;
  /** 保存设置状态。 */
  saving: boolean;
  /** 当前运行中的任务类型。 */
  running: '' | AccountTaskType;
  /** 最近一次任务统计。 */
  summary: AccountTaskSummary | null;
  /** 当前错误信息。 */
  error: string;
  /** 是否存在可重试动作。 */
  retryAvailable: boolean;
  /** 最近一次保存是否成功且表单尚未再次修改。 */
  saved: boolean;
};
