import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { ApiError } from "@/api/client/request";
import type { MCPToken } from "@/api/generated/models";
import { RemoteMcpSection } from "./remote";

const api = vi.hoisted(() => ({
  create: vi.fn(),
  rotate: vi.fn(),
  revoke: vi.fn(),
  adminRevoke: vi.fn(),
  capabilities: vi.fn(),
  list: vi.fn(),
  permissions: [] as string[],
}));

vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  mcpManagementCreate: api.create,
  mcpManagementRotate: api.rotate,
  mcpManagementRevoke: api.revoke,
  mcpManagementAdminRevoke: api.adminRevoke,
  useMcpManagementCapabilities: api.capabilities,
  useMcpManagementList: () => api.list(),
  useMcpManagementAudit: () => ({
    data: { status: 200, data: [] },
    refetch: vi.fn(),
  }),
  getMcpManagementListQueryKey: () => ["/api/auth/mcp/tokens"],
}));

vi.mock("@/features/administration/auth", () => ({
  useAuth: () => ({
    profile: {
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
    },
  }),
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
        <RemoteMcpSection />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

async function openCreate(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Add token" }));
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
  api.capabilities.mockReset();
  api.list.mockReset();
  api.permissions = [];
  api.list.mockReturnValue({
    data: { status: 200, data: [] },
    isPending: false,
    isError: false,
  });
  api.capabilities.mockReturnValue({
    data: {
      status: 200,
      data: ["context_get", "capabilities_list", "notifications_list"],
    },
  });
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

it("keeps the password and the chosen tools when the server rejects the request", async () => {
  const user = userEvent.setup();
  api.create.mockRejectedValue(new ApiError("forbidden", 403));
  show();
  await openCreate(user);
  await fill(user);
  await user.click(screen.getByLabelText("My notifications"));
  await user.click(screen.getByRole("button", { name: "Create token" }));

  expect(await screen.findByRole("alert")).toHaveTextContent(
    "The password is wrong, or this department may not issue tokens.",
  );
  expect(screen.getByLabelText("Current password")).toHaveValue(
    "correct-horse",
  );
  expect(screen.getByLabelText("My notifications")).toBeChecked();
  expect(api.create).toHaveBeenCalledWith({
    name: "CI reader",
    days: 30,
    departmentId: 1,
    tools: ["context_get", "capabilities_list", "notifications_list"],
    password: "correct-horse",
  });
});

it("shows the secret once, then clears the password before the next token", async () => {
  const user = userEvent.setup();
  api.create.mockResolvedValue({
    status: 201,
    data: { secret: "bmcp_secret", token: { id: 1 } },
  });
  show();
  await openCreate(user);
  await fill(user);
  await user.click(screen.getByRole("button", { name: "Create token" }));

  expect(await screen.findByText("bmcp_secret")).toBeVisible();
  expect(screen.queryByLabelText("Current password")).not.toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "I have saved the token" }),
  );
  await openCreate(user);
  await waitFor(() =>
    expect(screen.getByLabelText("Current password")).toHaveValue(""),
  );
});

it("explains when the tool catalog cannot be loaded", async () => {
  api.capabilities.mockReturnValue({
    data: undefined,
    isError: true,
    error: new ApiError("unavailable", 503),
  });
  const user = userEvent.setup();
  show();
  await openCreate(user);
  await fill(user);
  await user.click(screen.getByRole("button", { name: "Create token" }));

  const tools = screen.getByRole("group", { name: "Allowed tools" });
  expect(within(tools).getByRole("alert")).toHaveTextContent(
    "The tool catalog cannot be loaded; no token can be issued without it.",
  );
  expect(api.create).not.toHaveBeenCalled();
});

it("confirms rotating a listed token with the password inside a dialog", async () => {
  const user = userEvent.setup();
  api.list.mockReturnValue({
    data: { status: 200, data: [token] },
    isPending: false,
    isError: false,
  });
  api.rotate.mockResolvedValue({
    status: 201,
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
});

it("requires the management permission before offering an emergency revoke", () => {
  show();
  expect(
    screen.queryByRole("button", { name: "Emergency revoke" }),
  ).not.toBeInTheDocument();
});

it("revokes a token by id from the emergency revoke dialog", async () => {
  const user = userEvent.setup();
  api.permissions = ["mcp:revoke"];
  api.adminRevoke.mockResolvedValue({ status: 204, data: undefined });
  show();

  await user.click(screen.getByRole("button", { name: "Emergency revoke" }));
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
