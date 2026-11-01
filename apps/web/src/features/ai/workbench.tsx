import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { CornerDownLeft, Loader2, Sparkles } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Link, useLocation } from "react-router";
import {
  aiRun,
  aiRuns,
  aiStatus,
  capabilityApprovals,
  capabilityDecide,
} from "@/api/generated/baseline";
import type { AIRun, CapabilityApproval } from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import { ErrorBanner } from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
import { capabilityLabel, formatMoment } from "@/lib/capability";
import { useCapabilityKey } from "@/features/capabilities/shared";
import { ApprovalCard, RunStatusBadge, RunTrace, useAiKey } from "./shared";

interface Turn {
  id: string;
  role: "user" | "assistant";
  content: string;
  run?: AIRun;
}

function identifier() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`;
}

export function Workbench({
  sessionId,
  onTurnComplete,
}: {
  sessionId?: string | null;
  onTurnComplete?: () => void;
}) {
  const { t, i18n } = useTranslation("ai");
  const session = useSession();
  const location = useLocation();
  const client = useQueryClient();
  const [turns, setTurns] = useState<Turn[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [deciding, setDeciding] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [approval, setApproval] = useState<CapabilityApproval | null>(null);
  // A reopened task keeps its session id so the server can replay the thread;
  // a new task gets one on first use.
  const thread = useRef(sessionId ?? identifier());
  const controller = useRef<AbortController | null>(null);
  const statusKey = useAiKey("status");
  const approvalsKey = useAiKey("approvals");
  const runsKey = useAiKey("runs");
  const invocationsKey = useCapabilityKey("invocations");

  const status = useQuery({
    queryKey: statusKey,
    queryFn: () => aiStatus(),
    retry: false,
    staleTime: 60000,
  });
  const snapshot = status.data?.status === 200 ? status.data.data : undefined;
  // The agent only runs when the workspace enabled it and the model is usable.
  // Block only once the status is known: while it loads, typing stays possible.
  const blocked = Boolean(
    snapshot && !(snapshot.enabled && snapshot.configured),
  );
  const approvals = useQuery({
    queryKey: approvalsKey,
    queryFn: () => capabilityApprovals(),
    retry: false,
  });
  const pending =
    approvals.data?.status === 200
      ? approvals.data.data
      : ([] as CapabilityApproval[]);
  const historyKey = useAiKey(`runs-${sessionId ?? "new"}`);
  const history = useQuery({
    queryKey: historyKey,
    queryFn: () => aiRuns({ sessionId: sessionId ?? undefined, limit: 100 }),
    enabled: Boolean(sessionId),
    retry: false,
  });
  // A reopened task renders the stored turns plus anything asked in this view;
  // the stored copy wins for a run that is already shown locally, so a refetch
  // never doubles a turn.
  const restored = useMemo(() => {
    if (!sessionId || history.data?.status !== 200) return [] as Turn[];
    const out: Turn[] = [];
    for (const run of history.data.data) {
      out.push({ id: `q-${run.id}`, role: "user", content: run.question });
      out.push({
        id: `a-${run.id}`,
        role: "assistant",
        content: run.answer || t("workbench.emptyAnswer"),
        run,
      });
    }
    return out;
  }, [history.data, sessionId, t]);
  const messages = useMemo(() => {
    const sent = new Set(
      turns.map((turn) => turn.run?.id).filter((id): id is number => !!id),
    );
    const keep: Turn[] = [];
    for (let index = 0; index < restored.length; index += 1) {
      const turn = restored[index];
      if (turn.run && sent.has(turn.run.id)) {
        index += 1;
        continue;
      }
      keep.push(turn);
    }
    return [...keep, ...turns];
  }, [restored, turns]);

  async function send(text: string) {
    const question = text.trim();
    if (!question || busy || blocked) return;
    setError(null);
    setBusy(true);
    setDraft("");
    setTurns((current) => [
      ...current,
      { id: identifier(), role: "user", content: question },
    ]);
    const abort = new AbortController();
    controller.current = abort;
    try {
      const result = await aiRun(
        {
          question,
          sessionId: thread.current,
          page: { route: location.pathname },
        },
        { signal: abort.signal },
      );
      if (abort.signal.aborted) return;
      if (result.status !== 201) {
        throw new ApiError("unavailable", result.status);
      }
      const run = result.data;
      setTurns((current) => [
        ...current,
        {
          id: identifier(),
          role: "assistant",
          content: run.answer || t("workbench.emptyAnswer"),
          run,
        },
      ]);
      // The run only carries business invocations; a pending confirmation is a
      // gateway approval, so resolve it by invocation id.
      const cards = await capabilityApprovals();
      const card =
        cards.status === 200
          ? cards.data.find((item) =>
              run.invocations.some(
                (invocation) => invocation.invocationId === item.invocationId,
              ),
            )
          : undefined;
      setApproval(card ?? null);
      await client.invalidateQueries({ queryKey: runsKey });
      onTurnComplete?.();
      if (card) {
        await client.invalidateQueries({ queryKey: approvalsKey });
      }
    } catch (caught) {
      if (!abort.signal.aborted) setError(caught);
      // A refused turn is still a recorded task; show it in the run history.
      await client.invalidateQueries({ queryKey: runsKey });
      onTurnComplete?.();
    } finally {
      if (controller.current === abort) controller.current = null;
      setBusy(false);
    }
  }

  async function decide(approve: boolean, note: string) {
    if (!approval || deciding) return;
    setDeciding(true);
    setError(null);
    try {
      const result = await capabilityDecide(approval.id, { approve, note });
      if (result.status !== 200) {
        throw new ApiError("unavailable", result.status);
      }
      // A decision resumes the business invocation; reload the task trace and
      // the queues instead of guessing the new state on the client.
      setApproval(null);
      await client.invalidateQueries({ queryKey: approvalsKey });
      await client.invalidateQueries({ queryKey: runsKey });
      await client.invalidateQueries({ queryKey: invocationsKey });
    } catch (caught) {
      setError(caught);
    } finally {
      setDeciding(false);
    }
  }

  return (
    <div className="ai-workbench">
      <section className="admin-panel entity-card ai-chat">
        <header className="ai-chat-heading">
          <span className="ai-mark">
            <Sparkles size={17} aria-hidden="true" />
          </span>
          <div>
            <strong>{t("workbench.title")}</strong>
            <small>
              {snapshot
                ? t("workbench.runtime", {
                    model: snapshot.model || t("workbench.noModel"),
                    capabilities: snapshot.capabilityCount,
                  })
                : t("workbench.loadingRuntime")}
            </small>
          </div>
          {snapshot && !snapshot.configured && (
            <span className="ai-badge tone-3">
              {t("workbench.notConfigured")}
            </span>
          )}
        </header>

        <div className="ai-log" role="log" aria-live="polite">
          {!messages.length && (
            <div className="ai-empty">
              <p>{t("workbench.intro")}</p>
              <div className="ai-suggestions">
                {(["one", "two", "three"] as const).map((key) => (
                  <button
                    key={key}
                    type="button"
                    onClick={() => setDraft(t(`workbench.suggestion.${key}`))}
                  >
                    {t(`workbench.suggestion.${key}`)}
                  </button>
                ))}
              </div>
            </div>
          )}
          {messages.map((turn) =>
            turn.role === "user" ? (
              <div className="ai-turn is-user" key={turn.id}>
                <p>{turn.content}</p>
              </div>
            ) : (
              <div className="ai-turn is-assistant" key={turn.id}>
                <div className="ai-bubble">
                  <div className="ai-markdown">
                    <Markdown remarkPlugins={[remarkGfm]}>
                      {turn.content}
                    </Markdown>
                  </div>
                  {turn.run && (
                    <footer className="ai-run-heading">
                      <RunStatusBadge status={turn.run.status} />
                      <span>
                        {formatMoment(i18n.language, turn.run.createdAt)}
                      </span>
                    </footer>
                  )}
                  {turn.run && <RunTrace run={turn.run} />}
                </div>
              </div>
            ),
          )}
          {busy && (
            <div className="ai-turn is-assistant">
              <div className="ai-bubble ai-thinking">
                <Loader2
                  size={15}
                  className="animate-spin"
                  aria-hidden="true"
                />
                {t("workbench.thinking")}
              </div>
            </div>
          )}
        </div>

        {blocked && (
          <p className="ai-empty-note" role="status">
            {t("workbench.disabled")}{" "}
            <Link to="/settings">{t("workbench.openSettings")}</Link>
          </p>
        )}
        {!!error && <ErrorBanner error={error} />}
        {error instanceof ApiError && error.code === "unavailable" && (
          <p className="ai-empty-note" role="status">
            {t("workbench.modelFailed")}
          </p>
        )}
        {approval && (
          <ApprovalCard approval={approval} busy={deciding} onDecide={decide} />
        )}

        <form
          className="ai-composer"
          onSubmit={(event) => {
            event.preventDefault();
            void send(draft);
          }}
        >
          <label className="sr-only" htmlFor="ai-question">
            {t("workbench.ask")}
          </label>
          <textarea
            id="ai-question"
            value={draft}
            maxLength={2000}
            rows={2}
            placeholder={t("workbench.placeholder")}
            disabled={blocked}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                void send(draft);
              }
            }}
          />
          <button
            type="submit"
            className="ai-send"
            disabled={busy || blocked || !draft.trim()}
            aria-label={t("workbench.send")}
            title={t("workbench.send")}
          >
            {busy ? (
              <Loader2 size={16} className="animate-spin" aria-hidden="true" />
            ) : (
              <CornerDownLeft size={16} aria-hidden="true" />
            )}
          </button>
        </form>
      </section>

      <aside className="admin-panel entity-card ai-side">
        <h3>{t("workbench.pending")}</h3>
        {!pending.length ? (
          <p className="gov-empty">{t("workbench.noPending")}</p>
        ) : (
          <ul className="ai-pending">
            {/* Only the requester can confirm from the workbench; a reviewer with
                ai:approve sees the same card here and decides on behalf. */}
            {pending.map((item) => (
              <li key={item.id}>
                <div>
                  <strong>{capabilityLabel(t, item.capability)}</strong>
                  <small>{formatMoment(i18n.language, item.requestedAt)}</small>
                </div>
                <button
                  type="button"
                  className="ai-button is-ghost"
                  disabled={deciding}
                  onClick={() => setApproval(item)}
                >
                  {t("workbench.review")}
                </button>
              </li>
            ))}
          </ul>
        )}
        <dl className="ai-side-meta">
          <div>
            <dt>{t("workbench.department")}</dt>
            <dd>{session.activeDepartmentId}</dd>
          </div>
          <div>
            <dt>{t("workbench.registry")}</dt>
            <dd>{snapshot?.registryVersion ?? "—"}</dd>
          </div>
        </dl>
      </aside>
    </div>
  );
}
