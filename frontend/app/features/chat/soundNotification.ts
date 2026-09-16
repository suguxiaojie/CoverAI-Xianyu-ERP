import { useCallback,useSyncExternalStore } from 'react';
import type { ChatMessage } from './api';

// soundStorageKey 是当前浏览器保存聊天提示音偏好的本地键。
const soundStorageKey = 'ydisks.chat.sound.enabled';
// NativeSoundWindow 补充 macOS 原生壳在页面脚本执行前注入的运行标记。
type NativeSoundWindow = Window & { /** __COVERAI_NATIVE_APP__ 表示页面正在允许自动播放的原生 WKWebView 中运行。 */ __COVERAI_NATIVE_APP__?: boolean };
/** isNativeSoundRuntime 判断当前页面是否由已允许自动音频的 macOS 原生壳承载。 */
export const isNativeSoundRuntime = (): boolean => typeof window !== 'undefined' && Boolean((window as NativeSoundWindow).__COVERAI_NATIVE_APP__);
// importantTradeEvents 是需要即时声音提醒的买家交易事件。
const importantTradeEvents = new Set(['order_pending_payment','order_paid','refund_requested']);
// soundListeners 接收音频解锁或开关变化，不包含聊天内容。
const soundListeners = new Set<() => void>();
// playedMessageKeys 保存当前页面已经响铃的平台消息键和时间，避免重复推送重复响铃。
const playedMessageKeys = new Map<string,number>();
// lastSoundByCategory 保存普通消息和各交易事件最近响铃时间。
const lastSoundByCategory = new Map<string,number>();
// ChatSoundKind 区分四种已经验收的本地门铃场景。
type ChatSoundKind = 'ordinary'|'order_pending_payment'|'order_paid'|'refund_requested';
// soundPaths 保存四种提示音对应的项目内静态资源。
const soundPaths:Record<ChatSoundKind,string>={ordinary:'/static/sounds/chat-ordinary.wav',order_pending_payment:'/static/sounds/chat-order-pending.wav',order_paid:'/static/sounds/chat-order-paid.wav',refund_requested:'/static/sounds/chat-refund.wav'};
// soundPriorities 定义重要交易声音对普通消息声音的抢占顺序。
const soundPriorities:Record<ChatSoundKind,number>={ordinary:1,order_pending_payment:2,order_paid:3,refund_requested:3};
// audioCache 复用已经加载的本地音频元素，避免每条消息重新请求资源。
const audioCache=new Map<ChatSoundKind,HTMLAudioElement>();
// currentAudio、currentPriority 保存当前播放对象和优先级，低优先级不得打断重要交易提醒。
let currentAudio:HTMLAudioElement|null=null;
let currentPriority=0;
// nativeSoundRuntime 在模块启动时固定当前容器能力，不随 React 重渲染变化。
const nativeSoundRuntime = isNativeSoundRuntime();
// soundUnlocked 表示当前页面生命周期已获得音频播放权限；原生壳在每次刷新时直接就绪。
let soundUnlocked = nativeSoundRuntime;
// soundEnabled 保存当前容器的本地提示音偏好，首次默认开启且保留用户主动关闭的选择。
let soundEnabled = typeof localStorage === 'undefined' ? true : localStorage.getItem(soundStorageKey) !== 'false';

/** emitSoundState 通知聊天页重新读取提示音状态。 */
const emitSoundState = (): void => soundListeners.forEach(/* listener 是当前状态订阅者。 */ listener => listener());

/** playSound 播放项目内门铃资源，高优先级交易提醒可以抢占普通消息。 */
const playSound = async (kind:ChatSoundKind):Promise<void> => {
	if (!soundEnabled) return;
	// priority 是当前声音的抢占等级，交易提醒高于普通消息。
	const priority=soundPriorities[kind];
	if(currentAudio&&!currentAudio.paused&&priority<currentPriority)return;
	if(currentAudio){currentAudio.pause();currentAudio.currentTime=0;}
	// audio 是当前场景复用或首次创建的本地音频元素。
	const audio=audioCache.get(kind)||new Audio(soundPaths[kind]);
	audioCache.set(kind,audio);audio.volume=1;audio.currentTime=0;
	currentAudio=audio;currentPriority=priority;
	audio.onended=/* 当前回调只在该音频仍是活动对象时释放抢占状态。 */ ()=>{if(currentAudio===audio){currentAudio=null;currentPriority=0;}};
	await audio.play();
};

/** notifyChatSound 对普通客户消息和重要交易卡片执行去重及声音节流。 */
export const notifyChatSound = (message: ChatMessage): void => {
	if (!soundEnabled || !soundUnlocked || message.direction !== 'incoming') return;
	// tradeEvent 是当前结构化交易卡片事件；普通消息为空。
	const tradeEvent = message.system_card?.event || '';
	// ordinary 表示当前消息是普通客户文本、图片或视频。
	const ordinary = message.message_type !== 'system';
	if (!ordinary && !importantTradeEvents.has(tradeEvent)) return;
	// messageKey 是平台消息去重键；缺失时使用稳定本地键。
	const messageKey = message.platform_message_id || message.message_key;
	// now 是声音去重使用的浏览器毫秒时间。
	const now = Date.now();
	if (playedMessageKeys.has(messageKey)) return;
	playedMessageKeys.set(messageKey, now);
	// category 让不同重要交易事件可以分别响铃，普通连续消息两秒内只响一次。
	const category = ordinary ? 'ordinary' : `trade:${tradeEvent}:${message.system_card?.order_id || message.chat_id}`;
	// throttleMs 是普通消息或同一交易事件的声音节流窗口。
	const throttleMs = ordinary ? 2_000 : 5_000;
	if (now-(lastSoundByCategory.get(category) || 0) < throttleMs) return;
	lastSoundByCategory.set(category,now);
	// kind 是普通消息或结构化交易事件对应的已验收音频场景。
	const kind:ChatSoundKind=ordinary?'ordinary':tradeEvent as ChatSoundKind;
	void playSound(kind).catch(/* 普通浏览器播放失败需重新解锁；原生壳的自动播放授权不因输出设备短暂失败而回退。 */ ()=>{soundUnlocked=nativeSoundRuntime;emitSoundState();});
};

/** useChatSoundPreference 暴露当前浏览器提示音状态和用户手势切换动作。 */
export const useChatSoundPreference = (): {
	/** enabled 表示用户当前是否希望接收聊天提示音。 */ enabled:boolean;
	/** unlocked 表示当前容器是否可以在无额外手势时播放音频。 */ unlocked:boolean;
	/** toggle 由用户点击触发，负责首次解锁或切换持久偏好。 */ toggle:()=>Promise<void>;
} => {
	// subscribe 注册 React 外部状态监听并返回清理函数。
	const subscribe = useCallback(/* 当前回调将 React 订阅者加入局部集合。 */ (listener:()=>void):()=>void => { soundListeners.add(listener); return /* 当前清理函数在 Hook 卸载时移除同一订阅者。 */ () => soundListeners.delete(listener); },[]);
	// snapshot 返回提示音偏好和解锁状态的稳定字符串快照。
	const snapshot = useCallback(/* 当前回调生成 useSyncExternalStore 比较用的快照。 */ ():string => `${soundEnabled}:${soundUnlocked}`,[]);
	useSyncExternalStore(subscribe,snapshot,snapshot);
	// toggle 由用户点击触发，首次直接播放普通消息音以取得浏览器权限，之后切换本地偏好。
	const toggle = useCallback(/* 当前回调处理用户主动解锁或切换提示音。 */ async ():Promise<void> => {
		if (!soundUnlocked) {
			soundEnabled = true;
			localStorage.setItem(soundStorageKey,'true');
			await playSound('ordinary');
			soundUnlocked = true;
			emitSoundState();
			return;
		}
		soundEnabled = !soundEnabled;
		localStorage.setItem(soundStorageKey,String(soundEnabled));
		emitSoundState();
		if (soundEnabled) await playSound('ordinary'); else if(currentAudio){currentAudio.pause();currentAudio.currentTime=0;currentAudio=null;currentPriority=0;}
	},[]);
	return {enabled:soundEnabled,unlocked:soundUnlocked,toggle};
};
