# UI 代码参考

这些文件供业务实现时阅读与提取，不挂路由、不注册菜单、不进入应用构建，也不维护演示业务数据。导入路径使用目标应用的 `@/` 别名；采用时放入实际 feature，出现真正跨业务复用再移到 shared。

| 文件 | 用途 | 接入条件 |
| --- | --- | --- |
| [page-layouts.tsx](page-layouts.tsx) | TemplatePage、TemplatePanel、SplitLayout、FocusLayout、MetricCard | 使用现有 Card/PageHeading；所有文案由调用方翻译后传入 |
| [record-editor.tsx](record-editor.tsx) | RHF/Zod + EditorDialog 的短表单 | 父层每次编辑按对象 ID/打开会话 key 重新挂载，避免 defaultValues 保留旧对象；onSave 由业务层调用能力 SDK |
| [workflow-node.tsx](workflow-node.tsx) | 图节点内的 Card/Badge/Button 内容 | 保持现有 X6 的节点注册、端口、选择与事件逻辑；只映射服务端允许展示的数据 |

代码参考没有持久化、鉴权或自动化执行实现。RecordEditor 的 catch 将失败显式呈现为通用错误并保留输入，不展示服务端内部错误；正式业务需要字段级错误时按稳定 code 映射。错误 ID 使用 useId 隔离。提交期间禁止关闭的示例策略需与真实取消/长任务语义匹配。

提取布局时的结构示例（`t`、查询结果和业务动作由具体 feature 提供）：

```tsx
<TemplatePage title={t("title")} description={t("description")} actions={actions}>
  <SplitLayout
    main={<TemplatePanel title={t("details")}>{details}</TemplatePanel>}
    aside={<TemplatePanel title={t("summary")}>{summary}</TemplatePanel>}
  />
</TemplatePage>
```

布局选择、分页与远程搜索不是这些组件自动提供的能力，继续遵循 [页面模板](../page-templates.md)、[组件指南](../ui-component-guidelines.md) 和 [交互模式](../ui-interaction-patterns.md)。

AI 工作台、工具步骤和流程节点的官方模式及本项目接入边界见 [AI 界面参考](../ui-ai-patterns.md)。不要创建同名但行为不兼容的仿制 AI Elements，也不要把不在本地库存中的官方组件当作可直接导入。
