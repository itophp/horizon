import { Link } from "react-router";
import { Settings2 } from "lucide-react";
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
  Loading,
  type AdminColumn,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
export function InstallationsPage() {
  const { t } = useTranslation("runtime");
  const session = useSession();
  const client = useQueryClient();
  const canManage = session.permissions.includes("applications:manage");
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
  const rows = useMemo(() => {
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
  type Row = (typeof rows)[number];
  const columns: AdminColumn<Row>[] = [
    {
      id: "plugin",
      header: t("plugin"),
      cell: (row) => row.pluginId,
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
        if (!canManage) return null;
        const { pluginId, candidates, installation } = row;
        const installed = installation && installation.status !== "uninstalled";
        const currentAvailable = candidates.some(
          (candidate) => candidate.version === installation?.version,
        );
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
              ? `${applicationId}: ${name}`
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
        const actions = candidates.flatMap(({ version }) =>
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
        );
        if (installed && currentAvailable) {
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
        return <RowActions actions={actions} />;
      },
    },
  ];
  return (
    <>
      <PageHeading title={t("applications")} subtitle={t("installationHint")} />
      {session.permissions.includes("applications:design") && (
        <Link to="/applications/design">{t("design.heading")}</Link>
      )}
      {q.isPending ? (
        <Loading />
      ) : q.isError ? (
        <ErrorBanner error={q.error} />
      ) : (
        <DataTable rows={rows} columns={columns} />
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
