import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { KnowledgePage } from "./page";

const api = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  remove: vi.fn(),
  permissions: [] as string[],
}));

vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  knowledgeList: api.list,
  knowledgeCreate: api.create,
  knowledgeDelete: api.remove,
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

function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <KnowledgePage />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  await changeLanguage("en", false);
  api.permissions = ["knowledge:read", "knowledge:create", "knowledge:delete"];
  api.list.mockResolvedValue({ status: 200, data: [document], headers: {} });
  api.create.mockResolvedValue({ status: 200, data: document, headers: {} });
  api.remove.mockResolvedValue({ status: 200, data: { id: 7 }, headers: {} });
  vi.spyOn(window, "confirm").mockReturnValue(true);
});

it("lists indexed documents with their fragment count", async () => {
  show();
  expect(await screen.findByText("Remote access policy")).toBeTruthy();
  expect(screen.getByText(/3 fragments/)).toBeTruthy();
});

it("indexes a pasted document and clears the form", async () => {
  show();
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
  await waitFor(() =>
    expect(
      (
        screen.getByPlaceholderText(
          /Paste the policy text/,
        ) as HTMLTextAreaElement
      ).value,
    ).toBe(""),
  );
});

it("deletes a document after confirmation", async () => {
  show();
  await screen.findByText("Remote access policy");
  await userEvent.click(screen.getByRole("button", { name: "Delete" }));
  await waitFor(() => expect(api.remove).toHaveBeenCalledWith(7));
});

it("hides writing when the grant is read only", async () => {
  api.permissions = ["knowledge:read"];
  show();
  expect(await screen.findByText("Remote access policy")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "Index document" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Delete" })).toBeNull();
});
