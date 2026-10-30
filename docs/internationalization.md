# 国际化规范

统一 i18next + react-i18next。BCP 47 标签为 en、zh-CN，默认和 fallback 均为 en；不按浏览器语言覆盖显式选择。用户偏好只存 localStorage 的 baseline.language，不包含身份信息。统一 changeLanguage 会更新语言、文档 lang、标题和持久化；切换无需重载。当前小型应用内联三个命名空间，不引入不必要的远程资源加载，大规模功能再按需拆包。

资源路径：src/i18n/locales/{language}/{namespace}.json。common 管理基础 UI 和技术示例，navigation 管导航，validation 管输入错误和错误码。key 使用 camelCase 与语义层级，不把英文句子作 key。新增 UI 先查已有 key，提取所有可见文本（含占位、辅助名称），同步两种语言，运行 pnpm i18n:check 和测试。

使用 t('echo.result', {name}) 插值，t('formats.count', {count}) 复数；两个语言资源都保留 count_one/count_other（中文实际使用 other）以便 key 集一致。React 转义插值，不使用 dangerouslySetInnerHTML。开发环境缺失 key 输出明确 console.error，E2E 将控制台错误判为失败。Node 校验脚本比较每个语言的命名空间和叶子 key 集。ESLint 拒绝手写 JSX 可见文本。

日期、数字、货币统一使用 lib/format.ts 的 Intl。示例为固定 UTC instant，显示时显式 Asia/Shanghai；locale 变化不更改原始时间/数值。货币单位 USD 与 locale 分离；本地化不承担金额精确计算。后续业务日期应单独建模，不当作 UTC instant。

表单 Zod message 使用稳定资源 key（name），渲染时 t('validation:name')，切语言后原有错误也同步本地化。API 错误按稳定 HTTP 语义映射到 errors.invalid / unauthenticated / forbidden / not_found / unavailable / internal；未知错误统一 internal，不直接显示内部 message。

测试覆盖默认语言、fallback、插值/复数、日期/数字/货币、语言切换、文档 lang、刷新保持与资源完整性。浏览器端通过真实 API 场景验证，无 mock 网络。
