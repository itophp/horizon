import { expect, it } from "vitest";
import { projectFrontendPlugin } from "./plugin";
import { resources } from "@/i18n/config";

it("contributes only translations and permission labels, not page routes", () => {
  expect(projectFrontendPlugin.routes).toEqual([]);
  expect(projectFrontendPlugin.menus).toEqual([]);
  expect(resources.en.projects.heading).toBeTruthy();
  expect(resources["zh-CN"].projects.heading).toBeTruthy();
  expect(projectFrontendPlugin.permissionFunctions?.projects).toBe(
    "projects:heading",
  );
});
