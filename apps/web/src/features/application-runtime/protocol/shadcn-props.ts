import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { ComponentRendererProps } from "./component-registry";

export function labeledOptions(
  items: unknown,
  t: (key: string) => string,
  limit = 500,
) {
  const seen = new Set<string>();
  return Array.isArray(items)
    ? items.slice(0, limit).flatMap((item: unknown) => {
        if (
          typeof item !== "object" ||
          item === null ||
          !("value" in item) ||
          typeof item.value !== "string" ||
          !item.value ||
          seen.has(item.value) ||
          !("labelKey" in item) ||
          typeof item.labelKey !== "string"
        )
          return [];
        seen.add(item.value);
        return [
          {
            value: item.value,
            label: t(item.labelKey),
            disabled: "disabled" in item && item.disabled === true,
          },
        ];
      })
    : [];
}

export function useUI({ props, data, emit, readOnly }: ComponentRendererProps) {
  const { t, i18n } = useTranslation("runtime");
  const text = (key: unknown) =>
    typeof key === "string" && key ? t(key) : undefined;
  const string = (key: string) =>
    typeof props[key] === "string" ? props[key] : undefined;
  const number = (key: string) =>
    typeof props[key] === "number" && Number.isFinite(props[key])
      ? props[key]
      : undefined;
  const label = text(props.labelKey);
  const title = text(props.titleKey);
  const description = text(props.descriptionKey);
  const disabled = readOnly || props.disabled === true;
  const value = props.value ?? data;
  const options = labeledOptions(props.options, t);
  const event = (name: string, value?: unknown) => {
    if (!disabled) void emit(name, value);
  };
  return {
    t,
    i18n,
    label,
    title,
    description,
    disabled,
    value,
    options,
    event,
    text,
    string,
    number,
  };
}

// Keep local edits until the source changes, then display the new bound value.
export function useBoundValue<T>(source: T) {
  const [state, setState] = useState({ source, value: source });
  if (!Object.is(state.source, source)) {
    setState({ source, value: source });
  }
  return [
    Object.is(state.source, source) ? state.value : source,
    (value: T) => setState({ source, value }),
  ] as const;
}
