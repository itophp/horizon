import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { AssistantProvider } from "./provider";
import { AssistantLauncher } from "./widget";

const mocks = vi.hoisted(() => ({
  status: vi.fn(),
  chat: vi.fn(),
  agent: vi.fn(),
  runs: vi.fn(),
  invoke: vi.fn(),
  departmentId: 1,
  permissions: ["assistant:use"] as string[],
  toolsAvailable: true,
}));

vi.mock("@/api/generated/baseline", () => ({
  assistantStatus: mocks.status,
  assistantChat: mocks.chat,
  assistantAgent: mocks.agent,
  aiRuns: mocks.runs,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "test",
    account: { id: 1 },
    activeDepartmentId: mocks.departmentId,
    authorizationVersion: "test-version",
    permissions: mocks.permissions,
  }),
}));

vi.mock("@/features/webmcp/provider", () => ({
  useWebMcp: () => ({
    status: "disconnected",
    toolCount: 1,
    connection: {
      ensureLocalTools: async () => undefined,
      registry: {
        list: () =>
          mocks.toolsAvailable
            ? [
                {
                  name: "baseline_page_context",
                  description: "Read current route",
                  inputSchema: { type: "object", properties: {} },
                },
              ]
            : [],
        invoke: mocks.invoke,
      },
    },
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  window.sessionStorage.clear();
  mocks.departmentId = 1;
  mocks.permissions = ["assistant:use"];
  mocks.toolsAvailable = true;
});

it("does not execute a page tool that disappeared while the agent was planning", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: true,
      configured: true,
      agentEnabled: true,
      model: "test",
    },
  });
  mocks.agent
    .mockImplementationOnce(async () => {
      mocks.toolsAvailable = false;
      return {
        status: 200,
        data: {
          content: "",
          tools: [],
          mode: "agent",
          model: "test",
          runId: 7,
          resumeToken: "single-use",
          pageCall: {
            id: "call-revoked",
            name: "baseline_page_context",
            arguments: "{}",
          },
        },
      };
    })
    .mockResolvedValueOnce({
      status: 200,
      data: {
        content: "The page tool is no longer available.",
        tools: [],
        mode: "agent",
        model: "test",
        runId: 7,
      },
    });
  renderAssistant();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Open agent" }));
  await user.type(
    screen.getByRole("textbox", { name: "Ask the workspace agent…" }),
    "Read this page",
  );
  await user.click(screen.getByRole("button", { name: "Send" }));
  await screen.findByText("The page tool is no longer available.");
  expect(mocks.invoke).not.toHaveBeenCalled();
  const resumed = mocks.agent.mock.calls[1][0];
  expect(JSON.parse(resumed.toolResult)).toEqual({
    content: '{"code":"tool_unavailable"}',
    isError: true,
  });
});

function renderAssistant() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <AssistantProvider>
          <AssistantLauncher />
        </AssistantProvider>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

it("restores server history after a reload and starts a fresh session after clearing", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: { enabled: true, configured: true, agentEnabled: true },
  });
  mocks.agent.mockResolvedValue({
    status: 200,
    data: { content: "First reply", tools: [] },
  });
  mocks.runs.mockResolvedValue({
    status: 200,
    data: [{ id: 7, question: "First question", answer: "First reply" }],
  });

  const user = userEvent.setup();
  async function open() {
    await user.click(await screen.findByRole("button", { name: "Open agent" }));
  }
  async function send(text: string) {
    await user.type(
      screen.getByRole("textbox", { name: "Ask the workspace agent…" }),
      text,
    );
    await user.click(screen.getByRole("button", { name: "Send" }));
  }

  const first = renderAssistant();
  await open();
  await send("First question");
  await screen.findByText("First reply");
  const { sessionId } = mocks.agent.mock.calls[0][0];
  first.unmount();

  const restored = renderAssistant();
  await open();
  await screen.findByText("First reply");
  expect(screen.getByText("First question")).toBeInTheDocument();
  expect(mocks.runs).toHaveBeenCalledWith(
    { sessionId, limit: 100 },
    expect.anything(),
  );
  await send("Follow-up");
  expect(mocks.agent.mock.calls[1][0].sessionId).toBe(sessionId);
  await waitFor(() =>
    expect(
      screen.queryByRole("button", { name: "Stop" }),
    ).not.toBeInTheDocument(),
  );
  await user.click(screen.getByRole("button", { name: /clear/i }));
  expect(screen.queryByText("First question")).not.toBeInTheDocument();
  await send("New question");
  expect(mocks.agent.mock.calls[2][0].sessionId).not.toBe(sessionId);
  restored.unmount();

  mocks.departmentId = 2;
  renderAssistant();
  await open();
  expect(mocks.runs).toHaveBeenCalledTimes(1);
  expect(screen.queryByText("First question")).not.toBeInTheDocument();
});

it("stays hidden for accounts without the assistant grant", async () => {
  await changeLanguage("en");
  mocks.permissions = [];
  mocks.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: true,
      configured: true,
      agentEnabled: true,
      model: "glm-5.3-flash",
    },
  });
  renderAssistant();
  await new Promise((resolve) => setTimeout(resolve, 20));
  expect(mocks.status).not.toHaveBeenCalled();
  expect(
    screen.queryByRole("button", { name: "Open agent" }),
  ).not.toBeInTheDocument();
});

it("stays hidden while the workspace switch is off", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: { enabled: false, configured: true, model: "glm-5.3-flash" },
  });
  renderAssistant();
  await waitFor(() => expect(mocks.status).toHaveBeenCalled());
  expect(
    screen.queryByRole("button", { name: "Open agent" }),
  ).not.toBeInTheDocument();
});

it("opens the floating panel and returns to the icon only when closed", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: true,
      configured: true,
      agentEnabled: true,
      model: "glm-5.3-flash",
    },
  });
  renderAssistant();
  const user = userEvent.setup();
  const trigger = await screen.findByRole("button", { name: "Open agent" });
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

  await user.click(trigger);
  expect(
    screen.getByRole("dialog", { name: "Workspace agent" }),
  ).toBeInTheDocument();
  expect(screen.getByText("glm-5.3-flash")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Close agent" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Open agent" }),
  ).toBeInTheDocument();
});

it("closes the panel with Escape", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: true,
      configured: true,
      agentEnabled: true,
      model: "glm-5.3-flash",
    },
  });
  renderAssistant();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Open agent" }));
  expect(screen.getByRole("dialog")).toBeInTheDocument();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it.each([false, true])(
  "returns the WebMCP tool result to the workspace agent (isError=%s)",
  async (isError) => {
    await changeLanguage("en");
    mocks.status.mockResolvedValue({
      status: 200,
      data: {
        enabled: true,
        configured: true,
        agentEnabled: true,
        model: "glm-5.3-flash",
      },
    });
    mocks.invoke.mockResolvedValue({
      isError,
      content: [{ type: "text", text: '{"route":"/accounts"}' }],
    });
    mocks.agent
      .mockResolvedValueOnce({
        status: 200,
        data: {
          content: "",
          model: "glm-5.3-flash",
          mode: "agent",
          tools: [],
          runId: 42,
          resumeToken: "once",
          pageCall: {
            id: "call_1",
            name: "baseline_page_context",
            arguments: "{}",
          },
        },
      })
      .mockResolvedValueOnce({
        status: 200,
        data: {
          content: "You are on the accounts page.",
          model: "glm-5.3-flash",
          mode: "agent",
          tools: [],
          runId: 42,
          resumeToken: "once",
        },
      });

    renderAssistant();
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Open agent" }));
    await user.type(
      screen.getByRole("textbox", { name: "Ask the workspace agent…" }),
      "Where am I?",
    );
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(
        screen.getByText("You are on the accounts page."),
      ).toBeInTheDocument(),
    );
    expect(mocks.invoke).toHaveBeenCalledWith(
      "baseline_page_context",
      {},
      undefined,
    );
    const first = mocks.agent.mock.calls[0][0];
    const second = mocks.agent.mock.calls[1][0];
    expect(second).toMatchObject({
      runId: 42,
      resumeToken: "once",
      toolCallId: "call_1",
      sessionId: first.sessionId,
    });
    expect(JSON.parse(second.toolResult)).toEqual({
      content: '{"route":"/accounts"}',
      isError,
    });
    expect(first.tools).toEqual([
      {
        name: "baseline_page_context",
        description: "Read current route",
        parameters: { type: "object", properties: {} },
      },
    ]);
    expect(mocks.chat).not.toHaveBeenCalled();
  },
);

it("uses the workspace agent directly without a page mode switch", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: {
      enabled: true,
      configured: true,
      agentEnabled: true,
      model: "glm-5.3-flash",
    },
  });
  mocks.agent.mockResolvedValue({
    status: 200,
    data: {
      content: "密码至少 12 位（账号规范）。",
      tools: ["knowledge_search"],
      model: "glm-5.3-flash",
      mode: "agent",
    },
  });
  renderAssistant();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Open agent" }));
  expect(
    screen.queryByRole("group", { name: "Answer mode" }),
  ).not.toBeInTheDocument();
  await user.type(
    screen.getByRole("textbox", { name: "Ask the workspace agent…" }),
    "密码要求是什么",
  );
  await user.click(screen.getByRole("button", { name: "Send" }));

  await waitFor(() =>
    expect(
      screen.getByText("密码至少 12 位（账号规范）。"),
    ).toBeInTheDocument(),
  );
  expect(mocks.agent).toHaveBeenCalledWith(
    expect.objectContaining({
      question: "密码要求是什么",
      sessionId: expect.any(String),
      tools: expect.any(Array),
    }),
    expect.objectContaining({ signal: expect.anything() }),
  );
  expect(mocks.chat).not.toHaveBeenCalled();
  expect(screen.getByText("Requested knowledge_search")).toBeInTheDocument();
});
