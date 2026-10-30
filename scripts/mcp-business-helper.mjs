import express from "express";
import { randomBytes, timingSafeEqual } from "node:crypto";
import {
  mkdir,
  readFile,
  writeFile,
  copyFile,
  cp,
  chmod,
} from "node:fs/promises";
import { homedir } from "node:os";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const run = promisify(execFile);
export function serviceURL(raw) {
  const url = new URL(raw);
  if (
    url.username ||
    url.password ||
    url.search ||
    url.hash ||
    url.pathname !== "/mcp" ||
    !(
      url.protocol === "https:" ||
      (url.protocol === "http:" &&
        ["127.0.0.1", "localhost", "[::1]"].includes(url.hostname))
    )
  )
    throw new Error("invalid_service_url");
  return url.href;
}
export async function diagnose(url, token, expected) {
  if (!/^bmcp_[a-f0-9]{64}$/.test(token)) throw new Error("invalid_token");
  const client = new Client({
    name: "baseline-business-diagnostic",
    version: "1.0.0",
  });
  try {
    await client.connect(
      new StreamableHTTPClientTransport(new URL(serviceURL(url)), {
        requestInit: {
          headers: { Authorization: `Bearer ${token}` },
          redirect: "error",
        },
      }),
      { timeout: 10000 },
    );
    const list = await client.listTools({}, { timeout: 10000 });
    const result = await client.callTool(
      { name: "context_get", arguments: {} },
      undefined,
      { timeout: 10000 },
    );
    if (result.isError) throw new Error("context_failed");
    const item = result.content.find((value) => value.type === "text");
    const context = JSON.parse(item?.text || "null");
    if (
      !context ||
      context.accountId !== expected.accountId ||
      context.departmentId !== expected.departmentId
    )
      throw new Error("identity_mismatch");
    return { connected: true, toolCount: list.tools.length };
  } finally {
    await client.close();
  }
}
async function backup(path) {
  try {
    await copyFile(path, `${path}.backup-${Date.now()}`);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
export async function installBusiness(
  client,
  url,
  token,
  {
    home = process.env.MCP_CLIENT_HOME || homedir(),
    execute = run,
    codex = process.env.CODEX_BIN || "codex",
  } = {},
) {
  if (!["codex", "workbuddy"].includes(client))
    throw new Error("unsupported_client");
  serviceURL(url);
  if (!/^bmcp_[a-f0-9]{64}$/.test(token)) throw new Error("invalid_token");
  const config = resolve(
    home,
    client === "codex" ? ".codex/config.toml" : ".workbuddy/mcp.json",
  );
  let existing = {};
  if (client === "workbuddy") {
    try {
      existing = JSON.parse(await readFile(config, "utf8"));
    } catch (error) {
      if (error.code !== "ENOENT") throw new Error("invalid_client_config");
    }
    if (
      !existing ||
      typeof existing !== "object" ||
      Array.isArray(existing) ||
      (Object.hasOwn(existing, "mcpServers") &&
        (existing.mcpServers === null ||
          typeof existing.mcpServers !== "object" ||
          Array.isArray(existing.mcpServers)))
    )
      throw new Error("invalid_client_config");
  }
  const directory = resolve(home, ".local/share/baseline-mcp");
  await mkdir(directory, { recursive: true, mode: 0o700 });
  await chmod(directory, 0o700);
  const credential = resolve(directory, `${client}.json`);
  await backup(credential);
  await writeFile(credential, JSON.stringify({ url, token }), { mode: 0o600 });
  await chmod(credential, 0o600);
  await mkdir(dirname(config), { recursive: true });
  await backup(config);
  const args = [resolve(root, "scripts/mcp-business-stdio.mjs"), credential];
  if (client === "codex")
    await execute(
      codex,
      ["mcp", "add", "baseline-business", "--", process.execPath, ...args],
      {
        timeout: 15000,
        env: { ...process.env, CODEX_HOME: resolve(home, ".codex") },
      },
    );
  else {
    existing.mcpServers = {
      ...existing.mcpServers,
      "baseline-business": { type: "stdio", command: process.execPath, args },
    };
    await writeFile(config, JSON.stringify(existing, null, 2) + "\n", {
      mode: 0o600,
    });
  }
  const skill = resolve(
    home,
    client === "codex"
      ? ".agents/skills/baseline-business"
      : ".workbuddy/skills/baseline-business",
  );
  try {
    await cp(skill, `${skill}.backup-${Date.now()}`, {
      recursive: true,
      errorOnExist: true,
      force: false,
    });
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  await mkdir(dirname(skill), { recursive: true });
  await cp(resolve(root, "webskills/baseline-business"), skill, {
    recursive: true,
  });
  return { installed: true, client };
}
export function createBusinessHelper({
  url,
  key,
  port = 18796,
  origin = new URL(url).origin,
  install = installBusiness,
  check = diagnose,
}) {
  url = serviceURL(url);
  const app = express();
  app.disable("x-powered-by");
  let pending = null;
  let busy = false;
  app.use((req, res, next) => {
    res.setHeader("Cache-Control", "no-store");
    if (
      ![`127.0.0.1:${port}`, `localhost:${port}`].includes(req.headers.host) ||
      req.headers.origin !== origin
    )
      return res.status(403).json({ code: "invalid_origin" });
    res.setHeader("Access-Control-Allow-Origin", origin);
    if (req.headers["access-control-request-private-network"] === "true")
      res.setHeader("Access-Control-Allow-Private-Network", "true");
    res.setHeader("Vary", "Origin");
    res.setHeader("Access-Control-Allow-Headers", "Content-Type,Authorization");
    res.setHeader("Access-Control-Allow-Methods", "POST,OPTIONS");
    if (req.method === "OPTIONS") return res.sendStatus(204);
    const got = Buffer.from(req.headers.authorization || "");
    const want = Buffer.from(`Bearer ${key}`);
    if (got.length !== want.length || !timingSafeEqual(got, want))
      return res.status(401).json({ code: "unauthenticated" });
    next();
  });
  app.use(express.json({ limit: "16kb" }));
  app.post("/prepare", (req, res) => {
    const { client, accountId, departmentId } = req.body ?? {};
    if (
      !["codex", "workbuddy"].includes(client) ||
      !Number.isSafeInteger(accountId) ||
      accountId < 1 ||
      !Number.isSafeInteger(departmentId) ||
      departmentId < 1
    )
      return res.status(400).json({ code: "invalid_setup" });
    const proof = randomBytes(32).toString("hex");
    pending = {
      proof,
      client,
      accountId,
      departmentId,
      expires: Date.now() + 60000,
    };
    res.json({ proof, url, client });
  });
  app.post("/diagnose", async (req, res) => {
    if (busy) return res.status(409).json({ code: "busy" });
    busy = true;
    try {
      res.json(await check(url, req.body?.token, req.body ?? {}));
    } finally {
      busy = false;
    }
  });
  app.post("/install", async (req, res) => {
    if (busy) return res.status(409).json({ code: "busy" });
    const request = pending;
    pending = null;
    if (
      !request ||
      request.expires < Date.now() ||
      request.proof !== req.body?.proof
    )
      return res.status(403).json({ code: "invalid_pairing" });
    busy = true;
    try {
      await check(url, req.body?.token, request);
      res.json(await install(request.client, url, req.body.token));
    } finally {
      busy = false;
    }
  });
  app.use((_req, res) => res.status(404).json({ code: "not_found" }));
  app.use((_error, _req, res, _next) => {
    if (!res.headersSent) res.status(400).json({ code: "setup_failed" });
  });
  return app;
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const url = serviceURL(
      process.env.MCP_BUSINESS_URL || "http://127.0.0.1:5173/mcp",
    );
    const key = randomBytes(32).toString("hex");
    const app = createBusinessHelper({ url, key });
    const server = app.listen(18796, "127.0.0.1", () => {
      console.log(`Business MCP helper: ${url}`);
      console.log(`Pairing key: ${key}`);
    });
    server.on("error", () => {
      console.error("Business MCP helper could not start.");
      process.exitCode = 1;
    });
    const stop = () => {
      server.close();
      server.closeAllConnections();
    };
    process.once("SIGINT", stop);
    process.once("SIGTERM", stop);
  } catch {
    console.error(
      "Invalid MCP_BUSINESS_URL. Use HTTPS /mcp, or loopback HTTP for development.",
    );
    process.exitCode = 1;
  }
}
