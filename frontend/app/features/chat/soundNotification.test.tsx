// @vitest-environment jsdom
import { renderHook } from '@testing-library/react';
import { afterEach,describe,expect,test,vi } from 'vitest';

// NativeSoundWindow 是测试写入 macOS 原生运行标记时使用的局部 Window 扩展。
type NativeSoundWindow = Window & { /** __COVERAI_NATIVE_APP__ 模拟原生壳的文档启动注入。 */ __COVERAI_NATIVE_APP__?: boolean };

describe('chat sound native runtime', /* 当前测试组验证原生 App 刷新后的提示音解锁与偏好。 */ () => {
  afterEach(/* 当前回调清理原生标记、本地偏好和模块缓存。 */ () => {
    delete (window as NativeSoundWindow).__COVERAI_NATIVE_APP__;
    window.localStorage.clear();
    vi.resetModules();
  });

  test('native app defaults to enabled and unlocked after every module load', /* 当前回调模拟 WKWebView 刷新后重新加载聊天模块。 */ async () => {
    (window as NativeSoundWindow).__COVERAI_NATIVE_APP__ = true;
    // soundModule 是注入原生标记后重新初始化的提示音模块。
    const soundModule = await import('./soundNotification');
    // preference 是原生首次默认偏好的 Hook 结果。
    const preference = renderHook(/* 当前回调读取原生提示音快照。 */ () => soundModule.useChatSoundPreference());
    expect(preference.result.current).toMatchObject({ enabled: true, unlocked: true });
  });

  test('native app preserves an explicit disabled preference across refresh', /* 当前回调验证用户主动关闭不会被原生默认值覆盖。 */ async () => {
    (window as NativeSoundWindow).__COVERAI_NATIVE_APP__ = true;
    window.localStorage.setItem('ydisks.chat.sound.enabled', 'false');
    // soundModule 是带已关闭偏好重新初始化的提示音模块。
    const soundModule = await import('./soundNotification');
    // preference 是重新加载后仍已解锁但保持关闭的 Hook 结果。
    const preference = renderHook(/* 当前回调读取已关闭原生偏好。 */ () => soundModule.useChatSoundPreference());
    expect(preference.result.current).toMatchObject({ enabled: false, unlocked: true });
  });

  test('ordinary browser still requires a user gesture after refresh', /* 当前回调保持 Chrome 和 Safari 的原有自动播放安全语义。 */ async () => {
    // soundModule 是不含原生标记的普通浏览器模块。
    const soundModule = await import('./soundNotification');
    // preference 是普通浏览器刷新后的 Hook 结果。
    const preference = renderHook(/* 当前回调读取普通浏器偏好。 */ () => soundModule.useChatSoundPreference());
    expect(preference.result.current).toMatchObject({ enabled: true, unlocked: false });
  });
});
