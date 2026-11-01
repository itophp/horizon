import { expect, type APIRequestContext } from "@playwright/test";

// The console and the browser fixtures share one entry point: every write is a
// capability invocation, so a test cannot drift from what the UI actually calls.
export async function invoke<T>(
  request: APIRequestContext,
  headers: Record<string, string>,
  name: string,
  input: Record<string, unknown>,
): Promise<T> {
  const response = await request.post(`/api/capabilities/${name}/invocations`, {
    headers,
    data: { input, idempotencyKey: crypto.randomUUID() },
  });
  expect(
    response.ok(),
    `${name} failed: ${response.status()} ${await response.text()}`,
  ).toBeTruthy();
  const result = (await response.json()) as {
    status: string;
    errorCode?: string;
    data: T;
  };
  expect(result.status, `${name}: ${result.errorCode ?? ""}`).toBe("succeeded");
  return result.data;
}

export async function resetPreferences(
  request: APIRequestContext,
  headers: Record<string, string>,
) {
  await invoke(request, headers, "settings.preferences.update", {
    changes: [
      { key: "display.language", operation: "reset", value: "" },
      { key: "display.theme", operation: "reset", value: "" },
    ],
  });
}
