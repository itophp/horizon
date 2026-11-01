import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Archive, Loader2, Plus, RefreshCw } from "lucide-react";
import {
  capabilityApprovals,
  capabilityDecide,
  capabilityDefinitions,
  capabilityEvents,
  capabilityInvocations,
  capabilityInvoke,
} from "@/api/generated/baseline";
import type {
  CapabilityApproval,
  CapabilityContract,
  CapabilityEvent,
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
import {
  capabilityDescription,
  capabilityLabel,
  formatMoment,
} from "@/lib/capability";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AiStatusBadge,
  ApprovalPolicy,
  InvocationStatus,
  RiskBadge,
  statusLabel,
  useCapabilityKey,
} from "./shared";
import "./capability.css";
import { WorkflowExamplePanel } from "./workflow-example";

interface RecordRow {
  id: number;
  title: string;
  status: string;
}

const RECORDS = "example.record.list";
const CREATE = "example.record.create";
const ARCHIVE = "example.record.archive";

function resultRecords(result: CapabilityResult | undefined): RecordRow[] {
  const data = result?.data as { records?: RecordRow[] } | undefined;
  return data?.records ?? [];
}

export function CapabilityPage() {
  const { t, i18n } = useTranslation(["capability", "ai"]);
  const session = useSession();
  const client = useQueryClient();
  const [title, setTitle] = useState("");
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const canRead = session.permissions.includes("example:read");
  const canCreate = session.permissions.includes("example:create");
  const canArchive = session.permissions.includes("example:archive");

  const definitionsKey = useCapabilityKey("definitions");
  const recordsKey = useCapabilityKey("records");
  const eventsKey = useCapabilityKey("events");
  const invocationsKey = useCapabilityKey("invocations");
  const approvalsKey = useCapabilityKey("approvals");

  const definitions = useQuery({
    queryKey: definitionsKey,
    queryFn: () => capabilityDefinitions(),
    retry: false,
  });
  const records = useQuery({
    queryKey: recordsKey,
    queryFn: () => capabilityInvoke(RECORDS, { input: { limit: 30 } }),
    enabled: canRead,
    retry: false,
    staleTime: 5000,
  });
  const events = useQuery({
    queryKey: eventsKey,
    queryFn: () => capabilityEvents({ limit: 20 }),
    retry: false,
    refetchInterval: 5000,
  });
  const invocations = useQuery({
    queryKey: invocationsKey,
    queryFn: () => capabilityInvocations({ limit: 20 }),
    retry: false,
  });
  const approvals = useQuery({
    queryKey: approvalsKey,
    queryFn: () => capabilityApprovals(),
    retry: false,
    refetchInterval: 5000,
  });

  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: recordsKey }),
      client.invalidateQueries({ queryKey: eventsKey }),
      client.invalidateQueries({ queryKey: invocationsKey }),
      client.invalidateQueries({ queryKey: approvalsKey }),
    ]);
  };

  function failed(result: CapabilityResult) {
    return (
      result.status !== "succeeded" && result.status !== "awaiting_approval"
    );
  }

  const create = useMutation({
    mutationFn: async () => {
      const result = await capabilityInvoke(CREATE, {
        input: { title: title.trim() },
        idempotencyKey: crypto.randomUUID(),
      });
      if (result.status !== 200)
        throw new ApiError("unavailable", result.status);
      if (failed(result.data))
        throw new ApiError(result.data.errorCode || "invalid", 200);
      return result.data;
    },
    onSuccess: async (result) => {
      const created = result.data as RecordRow | undefined;
      setTitle("");
      setNotice(t("records.created", { id: created?.id ?? "?" }));
      setError(null);
      await refresh();
    },
    onError: setError,
  });

  const archive = useMutation({
    mutationFn: async (id: number) => {
      const result = await capabilityInvoke(ARCHIVE, {
        input: { id },
        idempotencyKey: `archive:${id}`,
      });
      if (result.status !== 200)
        throw new ApiError("unavailable", result.status);
      if (failed(result.data))
        throw new ApiError(result.data.errorCode || "invalid", 200);
      return result.data;
    },
    onSuccess: async () => {
      setNotice(null);
      setError(null);
      await refresh();
    },
    onError: setError,
  });

  const decide = useMutation({
    mutationFn: async ({ id, approve }: { id: number; approve: boolean }) => {
      const result = await capabilityDecide(id, { approve });
      if (result.status !== 200)
        throw new ApiError("unavailable", result.status);
      if (failed(result.data))
        throw new ApiError(result.data.errorCode || "invalid", 200);
      return result.data;
    },
    onSuccess: refresh,
    onError: setError,
  });

  const contracts =
    definitions.data?.status === 200 ? definitions.data.data : [];
  const recordRows =
    records.data?.status === 200 ? resultRecords(records.data.data) : [];
  const eventRows = events.data?.status === 200 ? events.data.data : [];
  const invocationRows =
    invocations.data?.status === 200 ? invocations.data.data : [];
  const approvalRows =
    approvals.data?.status === 200 ? approvals.data.data : [];

  if (definitions.isPending) return <Loading />;
  if (definitions.error) return <ErrorBanner error={definitions.error} />;

  return (
    <>
      <PageHeading title={t("title")} subtitle={t("subtitle")}>
        <span className="ai-badge">
          {t("summary.definitions", { total: contracts.length })}
        </span>
        <span className="ai-badge">
          {t("summary.events", { total: eventRows.length })}
        </span>
        {!!approvalRows.length && (
          <span className="ai-badge ai-status-awaiting_approval">
            {t("summary.pending", { total: approvalRows.length })}
          </span>
        )}
      </PageHeading>

      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice(null)} />
      )}
      {!!error && <ErrorBanner error={error} onRetry={() => void refresh()} />}

      <WorkflowExamplePanel contracts={contracts} />

      <div className="ai-workbench capability-workbench">
        <section className="admin-panel entity-card capability-records">
          <header className="ai-chat-heading">
            <div>
              <strong>{t("records.title")}</strong>
              <small>{t("records.hint")}</small>
            </div>
            <button
              type="button"
              className="ai-button is-ghost"
              onClick={() => void refresh()}
              aria-label={t("invocations.title")}
            >
              <RefreshCw size={14} aria-hidden="true" />
            </button>
          </header>

          {canCreate && (
            <form
              className="capability-create"
              onSubmit={(event) => {
                event.preventDefault();
                if (title.trim()) create.mutate();
              }}
            >
              <label htmlFor="record-title">{t("records.titleLabel")}</label>
              <div>
                <input
                  id="record-title"
                  value={title}
                  maxLength={200}
                  placeholder={t("records.titlePlaceholder")}
                  onChange={(event) => setTitle(event.target.value)}
                />
                <button
                  type="submit"
                  className="ai-button"
                  disabled={create.isPending || !title.trim()}
                >
                  {create.isPending ? (
                    <Loader2
                      size={14}
                      className="animate-spin"
                      aria-hidden="true"
                    />
                  ) : (
                    <Plus size={14} aria-hidden="true" />
                  )}
                  {create.isPending
                    ? t("records.creating")
                    : t("records.create")}
                </button>
              </div>
            </form>
          )}

          {!canRead ? (
            <p className="gov-empty">{t("forbidden")}</p>
          ) : records.isPending ? (
            <Loading />
          ) : records.error ? (
            <ErrorBanner error={records.error} />
          ) : !recordRows.length ? (
            <p className="gov-empty">{t("records.empty")}</p>
          ) : (
            <div className="admin-table-scroll">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("records.columns.id")}</TableHead>
                    <TableHead>{t("records.columns.title")}</TableHead>
                    <TableHead>{t("records.columns.status")}</TableHead>
                    <TableHead>{t("records.columns.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {recordRows.map((record) => (
                    <TableRow key={record.id}>
                      <TableCell>{record.id}</TableCell>
                      <TableCell>{record.title}</TableCell>
                      <TableCell>
                        <span className={`ai-badge record-${record.status}`}>
                          {statusLabel(t, record.status)}
                        </span>
                      </TableCell>
                      <TableCell>
                        {canArchive && record.status !== "archived" && (
                          <button
                            type="button"
                            className="ai-button is-ghost"
                            disabled={archive.isPending}
                            onClick={() => archive.mutate(record.id)}
                          >
                            {archive.isPending &&
                            archive.variables === record.id ? (
                              <Loader2
                                size={13}
                                className="animate-spin"
                                aria-hidden="true"
                              />
                            ) : (
                              <Archive size={13} aria-hidden="true" />
                            )}
                            {archive.isPending &&
                            archive.variables === record.id
                              ? t("records.archiving")
                              : t("records.archive")}
                          </button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </section>

        <aside className="admin-panel entity-card ai-side">
          <h3>{t("approvals.title")}</h3>
          {!approvalRows.length ? (
            <p className="gov-empty">{t("approvals.empty")}</p>
          ) : (
            <ul className="ai-pending">
              {approvalRows.map((card) => (
                <ApprovalRow
                  key={card.id}
                  card={card}
                  busy={decide.isPending}
                  onDecide={(approve) =>
                    decide.mutate({ id: card.id, approve })
                  }
                />
              ))}
            </ul>
          )}
          <h3>{t("events.title")}</h3>
          {!eventRows.length ? (
            <p className="gov-empty">{t("events.empty")}</p>
          ) : (
            <ul className="capability-events">
              {eventRows.map((event) => (
                <EventRow
                  key={event.eventId}
                  event={event}
                  language={i18n.language}
                />
              ))}
            </ul>
          )}
        </aside>
      </div>

      <section className="admin-panel entity-card capability-table-card">
        <header className="ai-chat-heading">
          <div>
            <strong>{t("invocations.title")}</strong>
          </div>
        </header>
        {!invocationRows.length ? (
          <p className="gov-empty">{t("invocations.empty")}</p>
        ) : (
          <div className="admin-table-scroll">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("invocations.columns.capability")}</TableHead>
                  <TableHead>{t("invocations.columns.origin")}</TableHead>
                  <TableHead>{t("invocations.columns.status")}</TableHead>
                  <TableHead>{t("invocations.columns.duration")}</TableHead>
                  <TableHead>{t("invocations.columns.time")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {invocationRows.map((item) => (
                  <InvocationRow
                    key={item.invocationId}
                    item={item}
                    language={i18n.language}
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </section>

      <Catalog contracts={contracts} />
    </>
  );
}

function ApprovalRow({
  card,
  busy,
  onDecide,
}: {
  card: CapabilityApproval;
  busy: boolean;
  onDecide: (approve: boolean) => void;
}) {
  const { t, i18n } = useTranslation("capability");
  return (
    <li className="capability-approval">
      <div>
        <strong>{capabilityLabel(t, card.capability)}</strong>
        <small>
          {t("approvals.expires", {
            time: formatMoment(i18n.language, card.expiresAt),
          })}
        </small>
      </div>
      <div className="capability-approval-actions">
        <button
          type="button"
          className="ai-button"
          disabled={busy}
          onClick={() => onDecide(true)}
        >
          {t("approvals.approve")}
        </button>
        <button
          type="button"
          className="ai-button is-ghost"
          disabled={busy}
          onClick={() => onDecide(false)}
        >
          {t("approvals.reject")}
        </button>
      </div>
    </li>
  );
}

function EventRow({
  event,
  language,
}: {
  event: CapabilityEvent;
  language: string;
}) {
  const { t } = useTranslation("capability");
  return (
    <li className="capability-event">
      <div>
        <strong>{event.type}</strong>
        <small>
          {t("events.aggregate", {
            type: event.aggregateType,
            id: event.aggregateId,
          })}
        </small>
        <small>
          {t("events.meta", {
            time: formatMoment(language, event.createdAt),
            depth: t("events.depth", { depth: event.depth }),
            policy: event.aiPolicy
              ? t("events.policySuffix", { policy: event.aiPolicy })
              : "",
          })}
        </small>
      </div>
      <AiStatusBadge status={event.aiStatus} />
    </li>
  );
}

function InvocationRow({
  item,
  language,
}: {
  item: CapabilityInvocation;
  language: string;
}) {
  const { t } = useTranslation("capability");
  return (
    <TableRow>
      <TableCell>
        <div className="ai-cell-stack">
          <strong>{capabilityLabel(t, item.capability)}</strong>
          <small>{`${item.capability}@${item.version}`}</small>
        </div>
      </TableCell>
      <TableCell>
        {t(`invocations.origin.${item.origin}`, { defaultValue: item.origin })}
      </TableCell>
      <TableCell>
        <div className="ai-cell-stack">
          <InvocationStatus status={item.status} />
          {item.errorCode && <small>{item.errorCode}</small>}
        </div>
      </TableCell>
      <TableCell>{`${item.durationMs}ms`}</TableCell>
      <TableCell>{formatMoment(language, item.createdAt)}</TableCell>
    </TableRow>
  );
}

function Catalog({ contracts }: { contracts: CapabilityContract[] }) {
  const { t } = useTranslation(["capability", "entity"]);
  const [search, setSearch] = useState("");
  const [risk, setRisk] = useState("all");
  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return contracts.filter((item) => {
      if (risk !== "all" && item.risk !== risk) return false;
      if (!needle) return true;
      return `${item.name} ${item.domain} ${item.permission}`
        .toLowerCase()
        .includes(needle);
    });
  }, [contracts, risk, search]);

  return (
    <section className="admin-panel entity-card capability-catalog">
      <header className="ai-toolbar">
        <label className="ai-search">
          <span className="sr-only">{t("catalog.search")}</span>
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("catalog.search")}
          />
        </label>
        <div className="ai-filter" role="group" aria-label={t("catalog.title")}>
          {["all", "low", "medium", "high", "critical"].map((option) => (
            <button
              key={option}
              type="button"
              aria-pressed={risk === option}
              onClick={() => setRisk(option)}
            >
              {option === "all" ? t("catalog.title") : t(`risk.${option}`)}
            </button>
          ))}
        </div>
        <span className="ai-count">
          {t("catalog.count", { total: rows.length })}
        </span>
      </header>
      {!rows.length ? (
        <p className="gov-empty">{t("catalog.empty")}</p>
      ) : (
        <div className="admin-table-scroll">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("catalog.columns.capability")}</TableHead>
                <TableHead>{t("catalog.columns.domain")}</TableHead>
                <TableHead>{t("catalog.columns.risk")}</TableHead>
                <TableHead>{t("catalog.columns.approval")}</TableHead>
                <TableHead>{t("catalog.columns.permission")}</TableHead>
                <TableHead>{t("catalog.columns.events")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((item) => (
                <TableRow key={`${item.name}@${item.version}`}>
                  <TableCell>
                    <div className="ai-cell-stack">
                      <strong>{capabilityLabel(t, item.name)}</strong>
                      <small>
                        {capabilityDescription(t, item.name, item.description)}
                      </small>
                      <code>{`${item.name}@${item.version}`}</code>
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="ai-cell-stack">
                      <span>{item.domain}</span>
                      <small>
                        {t(`${item.entity}.label`, {
                          ns: "entity",
                          defaultValue: item.entity,
                        })}
                      </small>
                    </div>
                  </TableCell>
                  <TableCell>
                    <RiskBadge risk={item.risk} />
                  </TableCell>
                  <TableCell>
                    <ApprovalPolicy policy={item.approval} />
                  </TableCell>
                  <TableCell>
                    <code>{item.permission || "—"}</code>
                  </TableCell>
                  <TableCell>
                    <div className="ai-cell-stack">
                      {item.successEvents.map((event) => (
                        <small key={event}>{event}</small>
                      ))}
                      {!item.successEvents.length && (
                        <small>{t("catalog.none")}</small>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </section>
  );
}
