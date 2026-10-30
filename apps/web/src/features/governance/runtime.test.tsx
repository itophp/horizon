import { afterEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { ThemeProvider, useTheme } from "@/app/providers/theme";
import { RuntimeProvider } from "./runtime";
const fixture = vi.hoisted(() => ({
  personal: false,
  preferences: vi.fn(async () => ({ status: 200 })),
}));
vi.mock("@/features/administration/auth", () => ({
  useAuth: () => ({
    profile: { tenantId: "runtime", account: { id: 1 }, activeDepartmentId: 2 },
  }),
}));
vi.mock("@/api/generated/baseline", async (original) => {
  const actual = await original<typeof import("@/api/generated/baseline")>();
  return {
    ...actual,
    settingsPreferences: fixture.preferences,
    settingsEffective: vi.fn(async () => ({
      status: 200,
      data: {
        revision: fixture.personal ? "personal" : "workspace",
        entries: [
          {
            key: "display.theme",
            value: fixture.personal ? '"dark"' : '"light"',
            source: fixture.personal ? "account:1" : "workspace:0",
            overridden: fixture.personal,
            configured: true,
          },
          {
            key: "display.language",
            value: '"en"',
            source: fixture.personal ? "account:1" : "workspace:0",
            overridden: fixture.personal,
            configured: true,
          },
        ],
      },
    })),
  };
});
function Indicator() {
  const { theme } = useTheme();
  return <output>{theme}</output>;
}
async function show() {
  await changeLanguage("en", false);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <QueryClientProvider client={client}>
          <RuntimeProvider>
            <Indicator />
          </RuntimeProvider>
        </QueryClientProvider>
      </ThemeProvider>
    </I18nextProvider>,
  );
}
afterEach(() => {
  fixture.preferences.mockClear();
  fixture.personal = false;
  localStorage.clear();
});
it("applies workspace defaults without manufacturing a personal preference", async () => {
  localStorage.clear();
  await show();
  await waitFor(() =>
    expect(screen.getByRole("status")).toHaveTextContent("light"),
  );
  expect(localStorage.getItem("baseline.theme")).toBeNull();
  expect(fixture.preferences).not.toHaveBeenCalled();
});
it("uses saved server preferences ahead of legacy browser values", async () => {
  localStorage.clear();
  localStorage.setItem("baseline.theme", "light");
  localStorage.setItem("baseline.language", "en");
  fixture.personal = true;
  await show();
  await waitFor(() =>
    expect(screen.getByRole("status")).toHaveTextContent("dark"),
  );
  expect(fixture.preferences).not.toHaveBeenCalled();
});
