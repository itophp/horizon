import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { ApiError } from "@/api/client/request";
import { changeLanguage, i18n } from "@/i18n";
import { KnowledgePage } from "./page";
import { KnowledgeDetailPage } from "./detail";

const api = vi.hoisted(() => ({
  query: vi.fn(),
  read: vi.fn(),
  update: vi.fn(),
  create: vi.fn(),
  remove: vi.fn(),
  reindex: vi.fn(),
  permissions: [] as string[],
}));

vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  knowledgeQuery: api.query,
  knowledgeRead: api.read,
  knowledgeUpdate: api.update,
  knowledgeCreate: api.create,
  knowledgeDelete: api.remove,
  knowledgeReindex: api.reindex,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant-a",
    activeDepartmentId: 1,
    permissions: api.permissions,
    account: { id: 4, fullName: "Ada", memberships: [] },
  }),
}));

const document = {
  id: 7,
  title: "Remote access policy",
  source: "manual",
  chunks: 3,
  createdAt: "2026-10-31T09:00:00Z",
};

function show(path = "/knowledge") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <MemoryRouter initialEntries={[path]}>
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>
          <Routes>
            <Route path="/knowledge" element={<KnowledgePage />} />
            <Route path="/knowledge/:id" element={<KnowledgeDetailPage />} />
          </Routes>
        </QueryClientProvider>
      </I18nextProvider>
    </MemoryRouter>,
  );
}

beforeEach(async () => {
  await changeLanguage("en", false);
  api.permissions = ["knowledge:read", "knowledge:create", "knowledge:delete"];
  api.query.mockReset().mockResolvedValue({
    status: 200,
    data: { documents: [document] },
    headers: {},
  });
  api.read.mockReset().mockResolvedValue({
    status: 200,
    data: { ...document, content: "Staff must use the VPN." },
    headers: {},
  });
  api.update.mockReset().mockResolvedValue({ status: 200, data: { id: 7 } });
  api.create.mockResolvedValue({ status: 200, data: document, headers: {} });
  api.remove.mockResolvedValue({ status: 200, data: { id: 7 }, headers: {} });
  vi.spyOn(window, "confirm").mockReturnValue(true);
});

it("lists indexed documents with their fragment count", async () => {
  show();
  expect(await screen.findByText("Remote access policy")).toBeTruthy();
  expect(screen.getByText(/3 fragments/)).toBeTruthy();
});

it("submits a content query and opens the original text from a result", async () => {
  api.permissions = ["knowledge:read"];
  const user = userEvent.setup();
  show();
  expect(screen.getByRole("search", { name: "Search" })).toBeVisible();
  expect(screen.queryByRole("dialog")).toBeNull();
  await screen.findByRole("link", { name: `View text for ${document.title}` });
  await user.type(screen.getByRole("searchbox"), "VPN");
  await user.click(screen.getByRole("button", { name: "Search" }));
  await waitFor(() =>
    expect(api.query).toHaveBeenLastCalledWith(
      { keyword: "VPN", beforeId: undefined },
      expect.any(AbortSignal),
    ),
  );
  await user.click(
    await screen.findByRole("link", {
      name: `View text for ${document.title}`,
    }),
  );
  expect(await screen.findByText("Staff must use the VPN.")).toBeVisible();
  expect(api.read).toHaveBeenCalledWith({ id: 7 }, expect.any(AbortSignal));
  expect(
    screen.getByRole("link", { name: "Back to knowledge base" }),
  ).toHaveAttribute("href", "/knowledge");
});

it("blocks a directly opened document for an account without knowledge access", () => {
  api.permissions = [];
  show("/knowledge/7");
  expect(screen.getByRole("alert")).toHaveTextContent(
    "You do not have permission",
  );
  expect(api.read).not.toHaveBeenCalled();
});

it("edits the document from its details and reloads the saved text", async () => {
  api.permissions.push("knowledge:update");
  api.read
    .mockResolvedValueOnce({
      data: {
        ...document,
        content: "Original text",
        version: "current-version",
      },
    })
    .mockResolvedValueOnce({
      data: {
        ...document,
        title: "New title",
        content: "Updated text",
        version: "next-version",
      },
    });
  const user = userEvent.setup();
  show("/knowledge/7");
  await user.click(
    await screen.findByRole("button", { name: "Edit document" }),
  );
  const title = screen.getByLabelText("Title");
  const content = screen.getByLabelText("Content");
  await user.clear(title);
  await user.type(title, "New title");
  await user.clear(content);
  await user.type(content, "Updated text");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(
    await screen.findByText("Document updated. Its new index is active."),
  ).toBeVisible();
  expect(await screen.findByText("Updated text")).toBeVisible();
  expect(api.update.mock.calls[0]?.[0]).toEqual({
    id: 7,
    expectedVersion: "current-version",
    title: "New title",
    content: "Updated text",
  });
});

it("preserves the draft and reports a concurrent edit", async () => {
  api.permissions.push("knowledge:update");
  api.update.mockRejectedValueOnce(new ApiError("conflict", 400));
  const user = userEvent.setup();
  show("/knowledge/7");
  await user.click(
    await screen.findByRole("button", { name: "Edit document" }),
  );
  const content = screen.getByLabelText("Content");
  await user.clear(content);
  await user.type(content, "My draft");
  await user.click(screen.getByRole("button", { name: "Save" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "This document has changed.",
  );
  expect(content).toHaveValue("My draft");
  expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
});

it("indexes a pasted document and clears the form", async () => {
  show();
  const add = screen.getByRole("button", { name: "Add a document" });
  await userEvent.click(add);
  await userEvent.type(
    await screen.findByPlaceholderText("Remote access policy"),
    "Remote access policy",
  );
  await userEvent.type(
    screen.getByPlaceholderText(/Paste the policy text/),
    "Staff must use the VPN.",
  );
  await userEvent.click(screen.getByRole("button", { name: "Index document" }));
  await waitFor(() =>
    expect(api.create).toHaveBeenCalledWith({
      title: "Remote access policy",
      content: "Staff must use the VPN.",
    }),
  );
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  await userEvent.click(add);
  expect(
    await screen.findByPlaceholderText(/Paste the policy text/),
  ).toHaveValue("");
});

it("deletes a document after confirmation", async () => {
  show();
  await screen.findByText("Remote access policy");
  await userEvent.click(screen.getByRole("button", { name: "Delete" }));
  await waitFor(() => expect(api.remove).toHaveBeenCalledWith(7));
});

it("reports the vector store failure and confirms a successful retry", async () => {
  api.permissions.push("aiConfig:read");
  api.reindex
    .mockRejectedValueOnce(
      new ApiError("knowledge_vector_store_unavailable", 400),
    )
    .mockResolvedValueOnce({ status: 200, data: { documents: 1 } });
  show();
  await screen.findByText("Remote access policy");
  const user = userEvent.setup();
  const rebuild = screen.getByRole("button", { name: "Rebuild vectors" });
  await user.click(rebuild);
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "The Redis vector store is unavailable",
  );
  expect(
    screen.getByRole("link", { name: "AI Configuration" }),
  ).toHaveAttribute("href", "/ai-settings?tab=general");
  await user.click(rebuild);
  expect(await screen.findByRole("status")).toHaveTextContent(
    "Rebuilt 1 document",
  );
  expect(screen.queryByRole("alert")).toBeNull();
});

it("hides writing when the grant is read only", async () => {
  api.permissions = ["knowledge:read"];
  show();
  expect(await screen.findByText("Remote access policy")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Index document" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Add a document" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Delete" })).toBeNull();
});
