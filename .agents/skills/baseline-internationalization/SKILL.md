---
name: baseline-internationalization
description: "Add or change user-visible copy, language selection, validation errors or locale-sensitive formatting in this repository. Maintain en and zh-CN resources and verify runtime behavior."
---

# 国际化变更

先读 [国际化规则](../../../apps/web/src/i18n/AGENTS.md) 和 [国际化约定](../../../docs/internationalization.md)。

先搜索已有 key；按命名空间增补语义 key，同步 en / zh-CN 的叶子 key 集。公共文案用 common、导航用 navigation、校验/稳定错误码用 validation；功能规模需要时新建独立命名空间并同步加载配置，不把所有业务翻译塞入 common。

文本节点、placeholder、title、alt、aria-label、空状态、错误、通知、表格标题都要本地化。用插值和复数，不拼接句子，不动态拼未注册 key，不直接渲染翻译 HTML。开发 missing key 诊断不可关闭。

使用项目 changeLanguage 保持偏好、document.lang 和标题同步；不得直接调用 i18n.changeLanguage 绕过持久化入口。默认/fallback 为 en，浏览器语言不得覆盖用户设置。

时间点/业务日期/时区分开建模；Intl locale 不改变时间点，货币单位不由 locale 猜测。使用 lib/format，金额精确计算不靠格式化字符串。

验证：pnpm i18n:check、pnpm lint、相关 Vitest。新增语言覆盖 fallback、插值/复数与格式化；改变切换/存储时补刷新保持的 Playwright 场景。只改资源文案不必无差别重跑数据库测试。


## 系统治理能力

治理 UI 使用 governance 命名空间双语资源；配置默认语言/主题与本人偏好走统一入口，服务端值优先，避免初始化/同步造成写回循环。字典标签按当前语言解析、停用历史仍可显示，邮件双语模板只用受控纯文本占位符。时间/数字设置须接入 Intl 消费者，不提供无效开关。
