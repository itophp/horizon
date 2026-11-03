# 项目协作功能需求文档

状态：已实现（2026-11-03）
分支：`feat/project-management-example`
模块：`services/api/internal/modules/projects`（Domain `projects`，能力前缀 `project.`）
涉及层：迁移、sqlc 查询、实体声明、能力目录、权限目录、AI Trigger、前端页面

本文档最初是实施前的契约，现已按 P0–P3 落地。§15 记录实际实现与验证结果，§6、§11 中两处语义修正也在那里说明。

## 1. 目标与范围

提供一个可用的项目协作模块：项目建档、成员协作、计划拆解、任务执行，并把「能力 → 事件 → AI → 能力」闭环落到真实业务，而不是再造一个 CRUD 样例。

它必须做到：

- 所有写操作只经 Capability Gateway，页面、Agent 工具、事件触发共用同一套能力。
- 成员边界是数据边界：非成员看不到项目及其计划、任务，管理员也不例外。
- 状态流转有唯一入口，每个转移绑定一条能力并有对应用例。
- 任务分配触发 AI，AI 结论落库为独立产物，不污染业务表。

## 2. 决策摘要

已确认（本轮答复）：

| 决策 | 结论 |
|---|---|
| 任务层级 | 任务挂项目，`planId` 可空；计划是较大工作项，任务是执行项 |
| 可见范围 | 仅项目成员可见；非成员读不到、写不进，管理员不绕过 |
| 完成语义 | 执行人自己流转到 done，不引入验收链 |
| AI 参与 | 含事件触发与 Agent 工具，端到端打通 |

原待确认项（已按默认取值实现）：

| 编号 | 待确认项 | 默认取值 |
|---|---|---|
| Q1 | 任务是否需要负责人（assignee） | 可空：允许先建未分配任务，再指派 |
| Q2 | 计划类型取值 | `milestone` / `phase` / `deliverable` 三项固定枚举 |
| Q3 | 项目编号是否允许修改 | 创建后不可改，作为稳定业务标识 |
| Q4 | 「预计结束时间」精度 | 日期（`date`），不是时间点 |
| Q5 | 是否需要删除能力 | 不提供硬删除；项目归档、计划/任务取消 |

全部按默认值实现，未引入偏离。

## 3. 角色与权限

权限点决定「能不能做这个动作」，成员/负责人身份决定「能对哪个项目做」。两者同时校验，任一不满足即拒绝。

新增权限点（登记到 `internal/identity/catalog.go`）：

| 权限 | 作用 | 说明 |
|---|---|---|
| `projects:read` | 读取自己可见的项目、计划、任务 | 实际可见范围再由成员表过滤 |
| `projects:write` | 编辑项目、创建与编辑计划与任务、流转状态 | 仍要求是该项目成员 |
| `projects:admin` | 创建项目 | 建项目时创建者自动成为负责人与成员 |

角色规则（业务意图）：

| 角色 | 定义 | 可做的事 |
|---|---|---|
| 负责人 | `projects.owner_id` 指向的账号，且必有成员行 | 全部写操作 + 成员增删 + 负责人转移 + 归档项目 |
| 成员 | `project_members` 中存在记录 | 项目内计划与任务的创建、编辑、状态流转 |
| 非成员 | 无成员行 | 无任何读写，返回 `not_found` 而非 `forbidden` |

三点硬约束：

1. **管理员不绕过成员边界**。`IsAdmin` 只影响权限点判定，不豁免 `EXISTS (SELECT 1 FROM project_members ...)`。与 `example.record.get` 的取向一致。
2. **非成员一律 `not_found`**。区分 `forbidden` 与 `not_found` 会泄露项目是否存在，读与写都统一返回不存在。
3. **移除负责人前必须先转移**。`project.member.remove` 拒绝移除当前负责人；唯一成员也不能移除自己。

## 4. 数据模型

四张表 + 一张 AI 产物表。编号接在 `00023` 之后：`00024_projects.sql`。

### 4.1 projects

```sql
CREATE TABLE projects (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 code text NOT NULL CHECK (length(code) BETWEEN 1 AND 32),
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
 status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','completed','archived')),
 priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
 due_date date,
 owner_id bigint NOT NULL,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, code),
 UNIQUE (tenant_id, id),
 CONSTRAINT projects_owner_fkey FOREIGN KEY (tenant_id, owner_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX projects_owner ON projects(tenant_id, owner_id, id DESC);
```

`due_date` 可空（立项时可能还没定结束时间）；`owner_id` 必填，无法建出无负责人的项目。

### 4.2 project_members

```sql
CREATE TABLE project_members (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 project_id bigint NOT NULL,
 account_id bigint NOT NULL,
 joined_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, project_id, account_id),
 CONSTRAINT project_members_project_fkey FOREIGN KEY (tenant_id, project_id)
   REFERENCES projects(tenant_id, id) ON DELETE CASCADE,
 CONSTRAINT project_members_account_fkey FOREIGN KEY (tenant_id, account_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
```

成员表**不存 role 字段**。负责人身份只以 `projects.owner_id` 为准，避免两处角色漂移；代价是「我的项目」查询需要一次 JOIN，可接受。创建项目时在同一事务插入负责人成员行——这是 `projects.owner_id` 与成员集合唯一的耦合点，由 `project.create` 与 `project.owner.transfer` 保证，并由集成用例断言。

### 4.3 project_plans

```sql
CREATE TABLE project_plans (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 project_id bigint NOT NULL,
 plan_type text NOT NULL DEFAULT 'phase' CHECK (plan_type IN ('milestone','phase','deliverable')),
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 content text NOT NULL DEFAULT '' CHECK (length(content) BETWEEN 0 AND 20000),
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','in_progress','done','cancelled')),
 priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
 due_date date,
 owner_id bigint NOT NULL,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT project_plans_project_fkey FOREIGN KEY (tenant_id, project_id)
   REFERENCES projects(tenant_id, id) ON DELETE CASCADE,
 CONSTRAINT project_plans_owner_fkey FOREIGN KEY (tenant_id, owner_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX project_plans_project ON project_plans(tenant_id, project_id, id DESC);
```

「每个计划配置一个负责人」→ `owner_id` 必填，创建能力要求必传，且该账号必须是项目成员（业务校验，不由外键保证）。

### 4.4 project_tasks

```sql
CREATE TABLE project_tasks (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 project_id bigint NOT NULL,
 plan_id bigint,
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 description text NOT NULL DEFAULT '' CHECK (length(description) BETWEEN 0 AND 20000),
 status text NOT NULL DEFAULT 'todo' CHECK (status IN ('todo','in_progress','done','cancelled')),
 priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
 assignee_id bigint,
 due_date date,
 completed_at timestamptz,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 CONSTRAINT project_tasks_project_fkey FOREIGN KEY (tenant_id, project_id)
   REFERENCES projects(tenant_id, id) ON DELETE CASCADE,
 CONSTRAINT project_tasks_plan_fkey FOREIGN KEY (tenant_id, plan_id)
   REFERENCES project_plans(tenant_id, id) ON DELETE SET NULL,
 CONSTRAINT project_tasks_assignee_fkey FOREIGN KEY (tenant_id, assignee_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX project_tasks_project ON project_tasks(tenant_id, project_id, status, id DESC);
CREATE INDEX project_tasks_assignee ON project_tasks(tenant_id, assignee_id, status);
```

`plan_id` 可空（Q1 同源：允许零散任务）；`assignee_id` 可空；`completed_at` 由状态能力写入或清空，不单独开设能力。

### 4.5 project_task_briefs（P2）

```sql
CREATE TABLE project_task_briefs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 task_id bigint NOT NULL,
 run_id bigint NOT NULL DEFAULT 0,
 summary text NOT NULL CHECK (length(summary) BETWEEN 1 AND 4000),
 model text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, task_id),
 CONSTRAINT project_task_briefs_task_fkey FOREIGN KEY (tenant_id, task_id)
   REFERENCES project_tasks(tenant_id, id) ON DELETE CASCADE
);
```

同一任务只保留最新简报（`UNIQUE (tenant_id, task_id)` + upsert）。AI 产物独立成表，任务表不因 AI 失败被改写。

## 5. 状态机

### 项目

```text
(创建) → active
active  → paused      project.status.change
paused  → active      project.status.change
active  → completed   project.status.change
paused  → completed   project.status.change
active  → archived    project.status.change
paused  → archived    project.status.change
completed → archived  project.status.change
```

`archived` 为终态，归档后只读：计划与任务一律不可写。`completed` 仍可回到 `archived`，不回到 `active`——已完成的项目重新激活属于新项目，不做反向流转。

### 计划

```text
(创建) → pending
pending     → in_progress   project.plan.status.change
in_progress → done          project.plan.status.change
done        → in_progress   project.plan.status.change   （重开）
pending     → cancelled     project.plan.status.change
in_progress → cancelled     project.plan.status.change
```

### 任务

```text
(创建) → todo
todo        → in_progress   project.task.status.change
in_progress → done          project.task.status.change
done        → todo          project.task.status.change   （重开）
todo        → cancelled     project.task.status.change
in_progress → cancelled     project.task.status.change
```

进入 `done` 时写 `completed_at = now()`，离开时清空；进入 `cancelled` 同样清空。

每条转移在 `Transition` 声明里绑定能力名，`check-entities` 的 E8 会校验绑定完整性；但 SQL 谓词是否真的等价只有集成用例能证明，所以**每条转移一条用例**（见 §11）。

## 6. 能力清单

全部 `Layer: atomic`。读能力 `ReadOnly: true` + `TransientResult`，写能力 `Idempotency: required`（`tenant+operation+key`）+ 成功事件。

### 项目

| 能力 | 事务 | 权限 | 额外行级校验 | 事件 |
|---|---|---|---|---|
| `project.create` | Tx | `projects:admin` | 编号在同租户唯一；创建者写成员行 | `project.created` |
| `project.get` | — | `projects:read` | 成员 | — |
| `project.list` | — | `projects:read` | 成员；支持状态/优先级/关键字 | — |
| `project.update` | Tx | `projects:write` | 负责人或成员；归档态拒绝 | `project.updated` |
| `project.status.change` | Tx | `projects:write` | 负责人（归档必须负责人） | `project.status.changed` |
| `project.member.list` | — | `projects:read` | 成员 | — |
| `project.member.search` | — | `projects:read` | 成员；仅返回 id/name/email 最小投影 | — |
| `project.member.add` | Tx | `projects:write` | 负责人；已是成员时 `conflict` | `project.member.added` |
| `project.member.remove` | Tx | `projects:write` | 负责人；拒绝移除负责人与最后一个成员 | `project.member.removed` |
| `project.owner.transfer` | Tx | `projects:write` | 负责人；新负责人须为成员 | `project.owner.transferred` |

`project.update` 只接受 `name`、`priority`、`dueDate`。`code` 不在输入里（Q3），改编号等于换项目标识。

`project.member.search` 存在的原因：成员管理要选人，而普通成员没有 `accounts:read`。它只按姓名/邮箱前缀匹配同租户账号，返回 `id`、`name`、`email` 三字段，不做分页以外的扩展。

`project.get` 额外返回聚合计数（计划总数、任务总数、按状态分组、逾期任务数），由一条 SQL 完成，不拆成同步组合能力——它没有独立语义，也不被其它业务动作复用。

### 计划

| 能力 | 事务 | 权限 | 额外行级校验 | 事件 |
|---|---|---|---|---|
| `project.plan.create` | Tx | `projects:write` | 成员；负责人须为项目成员 | `project.plan.created` |
| `project.plan.get` | — | `projects:read` | 成员 | — |
| `project.plan.list` | — | `projects:read` | 成员；按项目过滤 | — |
| `project.plan.update` | Tx | `projects:write` | 成员；负责人变更同样要求是项目成员 | `project.plan.updated` |
| `project.plan.status.change` | Tx | `projects:write` | 成员；归档项目拒绝 | `project.plan.status.changed` |

`project.plan.update` 接受 `title`、`content`（MD）、`planType`、`priority`、`dueDate`、`ownerId`。计划内容不进 `Loggable`，与 `article.body` 的处理一致：可读，但不进调用审计与 trace。

### 任务

| 能力 | 事务 | 权限 | 额外行级校验 | 事件 |
|---|---|---|---|---|
| `project.task.create` | Tx | `projects:write` | 成员；`planId` 须属于同一项目 | `project.task.created` |
| `project.task.get` | — | `projects:read` | 成员 | — |
| `project.task.list` | — | `projects:read` | 成员；按项目/计划/负责人/状态过滤 | — |
| `project.task.update` | Tx | `projects:write` | 成员；`planId` 改绑须同项目 | `project.task.updated` |
| `project.task.status.change` | Tx | `projects:write` | 成员；归档项目拒绝 | `project.task.status.changed` |
| `project.task.assign` | Tx | `projects:write` | 成员；被指派人须为项目成员；指派给当前负责人时 `conflict` | `project.task.assigned` |

`project.task.assign` 独立于 `update`：它是高频动作、有独立事件（AI 简报由它触发）、会被 Agent 单独调用。这不是机械拆分，而是动作语义与副作用都不同。

### 明确不做的能力

- `project.task.delete` / `project.plan.delete`：用 `cancelled` 表达放弃，历史可追溯。
- `project.plan.complete.withTasks`：批量完成会产生「部分成功」语义，与「能力边界即事务边界」冲突，不做。
- 同步组合能力：本模块没有「多源聚合 + 结果裁剪」的独立用例，聚合计数靠单条 SQL 就能表达。**不为了凑示例价值造组合层。**

## 7. 事件与 AI 参与

### 事件

| 事件 | 触发点 | 用途 |
|---|---|---|
| `project.created` | `project.create` | 可供 Trigger 做立项检查（P2 可选） |
| `project.status.changed` | `project.status.change` | 归档通知等后续扩展 |
| `project.member.added` / `removed` | 成员能力 | 通知被加入者（复用 notifications） |
| `project.plan.created` / `status.changed` | 计划能力 | 计划完成回归项目进度 |
| `project.task.created` | `project.task.create` | 统计与看板 |
| `project.task.assigned` | `project.task.assign` | **AI 简报入口** |
| `project.task.status.changed` | 任务状态 | 完成率统计 |

事件全部通过能力的 `SuccessEvents` 写入 Outbox，与业务数据同事务提交。

### AI 简报（P2 主线）

```text
任务被指派 → project.task.assigned（Outbox）
  → Trigger 策略 project-task-brief（async，agent=task_briefer，maxDepth=5）
  → 受限 ADK agent：工具白名单 = 该身份可见的只读能力（project.task.get、project.plan.list、project.member.list）
  → 输出结构化 { summary }
  → project.task.brief.record 落库（仅接受 OriginAgent / OriginWorkflow）
  → 任务详情展示，标注「AI 生成，仅供参考」
```

约束：

- `project.task.brief.record` 拒绝 `OriginHumanUI` 与 `OriginAPI`：人不能手写一条「AI 结论」。
- 模型失败、超时、结构化输出缺字段 → `ai_runs` 记失败，任务与指派状态不变（AI 任务与业务调用独立生命周期）。
- 简报是建议，不改变任何业务字段。
- Trigger 按 `policy + event` 去重，重复事件不产生第二次简报。

这是本仓库 AI-native 主张的最小可信示范：**能力产生事实 → 事件驱动 AI → AI 只能通过能力写回 → 写回受来源限制**。

### 逾期扫描（P3 可选）

后台 worker 每日扫描 `due_date < current_date` 且状态非终态的任务，发布 `project.task.overdue`。该事件的产生不由某个状态能力驱动，`Transition` 不绑定能力（与 `notification_delivery` 的 worker 驱动转移同类，`check-entities` 以 hint 输出并在文档说明原因）。

## 8. 页面与交互

路由 `/projects`，侧边栏项受 `projects:read` 控制。

| 区域 | 内容 |
|---|---|
| 项目列表 | 编号、名称、负责人、优先级、预计结束、状态、计划/任务/逾期计数；按状态与优先级筛选，关键字搜编号+名称 |
| 项目详情·概览 | 基本信息与编辑、状态流转按钮（按当前状态显示合法目标）、成员列表与增删、负责人转移 |
| 项目详情·计划 | 计划列表（类型、标题、负责人、完成时间、状态、优先级）；编辑抽屉含 Markdown 编辑与预览 |
| 项目详情·任务 | 任务列表（标题、计划、负责人、截止日、状态、优先级）；行内改状态；指派下拉只列项目成员 |
| 任务详情 | 描述（MD）、简报区（P2） |

实现约束（沿用仓库约定）：

- 全部经 `apps/web/src/lib/capabilities.ts` 的 `invoke` 调用，不裸 fetch。
- 远程状态用 TanStack Query，表单用 React Hook Form + Zod，表格用 TanStack Table。
- 文案走 i18next，`en` 与 `zh-CN` 键集合一致；新增 `projects.json`，并在 `entity.json` 补四个实体的 `label` 与 `states`。
- 状态与优先级的展示文案只在前端，Go 侧只有标识。
- 归档项目的所有写按钮不可见且不可调用（服务端同样拒绝）。

## 9. 安全与数据范围

每条 SQL 都必须带 `tenant_id`，且对非成员返回空集：

```sql
-- 项目读：成员判定内联，不使用「先查后判」
SELECT p.* FROM projects p
WHERE p.tenant_id = $1 AND p.id = $2
  AND EXISTS (SELECT 1 FROM project_members m
              WHERE m.tenant_id = p.tenant_id AND m.project_id = p.id AND m.account_id = $3);
```

计划与任务读同样要经项目成员判定，不能只按 `project_id` 过滤——否则知道 project_id 就能越权。

- `cap_invocations.input` 只落 `Loggable`：标识、枚举、日期，不含 `content` / `description` / 成员邮箱。
- 写能力在 SQL 层用条件更新（`WHERE status = $expected`）避免并发覆盖；受影响行数不为 1 即 `conflict`。
- 幂等键 `tenant+operation+key`；重放校验账号、输入摘要与能力版本。
- 审批：本模块无高风险动作。负责人转移与归档是普通业务写，`RiskMedium` + `ApprovalAuto`，不借风险审批冒充业务审核。

## 10. 非目标

- 甘特图、看板拖拽、任务依赖与关键路径。
- 工时、成本、资源负载。
- 跨租户共享、对外公开链接。
- 远程 MCP 令牌开放本项目能力（能力自动可被 Agent 使用，但远程 MCP 的允许集合本次不扩）。
- 计划类型的字典化（先用固定枚举，扩展点记录在 §13）。

## 11. 验收标准

**功能主路径**

1. 有 `projects:admin` 的账号创建项目 → 状态 `active`，创建者是负责人且成员表有对应行。
2. 负责人加两名成员 → 非成员访问任一读能力返回 `not_found`，成员可读。
3. 成员创建计划（含 MD 内容与负责人）→ 负责人非成员时拒绝；是成员时成功。
4. 创建任务并指派给成员 → `project.task.assigned` 进入 Outbox；P2 下产生一条简报且任务字段未变。
5. 任务 `todo → in_progress → done` → `completed_at` 被写入；重开回 `todo` 时被清空。
6. 项目 `active → completed → archived` → 归档后计划与任务写能力全部拒绝。

**状态与并发**

7. 每条 `Transition` 一条用例：合法来源成功、非法来源 `conflict`。
8. 同一任务并发两次 `status.change`（不同期望来源）→ 只有一次成功，另一次 `conflict`。
9. 相同幂等键重放 → 返回原结果，不产生第二条事件与第二条业务行。

**权限与范围**

10. 非成员读/写：项目、计划、任务各一条负向用例，断言 `not_found`。
11. 跨租户：用另一个租户的 `projectId` 调用 → `not_found`。
12. 无 `projects:write` 但有成员身份 → 读成功、写 `forbidden`。
13. 管理员非成员 → 同样 `not_found`（证明 `IsAdmin` 不豁免成员边界）。
14. 移除负责人 → `conflict`；移除最后一个成员 → `conflict`。
15. 负责人转移后，`projects.owner_id` 与新负责人成员行一致（一条断言，防冗余漂移）。

**AI（P2）**

16. `project.task.brief.record` 由页面或 API 直接调用 → `forbidden`。
17. 模型超时 → `ai_runs` 失败，任务状态与负责人不变，无简报行。
18. 同一 `project.task.assigned` 重投 → 不产生第二次简报。

**门禁**

19. `make check-entities` 通过，且**做过负例**：故意把某个枚举多写一个值、把主实体从 `Entities` 移除，确认门禁失败后改回。
20. `make ci-fast` 通过；涉及数据库的用例走 `make test-integration`（隔离 PostgreSQL）。
21. 浏览器验收按根规则暂停；如需旁证，只跑 `features/projects` 对应 spec，并在汇报中列出未覆盖集合。

## 12. 实施顺序

| 阶段 | 内容 | 交付判据 |
|---|---|---|
| P0 契约 | 迁移 `00024`、`db/query/projects.sql`、`make generate`（sqlc）、四个实体声明、能力定义与 Handler、权限点、`app` 装配 | `check-entities` + `check-generated` 通过；能力目录可见 |
| P1 业务 | 能力实现（范围过滤、状态条件更新、事件）、`test-integration` 覆盖 §11 的 1–15 | 集成用例全绿 |
| P2 页面 | `lib/capabilities.ts` 投影、`features/projects`、路由与菜单、i18n（en/zh-CN） | 主流程可点通；类型检查通过 |
| P3 AI | `ai_trigger_policies` 登记、`task_briefer` agent、`brief.record` 来源限制、简报展示 | §11 的 16–18 通过 |
| P4 可选 | 逾期扫描、看板视图 | 不影响主线验收 |

P2 与 P3 可并行，但 P3 依赖 P0 的能力契约。

## 13. 风险与已知缺口

| 风险 | 说明 | 处置 |
|---|---|---|
| 成员过滤漏一处即越权 | 每个能力各自写 SQL，任何一个忘加 `EXISTS` 就是数据泄露 | 每条 SQL 统一片段；§11 第 10–13 条按能力逐一覆盖，不抽样 |
| `owner_id` 与成员表冗余 | 两处都可表达「谁负责」 | 只由 `create` / `transfer` 写，禁止其它路径改；集成用例断言一致性（第 15 条） |
| E8 无法证明 SQL 谓词等价 | `WHERE status = 'pending'` 是否等价于 `From: [pending]` 只有用例能证明 | 每条转移一条用例（第 7 条），不靠声明通过推定 |
| 计划类型硬编码 | 新类型要改迁移与枚举 | 先用 CHECK 枚举；若类型需要租户自定义，改走 `dictionaries` 并新增 `project.plan.type` 校验能力 |
| 简报覆盖写 | 同任务只留最新简报，历史不留 | 有需要再加 `brief_history` 表；当前以 `ai_runs` 保留运行记录 |
| 大列表性能 | 项目/任务列表未分页时可全量返回 | `list` 一律带 `limit`（默认 30，上限 100），与 `example.record.list` 一致 |

## 14. 待你确认

1. §2 的 Q1–Q5 是否按默认取值。
2. §3 的角色规则是否符合预期，特别是「管理员不绕过成员边界」。
3. §10 非目标是否接受（尤其是不做删除、不做看板）。
4. 是否需要把「项目成员变动」接入现有通知能力（`notifications`），还是本次只落事件不推消息。

确认后按 P0 → P4 顺序实施，每个阶段结束汇报实际命令与结果。

## 15. 实现记录（2026-11-03）

### 落地文件

| 层 | 文件 |
|---|---|
| 迁移 | `services/api/db/migrations/00024_projects.sql`（四张业务表 + 简报 + trigger policy） |
| 查询 | `services/api/db/query/projects.sql` → `internal/repository/dbgen/projects.sql.go`（sqlc） |
| 实体 | `internal/modules/projects/{entities,entity_project,entity_project_member,entity_project_plan,entity_project_task,entity_project_task_brief}.go` |
| 能力 | `internal/modules/projects/{module,schemas,project,plan,task}.go`（22 个原子能力） |
| 装配 | `internal/identity/catalog.go`（3 个权限点）、`internal/app/{capabilities,entities}.go` |
| 前端 | `apps/web/src/features/projects/{page,fields,members,plans,tasks}.tsx`、`lib/capabilities.ts`、路由、菜单、`i18n/locales/*/projects.json`、`entity.json`、`lib/webmcp/routes.ts` |
| 测试 | `services/api/tests/projects_test.go`（5 个集成用例） |

### 两处语义修正（与初稿不同）

平台的 `appendEvents` 有一条硬约束：**非只读能力成功时必须产出一个事件**，否则返回 `event_missing`。初稿让「重复添加成员」「重复指派同一人」以成功 + 无事件结束，这在该约束下不可能成立，且会绕过「成功即事实」的不变量。

改为：**无变化的写返回 `conflict`**（`project.member.add` 已是成员、`project.task.assign` 指派人未变）。真正的重试仍由幂等键负责——相同 key 重放返回原 invocation。前端把这两种 `conflict` 当作「已满足」处理。

### 验证结果

| 检查 | 命令 | 结果 |
|---|---|---|
| 实体门禁 | `./scripts/go.sh run ./cmd/entitycheck` | 25 声明 / 53 表 / 493 列，0 问题；负例已确认（把 status 枚举多加 `ghost` → E4 失败、29 项 schema issue，恢复后通过） |
| 生成同步 | `node scripts/check-generated.mjs` | 通过 |
| 后端构建 | `./scripts/go.sh build ./...` | 通过 |
| 后端 vet | `./scripts/go.sh vet ./...` | 通过 |
| 后端单测 | `./scripts/go.sh test ./...` | 除 `internal/modules/files` 的符号链接用例（Windows 无权限创建 symlink，与本次无关）外全部通过 |
| 集成测试 | `TEST_DATABASE_URL=… ./scripts/go.sh test -tags=integration ./tests` | 全绿，含 `TestProjectCollaborationLifecycle`、`TestProjectMembershipRules`、`TestProjectConcurrencyAndReplay`、`TestProjectBriefSourceIsRestricted`、`TestProjectAssignmentIsTriggerable` |
| 前端类型 | `pnpm --filter @baseline/web typecheck` | 通过 |
| 前端单测 | `pnpm --filter @baseline/web test` | 112 通过（含 WebMCP 路由清单比对） |
| 前端规范 | `pnpm --filter @baseline/web lint` | 0 error（3 条 RHF `watch()` 的 react-compiler warning，与既有页面同类） |
| 文案一致 | `pnpm --filter @baseline/web i18n:check` | en / zh-CN 键集合一致 |
| 规则检查 | `pnpm rules:check`、workflow 相关 go test | 通过 |

### 未执行与已知环境噪音

- **浏览器验收（E2E）**：按根规则暂停，未启动。`/projects` 已加入 `lib/webmcp/routes.ts`（`connected: false`，暂不暴露页面工具），路由清单比对的单测通过。
- **真实 AI 运行**：`project-task-brief` 策略、事件与写回路径均已用集成测试证明；模型本身需要已配置的 provider 凭据，本地未配置，因此没有跑「真实模型产出简报」这一段。这不算通过，只算入口与出口已验证。
- **format 门禁**：`pnpm format:check` 与 `scripts/check-gofmt.sh` 在本机会对 138 个前端文件与 6 个既有 Go 测试文件报警，原因是工作区为 CRLF、索引为 LF（`git ls-files --eol` 已确认），与本次改动无关；本次新增/修改的文件均已格式化并通过。
- **已知 hint**：`check-entities` 新增一条 E6 hint —— `project_member.account` 的 `account_id` 只守卫 `gov_identity_refs` 的键，与 `file.organization` 同类，保持并说明原因。
