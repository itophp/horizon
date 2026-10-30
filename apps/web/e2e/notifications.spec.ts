import { test, expect } from "@playwright/test";
import type { LoginResult, InboxMessage } from "../src/api/generated/models";

test("notification center sends Markdown with an uploaded private file to mixed recipients", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(360000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const login = await request.post("/api/auth/login", {
    headers: { Origin: process.env.E2E_ORIGIN ?? "http://127.0.0.1:5179" },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(login.ok()).toBeTruthy();
  const session: LoginResult = await login.json();
  const reset = await request.put("/api/auth/preferences", {
    headers: {
      Origin: process.env.E2E_ORIGIN ?? "http://127.0.0.1:5179",
      "X-CSRF-Token": session.profile.csrfToken,
    },
    data: {
      changes: [
        { key: "display.language", operation: "reset", value: "" },
        { key: "display.theme", operation: "reset", value: "" },
      ],
    },
  });
  expect(reset.ok()).toBeTruthy();
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
    .getByRole("link", { name: "Notification center", exact: true })
    .click();
  await page.getByRole("tab", { name: "Compose", exact: true }).click();
  const unique = Date.now();
  const title = `Operations notice ${unique}`;
  const filename = `private-notice-${unique}.txt`;
  await page
    .getByRole("textbox", { name: "Notification title", exact: true })
    .fill(title);
  await page
    .getByRole("textbox", { name: "Search recipients", exact: true })
    .fill("Administrator");
  await page.getByRole("checkbox", { name: /Administrator/ }).check();
  await page
    .getByRole("combobox", { name: "Recipient type", exact: true })
    .selectOption("company");
  await page
    .getByRole("textbox", { name: "Search recipients", exact: true })
    .fill("Horizon Technology");
  await page.getByRole("checkbox", { name: /Horizon Technology/ }).check();
  await expect(
    page.getByText("2 targets selected", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("textbox", { name: "Markdown body", exact: true })
    .fill(
      "# Service notice\n\n**Private file** for our team.\n\n| Window | State |\n| --- | --- |\n| Today | Ready |",
    );
  await page
    .getByLabel("Upload and insert file", { exact: true })
    .setInputFiles({
      name: filename,
      mimeType: "text/plain",
      buffer: Buffer.from("Notification private file"),
    });
  await expect(
    page.getByRole("textbox", { name: "Markdown body", exact: true }),
  ).toHaveValue(/file:\d+/);
  await page.screenshot({
    path: testInfo.outputPath("notification-compose-dark.png"),
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Review and send", exact: true })
    .click();
  const dialog = page.getByRole("dialog", {
    name: "Confirm notification",
    exact: true,
  });
  await expect(dialog).toContainText("unique active accounts");
  await dialog
    .getByRole("button", { name: "Send notification", exact: true })
    .click();
  await expect(
    page.getByRole("tab", { name: "Sent", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await expect(
    page.getByRole("heading", { name: title, exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Inbox", exact: true }).click();
  const inbox = await page.request.get("/api/auth/notifications");
  const messages: InboxMessage[] = await inbox.json();
  const sentMessage = messages.find((n) => n.title === title);
  expect(sentMessage).toBeDefined();
  await page.goto(`/notifications?message=${sentMessage!.id}`);
  const message = page.locator(".gov-message").filter({ hasText: title });
  await expect(
    message.getByRole("heading", { name: "Service notice", exact: true }),
  ).toBeVisible();
  await expect(message.getByRole("table")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("notification-inbox-dark.png"),
    fullPage: true,
  });
  const downloading = page.waitForEvent("download");
  await message
    .getByRole("button", { name: filename, exact: true })
    .first()
    .click();
  expect((await downloading).suggestedFilename()).toBe(filename);
  await message
    .getByRole("button", { name: "Mark as read", exact: true })
    .click();
  await expect(
    message.getByRole("button", { name: "Read", exact: true }),
  ).toBeDisabled();
  await page
    .getByRole("tab", { name: "Delivery records", exact: true })
    .click();
  await expect(
    page.getByRole("row").filter({ hasText: title }).first(),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("notification-delivery-dark.png"),
    fullPage: true,
  });
  await page
    .getByRole("link", { name: "System settings", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Delivery records", exact: true }),
  ).toHaveCount(0);
  expect(errors).toEqual([]);
});
