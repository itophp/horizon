import { ToolListChangedNotificationSchema } from "@modelcontextprotocol/sdk/types.js";
import { test, expect } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import { fileURLToPath } from "node:url";

test("real WebMCP connection follows routes, limits account tools and revokes on disconnect", async ({
  page,
}, testInfo) => {
  test.setTimeout(240000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const login = await page.request.post("/api/auth/login", {
    headers: { Origin: "http://127.0.0.1:5179" },
    data: { username: "admin", password: "TestOnly!2026Secure" },
  });
  expect(login.ok()).toBe(true);
  await page.goto("/webmcp");
  await expect(
    page.getByRole("heading", { name: "MCP & WebSkills" }),
  ).toBeVisible();
  // The legacy /webmcp entry redirects into the WebMCP tab of the unified page.
  await expect(
    page.getByRole("tab", { name: "WebMCP", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await page.getByRole("button", { name: "Connect page", exact: true }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "Enter the 64-character pairing key printed by the local service.",
  );
  const file = new URL(
    "../../../.local/webmcp-e2e/connection.json",
    import.meta.url,
  );
  const credential: { token: string; sessionId?: string } = JSON.parse(
    await readFile(file, "utf8"),
  );
  await page.getByLabel("Local WebMCP port").fill("18794");
  await page
    .getByLabel("Pairing key from the local terminal")
    .fill(credential.token);
  await page.getByRole("button", { name: "Connect page", exact: true }).click();
  await expect(page.getByRole("status")).toHaveText("Connected");
  await expect(
    page.getByLabel("Pairing key from the local terminal"),
  ).toHaveValue("");
  const client = new Client({ name: "baseline-e2e", version: "1.0.0" });
  let toolChanges = 0;
  client.setNotificationHandler(ToolListChangedNotificationSchema, () => {
    toolChanges += 1;
  });
  try {
    await client.connect(
      new StdioClientTransport({
        command: process.execPath,
        args: [
          fileURLToPath(
            new URL("../../../scripts/webmcp-stdio.mjs", import.meta.url),
          ),
        ],
        env: {
          ...Object.fromEntries(
            Object.entries(process.env).filter(
              (entry): entry is [string, string] => entry[1] !== undefined,
            ),
          ),
          WEBMCP_STATE_DIR: ".local/webmcp-e2e",
        },
      }),
    );
    expect((await client.listTools()).tools.map((tool) => tool.name)).toContain(
      "baseline_navigate",
    );
    const context = await client.callTool({
      name: "baseline_page_context",
      arguments: {},
    });
    expect(JSON.stringify(context)).toContain("/mcp");
    expect(JSON.stringify(context)).not.toContain(credential.token);
    const moduleContext = await client.callTool({
      name: "baseline_module_context",
      arguments: {},
    });
    expect(JSON.stringify(moduleContext)).toContain("development");
    const currentPage = await client.callTool({
      name: "baseline_current_page",
      arguments: {},
    });
    expect(JSON.stringify(currentPage)).toContain("/mcp");
    const rejected = await client.callTool({
      name: "baseline_navigate",
      arguments: { path: "https://evil.test" },
    });
    expect(rejected.isError).toBe(true);
    const beforeNavigation = toolChanges;
    await client.callTool({
      name: "baseline_navigate",
      arguments: { path: "/accounts" },
    });
    await expect(
      page.getByRole("heading", { name: "Account management" }),
    ).toBeVisible();
    await expect
      .poll(async () =>
        (await client.listTools()).tools.map((tool) => tool.name),
      )
      .toContain("baseline_accounts_filter");
    await expect.poll(() => toolChanges).toBeGreaterThan(beforeNavigation);
    await expect
      .poll(async () =>
        JSON.stringify(
          await client.callTool({
            name: "baseline_module_context",
            arguments: {},
          }),
        ),
      )
      .toContain("administration");
    const filtered = await client.callTool({
      name: "baseline_accounts_filter",
      arguments: { query: "admin", status: "active" },
    });
    expect(filtered.isError).toBeUndefined();
    await expect(page).toHaveURL(/q=admin/);
    const outOfScope = await client.callTool({
      name: "baseline_accounts_details",
      arguments: { id: 2147483647 },
    });
    expect(outOfScope.isError).toBe(true);
    const diagnostic = await client.callTool({
      name: "baseline_diagnostics",
      arguments: {},
    });
    expect(JSON.stringify(diagnostic)).not.toContain("csrfToken");
    for (const item of [
      { path: "/organizations", tool: "baseline_organizations_filter" },
      { path: "/departments", tool: "baseline_departments_filter" },
      { path: "/roles", tool: "baseline_roles_filter" },
    ]) {
      await client.callTool({
        name: "baseline_navigate",
        arguments: { path: item.path },
      });
      await expect(page.getByLabel("Search this list…")).toBeVisible();
      await expect
        .poll(async () =>
          (await client.listTools()).tools.map((tool) => tool.name),
        )
        .toContain(item.tool);
      const filteredPage = await client.callTool({
        name: item.tool,
        arguments: { query: "example" },
      });
      expect(filteredPage.isError).toBeUndefined();
      await expect(page.getByLabel("Search this list…")).toHaveValue("example");
      expect(
        (
          await client.callTool({
            name: item.tool,
            arguments: { query: "x".repeat(101) },
          })
        ).isError,
      ).toBe(true);
    }
    await client.callTool({
      name: "baseline_navigate",
      arguments: { path: "/webmcp" },
    });
    await expect(
      page.getByRole("heading", { name: "MCP & WebSkills" }),
    ).toBeVisible();
    await expect
      .poll(async () =>
        (await client.listTools()).tools.map((tool) => tool.name),
      )
      .not.toContain("baseline_accounts_filter");
    await page.screenshot({
      path: testInfo.outputPath("webmcp-dark.png"),
      fullPage: true,
    });
    await page.getByRole("button", { name: "Switch appearance" }).click();
    await page.screenshot({
      path: testInfo.outputPath("webmcp-light.png"),
      fullPage: true,
    });
    await page.getByRole("button", { name: "Disconnect", exact: true }).click();
    await expect(page.getByRole("status")).toHaveText("Disconnected");
    await expect(
      page.getByRole("button", { name: "Set up MCP + Skills" }).first(),
    ).toBeDisabled();
    const tools = await page.evaluate(async () => {
      const context = (
        document as Document & {
          modelContext: { getTools: () => Promise<{ name: string }[]> };
        }
      ).modelContext;
      return (await context.getTools()).filter((tool) =>
        tool.name.startsWith("baseline_"),
      );
    });
    expect(tools).toEqual([]);
    expect(errors).toEqual([]);
  } finally {
    await client.close();
  }
});
