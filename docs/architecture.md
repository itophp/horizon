# 架构

单仓库、单 Go 服务；避免过早微服务化。浏览器 → Vite 同源代理 → Goa HTTP 传输 → 应用服务 → Repository → sqlc → pgx → PostgreSQL。

apps/web/src/app 管理 Router、Provider 和布局；components/ui 为 CLI 安装且可维护的 shadcn 源码；components/shared 承载以后真正共享的组件（当前没有人为创建空组件）；features/administration 实现登录、组织、账号与权限后台，features/example 保留技术验收。TanStack Query 管理远程状态，表单用 React Hook Form + Zod，不引入重复服务端缓存的全局状态库。

API 源契约在 design。Goa 生成端点、编解码、传输和 OpenAPI；internal/app 实现技术示例，internal/identity 实现身份管理模块（服务、Casbin 策略、模块 Repository、会话与密码）；internal/transport 组装生成的路由，并统一请求 ID、结构化日志、认证和授权。auth 建立可信 Principal；authorization 的 Casbin RBAC 域为租户；dataaccess 从可信身份解析 Scope；Repository 参数化 SQL 强制租户与范围条件。

健康检查、echo 是公开的无敏感数据技术端点。持久化演示是受保护的；当前默认策略为 OWN，可扩展 ALL/ORGANIZATION/DEPARTMENT/CUSTOM，所有范围都保留租户条件。正式 identity 服务另外实现企业/公司/部门树、账号、多角色及独立权限，支持六类可信范围；角色为工作区级策略配置。详见 decisions/002-identity-access.md。对象共享及行业模型尚未实现。

Goa gen、sqlc dbgen 和前端 api/generated 为生成器专属目录。Orval 使用 Goa 的 OpenAPI 3.0 输出；Goa 同时生成的 3.2 契约留作产物，当前 SDK 不消费它。唯一类型来源为 DSL，不在页面复制 DTO。

Playwright 启动真实 Go + Vite；数据库测试使用实际 PostgreSQL 的独立 schema，覆盖迁移往返、范围约束、并发修改、回滚和 HTTP 投影。CI 将快速验证置于数据库与浏览器之前。

浏览器端验收实际运行 Vite 生产构建和 preview 服务器（非 mock），通过同源代理访问真实 Go API。这样避免开发模式 StrictMode 的请求取消和 SDK 再生成触发 HMR 干扰验收；测试仍保留网络错误、控制台错误及页面行为断言。


账号多部门、部门独立策略与会话上下文由 [ADR 004](decisions/004-department-memberships.md) 定义，禁止将不同部门角色合并到一个 Principal。临时权限与申请部门绑定，所有授权修改同步规则/Skill。

系统治理服务入口为 design/governance.go 与 internal/{governance,settings,dictionaries,files,notifications}，共享 identity 的可信授权。领域/存储/事件决策及注册扩展点见 [ADR 005](decisions/005-system-governance.md)，产品行为见 [正式方案](system-settings-and-dictionaries-plan.md)。前端 features/governance 使用生成 SDK 和上下文隔离 Query；公开品牌与鉴权运行时配置分开，个人偏好落服务端。

[ADR 007](decisions/007-global-governance.md) 取代治理模块的组织范围设计：设置和字典统一共享，个人偏好独立，文件/消息/动态来源保持原数据授权。新业务按 [公共能力接入约定](shared-capabilities.md) 复用，避免平行实现配置、选项和文件基础设施。

独立通知中心及人工 Markdown 通知按 [ADR 008](decisions/008-notification-center.md)：announcement 与接收快照/附件/投递原子写入，邮件复用持久化队列；不再把投递记录放在系统设置中。通知附件通过共享文件服务的绑定鉴权入口读取。

页面 Agent 能力见 [ADR 009](decisions/009-webmcp.md)：features/mcp 管 `/mcp` 统一接入页与远程凭证面板，features/webmcp 管显式连接、客户端设置与 WebSkills 面板，lib/webmcp 管受控工具注册表/本地安装传输；scripts/webmcp-server.mjs 使用官方代理包，仅在本机 18793 服务。客户端 stdio 适配器复用当前页工具，三套可安装 WebSkills 在 webskills。未来 AI 助手复用注册表 list/invoke，当前未启用模型或聊天界面。

系统设置持久定时发布见 [ADR 010](decisions/010-settings-schedules.md)。共享设置服务复用现有发布事务，保存排期意图，worker 以数据库行锁领取并在执行时重新授权。状态随发布/失败原子保存，重启恢复 pending；不把前端定时器作为执行器。


账号级远程 MCP 见 [需求与实现方案](mcp-service-plan.md)（首阶段开发实现，验收状态见交付记录）。开发使用 baseline-mcp-development，功能变动主动确认暴露范围；账号 Token 与本机 WebMCP 配对分离。实施复用可信 Principal、Casbin、SQL 范围及字段投影，Token 固化工具集合与当前正式权限取交集，撤销/禁用实时生效，秘密仅一次显示且不进入日志/配置包。实现入口为 internal/mcp，协议、数据库和客户端验收须分别记录。

账号远程 MCP 使用官方 Go SDK Streamable HTTP，管理契约与前端仍走 Goa/Orval。工具适配复用 PrincipalForTask 和业务服务，不依赖页面；本机助手仅安装独立 stdio 桥。见 [ADR 011](decisions/011-remote-mcp.md)。
