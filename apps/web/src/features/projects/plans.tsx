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
  projectPlanCreate,
  projectPlanGet,
  projectPlanStatusChange,
  projectPlanUpdate,
  projectPlans,
  type ProjectMember,
  type ProjectPlan,
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

// planTransitions mirrors the declared state machine; the server still rejects
// an illegal edge, so this table only decides which buttons are offered.
const planTransitions: Record<string, string[]> = {
  pending: ["in_progress", "cancelled"],
  in_progress: ["done", "cancelled"],
  done: ["in_progress"],
  cancelled: [],
};

/** PlansPanel lists plans; creating and editing happen in a dialog. */
export function PlansPanel({
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
  const [dialog, setDialog] = useState<{ planId: number | null } | null>(null);

  const plans = useQuery({
    queryKey: ["project-plans", projectId],
    queryFn: () => projectPlans(projectId),
    retry: false,
  });

  // The responsible member of a plan must be a project member, so the dialog
  // offers exactly that list.
  const members = useQuery({
    queryKey: ["project-members", projectId],
    queryFn: () => projectMembers(projectId),
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

  const editable = canWrite && !archived;
  const rows = plans.data?.data.items ?? [];

  return (
    <section className="space-y-3">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold">{t("plans.heading")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("plans.description")}
          </p>
        </div>
        {editable && (
          <Button onClick={() => setDialog({ planId: null })}>
            {t("plans.new")}
          </Button>
        )}
      </div>
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
                      onClick={() => setDialog({ planId: plan.id })}
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
        <PlanDialog
          open={dialog !== null}
          planId={dialog?.planId ?? null}
          projectId={projectId}
          members={members.data?.data.items ?? []}
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
        />
      )}
    </section>
  );
}

function PlanDialog({
  open,
  onOpenChange,
  projectId,
  planId,
  members,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: number;
  planId: number | null;
  members: ProjectMember[];
}) {
  const { t } = useTranslation("projects");
  // The list projection has no body, so an edit loads the full plan first.
  const detail = useQuery({
    queryKey: ["project-plan", planId],
    queryFn: () => projectPlanGet(planId as number),
    enabled: open && planId !== null,
    retry: false,
  });

  return (
    <EditorDialog
      open={open}
      onOpenChange={onOpenChange}
      title={planId === null ? t("plans.new") : t("edit")}
      description={t("dialog.planHint")}
      wide
    >
      {planId !== null && detail.isPending ? (
        <p>{t("loading")}</p>
      ) : (
        <PlanForm
          projectId={projectId}
          plan={detail.data?.data}
          members={members}
          onDone={() => onOpenChange(false)}
        />
      )}
    </EditorDialog>
  );
}

function PlanForm({
  projectId,
  plan,
  members,
  onDone,
}: {
  projectId: number;
  plan?: ProjectPlan;
  members: ProjectMember[];
  onDone: () => void;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      planType: (plan?.planType as Values["planType"]) ?? "phase",
      title: plan?.title ?? "",
      content: plan?.content ?? "",
      priority: (plan?.priority as Values["priority"]) ?? "normal",
      dueDate: plan?.dueDate ?? "",
      ownerId: plan ? String(plan.ownerId) : "",
    },
  });

  const mutation = useMutation({
    mutationFn: async (values: Values) => {
      const payload = {
        planType: values.planType,
        title: values.title,
        content: values.content,
        priority: values.priority,
        dueDate: values.dueDate,
        ownerId: Number(values.ownerId),
      };
      if (plan) await projectPlanUpdate({ id: plan.id, ...payload });
      else await projectPlanCreate({ projectId, ...payload });
    },
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-plans", projectId],
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
        options={members.map((member) => ({
          value: String(member.accountId),
          label: member.name,
        }))}
      />

      <label htmlFor="plan-priority">{t("priority")}</label>
      <EnumSelect
        id="plan-priority"
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

      <label htmlFor="plan-due">{t("dueDate")}</label>
      <Input id="plan-due" type="date" {...form.register("dueDate")} />

      <label htmlFor="plan-content">{t("plans.content")}</label>
      <textarea
        id="plan-content"
        className="min-h-40 w-full rounded-md border p-3"
        {...form.register("content")}
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
