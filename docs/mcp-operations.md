# 远程业务 MCP 接入与恢复

## 服务与账号

部署现有 Go 服务并执行新增迁移，前端代理精确 `/mcp` 至 Go。生产使用 HTTPS、COOKIE_SECURE=true 和 HTTPS PUBLIC_APP_URL，代理保留原 Host 与 Authorization；不要记录请求正文/Authorization。开发使用 `make migrate` 与既有启动命令；测试使用隔离 schema，不迁移开发库代替验收。

进入“远程 MCP”，选择名称、部门、天数和工具，输入当前密码创建。默认 30 天，最大 90 天，最多 20 个有效 Token。完整值仅显示一次，立即保存到客户端的凭证存储；关闭提示后无法再次查询。轮换会立即撤销旧值并显示新值，到期时间和工具集合保持不变；撤销也需要当前密码。

上下文和目录适合先验证连接。字典选项要求绑定部门拥有正式 dictionaries:use；通知仅本人可读。MCP 不继承临时授权。停用、归属移除、重置密码和显式撤销会使相应凭证失效；退出网页不会使有效 Token 失效。

## 点击安装

在需要接入客户端的电脑、仓库根启动：

```sh
MCP_BUSINESS_URL=https://YOUR_HOST/mcp pnpm mcp:helper
```

本机开发可用 loopback HTTP；远程地址要求 HTTPS。助手固定监听 127.0.0.1:18796，终端打印配对码。在页面“安装与连接诊断”输入配对码和 Token，所选部门必须与 Token 绑定部门一致；点击目标客户端，核对地址与平台后确认。安装需要 context_get 工具用于校验本人及部门。

Codex 通过本机官方 CLI 配置；可用 CODEX_BIN 指定已有 CLI 路径。WorkBuddy 合并本机 `.workbuddy/mcp.json`。两者安装同一个受限 stdio 桥与 baseline-business 使用指南。安装完成后按客户端要求重启/刷新 MCP，不能把“已安装”当成“已连接”。

助手不运行时可复制页面中的无秘密配置，按客户端规则提供 Token。不要把真实 Token 写进共享配置、分享链接或命令参数。Codex 环境变量方式须确保实际 GUI/CLI 进程能获取变量；使用助手安装的本地凭证文件方式可避开 GUI 环境继承差异。

## 测试连接

独立诊断不要求启动助手。输入 Token 后点击“测试远程连接”，只有远程初始化、工具发现和 context_get 都成功才显示成功；结束后清空输入。失败时检查有效期、撤销、账号/部门和代理。此诊断证明当前浏览器至远程服务可用，真实 Codex/WorkBuddy 仍须各自调用一次工具确认。

## 恢复与卸载

安装只修改目标客户端的 baseline-business 配置、同名使用指南和独立凭证文件，不覆盖其他服务。修改前的文件带 `.backup-时间戳` 后缀。失败可能已经保存了凭证或部分配置，先停止助手，再在本机恢复相应备份；备份凭证同样属于秘密，限制访问并按需安全移除。

- Codex 配置：`~/.codex/config.toml`，通过官方 `codex mcp remove baseline-business` 移除该条配置，或恢复完整备份。
- WorkBuddy 配置：`~/.workbuddy/mcp.json`，仅移除 mcpServers.baseline-business 或恢复备份，保留其他条目。
- 凭证：`~/.local/share/baseline-mcp/codex.json` 或 `workbuddy.json`；不再使用时移除文件及其秘密备份，并在网页撤销 Token。
- 使用指南：Codex 的 `~/.agents/skills/baseline-business` 或 WorkBuddy 的 `~/.workbuddy/skills/baseline-business`，需要恢复旧版本时使用同名备份。

MCP_CLIENT_HOME 仅用于独立测试客户端配置，正常安装应省略。移动仓库或 Node 路径后重新安装。助手关闭不影响已安装 stdio 桥直接连接远程服务。

本轮实现及已执行验证见 [验证记录](mcp-verification.md)。WorkBuddy 桌面端最终调用由用户选择自行完成，步骤见该记录；助手和原生 CLI 通过不代替此项。
