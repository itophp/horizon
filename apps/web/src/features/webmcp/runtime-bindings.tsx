import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { useLocation } from "react-router";
import { z } from "zod";
import { BrowserRuntimeContext } from "@/features/application-runtime/protocol/browser-runtime";
import { useSession } from "@/features/administration/auth";
import {
  diagnoseRemote,
  installPrepared,
  prepareInstall,
} from "@/lib/mcp/connection";
import { fetchLocalConnection } from "@/lib/webmcp/connection";
import type { ConnectionStatus, WebMcpConnection } from "./provider";

const hexKey = z.string().regex(/^[a-f0-9]{64}$/);
const token = z.string().min(1).max(4096);
const agentClient = z.enum(["codex", "workbuddy"]);
const port = z.number().int().min(1024).max(65535);
const connectionInput = z.strictObject({ port, token: hexKey });
const clientInput = z.strictObject({ client: agentClient });
const diagnosticInput = z.strictObject({ token });
const prepareInput = z.strictObject({
  key: hexKey,
  token,
  client: agentClient,
  departmentId: z.number().int().positive().optional(),
});
const noInput = z.strictObject({});

type Connection = Pick<
  WebMcpConnection,
  "snapshot" | "registry" | "connect" | "disconnect" | "install"
>;

type PublicFeedback = {
  scope: string;
  web?: "connectFailed" | "disconnectFailed" | "setupDone" | "setupFailed";
  helper?: "ready" | "connected" | "installed" | "failed";
  prepared?: { client: string; url: string };
};

type Prepared = {
  scope: string;
  key: string;
  token: string;
  proof: string;
};

export function WebMcpRuntimeBindings({
  connection,
  status,
  toolCount,
  children,
}: PropsWithChildren<{
  connection: Connection;
  status: ConnectionStatus;
  toolCount: number;
}>) {
  const location = useLocation();
  const session = useSession();
  const tab = new URLSearchParams(location.search).get("tab");
  const onMcp = location.pathname === "/mcp";
  const scope = JSON.stringify([
    location.key,
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
  ]);
  const [feedback, setFeedback] = useState<PublicFeedback>();
  const [busyScope, setBusyScope] = useState<string>();
  const busy = useRef(false);
  const generation = useRef(0);
  const prepared = useRef<Prepared | undefined>(undefined);
  useEffect(() => {
    const clear = () => {
      generation.current++;
      prepared.current = undefined;
    };
    const revoke = () => {
      clear();
      setFeedback(undefined);
    };
    window.addEventListener("baseline:unauthenticated", revoke);
    window.addEventListener("baseline:webmcp-revoke", revoke);
    return () => {
      window.removeEventListener("baseline:unauthenticated", revoke);
      window.removeEventListener("baseline:webmcp-revoke", revoke);
      clear();
    };
  }, [scope]);
  const visible = feedback?.scope === scope ? feedback : undefined;
  const formDefaults = useCallback(
    async (preset: string, signal: AbortSignal) => {
      if (
        preset !== "webmcp-pairing" ||
        !onMcp ||
        tab !== "page" ||
        connection.snapshot() !== "disconnected"
      )
        return null;
      const defaults = await fetchLocalConnection(signal);
      return defaults &&
        !signal.aborted &&
        connection.snapshot() === "disconnected"
        ? { port: String(defaults.port), token: defaults.token }
        : null;
    },
    [connection, onMcp, tab],
  );

  async function execute(target: string, values: Record<string, unknown>) {
    if (!onMcp || busy.current) return false;
    const web = target.startsWith("webmcp.");
    if ((web && tab !== "page") || (!web && tab !== "remote")) return false;
    const version = generation.current;
    const show = (data: Omit<PublicFeedback, "scope">) => {
      if (generation.current === version) setFeedback({ scope, ...data });
    };
    busy.current = true;
    setBusyScope(scope);
    show({});
    try {
      switch (target) {
        case "webmcp.connect": {
          const args = connectionInput.parse(values);
          if (connection.snapshot() === "connected") return false;
          await connection.connect(args.port, args.token);
          return connection.snapshot() === "connected";
        }
        case "webmcp.disconnect":
          noInput.parse(values);
          await connection.disconnect();
          return true;
        case "webmcp.install":
          await connection.install(clientInput.parse(values).client);
          show({ web: "setupDone" });
          return true;
        case "remote-mcp.diagnose":
          prepared.current = undefined;
          await diagnoseRemote(diagnosticInput.parse(values).token);
          show({ helper: "connected" });
          return true;
        case "remote-mcp.prepare": {
          prepared.current = undefined;
          const args = prepareInput.parse(values);
          const departmentId = args.departmentId ?? session.activeDepartmentId;
          if (
            !(session.account.memberships ?? []).some(
              (item) => item.departmentId === departmentId,
            )
          )
            return false;
          const result = await prepareInstall(
            args.key,
            args.client,
            session.account.id,
            departmentId,
          );
          if (generation.current !== version) return false;
          prepared.current = {
            scope,
            key: args.key,
            token: args.token,
            proof: result.proof,
          };
          show({
            helper: "ready",
            prepared: { client: result.client, url: result.url },
          });
          return true;
        }
        case "remote-mcp.install": {
          noInput.parse(values);
          const pending = prepared.current;
          prepared.current = undefined;
          if (!pending || pending.scope !== scope) return false;
          await installPrepared(pending.key, pending.proof, pending.token);
          show({ helper: "installed" });
          return true;
        }
        case "remote-mcp.cancel":
          noInput.parse(values);
          prepared.current = undefined;
          show({ helper: "ready" });
          return true;
        default:
          return false;
      }
    } catch {
      if (web) {
        show({
          web:
            target === "webmcp.connect"
              ? "connectFailed"
              : target === "webmcp.disconnect"
                ? "disconnectFailed"
                : "setupFailed",
        });
      } else {
        prepared.current = undefined;
        show({ helper: "failed" });
      }
      return false;
    } finally {
      busy.current = false;
      setBusyScope(undefined);
    }
  }

  const scopes = connection.registry.list().reduce(
    (counts, tool) => {
      const scope = tool.metadata?.scope;
      if (scope === "global" || scope === "module" || scope === "page")
        counts[scope]++;
      return counts;
    },
    { global: 0, module: 0, page: 0 },
  );
  return (
    <BrowserRuntimeContext.Provider
      value={{
        execute,
        formDefaults,
        presentation: {
          origin: window.location.origin,
          status,
          toolCount,
          scopes,
          busy: busyScope === scope,
          webFeedback: visible?.web,
          helperStatus: visible?.helper ?? "ready",
          prepared: visible?.prepared,
        },
      }}
    >
      {children}
    </BrowserRuntimeContext.Provider>
  );
}
