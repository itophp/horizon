import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FormField } from "@/features/administration/forms";
import { ErrorBanner } from "@/features/administration/shared";
import { EnumSelect } from "./fields";
import {
  projectCreate,
  projectUpdate,
  projectPlanCreate,
  projectPlanUpdate,
  projectTaskCreate,
  projectTaskUpdate,
  type Project,
  type ProjectMember,
  type ProjectPlan,
  type ProjectTask,
} from "@/lib/capabilities";

const priorities = ["low", "normal", "high", "urgent"] as const;
const planTypes = ["milestone", "phase", "deliverable"] as const;

/** FormActions is the shared footer: cancel on the left of save, spinner while busy. */
function FormActions({
  busy,
  onCancel,
}: {
  busy: boolean;
  onCancel: () => void;
}) {
  const { t } = useTranslation("admin");
  return (
    <div className="editor-footer">
      <Button type="button" variant="outline" onClick={onCancel}>
        {t("actions.cancel")}
      </Button>
      <Button type="submit" disabled={busy}>
        {busy && <LoaderCircle size={14} className="animate-spin" />}
        {busy ? t("actions.saving") : t("actions.save")}
      </Button>
    </div>
  );
}

const projectSchema = z.object({
  code: z.string().trim().min(1, { message: "required" }).max(32),
  name: z.string().trim().min(1, { message: "required" }).max(120),
  priority: z.enum(priorities),
  dueDate: z.string(),
});
type ProjectValues = z.infer<typeof projectSchema>;

export function ProjectForm({
  record,
  onCancel,
  onSaved,
}: {
  record?: Project;
  onCancel: () => void;
  onSaved: (message: string) => void;
}) {
  const { t } = useTranslation(["projects", "admin"]);
  const client = useQueryClient();
  const form = useForm<ProjectValues>({
    resolver: zodResolver(projectSchema),
    defaultValues: {
      code: record?.code ?? "",
      name: record?.name ?? "",
      priority: (record?.priority as ProjectValues["priority"]) ?? "normal",
      dueDate: record?.dueDate ?? "",
    },
  });
  const mutation = useMutation({
    mutationFn: (values: ProjectValues) =>
      record
        ? projectUpdate({
            id: record.id,
            name: values.name,
            priority: values.priority,
            dueDate: values.dueDate,
          })
        : projectCreate(values),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["projects"] });
      if (record) {
        await client.invalidateQueries({ queryKey: ["project", record.id] });
      }
      onSaved(t("admin:feedback.saved"));
    },
  });

  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
    >
      {mutation.isError && <ErrorBanner error={mutation.error} />}
      <div className="editor-grid">
        <FormField
          label={t("code")}
          error={form.formState.errors.code?.message}
        >
          <Input disabled={!!record} {...form.register("code")} />
        </FormField>
        <FormField
          label={t("name")}
          error={form.formState.errors.name?.message}
        >
          <Input {...form.register("name")} />
        </FormField>
        <FormField label={t("priority")}>
          <EnumSelect
            value={form.watch("priority")}
            onChange={(value) =>
              form.setValue("priority", value as ProjectValues["priority"])
            }
            options={priorities.map((value) => ({
              value,
              label: t(`priorities.${value}`),
            }))}
          />
        </FormField>
        <FormField label={t("dueDate")}>
          <Input type="date" {...form.register("dueDate")} />
        </FormField>
      </div>
      <FormActions busy={mutation.isPending} onCancel={onCancel} />
    </form>
  );
}

const planSchema = z.object({
  planType: z.enum(planTypes),
  title: z.string().trim().min(1, { message: "required" }).max(200),
  content: z.string().max(20000),
  priority: z.enum(priorities),
  dueDate: z.string(),
  ownerId: z.string().min(1, { message: "required" }),
});
type PlanValues = z.infer<typeof planSchema>;

export function PlanForm({
  projectId,
  record,
  members,
  onCancel,
  onSaved,
}: {
  projectId: number;
  record?: ProjectPlan;
  members: ProjectMember[];
  onCancel: () => void;
  onSaved: (message: string) => void;
}) {
  const { t } = useTranslation(["projects", "admin"]);
  const client = useQueryClient();
  const form = useForm<PlanValues>({
    resolver: zodResolver(planSchema),
    defaultValues: {
      planType: (record?.planType as PlanValues["planType"]) ?? "phase",
      title: record?.title ?? "",
      content: record?.content ?? "",
      priority: (record?.priority as PlanValues["priority"]) ?? "normal",
      dueDate: record?.dueDate ?? "",
      ownerId: record
        ? String(record.ownerId)
        : members[0]
          ? String(members[0].accountId)
          : "",
    },
  });
  const mutation = useMutation({
    mutationFn: async (values: PlanValues) => {
      const payload = {
        planType: values.planType,
        title: values.title,
        content: values.content,
        priority: values.priority,
        dueDate: values.dueDate,
        ownerId: Number(values.ownerId),
      };
      if (record) await projectPlanUpdate({ id: record.id, ...payload });
      else await projectPlanCreate({ projectId, ...payload });
    },
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-plans", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      onSaved(t("admin:feedback.saved"));
    },
  });

  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
    >
      {mutation.isError && <ErrorBanner error={mutation.error} />}
      <div className="editor-grid">
        <FormField
          label={t("plans.title")}
          error={form.formState.errors.title?.message}
          wide
        >
          <Input {...form.register("title")} />
        </FormField>
        <FormField label={t("plans.type")}>
          <EnumSelect
            value={form.watch("planType")}
            onChange={(value) =>
              form.setValue("planType", value as PlanValues["planType"])
            }
            options={planTypes.map((value) => ({
              value,
              label: t(`plans.types.${value}`),
            }))}
          />
        </FormField>
        <FormField
          label={t("owner")}
          error={form.formState.errors.ownerId?.message}
        >
          <EnumSelect
            value={form.watch("ownerId")}
            onChange={(value) => form.setValue("ownerId", value)}
            options={members.map((member) => ({
              value: String(member.accountId),
              label: member.name,
            }))}
          />
        </FormField>
        <FormField label={t("priority")}>
          <EnumSelect
            value={form.watch("priority")}
            onChange={(value) =>
              form.setValue("priority", value as PlanValues["priority"])
            }
            options={priorities.map((value) => ({
              value,
              label: t(`priorities.${value}`),
            }))}
          />
        </FormField>
        <FormField label={t("dueDate")}>
          <Input type="date" {...form.register("dueDate")} />
        </FormField>
        <FormField label={t("plans.content")} wide>
          <textarea className="editor-textarea" {...form.register("content")} />
        </FormField>
      </div>
      <FormActions busy={mutation.isPending} onCancel={onCancel} />
    </form>
  );
}

const taskSchema = z.object({
  title: z.string().trim().min(1, { message: "required" }).max(200),
  description: z.string().max(20000),
  priority: z.enum(priorities),
  planId: z.string(),
  assigneeId: z.string(),
  dueDate: z.string(),
});
type TaskValues = z.infer<typeof taskSchema>;

export function TaskForm({
  projectId,
  record,
  members,
  plans,
  onCancel,
  onSaved,
}: {
  projectId: number;
  record?: ProjectTask;
  members: ProjectMember[];
  plans: ProjectPlan[];
  onCancel: () => void;
  onSaved: (message: string) => void;
}) {
  const { t } = useTranslation(["projects", "admin"]);
  const client = useQueryClient();
  const form = useForm<TaskValues>({
    resolver: zodResolver(taskSchema),
    defaultValues: {
      title: record?.title ?? "",
      description: record?.description ?? "",
      priority: (record?.priority as TaskValues["priority"]) ?? "normal",
      planId: record ? String(record.planId) : "0",
      assigneeId: record ? String(record.assigneeId) : "0",
      dueDate: record?.dueDate ?? "",
    },
  });
  const mutation = useMutation({
    mutationFn: async (values: TaskValues) => {
      const payload = {
        planId: Number(values.planId),
        title: values.title,
        description: values.description,
        priority: values.priority,
        assigneeId: Number(values.assigneeId),
        dueDate: values.dueDate,
      };
      if (record) await projectTaskUpdate({ id: record.id, ...payload });
      else await projectTaskCreate({ projectId, ...payload });
    },
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-tasks", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      onSaved(t("admin:feedback.saved"));
    },
  });

  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
    >
      {mutation.isError && <ErrorBanner error={mutation.error} />}
      <div className="editor-grid">
        <FormField
          label={t("tasks.title")}
          error={form.formState.errors.title?.message}
          wide
        >
          <Input {...form.register("title")} />
        </FormField>
        <FormField label={t("tasks.plan")}>
          <EnumSelect
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
        </FormField>
        <FormField label={t("tasks.assignee")}>
          <EnumSelect
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
        </FormField>
        <FormField label={t("priority")}>
          <EnumSelect
            value={form.watch("priority")}
            onChange={(value) =>
              form.setValue("priority", value as TaskValues["priority"])
            }
            options={priorities.map((value) => ({
              value,
              label: t(`priorities.${value}`),
            }))}
          />
        </FormField>
        <FormField label={t("dueDate")}>
          <Input type="date" {...form.register("dueDate")} />
        </FormField>
        <FormField label={t("tasks.descriptionField")} wide>
          <textarea
            className="editor-textarea"
            {...form.register("description")}
          />
        </FormField>
      </div>
      <FormActions busy={mutation.isPending} onCancel={onCancel} />
    </form>
  );
}
