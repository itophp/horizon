# Memory Center Phase 1 + Phase 2：AI 交接

更新时间：2026-11-05。此文档记录当前工作区快照，不代表阶段验收完成；继续工作前以代码、Git 状态和实际测试结果为准。需求来源为 [技术架构与开发规范](./AI-Native%20Memory%20Center%20技术架构与开发规范.md) 和本任务已确认的 Phase 1 + Phase 2 实施方案。开发前重新阅读根目录及目标目录的 `AGENTS.md` 和适用 Skill。

## 当前状态

已在工作区实现主要基础能力，**尚未完成完整验收**。原始事件与长期记忆分离；事件不会自动成为长期记忆。本阶段没有接入 LLM、Embedding、向量检索或自动提炼，也没有建设完整 Memory Console。

## 实现入口

| 位置 | 内容 |
| --- | --- |
| `services/kernel/memory/event.go` | Canonical Event Envelope、Collector、事件类型及载荷白名单策略。 |
| `services/db/migrations/00026_memory_events.sql`、`services/db/query/memory.sql` | 事件、消费进度、记忆、来源、关系、纠正、审计、保留策略，以及租户和范围约束查询。`dbgen` 由 sqlc 生成。 |
| `services/system/memory/` | 能力注册、实体声明、消费器、观察队列、记忆管理、范围授权、保留策略和重放。 |
| `services/runtime/app/capabilities.go`、`entities.go` | 模块及实体装配。 |
| `services/runtime/transport/memory_observation.go`、`services/kernel/capability/gateway/observation.go`、`services/kernel/workflow/observation.go` | HTTP、Gateway 和 Workflow 观测接入。 |
| `apps/web/src/lib/activity.ts`、`features/administration/activity-capture.tsx` | 已验证会话的页面访问和白名单交互采集；配置项为 `memory.activitySamplePercent`。 |
| `services/cmd/capabilitysdk/`、`apps/web/src/api/generated/capability-memory.ts`、`apps/web/src/lib/capabilities.ts` | 从能力定义生成的 TypeScript 类型及 Gateway 调用包装。禁止手改生成文件。 |
| `services/tests/memory_*_test.go` | 隔离 PostgreSQL 上的 Memory 功能、授权、消费和恢复测试。 |

## 已实现行为与边界

- 持久业务事件通过 `cap_events` 和独立 inbox 消费；以逐事件状态而非单一高水位处理迟提交，按租户和事件 ID 去重，支持失败重试、死信、单条及有界范围重放。范围重放每批最多 100 条，以源事件 ID 为闭区间，返回续跑游标。重放和对应 Outbox 写入同事务。持久调用及 AI 运行元数据另做投影，不复制请求、响应或对话正文。
- HTTP、Gateway、Agent/Workflow 等采集安全元数据；未经授权的正文、凭证和隐藏推理链不进入记忆事件。非业务观测使用有界内存队列和有限重试，崩溃时可能丢失队列内数据；不得将其描述为持久审计。现有自持事务模块保留提交后 best-effort 事件边界。
- 六分区共同模型支持人工创建、列表、详情、来源、版本化纠正、归档和删除。列表支持分区、所有者、创建时间、状态及分页；默认只返回有效的 active 记录，归档须显式请求。创建支持有效期，普通检索排除未生效和已过期记录。删除清理本阶段的来源、纠正及关联副本，不删除原始事件或审计事实。
- 新权限 `memory:read/write/collect/replay/manage` 不自动授予现有角色，也不可临时申请。个人数据按租户与本人限制；项目通过 `project.get` 和成员校验，部门/组织复用正式 IAM 范围。业务实体当前仅有 `example_record` 授权适配，其他类型默认拒绝。项目组合能力的内层写入及其事件原子提交，外层组合事件后续提交。
- Memory 能力标记为 `manual-only`，当前 Agent/Workflow 不能直接调用；内部项目/实体数据能力要求指定父能力。来源查询只返回授权的最小投影。细节及保留策略限制见 [安全文档](./security.md#memory-center-当前实现边界)。
- 保留期限由租户配置；当前只记录策略，未启用自动清理。此阶段不硬编码统一保留期。

## 测试证据与未完成门禁

以下为本地历史结果，后续改动后必须重新运行；并非远程 CI 结论：

- 2026-11-04：完整 PostgreSQL 集成测试曾通过。随后 MCP 并行修改出现后，完整集成测试的 `TestArticlesMCPSubmission` 和 `TestOrchestrationMCPProtocolAndApproval` 因工具集合与令牌范围预期不一致失败；应先复核当前代码及这些测试，不能假定问题仍然相同或自行覆盖并行改动。
- Memory 专项集成测试覆盖个人、项目、部门、业务实体、有效期、授权、重放、去重和 Outbox 回滚；最近一次所有者筛选的个人/项目/部门/实体测试通过。实体检查、前端类型检查和相关 ESLint 曾通过；完整前端单元测试曾为 38 个文件、143 项通过。
- `go test ./...` 曾因 Windows 账户缺少创建符号链接权限而在文件存储测试失败。Windows 环境当时没有可用 `make`，因此 `make ci-fast` 和 `make check-generated` 尚未完成；已单独生成 Memory SQL 和 SDK，不能由此推断整套生成链路无漂移。
- 按现行项目指示，默认浏览器 E2E 暂停；未运行完整 E2E。远程 CI 未运行。需要的单模块浏览器旁证按根 `AGENTS.md` 的映射和授权规则执行。

## 接手顺序

1. 复核现存缺口，尤其是来源资源授权、各采集点的关联字段、范围内原始事件查询、过滤及失败恢复；未能证明授权的资源继续拒绝。验证非业务遥测的有限重试及崩溃丢失边界是否满足最终验收。
2. 在隔离环境验证完整生成链路，不覆盖工作区中 Goa/Orval 等并行产物；完成 `make check-entities`、`make check-generated`、`make test-integration`、`make test-security` 和 `make ci-fast`，记录真实失败或环境阻碍。
3. 将 Memory 专项测试和真实项目作为事件来源的端到端测试结合起来；核对跨租户、跨用户、权限撤销、并发冲突、来源不足、乱序及崩溃恢复。按规则保持 E2E 暂停，除非获得适用授权。

测试使用项目本地 PostgreSQL，集成 fixture 应创建独立 schema。2026-11-05 本轮曾启动项目 `postgres` 容器供测试；交接时不要假定它仍在运行。历史测试曾误写入公共 schema 的一条测试审计记录，已按精确条件删除并修复测试隔离；后续不要通过修改或删除公共 schema 数据来修复测试。
