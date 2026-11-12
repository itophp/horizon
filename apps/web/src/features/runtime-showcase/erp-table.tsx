import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { tableFeatures, useTable } from "@tanstack/react-table";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
export type ErpColumn<T> = {
  id: string;
  label: string;
  cell: (row: T) => ReactNode;
};
const features = tableFeatures({});
export function ErpTable<T extends { id: string }>({
  rows,
  columns,
}: {
  rows: T[];
  columns: { id: string; label: string; cell: (row: T) => ReactNode }[];
}) {
  const { t } = useTranslation("showcase");
  const defs = useMemo(
    () => columns.map((c) => ({ id: c.id, header: t(c.label) })),
    [columns, t],
  );
  const table = useTable({
    features,
    data: rows,
    columns: defs,
    getRowId: (r) => r.id,
  });
  return (
    <div className="erp-table">
      <Table>
        <TableHeader>
          <TableRow>
            {columns.map((c) => (
              <TableHead key={c.id}>{t(c.label)}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map(({ original: r }) => (
            <TableRow key={r.id}>
              {columns.map((c) => (
                <TableCell key={c.id}>{c.cell(r)}</TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {!rows.length && <p className="sc-empty">{t("empty")}</p>}
    </div>
  );
}
export const money = (value: number, locale: string) =>
  new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "CNY",
    maximumFractionDigits: 2,
  }).format(value);
