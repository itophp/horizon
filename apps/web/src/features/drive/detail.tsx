import { useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, FolderOpen, Move, Pencil, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useParams, useSearchParams } from "react-router";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  ListSearch,
  Loading,
  PageHeading,
  RowActions,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  driveFileMove,
  driveFileRename,
  driveFileTrash,
  driveFiles,
  driveFolderCreate,
  driveFolderMove,
  driveFolderPath,
  driveFolderRename,
  driveFolderTrash,
  driveFolders,
  driveSpaces,
  driveSpaceUsage,
  driveUploadCancel,
  type DriveFile,
} from "@/lib/capabilities";
import { MoveDialog } from "./move-dialog";
import { NameDialog } from "./name-dialog";
import {
  downloadDriveFile,
  uploadDriveFile,
  type UploadProgress,
} from "./transfer";

type Item = {
  id: number;
  objectId: number;
  name: string;
  kind: "folder" | "file";
  file?: DriveFile;
};
type Dialog =
  { action: "rename" | "move" | "trash"; item: Item } | { action: "create" };

export function DriveDetailPage() {
  const { t, i18n } = useTranslation(["drive", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const { id } = useParams();
  const spaceId = Number(id);
  const [params, setParams] = useSearchParams();
  const folderId = Number(params.get("folder") ?? 0);
  const search = params.get("q") ?? "";
  const kind = params.get("type") ?? "all";
  const sort = params.get("sort") ?? "name";
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`;
  const uploadKey = `drive-upload:${identity}:${spaceId}:${folderId}`;
  const canUse =
    session.permissions.includes("drive:use") &&
    Number.isSafeInteger(spaceId) &&
    spaceId > 0 &&
    Number.isSafeInteger(folderId) &&
    folderId >= 0;
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [progress, setProgress] = useState<UploadProgress>();
  const progressKey = useRef(uploadKey);
  const [transferError, setTransferError] = useState<unknown>();
  const fileInput = useRef<HTMLInputElement>(null);
  const refresh = () =>
    client.invalidateQueries({ queryKey: ["drive", identity] });
  const spaces = useQuery({
    queryKey: ["drive", identity, "spaces"],
    queryFn: driveSpaces,
    enabled: canUse,
    retry: false,
  });
  const usage = useQuery({
    queryKey: ["drive", identity, spaceId, "usage"],
    queryFn: () => driveSpaceUsage(spaceId),
    enabled: canUse,
    retry: false,
  });
  const folders = useQuery({
    queryKey: ["drive", identity, spaceId, "folders", folderId],
    queryFn: () => driveFolders(spaceId, folderId),
    enabled: canUse,
    retry: false,
  });
  const files = useQuery({
    queryKey: ["drive", identity, spaceId, "files", folderId],
    queryFn: () => driveFiles(spaceId, folderId),
    enabled: canUse,
    retry: false,
  });
  const path = useQuery({
    queryKey: ["drive", identity, spaceId, "path", folderId],
    queryFn: () => driveFolderPath(spaceId, folderId),
    enabled: canUse && folderId > 0,
    retry: false,
  });
  const space = spaces.data?.data.items.find((item) => item.id === spaceId);
  const canEdit = space?.role === "admin" || space?.role === "editor";
  const upload = useMutation({
    mutationFn: async (selected: File[]) => {
      const savedId = Number(sessionStorage.getItem(uploadKey));
      let resumeId =
        Number.isSafeInteger(savedId) && savedId > 0 ? savedId : undefined;
      for (const file of selected) {
        if (file.size > 1 << 30) throw new Error("too_large");
        await uploadDriveFile(file, spaceId, folderId, resumeId, (next) => {
          progressKey.current = uploadKey;
          sessionStorage.setItem(uploadKey, String(next.uploadId));
          setProgress(next);
        });
        resumeId = undefined;
        sessionStorage.removeItem(uploadKey);
        setProgress(undefined);
      }
    },
    onSuccess: async () => {
      setTransferError(undefined);
      await refresh();
    },
    onError: setTransferError,
  });
  const discard = useMutation({
    mutationFn: async (item: Item) =>
      item.kind === "folder"
        ? driveFolderTrash(spaceId, item.objectId)
        : driveFileTrash(spaceId, item.objectId),
    onSuccess: async () => {
      setDialog(null);
      await refresh();
    },
  });
  const cancelUpload = useMutation({
    mutationFn: (uploadId: number) => driveUploadCancel(spaceId, uploadId),
    onSuccess: () => {
      sessionStorage.removeItem(progressKey.current);
      setProgress(undefined);
      setTransferError(undefined);
    },
    onError: setTransferError,
  });
  const rows = useMemo<Item[]>(() => {
    const values: Item[] = [
      ...(folders.data?.data.items ?? []).map((folder) => ({
        id: -folder.id,
        objectId: folder.id,
        name: folder.name,
        kind: "folder" as const,
      })),
      ...(files.data?.data.items ?? []).map((file) => ({
        id: file.id,
        objectId: file.id,
        name: file.name,
        kind: "file" as const,
        file,
      })),
    ];
    return values
      .filter(
        (item) =>
          (kind === "all" || item.kind === kind) &&
          item.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()),
      )
      .sort((a, b) => {
        if (a.kind !== b.kind) return a.kind === "folder" ? -1 : 1;
        if (sort === "size" && a.file && b.file && a.file.size !== b.file.size)
          return b.file.size - a.file.size;
        if (
          sort === "uploaded" &&
          a.file &&
          b.file &&
          a.file.createdAt !== b.file.createdAt
        )
          return b.file.createdAt.localeCompare(a.file.createdAt);
        return a.name.localeCompare(b.name, i18n.language, { numeric: true });
      });
  }, [folders.data, files.data, kind, search, sort, i18n.language]);
  function filter(key: string, value: string) {
    setParams(
      (old) => {
        const next = new URLSearchParams(old);
        if (value && value !== "all") next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  }
  const columns: AdminColumn<Item>[] = [
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
          item.name
        ),
    },
    {
      id: "kind",
      header: t("drive:kind"),
      cell: (item) => t(item.kind === "folder" ? "drive:folder" : "drive:file"),
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
  ];
  if (!canUse)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (spaces.isPending || folders.isPending || files.isPending)
    return <Loading />;
  if (!space)
    return <div className="admin-empty">{t("admin:errors.not_found")}</div>;
  const errorCode = transferError instanceof Error ? transferError.message : "";
  const transferMessage =
    errorCode === "too_large"
      ? t("drive:tooLarge")
      : errorCode === "upload_mismatch"
        ? t("drive:uploadMismatch")
        : errorCode === "streaming_download_unavailable"
          ? t("drive:downloadUnavailable")
          : errorCode === "download_incomplete"
            ? t("drive:downloadIncomplete")
            : "";
  return (
    <>
      <PageHeading
        title={space.kind === "personal" ? t("drive:personal") : space.name}
        subtitle={t("drive:description")}
      >
        <Link to="/drive">{t("drive:back")}</Link>
        {space.role === "admin" && space.kind === "shared" && (
          <Link to={`/drive/${spaceId}/members`}>{t("drive:members")}</Link>
        )}
        {canEdit && (
          <Link to={`/drive/${spaceId}/trash`}>{t("drive:trashTitle")}</Link>
        )}
      </PageHeading>
      {usage.isError && <ErrorBanner error={usage.error} />}
      {usage.data && (
        <p>
          {t(
            usage.data.data.quotaBytes
              ? "drive:capacityLimited"
              : "drive:capacityUnlimited",
            {
              used: new Intl.NumberFormat(i18n.language).format(
                usage.data.data.usedBytes,
              ),
              pending: new Intl.NumberFormat(i18n.language).format(
                usage.data.data.pendingBytes,
              ),
              quota: new Intl.NumberFormat(i18n.language).format(
                usage.data.data.quotaBytes,
              ),
            },
          )}
        </p>
      )}
      {folders.isError && <ErrorBanner error={folders.error} />}
      {files.isError && <ErrorBanner error={files.error} />}
      {path.isError && <ErrorBanner error={path.error} />}
      {transferMessage ? (
        <div role="alert" className="admin-error">
          {transferMessage}
        </div>
      ) : (
        Boolean(transferError) && <ErrorBanner error={transferError} />
      )}
      <nav aria-label={t("drive:root")}>
        <Button
          variant="link"
          type="button"
          onClick={() => filter("folder", "0")}
        >
          {t("drive:root")}
        </Button>
        {path.data?.data.items.map((entry) => (
          <Button
            variant="link"
            type="button"
            key={entry.id}
            onClick={() => filter("folder", String(entry.id))}
          >
            {entry.name}
          </Button>
        ))}
      </nav>
      <div className="standalone-toolbar">
        <ListSearch value={search} onChange={(value) => filter("q", value)} />
        <select
          className="toolbar-select"
          aria-label={t("drive:kind")}
          value={kind}
          onChange={(event) => filter("type", event.target.value)}
        >
          <option value="all">{t("admin:actions.all")}</option>
          <option value="folder">{t("drive:folder")}</option>
          <option value="file">{t("drive:file")}</option>
        </select>
        <select
          className="toolbar-select"
          aria-label={t("drive:sort")}
          value={sort}
          onChange={(event) => filter("sort", event.target.value)}
        >
          <option value="name">{t("drive:sortName")}</option>
          <option value="size">{t("drive:sortSize")}</option>
          <option value="uploaded">{t("drive:sortUploaded")}</option>
        </select>
        {canEdit && (
          <div className="toolbar-right">
            <Button
              type="button"
              onClick={() => setDialog({ action: "create" })}
            >
              {t("drive:createFolder")}
            </Button>
            <input
              ref={fileInput}
              type="file"
              multiple
              hidden
              aria-label={t("drive:upload")}
              onChange={(event) => {
                const selected = Array.from(event.target.files ?? []);
                event.target.value = "";
                if (selected.length) {
                  setTransferError(undefined);
                  upload.mutate(selected);
                }
              }}
            />
            <Button
              type="button"
              disabled={upload.isPending}
              onClick={() => fileInput.current?.click()}
            >
              {progress ? t("drive:selectSameFile") : t("drive:upload")}
            </Button>
          </div>
        )}
      </div>
      {progress && (
        <div>
          <p role="status">
            {t("drive:uploadProgress", {
              uploaded: progress.uploaded,
              total: progress.total,
            })}
          </p>
          <Button
            type="button"
            variant="outline"
            disabled={upload.isPending || cancelUpload.isPending}
            onClick={() => cancelUpload.mutate(progress.uploadId)}
          >
            {t("drive:cancel")}
          </Button>
          {Boolean(transferError) && (
            <Button
              type="button"
              variant="ghost"
              disabled={upload.isPending || cancelUpload.isPending}
              onClick={() => {
                sessionStorage.removeItem(progressKey.current);
                setProgress(undefined);
                setTransferError(undefined);
              }}
            >
              {t("drive:forgetUpload")}
            </Button>
          )}
          {Boolean(transferError) && <p>{t("drive:forgetUploadHint")}</p>}
        </div>
      )}
      <DataTable rows={rows} columns={columns} />
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
