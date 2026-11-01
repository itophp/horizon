---
name: baseline-security
description: "Implement or review authentication, authorization, tenant/data-scope controls, sensitive fields, public endpoints, file/export/batch operations, credentials, temporary-access approval/expiry or request-boundary security in this project."
---

# 认证、授权与基础安全

先读 [安全约定](../../../docs/security.md) 及 [后端实现规则](../../../services/api/internal/AGENTS.md)。涉及 SQL 时还要使用 baseline-database-change。

为变更说明：身份来源、resource/action、可信租户、适用 DataScope、输入边界、字段读写投影、日志信息及所有访问入口。未知权限与解析失败默认拒绝。

- Principal 必须来自已验证凭证/会话，不接受客户端 tenant/organization/owner 作为授权身份。正式 OIDC/SSO/会话按真实产品需求选择；本机开发 token 保持默认关闭且只能 loopback。
- Casbin 处理操作决策，ScopeResolver 解析可信范围，Repository/SQL 强制记录范围。Handler 不各自复制策略。权限撤销须符合当前同步/缓存失效设计并测试。
- 列表、详情、修改、删除、导出、批量、后台任务、聚合、关联和下载使用同等约束；不因为新入口遗漏租户过滤。敏感字段通过 DTO/SQL 投影和输入白名单保护。
- 新增公开端点必须明确公开理由和返回数据；公开健康检查只表示存活，不能泄露环境配置或内部依赖错误。
- 结合 Goa 约束、应用不变量、DB 约束验证输入；SQL 参数化，错误对外稳定且不泄密。日志不写令牌、密码、完整请求正文或敏感字段。
- CORS、cookie、CSRF、Token 存储由认证方式与跨域需求驱动；不要复制通配 CORS、把 token 放进 URL/持久化浏览器存储，或让演示凭证进入产物。新外部输入/上传须明确大小、类型与路径处理。
- 环境变量记录在 .env.example，真密钥不提交；修改配置时覆盖缺失/非法值和安全默认值。新依赖检查官方版本、兼容性、许可证与维护性，不引入同职能重复框架。

补允许和拒绝测试。make test-security 对当前真实认证/授权与 PostgreSQL 范围执行验证；扩展新的入口就补相应负向行为。人工检查的认证设计、字段分类和威胁边界要在报告说明，不能声称静态脚本已经证明安全。

## 两级目录与临时授权

变更申请/审批、有效期、范围提升或权限矩阵时，先读 [ADR 003](../../../docs/decisions/003-temporary-access.md)。服务端 identity/catalog.go 是 module/function/code/临时可申请性目录来源，经 Goa/SDK 传给页面；矩阵合并一级模块单元格，不另造权限代码。

- 申请不等于授权；普通账号仅操作本人申请。审批依赖独立正式 temporary:approve，禁止自批。最终操作/范围只能在审批人正式权限上限内，不能将临时获得的范围再次委托。
- 原始请求与最终批准分别保存，批准时固定解析相对范围，防止账号调部门后变相扩权。到期以数据库时钟为准，每次认证排除过期/撤销记录；固定 deny 始终优先，不覆写正式角色/账号策略。
- 权限配置、审批和角色策略写入不可临时授予；新增目录项明确 temporaryEligible 与 scopeConfigurable。保留原子状态转换和审批/撤销审计，跨租户及范围外 ID 默认拒绝。
- 扩展 tests/temporary_access_test.go 的允许/拒绝、修订、正式上限、到期、撤销、固定范围、deny、自批、并发和跨租户用例；前端同时覆盖合并目录与真实申请/审批流程。

临时写权限不能被用来留下长期提权：账号的角色/覆盖、凭证/标识、状态、公司/部门归属，以及组织树/状态等敏感变更，仍校验正式写权限及正式数据范围。特别测试临时 accounts:update 配合既有 access:manage 的永久角色自授予，以及临时 departments:update 重挂部门扩大正式部门树范围的拒绝路径。

## 多部门授权与规则联动

修改账号归属、部门角色/覆盖、会话上下文或临时授权时先读 [ADR 004](../../../docs/decisions/004-department-memberships.md)。每个 Principal 只使用当前已验证部门；不合并不同部门角色/覆盖。持久归属与策略变更校验正式 access:manage、正式写权限及所有原有/目标部门范围；OWN 和临时权限不能跨部门留下长期授权。切换验证本人有效归属并轮换 CSRF，清除旧上下文数据。临时请求与申请部门绑定，移除归属取消/撤销，重新加入不复活。

任何权限控制变更必须同步权限相关 Skill、目录 AGENTS.md 与安全/决策规则；涉及 UI/SQL 时同步对应项目 Skill。补真实允许/拒绝回归并在交付中列出同步位置，不能只改实现。覆盖部门隔离、显式 deny、非法切换、跨租户/公司不一致、正式授权边界、临时范围和撤权。


内置管理员的主部门保护不等于禁止本人配置附加部门。UI 的归属管理和当前部门策略编辑必须分开判断：主部门保留隐式全权保护，附加部门独立策略仍走正式授权/范围与本人校验；不能由此放开其他账号修改内置管理员或只读详情。详见 ADR 004。


## 系统治理能力

治理边界见 [ADR 005](../../../docs/decisions/005-system-governance.md)。管理写/发布/回退/导入导出/投递只用正式授权，拥有层级不能套用账号任意归属范围。敏感发布与回退消费绑定会话/部门/动作/资源/版本的五分钟单次证明；加密值不能出现在 DTO/历史/差异/导出。文件绑定不可变配置并约束根目录/类型/大小/范围，Logo 只由可信品牌绑定公开。动态字典复用来源授权；事件事务持久化，邮件投递前复验接收人及正式审批能力。修改权限或上述边界必须同步相关 Skill/AGENTS/安全决策，并补 governance/SMTP/范围负向测试。


公共设置/字典的最新范围按 [ADR 007](../../../docs/decisions/007-global-governance.md)，取代 ADR 005 的组织配置继承/字典覆盖：统一工作空间管理，个人白名单偏好独立。管理要求正式权限及共享资源 ALL 范围，旧部门授权不得自动扩权；文件、消息、动态字典来源仍执行原资源数据范围。业务需要设置项、字典选项、上传、读取、下载或通知时，先使用 [baseline-shared-capabilities](../baseline-shared-capabilities/SKILL.md)，复用现有服务/SDK，不复制实现。

通知中心及 Markdown/文件引用按 [ADR 008](../../../docs/decisions/008-notification-center.md)：独立正式发送权限、服务端全部组织范围/去重/幂等，原始 HTML/危险 URL 不执行；收件人附件入口用本人+有效归属+消息绑定 SQL 授权，不能扩大通用文件范围。变更同步公共能力 Skill 并验证收发、越权、失效归属和附件下载。

## WebMCP 页面工具

按 [ADR 009](../../../docs/decisions/009-webmcp.md) 保持本机 loopback、精确 Host/Origin、Bearer 配对和显式连接。注册表在断开/401/退出/切部门/身份变化后撤销，旧工具句柄不能执行。仅 allowlist 页面工具，输入严格验证；禁止任意 JS、凭证/缓存导出和业务写操作。账号工具只使用当前已授权快照，范围外 ID 拒绝，原后端权限/SQL 范围不放宽。安装器只在用户明确点击/CLI 时修改其本机客户端配置，备份合并，禁止任意参数变成命令或路径。新工具补投影、允许/拒绝与生命周期测试，未来助手复用相同边界。

按工具权限元数据过滤客户端目录，并在权限变化时撤销旧工具；该过滤不代替服务端授权、数据范围和工具执行时的检查。公开页面不得保留已登录工作区连接。

组织、部门、角色搜索工具只更新页面筛选状态，要求对应 read 权限及已加载的授权快照，不导出记录或提交业务变更。

设置定时发布按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：正式 publish/ALL，敏感单次 settings.schedule 证明在排期事务消费，到期重建原部门 Principal 并复查权限/摘要/依赖/版本；行锁与 savepoint 保证配置和结果原子提交，取消/重启/并发不能重复发布。UI 明示设备时区并转 UTC，使用生成 SDK；后台不借旧会话或临时权执行。修改时同步公共能力与权限规则并覆盖真实数据库、状态和取消测试。

账号远程 MCP 按 [实现方案](../../../docs/mcp-service-plan.md)（首阶段实现）：复用可信身份与范围，Token 固化工具集合与当前正式权限取交集，禁用/撤销实时生效，秘密仅一次显示，不入日志/配置包；账号凭证与页面配对密钥隔离。

远程 MCP 精确规则见 [ADR 011](../../../docs/decisions/011-remote-mcp.md)。创建/轮换/撤销须 VerifyCurrentPassword 和浏览器会话/Origin/CSRF；MCP 复用 PrincipalForTask 的正式授权。数据库延迟归属检查避免清空重建误撤销，但删除后重新加入不复活 Token，停用/密码重置也撤销。扩展测试须包含重认证拒绝、并发轮换、跨账号/租户、范围外 ID、身份失效及秘密投影。助手固定远程来源，不接受任意地址/命令/路径，安装证明短时单次且校验 Token 账号/部门。


## AI 与业务人工任务的边界

按 [开发工作流](../../../docs/agent-workflow.md#eino-选型与人工节点) 区分业务审核与 Gateway 风险确认。前者需审核人自身身份、任务可见范围、禁止自审、版本条件、重复/过期决定拒绝及可展示原因；不能用启动人的权限替审核人决定，也不能让审核人身份冒充原作者继续发布。

分别确认应用内 Agent、远程 MCP、页面 WebMCP 的操作与字段范围，不能因 Entries 注册或某人有权限就推断所有渠道都获准暴露。敏感读取的 TransientResult 只控制调用结果持久化与重放；还须检查模型回答、AI 历史、事件和 Workflow 检查点。模型可提出候选结论，状态转换、版本约束和权限仍由服务端能力校验；失败、不确定或规范化后无有效规则不得默认放行。


## 持久化流程

Eino Workflow/Graph/ADK 的业务节点仍经 Gateway；嵌套来源强制 OriginCapability，继承深度并拒绝事务内互调。恢复和节点前重建原账号/部门身份；Agent 工具版本白名单与当前权限取交集，禁止递归流程。run/read/cancel 权限独立，查询/取消强制 tenant+account；MCP Token 不自动获得新工具。检查点加密绑定 tenant/run/key，不进入明文审计。取消撤销未执行审批；已在执行或已提交动作不回滚。恢复读取原 invocation，独立流程动作不能因相同载荷共用审批。验证按 docs/orchestration-plan.md（仓库根路径）。

文章审核验证作者隔离、跨租户、版本条件、自审/自动化拒绝、伪造结果拒绝，以及正文不进入事件、调用审计或 AI 轨迹；敏感只读使用 TransientResult 禁止持久化与重放。
