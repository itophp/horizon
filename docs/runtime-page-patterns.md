# 前端页面模式评审（契约对接前）

本轮只实现前端 Mock，不改 Runtime、Schema、Gateway、后端与生成契约。先通过可操作页面确认布局、组件、交互，再据此对接现有 Runtime。

| 页面模式 | 布局变体 | 组件与交互重点 |
|---|---|---|
| 工作台 | 指标带 + 主趋势/侧待办 + 快捷入口 | 指标卡、趋势、状态、快捷导航 |
| 数据列表 | 纵向；左组织树+右列表；左类型菜单+右列表 | 状态 Tabs → 筛选 → Table → 分页；作用域切换清理选择与页码 |
| 详情 | 摘要与主操作 + 同对象内容 Tabs | 描述分组、进度、时间线、折叠技术信息、编辑 Dialog |
| 表单 | 单页新增；分组编辑；分步向导 | RHF/Zod、动态明细、就地校验、失败保留与重试 |
| 设置 | 左设置导航 + 右分组表单 | Switch、RadioGroup、Slider、Accordion、保存反馈 |
| 审批 | 左待办队列 + 右详情 + 处理区 | 申请摘要、影响说明、审批 Dialog、必填意见、队列更新 |
| 分析 | 指标带 + 趋势/分布 + 精确数值 | 时间范围切换、趋势与分布、表格口径 |
| 资源 | 左资源类型 + 右列表 + Sheet 预览 | 本地文件选择、搜索、分类、预览与焦点返回 |

使用者是业务操作员与管理员；信息优先级为任务/对象→主操作→查询/摘要→结果→辅助信息。复用现有 administration 样式与 PageHeading，Base Nova/shadcn 组件承载交互，不以普通 div 替代控件。桌面/平板/手机、Light/Dark、en/zh-CN 和边界状态分别验收。

原来的 13 引擎清单保留为架构背景，不作为本轮页面导航；52 场景草案暂停，不以数量作为完成标准。

## 本地验收（2026-11-06）

- 定向 ESLint、TypeScript、i18n 键一致性、独立 Vite 构建通过；构建保留 chunk 大小警告。
- `showcase.test.tsx` 3 项通过：表单失败保留/重试、搜索/批量状态、组织范围与状态组合/清除恢复。
- Chrome 对8类页面进行 1440/768/390 × Light/Dark 布局检查，截图在 `/tmp/runtime-page-patterns-review`；桌面工作台、组织树列表、移动暗色列表已复看。
- 浏览器定向交互通过：三种列表布局、组织/状态组合、搜索、审批意见校验与防重复、设置开关与保存、资源 Sheet、单页表单校验与保存、语言切换。修复联动中使用旧 URL 参数覆盖当前组织选择的问题；多个查询控件更新时合并当前完整筛选状态。
- `make ci-fast` 停在用户原有 `design-preview/main.tsx` 的 5 个 lint 错误（未使用导入/硬编码文案），日志 `/tmp/page-patterns-ci.log`；该文件未修改。`make check-rules` 与 `git diff --check` 通过。
- 未运行真实业务全量 E2E、数据库测试、远程 CI，未发布、未推送。Mock 页面通过不代表 Runtime 契约、生产权限或后端能力已对接。

当前组件范围：本地 shadcn Card/Tabs/Input/Textarea/Select/Table/Checkbox/Dialog/DropdownMenu/Badge/Skeleton，加新增 Accordion/Alert/Avatar/Breadcrumb/Popover/Progress/RadioGroup/Sheet/Slider/Switch。新增源码来自官方 base-nova Registry，保留已有 Button，不新增 npm 依赖。统计图沿用现有统计页的 SVG/CSS 路径；未安装 Chart、Drawer、Command 等尚未使用组件。

快速交互补充：本地 Mock 入口使用锁定 React Router 8.5.1 的 `BrowserRouter.useTransitions=false`，避免即时筛选状态被默认延迟导航中的旧参数覆盖。此设置仅作用于独立展示入口，没有修改正式应用 Router。浏览器布局与交互报告分开存储；仅交互重跑不会覆盖已通过的48项布局报告。
