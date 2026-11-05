import { createElement, type ComponentType } from "react";
import { z } from "zod";
import {
  PreciseControl,
  MultiControl,
  SubtableControl,
  FileControl,
} from "./advanced-controls";
import {
  runtimeFieldV2Schema,
  type RuntimeFieldV2,
} from "@/api/generated/runtime";
import { Input } from "@/components/ui/input";
import { useTranslation } from "react-i18next";
import type { Json } from "./expression";
export type RecordData = Record<string, Json>;
export type ControlProps = {
  definition: RuntimeFieldV2;
  value: Json | undefined;
  onChange: (v: Json) => void;
  onBlur: () => void;
  name: string;
  id: string;
  disabled: boolean;
  options?: { value: Json; label: string }[];
};
export type RuntimeComponent = {
  name: string;
  version: string;
  source: string;
  schema: z.ZodType<RuntimeFieldV2>;
  propsSchema?: z.ZodType<ControlProps>;
  dispose?: () => void;
  render: ComponentType<ControlProps>;
  display?: ComponentType<{
    definition: RuntimeFieldV2;
    value: Json | undefined;
  }>;
};
function TextControl(p: ControlProps) {
  return (
    <Input
      id={p.id}
      name={p.name}
      disabled={p.disabled}
      value={String(p.value ?? "")}
      onBlur={p.onBlur}
      onChange={(e) => p.onChange(e.target.value)}
      maxLength={p.definition.maxLength || undefined}
    />
  );
}
function NumberControl(p: ControlProps) {
  return (
    <Input
      id={p.id}
      name={p.name}
      type="number"
      disabled={p.disabled}
      value={p.value === undefined ? "" : String(p.value)}
      onBlur={p.onBlur}
      onChange={(e) =>
        p.onChange(e.target.value === "" ? null : Number(e.target.value))
      }
    />
  );
}
function DateControl(p: ControlProps) {
  return (
    <Input
      id={p.id}
      name={p.name}
      type="date"
      disabled={p.disabled}
      value={String(p.value ?? "")}
      onBlur={p.onBlur}
      onChange={(e) => p.onChange(e.target.value)}
    />
  );
}
function BooleanControl(p: ControlProps) {
  return (
    <input
      id={p.id}
      name={p.name}
      type="checkbox"
      disabled={p.disabled}
      checked={p.value === true}
      onBlur={p.onBlur}
      onChange={(e) => p.onChange(e.target.checked)}
    />
  );
}
function TextareaControl(p: ControlProps) {
  return (
    <textarea
      id={p.id}
      name={p.name}
      className="editor-textarea"
      disabled={p.disabled}
      value={String(p.value ?? "")}
      onBlur={p.onBlur}
      onChange={(e) => p.onChange(e.target.value)}
      maxLength={p.definition.maxLength || undefined}
    />
  );
}
function SelectControl(p: ControlProps) {
  const { t } = useTranslation("runtime");
  return (
    <select
      id={p.id}
      name={p.name}
      disabled={p.disabled}
      value={String(p.value ?? "")}
      onBlur={p.onBlur}
      onChange={(e) => {
        const option = p.options?.find(
          (o) => String(o.value) === e.target.value,
        );
        p.onChange(option?.value ?? e.target.value);
      }}
    >
      <option value="">{t("choose")}</option>
      {p.options?.map((o) => (
        <option key={String(o.value)} value={String(o.value)}>
          {o.label}
        </option>
      ))}
    </select>
  );
}
function DefaultDisplay({
  definition,
  value,
}: {
  definition: RuntimeFieldV2;
  value: Json | undefined;
}) {
  const { t, i18n } = useTranslation("runtime");
  if (definition.valueLabelPrefix && value)
    return <>{t(definition.valueLabelPrefix + value)}</>;
  if (definition.type === "boolean") return <>{t(value ? "yes" : "no")}</>;
  if (
    (definition.type === "date" || definition.type === "timestamp") &&
    typeof value === "string" &&
    value
  ) {
    const date = new Date(
      definition.type === "date" ? value + "T00:00:00" : value,
    );
    if (!Number.isNaN(date.valueOf()))
      return (
        <>
          {new Intl.DateTimeFormat(
            i18n.language,
            definition.type === "date"
              ? { year: "numeric", month: "2-digit", day: "2-digit" }
              : { dateStyle: "short", timeStyle: "short" },
          ).format(date)}
        </>
      );
  }
  return <>{String(value ?? "")}</>;
}
export class ComponentRegistry {
  private definitions = new Map<string, RuntimeComponent>();
  register(item: RuntimeComponent) {
    const key = `${item.name}@${item.version}`;
    if (this.definitions.has(key) || !item.source)
      throw new Error("component_not_registered");
    this.definitions.set(key, item);
    return this;
  }
  attach(item: RuntimeComponent) {
    this.register(item);
    return () => {
      try {
        item.dispose?.();
      } finally {
        this.definitions.delete(`${item.name}@${item.version}`);
      }
    };
  }
  unregisterSource(source: string) {
    for (const [key, item] of this.definitions)
      if (item.source === source && source !== "core") {
        item.dispose?.();
        this.definitions.delete(key);
      }
  }
  validateProps(props: ControlProps) {
    const item = this.definitions.get(
      `${props.definition.component}@${props.definition.componentVersion}`,
    );
    item?.propsSchema?.parse(props);
  }
  owner(field: RuntimeFieldV2) {
    this.resolve(field);
    return this.definitions.get(`${field.component}@${field.componentVersion}`)!
      .source;
  }
  display(field: RuntimeFieldV2) {
    this.resolve(field);
    return (
      this.definitions.get(`${field.component}@${field.componentVersion}`)
        ?.display ?? DefaultDisplay
    );
  }
  resolve(field: RuntimeFieldV2) {
    const item = this.definitions.get(
      `${field.component}@${field.componentVersion}`,
    );
    if (!item) throw new Error("component_not_registered");
    item.schema.parse(field);
    return item.render;
  }
}
export const componentRegistry = new ComponentRegistry();
for (const [name, render] of Object.entries({
  money: PreciseControl,
  time: PreciseControl,
  datetime: PreciseControl,
  multiselect: MultiControl,
  subtable: SubtableControl,
  file: FileControl,
  switch: BooleanControl,
  text: TextControl,
  string: TextControl,
  textarea: TextareaControl,
  integer: NumberControl,
  number: NumberControl,
  boolean: BooleanControl,
  date: DateControl,
  timestamp: TextControl,
  enum: SelectControl,
  reference: SelectControl,
}))
  componentRegistry.register({
    name,
    version: "2.0",
    source: "core",
    schema: runtimeFieldV2Schema,
    render,
  });
export function RegisteredDisplay({
  definition,
  value,
}: {
  definition: RuntimeFieldV2;
  value: Json | undefined;
}) {
  return createElement(componentRegistry.display(definition), {
    definition,
    value,
  });
}
export function RegisteredControl(props: ControlProps) {
  componentRegistry.validateProps(props);
  return createElement(
    props.definition.multiple
      ? MultiControl
      : componentRegistry.resolve(props.definition),
    props,
  );
}
