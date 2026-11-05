import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { type UploadProgress } from "./transfer";
export function UploadStatus({
  progress,
  busy,
  error,
  cancel,
  forget,
}: {
  progress?: UploadProgress;
  busy: boolean;
  error: boolean;
  cancel: (id: number) => void;
  forget: () => void;
}) {
  const { t } = useTranslation("drive");
  return (
    <>
      {progress && (
        <div className="mb-4 rounded-xl border border-border bg-muted/30 p-4">
          <p role="status">
            {t("drive:uploadProgress", {
              uploaded: progress.uploaded,
              total: progress.total,
            })}
          </p>
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => cancel(progress.uploadId)}
          >
            {t("drive:cancel")}
          </Button>
          {error && (
            <Button
              type="button"
              variant="ghost"
              disabled={busy}
              onClick={forget}
            >
              {t("drive:forgetUpload")}
            </Button>
          )}
          {error && <p>{t("drive:forgetUploadHint")}</p>}
        </div>
      )}
    </>
  );
}
