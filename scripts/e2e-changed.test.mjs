import assert from "node:assert/strict";
import { test } from "node:test";
import { selectSpecs } from "./e2e-changed.mjs";

test("selects the spec that owns the changed feature", () => {
  const { specs } = selectSpecs([
    "apps/web/src/features/administration/layout.tsx",
  ]);
  assert.deepEqual(specs, ["administration"]);
});

test("falls back to the cheap cross-layer file for shared frontend code", () => {
  const { specs } = selectSpecs(["apps/web/src/i18n/config.ts"]);
  assert.deepEqual(specs, ["foundation"]);
});

test("runs an edited spec even when nothing else changed", () => {
  const { specs } = selectSpecs(["apps/web/e2e/webmcp.spec.ts"]);
  assert.deepEqual(specs, ["webmcp"]);
});

test("keeps documentation and rule batches out of the browser", () => {
  const { specs, unmapped } = selectSpecs([
    "docs/testing.md",
    "AGENTS.md",
    "apps/web/e2e/AGENTS.md",
    ".agents/skills/baseline-quality-gates/SKILL.md",
  ]);
  assert.deepEqual(specs, []);
  assert.deepEqual(unmapped, []);
});

test("reports backend paths that have no browser route instead of guessing", () => {
  const { specs, unmapped } = selectSpecs([
    "services/api/db/migrations/0001_init.sql",
  ]);
  assert.deepEqual(specs, []);
  assert.deepEqual(unmapped, ["services/api/db/migrations/0001_init.sql"]);
});

test("routes backend modules to the feature they back", () => {
  assert.deepEqual(
    selectSpecs(["services/api/internal/notifications/service.go"]).specs,
    ["notifications"],
  );
  assert.deepEqual(
    selectSpecs(["services/api/internal/mcp/registry.go"]).specs,
    ["mcp"],
  );
  assert.deepEqual(selectSpecs(["services/api/db/query/mcp.sql"]).specs, [
    "mcp",
  ]);
  assert.deepEqual(
    selectSpecs(["services/api/internal/identity/catalog.go"]).specs,
    ["administration"],
  );
});

test("keeps harness and dependency edits out of the browser run", () => {
  const { specs, unmapped } = selectSpecs([
    "Makefile",
    "package.json",
    "apps/web/playwright.config.ts",
    "apps/web/vite.config.ts",
    "services/api/go.mod",
    "_tmp_24989_06f42f13d4a08dea5389387a78935a0e",
  ]);
  assert.deepEqual(specs, []);
  assert.deepEqual(unmapped, []);
});
