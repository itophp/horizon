import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  ErrorBanner,
  Loading,
  PageHeading,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  drivePersonalEnsure,
  driveSpaceCreate,
  driveSpaces,
  type DriveSpace,
} from "@/lib/capabilities";
import { NameDialog } from "./name-dialog";

export function DrivePage() {
  const { t } = useTranslation(["drive", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const [creating, setCreating] = useState(false);
  const canUse = session.permissions.includes("drive:use");
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`;
  const key = ["drive", identity, "spaces"];
  const ensured = useRef("");
  const { mutate: ensure, error: ensureError } = useMutation({
    mutationFn: drivePersonalEnsure,
    onSuccess: () => client.invalidateQueries({ queryKey: key }),
  });
  useEffect(() => {
    if (canUse && ensured.current !== identity) {
      ensured.current = identity;
      ensure();
    }
  }, [canUse, ensure, identity]);
  const query = useQuery({
    queryKey: key,
    queryFn: driveSpaces,
    enabled: canUse,
    retry: false,
  });
  const columns: AdminColumn<DriveSpace>[] = [
    {
      id: "name",
      header: t("drive:name"),
      cell: (row) => (
        <Link to={`/drive/${row.id}`}>
          {row.kind === "personal" ? t("drive:personal") : row.name}
        </Link>
      ),
    },
    {
      id: "kind",
      header: t("drive:kind"),
      cell: (row) =>
        t(row.kind === "personal" ? "drive:personal" : "drive:shared"),
    },
    {
      id: "role",
      header: t("drive:role"),
      cell: (row) => t(`drive:roles.${row.role}`),
    },
  ];
  if (!canUse)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading title={t("drive:heading")} subtitle={t("drive:description")}>
        {session.permissions.includes("drive:create") && (
          <Button onClick={() => setCreating(true)}>
            {t("drive:createSpace")}
          </Button>
        )}
      </PageHeading>
      {ensureError && <ErrorBanner error={ensureError} />}
      {query.isError && (
        <ErrorBanner
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      <DataTable rows={query.data?.data.items ?? []} columns={columns} />
      {creating && (
        <NameDialog
          title={t("drive:createSpace")}
          maxLength={120}
          onClose={() => setCreating(false)}
          onSave={async (name) => {
            await driveSpaceCreate(name);
            await client.invalidateQueries({ queryKey: key });
          }}
        />
      )}
    </>
  );
}
