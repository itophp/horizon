import { describe, expect, it } from "vitest";
import {
  assertTranslationNamespaces,
  registerFrontendPlugins,
  type FrontendPlugin,
} from "./registry";

const plugin: FrontendPlugin = {
  id: "com.horizon.test",
  routes: [{ path: "/test" }],
  menus: [],
  translations: {
    en: { test: { title: "Test" } },
    "zh-CN": { test: { title: "测试" } },
  },
};

describe("frontend plugin registry", () => {
  it("collects plugin routes and localized resources", () => {
    const registry = registerFrontendPlugins([plugin]);
    expect(registry.routes).toEqual(plugin.routes);
    expect(registry.translations[0]).toEqual(plugin.translations);
  });
  it("rejects duplicate plugin ids and route paths", () => {
    expect(() => registerFrontendPlugins([plugin, plugin])).toThrow(
      /Duplicate frontend plugin/,
    );
    expect(() =>
      registerFrontendPlugins([
        plugin,
        {
          ...plugin,
          id: "com.horizon.other",
          translations: { en: {}, "zh-CN": {} },
        },
      ]),
    ).toThrow(/Duplicate plugin route/);
  });
});

it("rejects plugin translation namespaces owned by the platform", () => {
  const core = { en: { common: {} }, "zh-CN": { common: {} } };
  expect(() =>
    assertTranslationNamespaces(core, [
      { en: { common: {} }, "zh-CN": { common: {} } },
    ]),
  ).toThrow(/conflicts with platform/);
});

it("rejects missing translation keys in a plugin locale", () => {
  expect(() =>
    registerFrontendPlugins([
      {
        ...plugin,
        translations: {
          en: { test: { title: "Test" } },
          "zh-CN": { test: { other: "测试" } },
        },
      },
    ]),
  ).toThrow(/locale key mismatch/);
});

it("requires plugin permission labels in its own translations", () => {
  expect(() =>
    registerFrontendPlugins([
      { ...plugin, permissionActions: { "test:write": "test:missing" } },
    ]),
  ).toThrow(/permission label missing translation/);
});
