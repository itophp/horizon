import { promisify } from "node:util";
import { test, expect } from "@playwright/test";
import { spawn, execFile, type ChildProcess } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const nativeAcceptance = process.env.MCP_NATIVE_ACCEPTANCE === "1";
const root = fileURLToPath(new URL("../../../", import.meta.url));
async function startHelper(
  home: string,
): Promise<{ process: ChildProcess; key: string }> {
  const child = spawn(
    process.execPath,
    [resolve(root, "scripts/mcp-business-helper.mjs")],
    {
      cwd: root,
      env: {
        ...process.env,
        MCP_BUSINESS_URL: "http://127.0.0.1:5179/mcp",
        MCP_CLIENT_HOME: home,
      },
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  return new Promise((resolve, reject) => {
    let text = "";
    const timer = setTimeout(() => {
      child.kill();
      reject(new Error("helper_start_timeout"));
    }, 15000);
    child.once("exit", () => {
      clearTimeout(timer);
      reject(new Error("helper_start_failed"));
    });
    child.stdout!.on("data", (data) => {
      text += String(data);
      const found = text.match(/Pairing key: ([a-f0-9]{64})/);
      if (found) {
        clearTimeout(timer);
        resolve({ process: child, key: found[1] });
      }
    });
  });
}
test("personal token, protocol diagnostic, client installation and independent revocation", async ({
  page,
}, testInfo) => {
  test.setTimeout(360000);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const home = await mkdtemp(resolve(tmpdir(), "business-mcp-e2e-"));
  const helper = await startHelper(home);
  let bridge: Client | undefined;
  let native:
    | {
        call: () => Promise<{ isError?: boolean | null; content: unknown[] }>;
        close: () => void;
      }
    | undefined;
  try {
    const login = await page.request.post("/api/auth/login", {
      headers: { Origin: "http://127.0.0.1:5179" },
      data: { username: "admin", password: "TestOnly!2026Secure" },
    });
    expect(login.status()).toBe(200);
    await page.goto("/mcp-settings");
    await expect(
      page.getByRole("heading", { name: "Agent collaboration", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("tab", { name: "Remote MCP", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await page.getByRole("tab", { name: "WebSkills", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Page skills", exact: true }),
    ).toBeVisible();
    await page.getByRole("tab", { name: "Remote MCP", exact: true }).click();
    await expect(page.getByLabel("Token name", { exact: true })).toBeVisible();
    await page
      .getByLabel("Token name", { exact: true })
      .fill("Browser integration");
    await page
      .getByLabel("Current password", { exact: true })
      .fill("TestOnly!2026Secure");
    await page
      .getByRole("button", { name: "Create token", exact: true })
      .click();
    const secretNode = page
      .getByRole("status")
      .filter({ has: page.locator("code") });
    await expect(secretNode).toBeVisible();
    const secret = await secretNode.locator("code").innerText();
    expect(secret).toMatch(/^bmcp_[a-f0-9]{64}$/);
    await expect(
      page.getByLabel("Current password", { exact: true }),
    ).toHaveValue("");
    await page
      .getByRole("button", { name: "I have saved the token", exact: true })
      .click();
    await expect(page.getByText(secret, { exact: true })).toHaveCount(0);
    await page
      .getByLabel("MCP Token for this connection", { exact: true })
      .fill(secret);
    await page
      .getByRole("button", { name: "Test remote connection", exact: true })
      .click();
    await expect(page.getByRole("status")).toHaveText(
      "Remote initialization, discovery and tool call succeeded.",
    );
    await expect(
      page.getByLabel("MCP Token for this connection", { exact: true }),
    ).toHaveValue("");
    for (const client of nativeAcceptance
      ? ["Codex", "WorkBuddy"]
      : ["WorkBuddy"]) {
      await page
        .getByLabel("Local helper pairing key", { exact: true })
        .fill(helper.key);
      await page
        .getByLabel("MCP Token for this connection", { exact: true })
        .fill(secret);
      await page
        .getByRole("button", { name: `Set up ${client}`, exact: true })
        .click();
      await expect(page.getByRole("dialog")).toBeVisible();
      await page
        .getByRole("button", { name: "Confirm installation", exact: true })
        .click();
      await expect(page.getByRole("status")).toHaveText(
        "Installed. Restart the target client and verify it.",
      );
    }
    if (nativeAcceptance) {
      const codex = await readFile(resolve(home, ".codex/config.toml"), "utf8");
      expect(codex).toContain("baseline-business");
      expect(codex).not.toContain(secret);
      const nativeCheckPath = resolve(
        root,
        "scripts/mcp-business-codex-check.mjs",
      );
      const { nativeCodexConnection } = await import(nativeCheckPath);
      native = await nativeCodexConnection(home);
      expect((await native!.call()).isError).not.toBe(true);
    }
    const workbuddy = JSON.parse(
      await readFile(resolve(home, ".workbuddy/mcp.json"), "utf8"),
    );
    expect(JSON.stringify(workbuddy)).not.toContain(secret);
    if (nativeAcceptance) {
      if (!process.env.WORKBUDDY_CLI)
        throw new Error("WORKBUDDY_CLI is required for native acceptance");
      const checked = await promisify(execFile)(
        process.execPath,
        [process.env.WORKBUDDY_CLI, "mcp", "list"],
        {
          timeout: 30000,
          env: {
            ...process.env,
            WORKBUDDY_CONFIG_DIR: resolve(home, ".workbuddy"),
            CODEBUDDY_CONFIG_DIR: resolve(home, ".workbuddy"),
            DISABLE_GALILEO: "1",
          },
        },
      );
      expect(checked.stdout).toContain("baseline-business");
      expect(checked.stdout).toContain("Connected");
      expect(checked.stdout).not.toContain(secret);
    }
    // The installed WorkBuddy stdio configuration is executed with an actual MCP client.
    const config = workbuddy.mcpServers["baseline-business"] as {
      command: string;
      args: string[];
    };
    bridge = new Client({ name: "installed-business-e2e", version: "1.0.0" });
    await bridge.connect(
      new StdioClientTransport({ command: config.command, args: config.args }),
    );
    expect((await bridge.listTools()).tools.map((tool) => tool.name)).toEqual(
      expect.arrayContaining(["context_get", "capabilities_list"]),
    );
    await page.goto("/");
    expect(
      (await bridge.callTool({ name: "context_get", arguments: {} })).isError,
    ).not.toBe(true);
    await page.goto("/mcp-settings");
    await expect(page.getByText(secret, { exact: true })).toHaveCount(0);
    await page
      .getByLabel("Current password", { exact: true })
      .fill("TestOnly!2026Secure");
    const rotateResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith("/rotate") &&
        response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Rotate", exact: true }).click();
    expect((await rotateResponse).status()).toBe(201);
    await expect(secretNode).toBeVisible();
    const replacement = await secretNode.locator("code").innerText();
    expect(replacement).not.toBe(secret);
    const oldCall = await bridge
      .callTool({ name: "context_get", arguments: {} })
      .catch(() => null);
    expect(oldCall === null || oldCall.isError).toBeTruthy();
    await page
      .getByRole("button", { name: "I have saved the token", exact: true })
      .click();
    await page
      .getByLabel("MCP Token for this connection", { exact: true })
      .fill(replacement);
    await page
      .getByRole("button", { name: "Test remote connection", exact: true })
      .click();
    await expect(page.getByRole("status")).toHaveText(
      "Remote initialization, discovery and tool call succeeded.",
    );
    await page
      .getByLabel("Current password", { exact: true })
      .fill("TestOnly!2026Secure");
    await page.getByRole("button", { name: "Revoke", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Revoked", exact: true }),
    ).toHaveCount(2);
    await page
      .getByLabel("MCP Token for this connection", { exact: true })
      .fill(replacement);
    await page
      .getByRole("button", { name: "Test remote connection", exact: true })
      .click();
    await expect(page.getByRole("status")).toHaveText(
      "Connection or installation failed. Check the helper, Token and service. Installation may have updated local files; restore the .backup files if needed.",
    );
    await page
      .getByRole("heading", { name: "Agent collaboration", exact: true })
      .scrollIntoViewIfNeeded();
    await page.screenshot({
      path: testInfo.outputPath("mcp-settings-theme-one.png"),
    });
    await page
      .getByRole("button", { name: "Switch appearance", exact: true })
      .click();
    await expect(
      page.getByRole("heading", { name: "Agent collaboration", exact: true }),
    ).toBeVisible();
    await page.screenshot({
      path: testInfo.outputPath("mcp-settings-theme-two.png"),
    });
    expect(errors).toEqual([]);
  } finally {
    native?.close();
    await bridge?.close();
    helper.process.kill("SIGTERM");
    await new Promise<void>((resolve) => {
      if (helper.process.exitCode !== null) resolve();
      else helper.process.once("exit", () => resolve());
    });
    await rm(home, { recursive: true, force: true });
  }
});
