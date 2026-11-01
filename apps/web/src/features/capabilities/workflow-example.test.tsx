import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import type {
  CapabilityContract,
  CapabilityResult,
} from "@/api/generated/models";
import { WorkflowExamplePanel } from "./workflow-example";

const api = vi.hoisted(() => ({
  invoke: vi.fn(),
  approvals: vi.fn(),
  invocations: vi.fn(),
  decide: vi.fn(),
  model: vi.fn(),
  permissions: [] as string[],
  status: "awaiting_approval",
  department: 1,
}));
vi.mock("@/lib/capabilities", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/capabilities")>()),
  invokeCapability: api.invoke,
}));
vi.mock("@/api/generated/baseline", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/api/generated/baseline")>()),
  aiStatus: api.model,
  capabilityApprovals: api.approvals,
  capabilityInvocations: api.invocations,
  capabilityDecide: api.decide,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant-a",
    activeDepartmentId: api.department,
    account: { id: 4 },
    permissions: api.permissions,
  }),
}));
const contracts: CapabilityContract[] = [
  {
    name: "orchestration.workflow.run",
    version: "1.0.0",
    title: "Workflow run",
    description: "Run",
    domain: "orchestration",
    entity: "orchestration_checkpoint",
    permission: "orchestration:run",
    risk: "high",
    approval: "confirm",
    idempotent: true,
    readOnly: false,
    executor: "go:run",
    tags: [],
    successEvents: [],
    inputSchema: {},
    outputSchema: {},
  },
];
const envelope = (
  data: unknown,
  status: CapabilityResult["status"] = "succeeded",
): CapabilityResult => ({
  data,
  status,
  invocationId: "start",
  traceId: "trace-demo",
  eventIds: [],
  errorCode: "",
  replayed: false,
});
function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const view = () => (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <WorkflowExamplePanel contracts={contracts} />
      </QueryClientProvider>
    </I18nextProvider>
  );
  return { ...render(view()), rerenderView: () => view(), client };
}
beforeEach(async () => {
  vi.resetAllMocks();
  await changeLanguage("en", false);
  api.department = 1;
  api.status = "awaiting_approval";
  api.permissions = [
    "orchestration:run",
    "orchestration:read",
    "orchestration:cancel",
    "example:create",
    "example:read",
    "example:archive",
  ];
  api.model.mockResolvedValue({
    status: 200,
    data: { enabled: true, configured: true },
  });
  api.invoke.mockImplementation(async (name: string) => {
    if (name === "orchestration.workflow.run") return envelope({ runId: 42 });
    if (name === "orchestration.workflow.cancel") {
      api.status = "cancelled";
      return envelope({ runId: 42 });
    }
    return envelope({ runId: 42, status: api.status, errorCode: "" });
  });
  api.invocations.mockImplementation(async () => ({
    status: 200,
    data: [
      {
        invocationId: "archive-demo",
        correlationId: "trace-demo",
        capability: "example.record.archive",
        origin: "workflow",
        status: api.status === "completed" ? "succeeded" : "awaiting_approval",
      },
    ],
  }));
  api.approvals.mockImplementation(async () => ({
    status: 200,
    data:
      api.status === "awaiting_approval"
        ? [
            {
              id: 7,
              invocationId: "archive-demo",
              capability: "example.record.archive",
              status: "pending",
              arguments: { id: 9 },
              risk: "high",
              expiresAt: "2026-11-01T18:00:00Z",
            },
            {
              id: 8,
              invocationId: "someone-else",
              capability: "example.record.archive",
              status: "pending",
              arguments: {},
              expiresAt: "2026-11-01T18:00:00Z",
            },
          ]
        : [],
  }));
  api.decide.mockImplementation(async () => {
    api.status = "completed";
    return { status: 200, data: envelope({ id: 9 }) };
  });
});
it("starts the basic flow, waits for an explicit decision, then shows server completion", async () => {
  const user = userEvent.setup();
  show();
  await user.click(screen.getByRole("button", { name: "Run basic test" }));
  await screen.findByRole("button", { name: "Confirm and run" });
  expect(api.invoke).toHaveBeenCalledWith(
    "orchestration.workflow.run",
    {
      flow: "example-create-archive",
      version: "1.0.0",
      input: { title: "Workflow page test record" },
    },
    expect.any(String),
  );
  expect(api.decide).not.toHaveBeenCalled();
  expect(
    screen.getAllByRole("button", { name: "Confirm and run" }),
  ).toHaveLength(1);
  expect(screen.getByRole("button", { name: "Run basic test" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Confirm and run" }));
  expect(api.decide).toHaveBeenCalledWith(7, { approve: true });
  await screen.findByText(
    "Test completed: the reference record finished the workflow and was archived.",
  );
});
it("keeps the basic flow available without a model, and enables the full flow only when ready", async () => {
  api.permissions.push("assistant:use");
  api.model.mockResolvedValue({
    status: 200,
    data: { enabled: false, configured: false },
  });
  const user = userEvent.setup();
  const { client } = show();
  expect(screen.getByRole("button", { name: "Run basic test" })).toBeEnabled();
  expect(
    screen.getByRole("button", { name: "Run full test (with agent)" }),
  ).toBeDisabled();
  await waitFor(() => expect(client.isFetching()).toBe(0));
  api.model.mockResolvedValue({
    status: 200,
    data: { enabled: true, configured: true },
  });
  await client.invalidateQueries();
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Run full test (with agent)" }),
    ).toBeEnabled(),
  );
  await user.click(
    screen.getByRole("button", { name: "Run full test (with agent)" }),
  );
  await waitFor(() =>
    expect(api.invoke).toHaveBeenCalledWith(
      "orchestration.workflow.run",
      expect.objectContaining({ flow: "example-lifecycle" }),
      expect.any(String),
    ),
  );
});
it("does not offer execution without the required permissions", () => {
  api.permissions = ["orchestration:read"];
  show();
  expect(
    screen.queryByRole("button", { name: "Run basic test" }),
  ).not.toBeInTheDocument();
  expect(api.invoke).not.toHaveBeenCalled();
});
it("cancels only the selected run through the capability SDK", async () => {
  api.status = "running";
  const user = userEvent.setup();
  show();
  await user.click(screen.getByRole("button", { name: "Run basic test" }));
  await user.click(await screen.findByRole("button", { name: "Cancel run" }));
  await screen.findByText("Run cancelled. Already committed records remain.");
  expect(api.invoke).toHaveBeenCalledWith(
    "orchestration.workflow.cancel",
    { runId: 42 },
    "workflow-demo:cancel:42",
  );
});
it("reuses the exact action payload and idempotency key on a start retry", async () => {
  api.invoke.mockRejectedValueOnce(new Error("network error"));
  const user = userEvent.setup();
  show();
  await user.click(screen.getByRole("button", { name: "Run basic test" }));
  const retry = await screen.findByRole("button", { name: "Try again" });
  const first = api.invoke.mock.calls[0];
  await user.click(retry);
  await waitFor(() =>
    expect(
      api.invoke.mock.calls.filter(
        (call) => call[0] === "orchestration.workflow.run",
      ),
    ).toHaveLength(2),
  );
  const calls = api.invoke.mock.calls.filter(
    (call) => call[0] === "orchestration.workflow.run",
  );
  expect(calls[1]).toEqual(first);
});
it("clears the selected run when the active department changes", async () => {
  const user = userEvent.setup();
  const view = show();
  await user.click(screen.getByRole("button", { name: "Run basic test" }));
  await screen.findByText("Run #42");
  api.department = 2;
  view.rerender(view.rerenderView());
  expect(screen.queryByText("Run #42")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Run basic test" })).toBeEnabled();
});
