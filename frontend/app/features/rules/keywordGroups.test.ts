import { expect,test } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parseKeywordBatch } from './pages/Rules';

test('关键词批量粘贴按中英文分隔符解析为去重气泡', /* 当前回调验证逗号、顿号、分号和换行可一次生成多个关键词。 */ () => {
  expect(parseKeywordBatch('要密码, 要账号、登陆账号\n提供账号；要密码')).toEqual(['要密码', '要账号', '登陆账号', '提供账号']);
  expect(parseKeywordBatch(' Price，price\nPRICE ')).toEqual(['Price']);
});

test('系统事件可无关键词且关键词反馈使用应用内组件', /* 当前回调验证系统事件直触发和浏览器 alert 退场文案。 */ () => {
  // actionsSource 是关键词保存协调器源文件。
  const actionsSource = readFileSync(resolve(__dirname, 'ruleActions.ts'), 'utf8');
  // pageSource 是关键词规则页面源文件。
  const pageSource = readFileSync(resolve(__dirname, 'pages/Rules.tsx'), 'utf8');
  expect(actionsSource).toContain('systemOnly');
  expect(actionsSource).toContain('关键词回复规则保存成功');
  expect(actionsSource).not.toContain("alert('请填写关键词和回复内容')");
  expect(pageSource).toContain('仅按系统事件触发');
  expect(pageSource).toContain('全局关键词规则');
  expect(pageSource).toContain('适用店铺');
  expect(pageSource).toContain('删除关键词回复规则？');
});

test('关键词规则提供单条和全部开关并覆盖成功失败气泡', /* 当前回调验证启停入口和用户反馈文案不会遗漏。 */ () => {
  // pageSource 是包含单条和全部关键词开关的页面源码。
  const pageSource = readFileSync(resolve(__dirname, 'pages/Rules.tsx'), 'utf8');
  expect(pageSource).toContain('全部关键词自动回复');
  expect(pageSource).toContain('handleToggleReplyRule');
  expect(pageSource).toContain('handleToggleAllReplyRules');
  expect(pageSource).toContain('该自动回复已开启');
  expect(pageSource).toContain('该自动回复已关闭');
  expect(pageSource).toContain('全部自动回复已开启');
  expect(pageSource).toContain('全部自动回复已关闭');
  expect(pageSource).toContain("kind: 'error'");
});
