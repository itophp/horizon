# 服务、安全与数据访问实现

继承服务规则，安全相关开发加载 baseline-security；规则详见 docs/security.md。

- auth 验证凭证、建立可信 Principal；authorization/Casbin 管资源操作；dataaccess 解析可信范围；repository/sqlc 在 SQL 层强制过滤。四个职责不能互相省略。
- 受保护请求默认拒绝；客户端 tenant/organization/department/owner 参数只能作为已授权范围内的业务输入，不能决定当前身份/权限。新增公开接口显式说明理由。
- 每个新资源说明 owner 的含义和 ALL/ORGANIZATION/DEPARTMENT/DEPARTMENT_TREE/OWN/CUSTOM 的解析；所有访问入口保持同等约束。详情/写入不能先无范围读取再判断。
- 策略只通过约定模型/策略载体更新并测试；缓存须有撤销失效策略，跨进程行为如实说明。禁止各 Handler 复制隐式权限。
- 响应与输入都用明确字段投影/白名单；新敏感字段检查 DTO、日志、错误、导出与批量入口。SQL 选择最小必要列。
- 数据层错误映射为稳定服务错误，不回传堆栈/SQL/凭证。context 传到数据库，事务不跨池误用，失败不吞异常或默认全量访问。
- 新增安全能力须有允许/拒绝、跨租户与范围外 ID 测试；适用时覆盖更新/删除、关联/聚合/批量/导出和撤销。运行 make test-security。

正式后台位于 internal/identity：Service 管用例和不变量、policy.go 管 Casbin/可信范围、Repository 调用生成的 dbgen、transport/identity.go 校验会话与 Origin/CSRF。旧 auth/authorization/dataaccess 为独立技术 demo，不要把 demo token 接到后台或复制第二套登录。身份变更参考 docs/decisions/002-identity-access.md，保留账号覆盖替换范围、内置管理员保护、授权管理独立门禁及即时撤销。

临时权限与两级目录遵循 [ADR 003](../../../docs/decisions/003-temporary-access.md)：目录由 identity/catalog.go 定义并通过 Goa 输出；请求默认只能读取本人记录，独立正式 temporary:approve 才能审批他人。审批不能超出本人正式权限/数据范围，不能自批或再委托临时范围；原始申请和最终批准分开存储。范围在批准时固定解析，每次认证用数据库时间排除过期/撤销记录，deny 优先。修改这些行为必须覆盖 tests/temporary_access_test.go 的负向与并发用例。


多部门授权遵循 [ADR 004](../../../docs/decisions/004-department-memberships.md)：一个 Principal 只解析当前已验证部门归属的角色/覆盖/范围，禁止跨部门合并。持久授权修改及多部门账号全局敏感变更必须覆盖所有原有和目标部门；临时申请与生效绑定申请部门。任何认证、权限控制或数据范围行为变更，必须同步维护权限控制相关的 Skill、目录 AGENTS.md 和安全/架构规则，并补齐受影响的允许与拒绝测试；交付说明列出同步位置，不允许只改代码。

系统治理遵循 [ADR 005](../../../docs/decisions/005-system-governance.md)：共享 Core 复用 identity 正式授权，配置拥有范围不套用账号任意归属可见；设置/字典发布必须版本锁和同事务快照/审计，敏感发布及回退消费单次绑定重新认证。秘密不进 DTO/历史/导出；字典动态来源只用受控提供器。文件绑定不可变存储配置、仅受控品牌 Logo 公开；通知投递前重查接收上下文及正式审批能力。相关权限变更同步 Skill/规则并覆盖 governance/SMTP/范围负向测试。

公共设置和字典按 [ADR 007](../../../docs/decisions/007-global-governance.md) 统一管理，不新增组织范围或字典覆盖。业务接入先读 [公共能力 Skill](../../../.agents/skills/baseline-shared-capabilities/SKILL.md)，复用注册键、选项/引用、鉴权文件服务和通知事件；文件、动态来源及消息仍保留原资源数据范围。

人工通知按 [ADR 008](../../../docs/decisions/008-notification-center.md)：notifications:send 正式权限，组织成员全部范围校验和账号去重；announcement/快照/附件/投递同事务且幂等。附件通过 recipient+有效归属+通知文件绑定 SQL 读取，不能套用全局 files:read 放大收件人权限；旧临时权限事件不伪造或替换。

设置排期按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：排期时正式 publish/ALL，敏感证明绑定 settings.schedule；到期重建原部门 Principal 并重查正式权限、草稿摘要、版本及依赖。共用发布事务并与完成状态原子提交，保存失败码。不能用旧会话/临时权限或后台特权绕过授权；读取验证共用事务，避免跨池/单连接死锁。
