# 浏览器验收规则

继承根与前端规则；使用 baseline-quality-gates，先读 docs/testing.md。

使用可提交的 Playwright 测试和真实 Go + 生产构建 preview；不能用 MCP 手动点击或截图目测替代。默认独立端口 5179/8099，禁止为了测试停止用户已有服务。

优先 role/label/name 定位；data-testid 仅用于无稳定可访问定位的元素。数据、浏览器上下文、语言和时区独立；等待明确 UI/请求状态，不用固定 sleep 或跨用例前置状态。

保留断言、console/page/network 诊断和失败 screenshot/trace。刷新/路由会取消查询，先明确测试期待的完成状态；不要把所有 ERR_ABORTED 粗略屏蔽。发现确定环境预算不足可记录证据并设置有界超时，不无限加重试。

生成链路检查先完成，再启动浏览器。浏览器/服务缺失必须失败并记录；本机 Chrome 可通过 PLAYWRIGHT_CHANNEL=chrome 选择，CI 仍安装匹配 Chromium。按当前用户任务选择场景，不为文档变化反复跑无关移动端测试。
