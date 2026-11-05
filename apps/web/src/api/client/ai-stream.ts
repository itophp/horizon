import { getAiRunStreamUrl } from "@/api/generated/baseline";
import type {
  AIStreamEvent,
  AIRun,
  RunRequestBody,
} from "@/api/generated/models";
import { ApiError, openResponse } from "./request";

// Orval supplies the URL and DTOs; the common client owns cookies, CSRF and errors.
// A mutation stream is never automatically retried or reconnected.
export async function streamAIRun(
  input: RunRequestBody,
  options: {
    signal: AbortSignal;
    onEvent: (event: AIStreamEvent) => void;
  },
): Promise<AIRun> {
  const response = await openResponse(getAiRunStreamUrl(), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    },
    body: JSON.stringify(input),
    signal: options.signal,
  });
  if (
    !response.headers.get("content-type")?.includes("text/event-stream") ||
    !response.body
  )
    throw new ApiError("unavailable", 502);
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "",
    sequence = 0;
  let result: AIRun | undefined;
  let terminal = false;
  function dispatch(frame: string) {
    const data = frame
      .split("\n")
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart())
      .join("\n");
    if (!data) return;
    const event = JSON.parse(data) as AIStreamEvent;
    if (
      !Number.isInteger(event.sequence) ||
      typeof event.kind !== "string" ||
      typeof event.delta !== "string" ||
      typeof event.capability !== "string" ||
      typeof event.status !== "string" ||
      typeof event.errorCode !== "string"
    )
      throw new ApiError("unavailable", 502);
    if (event.sequence <= sequence) return;
    sequence = event.sequence;
    options.onEvent(event);
    if (event.kind === "completed" || event.kind === "failed") {
      terminal = true;
      result = event.run;
      if (!result) throw new ApiError(event.errorCode || "unavailable", 502);
    }
  }
  try {
    while (!terminal) {
      const { value, done } = await reader.read();
      buffer = (buffer + decoder.decode(value, { stream: !done })).replace(
        /\r\n/g,
        "\n",
      );
      if (buffer.length > 1_048_576)
        throw new ApiError("result_too_large", 502);
      let boundary: number;
      while ((boundary = buffer.indexOf("\n\n")) >= 0) {
        const frame = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        dispatch(frame);
      }
      if (done) {
        if (buffer.trim()) dispatch(buffer);
        break;
      }
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  if (!terminal || !result) throw new ApiError("unavailable", 502);
  return result;
}
