---
name: baseline-capability-composition
description: "决定一个功能拆成哪些能力、以及能力之间如何互相调用。用于持久化流程/编排定义，以及新增或改造可独立执行的功能（HTTP 接口、页面动作、定时任务、事件处理、Agent/MCP 动作）、需要复用已有数据查询、或发现业务层绕过 Gateway 直连数据库/SDK 时。不适用于纯计算函数、DTO 转换或单表字段改名。"
---

# 能力拆分与组合

先读根 AGENTS.md、[ADR 015](../../../docs/decisions/015-capability-centric-platform.md) 和 [能力拆分与组合规范](../../../docs/capability-composition.md)。规范里的六条互调约束和运行时支撑是本节的操作依据，本文件只保留决策步骤。

## 两条硬要求

1. **可独立执行的功能必须注册为能力。** 任何能被单独触发、有独立权限或风险语义、或被第二个入口复用的动作，都进模块 `Entries()`。不允许只以 service 方法、HTTP handler 或后台 goroutine 的形式存在。
2. **能力之间只经 Gateway 互调。** 编排能力调用数据能力必须走 `gateway.Invoke`。不得 `registry.Resolve` 后直接调 handler，不得绕过能力直接调 `dbgen` / `pool` / 领域服务——那等于绕过权限、幂等、审批、Outbox 与审计。

## 先发现已有能力

先按 [能力复用优先](../../../docs/agent-workflow.md#能力复用优先先发现再复用最后新增) 搜索现有动作、对象、Definition/Schema 与测试，检查权限范围及事务/事件语义。已有完整动作直接复用；已有原子能力可满足时经 Gateway 组合；差异可兼容时在原模块扩展并维护消费者；存在独立语义缺口才新增。记录已有候选及不适用原因，不仅凭名称判断重复或可用。

复用不可绕过 Gateway 直调业务 Repository/Handler，不可把多次调用说成原子事务，也不可为复用扩大权限或把不同语义塞进万能能力。按各自约定复用纯计算和基础设施辅助，不因此增加业务能力包装。

## 先决定拆几层

| 情况 | 做法 |
|---|---|
| 只被一个入口使用，且就是一次查询或写入 | 一个能力，内部直接访问数据 |
| 子操作被多个业务用例独立复用，或有独立权限/风险 | 拆数据能力 + 组合能力 |
| 同一个完整能力被页面/Agent/API 复用 | 仍是一个能力，不因适配器增多而拆层 |
| 入口要聚合多个数据源，或做字段裁剪/分页投影 | 拆入口能力，内部调多个数据能力 |
| 多个写必须同一事务原子完成 | 一个能力 + `Entities` 声明全部实体，不靠组合拼事务 |
| 纯透传、无复用、两层权限相同 | 一个能力，不拆 |

拆分不是目的。为纯透传接口造两层能力，会让权限、审批、审计、文案各写两遍而没有收益。**只有复用、权限差异、聚合三者之一成立时才拆。**

## 分层职责

- 数据能力（原子）：持有业务规则与 SQL、租户与范围过滤、状态校验、事件产出。命名 `<domain>.<object>.<action>`，如 `notification.message.query`。
- 入口能力（编排）：持有分页约束、字段裁剪、多源聚合、缓存语义；不碰 `dbgen` / `pgx` / 领域 repository。命名 `<domain>.<usecase>.<action>`，如 `notification.inbox.read`。
- HTTP、页面、MCP、Agent 都是入口能力的**适配器**，不是第三层。能力名不编码传输协议——不要用 `notification.http.query` 这类名字。

## 类型声明

每个能力还要声明编排元数据，见 [ADR 016](../../../docs/decisions/016-capability-taxonomy-and-orchestration.md)：

- `Layer`：`atomic`（自己干活）或 `orchestration`（只调别人），**必填**。`orchestration` 不直接访问业务表，`Writes` 声明传递性业务影响。
- `Kind`：service / sql / command / external / event，默认从 `Executor` 前缀推导，形态不一致时必须显式写。
- `Reads` / `Writes`：数据依赖方向，默认由 `ReadOnly` + `Entities` 推导；读写不同实体时显式拆开。
- `Effect`：none / compensable / irreversible；重试保证独立，不由事务提交方式推导。

`Kind` 与 `Layer` 是两个维度：一个能力可以既是业务动作（有权限、要审批）又有不可回滚的外部副作用，只用一个 `type` 字段必然丢信息。

## 互调六条（缺一条就是缺陷）

1. **身份**：用外层同一 `ctx` 调 `Invoke`，内层拿到同一 Principal；不提权、不换租户。
2. **来源**：内层 `Origin` 由 Gateway 强制 `OriginCapability`（按自动化处理：`high` 要确认、`critical` 被拒）。禁止传 `OriginHumanUI` 或 `OriginApproval` 逃过审批。
3. **深度**：`TriggerDepth + 1`、`CorrelationID` 透传、`CausationID` 指向父调用；超 `MaxDepth` 直接失败。
4. **幂等**：内层写能力的键由父能力/版本、父调用幂等键和动作槽确定性派生（`gateway.ChildKey(ctx, actionSlot)`），禁止随机值或时间戳。
5. **事务**：能力边界即事务边界。**`TxExecutor` 内禁止调 Gateway**（第二个事务 + 行锁交叉可死锁）；要组合就把编排能力写成非事务 `Executor`。
6. **事件**：内层产出的事件就是事实，外层不得重复声明同一事件类型；只读编排声明 `ReadOnly: true`。

## 落地顺序

1. 列动作清单，按上表定层次与命名，写进 `Definition`（名称、版本、输入输出 Schema、权限、风险、审批、幂等、成功事件、可审计字段、`Entity`）。
2. 数据能力先实现并单独验证：范围过滤、跨租户、拒绝路径。
3. 入口能力只做编排，`Data` 二次投影；不声明内层事件。
4. 两层权限都进 `internal/identity/catalog.go`，en / zh-CN 文案同步。
5. 补负向用例：内层权限不足、跨租户、深度超限、幂等重放。

## 先判定运行方式，再写编排代码

按 [开发工作流的 Eino 选型与人工节点](../../../docs/agent-workflow.md#eino-选型与人工节点) 记录使用/不使用理由。不要把所有多步功能都做成 Workflow，也不能把明确需跨请求人工等待的业务拆成互不关联的函数后宣称流程已接入。

- 先还原角色、状态、分支、人工/外部等待和重启恢复需求，再拆能力；同步、无等待组合继续用 Go Executor。
- 用户明确要 Agent 或模型需调用受限能力时，用 Eino ADK Runner，工具为固定版本原子能力白名单与当前权限交集；固定分类可直接用模型接口，但需明确是单次模型调用。Graph 只承接实际的有界循环/只读重试，不为凑三引擎而加入。
- 业务人工任务须有查询/决定权限、版本绑定、驳回原因与可见入口。核对能否复用 Gateway 审批，不能将审核人决定冒充原作者调用；不匹配时明确扩展等待/恢复适配，不另造调度器。
- 检查 spec、引擎和测试是否实际支持所需分支、业务人工信号与 Agent 输出结构。answer 文本对象不等于业务结论 Schema；模型输出先校验再交给确定性能力改状态，未知/失败结果不能被当作通过。
- 原子能力及业务数据先完成，版本化流程定义再注册/生成/检查；用真实业务入口启动、重启恢复、重复事件、旧版本决定等用例证明接入。不能以通用示例或注入 Classify stub 的测试替代实际业务 Eino 路径。


## 持久化流程与调用上下文

Gateway 已提供 CurrentInvocation / ChildKey，嵌套 Invoke 强制 OriginCapability、父调用和深度+1，事务内互调直接拒绝。旧“两层以内”临时限制已撤销；MaxDepth 仍须运行时检查。

跨请求审批或恢复使用 Eino Workflow，循环/有限只读重试用 Eino Graph，受限决策用 Eino ADK Runner。数据引用/DependsOn 定顺序，Reads/Writes 检查冲突。原子能力不启动流程，编排入口能力负责启动；同步组合不强制使用 Workflow。业务写入下沉原子能力，运行记录属于基础设施。

待审批必须触发 Eino 原生 Interrupt；Gateway.Decide 执行原动作后恢复读取同一 invocation。补偿仍通过 Gateway；TxExecutor 不代表已提交动作可撤销。流程详情/取消仅本人，检查点加密绑定租户/运行。

## 以真实契约生成流程

先阅读 [完整示例](../../../docs/orchestration-example.md) 和 [Schema 路线](../../../docs/agent-generation-plan.md)。`docs/generated/workflows/workflow.schema.json`、`*.workflow.json` 从纯规格包 ai/orchestration/spec 的 Go Definition/Step 与共享常量生成，禁止手改导出文件。

1. 先选原子动作与权限边界；字段、版本、依赖使用当前能力目录，不猜能力名称。
2. 候选 JSON 用 `./scripts/go.sh run ./cmd/workflowcheck -file <文件>` 校验（脚本工作目录 services/api）。它不执行也不注册定义；结构合法不代表授权、SQL 范围或业务顺序正确。
3. 流程运行源仍是 Go Definition；编辑源后 `make generate-workflows`。字段映射/DependsOn 决定顺序，Reads/Writes 检查冲突，不自动猜顺序。
4. `make check-workflows` 校验结构/目录/依赖/预算，`make check-generated` 拦截漂移。阶段验收 ci-fast；完整参考的数据库验收用 `make test-orchestration-example`。
5. 参数类型、Schema、目录、语义和 Gateway 各守一层；禁止生成第二份权限/审批策略或新的任意 JSON 执行入口。

## 复核清单

- [ ] 可独立执行的动作是否都注册进 `Entries()`？
- [ ] 有没有 handler / service 直接碰 `dbgen` / `pool` 绕过能力？
- [ ] 有没有能力自己 `registry.Resolve` 调 handler？
- [ ] 编排能力是否声明了不属于它的事件？
- [ ] 内层 `Origin` 是 `OriginCapability`？写能力幂等键确定？
- [ ] 外层是否 `TxExecutor` 却调了 Gateway？
- [ ] 嵌套深度是否继承并受 MaxDepth 限制？
- [ ] 两层权限与两个语言文案是否都补齐？

验证：`make check-entities`、`make check-rules`，阶段验收 `make ci-fast`。
