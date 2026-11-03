import { useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import {
  Archive,
  ArrowUpRight,
  CheckCheck,
  Eye,
  FolderKanban,
  LayoutGrid,
  List,
  ListChecks,
  Pause,
  Pencil,
  Play,
  Plus,
  TriangleAlert,
} from "lucide-react";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  ListSearch,
  Loading,
  PageHeading,
  RowActions,
  ShortDate,
  StateBadge,
  stateTone,
  SuccessNotice,
  type AdminColumn,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import {
  projectStatusChange,
  projectsList,
  type Project,
} from "@/lib/capabilities";

import { ProjectForm } from "./forms";

const projectStatusValues = [
  "active",
  "paused",
  "completed",
  "archived",
] as const;

// projectTransitions mirrors the declared state machine; archiving is offered to
// the owner only and the server enforces it again.
const projectTransitions: Record<string, string[]> = {
  active: ["paused", "completed", "archived"],
  paused: ["active", "completed", "archived"],
  completed: ["archived"],
  archived: [],
};

const transitionIcons: Record<string, typeof Pause> = {
  paused: Pause,
  active: Play,
  completed: CheckCheck,
  archived: Archive,
};

/**
 * ProjectsPage follows the role page: one toolbar with search, a state filter
 * and a grid/list switch, and the same card anatomy in the grid view.
 */
export function ProjectsPage() {
  const { t } = useTranslation(["projects", "admin"]);
  const session = useSession();
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const status = params.get("status") ?? "all";
  const [view, setView] = useState<"grid" | "list">("grid");
  const [dialog, setDialog] = useState<{ record?: Project } | null>(null);
  const [notice, setNotice] = useState("");

  const can = (action: string) =>
    session.permissions.includes(`projects:${action}`);

  const query = useQuery({
    queryKey: ["projects"],
    queryFn: () => projectsList({ limit: 100 }),
    enabled: can("read"),
    retry: false,
  });

  function filter(key: string, value: string) {
    setParams(
      (old) => {
        const next = new URLSearchParams(old);
        if (value && value !== "all") next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  }

  const items = query.data?.data.items ?? [];
  const rows = useMemo(
    () =>
      items.filter(
        (project) =>
          (status === "all" || project.status === status) &&
          `${project.code} ${project.name} ${project.ownerName}`
            .toLowerCase()
            .includes(search.toLowerCase()),
      ),
    [items, status, search],
  );

  const transition = (project: Project, next: string) => {
    void projectStatusChange(project.id, project.status, next).then(() => {
      setNotice(t("admin:feedback.saved"));
      void query.refetch();
    });
  };

  // The card and the table share one command list so the two views never drift.
  const actions = (project: Project) => [
    {
      label: t("open"),
      icon: <Eye size={14} />,
      onClick: () => navigate(`/projects/${project.id}`),
    },
    ...(can("write") && project.status !== "archived"
      ? [
          {
            label: t("admin:actions.edit"),
            icon: <Pencil size={14} />,
            onClick: () => setDialog({ record: project }),
          },
        ]
      : []),
    ...(can("write")
      ? projectTransitions[project.status]
          .filter(
            (next) =>
              next !== "archived" || project.ownerId === session.account.id,
          )
          .map((next) => {
            const Icon = transitionIcons[next];
            return {
              label: t(`statusActions.${next}`),
              icon: <Icon size={14} />,
              onClick: () => transition(project, next),
            };
          })
      : []),
  ];

  const columns: AdminColumn<Project>[] = [
    {
      id: "name",
      header: t("name"),
      cell: (project) => (
        <button
          className="account-cell"
          onClick={() => navigate(`/projects/${project.id}`)}
        >
          <span>
            <strong>{project.name}</strong>
            <small>{project.code}</small>
          </span>
        </button>
      ),
    },
    {
      id: "owner",
      header: t("owner"),
      cell: (project) => project.ownerName,
    },
    {
      id: "priority",
      header: t("priority"),
      cell: (project) => t(`priorities.${project.priority}`),
    },
    {
      id: "due",
      header: t("dueDate"),
      cell: (project) =>
        project.dueDate ? <ShortDate value={project.dueDate} /> : t("dash"),
    },
    {
      id: "progress",
      header: t("progress"),
      cell: (project) =>
        t("ratio", { done: project.taskDoneCount, total: project.taskCount }),
    },
    {
      id: "status",
      header: t("status"),
      cell: (project) => (
        <StateBadge
          label={t(`states.${project.status}`)}
          tone={stateTone(project.status)}
        />
      ),
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (project) => <RowActions actions={actions(project)} />,
    },
  ];

  if (!can("read")) {
    return (
      <div className="admin-empty">
        <strong>{t("admin:errors.forbidden")}</strong>
      </div>
    );
  }
  if (query.isPending) return <Loading />;

  return (
    <>
      <PageHeading title={t("heading")} subtitle={t("description")} />
      {query.isError && (
        <ErrorBanner
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      <div className="standalone-toolbar">
        <ListSearch value={search} onChange={(value) => filter("q", value)} />
        <div className="toolbar-right">
          <select
            className="toolbar-select"
            aria-label={t("status")}
            value={status}
            onChange={(event) => filter("status", event.target.value)}
          >
            <option value="all">{t("admin:actions.all")}</option>
            {projectStatusValues.map((value) => (
              <option key={value} value={value}>
                {t(`states.${value}`)}
              </option>
            ))}
          </select>
          <span className="muted-text">
            {t("total", { count: rows.length })}
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
          {can("admin") && (
            <Button onClick={() => setDialog({})}>
              <Plus size={15} />
              {t("create")}
            </Button>
          )}
        </div>
      </div>

      {view === "grid" ? (
        <div className="role-card-grid">
          {rows.map((project) => (
            <article className="admin-panel role-card" key={project.id}>
              <div className="role-card-top">
                <span className={`role-symbol tone-${project.id % 5}`}>
                  <FolderKanban size={22} />
                </span>
                <RowActions actions={actions(project)} />
              </div>
              <h2>{project.name}</h2>
              <p>
                {t("cardMeta", {
                  code: project.code,
                  owner: project.ownerName,
                })}
              </p>
              <div className="role-card-scopes">
                <StateBadge
                  label={t(`states.${project.status}`)}
                  tone={stateTone(project.status)}
                />
                <span className="scope-badge">
                  {t(`priorities.${project.priority}`)}
                </span>
              </div>
              <div className="role-card-meta">
                <span>
                  <ListChecks size={14} />
                  {t("plansCount", { count: project.planCount })}
                </span>
                <span>
                  <CheckCheck size={14} />
                  {t("ratio", {
                    done: project.taskDoneCount,
                    total: project.taskCount,
                  })}
                </span>
                <span>
                  <TriangleAlert size={14} />
                  {project.taskOverdueCount}
                </span>
              </div>
              <div className="role-card-footer">
                <span className="muted-text">
                  {project.dueDate ? (
                    <ShortDate value={project.dueDate} />
                  ) : (
                    t("dash")
                  )}
                </span>
                <Link to={`/projects/${project.id}`}>
                  {t("open")}
                  <ArrowUpRight size={14} />
                </Link>
              </div>
            </article>
          ))}
          {rows.length === 0 && (
            <p className="muted-text">{t("admin:feedback.empty")}</p>
          )}
        </div>
      ) : (
        <section className="admin-panel">
          <DataTable rows={rows} columns={columns} />
        </section>
      )}

      <EditorDialog
        open={!!dialog}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        title={dialog?.record ? t("editProject") : t("create")}
        description={t("dialog.projectHint")}
        wide
      >
        <ProjectForm
          key={dialog?.record?.id ?? "new"}
          record={dialog?.record}
          onCancel={() => setDialog(null)}
          onSaved={(message) => {
            setDialog(null);
            setNotice(message);
          }}
        />
      </EditorDialog>
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}
