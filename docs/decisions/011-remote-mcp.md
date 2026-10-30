# ADR 011：账号级远程业务 MCP

日期：2026-10-30。状态：已实现首阶段代码与自动化验收，WorkBuddy 桌面调用由用户稍后手动验收；OAuth 与写工具不在首阶段。

## 入口与契约

现有 Go 进程挂载 `/mcp`，官方 Go SDK v1.8.0 管协议和版本协商；Streamable HTTP 使用无状态模式，每次 HTTP 请求重新验证凭证/身份/工具集合。浏览器管理 API 在 `design/mcp.go`，前端只消费生成 SDK。协议工具输入在 `internal/mcp/registry.go` 显式定义并映射到既有生成用例类型；不把 OpenAPI 全量转换为工具。客户端 stdio 桥只转发工具发现和调用，业务服务不依赖活动网页。

首批：context_get、capabilities_list、dictionary_options、notifications_list、notifications_get。工具只读；字典按 dictionaries:use 及原来源字段/数据范围授权；通知按本人、有效归属与通知绑定 SQL 过滤。通知列表在 SQL 层分页，字典选项复用既有有界文档消费者并对结果分页。上下文只返回账号和绑定部门 ID，不返回会话/CSRF/角色策略。

## 凭证与身份

Token 是 `bmcp_` 加 256 位随机秘密，只保存 SHA-256 摘要；使用 Authorization，索引定位后常量时间比较。明文只在创建/轮换成功响应显示一次，禁缓存，不进入业务日志、列表、调用记录或安装配置。管理以浏览器会话 + Origin/CSRF 为边界，创建/轮换/撤销还必须验证当前密码，沿用身份服务的重新认证限流。MCP Token 不能调用管理 API。

每个账号最多 20 个有效 Token；默认 30 天、最多 90 天，数据库时钟计算。账号行锁使并发签发配额一致，轮换同事务撤销旧凭证并创建新凭证，保留到期时间与固化工具集合；不自动接纳新工具。可用能力是固化集合、当前正式权限和注册开关的交集，复用 identity.PrincipalForTask，不加载临时授权或合并部门角色。

移除归属、停用或重置密码使凭证失效。归属删除使用延迟约束触发器，在身份事务结束后检查最终归属；既有“清空再重建”流程不误撤销未改变归属，后续重新加入不复活旧凭证。请求重新鉴权仍是权限撤销的执行边界。

IP 限制只使用 RemoteAddr，不信任客户端转发头；进程内有界限流按 IP、账号、Token 分组，4096 桶上限和闲置清理。多实例部署需网关叠加集群限流。请求最大 1 MiB，处理超时 30 秒，工具结果最大 512 KiB。安全部署要求 HTTPS PUBLIC_APP_URL；MCP 校验 Host 和 Origin，非浏览器可省略 Origin，不接受 URL Token。Vite/生产代理必须把精确 `/mcp` 转给 Go，保留 Host/Authorization，禁止缓存与请求正文日志。

审计仅保存 Token ID、账号、部门、工具、稳定结果、request ID 与耗时。服务每分钟有界删除 90 天前审计，以及已过期/撤销超过 90 天的 Token 元数据；这是没有外部输入和数据输出的系统保留期任务。

## 本机接入与诊断

`pnpm mcp:helper` 监听 loopback 18796，启动时固定 MCP_BUSINESS_URL，生成仅本进程有效的随机配对码。精确校验 Host/Origin 和 Bearer 配对码，不接受请求指定远程 URL、命令或文件路径。准备阶段生成 60 秒单次证明；页面展示客户端和地址，由显式确认消费。安装先对远程服务完成初始化、目录发现和 context_get，校验与页面当前账号及所选部门一致。

安装的客户端服务名为 baseline-business，与 baseline-web 独立。Codex 用官方 CLI 合并，WorkBuddy 合并 mcp.json，非法 JSON 拒绝覆盖。凭证位于本机用户目录 `.local/share/baseline-mcp/{client}.json`（目录 0700/文件 0600），客户端配置只引用固定 stdio 脚本和凭证文件路径；argv 无秘密。安装前备份配置、同名凭证和 Skill，失败可能部分完成，页面提示检查备份；恢复见操作指南。没有助手时使用不含真实凭证的配置示例导入。

浏览器的独立诊断使用官方 MCP 客户端对同源 `/mcp` 完成初始化、发现与实际只读调用；不修改客户端配置、不持久化 Token，结束后清空输入。已安装与已连接是不同状态；Codex/WorkBuddy 的真实客户端行为必须独立验收。

## 验证与扩展

`make test-mcp` 覆盖本机助手、协议与真实 PostgreSQL；`make test-mcp-e2e` 覆盖浏览器、代理、诊断、WorkBuddy 配置及已安装 stdio 的独立调用；`make test-mcp-native` 和 test-mcp-native-e2e 显式启用真实 Codex app-server 与 WorkBuddy CLI 验收。桌面实际调用由用户接回手动验收，不能用 CLI health 代替。代码生成检查与浏览器串行；阶段 ci-fast，权限 test-security，原 WebMCP 回归继续保留。

以后功能新增和变动使用 baseline-mcp-development，按用例选择操作，维护工具名/版本、输入输出、Goa 映射、权限/范围、风险/幂等和测试。独立业务 MCP 工具不要触发修改全局凭证、安装客户端或新增基础服务。客户端运行时指南位于 webskills/baseline-business。
