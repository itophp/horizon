import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import {
  runtimeAISuggestionsSchema,
  type RuntimeAIFieldSuggestion,
  type RuntimeAIFormAssistant,
} from "@/api/generated/runtime";
import { Button } from "@/components/ui/button";
import {
  AIComposer,
  invalidAIAttachment,
} from "@/components/shared/ai-composer";
import { InputGroupButton } from "@/components/ui/input-group";
import {
  attachmentInvalid,
  modelImageUnsupported,
  quotaExhausted,
} from "./ai-components";
import { readFileBase64 } from "@/components/shared/read-file";

export function AIFormAssistant({
  spec,
  draftSource,
  request,
  current,
  currentValue,
  label,
  canApply,
  apply,
  disabled,
}: {
  spec: RuntimeAIFormAssistant;
  draftSource?: {
    label: string;
    value: string;
    maxLength: number;
    onChange: (value: string) => void;
  };
  request: (source: Record<string, unknown>) => Promise<unknown>;
  currentValue: (field: string) => unknown;
  current: (field: string) => unknown;
  label: (field: string) => string;
  canApply: (suggestion: RuntimeAIFieldSuggestion) => boolean;
  apply: (suggestion: RuntimeAIFieldSuggestion) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation("runtime");
  const sourceId = useId();
  const [expanded, setExpanded] = useState(spec.defaultExpanded ?? false);
  const [sourceText, setSourceText] = useState("");
  const [file, setFile] = useState<File>();
  const mounted = useRef(false);
  const latestCanApply = useRef(canApply);
  useEffect(() => {
    latestCanApply.current = canApply;
  }, [canApply]);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const [state, setState] = useState<
    | "idle"
    | "running"
    | "failed"
    | "attachmentInvalid"
    | "pdfUnavailable"
    | "imageUnsupported"
    | "quotaExhausted"
    | "succeeded"
    | "applied"
  >("idle");
  const [items, setItems] = useState<RuntimeAIFieldSuggestion[]>([]);
  const running = state === "running";
  const hasInput =
    sourceText.trim() !== "" ||
    Boolean(file) ||
    spec.inputFields.some((name) => {
      const value = current(name);
      return (
        value !== undefined && value !== null && String(value).trim() !== ""
      );
    });
  async function generate() {
    if (disabled || running || !hasInput || invalidAIAttachment(file)) return;
    const original = new Map(
      spec.targetFields.map((name) => [name, currentValue(name)]),
    );
    setState("running");
    setItems([]);
    try {
      const source: Record<string, unknown> = {};
      if (!draftSource && sourceText.trim())
        source.sourceText = sourceText.trim();
      if (file) {
        source.attachment = {
          name: file.name,
          content: await readFileBase64(file),
        };
      }
      if (!mounted.current) return;
      const parsed = runtimeAISuggestionsSchema.safeParse(
        await request(source),
      );
      if (!mounted.current) return;
      if (!parsed.success) {
        setState("failed");
        return;
      }
      const suggestions = parsed.data.suggestions.filter((item) =>
        spec.targetFields.includes(item.field),
      );
      if (
        suggestions.length > 0 &&
        suggestions.every(
          (item) =>
            latestCanApply.current(item) &&
            Object.is(currentValue(item.field), original.get(item.field)),
        )
      ) {
        suggestions.forEach(apply);
        setState("applied");
      } else {
        setItems(suggestions);
        setState("succeeded");
      }
    } catch (error) {
      if (!mounted.current) return;
      const code = error instanceof Error ? error.message : "";
      setState(
        code === quotaExhausted
          ? "quotaExhausted"
          : code === attachmentInvalid
            ? "attachmentInvalid"
            : code === "ai_pdf_unavailable"
              ? "pdfUnavailable"
              : code === modelImageUnsupported
                ? "imageUnsupported"
                : "failed",
      );
    }
  }
  const submitDisabled =
    disabled || running || !hasInput || invalidAIAttachment(file);
  const generateButton = (
    <InputGroupButton
      type="button"
      variant="outline"
      className="ai-action"
      size="sm"
      disabled={submitDisabled}
      onClick={() => void generate()}
    >
      <Sparkles aria-hidden="true" size={14} />
      {t("ai.generate")}
    </InputGroupButton>
  );
  return (
    <div className="ai-form-assistant space-y-2">
      <Button
        type="button"
        variant="outline"
        size="icon-sm"
        className="ai-action"
        aria-label={t(spec.labelKey)}
        title={t(spec.labelKey)}
        aria-expanded={expanded}
        aria-controls={sourceId + "-panel"}
        onClick={() => setExpanded(!expanded)}
      >
        <Sparkles aria-hidden="true" size={14} />
      </Button>
      <div id={sourceId + "-panel"} hidden={!expanded}>
        <AIComposer
          label={draftSource?.label ?? t("ai.sourceText")}
          placeholder={draftSource?.label ?? t("ai.sourceText")}
          value={draftSource?.value ?? sourceText}
          onChange={draftSource?.onChange ?? setSourceText}
          maxLength={draftSource?.maxLength || 2000}
          file={file}
          onFileChange={(selected) => {
            setFile(selected);
            setState("idle");
          }}
          onSubmit={() => void generate()}
          submitDisabled={submitDisabled}
          disabled={disabled || running}
          busy={running}
          hint={t("ai.generateHint")}
          action={generateButton}
        />
      </div>
      {state === "running" && <p role="status">{t("ai.running")}</p>}
      {state === "failed" && <p role="status">{t("ai.failed")}</p>}
      {state === "attachmentInvalid" && (
        <p role="status">{t("ai.attachmentInvalid")}</p>
      )}
      {state === "imageUnsupported" && (
        <p role="status">
          {t("governance:aiSettings.errors.ai_model_image_unsupported")}
        </p>
      )}
      {state === "pdfUnavailable" && (
        <p role="status">{t("ai.pdfUnavailable")}</p>
      )}
      {state === "applied" && <p role="status">{t("ai.generated")}</p>}
      {state === "quotaExhausted" && (
        <p role="status">{t("ai.quotaExhausted")}</p>
      )}
      {state === "succeeded" && !items.length && (
        <p role="status">{t("ai.noSuggestions")}</p>
      )}
      {items.map((item) => (
        <div key={item.field} className="ai-form-suggestion">
          <p className="font-medium">{label(item.field)}</p>
          <p className="whitespace-pre-wrap text-muted-foreground">
            {t("ai.current", { value: String(current(item.field) ?? "") })}
          </p>
          <p className="whitespace-pre-wrap ai-suggestion-value">
            {t("ai.suggestion", { value: item.value })}
          </p>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={disabled || !canApply(item)}
            onClick={() => {
              apply(item);
              setItems((previous) =>
                previous.filter((entry) => entry.field !== item.field),
              );
              setState("idle");
            }}
          >
            {t("ai.apply")}
          </Button>
        </div>
      ))}
    </div>
  );
}
