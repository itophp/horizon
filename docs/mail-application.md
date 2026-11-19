# 个人邮箱应用实施记录

> 边界说明：业务应用与底座核心的拆分原则见 [业务应用插件化与内核边界说明](application-plugin-boundaries.md)。当前邮箱实现中仍有部分集成点待按该文档重构为插件化贡献。

## 当前实现

- 业务模块位于 `applications/mail`，以 IAM 账号为唯一拥有者；一个账号可配置多个外部 SMTP/POP3S 邮箱。
- `mail.mailbox.onboard` 在同一事务创建 mailbox、通道和密封凭证，并完成 SMTP/POP3S 连接验证；双通道成功后自动激活，失败时保留 unverified 供修正。第一个邮箱自动成为默认发送邮箱。
- 发送入口 `mail.message.send` 创建正文、收件人、附件绑定和 `mail_outbound_deliveries`，worker 经 `mail.outbound.process` 调用外部 SMTP；未到上限的失败自动回到 queued 并指数退避，超时标记 ambiguous。
- POP3S worker 经 `mail.inbox.sync` 按 UIDL 分批拉取；MIME 正文和附件本地导入成功后标记远端删除，QUIT 仅在本地事务提交后执行。
- 入站 HTML 与出站 Markdown 生成的 HTML 均经 bluemonday allowlist 清理；前端 `mail-workspace` 使用 sandboxed iframe 自动优先 HTML、缺失时回退纯文本。
- 页面自动预填邮箱域名对应的 SMTP/POP 主机、默认端口和用户名，自动选择默认邮箱，并自动填充 Reply/Reply All/Forward；Forward 自动携带原附件。邮箱编辑、凭证轮换、附件上传、发送重试/取消和连接测试均走对应 Gateway capability。

## 安全与一致性边界

- 权限：`mail:read`、`mail:send`、`mail:configure` 仅本人；`mail:sync` 为系统 worker 专用，不临时授予。
- 凭证：AES-GCM 密文，AAD 绑定租户、邮箱、用途和版本；仅 fingerprint/版本可投影。
- 网络：SMTP TLS/STARTTLS、POP3S direct TLS，endpoint 精确白名单，证书验证不可关闭。
- 外部发送按至少一次处理；provider 超时标记 ambiguous。POP3 事务失败或 QUIT 失败时远端邮件保留，下一批按 UID 去重。

## 验证状态

已实现单元/组件测试覆盖凭证 AAD、HTML 清理、MIME 解析、endpoint 精确白名单、超时分类、自动表单、附件接线、发送重试与 sandbox 渲染；真实 PostgreSQL 集成覆盖迁移、多邮箱、失败保存在 unverified、发送事务/事件、审计脱敏与跨账号拒绝。真实外部 SMTP/POP3 服务器互通和浏览器验收须在独立环境中另行执行，不用单元测试代替。
