// @vitest-environment jsdom
import { fireEvent,render,screen } from '@testing-library/react';
import { expect,test } from 'vitest';
import { ChatCreditBadges } from './ChatCreditBadges';

// creditProfileFixture 是同时包含五级买家和四级卖家信用的当前会话响应。
const creditProfileFixture = {
	user_id: 'buyer-1',
	buyer: { role: 'buyer' as const, level: 5, code: 'cs_buyer_level', text: '买家信用极好' },
	seller: { role: 'seller' as const, level: 4, code: 'cs_seller_level', text: '卖家信用优秀' },
	fetched_at: '2026-08-27T02:30:00Z', expires_at: '2026-08-28T02:30:00Z', stale: false,
};

test('当前会话信用按等级着色并点击查看双角色详情', /* creditBadgeColorCase 验证文字和颜色共同表达等级。 */ () => {
	render(<ChatCreditBadges profile={creditProfileFixture} loading={false} error="" />);
	// buyerButton 是买家五级绿色标签。
	const buyerButton = screen.getByRole('button', { name: '查看买家信用：极好' });
	// sellerButton 是卖家四级蓝色标签。
	const sellerButton = screen.getByRole('button', { name: '查看卖家信用：优秀' });
	expect(buyerButton.className).toContain('emerald');
	expect(sellerButton.className).toContain('blue');
	fireEvent.click(buyerButton);
	expect(screen.getByRole('dialog', { name: '信用信息' })).toBeTruthy();
	expect(screen.getAllByText('极好').length).toBeGreaterThan(0);
	expect(screen.getAllByText('优秀').length).toBeGreaterThan(0);
});

test('加载和冷却状态使用中性灰而不是信用较差红色', /* creditNeutralStateCase 防止平台失败被误解为低信用。 */ () => {
	// rerender 用于从加载状态切换到平台熔断状态。
	const { rerender } = render(<ChatCreditBadges loading profile={undefined} error="" />);
	expect(screen.getByLabelText('信用获取中').className).toContain('slate');
	rerender(<ChatCreditBadges loading={false} profile={undefined} error="信用信息暂缓更新" />);
	// coolingBadge 是不带可点击详情的中性熔断提示。
	const coolingBadge = screen.getByText('信用暂缓更新');
	expect(coolingBadge.className).toContain('slate');
	expect(coolingBadge.className).not.toContain('red-');
});
