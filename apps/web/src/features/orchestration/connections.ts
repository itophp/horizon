import type { Diagram, DiagramEdge } from "./graph";

export function connectionId(edge: DiagramEdge) {
  return JSON.stringify([edge.from, edge.to, edge.kind]);
}

// A target's condition belongs only to the branch from its declared source.
// Other data/control edges entering or leaving that node have no predicate.
export function connectionCondition(diagram: Diagram, edge: DiagramEdge) {
  if (edge.kind !== "branch") return undefined;
  const when = diagram.nodes.find((node) => node.id === edge.to)?.when;
  return when?.from === edge.from ? when : undefined;
}

export function conditionValue(value: unknown) {
  return typeof value === "string" ? value : JSON.stringify(value);
}
