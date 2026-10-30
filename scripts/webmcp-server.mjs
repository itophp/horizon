import express from "express";
import { useProxyHandles } from "@opentiny/agent";
import { randomBytes, timingSafeEqual } from "node:crypto";
import { mkdir, readFile, writeFile, chmod } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { root, installClient } from "./webmcp-install.mjs";

export const defaultPort = 18793;
export function portNumber(value = defaultPort) {
  const port = Number(value);
  if (!Number.isInteger(port) || port < 1024 || port > 65535)
    throw new Error("invalid_port");
  return port;
}
export async function credentials() {
  const directory = resolve(
    root,
    process.env.WEBMCP_STATE_DIR || ".local/webmcp",
  );
  await mkdir(directory, { recursive: true, mode: 0o700 });
  const path = resolve(directory, "connection.json");
  try {
    const saved = JSON.parse(await readFile(path, "utf8"));
    if (!/^[a-f0-9]{64}$/.test(saved.token))
      throw new Error("invalid_credentials");
    await chmod(path, 0o600);
    return saved;
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
    return { token: randomBytes(32).toString("hex") };
  }
}
export function createWebMcpApp({
  token,
  port = defaultPort,
  origins = ["http://127.0.0.1:5173", "http://localhost:5173"],
  install = installClient,
  bind = async () => {},
  handles = useProxyHandles(),
}) {
  const app = express();
  app.disable("x-powered-by");
  app.use((req, res, next) => {
    res.setHeader("Cache-Control", "no-store");
    const host = req.headers.host;
    if (![`127.0.0.1:${port}`, `localhost:${port}`].includes(host))
      return res.status(403).json({ code: "invalid_host" });
    const origin = req.headers.origin;
    if (origin && !origins.includes(origin))
      return res.status(403).json({ code: "invalid_origin" });
    if (origin) {
      res.setHeader("Access-Control-Allow-Origin", origin);
      res.setHeader("Access-Control-Allow-Credentials", "true");
      res.setHeader("Vary", "Origin");
      res.setHeader(
        "Access-Control-Allow-Methods",
        "GET, POST, DELETE, OPTIONS",
      );
      res.setHeader(
        "Access-Control-Allow-Headers",
        "Content-Type, Authorization, Mcp-Session-Id, Mcp-Protocol-Version, Stream-Session-Id, Last-Event-ID",
      );
      res.setHeader(
        "Access-Control-Expose-Headers",
        "Mcp-Session-Id, Stream-Session-Id",
      );
    }
    if (req.method === "OPTIONS") return res.sendStatus(204);
    // Public liveness only; returns no sessions, credentials, or environment details.
    if (req.path === "/health" && req.method === "GET")
      return res.json({ ok: true });
    const supplied = Buffer.from(req.headers.authorization || "");
    const expected = Buffer.from(`Bearer ${token}`);
    if (
      supplied.length !== expected.length ||
      !timingSafeEqual(supplied, expected)
    )
      return res.status(401).json({ code: "unauthenticated" });
    next();
  });
  // The upstream browser transport uses credentials:include. Never use or forward application cookies.
  app.use((req, _res, next) => {
    delete req.headers.cookie;
    next();
  });
  app.use(express.json({ limit: "256kb" }));
  app.all("/api/v1/webmcp/mcp", async (req, res) => {
    if (req.query.sessionId) await handles.handleStreamInspector(req, res);
    else await handles.handleStreamRequest(req, res);
  });
  app.post("/bind", async (req, res) => {
    if (!req.headers.origin || !origins.includes(req.headers.origin))
      return res.status(403).json({ code: "invalid_origin" });
    const { sessionId } = req.body ?? {};
    if (
      typeof sessionId !== "string" ||
      !Object.hasOwn(handles.clients, sessionId)
    )
      return res.status(400).json({ code: "invalid_session" });
    await bind(sessionId);
    res.json({ bound: true });
  });
  let installing = false;
  app.post("/setup", async (req, res) => {
    if (!req.headers.origin || !origins.includes(req.headers.origin))
      return res.status(403).json({ code: "invalid_origin" });
    const { client, sessionId } = req.body ?? {};
    if (
      !["codex", "workbuddy"].includes(client) ||
      typeof sessionId !== "string" ||
      !Object.hasOwn(handles.clients, sessionId)
    )
      return res.status(400).json({ code: "invalid_setup" });
    if (installing) return res.status(409).json({ code: "setup_busy" });
    installing = true;
    try {
      await bind(sessionId);
      res.json(await install(client));
    } finally {
      installing = false;
    }
  });
  app.use((_req, res) => res.status(404).json({ code: "not_found" }));
  app.use((_error, _req, res, _next) => {
    if (!res.headersSent) res.status(500).json({ code: "webmcp_failed" });
  });
  return { app, handles };
}
export async function startWebMcpServer() {
  const port = portNumber(process.env.WEBMCP_PORT);
  const saved = await credentials();
  // Never revive a page binding across a server restart.
  const connection = { port, token: saved.token };
  const path = resolve(
    root,
    process.env.WEBMCP_STATE_DIR || ".local/webmcp",
    "connection.json",
  );
  await writeFile(path, JSON.stringify(connection), { mode: 0o600 });
  const origins = (
    process.env.WEBMCP_ALLOWED_ORIGINS ||
    "http://127.0.0.1:5173,http://localhost:5173"
  ).split(",");
  const { app, handles } = createWebMcpApp({
    ...connection,
    origins,
    bind: async (sessionId) => {
      await writeFile(path, JSON.stringify({ ...connection, sessionId }), {
        mode: 0o600,
      });
    },
  });
  const server = app.listen(port, "127.0.0.1", () => {
    console.log(`WebMCP: http://127.0.0.1:${port}`);
    console.log(
      `Pairing key (enter in the WebMCP settings page): ${connection.token}`,
    );
  });
  const stop = () => {
    handles.reset();
    server.close();
    server.closeAllConnections();
  };
  process.once("SIGINT", stop);
  process.once("SIGTERM", stop);
  return server;
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  startWebMcpServer().catch(() => {
    console.error("WebMCP startup failed; check configuration.");
    process.exitCode = 1;
  });
}
