# 公共能力接入约定

业务功能需要设置项、可维护选项、附件上传/读取/下载或通知时，先加载 [baseline-shared-capabilities](../.agents/skills/baseline-shared-capabilities/SKILL.md)。配置和字典统一共享；身份授权、动态选项、文件和消息的数据权限仍按当前可信上下文执行。范围决策见 [ADR 007](decisions/007-global-governance.md)。

| 能力 | 前端消费入口 | 后端/扩展源 |
|---|---|---|
| 系统设置 | features/governance/runtime.tsx 的 useConfiguration；管理页不能代替普通消费接口 | system/governance/registry.go 注册键；Core.Setting + String/Number/Bool 解码；注入已有共享 Core |
| 本人偏好 | 生成 settingsPreferences，统一 changeLanguage / ThemeProvider | settings.Preferences，注册项的 account 白名单 |
| 选项与历史解析 | 生成 dictionariesOptions；DictionaryOptions 组件；历史 ids 参数 | dictionaries.Options；受控来源复用 identity.ReferenceOptions |
| 持久化字典引用 | 业务表单保存稳定 ID/代码，不保存临时显示标签作为身份 | 原业务范围查询 → 同事务 ReplaceReferences；旧引用从数据库获得 |
| 上传 | 生成 filesUpload；有界 JSON base64，当前 SDK content 为 string | files.Upload / detect / 注册式 Driver；租户、用途、实际大小和类型验证 |
| 读取/下载 | 生成 filesContent 或 downloadFile；只存 file ID | files.Content 及业务绑定授权，不暴露磁盘路径；旧文件用上传时 profile |
| 品牌 Logo | brandingLogo（部署绑定公开）/ settingsLogo（本人有效品牌） | 已发布品牌绑定，不能公开任意上传文件 |
| 通知 | notificationsInbox / notificationsMessage / notificationsRead / 本人偏好 | notifications.Service、事务事件及持久化投递；新事件类型需扩展受控契约 |

唯一 API 源为 foundation/design，SQL 源为 foundation/db/migrations 与 query，客户端使用 apps/web/src/api/generated；生成物不可手改。页面与业务服务不能直接写 gov_values、手工拼选项、裸 fetch 上传、公开服务器目录或自行发送 SMTP。

新增参数必须有后端真实消费者、受控默认值/验证和双语文案；敏感值仅服务端读取。静态字典与受控动态实体选项分开，关键状态/权限枚举保留契约。文件关联如果需要业务实体级权限，先扩展统一绑定和授权 SQL，不能只隐藏链接。

公共能力变更同时维护对应 Skill、接口/消费者、权限、文档和测试。设置要测发布生效与个人默认；字典要测来源越权与历史；文件要测实际限额、伪类型、路径、范围外下载及旧配置读取。命令和交付门禁见 [testing.md](testing.md)。

人工通知使用 notificationsAudience → notificationsPreview → notificationsSend，查看 notificationsSent；独立通知中心管理站内/SMTP 投递，SMTP 参数仍在系统设置。Markdown 使用 NotificationMarkdown，上传后保存附件 ID 并插入 file:ID，通知收件人的读取走 notificationsAttachment；不得要求收件人先获得全局文件管理权限。完整边界见 [ADR 008](decisions/008-notification-center.md)。

设置排期复用 settingsSchedules / settingsSchedule / settingsCancelSchedule 和现有草稿。敏感排期先调用 settingsReauthenticate（settings.schedule），证明只在提交时使用；消费者仍只读已发布设置。后台必须按 [ADR 010](decisions/010-settings-schedules.md) 重查原账号/部门正式权限、版本与依赖，不复制一套发布逻辑或用浏览器定时器执行。

## AI 模型管理

「AI 配置」是独立的工作空间管理入口，路径为 `/ai-settings`。系统设置 Runtime 基线 `1.0.3` 同时增加该页面、移出通用设置表单中的模型字段。新工作空间使用该基线，旧工作空间通过「应用管理」升级已有系统设置应用。定制版本按现有三方合并流程审查和发布。

- 默认模型、模型管理、业务模型分配通过 URL 标签切换。默认模型为 DeepSeek-V4.1-Flash，请求模型名为 `deepseek-flash`。各业务覆盖优先，选择「使用默认模型」时跟随工作空间默认。
- 模型管理沿用 DeepSeek、GLM 和自定义三个配置项，类型为纯文本、向量和多模态。DeepSeek 归为多模态，GLM 和自定义项可修改类型。向量类型不参与对话模型选择，现有知识库向量化继续使用独立配置。
- 配置沿用原有模型键和加密密钥。草稿、发布、排期和回退使用同一个工作空间设置版本及既有授权。选中的新模型或受类型变更影响的模型缺少配置时，设置校验指出具体模型和场景。纯文本模型收到图片请求时，运行时返回模型类型提示。
- 页面采用 B5 设置模板和 L6 表单布局，模型管理复用 DataTable、类型筛选和 EditorDialog。主操作为保存草稿，后续在页面共用的发布面板完成确认。配置加载、只读、空筛选结果及提交失败沿用后台状态组件。
