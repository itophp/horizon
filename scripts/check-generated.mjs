import { readdir, readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { runHorizon } from "./horizon-cli.mjs";

const root = fileURLToPath(new URL("../", import.meta.url));
const roots = [
  "docs/generated/workflows",
  "docs/generated/runtime",
  "foundation/gen",
  "internal/repository/dbgen",
  "apps/web/src/api/generated",
];
async function snapshot() {
  const result = {};
  async function visit(dir) {
    for (const entry of await readdir(resolve(root, dir), {
      withFileTypes: true,
    })) {
      const path = `${dir}/${entry.name}`;
      if (entry.isDirectory()) await visit(path);
      else
        result[path] = createHash("sha256")
          .update(await readFile(resolve(root, path)))
          .digest("hex");
    }
  }
  for (const root of roots) await visit(root);
  return result;
}
const before = await snapshot();
const run = runHorizon(root, ["generate"], "inherit");
if (run.status !== 0) process.exit(run.status ?? 1);
const after = await snapshot();
const changed = [
  ...new Set([...Object.keys(before), ...Object.keys(after)]),
].filter((path) => before[path] !== after[path]);
if (changed.length) {
  console.error("Generated output drift:\n" + changed.join("\n"));
  process.exit(1);
}
console.log(
  "Workflow schemas/examples, Goa, sqlc and Orval generation is reproducible.",
);
