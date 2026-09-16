import React from 'react';

interface CoverAILogoProps {
  /** className 表示附加的 CSS 类名。 */ className?: string;
}

interface CoverAIBrandIconProps {
  /** sizeClass 指定品牌图标尺寸相关的 Tailwind 类名，而非业务数量。 */ sizeClass?: string;
  /** logoClassName 表示品牌图标的 CSS 类名。 */ logoClassName?: string;
}

// CoverAILogo 渲染工作区既有的 CoverAI 官方透明图标，不再复用旧产品矢量路径。
const CoverAILogo: React.FC<CoverAILogoProps> = ({ className = 'h-full w-full object-contain' }) => (
  <img src="/static/coverai-logo.png" alt="" className={className} aria-hidden="true" />
);

// CoverAIBrandIcon 渲染品牌图标。
export const CoverAIBrandIcon: React.FC<CoverAIBrandIconProps> = ({
  sizeClass = 'w-12 h-12',
  logoClassName = 'h-full w-full object-contain',
}) => (
  <div className="relative z-10 flex items-center justify-center">
    <div className="absolute -inset-1 rounded-xl bg-cyan-400/20 opacity-25 blur" />
    <div className={`relative flex ${sizeClass} items-center justify-center overflow-hidden rounded-[28%] transition-transform duration-300 group-hover:scale-105`}>
      <CoverAILogo className={logoClassName} />
    </div>
  </div>
);

export default CoverAILogo;
