# 一次能力调用的逐层剖析

以 `example.record.archive` 为例，从浏览器按钮一路拆到数据库、Outbox、AI 触发，逐层说明**每层做了什么、依据哪段代码、失败时返回什么**。

选择这条链路的原因：它同时压到权限、Schema、幂等、风险策略、审批恢复、同事务 Outbox 与事件分发，一条调用能把平台的主要约定全部走一遍。

代码位置统一相对 `services/api/`，前端相对 `apps/web/src/`。

## 0. 先建立全局图

```
浏览器按钮
  └─ capabilityInvoke（生成的 SDK）
      └─ POST /api/capabilities/invocations
          ├─ handlerWithCore        请求 ID、安全头、体积上限、日志
          ├─ identityGuard          会话 + CSRF（在 mux 之前）
          ├─ Goa 解码               gen/http/capability/server
          ├─ capabilityAdapter      固定 origin=human-ui
          ├─ gateway.Invoke         身份→目录→权限→Schema→幂等→策略
          │    ├─ 需审批 → requestApproval（一个事务写调用行+审批卡）
          │    └─ 直接执行 → run
          │          ├─ CapCreateInvocation（executing）
          │          ├─ execute → TxExecutor：业务行 + cap_outbox 同事务
          │          └─ CapFinishInvocation（succeeded/failed + 耗时）
          └─ 返回 CapabilityResult
cap_outbox → Relay → cap_events → Hub → Trigger → ai_runs → Agent → 再调 Gateway
```

三层职责边界必须分清：

| 层 | 只负责 | 明确不做 |
|---|---|---|
| 传输（transport） | 会话、CSRF、体积、错误码映射 | 不判断能否执行、不碰业务数据 |
| 网关（gateway） | 身份、权限、Schema、幂等、策略、事务、审计、事件 | 不知道"归档"是什么意思 |
| 执行器（module） | 业务不变量、SQL | 不写审计、不发事件（只声明事件） |

---

## 第 0 层：浏览器

**入口**：`features/capabilities/page.tsx` 的归档按钮。

```ts
const result = await capabilityInvoke(ARCHIVE, {
  input: { id },
  idempotencyKey: `archive:${id}`,
});
if (result.status !== 200) throw new ApiError("unavailable", result.status);
if (failed(result.data)) throw new ApiError(result.data.errorCode || "invalid", 200);
```

三个值得注意的点：

1. **幂等键是稳定的**（`archive:<id>`），不是随机 UUID。同一记录重复点归档，第二次会拿到第一次的重放结果，而不是再判一次冲突。对照创建按钮用的是 `crypto.randomUUID()`——每次点击都是新的业务意图。
2. **HTTP 200 也可能是失败**。`failed()` 的判定是"既非 `succeeded` 也非 `awaiting_approval`"，所以业务失败与 `cancelled` 都走 200 信封，由页面转成 `ApiError`。
3. **前端不判断风险**。它不知道 `high`、不知道要不要审批，只是把参数送出去。

再往下是生成的客户端（`api/generated/baseline.ts` 的 `capabilityInvoke`）和统一 mutator（`api/client/request.ts`）：

```ts
const headers = new Headers(options.headers);
if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken)
  headers.set("X-CSRF-Token", csrfToken);
const response = await fetch(url, { ...options, signal, headers, credentials: "same-origin" });
```

- `credentials: "same-origin"`：会话 Cookie 自动带上，前端只持有 CSRF nonce。
- 超时：`/api/capabilities/` 与 `/api/assistant/chat` 给 120s，其余 10s。
- 非 2xx 映射成 `ApiError(code, status)`；401 且路径是 `/api/admin/` 或 `/api/capabilities/` 时派发 `baseline:unauthenticated` 事件，由 `AuthProvider` 刷新会话。

---

## 第 1 层：HTTP 服务器与守卫

### 1.1 进程级包装（`internal/transport/http.go:61`）

```go
id := hex.EncodeToString(raw[:])           // 16 字节随机
w.Header().Set("X-Request-ID", id)
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("Cache-Control", "no-store")
w.Header().Set("Referrer-Policy", "same-origin")
w.Header().Set("X-Frame-Options", "DENY")
defer func() {
    slog.Info("request completed", "request_id", id, "method", r.Method,
        "duration_ms", time.Since(started).Milliseconds())
}()
```

日志**故意只记 method 和耗时**，不记路径与凭证——路径里可能带资源 ID，能力调用体可能带业务内容。

体积上限按路由分层：

```go
limit := int64(1 << 20)                                  // 1 MB
if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/capabilities/") &&
   strings.HasSuffix(r.URL.Path, "/invocations") {
    limit = 29 << 20                                     // 29 MB，file.upload 的 base64 附件
}
r.Body = http.MaxBytesReader(w, r.Body, limit)
```

### 1.2 会话守卫（`internal/transport/identity.go:89`）

**注意顺序**：守卫在 `mux.ServeHTTP` 之前执行，所以它产生的 401/403 是 `deny()` 直接写的 JSON，**不经过 Goa 的错误编码器**。这解释了为什么守卫的响应体只有 `{"code": "..."}`。

```
guardedPath(/api/capabilities) == true
  → POST 属于"不安全方法" → sameOrigin() 必须通过
  → 路径不是 /api/auth/login → 必须带 baseline_session Cookie
  → service.Authenticate(cookie) 构造 Principal
  → unsafe 方法还需 CSRFMatches(p, X-CSRF-Token)（常量时间比较）
  → r.WithContext(identity.WithPrincipal(...))
```

`Authenticate`（`internal/identity/service.go:94`）就是登录第 12 步复用的同一个函数：查会话 → 查账号 → 定位当前部门 → 会话时效策略 → 取该部门角色授权 → 合并账号覆盖 → 建 Casbin 实例 → 挂临时范围。所以**每个请求都会重新解析权限**，撤销即时生效。

### 1.3 Goa 解码（`gen/http/capability/server/encode_decode.go`）

解码 `invoke` 的 payload：`name`、`version`、`input`（`Any`）、`idempotencyKey`、`correlationId`、`causationId`。这里只校验 HTTP 层面的形状（必填、类型）；**能力的业务 Schema 不在这里校验**，那是网关的事。

---

## 第 2 层：传输适配器（`internal/transport/capability.go:88`）

```go
func (a *capabilityAdapter) Invoke(ctx context.Context, payload *gen.InvokePayload) (*gen.CapabilityResult, error) {
	input, ok := payload.Input.(map[string]any)
	if !ok {
		return nil, capabilityProblem(contracts.Problem("invalid"))
	}
	...
	result, err := a.gateway.Invoke(ctx, gateway.Request{
		Name: payload.Name, Version: version, Input: input, IdempotencyKey: key,
		Origin: contracts.OriginHumanUI, CorrelationID: correlation, CausationID: causation,
	})
```

这一层只有四个动作：

1. 把 `Any` 断言成 `map[string]any`，失败即 `invalid`。
2. 解引用可选字段指针。
3. **固定 `Origin: contracts.OriginHumanUI`**。这是安全设计：HTTP 入口无法声明自己是 `agent` 或 `workflow`，否则外部调用可以伪装成"已获人工确认"的来源。Agent 的来源由 `internal/ai/tools/tools.go:77` 写入，两者不可互换。
4. 把 `contracts.Result` 投影成 Goa 类型（`resultProjection`）：`approvalId` 转指针、时间戳转 RFC3339、`replayed` 透传。

它不写一行业务逻辑，也不碰数据库。

---

## 第 3 层：网关入口（`internal/platform/capability/gateway/gateway.go:86`）

`Invoke` 是整条链路唯一做判定的地方。顺序固定，每一步失败都返回稳定错误码：

### 3.1 身份

```go
p, err := identity.Current(ctx)
if err != nil { return nil, contracts.Problem("unauthenticated") }
```

只认 `context` 里的 Principal；`Request` 结构体里**没有 tenant、account 字段**——结构上就无法从请求体传入身份。

### 3.2 请求形状（`validateRequest`，:149）

- `name` 非空，`TriggerDepth` 在 `0..MaxDepth(8)`。
- `origin` 必须在六个枚举内（`human-ui` / `agent` / `workflow` / `api` / `event` / `approval`）。
- `correlationId` / `causationId` 长度 ≤ 120 且不含 `\r\n`（防日志注入）。

### 3.3 目录解析（`registry.Resolve`，:82）

```go
if version == "" { return r.byKey[keys[0]], true }   // keys 已按版本新→旧排序
entry, ok := r.byKey[name+"@"+version]
```

目录在进程启动时由 `registry.New` 冻结：任何定义不合法（写能力缺幂等键、缺成功事件、Handler 双实现或双缺失、名称重复、工具名冲突）都会让 `catalog.Err()` 非空，服务照样起但所有调用返回 `not_found`——**部署错误不会被掩盖成请求期惊喜**。

### 3.4 权限

```go
caller := callerOf(p)                              // 包装 Principal.Can
if !caller.Allowed(definition.Permission) { ... "forbidden" }
```

`example.record.archive` 声明的权限是 `example:archive`。这一层是**第一次**权限判定，执行器里还有第二次（纵深防御）。

### 3.5 输入 Schema（`contracts/schema.go:36`）

```go
for key, raw := range object {
    child, declared := s.Properties[key]
    if !declared {
        if s.AdditionalProperties { continue }
        return Problem("invalid")     // 默认关闭：多余字段直接拒绝
    }
}
```

`AdditionalProperties` 默认 `false`，意味着定义是**封闭对象**。Agent 如果凭空多给一个字段，会被拒绝而不是被静默忽略——这条对模型场景很重要。

### 3.6 幂等键

```go
if definition.Idempotency.Required && strings.TrimSpace(request.IdempotencyKey) == "" {
    return nil, contracts.Problem("invalid")
}
```

归档是写能力，`Idempotency.Required=true`，所以缺键直接 400。

### 3.7 摘要与重放

```go
digest := contracts.Digest(request.Input)   // sha256(json.Marshal(input))
if request.IdempotencyKey != "" {
    if replay, found, e := g.replayWait(ctx, p, definition, request.IdempotencyKey); ... 
    else if found { return replay, nil }
}
```

`Digest` 用 `encoding/json` 序列化 map——Go 对 map key 排序，所以同一逻辑输入摘要稳定，可用于绑定审批载荷。

`replay`（:174）按 `(tenant, capability, idempotency_key)` 查 `cap_invocations`：

| 上次状态 | 本次返回 |
|---|---|
| `succeeded` | 原 `data` + `eventIds`，`replayed=true` |
| `failed` | 原 `errorCode`，`replayed=true` |
| `awaiting_approval` | 原 `approvalId` |
| `executing` / `requested` | `conflict` → `replayWait` 每 20ms 重试，最多 1s，等赢家落库 |

### 3.8 策略（`platform/policy/policy.go:23`）

```go
if origin == contracts.OriginApproval { return Decision{Allowed: true, Approval: ApprovalAuto} }
automated := origin == OriginAgent || origin == OriginWorkflow || origin == OriginEvent || origin == OriginAPI
if def.Risk == RiskCritical {
    if automated { return Decision{Code: "forbidden"} }
    return Decision{Allowed: true, Approval: ApprovalFourEyes}
}
switch def.Approval {
case ApprovalFourEyes: return Decision{Allowed: true, Approval: ApprovalFourEyes}
case ApprovalConfirm:
    if automated { return Decision{Allowed: true, Approval: ApprovalConfirm} }
    return Decision{Allowed: true, Approval: ApprovalAuto}   // ← 人类点击降级为直执行
default:
    if def.Risk == RiskHigh && automated { return Decision{Allowed: true, Approval: ApprovalConfirm} }
    return Decision{Allowed: true, Approval: ApprovalAuto}
}
```

这张表解释了一件容易被误读的事：**控制台点"归档"不会出审批卡**，因为 `confirm + human-ui → auto`。审批只拦自动化来源，以及 `critical` 需要第二人。

`default` 分支还兜了一个漏洞：定义忘了声明 `Approval` 但风险是 `high` 时，自动化仍要确认——策略表不因契约遗漏而降级。

### 3.9 审批卡复用

```go
if existing, e := dbgen.New(g.pool).CapPendingApprovalFor(ctx, ...{
    TenantID: p.TenantID, Capability: definition.Name,
    AccountID: p.Account.ID, ArgumentsDigest: digest,
}); e == nil {
    return &contracts.Result{ ... StatusAwaitingApproval, ApprovalID: existing.ID ... }, nil
}
```

同一能力 + 同一账号 + 同一载荷摘要已有未决卡，就直接返回它，不再占一行调用记录。否则会出现"第二行永远无法被恢复"的僵尸调用。

### 3.10 ID 与脱敏

```go
traceID := request.CorrelationID; if traceID == "" { traceID = identifier("trace_") }
invocationID := identifier("inv_")
redacted, _ := json.Marshal(definition.Redact(request.Input))
```

`Redact`（`contracts.go:200`）只保留 `Loggable` 白名单字段，字符串截断到 200 个 rune。归档只声明了 `Loggable: []string{"id"}`，所以审计里看不到其它任何东西。

---

## 第 4 层：策略之外的两个分支

### 4.1 需要审批（`requestApproval`，:231）

**一个事务**里做完三件事：

```go
tx, _ := g.pool.Begin(ctx)
q := dbgen.New(tx)
q.CapCreateInvocation(... Status: "awaiting_approval", Input: redacted ...)
approvalID, _ := g.approvals.Create(ctx, q, approval.Approval{
    ... Arguments: arguments, Digest: digest,        // 完整载荷，不是脱敏投影
}, ApprovalTTL)                                       // 10 分钟
q.CapSetInvocationStatus(... Status: "awaiting_approval", ApprovalID: approvalID)
tx.Commit(ctx)
```

- 调用行与审批卡**同事务**：不存在"有卡无调用"或"有调用无卡"。
- `approval.Store.Create`（`approval/store.go:59`）会把 `ttl <= 0 || ttl > 1h` 回落到 10 分钟。
- 审批表存**完整参数**（恢复执行要用），但 API 投影时走 `capabilityAdapter.redact` 再过一次白名单——正文、消息体不会出现在响应里。

### 4.2 审批决策（`Decide`，:277）

顺序即安全语义：

1. `approvals.Get(tenant, id)` 带租户条件；跨租户查不到 → `not_found`。
2. `card.Status != pending` → `conflict`（重复决策）。
3. `other := card.AccountID != p.Account.ID`；批别人的卡需要 `capability:approve`。
4. `four-eyes` 且是自己 → `forbidden`（不能自批）。`confirm` 允许本人确认。
5. 过期 → 顺手 `Expire`，返回 `conflict`。
6. `approvals.Decide` 是条件更新（`WHERE status='pending'`），返回 false 即 `conflict`——并发两个审批只有一个赢。
7. 拒绝：调用行置 `cancelled` + `rejected`，不执行任何业务。
8. 批准：`PrincipalForTask(tenant, 卡上申请人, 卡上部门)` **重建申请人身份**，再校验 `caller.Allowed(permission)`。**审批不放大权限**。
9. `run(..., resumed=true)`：复用同一 `invocation_id`，`CapResumeInvocation` 把状态改回 `executing`，`origin` 记为 `approval`（所以不会再要一次审批）。
10. 执行结果回写卡状态：`executed` 或 `failed` + 错误码。

---

## 第 5 层：执行编排（`gateway/execute.go`）

### 5.1 占行（`run`，:29）

```go
q.CapCreateInvocation(...{
    TenantID, AccountID, DepartmentID,          // 来自 Principal
    InvocationID, Capability, CapabilityVersion,
    Origin: string(request.Origin), IdempotencyKey, Status: "executing",
    Input: input,                               // 脱敏投影
    InputDigest: digest, CorrelationID: traceID, CausationID, TriggerDepth, TraceID,
})
```

唯一索引 `(tenant_id, capability, idempotency_key)` 冲突时（`pgcode 23505`）：

```go
if isUnique(err) && request.IdempotencyKey != "" {
    if replay, found, _ := g.replayWait(...); found { return replay, nil }
}
```

并发同键的两个请求，一个成功占行，另一个拿赢家结果，不会出现 500。

### 5.2 执行与事务边界（`execute`，:92）

```go
if entry.Handler.ExecuteTx != nil {
    tx, _ := g.pool.Begin(ctx)
    defer tx.Rollback(ctx)
    output, execErr := entry.Handler.ExecuteTx(ctx, tx, request.Input)
    if execErr != nil { return ..., execErr }
    eventIDs, eventErr := appendEvents(ctx, dbgen.New(tx), entry.Definition, output.Events, p.TenantID, meta)
    if eventErr != nil { return ..., eventErr }
    return output, eventIDs, tx.Commit(ctx)
}
// 无 TxExecutor 的旧服务：先执行业务，再另开事务写事件
```

两条路径的语义差别必须说清：

- `TxExecutor`：业务行与 `cap_outbox` **原子**，要么都成，要么都不成。
- 只有 `Execute`：事件在第二个事务，失败窗口存在，属于文档化的 best effort——所以新模块一律要求 `TxExecutor`。

### 5.3 事件契约（`appendEvents`，:136）

```go
if len(events) == 0 {
    if definition.ReadOnly { return []string{}, nil }
    return nil, contracts.Problem("event_missing")     // 写能力必须发事件
}
for _, event := range events {
    if !declared(definition.SuccessEvents, event.Type) { return nil, contracts.Problem("invalid") }
    if event.AggregateType == "" || event.AggregateID == "" { return nil, contracts.Problem("invalid") }
    pending := outbox.NewEvent(tenant, event.Type, event.AggregateType, event.AggregateID, event.Payload, meta)
    if !pending.Valid() { return nil, contracts.Problem("internal") }
    outbox.AppendTx(ctx, q, pending)
}
```

四条硬约束：写能力必须发事件、事件类型必须**事先在契约里声明**、聚合标识必填、事件 ID 由 `crypto/rand` 生成（`evt_` 前缀）。执行器想"偷偷改个状态不发事件"在这里被拦住。

### 5.4 收尾（`finish`，:177）

```go
q.CapFinishInvocation(...{ Status, Result: result, ErrorCode: code, DurationMs })
```

失败也写一行（`failed` + 稳定错误码 + 耗时），所以审计不会只留成功记录。

### 5.5 失败语义（`run` 的错误分支）

```go
code := mapCode(err)
_ = g.finish(ctx, p.TenantID, invocationID, contracts.StatusFailed, nil, code, time.Since(started))
if code == "forbidden" || code == "unauthenticated" {
    return nil, contracts.Problem(code)                 // 升格为 HTTP 4xx
}
return &contracts.Result{Status: "failed", ErrorCode: code, ...}, nil   // 200 信封
```

判定原则：**请求级拒绝**（没身份、没权限）走 HTTP 状态码；**业务级失败**（冲突、找不到、校验不过）走 200 + `errorCode`。

---

## 第 6 层：业务执行器（`internal/modules/example/module.go:160`）

```go
func (m *Module) archive(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil { return contracts.Output{}, contracts.Problem("unauthenticated") }
	if !p.Can(ArchivePermission) { return contracts.Output{}, contracts.Problem("forbidden") }
	id, ok := number(in["id"])
	if !ok || id < 1 { return contracts.Output{}, contracts.Problem("invalid") }
	row, err := dbgen.New(tx).ExampleRecordArchive(ctx, dbgen.ExampleRecordArchiveParams{
		TenantID: p.TenantID, ID: int64(id),
	})
	if err != nil { return contracts.Output{}, contracts.Problem("conflict") }
	...
	return contracts.Output{
		Data: map[string]any{"id": row.ID, "title": row.Title, "status": row.Status},
		Summary: fmt.Sprintf("record=%d", row.ID),
		Events: []contracts.Event{{
			Type: ArchivedEvent, AggregateType: "example_record", AggregateID: fmt.Sprintf("%d", row.ID),
			Payload: map[string]any{"recordId": row.ID, "status": row.Status},
		}},
	}, nil
}
```

四个设计选择：

1. **再取一次 Principal、再判一次权限**。网关已经判过，这里是纵深防御；模块可以被其它路径复用时不至于裸奔。
2. **租户来自 Principal，不来自入参**。`id` 是业务标识，`tenant_id` 是安全条件，两者拼在同一个 SQL 里。
3. **`number()` 放宽数字类型**：JSON 解码后所有数字都是 `float64`，但测试与内部调用可能传 `int`。
4. **不存在 → `conflict` 而不是 `not_found`**：避免外部用 404/409 的差异枚举记录是否存在。

---

## 第 7 层：Outbox 与事件分发

### 7.1 落库（`outbox.AppendTx`，`relay.go:59`）

`cap_outbox` 一行包含因果链：`invocation_id`、`correlation_id`、`causation_id`、`trigger_depth`、`account_id`、`department_id`。这些字段是后面去重与"谁触发的"追溯的唯一来源。

### 7.2 中继（`relay.go:86`）

```go
return &Relay{pool: pool, hub: hub, batch: 20, interval: 500 * time.Millisecond, maxAttempts: 5}
```

`claim` 用 `SKIP LOCKED` 领一批 → `publish` 做三件事：

```go
q.CapAppendEvent(...)        // 不可变事件日志
q.CapMarkOutboxPublished(...)
r.hub.Dispatch(ctx, eventhub.Event{...})   // 进程内 fan-out
```

顺序是"先落库、再推送"。推送失败不回滚已写入的事件；`retry` 用 `5 × attempts` 秒退避（上限 60s），超过 5 次标记 failed。

语义：**至少一次**。消费端必须按 `(tenant_id, event_id)` 去重，不能假设 exactly-once。

### 7.3 触发（`internal/ai/triggers/engine.go:142`）

```go
policies, _ := e.policies.PoliciesForEvent(ctx, event.TenantID, event.Type)
for _, policy := range policies {
    if event.Depth >= policy.MaxDepth { e.skip(ctx, event, policy, "depth_exceeded"); continue }
    executionID, claimed, _ := e.policies.Claim(ctx, event.TenantID, policy, event.EventID)
    if !claimed { continue }
    go e.run(event, policy, executionID)
}
```

- 深度检查在抢占之前，被拦的也写一条 `skipped/depth_exceeded` 执行记录——运维能区分"没策略"和"被深度拦住"。
- `Claim` 依赖 `(tenant_id, policy_id, event_id)` 唯一键，重复投递只会执行一次。

`run` 里再重建身份：

```go
principal, err := e.identity.PrincipalForTask(ctx, event.TenantID, event.AccountID, event.DepartmentID)
ctx = identity.WithPrincipal(ctx, principal)
run, err := e.launcher.RunTrigger(ctx, harness.TriggerInput{PolicyID, PolicyVersion, Instruction, Event})
```

Agent 拿的是**事件发起人**在该部门的身份，不是系统特权。

### 7.4 再调能力

`harness.RunTrigger` → `ai_runs` 建档 → Eino 循环 → `tools.Adapter.Tools()`：

```go
func (a *Adapter) Tools() []tool.BaseTool {
	definitions := a.Registry.Discover(a.Caller)     // 只返回调用者有权限、且非 forbidden 的能力
	...
}
```

工具执行时：

```go
key := fmt.Sprintf("run:%d:%s:%s", t.adapter.RunID, t.definition.Name, contracts.Digest(input))
result, err := t.adapter.Gateway.Invoke(ctx, gateway.Request{
	Name: t.definition.Name, Version: t.definition.Version, Input: input,
	IdempotencyKey: key, Origin: contracts.OriginAgent,
	CorrelationID: t.adapter.TraceID, CausationID: fmt.Sprintf("run:%d", t.adapter.RunID),
	TriggerDepth: t.adapter.Depth,
})
```

模型看到的工具结果是一个信封（`ok/status/errorCode/data`），业务失败以数据返回而不是中断对话；遇到 `awaiting_approval` 会标成 `approval_required`。**Agent 再调能力仍走完整网关**——权限、幂等、深度一样不少。

### 7.5 本例的真实结局

`example.record.archive` 发的是 `example.record.archived`，而 `00015` 只种了一条策略监听 `example.record.created`。所以归档的事件：

- 一定写进 `cap_events`；
- 没有任何策略命中，`aiStatus` 为空，不会产生 `ai_runs`。

要观察完整闭环，得看创建接口（`example.record.create`），它的事件会命中 `example-record-review`。

---

## 第 8 层：响应回程

`contracts.Result` → `resultProjection` → Goa 编码 → `request()` 解析：

```ts
const body = response.status === 204 ? "" : await response.text();
const data: unknown = body.length === 0 ? undefined : JSON.parse(body);
return { data, status: response.status, headers: response.headers } as T;
```

回到页面：`failed(result.data)` 为真就抛 `ApiError`，否则 `refresh()` 失效 records / events / invocations / approvals 四个 query（definitions 由能力目录自身缓存，不在刷新范围）。

---

## 9. 全链路失败码矩阵

| 阶段 | 触发条件 | 返回 |
|---|---|---|
| 守卫 | 无 Cookie / 会话过期 | 401 `unauthenticated` |
| 守卫 | Origin 不同源、CSRF 不匹配 | 403 `forbidden` |
| 守卫 | 登录路径超频 | 429 `rate_limited`（仅登录） |
| 传输 | 体积超限 | 连接中断 / 400 |
| 网关 | 名称不存在、目录构建失败 | 404 `not_found` |
| 网关 | 无权限 | 403 `forbidden` |
| 网关 | Schema 不符、缺幂等键、脏 correlation | 400 `invalid` |
| 网关 | 同键正在执行且 1s 内未完成 | 409 `conflict` |
| 网关 | 策略拒绝（critical + 自动化 / forbidden 审批） | 403 `forbidden` |
| 执行 | 业务冲突、找不到、校验失败 | **200** + `status:"failed"` + `errorCode` |
| 执行 | 数据库不可用 | 200 `failed` `unavailable` 或 503 |
| 审批 | 重复决策 / 已过期 | 409 `conflict` |
| 审批 | 跨租户 | 404 `not_found` |
| 审批 | 非审批人批别人的卡 / 自批 four-eyes | 403 `forbidden` |

---

## 10. 同一套代码的四个入口

| 入口 | origin 来源 | 与 human-ui 的差异 |
|---|---|---|
| 控制台按钮 | `transport/capability.go` 写死 `human-ui` | 高危直接执行，无需确认 |
| Agent 工具 | `ai/tools/tools.go` 写死 `agent` | 高危必须确认，复用卡去重 |
| 工作流 / 外部 API | 各自 adapter | 同 agent |
| 审批恢复 | `gateway/requestFrom` 写 `approval` | 跳过策略，复用原调用行 |

四个入口共用同一份权限、Schema、幂等、事务、审计代码——差别只在 `Origin` 一个字段。

---

## 11. 一致性边界（值得知道的取舍）

1. **`finish` 在业务事务之外**。`execute` 提交成功后若 `CapFinishInvocation` 失败，会返回错误，但业务数据已落库，调用行停在 `executing`。此时同键重试会得到 `conflict`。审计补齐依赖人工/巡检，代码里没有自动收敛。
2. **`execute`（非 TxExecutor）的事件是第二个事务**。窗口内崩溃 → 业务生效但无事件。新模块用 `TxExecutor` 可以规避。
3. **Hub 是进程内 fan-out**。Relay 用行锁领取，多副本不会重复领取；但触发只在抢到该行的进程里发生，没有跨实例广播。去重仍由 `(tenant, policy, event)` 唯一键保证。
4. **限流与审批都是单实例内存/数据库混合**：登录限流是进程内 map（多副本各自计数），审批是数据库（跨副本一致）。
5. **`cap_invocations.input` 只存脱敏投影**，完整参数只在 `cap_approvals.arguments`。想复现一次调用的完整输入，只有审批路径留了证据。

---

## 12. 建议的读代码顺序

1. `internal/transport/capability.go` —— 看入口有多薄（约 250 行，7 个方法）。
2. `internal/platform/capability/gateway/gateway.go` 的 `Invoke` —— 记住那 10 个判定顺序。
3. `internal/platform/capability/gateway/execute.go` —— 看事务边界与事件契约。
4. `internal/platform/policy/policy.go` —— 只有 57 行，风险矩阵全在这。
5. `internal/modules/example/module.go` —— 一个模块怎么写才是"对的"。
6. `internal/platform/outbox/relay.go` + `internal/ai/triggers/engine.go` —— 异步那半边。
7. `tests/ai_platform_test.go` —— 三条用例分别固定了闭环、跨租户、审批恢复的行为。

目录与文件的整体位置见 [能力地图](capability-map.md)。
