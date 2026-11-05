import { useState, type ReactNode } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useSession } from "@/features/administration/auth";
import {
  EditorDialog,
  ErrorBanner,
  Loading,
  PageHeading,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/api/client/request";
import { runtimeApplication, runtimeRead } from "@/lib/capabilities";
import { RuntimeList } from "./list";
import { RuntimeForm } from "./form";
import {
  Context,
  RuntimeBoundary,
  pageOf,
  validateApplication,
  Diagnostic,
} from "./registry";
import { listSchema, recordSchema, type RuntimeRecord } from "./records";
import type { RuntimeApplication, RuntimePage } from "@/api/generated/runtime";

export type RuntimeExtension = {
  render?: (page: RuntimePage) => ReactNode;
  actions?: (record: RuntimeRecord, refresh: () => Promise<void>) => ReactNode;
};
export function ApplicationPage({
  extension = {},
  applicationId,
  permission,
}: {
  extension?: RuntimeExtension;
  applicationId: string;
  permission: string;
}) {
  const session = useSession();
  const { i18n } = useTranslation();
  const identity = JSON.stringify([
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.permissions,
    i18n.language,
  ]);
  return (
    <RuntimeBoundary key={identity}>
      <Loader
        identity={identity}
        extension={extension}
        applicationId={applicationId}
        permission={permission}
      />
    </RuntimeBoundary>
  );
}
function Loader({
  identity,
  extension,
  applicationId,
  permission,
}: {
  identity: string;
  extension: RuntimeExtension;
  applicationId: string;
  permission: string;
}) {
  const { t } = useTranslation("articles");
  const session = useSession();
  const query = useQuery({
    queryKey: ["runtime", identity, "application", applicationId],
    queryFn: async ({ signal }) =>
      validateApplication(await runtimeApplication(applicationId, signal)),
    enabled: session.permissions.includes(permission),
    retry: false,
  });
  if (!session.permissions.includes(permission))
    return <div role="alert">{t("denied")}</div>;
  if (query.isPending) return <Loading />;
  if (query.isError && !(query.error instanceof ApiError))
    return <Diagnostic />;
  if (query.isError)
    return (
      <ErrorBanner
        error={query.error}
        onRetry={() => {
          void query.refetch();
        }}
      />
    );
  return <Loaded app={query.data} identity={identity} extension={extension} />;
}
function Loaded({
  app,
  identity,
  extension,
}: {
  app: RuntimeApplication;
  identity: string;
  extension: RuntimeExtension;
}) {
  const { id } = useParams();
  const { i18n } = useTranslation();
  const recordId = id ? Number(id) : undefined;
  if (id && (!Number.isSafeInteger(recordId) || Number(recordId) <= 0))
    throw new Error("runtime_context_invalid");
  const page =
    app.pages.find((p) => p.type === "custom") ??
    pageOf(app, recordId ? "detail" : "list");
  if (page.type === "custom" && !extension.render)
    throw new Error("component_not_registered");
  return (
    <Context.Provider
      value={{
        applicationId: app.id,
        pageId: page.id,
        entityRef: page.entityRef,
        recordId,
        metadataVersion: app.metadataVersion,
        locale: i18n.language,
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        selection: [],
      }}
    >
      {extension.render ? (
        extension.render(page)
      ) : (
        <Content
          key={id ?? "list"}
          app={app}
          page={page}
          identity={identity}
          recordId={recordId}
          extension={extension}
        />
      )}
    </Context.Provider>
  );
}
function Content({
  app,
  page,
  identity,
  recordId,
  extension,
}: {
  app: RuntimeApplication;
  page: RuntimePage;
  identity: string;
  recordId?: number;
  extension: RuntimeExtension;
}) {
  const { t } = useTranslation("articles");
  const client = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const formPage = pageOf(app, "form");
  const pageNumber = Math.max(1, Number(params.get("page")) || 1);
  const status = params.get("status") ?? "";
  const keyword = params.get("q") ?? "";
  const sort = params.get("sort") ?? "id:desc";
  const prefix = ["runtime", identity, app.metadataVersion];
  const input = recordId
    ? { id: recordId }
    : {
        page: pageNumber,
        pageSize: 20,
        keyword,
        sort,
        ...(status ? { status } : {}),
      };
  const query = useQuery({
    queryKey: [...prefix, page.id, input],
    queryFn: async ({ signal }) => {
      const raw = await runtimeRead(page.readCapability, input, signal);
      return recordId ? recordSchema.parse(raw) : listSchema.parse(raw);
    },
    retry: false,
    refetchInterval: 5000,
  });
  const refresh = async () => {
    await client.invalidateQueries({ queryKey: prefix });
  };
  const editRead = useMutation({
    mutationFn: async () => {
      if (!recordId) return;
      return recordSchema.parse(
        await runtimeRead(formPage.readCapability, { id: recordId }),
      );
    },
    onSuccess: () => setEditing(true),
  });
  const data = query.data;
  const record = data && "id" in data ? (data as RuntimeRecord) : undefined;
  const list = data && "items" in data ? listSchema.parse(data) : undefined;
  const change = (key: string, value: string) =>
    setParams((old) => {
      const next = new URLSearchParams(old);
      if (value) next.set(key, value);
      else next.delete(key);
      if (key !== "page") next.delete("page");
      return next;
    });
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading title={t(page.titleKey)} subtitle={t("description")} />
      {(query.isError || editRead.isError) && (
        <ErrorBanner
          error={query.error ?? editRead.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      {recordId ? (
        <>
          <Link to={app.route}>{t("runtime.back")}</Link>
          {record && (
            <section className="admin-panel">
              <dl className="detail-grid">
                {page.fields
                  .filter((f) => !f.hidden)
                  .map((f) => (
                    <div key={f.name}>
                      <dt>{t(f.labelKey)}</dt>
                      <dd>
                        {f.valueLabelPrefix && record[f.name]
                          ? t(`${f.valueLabelPrefix}${record[f.name]}`)
                          : String(record[f.name] ?? "")}
                      </dd>
                    </div>
                  ))}
              </dl>
              {formPage.updateCapability &&
                formPage.editableStates.includes(String(record.status)) && (
                  <Button
                    type="button"
                    disabled={editRead.isPending}
                    onClick={() => editRead.mutate()}
                  >
                    {t("article.edit")}
                  </Button>
                )}
              {extension.actions?.(record, refresh)}
            </section>
          )}
        </>
      ) : (
        <RuntimeList
          app={app}
          page={page}
          list={list}
          keyword={keyword}
          status={status}
          sort={sort}
          pageNumber={pageNumber}
          change={change}
          canCreate={!!formPage.createCapability}
          onCreate={() => setEditing(true)}
        />
      )}
      <EditorDialog
        open={editing}
        onOpenChange={(open) => {
          if (!open && !saving && window.confirm(t("runtime.discard")))
            setEditing(false);
        }}
        title={t(recordId ? "article.edit" : "new")}
        description={t("article.dialogHint")}
        wide
      >
        {editing && (
          <RuntimeForm
            page={formPage}
            onBusyChange={setSaving}
            record={recordId ? editRead.data : undefined}
            onCancel={() => setEditing(false)}
            onSaved={async () => {
              setEditing(false);
              await refresh();
            }}
          />
        )}
      </EditorDialog>
    </>
  );
}
