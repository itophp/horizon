# Application Runtime M3/M4 实施记录

日期：2026-11-05。状态：实现进行中，未达到联合验收完成标准。不得沿用 M1/M2 的通过记录作为本阶段通过证据。

## 已落地

- Runtime 2.1、部署候选目录与版本保留、生成的 deployment.json/TypeScript 摘要、离线 `./scripts/go.sh run ./cmd/runtimespec -check` 及 `-file` 贡献校验。
- 安装、应用状态、历史、运行依赖、调用依赖与任务事件迁移；正式 applications:read/manage 权限；生命周期 CAS、事务历史/Outbox、依赖与活动引用检查。
- Gateway 安装可用性检查、固定版本发现、字段读写政策、审批恢复检查；可选 Runtime 调用引用拒绝旧版本表单提交。
- 服务端 data.read/context.resolve/action.invoke，固定 Provider、查询预算与投影、记录范围复核、正文排除、受限映射和自动化子调用来源。
- 组件资源租约、释放/回滚、受控静态异步组件、Locale/Theme 资源接口；Split/Master-detail、Drawer Host、Shell 插槽、工作区和列偏好。
- 金额字符串、时间、多选、附件、数组子表、字段组、联动及远程引用搜索；定义变化保留过期表单内容并禁止提交。
- 文章/项目分析工作流复用 Eino ADK；服务端语义接入 Harness/浏览器恢复；固定工具集与授权目录取交集。
- Goa 原生 SSE 任务读取、序号重放、状态/步骤/工具调用事件；任务面板、取消能力、页面工具租约及注销。事件不存正文或工具参数。

## 十三个引擎对账

| 引擎 | 本次实现位置 | 尚需联合验收或补齐 |
|---|---|---|
| Application Registry | kernel/applicationruntime；system/applicationruntime | 多版本升级/回退、反向依赖与并发矩阵 |
| Page Runtime | features/application-runtime/page、route、page-view | 定义更新和停用后的草稿/缓存组合回归 |
| Component Registry | components、resource-adapters、contributions | 所有贡献类型的实际 Props/插槽端到端契约覆盖 |
| Layout Runtime | v2-layout、dialog-host、shell-slot、workspace | 移动布局组件测试和完整 Shell 插槽装配覆盖 |
| Data Runtime | system/applicationruntime/data、filter | 查询审计字段/耗时/数量完整化，嵌套 Reference 预算验证 |
| Form Runtime | v2-form、command-fields、advanced-controls | 子表嵌套引用、上传期间提交协调及定义变更测试 |
| Table Runtime | v2-table、table-preferences | 偏好恢复与动态权限撤销测试 |
| Action Runtime | action、runtime-tools、action.invoke | 跨渠道审批/恢复组合回归 |
| State Runtime | workspace、page-view | 工作区关闭保护、标签隔离与恢复专项 |
| Permission Runtime | Gateway access；module permission | 完整安全回归当前失败，需要修正测试身份与核查行为 |
| Runtime Context | context.resolve；harness/semantic | 撤权、版本变更、检查点恢复的组合覆盖 |
| Context Provider | Harness、Workflow、SSE、RuntimeTools | 外部模型服务未验收；Memory 未接入 |
| Extension Runtime | Plugin Registrar、contributions | 动态资源清理及多版本贡献联合测试 |

## 实际验证

- 定向 Go 测试通过：identity、applicationruntime、capability/contracts/gateway/registry、workflow、harness、runtime/app/transport。
- Runtime 前端定向：3 文件、30 项通过。
- 真实 PostgreSQL 中新增的 Runtime 生命周期、上下文脱敏/旧表单拒绝、ADK 分析/安全事件重放、SSE 账号撤权、批量操作测试通过。
- `make ci-fast` 尚未通过：第一次 lint 测试数字精度字面量已修复；第二次完整前端测试 158 通过/3 失败，涉及新增通配路由重复和可选工具调用参数，已修正断言，重跑完整前端 38 文件、161 项全部通过（`/tmp/runtime-m34-web-final.log`）；完整 ci-fast 尚需重跑。
- `make test-security` 尚未通过：专项单测已通过，集成测试中 Drive、Example 生命周期、Orchestration、Plugin permission、Project membership 和 TemporaryAccess 的部分旧身份夹具/行为断言失败。保留实时身份重建，不以关闭授权检查换取测试通过。
- `make check-rules`、`git diff --check` 通过。最终生成一致性、实体校验和构建仍需在修复后执行。

日志：`/tmp/runtime-m34-go12.log`、`/tmp/runtime-m34-rtl7.log`、`/tmp/runtime-m34-ci2.log`、`/tmp/runtime-m34-security2.log`。日志为本地临时证据，不是远程 CI 结果。

E2E 未执行（按当前指示暂停）。未推送、未发布、未触发远程 CI。Memory 留待下一轮；Phase 5 专用视图未计入完成声明。未新增依赖。
