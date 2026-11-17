import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { changeLanguage, i18n } from "@/i18n";
import { InstallationsPage } from "./installations";

const api = vi.hoisted(() => ({ read: vi.fn(), execute: vi.fn() }));
vi.mock("@/lib/capabilities", () => ({
  runtimeRead: api.read,
  runtimeExecute: api.execute,
}));
const auth = vi.hoisted(() => ({ permissions: [] as string[] }));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant",
    account: { id: 1 },
    permissions: auth.permissions,
  }),
}));
vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));

const pluginId = "com.horizon.projects";
const installation = {
  pluginId,
  version: "2.0.1",
  status: "enabled",
  revision: 3,
  digest: "current",
  previousVersion: "2.0.0",
};
const candidates = ["2.0.0", "2.0.1", "2.0.2"].map((version) => ({
  pluginId,
  version,
  digest: version,
  applications: ["projects"],
  dependencies: [],
  dependencyVersions: [],
  base: false,
  labelKeys: { projects: "projects:heading" },
}));

beforeEach(async () => {
  vi.resetAllMocks();
  auth.permissions = [
    "applications:read",
    "applications:manage",
    "applications:design",
  ];
  await changeLanguage("en", false);
});

it("hides platform core applications from application management", async () => {
  api.read.mockResolvedValue({
    candidates: [
      {
        pluginId: "builtin:system-settings",
        version: "1.0.3",
        digest: "settings",
        applications: ["system-settings"],
        dependencies: [],
        dependencyVersions: [],
        base: true,
        labelKeys: { "system-settings": "governance:settings" },
      },
    ],
    items: [
      {
        pluginId: "builtin:system-settings",
        version: "1.0.3",
        status: "enabled",
        revision: 2,
        digest: "settings",
        previousVersion: "",
      },
    ],
    applications: [
      {
        applicationId: "system-settings",
        pluginId: "builtin:system-settings",
        enabled: true,
        revision: 2,
      },
    ],
  });
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <InstallationsPage />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
  expect(
    await screen.findByRole("heading", { name: "Business applications" }),
  ).toBeVisible();
  expect(
    screen.queryByRole("heading", { name: "Core applications" }),
  ).toBeNull();
  expect(screen.queryByRole("cell", { name: "System settings" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Actions" })).toBeNull();
  expect(screen.getByRole("button", { name: "New application" })).toBeVisible();
});

it("shows the design action to accounts without lifecycle management", async () => {
  auth.permissions = ["applications:read", "applications:design"];
  api.read.mockResolvedValue({
    candidates,
    items: [],
    applications: [],
  });
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <InstallationsPage />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
  expect(await screen.findByRole("link", { name: "Design" })).toBeVisible();
  expect(screen.queryByRole("button", { name: "Actions" })).toBeNull();
});

it.each([
  ["Install (2.0.2)", "install", "2.0.2"],
  ["Upgrade (2.0.2)", "upgrade", "2.0.2"],
  ["Roll back (2.0.0)", "rollback", "2.0.0"],
  ["Check compatibility (2.0.0)", "check", "2.0.0"],
  ["Install (2.0.0)", "install", "2.0.0", "uninstalled"],
])(
  "shows one plugin row and submits %s for the selected version",
  async (label, action, version, status?: string) => {
    const installed = action !== "install";
    api.read.mockImplementation(async (name: string) =>
      name === "application.runtime.installation.list"
        ? {
            candidates,
            items: status
              ? [{ ...installation, status }]
              : installed
                ? [installation]
                : [],
            applications: installed
              ? [
                  {
                    applicationId: "projects",
                    pluginId,
                    enabled: true,
                    revision: 1,
                  },
                ]
              : [],
          }
        : { compatible: true, issues: [], candidate: candidates[0] },
    );
    api.execute.mockResolvedValue({ status: "succeeded" });
    const user = userEvent.setup();
    render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={new QueryClient()}>
          <MemoryRouter>
            <InstallationsPage />
          </MemoryRouter>
        </QueryClientProvider>
      </I18nextProvider>,
    );

    const cells = await screen.findAllByRole("cell", { name: "Projects" });
    expect(cells).toHaveLength(1);
    expect(screen.getByText("Business applications")).toBeVisible();
    expect(
      screen.getByText(
        `Installed: ${installed ? "2.0.1" : "—"} · Available: 2.0.0, 2.0.1, 2.0.2`,
      ),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Actions" }));
    expect(screen.getByRole("link", { name: "Design" })).toBeVisible();
    const menu = await screen.findByRole("menu");
    if (installed) {
      expect(
        within(menu).getAllByRole("menuitem", { name: "Disable" }),
      ).toHaveLength(1);
      expect(
        within(menu).getAllByRole("menuitem", { name: "Projects: Disable" }),
      ).toHaveLength(1);
      expect(
        within(menu).queryByRole("menuitem", { name: "Upgrade (2.0.1)" }),
      ).toBeNull();
    }
    await user.click(within(menu).getByRole("menuitem", { name: label }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(`Target version: ${version}`),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Confirm" }));

    if (action === "check") {
      expect(api.read).toHaveBeenCalledWith(
        "application.runtime.installation.check",
        { pluginId, version },
      );
      expect(await within(dialog).findByText("Compatible")).toBeVisible();
    } else {
      expect(api.execute).toHaveBeenCalledWith(
        `application.runtime.installation.${action}`,
        "1.0.0",
        { pluginId, version, ...(installed || status ? { revision: 3 } : {}) },
        expect.any(String),
        expect.any(String),
      );
    }
  },
);
