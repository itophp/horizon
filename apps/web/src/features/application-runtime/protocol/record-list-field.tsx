import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { pathValue } from "../expression";
import type { ActionField } from "./action-form";

export function RecordListField({
  field,
  initial,
  disabled,
}: {
  field: ActionField;
  initial: unknown;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  const [rows, setRows] = useState(() =>
    (Array.isArray(initial) ? initial : []).map(
      (value: Record<string, unknown>) => ({ key: crypto.randomUUID(), value }),
    ),
  );
  const fields = field.rowFields ?? [];
  return (
    <section className="admin-panel entity-card">
      <header className="gov-card-heading gov-line">
        <h2>{t(field.labelKey)}</h2>
        {!disabled && field.allowAdd !== false && (
          <Button
            type="button"
            variant="outline"
            onClick={() =>
              setRows((previous) => [
                ...previous,
                {
                  key: crypto.randomUUID(),
                  value: Object.fromEntries(
                    fields.map((item) => [item.name, item.defaultValue ?? ""]),
                  ),
                },
              ])
            }
          >
            {t(field.addKey ?? "admin:actions.create")}
          </Button>
        )}
      </header>
      <input
        type="hidden"
        name={field.name}
        value={JSON.stringify(rows.map((row) => row.value))}
      />
      <div className="gov-dictionary-items">
        {rows.length === 0 && field.emptyKey && (
          <p className="gov-empty">{t(field.emptyKey)}</p>
        )}
        {rows.map(({ key, value }) => {
          const stable = Boolean(
            field.rowStablePath && pathValue(value, field.rowStablePath),
          );
          const update = (name: string, next: unknown) =>
            setRows((previous) =>
              previous.map((row) =>
                row.key === key
                  ? { ...row, value: { ...row.value, [name]: next } }
                  : row,
              ),
            );
          return (
            <details className="gov-item" key={key} open={!stable}>
              <summary>
                <code>
                  {String(
                    pathValue(value, field.rowTitlePath ?? "name") ||
                      t(field.labelKey),
                  )}
                </code>
                {field.rowSubtitlePath && (
                  <span>
                    {String(pathValue(value, field.rowSubtitlePath) ?? "")}
                  </span>
                )}
              </summary>
              <div className="form-grid">
                {fields
                  .filter((item) => !item.hidden)
                  .map((item) => {
                    const locked = disabled || (stable && item.lockStable);
                    const current = value[item.name] ?? item.defaultValue ?? "";
                    return (
                      <label key={item.name}>
                        <span>{t(item.labelKey)}</span>
                        {item.type === "select" ? (
                          <select
                            value={String(current)}
                            disabled={locked}
                            onChange={(e) => update(item.name, e.target.value)}
                          >
                            {item.options?.map((option) => (
                              <option key={option.value} value={option.value}>
                                {option.labelKey
                                  ? t(option.labelKey)
                                  : (option.label ?? option.value)}
                              </option>
                            ))}
                          </select>
                        ) : item.type === "boolean" ? (
                          <input
                            type="checkbox"
                            disabled={locked}
                            checked={current === true}
                            onChange={(e) =>
                              update(item.name, e.target.checked)
                            }
                          />
                        ) : (
                          <Input
                            type={item.type === "integer" ? "number" : "text"}
                            value={String(current)}
                            disabled={locked}
                            required={item.required}
                            pattern={item.pattern}
                            minLength={item.minLength}
                            maxLength={item.maxLength}
                            onChange={(e) =>
                              update(
                                item.name,
                                item.type === "integer"
                                  ? Number(e.target.value)
                                  : e.target.value,
                              )
                            }
                          />
                        )}
                      </label>
                    );
                  })}
              </div>
              {!stable && !disabled && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() =>
                    setRows((previous) =>
                      previous.filter((row) => row.key !== key),
                    )
                  }
                >
                  {t(field.removeKey ?? "admin:actions.delete")}
                </Button>
              )}
            </details>
          );
        })}
      </div>
    </section>
  );
}
