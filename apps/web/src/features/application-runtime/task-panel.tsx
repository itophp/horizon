import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { aiRunDetail } from "@/api/generated/baseline";
import { runtimeExecute } from "@/lib/capabilities";
import { subscribeTaskEvents } from "@/api/client/task-events";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/features/administration/shared";
import { useSession } from "@/features/administration/auth";
export function TaskPanel({ runId }: { runId: number }) {
  const { t } = useTranslation("runtime");
  const session = useSession();
  const client = useQueryClient();
  const identity = JSON.stringify([
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
  ]);
  const [failed, setFailed] = useState(false);
  const [error, setError] = useState<unknown>();
  const key = ["runtime-task", identity, runId];
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => aiRunDetail(runId, { signal }),
    retry: false,
    refetchInterval: (query) =>
      failed &&
      !(
        query.state.data?.status === 200 &&
        ["completed", "failed", "cancelled"].includes(
          query.state.data.data.status,
        )
      )
        ? 5000
        : false,
  });
  useEffect(() => {
    let sequence = 0;
    return subscribeTaskEvents(
      runId,
      (event) => {
        const n = Number(event.eventId);
        if (!Number.isSafeInteger(n) || n < sequence) return;
        sequence = n;
        if (["RUN_FINISHED", "RUN_ERROR"].includes(event.type))
          void client.invalidateQueries({ queryKey: ["protocol-v3"] });
        void client.invalidateQueries({
          queryKey: ["runtime-task", identity, runId],
        });
      },
      () => setFailed(true),
    );
  }, [runId, identity, client]);
  const run = query.data?.status === 200 ? query.data.data : undefined;
  const terminal =
    run && ["completed", "failed", "cancelled"].includes(run.status);
  useEffect(() => {
    if (terminal)
      void client.invalidateQueries({
        predicate: (query) =>
          typeof query.queryKey[0] === "string" &&
          (query.queryKey[0] === "protocol-v3" ||
            query.queryKey[0].startsWith("runtime-design")),
      });
  }, [terminal, client]);
  return (
    <section
      className="admin-panel"
      aria-label={t("taskHeading", { id: runId })}
    >
      <h3>{t("taskHeading", { id: runId })}</h3>
      {run && (
        <>
          <p role="status">{t(`taskStates.${run.status}`)}</p>
          <p>{run.answer}</p>
          <small>{t("reference", { id: run.traceId })}</small>
        </>
      )}
      {(query.isError || error !== undefined) && (
        <ErrorBanner error={error ?? query.error} />
      )}
      {run && ["running", "awaiting_approval"].includes(run.status) && (
        <Button
          type="button"
          onClick={async () => {
            try {
              const result = await runtimeExecute(
                "orchestration.workflow.cancel",
                "1.0.0",
                { runId },
                crypto.randomUUID(),
                run.correlationId,
              );
              if (result.status === "failed") throw new Error(result.errorCode);
              await query.refetch();
            } catch (error) {
              setError(error);
            }
          }}
        >
          {t("cancelTask")}
        </Button>
      )}
    </section>
  );
}
