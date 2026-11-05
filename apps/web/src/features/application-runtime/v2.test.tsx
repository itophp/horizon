import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { ApplicationPage } from "./page";
import { evaluate, mapInput, validateInput } from "./expression";
import {
  componentRegistry,
  extensions,
  validateDefinition,
  semanticContext,
  type ViewContext,
  RegisteredControl,
} from "./v2-registry";
import type {
  RuntimeApplicationV2,
  RuntimeExpression,
} from "@/api/generated/runtime";
const api = vi.hoisted(() => ({
  application: vi.fn(),
  read: vi.fn(),
  providerRead: vi.fn(),
  runtimeReference: vi.fn(),
  execute: vi.fn(),
  identity: "a",
  permissions: [
    "projects:read",
    "projects:write",
    "projects:admin",
    "articles:read",
    "articles:write",
    "orchestration:run",
  ],
}));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeApplication: api.application,
  runtimeRead: (
    name: string,
    input: Record<string, unknown>,
    signal?: AbortSignal,
    version?: string,
  ) => {
    api.providerRead(name, input, signal, version);
    if (name === "application.runtime.data.read") {
      const provider = fixtures
        .find((app) => app.id === input.applicationId)
        ?.providers.find((provider) => provider.id === input.providerId);
      if (!provider) throw new Error("schema_invalid");
      return Promise.resolve(
        api.read(
          provider.capability.name,
          input.input,
          signal,
          provider.capability.version,
        ),
      ).then((value: unknown) => {
        if (
          provider.select?.length &&
          value &&
          typeof value === "object" &&
          "items" in value &&
          Array.isArray(value.items)
        )
          return {
            ...value,
            items: value.items.map((row: Record<string, unknown>) =>
              Object.fromEntries(
                [provider.keyField, ...provider.select!].map((key) => [
                  key,
                  row[key],
                ]),
              ),
            ),
          };
        return value;
      });
    }
    return api.read(name, input, signal, version);
  },
  runtimeExecute: (
    name: string,
    version: string,
    input: Record<string, unknown>,
    key: string,
    correlation: string,
    runtime?: unknown,
  ) => {
    api.runtimeReference(runtime);
    return api.execute(name, version, input, key, correlation);
  },
}));
vi.mock("./task-panel", () => ({
  TaskPanel: ({ runId }: { runId: number }) => (
    <div data-testid="runtime-task">{runId}</div>
  ),
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: api.identity,
    account: { id: 1 },
    activeDepartmentId: 1,
    permissions: api.permissions,
  }),
}));
vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));
const fixtures = JSON.parse(
  readFileSync(
    resolve(process.cwd(), "../../docs/generated/runtime/applications.json"),
    "utf8",
  ),
) as RuntimeApplicationV2[];
extensions.register({
  id: "project.related",
  source: "com.horizon.projects",
  version: "1.0.0",
  type: "component",
  render: () => null,
});
const project = {
  id: 1,
  code: "P1",
  name: "Sample",
  status: "active",
  priority: "normal",
  dueDate: "",
  ownerName: "Member",
  ownerId: 1,
  taskCount: 0,
};
function mount(app = "projects", page = "project.list", path = "/projects") {
  const element = <ApplicationPage applicationId={app} pageId={page} />;
  const router = createMemoryRouter(
    [
      { path: "/projects", element },
      {
        path: "/projects/:id",
        element: (
          <ApplicationPage
            applicationId="projects"
            pageId={page === "project.list" ? "project.detail" : page}
          />
        ),
      },
      {
        path: "/articles/:id",
        element: (
          <ApplicationPage applicationId="articles" pageId="article.detail" />
        ),
      },
    ],
    { initialEntries: [path] },
  );
  const view = render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({
            defaultOptions: {
              queries: { retry: false },
              mutations: { retry: false },
            },
          })
        }
      >
        <RouterProvider router={router} />
      </QueryClientProvider>
    </I18nextProvider>,
  );
  return { ...view, router };
}
beforeEach(async () => {
  vi.clearAllMocks();
  api.identity = "a";
  await changeLanguage("en", false);
  api.application.mockImplementation(async (id: string) =>
    structuredClone(fixtures.find((a) => a.id === id)),
  );
  api.read.mockImplementation(async (name: string) =>
    name === "project.list"
      ? { items: [project], page: 1, pageSize: 20, total: 45 }
      : name === "project.get"
        ? project
        : name === "article.get"
          ? {
              id: 1,
              title: "Draft",
              body: "Sensitive body",
              status: "draft",
              revision: 4,
              reviewCode: "",
            }
          : { items: [] },
  );
  api.execute.mockResolvedValue({
    status: "succeeded",
    invocationId: "i",
    traceId: "t",
    data: { id: 1 },
  });
});
it("uses a second application's declared fields and requests server pagination", async () => {
  const user = userEvent.setup();
  mount();
  await screen.findByRole("link", { name: "Sample" });
  await user.click(screen.getByRole("button", { name: "Next page" }));
  await waitFor(() =>
    expect(api.read).toHaveBeenLastCalledWith(
      "project.list",
      expect.objectContaining({ page: 2, pageSize: 20 }),
      expect.any(AbortSignal),
      "1.0.0",
    ),
  );
  expect(api.providerRead).toHaveBeenCalledWith(
    "application.runtime.data.read",
    expect.objectContaining({
      applicationId: "projects",
      pageId: "project.list",
      providerId: "projects",
      input: expect.objectContaining({ page: 2 }),
    }),
    expect.any(AbortSignal),
    "1.0.0",
  );
  expect(screen.queryByLabelText("Title")).not.toBeInTheDocument();
});
it("creates a project from its own schema and omits article revision", async () => {
  const user = userEvent.setup();
  mount();
  await user.click(await screen.findByRole("button", { name: "New project" }));
  await user.type(screen.getByLabelText("Code"), "P2");
  await user.type(screen.getByLabelText("Name"), "Next");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(api.execute).toHaveBeenCalledWith(
      "project.create",
      "1.0.0",
      { code: "P2", name: "Next", priority: "normal", dueDate: "" },
      expect.any(String),
      expect.any(String),
    ),
  );
});
it("edits without a transition or revision and honors a missing create binding", async () => {
  api.application.mockImplementation(async (id: string) => {
    const app = structuredClone(fixtures.find((a) => a.id === id)!);
    for (const p of app.pages) {
      p.actions = p.actions.filter((a) => a.id !== "project.new");
      for (const f of p.forms) f.create.capability.name = "";
    }
    return app;
  });
  const user = userEvent.setup();
  mount("projects", "project.detail", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  expect(screen.queryByLabelText("Code")).not.toBeInTheDocument();
  const input = screen.getByLabelText("Name");
  await user.clear(input);
  await user.type(input, "Updated");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(api.execute).toHaveBeenCalledWith(
      "project.update",
      "1.0.0",
      { id: 1, name: "Updated", priority: "normal", dueDate: "" },
      expect.any(String),
      expect.any(String),
    ),
  );
});
it("distinguishes workflow acceptance from business completion", async () => {
  api.execute.mockResolvedValue({ status: "succeeded", data: { runId: 42 } });
  const user = userEvent.setup();
  mount("articles", "article.detail", "/articles/1");
  await user.click(
    await screen.findByRole("button", { name: "Submit and start review" }),
  );
  await screen.findByText("Workflow started #42");
  expect(api.execute).toHaveBeenCalledWith(
    "orchestration.workflow.run",
    "1.0.0",
    {
      flow: "article-publish",
      version: "1.0.0",
      input: { id: 1, revision: 4 },
    },
    expect.any(String),
    expect.any(String),
  );
});
it("keeps a form open while awaiting approval", async () => {
  api.execute.mockResolvedValue({
    status: "awaiting_approval",
    approvalId: 12,
  });
  const user = userEvent.setup();
  mount("projects", "project.detail", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByText("Awaiting approval #12");
  expect(screen.getByLabelText("Name")).toBeInTheDocument();
});
it("protects dirty drafts on route changes", async () => {
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  const user = userEvent.setup();
  const { router } = mount();
  await user.click(await screen.findByRole("button", { name: "New project" }));
  await user.type(screen.getByLabelText("Name"), "Unsaved");
  await router.navigate("/projects/1");
  await waitFor(() => expect(confirm).toHaveBeenCalled());
  expect(screen.getByLabelText("Name")).toHaveValue("Unsaved");
  confirm.mockRestore();
});
it("rejects prototype access, unknown versions, components and wrong command types", () => {
  const expression = {
    kind: "path",
    path: "record.__proto__.secret",
    literal: "",
    items: [],
    fields: [],
  } as RuntimeExpression;
  expect(() => evaluate(expression, { record: { id: 1 } })).toThrow();
  expect(() =>
    validateInput(
      { id: "1" },
      '{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"]}',
    ),
  ).toThrow();
  const app = structuredClone(fixtures[0]);
  app.runtimeCompatibility = "99";
  expect(() => validateDefinition(app)).toThrow();
  expect(() =>
    componentRegistry.resolve({
      ...fixtures[0].pages[0].table.columns[0],
      component: "script",
    }),
  ).toThrow();
  expect(() =>
    mapInput([{ target: "constructor", value: expression }], {}),
  ).toThrow();
});
it("exports semantic references without body or drafts", () => {
  const app = fixtures[0];
  const ctx = {
    application: app,
    page: app.pages[1],
    scope: {
      record: { id: 1, status: "draft", body: "secret" },
      form: { body: "draft" },
    },
    selection: [],
    identity: "a",
    setSelection: vi.fn(),
    correlationId: "c",
    readOnly: false,
  } as ViewContext;
  const raw = JSON.stringify(semanticContext(ctx));
  expect(raw).not.toContain("secret");
  expect(raw).not.toContain('"body"');
  expect(raw).not.toContain("draft");
  expect(raw).toContain("article.submit");
});

it("loads controlled member references and keeps selected IDs numeric", async () => {
  api.read.mockImplementation(async (name: string) =>
    name === "project.task.list"
      ? { items: [], total: 0 }
      : name === "project.member.list"
        ? { items: [{ accountId: 9, name: "Allowed member", isOwner: true }] }
        : { items: [] },
  );
  const user = userEvent.setup();
  mount("projects", "project.tasks", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "New task" }));
  await user.type(screen.getByLabelText("Task title"), "Typed task");
  await screen.findByRole("option", { name: "Allowed member" });
  await user.selectOptions(
    await screen.findByRole("combobox", { name: "Assignee" }),
    "9",
  );
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(api.execute).toHaveBeenCalledWith(
      "project.task.create",
      "1.0.0",
      expect.objectContaining({
        projectId: 1,
        title: "Typed task",
        planId: 0,
        assigneeId: 9,
      }),
      expect.any(String),
      expect.any(String),
    ),
  );
  expect(api.read).toHaveBeenCalledWith(
    "project.member.list",
    { projectId: 1, keyword: "" },
    expect.any(AbortSignal),
    "1.0.0",
  );
});
it("shows per-item batch outcomes and retries only the failed subset", async () => {
  api.read.mockImplementation(async (name: string) =>
    name === "project.task.list"
      ? {
          items: [
            {
              id: 11,
              projectId: 1,
              priority: "normal",
              title: "First",
              status: "todo",
              assigneeName: "",
            },
            {
              id: 12,
              projectId: 1,
              priority: "normal",
              title: "Second",
              status: "todo",
              assigneeName: "",
            },
          ],
          total: 2,
        }
      : name === "project.member.list"
        ? { items: [{ accountId: 9, name: "Allowed member", isOwner: true }] }
        : { items: [] },
  );
  api.execute
    .mockResolvedValueOnce({
      status: "succeeded",
      data: {
        items: [
          { id: 11, status: "succeeded" },
          { id: 12, status: "failed" },
        ],
      },
    })
    .mockResolvedValue({
      status: "succeeded",
      data: { items: [{ id: 12, status: "succeeded" }] },
    });
  const user = userEvent.setup();
  mount("projects", "project.tasks", "/projects/1");
  await user.click(
    await screen.findByRole("checkbox", { name: "Select all on this page" }),
  );
  await user.click(
    screen.getByRole("button", { name: "Assign selected tasks" }),
  );
  await screen.findByRole("option", { name: "Allowed member" });
  await user.selectOptions(
    await screen.findByRole("combobox", { name: "Assignee" }),
    "9",
  );
  await user.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByText("1 succeeded; 1 failed or not executed.");
  expect(screen.getByText("12: Failed or not executed")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Retry failed items" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalledTimes(2));
  expect(api.execute.mock.calls[0][2].ids).toEqual([11, 12]);
  expect(api.execute.mock.calls[1][2].ids).toEqual([12]);
  expect(api.execute.mock.calls[0][2].operationId).not.toBe(
    api.execute.mock.calls[1][2].operationId,
  );
});
it("uses the same attempt key for unchanged retries and a new key after editing", async () => {
  api.execute.mockRejectedValue(new Error("network"));
  const user = userEvent.setup();
  mount("projects", "project.detail", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalledTimes(1));
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalledTimes(2));
  expect(api.execute.mock.calls[0][3]).toBe(api.execute.mock.calls[1][3]);
  await user.type(screen.getByLabelText("Name"), " changed");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalledTimes(3));
  expect(api.execute.mock.calls[1][3]).not.toBe(api.execute.mock.calls[2][3]);
});
it("confirms a schema action before dispatching", async () => {
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  const user = userEvent.setup();
  mount("projects", "project.detail", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "Archive" }));
  expect(api.execute).not.toHaveBeenCalled();
  confirm.mockReturnValue(true);
  await user.click(screen.getByRole("button", { name: "Archive" }));
  await waitFor(() =>
    expect(api.execute).toHaveBeenCalledWith(
      "project.status.change",
      "1.0.0",
      { id: 1, from: "active", status: "archived" },
      expect.any(String),
      expect.any(String),
    ),
  );
  confirm.mockRestore();
});
it("keeps boolean and integer component values typed", async () => {
  const boolean = vi.fn();
  const integer = vi.fn();
  const base = fixtures[1].pages[0].forms[0].create.fields[0];
  render(
    <I18nextProvider i18n={i18n}>
      <RegisteredControl
        definition={{ ...base, type: "boolean", component: "boolean" }}
        value={false}
        onChange={boolean}
        onBlur={vi.fn()}
        name="flag"
        id="flag"
        disabled={false}
      />
      <RegisteredControl
        definition={{ ...base, type: "integer", component: "integer" }}
        value={0}
        onChange={integer}
        onBlur={vi.fn()}
        name="count"
        id="count"
        disabled={false}
      />
    </I18nextProvider>,
  );
  const user = userEvent.setup();
  await user.click(screen.getByRole("checkbox"));
  expect(boolean).toHaveBeenCalledWith(true);
  await user.clear(screen.getByRole("spinbutton"));
  await user.type(screen.getByRole("spinbutton"), "7");
  expect(integer).toHaveBeenLastCalledWith(7);
});
it("cancels old reads and prevents an old identity's late result from appearing", async () => {
  let finish: ((value: unknown) => void) | undefined;
  let oldSignal: AbortSignal | undefined;
  api.read.mockImplementationOnce(
    (_name: string, _input: unknown, signal: AbortSignal) => {
      oldSignal = signal;
      return new Promise((resolve) => {
        finish = resolve;
      });
    },
  );
  const first = mount();
  await waitFor(() => expect(api.read).toHaveBeenCalled());
  first.unmount();
  expect(oldSignal?.aborted).toBe(true);
  api.identity = "b";
  api.read.mockResolvedValue({
    items: [{ ...project, id: 2, name: "New identity" }],
    total: 1,
  });
  mount();
  await screen.findByRole("link", { name: "New identity" });
  finish?.({ items: [{ ...project, name: "Old private value" }], total: 1 });
  await waitFor(() =>
    expect(screen.queryByText("Old private value")).not.toBeInTheDocument(),
  );
});

it("executes a declared custom action adapter through the fixed Gateway binding", async () => {
  const id = "test.action.normalizer";
  extensions.register({
    id,
    source: "com.horizon.projects",
    version: "1.0.0",
    type: "action",
    normalize: (input) => ({ ...input, status: "paused" }),
  });
  api.application.mockImplementation(async (name: string) => {
    const app = structuredClone(fixtures.find((a) => a.id === name)!);
    const detail = app.pages.find((p) => p.id === "project.detail")!;
    const action = structuredClone(
      detail.actions.find((a) => a.id === "project.status.paused")!,
    );
    action.type = "custom";
    action.target = id;
    action.id = "test.custom";
    action.labelKey = "runtime:yes";
    detail.actions = [action];
    app.extensions.push({
      id,
      source: app.source,
      version: "1.0.0",
      type: "action",
      entry: id,
      runtimeCompatibility: "2.0",
      permissions: ["projects:write"],
      dependencies: [],
    });
    return app;
  });
  const user = userEvent.setup();
  mount("projects", "project.detail", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "Yes" }));
  await waitFor(() =>
    expect(api.execute).toHaveBeenCalledWith(
      "project.status.change",
      "1.0.0",
      { id: 1, from: "active", status: "paused" },
      expect.any(String),
      expect.any(String),
    ),
  );
});
it("applies a trusted Provider adapter and validates its output arguments", async () => {
  const id = "test.provider.normalizer";
  extensions.register({
    id,
    source: "com.horizon.projects",
    version: "1.0.0",
    type: "provider",
    normalize: (input) => ({ ...input, id: 2 }),
  });
  api.application.mockImplementation(async (name: string) => {
    const app = structuredClone(fixtures.find((a) => a.id === name)!);
    app.providers.find((p) => p.id === "project")!.adapter = id;
    app.extensions.push({
      id,
      source: app.source,
      version: "1.0.0",
      type: "provider",
      entry: id,
      runtimeCompatibility: "2.0",
      permissions: ["projects:read"],
      dependencies: [],
    });
    return app;
  });
  mount("projects", "project.detail", "/projects/1");
  await screen.findByRole("button", { name: "Edit" });
  expect(api.read).toHaveBeenCalledWith(
    "project.get",
    { id: 2 },
    expect.any(AbortSignal),
    "1.0.0",
  );
});
it("shows a safe trace reference for business failure", async () => {
  api.execute.mockResolvedValue({
    status: "failed",
    errorCode: "forbidden",
    traceId: "safe-trace",
    invocationId: "safe-invocation",
  });
  const user = userEvent.setup();
  mount("projects", "project.detail", "/projects/1");
  await user.click(await screen.findByRole("button", { name: "Edit" }));
  await user.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByText("Reference: safe-trace");
  expect(screen.getByLabelText("Name")).toBeInTheDocument();
});

it("renders an explicit standalone form page through the shared typed command", async () => {
  const app = structuredClone(fixtures.find((app) => app.id === "projects")!);
  const page = app.pages.find((page) => page.id === "project.list")!;
  page.type = "form";
  page.actions = page.actions.filter((action) => action.id === "project.new");
  api.application.mockResolvedValue(app);
  mount();
  const user = userEvent.setup();
  await user.type(await screen.findByLabelText("Code"), "P2");
  await user.type(screen.getByLabelText("Name"), "Standalone");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(api.execute).toHaveBeenCalledWith(
      "project.create",
      "1.0.0",
      expect.objectContaining({ code: "P2", name: "Standalone" }),
      expect.any(String),
      expect.any(String),
    ),
  );
});
