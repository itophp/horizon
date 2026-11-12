import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { RuntimeRouteParams } from "../route-params";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ErrorBanner, Loading } from "@/features/administration/shared";
import { capabilityDecide } from "@/api/generated/baseline";
import { evaluate, validateInput } from "../expression";
import { ApiError } from "@/api/client/request";
import { useConfiguration } from "@/features/governance/runtime";
import { useSession } from "@/features/administration/auth";
import type { RuntimeInvocationReference } from "@/api/generated/models";
import { runtimeExecute } from "@/lib/capabilities";
import {
  ComponentRegistry,
  PresentationPermissions,
} from "./component-registry";
import {
  collectDataSpecs,
  useProtocolData,
  type DataOccurrence,
  type ProtocolScope,
} from "./data-resolver";
import {
  BindingIndex,
  ProtocolRenderer,
  type ProtocolEventHandler,
} from "./layout-renderer";
import { createAdminRegistry } from "./admin-components";
import { coerceProviderInput } from "./data-resolver";
import { ProtocolErrorDialog } from "./error-dialog";
import type { ApplicationV3, EventBinding, PageV3 } from "./types";
import "./protocol.css";

function useDebouncedValue<T>(value: T, delay: number): T {
  const committed = useRef(value);
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    if (Object.is(value, committed.current)) return;
    const timer = setTimeout(() => {
      committed.current = value;
      setDebounced(value);
    }, delay);
    return () => clearTimeout(timer);
  }, [delay, value]);
  return debounced;
}

function queryScope(searchParams: URLSearchParams): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  searchParams.forEach((value, key) => {
    result[key] = value;
  });
  return result;
}

function routeWithParameters(
  route: string,
  parameters: Record<string, unknown>,
): string {
  return route.replace(/:([a-zA-Z][a-zA-Z0-9]*)/g, (_match, key: string) => {
    const value = parameters[key];
    if (typeof value !== "string" && typeof value !== "number")
      throw new Error("runtime_context_invalid");
    return encodeURIComponent(String(value));
  });
}

function payloadObject(payload: unknown): Record<string, unknown> {
  return payload && typeof payload === "object" && !Array.isArray(payload)
    ? (payload as Record<string, unknown>)
    : {};
}

function invokeTarget(target: string): { name: string; version: string } {
  const position = target.lastIndexOf("@");
  return position === -1
    ? { name: target, version: "1.0.0" }
    : {
        name: target.slice(0, position),
        version: target.slice(position + 1),
      };
}

export function ProtocolPage({
  application,
  page,
  registry,
  allowed,
  readOnly = false,
  invocation,
}: {
  application: ApplicationV3;
  page: PageV3;
  registry?: ComponentRegistry;
  allowed?: boolean;
  readOnly?: boolean;
  invocation?: RuntimeInvocationReference;
}) {
  const { t, i18n } = useTranslation("runtime");
  const session = useSession();
  const timeZone = useConfiguration().value(
    "display.timezone",
    "Asia/Shanghai",
  );
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const routeParams = useParams();
  const previewRouteParams = useContext(RuntimeRouteParams);
  const [searchParams] = useSearchParams();
  const [localData, setLocalData] = useState<Record<string, unknown>>({});
  const [eventError, setEventError] = useState<unknown>(null);
  const [failedEvent, setFailedEvent] = useState<{
    events: EventBinding[];
    event: string;
    payload: unknown;
  } | null>(null);
  const inFlightActions = useRef(new Set<string>());
  const [correlationId] = useState(() => crypto.randomUUID());
  const defaultRegistry = useMemo(() => createAdminRegistry(), []);
  const activeRegistry = registry ?? defaultRegistry;

  const schemaValid =
    application.schemaVersion === "3.0" &&
    application.runtimeCompatibility === "3.0" &&
    page.schemaVersion === "3.0" &&
    page.root.type === "layout" &&
    Boolean(page.root.layout);
  const providers = useMemo(
    () =>
      new Map(
        application.providers
          .filter(
            (provider) =>
              !provider.permission ||
              provider.permission === "authenticated" ||
              session.permissions.includes(provider.permission),
          )
          .map((provider) => [provider.id, provider]),
      ),
    [application.providers, session.permissions],
  );
  const occurrences = useMemo<DataOccurrence[]>(
    () => (schemaValid ? collectDataSpecs(page.root) : []),
    [page.root, schemaValid],
  );
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId ?? ""}:${session.authorizationVersion ?? ""}`;
  const scope = useMemo<ProtocolScope>(
    () => ({
      route: { ...previewRouteParams, ...routeParams },
      query: queryScope(searchParams),
      context: {
        applicationId: application.id,
        applicationVersion: application.version,
        pageId: page.id,
        locale: i18n.language,
        timeZone,
        accountId: session.account.id,
        permissions: session.permissions,
      },
    }),
    [
      application.id,
      application.version,
      i18n.language,
      timeZone,
      page.id,
      previewRouteParams,
      routeParams,
      searchParams,
      session.account.id,
      session.permissions,
    ],
  );
  const localVariables = useMemo(() => {
    const values: Record<string, unknown> = {};
    for (const occurrence of occurrences) {
      if (!occurrence.spec.provider)
        values[occurrence.spec.id] =
          localData[occurrence.key] ?? occurrence.spec.initial ?? null;
    }
    return values;
  }, [localData, occurrences]);
  const debouncedLocalVariables = useDebouncedValue(localVariables, 250);
  const dataScope = useMemo<ProtocolScope>(
    () => ({ ...scope, data: debouncedLocalVariables }),
    [debouncedLocalVariables, scope],
  );
  const data = useProtocolData({
    application,
    page,
    occurrences,
    providers,
    scope: dataScope,
    identity,
  });

  const permissionAllowed =
    allowed ??
    (page.permission === "" ||
      page.permission === "authenticated" ||
      session.permissions.includes(page.permission));
  const dataIndex = useMemo(() => {
    const result: Record<string, unknown> = {};
    for (const occurrence of occurrences) {
      result[occurrence.key] =
        occurrence.spec.provider && localData[occurrence.key] !== undefined
          ? localData[occurrence.key]
          : occurrence.spec.provider
            ? data.values[occurrence.key]
            : (localData[occurrence.key] ?? occurrence.spec.initial ?? null);
    }
    return result;
  }, [data.values, localData, occurrences]);

  const handleEvent = useCallback<ProtocolEventHandler>(
    async (events: EventBinding[], event: string, payload?: unknown) => {
      const bindings = events.filter((binding) => binding.event === event);
      if (bindings.length === 0 || readOnly) return;
      setEventError(null);
      setFailedEvent(null);
      try {
        for (const binding of bindings) {
          const eventScope = {
            ...dataScope,
            data: {
              ...(dataScope.data as Record<string, unknown>),
              ...Object.fromEntries(
                occurrences.map(({ spec, key }) => [spec.id, dataIndex[key]]),
              ),
            },
            event: payload,
          };
          if (binding.when?.kind && !evaluate(binding.when, eventScope))
            continue;
          const eventPayload =
            binding.payload?.kind && binding.payload.kind !== ""
              ? binding.payload
              : undefined;
          const evaluated = eventPayload
            ? evaluate(eventPayload, eventScope)
            : payload;
          if (binding.action === "navigate") {
            const target = application.pages.find(
              (item) => item.id === binding.target,
            );
            if (binding.target.startsWith("/")) navigate(binding.target);
            else if (target)
              navigate(
                routeWithParameters(target.route, payloadObject(evaluated)),
              );
          } else if (binding.action === "setData") {
            const target =
              occurrences.find(
                (occurrence) => occurrence.spec.id === binding.target,
              )?.key ?? binding.target;
            setLocalData((current) => ({
              ...current,
              [target]: evaluated,
            }));
          } else if (binding.action === "refresh") {
            await data.refetch();
          } else if (
            binding.action === "invoke" ||
            binding.action === "system"
          ) {
            const target = invokeTarget(binding.target);
            const input = coerceProviderInput(
              payloadObject(evaluated),
              binding.inputSchema ?? "",
            );
            const key = binding.idempotencyKey?.kind
              ? evaluate(binding.idempotencyKey, eventScope)
              : undefined;
            if (inFlightActions.current.has(binding.target)) return false;
            inFlightActions.current.add(binding.target);
            const execute = async () => {
              try {
                return binding.action === "system"
                  ? await executeSystemAction(binding, input)
                  : await runtimeExecute(
                      target.name,
                      target.version,
                      input,
                      typeof key === "string" && key
                        ? key
                        : crypto.randomUUID(),
                      binding.resultTarget
                        ? typeof key === "string" && key
                          ? key
                          : crypto.randomUUID()
                        : correlationId,
                      invocation,
                    );
              } finally {
                inFlightActions.current.delete(binding.target);
              }
            };
            const result = await execute();
            if (
              result.status !== "succeeded" &&
              result.status !== "awaiting_approval" &&
              !(binding.action === "system" && result.status === "cancelled")
            )
              throw new ApiError(result.errorCode || "internal", 400);
            if (binding.resultTarget && binding.result?.kind) {
              const occurrence = occurrences.find(
                ({ spec }) =>
                  !spec.provider && spec.id === binding.resultTarget,
              );
              if (!occurrence) throw new Error("field_not_allowed");
              const value = evaluate(binding.result, {
                ...eventScope,
                event: result,
              });
              setLocalData((current) => ({
                ...current,
                [occurrence.key]: value,
              }));
            }
            await queryClient.invalidateQueries({
              predicate: ({ queryKey, meta }) =>
                (queryKey[0] === "protocol-v3" && queryKey[1] === identity) ||
                (result.status === "succeeded" &&
                  Array.isArray(meta?.refreshAfter) &&
                  meta.refreshAfter.includes(target.name)),
            });
          }
        }
        return true;
      } catch (cause) {
        setEventError(cause);
        if (
          bindings.length === 1 &&
          bindings[0].action === "invoke" &&
          bindings[0].idempotencyKey?.kind
        )
          setFailedEvent({ events, event, payload });
        return false;
      }
    },
    [
      application.pages,
      correlationId,
      data,
      dataScope,
      dataIndex,
      identity,
      invocation,
      navigate,
      occurrences,
      queryClient,
      readOnly,
    ],
  );

  if (!schemaValid) return <p role="alert">{t("schemaError")}</p>;
  if (!permissionAllowed) return <p role="alert">{t("denied")}</p>;
  if (data.pending) return <Loading />;
  if (data.error)
    return <ErrorBanner error={data.error} onRetry={data.refetch} />;

  return (
    <div className="protocol-page">
      {eventError !== null && (
        <ProtocolErrorDialog
          error={eventError}
          onClose={() => setEventError(null)}
          onRetry={
            failedEvent
              ? () => {
                  void handleEvent(
                    failedEvent.events,
                    failedEvent.event,
                    failedEvent.payload,
                  );
                }
              : undefined
          }
        />
      )}
      <ProtocolRuntimeContext.Provider
        value={{
          application,
          pageId: page.id,
          identity,
          correlationId,
          readOnly,
          invocation,
        }}
      >
        <PresentationPermissions.Provider value={session.permissions}>
          <BindingIndex.Provider value={data.bindings}>
            <ProtocolRenderer
              node={page.root}
              dataMap={{}}
              dataIndex={dataIndex}
              registry={activeRegistry}
              onEvent={handleEvent}
              readOnly={readOnly}
            />
          </BindingIndex.Provider>
        </PresentationPermissions.Provider>
      </ProtocolRuntimeContext.Provider>
    </div>
  );
}

async function executeSystemAction(
  binding: EventBinding,
  input: Record<string, unknown>,
) {
  if (binding.target !== "capability.approval.decide" || !binding.inputSchema)
    throw new ApiError("invalid", 400);
  validateInput(input, binding.inputSchema);
  if (typeof input.id !== "number" || typeof input.approve !== "boolean")
    throw new ApiError("invalid", 400);
  const result = await capabilityDecide(input.id, { approve: input.approve });
  if (result.status !== 200) throw new ApiError("unavailable", result.status);
  return result.data;
}

export type ProtocolRuntimeContextValue = {
  application: ApplicationV3;
  pageId: string;
  identity: string;
  correlationId: string;
  readOnly: boolean;
  invocation?: RuntimeInvocationReference;
};

export const ProtocolRuntimeContext =
  createContext<ProtocolRuntimeContextValue | null>(null);

export function useProtocolRuntime() {
  const value = useContext(ProtocolRuntimeContext);
  if (!value) throw new Error("runtime_context_invalid");
  return value;
}
