import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { executionPlans } from "@/lib/capabilities";
import { formatMoment } from "@/lib/capability";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  Loading,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { usePlanScope, type Target, type PlanRow } from "./state";

export function PlanActivity({
  target,
  plans,
  kind,
}: {
  target: Target;
  plans: PlanRow[];
  kind: "history" | "records";
}) {
  const { t, i18n } = useTranslation("capability");
  const scope = usePlanScope(target);
  const [trace, setTrace] = useState<string>();
  const history = useQuery({
    queryKey: [...scope, "history"],
    queryFn: ({ signal }) => executionPlans.history(target, signal),
    enabled: kind === "history",
    retry: false,
  });
  const records = useQuery({
    queryKey: [...scope, "records"],
    queryFn: ({ signal }) => executionPlans.records(target, signal),
    enabled: kind === "records",
    retry: false,
    refetchInterval: kind === "records" ? 5000 : false,
  });
  const name = (id: string, version: number) =>
    plans.find((row) => row.plan.planId === id && row.plan.version === version)
      ?.name ?? t("plans.unavailable");
  if (kind === "history")
    return (
      <section className="admin-panel">
        {history.isPending ? (
          <Loading />
        ) : history.error ? (
          <ErrorBanner error={history.error} />
        ) : (
          <DataTable
            rows={(history.data?.data.changes ?? []).map((row) => ({
              ...row,
              id: row.ID,
            }))}
            columns={[
              {
                id: "plan",
                header: t("plans.planName"),
                cell: (r) => (
                  <>
                    {t("plans.namedVersion", {
                      name: name(r.PlanID, r.PlanVersion),
                      version: r.PlanVersion,
                    })}
                  </>
                ),
              },
              {
                id: "action",
                header: t("plans.action"),
                cell: (r) =>
                  t(r.Action === "rollback" ? "plans.rollback" : "plans.bind"),
              },
              {
                id: "reason",
                header: t("plans.reason"),
                cell: (r) => r.Reason,
              },
              {
                id: "createdAt",
                header: t("plans.time"),
                cell: (r) => formatMoment(i18n.language, r.CreatedAt),
              },
            ]}
          />
        )}
      </section>
    );
  const rows = (records.data?.data.records ?? []).map((row, id) => ({
    ...row,
    id,
  }));
  return (
    <section className="admin-panel">
      {records.isPending ? (
        <Loading />
      ) : records.error ? (
        <ErrorBanner error={records.error} />
      ) : (
        <DataTable
          rows={rows}
          columns={[
            {
              id: "plan",
              header: t("plans.planName"),
              cell: (r) => (
                <>
                  {t("plans.namedVersion", {
                    name: name(r.binding.plan.planId, r.binding.plan.version),
                    version: r.binding.plan.version,
                  })}
                </>
              ),
            },
            {
              id: "status",
              header: t("plans.status"),
              cell: (r) =>
                t(`plans.runStates.${r.runStatus || r.status}`, {
                  defaultValue: t("plans.unavailable"),
                }),
            },
            {
              id: "time",
              header: t("plans.time"),
              cell: (r) => formatMoment(i18n.language, r.startedAt),
            },
            {
              id: "trace",
              header: t("plans.trace"),
              cell: (r) => (
                <>
                  <code>{r.traceId}</code>
                  <Button
                    variant="ghost"
                    onClick={() => setTrace(r.invocationId)}
                  >
                    {t("plans.nodeEvents")}
                  </Button>
                </>
              ),
            },
            {
              id: "error",
              header: t("plans.error"),
              cell: (r) =>
                r.errorCode
                  ? t(`errors.${r.errorCode}`, {
                      defaultValue: t("errors.internal"),
                    })
                  : t("plans.none"),
            },
          ]}
        />
      )}
      {trace && (
        <TraceDialog
          invocationId={trace}
          scope={scope}
          onClose={() => setTrace(undefined)}
        />
      )}
    </section>
  );
}
function TraceDialog({
  invocationId,
  scope,
  onClose,
}: {
  invocationId: string;
  scope: ReturnType<typeof usePlanScope>;
  onClose: () => void;
}) {
  const { t, i18n } = useTranslation("capability");
  const trace = useQuery({
    queryKey: [...scope, "trace", invocationId],
    queryFn: ({ signal }) => executionPlans.trace({ invocationId }, signal),
    retry: false,
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t("plans.nodeEvents")}
      description={t("plans.traceHint")}
    >
      {trace.isPending ? (
        <Loading />
      ) : trace.error ? (
        <ErrorBanner error={trace.error} />
      ) : (
        <DataTable
          rows={(trace.data?.data.events ?? []).map((event) => ({
            ...event,
            id: event.sequence,
          }))}
          columns={[
            { id: "step", header: t("plans.step"), cell: (r) => r.stepName },
            {
              id: "event",
              header: t("plans.action"),
              cell: (r) => t(`plans.events.${r.eventType}`),
            },
            {
              id: "time",
              header: t("plans.time"),
              cell: (r) => formatMoment(i18n.language, r.occurredAt),
            },
          ]}
        />
      )}
    </EditorDialog>
  );
}
