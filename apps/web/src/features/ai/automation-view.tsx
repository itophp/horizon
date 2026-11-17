import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, MessageSquare } from "lucide-react";
import { Link, Navigate, useParams, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import { ApplicationPage } from "@/features/application-runtime/page";
import { RuntimeRouteParams } from "@/features/application-runtime/route-params";
import { runtimeRead } from "@/lib/capabilities";
import { useAiKey } from "./shared";

function automationPath(id?: string, runId?: string) {
  if (runId) return `/ai/automation/runs/${encodeURIComponent(runId)}`;
  if (id) return `/ai/automation/${encodeURIComponent(id)}`;
  return "/ai/automation";
}

export function LegacyAutomationRedirect() {
  const { id, runId } = useParams();
  const [params] = useSearchParams();
  const next = new URLSearchParams(params);
  const path = automationPath(
    id ?? next.get("task") ?? undefined,
    runId ?? next.get("run") ?? undefined,
  );
  if (next.get("tab") === "automation") next.delete("tab");
  next.delete("task");
  next.delete("run");
  return <Navigate to={{ pathname: path, search: next.toString() }} replace />;
}

export function AutomationView() {
  const { t } = useTranslation(["ai", "runtime"]);
  const { permissions } = useSession();
  if (!permissions.includes("assistant:use")) {
    return <p role="alert">{t("ai:forbidden")}</p>;
  }
  if (!permissions.includes("automation:read")) {
    return <p role="alert">{t("runtime:denied")}</p>;
  }
  return <AutomationContent />;
}

function AutomationContent() {
  const { t } = useTranslation("ai");
  const { id: taskId = "", runId = "" } = useParams();
  const runKey = useAiKey("automation-run", runId);
  const id = Number(runId);
  const run = useQuery({
    queryKey: runKey,
    queryFn: ({ signal }) => runtimeRead("automation.run.get", { id }, signal),
    enabled: Number.isSafeInteger(id) && id > 0,
    retry: false,
    refetchInterval: 15000,
  });
  const result = run.data as { aiRunId?: number; taskId?: number } | undefined;
  const backPath = runId
    ? automationPath(result?.taskId ? String(result.taskId) : undefined)
    : taskId
      ? automationPath()
      : "/ai";
  return (
    <section className="min-w-0 space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Button
          variant="ghost"
          nativeButton={false}
          render={<Link to={backPath} />}
        >
          <ArrowLeft aria-hidden />
          {t(
            runId && result?.taskId
              ? "automation.backToTask"
              : runId || taskId
                ? "automation.back"
                : "automation.backToWorkbench",
          )}
        </Button>
        {runId && !!result?.aiRunId && (
          <Button
            variant="outline"
            nativeButton={false}
            render={
              <Link
                to={`/ai?session=${encodeURIComponent(`automation:${runId}`)}`}
              />
            }
          >
            <MessageSquare aria-hidden />
            {t("automation.openConversation")}
          </Button>
        )}
      </div>
      <RuntimeRouteParams.Provider value={{ id: taskId, runId }}>
        <ApplicationPage
          applicationId="automation"
          pageId={
            runId
              ? "automation.run.detail"
              : taskId
                ? "automation.detail"
                : "automation.list"
          }
        />
      </RuntimeRouteParams.Provider>
    </section>
  );
}
