import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { changeLanguage, i18n } from "@/i18n";
import { DrivePage } from "./page";

const api = vi.hoisted(() => ({
  ensure: vi.fn(),
  list: vi.fn(),
  create: vi.fn(),
  permissions: [] as string[],
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant-a",
    activeDepartmentId: 1,
    account: { id: 1 },
    permissions: api.permissions,
  }),
}));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  drivePersonalEnsure: api.ensure,
  driveSpaces: api.list,
  driveSpaceCreate: api.create,
}));

function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <DrivePage />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  vi.clearAllMocks();
  await changeLanguage("en", false);
  api.permissions = ["drive:use", "drive:create"];
  api.ensure.mockResolvedValue({ data: { id: 1 } });
  api.list.mockResolvedValue({
    data: {
      items: [{ id: 1, kind: "personal", name: "Personal", role: "admin" }],
    },
  });
  api.create.mockResolvedValue({ data: { id: 2 } });
});

it("initializes the personal space and lists it", async () => {
  show();
  expect(
    await screen.findByRole("link", { name: "Personal space" }),
  ).toBeTruthy();
  await waitFor(() => expect(api.ensure).toHaveBeenCalledTimes(1));
});

it("creates a shared space through the dialog", async () => {
  show();
  await userEvent.click(
    await screen.findByRole("button", { name: "Create shared space" }),
  );
  await userEvent.type(screen.getByLabelText("Name"), "Engineering");
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(api.create).toHaveBeenCalledWith("Engineering"));
});

it("does not offer shared-space creation without its permission", async () => {
  api.permissions = ["drive:use"];
  show();
  await screen.findByRole("link", { name: "Personal space" });
  expect(
    screen.queryByRole("button", { name: "Create shared space" }),
  ).toBeNull();
});
