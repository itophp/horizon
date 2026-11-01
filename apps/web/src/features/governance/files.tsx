import { useState } from "react";
import { useParams } from "react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Upload, Download, Trash2 } from "lucide-react";
import {
  filesList,
  filesUpload,
  filesContent,
  filesRemove,
} from "@/lib/capabilities";
import type { ManagedFile } from "@/api/types";
import { ApiError } from "@/api/client/request";
import { useSession } from "@/features/administration/auth";
import {
  PageHeading,
  DataTable,
  EditorDialog,
  ShortDate,
  ErrorBanner,
  Loading,
  type AdminColumn,
} from "@/features/administration/shared";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useGovKey, useGovTask, NoAccess, downloadContent } from "./shared";
export async function downloadFile(id: number) {
  const r = await filesContent(id);
  if (r.status === 200) {
    const bytes = Uint8Array.from(atob(r.data.content), (c) => c.charCodeAt(0));
    downloadContent(r.data.name, bytes, r.data.mime);
  }
}
export function FilesPage() {
  const { t, i18n } = useTranslation("governance");
  const session = useSession();
  const task = useGovTask();
  const [upload, setUpload] = useState(false);
  const [remove, setRemove] = useState<ManagedFile | null>(null);
  const q = useQuery({
    queryKey: useGovKey("files"),
    queryFn: () => filesList(),
    enabled: session.permissions.includes("files:read"),
    retry: false,
  });
  const columns: AdminColumn<ManagedFile>[] = [
    {
      id: "name",
      header: t("filename"),
      cell: (r) => <strong>{r.name}</strong>,
    },
    { id: "purpose", header: t("purpose"), cell: (r) => t(r.purpose) },
    {
      id: "size",
      header: t("size"),
      cell: (r) =>
        new Intl.NumberFormat(i18n.resolvedLanguage, {
          style: "unit",
          unit: "kilobyte",
          maximumFractionDigits: 1,
        }).format(r.size / 1024),
    },
    {
      id: "profile",
      header: t("storageVersion"),
      cell: (r) => r.storageVersion,
    },
    {
      id: "created",
      header: t("history"),
      cell: (r) => <ShortDate value={r.createdAt} />,
    },
    {
      id: "actions",
      header: t("select"),
      cell: (r) => (
        <div className="heading-actions">
          <Button
            variant="ghost"
            aria-label={t("download")}
            onClick={() => task.mutate(() => downloadFile(r.id))}
          >
            <Download size={16} />
          </Button>
          {session.permissions.includes("files:delete") && (
            <Button
              variant="ghost"
              aria-label={t("remove")}
              onClick={() => setRemove(r)}
            >
              <Trash2 size={16} />
            </Button>
          )}
        </div>
      ),
    },
  ];
  if (!session.permissions.includes("files:read")) return <NoAccess />;
  return (
    <>
      <PageHeading title={t("files")} subtitle={t("filesHint")}>
        {session.permissions.includes("files:create") && (
          <Button onClick={() => setUpload(true)}>
            <Upload size={16} />
            {t("upload")}
          </Button>
        )}
      </PageHeading>
      {task.error && <ErrorBanner error={task.error} />}{" "}
      {q.isPending ? (
        <Loading />
      ) : q.error ? (
        <ErrorBanner error={q.error} />
      ) : (
        <section className="admin-panel entity-card">
          <DataTable
            rows={q.data?.status === 200 ? q.data.data : []}
            columns={columns}
          />
        </section>
      )}
      {upload && <UploadDialog onClose={() => setUpload(false)} />}{" "}
      {remove && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setRemove(null);
          }}
          title={t("confirmRemove")}
          description={remove.name}
        >
          <div className="form-actions">
            <Button variant="outline" onClick={() => setRemove(null)}>
              {t("cancel")}
            </Button>
            <Button
              disabled={task.isPending}
              onClick={() =>
                task.mutate(async () => {
                  await filesRemove(remove.id);
                  setRemove(null);
                })
              }
            >
              {t("remove")}
            </Button>
          </div>
        </EditorDialog>
      )}
    </>
  );
}
function UploadDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation("governance");
  const task = useGovTask();
  const [file, setFile] = useState<File | null>(null);
  const form = useForm({
    resolver: zodResolver(
      z.object({ purpose: z.enum(["attachment", "logo"]) }),
    ),
    defaultValues: { purpose: "attachment" as "attachment" | "logo" },
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open && !task.isPending) onClose();
      }}
      title={t("upload")}
      description={t("uploadHint")}
    >
      <form
        className="admin-form"
        onSubmit={form.handleSubmit((data) =>
          task.mutate(async () => {
            if (!file) throw new ApiError("invalid", 400);
            if (file.size > (data.purpose === "logo" ? 2 : 20) * 1024 * 1024)
              throw new ApiError("invalid", 400);
            const raw = new Uint8Array(await file.arrayBuffer());
            let binary = "";
            for (let offset = 0; offset < raw.length; offset += 8192)
              binary += String.fromCharCode(
                ...raw.subarray(offset, offset + 8192),
              );
            await filesUpload({
              name: file.name,
              purpose: data.purpose,
              content: btoa(binary),
            });
            onClose();
          }),
        )}
      >
        <label>
          <span>{t("purpose")}</span>
          <select {...form.register("purpose")}>
            <option value="attachment">{t("attachment")}</option>
            <option value="logo">{t("logo")}</option>
          </select>
        </label>
        <label>
          <span>{t("filename")}</span>
          <Input
            type="file"
            accept=".png,.jpg,.jpeg,.webp,.pdf,.txt,.csv,.docx,.xlsx"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </label>
        {task.error && <ErrorBanner error={task.error} />}
        <div className="form-actions">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" disabled={task.isPending || !file}>
            {task.isPending ? t("busy") : t("upload")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
export function FileDownloadPage() {
  const { t } = useTranslation("governance");
  const { id } = useParams();
  const task = useGovTask();
  return (
    <>
      <PageHeading title={t("attachment")} subtitle={t("downloadHint")} />
      <Button
        onClick={() => task.mutate(() => downloadFile(Number(id)))}
        disabled={task.isPending}
      >
        {t("download")}
      </Button>
      {task.error && <ErrorBanner error={task.error} />}
    </>
  );
}
