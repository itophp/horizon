import { beforeEach, expect, it, vi } from "vitest";
import { render, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import { changeLanguage, i18n } from "@/i18n";
import { buildWorkflow, WorkflowGraph } from "./graph";
import type { Diagram } from "./graph";
import { connectionId } from "./connections";

// X6 ships a CommonJS "main" inside a "type": "module" package, which Node-side
// ESM resolution rejects. The canvas itself is exercised in the browser, so the
// tests keep the projection real and stub the rendering engine.
const canvas = vi.hoisted(() => ({
  options: undefined as Record<string, unknown> | undefined,
  nodes: [] as string[],
  nodeMetadata: [] as Record<string, unknown>[],
  edges: [] as string[],
  disposed: 0,
  handlers: new Map<string, (args: { edge: { id: string } }) => void>(),
  metadata: [] as Record<string, unknown>[],
  attr: vi.fn(),
  layoutOptions: [] as Record<string, unknown>[],
}));

vi.mock("@antv/x6", () => ({
  Graph: class {
    constructor(options: Record<string, unknown>) {
      canvas.options = options;
    }
    addNode(node: { id: string }) {
      canvas.nodes.push(node.id);
      canvas.nodeMetadata.push(node);
    }
    addEdge(edge: { id: string }) {
      canvas.edges.push(edge.id);
      canvas.metadata.push(edge);
    }
    getCellById(id: string) {
      return canvas.edges.includes(id)
        ? { isNode: () => false, isEdge: () => true, attr: canvas.attr }
        : null;
    }
    on(event: string, handler: (args: { edge: { id: string } }) => void) {
      canvas.handlers.set(event, handler);
    }
    off(event: string) {
      canvas.handlers.delete(event);
    }
    zoomToFit() {}
    positionContent() {}
    dispose() {
      canvas.disposed += 1;
    }
  },
}));

vi.mock("@antv/x6-react-shape", () => ({ register: () => {} }));

vi.mock("@antv/layout", () => ({
  DagreLayout: class {
    constructor(options: Record<string, unknown>) {
      canvas.layoutOptions.push(options);
    }
    async execute() {}
    forEachNode() {}
  },
}));

const diagram: Diagram = {
  name: "demo",
  version: "1.0.0",
  hash: "0123456789abcdef",
  input: { properties: { title: { type: "string" } }, required: ["title"] },
  nodes: [
    { id: "start", kind: "start", compiled: true },
    {
      id: "create",
      kind: "step",
      capability: "example.record.create",
      version: "1.0.0",
      compiled: true,
    },
    {
      id: "review",
      kind: "agent",
      tools: ["example.record.get@1.0.0"],
      dependsOn: ["create"],
      compiled: true,
    },
    {
      id: "reject",
      kind: "step",
      capability: "example.record.archive",
      version: "1.0.0",
      when: { from: "review", field: "decision", equals: "reject" },
      compiled: true,
    },
    { id: "end", kind: "end", compiled: true },
  ],
  edges: [
    // Eino attaches the root input to every node as a data edge; the canvas must
    // drop those or the picture becomes a web of crossing lines.
    { from: "start", to: "create", kind: "data" },
    { from: "start", to: "review", kind: "data" },
    { from: "create", to: "review", kind: "control" },
    { from: "review", to: "reject", kind: "branch" },
    { from: "review", to: "end", kind: "control" },
    { from: "reject", to: "end", kind: "control" },
    { from: "create", to: "end", kind: "data" },
  ],
};

function show(node: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{node}</I18nextProvider>);
}

beforeEach(async () => {
  await changeLanguage("en", false);
  canvas.nodes = [];
  canvas.nodeMetadata = [];
  canvas.edges = [];
  canvas.options = undefined;
  canvas.disposed = 0;
  canvas.handlers.clear();
  canvas.metadata = [];
  canvas.attr.mockClear();
  canvas.layoutOptions = [];
});

it("offers native edge insertion and side tools without changing data mappings", () => {
  const onAdd = vi.fn();
  show(<WorkflowGraph diagram={diagram} onAdd={onAdd} />);
  expect(canvas.options?.interacting).toMatchObject({
    nodeMovable: false,
    edgeMovable: false,
    toolsAddable: true,
    useEdgeTools: true,
  });
  const addTools = vi.fn<(tool: { args: { onClick: () => void } }) => void>();
  const removeTools = vi.fn();
  const edge = { id: connectionId(diagram.edges[2]), addTools, removeTools };
  canvas.handlers.get("edge:mouseenter")?.({ edge });
  expect(addTools).toHaveBeenCalledTimes(1);
  addTools.mock.calls[0][0].args.onClick();
  expect(onAdd).toHaveBeenCalledWith({ kind: "insert", edgeId: edge.id });
  canvas.handlers.get("edge:mouseleave")?.({ edge });
  expect(removeTools).toHaveBeenCalled();
  const dataEdge = { ...edge, id: connectionId(diagram.edges[6]) };
  canvas.handlers.get("edge:mouseenter")?.({ edge: dataEdge });
  expect(addTools).toHaveBeenCalledTimes(1);
  const tools = canvas.nodeMetadata.find((node) => node.id === "review")
    ?.tools as { args: { onClick: () => void; offset: { x: number } } }[];
  expect(tools.map((tool) => tool.args.offset.x)).toEqual([-26, 26]);
  tools[0].args.onClick();
  expect(onAdd).toHaveBeenCalledWith({
    kind: "sibling",
    nodeId: "review",
    side: "left",
  });
  tools[1].args.onClick();
  expect(onAdd).toHaveBeenCalledWith({
    kind: "sibling",
    nodeId: "review",
    side: "right",
  });
});

it("projects steps and edges for the canvas", () => {
  const { nodes, edges } = buildWorkflow(
    diagram,
    new Set(["create"]),
    i18n.getFixedT("en", "orchestration"),
  );
  expect(nodes).toHaveLength(diagram.nodes.length);
  // Two start data edges are dropped, five real edges remain.
  expect(edges).toHaveLength(5);
  expect(edges.map((edge) => edge.id)).toEqual([
    JSON.stringify(["create", "review", "control"]),
    JSON.stringify(["review", "reject", "branch"]),
    JSON.stringify(["review", "end", "control"]),
    JSON.stringify(["reject", "end", "control"]),
    JSON.stringify(["create", "end", "data"]),
  ]);
  expect(edges[0].kind).toBe("control");
  expect(edges[1].kind).toBe("branch");
  expect(edges[4].kind).toBe("data");
  const review = nodes.find((node) => node.id === "review");
  expect(review?.data.kindLabel).toBe("Agent");
  expect(review?.data.subtitle).toBe("Agent node");
  expect(nodes.find((node) => node.id === "reject")?.data.condition).toBe(
    "= reject",
  );
  const create = nodes.find((node) => node.id === "create");
  expect(create?.data.done).toBe(true);
  expect(create?.data.compiled).toBe(true);
  expect(nodes.find((node) => node.id === "start")?.data.title).toBe("Input");
  expect(nodes.find((node) => node.id === "end")?.data.title).toBe("End");
});

it("keeps a declared-only node distinct", () => {
  const declared: Diagram = {
    ...diagram,
    nodes: diagram.nodes.map((node) =>
      node.id === "create" ? { ...node, compiled: false } : node,
    ),
  };
  const { nodes } = buildWorkflow(
    declared,
    new Set(),
    i18n.getFixedT("en", "orchestration"),
  );
  expect(nodes.find((node) => node.id === "create")?.data.compiled).toBe(false);
});

it("mounts a read-only canvas with the projected cells", () => {
  const { getByRole } = show(<WorkflowGraph diagram={diagram} />);
  expect(
    getByRole("group", { name: "Compiled graph of demo 1.0.0" }),
  ).toBeInTheDocument();
  expect(canvas.options?.interacting).toMatchObject({
    nodeMovable: false,
    edgeMovable: false,
    toolsAddable: false,
  });
  expect(canvas.nodes).toEqual(["start", "create", "review", "reject", "end"]);
  // The two start data edges never reach the canvas.
  expect(canvas.edges).toEqual([
    JSON.stringify(["create", "review", "control"]),
    JSON.stringify(["review", "reject", "branch"]),
    JSON.stringify(["review", "end", "control"]),
    JSON.stringify(["reject", "end", "control"]),
    JSON.stringify(["create", "end", "data"]),
  ]);
});

it("preserves the canvas when status and selection change", () => {
  const view = show(<WorkflowGraph diagram={diagram} />);
  view.rerender(
    <I18nextProvider i18n={i18n}>
      <WorkflowGraph diagram={diagram} done={["create"]} selected="review" />
    </I18nextProvider>,
  );
  expect(canvas.disposed).toBe(0);
  expect(canvas.nodes).toHaveLength(diagram.nodes.length);
  view.unmount();
  expect(canvas.disposed).toBe(1);
});

it("defaults to vertical and switches layout and routing while preserving edge selection", async () => {
  const user = userEvent.setup();
  const onSelectEdge = vi.fn();
  const edgeId = connectionId(diagram.edges[3]);
  const view = show(
    <WorkflowGraph
      diagram={diagram}
      selectedEdge={edgeId}
      onSelectEdge={onSelectEdge}
    />,
  );
  expect(view.getByRole("button", { name: "Vertical" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(canvas.layoutOptions.at(-1)?.rankdir).toBe("TB");
  expect(canvas.metadata.at(-1)).toMatchObject({
    source: { anchor: { name: "bottom" } },
    target: { anchor: { name: "top" } },
  });
  await user.click(view.getByRole("button", { name: "Horizontal" }));
  await waitFor(() => expect(canvas.layoutOptions.at(-1)?.rankdir).toBe("LR"));
  expect(view.getByRole("button", { name: "Horizontal" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(canvas.metadata.at(-1)).toMatchObject({
    source: { anchor: { name: "right" } },
    target: { anchor: { name: "left" } },
  });
  expect(canvas.attr).toHaveBeenLastCalledWith("line", expect.anything());
  expect(canvas.attr).toHaveBeenCalledWith("line", {
    class: "orchestration-edge is-branch is-selected",
    strokeWidth: 3,
  });
  canvas.handlers.get("edge:click")?.({ edge: { id: edgeId } });
  expect(onSelectEdge).toHaveBeenCalledWith(edgeId);
  await user.click(view.getByRole("button", { name: "Vertical" }));
  await waitFor(() => expect(canvas.layoutOptions.at(-1)?.rankdir).toBe("TB"));
});

it("labels branch connections and dispatches edge clicks without rebuilding the canvas", () => {
  const onSelectEdge = vi.fn();
  const edgeId = connectionId(diagram.edges[3]);
  const view = show(
    <WorkflowGraph diagram={diagram} onSelectEdge={onSelectEdge} />,
  );
  expect(canvas.metadata[1]).toMatchObject({
    labels: [
      {
        attrs: { label: { text: "decision = reject", pointerEvents: "auto" } },
      },
    ],
    attrs: { wrap: { strokeWidth: 16 } },
  });
  expect(canvas.metadata[0]).toMatchObject({ labels: [] });
  canvas.handlers.get("edge:click")?.({ edge: { id: edgeId } });
  expect(onSelectEdge).toHaveBeenCalledWith(edgeId);
  view.rerender(
    <I18nextProvider i18n={i18n}>
      <WorkflowGraph
        diagram={diagram}
        selectedEdge={edgeId}
        onSelectEdge={onSelectEdge}
      />
    </I18nextProvider>,
  );
  expect(canvas.attr).toHaveBeenCalledWith("line", {
    class: "orchestration-edge is-branch is-selected",
    strokeWidth: 3,
  });
  expect(canvas.disposed).toBe(0);
  view.unmount();
  expect(canvas.handlers.has("edge:click")).toBe(false);
});

it("places sibling tools above and below nodes in horizontal layout", async () => {
  const user = userEvent.setup();
  const onAdd = vi.fn();
  const view = show(<WorkflowGraph diagram={diagram} onAdd={onAdd} />);
  await user.click(view.getByRole("button", { name: "Horizontal" }));
  const tools = canvas.nodeMetadata
    .filter((node) => node.id === "review")
    .at(-1)?.tools as {
    args: {
      x: string;
      y: number | string;
      offset: { x: number; y: number };
      onClick: () => void;
    };
  }[];
  expect(
    tools.map((tool) => ({
      x: tool.args.x,
      y: tool.args.y,
      offset: tool.args.offset,
    })),
  ).toEqual([
    { x: "50%", y: 0, offset: { x: 0, y: -26 } },
    { x: "50%", y: "100%", offset: { x: 0, y: 26 } },
  ]);
  tools[0].args.onClick();
  expect(onAdd).toHaveBeenLastCalledWith({
    kind: "sibling",
    nodeId: "review",
    side: "top",
  });
  tools[1].args.onClick();
  expect(onAdd).toHaveBeenLastCalledWith({
    kind: "sibling",
    nodeId: "review",
    side: "bottom",
  });
});
