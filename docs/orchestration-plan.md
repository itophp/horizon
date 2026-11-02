# 持久化编排：Eino Workflow / Graph / ADK

状态：核心实现已加入工作区。架构决策见 [ADR 016](decisions/016-capability-taxonomy-and-orchestration.md)，互调规则见 [能力组合](capability-composition.md)。

## 1. 执行路径

```text
页面 / Agent / 事件适配器 / API / MCP
  → Capability Gateway
    → orchestration.workflow.run（受理，立即返回 runId）
      → ai_runs + 加密 manifest + accepted Outbox（同事务）
        → 持久化工作器领取运行
          → Eino compose.Workflow（业务 DAG）
            ├─ Lambda → Gateway → 原子能力
            ├─ Eino compose.Graph → Gateway（有限只读轮询/重试）
            └─ Eino adk.Runner + ChatModelAgent → 工具 → Gateway
```

简单动作仍直接经过 Gateway，不强制包装成流程。Registry 为 Gateway 提供元数据。业务写入与 Outbox 继承原能力的实际事务保证；旧模块的 best-effort 边界不因增加编排而消失。

## 2. 能力入口

定义由 `internal/modules/orchestration` 的 Entries 自持，运行时位于 `internal/ai/orchestration`，现有 grounded answering 的 `ai/workflow` 保留。

| 能力 | 权限 | 语义 |
|---|---|---|
| orchestration.workflow.list | orchestration:read | 代码注册的流程目录、版本、hash 与输入 Schema |
| orchestration.workflow.diagram | orchestration:read | 只读投影：Eino 实际编译出的节点、控制边/数据边、节点类型与输入 Schema；不含输入值、检查点或模型消息 |
| orchestration.workflow.get | orchestration:read | 本人运行摘要，不返回输入、模型消息或 checkpoint |
| orchestration.workflow.run | orchestration:run | 高风险启动；受理与运行成功分开；要求幂等键 |
| orchestration.workflow.cancel | orchestration:cancel | 高风险取消本人运行；要求幂等键 |

人类页面按既有 high 策略执行；Agent/API/MCP 启动与取消必须人工确认。审批本身仍使用已有 Gateway 队列，不创建另一套审批 API。目录读取不代表拥有其中所有能力权限，节点实际执行再授权。

控制台 `/orchestration` 的服务端流程定义与运行数据只读：目录、编译图、步骤表、运行与节点轨迹。图来自编译期 `compose.GraphInfo`（`WithGraphCompileCallbacks` 按 hash 缓存），未编译节点会显式标注，前端不推断业务语义，提供前端本地草稿编辑（连接线插入、节点两侧同级条件分支、节点移除与前后自动重连、撤销／重置），不保存或发布到运行时。画布用 `@antv/x6`（锁定 3.1.8）+ `@antv/x6-react-shape` 渲染节点、`@antv/layout` 的 DagreLayout 分层，边用 manhattan 路由避让节点；节点拖拽和连线编辑关闭；支持节点选择、缩放、属性检查与输入 Schema 查看。`start` 的出边属于“根部输入可用”的数据依赖，不表达顺序，因此不绘制。该页按路由懒加载，图引擎不进入首屏包。

远程 MCP 工具：workflows_list / workflows_get / workflows_run / workflows_cancel。目录按 Token 固化集合与当前权限过滤，创建 Token 时也校验权限，旧 Token 不自动获得新工具。写工具声明非只读且需审批。客户端使用已有 Streamable HTTP；未执行原生客户端或浏览器验收时不得宣称互通全量通过。

## 3. 定义、编译与扩展

v1 流程由 Go 代码声明，Name + Version 唯一；运行固定定义 hash 和能力版本，Agent 工具名带 @version。默认注册：

- example-lifecycle@1.0.0：创建 → Graph 就绪检查 → ADK 只读评审 → 人工审批归档；完整用法见 [示例](orchestration-example.md)。
- example-create-archive@1.0.0：创建参考记录，再等待归档审批，用于证明恢复链。
- example-review@1.0.0：限定 ADK 使用参考记录的查询能力，复用现有服务端模型配置，执行者还需 assistant:use 权限。
- article-publish@1.0.0：文章进入审核中 → ADK 结构化评审 → 记录评审结论 → 按结论分叉到 article.publish 或 article.reject；见下节。

新增定义通过 orchestration.New 的 definitions 参数接入。Step 支持 ID、Capability、Version、Constants、Input 映射、DependsOn、Poll、MaxAttempts、Agent、Compensation、When。Input 映射 From=input 表示流程输入，否则是前驱步骤 ID；Field 为对象字段，缺失即失败。

- 明确数据引用与 DependsOn 定顺序；读写冲突没有顺序时编译失败。
- ID 独立于能力名，同一能力可重复使用。
- 普通能力输出投影为 JSON 对象；当前不支持直接数组/标量输出作为流程步骤。
- 无依赖只读步骤由 Workflow 原生并行执行；不自建拓扑调度器。
- Graph 用回边表达只读轮询和有限瞬态重试，设置 MaxRunSteps；MaxAttempts≤5，Poll.MaxSteps≤100，单流程≤64 步。
- ADK Runner 的工具列表来自固定版本白名单与当前权限交集；工具顺序执行，MaxIterations≤20。声明 `agent.output` 时节点结果必须是该 Schema 描述的 JSON 对象：回答里的额外字段会被丢弃，散文中的最后一个 JSON 对象会被提取；解析失败时追加一次纠正式追问（只重述结论、不再执行工具），仍失败才判定 `invalid`。未声明时仍是 answer 文本对象。
- 禁止业务使用 ADK Sequential/Parallel/Loop；由 AST 测试检查。流程内只允许 atomic 工具，不允许递归流程。

### 3.1 条件分叉

`when: {from, field, equals}` 让步骤变成可选分支：仅当 `from` 步骤输出里 `field` 等于 `equals` 时执行。同一 `(from, field)` 的步骤构成一个分支组，组内 `equals` 必须互不相同，运行时最多命中一个。

- 编译为 `compose.Workflow.AddBranch`：选中的节点执行，其余节点被 Eino 标记 skipped，并沿 END 传播，因此未走的业务分支不会产生任何能力调用。
- 分支目标只允许有分叉这一个控制前驱；根输入以 `WithNoDirectDependency` 只作数据依赖接入，否则该节点会绕过分支跳过判定直接执行（Eino 明确要求跨分支连接使用该选项）。
- 一个都不命中时分支返回 `branch_unmatched` 并使运行失败，不静默成功——没有结论就不能当作已完成。
- 分支来源字段必须在其输出 Schema 中声明（能力 `Output` 或 `agent.output`）；在未声明结构化输出的 Agent 上分叉是注册期错误。
- 同组互斥步骤之间不要求排序，读写冲突检查对它们豁免；与组外步骤的冲突仍必须显式排序。
- 分支结论应来自已落库的事实，而不是模型自由文本：article-publish 让 `article.record-review` 写入 verdict 并返回归一化后的 `decision`，分叉只读这个 `decision`。
- 分支目标可以是高风险能力：`article.publish` 声明 high + confirm，节点返回 awaiting_approval 时 Eino 中断、运行转为 awaiting_approval，人工确认后从原 invocation 结果继续，未选中的分支依旧不执行。

```json
{"id": "publish", "capability": "article.publish", "version": "1.0.0",
 "when": {"from": "record", "field": "decision", "equals": "approve"},
 "input": {"id": {"from": "input", "field": "id"}, "revision": {"from": "input", "field": "revision"}}}
```

## 4. 持久化和状态机

沿用 ai_runs 作为运行头、cap_invocations 作为业务调用轨迹；新增 orch_checkpoints 保存 manifest、输入快照、节点回执、Eino Workflow/Graph 和 ADK 的字节状态。未创建另一份业务执行步骤表。

状态：running → awaiting_approval → running/awaiting_approval → completed/failed/cancelled。恢复执行期间允许运行仍显示 awaiting_approval，直到引擎给出下一稳定状态。补偿完成后原流程仍为 failed，不将原业务失败伪装为成功。只有补偿本身执行失败才报 errorCode=manual_recovery_required；已提交但没有声明补偿的步骤只在日志里标出，errorCode 保留节点的原始错误码，否则"模型没按 Schema 回答"和"业务冲突"都只能看到一个笼统代码。

启动能力使用基础设施事务写运行头、manifest、确定性启动标识和 accepted Outbox，不在该事务内执行任何业务节点。外层 capability invocation 的 succeeded 表示已受理，ai_runs 状态表示后续运行结果。

工作器每秒扫描队列，通过 SQL 条件更新领取 90 秒租约，单次运行时间片 60 秒；租约到期可重新领取，领取顺序按租约时间轮转。取消、查询始终要求 tenant + account。取消阻止后续步骤并拒绝尚未执行的审批；已经执行/已进入执行的动作不回滚。

检查点通过现有治理 AES-GCM 密钥加密，认证绑定 tenant/run/key；无密钥启动失败。单份明文状态最多 4 MiB，SQL 限制密文大小。API/审计/模型配置不暴露 checkpoint。密钥轮换迁移、归档和保留期自动清理仍待后续建设，部署方应按运行保留策略清理终态运行。

## 5. 审批、恢复与幂等

1. 节点先持久化输入，再调 Gateway。
2. Gateway 返回 awaiting_approval；节点保存 invocation 回执并发出 Eino Interrupt，引擎保存检查点。
3. Gateway.Decide 重建申请人身份并执行原动作。
4. 工作器重新读取申请人身份，恢复相同引擎 checkpoint。
5. 节点读取原 invocation 的已完成结果；后继步骤继续。

ADK 写工具执行前先触发原生工具中断，Runner 保存包含 toolCallID 的决策检查点后，下个时间片才执行写入；此类内部检查点保持 running。业务审批中断显示 awaiting_approval。Runner 使用独立 checkpoint，再向父 Workflow 传播暂停。恢复先恢复 Runner，工具读取原 invocation；不依靠模型重新规划来还原已批准动作。

节点回执补足进程崩溃窗口：没有最新 Eino checkpoint 时可从旧状态重跑控制逻辑，但已完成节点复用回执；业务调用提交后还没保存回执时由稳定幂等键重放。只读节点也保存结果，避免重启改变已完成输入事实。执行中崩溃留下 executing invocation 的不确定结果不得盲目重复写，需要人工对账。

节点键为 runID/stepID/iteration，Agent 工具步骤再纳入 toolCallID。审批 action_key 区分流程动作；非流程审批保留原有同载荷去重语义。相同幂等键但账号、版本或输入摘要不匹配时拒绝。

## 6. 失败与补偿

只读步骤可声明 MaxAttempts，以 Graph 对 unavailable/rate_limited/timeout 有限重试；权限、Schema、业务冲突不自动重试。写能力不因 TxExecutor 或 Effect 被自动重试。

EffectCompensable 的原子步骤可声明补偿能力、版本和映射（只引用流程输入或本步骤结果）。失败后从已完成写步骤构建反向依赖的补偿 Workflow，所有动作仍经过 Gateway，审批暂停继续使用检查点。未配置补偿的已提交写入、补偿失败或无法确认结果的外部副作用需要人工处理。

## 7. 调用上下文与安全

Gateway 注入可信 InvocationContext；嵌套调用强制 OriginCapability、深度+1、correlation 透传、causation 指父 invocation。TxExecutor 内再次 Invoke 被拒绝。同步组合能力通过 ChildKey(ctx, actionSlot) 派生动作键。

持久化运行保存父调用、关联 ID 和基准深度；每个节点调用仍由 Gateway 检查 MaxDepth=8。Graph 循环次数不累加调用栈深度，另受 MaxRunSteps 限制。Agent 工具额外增加一层深度，工具不能是编排能力。

身份在恢复和每次节点执行前通过 PrincipalForTask 重建；审批也按原申请人重建，取消后旧审批不得启动后续业务。run/read/cancel 权限均不可临时授予、不配置数据范围；数据只属于当前账号，管理员也不能凭 runId 读取别人的输入状态。

## 8. 验证与未覆盖项

本地定向用例位于 tests/orchestration_test.go：真实 PostgreSQL 的启动幂等、审批恢复、引擎重建、密文保护、Graph 轮询/上限/重试、ADK 工具调用与审批恢复、取消及补偿。单测覆盖依赖冲突、循环、版本/预算、嵌套来源与深度保护、AST 引擎使用约束。最终命令结果以交付汇报为准。

迁移 00021 扩展 origin 约束并新增审批 action_key、检查点表；Down 在独立待审批动作存在时可能被唯一约束拒绝，应先处置待审批流程，不自动合并或删除。SQL/Goa/SDK 均从源生成。

未实现：流程数据库发布链与完整 DSL 设计器（当前支持编译图查看及未发布的前端本地草稿）、外部动作对账协议、任意 SQL/command/external 执行器、长期检查点自动归档、分布式业务 exactly-once。E2E 依当前指示暂停，已用 Go SDK 验证真实 Streamable HTTP 协议的目录/启动审批/查询/取消审批/Token 撤销；真实在线模型与原生 MCP 客户端另行验证；测试模型只证明引擎和工具协议。

流程标准 Schema、候选检查和生成器路线见 [规划](agent-generation-plan.md)。make generate-workflows 导出实际 Go 定义；运行时 Validate 也复用同一结构规格，语义检查另行校验能力/依赖/冲突，授权仍在 Gateway。
