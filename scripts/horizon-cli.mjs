import { spawnSync } from "node:child_process";

export function runHorizon(root, args, stdio) {
  const executable = process.env.HORIZON_CLI_EXECUTABLE;
  const [command, ...prefix] = executable
    ? [executable]
    : process.platform === "win32"
      ? ["go", "run", "./cmd/horizon-cli"]
      : ["./scripts/go.sh", "run", "../cmd/horizon-cli"];
  const result = spawnSync(command, [...prefix, ...args], {
    cwd: root,
    encoding: "utf8",
    stdio,
  });
  if (result.error) throw result.error;
  return result;
}
