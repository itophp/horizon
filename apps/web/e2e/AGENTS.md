# 浏览器验收规则

继承根与前端规则；使用 baseline-quality-gates，先读 docs/testing.md。

**E2E 暂停（现行指令）**：默认不启动浏览器验收。需要旁证时只跑改动模块对应的单个 spec（映射见 scripts/e2e-changed.mjs，用 `E2E_ARGS="e2e/<模块>.spec.ts"` 或 `make test-e2e-changed`）；映射未命中就不跑并在汇报里记未覆盖。完整集合（不带 `E2E_ARGS` 的 `make test-e2e`、`make test-e2e-parallel`、默认 `make ci`、远程 CI 浏览器阶段）必须先取得用户当轮明确授权；未授权写“E2E 未执行（按当前指示暂停）”。

使用可提交的 Playwright 测试和真实 Go + 生产构建 preview；不能用 MCP 手动点击或截图目测替代。默认独立端口 5179/8099，禁止为了测试停止用户已有服务。

日常只跑相关文件：`make test-e2e-changed` 按改动选文件（映射在 scripts/e2e-changed.mjs），`make test-e2e-smoke` 只跑标题带 `@smoke` 的用例；两者都不等于全量，交付时须写明选择与未覆盖集合。`@smoke` 只标记单文件内最便宜且真正跨层的用例，不改动断言强度。需要完整集合又不想等串行时用 `make test-e2e-parallel`，分组依据是各 spec 内写死的 Origin，新增 spec 先确认归属栈。

优先 role/label/name 定位；data-testid 仅用于无稳定可访问定位的元素。数据、浏览器上下文、语言和时区独立；等待明确 UI/请求状态，不用固定 sleep 或跨用例前置状态。

保留断言、console/page/network 诊断和失败 screenshot/trace。刷新/路由会取消查询，先明确测试期待的完成状态；不要把所有 ERR_ABORTED 粗略屏蔽。发现确定环境预算不足可记录证据并设置有界超时，不无限加重试。

生成链路检查先完成，再启动浏览器。浏览器/服务缺失必须失败并记录；本机 Chrome 可通过 PLAYWRIGHT_CHANNEL=chrome 选择，CI 仍安装匹配 Chromium。按当前用户任务选择场景，不为文档变化反复跑无关移动端测试。
