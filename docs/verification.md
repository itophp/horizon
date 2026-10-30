# 工程验收记录

下文按功能批次保留历史执行证据；早期的“尚未实现”只描述当时基线，正式身份后台的当前状态以文末记录和 ADR 002 为准。

日期：2026-10-28（Asia/Shanghai）。工作区 `/Users/selibra/www/xxh-baseline` 初始为空，已初始化本地 Git，未创建提交或远程仓库。

## 实际验证命令

| 命令 | 结果 |
|---|---|
| `CI=true pnpm install --frozen-lockfile --offline --registry=https://registry.npmjs.org` | 通过；锁文件无需更新，严格 peer 检查开启 |
| `make tools` 对应锁定版本 sqlc 安装 | 已执行 `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`，成功 |
| `make generate` | 通过；Goa / sqlc / Orval 均真实运行 |
| `make check-generated` | 通过；三个生成目录的生成前后 SHA-256 完全相同 |
| `make format-check lint typecheck test build` | 通过；含 Prettier、完整目录 gofmt、ESLint、go vet、TypeScript、Vitest、Go 测试、i18n key 和两端构建 |
| `docker compose up -d --wait` | 通过；PostgreSQL 容器 healthy，仅绑定 127.0.0.1:54329 |
| `docker compose exec -T postgres postgres --version` | PostgreSQL 18.6 |
| `DATABASE_URL=<本地开发连接> make migrate` | 通过；开发库实际执行 Goose 迁移 |
| `TEST_DATABASE_URL=<本地隔离测试连接> make test-integration` | 通过；1 个顶层测试和 6 个子测试，未跳过 |
| `docker compose config --quiet` | 通过 |
| `git diff --check` | 通过；当前新增文件尚未提交 |

前端为 4 个测试文件、7 个通过用例，涵盖 locale、fallback、插值/复数、日期/数字/货币、语言切换、基础交互、SDK 错误映射以及真实 Overview 组件的表单校验/响应渲染。Go 测试覆盖开发认证默认拒绝、loopback 限制、租户 RBAC、即时撤销、非法 DataScope 和生成 HTTP 编解码。

数据库集成验证：Goose Up/Down/Up 与重复 Up，五类范围全部保留租户条件，直接 ID 读写删除越权失败、事务回滚、并发原子修改、数据库约束、HTTP 字段投影与客户端伪造租户无效。每次创建独立 schema 并清理；没有连接任何生产数据库。未采用 RLS，因此不存在未验证却声称通过的 RLS 策略。

## 浏览器测试的环境和诊断

本机 Node 22.23.2、pnpm 10.14.0；系统 Go 仍为 1.21.8，实际项目使用经官方 SHA-256 校验安装到 .tools/go 的 Go 1.27.3，没有替换系统工具链。Go 官方代理在当前网络超时，安装使用 goproxy.cn 镜像，保留默认 sumdb 校验；没有关闭校验。

Playwright 1.64.1 配套 Chromium 下载缓慢，已中止；本地使用已安装的 Chrome 155.0.8059.86，`PLAYWRIGHT_CHANNEL=chrome`。CI 保留普通 Playwright Chromium 安装并使用同一套测试，云端 GitHub Actions 尚未触发。

浏览器测试使用 5179/8099 独立端口，避免占用现有 5173 服务。运行生产构建 + preview，通过同源代理访问真实 Go API；不用 mock。开发模式下的 StrictMode 请求取消、生成 SDK 触发 HMR、导航前未等待健康请求完成曾导致失败，已通过生产构建和明确状态等待修复，仍检查所有控制台/页面/网络错误。当前机器有明显的浏览器状态交付延迟，使用有界的 30 秒断言/120 秒测试限制，没有固定 sleep 或无限重试。

失败保存 screenshot、DOM trace 和 console/network 附件；本机不采集额外视频。测试报告在 apps/web/playwright-report，失败资产在 apps/web/test-results（均不提交）。

## 限制与后续决策

- GitHub Actions 工作流已配置；不能将本地通过等同于远程 CI 已通过。
- 认证是默认关闭的本机开发适配器，正式 OIDC/SSO/会话方案未选定；禁止把开发 token 模式部署到公网。
- 授权默认 OWN；组织树、ABAC 具体条件、策略持久化、角色管理 UI、跨实例撤销同步、长期审计和 RLS 仍属后续产品决策。
- Vite 对约 603 kB 的压缩前 JS 单块提出体积提示（gzip 约 192 kB）；未调高阈值或屏蔽警告。可以在后续按路由拆包。
- 未扩展行业模块、用户管理后台或业务表。技术演示表不是正式业务模型。

## 完整门禁执行记录

2026-10-28 已实际执行 `PLAYWRIGHT_CHANNEL=chrome TEST_DATABASE_URL=<本地测试连接> make ci`，退出码 0。快速门禁、真实 PostgreSQL 集成、桌面 API/SDK/UI/国际化流程及移动端用例全部通过（2 个 E2E）。这是当前规则补强前的完整功能基线，不等同于远程 GitHub Actions 已运行。

## 长期 Agent 规则补强

按用户新要求，持久开发规则已移除临时阶段限制；原始需求文件保持不变用于溯源。当前登记 13 份 AGENTS.md（根 1 + 子目录 12）、7 个 Skill（自产 6 + 官方 Goa 1）、8 类任务路由、8 个质量门禁入口。

实际执行结果：

- 六个项目 Skill 和官方 Goa Skill 均通过官方 quick_validate.py；作者校验使用临时隔离 Python 环境的 PyYAML 6.0.3，不新增项目运行依赖。
- make check-rules 通过：元数据/库存/本地链接/命令路由检查，以及 5 个规则检查器正负用例、3 个 ESLint 约束行为用例。
- make ci-fast 退出码 0：包含上述检查、格式化、ESLint、Go vet、TS、7 个前端测试、Go/HTTP 测试、i18n keys、重复生成和两端构建。
- TEST_DATABASE_URL=<本地隔离测试连接> make test-security 退出码 0：实际执行认证/授权/config/scope/HTTP 和 PostgreSQL 集成负向用例。
- 已核查所有 AGENTS.md 都不含临时阶段范围限制。结构校验不能证明 Skill 的自然语言语义或 Agent 绝不遗漏；任务路由、安全设计和字段分类仍需审查。
- 按用户本轮指示，没有再次执行移动端或完整 E2E；前述完整功能基线通过记录保留，远程 GitHub Actions 仍未触发。

语义路由审查：新建表单关注 RHF/Zod、i18n 和 mutation；新增 API 关注 Goa/OpenAPI/Orval；新增导出关注操作授权、SQL scope 与字段投影；仅修改翻译不要求 DB；仅修改规则不启动浏览器。框架矩阵和项目 Skill 按这些真实场景分工，而不是把全部依赖无差别加载。

## 登录、组织与权限后台验收（2026-10-29）

已实现真实持久化后台：登录/退出、账号 CRUD 与密码重置、企业/公司组织、树状部门、角色和权限矩阵。沿用 React Router、shadcn/Base UI、React Hook Form/Zod、TanStack Query/Table、Goa/OpenAPI/Orval、Goose/sqlc/pgx 和 Casbin；新增 Argon2id。开发按 feature-development、web-development、internationalization、database-change、security、quality-gates 与 goa-service-designer 的项目路由执行。Goa、sqlc、Orval 产物均由源契约/SQL 生成。

实际执行：`TEST_DATABASE_URL=<隔离本地测试连接> PLAYWRIGHT_CHANNEL=chrome make ci`，退出码 0。

- 快速门禁全部通过：13 份 AGENTS / 7 个 Skill 的规则检查，格式、ESLint、Go vet、TypeScript、6 个 Vitest 文件 / 12 个测试、Go/HTTP 测试、双语 key、重复生成一致性和前后端构建。
- PostgreSQL 集成全部通过：原有隔离/约束/回滚/并发/投影，加正式身份的六类数据范围、账号覆盖收窄角色范围、显式拒绝、范围外 ID 写入与授权升级拒绝、组织层级约束、跨工作区隔离、CSRF/Origin、角色停用、账号停用、密码重置和退出后会话失效。
- 4 个 Playwright 用例全部通过（7.0 分钟）：登录保护/验证/主题刷新保持/退出；创建组织、部门、角色和账号，分配角色与独立 OWN 范围、刷新后读取、编辑、删除；原有 SDK/真实 API/语言切换流程及原有视口检查。API 与生产构建 preview 使用 8099/5179，测试专用 schema，不使用开发管理员或浏览器接口 mock。
- 另执行过 `make test-security`，真实数据库与负向用例通过。开发库迁移及首次管理员初始化已执行，未设置通用默认密码。
- 真实浏览器已检查中文登录、概览、账号表及权限矩阵的浅色/暗色呈现；登录表单宽度约 322px，页面统计来自实际授权数据。

验收中修复两个真实问题：200 空响应的退出接口被客户端误作 JSON 解析；表格列渲染函数变化导致菜单被重新挂载。均有先复现、再修复的回归用例，最终浏览器通过退出和删除，没有移除断言、固定 sleep 或重试掩盖。中断前的失败日志/截图/trace 保留于本机临时归档；最新 Playwright 报告位于 apps/web/playwright-report。

本地通过不等于远程 GitHub Actions 已执行。当前生产包仍有约 858 kB 单块大小提示（gzip 约 268 kB），未屏蔽；SSO/MFA、自助邮箱找回、长期操作审计及部署 TLS/备份不在本次实现内，详见 ADR 002。旧验收记录中的 demo-only 认证限制仅适用于当时基线，正式后台使用数据库会话与持久化策略。

## 两级权限目录与临时授权验收（2026-10-29）

新增服务端 PermissionDefinition 目录，一级 module / 二级 function / 操作 code 保持单一来源，经 Goa / Orval 生成到页面；一级模块使用真实 rowSpan。原有权限代码兼容保留，新增独立正式 temporary:approve，默认仅内置管理员具有审批能力。

完整执行 `TEST_DATABASE_URL=<隔离本地连接> PLAYWRIGHT_CHANNEL=chrome make ci`，退出码 0。快速门禁通过：规则、格式、ESLint、Go vet、TypeScript、7 个 Vitest 文件 / 14 个测试、Go/HTTP、双语资源、重复生成和两端构建。真实 PostgreSQL 用例全部通过，7 个 Playwright 用例全部通过（10.1 分钟），包含原有管理/会话/布局/国际化流程与新增申请、审批修订和即时撤销。

新增数据库验收覆盖：无资源权限仍可申请、独立审批权限、已有 OWN 范围临时扩大并在撤销后恢复、修改批准时效/范围且保留原始申请、审批正式权限/范围上限、禁止自批和临时范围再委托、账号调部门后的固定范围、数据库时间到期、显式 deny、撤回/拒绝/撤销审计、并发审批唯一成功、跨工作区隔离。安全复核另补了临时账号编辑/创建与部门重挂不能留下永久角色、凭证或范围提权的拒绝用例；非敏感业务字段和无权限新账号创建仍允许。

更新 baseline-security、baseline-web-development 的 Skill，以及前/后端 AGENTS、安全/工具矩阵、ADR 003。两个 Skill 均通过官方 quick_validate.py；未修改上游 Goa Skill。详细制度见 decisions/003-temporary-access.md。

本地开发库已执行追加迁移 00003，没有重写旧迁移或重置既有账号/组织。新增测试使用一次性 schema，不创建开发账号或业务申请。申请/审批表单截图来自真实浏览器验收，保存在 apps/web/test-results；最新报告在 apps/web/playwright-report。

这仍是本地验证，远程 GitHub Actions 未触发；没有新增依赖。租期为批准起 30 分钟至 7 天；安全敏感的身份/策略/组织归属变更必须具备正式写权限与范围，临时授权不替代这类长期授权。


## 多部门账号与独立部门授权验收（2026-10-29）

账号保留主部门，增加同租户多公司/多部门归属，每个归属独立角色和权限覆盖。会话验证当前部门后建立 Principal；切换轮换 CSRF，前端取消旧请求并按用户/租户/部门隔离 Query 缓存。账号 SQL 范围匹配所有归属，角色成员去重；多部门的持久策略/敏感信息修改和删除要求正式权限覆盖全部归属。临时申请/批准绑定部门，移除归属后旧授权不复活。沿用 Goa/sqlc/Orval、Casbin、RHF/Zod、TanStack Query/Table 和既有 shadcn 视觉，没有新增依赖。

已更新根、前端、后端实现和数据库 AGENTS；baseline-security、baseline-feature-development、baseline-web-development、baseline-database-change 四个 Skill 均通过官方 quick_validate.py。新增全局规则：修改权限控制行为必须同步相关 Skill、目录规则和安全/决策文档及允许/拒绝测试。规则入口和语义在 ADR 004。

实际验证记录：

- `make test-security` 退出码 0，真实 PostgreSQL 隔离 schema 验证允许/拒绝、部门角色/覆盖隔离、独立 deny、范围匹配、Origin/CSRF、非法切换、同租户跨公司、跨租户/公司不匹配、正式权限边界、临时权限绑定/撤销、停用主部门的登录回退及停用附加归属清理。
- 新增迁移回归：隔离库回到版本 3，保留真实账号/角色/密码/会话及历史临时申请，再 Up 到 4 验证回填与幂等；原有完整 Up/Down/Up、约束和事务测试保留。
- 首次多部门专项浏览器用例曾因账号数量进入导航辅助名称而失败，已修复稳定辅助名称；专项重跑退出码 0（3.5 分钟），验证编辑/角色/覆盖保存与回显、普通用户切换后菜单/数据范围、刷新保持和切回清空。截图复核还修正了角色复选框被通用 span flex 拉宽的样式。
- 完整 `make ci` 中快速门禁（7 个 Vitest 文件 / 14 个测试、Go/HTTP、格式/lint/vet/TS、双语、规则、生成一致性、构建）与 PostgreSQL 集成均通过；浏览器 7 项通过、1 项在 newPage / beforeEach 初始化超时，完整命令退出码 2。Trace 显示初始化时间从约 540153ms 跳到 9203115ms，停顿约 2.4 小时，功能步骤尚未执行。失败报告和 Trace 保留在本机 `/private/tmp/xxh-multi-ci-sleep-failure`，不将该次完整命令报告为通过。
- 开发库已追加迁移 00004，并重启开发 API；通过 5180 真实预览只读确认既有管理员凭证、两级目录、当前部门及账号归属字段正常。未重置已有数据；测试账号和申请只在隔离 schema 中创建。

最终补记：

- 停用归属边界修正后的 `make ci-fast` 退出码 0，含生成一致性和两端构建；`make test-security` 退出码 0。
- 受环境停顿影响的专项使用单次 caffeinate 包装重跑，退出码 0；随后将刷新保持断言改为直接核对明确的预期部门 ID，并使用最终双语说明再次专项验证，退出码 0。未增加超时、重试或移除断言。完整轮次的 7 项通过与最终专项合计覆盖全部 8 项 E2E，属于分批验证，不把前述失败的 `make ci` 改记为成功。
- 最后仅调整文案与验收断言，`pnpm i18n:check`、`pnpm lint`、`pnpm format:check` 全部退出码 0；浏览器使用生产构建检查最新文案、角色选择样式与上下文刷新保持。最终截图在 apps/web/test-results，失败整轮报告在上述临时归档。
- 四个修改过的项目 Skill 作者校验均通过，没有改动上游 Goa Skill。规则检查另在验收文档补记后执行，结果见本次交付。

本地验证不等于远程 GitHub Actions 已执行。

## 内置管理员部门授权入口修复（2026-10-29）

账号编辑原先以 `!record.isAdmin` 同时控制附加部门入口与主部门策略编辑，导致内置管理员本人看不到添加部门。本次将归属管理与所选部门策略编辑拆开：具备既有管理权限的内置管理员本人可配置附加部门，主部门策略仍受保护；其他操作者、无权限和只读模式不开放部门策略。账号列表“配置权限”和权限管理页采用相同的本人判断，账号编辑页签改为“部门与权限”。

沿用 React Hook Form/Zod、生成 SDK、现有 shadcn 组件及双语资源；没有修改 API 契约、SQL、迁移或后端权限判定。同步 apps/web/AGENTS.md、docs/security.md、ADR 004、baseline-security 和 baseline-web-development；两个 Skill 均通过官方 quick_validate.py。

实际执行：

- `make ci-fast` 退出码 0：8 个 Vitest 文件 / 18 个测试通过（新增 4 个组件回归），包括类型、lint、格式、双语、规则、生成一致性及两端构建。组件覆盖内置管理员添加子部门归属、额外角色/权限提交、主部门仍锁定、其他操作者/无权限/只读拒绝。
- `make test-security` 退出码 0，原有真实 PostgreSQL 授权、内置管理员保护、租户及临时授权回归通过。
- Playwright 专项 `built-in administrator can edit additional department access while the primary stays protected` 退出码 0（3.0 分钟），真实隔离 API 验证账号编辑入口、列表配置权限入口、添加部门/角色/独立权限保存、主部门配置保留，以及权限管理页编辑与回显。截图在 apps/web/test-results 的 administrator-department-entry.png。
- 只读确认 5180 开发预览提供修正后的编辑组件。没有修改开发账号的部门归属，浏览器写入只发生在隔离测试 schema。

本次为前端入口修复，没有重跑完整 E2E 集合或触发远程 CI；最终规则及 lint 检查在本节补记后执行。

## 2026-10-29 系统设置、字典、通知与私有附件

按用户批准的 [正式方案](system-settings-and-dictionaries-plan.md) 执行，架构边界见 [ADR 005](decisions/005-system-governance.md)。沿用既有 Goa/OpenAPI/Orval、sqlc/pgx/PostgreSQL、React/shadcn、TanStack Query/Table、React Hook Form/Zod、i18next；未新增运行时依赖，也未修改 API/SQL/SDK 生成物作为实现源。

交付：

- 注册式设置、当前部门继承、显式空/false/0 与恢复继承、草稿/发布/版本冲突/回退/脱敏审计、五分钟单次绑定重新认证。接入品牌、语言主题、Intl、分页、账号创建/重置策略、登录锁定、绝对/空闲会话以及临时申请/审批租期消费者。收紧会话永久缩短既有到期时间，放宽不会复活旧会话。
- 平面/树形/动态字典、双语标签、稳定代码/ID、有效期/启停、部门覆盖和扩展、默认项及树校验、历史选项解析、CSV/JSON 预校验/导入草稿/发布/导出/历史/回退。动态来源复用原资源权限并投影选项字段，禁止借此导入导出实体。提供业务事务内的引用适配器。
- 私有本地附件、大小及内容检测、根目录与符号链接边界、随机键/失败清理、不可变存储配置及旧文件读取、受控品牌 Logo、鉴权下载和附件管理。仅注册 local 驱动，第三方能力通过注册接口扩展，未实现第三方存储或自动搬迁。
- 事务临时权限事件、按接收账号/部门隔离的站内消息、顶部消息入口、通知偏好、SMTP/双语模板/本人测试发送/投递记录、持久化幂等队列及退避重试。发送前复验接收身份、归属、正式审批能力与偏好；无邮箱不影响站内消息。
- system 一级模块及设置/字典/文件/通知二级功能，目录共 40 个权限；管理写与发布等要求正式权限，旧角色不因目录扩展自动扩权。个人偏好/消息使用本人入口；权限矩阵保留一级 rowSpan 并显示扩展操作。
- 追加 00005/00006 迁移，保留原有账号/会话。删除账号清理个人配置，不删除共享历史/附件；gov_identity_refs 保存历史 ID 并保留跨租户复合外键。共享配置按自身拥有范围授权，不能套用账号任意部门可见规则。

规则同步：六个 baseline 项目 Skill、apps/web/AGENTS.md、services/api/internal/AGENTS.md、services/api/db/AGENTS.md、安全、架构、测试、设计系统、工具矩阵与版本说明。六个 Skill 均通过官方 quick_validate.py；前端 Skill 额外记录 Base UI 表单提交按钮需显式 type="submit"。上游 Goa Skill 未改动。

实际本地验证：

- `make ci-fast`：通过；最终完整 CI 再次执行同一 profile。前端 11 个 Vitest 文件 / 22 个测试通过，Go 测试、规则、格式、lint/vet、类型、双语、生成一致性及两端构建通过。
- `make ci`：退出码 0；真实 PostgreSQL 集成通过，Playwright 11/11 通过（22.3 分钟），使用隔离 schema、真实 Go 服务、生产构建及本机 Chrome。覆盖原有登录/主题/CRUD/临时申请审批/多部门/内置管理员保护，以及新增设置发布、刷新保持、个人分页/主题、字典编辑/导入预校验/发布、真实私有附件下载、消息页与中文主题。
- 最后补充动态来源越权、动态导入导出拒绝、发布后的上传限制及 null 类型拒绝断言，再执行 `make test-security`：退出码 0；完整真实 PostgreSQL 集成通过（9.258 秒）。隔离 SMTP 服务器验证失败后持久化重试、成功投递及无邮箱时站内通知保留。
- 数据库还覆盖跨公司/部门切换、拥有范围拒绝、稳定代码、树环、停用父项的历史解析、一次性证明、密文脱敏、并发版本冲突、存储切换后旧文件读取、单连接事务、密码/锁定/绝对和空闲会话策略、账号删除与历史跨租户外键。附件路径/符号链接、伪类型、取消写入清理和 CSV 公式转义有单元回归。
- 浏览器验收发现并修复新表单漏写提交类型；增加真实点击到 SDK 的组件回归。登录现在保留受保护深链，更新旧用例验证回跳后再进入首页。早期失败/中断运行未计为通过，以上是最终成功结果。
- 已目检浅色个人偏好及暗色中文消息中心截图；现有无外间距、内容面板边框/圆角、内部滚动、顶栏和页脚布局保持一致。截图及 trace 位于 apps/web/test-results，HTML 报告位于 apps/web/playwright-report。
- 开发库迁移完成，保留既有业务数据；加密根密钥仅保存在忽略的本地 .env（0600），未输出凭证。通过原有 5180 Vite 代理只读验证设置、字典、附件、投递、本人消息、有效配置和公开品牌接口，全部 200；未向开发库写入验收业务记录。

边界与未执行项：没有触发远程 CI 或创建 PR。真实管理员 SMTP 主机白名单及账号尚未配置，外部真实邮箱投递未执行；已完成的邮件测试使用隔离本地服务器。SMTP 按至少一次投递设计。Vite 仍有主包大于 500 kB 的构建提示，构建成功，本次未增加代码拆分优化。第三方存储及自动迁移按批准方案未实现。

## 2026-10-29 公共配置与公共能力 Skill

按用户最新要求，以 [ADR 007](decisions/007-global-governance.md) 替代前节的组织设置继承和字典覆盖。设置与字典统一工作空间管理，页面移除配置范围；运行时只解析系统设置和允许的本人偏好。管理仍需正式权限与 ALL 范围，旧部门授权不自动扩权；文件、消息和动态来源保留原数据授权。

追加 00007 迁移，将旧字典及其项统一为公共定义并保留 ID/代码/引用；旧组织设置和覆盖仅保留历史，不再参与运行时或重新发布。秘密不跨加密范围自动搬移。字典历史文档读取/发布/回退使用公共目标，数据合并的 Down 不重新拆分。

新增 [baseline-shared-capabilities](../.agents/skills/baseline-shared-capabilities/SKILL.md) 和 [接入约定](shared-capabilities.md)，明确设置注册与消费者、字典选项与事务引用、文件 SDK/用途/私有读取/存储版本，以及通知事件接入。根 AGENTS、任务工具矩阵、agent-policy 及相关前后端/数据库/安全/质量 Skill 同步路由；业务消费时即可触发，不限于开发管理页面。新 Skill 和五个修改的项目 Skill 均通过官方 quick_validate.py；规则发现校验登记 8 个 Skill、9 类任务。

本地验证结果：

- `make check-rules` 通过，包含规则路由、链接与前端约束检查。
- `make test-security` 退出码 0，真实隔离 PostgreSQL 集成通过；新增公共配置跨公司/部门一致、旧组织目标/覆盖拒绝、旧管理范围不扩权与迁移稳定引用测试。
- 开发库已应用 00007 并重启当前项目 API；5180 代理只读确认设置、字典、文件、有效配置均为 200，设置注册层级仅 workspace/account。未向开发库写入验收业务记录。
- 最终 `make ci-fast` 退出码 0：规则、格式、lint/vet、类型、11 个 Vitest 文件/22 个测试、Go 测试、双语资源、生成一致性和两端构建通过。首次 ci-fast 因新增测试文件格式未更新而失败，格式修正后重跑；不将首次失败算为通过。
- `make ci` 的 fast profile 和 PostgreSQL 集成通过，但浏览器全量未通过：1 项通过，2 个既有用例分别耗尽 180/300 秒总预算，随后停止运行，1 项中断、7 项未运行。Trace 显示登录/退出流程在最后跳转前已消耗约 175 秒；CRUD 在账号保存阶段耗尽总预算。没有删除断言或保留扩大旧用例预算的修改。诊断在 apps/web/test-results；不能据此宣称完整 CI 通过。
- 本次公共设置、字典、文件流程单独执行 `playwright test e2e/governance.spec.ts`，退出码 0，2/2 通过（6.9 分钟）：覆盖移除配置范围选择器、设置草稿/发布/品牌/刷新保持、个人偏好与主题、字典编辑/导入预校验/发布以及真实私有附件下载。输出隔离于 /private/tmp/xxh-global-governance-results，避免覆盖上述全量诊断；已目检浅色个人偏好截图。

未触发远程 CI，未创建 PR。

## 2026-10-29 WebMCP 与 WebSkills

按 [ADR 009](decisions/009-webmcp.md) 接入 OpenTiny：`/mcp` 统一接入页（远程 MCP / WebMCP / WebSkills 三标签，旧 `/webmcp`、`/mcp-settings` 重定向）、默认 loopback 18793 服务、显式配对、Codex/WorkBuddy MCP 与三套 WebSkills 一键安装，未来助手预留共享注册表 list/invoke。继续使用 React/Router/shadcn、TanStack Query、React Hook Form/Zod 和 i18next；未新增业务 API、数据库迁移或业务写工具。

实现源：features/webmcp、lib/webmcp、scripts/webmcp-{server,install,stdio,result}.mjs、webskills。导航与账号页面挂载白名单工具；注册通过 AbortSignal 随路由撤销，401/退出/切部门/刷新清理连接。业务权限与 SQL 范围不变，账号详情只用已有授权快照；诊断只投影数量/稳定错误码。SDK 动态加载 core，固定依赖/许可证与实际接口兼容记录见 versions.md / versions.json。

规则同步：apps/web/AGENTS.md、scripts/AGENTS.md、baseline-security Skill、docs/security.md、architecture.md、agent-tooling.md、testing.md、agent-policy.json；双语 webmcp 命名空间已注册。使用 feature-development、web-development、internationalization、security、quality-gates、shared-capabilities（判断本机部署配置不进入业务设置）、Skill Creator 与 OpenAI Docs 工作流。

已完成验证：

- `make ci` 的 fast profile 通过：规则/格式/ESLint/Go vet/TypeScript、14 个 Vitest 文件 / 28 项测试、5 项本地服务/安装器测试、Go 单元/HTTP、双语资源、生成一致性与两端构建。Goa/sqlc/Orval 生成可复现，无源契约变更。
- 同次 `make ci` 的真实 PostgreSQL 集成测试通过，退出码 0，隔离 schema 未改开发工作区。
- `PLAYWRIGHT_CHANNEL=chrome TEST_DATABASE_URL=<隔离本地库> pnpm --filter @baseline/web exec playwright test e2e/webmcp.spec.ts` 退出码 0，1/1 通过。真实浏览器 → SDK → 官方代理 → stdio → MCP Client 覆盖连接、校验、导航、账号筛选、范围外 ID 拒绝、路由工具卸载、脱敏、断开与旧注册清理；浅/暗页面截图已目检。没有让 E2E 修改用户全局客户端配置。
- `pnpm webmcp:test` 通过：Bearer/Origin/Host/CORS、会话与客户端白名单、配置合并/备份/重复安装/坏配置拒绝、技能安装和 WebMCP 结果信封解包。真实 Codex CLI 在独立临时 home 中完成配置合并验证；WorkBuddy JSON 与技能安装在临时 home 验证。
- 三套 WebSkills 和修改的 baseline-security 均经官方 quick_validate.py 校验通过（uv 临时 PyYAML 环境）；安装器不会把技能备份放在自动发现目录。
- 浏览器初次因缺少配套 Chromium 未启动，改用本机 Chrome 后完成专项验收；真实 SDK 接口差异与 CORS 预检问题均根据 Trace 修复并重跑。`.local` 本地预览/Trace 产物与既有 Git 忽略对齐，排除出 Prettier/ESLint，不关闭源代码检查。

全量 `make ci` 最终退出码 2：浏览器 11/13 通过；部门隔离和内置管理员两个既有长流程分别耗尽 240/180 秒总预算。完整 Trace 显示，部门切换/刷新保持及保存/主部门保护断言均已完成，失败发生在最后一次切换/跳转阶段。仅把这两个用例上限调整为有界 300/240 秒，保留所有断言。随后执行 `playwright test e2e/administration.spec.ts --grep 'account department policies persist|built-in administrator can edit additional' --output=/tmp/baseline-webmcp-recheck-results`，退出码 0，2/2 通过（8.1 分钟）。13 个浏览器用例均获得通过记录，但不把首次 `make ci` 宣称为一次全绿。

收尾检查补充：新命名空间的中英文切换和原有字段错误动态翻译通过组件交互验证；自定义 WEBMCP_STATE_DIR 已传递给安装的客户端 env，配置中不包含密钥。最终 lint 发现治理排程表单渲染时调用 Date.now 的纯度告警，已改为弹窗 useState 惰性初始化，保留原默认时间语义；ESLint 与相关 settings/WebMCP 组件测试通过。工具目录变化标准通知已补齐，并经真实 stdio E2E 验证，专项 1/1 再次通过（2.2 分钟）；断开时移除订阅并拒绝旧连接代次通知。最终 `make ci-fast` 退出码 0：15 个 Vitest 文件 / 32 项测试、5 项 WebMCP 服务/安装器测试、Go 测试、规则/格式/ESLint/vet/类型、双语资源、Goa/sqlc/Orval 生成一致性和两端构建全部通过。

未触发远程 CI，未创建 PR；未验证 WorkBuddy 应用真实加载配置，也未启用 AI 模型或聊天 UI。Vite 仍提示已有主包超过 500 kB；SDK core 单独按需加载。当前代理用于单机显式连接，不作为生产多用户远程代理。

## 2026-10-29 独立通知中心、人工 Markdown 通知与附件

按用户最新要求及 [ADR 008](decisions/008-notification-center.md)，通知菜单独立为收到、发起、已发送和投递记录。投递记录不再嵌在系统设置；SMTP/模板参数仍在系统设置，连接/本人发送测试在通知中心。延续现有布局、语义 tokens、浅暗主题和 en/zh-CN。

新增正式 `notifications:send`，目录从 40 扩至 41 项，旧角色不自动扩权。支持账号/部门（含子部门）/公司/企业混选、搜索与已选保留；后端按正式范围确认完整目标、展开有效成员并按账号去重，最多 200 个目标、10,000 个账号。预览和发送都校验范围；发送固定接收快照，以客户端 UUID/内容摘要幂等，相同键不同内容拒绝。主体、消息、附件绑定、站内投递及可选 SMTP 队列同事务，审计不记录完整正文/邮箱。

正文使用固定 react-markdown/remark-gfm 的 GFM 预览，原始 HTML/MDX、危险 URL 和外部图片不执行/自动加载。上传复用已有文件服务，插入受控 file:ID 引用并与正文表格分隔，可移除附件和对应引用。服务端验证文件属于发送者、同租户、attachment 用途及引用白名单；绑定和删除共享事务锁。收件人按本人、有效归属及通知文件绑定鉴权，无需全局 files:read，不能访问他人消息/未绑定文件。邮件仍为纯文本 MIME，保留 Markdown，文件引用和入口指向鉴权通知详情；关闭站内偏好仍可读取本人邮件详情。

契约源在 services/api/design/governance.go，SQL 源为追加 00008 与 announcements/governance 查询；使用 Goa/sqlc/Orval 生成产物，不手改 SDK/dbgen。收件目标数组用 ArrayOfRequired，在 HTTP 边界拒绝 null 项。新增 announcements 服务实现，保留既有临时权限事件和持久化 SMTP 重试。权限相关 Skill（公共能力、安全、前端、质量）和 web/internal/db AGENTS、安全/架构/方案/工具约定同步维护；版本/许可证及官方来源在 versions.md。

实际本地验证：

- 最终 `make ci-fast` 退出码 0。当前共享工作区为 14 个 Vitest 文件/29 个测试，规则、格式、ESLint/Go vet、类型、Go 测试、国际化、生成一致性与两端构建通过。工作区含并行 WebMCP 功能，新增 Node 测试需 loopback 监听，最终使用已批准的本地执行权限；未关闭检查。
- `make test-security` 退出码 0，真实隔离 PostgreSQL 全集通过（14.728 秒）。新增测试覆盖混选去重、受限组织拒绝、CSRF、幂等/冲突、空目标项、跨租户与他人消息、无 files:read 的收件人下载、非本人文件绑定拒绝、偏好、归属失效、发送/删除文件并发及事务回滚。隔离 SMTP 服务验证人工 Markdown 内容、MIME 编码和保护详情链接，未投递到真实外部邮箱。
- 最终 `make ci E2E_ARGS="--config=playwright.governance.config.ts e2e/notifications.spec.ts"` 退出码 0：再次执行 fast profile、真实数据库和通知浏览器用例（1/1，3.8 分钟）。确认账号和公司两类选择同时保留，上传/预览/发送/收取/详情/下载/已读/投递完整链路，以及系统设置无投递记录区块。
- 另外两个治理浏览器用例在独立配置首轮通过（设置 3.4 分钟、字典/文件 2.5 分钟）。首轮通知用例因测试把 Mark as read 写成 Mark read 而超时；修正定位后重跑通过。补充组件测试复现并修复切换类型丢失选择、上传引用落入表格的问题，没有删除断言。当前新增组件测试共 3 项，包含 GFM、安全渲染、跨类型选择、缩进保留、文件插入/移除。
- 初期完整门禁遇到并行工作未格式化、测试服务端口占用及生产服务启动失败，均未计为通过。新增独立治理配置使用 5183/8101、独立构建/报告目录，避免停止或覆盖其他任务的服务；默认全量浏览器配置未被删改。首次失败痕迹保存在 /private/tmp/xxh-notification-first-browser-results，最终截图及 HTML 在 apps/web/.local/governance-results / governance-report。已目检暗色编写与 Markdown/附件展示，浅色样式沿用语义 tokens，另有治理个人偏好浅色截图。
- 00008 开发库迁移完成并重启 API；5180 代理的收件选项、已发送、投递和本人消息接口只读验证均 200，空收件人预览返回 400 且不发送消息。未向开发库写入测试通知；所有发送/收取验收在隔离测试 schema 完成。官方 quick_validate.py 校验四个修改的 Skill。

本次浏览器结果是上述三个相关用例分别成功，未重跑默认全量 E2E 集合，也未触发远程 CI 或创建 PR。真实 SMTP 仍由管理员配置。Vite 主包体积提示仍存在；本次未开展无关代码拆分优化。并行 WebMCP 工作保留，其功能验收不属于本节通知验证结论。

## 2026-10-29 系统设置定时发布

按 [ADR 010](decisions/010-settings-schedules.md)，已保存的系统设置草稿可选择设备本地时间排期，转换 UTC 后由持久化后台执行器到期发布；支持记录、取消、失败原因及刷新保持。默认一小时后，服务端允许超过一分钟至 365 天。复用正式 settings:publish/ALL，敏感排期使用用途、会话、部门、草稿和基准版本绑定的单次重新认证。到期重建当前身份与正式权限，校验草稿摘要、现行消费者和版本；失败不覆盖配置。配置、版本、审计和成功结果同事务，重启后继续处理 pending。

契约修改位于 Goa DSL，SQL 追加 00009 和 settings_schedules 查询，Goa/sqlc/Orval 生成可复现。公共能力、安全、前端、数据库及质量 Skill 和对应 AGENTS/架构/安全/测试/工具规则同步；五个 Skill 均通过官方 quick_validate.py。未新增依赖或权限代码，保留并行 WebMCP 工作。

实际本地结果：

- `make ci E2E_ARGS="--config=playwright.governance.config.ts e2e/settings-schedules.spec.ts"` 的 fast profile 和真实 PostgreSQL 集成均通过：15 个 Vitest 文件/32 项测试、规则/格式/ESLint/Go vet/TypeScript、国际化、单元/HTTP、本地服务测试、生成一致性和两端构建。整条命令最终退出码 2，因为浏览器阶段缺少 Playwright chromium_headless_shell，启动失败不能计为通过；随后使用已安装 Chrome 单独重跑浏览器阶段。
- `make test-security` 退出码 0，隔离 PostgreSQL 集合通过（14.058 秒）。定时发布专项另通过 7 项真实数据库子用例：未到期/取消/非法时间、双 worker 恰好一次、版本冲突、敏感单次用途绑定与原会话到期、正式权限/范围收紧及停用/跨租户、单连接 Logo 校验、实际 worker 循环和新服务实例恢复。所有发布写入位于隔离 schema。
- 开发库追加迁移 00009 完成，重启已核实的本仓库 API；经 5180 前端代理登录后，排期列表、设置和运行时配置接口均返回 200。未向开发库写测试排期或发布测试品牌。
- 初期检查发现时间输入重复属性，修正后类型检查通过；敏感排期集成曾暴露正式授权返回 Principal 不携带会话哈希的问题，已改为授权检查后保留原认证上下文消费证明，相关正反例重跑通过。无障碍输入名称和时区描述分开关联，保留原断言。

- `PLAYWRIGHT_CHANNEL=chrome make test-e2e E2E_ARGS="--config=playwright.governance.config.ts e2e/settings-schedules.spec.ts"` 退出码 0，1/1 通过（用例 2.8 分钟，整体 3.1 分钟）。真实浏览器确认保存草稿、选择排期、时区提示、未提前生效、禁止同草稿立即发布、刷新保持及取消后恢复立即发布，没有页面运行错误。浏览器未等待一小时后的到期；实际到期与重启恢复由上述真实数据库/worker 用例覆盖。

最终 `make check-rules` 通过。未重跑默认全量 E2E，也未触发远程 CI 或创建 PR；Vite 既有主包体积提示仍存在。到期执行依赖 API 服务运行，停机后恢复时重新校验并处理，不承诺硬实时。


## 2026-10-30 信息架构调整：Agent 协作入口与菜单合并

侧栏一级入口收敛为：工作台、账号管理、组织管理、权限管理、通知中心、Agent 协作、系统设置（个人偏好仍从顶栏进入）。原「远程 MCP」入口改名为「Agent 协作」，页面标题同步，内部保留 MCP Server / WebMCP / WebSkills 三个标签。

改动位置：`features/administration/layout.tsx`（菜单数组改为 any-of 权限过滤、顶栏临时申请改纯图标、删除“系统管理中心”文字）、`app/router/router.tsx`（`/departments`、`/roles`、`/dictionaries` 改为重定向；`/attachments` 保留路由但不再占菜单位置）、`features/administration/pages.tsx`（组织页承载组织架构/部门标签，权限页承载角色授权/账号独立授权/角色管理标签，各区块保留原表单、删除、分页与页内工具）、`features/governance/settings.tsx` 与 `dictionaries.tsx`（系统设置新增系统配置/字段管理标签，字典列表抽为 DictionarySection）、`styles/admin.css`（`.admin-page-tabs` 复用页签样式）、`i18n/locales/{en,zh-CN}/{admin,governance,mcp}.json`、`lib/webmcp/routes.ts`。

权限与数据范围未改：标签可见性按正式读取权限（organizations:read 或 departments:read、access:manage 或 roles:read、settings:read 或 dictionaries:read）决定，默认标签随可用权限回退；服务端授权的允许/拒绝用例由既有集合覆盖。页内搜索更新保留其它 URL 参数（`withQuery`），避免切换标签后丢失 `?tab=`/`?type=`。旧路径重定向保留，WebMCP 路由清单同步。

实际本地结果：

- `npx tsc --noEmit`：退出码 0。
- `vitest run`（apps/web）：16 个文件 / 39 项通过，含 MCP 页五个标签行为用例与设置页原生提交用例。
- `node scripts/check-i18n.mjs`：命名空间与键集合一致。
- `vitest run` 新增 `features/administration/navigation.test.tsx`：4 项覆盖部门标签、角色管理标签及无权限时隐藏，秒级完成。
- 浏览器验收只做过一次临时用例（隔离 schema、Go 8099、preview 5179，1/1 通过，3.6 分钟）：确认菜单仅剩七个入口、顶栏申请为无文本图标、三个合并页标签可新建、旧路径重定向生效、`/attachments` 仍可上传、浅暗截图一致。该用例运行成本过高，验收后已删除，回归改由上面的 Vitest 用例承担；需要真实链路时按需临时补回。
- `e2e/administration.spec.ts` 与 `e2e/governance.spec.ts` 已按新入口改写导航步骤，本轮未重跑这两个文件。

未执行：`make ci-fast`、完整 E2E 集合、远程 CI 与生产部署。仓库根 `eslint .` 仍因并行开发中的未跟踪临时文件 `apps/web/shot.mjs`、`apps/web/src/preview-editor.tsx` 报错，不属于本批次改动文件。

## 2026-10-30 通知中心列表只读摘要与独立详情页

按用户要求调整通知中心读取体验：收到的通知列表默认只显示标题与通知时间（保留未读底色与「标为已读」操作），点击标题行跳转独立详情页 `/notifications/:id` 查看完整正文、附件与来源事件；已发送列表默认收起正文，只显示标题、收件人数与时间，需要时展开「查看内容」。邮件里的历史详情链接仍是 `?message=<id>`，通知中心会把该参数重定向到新详情页，后端邮件模板与既有测试未改动。

改动位置：`features/governance/notifications.tsx`（InboxPanel 改为摘要行 + 链接、SentPanel 改为 `details` 收起、新增导出 `NotificationDetailPage`、`?message=` 重定向）、`app/router/router.tsx`（新增 `/notifications/:id`，`/notifications/compose` 仍为静态优先路由）、`styles/admin.css`（`.gov-message-summary`、`.gov-message-content`、`.gov-message-detail`、`.gov-message-meta` 复用现有 tokens）、`i18n/locales/{en,zh-CN}/governance.json`（新增 `notificationDetail`、`notificationDetailHint`、`viewContent`）、`features/governance/notifications.test.tsx`、`e2e/notifications.spec.ts`。数据读取未变化：详情继续用生成 SDK 的 `notificationsMessage`，附件走 `notificationsAttachment`，权限与数据范围未改，无新增公开端点。

`e2e/notifications.spec.ts` 原用例依赖已被 ADR 008 移除的「发起通知标签页」和发送页附件上传，长期无法执行；本次改为通过真实接口上传附件并发送通知（`/api/admin/files`、`/api/admin/notifications/send`），浏览器只验证通知中心读取链路，避免驱动第三方 Lexical 编辑器。

实际本地结果：

- `tsc --noEmit`（apps/web）：退出码 0。
- `vitest run src/features/governance/notifications.test.tsx`：7/7 通过，新增「列表只显示标题与时间并跳转详情」和「已发送正文默认收起」两项。
- `node scripts/check-i18n.mjs`：命名空间与键集合一致；改动文件 ESLint 与 Prettier 检查通过。
- `PLAYWRIGHT_CHANNEL=chrome make test-e2e E2E_ARGS="e2e/notifications.spec.ts"`：退出码 0，1/1 通过（用例 2.1 分钟，整体 2.5 分钟）。真实浏览器确认列表不含正文、时间可见、点击进入 `/notifications/<id>`、详情渲染标题/表格/附件下载、标为已读后按钮禁用、已发送正文初始隐藏且展开后可见、投递记录包含该通知，无页面运行错误。
- 环境注记：本机设置 `HTTP_PROXY/HTTPS_PROXY=127.0.0.1:57403` 的沙箱代理会让 Playwright 的 webServer 健康检查把 8099/5179 误判为已占用，浏览器验收需 `env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy` 后运行；`PLAYWRIGHT_CHANNEL=chrome` 使用本机 Chrome，因为未安装 Playwright chromium_headless_shell。
- `make check-rules`：退出码 0。

未执行：`make ci-fast`、完整 E2E 集合、远程 CI。发送页编辑器（第三方 Lexical/MDXEditor）的浏览器交互仍无自动化覆盖，写入校验/幂等由 Go 集成测试与组件测试承担；本轮未改动后端与生成物。
