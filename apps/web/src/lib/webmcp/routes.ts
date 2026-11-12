import { runtimeProtocolRoutes } from "@/api/generated/runtime";

const runtimeRoutePolicy: Record<
  string,
  { module: string; permission?: string }
> = {
  "/capabilities": { module: "capability", permission: "example:read" },
  "/analytics": { module: "workspace" },
  "/mcp": { module: "development" },
  "/settings": { module: "governance", permission: "settings:read" },
  "/preferences": { module: "governance" },
};

const runtimeRoutes = runtimeProtocolRoutes.map((route) => {
  const policy = runtimeRoutePolicy[route.route];
  return {
    path: route.route,
    module: policy?.module ?? route.applicationId,
    permission:
      policy?.permission ??
      (route.permission === "authenticated" ? "" : route.permission),
    connected: true,
  };
});

// This inventory includes public routes even though the authenticated provider
// deliberately disconnects before entering them.
export const webMcpRoutes = [
  ...runtimeRoutes,
  {
    path: "/applications/design",
    module: "runtime",
    permission: "applications:design",
    connected: false,
  },
  {
    path: "/applications",
    module: "runtime",
    permission: "applications:read",
    connected: false,
  },
  {
    path: "/drive",
    module: "drive",
    permission: "drive:use",
    connected: false,
  },
  {
    path: "/drive/:id",
    module: "drive",
    permission: "drive:use",
    connected: false,
  },
  {
    path: "/drive/:id/members",
    module: "drive",
    permission: "drive:use",
    connected: false,
  },
  {
    path: "/drive/:id/trash",
    module: "drive",
    permission: "drive:use",
    connected: false,
  },
  {
    path: "/mcp-settings",
    module: "development",
    permission: "",
    connected: true,
  },
  { path: "/", module: "workspace", permission: "", connected: true },
  {
    path: "/capabilities/:name",
    module: "capability",
    permission: "executionPlans:read",
    connected: false,
  },
  {
    path: "/orchestration",
    module: "orchestration",
    permission: "orchestration:read",
    connected: false,
  },
  {
    path: "/ai",
    module: "ai",
    permission: "assistant:use",
    connected: true,
  },
  {
    path: "/knowledge",
    module: "knowledge",
    permission: "knowledge:read",
    connected: true,
  },
  {
    path: "/departments",
    module: "administration",
    permission: "departments:read",
    connected: true,
  },
  {
    path: "/roles",
    module: "administration",
    permission: "roles:read",
    connected: true,
  },
  {
    path: "/temporary-access",
    module: "administration",
    permission: "",
    connected: true,
  },
  {
    path: "/dictionaries",
    module: "governance",
    permission: "dictionaries:read",
    connected: true,
  },
  {
    path: "/attachments",
    module: "governance",
    permission: "files:read",
    connected: true,
  },
  {
    path: "/files/:id",
    module: "governance",
    permission: "files:read",
    connected: true,
  },
  {
    path: "/notifications",
    module: "governance",
    permission: "",
    connected: true,
  },
  {
    path: "/notifications/:id",
    module: "governance",
    permission: "",
    connected: true,
  },
  {
    path: "/notifications/compose",
    module: "governance",
    permission: "notifications:send",
    connected: true,
  },
  { path: "/webmcp", module: "development", permission: "", connected: true },
  { path: "/login", module: "public", permission: "", connected: false },
  { path: "/foundation", module: "example", permission: "", connected: false },
  {
    path: "/architecture",
    module: "example",
    permission: "",
    connected: false,
  },
  { path: "*", module: "public", permission: "", connected: false },
] as const;

function matchesRoute(routePath: string, path: string) {
  const expression = routePath
    .replace(/[.+?^${}()|[\]\\]/g, "\\$&")
    .replace(/:[A-Za-z0-9_]+/g, "[^/]+");
  return new RegExp(`^${expression}$`).test(path);
}

export function findWebMcpRoute(path: string) {
  return (
    webMcpRoutes.find((route) => route.path === path) ??
    webMcpRoutes.find(
      (route) => route.path.includes(":") && matchesRoute(route.path, path),
    )
  );
}
