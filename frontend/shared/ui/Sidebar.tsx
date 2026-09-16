import React from 'react';
import {
  Bell, Box, CreditCard, LayoutDashboard, LogOut, Menu, MessageCircleMore,
  Settings, ShoppingBag, Users, X, Zap,
} from 'lucide-react';
import { CoverAIBrandIcon } from './CoverAILogo';

// SidebarBuildInfo 描述应用壳传递给侧边栏的公开构建版本信息。
export interface SidebarBuildInfo {
  // version 是当前服务构建版本，只用于界面展示。
  version: string;
  // commit 是当前服务构建提交标识，只用于界面展示。
  commit: string;
}

// SidebarProps 描述侧边栏所需的导航和会话回调。
interface SidebarProps {
  /** activeTab 表示当前Tab。 */ activeTab: string;
  /** isAdmin 表示当前用户是否为管理员。 */ isAdmin?: boolean;
  /** collapsed 表示侧边栏是否折叠。 */ collapsed: boolean;
  /** onToggleCollapsed 表示切换侧边栏折叠状态的回调。 */ onToggleCollapsed: () => void;
  /** onNavigate 表示切换主导航页面的回调。 */ onNavigate: (tab: string) => void;
  /** onLogout 表示注销当前会话的回调。 */ onLogout: () => void;
  /** buildInfo 是由应用壳加载并传入的公开构建版本信息。 */ buildInfo: SidebarBuildInfo;
  /** hasUnreadChatMessage 表示在线聊天入口是否仍有未读消息，需要展示红点。 */ hasUnreadChatMessage?: boolean;
}

// Sidebar 保留历史组件名以兼容应用壳导入，实际渲染方案三的紧凑顶部导航。
const Sidebar: React.FC<SidebarProps> = ({
  activeTab, isAdmin = false, collapsed, onToggleCollapsed, onNavigate, onLogout, hasUnreadChatMessage = false,
}) => {
  // menuItems 侧边栏菜单项。
  const menuItems = [
    { id: 'dashboard', icon: LayoutDashboard, label: '仪表盘' },
    { id: 'accounts', icon: Users, label: '账号管理' },
    { id: 'chat', icon: MessageCircleMore, label: '在线聊天' },
    { id: 'cards', icon: CreditCard, label: '卡密库存' },
    { id: 'items', icon: Box, label: '商品列表' },
    { id: 'orders', icon: ShoppingBag, label: '订单管理' },
    { id: 'rules', icon: Zap, label: '自动化规则' },
    { id: 'notifications', icon: Bell, label: '通知设置' },
    ...(isAdmin ? [{ id: 'settings', icon: Settings, label: '系统与AI' }] : []),
  ];
  return (
    <header className="fixed inset-x-0 top-0 z-30 border-b border-slate-200 bg-white/95 backdrop-blur-xl" data-testid="top-navigation">
      <div className="mx-auto grid h-16 max-w-[1680px] grid-cols-[minmax(0,1fr)_auto] items-center px-4 lg:grid-cols-[220px_auto_minmax(24px,1fr)_72px] lg:px-6">
        <button type="button" className="flex min-w-0 shrink-0 items-center gap-2.5 lg:w-[220px]" onClick={/* brandHomeAction 返回经营概览。 */ () => onNavigate('dashboard')} aria-label="返回仪表盘">
          <CoverAIBrandIcon sizeClass="h-9 w-9" />
          <span className="hidden whitespace-nowrap text-base font-black tracking-tight text-slate-950 sm:inline">CoverAI 闲鱼助手</span>
        </button>

        <button type="button" onClick={onToggleCollapsed} className="flex h-9 w-9 items-center justify-center justify-self-end rounded-md text-slate-500 hover:bg-slate-100 lg:hidden" aria-label={collapsed ? '展开主导航' : '收起主导航'}>
          {collapsed ? <Menu className="h-5 w-5" /> : <X className="h-5 w-5" />}
        </button>

        <nav className={`${collapsed ? 'hidden' : 'flex'} absolute inset-x-0 top-16 max-h-[calc(100vh-4rem)] flex-col overflow-y-auto border-b border-slate-200 bg-white p-3 shadow-lg lg:static lg:ml-[120px] lg:flex lg:min-w-0 lg:flex-row lg:items-stretch lg:justify-start lg:overflow-x-auto lg:border-0 lg:bg-transparent lg:p-0 lg:shadow-none`} aria-label="主导航">
          {menuItems.map(/* navigationItemRenderer 渲染保持全部业务入口的紧凑导航项。 */ item => {
            // Icon 是当前导航项使用的既有线性图标组件。
            const Icon = item.icon;
            // active 表示当前路由是否与导航项一致。
            const active = activeTab === item.id;
            return <button key={item.id} type="button" aria-label={item.label} aria-current={active ? 'page' : undefined} onClick={/* navigationAction 切换业务页面。 */ () => onNavigate(item.id)} className={`group relative flex h-11 shrink-0 items-center gap-1.5 px-2.5 text-[13px] font-semibold transition-colors lg:h-16 xl:px-3 ${active ? 'text-blue-600' : 'text-slate-600 hover:text-slate-950'}`}>
              <span className="relative flex h-4 w-4 shrink-0">
                <Icon className={`h-4 w-4 ${active ? 'text-blue-600' : 'text-slate-400 group-hover:text-slate-700'}`} />
                {item.id === 'chat' && hasUnreadChatMessage && <span className="absolute -right-1.5 -top-1.5 h-2.5 w-2.5 rounded-full border-2 border-white bg-red-500" role="status" aria-label="在线聊天有未读消息" />}
              </span>
              <span className="whitespace-nowrap">{item.label}</span>
              {active && <span className="absolute bottom-0 left-2 right-2 h-0.5 rounded-full bg-blue-600" />}
            </button>;
          })}
        </nav>

        <div className="hidden h-16 w-[72px] items-center justify-end lg:col-start-4 lg:flex">
          <button type="button" onClick={onLogout} aria-label="退出登录" title="退出登录" className="flex h-9 items-center gap-1.5 rounded-md px-2 text-xs font-semibold text-slate-400 hover:bg-red-50 hover:text-red-600"><LogOut className="h-4 w-4" /><span>退出</span></button>
        </div>
      </div>
    </header>
  );
};

export default Sidebar;
