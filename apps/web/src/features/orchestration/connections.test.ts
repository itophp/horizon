import { expect, it } from "vitest";
import {
  connectionCondition,
  connectionId,
  conditionValue,
} from "./connections";
import type { Diagram } from "./graph";

it("attaches a when predicate only to its matching branch edge", () => {
  const when = { from: "review", field: "decision", equals: false };
  const diagram: Diagram = {
    name: "demo",
    version: "1",
    hash: "hash",
    nodes: [{ id: "publish", kind: "step", compiled: true, when }],
    edges: [],
  };
  expect(
    connectionCondition(diagram, {
      from: "review",
      to: "publish",
      kind: "branch",
    }),
  ).toEqual(when);
  expect(
    connectionCondition(diagram, {
      from: "other",
      to: "publish",
      kind: "branch",
    }),
  ).toBeUndefined();
  expect(
    connectionCondition(diagram, {
      from: "review",
      to: "publish",
      kind: "data",
    }),
  ).toBeUndefined();
  expect(
    connectionCondition(diagram, {
      from: "review",
      to: "missing",
      kind: "branch",
    }),
  ).toBeUndefined();
  expect(conditionValue(false)).toBe("false");
  expect(conditionValue(0)).toBe("0");
  expect(conditionValue(null)).toBe("null");
});

it("distinguishes control and data edges between the same nodes", () => {
  expect(connectionId({ from: "a", to: "b", kind: "control" })).not.toBe(
    connectionId({ from: "a", to: "b", kind: "data" }),
  );
});
