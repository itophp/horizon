# SQL 与迁移

继承服务规则，使用 baseline-database-change，细则见 docs/database-conventions.md。

迁移是 schema 唯一来源，已应用版本不重写；查询放 query，通过 sqlc 生成 dbgen。所有外部值参数化，租户 + 可信 DataScope 在查询层强制实施，写入用带范围的原子条件。

查询必须考虑 list/detail/update/delete/aggregate/export/batch/joins 的同等隔离与字段投影；不要 SELECT * 暴露后续敏感新增列。多步操作用同一事务 Queries。

变更运行 make generate、make check-generated 和真实 PostgreSQL test-integration，覆盖迁移、约束、越权及实际受影响的事务/并发行为。测试环境不可用时明确失败，不跳过或替换内存实现。RLS 仅在实际角色/连接池上下文验证后按需启用。


多部门模型见 [ADR 004](../../../docs/decisions/004-department-memberships.md)。账号记录范围必须考虑主/附加归属；角色成员去重，账号与所有部门角色写入共用事务。复合外键保持租户与公司部门一致性，临时授权按部门绑定且移除归属立即失效。

治理表 gov_* 使用拥有 scope_key 和资源 revision；草稿发布/回退/审计同事务，稳定字典项不物理删除，引用用复合租户外键。文件历史绑定不可变存储 profile；临时状态与 gov_events 同事务，队列恢复用租约/唯一键。见 [ADR 005](../../../docs/decisions/005-system-governance.md)，不要把后台清理和投递任务当作可忽略租户/权限的入口。

公共配置按 [ADR 007](../../../docs/decisions/007-global-governance.md) 统一为 workspace:0。迁移既有字典时保留 ID/代码/引用，旧组织设置不再解析，不移动依赖 scope 加密的秘密；历史只保留追踪，不恢复旧范围。消费公共能力先用 baseline-shared-capabilities。

人工通知见 [ADR 008](../../../docs/decisions/008-notification-center.md)：群发主体、接收快照、文件绑定与投递同事务，按账号去重及内容摘要幂等。组织目标只在全部有效成员受信任发送范围覆盖时返回；通知/文件读取用接收账号和有效归属、绑定关系过滤。文件绑定与删除共享事务锁，避免并发发送后附件被删除。

定时发布 gov_settings_schedules 使用 pending 部分唯一索引、SKIP LOCKED 行锁与 savepoint；配置/版本/审计/任务完成在同一外层事务，崩溃后 pending 可恢复。取消与领取通过同一行锁串行化，版本冲突终止而不自动重新基线。见 ADR 010。
