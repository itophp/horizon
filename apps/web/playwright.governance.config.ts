import { defineConfig } from "@playwright/test";
import baseline from "./playwright.config";

// Independent services, build output and reports let governance verification run
// while other work uses the default end-to-end servers.
const origin = "http://127.0.0.1:5183";
process.env.E2E_ORIGIN = origin;
export default defineConfig({
  ...baseline,
  testMatch: [
    "**/governance.spec.ts",
    "**/notifications.spec.ts",
    "**/settings-schedules.spec.ts",
  ],
  use: { ...baseline.use, baseURL: origin },
  outputDir: ".local/governance-results",
  reporter: [
    ["list"],
    ["html", { outputFolder: ".local/governance-report", open: "never" }],
  ],
  webServer: [
    {
      command:
        "env -u DATABASE_URL -u DEMO_AUTH_TOKEN API_ADDR=127.0.0.1:8101 ../../scripts/go.sh run ./cmd/e2e",
      url: "http://127.0.0.1:8101/api/health",
      reuseExistingServer: false,
      gracefulShutdown: { signal: "SIGTERM", timeout: 10000 },
      timeout: 120000,
    },
    {
      command:
        "pnpm exec tsc --noEmit && pnpm exec vite build --outDir .local/governance-preview && API_PROXY_TARGET=http://127.0.0.1:8101 pnpm exec vite preview --outDir .local/governance-preview --host 127.0.0.1 --port 5183 --strictPort",
      url: origin,
      reuseExistingServer: false,
      timeout: 120000,
    },
  ],
});
