import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  EditorDialog,
  ErrorBanner,
  Loading,
} from "@/features/administration/shared";
import { driveFolderPath, driveFolders } from "@/lib/capabilities";

export function MoveDialog({
  spaceId,
  identity,
  onClose,
  onSave,
}: {
  spaceId: number;
  identity: string;
  onClose: () => void;
  onSave: (folderId: number) => Promise<unknown>;
}) {
  const { t } = useTranslation("drive");
  const [folderId, setFolderId] = useState(0);
  const folders = useQuery({
    queryKey: ["drive", identity, spaceId, "folders", folderId],
    queryFn: () => driveFolders(spaceId, folderId),
    retry: false,
  });
  const path = useQuery({
    queryKey: ["drive", identity, spaceId, "path", folderId],
    queryFn: () => driveFolderPath(spaceId, folderId),
    enabled: folderId > 0,
    retry: false,
  });
  const mutation = useMutation({ mutationFn: onSave, onSuccess: onClose });
  const name = folderId
    ? (path.data?.data.items.at(-1)?.name ?? "")
    : t("root");
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t("chooseDestination")}
      description={t("editDescription")}
    >
      <nav aria-label={t("chooseDestination")}>
        <Button variant="link" type="button" onClick={() => setFolderId(0)}>
          {t("root")}
        </Button>
        {path.data?.data.items.map((item) => (
          <Button
            variant="link"
            type="button"
            key={item.id}
            onClick={() => setFolderId(item.id)}
          >
            {item.name}
          </Button>
        ))}
      </nav>
      <p>{t("currentDestination", { name })}</p>
      {folders.isPending ? (
        <Loading />
      ) : (
        folders.data?.data.items.map((folder) => (
          <div key={folder.id}>
            <Button
              variant="ghost"
              type="button"
              onClick={() => setFolderId(folder.id)}
            >
              {folder.name}
            </Button>
          </div>
        ))
      )}
      {folders.isError && <ErrorBanner error={folders.error} />}
      {path.isError && <ErrorBanner error={path.error} />}
      {mutation.isError && <ErrorBanner error={mutation.error} />}
      <div className="editor-footer">
        <Button type="button" variant="outline" onClick={onClose}>
          {t("cancel")}
        </Button>
        <Button
          type="button"
          disabled={mutation.isPending || (folderId > 0 && !path.data)}
          onClick={() => mutation.mutate(folderId)}
        >
          {t("move")}
        </Button>
      </div>
    </EditorDialog>
  );
}
