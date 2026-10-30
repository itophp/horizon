# 真实数据库与集成测试

继承服务规则，使用 baseline-quality-gates；SQL 变更结合 baseline-database-change。

TEST_DATABASE_URL 必须显式指向隔离本地/CI PostgreSQL，缺失/不可用即失败。每次测试创建唯一 schema/数据库，迁移和连接池使用同一 search_path，结束清理，不使用生产服务。

测试 SQL 的约束和投影，而不是在 Go 中重新实现过滤当 oracle。覆盖迁移往返、重复 Up、允许/拒绝、跨租户 ID 读写删除；事务/并发/幂等在实际改动涉及时补足。新增访问入口就补对应越权用例，不因已有 list 测试推定 export/batch 安全。

生成代码关注重复生成、编译与手写层集成，不逐行镜像测试。运行 make test-integration；安全相关测试也由 make test-security 复用。
