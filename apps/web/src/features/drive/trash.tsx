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
  ShortDate,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  driveFileRestore,
  driveFolderRestore,
  driveSpaces,
  driveTrash,
  driveTrashPurge,
  type DriveTrashItem,
} from "@/lib/capabilities";
import { MoveDialog } from "./move-dialog";
import { NameDialog } from "./name-dialog";

export function DriveTrashPage() {
  const { t } = useTranslation(["drive", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const spaceId = Number(useParams().id);
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`;
  const [selected, setSelected] = useState<DriveTrashItem>();
  const [name, setName] = useState<string>();
  const [purging, setPurging] = useState<DriveTrashItem>();
  const spaces = useQuery({
    queryKey: ["drive", identity, "spaces"],
    queryFn: driveSpaces,
    enabled: session.permissions.includes("drive:use") && spaceId > 0,
    retry: false,
  });
  const canEdit =
    spaces.data?.data.items.some(
      (space) =>
        space.id === spaceId &&
        (space.role === "admin" || space.role === "editor"),
    ) ?? false;
  const canPurge =
    spaces.data?.data.items.some(
      (space) => space.id === spaceId && space.role === "admin",
    ) ?? false;
  const purge = useMutation({
    mutationFn: (batchId: number) => driveTrashPurge(spaceId, batchId),
    onSuccess: async () => {
      setPurging(undefined);
      await client.invalidateQueries({ queryKey: ["drive", identity] });
    },
  });
  const query = useQuery({
    queryKey: ["drive", identity, spaceId, "trash"],
    queryFn: () => driveTrash(spaceId),
    enabled: canEdit,
    retry: false,
  });
  const columns: AdminColumn<DriveTrashItem>[] = [
    { id: "name", header: t("drive:name"), cell: (row) => row.name },
    {
      id: "kind",
      header: t("drive:kind"),
      cell: (row) => t(row.kind === "folder" ? "drive:folder" : "drive:file"),
    },
    {
      id: "deleted",
      header: t("drive:deletedAt"),
      cell: (row) => <ShortDate value={row.deletedAt} />,
    },
    {
      id: "expires",
      header: t("drive:expiresAt"),
      cell: (row) => <ShortDate value={row.expiresAt} />,
    },
    {
      id: "actions",
      header: t("drive:actions"),
      cell: (row) => (
        <>
          <Button
            type="button"
            variant="ghost"
            onClick={() => setSelected(row)}
          >
            {t("drive:restore")}
          </Button>
          {canPurge && (
            <Button
              type="button"
              variant="ghost"
              onClick={() => setPurging(row)}
            >
              {t("drive:purge")}
            </Button>
          )}
        </>
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
  if (!canEdit)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("drive:trashTitle")}
        subtitle={t("drive:editDescription")}
      >
        <Link to={`/drive/${spaceId}`}>{t("drive:back")}</Link>
      </PageHeading>
      {query.isError && <ErrorBanner error={query.error} />}
      <DataTable rows={query.data?.data.items ?? []} columns={columns} />
      {selected && name === undefined && (
        <NameDialog
          title={t("drive:restore")}
          initial={selected.name}
          closeOnSuccess={false}
          onClose={() => setSelected(undefined)}
          onSave={async (value) => {
            setName(value);
          }}
        />
      )}
      {selected && name !== undefined && (
        <MoveDialog
          spaceId={spaceId}
          identity={identity}
          onClose={() => {
            setSelected(undefined);
            setName(undefined);
          }}
          onSave={async (folderId) => {
            if (selected.kind === "folder")
              await driveFolderRestore(spaceId, selected.id, folderId, name);
            else await driveFileRestore(spaceId, selected.id, folderId, name);
            await client.invalidateQueries({ queryKey: ["drive", identity] });
          }}
        />
      )}
      {purging && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setPurging(undefined);
          }}
          title={t("drive:purge")}
          description={t("drive:confirmPurge")}
        >
          {purge.isError && <ErrorBanner error={purge.error} />}
          <div className="editor-footer">
            <Button
              variant="outline"
              type="button"
              onClick={() => setPurging(undefined)}
            >
              {t("drive:cancel")}
            </Button>
            <Button
              variant="destructive"
              type="button"
              disabled={purge.isPending}
              onClick={() => purge.mutate(purging.id)}
            >
              {t("drive:purge")}
            </Button>
          </div>
        </EditorDialog>
      )}
    </>
  );
}
