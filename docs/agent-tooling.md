# Agent 任务与工具矩阵

这是日常开发入口。目标是让 Agent 根据当前功能选正确层和检查，不要求每次加载全部 Skill，也不为每个 npm 包复制一份手册。长期约束在 AGENTS.md，步骤在项目 Skill，原理与决策在 docs，能机器验证的约束在 lint / tests / CI。

## 根据任务选择

| 任务 | 重点框架/工具与职责 | 使用的 Skill | 读取的目录规则 | 必要验证 |
|---|---|---|---|---|
| 新建/修改完整功能 | 源契约→服务→持久化→SDK→界面；只走实际涉及的层 | baseline-feature-development，再选以下相关项 | 根到所有目标路径的 AGENTS | 按批次累计待验项；阶段验收 ci-fast，按风险追加 SQL/安全/相关 E2E |
| 业务配置、字典选项、附件上传/读取/下载、消息通知 | 复用共享治理服务、注册表、生成 SDK 和受控存储 | baseline-shared-capabilities，再按层加载 web/security/database/Goa | docs/shared-capabilities.md、目标目录规则 | 消费者生效、来源越权、历史引用和文件授权；跨层 ci |
| 页面、路由、共享布局 | React + React Router；复用 shadcn + Tailwind tokens | baseline-web-development | apps/web | ESLint、TS、相关组件测试、build |
| 表单、校验、提交 | React Hook Form + Zod；服务端 Goa 校验；TanStack Query mutation/失效 | baseline-web-development；有新文案加 internationalization | apps/web、涉及 API 时 services/api/design | 无效/有效输入、pending/error/success、相关 HTTP 测试 |
| 列表、表格、筛选 | TanStack Table 9 管表格；Router 管可分享筛选；Query 管远程数据 | baseline-web-development | apps/web | 相关交互、排序/筛选/分页语义；服务端范围测试 |
| 请求、缓存、API 消费 | Orval SDK + TanStack Query，统一 client；不裸 fetch，不重复 DTO | baseline-web-development；契约变更再用 Goa | apps/web、services/api/design | 两端类型/构建、HTTP、check-generated |
| API 字段/参数/错误/安全声明 | Goa DSL 是源，生成 OpenAPI；Orval 消费 3.0 产物 | goa-service-designer | services/api/design | generate、HTTP、check-generated、两端编译/类型 |
| 服务逻辑、传输与配置 | Go 小接口、context、httptest、结构化日志与统一错误 | baseline-feature-development；涉及边界用 security | services/api/internal | Go 单元/HTTP、vet、配置错误分支 |
| 表结构、查询、事务 | Goose 管迁移；sqlc 管类型化查询；pgx 管连接/事务；PostgreSQL 执行约束 | baseline-database-change | services/api/db、internal、tests | generate、integration、迁移/约束/范围/事务；涉及契约再测 SDK |
| 认证、操作授权、数据隔离 | Authenticator/Principal、Casbin、ScopeResolver、Repository/SQL 投影 | baseline-security；SQL 变更加 database-change | services/api/internal、db、tests | test-security，允许+拒绝、跨租户、范围外 ID、撤销 |
| 导出、批量、聚合、下载、后台任务 | 复用相同授权/数据范围；按流量/文件类型明确输入和响应边界 | baseline-security；必要时 Goa/database-change | 服务实现及 SQL 目录 | 新入口的越权/字段泄露测试，不能拿 list 已通过代替 |
| 文案、辅助名称、语言、数值展示 | i18next/react-i18next 管消息；Intl 管日期/数值/货币 | baseline-internationalization | apps/web/src/i18n | i18n:check、lint、相关 locale 测试；存储/切换变更加 E2E |
| 测试失败、交付、CI 变更 | Vitest/RTL、Go testing/httptest、隔离 PostgreSQL、Playwright | baseline-quality-gates | 测试目录或 scripts | 最小失败集→模块集→相应交付 profile |
| 文档、Skill、规则维护 | 本地规则清单、元数据与链接检查；Skill Creator 用于作者校验 | baseline-quality-gates；编写 Skill 时使用可用的 skill-creator | docs、scripts、.agents/skills | check-rules；新脚本的正/负路径；不运行无关浏览器 |

所有路径相对仓库根。可用项目 Skill 及路径由 [agent-policy.json](agent-policy.json) 显式登记，`make check-rules` 检查它们确实存在、元数据有效、路由引用可达，避免表格指向不存在的规则。

表中验证项表示需覆盖的行为与工具，执行时机统一遵循 [测试执行节奏](testing.md#测试执行节奏)：普通功能完成不自动跑测试，阶段集中验证，高风险先定向验证，发布/CI 全量回归。

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

## 页面 Agent 接入

按 [ADR 009](decisions/009-webmcp.md) 在 `/webmcp` 配对本机 `pnpm webmcp`（默认 18793），通过客户端按钮显式安装 MCP 与 WebSkills；当前只支持 Codex / WorkBuddy 一键设置，其他客户端使用相同 stdio 适配器。此用户授权入口是上述“不修改全局 Codex 配置”的明确例外，不在启动/浏览页面时自动安装。webskills 中的导航、账号只读与诊断技能是分发给运行时客户端的资源，不替代 `.agents/skills` 中的工程开发规则。开发新工具使用 feature/web/security/i18n/quality Skill，测试 `pnpm webmcp:test`、Vitest 注册表和 Playwright 真实 MCP 链路；新增业务 API 仍回到 Goa 源契约。

路由清单随新页面维护；global/module/page 的范围和权限标识不替代服务端授权。`make test-webmcp-e2e` 是生成检查之后、完整浏览器集合之前的确定性专项入口；可选模型评测不能替代该测试或真实用户交互。

系统设置的定时发布也属于公共设置能力：复用 settings 的排期 SDK 与后台 worker，发布、版本锁、存储消费者和审计仍用共享事务；具体边界见 [ADR 010](decisions/010-settings-schedules.md)。输入设备时区、到期重新授权及失败恢复按该决策验证，不引入浏览器定时执行器。
