# 业务应用插件化与内核边界说明

## 目的

本文说明新增业务应用时应如何选择“应用插件贡献”和“底座核心变更”，避免每新增一个业务能力都修改 IAM、Runtime、文件、治理等核心目录。适用范围包括新业务模块、内置业务应用、外部协议集成，以及需要复用平台身份、权限、文件和 Runtime 的功能。

核心原则：

- **业务语义归应用**：能力定义、实体、事件、页面、菜单、文案、协议适配和业务配置由所属应用维护。
- **平台语义归底座**：身份认证、授权决策、Gateway、事件总线、存储抽象、Runtime 协议和通用扩展机制由底座维护。
- **应用通过扩展点贡献**：应用不直接改核心目录中的业务枚举或清单；应用插件声明权限、能力、实体、事件、应用、组件和文件用途。
- **核心只在缺扩展点时修改**：新增的必须是通用机制，不能是某个业务的专用硬编码。

## 推荐结构

```text
applications/<module>/
  plugin.go             # Plugin interface + Registrar 注册
  plugin.yaml           # builtin 插件 manifest
  module.go             # 模块与 Executor
  capability.go         # 能力契约
  entities.go           # 实体声明
  runtime_v3.go         # Runtime 应用贡献
  options.go            # 应用自身业务配置

apps/web/src/features/<module>/
  plugin.tsx            # 前端插件贡献：路由、菜单、翻译、组件
  ...                   # 业务页面组件
```

部署装配只负责创建和启动插件，不逐项理解业务模块内部结构：

```go
plugins := plugin.New(
    projects.New(pool),
    mail.New(core, mailOptions),
)
```

插件通过 `Registrar` 贡献：

- capabilities
- entities
- permissions
- events
- applications
- frontend metadata
- file purposes
- settings（业务配置键；可在 manifest 声明）
- ui contracts（可信 Runtime 组件）
- workflows / graphs（持久流程与图资产）

声明式部分写入 `plugin.yaml` 的 `extensions`，内核按扩展枚举校验：

```yaml
id: com.horizon.example
version: 1.0.0
managed: true
extensions:
  - capability
  - entity
  - permission
  - event
  - application
  - settings
settings:
  - key: example.blockedWords
    group: example
    kind: string
    default: '""'
```

## 判断矩阵

| 变更 | 应放哪里 | 说明 |
|---|---|---|
| 业务能力契约 | 应用插件 | 能力归属业务模块，`Entries()` 由插件注册。 |
| 业务实体和状态机 | 应用插件 | 表和业务状态语义由应用维护。 |
| 业务事件 | 应用插件 | 事件名和 payload 语义由应用声明。 |
| 业务页面、菜单、文案 | 前端插件 / Runtime 应用贡献 | 不修改 core i18n 或 central navigation。 |
| 权限目录 | 应用插件注册 | IAM 只合并插件权限目录，不硬编码业务权限。 |
| 内置应用生命周期 | builtin plugin | 应用可安装、启停、升级，但由部署插件提供定义。 |
| 业务表迁移 | 应用包 `applications/<module>/migrations/` | 由应用安装、升级、卸载执行；不写入 `foundation/db/migrations/`。 |
| 平台共享 schema 与 SQL 源 | `foundation/db` | 身份、Runtime、治理等平台表的迁移与查询源；业务表不在此新增。 |
| sqlc / OpenAPI / Runtime 生成物 | 全局生成目录 | 从源生成，禁止手改。 |
| 新文件用途 | 插件声明 + 底座通用扩展点 | Files 提供通用 purpose 注册机制，不写死具体业务用途。 |
| 外部 endpoint 白名单 | 底座通用策略 | 与具体业务无关的 SSRF/TLS/超时策略属于平台能力。 |
| 业务大小、批次、重试配置 | 应用插件配置 | 不塞入 governance core。 |
| Runtime 协议缺陷 | 独立底座修复 | 不混入业务功能变更。 |

## 权限边界

### 错误做法

在 IAM 中央目录中硬编码每个业务的权限：

```go
"mail": {"read", "send", "configure", "sync"}
```

这会导致：

- 每新增应用都修改 IAM；
- 权限测试中的中央数量不断变化；
- 应用无法独立声明生命周期和权限元数据；
- 内核逐渐知道所有业务细节。

### 正确做法

应用插件注册权限：

```go
r.Permissions(
    plugin.Permission{
        Code: "mail:read", Module: "mail", Function: "mail", Action: "read",
    },
)
```

IAM 在启动时合并可信 builtin 插件权限目录。注册不等于授权；角色、覆盖和临时授权仍由 IAM 正式流程控制。

## 文件用途边界

### 不应复用普通 attachment 的情况

如果一个业务附件必须满足以下条件，就不应简单使用通用 `attachment`：

- 只能通过业务对象绑定读取；
- 不能出现在通用文件列表；
- 授权语义比 `files:read` 更窄；
- 删除、保留或清理与业务生命周期绑定。

邮件附件就是这种情况：收件人可以下载某封邮件的附件，但不应因此获得全局文件读取权限。

### 正确扩展方式

不应在 Files 核心里硬编码：

```text
mail_attachment
```

而应提供通用插件扩展点，例如：

```yaml
extensions:
  - file-purpose
filePurposes:
  - mail_attachment
```

Files Service 启动时合并插件声明的可信用途，并统一处理：

- upload schema；
- 数据库约束；
- 实体/生成校验；
- 绑定式读取；
- 存储和清理。

这样 Mail、Drive、Knowledge 或后续应用都不需要修改 Files 核心枚举。

## 配置边界

### 平台通用配置

以下属于底座通用策略：

- 外部 endpoint 精确白名单；
- TLS 最低版本；
- 连接和读写超时；
- SSRF 防护；
- outbound request 统一预算；
- 存储根目录和密钥策略。

建议建模为通用 `ExternalEndpointPolicy`，而不是：

```go
MailAllowedEndpoints
```

### 应用业务配置

以下属于应用自身配置：

- 邮件最大正文；
- 附件最大数量；
- POP3 同步批次；
- 发送重试上限；
- 业务级限额。

应由部署配置解析成应用插件 options：

```go
type Options struct {
    MaxMessageBytes    int
    MaxAttachmentBytes int
    MaxAttachments     int
    SyncBatch          int
    RetryMax           int
}
```

不应放入 `governance.Options`。Governance 只保留跨应用共享的策略和根密钥等平台能力。

## Runtime 边界

### 内置业务应用

业务应用应作为 builtin plugin 贡献：

```yaml
id: com.horizon.mail
version: 1.0.0
apiVersion: "1.0"
mode: builtin
extensions:
  - capability
  - entity
  - permission
  - event
  - application
```

部署基线应用在 manifest 声明 `managed: true`：reconcile 自动为其安装、升级并发布部署基线，租户生命周期和定义能力对它们返回 forbidden，无需为每个租户写种子迁移。示例应用的记录台与 workflow 演示就是这样从平台控制台迁回 `applications/example` 的。

`base_applications.go` 只列平台自有包；插件声明的托管包经 `Catalog.Managed` 合并进同一分类，业务插件 ID 不写死在那里。

### 前端组件

业务组件、路由、菜单和翻译由前端插件贡献：

```ts
export const mailFrontendPlugin: FrontendPlugin = {
  id: "com.horizon.mail",
  routes: [...],
  menus: [...],
  translations: {
    en: { mail: enMail },
    "zh-CN": { mail: zhMail },
  },
};
```

Runtime V3 的可信组件由插件注册（`applicationruntime.RegisterUIContracts` 或插件 `Register` 内调用），内核只保留通用渲染契约。业务组件名不得写进 central UI contract；`mail-workspace` 已按此方式迁到 `applications/mail/ui_contract.go`。

### Runtime 缺陷

例如“V3 detail page 的语义上下文解析仍只看 V2 page list”，这是平台缺陷。处理方式：

1. 单独提交底座修复；
2. 单独测试 article/project 等既有 V3 应用；
3. 不与某个业务功能混在同一变更中；
4. 说明修复的是协议行为，不是为某个应用开专用分支。

## 数据库和生成物边界

归属规则：

- 业务表的 schema 变更归所属应用：迁移随 `applications/<module>/migrations/`，由应用的安装、升级、卸载流程执行，不写入 `foundation/db/migrations/`；
- 平台共享 schema（身份、Runtime、治理、审计等）的迁移留在 `foundation/db/migrations`；
- sqlc 查询源与生成代码保持在 `foundation/db/query`、`internal/repository/dbgen`，由源重新生成，禁止手改；
- Runtime / OpenAPI / TS 生成物集中在各自生成目录。

状态：迁移按应用拆分的执行链路尚未落地，当前业务迁移仍在 `foundation/db/migrations`，作为后续独立阶段处理；规则以本节为目标，新增业务表不得再以此为由进入内核。

判断标准是：

- 新表和查询是否由业务模块拥有；
- 生成输入是否来自业务模块契约；
- 生成物是否禁止手改；
- 底座代码是否没有解析业务专用语义。

## 新应用接入清单

新增应用时按顺序确认：

1. 是否可作为 builtin plugin？
2. 权限是否能由插件 Registrar 声明？
3. 应用、页面、菜单和翻译是否能由插件贡献？
4. 是否真的需要新平台扩展点？
5. 如果需要，扩展点是否对后续应用通用？
6. 文件用途是否需要插件式 purpose？
7. 外部网络访问是否复用平台 endpoint policy？
8. 业务限额是否保留在应用 options？
9. 业务表迁移是否放在本应用包内并由安装/升级/卸载执行？平台表是否才进 `foundation/db`？
10. 底座缺陷是否拆成独立修复？

## 反模式

出现以下情况时说明边界已经失衡：

```text
新业务应用 -> 修改 IAM 权限枚举
新业务应用 -> 修改 base application 清单
新业务应用 -> 修改 core i18n
新业务应用 -> 修改 Files purpose 枚举
新业务应用 -> 修改 governance Options 字段
新业务应用 -> 修改 Runtime central component contract
新业务应用 -> 在 foundation/db/migrations 新增业务表
```

应替换为：

```text
新业务应用 -> 插件 manifest
新业务应用 -> Registrar 权限/能力/实体/事件/应用
新业务应用 -> 前端插件注册
新业务应用 -> 平台通用扩展点（如确实缺失）
```

## 落地状态

按本文边界已完成的调整（新增同类应用不应再触碰这些底座清单）：

| 项目 | 状态 |
|---|---|
| 权限目录 | 插件经 Registrar 声明，IAM 合并可信目录；内核无业务权限。 |
| 业务设置 | manifest `settings` 声明，governance 经 `RegisterDefinitions` 聚合；内核无业务键。 |
| 业务工作流/图 | 由插件 `Workflows()`/`Graphs()` 贡献，内核只保留类型、校验与通用构造器。 |
| 内存实体作用域 | 部署经 `MemoryEntityScopes` 注入资源、权限与存在性校验能力。 |
| 记忆页面白名单/项目写权限 | 部署注入，内核不硬编码业务页面与权限名。 |
| Runtime 页面业务区块 | 迁回所属应用（示例应用 playground）；控制台只保留平台视角。 |
| 可信 UI 组件 | 插件注册（`RegisterUIContracts`），内核无业务组件名。 |
| 文件用途 | 插件声明 purpose，Files 合并可信用途。 |

剩余项：业务表迁移从 `foundation/db/migrations` 拆到各应用并按安装/升级/卸载执行（独立阶段）。
