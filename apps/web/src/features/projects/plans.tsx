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
  projectPlanCreate,
  projectPlanGet,
  projectPlanStatusChange,
  projectPlanUpdate,
  projectPlans,
  type ProjectMember,
} from "@/lib/capabilities";
import { EnumSelect, targets } from "./fields";

const schema = z.object({
  planType: z.enum(["milestone", "phase", "deliverable"]),
  title: z.string().trim().min(1).max(200),
  content: z.string().max(20000),
  priority: z.enum(["low", "normal", "high", "urgent"]),
  dueDate: z
    .string()
    .refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value)),
  ownerId: z.string().min(1),
});
type Values = z.infer<typeof schema>;

const defaults: Values = {
  planType: "phase",
  title: "",
  content: "",
  priority: "normal",
  dueDate: "",
  ownerId: "",
};

// planTransitions mirrors the declared state machine; the server still rejects
// an illegal edge, so this table only decides which buttons are offered.
const planTransitions: Record<string, string[]> = {
  pending: ["in_progress", "cancelled"],
  in_progress: ["done", "cancelled"],
  done: ["in_progress"],
  cancelled: [],
};

export function PlansPanel({
  projectId,
  members,
  canWrite,
  archived,
}: {
  projectId: number;
  members: ProjectMember[];
  canWrite: boolean;
  archived: boolean;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const [editingId, setEditingId] = useState<number | null>(null);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
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
        queryKey: ["project-plans", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
    },
  });

  const memberOptions = members.map((member) => ({
    value: String(member.accountId),
    label: member.name,
  }));
  const editable = canWrite && !archived;
  const rows = plans.data?.data.items ?? [];

  const open = (id: number) =>
    mutation.mutate(async () => {
      const response = await projectPlanGet(id);
      const plan = response.data;
      setEditingId(id);
      form.reset({
        planType: plan.planType as Values["planType"],
        title: plan.title,
        content: plan.content ?? "",
        priority: plan.priority as Values["priority"],
        dueDate: plan.dueDate,
        ownerId: String(plan.ownerId),
      });
    });

  const reset = () => {
    setEditingId(null);
    form.reset({
      ...defaults,
      ownerId: members.length > 0 ? String(members[0].accountId) : "",
    });
  };

  return (
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">{t("plans.heading")}</h2>
      <p className="text-sm text-muted-foreground">{t("plans.description")}</p>
      {(plans.isError || mutation.isError) && <p role="alert">{t("error")}</p>}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("plans.title")}</TableHead>
            <TableHead>{t("plans.type")}</TableHead>
            <TableHead>{t("owner")}</TableHead>
            <TableHead>{t("dueDate")}</TableHead>
            <TableHead>{t("status")}</TableHead>
            <TableHead>{t("actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((plan) => (
            <TableRow key={plan.id}>
              <TableCell>{plan.title}</TableCell>
              <TableCell>{t(`plans.types.${plan.planType}`)}</TableCell>
              <TableCell>{plan.ownerName}</TableCell>
              <TableCell>{plan.dueDate || t("dash")}</TableCell>
              <TableCell>{t(`plans.states.${plan.status}`)}</TableCell>
              <TableCell>
                <div className="flex flex-wrap gap-2">
                  {editable && (
                    <Button
                      variant="outline"
                      disabled={mutation.isPending}
                      onClick={() => open(plan.id)}
                    >
                      {t("edit")}
                    </Button>
                  )}
                  {editable &&
                    targets(planTransitions, plan.status).map((next) => (
                      <Button
                        key={next}
                        variant="outline"
                        disabled={mutation.isPending}
                        onClick={() =>
                          mutation.mutate(async () => {
                            await projectPlanStatusChange(
                              plan.id,
                              plan.status,
                              next,
                            );
                          })
                        }
                      >
                        {t(`plans.actions.${next}`)}
                      </Button>
                    ))}
                </div>
              </TableCell>
            </TableRow>
          ))}
          {rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={6}>{t("plans.empty")}</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {editable && (
        <form
          className="space-y-3 rounded-md border p-4"
          onSubmit={form.handleSubmit((values) =>
            mutation.mutate(async () => {
              const payload = {
                planType: values.planType,
                title: values.title,
                content: values.content,
                priority: values.priority,
                dueDate: values.dueDate,
                ownerId: Number(values.ownerId),
              };
              if (editingId) {
                await projectPlanUpdate({ id: editingId, ...payload });
              } else {
                await projectPlanCreate({ projectId, ...payload });
              }
              reset();
            }),
          )}
        >
          <p className="font-medium">
            {editingId ? t("edit") : t("plans.new")}
          </p>
          <label htmlFor="plan-title">{t("plans.title")}</label>
          <Input id="plan-title" {...form.register("title")} />

          <label htmlFor="plan-type">{t("plans.type")}</label>
          <EnumSelect
            id="plan-type"
            value={form.watch("planType")}
            onChange={(value) =>
              form.setValue("planType", value as Values["planType"])
            }
            options={(["milestone", "phase", "deliverable"] as const).map(
              (value) => ({ value, label: t(`plans.types.${value}`) }),
            )}
          />

          <label htmlFor="plan-owner">{t("owner")}</label>
          <EnumSelect
            id="plan-owner"
            value={form.watch("ownerId")}
            onChange={(value) => form.setValue("ownerId", value)}
            options={memberOptions}
          />

          <label htmlFor="plan-priority">{t("priority")}</label>
          <EnumSelect
            id="plan-priority"
            value={form.watch("priority")}
            onChange={(value) =>
              form.setValue("priority", value as Values["priority"])
            }
            options={(["low", "normal", "high", "urgent"] as const).map(
              (value) => ({ value, label: t(`priorities.${value}`) }),
            )}
          />

          <label htmlFor="plan-due">{t("dueDate")}</label>
          <Input id="plan-due" type="date" {...form.register("dueDate")} />

          <label htmlFor="plan-content">{t("plans.content")}</label>
          <textarea
            id="plan-content"
            className="w-full min-h-40 rounded-md border p-3"
            {...form.register("content")}
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
