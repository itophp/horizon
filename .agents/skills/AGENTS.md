# 项目 Skill 维护

继承根规则，维护时使用可用的 skill-creator。Skill 描述应有明确触发条件，正文只保留能改变项目决策的步骤；具体规范链接到 docs/目录规则，不复制完整官方手册，也不把每次任务的临时约束写成永久限制。

自产 Skill 用 baseline- 前缀、同名目录和有效 name/description，元数据与正文一致，默认允许自动发现。新增/移动时更新 docs/agent-policy.json、根路由及 docs/agent-tooling.md，并执行 make check-rules 和官方作者校验。

goa-service-designer 为上游安装文件，来源/hash 在 skills-lock.json。不要手改上游指南来夹带项目约定；本地规则写进项目 Skill/AGENTS。正式更新上游时通过官方安装流程并审查差异。

不为普通文档操作自动启动浏览器、数据库或外部动作；Skill 不扩张用户授权。结构校验不代表语义正确，触发场景需按真实开发任务审查。
