# Application Runtime：仓库分析、适配设计与分阶段计划

日期：2026-11-05。状态：源码审查与建议设计，尚未实现 Runtime，也未对业务行为作运行验证。

依据：[实施规范](AI-Native%20Application%20Runtime%20开发实施规范.md)、[根规则](../AGENTS.md)、[架构](architecture.md)、[实体模型](entity-model.md)、[任务矩阵](agent-tooling.md)。已完整阅读实施规范第 0–19 节。用户本轮要求优先于规范中的 MES 示例：第一阶段采用真实文章模块，不新增 production_order 或 MES 业务。

本轮仅交付架构分析和计划，不修改业务代码、现有 Metadata Schema、SQL 或生成产物。后续实施以本方案为评审输入。

## 1. 结论

现有系统已经实现 Capability-Centric 运行底座、业务实体模型及静态内置插件机制。缺口集中在 Entity Metadata 到页面定义的适配、页面公共契约、通用渲染和受控查询参数，不应重建 Gateway、IAM、Entity Engine、插件管理器或文章 CRUD。

Metadata Engine 当前是代码声明、迁移校验与能力目录派生的对象视图，并非具备数据库发布版本和页面存储的低代码元数据服务。Runtime 应添加独立 Page/Application 定义，通过 Adapter 读取既有 Entity/Definition，不能把现有 Entity 改造成 UI Schema。

文章已经有手写列表、详情弹窗、新增/编辑表单和持久化审核发布流程。第一阶段应以同一组通用 List/Detail/Form Renderer 替换其页面重复结构，以原有能力完成真实读取、创建、编辑和刷新。保存需要的最小动作绑定必须在第一阶段交付，不能等到第二阶段才接 Gateway。

## 2. 已有架构与复用证据

以下“已有”表示找到源码，不表示本轮已运行通过。

| 范围 | 源码证据 | 现状与复用方式 |
|---|---|---|
| Entity/Field/Relation | `services/kernel/capability/contracts/entity.go`、`services/applications/*/entity_*.go` | Entity 有 Name/Domain/Table/Key，Attribute 有 Kind/Type/Required/Nullable/Enum/Sensitive 等，Relation 有 Target/Kind/Via；保留原模型 |
| Metadata Engine | `services/kernel/metadata/{schema,entity,validate,transitions}.go` | embed Goose 迁移解析物理结构；E1–E10 校验；`metadata.Build` 只从能力推导对象及能力/事件，不能单独当完整字段目录 |
| 完整实体目录 | `services/kernel/capability/registry/registry.go`、`services/runtime/app/entities.go` | `RegisterEntities` 加入声明，`Registry.Entities()` 合并字段与派生能力/事件；`EntitiesWithPlugins` 汇总插件实体 |
| Metadata 授权入口 | `services/kernel/capability/gateway/gateway.go` 的 `Entities` | 根据可发现能力过滤实体；这是 Go 内部接口，不等于已有完整公开 Entity HTTP DTO，也不代表字段和记录级授权 |
| Capability | `services/kernel/capability/{contracts,registry,gateway}`、`services/runtime/app/capabilities.go` | 版本、Schema、风险、幂等、审批、Outbox、审计以及统一装配；Runtime 继续使用原入口 |
| IAM/Policy | `services/system/identity`、`services/kernel/{policy,approval}` | Principal、权限和审批执行复用；不新建权限数据库或浏览器策略引擎 |
| 事件 | `services/kernel/{outbox,eventhub}`、`services/ai/triggers` | 已有事件链；文章写能力使用 ExecuteTx；不新增第二条页面事件总线代替业务 Outbox |
| 后端插件 | `services/kernel/plugin/plugin.go`、`services/runtime/app/plugins.go` | 已有 builtin Manifest、精确版本依赖校验、实体/能力/权限/事件注册、Start/Stop；内置 projects 接入，不是动态安装市场 |
| 前端插件 | `apps/web/src/app/plugins/{registry.ts,builtins.tsx}` | routes/menus/translations/permission labels 注册、重复及双语检查；已有 projects 插件 |
| AI/上下文 | `services/ai/{context,harness,tools,workflow,knowledge}` | Eino、页面 route/entity/entityId 与有预算的实体上下文已有；尚非规范中的完整 Page Semantic Context |
| Memory Center | README 明确标记规划 | knowledge 检索、调用记录和检查点不能冒充统一记忆中心；只预留 Adapter，不建立虚假服务依赖 |
| 前端底座 | `apps/web/package.json`、`src/app/router/router.tsx` | React 19、TS、React Router、shadcn/Tailwind、TanStack Query/Table、RHF/Zod、i18next；全部复用锁定版本 |
| SDK | `services/design/capability.go`、`services/runtime/transport/capability.go`、`apps/web/src/lib/capabilities.ts` | Goa/OpenAPI/Orval 生成传输层；业务 helper 和部分 DTO 仍手写，不能宣称已有完整能力类型生成 |
| 数据与装配 | `services/db/{migrations,query}`、`internal/repository/dbgen`、`runtime/{app,transport,server,config}` | PostgreSQL/Goose/sqlc/pgx；显式装配、小接口、统一错误和请求上下文；不另起 Runtime 服务进程 |

版本现状：Capability 有 Version，插件有 Manifest Version/APIVersion；Entity 本身无独立发布版本，Page Schema/Metadata 发布存储与升级机制未见现成实现。第一阶段采用随代码发布的只读定义，不新增元数据表与发布平台。

## 3. 文章真实闭环与缺口

事实来源：`services/applications/articles/module.go`、`entity_article.go`、`services/db/query/articles.sql`、`apps/web/src/features/articles/page.tsx`、`apps/web/src/lib/capabilities.ts`。

| 用例 | 已有能力 @1.0.0 | 约束/处理 |
|---|---|---|
| 列表 | `article.list` | articles:read；租户 + 作者本人或审核人；无输入，id DESC，最多 100 条；列表无 body |
| 详情 | `article.get` | articles:read；同样记录范围；返回 body；只读结果 TransientResult |
| 新建 | `article.create` | articles:write；title 1–200、body 1–20000；服务端拒绝全空白；Principal 注入租户/作者；草稿起始状态 |
| 编辑 | `article.update` | articles:write；作者本人、revision CAS、仅 draft/rejected；成功增加 revision、回到 draft |
| 提交发布流程 | `orchestration.workflow.run`，flow=`article-publish` | 保留既有 Workflow/ADK/人工风险确认路径；不能让 Schema 直接调用 article.publish 跳过审核 |
| 人工决定 | `article.decide` | articles:review；pending_review，revision 匹配且不可自审；不是普通字段更新 |

写能力已有必需幂等键、TxExecutor 和成功事件；Loggable 为空。读取 Schema 与 UI 类型并不等于开放全部实体字段。

现有页面用 URL q/status、本地过滤及 5 秒轮询；详情/编辑先读取正文再打开弹窗。没有服务端分页、排序、筛选与总数。Query key 当前主要为 `["articles"]`；Runtime 需显式按身份上下文隔离，继续沿用会话切换时清缓存的机制。详情读取使用 Promise 后打开，公共 Runtime 需补齐读取失败、取消和过期响应处理。

另有既存行为差异：页面对 rejected 展示重试提交，但 ArticleSubmit SQL 仅接受 draft/pending_ai；编辑 rejected 后保存会回到 draft。迁移页面时不得复制成“拒绝后可直接重提”的新承诺；第一阶段保留“编辑保存后重提”，如需改变重提语义须另行修改与验证业务契约。

## 4. 差距与优先级

| 目标 | 差距 | 第一阶段策略 |
|---|---|---|
| Application Registry | 无完整应用定义及租户安装授权生命周期 | 静态 application contribution + 必需权限/能力检查；复用现有导航和插件装配，不宣称支持安装/升级 |
| Metadata Adapter | Entity 与页面字段/动作/数据投影无稳定连接 | 从合并目录 + Definition I/O + 页面显式配置构建只读 Runtime Entity View |
| Page/Layout/Component | 没有统一 Page Schema、组件 Props 校验和解释器 | 新增版本化 Page 联合类型、standard/section 基础布局、可信组件白名单和 ErrorBoundary |
| List/Detail/Form | 文章页面结构仍手写 | 通用 Renderer 驱动文章页面；复用 DataTable、FormField、EditorDialog、ErrorBanner 等 |
| Query Runtime | article.list 有 100 条截断 | 原模块兼容扩展查询参数和 SQL；不做任意 Entity SQL 引擎，不在前端分页截断数据 |
| Action | 已有 helper，无通用声明绑定 | MVP 仅 navigation/dialog/create/update 与失效刷新；复杂 DSL、batch 留到下一阶段 |
| 权限 | 有能力/记录级控制，无通用字段策略投影 | 返回明确字段读写白名单；未知字段拒绝；页面显示不替代 SQL 和 Gateway |
| Context | 已有 AI Page，但缺应用/页面版本/选择集等 | 新建 UI Context 投影，服务端从 Principal 重建身份；提供未来语义 Adapter |
| 插件扩展 | 后端仅允许四种扩展，前端无 Runtime Component/Page contribution | 先可信编译期内置注册，不绕过当前 plugin 校验塞新扩展类型；完整扩展协议在后续阶段推进 |
| 版本/错误/缓存 | 无 Runtime Schema 版本和公共错误分类 | v1 精确支持、未知版本失败；稳定错误映射、身份/版本隔离键和失效机制 |

## 5. Metadata → Runtime 适配

### 5.1 事实源分工

1. Entity：对象语义、分类、关系、状态；不加入布局和 React 信息。
2. Capability Definition：具体动作输入输出、长度限制、权限、风险、幂等与事件。
3. 独立 Application/Page Definition：标签键、字段选择、布局、组件与动作绑定。
4. SQL：物理结构、参数化查询与记录范围；Runtime 不消费 Table/Column 来动态拼 SQL。
5. Goa DSL：公开 Runtime 投影和传输；生成 TS 客户端，不双写长期公共类型。

Adapter 内部读 `Registry.Entities()`，按当前 Principal 过滤输出；复用 `Gateway.Entities(ctx)` 的可见目录逻辑。原始 Table/Column/Relation.Via 不发送浏览器。没有字段声明的 name-only 实体拒绝自动生成页面，并给出诊断。

### 5.2 映射规则

| 原字段 | Runtime 含义 | 限制 |
|---|---|---|
| Name/Domain/Key | entityRef/domain/keyField | 保留 `article` 标识，不改名为 `articles.article` |
| Attribute.Name | fieldRef | camelCase 保持，不能由前端指定 SQL 列 |
| Type/Enum/Shape/Representation | 字段类型与允许控件 | unknown/bytes 默认不自动渲染；decimal、大数按字符串精度策略处理 |
| Kind=business | 可编辑候选 | 与绑定能力 Input 的 properties 取交集；并非自动可写 |
| Kind=system/state/reference | 系统只读/状态展示/受控引用 | revision 可作命令前置版本参数，不能成为用户编辑字段；状态只能经能力转移 |
| Required/Nullable/MaxLength | 语义约束提示 | 表单必填和具体长度仍以当前能力 Input 为准，不能把实体全部 Required 变成表单必填 |
| Sensitive | 安全分类 | 默认不入列表、日志、Trace、AI Context；授权详情/编辑可显式读 body，不等于全局隐藏正文 |
| Relations | 关系描述及受控 providerRef | 不根据 Via 自动 JOIN；逐条关联授权，未知 Provider 拒绝 |
| States/Transitions | 状态显示与动作条件提示 | 不生成状态写接口；服务端重新检查状态与 revision |
| Capabilities/Events | 可绑定动作候选 | 注册存在不代表当前用户或 MCP/Agent 有调用权限 |

文章 title/body 从 create/update Input 取得必填和长度；id/revision 从已授权记录映射；status/reviewCode 只读。body 仅在详情/编辑显式加载，禁止进入上下文持久化和错误信息。文章当前未声明 Relation，不伪造作者关联；关系机制用已有带 Relation 的实体做 Adapter 单测，业务关联选择器分期增加。

### 5.3 版本与契约来源

建议在 `services/kernel/applicationruntime` 定义纯 Go 契约/Adapter/校验器；Go 数据源随代码版本发布，生成版本化 Page JSON Schema 供离线验证。公开 Application/Page/Runtime Entity View 在 `services/design` 声明 Goa 投影，OpenAPI → Orval 生成前端模型。Page Schema 校验与 Goa 投影之间须有生成映射或等价性测试，禁止两套人工模型静默漂移。

新增 Runtime schemaVersion=`1.0`；Application version 独立于 Capability version。metadataVersion 用规范化实体/能力/页面定义的内容摘要，而非修改原 Entity 添加版本。启动验证依赖、字段、组件、能力版本引用，未知版本拒绝。对外错误不暴露原始元数据或 SQL。

## 6. MVP 契约与执行边界（建议，未实现）

- ApplicationDefinition：appId、version、metadataVersion、runtimeCompatibility、labelKey、pages、navigation、requiredCapabilities/Permissions；builtin 来源可显式表示，不给文章虚构已安装 pluginId。
- PageDefinition：schemaVersion、pageId、applicationId、pageType(list/detail/form/custom)、entityRef、titleKey、layout、dataBinding、fields/columns、actions、mode。form 区分 create/edit 和相应 capabilityRef；Metadata-driven/Hybrid/Custom 共用 Context 与错误边界。
- ComponentDefinition：name、version、可信 source、propsSchema、slots、renderer；重复、未知、非法 Props 拒绝。custom 只能指向已编译注册的 React 组件。
- RuntimeContext：application/page/entity/version、recordId、locale、timezone、selection；UI 可带会话提供的展示身份，后端永远重建 Principal。公开投影不含 sessionId、Token 或正文。
- QueryRequest：providerRef、select、受限 filter、sort、pagination；provider 绑定实体和既有能力，不接受 capabilityRef/SQL 任意覆盖。
- QueryResult：items/record、分页信息、可见字段和请求关联信息。列表 total 来自相同授权谓词的计数，不泄露不可见记录数量。
- ActionBinding：actionId、type、capabilityRef/version、白名单来源映射、labelKey、queryInvalidation；MVP 无任意表达式和递归属性解析。
- 错误：schema_invalid、schema_version_unsupported、component_not_registered、page_not_found、field_not_allowed 等 Runtime 内部分类映射到现有稳定错误信封；UI 使用翻译键，不透传后端内部错误文本。

### 6.1 Data Runtime 选型

前端 Data Adapter 直接通过 `lib/capabilities.ts` → 生成的 capabilityInvoke SDK 调用 `article.list/get`。后端受控查询留在 articles 模块；共享 Runtime 负责绑定校验与结果映射。此阶段不新增只做透传的 `runtime.query`，不新增 `/articles` REST，不由 Runtime 直连 dbgen。

为达成真实服务端查询闭环，兼容扩展 `article.list`：可选 keyword/status/page/pageSize/sort/select；默认参数保留旧调用可用语义，新增调用必须显式请求分页。pageSize 限制 1–100，sort 先仅 id/title/status/revision，必须稳定追加 id；select 仅列表已有五字段。where、count、page 使用同一可信租户/作者或审核人范围。对空参数旧行为、输出新增分页字段和 Schema 严格性做兼容测试；如无法保持兼容，再引入明确的新版本，不复用旧版本名改变语义。

参数化 SQL 通过 sqlc；排序使用已审查静态分支或固定 SQL 枚举映射，不能插值客户端字段。详情 select 仅允许 get 的授权投影。未知 filter/operator/select/sort 拒绝。关系展开首期不开放，不能收到请求后静默忽略。

### 6.2 页面和动作闭环

列表装载 application/page → 校验版本/组件/权限 → article.list → 点击详情调用 article.get → 通用 Detail Renderer → 编辑调用同一 get → RHF/Zod Form Renderer → create/update → Gateway → 原事务及 Outbox → 成功失效 list/detail 缓存 → 重新读取。保存编辑带原 revision；冲突提示刷新，保留用户草稿但不自动覆盖。

同一次保存重试复用幂等键，下一次修改生成新键；pending 禁止重复提交。未知结果及 awaiting_approval 不能当作已保存。保留原文章审核/发布入口，可通过可信 Hybrid action adapter 接原 helper，完整通用 Action Runtime 下一阶段再做。只替换页面实现，不复制文章 Workflow、ADK 或人工审核逻辑。

Runtime Query key 至少包含 tenant/account/department/授权上下文版本、application/page/metadataVersion、locale、查询参数或 recordId。身份切换取消请求并清私有缓存；旧请求不得将正文装入新身份页面。Schema 与字段白名单缓存也须随权限变化失效。

## 7. 目录与依赖建议

| 位置 | 职责 |
|---|---|
| `services/kernel/applicationruntime/`（新增） | 纯契约、版本/引用校验、Entity Adapter；依赖 contracts，不依赖 React/SQL |
| `services/system/applicationruntime/`（新增） | application/page resolve 只读能力、授权投影；模块自己 Entries；不承载文章 CRUD |
| `services/applications/articles/`（扩展） | 文章页面贡献与数据绑定、list 查询兼容扩展；领域规则仍在本模块 |
| `services/runtime/app/`（扩展） | 共享目录与能力装配；运行时及离线校验共用 |
| `services/design/`（扩展） | 公开 Runtime 读模型与稳定传输；生成代码不手改 |
| `apps/web/src/features/application-runtime/`（新增） | Page/Layout/List/Detail/Form、组件注册、Context、data/action adapter、错误边界 |
| `apps/web/src/features/articles/`（收敛） | 最小应用入口和必要 Hybrid 业务动作，不保留一份重复通用 Renderer |
| `apps/web/src/app/plugins/`、router（扩展） | 复用前端贡献装配、导航和路径；旧 /articles 保留 |

Application/Page resolve 是可独立执行的查询，建议新增 `application.runtime.resolve` 与 `application.runtime.page.resolve`（最终命名实施前按目录确认）：只读、低风险、无业务成功事件/写幂等要求、TransientResult，输入仅 ID/版本，授权来自文章所需权限与可发现能力交集；返回裁剪定义。它们是基础设施目录投影，不伪造 Entity/Transition，也不新建业务表。Kernel 不主动调用业务模块，模块贡献由 app 装配注入。

不引入新的依赖、Redis 必选缓存、微服务或动态远程脚本。新增依赖如确有必要，实施时再按版本规则评估。

## 8. 分阶段实施及验收

### 阶段 0：本轮分析交付

交付本分析及 [ADR 017](decisions/017-application-runtime-metadata-adapter.md)。完成现状、差距、映射、能力复用和实施顺序；无业务代码。设计评审后进入阶段 1。

### 阶段 1：文章 List/Detail/Form 运行闭环

按依赖分为四个增量，每个增量保持项目可编译：

1. **契约与 Adapter**：建立只读 Application/Page 定义、Entity Adapter、版本/字段/组件引用校验和公开读投影；文章贡献三类页面。验证敏感字段分类、未知引用与版本拒绝。
2. **受控读取**：原文章 list 能力和 sqlc 查询兼容扩展分页/搜索/排序/投影、同范围 total；完成 Application/Page resolve 和 Principal 授权。验证超过 100 条记录的后续页能读到且无跨租户/作者泄露。
3. **通用页面闭环**：组件白名单、standard/section、List/Detail/Form、Runtime Context、基础导航/弹窗、最小保存动作；保留 /articles 和原发布流程。title/body 新建→详情→编辑→列表刷新，revision 冲突、取消、加载失败均可观察。
4. **阶段验证**：定向单元/RTL、真实 PostgreSQL 集成、生成检查及 ci-fast；同步中英文文案、运行说明和行为验证记录。没有通过的门禁不得称阶段完成。

阶段 1 必须覆盖：Schema 驱动三类页面，无独立手写 ArticleForm/Detail 结构；服务端分页筛选排序；字段必填/只读/隐藏；未知组件、非法版本/Props、请求失败有诊断；直接请求越权拒绝；正文不进日志/Trace/AI；身份切换缓存隔离；保存幂等与并发冲突；现有审核发布流程无意外扩权。基础状态保留/销毁、URL 查询和未保存提示需要明确策略。

首期仅标题/正文控件、状态只读、固定安全排序/操作符；远程关系选择、子表、完整字段策略配置、批量操作、列固定等完整 Table Runtime 功能不属于这一闭环的完成声明，后续分期补齐。

### 阶段 2：通用 Action Runtime

在最小保存绑定上增加受限 Input Mapping DSL、确认/审批结果显示、workflow/agent action、查询依赖失效与有上限批量动作。复用 article-publish 验证受理、等待审批、恢复、失败和 runId 关联；不得用普通确认弹窗替代 Gateway 审批。

### 阶段 3：插件与应用生命周期

扩展已有 Manifest/Registrar 和前端贡献契约以支持 page/component/provider；兼容性、启停清理、历史页面引用、错误隔离和升级测试。再按需求补租户应用启用/授权存储；静态 MVP 不冒充动态安装。无需另造 Plugin Manager。

### 阶段 4：AI 页面语义

Runtime Context → 已有 ai/context 与 WebMCP 生命周期 Adapter；Page Semantic Context 按授权字段和预算裁剪，记录来源版本；接入已有 Harness。Memory Center 未交付前显式返回不可用/空检索语义，不能把知识库包装成已实现记忆治理。AG-UI 按实际缺口独立适配。

### 阶段 5：扩展业务视图

先迁移第二个真实模块验证复用，再按需求增加关系选择器、字段策略、Tree/Dashboard/Calendar/Gantt 等；复杂业务采用 Hybrid/Custom。MES 是后续真实业务接入，不作为文章第一阶段的假数据验收替代品。

## 9. 验证计划与本轮结果

后续代码阶段：Adapter/Schema/Registry 单测；文章 PostgreSQL 允许/拒绝、范围、分页总数、稳定排序、CAS、幂等与 Outbox 集成；RTL 验证页面跳转、保存、错误及身份切换；公开投影与生成契约一致性检查。复用 `services/tests/articles_test.go`、`articles_mcp_test.go` 和模块现有测试，按缺口补用例。

每阶段按实际变更集中执行 lint/typecheck/test/build；契约/SQL 源变更执行 `make generate` 与 `make check-generated`；阶段验收执行 `make ci-fast`，SQL/范围变化加 `make test-integration`，授权变化加安全专项。无表结构变更不造迁移；运行生成和浏览器验收必须串行。

本轮是文档任务，仅运行 `make check-rules`，实际结果见本轮交付回复。未运行业务测试、Go/前端构建、数据库集成、真实模型、远程 CI。E2E 未执行（按当前指示暂停）。未推送、未开 PR。

残余不确定性：本轮未证明现有代码运行正常；公开投影设计和 list 参数兼容性仍须实现时以生成产物及真实测试核对。字段权限采用文章固定白名单，不意味着平台已有通用 ABAC 字段策略。后续若要求租户自助安装、元数据在线编辑或动态插件装载，将扩展阶段 3 的存储/授权设计，不能隐含塞进阶段 1。
