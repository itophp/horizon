---
name: baseline-quality-gates
description: "Select and run appropriate frontend, Go, database, generation and end-to-end checks, diagnose failed tests, or prepare a completed change for review. Use the project quality profiles and report real results."
---

# 测试与交付门禁

先读 [测试矩阵与执行节奏](../../../docs/testing.md#测试执行节奏) 和目标路径 AGENTS.md。普通功能完成、进度汇报和聊天结束不触发测试；连续开发累计受影响模块与待验命令，在明确阶段验收或提交/PR 前统一执行一次 ci-fast 并追加相关检查。复现失败或高风险变更先做最小定向验证。同一批次已通过且输入未变的检查复用，不重复执行。下列各层要求用于选择覆盖，不要求逐功能运行；不能以文档或截图目测替代断言。

- 文档/规则：make check-rules；检查链接和适用范围。新 Skill 另用官方 quick_validate.py 验证，不能依赖该全局脚本作为仓库 CI 前提。
- 前端：lint/typecheck、受影响 Vitest、i18n key、build。交互用 RTL + user-event；关键流程用普通 Playwright 测试。
- Go：gofmt、go vet、相关 go test；HTTP 编解码与状态用 httptest，依赖用小接口/Fake。
- DSL/SQL/SDK：make generate → 实现/消费者 → make check-generated → 两端编译和相关测试。生成检查与浏览器验收串行。
- SQL/迁移：make test-integration，真实隔离 PostgreSQL，不因本机环境不支持而吞掉/静默跳过。
- 权限：make test-security，覆盖允许与拒绝、范围外直接 ID、跨租户和撤销；授权 UI 隐藏不是安全证据。
- 跨层批次验收：快速门禁加相关数据库/安全/E2E 文件；发布候选、明确全量验收或 CI 才执行默认 make ci。计划跑 ci 时不先重复执行其子门禁；test-security 已包含 integration。环境不足则列出失败/未执行项，不报告全量通过。

读取完整退出码、断言、堆栈和日志，先复现，再修源代码或夹具；重跑失败集，再扩大到受影响模块。同一根因连续三轮未解决时暂停该部分自动修改，汇报证据和判断选项；已识别的新根因可以单独验证，不盲猜更多补丁。

不删除断言、不关闭静态检查、不用 any、不无限重试。诊断明确是环境预算不足时可调整有界超时并说明证据；不添加固定 sleep。数据和端口隔离，优先 role/label 定位；失败保留 screenshot/trace/console/network。

报告命令、退出码/结果、覆盖的行为、未运行项、环境和剩余风险；“配置已写入”不等于“远程 CI 通过”。文档变更不强制运行无关慢测试。


## 系统治理能力

治理功能按 [ADR 005](../../../docs/decisions/005-system-governance.md) 选择 tests/governance*、SMTP 隔离服务器、文件临时目录和 e2e/governance.spec.ts。必须验证原子发布/冲突/单次证明/回退、公共配置跨部门一致、本人偏好隔离、旧组织目标拒绝与原资源数据隔离、动态源越权/禁用历史、文件路径/伪类型/旧版本读取、队列重复/重试/撤权/无邮箱；生成检查先于浏览器验收。


公共设置/字典的最新范围按 [ADR 007](../../../docs/decisions/007-global-governance.md)，取代 ADR 005 的组织配置继承/字典覆盖：统一工作空间管理，个人白名单偏好独立。管理要求正式权限及共享资源 ALL 范围，旧部门授权不得自动扩权；文件、消息、动态字典来源仍执行原资源数据范围。业务需要设置项、字典选项、上传、读取、下载或通知时，先使用 [baseline-shared-capabilities](../baseline-shared-capabilities/SKILL.md)，复用现有服务/SDK，不复制实现。

通知中心及 Markdown/文件引用按 [ADR 008](../../../docs/decisions/008-notification-center.md)：独立正式发送权限、服务端全部组织范围/去重/幂等，原始 HTML/危险 URL 不执行；收件人附件入口用本人+有效归属+消息绑定 SQL 授权，不能扩大通用文件范围。变更同步公共能力 Skill 并验证收发、越权、失效归属和附件下载。

通知/治理可用 playwright.governance.config.ts 的独立端口、构建及报告目录验收，仍使用完整生产应用、真实 Go 与隔离 PostgreSQL。选择此配置只代表所选相关用例通过，不代表默认全量浏览器集合；命令与边界见 docs/testing.md。

设置定时发布按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：正式 publish/ALL，敏感单次 settings.schedule 证明在排期事务消费，到期重建原部门 Principal 并复查权限/摘要/依赖/版本；行锁与 savepoint 保证配置和结果原子提交，取消/重启/并发不能重复发布。UI 明示设备时区并转 UTC，使用生成 SDK；后台不借旧会话或临时权执行。修改时同步公共能力与权限规则并覆盖真实数据库、状态和取消测试。
