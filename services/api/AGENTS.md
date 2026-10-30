# Go API 规则

继承根规则。Goa DSL/传输契约变更使用官方 goa-service-designer；权限变更使用 baseline-security；SQL/迁移变更使用 baseline-database-change。

- design 为 API 契约源，gen 禁止手改；cmd 组装和生命周期，internal/app 为应用逻辑，internal/transport 为传输与中间件，repository 使用 sqlc/dbgen。不要在生成目录实现领域行为。
- 校验、Payload/Result/Error/HTTP/security 声明回到 DSL；读取生成接口后再实现，不凭记忆猜签名。OpenAPI + Orval 是前端类型来源。
- 只运行项目锁定 Goa 生成器；scripts/go.sh 在正确模块和工具链执行。全目录格式检查包含 integration build tag 文件，不能只 go fmt ./... 漏掉带标签文件。
- 配置集中验证，非法安全配置启动失败。统一请求 ID、结构化日志、时间/体积限制和稳定错误映射；日志与错误不得泄露凭证/内部数据。
- 用小接口/Fake 隔离依赖，context/cancellation 传播到 SQL；明确事务提交回滚，服务关闭有界。
- 先执行相关 scripts/go.sh test；后续 make ci-fast，触及 DB 再 test-integration，跨层行为再 test-e2e。生成变更必须 check-generated；安全变更必须允许+拒绝测试。
