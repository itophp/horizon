// This inventory includes public routes even though the authenticated provider
// deliberately disconnects before entering them.
export const webMcpRoutes = [
  {
    path: "/articles",
    module: "articles",
    permission: "articles:read",
    connected: false,
  },
  { path: "/mcp", module: "development", permission: "", connected: true },
  // Legacy entry points redirect to the unified page; kept so older links and
  // agent instructions still resolve.
  {
    path: "/mcp-settings",
    module: "development",
    permission: "",
    connected: true,
  },
  { path: "/", module: "workspace", permission: "", connected: true },
  // Statistics took over the former landing dashboard; "/" now redirects to the
  // AI workbench, so the page keeps its own path.
  { path: "/analytics", module: "workspace", permission: "", connected: true },
  // The workbench is a first class console surface: its page tools are only
  // published while the agent grant is held.
  {
    path: "/capabilities",
    module: "capability",
    permission: "example:read",
    connected: true,
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
    path: "/accounts",
    module: "administration",
    permission: "accounts:read",
    connected: true,
  },
  {
    path: "/organizations",
    module: "administration",
    permission: "organizations:read",
    connected: true,
  },
  // Departments, roles and dictionaries are merged into their parent pages and
  // only kept as redirects for older links and agent instructions.
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
    path: "/permissions",
    module: "administration",
    permission: "access:manage",
    connected: true,
  },
  {
    path: "/temporary-access",
    module: "administration",
    permission: "",
    connected: true,
  },
  {
    path: "/settings",
    module: "governance",
    permission: "settings:read",
    connected: true,
  },
  {
    path: "/preferences",
    module: "governance",
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

export function findWebMcpRoute(path: string) {
  return (
    webMcpRoutes.find((route) => route.path === path) ??
    (/^\/files\/[^/]+$/.test(path)
      ? webMcpRoutes.find((route) => route.path === "/files/:id")
      : undefined)
  );
}
