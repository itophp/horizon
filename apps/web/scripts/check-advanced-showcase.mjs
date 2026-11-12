import { chromium, expect } from "@playwright/test";
import { mkdir, writeFile } from "node:fs/promises";
const output = "/tmp/advanced-showcase-review";
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
const results = [];

// Layout: 2 patterns x 3 widths x light/dark = 12 checks.
for (const [pattern, title] of [
  ["workViews", "Work views"],
  ["interactionLab", "Interaction lab"],
]) {
  await page.goto(
    `http://127.0.0.1:5192/runtime-showcase.html?pattern=${pattern}`,
  );
  await page.getByRole("heading", { level: 1, name: title }).waitFor();
  for (const width of [1440, 768, 390]) {
    await page.setViewportSize({ width, height: 1000 });
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
      if (width === 1440 || (width === 390 && pattern === "workViews"))
        await page.screenshot({
          path: `${output}/${pattern}-${width}-${dark ? "dark" : "light"}.png`,
          fullPage: true,
          animations: "disabled",
        });
      results.push({ pattern, width, dark });
    }
  }
  globalThis.console.log("checked layouts", pattern);
}

// Work views interactions.
await page.setViewportSize({ width: 1440, height: 1000 });
await page.evaluate(() =>
  globalThis.document.documentElement.classList.remove("dark"),
);
await page.goto(
  "http://127.0.0.1:5192/runtime-showcase.html?pattern=workViews",
);
const doing = page.getByRole("region", { name: "In progress" });
const review = page.getByRole("region", { name: "In review" });
const atlas = page.locator(".sc-board-card", {
  hasText: "Atlas Customer Portal",
});
await atlas.dragTo(review);
await expect(review.getByText("Atlas Customer Portal")).toBeVisible();
await expect(page.getByRole("status")).toContainText(
  "Moved Atlas Customer Portal to In review",
);
await atlas.click({ button: "right" });
await page.getByRole("menuitem", { name: "Move back" }).click();
await page.waitForTimeout(400);
await expect(doing.getByText("Atlas Customer Portal")).toBeVisible();
await page.getByRole("tab", { name: "Delivery schedule" }).click();
await expect(page.locator(".sc-gantt-bar")).toHaveCount(6);
await page
  .getByRole("button", { name: "Atlas Customer Portal" })
  .first()
  .hover();
await expect(page.locator('[data-slot="tooltip-content"]')).toContainText(
  "complete",
);
await expect(page.locator('[data-slot="tooltip-content"]')).toContainText(
  "Atlas Customer Portal",
);
await page.getByRole("tab", { name: "November 2026" }).click();
await page.getByRole("button", { name: /^12\b/ }).click();
await expect(page.getByRole("heading", { name: "Daily agenda" })).toBeVisible();
await expect(page.locator(".sc-agenda-list li").first()).toBeVisible();
await page.screenshot({
  path: `${output}/workviews-calendar-interactions.png`,
  fullPage: true,
});

// Interaction lab interactions.
await page.goto(
  "http://127.0.0.1:5192/runtime-showcase.html?pattern=interactionLab",
);
await page.getByRole("button", { name: "Open command palette" }).click();
await page
  .getByRole("textbox", { name: "Command palette", exact: true })
  .fill("review");
await expect(page.getByRole("option", { name: /Review desk/ })).toBeVisible();
await page.keyboard.press("Enter");
await expect(page).toHaveURL(/pattern=reviewDesk/);
await expect(page.locator(".sc-toast")).toContainText("Command executed");
await page.goto(
  "http://127.0.0.1:5192/runtime-showcase.html?pattern=interactionLab",
);
await page.getByRole("button", { name: "Simulate success" }).click();
await expect(page.locator(".sc-toast")).toContainText("Draft saved locally.");
await page.getByRole("button", { name: "Dismiss notification" }).last().click();
await expect(page.locator(".sc-toast")).toHaveCount(0);
const editor = page.locator(".sc-rich-editor [contenteditable]");
const before = await page.locator(".sc-rich-meta").innerText();
await editor.click();
await page.keyboard.type(" Reviewed. ");
const after = await page.locator(".sc-rich-meta").innerText();
expect(after.length).toBeGreaterThan(before.length);
await page.getByRole("button", { name: "Edit row" }).first().click();
await page
  .getByRole("textbox", { name: "Project name" })
  .fill("Northstar refresh");
await page.getByRole("button", { name: "Save changes" }).click();
await expect(page.getByText("Northstar refresh")).toBeVisible();
await expect(page.getByText("Row saved")).toBeVisible();
await page.getByRole("textbox", { name: "Tag input" }).fill("Delivery");
await page.keyboard.press("Enter");
await expect(page.getByText("Delivery")).toBeVisible();
await page.getByRole("textbox", { name: "Tag input" }).fill("Delivery");
await page.keyboard.press("Enter");
await expect(page.getByRole("alert")).toContainText("Tag already exists");
await expect(page.locator(".sc-donut circle")).toHaveCount(4);
await expect(page.locator(".sc-area-chart")).toBeVisible();
await page.getByRole("button", { name: "切换中文" }).click();
await expect(
  page.getByRole("heading", { level: 1, name: "交互实验室" }),
).toBeVisible();
await page.screenshot({
  path: `${output}/interaction-lab-zh.png`,
  fullPage: true,
});
expect(errors).toEqual([]);
await writeFile(
  `${output}/layouts.json`,
  JSON.stringify({ results, errors }, null, 2),
);
await writeFile(
  `${output}/interactions.json`,
  JSON.stringify(
    {
      passed: true,
      errors,
      checks: [
        "board drag and drop",
        "right-click card menu",
        "gantt bars and tooltip",
        "calendar agenda",
        "command palette filter and navigation",
        "toast show and dismiss",
        "rich text editing",
        "inline row save",
        "tag add and duplicate rejection",
        "donut and area charts",
        "language switch",
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
  "layout checks and advanced interactions",
  output,
);
