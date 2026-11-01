<p align="center">
  <img src="docs/assets/horizon-cover-simple.png" alt="Horizon：现代应用，连接人与 Agent" width="100%" />
</p>

<h1 align="center">Horizon</h1>
<p align="center"><strong>以业务能力为契约的 AI 原生企业应用框架</strong></p>
<p align="center">Capability · Gateway · Eino Workflow / Graph / ADK · MCP · Skills</p>

Horizon 将页面、Agent、流程和外部客户端接入同一套业务能力。业务模块定义一次契约，Capability Gateway 统一执行身份、权限、Schema、幂等、风险审批和审计；业务事件再驱动 AI，形成可追踪的工作闭环。

```text
Business Capability → Business Event → AI Behavior → Business Capability
```

采用单仓库、单 Go 服务，优先复用当前能力，不因接入 Agent 而复制业务逻辑或提前拆成微服务。前端使用 React、TypeScript 与 shadcn/ui，后端使用 Go、Goa、PostgreSQL 与 Eino。

## 当前架构

![Horizon 当前能力中心架构与事件闭环](docs/assets/horizon-capability-architecture.png)

[分层概念与可编辑 Mermaid 图](docs/architecture.md) · [SVG 原图](docs/assets/horizon-capability-architecture.svg) · [编排协议](docs/orchestration-plan.md)

图中 Registry 为 Gateway 提供契约，执行层按能力选择原子动作、同步组合或持久化编排；它们不是每次请求都必须依次经过的流水线。Workflow 节点、Graph 子图及 ADK 业务工具再次调用 Gateway。

| 层 / 概念 | 职责与边界 |
|---|---|
| **交互与接入** | React 页面、内置助手、远程 MCP、页面 WebMCP、事件与流程触发；只适配输入输出，不复制业务规则。 |
| **Capability 契约与 Registry** | 模块 `Entries()` 定义名称、版本、输入输出 Schema、权限、风险、审批、幂等、事件和编排元数据；Registry 校验并提供目录。 |
| **Capability Gateway** | 唯一业务执行入口，统一身份、操作授权、输入校验、幂等、确定性策略、审批、执行、Outbox 与审计。能力之间也通过它互调。 |
| **业务能力与领域实现** | 原子能力负责状态、数据范围和业务 SQL；同步组合只调用其他能力。能力边界也是事务边界，不能把多次调用描述成一个原子事务。 |
| **Eino 编排与 AI** | Workflow 管依赖、持久化和暂停恢复；Graph 管有界循环/只读重试；ADK 管受限工具决策。简单动作无需 Workflow，一次模型分类不等于 ADK。 |
| **存储与执行记录** | PostgreSQL 保存业务数据、调用/审批/事件、AI 运行和加密流程检查点；知识检索使用 Redis 向量索引。AI 任务与业务调用生命周期独立。 |
| **事件闭环** | 事务 Outbox → Relay → Event Hub → Trigger 去重与深度限制 → AI/能力；采用至少一次投递，消费者必须去重。 |
| **工程契约与门禁** | AGENTS / Skills 指导开发；Goa、SQL、能力 Definition、实体声明和流程 spec 各自维护唯一事实源，生成和行为测试检查漂移与边界。 |

## 能力如何组合

| 需求 | 实现方式 |
|---|---|
| 一次查询或原子业务写入 | 原子 Capability；新模块写入使用 `TxExecutor`，业务数据与 Outbox 同事务。 |
| 无跨请求等待的多能力聚合 | Go 编排 Executor，经 Gateway 复用已有能力。 |
| 跨请求等待、操作审批、重启恢复 | 版本化 Eino Workflow，保存检查点和节点回执。 |
| 有界轮询、只读瞬态重试 | Eino Graph 子图，显式限制步数与次数。 |
| 模型选择工具和多步判断 | Eino ADK Runner，业务工具经 Gateway，按权限和允许集合约束。 |

`Layer` 区分原子与编排，`Kind` 描述执行形态；`Reads/Writes` 描述数据依赖，`Effect` 描述副作用，`RetrySafe` 描述安全重试。依赖由显式映射与 `DependsOn` 决定，不能仅凭读写元数据猜业务顺序。

编排入口为 `orchestration.workflow.{list,get,run,cancel}`。完整参考 `example-lifecycle@1.0.0` 串起创建、Graph 就绪检查、ADK 评审与人工确认归档。每一步重新授权，取消不回滚已提交动作。详见 [完整示例](docs/orchestration-example.md)。

## 人与 Agent 使用同一套业务能力

- **页面按钮**：通过 `apps/web/src/lib/capabilities.ts` 调用统一 capability SDK。
- **内置助手**：Eino ADK 工具适配能力目录，服务端 Gateway 始终执行授权与策略。
- **远程 MCP**：账号级 Token、固定工具允许集合、当前权限交集；已提供文章草稿/投稿、流程查询/启动/取消等工具，具体范围见 [操作指南](docs/mcp-operations.md)。
- **页面 WebMCP**：显式配对、受控页面上下文与只读工具，随路由和身份变化撤销；本机桥接与远程账号 MCP 分离。
- **事件 / 后台任务**：可信身份重建、关联链、去重与深度限制；后续业务动作仍回到 Gateway。

工具目录或模型输出都不授予权限。自动化的 high 风险动作需人工确认；critical 不允许自动化执行。业务人工审核与 Gateway 风险确认有不同语义，必须明确审核人、对象版本和原因，不能用一个确认按钮替代完整审核业务。

## 当前功能与实现边界

已提供身份/组织/部门/角色/临时权限，系统设置与版本治理、字典、文件、通知、AI 工作台与知识库、能力与审批页面、远程 MCP/页面 WebMCP，以及 Eino 持久化编排参考。页面支持中英文与深浅主题。

业务能力中心、实体校验、流程 Schema 生成、审批恢复和参考链路已落地，但不表示每个业务模块都已迁入 Workflow/ADK：

- **文章管理**当前仍是投稿事件 → 普通函数评审 → 直接模型分类 → 状态写入；文章 Workflow/ADK、独立人工任务页面和完整驳回原因链尚需改造。审查还发现审核员读取范围、通用 Agent 暴露、无有效词条处理和详情错误映射问题，尚未在本次文档更新中修复。
- 既有领域服务自持事务的模块，业务提交后写 Outbox 仍存在 best-effort 窗口；只有实际采用同一事务的路径才可声明原子。
- 当前支持 Gateway 风险审批的中断恢复，不代表任意业务人工任务、外部信号和条件分支都已支持；ADK answer 文本也不等于结构化业务结论。
- 流程 Go 定义随代码发布；导出的 JSON 用于查看/检查，不提供任意流程上传或在线发布入口。知识库契约归属、实体声明与部分存量适配仍有待收敛项，见 [Schema 路线](docs/agent-generation-plan.md)。

## 唯一契约源与生成方向

| 对象 | 源 | 生成 / 校验 |
|---|---|---|
| 会话、系统与统一传输信封 | `services/api/design` | Goa → OpenAPI → Orval SDK |
| 业务能力 | 模块 `Definition` / `Entries()` | Gateway Schema 校验、目录与工具适配；工具暴露须显式选择 |
| 物理数据库与查询 | `services/api/db/migrations`、`db/query` | Goose、sqlc → `dbgen` |
| 业务实体语义 | 模块 `entity_*.go` | 对照迁移与能力校验字段、关系及状态转移 |
| 持久化流程 | `ai/orchestration/spec` 的 Go 定义 | 标准 JSON Schema、示例 JSON、结构/目录/预算检查 |

不手改 `services/api/gen`、`internal/repository/dbgen`、`apps/web/src/api/generated` 或生成的流程 JSON。不声称当前所有业务类型和 MCP 适配器都已自动生成。

## 本地启动与测试

环境版本以 [版本记录](docs/versions.md)、锁文件及 `services/api/go.mod` 为准。准备 Node / pnpm、项目 Go 工具链及 Docker 后：

```sh
cp .env.example .env
make install
make db-up
make migrate
```

在本地环境设置 `BOOTSTRAP_PASSWORD` 后运行 `make bootstrap` 创建管理员，密码不应进入 Git。两个终端分别运行：

```sh
make dev-api
```

```sh
make dev
```

默认访问 `http://127.0.0.1:5173`，API 位于 `127.0.0.1:8080`。模型与治理秘密通过既有设置管理；持久化流程检查点和敏感配置需要有效 `SETTINGS_ENCRYPTION_KEY`，不要提交密钥。旧角色需显式补充新能力权限，不能假定迁移自动扩权。

进入「业务能力 → 流程示例测试」（`/capabilities`）：基础测试无需模型；完整测试需启用并配置 Agent 模型；归档需要人工确认。每次点击会创建真实示例记录。

```sh
make ci-fast
make test-security
# 真实 PostgreSQL + Eino，使用可控测试模型，无需浏览器或模型密钥
make test-orchestration-example
```

数据库测试要求 `.env` 中 `TEST_DATABASE_URL` 明确指向本地/CI 测试服务，测试创建并清理独立 schema。`ci-fast` 检查规则、格式、静态分析、单元测试、生成一致性、实体、流程和构建；`test-security` 还包含真实数据库集成测试。

当前浏览器 E2E 按项目指示暂停，不默认执行 `make ci`、浏览器集合或推送触发远程 CI。本地测试、真实模型效果、原生客户端互通和远程 CI 需要分别报告。

## 为下一次 Agent 开发保留方法

开发顺序是：检查现状 → 明确角色/状态 → 仅询问未决业务问题 → 发现并复用能力 → 确定事务与引擎 → 修改唯一源并生成 → 贯通入口与页面 → 验证实际行为。

优先直接复用已有动作，其次 Gateway 组合，再考虑兼容扩展；确有独立语义缺口才新增。工程决策由 Agent 基于代码完成，不要求用户反复批准已授权工作。

- [Agent 开发步骤与提问矩阵](docs/agent-workflow.md)
- [任务与 Skill 路由](docs/agent-tooling.md)
- [能力拆分与组合](docs/capability-composition.md)
- [实体模型](docs/entity-model.md) · [安全边界](docs/security.md)
- [测试与质量门禁](docs/testing.md) · [决策记录](docs/decisions/)
