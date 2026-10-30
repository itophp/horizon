import { readdir, readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
const roots = [
  "services/api/gen",
  "services/api/internal/repository/dbgen",
  "apps/web/src/api/generated",
];
async function snapshot() {
  const result = {};
  async function visit(dir) {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const path = `${dir}/${entry.name}`;
      if (entry.isDirectory()) await visit(path);
      else
        result[path] = createHash("sha256")
          .update(await readFile(path))
          .digest("hex");
    }
  }
  for (const root of roots) await visit(root);
  return result;
}
const before = await snapshot();
const run = spawnSync("make", ["generate"], { stdio: "inherit" });
if (run.status !== 0) process.exit(run.status ?? 1);
const after = await snapshot();
const changed = [
  ...new Set([...Object.keys(before), ...Object.keys(after)]),
].filter((path) => before[path] !== after[path]);
if (changed.length) {
  console.error("Generated output drift:\n" + changed.join("\n"));
  process.exit(1);
}
console.log("Goa, sqlc and Orval generation is reproducible.");
