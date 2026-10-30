import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, it, expect } from "vitest";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { Shell } from "./shell";
describe("application shell", () => {
  it("switches language without navigating or reloading", async () => {
    await changeLanguage("en");
    render(
      <I18nextProvider i18n={i18n}>
        <MemoryRouter>
          <Shell />
        </MemoryRouter>
      </I18nextProvider>,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Language" }),
      "zh-CN",
    );
    expect(screen.getByRole("navigation")).toHaveAccessibleName("主导航");
    expect(screen.getByRole("link", { name: "架构说明" })).toBeVisible();
    await changeLanguage("en");
  });
});
