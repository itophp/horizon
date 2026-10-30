# 数据库约定

PostgreSQL 18.6 开发容器仅绑定 127.0.0.1:54329；示例账号只用于本地。生产连接必须独立配置、启用 TLS 和最小权限。

采用 Goose 版本化 SQL，嵌入 migrations/*.sql，通过独立 cmd/migrate 执行；API 不自动迁移。新迁移只追加，包含 Up / Down 及可审查的风险。sqlc 从同一迁移目录读取 schema，从 db/query 生成 pgx/v5 查询。

technical_demo_records 是显式技术演示表，不是正式业务实体。所有 list/get/update/delete SQL 共享租户条件及 scope 条件；更新删除在原子 SQL 中检查范围。查询只投影 id/label，不读取 internal_note。禁止通过客户端指定当前 tenant 或 scope。

技术演示的 ScopeResolver 只授予 OWN。正式 IAM 的 iam_organizations、iam_departments、iam_accounts、iam_roles、iam_account_roles、iam_sessions、iam_login_attempts 由 00002_identity.sql 创建；复合外键约束工作区、公司和部门的一致性。下级部门范围通过服务端递归 SQL 解析，角色/账号的有效范围应用于真实查询。会话仅存令牌摘要，账号写入、角色关系、密码重置和会话撤销在事务内提交。初始化使用事务级 advisory lock。用户名全库唯一；部署多个工作区时不能重复使用同一个登录名。

集成测试必须显式设置 TEST_DATABASE_URL，创建唯一 schema 并把 pool 和 Goose 的 search_path 绑定该 schema；最终清理。覆盖 Up/Down/Up、幂等 Up、五类范围、直接 ID 越权、跨租户修改删除、字段投影、回滚、并发及约束。没有 TEST_DATABASE_URL 就失败，不将跳过算通过。不要给该变量配置任何生产数据库。

RLS 当前未启用；启用前必须定义非 BYPASSRLS 应用角色、事务内上下文、连接池复用及完整 CRUD 负向测试。当前安全边界是可信身份、Casbin、Repository 约束与 SQL 投影，不能以管理员数据库账号的 RLS 测试代替实际角色验证。

00003_temporary_access.sql 保存临时申请与审批/撤回/撤销审计。原始请求和最终授权分开保存，相对范围批准时固定解析，每次认证按数据库时钟排除到期记录。审批使用带 tenant、操作者与 pending 状态的原子更新，防止并发重复批准。申请人与审批人的可信来源、最小目录字段投影和批准范围的正式授权上限见 ADR 003。


00004_department_memberships.sql 为账号增加附加部门/部门角色与会话当前部门，保留原有主部门与策略。临时请求回填原主部门并以可信部门绑定生效。完整关联、投影、事务和回滚边界见 [ADR 004](decisions/004-department-memberships.md)。
