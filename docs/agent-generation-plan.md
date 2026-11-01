# Agent 按架构生成代码：规则审查与 Schema 路线

日期：2026-11-01。范围：当前仓库的 AGENTS、项目 Skills、可执行门禁和编排定义。结论：应当推进，以“单一契约源 + 标准 Schema + 语义检查 + 行为测试”约束生成。自然语言规则有指导作用，不能保证未来每个 Agent 都自动遵守；Schema 也不能证明业务授权、SQL 范围或事务正确。

## 1. 当前审查结果

| 发现 | 影响 | 本轮处理 / 后续 |
|---|---|---|
| 根路由中能力组合一行的 Markdown 列数不一致 | 工具/Skill 路由难以准确读取 | 修正为三列，并增加编排路由 |
| Skill 把“第二个调用入口”写成必须再拆能力，而组合规范要求有独立子操作复用才拆 | 页面与 Agent 复用同一完整能力可能被机械拆分 | 同步 Skill 的拆分判据 |
| 根规则“新模块写能力全部 TxExecutor”缺少组合层例外 | 未来生成器可能在事务里组合调用 Gateway | 明确原子业务写用 TxExecutor，同步组合用 Executor；运行受理基础设施事务单列 |
| feature-development 未显式路由持久化编排 | 容易生成后台 goroutine 或第二套调度器 | 加入流程选型与规格检查入口 |
| agent-policy.json 缺少 orchestration 任务与相应门禁 | 静态规则清单无法发现编排验证入口 | 补任务、Schema/示例检查和专项测试入口 |
| 运行时与 entitycheck 过去分别装配能力目录，后者未包含知识库能力 | 离线验证可能使用另一份目录 | 用 app.CapabilityModules 共享实际装配源；新 CLI 也用它 |
| Workflow 的普通 JSON 解码会忽略未知字段 | Agent 生成错字段时可能静默丢配置 | 增加生成的严格结构规格与 DecodeDefinition；运行时也复用规格 |
| 存量远程 MCP 的 dictionary_options / notifications_* 仍直接调用领域服务，新的流程工具已走 Gateway | 规范与存量实现不完全一致，不能把所有旧代码都当生成模板 | 下一批将这些只读工具迁为能力适配，补原权限/账号范围与协议回归；本轮不声称全仓无绕过 |
| 知识库能力仍由 ai/knowledge 的 Entries 提供，与根规则仅列 modules/identity 的归属表述不一致 | 新生成器可能照抄出两种模块布局 | 优先将知识库能力契约收敛到领域模块，或正式澄清例外；不能复制第二份契约 |
| AST 检查只覆盖 ADK 的三个编排原语 | 不能阻止所有 Gateway 绕过或编排层业务 SQL | 保留现有检查；类型感知静态分析列入下一阶段 |
| entitycheck 对尚未声明的实体仅报告清单，不全部失败 | 仍可能存在模型漏声明；共享目录后暴露 knowledge_document 缺口 | 如实保留；先补真实实体声明，再引入明确的严格模式，不能加豁免掩盖问题 |
| 输入映射目前未静态验证所有源字段和类型兼容性 | 合法 JSON 仍可能在运行时映射失败 | 下一阶段增加有明确“未知”状态的字段/类型校验，不声称当前检查已覆盖 |

这次提供完整生命周期示例。测试还校正了流程内 ADK 工具的来源记录：工具调用归为 agent，普通流程步骤归为 workflow；两者均受自动化策略约束。

## 2. 单一事实源与导出方向

| 对象 | 当前唯一源 | 适合生成什么 | 不应让模型自动推断什么 |
|---|---|---|---|
| HTTP 信封/会话/公开投影 | Goa design | OpenAPI、传输代码、Orval SDK，保持现有生成链 | 业务权限和领域状态机 |
| 能力输入/输出 | 模块 Definition 的 contracts.Schema | 标准 JSON Schema、工具参数、校验和后续类型辅助 | Permission、Risk、Approval、Loggable |
| 编排定义 | orchestration.Definition/Step 等 Go 类型、共享限制常量、代码定义 | 已实现：结构 Schema、内置流程 JSON、候选 JSON 检查 | 业务顺序；Reads/Writes 只检查冲突，不能猜先后 |
| 实体物理结构 | SQL migrations/query | sqlc、字段/约束解析、实体模型核对 | 从手写实体反向覆盖数据库迁移 |
| 实体语义 | 模块 entity_*.go | 后续导出对象视图、关系/状态可视化与 Schema | 强原子事务、数据范围和状态转移的领域含义 |
| 事件 | 当前 SuccessEvents 声明 + 各能力事件构造 | 后续增加类型化事件载荷源，再生成事件 Schema/版本文档 | 仅凭事件名称推断载荷结构 |
| MCP | 能力契约 + 明确的工具允许集/输入映射 | 后续集中导出工具 Schema 与契约兼容检查 | 自动开放新工具、自动扩大旧 Token 集合 |
| 测试 | 业务不变量和风险 | 初始化成功/拒绝/重放/恢复测试骨架 | 从实现反射出“期望结果”，那会失去独立验证价值 |

当前新增的规范使用 [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)。复用锁定的 [google/jsonschema-go](https://github.com/google/jsonschema-go) v0.4.3，从 Go 类型生成字段；补充反射无法推导的分支与预算限制。它负责流程 DSL 结构，内嵌的能力输入 Schema 仍限制为 contracts.Schema 实际支持的子集，不假装支持任意 $ref/oneOf 作为能力输入。

## 3. 已落地的最小闭环

```text
Go 类型 + 共享限制常量
  → workflowspec 生成标准流程 Schema 与内置示例
  → Agent 按 Schema 生成候选 JSON
  → workflowcheck -file：结构校验 + 实际目录解析 + 依赖/冲突/预算校验
  → 审查后转换成 Go Definition 并注册
  → Eino Compile
  → Gateway 每次执行时重新鉴权、审批、幂等与审计
  → 真实数据库的独立行为测试
```

make generate-workflows 写 docs/generated/workflows；make check-generated 检查生成漂移；make check-workflows 检查生成 Schema、拒绝路径和离线校验。ci-fast 已加入 check-workflows。make test-orchestration-example 验证完整示例，test-security 包含数据库集合。

纯规格位于 ai/orchestration/spec，workflowspec 不依赖应用服务或生成的 Goa/sqlc，workflowcheck 才依赖共享装配目录，避免“必须先编译未生成的代码才能生成”的循环。

生成物不成为第二份运行时定义；编辑导出 JSON 不会更改已部署流程。离线 checker 不执行定义、不访问数据库、不授权运行。整个阶段不增加远程任意流程上传入口。

## 4. 建议实施顺序

### A：现在已做

统一规则语义、共享能力装配、流程 Schema 与示例生成、严格候选校验、完整 Eino 示例和自动验收。这些投入直接减少拼错字段、预算缺失、目录漂移与示例照搬错误。

### B：下一批优先做

1. 扩展映射校验：检查流程输入、前驱输出和目标能力字段；对开放对象/动态 Agent 结果明确报告“无法静态证明”，不能当作通过。
2. 类型化能力工厂：评估 Define[Input,Output](显式安全元数据, Handler)，从参数类型生成支持的 Schema；先迁移示例，unsupported 类型立即失败。与现有 Definition 保持适配，避免一次改全部模块。
3. 提供经过审查的模块脚手架：Entries、实体声明、权限/i18n 接入点、Workflow 和拒绝测试模板；生成器遇到现有文件拒绝覆盖，不自动写业务 SQL/权限策略。
4. 扩展结构静态检查：用 Go AST/类型信息追踪组合层对业务 repository 的引用、直接执行 Handler、TxExecutor 内 Gateway 调用。基础设施与原子模块要区分，不能用字符串 grep 冒充授权证明。
5. 补齐已发现的真实实体模型，收敛存量 MCP 只读绕过与知识库契约归属，再决定严格实体/架构门禁；保留显式且有理由的例外，不能把迁移债务当新代码规范。

### C：运行协议稳定后再做

- 类型化事件载荷及兼容性检查。
- 版本化流程定义的草稿/发布/审批/回退，才考虑 JSON 定义的受控入库与设计器。
- 编排图、权限影响和恢复边界的可视化预览；预览不替代服务端策略。
- 检查点保留/密钥轮换、外部副作用对账、长任务预算与运维操作。

## 5. 不建议做的事

不建立一个新的“大一统 JSON”同时生成数据库、业务逻辑、权限、审批和界面。它会复制 Goa、SQL 和实体模型的事实源，也无法从结构中推断领域风险。不要因为 Schema 合法就自动注册或执行 Agent 提供的流程；不要从 Kind=sql/command 自动开放任意 SQL/命令运行。

规则应指导 Agent 去正确的源文件，Schema 约束可声明结构，静态检查守住部分代码边界，行为测试验证真实效果；四者互补。当前已有的结构检查和本次完整示例是可执行基础，剩余路线不应写成已经实现或“保证 Agent 永不违规”。

## 6. 功能开发决策与验收补充

文章管理审查暴露了另一类缺口：能力与实体都能通过校验，但业务仍可能采用事件后普通函数和直接模型调用，没有进入用户要求的 Workflow/ADK。`Layer=orchestration`、`ai_runs` 及示例测试通过均不足以证明实际业务接入。

已补充的指导入口是 [开发步骤、条件提问与文章目标示例](agent-workflow.md)，并同步功能开发、能力组合、安全和质量 Skills。开发前记录角色/状态、能力/事务、运行方式及不用某引擎的理由、Schema 来源、人工页面、工具暴露和验证缺口；需求已明确时直接实施，不引入新的通用批准流程。

Schema 继续从唯一事实源生成：流程 Go spec → 标准流程 Schema，能力 Definition → 输入输出契约，Goa/SQL → 原有 SDK/sqlc。评审结论、原因和人工决定应有独立的明确输入输出契约；不凭自然语言 answer 直接改状态。不支持的规格先扩展源类型、生成器与运行时，不能把任意字段塞进 JSON 后宣称引擎支持。

当前尚未新增自动证明“每个业务入口使用了正确引擎”的静态门禁，也未在本次文档维护中实现能力 TypeScript 类型自动生成、业务人工任务通用恢复或文章 Workflow/ADK 改造。文章目标与当前实现的差异在工作流文档明确保留。后续行为测试应覆盖真实业务入口、原生引擎、工具允许集、结构化结果、人工等待恢复和作者可见原因；不能用检查文档包含某句话替代这些断言。
