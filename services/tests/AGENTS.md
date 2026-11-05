# 真实数据库与集成测试

继承服务规则，使用 baseline-quality-gates；SQL 变更结合 baseline-database-change。

TEST_DATABASE_URL 必须显式指向隔离本地/CI PostgreSQL，缺失/不可用即失败。每次测试创建唯一 schema/数据库，迁移和连接池使用同一 search_path，结束清理，不使用生产服务。

测试 SQL 的约束和投影，而不是在 Go 中重新实现过滤当 oracle。覆盖迁移往返、重复 Up、允许/拒绝、跨租户 ID 读写删除；事务/并发/幂等在实际改动涉及时补足。新增访问入口就补对应越权用例，不因已有 list 测试推定 export/batch 安全。

生成代码关注重复生成、编译与手写层集成，不逐行镜像测试。运行 make test-integration；安全相关测试也由 make test-security 复用。

账号远程 MCP 按 [ADR 011](../../docs/decisions/011-remote-mcp.md)：工具复用原授权用例，凭证绑定本人/部门和固化集合；敏感管理操作重新认证，身份变更撤销与延迟归属检查、客户端配对/备份及协议专项测试同步维护。禁止将安装状态当连接成功。

## 远程 MCP 动态能力目录（现行）

远程 MCP 与 AI 助手共享自动化能力发现；Token 绑定账号和部门，每次请求按当前正式权限重建目录，历史固化工具集合不再限制能力。业务调用必须经 Gateway，并保持来源、数据范围、审批和幂等检查；手动和内部专用能力不进入自动化目录。权限变更须覆盖发现、猜测调用和撤权后的拒绝。详见 docs/security.md 和 docs/decisions/011-remote-mcp.md（从仓库根解析）。
