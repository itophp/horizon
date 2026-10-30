import enMcp from "./locales/en/mcp.json";
import zhMcp from "./locales/zh-CN/mcp.json";
import enWebMcp from "./locales/en/webmcp.json";
import zhWebMcp from "./locales/zh-CN/webmcp.json";
import enGovernance from "./locales/en/governance.json";
import zhGovernance from "./locales/zh-CN/governance.json";
import enAssistant from "./locales/en/assistant.json";
import zhAssistant from "./locales/zh-CN/assistant.json";
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
    mcp: enMcp,
    webmcp: enWebMcp,
    governance: enGovernance,
    assistant: enAssistant,
    common: enCommon,
    navigation: enNavigation,
    validation: enValidation,
  },
  "zh-CN": {
    admin: zhAdmin,
    mcp: zhMcp,
    webmcp: zhWebMcp,
    governance: zhGovernance,
    assistant: zhAssistant,
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
