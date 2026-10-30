import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { changeLanguage } from "@/i18n";
import type { PageTool } from "@/lib/webmcp/runtime";
import { WebMcpProvider } from "./provider";
import { WebMcpSettingsPage } from "./settings";

const fixture = vi.hoisted(() => ({
  tools: new Map<string, PageTool>(),
  close: vi.fn(async () => {}),
  send: vi.fn(async () => {}),
  bind: vi.fn(async () => {}),
  install: vi.fn(async () => {}),
  failed: false,
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
async function show() {
  await changeLanguage("en", false);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/webmcp"]}>
        <WebMcpProvider>
          <WebMcpSettingsPage />
        </WebMcpProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}
beforeEach(() => {
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
  expect(
    screen.getByRole("heading", { name: "WebMCP 与 WebSkills" }),
  ).toBeVisible();
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
