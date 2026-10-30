import enWebMcp from "./locales/en/webmcp.json";
import zhWebMcp from "./locales/zh-CN/webmcp.json";
import enGovernance from "./locales/en/governance.json";
import zhGovernance from "./locales/zh-CN/governance.json";
import enAdmin from "./locales/en/admin.json";
import zhAdmin from "./locales/zh-CN/admin.json";
import enCommon from "./locales/en/common.json";
import enNavigation from "./locales/en/navigation.json";
import enValidation from "./locales/en/validation.json";
import zhCommon from "./locales/zh-CN/common.json";
import zhNavigation from "./locales/zh-CN/navigation.json";
import zhValidation from "./locales/zh-CN/validation.json";
export const resources = {
  en: {
    admin: enAdmin,
    webmcp: enWebMcp,
    governance: enGovernance,
    common: enCommon,
    navigation: enNavigation,
    validation: enValidation,
  },
  "zh-CN": {
    admin: zhAdmin,
    webmcp: zhWebMcp,
    governance: zhGovernance,
    common: zhCommon,
    navigation: zhNavigation,
    validation: zhValidation,
  },
};
export const storageKey = "baseline.language";
export function preferredLanguage() {
  const saved = localStorage.getItem(storageKey);
  return saved === "zh-CN" ? saved : "en";
}
