import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { checkLocalLinks, checkPolicy } from "./check-agent-rules.mjs";
const root = fileURLToPath(new URL("../", import.meta.url));
const policy = JSON.parse(
  await readFile(new URL("../docs/agent-policy.json", import.meta.url), "utf8"),
);

test("repository routes resolve to real skills, scoped rules and make targets", async () => {
  const result = await checkPolicy(root);
  assert.ok(result.skills > 0 && result.instructions > 0 && result.gates > 0);
});
test("rejects a task that points to a missing skill", async () => {
  const invalid = structuredClone(policy);
  invalid.tasks[0].skills = ["missing-skill"];
  await assert.rejects(checkPolicy(root, invalid), /unknown skill/);
});
test("rejects a named gate whose executable target does not exist", async () => {
  const invalid = structuredClone(policy);
  invalid.gates.fast = "missing-target";
  await assert.rejects(checkPolicy(root, invalid), /missing make target/);
});
test("rejects broken local references and paths outside the repository", async () => {
  await assert.rejects(
    checkLocalLinks(root, "AGENTS.md", "[rule](docs/not-a-rule.md)"),
    /Broken local reference/,
  );
  await assert.rejects(
    checkLocalLinks(root, "AGENTS.md", "[rule](../outside-project.md)"),
    /escapes repository/,
  );
});
test("rejects inventory entries that would escape the project", async () => {
  const invalid = structuredClone(policy);
  invalid.skills[0].file = "../outside-project.md";
  await assert.rejects(checkPolicy(root, invalid), /escapes repository/);
});
