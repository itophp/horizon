# 完整参考：记录生命周期与事件驱动复核

本例用于复制开发方式和验证架构，不代表真实审批业务的领域规则。运行定义为 `example-lifecycle@1.0.0`，已通过现有流程目录、启动能力与 MCP 工具提供。所有引擎使用 Eino v0.9.22，不需要增加新的 MCP 工具或业务 REST 端点。

## 1. 同一条业务链

```text
页面/Agent/MCP → Gateway → orchestration.workflow.run
  → Eino Workflow
    → create：example.record.create，事务写记录 + Outbox
    → wait：Eino Graph 调 example.record.get，最多 3 轮，ready=true 退出
    → review：Eino ADK Runner，工具白名单只有 example.record.get@1.0.0
    → archive：Gateway 返回 awaiting_approval，Eino 中断并保存检查点
  → 人工批准，Gateway 重新授权并执行原 archive invocation
  → 重建引擎/服务重启，从检查点读取原结果，流程 completed
  → Outbox Relay → Event Hub → AI Trigger 去重 → ADK → Gateway → 查询记录
```

就绪条件来自真实记录状态：draft/active 可以评审，archived 不可评审。本例没有虚构设备等待或人为延迟，正常新建记录第一轮即就绪。Graph 的多轮、重试上限和失败补偿另由 orchestration_test.go 覆盖。Agent 只提供评审意见，不能批准归档或扩大权限。

## 2. 源与生成物

- 运行源：`services/api/internal/ai/orchestration/spec/examples.go` 的 LifecycleExample。
- 原子能力：`services/api/internal/modules/example/module.go`。新增 get 强制 tenant + created_by，不因为 Admin 而放宽详情归属；复用 example:read 权限。
- 标准规格：`docs/generated/workflows/workflow.schema.json`（JSON Schema Draft 2020-12）。
- 可读导出：`docs/generated/workflows/example-lifecycle.workflow.json`。它来自 Go 定义，是生成产物，不能修改后期待服务器自动加载。
- 检查命令：`make check-workflows`；候选 JSON 用 `./scripts/go.sh run ./cmd/workflowcheck -file <文件路径>` 检查。脚本的工作目录是 services/api，根目录文件需使用相应路径。

新增业务流程可以先生成候选 JSON 做检查，再转换为同一个 Definition 的 Go 声明、加入 Builtins，执行 `make generate-workflows`。当前不提供从 HTTP/MCP 上传任意流程定义的入口，流程发布链尚未实现。

## 3. 使用方式

先应用项目迁移并重启 API；配置治理加密密钥。真实模型模式复用 agent.enabled、agent.baseUrl、assistant.model、assistant.token，不把凭证写入流程 JSON。

申请人需要 orchestration:run/read、example:create/read/archive，Agent 节点另需 assistant:use。取消需要 orchestration:cancel。新增权限不会自动授予旧角色。使用远程 MCP 时，Token 还需显式包含 workflows_run/get/list/cancel 中实际使用的工具。

现有能力 SDK 或 workflows_run 工具使用以下参数（MCP 的 idempotencyKey 为外层字段）：

```json
{
  "flow": "example-lifecycle",
  "version": "1.0.0",
  "input": { "title": "Reference lifecycle review" },
  "idempotencyKey": "lifecycle-demo-001"
}
```

MCP/Agent 启动先产生启动审批，批准后得到 runId；人类页面按现有 high 策略直接受理。随后归档节点还需要单独审批，人类启动流程不等于批准其中所有高风险动作。用 workflows_get 查询 runId；复用同一启动键不会创建第二个运行，新的业务运行使用新键。

系统已有全局 example.record.created 触发策略。完整示例测试另外在隔离库创建 archived 复核策略，以独立验证事件后的 ADK 查询、关联信息和至少一次去重；该测试策略不会写入开发/生产配置。

### 页面按钮测试

登录后进入「业务能力」（`/capabilities`），找到「流程示例测试」面板：

1. 点击「一键运行基础测试」：无需模型，运行创建记录 → 等待归档审批 → 归档。
2. 页面出现审批卡片后，点击批准执行；看到运行状态变为完成，表示该次流程已结束。审批前不会归档。
3. 如需验证 Workflow、Graph 和 ADK，配置并启用模型后点击「运行完整测试（含 Agent）」。页面自动刷新当前运行和近期步骤记录。
4. 运行中可点击「取消运行」，已提交的示例记录会保留。失败时可重试启动，同一次重试复用原幂等键；重新点击启动按钮则创建新的运行。

每次启动都会真实创建一条示例记录。页面按钮测试不等于下述隔离数据库验收，也不验证事件重复投递等后台断言。刷新整个浏览器会清除面板的本次选择，可在原有调用记录中查阅历史。

## 4. 一条命令验收

```sh
# 显式设置为隔离本地测试 PostgreSQL，不使用生产地址。
TEST_DATABASE_URL='<本机隔离 PostgreSQL URL>' make test-orchestration-example
```

命令运行真实 PostgreSQL、真实 Gateway、Eino Workflow/Graph/ADK、审批、加密检查点、Outbox Relay、Event Hub 和 Trigger。仅将远程模型替换为确定性测试模型，因此不需要模型密钥，也不产生外部模型费用。它不是浏览器 E2E，也不能证明真实模型的推理质量。

断言包括：

1. 归档前必须有人工审批，业务记录仍为 draft。
2. 批准后重建 Eino 引擎，从持久化状态完成；创建、轮询、Agent 查询、归档次数准确。
3. 详情跨账号不可见，Agent 工具来源记录为 agent。
4. 归档事件携带原 correlation 与 invocation 信息。
5. 同一事件交付两次只产生一次 AI 运行，AI 的查询仍经过 Gateway。

取消、补偿、Graph 重试/上限、账号撤销、MCP 启动/取消审批等使用现有 TestOrchestration* 用例覆盖。阶段验收运行 ci-fast 和 test-security；浏览器与原生客户端专项按现行规则另行执行。
