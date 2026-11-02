import type { Diagram, DiagramEdge, DiagramNode, DiagramWhen } from "./graph";
import { connectionId } from "./connections";

export type AddRequest =
  | { kind: "insert"; edgeId: string }
  | {
      kind: "sibling";
      nodeId: string;
      side: "left" | "right" | "top" | "bottom";
    };
export type DraftStep = {
  id: string;
  capability?: string;
  version?: string;
  dependsOn?: string[];
  constants: Record<string, unknown>;
  input: Record<string, { from: string; field: string }>;
  agent?: {
    instruction: string;
    tools: string[];
    maxIterations: number;
    output?: Record<string, unknown>;
  };
  poll?: { doneField: string; maxSteps: number };
  when?: DiagramWhen;
};

export class DraftError extends Error {
  constructor(public code: string) {
    super(code);
  }
}

export function ancestors(diagram: Diagram, id: string): string[] {
  const found = new Set<string>();
  const visit = (target: string) => {
    for (const edge of diagram.edges.filter((edge) => edge.to === target)) {
      if (!found.has(edge.from)) {
        found.add(edge.from);
        visit(edge.from);
      }
    }
  };
  visit(id);
  return [...found].filter((source) => source !== "start" && source !== id);
}

export function applyAddition(
  diagram: Diagram,
  request: AddRequest,
  step: DraftStep,
  originalCase?: unknown,
): Diagram {
  if (
    diagram.nodes.some((node) => node.id === step.id) ||
    ["input", "start", "end"].includes(step.id) ||
    !/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(step.id)
  )
    throw new DraftError("idInvalid");
  if (
    diagram.nodes.filter((node) => node.kind !== "start" && node.kind !== "end")
      .length >= 64
  )
    throw new DraftError("stepLimit");
  const nodes = structuredClone(diagram.nodes);
  let edges = structuredClone(diagram.edges);
  const added: DiagramNode = {
    id: step.id,
    kind: step.agent ? "agent" : step.poll ? "poll" : "step",
    capability: step.capability,
    version: step.version,
    tools: step.agent?.tools,
    compiled: false,
    draftStep: structuredClone(step),
  };
  if (request.kind === "insert") {
    const edge = edges.find((edge) => connectionId(edge) === request.edgeId);
    if (!edge || edge.kind === "data") throw new DraftError("dataEdge");
    const target = nodes.find((node) => node.id === edge.to);
    if (!target) throw new DraftError("targetMissing");
    if (edge.kind !== "branch" && target.when)
      throw new DraftError("branchDependencies");
    const allowed = new Set([
      "input",
      edge.from,
      ...ancestors(diagram, edge.from),
    ]);
    if (Object.values(step.input).some((mapping) => !allowed.has(mapping.from)))
      throw new DraftError("mappingSource");
    edges = edges.filter((item) => connectionId(item) !== request.edgeId);
    if (edge.kind === "branch") {
      if (!target.when || target.when.from !== edge.from)
        throw new DraftError("conditionMissing");
      added.when = structuredClone(target.when);
      added.draftStep!.when = added.when;
      delete target.when;
      if (target.draftStep) delete target.draftStep.when;
    } else if (edge.from !== "start") added.dependsOn = [edge.from];
    if (target.kind !== "end") {
      target.compiled = false;
      target.dependsOn = [
        ...new Set([
          ...(target.dependsOn ?? []).filter((id) => id !== edge.from),
          step.id,
        ]),
      ];
      if (target.draftStep) target.draftStep.dependsOn = target.dependsOn;
    }
    edges.push(
      { from: edge.from, to: step.id, kind: edge.kind },
      { from: step.id, to: edge.to, kind: "control" },
    );
  } else {
    const anchor = nodes.find((node) => node.id === request.nodeId);
    if (!anchor || anchor.kind === "start" || anchor.kind === "end")
      throw new DraftError("targetMissing");
    const when = step.when;
    if (
      !when ||
      when.equals === null ||
      when.equals === undefined ||
      !when.field.trim() ||
      !ancestors(diagram, anchor.id).includes(when.from)
    )
      throw new DraftError("conditionMissing");
    if (
      anchor.when &&
      (anchor.when.from !== when.from || anchor.when.field !== when.field)
    )
      throw new DraftError("conditionGroup");
    if (!anchor.when) {
      if (originalCase === null || originalCase === undefined)
        throw new DraftError("originalCaseError");
      if (
        edges.some(
          (edge) =>
            edge.to === anchor.id &&
            edge.from !== "start" &&
            edge.from !== when.from,
        )
      )
        throw new DraftError("branchDependencies");
      anchor.when = {
        from: when.from,
        field: when.field,
        equals: originalCase,
      };
      // Preserve data mappings; only change the control dependency into a branch.
      edges = edges.filter(
        (edge) => !(edge.to === anchor.id && edge.kind !== "data"),
      );
      edges.push({ from: when.from, to: anchor.id, kind: "branch" });
      anchor.dependsOn = [];
      anchor.compiled = false;
      if (anchor.draftStep) {
        anchor.draftStep.when = anchor.when;
        anchor.draftStep.dependsOn = [];
      }
    }
    const valueKind = (value: unknown) =>
      Array.isArray(value) ? "array" : typeof value;
    if (valueKind(anchor.when!.equals) !== valueKind(when.equals))
      throw new DraftError("caseType");
    const canonical = (value: unknown): string =>
      JSON.stringify(value, (_key, item: unknown) =>
        item && typeof item === "object" && !Array.isArray(item)
          ? Object.fromEntries(
              Object.entries(item).sort(([a], [b]) => a.localeCompare(b)),
            )
          : item,
      );
    if (
      nodes.some(
        (node) =>
          node.when?.from === when.from &&
          node.when.field === when.field &&
          canonical(node.when.equals) === canonical(when.equals),
      )
    )
      throw new DraftError("duplicateCase");
    added.when = structuredClone(when);
    added.dependsOn = [];
    edges.push({ from: when.from, to: step.id, kind: "branch" });
    // Workflow END collects every branch. A sibling does not inherit another
    // branch's downstream business actions or its input mappings.
    edges.push({ from: step.id, to: "end", kind: "control" });
  }
  if (
    added.when &&
    Object.values(step.input).some(
      (mapping) =>
        mapping.from !== "input" && mapping.from !== added.when!.from,
    )
  )
    throw new DraftError("branchDependencies");
  for (const mapping of Object.values(step.input)) {
    const from = mapping.from === "input" ? "start" : mapping.from;
    if (!edges.some((edge) => edge.from === from && edge.to === step.id))
      edges.push({ from, to: step.id, kind: "data" });
  }
  added.draftStep!.dependsOn = added.dependsOn;
  const anchorIndex =
    request.kind === "sibling"
      ? nodes.findIndex((node) => node.id === request.nodeId)
      : -1;
  if (request.kind === "sibling")
    nodes.splice(
      anchorIndex +
        (request.side === "right" || request.side === "bottom" ? 1 : 0),
      0,
      added,
    );
  else nodes.push(added);
  return { ...diagram, nodes, edges };
}

export function applyRemoval(diagram: Diagram, id: string): Diagram {
  const removed = diagram.nodes.find((node) => node.id === id);
  if (!removed || removed.kind === "start" || removed.kind === "end")
    throw new DraftError("removeProtected");
  const remaining = diagram.nodes.filter((node) => node.id !== id);
  if (remaining.some((node) => node.when?.from === id))
    throw new DraftError("removeConditionSource");
  if (
    remaining.some((node) =>
      Object.values(node.draftStep?.input ?? {}).some(
        (mapping) => mapping.from === id,
      ),
    ) ||
    diagram.edges.some(
      (edge) => edge.from === id && edge.kind === "data" && edge.to !== "end",
    )
  )
    throw new DraftError("removeDataSource");

  const incoming = diagram.edges.filter(
    (edge) => edge.to === id && edge.kind !== "data",
  );
  const outgoing = diagram.edges.filter(
    (edge) => edge.from === id && edge.kind !== "data",
  );
  if (outgoing.some((edge) => edge.kind === "branch"))
    throw new DraftError("removeConditionSource");
  if (incoming.some((edge) => edge.kind === "branch") && !removed.when)
    throw new DraftError("removeAmbiguous");
  const predecessors = [
    ...new Set(
      incoming.length
        ? incoming.map((edge) => edge.from)
        : removed.dependsOn?.length
          ? removed.dependsOn
          : ["start"],
    ),
  ];
  const successors = [
    ...new Set(outgoing.length ? outgoing.map((edge) => edge.to) : ["end"]),
  ];
  const businessTargets = successors.filter((target) => target !== "end");
  const nodes = structuredClone(remaining);
  const edges = structuredClone(
    diagram.edges.filter((edge) => edge.from !== id && edge.to !== id),
  );

  if (removed.when && businessTargets.length) {
    // A predicate can move to one successor. Multiple targets would duplicate
    // a mutually exclusive case, and an existing predicate cannot be composed.
    if (
      businessTargets.length !== 1 ||
      predecessors.length !== 1 ||
      predecessors[0] !== removed.when.from
    )
      throw new DraftError("removeAmbiguous");
    const target = nodes.find((node) => node.id === businessTargets[0]);
    if (
      !target ||
      target.when ||
      edges.some(
        (edge) =>
          edge.to === target.id &&
          edge.from !== "start" &&
          edge.from !== removed.when!.from,
      )
    )
      throw new DraftError("removeAmbiguous");
    target.when = structuredClone(removed.when);
    if (target.draftStep) target.draftStep.when = target.when;
  }

  const reconnect = (edge: DiagramEdge) => {
    if (
      edge.from !== edge.to &&
      !edges.some((item) => connectionId(item) === connectionId(edge))
    )
      edges.push(edge);
  };
  for (const from of predecessors) {
    for (const to of successors) {
      // Deleting a terminal branch removes that alternative, rather than
      // inventing an unconditional bypass beside its surviving siblings.
      if (
        removed.when &&
        to === "end" &&
        (businessTargets.length ||
          edges.some((edge) => edge.from === from && edge.kind !== "data"))
      )
        continue;
      reconnect({
        from,
        to,
        kind: removed.when && to !== "end" ? "branch" : "control",
      });
    }
  }
  for (const node of nodes) {
    if (
      !successors.includes(node.id) &&
      !node.dependsOn?.includes(id) &&
      !node.draftStep?.dependsOn?.includes(id)
    )
      continue;
    if (node.kind === "end") continue;
    node.compiled = false;
    node.dependsOn = node.when
      ? []
      : [
          ...new Set([
            ...(node.dependsOn ?? node.draftStep?.dependsOn ?? []).filter(
              (source) => source !== id,
            ),
            ...predecessors.filter((source) => source !== "start"),
          ]),
        ];
    if (node.draftStep) node.draftStep.dependsOn = node.dependsOn;
  }
  return { ...diagram, nodes, edges };
}
