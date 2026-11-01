# API Design First

继承服务规则；使用 .agents/skills/goa-service-designer/SKILL.md（仓库根路径）。按变更阅读其 modeling/transport/generated-code 参考，不必加载无关参考。

Payload、Result、Required、Validation、HTTP 映射、Error 和 Security 必须在 DSL 明确。按方法实际适用范围声明错误；不要为获得构造器把单方法错误提升到整个服务。

命名检查下游 TypeScript 内建类型冲突，例如使用 TechnicalRecord 而不是 Record。UTC 时间点用明确格式；租户/owner 不作为客户端可信授权来源。

执行 make generate，更新非生成目录实现与受影响 SDK 消费者，再 make check-generated、两端类型/编译和 HTTP 测试。不得手改 OpenAPI 或 SDK 修补不兼容；回到契约或生成器配置。报告字段删除、收紧校验、状态码/权限改变的兼容性影响。

## 契约边界

- design 只保留 `identity`（登录、会话、快照、临时权限）、`capability`、`ai`、`assistant`、`branding`（公开投影）与 `system`。
- 业务模块（settings、dictionaries、files、notifications、mcp、identity 管理）不再声明 Goa 服务；它们的契约是能力定义。新增业务动作时改 `internal/modules/*/capability.go`，不要在这里加 Method。
- 只有新增/修改能力信封、事件、审批或会话契约才需要改 design 并重新生成 SDK。

## 能力与 AI 契约

- capability 服务的 `CapabilityContract`、`CapabilityResult`、`CapabilityInvocation`、`CapabilityApproval`、`CapabilityEvent` 是平台公开契约；字段改名或状态枚举变化属于兼容性变更，必须同步生成 SDK、前端与测试。
- `CapabilityResult.status` 表达业务结果（succeeded/failed/awaiting_approval），请求级拒绝仍用 HTTP 4xx + IAMProblem；不要把两者混用。
- `input`/`result`/`arguments` 一律是 `Any` 的脱敏投影；不得把完整审批正文放进契约。
- ai 服务只包含任务入口与执行记录；能力目录、审批、事件属于 capability 服务，禁止在 ai 服务里复制。
