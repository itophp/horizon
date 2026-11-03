#!/usr/bin/env node
import { execFileSync, spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const defaultRoot = fileURLToPath(new URL("../", import.meta.url));

// Explicit mapping keeps the selection auditable: a path nobody mapped is
// reported instead of silently growing into the whole browser suite.
const routes = [
  {
    prefix: "apps/web/src/features/administration/",
    specs: ["administration"],
  },
  { prefix: "apps/web/src/features/governance/", specs: ["governance"] },
  { prefix: "apps/web/src/features/capabilities/", specs: ["capabilities"] },
  { prefix: "apps/web/src/features/ai/", specs: ["ai"] },
  {
    prefix: "apps/web/src/lib/capabilities.ts",
    specs: ["capabilities", "governance", "notifications"],
  },
  { prefix: "apps/web/src/features/webmcp/", specs: ["webmcp"] },
  { prefix: "apps/web/src/features/mcp/", specs: ["mcp"] },
  { prefix: "services/design/", specs: ["foundation"] },
  { prefix: "services/runtime/server/", specs: ["foundation"] },
  { prefix: "services/runtime/config/", specs: ["foundation"] },
  { prefix: "services/runtime/app/", specs: ["foundation"] },
  { prefix: "services/system/governance/", specs: ["governance"] },
  {
    prefix: "services/system/settings/",
    specs: ["governance", "settings-schedules"],
  },
  {
    prefix: "services/system/dictionaries/",
    specs: ["governance"],
  },
  { prefix: "services/system/files/", specs: ["governance"] },
  {
    prefix: "services/system/notifications/",
    specs: ["notifications"],
  },
  { prefix: "services/integrations/mcp/", specs: ["mcp"] },
  { prefix: "services/system/orchestration/", specs: ["capabilities"] },
  { prefix: "services/applications/example/", specs: ["capabilities"] },
  { prefix: "services/kernel/workflow/", specs: ["ai"] },
  {
    prefix: "services/kernel/",
    specs: ["capabilities"],
  },
  { prefix: "services/ai/", specs: ["ai"] },
  { prefix: "services/applications/", specs: ["capabilities"] },
  { prefix: "services/db/query/capability.sql", specs: ["capabilities"] },
  { prefix: "services/db/migrations/00015_", specs: ["capabilities"] },
  {
    prefix: "services/system/identity/",
    specs: ["administration"],
  },

  { prefix: "services/runtime/transport/", specs: ["administration"] },
  { prefix: "services/db/query/mcp.sql", specs: ["mcp"] },
  { prefix: "services/db/query/governance", specs: ["governance"] },
  {
    prefix: "services/internal/repository/dbgen/mcp.sql.go",
    specs: ["mcp"],
  },
  {
    prefix: "services/internal/repository/dbgen/governance",
    specs: ["governance"],
  },
  {
    prefix: "services/internal/repository/dbgen/models.go",
    specs: ["governance"],
  },
  { prefix: "services/gen/", specs: ["foundation"] },
  { prefix: "webskills/", specs: ["mcp"] },
];

// Shared frontend code has no owning feature, so it falls back to the cheapest
// cross-layer file instead of the whole suite.
const frontendPrefix = "apps/web/src/";

// Harness, documentation and CI edits are covered by ci-fast / check-rules /
// integration tests, so they never start a browser.
const inert = [
  /AGENTS\.md$/,
  /\.md$/,
  /^\.agents\//,
  /^docs\//,
  /^\.workbuddy(\/|$)/,
  /^\.github\//,
  /^_tmp_/,
  /^\.env/,
  /^compose\.yaml$/,
  /^Makefile$/,
  /^(pnpm-lock\.yaml|pnpm-workspace\.yaml|package\.json|skills-lock\.json)$/,
  /^apps\/web\/(package\.json|tsconfig\.json|vite\.config\.ts|orval\.config\.ts|eslint)/,
  /^apps\/web\/playwright[^/]*\.config\.ts$/,
  /^apps\/web\/lint-rules\.test\.mjs$/,
  /^services\/go\.(mod|sum)$/,
  /^services\/tests\//,
];

const mcpBackend =
  /^services\/(gen\/.*mcp|internal\/repository\/dbgen\/mcp|db\/migrations\/\d+_mcp_|db\/query\/mcp)/;

function under(file, prefix) {
  return file === prefix.replace(/\/$/, "") || file.startsWith(prefix);
}

function classify(file) {
  const spec = file.match(/^apps\/web\/e2e\/([a-z-]+)\.spec\.ts$/);
  if (spec) return { specs: [spec[1]] };
  if (inert.some((pattern) => pattern.test(file))) return { specs: [] };
  if (file.startsWith("scripts/"))
    return { specs: file.includes("webmcp") ? ["webmcp"] : [] };
  if (mcpBackend.test(file)) return { specs: ["mcp"] };

  const route = routes.find((entry) => under(file, entry.prefix));
  if (route) return { specs: route.specs };
  if (file.startsWith(frontendPrefix)) return { specs: ["foundation"] };
  return { specs: [], unmapped: file };
}

export function selectSpecs(files) {
  const selected = new Set();
  const unmapped = [];
  for (const file of files) {
    const result = classify(file);
    result.specs.forEach((name) => selected.add(name));
    if (result.unmapped) unmapped.push(result.unmapped);
  }
  return { specs: [...selected].sort(), unmapped };
}

export function changedFiles(root) {
  const output = execFileSync("git", ["status", "--porcelain"], {
    cwd: root,
    encoding: "utf8",
  });
  return output
    .split("\n")
    .map((line) => line.trimEnd())
    .filter((line) => line.length > 3)
    .map((line) => {
      const path = line.slice(3).replace(/^"|"$/g, "");
      const renamed = path.split(" -> ");
      return renamed[renamed.length - 1].replace(/\/$/, "");
    });
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  let files;
  try {
    files = changedFiles(defaultRoot);
  } catch (error) {
    console.error("无法读取 git 改动：", error.message);
    process.exit(1);
  }
  const { specs, unmapped } = selectSpecs(files);
  const targets = specs.length
    ? specs.map((name) => `e2e/${name}.spec.ts`)
    : unmapped.length > 0
      ? ["--grep", "@smoke"]
      : [];

  if (targets.length === 0) {
    console.log(
      `本次改动不涉及浏览器验收，未启动 E2E（变更 ${files.length} 个文件）。`,
    );
    process.exit(0);
  }
  console.log(`按改动选中：${targets.join(" ")}`);
  if (specs.length === 0)
    console.log(`无专属映射的路径按 @smoke 快速集处理：${unmapped.join(", ")}`);
  else if (unmapped.length)
    console.log(
      `未映射、本次未选浏览器文件（请人工确认范围）：${unmapped.join(", ")}`,
    );
  console.log(
    "这是所选文件的验收，不等于全量浏览器集合；全量用 make test-e2e。",
  );
  if (process.argv.includes("--dry-run")) process.exit(0);

  const result = spawnSync("pnpm", ["test:e2e", ...targets], {
    cwd: defaultRoot,
    stdio: "inherit",
  });
  if (result.error) {
    console.error("无法启动 Playwright：", result.error.message);
    process.exit(1);
  }
  process.exit(result.status ?? 1);
}
