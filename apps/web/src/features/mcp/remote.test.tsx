import { readFileSync } from "node:fs";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { ApiError } from "@/api/client/request";
import type { MCPToken } from "@/api/types";
import { MemoryRouter } from "react-router";
import { DynamicApplicationRoute } from "@/features/application-runtime/route";
import type { RuntimeApplicationV2 } from "@/api/generated/runtime";

const collaboration = (
  JSON.parse(
    readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
  ) as RuntimeApplicationV2[]
).find((app) => app.id === "agent-collaboration" && app.version === "1.0.2")!;

const api = vi.hoisted(() => ({
  create: vi.fn(),
  rotate: vi.fn(),
  revoke: vi.fn(),
  adminRevoke: vi.fn(),
  list: vi.fn(),
  permissions: [] as string[],
}));

vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeExecute: async (
    name: string,
    _version: string,
    input: Record<string, unknown>,
  ) => {
    const { id, ...values } = input;
    const response =
      name === "mcp.token.create"
        ? await api.create(values)
        : name === "mcp.token.rotate"
          ? await api.rotate(id, values)
          : name === "mcp.token.revoke"
            ? await api.revoke(id, values)
            : name === "mcp.token.admin_revoke"
              ? await api.adminRevoke(id, values)
              : null;
    if (!response) throw new Error("Unexpected action: " + name);
    return { status: "succeeded", data: response.data };
  },
  runtimeApplication: async () => collaboration,
  runtimeRead: async (name: string) => {
    if (name === "application.runtime.list") return { items: [collaboration] };
    if (name === "mcp.token.list") return api.list();
    if (name === "mcp.audit.list") return [];
    throw new Error("Unexpected read: " + name);
  },
}));

vi.mock("@/features/administration/auth", () => {
  const useSession = () => ({
    tenantId: "t",
    activeDepartmentId: 1,
    csrfToken: "csrf",
    permissions: api.permissions,
    account: {
      id: 1,
      memberships: [
        { departmentId: 1, departmentName: "Administration" },
        { departmentId: 2, departmentName: "Engineering" },
      ],
    },
  });
  return { useSession, useAuth: () => ({ profile: useSession() }) };
});

vi.mock("@/features/webmcp/provider", () => ({
  useWebMcp: () => ({ status: "disconnected", toolCount: 0 }),
}));

const token: MCPToken = {
  id: 7,
  name: "CI reader",
  prefix: "bmcp_7ab",
  departmentId: 1,
  expiresAt: "2026-12-01T00:00:00Z",
  createdAt: "2026-10-01T00:00:00Z",
  lastUsedAt: "",
  revokedAt: "",
  revokedBy: 0,
  tools: ["context_get"],
};

function show() {
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter initialEntries={["/mcp?tab=remote"]}>
          <DynamicApplicationRoute />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

async function openCreate(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole("button", { name: "Add token" }));
  return screen.findByRole("button", { name: "Create token" });
}

async function fill(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Token name"), "CI reader");
  await user.type(screen.getByLabelText("Current password"), "correct-horse");
}

beforeEach(async () => {
  await changeLanguage("en", false);
  api.create.mockReset();
  api.rotate.mockReset();
  api.revoke.mockReset();
  api.adminRevoke.mockReset();
  api.list.mockReset();
  api.permissions = [];
  api.list.mockResolvedValue([]);
});

it("associates every validation error with its own field", async () => {
  const user = userEvent.setup();
  show();
  await user.click(await openCreate(user));

  expect(
    await screen.findByText("Enter a token name of 1–100 characters."),
  ).toBeVisible();
  const name = screen.getByLabelText("Token name");
  expect(name).toHaveAttribute("aria-invalid", "true");
  expect(name).toHaveAccessibleDescription(
    "Enter a token name of 1–100 characters.",
  );
  expect(screen.getByLabelText("Current password")).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  expect(api.create).not.toHaveBeenCalled();
});

it("keeps the password when the server rejects the request", async () => {
  const user = userEvent.setup();
  api.create.mockRejectedValue(new ApiError("forbidden", 403));
  show();
  await openCreate(user);
  await fill(user);
  await user.click(screen.getByRole("button", { name: "Create token" }));

  expect(await screen.findByRole("alert")).toHaveTextContent(
    "The password is wrong, or this department may not issue tokens.",
  );
  expect(screen.getByLabelText("Current password")).toHaveValue(
    "correct-horse",
  );
  expect(api.create).toHaveBeenCalledWith({
    name: "CI reader",
    days: 30,
    departmentId: 1,
    password: "correct-horse",
  });
});

it("shows the secret once, then clears the password before the next token", async () => {
  const user = userEvent.setup();
  api.create.mockResolvedValue({
    status: 200,
    data: { secret: "bmcp_secret", token: { id: 1 } },
  });
  show();
  await openCreate(user);
  await fill(user);
  await user.click(screen.getByRole("button", { name: "Create token" }));

  expect(await screen.findByText("bmcp_secret")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Copy token" }));
  expect(await navigator.clipboard.readText()).toBe("bmcp_secret");
  expect(screen.queryByLabelText("Current password")).not.toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "I have saved the token" }),
  );
  await openCreate(user);
  await waitFor(() =>
    expect(screen.getByLabelText("Current password")).toHaveValue(""),
  );
});

it("explains that credentials follow current permissions without selecting tools", async () => {
  const user = userEvent.setup();
  show();
  await openCreate(user);
  expect(
    screen.getAllByText(/Access is checked on each request/).length,
  ).toBeGreaterThan(0);
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
});

it("confirms rotating a listed token with the password inside a dialog", async () => {
  const user = userEvent.setup();
  api.permissions = ["mcp:revoke"];
  api.list.mockResolvedValue([token]);
  api.rotate.mockResolvedValue({
    status: 200,
    data: { secret: "bmcp_rotated", token: { id: 7 } },
  });
  show();

  await user.click(await screen.findByRole("button", { name: "Rotate" }));
  expect(screen.getByText("Rotate token: CI reader")).toBeVisible();
  await user.type(screen.getByLabelText("Current password"), "correct-horse");
  await user.click(screen.getByRole("button", { name: "Rotate now" }));

  await waitFor(() =>
    expect(api.rotate).toHaveBeenCalledWith(7, { password: "correct-horse" }),
  );
  expect(await screen.findByText("bmcp_rotated")).toBeVisible();
  await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
});

it("requires the management permission before offering an emergency revoke", async () => {
  show();
  await screen.findByRole("button", { name: "Add token" });
  expect(
    screen.queryByRole("button", { name: "Emergency revoke" }),
  ).not.toBeInTheDocument();
});

it("revokes a token by id from the emergency revoke dialog", async () => {
  const user = userEvent.setup();
  api.permissions = ["mcp:revoke"];
  api.adminRevoke.mockResolvedValue({ status: 204, data: undefined });
  show();

  await user.click(
    await screen.findByRole("button", { name: "Emergency revoke" }),
  );
  expect(screen.getByText("Emergency token revoke")).toBeVisible();
  await user.type(screen.getByLabelText("Token ID to revoke"), "7");
  await user.type(
    screen.getByLabelText("Administrator password"),
    "admin-pass",
  );
  await user.click(screen.getByRole("button", { name: "Revoke immediately" }));

  await waitFor(() =>
    expect(api.adminRevoke).toHaveBeenCalledWith(7, { password: "admin-pass" }),
  );
  expect(await screen.findByText("Token revoked.")).toBeVisible();
});
