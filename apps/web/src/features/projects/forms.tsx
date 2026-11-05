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
  projectPlanCreate,
  projectPlanUpdate,
  type ProjectMember,
  type ProjectPlan,
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
