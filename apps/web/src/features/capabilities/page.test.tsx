import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import type { CapabilityContract } from "@/api/generated/models";
import { CapabilityPage } from "./page";

const api = vi.hoisted(() => ({
  definitions: vi.fn(),
  invoke: vi.fn(),
  events: vi.fn(),
  invocations: vi.fn(),
  approvals: vi.fn(),
  decide: vi.fn(),
  permissions: [] as string[],
}));

vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  capabilityDefinitions: api.definitions,
  capabilityInvoke: api.invoke,
  capabilityEvents: api.events,
  capabilityInvocations: api.invocations,
  capabilityApprovals: api.approvals,
  capabilityDecide: api.decide,
}));

vi.mock("@/features/administration/auth", () => ({
  useSession: () => ({
    tenantId: "tenant-a",
    activeDepartmentId: 1,
    permissions: api.permissions,
    account: { id: 4, fullName: "Ada", memberships: [] },
  }),
}));

const contracts: CapabilityContract[] = [
  {
    name: "example.record.list",
    version: "1.0.0",
    title: "List example records",
    description: "List records.",
    domain: "example",
    entity: "example_record",
    permission: "example:read",
    risk: "low",
    approval: "auto",
    idempotent: true,
    readOnly: true,
    executor: "go:example.list",
    tags: ["example"],
    successEvents: [],
    inputSchema: { type: "object" },
    outputSchema: { type: "object" },
  },
  {
    name: "example.record.archive",
    version: "1.0.0",
    title: "Archive example record",
    description: "Archive a record.",
    domain: "example",
    entity: "example_record",
    permission: "example:archive",
    risk: "high",
    approval: "confirm",
    idempotent: true,
    readOnly: false,
    executor: "go:example.archive",
    tags: ["example"],
    successEvents: ["example.record.archived"],
    inputSchema: { type: "object" },
    outputSchema: { type: "object" },
  },
];

const records = {
  invocationId: "inv_records",
  status: "succeeded" as const,
  data: {
    records: [
      { id: 7, title: "Line 3 changeover", status: "draft" },
      { id: 8, title: "Old checklist", status: "archived" },
    ],
  },
  eventIds: [],
  errorCode: "",
  traceId: "trace",
  replayed: false,
};

function show() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <CapabilityPage />
      </QueryClientProvider>
    </I18nextProvider>,
  );
}

beforeEach(async () => {
  await changeLanguage("en", false);
  api.permissions = ["example:read", "example:create", "example:archive"];
  api.definitions.mockResolvedValue({ status: 200, data: contracts });
  api.events.mockResolvedValue({
    status: 200,
    data: [
      {
        eventId: "evt_1",
        type: "example.record.created",
        aggregateType: "example_record",
        aggregateId: "7",
        payload: { recordId: 7 },
        invocationId: "inv_2",
        correlationId: "trace",
        causationId: "run:1",
        depth: 0,
        createdAt: "2026-10-31T02:00:00Z",
        aiStatus: "completed",
        aiPolicy: "example-record-review",
        aiRunId: 3,
      },
    ],
  });
  api.invocations.mockResolvedValue({
    status: 200,
    data: [
      {
        invocationId: "inv_2",
        capability: "example.record.create",
        version: "1.0.0",
        origin: "human-ui",
        status: "succeeded",
        input: { title: "Line 3 changeover" },
        inputDigest: "digest",
        result: { id: 7 },
        errorCode: "",
        approvalId: 0,
        correlationId: "trace",
        causationId: "",
        triggerDepth: 0,
        durationMs: 4,
        traceId: "trace",
        createdAt: "2026-10-31T02:00:00Z",
        finishedAt: "2026-10-31T02:00:00Z",
      },
    ],
  });
  api.approvals.mockResolvedValue({ status: 200, data: [] });
  api.invoke.mockImplementation((name: string) => {
    if (name === "example.record.list") {
      return Promise.resolve({ status: 200, data: records });
    }
    if (name === "example.record.create") {
      return Promise.resolve({
        status: 200,
        data: {
          invocationId: "inv_3",
          status: "succeeded",
          data: { id: 9, title: "New record", status: "draft" },
          eventIds: ["evt_2"],
          errorCode: "",
          traceId: "trace",
          replayed: false,
        },
      });
    }
    return Promise.resolve({
      status: 200,
      data: {
        invocationId: "inv_4",
        status: "succeeded",
        data: { id: 7, title: "Line 3 changeover", status: "archived" },
        eventIds: ["evt_3"],
        errorCode: "",
        traceId: "trace",
        replayed: false,
      },
    });
  });
});

it("renders records, events with AI status and the catalog", async () => {
  show();
  expect(await screen.findByText("Line 3 changeover")).toBeTruthy();
  expect(screen.getByText("example.record.created")).toBeTruthy();
  expect(screen.getByText("AI reviewed")).toBeTruthy();
  expect(screen.getByText("Archive example record")).toBeTruthy();
  expect(screen.getByText("example.record.archive@1.0.0")).toBeTruthy();
  // The entity tag renders the localized label, not the raw identifier.
  expect(screen.getAllByText("Example record").length).toBeGreaterThan(0);
  expect(screen.queryByText("example_record")).toBeNull();
});

it("creates a record through the capability gateway", async () => {
  show();
  const input = await screen.findByLabelText("Record title");
  await userEvent.type(input, "New record");
  await userEvent.click(screen.getByRole("button", { name: /Create record/ }));
  await waitFor(() =>
    expect(api.invoke).toHaveBeenCalledWith(
      "example.record.create",
      expect.objectContaining({
        input: { title: "New record" },
        idempotencyKey: expect.any(String),
      }),
    ),
  );
  expect(await screen.findByText(/Record #9 created/)).toBeTruthy();
});

it("archives with a stable idempotency key", async () => {
  show();
  const buttons = await screen.findAllByRole("button", { name: /Archive/ });
  await userEvent.click(buttons[0]);
  await waitFor(() =>
    expect(api.invoke).toHaveBeenCalledWith("example.record.archive", {
      input: { id: 7 },
      idempotencyKey: "archive:7",
    }),
  );
});

it("hides write actions and explains the missing grant", async () => {
  api.permissions = [];
  show();
  expect(
    await screen.findByText(/cannot use the capability console/i),
  ).toBeTruthy();
  expect(screen.queryByLabelText("Record title")).toBeNull();
});
