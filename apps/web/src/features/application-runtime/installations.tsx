import { Link } from "react-router";
import { Settings2 } from "lucide-react";
import { useState } from "react";
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
    candidate: RuntimeCandidate;
    installation?: RuntimeInstallation;
    action: string;
    applicationId?: string;
  }>();
  const [error, setError] = useState<unknown>();
  const [pending, setPending] = useState(false);
  const [issues, setIssues] = useState<string[]>();
  const rows = (q.data?.candidates ?? []).map((candidate, index) => ({
    id: index,
    candidate,
    installation: q.data?.items.find(
      (item) => item.pluginId === candidate.pluginId,
    ),
  }));
  type Row = (typeof rows)[number];
  const columns: AdminColumn<Row>[] = [
    {
      id: "plugin",
      header: t("plugin"),
      cell: (row) => row.candidate.pluginId,
    },
    {
      id: "version",
      header: t("version"),
      cell: (row) =>
        `${row.installation?.version ?? "—"} → ${row.candidate.version}`,
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
        const currentAvailable = q.data?.candidates.some(
          (candidate) =>
            candidate.pluginId === row.candidate.pluginId &&
            candidate.version === row.installation?.version,
        );
        return (
          <RowActions
            actions={
              canManage
                ? [
                    ...(!row.installation ||
                    row.installation.status === "uninstalled"
                      ? ["install"]
                      : !currentAvailable
                        ? ["check"]
                        : row.installation.status === "enabled"
                          ? ["disable", "check", "uninstall"]
                          : ["enable", "check", "uninstall"]),
                    ...(row.installation &&
                    row.installation.status !== "uninstalled" &&
                    row.installation.version !== row.candidate.version
                      ? ["upgrade"]
                      : []),
                    ...(row.installation?.previousVersion &&
                    q.data?.candidates.some(
                      (candidate) =>
                        candidate.pluginId === row.candidate.pluginId &&
                        candidate.version === row.installation?.previousVersion,
                    )
                      ? ["rollback"]
                      : []),
                  ]
                    .map((action) => ({
                      label: t(`installationActions.${action}`),
                      danger: action === "uninstall",
                      icon: <Settings2 size={14} />,
                      onClick: () => {
                        setChoice({ ...row, action });
                        setError(undefined);
                        setIssues(undefined);
                      },
                    }))
                    .concat(
                      (q.data?.applications ?? [])
                        .filter(
                          (app) =>
                            app.pluginId === row.candidate.pluginId &&
                            currentAvailable &&
                            row.installation?.status !== "uninstalled",
                        )
                        .map((app) => ({
                          label: `${app.applicationId}: ${t(`installationActions.${app.enabled ? "disable" : "enable"}`)}`,
                          danger: false,
                          icon: <Settings2 size={14} />,
                          onClick: () => {
                            setChoice({
                              ...row,
                              applicationId: app.applicationId,
                              action: app.enabled
                                ? "application.disable"
                                : "application.enable",
                            });
                            setError(undefined);
                            setIssues(undefined);
                          },
                        })),
                    )
                : []
            }
          />
        );
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
            <p>{choice.candidate.pluginId}</p>
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
                  const version =
                    choice.action === "rollback"
                      ? choice.installation?.previousVersion
                      : choice.candidate.version;
                  const input = {
                    pluginId: choice.candidate.pluginId,
                    ...(choice.applicationId
                      ? { applicationId: choice.applicationId }
                      : {}),
                    ...(["install", "upgrade", "rollback", "check"].includes(
                      choice.action,
                    )
                      ? { version }
                      : {}),
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
