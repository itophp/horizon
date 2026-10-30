---
name: baseline-feature-development
description: "Implement or change a feature in this repository; route affected layers to the relevant project skills, framework choices and verification commands. Use for feature work spanning or changing application behavior, not documentation-only edits."
---

# 功能开发路由

先读根 AGENTS.md 和 [任务工具矩阵](../../../docs/agent-tooling.md)。修改任何路径前，读取根到目标文件父目录之间的 AGENTS.md；不要假设未进入的子目录规则已经加载。

功能新增/需求变化先使用 [baseline-mcp-development](../baseline-mcp-development/SKILL.md) 集中确认 MCP 暴露操作、客户端、读写/确认范围，沿用已答选择。待定时继续独立开发但不默认开放；已有工具受影响仍需同步维护。

把用户需求转成可观察的验收条件，确认涉及 UI、API、持久化、权限、国际化中的哪些层。只加载相关 Skill：

- 需要设置项、字典选项、文件上传/读取/下载或通知：先用 baseline-shared-capabilities 复用公共服务，再按层路由。
- 页面、表单、表格：baseline-web-development。
- HTTP 契约、参数、响应、错误或安全方案：现有官方 goa-service-designer；不要另造后端 DTO 或请求类型。
- 表、查询、事务和范围：baseline-database-change。
- 新文案、locale、校验消息：baseline-internationalization。
- 身份、权限、数据范围、敏感字段、导出/批量或公开接口：baseline-security。
- 选择和执行验证、修复失败、交付：baseline-quality-gates。

按源数据依赖顺序工作：SQL/迁移与 Goa DSL → 生成 → Repository/服务/传输 → SDK 消费者 → UI → 验证。没有数据库变更时不造迁移；纯 UI 不改 API；文档任务不启动无关服务。不为“完整”创建未被用户要求的业务。

先寻找现有实现和可复用组件，再扩展。每个新受保护能力声明 resource/action/可信 scope/字段投影；业务流程的所有入口（详情、修改、删除、导出、批量和后台任务）保持同等约束。

依照 [测试执行节奏](../../../docs/testing.md#测试执行节奏) 按批次累计待验模块，不在每个功能完成或每次回复时启动测试。普通开发继续实现，高风险变更或失败诊断先做最小定向验证；明确阶段验收/提交/PR 前集中执行 ci-fast 和相关专项检查，发布候选、明确全量验收或 CI 才运行默认 make ci。复用输入未变的通过结果；未验证明确报告，不把配置存在当作行为验证。

涉及认证、权限、数据范围或部门归属的行为变更，交付前同步 baseline-security、受影响的目录 AGENTS.md 与 docs/security.md / 对应决策；UI/SQL 约束变化同步对应项目 Skill。规则维护与允许/拒绝测试是功能的一部分，报告具体同步位置。多部门模型入口：[ADR 004](../../../docs/decisions/004-department-memberships.md)。


## 系统治理能力

系统设置、字典、通知、附件功能先读 [ADR 005](../../../docs/decisions/005-system-governance.md)，路由到现有治理服务与消费者。范围以 [ADR 007](../../../docs/decisions/007-global-governance.md) 为准：设置与字典工作空间共享，展示/通知白名单可有本人偏好；配置键要说明实际消费者；新增管理权限不能扩权旧角色，发布/回退/导入/投递需联动契约、事务、范围、SDK 与真实回归。
