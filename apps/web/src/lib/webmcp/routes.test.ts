import { expect, it } from "vitest";
import { router } from "@/app/router/router";
import { findWebMcpRoute, webMcpRoutes } from "./routes";

it("inventories every declared route and identifies parameterized pages", () => {
  function paths(routes: typeof router.routes): string[] {
    return routes.flatMap((route) => [
      ...(route.path ? [route.path] : []),
      ...(route.children ? paths(route.children) : []),
    ]);
  }
  expect(webMcpRoutes.map((route) => route.path).sort()).toEqual(
    paths(router.routes).sort(),
  );
  expect(findWebMcpRoute("/files/123")?.path).toBe("/files/:id");
  expect(findWebMcpRoute("/login")?.connected).toBe(false);
  expect(findWebMcpRoute("/files/123/other")).toBeUndefined();
});
