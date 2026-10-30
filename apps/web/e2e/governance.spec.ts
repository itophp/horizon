import { test, expect, type Page } from "@playwright/test";
import type { LoginResult } from "../src/api/generated/models";
async function signIn(page: Page) {
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill("admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Workspace overview", exact: true }),
  ).toBeVisible();
}
test.beforeEach(async ({ page, request }) => {
  const response = await request.post("/api/auth/login", {
    headers: { Origin: process.env.E2E_ORIGIN ?? "http://127.0.0.1:5179" },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(response.ok()).toBeTruthy();
  const login: LoginResult = await response.json();
  const reset = await request.put("/api/auth/preferences", {
    headers: {
      Origin: process.env.E2E_ORIGIN ?? "http://127.0.0.1:5179",
      "X-CSRF-Token": login.profile.csrfToken,
    },
    data: {
      changes: [
        { key: "display.language", operation: "reset", value: "" },
        { key: "display.theme", operation: "reset", value: "" },
        { key: "display.pageSize", operation: "reset", value: "" },
      ],
    },
  });
  expect(reset.ok()).toBeTruthy();
  await page.addInitScript(() =>
    localStorage.setItem("baseline.language", "en"),
  );
});
test("settings draft, publish, runtime branding and server preferences persist", async ({
  page,
}, testInfo) => {
  test.setTimeout(240000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await signIn(page);
  await page
    .getByRole("link", { name: "System settings", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "System settings", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("combobox", { name: "Configuration scope" }),
  ).toHaveCount(0);
  const name = `Workspace ${Date.now()}`;
  const reason = `Brand update ${Date.now()}`;
  await page
    .getByRole("combobox", { name: "Product name · Operation", exact: true })
    .selectOption("set");
  await page
    .getByRole("textbox", { name: "Product name", exact: true })
    .fill(name);
  await page
    .getByRole("textbox", { name: "Change reason", exact: true })
    .fill(reason);
  await page.getByRole("button", { name: "Save draft", exact: true }).click();
  const draft = page.locator(".gov-change").filter({ hasText: reason });
  await expect(draft).toBeVisible();
  await expect(page.locator(".console-brand strong")).not.toHaveText(name);
  await draft.getByRole("button", { name: "Publish", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Confirm", exact: true })
    .click();
  await expect(page.locator(".console-brand strong")).toHaveText(name);
  await page.reload();
  await expect(page.locator(".console-brand strong")).toHaveText(name);
  await page
    .getByRole("button", { name: "Personal preferences", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Personal preferences", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("combobox", { name: "Rows per page · Operation", exact: true })
    .selectOption("set");
  await page
    .getByRole("textbox", { name: "Rows per page", exact: true })
    .fill("5");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("status")).toHaveText("Changes saved");
  await page.reload();
  await expect(
    page.getByRole("textbox", { name: "Rows per page", exact: true }),
  ).toHaveValue("5");
  await page
    .getByRole("button", { name: "Switch appearance", exact: true })
    .click();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await expect
    .poll(async () => {
      const r = await page.request.get("/api/auth/configuration");
      const body = (await r.json()) as {
        entries: { key: string; value: string }[];
      };
      return body.entries.find((e) => e.key === "display.theme")?.value;
    })
    .toBe('"light"');
  await page.reload();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await page.screenshot({
    path: testInfo.outputPath("preferences-light.png"),
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
test("dictionary editing, import preview and private file downloads use real services", async ({
  page,
}, testInfo) => {
  test.setTimeout(300000);
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await signIn(page);
  // Dictionaries (field options) now live in the Field management tab of System settings.
  await page
    .getByRole("link", { name: "System settings", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Field management", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Create dictionary", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("combobox", { name: "Configuration scope" }),
  ).toHaveCount(0);
  const code = `e2e.${Date.now()}`;
  await dialog
    .getByRole("textbox", { name: "Stable code", exact: true })
    .fill(code);
  await dialog
    .getByRole("textbox", { name: "English name", exact: true })
    .fill("Priority choices");
  await dialog
    .getByRole("textbox", { name: "Chinese name", exact: true })
    .fill("优先级选项");
  await dialog.getByRole("button", { name: "Add item", exact: true }).click();
  await dialog
    .getByRole("textbox", { name: "Stable code", exact: true })
    .nth(1)
    .fill("normal");
  await dialog
    .getByRole("textbox", { name: "English label", exact: true })
    .fill("Normal");
  await dialog
    .getByRole("textbox", { name: "Chinese label", exact: true })
    .fill("普通");
  await dialog
    .getByRole("textbox", { name: "Change reason", exact: true })
    .fill("Create priority choices");
  await dialog.getByRole("button", { name: "Save draft", exact: true }).click();
  await expect(dialog).not.toBeVisible();
  await page.getByRole("button", { name: "Publish", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Confirm", exact: true })
    .click();
  await expect(
    page
      .getByRole("combobox", { name: "Usage preview", exact: true })
      .getByRole("option", { name: "Normal", exact: true }),
  ).toBeAttached();
  await page.getByRole("button", { name: "Import", exact: true }).click();
  const importer = page.getByRole("dialog");
  await importer.getByLabel("File name", { exact: true }).setInputFiles({
    name: "choices.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("code,labelEn,labelZh\nhigh,High,高\n"),
  });
  await importer
    .getByRole("button", { name: "Validate and preview", exact: true })
    .click();
  await expect(importer.getByRole("status")).toHaveText("1 added · 0 updated");
  await importer
    .getByRole("textbox", { name: "Change reason", exact: true })
    .fill("Import an additional option");
  await importer
    .getByRole("button", { name: "Save import draft", exact: true })
    .click();
  await expect(importer).not.toBeVisible();
  await page.getByRole("button", { name: "Publish", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Confirm", exact: true })
    .click();
  await expect(
    page
      .getByRole("combobox", { name: "Usage preview", exact: true })
      .getByRole("option", { name: "High", exact: true }),
  ).toBeAttached();
  // Attachment management is no longer a menu entry; the route stays for file access.
  await page.goto("/attachments");
  await page.getByRole("button", { name: "Upload file", exact: true }).click();
  const upload = page.getByRole("dialog");
  const filename = `e2e-${Date.now()}.txt`;
  await upload.getByLabel("File name", { exact: true }).setInputFiles({
    name: filename,
    mimeType: "text/plain",
    buffer: Buffer.from("Private attachment test"),
  });
  await upload
    .getByRole("button", { name: "Upload file", exact: true })
    .click();
  await expect(upload).not.toBeVisible();
  const row = page.getByRole("row").filter({ hasText: filename });
  await expect(row).toBeVisible();
  const download = page.waitForEvent("download");
  await row.getByRole("button", { name: "Download", exact: true }).click();
  expect((await download).suggestedFilename()).toBe(filename);
  await page.getByRole("link", { name: "Message center", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Notification center", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("combobox", { name: "Language", exact: true })
    .selectOption("zh-CN");
  await expect(
    page.getByRole("heading", { name: "通知中心", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("messages-dark-zh.png"),
    fullPage: true,
  });
  expect(errors).toEqual([]);
});
