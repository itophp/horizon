import { getAiRunEventsUrl } from "@/api/generated/baseline";
import type { AIRunEvent } from "@/api/generated/models";
// A companion to the generated SSE contract. No business mutation is exposed here.
export function subscribeTaskEvents(
  runId: number,
  onEvent: (event: AIRunEvent) => void,
  onError: () => void,
  after = 0,
) {
  const source = new EventSource(getAiRunEventsUrl(runId, { after }));
  source.onmessage = (message) => {
    try {
      const value: unknown = JSON.parse(message.data);
      if (
        !value ||
        typeof value !== "object" ||
        !("type" in value) ||
        typeof value.type !== "string" ||
        !("runId" in value) ||
        value.runId !== String(runId) ||
        !("eventId" in value) ||
        typeof value.eventId !== "string"
      )
        throw new Error("invalid");
      onEvent(value as AIRunEvent);
      const snapshot = "snapshot" in value ? value.snapshot : undefined;
      const terminalStatus =
        snapshot &&
        typeof snapshot === "object" &&
        "status" in snapshot &&
        ["completed", "failed", "cancelled"].includes(String(snapshot.status));
      if (["RUN_FINISHED", "RUN_ERROR"].includes(value.type) || terminalStatus)
        source.close();
    } catch {
      source.close();
      onError();
    }
  };
  source.onerror = () => {
    source.close();
    onError();
  };
  return () => source.close();
}
