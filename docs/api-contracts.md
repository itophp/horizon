# API 契约

1. 阅读 design/design.go、当前生成接口及实现。
2. 修改 Goa DSL 的 Payload / Result / HTTP / Errors，字段 Required 与 Validation 必须明确。
3. make generate：相同模块图中的 Goa → OpenAPI；锁定 sqlc 生成 SQL；锁定 Orval 输出 hooks 与模型，最后统一格式化生成 SDK。
4. 在 internal/app / internal/identity、Repository 和 internal/transport 实现逻辑，在前端使用生成的 hooks。
5. make check-generated、make lint、make typecheck、make test、make build；改数据库时额外 test-integration，完成流程后 test-e2e。
6. 审查生成差异。删除字段、缩窄输入、改变授权范围都要报告兼容性影响。

GET /api/health 返回存活状态。GET /api/echo?name=… 要求 1–80 个字符，返回同一 name 和 RFC3339 UTC 时间点。演示记录 API 为租户/所有者隔离的持久化示例。

前端 request 统一 fetch、同源 credentials、10s 超时和 AbortSignal。查询至多重试 1 次，echo 为显式不重试；写操作不自动重试。响应保留 data / status / headers，与 Orval fetch 输出匹配。HTTP 错误映射为 invalid/unauthenticated/forbidden/not_found/conflict/rate_limited/unavailable/internal，由资源文件本地化，不显示服务端内部详情。

正式后台契约位于 design/identity.go：公开 POST /api/auth/login，受保护 GET /api/auth/me、POST /api/auth/logout、GET /api/admin/snapshot，以及 accounts / organizations / departments / roles 的创建、更新、删除。Cookie 在生成安全契约中声明，Set-Cookie 由生成响应传输映射，登录 JSON 不包含会话令牌。Snapshot 按每类 read 权限和 SQL 范围返回可见数据，没有列表权限时返回对应空数组。

统一 transport 在生成路由前验证数据库会话。所有写请求（含登录）需合法同源 Origin；登录以外还需 X-CSRF-Token，SDK request 自动附带内存 nonce。认证 token 不进入 localStorage；语言和主题可持久化。API 写入由服务授权并在 SQL 中检查范围，不能直接相信 Cookie 以外的客户端身份声明。技术 demo 保留独立 loopback token 适配器。

两级权限目录由 PermissionDefinition 输出，前端不复制授权代码。临时授权接口为 GET/POST `/api/admin/temporary-access`，以及 POST `/{id}/review`、`/{id}/cancel`、`/{id}/revoke`。申请需要有效会话，审批需要正式 temporary:approve，最终操作和范围不得超过审批人的正式上限；原始请求与最终决定分别保存，身份、工作区、时间和已解析范围由服务端确定。安全敏感的身份/凭证/策略与组织归属变更还要求操作者正式写权限/范围，详见 ADR 003。


部门授权契约见 [ADR 004](decisions/004-department-memberships.md)：AccountRecord.memberships、SessionProfile.activeDepartmentId 与可选 departmentAccess 替换对象由 Goa 生成；POST /api/auth/department 仅在服务端验证本人有效归属后切换并轮换 CSRF。
