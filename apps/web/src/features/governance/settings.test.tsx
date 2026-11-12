import { readFileSync } from "node:fs";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { I18nextProvider } from "react-i18next";
import { ThemeProvider, useTheme } from "@/app/providers/theme";
import { RuntimeProvider } from "./runtime";
import { i18n, changeLanguage } from "@/i18n";
import { DynamicApplicationRoute } from "@/features/application-runtime/route";
import { ProtocolPage } from "@/features/application-runtime/protocol";
import type { RuntimeApplicationV2 } from "@/api/generated/runtime";

const applications = (
  JSON.parse(
    readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
  ) as RuntimeApplicationV2[]
).filter(
  (app) => app.id === "system-settings" || app.id === "personal-preferences",
);

const priority = {
  id: 41,
  code: "priority",
  nameEn: "Priority",
  nameZh: "优先级",
  category: "general",
  kind: "flat",
  status: "active",
  revision: 1,
  target: { kind: "workspace", id: 0 },
};
const mocks = vi.hoisted(() => ({
  permissions: ["settings:read", "settings:update"],
  read: vi.fn(),
  savedTheme: "light",
  execute: vi.fn(),
  importPreview: vi.fn(),
  importDraft: vi.fn(async () => ({ status: 200 })),
  draft: vi.fn(async () => ({ status: 200, data: { id: 1 } })),
}));
const effective = () => ({
  revision: mocks.savedTheme,
  entries: [
    {
      key: "display.theme",
      value: JSON.stringify(mocks.savedTheme),
      source: mocks.savedTheme === "dark" ? "account:1" : "workspace:0",
      overridden: mocks.savedTheme === "dark",
      configured: true,
    },
  ],
});
vi.mock("@/features/administration/auth", () => {
  const useSession = () => ({
    tenantId: "t",
    activeDepartmentId: 1,
    account: { id: 1 },
    permissions: mocks.permissions,
  });
  return { useSession, useAuth: () => ({ profile: useSession() }) };
});
vi.mock("@/features/administration/shared", async (original) => {
  const actual =
    await original<typeof import("@/features/administration/shared")>();
  return {
    ...actual,
    useAdminData: () => ({
      data: { organizations: [], departments: [], catalog: [] },
    }),
  };
});
vi.mock("@/lib/capabilities", async (original) => {
  const actual = await original<typeof import("@/lib/capabilities")>();
  return {
    ...actual,
    runtimeApplication: async (id: string) =>
      applications.find((app) => app.id === id),
    runtimeRead: mocks.read,
    dictionariesGet: async () => ({
      status: 200,
      data: {
        document: { definition: priority, items: [] },
        drafts: [],
        history: [],
        references: 0,
        overrideRevision: 0,
      },
    }),
    dictionariesOptions: async () => ({ status: 200, data: [] }),
    dictionariesImport: mocks.importPreview,
    dictionariesImportDraft: mocks.importDraft,
    settingsDraft: mocks.draft,
    runtimeExecute: mocks.execute,
    settingsEffective: async () => ({ status: 200, data: effective() }),
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
beforeEach(async () => {
  await changeLanguage("en", false);
  vi.clearAllMocks();
  mocks.permissions = ["settings:read", "settings:update"];
  mocks.savedTheme = "light";
  mocks.execute.mockImplementation(async (_name, _version, input) => {
    mocks.savedTheme = input.changes[0]?.operation === "set" ? "dark" : "light";
    return { status: "succeeded" };
  });
  localStorage.clear();
  mocks.read.mockImplementation(async (name: string) => {
    if (name === "application.runtime.list") return { items: applications };
    if (name === "settings.effective.read") return effective();
    if (name === "dictionary.list.read")
      return {
        target: { kind: "workspace", id: 0 },
        definitions: [priority],
      };
    throw new Error("Unexpected read: " + name);
  });
});

function ThemeIndicator() {
  const { theme } = useTheme();
  return <output aria-label="Applied theme">{theme}</output>;
}

function show(path = "/settings", preview = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const settings = applications.find(
    (app) => app.id === "system-settings",
  )!.protocol!;
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[path]}>
        <QueryClientProvider client={client}>
          {preview ? (
            <ProtocolPage
              application={settings}
              page={settings.pages[0]}
              readOnly
            />
          ) : path === "/preferences" ? (
            <ThemeProvider>
              <RuntimeProvider>
                <DynamicApplicationRoute />
                <ThemeIndicator />
              </RuntimeProvider>
            </ThemeProvider>
          ) : (
            <DynamicApplicationRoute />
          )}
        </QueryClientProvider>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

it("submits a settings draft from the Runtime route", async () => {
  show();
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

it("opens the only authorized tab and binds the dictionary list through Runtime", async () => {
  mocks.permissions = ["dictionaries:read"];
  show("/settings?tab=config");
  expect(await screen.findByRole("button", { name: /Priority/ })).toBeVisible();
  expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  expect(mocks.read).toHaveBeenCalledWith(
    "dictionary.list.read",
    { targetKind: "workspace", targetId: 0 },
    expect.anything(),
    "1.0.0",
  );
});

it("shows an access denial when neither settings tab is authorized", async () => {
  mocks.permissions = [];
  show("/settings?tab=fields");
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "You do not have access",
  );
  expect(mocks.read).not.toHaveBeenCalledWith(
    "dictionary.list.read",
    expect.anything(),
    expect.anything(),
    expect.anything(),
  );
});

it("renders workspace settings in preview with write controls disabled", async () => {
  mocks.permissions.push("settings:publish", "settings:export");
  show("/settings", true);
  expect(
    await screen.findByRole("combobox", { name: "Product name · Operation" }),
  ).toBeDisabled();
  expect(
    screen.queryByRole("button", { name: "Save draft" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Export" }),
  ).not.toBeInTheDocument();
  expect(mocks.draft).not.toHaveBeenCalled();
});

it("saves personal preferences and reapplies the theme without management permissions", async () => {
  mocks.permissions = [];
  show("/preferences");
  expect(
    await screen.findByRole("heading", { name: "Personal preferences" }),
  ).toBeVisible();
  await waitFor(() =>
    expect(screen.getByLabelText("Applied theme")).toHaveTextContent("light"),
  );
  const user = userEvent.setup();
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Theme · Operation" }),
    "set",
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Theme" }),
    '"dark"',
  );
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(screen.getByLabelText("Applied theme")).toHaveTextContent("dark"),
  );
  expect(mocks.execute.mock.calls[0].slice(0, 3)).toEqual([
    "settings.preferences.update",
    "1.0.0",
    { changes: [{ key: "display.theme", operation: "set", value: '"dark"' }] },
  ]);
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Theme · Operation" }),
    "reset",
  );
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(screen.getByLabelText("Applied theme")).toHaveTextContent("light"),
  );
  expect(mocks.execute.mock.calls[1].slice(0, 3)).toEqual([
    "settings.preferences.update",
    "1.0.0",
    { changes: [{ key: "display.theme", operation: "reset", value: "" }] },
  ]);
});

it("previews a dictionary import and submits a draft from the Runtime table", async () => {
  mocks.permissions = ["dictionaries:read", "dictionaries:import"];
  mocks.importPreview.mockResolvedValue({
    status: 200,
    data: {
      document: {
        definition: priority,
        items: [{ code: "urgent", labelEn: "Urgent" }],
      },
      errors: [],
      added: 1,
      updated: 0,
    },
  });
  show("/settings?tab=fields");
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: /Priority/ }));
  await user.click(await screen.findByRole("button", { name: "Import" }));
  const dialog = within(await screen.findByRole("dialog", { name: "Import" }));
  const csv = "code,labelEn\nurgent,Urgent";
  const file = new File([csv], "priority.csv", { type: "text/csv" });
  Object.defineProperty(file, "text", { value: async () => csv });
  await user.upload(dialog.getByLabelText("File name"), file);
  expect(
    dialog.getByRole("button", { name: "Save import draft" }),
  ).toBeDisabled();
  await user.click(
    dialog.getByRole("button", { name: "Validate and preview" }),
  );
  expect(await dialog.findByText("Urgent")).toBeVisible();
  expect(mocks.importPreview).toHaveBeenCalledWith(41, {
    format: "csv",
    content: csv,
  });
  expect(mocks.importDraft).not.toHaveBeenCalled();
  await user.type(
    dialog.getByLabelText("Change reason"),
    "Add urgent priority",
  );
  await user.click(dialog.getByRole("button", { name: "Save import draft" }));
  await waitFor(() =>
    expect(mocks.importDraft).toHaveBeenCalledWith(41, {
      format: "csv",
      content: csv,
      reason: "Add urgent priority",
      baseRevision: 1,
    }),
  );
});
