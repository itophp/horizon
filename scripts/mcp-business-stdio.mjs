import { readFile, stat } from "node:fs/promises";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  ListToolsRequestSchema,
  CallToolRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";

try {
  const path = process.argv[2];
  const info = await stat(path);
  if (process.platform !== "win32" && info.mode & 0o077)
    throw new Error("insecure_credentials");
  const { url, token } = JSON.parse(await readFile(path, "utf8"));
  if (!/^bmcp_[a-f0-9]{64}$/.test(token))
    throw new Error("invalid_credentials");
  const client = new Client({
    name: "baseline-business-bridge",
    version: "1.0.0",
  });
  await client.connect(
    new StreamableHTTPClientTransport(new URL(url), {
      requestInit: {
        headers: { Authorization: `Bearer ${token}` },
        redirect: "error",
      },
    }),
  );
  const server = new Server(
    { name: "baseline-business", version: "1.0.0" },
    { capabilities: { tools: {} } },
  );
  server.setRequestHandler(ListToolsRequestSchema, () => client.listTools());
  server.setRequestHandler(CallToolRequestSchema, (request) =>
    client.callTool(request.params),
  );
  server.onclose = () => {
    void client.close();
  };
  await server.connect(new StdioServerTransport());
  const stop = async () => {
    await server.close();
    await client.close();
  };
  process.once("SIGINT", stop);
  process.once("SIGTERM", stop);
} catch {
  console.error(
    "Business MCP unavailable. Check the installed credential, expiry and remote service.",
  );
  process.exitCode = 1;
}
