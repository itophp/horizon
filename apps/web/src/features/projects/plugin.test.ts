import { expect, it } from "vitest";
import { projectFrontendPlugin } from "./plugin";
import { resources } from "@/i18n/config";
import { canShowMenu } from "@/app/plugins/registry";

it("contributes project routes, the permission-protected menu and both translations", () => {
  expect(projectFrontendPlugin.routes.map((route) => route.path)).toEqual([
    "/projects",
    "/projects/:id",
  ]);
  expect(projectFrontendPlugin.menus).toMatchObject([
    {
      to: "/projects",
      labelKey: "projects:heading",
      permissions: ["projects:read"],
    },
  ]);
  expect(resources.en.projects.heading).toBeTruthy();
  expect(resources["zh-CN"].projects.heading).toBeTruthy();
});

it("shows the project menu only with its existing read permission", () => {
  const [menu] = projectFrontendPlugin.menus;
  expect(canShowMenu(menu.permissions, [])).toBe(false);
  expect(canShowMenu(menu.permissions, ["projects:read"])).toBe(true);
});
