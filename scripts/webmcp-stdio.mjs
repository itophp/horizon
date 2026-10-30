import { normalizeWebMcpResult } from "./webmcp-result.mjs";
// Credentials stay in the local 0600 file, never in client config, argv or URLs.
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  ListToolsRequestSchema,
  CallToolRequestSchema,
  ToolListChangedNotificationSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { root } from "./webmcp-install.mjs";

try {
  const { port, token, sessionId } = JSON.parse(
    await readFile(
      resolve(
        root,
        process.env.WEBMCP_STATE_DIR || ".local/webmcp",
        "connection.json",
      ),
      "utf8",
    ),
  );
  if (
    !Number.isInteger(port) ||
    port < 1024 ||
    port > 65535 ||
    !/^[a-f0-9]{64}$/.test(token) ||
    !sessionId
  )
    throw new Error("not_paired");
  const client = new Client({ name: "baseline-web-bridge", version: "1.0.0" });
  const url = new URL(`http://127.0.0.1:${port}/api/v1/webmcp/mcp`);
  url.searchParams.set("sessionId", sessionId);
  await client.connect(
    new StreamableHTTPClientTransport(url, {
      requestInit: { headers: { Authorization: `Bearer ${token}` } },
    }),
  );
  const server = new Server(
    { name: "baseline-web", version: "1.0.0" },
    { capabilities: { tools: { listChanged: true } } },
  );
  server.setRequestHandler(ListToolsRequestSchema, () => client.listTools());
  server.setRequestHandler(CallToolRequestSchema, async (request) =>
    normalizeWebMcpResult(await client.callTool(request.params)),
  );
  client.setNotificationHandler(ToolListChangedNotificationSchema, () =>
    server.notification({ method: "notifications/tools/list_changed" }),
  );
  await server.connect(new StdioServerTransport());
  const close = async () => {
    await client.close();
    await server.close();
  };
  process.once("SIGINT", close);
  process.once("SIGTERM", close);
  server.onclose = () => {
    void client.close();
  };
} catch {
  console.error(
    "WebMCP unavailable. Start pnpm webmcp, connect the settings page and run client setup again.",
  );
  process.exitCode = 1;
}
