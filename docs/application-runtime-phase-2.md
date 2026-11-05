# Application Runtime：M1 通用化与 M2 动作执行

日期：2026-11-05。范围：落实确认计划的 M1、M2；M3 应用/插件在线生命周期和 M4 AI 深度接入不属于本轮交付。

## 现状与事实源

首轮文章 List/Detail/Form 示例已经运行，但不代表当时完成了通用平台。v2 将应用、页面、Provider、创建/编辑命令、字段、布局、表格、动作、表达式和扩展分开建模。Entity、Capability Definition 和 SQL 仍是各自事实源，未建立第二套实体、权限、Gateway 或 Workflow。

Go 模型生成 TS/Zod；能力 Output Schema 同源派生。表达式递归结构由结构校验与独立语义/类型检查共同验证，不能将反射生成的递归对象宽泛 Schema 当成完整授权。业务调用信封未改，没有新增 Goa 业务方法；会话契约仅增补可选授权缓存版本，见下文。`make generate` 还生成 `docs/generated/runtime/applications.json`，供跨语言测试消费；不手改生成物。

## 引擎交付边界

| 规范部分 | M1/M2 实现 | 后续边界 |
|---|---|---|
| 4.1 应用注册 | 共享装配、插件应用贡献、多应用只读解析/目录、当前身份裁剪 | 在线安装与租户启停待 M3 |
| 4.2 页面 | 明确 pageId、路由、List/Detail、独立表单与表单/详情弹窗、Hybrid/Custom、加载与错误状态 | 新企业视图按业务增加 |
| 4.3 组件 | 版本化可信注册、配置校验、表单/详情/单元格统一解析、局部错误边界 | 远程代码加载不开放 |
| 4.4 布局 | standard、section、响应式 grid、tabs；字段分区和可信插槽 | Shell 沿用原实现 |
| 4.5 数据 | Provider 固定能力/版本、输入/结果路径、主键、查询约束、服务端分页与关系引用 | 不拼 SQL，不自动 JOIN |
| 4.6 表单 | 创建/编辑分开、真实值类型、契约约束、默认值、隐藏/只读、引用选择和离开保护 | 子表/文件/复杂 JSON 待后续 |
| 4.7 表格 | 声明列、格式、筛选、排序、分页、选择、列显隐/宽度/固定配置 | 复杂报表与虚拟化待后续 |
| 4.8 动作 | 受限 DSL、导航、弹窗、能力、流程、业务批量与可信自定义 Adapter | agent 类型保留接口但未开放执行 |
| 4.9 状态 | 主页面与内嵌页面查询在 URL 中按 pageId 隔离、Query/RHF/局部状态、请求取消和身份隔离 | 草稿不持久化 |
| 4.10 权限 | 应用/页面/动作/命令及扩展裁剪；字段受能力 I/O 和系统/状态分类约束 | 不宣称有通用可配置字段 ABAC |
| 4.11 上下文 | 应用/页面/记录引用、选择集、语言时区及调用关联；后端使用可信 Principal | 浏览器上下文不授予权限 |
| 4.12 语义提供者 | 类型化快照、可用动作和已授权引用；不输出正文/草稿 | Harness/WebMCP/AG-UI/Memory 接入待 M4 |
| 4.13 扩展 | 可信编译期页面贡献与组件/动作/Provider/Context 接口，来源/版本/依赖校验 | 完整升级卸载生命周期待 M3 |

## 通用化与业务兼容

- Resolver 不再写死文章应用、页面、权限或实体；基于共享装配目录执行当前用户裁剪。项目经现有 Plugin Registrar 贡献应用，未另造 Plugin Manager。
- 启动装配拒绝重复 ID、未知引用、错误能力版本、非法输入映射/字段、错误组件版本、危险路由和循环依赖。生成时对真实 en/zh-CN 资源校验标签键；运行时不读取仓库源码文件。
- `1.0 → 2.0` 提供纯内存转换器；未知旧版本拒绝。旧版兼容渲染测试保留，但真实文章入口使用 v2。
- 创建与编辑分别从真实能力 Input 解析。项目创建保持 projects:admin，编辑保持 projects:write；不注入文章 revision，不要求普通更新绑定状态转移。
- 派生字段必须由能力 Output 声明且显式标为 derived，只读，不回填 Entity。原数据库模型与业务事务未重写。
- `/projects` 与项目基础信息、任务列表/表单采用 Runtime；成员和计划保留可信 Hybrid 插槽。任务详情保留描述和 AI 简报；单任务分配仍调用原 assign 能力，批量也不复制其业务逻辑。
- SQL 分页、计数分别保持文章作者/审核人与项目成员范围。旧 limit/空参数调用保持原默认数量。Runtime 不将截断的首批数据当成完整集合在浏览器分页。

## 动作与安全

表达式是明确节点：path、literal、object、array，以及 eq/in/and/or/not/exists。装配检查路径根、字段引用、深度、类型和必填映射；禁止原型路径、动态能力名、任意 JavaScript 和从 DOM/缓存取字段。表单、记录、路由、选择集和声明事件使用独立作用域。常量和表单值在调用前按能力 Schema 再校验。

UI 确认不代替 Gateway 风险审批。调用结果区分 succeeded、awaiting_approval、失败与 Workflow 受理；流程启动不表示领域工作完成。失败保留草稿；未改变载荷的普通动作重试复用幂等键。调用沿用 invocation/trace/correlation/run ID，敏感值不进入追踪或语义快照。

资源失效使用应用内注册的 Provider 标签，限定当前身份/应用缓存，不接受 Schema 中的任意 Query Predicate。读取传递 AbortSignal；身份/权限/部门/语言变化隔离定义和数据缓存。旧请求不能将数据投到新身份页面。

## 批量分配及恢复

新增 `project.task.assign.batch@1.0.0` 是 projects 模块的非事务组合能力：最多 100 个输入 ID，去重后顺序经 Gateway 调用原 assign，每项独立授权、事务和 Outbox。返回逐项状态与稳定错误码，父级完成事件只表示批量尝试结束，不表示全部成功或全局原子。

恢复规则与现有 Gateway 一致：同一外层幂等键重放已记录结果。连接中断后的恢复保留 operationId、使用新外层调用键，子调用键由可信租户/账号、operationId、任务与负责人确定，避免重复分配成功项；已知部分失败只重试失败子集，视为新操作。取消等待不回滚已经提交的任务分配。

没有开放新远程 MCP 或 WebMCP 工具；注册业务能力不自动授予渠道调用权。批量嵌套 Origin/深度仍由 Gateway 强制。

## 验证与后续

新增装配负例、独立创建/编辑契约、v1 升级、类型/原型路径、两应用交互、受控引用、审批等待、Workflow 受理、草稿保护、过期响应、批量部分失败/重试及真实 SQL 范围测试。验收命令与结果见下方记录。

M3 再建设应用/插件持久生命周期，届时才设计租户安装存储与迁移。M4 连接已有 ai/context/Harness，并单独治理页面动作发现、WebMCP 生命周期、AG-UI 与真实 Memory Adapter。未实现模块保持明确不可用，不使用知识库冒充 Memory Center。

会话契约还增加了可选的 authorizationVersion，正常服务端投影返回非敏感摘要，覆盖同权限代码下的范围变化与临时范围变化。Goa/OpenAPI/Orval 按源生成；旧客户端可忽略新字段。前端在发布新版本前清理私有缓存，版本不变的普通刷新不重挂草稿。此摘要不授予权限。

编译期 Action/Provider Adapter 仅规范化固定能力的输入，规范化后再次按能力 Schema 校验，再由统一 SDK 调 Gateway；不允许 Adapter 替代业务执行或伪造成功。组件展示和输入渲染共用版本化注册表。查询读取还校验能力 Output Schema，拒绝多余或形状不符的响应字段。

独立 Form 页面通过唯一的声明式 create/edit 表单动作指定目标与模式，复用 CommandForm、类型校验和离开保护；缺少或多于一个表单动作在装配时拒绝，没有对应命令权限的页面在投影时移除。

## 本地验收记录（2026-11-05）

- **文章示例完成**：原 List/Detail/Form 和审核发布能力保留，真实入口改由 v2 定义及声明动作驱动；旧 v1 兼容测试继续保留。
- **通用底座完成（M1 本轮范围）**：文章、项目共享 Runtime 核心；项目独立创建/编辑契约、无 revision 更新、派生列、关系 Provider、Hybrid 插槽及独立 Form 页面都有实现与回归覆盖。
- **Action Runtime 完成（M2 本轮范围）**：映射、确认、审批等待、流程受理、失败反馈、调用关联、资源失效和批量分配已落地；agent 执行与在线扩展生命周期仍按 M3/M4 边界关闭。

实际执行结果：

| 命令 | 结果与范围 |
|---|---|
| `make ci-fast` | 通过；规则、格式、lint、类型、测试、生成一致性、实体/流程校验、前后端构建通过；前端 37 文件 / 157 测试。最终日志 `/tmp/runtime-v2-ci-delivery.log`。 |
| `make test-security` | 通过；身份/Gateway/策略/MCP/传输专项，以及其依赖的真实 PostgreSQL `test-integration` 通过；含分页计数范围、批量部分失败、幂等事件、跨范围和成员撤销测试。日志 `/tmp/runtime-v2-security-final.log`。 |
| `pnpm --filter @baseline/web exec vitest run src/features/application-runtime/v2.test.tsx` | 18 项通过；包括新增独立 Form 页面提交；最终完整门禁也覆盖该文件。 |
| `git diff --check` | 通过。 |

生成影响：Goa/OpenAPI/Orval 同源生成可选会话授权版本；Runtime TS/Zod、跨语言目录样本与能力输出 Schema 由 Go 源生成；sqlc 从查询源重建。没有新增依赖或数据库迁移。en/zh-CN 文案及键集合检查通过；服务端记录范围和 Gateway 最终授权保留。

未执行与限制：**E2E 未执行（按当前指示暂停）**，没有浏览器验收；集成套件中的原生 MCP 客户端验收按其独立目标规则跳过，不能宣称该目标通过。未推送、未开 PR、未触发远程 CI。本地门禁仍报告项目计划表单/列表的两条非阻塞 React lint 警告，以及 Vite 大于 500 kB 的 chunk 提示；未关闭检查。布局滚动效果尚无浏览器旁证。M3/M4 不计入本轮完成声明。

实施沿用 React Router、TanStack Query/Table、React Hook Form + Zod、i18next 与既有 Capability Gateway。按任务路由应用 baseline-feature-development、baseline-capability-composition、baseline-web-development、baseline-database-change、baseline-entity-model、baseline-security、baseline-internationalization、baseline-quality-gates；会话契约补充按 goa-service-designer 执行。
