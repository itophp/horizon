import { readFileSync } from "node:fs";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
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
    account: { id: 7 },
    activeDepartmentId: 3,
    permissions: ["runtime.protocol.view", "accounts:read"],
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
