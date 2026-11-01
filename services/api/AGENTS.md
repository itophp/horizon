# Go API 规则

继承根规则。业务动作一律通过 `internal/platform/capability/gateway` 执行；新增能力先注册契约，再写 Executor，最后按需暴露 Goa。Goa DSL/传输契约变更使用官方 goa-service-designer；权限变更使用 baseline-security；SQL/迁移变更使用 baseline-database-change。

- design 为 API 契约源，gen 禁止手改；cmd 组装和生命周期，internal/app 为应用逻辑，internal/transport 为传输与中间件，repository 使用 sqlc/dbgen。不要在生成目录实现领域行为。
- 校验、Payload/Result/Error/HTTP/security 声明回到 DSL；读取生成接口后再实现，不凭记忆猜签名。OpenAPI + Orval 是前端类型来源。
- 只运行项目锁定 Goa 生成器；scripts/go.sh 在正确模块和工具链执行。全目录格式检查包含 integration build tag 文件，不能只 go fmt ./... 漏掉带标签文件。
- 配置集中验证，非法安全配置启动失败。统一请求 ID、结构化日志、时间/体积限制和稳定错误映射；日志与错误不得泄露凭证/内部数据。
- 用小接口/Fake 隔离依赖，context/cancellation 传播到 SQL；明确事务提交回滚，服务关闭有界。
- 先执行相关 scripts/go.sh test；后续 make ci-fast，触及 DB 再 test-integration。跨层行为按 E2E 暂停规则处理：默认不跑浏览器，需要旁证时只跑映射命中的单个 spec（见根 AGENTS.md）。生成变更必须 check-generated；安全变更必须允许+拒绝测试。

## Capability-Centric 分层

- `internal/platform/capability/contracts`：稳定词汇（Definition/Schema/Result/Event/Origin），不得引入存储或传输依赖。
- `internal/platform/capability/registry`：进程内定义中心；启动时校验版本、Handler、工具名冲突，失败即视为部署错误。
- `internal/platform/capability/gateway`：唯一执行入口；顺序固定为身份→权限→Schema→幂等→策略/审批→执行→Outbox→审计。禁止在 Handler 中绕开它直接调用 Executor。
- `internal/platform/{policy,approval,audit,outbox,eventhub,metadata}`：确定性策略、审批队列、审计读模型、事务 Outbox 与事件日志。`metadata` 是实体模型：从 embed 迁移解析表/列/外键、从能力目录派生 Capabilities，并执行 E1–E10 校验（`make check-entities`）。
- `internal/ai/{harness,tools,triggers,context,knowledge,workflow,evaluation}`：AI 任务、工具适配、事件触发与评估。Agent 工具必须经 Gateway；AI 任务与业务调用状态分离。
- `internal/modules/*`：业务模块（settings、dictionaries、files、notifications、mcp、example）。每个模块自持领域服务与 `Entries()`，并在 `entity_<name>.go` 声明自己的业务对象（`entities.go` 只汇总），不再声明自己的 Goa 服务或 HTTP 路由；写能力优先实现 `TxExecutor` 并声明 `SuccessEvents`。
- `design/` 只剩下会话/系统/公开投影与能力、AI、助手契约。新增业务动作不要加 Goa 方法，而是加一条能力定义并在 `internal/transport/governance.go` 装配。
- 新增 `/api/capabilities` 或 `/api/ai` 端点必须并入 `guardedPath`，补 `tests/ai_platform_test.go` 的允许/拒绝、跨租户、幂等与审批用例。
