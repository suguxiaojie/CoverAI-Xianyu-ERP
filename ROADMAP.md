# CoverAI 闲鱼助手 Roadmap

## 文档职责

本文件是公开仓库的动态状态摘要。六阶段架构计划的权威状态仍以 `docs/architecture/refactoring-master-plan.md` 为准，验收记录见 `docs/architecture/refactoring-progress.md`。

## 当前阶段

**阶段：开源快照与持续维护。**

六个正式架构重构阶段已完成，全部既有架构门禁持续启用。当前快照包含多账号、聊天、订单、商品、卡密、自动化、经营分析、通知、桌面打包与三数据库方言实现。

验收记录同步至：`2026-09-16 开源质量门禁收口`。

## 已完成

- 六阶段架构重构和对应的 fail-closed 门禁。
- Go／React 管理后台、SQLite／MySQL／PostgreSQL 迁移和 Playwright 浏览器运行时。
- Windows、macOS、Linux 打包脚本与 Docker 多架构工作流。
- 首次开源快照的 README、许可证核对、历史隔离与业务标识脱敏。
- 公开 CI 的 Go、React、race 和 SQLite／MySQL／PostgreSQL 三方言门禁。

## 进行中

- 持续收紧多账号运行、平台协议兼容、订单与自动化恢复路径。
- 持续增加确定性单元测试、race 回归和三方言数据库验证。
- 保持生成的前端静态资源与 React 源码同步。

## 阻塞与风险

- 闲鱼非公开接口和风控策略可能随时变化，不能保证长期兼容。
- 真实发送、发货、改价、关单和退款验收会修改外部平台状态，必须使用明确授权的测试对象。
- Windows 和 macOS 可分发安装包依赖维护者的代码签名凭据与发布环境审批。

## 待办与下一步

1. 优先处理可稳定复现的协议兼容和数据一致性问题。
2. 对新增业务分支补充成功、失败、取消、超时、重试与晚到结果测试。
3. 保持 `make architecture`、`make comments`、`make roadmap`、Go 和前端测试为提交前必要检查。
4. 涉及真实账号和外部平台写入时，先明确对象、影响、幂等性与回滚边界。

## 最近验证

- 2026-09-16 14:14：公开 Actions 运行 `35062582537` 通过 Go 全门禁、server race、前端 `574/574` 用例和 SQLite／MySQL 8.4／PostgreSQL 17 三方言实测。
- 2026-09-16 14:10：MySQL Schema 50 复合主键宽度已修复，新库迁移在 MySQL 8.4 实测通过。
- 2026-09-16 14:02：`golangci-lint v2.12.2` 为 `0 issues`，嵌入前端已按 `npm ci` 锁文件重建并通过 CI 一致性检查。
- 2026-09-16 13:43：前端 `90/90` 个测试文件、`574/574` 个用例通过，TypeScript 和 Vite `2403` 模块生产构建通过。
- 2026-09-16 13:42：`go test ./... -count=1`、`go vet ./...`、`go build ./cmd/server`通过。
- 2026-09-16 13:40：`make architecture`、Go／前端中文注释检查、`make roadmap` 和 `git diff --check`通过。
- Docker CLI 在当前本机不可用，因此未执行 `docker compose config` 和容器启动验证。

## 状态更新规则

- 只有已实现并完成必要验证的事项才能标记完成。
- 无法在当前环境执行的验证必须明确记录，不能隐藏或以历史结果代替。
- 文档不记录真实账号、订单、会话、Cookie、Token、密钥、本机路径或部署运行数据。
