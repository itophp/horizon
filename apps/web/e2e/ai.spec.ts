import { test, expect, type Page } from "@playwright/test";

// The AI workbench is a first class console surface: it must publish the
// capability catalog and explain a disabled model instead of failing silently.
async function signIn(page: Page) {
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill("admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  // The workbench is the default surface, so signing in already lands here.
  await expect(
    page.getByRole("heading", { name: "AI workbench", exact: true }),
  ).toBeVisible();
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("baseline.language", "en");
  });
});

test("workbench publishes policies and reports a disabled model", async ({
  page,
}, testInfo) => {
  const errors: string[] = [];
  const consoleErrors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") consoleErrors.push(message.text());
  });

  await signIn(page);
  await page.getByRole("link", { name: "AI workbench", exact: true }).click();
  await expect(page).toHaveURL(/\/ai$/);
  await expect(
    page.getByRole("heading", { name: "AI workbench", exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Agent disabled")).toBeVisible();
  await expect(page.getByRole("tab", { name: "Workbench" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Execution log" })).toBeVisible();

  // Without a published model the run is refused with a stable message rather
  // than an empty answer.
  await page
    .getByLabel("Ask the agent")
    .fill("Show me the notifications in my inbox");
  await page.getByRole("button", { name: "Send", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("not ready");

  await page.getByRole("tab", { name: "Execution log" }).click();
  await expect(page).toHaveURL(/tab=runs/);
  await expect(page.getByText("No records to show")).toBeVisible();

  expect(errors).toEqual([]);
  // The session bootstrap returns 401 once and the disabled model returns 503;
  // both are expected network statuses, not script failures.
  expect(
    consoleErrors.filter((text) => !text.includes("Failed to load resource")),
  ).toEqual([]);
  await testInfo.attach("ai-diagnostics", {
    body: JSON.stringify({ errors, consoleErrors }),
    contentType: "application/json",
  });
});
