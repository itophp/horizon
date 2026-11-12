import { useState, type MouseEvent, type ReactNode } from "react";
import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

// Generic tree primitives shared by any feature. They know nothing about
// organizations, departments or any other business object: callers provide
// nodes, rows, columns and localized labels.

export type TreeItem = {
  key: string;
  label: string;
  icon?: ReactNode;
  badge?: string | number;
  children?: TreeItem[];
};

export function TreePanel({
  title,
  titleIcon,
  allLabel,
  allIcon,
  allCount,
  items,
  selectedKey,
  onSelect,
  footnote,
  footnoteIcon,
}: {
  title: ReactNode;
  titleIcon?: ReactNode;
  allLabel?: ReactNode;
  allIcon?: ReactNode;
  allCount?: string | number;
  items: TreeItem[];
  selectedKey: string;
  onSelect: (key: string) => void;
  footnote?: ReactNode;
  footnoteIcon?: ReactNode;
}) {
  const allSelected = selectedKey === "";
  const node = (item: TreeItem, child = false) => (
    <button
      type="button"
      key={item.key}
      className={`tree-node ${child ? "tree-child " : ""}${
        selectedKey === item.key ? "selected" : ""
      }`}
      aria-pressed={selectedKey === item.key}
      onClick={() => onSelect(item.key)}
    >
      {item.icon}
      <span>{item.label}</span>
      {item.badge !== undefined && <small>{item.badge}</small>}
    </button>
  );
  return (
    <aside className="admin-panel tree-panel">
      <div className="tree-title">
        {titleIcon}
        {title}
      </div>
      {allLabel !== undefined && (
        <button
          type="button"
          className={`tree-node ${allSelected ? "selected" : ""}`}
          aria-pressed={allSelected}
          onClick={(event: MouseEvent) => {
            event.preventDefault();
            onSelect("");
          }}
        >
          {allIcon}
          <span>{allLabel}</span>
          {allCount !== undefined && <small>{allCount}</small>}
        </button>
      )}
      <div>
        {items.map((item) => (
          <div key={item.key}>
            {node(item)}
            {item.children?.map((child) => node(child, true))}
          </div>
        ))}
      </div>
      {footnote !== undefined && (
        <div className="tree-footnote">
          {footnoteIcon}
          {footnote}
        </div>
      )}
    </aside>
  );
}

export type TreeTableRow<T> = {
  key: string;
  record: T;
  depth: number;
  hasChildren: boolean;
};

export type TreeTableColumn<T> = {
  id: string;
  header: ReactNode;
  className?: string;
  cell: (record: T) => ReactNode;
};

export function TreeTable<T>({
  rows,
  columns,
  collapsedKeys,
  onToggle,
  labels,
  pageSize = 8,
  empty,
}: {
  rows: TreeTableRow<T>[];
  columns: TreeTableColumn<T>[];
  collapsedKeys: ReadonlySet<string>;
  onToggle: (key: string) => void;
  labels: {
    expand: string;
    collapse: string;
    range: (from: number, to: number, total: number) => string;
    page: (page: number, total: number) => string;
    previous: string;
    next: string;
  };
  pageSize?: number;
  empty?: ReactNode;
}) {
  const [page, setPage] = useState(1);
  const pages = Math.max(1, Math.ceil(rows.length / pageSize));
  const current = Math.min(page, pages);
  const visible = rows.slice((current - 1) * pageSize, current * pageSize);
  return (
    <>
      <div className="admin-table-scroll">
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map((column) => (
                <TableHead key={column.id} className={column.className}>
                  {column.header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {visible.map((row) => {
              const collapsed = collapsedKeys.has(row.key);
              return (
                <TableRow key={row.key}>
                  {columns.map((column, index) => (
                    <TableCell key={column.id} className={column.className}>
                      {index === 0 ? (
                        <span
                          className="tree-cell"
                          style={{ paddingLeft: row.depth * 19 }}
                        >
                          {row.hasChildren ? (
                            <button
                              type="button"
                              aria-label={
                                collapsed ? labels.expand : labels.collapse
                              }
                              onClick={() => onToggle(row.key)}
                            >
                              {collapsed ? (
                                <ChevronRight size={13} />
                              ) : (
                                <ChevronDown size={13} />
                              )}
                            </button>
                          ) : (
                            <span className="tree-spacer" />
                          )}
                          {column.cell(row.record)}
                        </span>
                      ) : (
                        column.cell(row.record)
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        {rows.length === 0 && (empty ?? null)}
      </div>
      <div className="table-pagination">
        <span>
          {labels.range(
            rows.length ? (current - 1) * pageSize + 1 : 0,
            Math.min(current * pageSize, rows.length),
            rows.length,
          )}
        </span>
        <div>
          <span>{labels.page(current, pages)}</span>
          <Button
            variant="outline"
            size="icon-sm"
            disabled={current === 1}
            aria-label={labels.previous}
            onClick={() => setPage(current - 1)}
          >
            <ChevronLeft size={14} />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            disabled={current === pages}
            aria-label={labels.next}
            onClick={() => setPage(current + 1)}
          >
            <ChevronRight size={14} />
          </Button>
        </div>
      </div>
    </>
  );
}
