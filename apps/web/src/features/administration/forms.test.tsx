import { beforeEach, it, expect, vi } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { changeLanguage } from "@/i18n";
import type {
  AccountRecord,
  AdminSnapshot,
  SessionProfile,
} from "@/api/generated/models";
import { AccountForm } from "./forms";
const mocks = vi.hoisted(() => ({
  session: vi.fn(),
  data: vi.fn(),
  update: vi.fn(),
  refresh: vi.fn(),
}));
vi.mock("./auth", () => ({ useSession: mocks.session }));
vi.mock("./shared", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./shared")>()),
  useAdminData: mocks.data,
  useRefreshAdmin: () => mocks.refresh,
}));
vi.mock("@/api/generated/baseline", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/api/generated/baseline")>()),
  useIdentityCreateAccount: () => ({ isPending: false, mutateAsync: vi.fn() }),
  useIdentityUpdateAccount: () => ({
    isPending: false,
    mutateAsync: mocks.update,
  }),
}));
const account: AccountRecord = {
  id: 1,
  username: "root-admin",
  fullName: "Administrator",
  email: "",
  status: "active",
  organizationId: 1,
  departmentId: 1,
  roleIds: [1],
  overrides: [],
  isAdmin: true,
  createdAt: "2026-10-29T00:00:00Z",
  lastLogin: "",
  memberships: [
    {
      organizationId: 1,
      departmentId: 1,
      departmentName: "Administration",
      roleIds: [1],
      overrides: [],
    },
  ],
};
const data: AdminSnapshot = {
  accounts: [account],
  organizations: [
    {
      id: 1,
      name: "Company",
      kind: "company",
      parentId: 0,
      code: "company",
      status: "active",
      description: "",
      createdAt: "",
    },
  ],
  departments: [
    {
      id: 1,
      name: "Administration",
      organizationId: 1,
      parentId: 0,
      code: "admin",
      status: "active",
      manager: "",
      description: "",
      createdAt: "",
    },
    {
      id: 2,
      name: "Child team",
      organizationId: 1,
      parentId: 1,
      code: "child",
      status: "active",
      manager: "",
      description: "",
      createdAt: "",
    },
  ],
  roles: [
    {
      id: 1,
      name: "Reader",
      code: "reader",
      status: "active",
      description: "",
      grants: [],
      memberCount: 0,
      createdAt: "",
    },
  ],
  permissions: ["access:manage", "accounts:update"],
  catalog: [
    {
      code: "accounts:read",
      module: "identity",
      function: "accounts",
      action: "read",
      temporaryEligible: true,
      scopeConfigurable: true,
    },
  ],
};
const session: SessionProfile = {
  account,
  tenantId: "test",
  activeDepartmentId: 1,
  csrfToken: "test",
  permissions: data.permissions,
};
beforeEach(async () => {
  vi.clearAllMocks();
  await changeLanguage("en");
  mocks.session.mockReturnValue(session);
  mocks.data.mockReturnValue({ data });
  mocks.refresh.mockResolvedValue(undefined);
  mocks.update.mockResolvedValue(undefined);
});
it("lets the built-in administrator configure an additional child department without unlocking primary permissions", async () => {
  const user = userEvent.setup();
  const saved = vi.fn();
  render(<AccountForm record={account} onSaved={saved} />);
  await user.click(
    screen.getByRole("button", {
      name: "Departments & permissions",
    }),
  );
  expect(screen.getByRole("checkbox", { name: /Reader/ })).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  await user.selectOptions(screen.getByLabelText("Additional department"), "2");
  await user.click(screen.getByRole("button", { name: "Add department" }));
  const reader = screen.getByRole("checkbox", { name: /Reader/ });
  expect(reader).not.toHaveAttribute("aria-disabled", "true");
  await user.click(reader);
  await user.click(
    within(screen.getByRole("group", { name: "Accounts View" })).getByRole(
      "button",
      { name: "Allow" },
    ),
  );
  await user.click(
    screen.getByRole("button", {
      name: "Administration · Primary department",
    }),
  );
  expect(screen.getByRole("checkbox", { name: /Reader/ })).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  expect(
    within(screen.getByRole("group", { name: "Accounts View" })).getByRole(
      "button",
      { name: "Allow" },
    ),
  ).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  await waitFor(() => expect(saved).toHaveBeenCalledOnce());
  expect(mocks.update).toHaveBeenCalledWith(
    expect.objectContaining({
      id: 1,
      data: expect.objectContaining({
        departmentId: 1,
        roleIds: [1],
        overrides: [],
        departmentAccess: {
          memberships: [
            {
              organizationId: 1,
              departmentId: 2,
              roleIds: [1],
              overrides: [
                {
                  permission: "accounts:read",
                  effect: "allow",
                  scope: "DEPARTMENT",
                  departmentIds: [],
                },
              ],
            },
          ],
        },
      }),
    }),
  );
});
it.each(["other-owner", "no-permission", "read-only"])(
  "keeps department controls protected for %s",
  async (mode) => {
    mocks.session.mockReturnValue({
      ...session,
      account: mode === "other-owner" ? { ...account, id: 2 } : account,
      permissions: mode === "no-permission" ? [] : session.permissions,
    });
    render(
      <AccountForm
        record={account}
        readOnly={mode === "read-only"}
        permissionsOnly
        onSaved={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Add department" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /Reader/ })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
  },
);
