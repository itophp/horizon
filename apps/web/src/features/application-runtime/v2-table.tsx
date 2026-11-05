import { useTablePreferences } from "./table-preferences";
import { useMemo } from "react";
import { Link, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { DataTable, type AdminColumn } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import {
  useRuntime,
  type RecordData,
  LocalBoundary,
  RegisteredDisplay,
} from "./v2-registry";
import { ActionButtons, routeTo } from "./action";
import type { RuntimeAction } from "@/api/generated/runtime";
import type { Scope, Json } from "./expression";
export function RuntimeTable({
  rows,
  total,
  change,
  onDialog,
  onSelectRecord,
}: {
  onSelectRecord?: (id: Json) => void;
  rows: RecordData[];
  total: number;
  change: (key: string, value: string) => void;
  onDialog: (a: RuntimeAction, s: Scope) => Promise<void>;
}) {
  const ctx = useRuntime();
  const navigate = useNavigate();
  const { t } = useTranslation("runtime");
  const { hidden, setHidden, widths, setWidths } = useTablePreferences();
  const table = ctx.page.table;
  const provider = ctx.application.providers.find(
    (p) => p.id === ctx.page.provider,
  );
  if (!provider) throw new Error("schema_invalid");
  const query = ctx.scope.query as Record<string, Json>;
  const number = Number(query.page ?? 1);
  const size = Number(query.pageSize ?? 20);
  const columns = useMemo<AdminColumn<{ id: number; data: RecordData }>[]>(
    () => [
      ...(table.selectable
        ? [
            {
              id: "selection",
              header: t("selectAll"),
              cell: (row: { id: number; data: RecordData }) => (
                <input
                  type="checkbox"
                  aria-label={t("selectRow", {
                    id: row.data[provider.keyField],
                  })}
                  checked={ctx.selection.includes(row.data[provider.keyField])}
                  onChange={(e) =>
                    ctx.setSelection(
                      e.target.checked
                        ? [...ctx.selection, row.data[provider.keyField]]
                        : ctx.selection.filter(
                            (id) => id !== row.data[provider.keyField],
                          ),
                    )
                  }
                />
              ),
            },
          ]
        : []),
      ...table.columns
        .filter((f) => !f.hidden && !hidden.includes(f.name))
        .map((f, index, visible) => ({
          id: f.name,
          className: f.fixed ? "sticky z-10 bg-background" : undefined,
          style: {
            width: widths[f.name] || f.width || 160,
            minWidth: widths[f.name] || f.width || 160,
            maxWidth: widths[f.name] || f.width || 160,
            boxSizing: "border-box" as const,
            left: f.fixed
              ? visible
                  .slice(0, index)
                  .filter((column) => column.fixed)
                  .reduce(
                    (offset, column) =>
                      offset + (widths[column.name] || column.width || 160),
                    0,
                  )
              : undefined,
          },
          header: t(f.labelKey),
          cell: (row: { id: number; data: RecordData }) => {
            const value = row.data[f.name];

            const page = ctx.application.pages.find((p) => p.id === f.linkPage);
            return (
              <LocalBoundary>
                <span
                  style={{
                    display: "inline-block",
                    width: "100%",
                  }}
                >
                  {page ? (
                    <Link
                      to={routeTo(page.route, {
                        id: row.data[provider.keyField],
                      })}
                    >
                      <RegisteredDisplay definition={f} value={value} />
                    </Link>
                  ) : (
                    <RegisteredDisplay definition={f} value={value} />
                  )}
                </span>
              </LocalBoundary>
            );
          },
        })),
      ...(ctx.page.actions.some((a) => a.placement === "row")
        ? [
            {
              id: "actions",
              header: t("actions"),
              cell: (row: { id: number; data: RecordData }) => (
                <ActionButtons
                  placement="row"
                  record={row.data}
                  onDialog={onDialog}
                />
              ),
            },
          ]
        : []),
    ],
    [table, hidden, widths, ctx, provider, t, onDialog],
  );
  return (
    <>
      <div className="standalone-toolbar">
        {table.selectable && (
          <input
            type="checkbox"
            aria-label={t("selectAll")}
            checked={
              rows.length > 0 &&
              rows.every((r) => ctx.selection.includes(r[provider.keyField]))
            }
            onChange={(e) =>
              ctx.setSelection(
                e.target.checked ? rows.map((r) => r[provider.keyField]) : [],
              )
            }
          />
        )}
        {table.filters.map((f) => (
          <label key={f.name}>
            {t(f.labelKey)}
            {f.type === "enum" ? (
              <select
                aria-label={t(f.labelKey)}
                value={String(query[f.name] ?? "")}
                onChange={(e) => change(f.name, e.target.value)}
              >
                <option value="">{t("all")}</option>
                {f.enum.map((v) => (
                  <option key={v} value={v}>
                    {f.valueLabelPrefix ? t(f.valueLabelPrefix + v) : v}
                  </option>
                ))}
              </select>
            ) : (
              <input
                aria-label={t(f.labelKey)}
                value={String(query[f.name] ?? "")}
                onChange={(e) => change(f.name, e.target.value)}
              />
            )}
          </label>
        ))}
        {table.sorts.length > 0 && (
          <label>
            {t("sort")}
            <select
              aria-label={t("sort")}
              value={String(query.sort ?? table.defaultSort)}
              onChange={(e) => change("sort", e.target.value)}
            >
              {table.sorts.map((s) => (
                <option key={s.value} value={s.value}>
                  {t(s.labelKey)}
                </option>
              ))}
            </select>
          </label>
        )}
        <ActionButtons placement="toolbar" onDialog={onDialog} />
        {table.selectable && (
          <ActionButtons placement="selection" onDialog={onDialog} />
        )}
        <details>
          <summary>{t("columns")}</summary>
          {table.columns
            .filter((f) => !f.hidden)
            .map((f) => (
              <label key={f.name}>
                <input
                  type="checkbox"
                  checked={!hidden.includes(f.name)}
                  onChange={(e) =>
                    setHidden((old) =>
                      e.target.checked
                        ? old.filter((n) => n !== f.name)
                        : [...old, f.name],
                    )
                  }
                />
                {t("columnVisibility", { column: t(f.labelKey) })}
                <input
                  type="range"
                  min={80}
                  max={500}
                  aria-label={t("columnWidth", { column: t(f.labelKey) })}
                  value={widths[f.name] || f.width || 160}
                  onChange={(e) =>
                    setWidths((old) => ({
                      ...old,
                      [f.name]: Number(e.target.value),
                    }))
                  }
                />
              </label>
            ))}
        </details>
      </div>
      <DataTable
        onRowClick={
          onSelectRecord || table.rowPage
            ? (row) => {
                if (onSelectRecord) onSelectRecord(row.data[provider.keyField]);
                else {
                  const target = ctx.application.pages.find(
                    (page) => page.id === table.rowPage,
                  );
                  if (target)
                    navigate(
                      routeTo(target.route, {
                        id: row.data[provider.keyField],
                      }),
                    );
                }
              }
            : undefined
        }
        rows={rows.map((data, index) => ({ id: index, data }))}
        columns={columns}
        serverPage
      />
      {provider.pagination === "server" && (
        <div className="editor-footer">
          <Button
            type="button"
            disabled={number <= 1}
            onClick={() => change("page", String(number - 1))}
          >
            {t("previous")}
          </Button>
          <span>{t("total", { count: total })}</span>
          <Button
            type="button"
            disabled={number * size >= total}
            onClick={() => change("page", String(number + 1))}
          >
            {t("next")}
          </Button>
        </div>
      )}
    </>
  );
}
