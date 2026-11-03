import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
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

const defaults: Values = {
  title: "",
  description: "",
  priority: "normal",
  planId: "0",
  assigneeId: "0",
  dueDate: "",
};

// taskTransitions mirrors the declared state machine; the server re-checks it.
const taskTransitions: Record<string, string[]> = {
  todo: ["in_progress", "cancelled"],
  in_progress: ["done", "cancelled"],
  done: ["todo"],
  cancelled: [],
};

export function TasksPanel({
  projectId,
  members,
  plans,
  canWrite,
  archived,
}: {
  projectId: number;
  members: ProjectMember[];
  plans: ProjectPlan[];
  canWrite: boolean;
  archived: boolean;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const [editingId, setEditingId] = useState<number | null>(null);
  const [detail, setDetail] = useState<ProjectTask | null>(null);
  const [assignee, setAssignee] = useState("0");
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
  });

  const tasks = useQuery({
    queryKey: ["project-tasks", projectId],
    queryFn: () => projectTasks(projectId),
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

  const memberOptions = [
    { value: "0", label: t("tasks.unassigned") },
    ...members.map((member) => ({
      value: String(member.accountId),
      label: member.name,
    })),
  ];
  const planOptions = [
    { value: "0", label: t("tasks.noPlan") },
    ...plans.map((plan) => ({ value: String(plan.id), label: plan.title })),
  ];
  const editable = canWrite && !archived;
  const rows = tasks.data?.data.items ?? [];

  const reset = () => {
    setEditingId(null);
    form.reset(defaults);
  };

  const open = (task: ProjectTask) =>
    mutation.mutate(async () => {
      const response = await projectTaskGet(task.id);
      const value = response.data;
      setDetail(value);
      setAssignee(String(value.assigneeId));
      setEditingId(value.id);
      form.reset({
        title: value.title,
        description: value.description ?? "",
        priority: value.priority as Values["priority"],
        planId: String(value.planId),
        assigneeId: String(value.assigneeId),
        dueDate: value.dueDate,
      });
    });

  return (
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">{t("tasks.heading")}</h2>
      <p className="text-sm text-muted-foreground">{t("tasks.description")}</p>
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
                    disabled={mutation.isPending}
                    onClick={() => open(task)}
                  >
                    {t("tasks.detail")}
                  </Button>
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

      {detail && (
        <div className="space-y-2 rounded-md border p-4">
          <p className="font-medium">{detail.title}</p>
          <p className="whitespace-pre-wrap text-sm">
            {detail.description || t("dash")}
          </p>
          {detail.completedAt && (
            <p className="text-sm text-muted-foreground">
              {t("tasks.completedAtValue", { value: detail.completedAt })}
            </p>
          )}
          <div className="space-y-1">
            <p className="font-medium">{t("tasks.brief")}</p>
            <p className="whitespace-pre-wrap text-sm">
              {detail.brief || t("tasks.briefEmpty")}
            </p>
            {detail.brief && (
              <p className="text-xs text-muted-foreground">
                {t("tasks.briefHint")}
                {detail.briefModel ? ` (${detail.briefModel})` : ""}
              </p>
            )}
          </div>
          {editable && (
            <div className="flex items-end gap-2">
              <div className="flex-1">
                <label htmlFor="task-assignee">{t("tasks.assignee")}</label>
                <EnumSelect
                  id="task-assignee"
                  value={assignee}
                  onChange={setAssignee}
                  options={memberOptions}
                />
              </div>
              <Button
                disabled={mutation.isPending || assignee === "0"}
                onClick={() =>
                  mutation.mutate(async () => {
                    await projectTaskAssign(detail.id, Number(assignee));
                    const response = await projectTaskGet(detail.id);
                    setDetail(response.data);
                  })
                }
              >
                {t("tasks.assign")}
              </Button>
            </div>
          )}
        </div>
      )}

      {editable && (
        <form
          className="space-y-3 rounded-md border p-4"
          onSubmit={form.handleSubmit((values) =>
            mutation.mutate(async () => {
              const payload = {
                planId: Number(values.planId),
                title: values.title,
                description: values.description,
                priority: values.priority,
                assigneeId: Number(values.assigneeId),
                dueDate: values.dueDate,
              };
              if (editingId) {
                await projectTaskUpdate({ id: editingId, ...payload });
              } else {
                await projectTaskCreate({ projectId, ...payload });
              }
              reset();
              setDetail(null);
            }),
          )}
        >
          <p className="font-medium">
            {editingId ? t("edit") : t("tasks.new")}
          </p>
          <label htmlFor="task-title">{t("tasks.title")}</label>
          <Input id="task-title" {...form.register("title")} />

          <label htmlFor="task-plan">{t("tasks.plan")}</label>
          <EnumSelect
            id="task-plan"
            value={form.watch("planId")}
            onChange={(value) => form.setValue("planId", value)}
            options={planOptions}
          />

          <label htmlFor="task-assignee-new">{t("tasks.assignee")}</label>
          <EnumSelect
            id="task-assignee-new"
            value={form.watch("assigneeId")}
            onChange={(value) => form.setValue("assigneeId", value)}
            options={memberOptions}
          />

          <label htmlFor="task-priority">{t("priority")}</label>
          <EnumSelect
            id="task-priority"
            value={form.watch("priority")}
            onChange={(value) =>
              form.setValue("priority", value as Values["priority"])
            }
            options={(["low", "normal", "high", "urgent"] as const).map(
              (value) => ({ value, label: t(`priorities.${value}`) }),
            )}
          />

          <label htmlFor="task-due">{t("dueDate")}</label>
          <Input id="task-due" type="date" {...form.register("dueDate")} />

          <label htmlFor="task-description">
            {t("tasks.descriptionField")}
          </label>
          <textarea
            id="task-description"
            className="w-full min-h-40 rounded-md border p-3"
            {...form.register("description")}
          />

          {Object.keys(form.formState.errors).length > 0 && (
            <p role="alert">{t("invalid")}</p>
          )}
          <div className="flex gap-2">
            <Button type="submit" disabled={mutation.isPending}>
              {t("save")}
            </Button>
            <Button type="button" variant="outline" onClick={reset}>
              {t("cancel")}
            </Button>
          </div>
        </form>
      )}
    </section>
  );
}
