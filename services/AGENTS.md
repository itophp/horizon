# Go API 规则

继承根规则。业务动作一律通过 `kernel/capability/gateway` 执行；新增能力先注册契约，再写 Executor，最后按需暴露 Goa。Goa DSL/传输契约变更使用官方 goa-service-designer；权限变更使用 baseline-security；SQL/迁移变更使用 baseline-database-change。

- design 为 API 契约源，gen 禁止手改；cmd 组装和生命周期，runtime/app 为应用逻辑，runtime/transport 为传输与中间件，repository 使用 sqlc/dbgen。不要在生成目录实现领域行为。
- 校验、Payload/Result/Error/HTTP/security 声明回到 DSL；读取生成接口后再实现，不凭记忆猜签名。OpenAPI + Orval 是前端类型来源。
- 只运行项目锁定 Goa 生成器；scripts/go.sh 在正确模块和工具链执行。全目录格式检查包含 integration build tag 文件，不能只 go fmt ./... 漏掉带标签文件。
- 配置集中验证，非法安全配置启动失败。统一请求 ID、结构化日志、时间/体积限制和稳定错误映射；日志与错误不得泄露凭证/内部数据。
- 用小接口/Fake 隔离依赖，context/cancellation 传播到 SQL；明确事务提交回滚，服务关闭有界。
- 先执行相关 scripts/go.sh test；后续 make ci-fast，触及 DB 再 test-integration。跨层行为按 E2E 暂停规则处理：默认不跑浏览器，需要旁证时只跑映射命中的单个 spec（见根 AGENTS.md）。生成变更必须 check-generated；安全变更必须允许+拒绝测试。

## Capability-Centric 分层

AI 任务 SSE 契约在 design/ai.go，传输复用既有 harness.Run 的 Principal/assistant:use/Gateway 边界；进度只允许公开阶段、能力名和状态，禁止原始工具数据与隐含推理。流式连接关闭与后台业务撤销不是同一动作。运行详情和会话历史关联业务审计时必须使用可信租户及已授权账号记录；确认后的等待状态以最新业务调用投影为准。

- `kernel/capability/contracts`：稳定词汇（Definition/Schema/Result/Event/Origin），不得引入存储或传输依赖。
- `kernel/capability/registry`：进程内定义中心；启动时校验版本、Handler、工具名冲突，失败即视为部署错误。
- `kernel/capability/gateway`：唯一执行入口；顺序固定为身份→权限→Schema→幂等→策略/审批→执行→Outbox→审计。禁止在 Handler 中绕开它直接调用 Executor。
- `kernel/{policy,approval,audit,outbox,eventhub,metadata}`：确定性策略、审批队列、审计读模型、事务 Outbox 与事件日志。`metadata` 是实体模型：从 embed 迁移解析表/列/外键、从能力目录派生 Capabilities，并执行 E1–E10 校验（`make check-entities`）。
- `ai/{harness,tools,triggers,context,knowledge,workflow,evaluation}`：AI 任务、工具适配、事件触发与评估。Agent 工具必须经 Gateway；AI 任务与业务调用状态分离。
- `system/*、applications/* 与 integrations/*`：业务模块（settings、dictionaries、files、notifications、mcp、example）。每个模块自持领域服务与 `Entries()`，并在 `entity_<name>.go` 声明自己的业务对象（`entities.go` 只汇总），不再声明自己的 Goa 服务或 HTTP 路由；写能力优先实现 `TxExecutor` 并声明 `SuccessEvents`。
- `design/` 只剩下会话/系统/公开投影与能力、AI、助手契约。新增业务动作不要加 Goa 方法，而是加一条能力定义并在 `runtime/transport/governance.go` 装配。
- 新增 `/api/capabilities` 或 `/api/ai` 端点必须并入 `guardedPath`，补 `tests/ai_platform_test.go` 的允许/拒绝、跨租户、幂等与审批用例。


# 服务、安全与数据访问实现

继承服务规则，安全相关开发加载 baseline-security；规则详见 docs/security.md。

- Memory Center 按 `docs/security.md` 的当前实现边界约束；新增共享作用域或业务来源必须先经 Gateway 验证原资源授权并在 SQL 强制可信范围，敏感内容不得进入能力审计、事件或模型历史。`manual-only` 能力不向 Agent 工具目录暴露，网关和执行端都拒绝自动化来源；`internal-only` 且声明 `parent:` 的能力只接受网关注入的指定父能力，不得信任客户端传入的父身份。

- auth 验证凭证、建立可信 Principal；authorization/Casbin 管资源操作；dataaccess 解析可信范围；repository/sqlc 在 SQL 层强制过滤。四个职责不能互相省略。
- 受保护请求默认拒绝；客户端 tenant/organization/department/owner 参数只能作为已授权范围内的业务输入，不能决定当前身份/权限。新增公开接口显式说明理由。
- 每个新资源说明 owner 的含义和 ALL/ORGANIZATION/DEPARTMENT/DEPARTMENT_TREE/OWN/CUSTOM 的解析；所有访问入口保持同等约束。详情/写入不能先无范围读取再判断。
- 策略只通过约定模型/策略载体更新并测试；缓存须有撤销失效策略，跨进程行为如实说明。禁止各 Handler 复制隐式权限。
- 响应与输入都用明确字段投影/白名单；新敏感字段检查 DTO、日志、错误、导出与批量入口。SQL 选择最小必要列。
- 数据层错误映射为稳定服务错误，不回传堆栈/SQL/凭证。context 传到数据库，事务不跨池误用，失败不吞异常或默认全量访问。
- 新增安全能力须有允许/拒绝、跨租户与范围外 ID 测试；适用时覆盖更新/删除、关联/聚合/批量/导出和撤销。运行 make test-security。

正式后台位于 system/identity（含 `Entries()` 身份管理能力）：Service 管用例和不变量、policy.go 管 Casbin/可信范围、Repository 调用生成的 dbgen、transport/identity.go 校验会话与 Origin/CSRF。旧 auth/authorization/dataaccess 为独立技术 demo，不要把 demo token 接到后台或复制第二套登录。身份变更参考 docs/decisions/002-identity-access.md，保留账号覆盖替换范围、内置管理员保护、授权管理独立门禁及即时撤销。

临时权限与两级目录遵循 [ADR 003](../docs/decisions/003-temporary-access.md)：目录由 identity/catalog.go 定义并通过 Goa 输出；请求默认只能读取本人记录，独立正式 temporary:approve 才能审批他人。审批不能超出本人正式权限/数据范围，不能自批或再委托临时范围；原始申请和最终批准分开存储。范围在批准时固定解析，每次认证用数据库时间排除过期/撤销记录，deny 优先。修改这些行为必须覆盖 tests/temporary_access_test.go 的负向与并发用例。


多部门授权遵循 [ADR 004](../docs/decisions/004-department-memberships.md)：一个 Principal 只解析当前已验证部门归属的角色/覆盖/范围，禁止跨部门合并。持久授权修改及多部门账号全局敏感变更必须覆盖所有原有和目标部门；临时申请与生效绑定申请部门。任何认证、权限控制或数据范围行为变更，必须同步维护权限控制相关的 Skill、目录 AGENTS.md 和安全/架构规则，并补齐受影响的允许与拒绝测试；交付说明列出同步位置，不允许只改代码。

系统治理遵循 [ADR 005](../docs/decisions/005-system-governance.md)：共享 Core 复用 identity 正式授权，配置拥有范围不套用账号任意归属可见；设置/字典发布必须版本锁和同事务快照/审计，敏感发布及回退消费单次绑定重新认证。秘密不进 DTO/历史/导出；字典动态来源只用受控提供器。文件绑定不可变存储配置、仅受控品牌 Logo 公开；通知投递前重查接收上下文及正式审批能力。相关权限变更同步 Skill/规则并覆盖 governance/SMTP/范围负向测试。

公共设置和字典按 [ADR 007](../docs/decisions/007-global-governance.md) 统一管理，不新增组织范围或字典覆盖。业务接入先读 [公共能力 Skill](../.agents/skills/baseline-shared-capabilities/SKILL.md)，复用注册键、选项/引用、鉴权文件服务和通知事件；文件、动态来源及消息仍保留原资源数据范围。

人工通知按 [ADR 008](../docs/decisions/008-notification-center.md)：notifications:send 正式权限，组织成员全部范围校验和账号去重；announcement/快照/附件/投递同事务且幂等。附件通过 recipient+有效归属+通知文件绑定 SQL 读取，不能套用全局 files:read 放大收件人权限；旧临时权限事件不伪造或替换。

设置排期按 [ADR 010](../docs/decisions/010-settings-schedules.md)：排期时正式 publish/ALL，敏感证明绑定 settings.schedule；到期重建原部门 Principal 并重查正式权限、草稿摘要、版本及依赖。共用发布事务并与完成状态原子提交，保存失败码。不能用旧会话/临时权限或后台特权绕过授权；读取验证共用事务，避免跨池/单连接死锁。


账号级远程 MCP 见 [需求与实现方案](../docs/mcp-service-plan.md)（首阶段开发实现，验收状态见交付记录）。功能变动核对能力的暴露及授权范围；账号 Token 与本机 WebMCP 配对分离。实施复用可信 Principal、Casbin、SQL 范围及字段投影，Token 固化工具集合与当前正式权限取交集，撤销/禁用实时生效，秘密仅一次显示且不进入日志/配置包。实现入口为 integrations/mcp；协议端点 `/mcp` 保留，令牌管理已改为 `mcp.token.*` 能力。

账号远程 MCP 按 [ADR 011](../docs/decisions/011-remote-mcp.md)：工具复用原授权用例，凭证绑定本人/部门和固化集合；敏感管理操作重新认证，身份变更撤销与延迟归属检查、客户端配对/备份及协议专项测试同步维护。禁止将安装状态当连接成功。

内嵌助手按 [ADR 012](../docs/decisions/012-embedded-assistant.md)：页面工具在浏览器执行，模型/端点/Token 只在服务端解析。浮动面板统一使用 Eino 工作区 Agent；浏览器工具使用原生中断和加密检查点交接结果，恢复必须绑定租户、账号、部门及单次凭证。业务动作一律经 Gateway，不信任浏览器工具结果授予权限。

Capability-Centric 平台按 [ADR 015](../docs/decisions/015-capability-centric-platform.md)：

- 业务动作只注册一次，入口是各模块的 `Entries()`（`system/*、applications/* 与 integrations/*` 与 `system/identity`）；`gateway` 是唯一运行时入口，负责身份、权限、Schema、幂等、风险/审批、执行、Outbox 与审计。业务模块不得再暴露自己的 REST 端点。
- 可独立执行的动作必须注册为能力，不允许只以 service 方法或 handler 形式存在；能力之间只经 `gateway.Invoke` 互调，禁止 `registry.Resolve` 后直接调 handler，或绕过能力直连 `dbgen`/`pool`/领域服务。内层 `Origin` 由 Gateway 强制为 `OriginCapability`、深度 +1、写能力确定性幂等键；`TxExecutor` 内禁止再调 Gateway。分层判据与六条约束见 [能力拆分与组合](../docs/capability-composition.md)。
- 模块构造早于 `gateway.New`，编排能力用 `AttachGateway` 在 `gateway.New` 之后回填；`gateway` 为 nil 时返回 `unavailable`，不得静默降级为直连数据。
- 新模块写能力使用 `TxExecutor`，业务数据与 `cap_outbox` 同事务；已迁移模块（settings/dictionaries/files/notifications/mcp）仍由领域服务自持事务，事件在成功提交后写 Outbox，属于已声明的 best effort 边界。
- 审批执行必须用 `PrincipalForTask` 重建申请人身份；决策条件更新，重复/过期/跨租户冲突。
- `cap_invocations.input` 只落 `Loggable`；审批正文不进 DTO；日志与错误不回传凭证、SQL 或上游响应。
- 每个能力必须声明 `Entity`（主对象）；一个调用改多张表时用 `Entities` 列出全部，且主实体必须在内。判据是“这个调用改了什么”，不是“它产出了什么”。
- 业务对象在模块内以 `entity_<name>.go` 一实体一文件声明，`entities.go` 只做汇总并接入 `runtime/app/entities.go`；`make check-entities` 对照迁移校验字段、枚举、关系与状态转移，改动表结构或能力必须一并跑。
- 改状态列的写能力必须绑定 `Transition.Capability`；由 worker 驱动的转移留空并作为事实输出。状态只能经转移改变的才用 `state`，一次调用可任意改值的按业务枚举处理。
- 事件必须带 correlation/causation/depth；Trigger 去重并受 maxDepth 限制；AI 任务与业务调用状态独立。
- 新增 `/api/capabilities`、`/api/ai` 端点并入 `guardedPath` 与权限目录，并补 `tests/ai_platform_test.go` 的允许/拒绝、跨租户、并发幂等与审批用例。


## 持久化流程安全

编排使用 Eino Workflow/Graph/ADK，业务节点仍经 Gateway。嵌套来源强制 OriginCapability，继承父调用/深度，事务内互调拒绝。恢复及节点前重建原账号/部门身份；Agent 工具固定版本白名单与当前权限取交集，禁止递归编排。run/read/cancel 权限独立，查询/取消强制 tenant+account；MCP Token 不自动获得新工具。检查点加密绑定 tenant/run/key，不进入明文审计或运行摘要。取消撤销尚未执行的流程审批，已在执行/已提交动作不回滚；审批恢复读取原 invocation，独立动作不因载荷相同共用审批。详情见 docs/orchestration-plan.md（从仓库根解析）。

文章审核遵循 docs/security.md 的文章评审规则：正文不落调用/AI 审计；评审结论由 article.record-review 落库并归一化为 decision（只接受 OriginWorkflow），发布声明 high + confirm，必须人工确认（多人部署可切 four-eyes），article.reject 只接受 OriginWorkflow|OriginApproval，且都要求 review_code 与状态、版本同时匹配；人工审核禁止自动化与作者自审；修改正文或版本不得沿用旧审核。条件分叉用 Step 的 when 声明，禁止用"两个都跑、内部判空"的方式伪造分支。

## 内置插件权限目录

内置插件可经 kernel/plugin Registrar 声明权限定义；身份服务实例必须在启动工作线程、处理请求或重建 Principal 前校验并合并插件权限。与内置身份目录冲突即失败，重复配置仅允许完全相同的目录；注册不授予权限。会话、部门切换、临时权限及 PrincipalForTask 必须使用同一实例目录，未知权限默认拒绝，deny 优先；不得使用进程级可变目录或信任插件输入作为调用身份。新增权限需声明 scope/临时授权语义，并覆盖 Gateway 允许/拒绝、跨租户、撤权和身份重建。

Application Runtime 的只读定义来自业务模块贡献及 kernel/applicationruntime Adapter；公共 TS 类型和结构校验从 Go 契约生成，禁止手改 api/generated/runtime.ts。页面定义不能扩权或替代文章 SQL 的本人/审核人范围；所有页面动作仍经 Gateway。

Runtime v2 目录解析必须按贡献中的实际权限投影，不能固定某个示例的权限或实体。创建/编辑分别用能力 I/O，派生字段只读，普通更新不依赖状态转移存在。批量组合须经注入的 Gateway 调用器执行子能力，禁止把多个独立事务称为原子事务；operationId/子幂等键与外层调用重放按 ADR 018 区分。
## 远程 MCP 动态能力目录（现行）

远程 MCP 与 AI 助手共享自动化能力发现；Token 绑定账号和部门，每次请求按当前正式权限重建目录，历史固化工具集合不再限制能力。业务调用必须经 Gateway，并保持来源、数据范围、审批和幂等检查；手动和内部专用能力不进入自动化目录。权限变更须覆盖发现、猜测调用和撤权后的拒绝。详见 docs/security.md 和 docs/decisions/011-remote-mcp.md（从仓库根解析）。

Runtime 2.2 定义存储见 docs/application-runtime-database.md（从仓库根解析）。数据库版本不可覆盖更新；生命周期状态和配置内容分别使用运行修订与四级版本，部署重启不得覆盖已发布定制。system-only 能力仅允许系统账号及事件来源，不能借用要求固定父能力的 internal-only 标签。
