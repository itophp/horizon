# 服务、安全与数据访问实现

继承服务规则，安全相关开发加载 baseline-security；规则详见 docs/security.md。

- auth 验证凭证、建立可信 Principal；authorization/Casbin 管资源操作；dataaccess 解析可信范围；repository/sqlc 在 SQL 层强制过滤。四个职责不能互相省略。
- 受保护请求默认拒绝；客户端 tenant/organization/department/owner 参数只能作为已授权范围内的业务输入，不能决定当前身份/权限。新增公开接口显式说明理由。
- 每个新资源说明 owner 的含义和 ALL/ORGANIZATION/DEPARTMENT/DEPARTMENT_TREE/OWN/CUSTOM 的解析；所有访问入口保持同等约束。详情/写入不能先无范围读取再判断。
- 策略只通过约定模型/策略载体更新并测试；缓存须有撤销失效策略，跨进程行为如实说明。禁止各 Handler 复制隐式权限。
- 响应与输入都用明确字段投影/白名单；新敏感字段检查 DTO、日志、错误、导出与批量入口。SQL 选择最小必要列。
- 数据层错误映射为稳定服务错误，不回传堆栈/SQL/凭证。context 传到数据库，事务不跨池误用，失败不吞异常或默认全量访问。
- 新增安全能力须有允许/拒绝、跨租户与范围外 ID 测试；适用时覆盖更新/删除、关联/聚合/批量/导出和撤销。运行 make test-security。

正式后台位于 internal/identity（含 `Entries()` 身份管理能力）：Service 管用例和不变量、policy.go 管 Casbin/可信范围、Repository 调用生成的 dbgen、transport/identity.go 校验会话与 Origin/CSRF。旧 auth/authorization/dataaccess 为独立技术 demo，不要把 demo token 接到后台或复制第二套登录。身份变更参考 docs/decisions/002-identity-access.md，保留账号覆盖替换范围、内置管理员保护、授权管理独立门禁及即时撤销。

临时权限与两级目录遵循 [ADR 003](../../../docs/decisions/003-temporary-access.md)：目录由 identity/catalog.go 定义并通过 Goa 输出；请求默认只能读取本人记录，独立正式 temporary:approve 才能审批他人。审批不能超出本人正式权限/数据范围，不能自批或再委托临时范围；原始申请和最终批准分开存储。范围在批准时固定解析，每次认证用数据库时间排除过期/撤销记录，deny 优先。修改这些行为必须覆盖 tests/temporary_access_test.go 的负向与并发用例。


多部门授权遵循 [ADR 004](../../../docs/decisions/004-department-memberships.md)：一个 Principal 只解析当前已验证部门归属的角色/覆盖/范围，禁止跨部门合并。持久授权修改及多部门账号全局敏感变更必须覆盖所有原有和目标部门；临时申请与生效绑定申请部门。任何认证、权限控制或数据范围行为变更，必须同步维护权限控制相关的 Skill、目录 AGENTS.md 和安全/架构规则，并补齐受影响的允许与拒绝测试；交付说明列出同步位置，不允许只改代码。

系统治理遵循 [ADR 005](../../../docs/decisions/005-system-governance.md)：共享 Core 复用 identity 正式授权，配置拥有范围不套用账号任意归属可见；设置/字典发布必须版本锁和同事务快照/审计，敏感发布及回退消费单次绑定重新认证。秘密不进 DTO/历史/导出；字典动态来源只用受控提供器。文件绑定不可变存储配置、仅受控品牌 Logo 公开；通知投递前重查接收上下文及正式审批能力。相关权限变更同步 Skill/规则并覆盖 governance/SMTP/范围负向测试。

公共设置和字典按 [ADR 007](../../../docs/decisions/007-global-governance.md) 统一管理，不新增组织范围或字典覆盖。业务接入先读 [公共能力 Skill](../../../.agents/skills/baseline-shared-capabilities/SKILL.md)，复用注册键、选项/引用、鉴权文件服务和通知事件；文件、动态来源及消息仍保留原资源数据范围。

人工通知按 [ADR 008](../../../docs/decisions/008-notification-center.md)：notifications:send 正式权限，组织成员全部范围校验和账号去重；announcement/快照/附件/投递同事务且幂等。附件通过 recipient+有效归属+通知文件绑定 SQL 读取，不能套用全局 files:read 放大收件人权限；旧临时权限事件不伪造或替换。

设置排期按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：排期时正式 publish/ALL，敏感证明绑定 settings.schedule；到期重建原部门 Principal 并重查正式权限、草稿摘要、版本及依赖。共用发布事务并与完成状态原子提交，保存失败码。不能用旧会话/临时权限或后台特权绕过授权；读取验证共用事务，避免跨池/单连接死锁。


账号级远程 MCP 见 [需求与实现方案](../../../docs/mcp-service-plan.md)（首阶段开发实现，验收状态见交付记录）。开发使用 baseline-mcp-development，功能变动主动确认暴露范围；账号 Token 与本机 WebMCP 配对分离。实施复用可信 Principal、Casbin、SQL 范围及字段投影，Token 固化工具集合与当前正式权限取交集，撤销/禁用实时生效，秘密仅一次显示且不进入日志/配置包。实现入口为 internal/modules/mcp；协议端点 `/mcp` 保留，令牌管理已改为 `mcp.token.*` 能力。

账号远程 MCP 按 [ADR 011](../../../docs/decisions/011-remote-mcp.md)：工具复用原授权用例，凭证绑定本人/部门和固化集合；敏感管理操作重新认证，身份变更撤销与延迟归属检查、客户端配对/备份及协议专项测试同步维护。禁止将安装状态当连接成功。

内嵌助手按 [ADR 012](../../../docs/decisions/012-embedded-assistant.md)：页面工具在浏览器执行，模型/端点/Token 只在服务端解析。Agent 模式已改为能力驱动，业务动作一律经 Gateway。

Capability-Centric 平台按 [ADR 015](../../../docs/decisions/015-capability-centric-platform.md)：

- 业务动作只注册一次，入口是各模块的 `Entries()`（`internal/modules/*` 与 `internal/identity`）；`gateway` 是唯一运行时入口，负责身份、权限、Schema、幂等、风险/审批、执行、Outbox 与审计。业务模块不得再暴露自己的 REST 端点。
- 可独立执行的动作必须注册为能力，不允许只以 service 方法或 handler 形式存在；能力之间只经 `gateway.Invoke` 互调，禁止 `registry.Resolve` 后直接调 handler，或绕过能力直连 `dbgen`/`pool`/领域服务。内层 `Origin` 由 Gateway 强制为 `OriginCapability`、深度 +1、写能力确定性幂等键；`TxExecutor` 内禁止再调 Gateway。分层判据与六条约束见 [能力拆分与组合](../../../docs/capability-composition.md)。
- 模块构造早于 `gateway.New`，编排能力用 `AttachGateway` 在 `gateway.New` 之后回填；`gateway` 为 nil 时返回 `unavailable`，不得静默降级为直连数据。
- 新模块写能力使用 `TxExecutor`，业务数据与 `cap_outbox` 同事务；已迁移模块（settings/dictionaries/files/notifications/mcp）仍由领域服务自持事务，事件在成功提交后写 Outbox，属于已声明的 best effort 边界。
- 审批执行必须用 `PrincipalForTask` 重建申请人身份；决策条件更新，重复/过期/跨租户冲突。
- `cap_invocations.input` 只落 `Loggable`；审批正文不进 DTO；日志与错误不回传凭证、SQL 或上游响应。
- 每个能力必须声明 `Entity`（主对象）；一个调用改多张表时用 `Entities` 列出全部，且主实体必须在内。判据是“这个调用改了什么”，不是“它产出了什么”。
- 业务对象在模块内以 `entity_<name>.go` 一实体一文件声明，`entities.go` 只做汇总并接入 `internal/app/entities.go`；`make check-entities` 对照迁移校验字段、枚举、关系与状态转移，改动表结构或能力必须一并跑。
- 改状态列的写能力必须绑定 `Transition.Capability`；由 worker 驱动的转移留空并作为事实输出。状态只能经转移改变的才用 `state`，一次调用可任意改值的按业务枚举处理。
- 事件必须带 correlation/causation/depth；Trigger 去重并受 maxDepth 限制；AI 任务与业务调用状态独立。
- 新增 `/api/capabilities`、`/api/ai` 端点并入 `guardedPath` 与权限目录，并补 `tests/ai_platform_test.go` 的允许/拒绝、跨租户、并发幂等与审批用例。


## 持久化流程安全

编排使用 Eino Workflow/Graph/ADK，业务节点仍经 Gateway。嵌套来源强制 OriginCapability，继承父调用/深度，事务内互调拒绝。恢复及节点前重建原账号/部门身份；Agent 工具固定版本白名单与当前权限取交集，禁止递归编排。run/read/cancel 权限独立，查询/取消强制 tenant+account；MCP Token 不自动获得新工具。检查点加密绑定 tenant/run/key，不进入明文审计或运行摘要。取消撤销尚未执行的流程审批，已在执行/已提交动作不回滚；审批恢复读取原 invocation，独立动作不因载荷相同共用审批。详情见 docs/orchestration-plan.md（从仓库根解析）。

文章审核遵循 docs/security.md 的文章评审规则：正文不落调用/AI 审计，article.apply 仅接收内部可信结果，人工审核禁止自动化与作者自审；修改正文或版本不得沿用旧审核。
