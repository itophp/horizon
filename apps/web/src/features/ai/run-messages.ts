import { useMemo } from "react";
import type { AIRun } from "@/api/generated/models";
import type { Turn } from "./use-workbench";

export function useRunMessages(
  history: AIRun[] | undefined,
  turns: Turn[],
  busy: boolean,
  emptyAnswer: string,
) {
  return useMemo(() => {
    const localIds = new Set(
      turns.flatMap((turn) => (turn.run ? [turn.run.id] : [])),
    );
    const restored: Turn[] = [];
    for (const run of history ?? []) {
      if (localIds.has(run.id)) continue;
      restored.push(
        { id: `q-${run.id}`, role: "user", content: run.question },
        {
          id: `a-${run.id}`,
          role: "assistant",
          content: run.answer || emptyAnswer,
          run,
        },
      );
    }
    const authoritative = new Map((history ?? []).map((run) => [run.id, run]));
    return [
      ...restored,
      ...turns.map((turn) => {
        const stored = turn.run && authoritative.get(turn.run.id);
        return stored && stored.status !== "running" && !busy
          ? { ...turn, run: stored }
          : turn;
      }),
    ];
  }, [history, turns, busy, emptyAnswer]);
}
