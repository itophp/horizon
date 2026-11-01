# 能力拆分与组合

适用：可独立触发的业务动作、复用的数据访问和多步骤业务组合。分类见 [ADR 016](decisions/016-capability-taxonomy-and-orchestration.md)，持久化运行协议见 [编排方案](orchestration-plan.md)。

## 1. 唯一契约与执行入口

可独立触发、有独立权限/风险或被其他业务动作复用的操作应注册为能力，由模块 Entries 自持。页面、Agent、MCP、事件、HTTP 只是适配器，不复制业务逻辑。

能力互调只经 Gateway，不得 Resolve 后直接调用 Handler，不得让组合层绕过数据能力访问业务 repository。纯计算、DTO 转换不是独立能力。认证、协议处理和基础设施持久化有自身职责，不机械套成业务能力。

新增能力前执行 [能力复用优先步骤](agent-workflow.md#能力复用优先先发现再复用最后新增)：查实际 Entries/Definition 与共享能力，核对语义、Schema/版本、权限/范围和副作用，优先直接复用、Gateway 组合或兼容扩展。确有独立语义缺口才新增，并在架构决策中简述已有候选为何不适用；不为不同调用渠道重复实现同一业务动作，也不为复用扩大权限或削弱原子事务。

## 2. 三种组合形态

| 形态 | 职责 | 实现 |
|---|---|---|
| 原子能力 | 具体业务规则、业务 SQL、范围过滤、状态校验与事实事件 | Layer=atomic；Executor 或 TxExecutor |
| 同步组合能力 | 多源聚合、结果裁剪、有限串联 | Layer=orchestration；Go Executor 内调用 Gateway |
| 持久化流程 | 跨请求等待、审批、循环、重启恢复 | 编排入口能力 → Eino Workflow / Graph / ADK |

atomic 表示业务单元，不承诺外部副作用可回滚。业务数据仅在原子能力/领域服务中访问；Gateway、检查点和队列访问基础设施数据库不属于绕过业务能力。

Layer 必填；Kind 表示执行形态，不决定事务保证。Reads/Writes 表示包括子调用在内的业务影响；orchestration 不直接写业务表，但可以声明传递性的 Writes。Effect 使用 none/compensable/irreversible；只读默认 none，写默认 irreversible；不得将 TxExecutor 提交后的动作默认当作可撤销。

## 3. 拆分判据

- 一个完整动作被页面、Agent、API 复用：仍是同一个能力，不因此拆两层。
- 某个子操作被多个业务用例独立复用，或有独立权限/风险：拆成原子能力与组合能力。
- 聚合多个已有能力、字段裁剪或投影：可以增加同步组合能力。
- 多个写必须全成全败：合并为一个原子能力，用 Entities 声明所有对象，共享一个事务。
- 纯透传、无独立语义：保持一个能力。
- 可能等待人工审批或需跨请求继续：使用持久化流程，不能依赖普通 Go 调用栈续接。

完整功能开发按 [开发步骤与问题矩阵](agent-workflow.md) 先明确角色、状态、人工任务和失败去向，再确定拆分与 Eino 选型。跨请求等待用 Workflow；工具决策或明确 Agent 要求用 ADK；有界循环才加入 Graph。单次模型分类可以不使用 ADK，但必须明确这种选择与用户要求是否一致，不能仅凭 ai_runs 记录宣称已接入 Agent。

命名使用业务语义 `<domain>.<object/usecase>.<action>`，不在能力名中编码 HTTP/MCP 协议。

## 4. 互调六条硬约束

### 4.1 身份

沿用可信 ctx 中的 Principal；不得换租户、账号或提升 Admin。持久化恢复与节点执行前通过 PrincipalForTask 重建身份，不能把启动时的权限快照长期保存后复用。

### 4.2 来源

执行器内部调用 Invoke 时，Gateway 强制 OriginCapability，按自动化规则处理：high 需确认，critical 不自动执行。不得通过 human-ui 或 approval 值绕过子动作审批。持久化 Workflow 节点和 Agent 工具分别由可信适配器声明来源。

### 4.3 调用链

Gateway 注入 InvocationContext，嵌套调用自动继承 correlation、以当前 invocation 为 causation，并将深度加一。超过 MaxDepth=8 拒绝。事件触发深度、Graph 循环步数与 Agent 轮数分别限制，不用循环次数代替调用栈深度。

### 4.4 幂等

同步组合用 `gateway.ChildKey(ctx, actionSlot)` 派生键；同一能力出现多次时必须有不同 actionSlot。父调用重试保持父幂等键；独立父调用不能仅按相同输入摘要合并。

持久化流程使用 runID + stepID + iteration；Agent 工具额外纳入 Eino toolCallID。同一逻辑动作恢复沿用原键，下一次真实动作使用新实例键。禁止随机或时间戳作为同一逻辑动作的重试键。Gateway 重放校验账号、输入摘要和能力版本。

### 4.5 事务

能力边界即事务边界。TxExecutor 内再次调用 Gateway 会被拒绝；组合能力用非事务 Executor。编排启动器仅写基础设施运行记录、manifest 和 accepted Outbox 时可以使用 TxExecutor，但事务内绝不执行业务节点。

单步失败前可回滚，已经提交的前序步骤不能被流程“丢弃”。需要补偿时声明独立补偿能力，由反向依赖的 Eino Workflow 经 Gateway 执行；补偿也可能审批或失败。

### 4.6 事件

子能力产出的业务事件已经是事实，外层不得重复声明同一事实。外层可以发布自己独有的流程事件，例如 accepted/cancelled；不得将流程受理事件表述成业务步骤全部成功，也不得声称整个流程和事件原子提交。既有自持事务模块的 best-effort Outbox 边界保持可追踪。

## 5. 同步组合示例

通知入口可以调用查询能力后做投影：

```go
result, err := m.gateway.Invoke(ctx, gateway.Request{
    Name: "notification.message.query", Version: "1.0.0",
    Input: contracts.Input{"limit": in["limit"]},
    Origin: contracts.OriginCapability,
    IdempotencyKey: gateway.ChildKey(ctx, "messages"),
})
```

身份、父调用、关联与深度由 Gateway 从执行器上下文继承。必须同时检查 err 和 result.Status；failed 可能是业务结果而不是 Go error。只读组合不产出重复业务事件。若子步骤可能返回 awaiting_approval，不能直接把它当普通数据让后继继续，需交给持久化运行协议。

## 6. 持久化流程边界

依赖由显式 DependsOn 与输入映射确定，Reads/Writes 用于检测并行冲突。无方向冲突应报错，不根据“谁写谁读”猜顺序。

原子能力不得启动 Workflow 或 Agent。编排入口能力负责启动持久化运行；同步组合能力不因此被强制迁入 Workflow。流程节点和 Agent 业务工具只允许 atomic 能力，禁止向上递归编排。Agent 可选择节点内部工具与顺序，不能改外部 DAG。

审批决策仍在 Gateway；Eino 节点适配器将待审批转换为原生中断。Gateway.Decide 已执行原动作，恢复必须读取原 invocation 结果。CheckPointStore 共用实现不等于 Workflow 与 ADK 状态自动互通，必须维护各自作用域和恢复桥接。

上述审批指 Gateway 操作风险确认，不代表业务人工任务的全部语义。业务审核还要明确审核人身份、任务可见范围、对象版本、通过/驳回原因与独立页面入口；先检查设施是否匹配再复用。当前任意业务人工信号、条件分支和结构化评审结果不能仅凭现有审批适配器推定已支持。缺口、目标设计与验收见 [Eino 与人工节点](agent-workflow.md#eino-选型与人工节点)。

## 7. 框架支撑与装配

### 7.1 OriginCapability

已加入 Origin 枚举、策略、数据库约束和公开调用轨迹投影。嵌套调用无法冒充页面或审批来源。

### 7.2 InvocationContext

Executor 中可用 CurrentInvocation 读取 invocationID、correlationID、Depth、IdempotencyKey、Origin。Gateway 自动派生嵌套元信息并拒绝事务内互调；旧的“两层以内、深度未传递”临时限制已撤销。

### 7.3 生命周期装配

模块构造 → Entries → Registry → Gateway → AttachGateway/Attach → 开始监听和后台工作器。依赖未装配返回 unavailable，不降级直连业务数据。固定流程定义装配时校验；版本与 hash 在恢复时再校验。

## 8. 复核与验证

- 是否为业务动作注册能力，同时避免机械拆分？
- 组合层是否绕过 Gateway 读写业务数据？
- 子调用是否继承身份、关联与深度，是否存在事务内互调？
- 幂等键是否区分步骤实例，待审批是否真正中断？
- 已提交事实是否被误称为可回滚，补偿是否经 Gateway？
- 查询、取消、恢复与检查点是否保持 tenant + account 隔离？
- 事件是否重复，模型消息和输入是否进入明文审计？

验证：check-entities、check-rules、ci-fast；数据库/审批/恢复改动加真实 PostgreSQL test-integration/test-security。E2E 依根规则暂停。
