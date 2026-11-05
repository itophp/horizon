# Application Runtime 第一阶段

实现入口：文章 `/articles`（列表）与 `/articles/:id`（详情），新增/编辑使用通用 Form 弹窗。使用原有 articles:read/write/review 权限和文章审核发布流程。拒绝后的文章先编辑保存为草稿，再重新提交。

## 定义与运行

- 业务模块在 `services/applications/articles/pages.go` 贡献 Application/Page 定义。Adapter 从原 Entity 与能力 I/O 获取字段语义、必填、长度、敏感分类与只读标记，并投影关系描述而不输出存储键。
- `services/system/applicationruntime` 提供两个只读 resolve 能力，经现有 Gateway 执行。文章是代码内置应用，不新增安装数据库或改变实体 Schema。
- `services/kernel/applicationruntime` 的 Go 模型是公共定义的事实源；`cmd/runtimespec` 生成 TS 类型及严格 Zod 结构校验，能力 Output Schema 同样从 Go 类型派生。这次没有改变传输信封，因此无需新增 Goa REST 方法。
- 前端 `features/application-runtime` 解析定义，提供字段组件注册表（版本与 Props Schema）、通用 List/Detail/Form、基础 standard 布局、Context 和错误边界。未知版本/组件/字段拒绝，错误文案本地化。
- Hybrid 操作使用可信 React action contribution；Custom 可通过编译期 `extension.render` 进入同一个 Context/错误边界。没有动态脚本执行或远程组件加载。
- Data Adapter 调用 `lib/capabilities.ts` 与生成 Gateway SDK。article.list 支持 keyword/status/page/pageSize/sort/select，固定排序字段，服务端分页和同范围 total。列表不读取正文。
- 表单只提交当前绑定能力允许的可编辑字段，系统 id/revision 从已读取记录映射；隐藏和只读字段不进入编辑输入。保存失败保留草稿与幂等键，修改载荷后使用新键。取消/关闭提示丢弃草稿，关闭浏览器有未保存提示。
- Query key 隔离当前租户/账号/部门/权限/语言及元数据版本，读取传递 AbortSignal；详情正文仅存在受保护的页面查询中，不进入 Runtime Context。

## 本地复现

按 [本地开发说明](local-development.md) 配置 `.env`、启动数据库和服务、运行迁移后，使用具备 articles:read/write 的账号打开 `/articles`：

1. 新增标题与正文，保存后列表刷新。
2. 点击标题进入详情，编辑保存后回读新 revision。
3. 以关键词/状态筛选，选择排序，翻页读取完整授权集合。
4. 草稿可进入原审核流程；非作者编辑或旧 revision 更新由原能力拒绝。

`make generate` 生成 workflow/Goa/sqlc/Orval 与 Runtime 类型；`make check-generated` 校验重复生成。新增页面字段需引用真实 Entity 与具体能力投影，不把整张表映射成表单。

## 验证与范围

新增 Go Adapter、敏感字段、版本、关系存储隐藏、元数据权限裁剪和共享定义不变性测试；RTL 覆盖动态表单、保存重试、隐藏只读、分页、详情/编辑导航及 Custom renderer；真实 PostgreSQL 测试超过 100 行的翻页、作者/租户隔离、敏感投影与非法参数。

实际门禁结果在交付回复记录。浏览器 E2E 按当前规则暂停，RTL 与数据库测试不等于真实浏览器验收。远程 CI 未触发。

完整 Action DSL、批量操作、远程关系选择、在线元数据发布、租户插件安装、Memory Center 与 AI 页面工具属于后续阶段；本阶段仅投影关系描述，不开放自动关联查询。现有前端 lint 的 RHF/项目列表 warnings 保留，不关闭检查。

## 本地验收记录（2026-11-05）

- `make ci-fast`：通过，含 35 个前端测试文件/137 个用例、Go 测试、lint/typecheck、i18n、生成一致性、实体/流程校验和前后端构建。新增 Runtime RTL 8 个用例通过。
- 使用隔离本机 PostgreSQL 执行 `make test-security`（含 `make test-integration`）：通过；其中 `TestMCPNativeClients` 按既有规则跳过，需独立 `make test-mcp-native`，本轮未执行该专项。
- `TestApplicationRuntimeArticleQueries`：真实数据库定向测试通过，覆盖 105 行数据翻页、作者与租户边界、敏感投影和非法参数。
- `git diff --check`：通过。保留 6 条既有 lint warning、实体 hint/accepted gaps 及构建分包体积提示。
- E2E 未执行（按当前指示暂停）；没有推送或触发远程 CI。本地检查不代表浏览器或原生客户端验收通过。

> 范围修正：以上是首轮文章示例验收记录，不表示当时完成了通用 Runtime。跨业务底座补齐和 Action Runtime 的交付边界见 [M1/M2 阶段说明](application-runtime-phase-2.md)。
