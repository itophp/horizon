export const defaultWebMcpPort = 18793;
export type AgentClient = "codex" | "workbuddy";
export type LocalConnection = { port: number; token: string };
function isLocalConnection(value: unknown): value is LocalConnection {
  if (typeof value !== "object" || value === null) return false;
  const saved = value as Record<string, unknown>;
  return (
    typeof saved.port === "number" &&
    Number.isInteger(saved.port) &&
    saved.port >= 1024 &&
    saved.port <= 65535 &&
    typeof saved.token === "string" &&
    /^[a-f0-9]{64}$/.test(saved.token)
  );
}
// Development-only helper served by the Vite dev server; it returns nothing in
// builds, so callers fall back to manual entry.
export async function fetchLocalConnection(): Promise<LocalConnection | null> {
  try {
    const response = await fetch("/__webmcp/connection", {
      headers: { Accept: "application/json" },
      credentials: "omit",
    });
    if (!response.ok) return null;
    const body: unknown = await response.json();
    return isLocalConnection(body) ? body : null;
  } catch {
    return null;
  }
}
export function endpoint(port: number) {
  if (!Number.isInteger(port) || port < 1024 || port > 65535)
    throw new Error("invalid_port");
  return `http://127.0.0.1:${port}`;
}
// This is local developer infrastructure, not an application API or DTO.
export async function setupClient(
  port: number,
  token: string,
  sessionId: string,
  client: AgentClient,
) {
  const response = await fetch(`${endpoint(port)}/setup`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ client, sessionId }),
    signal: AbortSignal.timeout(30000),
    credentials: "omit",
  });
  if (!response.ok) throw new Error("setup_failed");
}

export async function bindPage(port: number, token: string, sessionId: string) {
  const response = await fetch(`${endpoint(port)}/bind`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ sessionId }),
    signal: AbortSignal.timeout(10000),
    credentials: "omit",
  });
  if (!response.ok) throw new Error("bind_failed");
}
