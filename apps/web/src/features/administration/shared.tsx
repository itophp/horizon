import { useConfiguration } from "@/features/governance/runtime";
import { useSession } from "./auth";
import { useMemo, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { tableFeatures, useTable, type ColumnDef } from "@tanstack/react-table";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  ChevronLeft,
  ChevronRight,
  Search,
  Inbox,
  LoaderCircle,
  AlertCircle,
  Check,
  X,
  Trash2,
} from "lucide-react";
import {
  useIdentitySnapshot,
  getIdentitySnapshotQueryKey,
  getIdentityMeQueryKey,
  identityDeleteAccount,
  identityDeleteOrganization,
  identityDeleteDepartment,
  identityDeleteRole,
} from "@/api/generated/baseline";
import type { AdminSnapshot } from "@/api/generated/models";
import { ApiError } from "@/api/client/request";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Table,
  TableHeader,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
} from "@/components/ui/table";
export type Resource = "accounts" | "organizations" | "departments" | "roles";
const empty: AdminSnapshot = {
  accounts: [],
  organizations: [],
  departments: [],
  roles: [],
  permissions: [],
  catalog: [],
};
export function useAdminData() {
  const session = useSession();
  const query = useIdentitySnapshot({
    query: {
      retry: false,
      staleTime: 15000,
      queryKey: [
        ...getIdentitySnapshotQueryKey(),
        session.tenantId,
        session.account.id,
        session.activeDepartmentId,
      ],
    },
  });
  return {
    ...query,
    data: query.data?.status === 200 ? query.data.data : empty,
  };
}
export function useRefreshAdmin() {
  const client = useQueryClient();
  return async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: getIdentitySnapshotQueryKey() }),
      client.invalidateQueries({ queryKey: getIdentityMeQueryKey() }),
    ]);
  };
}
export function errorText(error: unknown, t: TFunction) {
  const code = error instanceof ApiError ? error.code : "internal";
  const messages: Record<string, string> = {
    invalid: t("admin:errors.invalid"),
    unauthenticated: t("admin:errors.unauthenticated"),
    forbidden: t("admin:errors.forbidden"),
    not_found: t("admin:errors.not_found"),
    conflict: t("admin:errors.conflict"),
    unavailable: t("admin:errors.unavailable"),
    rate_limited: t("admin:errors.rate_limited"),
    internal: t("admin:errors.internal"),
  };
  return messages[code] ?? messages.internal;
}
export function validationText(code: string | undefined, t: TFunction) {
  if (!code) return undefined;
  const messages: Record<string, string> = {
    required: t("admin:validation.required"),
    reason: t("admin:validation.reason"),
    note: t("admin:validation.note"),
    code: t("admin:validation.code"),
    username: t("admin:validation.username"),
    email: t("admin:validation.email"),
    password: t("admin:validation.password"),
    departments: t("admin:validation.departments"),
  };
  return messages[code] ?? messages.required;
}
export function initials(value: string) {
  return value
    .split(/\s+/)
    .map((part) => part[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
}
export function BrandMark() {
  return (
    <span className="console-brand-mark" aria-hidden="true">
      <svg viewBox="0 0 28 28" fill="none">
        <path
          d="M7 5l16 16M4 10l14 14M11 3l14 14M3 17l8 8"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
    </span>
  );
}
export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  return (
    <span
      className={`entity-status ${status === "active" ? "is-active" : "is-inactive"}`}
    >
      <i />
      {status === "active"
        ? t("admin:status.active")
        : t("admin:status.inactive")}
    </span>
  );
}
export function ScopeLabel({ scope }: { scope: string }) {
  const { t } = useTranslation();
  const values: Record<string, string> = {
    ALL: t("admin:permissions.all"),
    ORGANIZATION: t("admin:permissions.organization"),
    DEPARTMENT: t("admin:permissions.department"),
    DEPARTMENT_TREE: t("admin:permissions.departmentTree"),
    OWN: t("admin:permissions.own"),
    CUSTOM: t("admin:permissions.custom"),
  };
  return <span className="scope-badge">{values[scope] ?? scope}</span>;
}
export function ShortDate({ value }: { value: string }) {
  const config = useConfiguration();
  const { t, i18n } = useTranslation();
  return (
    <time dateTime={value || undefined}>
      {value
        ? new Intl.DateTimeFormat(i18n.resolvedLanguage, {
            timeStyle: "short",
            hourCycle: config.value("display.hour24", true) ? "h23" : "h12",
            dateStyle: config.value("display.dateStyle", "short") as
              "short" | "medium" | "long",
            timeZone: String(config.value("display.timezone", "Asia/Shanghai")),
          }).format(new Date(value))
        : t("admin:fields.never")}
    </time>
  );
}
export function PageHeading({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children?: ReactNode;
}) {
  return (
    <div className="admin-page-heading">
      <div>
        <h1>{title}</h1>
        <p>{subtitle}</p>
      </div>
      <div className="heading-actions">{children}</div>
    </div>
  );
}
export function ErrorBanner({
  error,
  onRetry,
}: {
  error: unknown;
  onRetry?: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="admin-error" role="alert">
      <AlertCircle size={17} />
      <span>{errorText(error, t)}</span>
      {onRetry && (
        <button onClick={onRetry}>{t("admin:feedback.retry")}</button>
      )}
    </div>
  );
}
export function SuccessNotice({
  message,
  onClose,
}: {
  message: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="admin-toast" role="status">
      <Check size={17} />
      {message}
      <button onClick={onClose} aria-label={t("admin:actions.close")}>
        <X size={15} />
      </button>
    </div>
  );
}
export function Loading() {
  const { t } = useTranslation();
  return (
    <div className="admin-loading" role="status">
      <LoaderCircle size={22} className="animate-spin" />
      {t("admin:feedback.loading")}
    </div>
  );
}
export function EmptyState() {
  const { t } = useTranslation();
  return (
    <div className="admin-empty">
      <span>
        <Inbox size={25} />
      </span>
      <strong>{t("admin:feedback.empty")}</strong>
      <p>{t("admin:feedback.emptyHint")}</p>
    </div>
  );
}
export function ListSearch({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="list-search">
      <Search size={15} />
      <Input
        aria-label={t("admin:search.local")}
        placeholder={t("admin:search.local")}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
const tableOptions = tableFeatures({});
export type AdminColumn<T> = {
  id: string;
  header: string;
  cell: (row: T) => ReactNode;
  className?: string;
};
export function DataTable<T extends { id: number }>({
  rows,
  columns,
  selectable = false,
}: {
  rows: T[];
  columns: AdminColumn<T>[];
  selectable?: boolean;
}) {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const config = useConfiguration();
  const pageSize = Number(config.value("display.pageSize", 8));
  const pages = Math.max(1, Math.ceil(rows.length / pageSize));
  const current = Math.min(page, pages);
  const visible = useMemo(
    () => rows.slice((current - 1) * pageSize, current * pageSize),
    [rows, current, pageSize],
  );
  const defs = useMemo<ColumnDef<typeof tableOptions, T>[]>(
    () =>
      columns.map((column) => ({
        id: column.id,
        header: column.header,
      })),
    [columns],
  );
  const table = useTable({
    features: tableOptions,
    data: visible,
    columns: defs,
    getRowId: (row) => String(row.id),
  });
  const toggle = (id: number) =>
    setSelected((old) => {
      const next = new Set(old);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  return (
    <>
      <div className="admin-table-scroll">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((group) => (
              <TableRow key={group.id}>
                {selectable && (
                  <TableHead className="selection-cell">
                    <input
                      type="checkbox"
                      aria-label={t("admin:actions.selectAll")}
                      checked={
                        visible.length > 0 &&
                        visible.every((row) => selected.has(row.id))
                      }
                      onChange={(event) =>
                        setSelected(
                          event.target.checked
                            ? new Set(visible.map((row) => row.id))
                            : new Set(),
                        )
                      }
                    />
                  </TableHead>
                )}
                {group.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={
                      columns.find((column) => column.id === header.column.id)
                        ?.className
                    }
                  >
                    <table.FlexRender header={header} />
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow
                key={row.id}
                data-selected={selected.has(row.original.id) || undefined}
              >
                {selectable && (
                  <TableCell className="selection-cell">
                    <input
                      type="checkbox"
                      aria-label={t("admin:accounts.details")}
                      checked={selected.has(row.original.id)}
                      onChange={() => toggle(row.original.id)}
                    />
                  </TableCell>
                )}
                {row.getAllCells().map((cell) => (
                  <TableCell
                    key={cell.id}
                    className={
                      columns.find((column) => column.id === cell.column.id)
                        ?.className
                    }
                  >
                    {/* Domain render callbacks return elements; invoking them keeps child component identity stable across query updates. */}
                    {columns
                      .find((column) => column.id === cell.column.id)
                      ?.cell(row.original)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {rows.length === 0 && <EmptyState />}
      </div>
      <div className="table-pagination">
        <span>
          {t("admin:feedback.rows", {
            from: rows.length ? (current - 1) * pageSize + 1 : 0,
            to: Math.min(current * pageSize, rows.length),
            total: rows.length,
          })}
        </span>
        <div>
          <span>
            {t("admin:feedback.page", { page: current, total: pages })}
          </span>
          <Button
            variant="outline"
            size="icon-sm"
            disabled={current === 1}
            aria-label={t("admin:actions.previous")}
            onClick={() => setPage(current - 1)}
          >
            <ChevronLeft size={14} />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            disabled={current === pages}
            aria-label={t("admin:actions.next")}
            onClick={() => setPage(current + 1)}
          >
            <ChevronRight size={14} />
          </Button>
        </div>
      </div>
    </>
  );
}
export function EditorDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  wide = false,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  children: ReactNode;
  wide?: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={`admin-dialog ${wide ? "dialog-wide" : ""}`}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  );
}
const deletions = {
  accounts: identityDeleteAccount,
  organizations: identityDeleteOrganization,
  departments: identityDeleteDepartment,
  roles: identityDeleteRole,
};
export function DeleteDialog({
  target,
  resource,
  onClose,
  onDeleted,
}: {
  target: { id: number; name: string } | null;
  resource: Resource;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const refresh = useRefreshAdmin();
  async function remove() {
    if (!target) return;
    setBusy(true);
    setError(null);
    try {
      await deletions[resource](target.id);
      await refresh();
      onDeleted();
      onClose();
    } catch (cause) {
      setError(cause);
    } finally {
      setBusy(false);
    }
  }
  return (
    <EditorDialog
      open={!!target}
      onOpenChange={(open) => {
        if (!open) {
          setError(null);
          onClose();
        }
      }}
      title={t("admin:feedback.deleteTitle")}
      description={t("admin:feedback.deleteDescription", {
        name: target?.name ?? "",
      })}
    >
      {!!error && <ErrorBanner error={error} />}
      <div className="editor-footer">
        <Button type="button" variant="outline" onClick={onClose}>
          {t("admin:actions.cancel")}
        </Button>
        <Button
          type="button"
          className="destructive-button"
          disabled={busy}
          onClick={() => {
            void remove();
          }}
        >
          <Trash2 size={14} />
          {busy ? t("admin:actions.saving") : t("admin:actions.delete")}
        </Button>
      </div>
    </EditorDialog>
  );
}
