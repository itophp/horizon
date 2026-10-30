import { readFile, readdir, stat } from "node:fs/promises";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const defaultRoot = fileURLToPath(new URL("../", import.meta.url));
const ignored = new Set([
  ".git",
  ".tools",
  "node_modules",
  "dist",
  "playwright-report",
  "test-results",
  "coverage",
]);
function requireValue(condition, message) {
  if (!condition) throw new Error(message);
}
function localPath(root, name) {
  requireValue(
    typeof name === "string" && name.length > 0 && !isAbsolute(name),
    `Expected repository-relative path: ${name}`,
  );
  const absolute = resolve(root, name);
  const rel = relative(root, absolute);
  requireValue(
    rel !== ".." && !rel.startsWith(`..${sep}`),
    `Path escapes repository: ${name}`,
  );
  return absolute;
}
function stringList(value, label) {
  requireValue(
    Array.isArray(value) &&
      value.length > 0 &&
      value.every((item) => typeof item === "string" && item.length > 0),
    `Expected non-empty string list: ${label}`,
  );
  requireValue(
    new Set(value).size === value.length,
    `Duplicate item in ${label}`,
  );
}
async function fileText(root, name) {
  const path = localPath(root, name);
  requireValue((await stat(path)).isFile(), `Expected file: ${name}`);
  return readFile(path, "utf8");
}
export async function checkLocalLinks(root, source, content) {
  const links = content.matchAll(
    /(?<!!)\[[^\]]+\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g,
  );
  for (const match of links) {
    const link = match[1].replace(/^<|>$/g, "");
    if (/^(https?:|mailto:|#)/.test(link)) continue;
    requireValue(
      !/^[a-z]+:/i.test(link),
      `Unsupported local reference in ${source}: ${link}`,
    );
    const target = decodeURIComponent(link.split("#")[0]);
    const path = localPath(root, join(dirname(source), target));
    try {
      await stat(path);
    } catch (error) {
      throw new Error(`Broken local reference in ${source}: ${link}`, {
        cause: error,
      });
    }
  }
}
async function discover(root, subdir = "") {
  const found = { instructions: [], skills: [] };
  for (const entry of await readdir(resolve(root, subdir), {
    withFileTypes: true,
  })) {
    if (ignored.has(entry.name) || entry.isSymbolicLink()) continue;
    const name = join(subdir, entry.name);
    if (entry.isDirectory()) {
      const nested = await discover(root, name);
      found.instructions.push(...nested.instructions);
      found.skills.push(...nested.skills);
    } else if (entry.name === "AGENTS.md") found.instructions.push(name);
    else if (
      entry.name === "SKILL.md" &&
      name.startsWith(`.agents${sep}skills${sep}`)
    )
      found.skills.push(name);
  }
  return found;
}
function sameFiles(actual, expected, label) {
  const missing = expected.filter((name) => !actual.includes(name));
  const unregistered = actual.filter((name) => !expected.includes(name));
  requireValue(
    !missing.length && !unregistered.length,
    `${label} inventory mismatch; missing: ${missing.join(", ") || "none"}; unregistered: ${unregistered.join(", ") || "none"}`,
  );
}

export async function checkPolicy(root = defaultRoot, suppliedPolicy) {
  const policy =
    suppliedPolicy ??
    JSON.parse(await fileText(root, "docs/agent-policy.json"));
  requireValue(policy.schemaVersion === 1, "Unsupported agent policy schema");
  stringList(policy.instructions, "instructions");
  stringList(policy.references, "references");
  requireValue(
    policy.instructions.includes("AGENTS.md"),
    "Root AGENTS.md is required",
  );
  requireValue(
    Array.isArray(policy.skills) && policy.skills.length > 0,
    "Skills inventory is required",
  );
  const skillNames = policy.skills.map((skill) => skill.name);
  stringList(skillNames, "skill names");
  const skillFiles = policy.skills.map((skill) => skill.file);
  stringList(skillFiles, "skill files");
  for (const skill of policy.skills) {
    requireValue(
      ["project", "upstream"].includes(skill.kind),
      `Unknown skill ownership: ${skill.name}`,
    );
    const content = await fileText(root, skill.file);
    const header = content.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/)?.[1];
    requireValue(header, `Missing skill frontmatter: ${skill.file}`);
    const declaredName = header.match(
      /^name:\s*["']?([a-z0-9-]+)["']?\s*$/m,
    )?.[1];
    const description = header.match(/^description:\s*(.+)$/m)?.[1]?.trim();
    requireValue(
      declaredName === skill.name &&
        /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(declaredName) &&
        declaredName.length <= 64,
      `Invalid/mismatched skill name: ${skill.file}`,
    );
    requireValue(
      description &&
        description.length <= 1100 &&
        !description.includes("[TODO:"),
      `Missing/unfinished skill description: ${skill.file}`,
    );
    requireValue(
      content.slice(content.indexOf(header) + header.length + 4).trim().length >
        0,
      `Empty skill instructions: ${skill.file}`,
    );
    if (skill.kind === "project")
      await fileText(root, join(dirname(skill.file), "agents/openai.yaml"));
    await checkLocalLinks(root, skill.file, content);
  }
  for (const name of [...policy.instructions, ...policy.references]) {
    const content = await fileText(root, name);
    requireValue(content.trim().length > 0, `Empty rule/reference: ${name}`);
    if (policy.instructions.includes(name))
      requireValue(
        !/第一阶段|阶段一|stage[ -]one|phase[ -]?0?1/i.test(content),
        `Temporary stage restriction in persistent rules: ${name}`,
      );
    await checkLocalLinks(root, name, content);
  }
  for (const name of policy.instructions) {
    const chain = policy.instructions.filter(
      (parent) =>
        name === parent ||
        name.startsWith(
          `${dirname(parent) === "." ? "" : `${dirname(parent)}/`}`,
        ),
    );
    // Only actual ancestors, never unrelated sibling guidance.
    const ancestors = chain.filter(
      (parent) =>
        dirname(parent) === "." ||
        dirname(name) === dirname(parent) ||
        dirname(name).startsWith(`${dirname(parent)}/`),
    );
    let bytes = 0;
    for (const parent of ancestors)
      bytes += Buffer.byteLength(await fileText(root, parent));
    requireValue(
      bytes <= 32768,
      `Repository instruction chain exceeds 32 KiB: ${name}`,
    );
  }
  const makefile = await fileText(root, "Makefile");
  const targets = new Set(
    [...makefile.matchAll(/^([a-z][a-z0-9-]*):(?:\s|$)/gm)].map(
      (match) => match[1],
    ),
  );
  requireValue(
    policy.gates &&
      typeof policy.gates === "object" &&
      Object.keys(policy.gates).length > 0,
    "Gate routing is required",
  );
  for (const [gate, target] of Object.entries(policy.gates))
    requireValue(
      targets.has(target),
      `Gate ${gate} references missing make target: ${target}`,
    );
  requireValue(
    Array.isArray(policy.tasks) && policy.tasks.length > 0,
    "Task routing is required",
  );
  stringList(
    policy.tasks.map((task) => task.id),
    "task ids",
  );
  for (const task of policy.tasks) {
    for (const property of ["skills", "instructions", "tools", "gates"])
      stringList(task[property], `${task.id}.${property}`);
    for (const skill of task.skills)
      requireValue(
        skillNames.includes(skill),
        `Task ${task.id} references unknown skill: ${skill}`,
      );
    for (const instruction of task.instructions)
      requireValue(
        policy.instructions.includes(instruction),
        `Task ${task.id} references unregistered instructions: ${instruction}`,
      );
    for (const gate of task.gates)
      requireValue(
        Object.hasOwn(policy.gates, gate),
        `Task ${task.id} references unknown gate: ${gate}`,
      );
  }
  const discovered = await discover(root);
  sameFiles(discovered.instructions, policy.instructions, "AGENTS.md");
  sameFiles(discovered.skills, skillFiles, "SKILL.md");
  return {
    instructions: policy.instructions.length,
    skills: skillNames.length,
    tasks: policy.tasks.length,
    gates: Object.keys(policy.gates).length,
  };
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  try {
    console.log("Agent rules verified:", await checkPolicy());
  } catch (error) {
    console.error("Agent rule verification failed:", error.message);
    process.exitCode = 1;
  }
}
