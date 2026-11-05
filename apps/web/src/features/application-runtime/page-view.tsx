import { RuntimeRouteParams } from "./route-params";
import { RuntimeDialogHost } from "./dialog-host";
import {
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type MutableRefObject,
} from "react";
import {
  useParams,
  useSearchParams,
  useBlocker,
  UNSAFE_DataRouterContext,
} from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useSession } from "@/features/administration/auth";
import {
  ErrorBanner,
  Loading,
  PageHeading,
} from "@/features/administration/shared";
import type {
  RuntimeAction,
  RuntimeApplicationV2,
  RuntimePageV2,
  RuntimeFormV2,
} from "@/api/generated/runtime";
import { providerOf, readProvider } from "./data";
import {
  RuntimeContext,
  useRuntime,
  LocalBoundary,
  type RecordData,
} from "./v2-registry";
import { CommandForm } from "./v2-form";
import { useWorkspaceInstance } from "./workspace";
import { RuntimeTools } from "./runtime-tools";
import { PageContent } from "./page-content";
import type { Json, Scope } from "./expression";
export function EmbeddedPage({
  pageId,
  readOnly = false,
}: {
  pageId: string;
  readOnly?: boolean;
}) {
  const parent = useRuntime();
  const page = parent.application.pages.find((p) => p.id === pageId);
  if (!page) throw new Error("page_not_found");
  return (
    <PageView
      application={parent.application}
      page={page}
      identity={parent.identity}
      embedded
      readOnly={readOnly}
    />
  );
}
function RouteGuard({ guard }: { guard: MutableRefObject<() => boolean> }) {
  const blocker = useBlocker(() => !guard.current());
  useEffect(() => {
    if (blocker.state === "blocked") blocker.reset();
  }, [blocker]);
  return null;
}
export function PageView({
  application,
  page,
  identity,
  embedded = false,
  readOnly = false,
  recordSeed,
  routeOverride,
}: {
  application: RuntimeApplicationV2;
  page: RuntimePageV2;
  identity: string;
  embedded?: boolean;
  readOnly?: boolean;
  recordSeed?: RecordData;
  routeOverride?: Record<string, Json>;
}) {
  const { t, i18n } = useTranslation("runtime");
  const session = useSession();
  const workspace = useWorkspaceInstance();
  const registeredParams = useContext(RuntimeRouteParams);
  const params = { ...useParams(), ...registeredParams };
  const [search, setSearch] = useSearchParams();
  const [selection, setSelection] = useState<Json[]>([]);
  const definitionKey = `${application.metadataVersion}:${application.installationRevision ?? 0}:${application.applicationRevision ?? 0}`;
  const [selectionVersion, setSelectionVersion] = useState(definitionKey);
  if (selectionVersion !== definitionKey) {
    setSelectionVersion(definitionKey);
    setSelection([]);
  }
  const [correlationId] = useState(() => crypto.randomUUID());
  const provider = providerOf(application, page.provider);
  const prefix = embedded ? `${page.id}.` : "";
  const get = (key: string) => search.get(prefix + key) ?? undefined;
  const route: Record<string, Json> = {};
  const inputShape = JSON.parse(provider.inputSchema) as {
    properties?: Record<string, { type?: string }>;
  };
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined) continue;
    const binding = provider.input.find(
      (m) => m.value.kind === "path" && m.value.path === `route.${key}`,
    );
    const type = binding
      ? inputShape.properties?.[binding.target]?.type
      : undefined;
    route[key] =
      type === "integer" || type === "number" ? Number(value) : value;
  }
  Object.assign(route, routeOverride ?? {});
  const queryState: Record<string, Json> = {
    page: Math.max(1, Number(get("page")) || 1),
    pageSize: provider.pageSize || 20,
  };
  for (const f of page.table.filters) {
    const value = get(f.name);
    if (value)
      queryState[f.name] =
        f.type === "integer" || f.type === "number"
          ? Number(value)
          : f.type === "boolean"
            ? value === "true"
            : value;
    else if (f.type !== "enum") queryState[f.name] = "";
  }
  if (page.table.defaultSort)
    queryState.sort = get("sort") ?? page.table.defaultSort;
  if (get("selectedId")) queryState.selectedId = get("selectedId")!;
  const baseScope: Scope = {
    record: recordSeed,
    route,
    query: queryState,
    selection: { ids: selection },
    context: {
      applicationId: application.id,
      pageId: page.id,
      locale: i18n.language,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      accountId: session.account.id,
      canUseAssistant: session.permissions.includes("assistant:use"),
    },
  };
  const data = useQuery({
    queryKey: [
      "runtime-v2",
      identity,
      application.id,
      `${application.metadataVersion}:${application.installationRevision ?? 0}:${application.applicationRevision ?? 0}`,
      provider.id,
      route,
      queryState,
      recordSeed?.[provider.keyField],
    ],
    queryFn: ({ signal }) =>
      readProvider(provider, baseScope, signal, application, page.id),
    retry: false,
    placeholderData: (previous, query) =>
      query?.queryKey[4] === provider.id &&
      JSON.stringify(query.queryKey[5]) === JSON.stringify(route) &&
      JSON.stringify(query.queryKey[6]) === JSON.stringify(queryState)
        ? previous
        : undefined,
    enabled: !readOnly,
    staleTime: provider.staleTimeMs ?? 0,
    refetchInterval: provider.pollInterval || false,
  });
  const scope: Scope = { ...baseScope, record: data.data?.record };
  const [dialog, setDialog] = useState<{
    definition?: RuntimeFormV2;
    pageDefinition?: RuntimePageV2;
    presentation?: string;
    mode: string;
    scope: Scope;
    record?: RecordData;
  } | null>(null);
  const [dialogError, setDialogError] = useState<unknown>();
  const [notice, setNotice] = useState("");
  const guard = useRef<() => boolean>(() => true);
  const onGuard = useCallback((fn: () => boolean) => {
    guard.current = fn;
  }, []);
  const dataRouter = useContext(UNSAFE_DataRouterContext);
  const dialogAbort = useRef<AbortController | null>(null);
  useEffect(() => () => dialogAbort.current?.abort(), []);
  const openDialog = async (action: RuntimeAction, actionScope: Scope) => {
    if (action.mode === "detail") {
      const target = application.pages.find((p) => p.id === action.target);
      if (!target) throw new Error("page_not_found");
      setDialog({
        presentation: action.presentation,
        pageDefinition: target,
        mode: action.mode,
        scope: actionScope,
        record: actionScope.record as RecordData,
      });
      return;
    }
    const form = page.forms.find((f) => f.id === action.target);
    if (!form) throw new Error("action_not_found");
    setDialogError(undefined);
    try {
      let record = actionScope.record as RecordData | undefined;
      if (action.mode === "edit" && form.provider) {
        dialogAbort.current?.abort();
        const controller = new AbortController();
        dialogAbort.current = controller;
        record = (
          await readProvider(
            providerOf(application, form.provider),
            actionScope,
            controller.signal,
            application,
            page.id,
          )
        ).record;
        if (controller.signal.aborted) return;
      }
      setDialog({
        presentation: action.presentation,
        definition: form,
        mode: action.mode,
        scope: actionScope,
        record,
      });
    } catch (e) {
      setDialogError(e);
      throw e;
    }
  };
  const change = (key: string, value: string) => {
    setSelection([]);
    setSearch((old) => {
      const next = new URLSearchParams(old);
      if (value) next.set(prefix + key, value);
      else next.delete(prefix + key);
      if (key !== "page") next.delete(prefix + "page");
      return next;
    });
  };
  const ctx = {
    application,
    page,
    scope,
    identity,
    selection,
    setSelection,
    correlationId,
    readOnly,
  };
  return (
    <RuntimeContext.Provider value={ctx}>
      {(!workspace || workspace.active) && <RuntimeTools />}
      <LocalBoundary key={page.id}>
        {dataRouter && !workspace && <RouteGuard guard={guard} />}
        {!embedded && <PageHeading title={t(page.titleKey)} subtitle="" />}
        {data.isPending ? (
          <Loading />
        ) : data.isError ? (
          <ErrorBanner
            error={data.error}
            onRetry={() => {
              void data.refetch();
            }}
          />
        ) : (
          <PageContent
            data={data.data}
            embedded={embedded}
            change={change}
            onDialog={openDialog}
            onGuard={onGuard}
            onSaved={(reference) => {
              setNotice(reference);
              guard.current = () => true;
            }}
          />
        )}
        {dialogError !== undefined && <ErrorBanner error={dialogError} />}
        {notice && (
          <div role="status">
            {t("saved")} <small>{t("reference", { id: notice })}</small>
          </div>
        )}
        <RuntimeDialogHost
          presentation={dialog?.presentation}
          open={!!dialog}
          onOpenChange={(open) => {
            if (!open && guard.current()) setDialog(null);
          }}
          title={
            dialog
              ? t(
                  dialog.definition?.labelKey ??
                    dialog.pageDefinition?.titleKey ??
                    "runtime:edit",
                )
              : t("edit")
          }
          description={t("formHint")}
        >
          {dialog?.pageDefinition && (
            <PageView
              application={application}
              page={dialog.pageDefinition}
              identity={identity}
              recordSeed={dialog.record}
              embedded
              readOnly={readOnly}
            />
          )}
          {dialog?.definition && (
            <CommandForm
              definition={dialog.definition}
              mode={dialog.mode}
              scope={dialog.scope}
              record={dialog.record}
              onGuard={onGuard}
              onSaved={(reference) => {
                setNotice(reference);
                guard.current = () => true;
                setDialog(null);
                setSelection([]);
              }}
            />
          )}
        </RuntimeDialogHost>
      </LocalBoundary>
    </RuntimeContext.Provider>
  );
}
