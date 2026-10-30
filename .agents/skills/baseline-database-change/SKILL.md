---
name: baseline-database-change
description: "Change PostgreSQL schema, Goose migrations, sqlc queries, pgx repositories, transactions or data-scope filtering. Verify changes against isolated real PostgreSQL and regenerate from SQL sources."
---

# 数据库变更

先读 [数据库目录规则](../../../services/api/db/AGENTS.md) 和 [数据库约定](../../../docs/database-conventions.md)。

修改前检查已有迁移与 query，明确数据生命周期、约束、租户归属、拥有者语义、查询范围和字段投影。新业务表由实际需求驱动，技术示例表不能直接冒充正式模型。

- 已应用迁移只追加版本；评估锁表、默认值、回填、索引以及已有数据兼容性。Up/Down 不自动等于生产可安全回滚；不可逆操作要明确记录，不自动执行破坏性生产变更。
- 表和约束在 migrations，参数化查询在 db/query。改 SQL 后 make generate，手写 Repository 只消费 dbgen。禁止直接修补 sqlc 输出。
- 所有租户数据的 list/get/update/delete/aggregate/export/batch/joins 都在 SQL 层实施可信 tenant + scope，写入优先带范围的原子查询；不先无约束查出再过滤。
- 通过 pgx 事务将多步操作绑定同一 DBTX/Queries；明确提交、回滚、并发和幂等策略，禁止事务部分逻辑误用池连接。
- CUSTOM/组织/部门集合来自后端解析，参数化传入；需要下级部门时明确集合来源。不把请求 tenant/owner 当授权依据。
- 只选择响应需要的字段。RLS 仅在需求确定、非 BYPASSRLS 角色和连接池事务上下文均验证后引入。

配置 TEST_DATABASE_URL 指向隔离本地/CI 数据库；make db-up 后执行 make test-integration。使用唯一 schema / 数据库并清理，验证迁移往返、约束、范围外 ID、跨租户读写删除及受影响的回滚/并发场景。数据库不可用时报告未通过，不换内存假库冒充 SQL 验证。

完成 make check-generated、Go 静态/单元检查；有 API/SDK 影响时再按矩阵检查前端与端到端。

账号多部门变更遵循 [ADR 004](../../../docs/decisions/004-department-memberships.md)。主/附加归属均参与账号 SQL 范围，部门角色复合外键同租户、公司部门一致；成员统计按账号去重。归属/角色写入共用事务，删除归属使临时授权永久失效。修改权限 SQL 必须联动 baseline-security 和安全/目录规则，补跨租户、范围、回滚及上下文隔离回归。


## 系统治理能力

治理表与队列见 [ADR 005](../../../docs/decisions/005-system-governance.md)：拥有 scope_key、版本锁、覆盖/快照/审计同事务，字典稳定 ID 和复合租户引用，文件不可变存储 profile，临时状态事件事务一致、租约及唯一键保证恢复幂等。真实测试覆盖冲突、回退、范围及队列重试，不跨池误用事务。


公共设置/字典的最新范围按 [ADR 007](../../../docs/decisions/007-global-governance.md)，取代 ADR 005 的组织配置继承/字典覆盖：统一工作空间管理，个人白名单偏好独立。管理要求正式权限及共享资源 ALL 范围，旧部门授权不得自动扩权；文件、消息、动态字典来源仍执行原资源数据范围。业务需要设置项、字典选项、上传、读取、下载或通知时，先使用 [baseline-shared-capabilities](../baseline-shared-capabilities/SKILL.md)，复用现有服务/SDK，不复制实现。

设置定时发布按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：正式 publish/ALL，敏感单次 settings.schedule 证明在排期事务消费，到期重建原部门 Principal 并复查权限/摘要/依赖/版本；行锁与 savepoint 保证配置和结果原子提交，取消/重启/并发不能重复发布。UI 明示设备时区并转 UTC，使用生成 SDK；后台不借旧会话或临时权执行。修改时同步公共能力与权限规则并覆盖真实数据库、状态和取消测试。
