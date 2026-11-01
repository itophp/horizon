import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Play, RefreshCw, Square } from "lucide-react";
import {
  aiStatus,
  capabilityApprovals,
  capabilityDecide,
  capabilityInvocations,
} from "@/api/generated/baseline";
import type {
  CapabilityContract,
  CapabilityResult,
} from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import { invokeCapability } from "@/lib/capabilities";
import { useSession } from "@/features/administration/auth";
import { ErrorBanner } from "@/features/administration/shared";
import { ApprovalCard, RunStatusBadge } from "@/features/ai/shared";
import { useCapabilityKey } from "./shared";

type Selection = {
  runId?: number;
  traceId: string;
  flow: string;
  pendingIds: string[];
};
const active = (status: string) =>
  status === "running" || status === "awaiting_approval";
function object(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
function runId(result: CapabilityResult): number | undefined {
  const value = object(result.data).runId;
  return typeof value === "number" && Number.isSafeInteger(value) && value > 0
    ? value
    : undefined;
}

// Remount local action state when the trusted account/department changes.
export function WorkflowExamplePanel({
  contracts,
}: {
  contracts: CapabilityContract[];
}) {
  const session = useSession();
  return (
    <ScopedWorkflowExample
      key={`${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`}
      contracts={contracts}
    />
  );
}
function ScopedWorkflowExample({
  contracts,
}: {
  contracts: CapabilityContract[];
}) {
  const { t } = useTranslation("capability");
  const session = useSession();
  const client = useQueryClient();
  const [selection, setSelection] = useState<Selection>();
  const scope = useCapabilityKey();
  const available = contracts.some(
    (item) => item.name === "orchestration.workflow.run",
  );
  const canStart = [
    "orchestration:run",
    "orchestration:read",
    "example:create",
    "example:read",
    "example:archive",
  ].every((p) => session.permissions.includes(p));
  const canAgent = session.permissions.includes("assistant:use");
  const model = useQuery({
    queryKey: [...scope, "workflow-model"],
    queryFn: () => aiStatus(),
    enabled: available && canAgent,
    retry: false,
  });
  const modelReady =
    model.data?.status === 200 &&
    model.data.data.enabled &&
    model.data.data.configured;
  const run = useQuery({
    queryKey: [...scope, "workflow-example", selection?.runId],
    queryFn: () =>
      invokeCapability("orchestration.workflow.get", {
        runId: selection?.runId,
      }),
    enabled: selection?.runId !== undefined,
    retry: false,
    refetchInterval: (query) =>
      active(String(object(query.state.data?.data).status ?? "running")) &&
      !query.state.error
        ? 2000
        : false,
  });
  const status =
    selection?.runId === undefined
      ? selection
        ? "awaiting_approval"
        : ""
      : String(object(run.data?.data).status ?? "running");
  const running = !!selection && active(status);
  const invocations = useQuery({
    queryKey: [...scope, "workflow-example-trace", selection?.traceId],
    queryFn: () => capabilityInvocations({ limit: 100 }),
    enabled: !!selection,
    refetchInterval: running ? 2000 : false,
    retry: false,
  });
  const approvals = useQuery({
    queryKey: [...scope, "workflow-example-approvals", selection?.traceId],
    queryFn: () => capabilityApprovals(),
    enabled: !!selection && running,
    refetchInterval: running ? 2000 : false,
    retry: false,
  });
  const trace =
    invocations.data?.status === 200
      ? invocations.data.data.filter(
          (item) => item.correlationId === selection?.traceId,
        )
      : [];
  const ownIds = new Set([
    ...trace.map((item) => item.invocationId),
    ...(selection?.pendingIds ?? []),
  ]);
  const cards =
    approvals.data?.status === 200
      ? approvals.data.data.filter(
          (card) => ownIds.has(card.invocationId) && card.status === "pending",
        )
      : [];
  const refresh = () => client.invalidateQueries({ queryKey: scope });
  function remember(result: CapabilityResult, flow?: string) {
    setSelection((previous) => ({
      runId: runId(result) ?? (flow ? undefined : previous?.runId),
      traceId: flow ? result.traceId : (previous?.traceId ?? result.traceId),
      flow: flow ?? previous?.flow ?? "",
      pendingIds: [
        ...(flow ? [] : (previous?.pendingIds ?? [])),
        ...(result.status === "awaiting_approval" ? [result.invocationId] : []),
      ],
    }));
  }
  const start = useMutation({
    mutationFn: async ({
      flow,
      key,
      title,
    }: {
      flow: string;
      key: string;
      title: string;
    }) => {
      const result = await invokeCapability(
        "orchestration.workflow.run",
        { flow, version: "1.0.0", input: { title } },
        key,
      );
      if (result.status === "succeeded" && !runId(result))
        throw new ApiError("invalid", 400);
      return result;
    },
    onSuccess: async (result, variables) => {
      remember(result, variables.flow);
      await refresh();
    },
    retry: false,
  });
  const decide = useMutation({
    mutationFn: async ({ id, approve }: { id: number; approve: boolean }) => {
      const result = await capabilityDecide(id, { approve });
      if (result.status !== 200)
        throw new ApiError("unavailable", result.status);
      if (
        result.data.status !== "succeeded" &&
        result.data.status !== "cancelled"
      )
        throw new ApiError(result.data.errorCode || "invalid", 400);
      return result.data;
    },
    onSuccess: async (result) => {
      if (runId(result)) remember(result);
      if (!selection?.runId && result.status === "cancelled")
        setSelection(undefined);
      await refresh();
    },
    retry: false,
  });
  const cancel = useMutation({
    mutationFn: () =>
      invokeCapability(
        "orchestration.workflow.cancel",
        { runId: selection?.runId },
        `workflow-demo:cancel:${selection?.runId}`,
      ),
    onSuccess: async (result) => {
      if (result.status === "awaiting_approval") remember(result);
      await refresh();
    },
    retry: false,
  });
  function begin(flow: string) {
    decide.reset();
    cancel.reset();
    start.mutate({
      flow,
      key: crypto.randomUUID(),
      title: t("demo.recordTitle"),
    });
  }
  const error =
    start.error ??
    decide.error ??
    cancel.error ??
    run.error ??
    invocations.error ??
    approvals.error;
  const errorCode = object(run.data?.data).errorCode;
  const stages =
    selection?.flow === "example-lifecycle"
      ? [
          { key: "create", capability: "example.record.create" },
          {
            key: "ready",
            capability: "example.record.get",
            origin: "workflow",
          },
          { key: "review", capability: "example.record.get", origin: "agent" },
          { key: "archive", capability: "example.record.archive" },
        ]
      : [
          { key: "create", capability: "example.record.create" },
          { key: "archive", capability: "example.record.archive" },
        ];
  return (
    <section
      className="admin-panel entity-card workflow-example"
      aria-label={t("demo.title")}
    >
      <header className="ai-chat-heading">
        <div>
          <strong>{t("demo.title")}</strong>
          <small>{t("demo.hint")}</small>
        </div>
        <button
          type="button"
          className="ai-button is-ghost"
          aria-label={t("demo.refresh")}
          onClick={() => void refresh()}
        >
          <RefreshCw size={14} aria-hidden="true" />
        </button>
      </header>
      <div className="workflow-example-body">
        {!available ? (
          <p className="gov-empty">{t("demo.unavailable")}</p>
        ) : !canStart ? (
          <p className="gov-empty">{t("demo.forbidden")}</p>
        ) : (
          <>
            <div className="workflow-example-actions">
              <button
                type="button"
                className="ai-button"
                disabled={start.isPending || running}
                onClick={() => begin("example-create-archive")}
              >
                <Play size={14} aria-hidden="true" />
                {t("demo.basic")}
              </button>
              <button
                type="button"
                className="ai-button is-ghost"
                disabled={
                  start.isPending || running || !canAgent || !modelReady
                }
                onClick={() => begin("example-lifecycle")}
              >
                <Play size={14} aria-hidden="true" />
                {t("demo.full")}
              </button>
              {selection?.runId &&
                running &&
                session.permissions.includes("orchestration:cancel") && (
                  <button
                    type="button"
                    className="ai-button is-ghost"
                    disabled={cancel.isPending}
                    onClick={() => cancel.mutate()}
                  >
                    <Square size={14} aria-hidden="true" />
                    {t("demo.cancel")}
                  </button>
                )}
            </div>
            <p className="workflow-example-hint">
              {canAgent && modelReady
                ? t("demo.fullReady")
                : t("demo.modelRequired")}
            </p>
          </>
        )}
        {start.isPending && (
          <p role="status">
            <Loader2 className="animate-spin" size={14} aria-hidden="true" />{" "}
            {t("demo.starting")}
          </p>
        )}
        {error && (
          <ErrorBanner
            error={error}
            onRetry={() => {
              if (start.error && start.variables) start.mutate(start.variables);
              else void refresh();
            }}
          />
        )}
        {selection && (
          <div className="workflow-example-run">
            <div className="workflow-example-actions" role="status">
              <strong>
                {selection.runId
                  ? t("demo.run", { id: selection.runId })
                  : t("demo.pendingStart")}
              </strong>
              <RunStatusBadge status={status} />
            </div>
            <ol className="workflow-example-stages">
              {stages.map((stage) => {
                const call = trace.find(
                  (item) =>
                    item.capability === stage.capability &&
                    (!("origin" in stage) || item.origin === stage.origin),
                );
                return (
                  <li key={stage.key}>
                    <span>{t(`demo.stages.${stage.key}`)}</span>
                    <span className="ai-badge">
                      {call
                        ? t(`invocations.status.${call.status}`)
                        : t("demo.waiting")}
                    </span>
                  </li>
                );
              })}
            </ol>
            {status === "awaiting_approval" && <p>{t("demo.approvalHint")}</p>}
            {status === "completed" && (
              <p className="workflow-example-success">{t("demo.completed")}</p>
            )}
            {status === "failed" && (
              <p role="alert">
                {t("demo.failed", {
                  code: typeof errorCode === "string" ? errorCode : "unknown",
                })}
              </p>
            )}
            {status === "cancelled" && <p>{t("demo.cancelled")}</p>}
            {cards.map((card) => (
              <ApprovalCard
                key={card.id}
                approval={card}
                busy={decide.isPending}
                onDecide={(approve) => {
                  decide.mutate({ id: card.id, approve });
                  return Promise.resolve();
                }}
              />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
