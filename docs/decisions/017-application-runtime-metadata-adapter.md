# ADR 017：在现有实体模型上适配 Application Runtime

状态：提议，未实施。日期：2026-11-05。

## 背景

[Application Runtime 实施规范](../AI-Native%20Application%20Runtime%20开发实施规范.md)要求先审查仓库。本仓库已有 Entity 声明、迁移校验、Capability Gateway、内置插件以及文章业务闭环；缺少 Page/Application Schema 和通用渲染。用户要求首期采用文章 List/Detail/Form。

## 决策提议

- 保留 `contracts.Entity` 和现有标识。通过 Adapter 读取合并实体目录、能力 I/O 与独立页面贡献；UI 配置不进入 Entity，实体全字段不直接成为命令输入。
- 随代码注册只读 Application/Page 定义，独立 Runtime Schema 版本与内容摘要；不先做元数据发布数据库或动态安装平台。
- 公共投影走 Goa/OpenAPI/Orval；业务动作仍由模块 Definition 定义。补齐生成映射或契约一致性测试，避免手工维护两套页面类型。
- Data Runtime 采用绑定既有能力的 Provider；文章列表在原模块扩展受控查询，Runtime 不拼任意 SQL、不直接调用领域 Handler。
- 第一阶段实现必要保存动作、刷新、幂等与错误处理；完整 Action DSL 留到后续，复用既有审核发布 Workflow。
- 复用现有 Shell、前端栈和 Plugin Manager；保留 Metadata-driven/Hybrid/Custom 三种方式，扩展代码仅可信编译期注册。
- 字段展示和语义上下文显式裁剪；授权、记录范围、状态及 revision 始终由服务端执行。

## 影响与替代方案

可以渐进迁移文章页面而不改业务事实源；代价是需要显式的字段/能力投影映射与版本校验。暂不选择通用 SQL CRUD、重写 Entity Schema、第二套 IAM 或新 Plugin Manager：这些方案重复现有机制，且会扩大授权与迁移成本。

静态贡献尚不提供在线发布、租户安装、远程组件加载或完整字段策略配置。第二个业务模块接入后再评估抽象扩展，避免以文章特例代替通用能力。

完整证据、接口建议、实施顺序与验证见 [差距分析](../application-runtime-gap-analysis.md)。本 ADR 是下一阶段设计输入，不表示代码与验收已完成。

## 第一阶段实施记录

第一阶段已按静态贡献、独立 Adapter 与原业务能力复用实现。公共页面数据契约采用 Go 模型派生能力 Output Schema，并由 cmd/runtimespec 生成 TS/Zod；由于未改变能力传输信封，没有添加 Goa 方法或重复维护公开 DTO。首期关系只输出安全描述，实际关联查询不开放。实施入口及验证范围见 [第一阶段说明](../application-runtime-phase-1.md)。上文长期方案仍保留后续阶段的提议状态。
