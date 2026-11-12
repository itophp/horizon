import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
const output = "/tmp/erp-showcase-review";
await mkdir(output, { recursive: true });
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
const modules = [
  "Operations overview",
  "Procurement",
  "Inventory",
  "Production planning",
  "Finance reconciliation",
];
const results = [];
await page.goto(
  "http://127.0.0.1:5191/runtime-showcase.html?pattern=dashboard",
);
await page
  .getByRole("heading", { level: 1, name: "Manufacturing ERP workspace" })
  .waitFor();
for (const width of [1440, 768, 390]) {
  await page.setViewportSize({ width, height: 1000 });
  for (const module of modules) {
    await page.getByRole("tab", { name: module, exact: true }).click();
    await expect(
      page.getByRole("tab", { name: module, exact: true }),
    ).toHaveAttribute("aria-selected", "true");
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
      if (width === 1440 || (width === 390 && module === "Inventory"))
        await page.screenshot({
          path: `${output}/${width}-${modules.indexOf(module)}-${dark ? "dark" : "light"}.png`,
          fullPage: true,
          animations: "disabled",
        });
      results.push({ width, module, dark });
    }
    globalThis.console.log("checked", width, module);
  }
}
await writeFile(
  `${output}/layouts.json`,
  JSON.stringify({ results, errors }, null, 2),
);
await page.setViewportSize({ width: 1440, height: 1000 });
await page.getByRole("tab", { name: "Procurement", exact: true }).click();
await page
  .getByRole("textbox", { name: "Search records…" })
  .fill("PO-202611-018");
await expect(page.locator("tbody tr")).toHaveCount(1);
await page.getByRole("button", { name: "PO-202611-018", exact: true }).click();
await expect(
  page.getByRole("dialog").getByText("苏州精工材料", { exact: true }),
).toBeVisible();
await page.getByRole("tab", { name: "Document history", exact: true }).click();
await expect(
  page.getByText("Supplier confirmed delivery date", { exact: true }),
).toBeVisible();
await page.getByRole("tab", { name: "Attachments", exact: true }).click();
await expect(
  page.getByText("Purchase contract.pdf", { exact: true }),
).toBeVisible();
await page
  .getByRole("button", { name: "Simulate receipt", exact: true })
  .click();
await page
  .getByRole("dialog")
  .last()
  .getByRole("button", { name: "Review and confirm", exact: true })
  .click();
await expect(
  page.getByRole("button", { name: "Simulate receipt", exact: true }),
).toBeDisabled();
await page.getByRole("button", { name: "Close", exact: true }).last().click();
await expect(
  page.getByText("Demonstration order marked as received.", { exact: true }),
).toBeVisible();
await page.getByRole("tab", { name: "Inventory", exact: true }).click();
await page
  .getByRole("combobox", { name: "Warehouse scope", exact: true })
  .click();
await page.getByRole("option", { name: "Finished goods", exact: true }).click();
await expect(page.locator("tbody tr")).toHaveCount(2);
await page.getByRole("button", { name: "切换中文" }).click();
await expect(
  page.getByRole("heading", { level: 1, name: "制造型 ERP 工作台" }),
).toBeVisible();
await page.screenshot({ path: `${output}/zh-inventory.png`, fullPage: true });
expect(errors).toEqual([]);
await writeFile(
  `${output}/interactions.json`,
  JSON.stringify(
    {
      passed: true,
      errors,
      checks: [
        "module navigation",
        "search",
        "purchase order sheet",
        "document history",
        "attachments",
        "receipt confirmation and disabled repeat",
        "warehouse filter",
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
  "layout checks and ERP interactions",
  output,
);
