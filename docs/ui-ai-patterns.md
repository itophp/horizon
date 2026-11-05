# AI 工作台与节点的组件参考

## 官方来源与采用方式

AI 页面开发除通用 [组件指南](ui-component-guidelines.md) 外，必须先查 [shadcn/ui AI Elements](https://ui.shadcn.com/docs/ai-elements) 和对应 Base UI 模式源码。2026-11-04 核对：该入口把 AI 界面作为 shadcn/ui 组件的组合模式提供，不是要求安装一个统一 AI Elements 包；其说明也区分了旧 AI SDK Elements。不要混用两套来源、包名和 API。

按模式列出的依赖核对本地组件，使用项目锁定 CLI 按需引入，再提取官方组合到实际 feature/可复用组件。官方示例中的 Next.js、AI SDK transport 和模拟对话不是本仓库运行时；不能为了采用 UI 模式替换既有 Eino、生成 SDK、Gateway 或服务端事件。

## AI 工作台的选型映射

| 本仓库界面职责 | 优先核对的官方模式/组件 | 接入要求 |
| --- | --- | --- |
| 工作台对话区域 | [Chat](https://ui.shadcn.com/docs/ai-elements/base/chat)、Message、Message Scroller、Bubble | 现有会话 ID、恢复/归档和权限继续来自服务端；不建立第二套本地对话事实 |
| 输入、发送、停止 | [Prompt Input](https://ui.shadcn.com/docs/ai-elements/base/prompt-input)、Input Group、Attachment | 输入法组合期间不意外提交；发送/停止对接既有请求；附件仍经共享文件权限路径 |
| 执行步骤与工具调用 | [Tool](https://ui.shadcn.com/docs/ai-elements/base/tool)、Task、Trace、Code Block | 运行状态来自真实调用记录，输入输出仅显示授权脱敏投影，不能默认展开原始参数 |
| 人工确认界面 | [Confirmation](https://ui.shadcn.com/docs/ai-elements/base/confirmation) | 只承担表现与点击动作，继续调用现有审批契约；不把前端确认视为已授权执行 |
| 计划与任务进度 | Plan、Queue | 仅在有对应计划/任务数据时采用；模型建议、后端已注册流程与已执行结果分别标注 |
| 子 Agent 执行 | Subagent | 需要真实子任务标识、生命周期与可见范围；卡片存在不证明后端已启动子 Agent |
| 回答来源 | Sources、Inline Citation、Hover Card | 只接受可信来源投影与安全 URL；来源打不开/无权时明确反馈 |
| 上下文使用、模型选择 | Context、Model Selector | 后端真实提供且允许调整才显示，不伪造 token 百分比或模型选项 |
| 可解释执行摘要 | Chain of Thought、Reasoning | 只呈现服务端允许公开的摘要/阶段，不要求或展示内部隐含思维链 |
| 回退点 | Checkpoint | 只有后端支持恢复语义才提供；回看对话不等于撤销已成功的业务写入 |
| 命令与诊断 | Terminal、Test Results、Stack Trace、File Tree | 只在有相应授权工具结果时采用；用户页用安全错误摘要，不显示原始堆栈/密钥 |
| 推荐下一步 | Suggestion、Questionnaire | 建议点击后仍经过输入/确认与完整 Gateway；不能将模型建议自动当成执行授权 |

从 AI Elements 总入口导航到对应模式后再核对完整 API；表中的显示名不等于本地已存在的文件名和导出。当前工作台已引入 Message、Bubble、Message Scroller、Trace、Collapsible、Input Group、Textarea、Empty、Skeleton，并组合现有 Card/Badge/Button/Select。未引入附件、模型选择、计划、子 Agent 等没有本次数据接入需求的模式。

## 当前表现层实现

### 流式与聊天生命周期

用户消息之后立即建立对应的助手消息占位；流式执行阶段、工具调用链与加载提示必须归入这一轮助手消息，禁止作为整个消息列表的前置或尾随区域。事件按回复标识绑定，后续提问不能清空或挪用前一轮的流程。完成后在同一位置展示服务端最新调用轨迹，再显示回答和完成状态，避免继续呈现过期的确认状态。

工作台通过 Goa 定义的 `POST /api/ai/runs/stream` 接收 SSE；原 JSON 任务接口保持兼容。事件按 sequence 排序，覆盖 started、planning、tool_start、tool_end、answer_delta、completed/failed。Eino ADK 使用实际模型流，前端按增量更新；阶段和调用链只显示公开状态及能力名，不投影模型隐含推理、原始工具输入或输出。SSE 客户端复用公共请求层的 Cookie、CSRF、超时与错误映射，URL/DTO 来自生成 SDK；不自动重连或重试任务写入。

已打开聊天的运行组件在 AI 页内保持挂载，切换聊天只切换可见区域，不中断旧请求；首次发送即把当前会话加入列表并标识执行中。主动停止、离开 AI 页面或身份作用域变化仍会取消页面等待；本次不提供浏览器关闭或服务重启后的后台任务接续保证。

审批完成后重新查询运行详情、历史和审批队列，按最新调用状态展示。服务端使用受信任租户参数补齐已授权的运行读模型，并关联业务调用；所有调用已终态时，等待确认的 AI 展示状态解除，业务失败仍单独保留。原模型回复作为历史文本保留，不改写成新的模型结论。

`use-workbench.ts` 保留既有会话、运行与审批查询/操作，`workbench.tsx` 组合界面，`chat-feed.tsx` 使用官方消息与滚动原语，`prompt-input.tsx` 按 Prompt Input 模式组合 Input Group，`shared.tsx` 的 RunTrace 使用 Trace/Collapsible、ApprovalCard 使用 Card/Badge/Button 保留原审批回调。会话列表复用 Button/Input/Select，加载和空状态分别使用 Skeleton/Empty。停止等待只中止页面请求，明确提示服务端任务可能继续，不宣称撤销业务。切换/卸载时取消等待并忽略迟到的响应；输入法组合期间 Enter 不发送。

架构映射：本次是表现层重组，继续直接复用已有 SDK、后端运行生命周期、权限及审批；没有新增能力、实体、事务、Outbox、SQL 或生成契约。新增依赖与源码来源记录在 [版本说明](versions.md)。

## AI 执行步骤与流程画布节点分开选型

- 对话中的工具步骤是一条执行轨迹，优先采用 Tool/Task/Trace；展示“发生了什么”。
- 可连线的 Workflow 节点属于图编辑器。官方当前总入口将旧 canvas/node/edge 等元素归为图引擎搭配 Card、Button Group 的组合，不提供可直接替换一切的统一节点库。
- 本仓库 `features/orchestration/graph.tsx` 已用 X6、x6-react-shape 和 Dagre。节点内容优先复用 shadcn 组件；端口、边、布局、平移缩放仍由现有引擎处理。不能仅因官方示例采用 React Flow 就并行引入第二套图引擎。
- [WorkflowNodeCard 代码参考](code-reference/workflow-node.tsx) 展示节点内容如何用现有 Card、Badge、Button 组合。它不是官方 AI Elements Node，不包含图引擎、流程 DSL、节点执行或持久化功能。
- 节点状态必须按具体 run/step 的服务端事实绑定；只读编译图和本地未发布草稿不能标成已保存/已执行。点击属性与运行操作分开，运行仍执行已注册版本。

## 对现有代码的采用位置

| 文件 | 评估替换/组合的表现层 | 必须保留 |
| --- | --- | --- |
| `apps/web/src/features/ai/workbench.tsx` | Chat、Prompt Input、Suggestion | 会话恢复、生成 SDK、取消、身份切换、错误和请求生命周期 |
| `apps/web/src/features/ai/shared.tsx` | RunTrace/ApprovalCard 的 Trace、Tool、Confirmation 表现 | 服务端真实状态、脱敏投影、审批权限与冲突处理 |
| `apps/web/src/features/ai/tasks.tsx` | Item/Queue 式列表与空/加载状态 | 真实会话的选择、重命名、归档，不能混淆会话列表和 Agent 待办 |
| `apps/web/src/features/orchestration/graph.tsx` | 节点卡片和查看属性动作 | X6 注册/端口/边、编译投影、节点语义及本地草稿限制 |

实际迁移前先列出已有交互与官方候选，检查依赖、许可证、Base UI 和主题，再实施局部替换。不能复制官方演示里的模拟回复、脚本化工具结果或 Next.js API 端点进入生产逻辑。

## 验收重点

聊天滚动遵循“在实时边缘才跟随”的策略，用户回看历史时不强制跳到底；提供回到最新消息入口。身份/部门变化取消旧请求并清理私有会话内容，旧响应不得追加到新身份。发送、取消、重试和重新打开会话均核对真实运行 ID，不能重复执行已成功能力。

测试覆盖消息顺序、滚动位置、输入法提交、请求取消/乱序、工具失败、审批冲突、权限失效、敏感字段、窄屏与键盘。普通 AI 状态和 `ai_runs` 生命周期不能改写已成功的 `cap_invocations` 业务事实。显示层复用不替代后端授权或状态验证。
