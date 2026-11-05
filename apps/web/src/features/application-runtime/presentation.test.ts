import { readFileSync } from "node:fs";
import { expect, it } from "vitest";
import type { RuntimeApplicationV2 } from "@/api/generated/runtime";
import { i18n } from "@/i18n";
import { ContributionResources } from "./contributions";
import { preparePresentation } from "./presentation";

it("isolates local messages by immutable version and releases the last lease", () => {
  const source = (
    JSON.parse(
      readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
    ) as RuntimeApplicationV2[]
  ).find((app) => app.id === "projects")!;
  const application = (version: string, label: string) =>
    preparePresentation({
      ...source,
      extensions: [],
      applicationVersion: version,
      labelKey: "projects_local:preview.title",
      messages: [
        { key: "projects_local:preview.title", en: label, zhCN: label },
      ],
    });
  const old = application("1.0.0.1", "First"),
    next = application("1.0.0.2", "Second");
  const resources = new ContributionResources();
  const releases = [
    resources.acquire(old),
    resources.acquire(old),
    resources.acquire(next),
  ];
  try {
    expect(i18n.t(old.labelKey, { lng: "en" })).toBe("First");
    expect(i18n.t(next.labelKey, { lng: "en" })).toBe("Second");
    releases.shift()!();
    expect(i18n.hasResourceBundle("en", old.labelKey.split(":")[0])).toBe(true);
    releases.shift()!();
    expect(i18n.hasResourceBundle("en", old.labelKey.split(":")[0])).toBe(
      false,
    );
    expect(i18n.t(next.labelKey, { lng: "en" })).toBe("Second");
  } finally {
    for (const release of releases) release();
  }
  expect(i18n.hasResourceBundle("en", next.labelKey.split(":")[0])).toBe(false);
});
