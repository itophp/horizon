import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Play, Plus } from "lucide-react";
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
  projectPlanGet,
  projectPlanStatusChange,
  projectPlans,
  type ProjectPlan,
} from "@/lib/capabilities";

import { PlanForm } from "./forms";

// planTransitions mirrors the declared state machine; the server still rejects an
// illegal edge, so this table only decides which commands are offered.
const planTransitions: Record<string, string[]> = {
  pending: ["in_progress", "cancelled"],
  in_progress: ["done", "cancelled"],
  done: ["in_progress"],
  cancelled: [],
};

/** PlansPanel is the plan tab: a filtered table plus one create/edit dialog. */
export function PlansPanel({
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
  const [dialog, setDialog] = useState<{ record?: ProjectPlan } | null>(null);
  const [notice, setNotice] = useState("");

  const plans = useQuery({
    queryKey: ["project-plans", projectId],
    queryFn: () => projectPlans(projectId),
    retry: false,
  });
  // A plan's responsible member must belong to the project, so the dialog offers
  // exactly that list.
  const members = useQuery({
    queryKey: ["project-members", projectId],
    queryFn: () => projectMembers(projectId),
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: ({ plan, next }: { plan: ProjectPlan; next: string }) =>
      projectPlanStatusChange(plan.id, plan.status, next),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-plans", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      setNotice(t("admin:feedback.saved"));
    },
  });

  const items = plans.data?.data.items ?? [];
  const rows = useMemo(
    () =>
      items.filter((plan) =>
        `${plan.title} ${plan.ownerName}`
          .toLowerCase()
          .includes(search.toLowerCase()),
      ),
    [items, search],
  );

  const editable = canWrite && !archived;

  const columns: AdminColumn<ProjectPlan>[] = [
    {
      id: "title",
      header: t("plans.title"),
      cell: (plan) => plan.title,
    },
    {
      id: "type",
      header: t("plans.type"),
      cell: (plan) => t(`plans.types.${plan.planType}`),
    },
    {
      id: "owner",
      header: t("owner"),
      cell: (plan) => plan.ownerName,
    },
    {
      id: "due",
      header: t("dueDate"),
      cell: (plan) =>
        plan.dueDate ? <ShortDate value={plan.dueDate} /> : t("dash"),
    },
    {
      id: "status",
      header: t("status"),
      cell: (plan) => (
        <StateBadge
          label={t(`plans.states.${plan.status}`)}
          tone={stateTone(plan.status)}
        />
      ),
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (plan) => (
        <RowActions
          actions={[
            ...(editable
              ? [
                  {
                    label: t("admin:actions.edit"),
                    icon: <Pencil size={14} />,
                    // The list projection has no body, so an edit loads the full
                    // plan before the dialog opens.
                    onClick: () => {
                      void projectPlanGet(plan.id).then((response) =>
                        setDialog({ record: response.data }),
                      );
                    },
                  },
                ]
              : []),
            ...(editable
              ? planTransitions[plan.status].map((next) => ({
                  label: t(`plans.actions.${next}`),
                  icon: <Play size={14} />,
                  onClick: () => mutation.mutate({ plan, next }),
                }))
              : []),
          ]}
        />
      ),
    },
  ];

  if (plans.isPending) return <Loading />;

  return (
    <>
      {plans.isError && (
        <ErrorBanner
          error={plans.error}
          onRetry={() => {
            void plans.refetch();
          }}
        />
      )}
      <section className="admin-panel">
        <div className="table-toolbar">
          <ListSearch value={search} onChange={setSearch} />
          {editable && (
            <div className="toolbar-right">
              <Button onClick={() => setDialog({})}>
                <Plus size={15} />
                {t("plans.new")}
              </Button>
            </div>
          )}
        </div>
        <DataTable rows={rows} columns={columns} />
      </section>
      <EditorDialog
        open={!!dialog}
        onOpenChange={(open) => {
          if (!open) setDialog(null);
        }}
        title={dialog?.record ? t("edit") : t("plans.new")}
        description={t("dialog.planHint")}
        wide
      >
        <PlanForm
          key={dialog?.record?.id ?? "new"}
          projectId={projectId}
          record={dialog?.record}
          members={members.data?.data.items ?? []}
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
