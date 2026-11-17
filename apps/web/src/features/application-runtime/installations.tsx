import { Link, useNavigate } from "react-router";
import { PenLine, Plus, Settings2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/api/client/request";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import {
  runtimeInstallationCatalogSchema,
  runtimeCompatibilityReportSchema,
  type RuntimeCandidate,
  type RuntimeInstallation,
} from "@/api/generated/runtime";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  DataTable,
  RowActions,
  EditorDialog,
  ErrorBanner,
  EmptyState,
  Loading,
  type AdminColumn,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import "./runtime.css";

type CandidateGroup = {
  id: number;
  pluginId: string;
  candidates: RuntimeCandidate[];
  installation?: RuntimeInstallation;
};

function compareVersions(left: string, right: string) {
  const a = left.split(".").map(Number);
  const b = right.split(".").map(Number);
  for (let i = 0; i < Math.max(a.length, b.length); i += 1) {
    const difference = (a[i] ?? 0) - (b[i] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}

function latestCandidate(candidates: RuntimeCandidate[]) {
  return candidates.reduce((best, candidate) =>
    compareVersions(candidate.version, best.version) > 0 ? candidate : best,
  );
}

function applicationNames(
  candidate: RuntimeCandidate,
  translate: (key: string) => string,
) {
  return candidate.applications
    .map((id) => {
      const key = candidate.labelKeys[id];
      return key ? translate(key) : id;
    })
    .join(" · ");
}

export function InstallationsPage() {
  const { t } = useTranslation("runtime");
  const session = useSession();
  const client = useQueryClient();
  const navigate = useNavigate();
  const canManage = session.permissions.includes("applications:manage");
  const canDesign = session.permissions.includes("applications:design");
  const key = [
    "runtime-installations",
    session.tenantId,
    session.account.id,
    session.activeDepartmentId,
    session.authorizationVersion,
  ];
  const q = useQuery({
    queryKey: key,
    queryFn: async ({ signal }) =>
      runtimeInstallationCatalogSchema.parse(
        await runtimeRead("application.runtime.installation.list", {}, signal),
      ),
    enabled: session.permissions.includes("applications:read"),
    retry: false,
  });
  const [choice, setChoice] = useState<{
    pluginId: string;
    version?: string;
    installation?: RuntimeInstallation;
    action: string;
    applicationId?: string;
  }>();
  const [error, setError] = useState<unknown>();
  const [pending, setPending] = useState(false);
  const [issues, setIssues] = useState<string[]>();
  const rows = useMemo<CandidateGroup[]>(() => {
    const candidatesByPlugin = new Map<string, RuntimeCandidate[]>();
    for (const candidate of q.data?.candidates ?? []) {
      const candidates = candidatesByPlugin.get(candidate.pluginId) ?? [];
      candidates.push(candidate);
      candidatesByPlugin.set(candidate.pluginId, candidates);
    }
    return Array.from(candidatesByPlugin, ([pluginId, candidates], index) => ({
      id: index,
      pluginId,
      candidates,
      installation: q.data?.items.find((item) => item.pluginId === pluginId),
    }));
  }, [q.data]);
  const installableRows = rows.filter(
    (row) => !latestCandidate(row.candidates).base,
  );
  type Row = (typeof rows)[number];
  const columns: AdminColumn<Row>[] = [
    {
      id: "application",
      header: t("application"),
      cell: (row) => applicationNames(latestCandidate(row.candidates), t),
    },
    {
      id: "version",
      header: t("version"),
      cell: (row) =>
        t("installationVersions", {
          current:
            row.installation?.status === "uninstalled"
              ? "—"
              : (row.installation?.version ?? "—"),
          available: row.candidates
            .map((candidate) => candidate.version)
            .join(", "),
        }),
    },
    {
      id: "status",
      header: t("status"),
      cell: (row) =>
        t(`installationStates.${row.installation?.status ?? "uninstalled"}`),
    },
    {
      id: "actions",
      header: t("actions"),
      cell: (row) => {
        if (!canManage && !canDesign) return null;
        const { pluginId, candidates, installation } = row;
        const latest = latestCandidate(candidates);
        const installed = installation && installation.status !== "uninstalled";
        const currentAvailable = candidates.some(
          (candidate) => candidate.version === installation?.version,
        );
        const applicationLabel = (applicationId: string) => {
          const applicationKey = latest.labelKeys[applicationId];
          return applicationKey ? t(applicationKey) : applicationId;
        };
        const command = (
          action: string,
          version?: string,
          applicationId?: string,
        ) => {
          const name = t(
            `installationActions.${action.replace("application.", "")}`,
          );
          return {
            label: applicationId
              ? `${applicationLabel(applicationId)}: ${name}`
              : version
                ? t("installationVersionAction", { action: name, version })
                : name,
            danger: action === "uninstall",
            icon: <Settings2 size={14} />,
            onClick: () => {
              setChoice({
                pluginId,
                installation,
                action,
                version,
                applicationId,
              });
              setError(undefined);
              setIssues(undefined);
            },
          };
        };
        const actions = canManage
          ? candidates.flatMap(({ version }) =>
              installed
                ? [
                    command("check", version),
                    ...(version !== installation.version
                      ? [command("upgrade", version)]
                      : []),
                    ...(version === installation.previousVersion
                      ? [command("rollback", version)]
                      : []),
                  ]
                : [command("install", version)],
            )
          : [];
        const designLinks = canDesign
          ? latest.applications.map((applicationId) => (
              <Link
                key={applicationId}
                className="design-row-link"
                to={`/applications/design/${encodeURIComponent(applicationId)}`}
              >
                <PenLine size={13} />
                {t("design.action")}
              </Link>
            ))
          : [];
        if (canManage && installed && currentAvailable) {
          actions.unshift(
            command(installation.status === "enabled" ? "disable" : "enable"),
            command("uninstall"),
          );
          for (const app of q.data?.applications ?? []) {
            if (app.pluginId === pluginId) {
              actions.push(
                command(
                  app.enabled ? "application.disable" : "application.enable",
                  undefined,
                  app.applicationId,
                ),
              );
            }
          }
        }
        return designLinks.length || actions.length ? (
          <div className="design-row-actions">
            {designLinks}
            {actions.length > 0 && <RowActions actions={actions} />}
          </div>
        ) : null;
      },
    },
  ];
  return (
    <>
      <PageHeading title={t("applications")} subtitle={t("applicationsHint")} />
      {q.isPending ? (
        <Loading />
      ) : q.isError ? (
        <ErrorBanner error={q.error} />
      ) : (
        <div className="application-management">
          <section className="admin-panel">
            <header className="panel-heading">
              <div>
                <h2>{t("installableApplications")}</h2>
                <p>{t("installableApplicationHint")}</p>
              </div>
              {canDesign && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => navigate("/applications/design/new")}
                >
                  <Plus size={14} />
                  {t("design.createApplication")}
                </Button>
              )}
            </header>
            {installableRows.length ? (
              <DataTable rows={installableRows} columns={columns} />
            ) : (
              <EmptyState />
            )}
          </section>
        </div>
      )}
      <EditorDialog
        open={!!choice}
        onOpenChange={(open) => {
          if (!open && !pending) setChoice(undefined);
        }}
        title={
          choice
            ? t(
                `installationActions.${choice.action.replace("application.", "")}`,
              )
            : t("applications")
        }
        description={t("installationHint")}
      >
        {choice && (
          <>
            <p>{choice.pluginId}</p>
            {choice.version && (
              <p>
                {t("installationTargetVersion", { version: choice.version })}
              </p>
            )}
            {error !== undefined && <ErrorBanner error={error} />}
            {issues && (
              <ul>
                {issues.map((issue) => (
                  <li key={issue}>{t(`compatibilityIssues.${issue}`)}</li>
                ))}
              </ul>
            )}
            <Button
              type="button"
              disabled={pending}
              onClick={async () => {
                setPending(true);
                setError(undefined);
                try {
                  const input = {
                    pluginId: choice.pluginId,
                    ...(choice.applicationId
                      ? { applicationId: choice.applicationId }
                      : {}),
                    ...(choice.version ? { version: choice.version } : {}),
                    ...(choice.installation && choice.action !== "check"
                      ? { revision: choice.installation.revision }
                      : {}),
                  };
                  if (choice.action === "check") {
                    const report = runtimeCompatibilityReportSchema.parse(
                      await runtimeRead(
                        "application.runtime.installation.check",
                        input,
                      ),
                    );
                    setIssues(
                      report.issues.length ? report.issues : ["compatible"],
                    );
                  } else {
                    const result = await runtimeExecute(
                      choice.applicationId
                        ? `application.runtime.${choice.action}`
                        : `application.runtime.installation.${choice.action}`,
                      "1.0.0",
                      input,
                      crypto.randomUUID(),
                      crypto.randomUUID(),
                    );
                    if (result.status !== "succeeded")
                      throw new ApiError(
                        result.errorCode || "unavailable",
                        400,
                      );
                    await client.invalidateQueries({
                      predicate: (query) =>
                        typeof query.queryKey[0] === "string" &&
                        query.queryKey[0].startsWith("runtime"),
                    });
                    setChoice(undefined);
                  }
                } catch (error) {
                  setError(error);
                } finally {
                  setPending(false);
                }
              }}
            >
              {t("confirm")}
            </Button>
          </>
        )}
      </EditorDialog>
    </>
  );
}
