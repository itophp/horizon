# Capability 柔性编排实施记录

需求来源：[Capability 柔性编排功能方案](Capability%20柔性编排功能方案.md)。

## 当前交付范围

最新增量为 Phase 4 管理工作台：执行方案版本、发布/绑定/回滚、注册实现搜索、编译图、结构预检、版本差异和本人节点轨迹。复用已有 Gateway、Workflow、Eino Graph 和 X6 Viewer；在线 DSL 保存、业务模拟、灰度和更多作用域尚未实现。各阶段细节与验证边界见文末。

2026-11-06：已补齐 Phase 1 的 Direct 方案持久化、租户绑定、管理能力和全状态执行身份记录，并提前实现 Phase 2 的 Direct 版本校验、发布和绑定回滚部分。**以下保留初始 Direct 阶段记录；Workflow/Graph 后续增量见文末，管理页面仍待 Phase 4。**

### 已实现

- `contracts.ExecutionPlan` 与 `ExecutionBinding` 分离能力版本、方案版本与绑定修订。Registry 为每个能力版本派生 Direct V1 平台方案；服务装配时持久化全部已注册方案，Gateway 为独立装配场景按需确保默认方案存在。
- 追加迁移 `00029_execution_plans.sql`：方案版本、租户绑定、绑定历史与执行记录。平台方案用独立的空租户部署空间，租户覆盖只属于可信 Principal 的租户。未覆盖时使用平台 Direct 方案。
- 方案定义不可变；再次创建同一 Plan ID 生成下一个草稿版本，不能将同一 Plan ID 转移给另一个能力/契约版本。生命周期为 draft → validated → published。发布不自动切换绑定。
- 发布校验注册能力、契约摘要、原生实现引用与支持模式；当前只允许 Direct。已发布方案由数据库触发器禁止原地修改或删除。
- 绑定切换/回滚使用 `expectedRevision` 乐观并发控制。绑定指针、操作者、时间、原因、历史和成功 Outbox 在同一事务提交。无效版本、草稿、错误能力、旧修订和跨租户操作均拒绝；回滚通过选择已发布版本完成。
- Gateway 在新调用及审批受理时固定绑定身份。创建调用和执行记录使用同一 SQL 语句提交；失败、取消及等待审批也保留身份。审批恢复重新授权原申请人并读取原固定方案，后续绑定切换不改变它。
- 存量调用在迁移中回填 Direct V1 身份；迁移 Down/Up 往返后仍可回填，不改业务数据。原生 Handler、事务和 Outbox 执行路径保留。
- 执行记录复用 cap_invocations 的状态、时间、trace、causation 与错误码，不保存业务参数或结果副本。查询仅本人、当前租户。
- 权限目录及权限矩阵补齐中英文标签；未增加依赖、业务 REST 路由或第二套 Registry/Workflow 引擎。

### 管理入口

全部使用既有 `/api/capabilities/{name}/invocations` 与通用 Gateway SDK。读操作也通过该入口，写操作必须携带幂等键。管理动作使用 manual-only，不进入 Agent/MCP 自动化目录。`executionPlans:*` 均为独立、不可临时授予的正式权限；管理权限还必须与目标能力权限同时满足。

| Capability | 权限 | 主要输入 |
|---|---|---|
| execution.plan.list | executionPlans:read | capability、capabilityVersion |
| execution.plan.binding | executionPlans:read | capability、capabilityVersion |
| execution.plan.history | executionPlans:read | capability、capabilityVersion |
| execution.plan.records | executionPlans:records | 空对象；返回本人最近 100 条 |
| execution.plan.create | executionPlans:edit | capability、capabilityVersion、planId、name |
| execution.plan.validate | executionPlans:validate | capability、capabilityVersion、planId、planVersion |
| execution.plan.publish | executionPlans:publish | 同上 |
| execution.plan.bind | executionPlans:bind | 同上 + expectedRevision、reason |
| execution.plan.rollback | executionPlans:rollback | 同上 + expectedRevision、reason |

`planId` 自定义名称限小写字母开头及小写字母/数字/点/下划线/连字符，最多 120 字符。版本由服务端生成；输入不接受 tenant、scope、Handler 引用或 Schema。创建的是原生 Direct 的新草稿版本，尚不能输入 Workflow DSL。

操作顺序：list/binding → create → validate → publish → bind。第一次租户覆盖的 expectedRevision 为 0；以后必须使用 binding 查询返回的 revision。回到平台 Direct 时，rollback 选择 list 中平台方案的 planId 与 V1。操作原因不进入调用参数审计白名单或事件。

方案列表最多 200 条，绑定历史最多 100 条；分页与详情界面留待管理页面阶段。当前不提供归档入口。

## 复用与下一阶段

| 需求 | 已有实现及处理 |
|---|---|
| 能力目录与契约 | 继续使用 kernel/capability/registry、contracts |
| 鉴权、幂等、事务、Outbox | 继续使用 Gateway；管理原子写采用 TxExecutor |
| 持久化流程、分支、恢复 | 后续适配 kernel/workflow 的现有 Engine/spec |
| Graph/Agent | 后续复用现有 Eino 编译与 Agent 集成，不另建引擎 |
| 管理 API | system/executionplans.Entries；runtime/app 统一装配 |

现有 Workflow StartTx 返回异步受理状态，原能力可能承诺同步业务输出。Workflow Adapter 必须明确受理与最终结果的契约，不能将受理结果冒充业务结果，也不能在原业务事务中调度 Workflow。invoke_native 需要在完整 Gateway 检查内选择原生实现，不能在 Workflow 直接调用注册 Handler。

Phase 2 的注册引用适配与 Native/Pinned 节点、Phase 3 的 Graph Ref/Adapter、工具权限与预算已在后续增量实现，见文末。仍未完成：Phase 4 的 Capability 详情执行方案入口、在线编辑/测试/调试与版本差异。灰度和更多作用域按方案作为后续增强。

## 验证

- `make generate`、`make ci-fast`：通过，含生成一致性、实体、Workflow、前后端构建与单元测试。已有 React Compiler/Hook 两条 lint 警告及构建体积提示保留。
- `TestExecutionPlanLifecycleAndIsolation`：真实 PostgreSQL，通过草稿/校验/发布/绑定/回滚、幂等重放、发布不可变、租户/账号隔离、权限分离、自动化目录排除、待审批固定方案、失败记录、并发绑定和事务 Outbox 断言。
- `TestExecutionPlanMigrationPinsLegacyInvocations`：真实 PostgreSQL，通过 Down/Up、重复 Up 与存量调用回填。
- 旧审批恢复测试的手工 SQL 夹具补齐固定方案记录，原断言保留；修复后定向回归通过。
- 实体枚举漂移负例通过，错误枚举确实被模型检查拒绝。
- `make test-security`：最终重跑通过，包含安全单元测试及全套真实 PostgreSQL 集成测试（含新增方案和迁移用例）。
- `make check-rules`、`git diff --check`：通过。
- 部署前需先应用新增迁移 00029；本次只在隔离测试 schema 中执行迁移，未修改运行中业务数据库。
- E2E 未执行（按当前指示暂停）。没有推送、远程 CI 或真实 MCP 调试；本会话未连接项目运行时 MCP。

## Phase 2：注册 Workflow 方案适配

本阶段接入现有 `kernel/workflow` 的 Eino Workflow、分支、持久化检查点、原生审批中断与恢复；没有新增引擎或调度器。

- `execution.plan.create` 新增可选 `mode`（direct/workflow）与 `workflowRef`。Workflow 方案只能引用代码中已注册的 `名称@版本`，保存注册定义摘要；校验、发布、绑定及执行前均检查引用与摘要。草稿/版本/绑定管理仍走原有九个管理 Capability。
- 校验要求目标为非敏感临时结果的原子能力，Workflow 输入 Schema 与原能力一致。结果节点必须以 `invoke_native` 调用原能力；其他节点必须在结果节点之前。支持一个无条件 Native 结果，或同一个确定性分支组中的互斥 Native 结果，运行时必须恰好选中一个。最终输出从该 Native 调用的已持久化成功结果读取、按原 Schema 验证及按当前字段权限投影，不能由模型或适配器伪造成功。
- 节点支持 `resolve_active_plan`（省略时的默认值）、`invoke_native`、`invoke_pinned_plan`（需 planId/planVersion）。Native 不是直接调用 Handler：必须证明当前可信任务 runId 与原始调用绑定，且目标是该方案所属能力，再走完整 Gateway 身份/权限/来源/Schema/幂等/风险/事务/Outbox 路径。其他能力经 Gateway 选择其当前或固定方案。
- Workflow 入队只写基础设施状态。原调用、固定方案记录、运行头、加密 manifest 和 runId 关联在同一事务提交；业务节点在提交后由既有 Tick 执行。失败入队不留下新受理记录；带幂等键的重试复用同一运行。
- 同一 Capability ID 调用 Workflow 方案后首先返回 `status=executing`，不返回伪装成业务结果的 run receipt。现有 invocation 查询及执行记录可追踪；执行记录暴露本人的 runId，支持原 Workflow 查询/取消能力。完成后原 invocation 写入原业务输出与 Native 事件 ID，不重复产生业务事件。失败/取消同步关闭原 invocation，条件更新不覆盖已经成功的业务结果。
- Native 的自动化高风险操作继续触发现有 Gateway 确认。绑定回滚后审批恢复执行受理时的 Direct Native 快照；重建 Eino 引擎后仍恢复原 run。已有业务人工审核规则仍由原能力负责，参考流程中的确认是风险确认，不冒充主管业务审核。
- 原有节点数、深度、只读重试/循环预算、每次执行 60 秒上下文及审批 TTL 继续生效。静态校验拒绝自身 Active Plan 递归，并沿已绑定/固定 Workflow 引用检查可检测循环；租户绑定切换使用同一事务锁串行校验，运行时仍受 Gateway 深度限制。
- 模板禁止 Agent、补偿、敏感临时结果及人工/内部/系统专用能力，避免扩大工具范围或在检查点复制敏感结果。通用 Workflow 既有 Agent/补偿功能不受此限制；Graph/AI Plan 适配留给 Phase 3。

可运行参考：`example-archive-plan@1.0.0`，用于替换 `example.record.archive@1.0.0`，先调用已有 example.record.get，再通过 Native 节点归档。该模板不作为普通 Workflow 启动项展示，须通过执行方案绑定使用。

示例创建输入：

```json
{
  "capability": "example.record.archive",
  "capabilityVersion": "1.0.0",
  "planId": "archive-with-check",
  "name": "Archive with check",
  "mode": "workflow",
  "workflowRef": "example-archive-plan@1.0.0"
}
```

之后依次 validate → publish → bind；回滚选择平台 Direct V1，操作字段沿用上表。数据库需先应用迁移 `00030_workflow_execution_plans.sql`；运行中使用的注册流程版本必须保留，引用缺失或摘要漂移时拒绝执行，不能自动换用新定义。

限制与后续：本阶段保存的是注册 Workflow 引用和摘要，未开放数据库 DSL 上传/在线编辑，未提供业务专用主管审核页面。Graph Adapter 已在下述 Phase 3 实现；Phase 4 的编辑/测试/调试界面与灰度仍待后续实现。部署迁移本次仅在隔离测试 schema 内执行。

验证：真实 PostgreSQL + 真实 Eino 覆盖条件分支、Workflow 发布绑定、Native 风险审批、等待期间回滚、引擎重建恢复、固定版本选择、取消、前置节点失败、事件不重复和结果事件 ID 保持；Direct 生命周期及存量迁移用例同时回归。`make generate`、`make ci-fast`、`make test-security`（含全套真实数据库集成回归）、受影响包单元测试均通过。`git diff --check` 通过；`make check-rules` 通过（新增约束详细说明放在安全文档，目录规则保持在指令链大小限制内）。E2E 未执行（按当前指示暂停）。

## Phase 3：Eino Graph Adapter

本阶段已接入真实 Eino Graph，继续复用 Workflow 的持久化队列、加密检查点及 Gateway，不建立第二套引擎。没有新增依赖或迁移。

### 注册与调用

- `app.GraphDefinitions()` 是运行时与离线检查共享的 Graph 引用目录；`NewWithGraphs` 冻结部署声明。Graph 声明名称/版本、输入输出 Schema、固定只读能力及字段映射、模型指令和预算。Workflow 的 `Step.Graph` 固定 `{ref,hash}`，引用缺失、摘要漂移、无效预算、写工具或冲突执行器均拒绝。
- Graph 使用固定读取节点 → 模型节点 → 有界结构纠错分支。模型不选择工具，也不执行代码；模型提出任何 ToolCall 都拒绝。需要自主工具决策时仍应使用现有 ADK，不能把这个适配器称为 Agent 引擎。
- 工具仅允许低风险、无审批、非敏感临时结果、无字段权限投影的原子只读能力，排除人工/内部/系统专用能力。工具逐次经完整 Gateway 调用固定 Direct V1，防止活动 Workflow 方案间接引入写入；运行前重新检查当前权限，SQL 数据范围仍由原能力强制执行。Graph 的读取来源纳入原 Workflow 的插件激活依赖。
- Workflow 节点可消费 Graph 输出字段并作确定性分支；结构无效、模型不可用、读取失败及超时均失败，不默认放行。即使输出 safe，Native 的高风险确认仍由 Gateway 决定。

可运行引用：`example-risk@1.0.0`。可绑定的 Workflow 模板：`example-archive-graph-plan@1.0.0`，先分析，再按 safe/review 选择互斥 Native 节点，最终返回原归档结果。

### 独立 Graph Plan

`execution.plan.create` 还支持 `mode=graph` 和 `graphRef`。仅允许替换非 TransientResult 的只读能力，Graph 与能力的输入/输出 Schema 必须完全一致；写能力必须通过 Workflow 组合，不能直接替换为 Graph。校验/发布/绑定/回滚权限及版本机制不变。

新增参考能力 `example.record.assess@1.0.0`，权限复用 example:read。默认 Direct 通过 Gateway 读取本人示例记录并返回保守的 review 建议，不调用模型；绑定 `example-risk@1.0.0` 后使用 Graph，回滚后恢复 Direct。该能力不改变记录状态。示例创建输入：

```json
{
  "capability": "example.record.assess",
  "capabilityVersion": "1.0.0",
  "planId": "record-risk",
  "name": "Example risk analysis",
  "mode": "graph",
  "graphRef": "example-risk@1.0.0"
}
```

独立 Graph 也先返回 executing，并用内部 Workflow 包装复用原子受理、恢复和取消机制；最终仅返回原只读契约的结构化结果，不产生业务事件。内部包装不作为额外业务流程暴露。既有 workflow_hash 列复用为 Eino 引用摘要，Graph 对外记录使用 graphHash；已有 Workflow 数据兼容。

### 预算与验证

每个 Graph 限制模型调用次数（最多 8）、固定工具调用次数（最多 16）、单次请求输出 token（最多 2048）、上下文 UTF-8 字节（最多 32768）和绝对超时（最多 30 秒）。预算在外部调用前写入加密检查点，崩溃后不重新计数；已完成 Graph 的恢复不重复调用模型。成本限制是请求次数、请求上下文及输出上限，不是币种费用计量；供应商实际计费未接入。

测试采用真实 PostgreSQL、真实 Eino Graph/Workflow 与确定性模型，覆盖 safe/review 分支、严格 Schema、有界纠错、模型失败、超时、伪造写工具拒绝、权限不足、读取失败、预算跨重启保持、完成结果复用，以及独立 Direct → Graph → Direct 回滚。模型请求的输出 token 参数也有断言。真实模型服务的回答质量与供应商行为未在本次验收。

尚未完成 Phase 4 的执行方案管理界面、在线 DSL 编辑、模拟/节点调试、版本差异与灰度。E2E 未执行（按当前指示暂停）；未推送或触发远程 CI。

Phase 3 最终验证：`make generate`、`make ci-fast`、`make test-security`（含完整隔离 PostgreSQL 集成回归）、Graph/Workflow/Gateway 定向测试与 `git diff --check` 均通过。旧生命周期模型桩改为显式选择原先要求的读取工具，保留原业务断言，避免依赖能力目录排序。此前失败已修复并重跑通过。没有新增迁移，部署仍需前两阶段的 00029/00030；实际运行数据库未在本次修改。


## Phase 4：执行方案管理工作台

入口为业务能力目录中的“执行方案”，路由 `/capabilities/:name?version=...`。不新增一级菜单。该页面属于平台专用管理（B2/L1），需要展示编译图、不可变版本、绑定并发及调试数据，因此保留专用 React 页面；复用现有 PageHeading、DataTable、RowActions、EditorDialog、shadcn、TanStack Query、React Hook Form + Zod、React Router 和 WorkflowViewer，不另建 Runtime 页面引擎。

### 已实现

- 查看平台默认和租户方案、当前绑定及修订，复制已发布方案生成新草稿。创建、校验、发布、绑定为独立操作；发布不自动生效。绑定/回滚弹窗显示并固化打开时的 revision，冲突不自动改为新 revision 重试；同一提交重试复用幂等键。
- Workflow/Graph 实现由服务端按目标契约、当前节点权限和有效引用过滤，支持远程搜索并显示名称/版本。写能力不提供独立 Graph 模式。输入参数不接受任意代码、处理器、模型工具或租户。
- 查看输入/输出契约、编译图及 Graph 预算，复用已有画布的本地草图编辑。画布明确未保存/未发布，投影缺少完整字段映射，不能上传为可执行 DSL。注册定义仍为部署代码；复制草稿固定其引用和摘要。
- 与当前绑定比较方案版本、模式、实现引用及 Workflow/Graph 摘要；查看绑定历史。显式 URL 版本不可见时不静默替换为当前版本。
- 新增 `execution.plan.references`、`inspect`、`preflight`、`trace` 四项人工专用能力。引用、检查复用 read，预检使用 validate，轨迹使用 records 正式权限。所有动作继续经过 Gateway，管理权限不扩大目标或节点权限。
- **结构预检**仅验证样例输入 Schema、注册引用、摘要和当前授权，不读取业务记录、不执行节点/模型、不修改草稿状态。它不是业务模拟，也不承诺真实调用成功。元数据默认方案按原有机制按需落库，Gateway 仍产生基础设施调用审计。
- 本人执行记录支持目标能力筛选、原 invocation 与真实运行状态，按需查询实际 STEP_STARTED/STEP_FINISHED 事件。轨迹 SQL 强制 tenant+account+invocation，仅返回步骤名、类型、序号和时间，不返回原始 payload、模型回复或业务输入输出。不以同名能力调用推测节点完成。
- TypeScript 请求/响应类型从模块 Definition Schema 生成到 `capability-execution-plans.ts`；历史接口保留原 PascalCase 字段及 records 空输入的兼容行为。前端所有动作经 `lib/capabilities` 的生成 Gateway SDK；缓存包含租户、账号、部门、授权版本和目标版本，请求支持取消。中英文键同步；未新增依赖或迁移，未扩大 MCP/WebMCP 工具暴露。

### 待增强及验证边界

在线完整 DSL 编辑与持久化、带业务数据的模拟/断点/单步执行、灰度发布、部门/账号等更多绑定作用域仍未实现。当前是方案中“根据实际使用情况逐步增加”的管理增量，不能宣称上述功能完成。列表最多 200 版本（平台默认优先）、历史 100 条、本人记录 100 条、节点事件最近 200 条，未提供分页或归档。

本阶段验收采用页面 RTL 交互、真实 PostgreSQL、真实 Eino 编译/节点事件及生成契约校验；不使用真实外部模型。E2E 未执行（按当前指示暂停）。本会话未连接项目运行时 MCP，未进行线上 MCP 调试，也未推送/触发远程 CI。最终命令结果见本阶段验收记录。

### Phase 4 验收记录（2026-11-06）

- `make generate`：通过；sqlc 查询和执行方案 TS DTO 均从源生成，Goa/Orval 入口不另建契约。
- `pnpm --filter @baseline/web exec vitest run src/features/execution-plans`：2 文件、11 测试通过。覆盖草稿与绑定重试幂等、注册引用远程选择、独立回滚调用、结构预检输入、读取权限隐藏、URL 目标保留、不可见版本及真实轨迹接入。
- `TestExecutionPlanInspectionAndTraceIsolation`：真实隔离 PostgreSQL + Eino，通过 Workflow/Graph 元数据与预算、当前节点权限过滤、样例 Schema 拒绝、预检无业务记录/AI Run、草稿不变、历史及实际输出 Schema、能力筛选和本人/跨租户轨迹投影。
- `make test-security`：通过，包含完整隔离 PostgreSQL 集成回归；追加 Graph/权限负例后定向重跑通过。
- `make check-rules`、`git diff --check`：通过。目录规则链保持 32 KiB 限制。
- `make ci-fast`：最终完整通过，含前端全套 47 文件 / 189 测试、Go 单元测试、生成一致性、实体/流程检查及前后端构建。首次运行的本地 WebMCP 监听受沙箱 EPERM 阻止，获准本机权限后重跑；未通过削弱断言处理。
- 既有两条 React lint 警告、实体未声明提示及构建体积提示不属于本阶段新增问题。
- E2E 未执行（按当前指示暂停）。未验证浏览器布局、真实外部模型、在线完整 DSL 保存、业务模拟、灰度或更多作用域；未推送、未运行远程 CI。本阶段无新增迁移/依赖；部署仍需前阶段 00029/00030，本次未迁移运行中业务库。
