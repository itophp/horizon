import enArticles from "./locales/en/articles.json";
import zhArticles from "./locales/zh-CN/articles.json";
import enAi from "./locales/en/ai.json";
import zhAi from "./locales/zh-CN/ai.json";
import enCapability from "./locales/en/capability.json";
import zhCapability from "./locales/zh-CN/capability.json";
import enMcp from "./locales/en/mcp.json";
import zhMcp from "./locales/zh-CN/mcp.json";
import enWebMcp from "./locales/en/webmcp.json";
import zhWebMcp from "./locales/zh-CN/webmcp.json";
import enGovernance from "./locales/en/governance.json";
import zhGovernance from "./locales/zh-CN/governance.json";
import enAssistant from "./locales/en/assistant.json";
import enKnowledge from "./locales/en/knowledge.json";
import zhAssistant from "./locales/zh-CN/assistant.json";
import zhKnowledge from "./locales/zh-CN/knowledge.json";
import enAdmin from "./locales/en/admin.json";
import zhAdmin from "./locales/zh-CN/admin.json";
import enCommon from "./locales/en/common.json";
import enEntity from "./locales/en/entity.json";
import enNavigation from "./locales/en/navigation.json";
import enValidation from "./locales/en/validation.json";
import zhCommon from "./locales/zh-CN/common.json";
import zhEntity from "./locales/zh-CN/entity.json";
import zhNavigation from "./locales/zh-CN/navigation.json";
import zhValidation from "./locales/zh-CN/validation.json";
export const resources = {
  en: {
    articles: enArticles,
    admin: enAdmin,
    ai: enAi,
    capability: enCapability,
    mcp: enMcp,
    webmcp: enWebMcp,
    governance: enGovernance,
    assistant: enAssistant,
    knowledge: enKnowledge,
    common: enCommon,
    entity: enEntity,
    navigation: enNavigation,
    validation: enValidation,
  },
  "zh-CN": {
    articles: zhArticles,
    admin: zhAdmin,
    ai: zhAi,
    capability: zhCapability,
    mcp: zhMcp,
    webmcp: zhWebMcp,
    governance: zhGovernance,
    assistant: zhAssistant,
    knowledge: zhKnowledge,
    common: zhCommon,
    entity: zhEntity,
    navigation: zhNavigation,
    validation: zhValidation,
  },
};
export const storageKey = "baseline.language";
export function preferredLanguage() {
  const saved = localStorage.getItem(storageKey);
  return saved === "zh-CN" ? saved : "en";
}
