import { afterEach, expect, it, vi } from "vitest";
import { streamAIRun } from "./ai-stream";
import { setCSRFToken } from "./request";

afterEach(() => {
  vi.unstubAllGlobals();
  setCSRFToken("");
});

function response(parts: string[]) {
  const encoder = new TextEncoder();
  return new Response(
    new ReadableStream({
      start(controller) {
        for (const part of parts) controller.enqueue(encoder.encode(part));
        controller.close();
      },
    }),
    { headers: { "Content-Type": "text/event-stream" } },
  );
}
it("receives incremental SSE frames across chunks with CSRF and no mutation retry", async () => {
  const frame = (sequence: number, kind: string, extra = {}) =>
    `data: ${JSON.stringify({ sequence, kind, delta: "", capability: "", status: "", errorCode: "", ...extra })}\r\n\r\n`;
  const body =
    frame(1, "planning") +
    frame(2, "answer_delta", { delta: "你好" }) +
    frame(3, "completed", { run: { id: 7, status: "completed" } });
  const fetch = vi
    .fn()
    .mockResolvedValue(
      response([
        body.slice(0, body.indexOf("\r") + 1),
        body.slice(body.indexOf("\r") + 1),
      ]),
    );
  vi.stubGlobal("fetch", fetch);
  setCSRFToken("nonce");
  const received: string[] = [];
  const result = await streamAIRun(
    { question: "question" },
    {
      signal: new AbortController().signal,
      onEvent: (event) => received.push(event.kind),
    },
  );
  expect(received).toEqual(["planning", "answer_delta", "completed"]);
  expect(result.id).toBe(7);
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(fetch.mock.calls[0][0]).toBe("/api/ai/runs/stream");
  expect(fetch.mock.calls[0][1].headers.get("X-CSRF-Token")).toBe("nonce");
});
it("rejects incomplete streams and pre-stream authorization errors", async () => {
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(
      response([
        'data: {"sequence":1,"kind":"planning","delta":"","capability":"","status":"","errorCode":""}\n\n',
      ]),
    )
    .mockResolvedValueOnce(new Response("private details", { status: 403 }));
  vi.stubGlobal("fetch", fetch);
  const options = { signal: new AbortController().signal, onEvent: vi.fn() };
  await expect(
    streamAIRun({ question: "question" }, options),
  ).rejects.toMatchObject({ code: "unavailable" });
  await expect(
    streamAIRun({ question: "question" }, options),
  ).rejects.toMatchObject({ code: "forbidden", status: 403 });
  expect(fetch).toHaveBeenCalledTimes(2);
});
