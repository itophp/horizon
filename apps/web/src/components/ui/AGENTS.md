# 公共 UI 原语

继承前端规则。这里是项目拥有的 shadcn 源码，可以维护，不等同于禁止手改的 api/generated。

修改前 rg 搜索所有调用方，保持 props、键盘交互、focus-visible、disabled、ARIA 语义和 tokens 一致。原语保持领域无关；业务取数、授权和表单工作流留在 feature/shared 层。

新增组件优先使用锁定版本 shadcn CLI 与官方 Registry，审核依赖/许可证及不需要的示例内容；安装后的可见默认文字也必须通过调用方本地化。不可把第三方示例硬编码文案视为例外。

验证受影响调用方的类型与交互、pnpm lint/typecheck/build。修改按钮/输入/对话框等交互原语时补或运行实际用户操作测试，避免只对 CSS 类名断言。
