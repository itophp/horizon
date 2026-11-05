import enRuntime from "./locales/en/runtime.json";
import zhRuntime from "./locales/zh-CN/runtime.json";
import enArticles from "./locales/en/articles.json";
import enDrive from "./locales/en/drive.json";
import zhDrive from "./locales/zh-CN/drive.json";
import { frontendExtensions } from "@/app/plugins/builtins";
import { assertTranslationNamespaces } from "@/app/plugins/registry";
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
import enOrchestration from "./locales/en/orchestration.json";
import zhAssistant from "./locales/zh-CN/assistant.json";
import zhKnowledge from "./locales/zh-CN/knowledge.json";
import zhOrchestration from "./locales/zh-CN/orchestration.json";
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
const pluginResources = {
  en: Object.assign(
    {},
    ...frontendExtensions.translations.map((item) => item.en),
  ),
  "zh-CN": Object.assign(
    {},
    ...frontendExtensions.translations.map((item) => item["zh-CN"]),
  ),
};
const coreResources = {
  en: {
    runtime: enRuntime,
    articles: enArticles,
    drive: enDrive,
    admin: enAdmin,
    ai: enAi,
    capability: enCapability,
    mcp: enMcp,
    webmcp: enWebMcp,
    governance: enGovernance,
    assistant: enAssistant,
    knowledge: enKnowledge,
    orchestration: enOrchestration,
    common: enCommon,
    entity: enEntity,
    navigation: enNavigation,
    validation: enValidation,
  },
  "zh-CN": {
    runtime: zhRuntime,
    articles: zhArticles,
    drive: zhDrive,
    admin: zhAdmin,
    ai: zhAi,
    capability: zhCapability,
    mcp: zhMcp,
    webmcp: zhWebMcp,
    governance: zhGovernance,
    assistant: zhAssistant,
    knowledge: zhKnowledge,
    orchestration: zhOrchestration,
    common: zhCommon,
    entity: zhEntity,
    navigation: zhNavigation,
    validation: zhValidation,
  },
};
assertTranslationNamespaces(coreResources, frontendExtensions.translations);
export const resources = {
  en: { ...coreResources.en, ...pluginResources.en },
  "zh-CN": { ...coreResources["zh-CN"], ...pluginResources["zh-CN"] },
};
export const storageKey = "baseline.language";
export function preferredLanguage() {
  const saved = localStorage.getItem(storageKey);
  return saved === "zh-CN" ? saved : "en";
}
