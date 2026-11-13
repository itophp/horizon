import {
  keepPreviousData,
  useQueries,
  useQueryClient,
} from "@tanstack/react-query";
import { readSystemData } from "./system-data";
import { runtimeRead } from "@/lib/capabilities";
import { evaluate, validateInput, pathValue } from "../expression";
import type {
  ApplicationV3,
  DataSpec,
  Expression,
  Mapping,
  PageV3,
  ProtocolNode,
  ProtocolProvider,
} from "./types";

export type DataMap = Record<string, unknown>;
export type ProviderMap = Map<string, ProtocolProvider>;
export type ProtocolScope = Record<string, unknown>;

export interface DataOccurrence {
  key: string;
  path: string;
  spec: DataSpec;
}

export type BindingState = {
  pending: boolean;
  placeholder?: boolean;
  error: unknown;
  refetch: () => void;
};
export interface ProtocolDataState {
  bindings: Record<string, BindingState>;
  values: Record<string, unknown>;
  pending: boolean;
  error: unknown;
  refetch: () => Promise<void>;
}

const unsafeKeys = new Set(["__proto__", "prototype", "constructor"]);

export function collectDataSpecs(
  node: ProtocolNode | undefined,
  parentPath = "",
): DataOccurrence[] {
  if (node?.type !== "layout" || !node.layout) return [];
  const layoutPath = parentPath;
  const result: DataOccurrence[] = [];
  node.layout.data?.forEach((spec, index) => {
    result.push({
      key: `${layoutPath}:${index}:${spec.id}`,
      path: layoutPath,
      spec,
    });
  });
  node.layout.children?.forEach((child, index) => {
    if (child.component?.content) {
      result.push(
        ...collectDataSpecs(
          { type: "layout", layout: child.component.content },
          `${layoutPath}/component:${child.component.id}`,
        ),
      );
    }
    if (child.type !== "layout" || !child.layout) return;
    const childPath = `${layoutPath}/${index}:${child.layout.id}`;
    result.push(...collectDataSpecs(child, childPath));
  });
  return result;
}

function evaluateExpression(
  expression: Expression | undefined,
  scope: ProtocolScope,
  depth = 0,
): unknown {
  return expression?.kind ? evaluate(expression, scope, depth) : undefined;
}

function mapProtocolInput(
  mappings: Mapping[],
  scope: ProtocolScope,
  depth = 0,
): Record<string, unknown> {
  const input: Record<string, unknown> = {};
  for (const mapping of mappings) {
    if (unsafeKeys.has(mapping.target) || Object.hasOwn(input, mapping.target))
      throw new Error("field_not_allowed");
    const value = evaluateExpression(mapping.value, scope, depth);
    if (value !== undefined) input[mapping.target] = value;
  }
  return input;
}

function buildInput(
  mappings: Mapping[] | undefined,
  scope: ProtocolScope,
): Record<string, unknown> {
  return mapProtocolInput(mappings ?? [], scope);
}

type JsonSchema = {
  type?: string;
  properties?: Record<string, JsonSchema>;
  items?: JsonSchema;
};

function coerceSchemaValue(
  value: unknown,
  schema: JsonSchema | undefined,
): unknown {
  if (!schema?.type) return value;
  if (schema.type === "integer" || schema.type === "number") {
    if (typeof value === "string" && value.trim() !== "") {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : value;
    }
    return value;
  }
  if (schema.type === "boolean" && (value === "true" || value === "false"))
    return value === "true";
  if (
    schema.type === "object" &&
    value &&
    typeof value === "object" &&
    !Array.isArray(value)
  ) {
    const source = value as Record<string, unknown>;
    const result: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(source)) {
      if (unsafeKeys.has(key)) continue;
      result[key] = coerceSchemaValue(item, schema.properties?.[key]);
    }
    return result;
  }
  if (schema.type === "array" && Array.isArray(value))
    return value.map((item) => coerceSchemaValue(item, schema.items));
  return value;
}

export function coerceProviderInput(
  value: Record<string, unknown>,
  rawSchema: string,
) {
  let schema: JsonSchema | undefined;
  try {
    schema = JSON.parse(rawSchema) as JsonSchema;
  } catch {
    schema = undefined;
  }
  return coerceSchemaValue(value, schema) as Record<string, unknown>;
}

async function readOccurrence(
  application: ApplicationV3,
  page: PageV3,
  occurrence: DataOccurrence,
  provider: ProtocolProvider,
  scope: ProtocolScope,
  signal: AbortSignal,
): Promise<unknown> {
  let input = buildInput(
    occurrence.spec.input?.length ? occurrence.spec.input : provider.input,
    scope,
  );
  if (provider.pageSize > 0 && input.pageSize === undefined)
    input.pageSize = provider.pageSize;
  if (provider.inputSchema) {
    input = coerceProviderInput(input, provider.inputSchema);
    validateInput(input, provider.inputSchema);
  }
  const raw =
    !provider.adapter || provider.adapter === "capability"
      ? await runtimeRead(
          provider.capability.name,
          input,
          signal,
          provider.capability.version,
        )
      : await readSystemData(provider.adapter, input, signal);
  if (provider.outputSchema) validateInput(raw, provider.outputSchema);
  const value = provider.resultPath ? pathValue(raw, provider.resultPath) : raw;
  if (!occurrence.spec.transform || occurrence.spec.transform.kind === "")
    return value;
  return evaluateExpression(occurrence.spec.transform, {
    ...scope,
    data: value,
  });
}

export function useProtocolData(options: {
  application: ApplicationV3;
  page: PageV3;
  occurrences: DataOccurrence[];
  providers: ProviderMap;
  scope: ProtocolScope;
  identity: string;
}): ProtocolDataState {
  const { application, page, occurrences, providers, scope, identity } =
    options;
  const queryOccurrences = occurrences.filter(
    (occurrence) => occurrence.spec.provider,
  );
  const client = useQueryClient();
  const resolved = queryOccurrences.map((occurrence) => {
    const provider = occurrence.spec.provider
      ? providers.get(occurrence.spec.provider)
      : undefined;
    const input = buildInput(
      occurrence.spec.input?.length ? occurrence.spec.input : provider?.input,
      scope,
    );
    return {
      occurrence,
      provider,
      queryKey: [
        "protocol-v3",
        identity,
        application.id,
        application.version,
        page.id,
        occurrence.key,
        occurrence.spec.provider,
        input,
      ],
    };
  });
  const queryOptions = resolved.map(({ occurrence, provider, queryKey }) => {
    return {
      queryKey,
      enabled:
        Boolean(provider) &&
        (!occurrence.spec.enabledWhen?.kind ||
          Boolean(evaluateExpression(occurrence.spec.enabledWhen, scope))),
      queryFn: async ({ signal }: { signal: AbortSignal }) => {
        if (!provider) throw new Error("provider_not_found");
        return readOccurrence(
          application,
          page,
          occurrence,
          provider,
          scope,
          signal,
        );
      },
      staleTime:
        occurrence.spec.staleMs ??
        provider?.staleTimeMs ??
        provider?.pollInterval ??
        15000,
      refetchInterval: (query: { state: { data: unknown } }) =>
        !occurrence.spec.pollWhen?.kind ||
        evaluateExpression(occurrence.spec.pollWhen, {
          ...scope,
          data: {
            ...(scope.data as Record<string, unknown>),
            ...Object.fromEntries(
              resolved.map(({ occurrence, queryKey }) => [
                occurrence.spec.id,
                client.getQueryData(queryKey),
              ]),
            ),
          },
          record: query.state.data,
        })
          ? (occurrence.spec.pollMs ?? provider?.pollInterval ?? false)
          : false,
      retry: false,
      placeholderData: keepPreviousData,
    };
  });
  const queries = useQueries({ queries: queryOptions });

  const bindings: Record<string, BindingState> = {};
  const values: Record<string, unknown> = {};
  queryOccurrences.forEach((occurrence, index) => {
    const query = queries[index];
    if (
      queryOptions[index]?.enabled &&
      (occurrence.spec.nonBlocking || query.isPlaceholderData)
    ) {
      bindings[occurrence.key] = {
        pending: occurrence.spec.nonBlocking === true && query.isPending,
        placeholder: query.isPlaceholderData,
        error: occurrence.spec.nonBlocking ? query.error : null,
        refetch: () => {
          void query.refetch();
        },
      };
    }
    values[occurrence.key] =
      (queryOptions[index]?.enabled ? queries[index]?.data : undefined) ??
      occurrence.spec.initial ??
      null;
  });

  const blocking = queries.filter(
    (_, index) =>
      queryOptions[index]?.enabled &&
      !queryOccurrences[index]?.spec.nonBlocking,
  );
  const pending = blocking.some((query) => query.isPending);
  const failed = blocking.find((query) => query.isError);
  return {
    values,
    bindings,
    pending,
    error: failed?.error,
    refetch: async () => {
      await Promise.all(
        queries.flatMap((query, index) =>
          queryOptions[index]?.enabled ? [query.refetch()] : [],
        ),
      );
    },
  };
}

export function resolveRef(ref: string | undefined, dataMap: DataMap): unknown {
  if (!ref?.startsWith("$")) return undefined;
  return dataMap[ref.slice(1)] ?? null;
}
