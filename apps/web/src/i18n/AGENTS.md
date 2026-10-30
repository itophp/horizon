# 翻译与 locale 规则

继承前端规则；使用 baseline-internationalization，完整约定见 docs/internationalization.md（从仓库根解析）。

- 默认/fallback=en；支持 en/zh-CN，BCP 47 标签。所有语言按命名空间保持相同 key，禁止把 key 当正常回退 UI。
- common/navigation/validation 是基础命名空间；规模需要时新增功能命名空间及明确加载注册，不把全部业务翻译塞进 common。
- key 用语义层级；插值和复数由 i18next 处理，不拼句、不渲染资源 HTML。新增 key 同步所有语言。
- 使用 changeLanguage 统一更新资源、偏好、document.lang 和标题；不得绕过该入口。浏览器语言不能覆盖用户选择。
- 表单与服务端错误先映射稳定 key/code，再本地化；组件不得展示内部错误明文。
- Intl 只负责展示；显式处理时区和货币单位，不改变原始数值，不将格式化金额用于精确计算。
- 修改后运行 pnpm i18n:check 与 locale 测试；影响动态切换/存储时再验证刷新保持。
