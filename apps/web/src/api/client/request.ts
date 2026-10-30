let csrfToken = "";
export function setCSRFToken(value: string) {
  csrfToken = value;
}
export class ApiError extends Error {
  constructor(
    public readonly code: string,
    public readonly status: number,
  ) {
    super(code);
  }
}
// No automatic mutation retries. The session stays in an HttpOnly cookie; only its CSRF nonce is held in memory.
export async function request<T>(
  url: string,
  options: RequestInit = {},
): Promise<T> {
  const timeout =
    url.startsWith("/api/admin/files") ||
    url.startsWith("/api/admin/dictionaries")
      ? 120000
      : 10000;
  const signal = options.signal
    ? AbortSignal.any([options.signal, AbortSignal.timeout(timeout)])
    : AbortSignal.timeout(timeout);
  const headers = new Headers(options.headers);
  const method = (options.method ?? "GET").toUpperCase();
  if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken)
    headers.set("X-CSRF-Token", csrfToken);
  const response = await fetch(url, {
    ...options,
    signal,
    headers,
    credentials: "same-origin",
  });
  if (!response.ok) {
    const codes: Record<number, string> = {
      400: "invalid",
      401: "unauthenticated",
      403: "forbidden",
      404: "not_found",
      409: "conflict",
      429: "rate_limited",
      503: "unavailable",
    };
    if (response.status === 401 && url.startsWith("/api/admin/"))
      window.dispatchEvent(new Event("baseline:unauthenticated"));
    throw new ApiError(codes[response.status] ?? "internal", response.status);
  }
  const body = response.status === 204 ? "" : await response.text();
  const data: unknown = body.length === 0 ? undefined : JSON.parse(body);
  return { data, status: response.status, headers: response.headers } as T;
}
