import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Loader2, Save } from "lucide-react";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/api/client/request";
import type { KnowledgeReadOutput } from "@/api/generated/capability-knowledge";
import { Button } from "@/components/ui/button";
import { EditorDialog } from "@/features/administration/shared";
import { knowledgeUpdate } from "@/lib/capabilities";
import { KnowledgeDocumentFields } from "./document-fields";
import { knowledgeEmbeddingError } from "./errors";

export function EditKnowledgeDocument({
  document,
  onClose,
  onSaved,
}: {
  document: KnowledgeReadOutput;
  onClose: (reload: boolean) => void;
  onSaved: () => Promise<void>;
}) {
  const { t } = useTranslation("knowledge");
  const [original] = useState(document);
  const [title, setTitle] = useState(original.title);
  const [content, setContent] = useState(original.content);
  const save = useMutation({
    mutationFn: knowledgeUpdate,
    onSuccess: onSaved,
    gcTime: 0,
  });
  const code = save.error instanceof ApiError ? save.error.code : undefined;
  const needsReload = code === "conflict" || code === "not_found";
  const unchanged =
    title.trim() === original.title && content.trim() === original.content;
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) onClose(needsReload);
      }}
      title={t("edit.title")}
      description={t("edit.description")}
    >
      {save.isError && (
        <p className="knowledge-error" role="alert">
          {t(
            knowledgeEmbeddingError(save.error) ??
              (code === "conflict"
                ? "edit.conflict"
                : code === "not_found"
                  ? "detail.notFound"
                  : code === "forbidden"
                    ? "errors.forbidden"
                    : "edit.failed"),
          )}
        </p>
      )}
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (
            save.isPending ||
            needsReload ||
            unchanged ||
            !title.trim() ||
            !content.trim()
          )
            return;
          save.mutate({
            id: original.id,
            expectedVersion: original.version ?? "",
            title: title.trim(),
            content: content.trim(),
          });
        }}
      >
        <KnowledgeDocumentFields
          title={title}
          content={content}
          disabled={save.isPending}
          onTitleChange={setTitle}
          onContentChange={setContent}
        />
        <Button
          type="submit"
          disabled={
            save.isPending ||
            needsReload ||
            unchanged ||
            !title.trim() ||
            !content.trim()
          }
        >
          {save.isPending ? (
            <Loader2 aria-hidden="true" className="animate-spin" />
          ) : (
            <Save aria-hidden="true" />
          )}
          {save.isPending ? t("edit.saving") : t("common:actions.save")}
        </Button>
      </form>
    </EditorDialog>
  );
}
