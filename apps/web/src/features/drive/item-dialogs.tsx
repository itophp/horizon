import { useTranslation } from "react-i18next";
import { useMutation } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";
import {
  driveFileMove,
  driveFileRename,
  driveFileTrash,
  driveFolderCreate,
  driveFolderMove,
  driveFolderRename,
  driveFolderTrash,
} from "@/lib/capabilities";
import { MoveDialog } from "./move-dialog";
import { NameDialog } from "./name-dialog";
import { type DriveDialog } from "./items-table";
export function ItemDialogs({
  dialog,
  setDialog,
  spaceId,
  folderId,
  identity,
  refresh,
}: {
  dialog: DriveDialog | null;
  setDialog: (value: DriveDialog | null) => void;
  spaceId: number;
  folderId: number;
  identity: string;
  refresh: () => Promise<unknown>;
}) {
  const { t } = useTranslation("drive");
  const discard = useMutation({
    mutationFn: async (
      item: Extract<DriveDialog, { item: unknown }>["item"],
    ) =>
      item.kind === "folder"
        ? driveFolderTrash(spaceId, item.objectId)
        : driveFileTrash(spaceId, item.objectId),
    onSuccess: async () => {
      setDialog(null);
      await refresh();
    },
  });
  return (
    <>
      {" "}
      {dialog?.action === "create" && (
        <NameDialog
          title={t("drive:createFolder")}
          onClose={() => setDialog(null)}
          onSave={async (name) => {
            await driveFolderCreate(spaceId, folderId, name);
            await refresh();
          }}
        />
      )}
      {dialog?.action === "rename" && (
        <NameDialog
          title={t("drive:rename")}
          initial={dialog.item.name}
          onClose={() => setDialog(null)}
          onSave={async (name) => {
            if (dialog.item.kind === "folder")
              await driveFolderRename(spaceId, dialog.item.objectId, name);
            else await driveFileRename(spaceId, dialog.item.objectId, name);
            await refresh();
          }}
        />
      )}
      {dialog?.action === "move" && (
        <MoveDialog
          spaceId={spaceId}
          identity={identity}
          onClose={() => setDialog(null)}
          onSave={async (destination) => {
            if (dialog.item.kind === "folder")
              await driveFolderMove(spaceId, dialog.item.objectId, destination);
            else
              await driveFileMove(spaceId, dialog.item.objectId, destination);
            await refresh();
          }}
        />
      )}
      {dialog?.action === "trash" && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
          title={t("drive:trash")}
          description={t("drive:confirmTrash")}
        >
          {discard.isError && <ErrorBanner error={discard.error} />}
          <div className="editor-footer">
            <Button
              type="button"
              variant="outline"
              onClick={() => setDialog(null)}
            >
              {t("drive:cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={discard.isPending}
              onClick={() => discard.mutate(dialog.item)}
            >
              {t("drive:trash")}
            </Button>
          </div>
        </EditorDialog>
      )}
    </>
  );
}
