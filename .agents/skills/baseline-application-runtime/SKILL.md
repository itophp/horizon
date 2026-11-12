---
name: baseline-application-runtime
description: "选择并开发 Application Runtime 配置、业务贡献或可信组件，处理菜单、页面、Provider、Action、版本及生命周期。涉及业务界面先判断适配；不适用于无 UI 的纯后端任务，也不强制迁移专用页面。"
---

# Application Runtime 开发

分析应用或页面 Schema 时，必须先用 Horizon CLI。用 `./horizon-cli application` 或 `./horizon-cli page` 发现条目，批量读取完整定义时加 `--full`。用 `./horizon-cli application schema <app[@version]>` 或 `./horizon-cli page schema <app/page[@version]>` 读取具体协议、页面树、Provider 和动作绑定。分析动作引用时继续用 `./horizon-cli capability schema <name@version>`。二进制与工作树不一致时，从根目录用 `go run ./foundation/cmd/horizon-cli` 执行相同参数。

先读 [Runtime 开发规范](../../../docs/application-runtime-development.md)、[数据库版本实施记录](../../../docs/application-runtime-database.md)及目标目录 AGENTS。业务贡献在 [applications](../../../applications/AGENTS.md)，底座协议、授权投影和装配见 [foundation](../../../foundation/AGENTS.md)，渲染和可信组件见 [apps/web](../../../apps/web/AGENTS.md)。通过 baseline-feature-development 路由其余涉及层。

1. 检查已有能力、实体、应用贡献及授权环境中的当前已发布定义。明确选择配置、代码或混合路径及缺口；无数据库访问时区分离线定义与实际租户版本。
2. 已有能力和可信组件足够时，贡献或修改声明，不复制列表/详情/表单，不在通用引擎加入业务常量。缺少表结构、能力或组件实现时回到正常代码开发及部署。
3. 部署贡献以原模块手写源为准，Go 契约生成 TS/Zod 和示例；租户定制以数据库已发布不可变版本为准。禁止直接写版本表、手改生成产物或把定制自动反写部署基线。
4. Provider 和 Action 引用固定能力版本，执行经 Gateway。创建/编辑独立绑定，字段类型、授权投影、引用 Provider、输入映射及资源预算按实际契约核对；定义不能扩权。
5. 配置修改形成候选，绑定父版本与修订，校验后展示精确差异和只读预览。第四级版本由服务端分配；Agent 不自行发布，人工发布绑定版本/摘要/修订。部署升级不得覆盖定制，先做兼容检查与三方合并。
6. 生效验证覆盖安装/权限、旧表单和工具失效、资源释放及重启保留。

只改规范执行 check-rules；在线纯配置使用候选校验和授权生效验证；引擎/契约/组件代码在阶段验收执行 ci-fast，按 SQL/安全影响追加检查。生成、离线通过、候选保存、发布及线上生效分别报告，浏览器沿用根规则。

涉及页面生成、布局或视觉优化时，使用 [baseline-ui-design](../baseline-ui-design/SKILL.md) 完成设计说明与实景评审。工程 Skill 不代表在线设计模型已加载规则。
