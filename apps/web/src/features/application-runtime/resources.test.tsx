import { expect, it, vi } from "vitest";
import {
  ContributionResources,
  localeContribution,
  themeContribution,
} from "./contributions";
import { formSchema } from "./form-schema";
import {
  runtimeFieldV2Schema,
  type RuntimeApplicationV2,
} from "@/api/generated/runtime";
import { readFileSync } from "node:fs";
const applications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as RuntimeApplicationV2[];
it("leases trusted resources once and releases only after the last page closes", () => {
  const resources = new ContributionResources();
  const dispose = vi.fn();
  const activate = vi.fn(() => dispose);
  const app = structuredClone(applications[0]);
  app.extensions = [
    {
      id: "test",
      source: app.source,
      entry: "test",
      version: "1",
      type: "shell",
      runtimeCompatibility: "2.1",
      permissions: [],
      dependencies: [],
    },
  ];
  const unregister = resources.register({
    source: app.source,
    entry: "test",
    version: "1",
    type: "shell",
    activate,
  });
  const closeA = resources.acquire(app),
    closeB = resources.acquire(app);
  expect(activate).toHaveBeenCalledTimes(1);
  expect(() => unregister()).toThrow();
  closeA();
  expect(dispose).not.toHaveBeenCalled();
  closeB();
  expect(dispose).toHaveBeenCalledTimes(1);
  unregister();
  expect(() => resources.acquire(app)).toThrow("component_not_registered");
});
it("rolls back resources when a later activation fails", () => {
  const resources = new ContributionResources();
  const dispose = vi.fn();
  const app = structuredClone(applications[0]);
  app.extensions = ["first", "second"].map((entry) => ({
    id: entry,
    source: app.source,
    entry,
    version: "1",
    type: "shell",
    runtimeCompatibility: "2.1",
    permissions: [],
    dependencies: [],
  }));
  resources.register({
    source: app.source,
    entry: "first",
    version: "1",
    type: "shell",
    activate: () => dispose,
  });
  resources.register({
    source: app.source,
    entry: "second",
    version: "1",
    type: "shell",
    activate: () => {
      throw new Error("unavailable");
    },
  });
  expect(() => resources.acquire(app)).toThrow();
  expect(dispose).toHaveBeenCalledTimes(1);
});
it("rejects translation drift and executable theme values", () => {
  const declaration = { source: "test", entry: "locale", version: "1" };
  expect(() =>
    localeContribution(declaration, "test", {
      en: { one: "One" },
      "zh-CN": { two: "二" },
    }),
  ).toThrow();
  expect(() =>
    themeContribution(declaration, {
      "--plugin-image": "url(https://untrusted.invalid)",
    }),
  ).toThrow();
});
it("keeps money exact and rejects malformed typed subtable values", () => {
  const base = structuredClone(
    applications[0].pages
      .flatMap((page) => page.forms)
      .find((form) => form.create.fields.length)!.create.fields[0],
  );
  const money = runtimeFieldV2Schema.parse({
    ...base,
    name: "amount",
    type: "money",
    component: "money",
    enum: [],
    minLength: 0,
    maxLength: 0,
  });
  expect(
    formSchema([money], {}).parse({ amount: "9007199254740993.01" }).amount,
  ).toBe("9007199254740993.01");
  expect(
    formSchema([money], {}).safeParse({ amount: Number("9007199254740993.01") })
      .success,
  ).toBe(false);
  const table = runtimeFieldV2Schema.parse({
    ...base,
    name: "rows",
    type: "array",
    component: "subtable",
    maxItems: 2,
    children: [{ ...money, name: "amount" }],
  });
  expect(
    formSchema([table], {}).safeParse({ rows: [{ amount: false }] }).success,
  ).toBe(false);
  expect(
    formSchema([table], {}).safeParse({
      rows: [{ amount: "1" }, { amount: "2" }, { amount: "3" }],
    }).success,
  ).toBe(false);
});
