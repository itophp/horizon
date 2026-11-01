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
  invoke: vi.fn(),
  permissions: ["assistant:use"] as string[],
}));

vi.mock("@/api/generated/baseline", () => ({
  assistantStatus: mocks.status,
  assistantChat: mocks.chat,
  assistantAgent: mocks.agent,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({ permissions: mocks.permissions }),
}));

vi.mock("@/features/webmcp/provider", () => ({
  useWebMcp: () => ({
    status: "disconnected",
    toolCount: 1,
    connection: {
      ensureLocalTools: async () => undefined,
      registry: {
        list: () => [
          {
            name: "baseline_page_context",
            description: "Read current route",
            inputSchema: { type: "object", properties: {} },
          },
        ],
        invoke: mocks.invoke,
      },
    },
  }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  mocks.permissions = ["assistant:use"];
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

it("stays hidden for accounts without the assistant grant", async () => {
  await changeLanguage("en");
  mocks.permissions = [];
  mocks.status.mockResolvedValue({
    status: 200,
    data: { enabled: true, configured: true, model: "glm-5.3-flash" },
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
    data: { enabled: true, configured: true, model: "glm-5.3-flash" },
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
    data: { enabled: true, configured: true, model: "glm-5.3-flash" },
  });
  renderAssistant();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Open agent" }));
  expect(screen.getByRole("dialog")).toBeInTheDocument();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("runs a requested page tool in the browser and returns the follow-up answer", async () => {
  await changeLanguage("en");
  mocks.status.mockResolvedValue({
    status: 200,
    data: { enabled: true, configured: true, model: "glm-5.3-flash" },
  });
  mocks.invoke.mockResolvedValue({
    content: [{ type: "text", text: '{"route":"/accounts"}' }],
  });
  mocks.chat
    .mockResolvedValueOnce({
      status: 200,
      data: {
        content: "",
        model: "glm-5.3-flash",
        toolCalls: [
          {
            id: "call_1",
            name: "baseline_page_context",
            arguments: "{}",
          },
        ],
      },
    })
    .mockResolvedValueOnce({
      status: 200,
      data: {
        content: "You are on the accounts page.",
        model: "glm-5.3-flash",
        toolCalls: [],
      },
    });

  renderAssistant();
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Open agent" }));
  await user.type(
    screen.getByRole("textbox", { name: "Ask about this page…" }),
    "Where am I?",
  );
  await user.click(screen.getByRole("button", { name: "Send" }));

  await waitFor(() =>
    expect(
      screen.getByText("You are on the accounts page."),
    ).toBeInTheDocument(),
  );
  expect(mocks.invoke).toHaveBeenCalledWith("baseline_page_context", {});
  const second = mocks.chat.mock.calls[1][0];
  expect(second.messages.at(-1)).toMatchObject({
    role: "tool",
    toolCallId: "call_1",
    content: '{"route":"/accounts"}',
  });
  expect(second.tools).toEqual([
    {
      name: "baseline_page_context",
      description: "Read current route",
      parameters: { type: "object", properties: {} },
    },
  ]);
});

it("routes the question to the Eino agent when the workspace agent mode is picked", async () => {
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
  await user.click(screen.getByRole("button", { name: "Workspace agent" }));
  await user.type(
    screen.getByRole("textbox", { name: "Ask about this page…" }),
    "密码要求是什么",
  );
  await user.click(screen.getByRole("button", { name: "Send" }));

  await waitFor(() =>
    expect(
      screen.getByText("密码至少 12 位（账号规范）。"),
    ).toBeInTheDocument(),
  );
  expect(mocks.agent).toHaveBeenCalledWith(
    { question: "密码要求是什么" },
    expect.objectContaining({ signal: expect.anything() }),
  );
  expect(mocks.chat).not.toHaveBeenCalled();
  expect(screen.getByText("Requested knowledge_search")).toBeInTheDocument();
});
