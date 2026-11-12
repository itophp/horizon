# 业务应用

继承根规则及 [Go 底座实现规则](../foundation/AGENTS.md) 中适用的能力、安全和数据访问约束。

- 新业务功能必须放在 `applications/<module>/`。已有业务在所属模块内扩展，新增独立业务在此创建模块。
- 网盘、项目、文章和示例分别维护各自的业务逻辑、能力 `Entries()` 和实体声明。底座通过 `foundation/runtime/app` 装配。仅当改动触及底座装配、传输或存储时，继续读取底座对应子目录规则。
- Runtime 应用定义和内置插件贡献随所属应用维护。业务页面按 [Runtime 开发规范](../docs/application-runtime-development.md) 选型，前端专用组件在 `apps/web`。
- 迁移与 SQL 查询统一在 `foundation/db`，数据访问复用根目录 `internal/repository/dbgen`。
