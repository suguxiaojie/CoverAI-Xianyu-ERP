import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, test } from 'vitest';
import CoverAILogo, { CoverAIBrandIcon } from './CoverAILogo';

describe('CoverAILogo', /* 当前回调处理用户交互或异步状态变化。 */ () => {
  test('uses the retained CoverAI transparent logo asset', /* coverAIAssetTest 验证品牌组件不再包含旧产品矢量路径。 */ () => {
    // html 渲染后的 HTML。
    const html = renderToStaticMarkup(<CoverAILogo />);
    expect(html).toContain('src="/static/coverai-logo.png"');
    expect(html).not.toContain('M121.73,57.0003');
  });

  test('renders the CoverAI asset in the shared responsive brand wrapper', /* coverAIWrapperTest 验证登录页和侧边栏共享尺寸入口。 */ () => {
    // html 渲染后的 HTML。
    const html = renderToStaticMarkup(<CoverAIBrandIcon />);
    expect(html).toContain('src="/static/coverai-logo.png"');
    expect(html).toContain('rounded-[28%]');
    expect(html).toContain('w-12 h-12');
  });
});
