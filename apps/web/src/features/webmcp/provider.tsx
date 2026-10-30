import {
  createContext,
  useContext,
  useEffect,
  useState,
  useSyncExternalStore,
  type PropsWithChildren,
} from "react";
import { useLocation, useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { useSession } from "@/features/administration/auth";
import {
  PageToolRegistry,
  pageTool,
  isToolAvailable,
  type PageTool,
  type ModelContext,
} from "@/lib/webmcp/runtime";
import {
  defaultWebMcpPort,
  endpoint,
  setupClient,
  bindPage,
  type AgentClient,
} from "@/lib/webmcp/connection";
import { ApiError } from "@/api/client/request";
import { findWebMcpRoute, webMcpRoutes } from "@/lib/webmcp/routes";

export type ConnectionStatus =
  "disconnected" | "connecting" | "connected" | "failed";
class WebMcpConnection {
  readonly registry = new PageToolRegistry();
  private client?: { close: () => Promise<void> };
  private generation = 0;
  private state: ConnectionStatus = "disconnected";
  private listeners = new Set<() => void>();
  private token = "";
  private sessionId = "";
  private removeToolSubscription?: () => void;
  private notificationPending = false;
  private localReady = false;
  port = defaultWebMcpPort;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  snapshot = () => this.state;
  private update(status: ConnectionStatus) {
    this.state = status;
    for (const listener of this.listeners) listener();
  }
  constructor() {
    this.registry.onError = () => {
      void this.disconnect().catch(() =>
        console.error(
          "WebMCP transport cleanup failed; page tools are disabled.",
        ),
      );
      this.update("failed");
    };
  }
  /**
   * Publish page tools to the browser builtin WebMCP context so the embedded
   * assistant can read the same registry as the remote MCP client. Callers must be
   * an explicit capability: a paired remote connection, or an enabled assistant.
   * Revocation still withdraws every page tool.
   */
  async ensureLocalTools() {
    if (this.localReady && this.registry.isActive()) return;
    const { initializeBuiltinWebMCP } = await import("@opentiny/next-sdk/core");
    initializeBuiltinWebMCP();
    const context = (document as Document & { modelContext?: ModelContext })
      .modelContext;
    if (!context) throw new Error("webmcp_unavailable");
    await this.registry.activate(context);
    if (!this.registry.isActive()) throw new Error("webmcp_unavailable");
    this.localReady = true;
  }
  async connect(port: number, token: string) {
    endpoint(port);
    if (!/^[a-f0-9]{64}$/.test(token)) throw new Error("invalid_pairing_key");
    await this.disconnect();
    const generation = ++this.generation;
    this.update("connecting");
    let candidate: { close: () => Promise<void> } | undefined;
    try {
      const { WebMcpClient } = await import("@opentiny/next-sdk/core");
      if (generation !== this.generation) return;
      await this.ensureLocalTools();
      if (generation !== this.generation) return;
      const client = new WebMcpClient();
      candidate = client;
      this.client = client;
      client.onclose = () => {
        if (generation === this.generation) this.revoke("disconnected");
      };
      const connection = await client.connect({
        agent: true,
        builtin: true,
        type: "stream",
        url: `${endpoint(port)}/api/v1/webmcp/mcp`,
        token,
      });
      if (generation !== this.generation) {
        await client.close();
        return;
      }
      await bindPage(port, token, connection.sessionId);
      if (generation !== this.generation) {
        await client.close();
        return;
      }
      this.port = port;
      this.token = token;
      this.sessionId = connection.sessionId;
      this.update("connected");
      this.removeToolSubscription = this.registry.subscribe(() => {
        if (this.notificationPending) return;
        this.notificationPending = true;
        queueMicrotask(() => {
          this.notificationPending = false;
          if (generation !== this.generation) return;
          void connection.transport
            .send({
              jsonrpc: "2.0",
              method: "notifications/tools/list_changed",
            })
            .catch(() => this.registry.onError?.());
        });
      });
    } catch {
      if (generation === this.generation) this.revoke("failed");
      if (candidate) await candidate.close();
      throw new Error("connection_failed");
    }
  }
  private revoke(status: ConnectionStatus) {
    ++this.generation;
    this.removeToolSubscription?.();
    this.removeToolSubscription = undefined;
    // Signing out, a 401 or a failed pairing withdraws every page tool; the
    // assistant must request publication again through ensureLocalTools.
    this.localReady = false;
    this.registry.deactivate();
    this.token = "";
    this.sessionId = "";
    this.update(status);
  }
  shutdown() {
    this.localReady = false;
    this.registry.deactivate();
  }
  async disconnect() {
    const client = this.client;
    this.client = undefined;
    this.revoke("disconnected");
    if (client) await client.close();
  }
  async install(client: AgentClient) {
    if (this.state !== "connected") throw new Error("not_connected");
    await setupClient(this.port, this.token, this.sessionId, client);
  }
}
const Context = createContext<WebMcpConnection | null>(null);
export function WebMcpProvider({ children }: PropsWithChildren) {
  const [connection] = useState(() => new WebMcpConnection());
  useEffect(() => {
    const revoke = () => {
      void connection.disconnect().catch(() => {
        console.error(
          "WebMCP transport cleanup failed; page tools are disabled.",
        );
      });
    };
    window.addEventListener("baseline:unauthenticated", revoke);
    window.addEventListener("baseline:webmcp-revoke", revoke);
    window.addEventListener("pagehide", revoke);
    return () => {
      window.removeEventListener("baseline:unauthenticated", revoke);
      window.removeEventListener("baseline:webmcp-revoke", revoke);
      window.removeEventListener("pagehide", revoke);
      void connection.disconnect().catch(() => {
        console.error(
          "WebMCP transport cleanup failed; page tools are disabled.",
        );
      });
      connection.shutdown();
    };
  }, [connection]);
  return (
    <Context.Provider value={connection}>
      <WorkspaceTools />
      {children}
    </Context.Provider>
  );
}
export function useWebMcp() {
  const value = useContext(Context);
  if (!value) throw new Error("WebMcpProvider required");
  const status = useSyncExternalStore(value.subscribe, value.snapshot);
  const toolCount = useSyncExternalStore(
    value.registry.subscribe,
    value.registry.snapshot,
  );
  return { connection: value, status, toolCount };
}
export function usePageTools(tools: PageTool[]) {
  const connection = useContext(Context);
  const session = useSession();
  useEffect(() => {
    if (!connection) return;
    const cleanup = tools
      .filter((tool) => isToolAvailable(tool, session.permissions))
      .map((tool) => connection.registry.register(tool));
    return () => cleanup.forEach((remove) => remove());
  }, [connection, tools, session.permissions]);
}
const routes = {
  "/": "",
  "/accounts": "accounts:read",
  "/organizations": "organizations:read",
  "/departments": "departments:read",
  "/roles": "roles:read",
  "/permissions": "access:manage",
  "/settings": "settings:read",
  "/preferences": "",
  "/dictionaries": "dictionaries:read",
  "/attachments": "files:read",
  "/notifications": "",
  "/temporary-access": "",
  "/webmcp": "",
} as const;
function WorkspaceTools() {
  const session = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const client = useQueryClient();
  // Deliberately project counts and stable error codes; never cache payloads, keys, errors or credentials.
  const tools = [
    pageTool(
      "baseline_page_context",
      "Read current route and allowed navigation destinations; no business records or credentials.",
      z.object({}).strict(),
      () => ({
        route: location.pathname,
        routes: Object.entries(routes)
          .filter(
            ([, permission]) =>
              !permission || session.permissions.includes(permission),
          )
          .map(([path]) => path),
      }),
      { scope: "global" },
    ),
    pageTool(
      "baseline_navigate",
      "Navigate to a permitted workspace route. Use page-specific tools after navigation.",
      z
        .object({
          path: z.enum(
            Object.keys(routes) as [
              keyof typeof routes,
              ...(keyof typeof routes)[],
            ],
          ),
        })
        .strict(),
      ({ path }) => {
        const permission = routes[path];
        if (permission && !session.permissions.includes(permission))
          throw new Error("forbidden");
        void navigate(path);
        return { requestedRoute: path };
      },
      { scope: "global", effect: "navigation" },
    ),
    pageTool(
      "baseline_diagnostics",
      "Read counts and stable HTTP error codes only. Use browser trace for layout/network details.",
      z.object({}).strict(),
      () => {
        const queries = client.getQueryCache().getAll();
        return {
          route: location.pathname,
          pending: queries.filter((q) => q.state.fetchStatus === "fetching")
            .length,
          errors: queries
            .filter((q) => q.state.status === "error")
            .map((q) =>
              q.state.error instanceof ApiError
                ? { code: q.state.error.code, status: q.state.error.status }
                : { code: "unavailable" },
            )
            .slice(0, 20),
          dialogs: document.querySelectorAll('[role="dialog"]').length,
          invalidFields: document.querySelectorAll('[aria-invalid="true"]')
            .length,
        };
      },
      { scope: "global" },
    ),
  ];
  usePageTools(tools);
  const current = findWebMcpRoute(location.pathname);
  const allowed =
    current?.connected &&
    (!current.permission || session.permissions.includes(current.permission));
  usePageTools(
    allowed && current
      ? [
          pageTool(
            "baseline_module_context",
            "Describe the current module and its permitted pages. No business records are returned.",
            z.object({}).strict(),
            () => ({
              module: current.module,
              routes: webMcpRoutes
                .filter(
                  (route) =>
                    route.connected &&
                    route.module === current.module &&
                    (!route.permission ||
                      session.permissions.includes(route.permission)),
                )
                .map((route) => route.path),
            }),
            { scope: "module", permission: current.permission || null },
          ),
          pageTool(
            "baseline_current_page",
            "Describe the current page and its visible interaction state without field values or records.",
            z.object({}).strict(),
            () => ({
              route: current.path,
              module: current.module,
              dialogs: document.querySelectorAll('[role="dialog"]').length,
              invalidFields: document.querySelectorAll('[aria-invalid="true"]')
                .length,
            }),
            { scope: "page", permission: current.permission || null },
          ),
        ]
      : [],
  );
  return null;
}
