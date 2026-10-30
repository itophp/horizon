import { it, expect, vi } from "vitest";
import { request, ApiError, setCSRFToken } from "./request";
it("maps internal server errors to stable codes", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(new Response("secret stack trace", { status: 500 })),
  );
  try {
    await expect(request("/api/echo")).rejects.toMatchObject({
      code: "internal",
      status: 500,
    });
    await expect(request("/api/echo")).rejects.toBeInstanceOf(ApiError);
  } finally {
    vi.unstubAllGlobals();
  }
});
it("supplies an in-memory CSRF nonce only to mutations and clears it on logout", async () => {
  const fetch = vi
    .fn()
    .mockImplementation(async () => new Response("{}", { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  try {
    setCSRFToken("verified-nonce");
    await request("/api/admin/accounts", { method: "POST" });
    expect(fetch.mock.calls[0][1].headers.get("X-CSRF-Token")).toBe(
      "verified-nonce",
    );
    expect(fetch.mock.calls[0][1].credentials).toBe("same-origin");
    await request("/api/admin/snapshot");
    expect(fetch.mock.calls[1][1].headers.has("X-CSRF-Token")).toBe(false);
    setCSRFToken("");
    await request("/api/auth/login", { method: "POST" });
    expect(fetch.mock.calls[2][1].headers.has("X-CSRF-Token")).toBe(false);
  } finally {
    setCSRFToken("");
    vi.unstubAllGlobals();
  }
});

it("accepts a successful empty response such as cookie-only logout", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(new Response(null, { status: 200 })),
  );
  try {
    await expect(
      request("/api/auth/logout", { method: "POST" }),
    ).resolves.toMatchObject({ data: undefined, status: 200 });
  } finally {
    vi.unstubAllGlobals();
  }
});
