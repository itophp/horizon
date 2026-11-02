import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Graph, type Node, type Edge } from "@antv/x6";
import { register } from "@antv/x6-react-shape";
import { DagreLayout } from "@antv/layout";
import type { TFunction } from "i18next";
import { Trash2 } from "lucide-react";
import {
  connectionCondition,
  connectionId,
  conditionValue,
} from "./connections";
import type { AddRequest, DraftStep } from "./draft";

// These mirror the read-only projection built in ai/orchestration/diagram.go.
// compose.GraphInfo itself cannot cross the wire: its nodes carry Go runtime
// values (Instance, reflect.Type), so the engine projects it first.
export type DiagramWhen = {
  from: string;
  field: string;
  equals: unknown;
};

export type DiagramNode = {
  id: string;
  kind: "start" | "end" | "step" | "agent" | "poll";
  capability?: string;
  version?: string;
  dependsOn?: string[];
  tools?: string[];
  maxAttempts?: number;
  compensation?: string;
  // Present on a mutually exclusive alternative: it runs only when the source
  // step reports this value.
  when?: DiagramWhen;
  compiled: boolean;
  draftStep?: DraftStep;
};

export type DiagramEdge = {
  from: string;
  to: string;
  // branch wins over control/data: exclusivity is the fact a reader needs.
  kind: "control" | "data" | "branch";
};

export type Diagram = {
  name: string;
  version: string;
  hash: string;
  input?: {
    properties?: Record<string, unknown>;
    required?: string[];
  };
  nodes: DiagramNode[];
  edges: DiagramEdge[];
};

const NODE_W = 184;
const NODE_H = 60;
const SHAPE = "orchestration-card";
const EDGE_STYLE: Record<
  DiagramEdge["kind"],
  { stroke: string; width: number; dash?: string }
> = {
  control: { stroke: "#8b8b95", width: 1.5 },
  data: { stroke: "#a9a9b2", width: 1.5, dash: "4 3" },
  branch: { stroke: "#6f6f7a", width: 2, dash: "8 3" },
};

export type WorkflowNodeData = {
  title: string;
  subtitle: string;
  condition: string;
  kindLabel: string;
  kind: DiagramNode["kind"];
  done: boolean;
  compiled: boolean;
  selected?: boolean;
  onSelect?: () => void;
  onRemove?: () => void;
  removeLabel?: string;
};

function caseText(value: unknown) {
  return typeof value === "string" ? value : JSON.stringify(value);
}

function WorkflowNodeView({ node }: { node: Node }) {
  const data = node.getData<WorkflowNodeData>();
  if (!data) return null;
  const badge = data.condition || data.kindLabel;
  return (
    <div
      className="workflow-node-shell"
      style={{ width: NODE_W, height: NODE_H }}
    >
      <button
        type="button"
        aria-pressed={!!data.selected}
        onClick={data.onSelect}
        className={`orchestration-node${data.selected ? " is-selected" : ""} is-${data.kind}${data.done ? " is-done" : ""}${data.compiled ? "" : " is-declared"}`}
        // X6's foreignObject body does not give a reliable percentage chain, so
        // the card carries the same pixel size the layout engine was told.
        style={{ width: NODE_W, height: NODE_H }}
      >
        <div className="orchestration-node-head">
          <strong>{data.title}</strong>
          {!!badge && <span className="orchestration-node-kind">{badge}</span>}
        </div>
        {!!data.subtitle && <small>{data.subtitle}</small>}
      </button>
      {data.onRemove && (
        <button
          type="button"
          className="workflow-node-remove"
          aria-label={data.removeLabel}
          title={data.removeLabel}
          onClick={(event) => {
            event.stopPropagation();
            data.onRemove?.();
          }}
        >
          <Trash2 size={12} aria-hidden="true" />
        </button>
      )}
    </div>
  );
}

register({
  shape: SHAPE,
  width: NODE_W,
  height: NODE_H,
  component: WorkflowNodeView,
  effect: ["data"],
});

// buildWorkflow keeps the projection pure and testable. START's data edges are
// dropped on purpose: Eino attaches the root input to every node as data, which
// carries no ordering and turns the canvas into a haystack of crossing lines.
export function buildWorkflow(
  diagram: Diagram,
  done: ReadonlySet<string>,
  t: TFunction,
) {
  const nodes = diagram.nodes.map((node) => ({
    id: node.id,
    data: {
      title:
        node.kind === "start"
          ? t("graph.start")
          : node.kind === "end"
            ? t("graph.end")
            : node.id,
      subtitle:
        node.capability ??
        (node.kind === "agent"
          ? t("graph.agent")
          : node.kind === "poll"
            ? t("graph.poll")
            : ""),
      kindLabel:
        node.kind === "start" || node.kind === "end"
          ? ""
          : t(`kind.${node.kind}`),
      condition: node.when ? `= ${caseText(node.when.equals)}` : "",
      kind: node.kind,
      done: done.has(node.id),
      compiled: node.compiled,
    } satisfies WorkflowNodeData,
  }));
  const edges = diagram.edges
    .filter((edge) => !(edge.kind === "data" && edge.from === "start"))
    .map((edge) => ({
      id: connectionId(edge),
      source: edge.from,
      target: edge.to,
      kind: edge.kind,
      condition: connectionCondition(diagram, edge),
    }));
  return { nodes, edges };
}

// X6 runs imperatively: the graph is built once per definition, then only node
// data is refreshed so a 2s run poll never resets the viewport.
export function WorkflowGraph({
  diagram,
  done = [],
  selected,
  onSelect,
  selectedEdge,
  onSelectEdge,
  onAdd,
  direction: suppliedDirection,
  onDirectionChange,
  onRemove,
}: {
  diagram: Diagram;
  done?: string[];
  selected?: string;
  onSelect?: (id: string) => void;
  selectedEdge?: string;
  onSelectEdge?: (id: string) => void;
  onAdd?: (request: AddRequest) => void;
  direction?: "TB" | "LR";
  onDirectionChange?: (direction: "TB" | "LR") => void;
  onRemove?: (id: string) => void;
}) {
  const { t } = useTranslation("orchestration");
  const container = useRef<HTMLDivElement>(null);
  const graph = useRef<Graph | null>(null);
  const [layoutError, setLayoutError] = useState(false);
  const [localDirection, setLocalDirection] = useState<"TB" | "LR">("TB");
  const direction = suppliedDirection ?? localDirection;
  const setDirection = (next: "TB" | "LR") => {
    setLocalDirection(next);
    onDirectionChange?.(next);
  };
  const add = useRef(onAdd);
  useEffect(() => {
    add.current = onAdd;
  }, [onAdd]);
  const editable = !!onAdd;

  useEffect(() => {
    const host = container.current;
    if (!host) return;
    const canvas = new Graph({
      container: host,
      autoResize: true,
      background: { color: "transparent" },
      grid: { visible: false },
      // Global false also disables toolsAddable, preventing hover tools from mounting.
      interacting: {
        nodeMovable: false,
        edgeMovable: false,
        edgeLabelMovable: false,
        arrowheadMovable: false,
        vertexMovable: false,
        vertexAddable: false,
        vertexDeletable: false,
        magnetConnectable: false,
        toolsAddable: editable,
        useEdgeTools: editable,
      },
      panning: { enabled: true },
      mousewheel: {
        enabled: true,
        modifiers: ["ctrl", "meta"],
        minScale: 0.4,
        maxScale: 1.5,
      },
    });
    graph.current = canvas;
    let disposed = false;

    const model = buildWorkflow(diagram, new Set(), t);
    const markup = (label: string, side = false) => [
      {
        tagName: "g",
        attrs: {
          class: `workflow-add-tool${side ? " is-side" : ""}`,
          "aria-label": label,
        },
        children: [
          { tagName: "title", textContent: label },
          {
            tagName: "circle",
            attrs: { r: 30, fill: "transparent", stroke: "none" },
          },
          { tagName: "circle", attrs: { r: 11, class: "workflow-add-circle" } },
          {
            tagName: "path",
            attrs: {
              d: "M -5 0 L 5 0 M 0 -5 L 0 5",
              class: "workflow-add-plus",
              fill: "none",
              strokeWidth: 1.8,
            },
          },
        ],
      },
    ];
    for (const node of model.nodes) {
      canvas.addNode({
        id: node.id,
        shape: SHAPE,
        // The size must be explicit: the shape default is not applied when the
        // node is added by id, and a 1x1 bbox makes routing cut through cards.
        width: NODE_W,
        height: NODE_H,
        x: 0,
        y: 0,
        data: node.data,
        tools:
          editable && node.data.kind !== "start" && node.data.kind !== "end"
            ? (direction === "TB"
                ? (["left", "right"] as const)
                : (["top", "bottom"] as const)
              ).map((side) => ({
                name: "button",
                args: {
                  x:
                    direction === "TB" ? (side === "left" ? 0 : "100%") : "50%",
                  y: direction === "LR" ? (side === "top" ? 0 : "100%") : "50%",
                  offset: {
                    x: direction === "TB" ? (side === "left" ? -26 : 26) : 0,
                    y: direction === "LR" ? (side === "top" ? -26 : 26) : 0,
                  },
                  markup: markup(
                    t(
                      side === "left"
                        ? "editor.addLeft"
                        : side === "right"
                          ? "editor.addRight"
                          : side === "top"
                            ? "editor.addTop"
                            : "editor.addBottom",
                    ),
                    true,
                  ),
                  onClick: () =>
                    add.current?.({
                      kind: "sibling",
                      nodeId: node.id,
                      side,
                    }),
                },
              }))
            : [],
      });
    }

    if (editable) {
      canvas.on("edge:mouseenter", ({ edge }: { edge: Edge }) => {
        const item = diagram.edges.find(
          (item) => connectionId(item) === edge.id,
        );
        if (!item || item.kind === "data") return;
        edge.addTools(
          {
            name: "button",
            args: {
              distance: "50%",
              markup: markup(t("editor.insert")),
              onClick: () => add.current?.({ kind: "insert", edgeId: edge.id }),
            },
          },
          { local: true, reset: true },
        );
      });
      canvas.on("edge:mouseleave", ({ edge }: { edge: Edge }) =>
        edge.removeTools(),
      );
    }
    for (const edge of model.edges) {
      const style = EDGE_STYLE[edge.kind];
      canvas.addEdge({
        id: edge.id,
        source: {
          cell: edge.source,
          anchor: { name: direction === "TB" ? "bottom" : "right" },
        },
        target: {
          cell: edge.target,
          anchor: { name: direction === "TB" ? "top" : "left" },
        },
        router: {
          name: "manhattan",
          args: {
            padding: 12,
            startDirections: [direction === "TB" ? "bottom" : "right"],
            endDirections: [direction === "TB" ? "top" : "left"],
          },
        },
        connector: { name: "rounded", args: { radius: 8 } },
        labels: edge.condition
          ? [
              {
                position: { distance: 0.5, offset: -14 },
                attrs: {
                  label: {
                    text: t("viewer.branchLabel", {
                      field: edge.condition.field,
                      value: conditionValue(edge.condition.equals),
                    }),
                    class: "orchestration-edge-label",
                    fontSize: 11,
                    cursor: "pointer",
                    pointerEvents: "auto",
                  },
                  body: {
                    class: "orchestration-edge-label-body",
                    cursor: "pointer",
                    pointerEvents: "auto",
                  },
                },
              },
            ]
          : [],
        attrs: {
          wrap: { strokeWidth: 16, cursor: "pointer" },
          line: {
            class: `orchestration-edge is-${edge.kind}`,
            stroke: style.stroke,
            strokeWidth: style.width,
            strokeDasharray: style.dash,
            targetMarker: { name: "block", width: 10, height: 6 },
          },
        },
      });
    }

    void (async () => {
      const layout = new DagreLayout({
        rankdir: direction,
        nodesep: editable ? 88 : 32,
        ranksep: 88,
        nodeSize: [NODE_W, NODE_H],
      });
      await layout.execute({
        nodes: model.nodes.map((node) => ({ id: node.id })),
        edges: model.edges.map((edge) => ({
          id: edge.id,
          source: edge.source,
          target: edge.target,
        })),
      });
      if (disposed) return;
      layout.forEachNode((node) => {
        const cell = canvas.getCellById(String(node.id));
        if (cell?.isNode()) {
          cell.position(node.x - NODE_W / 2, node.y - NODE_H / 2);
        }
      });
      // A seven step flow is wider than the panel: fitting it without a floor
      // shrinks labels to nothing. Keep a readable scale and let the reader pan.
      canvas.zoomToFit({ padding: 24, maxScale: 1, minScale: 0.8 });
      canvas.positionContent(direction === "TB" ? "top" : "left", {
        padding: 32,
      });
    })().catch(() => {
      if (!disposed) setLayoutError(true);
    });

    return () => {
      disposed = true;
      canvas.dispose();
      graph.current = null;
    };
  }, [diagram, t, direction, editable]);

  useEffect(() => {
    const canvas = graph.current;
    if (!canvas) return;
    const select = ({ edge }: { edge: { id: string } }) =>
      onSelectEdge?.(edge.id);
    canvas.on("edge:click", select);
    return () => {
      canvas.off("edge:click", select);
    };
  }, [diagram, t, direction, onSelectEdge]);

  useEffect(() => {
    for (const edge of diagram.edges) {
      const id = connectionId(edge);
      const cell = graph.current?.getCellById(id);
      if (cell?.isEdge())
        cell.attr("line", {
          class: `orchestration-edge is-${edge.kind}${selectedEdge === id ? " is-selected" : ""}`,
          strokeWidth: selectedEdge === id ? 3 : EDGE_STYLE[edge.kind].width,
        });
    }
  }, [diagram, t, direction, selectedEdge]);

  useEffect(() => {
    const finished = new Set(done);
    for (const node of diagram.nodes) {
      const cell = graph.current?.getCellById(node.id);
      if (cell?.isNode())
        cell.setData({
          done: finished.has(node.id),
          selected: selected === node.id,
          onSelect: () => onSelect?.(node.id),
          onRemove:
            onRemove && node.kind !== "start" && node.kind !== "end"
              ? () => onRemove(node.id)
              : undefined,
          removeLabel: t("editor.remove"),
        });
    }
  }, [diagram, done, selected, onSelect, onRemove, t, direction]);

  return (
    <div className="orchestration-canvas">
      <div
        className="orchestration-graph"
        role="group"
        aria-label={t("graph.label", {
          name: diagram.name,
          version: diagram.version,
        })}
        ref={container}
      />
      {layoutError && (
        <p role="alert" className="orchestration-problem">
          {t("graph.layoutError")}
        </p>
      )}
      <div className="orchestration-toolbar">
        <div
          role="group"
          aria-label={t("graph.direction")}
          className="orchestration-directions"
        >
          <button
            type="button"
            className="orchestration-fit"
            aria-pressed={direction === "TB"}
            onClick={() => setDirection("TB")}
          >
            {t("graph.vertical")}
          </button>
          <button
            type="button"
            className="orchestration-fit"
            aria-pressed={direction === "LR"}
            onClick={() => setDirection("LR")}
          >
            {t("graph.horizontal")}
          </button>
        </div>
        <button
          type="button"
          className="orchestration-fit"
          onClick={() =>
            graph.current?.zoom(0.2, { minScale: 0.2, maxScale: 2 })
          }
        >
          {t("graph.zoomIn")}
        </button>
        <button
          type="button"
          className="orchestration-fit"
          onClick={() =>
            graph.current?.zoom(-0.2, { minScale: 0.2, maxScale: 2 })
          }
        >
          {t("graph.zoomOut")}
        </button>
        <button
          type="button"
          className="orchestration-fit"
          onClick={() => graph.current?.zoomToFit({ padding: 16, maxScale: 1 })}
        >
          {t("graph.fit")}
        </button>
      </div>
      <span className="orchestration-hint">{t("graph.hint")}</span>
    </div>
  );
}
