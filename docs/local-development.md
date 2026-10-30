# 本地开发与工程说明

## 本地启动

需要 Node.js ≥22.23.2、pnpm 10.14.0、Go 1.27.3 和可运行 Compose 的 Docker。Go 可安装在系统；`scripts/go.sh` 会优先使用项目内 `.tools/go`，不会修改系统 Go。精确版本和官方链接见 [版本记录](versions.md)。

```sh
cp .env.example .env
pnpm install --frozen-lockfile
make tools
make db-up
make migrate
# 在终端安全输入一个 12–128 字符的初始密码（不使用预设默认密码）
read -s BOOTSTRAP_PASSWORD; export BOOTSTRAP_PASSWORD
BOOTSTRAP_USERNAME=admin make bootstrap
unset BOOTSTRAP_PASSWORD
make dev-api
# 另一个终端
make dev
```

访问 http://127.0.0.1:5173。API 为 http://127.0.0.1:8080/api/health。Vite 将同源 /api 代理给 Go；不需要开放 CORS。

访问 `/login` 登录。后台提供概览、账号、企业/公司组织、树状部门、角色和权限管理，可新增、编辑、停用和删除（有引用的结构禁止直接删除）。账号支持多角色、独立允许/拒绝与六类数据范围。界面默认暗色，支持浅色切换、中英文及刷新保持；概览统计来自授权范围内的实际数据。

技术验收示例保留在 `/foundation`，架构说明在 `/architecture`。登录与后台的生成 SDK、Query、表单、表格均沿用项目框架。

数据库并非运行公共 health/echo 的必要条件：不设置 DATABASE_URL 时 API 仍可启动；受保护的持久化接口不会假装成功。health 是进程存活检查，不表示数据库 readiness。

## 生成和验证

测试按批次执行：普通连续开发累计待验项，单个功能完成或聊天结束不自动跑测试；高风险改动先定向验证，明确阶段验收/提交/PR 前统一跑快速门禁和相关专项，发布候选、明确全量验收或 CI 才跑完整回归。已通过且输入未变的检查复用。详见 [测试执行节奏](testing.md#测试执行节奏)。

```sh
make generate           # Goa → OpenAPI；sqlc；Orval → TanStack Query SDK
make check-generated    # 生成前后文件摘要比对；不依赖 Git 已提交状态
make format
make format-check
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm i18n:check
make lint               # 前端 ESLint + Go vet
make test               # 前端 + Go 单元/HTTP + 翻译 key
make build              # 前端 + Go 构建
make test-integration   # 独立 PostgreSQL schema，缺少测试 URL 则失败
pnpm --filter @baseline/web exec playwright install chromium
make test-e2e           # 启动真实前后端；独立端口 5179/8099 必须空闲
make check-rules       # 项目规则、Skills、引用与门禁路由
make ci-fast           # 无需 DB/浏览器的完整快速门禁
make test-security     # 认证/授权/配置 + 真实数据库范围负向用例
make ci                # 所有门禁，含 DB 和 E2E
```

Go 命令可通过 `scripts/go.sh fmt ./...`、`scripts/go.sh vet ./...`、`scripts/go.sh test ./...` 执行；它们的工作目录为 services/api，工具链与缓存隔离在 .tools。使用系统 Go 时也可在 services/api 直接执行 gofmt / go vet / go test。

`.env` 由 Make 读取并导出；直接使用 pnpm 或脚本不会自动加载它。CI 为测试显式配置环境变量。`make db-down` 只停止容器；开发数据卷不会被删除。

## 账号与权限

初始账号由 `make bootstrap` 创建，不提供通用默认密码；已存在工作区不会被重复初始化覆盖。BOOTSTRAP_TENANT 可显式指定工作区，默认为 workspace-default。工作区内为“企业 → 公司 → 部门树 → 账号”，角色可重复分配。

活跃角色权限合并；账号独立允许替换同一操作的继承范围，独立拒绝优先。范围包括工作区全部、本人公司、本人部门、部门及下级、本人账号、自定义部门。操作授权由 Casbin 决策，SQL 强制数据范围与工作区隔离；页面隐藏不能替代后端校验。详见 [身份与权限决策](decisions/002-identity-access.md)。

会话为 8 小时 HttpOnly cookie，写请求校验 Origin + CSRF。密码使用 Argon2id，停用账号、重置密码会撤销会话；角色变更在下一个请求生效。公网部署要求 HTTPS 和 COOKIE_SECURE=true。

## 独立技术权限示例

仅为本地验证，可在 `.env` 设置任意非空 `DEMO_AUTH_TOKEN` 后启动 API。身份固定由后端解析为 `demo-user / demo-tenant`，不能通过请求头选择租户。此模式限制为数字形式的 loopback 监听地址。默认 token 为空，受保护接口全部拒绝。

```sh
# TOKEN 为自己本地配置的演示凭证，不要提交它
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/api/demo/records
```

演示表初始为空，集成测试在独立 schema 中创建并清理测试数据。GET /records、GET /records/{id}、PATCH /records/{id}、DELETE /records/{id} 均应用操作授权及可信 SQL 数据范围。字段 internal_note 从不进入响应类型。该技术示例与正式账号后台隔离，不为其开启浏览器凭证。导出和批量操作尚未提供，后续新增时必须同等保护。

## 工程文档

- [架构与模块边界](architecture.md)
- [编码约定](coding-conventions.md)
- [API 契约流程](api-contracts.md)
- [数据库约定](database-conventions.md)
- [设计系统](design-system.md)
- [国际化](internationalization.md)
- [Agent 工作流](agent-workflow.md)
- [任务与工具矩阵](agent-tooling.md)
- [测试和质量门禁](testing.md)
- [基础安全规则](security.md)
- [基础框架决策](decisions/001-foundation.md)
- [身份与权限决策](decisions/002-identity-access.md)
- [两级目录与临时权限](decisions/003-temporary-access.md)
- [验收记录](verification.md)

开发功能前从 [AGENTS.md](../AGENTS.md) 和 [任务工具矩阵](agent-tooling.md) 进入：按目标路径读取子目录规则，按任务加载 Skill，并执行相应门禁。

本机已安装 Chrome 时，可使用 `PLAYWRIGHT_CHANNEL=chrome make test-e2e`；否则按上文安装配套 Chromium。两者运行同一套功能断言和诊断采集。

临时权限入口为 `/temporary-access`，所有已登录账号可申请缺失功能或提升数据范围，选择 30 分钟至 7 天的时效；独立 `temporary:approve` 权限控制审批工作台。审批人可修改最终权限/时效/范围，服务端自动处理到期，支持撤回和撤销。默认仅内置管理员可审批；目录和安全边界详见 ADR 003。

系统设置、字典、附件和消息中心的产品与操作约定见 [正式方案](system-settings-and-dictionaries-plan.md) 和 [ADR 005](decisions/005-system-governance.md)。启用秘密配置前提供 SETTINGS_ENCRYPTION_KEY；附件目录受 FILE_STORAGE_ROOT 限制，SMTP 主机须加入 SMTP_ALLOWED_HOSTS。真实邮件凭证在设置页面输入并重新认证发布，不写入环境示例或仓库。数据库、存储目录及加密根密钥须配套备份；切换存储位置不搬迁旧文件。

新业务接入配置、字典、附件或通知前使用 [baseline-shared-capabilities](../.agents/skills/baseline-shared-capabilities/SKILL.md)，调用约定见 [公共能力接入](shared-capabilities.md)。设置与字典按最新 [ADR 007](decisions/007-global-governance.md) 统一管理，不关联组织配置范围。

## WebMCP 与 WebSkills

在仓库根运行 `pnpm webmcp`（默认 `127.0.0.1:18793`），登录应用进入侧栏「WebMCP 与 WebSkills」，填写终端中的配对密钥并连接。端口由 `WEBMCP_PORT` 配置，页面端口须一致。点击 Codex 或 WorkBuddy 的「一键设置 MCP + Skills」安装当前页面连接和三套技能，重启客户端后使用；Codex CLI 需在 PATH 或通过 `CODEX_BIN` 指定绝对路径。其他客户端配置 Node + `scripts/webmcp-stdio.mjs` 的绝对路径 stdio 启动即可。

连接跨应用路由保留，刷新/断开/退出/切部门后清理；重新连接或选择另一个标签页后再次设置客户端。安装在运行本地服务的电脑上，先备份并合并已有配置；密钥只存忽略提交的 `.local/webmcp/connection.json`，请勿分享。可手动执行 `pnpm webmcp:setup workbuddy` / `pnpm webmcp:setup codex` 安装配置（仍须先在设置页连接并绑定页面）。工具能力、安全边界与未来助手预留见 [ADR 009](decisions/009-webmcp.md)。
