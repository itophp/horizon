import { useId, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { ComponentRendererProps } from "./component-registry";

type Field = {
  key: string;
  labelKey: string;
  group: string;
  type: "text" | "integer" | "select";
  options?: { value: string; labelKey: string }[];
};
type Group = { id: string; labelKey: string };
type Entry = { key: string; value: string; overridden: boolean };
type Edit = { operation: "set" | "reset"; value: string };

function displayValue(entry: Entry | undefined): string {
  if (!entry) return "";
  try {
    const value: unknown = JSON.parse(entry.value);
    if (["string", "number", "boolean"].includes(typeof value))
      return String(value);
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
  const entries = (Array.isArray(data) ? data : []) as Entry[];
  const [group, selectGroup] = useState(groups[0]?.id);
  const [edits, setEdits] = useState<Record<string, Edit>>({});
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState<"noChangesKey" | "successKey">();
  const entryFor = (key: string) => entries.find((entry) => entry.key === key);
  const disabled = readOnly || pending;

  function edit(key: string, value: Edit) {
    setNotice(undefined);
    setEdits((previous) => ({ ...previous, [key]: value }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (disabled || !event.currentTarget.reportValidity()) return;
    const changes = fields.flatMap((field) => {
      const change = edits[field.key];
      if (!change) return [];
      const original = entryFor(field.key);
      if (change.operation === "reset") {
        return original?.overridden
          ? [{ key: field.key, operation: "reset", value: "" }]
          : [];
      }
      const value =
        field.type === "integer" ? Number(change.value) : change.value;
      if (typeof value === "number" && !Number.isSafeInteger(value)) return [];
      const encoded =
        field.type === "select"
          ? field.options?.find((option) => option.value === value)?.value
          : JSON.stringify(value);
      if (
        encoded === undefined ||
        (original?.overridden && original.value === encoded)
      )
        return [];
      return [{ key: field.key, operation: "set", value: encoded }];
    });
    if (!changes.length) {
      setNotice("noChangesKey");
      return;
    }
    setPending(true);
    setNotice(undefined);
    try {
      if ((await emit("submit", { changes })) === true) {
        setEdits({});
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
          const initialValue =
            field.type === "select"
              ? (original?.value ?? "")
              : displayValue(original);
          const current = edits[field.key] ?? {
            operation: original?.overridden ? "set" : "reset",
            value: initialValue,
          };
          const label = t(field.labelKey);
          const id = `${formId}-${field.key}`;
          const changeValue = (value: string) =>
            edit(field.key, { ...current, value });
          const locked = disabled || current.operation !== "set";
          return (
            <div
              className="gov-setting-row"
              hidden={!!group && field.group !== group}
              key={field.key}
            >
              <label htmlFor={id}>{label}</label>
              <div className="gov-setting-controls">
                <select
                  aria-label={t(String(props.operationLabelKey), {
                    field: label,
                  })}
                  value={current.operation}
                  disabled={disabled}
                  onChange={(event) =>
                    edit(field.key, {
                      ...current,
                      operation: event.target.value === "set" ? "set" : "reset",
                    })
                  }
                >
                  <option value="reset">
                    {t(String(props.resetLabelKey))}
                  </option>
                  <option value="set">{t(String(props.setLabelKey))}</option>
                </select>
                {field.type === "select" ? (
                  <select
                    id={id}
                    disabled={locked}
                    required
                    value={current.value}
                    onChange={(event) => changeValue(event.target.value)}
                  >
                    {(field.options ?? []).map((option) => (
                      <option value={option.value} key={option.value}>
                        {t(option.labelKey)}
                      </option>
                    ))}
                  </select>
                ) : (
                  <Input
                    id={id}
                    disabled={locked}
                    type={field.type === "integer" ? "number" : "text"}
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
      {!readOnly && (
        <div className="gov-save">
          <Button type="submit" disabled={disabled}>
            {t(String(props.saveLabelKey))}
          </Button>
        </div>
      )}
      {notice && (
        <p role={notice === "noChangesKey" ? "alert" : "status"}>
          {t(String(props[notice]))}
        </p>
      )}
    </form>
  );
}
