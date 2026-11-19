# 业务应用

继承根规则及 [Go 底座实现规则](../foundation/AGENTS.md) 中适用的能力、安全和数据访问约束。

- 新业务功能必须放在 `applications/<module>/`。已有业务在所属模块内扩展，新增独立业务在此创建模块。
- 网盘、项目、文章和示例分别维护各自的业务逻辑、能力 `Entries()` 和实体声明。顶层 `deployment/` 将应用贡献注册到 `foundation/runtime/app`。底座保持对应用包的零依赖。仅当改动触及底座装配、传输或存储时，继续读取底座对应子目录规则。
- Runtime 应用定义和内置插件贡献随所属应用维护。业务页面按 [Runtime 开发规范](../docs/application-runtime-development.md) 选型，前端专用组件在 `apps/web`。
- 业务表的 schema 变更归本应用：迁移随 `applications/<module>/migrations/`，由应用的安装、升级、卸载流程执行，不写入 `foundation/db/migrations/`（拆分执行链路落地前，新增业务表同样不得进入内核迁移）。
- 能力定义、实体、事件、Runtime 应用、可信组件、业务流程与业务设置都在本模块声明；manifest 用 `extensions` + `settings` + `managed` 表达，不在 `foundation/` 写业务名。
- 平台共享 schema 的 SQL 查询源仍在 `foundation/db`，数据访问复用根目录 `internal/repository/dbgen`。
