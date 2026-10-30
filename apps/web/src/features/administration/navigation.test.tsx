import type { ReactNode } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { changeLanguage } from "@/i18n";
import { OrganizationsPage, PermissionsPage } from "./pages";

const state = vi.hoisted(() => ({
  permissions: [] as string[],
}));
vi.mock("./auth", () => ({
  useSession: () => ({ permissions: state.permissions }),
}));
vi.mock("./shared", async (original) => {
  const actual = await original<typeof import("./shared")>();
  return {
    ...actual,
    useAdminData: () => ({
      data: { organizations: [], departments: [], accounts: [], roles: [] },
      isPending: false,
      isError: false,
    }),
  };
});

function show(page: ReactNode) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/"]}>{page}</MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(async () => {
  state.permissions = [];
  await changeLanguage("en", false);
});

it("keeps departments inside organization management", async () => {
  state.permissions = [
    "organizations:read",
    "departments:read",
    "departments:create",
  ];
  show(<OrganizationsPage />);
  expect(
    screen.getByRole("heading", { name: "Organization management" }),
  ).toBeVisible();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Departments" }));
  expect(
    await screen.findByRole("button", { name: "Create department" }),
  ).toBeVisible();
});

it("hides the department tab without its read permission", () => {
  state.permissions = ["organizations:read"];
  show(<OrganizationsPage />);
  expect(screen.queryByRole("button", { name: "Departments" })).toBeNull();
});

it("keeps role maintenance inside permission management", async () => {
  state.permissions = ["access:manage", "roles:read", "roles:create"];
  show(<PermissionsPage />);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Roles" }));
  expect(
    await screen.findByRole("button", { name: "Create role" }),
  ).toBeVisible();
});

it("hides role maintenance without the roles read permission", () => {
  state.permissions = ["access:manage"];
  show(<PermissionsPage />);
  expect(screen.queryByRole("button", { name: "Roles" })).toBeNull();
});
