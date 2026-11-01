---
name: baseline-feature-development
description: "Implement or change a feature in this repository; route affected layers to the relevant project skills, framework choices and verification commands. Use for feature work spanning or changing application behavior, not documentation-only edits."
---

# 功能开发路由

先读根 AGENTS.md 和 [任务工具矩阵](../../../docs/agent-tooling.md)。修改任何路径前，读取根到目标文件父目录之间的 AGENTS.md；不要假设未进入的子目录规则已经加载。

功能新增/需求变化先使用 [baseline-mcp-development](../baseline-mcp-development/SKILL.md) 集中确认 MCP 暴露操作、客户端、读写/确认范围，沿用已答选择。待定时继续独立开发但不默认开放；已有工具受影响仍需同步维护。

## 开发前必须完成的判断

完整步骤、条件提问矩阵和文章发布示例见 [Agent 开发工作流](../../../docs/agent-workflow.md)。新增/改变业务行为时读取其中“一次功能任务”“开发前需要确认的问题”“编码前的最小架构决策”；涉及 AI、人工节点或长任务时再读 Eino 与人工节点部分。不要只凭已有模块照搬实现。

1. 检查 Git、实际能力目录/实体/SQL/流程/入口和适用规则，保留已有修改；列出可观察的成功与失败条件。
2. 列出角色、触发动作、前后状态、公开时点、人工等待、失败/取消/重提。仅询问上下文不能确定且影响业务或安全的事项：谁能看/改/审、能否自审、驳回原因和重提规则、AI 的权限及材料范围、模型不确定/不可用的去向、各渠道工具暴露。合并问题、沿用已答选择；不让用户替 Agent 选择文件位置或技术库，不重复请求实现许可。
3. 在任务计划或必要的功能文档中简述架构映射：复用/新增能力及事务、实体与状态、同步/Workflow/Graph/ADK 选型及理由、Schema/生成来源、人工页面和工具允许集、验证与框架缺口。简单改动只记录相关项，不强制新增 ADR。
4. 按实际规则使用 Eino：跨请求人工等待/恢复用 Workflow；有界循环/只读重试用 Graph；受限工具决策或用户明确要求 Agent 用 ADK。固定输入的一次模型分类可以直接用模型接口，但须说明选型，不可称为 ADK。组件不强制凑齐，Layer 或 ai_runs 都不是引擎接入证据。
5. 先核对现有运行时能力。业务人工任务不等于 Gateway 风险确认；任意分支、人工信号、结构化 ADK 输出未获支持时，明确列出并补齐所需框架集成，不用普通函数、另起 goroutine 或通用审批卡悄悄替代已要求的流程。
6. 关键业务答案未定时继续独立工作，相关权限/工具不默认开放；时间经过不代表批准。实现后用真实入口、指定引擎、状态分支和恢复测试核对决策，按下文门禁交付。

## 先复用当前提供的能力

在新增能力或写业务逻辑前，执行 [能力发现与复用步骤](../../../docs/agent-workflow.md#能力复用优先先发现再复用最后新增)。搜索实际 Entries/Definition、共享装配与流程、Schema 和测试，核对版本、输入输出、权限/范围、风险/审批、幂等、事务/事件及敏感数据。优先直接复用已有动作，其次经 Gateway 组合，再考虑在原模块兼容扩展；确有独立业务缺口才新增。在架构决策中简述“需求动作 → 已有候选/版本 → 复用、组合、扩展或新增及原因”。

不能只复用框架却复制已有 SQL、校验或业务流程；页面、Agent、Workflow、MCP 不分别实现同一动作。也不为复用扩大权限、弱化事务或加纯透传层。工程复用选择由 Agent 完成，只有业务/授权含义有未决变化时询问用户；能力存在不代表每个调用渠道已获准使用。

## 按涉及层实现


把用户需求转成可观察的验收条件，确认涉及 UI、API、持久化、权限、国际化中的哪些层。只加载相关 Skill：

- 需要设置项、字典选项、文件上传/读取/下载或通知：先用 baseline-shared-capabilities 复用公共服务，再按层路由。
- 页面、表单、表格：baseline-web-development。
- 能力组合/持久化流程：baseline-capability-composition；从现有 Definition、生成的 workflow.schema.json 和完整生命周期示例开始。同步聚合用 Go Executor，跨请求等待用 Eino Workflow，循环用 Eino Graph，受限决策用 Eino ADK；先运行候选定义检查，不另造调度器。
- HTTP 契约、参数、响应、错误或安全方案：现有官方 goa-service-designer；不要另造后端 DTO 或请求类型。
- 表、查询、事务和范围：baseline-database-change。
- 新增/修改业务对象、字段、关系或状态流转：baseline-entity-model；新增能力时同时确认 `Entity`/`Entities` 与 `Transition`。
- 新文案、locale、校验消息：baseline-internationalization。
- 身份、权限、数据范围、敏感字段、导出/批量或公开接口：baseline-security。
- 选择和执行验证、修复失败、交付：baseline-quality-gates。

按源数据依赖顺序工作：SQL/迁移与 Goa DSL → 生成 → Repository/服务/传输 → SDK 消费者 → UI → 验证。没有数据库变更时不造迁移；纯 UI 不改 API；文档任务不启动无关服务。不为“完整”创建未被用户要求的业务。

先寻找现有实现和可复用组件，再扩展。每个新受保护能力声明 resource/action/可信 scope/字段投影；业务流程的所有入口（详情、修改、删除、导出、批量和后台任务）保持同等约束。

依照 [测试执行节奏](../../../docs/testing.md#测试执行节奏) 按批次累计待验模块，不在每个功能完成或每次回复时启动测试。普通开发继续实现，高风险变更或失败诊断先做最小定向验证；明确阶段验收/提交/PR 前集中执行 ci-fast 和相关专项检查，发布候选、明确全量验收或 CI 才运行默认 make ci。复用输入未变的通过结果；未验证明确报告，不把配置存在当作行为验证。

涉及认证、权限、数据范围或部门归属的行为变更，交付前同步 baseline-security、受影响的目录 AGENTS.md 与 docs/security.md / 对应决策；UI/SQL 约束变化同步对应项目 Skill。规则维护与允许/拒绝测试是功能的一部分，报告具体同步位置。多部门模型入口：[ADR 004](../../../docs/decisions/004-department-memberships.md)。


## 系统治理能力

系统设置、字典、通知、附件功能先读 [ADR 005](../../../docs/decisions/005-system-governance.md)，路由到现有治理服务与消费者。范围以 [ADR 007](../../../docs/decisions/007-global-governance.md) 为准：设置与字典工作空间共享，展示/通知白名单可有本人偏好；配置键要说明实际消费者；新增管理权限不能扩权旧角色，发布/回退/导入/投递需联动契约、事务、范围、SDK 与真实回归。
