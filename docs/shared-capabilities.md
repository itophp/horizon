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

「AI 配置」由平台托管应用 `builtin:ai-settings@1.0.3` 提供，入口为 `/ai-settings`。模型、AI 开关和知识库相关配置由 AI 应用统一管理。现有 PostgreSQL 实例中的 `ai_configuration` 按工作空间存储 AI 配置与修订号。迁移 00047 转移已生效的 AI 值和原有加密凭证，历史草稿保留审计记录。

- AI 配置通过 `ai.config.read@1.0.0` 和 `ai.config.save@1.0.0` 读取及保存。保存经 Gateway 使用事务修订校验，提交后供下一次 AI 调用读取。权限为正式 `aiConfig:read` 和 `aiConfig:update`，均要求工作空间 ALL。
- AI 配置保存只接受注册的 AI 键，系统设置保存和回退只处理系统键。凭证在服务器加密保存，读取只返回配置状态，能力调用记录和变更事件不记录值。
- 所有模型使用同一个新增和编辑表单。DeepSeek、GLM、OpenAI 兼容向量模型和阿里云百炼是预填条目。字段的适用性由模型类型和协议决定，GLM 的旧页面聊天地址通过可选的 Responses 地址保留。纯文本及多模态模型可用于默认模型和业务分配，向量模型可用于知识检索。
- 模型管理复用 DataTable、URL 类型筛选和 EditorDialog。默认模型、业务分配和 AI 服务有各自的 URL 标签。

### 知识检索向量模型

模型管理提供 OpenAI 兼容向量模型和阿里云百炼 `qwen3-vl-embedding`。原有 OpenAI 兼容连接字段在模型管理中使用原配置键。「AI 服务 → 知识检索」选择向量模型并管理 Redis 连接和索引名。百炼预置北京 DashScope API 基础地址，允许改用与密钥地域匹配的百炼业务空间地址。DashScope 协议按 [百炼多模态向量 API](https://docs.bailian.console.aliyun.com/zh/model-studio/multimodal-embedding-api-reference) 验证模型、地址和向量维度。百炼预填 2560 维，使用其他受支持的维度时生成匹配的独立索引。

更改选型后，下一次请求读取已提交的配置。不同向量连接使用独立的 Redis 索引与键前缀。原有 OpenAI 兼容与百炼默认配置可继续读取原索引。已有文档通过知识库的重建向量操作写入新索引。API 密钥复用 AI 配置的按租户密封和脱敏，重定向响应不转发凭证。检索命中由 PostgreSQL 批量复核文档在当前工作空间是否仍存在。
