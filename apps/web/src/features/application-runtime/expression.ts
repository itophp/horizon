import type {
  RuntimeExpression,
  RuntimeMapping,
} from "@/api/generated/runtime";
export type Json =
  string | number | boolean | null | Json[] | { [key: string]: Json };
export type Scope = Record<string, unknown>;
const unsafe = new Set(["__proto__", "prototype", "constructor"]);
export function pathValue(value: unknown, path: string): unknown {
  if (!path) return value;
  for (const segment of path.split(".")) {
    if (unsafe.has(segment) || !segment) throw new Error("field_not_allowed");
    if (
      value === null ||
      typeof value !== "object" ||
      !Object.hasOwn(value, segment)
    )
      return undefined;
    value = (value as Record<string, unknown>)[segment];
  }
  return value;
}
export function evaluate(
  e: RuntimeExpression,
  scope: Scope,
  depth = 0,
): unknown {
  if (depth > 12) throw new Error("schema_invalid");
  const child = (i: number) => evaluate(e.items[i], scope, depth + 1);
  switch (e.kind) {
    case "":
      return true;
    case "literal":
      return JSON.parse(e.literal) as unknown;
    case "path":
      if (
        !/^(record|form|route|query|selection|event|context|data)\./.test(
          e.path,
        )
      )
        throw new Error("field_not_allowed");
      return pathValue(scope, e.path);
    case "object":
      return mapInput(e.fields, scope, depth + 1);
    case "array":
      return e.items.map((i) => evaluate(i, scope, depth + 1));
    case "eq":
      if (e.items.length !== 2) throw new Error("schema_invalid");
      return child(0) === child(1);
    case "in": {
      const values = child(1);
      if (!Array.isArray(values)) throw new Error("schema_invalid");
      return values.includes(child(0));
    }
    case "and":
      return e.items.every((i) => Boolean(evaluate(i, scope, depth + 1)));
    case "or":
      return e.items.some((i) => Boolean(evaluate(i, scope, depth + 1)));
    case "not":
      return !child(0);
    case "if":
      return child(0) ? child(1) : child(2);
    case "get":
      return pathValue(child(0), String(child(1)));
    case "encode":
      return encodeURIComponent(String(child(0) ?? ""));
    case "replace":
      return String(child(0) ?? "").replaceAll(
        String(child(1)),
        String(child(2)),
      );
    case "coalesce":
      for (const item of e.items ?? []) {
        const value = evaluate(item, scope, depth + 1);
        if (value !== null && value !== undefined) return value;
      }
      return undefined;
    case "concat":
      return (e.items ?? [])
        .map((item) => evaluate(item, scope, depth + 1) ?? "")
        .join("");
    case "count": {
      const value = child(0);
      return Array.isArray(value) ? value.length : 0;
    }
    case "find":
    case "filter": {
      const value = child(0);
      const rows: unknown[] = Array.isArray(value) ? value : [];
      const matches = (record: unknown) =>
        Boolean(
          evaluate(
            e.items[1],
            { ...scope, event: scope.record, record },
            depth + 1,
          ),
        );
      return e.kind === "find" ? rows.find(matches) : rows.filter(matches);
    }
    case "exists":
      return child(0) !== undefined && child(0) !== null;
    default:
      throw new Error("schema_invalid");
  }
}
export function mapInput(
  bindings: RuntimeMapping[],
  scope: Scope,
  depth = 0,
): Record<string, unknown> {
  const input: Record<string, unknown> = {};
  for (const b of bindings) {
    if (unsafe.has(b.target) || Object.hasOwn(input, b.target))
      throw new Error("field_not_allowed");
    const value = evaluate(b.value, scope, depth);
    if (value !== undefined) input[b.target] = value;
  }
  return input;
}
// Validate the exact Schema subset published by the Capability contract.
export function validateInput(value: unknown, raw: string): void {
  const schema = JSON.parse(raw) as unknown;
  validateSchema(value, schema);
}
function validateSchema(value: unknown, raw: unknown): void {
  if (!raw || typeof raw !== "object") throw new Error("schema_invalid");
  const s = raw as Record<string, unknown>;
  const fail = () => {
    throw new Error("invalid");
  };
  if (s.type === "object") {
    if (!value || typeof value !== "object" || Array.isArray(value))
      return fail();
    const object = value as Record<string, unknown>;
    const props = (s.properties ?? {}) as Record<string, unknown>;
    for (const key of (s.required ?? []) as string[])
      if (!Object.hasOwn(object, key)) fail();
    for (const [key, v] of Object.entries(object)) {
      if (unsafe.has(key)) fail();
      if (Object.hasOwn(props, key)) validateSchema(v, props[key]);
      else if (!s.additionalProperties) fail();
    }
  } else if (s.type === "array") {
    if (!Array.isArray(value)) return fail();
    if (
      (typeof s.minItems === "number" && value.length < s.minItems) ||
      (typeof s.maxItems === "number" && value.length > s.maxItems)
    )
      fail();
    for (const item of value) validateSchema(item, s.items);
  } else if (s.type === "string") {
    if (typeof value !== "string") return fail();
    const length = [...value].length;
    if (
      (typeof s.minLength === "number" && length < s.minLength) ||
      (typeof s.maxLength === "number" && length > s.maxLength)
    )
      fail();
    if (Array.isArray(s.enum) && !s.enum.includes(value)) fail();
  } else if (s.type === "integer" || s.type === "number") {
    if (
      typeof value !== "number" ||
      !Number.isFinite(value) ||
      (s.type === "integer" && !Number.isInteger(value))
    )
      return fail();
    if (
      (typeof s.minimum === "number" && value < s.minimum) ||
      (typeof s.maximum === "number" && value > s.maximum)
    )
      fail();
  } else if (s.type === "boolean") {
    if (typeof value !== "boolean") fail();
  } else fail();
}
