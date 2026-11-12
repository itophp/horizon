# 业务页面前端评审库

本轮范围由用户最新要求确定：先实现信息化常用页面，确认布局、组件和交互，再对接既有 ApplicationRuntime。没有修改 Runtime 核心、Page Schema、业务契约、生成代码、数据库或后端。

## 页面与访问

入口 `/runtime-showcase.html`，默认工作台。使用 `?pattern=` 直接访问：

| pattern | 页面 | 布局与行为 |
|---|---|---|
| dashboard | 工作台 | 指标带、趋势/侧待办、团队动态与快捷详情 |
| listing | 数据列表 | 标准纵向、左组织树、左类型菜单三种变体；状态 Tabs、查询、排序、选择、批量、分页、新增 |
| recordDetail | 详情 | 摘要、进度、属性、动态、编辑 Dialog |
| entryForm | 表单 | 单页新增、分组编辑、分步向导；动态交付项、就地校验、失败保留 |
| preferences | 设置 | 左分类右表单；Switch、Slider、RadioGroup、Accordion、保存反馈 |
| reviewDesk | 审批 | 左队列右详情；影响说明、进度、审批意见校验、通过/驳回与禁止重复处理 |
| reporting | 分析 | 指标带、时间范围、折线/柱状切换、状态分布和对象明细 |
| assets | 资源 | 类型导航、搜索、文件列表、本地文件加入、Sheet 预览 |

列表 `layout=plainList|treeList|categoryList`；`group/status/q/p/sort` 保留在 URL。每类页面的“设计说明” Sheet 分别说明布局选择、组件组合与交互规则。详细规划见 [页面模式](runtime-page-patterns.md)。原 13 引擎清单仍见 application-runtime-phase-3-4.md；不再以引擎数量作为页面完成指标。52 场景计划已撤回。

## 组件与前端边界

沿用现有数据统计、账号、组织与权限管理页的 admin.css 和 PageHeading。实际使用本地 shadcn/Base UI 的 Card、Tabs、Input、Textarea、Select、Table、Checkbox、Dialog、DropdownMenu、Badge、Skeleton，以及本轮从官方 base-nova Registry 新增的 Accordion、Alert、Avatar、Breadcrumb、Popover、Progress、RadioGroup、Sheet、Slider、Switch。每项新增原语均有页面消费；没有新增 npm 依赖，没有覆盖已有 Button。折线/柱状图沿用现有统计页的 SVG/CSS 技术路线，未声称已接入 shadcn Chart。

前端状态由 React 管理；表单使用 RHF/Zod；列表以 TanStack Table 9 组织记录。文件只读取名称/大小，不上传。审批与角色信息都是 Mock，不代表真实授权。记录修改仅在浏览器当前会话保留。界面语言使用现有 i18next/changeLanguage，双语资源同键。

后续契约对接前，先确认布局变体、所需组件和联动，再映射现有 Runtime 的 Provider/Action/布局；缺失支持另行列出，不从本前端实现推断 Runtime 已具备该能力。

## 开发与验收

开发：`pnpm --filter @baseline/web dev --port 5188`。
独立生产构建：`pnpm --filter @baseline/web exec vite build --config vite.showcase.config.ts`，默认输出 `apps/web/dist/showcase`。
定向浏览器脚本：`apps/web/scripts/check-page-patterns.mjs`，读取端口 5190 的独立产物，使用本机 Chrome；截图与报告在 `/tmp/runtime-page-patterns-review`。

原来的 78 项引擎入口验收属于旧版记录，不适用于当前8类页面。当前结果见 runtime-page-patterns.md 的验收记录；不将本地 Mock 测试称为生产 Runtime、完整业务 E2E 或远程 CI 通过。
