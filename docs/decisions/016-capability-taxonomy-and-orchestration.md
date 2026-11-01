# ADR 016：能力分类与持久化编排

日期：2026-11-01。状态：核心实现已加入工作区；验证结果见 [编排方案](../orchestration-plan.md)。继承 [ADR 015](015-capability-centric-platform.md)，能力组合硬边界见 [能力组合规范](../capability-composition.md)。

## 1. 决策与架构

Capability 是唯一业务契约，Gateway 是唯一业务执行入口。简单操作直接调用能力；同步聚合可用 Go 组合能力；需要跨请求等待、审批恢复和重启恢复的过程使用持久化流程。`Layer=orchestration` 不等于必须启动 Workflow。

调用路径：调用方 → Gateway（编排入口能力）→ 编排运行时 → Gateway（每个业务动作）→ 原子能力。Registry 是 Gateway 的契约目录，不是独立执行服务。Outbox → Event Hub → Trigger → Agent → Gateway 的既有闭环保留。

三个引擎全部使用锁定的 Eino v0.9.22：

- `compose.Workflow`：业务 DAG，按显式依赖与字段映射调度。
- `compose.Graph`：局部轮询和有限瞬态重试，必须设 MaxRunSteps。
- `adk.ChatModelAgent` + `adk.Runner`：限定节点内的工具选择、顺序与推理，使用原生检查点恢复。

项目只实现业务适配、队列领取、持久化和策略，不自建节点调度器。业务顺序不使用 ADK Sequential/Parallel/Loop 原语；AST 测试检查该约束。

## 2. 能力元数据

| 字段 | 语义与默认 |
|---|---|
| Layer | atomic / orchestration；必填，无运行时默认 |
| Kind | service / sql / command / external / event；由 Executor 前缀推导，默认 service |
| Reads | 读取的业务实体；未声明时保守取 Entities |
| Writes | 包含间接调用在内的业务写影响；写能力未声明时取 Entities |
| Effect | none / compensable / irreversible；只读默认 none，写默认 irreversible |
| RetrySafe | 重试保证，默认仅只读为 true；不从 TxExecutor 推导 |

atomic 指可独立执行的业务单元，不保证整个动作可随数据库事务撤销。Kind 不决定事务保证；真正的事务边界由 Handler 绑定及副作用实现决定。orchestration 不得直接读写业务表，但可声明通过子能力产生的 Writes；运行记录与检查点属于基础设施持久化。

这些默认值支持保守迁移，不承诺仅补 Layer 就可以自动安全组合任何业务。正式流程使用的能力必须核对读写范围、风险和副作用。存量能力的 Layer 已显式补齐。

## 3. 顺序与并行

输入映射引用前驱输出时自动产生依赖，`DependsOn` 表达无数据传递的业务顺序。Reads/Writes 检查读写、写读和写写冲突；冲突节点没有依赖路径时拒绝编译，不猜测谁应先运行。无冲突节点由 Eino 自行并行调度。

实体粒度可能过于保守，暂不引入资源行键。生产数据库并发正确性仍由能力内部约束、锁和事务承担，流程冲突检查不能替代它们。

流程和能力版本显式固定；Agent 工具用 `name@version` 固定版本。运行保存定义及 hash，恢复要求当前部署保留相同定义。编译结果按定义 hash 缓存，不捕获单次运行身份或输入；柔性指执行前组图，执行中不修改图。

## 4. 事务、重试与补偿

能力边界即事务边界。TxExecutor 只保证该次业务数据与 Outbox 同事务；提交后不能因后续步骤失败自动回滚。强原子多实体写应仍是单个能力。

- v1 自动重试仅适用于显式设置 MaxAttempts 的只读原子步骤，最多 5 次，仅重试 unavailable/rate_limited/timeout；由 Graph 表达回边。
- 轮询只允许只读原子能力，最多 100 轮，并受 Graph MaxRunSteps 和运行时间片约束。
- compensable 能力可在流程步骤声明补偿能力和参数映射；失败后按反向依赖构造另一个 Eino Workflow，补偿仍经 Gateway，可能等待审批。
- 补偿不是回滚；已提交且未配置补偿的写入、补偿失败均标记 manual_recovery_required。补偿不会删除已经发布的事实或伪造 exactly-once。
- 外部写自动重试未开放；超时后的实际结果需要领域能力提供查询和对账语义。

## 5. 审批与恢复

Gateway 决定是否审批；节点适配器将 awaiting_approval 转换为 Eino 原生 Interrupt。普通状态对象不会自动暂停引擎。

Gateway.Decide 重建申请人身份并执行原 invocation；流程恢复读取同一 invocation 的结果，不重新发起该业务动作。节点输入、结果回执和引擎检查点加密持久化，解决审批完成早于 checkpoint、执行提交早于回执等恢复窗口。流程动作审批按稳定 action_key 隔离，不把两个输入相同的步骤合并成一次动作。

Workflow/Graph 与 ADK Runner 共用存储实现，但使用独立作用域；Agent 工具中断先由 ADK 保存，外层再中断 Workflow。恢复须测试整个桥接链，不能仅凭 CheckPointStore 接口相同就声称自动互通。

## 6. 身份与调用链

Gateway 为 Executor 注入可信 InvocationContext，提供 invocationID、correlationID、深度、幂等键与来源。嵌套 Invoke 强制派生 OriginCapability、父调用与深度，禁止冒充 human-ui/approval，TxExecutor 内嵌套被拒绝。OriginCapability 按自动化策略处理。

持久化工作器每次恢复重建 Principal，每个节点重新读取申请人的当前授权。Agent 工具集取当前权限与节点白名单交集，只允许 atomic 能力，不能从流程节点递归启动流程。事件深度、调用深度、Graph 步数、Agent 轮数和运行时间片分别有界。

流程节点幂等键：`orch:<runID>:<stepID>:<iteration>`；Agent 工具再纳入 Eino toolCallID。独立运行、重复步骤、循环轮次互不混淆；同一动作恢复复用原键。Gateway 重放校验账号、输入摘要与能力版本。

## 7. 范围与后续

实现落点、能力列表、运行语义、迁移、安全边界和验收见 [编排方案](../orchestration-plan.md)。暂不提供流程设计器、数据库发布链、任意命令/SQL/外部执行器；Kind 只是契约分类，不授权执行任意脚本。
