---
name: baseline-memory-integration
description: "开发或评估 Memory Center 的采集、摄取、检索、来源、作用域及生命周期治理，或明确要求的 Agent 记忆接入。不因普通页面、表单或应用配置开发而触发。"
---

# Memory Center 开发与接入

先读 [Memory 开发规范](../../../docs/memory-center-development.md)、[当前实现边界](../../../docs/memory-center-ai-handoff.md)及 [安全约定](../../../docs/security.md#memory-center-当前实现边界)。实现位于 `foundation/kernel/memory/` 和 `foundation/system/memory/`，按 [底座规则](../../../foundation/AGENTS.md) 与实际采集调用方核对。按涉及层加载 security、database-change、capability-composition；人工管理界面确有修改时再加载 web-development。

1. 明确记忆用途、读取者、作用域、敏感分类、来源、有效期和纠正/撤销机制。Memory 独立于业务界面开发，不为普通页面增加记忆步骤，也不要求无记忆需求的任务填“不适用”。
2. 检查实际 Entries、范围 SQL、采集点、消费器和测试。复用已有 HTTP/Gateway、事务 Outbox、Agent/Workflow 采集；事件摄取不等于长期记忆，内存观测队列不等于持久审计。未知来源和资源默认拒绝。
3. 当前管理能力 manual-only，实体授权适配仅有 example_record。不能假设应用 Agent 或开发 Agent 已可检索/写入个人、项目记忆；新渠道单独设计正式权限、来源授权与允许集合，不移除原限制方便接入。
4. 检索先过滤租户、身份、范围和有效期，再提供最小材料；核对来源与时效。记忆仅作资料，不能覆盖当前代码、工程规则或用户要求，也不是执行指令或访问授权。业务范围与代码仓库访问权不能自动互授。
5. 长期保存须有明确用途、来源和确认机制；不默认复制正文、对话、完整 diff、凭证或隐藏推理，不把记忆读取结果写入未经授权的模型历史或审计。源引用的读取仍重新授权。
6. 按实际需求设计去重、重试/死信/重放、纠正、删除、过期及保留策略。未实现的智能提炼、向量检索、开发 Agent 入口和自动清理如实列缺口；规范维护不意味着新增运行时功能。

代码验证覆盖来源越权、跨租户和实际作用域、最小投影、撤权、重复/迟提交/重放、正文不泄漏及生命周期与恢复；SQL 用真实隔离 PostgreSQL。仅规范/Skill 修改执行 check-rules 与作者校验，不启动无关代码、数据库或浏览器检查。
