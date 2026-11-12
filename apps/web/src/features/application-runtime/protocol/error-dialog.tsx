import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { EditorDialog, ErrorBanner } from "@/features/administration/shared";

export function ProtocolErrorDialog({
  error,
  onClose,
  onRetry,
}: {
  error: unknown;
  onClose: () => void;
  onRetry?: () => void;
}) {
  const { t } = useTranslation(["runtime", "admin"]);
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t("runtime:protocol.eventFailedTitle")}
      description={t("runtime:protocol.eventFailed")}
    >
      <ErrorBanner error={error} onRetry={onRetry} />
      <div className="editor-footer">
        <Button type="button" variant="outline" onClick={onClose}>
          {t("admin:actions.close")}
        </Button>
      </div>
    </EditorDialog>
  );
}
