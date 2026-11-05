import { useRunMessages } from "./run-messages";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useLocation } from "react-router";
import {
  aiRunDetail,
  aiRuns,
  aiStatus,
  capabilityApprovals,
  capabilityDecide,
} from "@/api/generated/baseline";
import { streamAIRun } from "@/api/client/ai-stream";
import type {
  AIStreamEvent,
  AIRun,
  CapabilityApproval,
} from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import { useSession } from "@/features/administration/auth";
import { useCapabilityKey } from "@/features/capabilities/shared";
import { useAiKey } from "./shared";

export interface Turn {
  id: string;
  role: "user" | "assistant";
  content: string;
  run?: AIRun;
  progress?: AIStreamEvent[];
}

function identifier() {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`;
}

export function useWorkbench({
  sessionId,
  onTurnComplete,
  onRunState,
}: {
  sessionId?: string | null;
  onTurnComplete?: () => void;
  onRunState?: (sessionId: string, title: string, running: boolean) => void;
}) {
  const { t } = useTranslation("ai");
  const session = useSession();
  const location = useLocation();
  const client = useQueryClient();
  const [turns, setTurns] = useState<Turn[]>([]);
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [stopped, setStopped] = useState(false);
  const [deciding, setDeciding] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [approval, setApproval] = useState<CapabilityApproval | null>(null);
  // A reopened task keeps its session id so the server can replay the thread;
  // a new task gets one on first use.
  const thread = useRef(sessionId ?? identifier());
  const controller = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      controller.current?.abort();
      controller.current = null;
    },
    [],
  );
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
  const historyKey = useAiKey("runs", sessionId ?? "new");
  const history = useQuery({
    queryKey: historyKey,
    queryFn: () => aiRuns({ sessionId: sessionId ?? undefined, limit: 100 }),
    enabled: Boolean(sessionId),
    retry: false,
  });
  const messages = useRunMessages(
    history.data?.status === 200 ? history.data.data : undefined,
    turns,
    busy,
    t("workbench.emptyAnswer"),
  );

  async function send(text: string) {
    const question = text.trim();
    if (!question || busy || controller.current || blocked) return;
    setError(null);
    setBusy(true);
    setStopped(false);
    onRunState?.(thread.current, question, true);
    setDraft("");
    const replyId = identifier();
    setTurns((current) => [
      ...current,
      { id: identifier(), role: "user", content: question },
      { id: replyId, role: "assistant", content: "", progress: [] },
    ]);
    const abort = new AbortController();
    controller.current = abort;
    try {
      const run = await streamAIRun(
        {
          question,
          sessionId: thread.current,
          page: { route: location.pathname },
        },
        {
          signal: abort.signal,
          onEvent: (event) => {
            if (abort.signal.aborted || controller.current !== abort) return;
            setTurns((current) =>
              current.map((turn) => {
                if (turn.id !== replyId) return turn;
                return {
                  ...turn,
                  run:
                    event.kind === "started" && event.run
                      ? event.run
                      : turn.run,
                  content:
                    event.kind === "answer_delta"
                      ? turn.content + event.delta
                      : turn.content,
                  progress:
                    event.kind === "answer_delta"
                      ? turn.progress
                      : [...(turn.progress ?? []).slice(-99), event],
                };
              }),
            );
            if (event.kind === "started") onTurnComplete?.();
          },
        },
      );
      if (abort.signal.aborted) return;
      setTurns((current) =>
        current.map((turn) =>
          turn.id === replyId
            ? {
                ...turn,
                content: run.answer || t("workbench.emptyAnswer"),
                run,
              }
            : turn,
        ),
      );
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
      if (abort.signal.aborted || controller.current !== abort) return;
      setApproval(card ?? null);
      await client.invalidateQueries({ queryKey: runsKey });
      onTurnComplete?.();
      if (card) {
        await client.invalidateQueries({ queryKey: approvalsKey });
      }
    } catch (caught) {
      if (abort.signal.aborted) return;
      setError(caught);
      setDraft((current) => current || question);
      // A refused turn is still a recorded task; show it in the run history.
      await client.invalidateQueries({ queryKey: runsKey });
      onTurnComplete?.();
    } finally {
      if (controller.current === abort) {
        controller.current = null;
        setBusy(false);
        onRunState?.(thread.current, question, false);
      }
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
      await client.invalidateQueries({ queryKey: historyKey });
      const ids = turns.flatMap((turn) => (turn.run ? [turn.run.id] : []));
      const refreshed = await Promise.all(ids.map((id) => aiRunDetail(id)));
      setTurns((current) =>
        current.map((turn) => {
          const response = refreshed.find(
            (value) => value.status === 200 && value.data.id === turn.run?.id,
          );
          return response?.status === 200
            ? { ...turn, run: response.data }
            : turn;
        }),
      );
    } catch (caught) {
      setError(caught);
    } finally {
      setDeciding(false);
    }
  }

  function stop() {
    controller.current?.abort();
    controller.current = null;
    setBusy(false);
    setStopped(true);
    onRunState?.(thread.current, "", false);
  }
  return {
    session,
    snapshot,
    blocked,
    pending,
    history,
    status,
    approvals,
    messages,
    draft,
    setDraft,
    busy,
    stopped,
    deciding,
    error,
    approval,
    setApproval,
    send,
    decide,
    stop,
  };
}
