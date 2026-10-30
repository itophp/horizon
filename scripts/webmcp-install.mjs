import { mkdir, readFile, writeFile, copyFile, cp } from "node:fs/promises";
import { homedir } from "node:os";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { execFile } from "node:child_process";
import { promisify } from "node:util";

export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
export const skillNames = [
  "baseline-web-navigate",
  "baseline-web-accounts",
  "baseline-web-diagnose",
];
const run = promisify(execFile);
async function backup(path) {
  try {
    await copyFile(path, `${path}.backup-${Date.now()}`);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
export async function installClient(
  client,
  {
    home = homedir(),
    codex = process.env.CODEX_BIN || "codex",
    execute = run,
    stateDir = process.env.WEBMCP_STATE_DIR,
  } = {},
) {
  if (!["codex", "workbuddy"].includes(client))
    throw new Error("unsupported_client");
  const clientEnv = stateDir
    ? { WEBMCP_STATE_DIR: resolve(root, stateDir) }
    : undefined;
  const envArgs = clientEnv
    ? ["--env", `WEBMCP_STATE_DIR=${clientEnv.WEBMCP_STATE_DIR}`]
    : [];
  const command = process.execPath;
  const args = [resolve(root, "scripts/webmcp-stdio.mjs")];
  const skills = resolve(
    home,
    client === "codex" ? ".agents/skills" : ".workbuddy/skills",
  );
  // Validate existing JSON before changing anything; never replace malformed configuration.
  const configPath = resolve(
    home,
    client === "codex" ? ".codex/config.toml" : ".workbuddy/mcp.json",
  );
  let workbuddy;
  if (client === "workbuddy") {
    try {
      workbuddy = JSON.parse(await readFile(configPath, "utf8"));
    } catch (error) {
      if (error.code !== "ENOENT") throw new Error("invalid_client_config");
      workbuddy = {};
    }
    if (
      !workbuddy ||
      typeof workbuddy !== "object" ||
      Array.isArray(workbuddy) ||
      (Object.hasOwn(workbuddy, "mcpServers") &&
        (workbuddy.mcpServers === null ||
          typeof workbuddy.mcpServers !== "object" ||
          Array.isArray(workbuddy.mcpServers)))
    )
      throw new Error("invalid_client_config");
  }
  await mkdir(dirname(configPath), { recursive: true });
  await backup(configPath);
  if (client === "codex") {
    // Let the official CLI merge TOML, preserving unrelated configuration.
    await execute(
      codex,
      ["mcp", "add", "baseline-web", ...envArgs, "--", command, ...args],
      {
        timeout: 15000,
        env: { ...process.env, CODEX_HOME: resolve(home, ".codex") },
      },
    );
  } else {
    workbuddy.mcpServers = {
      ...workbuddy.mcpServers,
      "baseline-web": {
        type: "stdio",
        command,
        args,
        ...(clientEnv ? { env: clientEnv } : {}),
      },
    };
    await writeFile(configPath, JSON.stringify(workbuddy, null, 2) + "\n", {
      mode: 0o600,
    });
  }
  await mkdir(skills, { recursive: true });
  const skillBackups = resolve(
    dirname(skills),
    "webmcp-backups",
    String(Date.now()),
  );
  for (const name of skillNames) {
    const destination = resolve(skills, name);
    try {
      await mkdir(skillBackups, { recursive: true });
      await cp(destination, resolve(skillBackups, name), {
        recursive: true,
        errorOnExist: true,
        force: false,
      });
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    await cp(resolve(root, "webskills", name), destination, {
      recursive: true,
    });
  }
  return { client, installed: true, skills: skillNames.length };
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  installClient(process.argv[2])
    .then((result) => console.log(JSON.stringify(result)))
    .catch(() => {
      console.error(
        "Client setup failed; check CLI availability and configuration.",
      );
      process.exitCode = 1;
    });
}
