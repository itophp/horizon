import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Blocks,
  ChevronRight,
  GitMerge,
  History,
  RefreshCw,
  Rocket,
} from "lucide-react";
import { runtimeRead } from "@/lib/capabilities";
import {
  runtimeDefinitionListSchema,
  runtimeDefinitionViewSchema,
} from "@/api/generated/runtime";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  ErrorBanner,
  Loading,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { TaskPanel } from "./task-panel";
import { DesignVersion } from "./design-version";
import { DesignHistory } from "./design-history";
import { DesignBaseline } from "./design-baseline";
import { DesignContracts } from "./design-contracts";
import { DesignRequest } from "./design-request";
import { DesignOverview } from "./design-overview";

const tabs = ["candidate", "history", "baseline", "contracts"] as const;
type DesignTab = (typeof tabs)[number];

export function ApplicationDesignPage() {
  const { t, i18n } = useTranslation("runtime");
  const session = useSession();
  const { applicationId = "new" } = useParams();
  const [params, setParams] = useSearchParams();
  const isNew = applicationId === "new";
  const identity = JSON.stringify([
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
  ]);
  const canDesign = session.permissions.includes("applications:design");
  const canGenerate =
    canDesign &&
    session.permissions.includes("assistant:use") &&
    session.permissions.includes("orchestration:run");
  const list = useQuery({
    queryKey: ["runtime-design-list", identity],
    queryFn: async ({ signal }) =>
      runtimeDefinitionListSchema.parse(
        await runtimeRead("application.runtime.definition.list", {}, signal),
      ),
    enabled: canDesign,
    retry: false,
  });
  const current = useQuery({
    queryKey: ["runtime-design-version", identity, applicationId],
    queryFn: async ({ signal }) =>
      runtimeDefinitionViewSchema.parse(
        await runtimeRead(
          "application.runtime.definition.get",
          { applicationId },
          signal,
        ),
      ),
    enabled: canDesign && !isNew,
    retry: false,
  });
  const [runId, setRunId] = useState<number>();
  const entry = list.data?.items.find(
    (item) => item.applicationId === applicationId,
  );
  const label = entry
    ? (i18n.language.startsWith("zh") ? entry.labelZhCN : entry.labelEn) ||
      t(entry.labelKey)
    : applicationId;
  const title = isNew ? t("design.new") : label;
  const subtitle = isNew ? t("design.newHint") : t("design.hint");
  const requestedTab = params.get("tab");
  const tab: DesignTab = (tabs as readonly string[]).includes(
    requestedTab ?? "",
  )
    ? (requestedTab as DesignTab)
    : "candidate";
  const updateTab = (value: string) => {
    if (!(tabs as readonly string[]).includes(value)) return;
    const next = new URLSearchParams(params);
    if (value === "candidate") next.delete("tab");
    else next.set("tab", value);
    setParams(next, { replace: true });
  };
  const back = (
    <nav className="design-breadcrumb" aria-label={t("design.breadcrumb")}>
      <Link to="/applications">
        <ArrowLeft size={13} />
        {t("applications")}
      </Link>
      <ChevronRight size={11} aria-hidden="true" />
      <span>{t("design.heading")}</span>
    </nav>
  );
  if (!canDesign) {
    return (
      <>
        {back}
        <PageHeading title={t("design.heading")} subtitle={t("design.hint")} />
        <p role="status">{t("design.forbidden")}</p>
      </>
    );
  }
  if (list.isPending) {
    return (
      <>
        {back}
        <Loading />
      </>
    );
  }
  if (list.isError) {
    return (
      <>
        {back}
        <ErrorBanner error={list.error} />
      </>
    );
  }
  if (!isNew && !entry) {
    return (
      <>
        {back}
        <PageHeading title={t("design.heading")} subtitle={t("design.hint")} />
        <p role="status">{t("design.unavailable")}</p>
      </>
    );
  }
  return (
    <>
      {back}
      <PageHeading title={title} subtitle={subtitle}>
        {!isNew && (
          <Button
            type="button"
            variant="outline"
            disabled={current.isFetching}
            onClick={() => {
              void list.refetch();
              void current.refetch();
            }}
          >
            <RefreshCw
              size={14}
              className={current.isFetching ? "animate-spin" : undefined}
            />
            {t("design.refresh")}
          </Button>
        )}
      </PageHeading>
      <div className="design-layout">
        <div className="design-main">
          <section className="admin-panel">
            <header className="panel-heading">
              <div>
                <h2>{t("design.workspace")}</h2>
                <p>{t("design.workspaceHint")}</p>
              </div>
              <span className="design-kicker">{title}</span>
            </header>
            <DesignRequest
              applicationId={isNew ? "new" : applicationId}
              canGenerate={canGenerate}
              onStarted={setRunId}
            />
          </section>
          {runId && <TaskPanel key={runId} runId={runId} />}
          {!isNew &&
            (current.isPending ? (
              <Loading />
            ) : current.isError ? (
              <ErrorBanner error={current.error} />
            ) : (
              current.data && (
                <section className="admin-panel design-tabs">
                  <Tabs
                    value={tab}
                    onValueChange={(value) => updateTab(String(value))}
                  >
                    <TabsList>
                      <TabsTrigger value="candidate">
                        <Rocket size={13} />
                        {t("design.tabs.candidate")}
                      </TabsTrigger>
                      <TabsTrigger value="history">
                        <History size={13} />
                        {t("design.tabs.history")}
                      </TabsTrigger>
                      <TabsTrigger value="baseline">
                        <GitMerge size={13} />
                        {t("design.tabs.baseline")}
                      </TabsTrigger>
                      <TabsTrigger value="contracts">
                        <Blocks size={13} />
                        {t("design.tabs.contracts")}
                      </TabsTrigger>
                    </TabsList>
                    <TabsContent value="candidate">
                      {tab === "candidate" && (
                        <DesignVersion
                          key={identity + applicationId}
                          applicationId={applicationId}
                          identity={identity}
                        />
                      )}
                    </TabsContent>
                    <TabsContent value="history">
                      {tab === "history" && (
                        <DesignHistory
                          current={current.data}
                          identity={identity}
                        />
                      )}
                    </TabsContent>
                    <TabsContent value="baseline">
                      {tab === "baseline" && (
                        <DesignBaseline
                          applicationId={applicationId}
                          identity={identity}
                          revision={current.data.revision}
                        />
                      )}
                    </TabsContent>
                    <TabsContent value="contracts">
                      {tab === "contracts" && (
                        <DesignContracts identity={identity} />
                      )}
                    </TabsContent>
                  </Tabs>
                </section>
              )
            ))}
        </div>
        {!isNew && current.data && (
          <DesignOverview
            applicationId={applicationId}
            identity={identity}
            label={label}
            view={current.data}
          />
        )}
      </div>
    </>
  );
}
