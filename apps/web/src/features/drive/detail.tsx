import { ArrowLeft, HardDrive, ChevronRight, Folder } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { SpaceNavigation } from "./space-navigation";
import { UploadStatus } from "./upload-progress";
import { DriveToolbar } from "./toolbar";
import { useCallback, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DriveItemsTable, type Item, type DriveDialog } from "./items-table";
import { useTranslation } from "react-i18next";
import { Link, useParams, useSearchParams } from "react-router";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import {
  ErrorBanner,
  Loading,
  PageHeading,
} from "@/features/administration/shared";
import {
  driveFiles,
  driveFolderPath,
  driveFolders,
  driveSpaces,
  driveSpaceUsage,
  driveUploadCancel,
} from "@/lib/capabilities";
import { ItemDialogs } from "./item-dialogs";
import { uploadDriveFile, type UploadProgress } from "./transfer";

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
  const [dialog, setDialog] = useState<DriveDialog | null>(null);
  const [progress, setProgress] = useState<UploadProgress>();
  const progressKey = useRef(uploadKey);
  const [transferError, setTransferError] = useState<unknown>();
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
  const filter = useCallback(
    (key: string, value: string) => {
      setParams(
        (old) => {
          const next = new URLSearchParams(old);
          if (value && value !== "all") next.set(key, value);
          else next.delete(key);
          return next;
        },
        { replace: true },
      );
    },
    [setParams],
  );
  if (!canUse)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (spaces.isPending || folders.isPending || files.isPending)
    return <Loading />;
  if (spaces.isError) return <ErrorBanner error={spaces.error} />;
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
        <Button variant="outline" render={<Link to="/drive" />}>
          <ArrowLeft />
          {t("drive:back")}
        </Button>
      </PageHeading>
      <section className="admin-panel mb-5 flex flex-wrap items-center gap-4 p-4">
        <span className="flex size-10 items-center justify-center rounded-lg bg-muted">
          <HardDrive aria-hidden="true" size={20} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="mb-1 flex flex-wrap gap-2">
            <Badge variant="secondary">
              {t(space.kind === "personal" ? "drive:personal" : "drive:shared")}
            </Badge>
            <Badge variant="outline">{t(`drive:roles.${space.role}`)}</Badge>
          </div>
          {usage.isError && <ErrorBanner error={usage.error} />}
          {usage.data && (
            <p className="text-sm text-muted-foreground">
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
        </div>
      </section>
      <SpaceNavigation space={space} current="files" />
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
      <nav className="drive-folder-path" aria-label={t("drive:root")}>
        <Button
          variant="link"
          type="button"
          onClick={() => filter("folder", "0")}
        >
          <Folder aria-hidden="true" size={16} />
          {t("drive:root")}
        </Button>
        {path.data?.data.items.map((entry) => (
          <Button
            variant="link"
            type="button"
            key={entry.id}
            onClick={() => filter("folder", String(entry.id))}
          >
            <ChevronRight aria-hidden="true" size={14} />
            {entry.name}
          </Button>
        ))}
      </nav>
      <DriveToolbar
        search={search}
        kind={kind}
        sort={sort}
        filter={filter}
        canEdit={canEdit}
        uploading={upload.isPending}
        resuming={Boolean(progress)}
        onCreate={() => setDialog({ action: "create" })}
        onUpload={(files) => {
          setTransferError(undefined);
          upload.mutate(files);
        }}
      />
      <UploadStatus
        progress={progress}
        busy={upload.isPending || cancelUpload.isPending}
        error={Boolean(transferError)}
        cancel={(id) => cancelUpload.mutate(id)}
        forget={() => {
          sessionStorage.removeItem(progressKey.current);
          setProgress(undefined);
          setTransferError(undefined);
        }}
      />
      <DriveItemsTable
        rows={rows}
        spaceId={spaceId}
        canEdit={canEdit}
        filter={filter}
        setDialog={setDialog}
        setTransferError={setTransferError}
      />
      <ItemDialogs
        dialog={dialog}
        setDialog={setDialog}
        spaceId={spaceId}
        folderId={folderId}
        identity={identity}
        refresh={refresh}
      />
    </>
  );
}
