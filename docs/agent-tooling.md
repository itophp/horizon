# Agent 任务与工具矩阵

这是日常开发入口。目标是让 Agent 根据当前功能选正确层和检查，不要求每次加载全部 Skill，也不为每个 npm 包复制一份手册。长期约束在 AGENTS.md，步骤在项目 Skill，原理与决策在 docs，能机器验证的约束在 lint / tests / CI。

完整功能开发先按 [Agent 开发工作流](agent-workflow.md) 发现现状、还原状态和角色、按条件集中提问，再记录能力/事务、Eino 选型、Schema 来源、工具暴露与验收。已有答案不重复询问；只有业务、安全或外部暴露含义不清时才问用户，不让用户替 Agent 选择技术实现。流程能力与运行时支持要从代码和测试确认，不能仅根据名称或元数据判断已接入。

所有业务功能先执行 [能力发现与复用](agent-workflow.md#能力复用优先先发现再复用最后新增)，记录需求动作对应的现有能力，优先直接复用、组合或兼容扩展，再决定是否新增；能力存在不代表当前用户、Agent 或 MCP 客户端已获准执行。

## 根据任务选择

| 任务 | 重点框架/工具与职责 | 使用的 Skill | 读取的目录规则 | 必要验证 |
|---|---|---|---|---|
| 功能对应 MCP 工具设计、开发及变更 | 功能用例映射工具契约、复用业务授权 | baseline-mcp-development；按层路由 | docs/mcp-service-plan.md、目标目录规则 | 分阶段协议/安全/客户端互通；规则批次 check-rules |
| 业务能力、Gateway、审批与调用审计 | Capability Registry 出契约，Gateway 统一鉴权/Schema/幂等/审批/Outbox/审计，业务执行仍回领域服务 | baseline-feature-development + baseline-security；契约变更加 goa-service-designer | [ADR 015](decisions/015-capability-centric-platform.md)、目标目录规则 | 契约/策略单测、`tests/capability_modules_test.go` 与 `TestCapability*`（模块闭环、幂等并发、跨租户、审批恢复）、两个语言文案 |
| 能力分层与互调：一个接口拆几个能力、能力调能力 | 数据能力持有业务规则与 SQL；入口能力只做协议投影；互调经 Gateway 并重受策略约束 | baseline-capability-composition | [能力拆分与组合](capability-composition.md)、[ADR 015](decisions/015-capability-centric-platform.md) | 负向用例（内层权限、跨租户、深度、幂等重放）、check-entities |
| 柔性编排：流程编排、循环子图、Agent 自主决策、Schema | Workflow 是唯一主干（节点为能力，依赖由显式映射/DependsOn 确定，Reads/Writes 检查冲突），Graph 只做循环子图，ADK 只做决策；叶子统一经 Gateway | baseline-capability-composition | [柔性编排方案](orchestration-plan.md)、[ADR 016](decisions/016-capability-taxonomy-and-orchestration.md) | check-workflows、check-generated、test-orchestration-example；依赖冲突、审批恢复、循环上限、深度保护 |
| 事件、Outbox、AI Trigger | 事务 Outbox → Relay → Event Hub → Trigger 去重/深度 → AI 任务 | baseline-feature-development + baseline-database-change + baseline-security | ADR 015、`db/AGENTS.md` | Relay/去重/深度集成测试、事件投影与租户隔离 |
| 新建/修改完整功能 | 业务状态/角色→条件澄清→能力/事务/引擎决策→源契约→生成与实现→入口/页面→行为验证 | baseline-feature-development，再选以下相关项 | [开发步骤与问题矩阵](agent-workflow.md)、根到目标路径的 AGENTS | 对照架构决策检查实际入口；按批次累计待验项；阶段验收 ci-fast，按风险追加 SQL/安全；浏览器按现行规则 |
| 业务配置、字典选项、附件上传/读取/下载、消息通知 | 复用共享治理服务、注册表、生成 SDK 和受控存储 | baseline-shared-capabilities，再按层加载 web/security/database/Goa | docs/shared-capabilities.md、目标目录规则 | 消费者生效、来源越权、历史引用和文件授权；跨层 ci-fast 加相关集成/安全 |
| 页面、路由、共享布局 | React + React Router；复用 shadcn + Tailwind tokens | baseline-web-development | apps/web | ESLint、TS、相关组件测试、build；跨层交互在授权范围内跑映射命中的单个 spec |
| 表单、校验、提交 | React Hook Form + Zod；服务端 Goa 校验；TanStack Query mutation/失效 | baseline-web-development；有新文案加 internationalization | apps/web、涉及 API 时 services/api/design | 无效/有效输入、pending/error/success、相关 HTTP 测试 |
| 列表、表格、筛选 | TanStack Table 9 管表格；Router 管可分享筛选；Query 管远程数据 | baseline-web-development | apps/web | 相关交互、排序/筛选/分页语义；服务端范围测试 |
| 请求、缓存、API 消费 | Orval SDK + TanStack Query，统一 client；不裸 fetch，不重复 DTO | baseline-web-development；契约变更再用 Goa | apps/web、services/api/design | 两端类型/构建、HTTP、check-generated |
| API 字段/参数/错误/安全声明 | Goa DSL 是源，生成 OpenAPI；Orval 消费 3.0 产物 | goa-service-designer | services/api/design | generate、HTTP、check-generated、两端编译/类型 |
| 服务逻辑、传输与配置 | Go 小接口、context、httptest、结构化日志与统一错误 | baseline-feature-development；涉及边界用 security | services/api/internal | Go 单元/HTTP、vet、配置错误分支 |
| 表结构、查询、事务 | Goose 管迁移；sqlc 管类型化查询；pgx 管连接/事务；PostgreSQL 执行约束 | baseline-database-change | services/api/db、internal、tests | generate、integration、迁移/约束/范围/事务；涉及契约再测 SDK |
| 实体声明、字段分类、关系与状态流转 | 声明是描述、迁移是事实；`platform/metadata` 解析迁移并做 E1–E10 校验；`make check-entities` 进 ci-fast | baseline-entity-model；改表结构同时加载 database-change | [实体模型](entity-model.md)、services/api/internal、services/api/db | check-entities（含负例）、新实体 i18n label；跨实体能力核对 `Entities` |
| 认证、操作授权、数据隔离 | Authenticator/Principal、Casbin、ScopeResolver、Repository/SQL 投影 | baseline-security；SQL 变更加 database-change | services/api/internal、db、tests | test-security，允许+拒绝、跨租户、范围外 ID、撤销 |
| 导出、批量、聚合、下载、后台任务 | 复用相同授权/数据范围；按流量/文件类型明确输入和响应边界 | baseline-security；必要时 Goa/database-change | 服务实现及 SQL 目录 | 新入口的越权/字段泄露测试，不能拿 list 已通过代替 |
| 文案、辅助名称、语言、数值展示 | i18next/react-i18next 管消息；Intl 管日期/数值/货币 | baseline-internationalization | apps/web/src/i18n | i18n:check、lint、相关 locale 测试；存储/切换变更在授权范围内跑对应 spec |
| 测试失败、交付、CI 变更 | Vitest/RTL、Go testing/httptest、隔离 PostgreSQL、Playwright | baseline-quality-gates | 测试目录或 scripts | 最小失败集→模块集→相应交付 profile |
| 文档、Skill、规则维护 | 本地规则清单、元数据与链接检查；Skill Creator 用于作者校验 | baseline-quality-gates；编写 Skill 时使用可用的 skill-creator | docs、scripts、.agents/skills | check-rules；新脚本的正/负路径；不运行无关浏览器 |

所有路径相对仓库根。可用项目 Skill 及路径由 [agent-policy.json](agent-policy.json) 显式登记，`make check-rules` 检查它们确实存在、元数据有效、路由引用可达，避免表格指向不存在的规则。

表中验证项表示需覆盖的行为与工具，执行时机统一遵循 [测试执行节奏](testing.md#测试执行节奏)：普通功能完成不自动跑测试，阶段集中验证，高风险先定向验证。**E2E 目前暂停**：默认不启动浏览器，需要旁证时只跑改动模块对应的单个 spec，全量集合与默认 `make ci` 必须先取得用户当轮明确授权。

## 避免错误地使用技术栈

- 查询结果和后端实体不要放 Zustand；本地展开/选中/临时草稿不要全部塞 Query。
- 普通表格不要提前引入 AG Grid；简单格式化不要提前引入日期库；不要并行引入多个 i18n 框架。
- 生成 SDK 不等于业务逻辑已实现；Casbin 不等于 SQL 已过滤；前端隐藏按钮不等于后端已授权；前端 Zod 不等于服务端已验证。
- shadcn 是可维护的组件源码；Goa/sqlc/Orval 输出是不可手改的生成代码，两者不能混淆。
- MCP/Registry 可辅助检索和交互，但本地生成、测试和 CI 必须独立可执行。不要为了“AI 适配”新增没有实际用途的 MCP、依赖或 Skill。

## 规则如何被发现

OpenAI 官方说明：AGENTS 按根到当前目录组合；从根启动不等于所有子目录规则已载入，所以根规则要求改文件前主动读取沿途规则。[AGENTS.md 官方说明](https://learn.chatgpt.com/docs/agent-configuration/agents-md)。

项目 Skill 放在 `.agents/skills`，元数据用于发现，详细内容按触发加载；也可显式 `$baseline-feature-development`。新增后通常可自动发现；若列表未刷新，重新打开会话。[Skills 官方说明](https://learn.chatgpt.com/docs/build-skills)。

项目不修改用户全局 Codex 配置。换用其他编码 Agent 时，确认其支持的规则/Skill 加载机制；不假定 Markdown 文件名就能自动生效。`check-rules` 验证仓库结构，不能证明某次会话实际加载了哪些指令；会话应说明读取来源。

## 身份与后台模块入口

登录、账号、组织、角色或部门功能从 features/administration、design/identity.go、internal/identity 进入；模块 Repository 仍只调用 sqlc dbgen。身份会话、Casbin、可信 scope 与参数化 SQL 是四个独立职责，即使组合在同一模块也不能省略。旧 demo 认证仅用于技术示例。改动合并优先级、六类数据范围、角色授权管理或会话策略时，先读 [身份决策](decisions/002-identity-access.md)，补 tests/identity_test.go 的允许与拒绝用例；关键 UI 流程补 e2e/administration.spec.ts。

修改权限目录、临时申请/审批或授权有效期时，使用 baseline-security 与 baseline-database-change，先读 [ADR 003](decisions/003-temporary-access.md)，联动 identity/catalog.go、会话授权解析、Goa/SDK、权限矩阵和临时权限页。审批上限只计算正式授权，验证范围固定、到期/撤销、deny、自批/再委托拒绝、跨租户及并发审批。


多部门账号、部门角色或上下文切换变更先读 [ADR 004](decisions/004-department-memberships.md)，加载 baseline-security + database-change + web-development。修改权限控制时必须同步相关 Skill、目录规则和安全/决策文档，并报告维护位置及真实验证结果。

设置/字典/通知/附件变更使用 baseline-feature-development、baseline-security，再按涉及层选择 Goa/database-change/web-development/internationalization/quality-gates；先读 [ADR 005](decisions/005-system-governance.md)。注册配置必须声明消费者和允许层级，新增动态字典来源复用原资源范围，驱动通过 files.Registry 扩展。触及发布、敏感值、存储版本或投递队列时，联动正式权限、事务、生成 SDK 和负向集成测试。禁止在前端绕过发布、另存服务器状态或复制权限推导。

公共设置与字典不再关联企业/公司/部门配置范围，最新决策见 [ADR 007](decisions/007-global-governance.md)；本人偏好、文件和动态来源的数据权限不能因此合并或放宽。

## 能力中心接入

新增业务动作不要写成 Agent 专用 tool，也不要让页面直接调业务接口。步骤：在 `platform/capability/registry` 的目录（`adapters` 或 `internal/modules/*`）注册契约（名称、版本、输入/输出 Schema、权限、风险、审批、幂等、成功事件、可审计字段）→ 实现 Executor（写能力用 `TxExecutor`，业务数据与 `cap_outbox` 同事务）→ 需要时用生成的 capability SDK 从页面调用。风险按 [ADR 015](decisions/015-capability-centric-platform.md)：人类点击可执行 high；Agent/Workflow/Event/API 的 high 必须确认；critical 对自动化关闭。事件必须带 correlation/causation/depth，Trigger 去重并限制 maxDepth。修改 Gateway、审批或审计范围时，同步 `internal/AGENTS.md`、本文件与 `tests/ai_platform_test.go` 的允许/拒绝用例。

一个功能拆几个能力、能力之间能否互调，按 [能力拆分与组合](capability-composition.md)（Skill：baseline-capability-composition）：数据能力持业务规则与 SQL，入口能力只做协议投影，两者都注册为能力；互调必须走 `gateway.Invoke`（内层 `OriginCapability`、深度 +1、写能力确定性幂等键），`TxExecutor` 内禁止再调 Gateway。禁止 handler/service 直连 `dbgen`/`pool`，也禁止能力自己 `registry.Resolve` 调 handler。

## 页面 Agent 接入

按 [ADR 009](decisions/009-webmcp.md) 在 `/mcp` 的 WebMCP 标签连接本机 `pnpm webmcp`（默认 18793，开发服务器自动把端口与配对密钥预填进表单），通过客户端按钮显式安装 MCP 与 WebSkills；当前只支持 Codex / WorkBuddy 一键设置，其他客户端使用相同 stdio 适配器。此用户授权入口是上述“不修改全局 Codex 配置”的明确例外，不在启动/浏览页面时自动安装。webskills 中的导航、账号只读与诊断技能是分发给运行时客户端的资源，不替代 `.agents/skills` 中的工程开发规则。开发新工具使用 feature/web/security/i18n/quality Skill，测试 `pnpm webmcp:test`、Vitest 注册表和 Playwright 真实 MCP 链路；新增业务 API 仍回到 Goa 源契约。

路由清单随新页面维护；global/module/page 的范围和权限标识不替代服务端授权。`make test-webmcp-e2e` 是生成检查之后、完整浏览器集合之前的确定性专项入口；可选模型评测不能替代该测试或真实用户交互。

系统设置的定时发布也属于公共设置能力：复用 settings 的排期 SDK 与后台 worker，发布、版本锁、存储消费者和审计仍用共享事务；具体边界见 [ADR 010](decisions/010-settings-schedules.md)。输入设备时区、到期重新授权及失败恢复按该决策验证，不引入浏览器定时执行器。

远程业务 MCP 见 [实现方案](mcp-service-plan.md)（待实现），功能批次主动确认暴露范围；个人 Token 不复用页面配对密钥。

账号远程业务 MCP 的实现边界与接入见 [ADR 011](decisions/011-remote-mcp.md) 和 [操作指南](mcp-operations.md)。功能工具在 internal/modules/mcp/registry.go 注册，业务输入映射到能力契约；工程 Skill 与 webskills/baseline-business 的客户端使用说明分开。验证使用 test-mcp、test-mcp-e2e，按批次执行。

流程规格与未来生成器的边界见 [Schema 路线](agent-generation-plan.md)；先使用 [完整生命周期示例](orchestration-example.md) 和生成的结构契约，不把自然语言规则或 Schema 校验当成业务安全证明。
