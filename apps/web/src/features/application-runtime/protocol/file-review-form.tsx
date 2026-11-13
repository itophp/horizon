import { useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ComponentRendererProps } from "./component-registry";

export function FileReviewForm({
  props,
  children,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const { t } = useTranslation();
  const [file, setFile] = useState<File>();
  const [busy, setBusy] = useState(false);
  const [reviewed, setReviewed] = useState<{
    content: string;
    format: string;
  }>();
  const [error, setError] = useState<string>();
  const operationKey = useRef<string | null>(null);
  const pending = useRef(false);
  if (readOnly) return null;

  async function review() {
    if (!file || pending.current) return;
    setReviewed(undefined);
    setError(undefined);
    const format = file.name.split(".").pop()?.toLowerCase() ?? "";
    if (
      !(props.formats as string[]).includes(format) ||
      file.size > Number(props.maxBytes)
    ) {
      setError(String(props.invalidFileKey));
      return;
    }
    pending.current = true;
    setBusy(true);
    try {
      const content = await file.text();
      if (
        (await emit("review", { content, format }, { ephemeral: true })) ===
        true
      )
        setReviewed({ content, format });
    } catch {
      setError(String(props.invalidFileKey));
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!reviewed || props.reviewAccepted !== true || pending.current) return;
    const reason = String(
      new FormData(event.currentTarget).get("reason") ?? "",
    ).trim();
    if (reason.length < 5) {
      setError(String(props.invalidReasonKey));
      return;
    }
    pending.current = true;
    setBusy(true);
    setError(undefined);
    operationKey.current ??= crypto.randomUUID();
    try {
      if (
        (await emit(
          "submit",
          { ...reviewed, reason, operationKey: operationKey.current },
          { ephemeral: true },
        )) === true
      )
        await emit("success");
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }

  return (
    <form className="admin-form" onSubmit={(event) => void submit(event)}>
      <label>
        <span>{t(String(props.fileKey))}</span>
        <Input
          type="file"
          accept={(props.formats as string[]).map((ext) => "." + ext).join(",")}
          disabled={busy}
          onChange={(event) => {
            setFile(event.target.files?.[0]);
            setReviewed(undefined);
            setError(undefined);
            operationKey.current = null;
          }}
        />
      </label>
      <Button
        type="button"
        variant="outline"
        disabled={!file || busy}
        onClick={() => void review()}
      >
        {t(String(props.reviewKey))}
      </Button>
      {reviewed && children}
      <label>
        <span>{t(String(props.reasonKey))}</span>
        <Input
          name="reason"
          required
          minLength={5}
          maxLength={500}
          disabled={busy}
          onChange={() => {
            operationKey.current = null;
          }}
        />
      </label>
      {error && (
        <p role="alert" className="field-error">
          {t(error)}
        </p>
      )}
      <div className="form-actions">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onClick={() => void emit("cancel")}
        >
          {t(String(props.cancelKey))}
        </Button>
        <Button
          type="submit"
          disabled={!reviewed || props.reviewAccepted !== true || busy}
        >
          {t(String(props.submitKey))}
        </Button>
      </div>
    </form>
  );
}
