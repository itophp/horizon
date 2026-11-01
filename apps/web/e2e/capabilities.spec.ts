import { test, expect, type Page } from "@playwright/test";

async function signIn(page: Page) {
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill("admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  // Sign in lands on the default workbench (AI), so wait for the console shell.
  await expect(
    page.getByRole("navigation", { name: "Main navigation" }),
  ).toBeVisible();
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("baseline.language", "en");
  });
});

test("console button drives the capability loop and the AI trigger", async ({
  page,
}, testInfo) => {
  test.setTimeout(180000);
  const errors: string[] = [];
  const consoleErrors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });

  await signIn(page);
  await page
    .getByRole("link", { name: "Business capabilities", exact: true })
    .click();
  await expect(page).toHaveURL(/\/capabilities$/);
  await expect(
    page.getByRole("heading", { name: "Business capabilities", exact: true }),
  ).toBeVisible();

  const title = `E2E record ${Date.now()}`;
  await page.getByLabel("Record title").fill(title);
  await page.getByRole("button", { name: "Create record" }).click();
  await expect(page.getByText(/Record #\d+ created/)).toBeVisible();
  await expect(page.getByRole("cell", { name: title })).toBeVisible();

  // The business write and its outbox event commit together; the relay then
  // publishes the event and the trigger engine records the AI outcome.
  await expect(page.getByText("example.record.created").first()).toBeVisible({
    timeout: 45000,
  });
  await expect(page.getByText("AI skipped").first()).toBeVisible({
    timeout: 45000,
  });

  const row = page.getByRole("row", { name: new RegExp(title) });
  await row.getByRole("button", { name: "Archive" }).click();
  await expect(row.getByText("Archived")).toBeVisible();

  await expect(
    page.getByRole("cell", { name: "example.record.archive@1.0.0" }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "High risk" }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "Confirm once" }).first(),
  ).toBeVisible();

  expect(errors).toEqual([]);
  expect(
    consoleErrors.filter((text) => !text.includes("Failed to load resource")),
  ).toEqual([]);
  await testInfo.attach("capability-diagnostics", {
    body: JSON.stringify({ errors, consoleErrors }),
    contentType: "application/json",
  });
});
