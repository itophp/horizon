import { test, expect } from "@playwright/test";
import type { LoginResult } from "../src/api/generated/models";

// 通知通过接口落地（含私有附件），浏览器只验证通知中心的读取体验：
// 列表只显示标题与时间、点击进入详情、已发送正文默认收起。
const origin = "http://127.0.0.1:5179";

test("notification center lists title and time, opens the detail page and keeps sent bodies collapsed", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(240000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const login = await request.post("/api/auth/login", {
    headers: { Origin: origin },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(login.ok()).toBeTruthy();
  const session: LoginResult = await login.json();
  const headers = { Origin: origin, "X-CSRF-Token": session.profile.csrfToken };
  const reset = await request.put("/api/auth/preferences", {
    headers,
    data: {
      changes: [
        { key: "display.language", operation: "reset", value: "" },
        { key: "display.theme", operation: "reset", value: "" },
      ],
    },
  });
  expect(reset.ok()).toBeTruthy();
  const unique = Date.now();
  const title = `Operations notice ${unique}`;
  const filename = `private-notice-${unique}.txt`;
  const upload = await request.post("/api/admin/files", {
    headers,
    data: {
      name: filename,
      purpose: "attachment",
      content: Buffer.from("Notification private file").toString("base64"),
    },
  });
  expect(upload.ok()).toBeTruthy();
  const fileId = ((await upload.json()) as { id: number }).id;
  const send = await request.post("/api/admin/notifications/send", {
    headers,
    data: {
      clientKey: crypto.randomUUID(),
      title,
      markdown: `## Service notice\n\n**Private file** for our team.\n\n| Window | State |\n| --- | --- |\n| Today | Ready |\n\n[Download](file:${fileId})`,
      fileIds: [fileId],
      email: false,
      targets: [{ kind: "account", id: session.profile.account.id }],
    },
  });
  expect(send.ok()).toBeTruthy();
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
  await page.getByRole("link", { name: "Message center", exact: true }).click();
  const row = page.locator(".gov-message").filter({ hasText: title });
  await expect(
    row.getByRole("link", { name: new RegExp(title) }),
  ).toBeVisible();
  await expect(row.locator("time")).toBeVisible();
  await expect(row).not.toContainText("Service notice");
  await page.screenshot({
    path: testInfo.outputPath("notification-inbox-dark.png"),
    fullPage: true,
  });
  await row.getByRole("link").click();
  await expect(page).toHaveURL(/\/notifications\/\d+$/);
  await expect(
    page.getByRole("heading", { level: 1, name: title, exact: true }),
  ).toBeVisible();
  const detail = page.locator(".gov-message-detail");
  await expect(
    detail.getByRole("heading", { name: "Service notice", exact: true }),
  ).toBeVisible();
  await expect(detail.getByRole("table")).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("notification-detail-dark.png"),
    fullPage: true,
  });
  const downloading = page.waitForEvent("download");
  await detail.getByRole("button", { name: filename, exact: true }).click();
  expect((await downloading).suggestedFilename()).toBe(filename);
  await page.getByRole("button", { name: "Mark as read", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Read", exact: true }),
  ).toBeDisabled();
  await page.getByRole("link", { name: "Back to inbox", exact: true }).click();
  await page.getByRole("tab", { name: "Sent", exact: true }).click();
  const sentRow = page.locator(".gov-sent").filter({ hasText: title });
  await expect(sentRow).toBeVisible();
  const body = sentRow.getByRole("heading", {
    name: "Service notice",
    exact: true,
  });
  await expect(body).toBeHidden();
  await page.screenshot({
    path: testInfo.outputPath("notification-sent-dark.png"),
    fullPage: true,
  });
  await sentRow.getByText("View content", { exact: true }).click();
  await expect(body).toBeVisible();
  await page
    .getByRole("tab", { name: "Delivery records", exact: true })
    .click();
  await expect(
    page.getByRole("row").filter({ hasText: title }).first(),
  ).toBeVisible();
  expect(errors).toEqual([]);
});
