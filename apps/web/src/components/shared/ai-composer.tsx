import { useEffect, useId, useRef, type ReactNode } from "react";
import { FileText, Plus, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupTextarea,
  InputGroupText,
} from "@/components/ui/input-group";

const extensions = [".png", ".jpg", ".jpeg", ".webp", ".docx", ".xlsx", ".pdf"];
function fileExtension(name: string) {
  return name.slice(Math.max(0, name.lastIndexOf("."))).toLowerCase();
}
const imageTypes = new Set(["image/png", "image/jpeg", "image/webp"]);

export function invalidAIAttachment(file?: File) {
  if (!file) return false;
  return (
    !extensions.includes(fileExtension(file.name)) ||
    [...file.name].length > 255 ||
    file.size === 0 ||
    file.size > 2 * 1024 * 1024
  );
}

function AttachmentPreview({ file }: { file: File }) {
  const image = useRef<HTMLImageElement>(null);
  useEffect(() => {
    const preview = image.current;
    if (!preview || typeof URL.createObjectURL !== "function") return;
    const url = URL.createObjectURL(file);
    preview.src = url;
    return () => {
      preview.removeAttribute("src");
      URL.revokeObjectURL(url);
    };
  }, [file]);
  if (!imageTypes.has(file.type)) {
    return <FileText className="size-5 shrink-0" aria-hidden="true" />;
  }
  return (
    <img ref={image} alt="" className="size-8 shrink-0 rounded object-cover" />
  );
}

export function AIComposer({
  value,
  onChange,
  label,
  placeholder,
  maxLength,
  file,
  onFileChange,
  onSubmit,
  submitDisabled,
  disabled,
  busy,
  hint,
  action,
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  placeholder: string;
  maxLength: number;
  file?: File;
  onFileChange: (file: File | undefined) => void;
  onSubmit: () => void;
  submitDisabled: boolean;
  disabled: boolean;
  busy: boolean;
  hint: string;
  action: ReactNode;
}) {
  const { t, i18n } = useTranslation("runtime");
  const id = useId();
  const picker = useRef<HTMLInputElement>(null);
  const invalidFile = invalidAIAttachment(file);
  const fileSize = file?.size ?? 0;
  const kilobyte = 1024;
  const megabyte = kilobyte * kilobyte;
  const size = new Intl.NumberFormat(i18n.language, {
    style: "unit",
    unit:
      fileSize < kilobyte
        ? "byte"
        : fileSize < megabyte
          ? "kilobyte"
          : "megabyte",
    maximumFractionDigits: 1,
  }).format(
    fileSize /
      (fileSize < kilobyte ? 1 : fileSize < megabyte ? kilobyte : megabyte),
  );

  return (
    <div className="min-w-0 space-y-1">
      <label htmlFor={id} className="sr-only">
        {label}
      </label>
      <InputGroup className="rounded-xl">
        {file && (
          <InputGroupAddon align="block-start">
            <div className="inline-flex max-w-full min-w-0 items-center gap-2 rounded-lg border border-input px-2 py-1">
              {invalidFile ? (
                <FileText className="size-5 shrink-0" aria-hidden="true" />
              ) : (
                <AttachmentPreview file={file} />
              )}
              <div className="min-w-0">
                <p
                  className="truncate text-sm text-foreground"
                  title={file.name}
                >
                  {file.name}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("ai.attachmentMetadata", {
                    format: fileExtension(file.name).slice(1).toUpperCase(),
                    size,
                  })}
                </p>
              </div>
              <InputGroupButton
                type="button"
                size="icon-xs"
                aria-label={t("ai.removeAttachment")}
                title={t("ai.removeAttachment")}
                disabled={disabled || busy}
                onClick={() => onFileChange(undefined)}
              >
                <X />
              </InputGroupButton>
            </div>
          </InputGroupAddon>
        )}
        <InputGroupTextarea
          id={id}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          rows={2}
          maxLength={maxLength}
          placeholder={placeholder}
          disabled={disabled}
          aria-describedby={`${id}-hint`}
          onKeyDown={(event) => {
            if (
              event.key !== "Enter" ||
              event.shiftKey ||
              event.nativeEvent.isComposing ||
              event.keyCode === 229
            )
              return;
            event.preventDefault();
            if (!submitDisabled && !disabled && !busy && !event.repeat)
              onSubmit();
          }}
        />
        <InputGroupAddon align="block-end" className="justify-between">
          <div className="flex min-w-0 items-center gap-2">
            <InputGroupButton
              type="button"
              size="icon-sm"
              aria-label={t("ai.addAttachment")}
              title={t("ai.attachmentHint")}
              disabled={disabled || busy}
              onClick={() => picker.current?.click()}
            >
              <Plus />
            </InputGroupButton>
            <InputGroupText
              id={`${id}-hint`}
              className="hidden text-xs sm:inline"
            >
              {hint}
            </InputGroupText>
          </div>
          {action}
        </InputGroupAddon>
      </InputGroup>
      <input
        ref={picker}
        type="file"
        hidden
        aria-label={t("ai.attachment")}
        accept={extensions.join(",")}
        disabled={disabled || busy}
        onChange={(event) => {
          const selected = event.target.files?.[0];
          if (selected) onFileChange(selected);
          event.target.value = "";
        }}
      />
      {invalidFile && (
        <p role="status" className="text-sm text-destructive">
          {t("ai.attachmentInvalid")}
        </p>
      )}
    </div>
  );
}
