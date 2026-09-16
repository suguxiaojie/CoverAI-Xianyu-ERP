# CoverAI 闲鱼助手

<p align="center">
  <img src="docs/assets/coverai-logo.png" alt="CoverAI 闲鱼助手" width="104" height="104">
</p>

<p align="center">
  基于 Go、React 和 Playwright 的自托管闲鱼多账号运营、客服与数字商品交付系统。
</p>

<p align="center">
  <a href="https://github.com/suguxiaojie/CoverAI-Xianyu-ERP/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/suguxiaojie/CoverAI-Xianyu-ERP/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/suguxiaojie/CoverAI-Xianyu-ERP/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26.4-00ADD8?logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=111827">
</p>

> [!IMPORTANT]
> 本项目不是闲鱼或阿里巴巴官方项目。它会连接真实账号，并可执行发消息、改价、关单、发货、求花、收花等会改变平台状态的操作。请只管理你有权操作的账号和交易，先在小范围测试环境验证，并自行遵守平台规则及当地法律法规。

## 为什么做这个项目

CoverAI 闲鱼助手将分散的多店铺运营流程收口到一个本地 Web 后台：账号连接、在线聊天、订单、商品、卡密库存、自动交付、运营统计和通知都在同一套系统内完成。

它尤其适合需要管理多规格数字商品、卡密／图片凭证交付和多账号客服的自托管场景。所有运行数据都由使用者自行保管，默认数据库为 SQLite，也可切换到 MySQL 或 PostgreSQL。

## 主要能力

| 模块 | 能力 |
| --- | --- |
| 账号管理 | 多账号启停、扫码登录、资料刷新、运行状态、人工风控接管和账号级自动任务。 |
| 在线聊天 | 会话搜索与置顶、未读数、图文消息、引用回复、撤回、交易卡片、消息提示音和同买家历史订单。 |
| 订单工具 | 增量同步、全量校准、单笔刷新、待付款改价与关单、发货凭证、退款详情与人工确认门禁。 |
| 商品与 SKU | 商品同步、多规格 SKU、本地成本、自动化关联、单商品发布与表格批量铺货。 |
| 卡密库存 | 文本、批量卡密和图片卡密；支持追加库存、规格匹配、延迟发送和不可逆消耗。 |
| 自动化 | 付款后交付、评价赠品、求花、收花、关键词回复、幂等投递、异常待办和失败恢复。 |
| 经营分析 | 按全部或单账号查看成交额、预估利润、成本覆盖率、趋势和商品排行。 |
| 通知与 AI | Bark、钉钉、飞书、企业微信、Telegram、邮件和 Webhook；支持 OpenAI 兼容模型配置。 |
| 桌面与容器 | Windows Service，macOS AppKit／WebKit 原生壳，Linux systemd 安装包和 Docker 部署。 |

## 快速开始

### 环境要求

- Go `1.26.4`。
- Node.js `24` 和 npm。
- 执行账号登录、指纹读取或风控接管时，需要与 playwright-go driver 匹配的 Chromium runtime。
- 本地默认使用 SQLite，无需额外数据库服务。

### 1. 获取源码

```bash
git clone https://github.com/suguxiaojie/CoverAI-Xianyu-ERP.git
cd CoverAI-Xianyu-ERP
```

### 2. 构建前端与服务

```bash
npm ci --prefix frontend
npm run build --prefix frontend
go build -o xianyu-server ./cmd/server
```

Vite 会将生产资源写入 `internal/webui/static/`，Go 构建再将它们嵌入服务二进制。前端变更后，必须重新构建前端和 Go 服务。

### 3. 准备 Chromium

```bash
go run ./cmd/browser-install
```

不要手工混用不同版本的 Playwright driver、Chromium 和 headless shell。

### 4. 启动服务

```bash
mkdir -p data
./xianyu-server \
  -workdir "$PWD/data" \
  -db xianyu_data.db \
  -addr 127.0.0.1:59188
```

打开 `http://127.0.0.1:59188`。新数据库会显示首次初始化页；输入并确认管理员密码后，系统创建 `admin` 并自动登录。

> [!WARNING]
> 本机自用应优先绑定 `127.0.0.1:59188`。`-addr :59188` 会监听所有网络接口；如需远程访问，请先配置防火墙、反向代理、HTTPS 和可信来源限制。

### 无浏览器模式

只有在明确不需要 Chromium 功能时才应启用：

```bash
./xianyu-server -workdir "$PWD/data" -db xianyu_data.db -addr 127.0.0.1:59188 -no-browser
```

该模式会禁用浏览器指纹读取、Token 风控接管和需要官方页面的自动化能力。

## Docker Compose

默认 Compose 方案使用 PostgreSQL 17 和 GHCR 镜像：

```bash
cp .env.example .env
# 编辑 .env，替换数据库密码、管理员密码和 XIANYU_DATA_KEY
docker compose up -d
docker compose ps
```

`.env` 中的 `POSTGRES_PASSWORD` 与 `DATABASE_URL` 必须一致；密码包含 `@`、`:`、`/` 或 `#` 时，需在 URL 中编码。请固定并离线备份 `XIANYU_DATA_KEY`，丢失后已加密的账号凭据无法恢复。

## 数据库

系统支持 SQLite、MySQL 和 PostgreSQL，连接配置优先级为：

```text
DATABASE_URL > -db-url > -db
```

PostgreSQL 示例：

```bash
DATABASE_URL="postgres://user:password@127.0.0.1:5432/xianyu?sslmode=disable" \
./xianyu-server -addr 127.0.0.1:59188
```

三种数据库方言使用同一组 Goose 迁移号。数据库行为变更至少应执行 SQLite 定向测试；提交关联修改前，建议同时运行 MySQL 与 PostgreSQL 回归。

## 配置要点

| 参数／环境变量 | 用途 |
| --- | --- |
| `-addr` | HTTP 监听地址；本机推荐 `127.0.0.1:59188`。 |
| `-workdir` | 数据、上传目录、浏览器资料和 `data-key` 的运行根目录。 |
| `-db` | SQLite 文件路径。 |
| `-db-url` / `DATABASE_URL` | SQLite、MySQL 或 PostgreSQL 连接 URL。 |
| `-data-key-file` / `XIANYU_DATA_KEY` | Cookie、Token 等敏感字段的加密主密钥。 |
| `-playwright-runtime-root` | 安装包内 Playwright runtime 根目录。 |
| `XIANYU_ADMIN_PASSWORD` | 容器或无交互环境的首次管理员初始化。 |
| `LOG_LEVEL` / `-log-level` | `debug`、`info`、`warn` 或 `error`。 |
| `LOG_FORMAT` / `-log-format` | `text` 或 `json`。 |
| `-no-browser` | 显式禁用 Chromium；会损失相关登录和验证能力。 |

## 架构

```mermaid
flowchart LR
    UI["React / Vite 管理后台"] --> HTTP["Go / chi HTTP API"]
    HTTP --> App["Application Services"]
    App --> DB["SQLite / MySQL / PostgreSQL"]
    App --> Runtime["多账号运行时"]
    Runtime --> WS["闲鱼 WebSocket"]
    Runtime --> MTOP["闲鱼 MTOP"]
    Runtime --> Automation["自动化中心"]
    Runtime --> Chromium["Playwright / Chromium"]
    Automation --> Notify["通知渠道"]
```

| 目录 | 职责 |
| --- | --- |
| `cmd/server` | 配置、信号、进程入口和生命周期。 |
| `internal/composition` | 生产组合根，组装应用服务、账号运行时和基础设施。 |
| `internal/application` | 用例编排、所有权、事务边界和补偿。 |
| `internal/adapter` | 将数据库、平台、账号运行时等实现适配到应用 Port。 |
| `internal/engine` | 单账号消息运行时、回复、投递和实时事件。 |
| `internal/automation` | 交付、评价、求花、收花、幂等与恢复调度。 |
| `internal/xianyu` | MTOP、WebSocket、扫码登录和协议解析。 |
| `internal/browser` | 必须依赖浏览器的验证与官方页面操作。 |
| `internal/db` | SQL、多方言迁移、repository 和敏感字段加密存储。 |
| `internal/server` | HTTP／WebSocket transport、管理员鉴权和 SPA 托管。 |
| `frontend` | React 19、Vite 与 feature 化管理后台。 |

详细的工程边界、重构状态与验收入口见 [`AGENTS.md`](AGENTS.md)、[`ROADMAP.md`](ROADMAP.md) 和 [`docs/architecture/`](docs/architecture/)。

## 开发与验证

```bash
make build              # 构建 Go 服务
make frontend           # 安装依赖并重建嵌入式前端
make test               # Go 全量测试
make test-server-race   # Server 生命周期与凭据并发回归
make vet                # go vet
make lint               # golangci-lint
make architecture       # 架构边界门禁
make comments           # Go 和 TypeScript／TSX 中文注释门禁
make roadmap            # Roadmap 与详细验收记录同步门禁
make cover              # Go 覆盖率
make cover-browser      # Chromium 集成覆盖率
make cover-frontend     # React / Vite V8 覆盖率
```

```bash
npm test --prefix frontend
npm run typecheck --prefix frontend
npm run comments:check --prefix frontend
npm run build --prefix frontend
```

提交前请至少运行与改动匹配的定向测试、`make architecture`、`make comments` 和 `git diff --check`。新增或修改确定性业务分支时，必须补充回归测试。

## 数据与安全

- 永远不要提交 `.env`、Cookie、Token、密码、`data-key`、真实数据库、浏览器 Profile、上传文件或备份。
- SQLite 部署的最小完整备份包含数据库、对应的 `data-key`、当前二进制、启动参数和托管配置。
- 不要在日志、通知、API 错误或测试失败输出中打印明文凭据。
- 不要在多个实例中同时运行同一闲鱼账号。
- 发货、改价、关单、退款、求花和发送消息等动作应保留精确对象、所有权、终态、幂等和人工确认门禁。
- 人工滑块验证的实现已冻结；未经明确需求，不应改动选择器、轨迹、距离、成功判定或 Playwright／CDP 回退顺序。

## 项目状态

- 六个正式架构重构阶段均已完成，现有架构门禁持续启用。
- 当前源码包含多账号、聊天、订单、商品、卡密、自动化、经营分析、通知和桌面打包的完整实现。
- 知识库应用与数据层代码已保留，但当前管理后台没有注册对外知识库路由。
- 没有真实账号、平台状态或外部服务的环境，不能视为已完成端到端交易验收。

更细的当前状态和已知边界见 [`ROADMAP.md`](ROADMAP.md)。

## 贡献

欢迎通过 [Issues](https://github.com/suguxiaojie/CoverAI-Xianyu-ERP/issues) 报告可重现问题或提交功能建议。贡献代码时请：

1. 不要提交真实账号、Cookie、Token、订单、卡密、密钥、日志或其他敏感数据。
2. 将无关主题拆成独立提交，并为协议、数据库和关键业务分支增加定向测试。
3. 遵守 [`AGENTS.md`](AGENTS.md) 中的架构、中文注释、敏感数据、生命周期和冻结滑块约束。
4. 提交前运行相关测试和静态检查，并在 PR 中说明未能执行的环境验证。

## 上游与许可证

本项目基于 [Christ9038/Ydisks-Xianyu-Helper](https://github.com/Christ9038/Ydisks-Xianyu-Helper) 持续开发，并在其基础上扩展了 CoverAI 的多规格交付、聊天交易工具、经营分析、桌面体验和工程治理。

代码按 [Apache License 2.0](LICENSE) 授权。分发原作或衍生作品时，请保留 [`NOTICE`](NOTICE) 和适用的许可声明，并清晰标记修改内容。

## 免责声明

本项目只提供技术研究和自托管管理能力，不承诺对闲鱼未来接口变化、账号风控决策、业务连续性或数据完整性的兼容。使用者对法律合规、平台合规、账号授权、备份、账号限制、交易纠纷、数据丢失以及其他使用后果自行负责。严禁将本项目用于骚扰、欺诈、未授权访问、规避平台安全控制或其他违法活动。
