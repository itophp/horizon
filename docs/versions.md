# 版本及官方依据

核实日期：2026-10-28。从官方 npm 注册表、Go module proxy 和 Go 下载清单查询版本，选择实际运行通过的兼容组合；全部直接依赖固定版本，传递依赖由 pnpm-lock.yaml / go.sum 锁定。机器可读记录见 versions.json。

| 依赖 | 选定版本 | 官方版本元数据 |
|---|---|---|
| react | 19.3.1 | [来源](https://registry.npmjs.org/react/latest) |
| react-dom | 19.3.1 | [来源](https://registry.npmjs.org/react-dom/latest) |
| vite | 8.3.5 | [来源](https://registry.npmjs.org/vite/latest) |
| @vitejs/plugin-react | 6.1.2 | [来源](https://registry.npmjs.org/@vitejs/plugin-react/latest) |
| typescript | 6.0.2 | [来源](https://registry.npmjs.org/typescript/latest) |
| tailwindcss | 4.3.3 | [来源](https://registry.npmjs.org/tailwindcss/latest) |
| @tailwindcss/vite | 4.3.3 | [来源](https://registry.npmjs.org/@tailwindcss/vite/latest) |
| react-router | 8.5.1 | [来源](https://registry.npmjs.org/react-router/latest) |
| @tanstack/react-query | 5.105.0 | [来源](https://registry.npmjs.org/@tanstack/react-query/latest) |
| @tanstack/react-table | 9.3.1 | [来源](https://registry.npmjs.org/@tanstack/react-table/latest) |
| react-hook-form | 7.91.0 | [来源](https://registry.npmjs.org/react-hook-form/latest) |
| @hookform/resolvers | 6.0.2 | [来源](https://registry.npmjs.org/@hookform/resolvers/latest) |
| zod | 4.6.7 | [来源](https://registry.npmjs.org/zod/latest) |
| i18next | 26.5.11 | [来源](https://registry.npmjs.org/i18next/latest) |
| react-i18next | 17.1.9 | [来源](https://registry.npmjs.org/react-i18next/latest) |
| lucide-react | 1.63.0 | [来源](https://registry.npmjs.org/lucide-react/latest) |
| vitest | 5.0.5 | [来源](https://registry.npmjs.org/vitest/latest) |
| @playwright/test | 1.64.1 | [来源](https://registry.npmjs.org/@playwright/test/latest) |
| @testing-library/react | 16.3.3 | [来源](https://registry.npmjs.org/@testing-library/react/latest) |
| @testing-library/user-event | 14.6.8 | [来源](https://registry.npmjs.org/@testing-library/user-event/latest) |
| @testing-library/jest-dom | 7.0.1 | [来源](https://registry.npmjs.org/@testing-library/jest-dom/latest) |
| jsdom | 30.1.3 | [来源](https://registry.npmjs.org/jsdom/latest) |
| eslint | 10.13.0 | [来源](https://registry.npmjs.org/eslint/latest) |
| @eslint/js | 10.0.1 | [来源](https://registry.npmjs.org/@eslint/js/latest) |
| typescript-eslint | 8.72.1 | [来源](https://registry.npmjs.org/typescript-eslint/latest) |
| eslint-plugin-react-hooks | 7.1.1 | [来源](https://registry.npmjs.org/eslint-plugin-react-hooks/latest) |
| eslint-plugin-react-refresh | 0.5.7 | [来源](https://registry.npmjs.org/eslint-plugin-react-refresh/latest) |
| eslint-plugin-jsx-a11y | 6.10.2 | [来源](https://registry.npmjs.org/eslint-plugin-jsx-a11y/latest) |
| prettier | 3.9.10 | [来源](https://registry.npmjs.org/prettier/latest) |
| orval | 8.45.0 | [来源](https://registry.npmjs.org/orval/latest) |
| shadcn | 4.23.0 | [来源](https://registry.npmjs.org/shadcn/latest) |
| clsx | 2.1.1 | [来源](https://registry.npmjs.org/clsx/latest) |
| tailwind-merge | 3.7.0 | [来源](https://registry.npmjs.org/tailwind-merge/latest) |
| class-variance-authority | 0.7.1 | [来源](https://registry.npmjs.org/class-variance-authority/latest) |
| @types/node | 26.6.5 | [来源](https://registry.npmjs.org/@types/node/latest) |
| @types/react | 19.3.0 | [来源](https://registry.npmjs.org/@types/react/latest) |
| @types/react-dom | 19.3.0 | [来源](https://registry.npmjs.org/@types/react-dom/latest) |
| globals | 17.13.0 | [来源](https://registry.npmjs.org/globals/latest) |
| goa.design/goa/v3 | v3.35.0 | [来源](https://proxy.golang.org/goa.design/goa/v3/@latest) |
| github.com/jackc/pgx/v5 | v5.11.0 | [来源](https://proxy.golang.org/github.com/jackc/pgx/v5/@latest) |
| github.com/sqlc-dev/sqlc | v1.31.1 | [来源](https://proxy.golang.org/github.com/sqlc-dev/sqlc/@latest) |
| github.com/pressly/goose/v3 | v3.29.0 | [来源](https://proxy.golang.org/github.com/pressly/goose/v3/@latest) |
| github.com/casbin/casbin/v2 | v2.135.0 | [来源](https://proxy.golang.org/github.com/casbin/casbin/v2/@latest) |
| go | go1.27.3 | [来源](https://go.dev/dl/?mode=json) |
| @base-ui/react | 1.9.0 | [来源](https://registry.npmjs.org/@base-ui/react/1.9.0) |
| @fontsource-variable/geist | 5.3.0 | [来源](https://registry.npmjs.org/@fontsource-variable/geist/5.3.0) |
| cn | 0.5.0 | [来源](https://registry.npmjs.org/cn/0.5.0) |
| tw-animate-css | 1.4.0 | [来源](https://registry.npmjs.org/tw-animate-css/1.4.0) |
| @mdxeditor/editor | 4.3.4 | [来源](https://registry.npmjs.org/@mdxeditor/editor/4.3.4) |

PostgreSQL 18.6，官方 release notes 确认为稳定主线 18；容器实际版本另见验收。pnpm 使用当前机器已有的 10.14.0，packageManager 和 CI 均固定一致；Node 使用实际安装且满足所有 engines 的 22.23.2。不是盲目使用每个包的 latest：TypeScript 7 因 ESLint peer 约束固定到 6.0.2，理由见 ADR。

官方使用文档：

- [React](https://react.dev/)
- [Vite](https://vite.dev/guide/)
- [TypeScript](https://www.typescriptlang.org/docs/)
- [shadcn/ui](https://ui.shadcn.com/docs/installation/vite)
- [Tailwind CSS](https://tailwindcss.com/docs/installation/using-vite)
- [React Router](https://reactrouter.com/start/declarative/installation)
- [TanStack Query](https://tanstack.com/query/latest/docs/framework/react/overview)
- [TanStack Table](https://tanstack.com/table/latest/docs/framework/react/quick-start)
- [React Hook Form](https://react-hook-form.com/get-started)
- [Zod](https://zod.dev/)
- [i18next](https://react.i18next.com/latest/using-with-hooks)
- [Go](https://go.dev/dl/)
- [Goa](https://goa.design/docs/1-goa/quickstart/)
- [pgx](https://github.com/jackc/pgx)
- [sqlc](https://docs.sqlc.dev/en/stable/overview/install.html)
- [Goose](https://pressly.github.io/goose/installation/)
- [Casbin](https://casbin.org/docs/rbac-with-domains/)
- [Orval](https://orval.dev/docs/reference/configuration/output/)
- [Vitest](https://vitest.dev/guide/)
- [Playwright](https://playwright.dev/docs/test-webserver)
- [PostgreSQL](https://www.postgresql.org/docs/current/release.html)

Goa 生成器通过 go.mod tool 指令与框架共享版本；sqlc CLI 用 make tools 固定版本。shadcn CLI 新增依赖也在初始化后改为精确版本。官方文档会随时间更新，以锁定版本及可执行验证结果为准。

Goa 官方 Skill 完整安装，内容 hash 见 skills-lock.json。当前模块的生成器 CLI 和服务依赖共用 go.mod tool 固定版本，安装 skill 不修改工具链版本。

身份管理新增 golang.org/x/crypto v0.58.0（BSD-3-Clause），使用 [Argon2id 官方接口](https://pkg.go.dev/golang.org/x/crypto/argon2)；实际版本锁定在 go.mod/go.sum，未加入浏览器密码库。

2026-10-29：系统治理沿用锁定依赖，不新增运行时包。加密、SMTP、受限文件系统使用锁定 Go 标准库 AES-GCM/net/smtp/os.Root。Orval 8.45.0 的 input transformer 按 Goa JSON bytes 实际 base64 编码将 format:binary 字段映射为 string，避免 Blob 被 JSON 序列化为空对象；适配源在 apps/web/orval.config.ts，生成物仍禁止手改。

2026-10-29 通知 Markdown：新增固定 `react-markdown@10.1.0`、`remark-gfm@4.0.1`（MIT），锁于 package.json/pnpm-lock.yaml。已核对 [react-markdown 官方用法/安全/React 兼容性](https://github.com/remarkjs/react-markdown)、[10.1.0 发布](https://github.com/remarkjs/react-markdown/releases) 与 [remark-gfm 官方 API/许可证](https://github.com/remarkjs/remark-gfm)。使用 React 组件输出和 GFM 插件；skipHtml、不安装 raw/MDX 执行插件，保留 defaultUrlTransform，仅对绑定的 file:ID 引用做受控扩展。

2026-10-29 WebMCP：固定 `@opentiny/next-sdk@0.4.11`、`@opentiny/agent@0.3.5`、`@modelcontextprotocol/sdk@1.25.3`、`express@5.3.0`（均 MIT）。核对 npm 官方元数据、已发布类型声明、官方代理路由和实际 Streamable HTTP 实现；Node 22.23.2 满足 Express >=18 / agent >=20，SDK 与应用 React 解耦。SDK 内部 Zod 3 与应用 Zod 4 独立锁定，不传递跨版本 schema，页面 JSON Schema 从应用 Zod 4 生成。使用 SDK core 动态导入，不启用额外模型/聊天依赖消费者；传递依赖由 pnpm-lock.yaml 固定。架构与安全边界见 [ADR 009](decisions/009-webmcp.md)。

0.4.11 的实际兼容适配：AbortSignal 撤销异步注册；精确来源 CORS 允许 SDK 默认 credentials:include / Stream-Session-Id，但代理删除 Cookie；stdio 适配器按 MCP schema 解包 DOMString 结果，保证 isError 不被文本包装丢失。未修改第三方包或生成物。

2026-10-29 系统设置排期未新增依赖：使用现有 Go time、pgx 事务/savepoint、PostgreSQL 行锁和生成 SDK；时间录入用 HTML datetime-local + Date/Intl，未引入额外日期或任务调度框架。

远程业务 MCP 开发依赖：github.com/modelcontextprotocol/go-sdk v1.8.0，固定于 go.mod/go.sum；使用 Streamable HTTP Stateless 适配器。上游 LICENSE 含 Apache-2.0/MIT 历史贡献声明，发布前须完成客户端协议兼容验收。

远程 MCP 兼容性核对：SDK tag v1.8.0 声明最低 Go 1.25.0，项目 Go 1.27.3 编译/测试通过。以下新增传递依赖的许可证已核对下载模块自带 LICENSE：

| 模块 | 固定版本 | 许可证 |
|---|---|---|
| google/jsonschema-go | v0.4.3 | MIT |
| segmentio/encoding | v0.5.4 | MIT |
| segmentio/asm | v1.2.1 | MIT-0 |
| yosida95/uritemplate/v3 | v3.0.2 | BSD-3-Clause |
| golang.org/x/oauth2 | v0.35.0 | BSD-3-Clause |
| golang.org/x/time | v0.15.0 | BSD-3-Clause |
| golang.org/x/sys | v0.49.0 | BSD-3-Clause |

SDK 与 x/time 是应用直接依赖，其余由 SDK 引入；全部由 go.mod/go.sum 固定，无 latest 安装。客户端验收使用本机 Codex CLI 与腾讯 WorkBuddy 5.8.2 随附运行时，具体边界见 MCP 验证记录。

本轮原生 Codex 验收版本为 codex-cli 0.165.0-alpha.1.2；浏览器/stdio 桥继续使用锁定的 @modelcontextprotocol/sdk 1.25.3，没有新增前端 MCP 依赖。

## 2026-10-30 通知正文所见即所得编辑器

固定 `@mdxeditor/editor@4.3.4`（MIT；peerDependencies 为 `react >= 18 || >= 19`，与本项目 React 19.3.1 兼容）。已核对 [npm 版本元数据](https://registry.npmjs.org/@mdxeditor/editor/4.3.4)、[官方 Vite 接入说明](https://mdxeditor.dev/editor/docs/getting-started) 和 4.3.4 产物类型声明（`MDXEditor`、`MDXEditorProps`、`toolbarPlugin`、`Translation`、`suppressHtmlProcessing`、`overlayContainer`）。

- 只以插件形式启用标题（1–3 级）、列表、引用、分隔线、链接与表格，工具栏按需裁剪；不安装 image、code-block、diff-source 插件，因此编辑器内没有“上传并插入文件”和源码/差异/预览浏览入口，编辑区即所见即所得结果。
- `suppressHtmlProcessing` 关闭原始 HTML 处理，`linkPlugin.validateUrl` 只放行 http(s)/mailto，与 NotificationMarkdown 的 URL 策略一致。
- 样式按后台 tokens 覆盖（`--base*` / `--accent*` 映射到 `--app-*`），浅暗主题共用；不引入第二套视觉体系，也不改第三方包源码。
- 传递依赖（lexical 0.48、codemirror 6、radix 原语等）由 pnpm-lock.yaml 固定，未使用 latest 安装，未修改 `services/api/gen` 与 `apps/web/src/api/generated`。
