import { readFileSync } from "node:fs";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { i18n, changeLanguage } from "@/i18n";
import { ComponentRegistry } from "./component-registry";
import { DynamicApplicationRoute } from "../route";
import { ProtocolPage } from "./protocol-page";
import type {
  ApplicationV3,
  Expression,
  PageV3,
  ProtocolProvider,
} from "./types";

const api = vi.hoisted(() => ({
  application: vi.fn(),
  read: vi.fn(),
  execute: vi.fn(),
}));

vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeApplication: api.application,
  runtimeRead: api.read,
  runtimeExecute: api.execute,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant",
    account: {
      id: 7,
      memberships: [
        { departmentId: 3, departmentName: "Engineering" },
        { departmentId: 4, departmentName: "Operations" },
      ],
    },
    activeDepartmentId: 3,
    permissions: ["runtime.protocol.view", "accounts:read", "projects:write"],
  }),
}));

vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));

function expression(value: Partial<Expression>): Expression {
  return { kind: "", path: "", literal: "", items: [], fields: [], ...value };
}

const accountProvider: ProtocolProvider = {
  adapter: "capability",
  outputSchema: "",
  permission: "",
  inputSchema: "",
  id: "account.provider",
  capability: { name: "test.account.list", version: "1.0.1" },
  input: [],
  resultPath: "items",
  keyField: "id",
  labelField: "name",
  pagination: "none",
  pageSize: 0,
  pollInterval: 0,
  tags: [],
};

const page: PageV3 = {
  schemaVersion: "3.0",
  id: "protocol.list",
  route: "/protocol",
  titleKey: "runtime:total",
  permission: "runtime.protocol.view",
  root: {
    type: "layout",
    layout: {
      id: "root",
      kind: "stack",
      props: { gap: "16px" },
      data: [
        {
          id: "_search",
          provider: "",
          initial: "",
        },
      ],
      children: [
        {
          type: "layout",
          layout: {
            id: "toolbar",
            kind: "grid",
            props: { columns: "260px" },
            children: [
              {
                type: "component",
                component: {
                  id: "search",
                  kind: "search",
                  version: "1.0",
                  props: {},
                  data: "$_search",
                  events: [
                    {
                      event: "change",
                      action: "setData",
                      target: "_search",
                    },
                  ],
                },
              },
            ],
          },
        },
        {
          type: "layout",
          layout: {
            id: "actions",
            kind: "grid",
            props: { columns: "auto" },
            children: [
              {
                type: "component",
                component: {
                  id: "refresh",
                  kind: "button",
                  version: "1.0",
                  props: {
                    labelKey: "runtime:refresh",
                    payload: { name: "protocol-demo" },
                  },
                  events: [
                    {
                      event: "click",
                      action: "invoke",
                      target: "test.action@1.2",
                      payload: expression({
                        kind: "object",
                        fields: [
                          {
                            target: "name",
                            value: expression({
                              kind: "path",
                              path: "event.name",
                            }),
                          },
                        ],
                      }),
                    },
                  ],
                },
              },
            ],
          },
        },
        {
          type: "layout",
          layout: {
            id: "table-panel",
            kind: "grid",
            props: {},
            children: [
              {
                type: "component",
                component: {
                  id: "card",
                  kind: "card",
                  version: "1.0",
                  props: {},
                  content: {
                    id: "table-content",
                    kind: "grid",
                    props: {},
                    data: [
                      {
                        id: "accounts",
                        provider: "account.provider",
                        input: [
                          {
                            target: "search",
                            value: expression({
                              kind: "path",
                              path: "data._search",
                            }),
                          },
                        ],
                      },
                    ],
                    children: [
                      {
                        type: "component",
                        component: {
                          id: "table",
                          kind: "data-table",
                          version: "1.0",
                          props: {
                            columns: [
                              { name: "name", labelKey: "runtime:total" },
                            ],
                          },
                          data: "$accounts",
                        },
                      },
                    ],
                  },
                },
              },
            ],
          },
        },
      ],
    },
  },
};

const deploymentApplications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as Array<{ id: string; protocol?: ApplicationV3 }>;

const application: ApplicationV3 = {
  schemaVersion: "3.0",
  id: "protocol-demo",
  version: "1.0.0",
  source: "tests.protocol",
  runtimeCompatibility: "3.0",
  labelKey: "runtime:applications",
  permission: "runtime.protocol.view",
  route: "/protocol",
  pages: [page],
  providers: [
    {
      ...accountProvider,
      resultPath: "items",
    },
  ],
};

function mount(readOnly = false, renderedPage = page) {
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({
            defaultOptions: { queries: { retry: false } },
          })
        }
      >
        <MemoryRouter initialEntries={["/protocol"]}>
          <ProtocolPage
            application={application}
            page={renderedPage}
            allowed
            readOnly={readOnly}
          />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  vi.clearAllMocks();
  await changeLanguage("en", false);
  const fixture = deploymentApplications.find(
    (item) => item.id === "runtime-protocol",
  );
  api.application.mockResolvedValue(fixture);
  api.read.mockImplementation(async (name: string) =>
    name === "application.runtime.list"
      ? { items: [fixture] }
      : name === "identity.accounts.admin.read"
        ? {
            items: [
              {
                id: 1,
                fullName: "Alice Chen",
                username: "alice",
                email: "alice@example.test",
                status: "active",
                createdAt: "2026-01-01T00:00:00Z",
                memberships: [],
              },
            ],
            organizations: [],
            departments: [],
            roles: [],
            catalog: [],
            total: 1,
          }
        : { items: [{ id: 1, name: "Alice" }] },
  );
  api.execute.mockResolvedValue({ status: "succeeded" });
});

it("loads scoped provider data and renders it through registered components", async () => {
  mount();
  expect(await screen.findByText("Alice")).toBeInTheDocument();
  expect(api.read).toHaveBeenCalledWith(
    "test.account.list",
    { search: "" },
    expect.any(AbortSignal),
    "1.0.1",
  );
  const table = screen.getByRole("table");
  const layout = table.closest('[data-layout-id="table-panel"]');
  expect(layout).not.toHaveClass("admin-panel");
  expect(table.closest("section.admin-panel")?.parentElement).toBe(layout);
});

it("binds component events to protocol data variables", async () => {
  const user = userEvent.setup();
  mount();
  const input = await screen.findByRole("textbox");
  await user.type(input, "Ali");
  expect(input).toHaveValue("Ali");
});

it("invokes declared capabilities through protocol event bindings", async () => {
  const user = userEvent.setup();
  mount();
  await user.click(await screen.findByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalled());
  expect(api.execute).toHaveBeenCalledWith(
    "test.action",
    "1.2",
    { name: "protocol-demo" },
    expect.any(String),
    expect.any(String),
    undefined,
  );
});

it("shows failed protocol actions in a dialog instead of the page top", async () => {
  const user = userEvent.setup();
  api.execute.mockResolvedValue({ status: "failed", errorCode: "conflict" });
  mount();
  await user.click(await screen.findByRole("button", { name: "Refresh" }));
  const dialog = await screen.findByRole("dialog");
  expect(dialog).toHaveTextContent("Action failed");
  expect(dialog).toHaveTextContent(
    "The data changed or conflicts with an existing record. Refresh and try again.",
  );
  expect(document.querySelector(".protocol-page > .admin-error")).toBeNull();
});
it("routes the deployment catalog through the V3 protocol renderer", async () => {
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({
            defaultOptions: { queries: { retry: false } },
          })
        }
      >
        <MemoryRouter initialEntries={["/runtime-protocol"]}>
          <DynamicApplicationRoute />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );

  expect(await screen.findByText("Alice Chen")).toBeInTheDocument();
  expect(
    screen.getByRole("heading", { name: "Runtime protocol preview" }),
  ).toBeInTheDocument();
  expect(api.read).toHaveBeenCalledWith(
    "identity.accounts.admin.read",
    {},
    expect.any(AbortSignal),
    "1.0.0",
  );
});

it("rejects duplicate component registrations deterministically", () => {
  const registry = new ComponentRegistry();
  registry.register("text", "1.0", () => null);
  expect(() => registry.register("text", "1.0", () => null)).toThrowError(
    "component_already_registered: text@1.0",
  );
});

it("blocks configured action bindings in read-only previews", async () => {
  mount(true);
  const action = await screen.findByRole("button", { name: "Refresh" });
  expect(action).toBeDisabled();
  await userEvent.click(action);
  expect(api.execute).not.toHaveBeenCalled();
});

function dialogPage(initialOpen = false): PageV3 {
  const literal = (value: boolean) =>
    expression({ kind: "literal", literal: String(value) });
  return {
    ...page,
    root: {
      type: "layout",
      layout: {
        id: "dialog-root",
        kind: "grid",
        props: {},
        data: [{ id: "_dialogOpen", provider: "", initial: initialOpen }],
        children: [
          {
            type: "layout",
            layout: {
              id: "trigger-layout",
              kind: "grid",
              props: {},
              children: [
                {
                  type: "component",
                  component: {
                    id: "trigger",
                    kind: "button",
                    version: "1.0",
                    props: { labelKey: "runtime:design.previewLoad" },
                    events: [
                      {
                        event: "click",
                        action: "setData",
                        target: "_dialogOpen",
                        payload: literal(true),
                      },
                    ],
                  },
                },
              ],
            },
          },
          {
            type: "layout",
            layout: {
              id: "dialog-layout",
              kind: "grid",
              props: {},
              children: [
                {
                  type: "component",
                  component: {
                    id: "preview-dialog",
                    kind: "dialog",
                    version: "1.0",
                    props: { titleKey: "runtime:design.preview" },
                    bindings: [
                      {
                        target: "open",
                        value: expression({
                          kind: "path",
                          path: "data._dialogOpen",
                        }),
                      },
                    ],
                    events: [
                      {
                        event: "close",
                        action: "setData",
                        target: "_dialogOpen",
                        payload: literal(false),
                      },
                    ],
                    content: {
                      id: "dialog-body",
                      kind: "grid",
                      props: {},
                      children: [
                        {
                          type: "component",
                          component: {
                            id: "description",
                            kind: "text",
                            version: "1.0",
                            props: { textKey: "runtime:design.previewHint" },
                          },
                        },
                      ],
                    },
                  },
                },
              ],
            },
          },
        ],
      },
    },
  };
}

it("opens a declared dialog, renders its content slot, and closes through the close binding", async () => {
  mount(false, dialogPage());
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Load authorized preview" }),
  );
  expect(
    await screen.findByRole("dialog", { name: "Read-only preview" }),
  ).toBeVisible();
  expect(
    screen.getByText(
      "Read providers may load authorized data. Business writes are disabled.",
    ),
  ).toBeVisible();
  await user.keyboard("{Escape}");
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
});

it("keeps dialog content unmounted in read-only preview", () => {
  mount(true, dialogPage(true));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Load authorized preview" }),
  ).toBeDisabled();
});

function disclosurePage(): PageV3 {
  const trigger = dialogPage().root.layout!.children![0];
  const component = trigger.layout!.children![0].component!;
  component.events = [
    {
      event: "click",
      action: "invoke",
      target: "mcp.token.create@1.0.0",
      disclosure: {
        field: "secret",
        titleKey: "mcp:secretOnce",
        copyLabelKey: "mcp:copyToken",
        dismissKey: "mcp:dismiss",
        copySuccessKey: "mcp:tokenCopied",
        copyErrorKey: "mcp:copyTokenFailed",
      },
    },
  ];
  return {
    ...page,
    root: {
      type: "layout",
      layout: { id: "disclose", kind: "grid", props: {}, children: [trigger] },
    },
  };
}

it("shows a fresh credential only in the disclosure dialog and clears it on dismiss", async () => {
  api.execute.mockResolvedValue({
    status: "succeeded",
    data: { secret: "fixture-one-time-secret" },
  });
  mount(false, disclosurePage());
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Load authorized preview" }),
  );
  expect(await screen.findByText("fixture-one-time-secret")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Copy token" }));
  expect(await navigator.clipboard.readText()).toBe("fixture-one-time-secret");
  await user.click(
    screen.getByRole("button", { name: "I have saved the token" }),
  );
  await waitFor(() =>
    expect(
      screen.queryByText("fixture-one-time-secret"),
    ).not.toBeInTheDocument(),
  );
});

it("discards result data if an invocation response is marked as replayed", async () => {
  api.execute.mockResolvedValue({
    status: "succeeded",
    replayed: true,
    data: { secret: "fixture-replayed-secret" },
  });
  mount(false, disclosurePage());
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Load authorized preview" }),
  );
  expect(api.execute).toHaveBeenCalled();
  expect(screen.queryByText("fixture-replayed-secret")).not.toBeInTheDocument();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("submits an action form without retaining its password in the page retry state", async () => {
  const formPage = disclosurePage();
  const form =
    formPage.root.layout!.children![0].layout!.children![0].component!;
  form.kind = "action-form";
  form.props = {
    fields: [
      {
        name: "password",
        type: "password",
        labelKey: "webmcp:pairingKey",
        required: true,
      },
    ],
    submitKey: "mcp:addToken",
  };
  form.events![0].event = "submit";
  form.events![0].idempotencyKey = expression({
    kind: "path",
    path: "event.operationKey",
  });
  form.events![0].payload = expression({ kind: "path", path: "event.values" });
  api.execute
    .mockResolvedValueOnce({ status: "failed", errorCode: "conflict" })
    .mockResolvedValueOnce({
      status: "succeeded",
      data: { secret: "fixture-action-form-secret" },
    });
  mount(false, formPage);
  const user = userEvent.setup();
  const password = screen.getByLabelText("Pairing key from the local terminal");
  const submit = screen.getByRole("button", { name: "Add token" });
  await user.click(submit);
  expect(api.execute).not.toHaveBeenCalled();
  await user.type(password, "fixture-input");
  await user.click(submit);
  const error = await screen.findByRole("dialog");
  expect(
    within(error).queryByRole("button", { name: "Retry" }),
  ).not.toBeInTheDocument();
  await user.keyboard("{Escape}");
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  await user.click(submit);
  expect(await screen.findByText("fixture-action-form-secret")).toBeVisible();
  expect(api.execute.mock.calls[0][2]).toEqual({ password: "fixture-input" });
  expect(api.execute.mock.calls[0][3]).toBe(api.execute.mock.calls[1][3]);
  expect(password).toHaveValue("");
});

it("resolves a bound dialog title and projects the current account's department options", async () => {
  const candidate = dialogPage();
  const modal =
    candidate.root.layout!.children![1].layout!.children![0].component!;
  modal.props.titleKey = "mcp:rotateTitle";
  modal.bindings!.push({
    target: "titleArgs",
    value: expression({
      kind: "literal",
      literal: JSON.stringify({ name: "Deployment" }),
    }),
  });
  const form = modal.content!.children![0].component!;
  form.kind = "action-form";
  form.props = { submitKey: "mcp:create" };
  form.bindings = [
    {
      target: "fields",
      value: expression({
        kind: "array",
        items: [
          expression({
            kind: "object",
            fields: [
              {
                target: "name",
                value: expression({
                  kind: "literal",
                  literal: '"departmentId"',
                }),
              },
              {
                target: "type",
                value: expression({ kind: "literal", literal: '"select"' }),
              },
              {
                target: "labelKey",
                value: expression({
                  kind: "literal",
                  literal: '"mcp:department"',
                }),
              },
              {
                target: "options",
                value: expression({
                  kind: "path",
                  path: "context.departmentOptions",
                }),
              },
              {
                target: "defaultValue",
                value: expression({
                  kind: "path",
                  path: "context.activeDepartmentId",
                }),
              },
            ],
          }),
        ],
      }),
    },
  ];
  form.events = [
    {
      event: "submit",
      action: "invoke",
      target: "mcp.token.create@1.0.0",
      payload: expression({ kind: "path", path: "event.values" }),
      inputSchema: JSON.stringify({
        type: "object",
        properties: { departmentId: { type: "integer" } },
      }),
    },
  ];
  mount(false, candidate);
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Load authorized preview" }),
  );
  const dialog = await screen.findByRole("dialog", {
    name: "Rotate token: Deployment",
  });
  const field = within(dialog).getByRole("combobox", { name: "Department" });
  expect(field).toHaveValue("3");
  expect(
    within(field)
      .getAllByRole("option")
      .map((option) => option.textContent),
  ).toEqual(["Engineering", "Operations"]);
  await user.selectOptions(field, "4");
  await user.click(
    within(dialog).getByRole("button", { name: "Create token" }),
  );
  expect(api.execute).toHaveBeenCalledWith(
    "mcp.token.create",
    "1.0.0",
    { departmentId: 4 },
    expect.any(String),
    expect.any(String),
    undefined,
  );
});

it.each([
  { status: "succeeded", expected: "Next", absent: "Previous" },
  { status: "failed", expected: "Previous", absent: "Next" },
])(
  "shows action feedback for a $status capability",
  async ({ status, expected, absent }) => {
    const toastPage = structuredClone(page);
    const component =
      toastPage.root.layout?.children?.[1]?.layout?.children?.[0]?.component;
    if (!component) throw new Error("Missing refresh action fixture");
    component.kind = "toast";
    component.props = {
      labelKey: "runtime:refresh",
      titleKey: "runtime:components.notifications",
      mode: "action",
      successKey: "runtime:components.nextPage",
      errorKey: "runtime:components.previousPage",
    };
    api.execute.mockResolvedValue({ status, errorCode: "conflict" });
    mount(false, toastPage);
    await userEvent.click(
      await screen.findByRole("button", { name: "Refresh" }),
    );
    expect(await screen.findByText(expected)).toBeVisible();
    expect(screen.queryByText(absent)).toBeNull();
  },
);

function mountProjectAI(pageId: string, route: string) {
  const projects = deploymentApplications.find(
    (item) => item.id === "projects",
  )!.protocol!;
  const projectPage = projects.pages.find((item) => item.id === pageId)!;
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter initialEntries={[route]}>
          <Routes>
            <Route
              path={projectPage.route}
              element={
                <ProtocolPage
                  application={{
                    ...projects,
                    permission: "runtime.protocol.view",
                    providers: projects.providers.map((provider) => ({
                      ...provider,
                      permission: "",
                      outputSchema: "",
                    })),
                  }}
                  page={{ ...projectPage, permission: "runtime.protocol.view" }}
                  invocation={{
                    applicationId: "projects",
                    pageId,
                    metadataVersion: "metadata",
                    installationRevision: 1,
                  }}
                />
              }
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

it("analyzes selected project rows and retries a failed row in place", async () => {
  api.read.mockResolvedValue({
    items: [
      {
        id: 11,
        name: "First project",
        status: "active",
        dueDate: "",
        taskOverdueCount: 1,
      },
      {
        id: 12,
        name: "Second project",
        status: "active",
        dueDate: "",
        taskOverdueCount: 0,
      },
    ],
    total: 2,
  });
  api.execute
    .mockResolvedValueOnce({ status: "failed", errorCode: "unavailable" })
    .mockResolvedValueOnce({
      status: "succeeded",
      data: {
        status: "succeeded",
        label: "high",
        summary: "One task is overdue",
      },
    });
  mountProjectAI("project.list", "/projects");
  const row = await screen.findByRole("row", { name: /First project/ });
  const user = userEvent.setup();
  await user.click(within(row).getByRole("checkbox"));
  await user.click(
    screen.getByRole("button", { name: "Analyze 1 selected records" }),
  );
  await user.click(await within(row).findByRole("button", { name: "Retry" }));
  expect(await within(row).findByText("High risk")).toBeInTheDocument();
  expect(api.execute.mock.calls.map(([, , input]) => input.recordId)).toEqual([
    11, 11,
  ]);
  expect(screen.getByRole("row", { name: /Second project/ })).toHaveTextContent(
    "Not analyzed",
  );
});

it("renders the project task table only in the Tasks tab", async () => {
  api.read.mockImplementation(async (name: string) => {
    if (name === "project.get")
      return { id: 7, name: "Release project", status: "active" };
    if (name === "project.task.list")
      return { items: [{ id: 11, title: "Prepare release" }], total: 1 };
    if (name === "project.plan.list")
      return { items: [{ id: 4, title: "Release plan" }] };
    return { items: [] };
  });
  mountProjectAI("project.detail", "/projects/7");
  const user = userEvent.setup();
  expect(
    await screen.findByRole("heading", { name: "Release project" }),
  ).toBeVisible();
  expect(screen.queryByText("Prepare release")).not.toBeInTheDocument();
  expect(
    api.read.mock.calls.some(([name]) => name === "project.task.list"),
  ).toBe(false);

  await user.click(screen.getByRole("tab", { name: "Tasks" }));
  expect(await screen.findByText("Prepare release")).toBeVisible();
  expect(screen.getAllByRole("table")).toHaveLength(1);
  await user.click(screen.getByRole("tab", { name: "Plans" }));
  expect(await screen.findByText("Release plan")).toBeVisible();
  expect(screen.queryByText("Prepare release")).not.toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: "Members" }));
  expect(screen.queryByText("Prepare release")).not.toBeInTheDocument();
});

it("keeps separate results for project analysis scenarios", async () => {
  api.read.mockImplementation(async (name: string) =>
    name === "project.get"
      ? {
          id: 7,
          name: "Release project",
          status: "active",
          taskOverdueCount: 1,
        }
      : { items: [], total: 0 },
  );
  api.execute.mockImplementation(async (_name, _version, input) => ({
    status: "succeeded",
    data: {
      status: "succeeded",
      summary:
        input.sceneId === "schedule-risk"
          ? "Release could slip"
          : "No visible blockers",
    },
  }));
  mountProjectAI("project.detail", "/projects/7");
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", { name: "Schedule risk" }),
  );
  expect(await screen.findByText("Release could slip")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Task blockers" }));
  expect(await screen.findByText("No visible blockers")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Schedule risk" }));
  expect(screen.getByText("Release could slip")).toBeInTheDocument();
  expect(api.execute).toHaveBeenCalledTimes(2);
});

it("applies a task description suggestion to the draft before the existing save", async () => {
  api.read.mockImplementation(async (name: string) =>
    name === "project.get"
      ? { id: 7, name: "Release project" }
      : { items: [], total: 0 },
  );
  api.execute.mockImplementation(async (name: string) =>
    name === "application.runtime.ai.suggest"
      ? {
          status: "succeeded",
          data: {
            suggestions: [
              { field: "description", value: "Prepare the release checklist." },
            ],
          },
        }
      : { status: "succeeded" },
  );
  mountProjectAI("project.detail", "/projects/7?tab=tasks&dialog=create");
  const user = userEvent.setup();
  const dialog = await screen.findByRole("dialog");
  await user.type(
    within(dialog).getByLabelText(/Task title/i),
    "Prepare release",
  );
  await user.click(
    within(dialog).getByRole("button", { name: "Draft task description" }),
  );
  expect(
    await within(dialog).findByText(
      "Suggested: Prepare the release checklist.",
    ),
  ).toBeInTheDocument();
  await user.click(
    within(dialog).getByRole("button", { name: "Apply to draft" }),
  );
  expect(api.execute).toHaveBeenCalledTimes(1);
  await user.click(
    within(dialog).getByRole("button", { name: "Save changes" }),
  );
  await waitFor(() =>
    expect(api.execute).toHaveBeenLastCalledWith(
      "project.task.create",
      "1.0.0",
      expect.objectContaining({
        description: "Prepare the release checklist.",
      }),
      expect.any(String),
      expect.any(String),
      expect.any(Object),
    ),
  );
});
