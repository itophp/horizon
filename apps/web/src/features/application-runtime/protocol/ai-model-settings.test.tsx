import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { expect, it, vi } from "vitest";
import { changeLanguage, i18n } from "@/i18n";
import { AIModelSettings } from "./ai-model-settings";

it("shows the active default and excludes embedding models from assignment", async () => {
  await changeLanguage("en", false);
  const user = userEvent.setup();
  const emit = vi.fn().mockResolvedValue(true);
  const entries = [
    ["aiModels.default", '"deepseek"'],
    ["aiModels.deepseek.displayName", '"DeepSeek-V4.1-Flash"'],
    ["aiModels.glm.displayName", '"GLM"'],
    ["aiModels.glm.type", '"text"'],
    ["aiModels.custom.displayName", '"Vector model"'],
    ["aiModels.custom.type", '"embedding"'],
  ].map(([key, value]) => ({ key, value, configured: true }));
  render(
    <MemoryRouter>
      <I18nextProvider i18n={i18n}>
        <AIModelSettings
          emit={emit}
          data={{ entries }}
          props={{ view: "default" }}
          readOnly={false}
        />
      </I18nextProvider>
    </MemoryRouter>,
  );
  expect(screen.getByText(/Active model.*DeepSeek/)).toBeVisible();
  screen.getByRole("combobox", { name: "Default model" }).focus();
  await user.keyboard("{ArrowDown}");
  const glm = await screen.findByRole("option", { name: "GLM" });
  expect(screen.queryByRole("option", { name: "Vector model" })).toBeNull();
  await user.click(glm);
  await user.type(
    screen.getByRole("textbox", { name: "Change reason" }),
    "Switch the default",
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(emit).toHaveBeenCalledWith("submit", {
    changes: [{ key: "aiModels.default", operation: "set", value: '"glm"' }],
    reason: "Switch the default",
  });
});
