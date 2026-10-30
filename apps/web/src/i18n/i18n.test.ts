import { describe, it, expect, afterEach } from "vitest";
import { createInstance } from "i18next";
import { resources, preferredLanguage, storageKey } from "./config";
import { i18n, changeLanguage } from "./index";
import {
  formatDate,
  formatNumber,
  formatCurrency,
  exampleInstant,
} from "@/lib/format";
afterEach(async () => {
  await changeLanguage("en");
  localStorage.clear();
});
describe("internationalization", () => {
  it("defaults to English, persists an explicit choice and updates the document", async () => {
    localStorage.clear();
    expect(preferredLanguage()).toBe("en");
    await changeLanguage("zh-CN");
    expect(i18n.t("actions.save")).toBe("保存");
    expect(localStorage.getItem(storageKey)).toBe("zh-CN");
    expect(preferredLanguage()).toBe("zh-CN");
    expect(document.documentElement.lang).toBe("zh-CN");
  });
  it("falls back for a missing Chinese translation", async () => {
    const instance = createInstance();
    await instance.init({
      lng: "zh-CN",
      fallbackLng: "en",
      resources: {
        en: { translation: { onlyEnglish: "Fallback" } },
        "zh-CN": { translation: {} },
      },
    });
    expect(instance.t("onlyEnglish")).toBe("Fallback");
  });
  it("interpolates and applies plural rules", async () => {
    expect(i18n.t("echo.result", { name: "Ada" })).toBe("Hello, Ada.");
    expect(i18n.t("formats.count", { count: 1 })).toBe("1 example");
    expect(i18n.t("formats.count", { count: 2 })).toBe("2 examples");
    await changeLanguage("zh-CN");
    expect(i18n.t("formats.count", { count: 2 })).toBe("2 个示例");
    expect(Object.keys(resources)).toEqual(["en", "zh-CN"]);
  });
  it("formats values with explicit locale and timezone", () => {
    expect(formatDate(exampleInstant, "en")).toBe(
      "January 15, 2026 at 12:30 PM",
    );
    expect(formatDate(exampleInstant, "zh-CN")).toBe("2026年1月15日 12:30");
    expect(formatNumber(1234567.89, "en")).toBe("1,234,567.89");
    expect(formatCurrency(1234.5, "en")).toBe("$1,234.50");
    expect(formatCurrency(1234.5, "zh-CN")).toBe("US$1,234.50");
  });
});
