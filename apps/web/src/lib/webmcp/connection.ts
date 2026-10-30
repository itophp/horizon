export const defaultWebMcpPort = 18793;
export type AgentClient = "codex" | "workbuddy";
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
