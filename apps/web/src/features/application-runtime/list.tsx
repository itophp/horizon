import { useMemo } from "react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import type { RuntimeApplication, RuntimePage } from "@/api/generated/runtime";
import {
  DataTable,
  ListSearch,
  type AdminColumn,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import type { RuntimeRecord } from "./records";
export function RuntimeList({
  app,
  page,
  list,
  keyword,
  status,
  sort,
  pageNumber,
  change,
  canCreate,
  onCreate,
}: {
  app: RuntimeApplication;
  page: RuntimePage;
  list?: { items: RuntimeRecord[]; total: number; pageSize: number };
  keyword: string;
  status: string;
  sort: string;
  pageNumber: number;
  change: (key: string, value: string) => void;
  canCreate: boolean;
  onCreate: () => void;
}) {
  const { t } = useTranslation("articles");
  const columns = useMemo<AdminColumn<RuntimeRecord>[]>(
    () =>
      page.fields
        .filter((f) => !f.hidden)
        .map((f) => ({
          id: f.name,
          header: t(f.labelKey),
          cell: (row) =>
            f.link ? (
              <Link to={`${app.route}/${row.id}`}>
                {String(row[f.name] ?? "")}
              </Link>
            ) : f.valueLabelPrefix && row[f.name] ? (
              t(`${f.valueLabelPrefix}${row[f.name]}`)
            ) : (
              String(row[f.name] ?? "")
            ),
        })),
    [page, app.route, t],
  );
  return (
    <>
      <div className="standalone-toolbar">
        <ListSearch value={keyword} onChange={(v) => change("q", v)} />
        <select
          aria-label={t("status")}
          value={status}
          onChange={(e) => change("status", e.target.value)}
        >
          <option value="">{t("runtime.all")}</option>
          {page.fields
            .find((f) => f.name === "status")
            ?.enum.map((s) => (
              <option key={s} value={s}>
                {t(`states.${s}`)}
              </option>
            ))}
        </select>
        <select
          aria-label={t("runtime.sort")}
          value={sort}
          onChange={(e) => change("sort", e.target.value)}
        >
          {[
            "id:desc",
            "id:asc",
            "title:asc",
            "title:desc",
            "status:asc",
            "status:desc",
            "revision:asc",
            "revision:desc",
          ].map((s) => (
            <option key={s} value={s}>
              {t(`runtime.sorts.${s.replace(":", "_")}`)}
            </option>
          ))}
        </select>
        {canCreate && (
          <Button type="button" onClick={onCreate}>
            {t("new")}
          </Button>
        )}
      </div>
      <section className="admin-panel">
        <DataTable rows={list?.items ?? []} columns={columns} serverPage />
      </section>
      <div className="editor-footer">
        <Button
          type="button"
          disabled={pageNumber <= 1}
          onClick={() => change("page", String(pageNumber - 1))}
        >
          {t("runtime.previous")}
        </Button>
        <span>{t("article.total", { count: list?.total ?? 0 })}</span>
        <Button
          type="button"
          disabled={!list || pageNumber * list.pageSize >= list.total}
          onClick={() => change("page", String(pageNumber + 1))}
        >
          {t("runtime.next")}
        </Button>
      </div>
    </>
  );
}
