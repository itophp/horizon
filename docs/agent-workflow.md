# Agent 长期开发工作流

根 AGENTS.md 是入口；任务路由见 [agent-tooling.md](agent-tooling.md)，可执行门禁见 [testing.md](testing.md)。原始规范是需求来源档案，当前用户要求及版本化的 AGENTS / Skill / CI 是持续开发依据。

## 一次功能任务

1. 读取当前 Git 变更、目标路径沿途 AGENTS；明确验收行为、涉及层及公开/权限边界。
2. 用 baseline-feature-development 选择所需 Skill，不把所有规则整份塞进上下文。
3. 阅读已有契约/SQL/实现/组件；选择矩阵中的工具，避免重复状态、DTO 和同职能框架。
4. 修改源 DSL / SQL / 配置，再生成，更新非生成实现和 SDK 消费者；仅改界面时不制造无关迁移。
5. 按风险补充本次行为的测试，普通连续开发累计待验模块，不在每个功能完成时执行测试；高风险改动或失败诊断先做最小定向验证。明确阶段验收/提交/PR 前按 testing.md 集中验证一次并复用有效结果，发布候选/明确全量验收/CI 才做完整回归。
6. 检查源与生成差异，报告真实命令/结果、兼容性、国际化、安全与未执行项。

API 工作流：design → Goa/OpenAPI → 非生成服务/传输 → Orval → UI。数据库工作流：migration/query → sqlc → Repository/事务 → 隔离 PostgreSQL。界面工作流：组件/tokens → 所有语言资源 → 交互/类型/构建 → 需要的真实浏览器流程。

## 项目 Skill 与规则维护

六个项目 Skill 位于 .agents/skills，另保留完整官方 goa-service-designer。元数据负责触发，正文指导流程；更细约束位于对应子目录 AGENTS，背景原理位于 docs。新增 Skill 应描述具体触发场景和不应激活的边界，避免为每个依赖复制手册。

官方 Goa Skill 通过 `npx --yes skills add goadesign/goa --skill goa-service-designer -a codex --yes --copy` 安装，来源/hash 在 skills-lock.json；项目自产 Skill 由 Git 跟踪，不伪造成上游安装项。官方指南：https://goa.design/docs/ai-development/ 。生成器依赖仍以 go.mod tool 固定版本，升级 Skill 不隐式升级服务依赖。

维护 Skill 时使用可用的 skill-creator 和官方 quick_validate.py 进行作者校验；仓库 CI 自带 check-rules，不依赖作者机器上的全局脚本。新/移动规则同步 agent-policy.json 与任务矩阵。不要直接改上游 Skill 来隐藏项目约定，把本地约定放自己的 AGENTS/Skill。

规则不自动授予部署、生产数据操作、凭证或外部消息权限；按用户授权范围持续完成工作。出现局部不明确时先推进不依赖它的部分，不把规则文档变成重复确认流程。

## 失败与可信汇报

读取完整错误，修手写源/夹具，生成问题回源契约或生成器配置。相同根因最多连续三轮盲修，之后说明证据与选项；不得跳过/删除/弱化检查。同源权限和 SQL 隔离测试是实际安全证据，前端按钮隐藏不是。

生成和浏览器验收串行，避免 Vite 热更新干扰。普通 Playwright 测试资产与 CI 独立于 MCP；失败保留退出码、screenshot、Trace 和 console/network。已配置、本地通过、远程 CI 通过分开报告。
