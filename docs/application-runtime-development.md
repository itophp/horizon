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

调用声明了 `TransientResult` 的人工写入能力时，事件可用 `disclosure` 显示一个契约中声明的字符串字段。此字段只出现在首次成功响应的弹窗中，关闭后清除。瞬时结果禁止绑定到 `resultTarget`，也不进入页面表达式或缓存。

`action-form@1.0` 按 Schema 渲染动作输入。一个表单只允许一个 `submit` 调用，提交值用于构造该次调用的输入。失败后的重试由表单发起，输入未改动时复用操作键，Runtime 的通用错误弹窗不保存提交值。成功或取消事件没有表单值，可用于关闭弹窗。

展示绑定可读取当前会话的 `context.activeDepartmentId` 与 `context.departmentOptions`。选项仅包含本人归属的部门 ID 和名称。凭据表单可用它们声明部门选择与默认值，提交时仍由 Gateway 和业务服务校验部门归属。

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

## shadcn/ui 组件

Runtime 支持[用户提供的 shadcn/ui 组件目录](https://www.shadcn-ui.cn/docs/components)中的 59 项。有 `applications:design` 权限的用户可在 `/applications/design` 查看组件契约。Runtime 设计 Agent 通过 `application.runtime.definition.contracts` 读取同一目录，先用 `kind: "element"` 分页查询，再用 `name: "<组件>@<版本>"` 查询属性、插槽和事件。页面声明使用 `kind` 和 `version` 固定版本。完整属性、内容插槽、数据绑定与事件见[生成的 UI 契约](generated/runtime/ui-contracts.json)。运行时组件复用仓库的 Base Nova 和主题 tokens。

| 用途 | Runtime 组件 |
| --- | --- |
| 内容与状态 | `alert`、`avatar`、`badge`、`card`、`empty`、`item`、`kbd`、`label`、`progress`、`skeleton`、`spinner`、`typography` |
| 内容布局 | `accordion`、`aspect-ratio`、`button-group`、`carousel`、`collapsible`、`resizable`、`scroll-area`、`separator`、`sidebar`、`tabs` |
| 操作与导航 | `breadcrumb`、`button`、`command`、`context-menu`、`dropdown-menu`、`menubar`、`navigation-menu`、`pagination` |
| 浮层与反馈 | `alert-dialog`、`dialog`、`drawer`、`hover-card`、`popover`、`sheet`、`sonner`、`toast`、`tooltip` |
| 输入 | `calendar`、`checkbox`、`combobox`、`date-picker`、`field`、`input`、`input-group`、`input-otp`、`native-select`、`radio-group`、`select`、`slider`、`switch`、`textarea`、`toggle`、`toggle-group` |
| 表单与数据 | Form 用 `action-form` 或 `record-form-panel`；表格与图表用 `table`、`data-table`、`chart` |

`content` 是嵌套布局。Carousel 在其内容布局下为每张卡片声明一个包含 `carousel-slide` 的布局，Resizable 同样使用包含 `resizable-panel` 和 `resizable-handle` 的布局，并按面板、手柄、面板的顺序声明。两者的内容根及直接子布局只提供声明和作用域，尺寸由组件管理。Accordion 可设置 `group: true`，并在内容布局内声明多个 `accordion-item`。Tabs 的内容包含对应 `options[].value` 的 `layout-tab-panel`。

`labelKey`、`titleKey`、`descriptionKey` 使用翻译键。固定枚举的 `options` 由 `{ value, labelKey }` 组成，业务对象选择使用带授权查询的 `search-picker-panel`。Calendar 和 Date Picker 支持单选、多选和范围选择，`change` 分别返回 `YYYY-MM-DD` 字符串、日期数组或含 `from` 和可选 `to` 的对象。常规输入控件的 `change` 返回标量值。Slider 的范围模式返回两个数字，Toggle Group 的多选模式返回选中值数组。绑定值更新后控件同步显示。固定枚举选项可用 `disabled` 禁用单项。分页链接保留现有查询参数，可在 Provider 映射中读取页面参数。设置 `numbered: true` 可直接跳到首页、末页或当前页附近的页面。Chart 使用 Provider 返回的行数据，支持柱状图、折线图、面积图和饼图。用 `series: [{ field, labelKey }]` 指定多系列，柱状图和面积图可用 `stacked: true` 堆叠。饼图读取第一组系列的正数数据。图表附可展开的数据表。Menubar 可用 `menus: [{ value, labelKey, options }]` 声明多组菜单，此时 `select` 返回 `{ menu, value }`。单组 `options` 仍返回字符串。Navigation Menu 可设置 `group: true`，通过子布局编排多个 `navigation-menu-item`。Drawer、Popover 和 Sheet 支持绑定 `open`、设置 `side`，并通过 `openChange` 返回打开状态。Toast 和 Sonner 提供由按钮触发的本地反馈。

事件通过现有 Runtime Action 分发，业务写入使用 Gateway。Input OTP 的 `complete` 只允许绑定一次已声明的能力调用，并走临时参数路径。只读预览禁用编辑和业务事件，保留展开、查看内容等本地操作。

菜单的 `options` 支持 `kind: "checkbox"`、`"radio"` 和 `"submenu"`。勾选事件返回 `{ value, checked }`，单选事件返回 `{ group, value }`，普通动作返回字符串。Menubar 分组时事件同时包含 `menu`。Combobox 多选用 `multiple: true` 和 `values`，显示选中项标签并返回选中值数组。

Avatar 支持同源图片路径 `src`、尺寸及缩写回退，多个头像可放在 `avatar-group` 的内容布局。Sidebar 设置 `sections: true` 后使用 `sidebar-header`、`sidebar-content` 和 `sidebar-footer` 组织内容，`sidebar-group` 提供带标题的分组。Resizable Panel 的 `defaultSize`、`minSize`、`maxSize` 以百分比声明。

Badge 支持六种样式，Skeleton 支持线条、圆形及块状占位，Progress 支持自定义量程和数值或百分比。Toast 和 Sonner 设置 `mode: "action"` 后通过 `click` 绑定 Runtime Action，用 `successKey` 和 `errorKey` 配置结果反馈。等待审批时不显示成功提示。

Command 支持内嵌和弹窗模式。使用 `mode: "dialog"`、`open` 和 `openChange` 绑定弹窗状态，选项使用 `groupKey` 分组，`shortcutKey` 显示快捷键提示。Carousel 使用零起点的 `startIndex` 和 `change`，`autoplayMs` 配置轮播间隔。自动轮播可暂停，并在鼠标悬停、键盘焦点进入或启用减少动态效果时停止。

Input 和 Input Group 支持输入类型，Textarea 支持行数。三者可声明必填、长度限制和错误状态，并通过 `describedBy` 关联 Field 的描述及错误。用同一 Runtime 数据绑定控件和 Field 的 `invalid` 状态，业务提交验证继续由现有表单和能力契约执行。Typography 支持一到六级标题、引用、行内代码和有序或无序列表。Tooltip 和 Hover Card 可设置方位与开关延迟，Collapsible 支持绑定展开状态并发出 `openChange`。

## 内嵌 AI 组件

项目应用部署基线 `com.horizon.projects@2.0.1` 提供 AI 风险列、详情分析面板和任务表单助手。现有安装通过应用安装升级流程选择 2.0.1，租户定制需要按现有流程审核合并。组件共用 AI 结果视图，结果只保留在当前页面，重新加载页面时清空。

| 配置位置 | Schema 结构 |
| --- | --- |
| `table-panel.props.columns[]` | `{ name, labelKey, display: "ai", analysis: AIAnalysisSpec }` |
| `ai-analysis-panel@1.0` | `data` 绑定主记录，`props.scenarios` 为 1–3 个 `{ id, labelKey, analysis: AIAnalysisSpec }` |
| `ai-result@1.0` | `data` 绑定 `AIResult`，`props.variant` 为 `compact` 或 `detail` |
| Runtime 表单 | `aiAssistant: { labelKey, action, inputFields, targetFields, prompt, createScope?, editScope? }` |

`AIAnalysisSpec` 包含 `action`、`prompt` 和 `input: [{ source, fields }]`。表格列使用一个单记录 Provider，详情场景使用主记录和可选的关联分页 Provider。数据引用从组件所在布局作用域解析。每个 Provider 通过 `route.id` 映射当前记录 ID。关联来源返回 `items` 和 `total`，固定读取首页最多 20 条。表单用对应模式的 scope Provider 校验记录权限。纯按需绑定可配置 `enabledWhen: { kind: literal, literal: "false" }`。

```yaml
# 项目列表的列
name: aiRisk
labelKey: projects:ai.risk
display: ai
analysis:
  action: analyze-risk-column
  input:
    - source: $_aiProject
      fields: [status, dueDate, taskOverdueCount]
  prompt: 根据项目状态、截止日期和逾期任务数判断风险，返回风险标签和一句话依据。
```

```yaml
id: project-ai-analysis
kind: ai-analysis-panel
version: "1.0"
data: $project
props:
  scenarios:
    - id: schedule-risk
      labelKey: projects:ai.scheduleRisk
      analysis:
        action: analyze-schedule
        input:
          - source: $project
            fields: [status, dueDate, taskOverdueCount]
        prompt: 说明项目是否存在延期风险并列出依据。
    - id: task-blockers
      labelKey: projects:ai.taskBlockers
      analysis:
        action: analyze-blockers
        input:
          - source: $project
            fields: [status]
          - source: $_tasks
            fields: [title, status, dueDate]
        prompt: 找出最多三项可能阻塞项目的任务并说明理由。
events:
  - { event: analyze-schedule, action: invoke, target: application.runtime.ai.analyze@1.0.0 }
  - { event: analyze-blockers, action: invoke, target: application.runtime.ai.analyze@1.0.0 }
```

```yaml
# 独立结果视图的配置。项目列表和详情已内嵌同一视图。
id: project-ai-result
kind: ai-result
version: "1.0"
data: $analysisResult
props:
  variant: detail
```

```yaml
aiAssistant:
  labelKey: projects:ai.draftTaskDescription
  action: suggest-task-description
  createScope: $_aiProject
  editScope: $_aiTask
  inputFields: [title, priority, dueDate]
  targetFields: [description]
  prompt: 根据任务标题、优先级和截止日期起草简短的描述。信息不足时返回空建议。
```

列和表单所在组件分别声明 `analyze-risk-column → application.runtime.ai.analyze@1.0.0` 和 `suggest-task-description → application.runtime.ai.suggest@1.0.0` 的 `invoke` 事件。AI 事件使用组件提供的调用载荷。分析返回 `{ status, summary, label?, evidence?, coverage? }`，列标签为 `high | medium | low | insufficient`。表单返回 `{ suggestions: [{ field, value }] }`，建议值可逐项采纳到表单草稿。

两个能力沿用 `assistant:use`、Runtime 发布版本校验及原 Provider/表单权限。服务端按 Schema 裁剪数据并执行一次无工具 Eino 模型调用。提示词、草稿和结果内容不写入能力审计。状态及记录标识可用于定位调用。模型生成不可用或结果校验失败时，原业务表单和页面数据仍可使用。

Button 和 Data Table 的契约描述了渲染器支持的属性。Button 的 `click` 可携带 `payload`，也可使用已有的 `action` 描述。Data Table 可从 `data` 或 `items` 读取行，通过 `columns` 配置字段和显示格式。声明 `rowEvent: "rowClick"` 并绑定同名事件即可接收行数据。设为 `serverPage: true` 时由外部 Provider 和分页控件驱动翻页。

Card 的内容可组合 `card-footer`。Alert、Empty、Item 可用 `sections: true` 组合 `alert-action`、`empty-media`、`item-media`、`item-actions` 和 `item-footer` 等区域。Tabs 可设置方向、自动或手动键盘激活和线条样式。Scroll Area 可配置滚动轴和保留滚动条。Alert Dialog 支持外部 `open` 绑定和 `confirm`、`cancel`、`openChange`，Dialog 关闭时仍发出 `close`，也可绑定 `openChange`。Chart 另支持环形图、雷达图和径向条形图。
