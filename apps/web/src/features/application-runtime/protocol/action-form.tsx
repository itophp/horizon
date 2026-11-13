import {
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { pathValue } from "../expression";
import { RecordListField } from "./record-list-field";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  localScheduleTime,
  scheduleInstant,
} from "@/features/governance/schedule-time";
import { BrowserFormContext } from "./browser-runtime";
import type { ComponentRendererProps } from "./component-registry";

export type ActionField = {
  name: string;
  labelKey: string;
  type:
    | "text"
    | "password"
    | "integer"
    | "select"
    | "datetime-local"
    | "boolean"
    | "records";
  visibleWhen?: { field: string; equals: string; invert?: boolean };
  locked?: boolean;
  hidden?: boolean;
  pattern?: string;
  trim?: boolean;
  valuePath?: string;
  rowFields?: ActionField[];
  rowTitlePath?: string;
  rowSubtitlePath?: string;
  rowStablePath?: string;
  addKey?: string;
  removeKey?: string;
  emptyKey?: string;
  lockStable?: boolean;
  allowAdd?: boolean;
  timeZoneKey?: string;
  defaultOffsetMinutes?: number;
  required?: boolean;
  invalidKey?: string;
  format?: "hex";
  min?: number;
  max?: number;
  minLength?: number;
  maxLength?: number;
  options?: {
    value: string | number;
    labelKey?: string;
    label?: string;
    disabled?: boolean;
  }[];
  defaultValue?: string | number | boolean;
};

export function ActionForm(input: ComponentRendererProps) {
  if (input.readOnly && input.props.showInPreview !== true) return null;
  return (
    <ActionFormFields key={String(input.props.instanceKey ?? "")} {...input} />
  );
}

function ActionFormFields({ props, readOnly, emit }: ComponentRendererProps) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const operationKey = useRef<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const submitting = useRef(false);
  const fields = (props.fields ?? []) as ActionField[];
  const initial = (field: ActionField) =>
    pathValue(props.initialValues, field.valuePath ?? field.name) ??
    field.defaultValue;
  const [choices, setChoices] = useState<Record<string, string>>({});
  const enabledFields = fields.filter((field) => {
    const condition = field.visibleWhen;
    if (!condition) return true;
    const source = fields.find((item) => item.name === condition.field);
    const value =
      choices[condition.field] ?? (source ? initial(source) : undefined);
    return (value === condition.equals) !== Boolean(condition.invert);
  });
  const [openedAt] = useState(Date.now);
  const defaults = useContext(BrowserFormContext);
  const formElement = useRef<HTMLFormElement>(null);
  const touched = useRef(new Set<string>());
  const [errorCode, setErrorCode] = useState<string>();
  const errorPrefix =
    typeof props.errorPrefix === "string" ? props.errorPrefix : undefined;
  const [prefilled, setPrefilled] = useState(false);
  const [invalidFields, setInvalidFields] = useState<Record<string, string>>(
    {},
  );
  const preset =
    typeof props.browserDefaults === "string"
      ? props.browserDefaults
      : undefined;
  const disabled = busy || readOnly || props.disabled === true;

  useEffect(() => {
    if (!defaults || !preset || readOnly) return;
    const controller = new AbortController();
    void defaults(preset, controller.signal)
      .then((values) => {
        if (controller.signal.aborted || !values || submitting.current) return;
        let applied = false;
        for (const [key, value] of Object.entries(values)) {
          const input = formElement.current?.elements.namedItem(key);
          if (input instanceof HTMLInputElement && !touched.current.has(key)) {
            input.value = value;
            applied = true;
          }
        }
        if (applied) setPrefilled(true);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [defaults, preset, readOnly]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    if (disabled || submitting.current || !form.reportValidity()) return;
    const raw = new FormData(form);
    const fieldsWithValues: [string, unknown][] = [];
    for (const field of enabledFields) {
      if (field.locked) {
        const value = initial(field);
        if (value !== undefined) fieldsWithValues.push([field.name, value]);
        continue;
      }
      if (field.type === "boolean") {
        fieldsWithValues.push([field.name, raw.has(field.name)]);
        continue;
      }
      const candidate = raw.get(field.name);
      if (typeof candidate !== "string") continue;
      const value = field.trim ? candidate.trim() : candidate;
      if (!value && !field.required && field.defaultValue === undefined)
        continue;
      if (field.required && !value) {
        setInvalidFields((previous) => ({
          ...previous,
          [field.name]: field.invalidKey ?? "runtime:invalid",
        }));
        return;
      }
      if (field.type === "records") {
        fieldsWithValues.push([field.name, JSON.parse(value) as unknown]);
        continue;
      }
      if (field.type === "datetime-local") {
        const instant = scheduleInstant(value);
        if (!instant) {
          setInvalidFields((previous) => ({
            ...previous,
            [field.name]: field.invalidKey ?? "runtime:invalid",
          }));
          return;
        }
        fieldsWithValues.push([field.name, instant]);
      } else {
        fieldsWithValues.push([
          field.name,
          field.type === "integer" ? Number(value) : value,
        ]);
      }
    }
    setSaved(false);
    setErrorCode(undefined);
    submitting.current = true;
    setBusy(true);
    operationKey.current ??= crypto.randomUUID();
    try {
      const done = await emit(
        "submit",
        {
          values: Object.fromEntries(fieldsWithValues),
          operationKey: operationKey.current,
        },
        { ephemeral: true, onError: errorPrefix ? setErrorCode : undefined },
      );
      if (done === true) {
        form.reset();
        touched.current.clear();
        setPrefilled(false);
        setInvalidFields({});
        setChoices({});
        operationKey.current = null;
        setSaved(true);
        await emit("success");
      }
    } finally {
      submitting.current = false;
      setBusy(false);
    }
  }

  const errorKey =
    errorCode && errorPrefix ? errorPrefix + "." + errorCode : undefined;
  return (
    <form
      className="entity-form"
      ref={formElement}
      onSubmit={(event) => void submit(event)}
      onChange={(event) => {
        setSaved(false);
        operationKey.current = null;
        const input = event.target;
        if (
          input instanceof HTMLInputElement ||
          input instanceof HTMLSelectElement
        ) {
          const name = input.name;
          touched.current.add(name);
          if (input instanceof HTMLSelectElement)
            setChoices((previous) => ({ ...previous, [name]: input.value }));
          setInvalidFields((previous) => {
            if (!Object.hasOwn(previous, name)) return previous;
            const next = { ...previous };
            delete next[name];
            return next;
          });
        }
      }}
      onInvalidCapture={(event) => {
        const input = event.target;
        if (!(
          input instanceof HTMLInputElement ||
          input instanceof HTMLSelectElement
        ))
          return;
        const field = fields.find((item) => item.name === input.name);
        if (field) {
          event.preventDefault();
          setInvalidFields((previous) => ({
            ...previous,
            [field.name]: field.invalidKey ?? "runtime:invalid",
          }));
        }
      }}
    >
      <div className="editor-grid">
        {enabledFields.map((field) => {
          const fieldId = `${id}-${field.name}`;
          const common = {
            id: fieldId,
            name: field.name,
            required: field.required,
            disabled: disabled || field.locked,
            "aria-invalid": Boolean(invalidFields[field.name]) || undefined,
            "aria-describedby":
              [
                invalidFields[field.name] ? fieldId + "-error" : "",
                field.timeZoneKey ? fieldId + "-zone" : "",
              ]
                .filter(Boolean)
                .join(" ") || undefined,
            defaultValue:
              field.type === "password"
                ? undefined
                : field.type === "datetime-local" &&
                    field.defaultOffsetMinutes !== undefined
                  ? localScheduleTime(
                      new Date(openedAt + field.defaultOffsetMinutes * 60000),
                    )
                  : String(initial(field) ?? ""),
          };
          if (field.type === "records")
            return (
              <RecordListField
                key={field.name}
                field={field}
                initial={initial(field)}
                disabled={disabled}
              />
            );
          return (
            <div className="editor-field" key={field.name}>
              <label htmlFor={fieldId}>{t(field.labelKey)}</label>
              {field.type === "select" ? (
                <select {...common}>
                  {field.options?.map((option) => (
                    <option
                      key={option.value}
                      value={option.value}
                      disabled={option.disabled}
                    >
                      {option.labelKey
                        ? t(option.labelKey)
                        : (option.label ?? option.value)}
                    </option>
                  ))}
                </select>
              ) : field.type === "boolean" ? (
                <input
                  type="checkbox"
                  name={field.name}
                  id={fieldId}
                  disabled={common.disabled}
                  defaultChecked={initial(field) === true}
                />
              ) : (
                <Input
                  {...common}
                  type={field.type === "integer" ? "number" : field.type}
                  min={field.min}
                  max={field.max}
                  minLength={field.minLength}
                  maxLength={field.maxLength}
                  step={field.type === "integer" ? 1 : undefined}
                  pattern={field.format === "hex" ? "[a-f0-9]+" : field.pattern}
                  autoComplete={field.type === "password" ? "off" : undefined}
                />
              )}
              {field.timeZoneKey && (
                <small id={fieldId + "-zone"}>
                  {t(field.timeZoneKey, {
                    zone: Intl.DateTimeFormat().resolvedOptions().timeZone,
                  })}
                </small>
              )}
              {invalidFields[field.name] && (
                <small
                  id={fieldId + "-error"}
                  role="alert"
                  className="field-error"
                >
                  {t(invalidFields[field.name])}
                </small>
              )}
            </div>
          );
        })}
      </div>
      {saved && typeof props.successKey === "string" && (
        <p role="status">{t(props.successKey)}</p>
      )}
      {prefilled && typeof props.prefilledKey === "string" && (
        <p className="muted-text">{t(props.prefilledKey)}</p>
      )}
      {errorKey && (
        <p role="alert" className="field-error">
          {t(i18n.exists(errorKey) ? errorKey : errorPrefix + ".unknown")}
        </p>
      )}
      <div className="form-actions">
        {typeof props.cancelKey === "string" && (
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            onClick={() => void emit("cancel")}
          >
            {t(props.cancelKey)}
          </Button>
        )}
        {!readOnly && props.disabled !== true && (
          <Button type="submit" disabled={disabled}>
            {t(String(props.submitKey))}
          </Button>
        )}
      </div>
    </form>
  );
}
