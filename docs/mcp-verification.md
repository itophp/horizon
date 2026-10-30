# 账号级远程 MCP 首阶段验证记录

日期：2026-10-30。本地验证，不代表远程 CI 或生产部署已执行。用户已选择自行完成 WorkBuddy 桌面端最后一次工具调用验收；这项不记为自动化通过。

## 对照首阶段交付

| 要求 | 实现与证据 | 状态 |
|---|---|---|
| 个人 Token 的创建、列表、轮换、撤销、工具集合与部门/有效期 | Goa 源 design/mcp.go；追加迁移 00010/00011；sqlc 查询；MCPTokenLifecycleAndIsolation 与 MCPSecurityBoundaries | 已实现、真实 PostgreSQL 已验 |
| 强随机秘密、不可逆摘要、仅显示一次、重新认证 | service.go 的随机签发与 VerifyCurrentPassword；HTTP 不返回摘要；浏览器关闭明文提示后不可查询 | 已实现、单元/数据库/浏览器已验 |
| 独立远程入口及首批五个只读工具 | internal/mcp 的 SDK 适配和显式注册表；原应用服务/生成类型映射；协议输入与 Goa 边界约束测试 | 已实现、协议与客户端已验 |
| 实时正式授权、本人/部门/租户隔离 | PrincipalForTask；原通知/字典服务；MCP 不继承已生效网页临时授权；跨租户通知 ID 拒绝 | 已实现、允许/拒绝已验 |
| 撤销、过期、禁用、移除归属与并发轮换 | 数据库时钟、账号锁与延迟归属触发器；移除后重加不能复活旧 Token；并发轮换只成功一次 | 已实现、真实 PostgreSQL 已验 |
| 调用审计、请求/结果上限、取消/超时与限流 | 有界 IP/账号/Token 限流、1 MiB 请求/512 KiB 结果、30 秒上下文和独立有界审计写入；保留期 worker | 已实现，直接规则与定向验证；不宣称已做生产压测 |
| 管理页面、中英文、主题一致性、状态与诊断 | `/mcp` 的远程 MCP 标签；生成 SDK/RHF/Zod/Query/DataTable；官方 MCP 浏览器客户端完成初始化/发现/调用 | 已实现、浏览器链路与浅暗截图已验 |
| 一键安装与无助手导入 | 固定服务地址的 loopback 助手；单次证明和显式确认；配置/凭证/Skill 备份；损坏配置拒绝覆盖；两平台示例 | 已实现、真实助手与隔离客户端配置已验 |
| Codex 原生调用与撤销拒绝 | 实际 codex-cli 0.165.0-alpha.1.2 app-server，独立临时配置；不启动模型 turn | 已验 |
| WorkBuddy 原生连接/发现与撤销拒绝 | 腾讯 WorkBuddy 5.8.2 随附 CLI 使用隔离配置执行真实 MCP health；撤销后不能连接 | 已验 |
| WorkBuddy 桌面端工具调用 | 用户明确选择稍后手动验收；没有改动其当前任务或配置 | 用户待验，不记通过 |
| 工程 Skill 与长期路由 | baseline-mcp-development 专注未来业务工具创建/变更；安全/目录规则/ADR/工具矩阵同步；官方作者验证器已运行 | 已完成 |

## 已执行检查

- `make ci-fast`：通过，含规则、格式、ESLint、TypeScript、前端测试、Go 检查/测试、已有 WebMCP 与新助手测试、生成可重复性和构建。生产构建仍有体积提示，无构建错误。
- `make test-security`：通过，包括真实隔离 PostgreSQL 集合；原生客户端测试采用显式独立 target，不将默认未选择它的情况当作原生验收。
- `make test-mcp`：通过，新增临时授权、通知入口范围与并发/过期/归属失效专项随后单独完成模块回归。
- `WORKBUDDY_CLI=本机随附CLI路径 make test-mcp-native`：通过；Codex 原生工具调用、WorkBuddy 原生连接及两者撤销后的拒绝，均未触及用户客户端配置。
- `PLAYWRIGHT_CHANNEL=chrome WORKBUDDY_CLI=本机随附CLI路径 make test-mcp-native-e2e`：通过，真实浏览器、Go、数据库、安装助手、原生客户端及安装后的 stdio 桥链路。
- 官方 `quick_validate.py`：baseline-mcp-development 与同步的 baseline-security 均通过；使用临时隔离 PyYAML 环境，没有改全局 Python。

最终相关浏览器回归的具体结果在交付时补充。未运行发布候选完整 `make ci`；未执行生产迁移/部署或远程 CI。

## 环境失败的处理

初次沙箱执行无法访问本机端口/数据库，获准后使用同一隔离目标重跑；初次浏览器缺配套 Chromium，改用本机 Chrome。长链路 Trace 证明 180 秒总预算在安装已成功后耗尽，当前测试有界使用 360 秒并保留全部断言。直接 pnpm 执行缺 TEST_DATABASE_URL 时未启动测试，随后通过 Make 加载项目隔离测试环境重跑。这些失败均未报告为通过。

## WorkBuddy 手动收尾步骤

1. 启动带新迁移的 Go 服务及前端，打开 `/mcp` 并停在远程 MCP 标签，创建本人/目标部门的 Token，勾选 context_get 和 capabilities_list；保存一次性明文。
2. 按页面命令在客户端电脑启动助手，填配对码与 Token，点击 WorkBuddy 并确认目标地址；重启/刷新 WorkBuddy 的 MCP 连接。
3. 在独立验收会话要求：“使用 baseline-business 实际调用 context_get 与 capabilities_list，输出账号和部门 ID、可用工具名，不修改数据。”检查工具调用卡与网页调用记录，不只依据模型文字判断。
4. 在网页撤销 Token，再要求实际调用，确认客户端提示鉴权失败。不要把 Token 粘进聊天；不再使用的本机凭证/备份按操作指南清理。

[操作与恢复指南](mcp-operations.md) · [架构和安全边界](decisions/011-remote-mcp.md)
