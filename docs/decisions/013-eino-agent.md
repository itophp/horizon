# ADR 013：Eino 智能体运行时（Agent + 工作流 + 知识库）

> **已被 [ADR 014](014-ai-native-platform.md) 与 [ADR 015](015-capability-centric-platform.md) 取代。** 仅保留模型配置与知识库结论作历史记录。

日期：2026-10-31。状态：已实现（真实模型联调待 Token；向量检索与 MCP 业务工具见「未覆盖」）。

## 背景

ADR 012 的内嵌助手是「浏览器内工具循环 + 服务端单轮模型网关」：模型只做一次补全，工具调用由页面执行。它解决了页面上下文与页面工具，但拿不到三样东西：服务端多步推理、可复用的知识沉淀、可编排的固定流程。本轮引入 CloudWeGo Eino 作为智能体运行时补齐这三点。

## 决策

保留页面内助手，新增服务端智能体通道。两者共用模型配置与授权，但运行时不同：

| 通道 | 运行时 | 工具执行位置 | 入口 |
|---|---|---|---|
| 页面内助手（保留） | 手写 `/responses` 网关 + 浏览器 4 轮循环 | 浏览器（`PageToolRegistry`） | `POST /api/assistant/chat` |
| 服务端智能体（新增） | Eino ADK `ChatModelAgent` | 服务端（知识库检索工具） | `POST /api/assistant/agent` |
| 固定工作流（新增） | Eino Graph：retrieve → prompt → model | 服务端 | `POST /api/assistant/workflow` |

不改写页面助手的原因：页面工具**必须**在浏览器执行，把循环搬到服务端会强依赖本机 WebMCP 桥（`pnpm webmcp` + 单标签绑定），这是产品行为变化而非重构。两条通道并存，各自边界清晰。

## 模型接入

- Eino 侧用 `eino-ext/components/model/openai` 的 `ChatModel`，只支持 Chat Completion 协议，因此新增设置 `agent.baseUrl`（默认 `https://open.bigmodel.cn/api/coding/paas/v4`），不复用带 `/responses` 语义的 `assistant.baseUrl`。
- `agent.model` 与 `agent.token` 不新增：复用 `assistant.model` / `assistant.token`，避免同一凭证存两份、轮换两处。
- 开关 `agent.enabled`（默认 false）；`/api/assistant/status` 增加 `agentEnabled` 供前端决定是否展示模式切换。
- 三个键都在工作空间作用域、UI 可见；token 仍是密封 secret，沿用既有发布与重新认证流程。

## 知识库

- 表：`kb_documents`（标题、来源、创建人）+ `kb_chunks`（序号、内容），均带 `tenant_id`，外键 `(tenant_id, document_id)` 级联。
- 切分：按 rune 窗口 600 字、重叠 80 字，单文档最多 200 块；重叠保证跨边界的句子两侧都可检索。
- 检索：`pg_trgm` 三元组相似度（`similarity >= 0.05`）+ 上限 6 条。当前 Postgres 只提供 `pg_trgm`，没有 pgvector/zhparser；三元组对中文可用，但对极短查询偏弱，属于已知限制。
- 隔离：租户来自认证 Principal，不从请求读取；`Retriever.Retrieve` 与 `Store.Search` 都不接受外部 tenant 参数。
- 写入：`POST /api/assistant/knowledge` 要求 `settings:update`（知识库会影响所有助手的回答），列表要求 `assistant:use`。

## 智能体与工作流

- `adk.NewChatModelAgent` 负责 ReAct 循环（模型自行决定是否检索）；工具集当前只有 `knowledge_search`，参数是单个 `query`，返回带标题的片段。
- 事件流收敛为 `content` + 工具名 + 模型名，不返回工具参数或原始结果，避免把内部片段写进日志或前端。
- 工作流用 `compose.NewGraph[string, *schema.Message]()`：`retrieve → prompt → answer`，提示词强制「只依据片段作答、缺失要说明、标注片段标题」，问题通过 context 传入（Graph 节点输入是片段）。
- 对上层暴露三种模式：`agent`（模型自主）、`workflow`（必定检索）、`chat`（页面内，原有）。

## 授权与边界

- 复用 `assistant:use`，不新增权限：智能体与页面助手是同一能力，只是执行位置不同。
- 模型、端点、凭证只在服务端解析；`AssistantAgentReply` 只含文本、工具名、模型名、模式。
- 上游错误统一映射为 `unavailable`，不透传响应体。
- 会话守卫仍覆盖 `/api/assistant/`，新增端点自动获得 Principal、Origin 与 CSRF 校验。

## 未覆盖

- **真实模型联调**：需要部署方提供 GLM Token 与 `agent.enabled` 发布后才能端到端验证。
- **向量检索**：没有 pgvector，当前为词法近似；接入 embedding 需新增扩展或外部向量库。
- **MCP 业务工具**：`internal/mcp` 的业务工具尚未注册进智能体工具集，接入需处理客户端鉴权与单会话绑定。
- **流式输出**：Eino 支持 `Stream`，但 HTTP 层仍是同步返回，改 SSE 需要新的 Goa 契约。
- **知识库管理页面**：目前只有接口，管理端 UI 与文件导入未做。

## 关键代码位置

| 职责 | 位置 |
|---|---|
| 表与查询 | `services/api/db/migrations/00013_knowledge.sql`、`db/query/knowledge.sql` |
| 模型与设置解析 | `services/api/internal/agent/config.go` |
| 切分、检索、工具 | `services/api/internal/agent/knowledge.go` |
| 工作流图 | `services/api/internal/agent/workflow.go` |
| 智能体与写入 | `services/api/internal/agent/agent.go` |
| 接口契约 | `services/api/design/assistant.go` |
| 服务装配 | `services/api/internal/transport/assistant.go` |
| 前端模式切换 | `apps/web/src/features/assistant/provider.tsx`、`widget.tsx` |
| 依赖与选型 | `docs/versions.md` |
