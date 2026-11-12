/* global console */
import { chromium } from "@playwright/test";
const browser = await chromium.launch({
  headless: true,
  executablePath:
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  args: ["--disable-gpu"],
});
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
page.setDefaultTimeout(30000);

// Login first
await page.goto("http://127.0.0.1:5173/");
await page.waitForTimeout(3000);
// Check if we're on login page
const heading = await page
  .locator("h2")
  .first()
  .textContent()
  .catch(() => "");
if (heading?.includes("Welcome") || heading?.includes("欢迎")) {
  // Login - we need credentials. Let's try the default admin
  const inputs = page.locator("input");
  const count = await inputs.count();
  console.log("Login page detected, inputs:", count);
  // We can't login without credentials. Let's just screenshot what we see.
  await page.screenshot({ path: "/tmp/current-login.png", fullPage: true });
  console.log("Login page screenshot saved");
} else {
  console.log("Heading:", heading);
}

// Try accounts page (might redirect to login)
await page.goto("http://127.0.0.1:5173/accounts");
await page.waitForTimeout(6000);
const url = page.url();
console.log("URL after navigation:", url);
await page.screenshot({ path: "/tmp/current-accounts.png", fullPage: true });

const body = await page.locator("body").innerHTML();
// Extract key elements
const hasPanel = body.includes("admin-panel");
const hasTabs = body.includes("panel-tabs");
const hasTable = body.includes("admin-table") || body.includes("data-table");
const hasAvatar =
  body.includes("person-avatar") || body.includes("entity-name");
const hasSearch = body.includes("table-toolbar");
const hasEntityForm = body.includes("entity-form");
console.log(
  JSON.stringify({
    hasPanel,
    hasTabs,
    hasTable,
    hasAvatar,
    hasSearch,
    hasEntityForm,
    isLogin: url.includes("login"),
  }),
);

await browser.close();
