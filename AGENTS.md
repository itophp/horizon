# 项目长期开发规则

本仓库是通用应用工程框架。按用户当前需求持续开发功能；规则不限制开发阶段，不擅自添加未请求的行业业务或替换既有技术栈。原始需求保留在 docs/source-specification.md 供溯源；其中历史执行范围不能覆盖最新用户要求。

## 开始任务与规则发现

1. 检查工作区、Git 状态和目标模块，保留用户已有变更。
2. 修改文件前，读取根目录到目标文件父目录沿途所有 AGENTS.md；从仓库根启动时也必须主动检查目标子目录，不能假定它们全部预加载。
3. 功能开发先使用 [baseline-feature-development](.agents/skills/baseline-feature-development/SKILL.md)，按 [任务工具矩阵](docs/agent-tooling.md) 选择相关 Skill 和测试；只读/文档任务只加载相关规则。
4. 使用锁文件中的版本；升级或新增依赖先核对官方 API、兼容性与许可证，记录于 docs/versions.md / docs/decisions。不要通过安装 latest 绕过本项目已确认的兼容组合。

功能开发或需求变动时，先按 [MCP 开发 Skill](.agents/skills/baseline-mcp-development/SKILL.md) 集中询问是否接入 MCP、接入哪些操作、目标客户端与读写/确认范围；沿用已明确选择，未回答不默认开放能力，继续独立工作。已有 MCP 工具受变更影响时必须同步兼容性与授权。远程服务方案见 [实现方案](docs/mcp-service-plan.md)。

## 不可遗漏的全局边界

- 唯一 API 契约源为 services/api/design；SQL 源为 db/migrations 与 db/query。禁止手工编辑 services/api/gen、services/api/internal/repository/dbgen、apps/web/src/api/generated。修改源后 make generate、make check-generated。
- 前端使用生成 SDK，不复制 API DTO；TanStack Query 管远程状态，局部状态管理界面交互，React Hook Form + Zod 管表单，TanStack Table 管常规表格。具体选择见矩阵。
- shadcn 组件源码归项目维护，不是禁止修改的 API 生成代码；统一使用既有组件与 design tokens，修改公共组件检查全部调用方。新建、扩展或改版页面必须延续当前已确认的界面风格，按 [前端一致性规则](apps/web/AGENTS.md#界面风格一致性) 和 [设计系统](docs/design-system.md) 执行；用户最新的明确设计要求优先。
- 所有用户可见文案与辅助名称走 i18next 资源，同步 en / zh-CN，使用统一语言切换入口和 Intl 格式化；不拼句、不直接渲染翻译 HTML。
- 受保护 API 必须声明 resource/action/可信 DataScope，后端认证、Casbin 操作授权及 Repository/SQL 数据范围缺一不可；不能信任客户端租户、组织、owner 字段。新增公开端点需明确公开理由。
- 列表、详情、修改、删除、导出、批量、后台任务、聚合、关联与下载保持相同租户/范围约束；新字段必须评估后端读写投影，尤其是凭证、个人信息、金额及内部备注。
- 权限策略、身份或范围解析失败一律拒绝；禁止在不同 Handler 中暗建权限规则。敏感信息不能进入 DTO、前端日志、异常正文或审计明文。
- SQL 参数化；已应用迁移只追加版本；真实密钥不提交，环境变量文档化；测试仅连接隔离本地/CI 数据库。基本安全细则见 [security.md](docs/security.md)。
- 禁止显式 TypeScript any、关闭检查、吞异常、删除断言或伪造测试结果。Go 生成器的 any 是语言/工具输出，不用手改生成代码清除它。

- 任何认证、权限控制或数据范围行为变更，必须同步维护权限控制相关的 Skill、目录 AGENTS.md 和安全/架构规则，并补齐受影响的允许与拒绝测试；交付说明列出同步位置，不允许只改代码。

## 任务路由

| 变更 | 使用的 Skill | 规则入口 |
|---|---|---|
| 功能 MCP 工具设计、创建、变更和暴露评估 | [baseline-mcp-development](.agents/skills/baseline-mcp-development/SKILL.md) | docs/mcp-service-plan.md |
| 设置项/字典选项/上传读取下载/通知接入 | [baseline-shared-capabilities](.agents/skills/baseline-shared-capabilities/SKILL.md) | docs/shared-capabilities.md |
| 页面/表单/表格/SDK 消费 | [baseline-web-development](.agents/skills/baseline-web-development/SKILL.md) | apps/web/AGENTS.md |
| HTTP/DTO/验证/错误/安全契约 | [goa-service-designer](.agents/skills/goa-service-designer/SKILL.md) | services/api/design/AGENTS.md |
| 迁移/查询/事务/范围过滤 | [baseline-database-change](.agents/skills/baseline-database-change/SKILL.md) | services/api/db/AGENTS.md |
| 文案/语言/格式化 | [baseline-internationalization](.agents/skills/baseline-internationalization/SKILL.md) | apps/web/src/i18n/AGENTS.md |
| 认证/授权/字段/跨租户/敏感入口 | [baseline-security](.agents/skills/baseline-security/SKILL.md) | services/api/internal/AGENTS.md |
| 测试/失败修复/交付门禁 | [baseline-quality-gates](.agents/skills/baseline-quality-gates/SKILL.md) | docs/testing.md |

## 验证与交付

- 测试按变更批次集中执行，不因单个功能完成、进度汇报或聊天结束自动运行测试。普通连续开发累计受影响模块与待验项；明确阶段验收、准备提交/PR 时统一运行一次 make ci-fast，并按风险追加相关数据库/安全/浏览器检查。完整 make ci 用于发布候选、用户明确要求全量验收或 CI，不作为每个功能的收尾动作。文档/规则批次仅运行 make check-rules。高风险定向验证、结果复用和具体命令见 [testing.md](docs/testing.md#测试执行节奏)。
- 浏览器验收分档：日常改动只跑相关文件（make test-e2e-changed，或 make test-e2e-smoke 快速旁证），完整集合才用 make test-e2e / make test-e2e-parallel。只跑子集必须报告选中的文件与未覆盖集合，不能宣称全量通过；不要为每轮文件变动重跑整套浏览器测试。
- 代码生成检查与浏览器验收串行，避免生成产物更新干扰应用。不要把缺失数据库/浏览器运行时导致的跳过报告为通过。
- 读取完整失败结果，修复源代码或夹具，重跑失败集再跑模块集。同一根因连续三轮无解时停止该部分盲改，报告证据和选项；不弱化检查。
- 汇报：行为和文件变化、使用的框架/Skill、实际命令与结果、生成影响、国际化/安全影响、未执行检查和剩余风险。未经执行不得宣称通过；本地验证和远程 CI 分开报告。

## Code Review Rules

优先报告：绕过授权/租户条件、新入口漏用 scope、敏感字段泄露、生成代码手改、源契约与 SDK 漂移、硬编码文案、服务端状态重复缓存、缺失负向用例，以及被跳过却报告通过的检查。格式与静态问题由 CI 执行，不用重复的人工作业代替门禁。
