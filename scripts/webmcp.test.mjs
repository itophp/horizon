import { normalizeWebMcpResult } from "./webmcp-result.mjs";
import { request as httpRequest } from "node:http";
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  readFile,
  writeFile,
  mkdir,
  readdir,
  rm,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createWebMcpApp, portNumber } from "./webmcp-server.mjs";
import { installClient, skillNames } from "./webmcp-install.mjs";

const token = "a".repeat(64);
async function withServer(fn) {
  const calls = [];
  const handles = {
    clients: { "page-session": {} },
    handleStreamRequest: async (_req, res) => res.json({ proxy: true }),
    handleStreamInspector: async (_req, res) => res.json({ inspector: true }),
  };
  const server = createWebMcpApp({
    token,
    port: 18795,
    origins: ["http://127.0.0.1:5173"],
    handles,
    install: async (client) => {
      calls.push(client);
      return { installed: true };
    },
    bind: async (session) => calls.push(session),
  }).app.listen(18795, "127.0.0.1");
  await new Promise((resolve) => server.once("listening", resolve));
  try {
    await fn(`http://127.0.0.1:18795`, calls);
  } finally {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
}
test("local service rejects missing credentials, hostile origin/host, unknown sessions and clients", async () => {
  await withServer(async (url, calls) => {
    assert.equal((await fetch(`${url}/health`)).status, 200);
    assert.equal((await fetch(`${url}/api/v1/webmcp/mcp`)).status, 401);
    const headers = {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
      Origin: "http://evil.test",
    };
    assert.equal(
      (await fetch(`${url}/setup`, { method: "POST", headers, body: "{}" }))
        .status,
      403,
    );
    headers.Origin = "http://127.0.0.1:5173";
    const preflight = await fetch(`${url}/api/v1/webmcp/mcp`, {
      method: "OPTIONS",
      headers: { Origin: headers.Origin },
    });
    assert.equal(
      preflight.headers.get("access-control-allow-credentials"),
      "true",
    );
    assert.match(
      preflight.headers.get("access-control-allow-headers"),
      /Stream-Session-Id/,
    );

    const hostileHost = await new Promise((resolve, reject) => {
      const request = httpRequest(
        `${url}/setup`,
        { method: "POST", headers: { ...headers, Host: "evil.test:18795" } },
        (response) => {
          response.resume();
          resolve(response.statusCode);
        },
      );
      request.on("error", reject);
      request.end("{}");
    });
    assert.equal(hostileHost, 403);
    for (const data of [
      { client: "other", sessionId: "page-session" },
      { client: "codex", sessionId: "not-connected" },
      { client: "codex", sessionId: "__proto__" },
    ])
      assert.equal(
        (
          await fetch(`${url}/setup`, {
            method: "POST",
            headers,
            body: JSON.stringify(data),
          })
        ).status,
        400,
      );
    for (const path of ["/bind", "/setup"]) {
      assert.equal(
        (
          await fetch(`${url}${path}`, {
            method: "POST",
            headers: {
              Origin: headers.Origin,
              "Content-Type": "application/json",
            },
            body: JSON.stringify({
              sessionId: "page-session",
              client: "codex",
            }),
          })
        ).status,
        401,
      );
    }
    assert.equal(
      (
        await fetch(`${url}/bind`, {
          method: "POST",
          headers: {
            Authorization: headers.Authorization,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ sessionId: "page-session" }),
        })
      ).status,
      403,
    );
    for (const sessionId of ["not-connected", "__proto__"]) {
      assert.equal(
        (
          await fetch(`${url}/bind`, {
            method: "POST",
            headers,
            body: JSON.stringify({ sessionId }),
          })
        ).status,
        400,
      );
    }
    assert.deepEqual(calls, []);
    assert.equal(
      (
        await fetch(`${url}/setup`, {
          method: "POST",
          headers,
          body: JSON.stringify({
            client: "workbuddy",
            sessionId: "page-session",
          }),
        })
      ).status,
      200,
    );
    assert.deepEqual(calls, ["page-session", "workbuddy"]);
    assert.deepEqual(
      await (await fetch(`${url}/api/v1/webmcp/mcp`, { headers })).json(),
      { proxy: true },
    );
    assert.deepEqual(
      await (
        await fetch(`${url}/api/v1/webmcp/mcp?sessionId=page-session`, {
          headers,
        })
      ).json(),
      { inspector: true },
    );
    assert.equal((await fetch(`${url}/list`, { headers })).status, 404);
  });
});
test("installer preserves other WorkBuddy servers, backs up configuration and installs actual skills", async () => {
  const home = await mkdtemp(join(tmpdir(), "baseline-webmcp-"));
  try {
    await mkdir(join(home, ".workbuddy"));
    const config = join(home, ".workbuddy/mcp.json");
    await writeFile(
      config,
      JSON.stringify({
        feature: true,
        mcpServers: { existing: { command: "keep" } },
      }),
    );
    await installClient("workbuddy", { home });
    const data = JSON.parse(await readFile(config, "utf8"));
    assert.equal(data.feature, true);
    assert.equal(data.mcpServers.existing.command, "keep");
    assert.equal(data.mcpServers["baseline-web"].type, "stdio");
    assert.ok(
      (await readdir(join(home, ".workbuddy"))).some((name) =>
        name.startsWith("mcp.json.backup-"),
      ),
    );
    for (const name of skillNames)
      assert.match(
        await readFile(
          join(home, ".workbuddy/skills", name, "SKILL.md"),
          "utf8",
        ),
        /name: baseline-web-/,
      );
    await installClient("workbuddy", { home });
    assert.deepEqual(
      (await readdir(join(home, ".workbuddy/skills"))).sort(),
      [...skillNames].sort(),
    );
    assert.ok(
      (await readdir(join(home, ".workbuddy/webmcp-backups"))).length > 0,
    );
    await installClient("workbuddy", {
      home,
      stateDir: ".local/custom-webmcp",
    });
    const customConfig = JSON.parse(await readFile(config, "utf8"));
    assert.match(
      customConfig.mcpServers["baseline-web"].env.WEBMCP_STATE_DIR,
      /custom-webmcp$/,
    );
    await writeFile(config, '{"mcpServers":""}');
    await assert.rejects(
      installClient("workbuddy", { home }),
      /invalid_client_config/,
    );
    assert.equal(await readFile(config, "utf8"), '{"mcpServers":""}');
    await writeFile(config, "{broken");
    await assert.rejects(
      installClient("workbuddy", { home }),
      /invalid_client_config/,
    );
    assert.equal(await readFile(config, "utf8"), "{broken");
    await assert.rejects(
      installClient("../escape", { home }),
      /unsupported_client/,
    );
  } finally {
    await rm(home, { recursive: true, force: true });
  }
});
test("Codex setup uses official merge command and keeps credentials out of argv", async () => {
  const home = await mkdtemp(join(tmpdir(), "baseline-codex-"));
  const calls = [];
  try {
    await installClient("codex", {
      home,
      execute: async (...args) => {
        calls.push(args);
      },
    });
    assert.deepEqual(calls[0][1].slice(0, 4), [
      "mcp",
      "add",
      "baseline-web",
      "--",
    ]);
    assert.equal(calls[0][2].env.CODEX_HOME, join(home, ".codex"));
    assert.ok(!JSON.stringify(calls[0][1]).includes(token));
    await installClient("codex", {
      home,
      stateDir: ".local/custom-webmcp",
      execute: async (...args) => {
        calls.push(args);
      },
    });
    assert.equal(calls[1][1][3], "--env");
    assert.match(calls[1][1][4], /^WEBMCP_STATE_DIR=.*custom-webmcp$/);

    assert.match(
      await readFile(
        join(home, ".agents/skills/baseline-web-navigate/SKILL.md"),
        "utf8",
      ),
      /baseline_page_context/,
    );
  } finally {
    await rm(home, { recursive: true, force: true });
  }
});
test("port configuration fails closed", () => {
  assert.equal(portNumber(), 18793);
  for (const value of ["foo", 80, 65536, 1234.5, ""])
    assert.throws(() => portNumber(value));
});

test("stdio adapter preserves WebMCP error envelopes without rewriting plain text", () => {
  const failure = {
    content: [{ type: "text", text: '{"code":"invalid_arguments"}' }],
    isError: true,
  };
  assert.deepEqual(
    normalizeWebMcpResult({
      content: [{ type: "text", text: JSON.stringify(failure) }],
    }),
    failure,
  );
  const text = { content: [{ type: "text", text: "ordinary text" }] };
  assert.equal(normalizeWebMcpResult(text), text);
  const plainJson = { content: [{ type: "text", text: '{"count":3}' }] };
  assert.equal(normalizeWebMcpResult(plainJson), plainJson);
});
