import { readFileSync } from "node:fs";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { changeLanguage, i18n } from "@/i18n";
import type {
  RuntimeApplicationV2,
  RuntimeDefinitionView,
} from "@/api/generated/runtime";
import { ApplicationDesignPage } from "./design-page";

const api = vi.hoisted(() => ({ read: vi.fn(), execute: vi.fn() }));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeRead: api.read,
  runtimeExecute: api.execute,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant",
    account: { id: 1 },
    activeDepartmentId: 1,
    authorizationVersion: 1,
    permissions: [
      "applications:read",
      "applications:design",
      "applications:manage",
      "assistant:use",
      "orchestration:run",
    ],
  }),
}));
vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));

const applications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as RuntimeApplicationV2[];

beforeEach(async () => {
  vi.resetAllMocks();
  await changeLanguage("en", false);
});

it("opens design for the application selected from its row", async () => {
  const application = applications.find((app) => app.id === "projects")!;
  const view: RuntimeDefinitionView = {
    application,
    enabled: true,
    version: "2.0.0.1",
    digest: "a".repeat(64),
    revision: 2,
    publishedVersion: "2.0.0.0",
    proposalVersion: "2.0.0.1",
    readOnly: false,
  };
  api.read.mockImplementation(async (name: string) => {
    switch (name) {
      case "application.runtime.definition.list":
        return {
          items: [
            {
              applicationId: "projects",
              labelKey: "projects:heading",
              labelEn: "Projects",
              labelZhCN: "项目协作",
              version: "2.0.0.1",
              digest: "a",
              revision: 2,
            },
          ],
        };
      case "application.runtime.definition.get":
        return view;
      case "application.runtime.installation.get":
        return {
          installation: {
            pluginId: application.source,
            version: application.pluginVersion,
            status: "enabled",
            revision: 2,
            digest: "d",
            previousVersion: "",
          },
        };
      case "application.runtime.definition.validate":
        return { valid: true, code: "" };
      default:
        throw new Error(name);
    }
  });
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <MemoryRouter initialEntries={["/applications/design/projects"]}>
          <Routes>
            <Route
              path="/applications/design/:applicationId"
              element={<ApplicationDesignPage />}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </I18nextProvider>,
  );
  expect(
    await screen.findByRole("heading", { name: "Projects" }),
  ).toBeVisible();
  expect(screen.getByRole("link", { name: "Applications" })).toBeVisible();
  expect(
    screen.getByRole("tab", { name: "Candidate & publish" }),
  ).toBeVisible();
  expect(
    screen.getByRole("button", { name: "Generate proposal" }),
  ).toBeVisible();
  expect(
    await screen.findByRole("button", { name: "Publish candidate" }),
  ).toBeVisible();
  expect(screen.getByText("Application overview")).toBeVisible();
  expect(screen.queryByRole("combobox", { name: "Application" })).toBeNull();
});
