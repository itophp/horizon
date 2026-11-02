import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Play, RefreshCw, Square } from "lucide-react";
import {
  capabilityApprovals,
  capabilityDecide,
  capabilityInvocations,
} from "@/api/generated/baseline";
import type {
  CapabilityApproval,
  CapabilityInvocation,
  CapabilityResult,
} from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import {
  ErrorBanner,
  Loading,
  PageHeading,
  SuccessNotice,
} from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
import { ApprovalCard, RunStatusBadge } from "@/features/ai/shared";
import { invokeCapability } from "@/lib/capabilities";
import { useCapabilityKey } from "@/features/capabilities/shared";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { WorkflowViewer } from "./viewer";
import type { Diagram } from "./graph";
import "./orchestration.css";

type WorkflowSummary = {
  name: string;
  version: string;
  hash: string;
  inputSchema?: unknown;
};

const LIST = "orchestration.workflow.list";
const DIAGRAM = "orchestration.workflow.diagram";
const RUN = "orchestration.workflow.run";
const GET = "orchestration.workflow.get";
const CANCEL = "orchestration.workflow.cancel";

function record(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
function textOf(value: unknown): string {
  return typeof value === "string" ? value : "";
}
function numberOf(value: unknown): number | undefined {
  return typeof value === "number" && Number.isSafeInteger(value) && value > 0
    ? value
    : undefined;
}
function workflowsOf(result: CapabilityResult | undefined): WorkflowSummary[] {
  const items = record(result?.data).workflows;
  if (!Array.isArray(items)) return [];
  const out: WorkflowSummary[] = [];
  for (const item of items) {
    const entry = record(item);
    const name = textOf(entry.name);
    const version = textOf(entry.version);
    if (!name || !version) continue;
    out.push({
      name,
      version,
      hash: textOf(entry.hash),
      inputSchema: entry.inputSchema,
    });
  }
  return out;
}
function diagramOf(result: CapabilityResult | undefined): Diagram | undefined {
  const data = record(result?.data);
  if (!Array.isArray(data.nodes) || !Array.isArray(data.edges))
    return undefined;
  return data as unknown as Diagram;
}
const active = (status: string) =>
  status === "running" || status === "awaiting_approval";

// A sample body is only a hint: the gateway still validates the real input
// against the capability schema, so a wrong guess fails closed.
function sampleInput(diagram: Diagram) {
  const properties = diagram.input?.properties ?? {};
  const out: Record<string, unknown> = {};
  for (const [key, raw] of Object.entries(properties)) {
    switch (textOf(record(raw).type)) {
      case "integer":
      case "number":
        out[key] = 0;
        break;
      case "boolean":
        out[key] = false;
        break;
      case "object":
        out[key] = {};
        break;
      case "array":
        out[key] = [];
        break;
      default:
        out[key] = "";
    }
  }
  return JSON.stringify(out, null, 2);
}

export function OrchestrationPage() {
  const { t } = useTranslation("orchestration");
  const session = useSession();
  const client = useQueryClient();
  const scope = useCapabilityKey();
  const [picked, setPicked] = useState("");
  const [search, setSearch] = useState("");
  const canRead = session.permissions.includes("orchestration:read");

  const list = useQuery({
    queryKey: [...scope, "workflows"],
    queryFn: () => invokeCapability(LIST, {}),
    enabled: canRead,
    retry: false,
  });
  const workflows = workflowsOf(list.data);
  const selected: WorkflowSummary | undefined =
    workflows.find((item) => `${item.name}@${item.version}` === picked) ??
    workflows[0];

  const diagram = useQuery({
    queryKey: [...scope, "diagram", selected?.name, selected?.version],
    queryFn: () =>
      invokeCapability(DIAGRAM, {
        flow: selected?.name,
        version: selected?.version,
      }),
    enabled: canRead && !!selected,
    retry: false,
  });
  const graph = diagramOf(diagram.data);

  if (!canRead) return <ErrorBanner error={new ApiError("forbidden", 403)} />;
  if (list.isPending) return <Loading />;
  if (list.error)
    return (
      <ErrorBanner error={list.error} onRetry={() => void list.refetch()} />
    );

  return (
    <>
      <PageHeading title={t("title")} subtitle={t("subtitle")}>
        <span className="ai-badge">
          {t("summary.workflows", { total: workflows.length })}
        </span>
      </PageHeading>

      <div className="orchestration-layout">
        <section className="admin-panel orchestration-list">
          <header className="ai-chat-heading">
            <div>
              <strong>{t("list.title")}</strong>
              <small>{t("list.hint")}</small>
            </div>
            <button
              type="button"
              className="ai-button is-ghost"
              onClick={() => {
                void list.refetch();
                void client.invalidateQueries({ queryKey: scope });
              }}
              aria-label={t("list.refresh")}
            >
              <RefreshCw size={14} aria-hidden="true" />
            </button>
          </header>
          {!workflows.length && (
            <p className="orchestration-empty">{t("list.empty")}</p>
          )}
          <input
            className="orchestration-search"
            aria-label={t("list.search")}
            placeholder={t("list.search")}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          {!!workflows.length &&
            !workflows.some((item) =>
              `${item.name}@${item.version}`
                .toLowerCase()
                .includes(search.toLowerCase()),
            ) && <p className="orchestration-empty">{t("list.noMatches")}</p>}
          <ul className="orchestration-items">
            {workflows
              .filter((item) =>
                `${item.name}@${item.version}`
                  .toLowerCase()
                  .includes(search.toLowerCase()),
              )
              .map((item) => {
                const key = `${item.name}@${item.version}`;
                return (
                  <li key={key}>
                    <button
                      type="button"
                      aria-pressed={selected === item}
                      className={selected === item ? "is-active" : ""}
                      onClick={() => setPicked(key)}
                    >
                      <strong>{item.name}</strong>
                      <small>
                        {t("list.meta", {
                          version: item.version,
                          hash: item.hash.slice(0, 8),
                        })}
                      </small>
                    </button>
                  </li>
                );
              })}
          </ul>
        </section>

        {graph && selected ? (
          <WorkflowDetail
            key={`${selected.name}@${selected.version}`}
            diagram={graph}
            flow={selected.name}
            version={selected.version}
          />
        ) : (
          <section className="admin-panel">
            {diagram.error ? (
              <ErrorBanner error={diagram.error} />
            ) : (
              <p className="orchestration-empty">
                {diagram.isPending ? t("detail.loading") : t("detail.empty")}
              </p>
            )}
          </section>
        )}
      </div>
    </>
  );
}

// Remounting on the workflow key resets the run form without an effect, so the
// sample body can never leak from one definition into another.
function WorkflowDetail({
  diagram,
  flow,
  version,
}: {
  diagram: Diagram;
  flow: string;
  version: string;
}) {
  const { t, i18n } = useTranslation(["orchestration", "ai", "capability"]);
  const session = useSession();
  const client = useQueryClient();
  const scope = useCapabilityKey();
  const [input, setInput] = useState(() => sampleInput(diagram));
  const [inputError, setInputError] = useState("");
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [runId, setRunId] = useState<number>();
  const [traceId, setTraceId] = useState("");

  const canRun = session.permissions.includes("orchestration:run");
  const canCancel = session.permissions.includes("orchestration:cancel");

  const run = useQuery({
    queryKey: [...scope, "run", runId],
    queryFn: () => invokeCapability(GET, { runId }),
    enabled: runId !== undefined,
    retry: false,
    refetchInterval: (query) =>
      active(textOf(record(query.state.data?.data).status)) ? 2000 : false,
  });
  const status = runId
    ? textOf(record(run.data?.data).status) || "running"
    : "";
  const running = active(status);
  const errorCode = textOf(record(run.data?.data).errorCode);

  const invocations = useQuery({
    queryKey: [...scope, "trace", traceId],
    queryFn: () => capabilityInvocations({ limit: 100 }),
    enabled: !!traceId,
    refetchInterval: running ? 2000 : false,
    retry: false,
  });
  const trace: CapabilityInvocation[] = useMemo(
    () =>
      invocations.data?.status === 200
        ? invocations.data.data.filter((item) => item.correlationId === traceId)
        : [],
    [invocations.data, traceId],
  );

  const approvals = useQuery({
    queryKey: [...scope, "approvals", traceId],
    queryFn: () => capabilityApprovals(),
    enabled: !!traceId && running,
    refetchInterval: running ? 2000 : false,
    retry: false,
  });
  const invocationIds = new Set(trace.map((item) => item.invocationId));
  const cards: CapabilityApproval[] =
    approvals.data?.status === 200
      ? approvals.data.data.filter(
          (card) =>
            card.status === "pending" && invocationIds.has(card.invocationId),
        )
      : [];

  const refresh = () => client.invalidateQueries({ queryKey: scope });

  const start = useMutation({
    mutationFn: async (body: Record<string, unknown>) =>
      invokeCapability(
        RUN,
        { flow, version, input: body },
        crypto.randomUUID(),
      ),
    onSuccess: async (result) => {
      const id = numberOf(record(result.data).runId);
      setTraceId(result.traceId);
      setRunId(id);
      setNotice(t("notice.started", { id: id ?? "?" }));
      setError(null);
      await refresh();
    },
    onError: setError,
    retry: false,
  });

  const cancel = useMutation({
    mutationFn: () =>
      invokeCapability(CANCEL, { runId }, `orchestration:cancel:${runId}`),
    onSuccess: async (result) => {
      setNotice(
        result.status === "awaiting_approval"
          ? t("notice.cancelPending")
          : t("notice.cancelled", { id: runId ?? "?" }),
      );
      await refresh();
    },
    onError: setError,
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
    onSuccess: refresh,
    onError: setError,
    retry: false,
  });

  function submit() {
    let body: unknown;
    try {
      body = JSON.parse(input);
    } catch {
      setInputError(t("run.invalidJson"));
      return;
    }
    if (typeof body !== "object" || body === null || Array.isArray(body)) {
      setInputError(t("run.objectOnly"));
      return;
    }
    setInputError("");
    setNotice(null);
    start.mutate(body as Record<string, unknown>);
  }

  return (
    <div className="orchestration-detail">
      <section className="admin-panel">
        <header className="ai-chat-heading">
          <div>
            <strong>{flow}</strong>
            <small>
              {t("detail.hash", { hash: diagram.hash.slice(0, 16) })}
            </small>
          </div>
          <span className="ai-badge">
            {t("detail.nodes", { total: diagram.nodes.length })}
          </span>
          {status && <RunStatusBadge status={status} />}
        </header>
        <WorkflowViewer key={diagram.hash} diagram={diagram} />
        <p className="orchestration-empty">{t("editor.runtimeHint")}</p>
        <details className="orchestration-step-table">
          <summary>{t("viewer.stepTable")}</summary>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("columns.id")}</TableHead>
                <TableHead>{t("columns.type")}</TableHead>
                <TableHead>{t("columns.target")}</TableHead>
                <TableHead>{t("columns.depends")}</TableHead>
                <TableHead>{t("columns.extra")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {diagram.nodes
                .filter((node) => node.kind !== "start" && node.kind !== "end")
                .map((node) => (
                  <TableRow key={node.id}>
                    <TableCell>
                      <code>{node.id}</code>
                    </TableCell>
                    <TableCell>{t(`kind.${node.kind}`)}</TableCell>
                    <TableCell>
                      {node.capability ? (
                        <span>{node.capability}</span>
                      ) : node.tools?.length ? (
                        <span>{node.tools.join(", ")}</span>
                      ) : (
                        <span>{t("value.none")}</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {node.dependsOn?.length
                        ? node.dependsOn.join(", ")
                        : t("value.none")}
                    </TableCell>
                    <TableCell>
                      {node.maxAttempts
                        ? t("extra.retry", { total: node.maxAttempts })
                        : node.compensation
                          ? t("extra.compensation", {
                              capability: node.compensation,
                            })
                          : t("value.none")}
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </details>
      </section>

      <section className="admin-panel orchestration-run">
        <header className="ai-chat-heading">
          <div>
            <strong>{t("run.title")}</strong>
            <small>{t("run.hint")}</small>
          </div>
          {status && <RunStatusBadge status={status} />}
        </header>
        {notice && (
          <SuccessNotice message={notice} onClose={() => setNotice(null)} />
        )}
        {!!error && (
          <ErrorBanner error={error} onRetry={() => void refresh()} />
        )}
        {!canRun && <p className="orchestration-empty">{t("run.forbidden")}</p>}
        {canRun && (
          <>
            <label className="orchestration-field">
              <span>{t("run.input")}</span>
              <textarea
                value={input}
                spellCheck={false}
                rows={8}
                onChange={(event) => setInput(event.target.value)}
                aria-label={t("run.input")}
              />
            </label>
            {!!inputError && (
              <p className="orchestration-problem">{inputError}</p>
            )}
            {!!errorCode && (
              <p className="orchestration-problem">
                {t("run.failed", {
                  code: t(`ai:code.${errorCode}`, { defaultValue: errorCode }),
                })}
              </p>
            )}
            <div className="orchestration-actions">
              <button
                type="button"
                className="ai-button"
                disabled={start.isPending || running}
                onClick={submit}
              >
                {start.isPending ? (
                  <Loader2
                    size={14}
                    className="is-spinning"
                    aria-hidden="true"
                  />
                ) : (
                  <Play size={14} aria-hidden="true" />
                )}
                {running ? t("run.running") : t("run.start")}
              </button>
              {!!runId && canCancel && (
                <button
                  type="button"
                  className="ai-button is-ghost"
                  disabled={cancel.isPending}
                  onClick={() => cancel.mutate()}
                >
                  <Square size={13} aria-hidden="true" />
                  {t("run.cancel")}
                </button>
              )}
              {!!runId && (
                <span className="orchestration-run-id">
                  {t("run.id", { id: runId })}
                </span>
              )}
            </div>
          </>
        )}

        {cards.map((card) => (
          <ApprovalCard
            key={card.invocationId}
            approval={card}
            busy={decide.isPending}
            onDecide={async (approve) => {
              await decide.mutateAsync({ id: card.id, approve });
              await refresh();
            }}
          />
        ))}

        <div className="orchestration-trace">
          <strong>{t("trace.title")}</strong>
          {!trace.length && (
            <p className="orchestration-empty">{t("trace.empty")}</p>
          )}
          {!!trace.length && (
            <ul>
              {trace.map((item) => (
                <li key={item.invocationId}>
                  <span className={`ai-badge ai-status-${item.status}`}>
                    {t(`capability:invocations.status.${item.status}`, {
                      defaultValue: item.status,
                    })}
                  </span>
                  <span>{item.capability}</span>
                  <small>
                    {new Intl.DateTimeFormat(i18n.language, {
                      hour: "2-digit",
                      minute: "2-digit",
                      second: "2-digit",
                    }).format(new Date(item.createdAt))}
                  </small>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>
    </div>
  );
}
