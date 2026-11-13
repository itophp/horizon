import { useId, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { pathValue } from "../expression";
import type { ComponentRendererProps } from "./component-registry";

type Field = {
  key: string;
  labelKey: string;
  group: string;
  type: "text" | "integer" | "select" | "json" | "password";
  defaultValue?: string;
  clearable?: boolean;
  optionRows?: { source: string; valuePath: string; labelPath: string };
  min?: number;
  max?: number;
  options?: { value: string; labelKey?: string; label?: string }[];
};
type Group = { id: string; labelKey: string };
type Entry = {
  key: string;
  value: string;
  overridden: boolean;
  configured?: boolean;
};
type Edit = { operation: "set" | "reset" | "clear"; value: string };

function displayValue(value: string): string {
  try {
    const parsed: unknown = JSON.parse(value);
    if (["string", "number", "boolean"].includes(typeof parsed))
      return String(parsed);
  } catch {
    // Entries without a scalar value have no editable default.
  }
  return "";
}

export function ChangeListForm({
  props,
  data,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const { t } = useTranslation();
  const formId = useId();
  const fields = (props.fields ?? []) as Field[];
  const groups = (props.groups ?? []) as Group[];
  const source =
    typeof props.entriesPath === "string"
      ? pathValue(data, props.entriesPath)
      : data;
  const entries = (Array.isArray(source) ? source : []) as Entry[];
  const [group, selectGroup] = useState(groups[0]?.id);
  const [edits, setEdits] = useState<Record<string, Edit>>({});
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState<
    "noChangesKey" | "successKey" | "invalidKey"
  >();
  const [reason, setReason] = useState("");
  const entryFor = (key: string) => entries.find((entry) => entry.key === key);
  const writable = !readOnly && props.disabled !== true;
  const disabled = !writable || pending;

  function fieldOptions(field: Field) {
    const rows =
      field.optionRows && pathValue(props.optionData, field.optionRows.source);
    const options = field.options ?? [];
    if (!Array.isArray(rows) || !field.optionRows) return options;
    const { valuePath, labelPath } = field.optionRows;
    return [
      ...options,
      ...rows.flatMap((row): NonNullable<Field["options"]> => {
        const value = pathValue(row, valuePath);
        const label = pathValue(row, labelPath);
        return (typeof value === "string" || typeof value === "number") &&
          typeof label === "string"
          ? [{ value: JSON.stringify(value), label }]
          : [];
      }),
    ];
  }

  function edit(key: string, value: Edit) {
    setNotice(undefined);
    setEdits((previous) => ({ ...previous, [key]: value }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (disabled || !event.currentTarget.reportValidity()) return;
    const changes: {
      key: string;
      operation: Edit["operation"];
      value: string;
    }[] = [];
    for (const field of fields) {
      const change = edits[field.key];
      if (!change) continue;
      const original = entryFor(field.key);
      if (change.operation === "reset") {
        if (original?.overridden)
          changes.push({ key: field.key, operation: "reset", value: "" });
        continue;
      }
      if (change.operation === "clear") {
        changes.push({ key: field.key, operation: "clear", value: "" });
        continue;
      }
      if (field.type === "password" && !change.value) continue;
      let value: string;
      if (field.type === "json") {
        try {
          JSON.parse(change.value);
        } catch {
          setNotice("invalidKey");
          return;
        }
        value = change.value;
      } else if (field.type === "select") {
        if (
          !fieldOptions(field).some((option) => option.value === change.value)
        )
          continue;
        value = change.value;
      } else {
        const scalar =
          field.type === "integer" ? Number(change.value) : change.value;
        if (typeof scalar === "number" && !Number.isSafeInteger(scalar)) {
          setNotice("invalidKey");
          return;
        }
        value = JSON.stringify(scalar);
      }
      if (!original?.overridden || original.value !== value)
        changes.push({ key: field.key, operation: "set", value });
    }
    if (props.reasonKey && reason.trim().length < 5) {
      setNotice("invalidKey");
      return;
    }
    if (!changes.length) {
      setNotice("noChangesKey");
      return;
    }
    setPending(true);
    setNotice(undefined);
    try {
      if (
        (await emit(
          "submit",
          { changes, reason: reason.trim() },
          { ephemeral: true },
        )) === true
      ) {
        setEdits({});
        setReason("");
        setNotice("successKey");
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="gov-settings" onSubmit={(event) => void submit(event)}>
      {groups.length > 1 && (
        <div className="gov-tabs">
          {groups.map((item) => (
            <button
              key={item.id}
              type="button"
              aria-pressed={group === item.id}
              onClick={() => selectGroup(item.id)}
            >
              {t(item.labelKey)}
            </button>
          ))}
        </div>
      )}
      <section className="admin-panel entity-card gov-fields">
        {fields.map((field) => {
          const original = entryFor(field.key);
          const raw = original?.value ?? field.defaultValue ?? "";
          const initialValue =
            field.type === "password"
              ? ""
              : field.type === "select" || field.type === "json"
                ? raw
                : displayValue(raw);
          const current = edits[field.key] ?? {
            operation: original?.overridden ? "set" : "reset",
            value: initialValue,
          };
          const label = t(field.labelKey);
          const id = `${formId}-${field.key}`;
          const changeValue = (value: string) =>
            edit(field.key, { ...current, value });
          const fieldDisabled =
            disabled ||
            Boolean(
              field.optionRows &&
              !Array.isArray(
                pathValue(props.optionData, field.optionRows.source),
              ),
            );
          const locked = fieldDisabled || current.operation !== "set";
          return (
            <div
              className="gov-setting-row"
              hidden={!!group && field.group !== group}
              key={field.key}
            >
              <div>
                <label htmlFor={id}>{label}</label>
                {field.type === "password" &&
                  typeof props.configuredKey === "string" && (
                    <small>
                      {t(
                        String(
                          original?.configured
                            ? props.configuredKey
                            : props.secretHintKey,
                        ),
                      )}
                    </small>
                  )}
                {field.type !== "password" &&
                  typeof props.effectiveKey === "string" && (
                    <small>
                      {t(String(props.effectiveKey))} {raw}
                    </small>
                  )}
              </div>
              <div className="gov-setting-controls">
                <select
                  aria-label={t(String(props.operationLabelKey), {
                    field: label,
                  })}
                  value={current.operation}
                  disabled={fieldDisabled}
                  onChange={(event) =>
                    edit(field.key, {
                      ...current,
                      operation:
                        event.target.value === "set"
                          ? "set"
                          : event.target.value === "clear"
                            ? "clear"
                            : "reset",
                    })
                  }
                >
                  <option value="reset">
                    {t(String(props.resetLabelKey))}
                  </option>
                  <option value="set">{t(String(props.setLabelKey))}</option>
                  {field.clearable && (
                    <option value="clear">
                      {t(String(props.clearLabelKey))}
                    </option>
                  )}
                </select>
                {field.type === "select" ? (
                  <select
                    id={id}
                    disabled={locked}
                    required
                    value={current.value}
                    onChange={(event) => changeValue(event.target.value)}
                  >
                    {fieldOptions(field).map((option) => (
                      <option value={option.value} key={option.value}>
                        {option.labelKey
                          ? t(option.labelKey)
                          : (option.label ?? option.value)}
                      </option>
                    ))}
                  </select>
                ) : (
                  <Input
                    id={id}
                    disabled={locked}
                    type={
                      field.type === "integer"
                        ? "number"
                        : field.type === "password"
                          ? "password"
                          : "text"
                    }
                    autoComplete={
                      field.type === "password" ? "new-password" : undefined
                    }
                    min={field.min}
                    max={field.max}
                    required={field.type === "integer"}
                    value={current.value}
                    onChange={(event) => changeValue(event.target.value)}
                  />
                )}
              </div>
            </div>
          );
        })}
      </section>
      {writable && (
        <div className="gov-save">
          {typeof props.reasonKey === "string" && (
            <label>
              {t(props.reasonKey)}
              <Input
                required
                minLength={5}
                maxLength={500}
                value={reason}
                disabled={pending}
                onChange={(event) => setReason(event.target.value)}
              />
            </label>
          )}
          <Button type="submit" disabled={disabled}>
            {t(String(props.saveLabelKey))}
          </Button>
        </div>
      )}
      {notice && (
        <p role={notice === "successKey" ? "status" : "alert"}>
          {t(String(props[notice] ?? "runtime:invalid"))}
        </p>
      )}
    </form>
  );
}
