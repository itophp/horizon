import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider, useAuth } from "./auth";
import type { SessionProfile } from "@/api/generated/models";
const api = vi.hoisted(() => ({ me: vi.fn() }));
vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  identityMe: api.me,
}));
function profile(version: string): SessionProfile {
  return {
    authorizationVersion: version.repeat(64),
    tenantId: "tenant-a",
    activeDepartmentId: 1,
    csrfToken: "test-nonce",
    permissions: ["accounts:read"],
    account: {
      id: 1,
      username: "demo",
      fullName: "Demo",
      email: "demo@example.test",
      departmentId: 1,
      organizationId: 1,
      isAdmin: false,
      status: "active",
      memberships: [],
      roleIds: [],
      overrides: [],
      createdAt: "",
      lastLogin: "",
    },
  };
}
function Probe() {
  const auth = useAuth();
  return (
    <>
      <span>{auth.profile?.authorizationVersion}</span>
      <input aria-label="Draft" />
      <button
        onClick={() => {
          void auth.refresh();
        }}
      >
        Refresh policy
      </button>
    </>
  );
}
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <AuthProvider>
        <Probe />
      </AuthProvider>
    </QueryClientProvider>,
  );
  return client;
}
beforeEach(() => {
  vi.clearAllMocks();
  api.me.mockResolvedValue({
    status: 200,
    data: profile("a"),
    headers: new Headers(),
  });
});
it("clears private cached data before publishing a scope-only authorization change", async () => {
  const client = mount();
  await screen.findByText("a".repeat(64));
  client.setQueryData(["private-records"], "old scope rows");
  api.me.mockResolvedValue({
    status: 200,
    data: profile("b"),
    headers: new Headers(),
  });
  await userEvent
    .setup()
    .click(screen.getByRole("button", { name: "Refresh policy" }));
  await screen.findByText("b".repeat(64));
  expect(client.getQueryData(["private-records"])).toBeUndefined();
});
it("preserves draft and cache when authorization is unchanged", async () => {
  const client = mount();
  await screen.findByText("a".repeat(64));
  client.setQueryData(["private-records"], "same scope rows");
  const user = userEvent.setup();
  await user.type(screen.getByRole("textbox", { name: "Draft" }), "Keep me");
  await user.click(screen.getByRole("button", { name: "Refresh policy" }));
  await waitFor(() => expect(api.me).toHaveBeenCalledTimes(2));
  expect(screen.getByRole("textbox", { name: "Draft" })).toHaveValue("Keep me");
  expect(client.getQueryData(["private-records"])).toBe("same scope rows");
});
