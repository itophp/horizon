# 项目长期开发规则（Capability-Centric AI-Native）

本仓库是通用企业信息化底座。**业务能力（Capability）是系统唯一契约**：React 按钮、Agent Tool、Workflow 节点、外部 API 和事件触发都必须经过同一个 Capability Gateway 执行；业务事件经 Outbox 和 Event Hub 驱动 AI Trigger，Agent 再调用能力形成闭环。

```
Business Capability → Business Event → AI Behavior → Business Capability
```

用户最新明确指令优先。

## 开始任务与规则发现

1. 检查 Git 状态、目标模块与未提交变更，保留用户已有修改。
2. 改文件前读取根到目标文件父目录沿途所有 AGENTS.md；从根启动也要主动检查子目录规则。
3. 功能开发先用 [baseline-feature-development](.agents/skills/baseline-feature-development/SKILL.md)，按 [任务工具矩阵](docs/agent-tooling.md) 选择 Skill。README 维护见 docs/AGENTS.md。
4. 使用锁文件版本；升级/新增依赖先核对官方 API、兼容性与许可证，记录于 docs/versions.md / docs/decisions。
5. 涉及 MCP 暴露面时按能力契约与安全规则确认读写和授权范围。
6. 分析 Capability、Workflow、Graph、Runtime Application 或 Page Schema 时，必须先从仓库根使用 Horizon CLI。需要发现条目时，直接运行 `./horizon-cli capability`、`./horizon-cli workflow`、`./horizon-cli graph`、`./horizon-cli application` 或 `./horizon-cli page` 列出对应类型，可加 `--full` 批量读取完整定义。分析具体契约时，运行 `./horizon-cli <类型> schema [标识@版本]` 后再读源码核对语义。Windows PowerShell 用 `.\horizon-cli.exe`。二进制不可用或定义未重新构建时，用 `go run ./foundation/cmd/horizon-cli` 执行相同参数。见 [Horizon CLI](docs/local-development.md#horizon-cli)。

## 跨目录契约

- 可独立触发的业务动作由所属模块在 `Entries()` 注册一次；页面、API、Agent、Workflow 和事件共用 Gateway。能力之间也只经 Gateway 调用，事务执行器内禁止互调。判据见 [能力拆分与组合](docs/capability-composition.md)。
- 业务模块维护实体、关系与状态转移。新增表、字段、能力或修改状态时同步实体声明，并运行 `make check-entities`。原子业务写入与 Outbox 同事务提交；既有的提交后写事件路径按其实际一致性边界说明。详见 [实体模型](docs/entity-model.md) 和 [底座实现规则](foundation/AGENTS.md)。
- 服务端以可信身份执行授权和租户、账号范围检查。各入口的审批、幂等与敏感数据保护遵循同一契约。调用、事件和 AI 任务通过关联标识追踪。
- 生成产物统一由源重新生成。改动契约或生成输入后运行 `make generate` 与 `make check-generated`。禁止手改生成代码、关闭检查、吞异常、删除断言或伪造测试结果。

## 目录与任务入口

**业务功能开发必须放在 `applications/`。** 新业务模块建在 `applications/<module>/`，已有业务功能在所属应用内扩展。业务逻辑、实体声明、能力定义和 Runtime 应用贡献随所属应用维护。

| 目录 | 职责与规则 |
|---|---|
| [foundation/](foundation/AGENTS.md) | Go 底座、运行时装配、身份、能力网关、编排、SQL 与 API 契约 |
| [applications/](applications/AGENTS.md) | 网盘、项目、文章和示例应用 |
| [apps/web/](apps/web/AGENTS.md) | React 前端、Runtime 渲染、SDK 消费与国际化 |
| `internal/repository/dbgen/` | 底座与应用共享的 sqlc 生成代码，输入和验证见 [SQL 规则](foundation/db/AGENTS.md) |
| [scripts/](scripts/AGENTS.md) · [docs/](docs/AGENTS.md) | 工程门禁、自动化脚本和规范文档 |
| [.agents/skills/](.agents/skills/AGENTS.md) | 项目工程 Skills 和维护约定 |

`go.mod` 在仓库根，Go 模块名为 `horizon`。任务按 [工具矩阵](docs/agent-tooling.md) 选 Skill；跨目录改动分别读取对应目录规则。

## 验证与交付

- **E2E 暂停（现行指令，优先于下述历史描述）**：默认不启动任何浏览器验收，包括 `make test-e2e`（无 `E2E_ARGS`）、`make test-e2e-parallel`、`make test-webmcp-e2e`、`make test-mcp-e2e`、`make test-mcp-native-e2e` 和默认 `make ci`。远程 CI 的 workflow 未改，仍是全量 E2E，因此推送/开 PR 等于触发全量，必须先取得授权。
- **只跑对应模块**：确需浏览器旁证时，按 `scripts/e2e-changed.mjs` 的现行映射选出改动模块对应的 spec，用 `E2E_ARGS="<spec>"` 只跑这些文件；映射未命中（如纯迁移 SQL）就不跑浏览器，在汇报里记未覆盖。一次只跑单个模块文件，不顺手扩大到同栈其它 spec。
- **全量需授权**：完整浏览器集合（`make test-e2e` 不带 `E2E_ARGS`、`test-e2e-parallel`、默认 `make ci`、推送触发远程 CI）必须在当轮得到用户明确授权后才执行；没有授权就写"E2E 未执行（按当前指示暂停）"，不得含糊成"未发现风险"。
- 测试按变更批次集中执行：普通连续开发累计受影响模块；阶段验收/提交/PR 跑一次 `make ci-fast`；触及表结构加 `make test-integration`。
- 浏览器验收优先相关文件；只跑子集必须报告选中文件与未覆盖集合，不能宣称全量通过。
- 代码生成与浏览器验收串行，避免生成产物干扰 Vite。不要把缺少数据库/浏览器导致的跳过报告为通过。
- 同一根因连续三轮无解时停止盲改，报告证据与选项。
- 汇报：行为与文件变化、使用的框架/Skill、实际命令与结果、生成影响、i18n/安全影响、未执行检查与剩余风险。本地通过与远程 CI 分开报告。

## Code Review 重点

优先报告跨层契约或权限边界被绕过、生成物与源不一致、缺失负向用例，以及未执行却报告通过的检查。底座实现和数据安全的专项检查见 [底座规则](foundation/AGENTS.md)，前端与国际化见 [Web 规则](apps/web/AGENTS.md)。
