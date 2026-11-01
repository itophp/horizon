# ADR 014：AI-Native 平台底座（Capability Registry / Tool Gateway / Run Trace）

> **已被 [ADR 015](015-capability-centric-platform.md) 取代。** 本文描述的能力注册表、Gateway 与审批已迁移为“Capability → Event → AI → Capability”闭环；文中 `internal/capabilities`、`internal/ai/agent` 等路径已删除，仅作历史记录。

日期：2026-10-31。状态：已实现（第一阶段与第二阶段核心）。取代 [ADR 013](013-eino-agent.md) 中「智能体只有一个知识库工具」的形态，保留其模型配置、知识库与页面内助手结论。

## 背景

ADR 012/013 解决了「有聊天入口、有 Eino 运行时、有知识库」，但离 AI-Native 还差三件事：

1. **业务动作只存在于页面和服务内部**：模型能查知识，却不能查收件箱、读字典、发通知；如果照旧为每个动作写一个 tool，权限、风险、审计会在多处重复。
2. **没有统一契约**：能力没有名称、版本、输入 Schema、权限、风险等级和幂等声明，UI、Agent、未来的 Workflow 无法共用同一份定义。
3. **没有执行痕迹与人工闸门**：模型调用了什么、参数是否敏感、失败原因、是否需要人工确认，都没有可追溯记录。

因此按《AI-Native 企业信息化应用框架》第 4、5、6、11、12 节重构：以 Entity / Capability / Policy / Workflow 四个抽象为长期边界，本次落地前三个，并把 Workflow 留给第三阶段。

## 决策

### 1. 四个抽象落在代码中的位置

| 抽象 | 位置 | 说明 |
|---|---|---|
| Entity | `internal/platform/capability`（`Entity`、`Registry.Entities`） | 由能力目录派生的实体清单，用于页面上下文校验与展示 |
| Capability | `internal/platform/capability`（定义/校验/结果信封）+ `internal/capabilities`（业务目录与执行器） | 平台只管契约与策略，业务执行复用既有领域服务 |
| Policy | `capability.Admit` + `identity.Principal`（Casbin、可信范围）+ `harness` 审批表 | 权限、风险等级、审批策略、幂等键 |
| Workflow | 未实现（第三阶段） | 现有 Eino Graph 检索链路保留，作为 Workflow 节点的雏形 |

目录同时按框架第 13 节收敛：

```text
services/api/internal/
├── platform/
│   ├── capability/     # 注册中心：契约、JSON Schema 校验、风险/审批策略、结果信封
│   └── knowledge/      # 知识引擎：切分、索引、租户内检索（从 internal/agent 移出）
├── capabilities/       # Go 业务能力层：目录 + 执行器（复用 identity/dictionaries/notifications）
├── ai/
│   ├── agent/          # Eino ADK 运行时 + Tool Gateway（原 internal/agent）
│   ├── assistant/      # 页面内助手网关（原 internal/assistant）
│   └── harness/        # 运行记录、步骤、审批队列、上下文引擎
```

### 2. Capability Registry 是唯一业务动作定义

`capability.Definition` 固定包含：`Name`、`Version`、`Domain`、`Entity`、`Description`、`Permission`、`Risk`、`Approval`、`Idempotent`、`ReadOnly`、`Executor`、`Input`（受限 JSON Schema）、`Loggable`（可落审计的字段白名单）。

- 注册时校验：名称+版本唯一、Schema 根为 object、风险/审批合法、执行器存在、扁平化后的工具名不冲突。冲突即整体构造失败，绝不带病运行。
- 发现时按调用者权限过滤；`ApprovalForbidden` 的能力对任何入口都不出现。
- 输入 Schema 由平台自己校验（required / 类型 / enum / 长度 / 数值边界 / 数组长度 / 拒绝未声明字段），同一份文档同时用于 UI 展示和 Eino 工具声明，避免「声明一套、校验另一套」。
- 工具名把 `notification.send` 扁平化为 `notification_send`（Chat Completion 工具名不允许点号），注册表保留反向映射。

首批能力（全部复用既有服务，无重复业务逻辑）：

| 能力 | 权限 | 风险 | 审批 | 幂等 | 执行器 |
|---|---|---|---|---|---|
| `knowledge.search` | `assistant:use` | low | auto | 是 | `knowledge.Store.Search` |
| `dictionary.options` | `dictionaries:use` | low | auto | 是 | `dictionaries.Service.Options` |
| `notification.inbox` | 登录即可 | low | auto | 是 | `notifications.Service.InboxPage` |
| `notification.send` | `notifications:send` | high | confirm | 是（确定性 UUID clientKey） | `notifications.Service.Send` |

### 3. Tool Gateway 是唯一执行入口

`internal/ai/agent/tools.go` 为每次运行构造一个网关：把当前 Principal 解析成 `capability.Caller`，按权限发现能力，逐个包装成 Eino Tool。每次调用按固定顺序处理：

1. 解析参数 → Schema 校验（不合法直接记为失败步骤，不进入业务层）。
2. 策略判定 `Admit(caller, actor=agent)`：缺权限即 `forbidden`；critical 对 Agent 一律拒绝；high 必须落审批。
3. 需要审批时：写 `pending_approval` 步骤 + `ai_approvals` 卡片（绑定参数摘要与 10 分钟有效期），返回 `approval_required`，**不执行**。
4. 否则带 60s 超时执行，结果统一为 `{ok, capability, data, summary, errorCode, stepId}` 信封；超过 12KB 的结果降级为 `result_too_large`，避免吞掉上下文。
5. 无论成功、失败、拒绝、待审批，都写入一条 `ai_run_steps`；参数只保留 `Loggable` 字段并截断，其余以摘要代替。

业务失败以数据形式回给模型（而非 Go error），让 Agent 能解释、换路或请求人工，而不是整轮 500。

### 4. 审批与「不放大权限」

- 高风险动作暂停，`ai_approvals` 保存**完整**审查载荷（如通知正文）以便执行；**API 只投影白名单字段**，正文永不返回。
- 审批通过后，用 `identity.PrincipalForTask(tenant, 申请人, 部门)` 重新解析申请人身份，并在该上下文执行——审批人不会把自己的更大范围「借」给申请人。
- 决策是条件更新（`status='pending' AND expires_at > now()`），并发只有一个赢家；过期卡不可批准；跨租户不可见（404）。
- `four-eyes` 能力禁止申请人自批；本次目录中没有 critical 能力，但策略与测试已就位。
- 决定后同步关闭挂起的工具步骤，避免 trace 永久停在 `pending_approval`。

### 5. 运行记录与上下文

- `ai_runs`：租户、账号、当前部门、会话、Agent、模型、指令版本、问题、回答、状态、错误码、工具次数、token、耗时、Trace ID、脱敏页面上下文；`ai_run_steps` 逐次记录；`ai_approvals` 记录人审。三者都带 `tenant_id`，查询强制租户条件，运行详情额外强制账号条件（跨账号 404）。
- 上下文引擎（`harness.Context`）只注入：账号显示名、用户名、角色、当前部门、有效权限码、当前页面与实体、已知实体列表。值被压成单行并截断，明确标注「是数据不是指令」；页面路由用正则限制、实体必须在目录内，防止把任意文本塞进提示词。
- 页面内助手（`/api/assistant/agent`）复用同一 `Run`，因此同样有 trace 与审批；助理模式只是没有显式页面上下文。

### 6. 契约与前端

- 新 Goa 服务 `ai`：`status`、`capabilities`、`run`、`runs`、`runDetail`、`approvals`、`decide`，全部会话鉴权；`/api/ai/` 并入 `guardedPath`，自动获得 Principal、Origin 与 CSRF 校验。
- 新增权限 `ai:approve`（审批他人申请）。使用入口仍复用 `assistant:use`，避免既有授权失效。
- 前端 `features/ai`：AI 工作台（对话 / 步骤轨迹 / 审批卡片 / 待确认队列）、业务能力（可按风险与关键词筛选、查看输入契约）、执行记录（列表 + 详情）。侧栏新增一级入口「AI 工作台」，路由 `/ai?tab=chat|capabilities|runs`，并同步 WebMCP 路由清单。
- 所有文案进 `ai` 命名空间（en / zh-CN），能力标题按 `capability.<name 点号换下划线>` 取译，未翻译能力回退服务端描述。

## 与框架的差距（后续阶段）

| 框架条目 | 状态 | 说明 |
|---|---|---|
| Capability Registry / Tool Gateway | 已实现 | 覆盖 UI、Agent、审批重放；外部 MCP 仍走自身工具集，未接入注册表 |
| Policy（RBAC/风险/审批/幂等） | 已实现（RBAC 复用 Casbin，ABAC 未单列） | 数据范围仍由各领域服务解析 |
| Harness（运行记录/审批/上下文） | 已实现 | 取消、重试、Checkpoint 未实现 |
| Eino ADK 运行时 | 已实现 | 单 Agent，无多 Agent 协作 |
| AG-UI / 流式 | 未实现 | 目前同步返回，改 SSE 需要新契约 |
| Workflow Engine + React Flow | 未实现 | 第三阶段；现有检索 Graph 保留 |
| Knowledge（RAG/pgvector） | 部分 | 仍为 pg_trgm 词法检索 |
| Evaluation / Prompt 回归 | 未实现 | 已有 trace 与 token 数据，缺评估基线 |
| Entity 完整元数据（字段/关系/检索方式） | 部分 | 只有名称、领域、标识符，字段级元数据待建 |
| WebMCP 页面工具进网关 | 未实现 | 页面工具仍在浏览器执行，经助手通道 |

## 影响与兼容

- `internal/agent`、`internal/assistant` 移动到 `internal/ai/*`，`internal/agent/knowledge.go` 移到 `internal/platform/knowledge`；仅传输层与测试引用，无外部契约变化。
- 移除 `knowledge_search` 专用工具，改由 `knowledge.search` 能力提供；模型侧工具名从 `knowledge_search` 变为 `knowledge_search`（扁平化后同名，兼容）。
- `assistant/agent` 行为变化：回答现在同样受能力策略约束，且会写运行记录；未配置模型时仍为 503。
- 测试夹具的 `search_path` 从「仅测试 schema」改为「测试 schema,public」，否则 00013 迁移的 `pg_trgm` 操作符类在隔离 schema 下不可解析。

## 关键代码位置

| 职责 | 位置 |
|---|---|
| 能力契约、Schema、策略、信封 | `services/api/internal/platform/capability/` |
| 业务能力目录与执行器 | `services/api/internal/capabilities/` |
| 运行记录、审批、上下文 | `services/api/internal/ai/harness/` |
| Agent 运行时与 Tool Gateway | `services/api/internal/ai/agent/agent.go`、`tools.go` |
| 知识引擎 | `services/api/internal/platform/knowledge/` |
| 契约 | `services/api/design/ai.go` |
| 表与查询 | `services/api/db/migrations/00014_ai_platform.sql`、`db/query/ai.sql` |
| 传输装配 | `services/api/internal/transport/ai.go`、`governance.go`、`identity.go` |
| 前端 | `apps/web/src/features/ai/` |
| 测试 | `internal/platform/capability/*_test.go`、`internal/ai/harness/*_test.go`、`internal/capabilities/*_test.go`、`apps/web/src/features/ai/page.test.tsx`、`services/api/tests/ai_platform_test.go` |
