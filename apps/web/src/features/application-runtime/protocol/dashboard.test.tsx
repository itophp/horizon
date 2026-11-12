import { readFileSync } from "node:fs";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, useLocation } from "react-router";
import { beforeEach, expect, it, vi } from "vitest";
import { i18n, changeLanguage } from "@/i18n";
import { DynamicApplicationRoute } from "../route";
import type { ApplicationV3 } from "./types";

const api = vi.hoisted(() => ({
  application: vi.fn(),
  read: vi.fn(),
  permissions: ["accounts:read"],
}));

vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeApplication: api.application,
  runtimeRead: api.read,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant",
    account: { id: 1, fullName: "Alice Chen" },
    activeDepartmentId: 2,
    permissions: api.permissions,
  }),
}));

vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (key: string, fallback: unknown) =>
      key === "display.timezone" ? "America/New_York" : fallback,
  }),
}));

const applications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as { id: string; protocol: ApplicationV3 }[];
const dashboard = applications.find(({ id }) => id === "analytics");

const data = {
  accounts: { active: 1, total: 1 },
  trend: [{ date: "2026-11-10", value: 1 }],
  recentAccounts: [
    {
      id: 1,
      fullName: "Alice Chen",
      username: "alice+qa",
      status: "active",
      createdAt: "2026-11-10T00:00:00Z",
      departments: ["Platform"],
    },
  ],
  activity: [
    {
      id: 1,
      key: "account-1",
      kind: "account",
      name: "Alice Chen",
      code: "alice+qa",
      createdAt: "2026-11-10T00:00:00Z",
    },
  ],
};

function Location() {
  return <output aria-label="URL">{useLocation().search}</output>;
}

function mount(path: string) {
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={[path]}>
          <DynamicApplicationRoute />
          <Location />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  vi.resetAllMocks();
  api.permissions = ["accounts:read"];
  await changeLanguage("en", false);
  if (!dashboard) throw new Error("Missing analytics deployment definition");
  api.application.mockResolvedValue(dashboard);
  api.read.mockImplementation(async (name: string) =>
    name === "application.runtime.list" ? { items: [dashboard] } : data,
  );
});

it("renders the declared analytics route and keeps authorized indicators across tabs", async () => {
  mount("/analytics?tab=invalid&keep=yes");
  await screen.findByRole("heading", { name: "Data statistics" });
  expect(screen.getByRole("tab", { name: "Overview" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(
    screen.getByRole("heading", { name: "Active accounts" }),
  ).toBeVisible();
  expect(
    screen.queryByRole("heading", { name: "Departments" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("table")).toHaveTextContent("Alice Chen");
  expect(api.read).toHaveBeenCalledWith(
    "identity.analytics.read",
    { timeZone: "America/New_York" },
    expect.any(AbortSignal),
    "1.0.0",
  );

  await userEvent.click(screen.getByRole("tab", { name: "Recently created" }));
  expect(screen.getByLabelText("URL")).toHaveTextContent(
    "?tab=recent&keep=yes",
  );
  expect(
    screen.getByRole("heading", { name: "Active accounts" }),
  ).toBeVisible();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: /Alice Chen/ })).toHaveAttribute(
    "href",
    "/accounts?q=alice%2Bqa",
  );
});

it("lets a signed-in user retry the provider and see an empty activity view", async () => {
  api.permissions = [];
  let fails = true;
  api.read.mockImplementation(async (name: string) => {
    if (name === "application.runtime.list") return { items: [dashboard] };
    if (fails) {
      fails = false;
      throw new Error("Provider unavailable");
    }
    return { activity: [] };
  });
  mount("/analytics?tab=recent");
  await userEvent.click(
    await screen.findByRole("button", { name: "Try again" }),
  );
  expect(await screen.findByText("No recent activity.")).toBeVisible();
  expect(
    screen.queryByRole("heading", { name: "Active accounts" }),
  ).not.toBeInTheDocument();
});

it("shows the department count, footer link, pie, icon navigation and access model", async () => {
  api.permissions = [
    "accounts:read",
    "departments:read",
    "organizations:read",
    "roles:read",
  ];
  api.read.mockImplementation(async (name: string) =>
    name === "application.runtime.list"
      ? { items: [dashboard] }
      : {
          ...data,
          departments: 6,
          distribution: [{ label: "Platform", value: 1 }],
        },
  );
  mount("/analytics");
  await screen.findByText("6 departments");
  const bars = screen.getByRole("list", { name: "People by department" });
  const departments = screen.getByRole("link", {
    name: "Department management",
  });
  expect(
    bars.compareDocumentPosition(departments) &
      Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  const organization = screen.getByRole("link", {
    name: /Organization structure/,
  });
  const pie = screen.getByRole("figure", {
    name: "Department membership share",
  });
  expect(within(pie).getByText("1 · 100%")).toBeVisible();
  expect(organization.querySelector(".entity-symbol")).toBeInTheDocument();
  expect(screen.getByText("Access model")).toBeVisible();

  await userEvent.click(screen.getByRole("tab", { name: "Recently created" }));
  expect(
    screen
      .getByRole("link", { name: /Alice Chen/ })
      .querySelector(".entity-symbol"),
  ).toBeInTheDocument();
});
