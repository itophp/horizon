import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { ApiError } from "@/api/client/request";
import { DraftDialog } from "./forms";
import { BindingDialog } from "./binding-dialog";
import { PlanInspector } from "./inspector";
import type { Binding, Source } from "./state";
import type { ReactNode } from "react";
const api = vi.hoisted(() => ({
  create: vi.fn(),
  bind: vi.fn(),
  rollback: vi.fn(),
  references: vi.fn(),
  inspect: vi.fn(),
  preflight: vi.fn(),
}));
vi.mock("@/lib/capabilities", async (original) => ({
  ...(await original<typeof import("@/lib/capabilities")>()),
  executionPlans: api,
}));
vi.mock("@/features/governance/runtime", () => ({
  useConfiguration: () => ({
    value: (_key: string, fallback: unknown) => fallback,
  }),
}));
vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "a",
    account: { id: 1 },
    activeDepartmentId: 2,
    authorizationVersion: 1,
  }),
}));
vi.mock("@/features/orchestration/viewer", () => ({
  WorkflowViewer: () => null,
}));
const target = {
  capability: "example.record.archive",
  capabilityVersion: "1.0.0",
};
const source: Source = {
  name: "Archive flow",
  scopeType: "tenant",
  plan: {
    planId: "archive-flow",
    capabilityId: target.capability,
    capabilityVersion: "1.0.0",
    version: 2,
    mode: "workflow",
    implementationRef: "example-archive-plan@1.0.0",
    status: "published",
    workflowHash: "hash",
  },
};
const binding: Binding = {
  bindingId: "default",
  scopeType: "tenant",
  revision: 3,
  plan: source.plan,
};
function show(children: ReactNode) {
  return render(
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
        {children}
      </QueryClientProvider>
    </I18nextProvider>,
  );
}
beforeEach(async () => {
  vi.resetAllMocks();
  await changeLanguage("en", false);
  api.references.mockResolvedValue({
    data: {
      references: [
        {
          ref: source.plan.implementationRef,
          name: "Archive implementation",
          version: "1.0.0",
          mode: "workflow",
          hash: "hash",
        },
      ],
    },
  });
  api.inspect.mockResolvedValue({
    data: { inputSchema: { type: "object" }, referenceHash: "hash" },
  });
  api.preflight.mockResolvedValue({
    data: { valid: true, effectsExecuted: false },
  });
});
it("saves a draft independently and preserves the retry key", async () => {
  const saved = vi.fn();
  api.create
    .mockRejectedValueOnce(new ApiError("unavailable", 503))
    .mockResolvedValueOnce({
      data: { planId: "plan-example-record-archive", planVersion: 1 },
    });
  show(
    <DraftDialog
      target={target}
      readOnly={false}
      onSaved={saved}
      onClose={vi.fn()}
    />,
  );
  await userEvent.type(screen.getByLabelText("Plan name"), "New plan");
  await userEvent.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() => expect(api.create).toHaveBeenCalledTimes(1));
  await screen.findByRole("alert");
  await userEvent.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() => expect(saved).toHaveBeenCalledTimes(1));
  expect(api.create.mock.calls[0]).toEqual(api.create.mock.calls[1]);
  expect(api.create.mock.calls[0][0]).toEqual({
    ...target,
    name: "New plan",
    planId: "plan-example-record-archive",
    mode: "direct",
  });
  expect(api.bind).not.toHaveBeenCalled();
  expect(api.rollback).not.toHaveBeenCalled();
});
it("copies a registered workflow through authorized remote selection", async () => {
  api.create.mockResolvedValue({});
  show(
    <DraftDialog
      target={target}
      source={source}
      readOnly={false}
      onSaved={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await waitFor(() =>
    expect(api.references).toHaveBeenCalledWith(
      { ...target, mode: "workflow", query: source.plan.implementationRef },
      expect.any(AbortSignal),
    ),
  );
  await userEvent.clear(
    screen.getByLabelText("Search registered implementations"),
  );
  await userEvent.type(
    screen.getByLabelText("Search registered implementations"),
    "archive",
  );
  await waitFor(() =>
    expect(api.references).toHaveBeenLastCalledWith(
      { ...target, mode: "workflow", query: "archive" },
      expect.any(AbortSignal),
    ),
  );
  expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
  await userEvent.click(
    screen.getByRole("combobox", { name: "Registered implementation" }),
  );
  await userEvent.click(
    await screen.findByRole("option", {
      name: "Archive implementation · V1.0.0",
    }),
  );
  await userEvent.click(screen.getByRole("button", { name: "Save draft" }));
  await waitFor(() =>
    expect(api.create).toHaveBeenCalledWith(
      {
        ...target,
        planId: "archive-flow",
        name: "Archive flow",
        mode: "workflow",
        workflowRef: source.plan.implementationRef,
      },
      expect.any(String),
    ),
  );
});
it("keeps the displayed revision and stable key when a binding conflicts", async () => {
  api.bind.mockRejectedValue(new ApiError("conflict", 409));
  const saved = vi.fn();
  show(
    <BindingDialog
      target={target}
      source={source}
      binding={binding}
      rollback={false}
      onSaved={saved}
      onClose={vi.fn()}
    />,
  );
  await userEvent.type(screen.getByLabelText("Reason"), "Reviewed");
  await userEvent.click(screen.getByRole("button", { name: "Set as default" }));
  await screen.findByRole("alert");
  await userEvent.click(screen.getByRole("button", { name: "Set as default" }));
  await waitFor(() => expect(api.bind).toHaveBeenCalledTimes(2));
  expect(api.bind.mock.calls[0]).toEqual(api.bind.mock.calls[1]);
  expect(api.bind.mock.calls[0][0]).toEqual({
    ...target,
    planId: "archive-flow",
    planVersion: 2,
    expectedRevision: 3,
    reason: "Reviewed",
  });
  expect(saved).not.toHaveBeenCalled();
});
it("uses the distinct rollback capability", async () => {
  api.rollback.mockResolvedValue({});
  show(
    <BindingDialog
      target={target}
      source={source}
      binding={binding}
      rollback
      onSaved={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  await userEvent.type(screen.getByLabelText("Reason"), "Restore");
  await userEvent.click(
    screen.getByRole("button", { name: "Roll back to this version" }),
  );
  await waitFor(() => expect(api.rollback).toHaveBeenCalledTimes(1));
  expect(api.bind).not.toHaveBeenCalled();
});
it("rejects invalid JSON locally and labels preflight as structure only", async () => {
  show(
    <PlanInspector
      target={target}
      source={source}
      current={source}
      canValidate
    />,
  );
  const sample = await screen.findByLabelText("Sample input JSON");
  await userEvent.clear(sample);
  await userEvent.type(sample, "[[]");
  await userEvent.click(
    screen.getByRole("button", { name: "Check structure" }),
  );
  await screen.findByRole("alert");
  expect(api.preflight).not.toHaveBeenCalled();
  await userEvent.clear(sample);
  await userEvent.type(sample, '{{"id":7}');
  await userEvent.click(
    screen.getByRole("button", { name: "Check structure" }),
  );
  await waitFor(() =>
    expect(api.preflight).toHaveBeenCalledWith({
      ...target,
      planId: "archive-flow",
      planVersion: 2,
      input: { id: 7 },
    }),
  );
  expect(
    await screen.findByText(
      "Structure checked. No business or model effects executed.",
    ),
  ).toBeTruthy();
});
it("does not offer preflight without validate permission", async () => {
  show(
    <PlanInspector
      target={target}
      source={source}
      current={source}
      canValidate={false}
    />,
  );
  await screen.findByText("Input contract");
  expect(screen.queryByRole("button", { name: "Check structure" })).toBeNull();
});
