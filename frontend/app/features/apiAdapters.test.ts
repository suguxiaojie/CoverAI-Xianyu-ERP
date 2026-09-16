import { afterEach,expect,test,vi } from 'vitest';
import {
addAccount,
cancelPasswordLogin,
checkPasswordLoginStatus,
checkQRLoginStatus,completeQRVerification,
deleteAccount,
generateQRLogin,
getAccountAISettings,
getAccountDetails,getAccountRuntimeStatuses,
getAccountTaskSettings,
getAllAISettings,
getLongLoginSettings,
passwordLogin,
refreshAccountProfile,
runAccountTask,
setLongLoginSettings,
updateAccountAISettings,
updateAccountAutoConfirm,
updateAccountCookie,
updateAccountLoginInfo,
updateAccountPauseDuration,
updateAccountRemark,
updateAccountSettings,
updateAccountStatus,
updateAccountTaskSettings,
} from './accounts/api';
import { appendCardData,batchCreateCards,cardImagePreviewURL,createCard,deleteCard,getCardDetails,getCards,isManagedCardImage,updateCard } from './cards/api';
import { getChatMessagePage,getChatMessages,getChatSessionPage,getChatSessions,getConversationOrderContext,markChatRead,recallChatMessage,refuseMerchantRefund,sendChatImage,sendChatMessage,setChatSessionPinned,shipOrderWithEvidence } from './chat/api';
import { getDashboardAccounts,getDashboardRuntimeStatuses,getDashboardStats,getOrderAnalytics,getValidOrders } from './dashboard/api';
import { cancelItemPublishBatch,createItem,deleteItem,deleteItemPublishBatch,getItemDetail,getItemPublishBatch,getItemPublishBatches,getItems,previewItemPublishBatch,publishItem,recommendPublishCategory,retryFailedItemPublishBatch,startItemPublishBatch,syncItemsFromAccount,updateItem,updateItemSKUCost } from './items/api';
import { createNotificationChannel,deleteAccountNotifications,deleteMessageNotification,deleteNotificationChannel,getAccountBindings,getMessageNotifications,getNotificationChannels,setAccountBindings,setMessageNotification,testNotificationChannel,updateNotificationChannel } from './notifications/api';
import { cancelOrderRefreshJob,deleteOrder,getAdminStats,getOrderDetail,getOrders,getRedFlowerStatus,importOrders,manualShipOrder,requestRedFlower,syncOrders,syncSingleOrder,updateOrder } from './orders/api';
import { clearDefaultReplyRecords,deleteDefaultReply,deleteReplyRule,deleteShippingRule,getAutomationIssues,getDefaultReplies,getDefaultReply,getReplyRules,getShippingRules,getShippingRulesPage,resolveAutomationRun,resolveDeferredAutomationTask,setAllReplyRulesEnabled,setReplyRuleEnabled,updateDefaultReply,updateReplyRule,updateShippingRule } from './rules/api';
import { initializeAdmin,login,logout,verifySession } from './session/api';
import { changePassword,fetchAIModels,getSystemSettings,updateLoginCredentials,updateSystemSettings } from './settings/api';
import { getHealth } from './system/api';

afterEach(() => {
	vi.unstubAllGlobals();
	vi.restoreAllMocks();
} /* 测试回调验证：全局 API 适配器测试环境清理。 */);

test('updateSystemSettings uses one atomic bulk request', async () => {
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	await updateSystemSettings({ theme_color: 'blue', renewal_log_retention_days: 15 });
	expect(fetchMock).toHaveBeenCalledTimes(1);
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/settings/system', expect.objectContaining({ method: 'PUT', credentials: 'include' }));
	expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ theme_color: 'blue', renewal_log_retention_days: 15 });
} /* 测试回调验证：updateSystemSettings uses one atomic bulk request。 */);

test('updateSystemSettings separates sensitive values into explicit commands', /* 当前回调验证敏感设置三态命令请求体。 */ async () => {
  // fetchMock 是系统设置更新请求的 HTTP 替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  await updateSystemSettings({ ai_api_key: 'new-secret', smtp_password: '' });
  // payload 是普通设置与敏感命令分离后的请求体。
  const payload = JSON.parse(fetchMock.mock.calls[0][1].body);
  expect(payload.values).toEqual({});
  expect(payload.secrets).toEqual({
    ai_api_key: { action: 'replace', value: 'new-secret' },
    smtp_password: { action: 'clear' },
  });
});

test('health API exposes build metadata through the request boundary', async () => {
  // fetchMock 是健康检查 API 的请求替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ version: '1.2.3', commit: 'abc123' }));
  vi.stubGlobal('fetch', fetchMock);
  const controller = new AbortController(); /* controller 表示controller。 */

  await expect(getHealth({ signal: controller.signal })).resolves.toEqual({ version: '1.2.3', commit: 'abc123' });
  expect(fetchMock).toHaveBeenCalledWith('/health', expect.objectContaining({ method: 'GET', signal: expect.any(AbortSignal) }));
} /* 测试回调验证：health API exposes build metadata through the request boundary。 */);

test('chat APIs preserve account and conversation scope', async () => {
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ sessions: [{ account_id: 'a1', chat_id: 'c1' }] }))
		.mockResolvedValueOnce(jsonResponse({ messages: [{ account_id: 'a1', chat_id: 'c1', id: 1 }] }))
		.mockImplementation(() => Promise.resolve(jsonResponse({ success: true, message: { id: 2 } })) /* 模拟新增动作接口返回包含标识的成功响应。 */); /* fetchMock 是本测试替代浏览器网络层的请求桩。 */
	vi.stubGlobal('fetch', fetchMock);
	await getChatSessions('a1');
	await getChatMessages('a1', 'c1', 9);
	await sendChatMessage({ account_id: 'a1', chat_id: 'c1', buyer_id: 'b1', text: 'hi' });
	await recallChatMessage('a1', 'local-1');
	await markChatRead('a1', 'c1', [{ messageId: 'm1', sessionId: 'c1', cid: 'c1@goofish', conversationType: 1 }]);
	expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/chat/sessions?account_id=a1');
	expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/chat/messages?account_id=a1&chat_id=c1&before_id=9');
	expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toMatchObject({ account_id: 'a1', chat_id: 'c1', buyer_id: 'b1' });
	expect(fetchMock.mock.calls[3][0]).toBe('/api/v1/chat/messages/local-1/recall');
	expect(JSON.parse(fetchMock.mock.calls[3][1].body)).toEqual({ account_id: 'a1' });
	expect(JSON.parse(fetchMock.mock.calls[4][1].body)).toEqual({ account_id: 'a1', chat_id: 'c1', message_ids: [{ messageId: 'm1', sessionId: 'c1', cid: 'c1@goofish', conversationType: 1 }] });
} /* 测试回调验证：chat APIs preserve account and conversation scope。 */);

test('chat reply API uses a dedicated route to prevent old servers from downgrading native replies', async () => {
	// fetchMock 是仅记录原生引用请求且不访问真实后端的网络替身。
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ message: { id: 2 } }));
	vi.stubGlobal('fetch', fetchMock);
	await sendChatMessage({ account_id: 'a1', chat_id: 'c1', buyer_id: 'b1', text: '引用回复', reply_to_message_key: 'target-local' });
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/chat/replies', expect.objectContaining({ method: 'POST' }));
	expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toMatchObject({ account_id: 'a1', chat_id: 'c1', reply_to_message_key: 'target-local' });
} /* 测试回调验证引用文本不会被旧普通消息接口静默降级。 */);

test('Chat 会话、消息和发送 API 转发外部取消信号', async () => {
  // fetchMock 验证会话切换、消息分页和文本/图片发送共享取消控制能力。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ sessions: [], has_more: false }))
    .mockResolvedValueOnce(jsonResponse({ messages: [], has_more: false }))
    .mockResolvedValueOnce(jsonResponse({ success: true, message: { message_key: 'm1' } }))
    .mockResolvedValueOnce(jsonResponse({ success: true, message: { message_key: 'm2' } }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是 Chat feature Hook 使用的请求控制器。
  const controller = new AbortController();
  await getChatSessionPage('a1', undefined, { signal: controller.signal });
  await getChatMessagePage('a1', 'c1', undefined, undefined, { signal: controller.signal });
  await sendChatMessage({ account_id: 'a1', chat_id: 'c1', buyer_id: 'b1', text: 'hi' }, { signal: controller.signal });
  await sendChatImage({ account_id: 'a1', chat_id: 'c1', buyer_id: 'b1', image: new File(['image'], 'chat.png', { type: 'image/png' }) }, { signal: controller.signal });
  await markChatRead('a1', 'c1', [], { signal: controller.signal });
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/chat/sessions?account_id=a1', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/chat/messages?account_id=a1&chat_id=c1', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/chat/messages', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/chat/images', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/chat/read', expect.objectContaining({ signal: expect.any(AbortSignal) }));
} /* 测试回调验证：Chat 会话、消息和发送 API 转发外部取消信号。 */);

test('Chat 会话历史搜索通过版本化接口传递账号和关键词', /* chatHistoricalSearchAPICase 验证关键词不会退化为前端摘要过滤。 */ async () => {
	// fetchMock 返回一条匹配会话摘要，不包含命中的历史消息原文。
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ sessions: [{ account_id: 'a1', chat_id: 'history-chat', last_message: '地址在这里' }], has_more: false }));
	vi.stubGlobal('fetch', fetchMock);
	// controller 是连续输入时由 Chat Hook 取消旧搜索使用的控制器。
	const controller = new AbortController();
	// page 是服务端历史消息搜索返回的会话页。
	const page = await getChatSessionPage('a1', undefined, { signal: controller.signal }, false, ' TEST-CARD-CODE-001 ');
	expect(page.sessions[0].chat_id).toBe('history-chat');
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/chat/sessions?account_id=a1&search=TEST-CARD-CODE-001', expect.objectContaining({ method: 'GET', credentials: 'include', signal: expect.any(AbortSignal) }));
});

test('Chat 会话置顶 API 使用精确路径并转发取消信号', /* pinApiAdapterTest 验证会话路径编码、请求体和取消边界。 */ async () => {
  // fetchMock 是返回服务端权威置顶状态的 HTTP 替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ account_id: 'a1', chat_id: 'chat/1', pinned: true }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是同一账号会话新操作可用于取消旧操作的请求控制器。
  const controller = new AbortController();
  await expect(setChatSessionPinned('a1', 'chat/1', true, { signal: controller.signal })).resolves.toMatchObject({ pinned: true });
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/chat/sessions/chat%2F1/pin', expect.objectContaining({ method: 'PUT', signal: expect.any(AbortSignal) }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ account_id: 'a1', pinned: true });
});

test('account task APIs keep rating and polish account-scoped', async () => {
	const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse({ success: true, summary: { task_type: 'auto_rate' } })) /* 模拟账号任务执行后返回任务摘要。 */); /* fetchMock 是本测试替代浏览器网络层的请求桩。 */
	vi.stubGlobal('fetch', fetchMock);
	await updateAccountTaskSettings('a1', {
		account_id: 'a1', auto_rate_enabled: true, rate_content: '交易愉快',
		auto_polish_enabled: true, polish_time: '03:00',
		auto_request_flower_enabled: true, request_flower_after_hours: 6, request_flower_after_seconds: 10,
		auto_receive_flower_enabled: true, receive_flower_show_browser: true, receive_flower_timeout_seconds: 120,
		auto_receipt_reminder_enabled: false, receipt_reminder_after_days: 2, receipt_reminder_time: '10:00', receipt_reminder_message: '请确认收货',
	});
	await runAccountTask('a1', 'auto_rate');
	expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/account-tasks/a1');
	expect(fetchMock.mock.calls[0][1].method).toBe('PUT');
	expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/account-tasks/a1/run');
	expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ task_type: 'auto_rate' });
} /* 测试回调验证：account task APIs keep rating and polish account-scoped。 */);

test('账号自动任务 API 转发外部取消信号', async () => {
  // fetchMock 验证读取、保存和执行账号任务都支持请求取消。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ account_id: 'a1', auto_rate_enabled: true, rate_content: '交易愉快', auto_polish_enabled: false, polish_time: '03:00' }))
    .mockResolvedValueOnce(jsonResponse({ account_id: 'a1', auto_rate_enabled: true, rate_content: '交易愉快', auto_polish_enabled: false, polish_time: '03:00' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, summary: { task_type: 'auto_rate', found: 1, success: 1, failed: 0, skipped: 0 } }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是 AccountAutomation feature Hook 使用的请求控制器。
  const controller = new AbortController();
  const settings = {
    account_id: 'a1', auto_rate_enabled: true, rate_content: '交易愉快', auto_polish_enabled: false, polish_time: '03:00',
    auto_request_flower_enabled: false, request_flower_after_hours: 24, request_flower_after_seconds: 10, auto_receive_flower_enabled: false,
	receive_flower_show_browser: true, receive_flower_timeout_seconds: 120,
	auto_receipt_reminder_enabled: false, receipt_reminder_after_days: 2, receipt_reminder_time: '10:00', receipt_reminder_message: '请确认收货',
  }; /* settings 表示settings。 */
  await getAccountTaskSettings('a1', { signal: controller.signal });
  await updateAccountTaskSettings('a1', settings, { signal: controller.signal });
  await runAccountTask('a1', 'auto_rate', { signal: controller.signal });
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/account-tasks/a1', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/account-tasks/a1', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/account-tasks/a1/run', expect.objectContaining({ signal: expect.any(AbortSignal) }));
} /* 测试回调验证：账号自动任务 API 转发外部取消信号。 */);

test('getItemPublishBatches unwraps persisted batch list', async () => {
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ batches: [{ id: 'batch-1', status: 'running' }] })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	await expect(getItemPublishBatches(10)).resolves.toEqual([{ id: 'batch-1', status: 'running' }]);
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/items/publish-batches?limit=10', expect.objectContaining({ credentials: 'include' }));
} /* 测试回调验证：getItemPublishBatches unwraps persisted batch list。 */);

test('automation issue APIs expose and resolve quarantined work', async () => {
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ runs: [{ id: 1 }], pending_tasks: [{ id: 2 }] }))
		.mockImplementation(() => Promise.resolve(jsonResponse({ success: true })) /* 模拟无消息体的成功操作响应。 */); /* fetchMock 是本测试替代浏览器网络层的请求桩。 */
	vi.stubGlobal('fetch', fetchMock);
	await expect(getAutomationIssues()).resolves.toEqual({ runs: [{ id: 1 }], pending_tasks: [{ id: 2 }] });
	await resolveAutomationRun(1, 'continue');
	await resolveDeferredAutomationTask(2, 'retry');
	expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/automation-runs/1/resolve');
	expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ resolution: 'continue' });
	expect(fetchMock.mock.calls[2][0]).toBe('/api/v1/automation-pending-tasks/2/resolve');
} /* 测试回调验证：automation issue APIs expose and resolve quarantined work。 */);

test('order multipart requests use the shared authenticated form request path', async () => {
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-1', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-1', status: 'succeeded', result: { partial_failure: false, message: '同步完成', summary: { discovered: 0, list_updated: 0, soft_deleted: 0, detail_total: 0, total: 0, updated: 0, no_change: 0, failed: 0 }, results: [] } }))
		.mockResolvedValueOnce(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	await syncOrders('acc1', 'pending_ship');
	await importOrders(new FormData());
	expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/orders/refresh', expect.objectContaining({ method: 'POST', credentials: 'include', body: expect.any(FormData) }));
	// refreshBody 保存默认订单同步创建请求的表单，用于验证增量模式显式传给后端。
	const refreshBody = fetchMock.mock.calls[0][1]?.body as FormData;
	expect(refreshBody.get('mode')).toBe('incremental');
	expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/orders/refresh/job-1', expect.objectContaining({ method: 'GET', credentials: 'include' }));
	expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/orders/import', expect.objectContaining({ method: 'POST', credentials: 'include', body: expect.any(FormData) }));
} /* 测试回调验证：order multipart requests use the shared authenticated form request path。 */);

test('syncOrders surfaces failed persisted job status', async () => {
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-failed', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-failed', status: 'failed', error_message: '平台会话已过期' })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	await expect(syncOrders()).rejects.toThrow('平台会话已过期');
} /* 测试回调验证：syncOrders surfaces failed persisted job status。 */);

test('syncOrders streams lightweight progress before the final result', async () => {
	// runningProgress 是服务端运行中的逐单详情进度，不包含完整结果集合。
	const runningProgress = { stage: 'syncing_details', message: '正在逐单同步订单详情', processed: 4, total: 10, succeeded: 3, failed: 1, percent: 56 } as const;
	// completedProgress 是任务成功终态的百分百进度。
	const completedProgress = { stage: 'completed', message: '订单同步完成', processed: 10, total: 10, succeeded: 9, failed: 1, percent: 100 } as const;
	// fetchMock 依次返回创建、运行进度和最终完整结果。
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-progress', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-progress', status: 'running', progress: runningProgress }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-progress', status: 'succeeded', progress: completedProgress, result: { partial_failure: false, message: '同步完成', summary: { discovered: 0, list_updated: 0, soft_deleted: 0, detail_total: 10, total: 10, updated: 4, no_change: 5, failed: 1 }, results: [] } }));
	vi.stubGlobal('fetch', fetchMock);
	// onProgress 收集创建、运行和成功三个轻量状态快照。
	const onProgress = vi.fn();
	await syncOrders(undefined, undefined, { onProgress, pollIntervalMs: 0 });
	expect(onProgress).toHaveBeenNthCalledWith(1, expect.objectContaining({ job_id: 'job-progress', status: 'running' }));
	expect(onProgress).toHaveBeenNthCalledWith(2, expect.objectContaining({ progress: runningProgress, status: 'running' }));
	expect(onProgress.mock.calls[1][0]).not.toHaveProperty('result');
	expect(onProgress).toHaveBeenNthCalledWith(3, expect.objectContaining({ status: 'succeeded', progress: completedProgress }));
} /* 测试回调验证：syncOrders streams lightweight progress before the final result。 */);

test('syncOrders aborts while waiting for persisted job status', async () => {
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-running', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-running', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-running', status: 'cancelled' })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	// controller 控制订单刷新轮询的取消信号。
	const controller = new AbortController();
	// pending 保存等待取消结果的订单刷新请求。
	const pending = syncOrders(undefined, undefined, { signal: controller.signal });
	await vi.waitFor(/* 等待创建和首次状态查询完成。 */ () => expect(fetchMock).toHaveBeenCalledTimes(2));
	controller.abort();
	await expect(pending).rejects.toThrow('请求已取消');
	await vi.waitFor(/* 等待取消命令发出。 */ () => expect(fetchMock).toHaveBeenCalledWith('/api/v1/orders/refresh/job-running', expect.objectContaining({ method: 'DELETE', credentials: 'include' })));
} /* 测试回调验证：syncOrders aborts while waiting for persisted job status。 */);

test('syncOrders reaches the front-end budget, cancels, and returns a concurrent success terminal result', async () => {
	// fetchMock 模拟轮询仍运行、取消命令返回冲突且复查已拿到成功终态的时间竞争。
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-timeout', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-timeout', status: 'running' }))
		.mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: 'conflict', message: '任务已结束' } }), { status: 409, headers: { 'content-type': 'application/json' } }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-timeout', status: 'succeeded', result: { partial_failure: false, message: '终态成功', summary: { discovered: 0, list_updated: 0, soft_deleted: 0, detail_total: 0, total: 0, updated: 0, no_change: 0, failed: 0 }, results: [] } }));
	vi.stubGlobal('fetch', fetchMock);

	// result 保存达到本地等待预算后由终态复查返回的成功订单刷新结果。
	const result = await syncOrders(undefined, undefined, { pollLimit: 1, pollIntervalMs: 0 });
	expect(result.message).toBe('终态成功');
	expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/orders/refresh/job-timeout', expect.objectContaining({ method: 'DELETE', credentials: 'include' }));
	expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/orders/refresh/job-timeout', expect.objectContaining({ method: 'GET', credentials: 'include' }));
} /* 测试回调验证：syncOrders reaches the front-end budget, cancels, and returns a concurrent success terminal result。 */);

test('syncOrders reports a timeout only after cancellation and final terminal read remain non-terminal', async () => {
	// fetchMock 模拟后台任务未在取消请求后及时进入成功或失败终态的场景。
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-still-running', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-still-running', status: 'running' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-still-running', status: 'cancelled' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, job_id: 'job-still-running', status: 'cancelled' }));
	vi.stubGlobal('fetch', fetchMock);

	await expect(syncOrders(undefined, undefined, { pollLimit: 1, pollIntervalMs: 0 })).rejects.toThrow('订单刷新任务等待超时，已请求取消');
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/orders/refresh/job-still-running', expect.objectContaining({ method: 'DELETE', credentials: 'include' }));
} /* 测试回调验证：syncOrders reports a timeout only after cancellation and final terminal read remain non-terminal。 */);

test('cancelOrderRefreshJob sends a user-scoped delete command', async () => {
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true, job_id: 'job-cancel', status: 'cancelled' })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	await expect(cancelOrderRefreshJob('job-cancel')).resolves.toEqual({ success: true, job_id: 'job-cancel', status: 'cancelled' });
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/orders/refresh/job-cancel', expect.objectContaining({ method: 'DELETE', credentials: 'include' }));
} /* 测试回调验证：cancelOrderRefreshJob sends a user-scoped delete command。 */);

test('legacy notification channel aliases are normalized for the editor', async () => {
	vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([{ id: 1, name: '旧飞书', type: 'lark', config: 'not-json', enabled: true }])));
	const result = await getNotificationChannels(); /* result 表示处理结果。 */
	expect(result.data?.[0]).toMatchObject({ type: 'feishu', config: {} });
} /* 测试回调验证：legacy notification channel aliases are normalized for the editor。 */);

const jsonResponse = (body: unknown) => new Response(JSON.stringify(body), {
  status: 200,
  headers: { 'content-type': 'application/json' },
}); /* jsonResponse 表示json接口响应结果。 */

test('getOrders normalizes backend order fields', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
    orders: [{ order_id: 'o1', order_status: 'received', quantity: '2', received_at: '2026-08-20T08:00:00Z' }, { order_id: 'o2', order_status: 'refunded', quantity: '1', refunded_at: '2026-08-20T09:00:00Z' }],
    total: 2,
	settlement_summary: { order_count: 3, gross_amount: '405.00', service_fee: '6.48', pending_amount: '398.52', service_fee_rate: '1.6%' },
  })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);
  const result = await getOrders(undefined, 'all', 1, 20, ' buyer '); /* result 表示处理结果。 */
  expect(result.data[0]).toMatchObject({ id: 'o1', status: 'received', quantity: 2, received_at: '2026-08-20T08:00:00Z' });
  expect(result.data[1]).toMatchObject({ id: 'o2', status: 'refunded', quantity: 1, refunded_at: '2026-08-20T09:00:00Z' });
  expect(result.total).toBe(2);
	expect(result.settlement_summary).toEqual({ order_count: 3, gross_amount: '405.00', service_fee: '6.48', pending_amount: '398.52', service_fee_rate: '1.6%' });
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/orders?page=1&page_size=20&search=buyer', expect.objectContaining({ method: 'GET' }));
} /* 测试回调验证：getOrders normalizes backend order fields。 */);

test('getOrders maps unsupported backend statuses to unknown', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
    data: [{ order_id: 'o-unknown', order_status: 'legacy_status' }],
  })));
  const result = await getOrders(); /* result 表示处理结果。 */
  expect(result.data[0].status).toBe('unknown');
} /* 测试回调验证：getOrders maps unsupported backend statuses to unknown。 */);

test('getOrders forwards the created-at half-open range', async () => {
  // fetchMock 记录订单时间范围转换后的 URL 查询参数。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ data: [], total: 0 }));
  vi.stubGlobal('fetch', fetchMock);
  await getOrders(undefined, 'all', 1, 20, '', {
    createdFrom: '2026-08-20T00:00:00.000Z',
    createdTo: '2026-08-21T00:00:00.000Z',
  });
  // requestURL 是订单列表调用实际提交的编码 URL。
  const requestURL = String(fetchMock.mock.calls[0]?.[0]);
  expect(requestURL).toContain('created_from=2026-08-20T00%3A00%3A00.000Z');
  expect(requestURL).toContain('created_to=2026-08-21T00%3A00%3A00.000Z');
} /* 测试回调验证：getOrders 转发订单创建时间半开区间。 */);

test('getOrders forwards the inclusive paid-amount range', async () => {
  // fetchMock 记录订单实付金额范围转换后的 URL 查询参数。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ data: [], total: 0 }));
  vi.stubGlobal('fetch', fetchMock);
  await getOrders(undefined, 'all', 1, 20, '', { minAmount: '135.00', maxAmount: '690' });
  // requestURL 是订单列表调用实际提交的编码 URL。
  const requestURL = String(fetchMock.mock.calls[0]?.[0]);
  expect(requestURL).toContain('min_amount=135.00');
  expect(requestURL).toContain('max_amount=690');
} /* 测试回调验证：getOrders 转发订单实付金额包含范围。 */);

test('订单查询和导入 API 转发外部取消信号', async () => {
  // fetchMock 是同时验证订单查询和文件上传请求控制参数的替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ data: [], total_pages: 1 }))
    .mockResolvedValueOnce(jsonResponse({ success_count: 1, failed_count: 0, results: [] }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是 feature Hook 传入 API 的取消控制器。
  const controller = new AbortController();
  await getOrders(undefined, 'all', 1, 20, '', { signal: controller.signal });
  await importOrders(new FormData(), { signal: controller.signal });
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/orders?page=1&page_size=20', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/orders/import', expect.objectContaining({ signal: expect.any(AbortSignal) }));
} /* 测试回调验证：订单查询和导入 API 转发外部取消信号。 */);

test('Dashboard 统计 API 转发外部取消信号', async () => {
  // fetchMock 验证 Dashboard 的概览、趋势和订单明细共用同一个取消信号。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ total_cookies: 1, active_cookies: 1, available_card_stock: 2 }))
    .mockResolvedValueOnce(jsonResponse({ data: [{ id: 'account-1', enabled: true, nickname: '店铺一', remark: '主店' }] }))
    .mockResolvedValueOnce(jsonResponse({ statuses: { 'account-1': { state: 'online', connected: true, failures: 0, updated_at: '' } } }))
    .mockResolvedValueOnce(jsonResponse({ revenue_stats: { total_amount: 1, total_orders: 1 }, daily_stats: [] }))
    .mockResolvedValueOnce(jsonResponse({ orders: [], total: 0, truncated: false }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是 Dashboard feature Hook 传入 API 的请求控制器。
  const controller = new AbortController();
  await getDashboardStats({ signal: controller.signal });
  // dashboardAccounts 是选择器需要的非敏感账号身份摘要。
  const dashboardAccounts = await getDashboardAccounts({ signal: controller.signal });
  await getDashboardRuntimeStatuses({ signal: controller.signal });
  await getOrderAnalytics({ start_date: '2026-08-15', end_date: '2026-08-15', account_id: 'account-1' }, { signal: controller.signal });
  await getValidOrders({ start_date: '2026-08-15', end_date: '2026-08-15', account_id: 'account-1' }, { signal: controller.signal });
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/analytics/dashboard', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/accounts/details', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/accounts/runtime-status', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, expect.stringContaining('account_id=account-1'), expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, expect.stringContaining('account_id=account-1'), expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(dashboardAccounts).toEqual([{ id: 'account-1', enabled: true, nickname: '店铺一', remark: '主店' }]);
} /* 测试回调验证：Dashboard 统计 API 转发外部取消信号。 */);

test('Chat 历史订单 API 使用精确会话参数并转发取消信号', async () => {
	// fetchMock 是故意按“取消、完成、已发货”乱序返回的历史订单 HTTP 替身。
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
		summary: { total: 3, current_chat: 1, completed: 1 },
		orders: [
			{ order_id: 'order-cancelled', item_id: 'item-3', quantity: '1', amount: '30', status: 'cancelled', association: 'same_buyer' },
			{ order_id: 'order-completed', item_id: 'item-2', quantity: '1', amount: '20', status: 'completed', association: 'same_buyer' },
			{ order_id: 'order-shipped', item_id: 'item-1', quantity: '1', amount: '10', status: 'shipped', association: 'current_chat' },
		],
		total: 3, page: 1, page_size: 20, total_pages: 1, truncated: false,
	}));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是会话切换时由历史订单 Hook 取消的请求控制器。
  const controller = new AbortController();
  // result 是前端适配后的历史订单分页结果。
  const result = await getConversationOrderContext('account-1', 'chat-1', 'buyer-1', 'all', 1, { signal: controller.signal });
  expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/api/v1/orders/conversation-context?'), expect.objectContaining({ signal: expect.any(AbortSignal) }));
  // requestedURL 是请求中经过编码的精确会话上下文地址。
  const requestedURL = String(fetchMock.mock.calls[0][0]);
  expect(requestedURL).toContain('account_id=account-1');
  expect(requestedURL).toContain('chat_id=chat-1');
  expect(requestedURL).toContain('buyer_id=buyer-1');
	expect(result.orders.map(/* order 是用于验证业务优先级的当前订单卡。 */ order => order.id)).toEqual([
		'order-completed', 'order-shipped', 'order-cancelled',
	]);
	expect(result.orders[1]).toMatchObject({ quantity: 1, status: 'shipped', association: 'current_chat' });
} /* 测试回调验证：Chat 历史订单 API 使用精确会话参数并转发取消信号。 */);

test('Settings 配置、模型和凭据 API 转发外部取消信号', async () => {
  // fetchMock 验证 Settings 的读取、模型发现和凭据保存共享取消控制能力。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ data: { log_level: 'info' } }))
    .mockResolvedValueOnce(jsonResponse({ models: ['model-a'] }))
    .mockResolvedValueOnce(jsonResponse({ authenticated: true, username: 'admin' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, message: '已更新' }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是 Settings feature Hook 使用的请求控制器。
  const controller = new AbortController();
  await getSystemSettings({ signal: controller.signal });
  await fetchAIModels('https://ai.example.com', 'secret', { signal: controller.signal });
  await verifySession({ signal: controller.signal });
  await updateLoginCredentials({ current_password: 'old', new_username: 'admin' }, { signal: controller.signal });
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/settings/system', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/settings/ai-models', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/session', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/session/credentials', expect.objectContaining({ signal: expect.any(AbortSignal) }));
} /* 测试回调验证：Settings 配置、模型和凭据 API 转发外部取消信号。 */);

test('通知渠道和 SMTP API 转发外部取消信号', async () => {
  // fetchMock 验证渠道读取、保存和 SMTP 读写都支持取消控制。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse([]))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ data: {} }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  // controller 是通知 feature 传给服务 API 的共享取消控制器。
  const controller = new AbortController();
  await getNotificationChannels({ signal: controller.signal });
  await updateNotificationChannel('channel-1', { enabled: false }, { signal: controller.signal });
  await getSystemSettings({ signal: controller.signal });
  await updateSystemSettings({ smtp_server: 'smtp.example.com' }, { signal: controller.signal });
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/notifications/channels', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/notifications/channels/channel-1', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/settings/system', expect.objectContaining({ signal: expect.any(AbortSignal) }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/settings/system', expect.objectContaining({ signal: expect.any(AbortSignal) }));
} /* 测试回调验证：通知渠道和 SMTP API 转发外部取消信号。 */);

test('getShippingRulesPage sends filters and preserves pagination metadata', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
    data: [{ id: 7, name: '付款规则', trigger_type: 'order_paid', enabled: false, actions: [] }],
    total: 21,
    page: 2,
    page_size: 20,
    total_pages: 2,
    trigger_counts: { order_paid: 8, buyer_reviewed: 7, review_missing_timeout: 6 },
  })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  const result = await getShippingRulesPage({
    cookieId: 'acc1',
    triggerType: 'order_paid',
    enabled: false,
    search: '  商品 ',
    page: 2,
    pageSize: 20,
  }); /* result 表示处理结果。 */

  expect(result).toMatchObject({
    total: 21,
    page: 2,
    page_size: 20,
    total_pages: 2,
    trigger_counts: { order_paid: 8, buyer_reviewed: 7, review_missing_timeout: 6 },
  });
  expect(result.data[0]).toMatchObject({ id: '7', name: '付款规则', enabled: false });
  expect(fetchMock).toHaveBeenCalledWith(
	    '/api/v1/automation-rules?page=2&page_size=20&cookie_id=acc1&trigger_type=order_paid&enabled=false&search=%E5%95%86%E5%93%81',
    expect.objectContaining({ method: 'GET', credentials: 'include' }),
  );
} /* 测试回调验证：getShippingRulesPage sends filters and preserves pagination metadata。 */);

test('getValidOrders accepts wrapped responses', async () => {
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
    orders: [{ order_id: 'o2', order_status: 'completed', quantity: '3' }],
	})); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	vi.spyOn(Date.prototype, 'getTimezoneOffset').mockReturnValue(-480);
  const result = await getValidOrders({ start_date: '2026-01-01', end_date: '2026-01-02' }); /* result 表示处理结果。 */
  expect(result).toEqual({
    orders: [expect.objectContaining({ id: 'o2', status: 'completed', quantity: 3 })],
    total: 1,
    truncated: false,
  });
	expect(fetchMock.mock.calls[0][0]).toContain('timezone_offset_minutes=480');
} /* 测试回调验证：getValidOrders accepts wrapped responses。 */);

test('getOrderAnalytics sends the browser timezone offset', async () => {
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ revenue_stats: {}, daily_stats: [], status_stats: [], city_stats: [] })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	vi.spyOn(Date.prototype, 'getTimezoneOffset').mockReturnValue(-330);
	await getOrderAnalytics({ start_date: '2026-01-01', end_date: '2026-01-02' });
	expect(fetchMock.mock.calls[0][0]).toContain('timezone_offset_minutes=330');
} /* 测试回调验证：getOrderAnalytics sends the browser timezone offset。 */);

test('getOrderAnalytics 支持数字天数参数', /* 当前回调验证订单分析默认日期范围构造。 */ async () => {
  // fetchMock 是数字天数分析请求的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ revenue_stats: {}, daily_stats: [], status_stats: [], city_stats: [] }));
  vi.stubGlobal('fetch', fetchMock);
  await getOrderAnalytics(3);
  expect(fetchMock.mock.calls[0][0]).toContain('/api/v1/analytics/orders?');
  expect(fetchMock.mock.calls[0][0]).toContain('start_date=');
  expect(fetchMock.mock.calls[0][0]).toContain('end_date=');
});

test('getOrders 序列化账号和状态筛选参数', /* 当前回调验证订单查询筛选参数分支。 */ async () => {
  // fetchMock 是订单筛选请求的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ data: [], total: 0 }));
  vi.stubGlobal('fetch', fetchMock);
  await getOrders('account-1', 'pending_ship');
  expect(fetchMock.mock.calls[0][0]).toContain('cookie_id=account-1');
  expect(fetchMock.mock.calls[0][0]).toContain('status=pending_ship');
});

test('paid orders are normalized to pending shipment', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ data: [{ order_id: 'o-paid', order_status: 'paid' }] })));
  const result = await getOrders(); /* result 表示处理结果。 */
  expect(result.data[0].status).toBe('pending_ship');
} /* 测试回调验证：paid orders are normalized to pending shipment。 */);

test('退款拒绝与无需寄件都通过认证 multipart 路径提交原始图片', /* orderProofMultipartCase 验证前端不提交任意图片 URL。 */ async () => {
	// fetchMock 返回两个动作的确定性成功响应。
	const fetchMock = vi.fn()
		.mockResolvedValueOnce(jsonResponse({ success: true, status: 'succeeded', order_id: 'order-1', refund_id: 'refund-1', action: 'reject' }))
		.mockResolvedValueOnce(jsonResponse({ success: true, status: 'succeeded', order_id: 'order-1' }));
	vi.stubGlobal('fetch', fetchMock);
	// proofImage 是两个业务入口共用的浏览器内存 PNG。
	const proofImage = new File([new Uint8Array([137, 80, 78, 71])], 'proof.png', { type: 'image/png' });
	await refuseMerchantRefund('seller-1', 'order-1', 'reason-1', '已经发货', 1, [proofImage]);
	await shipOrderWithEvidence('seller-1', 'order-1', '无需寄件', [proofImage]);
	expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/orders/order-1/refund-refuse', expect.objectContaining({ method: 'POST', credentials: 'include', body: expect.any(FormData) }));
	expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/orders/order-1/ship-with-evidence', expect.objectContaining({ method: 'POST', credentials: 'include', body: expect.any(FormData) }));
	// refundForm 是拒绝退款请求实际提交的 multipart 表单。
	const refundForm = fetchMock.mock.calls[0][1]?.body as FormData;
	// shipmentForm 是无需寄件请求实际提交的 multipart 表单。
	const shipmentForm = fetchMock.mock.calls[1][1]?.body as FormData;
	expect(refundForm.get('reason_id')).toBe('reason-1');
	expect(refundForm.get('negotiation_cents')).toBe('1');
	expect(refundForm.getAll('images')).toEqual([proofImage]);
	expect(shipmentForm.getAll('images')).toEqual([proofImage]);
});

test('无图片发货使用 JSON 避免 WKWebView 空 multipart 卡住', /* shipmentWithoutImagesCase 验证零凭证不创建上传流。 */ async () => {
	// fetchMock 返回无图片发货的确定性成功响应。
	const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse({ success: true, status: 'succeeded', order_id: 'order-json' }));
	vi.stubGlobal('fetch', fetchMock);
	await shipOrderWithEvidence('seller-1', 'order-json', '无需寄件', []);
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/orders/order-json/ship-with-evidence', expect.objectContaining({
		method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ account_id: 'seller-1', trade_text: '无需寄件' }),
	}));
});

test('completeQRVerification sends the immutable target account and explicit captcha mode', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true, account_id: 'acc1' })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);
  await completeQRVerification('session-1', 'acc1', 'manual');
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/qr-login/complete-verification/session-1', expect.objectContaining({
    method: 'POST',
    body: JSON.stringify({ target_account_id: 'acc1', captcha_mode: 'manual' }),
  }));
} /* 测试回调验证：completeQRVerification sends the immutable target account and explicit captcha mode。 */);

test('deleteItemPublishBatch removes an abandoned preview', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);
  await deleteItemPublishBatch('preview-1');
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/items/publish-batches/preview-1', expect.objectContaining({
    method: 'DELETE',
    credentials: 'include',
  }));
} /* 测试回调验证：deleteItemPublishBatch removes an abandoned preview。 */);

test('publishItem allows virtual publishing without an optional location', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);
  await publishItem({
    cookie_id: 'acc1',
    title: '虚拟商品',
    description: '',
    price: '12.50',
    quantity: 1,
    postage_mode: 'none',
    images: [],
  });
  const body = fetchMock.mock.calls[0][1].body as FormData; /* body 表示请求体。 */
  expect(body.get('location')).toBeNull();
} /* 测试回调验证：publishItem allows virtual publishing without an optional location。 */);

test('getItems normalizes multi-spec flags from backend values', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([{
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    item_title: '普通商品',
    is_multi_spec: '0',
    multi_quantity_delivery: 0,
  }, {
    cookie_id: 'cookie-1',
    item_id: 'item-2',
    item_title: '多规格商品',
    is_multi_spec: '1',
    multi_quantity_delivery: 1,
    skus: [{ sku_id: '6286771311126', inventory_id: '1114713017577732954', enabled: 1, cost_cents: null, properties: [] }],
  }])));

  const items = await getItems(); /* items 表示商品集合。 */
  expect(items[0]).toMatchObject({
    id: 'cookie-1-item-1',
    is_multi_spec: false,
    is_multi_qty_ship: false,
    multi_quantity_delivery: false,
  });
  expect(items[1]).toMatchObject({
    id: 'cookie-1-item-2',
    is_multi_spec: true,
    is_multi_qty_ship: true,
    multi_quantity_delivery: true,
    skus: [expect.objectContaining({ sku_id: '6286771311126', inventory_id: '1114713017577732954', enabled: true, cost_cents: null })],
  });
} /* 测试回调验证：getItems normalizes multi-spec flags from backend values。 */);

test('getItems forwards the selected account filter', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse([])); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await getItems('account-2');

  expect(fetchMock).toHaveBeenCalledWith('/api/v1/items?cookie_id=account-2', expect.objectContaining({
    method: 'GET',
    credentials: 'include',
  }));
} /* 测试回调验证：getItems forwards the selected account filter。 */);

test('getSystemSettings normalizes numeric renewal retention', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
    ai_model: 'qwen-plus',
    renewal_log_retention_days: 'invalid',
  })));

  const settings = await getSystemSettings(); /* settings 表示settings。 */
  expect(settings.ai_model).toBe('qwen-plus');
  expect(settings.renewal_log_retention_days).toBe(10);
} /* 测试回调验证：getSystemSettings normalizes numeric renewal retention。 */);

// getSystemSettings 脱敏响应测试验证客户端只接收敏感配置状态。
test('getSystemSettings keeps only sensitive configuration markers', /* 当前回调验证脱敏设置归一化。 */ async () => {
  // fetchMock 返回服务端的脱敏设置视图，不包含任何敏感明文。
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({
    ai_api_key_configured: 'true',
    smtp_password_configured: 'false',
  })));

  // settings 是客户端归一化后的脱敏设置对象。
  const settings = await getSystemSettings();
  expect(settings.ai_api_key).toBeUndefined();
  expect(settings.ai_api_key_configured).toBe(true);
  expect(settings.smtp_password_configured).toBe(false);
});

test('logout calls backend session invalidation route', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await logout();
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/session/logout', expect.objectContaining({
    method: 'POST',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({});
} /* 测试回调验证：logout calls backend session invalidation route。 */);

test('account cookie APIs include login_method when provided', async () => {
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse({ success: true })) /* 模拟取消操作的成功响应。 */); /* fetchMock 是本测试替代浏览器网络层的请求桩。 */
  vi.stubGlobal('fetch', fetchMock);

  await addAccount('acc1', 'unb=acc1', 'qr_scan');
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/accounts', expect.objectContaining({
    method: 'POST',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
    id: 'acc1',
    value: 'unb=acc1',
    login_method: 'qr_scan',
  });

  await updateAccountCookie('acc1', 'unb=acc1; x=1', 'qr_scan');
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/accounts/acc1', expect.objectContaining({
    method: 'PUT',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({
    id: 'acc1',
    value: 'unb=acc1; x=1',
    login_method: 'qr_scan',
  });
} /* 测试回调验证：account cookie APIs include login_method when provided。 */);

test('account editor settings use one aggregate request', async () => {
	const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
	vi.stubGlobal('fetch', fetchMock);
	await updateAccountSettings('acc1', {
	  remark: 'main', auto_confirm: false, pause_duration: 5,
	  username: 'user', show_browser: true, channel_ids: [1, 2],
	});
	expect(fetchMock).toHaveBeenCalledTimes(1);
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/accounts/acc1/settings', expect.objectContaining({ method: 'PUT' }));
	expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
	  remark: 'main', auto_confirm: false, pause_duration: 5,
	  username: 'user', show_browser: true, channel_ids: [1, 2],
	});
} /* 测试回调验证：account editor settings use one aggregate request。 */);

test('getAccountDetails normalizes show_browser and never exposes password', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([{
    id: 'acc1',
    enabled: true,
    auto_confirm: true,
    remark: '主账号',
    pause_duration: 0,
    paused_until: 1780000000,
    paused: true,
    username: 'login-user',
    show_browser: '1',
    auto_request_flower_enabled: true,
    request_flower_after_hours: 6,
    request_flower_after_seconds: 10,
    auto_receive_flower_enabled: true,
    receive_flower_show_browser: false,
    receive_flower_timeout_seconds: 180,
    login_password: 'should-not-leak',
  }])));

  const accounts = await getAccountDetails(); /* accounts 表示账号集合。 */
  expect(accounts[0]).toMatchObject({
    id: 'acc1',
    username: 'login-user',
    show_browser: true,
    paused_until: 1780000000,
    paused: true,
    auto_request_flower_enabled: true,
    request_flower_after_hours: 6,
    request_flower_after_seconds: 10,
    auto_receive_flower_enabled: true,
    receive_flower_show_browser: false,
    receive_flower_timeout_seconds: 180,
  });
  expect(accounts[0]).not.toHaveProperty('login_password');
  expect(accounts[0]).not.toHaveProperty('value');
} /* 测试回调验证：getAccountDetails normalizes show_browser and never exposes password。 */);

test('列表 API 将 null 和历史 data 包裹统一为空数组', async () => {
  // fetchMock 依次模拟账号、商品、订单和规则接口的历史空响应。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(null))
    .mockResolvedValueOnce(jsonResponse({ data: null }))
    .mockResolvedValueOnce(jsonResponse(null))
    .mockResolvedValueOnce(jsonResponse({ data: null }));
  vi.stubGlobal('fetch', fetchMock);

  await expect(getAccountDetails()).resolves.toEqual([]);
  await expect(getItems()).resolves.toEqual([]);
  await expect(getOrders()).resolves.toMatchObject({ data: [], total: 0 });
  await expect(getShippingRules()).resolves.toEqual([]);
} /* 回调函数验证列表接口的空集合契约。 */);

test('列表 API 接受历史命名包裹对象并保持具名 UI 结果', async () => {
  // fetchMock 模拟订单、商品、卡密和批次接口各自的历史字段名称。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ orders: [{ order_id: 'o-wrap', order_status: 'paid' }] }))
    .mockResolvedValueOnce(jsonResponse({ items: [{ cookie_id: 'a1', item_id: 'i1', item_title: '商品' }] }))
    .mockResolvedValueOnce(jsonResponse({ cards: [{ id: 1, name: '库存', type: 'data', enabled: true }] }))
    .mockResolvedValueOnce(jsonResponse({ batches: [{ id: 'b1', status: 'running' }] }));
  vi.stubGlobal('fetch', fetchMock);

  await expect(getOrders()).resolves.toMatchObject({ data: [{ id: 'o-wrap', status: 'pending_ship' }] });
  await expect(getItems()).resolves.toMatchObject([{ id: 'a1-i1' }]);
  await expect(getCards()).resolves.toMatchObject([{ id: 1, name: '库存' }]);
  await expect(getItemPublishBatches()).resolves.toEqual([{ id: 'b1', status: 'running' }]);
} /* 回调函数验证历史包裹字段的兼容归一。 */);

test('getAccountDetails 归一化头像地址并保留非法地址', /* 当前回调验证头像缓存参数和 URL 兼容边界。 */ async () => {
  // windowStub 是头像 URL 解析使用的浏览器位置替身。
  vi.stubGlobal('window', { location: { origin: 'http://localhost' } });
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([
    { id: 'acc1', enabled: true, avatar_url: 'https://avatar.alicdn.com/avatar.jpg' },
    { id: 'acc2', enabled: true, avatar_url: 'https://avatar.example.com/avatar.jpg' },
    { id: 'acc3', enabled: true, avatar_url: 'http://[invalid' },
  ])));
  // accounts 是头像地址归一化后的账号列表。
  const accounts = await getAccountDetails();
  expect(accounts[0].avatar_url).toContain('avatar.alicdn.com/avatar.jpg?_v=');
  expect(accounts[1].avatar_url).toBe('https://avatar.example.com/avatar.jpg');
  expect(accounts[2].avatar_url).toBe('http://[invalid');
});

test('updateAccountLoginInfo sends exactly provided fields', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateAccountLoginInfo('acc1', { username: 'login-user', show_browser: false });
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/accounts/acc1/login-info', expect.objectContaining({
    method: 'PUT',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
    username: 'login-user',
    show_browser: false,
  });
} /* 测试回调验证：updateAccountLoginInfo sends exactly provided fields。 */);

test('updateAccountLoginInfo can request explicit password clearing', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateAccountLoginInfo('acc1', { username: 'login-user', clear_password: true, show_browser: false });
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
    username: 'login-user',
    clear_password: true,
    show_browser: false,
  });
} /* 测试回调验证：updateAccountLoginInfo can request explicit password clearing。 */);

test('updateItem sends only the fields selected by the editor', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateItem('acc1', 'item1', { item_title: '改名商品' });
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/items/acc1/item1', expect.objectContaining({
    method: 'PUT',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
    item_title: '改名商品',
  });
} /* 测试回调验证：updateItem sends only the fields selected by the editor。 */);

test('password login service uses upstream-compatible routes', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true, session_id: 'sid', status: 'processing' }))
    .mockResolvedValueOnce(jsonResponse({ status: 'success', account_id: 'acc1', cookie_count: 2 }))
    .mockResolvedValueOnce(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await passwordLogin({ account_id: 'acc1', account: 'u', password: 'p' });
	  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/password-login', expect.objectContaining({
    method: 'POST',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
    account_id: 'acc1',
    account: 'u',
    password: 'p',
  });

  const status = await checkPasswordLoginStatus('sid'); /* status 表示status。 */
  expect(status.status).toBe('success');
	  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/password-login/check/sid', expect.objectContaining({ method: 'GET' }));

  await cancelPasswordLogin('sid');
	  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/password-login/cancel/sid', expect.objectContaining({ method: 'DELETE' }));
} /* 测试回调验证：password login service uses upstream-compatible routes。 */);

test('账号编辑子模块请求支持取消过期响应', async () => {
  // fetchMock 是验证请求取消信号透传的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ can_open_long_login: true, enabled: false }))
    .mockResolvedValueOnce(jsonResponse({ ai_enabled: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, session_id: 'sid' }));
  vi.stubGlobal('fetch', fetchMock);
  const controller = new AbortController(); /* controller 表示controller。 */

  await getLongLoginSettings('acc1', { signal: controller.signal });
  await getAccountAISettings('acc1', { signal: controller.signal });
  await passwordLogin({ account_id: 'acc1', account: 'u', password: 'p' }, { signal: controller.signal });

  expect(fetchMock.mock.calls[0][1].signal).toBeInstanceOf(AbortSignal);
  expect(fetchMock.mock.calls[1][1].signal).toBeInstanceOf(AbortSignal);
  expect(fetchMock.mock.calls[2][1].signal).toBeInstanceOf(AbortSignal);
} /* 测试回调验证：账号编辑子模块请求支持取消过期响应。 */);

test('getShippingRules exposes buyer reviewed gift rules as automation rules', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([{
    id: 12,
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    item_title: '测试商品',
    name: '评价后发送赠品 - 测试商品',
    trigger_type: 'buyer_reviewed',
    enabled: true,
    priority: 90,
    config_json: '{}',
    actions: [{
      id: 33,
      action_type: 'send_card',
      card_id: 7,
      card_name: '赠品库存',
      delivery_count: 1,
      config_json: '{"spec_name":"套餐","spec_value":"赠品"}',
      enabled: true,
      sort_order: 1,
    }],
  }])));

  const rules = await getShippingRules(); /* rules 表示规则集合。 */
  expect(rules[0]).toMatchObject({
    id: '12',
    trigger_type: 'buyer_reviewed',
    card_group_id: 7,
    card_group_name: '赠品库存',
  });
  expect(rules[0].variants[0]).toMatchObject({
    spec_name: '套餐',
    spec_value: '赠品',
    card_id: 7,
  });
} /* 测试回调验证：getShippingRules exposes buyer reviewed gift rules as automation rules。 */);

test('getReplyRules 兼容纯系统事件规则的空关键词集合', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse([{
    group_id: 'system-shipped',
    keywords: null,
    reply: '马上安排',
    type: 'text',
    match_type: 'contains',
    message_scope: 'system',
    message_scopes: ['system'],
    system_types: ['order_shipped'],
    account_ids: ['acc1'],
    enabled: false,
  }])); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  const rules = await getReplyRules('acc1'); /* rules 表示规则集合。 */
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/global-reply-rule-groups', expect.objectContaining({ method: 'GET' }));
  expect(rules[0]).toMatchObject({
    id: 'system-shipped',
    keyword: '',
    keywords: [],
    reply_content: '马上安排',
    match_type: 'contains',
    message_scope: 'system',
    message_scopes: ['system'],
    system_types: ['order_shipped'],
    account_ids: ['acc1'],
    enabled: false,
  });
} /* 测试回调验证：getReplyRules 兼容纯系统事件规则的空关键词集合。 */);

test('getReplyRules 全局读取不依赖当前账号', /* 当前回调验证全局关键词规则读取入口。 */ async () => {
  // fetchMock 模拟全局关键词规则接口返回空集合。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse([]));
  vi.stubGlobal('fetch', fetchMock);
  await expect(getReplyRules()).resolves.toEqual([]);
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/global-reply-rule-groups', expect.objectContaining({ method: 'GET' }));
});

test('关键词规则单条和全部开关使用全局启停接口', /* 当前回调验证开关请求路径和目标状态。 */ async () => {
  // fetchMock 模拟两个关键词启停接口成功返回。
  const fetchMock = vi.fn().mockImplementation(/* requestMock 为每次启停请求创建独立响应体。 */ async () => jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  await setReplyRuleEnabled('group-1', false);
  await setAllReplyRulesEnabled(true);
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/global-reply-rule-groups/group-1/enabled', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ enabled: false }) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/global-reply-rule-groups/enabled', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ enabled: true }) }));
});

test('getCards 解析 JSON 和损坏 JSON 的卡密接口配置', /* 当前回调验证卡密配置归一化边界。 */ async () => {
  // fetchMock 是卡密列表接口的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse([
    { id: 1, name: '有效', api_config: '{"endpoint":"https://example.com"}' },
    { id: 2, name: '损坏', api_config: '{broken' },
  ]));
  vi.stubGlobal('fetch', fetchMock);
  // cards 是卡密配置归一化后的库存列表。
  const cards = await getCards();
  expect(cards[0].api_config).toEqual({ endpoint: 'https://example.com' });
  expect(cards[1].api_config).toBeUndefined();
});

test('getCards 将后端空响应归一化为空列表', /* 当前回调防止空卡密响应阻断自动化规则账号加载。 */ async () => {
  // fetchMock 模拟兼容后端返回 JSON null 的卡密列表响应。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(null));
  vi.stubGlobal('fetch', fetchMock);
  await expect(getCards()).resolves.toEqual([]);
});

test('图片卡密创建和编辑使用 multipart 并生成受管预览地址', /* 当前回调验证拖拽图片 API 不再要求外部 URL。 */ async () => {
  // fetchMock 依次返回图片卡密创建和编辑成功响应。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true, id: 7 }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  // image 是等待与卡券 DTO 一起提交的本地图片文件。
  const image = new File(['png'], '教程图.png', { type: 'image/png' });
  await createCard({ name: '教程图', type: 'image' }, image);
  await updateCard(7, { name: '教程图更新', type: 'image' }, image);
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/cards', expect.objectContaining({ method: 'POST', body: expect.any(FormData) }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/cards/7', expect.objectContaining({ method: 'PUT', body: expect.any(FormData) }));
  // createForm 和 updateForm 是请求适配器生成的 multipart 表单。
  const createForm = fetchMock.mock.calls[0]?.[1]?.body as FormData;
  const updateForm = fetchMock.mock.calls[1]?.[1]?.body as FormData;
	// createdImage 是创建 FormData 为 multipart 文件字段保留的浏览器 File 对象。
	const createdImage = createForm.get('image') as File;
	// updatedImage 是更新 FormData 为 multipart 文件字段保留的浏览器 File 对象。
	const updatedImage = updateForm.get('image') as File;
	expect(createdImage.name).toBe(image.name);
	expect(createdImage.type).toBe(image.type);
  expect(JSON.parse(String(createForm.get('payload')))).toMatchObject({ name: '教程图', type: 'image' });
	expect(updatedImage.name).toBe(image.name);
  // managedReference 是后端 cards.image_url 返回的受管图片引用。
  const managedReference = `card-image://7/${'a'.repeat(64)}.png`;
  expect(isManagedCardImage(managedReference)).toBe(true);
  expect(cardImagePreviewURL({ id: 7, image_url: managedReference })).toBe('/api/v1/cards/7/image');
  expect(cardImagePreviewURL({ id: 8, image_url: 'https://example.com/card.png' })).toBe('https://example.com/card.png');
});

test('默认回复 API 补齐空字段默认值', /* 当前回调验证默认回复字段归一化和保存载荷。 */ async () => {
  // fetchMock 是默认回复读取和保存接口的网络替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ enabled: false }))
	    .mockResolvedValueOnce(jsonResponse({ success: true }))
	    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  await expect(getDefaultReply('account-1')).resolves.toEqual({ cookie_id: 'account-1', enabled: false, reply_content: '', reply_once: false, reply_image_url: '' });
  await updateDefaultReply('account-1', {});
  expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ enabled: false, reply_content: '', reply_once: false, reply_image_url: '' });
});

test('updateReplyRule preserves keyword image metadata when saving text edits', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateReplyRule({ id: '42', keyword: '发货', reply_content: '稍后安排', item_id: 'item-1' }, 'acc1');

  expect(fetchMock).toHaveBeenCalledTimes(1);
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/global-reply-rule-groups/42', expect.objectContaining({
    method: 'PUT',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
		keywords: ['发货'], reply: '稍后安排', item_id: 'item-1', type: 'text', image_url: '', match_type: 'contains',
		message_scope: 'customer', message_scopes: ['customer'], system_types: [], account_ids: [], enabled: true,
		reply_interval_seconds: 1, send_delay_seconds: 0,
  });
} /* 测试回调验证：updateReplyRule preserves keyword image metadata when saving text edits。 */);

test('updateReplyRule clears stale content when switching reply type', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateReplyRule({ id: '42', keyword: '发货', type: 'image', image_url: 'https://img.example/new.png' }, 'acc1');
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
		keywords: ['发货'], reply: '', item_id: '', type: 'image', image_url: 'https://img.example/new.png', match_type: 'contains',
		message_scope: 'customer', message_scopes: ['customer'], system_types: [], account_ids: [], enabled: true,
		reply_interval_seconds: 1, send_delay_seconds: 0,
  });
} /* 测试回调验证：updateReplyRule clears stale content when switching reply type。 */);

test('deleteReplyRule deletes one stable keyword row instead of replacing the list', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await deleteReplyRule('42', 'acc1');
  expect(fetchMock).toHaveBeenCalledTimes(1);
	expect(fetchMock).toHaveBeenCalledWith('/api/v1/global-reply-rule-groups/42', expect.objectContaining({
    method: 'DELETE',
    credentials: 'include',
  }));
} /* 测试回调验证：deleteReplyRule deletes one stable keyword row instead of replacing the list。 */);

test('createNotificationChannel persists email recipient as to_email config', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await createNotificationChannel({
    name: '邮件通知',
    type: 'email',
    config: {
      smtp_server: 'smtp.example.com',
      smtp_port: 587,
      smtp_user: 'from@example.com',
      smtp_password: 'secret',
      to_email: 'to@example.com',
    },
  });

  const body = JSON.parse(fetchMock.mock.calls[0][1].body); /* body 表示请求体。 */
  expect(body.type).toBe('email');
  expect(JSON.parse(body.config)).toMatchObject({
    to_email: 'to@example.com',
  });
  expect(JSON.parse(body.config)).not.toHaveProperty('from');
} /* 测试回调验证：createNotificationChannel persists email recipient as to_email config。 */);

test('createNotificationChannel allows email channel to rely on system SMTP settings', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await createNotificationChannel({
    name: '邮件通知',
    type: 'email',
    config: {
      to_email: 'to@example.com',
    },
  });

  const body = JSON.parse(fetchMock.mock.calls[0][1].body); /* body 表示请求体。 */
  expect(body.type).toBe('email');
  expect(JSON.parse(body.config)).toEqual({
    to_email: 'to@example.com',
  });
} /* 测试回调验证：createNotificationChannel allows email channel to rely on system SMTP settings。 */);

test('updateNotificationChannel supports partial enabled updates', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateNotificationChannel('7', { enabled: false });

	expect(fetchMock).toHaveBeenCalledWith('/api/v1/notifications/channels/7', expect.objectContaining({
    method: 'PUT',
    credentials: 'include',
  }));
  expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
    enabled: false,
  });
} /* 测试回调验证：updateNotificationChannel supports partial enabled updates。 */);

test('updateNotificationChannel serializes config and event types', /* 当前回调验证通知渠道请求体序列化。 */ async () => {
  // fetchMock 是通知渠道更新接口的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  await updateNotificationChannel('7', { config: { server_url: 'https://example.com' }, event_types: ['system_error'] });
  // body 是通知渠道更新请求体。
  const body = JSON.parse(fetchMock.mock.calls[0][1].body);
  expect(body.config).toBe(JSON.stringify({ server_url: 'https://example.com' }));
  expect(body.event_types).toBe(JSON.stringify(['system_error']));
});

test('getMessageNotifications 展开数组并忽略非法绑定值', /* 当前回调验证消息通知响应归一化。 */ async () => {
  // fetchMock 是消息通知接口的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
    'account-1': [{ channel_id: 1, channel_name: '邮件', enabled: true }],
    'account-2': null,
  }));
  vi.stubGlobal('fetch', fetchMock);
  await expect(getMessageNotifications()).resolves.toEqual({ success: true, data: [{ cookie_id: 'account-1', channel_id: 1, channel_name: '邮件', enabled: true }] });
});

test('publishItem 序列化图片和地点字段', /* 当前回调验证商品发布 multipart 请求体。 */ async () => {
  // fetchMock 是商品发布上传接口的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);
  // image 是待上传的商品图片文件。
  const image = new File(['image'], 'item.png', { type: 'image/png' });
  await publishItem({ cookie_id: 'account-1', title: '商品', description: '描述', price: '10', quantity: 2, postage_mode: 'free', images: [image], location: { area: '区域', city: '城市', division_id: '1', longitude: 120, latitude: 30, poi_id: 'poi-1', poi_name: '地点', province: '省' } });
  // body 是商品发布 multipart 请求体。
  const body = fetchMock.mock.calls[0][1].body as FormData;
  expect(body.get('location')).toContain('poi-1');
  expect(body.getAll('images')).toHaveLength(1);
});

test('通知事件字段支持 JSON 数组和分隔符格式', /* 当前回调验证通知事件响应解析兼容性。 */ async () => {
  // fetchMock 是返回多种通知事件编码的网络替身。
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse([
    { id: 1, name: 'JSON', type: 'bark', config: '{}', event_types: '["system_error", "order_paid"]', enabled: true },
    { id: 2, name: '分隔符', type: 'bark', config: '{}', event_types: 'system_error, order_paid; buyer_reviewed', enabled: true },
    { id: 3, name: '数组', type: 'bark', config: '{}', event_types: ['system_error'], enabled: true },
  ]));
  vi.stubGlobal('fetch', fetchMock);
  // result 是通知事件字段解析后的渠道列表。
  const result = await getNotificationChannels();
  expect(result.data?.[0].event_types).toEqual(['system_error', 'order_paid']);
  expect(result.data?.[1].event_types).toEqual(['system_error', 'order_paid', 'buyer_reviewed']);
  expect(result.data?.[2].event_types).toEqual(['system_error']);
});

test('updateShippingRule posts buyer reviewed gift payload to automation-rules', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true, id: 1 })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateShippingRule({
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    trigger_type: 'buyer_reviewed',
    enabled: true,
    variants: [{
      spec_name: '',
      spec_value: '',
      card_id: 7,
      delivery_count: 1,
      enabled: true,
    }],
  });

	  expect(fetchMock).toHaveBeenCalledWith('/api/v1/automation-rules', expect.objectContaining({
    method: 'POST',
    credentials: 'include',
  }));
  const body = JSON.parse(fetchMock.mock.calls[0][1].body); /* body 表示请求体。 */
  expect(body).toMatchObject({
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    name: '评价后发送赠品 - item-1',
    trigger_type: 'buyer_reviewed',
  });
  expect(body.actions).toEqual([
    expect.objectContaining({
      action_type: 'send_card',
      card_id: 7,
      sort_order: 1,
    }),
  ]);
} /* 测试回调验证：updateShippingRule posts buyer reviewed gift payload to automation-rules。 */);

test('updateShippingRule posts every matching card action before confirm shipment', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true, id: 3 })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateShippingRule({
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    trigger_type: 'order_paid',
    enabled: true,
    variants: [
      {
        spec_name: '套餐',
        spec_value: '30天',
        card_id: 8,
        delivery_count: 1,
        enabled: true,
      },
      {
        spec_name: '套餐',
        spec_value: '30天',
        card_id: 9,
        delivery_count: 2,
        enabled: true,
        delay_override: true,
        delay_seconds: 0,
      },
    ],
  });

  const body = JSON.parse(fetchMock.mock.calls[0][1].body); /* body 表示请求体。 */
  expect(body.trigger_type).toBe('order_paid');
  expect(body.actions).toEqual([
    expect.objectContaining({
      action_type: 'send_card',
      card_id: 8,
      sort_order: 1,
    }),
    expect.objectContaining({
      action_type: 'send_card',
      card_id: 9,
      delivery_count: 2,
      sort_order: 2,
    }),
    expect.objectContaining({
      action_type: 'confirm_shipment',
      sort_order: 3,
    }),
  ]);
  expect(JSON.parse(body.actions[0].config_json)).toEqual({ spec_name: '套餐', spec_value: '30天', delay_override: false });
  expect(JSON.parse(body.actions[1].config_json)).toEqual({ spec_name: '套餐', spec_value: '30天', delay_override: true });
  expect(body.actions[1].delay_seconds).toBe(0);
} /* 测试回调验证：updateShippingRule posts every matching card action before confirm shipment。 */);

test('updateShippingRule preserves text actions while editing card variants', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true, id: 4 })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateShippingRule({
    id: '4',
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    trigger_type: 'order_paid',
    variants: [{ spec_name: '', spec_value: '', card_id: 8, delivery_count: 1, enabled: true }],
    actions: [{ action_type: 'send_text', message_template: '发货提示', enabled: true, sort_order: 2 }],
  });

  const body = JSON.parse(fetchMock.mock.calls[0][1].body); /* body 表示请求体。 */
  expect(body.actions.map((action: { /* action_type 表示自动化动作类型。 */ action_type: string }) => action.action_type /* action 是当前待断言排序的自动化动作。 */)).toEqual([
    'send_card',
    'send_text',
    'confirm_shipment',
  ]);
  expect(body.actions[1].message_template).toBe('发货提示');
} /* 测试回调验证：updateShippingRule preserves text actions while editing card variants。 */);

test('updateShippingRule posts review request text action without card requirement', async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ success: true, id: 2 })); /* fetchMock 表示fetchMock。 */
  vi.stubGlobal('fetch', fetchMock);

  await updateShippingRule({
    cookie_id: 'cookie-1',
    item_id: 'item-1',
    trigger_type: 'review_missing_timeout',
    enabled: true,
    config_json: '{"after_shipped_hours":48,"max_attempts":2}',
    actions: [{
      action_type: 'send_text',
      message_template: '亲，方便的话麻烦给个评价～',
      enabled: true,
      sort_order: 1,
    }],
  });

  const body = JSON.parse(fetchMock.mock.calls[0][1].body); /* body 表示请求体。 */
  expect(body.trigger_type).toBe('review_missing_timeout');
  expect(body.actions).toEqual([
    expect.objectContaining({
      action_type: 'send_text',
      card_id: 0,
      message_template: '亲，方便的话麻烦给个评价～',
    }),
  ]);
} /* 测试回调验证：updateShippingRule posts review request text action without card requirement。 */);

// 会话 API 使用版本化兼容入口。
const runVersionedSessionAPITest = async () => {
  // fetchMock 是会话 API 请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true, authenticated: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, authenticated: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, authenticated: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, authenticated: true }));
  vi.stubGlobal('fetch', fetchMock);

  await login({ username: 'admin', password: 'pw' });
  await initializeAdmin('long-password');
  await verifySession();
  await logout();

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/session/login', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/session/initialize', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/session', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/session/logout', expect.objectContaining({ method: 'POST' }));
};

test('session APIs use versioned compatibility routes', runVersionedSessionAPITest);

// 账号摘要、详情和状态 API 使用版本化兼容入口。
const runVersionedAccountAPITest = async () => {
  // fetchMock 是账号 API 请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse([{ id: 'acc1', enabled: true, remark: '主账号' }]))
    .mockResolvedValueOnce(jsonResponse({ acc1: { state: 'error', connected: false, failures: 0, updated_at: '' } }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await getAccountDetails();
  await getAccountRuntimeStatuses();
  await updateAccountStatus('acc1', false);

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/accounts/details', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/accounts/runtime-status', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/accounts/acc1/status', expect.objectContaining({ method: 'PUT' }));
};

test('account summary and status APIs use versioned compatibility routes', runVersionedAccountAPITest);

// 账号设置、长登录和资料 API 使用版本化兼容入口。
const runVersionedAccountSettingsAPITest = async () => {
  // fetchMock 是账号设置与资料请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true, paused_until: 0, paused: false }))
    .mockResolvedValueOnce(jsonResponse({ can_open_long_login: true, enabled: false }))
    .mockResolvedValueOnce(jsonResponse({ can_open_long_login: true, enabled: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, id: 'acc1', nickname: '主账号', avatar_url: '', profile_error: '' }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, paused_until: 0, paused: false }));
  vi.stubGlobal('fetch', fetchMock);

  await updateAccountSettings('acc1', { remark: '主账号' });
  await getLongLoginSettings('acc1');
  await setLongLoginSettings('acc1', true);
  await refreshAccountProfile('acc1');
  await updateAccountRemark('acc1', '新的备注');
  await updateAccountAutoConfirm('acc1', true);
  await updateAccountPauseDuration('acc1', 15);

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/accounts/acc1/settings', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/accounts/acc1/long-login', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/accounts/acc1/long-login', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/accounts/acc1/refresh-profile', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/accounts/acc1/remark', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/v1/accounts/acc1/auto-confirm', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(7, '/api/v1/accounts/acc1/pause-duration', expect.objectContaining({ method: 'PUT' }));
};

test('account settings and profile APIs use versioned compatibility routes', runVersionedAccountSettingsAPITest);

// 订单列表、详情和更新 API 使用版本化兼容入口。
const runVersionedOrderAPITest = async () => {
  // fetchMock 是订单 API 请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ data: [{ order_id: 'order-1', order_status: 'pending_ship' }], total: 1 }))
    .mockResolvedValueOnce(jsonResponse({ success: true, order_id: 'order-1', data: { order_id: 'order-1', order_status: 'pending_ship' } }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await getOrders();
  await getOrderDetail('order-1');
  await updateOrder('order-1', { status: 'shipped' });

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/orders?page=1&page_size=20', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/orders/order-1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/orders/order-1', expect.objectContaining({ method: 'PUT' }));
};

test('order list, detail, and update APIs use versioned compatibility routes', runVersionedOrderAPITest);

// 订单刷新与批量操作 API 使用版本化兼容入口。
const runVersionedOrderRefreshAPITest = async () => {
  // fetchMock 是订单刷新与批量请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, status: 'succeeded', message: '求花成功' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, status: 'succeeded', message: '已求花' }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await syncSingleOrder('order-1');
  await requestRedFlower('order-1');
  await getRedFlowerStatus('order-1');
  await manualShipOrder(['order-1'], 'status_only');

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/orders/order-1/refresh', expect.objectContaining({ method: 'POST', credentials: 'include' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/orders/order-1/request-red-flower', expect.objectContaining({ method: 'POST', credentials: 'include' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/orders/order-1/request-red-flower', expect.objectContaining({ method: 'GET', credentials: 'include' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/orders/manual-ship', expect.objectContaining({ method: 'POST', credentials: 'include' }));
};

test('order refresh and batch APIs use versioned compatibility routes', runVersionedOrderRefreshAPITest);

// 商品列表、详情、发布、更新和删除 API 使用版本化兼容入口。
const runVersionedItemAPITest = async () => {
  // fetchMock 是商品 API 请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse([]))
    .mockResolvedValueOnce(jsonResponse({ cookie_id: 'acc1', item_id: 'item-1', item_title: '商品' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, item_id: 'item-1' }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await getItems('acc1');
  await getItemDetail('acc1', 'item-1');
  await publishItem({
    cookie_id: 'acc1', title: '商品', description: '', price: '1.00', quantity: 1,
    postage_mode: 'none', images: [],
  });
  await updateItem('acc1', 'item-1', { item_title: '新商品名' });
  await updateItemSKUCost('acc1', 'item-1', 'sku-1', 8800);
  await deleteItem('acc1', 'item-1');

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/items?cookie_id=acc1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/items/acc1/item-1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/items/publish', expect.objectContaining({ method: 'POST', body: expect.any(FormData) }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/items/acc1/item-1', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/items/acc1/item-1/skus/sku-1/cost', expect.objectContaining({ method: 'PUT', body: JSON.stringify({ cost_cents: 8800 }) }));
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/v1/items/acc1/item-1', expect.objectContaining({ method: 'DELETE' }));
};

test('item list, detail, publish, update, and delete APIs use versioned compatibility routes', runVersionedItemAPITest);

// 商品同步、类目推荐和批量发布 API 使用版本化兼容入口。
const runVersionedItemBatchAPITest = async () => {
  // fetchMock 是商品同步和批量发布请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, category: { cat_id: 'cat-1' } }))
    .mockResolvedValueOnce(jsonResponse({ success: true, preview_id: 'preview-1', total: 0, valid: 0, invalid: 0, rows: [] }))
    .mockResolvedValueOnce(jsonResponse({ success: true, batch_id: 'batch-1' }))
    .mockResolvedValueOnce(jsonResponse({ batches: [] }))
    .mockResolvedValueOnce(jsonResponse({ id: 'batch-1', status: 'preview', rows: [] }))
    .mockResolvedValueOnce(jsonResponse({ success: true, status: 'canceled' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, batch_id: 'batch-1' }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await syncItemsFromAccount('acc1');
  await recommendPublishCategory('acc1', '资料');
  await previewItemPublishBatch({
    file: new File(['order_id\nitem-1'], 'items.csv', { type: 'text/csv' }),
    imagesZip: new File(['zip'], 'images.zip', { type: 'application/zip' }),
    defaultCookieId: 'acc1',
    fallbackCategory: { catId: 'cat-1', catName: '资料', channelCatId: 'channel-1', tbCatId: 'tb-1' },
    location: { area: '区域', city: '城市', division_id: '1', longitude: 120, latitude: 30, poi_id: 'poi-1', poi_name: '地点', province: '省' },
    publishIntervalSeconds: 12,
  });
  await startItemPublishBatch('preview-1');
  await getItemPublishBatches(10);
  await getItemPublishBatch('batch-1');
  await cancelItemPublishBatch('batch-1');
  await retryFailedItemPublishBatch('batch-1');
  await deleteItemPublishBatch('batch-1');

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/items/get-all-from-account', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/items/publish-categories/recommend', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/items/publish-batches/preview', expect.objectContaining({ method: 'POST', body: expect.any(FormData) }));
  // previewBody 保存批量预检表单，确保用户设置的间隔进入后端持久化边界。
  const previewBody = fetchMock.mock.calls[2][1].body as FormData;
  expect(previewBody.get('publish_interval_seconds')).toBe('12');
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/items/publish-batches', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/items/publish-batches?limit=10', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/v1/items/publish-batches/batch-1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(7, '/api/v1/items/publish-batches/batch-1/cancel', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(8, '/api/v1/items/publish-batches/batch-1/retry-failed', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(9, '/api/v1/items/publish-batches/batch-1', expect.objectContaining({ method: 'DELETE' }));
};

test('item sync and batch publish APIs use versioned compatibility routes', runVersionedItemBatchAPITest);

// 设置、卡券和通知 API 使用版本化兼容入口。
const runVersionedSettingsCardNotificationAPITest = async () => {
  // fetchMock 是设置、卡券和通知请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ data: {} }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({ ai_enabled: false }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ models: ['qwen-plus'] }))
    .mockResolvedValueOnce(jsonResponse([]))
    .mockResolvedValueOnce(jsonResponse({ success: true, id: 1 }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ id: 1, name: '卡券', type: 'text', text_content: 'CARD' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, total: 0, created: 0, failed: 0, rows: [] }))
    .mockResolvedValueOnce(jsonResponse({ success: true, added: 1 }))
    .mockResolvedValueOnce(jsonResponse([]))
    .mockResolvedValueOnce(jsonResponse({ success: true, id: 1 }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ channel_ids: [] }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await getSystemSettings();
  await updateSystemSettings({ theme_color: 'blue' });
  await getAllAISettings();
  await getAccountAISettings('acc1');
  await updateAccountAISettings('acc1', { ai_enabled: true });
  await fetchAIModels('https://ai.example.com');
  await getCards();
  await createCard({ name: '卡券', type: 'text', text_content: 'CARD' });
  await updateCard(1, { enabled: false });
  await deleteCard(1);
  await getCardDetails(1);
  await batchCreateCards(new File(['name,type,content\n卡券,text,CARD'], 'cards.csv', { type: 'text/csv' }));
  await appendCardData(1, 'CARD-2');
  await getNotificationChannels();
  await createNotificationChannel({ name: '通知', type: 'bark', config: {} });
  await updateNotificationChannel('1', { enabled: false });
  await deleteNotificationChannel('1');
  await getMessageNotifications();
  await setMessageNotification('acc1', 1, true);
  await deleteMessageNotification('1');
  await deleteAccountNotifications('acc1');
  await getAccountBindings('acc1');
  await setAccountBindings('acc1', [1]);
  await testNotificationChannel('1');

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/settings/system', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/settings/system', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/settings/ai-reply', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/settings/ai-reply/acc1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/settings/ai-reply/acc1', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/v1/settings/ai-models', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(7, '/api/v1/cards', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(8, '/api/v1/cards', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(9, '/api/v1/cards/1', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(10, '/api/v1/cards/1', expect.objectContaining({ method: 'DELETE' }));
  expect(fetchMock).toHaveBeenNthCalledWith(11, '/api/v1/cards/1/details', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(12, '/api/v1/cards/batch', expect.objectContaining({ method: 'POST', body: expect.any(FormData) }));
  expect(fetchMock).toHaveBeenNthCalledWith(13, '/api/v1/cards/1/append-data', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(14, '/api/v1/notifications/channels', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(15, '/api/v1/notifications/channels', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(16, '/api/v1/notifications/channels/1', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(17, '/api/v1/notifications/channels/1', expect.objectContaining({ method: 'DELETE' }));
  expect(fetchMock).toHaveBeenNthCalledWith(18, '/api/v1/notifications/messages', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(19, '/api/v1/notifications/accounts/acc1/bindings', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(20, '/api/v1/notifications/messages/1', expect.objectContaining({ method: 'DELETE' }));
  expect(fetchMock).toHaveBeenNthCalledWith(21, '/api/v1/notifications/messages/account/acc1', expect.objectContaining({ method: 'DELETE' }));
  expect(fetchMock).toHaveBeenNthCalledWith(22, '/api/v1/notifications/accounts/acc1/bindings', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(23, '/api/v1/notifications/accounts/acc1/bindings', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(24, '/api/v1/notifications/channels/1/test', expect.objectContaining({ method: 'POST' }));
};

test('settings, card, and notification APIs use versioned compatibility routes', runVersionedSettingsCardNotificationAPITest);

// 聊天和账号任务 API 使用版本化兼容入口。
const runVersionedChatTaskAPITest = async () => {
  // fetchMock 是聊天和账号任务请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ sessions: [], has_more: false }))
    .mockResolvedValueOnce(jsonResponse({ messages: [], has_more: false }))
    .mockResolvedValueOnce(jsonResponse({ sessions: [], has_more: false }))
    .mockResolvedValueOnce(jsonResponse({ messages: [], has_more: false }))
    .mockResolvedValueOnce(jsonResponse({ message: { message_key: 'message-1' } }))
    .mockResolvedValueOnce(jsonResponse({ message: { message_key: 'message-2' } }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ account_id: 'acc1' }))
    .mockResolvedValueOnce(jsonResponse({ account_id: 'acc1' }))
    .mockResolvedValueOnce(jsonResponse({ success: true, summary: { task_type: 'auto_rate' } }));
  vi.stubGlobal('fetch', fetchMock);

  await getChatSessionPage('acc1', 3, undefined, true);
  await getChatMessagePage('acc1', 'chat-1', 4, 9);
  await getChatSessions('acc1');
  await getChatMessages('acc1', 'chat-1', 9);
  await sendChatMessage({ account_id: 'acc1', chat_id: 'chat-1', buyer_id: 'buyer-1', text: '你好' });
  await sendChatImage({ account_id: 'acc1', chat_id: 'chat-1', buyer_id: 'buyer-1', image: new File(['image'], 'chat.png', { type: 'image/png' }) });
  await markChatRead('acc1', 'chat-1', []);
  await getAccountTaskSettings('acc1');
  await updateAccountTaskSettings('acc1', {
    account_id: 'acc1', auto_rate_enabled: true, rate_content: '交易愉快', auto_polish_enabled: false, polish_time: '03:00',
    auto_request_flower_enabled: false, request_flower_after_hours: 24, request_flower_after_seconds: 10, auto_receive_flower_enabled: false,
	receive_flower_show_browser: true, receive_flower_timeout_seconds: 120,
	auto_receipt_reminder_enabled: false, receipt_reminder_after_days: 2, receipt_reminder_time: '10:00', receipt_reminder_message: '请确认收货',
  });
  await runAccountTask('acc1', 'auto_rate');

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/chat/sessions?account_id=acc1&cursor=3&refresh=1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/chat/messages?account_id=acc1&chat_id=chat-1&cursor=4&before_id=9', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/chat/sessions?account_id=acc1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/chat/messages?account_id=acc1&chat_id=chat-1&before_id=9', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/chat/messages', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/v1/chat/images', expect.objectContaining({ method: 'POST', body: expect.any(FormData) }));
  expect(fetchMock).toHaveBeenNthCalledWith(7, '/api/v1/chat/read', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(8, '/api/v1/account-tasks/acc1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(9, '/api/v1/account-tasks/acc1', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(10, '/api/v1/account-tasks/acc1/run', expect.objectContaining({ method: 'POST' }));
};

test('chat and account task APIs use versioned compatibility routes', runVersionedChatTaskAPITest);

// 关键词回复、指定商品回复和默认回复 API 使用版本化兼容入口。
const runVersionedReplyAPITest = async () => {
  // fetchMock 是关键词和默认回复请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse([]))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, id: 7 }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({}))
    .mockResolvedValueOnce(jsonResponse({ enabled: true, reply_content: '欢迎', reply_once: false, reply_image_url: '' }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await getReplyRules('acc1');
  await updateReplyRule({ id: '42', keyword: '发货', reply_content: '稍后安排' }, 'acc1');
  await updateReplyRule({ keyword: '售后', reply_content: '请联系客服' }, 'acc1');
  await deleteReplyRule('42', 'acc1');
  await getDefaultReplies();
  await getDefaultReply('acc1');
  await updateDefaultReply('acc1', { enabled: true, reply_content: '欢迎' });
  await deleteDefaultReply('acc1');
  await clearDefaultReplyRecords('acc1');

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/global-reply-rule-groups', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/global-reply-rule-groups/42', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/v1/global-reply-rule-groups', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, '/api/v1/global-reply-rule-groups/42', expect.objectContaining({ method: 'DELETE' }));
  expect(fetchMock).toHaveBeenNthCalledWith(5, '/api/v1/default-replies', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(6, '/api/v1/default-replies/acc1', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(7, '/api/v1/default-replies/acc1', expect.objectContaining({ method: 'PUT' }));
  expect(fetchMock).toHaveBeenNthCalledWith(8, '/api/v1/default-replies/acc1', expect.objectContaining({ method: 'DELETE' }));
  expect(fetchMock).toHaveBeenNthCalledWith(9, '/api/v1/default-replies/acc1/clear-records', expect.objectContaining({ method: 'POST' }));
};

test('keyword and default reply APIs use versioned compatibility routes', runVersionedReplyAPITest);

// 管理员、仪表盘和订单分析 API 使用版本化兼容入口。
const runVersionedAdminAnalyticsAPITest = async () => {
  // fetchMock 是管理员和统计分析请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ total_users: 1, total_cookies: 1, active_cookies: 1, total_cards: 0, total_keywords: 0, total_orders: 0 }))
    .mockResolvedValueOnce(jsonResponse({ total_cookies: 1, active_cookies: 1, total_cards: 0, available_card_stock: 0, total_keywords: 0, total_orders: 0 }))
    .mockResolvedValueOnce(jsonResponse({ revenue_stats: {}, daily_stats: [], status_stats: [], city_stats: [], item_stats: [] }))
    .mockResolvedValueOnce(jsonResponse({ orders: [], total: 0, page: 1, page_size: 500, truncated: false }));
  vi.stubGlobal('fetch', fetchMock);

  await getAdminStats();
  await getDashboardStats();
  await getOrderAnalytics({ start_date: '2026-01-01', end_date: '2026-01-02' });
  await getValidOrders({ start_date: '2026-01-01', end_date: '2026-01-02' });

  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/admin/stats', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/analytics/dashboard', expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(3, expect.stringContaining('/api/v1/analytics/orders?start_date=2026-01-01&end_date=2026-01-02'), expect.objectContaining({ method: 'GET' }));
  expect(fetchMock).toHaveBeenNthCalledWith(4, expect.stringContaining('/api/v1/analytics/orders/valid?start_date=2026-01-01&end_date=2026-01-02'), expect.objectContaining({ method: 'GET' }));
};

test('admin and analytics APIs use versioned compatibility routes', runVersionedAdminAnalyticsAPITest);

// 二维码生成和状态轮询使用版本化兼容入口。
const runVersionedQRLoginAPITest = async () => {
  // fetchMock 是二维码生成和状态轮询请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true, session_id: 'session-1', qr_code_url: 'data:image/png;base64,abc' }))
    .mockResolvedValueOnce(jsonResponse({ status: 'waiting', session_id: 'session-1' }));
  vi.stubGlobal('fetch', fetchMock);
  await generateQRLogin();
  await checkQRLoginStatus('session-1');
  expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/v1/qr-login/generate', expect.objectContaining({ method: 'POST' }));
  expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/v1/qr-login/check/session-1', expect.objectContaining({ method: 'GET' }));
};

test('QR login generation and polling use versioned routes', runVersionedQRLoginAPITest);

// 密码登录、会话凭证、账号删除、自动化以及剩余订单商品调用使用版本化入口。
const runVersionedRemainingAPITest = async () => {
  // fetchMock 是剩余公共 API 请求的测试替身。
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true, session_id: 'sid' }))
    .mockResolvedValueOnce(jsonResponse({ status: 'failed' }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse([]))
    .mockResolvedValueOnce(jsonResponse({ data: [], total: 0, page: 1, page_size: 10, total_pages: 0 }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ runs: [], pending_tasks: [] }))
    .mockResolvedValueOnce(jsonResponse({ success: true }))
    .mockResolvedValueOnce(jsonResponse({ success: true }));
  vi.stubGlobal('fetch', fetchMock);

  await changePassword('old-password', 'new-password');
  await updateLoginCredentials({ current_password: 'old-password', new_username: 'new-user' });
  await deleteAccount('acc1');
  await passwordLogin({ account_id: 'acc1', account: 'user', password: 'password' });
  await checkPasswordLoginStatus('sid');
  await cancelPasswordLogin('sid');
  await deleteOrder('order-1');
  await createItem('acc1', { item_title: '新商品' });
  await getShippingRules();
  await getShippingRulesPage();
  await updateShippingRule({ cookie_id: 'acc1', trigger_type: 'order_paid' });
  await deleteShippingRule('7');
  await getAutomationIssues();
  await resolveAutomationRun(1, 'retry');
  await resolveDeferredAutomationTask(2, 'dismiss');

  // paths 是请求层实际发出的版本化 URL 顺序。
  const paths: unknown[] = [];
  // index 是当前请求调用在模拟调用列表中的位置。
  let index = 0;
  for (index = 0; index < fetchMock.mock.calls.length; index += 1) {
    paths.push(fetchMock.mock.calls[index][0]);
  }
  expect(paths).toEqual([
    '/api/v1/session/password',
    '/api/v1/session/credentials',
    '/api/v1/accounts/acc1',
    '/api/v1/password-login',
    '/api/v1/password-login/check/sid',
    '/api/v1/password-login/cancel/sid',
    '/api/v1/orders/order-1',
    '/api/v1/items/acc1',
    '/api/v1/automation-rules',
    '/api/v1/automation-rules?page=1&page_size=10',
    '/api/v1/automation-rules',
    '/api/v1/automation-rules/7',
    '/api/v1/automation-issues',
    '/api/v1/automation-runs/1/resolve',
    '/api/v1/automation-pending-tasks/2/resolve',
  ]);
};

test('remaining public APIs use versioned compatibility routes', runVersionedRemainingAPITest);
