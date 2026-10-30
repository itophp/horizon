# 测试与质量门禁

## 测试执行节奏

采用“连续开发累计变更 → 阶段验收集中验证 → 发布/CI 全量回归”。测试覆盖要求保留，执行时机按批次安排；单个功能完成、一次回复或聊天结束都不自动构成验收节点，也不必为是否运行测试反复询问用户。

- 普通开发：完成实现并按风险补充测试，记录受影响模块和待验命令，继续后续功能；不自动启动 ci-fast、数据库集成或 E2E，也不要求每完成一个普通功能就运行单元测试。
- 提前定向验证：复现缺陷、排查编译错误，以及认证/授权/数据范围、敏感字段、迁移/事务、契约兼容等高风险改动，应在继续依赖该行为的开发前运行最小相关检查。SQL 行为使用真实隔离 PostgreSQL，权限覆盖允许与拒绝；这不触发无关模块或全量回归。
- 阶段验收：用户要求验收/测试，或准备提交、创建 PR 时，对累计代码变更统一运行一次 make ci-fast；涉及 SQL/事务追加 test-integration，涉及安全追加 test-security（已包含 integration，不再单独重复），关键跨层流程选相关 E2E 文件。仅文档/规则运行 make check-rules。普通跨层功能完成本身不触发 make ci。
- 完整回归：发布候选、用户明确要求全量验收或 CI 使用默认 make ci；保留现有 CI 全量覆盖。若已决定运行 make ci，不先另跑一遍 ci-fast、integration 和 E2E。
- 结果复用：记录命令、检查覆盖的变更范围、结果及此后修改。同一批次中，输入未变且未受后续改动影响的已通过检查直接复用；源码、测试、契约、依赖、配置或相关环境改变时重跑受影响检查。跨会话无可靠记录则不能假定已通过。失败先修复并重跑失败集，只有新的影响或验收节点才扩大范围。
- 延后验证的回复明确写“实现已完成，尚未验证”，简列累计待验模块/命令；不能宣称测试通过或可发布。阶段验收报告区分本次执行、复用证据、未执行项和风险。

例如：连续调整账号表单、角色列表和通知布局时，先累计三个模块的待验项，阶段验收统一跑快速门禁和相关浏览器文件；若中途修改部门授权范围，则立即定向验证该范围的允许/拒绝，不顺带启动其他浏览器流程。

## 可执行 profiles

| 命令 | 作用 | 何时执行 |
|---|---|---|
| make check-rules | 规则/Skill 元数据、目录范围、引用、命令路由及检查器正/负路径 | 文档/规则变更；所有代码交付前 |
| make ci-fast | check-rules + 格式 + ESLint/Go vet + TypeScript + Vitest/Go/HTTP + i18n keys + 生成一致性 + 构建 | 代码批次阶段验收统一执行一次；不需要 DB/浏览器 |
| make test-integration | 隔离真实 PostgreSQL、迁移、约束、scope、事务与并发 | SQL、Repository、事务或权限范围改动 |
| make test-security | 认证/授权/config/scope/HTTP 单元负向用例 + 数据库范围/字段投影/撤销 | 受保护端点、身份、权限、租户、敏感字段及配置边界变更 |
| make test-e2e | 真实 Go + 生产构建 preview 的关键流程，失败 screenshot/trace 和诊断 | 完成关键用户流程、路由/语言持久化或跨层功能 |
| make ci | ci-fast → integration → E2E，顺序执行 | 发布候选、明确全量验收和 CI |

表中“何时执行”表示覆盖要求，执行时机统一遵循上方批次规则，不表示每个功能都立即执行。文档-only 批次运行 check-rules 并审查语义，不为无关改动启动慢测试。前端快速迭代可先运行受影响 Vitest，后端先运行相关包 go test。纯布局变更用可观察交互/可访问性验证，不编写只镜像 CSS 常量的测试。

## 测试设计

- Vitest 验证格式化、状态、错误映射；RTL + user-event 验证表单和可见交互。API 数据来自生成类型，允许隔离网络的组件 Fake；不能把它算成端到端验证。
- Go 单元用小接口/Fake；HTTP 用 httptest 验证真实生成路由、状态和编解码。生成代码重点测一致性、编译及手写集成，不逐行镜像。
- SQL/迁移/约束/事务用真实 PostgreSQL：TEST_DATABASE_URL 必填、唯一 schema 和清理、不可连生产。没有服务不能静默跳过。
- 权限新增先列允许与拒绝矩阵，覆盖未认证、无操作权限、跨租户、范围外直接 ID、适用的更新/删除/批量/导出/关联及敏感字段/撤销。新增入口必须有它自己的负向测试。
- Playwright 优先 role/label，隔离上下文/数据、明确 locale/时区/视口、等待可见完成状态。禁止固定 sleep、共享前置用例和无限重试。未检查的场景与显式排除项写进报告。

## 失败处理和环境

保留原始退出码、失败断言、堆栈和日志；先复现最小失败集，修源/夹具，再验证模块与交付 profile。禁止删除断言、关闭检查、any、吞异常或伪造结果。同一根因三轮无解就停止盲改并汇报证据。

浏览器预算不足必须以 Trace/网络/截图证据解释，使用有界状态等待；不通过无限延长、固定 sleep、重试或屏蔽网络错误来掩盖真实缺陷。配套 Chromium 通过 Playwright 安装；本机 Chrome 可用 PLAYWRIGHT_CHANNEL=chrome。生成检查与 E2E 不并发，避免产物变化干扰应用。

CI 先运行 ci-fast，再数据库，再浏览器。依赖/工具缺失应失败；失败报告保留 artifacts。`make ci` 本地通过不等于 GitHub Actions 已执行。

## 规则与覆盖的边界

check-rules 能检查结构完整与引用，不理解自然语言是否写对，也不能保证 Agent 服从。lint/key 检查能阻挡明确的硬编码或翻译遗漏，不能发现所有运行时拼接和语义错误。安全行为由负向测试与审查共同保障；没有可靠自动化实现的威胁评估/字段分类必须明确人工审查，不能写成“已自动保证”。

当前 rules profile 还执行 ESLint 项目规则的行为测试：允许翻译函数和技术属性；拒绝 JSX 静态可见文案、占位/辅助名称硬编码，以及 app/features/components 中绕过 SDK 的直接 fetch。它不能证明间接封装、动态消息和权限设计正确，仍要审查。

身份端到端服务使用 cmd/e2e：TEST_DATABASE_URL 必填，创建唯一 schema、迁移、测试专用管理员，并在退出时清理。不会初始化或覆写开发工作区。浏览器验收包含登录保护、主题刷新保持、退出、组织/部门/角色创建、账号关联和独立范围保存、编辑与删除；后端集成覆盖六类范围、越权 ID、跨工作区、CSRF、停用和密码重置。测试固定密码只用于隔离 schema。


多部门回归覆盖 tests/memberships_test.go 的真实 HTTP + PostgreSQL：角色/覆盖隔离、SQL 成员匹配、非法切换/CSRF、跨租户及持久授权边界、临时权限部门绑定、移除/重新加入不复活；UI 使用 Playwright 验证多部门编辑与当前部门切换。

系统治理的跨层回归位于 tests/governance_test.go、governance_smtp_test.go 和后续 scope 回归；附件路径/伪类型和密文绑定另有单元测试。SMTP 测试服务与附件目录必须隔离，不能发送真实管理员邮件。关键设置/字典/文件/偏好流程在 e2e/governance.spec.ts 通过生产构建与真实后端验收。存储切换、一次性凭据、版本冲突、原子发布、动态源越权、禁用历史及跨部门缓存隔离须随行为变化扩展，不把“组件渲染”作为验收替代。

公共配置变更遵循 ADR 007：验证多公司/部门切换时系统参数与静态字典相同、本人偏好独立，旧组织目标/字典覆盖拒绝，旧受限管理权限不自动扩为 ALL。governance_global_migration_test.go 验证迁移保留字典 ID/代码/引用且不搬移旧组织设置；governance_scope_test.go 继续验证文件与跨租户隔离。公共消费功能按 baseline-shared-capabilities 选择现有消费者/选项/附件的真实允许与拒绝用例。

通知中心按 ADR 008 覆盖混选组织/账号去重、范围外组织拒绝、重复发送幂等/内容冲突、偏好/失效归属、绑定附件读取及跨租户/未绑定文件拒绝。Markdown 组件覆盖 HTML/脚本 URL/未绑定引用、GFM 与安全图片；真实浏览器从编写/上传/预览/发送走到收取/下载/投递记录，SMTP 仅隔离测试服务器。

`E2E_ARGS` 可显式选择相关浏览器文件，例如 `make ci E2E_ARGS="e2e/governance.spec.ts e2e/notifications.spec.ts"`。默认 `make ci` 仍跑全部浏览器用例。使用该参数交付时必须报告选择的文件和未重跑集合，不能宣称全量浏览器通过；本机已有无关用例总预算超时的诊断保留在 verification.md。

## WebMCP 验证

快速门禁包含工具参数、生命周期、元数据、重复名称和路由清单一致性测试。`make test-webmcp-e2e` 使用真实 MCP 客户端、桥接和浏览器验证连接、模块/页面上下文、账号筛选和撤销；`make ci` 在生成检查及数据库集成测试后、完整浏览器集之前运行此专项测试。它不依赖模型，也不能代替用户交互或后端授权测试。新增工具应验证正常及拒绝分支、页面状态与实际数据结果；保留真实点击、可访问性和后端范围测试。Agent 模型评测应作为独立的可选评测记录工具选择、参数、业务结果和失败原因，不能把概率性结果当作确定性门禁。

`pnpm webmcp:test` 已纳入 `make ci-fast`：真实 loopback HTTP 验证 Bearer、Origin、Host、有效会话和安装客户端白名单；临时 home 验证配置合并、备份、坏配置拒绝与技能安装。Vitest 验证严格参数、脱敏错误、权限导航和撤销旧句柄，以及设置页真实点击到安装传输。`e2e/webmcp.spec.ts` 启动隔离 WebMCP 18794、Go 8099、preview 5179，使用真实 MCP Client 读取/调用页面工具并检查路由卸载/断开；不在 E2E 写用户全局客户端配置。`WEBMCP_STATE_DIR=.local/webmcp-e2e` 隔离测试配对状态。

治理/通知可用 `make test-e2e E2E_ARGS="--config=playwright.governance.config.ts"` 独立验收。该配置仍构建完整应用并运行真实 Go/隔离 PostgreSQL，使用 5183/8101、独立 .local/governance-preview、governance-results 和 governance-report，不覆盖默认端口/构建/报告，也不启动无关连接测试服务。E2E_ORIGIN 由配置传给测试，Origin/CSRF 校验保持真实。此配置覆盖所选治理、通知和定时发布用例，不代表默认全量集合已通过。

设置排期的真实数据库用例在 settings_schedules_test.go，覆盖未到期、取消、重复/并发、敏感用途证明、版本冲突、会话过期、范围/账号失效、单连接文件校验与真实 worker 循环。界面由 settings-schedules.test.tsx 和 e2e/settings-schedules.spec.ts 验证；独立配置命令 `make ci E2E_ARGS="--config=playwright.governance.config.ts e2e/settings-schedules.spec.ts"`，仍须区分所选用例与默认全量。数据库测试推进隔离记录的 due 时间，不写开发库测试排期。

WebMCP 专项 E2E 还验证 `notifications/tools/list_changed` 从浏览器经过官方代理、stdio 到达真实 MCP Client，确保动态页面工具的目录变化可被客户端发现。
