import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Eye, Pencil, Play, Plus } from "lucide-react";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  ListSearch,
  Loading,
  RowActions,
  ShortDate,
  StateBadge,
  stateTone,
  SuccessNotice,
  type AdminColumn,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import {
  projectMembers,
  projectPlans,
  projectTaskAssign,
  projectTaskGet,
  projectTaskStatusChange,
  projectTasks,
  type ProjectMember,
  type ProjectTask,
} from "@/lib/capabilities";
import { EnumSelect } from "./fields";
import { TaskForm } from "./forms";

// taskTransitions mirrors the declared state machine; the server re-checks it.
const taskTransitions: Record<string, string[]> = {
  todo: ["in_progress", "cancelled"],
  in_progress: ["done", "cancelled"],
  done: ["todo"],
  cancelled: [],
};

type DialogState =
  | { kind: "create" }
  | { kind: "edit"; task: ProjectTask }
  | { kind: "detail"; taskId: number }
  | null;

/** TasksPanel is the task tab: a filtered table plus dialogs for detail and edit. */
export function TasksPanel({
  projectId,
  canWrite,
  archived,
}: {
  projectId: number;
  canWrite: boolean;
  archived: boolean;
}) {
  const { t } = useTranslation(["projects", "admin"]);
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const [dialog, setDialog] = useState<DialogState>(null);
  const [notice, setNotice] = useState("");

  const tasks = useQuery({
    queryKey: ["project-tasks", projectId],
    queryFn: () => projectTasks(projectId),
    retry: false,
  });
  const members = useQuery({
    queryKey: ["project-members", projectId],
    queryFn: () => projectMembers(projectId),
    retry: false,
  });
  const plans = useQuery({
    queryKey: ["project-plans", projectId],
    queryFn: () => projectPlans(projectId),
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: ({ task, next }: { task: ProjectTask; next: string }) =>
      projectTaskStatusChange(task.id, task.status, next),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-tasks", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      setNotice(t("admin:feedback.saved"));
    },
  });

  const items = tasks.data?.data.items ?? [];
  const rows = useMemo(
    () =>
      items.filter((task) =>
        `${task.title} ${task.assigneeName} ${task.planTitle}`
          .toLowerCase()
          .includes(search.toLowerCase()),
      ),
    [items, search],
  );

  const editable = canWrite && !archived;

  const columns: AdminColumn<ProjectTask>[] = [
    {
      id: "title",
      header: t("tasks.title"),
      cell: (task) => (
        <button
          className="account-cell"
          onClick={() => setDialog({ kind: "detail", taskId: task.id })}
        >
          <span>
            <strong>{task.title}</strong>
            <small>{task.planTitle || t("tasks.noPlan")}</small>
          </span>
        </button>
      ),
    },
    {
      id: "assignee",
      header: t("tasks.assignee"),
      cell: (task) => task.assigneeName || t("tasks.unassigned"),
    },
    {
      id: "due",
      header: t("dueDate"),
      cell: (task) =>
        task.dueDate ? <ShortDate value={task.dueDate} /> : t("dash"),
    },
    {
      id: "status",
      header: t("status"),
      cell: (task) => (
        <StateBadge
          label={t(`tasks.states.${task.status}`)}
          tone={stateTone(task.status)}
        />
      ),
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (task) => (
        <RowActions
          actions={[
            {
              label: t("tasks.detail"),
              icon: <Eye size={14} />,
              onClick: () => setDialog({ kind: "detail", taskId: task.id }),
            },
            ...(editable
              ? [
                  {
                    label: t("admin:actions.edit"),
                    icon: <Pencil size={14} />,
                    onClick: () => setDialog({ kind: "edit", task }),
                  },
                ]
              : []),
            ...(editable
              ? taskTransitions[task.status].map((next) => ({
                  label: t(`tasks.actions.${next}`),
                  icon: <Play size={14} />,
                  onClick: () => mutation.mutate({ task, next }),
                }))
              : []),
          ]}
        />
      ),
    },
  ];

  if (tasks.isPending) return <Loading />;

  return (
    <>
      {tasks.isError && (
        <ErrorBanner
          error={tasks.error}
          onRetry={() => {
            void tasks.refetch();
          }}
        />
      )}
      <section className="admin-panel">
        <div className="table-toolbar">
          <ListSearch value={search} onChange={setSearch} />
          {editable && (
            <div className="toolbar-right">
              <Button onClick={() => setDialog({ kind: "create" })}>
                <Plus size={15} />
                {t("tasks.new")}
              </Button>
            </div>
          )}
        </div>
        <DataTable rows={rows} columns={columns} />
      </section>

      <EditorDialog
        open={dialog?.kind === "create" || dialog?.kind === "edit"}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        title={dialog?.kind === "edit" ? t("edit") : t("tasks.new")}
        description={t("dialog.taskHint")}
        wide
      >
        <TaskForm
          key={dialog?.kind === "edit" ? dialog.task.id : "new"}
          projectId={projectId}
          record={dialog?.kind === "edit" ? dialog.task : undefined}
          members={members.data?.data.items ?? []}
          plans={plans.data?.data.items ?? []}
          onCancel={() => setDialog(null)}
          onSaved={(message) => {
            setDialog(null);
            setNotice(message);
          }}
        />
      </EditorDialog>

      {dialog?.kind === "detail" && (
        <TaskDetailDialog
          taskId={dialog.taskId}
          canAssign={editable}
          members={members.data?.data.items ?? []}
          onClose={() => setDialog(null)}
          onAssigned={() => setNotice(t("admin:feedback.saved"))}
        />
      )}

      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}

/** TaskDetailDialog is read-only except for the assignment command. */
function TaskDetailDialog({
  taskId,
  canAssign,
  members,
  onClose,
  onAssigned,
}: {
  taskId: number;
  canAssign: boolean;
  members: ProjectMember[];
  onClose: () => void;
  onAssigned: () => void;
}) {
  const { t } = useTranslation(["projects", "admin"]);
  const client = useQueryClient();
  const [assignee, setAssignee] = useState("0");

  const detail = useQuery({
    queryKey: ["project-task", taskId],
    queryFn: () => projectTaskGet(taskId),
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: (accountId: number) => projectTaskAssign(taskId, accountId),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["project-task", taskId] });
      await client.invalidateQueries({ queryKey: ["project-tasks"] });
      await client.invalidateQueries({ queryKey: ["project"] });
      onAssigned();
    },
  });

  const task = detail.data?.data;

  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={task?.title ?? t("tasks.detail")}
      description={t("dialog.taskDetailHint")}
      wide
    >
      {detail.isPending && <Loading />}
      {task && (
        <div className="entity-form">
          <div className="detail-body">
            <dl className="detail-grid">
              <div>
                <dt>{t("status")}</dt>
                <dd>
                  <StateBadge
                    label={t(`tasks.states.${task.status}`)}
                    tone={stateTone(task.status)}
                  />
                </dd>
              </div>
              <div>
                <dt>{t("tasks.assignee")}</dt>
                <dd>{task.assigneeName || t("tasks.unassigned")}</dd>
              </div>
              <div>
                <dt>{t("tasks.plan")}</dt>
                <dd>{task.planTitle || t("tasks.noPlan")}</dd>
              </div>
              <div>
                <dt>{t("dueDate")}</dt>
                <dd>
                  {task.dueDate ? (
                    <ShortDate value={task.dueDate} />
                  ) : (
                    t("dash")
                  )}
                </dd>
              </div>
              {task.completedAt && (
                <div>
                  <dt>{t("tasks.completedAt")}</dt>
                  <dd>
                    <ShortDate value={task.completedAt} />
                  </dd>
                </div>
              )}
            </dl>
          </div>

          <div className="brief-block">
            <strong>{t("tasks.brief")}</strong>
            <p>{task.brief || t("tasks.briefEmpty")}</p>
            {task.brief && (
              <small>
                {t("tasks.briefHint")}
                {task.briefModel ? ` (${task.briefModel})` : ""}
              </small>
            )}
          </div>

          {task.description && (
            <p className="brief-block">{task.description}</p>
          )}

          {canAssign && (
            <div className="editor-grid">
              <div className="editor-field">
                <label htmlFor="task-assign">{t("tasks.assignee")}</label>
                <EnumSelect
                  id="task-assign"
                  value={assignee}
                  onChange={setAssignee}
                  options={[
                    { value: "0", label: t("tasks.unassigned") },
                    ...members.map((member) => ({
                      value: String(member.accountId),
                      label: member.name,
                    })),
                  ]}
                />
              </div>
            </div>
          )}
          {mutation.isError && <p role="alert">{t("tasks.assignConflict")}</p>}
          <div className="editor-footer">
            <Button variant="outline" onClick={onClose}>
              {t("close")}
            </Button>
            {canAssign && (
              <Button
                disabled={mutation.isPending || assignee === "0"}
                onClick={() => mutation.mutate(Number(assignee))}
              >
                {t("tasks.assign")}
              </Button>
            )}
          </div>
        </div>
      )}
    </EditorDialog>
  );
}
