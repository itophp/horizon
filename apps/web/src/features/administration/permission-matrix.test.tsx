import { it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { changeLanguage } from "@/i18n";
import type { PermissionDefinition } from "@/api/generated/models";
import { PermissionMatrix } from "./permission-matrix";
const catalog: PermissionDefinition[] = [
  {
    code: "accounts:read",
    module: "identity",
    function: "accounts",
    action: "read",
    temporaryEligible: true,
    scopeConfigurable: true,
  },
  {
    code: "accounts:update",
    module: "identity",
    function: "accounts",
    action: "update",
    temporaryEligible: true,
    scopeConfigurable: true,
  },
  {
    code: "departments:read",
    module: "identity",
    function: "departments",
    action: "read",
    temporaryEligible: true,
    scopeConfigurable: true,
  },
  {
    code: "access:manage",
    module: "access",
    function: "policies",
    action: "manage",
    temporaryEligible: false,
    scopeConfigurable: false,
  },
  {
    code: "temporary:approve",
    module: "access",
    function: "temporary",
    action: "approve",
    temporaryEligible: false,
    scopeConfigurable: false,
  },
];
it("groups function rows under merged module cells and keeps approval independent", async () => {
  await changeLanguage("en");
  const onChange = vi.fn();
  render(
    <PermissionMatrix
      catalog={catalog}
      grants={[]}
      onChange={onChange}
      departments={[]}
    />,
  );
  expect(
    screen.getByText("Administration", { selector: "th" }),
  ).toHaveAttribute("rowspan", "2");
  expect(
    screen.getByText("Access control", { selector: "th" }),
  ).toHaveAttribute("rowspan", "2");
  await userEvent.click(
    screen.getByRole("checkbox", {
      name: "Temporary access approval",
    }),
  );
  expect(onChange).toHaveBeenLastCalledWith([
    {
      permission: "temporary:approve",
      effect: "allow",
      scope: "ALL",
      departmentIds: [],
    },
  ]);
});
it("request mode exposes eligible functions and allows a broader data scope", async () => {
  await changeLanguage("en");
  const onChange = vi.fn();
  render(
    <PermissionMatrix
      catalog={catalog}
      grants={[]}
      onChange={onChange}
      departments={[]}
      request
    />,
  );
  expect(
    screen.queryByRole("checkbox", {
      name: "Temporary access approval",
    }),
  ).not.toBeInTheDocument();
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: "Data scope" }),
    "ALL",
  );
  await userEvent.click(
    screen.getByRole("checkbox", { name: "Accounts View" }),
  );
  expect(onChange).toHaveBeenLastCalledWith([
    {
      permission: "accounts:read",
      effect: "allow",
      scope: "ALL",
      departmentIds: [],
    },
  ]);
});
