# ADR 009：页面 WebMCP 与 WebSkills

日期：2026-10-29。状态：已实现；验证结果由交付记录说明。

## 决策

OpenTiny next-sdk 提供浏览器 WebMCP polyfill 与页面代理，官方 @opentiny/agent 提供 Streamable HTTP 转发。现有 React、Router、shadcn、TanStack Query、生成 SDK 与后端 Goa/Casbin/SQL 范围不替换。SDK 从 core 入口按需动态加载；不启用任意 JS/通用 DOM 操作工具、不引入 Vue 聊天组件。

`/mcp` 是已登录用户的统一 Agent 接入页，沿用现有布局：远程 MCP 凭证、WebMCP 页面连接与 WebSkills 分列三个标签，`?tab=remote|page|skills` 只作为入口提示，切换在页内完成；旧 `/mcp-settings`、`/webmcp` 仅做重定向，不再维护第二套页面。配置属于本机部署和短期连接，不属于工作空间业务设置，因此不新增治理配置键、数据库或 API 契约。端口默认 18793，仅监听 127.0.0.1；WEBMCP_PORT 配置实际服务，页面输入必须匹配。来源白名单由 WEBMCP_ALLOWED_ORIGINS 部署配置，不允许页面放宽。

## 生命周期与字段边界

工具定义可声明 global、module、page 层级、版本、操作类型和权限标识；当前全局工具、当前模块上下文及当前页面上下文通过同一注册表发布。权限标识是描述信息，不是服务端授权。路由清单包含公开和参数化路由；连接只存在于已认证工作区，公开页面明确不接入会话。页面清单一致性由快速测试校验。当前模块和通用页面工具仅返回路由模式、权限允许的模块路由及交互计数，不代表各页面业务流程已经适配。

连接需显式配对。服务生成 256 位密钥，保存在忽略提交的 `.local/webmcp/connection.json`（目录 0700、文件 0600），通过终端显示给本机操作者；浏览器密码输入与连接对象只在内存保存，成功后清空输入。传输只使用 Authorization 请求头。SDK 的 sessionId query 是路由标识，另需 Bearer 才能访问；不把配对密钥放 URL。

为免手动抄写，开发服务器提供 `/__webmcp/connection`，把该文件的端口与密钥交给同源回环页面预填表单。该中间件只在 serve 时注册，不进入构建产物，响应 `no-store` 且不设 CORS 头，Host 非回环或 Origin 非同源一律 403，读不到有效文件返回 404 并由页面回退到手动填写。密钥仍由本机服务用 Bearer 校验，预填只改变输入方式，不放宽鉴权。

连接跨应用内路由保留；刷新、退出、切换部门、401、页面关闭或显式断开撤销注册。用户/租户/当前部门/CSRF 上下文变化重建 provider。注册表在执行时再次检查连接状态，捕获的旧工具句柄不能在断开后执行。

页面工具仅开放白名单：当前路由/允许导航目的地、受权限检查的导航、当前账号页面筛选计数与已有授权数据的只读详情，以及请求数量/稳定错误码/弹窗数量/无效字段数量。业务数据继续由生成 SDK 和真实后端授权获取；拒绝任意路由、未知字段、未授权/未加载账号和范围外 ID。无业务写入、通用 evaluate、整表/缓存导出、凭证或日志明文工具。诊断不返回 query key、异常 message、账号数据或字段值。

组织、部门及角色页面还提供复用页面现有搜索状态的筛选工具，返回请求已接收而非声称渲染完成；测试必须检查可见搜索值。工具只对拥有相应 read 权限的会话注册，不返回记录。

本地服务仅暴露 MCP、受鉴权的设置安装和不含配置的公开 liveness。Host 精确校验防 DNS rebinding，Origin 精确白名单，setup 还要求已连接页面 sessionId 与 Bearer。无 session 列表/reset/任意文件读写/任意命令接口。此服务是本机开发能力，不新增业务角色；生产远程部署和多用户代理不在本次支持范围。

## 一键设置

已连接页面按钮调用本地服务安装：Codex 通过官方 CLI `mcp add` 合并配置，WorkBuddy 合并用户 mcp.json；各自安装 webskills 下的三套技能。修改前备份配置和同名技能，保留其他服务；非法 JSON 拒绝覆盖。客户端都使用绝对 Node/script 路径的 stdio 适配器，其通过受限本地文件读取密钥和当前绑定页，无凭证写进 MCP 配置或 argv。

安装仅由用户点击或显式 CLI 发起，不在应用启动时自动修改全局配置。目标始终是本地服务所在电脑。当前绑定为一个活动标签页，重新连接/换页后再次设置并重启客户端；服务重启清除旧绑定。部分安装失败可能已经更新 MCP 配置，UI 提示失败并保留备份供恢复。移动仓库后重新安装。

## 助手预留

`PageToolRegistry.list / invoke` 是与远程 MCP 共享工具定义、输入验证、注册/撤销语义的适配接口。内嵌助手已按 [ADR 012](012-embedded-assistant.md) 启用：模型、端点与 Token 属于工作空间设置且只在服务端使用，浏览器只提交会话与工具定义；页面工具仅在远程配对或助手启用时发布，退出/401/切换部门/配对失败仍撤销全部注册。业务写操作继续走既有生成 SDK/校验/服务端授权，不能直接开放状态或凭证。

## 官方依据

- [OpenTiny quick start](https://docs.opentiny.design/next-sdk/guide/quick-start.html)
- [官方 Web Agent 路由](https://github.com/opentiny/web-agent/blob/main/src/routes/v1/webmcp.ts)
- [OpenAI MCP 配置示例](https://developers.openai.com/learn/docs-mcp)
- [Codex Skills 目录](https://learn.chatgpt.com/docs/build-skills)
- [WorkBuddy 连接器协议](https://open.workbuddy.cn/docs/connector)

已发布 SDK 的兼容细节：工具注册和发现是异步接口，撤销使用 AbortSignal；禁止依赖不存在的 unregisterTool。当前浏览器代理默认 credentials:include，并使用 Stream-Session-Id，因此本机服务只对精确白名单来源返回 Allow-Credentials，进入代理前删除 Cookie，不以应用 Cookie 鉴权。WebMCP 的 DOMString 结果经官方 SDK 包成 MCP text，stdio 适配器仅解包通过官方 CallToolResult schema 的完整信封，保留错误标志及普通文本。

工具注册表变化会合并为标准 `notifications/tools/list_changed` 通知，经浏览器代理与 stdio 转发给客户端，支持页面切换后的自动发现。连接撤销先移除订阅，旧连接代次的通知不再发送。真实 stdio 客户端已验证收到路由变化通知；客户端未刷新目录时，WebSkills 提示重新连接。
