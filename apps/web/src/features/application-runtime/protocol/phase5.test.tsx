import { readFileSync } from "node:fs";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
import { changeLanguage, i18n } from "@/i18n";
import { DynamicApplicationRoute } from "../route";
import type { ApplicationV3 } from "./types";

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
    account: { id: 1 },
    activeDepartmentId: 2,
    permissions: [
      "articles:read",
      "articles:write",
      "articles:review",
      "orchestration:run",
      "projects:read",
      "projects:write",
    ],
  }),
}));

vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));

type DeploymentApplication = {
  id: string;
  protocol?: ApplicationV3;
};

const applications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as DeploymentApplication[];
const articleApp = applications.find((item) => item.id === "articles")!;
const projectApp = applications.find((item) => item.id === "projects")!;

const articleRow = {
  id: 8,
  title: "Protocol migration",
  status: "pending_review",
  revision: 3,
  reviewCode: "human_review",
};
const articleDetail = {
  id: 8,
  title: "Protocol migration",
  body: "**Approved body**",
  status: "pending_review",
  revision: 3,
  reviewCode: "human_review",
};

function mount(path: string) {
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter initialEntries={[path]}>
          <DynamicApplicationRoute />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  vi.clearAllMocks();
  await changeLanguage("en", false);
  api.execute.mockResolvedValue({ status: "succeeded" });
  api.application.mockImplementation(
    async (id: string) =>
      applications.find((item) => item.id === id) ?? articleApp,
  );
  api.read.mockImplementation(async (name: string) => {
    if (name === "application.runtime.list")
      return { items: [articleApp, projectApp] };
    if (name === "article.list")
      return {
        items: [articleRow],
        total: 21,
        page: 1,
        pageSize: 20,
      };
    if (name === "article.get") return articleDetail;
    if (name === "project.get")
      return {
        id: 3,
        code: "OPS",
        name: "Operations",
        status: "active",
        priority: "high",
        ownerId: 1,
        ownerName: "Alice Chen",
        dueDate: "",
        taskCount: 4,
        taskDoneCount: 1,
        taskOverdueCount: 0,
        planCount: 1,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-01T00:00:00Z",
      };
    if (name === "project.list")
      return {
        items: [
          {
            id: 3,
            code: "OPS",
            name: "Operations",
            ownerId: 1,
            status: "active",
            priority: "high",
            ownerName: "Alice",
            taskCount: 4,
          },
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      };
    if (name === "project.task.get")
      return {
        id: 9,
        projectId: 3,
        title: "Release checklist",
        description: "**Ship it**",
        status: "todo",
        priority: "high",
        assigneeId: 0,
        assigneeName: "",
        planId: 0,
        planTitle: "",
        dueDate: "",
        brief: "",
        briefModel: "",
      };
    if (name === "project.member.list")
      return {
        items: [
          {
            accountId: 11,
            name: "Alice Chen",
            email: "alice@test",
            isOwner: true,
          },
          { accountId: 12, name: "Bob Lin", email: "bob@test", isOwner: false },
        ],
      };
    if (name === "project.member.search")
      return {
        items: [{ accountId: 13, name: "Cara Wu", email: "cara@test" }],
      };
    if (name === "project.plan.list")
      return {
        items: [
          {
            id: 4,
            projectId: 3,
            title: "Launch",
            planType: "milestone",
            status: "pending",
            priority: "high",
            ownerId: 1,
            ownerName: "Alice Chen",
            content: "",
            dueDate: "",
            createdAt: "2026-01-01T00:00:00Z",
            updatedAt: "2026-01-01T00:00:00Z",
          },
        ],
      };
    return { items: [] };
  });
});

it("renders the V3 article list with server pagination", async () => {
  const user = userEvent.setup();
  mount("/articles");
  expect(await screen.findByText("Protocol migration")).toBeInTheDocument();
  expect(document.querySelector(".table-pagination")?.textContent).toContain(
    "21",
  );
  const draft = screen.getByRole("button", { name: /Draft/i });
  await user.click(draft);
  expect(screen.getByRole("button", { name: /Draft/i })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await user.click(screen.getByRole("button", { name: "Next" }));
  await waitFor(() =>
    expect(api.read).toHaveBeenLastCalledWith(
      "article.list",
      expect.objectContaining({ page: 2, pageSize: 20 }),
      expect.any(AbortSignal),
      "1.0.0",
    ),
  );
});

it("invokes a schema-declared article decision from the generic detail panel", async () => {
  const user = userEvent.setup();
  mount("/articles/8");
  expect(await screen.findByText("Protocol migration")).toBeInTheDocument();
  expect(screen.getByText("Approved body")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: /Approve and publish/i }),
  );
  await waitFor(() => expect(api.execute).toHaveBeenCalled());
  expect(api.execute).toHaveBeenCalledWith(
    "article.decide",
    "1.0.0",
    { id: 8, revision: 3, approve: true },
    expect.any(String),
    expect.any(String),
    expect.anything(),
  );
});

it("creates a project through the schema-declared V3 form command", async () => {
  const user = userEvent.setup();
  mount("/projects");
  expect(await screen.findByText("Operations")).toBeInTheDocument();
  await user.click(
    screen.getAllByRole("button", { name: /New project/i }).at(-1)!,
  );
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  await user.type(screen.getByLabelText("Code"), "HR");
  await user.type(screen.getByLabelText("Name"), "Human resources");
  await user.selectOptions(screen.getByLabelText("Priority"), "high");
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalled());
  expect(api.execute).toHaveBeenCalledWith(
    "project.create",
    "1.0.0",
    expect.objectContaining({
      code: "HR",
      name: "Human resources",
      priority: "high",
      dueDate: "",
    }),
    expect.any(String),
    expect.any(String),
    expect.anything(),
  );
});

it("assigns a task from schema-declared project-scoped references", async () => {
  const user = userEvent.setup();
  mount("/projects/3/tasks/9");
  expect(await screen.findByText("Release checklist")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: /Assign/i }));
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  const assignee = screen.getByRole("combobox");
  expect(assignee).toHaveTextContent("Alice Chen");
  await user.selectOptions(assignee, "11");
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  await waitFor(() => expect(api.execute).toHaveBeenCalled());
  expect(api.execute).toHaveBeenCalledWith(
    "project.task.assign",
    "1.0.0",
    { id: 9, assigneeId: 11 },
    expect.any(String),
    expect.any(String),
    expect.anything(),
  );
});

it("adds a project member through the generic schema-declared search picker", async () => {
  const user = userEvent.setup();
  mount("/projects/3");
  expect(await screen.findByText("Operations")).toBeInTheDocument();
  await user.click(screen.getByRole("tab", { name: /Members/i }));
  expect(await screen.findByText("Alice Chen")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: /^Add/i }));
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  await user.type(
    screen.getByRole("textbox", { name: /name or email/i }),
    "Cara",
  );
  await user.click(screen.getByRole("button", { name: "Search" }));
  expect(await screen.findByText("Cara Wu")).toBeInTheDocument();
  await user.click(screen.getAllByRole("button", { name: "Add" }).at(-1)!);
  await waitFor(() => expect(api.execute).toHaveBeenCalled());
  expect(api.execute).toHaveBeenCalledWith(
    "project.member.add",
    "1.0.0",
    { projectId: 3, accountId: 13 },
    expect.any(String),
    expect.any(String),
    expect.anything(),
  );
});
