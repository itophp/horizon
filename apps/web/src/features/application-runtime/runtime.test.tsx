import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import { MemoryRouter, Routes, Route } from "react-router";
import { ApplicationPage } from "./legacy-page";
import { RuntimeForm } from "./form";
import { validateApplication } from "./registry";
import type { RuntimeApplication } from "@/api/generated/runtime";
const api = vi.hoisted(() => ({
  application: vi.fn(),
  read: vi.fn(),
  save: vi.fn(),
  permissions: ["articles:read", "articles:write"],
}));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeApplication: api.application,
  runtimeRead: api.read,
  runtimeSave: api.save,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "a",
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
const field = {
  name: "title",
  link: true,
  valueLabelPrefix: "",
  labelKey: "articles:title",
  component: "text",
  kind: "business",
  type: "string",
  required: true,
  sensitive: false,
  readOnly: false,
  hidden: false,
  maxLength: 200,
  enum: [],
};
const app: RuntimeApplication = {
  id: "articles",
  version: "1.0.0",
  metadataVersion: "hash",
  labelKey: "articles:heading",
  route: "/articles",
  pages: ["list", "detail", "form"].map((type) => ({
    relations: [],
    editableStates: ["draft", "rejected"],
    schemaVersion: "1.0",
    id: `article.${type}`,
    type,
    titleKey: "articles:heading",
    entityRef: "article",
    layout: "standard",
    fields:
      type === "form"
        ? [
            field,
            {
              ...field,
              name: "body",
              labelKey: "articles:body",
              component: "textarea",
              sensitive: true,
              maxLength: 20000,
            },
          ]
        : [{ ...field, readOnly: true }],
    readCapability: type === "list" ? "article.list" : "article.get",
    createCapability: "article.create",
    updateCapability: "article.update",
  })),
};
function wrap(node: React.ReactNode) {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
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
          {node}
        </QueryClientProvider>
      </MemoryRouter>
    </I18nextProvider>,
  );
}
beforeEach(async () => {
  vi.clearAllMocks();
  api.permissions = ["articles:read", "articles:write"];
  await changeLanguage("en", false);
  api.application.mockResolvedValue(app);
  api.read.mockResolvedValue({
    items: [{ id: 1, title: "First" }],
    total: 45,
    page: 1,
    pageSize: 20,
  });
  api.save.mockResolvedValue({ status: "succeeded" });
});
it("loads metadata and requests the next server page", async () => {
  const user = userEvent.setup();
  wrap(<ApplicationPage applicationId="articles" permission="articles:read" />);
  await screen.findByText("First");
  await user.click(screen.getByRole("button", { name: "Next page" }));
  await waitFor(() =>
    expect(api.read).toHaveBeenLastCalledWith(
      "article.list",
      expect.objectContaining({ page: 2, pageSize: 20 }),
      expect.any(AbortSignal),
    ),
  );
});
it("requires valid input and submits only editable fields with revision", async () => {
  const user = userEvent.setup();
  const saved = vi.fn();
  wrap(
    <RuntimeForm
      page={app.pages[2]}
      record={{ id: 7, revision: 3, title: "", body: "" }}
      onSaved={saved}
      onCancel={vi.fn()}
    />,
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  expect(api.save).not.toHaveBeenCalled();
  await user.type(screen.getByLabelText("Title"), "New title");
  await user.type(screen.getByLabelText("Body"), "Private content");
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() => expect(saved).toHaveBeenCalled());
  expect(api.save).toHaveBeenCalledWith(
    "article.update",
    { id: 7, revision: 3, title: "New title", body: "Private content" },
    expect.any(String),
  );
});
it("keeps the same idempotency key on unchanged failed retries", async () => {
  api.save.mockRejectedValue(new Error("failed"));
  const user = userEvent.setup();
  wrap(
    <RuntimeForm
      page={app.pages[2]}
      record={{ id: 1, revision: 1, title: "A", body: "B" }}
      onSaved={vi.fn()}
      onCancel={vi.fn()}
    />,
  );
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(1));
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() => expect(api.save).toHaveBeenCalledTimes(2));
  expect(api.save.mock.calls[0][2]).toBe(api.save.mock.calls[1][2]);
});
it("does not read metadata without page permission", () => {
  api.permissions = [];
  wrap(<ApplicationPage applicationId="articles" permission="articles:read" />);
  expect(api.application).not.toHaveBeenCalled();
});
it("rejects unknown components, versions, and props", () => {
  for (const mutate of [
    (a: RuntimeApplication) => {
      a.pages[0].schemaVersion = "2";
    },
    (a: RuntimeApplication) => {
      a.pages[0].fields[0].component = "script";
    },
  ]) {
    const value = structuredClone(app);
    mutate(value);
    expect(() => validateApplication(value)).toThrow();
  }
  expect(() => validateApplication({ ...app, script: "alert(1)" })).toThrow();
});

it("omits hidden and readonly fields from save input", async () => {
  const page = structuredClone(app.pages[2]);
  page.fields[1].hidden = true;
  page.fields.push({
    ...field,
    name: "revision",
    labelKey: "articles:article.revision",
    readOnly: true,
  });
  const user = userEvent.setup();
  wrap(
    <RuntimeForm
      page={page}
      record={{ id: 1, revision: 4, title: "Allowed", body: "secret" }}
      onSaved={vi.fn()}
      onCancel={vi.fn()}
    />,
  );
  expect(screen.queryByLabelText("Body")).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() => expect(api.save).toHaveBeenCalled());
  expect(api.save.mock.calls[0][1]).toEqual({
    id: 1,
    revision: 4,
    title: "Allowed",
  });
});
it("supports a trusted custom renderer inside the runtime context", async () => {
  const custom = structuredClone(app);
  custom.pages = [{ ...custom.pages[0], type: "custom" }];
  api.application.mockResolvedValue(custom);
  wrap(
    <ApplicationPage
      applicationId="articles"
      permission="articles:read"
      extension={{
        render: () => <span>{i18n.t("articles:runtime.back")}</span>,
      }}
    />,
  );
  await screen.findByText("Back to list");
  expect(api.read).not.toHaveBeenCalled();
});

it("navigates from the list into a schema detail and edits the loaded revision", async () => {
  api.read.mockImplementation(async (name: string) =>
    name === "article.list"
      ? { items: [{ id: 1, title: "First" }], total: 1, page: 1, pageSize: 20 }
      : {
          id: 1,
          title: "First",
          body: "Body text",
          status: "draft",
          revision: 2,
        },
  );
  const user = userEvent.setup();
  wrap(
    <Routes>
      <Route
        path="/"
        element={
          <ApplicationPage
            applicationId="articles"
            permission="articles:read"
          />
        }
      />
      <Route
        path="/articles/:id"
        element={
          <ApplicationPage
            applicationId="articles"
            permission="articles:read"
          />
        }
      />
    </Routes>,
  );
  await user.click(await screen.findByRole("link", { name: "First" }));
  await screen.findByRole("link", { name: "Back to list" });
  await user.click(screen.getByRole("button", { name: "Edit" }));
  const input = await screen.findByLabelText("Title");
  await user.clear(input);
  await user.type(input, "Revised");
  await user.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() =>
    expect(api.save).toHaveBeenCalledWith(
      "article.update",
      { id: 1, revision: 2, title: "Revised", body: "Body text" },
      expect.any(String),
    ),
  );
});
