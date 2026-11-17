import { useTranslation } from "react-i18next";
import { ArrowUp, Square } from "lucide-react";
import {
  AIComposer,
  invalidAIAttachment,
} from "@/components/shared/ai-composer";
import { InputGroupButton } from "@/components/ui/input-group";

export function PromptInput({
  draft,
  onChange,
  file,
  onFileChange,
  busy,
  blocked,
  onSend,
  onStop,
}: {
  draft: string;
  onChange: (value: string) => void;
  file?: File;
  onFileChange: (file: File | undefined) => void;
  busy: boolean;
  blocked: boolean;
  onSend: (value: string) => void;
  onStop: () => void;
}) {
  const { t } = useTranslation("ai");
  const submitDisabled =
    busy || blocked || (!draft.trim() && !file) || invalidAIAttachment(file);
  const submit = () => {
    if (!submitDisabled) onSend(draft);
  };
  return (
    <form
      className="ai-session-composer shrink-0 border-t p-3"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <AIComposer
        value={draft}
        onChange={onChange}
        label={t("workbench.ask")}
        placeholder={t("workbench.placeholder")}
        maxLength={2000}
        file={file}
        onFileChange={onFileChange}
        disabled={blocked}
        busy={busy}
        hint={t("workbench.inputHint")}
        onSubmit={submit}
        submitDisabled={submitDisabled}
        action={
          busy ? (
            <InputGroupButton
              size="icon-sm"
              variant="outline"
              onClick={onStop}
              aria-label={t("workbench.stop")}
            >
              <Square className="size-3.5" />
            </InputGroupButton>
          ) : (
            <InputGroupButton
              type="submit"
              size="icon-sm"
              variant="default"
              disabled={submitDisabled}
              aria-label={t("workbench.send")}
            >
              <ArrowUp />
            </InputGroupButton>
          )
        }
      />
    </form>
  );
}
