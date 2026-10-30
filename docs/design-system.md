# 设计系统

shadcn CLI 4.23.0 按当前官方 Vite 流程初始化（base-nova / Base UI），组件源码纳入 components/ui。components.json 为唯一 Registry 与路径配置，新增组件使用 pnpm --filter @baseline/web exec shadcn add <component>。当前 button/card/input/table 已用于示例。官方 registry 不需要额外凭证；尚未安装全局 MCP，CLI 保持可重复使用。

Tailwind 4 Vite 插件，基础 tokens 在 styles/index.css，后台在 styles/admin.css 中统一覆盖。/foundation 技术示例使用中性色背景、白色内容面板、蓝色主要动作；后台按下文现行基准使用浅/暗主题和中性主操作配色。统一 --primary / --background / --foreground / --radius，不在每页定义独立体系。Geist 字体通过 npm 本地打包，没有运行时字体 CDN。

/foundation 技术示例的桌面布局为 244px 侧栏、76px 顶栏、44px 主内容水平留白；1050px 以下示例卡片单列，640px 以下侧栏转顶部导航。保留 focus-visible、skip link、表单 label / aria-invalid / aria-describedby 和 live region。移动端 E2E 校验 390px 视口无水平溢出。

组件来源为官方 MIT 项目；新增依赖见锁文件。公共组件升级需审查可访问性、许可证、调用方和生成 CLI 的变动。第三方组件源码可由项目维护，API/SQL 生成目录仍禁止手改。

## 现有界面是设计基准

新页面和后续改版应延续当前已确认的同类界面，优先复用公共布局、组件和 tokens。后台参考现有账号/组织/部门/角色/权限页面，登录页参考现有登录页；技术验收示例不定义后台风格。保持配色层级、排版、间距密度、圆角边框、表单表格、图标和交互反馈一致，并同时核对浅色、暗色和滚动表现。不能为单页自建视觉体系；用户明确提出新的设计方向时，更新这份基准并检查公共页面。执行入口见 [前端规则](../apps/web/AGENTS.md#界面风格一致性)。

## 管理后台

后台参考用户提供的深色 shadcn 仪表台：随主题切换的中性页面底色、黑灰工作区、细边框面板、紧凑表格与固定侧栏，tokens 和布局集中在 styles/admin.css。ThemeProvider 管理 dark/light 与 baseline.theme，浅色使用同一结构和组件。登录页采用宽幅品牌区与约 322px 的紧凑表单，不把后台卡片密度搬到登录页。

管理页面使用同一 PageHeading、DataTable、EditorDialog、状态徽标及权限矩阵；统计只来自实际授权数据。Dialog、Checkbox、DropdownMenu 等采用 Base UI/shadcn 源码，前端 label/description/error 保持关联，错误不改变控件可访问名称。图标用 lucide，不依赖远程图片或字体。历史技术示例保留自己的 .app-shell 布局，后台不要复制其旧配色。

后台外层统一使用侧栏底色；侧栏本身不画右边框。console-main 是独立的有边框内容面板，左上角 28px 圆角，固定在视口内；顶栏与页脚不参与正文滚动，只有 console-content 独立滚动。不要恢复 window 滚动或把圆角只画在顶栏上，否则正文滚动会盖住包裹式圆角。

Base UI Button 在项目中默认作为普通按钮使用；表单主提交动作显式声明 type="submit"，取消、添加行、预校验和工具动作使用 type="button"。新表单验证实际点击后调用生成 SDK 的行为，避免仅检查按钮渲染。
