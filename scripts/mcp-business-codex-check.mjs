// Acceptance helper for the installed Codex app-server. It never starts a model turn.
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
export async function nativeCodexConnection(
  home,
  { binary = process.env.CODEX_BIN || "codex" } = {},
) {
  const child = spawn(binary, ["app-server", "--stdio"], {
    cwd: home,
    env: { ...process.env, CODEX_HOME: home + "/.codex" },
    stdio: ["pipe", "pipe", "ignore"],
  });
  let id = 0;
  const pending = new Map();
  const lines = createInterface({ input: child.stdout });
  lines.on("line", (line) => {
    let message;
    try {
      message = JSON.parse(line);
    } catch {
      return;
    }
    if (message.id === undefined) return;
    const slot = pending.get(message.id);
    if (!slot) return;
    pending.delete(message.id);
    clearTimeout(slot.timer);
    if (message.error)
      slot.reject(new Error(`native_codex_${slot.method}_failed`));
    else slot.resolve(message.result);
  });
  const close = () => {
    for (const slot of pending.values()) {
      clearTimeout(slot.timer);
      slot.reject(new Error("native_codex_closed"));
    }
    pending.clear();
    lines.close();
    child.kill("SIGTERM");
  };
  child.once("error", close);
  child.once("exit", close);
  const request = (method, params) =>
    new Promise((resolve, reject) => {
      const current = ++id;
      const timer = setTimeout(() => {
        pending.delete(current);
        reject(new Error(`native_codex_${method}_timeout`));
      }, 20000);
      pending.set(current, { resolve, reject, timer, method });
      child.stdin.write(JSON.stringify({ id: current, method, params }) + "\n");
    });
  try {
    await request("initialize", {
      clientInfo: { name: "baseline_mcp_acceptance", version: "1.0.0" },
      capabilities: { experimentalApi: true },
    });
    child.stdin.write(
      JSON.stringify({ method: "initialized", params: {} }) + "\n",
    );
    const started = await request("thread/start", {
      cwd: home,
      ephemeral: true,
      approvalPolicy: "never",
    });
    const threadId = started.thread.id;
    const inventory = await request("mcpServerStatus/list", {
      threadId,
      serverName: "baseline-business",
    });
    return {
      inventory,
      call: () =>
        request("mcpServer/tool/call", {
          threadId,
          server: "baseline-business",
          tool: "context_get",
          arguments: {},
        }),
      close,
    };
  } catch (error) {
    close();
    throw error;
  }
}
