import { readdir, readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
const base = fileURLToPath(
  new URL("../apps/web/src/i18n/locales/", import.meta.url),
);
function leaves(value, prefix = "") {
  return Object.entries(value)
    .flatMap(([key, item]) =>
      typeof item === "object"
        ? leaves(item, `${prefix}${key}.`)
        : [`${prefix}${key}`],
    )
    .sort();
}
const expected = (await readdir(`${base}en`)).sort();
for (const language of await readdir(base)) {
  const names = (await readdir(base + language)).sort();
  if (JSON.stringify(names) !== JSON.stringify(expected))
    throw new Error(`Namespace mismatch: ${language}`);
  for (const namespace of names) {
    const en = JSON.parse(await readFile(`${base}en/${namespace}`, "utf8"));
    const actual = JSON.parse(
      await readFile(`${base}${language}/${namespace}`, "utf8"),
    );
    if (JSON.stringify(leaves(en)) !== JSON.stringify(leaves(actual)))
      throw new Error(`Translation key mismatch: ${language}/${namespace}`);
  }
}
console.log("All translation namespaces and key sets match.");
