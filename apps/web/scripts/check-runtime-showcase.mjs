import { URL } from "node:url";
import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile, readFile } from "node:fs/promises";
const output = "/tmp/runtime-showcase-review";
await mkdir(output, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  args: ["--disable-gpu"],
  executablePath:
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
});
const page = await browser.newPage();
page.setDefaultTimeout(60000);
const errors = [];
page.on(
  "pageerror",
  (e) => (globalThis.console.error(e.message), errors.push(e.message)),
);
const results = [];
const messages = JSON.parse(
  await readFile(
    new URL("../src/i18n/locales/en/showcase.json", import.meta.url),
    "utf8",
  ),
);
await page.goto("http://127.0.0.1:5189/runtime-showcase.html?engine=0");
if (!globalThis.process.env.SKIP_LAYOUT)
  for (const width of [768, 390, 1440]) {
    await page.setViewportSize({ width, height: 1000 });
    for (let engine = 0; engine < 13; engine++) {
      globalThis.console.log("checking", width, engine);
      await page
        .locator(`.sc-sidebar nav a[href$="?engine=${engine}"]`)
        .click();
      await page
        .getByRole("heading", {
          level: 1,
          name: messages.engines[String(engine)].name,
          exact: true,
        })
        .waitFor();
      for (const dark of [false, true]) {
        await page.evaluate(
          (dark) =>
            globalThis.document.documentElement.classList.toggle("dark", dark),
          dark,
        );
        const overflow = await page.evaluate(
          () =>
            globalThis.document.documentElement.scrollWidth >
            globalThis.innerWidth,
        );
        if (overflow) {
          globalThis.console.log(
            await page.evaluate(() =>
              Array.from(globalThis.document.querySelectorAll("*"))
                .filter(
                  (e) =>
                    e.getBoundingClientRect().right > globalThis.innerWidth,
                )
                .map((e) => ({
                  tag: e.tagName,
                  cls: e.className,
                  width: e.getBoundingClientRect().width,
                  right: e.getBoundingClientRect().right,
                }))
                .slice(0, 25),
            ),
          );
          await page.screenshot({
            path: "/tmp/sc-overflow.png",
            timeout: 15000,
          });
          throw Error(`Overflow: ${engine}/${width}/${dark}`);
        }
        if ((!dark && width === 1440) || (engine === 6 && width !== 1440))
          await page.screenshot({
            path: `${output}/${width}-${engine}-${dark ? "dark" : "light"}.png`,
            fullPage: true,
            animations: "disabled",
            timeout: 60000,
          });
        results.push({ engine, width, dark, overflow });
      }
    }
  }
if (results.length)
  await writeFile(
    `${output}/layout-results.json`,
    JSON.stringify({ results, errors }, null, 2),
  );
await page.goto("http://127.0.0.1:5189/runtime-showcase.html?engine=6");
await page
  .getByRole("textbox", { name: "Search records…" })
  .pressSequentially("Atlas");
globalThis.console.log(
  "search",
  page.url(),
  await page.getByRole("textbox", { name: "Search records…" }).inputValue(),
);
await expect(page.locator("tbody tr")).toHaveCount(2, { timeout: 15000 });
await page.getByRole("textbox", { name: "Search records…" }).fill("");
await page.getByRole("button", { name: "Continue", exact: true }).click();
await page.getByText("Page 2 of 3", { exact: true }).waitFor();
await page.goto("http://127.0.0.1:5189/runtime-showcase.html?engine=5");
await page.getByRole("button", { name: "Continue", exact: true }).click();
await page.getByText("Enter at least 3 characters", { exact: true }).waitFor();
await page
  .getByRole("textbox", { name: "Project name" })
  .fill("Launch reference");
await page.getByRole("button", { name: "Continue", exact: true }).click();
await page
  .getByRole("button", { name: "Add deliverable", exact: true })
  .click();
await page.getByRole("button", { name: "Continue", exact: true }).click();
await page.getByRole("button", { name: "Save changes", exact: true }).click();
await page.getByRole("status").waitFor();
await page.goto("http://127.0.0.1:5189/runtime-showcase.html?engine=7");
await page.getByRole("button", { name: "Run simulation", exact: true }).click();
await page
  .getByRole("dialog")
  .getByRole("button", { name: "Run simulation", exact: true })
  .click();
await page.getByRole("heading", { name: "Execution completed" }).waitFor();
for (const state of [
  "empty",
  "loading",
  "skeleton",
  "error",
  "network",
  "denied",
  "readonly",
  "disabled",
  "partial",
  "long",
  "large",
  "multilingual",
]) {
  await page.goto(
    `http://127.0.0.1:5189/runtime-showcase.html?engine=6&state=${state}`,
  );
  await page.getByRole("heading", { level: 1 }).waitFor();
  await page.screenshot({
    path: `${output}/state-${state}.png`,
    fullPage: true,
    animations: "disabled",
    timeout: 60000,
  });
}
await page.getByRole("button", { name: "切换中文" }).click();
await page.getByRole("heading", { name: "表格运行时", exact: true }).waitFor();
if (errors.length) throw Error(errors.join("\n"));
await writeFile(
  `${output}/results.json`,
  JSON.stringify(
    {
      results,
      errors,
      interactions:
        "search, pagination, validation, wizard, save, confirmation, execution, language, states passed",
    },
    null,
    2,
  ),
);
await browser.close();
globalThis.console.log(
  `Passed ${results.length} viewport/theme checks and interaction paths. Evidence: ${output}`,
);
