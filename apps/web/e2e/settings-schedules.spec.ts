import { test, expect } from "@playwright/test";
import type { LoginResult } from "../src/api/generated/models";
import { resetPreferences } from "./helpers";

test("system settings schedules and cancels a saved draft without changing live branding", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(360000);
  const origin = process.env.E2E_ORIGIN ?? "http://127.0.0.1:5179";
  const login = await request.post("/api/auth/login", {
    headers: { Origin: origin },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(login.ok()).toBeTruthy();
  const result: LoginResult = await login.json();
  await resetPreferences(request, {
    Origin: origin,
    "X-CSRF-Token": result.profile.csrfToken,
  });
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.addInitScript(() =>
    localStorage.setItem("baseline.language", "en"),
  );
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill("admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page
    .getByRole("link", { name: "System settings", exact: true })
    .click();
  const unique = Date.now();
  const name = `Timed brand ${unique}`;
  const reason = `Scheduled publication ${unique}`;
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
  await draft
    .getByRole("button", { name: "Schedule publication", exact: true })
    .click();
  const dialog = page.getByRole("dialog", {
    name: "Schedule publication",
    exact: true,
  });
  await expect(
    dialog.getByLabel("Publish at", { exact: true }),
  ).not.toHaveValue("");
  await expect(dialog).toContainText("Asia/Shanghai");
  await dialog.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(dialog).not.toBeVisible();
  const row = page.getByRole("row").filter({ hasText: reason });
  await expect(row.getByText("Pending", { exact: true })).toBeVisible();
  await expect(
    draft.getByRole("button", { name: "Publish", exact: true }),
  ).toBeDisabled();
  await expect(page.locator(".console-brand strong")).not.toHaveText(name);
  await page.reload();
  await expect(row.getByText("Pending", { exact: true })).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("settings-schedule-pending-dark.png"),
    fullPage: true,
  });
  await row
    .getByRole("button", { name: "Cancel schedule", exact: true })
    .click();
  await expect(row.getByText("Cancelled", { exact: true })).toBeVisible();
  await expect(
    draft.getByRole("button", { name: "Publish", exact: true }),
  ).toBeEnabled();
  await expect(page.locator(".console-brand strong")).not.toHaveText(name);
  expect(errors).toEqual([]);
});
