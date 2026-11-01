---
name: baseline-quality-gates
description: "Select and run appropriate frontend, Go, database, generation and end-to-end checks, diagnose failed tests, or prepare a completed change for review. Use the project quality profiles and report real results."
---

# 测试与交付门禁

先读 [测试矩阵与执行节奏](../../../docs/testing.md#测试执行节奏) 和目标路径 AGENTS.md。普通功能完成、进度汇报和聊天结束不触发测试；连续开发累计受影响模块与待验命令，在明确阶段验收或提交/PR 前统一执行一次 ci-fast 并追加相关检查。复现失败或高风险变更先做最小定向验证。同一批次已通过且输入未变的检查复用，不重复执行。下列各层要求用于选择覆盖，不要求逐功能运行；不能以文档或截图目测替代断言。

**E2E 暂停（用户现行指令，优先于下文与 docs 中的历史描述）**：默认不启动任何浏览器验收，包括不带 `E2E_ARGS` 的 `make test-e2e`、`test-e2e-parallel`、`test-webmcp-e2e`、`test-mcp-e2e` 和默认 `make ci`，也不触发远程 CI 的浏览器阶段。确需旁证时只跑改动模块对应的单个 spec：按 `scripts/e2e-changed.mjs` 的映射选取，用 `E2E_ARGS="<spec>"` 或 `make test-e2e-changed` 执行，映射未命中就不跑浏览器并记未覆盖。全量浏览器集合必须在当轮得到用户明确授权；未授权时汇报写“E2E 未执行（按当前指示暂停）”。

- 编排/Schema：make check-workflows、check-generated；完整生命周期参考用 make test-orchestration-example（真实 PostgreSQL + Eino + 确定性测试模型），不把它描述成真实模型或浏览器验收。
- 文档/规则：make check-rules；检查链接和适用范围。新 Skill 另用官方 quick_validate.py 验证，不能依赖该全局脚本作为仓库 CI 前提。
- 前端：lint/typecheck、受影响 Vitest、i18n key、build。交互用 RTL + user-event；关键流程用普通 Playwright 测试。
- Go：gofmt、go vet、相关 go test；HTTP 编解码与状态用 httptest，依赖用小接口/Fake。
- DSL/SQL/SDK：make generate → 实现/消费者 → make check-generated → 两端编译和相关测试。生成检查与浏览器验收串行。
- SQL/迁移：make test-integration，真实隔离 PostgreSQL，不因本机环境不支持而吞掉/静默跳过。
- 权限：make test-security，覆盖允许与拒绝、范围外直接 ID、跨租户和撤销；授权 UI 隐藏不是安全证据。
- 跨层批次验收：快速门禁加相关数据库/安全检查；浏览器只在需要旁证时跑映射命中的单个模块 spec，全量集合与默认 `make ci` 需明确授权。计划跑 ci 时不先重复执行其子门禁；test-security 已包含 integration。环境不足则列出失败/未执行项，不报告全量通过。

读取完整退出码、断言、堆栈和日志，先复现，再修源代码或夹具；重跑失败集，再扩大到受影响模块。同一根因连续三轮未解决时暂停该部分自动修改，汇报证据和判断选项；已识别的新根因可以单独验证，不盲猜更多补丁。

不删除断言、不关闭静态检查、不用 any、不无限重试。诊断明确是环境预算不足时可调整有界超时并说明证据；不添加固定 sleep。数据和端口隔离，优先 role/label 定位；失败保留 screenshot/trace/console/network。

报告命令、退出码/结果、覆盖的行为、未运行项、环境和剩余风险；“配置已写入”不等于“远程 CI 通过”。文档变更不强制运行无关慢测试。


## 检查能力复用决策

对照 [能力发现与复用](../../../docs/agent-workflow.md#能力复用优先先发现再复用最后新增) 核对新增动作是否已有可用能力、是否重复业务 SQL/校验或按渠道复制实现；新增/扩展须有语义与兼容性理由。复用路径同样验证当前入口的权限/范围、敏感字段、失败及幂等；契约变化验证已有消费者，不把旧模块测试通过当作新入口已验收。

## 校验架构决策是否真正落地

涉及业务流程/AI/人工等待时，先读取 [开发工作流的验收项](../../../docs/agent-workflow.md#交付前需要证明的行为)，对照功能开发时的架构映射选测试，不只验证返回 200 或状态最后变成成功。

- 验证实际入口启动已注册业务 Workflow；要求 ADK 时验证原生 Runner、允许工具及结构化结果校验。固定分类 stub 只能证明分支，不能证明 ADK；ai_runs/Layer/流程目录存在也不能证明业务已接入引擎。
- 使用真实 Eino 与可控模型覆盖安全/问题/不确定/无效配置/模型失败，隔离 PostgreSQL 验证状态/任务/Outbox；等待人工时重建引擎，确认同一运行恢复而非重新创建或重复发布。
- 覆盖自审、跨租户、未投稿草稿可见性、撤权、重复/旧版本决定、重提/取消，以及人工队列和作者可见驳回原因；有页面时增加针对性的 RTL 交互验证，浏览器仍遵守现行暂停规则。
- 分别验证通用 Agent、远程 MCP、页面 WebMCP 的工具发现/调用边界，以及正文不会借模型回答或检查点进入未授权持久化路径。参考示例通过不代表新业务已通过。
- 汇报已验证/未验证/框架缺口与真实模型限制；未实现分支或恢复机制不能用 Schema 检查通过替代行为验证。纯规则文档变更仅跑规则与作者校验，不扩大到数据库/浏览器。


## 系统治理能力

治理功能按 [ADR 005](../../../docs/decisions/005-system-governance.md) 选择 tests/governance*、SMTP 隔离服务器、文件临时目录和 e2e/governance.spec.ts。必须验证原子发布/冲突/单次证明/回退、公共配置跨部门一致、本人偏好隔离、旧组织目标拒绝与原资源数据隔离、动态源越权/禁用历史、文件路径/伪类型/旧版本读取、队列重复/重试/撤权/无邮箱；生成检查先于浏览器验收。


公共设置/字典的最新范围按 [ADR 007](../../../docs/decisions/007-global-governance.md)，取代 ADR 005 的组织配置继承/字典覆盖：统一工作空间管理，个人白名单偏好独立。管理要求正式权限及共享资源 ALL 范围，旧部门授权不得自动扩权；文件、消息、动态字典来源仍执行原资源数据范围。业务需要设置项、字典选项、上传、读取、下载或通知时，先使用 [baseline-shared-capabilities](../baseline-shared-capabilities/SKILL.md)，复用现有服务/SDK，不复制实现。

通知中心及 Markdown/文件引用按 [ADR 008](../../../docs/decisions/008-notification-center.md)：独立正式发送权限、服务端全部组织范围/去重/幂等，原始 HTML/危险 URL 不执行；收件人附件入口用本人+有效归属+消息绑定 SQL 授权，不能扩大通用文件范围。变更同步公共能力 Skill 并验证收发、越权、失效归属和附件下载。

通知/治理可用 playwright.governance.config.ts 的独立端口、构建及报告目录验收，仍使用完整生产应用、真实 Go 与隔离 PostgreSQL。选择此配置只代表所选相关用例通过，不代表默认全量浏览器集合；命令与边界见 docs/testing.md。

设置定时发布按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：正式 publish/ALL，敏感单次 settings.schedule 证明在排期事务消费，到期重建原部门 Principal 并复查权限/摘要/依赖/版本；行锁与 savepoint 保证配置和结果原子提交，取消/重启/并发不能重复发布。UI 明示设备时区并转 UTC，使用生成 SDK；后台不借旧会话或临时权执行。修改时同步公共能力与权限规则并覆盖真实数据库、状态和取消测试。
