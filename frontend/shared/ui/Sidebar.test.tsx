// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, test, vi } from 'vitest';
import Sidebar from './Sidebar';

describe('Sidebar', /* 当前测试组验证全局聊天新消息状态在侧边栏中的可见标记。 */ () => {
  afterEach(/* sidebarCleanup 清理每个侧边栏测试的 DOM，避免导航按钮在测试间重复。 */ () => cleanup());
  test('未查看聊天消息时仅在在线聊天入口显示红点', /* 当前回调验证红点出现与消失不依赖页面标题状态。 */ () => {
    // navigationHandler 是侧边栏导航测试使用的无副作用回调。
    const navigationHandler = vi.fn();
    // logoutHandler 是侧边栏注销测试使用的无副作用回调。
    const logoutHandler = vi.fn();
    // collapseHandler 是侧边栏折叠测试使用的无副作用回调。
    const collapseHandler = vi.fn();
    // buildInfo 是渲染侧边栏所需的最小公开构建标识。
    const buildInfo = { version: 'dev', commit: 'test' };
    // view 保存带有新消息状态的侧边栏渲染控制器。
    const view = render(
      <Sidebar
        activeTab="dashboard"
        collapsed={false}
        onToggleCollapsed={collapseHandler}
        onNavigate={navigationHandler}
        onLogout={logoutHandler}
        buildInfo={buildInfo}
        hasUnreadChatMessage
      />,
    );
    // unreadIndicator 表示挂载在聊天图标右上角的未读提示元素。
    const unreadIndicator = screen.getByLabelText('在线聊天有未读消息');
    expect(unreadIndicator).toBeTruthy();
    expect(unreadIndicator.classList.contains('absolute')).toBe(true);
    expect(unreadIndicator.classList.contains('-right-1.5')).toBe(true);
    expect(unreadIndicator.classList.contains('-top-1.5')).toBe(true);
    expect(unreadIndicator.classList.contains('h-2.5')).toBe(true);
    expect(unreadIndicator.classList.contains('w-2.5')).toBe(true);
    view.rerender(
      <Sidebar
        activeTab="dashboard"
        collapsed={false}
        onToggleCollapsed={collapseHandler}
        onNavigate={navigationHandler}
        onLogout={logoutHandler}
        buildInfo={buildInfo}
        hasUnreadChatMessage={false}
      />,
    );
    expect(screen.queryByLabelText('在线聊天有未读消息')).toBeNull();
  });

  test('知识库下线后不再出现在业务导航', /* dormantKnowledgeNavigationTest 验证侧边栏不再暴露暂时停用的入口。 */ () => {
    // navigationHandler 记录侧边栏其他入口的导航行为，本用例不应收到知识库标识。
    const navigationHandler = vi.fn();
    // logoutHandler 是知识库导航测试中未使用的注销回调。
    const logoutHandler = vi.fn();
    // collapseHandler 是知识库导航测试中未使用的折叠回调。
    const collapseHandler = vi.fn();
    render(
      <Sidebar
        activeTab="dashboard"
        collapsed={false}
        onToggleCollapsed={collapseHandler}
        onNavigate={navigationHandler}
        onLogout={logoutHandler}
        buildInfo={{ version: 'dev', commit: 'test' }}
      />,
    );
    expect(screen.queryByRole('button', { name: '知识库' })).toBeNull();
    expect(navigationHandler).not.toHaveBeenCalledWith('knowledge');
  });

  test('top navigation removes development version badge and keeps a centered desktop grid', /* topNavigationBalanceTest 验证方案三导航不再被版本占位挤偏。 */ () => {
    render(<Sidebar activeTab="dashboard" collapsed={false} onToggleCollapsed={/* collapseAction 本用例不切换导航。 */ () => undefined} onNavigate={/* navigationAction 本用例不跳转。 */ () => undefined} onLogout={/* logoutAction 本用例不退出。 */ () => undefined} buildInfo={{ version: 'dev', commit: 'unknown' }} />);
    expect(screen.queryByText('dev')).toBeNull();
    // topNavigation 是固定品牌、中间导航和右侧动作组成的三段式容器。
    const topNavigation = screen.getByTestId('top-navigation').firstElementChild;
    expect(topNavigation?.className).toContain('lg:grid-cols-[220px_auto_minmax(24px,1fr)_72px]');
    expect(screen.getByRole('navigation', { name: '主导航' }).className).toContain('lg:ml-[120px]');
    expect(screen.getByText('退出')).toBeTruthy();
  });
});
