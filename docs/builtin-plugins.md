# 内置插件接入（Phase 1）

Phase 1 仅装配可信、随主程序编译的 Go 插件。插件框架位于 `foundation/kernel/plugin`，不动态加载进程、WASM 或远程前端脚本；安装、在线停用及持久化管理属于后续阶段。原始设计见 [AI-Native 插件接入方案](ai-native-plugin-integration.md)。

## 声明与装配

1. 业务包保持 `Entries()` 为能力唯一来源、`Entities()` 为实体来源；新增 `Plugin` 实现 `Manifest`、`Register`、`Start`、`Stop`。参考 `applications/projects/plugin.go`。
2. 提供并嵌入符合 `foundation/kernel/plugin/manifest.schema.json` 的 `plugin.yaml`。ID 唯一，版本采用 `major.minor.patch`；`apiVersion` 为 `1.0`，运行方式为 `builtin`。依赖填写插件 ID 和**精确版本**，不存在、版本不匹配或循环依赖均拒绝注册。
3. 在 `Register` 中向 Registrar 提供能力、实体及能力所引用的权限和成功事件。权限须提供 module、function、action、范围与临时授权语义；身份服务启动前将声明校验并合并到实例目录，已有权限必须完全一致，冲突则启动失败。事件和权限引用必须与能力声明一致，注册不会增加授权或创建另一套事件处理逻辑。所有类型必须在 Manifest 的 `extensions` 中声明。
4. 在 `runtime/app.BuiltinPlugins` 的内置插件列表提供插件实例及所需服务。它是运行时与离线检查共同的装配源；运行时将同一管理器的能力与实体加入既有目录。新插件必须补充 `plugintest.Contract` 测试，并验证真实 Gateway 调用与权限/租户边界。
5. 前端在业务功能包提供静态插件声明，再加入 `app/plugins/builtins.tsx`；插件注册表同时提供双语资源及可选的权限功能/动作标签映射；标签键必须来自该插件的双语资源，冲突或缺少译文时注册失败。页面路由、菜单以及双语资源由应用构建时引用。菜单的 `permissions` 仅用于过滤界面，业务动作仍使用 `lib/capabilities.ts` 和服务端 Gateway。

管理器先验证并暂存所有扩展，全部通过后发布；重复扩展或无效能力/实体使注册失败。服务启动时按依赖顺序调用 `Start`，失败时逆序停止已启动插件；进程关闭时逆序 `Stop`。离线目录检查只执行清单验证与注册，不调用插件 `Start`，也不执行能力。插件自身仍应保持注册阶段无副作用。

## 约束与测试

能力定义继续由业务模块维护，插件不能直接调用其他能力的 Handler 或绕过 Gateway；原有租户、审批、幂等、事务和 Outbox 边界不因插件化改变。内置 Go 插件与宿主同进程，属于可信代码，并非沙箱。服务重启或重新构建是唯一更新方式，不支持运行时热卸载。

至少运行插件包和业务包测试、`make check-entities`、`make check-generated` 及阶段验收 `make ci-fast`；变更权限行为、SQL 或事务时按仓库规则增加对应安全/数据库验证。浏览器 E2E 按根规则暂停。
