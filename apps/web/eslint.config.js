import { i18nText, sdkOnly } from "./eslint-rules.mjs";
import js from "@eslint/js";
import ts from "typescript-eslint";
import hooks from "eslint-plugin-react-hooks";
import globals from "globals";
export default ts.config(
  {
    ignores: [
      "dist",
      "src/api/generated",
      "playwright-report",
      "test-results",
      ".local/**",
    ],
  },
  js.configs.recommended,
  ...ts.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    plugins: { "react-hooks": hooks },
    rules: {
      ...hooks.configs.recommended.rules,
      "@typescript-eslint/no-explicit-any": "error",
    },
  },
  {
    files: ["src/**/*.tsx"],
    ignores: ["src/**/*.test.tsx"],
    plugins: { project: { rules: { "i18n-text": i18nText } } },
    rules: { "project/i18n-text": "error" },
  },
  {
    files: [
      "src/app/**/*.{ts,tsx}",
      "src/features/**/*.{ts,tsx}",
      "src/components/**/*.{ts,tsx}",
    ],
    ignores: ["src/**/*.test.{ts,tsx}"],
    rules: { "no-restricted-syntax": sdkOnly },
  },
);
