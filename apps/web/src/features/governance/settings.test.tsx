import { expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { SettingsPage } from "./settings";
const mocks = vi.hoisted(() => ({
  draft: vi.fn(async () => ({ status: 201, data: { id: 1 } })),
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "t",
    activeDepartmentId: 1,
    account: { id: 1 },
    permissions: ["settings:read", "settings:update"],
  }),
}));
vi.mock("@/features/administration/shared", async (original) => {
  const actual =
    await original<typeof import("@/features/administration/shared")>();
  return {
    ...actual,
    useAdminData: () => ({ data: { organizations: [], departments: [] } }),
  };
});
vi.mock("@/api/generated/baseline", async (original) => {
  const actual = await original<typeof import("@/api/generated/baseline")>();
  return {
    ...actual,
    settingsDraft: mocks.draft,
    settingsSchedules: vi.fn(async () => ({ status: 200, data: [] })),
    settingsWorkspace: vi.fn(async () => ({
      status: 200,
      data: {
        target: { kind: "workspace", id: 0 },
        revision: 0,
        definitions: [
          {
            key: "branding.name",
            group: "branding",
            kind: "string",
            choices: [],
            defaultValue: '"Horizon"',
            secret: false,
            sensitive: false,
            scopes: ["workspace"],
          },
          {
            key: "display.pageSize",
            group: "display",
            kind: "integer",
            choices: [],
            defaultValue: "8",
            secret: false,
            sensitive: false,
            scopes: ["workspace"],
          },
        ],
        entries: [
          {
            key: "branding.name",
            value: '"Horizon"',
            source: "default",
            overridden: false,
            configured: true,
          },
          {
            key: "display.pageSize",
            value: "8",
            source: "default",
            overridden: false,
            configured: true,
          },
        ],
        drafts: [],
        history: [],
      },
    })),
  };
});
it("submits changed settings through the generated SDK, including a native submit button", async () => {
  await changeLanguage("en");
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <SettingsPage />
      </QueryClientProvider>
    </I18nextProvider>,
  );
  expect(
    screen.queryByRole("combobox", { name: "Configuration scope" }),
  ).not.toBeInTheDocument();
  const user = userEvent.setup();
  await user.selectOptions(
    await screen.findByRole("combobox", { name: "Product name · Operation" }),
    "set",
  );
  await user.clear(screen.getByRole("textbox", { name: "Product name" }));
  await user.type(
    screen.getByRole("textbox", { name: "Product name" }),
    "Updated brand",
  );
  await user.type(
    screen.getByRole("textbox", { name: "Change reason" }),
    "Update organization branding",
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() =>
    expect(mocks.draft).toHaveBeenCalledWith({
      target: { kind: "workspace", id: 0 },
      baseRevision: 0,
      reason: "Update organization branding",
      changes: [
        { key: "branding.name", operation: "set", value: '"Updated brand"' },
      ],
    }),
  );
  expect(screen.getByRole("status")).toHaveTextContent("Changes saved");
});
