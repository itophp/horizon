---
name: baseline-web-development
description: "Build or modify React pages, navigation, shared UI, forms, tables, or frontend API consumers in apps/web. Apply the existing shadcn design system, server-state and generated SDK boundaries."
---

# 前端功能开发

先读 [前端规则](../../../apps/web/AGENTS.md)，公共 UI 变更再读 [组件规则](../../../apps/web/src/components/ui/AGENTS.md)。

- 页面进 features/<feature>，路由/布局/Provider 进 app。跨功能复用才进入 shared；领域逻辑不放入 UI 原语。
- React Router 管理 URL 和页面导航；可分享的筛选条件优先放 URL，不复制为多份全局状态。
- 查询和写操作通过 Orval 生成 SDK / TanStack Query 接入，更新后失效相关 query key；跨用户/租户切换需清理或隔离缓存。组件不用裸 fetch 或自建 API DTO。
- TanStack Query 保存服务端状态；React 局部状态保存临时交互。不要为了表单或接口结果引入 Zustand。
- React Hook Form + Zod 管表单与前端输入反馈；提交前后区分 pending/error/success。服务端继续做验证和授权；重复提交、失败重试和 mutation 幂等按实际语义设计。
- TanStack Table 9 使用显式 features，data/columns 引用稳定；仅真有复杂电子表格式编辑需求才评估 AG Grid。
- 从 components/ui 与现有 styles tokens 复用。需要新原语时用锁定 shadcn CLI / 官方 Registry，审查新增依赖和许可证，不为每个页面新建视觉系统。
- 新可见文本、错误和辅助名称触发 baseline-internationalization。日期/数值用 lib/format，不改变服务端原值。

可观察验收至少涵盖实际用户路径和相关 loading/error/empty 状态；表单补组件交互测试，关键跨层流程补 Playwright。使用 label、role、focus-visible 和键盘交互。前端隐藏按钮不代替权限测试。

执行 pnpm lint、pnpm typecheck、相关 Vitest、pnpm i18n:check、pnpm build；关键交互在授权范围内只跑映射命中的单个 spec（E2E 默认暂停，见根 AGENTS.md）。纯样式改动不要求发明镜像实现的单元测试。

权限矩阵和临时权限页以服务端 PermissionDefinition 的 module/function/code 为源，一级模块用语义化 rowSpan 合并。申请人入口与审批入口分开，分别展示原始请求/最终批准，沿用现有表单、弹窗、列表和浅暗主题；审批上限及有效期只由后端判定，细则见 [ADR 003](../../../docs/decisions/003-temporary-access.md)。

账号多部门界面遵循 [ADR 004](../../../docs/decisions/004-department-memberships.md)：每个归属分别保存角色和覆盖，主部门与附加部门明确区分；使用生成 departmentAccess 契约显式替换附加归属。切换当前部门调用生成 SDK、取消旧请求、清除私有 Query 缓存并重建界面，禁止合并部门权限。显示临时申请绑定部门。权限相关交互变更同步 baseline-security 与对应规则。


账号编辑按 ADR 004 区分归属管理和所选部门策略编辑，使用明确的“部门与权限”入口。不能因为目标是内置管理员就隐藏其本人添加附加部门的控件；只锁定受保护主部门策略，附加部门独立编辑。补内置管理员、普通账号、无权限、其他操作者及只读模式的交互回归。


## 系统治理能力

系统治理使用 features/governance 与生成 SDK，沿用现有后台风格。发布/回退和一次性重新认证交给后端；不在浏览器持久化证明/邮件密码。配置及选项 Query 按租户/账号/当前部门/语言/目标版本隔离；服务端偏好优先，旧浏览器偏好仅首次迁移。权限矩阵除旧四种动作还须渲染目录中的发布、回退等扩展动作，一级 rowSpan 不变。详细行为见 [ADR 005](../../../docs/decisions/005-system-governance.md)。

本项目 Base UI Button 默认不提交表单：主提交动作显式设置 `type="submit"`，辅助动作设置 `type="button"`；通过真实点击到生成 SDK mutation 的交互回归验证，不能只检查按钮存在。


公共设置/字典的最新范围按 [ADR 007](../../../docs/decisions/007-global-governance.md)，取代 ADR 005 的组织配置继承/字典覆盖：统一工作空间管理，个人白名单偏好独立。管理要求正式权限及共享资源 ALL 范围，旧部门授权不得自动扩权；文件、消息、动态字典来源仍执行原资源数据范围。业务需要设置项、字典选项、上传、读取、下载或通知时，先使用 [baseline-shared-capabilities](../baseline-shared-capabilities/SKILL.md)，复用现有服务/SDK，不复制实现。

通知中心及 Markdown/文件引用按 [ADR 008](../../../docs/decisions/008-notification-center.md)：独立正式发送权限、服务端全部组织范围/去重/幂等，原始 HTML/危险 URL 不执行；收件人附件入口用本人+有效归属+消息绑定 SQL 授权，不能扩大通用文件范围。变更同步公共能力 Skill 并验证收发、越权、失效归属和附件下载。

设置定时发布按 [ADR 010](../../../docs/decisions/010-settings-schedules.md)：正式 publish/ALL，敏感单次 settings.schedule 证明在排期事务消费，到期重建原部门 Principal 并复查权限/摘要/依赖/版本；行锁与 savepoint 保证配置和结果原子提交，取消/重启/并发不能重复发布。UI 明示设备时区并转 UTC，使用生成 SDK；后台不借旧会话或临时权执行。修改时同步公共能力与权限规则并覆盖真实数据库、状态和取消测试。
