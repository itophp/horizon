import { useId } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUp, Square } from "lucide-react";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupTextarea,
  InputGroupText,
} from "@/components/ui/input-group";

export function PromptInput({
  draft,
  onChange,
  busy,
  blocked,
  onSend,
  onStop,
}: {
  draft: string;
  onChange: (value: string) => void;
  busy: boolean;
  blocked: boolean;
  onSend: (value: string) => void;
  onStop: () => void;
}) {
  const { t } = useTranslation("ai");
  const id = useId();
  return (
    <form
      className="border-t p-4"
      onSubmit={(event) => {
        event.preventDefault();
        onSend(draft);
      }}
    >
      <label htmlFor={id} className="sr-only">
        {t("workbench.ask")}
      </label>
      <InputGroup className="rounded-xl">
        <InputGroupTextarea
          id={id}
          value={draft}
          onChange={(event) => onChange(event.target.value)}
          rows={2}
          maxLength={2000}
          placeholder={t("workbench.placeholder")}
          disabled={blocked}
          aria-describedby={`${id}-hint`}
          onKeyDown={(event) => {
            if (
              event.key === "Enter" &&
              !event.shiftKey &&
              !event.nativeEvent.isComposing &&
              event.keyCode !== 229
            ) {
              event.preventDefault();
              if (!busy) onSend(draft);
            }
          }}
        />
        <InputGroupAddon align="block-end" className="justify-between">
          <InputGroupText id={`${id}-hint`} className="text-xs">
            {t("workbench.inputHint")}
          </InputGroupText>
          {busy ? (
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
              disabled={blocked || !draft.trim()}
              aria-label={t("workbench.send")}
            >
              <ArrowUp />
            </InputGroupButton>
          )}
        </InputGroupAddon>
      </InputGroup>
    </form>
  );
}
