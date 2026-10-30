# ADR 001：基础框架技术决策

状态：基础技术栈采用，2026-10-28。本文认证/组织范围描述记录初始框架决策；正式后台已由 [ADR 002](002-identity-access.md) 的账号会话与持久化权限方案补充/替代，旧 demo 单独保留。

选择 React + Vite 声明式 Router，TanStack Query 管远程状态，TanStack Table 9 显式 features API。i18next 是唯一消息框架；不引入 Zustand、AG Grid 或日期库。

Goa 3.35.0 为契约源，Orval 消费 OpenAPI 3.0；避免直接消费其 3.2 输出。选择类型名 TechnicalRecord，避免生成模型 Record 遮蔽 TypeScript 内建泛型。TypeScript 最新 7.0.2 超出 typescript-eslint 8.72.1 的 <6.1.0 范围，故固定兼容的稳定版 6.0.2；移除已弃用 baseUrl，使用相对 paths，不通过 ignoreDeprecations 掩盖问题。

Goose 采用 Go library + 独立 migration 命令，与 API 共用锁定模块图，避免额外全局迁移工具。sqlc 1.31.1 独立安装到 .tools/bin。数据库测试用 Compose PostgreSQL + 每测试唯一 schema，满足隔离实际数据库方案，无需再叠加 Testcontainers。

认证方案暂未由产品需求指定：定义 Authenticator / Principal 并提供默认关闭的 loopback-only 开发 token，便于完整演示。正式 OIDC/SSO/会话、CSRF、密钥管理与生产部署需后续确认；当前不得作为生产认证使用。无登录 UI、用户管理或权限矩阵。

Casbin 使用嵌入的版本化策略，进程内 SyncedEnforcer，不启用决策缓存；撤销即时生效并测试。持久化策略 adapter、角色管理、跨实例策略同步尚未启用，不能跨进程承诺即时撤销。审计以结构化日志记录请求与允许授权，尚无合规长期审计存储。

范围模型支持 ALL/ORGANIZATION/DEPARTMENT/OWN/CUSTOM 的 SQL，演示 Resolver 仅授予 OWN。ABAC 属性解析、对象共享和组织树管理保留扩展接口，不提前发明组织业务。RLS 和 OpenFGA 均未启用。敏感字段 internal_note 不进入响应投影。

同源代理，不开放跨站 CORS；前端不保存认证 token。公共 health/echo 不含敏感数据，进程健康与数据库 readiness 区分。生产前需正式认证、速率限制、TLS、应用最小数据库权限和部署审查；当前开发配置不冒充生产就绪。
