import { createInstance } from "i18next";
import { initReactI18next } from "react-i18next";
import { resources, preferredLanguage, storageKey } from "./config";
export const i18n = createInstance();
await i18n.use(initReactI18next).init({
  resources,
  lng: preferredLanguage(),
  fallbackLng: "en",
  supportedLngs: ["en", "zh-CN"],
  defaultNS: "common",
  interpolation: { escapeValue: false },
  saveMissing: import.meta.env.DEV,
  missingKeyHandler: (_languages, namespace, key) => {
    console.error(`Missing translation: ${namespace}:${key}`);
  },
});
function syncLanguage(language: string) {
  document.documentElement.lang = language;
  document.title = i18n.t("appName");
}
syncLanguage(i18n.language);
i18n.on("languageChanged", syncLanguage);
export async function changeLanguage(language: string, persist = true) {
  if (language !== "en" && language !== "zh-CN")
    throw new Error("Unsupported locale");
  await i18n.changeLanguage(language);
  if (persist) localStorage.setItem(storageKey, language);
  if (persist)
    window.dispatchEvent(
      new CustomEvent("baseline:preference", {
        detail: { key: "display.language", value: language },
      }),
    );
}
