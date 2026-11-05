import { readFileSync } from "node:fs";
import { beforeEach, it, expect, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { i18n, changeLanguage } from "@/i18n";
import type {
  RuntimeApplicationV2,
  RuntimeDefinitionView,
} from "@/api/generated/runtime";
import { DesignVersion } from "./design-version";
const api = vi.hoisted(() => ({ read: vi.fn(), execute: vi.fn() }));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  runtimeRead: api.read,
  runtimeExecute: api.execute,
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({ permissions: ["applications:manage"] }),
}));
vi.mock("./design-activation", () => ({ DesignActivation: () => null }));
vi.mock("./design-history", () => ({ DesignHistory: () => null }));
vi.mock("./design-baseline", () => ({ DesignBaseline: () => null }));
vi.mock("./page-view", () => ({ PageView: () => null }));
vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));
const apps = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as RuntimeApplicationV2[];
beforeEach(async () => {
  vi.resetAllMocks();
  await changeLanguage("en", false);
});
it("publishes only the exact version reviewed before a background definition refresh", async () => {
  const application = apps.find((app) => app.id === "projects")!;
  const candidate: RuntimeDefinitionView = {
    application,
    enabled: true,
    version: "1.0.0.1",
    digest: "a".repeat(64),
    revision: 2,
    publishedVersion: "1.0.0.0",
    proposalVersion: "1.0.0.1",
    readOnly: false,
  };
  api.read.mockImplementation(async (name: string) =>
    name === "application.runtime.definition.get"
      ? candidate
      : {
          installation: {
            pluginId: application.source,
            version: application.pluginVersion,
            status: "enabled",
            revision: 1,
            digest: "d",
            previousVersion: "",
          },
        },
  );
  api.execute.mockResolvedValue({ status: "failed", errorCode: "conflict" });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <DesignVersion applicationId="projects" identity="identity" />
      </QueryClientProvider>
    </I18nextProvider>,
  );
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole("button", { name: "Publish candidate" }),
  );
  await act(async () => {
    client.setQueryData(["runtime-design-version", "identity", "projects"], {
      ...candidate,
      version: "1.0.0.2",
      digest: "b".repeat(64),
      revision: 3,
      proposalVersion: "1.0.0.2",
    });
  });
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  expect(api.execute).toHaveBeenCalledWith(
    "application.runtime.definition.publish",
    "1.0.0",
    {
      applicationId: "projects",
      version: "1.0.0.1",
      digest: "a".repeat(64),
      revision: 2,
    },
    expect.any(String),
    expect.any(String),
  );
  expect(screen.getByRole("dialog")).toBeInTheDocument();
});
