import { SpaceNavigation } from "./space-navigation";
import { ArrowLeft } from "lucide-react";
import { Trash2 } from "lucide-react";
import { MemberForm } from "./forms";
import { DriveSelect } from "./select";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  Loading,
  PageHeading,
  RowActions,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  driveMemberChange,
  driveMemberRemove,
  driveMembers,
  driveSpaces,
  type DriveMember,
} from "@/lib/capabilities";

export function DriveMembersPage() {
  const { t } = useTranslation(["drive", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const spaceId = Number(useParams().id);
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`;
  const key = ["drive", identity, spaceId, "members"];
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<DriveMember>();
  const spaces = useQuery({
    queryKey: ["drive", identity, "spaces"],
    queryFn: driveSpaces,
    enabled: session.permissions.includes("drive:use") && spaceId > 0,
    retry: false,
  });
  const canManage =
    spaces.data?.data.items.some(
      (space) =>
        space.id === spaceId &&
        space.kind === "shared" &&
        space.role === "admin",
    ) ?? false;
  const query = useQuery({
    queryKey: key,
    queryFn: () => driveMembers(spaceId),
    enabled: canManage,
    retry: false,
  });
  const change = useMutation({
    mutationFn: ({ id, role }: { id: number; role: DriveMember["role"] }) =>
      driveMemberChange(spaceId, id, role),
    onSuccess: () =>
      client.invalidateQueries({ queryKey: ["drive", identity] }),
  });
  const remove = useMutation({
    mutationFn: (id: number) => driveMemberRemove(spaceId, id),
    onSuccess: async () => {
      setRemoving(undefined);
      await client.invalidateQueries({ queryKey: ["drive", identity] });
    },
  });
  const columns: AdminColumn<DriveMember>[] = [
    {
      id: "type",
      header: t("drive:targetType"),
      cell: (row) =>
        t(row.targetType === "account" ? "drive:account" : "drive:department"),
    },
    { id: "target", header: t("drive:targetId"), cell: (row) => row.targetId },
    {
      id: "role",
      header: t("drive:role"),
      cell: (row) => (
        <DriveSelect
          label={t("drive:changeRole")}
          value={row.role}
          disabled={change.isPending}
          onValueChange={(role) => change.mutate({ id: row.id, role })}
          options={(["admin", "editor", "viewer"] as const).map((value) => ({
            value,
            label: t(`drive:roles.${value}`),
          }))}
        />
      ),
    },
    {
      id: "actions",
      header: t("drive:actions"),
      cell: (row) => (
        <RowActions
          actions={[
            {
              icon: <Trash2 size={14} />,
              label: t("drive:remove"),
              danger: true,
              onClick: () => setRemoving(row),
            },
          ]}
        />
      ),
    },
  ];
  if (
    !session.permissions.includes("drive:use") ||
    !Number.isSafeInteger(spaceId) ||
    spaceId < 1
  )
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (spaces.isPending) return <Loading />;
  if (spaces.isError) return <ErrorBanner error={spaces.error} />;
  if (!canManage)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("drive:members")}
        subtitle={t("drive:editDescription")}
      >
        <Button variant="outline" render={<Link to={`/drive/${spaceId}`} />}>
          <ArrowLeft />
          {t("drive:open")}
        </Button>
        <Button onClick={() => setAdding(true)}>{t("drive:addMember")}</Button>
      </PageHeading>
      {spaces.data?.data.items.find((space) => space.id === spaceId) && (
        <SpaceNavigation
          space={spaces.data.data.items.find((space) => space.id === spaceId)!}
          current="members"
        />
      )}
      {query.isError && <ErrorBanner error={query.error} />}
      {change.isError && <ErrorBanner error={change.error} />}
      <DataTable rows={query.data?.data.items ?? []} columns={columns} />
      {adding && (
        <MemberForm
          spaceId={spaceId}
          onClose={() => setAdding(false)}
          onSaved={() =>
            client.invalidateQueries({ queryKey: ["drive", identity] })
          }
        />
      )}
      {removing && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setRemoving(undefined);
          }}
          title={t("drive:remove")}
          description={t("drive:editDescription")}
        >
          {remove.isError && <ErrorBanner error={remove.error} />}
          <div className="editor-footer">
            <Button
              variant="outline"
              type="button"
              onClick={() => setRemoving(undefined)}
            >
              {t("drive:cancel")}
            </Button>
            <Button
              variant="destructive"
              type="button"
              disabled={remove.isPending}
              onClick={() => remove.mutate(removing.id)}
            >
              {t("drive:remove")}
            </Button>
          </div>
        </EditorDialog>
      )}
    </>
  );
}
