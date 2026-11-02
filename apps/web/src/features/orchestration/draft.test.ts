import { expect, it } from "vitest";
import { applyAddition, applyRemoval, type DraftStep } from "./draft";
import { connectionId } from "./connections";
import type { Diagram } from "./graph";

const base: Diagram = {
  name: "demo",
  version: "1",
  hash: "registered",
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
    { from: "review", to: "approve", kind: "data" },
    { from: "approve", to: "end", kind: "control" },
  ],
};
const step = (): DraftStep => ({
  id: "newStep",
  capability: "example.record.get",
  version: "1.0.0",
  constants: {},
  input: {},
});

it("removes an inserted branch node and restores the original condition and connection", () => {
  const inserted = applyAddition(
    base,
    { kind: "insert", edgeId: connectionId(base.edges[0]) },
    step(),
  );
  const removed = applyRemoval(inserted, "newStep");
  expect(removed.nodes.some((node) => node.id === "newStep")).toBe(false);
  expect(removed.nodes.find((node) => node.id === "approve")?.when).toEqual(
    base.nodes[2].when,
  );
  expect(removed.edges).toContainEqual({
    from: "review",
    to: "approve",
    kind: "branch",
  });
  expect(removed.edges).toContainEqual({
    from: "review",
    to: "approve",
    kind: "data",
  });
  expect(
    removed.nodes.find((node) => node.id === "approve")?.dependsOn,
  ).toEqual([]);
  expect(inserted.nodes.some((node) => node.id === "newStep")).toBe(true);
});

it("reconnects multiple control predecessors and successors without duplicate edges", () => {
  const diagram: Diagram = {
    name: "multi",
    version: "1",
    hash: "base",
    nodes: ["start", "a", "b", "middle", "c", "d", "end"].map((id) => ({
      id,
      kind: id === "start" ? "start" : id === "end" ? "end" : "step",
      compiled: true,
      dependsOn: id === "c" ? ["middle", "a"] : undefined,
    })),
    edges: [
      { from: "a", to: "middle", kind: "control" },
      { from: "b", to: "middle", kind: "control" },
      { from: "middle", to: "c", kind: "control" },
      { from: "middle", to: "d", kind: "control" },
      { from: "a", to: "c", kind: "control" },
    ],
  };
  const result = applyRemoval(diagram, "middle");
  expect(result.edges).toHaveLength(4);
  for (const from of ["a", "b"])
    for (const to of ["c", "d"])
      expect(result.edges).toContainEqual({ from, to, kind: "control" });
  expect(result.nodes.find((node) => node.id === "c")?.dependsOn).toEqual([
    "a",
    "b",
  ]);
  expect(
    result.edges.some((edge) => edge.from === "middle" || edge.to === "middle"),
  ).toBe(false);
});

it("removes a branch leaf without adding an unconditional bypass beside surviving branches", () => {
  const added = applyAddition(
    base,
    { kind: "sibling", nodeId: "approve", side: "right" },
    {
      ...step(),
      when: { from: "review", field: "decision", equals: "reject" },
    },
  );
  const result = applyRemoval(added, "newStep");
  expect(result.edges).toEqual(base.edges);
  expect(result.edges).not.toContainEqual({
    from: "review",
    to: "end",
    kind: "control",
  });
});

it("refuses to remove start/end, condition sources or referenced data outputs", () => {
  expect(() => applyRemoval(base, "start")).toThrow("removeProtected");
  expect(() => applyRemoval(base, "end")).toThrow("removeProtected");
  expect(() => applyRemoval(base, "review")).toThrow("removeConditionSource");
  const data: Diagram = {
    ...base,
    nodes: base.nodes.map((node) => ({ ...node, when: undefined })),
  };
  expect(() => applyRemoval(data, "review")).toThrow("removeDataSource");
});

it("inserts into a branch, preserves its condition and existing data mapping, and does not mutate the source", () => {
  const updated = applyAddition(
    base,
    { kind: "insert", edgeId: connectionId(base.edges[0]) },
    step(),
  );
  const added = updated.nodes.find((node) => node.id === "newStep");
  expect(added?.when).toEqual(base.nodes[2].when);
  expect(added?.draftStep?.when).toEqual(base.nodes[2].when);
  expect(
    updated.nodes.find((node) => node.id === "approve")?.when,
  ).toBeUndefined();
  expect(
    updated.nodes.find((node) => node.id === "approve")?.dependsOn,
  ).toEqual(["newStep"]);
  expect(updated.edges).toContainEqual({
    from: "review",
    to: "approve",
    kind: "data",
  });
  expect(updated.edges).toContainEqual({
    from: "newStep",
    to: "approve",
    kind: "control",
  });
  expect(base.nodes[2].when?.equals).toBe("approve");
  expect(base.nodes).toHaveLength(4);
});

it("adds a mutually exclusive sibling and leaves other branches intact", () => {
  const added = {
    ...step(),
    when: { from: "review", field: "decision", equals: "reject" },
  };
  const updated = applyAddition(
    base,
    { kind: "sibling", nodeId: "approve", side: "left" },
    added,
  );
  expect(updated.nodes.map((node) => node.id)).toEqual([
    "start",
    "review",
    "newStep",
    "approve",
    "end",
  ]);
  expect(updated.edges).toContainEqual({
    from: "review",
    to: "newStep",
    kind: "branch",
  });
  expect(updated.edges).toContainEqual({
    from: "newStep",
    to: "end",
    kind: "control",
  });
  expect(updated.nodes.find((node) => node.id === "approve")?.when).toEqual(
    base.nodes[2].when,
  );
  expect(() =>
    applyAddition(
      base,
      { kind: "sibling", nodeId: "approve", side: "right" },
      { ...added, when: { ...added.when, equals: "approve" } },
    ),
  ).toThrow("duplicateCase");
  expect(() =>
    applyAddition(
      base,
      { kind: "sibling", nodeId: "approve", side: "right" },
      { ...added, when: { ...added.when, equals: true } },
    ),
  ).toThrow("caseType");
});

it("requires both case values when converting an unconditional step to sibling branches", () => {
  const plain: Diagram = {
    ...base,
    nodes: base.nodes.map((node) => ({ ...node, when: undefined })),
    edges: base.edges.map((edge) =>
      edge.kind === "branch" ? { ...edge, kind: "control" } : edge,
    ),
  };
  const added = {
    ...step(),
    when: { from: "review", field: "decision", equals: false },
  };
  expect(() =>
    applyAddition(
      plain,
      { kind: "sibling", nodeId: "approve", side: "right" },
      added,
    ),
  ).toThrow("originalCaseError");
  const updated = applyAddition(
    plain,
    { kind: "sibling", nodeId: "approve", side: "right" },
    added,
    true,
  );
  expect(
    updated.nodes.find((node) => node.id === "approve")?.when?.equals,
  ).toBe(true);
  expect(
    updated.nodes.find((node) => node.id === "newStep")?.when?.equals,
  ).toBe(false);
});

it("rejects duplicate IDs, invalid condition sources, data-edge insertions and future input references", () => {
  expect(() =>
    applyAddition(
      base,
      { kind: "insert", edgeId: connectionId(base.edges[1]) },
      step(),
    ),
  ).toThrow("dataEdge");
  expect(() =>
    applyAddition(
      base,
      { kind: "insert", edgeId: connectionId(base.edges[0]) },
      { ...step(), id: "approve" },
    ),
  ).toThrow("idInvalid");
  expect(() =>
    applyAddition(
      base,
      { kind: "sibling", nodeId: "approve", side: "left" },
      { ...step(), when: { from: "approve", field: "decision", equals: 0 } },
    ),
  ).toThrow("conditionMissing");
  expect(() =>
    applyAddition(
      base,
      { kind: "insert", edgeId: connectionId(base.edges[0]) },
      { ...step(), input: { value: { from: "approve", field: "value" } } },
    ),
  ).toThrow("mappingSource");
});
