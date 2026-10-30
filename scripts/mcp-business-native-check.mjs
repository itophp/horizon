// Native-client acceptance: isolated configuration, no model requests or user profile changes.
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { createInterface } from "node:readline";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { installBusiness } from "./mcp-business-helper.mjs";
import { nativeCodexConnection } from "./mcp-business-codex-check.mjs";
const lines = createInterface({ input: process.stdin });
const iterator = lines[Symbol.asyncIterator]();
const home = await mkdtemp(resolve(tmpdir(), "baseline-native-mcp-"));
let codex;
try {
  const { url, token } = JSON.parse((await iterator.next()).value);
  if (!process.env.WORKBUDDY_CLI) throw new Error("WORKBUDDY_CLI is required");
  await installBusiness("codex", url, token, { home });
  await installBusiness("workbuddy", url, token, { home });
  codex = await nativeCodexConnection(home);
  const result = await codex.call();
  if (result.isError || !JSON.stringify(result.content).includes("accountId"))
    throw new Error("native_codex_tool_failed");
  const workbuddy = await promisify(execFile)(
    process.execPath,
    [process.env.WORKBUDDY_CLI, "mcp", "list"],
    {
      timeout: 30000,
      cwd: home,
      env: {
        ...process.env,
        WORKBUDDY_CONFIG_DIR: resolve(home, ".workbuddy"),
        CODEBUDDY_CONFIG_DIR: resolve(home, ".workbuddy"),
        DISABLE_GALILEO: "1",
      },
    },
  );
  if (
    !workbuddy.stdout.includes("baseline-business") ||
    !workbuddy.stdout.includes("Connected")
  )
    throw new Error("native_workbuddy_connection_failed");
  console.log(
    JSON.stringify({
      phase: "connected",
      codexToolCall: true,
      workbuddyConnected: true,
    }),
  );
  await iterator.next();
  const denied = await codex.call().then(
    (value) => value.isError === true,
    () => true,
  );
  if (!denied) throw new Error("native_revocation_failed");
  const after = await promisify(execFile)(
    process.execPath,
    [process.env.WORKBUDDY_CLI, "mcp", "list"],
    {
      timeout: 30000,
      cwd: home,
      env: {
        ...process.env,
        WORKBUDDY_CONFIG_DIR: resolve(home, ".workbuddy"),
        CODEBUDDY_CONFIG_DIR: resolve(home, ".workbuddy"),
        DISABLE_GALILEO: "1",
      },
    },
  );
  if (
    !after.stdout.includes("baseline-business") ||
    after.stdout.includes("Connected")
  )
    throw new Error("native_workbuddy_revocation_failed");
  console.log(
    JSON.stringify({
      phase: "revoked",
      codexRejected: true,
      workbuddyRejected: true,
    }),
  );
} catch (error) {
  console.error(
    error instanceof Error ? error.message : "native_acceptance_failed",
  );
  process.exitCode = 1;
} finally {
  codex?.close();
  lines.close();
  await rm(home, { recursive: true, force: true });
}
