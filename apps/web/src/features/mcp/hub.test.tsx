import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { changeLanguage } from "@/i18n";
import type { PageTool } from "@/lib/webmcp/runtime";
import { WebMcpProvider } from "@/features/webmcp/provider";
import { McpHubPage } from "./hub";

const fixture = vi.hoisted(() => ({
  tools: new Map<string, PageTool>(),
  close: vi.fn(async () => {}),
  send: vi.fn(async () => {}),
  bind: vi.fn(async () => {}),
  install: vi.fn(async () => {}),
  failed: false,
}));
vi.mock("@/features/mcp/remote", () => ({
  RemoteMcpSection: () => <p>Remote MCP panel</p>,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({ permissions: [] }),
}));
vi.mock("@/lib/webmcp/connection", async (original) => ({
  ...(await original<typeof import("@/lib/webmcp/connection")>()),
  bindPage: fixture.bind,
  setupClient: fixture.install,
}));
vi.mock("@opentiny/next-sdk/core", () => ({
  initializeBuiltinWebMCP: () =>
    Object.defineProperty(document, "modelContext", {
      configurable: true,
      value: {
        registerTool: (tool: PageTool, { signal }: { signal: AbortSignal }) => {
          fixture.tools.set(tool.name, tool);
          signal.addEventListener("abort", () => {
            if (fixture.tools.get(tool.name) === tool)
              fixture.tools.delete(tool.name);
          });
        },
      },
    }),
  WebMcpClient: class {
    close = fixture.close;
    async connect() {
      if (fixture.failed) throw new Error("private server error");
      return { sessionId: "test-page", transport: { send: fixture.send } };
    }
  },
}));
async function show(entry = "/mcp?tab=page") {
  await changeLanguage("en", false);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[entry]}>
        <WebMcpProvider>
          <McpHubPage />
        </WebMcpProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
beforeEach(() => {
  sessionStorage.clear();
  fixture.tools.clear();
  fixture.bind.mockClear();
  fixture.install.mockClear();
  fixture.close.mockClear();
  fixture.send.mockClear();
  fixture.failed = false;
});
it("validates pairing, installs from the connected page and revokes captured tools on 401", async () => {
  await show();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Connect page" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("64-character");
  await changeLanguage("zh-CN", false);
  expect(screen.getByRole("heading", { name: "Agent 协作" })).toBeVisible();
  expect(screen.getByRole("alert")).toHaveTextContent("64 位配对密钥");
  await changeLanguage("en", false);

  await user.type(
    screen.getByLabelText("Pairing key from the local terminal"),
    "a".repeat(64),
  );
  await user.click(screen.getByRole("button", { name: "Connect page" }));
  await waitFor(() =>
    expect(screen.getByRole("status")).toHaveTextContent("Connected"),
  );
  expect(fixture.bind).toHaveBeenCalledWith(18793, "a".repeat(64), "test-page");
  expect(
    screen.getByLabelText("Pairing key from the local terminal"),
  ).toHaveValue("");
  const navigate = fixture.tools.get("baseline_navigate")!;
  expect((await navigate.execute({ path: "/accounts" })).isError).toBe(true);
  expect((await navigate.execute({ path: "https://evil.test" })).isError).toBe(
    true,
  );
  const context = await fixture.tools.get("baseline_page_context")!.execute({});
  expect(JSON.stringify(context)).not.toContain("/accounts");
  await navigate.execute({ path: "/" });
  await waitFor(() =>
    expect(fixture.send).toHaveBeenCalledWith({
      jsonrpc: "2.0",
      method: "notifications/tools/list_changed",
    }),
  );

  await user.click(
    screen.getAllByRole("button", { name: "Set up MCP + Skills" })[0],
  );
  expect(fixture.install).toHaveBeenCalledWith(
    18793,
    "a".repeat(64),
    "test-page",
    "codex",
  );
  expect(
    await screen.findByText(
      "MCP and three WebSkills installed. Restart the client to load them.",
    ),
  ).toBeVisible();
  window.dispatchEvent(new Event("baseline:unauthenticated"));
  await waitFor(() => expect(fixture.tools.size).toBe(0));
  expect((await navigate.execute({ path: "/" })).isError).toBe(true);
  expect(fixture.close).toHaveBeenCalled();
});
it("prefills the pairing form from the local dev service", async () => {
  const fetchMock = vi.fn(async () => ({
    ok: true,
    json: async () => ({ port: 18794, token: "c".repeat(64) }),
  }));
  vi.stubGlobal("fetch", fetchMock);
  try {
    await show();
    await waitFor(() =>
      expect(screen.getByLabelText("Local WebMCP port")).toHaveValue(18794),
    );
    expect(
      screen.getByText(
        "Port and pairing key were filled from the local service. Connect to continue.",
      ),
    ).toBeVisible();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Connect page" }));
    await waitFor(() =>
      expect(screen.getByRole("status")).toHaveTextContent("Connected"),
    );
    expect(fixture.bind).toHaveBeenCalledWith(
      18794,
      "c".repeat(64),
      "test-page",
    );
  } finally {
    vi.unstubAllGlobals();
  }
});
it("reports connection and installation failures without revealing exception content", async () => {
  fixture.failed = true;
  await show();
  const user = userEvent.setup();
  await user.type(
    screen.getByLabelText("Pairing key from the local terminal"),
    "b".repeat(64),
  );
  await user.click(screen.getByRole("button", { name: "Connect page" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Could not connect",
  );
  expect(screen.queryByText("private server error")).not.toBeInTheDocument();
  expect(fixture.tools.size).toBe(0);
  fixture.failed = false;
  await user.click(screen.getByRole("button", { name: "Connect page" }));
  await waitFor(() =>
    expect(screen.getByRole("status")).toHaveTextContent("Connected"),
  );
  fixture.install.mockRejectedValueOnce(new Error("private path"));
  await user.click(
    screen.getAllByRole("button", { name: "Set up MCP + Skills" })[1],
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Setup failed");
  expect(screen.queryByText("private path")).not.toBeInTheDocument();
});
it("switches between remote MCP, WebMCP and WebSkills tabs and follows the URL", async () => {
  await show("/mcp");
  const user = userEvent.setup();
  const tabs = screen.getByRole("tablist", { name: "MCP access modes" });
  expect(within(tabs).getAllByRole("tab")).toHaveLength(3);
  expect(screen.getByText("Remote MCP panel")).toBeVisible();
  expect(
    screen.getByRole("heading", { name: "Agent collaboration" }),
  ).toBeVisible();

  await user.click(screen.getByRole("tab", { name: "WebMCP" }));
  expect(
    await screen.findByRole("heading", { name: "Page connection" }),
  ).toBeVisible();
  expect(screen.getByRole("tab", { name: "WebMCP" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  await user.click(screen.getByRole("tab", { name: "WebSkills" }));
  expect(
    await screen.findByRole("heading", { name: "Page skills" }),
  ).toBeVisible();
  expect(screen.getByText("webskills/baseline-web-navigate")).toBeVisible();
  expect(screen.queryByText("Remote MCP panel")).not.toBeInTheDocument();
});

it("opens the WebSkills tab directly from a deep link and ignores unknown tabs", async () => {
  await show("/mcp?tab=skills");
  expect(
    await screen.findByRole("heading", { name: "Page skills" }),
  ).toBeVisible();
});

it("falls back to the remote MCP tab for unknown tab values", async () => {
  await show("/mcp?tab=unknown");
  expect(screen.getByText("Remote MCP panel")).toBeVisible();
});
