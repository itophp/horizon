import test from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  readFile,
  writeFile,
  mkdir,
  stat,
  readdir,
  rm,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import {
  createBusinessHelper,
  installBusiness,
  serviceURL,
} from "./mcp-business-helper.mjs";
const token = "bmcp_" + "a".repeat(64);
test("business installer keeps secrets out of configuration, preserves other servers and backs up credentials", async () => {
  const home = await mkdtemp(resolve(tmpdir(), "mcp-business-"));
  try {
    await mkdir(resolve(home, ".workbuddy"));
    await writeFile(
      resolve(home, ".workbuddy/mcp.json"),
      JSON.stringify({ mcpServers: { existing: { command: "keep" } } }),
    );
    await installBusiness("workbuddy", "https://service.test/mcp", token, {
      home,
    });
    let config = JSON.parse(
      await readFile(resolve(home, ".workbuddy/mcp.json"), "utf8"),
    );
    assert.equal(config.mcpServers.existing.command, "keep");
    assert.equal(config.mcpServers["baseline-business"].type, "stdio");
    assert.ok(!JSON.stringify(config).includes(token));
    const secretFile = resolve(
      home,
      ".local/share/baseline-mcp/workbuddy.json",
    );
    assert.equal((await stat(secretFile)).mode & 0o777, 0o600);
    assert.equal(JSON.parse(await readFile(secretFile, "utf8")).token, token);
    await installBusiness("workbuddy", "https://service.test/mcp", token, {
      home,
    });
    assert.ok(
      (await readdir(resolve(home, ".workbuddy"))).some((name) =>
        name.startsWith("mcp.json.backup-"),
      ),
    );
    let args;
    await installBusiness("codex", "https://service.test/mcp", token, {
      home,
      execute: async (_command, values) => {
        args = values;
      },
    });
    assert.deepEqual(args.slice(0, 4), [
      "mcp",
      "add",
      "baseline-business",
      "--",
    ]);
    assert.ok(!JSON.stringify(args).includes(token));
    assert.match(
      await readFile(
        resolve(home, ".agents/skills/baseline-business/SKILL.md"),
        "utf8",
      ),
      /capabilities_list/,
    );
    for (const invalid of [null, 0, "bad", []]) {
      await writeFile(
        resolve(home, ".workbuddy/mcp.json"),
        JSON.stringify({ mcpServers: invalid }),
      );
      await assert.rejects(
        () =>
          installBusiness("workbuddy", "https://service.test/mcp", token, {
            home,
          }),
        /invalid_client_config/,
      );
    }
    await writeFile(resolve(home, ".workbuddy/mcp.json"), "invalid JSON");
    await assert.rejects(
      () =>
        installBusiness("workbuddy", "https://service.test/mcp", token, {
          home,
        }),
      /invalid_client_config/,
    );
  } finally {
    await rm(home, { recursive: true, force: true });
  }
});
test("helper pins origin and service, verifies credentials before installing, consumes pairing once", async () => {
  let installs = 0;
  const app = createBusinessHelper({
    url: "https://service.test/mcp",
    key: "b".repeat(64),
    port: 18797,
    check: async (_url, secret, expected) => {
      assert.equal(secret, token);
      assert.equal(expected.accountId, 1);
      return { connected: true, toolCount: 2 };
    },
    install: async () => {
      installs++;
      return { installed: true, client: "codex" };
    },
  });
  const server = app.listen(18797, "127.0.0.1");
  await new Promise((resolve, reject) => {
    server.once("listening", resolve);
    server.once("error", reject);
  });
  try {
    const request = (
      path,
      body,
      origin = "https://service.test",
      key = "b".repeat(64),
    ) =>
      fetch("http://127.0.0.1:18797" + path, {
        method: "POST",
        headers: {
          Origin: origin,
          Authorization: `Bearer ${key}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      });
    assert.equal(
      (await request("/prepare", {}, "https://hostile.test")).status,
      403,
    );
    assert.equal((await request("/prepare", {}, undefined, "bad")).status, 401);
    assert.equal(
      (
        await request("/prepare", {
          client: "shell",
          accountId: 1,
          departmentId: 2,
        })
      ).status,
      400,
    );
    const response = await request("/prepare", {
      client: "codex",
      accountId: 1,
      departmentId: 2,
    });
    assert.equal(response.status, 200);
    const prepared = await response.json();
    assert.equal(prepared.url, "https://service.test/mcp");
    assert.equal(
      (await request("/install", { proof: prepared.proof, token })).status,
      200,
    );
    assert.equal(
      (await request("/install", { proof: prepared.proof, token })).status,
      403,
    );
    assert.equal(installs, 1);
  } finally {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  }
});
test("service URL rejects remote HTTP and credential-bearing URLs", () => {
  for (const url of [
    "http://remote.test/mcp",
    "https://user:pass@remote.test/mcp",
    "https://remote.test/mcp?token=secret",
    "file:///mcp",
  ])
    assert.throws(() => serviceURL(url));
  assert.equal(
    serviceURL("http://127.0.0.1:5173/mcp"),
    "http://127.0.0.1:5173/mcp",
  );
});
