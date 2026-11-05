import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { filesUpload } from "@/lib/capabilities";
import { downloadFile } from "@/features/governance/files";
import { ErrorBanner } from "@/features/administration/shared";
import { RegisteredControl, type ControlProps } from "./components";
import type { Json } from "./expression";
export function PreciseControl(p: ControlProps) {
  const type =
    p.definition.type === "time"
      ? "time"
      : p.definition.type === "datetime"
        ? "datetime-local"
        : "text";
  return (
    <Input
      id={p.id}
      name={p.name}
      disabled={p.disabled}
      type={type}
      inputMode={p.definition.type === "money" ? "decimal" : undefined}
      value={typeof p.value === "string" ? p.value : ""}
      onBlur={p.onBlur}
      onChange={(event) => p.onChange(event.target.value)}
    />
  );
}
export function MultiControl(p: ControlProps) {
  const values = Array.isArray(p.value) ? p.value : [];
  return (
    <select
      id={p.id}
      name={p.name}
      multiple
      disabled={p.disabled}
      className="editor-input"
      value={values.map(String)}
      onBlur={p.onBlur}
      onChange={(event) =>
        p.onChange(
          Array.from(
            event.target.selectedOptions,
            (option) =>
              p.options?.find((item) => String(item.value) === option.value)
                ?.value ?? option.value,
          ),
        )
      }
    >
      {p.options?.map((option) => (
        <option key={String(option.value)} value={String(option.value)}>
          {option.label}
        </option>
      ))}
    </select>
  );
}
export function SubtableControl(p: ControlProps) {
  const { t } = useTranslation("runtime");
  const rows = Array.isArray(p.value) ? p.value : [];
  const fields = p.definition.children ?? [];
  const replace = (index: number, name: string, value: Json) =>
    p.onChange(
      rows.map((row, at) =>
        at === index
          ? {
              ...(row && typeof row === "object" && !Array.isArray(row)
                ? row
                : {}),
              [name]: value,
            }
          : row,
      ),
    );
  return (
    <div id={p.id} className="runtime-subtable">
      {rows.map((row, index) => (
        <fieldset key={index} disabled={p.disabled}>
          <legend>{t("rowNumber", { number: index + 1 })}</legend>
          {fields.map((field) => (
            <label key={field.name} htmlFor={`${p.id}-${index}-${field.name}`}>
              {t(field.labelKey)}
              <RegisteredControl
                definition={field}
                id={`${p.id}-${index}-${field.name}`}
                name={`${p.name}.${index}.${field.name}`}
                value={
                  row && typeof row === "object" && !Array.isArray(row)
                    ? row[field.name]
                    : undefined
                }
                onChange={(value) => replace(index, field.name, value)}
                onBlur={p.onBlur}
                disabled={p.disabled || field.readOnly}
              />
            </label>
          ))}
          <Button
            type="button"
            disabled={p.disabled}
            onClick={() => p.onChange(rows.filter((_, at) => at !== index))}
          >
            {t("removeRow")}
          </Button>
        </fieldset>
      ))}
      <Button
        type="button"
        disabled={p.disabled || rows.length >= (p.definition.maxItems || 100)}
        onClick={() =>
          p.onChange([
            ...rows,
            Object.fromEntries(
              fields.map((field) => [
                field.name,
                field.type === "boolean"
                  ? false
                  : field.type === "array"
                    ? []
                    : "",
              ]),
            ),
          ])
        }
      >
        {t("addRow")}
      </Button>
    </div>
  );
}
function readBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("unavailable"));
    reader.onload = () =>
      typeof reader.result === "string"
        ? resolve(reader.result.split(",")[1])
        : reject(new Error("invalid"));
    reader.readAsDataURL(file);
  });
}
export function FileControl(p: ControlProps) {
  const { t } = useTranslation("runtime");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  return (
    <div>
      <Input
        id={p.id}
        name={p.name}
        type="file"
        disabled={p.disabled || busy}
        onBlur={p.onBlur}
        onChange={async (event) => {
          const file = event.target.files?.[0];
          if (!file) return;
          setBusy(true);
          setError(undefined);
          try {
            const content = await readBase64(file);
            const response = await filesUpload({
              name: file.name,
              purpose: "attachment",
              content,
            });
            if (response.status !== 200 && response.status !== 201)
              throw new Error("unavailable");
            p.onChange(response.data.id);
          } catch (error) {
            setError(error);
          } finally {
            setBusy(false);
          }
        }}
      />
      {typeof p.value === "number" && p.value > 0 && (
        <Button
          type="button"
          disabled={busy}
          onClick={async () => {
            try {
              await downloadFile(Number(p.value));
            } catch (error) {
              setError(error);
            }
          }}
        >
          {t("downloadAttachment")}
        </Button>
      )}
      {error !== undefined && <ErrorBanner error={error} />}
    </div>
  );
}
