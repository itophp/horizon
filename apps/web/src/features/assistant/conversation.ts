import { skipToken, useQuery } from "@tanstack/react-query";
import { aiRuns } from "@/api/generated/baseline";
import { ApiError } from "@/api/client/request";
import type { AssistantTurn } from "./provider";

const storageKey = "baseline.assistant.conversation";

export function readConversation(identityKey: string): string | undefined {
  try {
    const prefix = `${identityKey}:`;
    const value = window.sessionStorage.getItem(storageKey);
    if (!value?.startsWith(prefix)) return;
    const id = value.slice(prefix.length);
    if (/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(id)) {
      return id;
    }
  } catch {
    // The assistant can still run when browser storage is unavailable.
  }
}

export function rememberConversation(identityKey: string, id?: string) {
  try {
    if (id) {
      window.sessionStorage.setItem(storageKey, `${identityKey}:${id}`);
    } else {
      window.sessionStorage.removeItem(storageKey);
    }
  } catch {
    // Keep the in-memory conversation usable when storage is unavailable.
  }
}

export function useRestoredConversation(
  identityKey: string,
  sessionId: string | undefined,
  ready: boolean,
) {
  return useQuery({
    queryKey: ["assistant-history", identityKey, sessionId],
    // Load the prior transcript once. New turns are displayed from live state.
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
    queryFn:
      ready && sessionId
        ? async ({ signal }): Promise<AssistantTurn[]> => {
            const response = await aiRuns(
              { sessionId, limit: 100 },
              { signal },
            );
            if (response.status !== 200)
              throw new ApiError("unavailable", response.status);
            return response.data.flatMap((run) => {
              const turns: AssistantTurn[] = [
                {
                  id: `run-${run.id}-question`,
                  role: "user",
                  content: run.question,
                },
              ];
              if (run.answer) {
                turns.push({
                  id: `run-${run.id}-answer`,
                  role: "assistant",
                  content: run.answer,
                });
              }
              return turns;
            });
          }
        : skipToken,
  });
}
