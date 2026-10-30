import { z } from "zod";
import { usePageTools } from "@/features/webmcp/provider";
import { pageTool } from "@/lib/webmcp/runtime";
import { useMemo, useState, type ReactNode } from "react";
import { useSearchParams, Link } from "react-router";
import { useTranslation } from "react-i18next";
import {
  Plus,
  MoreHorizontal,
  Pencil,
  Trash2,
  Eye,
  ShieldCheck,
  Users,
  Building2,
  Building,
  Network,
  ChevronDown,
  ChevronRight,
  KeyRound,
  LayoutGrid,
  List,
  ArrowUpRight,
  GitBranch,
  LockKeyhole,
} from "lucide-react";
import type {
  AccountRecord,
  OrganizationRecord,
  DepartmentRecord,
  RoleRecord,
} from "@/api/generated/models";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { useSession } from "./auth";
import {
  DataTable,
  PageHeading,
  ListSearch,
  StatusBadge,
  ShortDate,
  ScopeLabel,
  EditorDialog,
  DeleteDialog,
  SuccessNotice,
  Loading,
  ErrorBanner,
  EmptyState,
  initials,
  useAdminData,
  type AdminColumn,
} from "./shared";
import {
  AccountForm,
  OrganizationForm,
  DepartmentForm,
  RoleForm,
} from "./forms";
function RowActions({
  actions,
}: {
  actions: {
    label: string;
    icon: ReactNode;
    onClick: () => void;
    danger?: boolean;
  }[];
}) {
  const { t } = useTranslation();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button className="row-more" aria-label={t("admin:fields.actions")}>
            <MoreHorizontal size={18} />
          </button>
        }
      />
      <DropdownMenuContent align="end">
        {actions.map((action) => (
          <DropdownMenuItem
            key={action.label}
            className={action.danger ? "destructive-menu-item" : ""}
            onClick={action.onClick}
          >
            {action.icon}
            {action.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
function NoAccess() {
  const { t } = useTranslation();
  return (
    <div className="admin-empty">
      <LockKeyhole size={30} />
      <strong>{t("admin:errors.forbidden")}</strong>
      <Link to="/">{t("admin:actions.back")}</Link>
      <Link to="/temporary-access">{t("admin:temporary.create")}</Link>
    </div>
  );
}
function StatusFilter({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <select
      className="toolbar-select"
      aria-label={t("admin:fields.status")}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    >
      <option value="all">{t("admin:status.all")}</option>
      <option value="active">{t("admin:status.active")}</option>
      <option value="inactive">{t("admin:status.inactive")}</option>
    </select>
  );
}
export function AccountsPage() {
  const { t } = useTranslation();
  const session = useSession();
  const query = useAdminData();
  const data = query.data;
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const status = params.get("status") ?? "all";
  const [dialog, setDialog] = useState<{
    record?: AccountRecord;
    readOnly?: boolean;
    access?: boolean;
  } | null>(null);
  const [deleting, setDeleting] = useState<AccountRecord | null>(null);
  const [notice, setNotice] = useState("");
  const can = (action: string) =>
    session.permissions.includes(`accounts:${action}`);
  function filter(key: string, value: string) {
    setParams((old) => {
      const next = new URLSearchParams(old);
      if (value && value !== "all") next.set(key, value);
      else next.delete(key);
      return next;
    });
  }
  const rows = useMemo(
    () =>
      data.accounts.filter(
        (account) =>
          (status === "all" || account.status === status) &&
          `${account.fullName} ${account.username} ${account.email}`
            .toLowerCase()
            .includes(search.toLowerCase()),
      ),
    [data.accounts, status, search],
  );
  usePageTools([
    pageTool(
      "baseline_accounts_filter",
      "Filter the authorized account page. Returns matching count only, no account records.",
      z
        .object({
          query: z.string().max(100).default(""),
          status: z.enum(["all", "active", "inactive"]).default("all"),
        })
        .strict(),
      ({ query: text, status: selected }) => {
        if (!can("read") || query.isPending || query.isError)
          throw new Error("unavailable");
        const next = new URLSearchParams();
        if (text) next.set("q", text);
        if (selected !== "all") next.set("status", selected);
        setParams(next);
        return {
          matchingCount: data.accounts.filter(
            (account) =>
              (selected === "all" || account.status === selected) &&
              `${account.fullName} ${account.username} ${account.email}`
                .toLowerCase()
                .includes(text.toLowerCase()),
          ).length,
        };
      },
      { scope: "page", effect: "interaction", permission: "accounts:read" },
    ),
    pageTool(
      "baseline_accounts_details",
      "Open read-only details for an account already present in the authorized page data. Does not edit or submit.",
      z.object({ id: z.number().int().positive() }).strict(),
      ({ id }) => {
        if (!can("read") || query.isPending || query.isError)
          throw new Error("unavailable");
        const record = data.accounts.find((account) => account.id === id);
        if (!record) throw new Error("not_found");
        setDialog({ record, readOnly: true });
        return { opened: true, readOnly: true };
      },
      { scope: "page", effect: "interaction", permission: "accounts:read" },
    ),
  ]);
  const columns: AdminColumn<AccountRecord>[] = [
    {
      id: "name",
      header: t("admin:fields.fullName"),
      cell: (account) => (
        <button
          className="account-cell"
          onClick={() => setDialog({ record: account, readOnly: true })}
        >
          <span className={`person-avatar tone-${account.id % 5}`}>
            {initials(account.fullName)}
          </span>
          <span>
            <strong>
              {account.fullName}
              {account.isAdmin && <ShieldCheck size={13} />}
            </strong>
            <small>{account.email || account.username}</small>
          </span>
        </button>
      ),
    },
    {
      id: "username",
      header: t("admin:fields.username"),
      cell: (account) => <span className="mono-muted">{account.username}</span>,
    },
    {
      id: "organization",
      header: t("admin:fields.organization"),
      cell: (account) =>
        data.organizations.find((org) => org.id === account.organizationId)
          ?.name ?? t("admin:fields.unassigned"),
    },
    {
      id: "department",
      header: t("admin:fields.department"),
      cell: (account) => (
        <div className="cell-tags">
          {account.memberships.map((m) => (
            <span className="neutral-badge" key={m.departmentId}>
              {m.departmentName ??
                data.departments.find((d) => d.id === m.departmentId)?.name ??
                t("admin:fields.unassigned")}
            </span>
          ))}
        </div>
      ),
    },
    {
      id: "roles",
      header: t("admin:fields.role"),
      cell: (account) => (
        <div className="cell-tags">
          {account.isAdmin ? (
            <span className="neutral-badge">
              <ShieldCheck size={11} />
              {t("admin:accounts.admin")}
            </span>
          ) : account.memberships.some((m) => m.roleIds.length > 0) ? (
            account.memberships.flatMap((m) =>
              m.roleIds.map((id) => (
                <span className="neutral-badge" key={`${m.departmentId}-${id}`}>
                  {t("admin:memberships.roleLabel", {
                    department:
                      m.departmentName ??
                      data.departments.find((d) => d.id === m.departmentId)
                        ?.name ??
                      t("admin:fields.unassigned"),
                    role:
                      data.roles.find((r) => r.id === id)?.name ??
                      t("admin:accounts.roleCount", { count: 1 }),
                  })}
                </span>
              )),
            )
          ) : (
            <span className="muted-text">{t("admin:accounts.noRoles")}</span>
          )}
        </div>
      ),
    },
    {
      id: "status",
      header: t("admin:fields.status"),
      cell: (account) => <StatusBadge status={account.status} />,
    },
    {
      id: "created",
      header: t("admin:fields.createdAt"),
      cell: (account) => <ShortDate value={account.createdAt} />,
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (account) => (
        <RowActions
          actions={[
            {
              label: t("admin:actions.details"),
              icon: <Eye size={14} />,
              onClick: () => setDialog({ record: account, readOnly: true }),
            },
            ...(can("update")
              ? [
                  {
                    label: t("admin:actions.edit"),
                    icon: <Pencil size={14} />,
                    onClick: () => setDialog({ record: account }),
                  },
                ]
              : []),
            ...(can("update") &&
            session.permissions.includes("access:manage") &&
            (!account.isAdmin || account.id === session.account.id)
              ? [
                  {
                    label: t("admin:actions.manageAccess"),
                    icon: <KeyRound size={14} />,
                    onClick: () => setDialog({ record: account, access: true }),
                  },
                ]
              : []),
            ...(can("delete") &&
            !account.isAdmin &&
            account.id !== session.account.id
              ? [
                  {
                    label: t("admin:actions.delete"),
                    icon: <Trash2 size={14} />,
                    danger: true,
                    onClick: () => setDeleting(account),
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];
  if (!can("read")) return <NoAccess />;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("admin:accounts.title")}
        subtitle={t("admin:accounts.subtitle")}
      >
        {can("create") && (
          <Button onClick={() => setDialog({})}>
            <Plus size={15} />
            {t("admin:accounts.create")}
          </Button>
        )}
      </PageHeading>
      {query.isError && (
        <ErrorBanner
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      <section className="admin-panel">
        <div className="panel-tabs">
          <button
            className={status === "all" ? "active" : ""}
            onClick={() => filter("status", "all")}
          >
            {t("admin:actions.all")}
            <span>{data.accounts.length}</span>
          </button>
          <button
            className={status === "active" ? "active" : ""}
            onClick={() => filter("status", "active")}
          >
            {t("admin:status.active")}
            <span>
              {data.accounts.filter((a) => a.status === "active").length}
            </span>
          </button>
          <button
            className={status === "inactive" ? "active" : ""}
            onClick={() => filter("status", "inactive")}
          >
            {t("admin:status.inactive")}
            <span>
              {data.accounts.filter((a) => a.status === "inactive").length}
            </span>
          </button>
        </div>
        <div className="table-toolbar">
          <ListSearch value={search} onChange={(value) => filter("q", value)} />
          <div className="toolbar-right">
            <StatusFilter
              value={status}
              onChange={(value) => filter("status", value)}
            />
          </div>
        </div>
        <DataTable rows={rows} columns={columns} selectable />
      </section>
      <EditorDialog
        open={!!dialog}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        title={
          dialog?.readOnly
            ? t("admin:accounts.details")
            : dialog?.record
              ? t("admin:accounts.edit")
              : t("admin:accounts.create")
        }
        description={t("admin:accounts.description")}
        wide
      >
        <AccountForm
          key={`${dialog?.record?.id ?? "new"}-${!!dialog?.access}-${!!dialog?.readOnly}`}
          record={dialog?.record}
          readOnly={dialog?.readOnly}
          permissionsOnly={dialog?.access}
          onCancel={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            setNotice(t("admin:feedback.saved"));
          }}
        />
      </EditorDialog>
      <DeleteDialog
        target={deleting ? { id: deleting.id, name: deleting.fullName } : null}
        resource="accounts"
        onClose={() => setDeleting(null)}
        onDeleted={() => setNotice(t("admin:feedback.deleted"))}
      />
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
export function OrganizationsPage() {
  const { t } = useTranslation();
  const session = useSession();
  const query = useAdminData();
  const data = query.data;
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const [selected, setSelected] = useState(0);
  const [kind, setKind] = useState("all");
  const [editing, setEditing] = useState<
    OrganizationRecord | null | undefined
  >();
  const [deleting, setDeleting] = useState<OrganizationRecord | null>(null);
  const [notice, setNotice] = useState("");
  const can = (action: string) =>
    session.permissions.includes(`organizations:${action}`);
  const rows = useMemo(
    () =>
      data.organizations.filter(
        (org) =>
          (kind === "all" || org.kind === kind) &&
          (!selected || org.id === selected || org.parentId === selected) &&
          `${org.name} ${org.code}`
            .toLowerCase()
            .includes(search.toLowerCase()),
      ),
    [data.organizations, kind, search, selected],
  );
  usePageTools([
    pageTool(
      "baseline_organizations_filter",
      "Filter the authorized organizations page by name or code using its existing search state.",
      z.object({ query: z.string().max(100) }).strict(),
      ({ query: text }) => {
        if (!can("read") || query.isPending || query.isError)
          throw new Error("unavailable");
        setParams(text ? { q: text } : {});
        return { requestedQuery: text };
      },
      {
        scope: "page",
        effect: "interaction",
        permission: "organizations:read",
      },
    ),
  ]);
  const columns: AdminColumn<OrganizationRecord>[] = [
    {
      id: "name",
      header: t("admin:fields.name"),
      cell: (org) => (
        <div className="entity-name">
          <span className="entity-symbol">
            {org.kind === "enterprise" ? (
              <Building2 size={17} />
            ) : (
              <Building size={17} />
            )}
          </span>
          <div>
            <strong>{org.name}</strong>
            <small>{org.code}</small>
          </div>
        </div>
      ),
    },
    {
      id: "kind",
      header: t("admin:organizations.kind"),
      cell: (org) => (
        <span className="neutral-badge">
          {org.kind === "enterprise"
            ? t("admin:organizations.enterprise")
            : t("admin:organizations.company")}
        </span>
      ),
    },
    {
      id: "parent",
      header: t("admin:fields.parent"),
      cell: (org) =>
        data.organizations.find((parent) => parent.id === org.parentId)?.name ??
        t("admin:organizations.none"),
    },
    {
      id: "status",
      header: t("admin:fields.status"),
      cell: (org) => <StatusBadge status={org.status} />,
    },
    {
      id: "created",
      header: t("admin:fields.createdAt"),
      cell: (org) => <ShortDate value={org.createdAt} />,
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (org) => (
        <RowActions
          actions={[
            ...(can("update")
              ? [
                  {
                    label: t("admin:actions.edit"),
                    icon: <Pencil size={14} />,
                    onClick: () => setEditing(org),
                  },
                ]
              : []),
            ...(can("delete")
              ? [
                  {
                    label: t("admin:actions.delete"),
                    icon: <Trash2 size={14} />,
                    danger: true,
                    onClick: () => setDeleting(org),
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];
  if (!can("read")) return <NoAccess />;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("admin:organizations.title")}
        subtitle={t("admin:organizations.subtitle")}
      >
        {can("create") && (
          <Button onClick={() => setEditing(null)}>
            <Plus size={15} />
            {t("admin:organizations.create")}
          </Button>
        )}
      </PageHeading>
      {query.isError && <ErrorBanner error={query.error} />}
      <div className="management-split">
        <aside className="admin-panel tree-panel">
          <div className="tree-title">
            <Network size={16} />
            {t("admin:organizations.tree")}
          </div>
          <button
            className={`tree-node ${selected === 0 ? "selected" : ""}`}
            onClick={() => setSelected(0)}
          >
            <LayoutGrid size={15} />
            <span>{t("admin:organizations.all")}</span>
            <small>{data.organizations.length}</small>
          </button>
          <div className="organization-tree">
            {data.organizations
              .filter(
                (org) =>
                  org.parentId === 0 ||
                  !data.organizations.some(
                    (parent) => parent.id === org.parentId,
                  ),
              )
              .map((org) => (
                <div key={org.id}>
                  <button
                    className={`tree-node ${selected === org.id ? "selected" : ""}`}
                    onClick={() => setSelected(org.id)}
                  >
                    <ChevronDown size={12} />
                    {org.kind === "enterprise" ? (
                      <Building2 size={15} />
                    ) : (
                      <Building size={15} />
                    )}
                    <span>{org.name}</span>
                  </button>
                  {data.organizations
                    .filter((child) => child.parentId === org.id)
                    .map((child) => (
                      <button
                        className={`tree-node tree-child ${selected === child.id ? "selected" : ""}`}
                        key={child.id}
                        onClick={() => setSelected(child.id)}
                      >
                        <Building size={14} />
                        <span>{child.name}</span>
                      </button>
                    ))}
                </div>
              ))}
          </div>
          <div className="tree-footnote">
            <GitBranch size={15} />
            {t("admin:organizations.description")}
          </div>
        </aside>
        <section className="admin-panel">
          <div className="panel-tabs">
            {[
              { key: "all", label: t("admin:actions.all") },
              {
                key: "enterprise",
                label: t("admin:organizations.enterprises"),
              },
              { key: "company", label: t("admin:organizations.companies") },
            ].map((tab) => (
              <button
                key={tab.key}
                className={kind === tab.key ? "active" : ""}
                onClick={() => setKind(tab.key)}
              >
                {tab.label}
                <span>
                  {
                    data.organizations.filter(
                      (org) => tab.key === "all" || org.kind === tab.key,
                    ).length
                  }
                </span>
              </button>
            ))}
          </div>
          <div className="table-toolbar">
            <ListSearch
              value={search}
              onChange={(value) => setParams(value ? { q: value } : {})}
            />
            <span className="muted-text">
              {t("admin:organizations.total", { count: rows.length })}
            </span>
          </div>
          <DataTable rows={rows} columns={columns} />
        </section>
      </div>
      <EditorDialog
        open={editing !== undefined}
        onOpenChange={(open) => {
          if (!open) setEditing(undefined);
        }}
        title={
          editing
            ? t("admin:organizations.edit")
            : t("admin:organizations.create")
        }
        description={t("admin:organizations.description")}
      >
        <OrganizationForm
          key={editing?.id ?? "new"}
          record={editing ?? undefined}
          kind={kind === "company" ? "company" : "enterprise"}
          onCancel={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            setNotice(t("admin:feedback.saved"));
          }}
        />
      </EditorDialog>
      <DeleteDialog
        resource="organizations"
        target={deleting}
        onClose={() => setDeleting(null)}
        onDeleted={() => {
          setSelected(0);
          setNotice(t("admin:feedback.deleted"));
        }}
      />
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
export function DepartmentsPage() {
  const { t } = useTranslation();
  const session = useSession();
  const query = useAdminData();
  const data = query.data;
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const [company, setCompany] = useState(0);
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [editing, setEditing] = useState<DepartmentRecord | null | undefined>();
  const [deleting, setDeleting] = useState<DepartmentRecord | null>(null);
  const [notice, setNotice] = useState("");
  const can = (action: string) =>
    session.permissions.includes(`departments:${action}`);
  const rows = useMemo(() => {
    const out: (DepartmentRecord & { depth: number })[] = [];
    const candidates = data.departments.filter(
      (dept) => !company || dept.organizationId === company,
    );
    const visited = new Set<number>();
    function append(parent: number, depth: number) {
      for (const dept of candidates.filter((row) => row.parentId === parent)) {
        if (visited.has(dept.id)) continue;
        visited.add(dept.id);
        if (
          `${dept.name} ${dept.code}`
            .toLowerCase()
            .includes(search.toLowerCase())
        )
          out.push({ ...dept, depth });
        if (!collapsed.has(dept.id) || search) append(dept.id, depth + 1);
      }
    }
    append(0, 0);
    for (const dept of candidates) {
      if (
        !visited.has(dept.id) &&
        !candidates.some((parent) => parent.id === dept.parentId)
      ) {
        out.push({ ...dept, depth: 0 });
        append(dept.id, 1);
      }
    }
    return out;
  }, [data.departments, company, search, collapsed]);
  usePageTools([
    pageTool(
      "baseline_departments_filter",
      "Filter the authorized departments page by name or code using its existing search state.",
      z.object({ query: z.string().max(100) }).strict(),
      ({ query: text }) => {
        if (!can("read") || query.isPending || query.isError)
          throw new Error("unavailable");
        setParams(text ? { q: text } : {});
        return { requestedQuery: text };
      },
      { scope: "page", effect: "interaction", permission: "departments:read" },
    ),
  ]);
  const columns: AdminColumn<DepartmentRecord & { depth: number }>[] = [
    {
      id: "name",
      header: t("admin:fields.name"),
      cell: (dept) => (
        <div
          className="department-name"
          style={{ paddingLeft: dept.depth * 19 }}
        >
          {data.departments.some((child) => child.parentId === dept.id) ? (
            <button
              aria-label={
                collapsed.has(dept.id)
                  ? t("admin:actions.expand")
                  : t("admin:actions.collapseDepartment")
              }
              onClick={() =>
                setCollapsed((old) => {
                  const next = new Set(old);
                  if (next.has(dept.id)) next.delete(dept.id);
                  else next.add(dept.id);
                  return next;
                })
              }
            >
              {collapsed.has(dept.id) ? (
                <ChevronRight size={13} />
              ) : (
                <ChevronDown size={13} />
              )}
            </button>
          ) : (
            <span className="tree-spacer" />
          )}
          <span className="entity-symbol">
            <Network size={15} />
          </span>
          <span>
            <strong>{dept.name}</strong>
            <small>{dept.code}</small>
          </span>
        </div>
      ),
    },
    {
      id: "company",
      header: t("admin:fields.organization"),
      cell: (dept) =>
        data.organizations.find((org) => org.id === dept.organizationId)
          ?.name ?? t("admin:fields.unassigned"),
    },
    {
      id: "manager",
      header: t("admin:departments.manager"),
      cell: (dept) => dept.manager || t("admin:fields.unassigned"),
    },
    {
      id: "members",
      header: t("admin:fields.members"),
      cell: (dept) => (
        <span className="member-count">
          <Users size={13} />
          {
            data.accounts.filter((account) =>
              account.memberships.some((m) => m.departmentId === dept.id),
            ).length
          }
        </span>
      ),
    },
    {
      id: "status",
      header: t("admin:fields.status"),
      cell: (dept) => <StatusBadge status={dept.status} />,
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (dept) => (
        <RowActions
          actions={[
            ...(can("update")
              ? [
                  {
                    label: t("admin:actions.edit"),
                    icon: <Pencil size={14} />,
                    onClick: () => setEditing(dept),
                  },
                ]
              : []),
            ...(can("delete")
              ? [
                  {
                    label: t("admin:actions.delete"),
                    icon: <Trash2 size={14} />,
                    danger: true,
                    onClick: () => setDeleting(dept),
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];
  if (!can("read")) return <NoAccess />;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("admin:departments.title")}
        subtitle={t("admin:departments.subtitle")}
      >
        {can("create") && (
          <Button onClick={() => setEditing(null)}>
            <Plus size={15} />
            {t("admin:departments.create")}
          </Button>
        )}
      </PageHeading>
      {query.isError && <ErrorBanner error={query.error} />}
      <div className="management-split">
        <aside className="admin-panel tree-panel">
          <div className="tree-title">
            <Building2 size={16} />
            {t("admin:departments.companyTree")}
          </div>
          <button
            className={`tree-node ${company === 0 ? "selected" : ""}`}
            onClick={() => setCompany(0)}
          >
            <LayoutGrid size={15} />
            <span>{t("admin:departments.allCompanies")}</span>
          </button>
          {data.organizations
            .filter((org) => org.kind === "company")
            .map((org) => (
              <button
                key={org.id}
                className={`tree-node ${company === org.id ? "selected" : ""}`}
                onClick={() => setCompany(org.id)}
              >
                <Building size={15} />
                <span>{org.name}</span>
                <small>
                  {
                    data.departments.filter(
                      (dept) => dept.organizationId === org.id,
                    ).length
                  }
                </small>
              </button>
            ))}
          <div className="tree-footnote">
            <ShieldCheck size={15} />
            {t("admin:overview.scopeNote")}
          </div>
        </aside>
        <section className="admin-panel">
          <div className="panel-heading">
            <div>
              <h2>
                {company
                  ? data.organizations.find((org) => org.id === company)?.name
                  : t("admin:departments.title")}
              </h2>
              <p>{t("admin:departments.total", { count: rows.length })}</p>
            </div>
            <Network size={18} />
          </div>
          <div className="table-toolbar">
            <ListSearch
              value={search}
              onChange={(value) => setParams(value ? { q: value } : {})}
            />
          </div>
          <DataTable rows={rows} columns={columns} />
        </section>
      </div>
      <EditorDialog
        open={editing !== undefined}
        onOpenChange={(open) => {
          if (!open) setEditing(undefined);
        }}
        title={
          editing ? t("admin:departments.edit") : t("admin:departments.create")
        }
        description={t("admin:departments.description")}
      >
        <DepartmentForm
          key={editing?.id ?? "new"}
          record={editing ?? undefined}
          companyId={company}
          onCancel={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            setNotice(t("admin:feedback.saved"));
          }}
        />
      </EditorDialog>
      <DeleteDialog
        resource="departments"
        target={deleting}
        onClose={() => setDeleting(null)}
        onDeleted={() => setNotice(t("admin:feedback.deleted"))}
      />
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
export function RolesPage() {
  const { t } = useTranslation();
  const session = useSession();
  const query = useAdminData();
  const data = query.data;
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const [view, setView] = useState<"grid" | "list">("grid");
  const [editing, setEditing] = useState<RoleRecord | null | undefined>();
  const [deleting, setDeleting] = useState<RoleRecord | null>(null);
  const [notice, setNotice] = useState("");
  const can = (action: string) =>
    session.permissions.includes(`roles:${action}`) &&
    session.permissions.includes("access:manage");
  const rows = useMemo(
    () =>
      data.roles.filter((role) =>
        `${role.name} ${role.code} ${role.description}`
          .toLowerCase()
          .includes(search.toLowerCase()),
      ),
    [data.roles, search],
  );
  usePageTools([
    pageTool(
      "baseline_roles_filter",
      "Filter the authorized roles page by name, code or description using its existing search state.",
      z.object({ query: z.string().max(100) }).strict(),
      ({ query: text }) => {
        if (
          !session.permissions.includes("roles:read") ||
          query.isPending ||
          query.isError
        )
          throw new Error("unavailable");
        setParams(text ? { q: text } : {});
        return { requestedQuery: text };
      },
      { scope: "page", effect: "interaction", permission: "roles:read" },
    ),
  ]);
  function actions(role: RoleRecord) {
    return [
      {
        label: t("admin:actions.details"),
        icon: <Eye size={14} />,
        onClick: () => setEditing(role),
      },
      ...(can("update")
        ? [
            {
              label: t("admin:actions.manageAccess"),
              icon: <Pencil size={14} />,
              onClick: () => setEditing(role),
            },
          ]
        : []),
      ...(can("delete")
        ? [
            {
              label: t("admin:actions.delete"),
              icon: <Trash2 size={14} />,
              danger: true,
              onClick: () => setDeleting(role),
            },
          ]
        : []),
    ];
  }
  const columns: AdminColumn<RoleRecord>[] = [
    {
      id: "name",
      header: t("admin:fields.name"),
      cell: (role) => (
        <div className="entity-name">
          <span className="entity-symbol">
            <ShieldCheck size={17} />
          </span>
          <div>
            <strong>{role.name}</strong>
            <small>{role.code}</small>
          </div>
        </div>
      ),
    },
    {
      id: "scope",
      header: t("admin:permissions.dataScope"),
      cell: (role) => (
        <div className="cell-tags">
          {[...new Set(role.grants.map((g) => g.scope))].map((scope) => (
            <ScopeLabel key={scope} scope={scope} />
          ))}
        </div>
      ),
    },
    {
      id: "members",
      header: t("admin:fields.members"),
      cell: (role) => t("admin:roles.members", { count: role.memberCount }),
    },
    {
      id: "status",
      header: t("admin:fields.status"),
      cell: (role) => <StatusBadge status={role.status} />,
    },
    {
      id: "created",
      header: t("admin:fields.createdAt"),
      cell: (role) => <ShortDate value={role.createdAt} />,
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      cell: (role) => <RowActions actions={actions(role)} />,
    },
  ];
  if (!session.permissions.includes("roles:read")) return <NoAccess />;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("admin:roles.title")}
        subtitle={t("admin:roles.subtitle")}
      >
        {can("create") && (
          <Button onClick={() => setEditing(null)}>
            <Plus size={15} />
            {t("admin:roles.create")}
          </Button>
        )}
      </PageHeading>
      {query.isError && <ErrorBanner error={query.error} />}
      <div className="standalone-toolbar">
        <ListSearch
          value={search}
          onChange={(value) => setParams(value ? { q: value } : {})}
        />
        <div className="toolbar-right">
          <span className="muted-text">
            {t("admin:roles.total", { count: rows.length })}
          </span>
          <div className="view-toggle">
            <button
              aria-label={t("admin:actions.grid")}
              className={view === "grid" ? "active" : ""}
              onClick={() => setView("grid")}
            >
              <LayoutGrid size={15} />
            </button>
            <button
              aria-label={t("admin:actions.list")}
              className={view === "list" ? "active" : ""}
              onClick={() => setView("list")}
            >
              <List size={15} />
            </button>
          </div>
        </div>
      </div>
      {view === "grid" ? (
        <div className="role-card-grid">
          {rows.map((role) => (
            <article className="admin-panel role-card" key={role.id}>
              <div className="role-card-top">
                <span className={`role-symbol tone-${role.id % 5}`}>
                  <ShieldCheck size={22} />
                </span>
                <RowActions actions={actions(role)} />
              </div>
              <h2>{role.name}</h2>
              <p>{role.description || t("admin:roles.noDescription")}</p>
              <div className="role-card-scopes">
                {[...new Set(role.grants.map((g) => g.scope))].map((scope) => (
                  <ScopeLabel key={scope} scope={scope} />
                ))}
              </div>
              <div className="role-card-meta">
                <span>
                  <Users size={14} />
                  {t("admin:roles.members", { count: role.memberCount })}
                </span>
                <span>
                  <KeyRound size={14} />
                  {t("admin:roles.permissions", { count: role.grants.length })}
                </span>
              </div>
              <div className="role-card-footer">
                <StatusBadge status={role.status} />
                <Link to={`/permissions?type=role&id=${role.id}`}>
                  {t("admin:actions.manageAccess")}
                  <ArrowUpRight size={14} />
                </Link>
              </div>
            </article>
          ))}
          {rows.length === 0 && <EmptyState />}
        </div>
      ) : (
        <section className="admin-panel">
          <DataTable rows={rows} columns={columns} />
        </section>
      )}
      <EditorDialog
        open={editing !== undefined}
        onOpenChange={(open) => {
          if (!open) setEditing(undefined);
        }}
        title={editing ? t("admin:roles.edit") : t("admin:roles.create")}
        description={t("admin:roles.description")}
        wide
      >
        <RoleForm
          key={editing?.id ?? "new"}
          record={editing ?? undefined}
          onCancel={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            setNotice(t("admin:feedback.saved"));
          }}
        />
      </EditorDialog>
      <DeleteDialog
        resource="roles"
        target={deleting}
        onClose={() => setDeleting(null)}
        onDeleted={() => setNotice(t("admin:feedback.deleted"))}
      />
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
export function PermissionsPage() {
  const { t } = useTranslation();
  const session = useSession();
  const query = useAdminData();
  const { data } = query;
  const [params, setParams] = useSearchParams();
  const mode = params.get("type") === "account" ? "account" : "role";
  const [notice, setNotice] = useState("");
  const items =
    mode === "role"
      ? data.roles.map((role) => ({
          id: role.id,
          name: role.name,
          code: role.code,
        }))
      : data.accounts.map((account) => ({
          id: account.id,
          name: account.fullName,
          code: account.username,
        }));
  const id = Number(params.get("id")) || items[0]?.id || 0;
  const role = data.roles.find((item) => item.id === id);
  const account = data.accounts.find((item) => item.id === id);
  if (
    !session.permissions.includes("roles:read") &&
    !session.permissions.includes("accounts:read")
  )
    return <NoAccess />;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("admin:permissions.title")}
        subtitle={t("admin:permissions.subtitle")}
      />
      {query.isError && <ErrorBanner error={query.error} />}
      <div className="permission-page-tabs">
        <button
          className={mode === "role" ? "active" : ""}
          onClick={() => setParams({ type: "role" })}
        >
          <ShieldCheck size={15} />
          {t("admin:permissions.roleTab")}
        </button>
        <button
          className={mode === "account" ? "active" : ""}
          onClick={() => setParams({ type: "account" })}
        >
          <Users size={15} />
          {t("admin:permissions.accountTab")}
        </button>
      </div>
      <div className="management-split permissions-split">
        <aside className="admin-panel permission-subjects">
          <div className="tree-title">
            {mode === "role"
              ? t("admin:permissions.selectRole")
              : t("admin:permissions.selectAccount")}
          </div>
          {items.map((item) => (
            <button
              className={`subject-item ${id === item.id ? "selected" : ""}`}
              key={item.id}
              onClick={() => setParams({ type: mode, id: String(item.id) })}
            >
              <span className="subject-icon">
                {mode === "role" ? (
                  <ShieldCheck size={17} />
                ) : (
                  <Users size={17} />
                )}
              </span>
              <span>
                <strong>{item.name}</strong>
                <small>{item.code}</small>
              </span>
              <ChevronRight size={14} />
            </button>
          ))}
          {!items.length && <EmptyState />}
        </aside>
        <section className="admin-panel permission-detail">
          {mode === "role" && role ? (
            <>
              <div className="permission-detail-heading">
                <div>
                  <h2>{role.name}</h2>
                  <p>{role.description || t("admin:roles.description")}</p>
                </div>
                <StatusBadge status={role.status} />
              </div>
              <RoleForm
                key={`role-${role.id}`}
                record={role}
                permissionsOnly
                onSaved={() => setNotice(t("admin:feedback.saved"))}
              />
            </>
          ) : mode === "account" && account ? (
            <>
              <div className="permission-detail-heading">
                <div>
                  <h2>{account.fullName}</h2>
                  <p>{account.username}</p>
                </div>
                <StatusBadge status={account.status} />
              </div>
              <AccountForm
                key={`account-${account.id}`}
                record={account}
                permissionsOnly
                readOnly={
                  (account.isAdmin && account.id !== session.account.id) ||
                  !session.permissions.includes("access:manage") ||
                  !session.permissions.includes("accounts:update")
                }
                onSaved={() => setNotice(t("admin:feedback.saved"))}
              />
            </>
          ) : (
            <EmptyState />
          )}
        </section>
      </div>
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
