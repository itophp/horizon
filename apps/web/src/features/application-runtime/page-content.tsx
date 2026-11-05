import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import type { RuntimeAction } from "@/api/generated/runtime";
import { ErrorBanner } from "@/features/administration/shared";
import { useRuntime } from "./v2-registry";
import { type readProvider } from "./data";
import { type Scope } from "./expression";
import { PageView } from "./page-view";
import { CommandForm } from "./v2-form";
import { RuntimeTable } from "./v2-table";
import { PageLayout } from "./v2-layout";
import { ActionButtons } from "./action";
export function PageContent({
  data,
  embedded,
  change,
  onDialog,
  onGuard,
  onSaved,
}: {
  data: Awaited<ReturnType<typeof readProvider>>;
  embedded: boolean;
  change: (key: string, value: string) => void;
  onDialog: (action: RuntimeAction, scope: Scope) => Promise<void>;
  onGuard: (guard: () => boolean) => void;
  onSaved: (reference: string) => void;
}) {
  const { t } = useTranslation("runtime");
  const ctx = useRuntime();
  const { page, application } = ctx;
  if (page.type === "form") {
    const action = page.actions.find(
      (action) =>
        action.type === "dialog" &&
        (action.mode === "create" || action.mode === "edit"),
    );
    const form = page.forms.find((form) => form.id === action?.target);
    return form && action ? (
      <CommandForm
        definition={form}
        mode={action.mode}
        scope={ctx.scope}
        record={data.record}
        onGuard={onGuard}
        onSaved={onSaved}
      />
    ) : (
      <ErrorBanner error={new Error("action_not_found")} />
    );
  }
  if (page.type === "list") {
    const provider = application.providers.find(
      (provider) => provider.id === page.provider,
    )!;
    const query = ctx.scope.query;
    const selectedId =
      query && typeof query === "object" && !Array.isArray(query)
        ? query.selectedId
        : undefined;
    const selected = data.items.find(
      (row) => String(row[provider.keyField]) === String(selectedId),
    );
    const detail = application.pages.find(
      (target) => target.id === page.layout.sections[0]?.pageId,
    );
    return (
      <div
        className={
          page.layout.type === "master-detail"
            ? "runtime-master-detail"
            : undefined
        }
      >
        <RuntimeTable
          rows={data.items}
          total={data.total}
          change={change}
          onDialog={onDialog}
          onSelectRecord={
            page.layout.type === "master-detail"
              ? (id) => change("selectedId", String(id))
              : undefined
          }
        />
        {page.layout.type === "master-detail" && selected && detail && (
          <section>
            <button type="button" onClick={() => change("selectedId", "")}>
              {t("backToList")}
            </button>
            <PageView
              key={String(selectedId)}
              application={application}
              page={detail}
              identity={ctx.identity}
              embedded
              routeOverride={{ id: selected[provider.keyField] }}
              readOnly={ctx.readOnly}
            />
          </section>
        )}
      </div>
    );
  }
  return (
    <>
      {!embedded && <Link to={application.route}>{t("back")}</Link>}
      <PageLayout record={data.record ?? {}}>
        <ActionButtons placement="detail" onDialog={onDialog} />
      </PageLayout>
    </>
  );
}
