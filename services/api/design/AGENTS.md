# API Design First

继承服务规则；使用 .agents/skills/goa-service-designer/SKILL.md（仓库根路径）。按变更阅读其 modeling/transport/generated-code 参考，不必加载无关参考。

Payload、Result、Required、Validation、HTTP 映射、Error 和 Security 必须在 DSL 明确。按方法实际适用范围声明错误；不要为获得构造器把单方法错误提升到整个服务。

命名检查下游 TypeScript 内建类型冲突，例如使用 TechnicalRecord 而不是 Record。UTC 时间点用明确格式；租户/owner 不作为客户端可信授权来源。

执行 make generate，更新非生成目录实现与受影响 SDK 消费者，再 make check-generated、两端类型/编译和 HTTP 测试。不得手改 OpenAPI 或 SDK 修补不兼容；回到契约或生成器配置。报告字段删除、收紧校验、状态码/权限改变的兼容性影响。
