# ADR 012：内嵌 AI 助手（页面浮动入口 + 服务端模型网关）

日期：2026-10-30。状态：已实现（真实模型联调需部署方提供 Token，见「验证边界」）。

## 背景

ADR 009 在「助手预留」中约定：`PageToolRegistry.list / invoke` 是未来内嵌助手的适配接口，与远程 MCP 共享工具定义，但当时不启用模型、密钥与聊天界面。本次按用户要求启用内嵌助手，因此该约定在此升级为正式能力，并保留原安全边界。

## 决策

助手是**浏览器侧 agent + 服务端模型网关**的组合：

- 页面右下角浮动图标 → 浮动对话面板 → 关闭后回到仅图标状态；开关与模型配置来自工作空间设置，关闭时组件不渲染。
- 工具仍在页面内执行：`features/webmcp/provider` 的同一个 `PageToolRegistry` 既服务远程 MCP，也服务助手。模型只提出工具调用，执行、输入校验与权限过滤留在浏览器。
- 模型只在服务端调用：`internal/assistant` 解析治理设置后以 OpenAI **Response** 协议（`POST {baseUrl}/responses`）访问模型，浏览器永远拿不到 Token 与端点。
- 协议：GLM Coding Plan 的 Response 端点基址为 `https://open.bigmodel.cn/api/v1`，默认模型 `glm-5.3-flash`。
- 本轮为**非流式**单轮网关：一次请求返回文本 + 可能的工具调用，由页面驱动最多 4 轮工具循环。流式输出属于后续增强，不改变现有 DTO 语义。

## 配置与授权

四个设置键（`internal/governance/registry.go`，全部 workspace 作用域，UI 可见）：

| 键 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `assistant.enabled` | boolean | `false` | 助手开关；关闭时前端不渲染入口 |
| `assistant.baseUrl` | string | `https://open.bigmodel.cn/api/v1` | 模型端点；仅 TLS，回环 HTTP 供本地联调 |
| `assistant.model` | string | `glm-5.3-flash` | 模型编码 |
| `assistant.token` | secret | 空 | API Token；AES-GCM 密封存储，读取需 `SETTINGS_ENCRYPTION_KEY` |

- 保存与发布沿用既有治理流程：`settings:update` 起草、`settings:publish` 发布；token 标记 sensitive，发布需单次重新认证证明。
- 使用能力为新增正式权限 `assistant:use`（`internal/identity/catalog.go`），不在旧 viewer 种子内；未授权账号默认拒绝。内置管理员按既有 `IsAdmin` 规则可用。
- `GET /api/assistant/status` 只返回 `enabled / configured / model`；不返回 baseUrl、Token 或密文。
- 两个端点都在会话守卫范围内（`internal/transport/identity.go` 的 `guardedPath`：`/api/admin/`、`/api/auth/`、`/api/assistant/`）。守卫负责附加可信 Principal，并对非安全方法校验 Origin 与 CSRF；新增会话级接口一律要加入该清单，否则处理器拿不到身份。
- `POST /api/assistant/chat` 只接受会话消息与工具定义；模型、端点、凭证一律服务端解析，浏览器无法覆盖。
- baseUrl 拒绝 userinfo、查询串、片段、控制字符，非 https 仅允许 127.0.0.1 / localhost / ::1，避免服务端被指向任意主机。

## 生命周期与撤销

- 页面工具仅在「远程 MCP 已配对」或「助手已开启且已配置」时发布到 `document.modelContext`；两者共用 `ensureLocalTools()`，重复调用幂等。
- 退出登录、401、切换部门、配对失败、页面关闭都执行 `registry.deactivate()`，已发布工具立即撤销；助手再次使用前必须重新发布。这与 ADR 009 的撤销语义一致。
- 助手面板在退出登录时随 `RequireSession` 卸载，会话消息只存在内存，不写入 localStorage / 服务端。
- 工具结果按注册期权限过滤：无 `xxx:read` 的工具根本不会注册，模型看不到；执行时 `registry.invoke` 再次校验连接状态与工具存在性。

## 数据与日志边界

- 上游错误响应体不转发，统一映射为 `unavailable`，避免回显提示词或凭证。
- 请求体上限沿用 1 MB；单条消息与工具结果截断到 8000 字符，工具数上限 40，工具循环 4 轮。
- 不记录 Token、baseUrl 请求头或模型原始响应；审计只体现设置变更（token 值已脱敏）。

## 验证边界

- 已覆盖：设置键默认值与 baseUrl 校验、角色到 Response 协议条目的映射、上游请求形状（路径/鉴权头/模型/工具）、文本与 `function_call` 解析、上游失败不泄露细节；前端覆盖开关关闭不渲染、开合交互、Esc 关闭、工具调用回传结果并续答。
- 未覆盖：真实 GLM 端点联调（需部署方 Token）、流式输出、多用户配额与限流、助手写操作（当前工具均为只读/导航）。

## 关键代码位置

| 职责 | 位置 |
|---|---|
| 接口契约 | `services/api/design/assistant.go` |
| 生成代码 | `services/api/gen/assistant`、`services/api/gen/http/assistant/server`、`apps/web/src/api/generated` |
| 模型网关 | `services/api/internal/assistant/service.go` |
| 设置键与校验 | `services/api/internal/governance/registry.go` |
| 使用权限 | `services/api/internal/identity/catalog.go` |
| 路由装配 | `services/api/internal/transport/governance.go` |
| 助手状态与工具循环 | `apps/web/src/features/assistant/provider.tsx` |
| 浮动图标与弹窗 | `apps/web/src/features/assistant/widget.tsx`、`assistant.css` |
| 工具注册表复用 | `apps/web/src/features/webmcp/provider.tsx`、`apps/web/src/lib/webmcp/runtime.ts` |
| 挂载点 | `apps/web/src/features/administration/layout.tsx`（`RequireSession`） |
