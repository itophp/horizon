# shadcn/ui 组合与交互模式

本文是 [组件使用指南](ui-component-guidelines.md) 的配套规范，定义场景、组件组合、状态与验收；不是新增组件的安装清单。缺失原语按主指南的官方检索、锁定 CLI、依赖与 Base UI 兼容性流程引入。代码参考与模板范围见 [页面模板](page-templates.md)，不能把本文方案当作已安装或已验证的代码。

## 场景优先的选择顺序

先回答“用户要导航、选择、执行、查看还是等待”，再选组件。最大化复用指优先使用组件已有的语义、键盘、焦点、浮层、状态与组合能力；不是最大化一个页面里的组件数量。

| 用户意图 | 首选 | 容易混淆的替代项 |
| --- | --- | --- |
| 去另一个页面 | Router Link + NavigationMenu/Sidebar/Breadcrumb | DropdownMenu 主要承载命令；Tabs 主要切换同一上下文的内容 |
| 换一页数据 | Pagination + URL + 数据查询 | Pagination 不负责自动取数，也不能修复后端缺失的分页 |
| 选择一个业务值 | RadioGroup / Select / Combobox | ToggleGroup 不默认取代表单字段，菜单不能冒充选择器 |
| 切换工具状态 | Toggle / ToggleGroup | ButtonGroup 只组织按钮，不自动拥有单选/多选状态 |
| 做一个命令 | Button / DropdownMenu / Command | 内容导航与业务执行保留不同语义 |
| 看一小段说明 | Tooltip | 内容摘要用 HoverCard；交互表单用 Popover |
| 看链接对象摘要 | HoverCard + 可访问详情链接 | 不能把必要信息与动作仅藏在悬停里 |
| 局部调整/筛选 | Popover | 复杂录入用 Dialog/独立页，桌面侧向辅助区用 Sheet |
| 聚焦填写、完成或取消 | Dialog | 危险确认用 AlertDialog；长任务改用页面 |
| 等待首次数据 | Skeleton；小型操作用 Spinner | 未知时长不用虚假 Progress，加载中不能直接显示 Empty |
| 没有可展示记录 | Empty | 失败用 Alert，无权限用权限状态；不是所有无内容都叫空数据 |
| 获得短反馈 | Toast | 必须持续处理的失败、任务进度和字段错误不能只靠 Toast |
| 文本和布局方向 | DirectionProvider + document.dir | 不是翻译、语言加载或 locale 格式化工具 |

## Pagination：分页是数据与导航契约

适用：有界页码、记录查找、可以分享的列表位置。信息流或游标型目录通常使用“上一批/下一批”或加载更多；没有总数就不编造页数。

- **组合**：Pagination → PaginationContent → PaginationItem → PaginationPrevious / PaginationLink / PaginationEllipsis / PaginationNext；每页数量用 Select，范围摘要用本地化文本。具体导出及 Router Link 组合按引入源码核对。
- **导航**：能用 URL 表达时提供真实目标地址，保留筛选与排序，标明当前页 `aria-current="page"`。不能用 `href="#"` 或 clickable div 代替翻页。
- **状态**：统一 URL 从 1 起的页面语义；适配 TanStack Table 的索引时明确换算。页码、每页数量、筛选进入 Query key；页大小和筛选改变回第一页，避免两次状态更新分别发出无效查询。
- **边界**：首页无上一页、末页无下一页；没有数据时无可用翻页。禁用链接要阻止激活与错误导航，不能只降低透明度。页数较多用省略号，不能列出数百个按钮。
- **远程数据**：排序、筛选和分页属于同一后端数据范围；不能拿服务器一页数据再算“总计”。后台刷新可保留旧表并标记加载，新的失败提示与旧数据并存；避免用 Skeleton 闪烁覆盖。
- **删除与并发**：当前末页删空时定位到有效页；接口总数变化后校准页码，避免无限重定向/请求。跨页批量选择必须说明只选本页还是全部命中项。
- **验收**：URL 直达、浏览器前后退、保留筛选、改变页大小、空末页、慢响应、窄屏与当前页播报。不能只断言页码按钮存在。

## Navigation Menu：页面导航，不是命令菜单

适用：门户/产品站横向导航、多组目的链接。后台主导航继续遵循现有侧栏与菜单架构；简单侧栏不为“多用组件”强制改成横向 NavigationMenu。

- **组合**：NavigationMenu + List + Item + Trigger/Content + Link。直接目的地用 Link；需要展开一组目的地才用 Trigger。保留真实地址与 Router 语义。
- **层级**：用 Sidebar 管应用侧栏，Breadcrumb 管当前位置，Tabs 管当前对象的子视图，NavigationMenu 管一组页面入口；不同组件不复制同一层导航。
- **交互**：支持键盘打开、移动、关闭及回到触发项；活动路由有可见标记与辅助语义，不依赖 hover。移动端用 Sheet/折叠导航保留相同信息架构。
- **授权**：菜单只显示可发现入口；路由与服务端继续授权。折叠菜单不泄露未授权的对象计数或名称。
- **验收**：直接访问、刷新活动项、打开新标签、键盘、窄屏入口、无权限目的地与长翻译。禁止用命令菜单模拟整套网站导航。

## Popover：有锚点的小型交互

适用：少量筛选条件、日期范围、紧凑属性设置。它与触发项有明确空间关系；长表单、大量说明、多步任务换成 Dialog/页面。

- **组合**：Popover + Trigger + Content，按当前 Base UI 文档补 Header/Title/Description；触发用 Button 的 `render` 组合能力，避免 button 嵌套 button。旧源码没有相应导出时先核对版本，不凭名称编造 API。
- **应用方式**：筛选明确选择“即时生效”或“草稿 + 应用/重置”。即时模式每次合法变更更新 URL；草稿模式取消不提交，重新打开从已应用值初始化。
- **焦点**：打开时进入合理控件，关闭后返回触发项；Escape/外部点击遵循原语与任务语义。宽高限制在可用视口内，溢出在面板内部处理。
- **组合浮层**：Dialog 内日期选择和下拉选择复用原语的 portal、焦点与层叠机制，不另写全局点击关闭处理，也不用任意高 z-index 掩盖问题。
- **验收**：应用/取消区别、重复打开、外部点击、键盘、靠近视口边缘、Dialog 内 Select/Calendar 和窄屏布局。

## Radio Group：把重要的互斥选择直接呈现

适用：少量、文案可比较、用户应理解差异的选项，例如权限范围、交付方式、显示密度。选项多且稳定用 Select，需搜索用 Combobox，独立多选用 Checkbox。

- **组合**：Field/FieldSet 的组名称与说明 + RadioGroup + RadioGroupItem + 每项 Label；有成本或后果差异时每项附说明。卡片式选项也保留 RadioGroupItem，不能用 Card 点击和颜色假装 radio。
- **状态**：RHF Controller 控制稳定 value，不能存已翻译的显示文本。必填但无合理默认时允许未选择并显示错误；有合理默认时才预选，不能默默选择最宽权限。
- **交互**：点击标签可选，方向键在组内移动，Tab 在合理的组边界移动。选择不自动提交高风险业务，也不意外跳转页面。
- **验收**：组名、每项名称/说明、选中播报、方向键、未选提交、禁用选项、长说明与动态切语言。ToggleGroup 适合工具栏视图状态，不替代表单 radio 的必填语义。

## Toast：短反馈与可靠结果分离

适用：保存成功、复制完成、非阻塞失败等简短消息。按官方当前 Base UI Toast 或兼容的 Sonner 路径选一种项目实现；不能把所有 Toast 示例都默认视为同一套 API。Toaster 在应用 Provider 层统一挂载，页面不各建实例。

- **成功时点**：只有真实业务成功才能显示成功；后台任务受理显示“已提交”，结果在任务记录中持续展示。失败不得先展示成功再悄悄回滚。
- **交互**：可关闭，显示时间考虑阅读长度，悬停/焦点暂停等使用所选组件能力；重要动作提供可恢复入口，不能要求用户在几秒内点到唯一入口。
- **去重**：同一操作用稳定 toast ID 更新 loading → success/error，避免 Query 重试或组件重渲染重复轰炸。加载提示及时结算，不留永远旋转的消息。
- **错误边界**：字段错误留在 FieldError，无法继续的页面错误用 Alert，长期任务用任务面板/通知中心；Toast 不展示后端堆栈、凭据或敏感正文。
- **撤销**：只有后端支持真实逆操作/撤销能力才提供 Undo，走相同 Gateway/权限/幂等；关闭提示不能假装撤销数据。
- **无障碍**：沿用 live region，不每条都使用强制打断的 alert；显示位置避开表单操作区、页脚、移动键盘。验收重复提交、失败、重试、计时关闭、焦点与主题。

## Hover Card：非必要的对象预览

适用：链接背后的人员、组织、项目等摘要；如 Avatar + 姓名 + 归属 + 简短统计。短图标说明用 Tooltip，需输入或处理动作的面板用 Popover/Dialog。

- **组合**：HoverCard + Trigger + Content，触发项保留可正常打开的详情链接；Card 内的摘要不改变链接原本语义。预览数据也执行服务端字段与范围授权。
- **取数**：按展开需要查询、缓存授权摘要；悬停不拉取整份详情/敏感正文。防抖/延迟避免指针经过一列就触发大量请求；离开后取消或忽略无关结果。
- **回退**：没有 hover 的触屏和辅助技术仍能从详情链接读到所需内容；不能仅凭 HoverCard 验收键盘与屏幕阅读器可用性。
- **验收**：指针从触发项移入卡片不意外消失，快速移过多项、加载与失败回退、链接直达、无预览权限、触屏路径。关键操作不能依赖预览展开。

## Dialog、Alert Dialog、Sheet、Drawer：聚焦程度决定容器

| 容器 | 典型任务 | 项目默认选择 |
| --- | --- | --- |
| Dialog / EditorDialog | 短表单、局部详情、选择对象 | 普通后台短任务复用 EditorDialog；内容较长改用独立页 |
| AlertDialog | 删除、不可恢复动作、需要明确决定的风险提示 | 明确对象与后果，取消与具体动作并列；不把所有提示都升级为警报 |
| Sheet | 侧向预览、辅助编辑、窄屏筛选/导航 | 保留上下文；模态或非模态按实际原语能力与任务选择 |
| Drawer | 确有移动底部抽屉需求 | 核对实现与附带依赖，不把普通侧面板随意称为 Drawer |

- **结构**：保留 Title、必要 Description、主体与 Footer；主要动作在固定一致的位置。短任务建议窄/普通宽度（约 400/560px），宽表单约 720px 需有内容理由；以项目公共样式为实现源，不在每页复制尺寸。
- **初始焦点**：短输入任务可落首字段，长说明先落标题/可阅读内容；危险确认不默认聚焦破坏性动作。关闭后回触发项，触发项已消失时回到合理的列表位置。
- **提交**：等待真实成功后关闭；失败保留输入和错误。pending 限制重复提交，若不允许关闭必须给出进度/原因；允许取消请求不代表业务事务一定撤销，结果仍需核实。
- **脏数据**：Escape、关闭按钮、背景点击、路由跳转的丢弃策略一致。需要确认时明确继续编辑或放弃，不为无修改内容弹确认。
- **滚动**：限制在可用视口，头/尾与内容滚动职责明确；移动键盘不挡提交。不要通过隐藏关闭按钮或取消 Escape 弥补焦点错误。
- **层叠**：尽量一层业务 Dialog，局部 Select/Popover 使用原语允许的组合。多步业务进程用向导/独立页，不连环弹窗。
- **验收**：打开/关闭/取消/成功/失败、Tab 焦点约束、焦点恢复、脏数据、多语言、窄屏键盘、嵌套选择器；不能只检验遮罩与按钮存在。

## Direction：方向上下文，不是翻译服务

项目当前 en、zh-CN 均为 LTR，不为展示组件数量引入 RTL。真正增加 RTL 语言时，在应用根按语言设置 DirectionProvider 的 ltr/rtl，同时同步 document.dir 和语言属性；按 Base UI 实现核对 portal 中的方向继承。

- 布局优先 inline-start/inline-end、start/end、逻辑 margin/padding；方向性返回箭头与翻页图标按语义镜像，品牌图标/图表坐标不能无差别翻转。
- 混合用户名、编号、邮箱与 RTL 正文时，按内容使用 bdi/dir="auto" 等语义 HTML；只加 text-align:right 不能替代方向上下文。
- Direction 不负责加载资源、改变货币、时区或服务端数据；翻译与格式化继续走项目 i18next/Intl 规则。
- 验收包括 Tabs/RadioGroup 方向键、分页、菜单、Sheet 方向、Popover 对齐、portal、数字与混合文本；不宣称新增 Provider 就完成 RTL 支持。

## Empty、Skeleton、Spinner、Progress、Alert：数据状态组合

| 当前事实 | 组件与表现 | 可用操作 |
| --- | --- | --- |
| 首次加载，尚无可展示内容 | 与最终结构接近的 Skeleton，数据容器 aria-busy | 视场景允许离开/取消；不展示“暂无数据” |
| 小型操作进行中 | Button + Spinner，保留动作含义 | 防重复，允许的取消按真实语义提供 |
| 有真实总量与当前量 | Progress + 数量/阶段说明 | 长任务提供持续结果入口 |
| 已有数据后台刷新 | 保留数据 + 局部加载标记 | 不清空表格/用户输入；失败仍可浏览旧数据 |
| 真正没有任何记录 | Empty：对象说明 + 主操作 | 有权限才显示创建/导入入口 |
| 筛选后无匹配 | Empty：当前条件下无结果 | 清除/修改筛选；不要误导用户重复创建 |
| 尚未选择对象/依赖 | Empty：先选择对象的引导 | 选择入口；不是无数据错误 |
| 查询失败，无可展示结果 | Alert + 可重试入口 | 重试、返回；不能伪装成 Empty |
| 无权限/对象不存在 | 对应授权或不可见状态 | 返回/按现有机制申请访问；不泄露对象存在性 |

- **Empty 组合**：按所引入源码使用 Empty、Header、Media、Title、Description、Content，加入明确且有用的动作。小表格内用紧凑状态，不为每个空单元格放巨大插画。
- **Skeleton 结构**：列表按实际列宽与少量行、详情按标题/段落/辅助栏、卡片按头像/文本骨架；不渲染虚假金额、姓名或“0”作为占位。骨架装饰对辅助技术隐藏，加载提示集中播报，避免每个骨架都朗读。
- **时序**：快速响应可延迟显示骨架以减少闪烁，禁止人为延长网络等待来“展示效果”。完成后真实内容替换，失败进入失败态；未知进度不用 Progress 假数。
- **视觉**：保留足够空间避免跳动，暗色可辨识，遵循 reduced-motion 偏好；不以无限动画掩盖请求失败。
- **验收**：首次慢请求、快速返回、刷新、空数据、无匹配、失败、重试和无权限的分支必须各自可观察，不能一个“加载完成”分支全包。

## 官方组件目录的覆盖与组合位置

下表把官方 llms.txt 中的通用组件归入本项目的使用位置。是按场景组织的索引，具体 props 仍需读当前官方页面与本地源码；新增官方条目通过索引发现，不假定某天的列表永久完整。

| 能力族 | 组件/官方组合方案 | 优先组合与使用位置 |
| --- | --- | --- |
| 动作 | Button、ButtonGroup | 页面主次操作、表单提交、相关命令分组；尺寸见主指南 |
| 字段组织 | Field、Label | FieldGroup/组名称/说明/错误 + RHF；不把占位当标签 |
| 文本输入 | Input、InputGroup、Textarea、InputOTP | 搜索、单位输入、多行说明、验证码；保持原组件语义 |
| 值选择 | Checkbox、RadioGroup、Select、NativeSelect、Combobox、Switch、Slider | 按选择数量、搜索、即时启停、连续值选型；NativeSelect 仅明确平台例外 |
| 日期 | Calendar、Date Picker | Calendar + Popover + Button/Input；不是完整业务日历 |
| 路径与入口 | Sidebar、NavigationMenu、Breadcrumb、Pagination | 应用导航、页面入口、当前位置、数据页码，各负其责 |
| 内容切换 | Tabs、Accordion、Collapsible | 同上下文视图、多组折叠、单块展开；关键字段不默认藏起来 |
| 工具状态 | Toggle、ToggleGroup | 编辑器粗体/多选工具状态、视图单选；ButtonGroup 本身不管理选中 |
| 命令发现 | DropdownMenu、ContextMenu、Menubar、Command、Kbd | 行命令、辅助右键、编辑器菜单、命令检索、快捷键说明；始终有可见替代路径 |
| 任务浮层 | Dialog、AlertDialog、Sheet、Drawer | 短任务、风险决定、侧向辅助、移动抽屉；长流程改页面 |
| 局部信息 | Popover、Tooltip、HoverCard | 局部交互、短说明、链接对象摘要，不能相互任意替代 |
| 数据阅读 | Table、Data Table、Card、Item、Badge、Avatar | 比较数据、可操作表格、独立分组、紧凑条目、状态、人员摘要 |
| 图形媒体 | Chart、AspectRatio、Carousel | 回答数据问题、封面比例、确需顺序浏览的媒体；图形选型见主指南 |
| 状态反馈 | Alert、Toast、Sonner、Empty、Skeleton、Spinner、Progress | 持续问题、短反馈、无内容、首次加载、局部进行中、可测进度；Toast 实现只选一套 |
| 布局基础 | Separator、ScrollArea、Resizable | 分组边界、必要局部滚动、工作区面板；不套无限滚动容器 |
| 排版方向 | Typography、Direction | 语义文本层级、方向上下文；不代替 i18n 和 locale 格式化 |

从 Components 当前目录发现 llms 索引尚未列出的条目或 AI 专用组件时，同样先按任务判断，例如 Attachment/Message/Trace 等只在确有附件、对话、追踪语义时评估；不能因为出现在目录里就安装，也不能把 AI 显示组件当作后端 Agent/审批实现。新组件接入仍受项目安全、SDK、国际化和依赖规则约束。

## 建议优先复用的业务组合

下列是组合配方而非已经存在的同名导出。先找现有实现，真正跨页面复用再抽取；避免每页重写取数、键盘与浮层逻辑。

| 业务需求 | 原语组合 | 必须保留的行为 |
| --- | --- | --- |
| 标准远程列表 | InputGroup + Select/Combobox + Table/DataTable + Pagination + Skeleton/Empty/Alert | URL、范围、分页与请求状态一致 |
| 多条件筛选 | Button + Popover + Field + Select/RadioGroup/Calendar + Badge | 草稿/应用/重置、已用条件、关闭策略 |
| 日期范围选择 | Button/Input + Popover + Calendar + FieldError | 起止校验、清除、时区、键盘输入 |
| 搜索并选择业务对象 | Combobox + Query/SDK + Spinner/Empty/Alert + 多选 chips | 防抖、乱序、分页、回显与权限隔离，详见主指南 |
| 人员/组织摘要 | 详情 Link + HoverCard + Avatar + 简短只读字段 | 链接回退、授权摘要、按需取数 |
| 表格行命令 | Button icon + DropdownMenu + AlertDialog + Toast/Alert | 可访问名称、危险确认、真实结果、焦点恢复 |
| 方案卡片单选 | FieldSet + RadioGroup/Item + Label + 描述/Card | 单选语义、键盘、说明与提交校验 |
| 创建/编辑小对象 | EditorDialog/Dialog + Field + RHF/Zod + Button + Spinner/Alert | 保留输入、失败不关闭、重复提交与焦点 |
| 移动筛选/导航 | Sheet + 同源菜单/筛选内容 + Button | 不复制一套不同的数据状态，关闭后恢复焦点 |
| 可调编辑工作区 | Resizable + 工具栏 + Tabs + 属性 Field + 局部 ScrollArea | 面板最小尺寸、键盘替代、脏状态与窄屏回退 |
| 空列表引导 | Empty + Title/Description + 有权的 Button | 无数据/无匹配区别，不将失败报为空 |
| 可恢复操作反馈 | 统一 Toaster + 业务 mutation + 持久结果入口 | 去重、受理/完成区别、真实撤销能力 |

## 每次实现必须回答的组件检查

开发者在实现说明中简要记录，不要求用户逐项批准：

1. 场景属于导航、业务选择、命令、辅助预览还是聚焦任务？为什么选择该组件？
2. 本地是否已有组件或组合？从 llms.txt/Components/Blocks 找到的官方候选是什么，是否匹配 Base UI 与锁文件？
3. 复用了哪些状态、键盘、焦点、portal、方向与可访问能力？还有哪些需要业务层接入？
4. 加载、无数据、无匹配、失败、权限、提交中与冲突分别使用什么反馈？
5. URL/Query/RHF/局部状态各管什么？是否存在失效标签、旧响应覆盖、重复提交或丢失输入？
6. 哪些交互已实现、哪些已验证、哪些原语尚未安装？不能以组件使用数量或视觉截图代替验证。

## 官方核对入口

以下官方页面于 2026-11-04 核对。上述业务状态、尺寸和验收条件属于本项目约定，官方链接用于核对组合结构与当前组件能力。

- [llms.txt](https://ui.shadcn.com/llms.txt) 与 [Components](https://ui.shadcn.com/docs/components)：持续发现入口。
- [Pagination](https://ui.shadcn.com/docs/components/base/pagination)、[Navigation Menu](https://ui.shadcn.com/docs/components/base/navigation-menu)：分页与导航组合。
- [Popover](https://ui.shadcn.com/docs/components/base/popover)、[Radio Group](https://ui.shadcn.com/docs/components/base/radio-group)：局部交互与互斥选择。
- [Toast](https://ui.shadcn.com/docs/components/base/toast)、[Hover Card](https://ui.shadcn.com/docs/components/base/hover-card)、[Dialog](https://ui.shadcn.com/docs/components/base/dialog)：短反馈、摘要和聚焦任务。
- [Direction](https://ui.shadcn.com/docs/components/base/direction)、[Empty](https://ui.shadcn.com/docs/components/base/empty)、[Skeleton](https://ui.shadcn.com/docs/components/base/skeleton)：方向、无内容和加载占位。
