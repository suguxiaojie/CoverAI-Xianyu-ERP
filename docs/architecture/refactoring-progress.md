# 架构重构与维护验收记录

本公开文件只保留可复现的工程结论。包含真实账号、订单、会话、本机路径、部署 PID、备份位置和签名环境的私有运维记录不进入开源仓库。

## 阶段结论

- 阶段 1：稳定性、敏感数据与启动生命周期基线已完成。
- 阶段 2：`internal/composition` 已成为唯一生产组合根，Server 只依赖消费者 Port。
- 阶段 3：后台任务的 owner、Context、Cancel 和 Wait／Join 路径已收口。
- 阶段 4：React 已按 `app -> features -> shared` 分层，异步请求具备取消或代次隔离。
- 阶段 5：上层裸 SQL／事务入口已清理，SQLite、MySQL 和 PostgreSQL 迁移保持对齐。
- 阶段 6：架构、兼容、复杂度、注释和前端依赖方向门禁已永久启用。

### 2026-09-16 首次脱敏开源快照

- 开源快照从已验证的当前树生成，不包含私有仓库历史。
- README 重构为公开项目入口，并根据实际 `LICENSE` 纠正为 Apache-2.0。
- 公开 Roadmap 和本记录已去除真实业务标识、本机路径和部署运行证据；测试中的账号夹具也替换为明确的合成值。
- 验证通过：`go test ./... -count=1`、`go vet ./...`、`go build ./cmd/server`、`make architecture`、Go／前端中文注释门禁、TypeScript、前端 `90/90` 文件 `574/574` 用例与 Vite `2403` 模块生产构建。
- Docker CLI 在当前本机不可用，未执行 Compose 解析与容器启动验证。

### 2026-09-16 开源质量门禁收口

- 首次公开 CI 发现并修复 9 个既有 golangci-lint 问题；修复保持外部行为，本地与 GitHub Actions 的 `golangci-lint v2.12.2` 均为 `0 issues`。
- 前端嵌入资源改为按 CI 使用的 `package-lock.json` 和 `npm ci` 重建；日期选择器测试显式固定时钟，避免随真实月份漂移。
- MySQL Schema 50 中三个参与 `utf8mb4` 复合主键的业务标识列收紧为 `VARCHAR(191)`，使最坏索引宽度低于 InnoDB 3072 字节限制，并增加静态迁移回归。
- 公开提交 `7b541da` 的质量门禁运行 `35062582537` 全部成功：Go 格式、注释、架构、vet、lint、全仓测试、server race，前端安装、注释、类型、`574/574` 用例、嵌入产物一致性，以及 SQLite／MySQL 8.4／PostgreSQL 17 三方言实测均通过。
- 桌面工作流仍需维护者配置 Windows／macOS 签名凭据；未将正式包降级为未签名产物。
