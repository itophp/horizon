import type {
  AccountRecord,
  AdminSnapshot,
  LoginResult,
  TemporaryAccessRecord,
  TemporaryAccessWorkspace,
} from "../src/api/generated/models";
import {
  test,
  expect,
  type Page,
  type APIRequestContext,
} from "@playwright/test";

async function signIn(page: Page, heading = "Workspace overview") {
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill("admin");
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: heading, exact: true }),
  ).toBeVisible();
}
const diagnostics = new WeakMap<
  Page,
  { console: string[]; failed: string[] }
>();
test.afterEach(async ({ page }, testInfo) => {
  await testInfo.attach("console-network-diagnostics", {
    body: JSON.stringify(diagnostics.get(page)),
    contentType: "application/json",
  });
});
test.beforeEach(async ({ page, request }) => {
  const response = await request.post("/api/auth/login", {
    headers: { Origin: "http://127.0.0.1:5179" },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(response.ok()).toBeTruthy();
  const session: LoginResult = await response.json();
  const reset = await request.put("/api/auth/preferences", {
    headers: {
      Origin: "http://127.0.0.1:5179",
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
  const diagnostic = { console: [] as string[], failed: [] as string[] };
  diagnostics.set(page, diagnostic);
  page.on("console", (message) => {
    if (message.type() === "error")
      diagnostic.console.push(`${message.location().url}: ${message.text()}`);
  });
  page.on("requestfailed", (request) =>
    diagnostic.failed.push(
      `${request.method()} ${request.url()} ${request.failure()?.errorText}`,
    ),
  );
  await page.addInitScript(() => {
    localStorage.setItem("baseline.language", "en");
  });
});
test("session protection, compact login, theme persistence and logout", async ({
  page,
}, testInfo) => {
  test.setTimeout(180000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/accounts");
  await expect(page).toHaveURL(/\/login$/);
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("alert").first()).toBeVisible();
  await expect(page.getByLabel("Password", { exact: true })).toHaveAttribute(
    "type",
    "password",
  );
  await signIn(page, "Account management");
  await expect(page).toHaveURL(/\/accounts$/);
  await page.getByRole("link", { name: "Overview", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Workspace overview", exact: true }),
  ).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.getByRole("button", { name: "Switch appearance" }).click();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Workspace overview", exact: true }),
  ).toBeVisible();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  await page.locator(".sidebar-profile").click();
  await page.getByRole("menuitem", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.goto("/roles");
  await expect(
    page.getByRole("heading", { name: "Welcome back" }),
  ).toBeVisible();
  expect(errors).toEqual([]);
  await testInfo.attach("page-errors", {
    body: JSON.stringify(errors),
    contentType: "application/json",
  });
});
test("organization, department, role and account changes persist through real API", async ({
  page,
}, testInfo) => {
  // This host's installed Chrome has ~3s per operation; the full cross-page CRUD journey is bounded separately.
  test.setTimeout(300000);
  const unique = Date.now().toString(36);
  const enterpriseName = `Acceptance Enterprise ${unique}`;
  const departmentName = `Acceptance Team ${unique}`;
  const roleName = `Acceptance Reader ${unique}`;
  const accountName = `Acceptance User ${unique}`;
  const updatedName = `Acceptance Updated ${unique}`;
  const errors: string[] = [];
  const responses: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("response", (response) => {
    if (response.url().includes("/api/admin/"))
      responses.push(
        `${response.status()} ${response.request().method()} ${response.url()}`,
      );
  });
  try {
    await signIn(page);
    await page
      .getByRole("link", { name: "Organizations", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Create organization", exact: true })
      .click();
    let dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name", { exact: true }).fill(enterpriseName);
    await dialog
      .getByLabel("Code", { exact: true })
      .fill(`acceptance-${unique}-enterprise`);
    await dialog.getByRole("button", { name: "Save changes" }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("row", { name: new RegExp(enterpriseName) }),
    ).toBeVisible();

    await page.getByRole("link", { name: "Departments", exact: true }).click();
    await page
      .getByRole("button", { name: "Create department", exact: true })
      .click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name", { exact: true }).fill(departmentName);
    await dialog
      .getByLabel("Code", { exact: true })
      .fill(`acceptance-${unique}-team`);
    await dialog
      .getByLabel("Company", { exact: true })
      .selectOption({ label: "Horizon Technology" });
    await dialog.getByRole("button", { name: "Save changes" }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("row", { name: new RegExp(departmentName) }),
    ).toBeVisible();

    await page.getByRole("link", { name: "Roles", exact: true }).click();
    await page
      .getByRole("button", { name: "Create role", exact: true })
      .click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name", { exact: true }).fill(roleName);
    await dialog
      .getByLabel("Code", { exact: true })
      .fill(`acceptance-${unique}-reader`);
    await dialog
      .getByRole("checkbox", { name: "Accounts View", exact: true })
      .check();
    await dialog
      .getByRole("combobox", { name: "Data scope", exact: true })
      .selectOption("DEPARTMENT");
    await dialog.getByRole("button", { name: "Save changes" }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("heading", { name: roleName, exact: true }),
    ).toBeVisible();

    await page.getByRole("link", { name: /^Accounts/ }).click();
    await page
      .getByRole("button", { name: "Create account", exact: true })
      .click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Full name", { exact: true }).fill(accountName);
    await dialog
      .getByLabel("Account", { exact: true })
      .fill(`acceptance.${unique}`);
    await dialog
      .getByLabel("Company", { exact: true })
      .selectOption({ label: "Horizon Technology" });
    await dialog
      .getByLabel("Department", { exact: true })
      .selectOption({ label: departmentName });
    await dialog
      .getByLabel("Initial password", { exact: true })
      .fill("Acceptance!2026Secure");
    await dialog
      .getByRole("button", { name: "Departments & permissions", exact: true })
      .click();
    await dialog.getByRole("checkbox", { name: new RegExp(roleName) }).check();
    await dialog
      .getByRole("group", { name: "Accounts View", exact: true })
      .getByRole("button", { name: "Allow", exact: true })
      .click();
    await dialog
      .getByRole("combobox", { name: "Data scope", exact: true })
      .selectOption("OWN");
    await dialog.getByRole("button", { name: "Save changes" }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("row", { name: new RegExp(accountName) }),
    ).toBeVisible();
    await page.reload();
    const row = page.getByRole("row", { name: new RegExp(accountName) });
    await expect(row).toBeVisible();
    await row.getByRole("button", { name: "Actions" }).click();
    await page.getByRole("menuitem", { name: "Edit", exact: true }).click();
    dialog = page.getByRole("dialog");
    await dialog.getByLabel("Full name", { exact: true }).fill(updatedName);
    await dialog
      .getByRole("button", { name: "Departments & permissions", exact: true })
      .click();
    await expect(
      dialog.getByRole("combobox", { name: "Data scope", exact: true }),
    ).toHaveValue("OWN");
    await expect(
      dialog
        .getByRole("group", { name: "Accounts View", exact: true })
        .getByRole("button", { name: "Allow", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    await dialog.getByRole("button", { name: "Save changes" }).click();
    await expect(dialog).not.toBeVisible();
    const updated = page.getByRole("row", { name: new RegExp(updatedName) });
    await expect(updated).toBeVisible();
    await updated.getByRole("button", { name: "Actions" }).click();
    await page.getByRole("menuitem", { name: "Delete", exact: true }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Delete", exact: true })
      .click();
    await expect(updated).not.toBeVisible();
    expect(
      responses.some(
        (line) => line.startsWith("201 POST") && line.includes("/accounts"),
      ),
    ).toBe(true);
    expect(
      responses.some(
        (line) => line.startsWith("200 PUT") && line.includes("/accounts/"),
      ),
    ).toBe(true);
    expect(
      responses.some(
        (line) => line.startsWith("204 DELETE") && line.includes("/accounts/"),
      ),
    ).toBe(true);
    expect(errors).toEqual([]);
  } finally {
    await testInfo.attach("management-diagnostics", {
      body: JSON.stringify({ errors, responses }, null, 2),
      contentType: "application/json",
    });
  }
});

test("workspace panel owns scrolling and keeps its rounded shell stationary", async ({
  page,
}, testInfo) => {
  // Bound the real login, navigation, scroll and theme journey for this host's slow Chrome protocol.
  test.setTimeout(180000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.setViewportSize({ width: 1114, height: 937 });
  await signIn(page);
  await page.getByRole("link", { name: "Permissions", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Permission management", exact: true }),
  ).toBeVisible();
  const panel = page.locator(".console-main");
  const header = page.locator(".console-topbar");
  const content = page.getByRole("main");
  const panelBefore = await panel.boundingBox();
  const headerBefore = await header.boundingBox();
  expect(panelBefore).not.toBeNull();
  expect(headerBefore).not.toBeNull();
  const save = page.getByRole("button", { name: "Save changes", exact: true });
  await save.scrollIntoViewIfNeeded();
  await expect(save).toBeInViewport();
  await expect
    .poll(() => content.evaluate((element) => element.scrollTop))
    .toBeGreaterThan(0);
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
  expect(await panel.boundingBox()).toEqual(panelBefore);
  expect((await header.boundingBox())?.y).toBe(headerBefore?.y);
  await page.screenshot({ path: testInfo.outputPath("workspace-dark.png") });
  await page
    .getByRole("button", { name: "Switch appearance", exact: true })
    .click();
  await expect(page.locator("html")).not.toHaveClass(/dark/);
  expect(await panel.boundingBox()).toEqual(panelBefore);
  expect((await header.boundingBox())?.y).toBe(headerBefore?.y);
  await expect(save).toBeInViewport();
  expect(errors).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath("workspace-light.png") });
});

test("an account without resource permissions can request temporary access", async ({
  page,
  request,
}, testInfo) => {
  test.setTimeout(180000);
  const { account } = await temporaryApplicantFixture(request);
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill(account.username);
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Workspace overview", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("link", { name: /^Accounts/ })).not.toBeVisible();
  await expect(
    page
      .getByRole("navigation")
      .getByRole("link", { name: "Temporary access", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("banner")
    .getByRole("link", { name: "Request temporary access", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Request temporary access", exact: true })
    .click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByLabel("Reason for request", { exact: true })
    .fill("Investigate a scoped support incident");
  await dialog
    .getByRole("combobox", { name: "Validity period", exact: true })
    .selectOption("120");
  await dialog
    .getByRole("combobox", { name: "Data scope", exact: true })
    .selectOption("ALL");
  await dialog
    .getByRole("checkbox", { name: "Accounts View", exact: true })
    .check();
  await expect(
    dialog.getByRole("checkbox", {
      name: "Temporary access approval",
      exact: true,
    }),
  ).not.toBeVisible();
  await dialog.screenshot({
    path: testInfo.outputPath("temporary-request-form.png"),
  });
  await dialog
    .getByRole("button", { name: "Submit request", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  await expect(
    page.getByRole("row", { name: new RegExp(account.fullName) }),
  ).toContainText("Pending");
  const response = await page.request.get("/api/admin/temporary-access");
  expect(response.status()).toBe(200);
  const data = (await response.json()) as TemporaryAccessWorkspace;
  expect(data.canReview).toBe(false);
  expect(data.requests[0].requestedMinutes).toBe(120);
  expect(data.requests[0].requestedGrants).toEqual([
    {
      permission: "accounts:read",
      effect: "allow",
      scope: "ALL",
      departmentIds: [],
    },
  ]);
  await testInfo.attach("temporary-request-id", {
    body: String(data.requests[0].id),
    contentType: "text/plain",
  });
});
test("approver amends temporary authorization and revokes it through the UI", async ({
  page,
  request,
  playwright,
}, testInfo) => {
  test.setTimeout(240000);
  const { account, admin } = await temporaryApplicantFixture(request);
  const applicant = await playwright.request.newContext({
    baseURL: "http://127.0.0.1:5179",
  });
  try {
    const login = await applicant.post("/api/auth/login", {
      headers: { Origin: "http://127.0.0.1:5179" },
      data: { username: account.username, password: "TestOnly!2026Secure" },
    });
    expect(login.status()).toBe(200);
    const loginResult = (await login.json()) as LoginResult;
    const created = await applicant.post("/api/admin/temporary-access", {
      headers: {
        Origin: "http://127.0.0.1:5179",
        "X-CSRF-Token": loginResult.profile.csrfToken,
      },
      data: {
        reason: "Need access to investigate an incident",
        minutes: 120,
        grants: [
          {
            permission: "accounts:read",
            effect: "allow",
            scope: "ALL",
            departmentIds: [],
          },
        ],
      },
    });
    expect(created.status()).toBe(200);
    const record = (await created.json()) as TemporaryAccessRecord;
    await signIn(page);
    await page.getByRole("link", { name: "Permissions", exact: true }).click();
    await expect(
      page.locator("th").getByText("Administration", { exact: true }),
    ).toHaveAttribute("rowspan", "4");
    await page
      .getByRole("banner")
      .getByRole("link", { name: "Request temporary access", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Approval inbox", exact: true })
      .click();
    const row = page.getByRole("row", { name: new RegExp(account.fullName) });
    await row
      .getByRole("button", { name: "Review request", exact: true })
      .click();
    let dialog = page.getByRole("dialog");
    await dialog
      .getByRole("combobox", { name: "Validity period", exact: true })
      .selectOption("30");
    await dialog
      .getByRole("combobox", { name: "Data scope", exact: true })
      .selectOption("DEPARTMENT");
    await dialog
      .getByLabel("Review notes", { exact: true })
      .fill("Department access for 30 minutes only");
    await dialog.screenshot({
      path: testInfo.outputPath("temporary-approval-form.png"),
    });
    await dialog
      .getByRole("button", { name: "Submit decision", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await expect(row).toContainText("Active");
    const workspaceResponse = await applicant.get(
      "/api/admin/temporary-access",
    );
    expect(workspaceResponse.status()).toBe(200);
    const workspace =
      (await workspaceResponse.json()) as TemporaryAccessWorkspace;
    const approved = workspace.requests.find((item) => item.id === record.id);
    expect(approved?.requestedMinutes).toBe(120);
    expect(approved?.requestedGrants[0].scope).toBe("ALL");
    expect(approved?.approvedMinutes).toBe(30);
    expect(approved?.approvedGrants[0].scope).toBe("DEPARTMENT");
    const visibleResponse = await applicant.get("/api/admin/snapshot");
    expect(visibleResponse.status()).toBe(200);
    const snapshot = (await visibleResponse.json()) as AdminSnapshot;
    expect(snapshot.accounts.some((item) => item.id === account.id)).toBe(true);
    expect(snapshot.accounts.some((item) => item.id === admin.account.id)).toBe(
      false,
    );
    await row
      .getByRole("button", { name: "Revoke access", exact: true })
      .click();
    dialog = page.getByRole("dialog");
    await dialog
      .getByRole("button", { name: "Revoke access", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await expect(row).toContainText("Revoked");
    const revokedResponse = await applicant.get("/api/admin/snapshot");
    expect(revokedResponse.status()).toBe(200);
    expect(((await revokedResponse.json()) as AdminSnapshot).accounts).toEqual(
      [],
    );
  } finally {
    await applicant.dispose();
  }
});
async function temporaryApplicantFixture(request: APIRequestContext) {
  const login = await request.post("/api/auth/login", {
    headers: { Origin: "http://127.0.0.1:5179" },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(login.status()).toBe(200);
  const { profile: admin } = (await login.json()) as LoginResult;
  const snapshotResponse = await request.get("/api/admin/snapshot");
  expect(snapshotResponse.status()).toBe(200);
  const snapshot = (await snapshotResponse.json()) as AdminSnapshot;
  const department = snapshot.departments.find(
    (item) =>
      item.organizationId === admin.account.organizationId &&
      item.id !== admin.account.departmentId,
  );
  expect(department).toBeDefined();
  const suffix = Date.now().toString(36);
  const created = await request.post("/api/admin/accounts", {
    headers: {
      Origin: "http://127.0.0.1:5179",
      "X-CSRF-Token": admin.csrfToken,
    },
    data: {
      username: `temporary.${suffix}`,
      fullName: `Temporary applicant ${suffix}`,
      email: "",
      status: "active",
      organizationId: admin.account.organizationId,
      departmentId: department?.id,
      roleIds: [],
      overrides: [],
      password: "TestOnly!2026Secure",
    },
  });
  expect(created.status()).toBe(201);
  const account = (await created.json()) as AccountRecord;
  return { account, admin };
}

test("account department policies persist and switching departments isolates permissions", async ({
  page,
  request,
}, testInfo) => {
  // Local Chrome trace reaches the final switch near 240s; retain every assertion with a bounded 300s budget.
  test.setTimeout(300000);
  const { account, admin } = await temporaryApplicantFixture(request);
  await signIn(page);
  await page.getByRole("link", { name: "Accounts", exact: true }).click();
  const row = page.getByRole("row", { name: new RegExp(account.fullName) });
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Edit", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .getByLabel("Additional department", { exact: true })
    .selectOption({ label: "Horizon Technology / Administration" });
  await dialog
    .getByRole("button", { name: "Add department", exact: true })
    .click();
  await dialog.getByRole("checkbox", { name: /Department viewer/ }).check();
  await dialog
    .getByRole("group", { name: "Accounts View", exact: true })
    .getByRole("button", { name: "Allow", exact: true })
    .click();
  await dialog
    .getByRole("combobox", { name: "Data scope", exact: true })
    .selectOption("DEPARTMENT");
  await dialog.locator(".membership-editor").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("department-membership-editor.png"),
  });
  await dialog.getByRole("button", { name: "Save changes" }).click();
  await expect(dialog).not.toBeVisible();
  await expect(row).toContainText("Administration");
  await page.reload();
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Edit", exact: true }).click();
  await dialog
    .getByRole("button", { name: "Administration", exact: true })
    .click();
  await expect(
    dialog.getByRole("checkbox", { name: /Department viewer/ }),
  ).toBeChecked();
  await expect(
    dialog
      .getByRole("group", { name: "Accounts View", exact: true })
      .getByRole("button", { name: "Allow", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  // A new browser login uses its unprivileged primary department.
  await page.context().clearCookies();
  await page.goto("/login");
  await page
    .getByRole("textbox", { name: "Account", exact: true })
    .fill(account.username);
  await page
    .getByLabel("Password", { exact: true })
    .fill("TestOnly!2026Secure");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Workspace overview", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Accounts", exact: true }),
  ).toHaveCount(0);
  await page
    .getByRole("combobox", { name: "Current department", exact: true })
    .selectOption({ label: "Administration" });
  await expect(
    page.getByRole("link", { name: "Accounts", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Accounts", exact: true }).click();
  await expect(
    page.getByRole("row", { name: /Administrator/ }).first(),
  ).toBeVisible();
  await page.reload();
  const persisted = (await (
    await page.request.get("/api/auth/me")
  ).json()) as LoginResult["profile"];
  expect(persisted.activeDepartmentId).toBe(admin.account.departmentId);
  await expect(
    page.getByRole("combobox", { name: "Current department", exact: true }),
  ).toHaveValue(String(admin.account.departmentId));
  await page
    .getByRole("combobox", { name: "Current department", exact: true })
    .selectOption({ label: "Engineering" });
  await expect(
    page.getByRole("link", { name: "Accounts", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Workspace overview", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("department-context-isolation.png"),
  });
});

test("built-in administrator can edit additional department access while the primary stays protected", async ({
  page,
}, testInfo) => {
  // Local Chrome trace completes save/permission assertions near 180s before the final route check.
  test.setTimeout(240000);
  await signIn(page);
  const me = (await (
    await page.request.get("/api/auth/me")
  ).json()) as LoginResult["profile"];
  const before = (await (
    await page.request.get("/api/admin/snapshot")
  ).json()) as AdminSnapshot;
  const primary = before.accounts.find((a) => a.id === me.account.id);
  const extra = before.departments.find(
    (d) =>
      d.organizationId === me.account.organizationId &&
      d.id !== me.activeDepartmentId,
  );
  expect(primary).toBeDefined();
  expect(extra).toBeDefined();
  await page.getByRole("link", { name: "Accounts", exact: true }).click();
  await page
    .getByRole("row", { name: /Administrator/ })
    .first()
    .getByRole("button", { name: "Actions" })
    .click();
  await expect(
    page.getByRole("menuitem", { name: "Configure access", exact: true }),
  ).toBeVisible();
  await page.getByRole("menuitem", { name: "Edit", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(
    dialog.getByRole("button", { name: "Add department", exact: true }),
  ).toBeVisible();
  await dialog
    .getByRole("button", { name: "Departments & permissions", exact: true })
    .click();
  await expect(
    dialog.getByRole("checkbox", { name: /Department viewer/ }),
  ).toBeDisabled();
  await dialog
    .getByLabel("Additional department", { exact: true })
    .selectOption(String(extra?.id));
  await dialog
    .getByRole("button", { name: "Add department", exact: true })
    .click();
  await dialog.getByRole("checkbox", { name: /Department viewer/ }).check();
  await dialog
    .getByRole("group", { name: "Accounts View", exact: true })
    .getByRole("button", { name: "Allow", exact: true })
    .click();
  await dialog.locator(".membership-editor").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: testInfo.outputPath("administrator-department-entry.png"),
  });
  await dialog
    .getByRole("button", { name: "Save changes", exact: true })
    .click();
  await expect(dialog).not.toBeVisible();
  const after = (await (
    await page.request.get("/api/admin/snapshot")
  ).json()) as AdminSnapshot;
  const updated = after.accounts.find((a) => a.id === me.account.id);
  expect(updated?.departmentId).toBe(primary?.departmentId);
  expect(updated?.roleIds).toEqual(primary?.roleIds);
  expect(updated?.overrides).toEqual(primary?.overrides);
  const membership = updated?.memberships.find(
    (m) => m.departmentId === extra?.id,
  );
  expect(membership?.roleIds).toContain(
    before.roles.find((r) => r.code === "department-viewer")?.id,
  );
  expect(membership?.overrides).toContainEqual({
    permission: "accounts:read",
    effect: "allow",
    scope: "DEPARTMENT",
    departmentIds: [],
  });
  await page.goto(`/permissions?type=account&id=${me.account.id}`);
  await expect(
    page.getByRole("button", { name: "Add department", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: extra?.name, exact: true }).click();
  await expect(
    page.getByRole("checkbox", { name: /Department viewer/ }),
  ).toBeEnabled();
  await expect(
    page.getByRole("checkbox", { name: /Department viewer/ }),
  ).toBeChecked();
});
