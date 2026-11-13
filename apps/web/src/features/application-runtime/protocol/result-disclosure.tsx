import { useState } from "react";
import { useTranslation } from "react-i18next";
import { EditorDialog } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import type { EventBinding } from "./types";

export type ResultDisclosure = {
  scope: string;
  labels: NonNullable<EventBinding["disclosure"]>;
  value: string;
};

export function ResultDisclosureDialog({
  disclosure,
  onClose,
}: {
  disclosure: ResultDisclosure;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [copy, setCopy] = useState<"success" | "error">();
  const { labels, value } = disclosure;
  return (
    <EditorDialog
      open
      title={t(labels.titleKey)}
      description=""
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <code className="break-all">{value}</code>
      <div className="form-actions">
        <Button
          type="button"
          variant="outline"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(value);
              setCopy("success");
            } catch {
              setCopy("error");
            }
          }}
        >
          {t(labels.copyLabelKey)}
        </Button>
        <Button type="button" onClick={onClose}>
          {t(labels.dismissKey)}
        </Button>
      </div>
      {copy && (
        <p role={copy === "success" ? "status" : "alert"}>
          {t(copy === "success" ? labels.copySuccessKey : labels.copyErrorKey)}
        </p>
      )}
    </EditorDialog>
  );
}
