import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { EditorDialog } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  projectMembers,
  projectPlans,
  projectTaskAssign,
  projectTaskCreate,
  projectTaskGet,
  projectTaskStatusChange,
  projectTaskUpdate,
  projectTasks,
  type ProjectMember,
  type ProjectPlan,
  type ProjectTask,
} from "@/lib/capabilities";
import { EnumSelect, targets } from "./fields";

const schema = z.object({
  title: z.string().trim().min(1).max(200),
  description: z.string().max(20000),
  priority: z.enum(["low", "normal", "high", "urgent"]),
  planId: z.string(),
  assigneeId: z.string(),
  dueDate: z
    .string()
    .refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value)),
});
type Values = z.infer<typeof schema>;

// taskTransitions mirrors the declared state machine; the server re-checks it.
const taskTransitions: Record<string, string[]> = {
  todo: ["in_progress", "cancelled"],
  in_progress: ["done", "cancelled"],
  done: ["todo"],
  cancelled: [],
};

type DialogState =
  | { kind: "create" }
  | { kind: "edit"; taskId: number }
  | { kind: "detail"; taskId: number }
  | null;

/** TasksPanel lists tasks; detail and editors are dialogs. */
export function TasksPanel({
  projectId,
  canWrite,
  archived,
}: {
  projectId: number;
  canWrite: boolean;
  archived: boolean;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const [dialog, setDialog] = useState<DialogState>(null);

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
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-tasks", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
    },
  });

  const editable = canWrite && !archived;
  const rows = tasks.data?.data.items ?? [];

  return (
    <section className="space-y-3">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold">{t("tasks.heading")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("tasks.description")}
          </p>
        </div>
        {editable && (
          <Button onClick={() => setDialog({ kind: "create" })}>
            {t("tasks.new")}
          </Button>
        )}
      </div>
      {(tasks.isError || mutation.isError) && <p role="alert">{t("error")}</p>}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("tasks.title")}</TableHead>
            <TableHead>{t("tasks.plan")}</TableHead>
            <TableHead>{t("tasks.assignee")}</TableHead>
            <TableHead>{t("dueDate")}</TableHead>
            <TableHead>{t("status")}</TableHead>
            <TableHead>{t("actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((task) => (
            <TableRow key={task.id}>
              <TableCell>{task.title}</TableCell>
              <TableCell>{task.planTitle || t("tasks.noPlan")}</TableCell>
              <TableCell>
                {task.assigneeName || t("tasks.unassigned")}
              </TableCell>
              <TableCell>{task.dueDate || t("dash")}</TableCell>
              <TableCell>{t(`tasks.states.${task.status}`)}</TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-2">
                  <Button
                    variant="outline"
                    onClick={() =>
                      setDialog({ kind: "detail", taskId: task.id })
                    }
                  >
                    {t("tasks.detail")}
                  </Button>
                  {editable && (
                    <Button
                      variant="outline"
                      onClick={() =>
                        setDialog({ kind: "edit", taskId: task.id })
                      }
                    >
                      {t("edit")}
                    </Button>
                  )}
                  {editable &&
                    targets(taskTransitions, task.status).map((next) => (
                      <Button
                        key={next}
                        variant="outline"
                        disabled={mutation.isPending}
                        onClick={() =>
                          mutation.mutate(async () => {
                            await projectTaskStatusChange(
                              task.id,
                              task.status,
                              next,
                            );
                          })
                        }
                      >
                        {t(`tasks.actions.${next}`)}
                      </Button>
                    ))}
                </div>
              </TableCell>
            </TableRow>
          ))}
          {rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={6}>{t("tasks.empty")}</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {editable && (
        <TaskDialog
          state={dialog}
          projectId={projectId}
          members={members.data?.data.items ?? []}
          plans={plans.data?.data.items ?? []}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === "detail" && (
        <TaskDetailDialog
          taskId={dialog.taskId}
          canAssign={editable}
          members={members.data?.data.items ?? []}
          onClose={() => setDialog(null)}
        />
      )}
    </section>
  );
}

function TaskDialog({
  state,
  projectId,
  members,
  plans,
  onClose,
}: {
  state: DialogState;
  projectId: number;
  members: ProjectMember[];
  plans: ProjectPlan[];
  onClose: () => void;
}) {
  const { t } = useTranslation("projects");
  const open = state !== null && state.kind !== "detail";
  const taskId = state?.kind === "edit" ? state.taskId : null;

  const detail = useQuery({
    queryKey: ["project-task", taskId],
    queryFn: () => projectTaskGet(taskId as number),
    enabled: open && taskId !== null,
    retry: false,
  });

  return (
    <EditorDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
      title={taskId === null ? t("tasks.new") : t("edit")}
      description={t("dialog.taskHint")}
      wide
    >
      {taskId !== null && detail.isPending ? (
        <p>{t("loading")}</p>
      ) : (
        <TaskForm
          projectId={projectId}
          task={detail.data?.data}
          members={members}
          plans={plans}
          onDone={onClose}
        />
      )}
    </EditorDialog>
  );
}

function TaskForm({
  projectId,
  task,
  members,
  plans,
  onDone,
}: {
  projectId: number;
  task?: ProjectTask;
  members: ProjectMember[];
  plans: ProjectPlan[];
  onDone: () => void;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: task?.title ?? "",
      description: task?.description ?? "",
      priority: (task?.priority as Values["priority"]) ?? "normal",
      planId: task ? String(task.planId) : "0",
      assigneeId: task ? String(task.assigneeId) : "0",
      dueDate: task?.dueDate ?? "",
    },
  });

  const mutation = useMutation({
    mutationFn: async (values: Values) => {
      const payload = {
        planId: Number(values.planId),
        title: values.title,
        description: values.description,
        priority: values.priority,
        assigneeId: Number(values.assigneeId),
        dueDate: values.dueDate,
      };
      if (task) await projectTaskUpdate({ id: task.id, ...payload });
      else await projectTaskCreate({ projectId, ...payload });
    },
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-tasks", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      onDone();
    },
  });

  return (
    <form
      className="space-y-3"
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
    >
      <label htmlFor="task-title">{t("tasks.title")}</label>
      <Input id="task-title" {...form.register("title")} />

      <label htmlFor="task-plan">{t("tasks.plan")}</label>
      <EnumSelect
        id="task-plan"
        value={form.watch("planId")}
        onChange={(value) => form.setValue("planId", value)}
        options={[
          { value: "0", label: t("tasks.noPlan") },
          ...plans.map((plan) => ({
            value: String(plan.id),
            label: plan.title,
          })),
        ]}
      />

      <label htmlFor="task-assignee">{t("tasks.assignee")}</label>
      <EnumSelect
        id="task-assignee"
        value={form.watch("assigneeId")}
        onChange={(value) => form.setValue("assigneeId", value)}
        options={[
          { value: "0", label: t("tasks.unassigned") },
          ...members.map((member) => ({
            value: String(member.accountId),
            label: member.name,
          })),
        ]}
      />

      <label htmlFor="task-priority">{t("priority")}</label>
      <EnumSelect
        id="task-priority"
        value={form.watch("priority")}
        onChange={(value) =>
          form.setValue("priority", value as Values["priority"])
        }
        options={(["low", "normal", "high", "urgent"] as const).map(
          (value) => ({
            value,
            label: t(`priorities.${value}`),
          }),
        )}
      />

      <label htmlFor="task-due">{t("dueDate")}</label>
      <Input id="task-due" type="date" {...form.register("dueDate")} />

      <label htmlFor="task-description">{t("tasks.descriptionField")}</label>
      <textarea
        id="task-description"
        className="min-h-40 w-full rounded-md border p-3"
        {...form.register("description")}
      />

      {Object.keys(form.formState.errors).length > 0 && (
        <p role="alert">{t("invalid")}</p>
      )}
      {mutation.isError && <p role="alert">{t("error")}</p>}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onDone}>
          {t("cancel")}
        </Button>
        <Button type="submit" disabled={mutation.isPending}>
          {t("save")}
        </Button>
      </div>
    </form>
  );
}

/** Read-only task detail, including the AI brief and the assignment action. */
function TaskDetailDialog({
  taskId,
  canAssign,
  members,
  onClose,
}: {
  taskId: number;
  canAssign: boolean;
  members: ProjectMember[];
  onClose: () => void;
}) {
  const { t } = useTranslation("projects");
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
    },
  });

  const task = detail.data?.data;

  return (
    <EditorDialog
      open
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
      title={task?.title ?? t("tasks.detail")}
      description={t("dialog.taskDetailHint")}
      wide
    >
      {detail.isPending && <p>{t("loading")}</p>}
      {task && (
        <div className="space-y-3">
          <p className="whitespace-pre-wrap">{task.description || t("dash")}</p>
          <dl className="grid gap-2 sm:grid-cols-2">
            <div>
              <dt className="text-sm text-muted-foreground">{t("status")}</dt>
              <dd>{t(`tasks.states.${task.status}`)}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">
                {t("tasks.assignee")}
              </dt>
              <dd>{task.assigneeName || t("tasks.unassigned")}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">
                {t("tasks.plan")}
              </dt>
              <dd>{task.planTitle || t("tasks.noPlan")}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">{t("dueDate")}</dt>
              <dd>{task.dueDate || t("dash")}</dd>
            </div>
            {task.completedAt && (
              <div>
                <dt className="text-sm text-muted-foreground">
                  {t("tasks.completedAt")}
                </dt>
                <dd>{task.completedAt}</dd>
              </div>
            )}
          </dl>

          <div className="space-y-1 rounded-md border p-3">
            <p className="font-medium">{t("tasks.brief")}</p>
            <p className="whitespace-pre-wrap text-sm">
              {task.brief || t("tasks.briefEmpty")}
            </p>
            {task.brief && (
              <p className="text-xs text-muted-foreground">
                {t("tasks.briefHint")}
                {task.briefModel ? ` (${task.briefModel})` : ""}
              </p>
            )}
          </div>

          {canAssign && (
            <div className="flex items-end gap-2">
              <div className="flex-1">
                <label htmlFor="task-assign-select">
                  {t("tasks.assignee")}
                </label>
                <EnumSelect
                  id="task-assign-select"
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
              <Button
                disabled={mutation.isPending || assignee === "0"}
                onClick={() => mutation.mutate(Number(assignee))}
              >
                {t("tasks.assign")}
              </Button>
            </div>
          )}
          {mutation.isError && <p role="alert">{t("tasks.assignConflict")}</p>}
          <div className="flex justify-end">
            <Button variant="outline" onClick={onClose}>
              {t("close")}
            </Button>
          </div>
        </div>
      )}
    </EditorDialog>
  );
}
