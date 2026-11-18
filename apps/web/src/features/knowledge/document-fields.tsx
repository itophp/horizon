import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

export function KnowledgeDocumentFields({
  title,
  content,
  disabled,
  onTitleChange,
  onContentChange,
}: {
  title: string;
  content: string;
  disabled: boolean;
  onTitleChange: (value: string) => void;
  onContentChange: (value: string) => void;
}) {
  const { t } = useTranslation("knowledge");
  return (
    <>
      <label className="knowledge-field">
        <span>{t("create.name")}</span>
        <Input
          value={title}
          maxLength={200}
          disabled={disabled}
          placeholder={t("create.namePlaceholder")}
          onChange={(event) => onTitleChange(event.target.value)}
        />
      </label>
      <label className="knowledge-field">
        <span>{t("create.content")}</span>
        <Textarea
          value={content}
          rows={10}
          maxLength={100000}
          disabled={disabled}
          placeholder={t("create.contentPlaceholder")}
          onChange={(event) => onContentChange(event.target.value)}
        />
      </label>
    </>
  );
}
