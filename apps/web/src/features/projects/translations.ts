import en from "@/i18n/locales/en/projects.json";
import zhCN from "@/i18n/locales/zh-CN/projects.json";

// Kept separate from page imports so i18n initialization has no React cycle.
export const projectTranslations = {
  en: { projects: en },
  "zh-CN": { projects: zhCN },
};
