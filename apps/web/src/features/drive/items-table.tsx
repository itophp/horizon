import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Download, File, FolderOpen, Move, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DataTable,
  RowActions,
  type AdminColumn,
} from "@/features/administration/shared";
import { type DriveFile } from "@/lib/capabilities";
import { downloadDriveFile } from "./transfer";
export type Item = {
  id: number;
  objectId: number;
  name: string;
  kind: "folder" | "file";
  file?: DriveFile;
};
export type DriveDialog =
  { action: "rename" | "move" | "trash"; item: Item } | { action: "create" };
export function DriveItemsTable({
  rows,
  spaceId,
  canEdit,
  filter,
  setDialog,
  setTransferError,
}: {
  rows: Item[];
  spaceId: number;
  canEdit: boolean;
  filter: (key: string, value: string) => void;
  setDialog: (dialog: DriveDialog) => void;
  setTransferError: (error: unknown) => void;
}) {
  const { t, i18n } = useTranslation(["drive", "admin"]);
  const columns = useMemo<AdminColumn<Item>[]>(
    () => [
      {
        id: "name",
        header: t("drive:name"),
        cell: (item) =>
          item.kind === "folder" ? (
            <Button
              type="button"
              variant="link"
              onClick={() => filter("folder", String(item.objectId))}
            >
              <FolderOpen size={14} />
              {item.name}
            </Button>
          ) : (
            <span className="flex items-center gap-2">
              <File
                aria-hidden="true"
                size={16}
                className="shrink-0 text-muted-foreground"
              />
              <span className="break-all">{item.name}</span>
            </span>
          ),
      },
      {
        id: "kind",
        header: t("drive:kind"),
        cell: (item) =>
          t(item.kind === "folder" ? "drive:folder" : "drive:file"),
      },
      {
        id: "size",
        header: t("drive:size"),
        cell: (item) =>
          item.file
            ? t("drive:sizeBytes", {
                value: new Intl.NumberFormat(i18n.language).format(
                  item.file.size,
                ),
              })
            : "—",
      },
      {
        id: "actions",
        header: t("drive:actions"),
        cell: (item) => (
          <RowActions
            actions={[
              ...(item.file
                ? [
                    {
                      label: t("drive:download"),
                      icon: <Download size={14} />,
                      onClick: () => {
                        void downloadDriveFile(spaceId, item.file!).catch(
                          setTransferError,
                        );
                      },
                    },
                  ]
                : []),
              ...(canEdit
                ? [
                    {
                      label: t("drive:rename"),
                      icon: <Pencil size={14} />,
                      onClick: () => setDialog({ action: "rename", item }),
                    },
                    {
                      label: t("drive:move"),
                      icon: <Move size={14} />,
                      onClick: () => setDialog({ action: "move", item }),
                    },
                    {
                      label: t("drive:trash"),
                      icon: <Trash2 size={14} />,
                      danger: true,
                      onClick: () => setDialog({ action: "trash", item }),
                    },
                  ]
                : []),
            ]}
          />
        ),
      },
    ],
    [t, i18n.language, spaceId, canEdit, filter, setDialog, setTransferError],
  );
  return <DataTable rows={rows} columns={columns} />;
}
