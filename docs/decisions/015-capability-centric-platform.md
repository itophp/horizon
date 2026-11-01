# ADR 015：以业务能力为中心的 AI-Native 平台

日期：2026-10-31。状态：已实现（Phase 1 完成，Phase 2 核心闭环完成）。取代 [ADR 013](013-eino-agent.md) 与 [ADR 014](014-ai-native-platform.md) 中“能力只服务 Agent、按钮仍走业务接口”的形态。

## 背景

ADR 013/014 建立了 Eino 运行时、能力契约与审批，但仍有三处结构性缺口：

1. **按钮与 Agent 不是同一条路径**：页面按钮直接调用 Goa 业务接口，只有 Agent 经过 Gateway，权限、幂等与审计无法一致。
2. **能力成功之后没有可靠事件**：业务变更与事件是两次写，无法驱动“业务成功 → 异步 AI”。
3. **审批与 AI 绑定**：审批属于业务执行的安全闸门，却被放在 AI 服务里，形成第二套入口。

本 ADR 按最新架构文档重构为 **Capability → Event → AI → Capability** 闭环。

## 决策

### 1. 四个抽象 + Event + AI Trigger

| 抽象 | 位置 | 说明 |
|---|---|---|
| Entity | `platform/metadata`、registry 派生 | 由能力目录派生的实体、操作与事件 |
| Capability | `platform/capability/contracts` + `registry` | 名称/版本/Schema/权限/风险/审批/幂等/成功事件 |
| Policy | `platform/policy` | 风险 × 来源 × 权限的确定性判定 |
| Workflow | 未实现（Phase 4） | Eino 检索图保留在 `ai/workflow` |
| Event | `platform/outbox` + `platform/eventhub` | 事务 Outbox、事件日志、进程内分发 |
| AI Trigger Policy | `ai/triggers` + `ai_trigger_policies` | 事件匹配、去重、深度限制、独立任务状态 |

### 2. Gateway 是唯一运行时入口

`POST /api/capabilities/{name}/invocations` 与 Agent Tool 共用 `gateway.Invoke`，固定顺序：

`身份 → 权限 → Schema → 幂等 → 策略/审批 → 执行 → Outbox → 审计`

- 幂等：`(tenant_id, capability, idempotency_key)` 部分唯一索引；重复请求返回存储结果（`replayed=true`），并发失败者短暂等待赢家后重放。
- 审批：high risk 的自动化来源写入 `cap_approvals` 并返回 `awaiting_approval`；人类点击直接执行。审批通过后用 `PrincipalForTask` 重建申请人执行，决策条件更新。
- 审计：`cap_invocations` 记录来源、版本、脱敏输入、digest、状态、耗时、correlation/causation/depth。

### 3. 事务 Outbox 与至少一次事件

新模块写能力实现 `contracts.TxExecutor`，在同一事务写业务数据与 `cap_outbox`；Relay 领取 pending 行（`FOR UPDATE SKIP LOCKED`），写入 `cap_events` 后通知 Event Hub。消费端以 `(tenant_id, event_id)` 去重。

既有模块（字典、通知）无法共享事务，采用执行成功后补写 Outbox 的 best-effort 方式；这是已知限制，不宣称原子。

### 4. 事件驱动 AI

`ai_trigger_policies` 版本化（tenant 特定优先于 `*`），`ai_trigger_executions` 以 `(tenant_id, policy_id, event_id)` 唯一去重。Trigger Engine：

1. 事件深度达到 `maxDepth` 记录 `skipped`；
2. 去重失败直接忽略；
3. 用事件发起账号重建 Principal，异步调用 `harness.RunTrigger`；
4. AI 任务记录 `origin=event`、policy、event id、depth，与业务调用通过 correlation 关联。

Agent 再次调用能力仍走完整 Gateway，因此权限、审批与审计不会因为“来自 AI”而放宽；深度上限阻止自触发循环。

### 5. AI 任务与业务调用分离

- `ai_runs`：AI 任务（用户对话或事件触发），状态独立。
- `cap_invocations`：业务调用，状态独立。
- 运行详情通过 correlation 聚合业务调用；不再维护重复的 `ai_run_steps`。

### 6. 契约、权限与前端

- 新 Goa 服务 `capability`：definitions / invoke / invocations / invocation / approvals / decide / events。
- `ai` 服务收敛为 status / run / runs / runDetail。
- 新权限 `capability:approve`；示例模块新增 `example:read/create/archive`。使用入口仍复用 `assistant:use`。
- `/api/capabilities`、`/api/ai` 并入 `guardedPath`。
- 前端新增 `/capabilities`：示例实体列表、创建/归档按钮（capability SDK + 幂等键）、审批队列、事件与 AI 状态、调用轨迹、能力目录。AI 工作台只保留对话与执行记录。
- 示例模块 `internal/modules/example` 提供查询/创建/归档三类能力与 `example.record.created/archived` 事件；迁移内置一条全局 AI 触发策略，开箱即可看到事件 → AI（无模型时标记 skipped）。

### 7. 删除的旧实现

| 删除 | 原因 |
|---|---|
| `internal/capabilities` | 被 `platform/capability/adapters` 与 `internal/modules/*` 取代 |
| `internal/ai/agent` | 拆分为 `ai/harness`、`ai/tools`、`ai/context`、`ai/workflow` |
| `ai_approvals` | 审批属于 Gateway，改为 `cap_approvals` |
| `ai_run_steps` | 业务调用已由 `cap_invocations` 审计，避免双份轨迹 |
| `/api/ai/capabilities`、`/api/ai/approvals`、`/api/ai/approvals/{id}/decisions` | 迁移到 capability 服务 |
| 前端 AI 页“业务能力”标签 | 迁移到独立 `/capabilities` 页 |

## 兼容与限制

- 旧 `assistant/chat` 页面工具循环保留，属于 AI Assist；其业务动作仍应通过能力（页面工具仅做页面上下文与交互）。
- 既有模块 Outbox 为 best-effort；新模块必须使用 TxExecutor。
- 无消息队列：Event Hub 为进程内分发，重启后 Relay 从数据库继续投递；多实例部署需要共享订阅或改为外部 MQ（尚未实现）。
- Trigger 的深度上限 8、审批 TTL 10 分钟、Outbox 最大重试 5 次均为代码常量，可后续配置化。
- 未实现：Workflow/React Flow、AG-UI 流式、WebMCP 页面工具进 Gateway、Prompt/Model 回归评估、多 Agent 协作、向量检索。

## 验证

- 单元：contracts（Schema/定义/脱敏/digest）、policy（风险 × 来源）、registry（版本/冲突/发现）。
- 集成：`tests/ai_platform_test.go` 覆盖按钮→业务写入→Outbox→事件→Trigger（无模型 skipped）、幂等重放与并发、权限拒绝、跨租户隔离、审批恢复/拒绝/重复决策/跨租户决策。
- 前端：AI 工作台与业务能力页组件测试；浏览器 `e2e/capabilities.spec.ts` 验证按钮→记录→事件→AI 状态→归档，`e2e/ai.spec.ts` 验证未配置模型的可观测降级。

## 2026-10-31 增量：全部业务模块迁移完成

第一版只迁移了示例模块，其他模块仍有各自的 Goa 服务。本增量把所有业务模块并入能力契约，**不再保留旧入口**：

| 模块 | 旧入口 | 现在 |
|---|---|---|
| 设置 | `settings` 服务 12 个方法 | `settings.*` 12 个能力 |
| 字典 | `dictionaries` 服务 11 个方法 | `dictionary.*` 11 个能力 |
| 文件 | `files` 服务 5 个方法 | `file.*` 5 个能力 |
| 通知 | `notifications` 服务 11 个方法 | `notification.*` 11 个能力 |
| MCP 令牌 | `mcp_management` 服务 7 个方法 | `mcp.token.*` / `mcp.capability.list` / `mcp.audit.list` |
| 身份管理 | `identity` 的 12 个 CRUD 方法 | `identity.*` 12 个能力 |
| 技术演示 | `demo` bearer 服务 | 删除 |

结果：

- `design/` 只剩会话（identity 登录/会话/快照/临时权限）、capability、ai、assistant、branding（公开投影）与 system；业务模块没有 REST 端点。
- 领域类型从生成包迁回模块（`internal/modules/*/types.go`），JSON 标签对齐原响应体：必填字段（含 0/false）始终出现，数组始终是 `[]`。
- 前端新增 `apps/web/src/lib/capabilities.ts` 作为唯一业务调用层；页面不再 import 模块 REST 函数。业务类型在 `apps/web/src/api/types.ts`（能力契约不生成逐模块 DTO）。
- 每次调用都是 POST，因此也要求 CSRF；`request()` 对所有 `/api/capabilities/` 请求附带 `X-CSRF-Token`，并对 401 触发会话失效。
- 模块写入仍由领域服务自持事务，事件在提交后写 Outbox（best-effort），在本文“兼容与限制”中保留说明。
- 旧模块集成测试（200+ 端点级用例）删除，替换为 `tests/capability_modules_test.go`：设置草稿→重新认证→发布→事件、文件上传/读取/删除、通知发送→收件箱→已读、字典目录、MCP 令牌生命周期；身份、临时权限、能力网关与租户隔离用例保留。
