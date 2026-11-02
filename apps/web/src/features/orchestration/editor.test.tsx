import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { WorkflowViewer } from "./viewer";
import type { AddRequest } from "./draft";

vi.mock("@/features/capabilities/shared", () => ({
  useCapabilityKey: () => ["test-definitions"],
}));
vi.mock("@/api/generated/baseline", async (original) => ({
  ...(await original<typeof import("@/api/generated/baseline")>()),
  capabilityDefinitions: async () => ({
    status: 200,
    data: [
      {
        name: "example.record.get",
        version: "1.0.0",
        layer: "atomic",
        readOnly: true,
      },
    ],
  }),
}));
vi.mock("./graph", () => ({
  WorkflowGraph: ({ onAdd }: { onAdd?: (request: AddRequest) => void }) => (
    <>
      <button
        type="button"
        onClick={() =>
          onAdd?.({
            kind: "insert",
            edgeId: JSON.stringify(["review", "approve", "branch"]),
          })
        }
      >
        Insert on connection
      </button>
      <button
        type="button"
        onClick={() =>
          onAdd?.({ kind: "sibling", nodeId: "approve", side: "right" })
        }
      >
        Add peer
      </button>
    </>
  ),
}));

beforeEach(async () => {
  await changeLanguage("en", false);
});
function show() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <I18nextProvider i18n={i18n}>
        <WorkflowViewer
          diagram={{
            name: "demo",
            version: "1",
            hash: "base",
            nodes: [
              { id: "start", kind: "start", compiled: true },
              { id: "review", kind: "step", compiled: true },
              {
                id: "approve",
                kind: "step",
                compiled: true,
                when: { from: "review", field: "decision", equals: "approve" },
              },
              { id: "end", kind: "end", compiled: true },
            ],
            edges: [
              { from: "review", to: "approve", kind: "branch" },
              { from: "approve", to: "end", kind: "control" },
            ],
          }}
        />
      </I18nextProvider>
    </QueryClientProvider>,
  );
}

it("inserts a configured node into a local draft and can undo it", async () => {
  show();
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Insert on connection" }),
  );
  const dialog = screen.getByRole("dialog");
  await user.type(
    within(dialog).getByRole("textbox", { name: "Step ID" }),
    "audit",
  );
  await user.selectOptions(
    within(dialog).getByRole("combobox", { name: "Capability and version" }),
    await screen.findByRole("option", { name: "example.record.get@1.0.0" }),
  );
  await user.click(
    within(dialog).getByRole("button", { name: "Add to draft" }),
  );
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "Selected node" })).toHaveValue(
    "audit",
  );
  expect(screen.getByText(/Unpublished local draft/)).toBeInTheDocument();
  expect(
    screen.getByText(/"capability": "example.record.get"/),
  ).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Remove node" }));
  expect(
    screen.queryByRole("option", { name: "audit" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Undo change" }));
  expect(screen.getByRole("option", { name: "audit" })).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Undo change" }));
  expect(
    screen.queryByRole("option", { name: "audit" }),
  ).not.toBeInTheDocument();
});

it("requires a distinct JSON when value for a sibling and cancels without modifying the graph", async () => {
  show();
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Add peer" }));
  const dialog = screen.getByRole("dialog");
  await user.type(
    within(dialog).getByRole("textbox", { name: "Step ID" }),
    "reject",
  );
  await user.selectOptions(
    within(dialog).getByRole("combobox", { name: "Capability and version" }),
    await screen.findByRole("option", { name: "example.record.get@1.0.0" }),
  );
  await user.type(
    within(dialog).getByRole("textbox", { name: "Comparison value" }),
    '"approve"',
  );
  await user.click(
    within(dialog).getByRole("button", { name: "Add to draft" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "This branch comparison value already exists.",
  );
  await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
  expect(
    screen.queryByRole("option", { name: "reject" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Undo change" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Add peer" }));
  await user.type(screen.getByRole("textbox", { name: "Step ID" }), "reject");
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Capability and version" }),
    await screen.findByRole("option", { name: "example.record.get@1.0.0" }),
  );
  await user.type(
    screen.getByRole("textbox", { name: "Comparison value" }),
    '"reject"',
  );
  await user.click(screen.getByRole("button", { name: "Add to draft" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "Selected node" })).toHaveValue(
    "reject",
  );
});

it("configures an Agent node using the registered tool allowlist", async () => {
  show();
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Insert on connection" }),
  );
  await user.type(
    screen.getByRole("textbox", { name: "Step ID" }),
    "investigate",
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Node type" }),
    "agent",
  );
  await user.type(
    screen.getByRole("textbox", { name: "Agent instruction" }),
    "Review the record",
  );
  await user.selectOptions(
    screen.getByRole("listbox", { name: "Allowed tools" }),
    await screen.findByRole("option", { name: "example.record.get@1.0.0" }),
  );
  await user.click(screen.getByRole("button", { name: "Add to draft" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: "Selected node" })).toHaveValue(
    "investigate",
  );
  expect(
    screen.getByText(/"instruction": "Review the record"/),
  ).toBeInTheDocument();
});

it("keeps a condition source in place and explains why removal is blocked", async () => {
  show();
  const user = userEvent.setup();
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Selected node" }),
    "review",
  );
  await user.click(screen.getByRole("button", { name: "Remove node" }));
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Other branches use this node as their condition source.",
  );
  expect(screen.getByRole("combobox", { name: "Selected node" })).toHaveValue(
    "review",
  );
  expect(
    screen.queryByRole("button", { name: "Undo change" }),
  ).not.toBeInTheDocument();
});

it("rejects malformed mapping JSON before creating a polling node", async () => {
  show();
  const user = userEvent.setup();
  await user.click(
    screen.getByRole("button", { name: "Insert on connection" }),
  );
  await user.type(
    screen.getByRole("textbox", { name: "Step ID" }),
    "waitReady",
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Node type" }),
    "poll",
  );
  await user.selectOptions(
    screen.getByRole("combobox", { name: "Capability and version" }),
    await screen.findByRole("option", { name: "example.record.get@1.0.0" }),
  );
  await user.type(
    screen.getByRole("textbox", { name: "Polling completion field" }),
    "ready",
  );
  const mapping = screen.getByRole("textbox", { name: "Input mappings JSON" });
  await user.clear(mapping);
  await user.type(mapping, "invalid");
  await user.click(screen.getByRole("button", { name: "Add to draft" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Enter valid JSON objects",
  );
  await user.clear(mapping);
  await user.type(mapping, "{{}");
  await user.click(screen.getByRole("button", { name: "Add to draft" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(screen.getByText(/"doneField": "ready"/)).toBeInTheDocument();
});
