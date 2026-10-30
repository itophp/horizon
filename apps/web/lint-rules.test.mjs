import { test } from "node:test";
import assert from "node:assert/strict";
import { Linter } from "eslint";
import ts from "typescript-eslint";
import { i18nText, sdkOnly } from "./eslint-rules.mjs";
const linter = new Linter();
function messages(source) {
  return linter.verify(
    source,
    [
      {
        files: ["**/*.tsx"],
        languageOptions: {
          parser: ts.parser,
          parserOptions: { ecmaFeatures: { jsx: true } },
        },
        plugins: { project: { rules: { "i18n-text": i18nText } } },
        rules: {
          "project/i18n-text": "error",
          "no-restricted-syntax": sdkOnly,
        },
      },
    ],
    { filename: "feature.tsx" },
  );
}
test("localized visible copy and technical attributes remain valid", () => {
  assert.deepEqual(
    messages(
      "export const View = ({t}) => <><button type='submit'>{t('actions.save')}</button><input id='name' placeholder={t('echo.placeholder')}/></>;",
    ),
    [],
  );
});
test("rejects literal visible copy and accessibility/placeholder copy", () => {
  for (const source of [
    "export const View = () => <button>Save</button>;",
    "export const View = () => <input placeholder='Name'/>;",
    "export const View = () => <button aria-label={'Save'}/>;",
    "export const View = () => <p>{'Welcome'}</p>;",
  ])
    assert.ok(
      messages(source).some(
        (message) => message.ruleId === "project/i18n-text",
      ),
      source,
    );
});
test("rejects requests that bypass the SDK boundary", () => {
  for (const call of [
    "fetch('/api/records')",
    "window.fetch('/api/records')",
  ]) {
    assert.ok(
      messages(`export const View = () => { ${call}; return <div/>; };`).some(
        (message) => message.ruleId === "no-restricted-syntax",
      ),
    );
  }
});
