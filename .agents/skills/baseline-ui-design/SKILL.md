---
name: baseline-ui-design
description: "为 Runtime 或 React 业务页面建立信息层级、布局与组件组合，基于实际渲染评审和修正视觉质量。"
---

# 业务页面设计闭环

读取 [生成标准](../../../docs/runtime-ui-generation.md)，沿用 [设计系统](../../../docs/design-system.md)。

1. 实现前按标准六项格式写设计说明，从真实任务排列字段；查看已确认的同类页并记录可复用组合。
2. 按 [页面模板](../../../docs/page-templates.md) 选择任务模式，按 [组件指南](../../../docs/ui-component-guidelines.md) 核对库存。声明式路径检查注册组件是否支持目标，不虚构布局、组件或 CSS。
3. 按标准八项约束实现；不为视觉填充新增业务动作、统计或权限。项目 Token 优先于外部审美建议。
4. 所有按钮必须配与动作语义匹配的图标，优先复用项目现有图标库。文字按钮保留动作文字，纯图标按钮提供可访问名称。加载时可用加载图标替换动作图标。评审时逐一检查。
5. 依评审表检查真实渲染，保留同视口前后证据，记录问题、修正和未验证项。配置 validate/diff 不代表视觉验收，不自动发布。

本 Skill 负责设计与视觉评审；baseline-application-runtime 负责路径与版本；baseline-web-development 负责 React/SDK；baseline-quality-gates 负责工程验证。浏览器遵守当前授权；不可用时明确未验证。
