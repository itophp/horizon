import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { CapabilityPlansPage } from "./page";
const api = vi.hoisted(() => ({
  permissions: [] as string[],
  definitions: vi.fn(),
  list: vi.fn(),
  binding: vi.fn(),
  inspect: vi.fn(),
  history: vi.fn(),
  records: vi.fn(),
  trace: vi.fn(),
}));
vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  capabilityDefinitions: api.definitions,
}));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  executionPlans: api,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "a",
    account: { id: 1 },
    activeDepartmentId: 2,
    authorizationVersion: 1,
    permissions: api.permissions,
  }),
}));
vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));
vi.mock("@/features/orchestration/viewer", () => ({
  WorkflowViewer: () => null,
}));
const target = {
  capability: "example.record.archive",
  capabilityVersion: "1.0.0",
};
const plan = {
  planId: "default-archive",
  capabilityId: target.capability,
  capabilityVersion: "1.0.0",
  version: 1,
  mode: "direct",
  implementationRef: "example.record.archive@1.0.0",
  status: "published",
};
const row = {
  plan,
  name: "Default archive",
  scopeType: "platform",
  createdBy: 0,
  createdAt: "2026-11-06T00:00:00Z",
  publishedBy: 0,
};
function Location() {
  const loc = useLocation();
  return <output aria-label="Location">{loc.search}</output>;
}
function show(search = "?version=1.0.0") {
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter
          initialEntries={[`/capabilities/${target.capability}${search}`]}
        >
          <Routes>
            <Route
              path="/capabilities/:name"
              element={
                <>
                  <CapabilityPlansPage />
                  <Location />
                </>
              }
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}
beforeEach(async () => {
  vi.resetAllMocks();
  await changeLanguage("en", false);
  api.permissions = ["executionPlans:read"];
  api.definitions.mockResolvedValue({
    status: 200,
    data: [{ name: target.capability, version: "1.0.0", readOnly: false }],
  });
  api.list.mockResolvedValue({ data: { plans: [row] } });
  api.binding.mockResolvedValue({
    data: {
      binding: {
        bindingId: "default",
        scopeType: "platform",
        revision: 0,
        plan,
      },
    },
  });
  api.inspect.mockResolvedValue({
    data: { inputSchema: { type: "object" }, referenceHash: "" },
  });
  api.history.mockResolvedValue({ data: { changes: [] } });
  api.records.mockResolvedValue({ data: { records: [] } });
});
it("denies the page before fetching metadata when read permission is absent", async () => {
  api.permissions = [];
  show();
  expect(
    await screen.findByText("Execution plan read permission is required."),
  ).toBeTruthy();
  expect(api.definitions).not.toHaveBeenCalled();
  expect(api.list).not.toHaveBeenCalled();
});
it("shows read-only metadata without edit, publication, binding or records controls", async () => {
  show();
  await screen.findByText("Input contract");
  for (const name of [
    "Create draft",
    "Set as default",
    "Check structure",
    "Execution records",
  ]) {
    expect(screen.queryByRole("button", { name })).toBeNull();
  }
  expect(screen.queryByRole("tab", { name: "Execution records" })).toBeNull();
  expect(api.records).not.toHaveBeenCalled();
  expect(api.list).toHaveBeenCalledWith(target, expect.any(AbortSignal));
});
it("keeps the URL target and selection when switching history tabs", async () => {
  show("?version=1.0.0&planId=default-archive&planVersion=1");
  await screen.findByText("Input contract");
  await userEvent.click(screen.getByRole("tab", { name: "Binding history" }));
  await waitFor(() =>
    expect(api.history).toHaveBeenCalledWith(target, expect.any(AbortSignal)),
  );
  expect(screen.getByLabelText("Location").textContent).toContain(
    "planId=default-archive",
  );
  expect(screen.getByLabelText("Location").textContent).toContain(
    "tab=history",
  );
});
it("does not substitute the active plan for an inaccessible version in a URL", async () => {
  show("?version=1.0.0&planId=foreign&planVersion=99");
  expect(await screen.findByText("Unavailable or not authorized")).toBeTruthy();
  expect(api.inspect).not.toHaveBeenCalled();
});
it("loads actual node observations for an owned invocation through the gateway API", async () => {
  api.permissions.push("executionPlans:records");
  api.records.mockResolvedValue({
    data: {
      records: [
        {
          invocationId: "inv-own",
          binding: { plan },
          status: "executing",
          runStatus: "running",
          traceId: "trace-own",
          startedAt: "2026-11-06T00:00:00Z",
          errorCode: "",
        },
      ],
    },
  });
  api.trace.mockResolvedValue({
    data: {
      events: [
        {
          sequence: 1,
          stepName: "check",
          eventType: "STEP_STARTED",
          occurredAt: "2026-11-06T00:00:00Z",
        },
      ],
    },
  });
  show("?version=1.0.0&tab=records");
  await userEvent.click(
    await screen.findByRole("button", { name: "Node events" }),
  );
  await screen.findByText("check");
  expect(api.trace).toHaveBeenCalledWith(
    { invocationId: "inv-own" },
    expect.any(AbortSignal),
  );
  expect(api.records).toHaveBeenCalledWith(target, expect.any(AbortSignal));
});
