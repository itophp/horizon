import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { changeLanguage, i18n } from "@/i18n";
import type { AIRun, CapabilityApproval } from "@/api/generated/models";
import { AiPage } from "./page";

const api = vi.hoisted(() => ({
  status: vi.fn(),
  run: vi.fn(),
  runs: vi.fn(),
  runDetail: vi.fn(),
  conversations: vi.fn(),
  updateConversation: vi.fn(),
  approvals: vi.fn(),
  decide: vi.fn(),
  permissions: [] as string[],
  activeDepartment: 1,
}));

vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  aiStatus: api.status,
  aiRun: api.run,
  aiRuns: api.runs,
  aiConversations: api.conversations,
  aiUpdateConversation: api.updateConversation,
  aiRunDetail: api.runDetail,
  capabilityApprovals: api.approvals,
  capabilityDecide: api.decide,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant-a",
    activeDepartmentId: api.activeDepartment,
    permissions: api.permissions,
    account: { id: 4, fullName: "Ada", memberships: [] },
  }),
}));

const run: AIRun = {
  id: 9,
  question: "archive the record",
  answer: "The archive action is ready and waits for your confirmation.",
  status: "awaiting_approval",
  agent: "workspace_agent",
  model: "glm-test",
  instructionVersion: "test",
  origin: "human-ui",
  errorCode: "",
  toolCalls: 1,
  promptTokens: 12,
  completionTokens: 8,
  durationMs: 900,
  traceId: "trace-1234567890",
  correlationId: "trace-1234567890",
  causationId: "run:9",
  triggerDepth: 0,
  triggerPolicy: "",
  eventId: "",
  createdAt: "2026-10-31T02:00:00Z",
  finishedAt: "2026-10-31T02:00:01Z",
  invocations: [
    {
      invocationId: "inv_1",
      capability: "example.record.archive",
      version: "1.0.0",
      origin: "agent",
      status: "awaiting_approval",
      input: { id: 7 },
      inputDigest: "digest",
      result: undefined,
      errorCode: "approval_required",
      approvalId: 5,
      correlationId: "trace-1234567890",
      causationId: "run:9",
      triggerDepth: 0,
      durationMs: 3,
      traceId: "trace-1234567890",
      createdAt: "2026-10-31T02:00:00Z",
      finishedAt: "",
    },
  ],
};

const card: CapabilityApproval = {
  id: 5,
  invocationId: "inv_1",
  capability: "example.record.archive",
  version: "1.0.0",
  risk: "high",
  approval: "confirm",
  status: "pending",
  arguments: { id: 7 },
  requestedAt: "2026-10-31T02:00:00Z",
  expiresAt: "2026-10-31T02:10:00Z",
  decidedAt: "",
  decisionNote: "",
  executionCode: "",
  mine: true,
};

function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/ai"]}>
          <AiPage />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  await changeLanguage("en", false);
  api.permissions = ["assistant:use"];
  api.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: true,
      configured: true,
      model: "glm-test",
      registryVersion: "test",
      capabilityCount: 7,
      tasksTotal: 0,
      tasksCompleted: 0,
      tasksFailed: 0,
      completionRate: 0,
      averageDurationMs: 0,
      promptTokens: 0,
      completionTokens: 0,
    },
  });
  api.runs.mockResolvedValue({ status: 200, data: [] });
  api.conversations.mockResolvedValue({ status: 200, data: [] });
  api.approvals.mockResolvedValue({ status: 200, data: [card] });
  api.run.mockResolvedValue({ status: 201, data: run });
  api.updateConversation.mockResolvedValue({
    status: 200,
    data: {
      sessionId: "session-1",
      title: "archive the record",
      updatedAt: "2026-10-31T09:00:00Z",
      messageCount: 2,
      archived: false,
    },
  });
  api.decide.mockResolvedValue({
    status: 200,
    data: {
      invocationId: "inv_1",
      status: "succeeded",
      eventIds: ["evt_1"],
      errorCode: "",
      traceId: "trace-1234567890",
      replayed: false,
    },
  });
});

it("keeps the workbench closed without the AI grant", async () => {
  api.permissions = [];
  show();
  expect(
    await screen.findByText(/does not hold the AI usage grant/i),
  ).toBeTruthy();
  expect(api.run).not.toHaveBeenCalled();
});

it("explains a disabled agent instead of failing on send", async () => {
  api.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: false,
      configured: true,
      model: "glm-test",
      registryVersion: "test",
      capabilityCount: 7,
      tasksTotal: 0,
      tasksCompleted: 0,
      tasksFailed: 0,
      completionRate: 0,
      averageDurationMs: 0,
      promptTokens: 0,
      completionTokens: 0,
    },
  });
  show();
  expect(
    await screen.findByText(/not enabled for this workspace/i),
  ).toBeTruthy();
  expect(
    screen.getByRole("link", { name: "Open system settings" }),
  ).toBeTruthy();
  expect(
    (screen.getByLabelText("Ask the agent") as HTMLTextAreaElement).disabled,
  ).toBe(true);
  expect(api.run).not.toHaveBeenCalled();
});

it("runs a question and renders business invocations", async () => {
  show();
  const composer = await screen.findByLabelText("Ask the agent");
  await userEvent.type(composer, "archive the record");
  await userEvent.click(screen.getByRole("button", { name: /^Send$/ }));
  await waitFor(() =>
    expect(api.run).toHaveBeenCalledWith(
      expect.objectContaining({
        question: "archive the record",
        page: { route: "/ai" },
      }),
      expect.anything(),
    ),
  );
  expect(await screen.findByText(run.answer)).toBeTruthy();
  expect(screen.getAllByText("Archive example record").length).toBeGreaterThan(
    0,
  );
  expect(screen.getByText("Awaiting approval")).toBeTruthy();
  expect(api.approvals).toHaveBeenCalled();
});

it("requires an explicit confirmation before a high risk capability runs", async () => {
  show();
  const composer = await screen.findByLabelText("Ask the agent");
  await userEvent.type(composer, "archive the record");
  await userEvent.click(screen.getByRole("button", { name: /^Send$/ }));
  const confirm = await screen.findByRole("button", {
    name: "Confirm and run",
  });
  expect(screen.getByText("This action needs your confirmation")).toBeTruthy();
  expect(api.decide).not.toHaveBeenCalled();
  await userEvent.click(confirm);
  await waitFor(() =>
    expect(api.decide).toHaveBeenCalledWith(5, { approve: true, note: "" }),
  );
  await waitFor(() =>
    expect(
      screen.queryByText("This action needs your confirmation"),
    ).toBeNull(),
  );
});

it("lists tasks and reopens one with its history", async () => {
  api.conversations.mockResolvedValue({
    status: 200,
    data: [
      {
        sessionId: "session-1",
        title: "archive the record",
        updatedAt: "2026-10-31T09:00:00Z",
        messageCount: 2,
        archived: false,
      },
    ],
  });
  api.runs.mockResolvedValue({ status: 200, data: [run] });
  show();
  const task = await screen.findByRole("button", {
    name: /^archive the record/,
  });
  await userEvent.click(task);
  expect(await screen.findByText("Awaiting approval")).toBeTruthy();
  await waitFor(() =>
    expect(api.runs).toHaveBeenCalledWith(
      expect.objectContaining({ sessionId: "session-1" }),
    ),
  );
});

const conversations = [
  {
    sessionId: "session-1",
    title: "archive the record",
    updatedAt: "2026-10-31T09:00:00Z",
    messageCount: 2,
    archived: false,
  },
  {
    sessionId: "session-2",
    title: "summarize the audit log",
    updatedAt: "2026-10-31T10:00:00Z",
    messageCount: 1,
    archived: false,
  },
  {
    sessionId: "session-3",
    title: "old backlog review",
    updatedAt: "2026-10-30T10:00:00Z",
    messageCount: 4,
    archived: true,
  },
  {
    sessionId: "session-4",
    title: "zzz follow up",
    updatedAt: "2026-10-31T08:00:00Z",
    messageCount: 1,
    archived: false,
  },
];

it("renames a task through the owner's own title", async () => {
  api.conversations.mockResolvedValue({ status: 200, data: conversations });
  show();
  await userEvent.click(
    await screen.findByRole("button", {
      name: "Rename archive the record",
    }),
  );
  const field = screen.getByLabelText("Task title");
  await userEvent.clear(field);
  await userEvent.type(field, "Line 3 changeover");
  await userEvent.click(screen.getByRole("button", { name: "Save title" }));
  await waitFor(() =>
    expect(api.updateConversation).toHaveBeenCalledWith("session-1", {
      title: "Line 3 changeover",
    }),
  );
  expect(screen.queryByLabelText("Task title")).toBeNull();
});

it("keeps a rename open and reports a refused write", async () => {
  api.conversations.mockResolvedValue({ status: 200, data: conversations });
  api.updateConversation.mockResolvedValue({ status: 404, data: {} });
  show();
  await userEvent.click(
    await screen.findByRole("button", {
      name: "Rename archive the record",
    }),
  );
  const field = screen.getByLabelText("Task title");
  await userEvent.type(field, " changed");
  await userEvent.click(screen.getByRole("button", { name: "Save title" }));
  await waitFor(() => expect(api.updateConversation).toHaveBeenCalled());
  expect(await screen.findByLabelText("Task title")).toBeTruthy();
});

it("archives an active task and restores it from the archive view", async () => {
  api.conversations.mockResolvedValue({ status: 200, data: conversations });
  show();
  await userEvent.click(
    await screen.findByRole("button", { name: "Archive archive the record" }),
  );
  await waitFor(() =>
    expect(api.updateConversation).toHaveBeenCalledWith("session-1", {
      archived: true,
    }),
  );

  await userEvent.click(screen.getByRole("button", { name: /^Archived/ }));
  expect(
    await screen.findByRole("button", { name: /^old backlog review/ }),
  ).toBeTruthy();
  expect(
    screen.queryByRole("button", { name: /^archive the record/ }),
  ).toBeNull();

  await userEvent.click(
    screen.getByRole("button", { name: "Restore old backlog review" }),
  );
  await waitFor(() =>
    expect(api.updateConversation).toHaveBeenCalledWith("session-3", {
      archived: false,
    }),
  );
});

it("sorts tasks by last activity, oldest first or title", async () => {
  api.conversations.mockResolvedValue({ status: 200, data: conversations });
  show();
  const titles = () =>
    screen
      .getAllByRole("button", { name: /turns/ })
      .map((item) => item.querySelector("strong")?.textContent);

  await waitFor(() =>
    expect(titles()).toEqual([
      "summarize the audit log",
      "archive the record",
      "zzz follow up",
    ]),
  );
  await userEvent.selectOptions(screen.getByLabelText("Sort tasks"), "oldest");
  await waitFor(() =>
    expect(titles()).toEqual([
      "zzz follow up",
      "archive the record",
      "summarize the audit log",
    ]),
  );
  await userEvent.selectOptions(screen.getByLabelText("Sort tasks"), "title");
  await waitFor(() =>
    expect(titles()).toEqual([
      "archive the record",
      "summarize the audit log",
      "zzz follow up",
    ]),
  );
});
