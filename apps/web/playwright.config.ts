import { defineConfig, devices } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  workers: 1,
  timeout: 120000,
  expect: { timeout: 30000 },
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: "http://127.0.0.1:5179",
    timezoneId: "Asia/Shanghai",
    locale: "en-US",
    trace: { mode: "retain-on-failure", screenshots: false, snapshots: true },
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        channel:
          process.env.PLAYWRIGHT_CHANNEL === "chrome" ? "chrome" : undefined,
      },
    },
  ],
  webServer: [
    {
      command:
        "WEBMCP_PORT=18794 WEBMCP_STATE_DIR=.local/webmcp-e2e WEBMCP_ALLOWED_ORIGINS=http://127.0.0.1:5179 node ../../scripts/webmcp-server.mjs",
      url: "http://127.0.0.1:18794/health",
      reuseExistingServer: false,
      gracefulShutdown: { signal: "SIGTERM", timeout: 10000 },
      timeout: 30000,
    },
    {
      command:
        "env -u DATABASE_URL -u DEMO_AUTH_TOKEN API_ADDR=127.0.0.1:8099 ../../scripts/go.sh run ./cmd/e2e",
      url: "http://127.0.0.1:8099/api/health",
      reuseExistingServer: false,
      gracefulShutdown: { signal: "SIGTERM", timeout: 10000 },
      timeout: 120000,
    },
    {
      command:
        "pnpm build && API_PROXY_TARGET=http://127.0.0.1:8099 pnpm exec vite preview --host 127.0.0.1 --port 5179 --strictPort",
      url: "http://127.0.0.1:5179",
      reuseExistingServer: false,
      timeout: 120000,
    },
  ],
});
