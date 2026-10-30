import { expect, it, vi } from "vitest";
import { z } from "zod";
import {
  PageToolRegistry,
  pageTool,
  isToolAvailable,
  type ModelContext,
  type PageTool,
} from "./runtime";
it("requires explicit activation, validates arguments and revokes tools including captured handles", async () => {
  const registry = new PageToolRegistry();
  const execute = vi.fn(() => ({ ok: true }));
  const unregister = registry.register(
    pageTool(
      "test_action",
      "test",
      z.object({ id: z.number().int().positive() }).strict(),
      execute,
    ),
  );
  expect((await registry.invoke("test_action", { id: 1 })).isError).toBe(true);
  const tools = new Map<string, PageTool>();
  const context: ModelContext = {
    registerTool: (tool, { signal }) => {
      tools.set(tool.name, tool);
      signal.addEventListener("abort", () => {
        if (tools.get(tool.name) === tool) tools.delete(tool.name);
      });
    },
    getTools: () => [...tools.values()],
  };
  await registry.activate(context);
  const captured = tools.get("test_action")!;
  expect((await captured.execute({ id: -1 })).isError).toBe(true);
  expect(
    (await captured.execute({ id: 1, tenantId: "injected" })).isError,
  ).toBe(true);
  expect(execute).not.toHaveBeenCalled();
  expect((await captured.execute({ id: 1 })).isError).toBeUndefined();
  expect(execute).toHaveBeenCalledOnce();
  registry.deactivate();
  expect(tools.size).toBe(0);
  expect((await captured.execute({ id: 1 })).isError).toBe(true);
  expect(execute).toHaveBeenCalledOnce();
  await registry.activate(context);
  unregister();
  expect((await registry.invoke("test_action", { id: 1 })).isError).toBe(true);
});
it("returns stable errors without exposing exceptions", async () => {
  const tool = pageTool("failure", "test", z.object({}).strict(), () => {
    throw new Error("password=private");
  });
  const result = await tool.execute({});
  expect(result.isError).toBe(true);
  expect(JSON.stringify(result)).not.toContain("private");
});
it("records scope metadata and rejects duplicate registrations", () => {
  const registry = new PageToolRegistry();
  const tool = pageTool(
    "module_status",
    "status",
    z.object({}).strict(),
    () => ({ ok: true }),
    { scope: "module", permission: "accounts:read" },
  );
  const remove = registry.register(tool);
  expect(registry.list()[0].metadata).toEqual({
    scope: "module",
    version: "1.0.0",
    effect: "read",
    permission: "accounts:read",
  });
  expect(() => registry.register(tool)).toThrow("duplicate_tool");
  expect(isToolAvailable(tool, [])).toBe(false);
  expect(isToolAvailable(tool, ["accounts:read"])).toBe(true);
  remove();
  expect(registry.list()).toEqual([]);
});
