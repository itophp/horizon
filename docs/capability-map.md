# 能力地图

本文件回答两个问题：**这个项目由哪些目录和文件组成、各自负责什么**；以及**系统对外提供哪些业务能力、它们分别落在哪一层**。

它不是架构决策记录（见 [decisions](decisions/)），也不是任务路由表（见 [agent-tooling.md](agent-tooling.md)）。改动目录结构、生成边界或新增业务能力时同步更新本文件；纯内部重构不需要逐行维护文件清单之外的细节。

## 阅读方式

| 你想知道 | 直接看 |
|---|---|
| 某个目录/文件是干什么的 | §3 根目录、§6 服务端、§7 前端、§8 脚本、§9 文档 |
| 系统对外有哪些能力、风险与审批级别 | §10 能力清单 |
| 一次点击/一次 Agent 调用经过哪些代码 | §11 运行时链路 |
| 要改某个功能该动哪些文件 | §12 任务落点速查 |
| 哪些文件不能手改 | §13 生成边界与禁区 |
| 仓库里的构建产物与本地状态 | §14 产物与本地目录 |

规模参考（不含 `node_modules`、`dist`、`.local` 等产物）：手写 Go 115 个文件、生成 Go 85 个、手写前端 86 个文件、生成前端 66 个、SQL 24 个、Markdown 文档 36 个。

## 1. 一句话分层

```
契约层   services/api/design（Goa DSL）      → 唯一 API 源，生成 gen/ 与前端 SDK
数据层   services/api/db（migrations/query） → 唯一 schema 与 SQL 源，生成 dbgen/
运行时   services/api/internal/platform/capability（contracts → registry → gateway）
业务层   services/api/internal/{identity,modules/*,ai/*}  → 只在 registry 注册能力
传输层   services/api/internal/transport     → 装配路由、会话、限流、中间件
交付面   apps/web/src/features/*             → 按钮通过 capability SDK 调 Gateway
验证面   services/api/tests、apps/web/e2e、scripts/*.mjs → 门禁与真实链路验收
```

四条不变量贯穿全部文件：

1. 业务动作只在 `registry` 注册一次，运行时只经 `gateway` 执行。
2. 写能力用 `TxExecutor`，业务数据与 `cap_outbox` 同事务。
3. `design` 与 `db` 是生成源，`gen`、`dbgen`、`api/generated` 禁止手改。
4. 前端按钮、Agent Tool、Workflow、外部 API、审批恢复共用同一套身份/权限/范围判定。

## 2. 顶层目录树

```
xxh-baseline/
├── apps/web/                  React 控制台（Vite + TS + shadcn）
├── services/api/              Go 单体服务（Goa + sqlc + PostgreSQL）
├── .agents/skills/            工程侧 Skill：Agent 开发时要加载的方法
├── webskills/                 运行时侧 Skill：分发给 MCP 客户端的页面/业务使用说明
├── scripts/                   门禁、代码生成校验、WebMCP/远程 MCP 本机服务
├── docs/                      架构、安全、测试、决策记录与外部规范
├── .github/workflows/ci.yml   CI 质量门禁
├── Makefile                   全部可执行入口（开发、生成、测试、CI）
├── compose.yaml               本地 PostgreSQL 18.6（54329）
├── package.json               pnpm 工作区根脚本
├── skills-lock.json           上游 Skill 的版本与 hash 锁定
├── AGENTS.md                  项目长期开发规则（根）
└── README.md                  项目定位与架构方向说明
```

## 3. 根目录文件

| 文件 | 作用 |
|---|---|
| `AGENTS.md` | 根规则：能力契约、Gateway 唯一入口、事务与事件、策略确定性、生成禁区、验证与交付纪律。改文件前必须从根沿路径读到目标目录的规则。 |
| `README.md` | 对外介绍：Skills / MCP / WebMCP / AI Assistant 四件配套、架构方向与能力归属判据。 |
| `Makefile` | 唯一命令入口。开发、迁移、生成、检查、测试、CI 全在此定义（见 §4）。 |
| `compose.yaml` | 本地依赖：`postgres:18.6`，只绑定 `127.0.0.1:54329`，数据落 `postgres-data` 卷。 |
| `package.json` | pnpm 根脚本，聚合 web 的 lint/typecheck/test/build，以及 `rules:check`、`webmcp:*`、`mcp:*`。 |
| `pnpm-workspace.yaml` | 工作区定义（`apps/*`）。 |
| `pnpm-lock.yaml` | 依赖锁文件；版本升级需在 `docs/versions.md` 记录依据。 |
| `.npmrc` | npm registry、`save-exact`、`strict-peer-dependencies=true`。 |
| `.env` | 本地实际配置，不入库；由 Makefile `-include` 读入。 |
| `.env.example` | 全部环境变量样例与语义注释：`DATABASE_URL`、`COOKIE_SECURE`、`BOOTSTRAP_*`、`DEMO_AUTH_TOKEN`、`SETTINGS_ENCRYPTION_KEY`、`FILE_STORAGE_ROOT`、`PUBLIC_WORKSPACE_ID`、`PUBLIC_APP_URL`、`SMTP_ALLOWED_HOSTS`、`WEBMCP_*`、`MCP_BUSINESS_URL`。 |
| `.gitignore` / `.prettierignore` | 忽略产物目录（`dist`、`.local`、`test-results`、`playwright-report`、`.tools`）与格式化范围。 |
| `skills-lock.json` | 上游 `goa-service-designer` 等 Skill 的来源与 hash，防止手改上游指南夹带项目约定。 |
| `.github/workflows/ci.yml` | CI：pnpm/node 22.23.2 + Go + PostgreSQL service → `make ci-fast` → `test-integration` → WebMCP 真实客户端 → 全量 E2E，失败上传报告。 |
| `.workbuddy/memory/` | 会话工作记忆（每日日志 + 长期项目约定），不参与构建。 |

## 4. 命令入口（Makefile / npm scripts）

| 命令 | 作用 |
|---|---|
| `make install` / `make tools` | 安装前端依赖 / 安装锁版本 sqlc 到 `.tools/bin`。 |
| `make db-up` / `make db-down` | 起停本地 PostgreSQL。 |
| `make migrate` / `make bootstrap` | 执行迁移 / 首次创建内置管理员与租户。 |
| `make dev` / `make dev-api` | Vite 5173 / Go 8080（经 `scripts/go.sh` 使用项目工具链）。 |
| `make generate` | Goa → sqlc → Orval → prettier，四步全量生成。 |
| `make check-generated` | 重新生成后比对，防止手改生成代码。 |
| `make format` / `format-check` / `lint` / `typecheck` | 格式、ESLint + `go vet`、`tsc --noEmit`。 |
| `make test` | Vitest + `go test ./...` + i18n key 一致性。 |
| `make build` | 前端生产构建 + `go build ./...`。 |
| `make test-integration` | `-tags=integration`，连真实 PostgreSQL 的迁移/约束/范围/事务测试。 |
| `make test-e2e` / `-smoke` / `-changed` / `-parallel` | 全量 / `@smoke` 最小链路 / 按改动选文件（`scripts/e2e-changed.mjs`）/ 两套独立栈并行。 |
| `make test-webmcp-e2e` / `test-mcp` / `test-mcp-e2e` / `test-mcp-native*` | 页面工具专项 / 远程 MCP 协议+DB+浏览器 / 真实 Codex、WorkBuddy 客户端验收。 |
| `make check-rules` | 校验 AGENTS/Skill/清单结构与链接。 |
| `make test-security` | 认证、授权、配置、范围、身份、MCP、传输单测 + 集成测试。 |
| `make ci-fast` / `make ci` | 提交级门禁 / 发布候选全量。 |

## 5. Agent 资产

### 5.1 `.agents/skills/`（工程开发，按任务加载）

| Skill | 何时使用 |
|---|---|
| `baseline-feature-development` | 功能开发总入口：把受影响的层路由到其他 Skill 与验证命令。 |
| `baseline-web-development` | React 页面、路由、共享 UI、表单、表格、前端 API 消费。 |
| `baseline-database-change` | PostgreSQL schema、Goose 迁移、sqlc 查询、pgx 仓储、事务、数据范围。 |
| `baseline-security` | 认证、授权、租户/范围、敏感字段、公开端点、文件/导出/批量、临时权限。 |
| `baseline-shared-capabilities` | 复用共享设置、字典、私有文件、通知，而不是重写配置/文件基础设施。 |
| `baseline-internationalization` | 文案、语言、校验错误、locale 相关格式化。 |
| `baseline-mcp-development` | 功能对应的 MCP 工具设计、暴露面评估与变更。 |
| `baseline-quality-gates` | 选择并执行检查、诊断失败测试、准备评审。 |
| `goa-service-designer` | Goa DSL 契约变更（上游安装，禁止手改；附 4 篇参考：建模校验、传输错误拦截器、生成代码与实现、排错）。 |
| `AGENTS.md` | Skill 本身的维护规则：命名、元数据、清单同步。 |

每个 Skill 目录下的 `agents/openai.yaml` 是客户端发现用的元数据（名称、描述、触发条件）。

### 5.2 `webskills/`（运行时，随 MCP 客户端分发）

| Skill | 作用 |
|---|---|
| `baseline-web-navigate` | 在已显式连接的页面上导航、进入工作流。 |
| `baseline-web-accounts` | 筛选账号页、打开授权范围内的只读详情。 |
| `baseline-web-diagnose` | 用投影后的诊断工具复现加载/请求/校验问题。 |
| `baseline-business` | 不打开 Web 应用，直接用远程业务 MCP 读取字典选项与个人通知。 |

两者互不替代：`.agents/skills` 约束怎么写代码，`webskills` 约束客户端怎么用应用。

## 6. services/api

### 6.1 分层

| 目录 | 职责 |
|---|---|
| `design/` | Goa DSL 契约源。Payload/Result/Error/HTTP/Security 只在这里声明。 |
| `cmd/` | 四个进程入口，只做装配与生命周期。 |
| `internal/platform/capability/` | 能力契约、定义中心与唯一执行网关。 |
| `internal/platform/{policy,approval,audit,outbox,eventhub,metadata}/` | 确定性策略、审批队列、审计读模型、事务 Outbox、事件日志；`metadata` 是实体模型（解析迁移 + 派生 + E1–E10 校验），见 [实体模型](entity-model.md)。 |
| `internal/{identity,governance,auth,authorization,dataaccess}/` | 身份后台、系统治理、演示认证、Casbin 授权、数据范围解析。 |
| `internal/modules/*` | 业务模块，各自注册能力并实现 Executor。 |
| `internal/ai/*` | Eino Agent、工具适配、事件触发、上下文、知识库、工作流、评估。 |
| `internal/transport/` | 路由装配、会话守卫、限流、中间件、运行时组合根。 |
| `internal/{app,server,config,repository}/` | 技术示例服务、进程装配、配置校验、sqlc 生成仓储。 |
| `db/` | 迁移与 SQL 查询源。 |
| `gen/`、`internal/repository/dbgen/` | 生成代码，禁止手改。 |
| `tests/` | 真实 PostgreSQL 集成测试。 |

规则文件：`AGENTS.md`（服务总则）、`internal/AGENTS.md`（安全与数据访问）、`db/AGENTS.md`、`design/AGENTS.md`、`tests/AGENTS.md`。

### 6.2 design/ — API 契约源

| 文件 | 内容 |
|---|---|
| `design.go` | `API("baseline")` 定义与 `system` 服务：`GET /api/health`、`GET /api/echo`；共享 `resourceId()` 片段。 |
| `identity.go` | `identity` 服务：`login`、`me`、`logout`、`snapshot`、`switch_department`；会话 Cookie、组织/部门/角色/账号 Payload 构造器。 |
| `temporary_access.go` | 临时权限相关类型与校验（时长、方法枚举）。 |
| `governance.go` | 公共 `branding` 服务（`GET /api/public/branding`、`/logo`，`NoSecurity`）与 `ConfigEntry`、`EffectiveConfiguration`、`FileContent` 类型。 |
| `capability.go` | `capability` 服务：`definitions`、`invoke`、`invocations`、`invocation`、`approvals`、`decide`、`events`；`CapabilityContract/Result/Invocation/Approval/Event` 公开契约。 |
| `ai.go` | `ai` 服务：`status`、`run`、`runs`、`runDetail`（只含任务入口与执行记录）。 |
| `assistant.go` | `assistant` 服务：`status`、`chat`、`agent`、`workflow`、`knowledge`、`ingest`。 |

生成结果：`gen/{system,identity,capability,ai,assistant,branding}/`（service、endpoints、client）、`gen/http/<服务>/{client,server}/`、`gen/http/openapi*.{json,yaml}`（Orval 消费 3.0 版本）、`gen/http/cli/api/cli.go`。

### 6.3 cmd/ — 进程入口

| 文件 | 作用 |
|---|---|
| `cmd/api/main.go` | 加载配置、初始化结构化日志、信号处理，交给 `server.Run`。 |
| `cmd/bootstrap/main.go` | 首次部署创建内置管理员与工作区（读取 `BOOTSTRAP_*`）。 |
| `cmd/migrate/main.go` | 执行 Goose 迁移（嵌入 `db/migrations`）。 |
| `cmd/e2e/main.go` | E2E 专用启动器：从 `TEST_DATABASE_URL` 建隔离数据库/schema，迁移后以独立端口启动服务，供 Playwright `webServer` 使用。 |

### 6.4 能力平台核心

| 文件 | 作用 |
|---|---|
| `platform/capability/contracts/contracts.go` | 平台稳定词汇：`Risk`、`Approval`、`Origin`、`Status`、`Definition`、`Input/Output/Result`、`Event`、`Executor`/`TxExecutor`、`Handler`、`Caller`、`Redact`、`Digest`、`Validate`。不依赖存储与传输。 |
| `contracts/schema.go` | 输入/输出 Schema 与运行时校验（类型、长度、枚举、数值边界、JSON）。 |
| `contracts/helpers.go` | Schema 构造助手：`Bind`、`Str`、`Text`、`Enum`、`Int`、`Bool`。 |
| `contracts/errors.go` | 稳定错误码、`Problem` 投影与业务/请求错误区分。 |
| `contracts/contracts_test.go` | 定义校验、工具名冲突、脱敏白名单与摘要一致性。 |
| `platform/capability/registry/registry.go` | 进程内定义中心：启动时校验版本、Handler、工具名冲突，`Resolve`/`All`/`Entities`；调用不跨网络。 |
| `registry/registry_test.go` | 拒绝非法目录、选最新版本、按权限发现、实体派生。 |
| `platform/capability/gateway/gateway.go` | 唯一运行时入口：身份→权限→Schema→幂等→策略/审批→执行→Outbox→审计；`Invoke`、`Decide`、`Definitions`、`Invocations`、`Approvals`、`Events`。 |
| `gateway/execute.go` | 实际执行编排：占用调用行、`TxExecutor` 与 Outbox 同事务、事件追加与去重、结果落库、错误码映射。 |

### 6.5 平台支撑包

| 文件 | 作用 |
|---|---|
| `platform/policy/policy.go` | 确定性准入：风险 × 来源 → 直接执行 / 需确认 / 拒绝；人类点击可执行 high，自动化 high 需确认，critical 对自动化关闭。 |
| `platform/policy/policy_test.go` | 上述四类判定的回归用例。 |
| `platform/approval/store.go` | 审批持久化与条件决策（重复/过期/跨租户冲突），审批载荷脱敏存储。 |
| `platform/audit/store.go` | 调用审计读模型：谁、经哪个入口、做了什么、结果如何；按 correlation 串联。 |
| `platform/outbox/relay.go` | 事务 Outbox 的 `Meta`/`Event`/`AppendTx` 与至少一次 Relay。 |
| `platform/eventhub/hub.go` | 持久事件日志 + 进程内 fan-out，唤醒 AI Trigger；消费端按事件 id 去重。 |
| `platform/metadata/entity.go` | 从能力目录派生实体元数据（有哪些业务对象、能做什么），供控制台与 Agent 上下文使用。 |
| `platform/metadata/schema.go` | 解析 embed 的 goose 迁移，还原基表、列、CHECK 枚举与复合外键（`ParentOf`/`ScopeTarget`/`ChildrenOf`）。 |
| `platform/metadata/validate.go` | 实体声明与物理 schema 的 E1–E6、E10 校验；`ReferenceAliases` 处理身份指针表。 |
| `platform/metadata/transitions.go` | E8：状态转移与能力目录的双向核对。 |
| `cmd/entitycheck/` | `make check-entities` 入口：结构问题、契约问题、schema 问题、hint 与 accepted gaps 分列输出。 |

### 6.6 internal/identity — 身份与权限后台

| 文件 | 作用 |
|---|---|
| `service.go` | `Service` 聚合与用例编排入口（`New`、可用性检查、错误映射）。 |
| `policy.go` | `Principal` 定义、上下文读写、`Can` 权限判定。 |
| `governance.go` | `SecurityPolicy`：为治理模块提供 `Authorize`/`ScopeFor`（复用正式授权，不另起一套）。 |
| `catalog.go` | 两级权限目录 `PermissionCatalog`，输出给 DSL、权限矩阵与临时申请。 |
| `capability.go` | 12 个身份写能力（`identity.{account,organization,department,role}.{create,update,delete}`），统一 high + 需确认，并声明成功事件。 |
| `management.go` | 账号/组织/部门/角色的不变量校验：归属合法性、内置管理员保护、授权管理独立门禁。 |
| `management_types.go` | 管理用例输入类型（`AccountInput`、`OrganizationInput`、`DepartmentInput`、`RoleInput`、`DeleteInput`）。 |
| `memberships.go` | 多部门归属与部门角色：`SwitchDepartment`、归属变更校验、部门授权聚合（不跨部门合并）。 |
| `temporary.go` | 临时权限：申请、审批（`canReview`）、上限校验、到期与撤销、范围在批准时固定。 |
| `repository.go` | 模块仓储：调用 sqlc `dbgen`，做字段投影与模型映射。 |
| `password.go` | 密码哈希与校验（含恒定时间占位工作，避免时序泄漏）。 |
| `migrate.go` | 迁移执行助手（bootstrap 与 e2e 共用）。 |
| `bootstrap.go` | 首次部署：创建内置管理员、租户、基础角色。 |
| `identity_test.go` | 哈希、账号覆盖替换范围、deny 优先、策略并集、目录不放大权限。 |

### 6.7 认证、授权、数据范围、配置、治理、装配

| 文件 | 作用 |
|---|---|
| `auth/principal.go` | `Principal` 抽象与 `Authenticator` 接口；可信身份只从这里进入。 |
| `auth/development.go` | 本地演示令牌认证（仅 loopback 开启，与浏览器会话分离）。 |
| `auth/development_test.go` | 令牌必须显式开启。 |
| `authorization/enforcer.go` | Casbin RBAC（域 = 租户）封装：`Allow`、`Revoke`。 |
| `authorization/model.conf` / `policy.csv` | 授权模型与初始策略。 |
| `authorization/enforcer_test.go` | 租户隔离与即时撤销。 |
| `dataaccess/scope.go` | 六类可信范围解析（ALL/ORGANIZATION/DEPARTMENT/DEPARTMENT_TREE/OWN/CUSTOM），失败即拒绝。 |
| `dataaccess/scope_test.go` | fail-closed 行为。 |
| `config/config.go` | 环境变量加载与集中校验：非法安全配置直接启动失败。 |
| `config/config_test.go` | 演示令牌限 loopback、生产必须 HTTPS 原点。 |
| `governance/core.go` | 治理 `Core`：连接池、`Q`、身份、文件存储、加密（设置/字典/通知共用）。 |
| `governance/registry.go` | 配置项/字典定义的注册表：所有者、层级、校验、消费者声明。 |
| `governance/core_test.go` | 加密绑定租户与密钥、注册项拒绝不安全输入。 |
| `app/system.go` | 技术示例服务 `System`：`Health`、`Echo`（无敏感数据）。 |
| `server/server.go` | 进程装配：建池、构造 identity/governance/各模块、启动通知与排期 worker、MCP 保留期清理、HTTP 超时与优雅关闭。 |

### 6.8 internal/transport — 传输与组合根

| 文件 | 作用 |
|---|---|
| `http.go` | `Handler*` 构造函数与中间件链：请求 ID、结构化日志、认证、授权、错误映射、CORS/CSRF。 |
| `identity.go` | 挂载 identity 服务；`guardedPath`（`/api/admin\|auth\|assistant\|ai/` 与 `/api/capabilities`）会话守卫、同源校验、登录限流。 |
| `capability.go` | capability 服务适配：把网关返回投影为 Goa 类型，统一错误编码。 |
| `ai.go` | ai 服务适配（status/run/runs/runDetail）。 |
| `assistant.go` | assistant 服务适配（chat/agent/workflow/knowledge/ingest）。 |
| `governance.go` | **运行时组合根**：MCP `/mcp` 端点、知识库、harness、trigger engine、Outbox relay、能力目录装配、Gateway 构造、branding 投影、后台 starter。 |
| `http_test.go` | 每个受保护面都被会话守卫覆盖、HTTP 边界行为。 |

### 6.9 internal/modules — 业务模块

| 文件 | 作用 |
|---|---|
| `example/module.go` | 参考模块：3 个能力（list/create/archive，覆盖读→写→高风险状态变更）+ Outbox 事件 + 事件触发 AI 评审，示范完整闭环。 |
| `settings/types.go` | 设置域类型：`ConfigDefinition`、`ConfigEntry`、`ChangeEntry/ChangeRecord`、`ConfigTarget`。 |
| `settings/capability.go` | 12 个设置能力（读工作区/生效值/偏好、草稿、发布、回滚、导出、Logo、密码验证、排期增删查）。 |
| `settings/service.go` | 发布事务：版本锁、同事务快照与审计、敏感发布单次绑定重新认证、偏好写入。 |
| `settings/schedules.go` | 定时发布 worker：`pending` 部分唯一索引 + `SKIP LOCKED` 领取，到期重建原 Principal 重新授权，状态与失败码原子保存。 |
| `dictionaries/types.go` | 字典域类型：定义、文档、条目、覆盖、引用。 |
| `dictionaries/capability.go` | 11 个字典能力（列表/详情/选项、草稿、发布、回滚、删除、覆盖、导入预览与草稿、导出）。 |
| `dictionaries/service.go` | 字典发布/版本/草稿规范化与所有者判定。 |
| `dictionaries/import.go` | CSV 导入解析：列校验、公式转义往返、预览与草稿生成。 |
| `dictionaries/import_test.go` | 公式保护往返、重复列与畸形行拒绝。 |
| `files/types.go` | 文件域类型：`ManagedFile`、`FileContent`、`DriverDescriptor`。 |
| `files/capability.go` | 5 个文件能力（列表、驱动、上传、读取、删除）。 |
| `files/service.go` | 上传/读取/删除用例、绑定校验、过期清理 `Purge`。 |
| `files/storage.go` | 存储驱动注册表（`Registry`/`Profile`）与本地驱动：路径包含校验、类型按字节嗅探、失败清理。 |
| `files/storage_test.go` | 路径逃逸与类型伪装负向用例。 |
| `notifications/types.go` | 通知域类型：公告、受众、投递记录、收件箱消息、附件。 |
| `notifications/capability.go` | 11 个通知能力（受众/预览/发送/已发/附件/消息/收件箱/投递/重试/SMTP 测试）。 |
| `notifications/service.go` | 发送事务（主体+快照+附件绑定+投递同事务、账号去重、摘要幂等）、投递 worker `Run`、收件箱。 |
| `notifications/announcements.go` | 受众解析与覆盖校验、内容/引用校验、预览。 |
| `notifications/announcements_test.go` | 内容引用与邮件头边界。 |
| `notifications/smtp.go` | SMTP 配置解析、主机白名单、TLS 连接与发送。 |
| `mcp/types.go` | 远程 MCP 域类型：令牌、工具集合、调用审计。 |
| `mcp/capability.go` | 7 个 MCP 管理能力（令牌创建/列表/撤销/轮换、能力目录、审计查询），均为高危需确认（除只读）。 |
| `mcp/service.go` | 令牌签发与校验（哈希存储、集合与正式权限取交集、撤销实时生效）、敏感操作重新认证。 |
| `mcp/http.go` | 远程 MCP 协议端点 `/mcp`（Streamable HTTP）：凭证校验后转调能力，与业务命令分离。 |
| `mcp/registry.go` | 远程只读工具目录：页面/字典/通知等工具的业务输入映射回 Goa 源类型。 |
| `mcp/limiter.go` | 按令牌的独立限流桶。 |
| `mcp/maintenance.go` | 令牌与审计的保留期清理。 |
| `mcp/http_test.go` / `registry_test.go` | 缺失凭证/跨域拒绝；工具 Schema 缺参拒绝、限流独立、字典边界与契约一致。 |

### 6.10 internal/ai — Agent 运行时

| 文件 | 作用 |
|---|---|
| `harness/types.go` | `Run`/`Result`：AI 任务记录（与业务调用生命周期独立）。 |
| `harness/service.go` | Eino Agent 执行：模型解析、授权、调用工具（工具内部仍走 Gateway）、产出回复与轨迹。 |
| `harness/store.go` | `ai_runs` 持久化：`Start`、`Finish`、`List`、`Available`。 |
| `harness/config.go` | 模型/端点/密钥只从服务端设置解析，未配置即 `ready=false`。 |
| `tools/tools.go` | 能力 → Eino Tool 适配：只服务描述，每次调用都过 Gateway（权限、审批、幂等、审计一致）。 |
| `triggers/engine.go` | 业务事件 → AI 任务：策略查找、按 `(policy,event)` 去重、`maxDepth` 深度限制、独立状态。 |
| `context/context.go` | 交给模型的结构化上下文：谁在操作、在哪一页、能做什么；值当数据不当指令。 |
| `knowledge/knowledge.go` | 知识库：分块（重叠窗口、块数上限）、存储、租户范围检索。 |
| `knowledge/service.go` | 文档接入与列表。 |
| `knowledge/capability.go` | `knowledge.search` 能力（low/auto）。 |
| `knowledge/management.go` | `knowledge.list` / `knowledge.create` / `knowledge.delete`（控制台知识库管理）。 |
| `knowledge/components.go` | Eino `document.Transformer`（Chunker）与 `indexer.Indexer` 适配。 |
| `knowledge/knowledge_test.go` | 分块重叠与上限。 |
| `workflow/answer.go` | 确定性 Eino 图：检索 → 受限提示 → 模型作答（有据回答）。 |
| `evaluation/summary.go` | 运行轨迹指标：完成率、失败率、延迟、token 消耗。 |
| `assistant/service.go` | 内嵌助手模型网关：OpenAI Responses 协议映射、端点/主机限制、设置项读取、错误脱敏。 |
| `assistant/service_test.go` | 消息映射、协议调用、上游错误不外泄、端点不可指向任意主机。 |

### 6.11 db/ — 迁移与查询

| 迁移 | 内容 |
|---|---|
| `00001_technical_demo.sql` | `technical_demo_records`（技术示例）。 |
| `00002_identity.sql` | `iam_organizations`、`iam_departments`、`iam_roles`、`iam_accounts`、`iam_account_roles`、`iam_sessions`、`iam_login_attempts`。 |
| `00003_temporary_access.sql` | `iam_temporary_requests`（申请与最终批准分开存）。 |
| `00004_department_memberships.sql` | `iam_department_memberships`、`iam_department_membership_roles`。 |
| `00005_governance.sql` | `gov_scopes`、`gov_revisions`、`gov_values`、`gov_drafts`、`gov_versions`、`gov_audit`、`gov_reauth`、`gov_dictionaries*`、`gov_storage_profiles`、`gov_files`、`gov_file_bindings`、`gov_events`、`gov_notifications`、`gov_deliveries`。 |
| `00006_governance_identity_lifecycle.sql` | `gov_identity_refs`（身份变更联动）。 |
| `00007_global_governance.sql` | 设置/字典统一为 `workspace:0`，迁移既有引用。 |
| `00008_notification_announcements.sql` | `gov_announcements`、`gov_announcement_files`。 |
| `00009_settings_schedules.sql` | `gov_settings_schedules`（pending 部分唯一索引 + 行锁）。 |
| `00010_mcp_tokens.sql` | `mcp_tokens`、`mcp_token_tools`、`mcp_call_audits`。 |
| `00011_mcp_identity_lifecycle.sql` | 身份变更时令牌撤销与归属延迟检查。 |
| `00012_mcp_revocation_actor.sql` | 记录撤销执行者。 |
| `00013_knowledge.sql` | `kb_documents`、`kb_chunks`。 |
| `00014_ai_platform.sql` | `ai_runs`、`ai_run_steps`、`ai_approvals`（后两者已被 015 取代/清理）。 |
| `00015_capability_runtime.sql` | `cap_invocations`、`cap_outbox`、`cap_events`、`cap_approvals`、`ai_trigger_policies`、`ai_trigger_executions`、`example_records`；删除旧 AI 步骤/审批表。 |
| `embed.go` | 把迁移目录嵌入二进制，供 migrate/bootstrap 使用。 |

查询源（sqlc 输入，生成到 `internal/repository/dbgen`）：

| 文件 | 查询数 | 覆盖 |
|---|---|---|
| `capability.sql` | 32 | 调用占用/幂等查找/结束、事件、审批、触发策略与执行。 |
| `governance.sql` | 68 | 作用域、版本、草稿、审计、字典、文件绑定、通知投递。 |
| `identity.sql` | 50 | 组织、部门、角色、账号、会话、登录尝试、归属。 |
| `mcp.sql` | 16 | 令牌、工具集合、调用审计、保留期。 |
| `announcements.sql` | 11 | 公告、接收快照、附件、投递。 |
| `temporary_access.sql` | 8 | 申请、审批、到期、撤销。 |
| `settings_schedules.sql` | 7 | 排期领取、完成、取消。 |
| `ai.sql` | 6 | `ai_runs` 轨迹。 |
| `knowledge.sql` | 5 | 文档与分块检索。 |

### 6.12 生成目录

| 目录 | 生成器 | 说明 |
|---|---|---|
| `services/api/gen/` | Goa | 服务接口、端点、HTTP client/server 编解码、OpenAPI（3.0 给 Orval，3.2 留作产物）、CLI。 |
| `services/api/internal/repository/dbgen/` | sqlc | 每个查询源一个 `*.sql.go`，另有 `db.go`（`Queries`/事务）与 `models.go`。 |
| `apps/web/src/api/generated/` | Orval | `baseline.ts`（react-query hooks）与 `models/*`（DTO）。 |

`sqlc.yaml` 定义 sqlc 的输入输出与命名规则；`go.mod`/`go.sum` 锁定 Go 依赖。

### 6.13 tests/ — 集成测试

| 文件 | 覆盖 |
|---|---|
| `ai_platform_test.go` | 能力闭环（业务数据 + Outbox + 事件 → AI 触发）、权限与跨租户隔离、审批恢复（幂等并发、`PrincipalForTask` 重建身份）。 |
| `capability_modules_test.go` | 各业务模块能力的真实闭环与事件断言。 |
| `identity_test.go` | 登录、会话、身份管理、允许/拒绝、范围外 ID。 |
| `temporary_access_test.go` | 临时权限全生命周期与边界（自批、再委托、到期、并发）。 |
| `membership_migration_test.go` | 多部门迁移保留既有身份与申请。 |
| `governance_global_migration_test.go` | 公共治理迁移保留字典引用。 |
| `mcp_native_test.go` | 真实客户端（Codex/WorkBuddy）原生互通。 |
| `AGENTS.md` | 测试规约：必须连真实隔离 PostgreSQL，不得替换内存实现。 |

## 7. apps/web

### 7.1 入口与 app/

| 文件 | 作用 |
|---|---|
| `src/main.tsx` | 挂载 React、导入 i18n 与全局样式、渲染 Providers + Router。 |
| `src/app/providers/providers.tsx` | 组合 TanStack Query、主题、认证、助手、WebMCP 五类 Provider。 |
| `src/app/providers/theme.tsx` | 深浅主题切换与持久化。 |
| `src/app/layouts/shell.tsx` | 公开页面外壳（顶部导航、页脚），`shell.test.tsx` 覆盖。 |
| `src/app/router/router.tsx` | 全部路由：登录、`/`（解析到 AI 工作台或数据统计）、数据统计 `/analytics`、能力、AI、账号/组织/权限、设置、MCP、通知、附件、临时权限；旧路由 `/departments`、`/roles`、`/dictionaries`、`/webmcp`、`/mcp-settings` 只做重定向；`/foundation`、`/architecture` 为公开页。 |
| `src/test/setup.ts` | Vitest jsdom 环境与 jest-dom、全局 mock。 |

### 7.2 components/ui

12 个自有 shadcn 源码：`badge`、`button`、`card`、`checkbox`、`dialog`、`dropdown-menu`、`input`、`select`、`separator`、`table`、`tabs`、`tooltip`。可维护但必须领域无关；局部规则见 `components/ui/AGENTS.md`。

### 7.3 features/

**administration（登录、组织、账号、权限）**

| 文件 | 作用 |
|---|---|
| `auth.tsx` | `AuthProvider`/`useSession`/`usePermission`：会话获取、切换部门、权限判断（仅影响体验，真正授权在服务端）。 |
| `layout.tsx` | `AuthBoundary`、`RequireSession`、`AdminLayout`：侧栏 + 内容面板框架、未登录跳转。 |
| `login.tsx` | 登录页（`login.test.tsx`）。 |
| `dashboard.tsx` | 数据统计页（`/analytics`，原工作台看板）。 |
| `pages.tsx` | 账号页、组织页（含部门标签）、权限页（角色授权/账号独立授权/角色管理标签）。 |
| `forms.tsx` | 组织/部门/角色/账号表单（RHF + Zod）。 |
| `permission-matrix.tsx` | 权限矩阵：读取后端目录做一/二级合并，不硬编码权限码。 |
| `temporary-access.tsx` | 临时权限申请与审批页（申请与最终批准分开展示）。 |
| `shared.tsx` | 查询 hooks、`BrandMark`、状态徽标、范围标签、错误/校验文案映射。 |
| `*.test.tsx` | 表单、登录、导航、权限矩阵、共享组件的 RTL 用例。 |

**governance（系统设置、字典、通知、附件）**

| 文件 | 作用 |
|---|---|
| `runtime.tsx` | `RuntimeProvider`/`useConfiguration`：拉取公开品牌与鉴权运行时配置并缓存。 |
| `branding.tsx` | `useBranding`、`PublishedLogo`：公开品牌读取与 Logo 展示。 |
| `settings.tsx` | 系统设置页（系统配置/字段标签、个人偏好复用同组件）。 |
| `dictionaries.tsx` | 字典管理：列表、详情、草稿编辑、发布/回滚、`DictionaryOptions` 供业务引用。 |
| `files.tsx` | 附件列表、上传、`downloadFile`、按 id 的下载页。 |
| `notifications.tsx` | 铃铛、通知列表、详情、投递面板。 |
| `notification-compose.tsx` / `notification-composer.tsx` / `notification-editor.tsx` | 通知撰写页、撰写器（受众选择、附件、预览）与 Markdown 编辑器。 |
| `markdown.tsx` | 通知正文的受限 Markdown 渲染（不渲染资源 HTML）。 |
| `schedule-time.ts` | 排期的本地时间 ↔ UTC 时刻换算（输入设备时区）。 |
| `shared.tsx` | 治理查询 hooks、`NoAccess`、`ChangeHistory`、`downloadContent`、`FormError`。 |

**capabilities / ai**

| 文件 | 作用 |
|---|---|
| `capabilities/page.tsx` | `/capabilities`：能力目录、调用、调用轨迹、审批队列与事件（页面按钮接能力的基准）。 |
| `capabilities/shared.tsx` | 风险徽标、审批策略、调用状态、AI 状态徽标与标签映射。 |
| `ai/page.tsx` | `/ai`：`?tab=chat\|runs` 单一入口。 |
| `ai/workbench.tsx` | 对话/Agent 工作台：发送、轨迹、审批卡片（只展示服务端投影参数）。 |
| `ai/runs.tsx` | 执行记录与运行详情。 |
| `ai/shared.tsx` | AI 查询 hooks、状态徽标、`RunTrace`、`ApprovalCard`。 |

**mcp / webmcp / assistant / example**

| 文件 | 作用 |
|---|---|
| `mcp/hub.tsx` | `/mcp` 三个标签：MCP Server、WebMCP、WebSkills。 |
| `mcp/remote.tsx` | 账号级令牌面板：创建（秘密只显示一次）、列表、撤销、轮换、审计。 |
| `webmcp/provider.tsx` | `WebMcpProvider`：显式连接、页面工具注册/撤销、`usePageTools`。 |
| `webmcp/settings.tsx` | 本机服务连接表单（端口/配对密钥，开发服务器自动预填）。 |
| `webmcp/skills.tsx` | WebSkills 安装面板。 |
| `assistant/provider.tsx` | 助手状态与对话上下文 Provider。 |
| `assistant/widget.tsx` | 页面浮动入口 `AssistantLauncher`。 |
| `example/overview.tsx` | `/foundation` 技术验收页（不作为后台视觉基准）。 |

各目录下的 `*.test.tsx` 是组件/交互用例，`*.css` 是该功能的局部样式（`ai.css`、`assistant.css`、`capability.css`、`hub.css`、`mcp.css`、`webmcp.css`）。

### 7.4 lib/

| 文件 | 作用 |
|---|---|
| `lib/capabilities.ts` | 控制台能力 SDK：为已开放能力提供 59 个薄封装，只整形参数并解包 `CapabilityResult`，写操作带稳定 `idempotencyKey`。 |
| `lib/capability.ts` | 能力标签/描述/风险排序与色调映射。 |
| `lib/format.ts` | `Intl` 日期、数值、货币格式化（只负责展示）。 |
| `lib/utils.ts` | `cn`（clsx + tailwind-merge）。 |
| `lib/mcp/connection.ts` | 远程 MCP 的安装/预置/诊断（`prepareInstall`、`installPrepared`、`diagnoseRemote`）。 |
| `lib/webmcp/runtime.ts` | 受控页面工具注册表：`pageTool`、可用性判定、结果/失败包装。 |
| `lib/webmcp/routes.ts` | 页面工具路由清单（25 条，含公开路由），与路由表同步维护。 |
| `lib/webmcp/connection.ts` | 本机连接信息获取、客户端设置与页面绑定。 |

`lib/webmcp/routes.test.ts` 与 `runtime.test.ts` 覆盖路由清单一致性与工具注册表行为；`api/client/request.test.ts`、`i18n/i18n.test.ts` 同理覆盖各自约定。

### 7.5 api/

| 文件 | 作用 |
|---|---|
| `api/client/request.ts` | Orval 的统一 mutator：`fetch` 封装、CSRF 令牌、`ApiError` 归一化。 |
| `api/client/request.test.ts` | 错误映射与 CSRF 行为。 |
| `api/types.ts` | 能力模块的业务类型（契约即 API，形态由控制台拥有，镜像 Go 域类型）。 |
| `api/generated/baseline.ts` + `models/*` | 生成代码：react-query hooks 与 DTO，禁止手改。 |

### 7.6 i18n/

| 文件 | 作用 |
|---|---|
| `config.ts` | 资源加载、`storageKey`、浏览器语言与用户偏好解析（默认 `en`）。 |
| `index.ts` | i18n 实例与统一 `changeLanguage`（同步 `document.lang` 与标题）。 |
| `locales/en\|zh-CN/*.json` | 10 个命名空间：`admin`、`ai`、`assistant`、`capability`、`common`、`governance`、`mcp`、`navigation`、`validation`、`webmcp`；两种语言 key 集合必须一致。 |
| `i18n.test.ts` | 语言切换与 key 一致性。 |

### 7.7 样式

| 文件 | 作用 |
|---|---|
| `src/styles/index.css` | 语义 tokens、Tailwind 分层、基础排版与主题变量。 |
| `src/styles/admin.css` | 后台框架：侧栏底色 + 内容面板、滚动与圆角结构。 |

### 7.8 构建与测试配置

| 文件 | 作用 |
|---|---|
| `vite.config.ts` | React + Tailwind 插件、`@` 别名、5173 端口、`/api` 与 `/mcp` 同源代理、开发期 `/__webmcp/connection` 便利端点（仅 loopback + 同源）、Vitest jsdom 配置。 |
| `tsconfig.json` | 严格 TS 配置与路径别名。 |
| `index.html` / `public/favicon.svg` | 页面骨架与图标。 |
| `components.json` | shadcn CLI 配置（风格、别名、registry）。 |
| `eslint.config.js` | ESLint 分层配置（TS、react-hooks、react-refresh）。 |
| `eslint-rules.mjs` | 自定义 ESLint 规则：用户可见文案必须走 i18next（含 `placeholder`/`title`/`alt`/`aria-label`）、请求必须用生成 SDK 与共享 client 而非裸 fetch。 |
| `lint-rules.test.mjs` | 自定义规则的通过/失败用例。 |
| `orval.config.ts` | 读取 Goa 的 `openapi3.json`，修正 binary 格式，输出 react-query 客户端与 models，mutator 指向 `api/client/request.ts`。 |
| `playwright.config.ts` | 默认栈：5179 前端 + 8099 Go + 18794 WebMCP，单 worker，Asia/Shanghai、en-US，失败留 trace/screenshot。 |
| `playwright.governance.config.ts` | 治理专用栈：5183/8101，只跑 `governance`/`notifications`/`settings-schedules`。 |
| `e2e/*.spec.ts` | 9 个浏览器验收文件：`foundation`、`administration`、`capabilities`、`ai`、`governance`、`notifications`、`settings-schedules`、`mcp`、`webmcp`。 |
| `e2e/AGENTS.md` | 浏览器验收规则：真实 Go + 生产构建 preview、role/label 定位、按改动选文件。 |

## 8. scripts/

| 文件 | 作用 |
|---|---|
| `go.sh` | 用 `.tools/go` 与项目 `GOMODCACHE`/`GOCACHE` 在 `services/api` 下执行 Go 命令。 |
| `check-gofmt.sh` | 检查（或 `--write`）全目录格式，含 integration build tag 文件。 |
| `check-generated.mjs` | 对 `gen`、`dbgen`、`api/generated` 做 hash 快照，重新生成后比对，发现手改即失败。 |
| `check-i18n.mjs` | 校验语言目录与命名空间文件一致、key 集合一致。 |
| `check-agent-rules.mjs` | 校验 AGENTS/Skill/`agent-policy.json` 结构、元数据与链接可达。 |
| `check-agent-rules.test.mjs` | 上述检查的正/负路径。 |
| `e2e-changed.mjs` | 按 git 改动显式映射到 E2E 文件；未映射路径会被报告而不是扩散成全量。 |
| `e2e-changed.test.mjs` | 映射选择的用例。 |
| `e2e-parallel.sh` | 两套独立栈并行跑完整浏览器集合（默认栈 + 治理栈）。 |
| `webmcp-server.mjs` | 本机 WebMCP 服务：默认 18793 loopback、Host/Origin 精确校验、配对密钥文件、官方代理包。 |
| `webmcp-install.mjs` | 客户端设置安装器：allowlist 客户端、备份后合并配置、失败正确报告。 |
| `webmcp-stdio.mjs` | stdio 适配器：复用当前页工具并转发工具目录变化。 |
| `webmcp-result.mjs` | 解码 MCP 结果信封，同时保留普通工具文本。 |
| `webmcp.test.mjs` | 服务/安装/适配器的成功与拒绝路径。 |
| `mcp-business-helper.mjs` | 远程业务 MCP 本机助手（127.0.0.1:18796，启动时固定服务原点）。 |
| `mcp-business-stdio.mjs` | 远程业务 MCP 的 stdio 桥：凭证只从 0600 文件读，校验前缀与权限。 |
| `mcp-business-codex-check.mjs` | Codex app-server 验收助手，不发起模型对话。 |
| `mcp-business-native-check.mjs` | 原生客户端验收：隔离配置，不动用户 profile。 |
| `mcp-business.test.mjs` | 远程 MCP 脚本层用例。 |
| `AGENTS.md` | 脚本规则：退出码、不吞异常、清单同步、生成只写约定目录。 |

## 9. docs/

| 文件 | 作用 |
|---|---|
| `architecture.md` | 分层与链路总览、生成边界、测试策略。 |
| `agent-tooling.md` | 任务 → 框架/工具 → Skill → 目录规则 → 验证 的矩阵。 |
| `agent-workflow.md` | Agent 长期开发工作流。 |
| `api-contracts.md` | API 契约约定。 |
| `coding-conventions.md` | 编码约定。 |
| `database-conventions.md` | 迁移、查询、事务、范围过滤约定。 |
| `design-system.md` | 设计系统与视觉一致性基准。 |
| `internationalization.md` | 国际化规范。 |
| `security.md` | 认证、授权、数据范围、敏感字段、公开端点规则。 |
| `shared-capabilities.md` | 公共能力接入约定（设置/字典/文件/通知）。 |
| `testing.md` | 测试分层与执行节奏（含 `#测试执行节奏`）。 |
| `local-development.md` | 本地启动与工具链。 |
| `mcp-service-plan.md` / `mcp-operations.md` / `mcp-verification.md` | 远程业务 MCP 的实现方案、接入与恢复、首阶段验证记录。 |
| `system-settings-and-dictionaries-plan.md` | 设置/字典/通知附件的正式方案。 |
| `source-specification.md` | 原始框架规范（保留来源，不追改）。 |
| `verification.md` / `versions.md` / `versions.json` | 验收记录、版本依据、机器可读版本表。 |
| `agent-policy.json` | Skill 与规则的显式登记清单，`check-rules` 的输入。 |
| `decisions/001…015` | ADR：基础框架、身份权限、临时授权、多部门、系统治理、公共治理、通知中心、WebMCP、定时发布、远程 MCP、内嵌助手、Eino Agent、AI-Native、能力中心。 |
| `licenses/*` | 第三方源码说明（Goa MIT、shadcn MIT）与许可证原文。 |
| `assets/*` | README 与架构图资源（png/svg）。 |

## 10. 能力清单

共 65 个已注册能力，全部在 `internal/modules/*`、`internal/identity`、`internal/ai/knowledge` 的 `Entries()` 中注册一次。业务模块不再有各自的 REST 端点。风险与审批来自契约声明；实际是否放行由 `platform/policy` 按「风险 × 来源」决定。

**身份（12，全部 high + 需确认，均声明成功事件）**

| 能力 | 说明 |
|---|---|
| `identity.account.create/update/delete` | 账号增改删（含部门与权限覆盖），事件 `identity.account.{created,updated,deleted}`。 |
| `identity.organization.create/update/delete` | 组织（企业/公司）增改删。 |
| `identity.department.create/update/delete` | 部门增改删。 |
| `identity.role.create/update/delete` | 角色与授权增改删。 |

**设置（12）**

| 能力 | 风险/审批 | 说明 |
|---|---|---|
| `settings.workspace.read` | low/auto | 工作区配置目录。 |
| `settings.effective.read` | low/auto | 生效配置与 revision。 |
| `settings.preferences.update` | low/auto | 个人偏好。 |
| `settings.draft.create` | medium/auto | 创建草稿。 |
| `settings.draft.publish` | high/confirm | 发布（版本锁 + 同事务快照/审计）。 |
| `settings.history.rollback` | high/confirm | 回滚到历史版本。 |
| `settings.schedule.list/create/cancel` | low / high / medium | 定时发布查询、创建、取消。 |
| `settings.export.read` | low/auto | 导出配置。 |
| `settings.logo.read` | low/auto | 品牌 Logo。 |
| `settings.password.verify` | medium/auto | 敏感操作重新认证。 |

**字典（11）**

| 能力 | 风险/审批 | 说明 |
|---|---|---|
| `dictionary.list.read` / `detail.read` / `options.read` | low/auto | 列表、详情、业务选项。 |
| `dictionary.draft.create` | medium/auto | 草稿编辑。 |
| `dictionary.draft.publish` | high/confirm | 发布。 |
| `dictionary.history.rollback` | high/confirm | 回滚。 |
| `dictionary.delete` | high/confirm | 删除（历史保留引用）。 |
| `dictionary.override.save` | medium/auto | 覆盖项。 |
| `dictionary.import.preview` / `import.draft` | low / medium | CSV 导入预览与生成草稿。 |
| `dictionary.export.read` | low/auto | 导出。 |

**文件（5）**

| 能力 | 风险/审批 | 说明 |
|---|---|---|
| `file.list.read` / `drivers.read` / `content.read` | low/auto | 列表、驱动、受绑定鉴权读取。 |
| `file.upload` | medium/auto | 上传（类型按字节校验）。 |
| `file.delete` | high/confirm | 删除。 |

**通知（11）**

| 能力 | 风险/审批 | 说明 |
|---|---|---|
| `notification.audience.read` / `preview.read` | low/auto | 受众解析、发送预览。 |
| `notification.send` | high/confirm | 发送（主体/快照/附件/投递同事务、账号去重、幂等）。 |
| `notification.sent.read` / `delivery.read` / `delivery.retry` | low / low / medium | 已发、投递记录、重试。 |
| `notification.inbox.read` / `message.read` / `attachment.read` | low/auto | 收件箱、消息、附件（按接收人+归属过滤）。 |
| `notification.smtp.test` | medium/auto | SMTP 连通测试。 |

**MCP 管理（7）**

| 能力 | 风险/审批 | 说明 |
|---|---|---|
| `mcp.token.create` / `revoke` / `rotate` | high/confirm | 令牌生命周期（秘密只显示一次）。 |
| `mcp.token.list` / `capability.list` / `audit.list` | low/auto | 令牌、可用工具、调用审计。 |

**参考模块与知识（4）**

| 能力 | 风险/审批 | 说明 |
|---|---|---|
| `example.record.list` | low/auto | 列表（技术验收）。 |
| `example.record.create` | medium/auto | 创建，事件 `example.record.created`；内置 `example-record-review` 触发策略，会起一次 AI 评审。 |
| `example.record.archive` | high/confirm | 归档，事件 `example.record.archived`；未内置触发策略，事件只落 `cap_events`。 |
| `knowledge.search` | low/auto | 知识库租户范围检索。 |

## 11. 运行时链路

下面四条是骨架；`example.record.archive` 的逐层代码剖析（每层的判定顺序、事务边界、失败码）见 [一次能力调用的逐层剖析](capability-invocation-walkthrough.md)。

**页面按钮**

```
React 组件 → lib/capabilities.ts（整形参数 + idempotencyKey）
  → POST /api/capabilities/{name}/invocations → transport/capability.go
  → gateway.Invoke：身份 → 权限 → Schema → 幂等 → policy → [审批] → TxExecutor
  → 业务数据 + cap_outbox（同事务）→ cap_invocations 审计 → 返回 CapabilityResult
```

**事件驱动**

```
cap_outbox → outbox.Relay（至少一次）→ eventhub → triggers.Engine（(policy,event) 去重 + maxDepth）
  → harness.Run（ai_runs）→ Eino Agent → tools.Adapter → gateway.Invoke（重新授权，origin=agent）
```

**审批**

```
policy 判定需确认 → approval.Store 创建卡片（脱敏载荷 + digest + TTL）
  → 人类 decide → gateway.Decide 条件更新 → PrincipalForTask 重建申请人 → 复用同一 invocation 行执行
```

**Agent / 外部 MCP**

```
远程客户端 → /mcp（Streamable HTTP，令牌校验）→ mcp.registry 工具 → 能力调用
页面客户端 → WebMCP（本机 18793）→ lib/webmcp/runtime 工具 → 同源 /api 调用
```

**定时发布**

```
settings.schedule.create → gov_settings_schedules(pending)
  → 后台 RunScheduled：SKIP LOCKED 领取 → 到期重建原部门 Principal → 重查权限/摘要/版本
  → 与完成状态同事务提交（失败保存稳定错误码）
```

## 12. 任务落点速查

| 要改的东西 | 主要文件 |
|---|---|
| 新增业务能力 | `internal/modules/<模块>/capability.go`（契约 + 注册）、service 里实现 Executor、必要时 `db/migrations` + `db/query`、`tests/capability_modules_test.go` |
| 新增会话/系统端点 | `design/*.go` → `make generate` → `internal/transport/*.go` 适配 → `guardedPath` 与 `tests` |
| 改权限/角色/范围 | `internal/identity/{catalog,policy,governance}.go`、`authorization/*`、`dataaccess/scope.go`、`tests/identity_test.go`、前端 `permission-matrix.tsx` |
| 改临时权限 | `internal/identity/temporary.go`、`db/query/temporary_access.sql`、`temporary-access.tsx`、`tests/temporary_access_test.go` |
| 改设置/字典 | `internal/modules/{settings,dictionaries}`、`governance/registry.go`、`gov_*` 迁移、`features/governance/*` |
| 改通知/附件 | `internal/modules/{notifications,files}`、`gov_announcements`/`gov_files`、`notification-*.tsx`、`files.tsx` |
| 改 Agent/触发/工具 | `internal/ai/*`、`internal/platform/{eventhub,outbox}`、`ai/page.tsx`、`tests/ai_platform_test.go` |
| 改远程 MCP | `internal/modules/mcp/*`、`db/query/mcp.sql`、`mcp/remote.tsx`、`scripts/mcp-business-*`、`webskills/baseline-business` |
| 改页面工具 | `features/webmcp/*`、`lib/webmcp/{runtime,routes}.ts`、`e2e/webmcp.spec.ts`、`webskills/baseline-web-*` |
| 加文案 | `i18n/locales/{en,zh-CN}/<namespace>.json`（两侧同步） |
| 加门禁/脚本 | `scripts/*.mjs` + 自身测试、`Makefile`、`docs/agent-policy.json`、`ci.yml` |

## 13. 生成边界与禁区

| 源（可改） | 生成物（禁止手改） |
|---|---|
| `services/api/design/*.go` | `services/api/gen/**` |
| `services/api/db/migrations/*.sql`、`db/query/*.sql` | `services/api/internal/repository/dbgen/**` |
| `services/api/gen/http/openapi3.json`（间接） | `apps/web/src/api/generated/**` |

- 改源之后必须 `make generate`，再 `make check-generated`；生成失败时用正确 Node（`/usr/local/opt/node@22/bin`）重新生成，不手写生成文件。
- `components/ui/*` 是可维护的 shadcn 源码，与生成目录性质不同，不适用"禁改"。
- 契约字段改名、收紧校验、状态码或权限变化属于兼容性变更，必须同步 SDK、前端与测试。

## 14. 产物与本地目录

| 路径 | 性质 |
|---|---|
| `apps/web/dist/` | 生产构建产物（E2E preview 使用）。 |
| `apps/web/.local/`、`test-results/`、`playwright-report/` | 浏览器验收产物、报告、trace。 |
| `.local/webmcp/`、`.local/webmcp-e2e/` | WebMCP 本机服务配对状态（含受限凭证文件，不入库）。 |
| `.tools/` | 项目工具链（Go、sqlc）与模块缓存。 |
| `node_modules/` | 依赖。 |
| `.workbuddy/memory/` | 会话记忆，不参与构建。 |

## 15. 维护约定

1. 目录或生成边界变化必须同步本文件 §2、§13。
2. 新增/删除能力必须同步 §10 与 `docs/agent-tooling.md` 的任务矩阵。
3. 新增页面或路由必须同步 §7.1、`lib/webmcp/routes.ts` 与 E2E 清单。
4. 新增 Skill 或规则目录必须同步 §5 与 `docs/agent-policy.json`，然后跑 `make check-rules`。
5. 本文件描述"现状"，不写计划；未实现的能力标注为未实现，不提前登记。
