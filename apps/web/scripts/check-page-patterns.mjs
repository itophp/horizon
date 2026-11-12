import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
const out = "/tmp/runtime-page-patterns-review";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath:
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  args: ["--disable-gpu"],
});
const page = await browser.newPage();
page.setDefaultTimeout(30000);
const errors = [];
page.on("pageerror", (e) => errors.push(e.message));
const patterns = [
  "dashboard",
  "listing",
  "recordDetail",
  "entryForm",
  "preferences",
  "reviewDesk",
  "reporting",
  "assets",
];
const headings = [
  "Dashboard",
  "Data list",
  "Record details",
  "Data entry",
  "Settings",
  "Review desk",
  "Analytics",
  "Resources",
];
const results = [];
await page.goto(
  "http://127.0.0.1:5190/runtime-showcase.html?pattern=dashboard",
);
if (!globalThis.process.env.SKIP_LAYOUT)
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    for (let i = 0; i < patterns.length; i++) {
      await page
        .locator(`.sc-sidebar nav a[href$="?pattern=${patterns[i]}"]`)
        .click();
      await page
        .getByRole("heading", { level: 1, name: headings[i], exact: true })
        .waitFor();
      for (const dark of [false, true]) {
        await page.evaluate(
          (value) =>
            globalThis.document.documentElement.classList.toggle("dark", value),
          dark,
        );
        expect(
          await page.evaluate(
            () =>
              globalThis.document.documentElement.scrollWidth <=
              globalThis.innerWidth,
          ),
        ).toBe(true);
        if (width === 1440 || (width === 390 && i === 1))
          await page.screenshot({
            path: `${out}/${width}-${patterns[i]}-${dark ? "dark" : "light"}.png`,
            animations: "disabled",
            fullPage: true,
          });
        results.push({ pattern: patterns[i], width, dark });
      }
      globalThis.console.log("checked", width, patterns[i]);
    }
  }
if (results.length)
  await writeFile(
    `${out}/layouts.json`,
    JSON.stringify({ results, errors }, null, 2),
  );
await page.setViewportSize({ width: 1440, height: 1000 });
await page.goto("http://127.0.0.1:5190/runtime-showcase.html?pattern=listing");
for (const variant of [
  "Standard list",
  "Categories and list",
  "Tree and list",
]) {
  await page
    .getByRole("combobox", { name: "Layout decision", exact: true })
    .click();
  await page.getByRole("option", { name: variant, exact: true }).click();
  if (variant === "Standard list")
    await expect(page.locator(".sc-filter-nav")).toHaveCount(0);
  else
    await expect(
      page.getByRole("heading", {
        name:
          variant === "Tree and list"
            ? "Organization hierarchy"
            : "Business category",
        exact: true,
      }),
    ).toBeVisible();
}
await page
  .getByRole("button", { name: "Platform engineering", exact: true })
  .click();
await expect(page.locator("tbody tr")).toHaveCount(8);
await expect(page.getByText("8 records", { exact: true })).toBeVisible();
await page.getByRole("tab", { name: "In review", exact: true }).click();
await expect(
  page.getByRole("heading", { name: "No records yet" }),
).toBeVisible();
await page.getByRole("button", { name: "Clear search", exact: true }).click();
await expect(page.getByText("24 records", { exact: true })).toBeVisible();
await page.getByRole("textbox", { name: "Search records…" }).fill("Atlas");
await expect(page.locator("tbody tr")).toHaveCount(2);
globalThis.console.log("passed filters");
await page.goto(
  "http://127.0.0.1:5190/runtime-showcase.html?pattern=reviewDesk",
);
await page.getByRole("button", { name: "Approve", exact: true }).click();
await page
  .getByRole("dialog")
  .getByRole("button", { name: "Review and confirm", exact: true })
  .click();
await expect(page.getByRole("alert")).toBeVisible();
await page
  .getByRole("textbox", { name: "Decision note" })
  .fill("Ready for release");
await page
  .getByRole("dialog")
  .getByRole("button", { name: "Review and confirm", exact: true })
  .click();
await expect(
  page.getByRole("button", { name: "Approve", exact: true }),
).toBeDisabled();
await page.goto(
  "http://127.0.0.1:5190/runtime-showcase.html?pattern=preferences",
);
await page
  .getByRole("button", { name: "Delivery and notifications", exact: true })
  .click();
await page.getByRole("switch", { name: "Daily digest" }).click();
await page.getByRole("button", { name: "Save changes", exact: true }).click();
await expect(page.getByRole("alert")).toBeVisible();
await page.goto("http://127.0.0.1:5190/runtime-showcase.html?pattern=assets");
await page
  .getByRole("button", { name: "Preview", exact: true })
  .first()
  .click();
await expect(page.getByRole("dialog")).toBeVisible();
await page.getByRole("button", { name: "Close", exact: true }).click();
await expect(page.getByRole("dialog")).toHaveCount(0);
await page.goto(
  "http://127.0.0.1:5190/runtime-showcase.html?pattern=entryForm",
);
await page.getByRole("button", { name: "Save changes", exact: true }).click();
await expect(page.getByRole("alert")).toBeVisible();
await page
  .getByRole("textbox", { name: "Project name", exact: true })
  .fill("Operations rollout");
await page.getByRole("button", { name: "Save changes", exact: true }).click();
await expect(
  page.getByText("Saved locally. No server data changed.", { exact: true }),
).toBeVisible();
await page.getByRole("button", { name: "切换中文" }).click();
await expect(
  page.getByRole("heading", { level: 1, name: "业务表单" }),
).toBeVisible();
expect(errors).toEqual([]);
await writeFile(
  `${out}/interactions.json`,
  JSON.stringify(
    {
      passed: true,
      errors,
      checks: [
        "tree scope",
        "status scope",
        "search",
        "approval validation and decision",
        "settings switch and save",
        "resource sheet",
        "single form validation and save",
        "language",
      ],
    },
    null,
    2,
  ),
);
await browser.close();
globalThis.console.log(
  "Passed",
  results.length,
  "layout/theme checks and interaction paths",
  out,
);
