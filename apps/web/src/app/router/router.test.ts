import { expect, it } from "vitest";
import { defaultWorkspacePath } from "./router";

it("lands on the AI workbench when the account holds the agent grant", () => {
  expect(defaultWorkspacePath(["assistant:use", "accounts:read"])).toBe("/ai");
});

it("falls back to statistics instead of a workbench it may not open", () => {
  expect(defaultWorkspacePath([])).toBe("/analytics");
  expect(defaultWorkspacePath(["accounts:read"])).toBe("/analytics");
});
