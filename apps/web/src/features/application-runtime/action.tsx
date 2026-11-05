import { TaskPanel } from "./task-panel";
import { useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type {
  RuntimeAction,
  RuntimeCapabilityRef,
} from "@/api/generated/runtime";
import { runtimeExecute } from "@/lib/capabilities";
import { ApiError } from "@/api/client/request";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/features/administration/shared";
import { mapInput, validateInput, type Scope } from "./expression";
import {
  extensions,
  actionVisible,
  useRuntime,
  type RecordData,
} from "./v2-registry";
export class RuntimeCallError extends ApiError {
  constructor(
    code: string,
    public readonly traceId: string,
    public readonly invocationId: string,
  ) {
    super(code, 400);
  }
}
export function errorReference(error: unknown, fallback: string) {
  return error instanceof RuntimeCallError
    ? error.traceId || error.invocationId || fallback
    : fallback;
}
export type CallSpec = {
  id: string;
  type: string;
  capability: RuntimeCapabilityRef;
  inputSchema: string;
  eventSchema?: string;
  target?: string;
  input: RuntimeAction["input"];
  tags: string[];
};
export function useDispatcher() {
  const ctx = useRuntime();
  const client = useQueryClient();
  const attempts = useRef(
    new Map<string, { digest: string; key: string; operationId: string }>(),
  );
  const invalidate = async (tags: string[]) => {
    const providers = ctx.application.providers.filter((p) =>
      p.tags.some((tag) => tags.includes(tag)),
    );
    await client.invalidateQueries({
      predicate: (q) =>
        q.queryKey[0] === "runtime-v2" &&
        q.queryKey[1] === ctx.identity &&
        q.queryKey[2] === ctx.application.id &&
        providers.some((p) => q.queryKey[4] === p.id),
    });
  };
  const call = async (spec: CallSpec, scope: Scope) => {
    const context =
      typeof scope.context === "object" &&
      scope.context &&
      !Array.isArray(scope.context)
        ? scope.context
        : {};
    const preliminary = JSON.stringify([
      spec.capability,
      ctx.application.metadataVersion,
      ctx.application.installationRevision,
      scope.record,
      scope.form,
      scope.selection,
    ]);
    let attempt = attempts.current.get(spec.id);
    if (!attempt || attempt.digest !== preliminary) {
      attempt = {
        digest: preliminary,
        key: crypto.randomUUID(),
        operationId: crypto.randomUUID(),
      };
      attempts.current.set(spec.id, attempt);
    }
    if (spec.eventSchema) validateInput(scope.event, spec.eventSchema);
    let input = mapInput(spec.input, {
      ...scope,
      context: { ...context, operationId: attempt.operationId },
    });
    if (spec.type === "custom") {
      const adapter = extensions.resolve(
        spec.target ?? "",
        ctx.application,
      ).normalize;
      if (!adapter) throw new Error("action_not_found");
      input = adapter(input, scope);
    }
    validateInput(input, spec.inputSchema);
    let result;
    try {
      result = await runtimeExecute(
        spec.capability.name,
        spec.capability.version,
        input,
        attempt.key,
        ctx.correlationId,
        {
          applicationId: ctx.application.id,
          pageId: ctx.page.id,
          metadataVersion: ctx.application.metadataVersion,
          installationRevision: ctx.application.installationRevision ?? 1,
        },
      );
    } catch (error) {
      if (spec.type === "batch") attempt.key = crypto.randomUUID();
      throw error;
    }
    if (spec.type === "batch" && result.status === "failed")
      attempt.key = crypto.randomUUID();
    if (result.status !== "succeeded" && result.status !== "awaiting_approval")
      throw new RuntimeCallError(
        result.errorCode || "internal",
        result.traceId,
        result.invocationId,
      );
    if (result.status === "succeeded") await invalidate(spec.tags);
    return result;
  };
  return { call, invalidate };
}
export function routeTo(
  pageRoute: string,
  parameters: Record<string, unknown>,
) {
  return pageRoute.replace(
    /:([a-zA-Z][a-zA-Z0-9]*)/g,
    (_match, key: string) => {
      const value = parameters[key];
      if (typeof value !== "string" && typeof value !== "number")
        throw new Error("runtime_context_invalid");
      return encodeURIComponent(String(value));
    },
  );
}
export function ActionButtons({
  placement,
  record,
  onDialog,
}: {
  placement: string;
  record?: RecordData;
  onDialog: (action: RuntimeAction, scope: Scope) => Promise<void>;
}) {
  const ctx = useRuntime();
  const scope = record ? { ...ctx.scope, record } : ctx.scope;
  return (
    <>
      {ctx.page.actions
        .filter(
          (a) => a.placement === placement && actionVisible(ctx, a, scope),
        )
        .map((a) => (
          <ActionButton
            key={a.id}
            action={a}
            scope={scope}
            onDialog={onDialog}
          />
        ))}
    </>
  );
}
function ActionButton({
  action,
  scope,
  onDialog,
}: {
  action: RuntimeAction;
  scope: Scope;
  onDialog: (action: RuntimeAction, scope: Scope) => Promise<void>;
}) {
  const { t } = useTranslation("runtime");
  const navigate = useNavigate();
  const ctx = useRuntime();
  const dispatcher = useDispatcher();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const [runId, setRunId] = useState<number>();
  const [notice, setNotice] = useState("");
  const run = async () => {
    if (pending) return;
    if (action.confirmKey && !window.confirm(t(action.confirmKey))) return;
    setPending(true);
    setError(undefined);
    setNotice("");
    try {
      if (action.type === "dialog") await onDialog(action, scope);
      else if (action.type === "navigation") {
        const page = ctx.application.pages.find((p) => p.id === action.target);
        if (!page) throw new Error("page_not_found");
        navigate(routeTo(page.route, mapInput(action.input, scope)));
      } else {
        const result = await dispatcher.call(action, scope);
        if (result.status === "awaiting_approval")
          setNotice(t("approval", { id: result.approvalId }));
        else if (action.type === "workflow" || action.type === "agent") {
          const id = (result.data as { runId?: number } | undefined)?.runId;
          setRunId(id);
          setNotice(
            t("started", {
              id: (result.data as { runId?: number } | undefined)?.runId,
            }),
          );
        } else if (
          action.type === "batch" ||
          Array.isArray(
            (result.data as { items?: unknown[] } | undefined)?.items,
          )
        ) {
          const rows = (result.data as { items: { status: string }[] }).items;
          setNotice(
            t("batchResult", {
              succeeded: rows.filter((r) => r.status === "succeeded").length,
              failed: rows.filter((r) => r.status !== "succeeded").length,
            }),
          );
        } else setNotice(t(action.successKey || "saved"));
      }
    } catch (e) {
      setError(e);
    } finally {
      setPending(false);
    }
  };
  return (
    <span>
      {runId && <TaskPanel runId={runId} />}
      <Button
        type="button"
        disabled={
          pending ||
          (action.placement === "selection" && ctx.selection.length === 0)
        }
        onClick={() => {
          void run();
        }}
      >
        {t(action.labelKey)}
      </Button>
      {error !== undefined && (
        <>
          <ErrorBanner error={error} />
          <small>
            {t("reference", { id: errorReference(error, ctx.correlationId) })}
          </small>
        </>
      )}{" "}
      {notice && (
        <span role="status">
          {notice}
          {notice.startsWith(t("approvalPrefix")) && (
            <Link to="/capabilities">{t("viewApproval")}</Link>
          )}
        </span>
      )}
    </span>
  );
}
