---
name: baseline-entity-model
description: "Declare or change the business entity model (fields, relations, states) that is validated against the migrations. Use when adding a table or column, adding a capability, changing a status column or its transitions, or when make check-entities fails. The model is descriptive, not a second schema: every attribute, enum, relation and transition must match the physical schema."
---

# 实体模型

实体模型描述业务对象（有哪些字段、和谁关联、状态怎么变），由 `make check-entities` 对照迁移强制校验。它是能力目录的对象视图，不是第二套 schema，也不是第二个执行入口。

先读 [实体模型](../../../docs/entity-model.md) 与 [服务实现规则](../../../services/api/internal/AGENTS.md)。

## 什么时候必须动它

| 触发 | 动作 |
|---|---|
| 新增表或列 | 新实体写 `entity_<name>.go`；加列就在同实体加 `Attribute` |
| 新增能力 | 在 `Definition` 上填 `Entity`（主实体）；跨实体写用 `Entities`，主实体必须在内 |
| 改状态列 | 同步 `States` / `Transitions`，每个转移绑定触发它的能力 |
| 改枚举取值 | 改 `Attribute.Enum`，必须与 `CHECK (... IN ...)` 完全一致 |
| 新增/改外键 | 同步 `Relation`（正向 `Via` 是本地列，反向 `Via` 是子表名） |
| `make check-entities` 报错 | 按 code 修声明；**不要**为了让检查通过而删声明或放宽规则 |

## 文件布局

一实体一文件，聚合器只做汇总：

```text
internal/<module>/entity_<name>.go   # 一个 contracts.Entity 的完整声明
internal/<module>/entities.go        # func Entities() []contracts.Entity{...}
internal/app/entities.go             # 汇总所有模块，供 CLI 与 transport 共用
```

`internal/platform/metadata` 负责另一半：`schema.go` 从 embed 的 goose 迁移解析表/列/外键，`entity.go` 从能力目录派生 `Capabilities`/`Events`，`validate.go` 与 `transitions.go` 是 E1–E10。

实体声明是纯数据，写包级函数（`func exampleRecordEntity() contracts.Entity`），不要挂在 `*Service` 上——检查链路不该构造服务实例。

## 四个判据

**1. `Kind` 决定字段能不能被写。** `business`（用户可写）、`system`（服务端注入，如 `tenant_id`、`created_by`、`token_hash`）、`state`（只能经转移改变）、`reference`（指向其他实体）。模型和前端都靠它区分“表单字段”和“注入字段”，写错会让 Agent 往 input 里塞租户字段。

**2. 状态机与业务枚举要分开。** 只有当“状态变化走独立能力、有明确合法转移”时才用 `state` + `States` + `Transitions`。一次 update 任意改值的（如 `account.status`、`organization.status`）就是 `business` + `TypeEnum`，不声明状态机——否则转移图只能写成自环，读起来是错的。

**3. 转移必须绑能力，绑不到就说明白。** `Transition.Capability` 指向触发该转移的能力，E8 会检查该能力存在且声明了本实体。由 worker 驱动的转移（投递、排期发布）`Capability` 留空：这是事实，会以 hint 输出，不要硬塞一个能力名。

**4. 关系要经得起外键核对。** 全库复合外键是常态：`(tenant_id, x_id) → t(tenant_id, id)`。与外键目标列**同名**的列是作用域守卫（如 `gov_files.organization_id` 在 `(tenant_id, department_id, organization_id)` 里），不构成直接引用，声明成关系只会得到 hint。指向 `gov_identity_refs` 的键表示“指向某个账号”，`metadata.ReferenceAliases` 已把它映射到 `account`。

## 验证

```bash
make check-entities                        # 门禁：E1–E8、E10，已接入 ci-fast
ENTITY_DUMP=1 ./scripts/go.sh test ./internal/platform/metadata -run TestDumpSchema -v   # 表与列
FK_DUMP=1 ./scripts/go.sh test ./internal/platform/metadata -run TestDumpForeignKeys -v  # 外键
```

输出里的三类信息要区别对待：`schema issues` 与 `declaration problems` 是错误，必须修；`hints` 是“真实但没有键或能力强制”的事实，如果它描述的确实是设计意图，就保持并说明原因；`accepted gaps` 是同表不同行的实体故意不重复建模，新增时要给出理由。

**负例也是交付的一部分**：改声明后顺手把枚举多写一个值、或把字段改名，确认门禁真的会失败，再改回来。否则无法区分“校验通过”和“校验没生效”。

## 常见错误

- 只改迁移不改声明：加列后 E3 立刻报 `column "x" is NOT NULL but has no attribute`，看似麻烦，其实是唯一防止模型腐烂的机制。
- 手抄列清单：不要照搬 `CREATE TABLE`。字段与非空、枚举、json 形状由迁移派生后校验，人只声明 `Kind`、必填、关系、状态语义。
- 能力声明漏掉被修改的对象：`settings.draft.publish` 曾经把 `Entity` 写成产物 `settings_version`，`Entities` 里没有实际被改的 `settings_draft`。判断标准是“这个调用改了什么”，不是“它创建了什么”。
- 两类 code 要分清：`E8 ... does not declare entity` 是实体对不上；`C0 ... primary entity X is not listed in Entities` 是契约自身不合法，此时目录不可用，转移检查会被跳过（`skipped transition checks`），只需修 C0 那一行，不要被后续缺失的转移检查误导。
- 声明了实体却不加文案：新实体要在 `apps/web/src/i18n/locales/{en,zh-CN}/entity.json` 补 `label`，否则控制台退回标识符。
- 用 `Entity` 承载权限或查询逻辑：它只是分组与描述，授权仍在 Gateway 与 SQL 范围里。

## 不要做

- 不要为没有消费方的字段写文案（例如 `attribute.title`）：没有表单读它，加了就是会过期的手抄表。
- 不要手改生成代码或迁移去迁就声明；源是 `db/migrations` 和 `design/`。
- 不要把实体声明当成校验器：输入校验仍然是 `contracts.Schema`，实体只描述对象。
