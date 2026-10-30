import { z } from "zod";
const helper = "http://127.0.0.1:18796";
const prepared = z.object({
  proof: z.string().regex(/^[a-f0-9]{64}$/),
  url: z.string().url(),
  client: z.enum(["codex", "workbuddy"]),
});
export type BusinessClient = "codex" | "workbuddy";
async function local(
  path: string,
  key: string,
  body: unknown,
): Promise<unknown> {
  const response = await fetch(`${helper}${path}`, {
    method: "POST",
    credentials: "omit",
    headers: {
      Authorization: `Bearer ${key}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(30000),
  });
  if (!response.ok) throw new Error("helper_failed");
  return response.json();
}
export async function prepareInstall(
  key: string,
  client: BusinessClient,
  accountId: number,
  departmentId: number,
) {
  const result = prepared.parse(
    await local("/prepare", key, { client, accountId, departmentId }),
  );
  if (result.url !== `${window.location.origin}/mcp`)
    throw new Error("helper_service_mismatch");
  return result;
}
export async function installPrepared(
  key: string,
  proof: string,
  token: string,
) {
  const result = z
    .object({
      installed: z.literal(true),
      client: z.enum(["codex", "workbuddy"]),
    })
    .parse(await local("/install", key, { proof, token }));
  return result;
}
export async function diagnoseRemote(token: string) {
  const [{ Client }, { StreamableHTTPClientTransport }] = await Promise.all([
    import("@modelcontextprotocol/sdk/client/index.js"),
    import("@modelcontextprotocol/sdk/client/streamableHttp.js"),
  ]);
  const client = new Client({
    name: "baseline-browser-diagnostic",
    version: "1.0.0",
  });
  try {
    await client.connect(
      new StreamableHTTPClientTransport(
        new URL("/mcp", window.location.origin),
        {
          requestInit: {
            headers: { Authorization: `Bearer ${token}` },
            credentials: "omit",
            redirect: "error",
          },
        },
      ),
      { timeout: 10000 },
    );
    const tools = await client.listTools({}, { timeout: 10000 });
    const result = await client.callTool(
      { name: "context_get", arguments: {} },
      undefined,
      { timeout: 10000 },
    );
    if (result.isError) throw new Error("context_failed");
    return { toolCount: tools.tools.length };
  } finally {
    await client.close();
  }
}
