# Application Runtime 开发规范

## 职责与触发场景

Application Runtime 负责应用、导航、页面、布局、表单、表格、Provider、Action 和可信组件的声明、解释及生命周期。涉及业务界面或 Runtime 本身时使用 baseline-application-runtime；无界面要求的普通后端任务不强制加载。

代码契约、可信组件、业务能力和部署基线由 Git 管理；租户当前菜单、页面和交互以数据库已发布不可变应用版本为准。Runtime 不接管业务数据规则、IAM 或 Gateway，也不根据实体表声明自动生成 SQL。

## 先选配置、代码或混合路径

| 路径 | 选择条件 | 交付内容 |
|---|---|---|
| 配置开发 | 已有 Entity、Capability、Provider 和可信组件足以表达需求 | 配置候选、校验/差异/预览证据、精确版本及发布状态 |
| 代码开发 | 缺少业务语义、表结构、能力或可信组件实现 | 原模块源代码、生成产物、部署基线及行为测试 |
| 混合开发 | 通用页面包含专用交互 | 定义承载通用部分，可信组件或插槽承载复杂区域 |

普通业务列表、详情、表单和按钮优先复用 Runtime；平台管理页、专用编辑器及复杂视图按实际能力选型，不强制迁移。不复制通用页面，不在引擎内增加业务字段、权限常量或 SQL。缺少业务能力时回到原模块开发，不通过页面定义补写业务逻辑。

## 定义与绑定

开发前核对实际能力目录、实体、当前贡献和已有组件。Provider、创建/编辑命令及 Action 分别绑定固定能力版本；保留真实输入类型，检查派生字段、引用 Provider、输入映射、字段投影及预算。展示条件不替代服务端授权，所有业务调用继续经 Gateway。

### Schema Protocol v3

V3 应用以 `ApplicationV2.protocol: ApplicationV3` 随部署定义发布，外层元数据必须与内层 `ApplicationV3` 一致，`schemaVersion` / `runtimeCompatibility` 均为 `3.0`。页面由 Layout、Data、Component 三类声明组成：Layout 可嵌套并映射 CSS Grid；Data 绑定只读 Provider 并以 `$id` 供后代引用；Component 只能引用 `kind@version` 注册组件。一个 Layout 下最多一个 Component，或任意多个子 Layout。

装配时 V3 Provider 使用同一能力目录绑定权限、输入/输出 Schema 和版本；事件里的 Capability 调用同样必须是已注册版本。目录投影按应用、页面、Provider 和事件权限移除不可见声明，前端只做展示控制，实际读写仍由 Gateway 授权。`schemaVersion` / `runtimeCompatibility` 不是 `3.0` 或缺少 `protocol` 的页面定义会被装配与前端校验拒绝，不再提供 V2 页面兼容路径。当前内置 `/runtime-protocol` 是使用真实 `identity.accounts.admin.read` 的 V3 冒烟页，用于验证目录、路由、Schema 渲染与授权数据链路。

布局节点只负责排列、尺寸、间距和响应式。需要边框、背景、标题或内容容器时，在布局中放置 `card@1.0`，用组件的 `content` 插槽承载下一层布局及内部组件。卡片插槽参与作用域、深度与权限校验。

`dialog@1.0` 使用 `content` 插槽编排弹窗正文。通过组件绑定将页面局部布尔值映射到 `open`，并用 `close` 事件将其设回 false。按需读取弹窗数据时，Provider 的 Data 声明应通过 `enabledWhen` 引用同一状态。只读预览不打开交互弹窗。

`change-list-form@1.0` 从 Schema 读取分组、字段和选项，从绑定数据读取键、JSON 值与覆盖状态。它只发出实际发生的 `set` 或 `reset`，由页面的 `submit` 事件将变更映射到目标能力。个人偏好在 `builtin:personal-preferences@1.0.1` 使用该组件，已有发布定义通过 Runtime 安装升级流程更新。

`/capabilities` 由 `builtin:capabilities@1.0.1` 的 V3 定义提供。部署迁移 00031 为现有租户写入安装记录，并为新租户登记默认安装模板，服务启动时由 Runtime reconcile 初始化发布定义。00032 将安装模板和未初始化的安装推进到 1.0.1，已有发布或草稿通过 Runtime 安装升级流程处理。系统 Provider 只接受固定适配器，复用现有 SDK 和 Gateway 的授权投影。调用记录、审批和事件绑定只保留显示所需字段。部署内置页面可声明 `authenticated` 可见范围，页面内的 Provider 和动作继续各自鉴权。


Phase 4 起，账号、组织和权限管理页已由 `system.identity` V3 定义提供，当前部署包为 `system.identity@2.3.0`，并通过通用 `table-panel`、`page-tabs`、`tree-table-panel`、`cards-panel`、`master-detail-panel` 与表单 Workbench 渲染。原账号/组织/权限 React 页面、custom-view 贡献和旧页面路由已移除。浏览器验收覆盖账号列表与账号表单、组织/部门两个视图、角色卡片/列表与账号独立授权，实际数据分别来自 `identity.accounts.admin.read`、`identity.organizations.read`、`identity.departments.read` 和 `identity.roles.read`。

Phase 5 起，文章与项目也全部改为 V3：`builtin:articles@2.0.0` 提供列表、详情、创建与编辑页，`com.horizon.projects@2.0.0` 提供项目、概览/计划/任务/成员详情页、项目任务与任务详情页。前端只保留通用 `detail-panel`、`record-form-panel`、`search-picker-panel`、`markdown`、分页表格和事件动作适配；文章/项目专用 React 路由、页面和扩展贡献已删除。Provider、表单命令与事件调用都在装配时绑定能力输入 Schema，前端按 Schema 做类型转换后仍经 Gateway 执行。设计契约同样只暴露 V3 Layout、Component 与 Interaction，不再暴露 V2 Composition/container/custom-view 目录。

`/analytics` 由 `builtin:analytics@1.0.0` 提供，内置安装覆盖现有租户和新租户。作为已登录用户的默认回落入口，应用和部署包均受停用保护。页面采用响应式网格、卡片、`layout-tabs` 与 `layout-tab-panel` 内容插槽。指标和图表读取 `identity.analytics.read@1.0.0` 投影，近期列表只显示当前范围内的创建记录。

`/settings`、`/preferences` 和 `/mcp` 分别由 `builtin:system-settings@1.0.0`、`builtin:personal-preferences@1.0.0` 和 `builtin:agent-collaboration@1.0.0` 提供。迁移 00035 为现有租户登记安装并提供新租户默认模板。设置保留侧栏入口，个人偏好和 Agent 协作保留顶栏入口。原标签深链接继续映射到 V3 页面。

字段、Token 和审计列表复用 Runtime 表格，WebSkills 使用声明式内容。可信组件复用既有设置发布、字典编辑、凭据管理和本机连接流程。表格刷新由 V3 事件触发，业务写入仍走既有 Gateway 能力。只读预览允许查看已授权设置和列表，禁用变更及本机连接操作。

Go Runtime 契约通过现有生成器输出 TS/Zod 和示例；修改手写源后生成，禁止手改生成结果。部署贡献是可信基线，数据库保存已发布定制；不能直接改版本表、用重启/部署覆盖定制，或把单租户配置自动反写为公共部署默认值。无授权运行环境时可分析离线定义，但不能宣称已读取当前租户版本。

## 配置版本和发布

配置工作流：授权读取精确当前版本 → 提案 → 结构/引用/类型/权限校验 → 差异及隔离预览 → 人工发布精确候选 → 验证实际生效。

- 区分 schemaVersion、三级部署包版本、四级应用配置版本、内容摘要和运行修订。第四级由服务端分配，Agent 与人工在线修改共用序列，不能指定前三位。
- 提案绑定父版本与修订；发布绑定已审查的候选版本、摘要和修订。Agent 只形成候选，不能自行发布、回退或扩权。
- 带定制的部署升级先做三方合并和兼容检查；回退按现有规则形成新版本，不倒退配置修订。
- 预览不执行写副作用，候选不混入普通导航。验证发布后的菜单/页面、旧表单及工具失效、资源释放、安装状态和重启保留。
- 代码交付继续走修改唯一源、生成、测试、提交及按授权部署；代码提交/推送授权不代替配置发布授权。

## 验证与交付

在线纯配置修改使用精确候选校验、差异、预览和授权生效验证，不因菜单调整自动执行代码生成或全量 CI；受影响动作与权限仍需相应行为证据。引擎、组件、贡献和契约代码在阶段验收执行 ci-fast，按 SQL/权限影响追加集成或安全检查。

交付分别报告 Git revision、应用配置版本/摘要及“候选已保存、已发布、生效已验证”。离线结构校验不能代替业务行为或线上生效。仅规范/Skill 修改执行 check-rules 与作者校验，浏览器及远程交付沿用根规则。

实际实现和历史验证见 [数据库定义实施记录](application-runtime-database.md)。维护本规范不意味着新增能力、权限、迁移或业务页面。

## UI 生成质量

生成页面先执行 [UI 设计标准](runtime-ui-generation.md) 的设计说明，再选择已注册布局与组件。工程 Agent 使用 baseline-ui-design；在线 Agent 通过工作流指令获得精简约束，不假设能读取仓库 Skill。结构校验与视觉验收分别报告。
