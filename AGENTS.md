# 项目长期开发规则（Capability-Centric AI-Native）

本仓库是通用企业信息化底座。**业务能力（Capability）是系统唯一契约**：React 按钮、Agent Tool、Workflow 节点、外部 API 和事件触发都必须经过同一个 Capability Gateway 执行；能力成功后在同一事务写入 Outbox，事件经 Event Hub 驱动 AI Trigger，Agent 再调用能力形成闭环。

```
Business Capability → Business Event → AI Behavior → Business Capability
```

规则不限制开发阶段，也不替代用户最新要求；用户明确指令优先于历史文档。

## 开始任务与规则发现

1. 检查 Git 状态、目标模块与未提交变更，保留用户已有修改。
2. 改文件前读取根到目标文件父目录沿途所有 AGENTS.md；从根启动也要主动检查子目录规则。
3. 功能开发先用 [baseline-feature-development](.agents/skills/baseline-feature-development/SKILL.md)，按 [任务工具矩阵](docs/agent-tooling.md) 选择 Skill。
4. 使用锁文件版本；升级/新增依赖先核对官方 API、兼容性与许可证，记录于 docs/versions.md / docs/decisions。
5. 涉及 MCP 暴露面时使用 [baseline-mcp-development](.agents/skills/baseline-mcp-development/SKILL.md) 并确认读写与授权范围。

## 不可违反的架构边界

- **单一能力契约**：能力定义只注册一次，且只由业务模块自己提供：`internal/modules/*` 的 `Entries()`（settings、dictionaries、files、notifications、mcp、example）与 `internal/identity` 的身份管理能力。每项必须声明名称、版本、输入/输出 Schema、权限、风险、审批、幂等、成功事件、可审计字段，以及编排元数据 `Layer`（必填）/`Kind`/`Reads`/`Writes`/`Effect`/`RetrySafe`（其余项有保守推导默认值，见 [ADR 016](docs/decisions/016-capability-taxonomy-and-orchestration.md)）。禁止为 UI、Agent、Workflow 各写一套业务逻辑。
- **可独立执行的动作必须成为能力**：能被单独触发的动作（HTTP 接口、页面动作、定时任务、事件消费、Agent/MCP 动作）必须注册进 `Entries()`，不允许只以 service 方法、HTTP handler 或后台 goroutine 的形式存在。判据与命名见 [能力拆分与组合](docs/capability-composition.md)。
- **能力之间只经 Gateway 互调**：编排能力调用数据能力必须走 `gateway.Invoke`，禁止 `registry.Resolve` 后直接调 handler，也禁止绕过能力直接访问 `dbgen`/`pool`/领域服务。内层 `Origin` 由 Gateway 强制为 `OriginCapability`、深度 +1、写能力用确定性幂等键；`TxExecutor` 内禁止再调 Gateway，能力边界即事务边界。
- **实体模型是能力的对象视图**：业务对象在 `internal/modules/*/entity_<name>.go` 声明（字段分类、关系、状态转移），由 `internal/platform/metadata` 对照迁移校验；`make check-entities` 已进 `ci-fast`。新增表/列、新增能力、改状态列都必须同步，禁止手抄列清单或为通过校验删声明。跨实体写用 `Definition.Entities`，主实体必须在其中；改状态的能力必须绑定 `Transition`。详见 [实体模型](docs/entity-model.md)。
- **Gateway 是唯一运行时入口**：`internal/platform/capability/gateway` 负责身份、权限、Schema、风险、审批、幂等、执行、Outbox 与审计。`/api/capabilities/{name}/invocations` 是唯一业务写入口；业务模块不再暴露自己的 REST 端点。页面按钮只通过 `apps/web/src/lib/capabilities.ts` 调用，不得裸 fetch。
- **事务与事件**：新模块的原子业务写能力必须实现 `TxExecutor`，业务数据与 `cap_outbox` 在同一事务提交。同步组合使用非事务 Executor；编排受理可用仅写运行状态的基础设施事务，事务内不执行业务节点。已迁移模块仍由领域服务自持事务，事件在成功提交后写入 Outbox；这是已知的 best effort 边界，必须保持可追踪，不得声称原子。
- **Policy 确定性**：权限、风险等级、审批和深度限制是代码规则。人类点击可执行 high；Agent/Workflow/Event/API 的 high 必须人工确认；critical 对自动化关闭且人类调用需第二人审批。模型输出、Tool Schema、页面注册都不能替代服务端授权。
- **审批不放大权限**：审批通过后必须用 `PrincipalForTask` 重建申请人身份执行；决策是条件更新，重复/过期/跨租户一律冲突或不可见。
- **AI 任务独立生命周期**：`ai_runs` 记录 AI 任务，`cap_invocations` 记录业务调用，二者用 correlation id 关联；AI 失败不得把已成功的业务动作标记为失败。
- **防循环**：事件带 `correlationId`、`causationId`、`triggerDepth`；Trigger 必须去重（policy+event 唯一）并受 `maxDepth` 限制。Agent 再次调用能力仍走完整 Gateway。
- **Outbox 至少一次**：消费端按 `(tenant_id, event_id)` 去重；不得假设 exactly-once。
- **租户与范围**：租户、账号、部门来自可信 Principal；查询强制租户条件，详情强制账号条件；事件/审批只投影本人或具备审批权限者可见的数据。
- **敏感字段**：`cap_invocations.input` 只落 `Loggable` 白名单；审批 API 只返回脱敏投影；凭证、正文、个人信息不得进入日志、错误、导出或 trace。
- **生成代码禁区**：传输信封/会话/公开投影契约源为 `services/api/design`，业务动作契约源为模块 Definition；SQL 源为 `db/migrations` 与 `db/query`。禁止手改 `services/api/gen`、`internal/repository/dbgen`、`apps/web/src/api/generated`。修改源后 `make generate`、`make check-generated`。
- **前端**：页面按钮调用生成的 capability SDK；不得裸 fetch 或复制 DTO。TanStack Query 管远程状态，React Hook Form + Zod 管表单，TanStack Table 管常规表格。
- **国际化**：所有用户可见文案、状态、辅助名称走 i18next，`en`/`zh-CN` 键集合一致；不拼句、不渲染翻译 HTML。
- 禁止显式 TypeScript `any`、关闭检查、吞异常、删除断言或伪造测试结果。

## 目录

```text
services/api/internal/
├── platform/
│   ├── capability/{contracts,registry,gateway}
│   ├── policy/ approval/ audit/ outbox/ eventhub/
│   ├── metadata/       # 实体模型：迁移解析、能力派生、E1–E10 校验
├── identity/            # 会话、授权代理与身份管理能力
├── governance/          # 配置内核（策略、范围、版本链）
├── ai/
│   ├── harness/         # Eino ADK + AI 任务
│   ├── tools/           # 能力 → Eino Tool 适配
│   ├── triggers/        # 事件 → AI 策略、去重、深度
│   ├── context/ knowledge/ workflow/ orchestration/ evaluation/
├── modules/
│   ├── settings/ dictionaries/ files/ notifications/ mcp/   # 领域服务 + Entries()
│   └── example/         # 参考业务模块（查询/创建/归档 + 事件）
└── transport/           # Goa 传输装配与后台 starter
```

## 任务路由

| 变更 | 使用的 Skill | 规则入口 |
|---|---|---|
| 新增/修改业务能力、审批、审计 | baseline-feature-development + baseline-security；只有会话/系统契约变更才需要 goa-service-designer | 本文件、[ADR 015](docs/decisions/015-capability-centric-platform.md)、`services/api/internal/AGENTS.md` |
| 决定拆几个能力、能力互调、发现业务层绕过 Gateway | baseline-capability-composition | [能力拆分与组合](docs/capability-composition.md)、ADR 015 |
| 持久化流程、Graph、ADK、流程 Schema | baseline-capability-composition + baseline-security | [完整示例](docs/orchestration-example.md)、[Schema 路线](docs/agent-generation-plan.md) |
| 新增事件、Trigger 策略、去重/深度 | baseline-feature-development + baseline-security + baseline-database-change | ADR 015、`db/AGENTS.md` |
| 页面/表单/表格/SDK 消费 | baseline-web-development | `apps/web/AGENTS.md` |
| 会话/系统/公开投影契约 | goa-service-designer | `services/api/design/AGENTS.md` |
| 新增/修改表结构、实体声明、状态流转 | baseline-entity-model + baseline-database-change | [实体模型](docs/entity-model.md)、`services/api/internal/AGENTS.md`、`services/api/db/AGENTS.md` |
| 迁移/查询/事务/范围过滤 | baseline-database-change | `services/api/db/AGENTS.md` |
| 文案/语言/格式化 | baseline-internationalization | `apps/web/src/i18n/AGENTS.md` |
| 认证/授权/字段/跨租户 | baseline-security | `services/api/internal/AGENTS.md` |
| 测试/交付门禁 | baseline-quality-gates | `docs/testing.md` |
| MCP 暴露面 | baseline-mcp-development | `docs/mcp-service-plan.md` |

## 流程规格与生成

流程运行源为 Go Definition/Step；`docs/generated/workflows` 的标准 Schema 与示例从源生成，禁止手改。候选 JSON 先经 workflowcheck -file 校验，再审查注册；离线检查不执行、不授权。`make check-workflows` + `check-generated` 拦截结构错误与生成漂移；完整示例用 `make test-orchestration-example`。能力装配统一使用 app.CapabilityModules，运行时和离线工具不得复制不同目录。Schema、静态检查与行为测试互补，不能替代 Gateway 或 SQL 范围校验。

## 验证与交付

- **E2E 暂停（现行指令，优先于下述历史描述）**：默认不启动任何浏览器验收，包括 `make test-e2e`（无 `E2E_ARGS`）、`make test-e2e-parallel`、`make test-webmcp-e2e`、`make test-mcp-e2e`、`make test-mcp-native-e2e` 和默认 `make ci`。远程 CI 的 workflow 未改，仍是全量 E2E，因此推送/开 PR 等于触发全量，必须先取得授权。
- **只跑对应模块**：确需浏览器旁证时，按 `scripts/e2e-changed.mjs` 的映射（`features/<模块>` → 同名 spec；共享 `src/{api,i18n,app,lib,components}`、`design/`、`internal/*` → foundation）选出改动模块对应的 spec，用 `E2E_ARGS="<spec>"` 只跑这些文件；映射未命中（如纯迁移 SQL）就不跑浏览器，在汇报里记未覆盖。一次只跑单个模块文件，不顺手扩大到同栈其它 spec。
- **全量需授权**：完整浏览器集合（`make test-e2e` 不带 `E2E_ARGS`、`test-e2e-parallel`、默认 `make ci`、推送触发远程 CI）必须在当轮得到用户明确授权后才执行；没有授权就写"E2E 未执行（按当前指示暂停）"，不得含糊成"未发现风险"。
- 测试按变更批次集中执行：普通连续开发累计受影响模块；阶段验收/提交/PR 跑一次 `make ci-fast`；触及表结构加 `make test-integration`。
- 浏览器验收优先相关文件；只跑子集必须报告选中文件与未覆盖集合，不能宣称全量通过。
- 代码生成与浏览器验收串行，避免生成产物干扰 Vite。不要把缺少数据库/浏览器导致的跳过报告为通过。
- 同一根因连续三轮无解时停止盲改，报告证据与选项。
- 汇报：行为与文件变化、使用的框架/Skill、实际命令与结果、生成影响、i18n/安全影响、未执行检查与剩余风险。本地通过与远程 CI 分开报告。

## Code Review 重点

优先报告：绕过 Gateway 直接执行能力、实体声明与迁移漂移（加列不声明、状态转移绑不到能力、跨实体能力漏 `Entities`）、缺失租户/账号条件、审批后未重建申请人身份、事件与业务不同事务却声称原子、缺少去重/深度限制、敏感字段进入 trace/DTO、生成代码手改、源契约与 SDK 漂移、硬编码文案、缺失负向用例、被跳过却报告通过的检查。
