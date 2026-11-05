import { beforeEach, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
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
  stream: vi.fn(),
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

vi.mock("@/api/client/ai-stream", () => ({
  streamAIRun: (...args: unknown[]) => api.stream(...args),
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
  api.stream.mockImplementation(async (input, options) => {
    const result = await api.run(input, { signal: options.signal });
    if (result.status !== 201) throw new Error("unavailable");
    options.onEvent({
      sequence: 1,
      kind: "started",
      delta: "",
      capability: "",
      status: "",
      errorCode: "",
      run: result.data,
    });
    options.onEvent({
      sequence: 2,
      kind: "completed",
      delta: "",
      capability: "",
      status: "",
      errorCode: "",
      run: result.data,
    });
    return result.data;
  });
  api.runDetail.mockResolvedValue({
    status: 200,
    data: {
      ...run,
      status: "completed",
      invocations: run.invocations.map((item) => ({
        ...item,
        status: "succeeded",
        errorCode: "",
      })),
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
  const composer = await screen.findByRole("textbox", {
    name: "Ask the agent",
  });
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
  api.approvals.mockResolvedValue({ status: 200, data: [] });
  await userEvent.click(confirm);
  await waitFor(() =>
    expect(api.decide).toHaveBeenCalledWith(5, { approve: true, note: "" }),
  );
  await waitFor(() =>
    expect(
      screen.queryByText("This action needs your confirmation"),
    ).toBeNull(),
  );
  expect(await screen.findByText("Completed")).toBeVisible();
  expect(screen.queryByText("Awaiting approval")).not.toBeInTheDocument();
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
  api.runs.mockImplementation(async ({ sessionId }) => ({
    status: 200,
    data: sessionId === "session-1" ? [run] : [],
  }));
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
  await userEvent.click(screen.getByRole("combobox", { name: "Sort tasks" }));
  await userEvent.click(
    await screen.findByRole("option", { name: "Oldest first" }),
  );
  await waitFor(() =>
    expect(titles()).toEqual([
      "zzz follow up",
      "archive the record",
      "summarize the audit log",
    ]),
  );
  await userEvent.click(screen.getByRole("combobox", { name: "Sort tasks" }));
  await userEvent.click(
    await screen.findByRole("option", { name: "Title A–Z" }),
  );
  await waitFor(() =>
    expect(titles()).toEqual([
      "archive the record",
      "summarize the audit log",
      "zzz follow up",
    ]),
  );
});

it("does not submit Enter while composing text or adding a new line", async () => {
  show();
  const composer = await screen.findByLabelText("Ask the agent");
  await userEvent.type(composer, "整理通知");
  fireEvent.keyDown(composer, {
    key: "Enter",
    isComposing: true,
    keyCode: 229,
  });
  expect(api.run).not.toHaveBeenCalled();
  fireEvent.keyDown(composer, { key: "Enter", shiftKey: true });
  expect(api.run).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  await waitFor(() => expect(api.run).toHaveBeenCalledTimes(1));
});

it("stops waiting and ignores a late answer without deciding a business action", async () => {
  let resolveRun!: (value: { status: 201; data: AIRun }) => void;
  api.run.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveRun = resolve;
      }),
  );
  show();
  await userEvent.type(
    await screen.findByLabelText("Ask the agent"),
    "archive the record",
  );
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  await userEvent.click(
    await screen.findByRole("button", { name: "Stop waiting" }),
  );
  expect(api.run.mock.calls.at(-1)?.[1].signal.aborted).toBe(true);
  expect(screen.getByText(/server task may still finish/)).toBeVisible();
  await act(async () => {
    resolveRun({ status: 201, data: run });
  });
  expect(screen.queryByText(run.answer)).not.toBeInTheDocument();
  expect(api.decide).not.toHaveBeenCalled();
});

it("aborts the response wait when leaving the workbench", async () => {
  let resolveRun!: (value: { status: 201; data: AIRun }) => void;
  api.run.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveRun = resolve;
      }),
  );
  const view = show();
  await userEvent.type(
    await screen.findByLabelText("Ask the agent"),
    "archive the record",
  );
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  const signal = api.run.mock.calls.at(-1)?.[1].signal;
  view.unmount();
  expect(signal.aborted).toBe(true);
  await act(async () => {
    resolveRun({ status: 201, data: run });
  });
  expect(api.decide).not.toHaveBeenCalled();
});

it("keeps failed input available and exposes trace expansion through the keyboard", async () => {
  api.run.mockRejectedValueOnce(new Error("unavailable"));
  show();
  const composer = await screen.findByLabelText("Ask the agent");
  await userEvent.type(composer, "archive the record");
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  await waitFor(() => expect(composer).toHaveValue("archive the record"));
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  const trace = await screen.findByRole("button", { name: "Execution steps" });
  expect(trace).toHaveAttribute("aria-expanded", "true");
  trace.focus();
  await userEvent.keyboard("{Enter}");
  expect(trace).toHaveAttribute("aria-expanded", "false");
  await userEvent.keyboard("{Enter}");
  expect(trace).toHaveAttribute("aria-expanded", "true");
});

it("renders invocation summaries without exposing raw tool input", async () => {
  api.run.mockResolvedValueOnce({
    status: 201,
    data: {
      ...run,
      invocations: [
        { ...run.invocations[0], input: { credential: "must-never-appear" } },
      ],
    },
  });
  show();
  await userEvent.type(
    await screen.findByLabelText("Ask the agent"),
    "archive the record",
  );
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  expect(await screen.findByText(run.answer)).toBeVisible();
  expect(screen.queryByText(/must-never-appear/)).not.toBeInTheDocument();
  expect(screen.queryByText("credential")).not.toBeInTheDocument();
});

it("lists a running conversation immediately and keeps its stream alive across chat switches", async () => {
  let finish!: (value: AIRun) => void;
  let signal!: AbortSignal;
  api.stream.mockImplementationOnce((_input, options) => {
    signal = options.signal;
    const event = {
      sequence: 1,
      kind: "started",
      delta: "",
      capability: "",
      status: "",
      errorCode: "",
      run: { ...run, status: "running", answer: "", invocations: [] },
    };
    options.onEvent(event);
    options.onEvent({
      ...event,
      sequence: 2,
      kind: "planning",
      run: undefined,
    });
    options.onEvent({
      ...event,
      sequence: 3,
      kind: "tool_start",
      capability: "example.record.archive",
      status: "running",
      run: undefined,
    });
    return new Promise((resolve) => {
      finish = resolve;
    });
  });
  show();
  await userEvent.type(
    await screen.findByRole("textbox", { name: "Ask the agent" }),
    "background request",
  );
  await userEvent.click(screen.getByRole("button", { name: "Send" }));
  expect(
    await screen.findByRole("button", { name: /^background request/ }),
  ).toHaveTextContent("Running");
  expect(screen.getByText(/Agent →/)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "New task" }));
  expect(signal.aborted).toBe(false);
  await act(async () => {
    finish({ ...run, status: "completed", invocations: [] });
  });
  await userEvent.click(
    screen.getByRole("button", { name: /^background request/ }),
  );
  expect(await screen.findByText(run.answer)).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "Stop waiting" }),
  ).not.toBeInTheDocument();
});
