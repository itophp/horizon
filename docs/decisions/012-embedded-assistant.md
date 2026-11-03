# ADR 012：内嵌工作区 Agent 与浏览器 WebMCP 工具

日期：2026-10-30；更新：2026-11-02。状态：已实现；真实在线模型联调未执行。

## 决策

浮动助手只提供工作区 Agent，不再提供「当前页面」模式。后端 Eino ADK Agent 在同一任务中选择业务能力和当前浏览器提供的 WebMCP 工具。模型、端点及凭证由服务端设置解析，浏览器不接触模型凭证。

- 业务动作由 `ai/tools.Adapter` 经 Capability Gateway 执行，保持 `OriginAgent`、实时权限、风险确认、幂等和审计。
- 页面工具由既有 `PageToolRegistry.list/invoke` 适配，保持注册、参数校验、权限过滤及撤销语义；本次不扩大远程 MCP 工具集合。
- 浏览器首次提交问题、会话 ID 和最多 40 个页面工具描述。服务端将工具描述视为不可信输入，限制名称及 Schema 大小，禁止 Schema 加载外部引用。页面工具使用 `webmcp_` 命名空间，与业务能力工具分离。
- Eino 请求页面工具时使用原生 `tool.Interrupt` 和 `adk.Runner` 检查点暂停，返回单个 `pageCall`、原 `runId` 及短期恢复凭证。浏览器执行后回传该调用 ID 与 JSON 结果，服务端恢复同一 Runner；不让模型重新规划已完成的业务动作。
- 每个问题固定首次提交的工具清单；导航后新增的页面工具在下次提问时加入。前端在执行前核对当前注册表中仍有相同名称和 Schema 的工具。导航、撤权后不可复用旧的工具执行闭包；结果只作为不可信上下文，不授予业务权限。

## 会话与配置

浮动面板保留开合、清空和停止操作。浏览器以随机会话 ID 串联问题，服务端从本人已完成的 `ai_runs` 加载问答历史；客户端不提供系统指令或伪造的模型历史。清空开启新的会话；切换租户、账号或部门会重新挂载 Provider、撤销旧请求并清理可见消息。

| 设置                | 用途                      |
| ------------------- | ------------------------- |
| `assistant.enabled` | 浮动入口开关              |
| `agent.enabled`     | 工作区 Agent 开关         |
| `agent.baseUrl`     | Eino Chat Completion 端点 |
| `assistant.model`   | 服务端模型编码            |
| `assistant.token`   | 密封存储的模型凭证        |

`assistant.baseUrl` 和 `/api/assistant/chat` 保留兼容旧调用方；浮动面板不再使用 Response 协议页面模式。状态接口按 Agent 端点判断配置是否齐全。

## 安全与恢复

- `/api/assistant/agent` 延续会话身份、Origin、CSRF 和正式 `assistant:use` 校验。恢复时再次检查当前权限、租户、本人账号、当前部门、原会话、问题及模型；恢复不放大权限。
- 复用 `orch_checkpoints` 的加密运行状态存储，键前缀为 `embedded:`，不写工作流 worker 使用的 `manifest`。无需新增表或状态枚举；`ai_runs` 等待浏览器期间保持 `running`。
- 模型消息、工具参数和结果只进入 AES-GCM 加密检查点，认证绑定租户、run 与 key；不进入审计日志或公开运行摘要。
- 恢复凭证绑定账号和部门，15 分钟到期，SQL 条件更新原子消费，重复或并发恢复只允许一个成功。已领取的恢复时间片限 2 分钟。过期任务在该账号后续 Agent 请求时标记失败，错误码 `timeout`。
- 页面结果绑定待执行调用 ID，最大 12000 字符。最多 12 次浏览器暂停，Agent 原生迭代预算为 12；业务工具调用保持 Gateway 自身的策略约束。
- 工具执行失败或工具已撤销作为明确错误结果返回给 Agent；中止请求不代表回滚已成功的业务动作。关闭页面后不会继续执行新的浏览器工具，未恢复任务按到期策略处理。

## 验证边界

测试覆盖同一原生 Eino Runner 的业务工具 → 浏览器工具中断 → 返回结果 → 继续回答，已完成业务调用不重复执行；覆盖加密状态的租户/run 绑定，以及真实 PostgreSQL 的租户、账号、部门、过期及单次恢复条件。HTTP 集成测试用可控模型端点验证接口与任务生命周期；前端覆盖唯一入口、工具回传和注册表撤销。

可控模型测试不等于真实在线模型联调；浏览器 E2E 按根规则暂停。页面专用业务工具的可用范围仍取决于各页面已有注册，不能从通用页面上下文适配推断全部业务都已接入。

## 关键代码

- 契约与生成：`services/api/design/assistant.go` → Goa / Orval。
- Agent 与加密检查点：`services/api/internal/ai/harness/browser.go`。
- 单次恢复 SQL：`services/api/db/query/ai.sql`。
- 会话适配：`services/api/internal/transport/assistant.go`。
- 浮动面板与 WebMCP 执行：`apps/web/src/features/assistant/provider.tsx`、`widget.tsx`。
