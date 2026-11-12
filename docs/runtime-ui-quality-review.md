# Application Runtime UI 质量优化记录（2026-11-06）

## 现状与架构判断

已在浏览器登录后观察 /articles 的真实列表：标题、搜索、状态、排序、新建、列设置、四列表格、分页；当前仅两条文章，不能代表长列表与长文本验收。

- 信息架构已有列表/详情分离，符合 CRUD；不需重写文章页面或数据层。
- 搜索是无边界的原生 input，状态/排序是原生 select，列设置是 details。虽安装 shadcn，RuntimeTable 未实际采用对应原语；表单与管理页面的组件规范没有贯彻到通用渲染器。
- standalone-toolbar 的 space-between 将查询和命令均摊；缺少分组边界，输入与文字混在一起。标题与元数据等宽；字号细小、权重接近，主次不足。
- 原分页复用主按钮，禁用呈实心灰色，反而抢过内容。列设置展开会干扰工具栏布局。
- 中性配色和字体本身与后台一致，没有必要换品牌色。主要廉价感来自未完成的组合与层级；截图未表现出渐变/装饰卡片滥用，不应把通用 AI 症状冒充本页事实。

/articles 的 page.tsx 只是 ApplicationPage 入口；V2 PageView/RuntimeTable 负责解释已发布定义，TanStack Query 与 Gateway 路径已经存在。改独立文章 React 页面会绕过通用收益。本次选择代码路径，修改可信渲染器及设计指导，不改业务 Provider、能力、SQL、权限与已发布租户配置。

## 开源 Skill 调研与取舍

下列为 2026-11-06 查阅的上游来源；只借鉴方法，没有复制整套源码、安装依赖或引入额外许可负担。

| 来源 | 核心能力/约束与解决的问题 | 本项目取舍 |
|---|---|---|
| [Anthropic frontend-design](https://github.com/anthropics/skills/blob/main/skills/frontend-design/SKILL.md) | 先根据业务主题确定视觉方向、字体、布局；先计划后批评，反对无意义装饰、同构卡片和模板化文案。帮助 Agent 从组件拼装转向有意图的设计。 | 借鉴业务语义与两轮设计评审；其鼓励独特视觉身份的取向不能覆盖企业后台现有 Token，不照搬营销页 hero/字体更换建议。 |
| [Vercel web-design-guidelines](https://github.com/vercel-labs/agent-skills/blob/main/skills/web-design-guidelines/SKILL.md) | 将评审与可更新的 Web Interface Guidelines 连接，检查后给出具体文件位置；适合发现交互、焦点、表单和布局问题。 | 借鉴逐项证据和可定位修法；不是页面构思器，也不能替代业务任务分析。外部规则仅参考，仓库规则优先。 |
| [shadcn 官方 Skill](https://ui.shadcn.com/docs/skills) | 提供组件上下文、工作流和组合指导，让 Agent 了解项目组件配置与正确 API。 | 优先参考官方组合与本地库存，锁文件/Base UI 优先。解决“怎么正确组合”，不能独自决定“哪些信息最重要”。本次不升级 CLI 或安装新组件。 |
| [UI UX Pro Max](https://github.com/nextlevelbuilder/ui-ux-pro-max-skill) | 按产品类型检索设计方向、色板、字体、图表和 UX 约束，并给出栈相关建议；适合早期需求不清晰时建立候选设计系统。 | 借鉴业务类型→模式选择及反模式检查；本项目已有设计系统，不全量引入风格/字体库，也不依其检索结果改变租户配置和授权。 |

推荐组合：业务任务分析与批评方法 + shadcn 正确组合 + 证据化评审。框架性能优化与视觉设计分工，不用 React 优化规则替代设计能力。

## 根因

组件库只提供局部形态与交互，不决定页面信息顺序、密度、列宽和动作优先级。实体/Schema 的字段顺序也不等于用户决策顺序。通用 Agent 缺少语义简报时容易复用训练中的“标题+四卡片+表格”，而结构验证只能发现非法引用，无法识别视觉层级。

原 RuntimeDesigner 指令主要约束契约发现、稳定 ID、翻译与人工发布，没有要求设计简报和视觉证据。工程 Skill 是给代码 Agent 的，并不会自动注入在线模型。现有文档已经包含大量正确规范，问题同时存在于路由、执行和渲染层，继续堆超长 Prompt 无法修复原生 input/select。

## 规范与体系落地

完整可执行规范见 [runtime-ui-generation.md](runtime-ui-generation.md)，覆盖 Design/Layout/Component/Page/Interaction/Visual/Responsive/Accessibility 八项。六项设计简报是生成前产物，逐项证据表是生成后产物。

新增 baseline-ui-design 只负责设计决策和实景评审；Runtime Skill 负责路径/版本，Web Skill 负责 React 与 SDK，quality-gates 负责工程检查。通过根任务路由、agent-tooling 与 agent-policy 登记，并在 Runtime/Web Skill 链接，避免复制规范。

在线 RuntimeDesigner 增加任务/字段/动作/模式/组件/状态的简短分析约束；仍只使用原有受限工具，没有新增权限或发布工具。没有浏览器工具时必须明确视觉未验证；当前尚无自动截图打分器，也不声称模型已经具备可靠的自主视觉评审闭环。

## 修改与前后对照

| 文件/区域 | 优化前 → 优化后 |
|---|---|
| v2-table.tsx、table-select.tsx | 原生搜索和筛选 → 本地 Input/Select；保持查询值、URL、排序、分页与能力行为 |
| runtime.css | 横向均摊的散落控件 → 可换行查询组、独立命令区、结果与分页的统一边框面板；链接加强识别，分页次级化 |
| v2-table.tsx | 原生 details 列设置 → shadcn Dialog + Checkbox，保留显隐与宽度控制 |
| RuntimeDesigner Go 源及生成 workflow JSON | 只约束配置合法性 → 增加设计说明、模式选择和视觉证据边界；生成文件由 make generate-workflows 产生 |
| baseline-ui-design 及两项已有 Skill | 新增设计职责，链接到独立规范；没有把全部规则塞入模型指令 |
| AGENTS、agent-policy、agent-tooling、Runtime 开发规范 | 增加实际发现和职责路由 |

本次没有修改翻译资源，复用现有双语 key；无业务授权、事务和数据模型变化。通用列表改动会影响所有 V2 Runtime 列表，不能只凭文章两条记录证明所有应用通过。

## 验证记录与边界

- `make generate-workflows`：通过；`make check-generated`：通过，全套生成可复现，实际差异仅 designer workflow JSON。
- `pnpm typecheck`：通过；`pnpm build`：通过，有现存大 chunk 提示。
- `vitest run src/features/application-runtime/v2.test.tsx`：18 项通过；快速门禁前端套件：47 文件、189 项通过。
- `git diff --check`：通过。规则检查、format-check、lint 已通过；最终快速门禁结果另见交付汇报。
- 第一次 `make ci-fast` 的 WebMCP 测试因沙箱本地端口监听 EPERM 失败；不是断言通过。已在允许端口的环境重跑。
- 官方 `quick_validate.py` 无法运行：系统 Python 缺少 PyYAML，隔离 uv 安装因网络 DNS 不可用失败；仓库自身 Skill 元数据检查已通过，不将两者等同。
- 浏览器实际观察浅/暗列表、草稿筛选 URL 和返回记录、列设置对话框及初始焦点、390px 查询换行；已恢复默认视口与浅色。未覆盖长文本、大数据、全部键盘路径、完整业务写入和全语言矩阵。
- E2E 未执行（按当前指示暂停）；本次授权的 /articles 手工浏览器检查单独报告，没有推送或触发远程 CI。

最终 `make ci-fast` 在允许本地端口的环境退出码 0：规则、格式、lint、typecheck、测试、生成一致性、实体、workflow、前后端构建全部通过。保留构建大 chunk 与 Go sonic 回退提示；没有远程 CI 结果。
