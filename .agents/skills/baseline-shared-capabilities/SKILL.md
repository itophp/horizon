---
name: baseline-shared-capabilities
description: "Integrate shared system settings, dictionaries, private file upload/read, or notifications into business features in this repository. Use when a feature needs a configurable value, selectable reference data, an attachment/logo, download, or event notification; reuse the existing platform services. Does not apply to merely reading or editing repository files."
---

# 公共能力接入

先读根到目标目录的 AGENTS.md、[公共能力接入约定](../../../docs/shared-capabilities.md) 和 [ADR 007](../../../docs/decisions/007-global-governance.md)。本 Skill 用于业务功能消费公共能力，也用于扩展公共模块；不能只在修改设置/字典管理页面时才使用。

## 先判断能力归属

| 功能需要 | 优先复用 | 禁止另建 |
|---|---|---|
| 管理员可调整的运行参数、品牌、安全/上传策略 | 注册式系统设置；后端 `Core.Setting`，前端 `useConfiguration` | 独立配置表、页面 localStorage 常量、没有真实消费者的开关 |
| 本人的语言、主题、分页或通知偏好 | 本人偏好白名单、`settingsPreferences`、统一语言/主题入口 | 个人设置覆盖安全参数，按部门复制一套系统配置 |
| 可维护的分类、标签、树形选项 | 字典及 `dictionariesOptions` / `DictionaryOptions` | 每个业务页面一份硬编码业务字典或复制字典 CRUD |
| 组织、部门、角色、账号选项 | 受控动态来源，复用原资源权限/数据范围 | 静态复制实体、任意 SQL/URL 数据源、借字典开放实体批量导入导出 |
| 上传、Logo、附件读取/下载 | 文件服务、生成 SDK、已有下载辅助函数 | 裸文件路径、公开磁盘目录、业务自建上传服务或直接接第三方存储 |
| 站内或邮件事件通知 | 持久化事件/队列、通知服务、模板注册 | 在请求/事务内直接发 SMTP，或只发内存事件 |

权限代码、账号状态和审批状态等关键枚举继续由 Goa 契约维护，不能改成普通可编辑字典。数据库连接、加密根密钥、允许存储根目录和 SMTP 出口白名单属于部署配置，不能伪装为普通业务设置。

## 设置项

1. 搜索 `internal/governance/registry.go` 与已有消费者，复用同义键；新增稳定命名空间键时声明类型、默认值、校验/上下界、敏感性和消费者。
2. 系统设置、字典是当前工作空间公共能力，不关联企业/公司/部门配置范围。仅明确允许的展示/通知键可由本人偏好覆盖；公共不等于跨租户或免权限。
3. 后端注入现有共享 Core，用 `Setting(ctx, principal, key)` 读取有效值，再以 `governance.String/Number/Bool` 解码；业务校验必须在后端执行。前端非敏感展示值使用 `useConfiguration`，不要把管理接口当普通页面初始化接口。
4. 新键同时实现真实消费者和 en/zh-CN 标签；必要时更新分组、个人白名单及运行时投影。系统公共配置修改经过既有草稿/发布/版本/审计流程；敏感发布和回退保留单次重新认证。
5. 系统设置定时发布复用 `settingsSchedules` / `settingsSchedule` / `settingsCancelSchedule`，先保存草稿，再由持久化后台 worker 执行；消费者只读取已发布值。敏感排期使用 `settings.schedule` 单次重新认证证明，不能复用立即发布证明或保存长效令牌。执行时重查原账号/部门正式权限与 ALL、内容摘要、依赖及版本，取消/失败不覆盖当前配置；语义见 [ADR 010](../../../docs/decisions/010-settings-schedules.md)。不要用前端定时器或额外脚本复制发布逻辑。
6. 区分恢复默认值、显式空字符串、false、0；不接受未注册键或 null。秘密留在后端，加密保存，不能通过运行时 DTO、导出、差异、日志或浏览器缓存读取明文。

## 字典

- 先决定用契约枚举、静态 flat/tree 字典，还是受控动态提供器。定义和字典项统一共享，不新增部门覆盖或配置范围选择。
- 前端通过生成的 `dictionariesOptions` 或 `features/governance/dictionaries.tsx` 的 `DictionaryOptions` 读取当前语言选项；Query 保留身份/当前部门/语言隔离，因为动态来源仍有业务数据权限。
- 保存引用使用稳定 ID/代码。停用项不允许新选，但编辑无关字段应保留已授权历史引用；需要解析历史项时传明确 ids，不把历史 disabled 选项重新当可新选项。
- 业务拥有记录先由原资源权限和 SQL 范围加载；在同一事务用 `dictionaries.Service.ReplaceReferences` 维护引用。resourceType/resourceID/previousIDs 必须来自可信业务记录，不能相信客户端提交的旧引用。删除业务记录时同步清理其引用。
- 新动态来源先扩展受控契约/提供器并复用原资源授权，只投影选项字段；不绕过账号/组织等原服务的范围或敏感字段限制。

## 文件上传、读取和下载

- 前端使用生成的 `filesUpload`、`filesContent`，或复用 `features/governance/files.tsx` 的 `downloadFile`。当前传输是有界 JSON base64，content 是编码字符串，不是 FormData/Blob；先检查实际 SDK 和上传限制再实现界面。
- 业务字段保存 file ID；服务端验证文件存在、租户、用途和关联权限，不接受客户端的 tenant/owner/磁盘键。不要持久化物理路径、加密存储配置或临时浏览器 blob URL。
- 普通附件和 Logo 走已有大小、扩展名/真实内容、像素及失败清理校验。需要更严格限制时扩展公共验证规则，不在页面独立放宽。
- 私有文件始终通过应用鉴权入口读取。若业务附件需要比通用 files:read 更窄的实体级权限，先扩展统一绑定/授权与 SQL 读入口；仅隐藏下载按钮不能保护附件。
- 公开品牌 Logo 只用发布绑定入口 `brandingLogo`；登录后品牌使用 `settingsLogo`。上传一个文件不等于允许公开它。
- 新文件绑定上传时不可变的 driver/config-version/key；读取旧文件必须使用原版本，不能用当前目录拼接旧路径。新增驱动注册到共享文件服务，不在业务模块自行创建存储客户端或自动搬迁旧文件。

## 通知及交付

通知中心独立导航（收到/发起/已发送/投递记录），SMTP 参数留在系统设置。人工通知按 [ADR 008](../../../docs/decisions/008-notification-center.md)，使用 `notificationsAudience` → `notificationsPreview` → `notificationsSend`，独立正式 notifications:send；混选账号/部门/公司/企业，部门含子部门，组织目标须全部在正式范围内，服务端发送时重新展开、按账号去重。发送 UUID 绑定内容，失败重试保持同键；改正文/对象后使用新键。主体、收件快照、附件、投递同事务，不能前端循环逐人发送。

Markdown 复用 `NotificationMarkdown`（GFM、禁原始 HTML/MDX/危险 URL/外部图片），文件仍走 filesUpload 并保存 ID，以 受控 `file:ID` Markdown 链接/图片引用 插入。服务端确认发送者拥有文件且用途为 attachment，正文引用须在附件列表，绑定后不可删除。邮件深链用 `notificationsMessage` 加载本人通知详情；通知收件人用 `notificationsAttachment`：SQL 校验本人、有效归属和消息文件绑定，复用原存储版本；不要求给收件人全局 files:read，也不能用通用文件链接绕过绑定。发送者预览仍按通用文件权限。

自动事件源继续支持临时权限申请/批准/拒绝/审批人撤销，新增自动事件须扩展事件契约与持久化，不能伪造临时申请 ID。人工消息用 announcement，不套用临时权限事件。复用通知偏好、模板/持久化重试；投递前复验接收身份/归属/权限。SMTP 凭证不进入前端日志或仓库。

按涉及层加载 baseline-web-development、baseline-internationalization、goa-service-designer、baseline-database-change、baseline-security 和 baseline-quality-gates。只消费现有 SDK 时不要改契约/迁移；扩展公共能力时同步本 Skill、接入文档和相应规则。

验证真实行为：设置发布后消费者生效、个人偏好优先级、动态来源越权、停用历史引用、上传超限/伪类型、跨租户/范围外下载、存储切换后旧文件可读。需要 SQL 用隔离 PostgreSQL，SMTP 用隔离服务器。执行时机遵循 [批次规则](../../../docs/testing.md#测试执行节奏)：普通开发累计待验项，阶段验收统一执行 ci-fast 和相关专项检查，发布候选/明确全量验收/CI 才执行默认 ci；生成检查先于浏览器验收。未实现的绑定或消费者必须如实报告，不能把接上一个按钮当作完成。
