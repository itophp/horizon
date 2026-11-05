import { useWorkspaceInstance } from "./workspace";
import { useMemo, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { publishPageReference } from "./semantic-store";
import { useNavigate, useLocation } from "react-router";
import { usePageTools } from "@/features/webmcp/provider";
import { pageTool } from "@/lib/webmcp/runtime";
import { runtimeExecute } from "@/lib/capabilities";
import { useRuntime, actionVisible } from "./v2-registry";
import { routeTo } from "./action";
import { mapInput } from "./expression";
export function RuntimeTools() {
  const ctx = useRuntime();
  const workspace = useWorkspaceInstance();
  const active = workspace?.active ?? true;
  const { t } = useTranslation("runtime");
  const navigate = useNavigate();
  const location = useLocation();
  const identity = JSON.stringify([
    active,
    ctx.identity,
    ctx.page.id,
    ctx.application.metadataVersion,
    ctx.application.installationRevision,
    ctx.application.applicationRevision,
    ctx.scope.record &&
    typeof ctx.scope.record === "object" &&
    !Array.isArray(ctx.scope.record)
      ? [
          ctx.scope.record.id,
          ctx.scope.record.status,
          ctx.scope.record.revision,
          ctx.scope.record.ownerId,
        ]
      : null,
    ctx.selection,
    ctx.readOnly,
  ]);
  const lease = useMemo(() => new ToolLease(identity), [identity]);
  useEffect(() => {
    lease.activate();
    return () => lease.deactivate();
  }, [lease]);
  useEffect(
    () =>
      active
        ? publishPageReference(identity, {
            applicationId: ctx.application.id,
            pageId: ctx.page.id,
            metadataVersion: ctx.application.metadataVersion,
            route: location.pathname,
            entity: ctx.page.entityRef,
            entityId:
              ctx.scope.record &&
              typeof ctx.scope.record === "object" &&
              !Array.isArray(ctx.scope.record)
                ? Number(ctx.scope.record.id)
                : undefined,
            selection: ctx.selection.map(Number).slice(0, 20),
          })
        : undefined,
    [active, identity, ctx, location.pathname],
  );
  const tools = useMemo(() => {
    if (ctx.readOnly || !active) return [];
    const base = {
      applicationId: ctx.application.id,
      pageId: ctx.page.id,
      metadataVersion: ctx.application.metadataVersion,
      ...(ctx.scope.record &&
      typeof ctx.scope.record === "object" &&
      !Array.isArray(ctx.scope.record)
        ? { recordId: ctx.scope.record.id }
        : {}),
      selection: ctx.selection,
    };
    const check = () => {
      lease.check();
    };
    const prefix = `rt_${ctx.page.id.replace(/[^a-zA-Z0-9]/g, "_").slice(0, 10)}_${ctx.application.metadataVersion.slice(0, 6)}_${lease.token}`;
    return [
      pageTool(
        `${prefix}_context`,
        t("contextTool"),
        z.object({}),
        async (_input, invocation) => {
          check();
          const result = await runtimeExecute(
            "application.runtime.context.resolve",
            "1.0.0",
            { ...base, ...(invocation ? { runId: invocation.runId } : {}) },
            "",
            invocation?.correlationId ?? ctx.correlationId,
          );
          if (result.status !== "succeeded")
            throw new Error(result.errorCode || "unavailable");
          return result.data;
        },
        { scope: "page", effect: "read" },
      ),
      ...ctx.page.actions
        .filter(
          (action) => action.webMcp && actionVisible(ctx, action, ctx.scope),
        )
        .map((action, index) =>
          pageTool(
            `${prefix}_a${index}`,
            t(action.labelKey),
            z.object({}),
            async (_input, invocation) => {
              check();
              if (action.type === "navigation") {
                const target = ctx.application.pages.find(
                  (page) => page.id === action.target,
                );
                if (!target) throw new Error("not_found");
                navigate(
                  routeTo(target.route, mapInput(action.input, ctx.scope)),
                );
                return { requested: true };
              }
              const result = await runtimeExecute(
                "application.runtime.action.invoke",
                "1.0.0",
                {
                  ...base,
                  actionId: action.id,
                  ...(invocation ? { runId: invocation.runId } : {}),
                },
                lease.key(
                  invocation
                    ? `${invocation.runId}:${invocation.callId}:${action.id}`
                    : action.id,
                ),
                invocation?.correlationId ?? ctx.correlationId,
              );
              return result.data;
            },
            {
              scope: "page",
              effect: action.type === "navigation" ? "navigation" : "write",
              permission: action.permission || null,
            },
          ),
        ),
    ];
  }, [ctx, t, navigate, lease, active]);
  usePageTools(tools);
  return null;
}

class ToolLease {
  readonly token = crypto.randomUUID().slice(0, 8);
  private active = false;
  private keys = new Map<string, string>();
  key(attempt: string) {
    this.check();
    let key = this.keys.get(attempt);
    if (!key) {
      key = crypto.randomUUID();
      this.keys.set(attempt, key);
    }
    return key;
  }
  constructor(readonly identity: string) {}
  activate() {
    this.active = true;
  }
  deactivate() {
    this.active = false;
  }
  check() {
    if (!this.active) throw new Error("conflict");
  }
}
