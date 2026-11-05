import { Component, createContext, useContext, type ReactNode } from "react";
import {
  runtimeApplicationSchema,
  runtimeFieldSchema,
} from "@/api/generated/runtime";
import type {
  RuntimeApplication,
  RuntimeField,
  RuntimePage,
} from "@/api/generated/runtime";
import { Input } from "@/components/ui/input";
import { useTranslation } from "react-i18next";

export function validateApplication(value: unknown): RuntimeApplication {
  const app = runtimeApplicationSchema.parse(value);
  if (new Set(app.pages.map((p) => p.id)).size !== app.pages.length)
    throw new Error("schema_invalid");
  for (const p of app.pages) {
    if (
      p.schemaVersion !== "1.0" ||
      p.layout !== "standard" ||
      !["list", "detail", "form", "custom"].includes(p.type)
    )
      throw new Error("schema_version_unsupported");
    if (new Set(p.fields.map((f) => f.name)).size !== p.fields.length)
      throw new Error("schema_invalid");
    for (const f of p.fields)
      if (
        !Object.hasOwn(componentRegistry, f.component) ||
        !f.labelKey ||
        f.maxLength < 0
      )
        throw new Error("component_not_registered");
  }
  return app;
}
export function pageOf(app: RuntimeApplication, type: string): RuntimePage {
  const p = app.pages.find((p) => p.type === type);
  if (!p) throw new Error("page_not_found");
  return p;
}
type ControlProps = {
  field: RuntimeField;
  name: string;
  value?: string;
  onChange: (value: string) => void;
  onBlur: () => void;
  ref: (element: HTMLInputElement | HTMLTextAreaElement | null) => void;
};
export const components = {
  text: ({ field, ...props }: ControlProps) => (
    <Input
      {...props}
      disabled={field.readOnly}
      maxLength={field.maxLength || undefined}
      onChange={(e) => props.onChange(e.target.value)}
    />
  ),
  textarea: ({ field, ...props }: ControlProps) => (
    <textarea
      {...props}
      className="editor-textarea"
      disabled={field.readOnly}
      maxLength={field.maxLength || undefined}
      onChange={(e) => props.onChange(e.target.value)}
    />
  ),
  status: ({ value }: ControlProps) => <span>{value}</span>,
};
export function registerComponents(
  entries: {
    name: string;
    version: string;
    propsSchema: typeof runtimeFieldSchema;
    render: typeof components.text;
  }[],
) {
  const result: Record<string, (typeof entries)[number]> = {};
  for (const entry of entries) {
    if (Object.hasOwn(result, entry.name) || entry.version !== "1.0")
      throw new Error("component_not_registered");
    result[entry.name] = entry;
  }
  return Object.freeze(result);
}
export const componentRegistry = registerComponents(
  Object.entries(components).map(([name, render]) => ({
    name,
    version: "1.0",
    propsSchema: runtimeFieldSchema,
    render,
  })),
);
export type RuntimeContext = {
  applicationId: string;
  pageId: string;
  entityRef: string;
  recordId?: number;
  metadataVersion: string;
  locale: string;
  timezone: string;
  selection: number[];
};
export const Context = createContext<RuntimeContext | null>(null);
export const useRuntimeContext = () => useContext(Context);
export function Diagnostic() {
  const { t } = useTranslation("articles");
  return <div role="alert">{t("runtime.schemaError")}</div>;
}
export class RuntimeBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? <Diagnostic /> : this.props.children;
  }
}
