# 前端规则

适用 apps/web，继承根规则。编码前使用 baseline-web-development；新增文案使用 baseline-internationalization；有新 API 需求先回 Goa DSL。

- app 管路由、Provider、布局；features/<feature> 管功能；components/shared 只放真实跨功能复用；components/ui 复用 shadcn 原语。API/SQL 生成目录边界不变。
- React Router 管导航与 URL 状态；TanStack Query 管服务端数据/缓存和取消；Orval SDK 是请求/DTO 来源。页面不裸 fetch，不复制 API 类型或把 query 结果再存全局状态。
- 表单使用 React Hook Form + Zod，字段错误存稳定翻译 key，动态语言切换时重新本地化；前端校验不代替后端校验。mutation 不默认重试，设计重复提交与 query invalidation。
- 常规数据表使用 TanStack Table 9 的 features API，数据与列定义保持稳定引用。复杂表格才按需求评估其他工具，不叠加同职能库。
- 新全局客户端状态需有跨页面需要，不能只为服务器数据引入 Zustand；格式化先用 Intl，不无故引入日期库。
- 复用样式 tokens，保留可访问 label/role、键盘和焦点；不自建每页视觉体系。修改公共组件需读取其局部规则并检查调用方。
- 文本、占位、空/错/载入状态、通知、表头及辅助名称全部走资源。i18n:check + lint 自动检查只是部分保障，仍需评估动态文案与 API 错误映射。
- 认证状态/按钮显示只改善体验；真正授权在服务端。不要持久化演示 token，切租户/用户时隔离或清理 query 缓存。
- pnpm 在仓库根可执行 lint/typecheck/test/build/i18n:check；单模块 Vitest 使用 pnpm --filter @baseline/web exec vitest run <路径>。交互覆盖 RTL + user-event；关键真实流程用 e2e。

## 界面风格一致性

新建页面、扩展功能和局部改版必须与当前已确认的同类界面保持一致。实现前先查看现有参考页面及源码，读取 [设计系统](../../docs/design-system.md)，确定可复用的布局、组件和 tokens；不能仅凭通用模板或个人审美另起一套视觉体系。

- 后台以现有账号、组织、部门、角色和权限页面为基准，优先复用 features/administration 的公共布局、PageHeading、DataTable、EditorDialog 与状态组件。登录页延续现有登录页风格；/foundation 技术示例不作为后台的视觉基准。
- 保持页面框架、背景层级、字体与字号、间距和信息密度、圆角、边框、阴影、按钮/表单/表格、图标及交互状态一致；浅色和暗色沿用同一套语义 tokens。不得为单个页面新增独立配色、布局体系或同职能 UI 库。
- 后台沿用侧栏底色包裹独立内容面板的结构：边框和圆角属于内容面板，正文内部滚动，顶栏/页脚与圆角保持固定；不恢复外围留白、侧栏分隔边框或整页滚动。具体数值以现行设计系统和公共样式为准。
- 缺少现成组件时，按现有视觉语言补充可复用组件；调整公共样式需核对所有调用页面。验收在相同视口下对照同类页面，检查浅/暗主题、交互状态及滚动后的整体一致性。
- 用户明确要求改变视觉方向时按最新要求执行，并同步设计系统及受影响的公共页面；一致性规则不能覆盖用户最新指示。

## 信息架构与菜单

侧栏只保留一级入口：工作台、账号管理、组织管理、权限管理、通知中心、Agent 协作、系统设置；个人偏好从顶栏进入。部门、角色、字段（字典）不再是独立菜单项，分别并入组织管理、权限管理、系统设置的标签内；附件管理只保留路由，不占菜单位置。

- 合并页面用页内标签承载各自职责：组织管理 = 组织架构 / 部门，权限管理 = 角色授权 / 账号独立授权 / 角色管理，系统设置 = 系统配置 / 字段管理，Agent 协作 = MCP Server / WebMCP / WebSkills 三个标签。新增能力按归属追加标签，不要再起平行菜单项。
- 标签可见性与默认标签由正式读取权限决定（organizations:read 或 departments:read、access:manage 或 roles:read、settings:read 或 dictionaries:read），前端隐藏只是体验，服务端授权不变。搜索等页内状态更新保留 URL 其他参数，避免切换标签丢失。
- 旧路由 `/departments`、`/roles`、`/dictionaries`、`/webmcp`、`/mcp-settings` 只做重定向，不恢复页面组件；新增或调整入口时同步 WebMCP 路由清单与 E2E。
- 顶栏「申请临时访问」为纯图标按钮，必须保留 aria-label/title；顶栏不再显示“系统管理中心”之类的装饰文字。

权限界面使用后端 PermissionDefinition 目录的一/二级层次，一级模块用真实 rowSpan 合并；不得硬编码另一套权限代码。临时申请入口对已认证账号可见，审批入口按服务端 canReview 显示。申请与最终批准分别展示，支持审批修订权限、时效和范围；前端计时/隐藏按钮不能代替服务端授权、审批上限和到期判断，语义见 [ADR 003](../../docs/decisions/003-temporary-access.md)。

账号多部门配置和上下文切换遵循 [ADR 004](../../docs/decisions/004-department-memberships.md)。每个部门独立编辑角色与权限覆盖；切换仅调用生成 SDK，取消旧请求、清除私有 Query 缓存并重建页面。不能在浏览器合并各部门授权；申请/审批显示绑定部门。权限行为改变须同步 baseline-security 及相关规则。

内置管理员账号仍禁止停用、删除及移动受保护主部门；界面中主部门策略的锁定不能连带隐藏本人添加附加部门的入口。附加部门仍独立配置角色/覆盖，遵守既有正式权限与范围校验。其他账号不能编辑内置管理员，详情模式及无部门授权管理权限时，部门策略控件保持只读。 账号编辑的入口明确标为“部门与权限”，不要用主部门策略保护开关同时关闭部门归属管理。

系统治理页面复用 features/governance 的 ChangeHistory、运行时配置和生成 SDK；系统设置与字典不再关联配置范围；发布/回退/重新认证在服务端完成，浏览器不保存证明或 SMTP 密码。Query 键包括租户、账号、当前部门、语言与目标范围/版本，切换仍清私有缓存。个人偏好以服务端值优先，原浏览器值仅首次迁移；运行时字典和 Logo 使用明确消费入口。权限目录的扩展操作不能因旧四列矩阵而丢失，仍保持一级真实合并单元格。详见 [ADR 005](../../docs/decisions/005-system-governance.md)。

业务需要设置项、选项、上传/读取/下载或通知时先使用 [baseline-shared-capabilities](../../.agents/skills/baseline-shared-capabilities/SKILL.md)，不要只在管理页面开发时才读取公共能力约定。

通知中心见 [ADR 008](../../docs/decisions/008-notification-center.md)：侧栏不放菜单，入口是顶栏右上角铃铛；投递记录不嵌入设置。人工消息复用 NotificationComposer/NotificationMarkdown、受控组织选择与生成 SDK；禁止 raw HTML/MDX 和任意文件 URL，通知附件使用绑定鉴权入口。收件人预览后发送时仍以后端重新解析为准。发起通知是通知中心的按钮入口，跳转独立页面 `/notifications/compose`，正文使用 features/governance 封装的 MDXEditor 所见即所得编辑器；工具栏不提供上传插入文件、源码/差异/预览浏览，新增编辑器文案走 i18next 且浅暗主题沿用后台 tokens。新增路由同步 WebMCP 路由清单。

WebMCP 接入遵循 [ADR 009](../../docs/decisions/009-webmcp.md)。页面工具通过 features/webmcp 与 lib/webmcp 注册白名单，随路由卸载；不能直接暴露任意 JS、Query 缓存或凭证。账号工具只操作已有授权数据的只读详情/筛选，业务写入未开放。连接显式开启，身份/部门变化和退出/401 必须撤销；新增工具补输入、权限拒绝、范围外 ID 和生命周期测试。开发服务器 `/__webmcp/connection` 只用于把本机服务的端口与配对密钥预填进表单：仅 serve 生效、Host 必须回环、Origin 必须同源、`no-store`、无 CORS 头、失败返回 404；不得扩展为任意文件或配置读取，构建产物中不得出现该端点。未来助手复用同一工具定义，不绕过生成 SDK 或后端范围。远程 MCP、WebMCP 与 WebSkills 合并在 `/mcp` 单页，按标签切换（`?tab=remote|page|skills` 只是入口提示）；新增 Agent 接入内容按标签拆成可复用区域，不再另起独立页面，旧 `/mcp-settings`、`/webmcp` 只保留重定向。

新增路由同步 WebMCP 路由清单并声明是否允许连接；新增工具声明 global/module/page 层级、权限及操作类型，测试应验证其可用范围和撤销。模块/页面上下文是基础适配，不代表业务流程已适配；业务操作应复用页面真实动作和现有服务端授权，并通过真实 MCP 链路及必要的用户交互测试验证。

组织、部门及角色的搜索工具复用现有列表搜索状态，只返回请求确认；必须验证对应 read 权限、加载状态以及真实显示结果。

系统设置定时发布见 [ADR 010](../../docs/decisions/010-settings-schedules.md)：保存草稿后调用生成的排期接口；datetime-local 明示设备时区并转 UTC，不能靠浏览器定时器执行。pending 草稿不再直接发布，支持取消及状态/错误展示；敏感排期走单次重新认证，状态变化刷新已发布配置与历史缓存。

账号远程 MCP 按 [ADR 011](../../docs/decisions/011-remote-mcp.md)：工具复用原授权用例，凭证绑定本人/部门和固化集合；敏感管理操作重新认证，身份变更撤销与延迟归属检查、客户端配对/备份及协议专项测试同步维护。禁止将安装状态当连接成功。
