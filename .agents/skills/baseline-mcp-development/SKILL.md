---
name: baseline-mcp-development
description: "Design, implement and maintain MCP tools for application features when functionality is added or requirements change. Assess exposure, select operations and preserve authorization and compatibility."
---

# 为业务功能创建和维护 MCP 工具

在功能开发或需求变动时使用本 Skill；先读根到目标路径的 AGENTS.md、[MCP 方案](../../../docs/mcp-service-plan.md)和现有实现。方案中待实现的服务不能假定已经可用。业务工具开发按涉及层加载 Goa、数据库、安全、Web、国际化及质量门禁 Skill。

1. **确认暴露范围**：集中询问是否接入 MCP、哪些业务操作、目标客户端、只读或写入和确认方式。沿用用户已明确的选择；未回答时继续独立开发并记录待定，不默认暴露。已有 MCP 工具受业务变更影响时仍需维护，不能因未选择新工具而跳过。
2. **梳理业务用例**：检查源 Goa 契约、应用服务、权限、数据范围、字段投影及已有 MCP 工具。按用户要完成的任务选择粒度，例如搜索、详情、创建；避免将每个 HTTP 端点机械地变成工具。记录未暴露的操作和原因。
3. **设计工具契约**：每个工具记录稳定名称/版本、用途与适用条件、输入 schema、必要字段和限制、输出字段与分页、错误码、resource/action、可信部门/DataScope、Token 允许集合、读写风险、幂等和确认方式。描述应帮助客户端选择工具；不把敏感字段或内部实现细节暴露给模型。示例：订单搜索工具使用业务搜索条件和分页，返回有权限的最小摘要；订单详情按 ID 调用时仍须独立检查范围。
4. **实现和接入**：只为已选择能力注册工具；复用现有应用服务、Goa 源类型及 Repository，必要时写显式映射并用契约测试防漂移。不得直接访问裸 dbgen、复制业务规则或信任输入的租户/owner。tools/list 的过滤不能替代 tools/call 的服务端认证、Casbin 和 SQL 范围检查；权限取当前正式权限、Token 固化工具集合和启用状态的交集。写操作须符合业务事务、幂等和服务端确认要求。
5. **同步变更**：业务字段、权限、流程变化时检查所有受影响工具、schema、测试及客户端运行时使用指南；破坏性变更通过新版本或弃用流程处理。工程开发 Skill 与客户端运行时 Skill 分开。现有服务未实现时记录依赖和待验项，不声称工具已接入。
6. **验证交付**：按测试节奏验证成功、无效输入、无权限、跨租户/范围外 ID、敏感字段、撤权/撤销、分页和相关写入/确认负向路径；协议端到端验证实际注册、发现和调用。生成变更先 make generate/check-generated，再做客户端/浏览器验证；按风险运行真实数据库与安全检查。交付报告工具清单、契约和授权、运行结果及未验证项。


当前实现参考 [ADR 011](../../../docs/decisions/011-remote-mcp.md)：在 internal/mcp/registry.go 用操作专用输入注册工具，在 http.go 适配既有生成用例类型；不要让所有工具共享无约束参数袋。对外业务字段仍以 Goa 源为准。扩展工具时同步允许集合、当前权限过滤、运行时指南和 test-mcp；用户已选工具不会因未来新增工具自动扩权。普通工具开发不需要修改客户端安装器或用户全局配置。
