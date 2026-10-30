import { test, expect } from "@playwright/test";
test("real API → SDK → UI, validation, language persistence and navigation @smoke", async ({
  page,
}, testInfo) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("requestfailed", (request) =>
    errors.push(
      `${request.method()} ${request.url()} ${request.failure()?.errorText}`,
    ),
  );
  const responses: string[] = [];
  page.on("response", (response) => {
    if (response.url().includes("/api/"))
      responses.push(`${response.status()} ${response.url()}`);
  });
  try {
    await page.goto("/foundation");
    await expect(
      page.getByRole("heading", { name: "A foundation to build on." }),
    ).toBeVisible();
    await expect(page.getByText("API is healthy")).toBeVisible();
    await page.getByRole("button", { name: "Send request" }).click();
    await expect(page.getByRole("alert")).toHaveText(
      "Enter between 1 and 80 characters.",
    );
    await page.getByRole("textbox", { name: "Name", exact: true }).fill("Ada");
    await page.getByRole("button", { name: "Send request" }).click();
    await expect(page.getByText("Hello, Ada.")).toBeVisible();
    await page
      .getByRole("combobox", { name: "Language", exact: true })
      .selectOption("zh-CN");
    await expect(
      page.getByRole("heading", { name: "从可靠的基础开始。" }),
    ).toBeVisible();
    await expect(page.getByText("你好，Ada。")).toBeVisible();
    await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "从可靠的基础开始。" }),
    ).toBeVisible();
    await expect(page.getByText("2026年1月15日 12:30")).toBeVisible();
    await expect(page.getByText("API 运行正常")).toBeVisible();
    await page.getByRole("link", { name: "架构说明" }).click();
    await expect(page.getByRole("heading", { name: "工程边界" })).toBeVisible();
    expect(errors).toEqual([]);
    expect(
      responses.some(
        (line) => line.includes("200") && line.includes("/api/echo?name=Ada"),
      ),
    ).toBe(true);
  } finally {
    await testInfo.attach("browser-diagnostics", {
      body: JSON.stringify({ errors, responses }, null, 2),
      contentType: "application/json",
    });
  }
});
test("mobile layout fits the viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/foundation");
  await expect(page.getByText("API is healthy")).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
